//! x402 v2 facilitator role. Verification is a read-only translation and
//! settlement can report success only after the gateway verifies signed
//! sequencer evidence returned by the plane authority.

use std::collections::BTreeMap;

use base64::engine::general_purpose::STANDARD;
use base64::Engine as _;
use layerx_interop_gateway::adapter::AdapterId;
use layerx_interop_gateway::evidence::{SequencerEvidence, SettlementRung};
use layerx_interop_gateway::gateway::{TranslationKind, TranslationRequest, TranslationStatus};
use layerx_interop_gateway::principal::PrincipalId;
use layerx_interop_gateway::trace::{TraceId, Traced};
use layerx_interop_gateway::GatewayCore;
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_proof::receipt::verify_sequencer_signature;
use layerx_proof::sequencer_logs::verify_logs;
use serde::{Deserialize, Serialize};
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};

use crate::model::{
    PaymentPayload, PaymentRequirements, SettlementResponse, X402Error, X402_VERSION,
};
use crate::seller::{payment_matches, validate_network, validate_transaction};

const ADAPTER_ID: &str = "x402";
const VERIFY_DIGEST_DOMAIN: &[u8] = b"LayerX/x402/v2/facilitator/verify/digest\0";
const VERIFY_KEY_DOMAIN: &[u8] = b"LayerX/x402/v2/facilitator/verify/key\0";
const SETTLE_DIGEST_DOMAIN: &[u8] = b"LayerX/x402/v2/facilitator/settle/digest\0";
const SETTLE_KEY_DOMAIN: &[u8] = b"LayerX/x402/v2/facilitator/settle/key\0";

/// Exact request body shared by the standard `/verify` and `/settle`
/// facilitator endpoints.
#[derive(Clone, Debug, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields, rename_all = "camelCase")]
pub struct FacilitatorRequest {
    pub x402_version: u8,
    pub payment_payload: PaymentPayload,
    pub payment_requirements: PaymentRequirements,
}

impl FacilitatorRequest {
    pub(crate) fn validate(&self) -> Result<(), X402Error> {
        if self.x402_version != X402_VERSION {
            return Err(X402Error::WrongVersion);
        }
        self.payment_payload.validate()?;
        self.payment_requirements.validate()?;
        validate_network(&self.payment_requirements.network)?;
        self.payment_requirements.layerx_terms()?;
        if self.payment_payload.accepted != self.payment_requirements {
            return Err(X402Error::RequirementsMismatch);
        }
        Ok(())
    }
}

/// Exact standard response from the facilitator's read-only `/verify` API.
#[derive(Clone, Debug, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields, rename_all = "camelCase")]
pub struct VerifyResponse {
    pub is_valid: bool,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub invalid_reason: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub payer: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub extra: Option<Value>,
}

/// One scheme/network capability from the standard `/supported` response.
#[derive(Clone, Debug, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields, rename_all = "camelCase")]
pub struct FacilitatorKind {
    pub x402_version: u8,
    pub scheme: String,
    pub network: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub extra: Option<Value>,
}

/// Exact standard response from the facilitator's `/supported` API.
#[derive(Clone, Debug, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct SupportedResponse {
    pub kinds: Vec<FacilitatorKind>,
    pub extensions: Vec<String>,
    pub signers: BTreeMap<String, Vec<String>>,
}

/// Server-led settlement step. The distinction is committed to both the
/// request digest and gateway identity so escrow's two legitimate settle
/// calls cannot alias one another.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum SettlementStep {
    Single,
    EscrowDeposit,
    EscrowCharge,
}

impl SettlementStep {
    const fn code(self) -> u8 {
        match self {
            Self::Single => 1,
            Self::EscrowDeposit => 2,
            Self::EscrowCharge => 3,
        }
    }
}

