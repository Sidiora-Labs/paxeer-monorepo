"""Offline OpenRPC contract proof using the committed production gateway answer.

Rust/TypeScript runtime execution and additional capture coverage are owner wiring
requests in the repair record; source checks do not claim those gates ran here.
"""

import ast
from copy import deepcopy
import hashlib
import json
from pathlib import Path
import re
import tomllib

from jsonschema import Draft7Validator, ValidationError
import pytest


ROOT = Path(__file__).resolve().parents[3]
GATEWAY = ROOT / "services/platform/hosted/gateway"
RECORD = Path(__file__).with_name("repair_22_33_1443.kvx")
CAPTURE = ROOT / "apps/web/e2e/fixtures/px-get-unified-history.json"


def source(relative):
    return (ROOT / relative).read_text(encoding="utf-8")


def methods():
    document = json.loads((GATEWAY / "openrpc.json").read_text())
    return {method["name"]: method for method in document["methods"]}


def result_schema(name):
    return methods()[name]["result"]["schema"]


def recorded_answer():
    raw = CAPTURE.read_bytes()
    assert hashlib.sha256(raw).hexdigest() == (
        "0d069680c781ebacc146c8e41debaa52dfa9dba137907283794a394b5ff75aef"
    )
    answer = json.loads(raw)
    assert set(answer) == {"id", "jsonrpc", "result"}
    assert answer["jsonrpc"] == "2.0" and answer["id"] == 1
    assert len(answer["result"]["items"]) == 4
    return answer


def rust_function(filename, name):
    text = (GATEWAY / filename).read_text()
    start = re.search(r"\bfn " + re.escape(name) + r"\s*\(", text)
    assert start is not None, (filename, name)
    # Top-level functions and their nested blocks have distinct indentation.
    return text[start.start():text.index("\n}", start.start()) + 2]


def closed_paths(schema, path=()):
    if schema.get("additionalProperties") is False:
        yield path
    for key, child in schema.get("properties", {}).items():
        yield from closed_paths(child, (*path, key))
    if "items" in schema:
        yield from closed_paths(schema["items"], (*path, "[]"))


def recorded_closed_objects(schema, value, path=()):
    if isinstance(value, dict):
        if schema.get("additionalProperties") is False:
            yield path, schema, value
        for key, child in schema.get("properties", {}).items():
            if key in value:
                yield from recorded_closed_objects(child, value[key], (*path, key))
    elif isinstance(value, list) and "items" in schema:
        for item in value:
            yield from recorded_closed_objects(schema["items"], item, (*path, "[]"))


def test_publication_and_manifest_describe_the_production_method_inventory():
    document = json.loads((GATEWAY / "openrpc.json").read_text())
    manifest = tomllib.loads((GATEWAY / "Cargo.toml").read_text())
    assert manifest["package"]["name"] == "layerx-platform-gateway"
    assert manifest["package"]["description"] == (
        "Hosted Paxeer JSON-RPC gateway for account, balance, asset and indexed EVM history reads"
    )
    assert manifest["bin"] == [{"name": "layerx-gateway", "path": "src/main.rs"}]
    library = (GATEWAY / "src/lib.rs").read_text()
    inventory = library.split("pub const ROUTER_METHODS:", 1)[1].split("];", 1)[0]
    names = re.findall(r'"([a-zA-Z_]+)"', inventory)
    assert len(names) == len(set(names)) == 17
    assert len(document["methods"]) == 22
    inference = {
        "px_getInferenceRooms", "px_getInferenceRoom", "px_getInferenceDeal",
        "px_getInferencePeer", "px_getInferenceReceipts",
    }
    assert set(names) == set(methods()) - inference
    assert set(methods()) - set(names) == inference
    node = source("rpc/server.go")
    assert node.count('Service: &pxInferenceAPI{account: pxAPI}') == 2
    dispatch = rust_function("src/rpc.rs", "dispatch")
    assert 'error(&id, -32601, "Method not found")' in dispatch
    description = document["info"]["description"]
    for retired in ("authenticated activity submission", "scoped subscriptions", "native core reads"):
        assert retired not in description
    assert "Use native signed transactions and chain reads" in description
    assert "subject to each method's authentication and backend requirements" in description
    assert document["openrpc"] == "1.3.2"
    for method in document["methods"]:
        assert method["paramStructure"] == "by-position"
        Draft7Validator.check_schema(method["result"]["schema"])
        for parameter in method["params"]:
            Draft7Validator.check_schema(parameter["schema"])


