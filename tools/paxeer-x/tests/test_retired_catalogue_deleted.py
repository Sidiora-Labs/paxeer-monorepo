import json
from pathlib import Path
import subprocess
import unittest


ROOT = Path(__file__).resolve().parents[3]
CATALOGUE = "tools/paxeer-x/retired_callers.json"
CHECKER = "tools/paxeer-x/tests/retired_callers_test.py"
DELETED_TREES = {
    "finality-authority": (
        "cmd/layerxd",
        "cmd/layerx-handover",
        "cmd/layerx-verify",
        "tests/daemon",
    ),
    "guarantor-runtime": ("cmd/layerx-guarantor",),
    "lni": ("cmd/layerxd",),
    "replica": ("docker/relay-archive",),
    "intent-ingester": ("docker/intent-ingester",),
}


class RetiredCatalogueDeletedTest(unittest.TestCase):
    def test_catalogue_valid_and_self_test_passes(self):
        result = subprocess.run(
            ["python3", CHECKER, "--self-test"],
            cwd=ROOT,
            capture_output=True,
            text=True,
            timeout=60,
        )
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

        catalogue = json.loads((ROOT / CATALOGUE).read_text(encoding="utf-8"))
        for component, trees in DELETED_TREES.items():
            for tree in trees:
                with self.subTest(component=component, tree=tree):
                    self.assertIn(
                        tree,
                        catalogue["components"][component]["deleted_trees"],
                        f"{component}: deleted tree missing from catalogue: {tree}",
                    )


if __name__ == "__main__":
    unittest.main()
