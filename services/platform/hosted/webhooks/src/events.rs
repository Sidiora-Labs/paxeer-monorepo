//! Protocol event identity, per-subject ordering, and the exact verification
//! vocabulary the human plane displays on every protocol fact.

use std::fmt::{Display, Formatter};

use layerx_platform_gateway::{PrincipalId, VerifiedOperation};
use serde::{Deserialize, Serialize};

use crate::encoding::{digest, hex_encode, is_digest};
use crate::error::WebhookError;

const MAXIMUM_TOKEN: usize = 128;
const MAXIMUM_VALUE: usize = 512;
const MAXIMUM_FACTS: usize = 32;

/// The presentation levels the human plane displays, in lattice order. A value
/// never claims a level stronger than the weakest evidence behind it.
#[derive(Clone, Copy, Debug, Eq, Ord, PartialEq, PartialOrd, Serialize, Deserialize)]
#[serde(rename_all = "kebab-case")]
pub enum Verification {
    /// No protocol evidence has been checked.
    Unverified,
    /// A sequencer-signed `LayerX` receipt verified.
    ReceiptVerified,
    /// A sequencer receipt included in a committed chain block.
    BlockCommitted,
}

#[doc(hidden)]
pub fn deserialize_stored_verification<'de, D: serde::Deserializer<'de>>(
    d: D,
) -> Result<Verification, D::Error> {
    let word = String::deserialize(d)?;
    match word.as_str() {
        "unverified" => Ok(Verification::Unverified),
        "receipt-verified" => Ok(Verification::ReceiptVerified),
        "block-committed" | "checkpoint-finalised" | "paxeer-finalised" => {
            Ok(Verification::BlockCommitted)
        }
        _ => Err(serde::de::Error::unknown_variant(
            &word,
            &[
                "unverified",
                "receipt-verified",
                "block-committed",
                "checkpoint-finalised",
                "paxeer-finalised",
            ],
        )),
    }
}

impl Verification {
    /// Returns the exact wire word the human plane renders.
    #[must_use]
    pub const fn as_str(self) -> &'static str {
        match self {
            Self::Unverified => "unverified",
            Self::ReceiptVerified => "receipt-verified",
            Self::BlockCommitted => "block-committed",
        }
    }

    /// Parses one wire word from the human plane vocabulary.
    ///
    /// # Errors
    /// Returns [`WebhookError::InvalidRequest`] for a word outside the vocabulary.
    pub fn parse(value: &str) -> Result<Self, WebhookError> {
        match value {
            "unverified" => Ok(Self::Unverified),
            "receipt-verified" => Ok(Self::ReceiptVerified),
            "block-committed" => Ok(Self::BlockCommitted),
            _ => Err(WebhookError::InvalidRequest),
        }
    }

    /// Projects a protocol verification rank onto the human plane vocabulary.
    ///
    /// Sequencer-signed, batch-included and state-proven receipts all present
    /// as `receipt-verified`; ranks 4 and 5 present as `block-committed`.
    #[must_use]
    pub const fn from_wire_rank(rank: u8) -> Self {
        match rank {
            1..=3 => Self::ReceiptVerified,
            4..=5 => Self::BlockCommitted,
            _ => Self::Unverified,
        }
    }

    /// Returns true when this level is at least the required level.
    #[must_use]
    pub const fn at_least(self, required: Self) -> bool {
        (self as u8) >= (required as u8)
    }

    /// Returns true when the level requires receipt evidence to be presented.
    #[must_use]
    pub const fn requires_receipt(self) -> bool {
        !matches!(self, Self::Unverified)
    }
}

/// The event families the hosted surface delivers.
#[derive(Clone, Copy, Debug, Eq, Ord, PartialEq, PartialOrd, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum EventKind {
    /// Human-plane journey progress.
    Journey,
    /// Value movement executed by the protocol.
    Payment,
    /// Approval requested, granted, refused or expired.
    Approval,
    /// Program registration, version and deprecation transitions.
    Program,
}

