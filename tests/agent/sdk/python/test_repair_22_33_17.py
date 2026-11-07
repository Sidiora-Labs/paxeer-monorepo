"""Audit 22.30.100: real HTTP, signed receipts, committed blocks and durable SQLite.

The shared peers below are committed, in-process producers from the prerequisite
repair suites. Importing them does not execute their tests or invoke toolchains.
"""
from __future__ import annotations

import ast
import copy
import json
import sqlite3
from dataclasses import replace
from hashlib import sha256
from pathlib import Path

import pytest
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey

import layerx_sdk as sdk
from layerx_sdk import programs, verifier, x402, x402_rpc
from layerx_sdk.account_derivation import keccak256
from layerx_sdk.agent_http import AgentHttpTransport
from layerx_sdk.evm_transaction import decode_call, decode_signed, encode_call
from layerx_sdk.production import ProductionClient
from layerx_sdk.program_lifecycle import NativeProgramLifecycleRequest, NativeProgramWindDown
from layerx_sdk.revert import RevertReasonError
from layerx_sdk.verifier import ReceiptKey, SequencerReceiptVerificationError
from layerx_sdk.x402_activity import bind_receive_activity
from layerx_sdk.x402_draw import PreparedGrantDraws
from layerx_sdk.x402_http import (
    BuyerMiddleware, ConfiguredReceiptAuthority, FulfillmentStore, PaymentEvidence,
    SellerMiddleware, decode_header, encode_header,
)
from layerx_sdk.x402_receive import decode_receive, encode_receive
from layerx_sdk.x402_rpc import PaymentRpc, rpc_committed_evidence, rpc_payment_evidence

from test_repair_22_31_147 import (
    commit_producer, finality_bundle, http_server, payment_terms,
)
from test_repair_22_33_16 import credential, sign_transaction, signatures

ROOT = Path(__file__).resolve().parents[4]
SDK = ROOT / "services/agent/sdk/python/layerx_sdk"
CORPUS = json.loads((ROOT / "sdk/platform/conformance/fixtures/sequencer-receipt-v1.json").read_text())
PAYMENT = next(case for case in CORPUS["cases"] if case["id"] == "V1-layerxledger-pay-exact")
PROGRAM = next(case for case in CORPUS["cases"] if case["id"] == "V1-layerxprograms-call")


def raw(value):
    return bytes.fromhex(value.removeprefix("0x"))


def pin(vector=PAYMENT):
    value = vector["pinnedKey"]
    return ReceiptKey(raw(value["keyId"]), raw(value["publicKey"]), value["firstHeight"], value["lastHeight"])


