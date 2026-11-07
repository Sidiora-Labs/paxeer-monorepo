from __future__ import annotations

import ast
import json
import sys
from dataclasses import FrozenInstanceError, replace
from hashlib import sha256
from pathlib import Path

import pytest
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
from cryptography.hazmat.primitives.serialization import Encoding, PublicFormat

from layerx_sdk.production import SdkErrorCode
from layerx_sdk.account_derivation import keccak256
from layerx_sdk.evm_transaction import decode_signed_transaction
from layerx_sdk.verifier import (
    ReceiptKey,
    SequencerReceiptVerificationError,
    verify_sequencer_receipt,
)
from layerx_sdk.x402_rpc import _transaction_bytes, verify_rpc_payment
from layerx_sdk.x402 import LAYERX_LEDGER_ADDRESS, PaymentBindingError, verify_payment_receipt

ROOT = Path(__file__).resolve().parents[5]
VECTORS = json.loads(
    (ROOT / "sdk/platform/conformance/fixtures/sequencer-receipt-v1.json").read_text()
)["cases"]


def _payment_vector():
    settlement = json.loads(
        (ROOT / "daemon/committed-fixture/vectors/vector-v4-committed.json").read_text()
    )["settlement"]
    governed = json.loads(
        (ROOT / "sdk/platform/conformance/fixtures/vector-v4-committed.json").read_text()
    )["positive"]
    receipt = settlement["receipt"]
    assert receipt["txHash"] == governed["receiptJson"]["txHash"]
    assert receipt["receipt"] == governed["signedReceipt"]
    assert settlement["raw_transaction"] == governed["rawTransaction"]
    assert receipt["keyId"] == governed["pinnedKey"]["keyId"]
    assert receipt["signingKey"] == governed["pinnedKey"]["publicKey"]
    return {
        "rawTransaction": settlement["raw_transaction"],
        "signedReceipt": receipt["receipt"],
        "receiptJson": receipt,
        "receiptDigest": governed["receiptDigest"],
        "pinnedKey": governed["pinnedKey"],
    }


def _payment_terms(vector):
    receipt = vector["receiptJson"]
    topic = "0x" + keccak256(
        b"PaymentExact(bytes32,bytes32,bytes32,bytes32,uint128)"
    ).hex()
    logs = [log for log in receipt["logs"] if log["topics"][0] == topic]
    assert len(logs) == 1
    log = logs[0]
    data = _bytes(log["data"])
    return {
        "amount": str(int.from_bytes(data[32:], "big")),
        "asset": data[:32].hex(),
        "pay_to": _bytes(log["topics"][3]).hex(),
        "payer": _bytes(log["topics"][2]).hex(),
    }


def test_payment_recording_binds_the_production_ledger_transaction():
    vector = _payment_vector()
    raw = _transaction_bytes(vector["rawTransaction"])
    transaction, _ = decode_signed_transaction(raw)
    terms = _payment_terms(vector)
    assert transaction.chain_id == 125
    assert transaction.to == LAYERX_LEDGER_ADDRESS
    assert transaction.value == 0
    assert transaction.data[4:36].hex() == terms["payer"]
    assert transaction.data[36:68].hex() == terms["pay_to"]
    assert transaction.data[68:100].hex() == terms["asset"]
    assert int.from_bytes(transaction.data[100:132], "big") == int(terms["amount"])
    assert "0x" + keccak256(raw).hex() == vector["receiptJson"]["txHash"]
    verified = verify_payment_receipt(
        _bytes(vector["signedReceipt"]), raw, [_key(vector)], **terms
    )
    assert verified.receipt_digest == _bytes(vector["receiptDigest"])


