package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

// Snippets are normalized to one line so an identity survives reindentation and line wrapping. A snippet longer than
// maxSnippetRunes keeps its first snippetPrefix runes plus a hash of the full text, so two long snippets that share a
// prefix still get different identities.
const (
	maxSnippetRunes = 80
	snippetPrefix   = 56
)

// normalizeSnippet collapses whitespace to single spaces, trims it, and bounds the result.
func normalizeSnippet(text string) string {
	joined := strings.Join(strings.Fields(text), " ")
	runes := []rune(joined)
	if len(runes) <= maxSnippetRunes {
		return joined
	}
	digest := sha256.Sum256([]byte(joined))

	return string(runes[:snippetPrefix]) + "…" + hex.EncodeToString(digest[:4])
}

// makeIdentity names one mutant by where it lives and what it does, never by line number.
func makeIdentity(file, function, operator, original, mutated string, ordinal int) string {
	return fmt.Sprintf("%s|%s|%s|%s => %s|%d", file, function, operator, original, mutated, ordinal)
}

// assignOrdinals sorts mutants by source position and numbers identical mutant tuples in source order.
func assignOrdinals(mutants []*mutation) {
	sort.SliceStable(mutants, func(left, right int) bool {
		return mutants[left].offset < mutants[right].offset
	})
	seen := make(map[string]int, len(mutants))
	for _, mutant := range mutants {
		key := strings.Join(
			[]string{mutant.file, mutant.function, mutant.operator, mutant.original, mutant.mutated},
			"\x00",
		)
		seen[key]++
		mutant.identity = makeIdentity(
			mutant.file, mutant.function, mutant.operator, mutant.original, mutant.mutated, seen[key],
		)
	}
}

// entryFile returns the repo-relative file an exception identity names.
func entryFile(identity string) string {
	file, _, found := strings.Cut(identity, "|")
	if !found {
		return ""
	}

	return file
}
