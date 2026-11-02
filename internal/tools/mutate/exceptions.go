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

// retiredStatus names the status column the format no longer carries, so its reuse fails with a useful message.
const (
	retiredStatus          = "untriaged"
	retiredJustifiedStatus = "justified"
	entryFormat            = "<identity><TAB><reason>"
)

// entry is one line of mutation/exceptions.txt: a mutant identity and the reason it may survive.
type entry struct {
	identity string
	reason   string
}

// exceptionsHeader documents the file's format for humans editing it.
const exceptionsHeader = `# Mutation-testing exceptions, one justified mutant per line.
#
# Format: <identity><TAB><reason>
# Every entry is a justified exception: the reason must show the mutant is equivalent, because no reachable input
# distinguishes it, or that pinning it would assert incidental detail, and it must be checkable against the code.
#
# Prefer killing the mutant with a test. If a reason is genuinely justified, add the entry by hand for Standards
# review. ./scripts/mutation.sh -update only removes entries whose mutant is now killed, not viable or no longer
# generated; it never records a new survivor.
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
	identityPart, reasonPart, found := strings.Cut(line, "\t")
	if !found {
		return entry{}, fmt.Errorf("want %s", entryFormat)
	}
	identity := strings.TrimSpace(identityPart)
	if identity == retiredStatus || identity == retiredJustifiedStatus {
		return entry{}, fmt.Errorf("the list is %s; the %q category is retired: kill the mutant or justify it",
			entryFormat, identity)
	}
	if identity == "" {
		return entry{}, fmt.Errorf("empty identity")
	}
	reason := strings.TrimSpace(reasonPart)
	if reason == "" {
		return entry{}, fmt.Errorf("entry %q needs a reason", identity)
	}

	return entry{identity: identity, reason: reason}, nil
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
