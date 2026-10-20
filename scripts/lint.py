#!/usr/bin/env python3
"""Lint changed or all Go files under cmd/ and examples/, with package context."""

import argparse
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys


TOOLS = {
    "golangci": ("GOLANGCI", ".gosim/lint/golangci-v2.14.0/golangci-lint", "golangci-lint"),
    "revive": ("REVIVE", ".gosim/lint/revive-v1.17.1/revive", "revive"),
}


def in_scope(path):
    return path.endswith(".go") and path.startswith(("cmd/", "examples/"))


def git(root, *args):
    return subprocess.check_output(["git", *args], cwd=root)


def changed_files(root, staged=False):
    """Use separate diffs so pending edits cannot cancel staged path selection."""
    if git(root, "diff", "--name-only", "--diff-filter=U", "-z"):
        raise ValueError("resolve unmerged files before linting")
    commands = [("diff", "--cached", "--name-only", "--no-renames", "--diff-filter=ACM", "-z")]
    if not staged:
        commands += [
            ("diff", "--name-only", "--no-renames", "--diff-filter=ACM", "-z"),
            ("ls-files", "--others", "--exclude-standard", "-z"),
        ]
    return sorted({os.fsdecode(path) for args in commands for path in git(root, *args).split(b"\0")
                   if in_scope(os.fsdecode(path))})


def all_files(root):
    """Include unchanged tracked files and non-ignored new files in allowed paths."""
    if git(root, "diff", "--name-only", "--diff-filter=U", "-z"):
        raise ValueError("resolve unmerged files before linting")
    paths = git(root, "ls-files", "--cached", "--others", "--exclude-standard", "-z", "--", "cmd", "examples")
    return sorted({os.fsdecode(path) for path in paths.split(b"\0") if in_scope(os.fsdecode(path))})


def analyzer_environment(root):
    env = os.environ.copy()
    for name in git(root, "rev-parse", "--local-env-vars").decode().splitlines():
        env.pop(name, None)
    env["GOWORK"] = "off"
    # Golangci caches absolute diagnostic paths; do not share them across worktrees.
    env["GOLANGCI_LINT_CACHE"] = str(root / ".gosim/lint-cache")
    if env.get("GOSIM_TOOLCHAIN"):
        env["GOTOOLCHAIN"] = env["GOSIM_TOOLCHAIN"]
    env["GOFLAGS"] = env.get("GOFLAGS", "") + " -ldflags=-checklinkname=0"
    return env


def find_tools(root):
    tools = {}
    for name, (variable, local, command) in TOOLS.items():
        candidate = os.environ.get(variable)
        if not candidate:
            candidate = str(root / local) if (root / local).is_file() else command
        executable = shutil.which(candidate)
        if not executable:
            raise ValueError(f"{candidate} not found; run make lint-tools or set {variable}")
        tools[name] = str(Path(executable).resolve())
    return tools


def module_directory(root, path):
    directory = path.parent
    while not (directory / "go.mod").is_file():
        if directory == root:
            raise ValueError(f"no go.mod owns {path.relative_to(root)}")
        directory = directory.parent
    return directory


def json_objects(output):
    decoder = json.JSONDecoder()
    while output.strip():
        output = output.lstrip()
        value, end = decoder.raw_decode(output)
        yield value
        output = output[end:]


def run_analyzer(command, directory, tool, selected, root, env):
    result = subprocess.run(command, cwd=directory, env=env, stdout=subprocess.PIPE, text=True)
    if result.returncode not in (0, 1):
        print(result.stdout, end="")
        raise ValueError(f"{tool} failed with exit status {result.returncode}")
    # Never turn a tool failure or an unexpected output format into a pass.
    data = json.loads(result.stdout)
    issues = (data.get("Issues") or []) if tool == "golangci" else (data or [])
    if result.returncode and not issues:
        raise ValueError(f"{tool} failed without diagnostics")
    failed = False
    for issue in issues:
        if tool == "golangci":
            position, message, rule = issue["Pos"], issue["Text"], issue["FromLinter"]
        else:
            position, message, rule = issue["Position"]["Start"], issue["Failure"], issue["RuleName"]
        path = Path(position["Filename"])
        if not path.is_absolute():
            path = directory / path
        path = path.resolve()
        if path not in selected and rule != "typecheck":
            continue
        name = path.relative_to(root) if path.is_relative_to(root) else path
        print(f"{name}:{position['Line']}:{position['Column']}: {message} ({rule})", flush=True)
        failed = True
    return failed


def lint_files(root, paths, tools, env):
    selected = {(root / path).resolve() for path in paths if in_scope(path) and (root / path).is_file()}
    modules = {}
    for path in sorted(selected):
        modules.setdefault(module_directory(root, path), set()).add(path.parent)
    failed = False
    for module, directories in sorted(modules.items()):
        patterns = ["./" + str(directory.relative_to(module)) for directory in sorted(directories)]
        listing = subprocess.check_output(
            ["go", "list", "-e", "-json", "-tags=linkname", *patterns], cwd=module, env=env, text=True,
        )
        native = []
        revive_inputs = {}
        for package in json_objects(listing):
            # Other compiler families and sim-only packages are still checked
            # by revive below, which parses explicit files without build tags.
            if package.get("Error") and not (
                package.get("IgnoredGoFiles") and not any(package.get(key) for key in
                    ("GoFiles", "CgoFiles", "TestGoFiles", "XTestGoFiles"))
                and "build constraints exclude all Go files" in package["Error"]["Err"]
            ):
                raise ValueError(package["Error"]["Err"])
            if any(package.get(key) for key in ("GoFiles", "CgoFiles", "TestGoFiles", "XTestGoFiles")):
                native.append("./" + str(Path(package["Dir"]).relative_to(module)))
                # A directory preserves package context, including doc.go.
                revive_inputs[Path(package["Dir"])] = [package["Dir"]]
            else:
                revive_inputs[Path(package["Dir"])] = []
            # Explicit files also cover sim-only examples. Revive's directory
            # discovery otherwise ignores sources excluded by build tags.
            revive_inputs[Path(package["Dir"])].extend(
                str(Path(package["Dir"]) / name) for name in package.get("IgnoredGoFiles", ())
            )
        if native:
            failed |= run_analyzer(
                [tools["golangci"], "run", "--config", str(root / ".golangci.yml"),
                 "--build-tags=linkname", "--timeout=5m", "--max-issues-per-linter=0",
                 "--max-same-issues=0", "--uniq-by-line=false", "--output.text.path=",
                 "--output.json.path=stdout", "--show-stats=false", "--path-mode=abs", *native],
                module, "golangci", selected, root, env,
            )
        for directory in sorted(directories):
            failed |= run_analyzer(
                [tools["revive"], "-config", str(root / "revive.toml"), "-formatter", "json",
                 "-set_exit_status", *revive_inputs[directory]],
                module, "revive", selected, root, env,
            )
    return int(failed)


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--full", action="store_true", help="lint all Go files in the allowed paths, including unchanged files")
    args = parser.parse_args(argv)
    root = Path(os.fsdecode(git(Path.cwd(), "rev-parse", "--show-toplevel")).strip()).resolve()
    paths = [path for path in (all_files(root) if args.full else changed_files(root)) if (root / path).is_file()]
    if not paths:
        return 0
    return lint_files(root, paths, find_tools(root), analyzer_environment(root))


def entrypoint(run):
    try:
        return run()
    except (OSError, subprocess.CalledProcessError, ValueError, KeyError, TypeError) as error:
        print(f"Lint failed: {error}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(entrypoint(main))