/// Typed, digest-bound request handed to the real facilitator plane.
#[derive(Clone, Debug, PartialEq)]
pub struct FacilitatorPaymentRequest {
    pub payment_payload: PaymentPayload,
    pub payment_requirements: PaymentRequirements,
    pub idempotency_key: [u8; 32],
    pub request_digest: [u8; 32],
    pub settlement_step: SettlementStep,
}

/// Stable state-changing identity shared by HTTP, MCP and A2A bindings. The
/// transport is intentionally absent: identical x402 semantics retry under
/// one economic identity regardless of delivery channel.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct SettlementIdentity {
    pub idempotency_key: [u8; 32],
    pub request_digest: [u8; 32],
}

impl SettlementIdentity {
    /// Derives the facilitator's stable economic identity from canonical x402
    /// request semantics, principal, caller identity and settlement step.
    ///
    /// # Errors
    ///
    /// Refuses zero caller identity, invalid x402 values and serialization
    /// failures. No transport label participates in either digest.
    pub fn derive(
        principal: &PrincipalId,
        request: &FacilitatorRequest,
        stable_identity: [u8; 32],
        step: SettlementStep,
    ) -> Result<Self, X402Error> {
        if stable_identity == [0; 32] {
            return Err(X402Error::InvalidPayload);
        }
        request.validate()?;
        let canonical = serde_json::to_vec(request).map_err(|_| X402Error::Encode)?;
        let step_code = [step.code()];
        Ok(Self {
            request_digest: digest(SETTLE_DIGEST_DOMAIN, &[&step_code, &canonical]),
            idempotency_key: digest(
                SETTLE_KEY_DOMAIN,
                &[principal.as_str().as_bytes(), &stable_identity, &step_code],
            ),
        })
    }
}

/// Honest result of a read-only plane verification.
#[derive(Clone, Debug, PartialEq)]
pub enum PlaneVerifyOutcome {
    Valid {
        payer: Option<String>,
        extra: Option<Value>,
    },
    Invalid {
        reason: &'static str,
        payer: Option<String>,
    },
}

/// Honest result of a state-committing plane settlement.
#[derive(Debug)]
pub enum FacilitatorSettlementOutcome {
    Pending {
        transaction: String,
    },
    Refused {
        reason: &'static str,
        payer: Option<String>,
    },
    Executed(SequencerEvidence),
}

/// The only authority boundary used by the facilitator. Verification and
/// settlement are deliberately separate methods, making a state-changing
/// implementation impossible to call through the read-only route by mistake.
pub trait FacilitatorPlane {
    /// Independently configured sequencer authority pins for settlement.
    fn receipt_keys(&self) -> &[SequencerAuthorization];

    /// Validates without committing payment state.
    ///
    /// # Errors
    ///
    /// Returns a typed scheme, policy, or network-observation refusal.
    fn verify(
        &mut self,
        request: &FacilitatorPaymentRequest,
        trace: &TraceId,
    ) -> Result<PlaneVerifyOutcome, X402Error>;

    /// Idempotently commits payment state for one stable request identity.
    ///
    /// # Errors
    ///
    /// Returns a typed scheme, policy, or settlement refusal.
    fn settle(
        &mut self,
        request: FacilitatorPaymentRequest,
        trace: &TraceId,
    ) -> Result<FacilitatorSettlementOutcome, X402Error>;
}

/// A relay result awaiting the facilitator's local evidence checks.
#[derive(Clone, Debug)]
pub struct SequencerFacilitatorPlane {
    evidence: SequencerEvidence,
    receipt_keys: Vec<SequencerAuthorization>,
}

impl SequencerFacilitatorPlane {
    #[must_use]
    pub const fn new(
        evidence: SequencerEvidence,
        receipt_keys: Vec<SequencerAuthorization>,
    ) -> Self {
        Self {
            evidence,
            receipt_keys,
        }
    }
}

impl FacilitatorPlane for SequencerFacilitatorPlane {
    fn receipt_keys(&self) -> &[SequencerAuthorization] {
        &self.receipt_keys
    }

