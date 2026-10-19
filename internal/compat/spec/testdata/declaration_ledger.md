# Hook and shim declaration audit

`declaration_ledger.json` preserves all 964 function/type declarations from the
hook and shim sections of the e6122b1 declaration inventory. Original source
paths, names and signatures are immutable audit input, including the go123 and
go125 copies and both generated architectures. The source-document digest is
provenance; the declaration digest and count are independently checked by tests.
The tests do not require the ignored brainstorm document or retired source files.

Each record has one reviewed disposition:

- `hook`: selected source declaration and family-owned callable target.
- `binding`: retained private adapter or translated shim declaration. Named
  capability records specify rejected, limited and stubbed behavior; retaining
  a declaration is not a claim of complete standard-library semantics.
- `moved-hook`: the four mutex helpers moved from sync to internal/sync in both
  selected toolchains. The old declarations must be absent in pinned source.
- `replacement`: the target's engine/kernel/control/test-entry replacement, or
  the replacement of a native ChaCha8 linkname by translated computation.
- `package-reject`: the target excludes runtime/debug, runtime/trace, os/signal
  and weak as application packages, superseding their legacy partial hooks.

`{family}` expands independently to go125 and go126. Architecture restrictions
come from selected Linux source: CPU intrinsics differ, and private gettimeofday
exists only on amd64. Go 1.25 has no runtimeClearenv hook; Go 1.26 owns its map
argument. Tests require absent declarations to stay out of selected manifests.
All other targets must exist with the correct declaration kind in their exact
package under the selected build constraints.

The route inventory is `internal/routegen/routes.json`; it retains stable IDs,
owners and legacy route dispositions. Target service coverage and generated
switch coverage are checked separately from the hook ledger. Existing source,
layout, marshaling, runtime and rejection tests establish behavior; this ledger
establishes completeness of migration accounting. Actual translated composition
and public cutover remain Tasks 21 and 22, and certification remains Task 32.
