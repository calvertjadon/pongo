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

// updateEntries drops entries whose mutant is now dead or no longer generated, keeping every other entry and
// never adding a new one.
func updateEntries(entries []entry, mutants []*mutation, scope []string) []entry {
	index := mutantIndex(mutants)
	kept := make([]entry, 0, len(entries))
	for _, item := range entries {
		if keepEntry(item, index, scope) {
			kept = append(kept, item)
		}
	}
	sort.SliceStable(kept, func(left, right int) bool {
		return kept[left].identity < kept[right].identity
	})

	return kept
}

// keepEntry reports whether an entry still describes a live mutant, keeping out-of-scope entries untouched.
func keepEntry(item entry, index map[string]*mutation, scope []string) bool {
	if !inScope(scope, entryFile(item.identity)) {
		return true
	}
	mutant, found := index[item.identity]

	return found && !isDead(mutant.outcome)
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
		_, _ = fmt.Fprintln(output, "mutation testing: every surviving mutant is a justified exception")
		return exitSuccess
	}
	for _, mutant := range missing {
		_, _ = fmt.Fprintf(output, "%s: %s\n", outcomeLabel(mutant.outcome), mutant.identity)
		_, _ = fmt.Fprintf(output, "    location: %s:%d\n", mutant.file, mutant.line)
		_, _ = fmt.Fprintf(output, "    before: %s\n", mutant.original)
		_, _ = fmt.Fprintf(output, "    after:  %s\n", mutant.mutated)
	}
	if len(missing) > 0 {
		_, _ = fmt.Fprintf(output, "kill each mutant with a test, or add %s by hand for Standards review\n",
			entryFormat)
	}
	for _, item := range stale {
		location := ""
		if mutant, found := index[item.identity]; found {
			location = fmt.Sprintf(" (%s:%d)", mutant.file, mutant.line)
		}
		_, _ = fmt.Fprintf(output, "stale exception: %s%s\n", item.identity, location)
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
