package main

import (
	"bytes"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTestFile writes contents to name under the test's temporary directory and returns its path.
func writeTestFile(t *testing.T, name, contents string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}

	return path
}

// parseFixture parses source as one module file and returns its measured declarations.
func parseFixture(t *testing.T, source string) []function {
	t.Helper()

	functions, err := parseFunctions(sourceFile{
		path:         writeTestFile(t, "sample.go", source),
		coverageName: "example.com/sample/sample.go",
		displayName:  "sample.go",
	})
	if err != nil {
		t.Fatalf("parse sample: %v", err)
	}

	return functions
}

// findFunction returns the function with the given reported name.
func findFunction(t *testing.T, functions []function, name string) function {
	t.Helper()

	for _, fn := range functions {
		if fn.name == name {
			return fn
		}
	}
	t.Fatalf("no function named %q in %v", name, functions)

	return function{}
}

func TestComplexityCountsEveryBranch(t *testing.T) {
	t.Parallel()

	source := `package sample

func plain() {}

func branches(a, b bool) {
	if a && b {
	}
	for {
	}
	for range []int{1} {
	}
	switch {
	case a:
	case !a:
	default:
	}
	select {
	case <-make(chan int):
	default:
	}
	if a || b {
	}
	_ = func() {
		if a {
		}
	}
}
`
	functions := parseFixture(t, source)
	cases := map[string]int{
		"plain":    1,
		"branches": 11,
	}
	for name, complexity := range cases {
		if actual := findFunction(t, functions, name).complexity; actual != complexity {
			t.Errorf("%s complexity = %d, want %d", name, actual, complexity)
		}
	}
}

func TestFunctionNameIncludesReceiver(t *testing.T) {
	t.Parallel()

	source := `package sample

type Model[T any] struct{}

func plain() {}

func (model Model[T]) Value() {}

func (model *Model[T]) Pointer() {}
`
	functions := parseFixture(t, source)
	want := []string{"plain", "(Model[T]).Value", "(*Model[T]).Pointer"}
	if len(functions) != len(want) {
		t.Fatalf("found %d functions, want %d", len(functions), len(want))
	}
	for index, name := range want {
		if functions[index].name != name {
			t.Errorf("function %d named %q, want %q", index, functions[index].name, name)
		}
	}
}

func TestCoverageTreatsNoStatementsAsCovered(t *testing.T) {
	t.Parallel()

	if coverage := (function{}).coverage(); coverage != 1 {
		t.Fatalf("coverage of empty function = %v, want 1", coverage)
	}
	if coverage := (function{statements: 4, covered: 1}).coverage(); coverage != 0.25 {
		t.Fatalf("coverage of one of four statements = %v, want 0.25", coverage)
	}
}

func TestContainsAttributesBlocksToEnclosingFunction(t *testing.T) {
	t.Parallel()

	fn := function{coverageName: "example.com/sample/sample.go", startLine: 10, endLine: 20}
	cases := []struct {
		name string
		key  blockKey
		want bool
	}{
		{name: "first line", key: blockKey{file: fn.coverageName, startLine: 10, endLine: 10}, want: true},
		{name: "nested literal", key: blockKey{file: fn.coverageName, startLine: 12, endLine: 14}, want: true},
		{name: "last line", key: blockKey{file: fn.coverageName, startLine: 19, endLine: 20}, want: true},
		{name: "before", key: blockKey{file: fn.coverageName, startLine: 9, endLine: 9}, want: false},
		{name: "after", key: blockKey{file: fn.coverageName, startLine: 21, endLine: 21}, want: false},
		{
			name: "other file",
			key:  blockKey{file: "example.com/sample/other.go", startLine: 12, endLine: 12},
			want: false,
		},
	}
	for _, test := range cases {
		if actual := fn.contains(test.key); actual != test.want {
			t.Errorf("%s: contains = %t, want %t", test.name, actual, test.want)
		}
	}
}

func TestParseFunctionsRecordsLocation(t *testing.T) {
	t.Parallel()

	source := `package sample

type Model struct{}

func (model *Model) Update(step int) {
	if step > 0 {
		return
	}
}
`
	functions := parseFixture(t, source)
	if len(functions) != 1 {
		t.Fatalf("found %d functions, want 1", len(functions))
	}
	update := functions[0]
	if update.name != "(*Model).Update" || update.line != 5 || update.startLine != 5 || update.endLine != 9 {
		t.Fatalf("function = %+v, want (*Model).Update at lines 5-9", update)
	}
	if update.complexity != 2 {
		t.Fatalf("complexity = %d, want 2", update.complexity)
	}
	if update.displayName != "sample.go" || update.coverageName != "example.com/sample/sample.go" {
		t.Fatalf("function names = %q and %q", update.displayName, update.coverageName)
	}
}