    fn verify(
        &mut self,
        request: &FacilitatorPaymentRequest,
        _trace: &TraceId,
    ) -> Result<PlaneVerifyOutcome, X402Error> {
        let transaction = request
            .payment_payload
            .payload
            .get("transaction")
            .and_then(Value::as_str)
            .ok_or(X402Error::TransactionForm)?;
        validate_transaction(transaction)?;
        Ok(PlaneVerifyOutcome::Valid {
            payer: None,
            extra: None,
        })
    }

    fn settle(
        &mut self,
        request: FacilitatorPaymentRequest,
        trace: &TraceId,
    ) -> Result<FacilitatorSettlementOutcome, X402Error> {
        if let PlaneVerifyOutcome::Invalid { reason, payer } = self.verify(&request, trace)? {
            return Ok(FacilitatorSettlementOutcome::Refused { reason, payer });
        }
        if request.payment_payload.payload["transaction"]
            != format!("0x{}", hex(&self.evidence.transaction_hash))
        {
            return Ok(FacilitatorSettlementOutcome::Refused {
                reason: "transaction_mismatch",
                payer: None,
            });
        }
        if self.evidence.rung == SettlementRung::Submitted {
            return Ok(FacilitatorSettlementOutcome::Pending {
                transaction: format!("0x{}", hex(&self.evidence.transaction_hash)),
            });
        }
        Ok(FacilitatorSettlementOutcome::Executed(
            self.evidence.clone(),
        ))
    }
}

/// Receipt-backed implementation of the x402 v2 facilitator APIs.
#[derive(Clone, Debug)]
pub struct Facilitator {
    supported: SupportedResponse,
}

impl Facilitator {
    /// Creates a facilitator from a closed, bounded support declaration.
    ///
    /// # Errors
    ///
    /// Refuses empty, duplicate, malformed, or oversized declarations.
    pub fn new(supported: SupportedResponse) -> Result<Self, X402Error> {
        validate_supported(&supported)?;
        Ok(Self { supported })
    }

    /// Returns the immutable `/supported` response.
    #[must_use]
    pub fn supported(&self) -> SupportedResponse {
        self.supported.clone()
    }

    /// Implements read-only `/verify`. The gateway record is explicitly
    /// read-only and the plane's settlement method is not reachable here.
    ///
    /// # Errors
    ///
    /// Returns trace-bound wire, support, gateway, or plane failures.
    pub fn verify(
        &self,
        gateway: &mut GatewayCore,
        principal: &PrincipalId,
        request: &FacilitatorRequest,
        plane: &mut impl FacilitatorPlane,
        trace: &TraceId,
        now: u64,
    ) -> Result<VerifyResponse, Traced<X402Error>> {
        let fail = |error| trace.wrap(error);
        request.validate().map_err(fail)?;
        self.require_supported(&request.payment_requirements)
            .map_err(fail)?;
        let canonical = serde_json::to_vec(request).map_err(|_| fail(X402Error::Encode))?;
        let request_digest = digest(VERIFY_DIGEST_DOMAIN, &[&canonical]);
        let idempotency_key = digest(
            VERIFY_KEY_DOMAIN,
            &[principal.as_str().as_bytes(), &request_digest],
        );
        let translation = translation(TranslationKind::ReadOnly, idempotency_key, request_digest)
            .map_err(fail)?;
        let opened = gateway
            .begin_translation(principal, &translation, trace, now)
            .map_err(|error| trace.wrap(X402Error::Gateway(error.into_error())))?;
        if opened == TranslationStatus::Refused {
            return Err(fail(X402Error::PaymentRefused));
        }
        let plane_request = FacilitatorPaymentRequest {
            payment_payload: request.payment_payload.clone(),
            payment_requirements: request.payment_requirements.clone(),
            idempotency_key,
            request_digest,
            settlement_step: SettlementStep::Single,
        };
        let outcome = request
            .payment_payload
            .payload
            .get("transaction")
            .and_then(Value::as_str)
            .ok_or(X402Error::TransactionForm)
            .and_then(validate_transaction)
            .and_then(|()| plane.verify(&plane_request, trace));
        let outcome = match outcome {
            Ok(outcome) => outcome,
            Err(X402Error::TransactionForm) => PlaneVerifyOutcome::Invalid {
                reason: "transaction_form",
                payer: None,
            },
            Err(error) => return Err(fail(error)),
        };
        gateway
            .complete_read_only(principal, idempotency_key, trace, now)
            .map_err(|error| trace.wrap(X402Error::Gateway(error.into_error())))?;
        render_verify(outcome).map_err(fail)
    }

