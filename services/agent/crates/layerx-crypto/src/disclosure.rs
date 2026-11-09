//! Byte-bound EIP-1559 call disclosure.

#[path = "disclosure/call.rs"]
mod call_codec;

/// Call disclosure, including the exact-payment ABI produced by the web payer.
pub mod call {
    pub use super::call_codec::*;
    use super::*;

    /// Complete arguments of a ledger `payExact` call. Expiry is in milliseconds.
    #[derive(Clone, Debug, Eq, PartialEq)]
    pub struct PayExactCall {
        pub payer: [u8; 32],
        pub recipient: [u8; 32],
        pub asset: [u8; 32],
        pub amount: u128,
        pub expires_at_ms: u64,
        pub idempotency_key: [u8; 32],
    }

    pub(super) fn is_pay_exact(call: &Eip1559Call) -> bool {
        call.data.get(..4)
            == Some(
                ledger_selector("payExact(bytes32,bytes32,bytes32,uint128,uint64,bytes32)")
                    .as_slice(),
            )
    }

    /// Decode the exact ABI, refusing another target, noncanonical integers or trailing bytes.
    pub fn decode_pay_exact(call: &Eip1559Call) -> Result<PayExactCall, DisclosureError> {
        let mut ledger = [0; 20];
        ledger[18..].copy_from_slice(&[0x10, 0x1a]);
        if call.to != ledger || !is_pay_exact(call) || call.data.len() != 4 + 6 * 32 {
            return Err(DisclosureError::MalformedPayload);
        }
        let args = &call.data[4..];
        let word = |index: usize| -> [u8; 32] {
            args[index * 32..(index + 1) * 32]
                .try_into()
                .expect("checked ABI length")
        };
        let amount = word(3);
        let expiry = word(4);
        if amount[..16] != [0; 16] || expiry[..24] != [0; 24] {
            return Err(DisclosureError::MalformedPayload);
        }
        let payment = PayExactCall {
            payer: word(0),
            recipient: word(1),
            asset: word(2),
            amount: u128::from_be_bytes(amount[16..].try_into().expect("uint128")),
            expires_at_ms: u64::from_be_bytes(expiry[24..].try_into().expect("uint64")),
            idempotency_key: word(5),
        };
        if payment.payer == [0; 32]
            || payment.recipient == [0; 32]
            || payment.asset == [0; 32]
            || payment.amount == 0
            || payment.expires_at_ms == 0
            || payment.idempotency_key == [0; 32]
        {
            return Err(DisclosureError::MalformedPayload);
        }
        Ok(payment)
    }

    /// Bind canonical transaction bytes and sender, including exact web payments.
    pub fn bind_call(call: &Eip1559Call, from: [u8; 20]) -> Result<Disclosure, DisclosureError> {
        if !is_pay_exact(call) {
            return super::call_codec::bind_call(call, from);
        }
        let fields = super::pay_exact_fields(call, from)?;
        Ok(Disclosure {
            activity_type: fields.activity_type,
            actor: fields.actor,
            authority: fields.authority,
            counterparties: fields.counterparties,
            amounts: fields.amounts,
            asset: fields.asset,
            fee_limit: fields.fee_limit,
            expiry: fields.expiry,
            idempotency_key: fields.idempotency_key,
            evm_payout_binding: fields.evm_payout_binding,
            withdrawal: fields.withdrawal,
            payment: fields.payment,
            authority_grant: fields.authority_grant,
            session_grant: fields.session_grant,
            onboarding: fields.onboarding,
            native_operation: fields.native_operation,
            source: Source::Call(call.clone()),
            call_sender: Some(from),
            signing_digest: call.signing_hash(),
            budget_context: None,
        })
    }

    /// Rebind the consumer's exact transaction and sender before trusting its disclosure.
    pub fn validate_call(
        call: &Eip1559Call,
        disclosure: &Disclosure,
        from: [u8; 20],
    ) -> Result<(), DisclosureError> {
        if &bind_call(call, from)? != disclosure {
            return Err(DisclosureError::FieldMismatch("call"));
        }
        Ok(())
    }
}

mod native;
mod program;

pub use program::{
    DisclosedProgramCall, DisclosedProgramDeploy, DisclosedProgramUpgrade,
    DisclosedProgramWindDown, DisclosedProgramWindDownOperation,
};

pub use native::{
    BudgetStateContext, DisclosedNativeBudgetCreate, DisclosedNativeBudgetDefund,
    DisclosedNativeBudgetFund, DisclosedNativeBudgetRevoke, DisclosedNativeIdentity,
    DisclosedNativeOperation, DisclosedRecoveryPolicy,
};

use std::fmt;

use sha2::{Digest as _, Sha256};

use layerx_types::payload::{ActivityType, ModuleId};
type Activity = layerx_wire::historical::activity::Activity;
use layerx_wire::decode::Decoder;
use layerx_wire::encode::Encoder;
use layerx_wire::hash;
use layerx_wire::WireError;