func TestParseFunctionsRejectsInvalidSource(t *testing.T) {
	t.Parallel()

	_, err := parseFunctions(sourceFile{path: writeTestFile(t, "broken.go", "package sample\nfunc {")})
	if err == nil {
		t.Fatal("parseFunctions accepted invalid source")
	}
}

func TestParseBlock(t *testing.T) {
	t.Parallel()

	key, statements, count, err := parseBlock("example.com/sample/main.go:3.2,5.9 4 7")
	if err != nil {
		t.Fatalf("parseBlock: %v", err)
	}
	want := blockKey{file: "example.com/sample/main.go", startLine: 3, startCol: 2, endLine: 5, endCol: 9}
	if key != want || statements != 4 || count != 7 {
		t.Fatalf("parseBlock = %+v, %d, %d; want %+v, 4, 7", key, statements, count, want)
	}

	for _, line := range []string{
		"no-colon",
		"example.com/sample/main.go:3.2,5.9",
		"example.com/sample/main.go:three.2,5.9 4 7",
		"example.com/sample/main.go:3.2,5.9 four 7",
	} {
		if _, _, _, err := parseBlock(line); err == nil {
			t.Errorf("parseBlock(%q) accepted a malformed block", line)
		}
	}
}

func TestCoverageProfileReadsBlocks(t *testing.T) {
	t.Parallel()

	path := writeTestFile(t, "coverage.out", "mode: set\n\nexample.com/sample/main.go:3.2,4.1 2 1\n")
	profile := newCoverageProfile()
	if err := profile.read(path); err != nil {
		t.Fatalf("read: %v", err)
	}
	key := blockKey{file: "example.com/sample/main.go", startLine: 3, startCol: 2, endLine: 4, endCol: 1}
	if profile.statements[key] != 2 || !profile.covered[key] {
		t.Fatalf("profile = %+v, want block with 2 statements covered", profile)
	}
}

func TestCoverageProfileReadRejectsBadInput(t *testing.T) {
	t.Parallel()

	profile := newCoverageProfile()
	if err := profile.read(filepath.Join(t.TempDir(), "missing.out")); err == nil {
		t.Fatal("read accepted a missing profile")
	}
	if err := profile.read(writeTestFile(t, "bad.out", "mode: set\nnonsense\n")); err == nil {
		t.Fatal("read accepted a malformed profile")
	}
}

func TestLoadCoverageMergesProfiles(t *testing.T) {
	t.Parallel()

	first := writeTestFile(t, "unit.out", "mode: set\n"+
		"example.com/sample/main.go:3.2,3.3 2 0\n"+
		"example.com/sample/main.go:4.2,4.3 1 1\n")
	second := writeTestFile(t, "e2e.out", "mode: atomic\n"+
		"example.com/sample/main.go:3.2,3.3 2 3\n"+
		"example.com/sample/main.go:4.2,4.3 1 0\n"+
		"example.com/sample/main.go:5.2,5.3 5 0\n")
	profile, err := loadCoverage([]string{first, second})
	if err != nil {
		t.Fatalf("loadCoverage: %v", err)
	}
	shared := blockKey{file: "example.com/sample/main.go", startLine: 3, startCol: 2, endLine: 3, endCol: 3}
	if profile.statements[shared] != 2 || !profile.covered[shared] {
		t.Fatalf("shared block = %d statements, covered %t; want 2, true",
			profile.statements[shared], profile.covered[shared])
	}
	if !profile.covered[blockKey{file: "example.com/sample/main.go", startLine: 4, startCol: 2, endLine: 4, endCol: 3}] {
		t.Fatal("second profile's uncovered count cleared the first profile's coverage")
	}
	late := blockKey{file: "example.com/sample/main.go", startLine: 5, startCol: 2, endLine: 5, endCol: 3}
	if profile.statements[late] != 5 {
		t.Fatal("profile without the block was not merged")
	}
	if _, err := loadCoverage([]string{filepath.Join(t.TempDir(), "missing.out")}); err == nil {
		t.Fatal("loadCoverage accepted a missing profile")
	}
}

