import base64
import json
import os
from pathlib import Path
import stat
import struct
import sys
import tempfile


RECEIPT_KEYS_FILE = "receipt-keys.json"
RECEIPT_KEYS_SECRET = "layerx-gateway-receipt-keys"
RECEIPT_KEYS_MOUNT = "/run/layerx/receipt-keys"
MAX_RECEIPT_KEYS_BYTES = 256 * 1024


def unique_fields(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise ValueError("duplicate receipt key field")
        result[key] = value
    return result


def receipt_keys(path):
    descriptor = os.open(path, os.O_RDONLY | os.O_NOFOLLOW)
    with os.fdopen(descriptor, "rb") as source:
        if not stat.S_ISREG(os.fstat(source.fileno()).st_mode):
            raise ValueError("receipt keys must be a regular file")
        document = source.read(MAX_RECEIPT_KEYS_BYTES + 1)
    if len(document) > MAX_RECEIPT_KEYS_BYTES:
        raise ValueError("receipt key list exceeds gateway limit")
    entries = json.loads(document, object_pairs_hook=unique_fields)
    if not isinstance(entries, list) or not entries:
        raise ValueError("receipt key list must be nonempty")
    normalized, seen = [], set()
    for entry in entries:
        if not isinstance(entry, dict):
            raise ValueError("invalid receipt key entry")
        snake = {"key_id", "public_key", "first_height", "last_height"}
        camel = {"keyId", "publicKey", "firstHeight", "lastHeight"}
        if set(entry) == snake:
            entry = dict(zip(("keyId", "publicKey", "firstHeight", "lastHeight"),
                             (entry[name] for name in
                              ("key_id", "public_key", "first_height", "last_height"))))
        elif set(entry) != camel:
            raise ValueError("invalid receipt key fields")
        for name in ("keyId", "publicKey"):
            value = entry[name]
            if not isinstance(value, str):
                raise ValueError("invalid receipt key bytes")
            raw = base64.b64decode(value, validate=True)
            if len(raw) != 32 or not any(raw) or base64.b64encode(raw).decode() != value:
                raise ValueError("invalid receipt key bytes")
        if entry["keyId"] in seen:
            raise ValueError("duplicate receipt key ID")
        seen.add(entry["keyId"])
        first, last = entry["firstHeight"], entry["lastHeight"]
        if (type(first) is not int or type(last) is not int
                or not 0 < first <= last <= (1 << 64) - 1):
            raise ValueError("invalid inclusive receipt key height range")
        normalized.append(entry)
    encoded = (json.dumps(normalized, separators=(",", ":")) + "\n").encode()
    if len(encoded) > MAX_RECEIPT_KEYS_BYTES:
        raise ValueError("receipt key list exceeds gateway limit")
    return encoded


def provision_gateway(source, output):
    # The source is an owner-supplied export of governed ReceiptKeys, never
    # the webhook trust history or a public key with an invented height range.
    document = receipt_keys(source)
    if output.is_symlink():
        raise ValueError("receipt key destination must not be a symlink")
    descriptor, temporary = tempfile.mkstemp(prefix=".receipt-keys-", dir=output.parent)
    try:
        with os.fdopen(descriptor, "wb") as destination:
            destination.write(document)
            destination.flush()
            os.fsync(destination.fileno())
        os.replace(temporary, output)
        directory = os.open(output.parent, os.O_RDONLY | os.O_DIRECTORY)
        try:
            os.fsync(directory)
        finally:
            os.close(directory)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)


def gateway_secret(directory, namespace):
    document = receipt_keys(directory / RECEIPT_KEYS_FILE)
    return {"apiVersion": "v1", "kind": "Secret", "type": "Opaque",
            "metadata": {"name": RECEIPT_KEYS_SECRET, "namespace": namespace},
            "data": {RECEIPT_KEYS_FILE: base64.b64encode(document).decode()}}


