#!/bin/bash
set -euo pipefail
cd "$(dirname "$0")/.."
# Retain the old serialized invocation spelling; no alternate execution mode.
if [[ $# -gt 0 ]]; then
  [[ $# -eq 2 && "$1" == --concurrency && "$2" == 1 ]] || { echo 'unsupported verification arguments' >&2; exit 2; }
fi
gosim_patch="${GOSIM_TOOLCHAIN:?set GOSIM_TOOLCHAIN to the exact patch}"
case "$gosim_patch" in go1.25.14|go1.26.9) ;; *) echo 'unsupported exact patch' >&2; exit 2;; esac
export GOTOOLCHAIN="$gosim_patch"
gosim_selected_root=$(go env GOROOT)
export PATH="$gosim_selected_root/bin:$PATH"
export GOTOOLCHAIN=local
mkdir -p ".gosim/ci/$gosim_patch"
gosim_worker="$PWD/.gosim/ci/$gosim_patch/gosim"
go build -ldflags=-checklinkname=0 -tags=linkname -o "$gosim_worker" ./cmd/gosim
exec "$gosim_worker" internal-candidate verify -go "$gosim_patch" -features rt,fs,net -output "$PWD/.gosim/verification/$gosim_patch"
