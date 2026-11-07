"""Production provisioning and source contracts for merged gateway receipt keys."""

import base64
import copy
import json
from pathlib import Path
import re
import stat

import pytest
import yaml

ROOT = Path(__file__).resolve().parents[3]
GATEWAY = ROOT / "services/platform/hosted/gateway"
PRODUCER = ROOT / "services/platform/hosted/tests/sequencer-pins.py"
DOCUMENTS = list(yaml.safe_load_all((GATEWAY / "deployment.yaml").read_text()))
DEPLOYMENT = next(
    item for item in DOCUMENTS
    if item["kind"] == "Deployment" and item["metadata"]["name"] == "layerx-gateway"
)
POD = DEPLOYMENT["spec"]["template"]["spec"]
CONTAINER = next(item for item in POD["containers"] if item["name"] == "gateway")
ENV = {
    item["name"]: (
        item["value"] if "value" in item else
        item["valueFrom"]["configMapKeyRef"]
        if "configMapKeyRef" in item["valueFrom"] else
        item["valueFrom"]["secretKeyRef"]
    )
    for item in CONTAINER["env"]
}
RECORDED = json.loads(
    (GATEWAY / "tests/fixtures/repair_22_33_1335_committed.json").read_text()
)


def producer():
    """Load the real existing producer, without its command-line dispatcher."""
    namespace = {"__name__": "gateway_receipt_key_provisioning"}
    exec(compile(PRODUCER.read_text(), str(PRODUCER), "exec"), namespace)
    return namespace


def expected_config(keys):
    return [
        {
            "keyId": base64.b64encode(bytes.fromhex(key["keyId"][2:])).decode("ascii"),
            "publicKey": base64.b64encode(bytes.fromhex(key["publicKey"][2:])).decode("ascii"),
            "firstHeight": key["firstHeight"],
            "lastHeight": key["lastHeight"],
        }
        for key in keys
    ]


def source_material(tmp_path, keys=None):
    source = tmp_path / "governed.json"
    source.write_text(json.dumps(expected_config(
        keys if keys is not None else [RECORDED["pinnedKey"]]
    )))
    destination = tmp_path / "retained"
    destination.mkdir()
    return source, destination / "receipt-keys.json"


def projected_bytes(secret):
    """Interpret the exact Secret item selected by the production pod volume."""
    volume = next(item for item in POD["volumes"] if item["name"] == "receipt-keys")
    projection = volume["secret"]
    assert projection["secretName"] == secret["metadata"]["name"]
    assert secret["metadata"]["namespace"] == DEPLOYMENT["metadata"]["namespace"]
    assert projection["items"] == [{"key": "receipt-keys.json", "path": "receipt-keys.json"}]
    assert set(secret["data"]) == {"receipt-keys.json"}
    mount = next(item for item in CONTAINER["volumeMounts"] if item["name"] == "receipt-keys")
    assert str(Path(mount["mountPath"]) / projection["items"][0]["path"]) == ENV[
        "LAYERX_GATEWAY_RECEIPT_KEYS_FILE"
    ]
    return base64.b64decode(secret["data"][projection["items"][0]["key"]], validate=True)