class ReceiptNode:
    """Loopback node peer serving actual recorded receipts and fresh lifecycle execution.

    Fresh lifecycle receipts are emitted with the same canonical v2 framing as the
    Go sequencer transcript. Rotations use independent real Ed25519 signing keys.
    """
    def __init__(self):
        self.requests = []
        self.transactions = {}
        self.receipts = {}
        self.keys = []
        self.result_override = None
        self.path = "/"

    def add_recording(self, vector):
        transaction = raw(vector["rawTransaction"])
        identity = "0x" + keccak256(transaction).hex()
        self.transactions[identity] = transaction
        self.receipts[identity] = copy.deepcopy(vector["receiptJson"])
        self.keys = [vector["pinnedKey"]]

    def execute_lifecycle(self, transaction, height, secret, key_id):
        tx, signature = decode_signed(transaction)
        actor = raw(tx.recover_signer(signature))
        identity = keccak256(transaction)
        summary = (21000).to_bytes(8, "big") * 2 + (1).to_bytes(8, "big")
        summary += bytes(4 + 20 + 20) + sha256(b"").digest() + bytes(8) + b"\x04uhpx"

        def integer(value, size):
            return value.to_bytes(size, "big")

        def bounded(value):
            return integer(len(value), 4) + value

        zero = bytes(32)
        effect = b"\x00\x0c\x00\x00\x00\x00\x01\x00" + bounded(zero) + bounded(summary)
        canonical = (
            b"\x00\x02\x52\x01\x00\x02" + bounded(identity)
            + integer(height + 100, 8) + bounded(zero) + bounded(sha256(transaction).digest())
            + bounded(sha256(b"").digest()) + integer(0, 4) + integer(1, 4) + effect
            + integer(21000, 16) + bounded(integer(height, 32))
            + integer(12, 2) + integer(1, 4) + integer(0, 4) + b"\x02"
            + bounded(zero) + integer(0, 16) + bounded(bytes(12) + actor)
            + integer(0, 16) * 2 + integer(tx.nonce, 8)
            + bounded(bytes(12) + tx.to) + integer(0, 16) * 2 + bounded(zero) * 2
            + bounded(key_id) + integer(1700000000000, 8)
        )
        digest = sha256(b"LXP/v1/receipt\0" + canonical + b"\0").digest()
        signed = canonical + b"\x01" + bounded(secret.sign(digest))
        result = dict(
            rung="receipted", txHash="0x" + identity.hex(), receipt="0x" + signed.hex(),
            nonce=hex(tx.nonce), seq=hex(height + 100), height=hex(height), index="0x0",
            timestamp=hex(1700000000000), status="0x1", keyId="0x" + key_id.hex(),
            returnData="0x", logs=[], gasUsed=hex(21000), cumulativeGasUsed=hex(21000),
            effectiveGasPrice="0x1", contractAddress="0x" + bytes(20).hex(),
            sponsor="0x" + bytes(20).hex(), vmError="", feeDenom="uhpx", feeAmount=hex(21000),
            signingKey="0x" + secret.public_key().public_bytes_raw().hex(),
        )
        result["from"], result["to"] = "0x" + actor.hex(), "0x" + tx.to.hex()
        self.transactions[result["txHash"]] = transaction
        self.receipts[result["txHash"]] = result
        return result

    def dispatch(self, method, path, headers, body):
        assert method == "POST" and path == self.path
        assert headers["Content-Type"] == "application/json"
        request = json.loads(body)
        assert set(request) == {"jsonrpc", "id", "method", "params"}
        assert request["jsonrpc"] == "2.0" and type(request["id"]) is int
        self.requests.append(request)
        operation, params = request["method"], request["params"]
        if operation == "px_getReceiptKeys":
            assert params == []
            result = {"keys": self.keys}
        elif operation == "eth_sendRawTransaction":
            assert len(params) == 1 and params[0].startswith("0x")
            transaction = raw(params[0])
            tx, signature = decode_signed(transaction)
            assert tx.chain_id == 125 and tx.recover_signer(signature)
            result = "0x" + keccak256(transaction).hex()
            if result in self.transactions:
                assert self.transactions[result] == transaction
            self.transactions[result] = transaction
        else:
            assert operation == "pax_sequencerReceipt" and len(params) == 1
            assert params[0].startswith("0x") and len(raw(params[0])) == 32
            result = self.receipts.get(params[0])
            if self.result_override is not None:
                result = self.result_override
        return 200, {"Content-Type": "application/json"}, {
            "jsonrpc": "2.0", "id": request["id"], "result": result,
        }


def lifecycle_request(nonce):
    value = NativeProgramWindDown(bytes.fromhex("11" * 32), 3)
    transaction, _ = sign_transaction(encode_call("tombstone", ("bytes32",), (value.program_id,)), nonce=nonce)
    return NativeProgramLifecycleRequest.wind_down(value, transaction)