    /// Implements state-committing `/settle`. The caller's stable identity is
    /// mandatory and a successful response is unreachable without matching,
    /// locally verified protocol evidence.
    ///
    /// # Errors
    ///
    /// Returns trace-bound wire, support, gateway, plane, or evidence failures.
    #[allow(clippy::too_many_arguments)]
    pub fn settle(
        &self,
        gateway: &mut GatewayCore,
        principal: &PrincipalId,
        request: &FacilitatorRequest,
        stable_identity: [u8; 32],
        step: SettlementStep,
        plane: &mut impl FacilitatorPlane,
        trace: &TraceId,
        now: u64,
    ) -> Result<SettlementResponse, Traced<X402Error>> {
        let fail = |error| trace.wrap(error);
        request.validate().map_err(fail)?;
        self.require_supported(&request.payment_requirements)
            .map_err(fail)?;
        let identity =
            SettlementIdentity::derive(principal, request, stable_identity, step).map_err(fail)?;
        let request_digest = identity.request_digest;
        let idempotency_key = identity.idempotency_key;
        let translation = translation(
            TranslationKind::StateChanging,
            idempotency_key,
            request_digest,
        )
        .map_err(fail)?;
        let opened = gateway
            .begin_translation(principal, &translation, trace, now)
            .map_err(|error| trace.wrap(X402Error::Gateway(error.into_error())))?;
        if opened == TranslationStatus::Refused {
            return render_refused(&request.payment_requirements, "payment_refused", None)
                .map_err(fail);
        }
        let plane_request = FacilitatorPaymentRequest {
            payment_payload: request.payment_payload.clone(),
            payment_requirements: request.payment_requirements.clone(),
            idempotency_key,
            request_digest,
            settlement_step: step,
        };
        match plane.settle(plane_request, trace).map_err(fail)? {
            FacilitatorSettlementOutcome::Pending { transaction } => {
                if matches!(opened, TranslationStatus::ReceiptVerified { .. }) {
                    return Err(fail(X402Error::EvidenceMissing));
                }
                render_pending(&request.payment_requirements, transaction).map_err(fail)
            }
            FacilitatorSettlementOutcome::Refused { reason, payer } => {
                if matches!(opened, TranslationStatus::ReceiptVerified { .. }) {
                    return Err(fail(X402Error::EvidenceMismatch));
                }
                gateway
                    .refuse_translation(principal, idempotency_key, trace, now)
                    .map_err(|error| trace.wrap(X402Error::Gateway(error.into_error())))?;
                render_refused(&request.payment_requirements, reason, payer).map_err(fail)
            }
            FacilitatorSettlementOutcome::Executed(executed) => settle_executed(
                gateway,
                principal,
                idempotency_key,
                &request.payment_requirements,
                &executed,
                plane.receipt_keys(),
                trace,
                now,
            )
            .map_err(fail),
        }
    }

    fn require_supported(&self, requirements: &PaymentRequirements) -> Result<(), X402Error> {
        if self.supported.kinds.iter().any(|kind| {
            kind.x402_version == X402_VERSION
                && kind.scheme == requirements.scheme
                && kind.network == requirements.network
        }) {
            Ok(())
        } else {
            Err(X402Error::UnsupportedOffer)
        }
    }
}

