"""Scoped SDK proofs over committed producer evidence; see repair_22_33_702.kvx."""

import copy
import json
import threading
from contextlib import contextmanager
from dataclasses import replace
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

import pytest

from layerx_sdk.account_derivation import keccak256
from layerx_sdk.evm_transaction import (
    EvmTransactionError, LegacyTransaction, Type2Transaction, _rlp, _rlp_decode,
    decode_signed, decode_signed_transaction,
)
from layerx_sdk.production import SdkErrorCode
from layerx_sdk.verifier import SequencerReceiptVerification, SequencerReceiptVerificationError, verify_sequencer_receipt
from layerx_sdk.web_search import WebSearchError, WebSearchOffer, verify_web_search_payment
from layerx_sdk.x402_rpc import PaymentRpc, verify_rpc_payment

from payment_receipt_evidence import ROOT, payment_terms, receipt_key, receipt_vector, unhex
import test_px as px
import test_web_search as search
import test_x402 as x402
import test_xweb_api as xweb


@contextmanager
def recorded_payment_rpc(vector, *, response_change=None):
    """Replay only the exact node-produced transaction and receipt pair."""
    observed, errors = [], []
    receipt = vector["receiptJson"]

    class Handler(BaseHTTPRequestHandler):
        def do_POST(self):
            try:
                request = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
                observed.append((self.path, self.headers["Content-Type"], request))
                assert self.path == "/"
                assert self.headers["Content-Type"] == "application/json"
                assert set(request) == {"jsonrpc", "id", "method", "params"}
                assert request["jsonrpc"] == "2.0" and type(request["id"]) is int
                if request["method"] == "eth_sendRawTransaction":
                    assert request["params"] == [vector["rawTransaction"]]
                    result = receipt["txHash"]
                elif request["method"] == "pax_sequencerReceipt":
                    assert request["params"] == [receipt["txHash"]]
                    result = copy.deepcopy(receipt)
                    if response_change is not None:
                        response_change(result)
                else:
                    raise AssertionError("request has no producer recording")
                encoded = json.dumps({"jsonrpc": "2.0", "id": request["id"], "result": result}).encode()
                self.send_response(200)
            except Exception as error:
                errors.append(error)
                encoded = json.dumps({"error": str(error)}).encode()
                self.send_response(500)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(encoded)))
            self.end_headers()
            self.wfile.write(encoded)

        def log_message(self, _format, *args):
            pass

    server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        yield PaymentRpc(f"http://127.0.0.1:{server.server_port}"), observed
    finally:
        server.shutdown()
        server.server_close()
        thread.join()
        assert not errors, errors


def test_px_retired_rpc_path_matches_gateway_contract():
    source = (ROOT / "services/platform/hosted/gateway/src/main.rs").read_text()
    assert 'if path == "/rpc"' in source
    assert 'return response(404, "not_found", None);' in source
    method = next(item for item in px.OPENRPC["methods"] if item["name"] == "px_getBalances")
    assert method["paramStructure"] == "by-position"
    assert set(px.BALANCES) == set(method["result"]["schema"]["required"])
    px.UnifiedNetworkReadTest().test_gateway_reads_and_json_rpc_errors_over_loopback()
    px.HistoryReadTest().test_history_reads_round_trip_over_loopback()


def test_px_invalid_url_and_unsupported_method_refusals_remain_independent():
    px.UnifiedNetworkReadTest().test_client_refuses_unsafe_endpoints_and_unknown_methods()