use crate::{
    authority_grant::AuthorityGrant,
    ct,
    evm_transaction::Eip1559Call,
    onboarding::{OnboardingConsent, SponsoredRegistration},
    payments::{Grant, Payment},
    session::{decode_session_key, IssuedSessionKey},
};

const ASSET_SEND_ORDINAL: u16 = 5;
const SEND_WIRE_TAG: u16 = 0x5301;
const SEND_FIELD_COUNT: u16 = 10;
const MAX_SEND_CONDITIONS: usize = 8;
const MAX_TRANSPORT_DISCLOSURE_BYTES: usize = 1_048_576;

/// The semantic role of one counterparty named in a disclosure.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum CounterpartyRole {
    /// Account whose balance is debited.
    Payer,
    /// Account whose balance is credited.
    Recipient,
}

/// One complete counterparty entry decoded from a module payload.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct Counterparty {
    /// Role this account has in the activity.
    pub role: CounterpartyRole,
    /// Canonical protocol account identifier.
    pub account: [u8; 32],
}

/// The semantic role of one amount named in a disclosure.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum AmountRole {
    /// Principal transferred from payer to recipient.
    Transfer,
    /// Maximum spend permitted in one configured budget period.
    SpendingLimit,
    /// Maximum units that can ever be issued for a registered asset.
    SupplyCap,
    /// Maximum units one payer-grant draw may transfer.
    PerDrawMaximum,
    /// Total units authorized by one payer grant.
    GrantAllowance,
}

/// One complete amount entry decoded from a module payload.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct DisclosedAmount {
    /// Role this amount has in the activity.
    pub role: AmountRole,
    /// Unsigned protocol amount.
    pub value: u128,
}

/// Complete governance wallet-binding semantics decoded from canonical bytes.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct DisclosedEvmPayoutBinding {
    /// Core DID identifier whose payout address changes.
    pub did_id: [u8; 32],
    /// Protocol network named by the wallet ownership proof.
    pub network_id: u32,
    /// Exact EVM address recovered from the ownership signature.
    pub payout_address: [u8; 20],
    /// Commitment to the full public ownership signature carried by the intent.
    pub ownership_signature_digest: [u8; 32],
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct DisclosedWithdrawal {
    pub account_sequence: u64,
    pub evm_recipient: [u8; 20],
    pub request_anchor: [u8; 32],
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct DisclosedSessionGrant {
    pub grant: IssuedSessionKey,
    pub expiry_sequence: u64,
    pub action_key: [u8; 32],
    pub replacement: Option<DisclosedSessionReplacement>,
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct DisclosedSessionReplacement {
    pub predecessor_grant_id: [u8; 32],
    pub expected_charge_state: [u8; 32],
}

/// Every time bound that can make the disclosed activity expire.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct Expiry {
    /// Inclusive envelope lower bound.
    pub not_before: u64,
    /// Inclusive envelope upper bound.
    pub not_after: u64,
    /// Module-payload expiry enforced by the asset transfer.
    pub payload_expires_at: u64,
}

/// A complete human-reviewable description derived from canonical bytes.
///
/// Semantic fields are public so a remote review surface can serialize and
/// present them. Any change is detected before a signer receives a request.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct Disclosure {
    /// Exact registered activity type.
    pub activity_type: ActivityType,
    /// Actor DID from the canonical envelope.
    pub actor: Vec<u8>,
    /// Exact protocol authority representation.
    pub authority: Vec<u8>,
    /// Every account whose balance participates in the operation.
    pub counterparties: Vec<Counterparty>,
    /// Every value transferred by the operation.
    pub amounts: Vec<DisclosedAmount>,
    /// Asset identifier the amounts are denominated in.
    pub asset: [u8; 32],
    /// Maximum fee authorised by the envelope.
    pub fee_limit: u128,
    /// Envelope and module expiry bounds.
    pub expiry: Expiry,
    /// Exact retry identity shared by envelope and payload.
    pub idempotency_key: [u8; 32],
    /// Governance wallet-binding semantics, present only for that activity type.
    pub evm_payout_binding: Option<DisclosedEvmPayoutBinding>,
    pub withdrawal: Option<DisclosedWithdrawal>,
    /// Decoded payment or Programs payload, present for those activity types.
    pub payment: Option<Payment>,
    pub authority_grant: Option<AuthorityGrant>,
    pub session_grant: Option<DisclosedSessionGrant>,
    pub onboarding: Option<DisclosedOnboarding>,
    pub native_operation: Option<DisclosedNativeOperation>,
    source: Source,
    call_sender: Option<[u8; 20]>,
    signing_digest: [u8; 32],
    budget_context: Option<BudgetStateContext>,
}

