"""Execute one tuple on the observed native worker and preserve its evidence."""
import argparse
import json
import os
from pathlib import Path
import platform
import subprocess


def arch(value):
    return {"aarch64": "arm64", "arm64": "arm64", "x86_64": "amd64", "amd64": "amd64"}.get(value, value)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--patch", required=True)
    parser.add_argument("--platform", required=True)
    parser.add_argument("--features", required=True)
    parser.add_argument("--race", action=argparse.BooleanOptionalAction, default=False)
    parser.add_argument("--evidence", required=True)
    parser.add_argument("--mode", default="native")
    parser.add_argument("--image", default="")
    parser.add_argument("--concurrency", type=int, default=2)
    args = parser.parse_args()
    if args.concurrency < 1:
        parser.error("concurrency must be positive")
    evidence = Path(args.evidence).resolve()
    evidence.mkdir(parents=True, exist_ok=True)
    env = dict(os.environ, GOTOOLCHAIN=args.patch, GOENV="off", GOMAXPROCS=str(args.concurrency))
    observed = json.loads(subprocess.check_output(
        ["go", "env", "-json", "GOVERSION", "GOROOT", "GOHOSTOS", "GOHOSTARCH", "GOARCH"], env=env))
    observed["uname_system"] = platform.system()
    observed["uname_machine"] = platform.machine()
    (evidence / "worker.json").write_text(json.dumps(observed, indent=2) + "\n")
    host_os, host_arch = args.platform.split("/")
    if (observed["GOVERSION"] != args.patch or observed["GOHOSTOS"] != host_os
            or observed["GOHOSTARCH"] != host_arch or observed["GOARCH"] != host_arch
            or platform.system().lower() != host_os or arch(platform.machine()) != host_arch):
        raise RuntimeError("observed Go/OS/architecture differs from requested tuple")
    selected_go = Path(observed["GOROOT"]) / "bin/go"
    env.update(GOTOOLCHAIN="local", PATH=str(selected_go.parent) + os.pathsep + env["PATH"])
    binary = Path(".gosim/matrix-worker").resolve()
    binary.parent.mkdir(exist_ok=True)
    subprocess.run([str(selected_go), "build", "-ldflags=-checklinkname=0", "-tags=linkname",
                    "-o", str(binary), "./cmd/gosim"], env=env, check=True)
    command = [str(binary), "internal-candidate", "certify", "-go", args.patch,
               "-features", args.features, "-output", str(evidence), "-execution-mode", args.mode,
               "-concurrency", str(args.concurrency)]
    if args.race:
        command.append("-race")
    if args.image:
        command += ["-image-digest", args.image]
    subprocess.run(command, env=env, check=True)


if __name__ == "__main__":
    main()
