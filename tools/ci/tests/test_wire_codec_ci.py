import pathlib
import subprocess
import unittest


ROOT = pathlib.Path(__file__).resolve().parents[3]


class WireCodecCiTest(unittest.TestCase):
    def test_ci_names_no_wire_crate(self):
        scan = subprocess.run(
            [
                "python3",
                "tools/paxeer-x/tests/retired_callers_test.py",
                "--component",
                "wire-codec",
                "--path",
                ".github/workflows/interop-x402.yml",
                "--path",
                ".github/workflows/platform.yml",
                "--path",
                "scripts/ci/release-version.py",
            ],
            cwd=ROOT,
            capture_output=True,
            text=True,
            check=False,
        )
        self.assertEqual(scan.returncode, 0, scan.stdout + scan.stderr)

        help_result = subprocess.run(
            ["python3", "scripts/ci/release-version.py", "--help"],
            cwd=ROOT,
            capture_output=True,
            text=True,
            check=False,
        )
        self.assertEqual(
            help_result.returncode, 0, help_result.stdout + help_result.stderr
        )


if __name__ == "__main__":
    unittest.main()
