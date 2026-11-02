#!/usr/bin/env python3
from pathlib import Path
import subprocess
import unittest

ROOT = Path(__file__).resolve().parents[3]


class GatesDeletedSourcesTest(unittest.TestCase):
    def test_gates_name_no_deleted_source(self):
        for name in (
            "tools/paxeer-x/tests/versioned_boundaries.py",
            "tools/paxeer-x/gates/0.5.sh",
        ):
            with self.subTest(path=name):
                path = ROOT / name
                self.assertFalse(path.exists() or path.is_symlink(), name)

        result = subprocess.run(
            ["bash", "tools/paxeer-x/verify-task.sh", "0.5"],
            cwd=ROOT,
            stdin=subprocess.DEVNULL,
            capture_output=True,
            text=True,
            timeout=60,
        )
        self.assertEqual(result.returncode, 2, result.stdout + result.stderr)
        self.assertIn("unknown task selector 0.5", result.stderr)

        registry = (ROOT / "tools/paxeer-x/gates/registry.kvx").read_text(
            encoding="utf-8"
        )
        self.assertNotIn("[task.0.5]", registry)
        self.assertNotIn("[task.2.2]", registry)

        gate = "tools/paxeer-x/gates/104.17.4.sh"
        result = subprocess.run(
            ["bash", "-n", gate],
            cwd=ROOT,
            stdin=subprocess.DEVNULL,
            capture_output=True,
            text=True,
            timeout=60,
        )
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        source = (ROOT / gate).read_text(encoding="utf-8")
        for name in ("hosted-boundary.sh", "run-load.sh"):
            with self.subTest(source=name):
                self.assertNotIn(name, source)


if __name__ == "__main__":
    unittest.main(verbosity=2)