def test_program_receipt_key_rotation_and_reopened_lifecycle_recovery(tmp_path):
    node = ReceiptNode()
    discovery = Ed25519PrivateKey.from_private_bytes(b"\x31" * 32)
    secrets = [Ed25519PrivateKey.from_private_bytes(bytes([number]) * 32) for number in (41, 42)]
    keys = tuple(ReceiptKey(bytes([number]) * 32, secret.public_key().public_bytes_raw(), first, last)
                 for number, secret, first, last in zip((51, 52), secrets, (1, 1001), (1000, 2000)))
    node.keys = [dict(keyId="0x" + key.key_id.hex(), publicKey="0x" + key.public_key.hex(),
                      firstHeight=key.first_height, lastHeight=key.last_height) for key in keys]
    requests = [lifecycle_request(nonce) for nonce in (3, 4)]
    results = [node.execute_lifecycle(request.signed_activity, height, secret, key.key_id)
               for request, height, secret, key in zip(requests, (1000, 1001), secrets, keys)]
    with http_server(node.dispatch) as endpoint:
        pins = PaymentRpc(endpoint).receipt_keys()
        assert pins == keys and len({key.public_key for key in pins}) == 2
        trust = programs.ProgramTrustContext(discovery.public_key().public_bytes_raw(), protocol_version=3,
                                             network_id=125, receipt_keys=pins)
        for index, (request, result) in enumerate(zip(requests, results)):
            path = tmp_path / f"lifecycle-{index}.raw"
            path.write_bytes(request.signed_activity)
            client = programs.ProgramOperations(ProductionClient(AgentHttpTransport(endpoint, credential=credential())), signatures(), trust)
            verified = client.lifecycle_receipt(request)
            assert verified.height == int(result["height"], 16)
            assert verified.receipt.context_hash == keys[index].key_id
            assert verified.canonical_bytes == raw(result["receipt"])
            acknowledgement = programs.resolve_lifecycle_response(
                {"state": "executed", "activity_id": result["txHash"][2:],
                 "receipt": raw(result["receipt"]).hex(), "terminal_payload": "", "call_graph": ""},
                request.bind(), trust.sequencer_public_key, signatures(), receipt_keys=pins,
            )
            assert acknowledgement["state"] == "acknowledged"
            del client, request
            reopened = programs.ProgramOperations(ProductionClient(AgentHttpTransport(endpoint, credential=credential())), signatures(), trust)
            restored = NativeProgramLifecycleRequest.wind_down(NativeProgramWindDown(bytes.fromhex("11" * 32), 3), path.read_bytes())
            assert reopened.lifecycle_receipt(restored) == verified
            for changed, check in ((replace(keys[index], first_height=verified.height + 1, last_height=max(keys[index].last_height, verified.height + 1)), "key_out_of_range"),
                                   (replace(keys[index], last_height=verified.height - 1, first_height=min(keys[index].first_height, verified.height - 1)), "key_out_of_range"),
                                   (replace(keys[index], key_id=b"\x71" * 32), "unpinned_key"),
                                   (replace(keys[index], public_key=discovery.public_key().public_bytes_raw()), "bad_signature")):
                changed_trust = replace(trust, receipt_keys=(changed,))
                refusing = programs.ProgramOperations(ProductionClient(AgentHttpTransport(endpoint, credential=credential())), signatures(), changed_trust)
                with pytest.raises(SequencerReceiptVerificationError) as failure:
                    refusing.lifecycle_receipt(restored)
                assert failure.value.check == check
            node.result_override = {**result, "signingKey": "0x" + trust.sequencer_public_key.hex()}
            with pytest.raises(ValueError, match="summary binding"):
                reopened.lifecycle_receipt(restored)
            node.result_override = None
    assert all(request["method"] in ("px_getReceiptKeys", "pax_sequencerReceipt") for request in node.requests)
    with pytest.raises(ValueError, match="duplicate"):
        replace(trust, receipt_keys=(keys[0], replace(keys[0], public_key=keys[1].public_key)))