def test_x402_retired_receipt_proof_real_rpc():
    vector = receipt_vector()
    raw = unhex(vector["rawTransaction"])
    terms = payment_terms(vector)
    for _ in range(2):
        # Reopen the consumer/transport and verify the original signed evidence;
        # the replay does not claim a new node execution or restart recovery.
        with recorded_payment_rpc(vector) as (rpc, observed):
            transaction_hash = rpc.send(vector["rawTransaction"])
            assert transaction_hash == "0x" + keccak256(raw).hex()
            result = rpc.receipt(transaction_hash)
            assert result == vector["receiptJson"]
            payment = verify_rpc_payment(
                result, transaction_hash, "0x" + terms["payer"],
                receipt_keys=(receipt_key(vector),), raw_transaction=raw,
                **{name: terms[name] for name in ("amount", "asset", "pay_to")},
            )
            assert payment == {
                "txHash": transaction_hash, "rung": "receipted",
                "height": int(result["height"], 16), "keyId": result["keyId"],
            }
            assert [row[2]["method"] for row in observed] == ["eth_sendRawTransaction", "pax_sequencerReceipt"]
            assert [row[2]["id"] for row in observed] == [1, 2]
    assert "blockHash" not in result
    with pytest.raises(ValueError, match="^commitment-mismatch$"):
        verify_rpc_payment(
            result, transaction_hash, "0x" + terms["payer"],
            receipt_keys=(receipt_key(vector),), raw_transaction=raw,
            commitment="committed", **{name: terms[name] for name in ("amount", "asset", "pay_to")},
        )


@pytest.mark.parametrize("field", ("payer", "asset", "pay_to", "amount", "height", "key", "raw", "signature"))
def test_x402_rpc_payment_refuses_rebinding(field):
    vector = receipt_vector()
    terms, key = payment_terms(vector), receipt_key(vector)
    raw = unhex(vector["rawTransaction"])

    def change(result):
        if field == "height":
            result["height"] = hex(int(result["height"], 16) + 1)
        elif field == "signature":
            canonical = unhex(result["receipt"])
            result["receipt"] = "0x" + (canonical[:-1] + bytes([canonical[-1] ^ 1])).hex()

    with recorded_payment_rpc(vector, response_change=change) as (rpc, observed):
        result = rpc.receipt(vector["receiptJson"]["txHash"])
        assert len(observed) == 1
    if field in ("payer", "asset", "pay_to"):
        terms[field] = "ab" * 32
    elif field == "amount":
        terms[field] = str(int(terms[field]) + 1)
    elif field == "key":
        key = replace(key, first_height=int(result["height"], 16) + 1)
    elif field == "raw":
        raw = raw[:-1] + bytes([raw[-1] ^ 1])
    with pytest.raises(ValueError) as raised:
        verify_rpc_payment(
            result, vector["receiptJson"]["txHash"], "0x" + terms.pop("payer"),
            receipt_keys=(key,), raw_transaction=raw, **terms,
        )
    if field in ("signature", "key"):
        assert str(raised.value) == "payment-binding-mismatch"
        assert isinstance(raised.value.__cause__, SequencerReceiptVerificationError)
        assert raised.value.__cause__.check == {"signature": "bad_signature", "key": "key_out_of_range"}[field]
        assert raised.value.__cause__.code == SdkErrorCode.VERIFICATION_FAILURE
    else:
        assert str(raised.value) == ("transaction-hash-mismatch" if field == "raw" else "payment-binding-mismatch")


@pytest.mark.parametrize("case,method", [
    (case, name)
    for case in (x402.VerifyPaymentReceiptTest, x402.PaymentRpcProtocolTest, x402.PaymentRevertBindingTest)
    for name in sorted(vars(case)) if name.startswith("test_")
])
def test_x402_current_receipts_and_retained_protocol_refusals(case, method):
    case.setUpClass()
    getattr(case(method), method)()


