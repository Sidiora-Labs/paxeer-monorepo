import pathlib
import subprocess
import unittest


ROOT = pathlib.Path(__file__).resolve().parents[3]


class BetaDeployRetiredTest(unittest.TestCase):
    def test_root_forge_build_without_beta_script(self):
        build = subprocess.run(
            ["make", "-n", "workspace-build"],
            cwd=ROOT,
            capture_output=True,
            text=True,
            check=False,
        )
        self.assertEqual(build.returncode, 0, build.stdout + build.stderr)
        self.assertFalse((ROOT / "scripts/PaxeerBetaDeploy.s.sol").exists())

        scanner = subprocess.run(
            [
                "python3",
                "tools/paxeer-x/tests/retired_callers_test.py",
                "--component",
                "anchor",
                "--component",
                "custody-claims",
                "--path",
                "scripts",
            ],
            cwd=ROOT,
            capture_output=True,
            text=True,
            check=False,
        )
        self.assertEqual(scanner.returncode, 0, scanner.stdout + scanner.stderr)


if __name__ == "__main__":
    unittest.main()