def test_discovery_authority_is_separate_from_rotated_receipt_authority():
    discovery = Ed25519PrivateKey.from_private_bytes(b"\x31" * 32)
    document = dict(program_id="11" * 32, lifecycle="active", version=1, code_hash="22" * 32,
                    abi_version=3, state_root="33" * 32, observed_sequence="7", observed_at="1000",
                    valid_through="2000", verification="registry-receipt-and-current-head-verified",
                    deployment_receipt_digest="44" * 32)
    digest = sha256(b"LayerX/program-discovery-proof/v1\0" + raw(document["program_id"]) + b"\x01"
                    + (1).to_bytes(4, "big") + raw(document["code_hash"]) + (3).to_bytes(2, "big")
                    + (7).to_bytes(8, "big") + (1000).to_bytes(8, "big") + (2000).to_bytes(8, "big")
                    + raw(document["state_root"])).digest()
    document.update(receipt_digest=digest.hex(), discovery_public_key=discovery.public_key().public_bytes_raw().hex(),
                    discovery_signature=discovery.sign(digest).hex())
    observed = []

    def dispatch(method, path, headers, body):
        assert method == "GET" and path.startswith("/v1/programs/registry/" + document["program_id"])
        assert headers["Authorization"].startswith("LayerX-Key ")
        observed.append(path)
        return 200, {"Content-Type": "application/json"}, {
            "request_id": "repair-22-33-17", "value": document,
            "verification_status": {"state": "achieved", "rung": "receipted"},
        }

    with http_server(dispatch) as endpoint:
        trust = programs.ProgramTrustContext(discovery.public_key().public_bytes_raw(), lambda: 1500,
                                             protocol_version=3, receipt_keys=(pin(),))
        client = programs.ProgramOperations(ProductionClient(AgentHttpTransport(endpoint, credential=credential())), signatures(), trust)
        assert client.discover(document["program_id"]).sequencer_signature_verified
        wrong = replace(trust, sequencer_public_key=pin().public_key)
        refused = programs.ProgramOperations(ProductionClient(AgentHttpTransport(endpoint, credential=credential())), signatures(), wrong)
        with pytest.raises(ValueError, match="discovery sequencer signature"):
            refused.discover(document["program_id"])
    assert len(observed) == 2


def test_retired_batch_authority_and_payment_evidence_exports_preserve_signed_receipts():
    assert sdk.PaymentEvidence is x402.PaymentEvidence is PaymentEvidence
    for module, names in ((sdk, ("AuthorizedReceiptBatch", "PaymentCommitmentEvidence", "rpc_batch_evidence")),
                          (verifier, ("AuthorizedReceiptBatch",)),
                          (x402, ("PaymentCommitmentEvidence", "_SequencerAuthorization")),
                          (x402_rpc, ("rpc_batch_evidence",)), (programs, ("_authority",))):
        for name in names:
            assert not hasattr(module, name)
    for filename in ("__init__.py", "__init__.pyi", "verifier.py", "verifier.pyi", "programs.py", "x402.py", "x402_rpc.py"):
        tree = ast.parse((SDK / filename).read_text())
        names = {node.name for node in ast.walk(tree) if isinstance(node, (ast.ClassDef, ast.FunctionDef))}
        names |= {node.id for node in ast.walk(tree) if isinstance(node, ast.Name)}
        names |= {node.name for node in ast.walk(tree) if isinstance(node, ast.alias)}
        assert names.isdisjoint({"AuthorizedReceiptBatch", "PaymentCommitmentEvidence", "_SequencerAuthorization", "rpc_batch_evidence"})
    node = ReceiptNode()
    node.add_recording(PROGRAM)
    transaction = raw(PROGRAM["rawTransaction"])
    with http_server(node.dispatch) as endpoint:
        rpc = PaymentRpc(endpoint)
        keys = rpc.receipt_keys()
        result = rpc.receipt(PROGRAM["receiptJson"]["txHash"])
        trust = programs.ProgramTrustContext(b"\x19" * 32, network_id=125, receipt_keys=keys)
        output = programs._submission(result, signatures(), trust, retained_signed_activity=transaction.hex())
        assert output == PROGRAM["receiptJson"]
        evidence = rpc_payment_evidence(result, transaction, keys)
        assert evidence == PaymentEvidence(raw(PROGRAM["signedReceipt"]), transaction)
        verified = sdk.verify_sequencer_receipt(evidence.canonical_receipt, transaction, keys)
        assert verified.receipt_digest == raw(PROGRAM["receiptDigest"])
        with pytest.raises(SequencerReceiptVerificationError):
            rpc_payment_evidence(result, transaction, ())


