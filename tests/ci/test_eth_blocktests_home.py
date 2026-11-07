import os
import re
import subprocess
import textwrap
from pathlib import Path

import pytest


WORKFLOW = (
    Path(__file__).resolve().parents[2]
    / ".github/workflows/paxeer-eth-blocktests.yml"
)


def _step_script(name, index):
    workflow = WORKFLOW.read_text(encoding="utf-8")
    header = f"      - name: {name}\n"
    assert workflow.count(header) == 1
    step = workflow.split(header, 1)[1].split("\n      - ", 1)[0]
    run = re.search(r"(?m)^        run: \|\n((?:          [^\n]*(?:\n|$)|\n)+)", step)
    assert run is not None, f"Missing bash script in {name}"
    script = textwrap.dedent(run.group(1)).replace(
        "${{ matrix.runner-index }}", str(index)
    )
    assert "${{" not in script, f"Unresolved workflow expression in {name}"
    return script


def _environment(tmp_path):
    home = tmp_path / "job home"
    home.mkdir()
    runner_temp = tmp_path / "runner temp"
    runner_temp.mkdir()
    runner_alias = tmp_path / "runner temp alias"
    runner_alias.symlink_to(runner_temp, target_is_directory=True)
    assert not runner_temp.resolve().is_relative_to(home.resolve())
    return {
        **os.environ,
        "HOME": str(home),
        "RUNNER_TEMP": str(runner_alias),
        "GITHUB_OUTPUT": str(tmp_path / "github output"),
    }


def _run_step(script, environment):
    return subprocess.run(
        ["bash", "--noprofile", "--norc", "-e", "-o", "pipefail", "-c", script],
        env=environment,
        capture_output=True,
        text=True,
        check=False,
        timeout=10,
    )


@pytest.mark.parametrize("index", [0, 4])
def test_prepare_step_has_no_home_requirement(tmp_path, index):
    environment = _environment(tmp_path)
    prepare = _step_script("Prepare isolated Pax home", index)
    result = _run_step(prepare, environment)
    assert result.returncode == 0, result.stdout + result.stderr
    output = Path(environment["GITHUB_OUTPUT"]).read_text(encoding="utf-8")
    lines = output.splitlines()
    assert len(lines) == 1
    assert lines[0].startswith("path=")
    pax_home = Path(lines[0].removeprefix("path="))
    assert pax_home.is_dir()
    assert pax_home == pax_home.resolve()
    assert pax_home.parent == Path(environment["RUNNER_TEMP"]).resolve()
    assert pax_home.name.startswith(f"pax-blocktests-{index}-")
    sentinel = pax_home / "blocktest data"
    sentinel.write_text("remove with the isolated home", encoding="utf-8")

    environment["PAX_HOME"] = str(pax_home)
    remove = _step_script("Remove isolated Pax home", index)
    result = _run_step(remove, environment)
    assert result.returncode == 0, result.stdout + result.stderr
    assert not pax_home.exists()
    assert Path(environment["HOME"]).is_dir()
    assert Path(environment["RUNNER_TEMP"]).resolve().is_dir()


@pytest.mark.parametrize(
    "case",
    ["wrong-prefix", "wrong-index", "outside-temp", "temp-root", "traversal", "symlink"],
)
def test_remove_step_keeps_runner_temp_prefix_guard(tmp_path, case):
    environment = _environment(tmp_path)
    runner_temp = Path(environment["RUNNER_TEMP"]).resolve()
    if case == "wrong-prefix":
        target = runner_temp / "unrelated-home"
        target.mkdir()
        candidate = target
    elif case == "wrong-index":
        target = runner_temp / "pax-blocktests-1-keep"
        target.mkdir()
        candidate = target
    elif case == "outside-temp":
        target = tmp_path / "pax-blocktests-0-keep"
        target.mkdir()
        candidate = target
    elif case == "temp-root":
        target = runner_temp
        candidate = target
    elif case == "traversal":
        target = tmp_path / "outside-home"
        target.mkdir()
        intermediate = runner_temp / "pax-blocktests-0-traversal"
        intermediate.mkdir()
        candidate = intermediate / ".." / ".." / target.name
    elif case == "symlink":
        target = tmp_path / "outside-home"
        target.mkdir()
        candidate = runner_temp / "pax-blocktests-0-symlink"
        candidate.symlink_to(target, target_is_directory=True)
    else:
        raise AssertionError(f"Unknown cleanup case: {case}")
    sentinel = target / "preserve data"
    sentinel.write_text("must survive refused cleanup", encoding="utf-8")

    environment["PAX_HOME"] = str(candidate)
    remove = _step_script("Remove isolated Pax home", 0)
    result = _run_step(remove, environment)
    assert result.returncode == 1, result.stdout + result.stderr
    assert (
        "Refusing to remove Pax home outside the runner-owned blocktest directory"
        in result.stderr
    ), result.stderr
    assert target.is_dir()
    assert sentinel.read_text(encoding="utf-8") == "must survive refused cleanup"
    assert runner_temp.is_dir()
    assert Path(environment["HOME"]).is_dir()
