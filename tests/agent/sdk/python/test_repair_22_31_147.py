"""Task 22.31.147: real codecs, recorded node/sidecar exchanges and HTTP consumers.

The retired payer-grant path refuses with the ledger producer's registered reason;
no successful draw receipt is accepted or invented here.
"""
from __future__ import annotations

import base64
import copy
import json
from pathlib import Path
import sqlite3
import sys
import threading
from contextlib import contextmanager
from dataclasses import replace
from hashlib import sha256
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.request import Request, urlopen

import pytest
from cryptography.hazmat.primitives import hashes, serialization
from cryptography.hazmat.primitives.asymmetric import ec, utils

from layerx_sdk import ai_market as market
from layerx_sdk.account_derivation import _N, _recover, keccak256, layerx_did_call
from layerx_sdk.agent_http import AgentHttpTransport
from layerx_sdk.committed import CommitTrust, _header_hash, _vote_bytes, verify_commit
from layerx_sdk.evm_transaction import EvmTransactionError, Type2Transaction, decode_abi, decode_call, decode_signed, encode_call
from layerx_sdk.mirror import MirrorCandidate, MirrorPolicy, MirrorVerifier, MirrorVerificationError
from layerx_sdk.native_program_call import NativeProgramCall, encode_native_program_call
from layerx_sdk.production import PlatformSdkError, ProductionClient
from layerx_sdk.program_lifecycle import (
    NativeProgramDeploy, NativeProgramUpgrade, NativeProgramWindDown, NativeProgramLifecycleRequest,
)
from layerx_sdk.program_wire import (
    bind_retained_program_call, bind_signed_program_lifecycle, decode_signed_program_call,
    verify_native_program_call_signature,
)
from layerx_sdk.programs import NativeProgramRequest, ProgramOperations, ProgramTrustContext
from layerx_sdk.verifier import ReceiptKey, SequencerReceiptVerificationError, verify_sequencer_receipt
from layerx_sdk.web_search import (
    WebSearchAssetTerms, WebSearchClient, WebSearchError, WebSearchOffer, WebSearchPreference, WebSearchMeteredPayment,
    verify_sequencer_receipt as verify_web_payment,
)
from layerx_sdk.x402 import LEDGER_DRAW_TYPES, LedgerDrawTerms, verify_payment_receipt
from layerx_sdk.revert import RevertReasonError, decode_revert_reason
from layerx_sdk.x402_activity import bind_receive_activity
from layerx_sdk.x402_agent import AgentGrantMiddleware, PaymentBudget
from layerx_sdk.x402_receive import decode_receive, encode_receive
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
from layerx_sdk.x402_draw import PreparedGrantDraws
from layerx_sdk.x402_http import (
    BuyerMiddleware, ConfiguredReceiptAuthority, FulfillmentStore, PaymentEvidence, SellerMiddleware,
    decode_header, encode_header,
)
from layerx_sdk.x402_rpc import PaymentRpc, decode_receipt_keys, verify_rpc_payment

ROOT = Path(__file__).resolve().parents[4]
SDK = ROOT / "services/agent/sdk/python/layerx_sdk"
sys.path.insert(0, str(SDK.parent / "tests"))
from test_ai_market_commit import signatures, signed_operation, pending
from test_x402_draw_sequencer import _grant_inputs

VECTORS = json.loads((ROOT / "sdk/platform/conformance/fixtures/sequencer-receipt-v1.json").read_text())
PAYMENT = next(value for value in VECTORS["cases"] if value["id"] == "V1-layerxledger-pay-exact")
WEB_FIXTURES = ROOT / "services/interop/crates/x-websearch/tests/fixtures"


def raw(value):
    return bytes.fromhex(value.removeprefix("0x"))


def pin(vector=PAYMENT):
    value = vector["pinnedKey"]
    return ReceiptKey(raw(value["keyId"]), raw(value["publicKey"]), value["firstHeight"], value["lastHeight"])


def payment_terms(vector=PAYMENT):
    log = vector["receiptJson"]["logs"][0]
    data = raw(log["data"])
    return dict(amount=str(int.from_bytes(data[32:], "big")), asset=data[:32].hex(),
                pay_to=raw(log["topics"][3]).hex(), payer=raw(log["topics"][2]).hex())


@contextmanager
def http_server(dispatch, *, tls=None):
    errors = []
    class Handler(BaseHTTPRequestHandler):
        def log_message(self, *args):
            pass
        def do_GET(self):
            self.exchange()
        def do_POST(self):
            self.exchange()
        def exchange(self):
            try:
                body = self.rfile.read(int(self.headers.get("Content-Length", "0")))
                status, headers, result = dispatch(self.command, self.path, self.headers, body)
                if not isinstance(result, bytes):
                    result = json.dumps(result, separators=(",", ":")).encode()
                self.send_response(status)
                for key, value in headers.items():
                    self.send_header(key, value)
                self.send_header("Content-Length", str(len(result)))
                self.end_headers()
                self.wfile.write(result)
            except Exception as error:
                errors.append(error)
                self.send_error(500)
    server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    if tls is not None:
        server.socket = tls.wrap_socket(server.socket, server_side=True)
    worker = threading.Thread(target=server.serve_forever, daemon=True)
    worker.start()
    try:
        scheme = "https" if tls else "http"
        yield f"{scheme}://127.0.0.1:{server.server_port}"
    finally:
        server.shutdown()
        server.server_close()
        worker.join()
        assert not errors, repr(errors)


