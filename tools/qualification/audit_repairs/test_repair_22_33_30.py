"""Offline contract/source qualification of the bank-plan and journey goldens.

Rust round trips live beside the production producers. This Python-only gate
checks their wiring and exact fixture contracts; it does not execute Rust or
turn a projection input into cryptographically verified receipt material.
"""

import ast
from datetime import datetime
from hashlib import sha256
import json
from pathlib import Path
import re
import tomllib


ROOT = Path(__file__).resolve().parents[3]
SERVICE = ROOT / "services/human/crates/layerx-human-service/src"
SCHEMA = ROOT / "services/human/schema/human-api"
GOLDEN = SCHEMA / "golden"
PRODUCTION = (SERVICE / "server/production_components.rs").read_text()
READS = (SERVICE / "server/production_reads.rs").read_text()
ROUTER = (SERVICE / "journeys/router.rs").read_text()
RECORD = Path(__file__).with_name("repair_22_33_30.kvx")


def load(name):
    return json.loads((GOLDEN / name).read_text())


def function(source, name):
    """Extract a declared Rust function, balancing braces outside strings/comments."""
    match = re.search(r"\bfn\s+" + re.escape(name) + r"\s*\(", source)
    assert match is not None, name
    start = source.index("{", match.end())
    depth = 0
    for token in re.finditer(r'"(?:\\.|[^"\\])*"|//[^\n]*|/\*.*?\*/|[{}]',
                             source[start:], re.S):
        value = token.group()
        if value == "{":
            depth += 1
        elif value == "}":
            depth -= 1
            if depth == 0:
                return source[start:start + token.end()]
    raise AssertionError(f"unclosed Rust function {name}")


def facts(source, *expected):
    compact = re.sub(r"\s+", "", source)
    for item in expected:
        assert re.sub(r"\s+", "", item) in compact, item


def result(name):
    envelope = load(name)
    assert envelope["status"] == 200
    assert envelope["body"]["ok"] is True
    assert set(envelope["body"]) == {"ok", "result", "trace"}
    return envelope["body"]["result"]


def contract_fields(schema, name, category="required"):
    # KVX accepts keys such as settlement_domain and settlement_domain.retired
    # together, so read the one exact section without coercing it to TOML.
    section = schema.split(f"[type.{name}]", 1)[1].split("\n[", 1)[0]
    match = re.search(rf"^{category}\s*=\s*(\[.*\])$", section, re.M)
    assert match is not None, (name, category)
    return {item.split(":", 1)[0] for item in json.loads(match[1])}


def test_supported_request_uses_authoritative_bank_producer_path():
    request = load("intent.plan.request.json")
    body = request["body"]
    context = load("producer/intent.plan.context.json")
    schema = (SCHEMA / "intent.kvx").read_text()
    assert request["method"] == "POST"
    assert request["path"] == "/v1/intents/plan"
    assert set(body) == contract_fields(schema, "PlanIntentRequest")
    assert body["source"] == {"kind": "human", "account": context["owner_account"]}
    assert body["destination"] == {"kind": "agent", "account": "agent:did:layerx:bob:main"}
    assert body["asset_id"] == sha256(context["denom"].encode()).hexdigest()
    balance = context["balance"]
    assert balance["asset_id"] == body["asset_id"]
    assert balance["currency"] == body["money"]["currency"] == "LXP"
    assert balance["global_sequence"] == balance["observed_head_sequence"]
    assert balance["age_seconds"] == 0
    assert int(balance["amount"]) >= int(body["money"]["amount"]) + int(result("intent.plan.response.json")["total_fee"]["amount"])
    assert datetime.fromisoformat(context["observed_at"]) < datetime.fromisoformat(body["constraints"]["deadline"])
    assert body["constraints"]["allow_top_up"] is False
    execute = function(PRODUCTION, "execute_intent_plan")
    calls = ["Self::intent_from_request", "self.intent_observed_state",
             "crate::journeys::plan", "resolve_kernel_intent_context",
             "self.intent_transactions", "intent_plan_json"]
    positions = [execute.index(call) for call in calls]
    assert positions == sorted(positions)
    facts(execute, 'money_field(&request.body, "money")?.1 != currency',
          'result: intent_plan_json(&planned, &currency, &transactions)')
    observed = function(PRODUCTION, "intent_observed_state")
    facts(observed, "let balance = agent.balance().map_err(agent_failure)?;",
          "balance.global_sequence != balance.observed_head_sequence",
          "balance.age_seconds > self.activity_freshness_seconds",
          "supported_intent_endpoints(intent).map_err(submit_failure)?;")
    facts(function(PRODUCTION, "resolve_kernel_intent_context"),
          ".account_sequence(&owner.actor, &owner.authority)")
    runtime = (SERVICE / "server/agent_runtime.rs").read_text()
    facts(function(runtime, "account_sequence"), "self.pending_nonce(actor, authority)")
    transactions = function(PRODUCTION, "intent_transactions")
    facts(transactions, "context .account_sequence .checked_add(",
          "agent.identity_resolve(did)", "if identity.frozen",
          ".bound_address(public_key)", "let denom = send_leg_denom(&assets, &balances, leg.asset())?;",
          "bank_send_calldata(wallet, recipient, &denom, leg.amount().value())",
          "bind_call(&evm_call(&transaction), wallet)")