def test_x402_committed_recording_keeps_the_actual_block_identity():
    source = json.loads((ROOT / "daemon/committed-fixture/vectors/vector-v4-committed.json").read_text())
    corpus = json.loads((ROOT / "sdk/platform/conformance/fixtures/vector-v4-committed.json").read_text())
    settlement = source["settlement"]
    receipt = settlement["receipt"]
    raw = unhex(settlement["raw_transaction"])
    transaction, signature = decode_signed_transaction(raw)
    assert isinstance(transaction, Type2Transaction)
    transaction_hash = keccak256(raw)
    assert receipt["txHash"] == "0x" + transaction_hash.hex()
    vector = {**corpus["positive"], "receiptJson": receipt}
    assert vector["rawTransaction"] == settlement["raw_transaction"]
    assert vector["signedReceipt"] == receipt["receipt"]
    terms = payment_terms(vector)
    signed = verify_sequencer_receipt(unhex(receipt["receipt"]), raw, (receipt_key(vector),))
    assert transaction.recover_signer(signature)[2:].lower().zfill(64) == signed.receipt.from_account.hex()
    with recorded_payment_rpc(vector) as (rpc, observed):
        assert rpc.send(vector["rawTransaction"]) == receipt["txHash"]
        result = rpc.receipt(receipt["txHash"])
        assert result == receipt
        with pytest.raises(ValueError, match="^payment-binding-mismatch$") as refused:
            verify_rpc_payment(
                result, receipt["txHash"], "0x" + terms.pop("payer"),
                receipt_keys=(receipt_key(vector),), raw_transaction=raw,
                commitment="committed", **terms,
            )
        assert refused.value.__cause__.code == SdkErrorCode.VERIFICATION_FAILURE
        with pytest.raises(ValueError, match="^missing-committed-evidence$"):
            verify_rpc_payment(
                result, receipt["txHash"], "0x" + signed.receipt.from_account.hex(),
                receipt_keys=(receipt_key(vector),), raw_transaction=raw,
                **terms,
            )
        assert len(observed) == 2
    block_hash = receipt["blockHash"].removeprefix("0x").lower()
    assert block_hash == settlement["execution_commit"]["block_id"]["hash"].lower()
    assert block_hash == settlement["header"]["last_block_id"]["hash"].lower()
    assert int(receipt["blockNumber"], 16) == int(settlement["execution_header"]["height"])


def test_recorded_eip155_decoder_preserves_canonical_and_signature_refusals():
    archive = json.loads((ROOT / "services/agent/sdk/python/tests/fixtures/vector-v4-chain713714.json").read_text())
    raw = unhex(archive["settlement"]["raw_transaction"])
    assert isinstance(decode_signed_transaction(raw)[0], LegacyTransaction)
    with pytest.raises(EvmTransactionError, match="^invalid_transaction$"):
        decode_signed(raw)
    with pytest.raises(EvmTransactionError, match="^invalid_rlp$"):
        decode_signed_transaction(raw + b"\x00")
    fields = _rlp_decode(raw)
    replacements = (
        (0, b"\x00" + fields[0], "invalid_transaction"),
        (3, fields[3][1:], "invalid_transaction"),
        (6, b"\x1b", "invalid_signature"),
        (6, b"\x24", "invalid_signature"),
        (7, b"", "invalid_signature"),
        (8, b"\xff" * 32, "invalid_signature"),
    )
    for index, replacement, error in replacements:
        changed = fields[:index] + (replacement,) + fields[index + 1:]
        with pytest.raises(EvmTransactionError, match=f"^{error}$"):
            decode_signed_transaction(_rlp(changed))
    for raw in (b"", b"\x01\xc0", b"\x03\xc0"):
        with pytest.raises(EvmTransactionError, match="^invalid_transaction$"):
            decode_signed_transaction(raw)
    vector = receipt_vector()
    raw = unhex(vector["rawTransaction"])
    assert decode_signed_transaction(raw) == decode_signed(raw)


def test_committed_payment_log_selection_refuses_missing_duplicate_and_rebound_events():
    corpus = json.loads((ROOT / "sdk/platform/conformance/fixtures/vector-v4-committed.json").read_text())
    original = corpus["positive"]
    for field in ("missing", "duplicate", "reference", "from", "to", "data"):
        vector = copy.deepcopy(original)
        logs = vector["receiptJson"]["logs"]
        assert len(logs) == 2
        payment = logs[1]
        if field == "missing":
            logs.pop()
        elif field == "duplicate":
            logs.append(copy.deepcopy(payment))
        elif field == "data":
            data = unhex(payment["data"])
            payment["data"] = "0x" + (data[:-1] + bytes([data[-1] ^ 1])).hex()
        else:
            payment["topics"][{"reference": 1, "from": 2, "to": 3}[field]] = "0x" + "ab" * 32
        error = (
            "expected exactly one recorded PaymentExact log"
            if field in ("missing", "duplicate")
            else "payment log differs from signed calldata"
        )
        with pytest.raises(AssertionError, match=f"^{error}$"):
            payment_terms(vector)


