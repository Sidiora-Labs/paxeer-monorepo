from __future__ import annotations

import importlib.util
import json
import sys
from copy import deepcopy
from types import SimpleNamespace
from dataclasses import replace
from hashlib import sha256
from pathlib import Path

import pytest
from cryptography.hazmat.primitives import hashes
from cryptography.hazmat.primitives.asymmetric import ec, utils
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
from cryptography.hazmat.primitives.serialization import Encoding, PublicFormat

from layerx_sdk import ai_market as market
from layerx_sdk.account_derivation import _N, _recover, keccak256
from layerx_sdk.native_program_call import NativeProgramCall, encode_native_program_call
from layerx_sdk.verifier import ReceiptKey, SequencerReceiptVerificationError

ROOT = Path(__file__).resolve().parents[5]
sys.path.insert(0, str(ROOT / "tests/agent/sdk/python"))
HEIGHT = 1200
SEQUENCE = 9001
COMMIT = market.CommitFinality(HEIGHT, b"\x21" * 32, b"\x22" * 32, b"\x23" * 32)
PUBLIC_KEY = Ed25519PrivateKey.from_private_bytes(b"\x33" * 32).public_key().public_bytes(
    Encoding.Raw, PublicFormat.Raw
)
RECEIPT_KEY = ReceiptKey(b"\x66" * 32, PUBLIC_KEY, HEIGHT, HEIGHT)


@pytest.fixture(scope="module")
def signatures():
    spec = importlib.util.spec_from_file_location(
        "market_commit_signatures",
        ROOT / "sdk/integrations/fastapi/layerx_fastapi/signatures.py",
    )
    assert spec is not None and spec.loader is not None
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module.LayerXSignatureVerifier()


@pytest.fixture(scope="module")
def receipt_producer():
    # Independent codec peer; node-produced output is exercised separately below.
    from test_repair_22_31_147 import receipt_producer as codec_peer
    return codec_peer.__wrapped__()


@pytest.fixture(scope="module")
def commit_producer():
    from test_repair_22_31_147 import commit_producer as validators
    return validators.__wrapped__()


@pytest.fixture
def committed_operation(signed_operation, commit_producer):
    from test_repair_22_31_147 import finality_bundle

    def at(height=HEIGHT):
        produced = commit_producer(signed_operation.signed_bytes, height)
        trust, evidence = finality_bundle(produced)
        return SimpleNamespace(trust=trust, evidence=evidence,
                               block_hash=bytes.fromhex(produced["header_hash"]), at=at)
    return at()


@pytest.fixture
def signed_operation(signatures):
    key = ec.derive_private_key(42, ec.SECP256K1())
    public = key.public_key().public_bytes(Encoding.X962, PublicFormat.CompressedPoint)
    envelope = market.PaxaiEnvelope(
        market.FUND, b"\x01" * 32, b"\x02" * 32, b"\x03" * 32,
        b"\x04" * 32, 0, 1, None, 1, 1300, b"\x05" * 32,
        (100).to_bytes(16, "big") + b"\x06" * 32 + (1).to_bytes(8, "big") + b"\x01",
    )
    terms = market.NativeTerms(
        125, b"did:layerx:" + b"a" * 64, public, 7, envelope.request,
        0, envelope.expiry, 100, b"", b"", 1024, (1000,) * 7,
        2_000_000, 3_000_000_000, 1_000_000_000,
    )
    payload = encode_native_program_call(NativeProgramCall(
        envelope.program, market.GUEST_ABI, market.ENTRYPOINT, envelope.encode(),
        terms.capabilities, terms.access_declaration, terms.response_capacity,
        terms.resources,
    ))
    unsigned = market._unsigned_activity(terms, payload)
    digest = keccak256(unsigned)
    r, s = utils.decode_dss_signature(key.sign(digest, ec.ECDSA(utils.Prehashed(hashes.SHA256()))))
    s = min(s, _N - s)
    recovered = key.public_key().public_bytes(Encoding.X962, PublicFormat.UncompressedPoint)[1:]
    parity = next(p for p in (0, 1) if _recover(digest, r, s, p) == recovered)
    signature = r.to_bytes(32, "big") + s.to_bytes(32, "big") + bytes([27 + parity])
    return market.OperationRecord.signed(
        market._attach_signature(unsigned, signature), sha256(public).digest(),
        envelope.request_digest(), signatures,
    )


