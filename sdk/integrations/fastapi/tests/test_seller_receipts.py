from __future__ import annotations

import asyncio
import json
import runpy
from dataclasses import asdict, replace
from hashlib import sha256
from pathlib import Path
from typing import Awaitable, Callable

import pytest
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
from cryptography.hazmat.primitives.serialization import Encoding, PublicFormat
from fastapi import HTTPException, Request
from fastapi.testclient import TestClient
from layerx_sdk.account_derivation import keccak256
from layerx_sdk.evm_transaction import decode_signed
from layerx_sdk.px import PxReceiptKeyEntry
from layerx_sdk.verifier import ReceiptKey, verify_sequencer_receipt

from layerx_fastapi import LayerXResource
from layerx_fastapi.config import DeclaredConfig, read_declared_config
from layerx_fastapi.protocol import (
    PAYMENT_REQUIRED_HEADER,
    PAYMENT_RESPONSE_HEADER,
    PAYMENT_SIGNATURE_HEADER,
    LayerXReceiptEvidence,
    MiddlewareError,
    MiddlewareErrorCode,
    PaymentPayload,
    canonical_json,
    decode_payment_required_header,
    decode_settlement_header,
    encode_base64,
    encode_payment_payload_header,
    parse_layerx_evidence,
    verify_payment_receipt,
)
from layerx_fastapi.seller import (
    ProposedFulfillment,
    ReceiptPayloadAuthority,
    ReleasedDecision,
    SellerMiddleware,
    SellerSettlementRequest,
    SettlementSettled,
    StoredFulfillment,
    idempotency_key,
    layerx_evidence_wire,
)
from layerx_fastapi.signatures import LayerXSignatureVerifier
from test_package_surface import mount_example_offer

ROOT = Path(__file__).resolve().parents[4]


class FileFulfillments:
    def __init__(self, directory: Path) -> None:
        self.directory = directory

    async def fulfill(
        self, proposed: ProposedFulfillment, release: Callable[[], Awaitable[bytes]]
    ) -> StoredFulfillment[bytes]:
        path = self.directory / (proposed.idempotency_key + ".json")
        if not path.exists():
            resource = await release()
            with path.open("x", encoding="utf-8") as handle:
                json.dump(
                    {
                        "idempotency_key": proposed.idempotency_key,
                        "request_digest": proposed.request_digest,
                        "raw_transaction": proposed.raw_transaction.hex(),
                        "signed_receipt": proposed.signed_receipt.hex(),
                        "receipt_key": asdict(proposed.receipt_key),
                        "resource": resource.hex(),
                    },
                    handle,
                )
        stored = json.loads(path.read_text(encoding="utf-8"))
        return StoredFulfillment(
            idempotency_key=stored["idempotency_key"],
            request_digest=stored["request_digest"],
            raw_transaction=bytes.fromhex(stored["raw_transaction"]),
            signed_receipt=bytes.fromhex(stored["signed_receipt"]),
            receipt_key=PxReceiptKeyEntry(**stored["receipt_key"]),
            resource=bytes.fromhex(stored["resource"]),
        )


