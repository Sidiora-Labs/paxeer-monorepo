from __future__ import annotations

import shlex
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
                [
                    f"revision={revision} command=make {target} exit=2 log={log_path}",
                    f"=== last 200 lines of {log_path} ===",
                    *log_path.read_text().splitlines()[-200:],
                ],
            )
            self.assertTrue(log_path.is_file())
            self.assertIn(target, log_path.read_text())
            self.assertFalse((log_dir / "wave-2-2.log").exists())
            self.assertEqual(completed.stderr, "")

    def test_echoes_failing_step_log_tail(self) -> None:
        revision = subprocess.run(
            ["git", "rev-parse", "HEAD"],
            cwd=ROOT,
            check=True,
            capture_output=True,
            text=True,
        ).stdout.strip()
        known_line = "wave gate failure from stderr"
        with tempfile.TemporaryDirectory() as temporary:
            for line_count in (0, 205):
                with self.subTest(line_count=line_count):
                    log_dir = Path(temporary) / str(line_count)
                    log_path = log_dir / "wave-0-1.log"
                    command = [
                        "python3",
                        "-c",
                        "import sys; "
                        f"[print(f'line {{i}}', file=sys.stderr) for i in range({line_count})]; "
                        f"print({known_line!r}, file=sys.stderr); sys.exit(3)",
                    ]
                    output = StringIO()
                    with redirect_stdout(output):
                        result = main(
                            [
                                "--wave",
                                "0",
                                "--log-dir",
                                str(log_dir),
                                "--",
                                *command,
                                "--",
                                "true",
                            ]
                        )
                    self.assertEqual(result, 3)
                    log_lines = [f"line {i}" for i in range(line_count)] + [known_line]
                    self.assertEqual(
                        output.getvalue().splitlines(),
                        [
                            f"revision={revision} command={shlex.join(command)} "
                            f"exit=3 log={log_path}",
                            f"=== last 200 lines of {log_path} ===",
                            *log_lines[-200:],
                        ],
                    )
                    self.assertEqual(log_path.read_text().splitlines(), log_lines)
                    self.assertFalse((log_dir / "wave-0-2.log").exists())

    def test_wave_scripts_default_log_dir_is_logs_waves(self) -> None:
        scripts = sorted((ROOT / "scripts" / "gates").glob("wave-*.sh"))
        self.assertEqual(
            [script.stem for script in scripts],
            [f"wave-{wave}" for wave in (*range(7), "programme")],
        )
        for script in scripts:
            with self.subTest(script=script.name):
                source = script.read_text()
                wave = script.stem.removeprefix("wave-")
                self.assertRegex(
                    source,
                    rf'--log-dir "\$\{{GATE_LOG_DIR:-\$(?:root|ROOT)/logs/waves/{wave}\}}"',
                )
                self.assertNotIn("mktemp", source)


if __name__ == "__main__":
    unittest.main()
