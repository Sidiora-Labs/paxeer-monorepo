"""Doc-13 source/schema proof; no subprocess, Rust or Elixir runtime required.

Validate real committed generator inputs and health transcripts in-process.
The registered Rust integration cases exercise the production client separately;
this gate makes no claim of running that toolchain or a deployed backend.
"""

import ast
from copy import deepcopy
import json
from pathlib import Path
import re
import shlex
import tomllib

from jsonschema import Draft4Validator, ValidationError
import pytest
import yaml


ROOT = Path(__file__).resolve().parents[3]
CLIENT = ROOT / "services/explorer/services/libs/blockscout-client"
BACKEND = ROOT / "services/explorer/backend/apps/block_scout_web/lib/block_scout_web"
RECORD = Path(__file__).with_name("repair_22_33_360.kvx")


def source(relative):
    return (CLIENT / relative).read_text(encoding="utf-8")


def document():
    return " ".join(source("README.md").split())


def load_yaml(relative):
    return yaml.safe_load(source(relative))


def nullable_schema(value):
    """Translate OpenAPI 3.0 nullable into JSON Schema's null union."""
    if isinstance(value, list):
        return [nullable_schema(item) for item in value]
    if not isinstance(value, dict):
        return value
    result = {key: nullable_schema(item) for key, item in value.items() if key != "nullable"}
    return {"anyOf": [result, {"type": "null"}]} if value.get("nullable") else result


def validator(spec, status):
    schema = spec["paths"]["/api/health"]["get"]["responses"][str(status)]["content"]["application/json"]["schema"]
    result = nullable_schema(spec)
    result.update(nullable_schema(schema))
    return Draft4Validator(result)


def test_doc_13_recording_command_matches_real_recorder_and_route():
    recording_section = source("README.md").split("### Recording responses\n", 1)[1]
    commands = re.findall(r"```sh\n(.*?)\n```", recording_section, re.S)
    assert len(commands) == 1
    args = shlex.split(commands[0])
    assert args == [
        "yarn", "record-stubs", "http://127.0.0.1:4001",
        "FULL_RUNNING_BACKEND_GIT_SHA", "/private/recorded-empty",
        "/api/health", "/api/v2/blocks",
    ]
    package = json.loads(source("package.json"))
    assert package["scripts"]["record-stubs"] == "ts-node ts-src/record-stubs.ts"
    recorder = source("ts-src/record-stubs.ts")
    for fact in (
        "await recordResponses(origin, revision, output, paths)",
        "['/api/health', '/api/v2/blocks']",
        "await fetch(url, { redirect: 'error'", "const body = await response.text()",
        "status: response.status", "source_revision: sourceRevision",
        "url: url.pathname + url.search", "flag: 'wx'",
    ):
        assert fact in recorder
    router = (BACKEND / "routers/api_router.ex").read_text()
    assert re.search(r'scope "/health" do\s+get\("/", BlockScoutWeb.API.HealthController, :health\)', router)
    assert 'scope "/api/health" do' in (BACKEND / "health_router.ex").read_text()
    assert "/api/v1/health" not in commands[0]


