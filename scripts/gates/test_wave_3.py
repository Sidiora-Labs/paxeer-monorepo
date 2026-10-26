from __future__ import annotations

import shlex
import subprocess
import tempfile
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
GATE = ROOT / "scripts" / "gates" / "wave-3.sh"


class Wave3GateTest(unittest.TestCase):
    def test_list_names_runnable_commands(self) -> None:
        expected = [
            "make workspace-build",
            "make workspace-test",
            "make agent-test",
            "make interop-test",
            "make programs-test",
        ]
        with tempfile.TemporaryDirectory() as temporary:
            listed = subprocess.run(
                [str(GATE), "--list"],
                cwd=temporary,
                capture_output=True,
                text=True,
                check=False,
            )
            self.assertEqual(listed.returncode, 0, listed.stderr)
            self.assertEqual(listed.stderr, "")
            self.assertEqual(listed.stdout, "\n".join(expected) + "\n")
            self.assertEqual(list(Path(temporary).iterdir()), [])

        for command in listed.stdout.splitlines():
            executable, target = shlex.split(command)
            with self.subTest(target=target):
                completed = subprocess.run(
                    [executable, "-n", target],
                    cwd=ROOT,
                    capture_output=True,
                    text=True,
                    check=False,
                )
                self.assertEqual(
                    completed.returncode, 0, completed.stdout + completed.stderr
                )


if __name__ == "__main__":
    unittest.main()