impl EventKind {
    /// Returns the wire word for this family.
    #[must_use]
    pub const fn as_str(self) -> &'static str {
        match self {
            Self::Journey => "journey",
            Self::Payment => "payment",
            Self::Approval => "approval",
            Self::Program => "program",
        }
    }

    /// Parses one wire word.
    ///
    /// # Errors
    /// Returns [`WebhookError::InvalidRequest`] for an unknown family.
    pub fn parse(value: &str) -> Result<Self, WebhookError> {
        match value {
            "journey" => Ok(Self::Journey),
            "payment" => Ok(Self::Payment),
            "approval" => Ok(Self::Approval),
            "program" => Ok(Self::Program),
            _ => Err(WebhookError::InvalidRequest),
        }
    }
}

impl Display for EventKind {
    fn fmt(&self, formatter: &mut Formatter<'_>) -> std::fmt::Result {
        formatter.write_str(self.as_str())
    }
}

fn valid_token(value: &str) -> bool {
    !value.is_empty()
        && value.len() <= MAXIMUM_TOKEN
        && value
            .bytes()
            .all(|byte| byte.is_ascii_alphanumeric() || matches!(byte, b'-' | b'_' | b'.' | b':'))
}

fn random_token(prefix: &str) -> Result<String, WebhookError> {
    let mut random = [0_u8; 16];
    getrandom::fill(&mut random).map_err(|_| WebhookError::Entropy)?;
    Ok(format!("{prefix}{}", hex_encode(&random)))
}

/// The developer principal an endpoint and its events belong to. Construction
/// reuses the hosted gateway's own identifier rule so the two surfaces cannot
/// disagree about who a record belongs to.
#[derive(Clone, Debug, Eq, Ord, PartialEq, PartialOrd, Serialize, Deserialize)]
pub struct Principal(String);

impl Principal {
    /// Accepts only identifiers the hosted gateway itself accepts.
    ///
    /// # Errors
    /// Returns [`WebhookError::Gateway`] when the gateway refuses the value.
    pub fn new(value: impl Into<String>) -> Result<Self, WebhookError> {
        let value = value.into();
        let _accepted = PrincipalId::new(value.as_str())?;
        Ok(Self(value))
    }

    /// Borrows the identifier.
    #[must_use]
    pub fn as_str(&self) -> &str {
        &self.0
    }

    /// Returns the gateway identifier for the same principal.
    ///
    /// # Errors
    /// Returns [`WebhookError::Gateway`] when the gateway refuses the value.
    pub fn gateway_id(&self) -> Result<PrincipalId, WebhookError> {
        PrincipalId::new(self.0.as_str()).map_err(WebhookError::from)
    }

    /// Returns the audit digest the hosted gateway records for this principal.
    #[must_use]
    pub fn audit_digest(&self) -> [u8; 32] {
        digest(self.0.as_bytes())
    }
}

impl Display for Principal {
    fn fmt(&self, formatter: &mut Formatter<'_>) -> std::fmt::Result {
        formatter.write_str(&self.0)
    }
}

/// The ordering subject a stream of events belongs to.
#[derive(Clone, Debug, Eq, Ord, PartialEq, PartialOrd, Serialize, Deserialize)]
pub struct SubjectId(String);

/// The stable identifier of one registered developer endpoint.
#[derive(Clone, Debug, Eq, Ord, PartialEq, PartialOrd, Serialize, Deserialize)]
pub struct EndpointId(String);

/// The stable identifier of one protocol event.
#[derive(Clone, Debug, Eq, Ord, PartialEq, PartialOrd, Serialize, Deserialize)]
pub struct EventId(String);

/// The stable identifier of one delivery attempt sequence.
#[derive(Clone, Debug, Eq, Ord, PartialEq, PartialOrd, Serialize, Deserialize)]
pub struct DeliveryId(String);

impl SubjectId {
    /// Creates an ordering subject.
    ///
    /// # Errors
    /// Returns [`WebhookError::InvalidRequest`] for an invalid identifier.
    pub fn new(value: impl Into<String>) -> Result<Self, WebhookError> {
        let value = value.into();
        if valid_token(&value) {
            Ok(Self(value))
        } else {
            Err(WebhookError::InvalidRequest)
        }
    }

    /// Borrows the identifier.
    #[must_use]
    pub fn as_str(&self) -> &str {
        &self.0
    }
}

