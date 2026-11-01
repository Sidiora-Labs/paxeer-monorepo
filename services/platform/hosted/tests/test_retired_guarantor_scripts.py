import json
from pathlib import Path
import stat
import subprocess
import sys


def test_hosted_tree_ships_no_guarantor_script():
    hosted = Path(__file__).resolve().parents[1]
    node = hosted / "node"
    assert node.is_dir(), "Hosted node directory must exist"

    launcher = ".".join(("guarantor", "sh"))
    authority = ".".join(("checkpoint-authority", "py"))
    retired = (
        node / launcher,
        node / authority,
        node / "tests" / "guarantor-provision.sh",
        node / "tests" / "publication-authorization-test.py",
    )
    for path in retired:
        assert not path.exists(), f"Retired script still exists: {path}"
        assert not path.is_symlink(), f"Retired script is a symlink: {path}"

    for path in sorted(hosted.rglob("*")):
        assert path.name not in (launcher, authority), f"Retired script remains: {path}"
        if path.is_file():
            contents = path.read_bytes()
            for name in (launcher, authority):
                assert name.encode("ascii") not in contents, (
                    f"Hosted file still names retired script {name}: {path}"
                )


def test_publication_policy_refuses_the_authorization_operation(tmp_path):
    policy = Path(__file__).with_name("publication-policy.py")
    output = tmp_path / "authorization.json"
    result = subprocess.run(
        [
            sys.executable,
            str(policy),
            "authorization",
            str(output),
            "402",
            "125",
            "0x" + "12" * 20,
            "0x" + "23" * 20,
            "0x" + "34" * 20,
            "45" * 32,
            "56" * 32,
            "67" * 20,
        ],
        capture_output=True,
        text=True,
        check=False,
    )
    assert result.returncode != 0
    assert result.stderr.strip() == "publication policy creation refused"
    assert result.stdout == ""
    assert not output.exists()
    assert list(tmp_path.iterdir()) == []

    treasury = tmp_path / "treasury.json"
    result = subprocess.run(
        [
            sys.executable,
            str(policy),
            "treasury",
            str(treasury),
            "402",
            "AB" * 32,
            "CD" * 20,
        ],
        capture_output=True,
        text=True,
        check=False,
    )
    assert result.returncode == 0, result.stderr
    assert result.stdout == ""
    assert result.stderr == ""
    assert json.loads(treasury.read_text(encoding="utf-8")) == {
        "version": 1,
        "network_id": 402,
        "asset_id": "ab" * 32,
        "recipient": "cd" * 20,
    }
    assert stat.S_IMODE(treasury.stat().st_mode) == 0o600