def test_doc_13_health_inputs_match_current_contract_without_legacy_operation():
    overlay = load_yaml("swaggers/overrides/002_health.yaml")
    merged = load_yaml("swaggers/blockscout-api-final.yaml")
    assert set(overlay["paths"]) == {"/api/health"}
    assert merged["paths"]["/api/health"] == overlay["paths"]["/api/health"]
    assert set(merged["paths"]["/api/health"]["get"]["responses"]) == {"200", "500"}
    assert "/api/v1/health" not in merged["paths"]
    assert all(item.get("get", {}).get("operationId") != "HealthV1" for item in merged["paths"].values())
    for name, schema in overlay["components"]["schemas"].items():
        assert merged["components"]["schemas"][name] == schema, name
    schemas = merged["components"]["schemas"]
    # Celestia's distinct /api/v2/health operation shares these legacy schemas.
    # Keep that unrelated operation and its referenced models intact.
    upstream = load_yaml("swaggers/blockscout-api-v2.yaml")
    assert merged["paths"]["/api/v2/health"] == upstream["paths"]["/api/v2/health"]
    # Health edits must preserve the adjacent response examples and HTTP 400.
    transfers = "/api/v2/transactions/{transaction_hash}/token-transfers"
    assert merged["paths"][transfers] == upstream["paths"][transfers]
    assert merged["paths"]["/api/v2/health"]["get"]["responses"]["200"]["content"]["application/json"]["schema"] == {
        "$ref": "#/components/schemas/v1HealthCheckResponse"
    }
    # The committed document and generated models carry fields absent from the
    # upstream snapshot. Check their serialized contract (gotcha 3008).
    field_schemas = {
        "bool": {"type": "boolean"},
        "String": {"type": "string"},
        "models::HealthCheckResponseServingStatus": {
            "$ref": "#/components/schemas/HealthCheckResponseServingStatus"
        },
        "models::V1Data": {"$ref": "#/components/schemas/v1Data"},
    }
    for name, model_name in (
        ("v1HealthCheckResponse", "v1_health_check_response"),
        ("v1Data", "v1_data"),
    ):
        model = source(f"crate/src/models/{model_name}.rs")
        assert re.search(r"#\[derive\([^)]*\bSerialize\b", model)
        fields = re.findall(
            r"#\[serde\(default\)\]\s+pub (\w+): Option<([^>]+)>,", model
        )
        assert fields and len(fields) == len(re.findall(r"^\s*pub \w+:", model, re.M))
        assert schemas[name]["type"] == "object"
        assert schemas[name]["properties"] == {
            field: field_schemas[rust_type] for field, rust_type in fields
        }, name
    latest = schemas["LatestBlock"]
    assert latest["required"] == ["cache", "db", "node"]
    for name in latest["required"]:
        fields = latest["properties"][name]["properties"]
        assert "number" in fields and "mber" not in fields
        assert fields["number"]["type"] == "string"
    assert "latest_block" not in schemas["HealthCheckResponseMetadata"]["properties"]
    assert schemas["HealthCheckResponseMetadata"]["required"] == ["blocks"]
    doc = document()
    for stale in ("still uses the old", "must be aligned before regeneration", "remains in the generator override"):
        assert stale not in doc
    for fact in (
        "`metadata.blocks.latest_block`", "`metadata.batches`", "`Error::UnsupportedCapability`",
        "`HealthError::Status500`", "`UnhealthyHealthCheckResponse`", "empty strings",
        "HTTP 200", "HTTP 500", "`try_as_success`", "`try_parse_as_success_model`",
    ):
        assert fact in doc


def test_doc_13_documented_example_and_committed_lifecycle_validate():
    spec = load_yaml("swaggers/blockscout-api-final.yaml")
    examples = re.findall(r"```json\n(.*?)\n```", source("README.md"), re.S)
    assert len(examples) == 1
    healthy = json.loads(examples[0])
    validator(spec, 200).validate(healthy)
    assert healthy["healthy"] is True
    assert set(healthy["metadata"]["blocks"]["latest_block"]) == {"db", "cache", "node"}
    recording = json.loads(source("crate/tests/recorded/eth_blockscout_com/_api_health.json"))
    assert recording["request"] == {"method": "GET", "urlPath": "/api/health"}
    assert recording["response"]["status"] == 200
    validator(spec, 200).validate(json.loads(recording["response"]["body"]))
    cases = json.loads(source("crate/tests/health/repair_22_31_269.json"))
    assert [case["case"] for case in cases] == ["healthy", "no_blocks", "stale", "rollup", "media_worker"]
    for case in cases:
        validator(spec, case["status"]).validate(case["body"])
        assert case["body"]["healthy"] is (case["status"] == 200)
        if case["status"] == 500:
            assert case["body"]["error"] == case["body"]["metadata"]["blocks"]["error"]
    # Required fields must be enforced, rather than silently dropping drift.
    for path in (("metadata", "blocks"), ("metadata", "blocks", "latest_block", "node"),
                 ("metadata", "blocks", "latest_block", "cache", "number")):
        bad = deepcopy(healthy)
        parent = bad
        for key in path[:-1]:
            parent = parent[key]
        del parent[path[-1]]
        with pytest.raises(ValidationError):
            validator(spec, 200).validate(bad)
    bad = deepcopy(cases[1]["body"])
    del bad["error"]
    with pytest.raises(ValidationError):
        validator(spec, 500).validate(bad)


