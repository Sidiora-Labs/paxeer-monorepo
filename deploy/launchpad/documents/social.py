"""Compose the social service's private runtime binding."""

from __future__ import annotations

import json
import re
from pathlib import Path
from typing import Any
from urllib.parse import urlparse

from common import DocumentError


_HEX64 = re.compile(r"^[0-9a-f]{64}$")
_NAME = re.compile(r"^[a-z0-9][a-z0-9_.-]{0,127}$")
_STREAM_NAME = re.compile(r"^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$")
_SUBJECT = re.compile(r"^[a-z0-9][a-z0-9_.-]{0,255}$")
_REQUIRED = {
    "version",
    "authorityPeers",
    "identityEndpoint",
    "identityServerPeerCertSHA256",
    "identitySigningKeyReference",
    "gatewayDNS",
    "gatewayCertSHA256",
    "caFile",
    "caSHA256",
    "serverCertFile",
    "serverCertSHA256",
    "serverKeyReference",
    "clientCertFile",
    "clientCertSHA256",
    "clientKeyReference",
    "natsCredentialsReference",
    "natsURL",
    "chatStream",
    "chatSubject",
    "chatDurable",
    "eventStream",
    "deliverySchema",
    "consumerGeneration",
    "erasureSubject",
    "erasureStream",
    "erasureDurable",
    "policySubject",
    "policyStream",
    "policyDurable",
    "recoverySchema",
    "recoveryEpoch",
    "recoveryRegion",
    "cursorKeyReference",
    "identitySchema",
    "accountSchema",
    "indexerSchema",
    "socialSchema",
    "interactionSubjects",
    "mentionSubject",
    "notificationEndpoint",
    "notificationTokenReference",
    "pendingOutboxLimit",
    "maxOutboxAgeSeconds",
}


def _values_path(values: Path) -> Path:
    for name in ("social.json", "social.example.json"):
        path = values / name
        if path.is_file():
            return path
    raise DocumentError("social: missing values file social.json")


def _read_values(values: Path) -> dict[str, Any]:
    path = _values_path(values)
    try:
        raw = json.loads(path.read_bytes())
    except (OSError, json.JSONDecodeError) as exc:
        raise DocumentError(f"social: invalid values file {path.name}") from exc
    if not isinstance(raw, dict):
        raise DocumentError("social: values document must be an object")
    missing = sorted(_REQUIRED - raw.keys())
    if missing:
        raise DocumentError(f"social: missing value {missing[0]}")
    return raw


def _string(values: dict[str, Any], key: str, *, pattern: re.Pattern[str] | None = None) -> str:
    value = values.get(key)
    if not isinstance(value, str) or not value.strip():
        raise DocumentError(f"social: malformed value {key}")
    value = value.strip()
    if pattern is not None and pattern.fullmatch(value) is None:
        raise DocumentError(f"social: malformed value {key}")
    return value


def _credential(values: dict[str, Any], key: str) -> str:
    value = values.get(key)
    if isinstance(value, dict):
        if value.get("test") is not True:
            raise DocumentError(f"social: malformed value {key}.test")
        value = value.get("value")
    if not isinstance(value, str) or not value.strip():
        raise DocumentError(f"social: malformed value {key}")
    return value.strip()


def _digest(values: dict[str, Any], key: str) -> str:
    value = _string(values, key)
    if _HEX64.fullmatch(value) is None:
        raise DocumentError(f"social: malformed value {key}")
    return value


def _url(values: dict[str, Any], key: str, scheme: str) -> str:
    value = _string(values, key)
    parsed = urlparse(value)
    if (
        parsed.scheme != scheme
        or not parsed.netloc
        or parsed.username is not None
        or parsed.password is not None
        or parsed.query
        or parsed.fragment
    ):
        raise DocumentError(f"social: malformed value {key}")
    return value


def _path(values: dict[str, Any], key: str) -> str:
    value = _string(values, key)
    if not value.startswith("/"):
        raise DocumentError(f"social: malformed value {key}")
    return value


def _name(values: dict[str, Any], key: str) -> str:
    return _string(values, key, pattern=_NAME)


def _stream_name(values: dict[str, Any], key: str) -> str:
    return _string(values, key, pattern=_STREAM_NAME)


def _subject(values: dict[str, Any], key: str) -> str:
    return _string(values, key, pattern=_SUBJECT)