def test_retired_wallet_fixture_reaches_typed_unsupported_plan():
    request = load("negative/intent.plan.paxeer-wallet.request.json")
    assert request["body"]["source"] == {"kind": "paxeer-wallet"}
    assert request["body"]["destination"] == load("intent.plan.request.json")["body"]["destination"]
    guard = function(PRODUCTION, "supported_intent_endpoints")
    facts(guard, "intent.source() == &crate::journeys::Endpoint::PaxeerWallet",
          "|| intent.destination() == &crate::journeys::Endpoint::PaxeerWallet",
          "return Err(crate::journeys::SubmitRefusal::UnsupportedPlan);", "Ok(())")
    facts(function(PRODUCTION, "intent_endpoint"),
          'if kind == "paxeer-wallet"', "return Ok(crate::journeys::Endpoint::PaxeerWallet);")
    refusal = load("negative/intent.plan.paxeer-wallet.response.json")
    assert refusal["status"] == 403
    assert refusal["body"]["ok"] is False
    assert refusal["body"]["error"] == {"code": "forbidden", "copy_key": "error.request.forbidden", "retry": "final"}
    assert refusal["body"]["error"] == load("intent.plan.failure.json")["body"]["error"]
    mapping = function(PRODUCTION, "submit_failure")
    for field in ("status", "code", "copy_key", "retry", "field"):
        assert f"refusal.{field}()" in mapping
    native = function(PRODUCTION, "intent_bank_golden_retired_wallet_is_unsupported_in_both_directions")
    facts(native, "for reverse in [false, true]", "supported_intent_endpoints(&intent)",
          "crate::journeys::SubmitRefusal::UnsupportedPlan", "submit_failure(refusal)",
          'expected["body"]["error"]')


