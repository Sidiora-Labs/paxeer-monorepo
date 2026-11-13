"""Exercise the production manifest guard without deployment tools or credentials."""

import json
import os
from pathlib import Path
import runpy
import subprocess
import tomllib

import pytest


ROOT = Path(__file__).resolve().parents[3]
DEPLOY = ROOT / "services/bridge/deploy"
CHECK = DEPLOY / "check-attestor-manifest.py"
MANIFEST = DEPLOY / "attestors.json"
VARIABLE = "PAXEER_BRIDGE_ATTESTOR_MANIFEST"
GOVERNANCE_STEP = "governance signer-set approval (22.31.402; decision 2522)"
GUARD = (
    'python3 "${BASH_SOURCE[0]%${BASH_SOURCE[0]##*/}}check-attestor-manifest.py"'
    ' || exit "$?"'
)
ENTRY_POINTS = ("checklist.sh", "deploy-evm-chain.sh")


def filled_manifest():
    return json.loads((DEPLOY / "tests/fixtures/attestors.json").read_text())


def write_manifest(tmp_path, document):
    path = tmp_path / "attestor manifest.json"
    path.write_text(json.dumps(document))
    return path


def run_check(monkeypatch, capsys, path=None):
    if path is None:
        monkeypatch.delenv(VARIABLE, raising=False)
    else:
        monkeypatch.setenv(VARIABLE, str(path))
    with pytest.raises(SystemExit) as result:
        runpy.run_path(str(CHECK), run_name="__main__")
    output = capsys.readouterr()
    assert output.out == ""
    return result.value.code, output.err


def test_accepts_committed_governed_manifest(monkeypatch, capsys):
    assert json.loads(MANIFEST.read_text()) == {
        "attestors": ["0x12598F221f1e645322C7665537D4Ab130281f51d"],
        "threshold": 1,
    }
    assert run_check(monkeypatch, capsys) == (0, "")


def test_refuses_committed_manifest_naming_governance(tmp_path, monkeypatch, capsys):
    # Preserve the former committed document as an explicit refusal fixture.
    document = {
        "attestors": [f"PLACEHOLDER:attestor-{i}" for i in range(1, 6)],
        "threshold": 3,
        "pending_governance": True,
    }
    path = write_manifest(tmp_path, document)
    code, error = run_check(monkeypatch, capsys, path)
    assert code == 1
    assert str(path) in error
    assert "pending_governance=true" in error
    assert GOVERNANCE_STEP in error
    assert "before the upgrade" in error
    assert "bridge governance must supply" in error


@pytest.mark.parametrize("byte", ["00", "11", "ab", "ff"])
def test_refuses_repeated_byte_addresses(tmp_path, monkeypatch, capsys, byte):
    document = {"attestors": ["0x" + byte * 20], "threshold": 1}
    path = write_manifest(tmp_path, document)
    code, error = run_check(monkeypatch, capsys, path)
    assert code == 1
    assert str(path) in error
    assert "attestors[0]: repeated-byte placeholder address" in error


@pytest.mark.parametrize("threshold", [0, -1, 6, True, 1.5, "3", None])
def test_refuses_invalid_threshold(tmp_path, monkeypatch, capsys, threshold):
    document = filled_manifest()
    document["threshold"] = threshold
    path = write_manifest(tmp_path, document)
    code, error = run_check(monkeypatch, capsys, path)
    assert code == 1
    assert str(path) in error
    assert "threshold must be an integer between 1 and 5" in error


@pytest.mark.parametrize("threshold", [1, 3, 5])
def test_accepts_filled_manifest(tmp_path, monkeypatch, capsys, threshold):
    document = filled_manifest()
    document["threshold"] = threshold
    document["attestors"][1] = "0x" + document["attestors"][1][2:].upper()
    path = write_manifest(tmp_path, document)
    assert run_check(monkeypatch, capsys, path) == (0, "")


@pytest.mark.parametrize("pending", [True, False, None, 0, "true"])
def test_marker_must_be_removed_even_with_filled_signers(
    tmp_path, monkeypatch, capsys, pending
):
    document = filled_manifest()
    document["pending_governance"] = pending
    path = write_manifest(tmp_path, document)
    code, error = run_check(monkeypatch, capsys, path)
    assert code == 1
    assert str(path) in error
    assert f"pending_governance={json.dumps(pending)}" in error
    assert GOVERNANCE_STEP in error


@pytest.mark.parametrize(
    "invalid", ["named-slot", "duplicate", "descending", "empty", "too-many"]
)
def test_refuses_invalid_signer_sets(tmp_path, monkeypatch, capsys, invalid):
    document = filled_manifest()
    if invalid == "named-slot":
        document["attestors"][0] = "PLACEHOLDER:attestor-1"
        expected = "expected a live 20-byte address"
    elif invalid == "duplicate":
        document["attestors"][1] = "0x" + document["attestors"][0][2:].upper()
        expected = "attestors must be distinct and in strictly ascending order"
    elif invalid == "descending":
        document["attestors"].reverse()
        expected = "attestors must be distinct and in strictly ascending order"
    else:
        document["attestors"] = (
            [] if invalid == "empty" else document["attestors"] * 13
        )
        expected = "attestors must contain between 1 and 64 signers"
    path = write_manifest(tmp_path, document)
    code, error = run_check(monkeypatch, capsys, path)
    assert code == 1
    assert str(path) in error
    assert expected in error


