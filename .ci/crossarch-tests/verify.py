"""Run the repository gate in an explicitly unpromoted source-matched copy."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile

from matrix import ROOT, PATCHES, snapshot, source_files


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--ordinary", action="store_true", help="defer the full race pass")
    parser.add_argument("--concurrency", choices=[1], type=int, default=1)
    args = parser.parse_args()
    patch = os.environ.get("GOSIM_TOOLCHAIN")
    if patch not in PATCHES:
        parser.error("set GOSIM_TOOLCHAIN to an exact supported patch")
    parent = ROOT / ".gosim/verification" / patch
    parent.mkdir(parents=True, exist_ok=True)
    output = Path(tempfile.mkdtemp(prefix="gate-", dir=parent))
    source = output / "source"
    inputs = source_files(ROOT)
    metadata = snapshot(source, inputs)
    record = {"toolchain": patch, "ordinary": args.ordinary, "source_copy": str(source),
              "candidate_metadata": metadata,
              "input_digest": hashlib.sha256(json.dumps(inputs, sort_keys=True).encode()).hexdigest()}
    evidence = output / "verification.json"
    evidence.write_text(json.dumps(record, indent=2) + "\n")
    print("verification:", output, flush=True)
    env = dict(os.environ, GOTOOLCHAIN=patch, GOENV="off", PYTHONDONTWRITEBYTECODE="1")
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
        command = [str(binary), "internal-candidate", "certify" if args.ordinary else "verify",
                   "-go", patch, "-features", "rt,fs,net", "-output", str(output / "evidence")]
        subprocess.run(command, cwd=source, env=env, check=True)
        if source_files(ROOT) != inputs or source_files(source) != inputs:
            raise RuntimeError("source changed during verification")
        record["status"] = "passed"
    except Exception as error:
        record.update(status="failed", error=str(error))
        raise
    finally:
        evidence.write_text(json.dumps(record, indent=2) + "\n")


if __name__ == "__main__":
    main()
