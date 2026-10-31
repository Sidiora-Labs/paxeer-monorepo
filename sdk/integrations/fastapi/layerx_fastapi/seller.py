from __future__ import annotations

from dataclasses import dataclass, field
from re import fullmatch
from typing import Awaitable, Callable, Generic, Literal, Mapping, Protocol, TypeVar

from layerx_sdk import LocalSignatureVerifier
from layerx_sdk.px import PxReceiptKeyEntry
from layerx_sdk.verifier import SequencerReceiptVerification

from .protocol import (
    PAYMENT_REQUIRED_HEADER,
    PAYMENT_RESPONSE_HEADER,
    JsonValue,
    LayerXReceiptEvidence,
    MiddlewareError,
    MiddlewareErrorCode,
    PaymentPayload,
    PaymentRequired,
    PaymentRequirements,
    SettlementResponse,
    canonical_json,
    constant_time_equal,
    decode_base64,
    decode_payment_payload_header,
    encode_base64,
    encode_payment_required_header,
    encode_settlement_header,
    match_requirements,
    parse_hex32,
    parse_layerx_evidence,
    parse_payment_required,
    payment_idempotency_key,
    sha256_digest,
    verify_payment_receipt,
)

T = TypeVar("T")


@dataclass(frozen=True)
class SellerSettlementRequest:
    principal: str
    payload: PaymentPayload
    requirements: PaymentRequirements
    idempotency_key: str
    request_digest: str


@dataclass(frozen=True)
class SettlementPending:
    kind: Literal["pending"] = "pending"


@dataclass(frozen=True)
class SettlementRefused:
    reason: str
    kind: Literal["refused"] = "refused"


@dataclass(frozen=True)
class SettlementSettled:
    raw_transaction: bytes
    signed_receipt: bytes
    receipt_key: PxReceiptKeyEntry
    kind: Literal["settled"] = "settled"


SellerSettlementOutcome = SettlementPending | SettlementRefused | SettlementSettled


class SellerPaymentAuthority(Protocol):
    async def settle(self, request: SellerSettlementRequest) -> SellerSettlementOutcome: ...


AuthorizedBatchResolver = tuple[PxReceiptKeyEntry, ...]
StaticAuthorizedBatches = tuple[PxReceiptKeyEntry, ...]


@dataclass(frozen=True)
class ProposedFulfillment:
    idempotency_key: str
    request_digest: str
    raw_transaction: bytes
    signed_receipt: bytes
    receipt_key: PxReceiptKeyEntry


@dataclass(frozen=True)
class StoredFulfillment(Generic[T]):
    idempotency_key: str
    request_digest: str
    raw_transaction: bytes
    signed_receipt: bytes
    receipt_key: PxReceiptKeyEntry
    resource: T


class FulfillmentRepository(Protocol[T]):
    async def fulfill(
        self,
        proposed: ProposedFulfillment,
        release: Callable[[], Awaitable[T]],
    ) -> StoredFulfillment[T]: ...


@dataclass(frozen=True)
class PaymentRequiredDecision:
    body: PaymentRequired
    headers: Mapping[str, str]
    status: Literal[402] = 402
    kind: Literal["payment-required"] = "payment-required"


@dataclass(frozen=True)
class PendingDecision:
    status: Literal[202] = 202
    kind: Literal["pending"] = "pending"
    headers: Mapping[str, str] = field(default_factory=dict)


@dataclass(frozen=True)
class RefusedDecision:
    settlement: SettlementResponse
    headers: Mapping[str, str]
    status: Literal[402] = 402
    kind: Literal["refused"] = "refused"


@dataclass(frozen=True)
class ReleasedDecision(Generic[T]):
    settlement: SettlementResponse
    headers: Mapping[str, str]
    verification: SequencerReceiptVerification
    resource: T
    status: Literal[200] = 200
    kind: Literal["released"] = "released"


SellerDecision = PaymentRequiredDecision | PendingDecision | RefusedDecision | ReleasedDecision[T]


class ReceiptPayloadAuthority:
    __slots__ = ("_receipt_keys",)

    def __init__(self, receipt_keys: tuple[PxReceiptKeyEntry, ...]) -> None:
        if not isinstance(receipt_keys, tuple) or not receipt_keys:
            raise MiddlewareError(MiddlewareErrorCode.VERIFICATION_FAILURE)
        self._receipt_keys = receipt_keys

    async def settle(self, request: SellerSettlementRequest) -> SellerSettlementOutcome:
        evidence = parse_layerx_evidence(request.payload.payload)
        pinned = next((key for key in self._receipt_keys if key == evidence.receipt_key), None)
        if pinned is None:
            raise MiddlewareError(MiddlewareErrorCode.VERIFICATION_FAILURE)
        code = MiddlewareErrorCode.INVALID_PAYMENT_PAYLOAD
        raw_transaction = decode_base64(evidence.raw_transaction, code)
        signed_receipt = decode_base64(evidence.signed_receipt, code)
        verified = verify_payment_receipt(
            signed_receipt, raw_transaction, (pinned,), request.requirements
        )
        if not constant_time_equal(parse_hex32(evidence.receipt_digest), verified.receipt_digest):
            raise MiddlewareError(MiddlewareErrorCode.VERIFICATION_FAILURE)
        return SettlementSettled(
            raw_transaction=raw_transaction,
            signed_receipt=signed_receipt,
            receipt_key=pinned,
        )


