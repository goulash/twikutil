// Copyright (c) 2015, Ben Morgan. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package key

import "reflect"

func IsTyper(v interface{}) bool {
	switch v.(type) {
	case reflect.Type, Typer:
		return true
	default:
		return false
	}
}

type Typer interface {
	String() string
	Coerce(v interface{}) (interface{}, error)
}

type typer struct {
	s  string
	fn func(v interface{}) (interface{}, error)
}

func (t typer) String() string { return t.s }
func (t typer) Coerce(v interface{}) (interface{}, error) {
	v, err := t.fn(v)
	if err == ErrCatch {
		return v, NewTypeError("", reflect.TypeOf(v), t.s)
	}
	return v, err
}

func TyperFunc(id string, f func(v interface{}) (interface{}, error)) Typer {
	return &typer{
		s:  id,
		fn: f,
	}
}

// TypeFunc checks that the value v is of the correct type.
// When the type is correct, it returns (v, nil).
// It may coerce it into the correct type and return (v, nil),
// or it may return one of the below errors in (nil, err).
type TypeFunc func(v interface{}) (interface{}, error)

var (
	// Base types:
	Anything = reflect.TypeOf((*interface{})(nil)).Elem()
	Int      = reflect.TypeOf(int(0))
	Int16    = reflect.TypeOf(int16(0))
	Int32    = reflect.TypeOf(int32(0))
	Int64    = reflect.TypeOf(int64(0))
	Float32  = reflect.TypeOf(float32(0))
	Float64  = reflect.TypeOf(float64(0))
	Rune     = reflect.TypeOf('0')
	String   = reflect.TypeOf("")
	Bool     = reflect.TypeOf(false)

	// Float coerces float32 to float64.
	Float = TyperFunc("float", func(v interface{}) (interface{}, error) {
		switch t := v.(type) {
		case float64:
			return t, nil
		case float32:
			return float64(t), nil
		default:
			return nil, ErrCatch
		}
	})
	// Integer coerces int16, int32, int64 to int.
	Integer = TyperFunc("integer", func(v interface{}) (interface{}, error) {
		switch t := v.(type) {
		case int:
			return t, nil
		case int64:
			return int(t), nil
		case int32:
			return int(t), nil
		case int16:
			return int(t), nil
		default:
			return nil, ErrCatch
		}
	})
)