def produced_receipt(producer, record, result_code=0, *, nonce=7, recipient=None, height=HEIGHT):
    from layerx_sdk.verifier import _decode_sequencer_receipt
    canonical = producer(record)
    body, unsigned = _decode_sequencer_receipt(canonical)
    # Codec mutations remain signed by the fixture authority, never by a pinned node key.
    def bounded(value):
        return len(value).to_bytes(4, "big") + value
    unsigned = unsigned.replace(bounded(body.batch_id), bounded(height.to_bytes(32, "big")), 1)
    offset = 6 + 36 + 8 + 36 * 3
    unsigned = unsigned[:offset] + result_code.to_bytes(4, "big") + unsigned[offset + 4:]
    summary = (body.module_id.to_bytes(2, "big") + body.module_version.to_bytes(4, "big")
               + body.parameter_version.to_bytes(4, "big") + bytes([body.operation])
               + bounded(body.asset) + body.amount.to_bytes(16, "big")
               + bounded(body.from_account) + bytes(32))
    start = unsigned.index(summary) + len(summary)
    unsigned = unsigned[:start] + nonce.to_bytes(8, "big") + unsigned[start + 8:]
    if recipient is not None:
        start += 8
        unsigned = unsigned[:start] + bounded(bytes(12) + recipient) + unsigned[start + 36:]
    key = Ed25519PrivateKey.from_private_bytes(b"\x33" * 32)
    return unsigned[:-1] + b"\x01" + bounded(key.sign(sha256(b"LXP/v1/receipt\0" + unsigned).digest()))


def pending(journal, record):
    journal.record_signed(record)
    record.state, record.attempt, record.rung = market.SdkState.PENDING, 1, market.Rung.SUBMITTED
    journal.persist(record)
    return journal.load(record.activity_id)


def test_committed_market_operation_survives_restart(tmp_path, signatures, receipt_producer, signed_operation, committed_operation):
    journal = market.OperationJournal(tmp_path, signatures, commit_trust=committed_operation.trust)
    record = pending(journal, signed_operation)
    receipt = produced_receipt(receipt_producer, record)
    assert journal.resolve(record, receipt, (RECEIPT_KEY,)) is market.SdkState.EXECUTED
    assert (record.global_sequence, record.receipt_height, record.rung) == (
        SEQUENCE, HEIGHT, market.Rung.RECEIPTED,
    )
    reopened = market.OperationJournal(tmp_path, signatures, commit_trust=committed_operation.trust)
    recovered = reopened.load(record.activity_id)
    assert recovered == record
    assert reopened.finalize(recovered, committed_operation.evidence) is market.SdkState.FINALIZED
    assert recovered.checkpoint == committed_operation.block_hash
    assert recovered.rung is market.Rung.COMMITTED
    assert recovered.domain is market.DomainStatus.PENDING
    assert recovered.signed_bytes == signed_operation.signed_bytes
    assert market.OperationJournal(tmp_path, signatures, commit_trust=committed_operation.trust).load(record.activity_id) == recovered
    with pytest.raises(market.AiMarketError, match="InvalidTransition"):
        reopened.finalize(recovered, committed_operation.evidence)


def test_reverted_market_operation_is_durable_failure(tmp_path, signatures, receipt_producer, signed_operation, committed_operation):
    journal = market.OperationJournal(tmp_path, signatures, commit_trust=committed_operation.trust)
    record = pending(journal, signed_operation)
    receipt = produced_receipt(receipt_producer, record, 1)
    assert journal.resolve(record, receipt, (RECEIPT_KEY,)) is market.SdkState.FAILED
    assert record.result_code == 1
    assert record.checkpoint is None
    assert record.rung is market.Rung.RECEIPTED
    recovered = market.OperationJournal(tmp_path, signatures, commit_trust=committed_operation.trust).load(record.activity_id)
    assert recovered == record
    with pytest.raises(market.AiMarketError, match="InvalidTransition"):
        journal.finalize(recovered, committed_operation.evidence)
    assert journal.load(record.activity_id) == record


