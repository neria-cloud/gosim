#!/usr/bin/env python3
"""Lint complete packages from the index; report findings in staged Go files."""

from __future__ import annotations

from collections import defaultdict
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile

from lint_policy import generated, git_files, repository_root


def module_root(snapshot: Path, path: Path) -> Path:
    for directory in (path.parent, *path.parent.parents):
        if (directory / "go.mod").is_file():
            return directory
        if directory == snapshot:
            break
    raise ValueError(f"No staged go.mod for {path.relative_to(snapshot)}")


def lint_staged(root: Path) -> int:
    paths = git_files(root, "diff", "--cached", "--name-only", "--diff-filter=ACMRT")
    paths = [path for path in paths if path.endswith(".go")]
    if not paths:
        print("No staged Go files to lint.")
        return 0

    # Avoid compiling partial packages or silently checking unstaged edits.
    # checkout-index neither updates the real index nor changes working files.
    with tempfile.TemporaryDirectory(prefix="gosim-staged-lint-") as temporary:
        snapshot = Path(temporary) / "source"
        snapshot.mkdir()
        subprocess.run(
            ["git", "checkout-index", "--all", f"--prefix={snapshot}{os.sep}"],
            cwd=root, check=True,
        )
        staged = {
            path for path in paths
            if (snapshot / path).is_file()
            and not (snapshot / path).is_symlink()
            and not generated(snapshot / path)
        }
        if not staged:
            print("No staged handwritten Go files to lint.")
            return 0
        modules: dict[Path, set[str]] = defaultdict(set)
        for name in sorted(staged):
            path = snapshot / name
            module = module_root(snapshot, path)
            package = path.parent.relative_to(module).as_posix()
            modules[module].add("." if package == "." else "./" + package)

        # Honor a staged config when present; otherwise use the active local
        # config (e.g. while adopting this workflow for the first time).
        config = snapshot / ".golangci.yml"
        if not config.is_file():
            config = root / ".golangci.yml"
        status = 0
        for module, packages in sorted(modules.items()):
            report = Path(temporary) / "issues.json"
            report.unlink(missing_ok=True)
            print(f"Linting staged packages: {', '.join(sorted(packages))}", flush=True)
            result = subprocess.run(
                [
                    "golangci-lint", "run", "--config", str(config),
                    "--fix=false", "--issues-exit-code=1",
                    "--max-issues-per-linter=0", "--max-same-issues=0",
                    "--show-stats=false", "--output.text.path=" + os.devnull,
                    "--output.json.path=" + str(report), *sorted(packages),
                ],
                cwd=module,
                # Analyzer caches can retain absolute filenames from deleted
                # snapshots. Reusing those results can hide staged findings.
                env={
                    **os.environ, "GOWORK": "off",
                    "GOLANGCI_LINT_CACHE": str(Path(temporary) / "cache"),
                },
            )
            if result.returncode not in (0, 1) or not report.is_file():
                return result.returncode or 2
            data = json.loads(report.read_text())
            if data.get("Report", {}).get("Error"):
                print(data["Report"]["Error"], file=sys.stderr)
                return 2
            issues = data.get("Issues") or []
            if result.returncode == 1 and not issues:
                print("Linter failed without an issue report.", file=sys.stderr)
                return 2
            for issue in issues:
                position = issue["Pos"]
                name = Path(position["Filename"])
                if not name.is_absolute():
                    name = module / name
                relative = os.path.relpath(name, snapshot).replace(os.sep, "/")
                # A package type error invalidates analysis, including when it
                # points at an unchanged sibling needed by a staged file.
                if relative not in staged and issue["FromLinter"] != "typecheck":
                    continue
                print(
                    f"{relative}:{position['Line']}:{position['Column']}: "
                    f"{issue['Text']} ({issue['FromLinter']})"
                )
                status = 1
        return status


def main() -> int:
    if sys.argv[1:] != ["staged"]:
        print(f"usage: {sys.argv[0]} staged", file=sys.stderr)
        return 2
    try:
        return lint_staged(repository_root())
    except (OSError, ValueError, subprocess.CalledProcessError) as error:
        print(f"Staged lint failed: {error}", file=sys.stderr)
        return 2


if __name__ == "__main__":
    raise SystemExit(main())
