// Package main runs mutation testing over Pongo's deterministic Core and its Shell.
//
// Each mutant is one single edit to one non-test Go file. The tool runs the mutant package's own tests against an
// overlay of the mutated file, so tracked files are never written, and it fails when a surviving or never-executed
// mutant is not listed in mutation/exceptions.txt.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Process exit statuses.
const (
	exitSuccess = 0
	exitFailure = 1
	exitUsage   = 2
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run parses the command line, runs mutation testing, and maps failures to exit statuses.
func run(args []string, stdout, stderr io.Writer) int {
	cfg, err := parseConfig(args, stderr)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "mutate: %v\n", err)
		return exitUsage
	}
	cfg.stdout = stdout
	cfg.stderr = stderr
	status, err := mutatePackages(cfg)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "mutate: %v\n", err)
		return exitUsage
	}

	return status
}

// parseConfig reads the mutation run's flags and package arguments.
func parseConfig(args []string, stderr io.Writer) (config, error) {
	set := flag.NewFlagSet("mutate", flag.ContinueOnError)
	set.SetOutput(stderr)
	exceptions := set.String("exceptions", "mutation/exceptions.txt", "exception list to read and update")
	files := set.String("files", "", "comma-separated repo-relative files to mutate")
	workers := set.Int("workers", runtime.NumCPU(), "mutant test runs in parallel")
	update := set.Bool("update", false, "rewrite the exception list")
	flags, positional, err := splitArguments(args)
	if err != nil {
		return config{}, err
	}
	if err := set.Parse(flags); err != nil {
		return config{}, err
	}
	if *workers < 1 {
		return config{}, fmt.Errorf("-workers must be at least 1, got %d", *workers)
	}
	scope, err := parseScope(*files)
	if err != nil {
		return config{}, err
	}
	cfg := config{
		args:       positional,
		exceptions: *exceptions,
		scope:      scope,
		workers:    *workers,
		update:     *update,
		dir:        ".",
	}
	if len(cfg.args) == 0 {
		cfg.args = []string{"./internal/core", "./internal/tui"}
	}

	return cfg, nil
}

// splitArguments accepts flags before or after package arguments, so the script can append flags at the end.
func splitArguments(args []string) (flags, positional []string, err error) {
	for index := 0; index < len(args); index++ {
		arg := args[index]
		if !strings.HasPrefix(arg, "-") {
			positional = append(positional, arg)
			continue
		}
		name, _, hasValue := strings.Cut(arg, "=")
		if hasValue || !takesValue(name) {
			flags = append(flags, arg)
			continue
		}
		if index+1 >= len(args) {
			return nil, nil, fmt.Errorf("%s needs a value", name)
		}
		flags = append(flags, arg, args[index+1])
		index++
	}

	return flags, positional, nil
}

func takesValue(name string) bool {
	switch name {
	case "-exceptions", "-files", "-workers":
		return true
	}

	return false
}

func parseScope(files string) ([]string, error) {
	if files == "" {
		return nil, nil
	}
	var scope []string
	for _, name := range strings.Split(files, ",") {
		name = strings.TrimSpace(name)
		if name == "" {
			return nil, fmt.Errorf("-files has an empty entry")
		}
		scope = append(scope, filepath.ToSlash(name))
	}

	return scope, nil
}