@pytest.fixture(scope="module")
def receipt_producer():
    """In-process sequencer peer: canonical receipt framing and a real signing key."""
    key = Ed25519PrivateKey.from_private_bytes(b"\x33" * 32)
    def produce(record):
        transaction, signature = decode_signed(record.signed_bytes)
        sender = raw(transaction.recover_signer(signature))
        def integer(value, size):
            return value.to_bytes(size, "big")
        def bounded(value):
            return integer(len(value), 4) + value
        zero = bytes(32)
        receipt = (
            b"\x00\x02\x52\x01\x00\x02" + bounded(record.activity_id)
            + integer(9001, 8) + bounded(zero) + bounded(b"\x44" * 32)
            + bounded(b"\x55" * 32) + integer(0, 4) + integer(0, 4)
            + integer(0, 16) + bounded(integer(1200, 32))
            + integer(12, 2) + integer(1, 4) + integer(0, 4) + b"\x02"
            + bounded(zero) + integer(0, 16) + bounded(bytes(12) + sender)
            + integer(0, 16) * 2 + integer(transaction.nonce, 8)
            + bounded(bytes(12) + transaction.to) + integer(0, 16) * 2
            + bounded(zero) * 2 + bounded(b"\x66" * 32) + integer(1700000000000, 8)
        )
        digest = sha256(b"LXP/v1/receipt\0" + receipt + b"\0").digest()
        return receipt + b"\x01" + bounded(key.sign(digest))
    return produce


def produced_receipt(producer, record):
    return producer(record)


@pytest.fixture(scope="module")
def commit_producer():
    """In-process validator peer, using the committed Comet transcript's wire codec.

    The codec is separately checked against light_commit_v1.json without re-signing
    that recording. Each scenario here commits to its actual retained transaction.
    """
    template = json.loads((ROOT / "consensus/types/testdata/light_commit_v1.json").read_text())
    keys = [Ed25519PrivateKey.from_private_bytes(sha256(bytes([i])).digest()) for i in range(1, 5)]
    keys.sort(key=lambda key: sha256(key.public_key().public_bytes_raw()).digest()[:20])
    validators = [dict(address=sha256(key.public_key().public_bytes_raw()).digest()[:20].hex(),
                       public_key=key.public_key().public_bytes_raw().hex(), voting_power=10) for key in keys]
    def merkle(leaves):
        if len(leaves) == 1:
            return sha256(b"\0" + leaves[0]).digest()
        split = 1 << ((len(leaves) - 1).bit_length() - 1)
        return sha256(b"\1" + merkle(leaves[:split]) + merkle(leaves[split:])).digest()
    def produce(transaction, height):
        header = copy.deepcopy(template["header"])
        header["height"] = str(height)
        header["validators_hash"] = merkle([b"\x0a\x22\x0a\x20" + raw(value["public_key"]) + b"\x10\x0a" for value in validators]).hex()
        header["next_validators_hash"] = header["validators_hash"]
        transactions = [transaction, b"other retained transaction", b"third retained transaction"]
        header["data_hash"] = merkle([sha256(value).digest() for value in transactions]).hex()
        block_hash = _header_hash(header)
        commit = copy.deepcopy(template["commit"])
        commit["height"] = height
        commit["block_id"]["hash"] = block_hash.hex()
        for signature, validator, key in zip(commit["signatures"], validators, keys):
            signature["validator_address"] = validator["address"]
            sign_bytes = _vote_bytes(header["chain_id"], commit, signature)
            signature["signature"] = key.sign(sign_bytes).hex()
            signature["vote_sign_bytes"] = sign_bytes.hex()
        return dict(header=header, header_hash=block_hash.hex(), commit=commit, validators=validators,
                    proof=dict(data=transaction.hex(), index=0, total=3,
                               aunts=[merkle([sha256(value).digest()]).hex() for value in transactions[1:]]))
    return produce


def finality_bundle(produced):
    evidence = {field: produced[field] for field in ("header", "commit", "proof")}
    trust = CommitTrust(produced["header"]["chain_id"], produced["commit"]["height"], tuple(produced["validators"]))
    return trust, evidence


@pytest.mark.parametrize("has_landing", (False, True))
def test_unverified_market_finality_and_restart(tmp_path, signatures, signed_operation, receipt_producer, commit_producer, has_landing):
    signed = signed_operation
    journal = market.OperationJournal(tmp_path, signatures)
    record = pending(journal, signed)
    receipt = produced_receipt(receipt_producer, record)
    public = serialization.Encoding.Raw
    from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
    key = ReceiptKey(b"\x66" * 32, Ed25519PrivateKey.from_private_bytes(b"\x33" * 32).public_key().public_bytes(public, serialization.PublicFormat.Raw), 1200, 1200)
    produced = commit_producer(record.signed_bytes, 1202)
    landing = {"transactionHash": "0x" + record.activity_id.hex(), "blockNumber": hex(1202),
               "blockHash": "0x" + produced["header_hash"], "status": "0x1"} if has_landing else None
    journal.resolve(record, receipt, [key], transaction_receipt=landing)
    assert record.state is market.SdkState.EXECUTED and record.rung is market.Rung.RECEIPTED
    with pytest.raises(market.AiMarketError):
        journal.finalize(record, market.CommitFinality(1200, b"a"*32, b"b"*32, b"c"*32))
    trust, evidence = finality_bundle(produced)
    recovered = market.OperationJournal(tmp_path, signatures, commit_trust=trust)
    restored = recovered.load(record.activity_id)
    assert restored.signed_bytes == signed.signed_bytes
    assert restored.receipt_block_hash == (raw(landing["blockHash"]) if has_landing else None)
    assert restored.receipt_height == (1202 if has_landing else 1200)
    bad = copy.deepcopy(evidence)
    bad["proof"]["data"] = "ff"
    with pytest.raises(market.AiMarketError):
        recovered.finalize(restored, bad)
    assert recovered.load(record.activity_id).rung is market.Rung.RECEIPTED
    recovered.finalize(restored, evidence)
    reopened = market.OperationJournal(tmp_path, signatures, commit_trust=trust).load(record.activity_id)
    assert reopened.state is market.SdkState.FINALIZED and reopened.rung is market.Rung.COMMITTED
    assert reopened.receipt_block_hash == bytes.fromhex(produced["header_hash"])
    assert reopened.receipt_height == 1202