def test_gateway_retired_dependencies_removed_from_manifest_and_example():
    main = (GATEWAY / "src/main.rs").read_text()
    kernel_names = set(re.findall(
        r'"(LAYERX_GATEWAY_[A-Z0-9_]+)"',
        main.split("const KERNEL_VARIABLES:", 1)[1].split("];", 1)[0],
    ))
    assert len(kernel_names) == 8
    assert kernel_names == {
        "LAYERX_GATEWAY_COMPONENT_URL", "LAYERX_GATEWAY_COMPONENT_TOKEN_FILE",
        "LAYERX_GATEWAY_PUBLIC_CORE_URL", "LAYERX_GATEWAY_AUTHORITY_URL",
        "LAYERX_GATEWAY_AUTHORITY_TOKEN_FILE", "LAYERX_GATEWAY_PROGRAM_REGISTRY_URL",
        "LAYERX_GATEWAY_PROGRAM_REGISTRY_TOKEN_FILE", "LAYERX_GATEWAY_MODULE_REGISTRY_FILE",
    }
    assert not kernel_names.intersection(ENV)
    assert not any(name.startswith("LAYERX_GATEWAY_SEQUENCER_") for name in ENV)
    text = (GATEWAY / "deployment.yaml").read_text()
    for retired in (
        "layerx-gateway-authority-client", "layerx-receipt-authority",
        "layerx-pending-core", "sequencer-first-batch", "sequencer-last-batch",
    ):
        assert retired not in text
    volumes = {volume["name"]: volume for volume in POD["volumes"]}
    assert not {
        "authority", "component", "identity", "identity-provisioning",
        "registry", "provisioning", "modules",
    }.intersection(volumes)
    kernel = main.split("fn configured_kernel()", 1)[1].split(
        "fn configured_service_identity", 1
    )[0]
    assert 'if env::var_os("LAYERX_GATEWAY_COMPONENT_URL").is_none()' in kernel
    assert "return Ok(None);" in kernel
    assert 'let service_identity = configured_service_identity()?;' in main
    assert 'if kernel.is_some() && service_identity.is_none()' in main
    assert 'return Err("gateway kernel client identity is required".into());' in main
    assert "Some(identity) => (Client::new(ca.clone(), identity), true)" in main
    assert "None => (Client::without_identity(ca.clone()), false)" in main
    example = dict(
        line.split("=", 1)
        for line in (GATEWAY / "railway.env.example").read_text().splitlines()
        if line and not line.startswith("#")
    )
    assert not kernel_names.intersection(example)
    assert not {
        "ENDPOINT_COMPONENT_TOKEN", "ENDPOINT_AUTHORITY_TOKEN", "ENDPOINT_IDENTITY_TOKEN",
        "ENDPOINT_IDENTITY_PROVISIONING_TOKEN", "ENDPOINT_PROGRAM_REGISTRY_TOKEN",
        "ENDPOINT_KEY_PROVISIONING_KEY", "ENDPOINT_MODULE_REGISTRY",
    }.intersection(example)
    for variables in (ENV, example):
        assert variables["LAYERX_GATEWAY_NETWORK_ID"] == "paxeer-mainnet-beta"
        assert variables["LAYERX_GATEWAY_PROTOCOL_NETWORK_ID"] == "125"
        assert variables["LAYERX_GATEWAY_LXP_WIRE_VERSION"] == "3"
    for suffix in ("PKCS12", "PASSWORD_FILE"):
        name = "LAYERX_GATEWAY_SERVICE_CLIENT_IDENTITY_" + suffix
        assert name in ENV
        assert name not in example


def test_receipt_keys_producer_projection_and_real_consumer_contract():
    node = (ROOT / "rpc/sequencer_receipt.go").read_text()
    for name in ("keyId", "publicKey", "firstHeight", "lastHeight", "keys"):
        assert f'`json:"{name}"`' in node
    assert "keys, err := a.receipts.receiptKeys()" in node
    assert "KeyID: common.BytesToHash(key.KeyId), PublicKey: bytes.Clone(key.PublicKey)" in node
    assert "FirstHeight: key.FirstHeight, LastHeight: key.LastHeight" in node
    assert "chain" in RECORDED["provenance"]["recording"]
    pin = RECORDED["pinnedKey"]
    assert pin["keyId"] == RECORDED["receipt"]["keyId"]
    assert pin["publicKey"] == RECORDED["receipt"]["signingKey"]
    assert pin["firstHeight"] <= int(RECORDED["receipt"]["height"], 16) <= pin["lastHeight"]

    rendered = copy.deepcopy(POD)
    container = next(item for item in rendered["containers"] if item["name"] == "gateway")
    # The maintained renderer must accept this template unchanged, including its
    # exact receipt-key environment, Secret volume and read-only mount.
    producer()["gateway_manifest"](rendered, container)
    assert rendered == POD
    volume = next(item for item in POD["volumes"] if item["name"] == "receipt-keys")
    projection = volume["secret"]
    assert projection["secretName"] == "layerx-gateway-receipt-keys"
    assert not projection.get("optional", False)
    assert projection["defaultMode"] == 0o440
    mount = next(item for item in CONTAINER["volumeMounts"] if item["name"] == "receipt-keys")
    assert mount["readOnly"] is True
    assert POD["securityContext"]["runAsNonRoot"] is True
    assert POD["securityContext"]["runAsUser"] == POD["securityContext"]["fsGroup"] == 4020
    main = (GATEWAY / "src/main.rs").read_text()
    assert 'env::var_os("LAYERX_GATEWAY_RECEIPT_KEYS_FILE")' in main
    assert "ReceiptKeyList::from_config(&receipt_key_document)" in main
    library = (GATEWAY / "src/lib.rs").read_text()
    for name in ("keyId", "publicKey", "firstHeight", "lastHeight"):
        assert f'#[serde(rename = "{name}"' in library
    assert "base64::decode(&entry.key_id)" in library
    assert "base64::decode(&entry.public_key)" in library
    assert "VerifyingKey::from_bytes(&bytes)" in library
    assert "key.is_weak()" in library
    assert "ReceiptKeys::HeightRanges(self.0.as_slice())" in library
    reader = (GATEWAY / "src/chain_reads.rs").read_text()
    assert 'return unavailable(id, "receipt_keys_unavailable")' in reader
    for field, method in (
        ("keyId", "sequencer_id"), ("publicKey", "public_key"),
        ("firstHeight", "first_batch_number"), ("lastHeight", "last_batch_number"),
    ):
        assert f'"{field}":' in reader and f"key.{method}()" in reader


