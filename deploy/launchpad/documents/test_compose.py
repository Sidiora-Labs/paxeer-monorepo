from __future__ import annotations

import base64
import hashlib
import json
import os
import re
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
ONE_SET_SERVICES = ("charts", "stats", "api", "indexer", "pnl", "ranking")


def _example_set(values: Path) -> dict[str, dict]:
    examples = {
        service: json.loads((values / f"{service}.example.json").read_bytes())
        for service in ONE_SET_SERVICES
    }
    examples["charts"] = examples["charts"]["delivery"]
    return examples


def _assert_one_set(documents: dict[str, dict]) -> None:
    indexer = documents["indexer"]
    charts, stats, api = (documents[service] for service in ("charts", "stats", "api"))
    for service in ("charts", "stats", "pnl", "ranking"):
        assert documents[service]["sourceStream"] == indexer["canonical_stream_id"], f"{service}.sourceStream"
    assert api["SourceStream"] == indexer["canonical_stream_id"], "api.SourceStream"
    for service in ("charts", "stats", "ranking"):
        assert documents[service]["projectionVersion"] == indexer["projection_version"], f"{service}.projectionVersion"
    assert api["ProjectionVersion"] == indexer["projection_version"], "api.ProjectionVersion"
    assert documents["pnl"]["projectionVersion"] == indexer["inventory_projection_version"], "pnl.projectionVersion"
    assert charts["generation"] == stats["generation"] == api["ProjectionGeneration"], "projection generation"
    assert documents["pnl"]["generation"] == indexer["inventory_generation"], "pnl.generation"
    assert api["ChartsDeliverySchema"] == charts["schema"], "api.ChartsDeliverySchema"
    assert api["StatsDeliverySchema"] == stats["schema"], "api.StatsDeliverySchema"
    assert api["ChartsConsumerID"] == charts["consumerId"], "api.ChartsConsumerID"
    assert api["StatsConsumerID"] == stats["consumerId"], "api.StatsConsumerID"


def test_example_values_one_set() -> None:
    _assert_one_set(_example_set(ROOT / "values"))


@pytest.mark.parametrize("example", sorted((ROOT / "values").glob("*.example.json")), ids=lambda path: path.name)
def test_example_schema_names_registered(example: Path) -> None:
    registry = json.loads((ROOT.parents[2] / "migrations/registry.json").read_bytes())
    schemas = {entry["schema"] for entry in registry["registrations"]}

    def check(value: object, location: str) -> None:
        if isinstance(value, dict):
            for name, member in value.items():
                setting = f"{location}.{name}"
                if name.lower().endswith("schema"):
                    assert isinstance(member, str) and member in schemas, f"{setting}: {member!r}"
                check(member, setting)
        elif isinstance(value, list):
            for index, member in enumerate(value):
                check(member, f"{location}[{index}]")

    check(json.loads(example.read_bytes()), example.name)


def test_compose_examples_one_set(tmp_path: Path) -> None:
    values, out = _values(tmp_path), tmp_path / "out"
    result = _run(values, out, *ONE_SET_SERVICES)
    assert result.returncode == 0, result.stderr
    documents = {}
    for service in ONE_SET_SERVICES:
        record = json.loads((out / f"{service}.json").read_bytes())
        name = {
            "api": "ACCOUNTING_READ_CONFIG_DOCUMENT",
            "ranking": "RANKING_RUNTIME_DOCUMENT",
        }.get(service, "DELIVERY_CONFIG_DOCUMENT")
        documents[service] = json.loads(base64.b64decode(record[name]))
    indexer = documents["indexer"]
    documents["indexer"] = {
        "canonical_stream_id": indexer["CanonicalStreamID"],
        "projection_version": indexer["ProjectionVersion"],
        "inventory_projection_version": indexer["InventoryProjectionVersion"],
        "inventory_generation": indexer["Generation"],
    }
    _assert_one_set(documents)


@pytest.mark.parametrize("service, member, comparison", [
    ("charts", "sourceStream", "charts.sourceStream"),
    ("stats", "sourceStream", "stats.sourceStream"),
    ("pnl", "sourceStream", "pnl.sourceStream"),
    ("ranking", "sourceStream", "ranking.sourceStream"),
    ("api", "SourceStream", "api.SourceStream"),
    ("charts", "projectionVersion", "charts.projectionVersion"),
    ("stats", "projectionVersion", "stats.projectionVersion"),
    ("ranking", "projectionVersion", "ranking.projectionVersion"),
    ("api", "ProjectionVersion", "api.ProjectionVersion"),
    ("pnl", "projectionVersion", "pnl.projectionVersion"),
    ("charts", "generation", "projection generation"),
    ("stats", "generation", "projection generation"),
    ("api", "ProjectionGeneration", "projection generation"),
    ("pnl", "generation", "pnl.generation"),
    ("api", "ChartsDeliverySchema", "api.ChartsDeliverySchema"),
    ("api", "StatsDeliverySchema", "api.StatsDeliverySchema"),
    ("api", "ChartsConsumerID", "api.ChartsConsumerID"),
    ("api", "StatsConsumerID", "api.StatsConsumerID"),
    ("charts", "schema", "api.ChartsDeliverySchema"),
    ("stats", "schema", "api.StatsDeliverySchema"),
    ("charts", "consumerId", "api.ChartsConsumerID"),
    ("stats", "consumerId", "api.StatsConsumerID"),
])
def test_example_values_one_set_refuses_mismatch(tmp_path: Path, service: str, member: str, comparison: str) -> None:
    values = _values(tmp_path)
    _assert_one_set(_example_set(values))
    path = values / f"{service}.example.json"
    example = json.loads(path.read_bytes())
    document = example["delivery"] if service == "charts" else example
    document[member] += "_other"
    path.write_text(json.dumps(example))
    with pytest.raises(AssertionError, match=comparison):
        _assert_one_set(_example_set(values))


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
    (values / "network-authority.json").write_bytes((ROOT / "values" / "network-authority.json").read_bytes())
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


def test_compose_expected_schema_file_form_preserves_other_documents(tmp_path: Path) -> None:
    values = _values(tmp_path)
    baseline_out = tmp_path / "baseline"
    baseline = _run(values, baseline_out, "social", "livestream")
    assert baseline.returncode == 0, baseline.stderr
    baseline_records = {
        service: json.loads((baseline_out / f"{service}.json").read_bytes())
        for service in ("social", "livestream")
    }
    expected = values / "expected-schema"
    schema_bytes = (expected / "social.json").read_bytes()
    (expected / "runtime-paths.json").write_text(json.dumps({"social": "/run/launchpad/social/expected-schema.json"}))

    out = tmp_path / "file-form"
    result = _run(values, out, "social", "livestream")
    assert result.returncode == 0, result.stderr
    social = json.loads((out / "social.json").read_bytes())
    livestream = json.loads((out / "livestream.json").read_bytes())
    assert social["EXPECTED_SCHEMA_FILE"] == "/run/launchpad/social/expected-schema.json"
    assert social["EXPECTED_SCHEMA_SHA256"] == hashlib.sha256(schema_bytes).hexdigest()
    assert "EXPECTED_SCHEMA_DOCUMENT" not in social
    assert livestream["EXPECTED_SCHEMA_DOCUMENT"] == baseline_records["livestream"]["EXPECTED_SCHEMA_DOCUMENT"]
    assert livestream["EXPECTED_SCHEMA_SHA256"] == baseline_records["livestream"]["EXPECTED_SCHEMA_SHA256"]
    for service in ("social", "livestream"):
        assert json.loads((out / f"{service}.json").read_bytes())["DB_BUDGET_DOCUMENT"] == baseline_records[service]["DB_BUDGET_DOCUMENT"]
        assert json.loads((out / f"{service}.json").read_bytes())["DB_BUDGET_SHA256"] == baseline_records[service]["DB_BUDGET_SHA256"]


@pytest.mark.parametrize(
    ("paths", "member"),
    [([], None), ({"unknown": "/run/unknown.json"}, "unknown"), ({"social": "relative.json"}, "social")],
)
def test_compose_refuses_invalid_expected_schema_runtime_paths(tmp_path: Path, paths: object, member: str | None) -> None:
    values = _values(tmp_path)
    path = values / "expected-schema" / "runtime-paths.json"
    path.write_text(json.dumps(paths))
    result = _run(values, tmp_path / "out", "social")
    assert result.returncode == 2
    assert "expected-schema/runtime-paths.json" in result.stderr
    if member is not None:
        assert member in result.stderr


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


def test_compose_api_readiness_documents_are_bound(tmp_path: Path) -> None:
    values, out = _values(tmp_path), tmp_path / "out"
    result = _run(values, out, "api")
    assert result.returncode == 0, result.stderr
    record = json.loads((out / "api.json").read_bytes())
    for name in ("STREAM_RELAY_CONFIG_DOCUMENT", "DISCOVERY_RUNTIME_CONFIG_DOCUMENT"):
        raw = base64.b64decode(record[name])
        assert hashlib.sha256(raw).hexdigest() == record[name.removesuffix("_DOCUMENT") + "_SHA256"]
    authority = (values / "network-authority.json").read_bytes()
    assert record["NETWORK_AUTHORITY_CONFIG_FILE"] == "/run/launchpad/api/network-authority.json"
    assert record["NETWORK_AUTHORITY_CONFIG_SHA256"] == hashlib.sha256(authority).hexdigest()
    relay = json.loads(base64.b64decode(record["STREAM_RELAY_CONFIG_DOCUMENT"]))
    assert relay["batch"] == 100 and relay["leaseDuration"] == 30_000_000_000
    discovery = json.loads(base64.b64decode(record["DISCOVERY_RUNTIME_CONFIG_DOCUMENT"]))
    assert discovery["d11TokenRef"] == "ACCOUNT_SERVICE_TOKEN"
    assert discovery["push"]["EnableAPNS"] is False


def test_private_listener_fixed_names() -> None:
    contracts = json.loads((ROOT.parent / "railway" / "variables.json").read_bytes())
    assert contracts["api"]["fixed"]["API_PRIVATE_LISTEN_ADDRESS"] == "[::]:8443"
    assert "API_PRIVATE_LISTEN_ADDRESS" not in contracts["api-sockets"]["fixed"]
    addresses = {
        contracts[service]["fixed"][f"{service.upper()}_PRIVATE_LISTEN_ADDRESS"]
        for service in ("api", "ranking", "user", "social", "metadata")
    }
    assert addresses == {"[::]:8443"}


@pytest.mark.parametrize(("section", "member", "message"), [
    ("stream_relay", "batch", "stream_relay.batch"),
    ("discovery_runtime", "d11_base", "discovery_runtime.d11_base"),
])
def test_compose_api_refuses_readiness_values_outside_runtime_bounds(tmp_path: Path, section: str, member: str, message: str) -> None:
    values = _values(tmp_path)
    data = json.loads((values / "api.example.json").read_bytes())
    data[section][member] = 0 if member == "batch" else "http://accounts.example"
    (values / "api.example.json").write_text(json.dumps(data))
    result = _run(values, tmp_path / "out", "api")
    assert result.returncode == 2
    assert message in result.stderr


def test_compose_api_binds_changed_network_authority_file_by_digest(tmp_path: Path) -> None:
    values = _values(tmp_path)
    out = tmp_path / "out"
    result = _run(values, out, "api")
    assert result.returncode == 0, result.stderr
    record = json.loads((out / "api.json").read_bytes())
    path = values / "network-authority.json"
    raw = bytearray(path.read_bytes())
    raw[-2] ^= 1
    path.write_bytes(raw)
    assert hashlib.sha256(raw).hexdigest() != record["NETWORK_AUTHORITY_CONFIG_SHA256"]


def _indexer_rendered_values(values: Path, out: Path) -> tuple[dict, dict[str, bytes]]:
    result = _run(values, out, "indexer")
    assert result.returncode == 0, result.stderr
    record = json.loads((out / "indexer.json").read_bytes())
    data = json.loads((values / "indexer.example.json").read_bytes())
    rendered = {}
    for name in ("receipt", "lifecycle"):
        document = json.loads(base64.b64decode(record[f"INDEXER_{name.upper()}_CONFIG_DOCUMENT"]))
        raw = (json.dumps(document, indent=2) + "\n").encode()
        (values / f"{name}.json").write_bytes(raw)
        data[f"{name}_rendered_file"] = f"{name}.json"
        data[f"{name}_runtime_path"] = f"/run/launchpad/indexer/{name}.json"
        rendered[name] = raw
    return data, rendered


