import json
import os
from pathlib import Path
import re
import subprocess
import sys

import pytest


ROOT = Path(__file__).resolve().parents[3]
SCRIPT = Path(__file__).with_name("run-real-node-tests.sh")
HARNESS = ROOT / "services/agent/crates/layerx-agentd/tests/support"


def provision_environment(overrides):
    script = SCRIPT.read_text(encoding="utf-8")
    preamble, separator, _ = script.partition('\ncase "${1-}" in\n')
    assert separator, "the provisioning preamble must precede target dispatch"
    environment = {
        name: value
        for name, value in os.environ.items()
        if not name.startswith("LAYERX_TEST_PAXEER_")
        and name != "PAXEER_X_RUNTIME_MANIFEST"
    }
    environment.update(overrides)
    return subprocess.run(
        ["sh", "-s", "--", sys.executable],
        input=preamble
        + '\nexport_paxeer_chain_environment\nexec "$1" -c '
        + "'import json, os; print(json.dumps(dict(os.environ)))'\n",
        env=environment,
        capture_output=True,
        text=True,
        check=False,
    )


def test_run_real_node_tests_env():
    required = {
        name
        for source in HARNESS.rglob("*.rs")
        for name in re.findall(
            r'std::env::var(?:_os)?\s*\(\s*"(LAYERX_TEST_PAXEER_[A-Z0-9_]+)"',
            source.read_text(encoding="utf-8"),
        )
    }
    assert required == {
        "LAYERX_TEST_PAXEER_CHAIN_ID",
        "LAYERX_TEST_PAXEER_RPC_PORT",
    }
    result = provision_environment({})
    assert result.returncode == 0, result.stderr
    exported = json.loads(result.stdout)
    assert required <= exported.keys()
    assert exported["LAYERX_TEST_PAXEER_CHAIN_ID"] == "125"
    assert exported["LAYERX_TEST_PAXEER_RPC_PORT"] == "8545"
    script = SCRIPT.read_text(encoding="utf-8")
    assert script.index("\nexport_paxeer_chain_environment\n") < script.index(
        'make "$test_target"'
    )


def test_configured_rpc_port():
    result = provision_environment(
        {"LAYERX_TEST_PAXEER_CHAIN_ID": "999", "LAYERX_TEST_PAXEER_RPC_PORT": "18545"}
    )
    assert result.returncode == 0, result.stderr
    exported = json.loads(result.stdout)
    assert exported["LAYERX_TEST_PAXEER_CHAIN_ID"] == "125"
    assert exported["LAYERX_TEST_PAXEER_RPC_PORT"] == "18545"


@pytest.mark.parametrize(
    "endpoint", [{"ports": [28545, 28546]}, {"chain_rpc": "http://127.0.0.1:28545"}]
)
def test_runtime_manifest_provides_both_values(tmp_path, endpoint):
    manifest = tmp_path / "runtime manifest.json"
    manifest.write_text(json.dumps({"chain_id": 126, **endpoint}), encoding="utf-8")
    result = provision_environment(
        {
            "PAXEER_X_RUNTIME_MANIFEST": str(manifest),
            "LAYERX_TEST_PAXEER_CHAIN_ID": "999",
            "LAYERX_TEST_PAXEER_RPC_PORT": "18545",
        }
    )
    assert result.returncode == 0, result.stderr
    exported = json.loads(result.stdout)
    assert exported["LAYERX_TEST_PAXEER_CHAIN_ID"] == "126"
    assert exported["LAYERX_TEST_PAXEER_RPC_PORT"] == "28545"


@pytest.mark.parametrize(
    "document",
    [
        "not JSON",
        {},
        {"chain_id": 126},
        {"chain_id": 126, "ports": []},
        {"chain_id": 0, "ports": [28545]},
        {"chain_id": 2**64, "ports": [28545]},
        {"chain_id": True, "ports": [28545]},
        {"chain_id": 126, "ports": [0]},
        {"chain_id": 126, "ports": [65536]},
        {"chain_id": 126, "ports": ["28545; exit 0"]},
        {"chain_id": 126, "chain_rpc": "http://127.0.0.1"},
    ],
)
def test_invalid_manifest_refuses_without_defaulting(tmp_path, document):
    manifest = tmp_path / "runtime.json"
    manifest.write_text(json.dumps(document), encoding="utf-8")
    result = provision_environment({"PAXEER_X_RUNTIME_MANIFEST": str(manifest)})
    assert result.returncode != 0
    assert result.stderr
    assert result.stdout == ""


def test_missing_manifest_refuses_without_defaulting(tmp_path):
    result = provision_environment(
        {"PAXEER_X_RUNTIME_MANIFEST": str(tmp_path / "missing.json")}
    )
    assert result.returncode != 0
    assert result.stderr
    assert result.stdout == ""


@pytest.mark.parametrize("port", ["", "0", "65536", "not-a-port"])
def test_invalid_configured_rpc_port_refuses(port):
    result = provision_environment({"LAYERX_TEST_PAXEER_RPC_PORT": port})
    assert result.returncode != 0
    assert result.stderr
    assert result.stdout == ""


def test_disposition_retains_evidence_and_names_repair():
    path = ROOT / "spec/layerx-merge/dispositions/d9-2.kvx"
    disposition = path.read_text(encoding="utf-8").split(
        "[disposition.9.3.1.3]\n", 1
    )[1].split("\n[", 1)[0]
    assert 'status = "open"' in disposition
    assert 'follow_up = "22.9.8"' in disposition
    assert (
        'evidence = "The shared release harness still requires '
        "LAYERX_TEST_PAXEER_CHAIN_ID at "
        "services/agent/crates/layerx-agentd/tests/support/real_authority.rs:1441, "
        "but services/agent/tools/run-real-node-tests.sh:48 invokes the generic "
        "make target without provisioning the chain variables that "
        'tools/qualification/paxeer-x/caps-codecs.py:74 supplies."'
    ) in disposition
