from __future__ import annotations

import ast
from contextlib import contextmanager
import sqlite3
from dataclasses import replace
from hashlib import sha256
from pathlib import Path

import pytest
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey

from layerx_sdk.account_derivation import keccak256
from layerx_sdk.generated.receipt import ReceiptFailureCode
from layerx_sdk.production import PlatformSdkError, SdkErrorCode
from layerx_sdk.revert import RevertReasonError
from layerx_sdk.verifier import (
    ReceiptVerificationError,
    SequencerReceiptVerificationError,
    _decode_sequencer_receipt,
)
from layerx_sdk.x402 import (
    PaymentCommitmentError,
    payment_commitment,
    receipt_protocol_version,
    verify_payment_receipt,
    verify_sequencer_receipt,
)
from layerx_sdk.x402_draw import PreparedGrantDraws
from layerx_sdk.x402_grant import validate_grant_draw
from layerx_sdk.x402_http import ConfiguredReceiptAuthority, PaymentEvidence
from layerx_sdk.x402_receive import (
    encode_receive,
    grant_authorization_message,
    receive_authorization_message,
)
from test_sequencer_receipt import (
    VECTORS,
    _bytes,
    _key,
    _payment_terms as _ledger_payment_terms,
    _payment_vector,
)


def _payment_terms(vector):
    terms = _ledger_payment_terms(vector)
    return {**terms, "ledger_payer": terms["payer"],
            "payer": _bytes(vector["receiptJson"]["from"]).rjust(32, b"\0").hex()}


def test_x402_draw_verifies_the_sequencer_receipt():
    for vector in VECTORS:
        if vector["vector"] != "V1":
            continue
        signed = _bytes(vector["signedReceipt"])
        raw = _bytes(vector["rawTransaction"])
        verified = verify_sequencer_receipt(signed, raw, [_key(vector)])
        assert verified.level == "sequencer-signed"
        assert verified.receipt.activity_id == keccak256(raw)
        assert verified.receipt.activity_id == _bytes(vector["receiptJson"]["txHash"])
        assert verified.canonical_bytes == signed
        assert verified.receipt_digest == _bytes(vector["receiptDigest"])
        assert verified.height == int(vector["receiptJson"]["height"], 16)
        assert verified.receipt.result_code == 0

    vector = _payment_vector()
    signed = _bytes(vector["signedReceipt"])
    raw = _bytes(vector["rawTransaction"])
    key = _key(vector)
    terms = _payment_terms(vector)
    payment = verify_payment_receipt(signed, raw, (key,), return_data=b"", **terms)
    assert payment == verify_sequencer_receipt(signed, raw, [key])
    payment_topic = "0x" + keccak256(
        b"PaymentExact(bytes32,bytes32,bytes32,bytes32,uint128)"
    ).hex()
    payment_logs = [
        entry for entry in vector["receiptJson"]["logs"]
        if entry["topics"][0] == payment_topic
    ]
    assert len(payment_logs) == 1
    log = payment_logs[0]
    assert int.from_bytes(_bytes(log["data"])[32:], "big") == int(terms["amount"])
    assert _bytes(log["data"])[:32].hex() == terms["asset"]
    assert _bytes(log["topics"][3]).hex() == terms["pay_to"]
    assert payment.receipt.from_account == _bytes(vector["receiptJson"]["from"]).rjust(32, b"\0")
    from layerx_sdk.payment_transaction import decode_payment_transaction
    assert decode_payment_transaction(raw).data[4:36].hex() == terms["ledger_payer"]
    assert raw == _bytes(vector["rawTransaction"])

    for changes in (
        {"amount": str(int(terms["amount"]) + 1)},
        {"amount": "01"},
        {"asset": "2a" * 32},
        {"pay_to": "2a" * 32},
        {"payer": "2a" * 32},
        {"ledger_payer": "2a" * 32},
        {"purpose_hash": "2a" * 32},
        {"return_data": b"altered"},
    ):
        with pytest.raises(PlatformSdkError) as raised:
            verify_payment_receipt(signed, raw, [key], **{**terms, **changes})
        assert raised.value.code == SdkErrorCode.VERIFICATION_FAILURE

    for canonical, transaction, keys, check in (
        (signed, raw + b"\0", [key], "hash_mismatch"),
        (signed[:-1] + bytes([signed[-1] ^ 1]), raw, [key], "bad_signature"),
        (
            signed,
            raw,
            [replace(key, first_height=payment.height + 1)],
            "key_out_of_range",
        ),
    ):
        with pytest.raises(SequencerReceiptVerificationError) as raised:
            verify_payment_receipt(canonical, transaction, keys, **terms)
        assert raised.value.check == check

    reverted = next(vector for vector in VECTORS if vector["vector"] == "V2")
    signed = _bytes(reverted["signedReceipt"])
    raw = _bytes(reverted["rawTransaction"])
    with pytest.raises(ReceiptVerificationError) as raised:
        verify_sequencer_receipt(signed, raw, [_key(reverted)])
    assert raised.value.check == ReceiptFailureCode.FIELDS
    with pytest.raises(RevertReasonError) as raised:
        verify_sequencer_receipt(
            signed,
            raw,
            [_key(reverted)],
            return_data=_bytes(reverted["receiptJson"]["returnData"]),
        )
    assert raised.value.code == "LXP_INSUFFICIENT_BALANCE"