func TestAttributeSumsStatementsInsideFunctions(t *testing.T) {
	t.Parallel()

	functions := []function{
		{coverageName: "example.com/sample/main.go", startLine: 10, endLine: 20},
		{coverageName: "example.com/sample/main.go", startLine: 30, endLine: 40},
	}
	profile := newCoverageProfile()
	profile.add(blockKey{file: functions[0].coverageName, startLine: 10, endLine: 11}, 2, 1)
	profile.add(blockKey{file: functions[0].coverageName, startLine: 15, endLine: 16}, 3, 0)
	profile.add(blockKey{file: functions[0].coverageName, startLine: 25, endLine: 26}, 5, 1)
	profile.add(blockKey{file: functions[0].coverageName, startLine: 31, endLine: 32}, 4, 1)
	attribute(functions, profile)

	if functions[0].statements != 5 || functions[0].covered != 2 || functions[0].coverage() != 0.4 {
		t.Fatalf("first function = %+v, want 2 of 5 statements covered", functions[0])
	}
	if functions[1].statements != 4 || functions[1].covered != 4 || functions[1].coverage() != 1 {
		t.Fatalf("second function = %+v, want all 4 statements covered", functions[1])
	}
}

func TestMeasureMergesSourceAndProfiles(t *testing.T) {
	t.Parallel()

	source := "package sample\n\nfunc covered() int { return 1 }\n\n" +
		"func uncovered() int {\n\tif true {\n\t\treturn 1\n\t}\n\n\treturn 2\n}\n"
	path := writeTestFile(t, "sample.go", source)
	profilePath := writeTestFile(t, "coverage.out", "mode: set\n"+
		"example.com/sample/sample.go:3.20,3.28 1 1\n"+
		"example.com/sample/sample.go:6.2,6.11 1 0\n"+
		"example.com/sample/sample.go:7.3,7.10 1 0\n"+
		"example.com/sample/sample.go:10.2,10.9 1 0\n")
	functions, err := measure(configuration{maximum: 12, profiles: []string{profilePath}}, []sourceFile{{
		path:         path,
		coverageName: "example.com/sample/sample.go",
		displayName:  "sample.go",
	}})
	if err != nil {
		t.Fatalf("measure: %v", err)
	}
	if len(functions) != 2 {
		t.Fatalf("measured %d functions, want 2", len(functions))
	}
	if coverage := findFunction(t, functions, "covered").coverage(); coverage != 1 {
		t.Fatalf("covered function coverage = %v, want 1", coverage)
	}
	if coverage := findFunction(t, functions, "uncovered").coverage(); coverage != 0 {
		t.Fatalf("uncovered function coverage = %v, want 0", coverage)
	}
}

func TestMeasureReportsBadInput(t *testing.T) {
	t.Parallel()

	config := configuration{maximum: 12, profiles: []string{filepath.Join(t.TempDir(), "missing.out")}}
	if _, err := measure(config, nil); err == nil {
		t.Fatal("measure accepted a missing profile")
	}
	config.profiles = []string{writeTestFile(t, "coverage.out", "mode: set\n")}
	broken := sourceFile{path: writeTestFile(t, "broken.go", "package sample\nfunc {")}
	if _, err := measure(config, []sourceFile{broken}); err == nil {
		t.Fatal("measure accepted an unparseable source file")
	}
}

func TestParsePackageList(t *testing.T) {
	t.Parallel()

	output := []byte("example.com/mod|/work/mod|main.go helper.go\n" +
		"example.com/mod/internal/core|/work/mod/internal/core|a.go b_test.go c.go\n")
	files, err := parsePackageList(output, "/work/mod")
	if err != nil {
		t.Fatalf("parsePackageList: %v", err)
	}
	want := []sourceFile{
		{path: "/work/mod/main.go", coverageName: "example.com/mod/main.go", displayName: "main.go"},
		{path: "/work/mod/helper.go", coverageName: "example.com/mod/helper.go", displayName: "helper.go"},
		{
			path:         "/work/mod/internal/core/a.go",
			coverageName: "example.com/mod/internal/core/a.go",
			displayName:  "internal/core/a.go",
		},
		{
			path:         "/work/mod/internal/core/c.go",
			coverageName: "example.com/mod/internal/core/c.go",
			displayName:  "internal/core/c.go",
		},
	}
	if len(files) != len(want) {
		t.Fatalf("parsed %d files, want %d: %+v", len(files), len(want), files)
	}
	for index := range want {
		if files[index] != want[index] {
			t.Errorf("file %d = %+v, want %+v", index, files[index], want[index])
		}
	}
	if _, err := parsePackageList([]byte("malformed\n"), "/work/mod"); err == nil {
		t.Fatal("parsePackageList accepted a malformed line")
	}
}

