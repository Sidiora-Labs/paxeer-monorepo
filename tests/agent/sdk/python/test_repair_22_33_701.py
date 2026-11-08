"""SDK-owned wire proofs; external execution requirements live in the task record."""

import json
from contextlib import contextmanager
from dataclasses import replace
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from threading import Thread

import pytest

from layerx_sdk.account_derivation import keccak256
from layerx_sdk.agent_http import (
    AgentEnvelopeTransport,
    AgentSessionCredential,
    LayerXKeyCredential,
    _envelope_reply,
)
from layerx_sdk.ai_market import _decode_programs_call
from layerx_sdk.evm_transaction import activity_id, decode_signed
from layerx_sdk.native_program_call import encode_native_program_call
from layerx_sdk.production import ProductionClient, SecretBytes
from layerx_sdk.programs import _verify_program_transaction
from layerx_sdk.verifier import SequencerReceiptVerification
from layerx_sdk.x402_rpc import PaymentRpc

from program_receipt_evidence import ProgramReceiptEvidence
import test_agent_operation_envelope as agent_envelope
import test_native_batch_header as batch_header
import test_precompile_calls as precompiles
import test_program_account_bound as account_bound
import test_program_http as program_http
import test_program_lifecycle as lifecycle
import test_program_trust as trust
import test_program_typed_responses as typed
import test_px as px

ROOT = Path(__file__).resolve().parents[4]
LIFECYCLE_NAMES = (
    "deploy", "upgrade", "wind-down-route", "wind-down-deprecate",
    "wind-down-tombstone", "wind-down-exit",
)


@contextmanager
def recorded_program_rpc(vector):
    """Replay the node producer's signed transaction/receipt pair unchanged."""
    observed = []
    failures = []
    receipt = vector["receiptJson"]

    class Handler(BaseHTTPRequestHandler):
        def do_POST(self):
            request = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
            observed.append((self.path, self.headers["Content-Type"], request))
            if (self.path != "/rpc" or self.headers["Content-Type"] != "application/json"
                    or set(request) != {"jsonrpc", "id", "method", "params"}
                    or request["jsonrpc"] != "2.0"):
                failures.append("unexpected JSON-RPC framing")
                self.send_error(400)
                return
            if request["method"] == "eth_sendRawTransaction" and request["params"] == [vector["rawTransaction"]]:
                result = receipt["txHash"]
                answer = {"result": result}
            elif request["method"] == "pax_sequencerReceipt" and request["params"] == [receipt["txHash"]]:
                result = receipt
                answer = {"result": result}
            else:
                failures.append("request differs from recorded producer transaction")
                self.send_error(400)
                return
            encoded = json.dumps({"jsonrpc": "2.0", "id": request["id"], **answer}).encode()
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(encoded)))
            self.end_headers()
            self.wfile.write(encoded)

        def log_message(self, _format, *args):
            pass

    server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    thread = Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        yield PaymentRpc(f"http://127.0.0.1:{server.server_port}/rpc"), observed
    finally:
        server.shutdown()
        server.server_close()
        thread.join()
        assert not failures, failures


def test_agent_restart_admission_barrier_refuses_missing_authority():
    probe = agent_envelope.Probe.__new__(agent_envelope.Probe)
    probe.restart_evidence = {}
    with pytest.raises(agent_envelope.ProbeRefused, match="authenticated durable journal"):
        probe._restart_record("admission_file", {}, "pending")


def test_precompile_legacy_encoding_and_perps_rejections_remain():
    precompiles.PrecompileCallsTest().test_every_write_encodes_its_abi_calldata()


