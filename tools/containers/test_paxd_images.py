from pathlib import Path
import re
import shlex
import subprocess
import unittest


class PaxdImagesTest(unittest.TestCase):
    def test_layerx_and_paxeer_images_pass_build_check(self):
        repo_root = Path(__file__).resolve().parents[2]
        result = subprocess.run(
            ["bash", "tools/containers/check.sh", "layerx", "paxeer"],
            cwd=repo_root,
            capture_output=True,
            text=True,
            check=False,
        )
        self.assertEqual(
            result.returncode,
            0,
            msg=f"Container build check failed:\n{result.stdout}\n{result.stderr}",
        )
        for recipe in (
            "docker/layerx/Dockerfile",
            "docker/paxeer/Dockerfile",
            "docker/paxeer/Dockerfile.paxd",
            "docker/paxeer/Dockerfile.paxd-node",
        ):
            self.assertIn(f"containers-check: {recipe} check ok", result.stdout)

    def test_root_recipe_reference_scan_only_matches_recipe_paths(self):
        repo_root = Path(__file__).resolve().parents[2]
        script = (repo_root / "tools/containers/check.sh").read_text(encoding="utf-8")
        assignment = re.search(r"(?m)^ROOT_RECIPE_REFERENCE=(.*)$", script)
        self.assertIsNotNone(assignment)
        pattern = shlex.split(assignment.group(1))[0]
        producer = repo_root / "services/platform/hosted/tests/beta_runtime.go"
        literals = subprocess.run(
            ["grep", "-n", "-E", pattern, str(producer)],
            capture_output=True,
            text=True,
            check=False,
        )
        self.assertEqual(literals.returncode, 1, literals.stdout + literals.stderr)
        for prefix in (
            "file:",
            "-f",
            "--file",
            "--file=",
            "dockerfile =",
            "dockerfile:",
            "-",
        ):
            for quote in ("", "'", '"'):
                for recipe in ("Dockerfile", "Dockerfile.dockerignore"):
                    with self.subTest(prefix=prefix, quote=quote, recipe=recipe):
                        for path, expected in (
                            (recipe, 0),
                            (f"docker/layerx/{recipe}", 1),
                            (f"{recipe}.historical", 1),
                        ):
                            result = subprocess.run(
                                ["grep", "-E", pattern],
                                input=f"{prefix} {quote}{path}{quote}\n",
                                capture_output=True,
                                text=True,
                                check=False,
                            )
                            self.assertEqual(result.returncode, expected, result.stderr)


if __name__ == "__main__":
    unittest.main()