def _fixture() -> tuple[bytes, bytes, PxReceiptKeyEntry, str, DeclaredConfig]:
    document = json.loads(
        (ROOT / "sdk/platform/conformance/fixtures/sequencer-receipt-v1.json").read_text()
    )
    assert document["provenance"]["generator"] == "rpc/sequencer_receipt_vectors_test.go"
    assert document["chainId"] == 125
    vectors = document["cases"]
    vector = next(row for row in vectors if row["id"] == "V1-layerxledger-pay-exact")
    transaction, _ = decode_signed(bytes.fromhex(vector["rawTransaction"][2:]))
    raw = bytes.fromhex(vector["rawTransaction"][2:])
    signed = bytes.fromhex(vector["signedReceipt"][2:])
    pin = vector["pinnedKey"]
    key = PxReceiptKeyEntry(
        pin["keyId"][2:], pin["publicKey"][2:], pin["firstHeight"], pin["lastHeight"]
    )
    receipt = verify_sequencer_receipt(
        signed,
        raw,
        (
            ReceiptKey(
                bytes.fromhex(key.key_id), bytes.fromhex(key.public_key),
                key.first_height, key.last_height,
            ),
        ),
    ).receipt
    config = read_declared_config(
        {
            "LAYERX_PRINCIPAL": "merchant",
            "LAYERX_PROTECTED_PATH": "/paid",
            "LAYERX_RESOURCE_URL": "https://merchant.example/paid",
            "LAYERX_X402_SCHEME": "exact",
            "LAYERX_X402_NETWORK": "eip155:125",
            "LAYERX_PRICE": str(int.from_bytes(transaction.data[100:132], "big")),
            "LAYERX_ASSET": transaction.data[68:100].hex(),
            "LAYERX_PAY_TO": transaction.data[36:68].hex(),
            "LAYERX_PAYMENT_TIMEOUT_SECONDS": "60",
            "LAYERX_RECEIPT_KEYS_JSON": json.dumps({"keys": [{
                "keyId": "0x" + key.key_id,
                "publicKey": "0x" + key.public_key,
                "firstHeight": hex(key.first_height),
                "lastHeight": hex(key.last_height),
            }]}),
            "LAYERX_WEBHOOK_PATH": "/webhook",
            "LAYERX_WEBHOOK_PUBLIC_KEYS_JSON": json.dumps(
                {"webhook": encode_base64(bytes.fromhex(key.public_key))}
            ),
        }
    )
    requirements = replace(
        config.requirements,
        extra={"layerx": {
            "commitment": "receipted",
            "purposeHash": transaction.data[164:196].hex(),
            "payer": transaction.data[4:36].hex(),
        }},
        has_extra=True,
    )
    config = replace(
        config,
        requirements=requirements,
        payment_required=replace(config.payment_required, accepts=(requirements,)),
    )
    assert receipt.activity_id == keccak256(raw)
    return raw, signed, key, vector["receiptDigest"][2:], config


def _payload(
    config: DeclaredConfig, raw: bytes, signed: bytes, key: PxReceiptKeyEntry
) -> PaymentPayload:
    return PaymentPayload(
        accepted=config.requirements,
        payload=layerx_evidence_wire(LayerXReceiptEvidence(
            raw_transaction="0x" + raw.hex(),
            signed_receipt="0x" + signed.hex(),
            receipt_digest="0x" + keccak256(signed).hex(),
            receipt_key=key,
        )),
    )


def test_seller_settles_signed_sequencer_receipt(tmp_path: Path) -> None:
    raw, signed, key, digest, config = _fixture()
    payload = _payload(config, raw, signed, key)
    authority = ReceiptPayloadAuthority(config.receipt_keys)
    request = SellerSettlementRequest(
        principal=config.principal,
        payload=payload,
        requirements=config.requirements,
        idempotency_key="payment",
        request_digest="request",
    )
    settled = asyncio.run(authority.settle(request))
    assert isinstance(settled, SettlementSettled)
    assert settled.raw_transaction == raw
    assert settled.signed_receipt == signed
    assert settled.receipt_key == key
    assert not hasattr(settled, "authorized_batch")
    verified = verify_payment_receipt(
        settled.signed_receipt, settled.raw_transaction, (settled.receipt_key,), config.requirements
    )
    assert verified.receipt.activity_id == keccak256(raw)
    assert verified.receipt_digest.hex() == digest
    assert verified.height == 1001

    releases: list[bytes] = []

    async def release() -> bytes:
        resource = b"paid resource"
        releases.append(resource)
        return resource

    seller = SellerMiddleware(
        config.payment_required, authority, FileFulfillments(tmp_path), LayerXSignatureVerifier()
    )
    header = encode_payment_payload_header(payload)
    decision = asyncio.run(seller.handle(config.principal, header, release))
    assert isinstance(decision, ReleasedDecision)
    assert decision.resource == b"paid resource"
    assert decision.verification == verified
    assert decision.settlement.success
    assert decision.settlement.transaction == "0x" + keccak256(raw).hex()
    assert decision.settlement.amount == config.requirements.amount
    assert decode_settlement_header(decision.headers[PAYMENT_RESPONSE_HEADER]) == decision.settlement
    evidence = parse_layerx_evidence(decision.settlement.extensions["layerx"])
    assert evidence == parse_layerx_evidence(payload.payload)

    reopened = SellerMiddleware(
        config.payment_required, authority, FileFulfillments(tmp_path), LayerXSignatureVerifier()
    )
    assert asyncio.run(reopened.handle(config.principal, header, release)) == decision
    assert releases == [b"paid resource"]
    records = list(tmp_path.glob("*.json"))
    assert len(records) == 1
    stored = json.loads(records[0].read_text())
    assert stored["raw_transaction"] == raw.hex()
    assert stored["signed_receipt"] == signed.hex()
    assert stored["receipt_key"] == asdict(key)
    assert "authorized_batch" not in stored

    for changes in (
        {"rawTransaction": "0x" + (raw + b"\0").hex()},
        {"signedReceipt": "0x" + (signed[:-1] + bytes([signed[-1] ^ 1])).hex()},
        {"receiptDigest": "0x" + "00" * 32},
        {"receiptKey": {**payload.payload["receiptKey"], "lastHeight": hex(key.last_height + 1)}},
    ):
        invalid = replace(payload, payload={**payload.payload, **changes})
        with pytest.raises(MiddlewareError) as raised:
            asyncio.run(seller.handle(config.principal, encode_payment_payload_header(invalid), release))
        assert raised.value.code is MiddlewareErrorCode.VERIFICATION_FAILURE
    assert releases == [b"paid resource"]
    assert list(tmp_path.glob("*.json")) == records

    stored["receipt_key"]["last_height"] += 1
    records[0].write_text(json.dumps(stored))
    with pytest.raises(MiddlewareError) as raised:
        asyncio.run(reopened.handle(config.principal, header, release))
    assert raised.value.code is MiddlewareErrorCode.FULFILLMENT_CONFLICT
    assert releases == [b"paid resource"]


