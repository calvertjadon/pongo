package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeFile(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	return os.WriteFile(path, []byte(content), 0o644)
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := writeFile(path, content); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func appendToFile(path, content string) error {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	_, err = file.WriteString(content)

	return err
}

func TestParseCoverLine(t *testing.T) {
	t.Parallel()
	name, block, err := parseCoverLine("example.com/demo/internal/core/a.go:4.2,6.1 2 1")
	if err != nil {
		t.Fatalf("parseCoverLine: %v", err)
	}
	if name != "example.com/demo/internal/core/a.go" {
		t.Fatalf("name = %q", name)
	}
	if block != (coverBlock{startLine: 4, startCol: 2, endLine: 6, endCol: 1, count: 1}) {
		t.Fatalf("block = %+v", block)
	}
}

func TestParseCoverLineRejectsMalformedInput(t *testing.T) {
	t.Parallel()
	cases := []string{
		"example.com/a.go 1 1",
		"example.com/a.go",
		"example.com/a.go:4.2 2 1",
		"example.com/a.go:x.2,6.1 2 1",
		"example.com/a.go:4.2,y.1 2 1",
		"example.com/a.go:4.2,6.1 2 z",
	}
	for _, input := range cases {
		if _, _, err := parseCoverLine(input); err == nil {
			t.Fatalf("parseCoverLine(%q) accepted malformed input", input)
		}
	}
}

func TestParseCoverageTrimsModulePrefix(t *testing.T) {
	t.Parallel()
	document := "mode: set\n" +
		"example.com/demo/internal/core/a.go:4.2,6.1 2 1\n" +
		"other.com/pkg/b.go:1.1,2.1 1 0\n"
	blocks, err := parseCoverage(strings.NewReader(document), "example.com/demo")
	if err != nil {
		t.Fatalf("parseCoverage: %v", err)
	}
	if len(blocks["internal/core/a.go"]) != 1 {
		t.Fatalf("missing core blocks in %v", blocks)
	}
	if len(blocks["other.com/pkg/b.go"]) != 1 {
		t.Fatalf("missing foreign blocks in %v", blocks)
	}
}

func TestParseCoverageReportsReadAndParseErrors(t *testing.T) {
	t.Parallel()
	if _, err := parseCoverage(failingReader{}, "example.com/demo"); err == nil {
		t.Fatal("parseCoverage accepted a failing reader")
	}
	if _, err := parseCoverage(strings.NewReader("mode: set\nbroken\n"), "example.com/demo"); err == nil {
		t.Fatal("parseCoverage accepted a malformed line")
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) {
	return 0, errors.New("read failure")
}

func TestProvenUnexecutedPositions(t *testing.T) {
	t.Parallel()
	blocks := []coverBlock{
		{startLine: 4, startCol: 2, endLine: 6, endCol: 1, count: 1},
		{startLine: 8, startCol: 2, endLine: 8, endCol: 11, count: 0},
		{startLine: 10, startCol: 2, endLine: 12, endCol: 1, count: 0},
		{startLine: 11, startCol: 2, endLine: 13, endCol: 1, count: 1},
	}
	cases := []struct {
		line, col int
		want      bool
	}{
		{4, 2, false},
		{5, 10, false},
		{6, 1, false},
		{6, 2, false},
		{3, 5, false},
		{8, 5, true},
		{10, 5, true},
		{11, 5, false},
	}
	for _, testCase := range cases {
		if got := provenUnexecuted(blocks, testCase.line, testCase.col); got != testCase.want {
			t.Fatalf("provenUnexecuted(%d, %d) = %t, want %t", testCase.line, testCase.col, got, testCase.want)
		}
	}
	if provenUnexecuted(nil, 1, 1) {
		t.Fatal("an empty profile proved a position unexecuted")
	}
}

func TestApplyMutation(t *testing.T) {
	t.Parallel()
	source := []byte("a + b")
	mutated := applyMutation(source, 0, 1, "c")
	if string(mutated) != "c + b" {
		t.Fatalf("applyMutation = %q", mutated)
	}
	if string(source) != "a + b" {
		t.Fatal("applyMutation modified its input")
	}
}

func TestClassify(t *testing.T) {
	t.Parallel()
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	cases := []struct {
		name   string
		ctx    context.Context
		output string
		err    error
		want   outcome
	}{
		{"passing tests", context.Background(), "ok", nil, outcomeSurvived},
		{"failing tests", context.Background(), "--- FAIL", errors.New("exit 1"), outcomeKilled},
		{"build failure", context.Background(), "FAIL [build failed]", errors.New("exit 1"), outcomeNotViable},
		{"setup failure", context.Background(), "FAIL [setup failed]", errors.New("exit 1"), outcomeNotViable},
		{
			"test timeout", context.Background(), "panic: test timed out after 30s", errors.New("exit 2"),
			outcomeTimedOut,
		},
		{"deadline", canceled, "", errors.New("signal: killed"), outcomeTimedOut},
	}
	for _, testCase := range cases {
		if got := classify(testCase.ctx, []byte(testCase.output), testCase.err); got != testCase.want {
			t.Fatalf("%s: classify = %v, want %v", testCase.name, got, testCase.want)
		}
	}
}

func TestMutantTimeout(t *testing.T) {
	t.Parallel()
	cases := []struct {
		baseline time.Duration
		want     time.Duration
	}{
		{0, minMutantTimeout},
		{time.Second, minMutantTimeout},
		{2 * time.Second, 40 * time.Second},
		{5 * time.Second, 100 * time.Second},
	}
	for _, testCase := range cases {
		if got := mutantTimeout(testCase.baseline); got != testCase.want {
			t.Fatalf("mutantTimeout(%s) = %s, want %s", testCase.baseline, got, testCase.want)
		}
	}
}

func TestFindRoot(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "go.mod"), "module example.com/demo\n")
	subdirectory := filepath.Join(root, "internal", "core")
	if err := os.MkdirAll(subdirectory, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	found, err := findRoot(subdirectory)
	if err != nil || found != root {
		t.Fatalf("findRoot = %q, %v; want %q", found, err, root)
	}
	if _, err := findRoot(t.TempDir()); err == nil {
		t.Fatal("findRoot found a go.mod above an isolated temporary directory")
	}
}

func TestModulePath(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "go.mod"), "module example.com/demo\n\ngo 1.27.0\n")
	module, err := modulePath(root)
	if err != nil || module != "example.com/demo" {
		t.Fatalf("modulePath = %q, %v", module, err)
	}
	if _, err := modulePath(t.TempDir()); err == nil {
		t.Fatal("modulePath accepted a missing go.mod")
	}
	broken := t.TempDir()
	mustWrite(t, filepath.Join(broken, "go.mod"), "go 1.27.0\n")
	if _, err := modulePath(broken); err == nil {
		t.Fatal("modulePath accepted a go.mod without a module line")
	}
}

