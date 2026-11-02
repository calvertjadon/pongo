#!/bin/sh
# Run mutation testing over the Core and the Shell. Every step exits non-zero on failure.
set -eu

cd "$(CDPATH= cd -- "$(dirname "$0")" && pwd)/.."

# Run on the toolchain CI pins, like the automated gate.
export GOTOOLCHAIN=go1.27.0

exec go run ./internal/tools/mutate -exceptions mutation/exceptions.txt ./internal/core ./internal/tui "$@"
