# Go lint gate for Pongo

Research snapshot: 2026-09-04. This note compares Staticcheck with golangci-lint v2 for Pongo's per-build-ticket approval gate. Pongo's architecture requires the deterministic core to avoid Bubble Tea, Lip Gloss, terminal APIs, clocks, and ambient randomness. The shell translates host events and performs external work. The core owns state transitions and deterministic rules. See the [architecture decision](https://github.com/calvertjadon/pongo/issues/9#issuecomment-5543181892).

## Recommendation

Use **golangci-lint v2.13.2 as the gate runner**, with one enabled analyzer at first. Keep the gate small:

```yaml
version: "2"

run:
  go: "1.27"
  modules-download-mode: readonly

linters:
  default: none
  enable:
    - staticcheck
  settings:
    staticcheck:
      checks:
        - "SA*"
```

This configuration runs Staticcheck's `SA` group, not its `S` simplifications, `ST` style rules, or `QF` quick fixes. Staticcheck defines `SA` as correctness checks, including standard-library misuse, concurrency, testing, correctness, performance, and dubious constructs. Its configuration accepts category prefixes such as `SA*`. Sources: [Staticcheck check groups](https://staticcheck.dev/docs/checks/), [Staticcheck check selection](https://staticcheck.dev/docs/configuration/options/#checks), and [golangci-lint's Staticcheck settings](https://golangci-lint.run/docs/linters/configuration/#staticcheck).

The choice of the runner is deliberate. Pongo gets an explicit versioned configuration now and a place to add an architecture rule later. It does not get the large policy bundle from the safebucket example. If no package-boundary rule is needed after the first tickets, standalone Staticcheck is simpler and remains a valid fallback.

Do not add `govet` to this golangci-lint set if the gate also runs `go vet ./...`. golangci-lint documents `govet` as the Go vet analyzer, and some Staticcheck `SA` checks cover related patterns. Keeping the standard command separate avoids running the same analyzer through the runner and the Go tool. Staticcheck itself recommends running `staticcheck ./...` in addition to `go vet ./...`. Sources: [golangci-lint default linters](https://golangci-lint.run/docs/welcome/quick-start/#linting), [golangci-lint linter list](https://golangci-lint.run/docs/linters/#all-linters), and [Staticcheck overview](https://staticcheck.dev/docs/).

## What the tools do

| Question | Staticcheck | golangci-lint v2 |
| --- | --- | --- |
| What runs? | One Go analyzer with 150+ checks. The command accepts Go package patterns, including `./...`. | A runner that invokes selected linters. Its zero-configuration set currently includes `errcheck`, `govet`, `ineffassign`, `staticcheck`, and `unused`. |
| What is configurable? | A `staticcheck.conf` file can select checks and adjust a small set of options. Files apply to package subtrees and deeper files override higher-level files. | A v2 YAML, TOML, or JSON configuration can select linters, configure them, and control exclusions. `version: "2"` identifies the configuration schema. |
| What is the useful Pongo scope? | `SA*` gives bug and performance diagnostics without making comment or naming preferences a gate. | `default: none` plus explicit `staticcheck` keeps the same analysis while leaving a controlled place for `depguard`. |
| What does it not do? | The documented options cover checks, initialisms, dot-import exceptions, and HTTP status-code exceptions. They do not provide a package allow or deny policy. | `depguard` checks package imports against allow and deny rules, so it can enforce the core-to-shell boundary. |

Sources for the table: [Staticcheck overview](https://staticcheck.dev/docs/), [Staticcheck configuration](https://staticcheck.dev/docs/configuration/), [golangci-lint quick start](https://golangci-lint.run/docs/welcome/quick-start/#linting), [golangci-lint configuration](https://golangci-lint.run/docs/configuration/file/), and [depguard settings](https://golangci-lint.run/docs/linters/configuration/#depguard).

Staticcheck is the narrower analyzer. Its own documentation says that its checks aim to produce few to no false positives and that the project is intended for CI. The same documentation says that some `ST` style checks may not apply to a codebase. That supports the `SA*` boundary for Pongo. A false positive still needs review. Staticcheck documents `//lint:ignore` with a required reason and reports unused directives, so a suppression must explain the code rather than hide a blanket failure. Sources: [Staticcheck overview](https://staticcheck.dev/docs/), [Staticcheck FAQ](https://staticcheck.dev/docs/faq/), and [Staticcheck configuration and directives](https://staticcheck.dev/docs/configuration/).

Golangci-lint adds a second maintenance layer. A release controls the runner, its embedded analyzers, and its configuration schema. The v2.13.2 release notes show an update of `honnef.co/go/tools` from 0.8.0 to 0.8.1, which changes the Staticcheck implementation without changing Pongo's YAML. A standalone Staticcheck pin changes one tool. A golangci-lint pin changes the runner and the versions it embeds. Sources: [golangci-lint v2.13.2 release](https://github.com/golangci/golangci-lint/releases/tag/v2.13.2), [Staticcheck 2026.2.1 release](https://github.com/dominikh/go-tools/releases/tag/2026.2.1), and [Staticcheck installation](https://staticcheck.dev/docs/getting-started/).

## The safebucket example

The [safebucket `.golangci.yml`](https://github.com/safebucket/safebucket/blob/main/.golangci.yml) is a useful example of a real v2 file, not a Pongo baseline. It enables formatters (`goimports` and `golines`), a large list of linters, custom complexity thresholds, strict `govet` shadow checking, and several exclusion presets. Its comments mark some policies as too strict or too noisy for that project, including `contextcheck`, `paralleltest`, `varnamelen`, `wsl`, and other checks. It also enables all Staticcheck checks and then excludes selected checks, rather than limiting Staticcheck to a bug group.

The following safebucket checks do not fit Pongo's current scope:

- **Service and data-stack checks.** `arangolint`, `clickhouselint`, `promlinter`, `protogetter`, `spancheck`, `sqlclosecheck`, `rowserrcheck`, `loggercheck`, `sloglint`, and `unqueryvet` target ArangoDB, ClickHouse, Prometheus, protobuf, telemetry, SQL, logging, or SQL-query usage. Pongo is a terminal game with a Bubble Tea shell and a deterministic game core, not one of those service stacks. The linter descriptions in the [golangci-lint catalog](https://golangci-lint.run/docs/linters/#all-linters) identify those integrations.
- **Test-framework checks.** `ginkgolinter`, `testifylint`, `testpackage`, `testableexamples`, `thelper`, `tparallel`, and `paralleltest` impose policies for Ginkgo, Testify, external test packages, examples, or parallel tests. Pongo should test deterministic state transitions; it should not require a framework or parallel-test convention that the project has not chosen. The catalog describes each of these checks in the [test section of the linter list](https://golangci-lint.run/docs/linters/#all-linters).
- **HTTP and inherited-context checks.** `bodyclose`, `canonicalheader`, `contextcheck`, `fatcontext`, and `noctx` target HTTP responses, HTTP headers, or context propagation. The Pongo core must not depend on a clock or host API, and the settled architecture does not require an HTTP service. These checks add no approval signal for the current game. Sources: [linter catalog](https://golangci-lint.run/docs/linters/#all-linters) and the [Pongo architecture decision](https://github.com/calvertjadon/pongo/issues/9#issuecomment-5543181892).
- **Complexity and size thresholds.** `cyclop`, `gocognit`, `gocyclo`, `funlen`, `maintidx`, `nestif`, and `dupl` measure thresholds or duplication. They are policy choices, not proof that a state transition is wrong. A fixed threshold can punish a clear educational transition table or a deliberately repeated branch. Keep complexity review human-led until the code has a measured problem.
- **Formatting and prose policy.** `goimports`, `golines`, `lll`, `godoclint`, `godot`, `revive`, `nlreturn`, `whitespace`, `funcorder`, `decorder`, `nonamedreturns`, `nakedret`, `ireturn`, `inamedparam`, `interfacebloat`, and `varnamelen` impose formatting, comment, declaration-order, naming, interface, or whitespace preferences. They are subjective style checks, or they duplicate a formatter. Use `gofmt` as the standard formatter and review the remaining style choices instead of making them a per-ticket failure. The v2 migration guide places `gofmt`, `gofumpt`, `goimports`, and `gci` in the formatter section, separate from linters. Sources: [safebucket configuration](https://github.com/safebucket/safebucket/blob/main/.golangci.yml), [gofmt documentation](https://pkg.go.dev/cmd/gofmt), and [golangci-lint v2 migration](https://golangci-lint.run/docs/product/migration-guide/#lintersenableformatter_name).
- **Micro-optimization and modernization checks.** `goconst`, `intrange`, `perfsprint`, `prealloc`, `unconvert`, `usestdlibvars`, and `exptostd` suggest refactors or optimizations. They may be useful in a larger service, but they do not establish that Pongo's fixed-step simulation or shell behavior is correct. The [catalog](https://golangci-lint.run/docs/linters/#all-linters) describes these as simplification, optimization, or replacement suggestions.
- **Unchosen policy checks.** `err113`, `errname`, `errorlint`, `wrapcheck`, `gochecknoinits`, `gochecknoglobals`, `mnd`, `predeclared`, `forbidigo`, `exhaustive`, and `gochecksumtype` each encode a project policy or a domain assumption. Add one only when a Pongo design decision makes it observable and worth enforcing. In particular, do not use `mnd` to reject the numeric constants that make the court geometry and fixed-step rules readable.

`gosec`, `errcheck`, and `nolintlint` deserve a separate decision. They can be useful in projects that handle untrusted input, many external resources, or many suppressions. Pongo's first gate does not need their extra policy surface. Add them only after a ticket records the specific failure they prevent. This is a scope decision, not a claim that those analyzers are defective. The [catalog](https://golangci-lint.run/docs/linters/#all-linters) describes their checks.

## Architecture enforcement with depguard

`depguard` is not a bug detector or a style checker. It is an import policy. The golangci-lint documentation supports file globs, `$all`, `$test`, `$gostd`, allow entries, deny entries, and exact package matches with a trailing `$`. That makes it suitable for the Pongo rule that the deterministic core must not import the Bubble Tea shell, terminal libraries, a clock, or ambient random sources. Source: [depguard settings](https://golangci-lint.run/docs/linters/configuration/#depguard).

Do not enable it before the learner chooses the core package path. The architecture decision intentionally leaves implementation names open. After the core package exists, add a rule like this and replace `internal/core/**/*.go` with the actual path:

```yaml
linters:
  default: none
  enable:
    - staticcheck
    - depguard
  settings:
    staticcheck:
      checks:
        - "SA*"
    depguard:
      rules:
        deterministic-core:
          files:
            - "internal/core/**/*.go"
          deny:
            - pkg: "charm.land/bubbletea/v2"
              desc: "the core accepts semantic events, not Bubble Tea messages"
            - pkg: "charm.land/lipgloss/v2"
              desc: "rendering belongs to the shell"
            - pkg: "golang.org/x/term"
              desc: "terminal access belongs to the shell"
            - pkg: "time$"
              desc: "the shell supplies elapsed time as data"
            - pkg: "math/rand$"
              desc: "randomness must be explicit and owned by the core"
            - pkg: "math/rand/v2$"
              desc: "randomness must be explicit and owned by the core"
            - pkg: "crypto/rand$"
              desc: "ambient entropy is outside the deterministic core"
```

This rule checks imports in matching files. It does not prove that a function is deterministic, that a renderer cannot mutate state, or that a shell does not duplicate state. Tests and review must enforce those properties. The rule also needs updates if Pongo selects another terminal package or a deliberate random-generator package. Keep those changes attached to an architecture decision, not an incidental lint fix.

## Pinning and reproducibility

Pin the Go toolchain and the gate binary independently.

1. Put the selected Go language version in `go.mod`, for example `go 1.27.0`. The Go documentation defines the `go` line as the module's minimum required Go version. A `toolchain go1.27.1` line can name a preferred toolchain when the project chooses that exact patch release. CI should install or select the same exact patch version. Source: [Go toolchains](https://go.dev/doc/toolchain).
2. Pin golangci-lint to a released version. At this snapshot, the official install page shows `v2.13.2`, and the CI documentation recommends installing a specific release because an `all` default or an upstream linter upgrade can make builds start failing together. Pongo's `default: none` avoids automatic adoption of new linter names, but it does not freeze the behavior of the enabled Staticcheck inside a changed runner. Sources: [local installation](https://golangci-lint.run/docs/welcome/install/local/), [CI installation](https://golangci-lint.run/docs/welcome/install/ci/), and [v2.13.2 release](https://github.com/golangci/golangci-lint/releases/tag/v2.13.2).
3. Use the same checked-in config locally and in CI. Golangci-lint searches parent directories and then the home directory for a config when one is not found. Pass `--config=.golangci.yml` in the gate so a developer's home config cannot change the result. Use `-v` when diagnosing config selection. Source: [configuration file lookup](https://golangci-lint.run/docs/configuration/file/).
4. Run the same package pattern, `./...`, in both places. Both tools document support for Go package patterns. Source: [Staticcheck installation and package patterns](https://staticcheck.dev/docs/getting-started/) and [golangci-lint quick start](https://golangci-lint.run/docs/welcome/quick-start/#linting).
5. Review pins as code. Staticcheck 2026.2.1 records a fix for two false positives introduced in 2026.2, and golangci-lint v2.13.2 records an embedded Staticcheck dependency update. Upgrade only in a ticket that runs the full gate and records changed diagnostics. Sources: [Staticcheck 2026.2.1 release](https://github.com/dominikh/go-tools/releases/tag/2026.2.1) and [golangci-lint v2.13.2 release](https://github.com/golangci/golangci-lint/releases/tag/v2.13.2).

Use the official binary or container rather than an unpinned installer in CI. The documented commands are:

```bash
# Local binary, pinned to the approved release.
curl -sSfL https://golangci-lint.run/install.sh | sh -s -- -b "$(go env GOPATH)/bin" v2.13.2
golangci-lint --version

# Gate command, with the repository config forced.
golangci-lint run --config=.golangci.yml ./...

# Equivalent container invocation for CI.
docker run --rm -v "$PWD":/app -w /app \
  golangci/golangci-lint:v2.13.2 \
  golangci-lint run --config=.golangci.yml ./...
```

The golangci-lint installation page documents the versioned binary and Docker forms and warns that `go install` compiles the tool with the local Go version and is not the recommended installation method. If the project chooses standalone Staticcheck instead, pin it with the documented Go install form and run it directly:

```bash
go install honnef.co/go/tools/cmd/staticcheck@2026.2.1
staticcheck ./...
```

Sources: [golangci-lint local installation](https://golangci-lint.run/docs/welcome/install/local/) and [Staticcheck installation](https://staticcheck.dev/docs/getting-started/).

## Commands that remain separate

A linter does not replace the compiler, formatter, tests, or the interactive terminal check. Keep these commands outside the linter configuration:

```bash
# Formatting gate. gofmt -l prints files whose formatted output differs.
test -z "$(gofmt -l .)"

# Standard-library vet checks. Do not also enable golangci-lint's govet in this gate.
go vet ./...

# Behavioral and package test gate.
go test ./...

# Compile every non-test package, including the main package.
go build ./...
```

`gofmt` formats Go programs and `-l` lists files that differ. `go vet` reports suspicious constructs, exits nonzero when it reports a problem, and warns that its heuristics are guidance rather than proof of correctness. `go test` runs package tests. `go build` compiles packages and dependencies and, for multiple packages, discards the objects after using compilation as a check. Sources: [gofmt](https://pkg.go.dev/cmd/gofmt), [go vet](https://pkg.go.dev/cmd/vet), and [Go command reference](https://pkg.go.dev/cmd/go).

Run `go mod tidy` only when dependency declarations change, then review `go.mod` and `go.sum`. In CI, `modules-download-mode: readonly` makes a missing module-file update an explicit failure instead of an implicit edit. Source: [Go modules reference](https://go.dev/ref/mod) and [golangci-lint run settings](https://golangci-lint.run/docs/configuration/file/#run-configuration).

The linter and these commands still cannot prove Pongo's terminal behavior. Each ticket must separately exercise the agreed interactive checks, including the Bubble Tea shell, terminal-too-small behavior, resize, quit, and restoration. Those checks belong to the ticket's acceptance procedure, not to Staticcheck or golangci-lint.
