//! x402 v2 buyer role: strict offer selection, payment construction through a
//! typed `LayerX` boundary, extension echoing, and local receipt capture.

use base64::engine::general_purpose::STANDARD;
use base64::Engine as _;
use layerx_committed_proof::{
    commit::{BlockIdFlag, Commit, CommitSig},
    committed::verify_committed,
    header::{BlockId, ConsensusVersion, Header, PartSetHeader, Timestamp, Validator},
    CommittedProof,
};
use layerx_interop_gateway::evidence::canonical_receipt_digest;
use layerx_interop_gateway::trace::{TraceId, Traced};
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_proof::receipt::verify_sequencer_signature;
use layerx_proof::sequencer_logs::{verify_logs, SequencerLog};
use serde::Deserialize;
use serde_json::Value;

use crate::codec::{decode_header, encode_header};
use crate::model::{
    PaymentPayload, PaymentRequired, PaymentRequirements, SettlementResponse, X402Error,
    X402_VERSION,
};
use crate::seller::validate_network;

const PAYMENT_DRAWN_TOPIC: [u8; 32] = [
    0x13, 0x47, 0x3d, 0x10, 0x3a, 0x2d, 0x52, 0x17, 0x20, 0x16, 0xc8, 0x72, 0xee, 0xae, 0x01, 0x53,
    0xe2, 0x54, 0xaf, 0x3c, 0x21, 0xa1, 0xd9, 0x87, 0x49, 0x2a, 0xb7, 0x37, 0xf1, 0xa3, 0x4f, 0xe1,
];

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct SupportedKind {
    pub scheme: String,
    pub network: String,
}

#[derive(Clone, Debug, PartialEq)]
pub struct PaymentBuildRequest {
    pub requirements: PaymentRequirements,
    pub idempotency_key: [u8; 32],
    pub trace: TraceId,
}

/// Boundary that constructs the scheme-specific payment through the plane's
/// typed authority. The buyer adapter never signs or invents payment bytes.
pub trait BuyerPaymentPlane {
    /// Constructs the scheme payload through the plane's authority.
    ///
    /// # Errors
    ///
    /// Returns a typed construction or policy refusal.
    fn construct(&mut self, request: PaymentBuildRequest) -> Result<Value, X402Error>;
}

#[derive(Clone, Debug, PartialEq)]
pub struct BuiltPayment {
    pub header: String,
    pub payload: PaymentPayload,
    pub idempotency_key: [u8; 32],
}

#[derive(Clone, Debug, PartialEq)]
pub struct CapturedSettlement {
    pub response: SettlementResponse,
    pub canonical_receipt: Vec<u8>,
    pub receipt_digest: [u8; 32],
    pub transaction_hash: [u8; 32],
    pub rung: String,
}

/// Receipt evidence carried by the seller's `layerx` settlement extension.
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields, rename_all = "camelCase")]
pub struct LayerXEvidence {
    receipt: String,
    receipt_digest: String,
    verification_level: String,
    logs: Vec<EvidenceLog>,
    #[serde(default)]
    block_number: Option<String>,
    #[serde(default)]
    block_hash: Option<String>,
    #[serde(default)]
    purpose_hash: Option<String>,
    #[serde(default)]
    proof: Option<Value>,
}

struct ProofValue<'a>(&'a Value);

impl<'a> ProofValue<'a> {
    fn field(&self, name: &str) -> Self {
        Self(&self.0[name])
    }