impl EndpointId {
    /// Creates an endpoint identifier.
    ///
    /// # Errors
    /// Returns [`WebhookError::InvalidRequest`] for an invalid identifier.
    pub fn new(value: impl Into<String>) -> Result<Self, WebhookError> {
        let value = value.into();
        if valid_token(&value) {
            Ok(Self(value))
        } else {
            Err(WebhookError::InvalidRequest)
        }
    }

    /// Generates a fresh endpoint identifier.
    ///
    /// # Errors
    /// Returns [`WebhookError::Entropy`] when the system entropy source fails.
    pub fn generate() -> Result<Self, WebhookError> {
        Ok(Self(random_token("whep_")?))
    }

    /// Borrows the identifier.
    #[must_use]
    pub fn as_str(&self) -> &str {
        &self.0
    }
}

impl EventId {
    /// Creates an event identifier.
    ///
    /// # Errors
    /// Returns [`WebhookError::InvalidRequest`] for an invalid identifier.
    pub fn new(value: impl Into<String>) -> Result<Self, WebhookError> {
        let value = value.into();
        if valid_token(&value) {
            Ok(Self(value))
        } else {
            Err(WebhookError::InvalidRequest)
        }
    }

    /// Generates a fresh event identifier.
    ///
    /// # Errors
    /// Returns [`WebhookError::Entropy`] when the system entropy source fails.
    pub fn generate() -> Result<Self, WebhookError> {
        Ok(Self(random_token("whev_")?))
    }

    /// Borrows the identifier.
    #[must_use]
    pub fn as_str(&self) -> &str {
        &self.0
    }
}

impl DeliveryId {
    /// Creates a delivery identifier.
    ///
    /// # Errors
    /// Returns [`WebhookError::InvalidRequest`] for an invalid identifier.
    pub fn new(value: impl Into<String>) -> Result<Self, WebhookError> {
        let value = value.into();
        if valid_token(&value) {
            Ok(Self(value))
        } else {
            Err(WebhookError::InvalidRequest)
        }
    }

    /// Generates a fresh delivery identifier.
    ///
    /// # Errors
    /// Returns [`WebhookError::Entropy`] when the system entropy source fails.
    pub fn generate() -> Result<Self, WebhookError> {
        Ok(Self(random_token("whdl_")?))
    }

    /// Borrows the identifier.
    #[must_use]
    pub fn as_str(&self) -> &str {
        &self.0
    }
}

impl Display for SubjectId {
    fn fmt(&self, formatter: &mut Formatter<'_>) -> std::fmt::Result {
        formatter.write_str(&self.0)
    }
}

impl Display for EndpointId {
    fn fmt(&self, formatter: &mut Formatter<'_>) -> std::fmt::Result {
        formatter.write_str(&self.0)
    }
}

impl Display for EventId {
    fn fmt(&self, formatter: &mut Formatter<'_>) -> std::fmt::Result {
        formatter.write_str(&self.0)
    }
}

impl Display for DeliveryId {
    fn fmt(&self, formatter: &mut Formatter<'_>) -> std::fmt::Result {
        formatter.write_str(&self.0)
    }
}

/// One displayed protocol fact bound to the evidence that established it.
#[derive(Clone, Debug, Eq, PartialEq, Serialize, Deserialize)]
pub struct ProtocolFact {
    name: String,
    value: String,
    #[serde(deserialize_with = "deserialize_stored_verification")]
    verification: Verification,
    receipt_digest: Option<String>,
}

impl ProtocolFact {
    /// Presents a fact established by verified receipt evidence.
    ///
    /// # Errors
    /// Returns [`WebhookError::InvalidRequest`] for an invalid name or value and
    /// [`WebhookError::VerificationRequired`] when the level is unverified or the
    /// receipt digest is not a receipt digest.
    pub(crate) fn verified(
        name: impl Into<String>,
        value: impl Into<String>,
        verification: Verification,
        receipt_digest: impl Into<String>,
    ) -> Result<Self, WebhookError> {
        let receipt_digest = receipt_digest.into();
        if !verification.requires_receipt() || !is_digest(&receipt_digest) {
            return Err(WebhookError::VerificationRequired);
        }
        let fact = Self {
            name: name.into(),
            value: value.into(),
            verification,
            receipt_digest: Some(receipt_digest),
        };
        fact.check_shape()?;
        Ok(fact)
    }

