import hashlib
import importlib.util
import json
from pathlib import Path
import struct
import subprocess

import pytest


DIRECTORY = Path(__file__).resolve().parent
DOMAIN = b"LayerX/sequencer-trust-history/v1\0"
SPEC = importlib.util.spec_from_file_location("sequencer_pins", DIRECTORY / "sequencer-pins.py")
assert SPEC is not None and SPEC.loader is not None
PINS = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(PINS)


@pytest.fixture
def generated(tmp_path):
    key = subprocess.run(["openssl", "genpkey", "-algorithm", "ed25519"],
                         check=True, capture_output=True).stdout
    public = subprocess.run(["openssl", "pkey", "-pubout", "-outform", "DER"],
                            input=key, check=True, capture_output=True).stdout[-32:]
    identity = hashlib.sha256(b"layerx-sequencer:" + public.hex().encode()).hexdigest()
    history = tmp_path / "trust-history"
    subprocess.run(["bash", "-c", 'source "$1"; encode_trust_history "$2" "$3" "$4"',
                    "trust-test", str(DIRECTORY / "beta-cluster.sh"), str(history),
                    identity, public.hex()], check=True, capture_output=True)
    (tmp_path / "sequencer-public-key").write_text(public.hex())
    return tmp_path, identity, public.hex()


def test_generated_protocol_three_history_provisions_exact_pins(generated):
    directory, identity, public = generated
    history = (directory / "trust-history").read_bytes()
    version, network, epoch = struct.unpack_from(">HIQ", history, len(DOMAIN) + 4)
    assert version == 3 and network > 0 and epoch == 1
    output = directory / "authorization.json"
    PINS.provision(directory, output)
    assert json.loads(output.read_text()) == {"layerx": {
        "sequencer_id": identity, "sequencer_public_key": public,
        "sequencer_first_batch": "1", "sequencer_last_batch": str(1 << 40)}}
    assert (directory / "sequencer-id").read_text() == identity
    assert (directory / "sequencer-first-batch").read_text() == "1"
    assert (directory / "sequencer-last-batch").read_text() == str(1 << 40)


@pytest.mark.parametrize("version", [0, 1, 2, 4, 65535])
def test_pin_provision_refuses_every_other_protocol(generated, version):
    directory, _, _ = generated
    history = bytearray((directory / "trust-history").read_bytes())
    struct.pack_into(">H", history, len(DOMAIN) + 4, version)
    (directory / "trust-history").write_bytes(history)
    assert_refused(directory)


def assert_refused(directory):
    output = directory / "authorization.json"
    with pytest.raises(AssertionError):
        PINS.provision(directory, output)
    assert not output.exists()
    for name in ("sequencer-id", "sequencer-first-batch", "sequencer-last-batch"):
        assert not (directory / name).exists()


@pytest.mark.parametrize("mutation", ["domain", "count", "trailing", "truncated", "network",
                                     "epoch", "first", "last", "revoked", "revoked_at", "key"])
def test_pin_provision_preserves_history_refusals(generated, mutation):
    directory, _, _ = generated
    history = bytearray((directory / "trust-history").read_bytes())
    start = len(DOMAIN) + 4
    offsets = {"network": (start + 2, ">I", 0), "epoch": (start + 6, ">Q", 0),
               "first": (start + 78, ">Q", 0), "last": (start + 86, ">Q", 0),
               "revoked": (start + 94, ">B", 1), "revoked_at": (start + 95, ">Q", 1)}
    if mutation in offsets:
        offset, encoding, value = offsets[mutation]
        struct.pack_into(encoding, history, offset, value)
    elif mutation == "domain":
        history[0] ^= 1
    elif mutation == "count":
        struct.pack_into(">H", history, len(DOMAIN), 2)
    elif mutation == "trailing":
        history.append(0)
    elif mutation == "truncated":
        history.pop()
    else:
        history[start + 46] ^= 1
    (directory / "trust-history").write_bytes(history)
    assert_refused(directory)


def test_gateway_projection_preserves_current_webhook_contract(tmp_path):
    import yaml

    hosted = DIRECTORY.parent
    original_webhooks = list(yaml.safe_load_all(
        (hosted / "webhooks/deployment.yaml").read_text()))
    (tmp_path / "developer.yaml").write_text(
        (hosted / "webhooks/deployment.yaml").read_text())
    (tmp_path / "gateway.yaml").write_text(
        (hosted / "gateway/deployment.yaml").read_text())
    PINS.manifests(tmp_path)
    rendered_webhooks = list(yaml.safe_load_all((tmp_path / "developer.yaml").read_text()))
    assert rendered_webhooks == original_webhooks
    workload = next(document for document in rendered_webhooks
                    if document["kind"] == "Deployment"
                    and document["metadata"]["name"] == "layerx-webhooks")
    pod = workload["spec"]["template"]["spec"]
    container = next(container for container in pod["containers"]
                     if container["name"] == "webhooks")
    environment = {entry["name"]: entry.get("value") for entry in container["env"]}
    assert environment["LAYERX_WEBHOOKS_SEQUENCER_PUBLIC_KEY_FILE"] == "/run/layerx/keys/sequencer"
    for suffix in ("ID", "FIRST_BATCH", "LAST_BATCH"):
        assert environment["LAYERX_WEBHOOKS_SEQUENCER_" + suffix + "_FILE"] == (
            "/run/layerx/keys/sequencer-" + suffix.lower().replace("_", "-"))
    assert "LAYERX_WEBHOOKS_RECEIPT_KEYS_FILE" not in environment
    runtime = next(volume for volume in pod["volumes"] if volume["name"] == "runtime")
    secret = next(source["secret"] for source in runtime["projected"]["sources"]
                  if source.get("secret", {}).get("name") == "layerx-developer-hosted-runtime")
    for suffix, path in (("public-key", "sequencer"), ("id", "sequencer-id"),
                         ("first-batch", "sequencer-first-batch"),
                         ("last-batch", "sequencer-last-batch")):
        assert {"key": "sequencer-" + suffix, "path": "keys/" + path} in secret["items"]