    fn text(&self) -> Result<&'a str, X402Error> {
        self.0.as_str().ok_or(X402Error::EvidenceMismatch)
    }

    fn bytes(&self) -> Result<Vec<u8>, X402Error> {
        unhex(self.text()?)
    }

    fn fixed<const N: usize>(&self) -> Result<[u8; N], X402Error> {
        self.bytes()?
            .try_into()
            .map_err(|_| X402Error::EvidenceMismatch)
    }

    fn unsigned(&self) -> Result<u64, X402Error> {
        self.0.as_u64().ok_or(X402Error::EvidenceMismatch)
    }

    fn signed(&self) -> Result<i64, X402Error> {
        self.0.as_i64().ok_or(X402Error::EvidenceMismatch)
    }

    fn array(&self) -> Result<&'a [Value], X402Error> {
        self.0
            .as_array()
            .map(Vec::as_slice)
            .ok_or(X402Error::EvidenceMismatch)
    }

    fn timestamp(&self) -> Result<Timestamp, X402Error> {
        Ok(Timestamp {
            seconds: self.field("seconds").signed()?,
            nanos: self
                .field("nanos")
                .unsigned()?
                .try_into()
                .map_err(|_| X402Error::EvidenceMismatch)?,
        })
    }

    fn block_id(&self) -> Result<BlockId, X402Error> {
        Ok(BlockId {
            hash: self.field("hash").bytes()?,
            part_set_header: PartSetHeader {
                total: self
                    .field("parts")
                    .field("total")
                    .unsigned()?
                    .try_into()
                    .map_err(|_| X402Error::EvidenceMismatch)?,
                hash: self.field("parts").field("hash").bytes()?,
            },
        })
    }

    fn header(&self) -> Result<Header, X402Error> {
        Ok(Header {
            version: ConsensusVersion {
                block: self.field("version").field("block").unsigned()?,
                app: self.field("version").field("app").unsigned()?,
            },
            chain_id: self.field("chain_id").text()?.to_owned(),
            height: self.field("height").signed()?,
            time: self.field("time").timestamp()?,
            last_block_id: self.field("last_block_id").block_id()?,
            last_commit_hash: self.field("last_commit_hash").bytes()?,
            data_hash: self.field("data_hash").bytes()?,
            validators_hash: self.field("validators_hash").fixed()?,
            next_validators_hash: self.field("next_validators_hash").bytes()?,
            consensus_hash: self.field("consensus_hash").bytes()?,
            app_hash: self.field("app_hash").bytes()?,
            last_results_hash: self.field("last_results_hash").bytes()?,
            evidence_hash: self.field("evidence_hash").bytes()?,
            proposer_address: self.field("proposer_address").bytes()?,
        })
    }

    fn commit(&self) -> Result<Commit, X402Error> {
        Ok(Commit {
            height: self.field("height").unsigned()?,
            round: self
                .field("round")
                .signed()?
                .try_into()
                .map_err(|_| X402Error::EvidenceMismatch)?,
            block_id: self.field("block_id").block_id()?,
            signatures: self
                .field("signatures")
                .array()?
                .iter()
                .map(|value| {
                    let vote = Self(value);
                    Ok(CommitSig {
                        block_id_flag: match vote.field("block_id_flag").unsigned()? {
                            1 => BlockIdFlag::Absent,
                            2 => BlockIdFlag::Commit,
                            3 => BlockIdFlag::Nil,
                            _ => return Err(X402Error::EvidenceMismatch),
                        },
                        validator_address: vote.field("validator_address").bytes()?,
                        timestamp: vote.field("timestamp").timestamp()?,
                        signature: if vote.0.get("signature") == Some(&Value::Null) {
                            None
                        } else {
                            Some(vote.field("signature").fixed()?)
                        },
                    })
                })
                .collect::<Result<_, X402Error>>()?,
        })
    }
}

fn verify_committed_evidence(
    value: &Value,
    receipt: &[u8],
    digest: [u8; 32],
    number: &str,
    hash: &str,
) -> Result<(), X402Error> {
    let value = ProofValue(value);
    let batch = value.field("batchProof");
    let execution_header = value.field("executionHeader").header()?;
    if batch.field("leafEncoding").text()? != "sha256-tx"
        || batch.field("rootHash").bytes()? != execution_header.data_hash
    {
        return Err(X402Error::EvidenceMismatch);
    }
    let proof = CommittedProof {
        header: value.field("header").header()?,
        commit: value.field("commit").commit()?,
        execution_header,
        batch_transaction: batch.field("data").bytes()?,
        batch_total: batch.field("total").unsigned()?,
        batch_index: batch.field("index").unsigned()?,
        batch_aunts: batch
            .field("aunts")
            .array()?
            .iter()
            .map(|aunt| ProofValue(aunt).fixed())
            .collect::<Result<_, _>>()?,
        signed_receipt: receipt.to_vec(),
        activity_ids: value
            .field("activityIds")
            .array()?
            .iter()
            .map(|id| ProofValue(id).fixed())
            .collect::<Result<_, _>>()?,
    };
    let validators = value
        .field("validators")
        .array()?
        .iter()
        .map(|validator| {
            let validator = ProofValue(validator);
            Ok(Validator {
                pub_key: validator.field("pub_key").fixed()?,
                voting_power: validator.field("voting_power").signed()?,
            })
        })
        .collect::<Result<Vec<_>, X402Error>>()?;
    let block = verify_committed(digest, &proof, &validators, value.field("chainId").text()?)
        .map_err(|_| X402Error::EvidenceMismatch)?;
    let number = number
        .strip_prefix("0x")
        .ok_or(X402Error::EvidenceMismatch)?;
    if u64::from_str_radix(number, 16).map_err(|_| X402Error::EvidenceMismatch)? != block.number
        || unhex(hash)?.as_slice() != block.hash
    {
        return Err(X402Error::EvidenceMismatch);
    }
    Ok(())
}

