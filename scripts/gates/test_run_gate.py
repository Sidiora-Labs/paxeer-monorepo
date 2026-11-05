from __future__ import annotations

import errno
import os
import re
import shlex
import shutil
import stat
import subprocess
import sys
import tempfile
import time
import unittest
from contextlib import redirect_stderr
from io import StringIO
from pathlib import Path

from scripts.gates.run_gate import main


ROOT = Path(__file__).resolve().parents[2]
RUNNER = ROOT / "scripts" / "gates" / "run_gate.py"


class RunGateTest(unittest.TestCase):
    def setUp(self) -> None:
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.directory = Path(self.temporary.name)
        self.root = self.directory / "repository"
        self.runner = self.root / "scripts" / "gates" / "run_gate.py"
        self.runner.parent.mkdir(parents=True)
        shutil.copyfile(RUNNER, self.runner)
        self.source = self.root / "source.txt"
        self.source.write_text("committed source\n")
        (self.root / ".gitignore").write_text(
            "__pycache__/\n.pytest_cache/\n/build/\n*.log\nignored-source.py\n"
        )
        self.git("init", "-q")
        self.git("config", "core.filemode", "true")
        self.git("add", ".")
        self.git("commit", "-qm", "Committed gate and source fixture")
        self.revision = self.git("rev-parse", "HEAD").stdout.strip()
        self.log_dir = self.directory / "logs"

    def git(self, *arguments: str) -> subprocess.CompletedProcess[str]:
        return subprocess.run(
            [
                "git", "-c", "user.name=Gate test", "-c",
                "user.email=gate@example.invalid", "-c", "core.hooksPath=/dev/null",
                *arguments,
            ],
            cwd=self.root,
            check=True,
            capture_output=True,
            text=True,
            timeout=15,
        )

    def python(self, source: str) -> list[str]:
        return [sys.executable, "-c", source]

    def arguments(
        self, *commands: list[str], wave: str = "0", log_dir: Path | None = None
    ) -> list[str]:
        arguments = [
            sys.executable, str(self.runner), "--wave", wave,
            "--log-dir", str(self.log_dir if log_dir is None else log_dir),
        ]
        for command in commands:
            arguments.extend(["--", *command])
        return arguments

    def run_commands(
        self, *commands: list[str], wave: str = "0", log_dir: Path | None = None
    ) -> subprocess.CompletedProcess[str]:
        return subprocess.run(
            self.arguments(*commands, wave=wave, log_dir=log_dir),
            cwd=self.directory,
            capture_output=True,
            text=True,
            check=False,
            timeout=20,
        )

    def assert_record(
        self, completed: subprocess.CompletedProcess[str], command: list[str],
        code: int, *, wave: str = "0", status: str = "", log_dir: Path | None = None,
    ) -> Path:
        self.assertEqual(completed.returncode, code, completed.stderr)
        self.assertEqual(completed.stderr, "")
        lines = completed.stdout.splitlines()
        self.assertTrue(lines, completed)
        match = re.search(r" log=(\S+)(?: status=.*)?$", lines[0])
        self.assertIsNotNone(match, completed.stdout)
        log_path = Path(match.group(1))
        expected_root = self.log_dir if log_dir is None else log_dir
        self.assertEqual(log_path.parent.parent, expected_root)
        self.assertRegex(log_path.parent.name, r"^attempt-[0-9a-f]{32}$")
        self.assertEqual(log_path.name, f"wave-{wave}-1.log")
        self.assertEqual(
            lines[0],
            f"revision={self.revision} command={shlex.join(command)} "
            f"exit={code} log={log_path}" + (f" {status}" if status else ""),
        )
        self.assertTrue(log_path.is_file())
        self.assertEqual(stat.S_IMODE(log_path.stat().st_mode), 0o400)
        self.assertEqual(stat.S_IMODE(log_path.parent.stat().st_mode), 0o500)
        expected = [lines[0]]
        if code:
            expected += [
                f"=== last 200 lines of {log_path} ===",
                *log_path.read_text().splitlines()[-200:],
            ]
        self.assertEqual(lines, expected)
        return log_path

    def later_command(self) -> list[str]:
        return self.python("from pathlib import Path; Path('later.txt').write_text('ran')")

    def assert_no_later_command(self, log_path: Path) -> None:
        self.assertFalse((self.root / "later.txt").exists())
        self.assertFalse((log_path.parent / log_path.name.replace("-1.log", "-2.log")).exists())

    def test_programme_wave_identifier_runs_commands(self) -> None:
        command = self.python("pass")
        for wave in ("programme", "0", "2"):
            with self.subTest(wave=wave):
                completed = self.run_commands(command, wave=wave)
                self.assert_record(completed, command, 0, wave=wave)

        second = self.python("print('second command')")
        completed = self.run_commands(command, second, wave="programme")
        self.assertEqual(completed.returncode, 0, completed.stderr)
        lines = completed.stdout.splitlines()
        self.assertEqual(len(lines), 2)
        paths = [Path(re.search(r" log=(\S+)$", line).group(1)) for line in lines]
        self.assertEqual(paths[0].parent, paths[1].parent)
        self.assertEqual(paths[0].read_text(), "")
        self.assertEqual(paths[1].name, "wave-programme-2.log")
        self.assertEqual(paths[1].read_text(), "second command\n")
        self.assertEqual(
            lines,
            [
                f"revision={self.revision} command={shlex.join(cmd)} exit=0 log={path}"
                for cmd, path in zip((command, second), paths)
            ],
        )

        for wave in ("other", "-1", "1.5", "Programme", ""):
            with self.subTest(invalid_wave=wave):
                log_dir = self.directory / "invalid"
                error = StringIO()
                with redirect_stderr(error), self.assertRaises(SystemExit) as raised:
                    main(["--wave", wave, "--log-dir", str(log_dir), "--", *command])
                self.assertEqual(raised.exception.code, 2)
                self.assertIn("argument --wave:", error.getvalue())
                self.assertFalse(log_dir.exists())

    def test_records_evidence_line_for_passing_command(self) -> None:
        command = self.python(
            "from pathlib import Path; print(Path('source.txt').read_text(), end='')"
        )
        completed = self.run_commands(command)
        log_path = self.assert_record(completed, command, 0)
        self.assertEqual(log_path.read_text(), "committed source\n")

    def test_nonzero_exit_is_recorded_and_returned(self) -> None:
        target = "__run_gate_unknown_target_16001__"
        command = self.python(f"import sys; print({target!r}, file=sys.stderr); sys.exit(2)")
        completed = self.run_commands(command, self.later_command(), wave="2")
        log_path = self.assert_record(completed, command, 2, wave="2")
        self.assertEqual(log_path.read_text(), target + "\n")
        self.assert_no_later_command(log_path)

    def test_echoes_failing_step_log_tail(self) -> None:
        known_line = "wave gate failure from stderr"
        for line_count in (0, 205):
            with self.subTest(line_count=line_count):
                command = self.python(
                    "import sys; "
                    f"[print(f'line {{i}}', file=sys.stderr) for i in range({line_count})]; "
                    f"print({known_line!r}, file=sys.stderr); sys.exit(3)"
                )
                completed = self.run_commands(command, self.later_command())
                log_path = self.assert_record(completed, command, 3)
                self.assertEqual(
                    log_path.read_text().splitlines(),
                    [f"line {i}" for i in range(line_count)] + [known_line],
                )
                self.assert_no_later_command(log_path)

    def test_repeated_log_paths_preserve_previous_attempts(self) -> None:
        self.log_dir.mkdir()
        legacy = self.log_dir / "wave-0-1.log"
        legacy.write_text("previous retained evidence\n")
        paths = []
        for text in ("first attempt", "second attempt"):
            if paths:
                self.git("commit", "--allow-empty", "-qm", "Next qualification revision")
                self.revision = self.git("rev-parse", "HEAD").stdout.strip()
            command = self.python(f"print({text!r})")
            completed = self.run_commands(command)
            paths.append(self.assert_record(completed, command, 0))
        self.assertNotEqual(paths[0].parent, paths[1].parent)
        self.assertEqual(paths[0].read_text(), "first attempt\n")
        self.assertEqual(paths[1].read_text(), "second attempt\n")
        self.assertEqual(legacy.read_text(), "previous retained evidence\n")

    def test_concurrent_log_paths_preserve_both_attempts(self) -> None:
        self.log_dir.mkdir()
        release = self.log_dir / "release"
        processes = []
        commands = []
        try:
            for index in range(2):
                ready = self.log_dir / f"ready-{index}"
                wait = f"while not Path({str(release)!r}).exists(): time.sleep(0.01)"
                command = self.python(
                    "from pathlib import Path; import time; "
                    f"Path({str(ready)!r}).write_text('ready'); "
                    f"exec({wait!r}); print('concurrent-{index}')"
                )
                commands.append(command)
                processes.append(subprocess.Popen(
                    self.arguments(command), cwd=self.directory,
                    stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True,
                ))
            deadline = time.monotonic() + 10
            while not all((self.log_dir / f"ready-{i}").exists() for i in range(2)):
                if time.monotonic() >= deadline:
                    self.fail("Both real gate children must overlap before release")
                time.sleep(0.01)
            release.write_text("release")
            paths = []
            for process, command in zip(processes, commands):
                stdout, stderr = process.communicate(timeout=15)
                paths.append(self.assert_record(
                    subprocess.CompletedProcess(process.args, process.returncode, stdout, stderr),
                    command, 0,
                ))
            self.assertNotEqual(paths[0].parent, paths[1].parent)
            for index, path in enumerate(paths):
                self.assertEqual(path.read_text(), f"concurrent-{index}\n")
        finally:
            release.touch()
            for process in processes:
                if process.poll() is None:
                    process.kill()
                process.communicate(timeout=15)

    def test_existing_command_log_collision_is_refused(self) -> None:
        command = self.python(
            "from pathlib import Path; "
            f"attempt = next(Path({str(self.log_dir)!r}).glob('attempt-*')); "
            "(attempt / 'wave-0-2.log').write_text('retain collision evidence'); "
            "print('first command')"
        )
        second = self.later_command()
        completed = self.run_commands(command, second, self.python("print('third')"))
        self.assertEqual(completed.returncode, 73, completed.stderr)
        self.assertEqual(completed.stderr, "")
        attempt, = self.log_dir.glob("attempt-*")
        first = attempt / "wave-0-1.log"
        collision = attempt / "wave-0-2.log"
        self.assertEqual(completed.stdout.splitlines(), [
            f"revision={self.revision} command={shlex.join(command)} exit=0 log={first}",
            f"revision={self.revision} command={shlex.join(second)} "
            f"exit=73 log={collision} status=evidence-refusal",
        ])
        self.assertEqual(first.read_text(), "first command\n")
        self.assertEqual(collision.read_text(), "retain collision evidence")
        self.assertFalse((attempt / "wave-0-3.log").exists())
        self.assertFalse((self.root / "later.txt").exists())

    def test_dirty_starting_source_is_refused(self) -> None:
        command = self.python("print('must not execute')")
        original = self.source.read_text()
        for kind in ("working", "index", "untracked", "ignored", "deleted", "mode"):
            with self.subTest(kind=kind):
                extra = None
                if kind in {"working", "index"}:
                    self.source.write_text("dirty source\n")
                    if kind == "index":
                        self.git("add", "source.txt")
                elif kind in {"untracked", "ignored"}:
                    extra = self.root / ("new-source.py" if kind == "untracked" else "ignored-source.py")
                    extra.write_text("print('uncommitted source')\n")
                elif kind == "deleted":
                    self.source.unlink()
                else:
                    self.source.chmod(0o755)
                completed = self.run_commands(command, self.later_command())
                log_path = self.assert_record(
                    completed, command, 125,
                    status="status=source-identity-refusal phase=before",
                )
                self.assertIn("source tree differs from HEAD:", log_path.read_text())
                self.assertNotIn("must not execute", log_path.read_text())
                self.assert_no_later_command(log_path)
                if extra is not None:
                    extra.unlink()
                self.source.write_text(original)
                self.source.chmod(0o644)
                self.git("add", "source.txt")

    def test_source_changes_during_command_are_refused(self) -> None:
        command = self.python(
            "from pathlib import Path; Path('source.txt').write_text('changed'); "
            "print('child actually ran')"
        )
        completed = self.run_commands(command, self.later_command())
        log_path = self.assert_record(
            completed, command, 125,
            status="status=source-identity-refusal phase=after",
        )
        self.assertEqual(log_path.read_text(),
            "child actually ran\n\n"
            f"source-identity-refusal phase=after expected={self.revision} command_exit=0\n"
            "source tree differs from HEAD:\n M 'source.txt'\n"
        )
        self.assert_no_later_command(log_path)

    def test_failed_child_cannot_hide_source_change(self) -> None:
        command = self.python(
            "from pathlib import Path; import sys; "
            "Path('source.txt').write_text('changed'); print('failed child'); sys.exit(3)"
        )
        completed = self.run_commands(command, self.later_command())
        log_path = self.assert_record(
            completed, command, 125,
            status="status=source-identity-refusal phase=after",
        )
        self.assertIn("failed child\n", log_path.read_text())
        self.assertIn("command_exit=3", log_path.read_text())
        self.assert_no_later_command(log_path)

    def test_changed_head_is_refused(self) -> None:
        command = [
            "git", "-c", "user.name=Gate test", "-c",
            "user.email=gate@example.invalid", "-c", "core.hooksPath=/dev/null",
            "commit", "--allow-empty", "-qm", "Different revision",
        ]
        completed = self.run_commands(command, self.later_command())
        log_path = self.assert_record(
            completed, command, 125,
            status="status=source-identity-refusal phase=after",
        )
        observed = self.git("rev-parse", "HEAD").stdout.strip()
        self.assertNotEqual(observed, self.revision)
        self.assertIn(
            f"HEAD changed: expected={self.revision} observed={observed}",
            log_path.read_text(),
        )
        self.assert_no_later_command(log_path)

    def test_untracked_source_created_during_command_is_refused(self) -> None:
        command = self.python(
            "from pathlib import Path; Path('new-source.py').write_text('print(1)')"
        )
        completed = self.run_commands(command, self.later_command())
        log_path = self.assert_record(
            completed, command, 125,
            status="status=source-identity-refusal phase=after",
        )
        self.assertIn("?? 'new-source.py'", log_path.read_text())
        self.assert_no_later_command(log_path)

    def test_declared_untracked_build_outputs_are_permitted(self) -> None:
        self.log_dir = self.root / "logs" / "waves" / "programme"
        command = self.python(
            "from pathlib import Path; "
            "[Path(p).parent.mkdir(parents=True, exist_ok=True) "
            "for p in ('build/object.o', '__pycache__/source.pyc', '.pytest_cache/result')]; "
            "[Path(p).write_text('generated') "
            "for p in ('build/object.o', '__pycache__/source.pyc', '.pytest_cache/result')]; "
            "print('build output')"
        )
        completed = self.run_commands(command)
        log_path = self.assert_record(completed, command, 0)
        self.assertEqual(log_path.read_text(), "build output\n")
        # A fresh process must accept the retained evidence and build outputs too.
        second = self.python("print('second process')")
        completed = self.run_commands(second)
        second_log = self.assert_record(completed, second, 0)
        self.assertEqual(second_log.read_text(), "second process\n")
        self.assertEqual(log_path.read_text(), "build output\n")

    def test_tracked_files_in_build_directories_are_source(self) -> None:
        tracked = self.root / "build" / "tracked.py"
        tracked.parent.mkdir()
        tracked.write_text("print('tracked source')\n")
        self.git("add", "-f", "build/tracked.py")
        self.git("commit", "-qm", "Tracked source under output directory")
        self.revision = self.git("rev-parse", "HEAD").stdout.strip()
        command = self.python(
            "from pathlib import Path; Path('build/tracked.py').write_text('changed')"
        )
        completed = self.run_commands(command, self.later_command())
        log_path = self.assert_record(
            completed, command, 125,
            status="status=source-identity-refusal phase=after",
        )
        self.assertIn("build/tracked.py", log_path.read_text())
        self.assert_no_later_command(log_path)

    def test_launch_errors_have_distinct_evidence_and_stop_sequence(self) -> None:
        nonexecutable = self.directory / "non-executable"
        nonexecutable.write_text("#!/bin/sh\nexit 0\n")
        nonexecutable.chmod(0o600)
        for executable, code, error_number in (
            (self.directory / "absent-executable", 127, errno.ENOENT),
            (nonexecutable, 126, errno.EACCES),
        ):
            with self.subTest(executable=executable.name):
                command = [str(executable)]
                completed = self.run_commands(command, self.later_command())
                log_path = self.assert_record(
                    completed, command, code, status="status=launch-failure",
                )
                self.assertEqual(
                    log_path.read_text(),
                    f"launch-failure errno={error_number} error="
                    f"{OSError(error_number, os.strerror(error_number), str(executable))}\n",
                )
                self.assertNotIn("Traceback", completed.stdout)
                self.assert_no_later_command(log_path)

    def test_child_exit_127_is_execution_evidence(self) -> None:
        command = self.python("print('child executed'); raise SystemExit(127)")
        completed = self.run_commands(command, self.later_command())
        log_path = self.assert_record(completed, command, 127)
        self.assertEqual(log_path.read_text(), "child executed\n")
        self.assertNotIn("status=launch-failure", completed.stdout)
        self.assert_no_later_command(log_path)

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
