#![forbid(unsafe_code)]

pub mod clients;
pub mod engine;
pub mod evm;
pub mod journal;
pub mod migration;

use layerx_client::sequencer::{ReceiptKeyEntry, SequencerLog};
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_proof::receipt::{verify_sequencer_signature, ReceiptCheck};
use layerx_types::account::{AccountId, AccountNamespace};
use layerx_types::rung::Rung;
use sha3::Keccak256;

use crate::evm::{
    decode_ledger_payment, encode_draw, encode_pay_exact, LedgerPayment, LEDGER_ADDRESS,
};
use serde::{Deserialize, Serialize};
use sha2::{Digest as _, Sha256};

pub const EXTERNAL_CUSTODY_LABEL: &str =
    "External custody: this independent market maker controls the off-platform funds and payout.";
pub const ORDER_DIGEST_DOMAIN: &[u8] = b"LXP/market-maker-ramp/order/v1\0";
pub const PROVIDER_CONTRACT_VERSION: &str = "layerx-ramp-provider-v1";
pub const COMPLIANCE_CONTRACT_VERSION: &str = "layerx-ramp-compliance-v1";
pub const PAXEER_CONTRACT_VERSION: &str = "layerx-ramp-paxeer-v1";

const MAX_IDENTIFIER_BYTES: usize = 128;
const MAX_CURRENCY_BYTES: usize = 16;
const MAX_TOKEN_BYTES: usize = 512;

#[derive(Clone, Copy, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "snake_case")]
pub enum RampDirection {
    OnRamp,
    OffRamp,
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(deny_unknown_fields)]
pub struct AuthenticatedPrincipal {
    pub principal_id: String,
    pub account: String,
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(deny_unknown_fields)]
pub struct OperatorIdentity {
    pub principal_id: String,
    pub account: String,
    pub signer_key_handle: String,
}

impl OperatorIdentity {
    /// # Errors
    /// Returns [`RampError::InvalidPrincipal`] or [`RampError::InvalidOrder`] when the operator identity is unbound.
    pub fn validate(&self) -> Result<(), RampError> {
        validate_operator(self)
    }
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(deny_unknown_fields)]
pub struct QuoteTerms {
    pub quote_id: String,
    pub direction: RampDirection,
    pub layerx_asset: [u8; 32],
    pub layerx_amount: u128,
    pub external_currency: String,
    pub external_amount_minor: u128,
    pub rate_numerator: u128,
    pub rate_denominator: u128,
    pub fee_minor: u128,
    pub maximum_slippage_bps: u16,
    pub context: [u8; 32],
    pub provider_token: String,
    pub payout_token: String,
    pub expires_at: u64,
}

impl QuoteTerms {
    /// # Errors
    /// Returns [`RampError::InvalidOrder`] when quote terms exceed their declared bounds.
    pub fn validate(&self, now: u64) -> Result<(), RampError> {
        validate_identifier(&self.quote_id)?;
        if self.layerx_asset == [0; 32]
            || self.layerx_amount == 0
            || self.external_amount_minor == 0
            || self.rate_numerator == 0
            || self.rate_denominator == 0
            || self.maximum_slippage_bps > 10_000
            || self.context == [0; 32]
            || self.expires_at <= now
            || self.external_currency.is_empty()
            || self.external_currency.len() > MAX_CURRENCY_BYTES
            || self.provider_token.is_empty()
            || self.provider_token.len() > MAX_TOKEN_BYTES
            || self.payout_token.is_empty()
            || self.payout_token.len() > MAX_TOKEN_BYTES
        {
            return Err(RampError::InvalidOrder);
        }
        validate_opaque(&self.provider_token, MAX_TOKEN_BYTES)?;
        validate_opaque(&self.payout_token, MAX_TOKEN_BYTES)
    }
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(deny_unknown_fields)]
pub struct CreateOrder {
    pub order_id: String,
    pub quote_id: String,
    pub payer_grant: Option<[u8; 32]>,
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(deny_unknown_fields)]
pub struct RampOrder {
    pub order_id: String,
    pub quote: QuoteTerms,
    pub customer: AuthenticatedPrincipal,
    pub operator: OperatorIdentity,
    pub payer_grant: Option<[u8; 32]>,
    pub context: [u8; 32],
    pub order_digest: [u8; 32],
}

