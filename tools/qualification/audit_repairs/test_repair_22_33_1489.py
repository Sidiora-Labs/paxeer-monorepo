"""Offline journey contract qualification; native runtime limits are in the record.

Execute the production Python request builders, protected input reader and BFT
consumer. Parse the shell's embedded Python and assert its actual guarded HTTP
commands against the Rust producer/handler/engine/journal source. bash syntax is
qualified once by hf-build. No native binary, build toolchain or live peer is
substituted or invoked by this module.
"""

import ast
from copy import deepcopy
import importlib.util
import json
from pathlib import Path
import re
import tomllib

import pytest


ROOT = Path(__file__).resolve().parents[3]
RAMPS = ROOT / "services/platform/ramps"
RECORD = Path(__file__).with_name("repair_22_33_1489.kvx")
SPEC = importlib.util.spec_from_file_location("journey_contract", RAMPS / "journey_contract.py")
CONTRACT = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(CONTRACT)


def source(path):
    return (RAMPS / path).read_text(encoding="utf-8")


def full_journey():
    blocks = re.findall(r"<<'PY'\n(.*?)\nPY\n", source("sandbox-journey.sh"), re.S)
    assert len(blocks) == 1
    return blocks[0], ast.parse(blocks[0])


def function(name):
    text, tree = full_journey()
    nodes = [node for node in tree.body if isinstance(node, ast.FunctionDef) and node.name == name]
    assert len(nodes) == 1, name
    return ast.get_source_segment(text, nodes[0])


def assert_facts(text, *facts):
    for fact in facts:
        assert fact in text, fact


def terms():
    return {
        "on": dict(order_id="repair-on", quote_id="on", amount="100",
                   asset="01" * 32, from_account="02" * 32, to_account="03" * 32,
                   context="04" * 32),
        "off": dict(order_id="repair-off", quote_id="off", amount="100",
                    asset="01" * 32, from_account="03" * 32, to_account="02" * 32,
                    context="04" * 32, customer_budget_id="05" * 32,
                    customer_budget_purpose="06" * 32, raw_transaction="0200"),
        "rebalance": dict(asset="01" * 32, amount="100", idempotency_key="07" * 32,
                          operator_account="agent:did:layerx:repair:main"),
    }


def observation(rung):
    # Consumer validation inputs, not claimed node transcripts or signature proof.
    body = terms()["rebalance"]
    return dict(settlement_domain="paxeer", operator_account=body["operator_account"],
                idempotency_key=CONTRACT.hash_bytes(body["idempotency_key"]),
                asset=CONTRACT.hash_bytes(body["asset"]), amount=int(body["amount"]),
                operation_id="operation-1", transaction_hash="0x" + "08" * 32,
                status=rung, rung=rung,
                block_height=12 if rung == "committed" else None,
                block_hash="0x" + "09" * 32 if rung == "committed" else None)


def test_journey_order_request_shared_builder_matches_real_create_order_binding():
    inputs = terms()
    CONTRACT.validate_inputs(inputs)
    on = json.loads(json.dumps(CONTRACT.order_request(inputs["on"])))
    off = json.loads(json.dumps(CONTRACT.order_request(inputs["off"])))
    assert on == {"order_id": "repair-on", "quote_id": "on"}
    assert set(off) == {"order_id", "quote_id", "customer_budget_id", "customer_budget_purpose"}
    assert off["customer_budget_id"] == [5] * 32
    assert off["customer_budget_purpose"] == [6] * 32
    lib = source("toolkit/src/lib.rs")
    assert_facts(lib, '#[serde(rename = "customer_budget_id", alias = "payer_grant")]',
                 '#[serde(rename = "customer_budget_purpose", alias = "payer_grant_purpose")]',
                 'request.payer_grant == Some([0; 32])',
                 'request.payer_grant_purpose == Some([0; 32])',
                 'request.payer_grant.is_none() || request.payer_grant_purpose.is_none()')
    handler = source("reference/src/main.rs")
    create = handler[handler.index("fn create_order("):handler.index("fn source_settlement(")]
    assert_facts(create, 'let principal = authenticate(state, request)?',
                 'serde_json::from_slice(&request.body)', 'existing.order.customer != principal',
                 'existing.order.payer_grant != create.payer_grant',
                 'existing.order.payer_grant_purpose != create.payer_grant_purpose',
                 'error(409, "order_id_conflict")', 'RampOrder::bind(create, quote, principal')


@pytest.mark.parametrize("field", ["payer_grant", "payer_grant_purpose", "canonical_receive_payload",
                                 "account_sequence", "unknown", "required_confirmations"])