#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields, rename_all = "camelCase")]
struct EvidenceLog {
    address: String,
    topics: Vec<String>,
    data: String,
    log_index: String,
}

impl EvidenceLog {
    fn decode(self) -> Result<SequencerLog, X402Error> {
        let index = self
            .log_index
            .strip_prefix("0x")
            .ok_or(X402Error::EvidenceMismatch)?;
        if index.is_empty()
            || index.len() > 16
            || !index.bytes().all(|byte| byte.is_ascii_hexdigit())
            || (index != "0" && index.starts_with('0'))
        {
            return Err(X402Error::EvidenceMismatch);
        }
        Ok(SequencerLog::new(
            unhex(&self.address)?
                .try_into()
                .map_err(|_| X402Error::EvidenceMismatch)?,
            self.topics
                .iter()
                .map(|topic| {
                    unhex(topic)?
                        .try_into()
                        .map_err(|_| X402Error::EvidenceMismatch)
                })
                .collect::<Result<_, _>>()?,
            unhex(&self.data)?,
            u64::from_str_radix(index, 16).map_err(|_| X402Error::EvidenceMismatch)?,
        ))
    }
}

/// x402 buyer with a closed list of supported scheme/network pairs.
#[derive(Clone, Debug)]
pub struct Buyer {
    supported: Vec<SupportedKind>,
    receipt_keys: Vec<SequencerAuthorization>,
}

impl Buyer {
    /// Constructs a buyer with at least one bounded supported kind.
    ///
    /// # Errors
    ///
    /// Refuses an empty or duplicate support declaration.
    pub fn new(supported: Vec<SupportedKind>) -> Result<Self, X402Error> {
        if supported.is_empty() || supported.len() > 64 {
            return Err(X402Error::UnsupportedOffer);
        }
        for (index, kind) in supported.iter().enumerate() {
            if kind.scheme.is_empty()
                || validate_network(&kind.network).is_err()
                || supported[..index].contains(kind)
            {
                return Err(X402Error::UnsupportedOffer);
            }
        }
        Ok(Self {
            supported,
            receipt_keys: Vec::new(),
        })
    }

    pub fn with_receipt_keys(
        supported: Vec<SupportedKind>,
        receipt_keys: Vec<SequencerAuthorization>,
    ) -> Result<Self, X402Error> {
        if receipt_keys.is_empty()
            || receipt_keys.iter().enumerate().any(|(index, key)| {
                key.first_batch_number() > key.last_batch_number()
                    || receipt_keys[..index]
                        .iter()
                        .any(|previous| previous.sequencer_id() == key.sequencer_id())
            })
        {
            return Err(X402Error::EvidenceMismatch);
        }
        let mut buyer = Self::new(supported)?;
        buyer.receipt_keys = receipt_keys;
        Ok(buyer)
    }

    #[must_use]
    pub fn receipt_keys(&self) -> &[SequencerAuthorization] {
        &self.receipt_keys
    }

