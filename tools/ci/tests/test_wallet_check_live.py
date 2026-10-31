import pathlib
import subprocess
import unittest


ROOT = pathlib.Path(__file__).resolve().parents[3]


class WalletCheckLiveTest(unittest.TestCase):
    def test_check_live_names_no_lx_method(self):
        scan = subprocess.run(
            [
                "python3",
                "tools/paxeer-x/tests/retired_callers_test.py",
                "--component",
                "kernel-web",
                "--component",
                "layerxctl-port",
                "--path",
                "scripts/wallet/check-live.sh",
            ],
            cwd=ROOT,
            capture_output=True,
            text=True,
            check=False,
        )
        self.assertEqual(scan.returncode, 0, scan.stdout + scan.stderr)

        usage = subprocess.run(
            ["bash", "scripts/wallet/check-live.sh"],
            cwd=ROOT,
            capture_output=True,
            text=True,
            check=False,
        )
        self.assertEqual(usage.returncode, 2, usage.stdout + usage.stderr)
        self.assertIn(
            "usage: scripts/wallet/check-live.sh endpoint | attestors | gateway | "
            "cutover | human | human-session\n",
            usage.stderr,
        )


if __name__ == "__main__":
    unittest.main()
