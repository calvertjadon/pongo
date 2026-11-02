package main

import (
	"strings"
	"testing"
)

func TestNormalizeSnippetCollapsesWhitespace(t *testing.T) {
	t.Parallel()
	got := normalizeSnippet("  x +\n\t y  ")
	if got != "x + y" {
		t.Fatalf("normalizeSnippet = %q, want %q", got, "x + y")
	}
}

func TestNormalizeSnippetBoundsLongTextWithStableHash(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("a", 100)
	first := normalizeSnippet(long)
	second := normalizeSnippet(long)
	if first != second {
		t.Fatalf("normalizeSnippet is not deterministic: %q != %q", first, second)
	}
	if runes := []rune(first); len(runes) != snippetPrefix+1+8 {
		t.Fatalf("truncated snippet has %d runes, want %d", len(runes), snippetPrefix+1+8)
	}
	almost := strings.Repeat("a", 99) + "b"
	if normalizeSnippet(almost) == first {
		t.Fatal("different long snippets share an identity snippet")
	}
}

func TestMakeIdentityFormat(t *testing.T) {
	t.Parallel()
	got := makeIdentity("internal/core/timing.go", "(*Model).Update", "call deletion", "f(x)", "x", 2)
	want := "internal/core/timing.go|(*Model).Update|call deletion|f(x) => x|2"
	if got != want {
		t.Fatalf("makeIdentity = %q, want %q", got, want)
	}
}

func TestAssignOrdinalsNumbersIdenticalMutantsInSourceOrder(t *testing.T) {
	t.Parallel()
	source := `package sample

func twice(x, y int) int {
	x = x + 1
	y = x + 1
	return y
}
`
	mutants := generateSource(t, source)
	first := findMutant(mutants, "arithmetic", "x + 1", "x - 1")
	if first == nil {
		t.Fatalf("missing arithmetic mutant in %v", describeMutants(mutants))
	}
	var identities []string
	for _, mutant := range mutants {
		if mutant.operator == "arithmetic" && mutant.original == "x + 1" {
			identities = append(identities, mutant.identity)
		}
	}
	if len(identities) != 2 {
		t.Fatalf("found %d identical mutants, want 2: %v", len(identities), identities)
	}
	if !strings.HasSuffix(identities[0], "|1") || !strings.HasSuffix(identities[1], "|2") {
		t.Fatalf("ordinals = %v, want |1 then |2", identities)
	}
}

// TestIdentitySurvivesUnrelatedEdit prepends code above the mutated function and checks every identity is unchanged.
func TestIdentitySurvivesUnrelatedEdit(t *testing.T) {
	t.Parallel()
	original := `package sample

func clamp(x, limit int) int {
	return min(max(x, 0), limit)
}
`
	retouched := `package sample

// helper is unrelated to the clamp below.
func helper(value int) int {
	return value * 2
}

var cache = map[int]int{}

func clamp(x, limit int) int {
	return min(max(x, 0), limit)
}
`
	before := identitiesOf(generateSource(t, original))
	after := identitiesOf(generateSource(t, retouched))
	if len(before) == 0 {
		t.Fatal("generated no mutants")
	}
	for identity := range before {
		if !after[identity] {
			t.Fatalf("identity %q changed after an unrelated edit (before %v, after %v)", identity, before, after)
		}
	}
}

func identitiesOf(mutants []*mutation) map[string]bool {
	identities := make(map[string]bool, len(mutants))
	for _, mutant := range mutants {
		identities[mutant.identity] = true
	}

	return identities
}

func TestClampDeletionIdentityNamesTheEnclosingCall(t *testing.T) {
	t.Parallel()
	source := `package sample

func admit(elapsed int) int {
	return min(max(elapsed, 0), 9000)
}
`
	mutants := generateSource(t, source)
	want := "sample.go|admit|call deletion|min(max(elapsed, 0), 9000) => min(elapsed, 9000)|1"
	if !identitiesOf(mutants)[want] {
		t.Fatalf("identity %q missing from %v", want, identitiesOf(mutants))
	}
}
