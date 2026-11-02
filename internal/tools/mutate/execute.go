package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/build"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// outcome is how one mutant fared against its package's tests.
type outcome int

const (
	outcomePending outcome = iota
	outcomeKilled
	outcomeSurvived
	outcomeNeverExecuted
	outcomeTimedOut
	outcomeNotViable
)

// Timeout policy: a mutant gets max(30s, 20x the package's measured baseline duration). mutants that hang are counted
// as killed, so the multiplier is generous enough that a slow-but-finishing mutant never flips to a timeout.
const (
	minMutantTimeout = 30 * time.Second
	mutantMultiplier = 20
	mutantGrace      = 10 * time.Second
	baselineLimit    = 5 * time.Minute
)

// pkgInfo is one target package: its import path, directory, non-test files, and measured coverage.
type pkgInfo struct {
	importPath string
	dir        string
	files      []string
	baseline   time.Duration
	blocks     map[string][]coverBlock
}

// coverBlock is one instrumented statement range from a Go coverage profile.
type coverBlock struct {
	startLine int
	startCol  int
	endLine   int
	endCol    int
	count     int
}

type commandFunc func(ctx context.Context, dir, name string, args ...string) ([]byte, error)

// environment is everything a run needs after the command line is parsed.
type environment struct {
	root    string
	module  string
	tmpDir  string
	goBin   string
	exec    commandFunc
	workers int
}

type config struct {
	args       []string
	exceptions string
	scope      []string
	workers    int
	update     bool
	dir        string
	stdout     io.Writer
	stderr     io.Writer
	exec       commandFunc
}

// preparedRun is a run with its repository, packages, mutants, and exception entries resolved.
type preparedRun struct {
	env        environment
	packages   []*pkgInfo
	mutants    []*mutation
	entries    []entry
	exceptions string
}

// mutatePackages runs mutation testing and returns the process exit status.
func mutatePackages(cfg config) (int, error) {
	started := time.Now()
	prepared, err := prepareRun(cfg)
	if err != nil {
		return 0, err
	}
	defer func() { _ = os.RemoveAll(prepared.env.tmpDir) }()
	if err := runMutants(prepared.env, prepared.packages, prepared.mutants); err != nil {
		return 0, err
	}

	return finishRun(cfg, prepared, started)
}

// prepareRun resolves the repository, reads the exception list, and generates every in-scope mutant.
func prepareRun(cfg config) (preparedRun, error) {
	root, err := findRoot(cfg.dir)
	if err != nil {
		return preparedRun{}, err
	}
	module, err := modulePath(root)
	if err != nil {
		return preparedRun{}, err
	}
	tmpDir, err := os.MkdirTemp("", "pongo-mutate-")
	if err != nil {
		return preparedRun{}, fmt.Errorf("create temporary directory: %w", err)
	}
	env := environment{root: root, module: module, tmpDir: tmpDir, goBin: "go", workers: cfg.workers}
	if env.workers < 1 {
		env.workers = 1
	}
	env.exec = cfg.exec
	if env.exec == nil {
		env.exec = runProcess
	}
	exceptionsPath := resolvePath(root, cfg.exceptions)
	entries, err := loadEntries(exceptionsPath)
	if err != nil {
		return preparedRun{}, err
	}
	packages, err := discoverPackages(root, cfg.args, cfg.scope)
	if err != nil {
		return preparedRun{}, err
	}
	mutants, err := generatePackages(packages, cfg.scope)
	if err != nil {
		return preparedRun{}, err
	}

	return preparedRun{
		env: env, packages: packages, mutants: mutants, entries: entries, exceptions: exceptionsPath,
	}, nil
}

// finishRun reports results and either updates the exception list or gates on it.
func finishRun(cfg config, prepared preparedRun, started time.Time) (int, error) {
	missing, stale := evaluate(prepared.mutants, prepared.entries, cfg.scope)
	printSummary(cfg.stdout, prepared.packages, prepared.mutants, time.Since(started))
	if cfg.update {
		updated := updateEntries(prepared.entries, prepared.mutants, cfg.scope)
		if err := writeEntries(prepared.exceptions, updated); err != nil {
			return 0, err
		}
		_, _ = fmt.Fprintf(cfg.stdout, "mutation testing: recorded %d exception(s) in %s\n", len(updated),
			cfg.exceptions)

		return exitSuccess, nil
	}

	return reportGate(cfg.stdout, missing, stale, mutantIndex(prepared.mutants)), nil
}