impl RampOrder {
    /// # Errors
    /// Returns [`RampError::InvalidOrder`] or [`RampError::InvalidPrincipal`] when the request, quote or principals are unbound.
    pub fn bind(
        request: CreateOrder,
        quote: QuoteTerms,
        customer: AuthenticatedPrincipal,
        operator: OperatorIdentity,
        now: u64,
    ) -> Result<Self, RampError> {
        validate_identifier(&request.order_id)?;
        validate_identifier(&request.quote_id)?;
        quote.validate(now)?;
        validate_principal(&customer)?;
        validate_operator(&operator)?;
        if request.quote_id != quote.quote_id
            || request.payer_grant == Some([0; 32])
            || matches!(quote.direction, RampDirection::OnRamp) && request.payer_grant.is_some()
            || matches!(quote.direction, RampDirection::OffRamp) && request.payer_grant.is_none()
        {
            return Err(RampError::InvalidOrder);
        }
        let context = quote.context;
        let mut order = Self {
            order_id: request.order_id,
            quote,
            customer,
            operator,
            payer_grant: request.payer_grant,
            context,
            order_digest: [0; 32],
        };
        order.order_digest = order.digest();
        Ok(order)
    }

    #[must_use]
    pub fn digest(&self) -> [u8; 32] {
        let mut hasher = Sha256::new();
        hasher.update(ORDER_DIGEST_DOMAIN);
        field(&mut hasher, self.direction_name().as_bytes());
        field(&mut hasher, self.order_id.as_bytes());
        field(&mut hasher, self.quote.quote_id.as_bytes());
        field(&mut hasher, self.customer.principal_id.as_bytes());
        field(&mut hasher, self.customer.account.as_bytes());
        field(&mut hasher, self.operator.principal_id.as_bytes());
        field(&mut hasher, self.operator.account.as_bytes());
        field(&mut hasher, self.operator.signer_key_handle.as_bytes());
        field(&mut hasher, &self.quote.layerx_asset);
        field(&mut hasher, &self.quote.layerx_amount.to_be_bytes());
        field(&mut hasher, self.quote.external_currency.as_bytes());
        field(&mut hasher, &self.quote.external_amount_minor.to_be_bytes());
        field(&mut hasher, &self.quote.rate_numerator.to_be_bytes());
        field(&mut hasher, &self.quote.rate_denominator.to_be_bytes());
        field(&mut hasher, &self.quote.fee_minor.to_be_bytes());
        field(&mut hasher, &self.quote.maximum_slippage_bps.to_be_bytes());
        field(&mut hasher, &self.context);
        field(&mut hasher, self.quote.provider_token.as_bytes());
        field(&mut hasher, self.quote.payout_token.as_bytes());
        field(&mut hasher, &self.quote.expires_at.to_be_bytes());
        field(
            &mut hasher,
            self.payer_grant
                .as_ref()
                .map_or(&[], |value| value.as_slice()),
        );
        hasher.finalize().into()
    }

    #[must_use]
    pub const fn direction(&self) -> RampDirection {
        self.quote.direction
    }

    fn direction_name(&self) -> &'static str {
        match self.quote.direction {
            RampDirection::OnRamp => "on_ramp",
            RampDirection::OffRamp => "off_ramp",
        }
    }

    /// # Errors
    /// Returns [`RampError::OrderBinding`] when the digest or context does not match the bound quote.
    pub fn validate_bound(&self) -> Result<(), RampError> {
        if self.order_digest != self.digest() || self.context != self.quote.context {
            return Err(RampError::OrderBinding);
        }
        Ok(())
    }
}

fn validate_identifier(value: &str) -> Result<(), RampError> {
    if value.is_empty()
        || value.len() > MAX_IDENTIFIER_BYTES
        || !value
            .bytes()
            .all(|byte| byte.is_ascii_alphanumeric() || matches!(byte, b'-' | b'_' | b'.'))
    {
        return Err(RampError::InvalidOrder);
    }
    Ok(())
}

