"""Compose the production startup documents for the launchpad indexer."""

from __future__ import annotations

import base64
import json
import hashlib
from datetime import datetime, timezone
import re
from pathlib import Path
from typing import Any

from common import FileDocument


class DocumentError(ValueError):
    """The indexer values do not describe a complete startup document set."""


_DIGEST = re.compile(r"^[0-9a-f]{64}$")
_ADDRESS = re.compile(r"^0x[0-9a-fA-F]{40}$")
_TOKEN = re.compile(r"^[A-Za-z0-9_.:-]{1,128}$")


def _values_path(values: Path) -> Path:
    for name in ("indexer.json", "indexer.example.json"):
        path = values / name
        if path.is_file():
            return path
    raise DocumentError("indexer: missing values file indexer.example.json")


def _read_values(values: Path) -> dict[str, Any]:
    path = _values_path(values)
    try:
        raw = json.loads(path.read_bytes())
    except (OSError, json.JSONDecodeError) as exc:
        raise DocumentError(f"indexer: invalid values file {path.name}") from exc
    if not isinstance(raw, dict):
        raise DocumentError("indexer: values file must be an object")
    return raw


def _field(data: dict[str, Any], key: str, kind: type, *, nonempty: bool = True) -> Any:
    value = data.get(key)
    if not isinstance(value, kind) or (nonempty and isinstance(value, str) and not value.strip()):
        raise DocumentError(f"indexer: {key}")
    return value


def _digest(data: dict[str, Any], key: str) -> str:
    value = _field(data, key, str).lower()
    if not _DIGEST.fullmatch(value):
        raise DocumentError(f"indexer: {key}")
    return value


def _address(data: dict[str, Any], key: str) -> str:
    value = _field(data, key, str)
    if not _ADDRESS.fullmatch(value) or value.lower() == "0x" + "0" * 40:
        raise DocumentError(f"indexer: {key}")
    return value


def _token(data: dict[str, Any], key: str) -> str:
    value = _field(data, key, str)
    if not _TOKEN.fullmatch(value):
        raise DocumentError(f"indexer: {key}")
    return value


def _positive(data: dict[str, Any], key: str, maximum: int) -> int:
    value = _field(data, key, int)
    if isinstance(value, bool) or value < 1 or value > maximum:
        raise DocumentError(f"indexer: {key}")
    return value


def _nonnegative(data: dict[str, Any], key: str, maximum: int) -> int:
    value = _field(data, key, int)
    if isinstance(value, bool) or value < 0 or value > maximum:
        raise DocumentError(f"indexer: {key}")
    return value


def _roles(values: dict[str, Any]) -> dict[str, dict[str, str]]:
    addresses = _field(values, "addresses", dict)
    roles: dict[str, dict[str, str]] = {}
    for role, key in (
        ("EventEmitter", "event_emitter"),
        ("PoolRegistry", "pool_registry"),
        ("Router", "router"),
        ("SidioraFactory", "factory"),
        ("Quoter", "quoter"),
        ("ProtocolConfig", "protocol_config"),
        ("FeeAccumulator", "fee_accumulator"),
        ("SidioraNFT", "sidiora_nft"),
        ("FeesRouter", "fees_router"),
        ("PoolBeacon", "pool_beacon"),
    ):
        address = _address(addresses, key)
        roles[role] = {"address": address, "kind": "surface" if role not in {"EventEmitter"} else "erc1967"}
    return roles