func generatePackages(packages []*pkgInfo, scope []string) ([]*mutation, error) {
	var mutants []*mutation
	for _, pkg := range packages {
		generated, err := generatePackage(pkg, scope)
		if err != nil {
			return nil, err
		}
		mutants = append(mutants, generated...)
	}

	return mutants, nil
}

func resolvePath(root, path string) string {
	if filepath.IsAbs(path) {
		return path
	}

	return filepath.Join(root, filepath.FromSlash(path))
}

// findRoot walks up from dir to the directory holding go.mod.
func findRoot(dir string) (string, error) {
	started, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("resolve working directory: %w", err)
	}
	for current := started; ; current = filepath.Dir(current) {
		if _, statErr := os.Stat(filepath.Join(current, "go.mod")); statErr == nil {
			return current, nil
		}
		if filepath.Dir(current) == current {
			return "", fmt.Errorf("no go.mod above %s", started)
		}
	}
}

// modulePath reads the module line of the repository's go.mod.
func modulePath(root string) (string, error) {
	content, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return "", fmt.Errorf("read go.mod: %w", err)
	}
	for _, line := range strings.Split(string(content), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "module" {
			return fields[1], nil
		}
	}

	return "", errors.New("go.mod has no module line")
}

// discoverPackages resolves the package arguments to directories, import paths, and mutatable files.
func discoverPackages(root string, args, scope []string) ([]*pkgInfo, error) {
	module, err := modulePath(root)
	if err != nil {
		return nil, err
	}
	packages := make([]*pkgInfo, 0, len(args))
	seen := make(map[string]bool)
	for _, arg := range args {
		dir, err := packageDir(root, arg)
		if err != nil {
			return nil, err
		}
		if seen[dir] {
			continue
		}
		seen[dir] = true
		files, err := goFiles(root, dir)
		if err != nil {
			return nil, err
		}
		if len(files) == 0 || !anyInScope(scope, files) {
			continue
		}
		relative, err := filepath.Rel(root, dir)
		if err != nil {
			return nil, fmt.Errorf("relativize %s: %w", dir, err)
		}
		importPath := module
		if relative != "." {
			importPath = module + "/" + filepath.ToSlash(relative)
		}
		packages = append(packages, &pkgInfo{importPath: importPath, dir: dir, files: files})
	}

	return packages, nil
}

func packageDir(root, arg string) (string, error) {
	dir := arg
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(root, filepath.FromSlash(arg))
	}
	info, err := os.Stat(dir)
	if err != nil {
		return "", fmt.Errorf("package %s: %w", arg, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("package %s is not a directory", arg)
	}
	absolute, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("resolve %s: %w", arg, err)
	}
	relative, err := filepath.Rel(root, absolute)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("package %s is outside the module", arg)
	}

	return absolute, nil
}

// goFiles lists the package's buildable non-test Go files as repo-relative slash paths.
func goFiles(root, dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("list %s: %w", dir, err)
	}
	var files []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		buildable, err := build.Default.MatchFile(dir, name)
		if err != nil {
			return nil, fmt.Errorf("match %s: %w", name, err)
		}
		if !buildable {
			continue
		}
		relative, err := filepath.Rel(root, filepath.Join(dir, name))
		if err != nil {
			return nil, fmt.Errorf("relativize %s: %w", name, err)
		}
		files = append(files, filepath.ToSlash(relative))
	}
	sort.Strings(files)

	return files, nil
}

func anyInScope(scope []string, files []string) bool {
	for _, file := range files {
		if inScope(scope, file) {
			return true
		}
	}

	return false
}

func inScope(scope []string, file string) bool {
	if len(scope) == 0 {
		return true
	}
	for _, name := range scope {
		if name == file {
			return true
		}
	}

	return false
}

// runMutants measures each package, then runs every mutant in parallel; one coverage proves unexecuted is built instead.
func runMutants(env environment, packages []*pkgInfo, mutants []*mutation) error {
	for _, pkg := range packages {
		if err := runBaseline(env, pkg); err != nil {
			return err
		}
	}
	runPending(env, mutants)
	for _, mutant := range mutants {
		if mutant.runErr != nil {
			return mutant.runErr
		}
	}

	return nil
}

