#!/bin/sh
# Run the complete automated gate. Every step exits non-zero on failure.
set -eu

cd "$(CDPATH= cd -- "$(dirname "$0")" && pwd)/.."

# Run on the toolchain CI pins. The pinned golangci-lint cannot read the export data of newer Go patch releases,
# and Go downloads this toolchain on first use.
export GOTOOLCHAIN=go1.27.0

echo "==> format check"
# fmt --diff exits 0 even when it prints a diff, so any output is a failure.
if ! diff_output=$(go tool -modfile=tools/go.mod golangci-lint fmt --diff --config=.golangci.yml); then
	printf '%s\n' "$diff_output"
	echo "format check failed: the formatter exited non-zero" >&2
	exit 1
fi
if [ -n "$diff_output" ]; then
	printf '%s\n' "$diff_output"
	echo "format check failed: the formatter printed a diff" >&2
	exit 1
fi

echo "==> go mod tidy -diff"
go mod tidy -diff

echo "==> go mod tidy -diff (tools)"
(cd tools && go mod tidy -diff)

echo "==> go vet ./..."
go vet ./...

echo "==> go tool golangci-lint run"
go tool -modfile=tools/go.mod golangci-lint run --config=.golangci.yml ./...

echo "==> go test -race -cover ./..."
go test -race -cover ./...

echo "==> go build ./..."
go build ./...
