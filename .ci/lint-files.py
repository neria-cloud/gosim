#!/usr/bin/env python3
"""Run golangci-lint on staged Go source files, grouped by directory."""

from __future__ import annotations

import subprocess
import sys
from collections import defaultdict


def git_files(*args: str) -> list[str]:
    result = subprocess.run(
        ["git", "-C", ".", *args, "-z"],
        check=True,
        stdout=subprocess.PIPE,
    )
    return [name.decode() for name in result.stdout.split(b"\0") if name]


def main() -> int:
    if len(sys.argv) != 2 or sys.argv[1] != "staged":
        print(f"usage: {sys.argv[0]} staged", file=sys.stderr)
        return 2

    paths = git_files("diff", "--cached", "--name-only", "--diff-filter=ACMRTUXB")
    go_files = [path for path in paths if path.endswith(".go")]
    if not go_files:
        print("No staged Go files to lint.")
        return 0

    grouped: dict[str, list[str]] = defaultdict(list)
    for path in go_files:
        grouped[path.rpartition("/")[0] or "."].append(path)

    for directory, files in sorted(grouped.items()):
        print(f"Linting staged Go files in {directory}...")
        result = subprocess.run(["golangci-lint", "run", *files])
        if result.returncode:
            return result.returncode
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