def test_root_node_and_explicit_hosted_gateway_endpoints():
    for host in [f"api{index}.mainnet-beta.paxeer.network" for index in range(1, 15)] + ["api-mainnet-beta.paxeer.network"]:
        for suffix in ("", "/"):
            endpoint = "https://" + host + suffix
            assert PaymentRpc(endpoint).endpoint == endpoint
    node = ReceiptNode()
    node.add_recording(PAYMENT)
    with http_server(node.dispatch) as endpoint:
        for root in (endpoint, endpoint + "/"):
            rpc = PaymentRpc(root, {"Authorization": "Bearer loopback-only"})
            assert rpc.receipt_keys() == (pin(),)
            assert rpc.receipt(PAYMENT["receiptJson"]["txHash"]) == PAYMENT["receiptJson"]
        gateway = endpoint + "/tenant/payments"
        node.path = "/tenant/payments"
        with pytest.raises(ValueError, match="invalid-rpc-endpoint"):
            PaymentRpc(gateway)
        hosted = PaymentRpc(gateway, hosted_gateway_endpoint=gateway)
        assert hosted.receipt_keys() == (pin(),)
        with pytest.raises(ValueError, match="invalid-rpc-endpoint"):
            PaymentRpc(gateway + "/other", hosted_gateway_endpoint=gateway)
    assert len(node.requests) == 5


@pytest.mark.parametrize("endpoint", [
    "http://api1.mainnet-beta.paxeer.network/", "https://user:secret@example.test/",
    "https://user@example.test/", "https://@example.test/", "https://example.test/?token=secret", "https://example.test/#fragment",
    "ftp://example.test/", "https://example.test/rpc", "https://example.test:99999/",
    "https://example.test/\n", " https://example.test/", "https://example.test\\@other.test/",
])
def test_rpc_endpoint_protections_apply_to_explicit_gateway_too(endpoint):
    with pytest.raises(ValueError):
        PaymentRpc(endpoint, hosted_gateway_endpoint=endpoint if "/rpc" not in endpoint else None)


def test_rpc_refuses_redirect_without_forwarding_credentials():
    destinations = []

    def target(method, path, headers, body):
        destinations.append(headers.get("Authorization"))
        return 200, {}, {}

    with http_server(target) as destination:
        def redirect(method, path, headers, body):
            assert headers["Authorization"] == "Bearer loopback-only"
            return 307, {"Location": destination + "/"}, b""
        with http_server(redirect) as endpoint:
            with pytest.raises(ValueError, match="rpc-redirect"):
                PaymentRpc(endpoint, {"Authorization": "Bearer loopback-only"}).receipt_keys()
    assert destinations == []


def required_payment(commitment):
    terms = payment_terms(PAYMENT)
    return dict(x402Version=2, resource={"url": "https://seller.example/resource"}, accepts=[dict(
        scheme="exact", network="layerx:125", amount=terms["amount"], asset=terms["asset"],
        payTo=terms["pay_to"], maxTimeoutSeconds=30, extra={"layerx": {"commitment": commitment}},
    )])


