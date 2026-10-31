import subprocess
import unittest
from pathlib import Path


class NodeBuildWorkflowTest(unittest.TestCase):
    def test_workflow_tests_no_custody_module(self):
        root = Path(__file__).resolve().parents[3]
        result = subprocess.run(
            [
                "python3",
                "tools/paxeer-x/tests/retired_callers_test.py",
                "--component",
                "custody-deposit",
                "--path",
                ".github/workflows/node-build.yml",
            ],
            cwd=root,
            capture_output=True,
            text=True,
            check=False,
        )
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)


if __name__ == "__main__":
    unittest.main()