def _receipt(values: dict[str, Any], addresses: dict[str, dict[str, str]]) -> dict[str, Any]:
    realm = _field(values, "realm", str)
    if realm != "production":
        raise DocumentError("indexer: realm")
    anchor_height = _nonnegative(values, "anchor_height", 2**63 - 1)
    anchor_hash = _field(values, "anchor_hash", str).lower()
    if not re.fullmatch(r"0x[0-9a-f]{64}", anchor_hash):
        raise DocumentError("indexer: anchor_hash")
    deployment = _token(values, "deployment_id")
    manifest_digest = _digest(values, "manifest_digest")
    timeout_ms = _positive(values, "rpc_timeout_ms", 15000)
    catalog = _field(values, "catalog", dict)
    catalog_bytes = json.dumps(catalog, sort_keys=True, separators=(",", ":")).encode("utf-8")
    epochs = []
    for role, identity in sorted(addresses.items()):
        epochs.append(
            {
                "Deployment": {
                    "Address": identity["address"],
                    "Role": role,
                    "ABIRevision": _token(values, "abi_revision"),
                    "From": {"Block": anchor_height + 1},
                },
                "Identity": identity,
            }
        )
    manifest = {
        "schemaVersion": 1,
        "deploymentVersion": deployment,
        "sourceManifestSha256": manifest_digest,
        "chainId": 125,
        "activationHeight": anchor_height + 1,
        "status": "active",
        "roles": addresses,
    }
    if "history_from_height" in values:
        source = Path(__file__).resolve().parents[3] / "sdk/launchpad/shared/config/deployment/current.json"
        try:
            manifest = json.loads(source.read_bytes())
        except (OSError, ValueError) as exc:
            raise DocumentError("indexer: deployment manifest unavailable") from exc
        if manifest["sourceManifestSha256"] != manifest_digest or manifest["deploymentVersion"] != deployment:
            raise DocumentError("indexer: history requires the matching rendered receipt manifest")
        for epoch in epochs:
            epoch["Deployment"]["From"]["Block"] = manifest["activationHeight"]
    production = {
        "Manifest": manifest,
        "CatalogJSON": base64.b64encode(catalog_bytes).decode("ascii"),
        "Epochs": epochs,
        "Discovery": [],
        "Anchor": {"Height": anchor_height, "Hash": anchor_hash},
        "BlockIdentity": "reported",
        "Timeout": timeout_ms * 1_000_000,
    }
    return {"Realm": realm, "Production": production}


