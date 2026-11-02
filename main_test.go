package main_test

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

type outcome struct {
	status int
	stdout string
	stderr string
}

func TestHelpWorksWithoutTerminal(t *testing.T) {
	t.Parallel()

	pongo := buildPongo(t)
	for _, argument := range []string{"--help"} {
		t.Run(argument, func(t *testing.T) {
			t.Parallel()

			result := runWithoutTerminal(t, pongo, argument)
			if result.status != 0 {
				t.Fatalf("pongo %s exited %d, want 0; stderr: %s", argument, result.status, result.stderr)
			}
			if result.stdout == "" {
				t.Fatalf("pongo %s printed no usage", argument)
			}
			if result.stderr != "" {
				t.Fatalf("pongo %s wrote to stderr: %s", argument, result.stderr)
			}
		})
	}
}

func TestVersionReportsEmbeddedSemanticVersion(t *testing.T) {
	t.Parallel()

	version := reportedVersion(t, buildPongo(t))
	if _, valid := semanticMajor(version); !valid {
		t.Fatalf("pongo --version = %q, want a semantic version", version)
	}
}

func TestDevelopmentBuildReportsPreReleaseVersion(t *testing.T) {
	t.Parallel()

	version := reportedVersion(t, buildPongo(t, "-buildvcs=false"))
	if major, valid := semanticMajor(version); !valid || major != "0" {
		t.Fatalf("development build version = %q, want a semantic version before v1.0.0", version)
	}
}

func TestUnsupportedArgumentsExitWithUsageStatus(t *testing.T) {
	t.Parallel()

	pongo := buildPongo(t)
	for _, arguments := range [][]string{
		{"--bogus"},
		{"-x"},
		{"-h"},
		{"play"},
		{"--help", "extra"},
		{"--version", "extra"},
		{"--version=maybe"},
	} {
		t.Run(strings.Join(arguments, " "), func(t *testing.T) {
			t.Parallel()

			result := runWithoutTerminal(t, pongo, arguments...)
			if result.status != 2 {
				t.Fatalf("pongo %q exited %d, want 2", arguments, result.status)
			}
			if result.stderr == "" {
				t.Fatalf("pongo %q printed no error", arguments)
			}
			if result.stdout != "" {
				t.Fatalf("pongo %q wrote to stdout: %s", arguments, result.stdout)
			}
		})
	}
}

// semanticMajor returns the major component of a Semantic Versioning 2.0.0 string with Go's "v" prefix.
func semanticMajor(version string) (string, bool) {
	pattern := regexp.MustCompile(
		`^v(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)` +
			`(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?(\+[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$`,
	)
	match := pattern.FindStringSubmatch(version)
	if match == nil {
		return "", false
	}

	return match[1], true
}

// reportedVersion runs pongo --version without a terminal and returns its single output line.
func reportedVersion(t *testing.T, executable string) string {
	t.Helper()

	result := runWithoutTerminal(t, executable, "--version")
	if result.status != 0 {
		t.Fatalf("pongo --version exited %d, want 0; stderr: %s", result.status, result.stderr)
	}
	if result.stderr != "" {
		t.Fatalf("pongo --version wrote to stderr: %s", result.stderr)
	}
	version, found := strings.CutSuffix(result.stdout, "\n")
	if !found || strings.Contains(version, "\n") {
		t.Fatalf("pongo --version printed %q, want one line", result.stdout)
	}

	return version
}

// e2eCoverDirectoryEnvironment names the directory the end-to-end tests make the coverage-instrumented executable write
// its counters to.
const e2eCoverDirectoryEnvironment = "PONGO_E2E_COVERDIR"

// coverageBuildFlags returns the build flags that instrument the executable for end-to-end coverage, if requested.
func coverageBuildFlags() []string {
	if os.Getenv(e2eCoverDirectoryEnvironment) == "" {
		return nil
	}

	return []string{"-cover", "-coverpkg=./..."}
}

// processEnvironment returns the test environment plus the coverage directory for the instrumented executable.
func processEnvironment() []string {
	environment := os.Environ()
	if directory := os.Getenv(e2eCoverDirectoryEnvironment); directory != "" {
		environment = append(environment, "GOCOVERDIR="+directory)
	}

	return environment
}

// buildPongo compiles the executable under test into a temporary directory.
func buildPongo(t *testing.T, buildFlags ...string) string {
	t.Helper()

	executable := filepath.Join(t.TempDir(), "pongo")
	if directory := os.Getenv(e2eCoverDirectoryEnvironment); directory != "" {
		if err := os.MkdirAll(directory, 0o777); err != nil {
			t.Fatalf("create coverage directory: %v", err)
		}
	}
	arguments := append([]string{"build"}, coverageBuildFlags()...)
	arguments = append(arguments, buildFlags...)
	arguments = append(arguments, "-o", executable, ".")
	if output, err := exec.Command("go", arguments...).CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, output)
	}

	return executable
}

// runWithoutTerminal runs pongo with standard input from the null device and captured output streams.
func runWithoutTerminal(t *testing.T, executable string, arguments ...string) outcome {
	t.Helper()

	var stdout, stderr bytes.Buffer
	command := exec.Command(executable, arguments...)
	command.Env = processEnvironment()
	command.Stdout = &stdout
	command.Stderr = &stderr

	err := command.Run()
	var exitError *exec.ExitError
	if err != nil && !errors.As(err, &exitError) {
		t.Fatalf("run pongo: %v", err)
	}

	return outcome{status: command.ProcessState.ExitCode(), stdout: stdout.String(), stderr: stderr.String()}
}