@pytest.mark.parametrize(
    "invalid", ["duplicate-field", "tail", "array", "unknown-field", "missing-field"]
)
def test_refuses_invalid_documents(tmp_path, monkeypatch, capsys, invalid):
    document = filled_manifest()
    path = write_manifest(tmp_path, document)
    if invalid == "duplicate-field":
        path.write_text(
            path.read_text().replace(
                '"threshold": 3', '"threshold": 3, "threshold": 1'
            )
        )
        expected = "duplicate JSON field threshold"
    elif invalid == "tail":
        path.write_text(path.read_text() + "{}")
        expected = "Extra data"
    elif invalid == "array":
        path.write_text("[]")
        expected = "manifest must be a JSON object"
    else:
        if invalid == "unknown-field":
            document["approval"] = "unexpected-schema-field"
        else:
            del document["threshold"]
        path = write_manifest(tmp_path, document)
        expected = "manifest must contain only attestors and threshold"
    code, error = run_check(monkeypatch, capsys, path)
    assert code == 1
    assert str(path) in error
    assert expected in error


def test_missing_manifest_does_not_fall_back(tmp_path, monkeypatch, capsys):
    path = tmp_path / "missing.json"
    code, error = run_check(monkeypatch, capsys, path)
    assert code == 1
    assert str(path) in error
    assert "No such file" in error


@pytest.mark.parametrize("entry", ENTRY_POINTS)
@pytest.mark.parametrize("invocation", ["absolute", "relative", "basename"])
@pytest.mark.parametrize("manifest_kind", ["committed", "repeated-byte", "threshold", "filled"])
def test_entry_points_guard_first(tmp_path, entry, invocation, manifest_kind):
    script = DEPLOY / entry
    commands = [
        line
        for line in script.read_text().splitlines()
        if line.strip() and not line.lstrip().startswith("#")
    ]
    assert commands[0] == GUARD
    assert commands[1] == "set -euo pipefail"
    parsed = subprocess.run(
        ["bash", "-n", str(script)], capture_output=True, text=True
    )
    assert parsed.returncode == 0, parsed.stderr

    environment = {
        "PATH": os.environ.get("PATH", os.defpath),
        "PYTHONDONTWRITEBYTECODE": "1",
    }
    if manifest_kind == "committed":
        path = MANIFEST
        expected = f"usage: {entry}"
    else:
        document = filled_manifest()
        if manifest_kind == "repeated-byte":
            document["attestors"][0] = "0x" + "11" * 20
            expected = "repeated-byte placeholder address"
        elif manifest_kind == "threshold":
            document["threshold"] = len(document["attestors"]) + 1
            expected = "threshold must be an integer between 1 and 5"
        else:
            expected = f"usage: {entry}"
        path = write_manifest(tmp_path, document)
        environment[VARIABLE] = str(path)

    if invocation == "absolute":
        argument, cwd = str(script), tmp_path
    elif invocation == "relative":
        argument, cwd = str(script.relative_to(ROOT)), ROOT
    else:
        argument, cwd = entry, DEPLOY
    result = subprocess.run(
        ["bash", "--noprofile", "--norc", "-x", argument],
        cwd=cwd,
        env=environment,
        capture_output=True,
        text=True,
        timeout=10,
    )
    assert result.returncode == (
        2 if manifest_kind in ("committed", "filled") else 1
    ), result.stderr
    assert result.stdout == ""
    assert expected in result.stderr
    trace = [line for line in result.stderr.splitlines() if line.startswith("+")]
    assert trace[0].startswith("+ python3 ")
    assert "check-attestor-manifest.py" in trace[0]
    if manifest_kind not in ("committed", "filled"):
        assert str(path) in result.stderr
        assert trace[1:] == ["+ exit 1"]
    else:
        assert trace[1] == "+ set -euo pipefail"
        assert trace[-1] == "+ exit 2"


def test_task_record_names_only_governance_signer_set_wiring():
    record = tomllib.loads((DEPLOY / "repair_22_31_446.kvx").read_text())[
        "repair"
    ]["22"]["31"]["446"]
    assert record["status"] == "governed-manifest-live"
    assert record["governance_step"] == (
        f"{GOVERNANCE_STEP}: CLOSED by decision 3042; attestors.json records "
        "0x12598F221f1e645322C7665537D4Ab130281f51d at threshold 1 with no pending marker."
    )
    assert record["wiring_requests"] == []
    assert record["wiring_fulfilled"] == (
        "Decision 3042 supplies the governed signer set and threshold."
    )
    assert record["proving_tests"] == [
        "test_accepts_committed_governed_manifest",
        "test_refuses_committed_manifest_naming_governance",
        "test_refuses_repeated_byte_addresses",
        "test_refuses_invalid_threshold",
        "test_accepts_filled_manifest",
        "test_marker_must_be_removed_even_with_filled_signers",
        "test_refuses_invalid_signer_sets",
        "test_refuses_invalid_documents",
        "test_missing_manifest_does_not_fall_back",
        "test_entry_points_guard_first",
        "test_task_record_names_only_governance_signer_set_wiring",
    ]
