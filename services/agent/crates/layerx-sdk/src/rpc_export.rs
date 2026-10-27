//! `export.offline` over the version 1 Agent operation envelope, and the offline decode and
//! complete verification of its answer.

use layerx_agent_api::error::RequestId;
use layerx_agent_api::export::{
    check_export_response, validate_export_request, FactRef, OfflineExport,
};
use layerx_agent_api::prepare::CanonicalBytes;
use layerx_agent_api::read::{ReadRequest, VerifiedRead};
use layerx_agent_api::verify::{Rung, VerificationStatus};
use layerx_proof::export::{
    verify_complete, CompleteExportError, CompleteVerificationReport, IndependentOfflineTrust,
};
use layerx_proof::export_codec::{
    ArtifactDecodeError, CompleteOfflineArtifact, EncodedOfflineArtifact,
};

use serde_json::{json, Value};

use crate::agent_envelope::{AgentEnvelopeTransport, EnvelopeCredential, EnvelopeError};
use crate::rpc_history::lower_hex;
use crate::rpc_projection::decode_freshness;
use crate::rpc_subscription::{decimal, object, text, violation};
use crate::Operation;

pub(crate) const fn rung_wire(rung: Rung) -> &'static str {
    rung.name()
}

pub(crate) fn rung_from_wire(value: &Value, operation: Operation) -> Result<Rung, EnvelopeError> {
    value
        .as_str()
        .and_then(Rung::from_name)
        .ok_or_else(|| violation(operation))
}

pub(crate) fn decode_bundle(
    value: &Value,
    operation: Operation,
) -> Result<layerx_agent_api::proof::ProofBundle, EnvelopeError> {
    use layerx_agent_api::proof::ProofBundle;
    use layerx_client::sequencer::ReceiptKeyEntry;
    let bundle = value.as_object().ok_or_else(|| violation(operation))?;
    let rung = rung_from_wire(
        bundle
            .get("verificationLevel")
            .ok_or_else(|| violation(operation))?,
        operation,
    )?;
    let mut fields = vec![
        "format",
        "verificationLevel",
        "rawTransaction",
        "signedReceipt",
        "receiptDigest",
        "receiptKey",
    ];
    if rung == Rung::Committed {
        fields.extend(["blockHeader", "receiptInclusion", "commit", "validatorSet"]);
        if bundle.contains_key("accountProof") {
            fields.push("accountProof");
        }
    }
    let bundle = object(value, &fields, operation)?;
    let bytes = |value: &Value| -> Result<CanonicalBytes, EnvelopeError> {
        value
            .as_str()
            .and_then(|s| s.strip_prefix("0x"))
            .and_then(lower_hex)
            .filter(|bytes| bytes.len() <= layerx_agent_api::proof::MAX_PROOF_BUNDLE_BYTES)
            .and_then(|bytes| CanonicalBytes::new(bytes).ok())
            .ok_or_else(|| violation(operation))
    };
    let id = |value: &Value| -> Result<[u8; 32], EnvelopeError> {
        bytes(value)?
            .as_bytes()
            .try_into()
            .map_err(|_| violation(operation))
    };
    let optional = |field: &str| bundle.get(field).map(&bytes).transpose();
    let key = object(
        &bundle["receiptKey"],
        &["keyId", "publicKey", "firstHeight", "lastHeight"],
        operation,
    )?;
    let result = ProofBundle {
        format: text(&bundle["format"], operation)?,
        verification_level: rung,
        raw_transaction: bytes(&bundle["rawTransaction"])?,
        signed_receipt: bytes(&bundle["signedReceipt"])?,
        receipt_digest: id(&bundle["receiptDigest"])?,
        receipt_key: ReceiptKeyEntry {
            key_id: id(&key["keyId"])?,
            public_key: id(&key["publicKey"])?,
            first_height: decimal(&key["firstHeight"], operation)?,
            last_height: decimal(&key["lastHeight"], operation)?,
        },
        block_header: optional("blockHeader")?,
        receipt_inclusion: optional("receiptInclusion")?,
        commit: optional("commit")?,
        validator_set: optional("validatorSet")?,
        account_proof: optional("accountProof")?,
    };
    result.validate().map_err(|_| violation(operation))?;
    Ok(result)
}