func TestPackageDir(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "go.mod"), "module example.com/demo\n")
	packagePath := filepath.Join(root, "internal", "core")
	if err := os.MkdirAll(packagePath, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if found, err := packageDir(root, "./internal/core"); err != nil || found != packagePath {
		t.Fatalf("packageDir relative = %q, %v", found, err)
	}
	if found, err := packageDir(root, packagePath); err != nil || found != packagePath {
		t.Fatalf("packageDir absolute = %q, %v", found, err)
	}
	if _, err := packageDir(root, "./missing"); err == nil {
		t.Fatal("packageDir accepted a missing package")
	}
	file := filepath.Join(root, "file.go")
	mustWrite(t, file, "package demo\n")
	if _, err := packageDir(root, file); err == nil {
		t.Fatal("packageDir accepted a file")
	}
	if _, err := packageDir(root, t.TempDir()); err == nil {
		t.Fatal("packageDir accepted a directory outside the module")
	}
}

func TestGoFilesSkipsTestsAndIgnoredFiles(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := filepath.Join(root, "internal", "core")
	mustWrite(t, filepath.Join(dir, "a.go"), "package core\n")
	mustWrite(t, filepath.Join(dir, "z.go"), "package core\n")
	mustWrite(t, filepath.Join(dir, "a_test.go"), "package core\n")
	mustWrite(t, filepath.Join(dir, "ignored.go"), "//go:build ignore\n\npackage core\n")
	if err := os.MkdirAll(filepath.Join(dir, "nested.go"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	files, err := goFiles(root, dir)
	if err != nil {
		t.Fatalf("goFiles: %v", err)
	}
	want := []string{"internal/core/a.go", "internal/core/z.go"}
	if strings.Join(files, ",") != strings.Join(want, ",") {
		t.Fatalf("goFiles = %v, want %v", files, want)
	}
	if _, err := goFiles(root, filepath.Join(root, "absent")); err == nil {
		t.Fatal("goFiles accepted a missing directory")
	}
}

func TestDiscoverPackages(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "go.mod"), "module example.com/demo\n")
	mustWrite(t, filepath.Join(root, "internal", "core", "a.go"), "package core\n")
	mustWrite(t, filepath.Join(root, "internal", "core", "a_test.go"), "package core\n")
	mustWrite(t, filepath.Join(root, "internal", "tui", "b.go"), "package tui\n")
	packages, err := discoverPackages(root, []string{"./internal/core", "./internal/tui"}, nil)
	if err != nil {
		t.Fatalf("discoverPackages: %v", err)
	}
	if len(packages) != 2 || packages[0].importPath != "example.com/demo/internal/core" {
		t.Fatalf("packages = %+v", packages)
	}
	if len(packages[0].files) != 1 {
		t.Fatalf("core files = %v, want only a.go", packages[0].files)
	}
	duplicated, err := discoverPackages(root, []string{"./internal/core", "./internal/core"}, nil)
	if err != nil || len(duplicated) != 1 {
		t.Fatalf("duplicate args = %d packages, %v", len(duplicated), err)
	}
	scoped, err := discoverPackages(root, []string{"./internal/core", "./internal/tui"}, []string{"internal/tui/b.go"})
	if err != nil || len(scoped) != 1 || scoped[0].importPath != "example.com/demo/internal/tui" {
		t.Fatalf("scoped packages = %+v, %v", scoped, err)
	}
	if _, err := discoverPackages(t.TempDir(), []string{"./internal/core"}, nil); err == nil {
		t.Fatal("discoverPackages accepted a directory without go.mod")
	}
	if _, err := discoverPackages(root, []string{"./missing"}, nil); err == nil {
		t.Fatal("discoverPackages accepted a missing package")
	}
}

