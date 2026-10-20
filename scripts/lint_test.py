"""Regression checks for changed-file coverage and exact-index hook behavior."""

import contextlib
import io
import json
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

import lint
import lint_staged


class LintTest(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name).resolve()
        self.git("init", "-q")
        self.git("config", "user.name", "Lint Test")
        self.git("config", "user.email", "lint@example.invalid")
        self.git("config", "core.hooksPath", "/dev/null")
        self.write("go.mod", "module example.com/lint\n\ngo 1.25.0\n")
        for name in ("cmd/staged.go", "cmd/pending.go", "cmd/deleted.go", "cmd/rename.go", "cmd/unchanged.go"):
            self.write(name, "package lint\n")
        self.write(".gitignore", "ignored.go\n")
        self.git("add", ".")
        self.git("commit", "-qm", "initial")

    def git(self, *args):
        return lint.git(self.root, *args)

    def write(self, name, text):
        path = self.root / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(text)

    def test_selection_covers_both_diffs_untracked_renames_and_spaces(self):
        self.write("cmd/staged.go", "package lint\n// staged\n")
        self.git("add", "cmd/staged.go")
        # The worktree can cancel a staged edit without dropping its path.
        self.write("cmd/staged.go", "package lint\n")
        self.write("cmd/pending.go", "package lint\n// pending\n")
        self.write("cmd/new directory/new file.go", "package lint\n")
        self.write("cmd/ignored.go", "package lint\n")
        self.write("examples/demo/new.go", "package demo\n")
        self.write("notes.txt", "notes\n")
        self.git("rm", "-q", "cmd/deleted.go")
        self.git("mv", "cmd/rename.go", "cmd/renamed.go")
        self.assertEqual(lint.changed_files(self.root),
                         ["cmd/new directory/new file.go", "cmd/pending.go", "cmd/renamed.go", "cmd/staged.go",
                          "examples/demo/new.go"])
        self.assertEqual(lint.changed_files(self.root, staged=True), ["cmd/renamed.go", "cmd/staged.go"])

    def test_initial_commit_needs_no_head(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            lint.git(root, "init", "-q")
            (root / "cmd").mkdir()
            (root / "cmd/first.go").write_text("package first\n")
            lint.git(root, "add", "cmd/first.go")
            self.assertEqual(lint.changed_files(root, staged=True), ["cmd/first.go"])

    def test_hook_checks_index_without_touching_pending_content(self):
        self.write("cmd/staged.go", "package lint\n// staged version\n")
        self.git("add", "cmd/staged.go")
        self.write("cmd/staged.go", "package lint\n// pending version\n")
        self.write("cmd/untracked.go", "package lint\n")
        before = self.git("diff", "--cached", "--binary")
        observed = []

        def check(snapshot, paths, tools, env):
            observed.append(snapshot)
            self.assertEqual(paths, ["cmd/staged.go"])
            self.assertEqual((snapshot / "cmd/staged.go").read_text(), "package lint\n// staged version\n")
            self.assertFalse((snapshot / "cmd/untracked.go").exists())
            self.assertEqual(env["GOWORK"], "off")
            self.assertNotIn("GIT_INDEX_FILE", env)
            self.assertEqual(env["GOLANGCI_LINT_CACHE"], str(snapshot / ".gosim/lint-cache"))
            return 1

        with patch.object(Path, "cwd", return_value=self.root), \
                patch.object(lint, "find_tools", return_value={}), \
                patch.object(lint, "lint_files", side_effect=check):
            self.assertEqual(lint_staged.main(), 1)
        self.assertEqual(len(observed), 1)
        self.assertFalse(observed[0].exists())
        self.assertEqual(self.git("diff", "--cached", "--binary"), before)
        self.assertEqual((self.root / "cmd/staged.go").read_text(), "package lint\n// pending version\n")

    def test_no_go_changes_does_not_require_tools(self):
        self.write("notes.txt", "notes\n")
        self.write("rt/runtime/ignored.go", "package runtime\n")
        self.write("internal/crypto/ignored.go", "package crypto\n")
        self.write("root.go", "package gosim\n")
        self.git("add", ".")
        with patch.object(Path, "cwd", return_value=self.root), \
                patch.object(lint, "find_tools", side_effect=AssertionError("tools should not run")), \
                contextlib.redirect_stdout(io.StringIO()):
            self.assertEqual(lint.main([]), 0)
            self.assertEqual(lint_staged.main(), 0)

    def test_full_selection_includes_unchanged_and_new_files_only_in_allowed_paths(self):
        self.write("examples/nested/go.mod", "module example.com/nested\n")
        self.write("examples/nested/committed.go", "package nested\n")
        self.write("internal/runtime.go", "package internal\n")
        self.git("add", ".")
        self.git("commit", "-qm", "nested module")
        self.write("cmd/new.go", "package lint\n")
        self.git("add", "cmd/new.go")
        self.write("examples/nested/new.go", "package nested\n")
        self.write("cmd/ignored.go", "package lint\n")
        self.write("examples-other/outside.go", "package outside\n")
        (self.root / "cmd/deleted.go").unlink()
        self.git("rm", "-q", "cmd/rename.go")
        selected = []

        def check(root, paths, tools, env):
            selected.extend(paths)
            return 1

        with patch.object(Path, "cwd", return_value=self.root), \
                patch.object(lint, "find_tools", return_value={}), \
                patch.object(lint, "lint_files", side_effect=check):
            self.assertEqual(lint.main(["--full"]), 1)
        self.assertEqual(selected, ["cmd/new.go", "cmd/pending.go", "cmd/staged.go", "cmd/unchanged.go",
                                    "examples/nested/committed.go", "examples/nested/new.go"])

    def test_nearest_module_owns_new_file(self):
        self.write("examples/nested/go.mod", "module example.com/nested\n")
        self.write("examples/nested/sub/new.go", "package sub\n")
        self.assertEqual(lint.module_directory(self.root, self.root / "examples/nested/sub/new.go"),
                         self.root / "examples/nested")

    def test_diagnostics_filter_whole_files_but_keep_type_errors(self):
        def issue(filename, rule):
            return {"Pos": {"Filename": filename, "Line": 1, "Column": 1},
                    "Text": "finding", "FromLinter": rule}

        findings = [issue("cmd/staged.go", "revive"), issue("cmd/unchanged.go", "revive"),
                    issue("cmd/pending.go", "typecheck")]
        result = subprocess.CompletedProcess([], 1, json.dumps({"Issues": findings}))
        output = io.StringIO()
        with patch.object(lint.subprocess, "run", return_value=result), contextlib.redirect_stdout(output):
            failed = lint.run_analyzer([], self.root, "golangci", {self.root / "cmd/staged.go"}, self.root, {})
        self.assertTrue(failed)
        self.assertIn("staged.go:1:1", output.getvalue())
        self.assertIn("pending.go:1:1", output.getvalue())
        self.assertNotIn("cmd/unchanged.go", output.getvalue())

    def test_tool_errors_are_never_filtered_to_success(self):
        for code, output in [(3, ""), (1, '{"Issues": []}'), (0, "invalid json")]:
            with self.subTest(code=code, output=output), \
                    patch.object(lint.subprocess, "run", return_value=subprocess.CompletedProcess([], code, output)):
                with self.assertRaises(ValueError):
                    lint.run_analyzer([], self.root, "golangci", set(), self.root, {})

    def test_revive_warning_exit_status_is_normalized(self):
        findings = [{"Position": {"Start": {"Filename": "cmd/staged.go", "Line": 1, "Column": 1}},
                     "Failure": "finding", "RuleName": "exported"}]
        result = subprocess.CompletedProcess([], 1, json.dumps(findings))
        with patch.object(lint.subprocess, "run", return_value=result), contextlib.redirect_stdout(io.StringIO()):
            self.assertTrue(lint.run_analyzer([], self.root, "revive", {self.root / "cmd/staged.go"}, self.root, {}))


if __name__ == "__main__":
    unittest.main()