// runBaseline runs the package's own tests once for its duration and executed blocks.
func runBaseline(env environment, pkg *pkgInfo) error {
	profile, err := os.CreateTemp(env.tmpDir, "cover-*.out")
	if err != nil {
		return fmt.Errorf("create coverage profile: %w", err)
	}
	path := profile.Name()
	if err := profile.Close(); err != nil {
		return fmt.Errorf("close coverage profile: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), baselineLimit)
	defer cancel()
	args := []string{"test", "-count=1", "-coverprofile=" + path, pkg.importPath}
	started := time.Now()
	output, err := env.exec(ctx, env.root, env.goBin, args...)
	pkg.baseline = time.Since(started)
	if err != nil {
		return fmt.Errorf("baseline tests for %s failed: %w\n%s", pkg.importPath, err, output)
	}
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open coverage profile: %w", err)
	}
	defer func() { _ = file.Close() }()
	blocks, err := parseCoverage(file, env.module)
	if err != nil {
		return fmt.Errorf("coverage for %s: %w", pkg.importPath, err)
	}
	pkg.blocks = blocks

	return nil
}

func runPending(env environment, mutants []*mutation) {
	jobs := make(chan *mutation)
	var group sync.WaitGroup
	for range env.workers {
		group.Add(1)
		go func() {
			defer group.Done()
			for mutant := range jobs {
				runMutant(env, mutant)
			}
		}()
	}
	for _, mutant := range mutants {
		if mutant.outcome == outcomePending {
			jobs <- mutant
		}
	}
	close(jobs)
	group.Wait()
}

// runMutant tests one mutant through an overlay so no tracked file is ever written. A mutant that coverage proves
// unexecuted is only built: it must compile to be never executed rather than not viable.
func runMutant(env environment, mutant *mutation) {
	sourcePath, overlayPath, err := writeMutantFile(env, mutant)
	if err != nil {
		mutant.runErr = err
		return
	}
	defer func() {
		_ = os.Remove(sourcePath)
		_ = os.Remove(overlayPath)
	}()
	if provenUnexecuted(mutant.pkg.blocks[mutant.file], mutant.line, mutant.col) {
		mutant.outcome = buildMutant(env, mutant, overlayPath)
		return
	}
	mutant.outcome = testMutant(env, mutant, overlayPath)
}

// writeMutantFile writes the mutant's source and the overlay that points the go command at it.
func writeMutantFile(env environment, mutant *mutation) (string, string, error) {
	mutated := applyMutation(mutant.src, mutant.offset, mutant.end, mutant.replacement)
	sourcePath, err := writeTempFile(env.tmpDir, "mutant-*.go", mutated)
	if err != nil {
		return "", "", err
	}
	overlayPath, err := writeOverlay(env.tmpDir, mutant.absPath, sourcePath)
	if err != nil {
		_ = os.Remove(sourcePath)

		return "", "", err
	}

	return sourcePath, overlayPath, nil
}

// testMutant runs the mutant against the package's own tests and classifies the result.
func testMutant(env environment, mutant *mutation, overlayPath string) outcome {
	timeout := mutantTimeout(mutant.pkg.baseline)
	ctx, cancel := context.WithTimeout(context.Background(), timeout+mutantGrace)
	defer cancel()
	args := []string{
		"test", "-count=1", "-failfast", "-timeout=" + timeout.String(),
		"-overlay=" + overlayPath, mutant.pkg.importPath,
	}
	output, err := env.exec(ctx, env.root, env.goBin, args...)

	return classify(ctx, output, err)
}

// buildMutant compiles the mutant and the package's tests without running them: a failed build is not viable, and a
// build that succeeds leaves the coverage-proven verdict of never executed.
func buildMutant(env environment, mutant *mutation, overlayPath string) outcome {
	timeout := mutantTimeout(mutant.pkg.baseline)
	ctx, cancel := context.WithTimeout(context.Background(), timeout+mutantGrace)
	defer cancel()
	args := []string{
		"test", "-count=1", "-run=^$", "-timeout=" + timeout.String(),
		"-overlay=" + overlayPath, mutant.pkg.importPath,
	}
	if _, err := env.exec(ctx, env.root, env.goBin, args...); err != nil {
		return outcomeNotViable
	}

	return outcomeNeverExecuted
}

// writeOverlay writes the JSON overlay that points the go command at the mutant's temporary file.
func writeOverlay(tmpDir, original, replacement string) (string, error) {
	payload, err := json.Marshal(overlay{Replace: map[string]string{original: replacement}})
	if err != nil {
		return "", fmt.Errorf("encode overlay: %w", err)
	}

	return writeTempFile(tmpDir, "overlay-*.json", payload)
}

// writeTempFile writes content to a new file in dir and returns its path.
func writeTempFile(dir, pattern string, content []byte) (string, error) {
	file, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return "", fmt.Errorf("create %s: %w", pattern, err)
	}
	path := file.Name()
	if _, err := file.Write(content); err != nil {
		_ = file.Close()
		_ = os.Remove(path)

		return "", fmt.Errorf("write %s: %w", path, err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)

		return "", fmt.Errorf("close %s: %w", path, err)
	}

	return path, nil
}