def test_fresh_receipt_material_uses_recorded_chain_pin_without_retired_services(tmp_path):
    source, target = source_material(tmp_path)
    before = source.read_bytes()
    functions = producer()
    functions["provision_gateway"](source, target)
    secret = functions["gateway_secret"](target.parent, "layerx-mainnet-beta")
    assert secret["kind"] == "Secret" and secret["type"] == "Opaque"
    transported = projected_bytes(secret)
    assert transported == target.read_bytes()
    assert json.loads(transported) == expected_config([RECORDED["pinnedKey"]])
    assert stat.S_IMODE(target.stat().st_mode) == 0o600
    assert source.read_bytes() == before
    assert list(target.parent.iterdir()) == [target]
    assert len(transported) <= 256 * 1024


def test_retained_material_reopens_without_retired_inputs(tmp_path):
    source, target = source_material(tmp_path)
    functions = producer()
    functions["provision_gateway"](source, target)
    secret = functions["gateway_secret"](target.parent, "layerx-mainnet-beta")
    retained = target.read_bytes()
    del functions
    source.unlink()
    # End the first invocation and reopen the real publisher/validator using
    # only its retained export, without the original source or retired files.
    reopened = producer()
    assert reopened["receipt_keys"](target) == retained
    assert reopened["gateway_secret"](target.parent, "layerx-mainnet-beta") == secret
    assert projected_bytes(secret) == retained
    replacement = tmp_path / "replacement"
    replacement.mkdir()
    replacement_file = replacement / "receipt-keys.json"
    reopened["provision_gateway"](target, replacement_file)
    assert replacement_file.read_bytes() == retained
    assert projected_bytes(reopened["gateway_secret"](replacement, "layerx-mainnet-beta")) == retained
    with target.open("rb") as stream:
        assert stream.read() == retained
    assert stat.S_IMODE(target.stat().st_mode) == 0o600
    assert {path.name for path in target.parent.iterdir()} == {"receipt-keys.json"}
    stateful = next(item for item in DOCUMENTS if item["kind"] == "StatefulSet")
    assert stateful["spec"]["volumeClaimTemplates"][0]["metadata"]["name"] == "data"
    redis = next(item for item in DOCUMENTS if item["kind"] == "ConfigMap")
    assert "appendonly yes\n" in redis["data"]["redis.conf"]
    assert "appendfsync always\n" in redis["data"]["redis.conf"]


@pytest.mark.parametrize("field,value", [
    ("keyId", base64.b64encode(bytes(32)).decode()), ("keyId", "AA=="),
    ("publicKey", base64.b64encode(bytes(32)).decode()), ("publicKey", "not-base64"),
    ("firstHeight", -1), ("firstHeight", True), ("firstHeight", "10"),
    ("firstHeight", 0), ("firstHeight", 11), ("lastHeight", 1 << 64), ("lastHeight", 9),
])
def test_invalid_pin_refuses_fresh_and_retained_material(tmp_path, field, value):
    source, target = source_material(tmp_path)
    producer()["provision_gateway"](source, target)
    retained = target.read_bytes()
    changed = {**expected_config([RECORDED["pinnedKey"]])[0], field: value}
    source.write_text(json.dumps([changed]))
    with pytest.raises(ValueError):
        producer()["provision_gateway"](source, target)
    assert target.read_bytes() == retained
    target.unlink()
    with pytest.raises(ValueError):
        producer()["provision_gateway"](source, target)
    assert not target.exists()
    assert list(target.parent.iterdir()) == []