@pytest.mark.parametrize("commitment", ("receipted", "committed"))
def test_live_payment_evidence_flows_through_buyer_seller_and_reopen(tmp_path, commit_producer, commitment):
    node = ReceiptNode()
    node.add_recording(PAYMENT)
    transaction = raw(PAYMENT["rawTransaction"])
    identity = PAYMENT["receiptJson"]["txHash"]
    produced = commit_producer(transaction, int(PAYMENT["receiptJson"]["height"], 16) + 2)
    trust, committed = finality_bundle(produced)
    receipt_result = copy.deepcopy(node.receipts[identity])
    if commitment == "committed":
        receipt_result.update(rung="committed", blockHash="0x" + produced["header_hash"])
    node.receipts[identity] = receipt_result
    required = required_payment(commitment)
    released = []
    with http_server(node.dispatch) as endpoint:
        rpc = PaymentRpc(endpoint)
        keys = rpc.receipt_keys()
        authority = ConfiguredReceiptAuthority(transaction, rpc=rpc, receipt_keys=keys,
                                               commit_trust=trust, committed_evidence=committed)
        buyer_path = tmp_path / "buyer.sqlite"
        buyer = BuyerMiddleware(rpc, keys, authority, [("exact", "layerx:125")], journal_path=buyer_path)
        header = buyer.prepare(encode_header(required), "0x" + transaction.hex(), identity, "0x" + payment_terms(PAYMENT)["payer"])
        assert decode_header(header)["accepted"] == required["accepts"][0]
        seller_path = tmp_path / "seller.sqlite"
        seller = SellerMiddleware(required, keys, authority, FulfillmentStore(seller_path))

        def release():
            released.append(True)
            return b"paid resource"

        paid = seller.handle("buyer", header, release)
        assert paid[0] == 200 and paid[2] == b"paid resource"
        assert buyer.capture_settlement(paid[1]["PAYMENT-RESPONSE"], header).height == int(receipt_result["height"], 16)
        evidence = rpc_payment_evidence(rpc.receipt(identity), transaction, keys,
                                        commit_trust=trust, committed_evidence=committed)
        assert evidence.canonical_receipt == raw(PAYMENT["signedReceipt"])
        assert evidence.raw_transaction == transaction
        assert (evidence.finality is not None) == (commitment == "committed")
        buyer._submissions.close()
        del buyer, seller
        reopened_rpc = PaymentRpc(endpoint)
        reopened_authority = ConfiguredReceiptAuthority(transaction, rpc=reopened_rpc, receipt_keys=keys,
                                                        commit_trust=trust, committed_evidence=committed)
        reopened = BuyerMiddleware(reopened_rpc, keys, reopened_authority, [("exact", "layerx:125")], journal_path=buyer_path)
        assert reopened.prepare(encode_header(required), "0x" + transaction.hex(), identity,
                                "0x" + payment_terms(PAYMENT)["payer"]) == header
        recovered_seller = SellerMiddleware(required, keys, reopened_authority, FulfillmentStore(seller_path))
        assert recovered_seller.handle("buyer", header, release) == paid
        assert reopened.capture_settlement(paid[1]["PAYMENT-RESPONSE"], header).canonical_bytes == raw(PAYMENT["signedReceipt"])
        reopened._submissions.close()
    assert released == [True]
    assert sum(request["method"] == "eth_sendRawTransaction" for request in node.requests) == 1
    with sqlite3.connect(buyer_path) as db:
        assert db.execute("SELECT tx_hash, canonical FROM buyer_submissions").fetchall() == [(identity, transaction)]
    with sqlite3.connect(seller_path) as db:
        assert db.execute("SELECT COUNT(*) FROM payment_fulfillments").fetchone() == (1,)


def test_committed_evidence_rejects_untrusted_blocks_and_retired_batch_shapes(commit_producer):
    transaction = raw(PAYMENT["rawTransaction"])
    height = int(PAYMENT["receiptJson"]["height"], 16)
    produced = commit_producer(transaction, height + 2)
    trust, evidence = finality_bundle(produced)
    result = {**PAYMENT["receiptJson"], "rung": "committed", "blockHash": "0x" + produced["header_hash"]}
    assert rpc_committed_evidence(result, transaction, (pin(),), trust, evidence) == (trust, evidence)
    assert rpc_payment_evidence(None, transaction, (pin(),)) is None
    pending = {"txHash": result["txHash"], "nonce": result["nonce"], "rung": "submitted"}
    assert rpc_payment_evidence(pending, transaction, (pin(),)) is None
    for change in ({"keyId": "0x" + "91" * 32}, {"height": hex(height + 1)},
                   {"blockHash": "0x" + "92" * 32}, {"txHash": "0x" + "93" * 32}):
        with pytest.raises(ValueError):
            rpc_committed_evidence({**result, **change}, transaction, (pin(),), trust, evidence)
    for field in ("data_hash", "app_hash", "validators_hash"):
        altered = copy.deepcopy(evidence)
        altered["header"][field] = "11" * 32
        with pytest.raises(ValueError):
            rpc_committed_evidence(result, transaction, (pin(),), trust, altered)
    altered = copy.deepcopy(evidence)
    altered["proof"]["data"] = raw(PROGRAM["rawTransaction"]).hex()
    with pytest.raises(ValueError, match="identity mismatch"):
        rpc_committed_evidence(result, transaction, (pin(),), trust, altered)
    altered = copy.deepcopy(evidence)
    altered["commit"]["signatures"] = altered["commit"]["signatures"][:2]
    with pytest.raises(ValueError, match="voting power"):
        rpc_committed_evidence(result, transaction, (pin(),), trust, altered)
    for missing_trust, missing_evidence in ((None, evidence), (trust, None), (trust, {"batch_evidence": {}})):
        with pytest.raises(ValueError):
            rpc_payment_evidence(result, transaction, (pin(),), commit_trust=missing_trust,
                                  committed_evidence=missing_evidence)
    with pytest.raises(ValueError):
        rpc_payment_evidence({"batch_evidence": {"signed_header": {}, "proof": {}}}, transaction, (pin(),))
    with pytest.raises(SequencerReceiptVerificationError) as failure:
        rpc_payment_evidence(result, transaction, (replace(pin(), last_height=height - 1),),
                              commit_trust=trust, committed_evidence=evidence)
    assert failure.value.check == "key_out_of_range"


