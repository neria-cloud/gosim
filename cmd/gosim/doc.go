/*
Gosim translates, builds and executes deterministic simulation tests.

Build the CLI with the same exact patch requested by its commands:

	GOTOOLCHAIN=go1.25.14 go build -ldflags=-checklinkname=0 -tags=linkname -o .gosim/gosimtool ./cmd/gosim

# Configuration

Commands share gosim.json, -config (or GOSIM_CONFIG), -go, -features and -race.
Features must be explicitly selected as rt, rt,fs, rt,net, or rt,fs,net; a command
line feature selection replaces the entire configured list. The simulated OS is
Linux, while the executable uses the actual supported host OS/architecture.

Normal commands do not require certification evidence. Compatibility and artifact
checks bind the actual compiler and source. Certification is an optional manual
operation, outside CI and ordinary tests.

# Commands

	gosim test -go go1.25.14 -features rt -run TestName -v -seeds 1-20 ./package
	gosim translate -go go1.25.14 -features rt ./package
	gosim build-tests -go go1.25.14 -features rt ./package
	gosim prepare-selftest -go go1.25.14 -features rt,fs,net
	gosim debug -go go1.25.14 -features rt -package ./package -test TestName -seed 1 -step 11

Test translates/builds applicable packages and executes the resulting binaries.
The seed syntax accepts comma-separated values and ranges. Trace output can be
selected with -simtrace. Debug runs a matching binary through Delve and supports
-headless; it requires an installed dlv executable.

Translate applies selected Go-family hooks/shims and generated composition before
package loading. Build-tests publishes content-verified artifacts for metatesting.
Artifacts are stored under .gosim/v2/<identity>/packages/<package-digest>/ with a
manifest for each package build. Matching identity alone cannot reuse another
package's contents. Prepare-selftest creates matching host test infrastructure.

Internal-candidate test/translate/build-tests/prepare-selftest use the same build
selection as ordinary commands. Internal-candidate certify manually records one
exact build's actual suites and requires explicit large-suite coverage. Verify
runs ordinary tests by default or only race tests with -race. Repository test
scripts use immutable source copies without rewriting certification metadata.
CI invokes make test and publishes test-reports; it does not invoke certify.
*/
package main