func TestDisplayPath(t *testing.T) {
	t.Parallel()

	if actual := displayPath("/work/mod", "/work/mod/internal/core/a.go"); actual != "internal/core/a.go" {
		t.Fatalf("displayPath = %q, want %q", actual, "internal/core/a.go")
	}
}

func TestParseArguments(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		arguments []string
		config    configuration
		wantErr   bool
	}{
		{name: "no profiles", arguments: nil, wantErr: true},
		{
			name:      "default",
			arguments: []string{"unit.out"},
			config:    configuration{maximum: 12, profiles: []string{"unit.out"}},
		},
		{
			name:      "separate max",
			arguments: []string{"-max", "20", "unit.out", "e2e.out"},
			config:    configuration{maximum: 20, profiles: []string{"unit.out", "e2e.out"}},
		},
		{
			name:      "joined max",
			arguments: []string{"-max=8.5", "unit.out"},
			config:    configuration{maximum: 8.5, profiles: []string{"unit.out"}},
		},
		{name: "missing max", arguments: []string{"-max"}, wantErr: true},
		{name: "empty max", arguments: []string{"-max=", "unit.out"}, wantErr: true},
		{name: "invalid max", arguments: []string{"-max", "many", "unit.out"}, wantErr: true},
		{name: "unknown flag", arguments: []string{"-bogus", "unit.out"}, wantErr: true},
	}
	for _, test := range cases {
		config, err := parseArguments(test.arguments)
		if test.wantErr {
			if err == nil {
				t.Errorf("%s: parseArguments accepted %v", test.name, test.arguments)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: parseArguments: %v", test.name, err)
			continue
		}
		if config.maximum != test.config.maximum ||
			strings.Join(config.profiles, ",") != strings.Join(test.config.profiles, ",") {
			t.Errorf("%s: configuration = %+v, want %+v", test.name, config, test.config)
		}
	}
}

func TestListPackagesFindsTheChecker(t *testing.T) {
	t.Parallel()

	files, err := listPackages()
	if err != nil {
		t.Fatalf("listPackages: %v", err)
	}
	found := false
	for _, file := range files {
		if strings.HasSuffix(file.coverageName, "/internal/tools/crap/crap.go") {
			found = true
		}
		if strings.HasSuffix(file.coverageName, "_test.go") {
			t.Fatalf("listPackages returned test file %q", file.coverageName)
		}
	}
	if !found {
		t.Fatalf("listPackages did not find the checker among %d files", len(files))
	}
}

func TestExecuteReportsFunctions(t *testing.T) {
	t.Parallel()

	source := writeTestFile(t, "sample.go", "package sample\n\nfunc doomed(step int) {\n\tif step > 0 {\n\t}\n"+
		"\tif step > 1 {\n\t}\n\tif step > 2 {\n\t}\n}\n")
	profile := writeTestFile(t, "coverage.out", "mode: set\nexample.com/sample/sample.go:4.2,4.12 1 0\n")
	file := sourceFile{path: source, coverageName: "example.com/sample/sample.go", displayName: "sample.go"}
	var output, errorOutput bytes.Buffer
	status := execute([]string{profile}, &output, &errorOutput, func() ([]sourceFile, error) {
		return []sourceFile{file}, nil
	})
	if status != exitFailure {
		t.Fatalf("execute status = %d, want %d", status, exitFailure)
	}
	if !strings.Contains(output.String(), "sample.go:3: doomed complexity=4 coverage=0.0% crap=20.0") {
		t.Fatalf("execute output = %q", output.String())
	}
	if errorOutput.Len() != 0 {
		t.Fatalf("execute wrote to standard error: %q", errorOutput.String())
	}
}

func TestExecuteRejectsUsageAndFailures(t *testing.T) {
	t.Parallel()

	var output, errorOutput bytes.Buffer
	if status := execute(nil, &output, &errorOutput, func() ([]sourceFile, error) {
		t.Fatal("listed packages for invalid arguments")
		return nil, nil
	}); status != exitUsage {
		t.Fatalf("status = %d, want %d", status, exitUsage)
	}
	if !strings.Contains(errorOutput.String(), "usage: crap") {
		t.Fatalf("usage error output = %q", errorOutput.String())
	}

	errorOutput.Reset()
	brokenList := errors.New("no packages")
	if status := execute([]string{"unit.out"}, &output, &errorOutput, func() ([]sourceFile, error) {
		return nil, brokenList
	}); status != exitFailure {
		t.Fatalf("status = %d, want %d", status, exitFailure)
	}
	if !strings.Contains(errorOutput.String(), brokenList.Error()) {
		t.Fatalf("list error output = %q", errorOutput.String())
	}

	errorOutput.Reset()
	if status := execute([]string{filepath.Join(t.TempDir(), "missing.out")}, &output, &errorOutput,
		func() ([]sourceFile, error) { return nil, nil }); status != exitFailure {
		t.Fatalf("status = %d, want %d", status, exitFailure)
	}
}