def test_seller_refuses_a_settled_receipt_for_a_new_request(tmp_path: Path) -> None:
    raw, signed, key, digest, config = _fixture()
    first = _payload(config, raw, signed, key)
    second = replace(first, resource=replace(config.payment_required.resource, description="second"))
    first_header = encode_payment_payload_header(first)
    second_header = encode_payment_payload_header(second)
    assert first_header != second_header
    assert parse_layerx_evidence(first.payload) == parse_layerx_evidence(second.payload)
    releases: list[bytes] = []

    async def release() -> bytes:
        releases.append(b"paid resource")
        return releases[-1]

    authority = ReceiptPayloadAuthority(config.receipt_keys)
    seller = SellerMiddleware(
        config.payment_required, authority, FileFulfillments(tmp_path), LayerXSignatureVerifier()
    )
    decision = asyncio.run(seller.handle(config.principal, first_header, release))
    assert isinstance(decision, ReleasedDecision)
    assert decision.resource == b"paid resource"
    assert decision.verification.receipt_digest.hex() == digest
    record = tmp_path / (idempotency_key(config.principal, decision.verification) + ".json")
    original_record = record.read_bytes()

    for active_seller in (
        seller,
        SellerMiddleware(
            config.payment_required, authority, FileFulfillments(tmp_path), LayerXSignatureVerifier()
        ),
    ):
        with pytest.raises(HTTPException) as raised:
            asyncio.run(active_seller.handle(config.principal, second_header, release))
        assert raised.value.status_code == 409
        assert raised.value.detail == "settled_receipt_reuse"
        assert releases == [b"paid resource"]
        assert record.read_bytes() == original_record
        assert asyncio.run(active_seller.handle(config.principal, first_header, release)) == decision
        assert releases == [b"paid resource"]
    assert list(tmp_path.glob("*.json")) == [record]