type overlay struct {
	Replace map[string]string `json:"Replace"`
}

func applyMutation(src []byte, start, end int, replacement string) []byte {
	mutated := make([]byte, 0, len(src)-(end-start)+len(replacement))
	mutated = append(mutated, src[:start]...)
	mutated = append(mutated, replacement...)
	mutated = append(mutated, src[end:]...)

	return mutated
}

// classify turns one go test result into an outcome. A timeout counts as killed: a hanging mutant is detected.
func classify(ctx context.Context, output []byte, err error) outcome {
	if err == nil {
		return outcomeSurvived
	}
	text := string(output)
	if ctx.Err() != nil || strings.Contains(text, "test timed out") {
		return outcomeTimedOut
	}
	if strings.Contains(text, "[build failed]") || strings.Contains(text, "[setup failed]") {
		return outcomeNotViable
	}

	return outcomeKilled
}

func mutantTimeout(baseline time.Duration) time.Duration {
	timeout := time.Duration(mutantMultiplier) * baseline
	if timeout < minMutantTimeout {
		return minMutantTimeout
	}

	return timeout
}

// runProcess runs a host command and returns its combined output.
func runProcess(ctx context.Context, dir, name string, args ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, name, args...)
	command.Dir = dir
	output, err := command.CombinedOutput()
	if err != nil {
		return output, fmt.Errorf("run %s %s: %w", name, strings.Join(args, " "), err)
	}

	return output, nil
}

// parseCoverage turns a Go coverage profile into executed and unexecuted statement ranges by repo-relative file.
func parseCoverage(reader io.Reader, module string) (map[string][]coverBlock, error) {
	scanner := bufio.NewScanner(reader)
	blocks := make(map[string][]coverBlock)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "mode:") {
			continue
		}
		name, block, err := parseCoverLine(line)
		if err != nil {
			return nil, err
		}
		relative := strings.TrimPrefix(name, module+"/")
		blocks[relative] = append(blocks[relative], block)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read coverage: %w", err)
	}

	return blocks, nil
}

func parseCoverLine(line string) (string, coverBlock, error) {
	fields := strings.Fields(line)
	if len(fields) != 3 {
		return "", coverBlock{}, fmt.Errorf("malformed coverage line %q", line)
	}
	colon := strings.LastIndexByte(fields[0], ':')
	if colon < 0 {
		return "", coverBlock{}, fmt.Errorf("malformed coverage line %q", line)
	}
	startText, endText, found := strings.Cut(fields[0][colon+1:], ",")
	if !found {
		return "", coverBlock{}, fmt.Errorf("malformed coverage line %q", line)
	}
	startLine, startCol, err := parsePoint(startText)
	if err != nil {
		return "", coverBlock{}, fmt.Errorf("coverage line %q: %w", line, err)
	}
	endLine, endCol, err := parsePoint(endText)
	if err != nil {
		return "", coverBlock{}, fmt.Errorf("coverage line %q: %w", line, err)
	}
	count, err := strconv.Atoi(fields[2])
	if err != nil {
		return "", coverBlock{}, fmt.Errorf("coverage line %q: %w", line, err)
	}

	return fields[0][:colon], coverBlock{
		startLine: startLine, startCol: startCol, endLine: endLine, endCol: endCol, count: count,
	}, nil
}

func parsePoint(text string) (int, int, error) {
	lineText, colText, found := strings.Cut(text, ".")
	if !found {
		return 0, 0, fmt.Errorf("malformed position %q", text)
	}
	line, err := strconv.Atoi(lineText)
	if err != nil {
		return 0, 0, fmt.Errorf("malformed position %q: %w", text, err)
	}
	col, err := strconv.Atoi(colText)
	if err != nil {
		return 0, 0, fmt.Errorf("malformed position %q: %w", text, err)
	}

	return line, col, nil
}

// provenUnexecuted reports whether coverage proves the position never ran: some block containing it has a zero count and
// no block containing it executed. Go's profile records no block at all for switch case expressions and package-level
// initializers, so a position in no block is not proof and its mutant is run.
func provenUnexecuted(blocks []coverBlock, line, col int) bool {
	contained := false
	for _, block := range blocks {
		if !block.contains(line, col) {
			continue
		}
		if block.count > 0 {
			return false
		}
		contained = true
	}

	return contained
}

func (b coverBlock) contains(line, col int) bool {
	afterStart := line > b.startLine || (line == b.startLine && col >= b.startCol)
	beforeEnd := line < b.endLine || (line == b.endLine && col <= b.endCol)

	return afterStart && beforeEnd
}
