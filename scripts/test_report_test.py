"""Regression checks for persisted results and runner failure handling."""

import contextlib
import io
import json
from pathlib import Path
import signal
import tempfile
import unittest
from unittest.mock import MagicMock, patch
import xml.etree.ElementTree as ET

import test_report as report


class FakeProcess:
    def __init__(self, events, code):
        self.stdout = io.StringIO("".join(json.dumps(e) + "\n" for e in events))
        self.code = code
        self.pid = 123456

    def poll(self):
        return None

    def wait(self, timeout=None):
        return self.code


class ReportTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.directory = Path(self.temp.name)
        self.stdout = io.StringIO()
        self.capture = contextlib.redirect_stdout(self.stdout)
        self.capture.__enter__()
        self.addCleanup(self.capture.__exit__, None, None, None)

    def event(self, action, second, name=None, **fields):
        event = dict(Action=action, Time=f"2026-10-12T00:00:{second:02}Z", Package=report.MODULE)
        if name:
            event["Test"] = name
        return dict(event, **fields)

    def run_events(self, events, code=0):
        run = report.Run("unit", self.directory)
        with patch.object(report.subprocess, "Popen", return_value=FakeProcess(events, code)):
            status = run.command("library", ["go", "test", "-json", "./..."])
        status = run.finish(status)
        return run, status

    def test_parallel_wait_is_not_reported_as_execution(self):
        package = report.Package("library", report.MODULE, "library-root", report.timestamp("2026-10-12T00:00:00Z"), self.directory)
        for event in [self.event("run", 1, "TestParallel"), self.event("pause", 2, "TestParallel"),
                      self.event("cont", 8, "TestParallel"), self.event("pass", 10, "TestParallel", Elapsed=3),
                      self.event("pass", 11, Elapsed=11)]:
            package.event(event, report.timestamp(event["Time"]))
        package.finish(report.timestamp("2026-10-12T00:00:11Z"))
        test = package.tests[0]
        self.assertEqual(test.elapsed, 3)
        self.assertEqual(test.end - test.start, 9)
        self.assertEqual([end - start for start, end in test.segments], [1, 2])

    def test_repeats_subtests_and_skip_are_preserved_in_junit(self):
        events = [self.event("start", 0)]
        for second in (1, 4):
            events += [self.event("run", second, "TestParent"),
                       self.event("run", second, "TestParent/child"),
                       self.event("pass", second + 1, "TestParent/child", Elapsed=1),
                       self.event("pass", second + 1, "TestParent", Elapsed=1)]
        events += [self.event("run", 7, "TestSkipped"), self.event("skip", 7, "TestSkipped", Elapsed=0),
                   self.event("pass", 8, Elapsed=8)]
        run, status = self.run_events(events)
        xml = ET.parse(run.directory / "library-root.xml").getroot()
        self.assertEqual(status, 0)
        self.assertEqual(xml.get("tests"), "5")
        self.assertEqual(xml.get("skipped"), "1")
        self.assertEqual(len(xml.findall(".//testcase[@name='TestParent']")), 2)
        self.assertIn("TESTS", self.stdout.getvalue())
        self.assertTrue((run.directory / "timeline.md").exists())
        self.assertTrue((run.directory / "report.md").exists())

    def test_failed_and_aborted_tests_keep_diagnostics_and_nonzero_exit(self):
        run, status = self.run_events([
            self.event("start", 0), self.event("run", 1, "TestFails"),
            self.event("output", 1, "TestFails", Output="failed <assertion> & diagnostic\x00\n"),
            self.event("fail", 2, "TestFails", Elapsed=1),
            self.event("run", 3, "TestAborted"), self.event("fail", 4, Elapsed=4),
        ], code=1)
        xml = ET.parse(run.directory / "library-root.xml").getroot()
        self.assertEqual(status, 1)
        self.assertEqual(xml.get("failures"), "1")
        self.assertEqual(xml.get("errors"), "1")
        self.assertIn("failed <assertion> & diagnostic\x00", (run.directory / "library-root.log").read_text())
        self.assertIn("diagnostic", (run.directory / "library-root.md").read_text())
        self.assertIn("Local tests: FAIL", self.stdout.getvalue())

    def test_teardown_failure_after_pass_is_an_error(self):
        run, status = self.run_events([self.event("start", 0), self.event("run", 1, "TestPasses"),
                                      self.event("pass", 2, "TestPasses", Elapsed=1),
                                      self.event("fail", 3, Elapsed=3)], code=1)
        xml = ET.parse(run.directory / "library-root.xml").getroot()
        self.assertEqual(status, 1)
        self.assertEqual(xml.get("tests"), "2")
        self.assertEqual(xml.get("errors"), "1")

    def test_build_failure_produces_error_report_without_any_test(self):
        run, status = self.run_events([dict(Action="build-output", ImportPath=report.MODULE,
                                           Output="undefined: MissingSymbol\n"),
                                      dict(Action="build-fail", ImportPath=report.MODULE)], code=1)
        xml = ET.parse(run.directory / "library-root.xml").getroot()
        self.assertEqual(status, 1)
        self.assertEqual(xml.get("errors"), "1")
        self.assertIn("MissingSymbol", (run.directory / "library-root.log").read_text())
        self.assertIn("FAIL", (run.directory / "timeline.md").read_text())

    def test_augmented_go_build_name_does_not_duplicate_package_errors(self):
        augmented = f"{report.MODULE} [{report.MODULE}.test]"
        run, status = self.run_events([dict(Action="build-output", ImportPath=augmented,
                                           Output="undefined: MissingSymbol\n"),
                                      dict(Action="build-fail", ImportPath=augmented),
                                      self.event("fail", 1, FailedBuild=augmented)], code=1)
        self.assertEqual(status, 1)
        self.assertEqual(len(run.packages), 1)
        xml = ET.parse(run.directory / "library-root.xml").getroot()
        self.assertEqual(xml.get("errors"), "1")

    def test_successful_compiler_diagnostics_are_not_test_errors(self):
        run, status = self.run_events([dict(Action="build-output", ImportPath="example.com/dependency",
                                           Output="compiler warning retained for investigation\n"),
                                      self.event("start", 0), self.event("pass", 1, Elapsed=1)])
        self.assertEqual(status, 0)
        self.assertEqual(len(run.packages), 1)
        self.assertIn("compiler warning", (run.directory / "command-library.log").read_text())

    def test_missing_executable_still_saves_summary_and_timeline(self):
        run = report.Run("unit", self.directory)
        with patch.object(report.subprocess, "Popen", side_effect=FileNotFoundError("missing executable")):
            status = run.command("docker-build", ["missing"], json_output=False)
        self.assertEqual(run.finish(status), 1)
        self.assertTrue((run.directory / "timeline.md").exists())
        xml = ET.parse(next(run.directory.glob("*.xml"))).getroot()
        self.assertEqual(xml.get("errors"), "1")
        self.assertIn("missing executable", next(run.directory.glob("docker-build-*.log")).read_text())

    def test_success_without_go_events_is_not_a_false_pass(self):
        run, status = self.run_events([])
        self.assertEqual(status, 1)
        self.assertIn("FAIL", (run.directory / "report.md").read_text())

    def test_new_runs_preserve_history_and_update_latest(self):
        first, _ = self.run_events([self.event("start", 0), self.event("pass", 1, Elapsed=1)])
        second, _ = self.run_events([self.event("start", 0), self.event("pass", 1, Elapsed=1)])
        self.assertNotEqual(first.directory, second.directory)
        self.assertTrue((first.directory / "report.md").exists())
        self.assertEqual((self.directory / "unit/latest").resolve(), second.directory)

    def test_local_runner_forwards_budget_and_race_without_containers(self):
        import argparse
        options = argparse.Namespace(suite="unit", parallel=3, packages=2, race=True)
        [(label, command)] = report.commands(options, [])
        self.assertEqual(label, "library")
        self.assertIn("--ordinary", command)
        self.assertIn("--json", command)
        self.assertIn("--race", command)
        self.assertNotIn("--large", command)
        self.assertEqual(command[command.index("--concurrency") + 1], "6")
        self.assertFalse(any(word in " ".join(command) for word in ("docker", "colima", "run-matrix")))

    def test_large_runner_uses_same_reports_and_separate_selection(self):
        import argparse
        options = argparse.Namespace(suite="large", parallel=4, packages=2, race=False)
        [(label, command)] = report.commands(options, [])
        self.assertEqual(label, "library")
        self.assertIn("--large", command)
        self.assertIn("--json", command)
        self.assertNotIn("--race", command)
        run = report.Run("large", self.directory)
        with patch.object(report.subprocess, "Popen", return_value=FakeProcess([
                self.event("start", 0), self.event("run", 1, "TestScript"),
                self.event("pass", 2, "TestScript", Elapsed=1),
                self.event("pass", 3, Elapsed=3)], 0)):
            code = run.command(label, command)
        self.assertEqual(run.finish(code), 0)
        self.assertEqual((self.directory / "large/latest").resolve(), run.directory)
        for filename in ("report.md", "timeline.json", "events.json", "library-root.xml"):
            self.assertTrue((run.directory / filename).is_file(), filename)

    def test_tuple_large_selection_does_not_repeat_sweeps(self):
        import argparse
        for features in ("rt", "rt,fs", "rt,net", "rt,fs,net"):
            options = argparse.Namespace(suite="large", tuple=features, go="/selected/go",
                                         parallel=2, packages=2, race=True)
            [(label, command)] = report.commands(options, [])
            self.assertEqual(label, "library")
            self.assertEqual(command[:2], ["/selected/go", "test"])
            self.assertIn("-race", command)
            self.assertNotIn("./...", command)
            self.assertEqual("./internal/translate" in command, features == "rt")
            self.assertEqual("./internal/tests/script" in command, features == "rt,fs,net")
            self.assertIn(("internal/candidate", "TestCandidateWorker/" + features),
                          report.required_large_tests(features))

    def test_required_large_coverage_rejects_skip_and_missing_subtest(self):
        for outcome in ("pass", "skip", "missing"):
            with self.subTest(outcome=outcome):
                run = report.Run("large", self.directory)
                events = [dict(self.event("start", 0), Package=report.MODULE + "/internal/candidate")]
                for name in ("TestCandidateWorker", "TestCandidateWorker/rt,fs"):
                    if outcome == "missing" and "/" in name:
                        continue
                    events += [dict(self.event("run", 1, name), Package=report.MODULE + "/internal/candidate"),
                               dict(self.event("skip" if outcome == "skip" else "pass", 2, name),
                                    Package=report.MODULE + "/internal/candidate")]
                events.append(dict(self.event("pass", 3), Package=report.MODULE + "/internal/candidate"))
                with patch.object(report.subprocess, "Popen", return_value=FakeProcess(events, 0)):
                    self.assertEqual(run.command("library", ["fake"]), 0)
                result = report.require_passes(run, report.required_large_tests("rt,fs"))
                self.assertEqual(result, int(outcome != "pass"))
                self.assertEqual(run.finish(result), result)

    def test_interruption_marks_running_test_as_error_and_keeps_timeline(self):
        events = [self.event("start", 0), self.event("run", 1, "TestInterrupted")]

        process = FakeProcess(events, 130)
        lines = list(process.stdout)
        def interrupted_lines():
            yield from lines
            raise KeyboardInterrupt()

        stream = MagicMock()
        stream.__iter__.side_effect = interrupted_lines
        process.stdout.close()
        process.stdout = stream
        run = report.Run("unit", self.directory)
        with patch.object(report.subprocess, "Popen", return_value=process), patch.object(report.os, "killpg") as kill:
            status = run.command("library", ["go", "test", "-json"])
        self.assertEqual(run.finish(status), 130)
        kill.assert_called_once_with(process.pid, signal.SIGINT)
        xml = ET.parse(run.directory / "library-root.xml").getroot()
        self.assertEqual(xml.get("errors"), "1")
        self.assertIn("FAIL", (run.directory / "timeline.md").read_text())


if __name__ == "__main__":
    unittest.main()
