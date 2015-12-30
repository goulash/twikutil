// Copyright (c) 2015, Ben Morgan. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package twikutil_test

import (
	"fmt"
	"io/ioutil"
	"testing"
	"time"

	"github.com/cassava/dromi/lisp/twikutil"
)

type formatTest struct {
	Value  interface{}
	Format string
}

func testfn1(args ...interface{}) []interface{} { return nil }

func TestFormat(z *testing.T) {
	name := "a"
	tests := []formatTest{
		{true, "a : bool"},
		{int(10), "a : int"},
		{int64(10), "a : int64"},
		{100, "a : int"},
		{100.0, "a : float64"},
		{"hello", "a : string"},
		{func() {}, "a :: ()"},
		{fmt.Printf, "a :: string -> ...{} => int"},
		{time.Now, "a :: time.Time"},
		{ioutil.ReadAll, "a :: io.Reader => []byte"},
		{ioutil.NopCloser, "a :: io.Reader => io.ReadCloser"},
		{ioutil.WriteFile, "a :: string -> []byte -> os.FileMode => ()"},
		{ioutil.ReadFile, "a :: string => []byte"},
		{testfn1, "a :: ...{} => []{}"},
	}

	for _, t := range tests {
		if f := twikutil.Format(name, t.Value); f != t.Format {
			z.Errorf("Format(%#v) = %q; want %q", t.Value, f, t.Format)
		}
	}
}

func TestFuncPrintf(z *testing.T) {
	// This should just compile and run without any panics.
	_, _ = twikutil.Func("printf", fmt.Fprintf)([]interface{}{ioutil.Discard, "%s %s!\n", "Hello", "world"})
}