def test_commit_codec_matches_recorded_comet_producer():
    fixture = json.loads((ROOT / "consensus/types/testdata/light_commit_v1.json").read_text())
    assert _header_hash(fixture["header"]).hex() == fixture["header_hash"]
    for signature in fixture["commit"]["signatures"]:
        assert _vote_bytes(fixture["header"]["chain_id"], fixture["commit"], signature).hex() == signature["vote_sign_bytes"]
    assert verify_commit(fixture["header"], fixture["commit"], trusted_validators=tuple(fixture["validators"]), chain_id=fixture["header"]["chain_id"]).hex() == fixture["header_hash"]
    altered = copy.deepcopy(fixture)
    altered["commit"]["signatures"] = altered["commit"]["signatures"][:2]
    with pytest.raises(ValueError, match="insufficient"):
        verify_commit(altered["header"], altered["commit"], trusted_validators=tuple(fixture["validators"]), chain_id=fixture["header"]["chain_id"])
    altered["commit"]["signatures"] *= 2
    with pytest.raises(ValueError, match="signer"):
        verify_commit(altered["header"], altered["commit"], trusted_validators=tuple(fixture["validators"]), chain_id=fixture["header"]["chain_id"])
    with pytest.raises(ValueError):
        verify_commit(fixture["header"], fixture["commit"], trusted_validators=tuple(fixture["validators"]), chain_id="other")


def sign_transaction(data, *, target="101d", chain=125, nonce=7):
    secret = ec.derive_private_key(42, ec.SECP256K1())
    point = secret.public_key().public_bytes(serialization.Encoding.X962, serialization.PublicFormat.UncompressedPoint)[1:]
    address = "0x" + keccak256(point)[-20:].hex()
    tx = Type2Transaction(chain, nonce, 1, 2, 2_000_000, bytes.fromhex(target.rjust(40, "0")), 0, data)
    digest = tx.signing_digest()
    r, s = utils.decode_dss_signature(secret.sign(digest, ec.ECDSA(utils.Prehashed(hashes.SHA256()))))
    s = min(s, _N-s)
    parity = next(value for value in (0, 1) if _recover(digest, r, s, value) == point)
    signature = r.to_bytes(32, "big") + s.to_bytes(32, "big") + bytes([27+parity])
    return tx.attach_signature(signature, address), address


def test_program_call_envelope_against_node_producer():
    vector = next(value for value in VECTORS["cases"] if value["id"] == "V1-layerxprograms-call")
    canonical = raw(vector["rawTransaction"])
    tx, signature = decode_signed(canonical)
    native, fee = market._decode_programs_call(tx.data)
    request = NativeProgramRequest(native, fee, canonical)
    decoded = decode_signed_program_call(request)
    assert decoded.activity_id == vector["receiptJson"]["txHash"][2:]
    assert decoded.canonical_bytes == canonical and decoded.sender.lower() == tx.recover_signer(signature).lower()
    assert decoded.nonce == tx.nonce
    verify_native_program_call_signature(canonical, None, 125)
    digest, abi, identity = bind_retained_program_call(canonical, decoded.activity_id, native.program_id.hex(), 3)
    assert digest == sha256(b"LXP/v1/payload-hash\0" + encode_native_program_call(native)).digest()
    assert abi == native.guest_abi and identity == decoded.activity_id
    for altered in (replace(native, entrypoint="other"), replace(native, capabilities=b"bad"), replace(native, resources=(1,)*7), replace(native, calldata=b"other")):
        with pytest.raises(ValueError):
            decode_signed_program_call(NativeProgramRequest(altered, fee, canonical))
    with pytest.raises(ValueError):
        decode_signed_program_call(NativeProgramRequest(native, fee+1, canonical))
    for changed in (dict(chain=126), dict(target="101a")):
        wrong, _ = sign_transaction(tx.data, **changed)
        with pytest.raises(ValueError):
            verify_native_program_call_signature(wrong, None, 125)


def test_lifecycle_transaction_binding_and_retained_http_bytes(tmp_path):
    wasm = b"\0asm\1\0\0\0"
    program = b"\x11" * 32
    digest = sha256(wasm).digest()
    deploy = NativeProgramDeploy(program, 5, bytes(32), digest, wasm)
    upgrade = NativeProgramUpgrade(program, 5, b"\x22"*32, digest, wasm)
    cases = [
        (1, deploy, "deploy", ("bytes32", "uint16", "uint8", "bytes32", "bytes32", "bytes", "bytes"),
         (program, 5, 0, bytes(32), digest, b"", wasm)),
        (2, upgrade, "upgrade", ("bytes32", "uint16", "uint8", "bytes32", "bytes32", "bytes", "bytes", "bytes", "uint128"),
         (program, 5, 0, b"\x22"*32, digest, b"", b"", wasm, 100)),
        (7, NativeProgramWindDown(program, 2, exit_program=b"\x33"*32, deadline_batch=9), "deprecate", ("bytes32", "bytes32", "uint64"), (program, b"\x33"*32, 9)),
        (7, NativeProgramWindDown(program, 3), "tombstone", ("bytes32",), (program,)),
        (7, NativeProgramWindDown(program, 4, account=b"\x44"*32), "exitValue", ("bytes32", "bytes32"), (program, b"\x44"*32)),
        (7, NativeProgramWindDown(program, 1, account=b"\x44"*32, asset=b"\x55"*32, destination=b"\x66"*32, seed=b"seed"), "setExitRoute", ("bytes32", "bytes32", "bytes32", "bytes32", "bytes"), (program,b"\x44"*32,b"\x55"*32,b"\x66"*32,b"seed")),
    ]
    received = []
    def dispatch(method, path, headers, body):
        assert method == "POST" and path == "/"
        assert headers["Content-Type"] == "application/json"
        request = json.loads(body)
        assert set(request) == {"jsonrpc", "id", "method", "params"}
        assert request["jsonrpc"] == "2.0"
        assert isinstance(request["id"], int)
        assert request["method"] == "eth_sendRawTransaction"
        assert request["params"] == ["0x" + canonical.hex()]
        transaction = raw(request["params"][0])
        key = headers["Idempotency-Key"]
        assert key == keccak256(transaction).hex() == identity
        received.append(transaction)
        return 200, {"Content-Type":"application/json"}, {
            "jsonrpc": "2.0", "id": request["id"], "result": "0x" + key,
        }
    with http_server(dispatch) as endpoint:
        transport = AgentHttpTransport(endpoint)
        for ordinal, value, name, kinds, values in cases:
            canonical, sender = sign_transaction(encode_call(name, kinds, values))
            request = NativeProgramLifecycleRequest(ordinal, value.encode(), canonical)
            identity = keccak256(canonical).hex()
            bound = request.bind(identity)
            assert bound.sender.lower() == sender.lower() and bound.nonce == 7
            assert bound.activity_id == identity and bound.canonical_bytes == canonical
            retained = tmp_path / (name + ".transaction")
            retained.write_bytes(canonical)
            restored = bind_signed_program_lifecycle(retained.read_bytes(), value.encode(), ordinal, identity)
            assert restored == bound
            operation = {1:"program.deploy", 2:"program.upgrade", 7:"program.wind-down"}[ordinal]
            result = transport.call("agent", operation, {"signed_activity":canonical.hex()}, identity)
            assert result == {
                "state": "submitted", "activity_id": identity,
                "idempotency_key": identity, "retained_signed_activity": canonical.hex(),
            }
            assert received[-1] == canonical == retained.read_bytes()
            assert raw(result["retained_signed_activity"]) == restored.canonical_bytes
            wrong, _ = sign_transaction(encode_call(name, kinds, values), chain=126)
            with pytest.raises(ValueError):
                NativeProgramLifecycleRequest(ordinal, value.encode(), wrong)
    assert len(received) == len(cases)