def _positive_int(values: dict[str, Any], key: str, *, maximum: int | None = None) -> int:
    value = values.get(key)
    if isinstance(value, bool) or not isinstance(value, int) or value <= 0 or (maximum is not None and value > maximum):
        raise DocumentError(f"social: malformed value {key}")
    return value


def _runtime(values: dict[str, Any]) -> dict[str, Any]:
    authority_peers = values.get("authorityPeers")
    if not isinstance(authority_peers, dict) or not authority_peers:
        raise DocumentError("social: malformed value authorityPeers")
    peers: dict[str, str] = {}
    for name, digest in authority_peers.items():
        if not isinstance(name, str) or not _NAME.fullmatch(name) or not isinstance(digest, str) or not _HEX64.fullmatch(digest):
            raise DocumentError("social: malformed value authorityPeers")
        peers[name] = digest

    interaction_subjects = values.get("interactionSubjects")
    expected_subjects = {"social.edge.changed", "social.comment.changed", "social.callout.changed"}
    if not isinstance(interaction_subjects, dict) or set(interaction_subjects) != expected_subjects:
        raise DocumentError("social: malformed value interactionSubjects")
    subjects: dict[str, str] = {}
    for name, subject in interaction_subjects.items():
        if not isinstance(subject, str) or not _SUBJECT.fullmatch(subject):
            raise DocumentError("social: malformed value interactionSubjects")
        subjects[name] = subject

    runtime = {
        "version": _positive_int(values, "version"),
        "authorityPeers": peers,
        "identityEndpoint": _url(values, "identityEndpoint", "https"),
        "identityServerPeerCertSHA256": _digest(values, "identityServerPeerCertSHA256"),
        "identitySigningKeyReference": _credential(values, "identitySigningKeyReference"),
        "gatewayDNS": _name(values, "gatewayDNS"),
        "gatewayCertSHA256": _digest(values, "gatewayCertSHA256"),
        "caFile": _path(values, "caFile"),
        "caSHA256": _digest(values, "caSHA256"),
        "serverCertFile": _path(values, "serverCertFile"),
        "serverCertSHA256": _digest(values, "serverCertSHA256"),
        "serverKeyReference": _credential(values, "serverKeyReference"),
        "clientCertFile": _path(values, "clientCertFile"),
        "clientCertSHA256": _digest(values, "clientCertSHA256"),
        "clientKeyReference": _credential(values, "clientKeyReference"),
        "natsCredentialsReference": _credential(values, "natsCredentialsReference"),
        "natsURL": _url(values, "natsURL", "tls"),
        "chatStream": _stream_name(values, "chatStream"),
        "chatSubject": _subject(values, "chatSubject"),
        "chatDurable": _stream_name(values, "chatDurable"),
        "eventStream": _stream_name(values, "eventStream"),
        "deliverySchema": _name(values, "deliverySchema"),
        "consumerGeneration": _stream_name(values, "consumerGeneration"),
        "erasureSubject": _subject(values, "erasureSubject"),
        "erasureStream": _stream_name(values, "erasureStream"),
        "erasureDurable": _stream_name(values, "erasureDurable"),
        "policySubject": _subject(values, "policySubject"),
        "policyStream": _stream_name(values, "policyStream"),
        "policyDurable": _stream_name(values, "policyDurable"),
        "recoverySchema": _name(values, "recoverySchema"),
        "recoveryEpoch": _positive_int(values, "recoveryEpoch"),
        "recoveryRegion": _name(values, "recoveryRegion"),
        "cursorKeyReference": _credential(values, "cursorKeyReference"),
        "identitySchema": _name(values, "identitySchema"),
        "accountSchema": _name(values, "accountSchema"),
        "indexerSchema": _name(values, "indexerSchema"),
        "socialSchema": _name(values, "socialSchema"),
        "interactionSubjects": subjects,
        "mentionSubject": _subject(values, "mentionSubject"),
        "notificationEndpoint": _url(values, "notificationEndpoint", "https"),
        "notificationTokenReference": _credential(values, "notificationTokenReference"),
        "pendingOutboxLimit": _positive_int(values, "pendingOutboxLimit"),
        "maxOutboxAgeSeconds": _positive_int(values, "maxOutboxAgeSeconds", maximum=86400),
    }
    if "voice" in values:
        raise DocumentError("social: malformed value voice")
    return runtime


def documents(service: str, values: str | Path) -> dict[str, Any]:
    if service != "social":
        raise DocumentError(f"social: unsupported service {service}")
    return {"SOCIAL_RUNTIME_DOCUMENT": _runtime(_read_values(Path(values)))}
