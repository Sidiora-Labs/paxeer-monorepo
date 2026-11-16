"""Compose the startup documents consumed by the launchpad API."""

from __future__ import annotations

import json
import math
import re
from pathlib import Path
from typing import Any
from urllib.parse import urlsplit

from common import DocumentError, FileDocument


TOKEN = r"[A-Za-z0-9_-]{1,128}"
SCHEMA = r"[a-z][a-z0-9_]{0,62}"
SUBJECT = r"[A-Za-z0-9_-]+(?:\.[A-Za-z0-9_-]+)*"
REFERENCE = r"[A-Z][A-Z0-9_]{0,127}"
DNS = r"[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?(?:\.[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?)*"
ADDRESS = r"0x[0-9a-fA-F]{40}"
REGION = r"[a-z][a-z0-9-]{1,62}"


def read_values(service: str, values: str | Path) -> Any:
    for name in (f"{service}.json", f"{service}.example.json"):
        path = Path(values) / name
        if path.is_file():
            try:
                return json.loads(path.read_bytes(), object_pairs_hook=unique_keys)
            except (OSError, UnicodeError, json.JSONDecodeError) as exc:
                raise DocumentError(f"{service}: invalid values file {name}") from exc
    raise DocumentError(f"{service}: missing values file {service}.json")


def unique_keys(pairs: list[tuple[str, Any]]) -> dict[str, Any]:
    result = {}
    for key, value in pairs:
        if key in result:
            raise DocumentError(f"{key}: duplicate value")
        result[key] = value
    return result


def object_keys(value: Any, keys: tuple[str, ...], key: str) -> dict[str, Any]:
    if not isinstance(value, dict):
        raise DocumentError(f"{key}: expected an object")
    for field in keys:
        if field not in value:
            raise DocumentError(f"{key}.{field}: missing value")
    for field in value:
        if field not in keys:
            raise DocumentError(f"{key}.{field}: unknown value")
    return value


def string_value(value: Any, key: str, maximum: int, pattern: str | None = None) -> str:
    try:
        length = len(value.encode("utf-8")) if isinstance(value, str) else 0
    except UnicodeError as exc:
        raise DocumentError(f"{key}: invalid UTF-8 string") from exc
    if (
        not isinstance(value, str)
        or not 1 <= length <= maximum
        or (pattern is not None and re.fullmatch(pattern, value) is None)
    ):
        raise DocumentError(f"{key}: invalid string")
    return value


def integer_value(value: Any, key: str, minimum: int, maximum: int) -> int:
    if type(value) is not int or not minimum <= value <= maximum:
        raise DocumentError(f"{key}: expected an integer in {minimum}..{maximum}")
    return value


def number_value(value: Any, key: str, minimum: float, maximum: float) -> float:
    if isinstance(value, bool) or not isinstance(value, (int, float)) or not math.isfinite(value) or not minimum <= value <= maximum:
        raise DocumentError(f"{key}: expected a number in {minimum}..{maximum}")
    return value


def _object(value: Any, keys: set[str], key: str) -> dict[str, Any]:
    return object_keys(value, tuple(sorted(keys)), key)


def _text(value: Any, key: str, pattern: str, maximum: int | None = None) -> str:
    if not isinstance(value, str) or not value or value != value.strip() or (maximum is not None and len(value.encode("utf-8")) > maximum) or re.fullmatch(pattern, value) is None:
        raise DocumentError(f"{key}: invalid value")
    return value


def _url_origin(value: Any, key: str) -> str:
    value = string_value(value, key, 2048)
    parsed = urlsplit(value)
    if (
        parsed.scheme != "https" or not parsed.hostname or parsed.username is not None or parsed.password is not None
        or parsed.query or parsed.fragment or parsed.path not in ("", "/") or parsed.port is None and ":" in parsed.netloc
    ):
        raise DocumentError(f"{key}: invalid value")
    try:
        parsed.port
    except ValueError as exc:
        raise DocumentError(f"{key}: invalid value") from exc
    return value