def test_protected_input_schema_refuses_retired_and_unknown_fields(field):
    inputs = terms()
    inputs["off"][field] = 1
    with pytest.raises(RuntimeError, match="order_input_schema"):
        CONTRACT.validate_inputs(inputs)


@pytest.mark.parametrize("field", ["customer_budget_id", "customer_budget_purpose"])
@pytest.mark.parametrize("value", [None, "00" * 32, "A1" * 32, "01" * 31, [1] * 32])
def test_protected_authority_is_required_canonical_and_nonzero(field, value):
    inputs = terms()
    inputs["off"][field] = value
    with pytest.raises(RuntimeError, match="nonzero_hash32"):
        CONTRACT.validate_inputs(inputs)


def test_protected_file_reader_reopens_real_inputs_and_refuses_unsafe_files(tmp_path):
    path = tmp_path / "inputs.json"
    path.write_text(json.dumps(terms()), encoding="utf-8")
    path.chmod(0o600)
    assert CONTRACT.load_inputs(path) == terms()
    # A fresh read reopens the protected file; no retained in-memory input is used.
    updated = terms()
    updated["off"]["customer_budget_purpose"] = "0a" * 32
    path.write_text(json.dumps(updated), encoding="utf-8")
    assert CONTRACT.load_inputs(path)["off"]["customer_budget_purpose"] == "0a" * 32
    path.chmod(0o644)
    with pytest.raises(RuntimeError, match="protected_inputs_permissions"):
        CONTRACT.load_inputs(path)
    path.chmod(0o600)
    link = tmp_path / "inputs-link.json"
    link.symlink_to(path)
    with pytest.raises(OSError):
        CONTRACT.load_inputs(link)
    path.write_text('{"on":{},"on":{}}', encoding="utf-8")
    with pytest.raises(RuntimeError, match="duplicate_input_field"):
        CONTRACT.load_inputs(path)


def test_both_branches_drive_authority_conflicts_and_explicit_retired_refusal():
    shell = source("sandbox-journey.sh")
    full, _ = full_journey()
    simple = shell.split("\nPY\nfi\n", 1)[1]
    assert_facts(function("create"), 'body = order_request(terms)',
                 'request("customer", "customer", "POST", "/v1/orders", body, statuses, error)')
    assert_facts(full, 'inputs = load_inputs(env("LAYERX_RAMP_INPUTS_FILE"))',
                 'for field in ("customer_budget_id", "customer_budget_purpose"):',
                 'create(inputs["off"], {field: changed}, (409,), "order_id_conflict")',
                 '{"canonical_receive_payload": []}', '(400,), "work_invalid"',
                 '(400,), "layerx_binding_refused"', 'work(off_digest, "submit_customer_payment")')
    assert_facts(simple, 'python3 "$contract" inputs "$LAYERX_RAMP_INPUTS_FILE"',
                 'python3 "$contract" order', 'python3 "$contract" work',
                 'for field in customer_budget_id customer_budget_purpose; do',
                 'expect_refusal customer /v1/orders "$missing" 400 request_refused',
                 'expect_refusal customer /v1/orders "$changed" 409 order_id_conflict',
                 'expect_refusal operator /internal/v1/work "$retired" 400 work_invalid',
                 'work "${off_digest}" submit_customer_payment', '[ "$status" = "$expected_status" ]',
                 "'.error == $error'")
    for retired in ("LAYERX_RAMP_OFF_GRANT_JSON", "LAYERX_RAMP_OFF_RECEIVER_SEQUENCE",
                    "native_payer_grant_authorization_required"):
        assert retired not in shell
    assert '"payer_grant":' not in function("create")


