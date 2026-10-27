//! x402 v2 buyer role: strict offer selection, payment construction through a
//! typed `LayerX` boundary, extension echoing, and local receipt capture.

use base64::engine::general_purpose::STANDARD;
use base64::Engine as _;
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

#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields, rename_all = "camelCase")]
struct LayerXEvidence {
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
            "receipted" => !has_block_fields,
            "committed" => match (
                evidence.block_number.as_deref(),
                evidence.block_hash.as_deref(),
            ) {
                (Some(number), Some(hash)) => {
                    number.strip_prefix("0x").is_some_and(|digits| {
                        !digits.is_empty()
                            && digits.len() <= 16
                            && (digits == "0" || !digits.starts_with('0'))
                            && digits.bytes().all(|byte| byte.is_ascii_hexdigit())
                    }) && hash.len() == 66
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
        let topic: [u8; 32] =
            unhex("0xc4da5d6f757c251cd70947b752917e5671b677517e3d953f362a1df6648ef5b1")
                .map_err(fail)?
                .try_into()
                .map_err(|_| fail(X402Error::EvidenceMismatch))?;
        let mut payments = logs
            .logs()
            .iter()
            .filter(|log| log.address() == ledger && log.topics().first() == Some(&topic));
        let payment = payments
            .next()
            .ok_or_else(|| fail(X402Error::EvidenceMismatch))?;
        if payments.next().is_some()
            || payment.topics().len() != 4
            || payment.data().len() != 64
            || payment.topics()[3] != expected.idempotency_key
            || expected.idempotency_key == [0; 32]
            || payment.data()[32..48] != [0; 16]
        {
            return Err(fail(X402Error::EvidenceMismatch));
        }
        let asset: [u8; 32] = payment.data()[..32]
            .try_into()
            .map_err(|_| fail(X402Error::EvidenceMismatch))?;
        let amount = u128::from_be_bytes(
            payment.data()[48..]
                .try_into()
                .map_err(|_| fail(X402Error::EvidenceMismatch))?,
        );
        let accepted = &expected.payload.accepted;
        accepted.validate().map_err(fail)?;
        if unhex(&accepted.asset).map_err(fail)?.as_slice() != asset
            || unhex(&accepted.pay_to).map_err(fail)?.as_slice() != payment.topics()[2]
            || accepted.amount.value() != amount
        {
            return Err(fail(X402Error::EvidenceMismatch));
        }
        let payer = hex(&payment.topics()[1]);
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
        Ok(CapturedSettlement {
            transaction_hash: protocol.activity_id(),
            rung: evidence.verification_level,
            response,
            canonical_receipt,
            receipt_digest,
        })
    }
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
    let digits = value
        .strip_prefix("0x")
        .ok_or(X402Error::EvidenceMismatch)?;
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

    use ed25519_dalek::SigningKey;
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
        0x9f, 0x6b, 0xe3, 0xb6, 0xfd, 0xa7, 0xdd, 0x66, 0x4d, 0xdb, 0x83, 0xa9, 0x01, 0xda, 0xd4,
        0x45, 0x8b, 0x78, 0x29, 0x8b, 0x59, 0xe3, 0x07, 0x15, 0x1c, 0x21, 0x7a, 0x2a, 0x1f, 0x13,
        0xd2, 0xbb,
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
                    pay_to: text(&log["topics"][2])?,
                    max_timeout_seconds: 90,
                    extra: Some(
                        json!({"layerx": {"account": "agent:did:layerx:payee:main", "currency": "LXP"}}),
                    ),
                },
                payload: json!({}),
                extensions: BTreeMap::new(),
            },
            idempotency_key: word(&log["topics"][3])?,
        };
        let response = SettlementResponse {
            success: true,
            error_reason: None,
            payer: Some(hex(&word(&log["topics"][1])?)),
            transaction: text(&case["receiptJson"]["txHash"])?,
            network: "eip155:125".to_owned(),
            amount: Some(amount),
            extensions: BTreeMap::from([(
                "layerx".to_owned(),
                json!({
                    "receipt": STANDARD.encode(unhex(&text(&case["signedReceipt"])?)?),
                    "receiptDigest": text(&case["receiptDigest"])?.trim_start_matches("0x"),
                    "verificationLevel": "receipted", "logs": case["receiptJson"]["logs"],
                }),
            )]),
        };
        let key = &case["pinnedKey"];
        let keys = vec![SequencerAuthorization::new(
            word(&key["keyId"])?,
            word(&key["publicKey"])?,
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
                Err(X402Error::EvidenceMismatch)
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