def _delivery(values: dict[str, Any]) -> dict[str, Any]:
    lag = _positive(values, "max_delivery_lag_ms", 2**63 // 1_000_000)
    epoch = _positive(values, "epoch", 2**31 - 1)
    schema = _token(values, "delivery_schema")
    owner = _token(values, "owner")
    region = _token(values, "region")
    stream = _token(values, "stream")
    canonical = _token(values, "canonical_stream_id")
    deployment = _token(values, "deployment_id")
    projection = _token(values, "projection_version")
    inventory_projection = _token(values, "inventory_projection_version")
    generation = _token(values, "inventory_generation")
    canonical_event = "canonical_creation"
    canonical_subject = "canonical.creation.v1"
    market = "accounting.market.v1"
    inventory = "accounting.inventory.v1"
    replay = "indexer.replay.v2"
    return {
        "MaxDeliveryLagMS": lag,
        "Schema": schema,
        "Owner": owner,
        "Epoch": epoch,
        "Region": region,
        "Stream": stream,
        "CanonicalStreamID": canonical,
        "DeploymentID": deployment,
        "ProjectionVersion": projection,
        "InventoryProjectionVersion": inventory_projection,
        "Generation": generation,
        "Policy": {
            "Subjects": {canonical_subject: True, market: True, inventory: True, replay: True},
            "EventTypes": {canonical_event: True, market: True, inventory: True, replay: True},
        },
        "Subjects": {canonical_event: canonical_subject, market: market, inventory: inventory},
    }


def _lifecycle(values: dict[str, Any]) -> dict[str, Any]:
    version = _positive(values, "lifecycle_version", 2)
    if version != 2:
        raise DocumentError("indexer: lifecycle_version")
    endpoint = _field(values, "primary_endpoint", str)
    if not (endpoint.startswith("https://") and "/" not in endpoint[8:]):
        raise DocumentError("indexer: primary_endpoint")
    credential = _field(values, "rpc_secret_reference", str)
    if not _TOKEN.fullmatch(credential):
        raise DocumentError("indexer: rpc_secret_reference")
    checkpoint_height = _nonnegative(values, "anchor_height", 2**63 - 1)
    checkpoint_hash = _field(values, "anchor_hash", str).lower()
    if not re.fullmatch(r"0x[0-9a-f]{64}", checkpoint_hash):
        raise DocumentError("indexer: anchor_hash")
    manifest_digest = _digest(values, "manifest_digest")
    return {
        "Version": version,
        "Committed": {
            "Endpoints": {"paxeer-primary": endpoint},
            "Anchor": {"Height": checkpoint_height, "Hash": checkpoint_hash},
            "BlockIdentity": "reported",
            "ManifestDigest": manifest_digest,
            "Epochs": [],
            "LiveDeadline": _positive(values, "live_deadline_ms", 2000) * 1_000_000,
            "BulkDeadline": _positive(values, "bulk_deadline_ms", 15000) * 1_000_000,
            "MaxResponseBytes": _positive(values, "max_response_bytes", 32 * 1024 * 1024),
        },
        "EndpointSecretReferences": {"paxeer-primary": credential},
        "OptionalEmitterTopics": {},
        "Webhooks": {"Targets": []},
        "Archive": {"Enabled": False},
        "ReplayBatchSize": _positive(values, "replay_batch_size", 64),
        "WorkConcurrency": _positive(values, "work_concurrency", 64),
        "WorkBurst": _positive(values, "work_burst", 256),
        "EventsPerSecond": _positive(values, "events_per_second", 10000),
        "ReplayRequests": [],
        "Rebuilds": [],
        "Operations": {},
        "InitialCheckpointHeight": checkpoint_height,
        "InitialCheckpointHash": checkpoint_hash,
    }


def _rendered_document(data: dict[str, Any], values: Path, name: str) -> FileDocument | None:
    file_key = f"{name}_rendered_file"
    path_key = f"{name}_runtime_path"
    if file_key not in data and path_key not in data:
        return None
    source = _field(data, file_key, str)
    runtime_path = _field(data, path_key, str)
    try:
        raw = (values / source).read_bytes()
        document = json.loads(raw)
    except (OSError, ValueError) as exc:
        raise DocumentError(f"indexer: {file_key} must name a rendered JSON object") from exc
    if not isinstance(document, dict):
        raise DocumentError(f"indexer: {file_key} must name a rendered JSON object")
    return FileDocument(raw, runtime_path)


def documents(service: str, values: str | Path) -> dict[str, Any]:
    """Return the indexer's delivery, lifecycle and receipt documents."""

    if service != "indexer":
        raise DocumentError(f"{service}: indexer document module received the wrong service")
    values = Path(values)
    data = _read_values(values)
    receipt = _rendered_document(data, values, "receipt")
    lifecycle = _rendered_document(data, values, "lifecycle")
    result = {
        "DELIVERY_CONFIG_DOCUMENT": _delivery(data),
    }
    if receipt is None:
        result["INDEXER_RECEIPT_CONFIG_DOCUMENT"] = _receipt(data, _roles(data))
    else:
        result["INDEXER_RECEIPT_CONFIG_FILE"] = receipt
    if lifecycle is None:
        result["INDEXER_LIFECYCLE_CONFIG_DOCUMENT"] = _lifecycle(data)
    else:
        result["INDEXER_LIFECYCLE_CONFIG_FILE"] = lifecycle
    if "history_from_height" in data:
        requests, _ = _history(data, result)
        if lifecycle is None:
            result["INDEXER_LIFECYCLE_CONFIG_DOCUMENT"]["BackfillRequests"] = requests
            receipt_file = result.get("INDEXER_RECEIPT_CONFIG_FILE")
            receipt_doc = json.loads(receipt_file.data) if receipt_file else result["INDEXER_RECEIPT_CONFIG_DOCUMENT"]
            production = receipt_doc.get("Production", receipt_doc.get("production", {}))
            epochs = []
            for epoch in production.get("Epochs", production.get("epochs", [])):
                deployment = epoch.get("deployment", epoch.get("Deployment", {}))
                order = deployment["From"]
                epochs.append({"Address": deployment["Address"].lower(), "Role": deployment["Role"], "ABIRevision": deployment["ABIRevision"], "ManifestDigest": data["manifest_digest"], "ActivationHeight": order.get("Block", order.get("block"))})
            result["INDEXER_LIFECYCLE_CONFIG_DOCUMENT"]["Committed"]["Epochs"] = epochs
        else:
            doc = json.loads(lifecycle.data)
            if doc.get("backfillRequest", doc.get("BackfillRequest")) or doc.get("backfillRequests", doc.get("BackfillRequests")):
                raise DocumentError("indexer: rendered lifecycle already has backfill requests")
            doc["backfillRequests"] = requests
            result["INDEXER_LIFECYCLE_CONFIG_FILE"] = FileDocument(_json(doc), lifecycle.path)
    return result


def _json(value: Any) -> bytes:
    return json.dumps(value, sort_keys=True, separators=(",", ":")).encode("utf-8")


def _history(data: dict[str, Any], docs: dict[str, Any]) -> tuple[list[dict[str, Any]], str]:
    receipt_file = docs.get("INDEXER_RECEIPT_CONFIG_FILE")
    receipt = json.loads(receipt_file.data) if receipt_file else docs["INDEXER_RECEIPT_CONFIG_DOCUMENT"]
    production = receipt.get("Production", receipt.get("production", {}))
    manifest = production.get("Manifest", production.get("manifest", {}))
    activation = manifest.get("activationHeight")
    anchor = production.get("Anchor", production.get("anchor", {}))
    first = _nonnegative(data, "history_from_height", 2**63 - 1)
    last = _nonnegative(data, "anchor_height", 2**63 - 1)
    if not isinstance(activation, int) or isinstance(activation, bool) or first < activation:
        raise DocumentError("indexer: history_from_height below manifest activation height")
    if first > last:
        raise DocumentError("indexer: history_from_height above anchor")
    if manifest.get("sourceManifestSha256") != _digest(data, "manifest_digest") or manifest.get("deploymentVersion") != _token(data, "deployment_id") or anchor.get("Height", anchor.get("height")) != last or anchor.get("Hash", anchor.get("hash", "")).lower() != data["anchor_hash"].lower():
        raise DocumentError("indexer: history receipt manifest or anchor mismatch")
    operator = _field(data, "history_operator", str)
    if not re.fullmatch(r"[a-zA-Z0-9_-]{1,128}", operator):
        raise DocumentError("indexer: history_operator")
    expiry = _field(data, "history_expires_at", str)
    try:
        parsed = datetime.fromisoformat(expiry.replace("Z", "+00:00"))
        if parsed.tzinfo is None or parsed.utcoffset() is None:
            raise ValueError("timezone required")
        expiry = parsed.astimezone(timezone.utc).isoformat()
    except ValueError as exc:
        raise DocumentError("indexer: history_expires_at must be an RFC3339 timestamp with timezone") from exc
    stream = _token(data, "canonical_stream_id")
    projection = _token(data, "projection_version")
    generation = _token(data, "inventory_generation")
    for token in (stream, projection, generation):
        if not re.fullmatch(r"[a-zA-Z0-9_-]{1,128}", token):
            raise DocumentError("indexer: history requires replay-safe stream, projection and generation identities")
    count = (last - first) // 1_000_000 + 1
    if count > 16:
        raise DocumentError("indexer: history exceeds 16 bounded requests")
    requests = []
    for start in range(first, last + 1, 1_000_000):
        request = {
            "SourceStream": stream, "Target": "indexer", "Generation": generation,
            "ProjectionRevision": projection, "Mode": "backfill", "Requester": operator,
            "FromHeight": start, "ToHeight": min(start + 999_999, last),
            "CheckpointHeight": last, "CheckpointHash": data["anchor_hash"].lower(),
        }
        identity = hashlib.sha256(_json(request)).hexdigest()
        request.update(JobID="history-" + identity, AuthorizationID="history-" + identity)
        requests.append(request)
    return requests, expiry


def environment(service: str, values: str | Path) -> dict[str, str]:
    data = _read_values(Path(values))
    if "history_from_height" not in data:
        return {}
    batch = _positive(data, "backfill_batch_size", 64) if "backfill_batch_size" in data else _positive(data, "replay_batch_size", 64)
    return {"INDEXER_MODE": "backfill", "BACKFILL_ONLY": "false", "BACKFILL_FROM_BLOCK": str(data["history_from_height"]), "BACKFILL_BATCH_SIZE": str(batch)}


def output_files(service: str, values: str | Path) -> dict[str, bytes]:
    data = _read_values(Path(values))
    if "history_from_height" not in data:
        return {}
    docs = documents(service, values)
    requests, expiry = _history(data, docs)
    columns = "authorization_id,operator_identity,authorization_evidence,action,target,generation,projection_revision,from_height,to_height,max_concurrent,expires_at,revoked_at"
    rows = []
    for request in requests:
        evidence = hashlib.sha256(_json(request)).hexdigest()
        fields = [request["AuthorizationID"], request["Requester"], evidence, request["Mode"], request["Target"], request["Generation"], request["ProjectionRevision"]]
        literals = ["'" + field.replace("'", "''") + "'" for field in fields]
        literals += [str(request["FromHeight"]), str(request["ToHeight"]), "1", "'" + expiry + "'::timestamptz", "NULL"]
        rows.append(f"INSERT INTO indexer.indexer_replay_authorizations ({columns}) VALUES ({','.join(literals)}) ON CONFLICT (authorization_id) DO NOTHING;")
    files = {"indexer-history-authorizations.sql": ("BEGIN;\n" + "\n".join(rows) + "\nCOMMIT;\n").encode("utf-8")}
    if "INDEXER_LIFECYCLE_CONFIG_FILE" in docs:
        files["indexer-lifecycle.json"] = docs["INDEXER_LIFECYCLE_CONFIG_FILE"].data
    return files


compose = documents
