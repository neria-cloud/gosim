#!/bin/bash
set -euo pipefail
cd "${0%/*}"
exec ./.ci/verify-step.sh "$@"