func TestScopeHelpers(t *testing.T) {
	t.Parallel()
	if !inScope(nil, "any.go") {
		t.Fatal("an empty scope rejected a file")
	}
	if !inScope([]string{"a.go", "b.go"}, "b.go") {
		t.Fatal("scope rejected a listed file")
	}
	if inScope([]string{"a.go"}, "b.go") {
		t.Fatal("scope accepted an unlisted file")
	}
	if !anyInScope(nil, []string{"a.go"}) {
		t.Fatal("anyInScope rejected an empty scope")
	}
	if !anyInScope([]string{"b.go"}, []string{"a.go", "b.go"}) {
		t.Fatal("anyInScope missed a listed file")
	}
	if anyInScope([]string{"c.go"}, []string{"a.go", "b.go"}) {
		t.Fatal("anyInScope accepted a missing file")
	}
}

func TestRunBaselineRecordsCoverage(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	env := environment{
		root: tmpDir, module: "example.com/demo", tmpDir: tmpDir, goBin: "go", workers: 1,
		exec: func(_ context.Context, _, _ string, args ...string) ([]byte, error) {
			profile := argument(args, "-coverprofile=")
			if profile == "" {
				return nil, errors.New("missing coverage profile")
			}
			document := "mode: set\nexample.com/demo/a.go:1.1,2.1 1 1\n"

			return []byte("ok"), os.WriteFile(profile, []byte(document), 0o644)
		},
	}
	pkg := &pkgInfo{importPath: "example.com/demo"}
	if err := runBaseline(env, pkg); err != nil {
		t.Fatalf("runBaseline: %v", err)
	}
	if len(pkg.blocks["a.go"]) != 1 {
		t.Fatalf("blocks = %v", pkg.blocks)
	}
}

func TestRunBaselineReportsFailures(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	failing := func(_ context.Context, _, _ string, _ ...string) ([]byte, error) {
		return []byte("FAIL"), errors.New("exit status 1")
	}
	pkg := &pkgInfo{importPath: "example.com/demo"}
	env := environment{root: tmpDir, module: "example.com/demo", tmpDir: tmpDir, goBin: "go", workers: 1, exec: failing}
	if err := runBaseline(env, pkg); err == nil || !strings.Contains(err.Error(), "baseline tests") {
		t.Fatalf("runBaseline error = %v, want a baseline failure", err)
	}
	invalidProfile := func(_ context.Context, _, _ string, args ...string) ([]byte, error) {
		profile := argument(args, "-coverprofile=")
		if profile == "" {
			return nil, errors.New("missing coverage profile")
		}

		return []byte("ok"), os.WriteFile(profile, []byte("not a profile\n"), 0o644)
	}
	env.exec = invalidProfile
	if err := runBaseline(env, pkg); err == nil || !strings.Contains(err.Error(), "coverage for") {
		t.Fatalf("runBaseline error = %v, want a coverage error", err)
	}
}

