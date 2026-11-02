import shlex
import subprocess
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]


def _assert_gate_lists_runnable_commands(wave: str) -> None:
    script = ROOT / "scripts" / "gates" / f"wave-{wave}.sh"
    assert script.is_file(), f"Missing wave gate: {script}"
    listed = subprocess.run(
        [str(script), "--list"],
        cwd=ROOT,
        capture_output=True,
        text=True,
        check=False,
    )
    assert listed.returncode == 0, (
        f"{script} --list exited {listed.returncode}\n{listed.stdout}{listed.stderr}"
    )
    commands = listed.stdout.splitlines()
    assert commands, f"{script} --list returned no commands"
    for command in commands:
        arguments = shlex.split(command)
        assert arguments, f"{script} listed an empty command: {command!r}"
        if arguments[0] != "make":
            continue
        targets = [argument for argument in arguments[1:] if argument != "-n"]
        assert targets, f"{script} listed make without a target: {command!r}"
        for target in targets:
            assert not target.startswith("-"), (
                f"{script} listed an unsupported make option: {command!r}"
            )
            dry_run = subprocess.run(
                ["make", "-n", target],
                cwd=ROOT,
                capture_output=True,
                text=True,
                check=False,
            )
            assert dry_run.returncode == 0, (
                f"{script} listed {command!r}; make -n {target} "
                f"exited {dry_run.returncode}\n{dry_run.stdout}{dry_run.stderr}"
            )


def test_wave_0_gate_lists_runnable_commands() -> None:
    _assert_gate_lists_runnable_commands("0")


def test_wave_1_gate_lists_runnable_commands() -> None:
    _assert_gate_lists_runnable_commands("1")


def test_wave_2_gate_lists_runnable_commands() -> None:
    _assert_gate_lists_runnable_commands("2")


def test_wave_3_gate_lists_runnable_commands() -> None:
    _assert_gate_lists_runnable_commands("3")


def test_wave_4_gate_lists_runnable_commands() -> None:
    _assert_gate_lists_runnable_commands("4")


def test_wave_5_gate_lists_runnable_commands() -> None:
    _assert_gate_lists_runnable_commands("5")


def test_wave_6_gate_lists_runnable_commands() -> None:
    _assert_gate_lists_runnable_commands("6")


def test_wave_programme_gate_lists_runnable_commands() -> None:
    _assert_gate_lists_runnable_commands("programme")