def test_search_payer_address_binds_the_recorded_offer_over_http():
    import base64

    with search._Replay(copy.deepcopy(search.EXCHANGE)) as peer:
        client = search.WebSearchClient(
            peer.endpoint, "eip155:125", search.ASSETS["PAX"]["asset_id"],
            search._terms(), search._metered_sid(), (search.RECEIPT_KEY,),
            payer_rpc=PaymentRpc(peer.endpoint),
            payer_address=search.PAYER_ADDRESS,
        )
        response = client._get("/search?q=paxeer", {})
        assert response.status == 402
        required = json.loads(base64.b64decode(response.headers["payment-required"]))
        offer, raw = client._select(required, search.PAYER_DID)
        grant = search.decode_payer_grant(bytes.fromhex(search.BUYER["grants"]["SID"]))
        assert offer.payer == raw["extra"]["layerx"]["payer"] == grant["from"]
        assert search._derived(
            search.PAYER_DID, offer.asset, search.ASSETS["PAX"]["asset_id"],
            "offer-payer-mismatch",
        )[1] != offer.payer
        rebound = copy.deepcopy(required)
        for candidate in rebound["accepts"]:
            if candidate["scheme"] == "metered":
                candidate["extra"]["layerx"]["payer"] = "ab" * 32
        with pytest.raises(WebSearchError, match="^offer-payer-mismatch$"):
            client._select(rebound, search.PAYER_DID)
        for address in ("", "0xab", "0x" + "ab" * 21, search.PAYER_DID):
            with pytest.raises(WebSearchError, match="^offer-payer-mismatch$"):
                search._payer_account(address, offer.asset, search.ASSETS["PAX"]["asset_id"])
        assert peer.served == [0]
        assert peer.unrecorded == []


@pytest.mark.parametrize("method", (
    "test_content_vectors", "test_derived_accounts_match_payment_vectors",
    "test_derived_accounts_refuse_noncanonical_selectors", "test_payer_grants_round_trip",
    "test_payment_reverts_keep_registry_code_and_retryability",
    "test_payment_reverts_refuse_malformed_return_data", "test_nonrevert_payment_errors_keep_the_original_error",
    "test_recorded_receipts_carry_sequencer_protocol_version_two",
    "test_version_three_layouts_and_unsupported_versions_are_refused",
    "test_untrusted_sequencer_receipt_is_never_sent",
))
def test_web_search_current_receipt_and_refusal_contracts(method):
    getattr(search.WebSearchClientTest(), method)()


def test_web_search_authorized_batch_cannot_replace_raw_payment_binding():
    vector = receipt_vector()
    verified = verify_sequencer_receipt(
        unhex(vector["signedReceipt"]), unhex(vector["rawTransaction"]), (receipt_key(vector),),
    )
    assert type(verified) is SequencerReceiptVerification
    assert verified.receipt.activity_id == keccak256(unhex(vector["rawTransaction"]))
    terms = payment_terms(vector)
    offer = WebSearchOffer(
        scheme="exact", network="eip155:125", amount=terms["amount"],
        asset=terms["asset"], pay_to=terms["pay_to"], payer=decode_signed_transaction(unhex(vector["rawTransaction"]))[0].data[4:36].hex(),
        max_timeout_seconds=60, currency="USDC", account="agent:did:layerx:seller:main",
    )
    assert verified.receipt.amount == 0
    assert verified.receipt.to_account.hex() != offer.pay_to
    raw = unhex(vector["rawTransaction"])
    keys = (receipt_key(vector),)
    verify_web_search_payment(verified, offer,
                              raw_transaction=raw, receipt_keys=keys)
    with pytest.raises(WebSearchError) as rebound:
        verify_web_search_payment(verified, replace(offer, payer="ab" * 32),
                                  raw_transaction=raw, receipt_keys=keys)
    assert rebound.value.code == "receipt-unverified"
    assert rebound.value.__cause__.field == "payment"
    for changed_raw, changed_keys in ((raw + b"\0", keys), (raw, ())):
        with pytest.raises(WebSearchError) as raised:
            verify_web_search_payment(verified, offer,
                                      raw_transaction=changed_raw, receipt_keys=changed_keys)
        assert raised.value.code == "receipt-unverified"