    /// Presents a fact that carries no protocol evidence at all.
    ///
    /// # Errors
    /// Returns [`WebhookError::InvalidRequest`] for an invalid name or value.
    pub fn unverified(
        name: impl Into<String>,
        value: impl Into<String>,
    ) -> Result<Self, WebhookError> {
        let fact = Self {
            name: name.into(),
            value: value.into(),
            verification: Verification::Unverified,
            receipt_digest: None,
        };
        fact.check_shape()?;
        Ok(fact)
    }

    /// Borrows the fact name.
    #[must_use]
    pub fn name(&self) -> &str {
        &self.name
    }

    /// Borrows the presented value.
    #[must_use]
    pub fn value(&self) -> &str {
        &self.value
    }

    /// Returns the level the evidence established.
    #[must_use]
    pub const fn verification(&self) -> Verification {
        self.verification
    }

    /// Borrows the receipt digest when evidence exists.
    #[must_use]
    pub fn receipt_digest(&self) -> Option<&str> {
        self.receipt_digest.as_deref()
    }

    fn check_shape(&self) -> Result<(), WebhookError> {
        if self.name.is_empty()
            || self.name.len() > MAXIMUM_TOKEN
            || self.value.len() > MAXIMUM_VALUE
            || self.value.contains(['\0', '\n', '\r'])
        {
            return Err(WebhookError::InvalidRequest);
        }
        Ok(())
    }

    /// Re-checks that the presented level is supported by presented evidence.
    ///
    /// # Errors
    /// Returns [`WebhookError::VerificationRequired`] when a level above
    /// `unverified` carries no receipt digest, or an unverified fact carries one.
    pub fn validate(&self) -> Result<(), WebhookError> {
        self.check_shape()?;
        match (self.verification.requires_receipt(), &self.receipt_digest) {
            (true, Some(value)) if is_digest(value) => Ok(()),
            (false, None) => Ok(()),
            _ => Err(WebhookError::VerificationRequired),
        }
    }
}

/// The fields a caller supplies to build one protocol event.
#[derive(Clone, Debug)]
pub struct EventDraft {
    /// Stable event identifier used for receiver deduplication.
    pub id: EventId,
    /// Event family.
    pub kind: EventKind,
    /// Owning developer principal.
    pub principal: Principal,
    /// Ordering subject.
    pub subject: SubjectId,
    /// Strictly increasing position within the subject.
    pub subject_sequence: u64,
    /// Protocol observation time in whole seconds.
    pub occurred_at: u64,
    /// Displayed protocol facts, each carrying its own evidence.
    pub facts: Vec<ProtocolFact>,
}

/// One protocol event, ordered inside its subject and carrying the exact
/// verification status of every fact it displays.
#[derive(Clone, Debug, Eq, PartialEq, Serialize, Deserialize)]
pub struct ProtocolEvent {
    id: EventId,
    kind: EventKind,
    principal: Principal,
    subject: SubjectId,
    subject_sequence: u64,
    occurred_at: u64,
    #[serde(deserialize_with = "deserialize_stored_verification")]
    verification: Verification,
    receipt_digest: Option<String>,
    facts: Vec<ProtocolFact>,
}

impl ProtocolEvent {
    /// Builds an event whose presented level is the weakest level among its
    /// facts, so a single unverified fact can never be shown as settled.
    ///
    /// # Errors
    /// Returns [`WebhookError::InvalidRequest`] when the draft carries no facts
    /// or more than the accepted maximum, and [`WebhookError::VerificationRequired`]
    /// when a fact claims a level its evidence does not support.
    pub(crate) fn new(draft: EventDraft) -> Result<Self, WebhookError> {
        if draft.facts.is_empty() || draft.facts.len() > MAXIMUM_FACTS {
            return Err(WebhookError::InvalidRequest);
        }
        for fact in &draft.facts {
            fact.validate()?;
        }
        let verification = draft
            .facts
            .iter()
            .map(ProtocolFact::verification)
            .min()
            .unwrap_or(Verification::Unverified);
        let receipt_digest = draft
            .facts
            .iter()
            .find(|fact| fact.verification == verification)
            .and_then(|fact| fact.receipt_digest.clone());
        Ok(Self {
            id: draft.id,
            kind: draft.kind,
            principal: draft.principal,
            subject: draft.subject,
            subject_sequence: draft.subject_sequence,
            occurred_at: draft.occurred_at,
            verification,
            receipt_digest,
            facts: draft.facts,
        })
    }

