from __future__ import annotations

import shlex
import subprocess
import tempfile
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
GATE = ROOT / "scripts" / "gates" / "wave-0.sh"


class Wave0GateTest(unittest.TestCase):
    def test_list_names_runnable_commands(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            completed = subprocess.run(
                [str(GATE), "--list"],
                cwd=temporary,
                capture_output=True,
                text=True,
                check=False,
            )
            self.assertEqual(completed.returncode, 0, completed.stderr)
            self.assertEqual(completed.stderr, "")
            self.assertEqual(list(Path(temporary).iterdir()), [])
        commands = completed.stdout.splitlines()
        self.assertEqual(
            commands,
            ["make -n build test", "make workspace-build", "make workspace-lint"],
        )
        for command in commands:
            arguments = shlex.split(command)
            self.assertEqual(arguments[0], "make")
            for target in arguments[1:]:
                if target == "-n":
                    continue
                with self.subTest(target=target):
                    dry_run = subprocess.run(
                        ["make", "-n", target],
                        cwd=ROOT,
                        capture_output=True,
                        text=True,
                        check=False,
                    )
                    self.assertEqual(
                        dry_run.returncode,
                        0,
                        f"make -n {target}\n{dry_run.stdout}{dry_run.stderr}",
                    )


if __name__ == "__main__":
    unittest.main()