def test_launchpad_selector_mutabilities_come_from_committed_abi_artifacts():
    roles = ("Router", "SidioraFactory", "Quoter", "PoolRegistry", "FeesRouter",
             "FeeAccumulator", "ProtocolConfig")
    expected = {}
    for role in roles:
        entries = json.loads((ROOT / "precompiles/launchpad/abi" / (role + ".json")).read_text())
        for entry in entries:
            if entry["type"] == "function" and entry["stateMutability"] not in ("view", "pure"):
                expected.setdefault(precompiles.signature_of(entry), entry["stateMutability"] == "payable")
    direct = {precompiles.signature_of(entry): entry["stateMutability"] == "payable"
              for entry in precompiles.abi("launchpad")
              if entry["type"] == "function" and entry["stateMutability"] not in ("view", "pure")}
    assert direct == expected
    assert set(precompiles.LAUNCHPAD_WRITES) == set(expected)
    selectors = [precompiles.abi_selector(signature) for signature in expected]
    assert len(selectors) == len(set(selectors))


@pytest.mark.parametrize("method", [
    "test_real_native_call_verifies_with_principal_account_binding",
    "test_account_bound_authority_rejects_changed_names_and_shape",
    "test_signed_native_call_commitments_remain_bound",
    "test_current_transaction_binds_hash_sender_and_precompile",
])
def test_program_account_legacy_identity(method):
    account_bound.ProgramAccountBoundTest.setUpClass()
    getattr(account_bound.ProgramAccountBoundTest(), method)()


def test_program_account_current_identity_over_recorded_rpc():
    evidence = ProgramReceiptEvidence()
    transaction, signature = decode_signed(evidence.signed)
    expected = keccak256(evidence.signed)
    for _ in range(2):
        with recorded_program_rpc(evidence.vector) as (rpc, observed):
            assert rpc.send(evidence.vector["rawTransaction"]) == "0x" + expected.hex()
            result = rpc.receipt("0x" + expected.hex())
            assert result == evidence.vector["receiptJson"]
            assert evidence.submission(result) == result
            verified = _verify_program_transaction(
                bytes.fromhex(result["receipt"][2:]), evidence.signed,
                (evidence.key,), transaction.chain_id, evidence.execution["program_id"],
            )
            assert type(verified) is SequencerReceiptVerification
            assert verified.receipt.from_account == bytes(12) + bytes.fromhex(transaction.recover_signer(signature)[2:])
            assert verified.receipt.to_account == bytes(12) + transaction.to
            assert verified.receipt.activity_id == expected
            assert verified.receipt.from_sequence == transaction.nonce
            assert [(path, content_type) for path, content_type, _ in observed] == [("/rpc", "application/json")] * 2


@pytest.mark.parametrize("mutation", ["signature", "network", "program", "pin"])
def test_program_account_current_identity_refuses_rebinding(mutation):
    evidence = ProgramReceiptEvidence()
    raw, network, program, keys = evidence.signed, evidence.trust.network_id, evidence.execution["program_id"], (evidence.key,)
    if mutation == "signature":
        raw = raw[:-1] + bytes([raw[-1] ^ 1])
    elif mutation == "network":
        network += 1
    elif mutation == "program":
        program = "00" * 32
    else:
        keys = (replace(evidence.key, public_key=bytes(32)),)
    from layerx_sdk.production import PlatformSdkError
    with pytest.raises((ValueError, PlatformSdkError)):
        _verify_program_transaction(evidence.encoded, raw, keys, network, program)


def test_program_http_current_ingress_and_protected_inputs(tmp_path):
    evidence = ProgramReceiptEvidence()
    transaction, _ = decode_signed(evidence.signed)
    native, fee = _decode_programs_call(transaction.data)
    directory = tmp_path / "protected"
    directory.mkdir(mode=0o700)
    payload = directory / "payload"
    raw = directory / "transaction"
    for path, content in ((payload, encode_native_program_call(native)), (raw, evidence.signed)):
        path.write_bytes(content)
        path.chmod(0o600)
    case = program_http.ProgramHttp()
    case.trust = evidence.trust
    request = case.request({"guest_abi": native.guest_abi, "payload_file": str(payload),
                            "raw_transaction_file": str(raw), "fee_limit": str(fee), "expected_result_code": 0})
    assert request.transaction_hash == evidence.vector["receiptJson"]["txHash"]
    with recorded_program_rpc(evidence.vector) as (rpc, observed):
        case.rpc = rpc
        assert rpc.send("0x" + request.raw.hex()) == request.transaction_hash
        assert case.verified_receipt(request) == evidence.vector["receiptJson"]
        assert [r[2]["method"] for r in observed] == ["eth_sendRawTransaction", "pax_sequencerReceipt"]
    payload.chmod(0o644)
    with pytest.raises(ValueError, match="protected real Programs"):
        program_http.protected(payload)
    payload.chmod(0o600)
    link = directory / "linked"
    link.symlink_to(payload)
    with pytest.raises(ValueError, match="protected real Programs"):
        program_http.protected(link)
    with pytest.raises(ValueError, match="duplicate Programs"):
        json.loads('{"version":1,"version":2}', object_pairs_hook=program_http.unique)