@pytest.mark.parametrize("file_names", [("receipt", "lifecycle"), ("receipt",), ("lifecycle",)])
def test_compose_indexer_file_form(tmp_path: Path, file_names: tuple[str, ...]) -> None:
    values = _values(tmp_path)
    data, rendered = _indexer_rendered_values(values, tmp_path / "inline")
    for name in ("receipt", "lifecycle"):
        if name not in file_names:
            del data[f"{name}_rendered_file"], data[f"{name}_runtime_path"]
    # Rendered inputs may also be absolute paths outside the values directory.
    if "receipt" in file_names:
        data["receipt_rendered_file"] = str(values / "receipt.json")
    (values / "indexer.example.json").write_text(json.dumps(data))
    result = _run(values, tmp_path / "files", "indexer")
    assert result.returncode == 0, result.stderr
    record = json.loads((tmp_path / "files" / "indexer.json").read_bytes())
    for name in ("receipt", "lifecycle"):
        prefix = f"INDEXER_{name.upper()}_CONFIG"
        if name in file_names:
            assert record[f"{prefix}_FILE"] == data[f"{name}_runtime_path"]
            assert record[f"{prefix}_SHA256"] == hashlib.sha256(rendered[name]).hexdigest()
            assert f"{prefix}_DOCUMENT" not in record
        else:
            assert f"{prefix}_FILE" not in record
            assert hashlib.sha256(base64.b64decode(record[f"{prefix}_DOCUMENT"])).hexdigest() == record[f"{prefix}_SHA256"]


@pytest.mark.parametrize("name", ["receipt", "lifecycle"])
@pytest.mark.parametrize("invalid", ["missing", "empty", "array", "null", "string", "number", "invalid", "encoding", "trailing"])
def test_compose_indexer_refuses_invalid_rendered_file(tmp_path: Path, name: str, invalid: str) -> None:
    values = _values(tmp_path)
    data = json.loads((values / "indexer.example.json").read_bytes())
    data[f"{name}_rendered_file"] = "rendered.json"
    data[f"{name}_runtime_path"] = f"/run/launchpad/indexer/{name}.json"
    payloads = {"empty": b"", "array": b"[]", "null": b"null", "string": b'"document"',
                "number": b"1", "invalid": b"{", "encoding": b"\xff", "trailing": b"{} {}"}
    if invalid != "missing":
        (values / "rendered.json").write_bytes(payloads[invalid])
    (values / "indexer.example.json").write_text(json.dumps(data))
    result = _run(values, tmp_path / "out", "indexer")
    assert result.returncode == 2
    assert f"indexer: {name}_rendered_file" in result.stderr
    assert not (tmp_path / "out" / "indexer.json").exists()


@pytest.mark.parametrize("name", ["receipt", "lifecycle"])
@pytest.mark.parametrize("key, value", [("rendered_file", "rendered.json"), ("runtime_path", "/run/indexer.json")])
def test_compose_indexer_refuses_unpaired_file_values(tmp_path: Path, name: str, key: str, value: str) -> None:
    values = _values(tmp_path)
    data = json.loads((values / "indexer.example.json").read_bytes())
    data[f"{name}_{key}"] = value
    (values / "indexer.example.json").write_text(json.dumps(data))
    result = _run(values, tmp_path / "out", "indexer")
    missing = "runtime_path" if key == "rendered_file" else "rendered_file"
    assert result.returncode == 2
    assert f"indexer: {name}_{missing}" in result.stderr


@pytest.mark.parametrize("name", ["receipt", "lifecycle"])
@pytest.mark.parametrize("key", ["rendered_file", "runtime_path"])
@pytest.mark.parametrize("value", [None, "", " ", 1])
def test_compose_indexer_refuses_invalid_file_values(tmp_path: Path, name: str, key: str, value: object) -> None:
    values = _values(tmp_path)
    data = json.loads((values / "indexer.example.json").read_bytes())
    data[f"{name}_rendered_file"] = "rendered.json"
    data[f"{name}_runtime_path"] = "/run/indexer.json"
    data[f"{name}_{key}"] = value
    (values / "indexer.example.json").write_text(json.dumps(data))
    result = _run(values, tmp_path / "out", "indexer")
    assert result.returncode == 2
    assert f"indexer: {name}_{key}" in result.stderr


@pytest.mark.parametrize("name", ["receipt", "lifecycle"])
def test_compose_indexer_document_source_exclusivity(tmp_path: Path, name: str) -> None:
    import compose
    import indexer

    values = _values(tmp_path)
    inline = indexer.documents("indexer", values)
    data, _ = _indexer_rendered_values(values, tmp_path / "inline")
    (values / "indexer.example.json").write_text(json.dumps(data))
    supplied = indexer.documents("indexer", values)
    sources = compose._contract()["indexer"]["documentSources"]
    document, path = f"INDEXER_{name.upper()}_CONFIG_DOCUMENT", f"INDEXER_{name.upper()}_CONFIG_FILE"
    supplied[document] = inline[document]
    with pytest.raises(compose.ComposeError, match=f"{document} and {path} are both provided"):
        compose._validate_document_sources("indexer", supplied, sources)
    del supplied[document], supplied[path]
    with pytest.raises(compose.ComposeError, match=f"{document} or {path} is not provided"):
        compose._validate_document_sources("indexer", supplied, sources)


def test_compose_platform_variable_values() -> None:
    result = subprocess.run(["bash", str(ROOT.parent / "tests" / "railway-values.test.sh")], text=True, capture_output=True)
    assert result.returncode == 0, result.stdout + result.stderr
    assert "PASS railway-values" in result.stdout


def test_compose_metadata_mutual_ingress(tmp_path: Path) -> None:
    values, out = _values(tmp_path), tmp_path / "out"
    result = _run(values, out, "metadata")
    assert result.returncode == 0, result.stderr
    record = json.loads((out / "metadata.json").read_bytes())
    assert set(record) == {
        "DB_BUDGET_DOCUMENT", "DB_BUDGET_SHA256",
        "EXPECTED_SCHEMA_DOCUMENT", "EXPECTED_SCHEMA_SHA256",
        "METADATA_PUBLICATION_CONFIG_DOCUMENT", "METADATA_PUBLICATION_CONFIG_SHA256",
    }
    publication = base64.b64decode(record["METADATA_PUBLICATION_CONFIG_DOCUMENT"], validate=True)
    assert hashlib.sha256(publication).hexdigest() == record["METADATA_PUBLICATION_CONFIG_SHA256"]
    assert json.loads(publication) == {
        "schemaVersion": 3, "environment": "paxeer125-mainnet-beta",
        "envelopeHeader": "X-Kindle-Operation", "signatureHeader": "X-Kindle-Operation-Signature",
    }
    contract = json.loads((ROOT.parent / "railway" / "variables.json").read_bytes())["metadata"]
    assert contract["fixed"]["METADATA_INGRESS"] == "mtls"
    assert set(record) <= set(contract["required"])
    assert {"METADATA_TLS_CERT_PEM", "METADATA_TLS_KEY_PEM", "METADATA_TLS_CLIENT_CA_PEM"} <= set(contract["required"])
    assert {"METADATA_ASSERTION_PRIVATE_KEY_HEX", "METADATA_TLS_KEY_PEM"} <= set(contract["secrets"])
    assert not {"IDENTITY_SERVICE_URL", "IDENTITY_SERVICE_TOKEN"} & set(contract["required"])


@pytest.mark.parametrize("profile", ["container", "platform"])
def test_compose_gateway_binding_forms(tmp_path: Path, profile: str) -> None:
    values, out = _values(tmp_path), tmp_path / "out"
    if profile == "platform":
        (values / "gateway.json").write_bytes((ROOT / "values/gateway.platform.example.json").read_bytes())
    result = _run(values, out, "gateway")
    assert result.returncode == 0, result.stderr
    record = json.loads((out / "gateway.json").read_bytes())
    runtime = json.loads(base64.b64decode(record["GATEWAY_RUNTIME_DOCUMENT"]))
    assert runtime["approvedBucket"] == "token"
    assert runtime["recoverySchema"] == "recovery"
    assert runtime["mediaSchema"] == "gateway"
    assert runtime["metadataSchema"] == "metadata"
    assert runtime["safetySchema"] == "chat"
    assert runtime["control"]["schema"] == runtime["control"]["authoritySchema"] == "pnl"
    assert runtime["audit"]["deliverySchema"] == "gateway_audit_source"
    assert runtime["audit"]["sinkDeliverySchema"] == runtime["audit"]["sinkSchema"] == "gateway_audit_sink"
    for service in ("ranking", "social"):
        assert runtime["recoverySchema"] == json.loads((values / f"{service}.example.json").read_bytes())["recoverySchema"]
    assert {worker["kind"]: (worker["bucket"], worker["prefix"]) for worker in runtime["objectWorkers"]} == {
        "variant": ("media-approved", "approved/"),
        "original": ("media-approved", "quarantine/"),
        "temporary": ("media-approved", "temporary/"),
    }
    routes = {upstream["name"]: upstream["routes"] for upstream in runtime["upstreams"]}
    assert routes["user"] == [{"Method": "GET", "Template": "/v2/me/watchlist", "Authenticate": True, "Optional": False, "Upload": False}]
    assert routes["social"] == [{"Method": "GET", "Template": "/v2/profiles/{wallet}/followers", "Authenticate": False, "Optional": True, "Upload": False}]
    assert routes["metadata"][1] == {"Method": "PUT", "Template": "/v2/tokens/{tokenAddress}/assets/{assetId}", "Authenticate": True, "Optional": False, "Upload": True}
    assert routes["livestream"] == [{"Method": "GET", "Template": "/v2/streams", "Authenticate": False, "Optional": False, "Upload": False}]
    budget = json.loads(base64.b64decode(record["DB_BUDGET_DOCUMENT"]))
    gateway = next(client for client in budget["clients"] if client["role"] == "mediagatewayd")
    assert gateway["transport"] == "direct"
    assert gateway["user"] == "identity_runtime"
    assert "pooler" not in gateway
    assert gateway["maxConnections"] >= runtime["pools"]["identityMaxConnections"] + runtime["pools"]["controlMaxConnections"]


