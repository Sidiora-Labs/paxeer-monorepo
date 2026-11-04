"""Protected input transport for the connected hosted Programs gate."""

import os
from pathlib import Path
import re
from urllib.parse import urlsplit

from compiler_manifest import private_bytes, private_json, require, write_private


SCHEMA = "paxeer-x.hosted-program-profile.v1"
INPUTS = {
    "LAYERX_PROGRAM_GATEWAY_URL", "LAYERX_GATEWAY_CA_FILE", "LAYERX_GATEWAY_KEY_ID",
    "LAYERX_GATEWAY_KEY_SECRET_FILE", "LAYERX_PROGRAM_CALL_ACTIVITY_FILE",
    "LAYERX_PROGRAM_ID", "LAYERX_PROGRAM_IDEMPOTENCY_KEY", "LAYERX_RECEIPT_VERIFY_BIN",
}


def validate(document, revision):
    require(set(document) == {"schema", "source_revision", "inputs"}
            and document["schema"] == SCHEMA and document["source_revision"] == revision,
            "current private hosted Programs profile required")
    inputs = document["inputs"]
    require(isinstance(inputs, dict) and set(inputs) == INPUTS
            and all(isinstance(value, str) and value for value in inputs.values()),
            "closed connected hosted Programs inputs required")
    origin = urlsplit(inputs["LAYERX_PROGRAM_GATEWAY_URL"])
    require(origin.scheme == "https" and origin.hostname in {"localhost", "127.0.0.1", "::1"}
            and origin.port is not None and origin.path in {"", "/"}
            and not origin.username and not origin.password and not origin.query and not origin.fragment,
            "disposable loopback HTTPS profile required")
    for key in ("LAYERX_PROGRAM_ID", "LAYERX_PROGRAM_IDEMPOTENCY_KEY"):
        require(re.fullmatch("[0-9a-f]{64}", inputs[key]), "canonical hosted Programs identity required")
    require(re.fullmatch("[A-Za-z0-9_-]{1,128}", inputs["LAYERX_GATEWAY_KEY_ID"]),
            "bounded gateway key identity required")
    return dict(inputs)


def publish(path, revision, inputs, root):
    document = {"schema": SCHEMA, "source_revision": revision, "inputs": inputs}
    validate(document, revision)
    path = Path(path)
    require(path.is_absolute() and path.resolve() == path and root not in path.parents,
            "canonical private profile outside source required")
    write_private(path, document)


def load(path, revision, root):
    inputs = validate(private_json(path, root), revision)
    for key in ("LAYERX_GATEWAY_CA_FILE", "LAYERX_GATEWAY_KEY_SECRET_FILE",
                "LAYERX_PROGRAM_CALL_ACTIVITY_FILE", "LAYERX_RECEIPT_VERIFY_BIN"):
        private_bytes(inputs[key], root)
    require(os.access(inputs["LAYERX_RECEIPT_VERIFY_BIN"], os.X_OK),
            "real retained receipt verifier must be executable")
    secret = private_bytes(inputs["LAYERX_GATEWAY_KEY_SECRET_FILE"], root, maximum=4096).decode().strip()
    require(re.fullmatch("[A-Za-z0-9_-]{16,4096}", secret), "bounded gateway key secret required")
    inputs["LAYERX_GATEWAY_KEY_SECRET"] = secret
    return inputs
