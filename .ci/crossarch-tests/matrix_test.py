import argparse
import hashlib
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

import matrix


class MatrixTests(unittest.TestCase):
    def test_complete_enumeration(self):
        jobs = matrix.enumerate_jobs(matrix.PLATFORMS, matrix.PATCHES)
        self.assertEqual(len(jobs), 48)
        self.assertEqual(len({matrix.job_name(j) for j in jobs}), 48)
        for platform in matrix.PLATFORMS:
            self.assertEqual(sum(j["platform"] == platform for j in jobs), 16)
        smoke = matrix.enumerate_jobs(matrix.PLATFORMS, matrix.PATCHES, smoke=True)
        self.assertEqual(len(smoke), 3)
        self.assertTrue(all(not j["race"] and j["features"] == "rt" for j in smoke))

    def test_worker_routing_rejects_cross_compile_as_execution(self):
        for requested in matrix.PLATFORMS:
            matrix.validate_route(True, "", requested, requested)
        matrix.validate_route(False, "colima", "linux/amd64", "darwin/arm64")
        matrix.validate_route(False, "colima", "linux/arm64", "darwin/arm64")
        for native, context, requested, actual in [
            (True, "", "linux/arm64", "linux/amd64"),
            (False, "colima", "linux/arm64", "linux/amd64"),
            (False, "desktop-linux", "linux/amd64", "darwin/arm64"),
            (False, "colima", "darwin/arm64", "darwin/arm64"),
        ]:
            with self.assertRaises(ValueError):
                matrix.validate_route(native, context, requested, actual)

    def test_failed_command_and_missing_report_are_not_passes(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            with self.assertRaises(RuntimeError):
                matrix.checked([matrix.sys.executable, "-c", "print('failure evidence'); exit(7)"], root / "log")
            self.assertIn("failure evidence", (root / "log").read_text())
            with self.assertRaises(RuntimeError):
                matrix.validate_evidence(root, {}, "native", "")

    def test_job_failure_is_persisted_and_returned(self):
        with tempfile.TemporaryDirectory() as tmp:
            job = matrix.enumerate_jobs(["darwin/arm64"], matrix.PATCHES, smoke=True)[0]
            with patch.object(matrix, "snapshot", return_value={}), patch.object(matrix, "checked", side_effect=RuntimeError("worker failed")):
                result = matrix.execute(job, Path(tmp), argparse.Namespace(native=True, worker_concurrency=2), {}, [], {}, {})
            self.assertEqual(result["status"], "failed")
            path = Path(tmp) / matrix.job_name(job) / "execution.json"
            self.assertIn("worker failed", json.loads(path.read_text())["error"])

    def test_container_failure_collects_evidence_and_removes_owned_container(self):
        job = matrix.enumerate_jobs(["linux/arm64"], matrix.PATCHES, smoke=True)[0]
        calls = []

        def command(args, *unused, **keywords):
            calls.append(args)
            if args[1] == "create":
                return "owned-container\n"
            if args[1] == "start":
                raise RuntimeError("suite failed")
            return ""

        with tempfile.TemporaryDirectory() as tmp, patch.object(matrix, "checked", side_effect=command):
            images = {(job["platform"], job["patch"]): {"image": "sha256:image", "base_image": "base"}}
            result = matrix.execute(job, Path(tmp), argparse.Namespace(native=False, worker_concurrency=2), {}, ["docker"], {"Architecture": "aarch64"}, images)
        self.assertEqual(result["status"], "failed")
        self.assertIn("suite failed", result["error"])
        self.assertEqual([cmd[1] for cmd in calls], ["create", "start", "cp", "cp", "rm"])
        self.assertEqual(calls[-1][-1], "owned-container")
        self.assertFalse(any("type=bind" in arg for cmd in calls for arg in cmd))

    def test_report_requires_all_applicable_logs(self):
        job = matrix.enumerate_jobs(["darwin/arm64"], matrix.PATCHES, smoke=True)[0]
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            report = {"identity": {"toolchain": job["patch"], "host_os": "darwin", "host_arch": "arm64",
                                   "features": ["rt"], "race": False, "translator_digest": "source"},
                      "execution_mode": "native", "container_image_digest": "", "result_digest": "result", "suites": []}
            for name in ["build", "execute", "native", "boundaries", "native-units"]:
                (root / (name + ".log")).write_text(name)
                report["suites"].append({"name": name, "passed": True, "log_digest": hashlib.sha256(name.encode()).hexdigest()})
            path = root / "report.json"
            canonical = dict(report, result_digest="")
            canonical["suites"] = sorted(report["suites"], key=lambda suite: suite["name"])
            report["result_digest"] = hashlib.sha256(json.dumps(canonical, separators=(",", ":")).encode()).hexdigest()
            path.write_text(json.dumps(report))
            matrix.validate_evidence(root, job, "native", "")
            report["suites"].pop()
            path.write_text(json.dumps(report))
            with self.assertRaises(RuntimeError):
                matrix.validate_evidence(root, job, "native", "")
            report["suites"][0]["passed"] = False
            path.write_text(json.dumps(report))
            with self.assertRaises(RuntimeError):
                matrix.validate_evidence(root, job, "native", "")

    def test_pending_copy_preserves_source_and_original_certification(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / ".gosim").mkdir()
            (root / "implementation.go").write_text("package implementation")
            original = [{"status": "certified", "toolchain": "go1.25.14", "features": ["rt"],
                         "source_digest": "source", "result_digest": "result"}]
            for family in ("go125", "go126"):
                profile = root / "internal/compat" / family / "profile"
                profile.mkdir(parents=True)
                (profile / "tuples.json").write_text(json.dumps(original))
                (profile / "evidence.json").write_text('[{"reviewed":true}]')
            inputs = matrix.source_files(root)
            destination = root / ".gosim/copy"
            with patch.object(matrix, "ROOT", root):
                metadata = matrix.snapshot(destination, inputs)
            self.assertEqual(inputs, matrix.source_files(destination))
            for family in ("go125", "go126"):
                rel = Path("internal/compat") / family / "profile"
                self.assertEqual(json.loads((root / rel / "tuples.json").read_text()), original)
                copied = json.loads((destination / rel / "tuples.json").read_text())
                self.assertEqual(copied, [{"status": "pending", "toolchain": "go1.25.14", "features": ["rt"]}])
                self.assertEqual(json.loads((destination / rel / "evidence.json").read_text()), [])
                self.assertEqual(metadata[family]["previously_certified"], 1)

    def test_source_copy_ignores_artifacts_but_detects_changed_tests(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / ".gosim").mkdir()
            (root / ".gosim/output").write_text("artifact")
            (root / "code_test.go").write_text("test one")
            before = matrix.source_files(root)
            (root / ".gosim/output").write_text("different artifact")
            self.assertEqual(before, matrix.source_files(root))
            (root / "code_test.go").write_text("test two")
            self.assertNotEqual(before, matrix.source_files(root))


if __name__ == "__main__":
    unittest.main()
