# GS-01 pinned consumer harness

This host-only helper prepares independent native and simulation copies from an
exact Git commit. It does not edit the source consumer checkout. Use Go 1.26 for
the pinned gquorum revision; Go 1.25 cannot build that consumer.

```sh
GOTOOLCHAIN=go1.26.9 go test -ldflags=-checklinkname=0 -tags=linkname ./.ci/gs01-consumer
GOTOOLCHAIN=go1.26.5 go run -ldflags=-checklinkname=0 -tags=linkname ./.ci/gs01-consumer \
  -source "$GS01_CONSUMER_REPO" \
  -revision 10c78f90aab45267d4dc29265c59f410c1d87c40 \
  -output "$GS01_CONSUMER_OUTPUT" -gosim-source "$PWD"
GOTOOLCHAIN=go1.26.5 go run -ldflags=-checklinkname=0 -tags=linkname ./.ci/gs01-consumer \
  -action run-a -output "$GS01_CONSUMER_OUTPUT"
GOTOOLCHAIN=go1.26.5 go run -ldflags=-checklinkname=0 -tags=linkname ./.ci/gs01-consumer \
  -action verify -output "$GS01_CONSUMER_OUTPUT"
```

The output directory must not exist. Preparation rejects archive traversal,
links, submodules and functional-tagged production source. Test header edits
replace only the parsed `functional` atom with `sim`; test bodies are unchanged.
The simulation-only init creates `/tmp` and configures TMPDIR/GOTMPDIR. Native
source, assertions, Go directive and original dependency minima are preserved.
The simulation module adds Gosim's requirements through minimum-version
selection, a local Gosim replacement, and checksums for the complete resolved
graph. It does not downgrade consumer dependencies or relax readonly translation.

`manifest.json` records source/compiler identity, original/final hashes,
production/body hashes, tag changes, bootstrap/config/module content, native and
simulated source selection, exact gate-A tests/seeds, unexecuted B/C cases,
command logs and outcomes. `run-a` checks integrity before/after execution and
requires all four test functions to run and pass at seeds 1–3. Skips, compilation
alone, missing tests and failed setup cannot qualify as acceptance. Generated
`.gosim` artifacts are excluded from the copy-integrity check. Retain manifests
and command logs with any report; this is ordinary evidence, not certification.

## Initial blocker and approved translator repair (2026-10-25)

The first unchanged consumer run stopped during graph selection, before test execution:

```text
github.com/asalimonov/gquorum/internal/recovery/disk_test [.../disk.test]
 -> github.com/asalimonov/gquorum [.../disk.test]:
test package github.com/asalimonov/gquorum [.../disk.test]
lacks base github.com/asalimonov/gquorum
```

`discovery_api_func_test.go` uses external package `disk_test` and imports the
public gquorum package, which transitively imports `disk`. Go produces a
recompiled dependency variant for that test binary. The original `classifyPackage` in
`internal/translate/packages.go` classified every ID ending in `.test]` as a
package's own test variant; `selectGraph` then required a separately loaded base
package that this dependency closure does not contain. Typed reload also grouped packages by explicitly requested roots.

A four-file fixture independently reproduces the issue using only `rt`: a leaf
package with an internal test, a parent importing the leaf, and an external leaf
test importing the parent. Both tests pass natively on Darwin ARM64/Go 1.26.5;
translation reports the same missing-base error. Evidence is under
`.gosim/validation/gs01/task6/testvariant-repro/` with sibling
`testvariant-native.log` and `testvariant-sim.log`. Consumer preparation and the
failed run are recorded in `.gosim/gs01-consumer/go1265-v3/manifest.json`.

The user approved including this repair in Task 6. Indirect test dependencies
now translate their full production declarations while retaining their original
IDs and import edges. Typed reload uses the owning test context and loads its
dependency syntax when an indirect variant misses the cache. Focused regressions
pass on Go 1.25.14 and 1.26.9, including production globals/initializers,
test-only exports, batched roots and cached reruns. No consumer imports or
assertions were changed. Fresh consumer evidence is recorded in `.gosim/gs01-consumer/go1265-v4/`:
all four unchanged A tests passed seeds 1–3 with Go 1.26.5, including their
parallel subtests. Integrity verification passed after execution. See the plan
and progress log for final-source revalidation.

Simulation gate B still depends on GS-03 statx, and C/hardlink on GS-03 Linkat.
Other C shapes remain unexecuted; no whole staging-test pass is claimed. The
GS-02 flock model is available, but this consumer run does not qualify lock
ownership/exclusion. Journal
boundary-error tests do not establish crash-state coverage. Native consumer
A/B/C validation is still pending under Task 8; native Linux/AMD64 execution is
user-deferred.
