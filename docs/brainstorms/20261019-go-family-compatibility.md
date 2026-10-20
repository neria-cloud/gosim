# Go family compatibility brainstorm

Status: research and first design decision; no implementation authorized by this brainstorm. No tests, generators, certification jobs, or toolchain downloads were run. Existing working-tree edits are preserved.

## Initial request

Support Go version families `1.25.*` and `1.26.*`, including callers using Go 1.25.1 through Go 1.26.9. Avoid making the library unusable through exact-patch pinning. Reduce the work required to support a new Go 1.X family. Reconsider runtime builds where necessary. Compare the implementation at `ffd3a613542675755e4cbf8186b5edaf404ed95c`.

The user identifies these files:

- `internal/compat/go125/profile/origin.json`
- `internal/compat/go125/profile/routes_amd64.json`
- `internal/compat/go125/profile/routes_arm64.json`
- `internal/compat/go126/profile/origin.json`
- `internal/compat/go126/profile/routes_amd64.json`
- `internal/compat/go126/profile/routes_arm64.json`

Working assumption: these are family-wide requirements, not a request to enumerate every patch up to 1.26.9. Exact lower bounds and treatment of future patches remain to be recorded explicitly.

The preceding instruction disabling AMD64 and race certification remains in effect. This discussion does not re-enable certification or authorize expensive validation.

## Sources and project state

The provided skill was read from `/Users/speckzzz/.claude/skills/go-brainstorm/SKILL.md`. No input-task file or `project.txt` is present; the conversation is the input and the current repository is the project. `architecture/` and `docs/architecture/` do not exist. Current architecture is in `docs/design.md`, with usage in `README.md` and historical requirements in `docs/brainstorms/20261018-modular-features-and-go-compatibility-target.md`.

At research start HEAD is `7ddc594`. Existing uncommitted edits disable AMD64/race certification and update documentation. README also has separately staged user edits. These changes are outside this brainstorm's modifications.

## Findings: where patch restrictions actually occur

| Component | Current behavior | Consequence for family support |
| --- | --- | --- |
| `go.mod` | Minimum `go 1.25.0` | The module minimum itself does not restrict users to 1.25.14 or 1.26.9. |
| `internal/buildspec/canonical.go` | Version parser already accepts exact `go1.25.N` and `go1.26.N` strings | The parser is not the two-patch allowlist; retain exact resolved versions in identities. Generalize family registration later. |
| `internal/buildcfg/config.go` | Requires a concrete version, including when no config exists | Add a default that resolves the application's effective toolchain; resolve optional family selectors once before building. |
| `internal/buildcfg/distribution.go` | Embedded official-distribution records allow only recorded patch/platform combinations; archive and installed GOROOT are verified | This prevents other patches before adapter selection. Decouple installation provenance from supported-family policy. |
| `internal/buildcfg/toolchain.go` | Worker must have exactly the selected compiler version and GOROOT; mismatch asks the user to rebuild | Matching worker/compiler remains useful, but automatic worker build/cache can remove manual rebuild friction. |
| `internal/buildspec/command.go` | Rejects automatic toolchain selection settings and disables workspaces in child processes | Separate initial application toolchain discovery from fixed execution after resolution; avoid silently selecting a different Go version mid-build. Workspace handling needs explicit design. |
| `internal/compat/selected` and `internal/hostabi` | Already select adapters using `go1.25 && !go1.26` and `go1.26 && !go1.27` build constraints | Family-owned runtime adapters can remain. No evidence yet that per-patch runtime source trees are necessary. |
| `internal/compat/spec/profile.go`, `validate.go` | Exact tuple membership plus pending/certified status required | Introduce family support independently from exact execution evidence. New patches must not require tuple entries to run. |
| `cmd/gosim/config.go`, `internal/buildcfg/certification.go` | Every ordinary command requires current-source reviewed evidence for the exact tuple | Family selection alone will not solve usability while this gate remains mandatory. Discuss making certification optional evidence, retaining truthful provenance. |
| `internal/compat/spec/origin.go` | Origin verification requires baseline patch, version executable/file equality, and every recorded source-file hash | Retain baseline provenance for audit/generation; do not compare arbitrary compatible patches to baseline file bytes as an eligibility check. |
| `internal/routegen/generate.go` | Explicit switch accepts only go125/1.25.14 or go126/1.26.9; emits generation records | Select by family; distinguish generator baseline metadata from supported toolchains. |
| `internal/compat/genprofile` | Profile generation delegates to spec generation | Audit its family lists and generated revisions during target design. |
| `internal/compat/spec/hooks.go` | Inspects actual selected Linux sources and checks required hook signatures | Reusable structural compatibility check; signatures alone cannot prove private layout or behavioral compatibility. |
| `internal/gosimtool`, `metatesting`, generated composition | Exact build identities, binary/manifest checks, handshake and host identity | Preserve exact artifact matching. Family compatibility does not imply cross-patch binary or replay interchangeability. |

