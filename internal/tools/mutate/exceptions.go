package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// The two statuses an exception entry may carry.
const (
	statusUntriaged = "untriaged"
	statusJustified = "justified"
)

// entry is one line of mutation/exceptions.txt.
type entry struct {
	status   string
	identity string
	reason   string
}

// exceptionsHeader documents the file's format for humans editing it.
const exceptionsHeader = `# Mutation-testing exceptions, one mutant per line.
#
# Format: <status><TAB><identity><TAB><reason>
#   untriaged  nobody has judged this mutant yet.
#   justified  the reason explains why this mutant may survive.
#
# Regenerate survivors with ./scripts/mutation.sh -update. Never delete an entry merely to make the job pass:
# kill the mutant with a test, or justify it with a reason.
`

// parseEntries reads the line-oriented exception list and rejects ambiguous or malformed entries.
func parseEntries(reader io.Reader) ([]entry, error) {
	scanner := bufio.NewScanner(reader)
	seen := make(map[string]bool)
	var entries []entry
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		parsed, err := parseEntryLine(line)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNumber, err)
		}
		if seen[parsed.identity] {
			return nil, fmt.Errorf("line %d: duplicate identity %q", lineNumber, parsed.identity)
		}
		seen[parsed.identity] = true
		entries = append(entries, parsed)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read exceptions: %w", err)
	}

	return entries, nil
}

func parseEntryLine(line string) (entry, error) {
	fields := strings.Split(line, "\t")
	if len(fields) < 2 {
		return entry{}, fmt.Errorf("want <status><TAB><identity><TAB><reason>")
	}
	status := strings.TrimSpace(fields[0])
	identity := strings.TrimSpace(fields[1])
	reason := ""
	if len(fields) > 2 {
		reason = strings.TrimSpace(strings.Join(fields[2:], " "))
	}
	if status != statusUntriaged && status != statusJustified {
		return entry{}, fmt.Errorf("unknown status %q", status)
	}
	if identity == "" {
		return entry{}, fmt.Errorf("empty identity")
	}
	if status == statusJustified && reason == "" {
		return entry{}, fmt.Errorf("justified entry %q needs a reason", identity)
	}

	return entry{status: status, identity: identity, reason: reason}, nil
}

// loadEntries reads the exception list, treating a missing file as an empty list.
func loadEntries(path string) ([]entry, error) {
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open exceptions: %w", err)
	}
	defer func() { _ = file.Close() }()
	entries, err := parseEntries(file)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	return entries, nil
}

// writeEntries rewrites the list sorted by identity so each file's entries form one block.
func writeEntries(path string, entries []entry) error {
	sorted := append([]entry(nil), entries...)
	sort.SliceStable(sorted, func(left, right int) bool {
		return sorted[left].identity < sorted[right].identity
	})
	var document strings.Builder
	document.WriteString(exceptionsHeader)
	for _, item := range sorted {
		document.WriteString(item.status)
		document.WriteByte('\t')
		document.WriteString(item.identity)
		document.WriteByte('\t')
		document.WriteString(item.reason)
		document.WriteByte('\n')
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create exceptions directory: %w", err)
	}
	if err := os.WriteFile(path, []byte(document.String()), 0o644); err != nil {
		return fmt.Errorf("write exceptions: %w", err)
	}

	return nil
}