    /// Parses a `PAYMENT-REQUIRED` header, selects the first supported offer
    /// in seller order, and asks the real plane boundary to construct its
    /// scheme payload. Required extensions are echoed byte-for-value.
    ///
    /// # Errors
    ///
    /// Returns a trace-bound typed refusal for malformed or unsupported
    /// offers and plane construction failures.
    pub fn build_payment(
        &self,
        required_header: &str,
        idempotency_key: [u8; 32],
        plane: &mut impl BuyerPaymentPlane,
        trace: &TraceId,
    ) -> Result<BuiltPayment, Traced<X402Error>> {
        let fail = |error| trace.wrap(error);
        if idempotency_key == [0; 32] {
            return Err(fail(X402Error::InvalidPayload));
        }
        let required: PaymentRequired = decode_header(required_header).map_err(fail)?;
        required.validate().map_err(fail)?;
        let accepted = required
            .accepts
            .iter()
            .find(|requirements| {
                self.supported.iter().any(|kind| {
                    kind.scheme == requirements.scheme && kind.network == requirements.network
                })
            })
            .cloned()
            .ok_or_else(|| fail(X402Error::UnsupportedOffer))?;
        validate_network(&accepted.network).map_err(fail)?;
        accepted.layerx_terms().map_err(fail)?;
        let scheme_payload = plane
            .construct(PaymentBuildRequest {
                requirements: accepted.clone(),
                idempotency_key,
                trace: trace.clone(),
            })
            .map_err(fail)?;
        if !scheme_payload.is_object() {
            return Err(fail(X402Error::InvalidPayload));
        }
        let payload = PaymentPayload {
            x402_version: X402_VERSION,
            resource: Some(required.resource),
            payload: scheme_payload,
            accepted,
            extensions: required.extensions,
        };
        payload.validate().map_err(fail)?;
        let header = encode_header(&payload).map_err(fail)?;
        Ok(BuiltPayment {
            header,
            payload,
            idempotency_key,
        })
    }