def test_x402_failure_paths_use_current_receipt_codes():
    reverted = next(vector for vector in VECTORS if vector["vector"] == "V2")
    signed = _bytes(reverted["signedReceipt"])
    raw = _bytes(reverted["rawTransaction"])
    key = _key(reverted)

    with pytest.raises(ReceiptVerificationError) as raised:
        verify_sequencer_receipt(signed, raw, [key])
    assert raised.value.check == ReceiptFailureCode.FIELDS

    with pytest.raises(RevertReasonError) as raised:
        verify_sequencer_receipt(
            signed,
            raw,
            [key],
            return_data=_bytes(reverted["receiptJson"]["returnData"]),
        )
    assert raised.value.code == "LXP_INSUFFICIENT_BALANCE"

    with pytest.raises(ReceiptVerificationError) as raised:
        receipt_protocol_version(b"\x00\x02\x52\x01\x00\x03")
    assert raised.value.check == ReceiptFailureCode.RECEIPT_VERSION

    for malformed in (b"", signed[:4] + b"\x00\x03" + signed[6:]):
        with pytest.raises(ReceiptVerificationError) as raised:
            _decode_sequencer_receipt(malformed)
        assert raised.value.check == ReceiptFailureCode.DECODE

    package = Path(__file__).resolve().parents[1] / "layerx_sdk"
    for path in sorted(package.rglob("*.py")):
        module = ast.parse(path.read_text(), filename=str(path))
        references = {
            node.attr
            for node in ast.walk(module)
            if isinstance(node, ast.Attribute)
            and isinstance(node.value, ast.Name)
            and node.value.id == "ReceiptFailureCode"
        }
        assert references <= ReceiptFailureCode.__members__.keys(), (
            path.relative_to(package),
            references - ReceiptFailureCode.__members__.keys(),
        )


def test_x402_draw_refuses_a_receipt_under_an_unknown_key():
    vector = _payment_vector()
    signed = _bytes(vector["signedReceipt"])
    raw = _bytes(vector["rawTransaction"])
    key = _key(vector)
    unknown = replace(key, key_id=b"\x2a" * 32)
    assert unknown.public_key == key.public_key
    for keys in ([unknown], []):
        with pytest.raises(SequencerReceiptVerificationError) as raised:
            verify_sequencer_receipt(signed, raw, keys)
        assert raised.value.check == "unpinned_key"
        assert raised.value.code == SdkErrorCode.VERIFICATION_FAILURE
        with pytest.raises(SequencerReceiptVerificationError) as raised:
            verify_payment_receipt(signed, raw, keys, **_payment_terms(vector))
        assert raised.value.check == "unpinned_key"
        assert raised.value.code == SdkErrorCode.VERIFICATION_FAILURE


@contextmanager
def _receipt_rpc(vector):
    from test_repair_22_33_17 import ReceiptNode, http_server
    from layerx_sdk.x402_rpc import PaymentRpc
    node = ReceiptNode()
    node.add_recording(vector)
    with http_server(node.dispatch) as endpoint:
        yield PaymentRpc(endpoint), node