    /// Re-derives the presented level from the event's own facts and refuses a
    /// record whose header claims more than its facts establish.
    ///
    /// # Errors
    /// Returns [`WebhookError::InvalidRequest`] for an empty or oversized fact
    /// list and [`WebhookError::VerificationRequired`] when the header level or
    /// receipt digest is not supported by the facts.
    pub fn validate(&self) -> Result<(), WebhookError> {
        if self.facts.is_empty() || self.facts.len() > MAXIMUM_FACTS {
            return Err(WebhookError::InvalidRequest);
        }
        for fact in &self.facts {
            fact.validate()?;
        }
        let weakest = self
            .facts
            .iter()
            .map(ProtocolFact::verification)
            .min()
            .unwrap_or(Verification::Unverified);
        if weakest != self.verification {
            return Err(WebhookError::VerificationRequired);
        }
        match (self.verification.requires_receipt(), &self.receipt_digest) {
            (true, Some(value)) if is_digest(value) => Ok(()),
            (false, None) => Ok(()),
            _ => Err(WebhookError::VerificationRequired),
        }
    }

    /// Borrows the event identifier.
    #[must_use]
    pub const fn id(&self) -> &EventId {
        &self.id
    }

    /// Returns the event family.
    #[must_use]
    pub const fn kind(&self) -> EventKind {
        self.kind
    }

    /// Borrows the owning principal.
    #[must_use]
    pub const fn principal(&self) -> &Principal {
        &self.principal
    }

    /// Borrows the ordering subject.
    #[must_use]
    pub const fn subject(&self) -> &SubjectId {
        &self.subject
    }

    /// Returns the position of this event inside its subject.
    #[must_use]
    pub const fn subject_sequence(&self) -> u64 {
        self.subject_sequence
    }

    /// Returns the protocol observation time in whole seconds.
    #[must_use]
    pub const fn occurred_at(&self) -> u64 {
        self.occurred_at
    }

    /// Returns the weakest level among the displayed facts.
    #[must_use]
    pub const fn verification(&self) -> Verification {
        self.verification
    }

    /// Borrows the receipt digest backing the presented level.
    #[must_use]
    pub fn receipt_digest(&self) -> Option<&str> {
        self.receipt_digest.as_deref()
    }

    /// Borrows the displayed facts.
    #[must_use]
    pub fn facts(&self) -> &[ProtocolFact] {
        &self.facts
    }
}

/// The fields a caller supplies to present one settled payment.
#[derive(Clone, Debug)]
pub struct PaymentDraft<'a> {
    /// Stable event identifier.
    pub id: EventId,
    /// Owning developer principal.
    pub principal: Principal,
    /// Ordering subject, normally the payment or account reference.
    pub subject: SubjectId,
    /// Strictly increasing position within the subject.
    pub subject_sequence: u64,
    /// Protocol observation time in whole seconds.
    pub occurred_at: u64,
    /// The receipt-backed operation the hosted gateway returned.
    pub operation: &'a VerifiedOperation,
    /// Exact protocol amount from the canonical payment source. This source
    /// annotation is not promoted to receipt-verified evidence.
    pub amount: String,
    /// Protocol asset identifier from the canonical payment source. This
    /// source annotation is not promoted to receipt-verified evidence.
    pub asset: String,
}

/// Builds a payment event whose settlement state and activity identity are
/// backed by real receipt bytes at `receipt-verified` or stronger. Amount and
/// asset remain visibly unverified source annotations.
///
/// # Errors
/// Returns [`WebhookError::VerificationRequired`] when the operation carries no
/// receipt bytes or a level weaker than a verified `LayerX` receipt, and
/// [`WebhookError::InvalidRequest`] for an invalid amount or asset.
pub fn settled_payment(draft: PaymentDraft<'_>) -> Result<ProtocolEvent, WebhookError> {
    let level = Verification::parse(draft.operation.verification_level())?;
    settled_payment_at(draft, level)
}