    /// Captures a `PAYMENT-RESPONSE` only after locally verifying the attached
    /// canonical `LayerX` receipt and logs under pinned sequencer keys.
    ///
    /// # Errors
    ///
    /// Refuses pending/failed responses as success, missing or mismatched
    /// evidence, malformed receipts and unauthorised sequencer signatures,
    /// a grant settlement whose `purposeHash` is absent or is not the one the
    /// accepted offer carries, and an exact settlement carrying one.
    pub fn capture_settlement(
        response_header: &str,
        expected: &BuiltPayment,
        receipt_keys: &[SequencerAuthorization],
        trace: &TraceId,
    ) -> Result<CapturedSettlement, Traced<X402Error>> {
        let fail = |error| trace.wrap(error);
        let response: SettlementResponse = decode_header(response_header).map_err(fail)?;
        response.validate_wire().map_err(fail)?;
        if !response.success {
            return Err(fail(X402Error::PaymentRefused));
        }
        if response.transaction.len() != 66
            || !response.transaction.starts_with("0x")
            || !response.transaction.as_bytes()[2..]
                .iter()
                .all(|byte| byte.is_ascii_digit() || (b'a'..=b'f').contains(byte))
        {
            return Err(fail(X402Error::EvidenceMismatch));
        }
        validate_network(&response.network).map_err(fail)?;
        let evidence_value = response
            .extensions
            .get("layerx")
            .cloned()
            .ok_or_else(|| fail(X402Error::EvidenceMissing))?;
        let has_block_fields = evidence_value.get("blockNumber").is_some()
            || evidence_value.get("blockHash").is_some();
        let evidence: LayerXEvidence = serde_json::from_value(evidence_value)
            .map_err(|_| fail(X402Error::EvidenceMismatch))?;
        let valid_rung = match evidence.verification_level.as_str() {
            "receipted" => !has_block_fields && evidence.proof.is_none(),
            "committed" => match (
                evidence.block_number.as_deref(),
                evidence.block_hash.as_deref(),
            ) {
                (Some(number), Some(hash)) => {
                    evidence.proof.is_some()
                        && number.strip_prefix("0x").is_some_and(|digits| {
                            !digits.is_empty()
                                && digits.len() <= 16
                                && (digits == "0" || !digits.starts_with('0'))
                                && digits.bytes().all(|byte| byte.is_ascii_hexdigit())
                        })
                        && hash.len() == 66
                        && hash.starts_with("0x")
                        && hash.as_bytes()[2..]
                            .iter()
                            .all(|byte| byte.is_ascii_digit() || (b'a'..=b'f').contains(byte))
                }
                _ => false,
            },
            _ => false,
        };
        if !valid_rung
            || evidence.purpose_hash
                != expected_purpose(&expected.payload.accepted).map_err(fail)?
        {
            return Err(fail(X402Error::EvidenceMismatch));
        }
        let canonical_receipt = STANDARD
            .decode(evidence.receipt.as_bytes())
            .map_err(|_| fail(X402Error::EvidenceMismatch))?;
        let verified = verify_sequencer_signature(&canonical_receipt, receipt_keys)
            .map_err(|_| fail(X402Error::EvidenceMismatch))?;
        let logs = evidence
            .logs
            .into_iter()
            .map(EvidenceLog::decode)
            .collect::<Result<Vec<_>, _>>()
            .map_err(fail)?;
        let logs = verify_logs(&verified, logs).map_err(|_| fail(X402Error::EvidenceMismatch))?;
        let protocol = verified
            .protocol()
            .ok_or_else(|| fail(X402Error::EvidenceMismatch))?;
        if protocol.result_code() != 0 {
            return Err(fail(X402Error::EvidenceMismatch));
        }
        let ledger: [u8; 20] = unhex("0x000000000000000000000000000000000000101a")
            .map_err(fail)?
            .try_into()
            .map_err(|_| fail(X402Error::EvidenceMismatch))?;
        let metered = expected.payload.accepted.scheme == "metered";
        let topic: [u8; 32] = if metered {
            PAYMENT_DRAWN_TOPIC
        } else {
            unhex("0xc4da5d6f757c251cd70947b752917e5671b677517e3d953f362a1df6648ef5b1")
                .map_err(fail)?
                .try_into()
                .map_err(|_| fail(X402Error::EvidenceMismatch))?
        };
        let mut payments = logs
            .logs()
            .iter()
            .filter(|log| log.address() == ledger && log.topics().first() == Some(&topic));
        let payment = payments
            .next()
            .ok_or_else(|| fail(X402Error::EvidenceMismatch))?;
        if payments.next().is_some()
            || payment.topics().len() != 4
            || payment.data().len() != if metered { 128 } else { 64 }
            || expected.idempotency_key == [0; 32]
            || payment.data()[32..48] != [0; 16]
        {
            return Err(fail(X402Error::EvidenceMismatch));
        }
        if metered {
            let grant = payment_reference(
                expected
                    .payload
                    .payload
                    .get("grantId")
                    .ok_or_else(|| fail(X402Error::EvidenceMismatch))?,
            )
            .map_err(fail)?;
            if payment.topics()[1] != grant
                || payment.data()[64..96] != expected.idempotency_key[..]
            {
                return Err(fail(X402Error::EvidenceMismatch));
            }
        } else if payment.topics()[1] != expected.idempotency_key {
            return Err(fail(X402Error::EvidenceMismatch));
        }
        let asset: [u8; 32] = payment.data()[..32]
            .try_into()
            .map_err(|_| fail(X402Error::EvidenceMismatch))?;
        let amount = u128::from_be_bytes(
            payment.data()[48..64]
                .try_into()
                .map_err(|_| fail(X402Error::EvidenceMismatch))?,
        );
        let accepted = &expected.payload.accepted;
        accepted.validate().map_err(fail)?;
        if unhex(&accepted.asset).map_err(fail)?.as_slice() != asset
            || unhex(&accepted.pay_to).map_err(fail)?.as_slice() != payment.topics()[3]
            || accepted.amount.value() != amount
        {
            return Err(fail(X402Error::EvidenceMismatch));
        }
        let payer = hex(&payment.topics()[2]);
        if response.network != expected.payload.accepted.network
            || response.amount != Some(expected.payload.accepted.amount)
            || response.payer.as_deref() != Some(payer.as_str())
        {
            return Err(fail(X402Error::EvidenceMismatch));
        }
        let receipt_digest = canonical_receipt_digest(&canonical_receipt)
            .map_err(|_| fail(X402Error::EvidenceMismatch))?;
        let digest_text = hex(&receipt_digest);
        if evidence.receipt_digest != digest_text
            || response.transaction != format!("0x{}", hex(&protocol.activity_id()))
        {
            return Err(fail(X402Error::EvidenceMismatch));
        }
        if evidence.verification_level == "committed" {
            verify_committed_evidence(
                evidence
                    .proof
                    .as_ref()
                    .ok_or_else(|| fail(X402Error::EvidenceMismatch))?,
                &canonical_receipt,
                receipt_digest,
                evidence
                    .block_number
                    .as_deref()
                    .ok_or_else(|| fail(X402Error::EvidenceMismatch))?,
                evidence
                    .block_hash
                    .as_deref()
                    .ok_or_else(|| fail(X402Error::EvidenceMismatch))?,
            )
            .map_err(fail)?;
        }
        Ok(CapturedSettlement {
            transaction_hash: protocol.activity_id(),
            rung: evidence.verification_level,
            response,
            canonical_receipt,
            receipt_digest,
        })
    }
}

