from __future__ import annotations

import importlib.util
import subprocess
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
def receipt_producer(tmp_path_factory):
    executable = tmp_path_factory.mktemp("market-receipt-producer") / "producer"
    sources = (
        "src/protocol/lxp_receipt.c",
        "src/codec/lxp_codec.c",
        "src/protocol/lxp_arena.c",
        "src/protocol/lxp_protocol.c",
        "src/protocol/lxp_result.c",
        "src/protocol/lxp_u128.c",
        "src/crypto/lxp_hash.c",
        "src/crypto/lxp_ct.c",
        "src/crypto/lxp_ed25519.c",
        "src/crypto/lxp_merkle.c",
    )
    compiled = subprocess.run(
        [
            "cc", "-std=c17", "-Wall", "-Wextra", "-Werror", "-Iinclude",
            str(Path(__file__).with_name("market_receipt_producer.c")),
            *sources, "-lcrypto", "-o", str(executable),
        ],
        cwd=ROOT, capture_output=True, timeout=60, check=False,
    )
    assert compiled.returncode == 0, compiled.stderr.decode()
    return executable


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
    fields = market._transaction_fields(record.signed_bytes, True)
    signature = (
        int.from_bytes(fields[10], "big").to_bytes(32, "big")
        + int.from_bytes(fields[11], "big").to_bytes(32, "big")
        + bytes([27 + int.from_bytes(fields[9], "big")])
    )
    public = market._signature_public_key(signature, keccak256(b"\x02" + market._rlp(fields[:9])))
    generated = subprocess.run(
        [str(producer)],
        input=(
            record.activity_id + bytes.fromhex(market._key_address(public)[2:])
            + (fields[5] if recipient is None else recipient)
            + nonce.to_bytes(8, "big") + bytes([result_code])
            + height.to_bytes(8, "big") + SEQUENCE.to_bytes(8, "big")
        ),
        capture_output=True, timeout=10, check=False,
    )
    assert generated.returncode == 0, generated.stderr.decode()
    assert generated.stdout
    return generated.stdout


def pending(journal, record):
    journal.record_signed(record)
    record.state, record.attempt, record.rung = market.SdkState.PENDING, 1, market.Rung.SUBMITTED
    journal.persist(record)
    return journal.load(record.activity_id)


def test_committed_market_operation_survives_restart(tmp_path, signatures, receipt_producer, signed_operation):
    journal = market.OperationJournal(tmp_path, signatures)
    record = pending(journal, signed_operation)
    receipt = produced_receipt(receipt_producer, record)
    assert journal.resolve(record, receipt, (RECEIPT_KEY,)) is market.SdkState.EXECUTED
    assert (record.global_sequence, record.receipt_height, record.rung) == (
        SEQUENCE, HEIGHT, market.Rung.RECEIPTED,
    )
    reopened = market.OperationJournal(tmp_path, signatures)
    recovered = reopened.load(record.activity_id)
    assert recovered == record
    assert reopened.finalize(recovered, COMMIT) is market.SdkState.FINALIZED
    assert recovered.checkpoint == COMMIT.block_hash
    assert recovered.rung is market.Rung.COMMITTED
    assert recovered.domain is market.DomainStatus.PENDING
    assert recovered.signed_bytes == signed_operation.signed_bytes
    assert market.OperationJournal(tmp_path, signatures).load(record.activity_id) == recovered
    with pytest.raises(market.AiMarketError, match="InvalidTransition"):
        reopened.finalize(recovered, COMMIT)


def test_reverted_market_operation_is_durable_failure(tmp_path, signatures, receipt_producer, signed_operation):
    journal = market.OperationJournal(tmp_path, signatures)
    record = pending(journal, signed_operation)
    receipt = produced_receipt(receipt_producer, record, 1)
    assert journal.resolve(record, receipt, (RECEIPT_KEY,)) is market.SdkState.FAILED
    assert record.result_code == 1
    assert record.checkpoint is None
    assert record.rung is market.Rung.RECEIPTED
    recovered = market.OperationJournal(tmp_path, signatures).load(record.activity_id)
    assert recovered == record
    with pytest.raises(market.AiMarketError, match="InvalidTransition"):
        journal.finalize(recovered, COMMIT)
    assert journal.load(record.activity_id) == record


