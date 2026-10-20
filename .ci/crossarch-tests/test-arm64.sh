#!/bin/bash
set -euo pipefail
exec "$(dirname "$0")/run-matrix.sh" --context colima --platform linux/arm64 "$@"
