"""Source and schema-contract proofs; no TypeScript, Rust or backend runtime.

The Python overlay evaluator specifies the declared replacement/removal contract.
Production merge wiring is checked separately in the TypeScript source. Recorded
health JSON is checked against the resulting schema, not a fabricated response.
"""

from copy import deepcopy
import json
from pathlib import Path
import re
import tomllib

from jsonschema import Draft4Validator
import yaml


ROOT = Path(__file__).resolve().parents[3]
CLIENT = ROOT / "services/explorer/services/libs/blockscout-client"
OVERRIDES = CLIENT / "swaggers/overrides"
BACKEND = ROOT / "services/explorer/backend/apps/block_scout_web/lib/block_scout_web"
RECORD = Path(__file__).with_name("repair_22_33_358.kvx")
REQUIRED = {
    "AddressParam": ["hash", "private_tags", "public_tags", "watchlist_names"],
    "Block": ["hash", "parent_hash", "height"],
    "TokenInfoDetailed": ["address_hash"],
    "TokenInfo": ["address_hash"],
    "TotalERC20": None,
    "TotalERC1155": ["token_id"],
    "TokenBalance": None,
    "NFTInstance": ["id"],
    "StatsResponse": None,
    "AddressNFTInstance": ["token_type", "id"],
    "Token": ["address_hash"],
    "AddressNFTInstanceCollection": ["token_type", "id"],
    "SearchResultToken": ["address_hash", "address_url"],
    "SearchResultAddressOrContract": ["address_hash", "url"],
    "SearchResultBlock": ["block_hash", "timestamp", "url"],
    "SearchResultTransaction": ["timestamp", "transaction_hash", "url"],
}


def load_yaml(path):
    return yaml.safe_load(path.read_text())


def overlay_contract(base, patch):
    """Independent data contract: objects merge, arrays/scalars replace."""
    if isinstance(base, dict) and isinstance(patch, dict):
        result = deepcopy(base)
        for key, value in patch.items():
            result[key] = overlay_contract(result.get(key), value)
        return result
    return deepcopy(patch)


def remove_markers(value):
    if isinstance(value, dict):
        return {
            key: remove_markers(item)
            for key, item in value.items()
            if item != "__REMOVE__"
        }
    if isinstance(value, list):
        return [remove_markers(item) for item in value]
    return value


def merged_contract():
    spec = load_yaml(CLIENT / "swaggers/blockscout-api-v2.yaml")
    for path in sorted(OVERRIDES.glob("*.yaml")):
        spec = overlay_contract(spec, load_yaml(path))
    return remove_markers(spec)


def walk(value):
    yield value
    if isinstance(value, dict):
        for item in value.values():
            yield from walk(item)
    elif isinstance(value, list):
        for item in value:
            yield from walk(item)


def typescript_function(source, name):
    match = re.search(
        rf"^(?:async )?function {name}\([^\n]*\)(?:: [^{{\n]+)? \{{", source, re.M
    )
    assert match is not None, f"missing declared function {name}"
    end = re.search(r"^\}", source[match.end():], re.M)
    assert end is not None
    return source[match.start():match.end() + end.end()]


def json_schema(value):
    """Translate OpenAPI 3.0 nullable for the in-process JSON Schema consumer."""
    if isinstance(value, list):
        return [json_schema(item) for item in value]
    if not isinstance(value, dict):
        return value
    result = {key: json_schema(item) for key, item in value.items() if key != "nullable"}
    if value.get("nullable") is True:
        return {"anyOf": [result, {"type": "null"}]}
    return result


def test_health_v1_dead_operation_is_retired():
    health = load_yaml(OVERRIDES / "002_health.yaml")
    assert set(health["paths"]) == {"/api/health"}
    operation = health["paths"]["/api/health"]["get"]
    assert operation["operationId"] == "Health"
    assert operation["tags"] == ["Health"]
    assert set(operation["responses"]) == {"200", "500"}
    assert "v1Data" not in health["components"]["schemas"]
    assert "v1HealthCheckResponse" not in health["components"]["schemas"]
    merged = merged_contract()
    assert "/api/v1/health" not in merged["paths"]
    assert all(
        node.get("operationId") != "HealthV1"
        for node in walk(merged) if isinstance(node, dict)
    )


