"""Offline producer, deployment transport and startup-policy proof for 22.33.73."""

import copy
import importlib.util
import json
from pathlib import Path
import subprocess
import sys
import tomllib

import pytest


ROOT = Path(__file__).resolve().parents[3]
DEPLOY = ROOT / "services/interop/deploy/bridge-relayer"
ENTRYPOINT = ROOT / "docker/bridge-relayer/entrypoint.sh"
RENDERER_PATH = DEPLOY / "render-signer-keys.py"
SPEC = importlib.util.spec_from_file_location("bridge_inventory_renderer", RENDERER_PATH)
RENDERER = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(RENDERER)
CHAINS = (
    "paxeer", "arbitrum", "avalanche", "base", "bnb", "ethereum",
    "hyperevm", "optimism", "polygon",
)


def assert_inventory_policy(inventory, instance):
    expected = {
        "bridge/attestor": {
            "handle": "bridge/attestor",
            "algorithm": "secp256k1",
            "key_file": f"/run/secrets/bridge/signer/attestor-{instance}.key",
            "domains": [
                "LayerX/bridge/attest-inbound/v1",
                "LayerX/bridge/attest-outbound/v1",
            ],
        },
        "bridge/fee-payer/solana": {
            "handle": "bridge/fee-payer/solana",
            "algorithm": "ed25519",
            "key_file": "/run/secrets/bridge/signer/solana-fee-payer.json",
            "domains": ["LayerX/bridge/solana-tx/v1"],
        },
    }
    for chain in CHAINS:
        domain = "paxeer" if chain == "paxeer" else "ethereum"
        handle = f"bridge/submitter/{chain}"
        expected[handle] = {
            "handle": handle,
            "algorithm": "secp256k1",
            "key_file": f"/run/secrets/bridge/signer/{chain}-submitter.key",
            "domains": [f"LayerX/bridge/{domain}-eip1559/v1"],
        }
    assert set(inventory) == {"keys"}
    assert len(inventory["keys"]) == 11
    assert {key["handle"]: key for key in inventory["keys"]} == expected


@pytest.mark.parametrize("instance", range(1, 6))
def test_attestor_inventory_path_rendered_secret_matches_deployment(instance, capsys):
    assert RENDERER.main(["--instance", str(instance)]) == 0
    output = capsys.readouterr()
    assert output.err == ""
    assert len(output.out.splitlines()) == 1
    name, value = output.out.strip().split("=", 1)
    assert name == "BRIDGE_SIGNER_KEYS"
    inventory = json.loads(value)
    assert_inventory_policy(inventory, instance)
    deployment = tomllib.loads((DEPLOY / f"relayer-{instance}.toml").read_text())
    assert deployment["app"] == f"paxeer-bridge-relayer-{instance}"
    assert deployment["build"]["dockerfile"] == "docker/bridge-relayer/Dockerfile"
    environment = deployment["env"]
    assert environment["LAYERX_BRIDGE_ATTESTOR_INDEX"] == str(instance)
    assert environment["LAYERX_BRIDGE_ATTESTOR_SET_SIZE"] == "5"
    assert environment["LAYERX_BRIDGE_ATTESTOR_THRESHOLD"] == "3"
    mounts = {item["secret_name"]: item["guest_path"] for item in deployment["files"]}
    assert mounts[name] == "/etc/layerx/bridge-signer/keys.json"
    attestors = [key for key in inventory["keys"] if key["handle"] == "bridge/attestor"]
    assert len(attestors) == 1
    assert attestors[0]["key_file"] == environment["LAYERX_BRIDGE_ATTESTOR_KEY_FILE"]
    assert attestors[0]["key_file"] == mounts["BRIDGE_SIGNER_ATTESTOR_KEY"]
    assert set(key["key_file"] for key in inventory["keys"]) == {
        path for path in mounts.values() if path.startswith("/run/secrets/bridge/signer/")
    }
    template = json.loads((DEPLOY / "signer-keys.json").read_text())
    assert_inventory_policy(template, 1)
    template["keys"][0]["key_file"] = attestors[0]["key_file"]
    assert inventory == template


