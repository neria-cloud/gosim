"""Bounded, isolated certification jobs. Enumeration is never execution evidence."""
import argparse
from concurrent.futures import ThreadPoolExecutor, as_completed
import hashlib
import itertools
import json
import os
from pathlib import Path
import platform
import subprocess
import sys
import shutil
import time

from test_support import ROOT, RunDirectory, memory_limit, source_files, snapshot as copy_snapshot
from worker import CERTIFICATION_PLATFORMS, arch, validate_certification, validate_toolchain

DEFAULT_TOOLCHAINS = ("go1.25.14", "go1.26.9")
PLATFORMS = CERTIFICATION_PLATFORMS
FEATURES = ("rt", "rt,fs", "rt,net", "rt,fs,net")


def enumerate_jobs(platforms, patches, smoke=False):
    for platform in platforms:
        validate_certification(platform)
    for toolchain in patches:
        validate_toolchain(toolchain)
    return [dict(platform=p, patch=v, features=f, race=False)
            for p, v, f in itertools.product(platforms, patches[:1] if smoke else patches,
                                            FEATURES[:1] if smoke else FEATURES)]


def job_name(job):
    return "-".join((job["platform"].replace("/", "-"), job["patch"],
                     job["features"].replace(",", "-"), "race" if job["race"] else "plain"))


def checked(command, log=None, cwd=None, env=None):
    result = subprocess.run(command, cwd=cwd, env=env, text=True, stdout=subprocess.PIPE,
                            stderr=subprocess.STDOUT)
    if log is not None:
        with Path(log).open("a") as stream:
            stream.write(json.dumps(command) + "\n" + result.stdout + "\n")
    if result.returncode:
        raise RuntimeError(f"command failed ({result.returncode}): {command!r}; log: {log}")
    return result.stdout


def snapshot(destination, expected):
    return copy_snapshot(ROOT, destination, expected)


def native_platform():
    return platform.system().lower() + "/" + arch(platform.machine())


def validate_route(native, context, requested, actual):
    if native:
        if requested != actual:
            raise ValueError("native execution requires the requested OS/architecture")
    elif actual != "darwin/arm64" or context != "colima" or not requested.startswith("linux/"):
        raise ValueError("Colima routes require a Darwin ARM64 host and a Linux target; use native CI elsewhere")