def test_reverted_market_transaction_landing_survives_restart(
    tmp_path, signatures, receipt_producer, signed_operation, committed_operation
):
    journal = market.OperationJournal(tmp_path, signatures, commit_trust=committed_operation.trust)
    record = pending(journal, signed_operation)
    receipt = produced_receipt(receipt_producer, record, 1)
    landing = {
        "transactionHash": "0x" + record.activity_id.hex(),
        "status": "0x0",
        "blockNumber": hex(HEIGHT),
        "blockHash": "0x" + committed_operation.block_hash.hex(),
    }
    before = record.encode()
    with pytest.raises(market.AiMarketError, match="ReceiptMismatch"):
        journal.resolve(record, receipt, (RECEIPT_KEY,), {**landing, "status": "0x1"})
    assert record.encode() == before
    assert journal.load(record.activity_id) == record
    assert journal.resolve(record, receipt, (RECEIPT_KEY,), landing) is market.SdkState.FAILED
    reopened = market.OperationJournal(tmp_path, signatures, commit_trust=committed_operation.trust)
    recovered = reopened.load(record.activity_id)
    assert recovered == record
    assert recovered.result_code == 1
    assert recovered.rung is market.Rung.RECEIPTED
    assert recovered.receipt_height == HEIGHT
    assert recovered.receipt_block_hash == committed_operation.block_hash
    assert recovered.checkpoint is None
    with pytest.raises(market.AiMarketError, match="InvalidTransition"):
        reopened.finalize(recovered, committed_operation.evidence)
    assert reopened.load(record.activity_id) == recovered


def test_market_operation_waits_for_covering_commit(tmp_path, signatures, receipt_producer, signed_operation, committed_operation):
    journal = market.OperationJournal(tmp_path, signatures, commit_trust=committed_operation.trust)
    record = pending(journal, signed_operation)
    before = record.encode()
    assert journal.resolve(record, None, ()) is market.SdkState.PENDING
    assert record.encode() == before
    receipt = produced_receipt(receipt_producer, record)
    assert journal.resolve(record, receipt, (RECEIPT_KEY,)) is market.SdkState.EXECUTED
    before = record.encode()
    with pytest.raises(market.AiMarketError, match="FinalityUnavailable"):
        journal.finalize(record, {**committed_operation.evidence, "commit": {**committed_operation.evidence["commit"], "height": HEIGHT - 1}})
    assert record.encode() == before
    assert journal.load(record.activity_id) == record
    assert record.checkpoint is None


def test_unknown_market_operation_resolves_after_restart(tmp_path, signatures, receipt_producer, signed_operation, committed_operation):
    journal = market.OperationJournal(tmp_path, signatures, commit_trust=committed_operation.trust)
    journal.record_signed(signed_operation)
    signed_operation.state = market.SdkState.SUBMITTING
    signed_operation.attempt = 1
    journal.persist(signed_operation)
    reopened = market.OperationJournal(tmp_path, signatures, commit_trust=committed_operation.trust)
    recovered = reopened.load(signed_operation.activity_id)
    assert recovered.state is market.SdkState.UNKNOWN
    assert recovered.rung is None
    assert reopened.load(recovered.activity_id) == recovered
    assert reopened.resolve(recovered, None, ()) is market.SdkState.UNKNOWN
    receipt = produced_receipt(receipt_producer, recovered)
    assert reopened.resolve(recovered, receipt, (RECEIPT_KEY,)) is market.SdkState.EXECUTED
    assert reopened.finalize(recovered, committed_operation.evidence) is market.SdkState.FINALIZED
    assert reopened.load(recovered.activity_id) == recovered
    assert recovered.signed_bytes == signed_operation.signed_bytes


