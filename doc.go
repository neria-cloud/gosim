/*
Package gosim provides machine lifecycle controls for deterministic Go simulations.
The fs, net and rt subpackages own filesystem, network and runtime controls.

# Running tests

Gosim translates application Go code with Linux source definitions and builds a
native executable using an exact selected Go patch. Configure one immutable
feature set: rt, rt+fs, rt+net, or rt+fs+net. Filesystem and network require rt.
Normal CLI commands require reviewed certification for the exact tuple and source.

	GOTOOLCHAIN=go1.25.14 go build -ldflags=-checklinkname=0 -tags=linkname -o .gosim/gosimtool ./cmd/gosim
	./.gosim/gosimtool test -go go1.25.14 -features rt -run TestName -seeds 1-20 ./package

See README.md for configuration, the candidate development workflow and current
compatibility evidence. Metatesting runs matching prebuilt simulation workers from
native Go tests; it validates their identities and contents before execution.

# Machines and features

NewMachine returns an unstarted machine. Configure optional services before Start.
Returning from Main gracefully ends that epoch. Stop is graceful, Crash retains
the filesystem crash outcome, and Restart creates a new epoch with fresh globals.
Wait observes the epoch active at entry. Destroy invalidates the control handle.
Machine termination cancels its goroutines without executing their pending defers.

Use the fs package for persistence configuration and owned crash snapshots, the
net package for IPv4/TCP addressing and directed network policy, and rt for timeout
and build/run information. Importing a control package does not enable its feature.
Disabled feature use aborts the simulation even when errors are ignored or panics
are recovered; it never falls back to host I/O.

Outside simulation, error-returning controls return ErrNotSim, IsSim returns false,
and CurrentMachine returns a zero handle. ExitSelf and CrashSelf terminate the
current simulated machine without returning; native calls panic with ErrNotSim.

# Determinism and scope

An identical complete build identity, replay revision and seed reproduces execution.
Scheduler, application, filesystem and network randomness use independent streams.
Only one simulation runs at a time in a host process.

The syscall model retains documented limitations in locking, permissions, mappings
and process services. UDP, IPv6 and unmodified HTTP/gRPC application stacks are not
claimed supported. Capability manifests distinguish implemented, limited, stub and
rejected behavior. See docs/design.md for package ownership and native/translated
boundaries, and docs/reports/syscall-support-for-gquorum.md for historical gaps.
*/
package gosim
