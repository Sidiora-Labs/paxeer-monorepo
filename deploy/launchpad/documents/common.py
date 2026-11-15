"""Documents shared by every launchpad runtime service."""

from __future__ import annotations

import json
from pathlib import Path
from typing import Any


DB_BUDGET = "DB_BUDGET_DOCUMENT"
EXPECTED_SCHEMA = "EXPECTED_SCHEMA_DOCUMENT"

SERVICE_ROLES = {
    "api": "apid:rest",
    "api-sockets": "apid:sockets",
    "indexer": "indexerd",
    "charts": "chartsd",
    "stats": "statsd",
    "pnl": "pnld",
    "ranking": "rankingd",
    "gateway": "mediagatewayd",
    "user": "userd",
    "social": "sociald",
    "metadata": "metadatad",
    "livestream": "livestreamd",
}


class DocumentError(ValueError):
    """A values directory cannot provide a trusted startup document."""


def _read_json(path: Path) -> Any:
    try:
        return json.loads(path.read_bytes())
    except (OSError, json.JSONDecodeError) as exc:
        raise DocumentError(f"cannot read values file {path}") from exc


def _stable_json(value: Any) -> bytes:
    try:
        return json.dumps(value, ensure_ascii=False, sort_keys=True, separators=(",", ":")).encode("utf-8")
    except (TypeError, ValueError) as exc:
        raise DocumentError("values document is not JSON serializable") from exc


def _budget_path(values: Path) -> Path:
    for name in ("common.json", "common.example.json"):
        path = values / name
        if path.is_file():
            return path
    raise DocumentError("missing values file common.json")


def _expected_schema(service: str, values: Path) -> bytes:
    path = values / "expected-schema" / f"{service}.json"
    if not path.is_file():
        raise DocumentError(f"{service}: missing values file expected-schema/{service}.json")
    try:
        raw = path.read_bytes()
        value = json.loads(raw)
    except (OSError, json.JSONDecodeError) as exc:
        raise DocumentError(f"{service}: invalid expected-schema/{service}.json") from exc
    if not isinstance(value, dict) or value.get("service") != service:
        raise DocumentError(f"{service}: expected schema must be one object naming {service}")
    return raw


def documents(service: str, values: str | Path) -> dict[str, bytes]:
    """Return the shared budget and service readiness documents.

    The budget is serialised in one canonical JSON form. Expected schema bytes
    are retained exactly as supplied so their digest identifies the reviewed
    source document.
    """

    if service not in SERVICE_ROLES:
        raise DocumentError(f"unknown service {service}")
    values_path = Path(values)
    budget = _stable_json(_read_json(_budget_path(values_path)))
    return {DB_BUDGET: budget, EXPECTED_SCHEMA: _expected_schema(service, values_path)}