def required_payment(commitment="receipted"):
    terms = payment_terms()
    return {"x402Version":2, "resource":{"url":"https://seller.example/resource"}, "accepts":[{
        "scheme":"exact", "network":"layerx:125", "amount":terms["amount"], "asset":terms["asset"], "payTo":terms["pay_to"],
        "maxTimeoutSeconds":30, "extra":{"layerx":{"commitment":commitment, "payer":terms["payer"]}}
    }]}


def receipt_header(required):
    canonical = raw(PAYMENT["signedReceipt"])
    return encode_header({"x402Version":2, "resource":required["resource"], "accepted":required["accepts"][0],
        "payload":{"receipt":base64.b64encode(canonical).decode(), "receiptDigest":sha256(b"LXP/v1/merkle-leaf\0"+canonical).hexdigest(), "verificationLevel":"sequencer-signed"}})


def test_buyer_send_contract_and_restart_recovery(tmp_path):
    submissions, lookups = [], []
    state = {"pending":True}
    transaction = raw(PAYMENT["rawTransaction"])
    canonical = raw(PAYMENT["signedReceipt"])
    required = required_payment()
    def dispatch(method, path, headers, body):
        assert method == "POST" and path == "/rpc"
        request = json.loads(body)
        if request["method"] == "eth_sendRawTransaction":
            assert request["params"] == [PAYMENT["rawTransaction"]]
            submissions.append(request)
            result = PAYMENT["receiptJson"]["txHash"]
        elif request["method"] == "pax_sequencerReceipt":
            lookups.append(request)
            result = None if state["pending"] else PAYMENT["receiptJson"]
        elif request["method"] == "px_getReceiptKeys":
            result = {"keys":[PAYMENT["pinnedKey"]]}
        else:
            raise AssertionError(request)
        return 200, {"Content-Type":"application/json"}, {"jsonrpc":"2.0", "id":request["id"], "result":result}
    with http_server(dispatch) as endpoint:
        rpc = PaymentRpc(endpoint+"/rpc", hosted_gateway_endpoint=endpoint+"/rpc")
        keys = rpc.receipt_keys()
        authority = ConfiguredReceiptAuthority(transaction)
        path = tmp_path / "buyer.sqlite"
        buyer = BuyerMiddleware(rpc, keys, authority, [("exact","layerx:125")], journal_path=path)
        args = encode_header(required), PAYMENT["rawTransaction"], PAYMENT["receiptJson"]["txHash"], "0x"+payment_terms()["payer"]
        assert buyer.prepare(*args) is None
        buyer._submissions.close()
        state["pending"] = False
        recovered = BuyerMiddleware(rpc, keys, authority, [("exact","layerx:125")], journal_path=path)
        header = recovered.prepare(*args)
        assert decode_header(header)["payload"] == decode_header(receipt_header(required))["payload"]
        assert len(submissions) == 1 and len(lookups) == 2
        seller = SellerMiddleware(required, keys, authority, FulfillmentStore(tmp_path/"seller.sqlite"))
        released = []
        def release():
            released.append(True)
            return b"paid resource"
        first = seller.handle("buyer", header, release)
        reopened = SellerMiddleware(required, keys, authority, FulfillmentStore(tmp_path/"seller.sqlite"))
        assert reopened.handle("buyer", header, release) == first
        assert first[0] == 200 and released == [True]
        assert recovered.capture_settlement(first[1]["PAYMENT-RESPONSE"], header).canonical_bytes == canonical


def test_seller_commitment_downgrade_and_verified_landing(tmp_path, commit_producer):
    required = required_payment("committed")
    canonical, transaction = raw(PAYMENT["signedReceipt"]), raw(PAYMENT["rawTransaction"])
    path = tmp_path/"seller-committed.sqlite"
    seller = SellerMiddleware(required, [pin()], ConfiguredReceiptAuthority(transaction), FulfillmentStore(path))
    released = []
    def release():
        released.append(True)
        return b"committed resource"
    with pytest.raises(PlatformSdkError):
        seller.handle("buyer", receipt_header(required), release)
    assert released == []
    produced = commit_producer(transaction, 1200)
    evidence = PaymentEvidence(canonical, transaction, finality_bundle(produced))
    def authority(receipt, offer):
        assert receipt == canonical and offer == required["accepts"][0]
        return evidence
    seller = SellerMiddleware(required, [pin()], authority, FulfillmentStore(path))
    assert seller.handle("buyer", receipt_header(required), release)[0] == 200
    assert released == [True]
    for field in ("data_hash", "app_hash"):
        bad = copy.deepcopy(evidence.finality[1])
        bad["header"][field] = "11"*32
        refused = replace(evidence, finality=(evidence.finality[0],bad))
        def bad_authority(receipt, offer):
            return refused
        with pytest.raises(Exception):
            SellerMiddleware(required, [pin()], bad_authority, FulfillmentStore(tmp_path/(field+".sqlite"))).handle("buyer", receipt_header(required), release)
    assert released == [True]