fn payment_reference(value: &Value) -> Result<[u8; 32], X402Error> {
    let text = value.as_str().ok_or(X402Error::EvidenceMismatch)?;
    let digits = text.strip_prefix("0x").unwrap_or(text);
    if digits.len() != 64 || !digits.bytes().all(|byte| byte.is_ascii_hexdigit()) {
        return Err(X402Error::EvidenceMismatch);
    }
    let mut reference = [0; 32];
    for (index, byte) in reference.iter_mut().enumerate() {
        *byte = u8::from_str_radix(&digits[index * 2..index * 2 + 2], 16)
            .map_err(|_| X402Error::EvidenceMismatch)?;
    }
    if reference == [0; 32] {
        return Err(X402Error::EvidenceMismatch);
    }
    Ok(reference)
}

/// The `purposeHash` a settlement of `accepted` must repeat: none for an
/// exact offer, and for a metered or subscription grant offer the 64 lowercase
/// hexadecimal characters of its challenge, which such an offer must carry.
fn expected_purpose(accepted: &PaymentRequirements) -> Result<Option<String>, X402Error> {
    if !matches!(accepted.scheme.as_str(), "metered" | "subscription") {
        return Ok(None);
    }
    accepted
        .extra
        .as_ref()
        .and_then(|extra| extra.pointer("/layerx/purposeHash"))
        .and_then(Value::as_str)
        .filter(|purpose| {
            purpose.len() == 64
                && purpose
                    .bytes()
                    .all(|byte| byte.is_ascii_digit() || (b'a'..=b'f').contains(&byte))
        })
        .map(|purpose| Some(purpose.to_owned()))
        .ok_or(X402Error::EvidenceMismatch)
}

