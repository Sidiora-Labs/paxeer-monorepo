"""Compose the production startup documents for the launchpad indexer."""

from __future__ import annotations

import base64
import json
import re
from pathlib import Path
from typing import Any


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


def documents(service: str, values: str | Path) -> dict[str, Any]:
    """Return the indexer's delivery, lifecycle and receipt documents."""

    if service != "indexer":
        raise DocumentError(f"{service}: indexer document module received the wrong service")
    data = _read_values(Path(values))
    addresses = _roles(data)
    return {
        "DELIVERY_CONFIG_DOCUMENT": _delivery(data),
        "INDEXER_LIFECYCLE_CONFIG_DOCUMENT": _lifecycle(data),
        "INDEXER_RECEIPT_CONFIG_DOCUMENT": _receipt(data, addresses),
    }


compose = documents
