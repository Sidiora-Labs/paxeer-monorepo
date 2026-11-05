from pathlib import Path
import subprocess
import unittest


class HumanServiceImageTest(unittest.TestCase):
    def test_movement_role_refused_and_image_checks(self):
        root = Path(__file__).resolve().parents[2]
        movement = subprocess.run(
            ["sh", "docker/human-service/entrypoint.sh", "movement"],
            cwd=root,
            capture_output=True,
            text=True,
        )
        self.assertEqual(movement.returncode, 64, movement.stdout + movement.stderr)
        self.assertIn("unknown Human role", movement.stderr)
        self.assertNotIn(
            "layerx-human-movement-provider",
            (root / "docker/human-service/entrypoint.sh").read_text(encoding="utf-8"),
        )
        self.assertNotIn(
            "layerx-human-movement-provider",
            (root / "docker/human-service/Dockerfile").read_text(encoding="utf-8"),
        )

        image_checks = subprocess.run(
            ["bash", "tools/containers/check.sh", "human-service"],
            cwd=root,
            capture_output=True,
            text=True,
        )
        self.assertEqual(
            image_checks.returncode, 0, image_checks.stdout + image_checks.stderr
        )
