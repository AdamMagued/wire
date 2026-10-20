// Copyright 2024 The Wire Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package wire

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"reflect"
	"testing"
)

func TestCopyAST_IndexListExpr(t *testing.T) {
	const src = `package example

type Pair[T1, T2 any] struct {
	first  T1
	second T2
}

type StringIntPair Pair[string, int]

var DefaultPair Pair[string, int] = Pair[string, int]{
	first:  "init",
	second: 100,
}

func NewPair() *Pair[string, int] {
	return &Pair[string, int]{
		first:  "hello",
		second: 42,
	}
}

func MakePair[A, B any](a A, b B) Pair[A, B] {
	return Pair[A, B]{
		first:  a,
		second: b,
	}
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "example.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parser.ParseFile failed: %v", err)
	}

	var hasIndexListExpr bool
	ast.Inspect(file, func(n ast.Node) bool {
		if _, ok := n.(*ast.IndexListExpr); ok {
			hasIndexListExpr = true
			return false
		}
		return true
	})
	if !hasIndexListExpr {
		t.Fatal("test source does not contain *ast.IndexListExpr")
	}

	for i, decl := range file.Decls {
		copied := copyAST(decl)
		if copied == nil {
			t.Fatalf("copyAST(decl[%d]) returned nil", i)
		}
		if copied == decl {
			t.Fatalf("copyAST(decl[%d]) returned identical pointer, expected deep copy", i)
		}

		var originalBuf bytes.Buffer
		if err := format.Node(&originalBuf, fset, decl); err != nil {
			t.Fatalf("format.Node(original[%d]) failed: %v", i, err)
		}

		var copiedBuf bytes.Buffer
		if err := format.Node(&copiedBuf, fset, copied); err != nil {
			t.Fatalf("format.Node(copied[%d]) failed: %v", i, err)
		}

		if originalBuf.String() != copiedBuf.String() {
			t.Errorf("decl[%d] formatted AST mismatch:\n--- Original ---\n%s\n--- Copied ---\n%s", i, originalBuf.String(), copiedBuf.String())
		}
	}
}

func TestCopyAST_IndexListExpr_DeepCopy(t *testing.T) {
	identX := &ast.Ident{Name: "Map", NamePos: 1}
	identK := &ast.Ident{Name: "string", NamePos: 5}
	identV := &ast.Ident{Name: "int", NamePos: 12}

	orig := &ast.IndexListExpr{
		X:       identX,
		Lbrack:  4,
		Indices: []ast.Expr{identK, identV},
		Rbrack:  15,
	}

	copiedNode := copyAST(orig)
	copied, ok := copiedNode.(*ast.IndexListExpr)
	if !ok {
		t.Fatalf("copyAST returned %T, want *ast.IndexListExpr", copiedNode)
	}

	if copied == orig {
		t.Fatal("copyAST returned the original *ast.IndexListExpr pointer")
	}
	if copied.Lbrack != orig.Lbrack || copied.Rbrack != orig.Rbrack {
		t.Errorf("brackets pos mismatch: got [%v, %v], want [%v, %v]", copied.Lbrack, copied.Rbrack, orig.Lbrack, orig.Rbrack)
	}

	// Idents should preserve identity according to copyAST design.
	if copied.X != orig.X {
		t.Errorf("X ident changed pointer: got %p, want %p", copied.X, orig.X)
	}
	if len(copied.Indices) != len(orig.Indices) {
		t.Fatalf("Indices length mismatch: got %d, want %d", len(copied.Indices), len(orig.Indices))
	}
	for i := range orig.Indices {
		if copied.Indices[i] != orig.Indices[i] {
			t.Errorf("Indices[%d] ident changed pointer: got %p, want %p", i, copied.Indices[i], orig.Indices[i])
		}
	}

	// Verify slice backing array is distinct.
	if reflect.ValueOf(copied.Indices).Pointer() == reflect.ValueOf(orig.Indices).Pointer() {
		t.Error("Indices slice backing array was not copied")
	}
}

func TestCopyAST_IndexListExpr_Nested(t *testing.T) {
	// Represents: Outer[Inner[A, B], C]
	identOuter := &ast.Ident{Name: "Outer"}
	identInner := &ast.Ident{Name: "Inner"}
	identA := &ast.Ident{Name: "A"}
	identB := &ast.Ident{Name: "B"}
	identC := &ast.Ident{Name: "C"}

	inner := &ast.IndexListExpr{
		X:       identInner,
		Indices: []ast.Expr{identA, identB},
	}
	outer := &ast.IndexListExpr{
		X:       identOuter,
		Indices: []ast.Expr{inner, identC},
	}

	copiedNode := copyAST(outer)
	copiedOuter, ok := copiedNode.(*ast.IndexListExpr)
	if !ok {
		t.Fatalf("copyAST returned %T, want *ast.IndexListExpr", copiedNode)
	}
	if copiedOuter == outer {
		t.Fatal("outer *ast.IndexListExpr pointer was not copied")
	}

	copiedInner, ok := copiedOuter.Indices[0].(*ast.IndexListExpr)
	if !ok {
		t.Fatalf("copiedOuter.Indices[0] is %T, want *ast.IndexListExpr", copiedOuter.Indices[0])
	}
	if copiedInner == inner {
		t.Fatal("nested *ast.IndexListExpr pointer was not copied")
	}

	if copiedInner.Indices[0] != identA || copiedInner.Indices[1] != identB {
		t.Error("nested Idents did not preserve pointer identity")
	}
	if copiedOuter.Indices[1] != identC {
		t.Error("identC did not preserve pointer identity")
	}
}