def test_empty_receipt_keys_and_rotated_node_responses():
    rotated = rotated_receipt_key()
    rotated_json = {"keyId":"0x"+rotated.key_id.hex(),"publicKey":"0x"+rotated.public_key.hex(),"firstHeight":rotated.first_height,"lastHeight":rotated.last_height}
    responses = [{"keys":[]}, {"keys":[PAYMENT["pinnedKey"]]}, {"keys":[PAYMENT["pinnedKey"],rotated_json]}, {"keys":[{**PAYMENT["pinnedKey"], "firstHeight":1001}, PAYMENT["pinnedKey"]]}]
    def dispatch(method, path, headers, body):
        request = json.loads(body)
        assert request["method"] == "px_getReceiptKeys" and request["params"] == []
        return 200, {"Content-Type":"application/json"}, {"jsonrpc":"2.0","id":request["id"],"result":responses.pop(0)}
    with http_server(dispatch) as endpoint:
        rpc = PaymentRpc(endpoint+"/rpc", hosted_gateway_endpoint=endpoint+"/rpc")
        assert rpc.receipt_keys() == ()
        with pytest.raises(SequencerReceiptVerificationError) as raised:
            verify_payment_receipt(raw(PAYMENT["signedReceipt"]), raw(PAYMENT["rawTransaction"]), (), **payment_terms())
        assert raised.value.check == "unpinned_key"
        assert rpc.receipt_keys() == (pin(),)
        assert rpc.receipt_keys() == (pin(),rotated)
        # Duplicate key authority is malformed, even when the height window changes.
        with pytest.raises(ValueError):
            rpc.receipt_keys()
    with pytest.raises(ValueError):
        decode_receipt_keys({"keys":None})


def test_web_search_payment_binding_uses_ledger_event():
    terms = payment_terms()
    offer = WebSearchOffer("exact", "eip155:125", terms["amount"], terms["asset"], terms["pay_to"], 30, "USDC", "agent:did:layerx:seller:main", terms["payer"])
    verified = verify_web_payment(raw(PAYMENT["signedReceipt"]), raw(PAYMENT["rawTransaction"]), offer, (pin(),))
    assert verified.receipt.activity_id == raw(PAYMENT["receiptJson"]["txHash"])
    transfer = next(value for value in VECTORS["cases"] if value["id"] == "V1-pax-transfer")
    misleading = replace(offer, amount=str(decode_signed(raw(transfer["rawTransaction"]))[0].value),
                        pay_to=transfer["receiptJson"]["to"][2:].zfill(64), payer=transfer["receiptJson"]["from"][2:].zfill(64))
    with pytest.raises(WebSearchError):
        verify_web_payment(raw(transfer["signedReceipt"]), raw(transfer["rawTransaction"]), misleading, (pin(transfer),))
    for changes in (dict(amount=str(int(terms["amount"])+1)),dict(pay_to="22"*32),dict(asset="22"*32),dict(payer="22"*32)):
        with pytest.raises(WebSearchError):
            verify_web_payment(raw(PAYMENT["signedReceipt"]), raw(PAYMENT["rawTransaction"]), replace(offer,**changes), (pin(),))


def test_mirror_request_and_response_shapes_through_real_process(tmp_path):
    """Drive the process wire codec over loopback; no external verifier is needed."""
    document = json.loads((ROOT / "services/interop/crates/layerx-mirror/tests/fixtures/committed_blocks.json").read_text())
    block_hash = raw(document["committedBlock"]["hash"])
    commitment = sha256(b"pinned archive locator").digest()
    policy = MirrorPolicy("exact", (MirrorCandidate(0, commitment),))
    transcript = {
        "level": "Committed", "committedHeight": "1000", "committedBlockHash": block_hash.hex(),
        "sourceId": "local-archive", "target": "ethereum:chain=1;genesis=" + "55" * 32,
        "canonicalPosition": "ethereum:block=100;hash=" + "66" * 32,
        "provenance": "Canonical", "latestBatch": "1000", "batchLag": "Known(0)",
        "failoverCount": 0, "agreeingSources": 1,
    }
    calls = []
    def dispatch(method, path, headers, body):
        assert method == "POST" and path == "/verify"
        request = json.loads(body)
        assert set(request) == {"batch_number", "policy", "evidence"}
        assert request["batch_number"] == "1000"
        assert request["policy"] == {"kind": "exact", "candidate": {"source": 0, "commitment_hex": commitment.hex()}}
        evidence = request["evidence"]
        calls.append(evidence)
        if evidence == {"kind": "receipt", "index": 0}:
            digest = document["receipts"][0]["receiptDigest"][2:]
        elif evidence == {"kind": "committed"}:
            digest = None
        else:
            assert set(evidence) == {"kind", "index"} and evidence["index"] == 100
            return 200, {"Content-Type": "application/json"}, {"ok": False, "error": "verification"}
        return 200, {"Content-Type": "application/json"}, {"ok": True, "verification": {**transcript, "evidenceDigest": digest}}
    with http_server(dispatch) as endpoint:
        def exchange(request):
            with urlopen(Request(endpoint + "/verify", data=request, headers={"Content-Type": "application/json"}), timeout=5) as response:
                return 0, response.read()
        verifier = MirrorVerifier(tmp_path / "verifier", tmp_path / "mirror.json", exchange=exchange)
        receipt = verifier.receipt(1000, policy, 0)
        header = verifier.committed(1000, policy)
        assert receipt.level == header.level == "Committed"
        assert receipt.committed_height == header.committed_height == 1000
        assert receipt.committed_block_hash == header.committed_block_hash == block_hash
        assert receipt.evidence_digest is not None and len(receipt.evidence_digest) == 32
        assert receipt.evidence_digest == raw(document["receipts"][0]["receiptDigest"])
        assert header.evidence_digest is None and not hasattr(header, "checkpoint_level")
        assert not hasattr(verifier, "state") and calls
        with pytest.raises(MirrorVerificationError):
            verifier.receipt(1000, policy, -1)
        with pytest.raises(MirrorVerificationError):
            verifier.receipt(1000, policy, 100)
    assert calls == [{"kind": "receipt", "index": 0}, {"kind": "committed"}, {"kind": "receipt", "index": 100}]


