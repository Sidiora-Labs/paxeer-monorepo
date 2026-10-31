import ast
import os
from pathlib import Path
import subprocess


ROOT = Path(__file__).resolve().parents[3]
QUALIFICATION = ROOT / "tools/qualification"


def test_no_digest_of_deleted_node_tests():
    for relative in (
        "tools/qualification/paxeer-x/registry-contract.py",
        "tools/qualification/paxeer-x/programs_accounts_winddown.py",
        "scripts/qualification/paxeer-x/program-http-producer.py",
    ):
        source = (ROOT / relative).read_text()
        tree = ast.parse(source, filename=relative)
        strings = [
            node.value
            for node in ast.walk(tree)
            if isinstance(node, ast.Constant) and isinstance(node.value, str)
        ]
        for value in strings:
            assert "services/platform/hosted/agent-boundary/tests/" not in value, relative
        assert "native_interfaces.rs" not in source, relative
        assert "account_upgrades.rs" not in source, relative

    winddown = (QUALIFICATION / "paxeer-x/programs_accounts_winddown.py").read_text()
    assert "'services/platform/hosted/agent-boundary'," not in winddown
    assert "services/platform/hosted/agent-boundary/src" in winddown
    producer = (ROOT / "scripts/qualification/paxeer-x/program-http-producer.py").read_text()
    assert ":(exclude)**/agent-boundary/tests/**" in producer


def test_custody_profile_omits_owner_custody():
    path = QUALIFICATION / "paxeer-x/movement-custody-profile.py"
    source = path.read_text()
    ast.parse(source, filename=str(path))
    assert "owner_custody.py" not in source
    assert "layerx-human-movement-provider" in source
    assert "services/platform/hosted/human/material.py" in source


def test_no_paxeer_client_crate_listed():
    for relative in ("beta_driver.py", "paxeer-x/movement-custody-profile.py"):
        path = QUALIFICATION / relative
        source = path.read_text()
        ast.parse(source, filename=str(path))
        assert "layerx-paxeer-client" not in source, relative


def test_explorer_availability_has_no_index():
    path = QUALIFICATION / "paxeer-x/explorer-availability.py"
    source = path.read_text()
    ast.parse(source, filename=str(path))
    assert "layerx-explorer-index" not in source
    assert "unified_availability" not in source
    assert "PAXEER_X_UNIFIED_AVAILABILITY" not in source
    assert "actual_browser_typecheck" in source
    assert "'typecheck'" in source


def test_retired_human_make_targets_absent():
    for target in ("human-test-explorer", "human-test-paxeer"):
        result = subprocess.run(
            ["make", "-n", target],
            cwd=ROOT,
            env={**os.environ, "LC_ALL": "C"},
            capture_output=True,
            text=True,
            check=False,
        )
        assert result.returncode == 2, result.stdout + result.stderr
        assert f"No rule to make target '{target}'" in result.stderr
        for relative in ("release_runner.py", "beta_driver.py"):
            assert target not in (QUALIFICATION / relative).read_text(), relative

    result = subprocess.run(
        ["make", "-n", "human-test"],
        cwd=ROOT,
        env={**os.environ, "LC_ALL": "C"},
        capture_output=True,
        text=True,
        check=False,
    )
    assert result.returncode == 0, result.stdout + result.stderr
    assert "LAYERX_EXPLORER_CORE_FIXTURE=" in result.stdout
