package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"sort"
)

// mutation is one single-edit mutant of one non-test source file.
type mutation struct {
	file     string
	absPath  string
	function string
	operator string
	line     int
	col      int
	offset   int
	end      int
	// original and mutated are normalized one-line snippets used in the identity and the report.
	original string
	mutated  string
	// replacement is the source text that replaces src[offset:end]; an empty replacement deletes the range.
	replacement string
	src         []byte
	pkg         *pkgInfo
	identity    string
	outcome     outcome
	runErr      error
}

// generator collects mutants from one parsed file.
type generator struct {
	file     string
	absPath  string
	src      []byte
	fset     *token.FileSet
	function string
	types    map[string]bool
	mutants  []*mutation
}

// generatePackage returns every mutant of the package's files that the scope admits, ordered by file and position.
func generatePackage(pkg *pkgInfo, scope []string) ([]*mutation, error) {
	sources := make(map[string][]byte, len(pkg.files))
	for _, file := range pkg.files {
		src, err := os.ReadFile(filepath.Join(pkg.dir, filepath.Base(file)))
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", file, err)
		}
		sources[file] = src
	}
	types, err := collectTypeNames(sources)
	if err != nil {
		return nil, err
	}
	var mutants []*mutation
	for _, file := range pkg.files {
		if !inScope(scope, file) {
			continue
		}
		absPath := filepath.Join(pkg.dir, filepath.Base(file))
		generated, err := generateFile(file, absPath, sources[file], types)
		if err != nil {
			return nil, err
		}
		for _, mutant := range generated {
			mutant.pkg = pkg
		}
		mutants = append(mutants, generated...)
	}
	sort.SliceStable(mutants, func(left, right int) bool {
		if mutants[left].file != mutants[right].file {
			return mutants[left].file < mutants[right].file
		}

		return mutants[left].offset < mutants[right].offset
	})

	return mutants, nil
}

// collectTypeNames gathers the package's own type names so call deletion can leave type conversions alone.
func collectTypeNames(sources map[string][]byte) (map[string]bool, error) {
	names := make(map[string]bool)
	for file, src := range sources {
		parsed, err := parser.ParseFile(token.NewFileSet(), file, src, parser.SkipObjectResolution)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", file, err)
		}
		ast.Inspect(parsed, func(node ast.Node) bool {
			spec, ok := node.(*ast.TypeSpec)
			if ok {
				names[spec.Name.Name] = true
			}

			return true
		})
	}

	return names, nil
}

// generateFile parses one file and returns its mutants with identities assigned.
func generateFile(file, absPath string, src []byte, types map[string]bool) ([]*mutation, error) {
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, file, src, parser.SkipObjectResolution)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", file, err)
	}
	collector := &generator{
		file:     file,
		absPath:  absPath,
		src:      src,
		fset:     fset,
		function: "package",
		types:    types,
	}
	collector.collect(parsed)
	assignOrdinals(collector.mutants)

	return collector.mutants, nil
}

// collect walks the declarations, giving function bodies their function name and initializers the package name.
func (g *generator) collect(file *ast.File) {
	for _, decl := range file.Decls {
		switch typed := decl.(type) {
		case *ast.FuncDecl:
			g.collectFunction(typed)
		case *ast.GenDecl:
			g.collectValues(typed)
		}
	}
}

func (g *generator) collectFunction(decl *ast.FuncDecl) {
	if decl.Body == nil {
		return
	}
	g.function = functionName(decl)
	g.walk(decl.Body)
	g.function = "package"
}

func (g *generator) collectValues(decl *ast.GenDecl) {
	if decl.Tok != token.VAR && decl.Tok != token.CONST {
		return
	}
	for _, spec := range decl.Specs {
		value, ok := spec.(*ast.ValueSpec)
		if !ok {
			continue
		}
		for _, expr := range value.Values {
			g.walk(expr)
		}
	}
}

// functionName names a function for an identity, including a method's receiver.
func functionName(decl *ast.FuncDecl) string {
	if decl.Recv == nil || len(decl.Recv.List) == 0 {
		return decl.Name.Name
	}

	return "(" + types.ExprString(decl.Recv.List[0].Type) + ")." + decl.Name.Name
}

func (g *generator) walk(node ast.Node) {
	var parents []ast.Node
	ast.Inspect(node, func(child ast.Node) bool {
		if child == nil {
			parents = parents[:len(parents)-1]
			return true
		}
		g.inspect(child, parents)
		parents = append(parents, child)

		return true
	})
}