@pytest.mark.parametrize(("path", "value", "key"), [
    (("runtime", "identityEndpoint"), "https://identity.kindlelaunch.example/", "identityEndpoint"),
    (("runtime", "identityEndpoint"), "https://@identity.kindlelaunch.example", "identityEndpoint"),
    (("runtime", "natsUrl"), "tls://nats.kindlelaunch.example:4222/", "natsUrl"),
    (("runtime", "purgeEndpoint"), "https://cdn.kindlelaunch.example/", "purgeEndpoint"),
    (("runtime", "upstreams", 0, "url"), "https://user.kindlelaunch.example/", "url"),
    (("runtime", "upstreams", 0, "routes", 0, "optional"), True, "upstreams[0].routes[0].optional"),
    (("runtime", "upstreams", 1, "routes", 1, "optional"), True, "upstreams[1].routes[1].optional"),
    (("runtime", "upstreams", 2, "routes", 0, "upload"), True, "upstreams[2].routes[0].upload"),
    (("runtime", "upstreams", 1, "routes", 0, "upload"), True, "upstreams[1].routes[0].upload"),
    (("runtime", "upstreams", 0, "routes"), [{"method": "GET", "template": "/v2/me/watchlist", "authenticate": True}] * 2, "upstreams[0].routes[1]"),
    (("runtime", "upstreams", 0, "routes"), [{"method": "GET", "template": "/v2/me/watchlist", "authenticate": True}] * 257, "upstreams[0].routes"),
    (("runtime", "pools", "identityMaxConnections"), 33, "identityMaxConnections"),
    (("runtime", "pools", "controlMaxConnections"), 2147483648, "controlMaxConnections"),
    (("runtime", "control", "schema"), "control", "control.schema"),
    (("runtime", "control", "authoritySchema"), "control", "control.authoritySchema"),
    (("credentials", "deleteOriginalAccess", "reference"), "GATEWAY_DELETE_VARIANT_ACCESS", "objectWorkers[1].deleteAccessKeyCredential"),
    (("credentials", "deleteVariantAccess", "reference"), "S3_ACCESS_KEY_ID", "objectWorkers[0].deleteAccessKeyCredential"),
    (("credentials", "inventoryTemporarySecret", "reference"), "S3_SECRET_ACCESS_KEY", "objectWorkers[2].inventorySecretKeyCredential"),
    (("runtime", "identityPeers", 0, "publicKey"), "A" * 42 + "B", "identityPeers[0].publicKey"),
    (("runtime", "identityPeers", 0, "publicKey"), "A" * 42 + "!", "identityPeers[0].publicKey"),
    (("runtime", "identityListenAddress"), "gateway:9443", "identityListenAddress"),
    (("runtime", "identityListenAddress"), "127.0.0.1:", "identityListenAddress"),
    (("runtime", "identityListenAddress"), "::1:9443", "identityListenAddress"),
    (("runtime", "stream"), "bad stream", "stream"),
    (("runtime", "durable"), "a" * 129, "durable"),
    (("runtime", "audit", "stream"), "bad stream", "audit.stream"),
    (("runtime", "audit", "durable"), "a" * 129, "audit.durable"),
    (("runtime", "audit", "subject"), "audit.*", "audit.subject"),
    (("runtime", "audit", "subject"), "a" * 257, "audit.subject"),
    (("runtime", "consumerGeneration"), "a" * 129, "consumerGeneration"),
    (("runtime", "consumerGeneration"), "é" * 65, "consumerGeneration"),
    (("identity", "keys", 0, "id"), "é" * 33, "keys[0].id"),
    (("runtime", "recoverySchema"), "bad-schema", "recoverySchema"),
    (("runtime", "recoverySchema"), "r", "recoverySchema"),
    (("runtime", "recoveryRegion"), "mainnet_eu", "recoveryRegion"),
    (("runtime", "recoveryEpoch"), 9223372036854775808, "recoveryEpoch"),
    (("runtime", "mediaSchema"), "bad schema", "mediaSchema"),
    (("runtime", "metadataSchema"), "bad-schema", "metadataSchema"),
    (("runtime", "safetySchema"), "A", "safetySchema"),
    (("runtime", "uploadNamespace"), "gateway upload", "uploadNamespace"),
    (("runtime", "uploadConcurrent"), 201, "uploadConcurrent"),
    (("runtime", "uploadBytesPerSecond"), 32767, "uploadBytesPerSecond"),
    (("runtime", "uploadBytesPerSecond"), (1 << 30) + 1, "uploadBytesPerSecond"),
    (("runtime", "audit", "deliverySchema"), "users", "audit.deliverySchema"),
    (("runtime", "audit", "sinkSchema"), "users", "audit.sinkSchema"),
    (("runtime", "audit", "sinkDeliverySchema"), "gateway_audit_source", "audit.sinkDeliverySchema"),
    (("runtime", "audit", "subject"), "identity.account.v1", "audit.subject"),
    (("runtime", "audit", "eventType"), "identity.account.revision", "audit.eventType"),
    (("runtime", "audit", "durable"), "gateway-identity-consumer", "audit.durable"),
    (("runtime", "audit", "deliverySchema"), "bad schema", "audit.deliverySchema"),
    (("runtime", "audit", "sinkDeliverySchema"), "bad schema", "audit.sinkDeliverySchema"),
    (("identity", "allowedOrigins"), ["https://app.kindlelaunch.example/"], "allowedOrigins[0]"),
    (("identity", "allowedOrigins"), ["https://app.kindlelaunch.example"] * 2, "allowedOrigins[1]"),
    (("identity", "keys", 0, "activeAt"), "2026-01-01", "keys[0].activeAt"),
    (("identity", "keys", 0, "retireAt"), "2027-02-30T00:00:00Z", "keys[0].retireAt"),
    (("identity", "keys", 0, "retireAt"), "2027-01-01T00:00:00", "keys[0].retireAt"),
    (("identity", "keys", 0, "activeAt"), "2026-12-31T23:00:00-02:00", "keys[0].retireAt"),
    (("identity", "keys"), [
        {"id": "gateway-2026-01", "credential": "identityKey", "activeAt": "2026-01-01T00:00:00Z", "retireAt": "2027-01-01T00:00:00Z"},
        {"id": "gateway-previous", "credential": "identityKey", "activeAt": "2025-01-01T00:00:00Z", "retireAt": "2026-01-01T00:15:01Z"},
    ], "keys[1].retireAt"),
    (("identity", "domain"), "bad_domain", "domain"),
    (("identity", "domain"), "a" * 64 + ".example", "domain"),
    (("identity", "uri"), "https://other.kindlelaunch.example/auth", "uri"),
    (("identity", "uri"), "https://app.kindlelaunch.example/" + "a" * 2048, "uri"),
])
def test_compose_gateway_refuses_startup_values(tmp_path: Path, path: tuple, value: object, key: str) -> None:
    values = _values(tmp_path)
    source = values / "gateway.example.json"
    document = json.loads(source.read_bytes())
    obj = document
    for part in path[:-1]:
        obj = obj[part]
    obj[path[-1]] = value
    source.write_text(json.dumps(document))
    result = _run(values, tmp_path / "out", "gateway")
    assert result.returncode != 0
    assert f"gateway: invalid value {key}" in result.stderr
    assert not (tmp_path / "out" / "gateway.json").exists()


def test_compose_gateway_preserves_route_list_authority(tmp_path: Path) -> None:
    values, out = _values(tmp_path), tmp_path / "out"
    source = values / "gateway.example.json"
    document = json.loads(source.read_bytes())
    document["runtime"]["upstreams"][0]["routes"] = [{"method": "GET", "template": "/v2/product-route"}]
    source.write_text(json.dumps(document))
    result = _run(values, out, "gateway")
    assert result.returncode == 0, result.stderr
    record = json.loads((out / "gateway.json").read_bytes())
    runtime = json.loads(base64.b64decode(record["GATEWAY_RUNTIME_DOCUMENT"]))
    assert runtime["upstreams"][0]["routes"][0]["Template"] == "/v2/product-route"


def _history_values(values: Path, *, first: int | None = None, last: int | None = None) -> dict:
    data = json.loads((values / "indexer.example.json").read_bytes())
    manifest = json.loads((ROOT.parents[2] / "sdk/launchpad/shared/config/deployment/current.json").read_bytes())
    activation = manifest["activationHeight"]
    data.update(anchor_height=last if last is not None else activation + 20,
                history_from_height=first if first is not None else activation,
                history_operator="launchpad_operator", history_expires_at="2099-01-01T00:00:00Z")
    (values / "indexer.example.json").write_text(json.dumps(data))
    return data


def _history_requests(out: Path) -> tuple[dict, list[dict]]:
    env = json.loads((out / "indexer.json").read_bytes())
    if "INDEXER_LIFECYCLE_CONFIG_DOCUMENT" in env:
        raw = base64.b64decode(env["INDEXER_LIFECYCLE_CONFIG_DOCUMENT"])
    else:
        raw = (out / "indexer-lifecycle.json").read_bytes()
    assert hashlib.sha256(raw).hexdigest() == env["INDEXER_LIFECYCLE_CONFIG_SHA256"]
    lifecycle = json.loads(raw)
    return env, lifecycle.get("BackfillRequests", lifecycle.get("backfillRequests", []))


def _expected_history_sql(requests: list[dict], data: dict) -> list[str]:
    rows = ["BEGIN;"]
    for request in requests:
        evidence = hashlib.sha256(json.dumps(request, sort_keys=True, separators=(",", ":")).encode()).hexdigest()
        rows.append(
            "INSERT INTO indexer.indexer_replay_authorizations "
            "(authorization_id,operator_identity,authorization_evidence,action,target,generation,projection_revision,from_height,to_height,max_concurrent,expires_at,revoked_at) "
            f"VALUES ('{request['AuthorizationID']}','{data['history_operator']}','{evidence}','backfill','indexer','{data['inventory_generation']}','{data['projection_version']}',{request['FromHeight']},{request['ToHeight']},1,'2099-01-01T00:00:00+00:00'::timestamptz,NULL) ON CONFLICT (authorization_id) DO NOTHING;"
        )
    return rows + ["COMMIT;"]


def test_history_below_anchor_absent_preserves_documents(tmp_path: Path) -> None:
    values = _values(tmp_path)
    data = json.loads((values / "indexer.example.json").read_bytes())
    first, second = tmp_path / "first", tmp_path / "second"
    assert _run(values, first, "indexer").returncode == 0
    data.update(history_operator="launchpad_operator", history_expires_at="2099-01-01T00:00:00Z", backfill_batch_size=16)
    (values / "indexer.example.json").write_text(json.dumps(data))
    result = _run(values, second, "indexer")
    assert result.returncode == 0, result.stderr
    assert (first / "indexer.json").read_bytes() == (second / "indexer.json").read_bytes()
    env = json.loads((first / "indexer.json").read_bytes())
    assert not any(name in env for name in ("INDEXER_MODE", "BACKFILL_BATCH_SIZE", "BACKFILL_ONLY", "BACKFILL_FROM_BLOCK"))
    assert not (second / "indexer-history-authorizations.sql").exists()
    life = json.loads(base64.b64decode(env["INDEXER_LIFECYCLE_CONFIG_DOCUMENT"]))
    assert life["ReplayRequests"] == [] and "BackfillRequests" not in life
    receipt = json.loads(base64.b64decode(env["INDEXER_RECEIPT_CONFIG_DOCUMENT"]))
    assert receipt["Production"]["Manifest"]["activationHeight"] == data["anchor_height"]


@pytest.mark.parametrize("bound", ["activation", "anchor"])
def test_history_below_anchor_bounds(tmp_path: Path, bound: str) -> None:
    values = _values(tmp_path)
    data = _history_values(values)
    if bound == "anchor":
        data["history_from_height"] = data["anchor_height"]
        (values / "indexer.example.json").write_text(json.dumps(data))
    out = tmp_path / "out"
    result = _run(values, out, "indexer")
    assert result.returncode == 0, result.stderr
    env, requests = _history_requests(out)
    assert len(requests) == 1
    request = requests[0]
    assert request["FromHeight"] == data["history_from_height"]
    assert request["ToHeight"] == request["CheckpointHeight"] == data["anchor_height"]
    assert request["CheckpointHash"] == data["anchor_hash"]
    assert request["Mode"] == "backfill" and request["Target"] == "indexer"
    assert env["INDEXER_MODE"] == "backfill" and env["BACKFILL_ONLY"] == "false"
    assert env["BACKFILL_FROM_BLOCK"] == str(data["history_from_height"])
    assert env["BACKFILL_BATCH_SIZE"] == str(data["replay_batch_size"])
    contract = json.loads((ROOT.parent / "railway/variables.json").read_bytes())["indexer"]
    for name in ("INDEXER_MODE", "BACKFILL_ONLY", "BACKFILL_FROM_BLOCK", "BACKFILL_BATCH_SIZE"):
        assert contract["optional"][name] is True and name not in contract["required"]
    sql = out / "indexer-history-authorizations.sql"
    assert sql.read_text().splitlines() == _expected_history_sql(requests, data)
    assert stat.S_IMODE(sql.stat().st_mode) == 0o600
    again = tmp_path / "again"
    repeated = _run(values, again, "indexer")
    assert repeated.returncode == 0, repeated.stderr
    assert (out / "indexer.json").read_bytes() == (again / "indexer.json").read_bytes()
    assert sql.read_bytes() == (again / sql.name).read_bytes()


