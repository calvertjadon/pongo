package main

import (
	"fmt"
	"io"
	"sort"
	"time"
)

// pkgCounts is one row of the summary table.
type pkgCounts struct {
	killed        int
	survived      int
	neverExecuted int
	timedOut      int
	notViable     int
}

func mutantIndex(mutants []*mutation) map[string]*mutation {
	index := make(map[string]*mutation, len(mutants))
	for _, mutant := range mutants {
		index[mutant.identity] = mutant
	}

	return index
}

// isDead reports whether a mutant can no longer justify an exception entry.
func isDead(result outcome) bool {
	return result == outcomeKilled || result == outcomeTimedOut || result == outcomeNotViable
}

// evaluate returns the unlisted surviving mutants and the exception entries that no longer match a live mutant.
func evaluate(mutants []*mutation, entries []entry, scope []string) (missing []*mutation, stale []entry) {
	index := mutantIndex(mutants)
	listed := make(map[string]bool, len(entries))
	for _, item := range entries {
		listed[item.identity] = true
	}
	for _, mutant := range mutants {
		if !inScope(scope, mutant.file) {
			continue
		}
		if mutant.outcome != outcomeSurvived && mutant.outcome != outcomeNeverExecuted {
			continue
		}
		if !listed[mutant.identity] {
			missing = append(missing, mutant)
		}
	}
	for _, item := range entries {
		if !inScope(scope, entryFile(item.identity)) {
			continue
		}
		mutant, found := index[item.identity]
		if !found || isDead(mutant.outcome) {
			stale = append(stale, item)
		}
	}
	sort.SliceStable(missing, func(left, right int) bool {
		return missing[left].identity < missing[right].identity
	})
	sort.SliceStable(stale, func(left, right int) bool {
		return stale[left].identity < stale[right].identity
	})

	return missing, stale
}

// updateEntries keeps justified entries and live untriaged ones, drops dead entries, and records new survivors.
func updateEntries(entries []entry, mutants []*mutation, scope []string) []entry {
	index := mutantIndex(mutants)
	existing := make(map[string]bool, len(entries))
	for _, item := range entries {
		existing[item.identity] = true
	}
	updated := keepEntries(entries, index, scope)
	updated = append(updated, newEntries(mutants, existing, scope)...)
	sort.SliceStable(updated, func(left, right int) bool {
		return updated[left].identity < updated[right].identity
	})

	return updated
}

func keepEntries(entries []entry, index map[string]*mutation, scope []string) []entry {
	kept := make([]entry, 0, len(entries))
	for _, item := range entries {
		if keepEntry(item, index, scope) {
			kept = append(kept, item)
		}
	}

	return kept
}

// keepEntry reports whether an existing entry still describes a live mutant, keeping out-of-scope and justified ones.
func keepEntry(item entry, index map[string]*mutation, scope []string) bool {
	if !inScope(scope, entryFile(item.identity)) || item.status == statusJustified {
		return true
	}
	mutant, found := index[item.identity]

	return found && !isDead(mutant.outcome)
}

func newEntries(mutants []*mutation, existing map[string]bool, scope []string) []entry {
	var added []entry
	for _, mutant := range mutants {
		if !inScope(scope, mutant.file) || existing[mutant.identity] {
			continue
		}
		if mutant.outcome != outcomeSurvived && mutant.outcome != outcomeNeverExecuted {
			continue
		}
		added = append(added, entry{status: statusUntriaged, identity: mutant.identity})
	}

	return added
}

func summarize(packages []*pkgInfo, mutants []*mutation) map[string]*pkgCounts {
	counts := make(map[string]*pkgCounts, len(packages))
	for _, pkg := range packages {
		counts[pkg.importPath] = &pkgCounts{}
	}
	for _, mutant := range mutants {
		row := counts[mutant.pkg.importPath]
		if row == nil {
			continue
		}
		switch mutant.outcome {
		case outcomeKilled:
			row.killed++
		case outcomeSurvived:
			row.survived++
		case outcomeNeverExecuted:
			row.neverExecuted++
		case outcomeTimedOut:
			row.timedOut++
		case outcomeNotViable:
			row.notViable++
		case outcomePending:
			// A finished run has no pending mutants; leave the row alone if one appears.
		}
	}

	return counts
}

func printSummary(output io.Writer, packages []*pkgInfo, mutants []*mutation, elapsed time.Duration) {
	counts := summarize(packages, mutants)
	_, _ = fmt.Fprintf(output, "%-46s %7s %9s %15s %10s %11s\n",
		"package", "killed", "survived", "never executed", "timed out", "not viable")
	for _, pkg := range packages {
		row := counts[pkg.importPath]
		if row == nil {
			continue
		}
		_, _ = fmt.Fprintf(output, "%-46s %7d %9d %15d %10d %11d\n", pkg.importPath,
			row.killed, row.survived, row.neverExecuted, row.timedOut, row.notViable)
	}
	_, _ = fmt.Fprintf(output, "mutation testing: %d mutant(s) in %s\n",
		len(mutants), elapsed.Round(10*time.Millisecond))
}

// reportGate prints every offender and returns the gate's exit status.
func reportGate(output io.Writer, missing []*mutation, stale []entry, index map[string]*mutation) int {
	if len(missing) == 0 && len(stale) == 0 {
		_, _ = fmt.Fprintln(output, "mutation testing: every surviving mutant is listed or justified")
		return exitSuccess
	}
	for _, mutant := range missing {
		_, _ = fmt.Fprintf(output, "%s: %s\n", outcomeLabel(mutant.outcome), mutant.identity)
		_, _ = fmt.Fprintf(output, "    location: %s:%d\n", mutant.file, mutant.line)
		_, _ = fmt.Fprintf(output, "    before: %s\n", mutant.original)
		_, _ = fmt.Fprintf(output, "    after:  %s\n", mutant.mutated)
	}
	for _, item := range stale {
		location := ""
		if mutant, found := index[item.identity]; found {
			location = fmt.Sprintf(" (%s:%d)", mutant.file, mutant.line)
		}
		_, _ = fmt.Fprintf(output, "stale exception: %s [%s]%s\n", item.identity, item.status, location)
	}
	_, _ = fmt.Fprintf(output, "mutation testing failed: %d unlisted mutant(s), %d stale exception(s)\n",
		len(missing), len(stale))

	return exitFailure
}

func outcomeLabel(result outcome) string {
	if result == outcomeSurvived {
		return "survived"
	}

	return "never-executed"
}