def test_plan_contains_every_transaction_field_and_exact_bank_call():
    request = load("intent.plan.request.json")["body"]
    plan = result("intent.plan.response.json")
    context = load("producer/intent.plan.context.json")
    schema = (SCHEMA / "intent.kvx").read_text()
    assert set(plan) == contract_fields(schema, "IntentPlan")
    assert plan["journey_kind"] == "move"
    assert len(plan["legs"]) == len(plan["signing_requirements"]) == 1
    leg = plan["legs"][0]
    assert set(leg) == contract_fields(schema, "IntentLeg") | contract_fields(schema, "IntentLeg", "optional")
    assert leg["index"] == 0
    assert (leg["mechanism"], leg["domain"]) == ("send", "layerx")
    for key in ("source", "destination", "money"):
        assert leg[key] == request[key]
    transaction = context["transaction"]
    for field in ("nonce", "chain_id", "gas_limit"):
        assert type(leg[field]) is int
        assert leg[field] == transaction[field]
    assert leg["chain_id"] == 125
    assert leg["nonce"] == int(context["nonce_observation"]["result"], 16)
    assert context["nonce_observation"] == {"method": "eth_getTransactionCount", "params": [context["sender"], "pending"], "result": "0x13"}
    for field in ("max_fee_per_gas", "max_priority_fee_per_gas"):
        assert leg[field] == str(transaction[field])
    assert 0 < int(leg["max_priority_fee_per_gas"]) <= int(leg["max_fee_per_gas"])
    assert leg["to"] == "0x" + bytes(transaction["to"]).hex() == "0x" + "00" * 18 + "1001"
    assert leg["value"] == "0" and transaction["value"] == [0] * 32
    calldata = bytes.fromhex(leg["calldata"][2:])
    assert calldata == bytes(transaction["calldata"])
    assert calldata[:4].hex() == "5c05961b"
    words = [calldata[offset:offset + 32] for offset in range(4, len(calldata), 32)]
    assert len(words) == 6 and all(len(word) == 32 for word in words)
    assert words[0] == bytes(12) + bytes.fromhex(context["sender"][2:])
    assert words[1] == bytes(12) + bytes.fromhex(context["recipient"][2:])
    assert int.from_bytes(words[2], "big") == 128
    assert int.from_bytes(words[3], "big") == int(request["money"]["amount"])
    length = int.from_bytes(words[4], "big")
    assert length == len(context["denom"].encode())
    assert words[5][:length] == context["denom"].encode()
    assert words[5][length:] == bytes(32 - length)
    fee = leg["gas_limit"] * int(leg["max_fee_per_gas"])
    assert plan["total_fee"] == leg["fee"] == {"amount": str(fee), "currency": "LXP"}
    assert fee <= int(request["constraints"]["max_fee"]["amount"])
    producer = function(PRODUCTION, "intent_plan_json")
    for field in ("nonce", "chain_id", "gas_limit"):
        facts(producer, f'"{field}": transactions[index].{field}')
    for field in ("max_fee_per_gas", "max_priority_fee_per_gas"):
        facts(producer, f'"{field}": transactions[index].{field}.to_string()')
    facts(producer, '"to": format!("0x{}", hex_bytes(&transactions[index].to))',
          '"calldata": format!("0x{}", hex_bytes(&transactions[index].calldata))', '"value": "0"')
    facts(function(PRODUCTION, "bank_send_calldata"), "vec![0x5c, 0x05, 0x96, 0x1b]",
          "abi_amount(128)", "abi_amount(amount)", "denom.as_bytes().to_vec()")
    native = function(PRODUCTION, "intent_bank_golden_request_produces_exact_eip1559_response")
    facts(native, "ProductionComponents::intent_from_request", "crate::journeys::plan(&intent, &observed)",
          "bank_send_calldata(sender, recipient,", "decode_unsigned(&call.encode_unsigned())",
          "assert_eq!(decoded, call)", "bind_call(&decoded, sender)",
          'assert_eq!(transported, expected["body"]["result"])')


