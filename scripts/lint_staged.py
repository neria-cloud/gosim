#!/usr/bin/env python3
"""Lint staged Go files in an isolated copy of the Git index (from gquorum)."""

import os
from pathlib import Path
import tempfile

import lint


def main():
    root = Path(os.fsdecode(lint.git(Path.cwd(), "rev-parse", "--show-toplevel")).strip()).resolve()
    paths = lint.changed_files(root, staged=True)
    if not paths:
        return 0
    tools = lint.find_tools(root)
    env = lint.analyzer_environment(root)
    # checkout-index leaves both the index and partially staged worktree intact.
    with tempfile.TemporaryDirectory(prefix="gosim-staged-lint-") as temporary:
        snapshot = Path(temporary).resolve()
        lint.git(root, "checkout-index", "--all", "--prefix=" + str(snapshot) + os.sep)
        # Cached golangci findings can retain absolute paths from another copy
        # of identical sources. Keep index diagnostics inside this snapshot.
        env["GOLANGCI_LINT_CACHE"] = str(snapshot / ".gosim/lint-cache")
        return lint.lint_files(snapshot, paths, tools, env)


if __name__ == "__main__":
    raise SystemExit(lint.entrypoint(main))
