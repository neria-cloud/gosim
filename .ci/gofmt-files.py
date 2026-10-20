#!/usr/bin/env python3
"""Format changed and untracked Go files without touching unrelated files."""

from __future__ import annotations

import subprocess


def git_files(*args: str) -> list[str]:
    result = subprocess.run(
        ["git", *args, "-z"], check=True, stdout=subprocess.PIPE
    )
    return [name.decode() for name in result.stdout.split(b"\0") if name]


def main() -> int:
    changed = git_files("diff", "HEAD", "--name-only", "--diff-filter=ACMRTUXB")
    untracked = git_files("ls-files", "--others", "--exclude-standard")
    files = sorted({path for path in changed + untracked if path.endswith(".go")})
    if not files:
        print("No changed Go files to format.")
        return 0

    subprocess.run(
        ["go", "run", "golang.org/x/tools/cmd/goimports", "-local=github.com/neria-cloud/gosim", "-format-only", "-w", *files],
        check=True,
    )
    subprocess.run(["go", "run", "mvdan.cc/gofumpt", "-w", *files], check=True)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
