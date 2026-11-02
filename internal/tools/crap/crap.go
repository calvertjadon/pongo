// Package main checks that every function in the module stays at or below the CRAP ceiling.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// Process exit statuses.
const (
	exitSuccess = 0
	exitFailure = 1
	exitUsage   = 2
)

// defaultMaximum is the CRAP ceiling. It is the flag default, so the ceiling lives here and nowhere else.
const defaultMaximum = 12.0

// packageTemplate asks go list for each package's import path, directory, and compiled non-test files.
const packageTemplate = `{{.ImportPath}}|{{.Dir}}|{{range .GoFiles}}{{.}} {{end}}`

// configuration holds the ceiling and the coverage profiles to merge.
type configuration struct {
	maximum  float64
	profiles []string
}

func main() {
	os.Exit(execute(os.Args[1:], os.Stdout, os.Stderr, listPackages))
}

// execute parses the command line, measures every function, and reports the functions over the ceiling.
func execute(arguments []string, output, errorOutput io.Writer, list func() ([]sourceFile, error)) int {
	config, err := parseArguments(arguments)
	if err != nil {
		_, _ = fmt.Fprintf(errorOutput, "crap: %v\nusage: crap [-max %g] <profile>...\n", err, defaultMaximum)
		return exitUsage
	}
	files, err := list()
	if err != nil {
		_, _ = fmt.Fprintf(errorOutput, "crap: %v\n", err)
		return exitFailure
	}
	functions, err := measure(config, files)
	if err != nil {
		_, _ = fmt.Fprintf(errorOutput, "crap: %v\n", err)
		return exitFailure
	}

	return report(output, functions, config.maximum)
}

// parseArguments accepts an optional -max value followed by at least one coverage profile.
func parseArguments(arguments []string) (configuration, error) {
	config := configuration{maximum: defaultMaximum}
	for index := 0; index < len(arguments); index++ {
		argument := arguments[index]
		switch {
		case argument == "-max":
			index++
			if index == len(arguments) {
				return config, errors.New("-max needs a value")
			}
			if err := config.setMaximum(arguments[index]); err != nil {
				return config, err
			}
		case strings.HasPrefix(argument, "-max="):
			if err := config.setMaximum(strings.TrimPrefix(argument, "-max=")); err != nil {
				return config, err
			}
		case strings.HasPrefix(argument, "-"):
			return config, fmt.Errorf("unknown flag %q", argument)
		default:
			config.profiles = append(config.profiles, argument)
		}
	}
	if len(config.profiles) == 0 {
		return config, errors.New("at least one coverage profile is required")
	}

	return config, nil
}

// setMaximum parses and stores a ceiling value.
func (config *configuration) setMaximum(value string) error {
	maximum, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fmt.Errorf("invalid -max value %q: %w", value, err)
	}
	config.maximum = maximum

	return nil
}

// listPackages lists every compiled file in the module's packages, excluding the nested tools module.
func listPackages() ([]sourceFile, error) {
	workingDirectory, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("working directory: %w", err)
	}
	output, err := exec.Command("go", "list", "-f", packageTemplate, "./...").Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return nil, fmt.Errorf("go list: %w: %s", err, strings.TrimSpace(string(exit.Stderr)))
		}

		return nil, fmt.Errorf("go list: %w", err)
	}

	return parsePackageList(output, workingDirectory)
}

// parsePackageList converts go list output into the module's source files.
func parsePackageList(output []byte, workingDirectory string) ([]sourceFile, error) {
	var files []sourceFile
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.SplitN(line, "|", 3)
		if len(fields) != 3 {
			return nil, fmt.Errorf("unexpected go list line %q", line)
		}
		for _, name := range strings.Fields(fields[2]) {
			if strings.HasSuffix(name, "_test.go") {
				continue
			}
			path := filepath.Join(fields[1], name)
			files = append(files, sourceFile{
				path:         path,
				coverageName: fields[0] + "/" + name,
				displayName:  displayPath(workingDirectory, path),
			})
		}
	}

	return files, nil
}

// displayPath returns path relative to the working directory when it can.
func displayPath(workingDirectory, path string) string {
	relative, err := filepath.Rel(workingDirectory, path)
	if err != nil {
		return path
	}

	return relative
}

// measure parses every source file and attributes the merged profile coverage to each function.
func measure(config configuration, files []sourceFile) ([]function, error) {
	profile, err := loadCoverage(config.profiles)
	if err != nil {
		return nil, err
	}
	functions := make([]function, 0, len(files))
	for _, file := range files {
		declarations, err := parseFunctions(file)
		if err != nil {
			return nil, err
		}
		functions = append(functions, declarations...)
	}
	attribute(functions, profile)

	return functions, nil
}
