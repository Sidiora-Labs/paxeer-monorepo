from __future__ import annotations

import shlex
import subprocess
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
GATE = ROOT / "scripts" / "gates" / "wave-1.sh"


class Wave1GateTest(unittest.TestCase):
    def test_list_names_runnable_commands(self) -> None:
        expected = [
            "make workspace-build",
            "make workspace-test",
            "make core-test-all",
            "make test-sanitizer-suite",
        ]
        listed = subprocess.run(
            [str(GATE), "--list"],
            cwd=ROOT,
            capture_output=True,
            text=True,
            check=False,
        )
        self.assertEqual(listed.returncode, 0, listed.stderr)
        self.assertEqual(listed.stdout, "\n".join(expected) + "\n")
        self.assertEqual(listed.stderr, "")
        for command in listed.stdout.splitlines():
            with self.subTest(command=command):
                arguments = shlex.split(command)
                dry_run = subprocess.run(
                    [arguments[0], "-n", *arguments[1:]],
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
