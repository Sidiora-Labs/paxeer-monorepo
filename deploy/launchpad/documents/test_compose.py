from __future__ import annotations

import base64
import hashlib
import json
import os
import stat
import subprocess
from pathlib import Path

import pytest


ROOT = Path(__file__).resolve().parent
COMPOSER = ROOT / "compose.py"
SERVICES = [
    "api", "api-sockets", "indexer", "charts", "stats", "pnl", "ranking",
    "gateway", "user", "social", "metadata", "livestream",
]
REGISTRY_NAMES = {
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


def _schema(service: str) -> dict:
    digest = "0" * 64
    return {
        "service": REGISTRY_NAMES[service],
        "runtime_role": "apid_rest" if service == "api" else "apid_sockets" if service == "api-sockets" else service.replace("-", "_") + "d",
        "registry_digest": digest,
        "interface_digest": digest,
        "reader_version": 1,
        "migrations": {"1": digest},
        "schemas": {"app": digest},
        "epoch": 1,
        "region": "mainnet",
        "privileges": [{"schema": "app", "object": "items", "privileges": ["SELECT"]}],
    }


def _values(tmp_path: Path) -> Path:
    values = tmp_path / "values"
    values.mkdir()
    (values / "common.json").write_bytes((ROOT / "values" / "common.example.json").read_bytes())
    for service in SERVICES:
        (values / f"{service}.example.json").write_bytes((ROOT / "values" / f"{service}.example.json").read_bytes())
    expected = values / "expected-schema"
    expected.mkdir()
    for service in SERVICES:
        (expected / f"{service}.json").write_text(json.dumps(_schema(service), separators=(",", ":")))
    return values


def _run(values: Path, out: Path, *services: str) -> subprocess.CompletedProcess[str]:
    command = ["python3", str(COMPOSER), "--values", str(values), "--out", str(out)]
    for service in services:
        command.extend(("--service", service))
    return subprocess.run(command, cwd=values.parent, text=True, capture_output=True)


def test_compose_documents_have_digests_and_private_files(tmp_path: Path) -> None:
    values, out = _values(tmp_path), tmp_path / "out"
    result = _run(values, out, *SERVICES)
    assert result.returncode == 0, result.stderr
    budgets = set()
    for service in SERVICES:
        path = out / f"{service}.json"
        assert stat.S_IMODE(path.stat().st_mode) == 0o600
        record = json.loads(path.read_bytes())
        assert record
        budget = base64.b64decode(record["DB_BUDGET_DOCUMENT"])
        assert hashlib.sha256(budget).hexdigest() == record["DB_BUDGET_SHA256"]
        budgets.add(budget)
        for name, encoded in record.items():
            if name.endswith("_DOCUMENT"):
                digest_name = name[:-len("_DOCUMENT")] + "_SHA256"
                assert hashlib.sha256(base64.b64decode(encoded)).hexdigest() == record[digest_name]
                assert encoded not in result.stdout
    assert len(budgets) == 1


def test_compose_refuses_unknown_service(tmp_path: Path) -> None:
    result = _run(_values(tmp_path), tmp_path / "out", "unknown")
    assert result.returncode != 0
    assert "unknown service unknown" in result.stderr


def test_compose_refuses_missing_expected_schema(tmp_path: Path) -> None:
    values = _values(tmp_path)
    (values / "expected-schema" / "api.json").unlink()
    result = _run(values, tmp_path / "out", "api")
    assert result.returncode != 0
    assert "api" in result.stderr and "EXPECTED_SCHEMA_DOCUMENT" not in result.stderr


def test_compose_registry_names_are_grants_services() -> None:
    grants = json.loads((ROOT.parents[2] / "migrations" / "grants.json").read_bytes())
    role_services = {entry["service"] for entry in grants["roles"]}
    assert set(REGISTRY_NAMES.values()) <= role_services


def test_compose_refuses_platform_schema_name(tmp_path: Path) -> None:
    values = _values(tmp_path)
    path = values / "expected-schema" / "api.json"
    schema = json.loads(path.read_bytes())
    schema["service"] = "api"
    path.write_text(json.dumps(schema, separators=(",", ":")))
    result = _run(values, tmp_path / "out", "api")
    assert result.returncode != 0
    assert "api" in result.stderr and "core/api" in result.stderr


def test_compose_refuses_schema_for_another_registry_service(tmp_path: Path) -> None:
    values = _values(tmp_path)
    path = values / "expected-schema" / "api.json"
    schema = json.loads(path.read_bytes())
    schema["service"] = REGISTRY_NAMES["indexer"]
    path.write_text(json.dumps(schema, separators=(",", ":")))
    result = _run(values, tmp_path / "out", "api")
    assert result.returncode != 0
    assert "api" in result.stderr and "core/api" in result.stderr


def test_compose_refuses_contract_document_from_module(tmp_path: Path) -> None:
    values = _values(tmp_path)
    module = ROOT / "api.py"
    original = module.read_bytes() if module.exists() else None
    try:
        module.write_text("def documents(service, values):\n    return {'NOT_REQUIRED_DOCUMENT': {}}\n")
        result = _run(values, tmp_path / "out", "api")
        assert result.returncode != 0
        assert "api" in result.stderr and "NOT_REQUIRED_DOCUMENT" in result.stderr
    finally:
        if original is None:
            module.unlink(missing_ok=True)
        else:
            module.write_bytes(original)

