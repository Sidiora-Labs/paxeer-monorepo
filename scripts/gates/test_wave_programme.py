from __future__ import annotations

import shlex
import subprocess
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
GATE = ROOT / "scripts" / "gates" / "wave-programme.sh"


class WaveProgrammeGateTest(unittest.TestCase):
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
            "make workspace-ci\n"
            "make beta-qualify\n"
            "python3 tools/paxeer-x/tests/retired_callers_test.py --deleted\n"
            "scripts/ci/beta-ledger-check.sh --revisions\n",
        )
        for command in completed.stdout.splitlines():
            arguments = shlex.split(command)
            if arguments[0] != "make":
                continue
            for target in arguments[1:]:
                with self.subTest(target=target):
                    dry_run = subprocess.run(
                        ["make", "-n", target],
                        cwd=ROOT,
                        capture_output=True,
                        text=True,
                        check=False,
                    )
                    self.assertEqual(dry_run.returncode, 0, dry_run.stderr)


if __name__ == "__main__":
    unittest.main()
