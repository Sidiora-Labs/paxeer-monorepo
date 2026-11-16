"""Compose the ranking service's version-two runtime binding."""

from __future__ import annotations

import json
import re
from pathlib import Path, PurePosixPath
from urllib.parse import urlsplit

from common import DocumentError


PATTERNS = {
    "rankingSchema": r"[a-z][a-z0-9_]{0,62}",
    "statsSchema": r"[a-z][a-z0-9_]{0,62}",
    "sourceSchema": r"[a-z][a-z0-9_]{0,62}",
    "sourceStream": r"[^\s\x00-\x1f\x7f]{1,128}",
    "projectionVersion": r"[^\s\x00-\x1f\x7f]{1,64}",
    "cacheNamespace": r"[a-z][a-z0-9_-]{2,62}",
    "recoverySchema": r"[a-z][a-z0-9_]{1,62}",
    "recoveryRegion": r"[a-z][a-z0-9-]{1,62}",
    "cursorKeyReference": r"[A-Za-z_][A-Za-z0-9_]{0,127}",
    "clientKeyReference": r"[A-Za-z_][A-Za-z0-9_]{0,127}",
    "serverKeyReference": r"[A-Za-z_][A-Za-z0-9_]{0,127}",
}
DIGESTS = (
    "visibilityCertSHA256", "gatewayCertSHA256", "caSha256",
    "clientCertSha256", "serverCertSha256",
)
FILES = ("caFile", "clientCertFile", "serverCertFile")
DNS_NAMES = ("visibilityDNS", "gatewayDNS")
LIMITS = {"recoveryEpoch": (1, 2**63 - 1), "maxAttempts": (1, 100), "maximumReads": (1, 64)}


def _invalid(key: str) -> DocumentError:
    return DocumentError(f"ranking: missing or malformed value {key}")


def _dns(value: object) -> bool:
    return (
        isinstance(value, str)
        and len(value) <= 253
        and all(re.fullmatch(r"[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?", label) for label in value.split("."))
    )


def documents(service: str, values: str | Path) -> dict[str, object]:
    if service != "ranking":
        raise DocumentError("ranking: service must be ranking")
    directory = Path(values)
    path = directory / "ranking.json"
    if not path.is_file():
        path = directory / "ranking.example.json"
    try:
        supplied = json.loads(path.read_bytes())
    except (OSError, ValueError) as exc:
        raise _invalid("ranking.json") from exc
    if not isinstance(supplied, dict):
        raise _invalid("ranking.json")

    keys = set(PATTERNS) | set(DIGESTS) | set(FILES) | set(DNS_NAMES) | set(LIMITS) | {"visibilityEndpoint"}
    for key in sorted(set(supplied) - keys - {"_example"}):
        raise _invalid(key)
    for key in sorted(keys):
        if key not in supplied:
            raise _invalid(key)
    for key, pattern in PATTERNS.items():
        value = supplied[key]
        if not isinstance(value, str) or re.fullmatch(pattern, value) is None:
            raise _invalid(key)
    for key in DIGESTS:
        value = supplied[key]
        if not isinstance(value, str) or re.fullmatch(r"[0-9a-f]{64}", value) is None:
            raise _invalid(key)
    for key in FILES:
        value = supplied[key]
        if (
            not isinstance(value, str)
            or not value.startswith("/")
            or len(value) > 4096
            or re.search(r"[\s\x00-\x1f\x7f]", value)
            or ".." in value.split("/")
            or str(PurePosixPath(value)) != value
            or value == "/"
        ):
            raise _invalid(key)
    for key in DNS_NAMES:
        if not _dns(supplied[key]):
            raise _invalid(key)
    for key, (minimum, maximum) in LIMITS.items():
        value = supplied[key]
        if type(value) is not int or not minimum <= value <= maximum:
            raise _invalid(key)

    key = "visibilityEndpoint"
    value = supplied[key]
    try:
        endpoint = urlsplit(value) if isinstance(value, str) else None
        if (
            endpoint is None
            or endpoint.scheme != "https"
            or not _dns(endpoint.hostname)
            or endpoint.username is not None
            or endpoint.password is not None
            or endpoint.path != "/internal/v2/social/ranking-visibility"
            or "?" in value
            or "#" in value
            or re.search(r"[\s\x00-\x1f\x7f]", value)
            or (":" in endpoint.netloc and (endpoint.port is None or endpoint.port < 1))
        ):
            raise _invalid(key)
    except ValueError as exc:
        raise _invalid(key) from exc

    return {"RANKING_RUNTIME_DOCUMENT": {"version": 2, **{key: supplied[key] for key in sorted(keys)}}}