def test_committed_recording_keeps_its_governed_pin_and_wrong_chain_refusal():
    archived = json.loads(
        (Path(__file__).parent / "fixtures/vector-v4-chain713714.json").read_text()
    )
    recording = archived["settlement"]
    governed = archived["positive"]
    receipt = recording["receipt"]
    assert receipt["txHash"] == governed["receiptJson"]["txHash"]
    assert receipt["receipt"] == governed["signedReceipt"]
    assert recording["raw_transaction"] == governed["rawTransaction"]
    raw = _transaction_bytes(recording["raw_transaction"])
    signed = _transaction_bytes(receipt["receipt"])
    key = _key(governed)
    verified = verify_sequencer_receipt(signed, raw, [key])
    assert verified.height == key.first_height == key.last_height == 10
    assert verified.receipt_digest == _bytes(governed["receiptDigest"])
    assert verified.receipt.context_hash == key.key_id
    assert receipt["signingKey"] == "0x" + key.public_key.hex()
    transaction, _ = decode_signed_transaction(raw)
    assert transaction.chain_id == 713714
    assert verified.receipt.operation == 0
    assert transaction.to == LAYERX_LEDGER_ADDRESS
    assert transaction.value == 0
    payment_log = receipt["logs"][1]
    data = _bytes(payment_log["data"])
    terms = dict(
        amount=str(int.from_bytes(data[32:], "big")),
        asset=data[:32].hex(),
        pay_to=_bytes(payment_log["topics"][3]).hex(),
        payer=_bytes(payment_log["topics"][2]).hex(),
    )
    with pytest.raises(PaymentBindingError) as raised:
        verify_payment_receipt(signed, raw, [key], **terms)
    assert raised.value.code == SdkErrorCode.VERIFICATION_FAILURE


def test_package_root_exports_sequencer_receipt_api():
    import layerx_sdk
    from layerx_sdk import (
        ReceiptKey,
        SequencerReceiptVerification,
        SequencerReceiptVerificationError,
        verifier,
        verify_sequencer_receipt,
    )

    stub = Path(layerx_sdk.__file__).with_suffix(".pyi")
    declarations = ast.parse(stub.read_text())
    aliases = {
        alias.asname
        for node in ast.walk(declarations)
        if isinstance(node, ast.ImportFrom)
        for alias in node.names
    }
    exports = {
        "ReceiptKey": ReceiptKey,
        "SequencerReceiptVerification": SequencerReceiptVerification,
        "SequencerReceiptVerificationError": SequencerReceiptVerificationError,
        "verify_sequencer_receipt": verify_sequencer_receipt,
    }
    assert exports.keys() <= aliases
    for name, exported in exports.items():
        assert exported is getattr(verifier, name)
        assert name in layerx_sdk.__all__


def _bytes(value: str) -> bytes:
    return bytes.fromhex(value.removeprefix("0x"))


def _key(vector: dict) -> ReceiptKey:
    key = vector["pinnedKey"]
    return ReceiptKey(
        _bytes(key["keyId"]),
        _bytes(key["publicKey"]),
        key["firstHeight"],
        key["lastHeight"],
    )


def test_verify_sequencer_receipt_without_cryptography_names_the_extra(monkeypatch):
    for module in (
        "cryptography",
        "cryptography.exceptions",
        "cryptography.hazmat.primitives.asymmetric.ed25519",
    ):
        monkeypatch.setitem(sys.modules, module, None)
    with pytest.raises(ImportError) as raised:
        verify_sequencer_receipt(b"", b"", [])
    assert str(raised.value) == (
        "sequencer receipt verification needs the 'cryptography' package: pip install 'layerx-sdk[derivation]'"
    )
    assert isinstance(raised.value.__cause__, ImportError)