@pytest.mark.parametrize("name", LIFECYCLE_NAMES)
def test_program_lifecycle_current_transaction_codec(name):
    fixture, raw, transaction_hash = lifecycle.current_lifecycle_transaction(name)
    transaction, signature = decode_signed(raw)
    assert transaction.chain_id == fixture["evm"]["chainId"] == 125
    assert transaction.to.hex() == "000000000000000000000000000000000000101d"
    assert transaction.data.hex() == fixture["evm"]["calldata"][2:]
    assert transaction.value == 0
    assert transaction_hash == keccak256(raw).hex() == activity_id(raw).hex()
    assert transaction.attach_signature(signature, transaction.recover_signer(signature)) == raw
    payload = bytes.fromhex(fixture["payload_hex"])
    decoder = {1: lifecycle.NativeProgramDeploy, 2: lifecycle.NativeProgramUpgrade,
               7: lifecycle.NativeProgramWindDown}[fixture["ordinal"]]
    value = decoder.decode(payload)
    assert value.encode() == payload
    entry = next(item for item in json.loads((ROOT / "precompiles/layerxprograms/abi.json").read_text())
                 if item["type"] == "function" and precompiles.abi_selector(precompiles.signature_of(item))[2:]
                 == transaction.data[:4].hex())
    assert entry["stateMutability"] == "nonpayable"
    assert entry["name"] == {"deploy": "deploy", "upgrade": "upgrade",
                              "wind-down-route": "setExitRoute",
                              "wind-down-deprecate": "deprecate",
                              "wind-down-tombstone": "tombstone",
                              "wind-down-exit": "exitValue"}[name]
    if name == "deploy":
        values = [value.program_id, value.guest_abi, value.policy, value.authority,
                  value.new_hash, value.interface or b"", value.wasm]
    elif name == "upgrade":
        values = [value.program_id, value.guest_abi, int(value.clear_interface) << 1,
                  value.old_hash, value.new_hash, value.migration_hook,
                  value.interface or b"", value.wasm, 1000]
    elif name == "wind-down-route":
        values = [value.program_id, value.account, value.asset, value.destination, value.seed]
    elif name == "wind-down-deprecate":
        values = [value.program_id, value.exit_program, value.deadline_batch]
    elif name == "wind-down-tombstone":
        values = [value.program_id]
    else:
        values = [value.program_id, value.account]
    from layerx_sdk.exchange import encode_signature_call
    values = ["0x" + item.hex() if isinstance(item, bytes) else item for item in values]
    assert encode_signature_call(precompiles.signature_of(entry), values) == fixture["evm"]["calldata"]



@pytest.mark.parametrize("method", [
    "test_boundary_refusal_and_unknown_envelopes", "test_c_signed_fixtures_and_refusals", "test_policy_flags_and_bounds",
])
def test_program_lifecycle_historical_internal_coverage(method):
    getattr(lifecycle.ProgramLifecycleTest(), method)()


@pytest.mark.parametrize("method", [
    "test_signed_validity_and_maximum_simulation_age_are_enforced",
    "test_discovery_cache_rejects_rollback_and_same_sequence_conflict",
    "test_simulation_rechecks_head_freshness_after_http",
    "test_concurrent_head_change_invalidates_inflight_simulation",
])
def test_program_trust_envelope_shape_and_freshness(method):
    getattr(trust.ProgramTrustTests(), method)()


