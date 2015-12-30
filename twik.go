// Copyright (c) 2015, Ben Morgan. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package twikutil

import (
	"bytes"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"gopkg.in/twik.v1"
)

func Export(s *twik.Scope, fm FuncMap) {
	fm.Export(s)
}

type FuncMap map[string]interface{}

func (fm FuncMap) Import(o FuncMap) {
	for k, v := range o {
		fm[k] = v
	}
}

func (fm FuncMap) Export(s *twik.Scope) {
	for k, v := range fm {
		if v != nil {
			s.Create(k, Func(k, v))
		}
	}
}

func (fm FuncMap) Keys() []string {
	keys := make([]string, 0, len(fm))
	for k := range fm {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func (fm FuncMap) FormatList() []string {
	xs := fm.Keys()
	for i, k := range xs {
		xs[i] = Format(k, fm[k])
	}
	return xs
}

// Since the number of return values is dependent on the function, it does not
// change, and therefore we can create a function now already to handle the
// output.
func funcReturn(name string, f interface{}) func([]reflect.Value) (interface{}, error) {
	t := reflect.TypeOf(f)
	out := t.NumOut()
	switch out {
	case 0:
		return func(_ []reflect.Value) (interface{}, error) { return nil, nil }
	case 1:
		if t.Out(0).Name() != "error" {
			return func(vo []reflect.Value) (interface{}, error) {
				return vo[0].Interface(), nil
			}
		} else {
			return func(vo []reflect.Value) (interface{}, error) {
				if err, ok := vo[0].Interface().(error); ok {
					if e, ok := err.(*TypeError); ok {
						e.setFn(name, f)
						return nil, e
					}
					return nil, err
				}
				return nil, nil
			}
		}
	case 2:
		if t.Out(1).Name() != "error" {
			panic("Func: second return value of f can only be an error")
		}
		return func(vo []reflect.Value) (interface{}, error) {
			v := vo[0].Interface()
			if err, ok := vo[0].Interface().(error); ok {
				if e, ok := err.(*TypeError); ok {
					e.setFn(name, f)
					return nil, e
				}
				return v, err
			}
			return v, nil
		}
	default:
		panic("Func: f can only return at most two values")
	}
}

func funcVariadic(name string, f interface{}) func([]interface{}) (interface{}, error) {
	ret := funcReturn(name, f)
	t := reflect.TypeOf(f)
	in := t.NumIn()
	last := in - 1
	vf := reflect.ValueOf(f)
	return func(args []interface{}) (interface{}, error) {
		n := len(args)
		if n < last {
			return nil, newParamError(name, f)
		}
		vi := make([]reflect.Value, n)
		for i := 0; i < last; i++ {
			if !funcAccepts(t.In(i), reflect.TypeOf(args[i])) {
				return nil, &TypeError{name, f, args[i], []string{typeName(t.In(i))}}
			}
			vi[i] = reflect.ValueOf(args[i])
		}
		for i := last; i < n; i++ {
			if !funcAccepts(t.In(last).Elem(), reflect.TypeOf(args[i])) {
				return nil, &TypeError{name, f, args[i], []string{typeName(t.In(i))}}
			}
			vi[i] = reflect.ValueOf(args[i])
		}
		return ret(vf.Call(vi))
	}
}

func funcStandard(name string, f interface{}) func([]interface{}) (interface{}, error) {
	ret := funcReturn(name, f)
	t := reflect.TypeOf(f)
	in := t.NumIn()
	vi := make([]reflect.Value, in)
	vf := reflect.ValueOf(f)
	return func(args []interface{}) (interface{}, error) {
		if len(args) != in {
			return nil, newParamError(name, f)
		}
		for i := 0; i < in; i++ {
			if !funcAccepts(t.In(i), reflect.TypeOf(args[i])) {
				return nil, &TypeError{name, f, args[i], []string{typeName(t.In(i))}}
			}
			vi[i] = reflect.ValueOf(args[i])
		}
		return ret(vf.Call(vi))
	}
}

func funcAccepts(ft, it reflect.Type) bool {
	if ft.Kind() == reflect.Interface {
		return it.Implements(ft)
	}
	return ft == it
}

func Func(name string, f interface{}) func([]interface{}) (interface{}, error) {
	t := reflect.TypeOf(f)
	if t.Kind() != reflect.Func {
		panic("Func: f must be a function")
	}

	if t.IsVariadic() {
		return funcVariadic(name, f)
	}
	return funcStandard(name, f)
}

func Format(name string, v interface{}) string {
	var buf bytes.Buffer
	buf.WriteString(name)
	t := reflect.TypeOf(v)

	// If it's not of type function, then just print the type.
	// We differentiate from functions by only printing one colon.
	if t.Kind() != reflect.Func {
		buf.WriteString(" : ")
		buf.WriteString(typeName(t))
		return buf.String()
	}

	buf.WriteString(" :: ")
	in := t.NumIn()
	last := in - 1
	for i := 0; i < in; i++ {
		it := t.In(i)
		if i == last {
			if t.IsVariadic() {
				buf.WriteString(variadicName(it))
			} else {
				buf.WriteString(typeName(it))
			}
			buf.WriteString(" => ")
		} else {
			buf.WriteString(typeName(it))
			buf.WriteString(" -> ")
		}
	}

	out := t.NumOut()
	invalidRetval := func() {
		for i := 0; i < out; i++ {
			buf.WriteString(typeName(t.Out(i)))
			if i != out-1 {
				buf.WriteString(" -> ")
			}
		}
	}
	if out == 0 {
		buf.WriteString("()")
	} else if out == 1 {
		if t.Out(0).Name() == "error" {
			buf.WriteString("()")
		} else {
			buf.WriteString(typeName(t.Out(0)))
		}
	} else if out == 2 {
		if t.Out(0).Name() == "error" {
			// Invalid: first return value cannot be error
			invalidRetval()
		} else if t.Out(1).Name() != "error" {
			// Invalid: second return value can only be error
			invalidRetval()
		} else {
			buf.WriteString(typeName(t.Out(0)))
		}
	} else {
		// Invalid function:
		invalidRetval()
	}
	return buf.String()
}

func typeName(t reflect.Type) string {
	switch n := t.String(); n {
	case "interface {}":
		return "{}"
	case "[]interface {}":
		return "[]{}"
	case "[]uint8":
		return "[]byte"
	default:
		return n
	}
}

func variadicName(t reflect.Type) string {
	n := t.String()
	n = strings.Replace(n, "[]", "", 1)
	if n == "interface {}" {
		n = "{}"
	} else if n == "[]uint8" {
		n = "[]byte"
	}
	return "..." + n
}

type TypeError struct {
	Name string
	Fn   interface{}
	Got  interface{}
	Want []string
}

func NewTypeError(got interface{}, want []string) *TypeError {
	return &TypeError{Got: got, Want: want}
}

func (e *TypeError) setFn(name string, fn interface{}) {
	e.Name = name
	e.Fn = fn
}

func (e TypeError) Error() string {
	var buf bytes.Buffer
	buf.WriteString("Incorrect parameter type to function ")
	buf.WriteString(e.Name)
	buf.WriteString(".\n\n\tGot type ")
	buf.WriteString(typeName(reflect.TypeOf(e.Got)))
	buf.WriteString(" but need type ")
	switch len(e.Want) {
	case 1:
		buf.WriteString(e.Want[0])
	case 2:
		buf.WriteString(e.Want[0])
		buf.WriteString(" or ")
		buf.WriteString(e.Want[1])
	default:
		last := len(e.Want) - 1
		for i := 0; i < last; i++ {
			buf.WriteString(e.Want[i])
			buf.WriteString(", ")
		}
		buf.WriteString("or ")
		buf.WriteString(e.Want[last])
	}
	buf.WriteString(".\n\t")
	buf.WriteString(Format(e.Name, e.Fn))
	return buf.String()
}

func newParamError(name string, v interface{}) error {
	return fmt.Errorf("Incorrect number of parameters to function %s.\n\n\t%s.", name, Format(name, v))
}