@pytest.mark.parametrize("invalid", ["below", "above", "bool", "negative", "null", "string"])
def test_history_below_anchor_refuses_invalid_height(tmp_path: Path, invalid: str) -> None:
    values = _values(tmp_path)
    data = _history_values(values)
    data["history_from_height"] = {"below": data["history_from_height"] - 1, "above": data["anchor_height"] + 1, "bool": True, "negative": -1, "null": None, "string": "42"}[invalid]
    (values / "indexer.example.json").write_text(json.dumps(data))
    result = _run(values, tmp_path / "out", "indexer")
    assert result.returncode == 2
    assert "history_from_height" in result.stderr
    assert not (tmp_path / "out/indexer.json").exists()
    assert not (tmp_path / "out/indexer-history-authorizations.sql").exists()


@pytest.mark.parametrize("length", [1_000_000, 1_000_001, 2_000_001])
def test_history_below_anchor_splits_requests_and_authorizations(tmp_path: Path, length: int) -> None:
    values = _values(tmp_path)
    data = _history_values(values)
    data["anchor_height"] = data["history_from_height"] + length - 1
    data["backfill_batch_size"] = 64
    (values / "indexer.example.json").write_text(json.dumps(data))
    out = tmp_path / "out"
    result = _run(values, out, "indexer")
    assert result.returncode == 0, result.stderr
    env, requests = _history_requests(out)
    assert len(requests) == (length + 999_999) // 1_000_000
    assert requests[0]["FromHeight"] == data["history_from_height"]
    assert requests[-1]["ToHeight"] == data["anchor_height"]
    for i, request in enumerate(requests):
        assert 1 <= request["ToHeight"] - request["FromHeight"] + 1 <= 1_000_000
        assert request["CheckpointHeight"] == data["anchor_height"]
        assert request["CheckpointHash"] == data["anchor_hash"]
        if i:
            assert request["FromHeight"] == requests[i - 1]["ToHeight"] + 1
    assert len({r["JobID"] for r in requests}) == len(requests)
    assert len({r["AuthorizationID"] for r in requests}) == len(requests)
    assert env["BACKFILL_BATCH_SIZE"] == "64"
    assert (out / "indexer-history-authorizations.sql").read_text().splitlines() == _expected_history_sql(requests, data)


@pytest.mark.parametrize("member,value", [("history_operator", "operator'"), ("history_operator", ""), ("history_expires_at", "2099-01-01"), ("history_expires_at", "tomorrow"), ("backfill_batch_size", 65), ("backfill_batch_size", 0)])
def test_history_below_anchor_refuses_invalid_operation_values(tmp_path: Path, member: str, value: object) -> None:
    values = _values(tmp_path)
    data = _history_values(values)
    data[member] = value
    (values / "indexer.example.json").write_text(json.dumps(data))
    result = _run(values, tmp_path / "out", "indexer")
    assert result.returncode == 2
    assert member in result.stderr


def test_history_below_anchor_rendered_lifecycle_is_bound(tmp_path: Path) -> None:
    values = _values(tmp_path)
    data = _history_values(values)
    # Render the initial source document with the same anchor before selecting history.
    first = data.pop("history_from_height")
    (values / "indexer.example.json").write_text(json.dumps(data))
    rendered, _ = _indexer_rendered_values(values, tmp_path / "initial")
    rendered["history_from_height"] = first
    # The receipt is composed from the reviewed deployment manifest for history.
    del rendered["receipt_rendered_file"], rendered["receipt_runtime_path"]
    (values / "indexer.example.json").write_text(json.dumps(rendered))
    out = tmp_path / "out"
    result = _run(values, out, "indexer")
    assert result.returncode == 0, result.stderr
    env, requests = _history_requests(out)
    assert env["INDEXER_LIFECYCLE_CONFIG_FILE"] == rendered["lifecycle_runtime_path"]
    assert "INDEXER_LIFECYCLE_CONFIG_DOCUMENT" not in env
    assert requests[0]["FromHeight"] == first
    assert (out / "indexer-history-authorizations.sql").read_text().splitlines() == _expected_history_sql(requests, rendered)


@pytest.mark.parametrize("service", SERVICES)
def test_runtime_contract_names_and_forms_cover_image_config(service: str) -> None:
    contract = json.loads((ROOT.parent / "railway/variables.json").read_bytes())[service]
    source = (ROOT.parents[2] / "services/launchpad" / contract["buildArgs"]["SERVICE"] / "internal/config/config.go").read_text()
    if re.search(r"^\s*sharedconfig.BaseEnv\s*$", source, re.M):
        source += (ROOT.parents[2] / "sdk/launchpad/shared/config/env.go").read_text()
    names = set(re.findall(r'env:"([^",]+)', source))
    names.update(contract.get("documentCredentials", {}))
    carried = set(contract["fixed"]) | set(contract["required"]) | set(contract["optional"])
    carried.update(contract.get("documentSources", {}))
    carried.update(contract.get("documentSources", {}).values())
    assert carried == names
    assert set(contract["forms"]) == names
    for document in contract["required"]:
        if document.endswith("_DOCUMENT"):
            path = document.removesuffix("_DOCUMENT") + "_FILE"
            if path in names:
                assert contract["documentSources"][document] == path
    for name in names:
        assert contract["forms"][name]
        if name.endswith("_DOCUMENT"):
            assert contract["forms"][name] == "standard base64 JSON"
        elif name.endswith("_SHA256"):
            assert contract["forms"][name] == "lowercase hexadecimal SHA-256"
    defaults = dict(re.findall(r'env:"([^",]+)[^"]*" envDefault:"([^"]*)"', source))
    assert contract["defaults"] == defaults


@pytest.mark.parametrize("endpoint", [
    "http://indexer.example.com:8080/internal/ranking-evidence",
    "https://indexer.example.com/internal/ranking-evidence",
    "http://127.0.0.1:8080/internal/ranking-evidence",
    "http://[::1]:8080/internal/ranking-evidence",
])
def test_ranking_evidence_endpoint_is_bound(tmp_path: Path, endpoint: str) -> None:
    values, out = _values(tmp_path), tmp_path / "out"
    path = values / "ranking.example.json"
    ranking = json.loads(path.read_bytes())
    ranking["evidenceEndpoint"] = endpoint
    path.write_text(json.dumps(ranking))
    result = _run(values, out, "ranking")
    assert result.returncode == 0, result.stderr
    record = json.loads((out / "ranking.json").read_bytes())
    raw = base64.b64decode(record["RANKING_RUNTIME_DOCUMENT"])
    binding = json.loads(raw)
    assert binding["version"] == 2
    assert binding["evidenceEndpoint"] == endpoint
    assert binding["visibilityEndpoint"] == ranking["visibilityEndpoint"]
    assert record["RANKING_RUNTIME_SHA256"] == hashlib.sha256(raw).hexdigest()


@pytest.mark.parametrize("endpoint", [
    None, "", False, 123,
    "ftp://indexer.example.com/internal/ranking-evidence",
    "http:///internal/ranking-evidence",
    "http://indexer.example.com/internal/ranking-evidence?height=200",
    "http://indexer.example.com/internal/ranking-evidence?",
    "http://indexer.example.com/internal/other",
    "http://indexer.example.com/internal/%72anking-evidence",
    "http://user@indexer.example.com/internal/ranking-evidence",
    "http://indexer.example.com/internal/ranking-evidence#pool",
    "http://indexer.example.com/internal/ranking-evidence#",
    "http://indexer.example.com:65536/internal/ranking-evidence",
    "http://indexer.example.com:/internal/ranking-evidence",
    "http://indexer.example.com /internal/ranking-evidence",
])
def test_ranking_evidence_endpoint_is_required(tmp_path: Path, endpoint: object) -> None:
    values, out = _values(tmp_path), tmp_path / "out"
    path = values / "ranking.example.json"
    ranking = json.loads(path.read_bytes())
    if endpoint is None:
        del ranking["evidenceEndpoint"]
    else:
        ranking["evidenceEndpoint"] = endpoint
    path.write_text(json.dumps(ranking))
    result = _run(values, out, "ranking")
    assert result.returncode != 0
    assert "missing or malformed value evidenceEndpoint" in result.stderr
    assert not (out / "ranking.json").exists()


@pytest.mark.parametrize("service", ["api", "gateway", "ranking", "social"])
def test_composed_credential_references_are_deployment_inputs(tmp_path: Path, service: str) -> None:
    values, out = _values(tmp_path), tmp_path / "out"
    result = _run(values, out, service)
    assert result.returncode == 0, result.stderr
    contract = json.loads((ROOT.parent / "railway/variables.json").read_bytes())[service]
    record = json.loads((out / f"{service}.json").read_bytes())

    def strings(value: object) -> set[str]:
        if isinstance(value, str):
            return {value}
        if isinstance(value, dict):
            return set().union(*(strings(member) for member in value.values()))
        if isinstance(value, list):
            return set().union(*(strings(member) for member in value))
        return set()

    documents = [json.loads(base64.b64decode(value)) for name, value in record.items() if name.endswith("_DOCUMENT")]
    referenced = strings(documents)
    for name in contract["documentCredentials"]:
        assert name in referenced
        assert name in contract["required"]
        assert name in contract["secrets"]
    example = json.loads((values / f"{service}.example.json").read_bytes())
    if service == "gateway":
        expected = {value["reference"] for value in example["credentials"].values()}
    elif service in {"ranking", "social"}:
        expected = {
            value if isinstance(value, str) else value["value"]
            for name, value in example.items() if name.endswith("Reference")
        }
    else:
        expected = {value for name, value in example["discovery_runtime"].items() if name.endswith("_ref")}
    assert set(contract["documentCredentials"]) == expected


def test_composed_database_budget_uses_service_direct_transport(tmp_path: Path) -> None:
    values, out = _values(tmp_path), tmp_path / "out"
    result = _run(values, out, *SERVICES)
    assert result.returncode == 0, result.stderr
    expected = json.loads((values / "common.json").read_bytes())
    assert len(expected["clients"]) == 12
    for service in SERVICES:
        record = json.loads((out / f"{service}.json").read_bytes())
        budget = json.loads(base64.b64decode(record["DB_BUDGET_DOCUMENT"]))
        assert budget == expected
        for client in budget["clients"]:
            assert client["transport"] == "direct"
            assert "pooler" not in client
            assert client["maxConnections"] == (12 if client["role"] == "mediagatewayd" else 1)
    assert len({client["user"] for client in expected["clients"]}) == 12


def test_chart_policy_admits_indexer_market_event(tmp_path: Path) -> None:
    values, out = _values(tmp_path), tmp_path / "out"
    result = _run(values, out, "charts", "indexer")
    assert result.returncode == 0, result.stderr
    docs = {}
    for service in ("charts", "indexer"):
        record = json.loads((out / f"{service}.json").read_bytes())
        docs[service] = json.loads(base64.b64decode(record["DELIVERY_CONFIG_DOCUMENT"]))
    subject = docs["charts"]["consumer"]["Subject"]
    assert subject == docs["indexer"]["Subjects"]["accounting.market.v1"]
    assert docs["charts"]["policy"]["Subjects"][subject] is True
    assert docs["charts"]["policy"]["EventTypes"] == {"accounting.market.v1": True}


@pytest.mark.parametrize(("field", "state"), [
    (None, "enabled"),
    ("Subjects", "missing"), ("EventTypes", "missing"),
    ("Subjects", "disabled"), ("EventTypes", "disabled"),
])
def test_charts_policy_names_market_type(tmp_path: Path, field: str | None, state: str) -> None:
    import charts

    values, out = _values(tmp_path), tmp_path / "out"
    path = values / "charts.example.json"
    document = json.loads(path.read_bytes())
    policy = document["delivery"]["policy"]
    if field is not None:
        del policy[field]["accounting.market.v1"]
        if state == "disabled":
            policy[field]["accounting.market.v1"] = False
    path.write_text(json.dumps(document))
    if field is None:
        composed = charts.documents("charts", values)["DELIVERY_CONFIG_DOCUMENT"]
        assert composed["policy"]["Subjects"]["accounting.market.v1"] is True
        assert composed["policy"]["EventTypes"]["accounting.market.v1"] is True
        result = _run(values, out, "charts")
        assert result.returncode == 0, result.stderr
        record = json.loads((out / "charts.json").read_bytes())
        delivery = json.loads(base64.b64decode(record["DELIVERY_CONFIG_DOCUMENT"]))
        assert delivery == composed
    else:
        message = f"policy.{field} must enable accounting.market.v1"
        with pytest.raises(ValueError, match=re.escape(message)):
            charts.documents("charts", values)
        result = _run(values, out, "charts")
        assert result.returncode == 2
        assert message in result.stderr
        assert not (out / "charts.json").exists()