def test_plan_digest_and_every_signing_requirement_bind_canonical_contract():
    # Independent wire-consumer check of canonical bytes, not another planner.
    request = load("intent.plan.request.json")["body"]
    plan = result("intent.plan.response.json")
    def endpoint(value):
        account = value["account"].encode()
        return bytes([{"human": 1, "agent": 2}[value["kind"]]]) + len(account).to_bytes(2, "big") + account
    ends = endpoint(request["source"]) + endpoint(request["destination"])
    asset = bytes.fromhex(request["asset_id"])
    amount = int(request["money"]["amount"]).to_bytes(16, "big")
    fee = int(plan["total_fee"]["amount"]).to_bytes(16, "big")
    deadline = int(datetime.fromisoformat(request["constraints"]["deadline"]).timestamp()).to_bytes(8, "big")
    intent_bytes = ends + asset + amount + deadline + int(request["constraints"]["max_fee"]["amount"]).to_bytes(16, "big") + bytes([0])
    # Version 1; one send/transfer leg; account-owner authority; no allowance.
    leg_bytes = bytes.fromhex("00000506") + ends + asset + amount + fee + bytes.fromhex("0200")
    canonical = bytes([1]) + intent_bytes + bytes.fromhex("0001") + leg_bytes + fee
    digest = sha256(b"layerx-human-unified-plan/v1" + canonical).digest()
    assert plan["plan_digest"] == digest.hex()
    requirement, = plan["signing_requirements"]
    assert set(requirement) == contract_fields((SCHEMA / "intent.kvx").read_text(), "IntentSigningRequirement")
    assert requirement["leg_index"] == 0
    assert requirement["authority"] == "account-owner"
    for field, domain in (("action_key", b"layerx-human-unified-action/v1"),
                          ("signing_context", b"layerx-human-unified-context/v1")):
        assert requirement[field] == sha256(domain + digest + bytes(8)).hexdigest()
    for name, domain in (("PLAN", "plan"), ("ACTION", "action"), ("CONTEXT", "context")):
        assert f'const {name}_DOMAIN: &[u8] = b"layerx-human-unified-{domain}/v1";' in ROUTER
    facts(function(ROUTER, "derive"), "digest.update(domain)", "digest.update(self.digest)",
          "digest.update(position.to_be_bytes())")
    producer = function(PRODUCTION, "intent_plan_json")
    for field in ("leg_index", "action_key", "signing_context", "authority"):
        assert f"requirement.{field}()" in producer
    assert '"plan_digest": hex_bytes(&plan.digest())' in producer
    assert '"total_fee": intent_money_json(plan.total_fee(), currency)' in producer


def test_current_journey_matches_route_projection_and_never_uses_retired_finality():
    record = load("producer/journey.get.record.json")
    journey = result("journey.get.response.json")
    request = load("journey.get.request.json")
    assert request["path"] == "/v1/journeys/" + record["journey_id"]
    assert set(journey) == contract_fields((SCHEMA / "journeys.kvx").read_text(), "Journey")
    for field in ("journey_id", "state", "started_at", "updated_at"):
        assert journey[field] == record[field]
    assert journey["kind"] == "move"
    assert journey["state_copy_key"] == "journey.state.done"
    stage, = journey["stages"]
    leg, = record["legs"]
    assert leg["verified"] is True and leg["refused"] is False and leg["submitted"] is True
    assert stage["stage_id"] == f'stg_{record["journey_id"]}_0'
    assert (stage["state"], stage["copy_key"]) == ("done", "journey.stage.confirmed")
    assert stage["tx_hash"] == leg["tx_hash"]
    assert stage["nonce"] == leg["nonce"] == result("intent.plan.response.json")["legs"][0]["nonce"]
    assert stage["evidence"] == journey["evidence"] == [{"evidence_id": "evd_" + leg["receipt_hash"][2:], "class": "layerx-receipt", "verification": "receipt-verified"}]
    assert re.fullmatch(r"0x[0-9a-f]{64}", leg["tx_hash"])
    assert re.fullmatch(r"0x[0-9a-f]{64}", leg["receipt_hash"])
    route = PRODUCTION.split('if request.operation.name == "journey.get" {', 1)[1].split("for journey in", 1)[0]
    facts(route, "self.advance_evm_intent(scope, &id, self.now()?)?;",
          "serde_json::from_slice(row.bytes())", "super::production_reads::intent_journey_json(&record)?")
    projector = function(READS, "intent_journey_json")
    facts(projector, '("refused", "refused", "layerx-receipt", "receipt-verified")',
          '("confirmed", "done", "layerx-receipt", "receipt-verified")',
          '("submitted", "processing", "submission-record", "unverified")',
          'leg["receipt_hash"]', 'format!("evd_{digest}")', 'format!("stg_{id}_{index}")')
    advance = function(PRODUCTION, "advance_evm_intent")
    facts(advance, "material.rung == Rung::Committed", "ProductionAgentCreation::protocol_evidence(",
          "verify_move_evm_outcome(&evidence, disclosure.activity_type)?", 'leg["verified"] = json!(true)')
    assert advance.index("verify_move_evm_outcome") < advance.index('leg["verified"] = json!(true)')
    for forbidden in ("paxeer-finality", "paxeer-finalised", "settlement-anchored", "raw_transaction"):
        assert forbidden not in projector
        assert forbidden not in json.dumps(journey)
    native = function(PRODUCTION, "intent_bank_golden_journey_uses_the_route_projection")
    facts(native, "production_reads::intent_journey_json(&stored)",
          'assert_eq!(actual, expected["body"]["result"])',
          '(false, false, "submission-record", "unverified")',
          '(true, false, "layerx-receipt", "receipt-verified")',
          '(false, true, "layerx-receipt", "receipt-verified")',
          "Err(ApiFailure::upstream_degraded())")


