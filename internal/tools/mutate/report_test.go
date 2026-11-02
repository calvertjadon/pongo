package main

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func makeMutant(file, suffix string, result outcome) *mutation {
	return &mutation{
		identity: file + "|" + suffix,
		file:     file,
		line:     10,
		original: "before",
		mutated:  "after",
		outcome:  result,
	}
}

func identities(mutants []*mutation) map[string]bool {
	found := make(map[string]bool, len(mutants))
	for _, mutant := range mutants {
		found[mutant.identity] = true
	}

	return found
}

func TestEvaluateFindsUnlistedSurvivorsAndStaleEntries(t *testing.T) {
	t.Parallel()
	mutants := []*mutation{
		makeMutant("a.go", "f|arithmetic|x + 1 => x - 1|1", outcomeSurvived),
		makeMutant("a.go", "f|arithmetic|y + 1 => y - 1|1", outcomeSurvived),
		makeMutant("a.go", "f|statement deletion|z++ => <deleted>|1", outcomeNeverExecuted),
		makeMutant("a.go", "f|arithmetic|k + 1 => k - 1|1", outcomeKilled),
		makeMutant("a.go", "f|arithmetic|n + 1 => n - 1|1", outcomeNotViable),
	}
	entries := []entry{
		{identity: "a.go|f|arithmetic|y + 1 => y - 1|1", reason: "justified reason"},
		{identity: "a.go|f|arithmetic|k + 1 => k - 1|1", reason: "justified reason"},
		{identity: "a.go|f|arithmetic|n + 1 => n - 1|1", reason: "justified reason"},
		{identity: "a.go|f|arithmetic|gone => gone|1", reason: "justified reason"},
	}
	missing, stale := evaluate(mutants, entries, nil)
	if len(missing) != 2 {
		t.Fatalf("missing = %v, want 2 entries", identities(missing))
	}
	for _, wanted := range []string{
		"a.go|f|arithmetic|x + 1 => x - 1|1",
		"a.go|f|statement deletion|z++ => <deleted>|1",
	} {
		if !identities(missing)[wanted] {
			t.Fatalf("missing does not contain %q", wanted)
		}
	}
	if len(stale) != 3 {
		t.Fatalf("stale = %v, want 3 entries", stale)
	}
}

func TestEvaluateHonoursScope(t *testing.T) {
	t.Parallel()
	mutants := []*mutation{
		makeMutant("a.go", "f|arithmetic|x + 1 => x - 1|1", outcomeSurvived),
		makeMutant("b.go", "f|arithmetic|y + 1 => y - 1|1", outcomeSurvived),
	}
	entries := []entry{
		{identity: "a.go|f|arithmetic|gone => gone|1", reason: "justified reason"},
		{identity: "b.go|f|arithmetic|gone => gone|1", reason: "justified reason"},
	}
	missing, stale := evaluate(mutants, entries, []string{"a.go"})
	if len(missing) != 1 || len(stale) != 1 {
		t.Fatalf("missing = %v, stale = %v; want one each", identities(missing), stale)
	}
	if missing[0].file != "a.go" || entryFile(stale[0].identity) != "a.go" {
		t.Fatalf("scope leaked: missing %v, stale %v", missing, stale)
	}
}

func TestRemoveStaleEntriesDropsDeadEntriesAndKeepsLiveOnes(t *testing.T) {
	t.Parallel()
	mutants := []*mutation{
		makeMutant("a.go", "f|arithmetic|kept => kept|1", outcomeSurvived),
		makeMutant("a.go", "f|arithmetic|new => new|1", outcomeSurvived),
		makeMutant("a.go", "f|statement deletion|never => <deleted>|1", outcomeNeverExecuted),
		makeMutant("a.go", "f|arithmetic|killed => killed|1", outcomeKilled),
		makeMutant("a.go", "f|arithmetic|broken => broken|1", outcomeNotViable),
	}
	entries := []entry{
		{identity: "a.go|f|arithmetic|kept => kept|1", reason: "no reachable input distinguishes it"},
		{identity: "a.go|f|arithmetic|killed => killed|1", reason: "stale because the mutant is killed"},
		{identity: "a.go|f|arithmetic|broken => broken|1", reason: "stale because the mutant is not viable"},
		{identity: "a.go|f|arithmetic|dead => dead|1", reason: "stale because the mutant is gone"},
	}
	updated := updateEntries(entries, mutants, nil)
	if len(updated) != 1 {
		t.Fatalf("updated = %+v, want only the live entry", updated)
	}
	if updated[0].identity != "a.go|f|arithmetic|kept => kept|1" ||
		updated[0].reason != "no reachable input distinguishes it" {
		t.Fatalf("kept entry lost its identity or reason: %+v", updated[0])
	}
}

func TestRemoveStaleEntriesNeverAddsUnlistedSurvivors(t *testing.T) {
	t.Parallel()
	mutants := []*mutation{
		makeMutant("a.go", "f|arithmetic|new => new|1", outcomeSurvived),
		makeMutant("a.go", "f|statement deletion|never => <deleted>|1", outcomeNeverExecuted),
	}
	updated := updateEntries(nil, mutants, nil)
	if len(updated) != 0 {
		t.Fatalf("updated = %+v, want no entries because -update must not record survivors", updated)
	}
}

func TestRemoveStaleEntriesLeavesOtherFilesAloneUnderScope(t *testing.T) {
	t.Parallel()
	mutants := []*mutation{makeMutant("b.go", "f|arithmetic|x + 1 => x - 1|1", outcomeKilled)}
	entries := []entry{{identity: "b.go|f|arithmetic|old => old|1", reason: "justified reason"}}
	updated := updateEntries(entries, mutants, []string{"a.go"})
	if len(updated) != 1 || updated[0].identity != "b.go|f|arithmetic|old => old|1" {
		t.Fatalf("out-of-scope entry changed: %+v", updated)
	}
}

