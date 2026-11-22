"""Compose the chart feature and delivery startup documents."""

from __future__ import annotations

import json
from pathlib import Path
from typing import Any
from urllib.parse import urlparse


FEATURES = "CHART_FEATURES_CONFIG_DOCUMENT"
DELIVERY = "DELIVERY_CONFIG_DOCUMENT"


def _values_file(values: Path) -> Path:
    for name in ("charts.json", "charts.example.json"):
        path = values / name
        if path.is_file():
            return path
    raise ValueError("charts: missing values file charts.json")


def _read_values(values: Path) -> dict[str, Any]:
    path = _values_file(values)
    try:
        value = json.loads(path.read_bytes())
    except (OSError, json.JSONDecodeError) as exc:
        raise ValueError(f"charts: invalid values file {path.name}") from exc
    if not isinstance(value, dict):
        raise ValueError("charts: values must be an object")
    return value


def _required_object(root: dict[str, Any], key: str) -> dict[str, Any]:
    value = root.get(key)
    if not isinstance(value, dict):
        raise ValueError(f"charts: {key} must be an object")
    return value


def _string(root: dict[str, Any], key: str, *, nonempty: bool = True) -> str:
    value = root.get(key)
    if not isinstance(value, str) or (nonempty and not value):
        raise ValueError(f"charts: {key} must be a non-empty string")
    if value != value.strip():
        raise ValueError(f"charts: {key} must not have surrounding whitespace")
    return value


def _integer(root: dict[str, Any], key: str, low: int, high: int) -> int:
    value = root.get(key)
    if isinstance(value, bool) or not isinstance(value, int) or value < low or value > high:
        raise ValueError(f"charts: {key} must be an integer from {low} to {high}")
    return value


def _origin(root: dict[str, Any], key: str) -> str:
    value = _string(root, key)
    parsed = urlparse(value)
    if (
        parsed.scheme not in {"http", "https"}
        or not parsed.netloc
        or parsed.username is not None
        or parsed.password is not None
        or parsed.query
        or parsed.fragment
        or parsed.params
        or parsed.path not in {"", "/"}
    ):
        raise ValueError(f"charts: {key} must be an origin URL")
    if parsed.scheme == "http" and parsed.hostname not in {"127.0.0.1", "localhost", "::1"}:
        raise ValueError(f"charts: {key} must use https")
    return value


def _features(root: dict[str, Any]) -> dict[str, Any]:
    _integer(root, "schemaVersion", 2, 2)
    _string(root, "endpointSetVersion")
    _string(root, "deploymentVersion")
    _integer(root, "anchorHeight", 1, 2**64 - 1)
    anchor_hash = _string(root, "anchorHash")
    if len(anchor_hash) != 66 or not anchor_hash.startswith("0x"):
        raise ValueError("charts: anchorHash must be a 32-byte hexadecimal hash")
    try:
        int(anchor_hash[2:], 16)
    except ValueError as exc:
        raise ValueError("charts: anchorHash must be a 32-byte hexadecimal hash") from exc
    if int(anchor_hash[2:], 16) == 0:
        raise ValueError("charts: anchorHash must not be zero")
    _origin(root, "canonicalApiOrigin")
    _origin(root, "replayGatewayOrigin")
    _string(root, "canonicalSeriesModel")
    _integer(root, "displayPrecision", 1, 12)
    _integer(root, "maxHistoryOrigins", 1, 32)
    _integer(root, "rollupBatch", 1, 64)
    _integer(root, "rollupIntervalMillis", 100, 60000)
    archive = root.get("archive")
    if archive is not None:
        if not isinstance(archive, dict):
            raise ValueError("charts: archive must be an object")
        _origin(archive, "endpoint")
        _string(archive, "bucket")
        if len(archive["bucket"]) > 128:
            raise ValueError("charts: bucket must be at most 128 characters")
        if "region" in archive:
            _string(archive, "region")
    allowed = {
        "schemaVersion", "endpointSetVersion", "deploymentVersion", "anchorHeight", "anchorHash",
        "canonicalApiOrigin", "replayGatewayOrigin", "canonicalSeriesModel", "displayPrecision",
        "maxHistoryOrigins", "rollupBatch", "rollupIntervalMillis", "archive",
    }
    unknown = sorted(set(root) - allowed)
    if unknown:
        raise ValueError(f"charts: {unknown[0]} is not a chart feature field")
    return dict(root)


