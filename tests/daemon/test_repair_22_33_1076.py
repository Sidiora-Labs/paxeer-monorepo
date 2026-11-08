"""Exercise the retained client path and require genuine commitment/recovery evidence."""

from copy import deepcopy
from dataclasses import replace

import pytest

from layerx_sdk.native_program_call import (
    NativeProgramCall,
    decode_native_program_call,
    encode_native_program_call,
)
from layerx_sdk.evm_transaction import EvmTransactionError, decode_signed
from layerx_sdk.program_wire import verify_native_program_call_signature
from layerx_sdk.production import PlatformSdkError, SdkErrorCode
from layerx_sdk.verifier import (
    ReceiptKey,
    SequencerReceiptVerificationError,
    verify_sequencer_receipt,
)

from recorded_paxd import (
    RecordedPaxd,
    program_transaction,
    programs_execution_case,
    verify_program_answer,
)
from test_handover_chain_no_read_consumer import (
    test_handover_chain_needs_no_read_consumer_binary as prove_handover_retirement,
)
from test_protocol_owner_binds_state_feed import (
    test_protocol_owner_binds_the_state_feed as prove_runtime_binding,
)
from test_serve_commits_programs_call import (
    test_serve_commits_an_admitted_programs_call as prove_program_commit,
)
from test_serve_startup_without_anchor_bind import (
    test_serve_startup_no_longer_binds_the_finality_authority as prove_anchorless_recovery,
)


def test_repair_f016_retained_handover_needs_no_read_consumer():
    prove_handover_retirement()


def test_repair_f017_embedded_runtime_owns_and_binds_state():
    prove_runtime_binding()


def test_repair_f018_programs_call_executes_and_commits():
    prove_program_commit()


def test_repair_f019_startup_and_reopen_without_anchor_store():
    prove_anchorless_recovery()


def test_original_programs_call_admission_payload_keeps_every_bound():
    call = NativeProgramCall(
        bytes.fromhex("11" * 32), 1, "layerx_call", b"", b"\0\0",
        b"LayerX/programs/access-declaration/v1\0\0", 16,
        (1_000_000, 16_777_216, 1_048_576, 1_048_576, 64, 1_048_576, 4096),
    )
    payload = encode_native_program_call(call)
    assert payload[32:50] == bytes.fromhex("0001000b0000000000020000002700000010")
    assert decode_native_program_call(payload) == call
    for length in range(len(payload)):
        with pytest.raises(ValueError):
            decode_native_program_call(payload[:length])
    with pytest.raises(ValueError):
        decode_native_program_call(payload + b"\0")
    with pytest.raises(ValueError):
        encode_native_program_call(replace(call, program_id=bytes(32)))


def test_recorded_embedded_program_execution_uses_real_codec_and_rpc():
    case = programs_execution_case()
    raw, call = program_transaction(case)
    peer = RecordedPaxd.execution(case)
    with peer.serving() as client:
        tx_hash = client.send(case["rawTransaction"])
        answer = client.receipt(tx_hash)
        assert answer == case["receiptJson"]
        verified = verify_program_answer(case, answer)
        assert verified.canonical_bytes == bytes.fromhex(case["signedReceipt"][2:])
        assert verified.receipt.activity_id.hex() == tx_hash[2:]
        assert call.guest_abi == 1
        assert raw[0] == 2
    assert [request["id"] for request in peer.requests] == [1, 2]
    assert [request["method"] for request in peer.requests] == [
        "eth_sendRawTransaction", "pax_sequencerReceipt",
    ]


def test_receipted_vector_cannot_stand_in_for_original_call():
    case = programs_execution_case()
    _, call = program_transaction(case)
    assert call.response_capacity == 64
    assert case["receiptJson"]["rung"] == "receipted"
    with pytest.raises(AssertionError, match="program response capacity differs from admission"):
        program_transaction(case, response_capacity=16)
    peer = RecordedPaxd.execution(case)
    with peer.serving() as client:
        tx_hash = client.send(case["rawTransaction"])
        answer = client.receipt(tx_hash)
        with pytest.raises(AssertionError, match="program response capacity differs from admission"):
            verify_program_answer(case, answer, response_capacity=16)