class SellerMiddleware(Generic[T]):
    __slots__ = ("_required", "_authority", "_fulfillments", "_signatures")

    def __init__(
        self,
        payment_required: PaymentRequired,
        authority: SellerPaymentAuthority,
        fulfillments: FulfillmentRepository[T],
        signatures: LocalSignatureVerifier,
    ) -> None:
        self._required = parse_payment_required(payment_required.to_wire())
        self._authority = authority
        self._fulfillments = fulfillments
        self._signatures = signatures

    @property
    def required(self) -> PaymentRequired:
        return self._required

    def payment_required(self) -> PaymentRequiredDecision:
        return PaymentRequiredDecision(
            body=self._required,
            headers={PAYMENT_REQUIRED_HEADER: encode_payment_required_header(self._required)},
        )

    async def handle(
        self,
        principal: str,
        payment_header: str | None,
        release: Callable[[], Awaitable[T]],
    ) -> SellerDecision[T]:
        if payment_header is None:
            return self.payment_required()
        if not 0 < len(principal) <= 512 or "\0" in principal:
            raise MiddlewareError(MiddlewareErrorCode.INVALID_PAYMENT_PAYLOAD)
        payload = decode_payment_payload_header(payment_header)
        requirements = match_requirements(self._required, payload)
        canonical = canonical_json(payload.to_wire())
        request_digest_bytes = sha256_digest(canonical.encode("utf-8"))
        request_digest = request_digest_bytes.hex()
        idempotency_key = payment_idempotency_key(principal, request_digest_bytes)
        outcome = await self._authority.settle(
            SellerSettlementRequest(
                principal=principal,
                payload=payload,
                requirements=requirements,
                idempotency_key=idempotency_key,
                request_digest=request_digest,
            )
        )
        if isinstance(outcome, SettlementPending):
            return PendingDecision()
        if isinstance(outcome, SettlementRefused):
            return refusal_decision(requirements, outcome.reason)
        proposed = ProposedFulfillment(
            idempotency_key=idempotency_key,
            request_digest=request_digest,
            raw_transaction=outcome.raw_transaction,
            signed_receipt=outcome.signed_receipt,
            receipt_key=outcome.receipt_key,
        )
        verification = verify_payment_receipt(
            proposed.signed_receipt,
            proposed.raw_transaction,
            (proposed.receipt_key,),
            requirements,
        )
        stored = await self._fulfillments.fulfill(proposed, release)
        if (
            stored.idempotency_key != idempotency_key
            or stored.request_digest != request_digest
            or stored.receipt_key != proposed.receipt_key
        ):
            raise MiddlewareError(MiddlewareErrorCode.FULFILLMENT_CONFLICT)
        stored_verification = verify_payment_receipt(
            stored.signed_receipt,
            stored.raw_transaction,
            (proposed.receipt_key,),
            requirements,
        )
        if not constant_time_equal(verification.receipt_digest, stored_verification.receipt_digest):
            raise MiddlewareError(MiddlewareErrorCode.FULFILLMENT_CONFLICT)
        receipt_digest = stored_verification.receipt_digest.hex()
        evidence = LayerXReceiptEvidence(
            raw_transaction=encode_base64(stored.raw_transaction),
            signed_receipt=encode_base64(stored.signed_receipt),
            receipt_digest=receipt_digest,
            receipt_key=stored.receipt_key,
        )
        settlement = SettlementResponse(
            success=True,
            payer=stored_verification.receipt.from_account.hex(),
            transaction="0x" + stored_verification.receipt.activity_id.hex(),
            network=requirements.network,
            amount=requirements.amount,
            extensions={"layerx": layerx_evidence_wire(evidence)},
            has_extensions=True,
        )
        return ReleasedDecision(
            settlement=settlement,
            headers={PAYMENT_RESPONSE_HEADER: encode_settlement_header(settlement)},
            verification=stored_verification,
            resource=stored.resource,
        )


def layerx_evidence_wire(evidence: LayerXReceiptEvidence) -> JsonValue:
    return {
        "rawTransaction": evidence.raw_transaction,
        "signedReceipt": evidence.signed_receipt,
        "receiptKey": {
            "keyId": evidence.receipt_key.key_id,
            "publicKey": evidence.receipt_key.public_key,
            "firstHeight": evidence.receipt_key.first_height,
            "lastHeight": evidence.receipt_key.last_height,
        },
        "receiptDigest": evidence.receipt_digest,
        "verificationLevel": evidence.verification_level,
    }


def refusal_decision(requirements: PaymentRequirements, reason: str) -> RefusedDecision:
    safe_reason = reason if fullmatch(r"[a-z][a-z0-9_]{0,63}", reason) is not None else "payment_refused"
    settlement = SettlementResponse(
        success=False,
        error_reason=safe_reason,
        transaction="",
        network=requirements.network,
    )
    return RefusedDecision(
        settlement=settlement,
        headers={PAYMENT_RESPONSE_HEADER: encode_settlement_header(settlement)},
    )