def test_broker_example_captures_all_api_relay_routes(tmp_path: Path) -> None:
    values, out = _values(tmp_path), tmp_path / "out"
    result = _run(values, out, "api", "api-sockets")
    assert result.returncode == 0, result.stderr
    record = json.loads((out / "api.json").read_bytes())
    relay = json.loads(base64.b64decode(record["STREAM_RELAY_CONFIG_DOCUMENT"]))
    broker = json.loads((ROOT / "values/broker.example.json").read_bytes())
    streams = {stream["name"]: stream for stream in broker["streams"]}
    for route in relay["routes"]:
        stream = streams[route["stream"]]
        assert stream["subjects"] == route["subjects"]
        assert stream["limits"]["replicas"] == 3
        assert stream["limits"]["maxAge"] >= 72 * 60 * 60 * 1_000_000_000
    sockets = json.loads((out / "api-sockets.json").read_bytes())
    consumers = json.loads(base64.b64decode(sockets["STREAM_CONSUMER_CONFIG_DOCUMENT"]))
    assert consumers == [consumer for consumer in broker["durables"] if consumer["Durable"] == "api_sockets_125_stats"]


def _broker_subject_covered(pattern: str, subject: str) -> bool:
    outer, inner = pattern.split("."), subject.split(".")
    for index, token in enumerate(outer):
        if token == ">":
            return index == len(outer) - 1 and len(inner) > index
        if index >= len(inner) or inner[index] == ">":
            return False
        if token != "*" and token != inner[index]:
            return False
    return len(outer) == len(inner)


@pytest.mark.parametrize("pattern,subject,covered", [
    ("a.*", "a.b", True), ("a.*", "a.b.c", False),
    ("a.>", "a", False), ("a.>", "a.b.c", True),
    ("a.>", "a.*", True), ("a.*", "a.>", False),
    ("a.*.>", "a.>", False), ("a.b", "a.*", False),
    ("market.candles.*.*", "market.candles.*.*", True),
])
def test_broker_subject_coverage(pattern: str, subject: str, covered: bool) -> None:
    assert _broker_subject_covered(pattern, subject) is covered


def _broker_example_bindings(value):
    if isinstance(value, list):
        for item in value:
            yield from _broker_example_bindings(item)
    elif isinstance(value, dict):
        fields = {key.replace("_", "").lower(): item for key, item in value.items()}
        for key, stream in fields.items():
            if not key.endswith("stream") or key == "sourcestream":
                continue
            assert isinstance(stream, str) and stream, (key, stream)
            prefix = key[:-len("stream")]
            subjects = []
            for subject_key in (prefix + "subject", prefix + "subjects"):
                if subject_key in fields:
                    named = fields[subject_key]
                    if isinstance(named, dict):
                        subjects.extend(named.values())
                    elif isinstance(named, list):
                        subjects.extend(named)
                    else:
                        subjects.append(named)
            if key == "outputstream":
                subjects.extend(fields[name] for name in ("statssubject", "searchinvalidationsubject") if name in fields)
            if key == "eventstream":
                subjects.extend(fields.get("interactionsubjects", {}).values())
                if "mentionsubject" in fields:
                    subjects.append(fields["mentionsubject"])
            yield stream, subjects
        for item in value.values():
            yield from _broker_example_bindings(item)


def test_broker_example_covers_publishers_and_consumers() -> None:
    examples = {path.name: json.loads(path.read_bytes()) for path in sorted((ROOT / "values").glob("*.example.json"))}
    broker = examples["broker.example.json"]
    streams = {stream["name"]: stream for stream in broker["streams"]}
    assert len(streams) == len(broker["streams"])
    bindings = []
    for name, example in examples.items():
        for stream, subjects in _broker_example_bindings(example):
            bindings.append((name, stream, subjects))
    bindings.append(("user.example.json", "USERS", list(examples["user.example.json"]["subjects"].values())))
    assert examples["pnl.journeys.example.json"]["journeys"]["brokerStream"] == "PNL_JOURNEYS"
    for name, stream, subjects in bindings:
        assert stream in streams, (name, stream)
        for subject in subjects:
            assert any(_broker_subject_covered(pattern, subject) for pattern in streams[stream]["subjects"]), (name, stream, subject)


def test_compose_mounts_and_private_listeners_match_contract() -> None:
    source = (ROOT.parent / "docker-compose.strangler.yml").read_text()
    contracts = json.loads((ROOT.parent / "railway/variables.json").read_bytes())
    for service in SERVICES:
        compose_name = "sockets" if service == "api-sockets" else service
        block = re.search(rf"^  {compose_name}:\n(.*?)(?=^  [a-z-]+:|\Z)", source, re.M | re.S)[1]
        assert f":/run/launchpad/{service}:ro" in block
        for name, value in contracts[service]["fixed"].items():
            if name.endswith("_PRIVATE_LISTEN_ADDRESS"):
                assert f'{name}: "{value}"' in block


@pytest.mark.parametrize("dockerfile", ["Dockerfile.svc", "Dockerfile.native"])
def test_service_image_build_inputs_are_consumed(dockerfile: str) -> None:
    source = (ROOT.parent / dockerfile).read_text()
    names = set(re.findall(r"^ARG ([A-Z_]+)", source, re.M))
    contracts = json.loads((ROOT.parent / "railway/variables.json").read_bytes())
    for service in SERVICES:
        assert set(contracts[service]["buildInputs"]) == names
        assert set(contracts[service]["buildArgs"]) == {"SERVICE", "BIN"}
    assert "FROM ${BUILD_IMAGE} AS build\nARG SOURCE_DATE_EPOCH\n" in source
    assert "SOURCE_DATE_EPOCH=${SOURCE_DATE_EPOCH}" in source
    assert "ARG SOURCE_DATE_EPOCH=0" in source


def test_web_build_contract_carries_consumed_arguments() -> None:
    source = (ROOT.parent / "Dockerfile.web").read_text()
    contract = json.loads((ROOT.parent / "railway/variables.json").read_bytes())["web"]
    names = set(re.findall(r"^ARG ([A-Z_]+)", source, re.M))
    assert set(contract["buildInputs"]) == names
    assert not {"NEXT_PUBLIC_SOCIAL_API_URL", "NEXT_PUBLIC_USER_API_URL"} & names
    assert set(contract["buildArgs"]) | set(contract["required"]) | set(contract["optional"]) == names - {"NODE_IMAGE"}
    app = ROOT.parents[2] / "apps/launchpad/web"
    app_source = "\n".join(path.read_text() for directory in (app / "src", app) for path in directory.glob("*.ts*"))
    app_source += "\n".join(path.read_text() for path in (app / "src").rglob("*.ts*"))
    for name in names - {"NODE_IMAGE"}:
        assert name in app_source, name
    for name in contract["contextEnvironment"]:
        assert name in app_source, name
        assert name in contract["forms"]


def test_livestream_example_emits_runtime_environment(tmp_path: Path) -> None:
    values, out = _values(tmp_path), tmp_path / "out"
    result = _run(values, out, "livestream")
    assert result.returncode == 0, result.stderr
    record = json.loads((out / "livestream.json").read_bytes())
    assert {name: value for name, value in record.items() if not name.endswith(("_DOCUMENT", "_SHA256"))} == {
        "BIND_ADDRESS": "0.0.0.0", "PORT": "8080",
        "RPC_URL": "https://api5.mainnet-beta.paxeer.network/", "ZONE": "mainnet-beta-a",
        "LIVEPEER_API_URL": "https://livepeer.studio/api", "LIVEPEER_API_KEY": "test",
        "MAX_STREAMS_PER_WALLET": "3", "MAX_GLOBAL_STREAMS": "100",
        "SECRETS_REFRESH_SECONDS": "30", "LIVEPEER_WEBHOOK_TIMESTAMP_UNIT": "seconds",
    } | {name: value for name, value in json.loads((values / "livestream.example.json").read_bytes())["environment"].items() if not name.endswith("_SHA256")} | {
        "NODE_ENV": "production", "SECRETS_PROVIDER": "env",
        "LIVESTREAM_PRIVATE_LISTEN_ADDRESS": "",
        "LIVESTREAM_TLS_CERT_FILE": "", "LIVESTREAM_TLS_CERT_PEM": "",
        "LIVESTREAM_TLS_KEY_FILE": "", "LIVESTREAM_TLS_KEY_PEM": "",
        "LIVESTREAM_TLS_CLIENT_CA_FILE": "", "LIVESTREAM_TLS_CLIENT_CA_PEM": "",
    }
    assert "PUBLIC_URL" not in record
    assert record["LIVEPEER_API_KEY"] not in result.stdout.split()


@pytest.mark.parametrize(("member", "value", "name"), [("bind", "::", "BIND_ADDRESS"), ("port", 9090, "PORT")])
def test_livestream_environment_preserves_fixed_values(tmp_path: Path, member: str, value: object, name: str) -> None:
    values = _values(tmp_path)
    path = values / "livestream.example.json"
    data = json.loads(path.read_bytes())
    data["addresses"][member] = value
    path.write_text(json.dumps(data))
    result = _run(values, tmp_path / "out", "livestream")
    assert result.returncode == 2
    assert f"{name} differs from the fixed contract value" in result.stderr


def test_indexer_receipt_discovery_matches_runtime_builder(tmp_path: Path) -> None:
    values, out = _values(tmp_path), tmp_path / "out"
    result = _run(values, out, "indexer")
    assert result.returncode == 0, result.stderr
    record = json.loads((out / "indexer.json").read_bytes())
    raw = base64.b64decode(record["INDEXER_RECEIPT_CONFIG_DOCUMENT"])
    assert hashlib.sha256(raw).hexdigest() == record["INDEXER_RECEIPT_CONFIG_SHA256"]
    receipt = json.loads(raw)["Production"]
    fixture = json.loads((ROOT / "fixtures/indexer-discovery.json").read_bytes())
    assert receipt["Discovery"] == fixture["Discovery"]
    assert [template["Role"] for template in receipt["Discovery"]] == ["SidioraPool", "SidioraERC20"]
    data = json.loads((values / "indexer.example.json").read_bytes())
    assert receipt["Discovery"][1]["RuntimeHash"] == data["erc20_masked_runtime_hash"]


def test_indexer_receipt_requires_masked_token_runtime_hash(tmp_path: Path) -> None:
    import indexer

    values = _values(tmp_path)
    path = values / "indexer.example.json"
    data = json.loads(path.read_bytes())
    del data["erc20_masked_runtime_hash"]
    path.write_text(json.dumps(data))
    with pytest.raises(indexer.DocumentError, match="erc20_masked_runtime_hash"):
        indexer.documents("indexer", values)
    result = _run(values, tmp_path / "out", "indexer")
    assert result.returncode == 2
    assert "erc20_masked_runtime_hash" in result.stderr
    assert not (tmp_path / "out/indexer.json").exists()


@pytest.mark.parametrize("offset", [0, 1])
def test_indexer_receipt_accepts_anchor_from_activation(tmp_path: Path, offset: int) -> None:
    import indexer

    values = _values(tmp_path)
    path = values / "indexer.example.json"
    data = json.loads(path.read_bytes())
    manifest = json.loads((ROOT.parents[2] / "sdk/launchpad/shared/config/deployment/current.json").read_bytes())
    data["anchor_height"] = manifest["activationHeight"] + offset
    data["erc20_masked_runtime_hash"] = "0x" + "ab" * 32
    path.write_text(json.dumps(data))
    receipt = indexer.documents("indexer", values)["INDEXER_RECEIPT_CONFIG_DOCUMENT"]["Production"]
    assert receipt["Anchor"]["Height"] == data["anchor_height"]
    assert receipt["Discovery"][1]["RuntimeHash"] == data["erc20_masked_runtime_hash"]


