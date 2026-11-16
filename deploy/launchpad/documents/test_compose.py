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


def test_compose_gateway_binding_forms(tmp_path: Path) -> None:
    values, out = _values(tmp_path), tmp_path / "out"
    result = _run(values, out, "gateway")
    assert result.returncode == 0, result.stderr
    record = json.loads((out / "gateway.json").read_bytes())
    runtime = json.loads(base64.b64decode(record["GATEWAY_RUNTIME_DOCUMENT"]))
    assert runtime["approvedBucket"] == "token"
    assert runtime["recoverySchema"] == "recovery"
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