fn translation(
    kind: TranslationKind,
    idempotency_key: [u8; 32],
    request_digest: [u8; 32],
) -> Result<TranslationRequest, X402Error> {
    let adapter = AdapterId::new(ADAPTER_ID).map_err(|error| X402Error::Gateway(error.into()))?;
    TranslationRequest::new(adapter, kind, idempotency_key, request_digest)
        .map_err(X402Error::Gateway)
}

fn render_verify(outcome: PlaneVerifyOutcome) -> Result<VerifyResponse, X402Error> {
    let response = match outcome {
        PlaneVerifyOutcome::Valid { payer, extra } => VerifyResponse {
            is_valid: true,
            invalid_reason: None,
            payer,
            extra,
        },
        PlaneVerifyOutcome::Invalid { reason, payer } => VerifyResponse {
            is_valid: false,
            invalid_reason: Some(safe_reason(reason)),
            payer,
            extra: None,
        },
    };
    validate_verify(&response)?;
    Ok(response)
}

fn render_pending(
    requirements: &PaymentRequirements,
    transaction_hash: String,
) -> Result<SettlementResponse, X402Error> {
    validate_network(&requirements.network)?;
    validate_transaction(&transaction_hash)?;
    let mut extensions = BTreeMap::new();
    extensions.insert(
        "layerx".to_owned(),
        json!({"verificationLevel": "submitted"}),
    );
    let response = SettlementResponse {
        success: false,
        error_reason: Some("settlement_pending".to_owned()),
        payer: None,
        transaction: transaction_hash,
        network: requirements.network.clone(),
        amount: None,
        extensions,
    };
    response.validate_wire()?;
    Ok(response)
}

fn render_refused(
    requirements: &PaymentRequirements,
    reason: &str,
    payer: Option<String>,
) -> Result<SettlementResponse, X402Error> {
    validate_network(&requirements.network)?;
    let response = SettlementResponse {
        success: false,
        error_reason: Some(safe_reason(reason)),
        payer,
        transaction: String::new(),
        network: requirements.network.clone(),
        amount: None,
        extensions: BTreeMap::new(),
    };
    response.validate_wire()?;
    Ok(response)
}

#[allow(clippy::too_many_arguments)]
fn settle_executed(
    gateway: &mut GatewayCore,
    principal: &PrincipalId,
    idempotency_key: [u8; 32],
    requirements: &PaymentRequirements,
    evidence: &SequencerEvidence,
    receipt_keys: &[SequencerAuthorization],
    trace: &TraceId,
    now: u64,
) -> Result<SettlementResponse, X402Error> {
    if evidence.rung == SettlementRung::Submitted {
        return Err(X402Error::EvidenceMissing);
    }
    let verified = verify_sequencer_signature(&evidence.signed_receipt, receipt_keys)
        .map_err(|_| X402Error::EvidenceMismatch)?;
    let protocol = verified.protocol().ok_or(X402Error::EvidenceMismatch)?;
    if protocol.activity_id() != evidence.transaction_hash || protocol.result_code() != 0 {
        return Err(X402Error::EvidenceMismatch);
    }
    let logs =
        verify_logs(&verified, evidence.logs.clone()).map_err(|_| X402Error::EvidenceMismatch)?;
    let mut payments = logs.logs().iter().filter(|log| {
        log.address()
            == [
                0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x10, 0x1a,
            ]
            && log.topics().len() == 4
            && hex(&log.topics()[0])
                == "c4da5d6f757c251cd70947b752917e5671b677517e3d953f362a1df6648ef5b1"
            && log.topics()[1] == idempotency_key
    });
    let payment = payments.next().ok_or(X402Error::EvidenceMismatch)?;
    if payments.next().is_some() || payment.data().len() != 64 || payment.data()[32..48] != [0; 16]
    {
        return Err(X402Error::EvidenceMismatch);
    }
    let asset = payment.data()[..32]
        .try_into()
        .map_err(|_| X402Error::EvidenceMismatch)?;
    let amount = u128::from_be_bytes(
        payment.data()[48..]
            .try_into()
            .map_err(|_| X402Error::EvidenceMismatch)?,
    );
    payment_matches(requirements, asset, payment.topics()[3], amount)?;
    let status = gateway
        .settle_with_sequencer_evidence(
            principal,
            idempotency_key,
            evidence,
            receipt_keys,
            trace,
            now,
        )
        .map_err(|error| match error.into_error() {
            layerx_interop_gateway::gateway::SequencerSettlementError::Gateway(error) => {
                X402Error::Gateway(error)
            }
            _ => X402Error::EvidenceMismatch,
        })?;
    let TranslationStatus::ReceiptVerified { receipt_digest } = status else {
        return Err(X402Error::EvidenceMissing);
    };
    let digest_text = hex(&receipt_digest);
    let mut extensions = BTreeMap::new();
    extensions.insert(
        "layerx".to_owned(),
        json!({
            "receipt": STANDARD.encode(&evidence.signed_receipt),
            "receiptDigest": digest_text,
            "verificationLevel": "receipted"
        }),
    );
    let response = SettlementResponse {
        success: true,
        error_reason: None,
        payer: Some(hex(&payment.topics()[2])),
        transaction: format!("0x{}", hex(&evidence.transaction_hash)),
        network: requirements.network.clone(),
        amount: Some(requirements.amount),
        extensions,
    };
    response.validate_wire()?;
    Ok(response)
}

