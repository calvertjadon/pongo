package main

import (
	"bytes"
	"os"
	"runtime"
	"strings"
	"testing"
)

func TestParseConfigDefaults(t *testing.T) {
	t.Parallel()
	cfg, err := parseConfig(nil, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	if len(cfg.args) != 2 || cfg.args[0] != "./internal/core" || cfg.args[1] != "./internal/tui" {
		t.Fatalf("args = %v, want the Core and Shell", cfg.args)
	}
	if cfg.exceptions != "mutation/exceptions.txt" {
		t.Fatalf("exceptions = %q", cfg.exceptions)
	}
	if cfg.workers != runtime.NumCPU() {
		t.Fatalf("workers = %d, want %d", cfg.workers, runtime.NumCPU())
	}
	if cfg.scope != nil || cfg.update || cfg.dir != "." {
		t.Fatalf("defaults = %+v", cfg)
	}
}

func TestParseConfigReadsFlags(t *testing.T) {
	t.Parallel()
	cfg, err := parseConfig([]string{
		"-exceptions", "list.txt", "-files", "internal/core/a.go, internal/tui/b.go", "-workers", "3",
		"-update", "./internal/core",
	}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	if cfg.exceptions != "list.txt" || cfg.workers != 3 || !cfg.update {
		t.Fatalf("flags = %+v", cfg)
	}
	if len(cfg.args) != 1 || cfg.args[0] != "./internal/core" {
		t.Fatalf("args = %v", cfg.args)
	}
	if strings.Join(cfg.scope, ",") != "internal/core/a.go,internal/tui/b.go" {
		t.Fatalf("scope = %v", cfg.scope)
	}
}

func TestParseConfigRejectsBadValues(t *testing.T) {
	t.Parallel()
	cases := [][]string{
		{"-workers", "0"},
		{"-workers", "many"},
		{"-files", ","},
		{"-unknown"},
	}
	for _, args := range cases {
		if _, err := parseConfig(args, &bytes.Buffer{}); err == nil {
			t.Fatalf("parseConfig(%v) accepted invalid arguments", args)
		}
	}
}

func TestParseScope(t *testing.T) {
	t.Parallel()
	scope, err := parseScope("a.go,b.go")
	if err != nil || strings.Join(scope, ",") != "a.go,b.go" {
		t.Fatalf("parseScope = %v, %v", scope, err)
	}
	empty, err := parseScope("")
	if err != nil || empty != nil {
		t.Fatalf("parseScope(\"\") = %v, %v", empty, err)
	}
	if _, err := parseScope("a.go,,b.go"); err == nil {
		t.Fatal("parseScope accepted an empty entry")
	}
}

func TestRunReportsUsageErrors(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	if status := run([]string{"-workers", "0"}, &stdout, &stderr); status != exitUsage {
		t.Fatalf("run status = %d, want %d", status, exitUsage)
	}
	if !strings.Contains(stderr.String(), "mutate:") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

// TestRunReportsPreparationErrors moves into a directory without go.mod, so the run fails before it starts tests.
func TestRunReportsPreparationErrors(t *testing.T) {
	original, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(original) })
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	var stdout, stderr bytes.Buffer
	if status := run(nil, &stdout, &stderr); status != exitUsage {
		t.Fatalf("run status = %d, want %d", status, exitUsage)
	}
	if !strings.Contains(stderr.String(), "go.mod") {
		t.Fatalf("stderr = %q, want it to name the missing go.mod", stderr.String())
	}
}

func TestSplitArgumentsAcceptsInterspersedFlags(t *testing.T) {
	t.Parallel()
	flags, positional, err := splitArguments([]string{
		"./internal/core", "-update", "-workers", "2", "-files=a.go", "./internal/tui",
	})
	if err != nil {
		t.Fatalf("splitArguments: %v", err)
	}
	if strings.Join(flags, " ") != "-update -workers 2 -files=a.go" {
		t.Fatalf("flags = %v", flags)
	}
	if strings.Join(positional, " ") != "./internal/core ./internal/tui" {
		t.Fatalf("positional = %v", positional)
	}
	if _, _, err := splitArguments([]string{"-workers"}); err == nil {
		t.Fatal("splitArguments accepted a flag without its value")
	}
}

func TestParseConfigReadsFlagsAfterPackages(t *testing.T) {
	t.Parallel()
	cfg, err := parseConfig([]string{"./internal/core", "-update"}, &bytes.Buffer{})
	if err != nil || !cfg.update || len(cfg.args) != 1 {
		t.Fatalf("parseConfig = %+v, %v", cfg, err)
	}
}