#[derive(Clone, Debug, Eq, PartialEq)]
enum Source {
    Call(Eip1559Call),
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum DisclosedOnboarding {
    Consent(OnboardingConsent),
    Registration(SponsoredRegistration),
}

impl DisclosedOnboarding {
    /// # Errors
    /// Refuses invalid or altered onboarding commitments.
    pub fn encode(&self) -> Result<Vec<u8>, DisclosureError> {
        let mut encoder = Encoder::new(2048);
        match self {
            Self::Consent(consent) => {
                encoder.u8(1)?;
                encoder.bytes(consent.target.as_bytes(), 255)?;
                encoder.fixed(&consent.target_public_key)?;
                encoder.bytes(
                    &consent
                        .payload()
                        .map_err(|_| DisclosureError::MalformedPayload)?,
                    1024,
                )?;
            }
            Self::Registration(registration) => {
                encoder.u8(2)?;
                encoder.bytes(
                    &registration
                        .payload()
                        .map_err(|_| DisclosureError::MalformedPayload)?,
                    1024,
                )?;
            }
        }
        Ok(encoder.finish())
    }
}

/// Typed refusal produced while deriving or validating a disclosure.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum DisclosureError {
    /// The canonical activity envelope was rejected by the wire crate.
    Wire(WireError),
    /// No complete semantic decoder exists for the named activity.
    UnsupportedActivity(u32),
    /// The module payload was malformed or semantically inconsistent.
    MalformedPayload,
    /// The envelope's payload commitment did not cover its payload bytes.
    PayloadHash,
    /// A named disclosure field differed from the canonical bytes.
    FieldMismatch(&'static str),
}

impl fmt::Display for DisclosureError {
    fn fmt(&self, formatter: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::Wire(error) => write!(
                formatter,
                "canonical activity rejected at byte {} with result {}",
                error.offset,
                error.result.raw()
            ),
            Self::UnsupportedActivity(activity_type) => {
                write!(
                    formatter,
                    "activity type {activity_type:#010x} has no complete disclosure"
                )
            }
            Self::MalformedPayload => {
                formatter.write_str("activity payload disclosure is malformed")
            }
            Self::PayloadHash => formatter.write_str("payload_hash does not match payload bytes"),
            Self::FieldMismatch(field) => {
                write!(
                    formatter,
                    "disclosure field {field} does not match canonical bytes"
                )
            }
        }
    }
}

impl std::error::Error for DisclosureError {}