@pytest.mark.parametrize("value", [
    None, True, 1, [], {}, "", " ", "9f" * 32, "0X" + "9f" * 32,
    "0x" + "9F" * 32, "0x" + "9f" * 31, "0x" + "9f" * 33,
    "0x" + "g" * 64, " 0x" + "9f" * 32, "0x" + "9f" * 32 + "\n",
])
def test_indexer_receipt_refuses_invalid_masked_token_runtime_hash(tmp_path: Path, value: object) -> None:
    import indexer

    values = _values(tmp_path)
    path = values / "indexer.example.json"
    data = json.loads(path.read_bytes())
    data["erc20_masked_runtime_hash"] = value
    path.write_text(json.dumps(data))
    with pytest.raises(indexer.DocumentError, match="erc20_masked_runtime_hash"):
        indexer.documents("indexer", values)
    result = _run(values, tmp_path / "out", "indexer")
    assert result.returncode == 2
    assert "erc20_masked_runtime_hash" in result.stderr
    assert not (tmp_path / "out/indexer.json").exists()


def test_indexer_receipt_refuses_anchor_below_activation(tmp_path: Path) -> None:
    import indexer

    values = _values(tmp_path)
    path = values / "indexer.example.json"
    data = json.loads(path.read_bytes())
    manifest = json.loads((ROOT.parents[2] / "sdk/launchpad/shared/config/deployment/current.json").read_bytes())
    data["anchor_height"] = manifest["activationHeight"] - 1
    path.write_text(json.dumps(data))
    with pytest.raises(indexer.DocumentError, match="anchor_height") as error:
        indexer.documents("indexer", values)
    for height in (data["anchor_height"], manifest["activationHeight"]):
        assert str(height) in str(error.value)
    result = _run(values, tmp_path / "out", "indexer")
    assert result.returncode == 2
    assert "anchor_height" in result.stderr and "activationHeight" in result.stderr
    assert not (tmp_path / "out/indexer.json").exists()


def test_indexer_receipt_and_committed_epochs_use_reviewed_provenance(tmp_path: Path) -> None:
    values, out = _values(tmp_path), tmp_path / "out"
    result = _run(values, out, "indexer")
    assert result.returncode == 0, result.stderr
    record = json.loads((out / "indexer.json").read_bytes())
    receipt = json.loads(base64.b64decode(record["INDEXER_RECEIPT_CONFIG_DOCUMENT"]))["Production"]
    lifecycle = json.loads(base64.b64decode(record["INDEXER_LIFECYCLE_CONFIG_DOCUMENT"]))
    manifest = json.loads((ROOT.parents[2] / "sdk/launchpad/shared/config/deployment/current.json").read_bytes())
    reviewed = json.loads((ROOT.parents[2] / "sdk/launchpad/protocol/catalog/reviewed.json").read_bytes())
    assert receipt["Manifest"] == manifest
    catalog = json.loads(base64.b64decode(receipt["CatalogJSON"]))
    originals = {contract["name"]: contract for contract in reviewed["contracts"]}
    names = {"USDL": "SidioraERC20", "LaunchpadPrecompile": "EventEmitter"}
    wanted = {names.get(role, role) for role in manifest["roles"]} | {"SidioraPool"}
    assert {contract["name"] for contract in catalog["contracts"]} == wanted & originals.keys()
    for contract in catalog["contracts"]:
        assert all(value == originals[contract["name"]][key] for key, value in contract.items())
        abi = json.dumps(contract["abi"], separators=(",", ":")).encode()
        assert hashlib.sha256(abi).hexdigest() == contract["abiSha256"]
    assert len(receipt["Epochs"]) == len(lifecycle["Committed"]["Epochs"]) == 13
    for epoch, committed in zip(receipt["Epochs"], lifecycle["Committed"]["Epochs"]):
        deployment = epoch["Deployment"]
        role = "USDL" if deployment["Role"] == "SidioraERC20" else deployment["Role"]
        assert epoch["Identity"] == manifest["roles"][role]
        assert deployment["ABIRevision"] == originals[names.get(role, role)]["abiSha256"]
        assert deployment["From"]["Block"] == manifest["activationHeight"]
        assert committed == {
            "Address": deployment["Address"].lower(), "Role": deployment["Role"],
            "ABIRevision": deployment["ABIRevision"],
            "ActivationHeight": manifest["activationHeight"],
            "ManifestDigest": lifecycle["Committed"]["ManifestDigest"],
        }
    assert lifecycle["Committed"]["ManifestDigest"] != manifest["sourceManifestSha256"]
    authority = json.loads((values / "network-authority.json").read_bytes())
    native_manifest = json.dumps(authority["manifest"], separators=(",", ":")).encode()
    assert lifecycle["Committed"]["ManifestDigest"] == hashlib.sha256(native_manifest).hexdigest()
    assert lifecycle["Committed"]["Anchor"] == receipt["Anchor"]


@pytest.mark.parametrize("member", ["deployment_id", "manifest_digest", "addresses"])
def test_indexer_values_keep_reviewed_deployment_identity(tmp_path: Path, member: str) -> None:
    values = _values(tmp_path)
    path = values / "indexer.example.json"
    data = json.loads(path.read_bytes())
    if member == "addresses":
        data[member]["router"] = data[member]["quoter"]
    elif member == "manifest_digest":
        data[member] = "0" * 64
    else:
        data[member] = "another-deployment"
    path.write_text(json.dumps(data))
    result = _run(values, tmp_path / "out", "indexer")
    assert result.returncode == 2
    assert "deployment manifest" in result.stderr


@pytest.mark.parametrize(("service", "name", "form"), [
    ("api", "IDENTITY_SIGNING_KEY", "unpadded standard base64 Ed25519 private key (64 bytes)"),
    ("user", "ACCOUNT_CURSOR_KEY", "standard base64 key (32 bytes)"),
    ("stats", "STATS_AUDIT_KEY", "hexadecimal key (32 to 64 bytes)"),
    ("metadata", "METADATA_PUBLICATION_AUDIT_KEY", "standard base64 key (32 bytes)"),
    ("ranking", "TEST_RANKING_CURSOR_KEY", "unpadded base64url key (at least 32 bytes)"),
    ("social", "SOCIAL_CURSOR_KEY", "unpadded base64url key (at least 32 bytes)"),
    ("gateway", "GATEWAY_ASSERTION_KEY", "unpadded base64url Ed25519 private key (64 bytes)"),
    ("livestream", "IDENTITY_ASSERTION_SIGNING_KEY", "standard base64 Ed25519 private key (64 bytes)"),
])
def test_contract_key_representations(service: str, name: str, form: str) -> None:
    contracts = json.loads((ROOT.parent / "railway/variables.json").read_bytes())
    assert contracts[service]["forms"][name] == form


def _livestream_form(values: Path, profile: str) -> dict:
    name = "livestream.platform.example.json" if profile == "platform" else "livestream.example.json"
    selected = json.loads((ROOT / "values" / name).read_bytes())
    (values / "livestream.json").write_text(json.dumps(selected))
    if profile == "platform":
        (values / "gateway.json").write_bytes((ROOT / "values/gateway.platform.example.json").read_bytes())
    return selected


@pytest.mark.parametrize("profile", ["container", "platform"])
def test_livestream_forms_compose_config_reader_settings(tmp_path: Path, profile: str) -> None:
    from urllib.parse import urlparse

    values, out = _values(tmp_path), tmp_path / "out"
    selected = _livestream_form(values, profile)
    result = _run(values, out, "livestream", "gateway", "web")
    assert result.returncode == 0, result.stderr
    record = json.loads((out / "livestream.json").read_bytes())
    web = json.loads((out / "web.json").read_bytes())
    assert record["RUNTIME_PROFILE"] == profile
    assert record["NODE_ENV"] == "production"
    assert record["BIND_ADDRESS"] == "0.0.0.0"
    assert record["PORT"] == "8080"
    assert len(record["LIVESTREAM_VIEWER_TOKEN_KEY"].encode()) >= 32
    assert 0 < int(record["MAX_STREAMS_PER_WALLET"]) <= int(record["MAX_GLOBAL_STREAMS"]) <= 100000
    assert 1 <= int(record["SECRETS_REFRESH_SECONDS"]) <= 60
    for name in ("DB_BUDGET", "EXPECTED_SCHEMA"):
        data = base64.b64decode(record[name + "_DOCUMENT"], validate=True)
        assert hashlib.sha256(data).hexdigest() == record[name + "_SHA256"]
    if profile == "platform":
        assert record["LIVEPEER_PROVIDER_MODE"] == "unavailable"
        assert record["LIVESTREAM_CONNECTED_ENABLED"] == "false"
        assert record["LIVESTREAM_PRIVATE_LISTEN_ADDRESS"] == "[::]:8443"
        assert 1 <= int(record["DB_MAX_CONNS"]) <= 200
        assert re.fullmatch(r"[0-9a-f]{64}", record["GATEWAY_PEER_CERT_SHA256"])
        for part in ("CERT", "KEY", "CLIENT_CA"):
            assert bool(record[f"LIVESTREAM_TLS_{part}_PEM"]) != bool(record[f"LIVESTREAM_TLS_{part}_FILE"])
            assert record[f"LIVESTREAM_TLS_{part}_PEM"] == selected["environment"][f"LIVESTREAM_TLS_{part}_PEM"]
        for name in ("LIVEPEER_API_KEY", "LIVEPEER_WEBHOOK_SECRET", "LIVEPEER_PLAYBACK_WEBHOOK_SECRET", "LIVESTREAM_BUDGET_POLICY_DOCUMENT", "LIVESTREAM_BUDGET_POLICY_FILE", "LIVESTREAM_BUDGET_POLICY_SHA256", "IDENTITY_SERVICE_TOKEN", "LIVESTREAM_VAULT_TRANSIT_TOKEN"):
            assert record[name] == ""
        assert record["LIVEPEER_API_URL"] == "https://livepeer.studio/api"
        assert record["CORS_ALLOWED_ORIGINS"] == "https://sidiora.fun"
        assert web["NEXT_PUBLIC_LIVESTREAM_API_URL"] == web["NEXT_PUBLIC_GATEWAY_URL"]
        gateway = json.loads((out / "gateway.json").read_bytes())
        runtime = json.loads(base64.b64decode(gateway["GATEWAY_RUNTIME_DOCUMENT"]))
        upstream = next(item for item in runtime["upstreams"] if item["name"] == "livestream")
        assert upstream["url"] == "https://livestream.kindlelaunch.example:8443"
        assert str(urlparse(upstream["url"]).port) != record["PORT"]
    else:
        assert record["LIVEPEER_PROVIDER_MODE"] == "available"
        assert record["LIVESTREAM_CONNECTED_ENABLED"] == "true"
        assert record["LIVEPEER_API_URL"] == "https://livepeer.studio/api"
        assert record["LIVEPEER_API_KEY"] == "test"
        for name in ("LIVEPEER_WEBHOOK_SECRET", "ACTION_AUDIT_KEY", "LIVEPEER_PLAYBACK_WEBHOOK_SECRET", "LIVESTREAM_BUDGET_ALERT_KEY"):
            assert len(record[name].encode()) >= 32
        assert len(record["LIVESTREAM_VAULT_TRANSIT_TOKEN"]) >= 16
        for name in ("LIVESTREAM_VAULT_TRANSIT_URL", "IDENTITY_SERVICE_URL"):
            parsed = urlparse(record[name])
            assert parsed.scheme == "https" and parsed.netloc and not parsed.path
            assert not parsed.username and not parsed.query and not parsed.fragment
        for name in ("LIVEPEER_PLAYBACK_WEBHOOK_URL", "LIVESTREAM_BUDGET_ALERT_URL"):
            assert urlparse(record[name]).scheme == "https"
        assert urlparse(record["LIVESTREAM_INGEST_ENDPOINT"]).scheme in {"https", "rtmps"}
        assert record["LIVEPEER_WEBHOOK_TIMESTAMP_UNIT"] in {"seconds", "milliseconds"}
        for name in ("LIVESTREAM_PRIVATE_LISTEN_ADDRESS", "LIVESTREAM_TLS_CERT_FILE", "LIVESTREAM_TLS_CERT_PEM", "LIVESTREAM_TLS_KEY_FILE", "LIVESTREAM_TLS_KEY_PEM", "LIVESTREAM_TLS_CLIENT_CA_FILE", "LIVESTREAM_TLS_CLIENT_CA_PEM", "GATEWAY_PEER_CERT_SHA256"):
            assert record[name] == ""
        budget = base64.b64decode(record["LIVESTREAM_BUDGET_POLICY_DOCUMENT"], validate=True)
        assert hashlib.sha256(budget).hexdigest() == record["LIVESTREAM_BUDGET_POLICY_SHA256"]
        assert json.loads(budget) == selected["budget_policy"]
        assert web["NEXT_PUBLIC_LIVESTREAM_API_URL"] == "https://live.sidiora.fun"


