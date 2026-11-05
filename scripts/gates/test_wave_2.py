from __future__ import annotations

import shlex
import subprocess
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
GATE = ROOT / "scripts" / "gates" / "wave-2.sh"


class Wave2GateTest(unittest.TestCase):
    def test_list_names_runnable_commands(self) -> None:
        completed = subprocess.run(
            [str(GATE), "--list"],
            cwd=ROOT,
            capture_output=True,
            text=True,
            check=False,
        )
        self.assertEqual(completed.returncode, 0, completed.stderr)
        self.assertEqual(completed.stderr, "")
        self.assertEqual(
            completed.stdout,
            "make workspace-build\n"
            "make workspace-test\n"
            "make core-test-all\n"
            "make agent-test\n",
        )
        for command in completed.stdout.splitlines():
            with self.subTest(command=command):
                arguments = shlex.split(command)
                dry_run = subprocess.run(
                    [arguments[0], "-n", "-f", str(ROOT / "Makefile"), *arguments[1:]],
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