def _duration_ms(value: Any, key: str, minimum: int, maximum: int) -> int:
    return integer_value(value, key, minimum, maximum) * 1_000_000


def _stream_routes(value: Any) -> list[dict[str, Any]]:
    if not isinstance(value, list) or not 1 <= len(value) <= 16:
        raise DocumentError("stream_relay.routes: expected 1..16 routes")
    routes: list[dict[str, Any]] = []
    streams: set[str] = set()
    filters: list[str] = []
    for index, raw in enumerate(value):
        key = f"stream_relay.routes[{index}]"
        route = _object(raw, {"stream", "subjects"}, key)
        stream = _text(route["stream"], f"{key}.stream", TOKEN)
        if stream in streams:
            raise DocumentError(f"{key}.stream: duplicate value")
        streams.add(stream)
        subjects = route["subjects"]
        if not isinstance(subjects, list) or not 1 <= len(subjects) <= 32:
            raise DocumentError(f"{key}.subjects: expected 1..32 subjects")
        normalized: list[str] = []
        for subject_index, subject in enumerate(subjects):
            subject_key = f"{key}.subjects[{subject_index}]"
            subject = _text(subject, subject_key, r"[A-Za-z0-9_*.>-]+", 256)
            tokens = subject.split(".")
            if any(token == ">" and token_index != len(tokens) - 1 for token_index, token in enumerate(tokens)) or any(token not in {"*", ">"} and re.fullmatch(r"[A-Za-z0-9_-]+", token) is None for token in tokens):
                raise DocumentError(f"{subject_key}: invalid subject filter")
            for previous in filters:
                if _filters_overlap(previous, subject):
                    raise DocumentError(f"{subject_key}: overlaps another subject filter")
            filters.append(subject)
            normalized.append(subject)
        routes.append({"stream": stream, "subjects": normalized})
    probes = (
        "market.trades.0x0000000000000000000000000000000000000000",
        "market.candles.0x0000000000000000000000000000000000000000.1m",
        "market.stats.0x0000000000000000000000000000000000000000",
        "market.price.0x0000000000000000000000000000000000000000",
        "market.rankings.trending",
        "private.portfolio.0x0000000000000000000000000000000000000000",
    )
    if any(not any(_subject_matches(subject, probe) for route in routes for subject in route["subjects"]) for probe in probes):
        raise DocumentError("stream_relay.routes: every committed topic family must be routed")
    return routes


def _subject_matches(pattern: str, subject: str) -> bool:
    expected, actual = pattern.split("."), subject.split(".")
    for index, token in enumerate(expected):
        if token == ">":
            return len(actual) > index
        if index >= len(actual) or (token != "*" and token != actual[index]):
            return False
    return len(expected) == len(actual)


def _filters_overlap(left: str, right: str) -> bool:
    a, b = left.split("."), right.split(".")
    for index in range(max(len(a), len(b))):
        if index == len(a) or index == len(b):
            return len(a) == len(b)
        if a[index] == ">" or b[index] == ">":
            return True
        if a[index] != "*" and b[index] != "*" and a[index] != b[index]:
            return False
    return True