def test_historical_payloads_preserve_baseline_bytes_and_decode_vocabulary():
    repair = tomllib.loads(RECORD.read_text())
    for name, digest in repair["baseline"]["preserved_sha256"].items():
        raw = (GOLDEN / name).read_bytes()
        assert sha256(raw).hexdigest() == digest
        decoded = json.loads(raw)
        assert json.loads(json.dumps(decoded)) == decoded
    old_plan = result("historical/intent.plan.response.json")
    old_journey = result("historical/journey.get.response.json")
    assert old_plan["journey_kind"] == old_journey["kind"] == "deposit"
    assert old_plan["legs"][0]["mechanism"] == "paxeer-custody-deposit"
    reference = old_journey["stages"][1]["evidence"][0]
    assert reference["class"] == "paxeer-finality"
    assert reference["verification"] == "paxeer-finalised"
    schema = (SCHEMA / "journeys.kvx").read_text()
    for type_name, value in (("JourneyKind", "deposit"), ("EvidenceClass", reference["class"]),
                             ("VerificationLevel", reference["verification"])):
        section = schema.split(f"[type.{type_name}]", 1)[1].split("\n[", 1)[0]
        for key in ("variants", "retired_variants"):
            match = re.search(rf"^{key}\s*=\s*(\[.*\])$", section, re.M)
            assert value in json.loads(match[1])
    facts(function(PRODUCTION, "intent_bank_golden_historical_payloads_still_decode_separately"),
          'golden!("historical/intent.plan.response.json")',
          'golden!("historical/journey.get.response.json")', "serde_json::from_slice(")


def test_repair_record_maps_all_findings_and_declares_evidence_limits():
    record = tomllib.loads(RECORD.read_text())
    assert record["repair"]["task"] == "22.33.30"
    assert record["repair"]["base_revision"] == "5fc0c55eaaa6d43868c4a2872366299b8b7e2007"
    assert record["repair"]["build_cmd"] == "python3 --version"
    assert record["repair"]["verify_cmd"] == "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_30.py"
    assert record["repair"]["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "not executed" in record["repair"]["review_only"]
    assert "projection input" in record["repair"]["review_only"]
    assert record["baseline"]["verification_run"] is False
    assert record["baseline"]["observed_failing_tests"] == []
    assert set(record["finding"]) == {"intent-request-retired-path", "intent-response-retired-path", "journey.get-retired-evidence"}
    declared = {node.name for node in ast.parse(Path(__file__).read_text()).body
                if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")}
    mapped = set()
    for finding in record["finding"].values():
        assert finding["audit"] == "22.30.102"
        assert finding["severity"] == "medium"
        assert finding["tests"] and set(finding["tests"]) <= declared
        assert (ROOT / finding["file"]).is_file()
        mapped.update(finding["tests"])
    assert mapped == declared
    assert all("Domain 10" in item for item in record["wiring"]["requests"])
    assert any("tools/schema-check" in item for item in record["wiring"]["requests"])
    for name in ("intent_bank_golden_request_produces_exact_eip1559_response",
                 "intent_bank_golden_retired_wallet_is_unsupported_in_both_directions",
                 "intent_bank_golden_journey_uses_the_route_projection",
                 "intent_bank_golden_historical_payloads_still_decode_separately"):
        assert re.search(r"#\[test\]\s*fn " + name + r"\(", PRODUCTION)
        assert "assert" in function(PRODUCTION, name)
    assert "#[ignore]" not in PRODUCTION.split("mod intent_bank_golden_tests {", 1)[1].split("mod send_leg_denom_tests", 1)[0]