func TestCrapFormula(t *testing.T) {
	t.Parallel()

	cases := []struct {
		complexity int
		coverage   float64
		want       float64
	}{
		{complexity: 3, coverage: 0, want: 12},
		{complexity: 4, coverage: 0, want: 20},
		{complexity: 2, coverage: 0, want: 6},
		{complexity: 5, coverage: 1, want: 5},
		{complexity: 4, coverage: 1, want: 4},
	}
	for _, test := range cases {
		if actual := crap(test.complexity, test.coverage); actual != test.want {
			t.Errorf("crap(%d, %g) = %v, want %v", test.complexity, test.coverage, actual, test.want)
		}
	}
	if actual := crap(5, 0.8); math.Abs(actual-5.2) > 1e-9 {
		t.Errorf("crap(5, 0.8) = %v, want 5.2", actual)
	}
}

func TestReportPrintsOffendersSorted(t *testing.T) {
	t.Parallel()

	functions := []function{
		{displayName: "main.go", line: 3, name: "safe", complexity: 3, statements: 1},
		{displayName: "main.go", line: 10, name: "(*Model).Update", complexity: 4, statements: 1},
		{displayName: "main.go", line: 20, name: "worse", complexity: 6, statements: 1},
	}
	var output bytes.Buffer
	if status := report(&output, functions, 12); status != exitFailure {
		t.Fatalf("report status = %d, want %d", status, exitFailure)
	}
	want := "main.go:20: worse complexity=6 coverage=0.0% crap=42.0\n" +
		"main.go:10: (*Model).Update complexity=4 coverage=0.0% crap=20.0\n"
	if output.String() != want {
		t.Fatalf("report output = %q, want %q", output.String(), want)
	}
}

func TestReportPassesAtCeiling(t *testing.T) {
	t.Parallel()

	functions := []function{
		{displayName: "main.go", line: 3, name: "safe", complexity: 3, statements: 1},
		{displayName: "main.go", line: 8, name: "held", complexity: 4, statements: 2, covered: 2, coverageName: "x"},
	}
	var output bytes.Buffer
	if status := report(&output, functions, 12); status != exitSuccess {
		t.Fatalf("report status = %d, want %d", status, exitSuccess)
	}
	if output.String() != "checked 2 functions; highest crap 12.0 in main.go:3 (safe)\n" {
		t.Fatalf("report output = %q", output.String())
	}
}

func TestReportHandlesNoFunctions(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	if status := report(&output, nil, 12); status != exitSuccess {
		t.Fatalf("report status = %d, want %d", status, exitSuccess)
	}
	if output.String() != "checked 0 functions\n" {
		t.Fatalf("report output = %q", output.String())
	}
}

func TestCompareFunctionsBreaksTies(t *testing.T) {
	t.Parallel()

	base := function{displayName: "a.go", line: 5, name: "first", complexity: 2}
	other := function{displayName: "b.go", line: 5, name: "second", complexity: 2}
	if compareFunctions(base, other) != -1 || compareFunctions(other, base) != 1 {
		t.Fatal("compareFunctions did not break a display name tie")
	}
	base = function{displayName: "a.go", line: 5, name: "first", complexity: 2}
	other = function{displayName: "a.go", line: 9, name: "second", complexity: 2}
	if compareFunctions(base, other) != -1 || compareFunctions(other, base) != 1 {
		t.Fatal("compareFunctions did not break a line tie")
	}
	base = function{displayName: "a.go", line: 9, name: "first", complexity: 2}
	other = function{displayName: "a.go", line: 9, name: "second", complexity: 2}
	if compareFunctions(base, other) != -1 || compareFunctions(other, base) != 1 {
		t.Fatal("compareFunctions did not break a name tie")
	}
	if compareFunctions(base, base) != 0 {
		t.Fatal("compareFunctions did not treat equal functions as equal")
	}
	strong := function{displayName: "a.go", line: 5, name: "strong", complexity: 6}
	weak := function{displayName: "a.go", line: 5, name: "weak", complexity: 4}
	if compareFunctions(strong, weak) != -1 || compareFunctions(weak, strong) != 1 {
		t.Fatal("compareFunctions did not sort by score descending")
	}
}
