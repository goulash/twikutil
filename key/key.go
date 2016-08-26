// Copyright (c) 2015, Ben Morgan. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package key

import (
	"errors"
	"fmt"
	"reflect"
	"sort"

	"github.com/goulash/errs"
	"github.com/goulash/twikutil"
)

// Error types {{{

var (
	ErrCatch        = errors.New("hidden: should only be used with TyperFunc")
	ErrKeyExists    = errors.New("key already exists in map")
	ErrMissingValue = errors.New("default value required")
	ErrNotTyper     = errors.New("type checker is invalid")
)

type RequiredError struct {
	Name string
}

func (e RequiredError) Error() string { return fmt.Sprintf("%s: required but unset", e.Name) }

type ImplementsError struct {
	Name  string
	Got   reflect.Type
	Wants string
}

func (e ImplementsError) Error() string {
	return fmt.Sprintf("%s: value (type %s) does not implement %s", e.Got, e.Wants)
}

type TypeError struct {
	Name  string
	Got   reflect.Type
	Wants string
}

func NewTypeError(name string, got interface{}, wants interface{}) *TypeError {
	err := &TypeError{Name: name}
	switch t := got.(type) {
	case reflect.Type:
		err.Got = t
	default:
		err.Got = reflect.TypeOf(t)
	}
	switch w := wants.(type) {
	case reflect.Type:
		err.Wants = w.String()
	case string:
		err.Wants = w
	default:
		err.Wants = reflect.TypeOf(wants).String()
	}
	return err
}

func (e TypeError) Error() string {
	return fmt.Sprintf("%s: value (type %v) is not of type %s", e.Name, e.Got, e.Wants)
}

// }}}

// Mode type {{{

type Mode int

const (
	Reserved Mode = 0
	Read     Mode = 1
	Write    Mode = 2
	Required Mode = 4

	ReadWrite Mode = Read | Write
)

// }}}

// KeyMap {{{

type KeyMap map[string]*Key

func NewKeyMap() KeyMap { return make(KeyMap) }

func (km KeyMap) KeyNames() []string {
	ns := make([]string, 0, len(km))
	for k := range km {
		ns = append(ns, k)
	}
	sort.Strings(ns)
	return ns
}

func (km KeyMap) Create(name string, typer, def interface{}, m Mode, desc string) (*Key, error) {
	if km[name] != nil {
		return nil, ErrKeyExists
	}
	k, err := New(name, typer, def, m, desc)
	if err != nil {
		return nil, err
	}
	km[name] = k
	return k, nil
}

func (km KeyMap) CreateAuto(name string, def interface{}, m Mode, desc string) (*Key, error) {
	if km[name] != nil {
		return nil, ErrKeyExists
	}
	k, err := NewAuto(name, def, m, desc)
	if err != nil {
		return nil, err
	}
	km[name] = k
	return k, nil
}

func (km KeyMap) Keys() Keys {
	ks := make(Keys, 0, len(km))
	for _, v := range km {
		ks = append(ks, v)
	}
	sort.Sort(ks)
	return ks
}

func (km KeyMap) Acquire(e *twikutil.Executer, h errs.Handler) error {
	return km.Keys().Acquire(e, h)
}

func (km KeyMap) Apply(e *twikutil.Executer) error {
	return km.Keys().Apply(e)
}

func (km KeyMap) Clobber(e *twikutil.Executer) error {
	return km.Keys().Clobber(e)
}

// }}}

// Keys {{{

type Keys []*Key

func (ks Keys) Len() int           { return len(ks) }
func (ks Keys) Less(i, j int) bool { return ks[i].name < ks[j].name }
func (ks Keys) Swap(i, j int)      { ks[i], ks[j] = ks[j], ks[i] }

func (ks Keys) Acquire(e *twikutil.Executer, h errs.Handler) error {
	for _, k := range ks {
		err := k.Acquire(e)
		if err = h(err); err != nil {
			return err
		}
	}
	return nil
}

func (ks Keys) Apply(e *twikutil.Executer) error {
	for _, k := range ks {
		err := k.Apply(e)
		if err != nil {
			return err
		}
	}
	return nil
}