def test_program_trust_real_producer_envelope_over_http():
    golden = json.loads((ROOT / "services/agent/schema/agent-api/golden/agent-http-success.json").read_bytes())
    observed = []

    class Handler(BaseHTTPRequestHandler):
        def do_POST(self):
            request = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
            observed.append((self.path, self.headers["Authorization"], request))
            # JSON-RPC replay remaps only the correlation id. The producer's
            # actual value and verification object are replayed unchanged.
            encoded = json.dumps({**golden, "request_id": request["request_id"]}).encode()
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(encoded)))
            self.end_headers()
            self.wfile.write(encoded)

        def log_message(self, _format, *args):
            pass

    server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    thread = Thread(target=server.serve_forever, daemon=True)
    thread.start()
    secret = SecretBytes(b"44" * 32)
    try:
        session = AgentSessionCredential("owner", "11" * 32, "22" * 32, 7)
        credential = LayerXKeyCredential("owner", secret)
        client = ProductionClient(AgentEnvelopeTransport(
            f"http://127.0.0.1:{server.server_port}", gateway_key=credential, session=session))
        response = client.agent("read.account", {"account": "11" * 32, "requested_verification_level": "submitted"})
        assert response.value == golden["value"]
        assert response.verification_status == {"state": "achieved", "rung": "submitted"}
        assert observed[0][0] == "/v1/agent/rpc"
        assert observed[0][1] == credential.use()
        assert observed[0][2]["credential"] == session.coordinates()
        assert response.request_id == observed[0][2]["request_id"]
        for invalid in ({"state": "Achieved", "rung": "submitted"}, {"state": "achieved", "level": "submitted"}):
            from layerx_sdk.production import PlatformSdkError
            with pytest.raises(PlatformSdkError):
                _envelope_reply(200, "application/json", json.dumps({**golden, "verification_status": invalid}).encode(), golden["request_id"], False)
    finally:
        secret.destroy()
        server.shutdown()
        server.server_close()
        thread.join()


@pytest.mark.parametrize("method", [
    "test_discovery_and_interface_are_typed_and_honest_about_server_verification",
    "test_current_envelope_preserves_achieved_and_unverified",
])
def test_typed_program_verification_state(method):
    getattr(typed.TypedProgramResponseTests(), method)()


@pytest.mark.parametrize("method", [
    "test_account_key_accepts_the_three_public_spellings",
    "test_chain_quantities_decode_exactly_and_never_as_floats",
    "test_balances_decode_keeps_an_unknown_half_unknown",
    "test_join_asset_table_and_network_head_decode",
    "test_gateway_reads_and_json_rpc_errors_over_loopback",
    "test_client_refuses_unsafe_endpoints_and_unknown_methods",
])
def test_px_network_fixture_shape(method):
    getattr(px.UnifiedNetworkReadTest(), method)()


@pytest.mark.parametrize("method", [
    "test_history_params_are_positional_and_bounded",
    "test_recorded_pages_decode_rows_and_joined_asset_metadata",
    "test_history_boundary_rows_decode",
    "test_explorer_row_ids_decode_as_strings",
    "test_malformed_pages_are_refused",
    "test_history_reads_round_trip_over_loopback",
])
def test_px_history_chain_pages_and_bounds(method):
    getattr(px.HistoryReadTest(), method)()