def test_web_search_recorded_http_producer_contract_is_not_rewritten():
    for currency in search.CURRENCIES:
        paid = search.EXCHANGE[search._fetch_index(currency)]
        payload = paid["request"]["headers"]["PAYMENT-SIGNATURE"]["payload"]
        assert set(payload) == {"idempotencyKey", "receiptDigest", "txHash", "verificationLevel"}
        assert "receipt" not in payload and "rawTransaction" not in payload
        assert payload == search.BUYER["exact"][currency]
        assert "rawTransaction" in search.EXACT_RPC["payments"][currency]
    metered = search.SEARCH_PAID["request"]["headers"]["PAYMENT-SIGNATURE"]["payload"]
    assert set(metered) == {"grantId", "idempotencyKey"}
    assert "LAYERX-PAYER-ADDRESS" in search.SEARCH_PAID["request"]["headers"]


def test_web_search_recorded_content_digest_and_tamper_refusals_over_http():
    import base64

    first, second = search.VECTORS[:2]
    recorded = base64.b64decode(search.EXCHANGE[10]["response"]["bodyBase64"])
    assert search.content_digest(recorded) == first["digest"]
    with search._Replay(copy.deepcopy(search.EXCHANGE), {f"/content/{second['digest']}": recorded}) as peer:
        client = search._client(peer.endpoint, search._exact_usdc())
        content = client.content(first["digest"])
        assert content.digest == first["digest"]
        assert content.content.text == first["text"]
        assert content.content.payload.decode() == first["payload"]
        assert content.settlement is None
        with pytest.raises(WebSearchError) as raised:
            client.content(second["digest"])
        assert raised.value.code == "content-digest-mismatch"
        with pytest.raises(WebSearchError) as raised:
            client.content("zz")
        assert raised.value.code == "invalid-digest"
        assert peer.served == [10]
        assert peer.unrecorded == []


def test_legacy_exchange_write_proof_retains_historical_vectors_and_current_direct_codecs():
    case = xweb.XWebApiVectorTests()
    case.test_historical_exchange_writes_match_the_typescript_legacy_signatures()
    case.test_exchange_argument_refusals()
    case.test_current_direct_order_and_position_calldata()


def test_payment_baseline_entries_are_retired_and_the_transport_baseline_is_preserved():
    import subprocess

    path = "tests/ci/data/agent_sdk_python_baseline.json"
    original = json.loads(subprocess.check_output(
        ["git", "show", "3a0f70d0fce59d96719043d5294b98aa3718e3bd:" + path],
        cwd=ROOT, text=True,
    ))
    retained = json.loads((ROOT / path).read_text())
    repaired_paths = {
        "tests/agent/sdk/python/" + name for name in (
            "test_ai_market.py", "test_web_search.py", "test_x402.py",
            "test_repair_22_33_16.py", "test_repair_22_33_702.py",
        )
    }
    retired = [entry for entry in original if entry["nodeid"].split("::")[0] in repaired_paths]
    assert len(retired) == 48
    assert all(entry["retiring_task"] == "22.39.120" for entry in retired)
    assert all(entry in original for entry in retained)
    assert not any(entry in retired for entry in retained)