def test_current_work_requests_bind_real_signed_customer_submission_path():
    digest = [7] * 32
    on = CONTRACT.work_request(digest, "submit_layerx")
    off = CONTRACT.work_request(digest, "submit_customer_payment", "0200")
    assert on == dict(order_digest=digest, action="submit_layerx", account_sequence=None)
    assert off == dict(order_digest=digest, action="submit_customer_payment",
                       account_sequence=None, raw_transaction=[2, 0])
    handler = source("reference/src/main.rs")
    work_struct = handler[handler.index("struct Work {"):handler.index("enum Rebalance {")]
    assert_facts(work_struct, 'raw_transaction: Option<Vec<u8>>', 'SubmitCustomerPayment')
    assert '#[serde(deny_unknown_fields)]\nstruct Work {' in handler
    work = handler[handler.index("fn perform_work("):handler.index("fn rebalance_presentation(")]
    assert_facts(work, 'require_operator(state, request)?', 'error(400, "work_invalid")',
                 'WorkAction::SubmitCustomerPayment', 'raw.len() <= 131_072',
                 'else if work.raw_transaction.is_some()',
                 'engine.submit_customer_payment(', 'work.raw_transaction', '.as_deref()',
                 '(RampDirection::OnRamp, None) => 0',
                 'work.account_sequence.is_some() && !matches!(work.action, WorkAction::SubmitLayerx)')
    assert work.index('require_operator(state, request)?') < work.index('serde_json::from_slice')
    engine = source("toolkit/src/engine.rs")
    customer = engine[engine.index("pub fn submit_customer_payment("):engine.index("fn submit_payment(")]
    assert_facts(customer, 'snapshot.stage != WorkflowStage::AwaitingLayerxPayment',
                 'snapshot.order.direction() != RampDirection::OffRamp',
                 '.prepare_customer_payment(&snapshot.order, raw, now)?', 'self.submit_payment(')
    clients = source("toolkit/src/clients.rs")
    prepare = clients[clients.index("pub fn prepare_customer_payment("):clients.index("fn payment_sender(")]
    assert_facts(prepare, 'self.authenticate_payment_transaction(order, tx_hash, raw)?',
                 'self.verify_customer_budget(order, None)?', '.pending_nonce(&sender)',
                 'call.nonce != pending', 'raw_transaction: raw.to_vec()')
    authentic = clients[clients.index("fn authenticate_payment_transaction("):
                        clients.index("pub fn authenticate_settlement")]
    assert_facts(authentic, 'transaction_hash(raw) != tx_hash', 'call.chain_id != self.chain_id',
                 'recover_sender(&call, &signature)', 'self.payment_sender(order)?',
                 'call.to != compiled.to', 'call.data != compiled.data')
    assert_facts(source("toolkit/src/evm.rs"), 'word(4)? != auth.purpose_hash',
                 'word(1)? != crate::ledger_account(&order.customer.account)?')


@pytest.mark.parametrize("value", [None, "", "00", "02", "0x0200", "02GG", "020", "02" + "00" * 131072])
def test_customer_work_preflight_rejects_malformed_or_unbounded_bytes(value):
    with pytest.raises(RuntimeError, match="signed_eip1559_transaction"):
        CONTRACT.work_request([1] * 32, "submit_customer_payment", value)


def test_other_work_actions_cannot_smuggle_customer_signed_bytes():
    for action in ("compliance", "submit_provider", "reconcile_provider", "submit_layerx", "resolve_layerx"):
        assert CONTRACT.work_request([1] * 32, action)["account_sequence"] is None
        with pytest.raises(RuntimeError, match="unexpected_signed_transaction"):
            CONTRACT.work_request([1] * 32, action, "0200")


def test_bft_consumer_admits_only_authenticated_commit_as_completion():
    previous = None
    for rung in ("submitted", "receipted", "committed"):
        value = observation(rung)
        assert CONTRACT.observe_rebalance(value, terms()["rebalance"], previous) is (rung == "committed")
        previous = value
    producer = source("reference/src/main.rs")
    producer = producer[producer.index("fn rebalance_presentation("):producer.index("fn rebalance(")]
    keys = set(re.findall(r'^\s*"([a-z_]+)":', producer, re.M))
    assert keys == set(observation("committed")) | {"external_custody_label"}
    assert_facts(producer, '"rung": snapshot.rung.map(|rung| rung.name())',
                 '"block_height": snapshot.block_height', '"status": snapshot.stage')
    engine = source("toolkit/src/engine.rs")
    poll = engine[engine.index("    pub fn poll("):]
    assert poll.index('.authenticate_settlement(transaction_hash, receipt)?') < poll.index('self.journal.observe_paxeer(')
    clients = source("toolkit/src/clients.rs")
    auth = clients[clients.index("pub fn authenticate_settlement"):clients.index("fn verify_customer_budget(")]
    assert_facts(auth, 'verify_sequencer_signature', 'block.hash != *block_hash',
                 '.find(|raw| transaction_hash(raw) == transaction)', '!receipt.success',
                 'receipt.tx_hash != transaction')
    full, _ = full_journey()
    assert_facts(full, 'committed = observe_rebalance(observed, rebalance, previous)',
                 'if committed:\n            break', '"authenticated_commit_required"',
                 '"rung": final["rung"], "block_height": final["block_height"]')
    for retired in ("required_confirmations", 'observed["confirmations"]', '"displaced"', '"confirming"', '"final"'):
        assert retired not in full