fn unhex(value: &str) -> Result<Vec<u8>, X402Error> {
    let digits = match value.strip_prefix("0x") {
        Some(digits) => digits,
        None if value.len() == 64
            && value
                .bytes()
                .all(|byte| byte.is_ascii_digit() || (b'a'..=b'f').contains(&byte)) =>
        {
            value
        }
        None => return Err(X402Error::EvidenceMismatch),
    };
    if digits.len() % 2 != 0 || !digits.bytes().all(|byte| byte.is_ascii_hexdigit()) {
        return Err(X402Error::EvidenceMismatch);
    }
    digits
        .as_bytes()
        .chunks_exact(2)
        .map(|pair| {
            let text = std::str::from_utf8(pair).map_err(|_| X402Error::EvidenceMismatch)?;
            u8::from_str_radix(text, 16).map_err(|_| X402Error::EvidenceMismatch)
        })
        .collect()
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
    use std::collections::BTreeMap;

    use ed25519_dalek::{Signer as _, SigningKey};
    use serde_json::{json, Value};

    use super::{hex, unhex, BuiltPayment, Buyer, CapturedSettlement, SupportedKind};
    use crate::codec::encode_header;
    use crate::model::{
        AtomicAmount, PaymentPayload, PaymentRequirements, SettlementResponse, X402Error,
        X402_VERSION,
    };
    use base64::engine::general_purpose::STANDARD;
    use base64::Engine as _;
    use layerx_interop_gateway::trace::{TraceId, Traced};
    use layerx_proof::inclusion::SequencerAuthorization;

    const TRANSACTION: [u8; 32] = [
        0xa0, 0xd9, 0x85, 0xd9, 0x43, 0x52, 0x18, 0x92, 0x40, 0x1c, 0x62, 0x3b, 0xd7, 0xb3, 0x2a,
        0x62, 0x71, 0x47, 0x58, 0x5c, 0xbe, 0x20, 0x6b, 0xad, 0xb4, 0xd2, 0x14, 0xd8, 0x44, 0xb1,
        0xcc, 0xd1,
    ];

    fn settlement() -> Result<
        (
            BuiltPayment,
            SettlementResponse,
            Vec<SequencerAuthorization>,
        ),
        X402Error,
    > {
        let document: Value = serde_json::from_str(include_str!(
            "../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
        ))
        .map_err(|_| X402Error::EvidenceMismatch)?;
        let case = document["cases"]
            .as_array()
            .and_then(|cases| {
                cases
                    .iter()
                    .find(|case| case["id"] == "V1-layerxledger-pay-exact")
            })
            .ok_or(X402Error::EvidenceMissing)?;
        let text = |value: &Value| {
            value
                .as_str()
                .map(str::to_owned)
                .ok_or(X402Error::EvidenceMismatch)
        };
        let word = |value: &Value| -> Result<[u8; 32], X402Error> {
            unhex(&text(value)?)?
                .try_into()
                .map_err(|_| X402Error::EvidenceMismatch)
        };
        let log = &case["receiptJson"]["logs"][0];
        let mut topics = log["topics"]
            .as_array()
            .ok_or(X402Error::EvidenceMismatch)?
            .iter()
            .map(word)
            .collect::<Result<Vec<_>, _>>()?;
        let recorded_topics = topics.concat();
        let words = crate::seller::tests::pay_exact_words(&unhex(&text(&case["rawTransaction"])?)?);
        topics[1..].copy_from_slice(&[words[5], words[0], words[1]]);
        topics[3] = crate::model::account_identifiers("agent:did:layerx:payee:main")?[1];
        let mut receipt = unhex(&text(&case["signedReceipt"])?)?;
        let positions: Vec<_> = receipt
            .windows(recorded_topics.len())
            .enumerate()
            .filter_map(|(index, value)| (value == recorded_topics).then_some(index))
            .collect();
        assert_eq!(positions.len(), 1);
        receipt[positions[0]..positions[0] + recorded_topics.len()]
            .copy_from_slice(&topics.concat());
        let digest = layerx_interop_gateway::evidence::canonical_receipt_digest(&receipt)
            .map_err(|_| X402Error::EvidenceMismatch)?;
        let signer = SigningKey::from_bytes(&[0x72; 32]);
        let signature_start = receipt.len() - 64;
        receipt[signature_start..].copy_from_slice(&signer.sign(&digest).to_bytes());
        let mut logs = case["receiptJson"]["logs"].clone();
        logs[0]["topics"] = json!(topics
            .iter()
            .map(|topic| format!("0x{}", hex(topic)))
            .collect::<Vec<_>>());
        let data = unhex(&text(&log["data"])?)?;
        let amount = AtomicAmount::from_u128(1_000_000);
        let payment = BuiltPayment {
            header: String::new(),
            payload: PaymentPayload {
                x402_version: X402_VERSION,
                resource: None,
                accepted: PaymentRequirements {
                    scheme: "exact".to_owned(),
                    network: "eip155:125".to_owned(),
                    amount,
                    asset: format!(
                        "0x{}",
                        hex(&data[..32]
                            .try_into()
                            .map_err(|_| X402Error::EvidenceMismatch)?)
                    ),
                    pay_to: format!("0x{}", hex(&topics[3])),
                    max_timeout_seconds: 90,
                    extra: Some(
                        json!({"layerx": {"account": "agent:did:layerx:payee:main", "currency": "LXP"}}),
                    ),
                },
                payload: json!({}),
                extensions: BTreeMap::new(),
            },
            idempotency_key: topics[1],
        };
        let response = SettlementResponse {
            success: true,
            error_reason: None,
            payer: Some(hex(&topics[2])),
            transaction: text(&case["receiptJson"]["txHash"])?,
            network: "eip155:125".to_owned(),
            amount: Some(amount),
            extensions: BTreeMap::from([(
                "layerx".to_owned(),
                json!({
                    "receipt": STANDARD.encode(&receipt),
                    "receiptDigest": hex(&digest),
                    "verificationLevel": "receipted", "logs": logs,
                }),
            )]),
        };
        let key = &case["pinnedKey"];
        let keys = vec![SequencerAuthorization::new(
            word(&key["keyId"])?,
            signer.verifying_key().to_bytes(),
            key["firstHeight"]
                .as_u64()
                .ok_or(X402Error::EvidenceMismatch)?,
            key["lastHeight"]
                .as_u64()
                .ok_or(X402Error::EvidenceMismatch)?,
        )];
        Ok((payment, response, keys))
    }

    fn capture(
        payment: &BuiltPayment,
        response: &SettlementResponse,
        keys: &[SequencerAuthorization],
    ) -> Result<CapturedSettlement, X402Error> {
        Buyer::capture_settlement(
            &encode_header(response)?,
            payment,
            keys,
            &TraceId::mint([0xab; 16]),
        )
        .map_err(Traced::into_error)
    }

    #[test]
    fn buyer_accepts_receipted_transaction_hash() -> Result<(), X402Error> {
        let (payment, response, batch) = settlement()?;
        let captured = capture(&payment, &response, &batch)?;
        assert_eq!(captured.response, response);
        assert_eq!(
            captured.response.transaction,
            format!("0x{}", hex(&TRANSACTION))
        );
        assert_eq!(
            captured.response.extensions["layerx"]["verificationLevel"],
            "receipted"
        );
        assert_eq!(
            response.extensions["layerx"]["receiptDigest"],
            json!(hex(&captured.receipt_digest))
        );
        Ok(())
    }

    #[test]
    fn buyer_refuses_lxp_form_and_malformed_hashes() -> Result<(), X402Error> {
        let (payment, mut response, batch) = settlement()?;
        for transaction in [
            "lxp:abc".to_owned(),
            format!("lxp:{}", hex(&TRANSACTION)),
            hex(&TRANSACTION),
            format!("0x{}", "61".repeat(31)),
            format!("0x{}", "61".repeat(33)),
            format!("0x{}", "ZZ".repeat(32)),
            format!("0x{}", "AB".repeat(32)),
        ] {
            response.transaction = transaction;
            assert_eq!(
                capture(&payment, &response, &batch),
                Err(X402Error::TransactionForm)
            );
        }
        Ok(())
    }

    #[test]
    fn buyer_binds_hash_network_digest_and_signature() -> Result<(), X402Error> {
        let (payment, response, batch) = settlement()?;
        let mut changed = response.clone();
        changed.transaction = format!("0x{}", hex(&[0x71; 32]));
        assert_eq!(
            capture(&payment, &changed, &batch),
            Err(X402Error::EvidenceMismatch)
        );
        changed = response.clone();
        changed.network = "eip155:126".to_owned();
        assert_eq!(
            capture(&payment, &changed, &batch),
            Err(X402Error::EvidenceMismatch)
        );
        changed = response.clone();
        changed
            .extensions
            .get_mut("layerx")
            .ok_or(X402Error::EvidenceMissing)?["receiptDigest"] = json!(hex(&[0x72; 32]));
        assert_eq!(
            capture(&payment, &changed, &batch),
            Err(X402Error::EvidenceMismatch)
        );
        let wrong_key = SigningKey::from_bytes(&[0x52; 32]);
        let wrong_batch = vec![SequencerAuthorization::new(
            batch[0].sequencer_id(),
            wrong_key.verifying_key().to_bytes(),
            batch[0].first_batch_number(),
            batch[0].last_batch_number(),
        )];
        assert_eq!(
            capture(&payment, &response, &wrong_batch),
            Err(X402Error::EvidenceMismatch)
        );
        Ok(())
    }

    #[test]
    fn buyer_refuses_legacy_pending_and_unproved_committed_rungs() -> Result<(), X402Error> {
        let (payment, mut response, batch) = settlement()?;
        for rung in ["sequencer-signed", "submitted", "committed"] {
            response
                .extensions
                .get_mut("layerx")
                .ok_or(X402Error::EvidenceMissing)?["verificationLevel"] = json!(rung);
            assert_eq!(
                capture(&payment, &response, &batch),
                Err(X402Error::EvidenceMismatch)
            );
        }
        Ok(())
    }

    #[test]
    fn buyer_support_requires_canonical_eip155_network() -> Result<(), X402Error> {
        let kind = |network: &str| SupportedKind {
            scheme: "exact".to_owned(),
            network: network.to_owned(),
        };
        Buyer::new(vec![kind("eip155:125")])?;
        for network in [
            "layerx:beta",
            "eip155:0",
            "eip155:0125",
            "eip155:+125",
            "eip155:",
        ] {
            assert_eq!(
                Buyer::new(vec![kind(network)]).err(),
                Some(X402Error::UnsupportedOffer)
            );
        }
        Ok(())
    }
}
