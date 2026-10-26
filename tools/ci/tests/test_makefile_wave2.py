import os
import pathlib
import subprocess
import unittest


ROOT = pathlib.Path(__file__).resolve().parents[3]


class MakefileWaveTwoTest(unittest.TestCase):
    def test_retired_targets_have_no_rule(self):
        for target in (
            "test-da",
            "test-da-possession",
            "test-da-verified",
            "test-da-retrieval",
            "test-da-challenge",
            "test-da-unavailable",
            "test-replica",
            "test-replay",
            "test-replica-divergence",
            "test-batch-seal",
            "test-batch-distribute",
            "test-gateway-receive",
            "test-guarantor",
            "test-guarantor-cert",
            "test-guarantor-bond",
            "test-guarantor-disagreement",
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
                "test-wave-10",
                "test-wave-12",
                "layerx-module-registry",
            ],
            cwd=ROOT,
            env={**os.environ, "LC_ALL": "C"},
            capture_output=True,
            text=True,
            check=False,
        )
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        for retired in (
            "lxp_da_",
            "lxp_replica",
            "lxp_batch_seal",
            "test_batch_publish",
            "test_gateway_receive",
            "lxp_guarantor",
            "cmd/layerx-module-registry/node.c",
        ):
            with self.subTest(retired=retired):
                self.assertNotIn(retired, result.stdout)


if __name__ == "__main__":
    unittest.main()
