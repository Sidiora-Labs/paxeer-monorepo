import subprocess
import unittest
from pathlib import Path


class HumanWorkflowTest(unittest.TestCase):
    def test_workflow_builds_no_retired_binary(self):
        root = Path(__file__).resolve().parents[3]
        workflow = (root / ".github/workflows/human.yml").read_text(encoding="utf-8")
        for binary in ("build/bin/layerxd", "build/tests/bridge/sign-credit"):
            with self.subTest(binary=binary):
                self.assertNotIn(binary, workflow)
        self.assertIn("build/bin/layerx-genesis-build", workflow)

        result = subprocess.run(
            [
                "python3",
                "tools/paxeer-x/tests/retired_callers_test.py",
                "--component",
                "guarantor-runtime",
                "--component",
                "custody-credit",
                "--path",
                ".github/workflows/human.yml",
            ],
            cwd=root,
            capture_output=True,
            text=True,
            check=False,
        )
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)


if __name__ == "__main__":
    unittest.main()