@pytest.mark.parametrize("profile, member, value, message", [
    ("platform", "NEXT_PUBLIC_LIVESTREAM_API_URL", "https://live.sidiora.fun", "NEXT_PUBLIC_LIVESTREAM_API_URL must equal NEXT_PUBLIC_GATEWAY_URL"),
    ("container", "LIVESTREAM_CONNECTED_ENABLED", "false", "LIVEPEER_PROVIDER_MODE=available requires LIVESTREAM_CONNECTED_ENABLED=true"),
    ("container", "LIVESTREAM_CONNECTED_ENABLED", None, "LIVEPEER_PROVIDER_MODE=available requires LIVESTREAM_CONNECTED_ENABLED=true"),
    ("platform", "LIVESTREAM_CONNECTED_ENABLED", "true", "RUNTIME_PROFILE=platform requires LIVEPEER_PROVIDER_MODE=unavailable and LIVESTREAM_CONNECTED_ENABLED=false"),
    ("platform", "LIVEPEER_PROVIDER_MODE", "available", "RUNTIME_PROFILE=platform requires LIVEPEER_PROVIDER_MODE=unavailable and LIVESTREAM_CONNECTED_ENABLED=false"),
    ("platform", "LIVESTREAM_PRIVATE_LISTEN_ADDRESS", "[::]:8080", "LIVESTREAM_PRIVATE_LISTEN_ADDRESS must be distinct from PORT"),
    ("platform", "LIVESTREAM_PRIVATE_LISTEN_ADDRESS", "livestream:8443", "LIVESTREAM_PRIVATE_LISTEN_ADDRESS must be an IP:port"),
    ("platform", "LIVESTREAM_TLS_KEY_PEM", "", "exactly one of LIVESTREAM_TLS_KEY_FILE and LIVESTREAM_TLS_KEY_PEM"),
    ("platform", "LIVESTREAM_TLS_CERT_FILE", "/run/launchpad/livestream/cert.pem", "exactly one of LIVESTREAM_TLS_CERT_FILE and LIVESTREAM_TLS_CERT_PEM"),
    ("platform", "GATEWAY_PEER_CERT_SHA256", "A" * 64, "GATEWAY_PEER_CERT_SHA256 must be lowercase hex sha256"),
    ("platform", "LIVESTREAM_VIEWER_TOKEN_KEY", "short", "LIVESTREAM_VIEWER_TOKEN_KEY must be at least 32 bytes"),
    ("platform", "DB_MAX_CONNS", "201", "DB_MAX_CONNS must be in 1..200"),
    ("platform", "CORS_ALLOWED_ORIGINS", "http://sidiora.fun", "invalid value origin"),
])
def test_livestream_forms_refuse_invalid_settings(tmp_path: Path, profile: str, member: str, value: object, message: str) -> None:
    values = _values(tmp_path)
    selected = _livestream_form(values, profile)
    target = selected["web"] if member.startswith("NEXT_PUBLIC_") else selected["environment"]
    if value is None:
        del target[member]
    else:
        target[member] = value
    (values / "livestream.json").write_text(json.dumps(selected))
    result = _run(values, tmp_path / "out", "livestream", "gateway", "web")
    assert result.returncode == 2
    assert message in result.stderr
    assert not (tmp_path / "out").exists()


def test_platform_gateway_refuses_plain_livestream_upstream(tmp_path: Path) -> None:
    values = _values(tmp_path)
    _livestream_form(values, "platform")
    path = values / "gateway.json"
    gateway = json.loads(path.read_bytes())
    next(item for item in gateway["runtime"]["upstreams"] if item["name"] == "livestream")["url"] = "https://livestream.kindlelaunch.example:8080"
    path.write_text(json.dumps(gateway))
    result = _run(values, tmp_path / "out", "gateway")
    assert result.returncode == 2
    assert "livestream upstream must use LIVESTREAM_PRIVATE_LISTEN_ADDRESS port" in result.stderr


def _pnl_journey_values(values: Path) -> dict:
    source = json.loads((ROOT / "values/pnl.journeys.example.json").read_bytes())
    (values / "pnl.json").write_text(json.dumps(source))
    return source


def test_compose_pnl_journey_documents_and_environment(tmp_path: Path) -> None:
    import pnl

    values, baseline = _values(tmp_path), tmp_path / "baseline"
    result = _run(values, baseline, "pnl")
    assert result.returncode == 0, result.stderr
    original = json.loads((baseline / "pnl.json").read_bytes())
    source = _pnl_journey_values(values)
    assert {name: value for name, value in source.items() if name != "journeys"} == json.loads(
        (ROOT / "values/pnl.example.json").read_bytes()
    )
    out = tmp_path / "journeys"
    result = _run(values, out, *SERVICES)
    assert result.returncode == 0, result.stderr
    record = json.loads((out / "pnl.json").read_bytes())
    assert set(record) == set(original) | {
        "PNL_JOURNEY_CONFIG_DOCUMENT", "PNL_JOURNEY_CONFIG_SHA256", "IDENTITY_SERVICE_URL", "PNL_FONT_DIR",
    }
    assert {name: record[name] for name in original} == original
    raw = base64.b64decode(record["PNL_JOURNEY_CONFIG_DOCUMENT"], validate=True)
    assert hashlib.sha256(raw).hexdigest() == record["PNL_JOURNEY_CONFIG_SHA256"]
    assert json.loads(raw) == source["journeys"] | {
        "version": 3, "identityIssuer": "pnld", "identityAudience": "identity",
        "fontRegularSha256": pnl.FONT_REGULAR_SHA256, "fontBoldSha256": pnl.FONT_BOLD_SHA256,
        "fontLicenseSha256": pnl.FONT_LICENSE_SHA256, "rewardManifestDigest": pnl.REWARD_MANIFEST_DIGEST,
    }
    document = json.loads(raw)
    assert record["IDENTITY_SERVICE_URL"] == document["identityEndpoint"]
    assert record["PNL_FONT_DIR"] == document["fontDirectory"]
    assert record["PNL_JOURNEY_CONFIG_DOCUMENT"] not in result.stdout


def test_compose_pnl_journey_variable_key_preserves_other_members(tmp_path: Path) -> None:
    values = _values(tmp_path)
    source = _pnl_journey_values(values)
    file_out = tmp_path / "file"
    result = _run(values, file_out, "pnl")
    assert result.returncode == 0, result.stderr
    original = json.loads((file_out / "pnl.json").read_bytes())
    expected = json.loads(base64.b64decode(original["PNL_JOURNEY_CONFIG_DOCUMENT"], validate=True))
    for member in ("identityClientKeyFile", "identityClientKeySha256"):
        assert expected[member] == source["journeys"][member]
        del source["journeys"][member]
        expected[member] = ""
    (values / "pnl.json").write_text(json.dumps(source))
    variable_out = tmp_path / "variable"
    result = _run(values, variable_out, "pnl")
    assert result.returncode == 0, result.stderr
    record = json.loads((variable_out / "pnl.json").read_bytes())
    raw = base64.b64decode(record["PNL_JOURNEY_CONFIG_DOCUMENT"], validate=True)
    assert json.loads(raw) == expected
    assert record["PNL_JOURNEY_CONFIG_SHA256"] == hashlib.sha256(raw).hexdigest()
    assert {name: value for name, value in record.items() if name not in {
        "PNL_JOURNEY_CONFIG_DOCUMENT", "PNL_JOURNEY_CONFIG_SHA256",
    }} == {name: value for name, value in original.items() if name not in {
        "PNL_JOURNEY_CONFIG_DOCUMENT", "PNL_JOURNEY_CONFIG_SHA256",
    }}


def test_compose_pnl_without_journeys_preserves_document_bytes(tmp_path: Path) -> None:
    import pnl

    values, out = _values(tmp_path), tmp_path / "out"
    result = _run(values, out, "pnl")
    assert result.returncode == 0, result.stderr
    record = json.loads((out / "pnl.json").read_bytes())
    assert set(record) == {
        "DB_BUDGET_DOCUMENT", "DB_BUDGET_SHA256", "EXPECTED_SCHEMA_DOCUMENT", "EXPECTED_SCHEMA_SHA256",
        "DELIVERY_CONFIG_DOCUMENT", "DELIVERY_CONFIG_SHA256",
    }
    expected = {
        "schema": "pnl", "sourceStream": "chain125-canonical", "consumerId": "pnl_inventory",
        "generation": "g1", "owner": "pnl-1", "projectionVersion": "inventory-v1", "epoch": 1,
        "region": "mainnet-beta", "consumer": {
            "Stream": "KINDLE_INDEXER_V1", "Durable": "pnl_inventory", "Subject": "accounting.inventory.v1",
            "Batch": 16, "MaxAckPending": 32, "MaxWaiting": 1, "MaxDeliver": 5,
            "FetchWait": 250_000_000, "AckWait": 30_000_000_000,
        },
    }
    expected_bytes = json.dumps(expected, sort_keys=True, separators=(",", ":")).encode()
    assert base64.b64decode(record["DELIVERY_CONFIG_DOCUMENT"], validate=True) == expected_bytes
    assert record["DELIVERY_CONFIG_SHA256"] == hashlib.sha256(expected_bytes).hexdigest()
    assert pnl.environment("pnl", values) == {}


@pytest.mark.parametrize("member,value", [
    ("unexpected", "private-input-value"), ("realm", "unknown-realm-value"),
    ("version", 3), ("identityIssuer", "private-issuer-value"),
    ("liveBatch", 0), ("liveBatch", 33), ("liveBatch", True),
    ("nodeIds", ["same", "same"]), ("nodeIds", ["", "archive"]), ("nodeIds", ["primary"]),
    ("nodeIds", ["é" * 129, "archive"]), ("nodeIds", [1, "archive"]),
    ("releaseDigest", "A" + "0" * 63), ("interfaceDigest", "0" * 63),
    ("identityPeerCertSha256", "0" * 63), ("identityCaSha256", "A" * 64),
    ("identityClientCertSha256", "g" * 64), ("identityClientKeySha256", "0x" + "0" * 64),
    ("fontDirectory", "relative-directory-value"), ("fontDirectory", "/"),
    ("fontDirectory", "/run/\ud800"),
    ("exportsDirectory", "/run/../private"), ("exportsDirectory", "/run/private/"),
    ("identityCaFile", "relative-file-value"), ("identityClientCertFile", "/run//cert.pem"),
    ("identityClientKeyFile", "/run/./key.pem"), ("brokerStream", "invalid stream value"),
    ("identityEndpoint", "http://identity.kindlelaunch.example"),
    ("identityEndpoint", "https://identity.kindlelaunch.example/path"),
    ("identityEndpoint", "https://identity.kindlelaunch.example?"),
    ("cdnOrigin", "https://cdn.kindlelaunch.example/path"),
    ("cdnOrigin", "https://user@cdn.kindlelaunch.example"),
    ("cdnOrigin", "https://cdn.kindlelaunch.example#"), ("cdnOrigin", "https://[invalid"),
    ("cdnOrigin", "https://cdn.kindlelaunch.example:invalid"),
    ("cdnZone", "a" * 31), ("cdnZone", "A" * 32),
    ("rewardRuntimeHashes", {}), ("rewardRuntimeHashes", {"private-address-value": "0x" + "0" * 64}),
    ("programs", {}), ("approvedCommunityWallets", {}),
    ("approvedCommunityWallets", ["0x" + "0" * 40]),
    ("approvedCommunityWallets", ["0x" + "a" * 40, "0x" + "A" * 40]),
    ("approvedCommunityWallets", [f"0x{number:040x}" for number in range(1, 66)]),
])
def test_compose_pnl_refuses_invalid_journey_member(tmp_path: Path, member: str, value: object) -> None:
    values = _values(tmp_path)
    source = _pnl_journey_values(values)
    source["journeys"][member] = value
    (values / "pnl.json").write_text(json.dumps(source))
    out = tmp_path / "out"
    result = _run(values, out, "pnl")
    assert result.returncode == 2
    kind = "unknown" if member in {"unexpected", "version", "identityIssuer"} else "missing" if value == {} and member == "rewardRuntimeHashes" else "invalid"
    assert re.fullmatch(rf"pnl: {kind} value journeys\.{member}(\[[0-9]+\])?", result.stderr.strip())
    assert not (out / "pnl.json").exists()