pub(crate) fn validate_verify(response: &VerifyResponse) -> Result<(), X402Error> {
    if response
        .payer
        .as_ref()
        .is_some_and(|payer| !bounded(payer, 256))
        || response
            .invalid_reason
            .as_ref()
            .is_some_and(|reason| !bounded(reason, 512))
        || response
            .extra
            .as_ref()
            .is_some_and(|extra| !extra.is_object())
        || response.is_valid == response.invalid_reason.is_some()
    {
        return Err(X402Error::InvalidPayload);
    }
    Ok(())
}

pub(crate) fn validate_supported(response: &SupportedResponse) -> Result<(), X402Error> {
    if response.kinds.is_empty()
        || response.kinds.len() > 64
        || response.extensions.len() > 32
        || response.signers.len() > 64
    {
        return Err(X402Error::UnsupportedOffer);
    }
    for (index, kind) in response.kinds.iter().enumerate() {
        if kind.x402_version != X402_VERSION
            || !identifier(&kind.scheme, 32)
            || validate_network(&kind.network).is_err()
            || kind.extra.as_ref().is_some_and(|extra| !extra.is_object())
            || response.kinds[..index].iter().any(|prior| {
                prior.x402_version == kind.x402_version
                    && prior.scheme == kind.scheme
                    && prior.network == kind.network
            })
        {
            return Err(X402Error::UnsupportedOffer);
        }
    }
    for (index, extension) in response.extensions.iter().enumerate() {
        if !identifier(extension, 32) || response.extensions[..index].contains(extension) {
            return Err(X402Error::UnsupportedOffer);
        }
    }
    for (pattern, signers) in &response.signers {
        if !network_pattern(pattern)
            || signers.is_empty()
            || signers.len() > 64
            || signers.iter().any(|signer| !bounded(signer, 256))
        {
            return Err(X402Error::UnsupportedOffer);
        }
    }
    Ok(())
}

fn network_pattern(value: &str) -> bool {
    value == "eip155:*" || validate_network(value).is_ok()
}

fn identifier(value: &str, limit: usize) -> bool {
    bounded(value, limit)
        && value
            .bytes()
            .all(|byte| byte.is_ascii_alphanumeric() || matches!(byte, b'-' | b'_' | b'.'))
}