func (g *generator) inspect(node ast.Node, parents []ast.Node) {
	switch typed := node.(type) {
	case *ast.BlockStmt:
		g.deleteStatements(typed)
	case *ast.BinaryExpr:
		g.binaryMutations(typed)
	case *ast.AssignStmt:
		g.assignMutations(typed)
	case *ast.IncDecStmt:
		g.incrementMutations(typed)
	case *ast.UnaryExpr:
		g.negativeMutation(typed)
	case *ast.CallExpr:
		g.callMutations(typed, parents)
	case *ast.Ident:
		g.booleanMutation(typed)
	}
}

// expressionRoot returns the outermost enclosing expression, so a call deletion reports the expression it changes
// rather than only the deleted call node.
func expressionRoot(node ast.Expr, parents []ast.Node) ast.Node {
	root := ast.Node(node)
	for index := len(parents) - 1; index >= 0; index-- {
		parent, ok := parents[index].(ast.Expr)
		if !ok {
			break
		}
		root = parent
	}

	return root
}

func (g *generator) deleteStatements(block *ast.BlockStmt) {
	for _, statement := range block.List {
		if deletable(statement) {
			g.add(statement, statement, "statement deletion", "", "")
		}
	}
}

func deletable(statement ast.Stmt) bool {
	switch typed := statement.(type) {
	case *ast.AssignStmt:
		return typed.Tok != token.DEFINE
	case *ast.ExprStmt, *ast.IncDecStmt, *ast.SendStmt, *ast.DeferStmt, *ast.GoStmt:
		return true
	default:
		return false
	}
}

func (g *generator) binaryMutations(expr *ast.BinaryExpr) {
	if op, ok := logicalRewrite(expr.Op); ok {
		g.rewriteBinary(expr, op, "logical")
	}
	for _, rewrite := range comparisonRewrites(expr.Op) {
		g.rewriteBinary(expr, rewrite.op, rewrite.operator)
	}
	for _, rewrite := range arithmeticRewrites(expr.Op) {
		g.rewriteBinary(expr, rewrite.op, rewrite.operator)
	}
}

type binaryRewrite struct {
	op       token.Token
	operator string
}

func logicalRewrite(op token.Token) (token.Token, bool) {
	if op == token.LAND {
		return token.LOR, true
	}
	if op == token.LOR {
		return token.LAND, true
	}

	return token.ILLEGAL, false
}

// comparisonRewrites lists the boundary and negation mutants of a comparison operator.
func comparisonRewrites(op token.Token) []binaryRewrite {
	switch {
	case op == token.LSS:
		return []binaryRewrite{
			{op: token.LEQ, operator: "conditional boundary"},
			{op: token.GEQ, operator: "conditional negation"},
		}
	case op == token.LEQ:
		return []binaryRewrite{
			{op: token.LSS, operator: "conditional boundary"},
			{op: token.GTR, operator: "conditional negation"},
		}
	case op == token.GTR:
		return []binaryRewrite{
			{op: token.GEQ, operator: "conditional boundary"},
			{op: token.LEQ, operator: "conditional negation"},
		}
	case op == token.GEQ:
		return []binaryRewrite{
			{op: token.GTR, operator: "conditional boundary"},
			{op: token.LSS, operator: "conditional negation"},
		}
	case op == token.EQL:
		return []binaryRewrite{{op: token.NEQ, operator: "conditional negation"}}
	case op == token.NEQ:
		return []binaryRewrite{{op: token.EQL, operator: "conditional negation"}}
	}

	return nil
}

// arithmeticRewrites lists the arithmetic mutants of an operator.
func arithmeticRewrites(op token.Token) []binaryRewrite {
	switch {
	case op == token.ADD:
		return []binaryRewrite{{op: token.SUB, operator: "arithmetic"}}
	case op == token.SUB:
		return []binaryRewrite{{op: token.ADD, operator: "arithmetic"}}
	case op == token.MUL:
		return []binaryRewrite{{op: token.QUO, operator: "arithmetic"}}
	case op == token.QUO:
		return []binaryRewrite{{op: token.MUL, operator: "arithmetic"}}
	}

	return nil
}

func (g *generator) rewriteBinary(expr *ast.BinaryExpr, op token.Token, operator string) {
	clone := *expr
	clone.Op = op
	g.replaceNode(expr, &clone, operator, expr)
}

func (g *generator) assignMutations(stmt *ast.AssignStmt) {
	switch {
	case stmt.Tok == token.ADD_ASSIGN:
		g.rewriteAssign(stmt, token.SUB_ASSIGN)
	case stmt.Tok == token.SUB_ASSIGN:
		g.rewriteAssign(stmt, token.ADD_ASSIGN)
	case stmt.Tok == token.MUL_ASSIGN:
		g.rewriteAssign(stmt, token.QUO_ASSIGN)
	case stmt.Tok == token.QUO_ASSIGN:
		g.rewriteAssign(stmt, token.MUL_ASSIGN)
	}
}

