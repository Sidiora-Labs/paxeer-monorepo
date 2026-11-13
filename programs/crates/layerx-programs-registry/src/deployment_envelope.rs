//! Canonical durable deployment envelopes shared by registry producers and recovery.

use crate::{hex, DeploymentProof, DeploymentRecord, VerifiedDeploymentEvidence};
use core::fmt::{self, Display};
use sha2::{Digest as _, Sha256};

const ENVELOPE_DOMAIN: &[u8] = b"LayerX/deployment-envelope/v1\0";
const SEAL_BYTES: usize = 32;

pub type DeploymentEnvelopeFrames = (Vec<u8>, Vec<u8>, [u8; SEAL_BYTES]);

/// One accepted deployment record together with the protocol admission proof
/// it was derived from. The two are published to the journal as a single
/// commit unit: one canonical envelope staged in a temporary file, fsynced and
/// renamed into place, so a restart observes both halves or neither.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct DeploymentEnvelope {
    receipt_digest: [u8; 32],
    record: DeploymentRecord,
    proof: DeploymentProof,
}

impl DeploymentEnvelope {
    /// Seals the record and proof carried by cryptographically verified
    /// evidence.
    #[must_use]
    pub fn from_evidence(evidence: &VerifiedDeploymentEvidence) -> Self {
        Self {
            receipt_digest: evidence.receipt_digest(),
            record: evidence.record().clone(),
            proof: evidence.proof().clone(),
        }
    }

    /// Pairs a decoded record with the proof it was filed beside.
    ///
    /// # Errors
    ///
    /// Refuses a proof whose receipt cannot be digested.
    pub fn pair(record: DeploymentRecord, proof: DeploymentProof) -> Result<Self, UnitDefect> {
        let entry =
            crate::DeploymentJournalEntry::from_proof(record.clone(), &proof).map_err(|error| {
                UnitDefect::Corrupt {
                    part: UnitPart::Record,
                    reason: error.to_string(),
                }
            })?;
        let receipt_digest =
            proof
                .claimed_receipt_digest()
                .map_err(|error| UnitDefect::Corrupt {
                    part: UnitPart::Proof,
                    reason: error.to_string(),
                })?;
        if receipt_digest != entry.transaction_id().bytes() {
            return Err(UnitDefect::Corrupt {
                part: UnitPart::Record,
                reason: "deployment provenance mismatch".into(),
            });
        }
        Ok(Self {
            receipt_digest,
            record,
            proof,
        })
    }

    /// Returns the receipt digest the unit is filed under.
    #[must_use]
    pub const fn receipt_digest(&self) -> [u8; 32] {
        self.receipt_digest
    }

    #[must_use]
    pub const fn record(&self) -> &DeploymentRecord {
        &self.record
    }

    #[must_use]
    pub const fn proof(&self) -> &DeploymentProof {
        &self.proof
    }

    ///
    /// # Errors
    ///
    /// Returns an error when the deployment input cannot be validated or encoded.
    pub fn journal_entry(&self) -> Result<crate::DeploymentJournalEntry, String> {
        crate::DeploymentJournalEntry::from_proof(self.record.clone(), &self.proof)
            .map_err(|error| error.to_string())
    }

    #[must_use]
    pub fn into_proof(self) -> DeploymentProof {
        self.proof
    }

    /// Encodes the unit: the envelope domain, the framed canonical record, the
    /// framed canonical proof and a seal over everything before it.
    ///
    /// # Errors
    ///
    /// Returns an error when the deployment input cannot be validated or encoded.
    pub fn canonical_encoding(&self) -> Result<Vec<u8>, String> {
        let (mut bytes, proof_frame, seal) = self.frames()?;
        bytes.extend_from_slice(&proof_frame);
        bytes.extend_from_slice(&seal);
        Ok(bytes)
    }

    /// Decodes one committed unit.
    ///
    /// # Errors
    ///
    /// Names the part the bytes end before, the part that does not decode, or
    /// a seal that does not cover the bytes.
    pub fn decode(bytes: &[u8]) -> Result<Self, UnitDefect> {
        let (record_bytes, proof_bytes) = sealed_frames(bytes)?;
        let record = decode_record(record_bytes)?;
        let proof = DeploymentProof::decode(proof_bytes).map_err(|error| UnitDefect::Corrupt {
            part: UnitPart::Proof,
            reason: error.to_string(),
        })?;
        Self::pair(record, proof)
    }

    ///
    /// # Errors
    ///
    /// Returns an error when the deployment input cannot be validated or encoded.
    pub fn frames(&self) -> Result<DeploymentEnvelopeFrames, String> {
        let record = self.record.canonical_encoding();
        let proof = self
            .proof
            .canonical_encoding()
            .map_err(|error| error.to_string())?;
        let mut record_frame = Vec::with_capacity(ENVELOPE_DOMAIN.len() + 4 + record.len());
        record_frame.extend_from_slice(ENVELOPE_DOMAIN);
        put_frame(&mut record_frame, &record);
        let mut proof_frame = Vec::with_capacity(4 + proof.len());
        put_frame(&mut proof_frame, &proof);
        let mut seal = Sha256::new();
        seal.update(&record_frame);
        seal.update(&proof_frame);
        Ok((record_frame, proof_frame, seal.finalize().into()))
    }
}