`origin.json` inventories 135 upstream files for go125 and 241 for go126. Both architecture-specific route records carry their family's baseline patch and generated adapter/transport/origin digests. These records describe generation provenance; changing their `toolchain` fields to wildcard strings would not resolve the other restrictions and would lose useful provenance.

`profile.Verify` invokes origin verification and hook checks during profile verification/generation. The ordinary command resolver invokes `VerifyHooks`; it does not directly invoke `VerifyOrigin`. These are distinct enforcement paths and should not be conflated.

## Historical comparison

The specified commit itself changes README only. Its tree is still a useful reference:

- Module minimum was Go 1.23.2.
- `internal/gosimtool.BuildConfig` held GOOS, GOARCH and Race, without the current certification identity model.
- Translation used `hooksGo123` and `internal/hooks/go123` directly.
- `cmd/gosim` invoked `go test` from the environment and retained the linkname flags.
- `internal/translate/main.go` explicitly left Go-version and GOTOOLCHAIN awareness as TODOs.

Conclusion: recover the simple caller experience, but do not treat this revision as an existing multi-family implementation or restore its incomplete version/cache handling.

## Compatibility assumption

Go aims to preserve source compatibility, including patch releases, but its documented exceptions include unsafe implementation dependencies, security fixes, and bug fixes. Private runtime internals and binary compatibility are not covered by a blanket stability guarantee. Gosim uses private hooks and host ABI access, so accepting a family is a Gosim support policy rather than proof that every patch has identical internals.

Sources:

- [Go compatibility policy](https://go.dev/doc/go1compat)
- [Go toolchain selection](https://go.dev/doc/toolchain)

Recommendation: accept ordinary patches in a supported family by default; verify the private contracts actually used; maintain narrowly scoped exceptions only when an actual incompatibility is found. Do not require baseline source-file byte equality or per-patch certification. Structural checks cannot prove unchanged behavior; representative CI remains necessary. No cross-patch deterministic replay promise is proposed.

## Proposed direction, pending discussion

Separate three concepts:

1. **Support policy:** supported Go families, optional minimum patches and documented exceptional incompatibilities.
2. **Build identity:** exact resolved toolchain, selected distribution/source content, adapter/runtime/provider revisions, application graph, platform, features and instrumentation.
3. **Validation evidence:** the actual toolchain/source/tuple tested and results. Evidence describes tested builds; it does not become a mandatory registration record for every caller patch.

Keep family adapters and architecture-specific layouts. Use the caller's selected patch to compile source; build and cache an exact matching worker automatically when the installed CLI was compiled by another patch/family. Exact worker matching is an internal consistency property, not a public patch allowlist.

Retain `origin.json` and route generation baselines as provenance, with schema names that make their purpose explicit. Separate `VerifyBaselineOrigin` from `CheckCompatibility` against an arbitrary supported toolchain. Do not regenerate all metadata just because a new compatible patch exists.

Illustrative types only; these are not finalized APIs:

```go
type GoFamily struct {
    Major int
    Minor int
}

type FamilySupport struct {
    Family GoFamily
    MinimumPatch int
    // Exceptional exclusions require a concrete incompatibility and reason.
    Exclusions []PatchExclusion
}

type GenerationOrigin struct {
    BaselineToolchain string // exact version used to generate/review adapters
    Files []SourceFile      // baseline provenance, not caller admission rules
}

type CompatibilityResult struct {
    Family GoFamily
    Toolchain string        // actual patch inspected
    ContractDigest string  // checked private hook/layout contract
    ToolchainDigest string // invalidates results when inspected inputs change
}
```

Provisional flow:

```text
discover Go selected in application context (module/workspace/environment)
  -> parse concrete version and find supported family
  -> locate/build/cache Gosim worker with that concrete compiler
  -> worker selects family adapters through build constraints
  -> check required private hooks/layout contracts
  -> compute exact build identity
  -> reuse exact matching artifact or translate/build from source
  -> execute with existing manifest/host-identity checks
```

Potential Go 1.27 onboarding should become: implement/audit the actual changed adapters, register family support, generate family metadata, add build-tag selectors, run selected CI checks. It should not require a new Gosim release for every Go 1.27 patch. A new minor family still requires an ABI/translation audit; automatic acceptance of unimplemented future families is not proposed.

No database or ORM is involved. Worker and compatibility-result caches will require explicit key, invalidation, concurrent publication and retention designs in the target document.

## Question 1: patch used to compile simulations

Asked using the asynchronous question tool:

> Should Gosim compile simulations with the application's selected Go patch, or choose a maintained patch internally for each supported family?

- **Recommended:** use the application's patch; accept Go 1.25.x/1.26.x and automatically build/cache a matching worker when needed.
- **Alternative:** accept callers on any supported patch, but compile simulations with a Gosim-selected family baseline.

The alternative reduces the number of simulation compilers but keeps hidden patch management, can diverge from application behavior/dependency minimums, and does not satisfy a requirement to simulate under the caller's exact patch.

Initial answer: user requested examples; subsequent clarification below establishes that ordinary Go tooling must resolve the compiler. The maintained-baseline alternative does not meet that default behavior.

## Follow-up: caller and library examples for both variants

User response: "Write examples for both variants. I don't understand how it will look like in callers code and in the library". This requests clarification, not selection of a variant.

Both designs preserve the caller-facing Go API. A simulation test can use the existing runtime info API to observe the concrete simulation toolchain:

```go
package app_test

import (
    "testing"
    "time"
    simrt "github.com/neria-cloud/gosim/rt"
)

func TestClock(t *testing.T) {
    start := time.Now()
    time.Sleep(time.Second)
    if time.Since(start) != time.Second {
        t.Fatal("unexpected simulated elapsed time")
    }
    t.Log("simulation compiler:", simrt.Info().Toolchain)
}
```

Assume the application requires `go 1.25.0`, depends on the redesigned Gosim release, and the Go toolchain selected in its directory is `go1.25.1`. The proposed simple entry point in either design is `gosim test -features rt ./...`; the versionless default is new behavior, not a claim that the current implementation supports it. These are simulation runs; ordinary native `go test` does not activate Gosim translation.

Variant A uses go1.25.1 to compile the matching worker and simulation, selects go125 adapters, and logs go1.25.1. If the application switches to go1.25.8, the same test and command use go1.25.8 with the same go125 adapter sources and a distinct cache identity. If it switches to go1.26.9, they use go1.26.9 with go126 adapters. No per-patch library release or tuple registration is required, assuming the patch satisfies the family's private contracts.

Variant B maps the application's selected family to a library-maintained compiler baseline. Illustratively, callers on go1.25.1 and go1.25.8 both build simulations using go1.25.14 and go125 adapters; callers on go1.26.1 use go1.26.9 and go126 adapters. These baseline versions are examples from the current repository, not a proposed immutable policy. The ordinary application may still build using its own patch. If a dependency requires a patch above the internal baseline, the tool must resolve that explicitly or fail; it must not silently downgrade the requirement.

Simplified internal pseudocode, not existing method signatures:

```go
// Variant A: no per-patch support map.
func chooseSimulationGo(app Toolchain) (Toolchain, error) {
    if err := requireSupportedFamily(app.Version); err != nil {
        return Toolchain{}, err
    }
    return app, nil
}

// Variant B: the library still maintains particular compiler versions.
var familyBaseline = map[string]string{
    "1.25": "go1.25.14",
    "1.26": "go1.26.9",
}

func chooseSimulationGo(app Toolchain) (Toolchain, error) {
    version, ok := familyBaseline[familyOf(app.Version)]
    if !ok {
        return Toolchain{}, ErrUnsupportedFamily
    }
    return locateOrDownloadGo(version)
}

// Shared worker flow; no cross-version linking of compiled Go packages.
func runSimulation(appDir string, args []string) error {
    appGo, err := discoverApplicationGo(appDir)
    if err != nil { return err }
    simulationGo, err := chooseSimulationGo(appGo)
    if err != nil { return err }
    worker, err := ensureMatchingWorker(simulationGo, gosimSourceIdentity())
    if err != nil { return err }
    return worker.Run(appDir, args) // worker checks contracts, translates, builds, runs
}
```

The native application process and a generated simulation executable are separate processes; the worker compiles the simulation's dependencies and Gosim runtime with its selected compiler. Neither variant mixes Go 1.25.1-compiled runtime objects into a Go 1.25.14 binary. A CLI built using a different patch can act as a launcher and start a cached worker. For that to work across families, launcher imports must be separated from family-bound runtime/translation code; merely relaxing `ValidateWorker` is insufficient.

Family adapter selection remains at compile time in both variants:

```go
//go:build go1.25 && !go1.26
// selects internal/compat/go125
```

```go
//go:build go1.26 && !go1.27
// selects internal/compat/go126
```

Recommendation remains A because it removes library-controlled compiler baselines from ordinary use and tests with the application-selected patch. B improves caller admission but retains internal patch pinning and can test a different standard-library/runtime implementation from the application.

## Clarification: Go tooling owns version selection

User: "Application shouldn't 'select' Go. The go tooling should choose the right version automatically. Does variant 1 fit it?"

Yes. The phrase "application-selected patch" meant the effective compiler resolved by Go in the application directory, not an application API, Gosim config field, or a manual patch choice. Use "Go-resolved toolchain" going forward.

Required behavior:

1. The caller runs `gosim test -features rt ./...` without a Gosim version setting.
2. Before rewriting compiler-related environment or disabling workspace context, the launcher invokes normal Go toolchain discovery in the caller's directory, honoring `go.mod`, applicable `go.work`, `GOTOOLCHAIN` and the user's Go configuration. For example, query `go env -json GOVERSION GOROOT GOHOSTOS GOHOSTARCH` through the user's Go command.
3. Go applies its normal toolchain rules, including downloading a required compiler when the user's configuration permits it. Gosim must not interpret the module's `go` directive as an exact compiler pin or independently choose the newest patch.
4. Gosim checks that the resolved compiler belongs to a supported family and locates/builds the matching worker automatically.
5. Worker creation and simulation compilation use that resolved concrete compiler. The launcher must preserve the resolution across its own source-module context instead of accidentally resolving again against Gosim's module and changing versions.
6. Exact compiler identity remains in cache/artifact/replay metadata. Freezing a resolved compiler for one operation is internal build consistency, not a library-wide supported-patch restriction.

With `GOTOOLCHAIN=local`, Gosim respects Go's refusal to auto-download or switch; it does not silently override that user setting. Ordinary Go selection does not mean "always use the latest available patch."

For a directly imported library, the calling `go build`/`go test` already resolves the compiler. Family build constraints then choose the corresponding adapters without runtime configuration. Native `go test` still does not activate simulation translation by itself.

This refines Variant A. Question 1's ownership issue is resolved by the user's stated requirement; there is no need to ask the user to choose a patch. Worker/bootstrap mechanics and certification-policy migration remain open design work.

## Decision 1: Variant A selected

User confirmation: "OK, the first variant".

Selected: normal Go tooling resolves the compiler in the caller's context. Gosim accepts supported families and automatically uses/builds a worker for the concrete Go-resolved compiler. No internal family baseline is substituted. Existing caller APIs remain unchanged. Exact toolchain identity is retained for worker, artifact, and replay consistency.

Additional source findings relevant to implementation:

- `cmd/gosim/main.go` imports the translator and build runner directly. A launcher capable of delegating across supported families needs a family-independent startup path; it cannot rely on running family-bound initialization first and only then noticing the compiler mismatch.
- `internal/gosimtool.invokeTestWorker` explicitly rejects mismatched workers and falls back to `go run` when no `GOSIMTOOL` is provided. Metatesting needs to participate in automatic worker discovery/cache. The native test binary's actual compiler is already fixed by the enclosing Go invocation, so metatest design must account for that identity rather than silently selecting a newly changed environment's compiler.
- `CertifiedIdentity` blocks ordinary commands on missing, pending, stale, or nonmatching tuple evidence. Keeping this unchanged would prevent the selected policy from working on arbitrary family patches.
- Current identity digests include certification status/evidence, while `SourceDigest` normalizes those fields. If evidence becomes informational, its attachment must be separated from executable cache/replay identity so adding a report does not rebuild unchanged programs or change replay identity.

## Question 2: role of certification

Asked:

> Should certification become optional CI evidence, so ordinary Gosim commands run on supported Go families without requiring a certification report?

- **Recommended:** optional CI evidence. Retain compatibility and artifact checks, but missing/stale certification does not block ordinary use. Never describe an untested concrete build as certified.
- **Alternative:** require family-level certification for each Gosim revision before ordinary commands can run. This avoids a patch-by-patch allowlist but preserves a release/source approval dependency for users and local development.

Answer: user selected optional evidence, keeping compatibility/artifact checks and allowing ordinary use when certification is missing or stale.

Immediate correction from user: "Certification is not a step of CI".

Final decision: certification is an optional, explicitly invoked manual operation. It is not an ordinary command prerequisite, an ordinary-test implementation detail, or a CI step. CI executes tests and publishes test reports without invoking certification, requiring certification metadata, or promoting results to certification. AMD64/race certification remains disabled. The earlier wording "optional CI evidence" is superseded.

The existing `.ci/crossarch-tests/verify.py` calls `internal-candidate certify` for ordinary/race validation, and `.github/workflows/main.yml` has a `certify` job. Both must change in implementation. Merely renaming CI output or removing the admission gate would leave this requirement unmet.

## Target draft

The companion `20261019-go-family-compatibility-target.md` specifies the selected design for review. No implementation changes or validation runs are performed by this brainstorm. Any policy choices not explicitly selected by the user are marked as draft defaults rather than user decisions.

## NOT CONSIDERED & TODO

- [resolved] Variant A confirmed: use the compiler resolved by ordinary Go tooling; automatic matching workers.
- [resolved] Certification is optional, manually invoked, absent from CI and ordinary-command admission. The target separates report metadata from executable identity.
- [postponed] Define exact supported lower bounds and future patch policy, including Go 1.25.0, prereleases, devel builds and custom distributions.
- [postponed] Specify which private hook/layout checks run locally, their caching and diagnostics, and what requires CI behavioral coverage. Avoid replacing certification with another expensive per-invocation gate.
- [postponed] Design installed-toolchain discovery/provenance without the embedded patch archive allowlist; distinguish local installation trust from byte-level cache identity.
- [postponed] Specify bootstrap under module replacements, workspaces, offline operation, recursive invocation and concurrent processes. Preserve selected Gosim source version.
- [postponed] Audit public library imports, metatesting worker selection and their dependency minimums using early supported patches. No compatibility execution has been performed in this brainstorm.
- [potential-improvement] Generate repetitive family registry/selector metadata when onboarding a new minor version; preserve explicit compile-time family ownership.
- [postponed] Update AGENTS/README/current architecture and supersede conflicting historical target requirements during implementation, not silently in research.

## Session summary

The main obstacle is policy spread across multiple layers, not a need to copy runtimes for every patch. Family-based support can preserve version-owned adapters and exact build/replay identities while removing patch allowlists and mandatory certification from ordinary use. The worker/toolchain choice is now resolved as Variant A; certification is optional and manual only. A detailed target draft is recorded in the companion architecture file for review.