def test_market_receipt_refusals_leave_the_journal_unchanged(tmp_path, signatures, receipt_producer, signed_operation, committed_operation):
    journal = market.OperationJournal(tmp_path, signatures, commit_trust=committed_operation.trust)
    record = pending(journal, signed_operation)
    receipt = produced_receipt(receipt_producer, record)
    before = record.encode()
    for evidence, keys, check in (
        (receipt, (), "unpinned_key"),
        (receipt, (replace(RECEIPT_KEY, first_height=HEIGHT + 1),), "key_out_of_range"),
        (receipt[:-1] + bytes([receipt[-1] ^ 1]), (RECEIPT_KEY,), "bad_signature"),
        (receipt[:40], (RECEIPT_KEY,), "bad_signature"),
    ):
        with pytest.raises(SequencerReceiptVerificationError) as raised:
            journal.resolve(record, evidence, keys)
        assert raised.value.check == check
        assert record.encode() == before
        assert journal.load(record.activity_id) == record
    for evidence in (
        produced_receipt(receipt_producer, record, nonce=8),
        produced_receipt(receipt_producer, record, recipient=b"\x11" * 20),
    ):
        with pytest.raises(market.AiMarketError, match="ReceiptMismatch"):
            journal.resolve(record, evidence, (RECEIPT_KEY,))
        assert record.encode() == before
        assert journal.load(record.activity_id) == record


def test_market_landing_matches_the_signed_receipt(tmp_path, signatures, receipt_producer, signed_operation, committed_operation):
    journal = market.OperationJournal(tmp_path, signatures, commit_trust=committed_operation.trust)
    record = pending(journal, signed_operation)
    receipt = produced_receipt(receipt_producer, record)
    landing = {
        "transactionHash": "0x" + record.activity_id.hex(),
        "status": "0x1", "blockNumber": hex(HEIGHT), "blockHash": "0x" + committed_operation.block_hash.hex(),
    }
    before = record.encode()
    for changes in (
        {"transactionHash": "0x" + "11" * 32}, {"status": "0x0"},
        {"blockNumber": hex(HEIGHT - 1)}, {"blockNumber": "0x04b0"},
        {"blockHash": "invalid"},
    ):
        with pytest.raises(market.AiMarketError, match="ReceiptMismatch"):
            journal.resolve(record, receipt, (RECEIPT_KEY,), {**landing, **changes})
        assert record.encode() == before
    assert journal.resolve(record, receipt, (RECEIPT_KEY,), landing) is market.SdkState.EXECUTED
    assert record.rung is market.Rung.RECEIPTED
    assert record.checkpoint is None
    assert journal.load(record.activity_id) == record
    assert journal.finalize(record, committed_operation.evidence) is market.SdkState.FINALIZED


def test_market_record_rejects_inconsistent_rungs_and_legacy_layout(tmp_path, signatures, receipt_producer, signed_operation, committed_operation):
    journal = market.OperationJournal(tmp_path, signatures, commit_trust=committed_operation.trust)
    record = pending(journal, signed_operation)
    assert journal.resolve(record, produced_receipt(receipt_producer, record), (RECEIPT_KEY,)) is market.SdkState.EXECUTED
    for invalid in (
        replace(record, rung=None), replace(record, rung=market.Rung.SUBMITTED),
        replace(record, rung=3), replace(record, receipt_height=None),
        replace(record, receipt_height=0), replace(record, global_sequence=0),
        replace(record, receipt_block_hash=bytes(32)),
        replace(record, state=market.SdkState.FINALIZED),
    ):
        with pytest.raises(market.AiMarketError, match="CorruptRecord"):
            market.OperationRecord.decode(invalid.encode(), signatures)
    with pytest.raises(market.AiMarketError, match="CorruptRecord"):
        market.OperationRecord.decode(b"PAXAIOP1" + record.encode()[8:], signatures)
    assert journal.load(record.activity_id) == record


def test_market_finality_covers_the_transaction_landing_after_restart(tmp_path, signatures, receipt_producer, signed_operation, committed_operation):
    committed_operation = committed_operation.at(HEIGHT + 1)
    journal = market.OperationJournal(tmp_path, signatures, commit_trust=committed_operation.trust)
    record = pending(journal, signed_operation)
    receipt = produced_receipt(receipt_producer, record)
    landing = {
        "transactionHash": "0x" + record.activity_id.hex(),
        "status": "0x1",
        "blockNumber": hex(HEIGHT + 1),
        "blockHash": "0x" + committed_operation.block_hash.hex(),
    }
    assert journal.resolve(record, receipt, (RECEIPT_KEY,), landing) is market.SdkState.EXECUTED
    reopened = market.OperationJournal(tmp_path, signatures, commit_trust=committed_operation.trust)
    recovered = reopened.load(record.activity_id)
    assert recovered.receipt_height == HEIGHT + 1
    assert recovered.receipt_block_hash == committed_operation.block_hash
    assert recovered.global_sequence == SEQUENCE
    assert recovered.rung is market.Rung.RECEIPTED
    before = recovered.encode()
    with pytest.raises(market.AiMarketError, match="FinalityUnavailable"):
        reopened.finalize(recovered, committed_operation.at(HEIGHT).evidence)
    assert recovered.encode() == before
    assert reopened.load(recovered.activity_id) == recovered
    assert reopened.finalize(recovered, committed_operation.evidence) is market.SdkState.FINALIZED
    assert reopened.load(recovered.activity_id) == recovered
    assert recovered.checkpoint == committed_operation.block_hash