def _stream_relay(data: dict[str, Any]) -> dict[str, Any]:
    root = _object(data.get("stream_relay"), {
        "routes", "epoch", "region", "batch", "max_attempts", "lease_duration_ms",
        "publish_timeout_ms", "retry_delay_ms", "idle_interval_ms", "max_lag_ms",
        "concurrency", "events_per_second", "burst",
    }, "stream_relay")
    routes = _stream_routes(root["routes"])
    epoch = integer_value(root["epoch"], "stream_relay.epoch", 1, 2**63 - 1)
    region = _text(root["region"], "stream_relay.region", REGION)
    batch = integer_value(root["batch"], "stream_relay.batch", 1, 1000)
    max_attempts = integer_value(root["max_attempts"], "stream_relay.max_attempts", 1, 100)
    lease = _duration_ms(root["lease_duration_ms"], "stream_relay.lease_duration_ms", 1000, 300000)
    publish = _duration_ms(root["publish_timeout_ms"], "stream_relay.publish_timeout_ms", 1, 299999)
    retry = _duration_ms(root["retry_delay_ms"], "stream_relay.retry_delay_ms", 1000, 3600000)
    idle = _duration_ms(root["idle_interval_ms"], "stream_relay.idle_interval_ms", 50, 10000)
    lag = _duration_ms(root["max_lag_ms"], "stream_relay.max_lag_ms", 1000, 3600000)
    if publish >= lease:
        raise DocumentError("stream_relay.publish_timeout_ms: must be less than lease_duration_ms")
    concurrency = integer_value(root["concurrency"], "stream_relay.concurrency", 1, 64)
    events = number_value(root["events_per_second"], "stream_relay.events_per_second", 1e-300, 10000)
    burst = integer_value(root["burst"], "stream_relay.burst", 1, 1000)
    return {
        "routes": routes, "epoch": epoch, "region": region, "batch": batch,
        "maxAttempts": max_attempts, "leaseDuration": lease, "publishTimeout": publish,
        "retryDelay": retry, "idleInterval": idle, "maxLag": lag, "concurrency": concurrency,
        "eventsPerSecond": events, "burst": burst,
    }


def _discovery_push(value: Any) -> dict[str, Any]:
    keys = {"apns_key", "apns_key_id", "apns_team_id", "apns_topic", "fcm_service_account", "vapid_key", "vapid_subject", "owner", "deep_link_origin", "apns_sandbox", "enable_apns", "enable_fcm", "enable_web_push"}
    push = _object(value, keys, "discovery_runtime.push")
    for key in ("apns_sandbox", "enable_apns", "enable_fcm", "enable_web_push"):
        if type(push[key]) is not bool:
            raise DocumentError(f"discovery_runtime.push.{key}: expected boolean")
    refs = (("apns_key", "enable_apns"), ("apns_key_id", "enable_apns"), ("apns_team_id", "enable_apns"), ("apns_topic", "enable_apns"), ("fcm_service_account", "enable_fcm"), ("vapid_key", "enable_web_push"), ("vapid_subject", "enable_web_push"))
    for key, enabled in refs:
        value = push[key]
        if push[enabled]:
            _text(value, f"discovery_runtime.push.{key}", REFERENCE)
        elif value != "":
            raise DocumentError(f"discovery_runtime.push.{key}: disabled provider must not name a secret")
    owner = push["owner"]
    origin = push["deep_link_origin"]
    if any(push[key] for key in ("enable_apns", "enable_fcm", "enable_web_push")):
        _text(owner, "discovery_runtime.push.owner", REFERENCE)
        _url_origin(origin, "discovery_runtime.push.deep_link_origin")
    elif not isinstance(owner, str) or not isinstance(origin, str):
        raise DocumentError("discovery_runtime.push: owner and deep_link_origin must be strings")
    return {
        "APNSKey": push["apns_key"], "APNSKeyID": push["apns_key_id"], "APNSTeamID": push["apns_team_id"],
        "APNSTopic": push["apns_topic"], "FCMServiceAccount": push["fcm_service_account"], "VAPIDKey": push["vapid_key"],
        "VAPIDSubject": push["vapid_subject"], "Owner": owner, "DeepLinkOrigin": origin,
        "APNSSandbox": push["apns_sandbox"], "EnableAPNS": push["enable_apns"], "EnableFCM": push["enable_fcm"], "EnableWebPush": push["enable_web_push"],
    }


