import unittest
from matrix import worker_budget


class ParallelBudgetTest(unittest.TestCase):
    def test_automatic_budget_uses_cpu_and_memory(self):
        self.assertEqual(worker_budget(None, 8, 16 * 1024**3, 16), (4, 2))
        self.assertEqual(worker_budget(None, 8, 4 * 1024**3, 16), (1, 2))
        self.assertEqual(worker_budget(None, 1, 1024**3, 16), (1, 1))

    def test_explicit_cap_and_small_matrix(self):
        self.assertEqual(worker_budget(1, 8, 16 * 1024**3, 16), (1, 4))
        self.assertEqual(worker_budget(99, 8, 16 * 1024**3, 16), (4, 2))
        self.assertEqual(worker_budget(None, 8, 16 * 1024**3, 1), (1, 4))

    def test_nested_workers_do_not_multiply_capacity(self):
        for cpus in range(1, 17):
            for gib in range(1, 33):
                jobs, workers = worker_budget(None, cpus, gib * 1024**3, 16)
                self.assertLessEqual(jobs * workers, cpus)
                self.assertLessEqual(jobs * workers, max(1, gib // 2))
