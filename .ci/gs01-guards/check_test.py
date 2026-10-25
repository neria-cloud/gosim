import importlib.util
from pathlib import Path
import unittest

spec = importlib.util.spec_from_file_location("gs01_guard_check", Path(__file__).with_name("check.py"))
guards = importlib.util.module_from_spec(spec)
spec.loader.exec_module(guards)


class ExpectedFailureChecks(unittest.TestCase):
    def test_no_build_timeout_or_return_is_a_pass(self):
        expected = "FeatureDisabled: feature=fs operation=syscall.openat machine=1 epoch=1"
        diagnostic = expected + " pc=0x1234: disabled"
        output = "=== RUN   TestGS01Guard (seed 1)\n" + diagnostic
        self.assertTrue(guards.expected_abort(1, output, "test", expected, False))
        self.assertTrue(guards.expected_abort(1, diagnostic, "init", expected, False))
        self.assertFalse(guards.expected_abort(1, diagnostic, "test", expected, False))
        self.assertFalse(guards.expected_abort(0, output, "test", expected, False))
        self.assertFalse(guards.expected_abort(-1, output, "test", expected, False))
        self.assertFalse(guards.expected_abort(1, "build failed", "init", expected, False))
        self.assertFalse(guards.expected_abort(1, output, "test", expected, True))
        self.assertFalse(guards.expected_abort(1, output.replace("0x1234", "0x0"), "test", expected, False))
        for marker in ("GS01_RETURNED", "GS01_UNWOUND"):
            self.assertFalse(guards.expected_abort(1, output + marker, "test", expected, False))
        self.assertFalse(guards.expected_abort(1, output.replace("epoch=1", "epoch=2"), "test", expected, False))

    def test_exact_case_inventory(self):
        cases = list(guards.cases())
        self.assertEqual(len(cases), 9)
        self.assertEqual(len({case[0] for case in cases}), 9)
        self.assertEqual(sum(case[2] == "init" for case in cases), 3)
        self.assertEqual({case[1] for case in cases}, {"rt", "rt,fs"})


if __name__ == "__main__":
    unittest.main()
