import os
import pathlib
import re
import subprocess
import unittest


ROOT = pathlib.Path(__file__).resolve().parents[3]


class EmulatorBuildTargetTest(unittest.TestCase):
    def test_emulator_builds_the_kernel_archive_without_the_daemon(self):
        source = (ROOT / "tools/platform/emulator/build.rs").read_text()
        make = re.search(
            r'let mut make = Command::new\("make"\);(.*?)run\(make,',
            source,
            re.DOTALL,
        )
        self.assertIsNotNone(make, "emulator must invoke make for the kernel archive")
        arguments = re.findall(r'\.arg\("([^"]+)"\)', make.group(1))
        self.assertIn("build/liblayerx.a", arguments)
        self.assertNotIn("build", arguments)

        build = subprocess.run(
            ["make", "-C", ".", "build/liblayerx.a"],
            cwd=ROOT,
            env={**os.environ, "LC_ALL": "C"},
            capture_output=True,
            text=True,
            check=False,
        )
        self.assertEqual(build.returncode, 0, build.stdout + build.stderr)
        self.assertTrue((ROOT / "build/liblayerx.a").is_file())


if __name__ == "__main__":
    unittest.main()