def test_verify_sequencer_receipt_pinned_key():
    for vector in VECTORS:
        if vector["vector"] not in ("V1", "V2"):
            continue
        signed = _bytes(vector["signedReceipt"])
        raw = _bytes(vector["rawTransaction"])
        verified = verify_sequencer_receipt(signed, raw, [_key(vector)])
        assert verified.level == "sequencer-signed"
        assert verified.height == int(vector["receiptJson"]["height"], 16)
        assert verified.canonical_bytes == signed
        assert verified.receipt_digest == _bytes(vector["receiptDigest"])
        assert verified.receipt.activity_id == _bytes(vector["receiptJson"]["txHash"])

    vector = VECTORS[0]
    signed = _bytes(vector["signedReceipt"])
    raw = _bytes(vector["rawTransaction"])
    key = _key(vector)
    assert (
        verify_sequencer_receipt(
            signed, raw, [replace(key, first_height=1000, last_height=1000)]
        ).height
        == 1000
    )
    with pytest.raises(FrozenInstanceError):
        key.first_height = 0

    refusals = [
        (signed, raw, [], "unpinned_key"),
        (signed, raw, [replace(key, key_id=b"\x2a" * 32)], "unpinned_key"),
        (signed, raw, [replace(key, first_height=1001)], "key_out_of_range"),
        (signed, raw, [replace(key, last_height=999)], "key_out_of_range"),
        (signed, raw, [replace(key, first_height=-1)], "key_out_of_range"),
        (signed, raw, [replace(key, last_height=1 << 64)], "key_out_of_range"),
        (
            signed,
            raw,
            [replace(key, first_height=1001, last_height=999)],
            "key_out_of_range",
        ),
        (signed, raw[:-1] + bytes([raw[-1] ^ 1]), [key], "hash_mismatch"),
        (signed[:-1] + bytes([signed[-1] ^ 1]), raw, [key], "bad_signature"),
        (signed, raw, [replace(key, public_key=b"\x2a" * 32)], "bad_signature"),
        (signed, raw, [replace(key, public_key=b"\x2a" * 31)], "bad_signature"),
        (signed[:20], raw, [key], "bad_signature"),
        (signed + b"\0", raw, [key], "bad_signature"),
    ]
    for canonical, transaction, keys, check in refusals:
        with pytest.raises(SequencerReceiptVerificationError) as raised:
            verify_sequencer_receipt(canonical, transaction, keys)
        assert raised.value.check == check
        assert raised.value.code == SdkErrorCode.VERIFICATION_FAILURE

    assert (
        verify_sequencer_receipt(
            signed,
            raw,
            [replace(key, public_key=b"\x2a" * 32), key],
        ).canonical_bytes
        == signed
    )
    for field_offset, replacement in ((0, b"\0\x03"), (4, b"\0\x03")):
        changed = signed[:field_offset] + replacement + signed[field_offset + 2 :]
        with pytest.raises(SequencerReceiptVerificationError) as raised:
            verify_sequencer_receipt(changed, raw, [key])
        assert raised.value.check == "bad_signature"


