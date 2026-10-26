from __future__ import annotations

import shlex
import subprocess
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
GATE = ROOT / "scripts" / "gates" / "wave-5.sh"


class Wave5GateTest(unittest.TestCase):
    def test_list_names_runnable_commands(self) -> None:
        expected = [
            "make workspace-build",
            "make workspace-test",
            "make human-test",
            "tools/containers/check.sh",
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
            arguments = shlex.split(command)
            if arguments[0] == "make":
                with self.subTest(command=command):
                    completed = subprocess.run(
                        ["make", "-n", *arguments[1:]],
                        cwd=ROOT,
                        capture_output=True,
                        text=True,
                        check=False,
                    )
                    self.assertEqual(
                        completed.returncode,
                        0,
                        completed.stdout + completed.stderr,
                    )


if __name__ == "__main__":
    unittest.main()