@pytest.mark.parametrize("member", sorted(json.loads((ROOT / "values/pnl.journeys.example.json").read_bytes())["journeys"]))
def test_compose_pnl_refuses_missing_journey_member(tmp_path: Path, member: str) -> None:
    values = _values(tmp_path)
    source = _pnl_journey_values(values)
    del source["journeys"][member]
    (values / "pnl.json").write_text(json.dumps(source))
    result = _run(values, tmp_path / "out", "pnl")
    assert result.returncode == 2
    assert result.stderr.strip() == f"pnl: missing value journeys.{member}"


@pytest.mark.parametrize("value", [None, [], "private-journey-value"])
def test_compose_pnl_refuses_nonobject_journeys(tmp_path: Path, value: object) -> None:
    values = _values(tmp_path)
    source = _pnl_journey_values(values)
    source["journeys"] = value
    (values / "pnl.json").write_text(json.dumps(source))
    result = _run(values, tmp_path / "out", "pnl")
    assert result.returncode == 2
    assert result.stderr.strip() == "pnl: invalid value journeys"


@pytest.mark.parametrize("digest", ["0x" + "A" * 64, "0" * 64, None])
def test_compose_pnl_refuses_invalid_reward_runtime_hash(tmp_path: Path, digest: object) -> None:
    values = _values(tmp_path)
    source = _pnl_journey_values(values)
    hashes = source["journeys"]["rewardRuntimeHashes"]
    hashes[next(iter(hashes))] = digest
    (values / "pnl.json").write_text(json.dumps(source))
    result = _run(values, tmp_path / "out", "pnl")
    assert result.returncode == 2
    assert result.stderr.strip() == "pnl: invalid value journeys.rewardRuntimeHashes"


def _pnl_program() -> dict:
    return {
        "ID": "campaign", "Version": "v1", "DeploymentID": "deployment", "Asset": "0x85fcd13735f4309833a503ee804ea32395851479",
        "ReferralRaw": "1000000", "CashbackStartHeight": "1", "CashbackEndHeight": "2", "CashbackBPS": 100,
        "AwardCapRaw": "1000000", "Treasury": "", "Custody": "", "Adapter": "", "AdapterCodeHash": "",
        "DailyCapRaw": "1000000", "BeneficiaryCapRaw": "1000000",
    }


@pytest.mark.parametrize("member,value", [
    ("ID", "invalid id value"), ("Version", ""), ("DeploymentID", "a" * 129),
    ("Asset", "0x" + "a" * 40), ("ReferralRaw", "01"), ("AwardCapRaw", str(1 << 256)),
    ("DailyCapRaw", -1), ("BeneficiaryCapRaw", "1.0"),
    ("CashbackBPS", 10001), ("CashbackBPS", True),
    ("CashbackStartHeight", "0"),
    ("CashbackEndHeight", "0"), ("CashbackEndHeight", str(1 << 64)),
    ("Treasury", "not-an-address-value"), ("Custody", "0x" + "A" * 40),
    ("Adapter", "invalid adapter value"), ("AdapterCodeHash", "A" * 64),
    ("unexpected", "private-program-value"),
])
def test_compose_pnl_refuses_invalid_program_member(tmp_path: Path, member: str, value: object) -> None:
    values = _values(tmp_path)
    source = _pnl_journey_values(values)
    program = _pnl_program()
    program[member] = value
    source["journeys"]["programs"] = [program]
    (values / "pnl.json").write_text(json.dumps(source))
    result = _run(values, tmp_path / "out", "pnl")
    assert result.returncode == 2
    kind = "unknown" if member == "unexpected" else "invalid"
    assert result.stderr.strip() == f"pnl: {kind} value journeys.programs[0].{member}"


@pytest.mark.parametrize("member", sorted(_pnl_program()))
def test_compose_pnl_refuses_missing_program_member(tmp_path: Path, member: str) -> None:
    values = _values(tmp_path)
    source = _pnl_journey_values(values)
    program = _pnl_program()
    del program[member]
    source["journeys"]["programs"] = [program]
    (values / "pnl.json").write_text(json.dumps(source))
    result = _run(values, tmp_path / "out", "pnl")
    assert result.returncode == 2
    assert result.stderr.strip() == f"pnl: missing value journeys.programs[0].{member}"


@pytest.mark.parametrize("batch", [1, 32])
def test_compose_pnl_journey_policies_and_bounds(tmp_path: Path, batch: int) -> None:
    values, out = _values(tmp_path), tmp_path / "out"
    source = _pnl_journey_values(values)
    journeys = source["journeys"]
    journeys.update(realm="private_fixture", liveBatch=batch, cdnZone="a" * 32)
    program = _pnl_program()
    program.update(Treasury="0x" + "a" * 40, Custody="0x" + "b" * 40, Adapter="settlement-v1", AdapterCodeHash="0x" + "a" * 64)
    journeys["programs"] = [program]
    journeys["approvedCommunityWallets"] = [f"0x{number:040x}" for number in range(1, 65)]
    (values / "pnl.json").write_text(json.dumps(source))
    result = _run(values, out, "pnl")
    assert result.returncode == 0, result.stderr
    record = json.loads((out / "pnl.json").read_bytes())
    document = json.loads(base64.b64decode(record["PNL_JOURNEY_CONFIG_DOCUMENT"]))
    assert all(document[member] == value for member, value in journeys.items())


def test_compose_pnl_refuses_duplicate_programs(tmp_path: Path) -> None:
    values = _values(tmp_path)
    source = _pnl_journey_values(values)
    source["journeys"]["programs"] = [_pnl_program(), _pnl_program()]
    (values / "pnl.json").write_text(json.dumps(source))
    result = _run(values, tmp_path / "out", "pnl")
    assert result.returncode == 2
    assert result.stderr.strip() == "pnl: invalid value journeys.programs[1]"


@pytest.mark.parametrize("member", ["PNL_JOURNEY_CONFIG_DOCUMENT", "PNL_JOURNEY_CONFIG_SHA256"])
def test_compose_optional_document_requires_contract_pair(tmp_path: Path, member: str) -> None:
    import compose

    values = _values(tmp_path)
    _pnl_journey_values(values)
    entry = compose._contract()["pnl"]
    del entry["optional"][member]
    with pytest.raises(compose.ComposeError, match="PNL_JOURNEY_CONFIG_DOCUMENT is not required by the contract"):
        compose._compose_service("pnl", values, entry)


def test_stats_social_policy_subscription(tmp_path: Path) -> None:
    values, out = _values(tmp_path), tmp_path / "out"
    result = _run(values, out, "stats")
    assert result.returncode == 0, result.stderr
    environment = json.loads((out / "stats.json").read_bytes())
    document = json.loads(base64.b64decode(environment["DELIVERY_CONFIG_DOCUMENT"]))
    social = json.loads((values / "social.example.json").read_bytes())
    subscriptions = document["phase2"]["searchConsumers"]
    policy = next(sub for sub in subscriptions if sub["kind"] == "social_policy")
    assert policy["eventType"] == "social.policy.changed.v2"
    assert policy["consumer"]["Stream"] == social["policyStream"]
    assert policy["consumer"]["Subject"] == social["policySubject"]
    assert document["policy"]["Subjects"][social["policySubject"]] is True
    assert document["policy"]["EventTypes"][policy["eventType"]] is True
    assert len({sub["consumer"]["Durable"] for sub in subscriptions}) == len(subscriptions)
    path = values / "stats.example.json"
    document["policy"]["EventTypes"]["profile.updated"] = True
    policy["eventType"] = "profile.updated"
    path.write_text(json.dumps(document))
    refused = _run(values, tmp_path / "wrong-event", "stats")
    assert refused.returncode == 2
    assert "eventType" in refused.stderr
    document["phase2"]["searchConsumers"].remove(policy)
    path.write_text(json.dumps(document))
    optional = _run(values, tmp_path / "optional", "stats")
    assert optional.returncode == 0, optional.stderr


def test_stats_account_profile_subscriptions(tmp_path: Path) -> None:
    values, out = _values(tmp_path), tmp_path / "out"
    result = _run(values, out, "stats")
    assert result.returncode == 0, result.stderr
    environment = json.loads((out / "stats.json").read_bytes())
    raw = base64.b64decode(environment["DELIVERY_CONFIG_DOCUMENT"])
    assert hashlib.sha256(raw).hexdigest() == environment["DELIVERY_CONFIG_SHA256"]
    document = json.loads(raw)
    subjects = json.loads((values / "user.example.json").read_bytes())["subjects"]
    subscriptions = document["phase2"]["searchConsumers"]
    profiles = [sub for sub in subscriptions if sub["kind"] == "account_profile"]
    expected = {
        "profile.updated": "stats_search_profile_updated",
        "account.deletion.requested": "stats_search_account_deletion_requested",
        "account.backfill.completed": "stats_search_account_backfill_completed",
    }
    assert len(profiles) == len(expected)
    assert {sub["eventType"] for sub in profiles} == set(expected)
    assert len({(sub["consumer"]["Stream"], sub["consumer"]["Durable"])
                for sub in subscriptions}) == len(subscriptions)
    broker = json.loads((ROOT / "values/broker.example.json").read_bytes())
    streams = {stream["name"]: stream for stream in broker["streams"]}
    for sub in profiles:
        event, consumer = sub["eventType"], sub["consumer"]
        assert consumer["Stream"] == "USERS"
        assert consumer["Durable"] == expected[event]
        assert consumer["Subject"] == subjects[event]
        assert subjects[event] in streams["USERS"]["subjects"]
        assert document["policy"]["Subjects"][subjects[event]] is True
        assert document["policy"]["EventTypes"][event] is True
        assert [durable for durable in broker["durables"]
                if (durable["Stream"], durable["Durable"]) ==
                (consumer["Stream"], consumer["Durable"])] == [consumer]


def test_stats_account_profile_rejects_other_event_type(tmp_path: Path) -> None:
    values = _values(tmp_path)
    path = values / "stats.example.json"
    document = json.loads(path.read_bytes())
    index, profile = next((index, sub) for index, sub in
                          enumerate(document["phase2"]["searchConsumers"])
                          if sub["kind"] == "account_profile")
    profile["eventType"] = "preferences.updated"
    document["policy"]["EventTypes"]["preferences.updated"] = True
    path.write_text(json.dumps(document))
    refused = _run(values, tmp_path / "wrong-event", "stats")
    assert refused.returncode == 2
    assert f"phase2.searchConsumers.{index}.eventType" in refused.stderr


@pytest.mark.parametrize("policy_member", ["Subjects", "EventTypes"])
def test_stats_account_profile_requires_policy(tmp_path: Path, policy_member: str) -> None:
    values = _values(tmp_path)
    path = values / "stats.example.json"
    document = json.loads(path.read_bytes())
    profile = next(sub for sub in document["phase2"]["searchConsumers"]
                   if sub["kind"] == "account_profile")
    member = profile["consumer"]["Subject"] if policy_member == "Subjects" else profile["eventType"]
    del document["policy"][policy_member][member]
    path.write_text(json.dumps(document))
    refused = _run(values, tmp_path / "missing-policy", "stats")
    assert refused.returncode == 2
    assert "phase2.searchConsumers" in refused.stderr
