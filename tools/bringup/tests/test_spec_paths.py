import subprocess
import unittest
from pathlib import Path


class BringupSpecPathsTest(unittest.TestCase):
    def test_docs_names_reads_existing_spec(self):
        root = Path(__file__).resolve().parents[3]
        docs_names = subprocess.run(
            ["bash", "tools/bringup/docs-names.sh"],
            cwd=root,
            capture_output=True,
            text=True,
            timeout=60,
            check=False,
        )
        check_live = subprocess.run(
            ["bash", "tools/bringup/check-live.sh"],
            cwd=root,
            capture_output=True,
            text=True,
            timeout=60,
            check=False,
        )

        self.assertEqual(
            docs_names.returncode, 0, docs_names.stdout + docs_names.stderr
        )
        usage = check_live.stdout + check_live.stderr
        self.assertEqual(check_live.returncode, 2, usage)
        self.assertIn("usage: tools/bringup/check-live.sh", usage)
        self.assertIn("spec/layerx-merge/spec.kvx", usage)