impl From<WireError> for DisclosureError {
    fn from(error: WireError) -> Self {
        Self::Wire(error)
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
struct SendSemantics {
    from: [u8; 32],
    to: [u8; 32],
    asset: [u8; 32],
    amount: u128,
    sequence: u64,
    idempotency_key: [u8; 32],
    expires_at: u64,
}

fn fixed<const N: usize>(decoder: &mut Decoder<'_>) -> Result<[u8; N], DisclosureError> {
    decoder
        .fixed(N)
        .map_err(DisclosureError::from)?
        .try_into()
        .map_err(|_| DisclosureError::MalformedPayload)
}

fn decode_send(payload: &[u8]) -> Result<SendSemantics, DisclosureError> {
    let mut decoder = Decoder::new(payload, 0);
    if decoder.u16()? != SEND_WIRE_TAG || decoder.u16()? != SEND_FIELD_COUNT {
        return Err(DisclosureError::MalformedPayload);
    }
    let from = fixed(&mut decoder)?;
    let to = fixed(&mut decoder)?;
    let asset = fixed(&mut decoder)?;
    let amount = decoder.u128()?;
    let sequence = decoder.u64()?;
    let idempotency_key = fixed(&mut decoder)?;
    let expires_at = decoder.u64()?;
    let context_hash: [u8; 32] = fixed(&mut decoder)?;
    let condition_count = usize::from(decoder.u8()?);
    if condition_count > MAX_SEND_CONDITIONS {
        return Err(DisclosureError::MalformedPayload);
    }
    for _ in 0..condition_count {
        if !matches!(decoder.u8()?, 1 | 2) {
            return Err(DisclosureError::MalformedPayload);
        }
        let _ = decoder.u64()?;
    }
    if !(1..=6).contains(&decoder.u8()?) {
        return Err(DisclosureError::MalformedPayload);
    }
    let controller: [u8; 32] = fixed(&mut decoder)?;
    let public_key: [u8; 32] = fixed(&mut decoder)?;
    let signature: [u8; 64] = fixed(&mut decoder)?;
    let signed_context_hash: [u8; 32] = fixed(&mut decoder)?;
    let network_id = decoder.u32()?;
    let protocol_version = decoder.u16()?;
    decoder.finish()?;

    if controller != from
        || signed_context_hash != context_hash
        || network_id == 0
        || !layerx_wire::limits::protocol_version_supported(protocol_version)
    {
        return Err(DisclosureError::MalformedPayload);
    }
    let authorization_offset = payload
        .len()
        .checked_sub(167)
        .ok_or(DisclosureError::MalformedPayload)?;
    let mut h = Sha256::new();
    h.update(hash::Domain::SignaturePreimage.tag());
    h.update(&payload[..2]);
    h.update(&payload[4..authorization_offset + 33]);
    h.update(&payload[authorization_offset + 129..]);
    crate::ed25519::verify_digest(&public_key, &signature, &h.finalize().into())
        .map_err(|_| DisclosureError::MalformedPayload)?;
    Ok(SendSemantics {
        from,
        to,
        asset,
        amount,
        sequence,
        idempotency_key,
        expires_at,
    })
}

fn decode_bridge_deposit_credit(
    payload: &[u8],
    activity: &Activity,
) -> Result<SendSemantics, DisclosureError> {
    if payload.len() < 368
        || payload.len() > layerx_types::limits::MAX_PAYLOAD_BYTES
        || &payload[..5] != b"LXDC3"
        || &payload[363..368] != b"LXLB1"
        || payload[327..359] != Sha256::digest(&payload[363..])[..]
        || payload[359..363] != 2_u32.to_be_bytes()
        || payload[37..41] != activity.network_id().to_be_bytes()
        || payload[41..43] != activity.protocol_version().to_be_bytes()
        || payload[107..139].iter().all(|byte| *byte == 0)
        || payload[171..191].iter().all(|byte| *byte == 0)
        || payload[191..207].iter().all(|byte| *byte == 0)
    {
        return Err(DisclosureError::MalformedPayload);
    }
    let mut nullifier = Sha256::new();
    nullifier.update(b"LX:DEPOSIT:NULLIFIER:v1");
    nullifier.update(&payload[43..75]);
    let idempotency_key: [u8; 32] = nullifier.finalize().into();
    if idempotency_key != activity.idempotency_key() {
        return Err(DisclosureError::MalformedPayload);
    }
    let mut from = [0; 32];
    from[12..].copy_from_slice(&payload[171..191]);
    let to = payload[107..139]
        .try_into()
        .map_err(|_| DisclosureError::MalformedPayload)?;
    let asset = payload[75..107]
        .try_into()
        .map_err(|_| DisclosureError::MalformedPayload)?;
    let amount = u128::from_be_bytes(
        payload[191..207]
            .try_into()
            .map_err(|_| DisclosureError::MalformedPayload)?,
    );
    Ok(SendSemantics {
        from,
        to,
        asset,
        amount,
        sequence: activity.account_sequence(),
        idempotency_key: activity.idempotency_key(),
        expires_at: activity.timestamp_bound().not_after,
    })
}

fn disclose_party(
    counterparties: &mut Vec<Counterparty>,
    role: CounterpartyRole,
    account: [u8; 32],
) {
    counterparties.push(Counterparty { role, account });
}

fn disclose_amount(amounts: &mut Vec<DisclosedAmount>, role: AmountRole, value: u128) {
    amounts.push(DisclosedAmount { role, value });
}

fn disclose_grant(
    grant: &Grant,
    counterparties: &mut Vec<Counterparty>,
    amounts: &mut Vec<DisclosedAmount>,
) {
    disclose_party(counterparties, CounterpartyRole::Payer, grant.from);
    disclose_party(counterparties, CounterpartyRole::Recipient, grant.recipient);
    disclose_grant_amounts(grant, amounts);
}

fn disclose_grant_amounts(grant: &Grant, amounts: &mut Vec<DisclosedAmount>) {
    disclose_amount(amounts, AmountRole::PerDrawMaximum, grant.per_draw_maximum);
    disclose_amount(amounts, AmountRole::GrantAllowance, grant.allowance);
}

fn payment_monetary_fields(
    payment: &Payment,
) -> (Vec<Counterparty>, Vec<DisclosedAmount>, [u8; 32]) {
    let mut counterparties = Vec::new();
    let mut amounts = Vec::new();
    let asset = match payment {
        Payment::Register(registration) => {
            disclose_amount(&mut amounts, AmountRole::SupplyCap, registration.supply_cap);
            registration.asset
        }
        Payment::OpenAccount { asset }
        | Payment::Pause { asset }
        | Payment::Unpause { asset }
        | Payment::ProgramAccount { asset, .. } => *asset,
        Payment::Receive {
            from,
            to,
            asset,
            amount,
            payer_grant,
            ..
        } => {
            disclose_party(&mut counterparties, CounterpartyRole::Payer, *from);
            disclose_party(&mut counterparties, CounterpartyRole::Recipient, *to);
            disclose_amount(&mut amounts, AmountRole::Transfer, *amount);
            disclose_grant_amounts(payer_grant, &mut amounts);
            *asset
        }
        Payment::Mint { asset, to, amount } => {
            disclose_party(&mut counterparties, CounterpartyRole::Recipient, *to);
            disclose_amount(&mut amounts, AmountRole::Transfer, *amount);
            *asset
        }
        Payment::Burn {
            asset,
            from,
            amount,
        } => {
            disclose_party(&mut counterparties, CounterpartyRole::Payer, *from);
            disclose_amount(&mut amounts, AmountRole::Transfer, *amount);
            *asset
        }
        Payment::IssueGrant(grant) => {
            disclose_grant(grant, &mut counterparties, &mut amounts);
            grant.asset
        }
        Payment::ProgramTransfer { legs, .. } => {
            for leg in legs {
                disclose_party(&mut counterparties, CounterpartyRole::Payer, leg.from);
                disclose_party(&mut counterparties, CounterpartyRole::Recipient, leg.to);
                disclose_amount(&mut amounts, AmountRole::Transfer, leg.amount);
            }
            [0; 32]
        }
        Payment::RevokeGrant { .. } => [0; 32],
    };
    (counterparties, amounts, asset)
}

struct DisclosureFields {
    activity_type: ActivityType,
    actor: Vec<u8>,
    authority: Vec<u8>,
    counterparties: Vec<Counterparty>,
    amounts: Vec<DisclosedAmount>,
    asset: [u8; 32],
    fee_limit: u128,
    expiry: Expiry,
    idempotency_key: [u8; 32],
    evm_payout_binding: Option<DisclosedEvmPayoutBinding>,
    withdrawal: Option<DisclosedWithdrawal>,
    payment: Option<Payment>,
    authority_grant: Option<AuthorityGrant>,
    session_grant: Option<DisclosedSessionGrant>,
    onboarding: Option<DisclosedOnboarding>,
    native_operation: Option<DisclosedNativeOperation>,
}

fn pay_exact_fields(
    call: &Eip1559Call,
    from: [u8; 20],
) -> Result<DisclosureFields, DisclosureError> {
    let payment = call::decode_pay_exact(call)?;
    Ok(DisclosureFields {
        activity_type: ActivityType::new(ModuleId::Asset, ASSET_SEND_ORDINAL)
            .map_err(|_| DisclosureError::MalformedPayload)?,
        actor: from.to_vec(),
        authority: from.to_vec(),
        counterparties: vec![
            Counterparty {
                role: CounterpartyRole::Payer,
                account: payment.payer,
            },
            Counterparty {
                role: CounterpartyRole::Recipient,
                account: payment.recipient,
            },
        ],
        amounts: vec![DisclosedAmount {
            role: AmountRole::Transfer,
            value: payment.amount,
        }],
        asset: payment.asset,
        fee_limit: u128::from(call.gas_limit)
            .checked_mul(call.max_fee_per_gas)
            .ok_or(DisclosureError::MalformedPayload)?,
        expiry: Expiry {
            not_before: 0,
            not_after: u64::MAX,
            payload_expires_at: payment.expires_at_ms,
        },
        idempotency_key: payment.idempotency_key,
        evm_payout_binding: None,
        withdrawal: None,
        payment: None,
        authority_grant: None,
        session_grant: None,
        onboarding: None,
        native_operation: None,
    })
}

impl Disclosure {
    #[must_use]
    pub fn canonical_payload(&self) -> &[u8] {
        match &self.source {
            Source::Call(call) => &call.data,
        }
    }

    #[must_use]
    pub const fn envelope_sequence(&self) -> u64 {
        match &self.source {
            Source::Call(call) => call.nonce,
        }
    }

    /// # Errors
    /// Returns a payload decoding error for malformed canonical semantics.
    pub fn payload_sequence(&self) -> Result<Option<u64>, DisclosureError> {
        let Source::Call(call) = &self.source;
        if call::is_pay_exact(call) {
            call::decode_pay_exact(call)?;
            return Ok(None);
        }
        call_codec::payload_sequence(call)
    }

    fn validate_fields(&self) -> Result<(), DisclosureError> {
        let expected = match &self.source {
            Source::Call(call) if call::is_pay_exact(call) => pay_exact_fields(
                call,
                self.call_sender.ok_or(DisclosureError::MalformedPayload)?,
            )?,
            Source::Call(call) => call_codec::fields(
                call,
                self.call_sender.ok_or(DisclosureError::MalformedPayload)?,
            )?,
        };
        macro_rules! require_field {
            ($field:ident) => {
                if self.$field != expected.$field {
                    return Err(DisclosureError::FieldMismatch(stringify!($field)));
                }
            };
        }
        require_field!(activity_type);
        require_field!(actor);
        require_field!(authority);
        require_field!(counterparties);
        require_field!(amounts);
        require_field!(asset);
        require_field!(fee_limit);
        require_field!(expiry);
        require_field!(idempotency_key);
        require_field!(withdrawal);
        require_field!(authority_grant);
        require_field!(session_grant);
        require_field!(onboarding);
        require_field!(native_operation);
        require_field!(evm_payout_binding);
        require_field!(payment);
        Ok(())
    }

    /// Checks every public semantic field against the retained producer bytes.
    ///
    /// # Errors
    /// Refuses altered semantics or a signing digest inconsistent with the source.
    pub fn validate(&self) -> Result<(), DisclosureError> {
        self.validated_digest().map(|_| ())
    }

    /// Checks canonical transaction bytes against the disclosure.
    ///
    /// # Errors
    /// Refuses changed semantics, another source kind, or different canonical bytes.
    pub fn validate_canonical(&self, canonical: &[u8]) -> Result<(), DisclosureError> {
        if self.reencode()? != canonical {
            return Err(DisclosureError::FieldMismatch("canonical_bytes"));
        }
        Ok(())
    }

    /// Re-encodes the transaction represented by this disclosure.
    ///
    /// # Errors
    ///
    /// Names the first changed semantic field or returns a typed wire error.
    pub fn reencode(&self) -> Result<Vec<u8>, DisclosureError> {
        self.validate_fields()?;
        match &self.source {
            Source::Call(call) => Ok(call.encode_unsigned()),
        }
    }

    /// Computes a domain-separated audit digest over the validated disclosure form.
    ///
    /// # Errors
    ///
    /// Returns the first semantic mismatch or canonical transport encoding failure.
    pub fn audit_digest(&self) -> Result<[u8; 32], DisclosureError> {
        let transport = self.transport_bytes()?;
        let mut hasher = Sha256::new();
        hasher.update(b"LXP/agent/disclosure/v1\0");
        hasher.update(transport);
        Ok(hasher.finalize().into())
    }

    pub(crate) fn transport_bytes(&self) -> Result<Vec<u8>, DisclosureError> {
        self.validate_fields()?;
        let mut encoder = Encoder::new(MAX_TRANSPORT_DISCLOSURE_BYTES);
        encoder.structure_header(0x4453)?;
        encoder.u8(
            if matches!(
                self.native_operation,
                Some(
                    DisclosedNativeOperation::BudgetFund(_)
                        | DisclosedNativeOperation::BudgetDefund(_)
                        | DisclosedNativeOperation::BudgetRevoke(_)
                )
            ) {
                5
            } else if self.onboarding.is_some() || self.native_operation.is_some() {
                4
            } else {
                3
            },
        )?;
        encoder.u64(self.envelope_sequence())?;
        match self.payload_sequence()? {
            Some(sequence) => {
                encoder.u8(1)?;
                encoder.u64(sequence)?;
            }
            None => encoder.u8(0)?,
        }
        encoder.u32(self.activity_type.value())?;
        encoder.bytes(&self.actor, 255)?;
        encoder.bytes(&self.authority, 524_288)?;
        encoder.sequence_length(self.counterparties.len(), 64)?;
        for counterparty in &self.counterparties {
            encoder.u8(match counterparty.role {
                CounterpartyRole::Payer => 1,
                CounterpartyRole::Recipient => 2,
            })?;
            encoder.fixed(&counterparty.account)?;
        }
        encoder.sequence_length(self.amounts.len(), 64)?;
        for amount in &self.amounts {
            encoder.u8(match amount.role {
                AmountRole::Transfer => 1,
                AmountRole::SpendingLimit => 2,
                AmountRole::SupplyCap => 3,
                AmountRole::PerDrawMaximum => 4,
                AmountRole::GrantAllowance => 5,
            })?;
            encoder.u128(amount.value)?;
        }
        encoder.fixed(&self.asset)?;
        encoder.u128(self.fee_limit)?;
        encoder.u64(self.expiry.not_before)?;
        encoder.u64(self.expiry.not_after)?;
        encoder.u64(self.expiry.payload_expires_at)?;
        encoder.fixed(&self.idempotency_key)?;
        match self.evm_payout_binding {
            Some(binding) => {
                encoder.u8(1)?;
                encoder.fixed(&binding.did_id)?;
                encoder.u32(binding.network_id)?;
                encoder.fixed(&binding.payout_address)?;
                encoder.fixed(&binding.ownership_signature_digest)?;
            }
            None => encoder.u8(0)?,
        }
        if let Some(withdrawal) = self.withdrawal {
            encoder.u64(withdrawal.account_sequence)?;
            encoder.fixed(&withdrawal.evm_recipient)?;
            encoder.fixed(&withdrawal.request_anchor)?;
        }
        if let Some(grant) = self.authority_grant {
            encoder.bytes(
                &grant
                    .encode()
                    .map_err(|_| DisclosureError::MalformedPayload)?,
                1024,
            )?;
        }
        if let Some(payment) = &self.payment {
            encoder.bytes(&payment.encode(&self.actor)?, 32768)?;
        }
        if let Some(session) = &self.session_grant {
            encoder.bytes(&session.grant.registration_payload, 1024)?;
            encoder.u64(session.expiry_sequence)?;
            encoder.fixed(&session.action_key)?;
            if let Some(replacement) = session.replacement {
                encoder.fixed(&replacement.predecessor_grant_id)?;
                encoder.fixed(&replacement.expected_charge_state)?;
            }
        }
        if let Some(onboarding) = &self.onboarding {
            encoder.bytes(&onboarding.encode()?, 2048)?;
        }
        if let Some(operation) = &self.native_operation {
            encoder.bytes(&operation.encode()?, 2048)?;
        }
        let Source::Call(call) = &self.source;
        encoder.fixed(&call.signing_hash())?;
        encoder.bytes(&call.data, MAX_TRANSPORT_DISCLOSURE_BYTES)?;
        Ok(encoder.finish())
    }

    /// Returns the signing digest after checking every disclosed field.
    ///
    /// # Errors
    /// Refuses any semantic field that differs from its source bytes.
    pub fn signing_digest(&self) -> Result<[u8; 32], DisclosureError> {
        self.validated_digest()
    }

    /// Returns all ABI arguments for a construct call, including grant and bank fields.
    ///
    /// # Errors
    /// Refuses malformed calldata.
    pub fn construct_method(&self) -> Result<Option<call::ConstructMethod>, DisclosureError> {
        self.validate()?;
        match &self.source {
            Source::Call(call) => call_codec::decode_method(call).map(Some),
        }
    }

    pub(crate) fn validated_digest(&self) -> Result<[u8; 32], DisclosureError> {
        self.validate_fields()?;
        let digest = match &self.source {
            Source::Call(call) => call.signing_hash(),
        };
        if !ct::eq_fixed(&digest, &self.signing_digest) {
            return Err(DisclosureError::FieldMismatch("canonical_bytes"));
        }
        Ok(digest)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use layerx_types::activity::{Authority, EnvelopeBuilder, TimestampBound as EnvelopeBound};
    use layerx_types::amount::Amount;
    use layerx_types::ids::{Did, IdempotencyKey};
    use layerx_types::payload::{ModuleRegistration, ModuleRegistry, Payload};
    use layerx_wire::historical::activity::encode_unsigned_envelope;

    fn custody_credit_payload(length: usize) -> Vec<u8> {
        let mut payload = vec![0; length];
        payload[..5].copy_from_slice(b"LXDC3");
        payload[37..41].copy_from_slice(&17_u32.to_be_bytes());
        payload[41..43].copy_from_slice(&3_u16.to_be_bytes());
        payload[43..75].fill(0x11);
        payload[75..107].fill(0x33);
        payload[107..139].fill(0x22);
        payload[139..171].fill(0x44);
        payload[171..191].fill(0x55);
        payload[191..207].copy_from_slice(&25_u128.to_be_bytes());
        payload[359..363].copy_from_slice(&2_u32.to_be_bytes());
        payload[363..368].copy_from_slice(b"LXLB1");
        let proof_hash = Sha256::digest(&payload[363..]);
        payload[327..359].copy_from_slice(&proof_hash);
        payload
    }

    fn custody_credit_envelope(
        payload: &[u8],
        protocol: u16,
    ) -> Result<(Vec<u8>, ModuleRegistry), String> {
        let kind = ActivityType::new(ModuleId::Bridge, 1).map_err(|e| format!("{e:?}"))?;
        let registration =
            ModuleRegistration::new(ModuleId::Bridge, &[kind]).map_err(|e| format!("{e:?}"))?;
        let registry = ModuleRegistry::new(&[registration]).map_err(|e| format!("{e:?}"))?;
        let payload = Payload::new(&registry, kind, payload).map_err(|e| format!("{e:?}"))?;
        let actor = Did::new(b"did:layerx:holder").map_err(|e| format!("{e:?}"))?;
        let mut nullifier = Sha256::new();
        nullifier.update(b"LX:DEPOSIT:NULLIFIER:v1");
        nullifier.update([0x11; 32]);
        let mut builder = EnvelopeBuilder::new();
        builder
            .protocol_version(protocol)
            .and_then(|b| b.network_id(17))
            .and_then(|b| b.activity_type(kind))
            .and_then(|b| b.actor_did(actor))
            .and_then(|b| b.authority(Authority::protocol_module(&[0x44; 32])?))
            .and_then(|b| b.account_sequence(7))
            .and_then(|b| b.timestamp_bound(EnvelopeBound::new(10, 100)?))
            .and_then(|b| b.idempotency_key(IdempotencyKey::new(nullifier.finalize().into())))
            .and_then(|b| b.fee_limit(Amount::from_u128(20)))
            .map_err(|e| format!("{e:?}"))?;
        builder
            .payload_hash(hash::payload_hash_for(&payload).map_err(|e| format!("{e:?}"))?)
            .and_then(|b| b.payload(payload))
            .map_err(|e| format!("{e:?}"))?;
        let envelope = builder.build().map_err(|e| format!("{e:?}"))?;
        let canonical = encode_unsigned_envelope(&envelope).map_err(|e| format!("{e:?}"))?;
        Ok((canonical, registry))
    }

    #[test]
    fn custody_credit_discloses_at_every_supported_length_and_scope() -> Result<(), String> {
        for length in [368, 512, 513, layerx_types::limits::MAX_PAYLOAD_BYTES] {
            for protocol in [1_u16, 2, 3] {
                let mut payload = custody_credit_payload(length);
                payload[41..43].copy_from_slice(&protocol.to_be_bytes());
                let (canonical, registry) = custody_credit_envelope(&payload, protocol)?;
                let activity =
                    layerx_wire::historical::activity::decode_unsigned(&canonical, &registry)
                        .map_err(|e| format!("{e:?}"))?;
                let disclosure = decode_bridge_deposit_credit(activity.payload(), &activity)
                    .map_err(|e| format!("{e:?}"))?;
                let mut payer = [0; 32];
                payer[12..].copy_from_slice(&payload[171..191]);
                assert_eq!(disclosure.from, payer);
                assert_eq!(disclosure.to, payload[107..139]);
                assert_eq!(disclosure.asset, payload[75..107]);
                assert_eq!(disclosure.amount.to_be_bytes(), payload[191..207]);
                let mut nullifier = Sha256::new();
                nullifier.update(b"LX:DEPOSIT:NULLIFIER:v1");
                nullifier.update(&payload[43..75]);
                assert_eq!(disclosure.idempotency_key, nullifier.finalize()[..]);
                assert_eq!(disclosure.sequence, 7);
                assert_eq!(disclosure.expires_at, 100);
                assert_eq!(
                    layerx_wire::historical::activity::encode_unsigned(&activity),
                    Ok(canonical)
                );
            }
        }
        Ok(())
    }

    #[test]
    fn custody_credit_refuses_inconsistent_scope_bundle_and_nullifier() -> Result<(), String> {
        for offset in [40, 42, 327, 43, 0, 359, 363] {
            let mut payload = custody_credit_payload(368);
            payload[offset] ^= 1;
            let (canonical, registry) = custody_credit_envelope(&payload, 3)?;
            assert_eq!(
                decode_bridge_deposit_credit(
                    layerx_wire::historical::activity::decode_unsigned(&canonical, &registry)
                        .map_err(|e| format!("{e:?}"))?
                        .payload(),
                    &layerx_wire::historical::activity::decode_unsigned(&canonical, &registry)
                        .map_err(|e| format!("{e:?}"))?,
                )
                .err(),
                Some(DisclosureError::MalformedPayload),
                "altered byte {offset}"
            );
        }
        for range in [107..139, 171..191, 191..207] {
            let mut payload = custody_credit_payload(368);
            payload[range.clone()].fill(0);
            let (canonical, registry) = custody_credit_envelope(&payload, 3)?;
            assert_eq!(
                decode_bridge_deposit_credit(
                    layerx_wire::historical::activity::decode_unsigned(&canonical, &registry)
                        .map_err(|e| format!("{e:?}"))?
                        .payload(),
                    &layerx_wire::historical::activity::decode_unsigned(&canonical, &registry)
                        .map_err(|e| format!("{e:?}"))?,
                )
                .err(),
                Some(DisclosureError::MalformedPayload),
                "zero field {range:?}"
            );
        }
        let payload = custody_credit_payload(368);
        for length in 0..368 {
            let (canonical, registry) = custody_credit_envelope(&payload[..length], 3)?;
            assert_eq!(
                decode_bridge_deposit_credit(
                    layerx_wire::historical::activity::decode_unsigned(&canonical, &registry)
                        .map_err(|e| format!("{e:?}"))?
                        .payload(),
                    &layerx_wire::historical::activity::decode_unsigned(&canonical, &registry)
                        .map_err(|e| format!("{e:?}"))?,
                )
                .err(),
                Some(DisclosureError::MalformedPayload),
                "truncated length {length}"
            );
        }
        Ok(())
    }

    #[test]
    fn custody_credit_refuses_legacy_tlv_framing() -> Result<(), String> {
        let mut payload = vec![0x11; 212];
        payload[..4].copy_from_slice(&[0x48, 0x01, 0x00, 0x07]);
        payload[68..100].fill(0x55);
        payload[100..132].fill(0x22);
        payload[132..164].fill(0x33);
        payload[164..180].copy_from_slice(&25_u128.to_be_bytes());
        let mut nullifier = Sha256::new();
        nullifier.update(b"LX:DEPOSIT:NULLIFIER:v1");
        nullifier.update([0x11; 32]);
        payload[180..212].copy_from_slice(&nullifier.finalize());
        let (canonical, registry) = custody_credit_envelope(&payload, 3)?;
        assert_eq!(
            decode_bridge_deposit_credit(
                layerx_wire::historical::activity::decode_unsigned(&canonical, &registry)
                    .map_err(|e| format!("{e:?}"))?
                    .payload(),
                &layerx_wire::historical::activity::decode_unsigned(&canonical, &registry)
                    .map_err(|e| format!("{e:?}"))?,
            )
            .err(),
            Some(DisclosureError::MalformedPayload)
        );
        Ok(())
    }

    #[test]
    fn custody_credit_refusal_preserves_payload_commitment() -> Result<(), String> {
        let (mut canonical, registry) = custody_credit_envelope(&custody_credit_payload(368), 3)?;
        let last = canonical.last_mut().ok_or("empty canonical envelope")?;
        *last ^= 1;
        let activity = layerx_wire::historical::activity::decode_unsigned(&canonical, &registry)
            .map_err(|e| format!("{e:?}"))?;
        assert_ne!(
            layerx_wire::historical::hash::payload_hash(&activity).map_err(|e| format!("{e:?}"))?,
            activity.payload_hash()
        );
        assert_eq!(
            decode_bridge_deposit_credit(activity.payload(), &activity).err(),
            Some(DisclosureError::MalformedPayload)
        );
        Ok(())
    }
}
