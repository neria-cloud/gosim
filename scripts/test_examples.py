#!/usr/bin/env python3
"""Run standalone example modules against the local Gosim checkout."""

import argparse
import os
from pathlib import Path
import subprocess


ROOT = Path(__file__).resolve().parents[1]
# The filesystem fixture deliberately asserts an exact compiler version.
EXAMPLES = (
    ("simple", "rt", None, True),
    ("filesystem", "rt,fs", "go1.25.2", False),
    ("bolt", "rt,fs", None, False),
    ("network/machines", "rt,fs,net", None, False),
    ("network/latency", "rt,net", None, False),
    ("network/grpc", "rt,net", None, False),
    ("network/nats", "rt,fs,net", None, False),
    ("etcd", "rt,fs,net", None, True),
)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--gosim", type=Path, default=ROOT / ".gosim/gosimtool")
    args = parser.parse_args()
    launcher = args.gosim.resolve()
    for name, features, compiler, native in EXAMPLES:
        directory = ROOT / "examples" / name
        env = dict(os.environ, GOWORK="off")
        compiler = compiler or env.get("GOSIM_TOOLCHAIN")
        if compiler:
            env["GOTOOLCHAIN"] = compiler
            env.pop("GOROOT", None)
        print(f"Testing examples/{name} ({features})", flush=True)
        subprocess.run([str(launcher), "-C", str(directory), "test",
                        "-features", features, "-v", "."], cwd=ROOT, env=env, check=True)
        if native:
            # Native replay metatests resolve the same local source and compiler.
            subprocess.run(["go", "test", "-ldflags=-checklinkname=0", "-tags=linkname",
                            "-count=1", "-v", "."], cwd=directory, env=env, check=True)


if __name__ == "__main__":
    main()
