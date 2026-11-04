"""020: drive the production retirement CLI through argv and argparse to stderr.

There is no replay server or artifact codec after retirement. Executing the actual
entry point in a fresh namespace proves refusal before any artifact transport;
no replacement producer, transport, native binary or external endpoint is used.
"""

from contextlib import redirect_stderr, redirect_stdout
from io import StringIO
import os
from pathlib import Path
import runpy
import sys

import pytest


QUAL = Path(__file__).resolve().parents[1]
CLI = QUAL / "replay-authority.py"
RETIRED = "batch-WAL replay authority qualification has been retired"
EVIDENCE_ENV = "LAYERX_REPLAY_AUTHORITY_EVIDENCE"


def invoke(*arguments):
    stdout, stderr = StringIO(), StringIO()
    original = sys.argv
    try:
        sys.argv = [str(CLI), *map(str, arguments)]
        with redirect_stdout(stdout), redirect_stderr(stderr):
            with pytest.raises(SystemExit) as stopped:
                runpy.run_path(str(CLI), run_name="__main__")
    finally:
        sys.argv = original
    return stopped.value.code, stdout.getvalue(), stderr.getvalue()


def test_retired_help_describes_environment_and_artifact_lifecycle():
    code, stdout, stderr = invoke("--help")
    assert code == 0
    assert stderr == ""
    help_text = " ".join(stdout.split())
    assert "--build" in help_text
    assert "--prepare-fixture PATH" in help_text
    assert help_text.count("retired; always refuses") == 2
    assert "all qualification modes refuse" in help_text
    assert f"{EVIDENCE_ENV} is ignored" in help_text
    assert "no evidence or fixtures are read or produced" in help_text


@pytest.mark.parametrize("mode", ["default", "build", "prepare-fixture"])
def test_retired_modes_refuse_on_each_fresh_invocation_without_artifacts(tmp_path, mode):
    output = tmp_path / "generated" / "replay-authority-fixture-base.inc"
    arguments = {
        "default": [],
        "build": ["--build"],
        "prepare-fixture": ["--prepare-fixture", output],
    }[mode]
    for _ in range(2):
        code, stdout, stderr = invoke(*arguments)
        assert code == 2
        assert stdout == ""
        assert f"error: {RETIRED}\n" in stderr
        assert "Traceback" not in stderr
        assert not output.exists()
        assert list(tmp_path.iterdir()) == []


@pytest.mark.parametrize("environment_state", ["unset", "missing", "existing"])
def test_retired_environment_neither_enables_production_nor_changes_evidence(
    tmp_path, environment_state
):
    evidence = tmp_path / "evidence.json"
    fixture = tmp_path / "fixture.inc"
    fixture.write_bytes(b"retained fixture\x00\xff\n")
    if environment_state == "existing":
        evidence.write_bytes(b"not a valid evidence document\x00\xff\n")
    before = {path.name: path.read_bytes() for path in tmp_path.iterdir()}
    original = os.environ.get(EVIDENCE_ENV)
    try:
        if environment_state == "unset":
            os.environ.pop(EVIDENCE_ENV, None)
        else:
            os.environ[EVIDENCE_ENV] = str(evidence)
        for arguments in [[], ["--build"], ["--prepare-fixture", fixture]]:
            code, stdout, stderr = invoke(*arguments)
            assert code == 2
            assert stdout == ""
            assert f"error: {RETIRED}\n" in stderr
            assert {path.name: path.read_bytes() for path in tmp_path.iterdir()} == before
    finally:
        if original is None:
            os.environ.pop(EVIDENCE_ENV, None)
        else:
            os.environ[EVIDENCE_ENV] = original


@pytest.mark.parametrize(
    ("arguments", "diagnostic"),
    [
        (["--build", "--prepare-fixture", "fixture.inc"], "not allowed with argument"),
        (["--prepare-fixture"], "expected one argument"),
        (["--bu"], "unrecognized arguments"),
        (["--prepare-f", "fixture.inc"], "unrecognized arguments"),
        (["--evidence", "evidence.json"], "unrecognized arguments"),
    ],
)
def test_retired_cli_keeps_strict_parser_refusals(arguments, diagnostic):
    code, stdout, stderr = invoke(*arguments)
    assert code == 2
    assert stdout == ""
    assert diagnostic in stderr
    assert "Traceback" not in stderr