const mutantSource = "package core\n\nfunc Negate(x int) int {\n\treturn -x\n}\n"

// fakeMutant is a mutant of the `-x` on line 4 of mutantSource, recorded at the given coverage count.
func fakeMutant(root, source string, count int) *mutation {
	offset := strings.Index(source, "-x")

	return &mutation{
		file: "a.go", absPath: filepath.Join(root, "a.go"), function: "Negate", operator: "unary",
		line: 4, col: 9, offset: offset, end: offset + 1, original: "-x", mutated: "x", src: []byte(source),
		pkg: &pkgInfo{
			importPath: "example.com/demo",
			dir:        root,
			blocks: map[string][]coverBlock{
				"a.go": {{startLine: 4, startCol: 9, endLine: 4, endCol: 11, count: count}},
			},
		},
	}
}

type execRecord struct {
	args     []string
	overlaid string
}

// recordingEnv returns an environment whose go command records what it was handed and reports err.
func recordingEnv(root string, err error) (environment, *execRecord) {
	record := &execRecord{}
	env := environment{
		root: root, module: "example.com/demo", tmpDir: root, goBin: "go", workers: 1,
		exec: func(_ context.Context, _, _ string, args ...string) ([]byte, error) {
			record.args = args
			overlayPath := argument(args, "-overlay=")
			if overlayPath == "" {
				return nil, errors.New("missing overlay")
			}
			content, readErr := overlayContent(overlayPath)
			if readErr != nil {
				return nil, readErr
			}
			record.overlaid = content

			return []byte("ok"), err
		},
	}

	return env, record
}

func hasArgument(args []string, want string) bool {
	for _, arg := range args {
		if arg == want {
			return true
		}
	}

	return false
}

func TestRunMutantBuildsUnexecutedMutants(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		err  error
		want outcome
	}{
		{"the build succeeds", nil, outcomeNeverExecuted},
		{"the build fails", errors.New("exit status 1"), outcomeNotViable},
	}
	for _, testCase := range cases {
		root := t.TempDir()
		mustWrite(t, filepath.Join(root, "a.go"), mutantSource)
		env, record := recordingEnv(root, testCase.err)
		mutant := fakeMutant(root, mutantSource, 0)
		runMutant(env, mutant)
		if mutant.runErr != nil {
			t.Fatalf("%s: runMutant error = %v", testCase.name, mutant.runErr)
		}
		if mutant.outcome != testCase.want {
			t.Fatalf("%s: outcome = %v, want %v", testCase.name, mutant.outcome, testCase.want)
		}
		if argument(record.args, "-run=") != "^$" || hasArgument(record.args, "-failfast") {
			t.Fatalf("%s: args = %v, want a build-only run", testCase.name, record.args)
		}
		if !strings.Contains(record.overlaid, "\treturn x\n") {
			t.Fatalf("%s: overlay content = %q, want the mutation applied", testCase.name, record.overlaid)
		}
	}
}

func TestRunMutantTestsExecutedMutants(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "a.go"), mutantSource)
	env, record := recordingEnv(root, errors.New("exit status 1"))
	mutant := fakeMutant(root, mutantSource, 1)
	runMutant(env, mutant)
	if mutant.runErr != nil {
		t.Fatalf("runMutant error = %v", mutant.runErr)
	}
	if mutant.outcome != outcomeKilled {
		t.Fatalf("outcome = %v, want %v", mutant.outcome, outcomeKilled)
	}
	if argument(record.args, "-run=") != "" || !hasArgument(record.args, "-failfast") {
		t.Fatalf("args = %v, want the package's tests run", record.args)
	}
}

func TestRunMutantReportsWriteErrors(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "a.go"), mutantSource)
	env, _ := recordingEnv(root, nil)
	env.tmpDir = filepath.Join(root, "absent")
	mutant := fakeMutant(root, mutantSource, 0)
	runMutant(env, mutant)
	if mutant.runErr == nil || !strings.Contains(mutant.runErr.Error(), "mutant-*.go") {
		t.Fatalf("runMutant error = %v, want a temporary file error", mutant.runErr)
	}
	if mutant.outcome != outcomePending {
		t.Fatalf("outcome = %v, want an unresolved mutant", mutant.outcome)
	}
}