def test_program_interface_transport_from_registry_serializer():
    interface = b"documented interface encoding"
    emitted = {"program_id": "11" * 32, "version": 1, "code_hash": "22" * 32, "abi_version": 2, "interface": interface.hex(),
               "interface_digest": sha256(interface).hexdigest(), "deployment_receipt_digest": "33" * 32, "state_root": "44" * 32,
               "observed_sequence": 7, "observed_at": 100, "valid_through": 200, "source": {"status": "unpublished"},
               "verification": "deployment-interface-and-current-head-verified"}
    # Keep this committed route transcript aligned with the real serializer's keys.
    source = (ROOT / "services/platform/hosted/registry/src/routes.rs").read_text()
    serializer = source.split("fn interface_response(", 1)[1].split("#[cfg(test)]", 1)[0]
    for field in emitted:
        assert f'"{field}":' in serializer
    assert "interface_response(" in source.split("fn read_interface(", 1)[1].split("fn verify(", 1)[0]
    seen = []
    def dispatch(method, path, headers, body):
        assert method == "GET" and path == f'/v1/programs/registry/{emitted["program_id"]}/interface'
        seen.append(json.loads(body))
        return 200, {"Content-Type": "application/json"}, emitted
    with http_server(dispatch) as endpoint:
        client = ProgramOperations(ProductionClient(AgentHttpTransport(endpoint)), None, ProgramTrustContext(b"\x55" * 32, clock_milliseconds=lambda: 150))
        response = client.interface(emitted["program_id"])
        assert response.interface == interface and response.receipt_digest == emitted["deployment_receipt_digest"]
        assert response.observed_sequence == 7 and response.observed_at == 100 and response.valid_through == 200
        assert response.verification == "server-side-receipt-verification-only" and len(seen) == 1
    from layerx_sdk.programs import _interface
    for field in ("observed_sequence", "observed_at", "valid_through"):
        with pytest.raises(ValueError):
            _interface({**emitted, field: str(emitted[field])}, emitted["program_id"], 150)
    altered = {**emitted, "receipt_digest": emitted["deployment_receipt_digest"]}
    del altered["deployment_receipt_digest"]
    with pytest.raises(ValueError):
        _interface(altered, emitted["program_id"], 150)


def rotated_receipt_key():
    public = Ed25519PrivateKey.from_private_bytes(sha256(b"x-websearch-test/key/sequencer").digest()).public_key().public_bytes_raw()
    return ReceiptKey(sha256(b"x-websearch-test/receipt-key/sequencer").digest(), public, 1, (1<<64)-1)


def test_grant_receipt_kind_from_recorded_server_producer(tmp_path):
    # This is the registered producer refusal, not the withdrawn successful draw transcript.
    reason = encode_call("Error", ("string",), ("LXP_NO_PAYER_GRANT: no payer grant",))
    refusal = decode_revert_reason(reason)
    assert refusal.code == "LXP_NO_PAYER_GRANT" and not refusal.retryable
    terms = payment_terms()
    binding = LedgerDrawTerms("11" * 32, "22" * 32, "33" * 32, "44" * 32)
    transaction = raw(PAYMENT["rawTransaction"])
    receipt = raw(PAYMENT["signedReceipt"])
    for scheme, grant in (("metered", None), ("subscription", None), ("exact", binding),
                          ("exact", {}), ("metered", binding), ("subscription", binding)):
        with pytest.raises(RevertReasonError) as failure:
            verify_payment_receipt(receipt, transaction, [pin()], scheme=scheme, grant_binding=grant, **terms)
        assert failure.value.code == refusal.code
        assert failure.value.message == refusal.message and failure.value.retryable is False
        # A pending result must not bypass the same refusal at the RPC consumer.
        for result in (PAYMENT["receiptJson"], {"txHash": PAYMENT["receiptJson"]["txHash"], "nonce": "0x0", "rung": "submitted"}):
            with pytest.raises(RevertReasonError, match="LXP_NO_PAYER_GRANT"):
                verify_rpc_payment(result, PAYMENT["receiptJson"]["txHash"], "0x" + terms["payer"],
                    receipt_keys=[pin()], raw_transaction=transaction, amount=terms["amount"],
                    asset=terms["asset"], pay_to=terms["pay_to"], scheme=scheme, grant_binding=grant)
    assert verify_payment_receipt(receipt, transaction, [pin()], **terms).receipt.activity_id == raw(PAYMENT["receiptJson"]["txHash"])
    for scheme in ("metered", "subscription"):
        offer = WebSearchOffer(scheme, "eip155:125", terms["amount"], terms["asset"], terms["pay_to"],
                               60, "SID", "agent:did:layerx:seller:main", terms["payer"], binding.purpose_hash)
        with pytest.raises(WebSearchError) as failure:
            verify_web_payment(receipt, transaction, offer, (pin(),), draw_terms=binding)
        assert isinstance(failure.value.__cause__, RevertReasonError)
        assert failure.value.__cause__.code == refusal.code
    wire, body, offer = _grant_inputs(PAYMENT)
    budgets = PaymentBudget(tmp_path / "budget.sqlite", "buyer", terms["asset"], int(terms["amount"]) * 2)
    middleware = AgentGrantMiddleware(budgets, None, [pin()])
    with pytest.raises(RevertReasonError, match="LXP_NO_PAYER_GRANT"):
        middleware("buyer", "55" * 32, body, offer)
    evidence = PaymentEvidence(receipt, transaction, grant_binding=decode_receive(wire))
    with pytest.raises(RevertReasonError, match="LXP_NO_PAYER_GRANT"):
        budgets.verify_and_commit(body["idempotencyKey"], "55" * 32, offer, evidence, [pin()])
    with sqlite3.connect(budgets.path) as db:
        assert db.execute("SELECT COUNT(*) FROM payment_reservations").fetchone() == (0,)
    # Even with the exact scheme, a signed draw cannot use a payExact receipt.
    calldata = encode_call("draw", LEDGER_DRAW_TYPES,
        (raw(binding.grant_id), raw(terms["pay_to"]), int(terms["amount"]), raw(binding.idempotency_key),
         0, raw(binding.purpose_hash), raw(binding.context_hash)))
    draw_transaction, _ = sign_transaction(calldata, target="101a")
    with pytest.raises(PlatformSdkError):
        verify_payment_receipt(receipt, draw_transaction, [pin()], scheme="exact", **terms)


