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
    def test_remaining_lni_tree_records_its_retiring_task(self):
        catalogue = json.loads((ROOT / CATALOGUE).read_text(encoding="utf-8"))
        entry = catalogue["components"]["lni"]
        tree = "services/agent/crates/layerx-client/src/lni"
        self.assertEqual(entry["retiring_trees"], {tree: "9.24.7"})
        self.assertNotIn(tree, entry["deleted_trees"])
        result = subprocess.run(
            ["python3", CHECKER, "--deleted", "--component", "lni", "--path", tree],
            cwd=ROOT, capture_output=True, text=True, timeout=60,
        )
        self.assertEqual(result.returncode, 1, result.stdout + result.stderr)
        self.assertIn(tree + ":1: lni: tree retirement pending task 9.24.7", result.stdout)

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

    def test_deleted_trees_absent_and_catalogue_valid(self):
        result = subprocess.run(
            ["python3", CHECKER, "--self-test"],
            cwd=ROOT,
            capture_output=True,
            text=True,
            timeout=60,
        )
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

        components = {
            "guarantor-runtime": ("cmd", "src"),
            "lni": ("cmd", "services/agent"),
            "replica": ("src", "services/platform", "docker"),
            "intent-ingester": ("services/interop/crates", "docker"),
        }
        catalogue = json.loads((ROOT / CATALOGUE).read_text(encoding="utf-8"))
        for component, paths in components.items():
            deleted_trees = catalogue["components"][component]["deleted_trees"]
            for tree in DELETED_TREES[component]:
                with self.subTest(component=component, tree=tree):
                    self.assertIn(
                        tree,
                        deleted_trees,
                        f"{component}: deleted tree missing from catalogue: {tree}",
                    )
            for tree in deleted_trees:
                path = ROOT / tree
                self.assertFalse(
                    path.exists() or path.is_symlink(),
                    f"{component}: deleted tree still exists: {tree}",
                )

            with self.subTest(component=component):
                result = subprocess.run(
                    [
                        "python3",
                        CHECKER,
                        "--deleted",
                        "--component",
                        component,
                        *[arg for path in paths for arg in ("--path", path)],
                    ],
                    cwd=ROOT,
                    capture_output=True,
                    text=True,
                    timeout=60,
                )
                self.assertEqual(result.returncode, 0, result.stdout + result.stderr)


if __name__ == "__main__":
    unittest.main()