def test_reverted_market_transaction_landing_survives_restart(
    tmp_path, signatures, receipt_producer, signed_operation
):
    journal = market.OperationJournal(tmp_path, signatures)
    record = pending(journal, signed_operation)
    receipt = produced_receipt(receipt_producer, record, 1)
    landing = {
        "transactionHash": "0x" + record.activity_id.hex(),
        "status": "0x0",
        "blockNumber": hex(HEIGHT),
        "blockHash": "0x" + COMMIT.block_hash.hex(),
    }
    before = record.encode()
    with pytest.raises(market.AiMarketError, match="ReceiptMismatch"):
        journal.resolve(record, receipt, (RECEIPT_KEY,), {**landing, "status": "0x1"})
    assert record.encode() == before
    assert journal.load(record.activity_id) == record
    assert journal.resolve(record, receipt, (RECEIPT_KEY,), landing) is market.SdkState.FAILED
    reopened = market.OperationJournal(tmp_path, signatures)
    recovered = reopened.load(record.activity_id)
    assert recovered == record
    assert recovered.result_code == 1
    assert recovered.rung is market.Rung.COMMITTED
    assert recovered.receipt_height == HEIGHT
    assert recovered.receipt_block_hash == COMMIT.block_hash
    assert recovered.checkpoint is None
    with pytest.raises(market.AiMarketError, match="InvalidTransition"):
        reopened.finalize(recovered, COMMIT)
    assert reopened.load(record.activity_id) == recovered


def test_market_operation_waits_for_covering_commit(tmp_path, signatures, receipt_producer, signed_operation):
    journal = market.OperationJournal(tmp_path, signatures)
    record = pending(journal, signed_operation)
    before = record.encode()
    assert journal.resolve(record, None, ()) is market.SdkState.PENDING
    assert record.encode() == before
    receipt = produced_receipt(receipt_producer, record)
    assert journal.resolve(record, receipt, (RECEIPT_KEY,)) is market.SdkState.EXECUTED
    before = record.encode()
    with pytest.raises(market.AiMarketError, match="FinalityMismatch"):
        journal.finalize(record, replace(COMMIT, block_height=HEIGHT - 1))
    assert record.encode() == before
    assert journal.load(record.activity_id) == record
    assert record.checkpoint is None


def test_unknown_market_operation_resolves_after_restart(tmp_path, signatures, receipt_producer, signed_operation):
    journal = market.OperationJournal(tmp_path, signatures)
    journal.record_signed(signed_operation)
    signed_operation.state = market.SdkState.SUBMITTING
    signed_operation.attempt = 1
    journal.persist(signed_operation)
    reopened = market.OperationJournal(tmp_path, signatures)
    recovered = reopened.load(signed_operation.activity_id)
    assert recovered.state is market.SdkState.UNKNOWN
    assert recovered.rung is None
    assert reopened.load(recovered.activity_id) == recovered
    assert reopened.resolve(recovered, None, ()) is market.SdkState.UNKNOWN
    receipt = produced_receipt(receipt_producer, recovered)
    assert reopened.resolve(recovered, receipt, (RECEIPT_KEY,)) is market.SdkState.EXECUTED
    assert reopened.finalize(recovered, replace(COMMIT, block_height=HEIGHT + 1)) is market.SdkState.FINALIZED
    assert reopened.load(recovered.activity_id) == recovered
    assert recovered.signed_bytes == signed_operation.signed_bytes


def test_market_receipt_refusals_leave_the_journal_unchanged(tmp_path, signatures, receipt_producer, signed_operation):
    journal = market.OperationJournal(tmp_path, signatures)
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