def _grant_inputs(vector):
    terms = _payment_terms(vector)
    key = "11" * 32
    purpose = _bytes(vector["receiptJson"]["logs"][0]["topics"][1]).hex()
    context = sha256(b"sequencer-grant-context").hexdigest()
    grant_signer = Ed25519PrivateKey.generate()
    receiver_signer = Ed25519PrivateKey.generate()
    grant = {
        "grant_id": "22" * 32,
        "from": terms["ledger_payer"],
        "recipient": terms["pay_to"],
        "asset": terms["asset"],
        "per_draw_maximum": terms["amount"],
        "allowance": terms["amount"],
        "recurring": False,
        "window_length": "0",
        "expiration": str((1 << 64) - 1),
        "purpose_hash": purpose,
        "has_reference": False,
        "reference_hash": "00" * 32,
        "revocation_sequence": "0",
        "public_key": grant_signer.public_key().public_bytes_raw().hex(),
        "signature": "00" * 64,
    }
    grant["signature"] = grant_signer.sign(grant_authorization_message(grant)).hex()
    receive = {
        "from": terms["ledger_payer"],
        "to": terms["pay_to"],
        "asset": terms["asset"],
        "amount": terms["amount"],
        "grant_id": grant["grant_id"],
        "receiver_sequence": "0",
        "idempotency_key": key,
        "context_hash": context,
        "payer_grant": grant,
        "receiver_authorization": {
            "kind": 1,
            "controller": terms["pay_to"],
            "public_key": receiver_signer.public_key().public_bytes_raw().hex(),
            "signature": "00" * 64,
            "signed_context_hash": context,
            "network_id": 125,
            "protocol_version": 2,
        },
    }
    receive["receiver_authorization"]["signature"] = receiver_signer.sign(
        receive_authorization_message(receive)
    ).hex()
    offer = {
        "scheme": "metered",
        "network": "eip155:125",
        "amount": terms["amount"],
        "asset": terms["asset"],
        "payTo": terms["pay_to"],
        "maxTimeoutSeconds": 30,
        "extra": {
            "layerx": {
                "commitment": "receipted",
                "payer": terms["ledger_payer"],
                "purposeHash": purpose,
            }
        },
    }
    wire = encode_receive(receive)
    return wire, {"receive": wire.hex(), "idempotencyKey": key}, offer


def test_grant_draws_verify_through_receipt_keys(tmp_path):
    from test_repair_22_33_17 import draw_inputs
    vector = _payment_vector()
    signed, raw, key = _bytes(vector["signedReceipt"]), _bytes(vector["rawTransaction"]), _key(vector)
    receive, body, offer, values, transaction, actor = draw_inputs()
    request_digest = sha256(b"grant-draw-request").hexdigest()
    path = tmp_path / "draws.sqlite"
    with _receipt_rpc(vector) as (rpc, node):
        authority = ConfiguredReceiptAuthority(transaction)
        for reopen in range(2):
            draws = PreparedGrantDraws(path, actor, 125, rpc, authority, [key])
            with pytest.raises(RevertReasonError) as error:
                draws.register("buyer", request_digest, transaction, receive, body["idempotencyKey"])
            assert error.value.code == "LXP_NO_PAYER_GRANT"
            with pytest.raises(RevertReasonError, match="LXP_NO_PAYER_GRANT"):
                draws("buyer", request_digest, body, offer)
            with sqlite3.connect(path) as db:
                assert db.execute("SELECT * FROM grant_draws").fetchall() == []
            del draws
        assert node.requests == []
        assert rpc.send(vector["rawTransaction"]) == vector["receiptJson"]["txHash"]
        result = rpc.receipt(vector["receiptJson"]["txHash"])
        assert result == vector["receiptJson"]
        verified = verify_payment_receipt(signed, raw, (key,), **_payment_terms(vector))
        assert verified.level == "sequencer-signed"
        assert verified.receipt_digest == _bytes(vector["receiptDigest"])
        for keys, check in (
            ([replace(key, key_id=b"\x2a" * 32)], "unpinned_key"),
            ([replace(key, public_key=b"\x2a" * 32)], "bad_signature"),
        ):
            with pytest.raises(SequencerReceiptVerificationError) as raised:
                verify_payment_receipt(signed, raw, keys, **_payment_terms(vector))
            assert raised.value.check == check
            assert raised.value.code == SdkErrorCode.VERIFICATION_FAILURE
        assert [request["method"] for request in node.requests] == ["eth_sendRawTransaction", "pax_sequencerReceipt"]