def test_draw_abi_decoder_and_binding_fields():
    wire, body, offer = _grant_inputs(PAYMENT)
    receive = decode_receive(wire)
    grant = receive["payer_grant"]
    expected = (raw(receive["grant_id"]), raw(receive["to"]), int(receive["amount"]), raw(receive["idempotency_key"]),
                int(receive["receiver_sequence"]), raw(grant["purpose_hash"]), raw(receive["context_hash"]))
    calldata = encode_call("draw", LEDGER_DRAW_TYPES, expected)
    assert decode_call("draw", LEDGER_DRAW_TYPES, calldata) == expected
    transaction, actor = sign_transaction(calldata, target="101a")
    assert bind_receive_activity(transaction, wire, actor, 125, body["idempotencyKey"]) == "0x" + keccak256(transaction).hex()
    binding = LedgerDrawTerms(receive["grant_id"], grant["purpose_hash"], receive["idempotency_key"], receive["context_hash"])
    for field in ("grant_id", "purpose_hash", "idempotency_key", "context_hash"):
        with pytest.raises(RevertReasonError, match="LXP_NO_PAYER_GRANT") as failure:
            verify_payment_receipt(raw(PAYMENT["signedReceipt"]), transaction, [pin()], scheme="metered",
                grant_binding=replace(binding, **{field: "99" * 32}), **payment_terms())
        assert failure.value.code == "LXP_NO_PAYER_GRANT" and not failure.value.retryable
        index = {"grant_id": 0, "purpose_hash": 5, "idempotency_key": 3, "context_hash": 6}[field]
        changed = list(expected)
        changed[index] = raw("99" * 32)
        mismatched, _ = sign_transaction(encode_call("draw", LEDGER_DRAW_TYPES, changed), target="101a")
        with pytest.raises(ValueError, match="grant draw transaction binding"):
            bind_receive_activity(mismatched, wire, actor, 125, body["idempotencyKey"])
    event_types = ("bytes32", "uint128", "bytes32", "bytes32")
    event_values = (raw(receive["asset"]), int(receive["amount"]), expected[3], expected[6])
    event_data = encode_call("drawEventData", event_types, event_values)[4:]
    assert decode_abi(event_types, event_data) == event_values
    with pytest.raises(EvmTransactionError):
        decode_abi(event_types, event_data + bytes(32))
    for malformed in (calldata + bytes(32), calldata[:-1], bytes(4) + calldata[4:]):
        with pytest.raises(EvmTransactionError):
            decode_call("draw", LEDGER_DRAW_TYPES, malformed)
    for index, width in ((2, 128), (4, 64)):
        malformed = bytearray(calldata)
        start = 4 + index * 32
        malformed[start:start + 32] = (1 << width).to_bytes(32, "big")
        with pytest.raises(EvmTransactionError):
            decode_call("draw", LEDGER_DRAW_TYPES, bytes(malformed))
    gateway = json.loads((WEB_FIXTURES / "gateway/metered.json").read_text())
    for vector in gateway["ledger"]["drawVectors"]:
        decoded = decode_call("draw", LEDGER_DRAW_TYPES, raw(vector["calldata"]))
        assert decoded == (raw(vector["grantId"]), raw(vector["recipient"]), int(vector["amount"]),
                           raw(vector["idempotencyKey"]), int(vector["receiverSequence"]),
                           raw(vector["purpose"]), raw(vector["trace"]))