// ApplyOrNil applies all keys, and keys that are read-only are
// still set to nil.
func (ks Keys) ApplyOrNil(e *twikutil.Executer) error {
	for _, k := range ks {
		err := k.ApplyOrNil(e)
		if err != nil {
			return err
		}
	}
	return nil
}

func (ks Keys) Clobber(e *twikutil.Executer) error {
	for _, k := range ks {
		err := k.Clobber(e)
		if err != nil {
			return err
		}
	}
	return nil
}

// }}}

type Key struct {
	name  string
	desc  string
	mode  Mode
	typer interface{}
	val   interface{}
}

// Must panics if err is not nil and returns the key otherwise.
func Must(k *Key, err error) *Key {
	if err != nil {
		panic(err)
	}
	return k
}

func New(name string, typer, def interface{}, m Mode, desc string) (*Key, error) {
	if !IsTyper(typer) {
		return nil, ErrNotTyper
	}
	k := &Key{
		name:  name,
		desc:  desc,
		mode:  m,
		typer: typer,
	}
	return k, k.Set(def)
}

func NewAuto(name string, def interface{}, m Mode, desc string) (*Key, error) {
	if def == nil {
		return nil, ErrMissingValue
	}
	return &Key{
		name:  name,
		desc:  desc,
		mode:  m,
		typer: reflect.TypeOf(def),
		val:   def,
	}, nil
}

func (k Key) Empty() bool     { return k.val == nil }
func (k Key) Type() string    { return k.typer.(fmt.Stringer).String() }
func (k Key) Name() string    { return k.name }
func (k Key) Desc() string    { return k.desc }
func (k Key) Mode() Mode      { return k.mode }
func (k *Key) SetMode(m Mode) { k.mode = m }

// Get returns a value that is either nil or guaranteed to conform to the defined type.
func (k Key) Get() interface{} { return k.val }

func (k Key) GetOr(def interface{}) interface{} {
	if k.val == nil {
		return def
	}
	return k.val
}

func (k *Key) Set(v interface{}) (err error) {
	// If v is nil, then we're effectively deleting the stored value.
	if v != nil {
		v, err = Check(k.name, k.typer, v)
		if err != nil {
			return err
		}
	}

	k.val = v
	return nil
}

func (k *Key) Acquire(e *twikutil.Executer) error {
	if k.mode&Read == 0 {
		// This key does not want to be updated.
		return nil
	}

	if !e.Has(k.name) {
		if k.mode&Required != 0 {
			return &RequiredError{k.name}
		}
		return nil
	}

	v, _ := e.Get(k.name)
	return k.Set(v)
}

func (k *Key) Apply(e *twikutil.Executer) error {
	if k.mode&Write == 0 {
		return nil
	}
	if !e.Has(k.name) {
		return e.Set(k.name, k.val)
	}
	return nil
}

func (k *Key) ApplyOrNil(e *twikutil.Executer) error {
	if k.mode&Write == 0 {
		return e.Set(k.name, nil)
	}
	if !e.Has(k.name) {
		return e.Set(k.name, k.val)
	}
	return nil
}

func (k *Key) Clobber(e *twikutil.Executer) error {
	if k.mode&Write == 0 {
		return nil
	}

	return e.Set(k.name, k.val)
}

func Check(name string, typer interface{}, v interface{}) (interface{}, error) {
	chkType := func(t reflect.Type) error {
		vt := reflect.TypeOf(v)
		if t.Kind() == reflect.Interface {
			if !vt.Implements(t) {
				return ImplementsError{name, vt, t.String()}
			}
		} else if vt != t {
			return NewTypeError(name, vt, t)
		}
		return nil
	}

	switch t := typer.(type) {
	case reflect.Type:
		return v, chkType(t)
	case Typer:
		v, err := t.Coerce(v)
		if err != nil {
			switch et := err.(type) {
			case *TypeError:
				if et.Name == "" {
					et.Name = name
				}
				return v, et
			case *ImplementsError:
				if et.Name == "" {
					et.Name = name
				}
				return v, et
			}
		}
		return v, err
	default:
		return nil, ErrNotTyper
	}

}