func argument(args []string, prefix string) string {
	for _, arg := range args {
		if strings.HasPrefix(arg, prefix) {
			return strings.TrimPrefix(arg, prefix)
		}
	}

	return ""
}

func TestPrepareRunErrors(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "go.mod"), "module example.com/demo\n")
	mustWrite(t, filepath.Join(root, "internal", "core", "a.go"), "package core\n")

	if _, err := prepareRun(config{dir: t.TempDir()}); err == nil {
		t.Fatal("prepareRun accepted a directory without go.mod")
	}
	brokenModule := t.TempDir()
	mustWrite(t, filepath.Join(brokenModule, "go.mod"), "go 1.27.0\n")
	if _, err := prepareRun(config{dir: brokenModule}); err == nil {
		t.Fatal("prepareRun accepted a go.mod without a module line")
	}
	mustWrite(t, filepath.Join(root, "mutation", "exceptions.txt"), "identity\t\n")
	if _, err := prepareRun(config{dir: root, exceptions: "mutation/exceptions.txt",
		args: []string{"./internal/core"}}); err == nil {
		t.Fatal("prepareRun accepted a malformed exception list")
	}
	if _, err := prepareRun(config{dir: root, exceptions: "absent.txt", args: []string{"./missing"}}); err == nil {
		t.Fatal("prepareRun accepted a missing package")
	}
	mustWrite(t, filepath.Join(root, "internal", "core", "bad.go"), "package core\nfunc {\n")
	if _, err := prepareRun(config{dir: root, exceptions: "absent.txt",
		args: []string{"./internal/core"}}); err == nil {
		t.Fatal("prepareRun accepted an unparseable source file")
	}
}

const endToEndSource = `package core

func Add(x int) int {
	if x > 0 {
		return x + 1
	}

	return -x
}
`

func TestMutatePackagesGatesUpdatesAndDropsStaleEntries(t *testing.T) {
	t.Parallel()
	root, cfg := newEndToEnd(t)
	offenders := requireInitialFailure(t, cfg)
	requireUpdateAddsNothing(t, root, cfg)
	requireGateStillFails(t, cfg, offenders)
	requireManualExceptionsPassGate(t, root, cfg, offenders)
	requireStaleEntryFails(t, root, cfg)
	requireUpdateDropsStale(t, root, cfg, len(offenders))
}

func newEndToEnd(t *testing.T) (string, config) {
	t.Helper()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "go.mod"), "module example.com/demo\n")
	mustWrite(t, filepath.Join(root, "internal", "core", "a.go"), endToEndSource)
	mustWrite(t, filepath.Join(root, "internal", "core", "a_test.go"), "package core\n")

	return root, config{
		args:       []string{"./internal/core"},
		exceptions: "mutation/exceptions.txt",
		workers:    2,
		dir:        root,
		exec:       fakeGoCommand(),
	}
}

func runMutation(t *testing.T, cfg config) (int, string) {
	t.Helper()
	var output bytes.Buffer
	cfg.stdout = &output
	status, err := mutatePackages(cfg)
	if err != nil {
		t.Fatalf("mutatePackages: %v", err)
	}

	return status, output.String()
}

// requireInitialFailure runs the gate and returns the identities it named as offenders.
func requireInitialFailure(t *testing.T, cfg config) []string {
	t.Helper()
	status, output := runMutation(t, cfg)
	if status != exitFailure {
		t.Fatalf("first run status = %d, want %d", status, exitFailure)
	}
	if !strings.Contains(output, "survived:") || !strings.Contains(output, "never-executed:") {
		t.Fatalf("first run did not report both offenders:\n%s", output)
	}
	var offenders []string
	for _, line := range strings.Split(output, "\n") {
		for _, prefix := range []string{"survived: ", "never-executed: "} {
			if identity, found := strings.CutPrefix(line, prefix); found {
				offenders = append(offenders, identity)
			}
		}
	}
	if len(offenders) == 0 {
		t.Fatalf("first run did not name an identity:\n%s", output)
	}

	return offenders
}

