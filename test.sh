#!/bin/bash
cd "${0%/*}"
set -e

# The live candidate harness requires the exact development patch and loader PATH.
export GOTOOLCHAIN=go1.26.9
gosim_development_root=$(go env GOROOT)
export PATH="$gosim_development_root/bin:$PATH"
export GOTOOLCHAIN=local
export GOSIMTOOL="$PWD/.gosim/gosimtool"
go run github.com/go-task/task/v3/cmd/task test "$@"
