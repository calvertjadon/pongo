package main

import (
	"cmp"
	"fmt"
	"io"
	"slices"
)

// crap returns the change-risk score: complexity squared times uncovered chance cubed, plus complexity.
func crap(complexity int, coverage float64) float64 {
	uncovered := 1 - coverage
	squared := float64(complexity) * float64(complexity)

	return squared*uncovered*uncovered*uncovered + float64(complexity)
}

// report prints one line for every function over the ceiling, sorted by score, and returns the exit status.
func report(output io.Writer, functions []function, maximum float64) int {
	slices.SortFunc(functions, compareFunctions)
	offenders := 0
	for _, fn := range functions {
		score := crap(fn.complexity, fn.coverage())
		if score <= maximum {
			continue
		}
		_, _ = fmt.Fprintf(output, "%s:%d: %s complexity=%d coverage=%.1f%% crap=%.1f\n",
			fn.displayName, fn.line, fn.name, fn.complexity, fn.coverage()*100, score)
		offenders++
	}
	if offenders > 0 {
		return exitFailure
	}
	_, _ = fmt.Fprintln(output, summarize(functions))

	return exitSuccess
}

// compareFunctions orders functions by score descending, then by location and name for a stable report.
func compareFunctions(first, second function) int {
	firstScore := crap(first.complexity, first.coverage())
	secondScore := crap(second.complexity, second.coverage())
	if order := cmp.Compare(secondScore, firstScore); order != 0 {
		return order
	}
	if order := cmp.Compare(first.displayName, second.displayName); order != 0 {
		return order
	}
	if order := cmp.Compare(first.line, second.line); order != 0 {
		return order
	}

	return cmp.Compare(first.name, second.name)
}

// summarize describes how many functions were checked and the highest score.
func summarize(functions []function) string {
	if len(functions) == 0 {
		return "checked 0 functions"
	}
	highest := functions[0]

	return fmt.Sprintf("checked %d functions; highest crap %.1f in %s:%d (%s)",
		len(functions), crap(highest.complexity, highest.coverage()), highest.displayName, highest.line, highest.name)
}
