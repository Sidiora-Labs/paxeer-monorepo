from __future__ import annotations

import shlex
import subprocess
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
GATE = ROOT / "scripts" / "gates" / "wave-6.sh"


class Wave6GateTest(unittest.TestCase):
    def test_list_names_runnable_commands(self) -> None:
        expected = [
            "make workspace-build",
            "make workspace-test",
            "make workspace-lint",
            "make core-test-all",
            "python3 tools/paxeer-x/tests/retired_callers_test.py --deleted",
            "make programs-test",
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
        self.assertEqual(completed.stderr, "")

        for command in completed.stdout.splitlines():
            arguments = shlex.split(command)
            if arguments[0] == "make":
                with self.subTest(command=command):
                    dry_run = subprocess.run(
                        ["make", "-n", *arguments[1:]],
                        cwd=ROOT,
                        capture_output=True,
                        text=True,
                        check=False,
                    )
                    self.assertEqual(dry_run.returncode, 0, dry_run.stderr)


if __name__ == "__main__":
    unittest.main()