def test_registration_documents_tenant_binding_and_the_current_pin():
    method = methods()["px_register"]
    description = method["description"]
    registration = (GATEWAY / "src/rpc_register.rs").read_text()
    configured = rust_function("src/rpc_register.rs", "configured_tenant")
    assert 'std::env::var("LAYERX_GATEWAY_TENANT")' in configured
    assert 'registration requires trusted LAYERX_GATEWAY_TENANT' in configured
    assert "LAYERX_GATEWAY_TENANT" in description
    assert "operator configures the trusted tenant" in description
    assert "no positional argument or request header selects a tenant" in description
    assert "<tenant>.<first 16 bytes" in description
    for domain in ("layerx-register-binding-v1", "layerx-register-subject-v1"):
        assert domain in description and f'b"{domain}"' in registration
    tagged = rust_function("src/rpc_register.rs", "tagged")
    assert "for part in [domain, tenant.as_bytes(), signer_public_key.as_slice()]" in tagged
    assert "hash.update((part.len() as u64).to_be_bytes());" in tagged
    assert "hash.update(part);" in tagged
    subject = rust_function("src/rpc_register.rs", "subject")
    assert '"{tenant}.{}"' in subject and "[..32]" in subject
    assert "&binding(tenant, signer_public_key)" in rust_function("src/rpc_register.rs", "proven")
    register = rust_function("src/rpc_register.rs", "register")
    assert "let sub = subject(tenant, &signer_public_key);" in register
    for path in ("/v1/principals", "/v1/signup", "/v1/signup/verify"):
        assert f'"{path}"' in register
    assert '"tenant": tenant' in register
    assert [p["name"] for p in method["params"]] == [
        "signer_public_key", "registration_signature", "email_or_verification_token"
    ]
    assert [p["required"] for p in method["params"]] == [True, True, False]
    for parameter, size in zip(method["params"], (64, 128)):
        assert parameter["schema"] == {"type": "string", "pattern": f"^[0-9a-f]{{{size}}}$"}
    assert "vfy_" in method["params"][2]["description"]


def test_history_selection_transport_and_consumers_match_the_documentation():
    served = rust_function("src/history.rs", "served")
    assert '(json!(key.clone()), vec![("paxeer", key)])' in served
    assert 'Ok((document, key)) => (document, vec![("paxeer", key)])' in served
    assert "super::paxeer::history_account(node, id, &account)" in served
    assert "let result = match history(source, registry, &accounts, &query, unified)" in served
    assert 'json!({"jsonrpc": "2.0", "id": id, "result": result})' in served
    assert "public_core" not in served and "accounts.push" not in served
    resolver = rust_function("src/paxeer.rs", "history_account")
    assert "resolve(node, &selector)" in resolver
    assert "evm::address_hex(&address)" in resolver
    assert "Selector::NativeAccount(_) => Err(-32602)" in rust_function(
        "src/paxeer.rs", "history_selector"
    )
    assert 'code == UNBOUND_DID' in rust_function("src/paxeer.rs", "history_refusal")
    page = rust_function("src/history.rs", "page")
    assert '&format!("/v1/history/{}", encode(account))' in page
    assert 'let tx_hash = text(row, "tx_id").ok_or_else(invalid)?.to_owned();' in page
    assert 'object.insert("tx_hash".to_owned(), json!(tx_hash));' in page
    transport = (GATEWAY / "src/history.rs").read_text()
    for fact in ("client.get_with_query(endpoint, path, query)", "http::loopback_get(",
                 "serde_json::from_slice(&upstream.body)"):
        assert fact in transport
    dispatch = rust_function("src/rpc.rs", "dispatch")
    assert 'super::history::dispatch(config, method, &id, value.get("params"))' in dispatch
    route = rust_function("src/rpc.rs", "route")
    assert "serde_json::from_slice::<Value>(&request.body)" in route
    assert "dispatch(config, request, &value)" in route and "json_response(200, &value)" in route
    web = source("apps/web/src/api/gateway.ts")
    sdk = source("sdk/wallet/src/endpoint.ts")
    assert 'historyPage(await rpc("px_getUnifiedHistory", historyParams(lowered, options)), true)' in web
    assert "txId: textOf(value.tx_id)" in web
    assert "await this.rpc.call('px_getUnifiedHistory', [address, cursor, query.limit ?? null, null])" in sdk
    assert "account: page.account.evm_address" in sdk
    for name in ("px_getHistory", "px_getUnifiedHistory"):
        description = methods()[name]["description"]
        assert "oldest first" in description
        assert "(blockHeight, txIndex, logIndex)" in description
        assert "tx_hash exactly equal to tx_id" in description
        assert "newest first" not in description
    unified = methods()["px_getUnifiedHistory"]["description"]
    assert "reads only its EVM address" in unified
    assert "no public core account listing is queried" in unified
    assert "An unbound DID is refused with -32602" in unified


