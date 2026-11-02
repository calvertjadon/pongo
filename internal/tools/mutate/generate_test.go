package main

import (
	"strings"
	"testing"
)

// generateSource generates mutants for one in-memory source file.
func generateSource(t *testing.T, source string) []*mutation {
	t.Helper()
	types, err := collectTypeNames(map[string][]byte{"sample.go": []byte(source)})
	if err != nil {
		t.Fatalf("collect type names: %v", err)
	}
	mutants, err := generateFile("sample.go", "/tmp/sample.go", []byte(source), types)
	if err != nil {
		t.Fatalf("generate mutants: %v", err)
	}

	return mutants
}

func findMutant(mutants []*mutation, operator, original, mutated string) *mutation {
	for _, mutant := range mutants {
		if mutant.operator == operator && mutant.original == original && mutant.mutated == mutated {
			return mutant
		}
	}

	return nil
}

func requireMutant(t *testing.T, mutants []*mutation, operator, original, mutated string) *mutation {
	t.Helper()
	mutant := findMutant(mutants, operator, original, mutated)
	if mutant == nil {
		t.Fatalf("no %s mutant %q => %q in %v", operator, original, mutated, describeMutants(mutants))
	}

	return mutant
}

func describeMutants(mutants []*mutation) []string {
	described := make([]string, 0, len(mutants))
	for _, mutant := range mutants {
		described = append(described, mutant.operator+":"+mutant.original+" => "+mutant.mutated)
	}

	return described
}

const sampleSource = `package sample

func clamp(x, y, limit int) int {
	if x < 0 || y > limit {
		x++
	}
	y = min(max(x, 0), limit)
	if x == y {
		return -x
	}

	return y
}
`

func TestGeneratesEveryOperator(t *testing.T) {
	t.Parallel()
	mutants := generateSource(t, sampleSource)
	cases := []struct{ operator, original, mutated string }{
		{"conditional boundary", "x < 0", "x <= 0"},
		{"conditional negation", "x < 0", "x >= 0"},
		{"logical", "x < 0 || y > limit", "x < 0 && y > limit"},
		{"conditional boundary", "y > limit", "y >= limit"},
		{"conditional negation", "y > limit", "y <= limit"},
		{"increment swap", "x++", "x--"},
		{"call deletion", "min(max(x, 0), limit)", "max(x, 0)"},
		{"call deletion", "min(max(x, 0), limit)", "limit"},
		{"call deletion", "min(max(x, 0), limit)", "min(x, limit)"},
		{"call deletion", "min(max(x, 0), limit)", "min(0, limit)"},
		{"statement deletion", "y = min(max(x, 0), limit)", "<deleted>"},
		{"statement deletion", "x++", "<deleted>"},
		{"conditional negation", "x == y", "x != y"},
		{"negative inversion", "-x", "x"},
	}
	for _, testCase := range cases {
		requireMutant(t, mutants, testCase.operator, testCase.original, testCase.mutated)
	}
}

func TestAssignmentArithmetic(t *testing.T) {
	t.Parallel()
	source := "package sample\n\nfunc add(x int) int {\n\tx += 2\n\treturn x\n}\n"
	mutants := generateSource(t, source)
	requireMutant(t, mutants, "assignment arithmetic", "x += 2", "x -= 2")

	multiply := "package sample\n\nfunc scale(x int) int {\n\tx *= 3\n\treturn x\n}\n"
	requireMutant(t, generateSource(t, multiply), "assignment arithmetic", "x *= 3", "x /= 3")
}

func TestBooleanLiteralFlip(t *testing.T) {
	t.Parallel()
	source := "package sample\n\nfunc flag() bool {\n\ton := false\n\treturn on\n}\n"
	requireMutant(t, generateSource(t, source), "boolean literal", "false", "true")
}

func TestStatementDeletionSkipsDeclarationsAndReturns(t *testing.T) {
	t.Parallel()
	source := "package sample\n\nfunc value() int {\n\tx := 1\n\ty := x\n\t_ = y\n\treturn x\n}\n"
	mutants := generateSource(t, source)
	for _, mutant := range mutants {
		if mutant.operator != "statement deletion" {
			continue
		}
		if strings.HasPrefix(mutant.original, "x := ") || strings.HasPrefix(mutant.original, "y := ") ||
			strings.HasPrefix(mutant.original, "return ") {
			t.Fatalf("deleted a statement that should be excluded: %q", mutant.original)
		}
	}
}

func TestFunctionNamesIncludeReceivers(t *testing.T) {
	t.Parallel()
	source := `package sample

type Model struct {
	value int
}

func (model *Model) Update() {
	model.value++
}

func free(value int) int {
	return value + 1
}
`
	mutants := generateSource(t, source)
	foundReceiver := false
	foundFree := false
	for _, mutant := range mutants {
		if mutant.function == "(*Model).Update" {
			foundReceiver = true
		}
		if mutant.function == "free" {
			foundFree = true
		}
	}
	if !foundReceiver || !foundFree {
		t.Fatalf("function names = %v, want both (*Model).Update and free", describeMutants(mutants))
	}
}

func TestPackageLevelMutationUsesPackageName(t *testing.T) {
	t.Parallel()
	source := "package sample\n\nvar limit = 2 * 3\n"
	mutants := generateSource(t, source)
	mutant := requireMutant(t, mutants, "arithmetic", "2 * 3", "2 / 3")
	if mutant.function != "package" {
		t.Fatalf("function = %q, want package", mutant.function)
	}
}

func TestCallDeletionSkipsTypeConversions(t *testing.T) {
	t.Parallel()
	source := "package sample\n\ntype Duration int\n\nfunc convert(x int) Duration {\n\treturn Duration(x)\n}\n"
	if findMutant(generateSource(t, source), "call deletion", "Duration(x)", "x") != nil {
		t.Fatal("a type conversion was treated as a call")
	}
}

func TestMutantRangeReplacesOnlyTheNode(t *testing.T) {
	t.Parallel()
	source := "package sample\n\nfunc add(x int) int {\n\treturn x + 1\n}\n"
	mutant := requireMutant(t, generateSource(t, source), "arithmetic", "x + 1", "x - 1")
	mutated := applyMutation(mutant.src, mutant.offset, mutant.end, mutant.replacement)
	if !strings.Contains(string(mutated), "return x - 1") {
		t.Fatalf("mutated source = %q, want it to contain the replacement", mutated)
	}
}
