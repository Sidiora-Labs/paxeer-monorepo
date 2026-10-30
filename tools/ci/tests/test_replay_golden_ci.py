from pathlib import Path
import subprocess
import unittest

import yaml


class ReplayGoldenCiTest(unittest.TestCase):
    def test_core_gates_and_routing_pass_without_replay_golden(self):
        root = Path(__file__).resolve().parents[3]
        for script in (
            "scripts/workspace/check-core-gates.sh",
            "scripts/ci/runner-routing-check.sh",
        ):
            with self.subTest(script=script):
                result = subprocess.run(
                    ["sh", script],
                    cwd=root,
                    capture_output=True,
                    text=True,
                    timeout=60,
                )
                self.assertEqual(
                    result.returncode, 0, result.stdout + result.stderr
                )

        self.assertFalse((root / "scripts/ci/replay-matrix.sh").exists())
        workflow = yaml.safe_load(
            (root / ".github/workflows/platform.yml").read_text(encoding="utf-8")
        )
        self.assertNotIn("replay-matrix", workflow["jobs"])
        self.assertNotIn(
            "replay-matrix", workflow["jobs"]["release-pipeline"]["needs"]
        )
        inventory = (root / "tools/workspace/core-gates.txt").read_text(
            encoding="utf-8"
        )
        targets = {row.split("|", 1)[0] for row in inventory.splitlines()}
        self.assertNotIn("test-replay-golden-local", targets)
        self.assertNotIn("test-replay-golden", targets)


if __name__ == "__main__":
    unittest.main()
