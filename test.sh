#!/bin/bash
set -euo pipefail
cd "${0%/*}"
export GOSIM_TOOLCHAIN="${GOSIM_TOOLCHAIN:-go1.26.9}"
exec ./.ci/verify-step.sh "$@"