// requireUpdateAddsNothing checks that -update removes stale entries only and records no new survivor.
func requireUpdateAddsNothing(t *testing.T, root string, cfg config) {
	t.Helper()
	cfg.update = true
	if status, output := runMutation(t, cfg); status != exitSuccess {
		t.Fatalf("update run = %d, want %d\n%s", status, exitSuccess, output)
	}
	entries, err := loadEntries(filepath.Join(root, "mutation", "exceptions.txt"))
	if err != nil {
		t.Fatalf("load entries: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("update recorded %d entries, want none: %+v", len(entries), entries)
	}
}

func requireGateStillFails(t *testing.T, cfg config, offenders []string) {
	t.Helper()
	status, output := runMutation(t, cfg)
	if status != exitFailure {
		t.Fatalf("gate after update = %d, want %d\n%s", status, exitFailure, output)
	}
	for _, identity := range offenders {
		if !strings.Contains(output, identity) {
			t.Fatalf("gate after update did not report %q:\n%s", identity, output)
		}
	}
}

// requireManualExceptionsPassGate adds a justified entry by hand for each survivor, as a developer must.
func requireManualExceptionsPassGate(t *testing.T, root string, cfg config, offenders []string) {
	t.Helper()
	var document strings.Builder
	for _, identity := range offenders {
		document.WriteString(identity)
		document.WriteString("\tthe reason Standards will check\n")
	}
	if err := appendToFile(filepath.Join(root, "mutation", "exceptions.txt"), document.String()); err != nil {
		t.Fatalf("append manual entries: %v", err)
	}
	if status, output := runMutation(t, cfg); status != exitSuccess {
		t.Fatalf("gate with manual exceptions = %d, want %d\n%s", status, exitSuccess, output)
	}
}

func requireStaleEntryFails(t *testing.T, root string, cfg config) {
	t.Helper()
	stale := "internal/core/a.go|Add|arithmetic|x + 1 => x - 1|1\tstale reason\n"
	if err := appendToFile(filepath.Join(root, "mutation", "exceptions.txt"), stale); err != nil {
		t.Fatalf("append stale entry: %v", err)
	}
	status, output := runMutation(t, cfg)
	if status != exitFailure {
		t.Fatalf("stale gate = %d, want %d", status, exitFailure)
	}
	if !strings.Contains(output, "stale exception:") {
		t.Fatalf("stale gate did not name the entry:\n%s", output)
	}
}

func requireUpdateDropsStale(t *testing.T, root string, cfg config, want int) {
	t.Helper()
	cfg.update = true
	if status, output := runMutation(t, cfg); status != exitSuccess {
		t.Fatalf("stale update = %d, want %d\n%s", status, exitSuccess, output)
	}
	entries, err := loadEntries(filepath.Join(root, "mutation", "exceptions.txt"))
	if err != nil || len(entries) != want {
		t.Fatalf("entries after dropping the stale one = %d, %v; want %d", len(entries), err, want)
	}
}

func fakeGoCommand() commandFunc {
	return func(_ context.Context, _, _ string, args ...string) ([]byte, error) {
		if profile := argument(args, "-coverprofile="); profile != "" {
			document := "mode: set\n" +
				"example.com/demo/internal/core/a.go:4.2,6.1 2 1\n" +
				"example.com/demo/internal/core/a.go:8.2,8.11 1 0\n"

			return []byte("ok"), os.WriteFile(profile, []byte(document), 0o644)
		}
		overlayPath := argument(args, "-overlay=")
		if overlayPath == "" {
			return nil, errors.New("missing overlay")
		}
		content, err := overlayContent(overlayPath)
		if err != nil {
			return nil, err
		}
		if argument(args, "-run=") == "^$" {
			return []byte("ok"), nil
		}
		switch {
		case strings.Contains(content, "x >= 0"):
			return []byte("panic: test timed out after 30s"), errors.New("exit status 2")
		case strings.Contains(content, "x <= 0"):
			return []byte("ok"), nil
		case strings.Contains(content, "x - 1"):
			return []byte("--- FAIL: TestAdd"), errors.New("exit status 1")
		default:
			return []byte("FAIL [build failed]"), errors.New("exit status 1")
		}
	}
}

func overlayContent(path string) (string, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var parsed overlay
	if err := json.Unmarshal(payload, &parsed); err != nil {
		return "", err
	}
	for _, replacement := range parsed.Replace {
		content, err := os.ReadFile(replacement)
		if err != nil {
			return "", err
		}

		return string(content), nil
	}

	return "", errors.New("overlay has no replacement")
}