pub(crate) fn validate_principal(principal: &AuthenticatedPrincipal) -> Result<(), RampError> {
    validate_identifier(&principal.principal_id)?;
    let account = AccountId::parse(&principal.account).map_err(|_| RampError::InvalidPrincipal)?;
    if account.namespace() != AccountNamespace::AgentMain {
        return Err(RampError::InvalidPrincipal);
    }
    Ok(())
}

fn validate_operator(operator: &OperatorIdentity) -> Result<(), RampError> {
    validate_identifier(&operator.principal_id)?;
    validate_opaque(&operator.signer_key_handle, MAX_TOKEN_BYTES)?;
    let account = AccountId::parse(&operator.account).map_err(|_| RampError::InvalidPrincipal)?;
    if account.namespace() != AccountNamespace::AgentMain {
        return Err(RampError::InvalidPrincipal);
    }
    Ok(())
}

fn validate_opaque(value: &str, maximum: usize) -> Result<(), RampError> {
    if value.is_empty()
        || value.len() > maximum
        || value
            .bytes()
            .any(|byte| byte.is_ascii_control() || matches!(byte, b'\'' | b'"' | b'\\'))
    {
        return Err(RampError::InvalidOrder);
    }
    Ok(())
}

fn field(hasher: &mut Sha256, bytes: &[u8]) {
    let length = bytes.len() as u128;
    hasher.update(length.to_be_bytes());
    hasher.update(bytes);
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct LedgerCall {
    pub to: [u8; 20],
    pub data: Vec<u8>,
}

fn ledger_account(account: &str) -> Result<[u8; 32], RampError> {
    let account = AccountId::parse(account).map_err(|_| RampError::InvalidPrincipal)?;
    layerx_wire::hash::account_id(&account).map_err(|_| RampError::Intent)
}

/// # Errors
/// Refuses on-ramp orders and off-ramp orders without a customer payer grant.
pub fn compile_payer_grant_draw(
    order: &RampOrder,
    receiver_sequence: u64,
) -> Result<LedgerCall, RampError> {
    order.validate_bound()?;
    if order.direction() != RampDirection::OffRamp {
        return Err(RampError::PayerGrantRequired);
    }
    let grant = order
        .payer_grant
        .filter(|grant| *grant != [0; 32])
        .ok_or(RampError::PayerGrantRequired)?;
    Ok(LedgerCall {
        to: LEDGER_ADDRESS,
        data: encode_draw(
            ledger_account(&order.customer.account)?,
            order.quote.layerx_asset,
            order.quote.layerx_amount,
            grant,
            receiver_sequence,
            order.order_digest,
            order.context,
        ),
    })
}

/// # Errors
/// Refuses orders that do not describe an operator-funded on-ramp payment.
pub fn compile_operator_send(order: &RampOrder) -> Result<LedgerCall, RampError> {
    order.validate_bound()?;
    if order.direction() != RampDirection::OnRamp || order.payer_grant.is_some() {
        return Err(RampError::InvalidOrder);
    }
    Ok(LedgerCall {
        to: LEDGER_ADDRESS,
        data: encode_pay_exact(
            ledger_account(&order.operator.account)?,
            ledger_account(&order.customer.account)?,
            order.quote.layerx_asset,
            order.quote.layerx_amount,
            order.quote.expires_at,
            order.order_digest,
        ),
    })
}

#[derive(Clone, Debug)]
pub struct ReceiptEvidence {
    pub raw_transaction: Vec<u8>,
    pub signed_receipt: Vec<u8>,
    pub receipt_keys: Vec<ReceiptKeyEntry>,
    pub logs: Vec<SequencerLog>,
}

#[derive(Clone, Debug, Eq, PartialEq, Serialize)]
pub struct VerifiedLayerxLeg {
    pub tx_hash: [u8; 32],
    pub receipt_digest: [u8; 32],
    pub height: u64,
    #[serde(serialize_with = "serialize_rung")]
    pub rung: Rung,
}

fn serialize_rung<S: serde::Serializer>(rung: &Rung, serializer: S) -> Result<S::Ok, S::Error> {
    serializer.serialize_str(rung.name())
}

/// # Errors
/// Refuses unpinned signatures, transaction mismatches and logs that do not bind exactly one payment to the order.
pub fn verify_order_receipt(
    order: &RampOrder,
    evidence: &ReceiptEvidence,
) -> Result<VerifiedLayerxLeg, RampError> {
    order.validate_bound()?;
    let keys: Vec<_> = evidence
        .receipt_keys
        .iter()
        .map(|key| {
            SequencerAuthorization::new(
                key.key_id,
                key.public_key,
                key.first_height,
                key.last_height,
            )
        })
        .collect();
    let receipt = verify_sequencer_signature(&evidence.signed_receipt, keys.as_slice())
        .map_err(|failure| RampError::Receipt(failure.check))?;
    let protocol = receipt.protocol().ok_or(RampError::ReceiptMismatch)?;
    let tx_hash: [u8; 32] = Keccak256::digest(&evidence.raw_transaction).into();
    if evidence.raw_transaction.is_empty()
        || protocol.activity_id() != tx_hash
        || protocol.result_code() != 0
    {
        return Err(RampError::ReceiptMismatch);
    }
    let logs = evidence
        .logs
        .iter()
        .map(|log| {
            layerx_proof::sequencer_logs::SequencerLog::new(
                log.address,
                log.topics.clone(),
                log.data.clone(),
                log.log_index,
            )
        })
        .collect();
    layerx_proof::sequencer_logs::verify_logs(&receipt, logs)
        .map_err(|_| RampError::ReceiptMismatch)?;
    let (from, to) = match order.direction() {
        RampDirection::OnRamp => (&order.operator.account, &order.customer.account),
        RampDirection::OffRamp => (&order.customer.account, &order.operator.account),
    };
    let from = ledger_account(from)?;
    let to = ledger_account(to)?;
    let expected = match order.direction() {
        RampDirection::OnRamp if order.payer_grant.is_none() => LedgerPayment::Exact {
            from,
            to,
            reference: order.order_digest,
            asset: order.quote.layerx_asset,
            amount: order.quote.layerx_amount,
        },
        RampDirection::OffRamp => LedgerPayment::Drawn {
            from,
            to,
            grant: order.payer_grant.ok_or(RampError::PayerGrantRequired)?,
            asset: order.quote.layerx_asset,
            amount: order.quote.layerx_amount,
            idempotency_key: order.order_digest,
            context: order.context,
        },
        RampDirection::OnRamp => return Err(RampError::ReceiptMismatch),
    };
    let mut payments = evidence.logs.iter().filter_map(decode_ledger_payment);
    if payments.next() != Some(expected) || payments.next().is_some() {
        return Err(RampError::ReceiptMismatch);
    }
    let height = u64::from_be_bytes(
        protocol.batch_id()[24..]
            .try_into()
            .map_err(|_| RampError::ReceiptMismatch)?,
    );
    Ok(VerifiedLayerxLeg {
        tx_hash,
        receipt_digest: Keccak256::digest(&evidence.signed_receipt).into(),
        height,
        rung: Rung::Receipted,
    })
}

#[derive(Clone, Copy, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "snake_case")]
pub enum AggregateStatus {
    Pending,
    Unknown,
    Refused,
    ManualReview,
    Reversed,
    Done,
}

#[derive(Clone, Debug, Eq, PartialEq, Serialize)]
pub struct RampPresentation {
    pub external_custody_label: &'static str,
    pub status: AggregateStatus,
    pub order_digest: [u8; 32],
    pub activity_id: Option<[u8; 32]>,
    pub receipt_digest: Option<[u8; 32]>,
    pub provider_evidence_digest: Option<[u8; 32]>,
    pub refusal_code: Option<String>,
    pub retry_at: Option<u64>,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum RampError {
    SourceSettlement(layerx_migrate::MigrationError),
    InvalidPrincipal,
    InvalidOrder,
    OrderBinding,
    PayerGrantRequired,
    Intent,
    Receipt(ReceiptCheck),
    ReceiptMismatch,
    Journal,
    Conflict,
    LeaseHeld,
    IllegalTransition,
    Compliance,
    Provider,
    Layerx,
    Paxeer,
    Configuration,
}

#[must_use]
pub const fn platform_ramp_toolkit() -> &'static str {
    "durable-ordinary-principal-receipt-gated-market-maker-ramp"
}