func TestSummarizeCountsEveryOutcome(t *testing.T) {
	t.Parallel()
	first := &pkgInfo{importPath: "first"}
	second := &pkgInfo{importPath: "second"}
	orphan := &pkgInfo{importPath: "orphan"}
	mutants := []*mutation{
		{pkg: first, outcome: outcomeKilled},
		{pkg: first, outcome: outcomeSurvived},
		{pkg: second, outcome: outcomeNeverExecuted},
		{pkg: second, outcome: outcomeTimedOut},
		{pkg: second, outcome: outcomeNotViable},
		{pkg: second, outcome: outcomePending},
		{pkg: orphan, outcome: outcomeKilled},
	}
	counts := summarize([]*pkgInfo{first, second}, mutants)
	if counts["first"].killed != 1 || counts["first"].survived != 1 {
		t.Fatalf("first counts = %+v", counts["first"])
	}
	if counts["second"].neverExecuted != 1 || counts["second"].timedOut != 1 || counts["second"].notViable != 1 {
		t.Fatalf("second counts = %+v", counts["second"])
	}
	if counts["second"].killed != 0 {
		t.Fatalf("pending mutant was counted as killed: %+v", counts["second"])
	}
	if counts["orphan"] != nil {
		t.Fatal("summarize invented a package row")
	}
}

func TestPrintSummaryTable(t *testing.T) {
	t.Parallel()
	pkg := &pkgInfo{importPath: "example.com/demo/internal/core"}
	var output bytes.Buffer
	printSummary(&output, []*pkgInfo{pkg}, []*mutation{{pkg: pkg, outcome: outcomeKilled}}, 1500*time.Millisecond)
	text := output.String()
	for _, wanted := range []string{"package", "killed", "never executed", "timed out", "not viable",
		"example.com/demo/internal/core", "mutation testing: 1 mutant(s) in 1.5s"} {
		if !strings.Contains(text, wanted) {
			t.Fatalf("summary %q does not contain %q", text, wanted)
		}
	}
}

func TestReportGatePassesAndFails(t *testing.T) {
	t.Parallel()
	t.Run("pass", func(t *testing.T) {
		t.Parallel()
		var output bytes.Buffer
		if status := reportGate(&output, nil, nil, nil); status != exitSuccess {
			t.Fatalf("status = %d, want %d", status, exitSuccess)
		}
		if !strings.Contains(output.String(), "every surviving mutant is a justified exception") {
			t.Fatalf("pass message = %q", output.String())
		}
	})
	t.Run("fail", func(t *testing.T) {
		t.Parallel()
		survivor := makeMutant("a.go", "f|arithmetic|x + 1 => x - 1|1", outcomeSurvived)
		never := makeMutant("a.go", "f|statement deletion|z++ => <deleted>|1", outcomeNeverExecuted)
		stale := entry{identity: survivor.identity, reason: "this reason no longer holds"}
		unknown := entry{identity: "a.go|f|arithmetic|gone => gone|1", reason: "justified reason"}
		var output bytes.Buffer
		status := reportGate(&output, []*mutation{survivor, never}, []entry{stale, unknown},
			map[string]*mutation{survivor.identity: survivor})
		if status != exitFailure {
			t.Fatalf("status = %d, want %d", status, exitFailure)
		}
		for _, wanted := range []string{
			"survived: " + survivor.identity,
			"never-executed: " + never.identity,
			"location: a.go:10",
			"before: before",
			"after:  after",
			"kill each mutant with a test, or add <identity><TAB><reason> by hand for Standards review",
			"stale exception: " + survivor.identity + " (a.go:10)",
			"stale exception: " + unknown.identity,
			"2 unlisted mutant(s), 2 stale exception(s)",
		} {
			if !strings.Contains(output.String(), wanted) {
				t.Fatalf("report %q does not contain %q", output.String(), wanted)
			}
		}
	})
	t.Run("stale only", func(t *testing.T) {
		t.Parallel()
		stale := entry{identity: "a.go|f|arithmetic|gone => gone|1", reason: "the mutant is gone"}
		var output bytes.Buffer
		if status := reportGate(&output, nil, []entry{stale}, nil); status != exitFailure {
			t.Fatalf("status = %d, want %d", status, exitFailure)
		}
		if strings.Contains(output.String(), "kill each mutant") {
			t.Fatalf("stale-only report offered advice for a missing mutant: %q", output.String())
		}
		if !strings.Contains(output.String(), "stale exception: "+stale.identity) {
			t.Fatalf("stale-only report = %q", output.String())
		}
	})
}

func TestIsDeadAndOutcomeLabel(t *testing.T) {
	t.Parallel()
	dead := map[outcome]bool{
		outcomePending: false, outcomeKilled: true, outcomeSurvived: false, outcomeNeverExecuted: false,
		outcomeTimedOut: true, outcomeNotViable: true,
	}
	for result, want := range dead {
		if got := isDead(result); got != want {
			t.Fatalf("isDead(%v) = %t, want %t", result, got, want)
		}
	}
	if outcomeLabel(outcomeSurvived) != "survived" || outcomeLabel(outcomeNeverExecuted) != "never-executed" {
		t.Fatal("outcomeLabel returned an unexpected label")
	}
}

func TestEntryFile(t *testing.T) {
	t.Parallel()
	if got := entryFile("internal/core/a.go|f|arithmetic|x => y|1"); got != "internal/core/a.go" {
		t.Fatalf("entryFile = %q", got)
	}
	if got := entryFile("no-separator"); got != "" {
		t.Fatalf("entryFile = %q, want empty", got)
	}
}