fn bounded(value: &str, limit: usize) -> bool {
    !value.is_empty() && value.len() <= limit && !value.contains('\0')
}

fn safe_reason(reason: &str) -> String {
    if !reason.is_empty()
        && reason.len() <= 64
        && reason
            .bytes()
            .all(|byte| byte.is_ascii_lowercase() || byte == b'_')
    {
        reason.to_owned()
    } else {
        "payment_refused".to_owned()
    }
}

fn digest(domain: &[u8], parts: &[&[u8]]) -> [u8; 32] {
    let mut hash = Sha256::new();
    hash.update(domain);
    for part in parts {
        hash.update(part);
    }
    hash.finalize().into()
}

fn hex(bytes: &[u8; 32]) -> String {
    const DIGITS: &[u8; 16] = b"0123456789abcdef";
    let mut output = String::with_capacity(64);
    for byte in bytes {
        output.push(char::from(DIGITS[usize::from(byte >> 4)]));
        output.push(char::from(DIGITS[usize::from(byte & 0x0f)]));
    }
    output
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::seller::tests::{opened_gateway, required};
    use ed25519_dalek::{Signer as _, SigningKey};
    use layerx_interop_gateway::evidence::canonical_receipt_digest;
    use layerx_proof::sequencer_logs::SequencerLog;

    fn executed(
        accepted: &PaymentRequirements,
    ) -> (SequencerEvidence, Vec<SequencerAuthorization>) {
        fn bytes(value: &str) -> Vec<u8> {
            let value = value.strip_prefix("0x").unwrap_or(value);
            (0..value.len())
                .step_by(2)
                .map(|index| {
                    u8::from_str_radix(&value[index..index + 2], 16)
                        .unwrap_or_else(|error| panic!("hex: {error}"))
                })
                .collect()
        }
        fn replace(receipt: &mut [u8], old: &[u8], new: &[u8]) {
            assert_eq!(old.len(), new.len());
            let positions: Vec<_> = receipt
                .windows(old.len())
                .enumerate()
                .filter_map(|(index, value)| (value == old).then_some(index))
                .collect();
            assert_eq!(positions.len(), 1);
            receipt[positions[0]..positions[0] + old.len()].copy_from_slice(new);
        }
        let fixture: Value = serde_json::from_str(include_str!(concat!(
            env!("CARGO_MANIFEST_DIR"),
            "/../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
        )))
        .unwrap_or_else(|error| panic!("fixture: {error}"));
        let case = fixture["cases"]
            .as_array()
            .unwrap_or_else(|| panic!("cases"))
            .iter()
            .find(|case| case["id"] == "V1-layerxledger-pay-exact")
            .unwrap_or_else(|| panic!("PaymentExact fixture"));
        let text = |value: &Value| {
            value
                .as_str()
                .unwrap_or_else(|| panic!("fixture string"))
                .to_owned()
        };
        let mut receipt = bytes(&text(&case["signedReceipt"]));
        let log = &case["receiptJson"]["logs"][0];
        let mut topics: Vec<[u8; 32]> = log["topics"]
            .as_array()
            .unwrap_or_else(|| panic!("topics"))
            .iter()
            .map(|value| {
                bytes(&text(value))
                    .try_into()
                    .unwrap_or_else(|_| panic!("topic"))
            })
            .collect();
        let recipient: [u8; 32] = bytes(&accepted.pay_to)
            .try_into()
            .unwrap_or_else(|_| panic!("payee"));
        let recorded_topics = topics.concat();
        topics = vec![topics[0], [0x74; 32], topics[1], recipient];
        replace(&mut receipt, &recorded_topics, &topics.concat());
        let old_data = bytes(&text(&log["data"]));
        let mut data = bytes(&accepted.asset);
        data.extend_from_slice(&[0; 16]);
        data.extend_from_slice(&accepted.amount.value().to_be_bytes());
        replace(
            &mut receipt,
            &Sha256::digest(&old_data),
            &Sha256::digest(&data),
        );
        replace(
            &mut receipt,
            &bytes(&text(&case["receiptJson"]["txHash"])),
            &[0x61; 32],
        );
        let key = SigningKey::from_bytes(&[0x72; 32]);
        let digest =
            canonical_receipt_digest(&receipt).unwrap_or_else(|error| panic!("digest: {error}"));
        let signature_start = receipt.len() - 64;
        receipt[signature_start..].copy_from_slice(&key.sign(&digest).to_bytes());
        let key_id = bytes(&text(&case["pinnedKey"]["keyId"]))
            .try_into()
            .unwrap_or_else(|_| panic!("key id"));
        (
            SequencerEvidence {
                transaction_hash: [0x61; 32],
                signed_receipt: receipt,
                logs: vec![SequencerLog::new(
                    bytes(&text(&log["address"]))
                        .try_into()
                        .unwrap_or_else(|_| panic!("address")),
                    topics,
                    data,
                    0,
                )],
                rung: SettlementRung::Receipted,
            },
            vec![SequencerAuthorization::new(
                key_id,
                key.verifying_key().to_bytes(),
                1000,
                1999,
            )],
        )
    }

    #[test]
    fn settlement_transaction_is_tx_hash() {
        let required = required();
        let accepted = &required.accepts[0];
        let (executed, receipt_keys) = executed(accepted);
        let (mut gateway, principal, trace) = opened_gateway();
        let response = settle_executed(
            &mut gateway,
            &principal,
            [0x74; 32],
            accepted,
            &executed,
            &receipt_keys,
            &trace,
            1,
        )
        .unwrap_or_else(|error| panic!("settlement: {error}"));
        assert_eq!(response.transaction, format!("0x{}", "61".repeat(32)));
        assert_eq!(response.network, "eip155:125");
        assert_eq!(
            response.extensions["layerx"]["verificationLevel"],
            "receipted"
        );
        assert_eq!(response.validate_wire(), Ok(()));
    }

    #[test]
    fn pending_transaction_carries_submitted_rung() {
        let required = required();
        let accepted = &required.accepts[0];
        let transaction = format!("0x{}", "61".repeat(32));
        let response = render_pending(accepted, transaction.clone())
            .unwrap_or_else(|error| panic!("pending: {error}"));
        assert!(!response.success);
        assert_eq!(response.error_reason.as_deref(), Some("settlement_pending"));
        assert_eq!(response.transaction, transaction);
        assert_eq!(response.network, "eip155:125");
        assert_eq!(
            response.extensions["layerx"]["verificationLevel"],
            "submitted"
        );
        for transaction in ["lxp:abc", "0xabc", "", &format!("0x{}", "00".repeat(32))] {
            assert_eq!(
                render_pending(accepted, transaction.to_owned()),
                Err(X402Error::TransactionForm)
            );
        }
    }

    #[test]
    fn supported_networks_are_eip155() {
        let supported = SupportedResponse {
            kinds: vec![FacilitatorKind {
                x402_version: X402_VERSION,
                scheme: "exact".to_owned(),
                network: "eip155:125".to_owned(),
                extra: None,
            }],
            extensions: Vec::new(),
            signers: BTreeMap::from([("eip155:*".to_owned(), vec!["receipt-key".to_owned()])]),
        };
        assert_eq!(validate_supported(&supported), Ok(()));
        let mut legacy = supported.clone();
        legacy.kinds[0].network = "layerx:beta".to_owned();
        assert_eq!(
            validate_supported(&legacy),
            Err(X402Error::UnsupportedOffer)
        );
        let mut legacy_signers = supported;
        legacy_signers.signers =
            BTreeMap::from([("layerx:*".to_owned(), vec!["receipt-key".to_owned()])]);
        assert_eq!(
            validate_supported(&legacy_signers),
            Err(X402Error::UnsupportedOffer)
        );
    }
}