def draw_inputs():
    # A codec input and signed transaction, never a successful retired draw receipt.
    from test_x402_draw_sequencer import _grant_inputs
    receive, body, offer = _grant_inputs(PAYMENT)
    decoded = decode_receive(receive)
    grant = decoded["payer_grant"]
    values = (raw(decoded["grant_id"]), raw(decoded["to"]), int(decoded["amount"]), raw(body["idempotencyKey"]),
              int(decoded["receiver_sequence"]), raw(grant["purpose_hash"]), raw(decoded["context_hash"]))
    calldata = encode_call("draw", x402.LEDGER_DRAW_TYPES, values)
    transaction, actor = sign_transaction(calldata, target="101a")
    return receive, body, offer, values, transaction, actor


def test_receive_binding_uses_signed_evm_hash_and_canonical_ledger_calldata():
    receive, body, offer, values, transaction, actor = draw_inputs()
    identity = bind_receive_activity(transaction, receive, actor, 125, body["idempotencyKey"])
    assert identity == "0x" + keccak256(transaction).hex()
    assert decode_call("draw", x402.LEDGER_DRAW_TYPES, decode_signed(transaction)[0].data) == values
    for index in range(7):
        changed = list(values)
        changed[index] = values[index] + 1 if isinstance(values[index], int) else b"\x99" * 32
        mismatch, _ = sign_transaction(encode_call("draw", x402.LEDGER_DRAW_TYPES, changed), target="101a")
        with pytest.raises(ValueError, match="binding"):
            bind_receive_activity(mismatch, receive, actor, 125, body["idempotencyKey"])
    for changes in ({"chain": 126, "target": "101a"}, {"target": "101d"}):
        mismatch, _ = sign_transaction(encode_call("draw", x402.LEDGER_DRAW_TYPES, values), **changes)
        with pytest.raises(ValueError, match="binding"):
            bind_receive_activity(mismatch, receive, actor, 125, body["idempotencyKey"])
    for wrong_actor, wrong_network, wrong_key in (("0x" + "99" * 20, 125, body["idempotencyKey"]),
                                                 (actor, True, body["idempotencyKey"]),
                                                 (actor, 125, "99" * 32), (actor, 125, " " * 64)):
        with pytest.raises(ValueError, match="binding"):
            bind_receive_activity(transaction, receive, wrong_actor, wrong_network, wrong_key)
    for field in ("asset", "grant_id", "from", "to"):
        mismatch = copy.deepcopy(decode_receive(receive))
        mismatch[field] = "99" * 32
        with pytest.raises(ValueError, match="binding"):
            bind_receive_activity(transaction, encode_receive(mismatch), actor, 125, body["idempotencyKey"])
    with pytest.raises(ValueError):
        bind_receive_activity(b"\x00\x01\x10\x01" + bytes(200), receive, actor, 125, body["idempotencyKey"])


def seed_legacy_journal(path, transaction, receive, key, previous, attempted):
    # The committed legacy schema is a storage peer, using real durable SQLite bytes.
    with sqlite3.connect(path) as db:
        db.execute("CREATE TABLE grant_draws (idempotency_key TEXT PRIMARY KEY, principal TEXT NOT NULL, request_digest TEXT NOT NULL, canonical BLOB NOT NULL, receive BLOB NOT NULL, activity_id TEXT NOT NULL UNIQUE, period_key TEXT UNIQUE, attempted INTEGER NOT NULL DEFAULT 0, attempted_at INTEGER)")
        db.execute("INSERT INTO grant_draws VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
                   (key, "buyer", "55" * 32, transaction, receive, previous, "period-7", attempted,
                    1700000000 if attempted else None))