pub(crate) fn settled_payment_at(
    draft: PaymentDraft<'_>,
    verification: Verification,
) -> Result<ProtocolEvent, WebhookError> {
    if draft.operation.receipt().is_empty() {
        return Err(WebhookError::VerificationRequired);
    }
    if draft.operation.result_code() != 0 {
        return Err(WebhookError::VerificationRequired);
    }
    if !verification.at_least(Verification::ReceiptVerified) {
        return Err(WebhookError::VerificationRequired);
    }
    if draft.amount.is_empty() || !draft.amount.bytes().all(|byte| byte.is_ascii_digit()) {
        return Err(WebhookError::InvalidRequest);
    }
    let receipt_digest = hex_encode(&draft.operation.receipt_digest());
    let facts = vec![
        ProtocolFact::verified("state", "settled", verification, receipt_digest.as_str())?,
        ProtocolFact::unverified("amount", draft.amount)?,
        ProtocolFact::unverified("asset", draft.asset)?,
        ProtocolFact::verified(
            "activity_id",
            hex_encode(&draft.operation.activity_id()),
            verification,
            receipt_digest.as_str(),
        )?,
        ProtocolFact::verified(
            "receipt_bytes",
            draft.operation.receipt().len().to_string(),
            verification,
            receipt_digest.as_str(),
        )?,
    ];
    ProtocolEvent::new(EventDraft {
        id: draft.id,
        kind: EventKind::Payment,
        principal: draft.principal,
        subject: draft.subject,
        subject_sequence: draft.subject_sequence,
        occurred_at: draft.occurred_at,
        facts,
    })
}

#[cfg(test)]
mod tests {
    use super::{
        EventDraft, EventId, EventKind, Principal, ProtocolEvent, ProtocolFact, SubjectId,
        Verification,
    };
    use crate::error::WebhookError;

    #[test]
    fn stored_retired_verification_words_decode_as_block_committed() {
        let event = ProtocolEvent::new(EventDraft {
            id: EventId::new("stored-event")
                .unwrap_or_else(|error| panic!("valid event id: {error:?}")),
            kind: EventKind::Payment,
            principal: Principal::new("stored-principal")
                .unwrap_or_else(|error| panic!("valid principal: {error:?}")),
            subject: SubjectId::new("stored-subject")
                .unwrap_or_else(|error| panic!("valid subject: {error:?}")),
            subject_sequence: 1,
            occurred_at: 1,
            facts: vec![ProtocolFact::verified(
                "payment",
                "settled",
                Verification::BlockCommitted,
                "ab".repeat(32),
            )
            .unwrap_or_else(|error| panic!("valid committed fact: {error:?}"))],
        })
        .unwrap_or_else(|error| panic!("valid committed event: {error:?}"));
        let canonical = serde_json::to_value(&event)
            .unwrap_or_else(|error| panic!("event serializes: {error:?}"));

        for event_word in ["checkpoint-finalised", "paxeer-finalised"] {
            assert!(matches!(
                Verification::parse(event_word),
                Err(WebhookError::InvalidRequest)
            ));
            assert!(serde_json::from_value::<Verification>(serde_json::json!(event_word)).is_err());

            for fact_word in ["checkpoint-finalised", "paxeer-finalised"] {
                let mut stored = canonical.clone();
                stored["verification"] = serde_json::json!(event_word);
                stored["facts"][0]["verification"] = serde_json::json!(fact_word);
                let decoded: ProtocolEvent = serde_json::from_value(stored.clone())
                    .unwrap_or_else(|error| panic!("retired stored words decode: {error:?}"));
                assert_eq!(decoded.verification(), Verification::BlockCommitted);
                assert_eq!(
                    decoded.facts()[0].verification(),
                    Verification::BlockCommitted
                );
                decoded.validate().unwrap_or_else(|error| {
                    panic!("decoded event retains its evidence: {error:?}")
                });
                let serialized = serde_json::to_value(&decoded)
                    .unwrap_or_else(|error| panic!("decoded event serializes: {error:?}"));
                assert_eq!(serialized["verification"], "block-committed");
                assert_eq!(serialized["facts"][0]["verification"], "block-committed");
                assert_eq!(serialized, canonical);

                let mut unknown_event = stored.clone();
                unknown_event["verification"] = serde_json::json!("finalised-forever");
                assert!(serde_json::from_value::<ProtocolEvent>(unknown_event).is_err());
                stored["facts"][0]["verification"] = serde_json::json!("finalised-forever");
                assert!(serde_json::from_value::<ProtocolEvent>(stored).is_err());
            }
        }

        for level in [
            Verification::Unverified,
            Verification::ReceiptVerified,
            Verification::BlockCommitted,
        ] {
            let mut stored = canonical.clone();
            stored["verification"] = serde_json::json!(level.as_str());
            stored["facts"][0]["verification"] = serde_json::json!(level.as_str());
            let decoded: ProtocolEvent = serde_json::from_value(stored)
                .unwrap_or_else(|error| panic!("current stored words decode: {error:?}"));
            assert_eq!(decoded.verification(), level);
            assert_eq!(decoded.facts()[0].verification(), level);
        }
    }

