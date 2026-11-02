package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
)

// sourceFile is one compiled Go file in the module.
type sourceFile struct {
	path         string // file system path to read
	coverageName string // import path plus file name, as coverage profiles spell it
	displayName  string // short path used in the report
}

// function is one function declaration with its complexity and measured statement coverage.
type function struct {
	displayName  string
	coverageName string
	line         int
	name         string
	complexity   int
	startLine    int
	endLine      int
	statements   int
	covered      int
}

// coverage returns the function's statement coverage as a fraction, treating a statement-free function as covered.
func (fn function) coverage() float64 {
	if fn.statements == 0 {
		return 1
	}

	return float64(fn.covered) / float64(fn.statements)
}

// contains reports whether a profile block lies inside the function's body. Blocks in function literals lie inside
// the enclosing declaration and therefore count for it.
func (fn function) contains(key blockKey) bool {
	return key.file == fn.coverageName && key.startLine >= fn.startLine && key.endLine <= fn.endLine
}

// parseFunctions parses one file and returns its function declarations.
func parseFunctions(file sourceFile) ([]function, error) {
	set := token.NewFileSet()
	parsed, err := parser.ParseFile(set, file.path, nil, 0)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", file.path, err)
	}
	functions := make([]function, 0, len(parsed.Decls))
	for _, declaration := range parsed.Decls {
		declared, ok := declaration.(*ast.FuncDecl)
		if !ok || declared.Body == nil {
			continue
		}
		functions = append(functions, function{
			displayName:  file.displayName,
			coverageName: file.coverageName,
			line:         set.Position(declared.Pos()).Line,
			name:         functionName(declared),
			complexity:   complexity(declared.Body),
			startLine:    set.Position(declared.Body.Lbrace).Line,
			endLine:      set.Position(declared.Body.Rbrace).Line,
		})
	}

	return functions, nil
}

// functionName reports the declaration's name, with its receiver for a method.
func functionName(declared *ast.FuncDecl) string {
	if declared.Recv == nil || len(declared.Recv.List) == 0 {
		return declared.Name.Name
	}

	return "(" + types.ExprString(declared.Recv.List[0].Type) + ")." + declared.Name.Name
}

// complexity counts cyclomatic complexity gocyclo-style: one plus every branch point in the body.
func complexity(body ast.Node) int {
	score := 1
	ast.Inspect(body, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt:
			score++
		case *ast.CaseClause:
			if node.List != nil {
				score++
			}
		case *ast.CommClause:
			if node.Comm != nil {
				score++
			}
		case *ast.BinaryExpr:
			if node.Op == token.LAND || node.Op == token.LOR {
				score++
			}
		}

		return true
	})

	return score
}

// attribute sums the profile statements that fall inside each function.
func attribute(functions []function, profile coverageProfile) {
	for index := range functions {
		for key, statements := range profile.statements {
			if !functions[index].contains(key) {
				continue
			}
			functions[index].statements += statements
			if profile.covered[key] {
				functions[index].covered += statements
			}
		}
	}
}
