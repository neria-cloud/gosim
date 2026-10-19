import json
import tempfile
import unittest
from pathlib import Path

from unittest.mock import patch

from test_support import source_files, snapshot
from verify import check_inputs, large_commands, resolve_toolchain, verification_command


class VerificationInputsTest(unittest.TestCase):
    def test_large_runs_only_parent_suites_after_preparation(self):
        prepare, tests = large_commands(Path("/worker"), "/go", "go1.26.9", 4, 2, True)
        self.assertEqual(prepare, ["/worker", "internal-candidate", "prepare-selftest",
                                   "-go", "go1.26.9", "-features", "rt,fs,net"])
        self.assertIn("-json", tests)
        self.assertIn("-parallel=4", tests)
        self.assertIn("-p=2", tests)
        self.assertEqual([arg for arg in tests if arg.startswith("./")],
                         ["./internal/candidate", "./internal/translate", "./internal/tests/script"])
        import re
        pattern = next(arg.removeprefix("-run=") for arg in tests if arg.startswith("-run="))
        for name in ("TestCandidate", "TestSelectedTranslation", "TestScript"):
            self.assertIsNotNone(re.fullmatch(pattern, name))
        for name in ("TestCandidateWorker", "TestSelectedTranslationWorker", "TestOther"):
            self.assertIsNone(re.fullmatch(pattern, name))

    def test_local_snapshot_survives_worktree_edit(self):
        with tempfile.TemporaryDirectory() as tmp:
            root, source = Path(tmp) / "checkout", Path(tmp) / "snapshot"
            for directory in (root, source):
                directory.mkdir()
                (directory / "Makefile").write_text("UNIT_PARALLEL ?= 5\n")
                (directory / "code.go").write_text("package example\n")
            inputs = source_files(root)
            self.assertEqual(check_inputs(root, source, inputs), [])

            (root / "Makefile").write_text("UNIT_PARALLEL ?= 4\n")
            self.assertEqual(check_inputs(root, source, inputs, True), ["Makefile"])
            with self.assertRaisesRegex(RuntimeError, "working source.*Makefile"):
                check_inputs(root, source, inputs)

            (root / "code.go").unlink()
            (root / "new.go").write_text("package example\n")
            self.assertEqual(check_inputs(root, source, inputs, True),
                             ["Makefile", "code.go", "new.go"])

            (source / "code.go").write_text("package changed\n")
            with self.assertRaisesRegex(RuntimeError, "tested source copy changed"):
                check_inputs(root, source, inputs, True)


class LocalExecutionPolicyTest(unittest.TestCase):
    def test_verification_selects_one_mode_without_certification(self):
        for race in (False, True):
            with self.subTest(race=race):
                command = verification_command(Path("/worker"), "go1.25.1", Path("/logs"),
                                               4, 2, 2, True, race)
                self.assertEqual(command[1:3], ["internal-candidate", "verify"])
                self.assertIn("-race" if race else "-ordinary", command)
                self.assertNotIn("-ordinary" if race else "-race", command)
                self.assertNotIn("certify", command)
                self.assertNotIn("-large", command)
                self.assertIn("-json", command)

    def test_go_resolves_default_compiler_and_children_freeze_it(self):
        for version in ("go1.25.1", "go1.25.5", "go1.26.9"):
            with self.subTest(version=version), tempfile.TemporaryDirectory() as tmp:
                root = Path(tmp)
                data = json.dumps(dict(GOVERSION=version, GOROOT=tmp,
                                       GOHOSTOS="darwin", GOHOSTARCH="arm64"))
                original = {"PATH": "/bin", "GOTOOLCHAIN": "auto"}
                with patch("verify.subprocess.check_output", return_value=data) as command:
                    resolved, go, env = resolve_toolchain(root, original)
                self.assertEqual(resolved, version)
                self.assertEqual(go, str(root.resolve() / "bin/go"))
                self.assertEqual(command.call_args.kwargs["env"]["GOTOOLCHAIN"], "auto")
                self.assertEqual(command.call_args.kwargs["cwd"], root)
                self.assertEqual(env["GOTOOLCHAIN"], "local")
                self.assertEqual(env["GOROOT"], str(root.resolve()))
                self.assertEqual(original, {"PATH": "/bin", "GOTOOLCHAIN": "auto"})

    def test_explicit_override_is_resolved_and_checked(self):
        data = json.dumps(dict(GOVERSION="go1.26.9", GOROOT="/compiler"))
        with patch("verify.subprocess.check_output", return_value=data) as command:
            resolve_toolchain(Path("/checkout"), {"GOSIM_TOOLCHAIN": "go1.26.9", "GOROOT": "/other"})
            self.assertEqual(command.call_args.kwargs["env"]["GOTOOLCHAIN"], "go1.26.9")
            self.assertNotIn("GOROOT", command.call_args.kwargs["env"])
            with self.assertRaisesRegex(ValueError, "differs"):
                resolve_toolchain(Path("/checkout"), {"GOSIM_TOOLCHAIN": "go1.25.1"})
        for version in ("devel go1.27", "go1.26rc1", "go1.26.9-custom", "../bad"):
            with patch("verify.subprocess.check_output", return_value=json.dumps(dict(GOVERSION=version))):
                with self.assertRaisesRegex(ValueError, "stable Go release"):
                    resolve_toolchain(Path("/checkout"), {})

    def test_local_snapshot_preserves_existing_evidence(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp) / "checkout"
            profile = root / "internal/compat/go125/profile"
            profile.mkdir(parents=True)
            (root / "main.go").write_text("package main\n")
            # Historical metadata is not interpreted, reset or required.
            (profile / "evidence.json").write_text("existing evidence")
            source = Path(tmp) / "source"
            snapshot(root, source, source_files(root))
            self.assertEqual((source / "internal/compat/go125/profile/evidence.json").read_text(),
                             "existing evidence")
            self.assertFalse((source / "internal/compat/go126").exists())
