#!/usr/bin/env python3
"""Focused expected-failure checks; never invokes a candidate/matrix/race suite."""

import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import sys


def cases():
    for operation, body, symbol in (
        ("symlink", '_ = os.Symlink("target", probePath)', "syscall.symlinkat"),
        ("readlink", '_, _ = os.Readlink(probePath)', "syscall.readlinkat"),
        ("root", 'root, _ := os.OpenRoot(probePath); if root != nil { root.Close() }', "syscall.openat"),
    ):
        for phase in ("test", "init"):
            yield (f"disabled-{operation}-{phase}", "rt", phase, "", body,
                   "FeatureDisabled", "fs", symbol)
    setup = '''must(os.MkdirAll("/app/guard", 0700))
    must(os.WriteFile("/app/guard/source", []byte("data"), 0600))
    root, err := os.OpenRoot("/app/guard"); must(err)'''
    yield ("root-link", "rt,fs", "test", "", setup + '\n_ = root.Link("source", "link")',
           "UnsupportedOperation", "rt", "syscall.Syscall6")
    yield ("root-chtimes", "rt,fs", "test", '"time"',
           setup + '\n_ = root.Chtimes("source", time.Unix(1, 0), time.Unix(1, 0))',
           "UnsupportedOperation", "rt", "syscall.Syscall6")
    yield ("directory-overflow", "rt,fs", "test", '"fmt"', '''
    must(os.MkdirAll("/app/guard", 0700))
    for n := 0; n < 140; n++ {
        must(os.WriteFile(fmt.Sprintf("/app/guard/%041d", n), nil, 0600))
    }
    _, _ = os.ReadDir("/app/guard")''',
           "UnsupportedOperation", "fs", "getdents64")


def expected_abort(code, output, phase, expected, host_effect):
    pc = re.search(r"pc=(0x[0-9a-f]+)", output)
    # Init aborts occur before the testing banner. The diagnostic itself proves
    # that the specified package initializer reached its disabled operation.
    selected = phase == "init" or "=== RUN   TestGS01Guard" in output
    return (code > 0 and expected in output and selected
            and pc is not None and int(pc.group(1), 16) != 0
            and "GS01_RETURNED" not in output and "GS01_UNWOUND" not in output
            and not host_effect)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--gosim", required=True)
    parser.add_argument("--toolchain", required=True)
    parser.add_argument("--reports", required=True)
    args = parser.parse_args()
    root = Path(__file__).resolve().parents[2]
    launcher = Path(args.gosim).resolve()
    reports = Path(args.reports).resolve()
    reports.mkdir(parents=True, exist_ok=True)
    env = os.environ.copy()
    env["GOTOOLCHAIN"] = args.toolchain
    compiler = subprocess.check_output(
        ["go", "env", "GOVERSION", "GOHOSTOS", "GOHOSTARCH"], cwd=root, env=env, text=True
    ).splitlines()
    template = Path(__file__).with_name("guard.go.txt").read_text()
    # Include tracked and untracked executable inputs, without private env data.
    names = subprocess.check_output(
        ["git", "ls-files", "-z", "--cached", "--others", "--exclude-standard"], cwd=root
    ).decode().split("\0")
    inputs = {}
    for name in sorted(set(names)):
        path = root / name
        if (name and not name.startswith("docs/") and path.is_file()
                and (path.suffix in (".go", ".json") or path.name in ("go.mod", "go.sum"))):
            inputs[name] = hashlib.sha256(path.read_bytes()).hexdigest()
    source_digest = hashlib.sha256(json.dumps(inputs, sort_keys=True).encode()).hexdigest()
    report = {"schema": 1, "compiler": compiler, "source_manifest_sha256": source_digest,
              "source_files": inputs, "cases": []}
    for name, features, phase, imports, body, kind, feature, operation in cases():
        work = root / ".gosim" / "gs01-guards" / args.toolchain / name
        work.mkdir(parents=True, exist_ok=True)
        probe = work / "forbidden-host-effect"
        if probe.exists() or probe.is_symlink():
            raise RuntimeError(f"refusing existing host probe: {probe}")
        source = (template.replace("{{IMPORTS}}", imports).replace("{{BODY}}", body)
                  .replace("{{PROBE}}", json.dumps(str(probe), ensure_ascii=False))
                  .replace("{{INIT}}", "func init() { operation() }" if phase == "init" else ""))
        (work / "guard_test.go").write_text(source)
        # The generated composition loads reviewed shim packages in addition
        # to the worker itself. Seed these tiny probe modules with the exact
        # framework requirements/sums; do not make the translator edit modules
        # or relax its read-only graph checks.
        module = re.sub(r"(?m)^module .+$", "module example.com/gs01-guard",
                        (root / "go.mod").read_text(), count=1)
        module += ("\nrequire github.com/neria-cloud/gosim v0.0.0\n"
                   f"replace github.com/neria-cloud/gosim => {json.dumps(str(root))}\n")
        (work / "go.mod").write_text(module)
        (work / "go.sum").write_bytes((root / "go.sum").read_bytes())
        command = [str(launcher), "-C", str(work), "test", "-features", features,
                   "-seeds", "1", "-run", "^TestGS01Guard$", "."]
        preparation = ["go", "list", "-mod=mod", "-tags=linkname", "-deps",
                       "github.com/neria-cloud/gosim/cmd/gosim-worker"]
        identity_command = [str(launcher), "-C", str(work), "resolve-build", "-features", features]
        identity = None
        try:
            prepared = subprocess.run(preparation, cwd=work, env=env, text=True,
                                      stdout=subprocess.PIPE, stderr=subprocess.STDOUT, timeout=240)
            (reports / f"{name}-prepare.log").write_text(prepared.stdout)
            if prepared.returncode != 0:
                code, output = prepared.returncode, "GUARD PREPARATION FAILED\n" + prepared.stdout
            else:
                identified = subprocess.run(identity_command, cwd=root, env=env, text=True,
                                            stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=240)
                if identified.returncode != 0:
                    code, output = identified.returncode, "GUARD IDENTITY FAILED\n" + identified.stderr + identified.stdout
                else:
                    identity = json.loads(identified.stdout)
                    result = subprocess.run(command, cwd=root, env=env, text=True,
                                            stdout=subprocess.PIPE, stderr=subprocess.STDOUT, timeout=240)
                    code, output = result.returncode, result.stdout
        except subprocess.TimeoutExpired as err:
            code = -1
            output = err.stdout or b""
            if isinstance(output, bytes):
                output = output.decode(errors="replace")
            output += "\nGUARD HARNESS TIMEOUT\n"
        log = reports / f"{name}.log"
        log.write_text(output)
        expected = f"{kind}: feature={feature} operation={operation} machine=1 epoch=1"
        passed = expected_abort(code, output, phase, expected,
                                probe.exists() or probe.is_symlink())
        report["cases"].append({"name": name, "features": features, "phase": phase, "execution": "package-init" if phase == "init" else "test",
                                "command": command, "exit_code": code, "passed": passed,
                                "preparation": preparation, "identity_command": identity_command, "identity": identity,
                                "go_mod_sha256": hashlib.sha256((work / "go.mod").read_bytes()).hexdigest(),
                                "expected": expected, "log": str(log),
                                "fixture_sha256": hashlib.sha256(source.encode()).hexdigest()})
        (reports / "report.json").write_text(json.dumps(report, indent=2) + "\n")
        print(f"{name}: {'PASS' if passed else 'FAIL'} ({log})", flush=True)
    return 0 if all(case["passed"] for case in report["cases"]) else 1


if __name__ == "__main__":
    sys.exit(main())
