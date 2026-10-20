# Linting without changing simulation contracts

`.golangci.yml` is the policy used by `make lint`, `make lint-full`, the
pre-commit hook, and `make fmt`. `revive.toml` is a secondary configuration for
standalone revive; golangci-lint does not read it.

## Contract-owned source rules

Handwritten files in `internal/compat/go*/`, `internal/compat/selected/`,
`internal/compat/spec/`, `internal/hostabi/`, `internal/buildspec/`, and
`rt/runtime/` remain linted. Their exclusions select individual linters or
specific revive diagnostic names; revive is not excluded as a whole.

| Checks | Policy in these paths |
| --- | --- |
| Cognitive complexity, cyclomatic complexity, function length | Enforced at the existing limits: 16, 24, and 42 statements respectively |
| Bug/security checks, including ineffassign, staticcheck and gosec | Active, subject to the shared rule and test-file exclusions |
| Revive naming, argument/result conventions, receiver-type and nested-struct conventions | Excluded to preserve contract-owned declarations and ABI layouts |
| unused, predeclared, copyloopvar | Excluded to preserve indirectly referenced symbols, names and loop semantics |
| lll, revive line-length-limit, automatic formatting | Excluded for byte-sensitive sources |

All other enabled revive rules apply. Generated files remain excluded and must
be updated through their generators. Complexity findings call for reviewed
refactoring that preserves method signatures, scheduling and evaluation order.
Changes to fingerprinted adapter sources still require profile regeneration
and the relevant contract tests.

## Commands and regression checks

`make lint` exports the Git index to a temporary snapshot, analyzes complete
packages with the `linkname` tag, and reports findings across each staged file
in the selected compiler's native build. Inactive Go-family files and `sim`-only
source require their corresponding validation workflow.
It supports nested Go modules and includes unchanged sibling files for type
checking. Package type errors remain failures even if they point to an
unchanged sibling. Neither unstaged edits nor missing sibling declarations can
silently change the result. Snapshot-specific analyzer caches avoid findings
whose filenames refer to deleted snapshots.

`make lint-full` reports the existing codebase backlog using the same policy.
It is not expected to become clean by mechanically applying every suggestion.
`make fmt` selects modified, added and untracked Go files, respects the YAML
exclusions, and does not stage changes.

Run `make test-lint-tools` to validate the configuration and exercise real
Git/Go/linter regressions for partial staging, sibling type errors, nested
modules, repeated snapshots, protected-source diagnostics, and formatter
exclusions. The complexity regression checks every protected path group and
verifies that ordinary source still receives naming/signature findings.
Run `make test` for the ordinary native and simulation suite after source
changes.

The configuration follows the upstream
[exclusion settings](https://golangci-lint.run/docs/configuration/file/) and
[explicit revive rules](https://golangci-lint.run/docs/linters/configuration/#revive).