def test_doc_13_controller_and_generated_consumer_preserve_documented_fields():
    controller = (BACKEND / "controllers/api/health_controller.ex").read_text()
    for fact in (
        "blocks: indexing_status.blocks", "Map.put(:healthy, indexing_status.blocks.healthy)",
        "Map.get(blocks_property, :error)", "health_status_with_error\n      |> Jason.encode!()",
        "if Map.get(health_status, :healthy) do\n        :ok\n      else\n        500",
        '"healthy" => true,\n        "data" => %{}',
        "number: to_string(health_status[:health_latest_block_number_from_db])",
        "number: to_string(health_status[:health_latest_block_number_from_cache])",
        "number: to_string(health_status[:health_latest_block_number_from_node])",
    ):
        assert fact in controller
    api = source("crate/src/apis/health_api.rs")
    assert 'format!("{}/api/health", local_var_configuration.rest_origin()?)' in api
    assert "local_var_client.execute(local_var_req).await?" in api
    assert "super::decode_success(" in api
    assert "Status500(Box<models::UnhealthyHealthCheckResponse>)" in api
    assert "pub fn try_as_success" in api and "pub fn try_parse_as_success_model" in api
    legacy = api.split("async fn health_v1(&self)")[2].split("/// struct", 1)[0]
    assert "Err(Error::UnsupportedCapability(" in legacy
    assert ".execute(" not in legacy
    fields = {
        "health_check_response": ["pub healthy: bool", "pub metadata: Option<models::HealthCheckResponseMetadata>"],
        "health_check_response_metadata": ["pub blocks: models::BlockIndexingStatus"],
        "block_indexing_status": ["pub latest_block: models::LatestBlock", "pub error: Option<models::HealthCheckError>"],
        "latest_block": ["pub cache: models::LatestBlockCache", "pub db: models::LatestBlockDb", "pub node: models::LatestBlockNode"],
        "latest_block_cache": ["pub number: String", "pub timestamp: String"],
        "latest_block_db": ["pub number: String", "pub timestamp: String"],
        "latest_block_node": ["pub number: String"],
        "unhealthy_health_check_response": ["pub metadata: models::HealthCheckResponseMetadata", "pub error: models::HealthCheckError"],
    }
    for name, required in fields.items():
        model = source(f"crate/src/models/{name}.rs")
        for field in required:
            assert field in model


def test_doc_13_real_client_integration_is_registered_and_disclosed():
    manifest = tomllib.loads(source("crate/Cargo.toml"))
    targets = [item for item in manifest["test"] if item["name"] == "repair_22_33_360"]
    assert targets == [{"name": "repair_22_33_360", "path": "../../tests/repair_22_33_360.rs"}]
    rust = (CLIENT / "crate" / targets[0]["path"]).read_text()
    record = tomllib.loads(RECORD.read_text())
    cases = re.findall(r"#\[tokio::test\]\s*async fn (\w+)\(", rust)
    assert set(cases) == set(record["finding"]["doc-13"]["rust_tests"])
    assert len(cases) == 3
    for fact in (
        'TcpListener::bind("127.0.0.1:0")', "ApiClient::new(", "client.health_api().health().await",
        "response.try_as_success()", "response.try_parse_as_success_model()",
        "Error::ResponseError(response)", "HealthError::Status500(model)",
        'assert_eq!(response.content, body)', "assert_preserved(&produced",
        "client.health_api().health_v1().await", "Error::UnsupportedCapability(_)",
        "_api_health.json", "health/repair_22_31_269.json",
    ):
        assert fact in rust
    assert "it does not execute Rust or Elixir" in document()
    assert "**upstream samples**" in source("README.md")
    assert "source-derived controller lifecycle cases" in document()


def test_doc_13_repair_record_maps_finding_and_exact_evidence_policy():
    record = tomllib.loads(RECORD.read_text())
    task = record["task"]["22"]["33"]["360"]
    finding = record["finding"]["doc-13"]
    audit = tomllib.loads((ROOT / "spec/layerx-merge/programme/audit/records/22.30.139.kvx").read_text())
    original = audit["finding"]["doc-13"]
    for field in ("file", "class", "severity", "expected", "owner_domain"):
        assert finding[field] == original[field]
    assert finding["audit"] == "22.30.139"
    tests = {node.name for node in ast.parse(Path(__file__).read_text()).body
             if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")}
    assert set(finding["tests"]) == tests
    assert task["wiring_requests"] == []
    assert "No pre-change verification was run" in task["baseline"]
    assert "not executed" in task["runtime_limits"]
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in task["verified_revision"]
    assert task["build_cmd"] == "python3 -m py_compile tools/qualification/audit_repairs/test_repair_22_33_360.py"
    assert task["verify_cmd"] == "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_360.py"
    assert json.loads(source("package.json"))["scripts"]["test:doc-13"] == (
        "python3 -m pytest -q ../../../../../tools/qualification/audit_repairs/test_repair_22_33_360.py"
    )