@pytest.mark.parametrize("raw", [
    b'[]', b'null', b'{}', b'{"keys":[]}', b'[{}]',
    b'[{"keyId":"x","keyId":"y"}]', b'[', b' ' * (256 * 1024 + 1),
])
def test_malformed_or_empty_pinned_snapshot_never_publishes(tmp_path, raw):
    source, target = source_material(tmp_path)
    source.write_bytes(raw)
    with pytest.raises(ValueError):
        producer()["provision_gateway"](source, target)
    assert not target.exists()


def test_duplicate_key_ids_and_unknown_fields_refuse_without_publication(tmp_path):
    source, target = source_material(tmp_path)
    pin = expected_config([RECORDED["pinnedKey"]])[0]
    for keys in ([pin, pin], [{**pin, "batchAuthority": "retired"}], [{"keyId": pin["keyId"]}]):
        source.write_text(json.dumps(keys))
        with pytest.raises(ValueError):
            producer()["provision_gateway"](source, target)
        assert not target.exists()


def test_wrong_shape_and_missing_material_fail_without_retained_fallback(tmp_path):
    source, target = source_material(tmp_path)
    producer()["provision_gateway"](source, target)
    retained = target.read_bytes()
    source.write_text(json.dumps({"keys": [RECORDED["pinnedKey"]]}))
    with pytest.raises(ValueError):
        producer()["provision_gateway"](source, target)
    assert target.read_bytes() == retained
    source.unlink()
    with pytest.raises(FileNotFoundError):
        producer()["provision_gateway"](source, target)
    assert target.read_bytes() == retained


def test_atomic_rotation_keeps_exact_height_windows_and_protected_output(tmp_path):
    source, target = source_material(tmp_path)
    producer()["provision_gateway"](source, target)
    # Exercise the uint64 boundary without inventing a signing key or changing
    # the real recorded pin. This is an input-shape test, not governance proof.
    keys = copy.deepcopy([RECORDED["pinnedKey"]])
    keys[0]["lastHeight"] = (1 << 64) - 1
    source.write_text(json.dumps(expected_config(keys)))
    producer()["provision_gateway"](source, target)
    assert json.loads(target.read_bytes()) == expected_config(keys)
    assert stat.S_IMODE(target.stat().st_mode) == 0o600
    assert list(target.parent.iterdir()) == [target]
    assert projected_bytes(producer()["gateway_secret"](target.parent, "layerx-mainnet-beta")) == target.read_bytes()


def test_unsafe_retained_output_is_refused(tmp_path):
    source, target = source_material(tmp_path)
    unrelated = tmp_path / "unrelated"
    unrelated.write_bytes(b"preserve this file")
    target.symlink_to(unrelated)
    with pytest.raises(ValueError, match="symlink"):
        producer()["provision_gateway"](source, target)
    assert unrelated.read_bytes() == b"preserve this file"
    source_link = tmp_path / "source-link"
    source_link.symlink_to(source)
    target.unlink()
    with pytest.raises(OSError):
        producer()["provision_gateway"](source_link, target)
    assert not target.exists()


def test_identifier_meanings_and_proof_limits_are_published():
    assert all(item["metadata"]["namespace"] == "layerx-mainnet-beta" for item in DOCUMENTS)
    documentation = (GATEWAY / "README.md").read_text()
    for fact in (
        "Kubernetes namespace `layerx-testnet`", "EVM chain ID `125` (`0x7d`)",
        "LAYERX_GATEWAY_PROTOCOL_NETWORK_ID=125", "LAYERX_GATEWAY_NETWORK_ID=paxeer-mainnet-beta",
        "LAYERX_GATEWAY_LXP_WIRE_VERSION=3", "inclusive uint64\nchain heights",
        "A chain-ID label alone is not authentication", "Key rotation requires",
        "It does not launch the Rust", "material provisioning",
    ):
        assert fact in documentation
    record = (ROOT / "tools/qualification/audit_repairs/repair_22_33_1395.kvx").read_text()
    assert 'id = "gateway-retired-dependencies"' in record
    assert "test_retained_material_reopens_without_retired_inputs" in record
    assert "test_fresh_receipt_material_uses_recorded_chain_pin_without_retired_services" in record
    assert "wiring_request" in record
    assert "Not run" in record and "No baseline failures are claimed" in record