def gateway_manifest(pod, container):
    legacy = {"LAYERX_GATEWAY_SEQUENCER_" + suffix + "_FILE"
              for suffix in ("PUBLIC_KEY", "ID", "FIRST_BATCH", "LAST_BATCH")}
    container["env"] = [entry for entry in container.get("env", [])
                        if entry["name"] not in legacy]
    env = {"name": "LAYERX_GATEWAY_RECEIPT_KEYS_FILE",
           "value": RECEIPT_KEYS_MOUNT + "/" + RECEIPT_KEYS_FILE}
    mount = {"name": "receipt-keys", "mountPath": RECEIPT_KEYS_MOUNT, "readOnly": True}
    volume = {"name": "receipt-keys", "secret": {
        "secretName": RECEIPT_KEYS_SECRET, "defaultMode": 0o440,
        "items": [{"key": RECEIPT_KEYS_FILE, "path": RECEIPT_KEYS_FILE}]}}
    for entries, item in ((container["env"], env),
                          (container.setdefault("volumeMounts", []), mount),
                          (pod.setdefault("volumes", []), volume)):
        existing = [entry for entry in entries if entry["name"] == item["name"]]
        if existing:
            if existing != [item]:
                raise ValueError("conflicting gateway receipt key projection")
        else:
            entries.append(item)


def provision(directory, output):
    history = (directory / "trust-history").read_bytes()
    domain = b"LayerX/sequencer-trust-history/v1\0"
    assert history.startswith(domain) and len(history) == len(domain) + 4 + 103
    assert struct.unpack_from(">HH", history, len(domain)) == (1, 0)
    entry = history[len(domain) + 4:]
    version, network, epoch = struct.unpack_from(">HIQ", entry)
    assert version == 3 and network > 0 and epoch == 1
    sequencer_id, public_key = entry[14:46].hex(), entry[46:78].hex()
    first, last, revoked, revoked_at = struct.unpack_from(">QQBQ", entry, 78)
    assert 0 < first <= last and revoked == 0 and revoked_at == 0
    assert public_key == (directory / "sequencer-public-key").read_text()
    pins = dict(sequencer_id=sequencer_id, sequencer_public_key=public_key,
                sequencer_first_batch=str(first), sequencer_last_batch=str(last))
    for key in ("sequencer_id", "sequencer_first_batch", "sequencer_last_batch"):
        (directory / key.replace("_", "-")).write_text(pins[key])
    output.write_text(json.dumps({"layerx": pins}, indent=2) + "\n")


def manifests(directory):
    import yaml

    for filename, name, prefix, mount in (
            ("gateway.yaml", "layerx-gateway", "LAYERX_GATEWAY", "/run/layerx/authority"),
            ("developer.yaml", "layerx-webhooks", "LAYERX_WEBHOOKS", "/run/layerx/keys")):
        path = directory / filename
        documents = list(yaml.safe_load_all(path.read_text()))
        workload = next(d for d in documents if d["kind"] == "Deployment" and d["metadata"]["name"] == name)
        pod = workload["spec"]["template"]["spec"]
        container = next(c for c in pod["containers"] if c["name"] == name.removeprefix("layerx-"))
        if name == "layerx-gateway":
            gateway_manifest(pod, container)
            path.write_text(yaml.safe_dump_all(documents, sort_keys=False))
            continue
        for suffix in ("ID", "FIRST_BATCH", "LAST_BATCH"):
            env_name = prefix + "_SEQUENCER_" + suffix + "_FILE"
            value = mount + "/sequencer-" + suffix.lower().replace("_", "-")
            existing = [e for e in container["env"] if e["name"] == env_name]
            if existing:
                assert existing == [{"name": env_name, "value": value}]
            else:
                container["env"].append({"name": env_name, "value": value})
        if name == "layerx-webhooks":
            volume = next(v for v in pod["volumes"] if v["name"] == "runtime")
            secret = next(s["secret"] for s in volume["projected"]["sources"]
                          if s.get("secret", {}).get("name") == "layerx-developer-hosted-runtime")
            for suffix in ("id", "first-batch", "last-batch"):
                item = {"key": "sequencer-" + suffix, "path": "keys/sequencer-" + suffix}
                if item not in secret["items"]:
                    assert not any(i["key"] == item["key"] or i["path"] == item["path"] for i in secret["items"])
                    secret["items"].append(item)
        path.write_text(yaml.safe_dump_all(documents, sort_keys=False))


if __name__ == "__main__":
    if sys.argv[1] == "--manifests":
        manifests(Path(sys.argv[2]))
    elif sys.argv[1] == "--gateway":
        provision_gateway(Path(sys.argv[2]), Path(sys.argv[3]))
    elif sys.argv[1] == "--gateway-check":
        receipt_keys(Path(sys.argv[2]))
    elif sys.argv[1] == "--gateway-secret":
        print(json.dumps(gateway_secret(Path(sys.argv[2]), sys.argv[3])))
    else:
        provision(Path(sys.argv[1]), Path(sys.argv[2]))