def test_real_recorded_unified_answer_validates_every_closed_history_object():
    schema = result_schema("px_getUnifiedHistory")
    result = recorded_answer()["result"]
    Draft7Validator(schema).validate(result)
    closed = list(recorded_closed_objects(schema, result))
    assert {path for path, _, _ in closed} == set(closed_paths(schema))
    assert len(set(closed_paths(schema))) == 7
    assert result["account"]["bound"] is True
    assert result["accounts"] == [{
        "side": "paxeer", "account": result["account"]["evm_address"]
    }]
    assert all(row["side"] == "paxeer" for row in result["items"])
    for row in result["items"]:
        assert row["tx_hash"] == row["tx_id"]
        assert row["account"] == result["accounts"][0]["account"]
    properties = schema["properties"]
    assert properties["accounts"]["minItems"] == properties["accounts"]["maxItems"] == 1
    assert properties["accounts"]["items"]["properties"]["side"]["enum"] == ["paxeer"]
    assert properties["items"]["items"]["properties"]["side"]["enum"] == ["paxeer"]
    account_schema = deepcopy(result_schema("px_getAccount")["properties"]["account"])
    actual = deepcopy(properties["account"])
    actual.pop("description")
    account_schema.pop("description", None)
    assert actual == account_schema
    producer = (GATEWAY / "src/history.rs").read_text()
    assert "fn unified_history_answer_matches_the_web_fixture()" in producer
    assert "assert_eq!(recorded, answer);" in producer
    assert 'apps/web/e2e/fixtures/px-get-unified-history.json' in producer
    assert 'fixtures/px-get-unified-history.json' in source("apps/web/e2e/gateway-history.test.ts")


def test_plain_history_contract_matches_recorded_fields_and_producer():
    # Inspect the real recording's fields; do not invent a second wire capture.
    plain = result_schema("px_getHistory")
    unified = result_schema("px_getUnifiedHistory")
    result = recorded_answer()["result"]
    assert plain["additionalProperties"] is False
    assert set(plain["required"]) == set(plain["properties"]) == set(result) - {"accounts"}
    for field in plain["properties"]:
        if field == "account":
            Draft7Validator(plain["properties"][field]).validate(result["accounts"][0]["account"])
        elif field != "items":
            assert plain["properties"][field] == unified["properties"][field]
            Draft7Validator(plain["properties"][field]).validate(result[field])
    row_schema = plain["properties"]["items"]["items"]
    unified_row = deepcopy(unified["properties"]["items"]["items"])
    unified_row["properties"].pop("side")
    unified_row["required"].remove("side")
    assert row_schema == unified_row
    assert row_schema["additionalProperties"] is False
    for row in result["items"]:
        assert set(row) - {"side"} <= set(row_schema["properties"])
        assert set(row_schema["required"]) <= set(row) - {"side"}
        for field, field_schema in row_schema["properties"].items():
            if field in row:
                Draft7Validator(field_schema).validate(row[field])
    producer = (GATEWAY / "src/history.rs").read_text()
    assert "fn answer_tags_every_unified_history_row_with_side_and_no_plain_history_row()" in producer
    assert 'row.remove("side");' in producer
    assert "assert_eq!(*plain, untagged);" in producer
    prior = (GATEWAY / "tests/repair_22_31_202.rs").read_text()
    assert 'for method in ["px_getHistory", "px_getUnifiedHistory"]' in prior
    assert 'matches_schema(&answer["result"], &schema)' in prior
    assert "closed history consumer DTO" in prior