@pytest.mark.parametrize("field", ["returnData", "logs", "receipt", "status", "from", "txHash"])
def test_program_execution_refuses_tampered_transport_evidence(field):
    case = programs_execution_case()
    changed = deepcopy(case["receiptJson"])
    if field == "logs":
        changed["logs"][0]["data"] = "0x00"
    elif field == "receipt":
        wire = bytearray.fromhex(changed[field][2:])
        wire[-1] ^= 1
        changed[field] = "0x" + wire.hex()
    elif field == "status":
        changed[field] = "0x0"
    elif field == "from":
        changed[field] = "0x" + "00" * 20
    elif field == "txHash":
        changed[field] = "0x" + "00" * 32
    else:
        changed[field] = "0x00"
    peer = RecordedPaxd.execution(case)
    peer.exchanges[1]["result"] = changed
    with peer.serving() as client:
        tx_hash = client.send(case["rawTransaction"])
        if field == "status":
            with pytest.raises(PlatformSdkError) as failure:
                client.receipt(tx_hash)
            assert failure.value.code == SdkErrorCode.DECODE_FAILURE
            with pytest.raises(ValueError, match="program sequencer receipt binding failed"):
                verify_program_answer(case, changed)
        else:
            error = SequencerReceiptVerificationError if field == "receipt" else ValueError
            with pytest.raises(error):
                answer = client.receipt(tx_hash)
                verify_program_answer(case, answer)


@pytest.mark.parametrize("mutation, check", [
    ("key_id", "unpinned_key"),
    ("public_key", "bad_signature"),
    ("first_height", "key_out_of_range"),
    ("last_height", "key_out_of_range"),
])
def test_program_receipt_refuses_untrusted_or_expired_credentials(mutation, check):
    case = programs_execution_case()
    raw, _ = program_transaction(case)
    pin = case["pinnedKey"]
    key = ReceiptKey(
        bytes.fromhex(pin["keyId"][2:]), bytes.fromhex(pin["publicKey"][2:]),
        pin["firstHeight"], pin["lastHeight"],
    )
    receipt = bytes.fromhex(case["signedReceipt"][2:])
    verified = verify_sequencer_receipt(receipt, raw, (key,))
    changes = {
        "key_id": b"\x01" * 32,
        "public_key": b"\x02" * 32,
        "first_height": verified.height + 1,
        "last_height": verified.height - 1,
    }
    with pytest.raises(SequencerReceiptVerificationError) as failure:
        verify_sequencer_receipt(receipt, raw, (replace(key, **{mutation: changes[mutation]}),))
    assert failure.value.check == check


def test_native_program_admission_refuses_foreign_network_and_forged_signature():
    case = programs_execution_case()
    canonical, _ = program_transaction(case)
    transaction, signature = decode_signed(canonical)
    signer = case["receiptJson"]["from"]
    assert transaction.attach_signature(signature, signer) == canonical
    verify_native_program_call_signature(canonical, None, expected_network=125)
    with pytest.raises(ValueError, match="^invalid program transaction network$"):
        verify_native_program_call_signature(canonical, None, expected_network=7)

    invalid_signature = canonical[:-32] + bytes(32)
    with pytest.raises(EvmTransactionError, match="^invalid_signature$"):
        verify_native_program_call_signature(invalid_signature, None, expected_network=125)

    forged = canonical[:-1] + bytes([canonical[-1] ^ 1])
    _, forged_signature = decode_signed(forged)
    with pytest.raises(EvmTransactionError, match="^signer_mismatch$"):
        transaction.attach_signature(forged_signature, signer)
    pin = case["pinnedKey"]
    key = ReceiptKey(
        bytes.fromhex(pin["keyId"][2:]), bytes.fromhex(pin["publicKey"][2:]),
        pin["firstHeight"], pin["lastHeight"],
    )
    with pytest.raises(SequencerReceiptVerificationError) as failure:
        verify_sequencer_receipt(bytes.fromhex(case["signedReceipt"][2:]), forged, (key,))
    assert failure.value.check == "hash_mismatch"