def test_market_landing_matches_the_signed_receipt(tmp_path, signatures, receipt_producer, signed_operation):
    journal = market.OperationJournal(tmp_path, signatures)
    record = pending(journal, signed_operation)
    receipt = produced_receipt(receipt_producer, record)
    landing = {
        "transactionHash": "0x" + record.activity_id.hex(),
        "status": "0x1", "blockNumber": hex(HEIGHT), "blockHash": "0x" + COMMIT.block_hash.hex(),
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
    assert record.rung is market.Rung.COMMITTED
    assert record.checkpoint is None
    assert journal.load(record.activity_id) == record
    assert journal.finalize(record, COMMIT) is market.SdkState.FINALIZED


def test_market_record_rejects_inconsistent_rungs_and_legacy_layout(tmp_path, signatures, receipt_producer, signed_operation):
    journal = market.OperationJournal(tmp_path, signatures)
    record = pending(journal, signed_operation)
    assert journal.resolve(record, produced_receipt(receipt_producer, record), (RECEIPT_KEY,)) is market.SdkState.EXECUTED
    for invalid in (
        replace(record, rung=None), replace(record, rung=market.Rung.SUBMITTED),
        replace(record, rung=3), replace(record, receipt_height=None),
        replace(record, receipt_height=0), replace(record, global_sequence=0),
        replace(record, receipt_block_hash=COMMIT.block_hash),
        replace(record, state=market.SdkState.FINALIZED),
    ):
        with pytest.raises(market.AiMarketError, match="CorruptRecord"):
            market.OperationRecord.decode(invalid.encode(), signatures)
    with pytest.raises(market.AiMarketError, match="CorruptRecord"):
        market.OperationRecord.decode(b"PAXAIOP1" + record.encode()[8:], signatures)
    assert journal.load(record.activity_id) == record


def test_market_finality_covers_the_transaction_landing_after_restart(tmp_path, signatures, receipt_producer, signed_operation):
    journal = market.OperationJournal(tmp_path, signatures)
    record = pending(journal, signed_operation)
    receipt = produced_receipt(receipt_producer, record)
    landing = {
        "transactionHash": "0x" + record.activity_id.hex(),
        "status": "0x1",
        "blockNumber": hex(HEIGHT + 1),
        "blockHash": "0x" + COMMIT.block_hash.hex(),
    }
    assert journal.resolve(record, receipt, (RECEIPT_KEY,), landing) is market.SdkState.EXECUTED
    reopened = market.OperationJournal(tmp_path, signatures)
    recovered = reopened.load(record.activity_id)
    assert recovered.receipt_height == HEIGHT + 1
    assert recovered.receipt_block_hash == COMMIT.block_hash
    assert recovered.global_sequence == SEQUENCE
    assert recovered.rung is market.Rung.COMMITTED
    before = recovered.encode()
    with pytest.raises(market.AiMarketError, match="FinalityMismatch"):
        reopened.finalize(recovered, COMMIT)
    assert recovered.encode() == before
    assert reopened.load(recovered.activity_id) == recovered
    assert reopened.finalize(recovered, replace(COMMIT, block_height=HEIGHT + 1)) is market.SdkState.FINALIZED
    assert reopened.load(recovered.activity_id) == recovered
    assert recovered.checkpoint == COMMIT.block_hash


def test_market_finality_must_match_the_landing_block_after_restart(
    tmp_path, signatures, receipt_producer, signed_operation
):
    journal = market.OperationJournal(tmp_path, signatures)
    record = pending(journal, signed_operation)
    landing = {
        "transactionHash": "0x" + record.activity_id.hex(),
        "status": "0x1",
        "blockNumber": hex(HEIGHT),
        "blockHash": "0x" + COMMIT.block_hash.hex(),
    }
    receipt = produced_receipt(receipt_producer, record)
    assert journal.resolve(record, receipt, (RECEIPT_KEY,), landing) is market.SdkState.EXECUTED
    reopened = market.OperationJournal(tmp_path, signatures)
    recovered = reopened.load(record.activity_id)
    assert recovered.receipt_block_hash == COMMIT.block_hash
    before = recovered.encode()
    with pytest.raises(market.AiMarketError, match="FinalityMismatch"):
        reopened.finalize(recovered, replace(COMMIT, block_hash=b"\x77" * 32))
    assert recovered.encode() == before
    assert reopened.load(record.activity_id) == recovered
    assert reopened.finalize(recovered, COMMIT) is market.SdkState.FINALIZED
    assert reopened.load(record.activity_id) == recovered


def test_market_receipt_requires_a_nonzero_execution_height(tmp_path, signatures, receipt_producer, signed_operation):
    journal = market.OperationJournal(tmp_path, signatures)
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