def test_example_repository_refuses_settled_receipt_reuse(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    raw, signed, key, digest, config = _fixture()
    resource_file = tmp_path / "resource.json"
    resource_file.write_bytes(b'{"paid":true}')
    directory = tmp_path / "fulfillments"
    environment = {
        "LAYERX_PRINCIPAL": config.principal,
        "LAYERX_PROTECTED_PATH": config.protected_path,
        "LAYERX_RESOURCE_URL": config.payment_required.resource.url,
        "LAYERX_X402_SCHEME": config.requirements.scheme,
        "LAYERX_X402_NETWORK": config.requirements.network,
        "LAYERX_PRICE": config.requirements.amount,
        "LAYERX_ASSET": config.requirements.asset,
        "LAYERX_PAY_TO": config.requirements.pay_to,
        "LAYERX_PAYMENT_TIMEOUT_SECONDS": str(config.requirements.max_timeout_seconds),
        "LAYERX_RECEIPT_KEYS_JSON": json.dumps({"keys": [{
            "keyId": "0x" + key.key_id,
            "publicKey": "0x" + key.public_key,
            "firstHeight": hex(key.first_height),
            "lastHeight": hex(key.last_height),
        }]}),
        "LAYERX_WEBHOOK_PATH": config.webhook.path,
        "LAYERX_WEBHOOK_PUBLIC_KEYS_JSON": json.dumps(
            {name: encode_base64(value) for name, value in config.webhook.public_keys.items()}
        ),
        "LAYERX_RESOURCE_FILE": str(resource_file),
        "LAYERX_FULFILLMENT_DIR": str(directory),
    }
    for name, value in environment.items():
        monkeypatch.setenv(name, value)
    example = runpy.run_path(str(ROOT / "sdk/integrations/fastapi/examples/paid_api.py"))
    repository_type = example["FileFulfillmentRepository"]
    first = _payload(config, raw, signed, key)
    second = replace(first, resource=replace(config.payment_required.resource, description="second"))
    first_header = encode_payment_payload_header(first)
    second_header = encode_payment_payload_header(second)
    original_digest = sha256(canonical_json(first.to_wire()).encode("utf-8")).hexdigest()
    assert first_header != second_header
    assert parse_layerx_evidence(first.payload) == parse_layerx_evidence(second.payload)
    releases: list[LayerXResource] = []

    async def release() -> LayerXResource:
        resource = await example["mount"].runtime.resources.release(
            Request({"type": "http", "method": "GET", "path": "/paid", "headers": []})
        )
        releases.append(resource)
        return resource

    seller = SellerMiddleware(
        config.payment_required,
        ReceiptPayloadAuthority(config.receipt_keys),
        repository_type(directory),
        LayerXSignatureVerifier(),
    )
    decision = asyncio.run(seller.handle(config.principal, first_header, release))
    assert isinstance(decision, ReleasedDecision)
    assert decision.resource == LayerXResource(
        content_type="application/json", body=resource_file.read_bytes()
    )
    assert decision.verification.receipt_digest.hex() == digest
    record = directory / (idempotency_key(config.principal, decision.verification) + ".json")
    original_record = record.read_bytes()
    assert json.loads(original_record)["requestDigest"] == original_digest

    for active_seller in (
        seller,
        SellerMiddleware(
            config.payment_required,
            ReceiptPayloadAuthority(config.receipt_keys),
            repository_type(directory),
            LayerXSignatureVerifier(),
        ),
    ):
        with pytest.raises(HTTPException) as raised:
            asyncio.run(active_seller.handle(config.principal, second_header, release))
        assert raised.value.status_code == 409
        assert raised.value.detail == "settled_receipt_reuse"
        assert releases == [decision.resource]
        assert record.read_bytes() == original_record
        assert asyncio.run(active_seller.handle(config.principal, first_header, release)) == decision
        assert releases == [decision.resource]
        assert list(directory.glob("*.json")) == [record]
        assert record.read_bytes() == original_record
        assert json.loads(record.read_bytes())["requestDigest"] == original_digest


def test_verify_payment_receipt_refuses_unpinned_key(tmp_path: Path) -> None:
    raw, signed, key, _, config = _fixture()
    secret = Ed25519PrivateKey.from_private_bytes(b"\x2a" * 32)
    unpinned_key = replace(
        key,
        key_id="2a" * 32,
        public_key=secret.public_key().public_bytes(Encoding.Raw, PublicFormat.Raw).hex(),
    )
    changed = bytearray(signed)
    changed[-109:-77] = bytes.fromhex(unpinned_key.key_id)
    digest = sha256(b"LXP/v1/receipt\0" + bytes(changed[:-69]) + b"\0").digest()
    changed[-64:] = secret.sign(digest)
    unpinned = bytes(changed)
    assert verify_sequencer_receipt(
        unpinned,
        raw,
        (
            ReceiptKey(
                bytes.fromhex(unpinned_key.key_id), bytes.fromhex(unpinned_key.public_key),
                unpinned_key.first_height, unpinned_key.last_height,
            ),
        ),
    ).receipt_digest == digest
    with pytest.raises(MiddlewareError) as raised:
        verify_payment_receipt(unpinned, raw, config.receipt_keys, config.requirements)
    assert raised.value.code is MiddlewareErrorCode.VERIFICATION_FAILURE
    assert raised.value.__cause__.check == "unpinned_key"

    for keys, check in (
        ((), "unpinned_key"),
        ((replace(key, first_height=1002),), "key_out_of_range"),
        ((replace(key, last_height=1000),), "key_out_of_range"),
        ((replace(key, public_key=unpinned_key.public_key),), "bad_signature"),
    ):
        with pytest.raises(MiddlewareError) as raised:
            verify_payment_receipt(signed, raw, keys, config.requirements)
        assert raised.value.code is MiddlewareErrorCode.VERIFICATION_FAILURE
        assert raised.value.__cause__.check == check
    for requirements in (
        replace(config.requirements, amount=str(int(config.requirements.amount) + 1)),
        replace(config.requirements, asset="2a" * 32),
        replace(config.requirements, pay_to="2a" * 32),
        replace(config.requirements, extra={"layerx": {
            **config.requirements.extra["layerx"], "purposeHash": "2a" * 32,
        }}),
        replace(config.requirements, extra={"layerx": {
            **config.requirements.extra["layerx"], "payer": "2a" * 32,
        }}),
    ):
        with pytest.raises(MiddlewareError) as raised:
            verify_payment_receipt(signed, raw, config.receipt_keys, requirements)
        assert raised.value.code is MiddlewareErrorCode.VERIFICATION_FAILURE

    releases: list[bytes] = []

    async def release() -> bytes:
        releases.append(b"protected")
        return releases[-1]

    seller = SellerMiddleware(
        config.payment_required,
        ReceiptPayloadAuthority(config.receipt_keys),
        FileFulfillments(tmp_path),
        LayerXSignatureVerifier(),
    )
    payload = _payload(config, raw, unpinned, unpinned_key)
    with pytest.raises(MiddlewareError) as raised:
        asyncio.run(seller.handle(config.principal, encode_payment_payload_header(payload), release))
    assert raised.value.code is MiddlewareErrorCode.VERIFICATION_FAILURE
    assert releases == []
    assert list(tmp_path.iterdir()) == []


def mounted_environment(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, config: DeclaredConfig, key: PxReceiptKeyEntry
) -> dict[str, str]:
    resource = tmp_path / "resource.json"
    resource.write_bytes(b'{"paid":true}')
    directory = tmp_path / "fulfillments"
    environment = {
        "LAYERX_PRINCIPAL": config.principal,
        "LAYERX_PROTECTED_PATH": config.protected_path,
        "LAYERX_RESOURCE_URL": config.payment_required.resource.url,
        "LAYERX_X402_SCHEME": config.requirements.scheme,
        "LAYERX_X402_NETWORK": config.requirements.network,
        "LAYERX_PRICE": config.requirements.amount,
        "LAYERX_ASSET": config.requirements.asset,
        "LAYERX_PAY_TO": config.requirements.pay_to,
        "LAYERX_PAYMENT_TIMEOUT_SECONDS": str(config.requirements.max_timeout_seconds),
        "LAYERX_RECEIPT_KEYS_JSON": json.dumps({"keys": [{
            "keyId": "0x" + key.key_id,
            "publicKey": "0x" + key.public_key,
            "firstHeight": hex(key.first_height),
            "lastHeight": hex(key.last_height),
        }]}),
        "LAYERX_WEBHOOK_PATH": config.webhook.path,
        "LAYERX_WEBHOOK_PUBLIC_KEYS_JSON": json.dumps({
            name: encode_base64(value) for name, value in config.webhook.public_keys.items()
        }),
        "LAYERX_RESOURCE_FILE": str(resource),
        "LAYERX_FULFILLMENT_DIR": str(directory),
    }
    for name, value in environment.items():
        monkeypatch.setenv(name, value)
    return environment


def test_mounted_seller_accepts_current_producer_evidence_and_replays(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    raw, signed, key, digest, config = _fixture()
    environment = mounted_environment(tmp_path, monkeypatch, config, key)
    resource = Path(environment["LAYERX_RESOURCE_FILE"])
    directory = Path(environment["LAYERX_FULFILLMENT_DIR"])
    example_path = ROOT / "sdk/integrations/fastapi/examples/paid_api.py"
    mounted_purpose = config.requirements.extra["layerx"]["purposeHash"]
    example = mount_example_offer(example_path, environment, mounted_purpose)
    mount = example["mount"]
    payload = _payload(mount.config, raw, signed, key)
    header = encode_payment_payload_header(payload)
    assert payload.accepted.network == "eip155:125"
    assert payload.payload["format"] == "layerx-receipt-proof-v2"
    assert payload.payload["verificationLevel"] == "receipted"
    assert verify_payment_receipt(signed, raw, config.receipt_keys, config.requirements).receipt_digest.hex() == digest

    with TestClient(example["app"]) as client:
        challenge = client.get(config.protected_path)
        assert challenge.status_code == 402
        offer = decode_payment_required_header(challenge.headers[PAYMENT_REQUIRED_HEADER])
        assert offer.to_wire() == challenge.json()
        assert offer.accepts == (mount.config.requirements,)
        assert offer.accepts[0].extra["layerx"]["purposeHash"] == mounted_purpose
        assert not directory.exists()
        for changes in (
            {"rawTransaction": "0x" + (raw + b"\0").hex()},
            {"signedReceipt": "0x" + (signed[:-1] + bytes([signed[-1] ^ 1])).hex()},
            {"receiptDigest": "0x" + "00" * 32},
            {"receiptKey": {**payload.payload["receiptKey"], "lastHeight": hex(key.last_height + 1)}},
        ):
            invalid = replace(payload, payload={**payload.payload, **changes})
            refused = client.get(config.protected_path, headers={
                PAYMENT_SIGNATURE_HEADER: encode_payment_payload_header(invalid)
            })
            assert refused.status_code == 402
            assert refused.json() == {"error": "verification-failure"}
            assert PAYMENT_RESPONSE_HEADER not in refused.headers
            assert not directory.exists()
        paid = client.get(config.protected_path, headers={PAYMENT_SIGNATURE_HEADER: header})
        assert paid.status_code == 200, paid.text
        assert paid.content == b'{"paid":true}'
        settlement = decode_settlement_header(paid.headers[PAYMENT_RESPONSE_HEADER])
        assert settlement.success
        assert settlement.network == config.requirements.network
        assert settlement.amount == config.requirements.amount
        assert settlement.transaction == "0x" + keccak256(raw).hex()
        assert settlement.extensions["layerx"] == payload.payload
        assert paid.headers["layerx-transaction"] == settlement.transaction
        assert paid.headers["layerx-receipt-digest"] == keccak256(signed).hex()
        replay = client.get(config.protected_path, headers={PAYMENT_SIGNATURE_HEADER: header})
        assert replay.status_code == 200
        assert replay.content == paid.content
        assert replay.headers[PAYMENT_RESPONSE_HEADER] == paid.headers[PAYMENT_RESPONSE_HEADER]

    records = list(directory.glob("*.json"))
    assert len(records) == 1
    persisted = records[0].read_bytes()
    resource.write_bytes(b'{"paid":"changed after shutdown"}')
    reopened = mount_example_offer(example_path, environment, mounted_purpose)
    assert reopened["mount"].seller is not mount.seller
    with TestClient(reopened["app"]) as client:
        recovered = client.get(config.protected_path, headers={PAYMENT_SIGNATURE_HEADER: header})
        assert recovered.status_code == 200
        assert recovered.content == paid.content
        assert recovered.headers[PAYMENT_RESPONSE_HEADER] == paid.headers[PAYMENT_RESPONSE_HEADER]
    assert records[0].read_bytes() == persisted
    assert list(directory.glob("*.json")) == records