def _discovery_runtime(data: dict[str, Any]) -> dict[str, Any]:
    root = _object(data.get("discovery_runtime"), {
        "quote_token_address", "d11_base", "d11_token_ref", "d12_base", "d12_token_ref",
        "cursor_key_ref", "internal_token_ref", "delete_peer_dns", "chat_peer_dns", "push",
    }, "discovery_runtime")
    quote = _text(root["quote_token_address"], "discovery_runtime.quote_token_address", ADDRESS)
    if quote.lower() != quote:
        raise DocumentError("discovery_runtime.quote_token_address: must be lowercase")
    d11 = _url_origin(root["d11_base"], "discovery_runtime.d11_base")
    d12 = _url_origin(root["d12_base"], "discovery_runtime.d12_base")
    refs = {}
    for key in ("d11_token_ref", "d12_token_ref", "cursor_key_ref", "internal_token_ref"):
        refs[key] = _text(root[key], f"discovery_runtime.{key}", REFERENCE)
    for key in ("delete_peer_dns", "chat_peer_dns"):
        _text(root[key], f"discovery_runtime.{key}", DNS, 253)
    return {
        "quoteTokenAddress": quote, "d11Base": d11, "d11TokenRef": refs["d11_token_ref"],
        "d12Base": d12, "d12TokenRef": refs["d12_token_ref"], "cursorKeyRef": refs["cursor_key_ref"],
        "internalTokenRef": refs["internal_token_ref"], "deletePeerDNS": root["delete_peer_dns"],
        "chatPeerDNS": root["chat_peer_dns"], "push": _discovery_push(root["push"]),
    }


def _rendered_document(data: dict[str, Any], values: Path) -> FileDocument:
    file_key, path_key = "network_authority_rendered_file", "network_authority_runtime_path"
    if file_key not in data or path_key not in data:
        raise DocumentError(f"api: {file_key} and {path_key} are required")
    source = _text(data[file_key], file_key, r"[^\x00-\x1f\x7f]+", 4096)
    runtime_path = _text(data[path_key], path_key, r"[^\x00-\x1f\x7f]+", 4096)
    try:
        raw = (values / source).read_bytes()
        document = json.loads(raw)
    except (OSError, UnicodeError, json.JSONDecodeError) as exc:
        raise DocumentError(f"api: {file_key} must name a rendered JSON object") from exc
    if not isinstance(document, dict):
        raise DocumentError(f"api: {file_key} must name a rendered JSON object")
    return FileDocument(raw, runtime_path)


def documents(service: str, values: str | Path) -> dict[str, Any]:
    if service != "api":
        raise DocumentError(f"{service}: expected api")
    data = read_values(service, values)
    if not isinstance(data, dict):
        raise DocumentError("api: values file must be an object")
    schemas = (
        "ChartsSchema", "StatsSchema", "ChartsDeliverySchema", "StatsDeliverySchema",
        "StreamSchema", "SourcePartitionSchema",
    )
    tokens = ("ProjectionGeneration", "ChartsConsumerID", "StatsConsumerID")
    keys = schemas + tokens + ("ProjectionVersion", "SourceStream")
    allowed = keys + ("stream_relay", "discovery_runtime", "network_authority_rendered_file", "network_authority_runtime_path")
    object_keys(data, allowed, "ACCOUNTING_READ_CONFIG_DOCUMENT")
    document = {key: data[key] for key in keys}
    for key in schemas:
        string_value(document[key], key, 63, r"[a-z][a-z0-9_]{0,62}")
    for key in tokens:
        string_value(document[key], key, 128, r"[a-zA-Z0-9_-]{1,128}")
    string_value(document["ProjectionVersion"], "ProjectionVersion", 64)
    string_value(document["SourceStream"], "SourceStream", 128)
    if document["ChartsConsumerID"] == document["StatsConsumerID"]:
        raise DocumentError("StatsConsumerID: must differ from ChartsConsumerID")
    return {
        "ACCOUNTING_READ_CONFIG_DOCUMENT": document,
        "STREAM_RELAY_CONFIG_DOCUMENT": _stream_relay(data),
        "DISCOVERY_RUNTIME_CONFIG_DOCUMENT": _discovery_runtime(data),
        "NETWORK_AUTHORITY_CONFIG_FILE": _rendered_document(data, Path(values)),
    }
