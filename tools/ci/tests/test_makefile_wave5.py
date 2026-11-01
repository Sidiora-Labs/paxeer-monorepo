import os
import pathlib
import subprocess
import unittest


ROOT = pathlib.Path(__file__).resolve().parents[3]


class MakefileWaveFiveTest(unittest.TestCase):
    def test_retired_targets_have_no_rule(self):
        for target in (
            "test-batch-wal-recovery",
            "test-maintenance-publication",
            "test-terminal-rejection",
            "test-batch-identity",
            "test-finality-evidence",
            "test-daemon-finality-authority",
            "test-light-credit",
            "test-bridge-credit",
            "custody-proof-build",
            "test-paxeer-membership-sync",
            "test-paxeer-anchor-submit",
            "test-history",
            "test-equivocation",
            "test-versioned-boundaries",
            "test-program-artifacts",
            "test-daemon-maintenance-protocol",
            "test-paxeer-membership-chain",
            "test-paxai-blob-admission",
            "test-paxai-storage-continuity",
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
                "human-test",
                "human-test-integration",
                "test-native-withdrawal-proof",
                "test-paxd-domain",
            ],
            cwd=ROOT,
            env={**os.environ, "LC_ALL": "C"},
            capture_output=True,
            text=True,
            check=False,
        )
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        for retired in (
            "layerx-custody-proof",
            "sign-credit",
            "lxp_daemon_batch_wal",
            "lxp_daemon_finality_authority",
            "native-withdrawal-proof.json",
            "anchor_vectors.json",
            "native_handover_reads",
            "layerxcustody",
        ):
            with self.subTest(retired=retired):
                offending = [line for line in result.stdout.splitlines() if retired in line]
                self.assertEqual(offending, [], "\n".join(offending))
        self.assertIn("LAYERX_EXPLORER_CORE_FIXTURE=", result.stdout)
        self.assertIn("tests/test_asset_withdraw_activity", result.stdout)


if __name__ == "__main__":
    unittest.main()