@pytest.mark.parametrize("rung", ["submitted", "receipted"])
@pytest.mark.parametrize("field,value", [("block_hash", "0x" + "01" * 32), ("block_height", 12)])
def test_no_credit_before_commit_rejects_premature_block_evidence(rung, field, value):
    body = observation(rung)
    body[field] = value
    with pytest.raises(RuntimeError, match="no_credit_before_authenticated_commit"):
        CONTRACT.observe_rebalance(body, terms()["rebalance"])


@pytest.mark.parametrize("field,value,code", [
    ("block_height", 0, "committed_height"), ("block_height", True, "committed_height"),
    ("block_height", 2**64, "committed_height"), ("block_hash", "0x" + "00" * 32, "committed_block"),
    ("status", "final", "rebalance_rung"), ("amount", True, "rebalance_terms"),
    ("transaction_hash", "0x" + "00" * 32, "rebalance_transaction"),
    ("idempotency_key", [8] * 32, "rebalance_terms"), ("asset", [8] * 32, "rebalance_terms"),
    ("operator_account", "agent:did:layerx:other:main", "rebalance_terms"),
])
def test_bft_commit_rejects_unbound_or_invalid_evidence(field, value, code):
    body = observation("committed")
    body[field] = value
    with pytest.raises(RuntimeError, match=code):
        CONTRACT.observe_rebalance(body, terms()["rebalance"])


@pytest.mark.parametrize("field,value,code", [
    ("operation_id", "another-operation", "rebalance_identity_changed"),
    ("transaction_hash", "0x" + "0a" * 32, "rebalance_identity_changed"),
    ("block_height", 13, "committed_identity_changed"),
    ("block_hash", "0x" + "0a" * 32, "committed_identity_changed"),
])
def test_recovered_committed_identifiers_cannot_change(field, value, code):
    retained = observation("committed")
    reopened = deepcopy(retained)
    reopened[field] = value
    with pytest.raises(RuntimeError, match=code):
        CONTRACT.observe_rebalance(reopened, terms()["rebalance"], retained)
    with pytest.raises(RuntimeError, match="rebalance_rung_regressed"):
        CONTRACT.observe_rebalance(observation("receipted"), terms()["rebalance"], retained)


def test_restart_journey_stops_and_reopens_service_before_retained_state_checks():
    restart = function("restart_service")
    assert_facts(function("service_control"), 'read_protected("LAYERX_RAMP_SERVICE_CONTROL_FILE")',
                 'set(control) == {"stop", "start"}', 'Path(argv[0]).is_absolute()')
    assert_facts(restart, 'for action in ("stop", "start"):', 'subprocess.run(control[action]',
                 'require(completed.returncode == 0', 'if action == "stop":',
                 'socket.create_connection', 'except ConnectionRefusedError:',
                 'raise RuntimeError("service_still_listening")',
                 '"GET", "/readyz"', 'ready["journal"]["ready"] is True',
                 'ready["journal"]["recovery_required"] is False',
                 'ready["build_revision"] == expected_revision',
                 'ready["build_source_digest"] == expected_digest')
    full, _ = full_journey()
    recovery = full[full.index('current_step = "service_restart"'):]
    assert recovery.index('restart_service()') < recovery.index('replay = create(inputs[side])')
    assert_facts(recovery, 'require(read_order(digest) == done',
                 'observe_rebalance(recovered, rebalance, final)',
                 'require(recovered == final, "rebalance_recovery_changed")')
    service = source("reference/src/main.rs")
    assert_facts(service, 'Journal::open(&config.journal_path)',
                 'if verified_recovery(&state).is_err()', 'journal.recover_verified(|projection|',
                 'layerx_ramp_toolkit::clients::verify_recovery_settlement(')
    journal = source("toolkit/src/journal.rs")
    assert_facts(journal, 'observation.rung < rung',
                 'committed != observation.block_hash.is_some()',
                 'committed != observation.block_height.is_some()')


def test_repair_record_names_findings_tests_limits_and_wiring():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    assert record["repair"]["task"] == "22.33.1489"
    assert set(record["finding"]) == {"journey-order-request", "journey-confirmation-vocabulary"}
    for finding in record["finding"].values():
        assert all(callable(globals().get(name)) for name in finding["tests"])
    assert record["proof"]["review_only"]
    assert record["repair"]["baseline"].startswith("Not run:")
    assert record["wiring"]["hosted_inputs"].startswith("Domain 8:")
    assert record["wiring"]["native_qualification"].startswith("Domain 8:")
    for path, name in (("reference/Cargo.toml", "layerx-reference-ramp"),
                       ("toolkit/Cargo.toml", "layerx-ramp-toolkit")):
        assert tomllib.loads(source(path))["package"]["name"] == name