def _consumer(root: dict[str, Any]) -> dict[str, Any]:
    for key in ("Stream", "Durable", "Subject"):
        _string(root, key)
    _integer(root, "Batch", 1, 64)
    _integer(root, "MaxAckPending", root["Batch"], 10000)
    _integer(root, "MaxWaiting", 1, 32)
    _integer(root, "MaxDeliver", 1, 100)
    _integer(root, "FetchWait", 10_000_000, 1_000_000_000)
    _integer(root, "AckWait", 1_000_000_000, 3_600_000_000_000)
    allowed = {"Stream", "Durable", "Subject", "Batch", "MaxAckPending", "MaxWaiting", "MaxDeliver", "FetchWait", "AckWait"}
    unknown = sorted(set(root) - allowed)
    if unknown:
        raise ValueError(f"charts: consumer.{unknown[0]} is not a consumer field")
    return dict(root)


def _policy(root: dict[str, Any]) -> dict[str, dict[str, bool]]:
    subjects = root.get("Subjects")
    event_types = root.get("EventTypes")
    if not isinstance(subjects, dict) or not isinstance(event_types, dict):
        raise ValueError("charts: policy must contain Subjects and EventTypes objects")
    for name, values in (("policy.Subjects", subjects), ("policy.EventTypes", event_types)):
        if values.get("accounting.market.v1") is not True:
            raise ValueError(f"charts: {name} must enable accounting.market.v1")
        if not values or any(not isinstance(key, str) or not key or not isinstance(value, bool) or not value for key, value in values.items()):
            raise ValueError(f"charts: {name} must contain enabled boolean entries")
    return {"Subjects": dict(subjects), "EventTypes": dict(event_types)}


def _delivery(root: dict[str, Any]) -> dict[str, Any]:
    required_strings = ("schema", "sourceStream", "consumerId", "generation", "owner", "projectionVersion", "region")
    for key in required_strings:
        _string(root, key)
    _integer(root, "epoch", 1, 2**63 - 1)
    _integer(root, "maxReplayBlocks", 1, 4096)
    _integer(root, "maxReplayFills", 1, 10000)
    policy = _policy(_required_object(root, "policy"))
    consumer = _consumer(_required_object(root, "consumer"))
    outbox_root = _required_object(root, "chartOutbox")
    stream = _string(outbox_root, "stream")
    subject = _string(outbox_root, "subject")
    if subject != "candle.updated.v2":
        raise ValueError("charts: chartOutbox.subject must be candle.updated.v2")
    if subject not in policy["Subjects"]:
        raise ValueError("charts: chartOutbox.subject must be admitted by policy.Subjects")
    batch = _integer(outbox_root, "batch", 1, 64)
    _integer(outbox_root, "intervalMillis", 10, 10000)
    if set(outbox_root) - {"stream", "subject", "batch", "intervalMillis"}:
        unknown = sorted(set(outbox_root) - {"stream", "subject", "batch", "intervalMillis"})[0]
        raise ValueError(f"charts: chartOutbox.{unknown} is not a chart outbox field")
    allowed = {
        "schema", "sourceStream", "consumerId", "generation", "owner", "projectionVersion", "epoch",
        "region", "maxReplayBlocks", "maxReplayFills", "policy", "consumer", "chartOutbox",
    }
    unknown = sorted(set(root) - allowed)
    if unknown:
        raise ValueError(f"charts: {unknown[0]} is not a delivery field")
    result = dict(root)
    result["policy"] = policy
    result["consumer"] = consumer
    result["chartOutbox"] = {"stream": stream, "subject": subject, "batch": batch, "intervalMillis": outbox_root["intervalMillis"]}
    return result


def documents(service: str, values: str | Path) -> dict[str, Any]:
    if service != "charts":
        raise ValueError(f"charts: unsupported service {service}")
    root = _read_values(Path(values))
    features = _features(_required_object(root, "features"))
    delivery = _delivery(_required_object(root, "delivery"))
    allowed = {"features", "delivery"}
    unknown = sorted(set(root) - allowed)
    if unknown:
        raise ValueError(f"charts: {unknown[0]} is not a deployment value")
    return {FEATURES: features, DELIVERY: delivery}


compose = documents
