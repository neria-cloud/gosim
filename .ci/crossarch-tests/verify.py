"""Run the repository gate in an explicitly unpromoted source-matched copy."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile

from matrix import ROOT, PATCHES, snapshot, source_files, memory_limit


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


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--ordinary", action="store_true", help="defer the full race pass")
    parser.add_argument("--race", action="store_true", help="run only the local race pass")
    parser.add_argument("--large", action="store_true", help="run only the opt-in expensive suites")
    parser.add_argument("--json", action="store_true", help="stream test events for the report runner")
    parser.add_argument("--allow-worktree-changes", action="store_true",
                        help="report the immutable tested snapshot even if the working checkout changes")
    parser.add_argument("--parallel", type=int, default=0)
    parser.add_argument("--packages", type=int, default=0)
    parser.add_argument("--concurrency", type=int, default=None,
                        help="test worker budget (default: up to 4, bounded by CPUs and memory)")
    args = parser.parse_args()
    if args.large and args.race:
        parser.error("large suites are separate from the race pass")
    if args.concurrency is not None and args.concurrency < 1:
        parser.error("concurrency must be positive")
    if args.parallel < 0 or args.packages < 0:
        parser.error("parallel and packages must be nonnegative")
    concurrency = args.concurrency or min(4, os.cpu_count() or 1,
                                          max(1, memory_limit() // (2 * 1024**3)))
    patch = os.environ.get("GOSIM_TOOLCHAIN")
    if patch not in PATCHES:
        parser.error("set GOSIM_TOOLCHAIN to an exact supported patch")
    parent = ROOT / ".gosim/verification" / patch
    parent.mkdir(parents=True, exist_ok=True)
    output = Path(tempfile.mkdtemp(prefix="gate-", dir=parent))
    source = output / "source"
    inputs = source_files(ROOT)
    metadata = snapshot(source, inputs)
    record = {"toolchain": patch, "ordinary": args.ordinary, "large": args.large,
              "concurrency": concurrency, "source_copy": str(source),
              "candidate_metadata": metadata,
              "input_digest": hashlib.sha256(json.dumps(inputs, sort_keys=True).encode()).hexdigest()}
    evidence = output / "verification.json"
    evidence.write_text(json.dumps(record, indent=2) + "\n")
    print("verification:", output, flush=True)
    env = dict(os.environ, GOTOOLCHAIN=patch, GOENV="off", PYTHONDONTWRITEBYTECODE="1",
               GOMAXPROCS=str(concurrency), GOSIM_TEST_LARGE="1" if args.large else "0",
               GOSIM_LARGE_TUPLE="")
    goroot = subprocess.check_output(["go", "env", "GOROOT"], env=env, text=True).strip()
    go = str(Path(goroot) / "bin/go")
    env.update(GOTOOLCHAIN="local", PATH=str(Path(go).parent) + os.pathsep + env["PATH"])
    binary = source / ".gosim/worker"
    binary.parent.mkdir()
    temporary = source / ".gosim/tmp"
    temporary.mkdir()
    env["TMPDIR"] = str(temporary)
    try:
        subprocess.run(["./.ci/gofmt", "check"], cwd=source, env=env, check=True)
        subprocess.run([go, "build", "-ldflags=-checklinkname=0", "-tags=linkname", "-o", str(binary), "./cmd/gosim"],
                       cwd=source, env=env, check=True)
        command = [str(binary), "internal-candidate", "certify" if args.ordinary or args.race else "verify",
                   "-go", patch, "-features", "rt,fs,net", "-output", str(output / "evidence"),
                   "-concurrency", str(concurrency)]
        if args.json:
            command.append("-json")
        if args.race:
            command.append("-race")
        command += ["-test-parallel", str(args.parallel), "-test-packages", str(args.packages)]
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
