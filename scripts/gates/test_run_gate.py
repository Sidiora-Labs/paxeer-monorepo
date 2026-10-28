from __future__ import annotations

import subprocess
import sys
import tempfile
import unittest
from contextlib import redirect_stderr, redirect_stdout
from io import StringIO
from pathlib import Path

from scripts.gates.run_gate import main


ROOT = Path(__file__).resolve().parents[2]
RUNNER = ROOT / "scripts" / "gates" / "run_gate.py"


class RunGateTest(unittest.TestCase):
    def test_programme_wave_identifier_runs_commands(self) -> None:
        revision = subprocess.run(
            ["git", "rev-parse", "HEAD"],
            cwd=ROOT,
            check=True,
            capture_output=True,
            text=True,
        ).stdout.strip()
        with tempfile.TemporaryDirectory() as temporary:
            for wave in ("programme", "0", "2"):
                with self.subTest(wave=wave):
                    log_dir = Path(temporary) / wave
                    arguments = ["--wave", wave, "--log-dir", str(log_dir)]
                    output = StringIO()
                    with redirect_stdout(output):
                        self.assertEqual(main([*arguments, "--", "true"]), 0)
                    log_path = log_dir / f"wave-{wave}-1.log"
                    self.assertTrue(log_path.is_file())
                    self.assertEqual(
                        output.getvalue().splitlines(),
                        [f"revision={revision} command=true exit=0 log={log_path}"],
                    )

            log_dir = Path(temporary) / "multiple"
            with redirect_stdout(StringIO()):
                self.assertEqual(
                    main(
                        [
                            "--wave",
                            "programme",
                            "--log-dir",
                            str(log_dir),
                            "--",
                            "true",
                            "--",
                            sys.executable,
                            "-c",
                            "print('second command')",
                        ]
                    ),
                    0,
                )
            self.assertTrue((log_dir / "wave-programme-1.log").is_file())
            self.assertEqual(
                (log_dir / "wave-programme-2.log").read_text(), "second command\n"
            )

            for wave in ("other", "-1", "1.5", "Programme", ""):
                with self.subTest(invalid_wave=wave):
                    log_dir = Path(temporary) / "invalid"
                    error = StringIO()
                    with redirect_stderr(error), self.assertRaises(SystemExit) as raised:
                        main(
                            ["--wave", wave, "--log-dir", str(log_dir), "--", "true"]
                        )
                    self.assertEqual(raised.exception.code, 2)
                    self.assertIn("argument --wave:", error.getvalue())
                    self.assertFalse(log_dir.exists())

    def test_records_evidence_line_for_passing_command(self) -> None:
        revision = subprocess.run(
            ["git", "rev-parse", "HEAD"],
            cwd=ROOT,
            check=True,
            capture_output=True,
            text=True,
        ).stdout.strip()
        with tempfile.TemporaryDirectory() as temporary:
            log_dir = Path(temporary) / "logs"
            completed = subprocess.run(
                [
                    sys.executable,
                    str(RUNNER),
                    "--wave",
                    "0",
                    "--log-dir",
                    str(log_dir),
                    "--",
                    "make",
                    "-n",
                    "build",
                ],
                cwd=temporary,
                capture_output=True,
                text=True,
                check=False,
            )
            self.assertEqual(completed.returncode, 0, completed.stderr)
            log_path = log_dir / "wave-0-1.log"
            self.assertEqual(
                completed.stdout.splitlines(),
                [f"revision={revision} command=make -n build exit=0 log={log_path}"],
            )
            self.assertTrue(log_path.is_file())
            self.assertTrue(log_path.read_text().strip())
            self.assertEqual(completed.stderr, "")

    def test_nonzero_exit_is_recorded_and_returned(self) -> None:
        revision = subprocess.run(
            ["git", "rev-parse", "HEAD"],
            cwd=ROOT,
            check=True,
            capture_output=True,
            text=True,
        ).stdout.strip()
        target = "__run_gate_unknown_target_16001__"
        with tempfile.TemporaryDirectory() as temporary:
            log_dir = Path(temporary) / "logs"
            completed = subprocess.run(
                [
                    sys.executable,
                    str(RUNNER),
                    "--wave",
                    "2",
                    "--log-dir",
                    str(log_dir),
                    "--",
                    "make",
                    target,
                    "--",
                    "make",
                    "-n",
                    "build",
                ],
                cwd=temporary,
                capture_output=True,
                text=True,
                check=False,
            )
            self.assertEqual(completed.returncode, 2, completed.stderr)
            log_path = log_dir / "wave-2-1.log"
            self.assertEqual(
                completed.stdout.splitlines(),
                [f"revision={revision} command=make {target} exit=2 log={log_path}"],
            )
            self.assertTrue(log_path.is_file())
            self.assertIn(target, log_path.read_text())
            self.assertFalse((log_dir / "wave-2-2.log").exists())
            self.assertEqual(completed.stderr, "")


if __name__ == "__main__":
    unittest.main()
