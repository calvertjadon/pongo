package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestParseEntries(t *testing.T) {
	t.Parallel()
	document := `# a comment

untriaged	file.go|f|arithmetic|a + 1 => a - 1|1
justified	file.go|f|arithmetic|b + 1 => b - 1|1	the boundary is unreachable
`
	entries, err := parseEntries(strings.NewReader(document))
	if err != nil {
		t.Fatalf("parseEntries: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("parsed %d entries, want 2", len(entries))
	}
	if entries[0].status != statusUntriaged || entries[0].reason != "" {
		t.Fatalf("first entry = %+v, want an untriaged entry without a reason", entries[0])
	}
	if entries[1].status != statusJustified || entries[1].reason != "the boundary is unreachable" {
		t.Fatalf("second entry = %+v", entries[1])
	}
}

func TestParseEntriesRejectsMalformedLines(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		document string
		want     string
	}{
		{"missing tab", "untriaged-only-identity\n", "want <status>"},
		{"unknown status", "accepted\tidentity\t\n", "unknown status"},
		{"justified without reason", "justified\tidentity\t\n", "needs a reason"},
		{"empty identity", "untriaged\t\t\n", "empty identity"},
		{"duplicate", "untriaged\tidentity\t\nuntriaged\tidentity\t\n", "duplicate identity"},
	}
	for _, testCase := range cases {
		_, err := parseEntries(strings.NewReader(testCase.document))
		if err == nil || !strings.Contains(err.Error(), testCase.want) {
			t.Fatalf("%s: error = %v, want it to contain %q", testCase.name, err, testCase.want)
		}
	}
}

func TestWriteAndLoadEntries(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "nested", "exceptions.txt")
	entries := []entry{
		{status: statusUntriaged, identity: "b.go|f|arithmetic|x + 1 => x - 1|1"},
		{status: statusJustified, identity: "a.go|f|arithmetic|y + 1 => y - 1|1", reason: "cannot be observed"},
	}
	if err := writeEntries(path, entries); err != nil {
		t.Fatalf("writeEntries: %v", err)
	}
	loaded, err := loadEntries(path)
	if err != nil {
		t.Fatalf("loadEntries: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("loaded %d entries, want 2", len(loaded))
	}
	if loaded[0].identity != entries[1].identity || loaded[0].reason != "cannot be observed" {
		t.Fatalf("entries are not sorted by identity: %+v", loaded)
	}
	if loaded[1].status != statusUntriaged || loaded[1].reason != "" {
		t.Fatalf("round trip lost the untriaged entry: %+v", loaded[1])
	}
}

func TestLoadEntriesMissingFileIsEmpty(t *testing.T) {
	t.Parallel()
	entries, err := loadEntries(filepath.Join(t.TempDir(), "absent.txt"))
	if err != nil || entries != nil {
		t.Fatalf("loadEntries = %v, %v; want no entries and no error", entries, err)
	}
}

func TestLoadEntriesReportsParseErrors(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "exceptions.txt")
	if err := writeEntries(path, nil); err != nil {
		t.Fatalf("writeEntries: %v", err)
	}
	if err := appendToFile(path, "justified\tidentity\t\n"); err != nil {
		t.Fatalf("append: %v", err)
	}
	if _, err := loadEntries(path); err == nil || !strings.Contains(err.Error(), "needs a reason") {
		t.Fatalf("loadEntries error = %v, want a reason error", err)
	}
}

func TestWriteEntriesReportsFilesystemErrors(t *testing.T) {
	t.Parallel()
	parent := filepath.Join(t.TempDir(), "file")
	if err := writeFile(parent, "content"); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := writeEntries(filepath.Join(parent, "exceptions.txt"), nil); err == nil {
		t.Fatal("writeEntries under a file parent should fail")
	}
	directory := t.TempDir()
	if err := writeEntries(directory, nil); err == nil {
		t.Fatal("writeEntries on a directory should fail")
	}
}