def test_px_chain_only_openrpc_contract_and_retired_anchor_refusal():
    network = px.PX_SCHEMAS["px_getNetwork"]
    balances = px.PX_SCHEMAS["px_getBalances"]
    assert network["additionalProperties"] is False
    assert set(network["required"]) == {"network_id", "paxeer", "consensus"}
    assert "anchor" not in network["properties"]
    assert "layerx" not in network["properties"]
    assert balances["additionalProperties"] is False
    assert set(balances["required"]) == {"account", "height", "balances"}
    assert set(balances["properties"]["balances"]["items"]["required"]) == {"denom", "amount"}
    bank_abi = json.loads((ROOT / "precompiles/bank/abi.json").read_text())
    coins = next(entry for entry in bank_abi if entry.get("name") == "all_balances")
    amount = next(field for field in coins["outputs"][0]["components"] if field["name"] == "amount")
    assert amount["type"] == "uint256"
    assert set(px.BALANCES) == set(balances["required"])
    assert all(set(row) == {"denom", "amount"} for row in px.BALANCES["balances"])
    assert all(side["side"] == "paxeer" for side in px.RECORDED_UNIFIED["accounts"])
    assert all(row["chain"] == row["side"] == "paxeer" for row in px.RECORDED_UNIFIED["items"])
    assert px.NETWORK_UNAVAILABLE["paxeer"]["chain_id"] == "0x7d"
    with pytest.raises(ValueError, match="px-consensus-unavailable"):
        px.decode_px_network_head(px.NETWORK_UNAVAILABLE)
    # The SDK already refuses retired finality; the producer schema refuses
    # custody/kernel fields in current balances. Typed history refusal is wired.
    for anchor in (None, {}, {"latest_finalized_batch": 41}):
        with pytest.raises(ValueError, match="invalid-px-finality"):
            px.decode_px_network_head({**px.NETWORK_UNAVAILABLE, "anchor": anchor})


def test_px_recorded_chain_125_root_answers_over_sdk_rpc():
    from urllib.parse import urlsplit

    for node in px.ROOT_RECORDINGS:
        assert urlsplit(node["endpoint"]).path == "/"
        vectors = node["exchanges"]
        observed = []
        failures = []

        class Handler(BaseHTTPRequestHandler):
            def do_POST(self):
                request = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
                observed.append(request)
                vector = vectors[len(observed) - 1]
                if self.path != "/rpc" or self.headers["Content-Type"] != "application/json" or request != vector["request"]:
                    failures.append(request)
                    self.send_error(400)
                    return
                encoded = json.dumps(vector["response"]).encode()
                self.send_response(vector["status"])
                self.send_header("Content-Type", vector["content_type"])
                self.send_header("Content-Length", str(len(encoded)))
                self.end_headers()
                self.wfile.write(encoded)

            def log_message(self, _format, *args):
                pass

        server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        thread = Thread(target=server.serve_forever, daemon=True)
        thread.start()
        try:
            rpc = PaymentRpc(f"http://127.0.0.1:{server.server_port}/rpc")
            # The source transcript uses the node root. PaymentRpc currently
            # accepts only /rpc; that production endpoint change is a wiring request.
            assert px.px_quantity(rpc.call("eth_chainId", [])) == 125
            head = rpc.call("eth_blockNumber", [])
            assert head == vectors[1]["response"]["result"]
            assert px.px_quantity(head) > 0
            balance = rpc.call("eth_getBalance", [px.EVM, "latest"])
            assert balance == vectors[2]["response"]["result"]
            assert px.px_quantity(balance) == 0
            from layerx_sdk.x402_rpc import PaymentRpcError
            for method, params in (("px_getNetwork", []), ("status", {})):
                with pytest.raises(PaymentRpcError) as caught:
                    rpc.call(method, params)
                vector = vectors[len(observed) - 1]
                assert caught.value.code == vector["response"]["error"]["code"] == -32601
                assert caught.value.data == vector["response"]["error"].get("data")
            assert len(observed) == len(vectors)
            assert not failures
        finally:
            server.shutdown()
            server.server_close()
            thread.join()


@pytest.mark.parametrize("method", [
    "test_network_head_reads_native_block_and_committed_height",
    "test_network_head_refuses_unavailable_or_retired_finality",
    "test_network_head_requires_unsigned_height_bounds",
    "test_network_head_requires_network_and_block_fields",
])
def test_px_current_network_numeric_bounds(method):
    case = batch_header.NativeBatchHeaderTest()
    case.setUp()
    getattr(case, method)()
