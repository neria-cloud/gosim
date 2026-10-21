"""Source identity utility checks; no validation/certification job is launched."""
from pathlib import Path
import tempfile
import unittest

from test_support import source_files


class SourceIdentityTest(unittest.TestCase):
    def test_only_history_is_excluded(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            names = ["internal/compat/profiles/evidence.json",
                     "internal/compat/profiles/go125.origin.json",
                     "internal/compat/profiles/go125.json",
                     "internal/compat/profiles/nested/evidence.json"]
            for name in names:
                path = root / name
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_text("old")
            before = source_files(root)
            self.assertEqual(set(before), set(names[1:]))
            for name in names[:1]:
                (root / name).write_text("history changed")
            self.assertEqual(before, source_files(root))
            for name in names[1:]:
                (root / name).write_text("source changed")
                self.assertNotEqual(before[name], source_files(root)[name])


if __name__ == "__main__":
    unittest.main()
