from pathlib import Path
import subprocess
import unittest


class RetiredImagesTest(unittest.TestCase):
    def test_publication_table_resolves_only_existing_recipes(self):
        root = Path(__file__).resolve().parents[2]
        result = subprocess.run(
            [
                "bash",
                "-euc",
                """
source services/platform/hosted/tests/beta-images.sh
for name in "${IMAGE_NAMES[@]}"; do
    printf '%s ' "$name"
    image_source "$name"
    printf '\\n'
done
""",
            ],
            cwd=root,
            capture_output=True,
            text=True,
        )
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        rows = result.stdout.splitlines()
        self.assertTrue(rows, "publication table is empty")
        names = []
        for row in rows:
            name, _canonical, dockerfile = row.split()
            names.append(name)
            with self.subTest(image=name):
                resolved = (root / dockerfile).resolve()
                self.assertTrue(resolved.is_file(), f"missing Dockerfile: {resolved}")
        self.assertNotIn("relay-archive", names)
        self.assertNotIn("intent-ingester", names)

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