def test_health_producer_transport_and_recorded_response_match_overlay():
    router = (BACKEND / "health_router.ex").read_text()
    assert 'scope "/api/health" do' in router
    assert 'get("/", BlockScoutWeb.API.HealthController, :health)' in router
    api_router = (BACKEND / "routers/api_router.ex").read_text()
    assert re.search(
        r'scope "/health" do\s+get\("/", BlockScoutWeb.API.HealthController, :health\)',
        api_router,
    )
    controller = (BACKEND / "controllers/api/health_controller.ex").read_text()
    assert 'put_resp_content_type("application/json")' in controller
    assert "health_status_with_error\n      |> Jason.encode!()" in controller
    consumer = (CLIENT / "crate/src/apis/health_api.rs").read_text()
    assert 'format!("{}/api/health", local_var_configuration.rest_origin()?)' in consumer
    assert "local_var_client.request(reqwest::Method::GET, local_var_uri_str.as_str())" in consumer
    assert "super::decode_success(" in consumer
    assert "HealthSuccess::Status200" in consumer
    spec = merged_contract()
    operation = spec["paths"]["/api/health"]["get"]
    recorded = json.loads(
        (CLIENT / "crate/tests/recorded/eth_blockscout_com/_api_health.json").read_text()
    )
    assert recorded["request"] == {"method": "GET", "urlPath": "/api/health"}
    assert recorded["response"]["status"] == 200
    assert recorded["response"]["headers"]["Content-Type"] == "application/json"
    body = json.loads(recorded["response"]["body"])
    response_schema = operation["responses"]["200"]["content"]["application/json"]["schema"]
    document = json_schema(spec)
    document.update(json_schema(response_schema))
    Draft4Validator(document).validate(body)
    assert body["healthy"] is True
    assert body["metadata"]["blocks"]["healthy"] is True


def test_openapi_null_required_uses_real_property_lists_and_explicit_removal():
    patches = load_yaml(OVERRIDES / "003_fix_types.yaml")["components"]["schemas"]
    assert {name for name, patch in patches.items() if "required" in patch} == set(REQUIRED)
    schemas = merged_contract()["components"]["schemas"]
    for name, expected in REQUIRED.items():
        if expected is None:
            assert patches[name]["required"] == "__REMOVE__"
            assert "required" not in schemas[name]
        else:
            assert patches[name]["required"] == expected
            assert schemas[name]["required"] == expected
            assert set(expected) <= set(schemas[name]["properties"])
    for node in walk(schemas):
        if isinstance(node, dict) and "required" in node:
            required = node["required"]
            assert isinstance(required, list) and required
            assert all(isinstance(name, str) for name in required)
            assert len(required) == len(set(required))
    assert "__REMOVE__" not in json.dumps(schemas)
    created_contract = schemas["Transaction"]["properties"]["created_contract"]
    assert "$ref" not in created_contract
    assert created_contract["nullable"] is True
    assert created_contract["allOf"] == [{"$ref": "#/components/schemas/AddressParam"}]


def test_merge_overrides_wires_array_replacement_removal_and_validation_before_write():
    source = (CLIENT / "ts-src/merge-overrides.ts").read_text()
    assert "import { mergeWith } from 'lodash';" in source
    replace = typescript_function(source, "replaceArrays")
    assert re.search(r"if \(Array\.isArray\(source\)\) \{\s+return source\.slice\(\);", replace)
    assert re.search(r"\}\s+return undefined;\s*\}$", replace)
    cleanup = typescript_function(source, "removeMarkedFields")
    assert "return obj.map(item => removeMarkedFields(item));" in cleanup
    assert re.search(r"if \(value === '__REMOVE__'\) \{\s+continue;", cleanup)
    assert "result[key] = removeMarkedFields(value);" in cleanup
    validation = typescript_function(source, "validateRequiredProperties")
    for guard in (
        "!Array.isArray(required)", "required.length === 0",
        "required.some(name => typeof name !== 'string')",
        "new Set(required).size !== required.length",
        "!Object.prototype.hasOwnProperty.call(properties, name)",
    ):
        assert guard in validation
    assert validation.count("throw new Error(") == 2
    assert "validateRequiredProperties(property, `${location}/properties/${name}`)" in validation
    assert "['items', 'additionalProperties', 'allOf', 'anyOf', 'oneOf', 'not']" in validation
    merge = typescript_function(source, "mergeOverrides")
    ordered = (
        "fs.readFileSync(MAIN_SPEC_PATH, 'utf8')",
        "fs.readdirSync(OVERRIDES_DIR)", "overrideFiles.sort()",
        "parse(overrideContent)", "mergeWith(mergedSpec, overrideSpec, replaceArrays)",
        "removeMarkedFields(mergedSpec)", "Object.entries(mergedSpec.components.schemas)",
        "validateRequiredProperties(schema as JsonValue, `#/components/schemas/${name}`)",
        "stringify(mergedSpec)", "fs.writeFileSync(OUTPUT_PATH, mergedYaml)",
    )
    positions = [merge.index(statement) for statement in ordered]
    assert positions == sorted(positions)
    assert "process.exit(1)" in merge
    assert source.rstrip().endswith("mergeOverrides();")


def test_repair_record_maps_findings_and_out_of_scope_wiring():
    record = tomllib.loads(RECORD.read_text())
    findings = record["finding"]
    assert set(findings) == {"health-v1-dead-operation", "openapi-null-required"}
    for finding in findings.values():
        for test in finding["tests"]:
            assert callable(globals()[test])
    assert record["repair"]["22.33.358"]["source_revision"] == "8a5302e1b13ed685ab2cd21b29cd69052c2a8f6a"
    for request in record["wiring"].values():
        assert request.startswith("Domain 14:")
        assert "services/explorer/services/libs/blockscout-client/" in request