pub fn decode_offline_export(
    value: &Value,
    operation: Operation,
) -> Result<OfflineExport, EnvelopeError> {
    let export = object(value, &["facts", "bundles", "height"], operation)?;
    Ok(OfflineExport {
        facts: export["facts"]
            .as_array()
            .ok_or_else(|| violation(operation))?
            .iter()
            .map(|fact| FactRef::new(text(fact, operation)?).map_err(|_| violation(operation)))
            .collect::<Result<Vec<_>, _>>()?,
        bundles: export["bundles"]
            .as_array()
            .ok_or_else(|| violation(operation))?
            .iter()
            .map(|value| decode_bundle(value, operation))
            .collect::<Result<Vec<_>, _>>()?,
        height: decimal(&export["height"], operation)?,
    })
}

/// Decodes a historical kernel artifact. The trust
/// context is not consulted by decoding; it is taken so the artifact is only ever produced on
/// the path that then verifies it with [`verify_complete_historical_offline_export`].
///
/// # Errors
/// Returns the strict codec refusal of the first record that is not canonical.
pub fn decode_historical_offline_export(
    export: &EncodedOfflineArtifact,
    _trust: &IndependentOfflineTrust,
) -> Result<CompleteOfflineArtifact, ArtifactDecodeError> {
    let facts: Vec<&str> = export.facts.iter().map(String::as_str).collect();
    CompleteOfflineArtifact::decode(
        &facts,
        &export.receipts,
        &export.proofs,
        &export.certificates,
        &export.headers,
    )
}

/// Refusal of historical artifact verification.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum OfflineVerificationError {
    Codec(ArtifactDecodeError),
    Verify(CompleteExportError),
    /// The independently verified rung is below the rung the response claimed.
    ClaimExceedsVerified {
        claimed: Rung,
        verified: Rung,
    },
}

/// Decodes and verifies a received export without the daemon or the network, at the level
/// originally requested. The response's claimed level is not trusted: a verified level below
/// it is refused.
///
/// # Errors
/// Returns the codec refusal, the complete verifier refusal, or
/// [`OfflineVerificationError::ClaimExceedsVerified`].
pub fn verify_complete_historical_offline_export(
    artifact: &CompleteOfflineArtifact,
    claimed: Rung,
    requested: Rung,
    trust: &IndependentOfflineTrust,
) -> Result<CompleteVerificationReport, OfflineVerificationError> {
    let report =
        verify_complete(artifact, trust, requested).map_err(OfflineVerificationError::Verify)?;
    let verified = report.achieved();
    if verified < claimed {
        return Err(OfflineVerificationError::ClaimExceedsVerified { claimed, verified });
    }
    Ok(report)
}

impl AgentEnvelopeTransport {
    /// Retrieves the complete offline evidence for 1 through 16 facts at the requested level.
    ///
    /// # Errors
    ///
    /// Returns `InvalidRequest` for a fact set outside the shared grammar before sending, the established error envelope, `Transport`, or `Decode` when the
    /// answer is not exactly the requested facts at or above the requested level.
    pub fn export_offline(
        &self,
        request_id: RequestId,
        credential: &EnvelopeCredential,
        request: &ReadRequest<Vec<FactRef>>,
    ) -> Result<VerifiedRead<OfflineExport>, EnvelopeError> {
        let operation = Operation::ExportOffline;
        let request =
            validate_export_request(request.clone()).map_err(|_| EnvelopeError::InvalidRequest)?;
        let success = self.send_operation(
            operation,
            request_id,
            &json!({
                "fact_set": request.selector.iter().map(FactRef::as_str).collect::<Vec<_>>(),
                "requested_rung": rung_wire(request.requested_rung),
            }),
            Some(credential),
            None,
        )?;
        let read = object(
            &success.value,
            &["value", "achieved_rung", "freshness"],
            operation,
        )?;
        let read = VerifiedRead::new(
            decode_offline_export(&read["value"], operation)?,
            rung_from_wire(&read["achieved_rung"], operation)?,
            decode_freshness(&read["freshness"], operation)?,
        );
        if !matches!(success.verification_status, VerificationStatus::Achieved(level) if level.rung() == Some(read.achieved_rung))
        {
            return Err(violation(operation));
        }
        check_export_response(&request, &read).map_err(|_| violation(operation))?;
        Ok(read)
    }
}