def test_attestor_inventory_path_entrypoint_guards_and_signer_consumer():
    syntax = subprocess.run(
        ["bash", "-n", str(ENTRYPOINT)], capture_output=True, text=True, check=False,
    )
    assert syntax.returncode == 0, syntax.stderr
    shell = ENTRYPOINT.read_text()
    index_guard = '[ "$attestor_key" = "$secrets/signer/attestor-$index.key" ] ||'
    inventory_guard = (
        'jq -e --arg key "$attestor_key" '
        "'[.keys[] | select(.handle == \"bridge/attestor\")] | "
        "length == 1 and .[0].key_file == $key'"
    )
    for fact in (
        "set -eu", "secrets=/run/secrets/bridge",
        'signer_keys=/etc/layerx/bridge-signer/keys.json',
        'index=$LAYERX_BRIDGE_ATTESTOR_INDEX',
        '[ "$index" -le "$set_size" ] || fail',
        '[ $((threshold * 2)) -gt "$set_size" ] && [ "$threshold" -le "$set_size" ] ||',
        index_guard, 'fail "attestor key mount does not match attestor index $index"',
        '[ -n "$attestor_key" ] && [ -f "$attestor_key" ] || fail',
        '[ -f "$signer_keys" ] || fail', inventory_guard,
        '"$signer_keys" >/dev/null || fail "signer key manifest does not name',
        '/usr/local/bin/layerx-mirror-signer bridge --socket "$signer_socket" &',
    ):
        assert fact in shell, fact
    assert 'fail() {\n\techo "bridge-relayer-entrypoint: $*" >&2\n\texit 1\n}' in shell
    assert shell.index(index_guard) < shell.index(inventory_guard)
    assert shell.index(inventory_guard) < shell.index('/usr/local/bin/layerx-mirror-signer bridge')
    dockerfile = (ROOT / "docker/bridge-relayer/Dockerfile").read_text()
    assert (
        "COPY services/interop/deploy/bridge-relayer/signer-keys.json "
        "/etc/layerx/bridge-signer/keys.json"
    ) in dockerfile.splitlines()
    assert 'ENTRYPOINT ["/usr/local/bin/bridge-relayer-entrypoint"]' in dockerfile.splitlines()
    consumer = (ROOT / "services/interop/crates/layerx-mirror-signer/src/lib.rs").read_text()
    assert 'pub const DEFAULT_BRIDGE_KEYS_FILE: &str = "/etc/layerx/bridge-signer/keys.json";' in consumer
    assert 'keys_file: PathBuf::from(DEFAULT_BRIDGE_KEYS_FILE)' in consumer
    assert 'pub fn load_bridge(options: &BridgeOptions)' in consumer
    assert 'must hold exactly handle, algorithm, key_file and domains' in consumer
    assert 'root.get("keys").and_then(serde_json::Value::as_array)' in consumer
    assert 'fields.get("domains").and_then(serde_json::Value::as_array)' in consumer
    assert 'load_ed25519(key_file)?' in consumer
    assert 'load_secp256k1(key_file)?' in consumer


@pytest.mark.parametrize("field,value", [
    ("LAYERX_BRIDGE_ATTESTOR_INDEX", "2"),
    ("LAYERX_BRIDGE_ATTESTOR_SET_SIZE", "4"),
    ("LAYERX_BRIDGE_ATTESTOR_THRESHOLD", "2"),
    ("LAYERX_BRIDGE_ATTESTOR_KEY_FILE", "/run/secrets/bridge/signer/attestor-2.key"),
])
def test_attestor_inventory_path_refuses_environment_drift(field, value):
    deployment = RENDERER.load_deployment(1)
    deployment["env"][field] = value
    with pytest.raises(ValueError, match=field):
        RENDERER.render_inventory(1, deployment)


@pytest.mark.parametrize("secret,path", [
    ("BRIDGE_SIGNER_ATTESTOR_KEY", "/run/secrets/bridge/signer/attestor-2.key"),
    ("BRIDGE_SIGNER_KEYS", "/etc/layerx/bridge-signer/wrong.json"),
    ("BRIDGE_SIGNER_PAXEER_SUBMITTER_KEY", "/run/secrets/bridge/signer/wrong.key"),
])
def test_attestor_inventory_path_refuses_mount_drift(secret, path):
    deployment = RENDERER.load_deployment(1)
    for item in deployment["files"]:
        if item["secret_name"] == secret:
            item["guest_path"] = path
    with pytest.raises(ValueError):
        RENDERER.render_inventory(1, deployment)


def test_attestor_inventory_path_refuses_missing_duplicate_and_cross_instance_mounts():
    deployment = RENDERER.load_deployment(1)
    missing = copy.deepcopy(deployment)
    missing["files"] = [
        item for item in missing["files"] if item["secret_name"] != "BRIDGE_SIGNER_ATTESTOR_KEY"
    ]
    duplicate = copy.deepcopy(deployment)
    duplicate["files"].append(copy.deepcopy(duplicate["files"][0]))
    duplicate_path = copy.deepcopy(deployment)
    duplicate_path["files"][0]["guest_path"] = duplicate_path["files"][1]["guest_path"]
    for invalid in (missing, duplicate, duplicate_path, RENDERER.load_deployment(2)):
        with pytest.raises(ValueError):
            RENDERER.render_inventory(1, invalid)


@pytest.mark.parametrize("instance", [0, 6, -1, True, "1"])
def test_attestor_inventory_path_refuses_invalid_instance(instance):
    with pytest.raises(ValueError, match="instance"):
        RENDERER.render_inventory(instance)
    with pytest.raises(ValueError, match="instance"):
        RENDERER.load_deployment(instance)


def test_attestor_inventory_path_reads_public_files_only():
    opened = []
    active = True

    def audit(event, arguments):
        if active and event == "open":
            opened.append(Path(arguments[0]).resolve())

    sys.addaudithook(audit)
    try:
        for instance in range(1, 6):
            assert_inventory_policy(RENDERER.render_inventory(instance), instance)
    finally:
        active = False
    assert set(opened) == {
        DEPLOY / "signer-keys.json",
        *(DEPLOY / f"relayer-{instance}.toml" for instance in range(1, 6)),
    }
    assert len(opened) == 10


def test_attestor_inventory_path_cli_requires_explicit_instance(capsys):
    for arguments in ([], ["--instance", "0"], ["--instance", "6"]):
        with pytest.raises(SystemExit) as stopped:
            RENDERER.main(arguments)
        assert stopped.value.code == 2
        output = capsys.readouterr()
        assert output.out == ""
        assert "error:" in output.err
