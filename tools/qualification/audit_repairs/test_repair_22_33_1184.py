"""Source and real-shell proof for retirement of the unauthenticated installer."""

import ast
import os
import shlex
import subprocess
import tomllib
from pathlib import Path

import pytest


ROOT = Path(__file__).resolve().parents[3]
SCRIPT = ROOT / "consensus/scripts/get_nodejs.sh"
RECORD = Path(__file__).with_name("repair_22_33_1184.kvx")


def test_nodejs_bootstrap_http_retired_source():
    source = SCRIPT.read_text(encoding="utf-8")
    assert source.startswith("#!/usr/bin/env bash\n")
    # Reject expansions even inside otherwise allowed diagnostic commands.
    assert "$" not in source
    assert "`" not in source
    commands = [
        shlex.split(line, comments=True)
        for line in source.replace("\\\n", "").splitlines()
        if line.strip() and not line.lstrip().startswith("#")
    ]
    assert len(commands) == 2
    diagnostic, refusal = commands
    assert diagnostic[:2] == ["printf", "%s\\n"]
    assert len(diagnostic) == 5
    assert diagnostic[-1] == ">&2"
    assert "retired" in diagnostic[2]
    assert "Node 12 is unsupported" in diagnostic[2]
    assert "Dredd is archived" in diagnostic[2]
    assert "supported Node.js" in diagnostic[3]
    assert "mise" in diagnostic[3]
    assert refusal == ["exit", "1"]
    # No hidden operators, substitutions or alternate commands in the two lines.
    lexer = shlex.shlex(source.replace("\\\n", ""), posix=True, punctuation_chars=True)
    lexer.whitespace_split = True
    tokens = list(lexer)
    assert tokens == diagnostic[:-1] + [">&", "2"] + refusal


def test_nodejs_bootstrap_http_shell_syntax():
    result = subprocess.run(
        ["bash", "--noprofile", "--norc", "-n", str(SCRIPT)],
        capture_output=True,
        text=True,
        timeout=5,
        check=False,
    )
    assert result.returncode == 0, result.stderr
    assert result.stdout == ""
    assert result.stderr == ""


@pytest.mark.parametrize("arguments", [[], ["--help"], ["v12.9.0"], ["--force"]])
def test_nodejs_bootstrap_http_refuses_without_mutation(tmp_path, arguments):
    # Existing caller files must survive an invocation of the production script.
    for relative in (
        ".local/node/archive.tar.gz",
        ".local/bin/node",
        ".local/bin/npm",
        ".local/bin/dredd",
    ):
        path = tmp_path / relative
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_bytes(b"existing caller data\n")
    before = {
        path.relative_to(tmp_path): path.read_bytes()
        for path in tmp_path.rglob("*")
        if path.is_file()
    }
    result = subprocess.run(
        ["bash", "--noprofile", "--norc", str(SCRIPT), *arguments],
        cwd=tmp_path,
        env={**os.environ, "HOME": str(tmp_path)},
        capture_output=True,
        text=True,
        timeout=5,
        check=False,
    )
    assert result.returncode == 1
    assert result.stdout == ""
    assert "retired" in result.stderr
    assert "Node 12 is unsupported" in result.stderr
    assert "Dredd is archived" in result.stderr
    assert "mise" in result.stderr
    after = {
        path.relative_to(tmp_path): path.read_bytes()
        for path in tmp_path.rglob("*")
        if path.is_file()
    }
    assert after == before
    assert not any(path.is_symlink() for path in tmp_path.rglob("*"))


def test_nodejs_bootstrap_http_record_maps_proof():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    task = record["task"]["22"]["33"]["1184"]
    finding = record["finding"]["nodejs-bootstrap-http"]
    audit = tomllib.loads(
        (ROOT / "spec/layerx-merge/programme/audit/records/22.30.55.kvx").read_text(
            encoding="utf-8"
        )
    )["finding"]["nodejs-bootstrap-http"]
    for key in ("file", "class", "severity", "owner_domain"):
        assert finding[key] == audit[key]
    declared_tests = {
        node.name
        for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert len(finding["tests"]) == len(declared_tests) == 4
    assert set(finding["tests"]) == declared_tests
    assert task["audit"] == "22.30.55"
    assert task["requirement"] == "22998.1"
    assert task["build_cmd"] == "bash -n consensus/scripts/get_nodejs.sh"
    assert task["verify_cmd"] == (
        "python3 -m pytest -q tools/qualification/audit_repairs/"
        "test_repair_22_33_1184.py"
    )
    assert task["wiring_requests"] == []
    assert "No pre-change verification was run" in task["baseline"]
    assert task["source_proof"]
    assert task["review_only"]
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in task["evidence"]
    assert record["maintenance"]["dredd"]["archived"] is True
    assert record["maintenance"]["dredd"]["source"] == (
        "https://api.github.com/repos/apiaryio/dredd"
    )
