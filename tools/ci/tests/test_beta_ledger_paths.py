import os
from pathlib import Path
import subprocess
import tempfile
import unittest


class BetaLedgerPathsTest(unittest.TestCase):
    def test_ledger_check_reads_layerx_merge_ledger(self):
        root = Path(__file__).resolve().parents[3]
        environment = os.environ.copy()
        for name in (
            "PAXEER_X_BETA_LEDGER_FILE",
            "PAXEER_X_BETA_SPEC_FILE",
            "PAXEER_X_BETA_CANDIDATE",
        ):
            environment.pop(name, None)

        with tempfile.TemporaryDirectory(prefix="beta-ledger-paths-") as evidence:
            ledger_check = subprocess.run(
                [
                    "bash",
                    "scripts/ci/beta-ledger-check.sh",
                    "--evidence-root",
                    evidence,
                ],
                cwd=root,
                env=environment,
                capture_output=True,
                text=True,
                timeout=60,
            )
            report_check = subprocess.run(
                ["bash", "tools/ci/beta-report.sh", "--check"],
                cwd=root,
                env=environment,
                capture_output=True,
                text=True,
                timeout=60,
            )

        with self.subTest(script="beta-ledger-check"):
            self.assertEqual(
                ledger_check.returncode,
                0,
                ledger_check.stdout + ledger_check.stderr,
            )
            self.assertIn(
                "spec/layerx-merge/qualification.kvx", ledger_check.stdout
            )
        with self.subTest(script="beta-report"):
            self.assertEqual(
                report_check.returncode,
                0,
                report_check.stdout + report_check.stderr,
            )


if __name__ == "__main__":
    unittest.main()
