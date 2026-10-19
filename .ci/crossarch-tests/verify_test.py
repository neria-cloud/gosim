import tempfile
import unittest
from pathlib import Path

from matrix import source_files
from verify import check_inputs, large_commands


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
