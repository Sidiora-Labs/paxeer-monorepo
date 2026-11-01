from __future__ import annotations

import shlex
import subprocess
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
GATE = ROOT / "scripts" / "gates" / "wave-4.sh"


class Wave4GateTest(unittest.TestCase):
    def test_list_names_runnable_commands(self) -> None:
        expected = [
            "make workspace-build",
            "make workspace-test",
            "make core-qualification-environment",
            "make beta-driver-test",
        ]
        completed = subprocess.run(
            [str(GATE), "--list"],
            cwd=ROOT,
            capture_output=True,
            text=True,
            check=False,
        )
        self.assertEqual(completed.returncode, 0, completed.stderr)
        self.assertEqual(completed.stdout, "\n".join(expected) + "\n")
        for command in completed.stdout.splitlines():
            with self.subTest(command=command):
                executable, target = shlex.split(command)
                dry_run = subprocess.run(
                    [executable, "-n", target],
                    cwd=ROOT,
                    capture_output=True,
                    text=True,
                    check=False,
                )
                self.assertEqual(
                    dry_run.returncode, 0, dry_run.stdout + dry_run.stderr
                )


if __name__ == "__main__":
    unittest.main()
