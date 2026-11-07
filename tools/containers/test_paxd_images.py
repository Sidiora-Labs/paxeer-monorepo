from pathlib import Path
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


if __name__ == "__main__":
    unittest.main()
