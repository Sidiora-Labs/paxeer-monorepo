import subprocess
import unittest
from pathlib import Path


class XwebWorkflowTest(unittest.TestCase):
    def test_workflow_has_no_kernel_web_job(self):
        root = Path(__file__).resolve().parents[3]
        commands = (
            (
                "python3",
                "tools/paxeer-x/tests/retired_callers_test.py",
                "--component",
                "oracle-web-roots",
                "--path",
                ".github/workflows/xweb-test.yml",
            ),
            ("bash", "scripts/ci/xweb-workflow-check.sh"),
        )
        for command in commands:
            with self.subTest(command=command):
                result = subprocess.run(
                    command, cwd=root, capture_output=True, text=True, check=False
                )
                self.assertEqual(
                    result.returncode,
                    0,
                    f"{' '.join(command)} failed:\n{result.stdout}{result.stderr}",
                )
