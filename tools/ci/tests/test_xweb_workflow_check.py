import subprocess
import unittest
from pathlib import Path


class XwebWorkflowCheckTest(unittest.TestCase):
    def test_check_passes_against_head_tree(self):
        root = Path(__file__).resolve().parents[3]
        result = subprocess.run(
            ["bash", "scripts/ci/xweb-workflow-check.sh"],
            cwd=root,
            capture_output=True,
            text=True,
            check=False,
        )
        self.assertEqual(
            result.returncode,
            0,
            f"xweb workflow check failed:\n{result.stdout}{result.stderr}",
        )
