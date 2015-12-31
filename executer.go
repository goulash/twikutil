// Copyright (c) 2015, Ben Morgan. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package twikutil

import (
	"errors"
	"io/ioutil"

	"github.com/goulash/pre"
	past "github.com/goulash/pre/ast"

	"gopkg.in/twik.v1"
	"gopkg.in/twik.v1/ast"
)

var ErrFuncExists = errors.New("cannot set variable with name of existing function")

type LoaderFunc func(*twik.Scope) FuncMap

type Executer struct {
	PreProcessor *pre.Processor

	fset  *ast.FileSet
	scope *twik.Scope
	funcs map[string]bool
}

func New(loader LoaderFunc) *Executer {
	fset := twik.NewFileSet()
	s := twik.NewScope(fset)
	fns := loader(s)
	keys := make(map[string]bool)
	for k := range fns {
		keys[k] = true
	}
	fns.Export(s)
	return &Executer{
		fset:  fset,
		scope: s,
		funcs: keys,
	}
}

func (e *Executer) Scope() *twik.Scope { return e.scope }

// It is an error to use a key that has already been used as a function.
func (e *Executer) Set(key string, value interface{}) error {
	if e.funcs[key] {
		return errors.New("function with that name already exists")
	}
	_, err := e.scope.Get(key)
	if err == nil {
		return e.scope.Set(key, value)
	}
	return e.scope.Create(key, value)
}

// It is an error to get a key that has already been used as a function.
func (e *Executer) Get(key string) (interface{}, error) {
	if e.funcs[key] {
		return nil, errors.New("functions cannot be gotten")
	}
	return e.scope.Get(key)
}

func (e *Executer) Has(key string) bool {
	if e.funcs[key] {
		return false
	}
	v, err := e.scope.Get(key)
	return err == nil && v != nil
}

func (e *Executer) Create(key string, fn interface{}) error {
	err := e.scope.Create(key, Func(key, fn))
	if err != nil {
		return err
	}
	e.funcs[key] = true
	return nil
}

func (e *Executer) Override(key string, fn interface{}) error {
	if !e.funcs[key] {
		return errors.New("no function by that name exists")
	}
	return e.scope.Set(key, Func(key, fn))
}

func (e *Executer) Exec(file string) (s *twik.Scope, err error) {
	bs, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}
	return e.ExecString(file, string(bs))
}

func (e *Executer) ExecString(name, code string) (s *twik.Scope, err error) {
	// When the preprocessor is active, we need to convert any error messages
	// we get from twik so that they correspond to the correct file name, line
	// and column. This we do in the deferred function.
	var root past.Node
	if e.PreProcessor != nil {
		root, err = e.PreProcessor.ParseString(name, code)
		code = root.String()
		if err != nil {
			return nil, err
		}

		defer func() {
			e, ok := err.(*twik.Error)
			if ok {
				epi := e.PosInfo
				pi := root.OffsetLC(epi.Line, epi.Column)
				epi.Name = pi.Name
				epi.Line = pi.Line
				epi.Column = pi.Column
				err = e
			}
		}()
	}

	node, err := twik.ParseString(e.fset, name, code)
	if err != nil {
		return nil, err
	}
	_, err = e.scope.Eval(node)
	return e.scope, err
}