@pytest.fixture(scope="session")
def native_gateway_consumer(tmp_path_factory):
    directory = tmp_path_factory.mktemp("native-gateway-consumer")
    manifest = directory / "Cargo.toml"
    manifest.write_text(
        '[package]\nname = "receipt-pins-consumer"\nversion = "0.1.0"\nedition = "2021"\n'
        '[workspace]\n[[bin]]\nname = "receipt-pins-consumer"\n'
        f'path = {json.dumps(str(DIRECTORY / "receipt-pins-consumer.rs"))}\n'
        '[dependencies]\nlayerx-platform-gateway = { path = '
        + json.dumps(str(DIRECTORY.parent / "gateway")) + ' }\n')
    subprocess.run(["cargo", "build", "--quiet", "--manifest-path", str(manifest),
                    "--target-dir", str(directory / "target")], check=True)
    return directory / "target/debug/receipt-pins-consumer"


def test_native_gateway_manifest_rotation_reopen_and_height_refusals(tmp_path, native_gateway_consumer):
    import base64
    import os
    import yaml

    corpus = json.loads((DIRECTORY.parents[3] /
                        "sdk/platform/conformance/fixtures/receipt-sequencer-v1.json").read_text())
    primary = next(vector for vector in corpus["vectors"]
                   if vector["id"] == "receipted-primary-key")
    pins = primary["receiptKeys"]
    normalized = [dict(entry, keyId=base64.b64encode(bytes.fromhex(entry["keyId"][2:])).decode(),
                       publicKey=base64.b64encode(bytes.fromhex(entry["publicKey"][2:])).decode())
                  for entry in pins]
    source, retained = tmp_path / "governed.json", tmp_path / PINS.RECEIPT_KEYS_FILE
    source.write_text(json.dumps(normalized[:1]))
    PINS.provision_gateway(source, retained)
    manifests = tmp_path / "manifests"
    manifests.mkdir()
    (manifests / "gateway.yaml").write_text((DIRECTORY.parent / "gateway/deployment.yaml").read_text())
    (manifests / 'developer.yaml').write_text(
        (DIRECTORY.parent / 'webhooks/deployment.yaml').read_text())
    PINS.manifests(manifests)
    documents = list(yaml.safe_load_all((manifests / "gateway.yaml").read_text()))
    pod = next(document for document in documents if document["kind"] == "Deployment"
               and document["metadata"]["name"] == "layerx-gateway")["spec"]["template"]["spec"]
    container = next(container for container in pod["containers"] if container["name"] == "gateway")
    configured = next(entry["value"] for entry in container["env"]
                      if entry["name"] == "LAYERX_GATEWAY_RECEIPT_KEYS_FILE")
    mount = next(mount for mount in container["volumeMounts"] if mount["name"] == "receipt-keys")
    assert mount["readOnly"] and "subPath" not in mount
    volume = next(volume["secret"] for volume in pod["volumes"] if volume["name"] == mount["name"])
    secret = PINS.gateway_secret(tmp_path, "layerx-testnet")
    assert volume["secretName"] == secret["metadata"]["name"]
    assert volume["defaultMode"] == 0o440
    item = next(item for item in volume["items"] if configured == mount["mountPath"] + "/" + item["path"])
    projected = tmp_path / "projection" / item["path"]
    projected.parent.mkdir()

    def project():
        secret = PINS.gateway_secret(tmp_path, "layerx-testnet")
        projected.write_bytes(base64.b64decode(secret["data"][item["key"]], validate=True))

    def consume(commands, count):
        environment = dict(os.environ, LAYERX_GATEWAY_RECEIPT_KEYS_FILE=str(projected))
        result = subprocess.run([str(native_gateway_consumer)], env=environment,
                                input="\n".join(commands) + "\n", text=True,
                                capture_output=True, check=True).stdout.splitlines()
        assert result[0] == f"ready:{count}"
        return result[1:]

    receipt = tmp_path / "primary.receipt"
    receipt.write_bytes(bytes.fromhex(primary["receipt"][2:]))
    project()
    assert consume(["verify " + str(receipt)], 1)[0].startswith("verified:")
    # Persist a governed rotation and restart the actual Rust consumer. The
    # fixture carries signed receipts; headers prove both inclusive windows.
    source.write_text(json.dumps(normalized))
    PINS.provision_gateway(source, retained)
    saved = retained.read_bytes()
    source.unlink()
    project()
    commands = ["verify " + str(receipt)]
    expected = ["verified:"]
    for pin in pins:
        for height, accepted in ((pin["firstHeight"], True), (pin["lastHeight"], True),
                                 (pin["firstHeight"] - 1, False), (pin["lastHeight"] + 1, False)):
            commands.append(f'header {pin["keyId"][2:]} {height}')
            expected.append("authorized" if accepted else "refused:UntrustedSequencer")
    for vector in corpus["vectors"]:
        if vector["receiptKeys"] != pins:
            continue
        path = tmp_path / (vector["id"] + ".receipt")
        path.write_bytes(bytes.fromhex(vector["receipt"][2:]))
        commands.append("verify " + str(path))
        expected.append("verified:" if vector["expected"]["verifier"] == "accepts" else "refused:")
    for actual, expected_prefix in zip(consume(commands, 2), expected, strict=True):
        assert actual.startswith(expected_prefix), (actual, expected_prefix)
    project()
    assert consume(["verify " + str(receipt)], 2)[0].startswith("verified:")
    assert retained.read_bytes() == saved