func (g *generator) rewriteAssign(stmt *ast.AssignStmt, tok token.Token) {
	clone := *stmt
	clone.Tok = tok
	g.replaceNode(stmt, &clone, "assignment arithmetic", stmt)
}

func (g *generator) incrementMutations(stmt *ast.IncDecStmt) {
	switch {
	case stmt.Tok == token.INC:
		g.rewriteIncrement(stmt, token.DEC)
	case stmt.Tok == token.DEC:
		g.rewriteIncrement(stmt, token.INC)
	}
}

func (g *generator) rewriteIncrement(stmt *ast.IncDecStmt, tok token.Token) {
	clone := *stmt
	clone.Tok = tok
	g.replaceNode(stmt, &clone, "increment swap", stmt)
}

func (g *generator) negativeMutation(expr *ast.UnaryExpr) {
	if expr.Op != token.SUB {
		return
	}
	text, ok := g.printNode(expr.X)
	if !ok {
		return
	}
	g.add(expr, expr, "negative inversion", text, text)
}

func (g *generator) callMutations(call *ast.CallExpr, parents []ast.Node) {
	if len(call.Args) == 0 || g.isTypeName(call) {
		return
	}
	root := expressionRoot(call, parents)
	for _, arg := range call.Args {
		text, ok := g.printNode(arg)
		if !ok {
			continue
		}
		g.add(call, root, "call deletion", text, text)
	}
}

// isTypeName reports a conversion like Duration(x) or a builtin whose argument is not an expression.
func (g *generator) isTypeName(call *ast.CallExpr) bool {
	ident, ok := call.Fun.(*ast.Ident)
	if !ok {
		return false
	}

	return g.types[ident.Name] || isPredeclaredNonCall(ident.Name)
}

func (g *generator) booleanMutation(ident *ast.Ident) {
	switch ident.Name {
	case "true":
		g.add(ident, ident, "boolean literal", "false", "false")
	case "false":
		g.add(ident, ident, "boolean literal", "true", "true")
	}
}

// replaceNode records a mutation that replaces node with a printed copy of replacement.
func (g *generator) replaceNode(node, replacement ast.Node, operator string, root ast.Node) {
	text, ok := g.printNode(replacement)
	if !ok {
		return
	}
	g.add(node, root, operator, text, text)
}

// add records one mutation over node's range. root bounds the snippet shown in the identity, and an empty mutation
// means the node's range is deleted.
func (g *generator) add(node, root ast.Node, operator, replacement, mutated string) {
	nodeStart := g.fset.Position(node.Pos()).Offset
	nodeEnd := g.fset.Position(node.End()).Offset
	rootStart := g.fset.Position(root.Pos()).Offset
	rootEnd := g.fset.Position(root.End()).Offset
	if nodeStart < rootStart || nodeEnd > rootEnd || nodeStart >= nodeEnd {
		return
	}
	var mutatedText string
	switch {
	case mutated == "":
		mutatedText = "<deleted>"
	case root == node:
		mutatedText = mutated
	default:
		mutatedText = string(g.src[rootStart:nodeStart]) + replacement + string(g.src[nodeEnd:rootEnd])
	}
	g.mutants = append(g.mutants, &mutation{
		file:        g.file,
		absPath:     g.absPath,
		function:    g.function,
		operator:    operator,
		line:        g.fset.Position(node.Pos()).Line,
		col:         g.fset.Position(node.Pos()).Column,
		offset:      nodeStart,
		end:         nodeEnd,
		original:    normalizeSnippet(string(g.src[rootStart:rootEnd])),
		mutated:     normalizeSnippet(mutatedText),
		replacement: replacement,
		src:         g.src,
	})
}

func (g *generator) printNode(node ast.Node) (string, bool) {
	var buffer bytes.Buffer
	if err := printer.Fprint(&buffer, g.fset, node); err != nil {
		return "", false
	}

	return buffer.String(), true
}

// isPredeclaredNonCall reports builtins that cannot be treated as a called function.
func isPredeclaredNonCall(name string) bool {
	switch name {
	case "make", "new", "nil", "true", "false":
		return true
	}

	return isPredeclaredType(name)
}

func isPredeclaredType(name string) bool {
	switch name {
	case "any", "bool", "byte", "complex64", "complex128", "error", "float32", "float64",
		"int", "int8", "int16", "int32", "int64", "rune", "string",
		"uint", "uint8", "uint16", "uint32", "uint64", "uintptr":
		return true
	}

	return false
}
