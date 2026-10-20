#!/usr/bin/env python3
"""Display the deterministic ping/pong trace at one second per simulated second."""

import argparse
import datetime
import json
from pathlib import Path
import re
import subprocess
import sys
import time


CASES = ("TestLatency0To5ms", "TestLatency50To100ms", "TestLatency1To2s")
ROOT = Path(__file__).resolve().parents[3]


def read_cases(output, names):
    cases = {}
    current = None
    for line in output.splitlines():
        match = re.fullmatch(r"=== RUN\s+(\w+) \(seed 1\)", line)
        if match:
            current = match[1]
            if current in cases:
                raise ValueError(f"duplicate case: {current}")
            cases[current] = {"events": [], "passed": False}
        elif current and line.startswith("{"):
            record = json.loads(line)
            stamp = datetime.datetime.fromisoformat(record["time"].replace("Z", "+00:00"))
            cases[current]["events"].append((stamp, record["msg"]))
        elif current and line.startswith(f"--- PASS: {current} "):
            cases[current]["passed"] = True
            current = None
        elif line.startswith("--- FAIL:"):
            raise ValueError(line)

    # Validate the entire recording before displaying any case as successful.
    for name in names:
        case = cases.get(name)
        if not case or not case["passed"] or not case["events"]:
            raise ValueError(f"missing successful recording for {name}")
        events = case["events"]
        if not events[0][1].startswith("BEGIN ") or not events[-1][1].startswith("END "):
            raise ValueError(f"incomplete observation for {name}")
        if (events[-1][0] - events[0][0]).total_seconds() != 15:
            raise ValueError(f"{name}: expected a 15-second observation")
        if any(b[0] < a[0] for a, b in zip(events, events[1:])):
            raise ValueError(f"{name}: timestamps are out of order")
    return cases


def display(name, case):
    print(f"\n=== {name}: 15 seconds, playback at 1x ===", flush=True)
    origin = case["events"][0][0]
    started = time.monotonic()
    for stamp, message in case["events"]:
        elapsed = (stamp - origin).total_seconds()
        time.sleep(max(0, started + elapsed - time.monotonic()))
        print(f"[+{elapsed:6.3f}s] {message}", flush=True)
    print(f"--- PASS: {name} (15s simulated; {time.monotonic() - started:.3f}s playback)", flush=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--test", choices=CASES, help="display one case instead of all three")
    parser.add_argument("--log", type=Path, help="replay a saved -v -logformat raw seed-1 recording")
    args = parser.parse_args()
    names = (args.test,) if args.test else CASES

    if args.log:
        output = args.log.read_text()
    else:
        command = [
            str(ROOT / ".gosim/gosimtool"), "test", "-features", "rt,net",
            "-v", "-logformat", "raw", "-seeds", "1",
            "-run", "^(" + "|".join(names) + ")$",
            "./examples/network/latency",
        ]
        print("Building and recording ping/pong simulations before timed playback...", flush=True)
        result = subprocess.run(command, cwd=ROOT, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True)
        if result.returncode:
            print(result.stdout, file=sys.stderr)
            return 1
        output = result.stdout

    cases = read_cases(output, names)
    for name in names:
        display(name, cases[name])
    return 0


if __name__ == "__main__":
    try:
        sys.exit(main())
    except KeyboardInterrupt:
        sys.exit(130)
    except (OSError, ValueError, KeyError) as error:
        sys.exit(f"Visualization failed: {error}")
