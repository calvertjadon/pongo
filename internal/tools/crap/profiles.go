package main

import (
	"fmt"
	"os"
	"strings"
)

// blockKey identifies one coverage block by its source range, so the same block can be merged across profiles.
type blockKey struct {
	file      string
	startLine int
	startCol  int
	endLine   int
	endCol    int
}

// coverageProfile merges statement blocks from every coverage profile given to the checker.
type coverageProfile struct {
	statements map[blockKey]int
	covered    map[blockKey]bool
}

// newCoverageProfile returns an empty merged profile.
func newCoverageProfile() coverageProfile {
	return coverageProfile{
		statements: make(map[blockKey]int),
		covered:    make(map[blockKey]bool),
	}
}

// add records one block, keeping it covered when any profile reports it executed. All profiles instrument the same
// source, so the first statement count seen for a block is its count.
func (profile coverageProfile) add(key blockKey, statements int, count int64) {
	if _, seen := profile.statements[key]; !seen {
		profile.statements[key] = statements
	}
	if count > 0 {
		profile.covered[key] = true
	}
}

// loadCoverage merges every text-format coverage profile named by paths.
func loadCoverage(paths []string) (coverageProfile, error) {
	profile := newCoverageProfile()
	for _, path := range paths {
		if err := profile.read(path); err != nil {
			return profile, err
		}
	}

	return profile, nil
}

// read merges the profile at path into this one.
func (profile coverageProfile) read(path string) error {
	contents, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read coverage profile: %w", err)
	}
	for number, line := range strings.Split(string(contents), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "mode:") {
			continue
		}
		key, statements, count, err := parseBlock(line)
		if err != nil {
			return fmt.Errorf("%s:%d: %w", path, number+1, err)
		}
		profile.add(key, statements, count)
	}

	return nil
}

// parseBlock parses one block: file:startLine.startCol,endLine.endCol statements count.
func parseBlock(line string) (blockKey, int, int64, error) {
	colon := strings.LastIndexByte(line, ':')
	if colon < 0 {
		return blockKey{}, 0, 0, fmt.Errorf("malformed coverage block %q", line)
	}
	positions, counts, found := strings.Cut(line[colon+1:], " ")
	if !found {
		return blockKey{}, 0, 0, fmt.Errorf("malformed coverage block %q", line)
	}
	var startLine, startCol, endLine, endCol int
	if _, err := fmt.Sscanf(positions, "%d.%d,%d.%d", &startLine, &startCol, &endLine, &endCol); err != nil {
		return blockKey{}, 0, 0, fmt.Errorf("coverage block range: %w", err)
	}
	var statements int
	var count int64
	if _, err := fmt.Sscanf(counts, "%d %d", &statements, &count); err != nil {
		return blockKey{}, 0, 0, fmt.Errorf("coverage block counts: %w", err)
	}
	key := blockKey{
		file:      line[:colon],
		startLine: startLine,
		startCol:  startCol,
		endLine:   endLine,
		endCol:    endCol,
	}

	return key, statements, count, nil
}
