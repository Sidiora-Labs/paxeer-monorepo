"""Compose the stats service's bounded delivery configuration."""

from __future__ import annotations

import json
import re
from pathlib import Path

from common import DocumentError


TOKEN = r"[A-Za-z0-9_-]{1,128}"
SCHEMA = r"[a-z][a-z0-9_]{0,62}"
SUBJECT = r"[A-Za-z0-9_-]+(?:\.[A-Za-z0-9_-]+)*"


def _require(condition: bool, key: str) -> None:
    if not condition:
        raise DocumentError(f"stats: missing or malformed {key}")


def _object(value: object, keys: set[str], key: str) -> dict:
    _require(isinstance(value, dict), key)
    for name in sorted(keys):
        _require(name in value, f"{key}.{name}" if key else name)
    for name in value:
        _require(name in keys, f"{key}.{name}" if key else name)
    return value


def _text(value: object, pattern: str, key: str) -> None:
    _require(isinstance(value, str) and re.fullmatch(pattern, value) is not None, key)


def _integer(value: object, lower: int, upper: int, key: str) -> None:
    _require(type(value) is int and lower <= value <= upper, key)


def _consumer(value: object, key: str) -> dict:
    value = _object(value, {
        "Stream", "Durable", "Subject", "Batch", "MaxAckPending", "MaxWaiting",
        "MaxDeliver", "FetchWait", "AckWait",
    }, key)
    for name in ("Stream", "Durable"):
        _text(value[name], TOKEN, f"{key}.{name}")
    _text(value["Subject"], SUBJECT, f"{key}.Subject")
    _require(len(value["Subject"]) <= 256, f"{key}.Subject")
    for name, lower, upper in (
        ("Batch", 1, 64), ("MaxAckPending", 1, 10000), ("MaxWaiting", 1, 32),
        ("MaxDeliver", 1, 100), ("FetchWait", 10000000, 1000000000),
        ("AckWait", 1000000000, 3600000000000),
    ):
        _integer(value[name], lower, upper, f"{key}.{name}")
    _require(value["MaxAckPending"] >= value["Batch"], f"{key}.MaxAckPending")
    return value