@pytest.mark.parametrize("attempted", (0, 1), ids=("crash-before-send", "unknown-send-outcome"))
@pytest.mark.parametrize("response_rung", ("absent", "submitted"))
@pytest.mark.parametrize("legacy_identity", (False, True))
def test_draw_null_receipt_reopens_with_exact_bytes_hash_and_pending_status(tmp_path, attempted, response_rung, legacy_identity):
    receive, body, offer, values, transaction, actor = draw_inputs()
    identity = "0x" + keccak256(transaction).hex()
    historical = sha256(b"LXP/v1/activity-id\0" + transaction).hexdigest()
    path = tmp_path / "draws.sqlite"
    seed_legacy_journal(path, transaction, receive, body["idempotencyKey"], historical if legacy_identity else identity, attempted)
    node = ReceiptNode()
    if response_rung == "submitted":
        node.receipts[identity] = {"txHash": identity, "nonce": hex(decode_signed(transaction)[0].nonce), "rung": "submitted"}
    with http_server(node.dispatch) as endpoint:
        for reopen in range(3):
            rpc = PaymentRpc(endpoint)
            draws = PreparedGrantDraws(path, actor, 125, rpc, ConfiguredReceiptAuthority(transaction), (pin(),))
            assert draws("buyer", "55" * 32, body, offer) is None
            assert draws.recover_pending(body["idempotencyKey"]) is None
            with sqlite3.connect(path) as db:
                assert db.execute("SELECT * FROM grant_draws").fetchall() == [
                    (body["idempotencyKey"], "buyer", "55" * 32, transaction, receive, identity,
                     "period-7", attempted, 1700000000 if attempted else None),
                ]
                columns = {row[1] for row in db.execute("PRAGMA table_info(grant_draws)")}
                assert "transaction_hash" in columns and "activity_id" not in columns
            with pytest.raises(RevertReasonError, match="LXP_NO_PAYER_GRANT"):
                draws.register("buyer", "55" * 32, transaction, receive, body["idempotencyKey"])
            with pytest.raises(ValueError, match="request binding"):
                draws("other-principal", "55" * 32, body, offer)
            del draws, rpc
    assert len(node.requests) == 6
    assert all(request["method"] == "pax_sequencerReceipt" and request["params"] == [identity]
               for request in node.requests)
    assert node.transactions == {}


def test_draw_migration_refuses_unbound_rows_without_partial_rewrite(tmp_path):
    receive, body, offer, values, transaction, actor = draw_inputs()
    path = tmp_path / "invalid.sqlite"
    seed_legacy_journal(path, transaction, receive, body["idempotencyKey"], "77" * 32, 1)
    node = ReceiptNode()
    with http_server(node.dispatch) as endpoint:
        with pytest.raises(ValueError, match="stored identity"):
            PreparedGrantDraws(path, actor, 125, PaymentRpc(endpoint), ConfiguredReceiptAuthority(transaction), (pin(),))
    with sqlite3.connect(path) as db:
        assert "activity_id" in {row[1] for row in db.execute("PRAGMA table_info(grant_draws)")}
        assert db.execute("SELECT canonical, activity_id, attempted FROM grant_draws").fetchall() == [(transaction, "77" * 32, 1)]
    assert node.requests == []


def test_new_grant_registration_stays_retired_across_reopen(tmp_path):
    receive, body, offer, values, transaction, actor = draw_inputs()
    node = ReceiptNode()
    path = tmp_path / "new.sqlite"
    with http_server(node.dispatch) as endpoint:
        for reopen in range(2):
            draws = PreparedGrantDraws(path, actor, 125, PaymentRpc(endpoint), ConfiguredReceiptAuthority(transaction), (pin(),))
            for operation in (lambda: draws.register("buyer", "55" * 32, transaction, receive, body["idempotencyKey"]),
                              lambda: draws("buyer", "55" * 32, body, offer),
                              lambda: draws._resolve(False, (transaction, receive), offer)):
                with pytest.raises(RevertReasonError, match="LXP_NO_PAYER_GRANT") as refusal:
                    operation()
                assert refusal.value.retryable is False
            with sqlite3.connect(path) as db:
                assert db.execute("SELECT COUNT(*) FROM grant_draws").fetchone() == (0,)
            del draws
    assert node.requests == []