fn put_frame(bytes: &mut Vec<u8>, frame: &[u8]) {
    bytes.extend_from_slice(&u32::try_from(frame.len()).unwrap_or(u32::MAX).to_be_bytes());
    bytes.extend_from_slice(frame);
}

fn take_frame<'a>(bytes: &'a [u8], cursor: &mut usize) -> Option<&'a [u8]> {
    let start = cursor.checked_add(4)?;
    let prefix: [u8; 4] = bytes.get(*cursor..start)?.try_into().ok()?;
    let length = usize::try_from(u32::from_be_bytes(prefix)).ok()?;
    let end = start.checked_add(length)?;
    let frame = bytes.get(start..end)?;
    *cursor = end;
    Some(frame)
}

///
/// # Errors
///
/// Returns an error when the deployment input cannot be validated or encoded.
pub fn sealed_frames(bytes: &[u8]) -> Result<(&[u8], &[u8]), UnitDefect> {
    let domain = bytes.get(..ENVELOPE_DOMAIN.len()).unwrap_or(bytes);
    if !ENVELOPE_DOMAIN.starts_with(domain) {
        return Err(UnitDefect::Corrupt {
            part: UnitPart::Record,
            reason: "the envelope carries a foreign domain".to_owned(),
        });
    }
    if domain.len() < ENVELOPE_DOMAIN.len() {
        return Err(UnitDefect::Missing(UnitPart::Record));
    }
    let mut cursor = ENVELOPE_DOMAIN.len();
    let record = take_frame(bytes, &mut cursor).ok_or(UnitDefect::Missing(UnitPart::Record))?;
    let proof = take_frame(bytes, &mut cursor).ok_or(UnitDefect::Missing(UnitPart::Proof))?;
    let sealed = cursor;
    let seal = bytes
        .get(sealed..sealed.saturating_add(SEAL_BYTES))
        .filter(|seal| seal.len() == SEAL_BYTES)
        .ok_or(UnitDefect::Missing(UnitPart::Seal))?;
    if sealed.saturating_add(SEAL_BYTES) != bytes.len() {
        return Err(UnitDefect::Corrupt {
            part: UnitPart::Seal,
            reason: "the envelope carries bytes after its seal".to_owned(),
        });
    }
    let expected: [u8; SEAL_BYTES] = Sha256::digest(&bytes[..sealed]).into();
    if seal != expected {
        return Err(UnitDefect::Corrupt {
            part: UnitPart::Seal,
            reason: "the seal does not cover the record and proof".to_owned(),
        });
    }
    Ok((record, proof))
}

///
/// # Errors
///
/// Returns an error when the deployment input cannot be validated or encoded.
pub fn decode_record(bytes: &[u8]) -> Result<DeploymentRecord, UnitDefect> {
    let record = DeploymentRecord::decode(bytes).map_err(|error| UnitDefect::Corrupt {
        part: UnitPart::Record,
        reason: error.to_string(),
    })?;
    record.validate().map_err(|error| UnitDefect::Corrupt {
        part: UnitPart::Record,
        reason: error.to_string(),
    })?;
    Ok(record)
}

/// One half of a commit unit, its seal, or the commit that publishes it.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum UnitPart {
    Record,
    Proof,
    Seal,
    Commit,
}

impl Display for UnitPart {
    fn fmt(&self, formatter: &mut fmt::Formatter<'_>) -> fmt::Result {
        formatter.write_str(match self {
            Self::Record => "deployment record",
            Self::Proof => "admission proof",
            Self::Seal => "seal",
            Self::Commit => "commit",
        })
    }
}

/// Why one unit was set aside instead of loaded. Every variant names what the
/// unit is missing or which part failed.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum UnitDefect {
    /// Publication stopped before the unit was committed; the interrupted
    /// attempt ends before this part.
    Interrupted(UnitPart),
    /// A committed unit ends before this part.
    Missing(UnitPart),
    /// A committed part does not decode.
    Corrupt { part: UnitPart, reason: String },
    /// The proof names a different receipt digest than the file it is filed
    /// under.
    Misfiled { claimed: [u8; 32] },
    /// A unit file could not be read.
    Unreadable(String),
}

impl Display for UnitDefect {
    fn fmt(&self, formatter: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::Interrupted(part) => {
                write!(formatter, "publication was interrupted before its {part}")
            }
            Self::Missing(part) => write!(formatter, "the committed unit has no {part}"),
            Self::Corrupt { part, reason } => {
                write!(formatter, "its {part} does not decode: {reason}")
            }
            Self::Misfiled { claimed } => write!(
                formatter,
                "its proof is filed under a different receipt digest {}",
                hex::encode(claimed)
            ),
            Self::Unreadable(reason) => write!(formatter, "it could not be read: {reason}"),
        }
    }
}
