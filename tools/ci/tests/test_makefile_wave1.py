import os
import pathlib
import subprocess
import unittest


ROOT = pathlib.Path(__file__).resolve().parents[3]


class MakefileWaveOneTest(unittest.TestCase):
    def test_retired_targets_have_no_rule(self):
        for target in (
            "test-replay-golden",
            "test-replay-golden-local",
            "test-oracle-adapter",
            "test-oracle-root",
            "test-web-adapter",
            "test-web-intake",
            "test-web-root",
            "test-web-program-path",
            "test-idempotency",
        ):
            with self.subTest(target=target):
                result = subprocess.run(
                    ["make", "-n", target],
                    cwd=ROOT,
                    env={**os.environ, "LC_ALL": "C"},
                    capture_output=True,
                    text=True,
                    check=False,
                )
                self.assertEqual(result.returncode, 2, result.stdout + result.stderr)
                self.assertIn(f"No rule to make target '{target}'", result.stderr)

    def test_aggregates_dry_run_clean(self):
        result = subprocess.run(
            [
                "make",
                "-n",
                "test",
                "core-test-all",
                "test-wave-8",
                "test-journal",
                "test-oracle-intake",
                "test-oracle-bounds",
                "test-oracle-failclosed",
            ],
            cwd=ROOT,
            env={**os.environ, "LC_ALL": "C"},
            capture_output=True,
            text=True,
            check=False,
        )
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        for retired in (
            "lx_oracle_adapter",
            "lx_web_adapter",
            "lx_web_root",
            "lx_oracle_root",
            "lxp_idempotency",
            "replay-golden",
            "lxp_checkpoint_cert",
        ):
            with self.subTest(retired=retired):
                self.assertNotIn(retired, result.stdout)


if __name__ == "__main__":
    unittest.main()