def test_closed_history_contracts_reject_extra_missing_and_malformed_fields():
    schema = result_schema("px_getUnifiedHistory")
    result = recorded_answer()["result"]
    for path, closed_schema, value in recorded_closed_objects(schema, result):
        extra = deepcopy(value)
        extra["undeclared"] = True
        with pytest.raises(ValidationError):
            Draft7Validator(closed_schema).validate(extra)
        for required in closed_schema["required"]:
            missing = deepcopy(value)
            del missing[required]
            with pytest.raises(ValidationError):
                Draft7Validator(closed_schema).validate(missing)
    for name in ("px_getHistory", "px_getUnifiedHistory"):
        row = result_schema(name)["properties"]["items"]["items"]
        assert "tx_hash" in row["required"]
        assert row["properties"]["tx_hash"]["pattern"] == "^0x[0-9a-f]{64}$"
        assert row["additionalProperties"] is False
        for invalid in (None, 7, "0x1234", "0x" + "g" * 64, "f" * 64):
            with pytest.raises(ValidationError):
                Draft7Validator(row["properties"]["tx_hash"]).validate(invalid)
    for accounts in ([], result["accounts"] * 2, [{"side": "layerx", "account": "ab" * 32}]):
        invalid = deepcopy(result)
        invalid["accounts"] = accounts
        with pytest.raises(ValidationError):
            Draft7Validator(schema).validate(invalid)
    for field, value in (("side", "layerx"), ("amount", 1), ("blockHeight", -1)):
        invalid = deepcopy(result)
        invalid["items"][0][field] = value
        with pytest.raises(ValidationError):
            Draft7Validator(schema).validate(invalid)
    old_schema = deepcopy(schema)
    old_row = old_schema["properties"]["items"]["items"]
    old_row["properties"].pop("tx_hash")
    old_row["required"].remove("tx_hash")
    with pytest.raises(ValidationError):
        Draft7Validator(old_schema).validate(result)


def test_repair_record_maps_the_finding_and_discloses_unexecuted_owner_work():
    record = tomllib.loads(RECORD.read_text())
    audit = tomllib.loads(source("spec/layerx-merge/programme/audit/records/22.30.81.kvx"))
    finding = record["finding"]["doc-openrpc"]
    assert set(record["finding"]) == {"doc-openrpc"}
    assert finding["file"] == audit["finding"]["doc-openrpc"]["file"]
    assert finding["class"] == "doc-drift"
    tests = {node.name for node in ast.parse(Path(__file__).read_text()).body
             if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")}
    assert set(finding["tests"]) == tests
    assert record["task"]["id"] == "22.33.1443"
    assert record["task"]["base_revision"] == "876fbe3211af9ef09a8e5589f822c0e986d0a140"
    assert record["task"]["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "not run" in record["task"]["baseline"]
    assert "not executed" in record["proof"]["review_only"]
    assert "plain" in record["proof"]["capture_limits"]
    assert set(record["wiring"]) == {"tenancy", "sdk", "web", "captures", "snapshot"}
    for request in record["wiring"].values():
        assert request["path"] and request["owner_domain"] and request["request"]