@pytest.mark.parametrize("rung", ("unknown", "submitted", "receipted"))
def test_grant_draw_recovery_uses_payment_rpc_without_resubmission(tmp_path, rung):
    from test_repair_22_33_17 import draw_inputs, seed_legacy_journal
    from layerx_sdk.evm_transaction import decode_signed
    receive, body, offer, values, transaction, actor = draw_inputs()
    identity = "0x" + keccak256(transaction).hex()
    path = tmp_path / "legacy.sqlite"
    seed_legacy_journal(path, transaction, receive, body["idempotencyKey"], identity, 1)
    with _receipt_rpc(_payment_vector()) as (rpc, node):
        if rung != "unknown":
            node.receipts[identity] = {
                "txHash": identity, "nonce": hex(decode_signed(transaction)[0].nonce), "rung": rung,
                **({"status": "0x1", "receipt": _payment_vector()["signedReceipt"],
                    "keyId": _payment_vector()["pinnedKey"]["keyId"]} if rung == "receipted" else {}),
            }
        for reopen in range(2):
            draws = PreparedGrantDraws(path, actor, 125, rpc, ConfiguredReceiptAuthority(transaction), [_key(_payment_vector())])
            if rung == "receipted":
                # Legacy state can be looked up, but receipt-shaped data cannot revive grants.
                with pytest.raises(RevertReasonError, match="LXP_NO_PAYER_GRANT"):
                    draws.recover_pending(body["idempotencyKey"])
            else:
                assert draws.recover_pending(body["idempotencyKey"]) is None
            with sqlite3.connect(path) as db:
                assert db.execute("SELECT canonical, receive, transaction_hash, attempted FROM grant_draws").fetchall() == [(transaction, receive, identity, 1)]
            del draws
        assert len(node.requests) == 2
        assert all(request["method"] == "pax_sequencerReceipt" and request["params"] == [identity] for request in node.requests)
        assert identity not in node.transactions


def test_payment_commitment_refuses_the_retired_executed_rung():
    with pytest.raises(PaymentCommitmentError):
        payment_commitment({"layerx": {"commitment": "executed"}})
    receive, body, offer = _grant_inputs(_payment_vector())
    validated = validate_grant_draw(receive, offer, body["idempotencyKey"], 125, 0)
    assert validated["from"] == offer["extra"]["layerx"]["payer"]
    assert validated["receiver_authorization"]["network_id"] == 125
    retired = {
        **offer,
        "extra": {
            "layerx": {**offer["extra"]["layerx"], "commitment": "executed"}
        },
    }
    with pytest.raises(PaymentCommitmentError):
        validate_grant_draw(receive, retired, body["idempotencyKey"], 125, 0)


@pytest.mark.parametrize("commitment", ("submitted", "executed", "batched", "finalised", "finalized", "acknowledged", None, 1, [], {}))
def test_payment_commitment_cannot_downgrade_receipt_verification(commitment):
    vector = _payment_vector()
    with pytest.raises(PaymentCommitmentError):
        verify_payment_receipt(
            _bytes(vector["signedReceipt"]), _bytes(vector["rawTransaction"]),
            (_key(vector),), commitment=commitment, **_payment_terms(vector),
        )


def test_payment_sender_and_ledger_accounts_are_independently_bound():
    vector = _payment_vector()
    terms = _payment_terms(vector)
    signed, raw, key = _bytes(vector["signedReceipt"]), _bytes(vector["rawTransaction"]), _key(vector)
    assert terms["payer"] != terms["ledger_payer"]
    verified = verify_payment_receipt(signed, raw, (key,), **{
        **terms, "asset": "0x" + terms["asset"], "pay_to": "0x" + terms["pay_to"],
    })
    assert verified.canonical_bytes == signed
    for changes in ({"payer": terms["ledger_payer"]}, {"ledger_payer": terms["payer"]}, {"payer": "00" * 32}):
        with pytest.raises(PlatformSdkError):
            verify_payment_receipt(signed, raw, (key,), **{**terms, **changes})