def test_market_finality_must_match_the_landing_block_after_restart(
    tmp_path, signatures, receipt_producer, signed_operation, committed_operation
):
    journal = market.OperationJournal(tmp_path, signatures, commit_trust=committed_operation.trust)
    record = pending(journal, signed_operation)
    landing = {
        "transactionHash": "0x" + record.activity_id.hex(),
        "status": "0x1",
        "blockNumber": hex(HEIGHT),
        "blockHash": "0x" + committed_operation.block_hash.hex(),
    }
    receipt = produced_receipt(receipt_producer, record)
    assert journal.resolve(record, receipt, (RECEIPT_KEY,), landing) is market.SdkState.EXECUTED
    reopened = market.OperationJournal(tmp_path, signatures, commit_trust=committed_operation.trust)
    recovered = reopened.load(record.activity_id)
    assert recovered.receipt_block_hash == committed_operation.block_hash
    unrelated = committed_operation.at(HEIGHT + 1)
    before = recovered.encode()
    refused = market.OperationJournal(tmp_path, signatures, commit_trust=unrelated.trust)
    with pytest.raises(market.AiMarketError, match="FinalityMismatch"):
        refused.finalize(recovered, unrelated.evidence)
    assert recovered.encode() == before
    assert reopened.load(record.activity_id) == recovered
    assert reopened.finalize(recovered, committed_operation.evidence) is market.SdkState.FINALIZED
    assert reopened.load(record.activity_id) == recovered


def test_market_receipt_requires_a_nonzero_execution_height(tmp_path, signatures, receipt_producer, signed_operation, committed_operation):
    journal = market.OperationJournal(tmp_path, signatures, commit_trust=committed_operation.trust)
    record = pending(journal, signed_operation)
    receipt = produced_receipt(receipt_producer, record, height=0)
    key = replace(RECEIPT_KEY, first_height=0)
    before = record.encode()
    with pytest.raises(market.AiMarketError, match="ReceiptMismatch"):
        journal.resolve(record, receipt, (key,))
    assert record.encode() == before
    assert journal.load(record.activity_id) == record


@pytest.mark.parametrize("field", ("block_hash", "app_hash", "store_proof_digest"))
def test_market_commit_requires_nonzero_digests(field):
    for invalid in (bytes(32), b"\x01" * 31):
        with pytest.raises(market.AiMarketError, match="FinalityUnavailable"):
            replace(COMMIT, **{field: invalid})


@pytest.mark.parametrize("height", (-1, 1 << 64, True, "1200"))
def test_market_commit_requires_a_canonical_height(height):
    with pytest.raises(market.AiMarketError, match="FinalityUnavailable"):
        replace(COMMIT, block_height=height)


@pytest.mark.parametrize("mutation", ("transaction", "aunt", "index", "header", "signature", "quorum", "chain"))
def test_market_forged_commit_is_refused_after_restart(
    tmp_path, signatures, receipt_producer, signed_operation, committed_operation, mutation
):
    journal = market.OperationJournal(tmp_path, signatures)
    record = pending(journal, signed_operation)
    journal.resolve(record, produced_receipt(receipt_producer, record), (RECEIPT_KEY,))
    reopened = market.OperationJournal(tmp_path, signatures, commit_trust=committed_operation.trust)
    recovered = reopened.load(record.activity_id)
    before = recovered.encode()
    forged = deepcopy(committed_operation.evidence)
    if mutation == "transaction":
        forged["proof"]["data"] = "ff"
    elif mutation == "aunt":
        forged["proof"]["aunts"][0] = "00" * 32
    elif mutation == "index":
        forged["proof"]["index"] = 1
    elif mutation == "header":
        forged["header"]["app_hash"] = "00" * 32
    elif mutation == "signature":
        forged["commit"]["signatures"][0]["signature"] = "00" * 64
    elif mutation == "quorum":
        forged["commit"]["signatures"] = forged["commit"]["signatures"][:2]
    else:
        forged["header"]["chain_id"] = "unrelated-chain"
    with pytest.raises(market.AiMarketError, match="FinalityUnavailable"):
        reopened.finalize(recovered, forged)
    assert recovered.encode() == before
    assert reopened.load(record.activity_id) == recovered
    assert reopened.finalize(recovered, committed_operation.evidence) is market.SdkState.FINALIZED
    assert market.OperationJournal(tmp_path, signatures).load(record.activity_id) == recovered


