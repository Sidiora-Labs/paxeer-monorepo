from pathlib import Path
import subprocess
import unittest


class RetiredImagesTest(unittest.TestCase):
    def test_container_layout_passes_without_retired_images(self):
        root = Path(__file__).resolve().parents[2]
        for service in ("relay-archive", "intent-ingester"):
            with self.subTest(service=service):
                self.assertFalse((root / "docker" / service).exists())

        tracked = subprocess.run(
            ["git", "ls-files", "docker/relay-archive", "docker/intent-ingester"],
            cwd=root,
            capture_output=True,
            text=True,
        )
        self.assertEqual(tracked.returncode, 0, tracked.stdout + tracked.stderr)
        self.assertEqual(tracked.stdout, "")

        result = subprocess.run(
            ["bash", "tools/containers/check.sh", "kernel"],
            cwd=root,
            capture_output=True,
            text=True,
        )
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
