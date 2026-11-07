import importlib.util
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys

import pytest

ROOT = Path(__file__).resolve().parents[3]
TOOL = ROOT / "tools/release/upgrade_info.py"
WORKFLOW = ROOT / ".github/workflows/paxeer-release-check.yml"
SPEC = importlib.util.spec_from_file_location("upgrade_info", TOOL)
upgrade_info = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(upgrade_info)


def run_release(tool, sums, *arguments):
    return subprocess.run(
        [sys.executable, str(tool), "release", "--sums", str(sums), *arguments],
        capture_output=True,
        text=True,
        check=False,
    )


def test_plan_name_table_matches_node_declarations():
    node = (ROOT / "node/upgrades.go").read_text()
    common = (ROOT / "precompiles/common/precompiles.go").read_text()
    symbols = "MergeUpgrade|AnchorStoreDeleteUpgrade"
    declarations = dict(
        re.findall(rf'^const ({symbols}) = "([^"]+)"$', node + "\n" + common, re.M)
    )
    assert declarations == {
        "MergeUpgrade": "v7.0",
        "AnchorStoreDeleteUpgrade": "v7.0-anchor-store-delete",
    }
    assert upgrade_info.REGISTERED_PLAN_NAMES == declarations
    assert re.search(r"^const MergeUpgrade = pcommon\.MergeUpgrade$", node, re.M)
    for symbol in declarations:
        assert f"SetUpgradeHandler({symbol}," in node


def test_version_json_carries_v700_and_merge_plan():
    identity = json.loads(TOOL.with_name("version.json").read_text())
    assert identity == {"version": "v7.0.0", "upgrade": "v7.0"}
    assert upgrade_info.release_identity() == identity


def test_release_accepts_merge_plan_and_uses_monorepo_asset(tmp_path):
    binary = tmp_path / "paxd-v7.0.0-linux-amd64"
    shutil.copy2(sys.executable, binary)
    checksums = subprocess.run(
        ["sha256sum", binary.name],
        cwd=tmp_path,
        capture_output=True,
        text=True,
        check=True,
    ).stdout
    sums = tmp_path / "SHA256SUMS"
    sums.write_text(checksums)
    output = tmp_path / "release.json"
    for arguments in ((), ("--name", "v7.0")):
        result = run_release(TOOL, sums, "--tag", "paxeer-network/v7.0.0", *arguments)
        assert result.returncode == 0, result.stderr
        document = json.loads(result.stdout)
        assert document["name"] == "v7.0"
        assert json.loads(document["info"]) == {
            "binaries": {
                "linux/amd64": (
                    "https://github.com/Sidiora-Labs/paxeer-monorepo/releases/"
                    "download/paxeer-network/v7.0.0/paxd-v7.0.0-linux-amd64"
                    f"?checksum=sha256:{checksums.split()[0]}"
                )
            }
        }
        output.write_text(result.stdout)
        assert upgrade_info.release_document_name(
            output, checksums.split()[0], upgrade_info.release_identity()
        ) == "v7.0"
    home = tmp_path / "home"
    result = subprocess.run(
        [
            sys.executable,
            str(TOOL),
            "node",
            "--home",
            str(home),
            "--halt-height",
            "20",
            "--release-doc",
            str(output),
            "--sha256",
            checksums.split()[0],
        ],
        capture_output=True,
        text=True,
        check=False,
    )
    assert result.returncode == 0, result.stderr
    assert json.loads((home / "data/upgrade-info.json").read_text()) == {
        "name": "v7.0",
        "height": 21,
    }


def test_release_refuses_v7_without_writing_output(tmp_path):
    sums = tmp_path / "SHA256SUMS"
    sums.write_text("a" * 64 + "  paxd-v7.0.0-linux-amd64\n")
    output = tmp_path / "release.json"
    result = run_release(TOOL, sums, "--name", "v7", "--out", str(output))
    assert result.returncode == 1
    assert "name v7 is not the release upgrade v7.0" in result.stderr
    assert result.stdout == ""
    assert not output.exists()


@pytest.mark.parametrize(
    "name", ["v6.12.0", "v7.0.0", *upgrade_info.REGISTERED_PLAN_NAMES.values()]
)
def test_release_identity_accepts_semver_and_registered_plans(tmp_path, name):
    tool = tmp_path / TOOL.name
    shutil.copy2(TOOL, tool)
    tool.with_name("version.json").write_text(
        json.dumps({"version": "v7.0.0", "upgrade": name})
    )
    sums = tmp_path / "SHA256SUMS"
    sums.write_text("a" * 64 + "  paxd-v7.0.0-linux-amd64\n")
    result = run_release(tool, sums, "--name", name)
    assert result.returncode == 0, result.stderr
    assert json.loads(result.stdout)["name"] == name


@pytest.mark.parametrize(
    "field,value",
    [
        ("upgrade", "v7"),
        ("upgrade", "v7.1"),
        ("upgrade", "v7.0-unknown"),
        ("upgrade", None),
        ("version", "v7.0"),
        ("version", "v7.0-anchor-store-delete"),
    ],
)
def test_release_identity_refuses_unregistered_names(tmp_path, field, value):
    tool = tmp_path / TOOL.name
    shutil.copy2(TOOL, tool)
    identity = {"version": "v7.0.0", "upgrade": "v7.0"}
    identity[field] = value
    tool.with_name("version.json").write_text(json.dumps(identity))
    sums = tmp_path / "SHA256SUMS"
    sums.write_text("a" * 64 + "  paxd-v7.0.0-linux-amd64\n")
    result = run_release(tool, sums)
    assert result.returncode == 1
    assert f"invalid release {field}" in result.stderr
    assert result.stdout == ""


def test_release_url_names_paxeer_monorepo():
    assert upgrade_info.RELEASE_URL == (
        "https://github.com/Sidiora-Labs/paxeer-monorepo/releases/download"
    )


def test_workflow_greps_declaring_lines():
    workflow = WORKFLOW.read_text()
    match = re.search(r"if ! (grep .*?); then", workflow, re.S)
    assert match is not None
    command = match[1]
    assert "^[[:space:]]*const " in command
    assert "(MergeUpgrade|AnchorStoreDeleteUpgrade|V6120Upgrade)" in command
    assert "node/upgrades.go precompiles/common/precompiles.go" in command
    assert "node/upgrades_v6120.go" not in command
    for name in upgrade_info.REGISTERED_PLAN_NAMES.values():
        result = subprocess.run(
            ["bash", "-euo", "pipefail", "-c", command],
            cwd=ROOT,
            env={**os.environ, "upgrade": name},
            capture_output=True,
            text=True,
            check=False,
        )
        assert result.returncode == 0, result.stderr
    for name in ("v7", "v7.0.*", "v7.0-unknown", "pcommon.MergeUpgrade"):
        result = subprocess.run(
            ["bash", "-euo", "pipefail", "-c", command],
            cwd=ROOT,
            env={**os.environ, "upgrade": name},
            capture_output=True,
            text=True,
            check=False,
        )
        assert result.returncode == 1, result.stderr
