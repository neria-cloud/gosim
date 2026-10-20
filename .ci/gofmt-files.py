#!/usr/bin/env python3
"""Format changed Go files using the policy in .golangci.yml."""

import subprocess
from lint_policy import git_files, repository_root


def main() -> int:
    root = repository_root()
    changed = git_files(root, "diff", "HEAD", "--name-only", "--diff-filter=ACMRT")
    untracked = git_files(root, "ls-files", "--others", "--exclude-standard")
    files = sorted({
        path for path in changed + untracked
        if path.endswith(".go") and (root / path).is_file()
        and not (root / path).is_symlink()
    })
    if not files:
        print("No changed Go files to format.")
        return 0
    # The formatter applies generated/path exclusions from the canonical config.
    # Explicit file arguments keep unrelated working files and the index intact.
    return subprocess.run(
        ["golangci-lint", "fmt", "--config", str(root / ".golangci.yml"),
         *("./" + path for path in files)],
        cwd=root,
    ).returncode


if __name__ == "__main__":
    raise SystemExit(main())