    #[test]
    fn block_committed_is_the_top_verification_above_receipt_verified() {
        let levels = [
            (Verification::Unverified, "unverified"),
            (Verification::ReceiptVerified, "receipt-verified"),
            (Verification::BlockCommitted, "block-committed"),
        ];
        for (rank, (level, word)) in levels.iter().enumerate() {
            assert_eq!(level.as_str(), *word);
            let parsed = Verification::parse(level.as_str())
                .unwrap_or_else(|error| panic!("wire level round-trips: {error:?}"));
            assert_eq!(parsed, *level);
            for (required_rank, (required, required_word)) in levels.iter().enumerate() {
                let parsed_required = Verification::parse(required_word)
                    .unwrap_or_else(|error| panic!("required wire level round-trips: {error:?}"));
                assert_eq!(parsed_required, *required);
                assert_eq!(parsed.at_least(parsed_required), rank >= required_rank);
                assert_eq!(parsed.cmp(&parsed_required), rank.cmp(&required_rank));
            }
        }
        assert!(Verification::Unverified < Verification::ReceiptVerified);
        assert!(Verification::ReceiptVerified < Verification::BlockCommitted);
        assert!(Verification::BlockCommitted.at_least(Verification::ReceiptVerified));
        assert!(!Verification::ReceiptVerified.at_least(Verification::BlockCommitted));
        for word in ["checkpoint-finalised", "paxeer-finalised"] {
            assert!(matches!(
                Verification::parse(word),
                Err(WebhookError::InvalidRequest)
            ));
        }
    }

    #[test]
    fn block_committed_orders_above_receipt() {
        let committed = Verification::BlockCommitted;
        assert_eq!(committed.as_str(), "block-committed");
        assert!(matches!(
            Verification::parse(committed.as_str()),
            Ok(Verification::BlockCommitted)
        ));

        assert!(Verification::ReceiptVerified < committed);
        assert!(Verification::Unverified < committed);
        assert!(committed.at_least(Verification::ReceiptVerified));
        assert!(committed.at_least(committed));
        assert!(!Verification::ReceiptVerified.at_least(committed));
        assert!(!Verification::Unverified.at_least(committed));
        assert!(committed.at_least(Verification::Unverified));
    }

    #[test]
    fn kernel_finality_words_are_refused() {
        for word in ["checkpoint-finalised", "paxeer-finalised"] {
            assert!(matches!(
                Verification::parse(word),
                Err(WebhookError::InvalidRequest)
            ));
            assert!(serde_json::from_value::<Verification>(serde_json::json!(word)).is_err());
        }

        for rank in 1..=3 {
            assert_eq!(
                Verification::from_wire_rank(rank),
                Verification::ReceiptVerified
            );
        }
        for rank in 4..=5 {
            assert_eq!(
                Verification::from_wire_rank(rank),
                Verification::BlockCommitted
            );
        }
        for rank in [0, 6, u8::MAX] {
            assert_eq!(Verification::from_wire_rank(rank), Verification::Unverified);
        }
        for (word, expected) in [
            ("unverified", Verification::Unverified),
            ("receipt-verified", Verification::ReceiptVerified),
            ("block-committed", Verification::BlockCommitted),
        ] {
            assert!(matches!(Verification::parse(word), Ok(level) if level == expected));
            assert_eq!(expected.as_str(), word);
        }
    }
}