def documents(service: str, values: str | Path) -> dict[str, object]:
    _require(service == "stats", "service")
    directory = Path(values)
    path = directory / "stats.json"
    if not path.is_file():
        path = directory / "stats.example.json"
    try:
        value = json.loads(path.read_bytes())
    except (OSError, ValueError) as exc:
        raise DocumentError("stats: missing or malformed stats.json") from exc
    d = _object(value, {
        "schema", "sourceStream", "consumerId", "generation", "owner",
        "projectionVersion", "epoch", "region", "maxReplayBlocks",
        "maxReplayFills", "policy", "consumers", "phase2",
    }, "")
    _require(d["schema"] == "stats", "schema")
    for name in ("sourceStream", "consumerId", "generation", "owner", "projectionVersion"):
        _text(d[name], TOKEN, name)
    _text(d["region"], r"[a-z][a-z0-9-]{1,62}", "region")
    _integer(d["epoch"], 1, 2**63 - 1, "epoch")
    _integer(d["maxReplayBlocks"], 1, 4096, "maxReplayBlocks")
    _integer(d["maxReplayFills"], 1, 10000, "maxReplayFills")
    policy = _object(d["policy"], {"Subjects", "EventTypes"}, "policy")
    for name in ("Subjects", "EventTypes"):
        _require(isinstance(policy[name], dict) and bool(policy[name]), f"policy.{name}")
        for item, enabled in policy[name].items():
            _text(item, SUBJECT, f"policy.{name}")
            _require(len(item) <= 256 and type(enabled) is bool, f"policy.{name}.{item}")
    for name in ("Subjects", "EventTypes"):
        _require(policy[name].get("accounting.market.v1") is True, f"policy.{name}.accounting.market.v1")
    _require(isinstance(d["consumers"], list) and len(d["consumers"]) == 1, "consumers")
    market = _consumer(d["consumers"][0], "consumers.0")
    _require(market["Subject"] == "accounting.market.v1", "consumers.0.Subject")
    bounds = {
        "cursorTtlSeconds": (60, 3600), "expiryIntervalSeconds": (1, 60),
        "publishBatch": (1, 100), "publishTimeoutSeconds": (1, 20),
        "publishMaxAttempts": (1, 100), "holderIntervalMs": (100, 60000),
        "holderTokenBatch": (1, 100), "holderTransferBatch": (1, 1000),
        "analyticsExpirySeconds": (1, 30), "analyticsBatch": (1, 100),
        "reactionWalletPerMinute": (1, 12), "reactionIpPerMinute": (1, 4000),
        "searchMaxFills": (1, 128),
    }
    p = _object(d["phase2"], set(bounds) | {
        "windows", "outputStream", "statsSubject", "holderSchema",
        "reactionPolicyVersion", "reactionNamespace", "searchConsumerId",
        "searchGeneration", "searchInvalidationSubject", "searchInvalidationEventType",
        "searchSources", "searchConsumers",
    }, "phase2")
    for name, (lower, upper) in bounds.items():
        _integer(p[name], lower, upper, f"phase2.{name}")
    for name in ("outputStream", "reactionPolicyVersion", "searchConsumerId", "searchGeneration"):
        _text(p[name], TOKEN, f"phase2.{name}")
    _text(p["reactionNamespace"], r"[A-Za-z0-9_-]{1,64}", "phase2.reactionNamespace")
    _text(p["holderSchema"], SCHEMA, "phase2.holderSchema")
    for name in ("statsSubject", "searchInvalidationSubject", "searchInvalidationEventType"):
        _text(p[name], SUBJECT, f"phase2.{name}")
        _require(len(p[name]) <= 256, f"phase2.{name}")
    for name in ("statsSubject", "searchInvalidationSubject"):
        _require(policy["Subjects"].get(p[name]) is True, f"phase2.{name}")
    for event in ("stats.minute.v2", p["searchInvalidationEventType"]):
        _require(policy["EventTypes"].get(event) is True, f"policy.EventTypes.{event}")
    windows = p["windows"]
    _require(isinstance(windows, list) and 1 <= len(windows) <= 16, "phase2.windows")
    for i, window in enumerate(windows):
        _integer(window, 1, 86400, f"phase2.windows.{i}")
        _require(window not in windows[:i], f"phase2.windows.{i}")
    sources = p["searchSources"]
    _require(isinstance(sources, dict), "phase2.searchSources")
    source_kinds = {"metadata", "lifecycle", "metrics", "visibility", "profile_deletion", "preferences"}
    for kind, relation in sources.items():
        key = f"phase2.searchSources.{kind}"
        _require(kind in source_kinds, key)
        relation = _object(relation, {"Schema", "View"}, key)
        for name in ("Schema", "View"):
            _text(relation[name], SCHEMA, f"{key}.{name}")
        _require(relation["View"].endswith("_v2"), f"{key}.View")
    subscriptions = p["searchConsumers"]
    _require(isinstance(subscriptions, list) and 2 <= len(subscriptions) <= 16, "phase2.searchConsumers")
    seen, required = set(), set()
    for i, sub in enumerate(subscriptions):
        key = f"phase2.searchConsumers.{i}"
        sub = _object(sub, {"kind", "eventType", "consumer"}, key)
        _text(sub["kind"], TOKEN, f"{key}.kind")
        _text(sub["eventType"], SUBJECT, f"{key}.eventType")
        c = _consumer(sub["consumer"], f"{key}.consumer")
        _require(policy["Subjects"].get(c["Subject"]) is True, f"{key}.consumer.Subject")
        _require(policy["EventTypes"].get(sub["eventType"]) is True, f"{key}.eventType")
        identity = (c["Stream"], c["Durable"])
        _require(identity not in seen, f"{key}.consumer.Durable")
        seen.add(identity)
        kind = sub["kind"]
        if kind in ("creation", "invalidation"):
            _require(kind not in required, f"{key}.kind")
            required.add(kind)
            subject, event, stream = (
                ("canonical.creation.v1", "canonical_creation", market["Stream"])
                if kind == "creation" else
                (p["searchInvalidationSubject"], p["searchInvalidationEventType"], p["outputStream"])
            )
            _require(c["Subject"] == subject, f"{key}.consumer.Subject")
            _require(c["Stream"] == stream, f"{key}.consumer.Stream")
            _require(sub["eventType"] == event, f"{key}.eventType")
        elif kind in source_kinds:
            _require(kind in sources, f"phase2.searchSources.{kind}")
        elif kind == "account_profile":
            _require(sub["eventType"] in {"profile.updated", "account.deletion.requested", "account.backfill.completed"}, f"{key}.eventType")
        else:
            _require(False, f"{key}.kind")
    _require(required == {"creation", "invalidation"}, "phase2.searchConsumers")
    return {"DELIVERY_CONFIG_DOCUMENT": d}