def test_draw_send_contract_and_restart_recovery(tmp_path):
    wire, body, offer = _grant_inputs(PAYMENT)
    receive = decode_receive(wire)
    grant = receive["payer_grant"]
    data = encode_call("draw", LEDGER_DRAW_TYPES,
        (raw(receive["grant_id"]), raw(receive["to"]), int(receive["amount"]), raw(receive["idempotency_key"]),
         int(receive["receiver_sequence"]), raw(grant["purpose_hash"]), raw(receive["context_hash"])))
    canonical, actor = sign_transaction(data, target="101a")
    activity = "0x" + keccak256(canonical).hex()
    digest = sha256(b"authenticated grant request").hexdigest()
    submissions, lookups = [], []
    def dispatch(method, path, headers, body):
        request = json.loads(body)
        assert method == "POST" and path == "/rpc"
        assert request["method"] == "eth_sendRawTransaction"
        assert request["params"] == ["0x" + canonical.hex()]
        decoded, signature = decode_signed(raw(request["params"][0]))
        assert decoded.data == data and decoded.recover_signer(signature).lower() == actor.lower()
        submissions.append(request)
        reason = encode_call("Error", ("string",), ("LXP_NO_PAYER_GRANT: no payer grant",))
        return 200, {"Content-Type": "application/json"}, {"jsonrpc": "2.0", "id": request["id"],
            "error": {"code": 3, "message": "execution reverted", "data": "0x" + reason.hex()}}
    with http_server(dispatch) as endpoint:
        rpc = PaymentRpc(endpoint + "/rpc", hosted_gateway_endpoint=endpoint + "/rpc")
        # The real RPC send contract preserves the producer's typed refusal.
        with pytest.raises(RevertReasonError, match="LXP_NO_PAYER_GRANT"):
            rpc.send("0x" + canonical.hex())
        assert len(submissions) == 1
        path = tmp_path / "draw.sqlite"
        draws = PreparedGrantDraws(path, actor, 125, rpc, ConfiguredReceiptAuthority(canonical), [pin()])
        for reopen in (False, True):
            if reopen:
                del draws
                draws = PreparedGrantDraws(path, actor, 125, rpc, ConfiguredReceiptAuthority(canonical), [pin()])
            with pytest.raises(RevertReasonError, match="LXP_NO_PAYER_GRANT"):
                draws.register("buyer", digest, canonical, wire, body["idempotencyKey"])
            with pytest.raises(RevertReasonError, match="LXP_NO_PAYER_GRANT"):
                draws("buyer", digest, body, offer)
            with pytest.raises(RevertReasonError, match="LXP_NO_PAYER_GRANT"):
                draws._resolve(True, {"canonical": canonical, "activity_id": activity}, offer)
            with sqlite3.connect(path) as db:
                assert db.execute("SELECT COUNT(*) FROM grant_draws").fetchone() == (0,)
        assert len(submissions) == 1 and lookups == []


@pytest.mark.parametrize("currency",("SID","PAX","USDC","USDL"))
@pytest.mark.parametrize("registered_did", (False, True))
def test_web_search_payer_header_and_ledger_accounts(currency, registered_did):
    exchange = json.loads((WEB_FIXTURES/"client-exchange.json").read_text())["exchange"]
    gateway = json.loads((WEB_FIXTURES/"gateway/metered.json").read_text())
    challenge = exchange[0]["response"]
    required = challenge["headers"]["PAYMENT-REQUIRED"]
    accepted = next(offer for offer in required["accepts"] if offer["scheme"] == "metered" and offer["extra"]["layerx"]["currency"] == currency)
    address = exchange[0]["request"]["headers"]["LAYERX-PAYER-ADDRESS"]
    owner = "00"*12+address[2:]
    did_key = "ab" * 32 if registered_did else owner
    did = "did:layerx:"+did_key
    accounts = []
    requests = []
    grant = next(grant for grant in gateway["ledger"]["grants"] if grant["name"] == currency)
    class Wallet:
        preferences = (WebSearchPreference(currency,"metered"),)
        def __init__(self):
            self.did = did
        def metered(self,offer,target):
            assert offer.pay_to == gateway["ledger"]["receiverAccounts"][currency]
            assert offer.payer == accepted["extra"]["layerx"]["payer"]
            assert target == "/search?q=paxeer"
            return WebSearchMeteredPayment(raw(grant["canonical"]),"27"*32)
    def rpc_dispatch(method,path,headers,body):
        request = json.loads(body)
        assert request["method"] == "eth_call" and request["params"][1] == "pending"
        call = request["params"][0]
        if call["data"] == layerx_did_call(address):
            encoded_did = did.encode()
            result = "0x"+(bytes.fromhex(did_key)+(64).to_bytes(32,"big")+len(encoded_did).to_bytes(32,"big")+encoded_did+bytes(-len(encoded_did)%32)).hex()
        else:
            assert call["to"] == "0x000000000000000000000000000000000000101a"
            data = raw(call["data"])
            assert data[:4] == keccak256(b"accountId(bytes32,bytes32)")[:4]
            assert data[36:].hex() == accepted["asset"]
            accounts.append(data[4:36].hex())
            if data[4:36].hex() == gateway["ledger"]["receiverDidKey"]:
                result = "0x"+accepted["payTo"]
            else:
                assert data[4:36].hex() == owner
                result = "0x"+accepted["extra"]["layerx"]["payer"]
        return 200,{"Content-Type":"application/json"},{"jsonrpc":"2.0","id":request["id"],"result":result}
    def dispatch(method,path,headers,body):
        assert headers["LAYERX-PAYER-ADDRESS"] == address and "LAYERX-PAYER-DID" not in headers
        assert path == "/search?q=paxeer"
        requests.append(headers.get("PAYMENT-SIGNATURE"))
        if "PAYMENT-SIGNATURE" not in headers:
            return 402,{"Content-Type":"application/json","PAYMENT-REQUIRED":encode_header(required)},challenge["body"]
        payload = decode_header(headers["PAYMENT-SIGNATURE"])
        assert payload["accepted"] == accepted
        assert payload["payload"] == {"grantId":grant["grantId"],"idempotencyKey":"27"*32}
        # Preserve the producer's configured-payer mismatch refusal.
        return 400,{"Content-Type":"application/json"},{"error":"payer_mismatch"}
    with http_server(rpc_dispatch) as rpc_endpoint,http_server(dispatch) as endpoint:
        assets = {offer["extra"]["layerx"]["currency"]:WebSearchAssetTerms(offer["asset"],int(offer["amount"])) for offer in required["accepts"]}
        client = WebSearchClient(endpoint,"eip155:125",assets["PAX"].asset_id,assets,Wallet(),(),now=lambda:1,
                                 payer_rpc=PaymentRpc(rpc_endpoint+"/rpc", hosted_gateway_endpoint=rpc_endpoint+"/rpc"),payer_address=address)
        with pytest.raises(WebSearchError) as raised:
            client.search("paxeer")
        assert raised.value.code == "payment-refused:payer_mismatch" and raised.value.status == 400
        assert len(requests) == 2 and requests[0] is None and requests[1] is not None
        assert accounts == ([] if currency == "PAX" else [gateway["ledger"]["receiverDidKey"],owner])