def worker_budget(requested_jobs, cpus, memory, count):
    """Share host capacity between isolated tuples and workers within each tuple."""
    capacity = min(max(1, cpus // 2), max(1, memory // (4 * 1024**3)), count)
    jobs = min(requested_jobs or capacity, capacity)
    return jobs, min(4, max(1, cpus // jobs), max(1, memory // jobs // (2 * 1024**3)))


def validate_evidence(directory, job, mode, image):
    reports = list(directory.rglob("report.json"))
    if len(reports) != 1:
        raise RuntimeError(f"expected one completed report, found {len(reports)}")
    path = reports[0]
    report = json.loads(path.read_text())
    identity = report["identity"]
    requested = (job["patch"], *job["platform"].split("/"), job["features"].split(","), job["race"])
    observed = (identity["toolchain"], identity["host_os"], identity["host_arch"], identity["features"], identity["race"])
    if observed != requested or report["execution_mode"] != mode or report["container_image_digest"] != image:
        raise RuntimeError("report differs from executed tuple/route")
    if report.get("validation_only", False):
        raise RuntimeError("ordinary validation report cannot certify a tuple")
    required = {"build", "execute", "native", "large"}
    if len(identity["features"]) < 3:
        required |= {"native-units"}
    seen = set()
    for suite in report["suites"]:
        name = suite["name"]
        if name not in required or name in seen or not suite["passed"]:
            raise RuntimeError("failed, duplicate or unexpected suite")
        seen.add(name)
        data = (path.parent / (name + ".log")).read_bytes()
        if hashlib.sha256(data).hexdigest() != suite["log_digest"]:
            raise RuntimeError("suite log digest mismatch")
    if seen != required:
        raise RuntimeError("mandatory suites missing")
    # Independently bind collected logs to the report that Go already validated.
    canonical = dict(report, result_digest="")
    canonical["suites"] = sorted(report["suites"], key=lambda suite: suite["name"])
    encoded = json.dumps(canonical, ensure_ascii=False, separators=(",", ":"))
    for char, escaped in [("<", "\\u003c"), (">", "\\u003e"), ("&", "\\u0026"), ("\u2028", "\\u2028"), ("\u2029", "\\u2029")]:
        encoded = encoded.replace(char, escaped)
    if hashlib.sha256(encoded.encode()).hexdigest() != report["result_digest"]:
        raise RuntimeError("certification result digest mismatch")
    return {"report": str(path), "result_digest": report["result_digest"],
            "translator_digest": identity["translator_digest"]}


def docker_image(docker, job, run, source):
    stem = job["platform"].replace("/", "-") + "-" + job["patch"]
    log = run / (stem + "-image.log")
    tag = "golang:" + job["patch"].removeprefix("go") + "-bookworm"
    checked(docker + ["pull", "--platform", job["platform"], tag], log)
    digests = json.loads(checked(docker + ["image", "inspect", "--format", "{{json .RepoDigests}}", tag], log))
    pinned = next((d for d in digests if d.startswith("golang@sha256:")), None)
    if not pinned:
        raise RuntimeError("base image has no resolved registry digest")
    iid = run / (stem + "-image.id")
    checked(docker + ["build", "--platform", job["platform"], "--iidfile", str(iid),
                      "--build-arg", "GO_IMAGE=" + pinned, "--build-arg", "GO_PATCH=" + job["patch"],
                      "-f", str(source / ".ci/crossarch-tests/Dockerfile.test"), str(source)], log)
    image = iid.read_text().strip()
    metadata = json.loads(checked(docker + ["image", "inspect", image], log))[0]
    if metadata["Os"] + "/" + arch(metadata["Architecture"]) != job["platform"]:
        raise RuntimeError("built image architecture mismatch")
    return {"image": image, "base_image": pinned}


def execute(job, run, args, inputs, docker, daemon, images):
    started = time.monotonic()
    directory = run / job_name(job)
    directory.mkdir()
    evidence = directory / "evidence"
    evidence.mkdir()
    log = directory / "job.log"
    record = {"tuple": job, "status": "running", "daemon": daemon}
    status = directory / "execution.json"
    status.write_text(json.dumps(record, indent=2) + "\n")
    try:
        validate_certification(job["platform"], job["race"])
        common = ["--patch", job["patch"], "--platform", job["platform"], "--features", job["features"],
                  "--concurrency", str(args.worker_concurrency)]
        if job["race"]:
            common += ["--race"]
        if args.native:
            source = directory / "source"
            record["snapshot_metadata"] = snapshot(source, inputs)
            mode, image = "native", ""
            # Go's content-addressed cache safely separates compiler/source
            # identities. Reuse compilation across runs, never test results.
            cache = os.environ.get("GOCACHE") or str(ROOT / ".gosim/certification/go-build-cache")
            env = dict(os.environ, PYTHONDONTWRITEBYTECODE="1", GOCACHE=cache)
            checked([sys.executable, "-B", str(source / ".ci/crossarch-tests/worker.py"),
                     *common, "--evidence", str(evidence)], log, cwd=source, env=env)
        else:
            image_info = images[(job["platform"], job["patch"])]
            image = image_info["image"]
            record.update(image_info)
            mode = "container-native" if arch(daemon["Architecture"]) == job["platform"].split("/")[1] else "container-emulated"
            cache = "gosim-matrix-" + job["platform"].replace("/", "-") + "-" + job["patch"]
            container = checked(docker + ["create", "--platform", job["platform"],
                                          "--mount", f"type=volume,source={cache},target=/cache",
                                          "-e", "GOMAXPROCS=" + str(args.worker_concurrency), image, *common, "--evidence", "/evidence",
                                          "--mode", mode, "--image", image], log).strip()
            record["container_id"] = container
            failures = []
            try:
                checked(docker + ["start", "--attach", container], log)
                state_info = json.loads(checked(docker + ["inspect", "--format", "{{json .State}}", container], log))
                if state_info["Running"] or state_info["ExitCode"] != 0:
                    raise RuntimeError(f"container did not complete successfully: {state_info}")
            except Exception as error:
                failures.append(str(error))
            finally:
                # Docker copies through the API: no VM mount configuration or
                # assumption that a host /Volumes path exists in Colima.
                try:
                    checked(docker + ["cp", container + ":/evidence/.", str(evidence)], log)
                except Exception as error:
                    failures.append(str(error))
                try:
                    checked(docker + ["rm", "--force", container], log)
                except Exception as error:
                    failures.append(str(error))
            if failures:
                raise RuntimeError("; ".join(failures))
        record.update(validate_evidence(evidence, job, mode, image), status="passed", execution_mode=mode)
        if source_files(ROOT) != inputs:
            raise RuntimeError("source changed during matrix execution")
    except Exception as error:
        record.update(status="failed", error=str(error))
    finally:
        source = directory / "source"
        if source.exists():
            shutil.rmtree(source)
    record["elapsed_seconds"] = round(time.monotonic() - started, 3)
    status.write_text(json.dumps(record, indent=2) + "\n")
    print(job_name(job), record["status"], f"({record['elapsed_seconds']:.1f}s)", flush=True)
    if record["status"] == "failed":
        print(f"  {record['error']}", flush=True)
        for report in sorted(evidence.rglob("report.json")):
            try:
                suites = json.loads(report.read_text()).get("suites", [])
            except (OSError, ValueError):
                continue
            for suite in suites:
                if not suite.get("passed"):
                    print(f"  failed suite log: {report.parent / (suite['name'] + '.log')}", flush=True)
    return record


def main(argv=None):
    started = time.monotonic()
    parser = argparse.ArgumentParser()
    parser.add_argument("--native", action="store_true")
    parser.add_argument("--context", default="colima")
    parser.add_argument("--platform", choices=(*PLATFORMS, "all"), required=True)
    parser.add_argument("--toolchains", default=",".join(DEFAULT_TOOLCHAINS))
    parser.add_argument("--output", default=".gosim/certification")
    parser.add_argument("--jobs", type=int, default=None,
                        help="maximum concurrent tuples (default: CPU/memory bounded)")
    parser.add_argument("--dry-run", action="store_true")
    parser.add_argument("--smoke", action="store_true", help="one ordinary rt tuple per platform; never a full matrix")
    args = parser.parse_args(argv)
    patches = args.toolchains.split(",")
    if not patches or len(set(patches)) != len(patches) or (args.jobs is not None and args.jobs < 1):
        parser.error("require unique exact stable toolchains and a positive job limit")
    platforms = PLATFORMS if args.platform == "all" else (args.platform,)
    try:
        jobs = enumerate_jobs(platforms, patches, args.smoke)
    except ValueError as error:
        parser.error(str(error))
    if args.dry_run:
        print(json.dumps(jobs, indent=2))
        return 0
    for requested in platforms:
        validate_route(args.native, args.context, requested, native_platform())
    output = Path(args.output).resolve()
    if not output.is_relative_to(ROOT / ".gosim"):
        parser.error("output must be under the repository .gosim directory")
    output.mkdir(parents=True, exist_ok=True)
    with RunDirectory(output / "matrix") as run:
        return run_matrix(args, jobs, run, started)


def run_matrix(args, jobs, run, started):
    print("matrix:", run, flush=True)
    inputs = source_files(ROOT)
    (run / "source.json").write_text(json.dumps(inputs, sort_keys=True, indent=2) + "\n")
    docker, daemon, images = [], {}, {}
    available_memory = memory_limit()
    available_cpus = os.cpu_count() or 1
    plan = {"schema": 1, "jobs": jobs, "smoke": args.smoke, "status": "preparing"}
    manifest = run / "matrix.json"
    manifest.write_text(json.dumps(plan, indent=2) + "\n")
    try:
        if not args.native:
            docker = ["docker", "--context", args.context]
            daemon = json.loads(checked(docker + ["info", "--format", '{"Architecture":{{json .Architecture}},"OSType":{{json .OSType}},"MemTotal":{{json .MemTotal}},"NCPU":{{json .NCPU}},"Name":{{json .Name}}}' ], run / "readiness.log"))
            if daemon["OSType"] != "linux":
                raise RuntimeError("Docker daemon is not Linux")
            available_memory = int(daemon["MemTotal"])
            available_cpus = int(daemon["NCPU"])
            source = run / "image-source"
            snapshot_metadata = snapshot(source, inputs)
            for job in jobs:
                key = job["platform"], job["patch"]
                if key not in images:
                    images[key] = docker_image(docker, job, run, source)
                    images[key]["snapshot_metadata"] = snapshot_metadata
    except Exception as error:
        plan.update(status="failed", preparation_error=str(error))
        manifest.write_text(json.dumps(plan, indent=2) + "\n")
        raise
    finally:
        source = run / "image-source"
        if source.exists():
            shutil.rmtree(source)
    concurrency, args.worker_concurrency = worker_budget(args.jobs, available_cpus, available_memory, len(jobs))
    plan = {"schema": 1, "jobs": jobs, "concurrency": concurrency, "memory_bytes": available_memory,
            "worker_concurrency": args.worker_concurrency, "cpus": available_cpus,
            "smoke": args.smoke, "status": "running"}
    manifest = run / "matrix.json"
    manifest.write_text(json.dumps(plan, indent=2) + "\n")
    results = []
    with ThreadPoolExecutor(max_workers=concurrency) as pool:
        futures = [pool.submit(execute, job, run, args, inputs, docker, daemon, images) for job in jobs]
        for future in as_completed(futures):
            results.append(future.result())
            plan["results"] = results
            manifest.write_text(json.dumps(plan, indent=2) + "\n")
    passed = len(results) == len(jobs) and all(r["status"] == "passed" for r in results)
    plan["status"] = "passed" if passed else "failed"
    plan["elapsed_seconds"] = round(time.monotonic() - started, 3)
    manifest.write_text(json.dumps(plan, indent=2) + "\n")
    print(f"matrix: {plan['status']} ({plan['elapsed_seconds']:.1f}s); summary: {manifest}", flush=True)
    return 0 if passed else 1


if __name__ == "__main__":
    try:
        sys.exit(main())
    except (OSError, ValueError, RuntimeError, subprocess.SubprocessError) as error:
        print(error, file=sys.stderr)
        sys.exit(1)