def test_verify_rpc_payment_uses_receipt_keys():
    vector = _payment_vector()
    raw = _bytes(vector["rawTransaction"])
    fixture_receipt = verify_sequencer_receipt(
        _bytes(vector["signedReceipt"]), raw, [_key(vector)]
    ).receipt
    payment_secret = Ed25519PrivateKey.from_private_bytes(b"\x31" * 32)
    key = ReceiptKey(
        b"k" * 32,
        payment_secret.public_key().public_bytes(Encoding.Raw, PublicFormat.Raw),
        int(vector["receiptJson"]["height"], 16),
        int(vector["receiptJson"]["height"], 16),
    )
    canonical = bytearray(_bytes(vector["signedReceipt"]))
    canonical[-109:-77] = key.key_id
    digest = sha256(b"LXP/v1/receipt\0" + bytes(canonical[:-69]) + b"\0").digest()
    canonical[-64:] = payment_secret.sign(digest)
    signed = bytes(canonical)
    receipt = verify_sequencer_receipt(signed, raw, [key]).receipt
    result = {
        **vector["receiptJson"],
        "receipt": "0x" + signed.hex(),
        "keyId": "0x" + key.key_id.hex(),
    }
    payment_terms = _payment_terms(vector)
    terms = {
        "raw_transaction": raw,
        **{name: payment_terms[name] for name in ("amount", "asset", "pay_to")},
    }

    def verify(
        row, keys=(key,), payer="0x" + payment_terms["payer"], **changes
    ):
        return verify_rpc_payment(
            row,
            "0x" + receipt.activity_id.hex(),
            payer,
            receipt_keys=keys,
            **{**terms, **changes},
        )

    payment = verify(result)
    assert payment == {
        "txHash": vector["receiptJson"]["txHash"],
        "rung": vector["receiptJson"]["rung"],
        "height": int(vector["receiptJson"]["height"], 16),
        "keyId": "0x" + key.key_id.hex(),
    }
    verified = verify_sequencer_receipt(signed, raw, [key])
    assert verified.receipt == receipt
    assert verified.canonical_bytes == signed
    assert verified.receipt_digest == digest
    assert verified.receipt.module_id == fixture_receipt.module_id
    assert verified.receipt.operation == fixture_receipt.operation
    assert verify({**result, "returnData": "0x"}) == payment
    with pytest.raises(SequencerReceiptVerificationError) as raised:
        verify(result, keys=())
    assert raised.value.check == "unpinned_key"

    secret = Ed25519PrivateKey.from_private_bytes(b"\x2a" * 32)
    public_key = secret.public_key().public_bytes(Encoding.Raw, PublicFormat.Raw)
    unpinned = bytearray(signed)
    unpinned[-109:-77] = b"\x2a" * 32
    unpinned[-64:] = secret.sign(
        sha256(b"LXP/v1/receipt\0" + bytes(unpinned[:-69]) + b"\0").digest()
    )
    unpinned_result = {
        **result,
        "receipt": "0x" + bytes(unpinned).hex(),
        "keyId": "0x" + "2a" * 32,
    }
    with pytest.raises(SequencerReceiptVerificationError) as raised:
        verify(unpinned_result)
    assert raised.value.check == "unpinned_key"
    other_key = ReceiptKey(b"\x2a" * 32, public_key, key.first_height, key.last_height)
    assert verify(unpinned_result, keys=(other_key,)) == {
        **payment, "keyId": "0x" + other_key.key_id.hex()
    }
    assert verify_sequencer_receipt(
        bytes(unpinned), raw, [other_key]
    ).canonical_bytes == bytes(unpinned)

    with pytest.raises(SequencerReceiptVerificationError) as raised:
        verify(result, keys=(replace(key, first_height=payment["height"] + 1),))
    assert raised.value.check == "key_out_of_range"
    with pytest.raises(ValueError, match="^transaction-hash-mismatch$"):
        verify(result, raw_transaction=_bytes(VECTORS[0]["rawTransaction"]))
    for changes in (
        {"amount": str(int(terms["amount"]) + 1)},
        {"asset": "2a" * 32},
        {"pay_to": "2a" * 32},
    ):
        with pytest.raises(ValueError, match="^payment-binding-mismatch$"):
            verify(result, **changes)
    with pytest.raises(ValueError, match="^payment-binding-mismatch$"):
        verify({**result, "returnData": "0x2a"})
    with pytest.raises(ValueError, match="^payment-binding-mismatch$"):
        verify(result, payer="0x" + "2a" * 32)
    altered_root = signed.replace(receipt.resulting_state_root, b"\x2a" * 32, 1)
    assert altered_root != signed
    with pytest.raises(SequencerReceiptVerificationError) as raised:
        verify({**result, "receipt": "0x" + altered_root.hex()})
    assert raised.value.check == "bad_signature"
    assert verify(
        {"txHash": result["txHash"], "nonce": result["nonce"], "rung": "submitted"}
    ) == {
        "txHash": result["txHash"],
        "rung": "pending",
        "height": None,
        "keyId": None,
    }


def test_rpc_transaction_hash_requires_prefix_and_32_bytes():
    transaction_hash = "ab" * 32
    assert _transaction_bytes("0x" + transaction_hash, 32) == bytes.fromhex(
        transaction_hash
    )
    for invalid_hash in (
        None,
        transaction_hash,
        "0x" + "ab" * 31,
        "0x" + "ab" * 33,
        "0x" + "gg" * 32,
    ):
        with pytest.raises(ValueError, match="^invalid-rpc-hex$"):
            _transaction_bytes(invalid_hash, 32)
