"""Documents shared by every launchpad runtime service."""

from __future__ import annotations

import json
from pathlib import Path
from typing import Any, NamedTuple


DB_BUDGET = "DB_BUDGET_DOCUMENT"
EXPECTED_SCHEMA = "EXPECTED_SCHEMA_DOCUMENT"
EXPECTED_SCHEMA_FILE = "EXPECTED_SCHEMA_FILE"

SERVICE_REGISTRY_NAMES = {
    "api": "core/api",
    "api-sockets": "core/api",
    "indexer": "core/indexer",
    "charts": "core/trading-charts",
    "stats": "core/stats-workers",
    "pnl": "core/pnl-tracker",
    "ranking": "core/ranking-algo",
    "gateway": "media/gateway",
    "user": "media/user",
    "social": "media/social",
    "metadata": "media/metadata",
    "livestream": "media/livestream",
}


class DocumentError(ValueError):
    """A values directory cannot provide a trusted startup document."""


class FileDocument(NamedTuple):
    """Rendered bytes and the path where the service reads them."""

    data: bytes
    path: str


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


def _expected_schema_runtime_paths(values: Path) -> dict[str, str]:
    path = values / "expected-schema" / "runtime-paths.json"
    if not path.exists():
        return {}
    try:
        raw = json.loads(path.read_bytes())
    except (OSError, json.JSONDecodeError) as exc:
        raise DocumentError(f"{path}: expected a JSON object") from exc
    if not isinstance(raw, dict):
        raise DocumentError(f"{path}: expected a JSON object")
    result: dict[str, str] = {}
    for member, runtime_path in raw.items():
        if member not in SERVICE_REGISTRY_NAMES:
            raise DocumentError(f"{path}: member {member} is not a service in the contract")
        if not isinstance(runtime_path, str) or not Path(runtime_path).is_absolute():
            raise DocumentError(f"{path}: member {member} must be an absolute path")
        result[member] = runtime_path
    return result


def _expected_schema(service: str, values: Path, runtime_paths: dict[str, str]) -> bytes | FileDocument:
    registry_name = SERVICE_REGISTRY_NAMES[service]
    path = values / "expected-schema" / f"{service}.json"
    if not path.is_file():
        raise DocumentError(f"{service}: missing values file expected-schema/{service}.json")
    try:
        raw = path.read_bytes()
        value = json.loads(raw)
    except (OSError, json.JSONDecodeError) as exc:
        raise DocumentError(f"{service}: invalid expected-schema/{service}.json") from exc
    if not isinstance(value, dict) or value.get("service") != registry_name:
        raise DocumentError(f"{service}: expected schema must be one object naming {registry_name}")
    runtime_path = runtime_paths.get(service)
    if runtime_path is not None:
        return FileDocument(raw, runtime_path)
    return raw


def documents(service: str, values: str | Path) -> dict[str, bytes | FileDocument]:
    """Return the shared budget and service readiness documents.

    The budget is serialised in one canonical JSON form. Expected schema bytes
    are retained exactly as supplied so their digest identifies the reviewed
    source document.
    """

    if service not in SERVICE_REGISTRY_NAMES:
        raise DocumentError(f"unknown service {service}")
    values_path = Path(values)
    budget = _stable_json(_read_json(_budget_path(values_path)))
    runtime_paths = _expected_schema_runtime_paths(values_path)
    expected = _expected_schema(service, values_path, runtime_paths)
    expected_name = EXPECTED_SCHEMA_FILE if isinstance(expected, FileDocument) else EXPECTED_SCHEMA
    return {DB_BUDGET: budget, expected_name: expected}

