"""Run a selected local test suite in an immutable source snapshot."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import subprocess
import re
import shutil

from test_support import ROOT, RunDirectory, snapshot, source_files, memory_limit


def check_inputs(root, source, inputs, allow_worktree_changes=False):
    if source_files(source) != inputs:
        raise RuntimeError("tested source copy changed during verification")
    current = source_files(root)
    changed = sorted(name for name in inputs.keys() | current.keys()
                     if inputs.get(name) != current.get(name))
    if changed and not allow_worktree_changes:
        raise RuntimeError("working source changed during verification: " + ", ".join(changed))
    return changed


def large_commands(binary, go, patch, parallel, packages, json_output):
    prepare = [str(binary), "internal-candidate", "prepare-selftest", "-go", patch,
               "-features", "rt,fs,net"]
    tests = [go, "test", "-ldflags=-checklinkname=0", "-tags=linkname", "-count=1",
             "-timeout=30m", "-parallel=" + str(parallel), "-p=" + str(packages),
             "-run=^(TestCandidate|TestSelectedTranslation|TestScript)$",
             "./internal/candidate", "./internal/translate", "./internal/tests/script",
             "-json" if json_output else "-v"]
    return [prepare, tests]


def resolve_toolchain(root, environment):
    """Honor Go's caller-context selection, then freeze the resolved compiler."""
    env = dict(environment)
    override = env.get("GOSIM_TOOLCHAIN", "")
    if override:
        env["GOTOOLCHAIN"] = override
        env.pop("GOROOT", None)
    data = json.loads(subprocess.check_output(
        ["go", "env", "-json", "GOVERSION", "GOROOT", "GOHOSTOS", "GOHOSTARCH"],
        cwd=root, env=env, text=True))
    patch = data["GOVERSION"]
    # Family policy is checked by the Go worker's registry. This check only
    # keeps prerelease/custom strings out of paths and commands.
    if not re.fullmatch(r"go[1-9][0-9]*\.(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)", patch):
        raise ValueError("tests require a stable Go release, resolved " + patch)
    if override and override != patch:
        raise ValueError("requested compiler differs from resolved compiler: " + patch)
    goroot = Path(data["GOROOT"]).resolve()
    go = str(goroot / "bin/go")
    env = dict(env, GOTOOLCHAIN="local", GOROOT=str(goroot),
               PATH=str(goroot / "bin") + os.pathsep + env.get("PATH", ""))
    return patch, go, env


def verification_command(binary, patch, output, concurrency, parallel, packages,
                         json_output=False, race=False):
    command = [str(binary), "internal-candidate", "verify", "-go", patch,
               "-features", "rt,fs,net", "-output", str(output),
               "-concurrency", str(concurrency), "-race" if race else "-ordinary",
               "-test-parallel", str(parallel), "-test-packages", str(packages)]
    if json_output:
        command.append("-json")
    return command


def main():
    parser = argparse.ArgumentParser()
    modes = parser.add_mutually_exclusive_group()
    modes.add_argument("--ordinary", action="store_true", help="run ordinary local tests (default)")
    modes.add_argument("--race", action="store_true", help="run only the explicitly requested local race pass")
    modes.add_argument("--large", action="store_true", help="run only the opt-in expensive suites")
    parser.add_argument("--json", action="store_true", help="stream test events for the report runner")
    parser.add_argument("--allow-worktree-changes", action="store_true",
                        help="report the immutable tested snapshot even if the working checkout changes")
    parser.add_argument("--parallel", type=int, default=0)
    parser.add_argument("--packages", type=int, default=0)
    parser.add_argument("--concurrency", type=int, default=None,
                        help="test worker budget (default: up to 4, bounded by CPUs and memory)")
    args = parser.parse_args()
    if args.concurrency is not None and args.concurrency < 1:
        parser.error("concurrency must be positive")
    if args.parallel < 0 or args.packages < 0:
        parser.error("parallel and packages must be nonnegative")
    concurrency = args.concurrency or min(4, os.cpu_count() or 1,
                                          max(1, memory_limit() // (2 * 1024**3)))
    patch, go, env = resolve_toolchain(ROOT, os.environ)
    parent = ROOT / ".gosim/verification" / patch
    parent.mkdir(parents=True, exist_ok=True)
    mode = "large" if args.large else "race" if args.race else "ordinary"
    with RunDirectory(parent / mode) as output:
        try:
            run_verification(args, patch, go, env, concurrency, output)
        finally:
            source = output / "source"
            if source.exists():
                shutil.rmtree(source)


def run_verification(args, patch, go, env, concurrency, output):
    source = output / "source"
    inputs = source_files(ROOT)
    metadata = snapshot(ROOT, source, inputs)
    record = {"toolchain": patch, "ordinary": not (args.race or args.large), "race": args.race, "large": args.large,
              "concurrency": concurrency, "source_copy": str(source),
              "snapshot_metadata": metadata,
              "input_digest": hashlib.sha256(json.dumps(inputs, sort_keys=True).encode()).hexdigest()}
    evidence = output / "verification.json"
    evidence.write_text(json.dumps(record, indent=2) + "\n")
    print("verification:", output, flush=True)
    env.update(GOENV="off", PYTHONDONTWRITEBYTECODE="1",
               GOMAXPROCS=str(concurrency), GOSIM_TEST_LARGE="1" if args.large else "0",
               GOSIM_LARGE_TUPLE="", GOWORK="off")
    binary = source / ".gosim/worker"
    binary.parent.mkdir()
    temporary = source / ".gosim/tmp"
    temporary.mkdir()
    env["TMPDIR"] = str(temporary)
    try:
        subprocess.run(["./.ci/gofmt", "check"], cwd=source, env=env, check=True)
        subprocess.run([go, "build", "-ldflags=-checklinkname=0", "-tags=linkname", "-o", str(binary), "./cmd/gosim"],
                       cwd=source, env=env, check=True)
        command = verification_command(binary, patch, output / "logs", concurrency,
                                       args.parallel, args.packages, args.json, args.race)
        commands = large_commands(binary, go, patch, args.parallel or 1,
                                  args.packages or 1, args.json) if args.large else [command]
        for command in commands:
            subprocess.run(command, cwd=source, env=env, check=True)
        record["worktree_changes"] = check_inputs(ROOT, source, inputs, args.allow_worktree_changes)
        if record["worktree_changes"]:
            print("Working checkout changed; results apply to the tested snapshot only: "
                  + ", ".join(record["worktree_changes"]), flush=True)
        record["status"] = "passed"
    except Exception as error:
        record.update(status="failed", error=str(error))
        raise
    finally:
        evidence.write_text(json.dumps(record, indent=2) + "\n")


if __name__ == "__main__":
    main()