def test_market_node_receipt_summary_events_and_return_binding(tmp_path, signatures, signed_operation):
    from test_repair_22_33_17 import ReceiptNode, http_server
    from layerx_sdk import programs
    from layerx_sdk.x402 import verify_sequencer_receipt
    from layerx_sdk.x402_rpc import HostedGatewayRpc, PaymentRpc

    corpus = json.loads((ROOT / "sdk/platform/conformance/fixtures/sequencer-receipt-v1.json").read_text())
    assert corpus["provenance"]["generator"] == "rpc/sequencer_receipt_vectors_test.go"
    vector = next(case for case in corpus["cases"] if case["id"] == "V1-layerxprograms-call")
    raw = bytes.fromhex(vector["rawTransaction"][2:])
    node = ReceiptNode()
    node.add_recording(vector)
    gateway = ReceiptNode()
    gateway.path = "/rpc"
    gateway.add_recording(vector)
    with http_server(node.dispatch) as endpoint, http_server(gateway.dispatch) as gateway_endpoint:
        rpc = PaymentRpc(endpoint)
        keys = HostedGatewayRpc(gateway_endpoint + "/rpc").receipt_keys()
        result = rpc.receipt(vector["receiptJson"]["txHash"])
        assert [request["method"] for request in gateway.requests] == ["px_getReceiptKeys"]
        assert [request["method"] for request in node.requests] == ["pax_sequencerReceipt"]
        receipt = bytes.fromhex(result["receipt"][2:])
        verified = verify_sequencer_receipt(receipt, raw, keys, return_data=bytes.fromhex(result["returnData"][2:]))
        assert len(verified.receipt.effects) == len(result["logs"]) + 1
        assert verified.receipt.effects[0].kind == 1
        assert verified.receipt.activity_root == sha256(bytes.fromhex(result["returnData"][2:])).digest()
        trust = programs.ProgramTrustContext(keys[0].public_key, network_id=125, receipt_keys=keys)
        assert programs._submission(result, signatures, trust, retained_signed_activity=raw.hex()) == result
        for field, value in (("returnData", "0x00"), ("gasUsed", "0x1"), ("logs", [])):
            with pytest.raises(ValueError, match="binding failed"):
                programs._submission({**result, field: value}, signatures, trust, retained_signed_activity=raw.hex())
        journal = market.OperationJournal(tmp_path, signatures)
        record = pending(journal, signed_operation)
        before = record.encode()
        with pytest.raises(SequencerReceiptVerificationError) as error:
            journal.resolve(record, receipt, keys)
        assert error.value.check == "hash_mismatch"
        assert journal.load(record.activity_id).encode() == before



def test_market_commit_codec_matches_recorded_producer():
    from test_repair_22_31_147 import test_commit_codec_matches_recorded_comet_producer
    test_commit_codec_matches_recorded_comet_producer()


def test_market_finality_dto_cannot_authorize_a_commit(tmp_path, signatures, receipt_producer, signed_operation):
    journal = market.OperationJournal(tmp_path, signatures)
    record = pending(journal, signed_operation)
    journal.resolve(record, produced_receipt(receipt_producer, record), (RECEIPT_KEY,))
    before = record.encode()
    with pytest.raises(market.AiMarketError, match="FinalityUnavailable"):
        journal.finalize(record, COMMIT)
    assert record.encode() == before
    assert journal.load(record.activity_id).encode() == before
