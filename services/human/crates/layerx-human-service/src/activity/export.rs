use std::collections::BTreeSet;
use std::fmt::{Display, Formatter, Write as _};

use layerx_client::sequencer::ReceiptKeyEntry;
use layerx_proof::export::{
    CheckpointFact, DerivedAggregate, ExportVerificationError, InclusionFact, InclusionKind,
    VerificationReport,
};
use layerx_proof::inclusion::{verify_activity, verify_state, SequencerAuthorization};
use layerx_proof::merkle::{decode_proof, encode_proof};
use layerx_proof::receipt::verify_sequencer_signature;
use layerx_types::rung::Rung;
use sha2::{Digest as _, Sha256};

use crate::audit::{verify_export as verify_audit, AuditChain, AuditError};
use crate::notify::ActivityEntryId;
use crate::store::{PrincipalId, PrincipalScope};

use super::{
    ActivityEntry, ActivityKind, AppliedFilters, Feed, FeedCursor, FeedError, PageRequest,
};

const PAGE_SIZE: usize = 100;
const MAXIMUM_EXPORT_BYTES: usize = 64 * 1024 * 1024;
const PRINCIPAL_DOMAIN: &[u8] = b"layerx-human-export-principal/v1";
const CSV_HEADER: &str =
    "entry_id,kind,status,occurred_at,projected_at,verification,receipt_references\r\n";
const BUNDLE_MAGIC: &[u8; 8] = b"LXHEXPV2";

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct StatementExport {
    content: Vec<u8>,
    entries: usize,
}

impl StatementExport {
    #[must_use]
    pub fn content(&self) -> &[u8] {
        &self.content
    }

    #[must_use]
    pub const fn content_type() -> &'static str {
        "text/csv; charset=utf-8"
    }

    #[must_use]
    pub const fn entries(&self) -> usize {
        self.entries
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ReceiptFact {
    pub statement: String,
    pub canonical_receipt_bytes: Vec<u8>,
    pub tx_hash: [u8; 32],
    pub receipt_hash: [u8; 32],
    pub block_height: u64,
    pub rung: Rung,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ReceiptExport {
    pub receipts: Vec<ReceiptFact>,
    pub inclusions: Vec<InclusionFact>,
    pub checkpoints: Vec<CheckpointFact>,
    pub derived_aggregates: Vec<DerivedAggregate>,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct EvidenceEntry {
    entry_id: ActivityEntryId,
    receipt_references: Vec<String>,
    tx_hash: [u8; 32],
    receipt_hash: [u8; 32],
    block_height: u64,
    rung: Rung,
}

impl EvidenceEntry {
    #[must_use]
    pub const fn entry_id(&self) -> &ActivityEntryId {
        &self.entry_id
    }
    #[must_use]
    pub fn receipt_references(&self) -> &[String] {
        &self.receipt_references
    }
    #[must_use]
    pub const fn tx_hash(&self) -> [u8; 32] {
        self.tx_hash
    }
    #[must_use]
    pub const fn receipt_hash(&self) -> [u8; 32] {
        self.receipt_hash
    }
    #[must_use]
    pub const fn block_height(&self) -> u64 {
        self.block_height
    }
    #[must_use]
    pub const fn rung(&self) -> Rung {
        self.rung
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct EvidenceBundle {
    principal_binding: [u8; 32],
    entries: Vec<EvidenceEntry>,
    protocol_evidence: Vec<ReceiptExport>,
    audit_export: Option<Vec<u8>>,
    bounded_bytes: usize,
}

impl EvidenceBundle {
    pub fn encode(&self) -> Result<Vec<u8>, ExportError> {
        let mut out = Vec::new();
        out.extend_from_slice(BUNDLE_MAGIC);
        out.push(2);
        out.extend_from_slice(&self.principal_binding);
        push_u32(&mut out, self.entries.len())?;
        for entry in &self.entries {
            push_bytes(&mut out, entry.entry_id.as_str().as_bytes())?;
            push_u32(&mut out, entry.receipt_references.len())?;
            for reference in &entry.receipt_references {
                push_bytes(&mut out, reference.as_bytes())?;
            }
            out.extend_from_slice(&entry.tx_hash);
            out.extend_from_slice(&entry.receipt_hash);
            out.extend_from_slice(&entry.block_height.to_be_bytes());
            out.push(rung_byte(entry.rung));
        }
        push_u32(&mut out, self.protocol_evidence.len())?;
        for export in &self.protocol_evidence {
            if !export.checkpoints.is_empty() || !export.derived_aggregates.is_empty() {
                return Err(ExportError::UnboundProtocolEvidence);
            }
            push_u32(&mut out, export.receipts.len())?;
            for receipt in &export.receipts {
                push_bytes(&mut out, receipt.statement.as_bytes())?;
                push_bytes(&mut out, &receipt.canonical_receipt_bytes)?;
                out.extend_from_slice(&receipt.tx_hash);
                out.extend_from_slice(&receipt.receipt_hash);
                out.extend_from_slice(&receipt.block_height.to_be_bytes());
                out.push(rung_byte(receipt.rung));
            }
            push_u32(&mut out, export.inclusions.len())?;
            for fact in &export.inclusions {
                push_bytes(&mut out, fact.statement.as_bytes())?;
                out.push(match fact.kind {
                    InclusionKind::Activity => 0,
                    InclusionKind::State => 1,
                });
                push_bytes(&mut out, &fact.canonical_leaf_bytes)?;
                push_bytes(&mut out, &encode_proof(&fact.proof))?;
                out.extend_from_slice(&fact.named_root);
                push_bytes(&mut out, &fact.canonical_header_bytes)?;
                out.extend_from_slice(&fact.header_signature);
                out.extend_from_slice(&fact.sequencer_authorization.sequencer_id());
                out.extend_from_slice(&fact.sequencer_authorization.public_key());
                out.extend_from_slice(
                    &fact
                        .sequencer_authorization
                        .first_batch_number()
                        .to_be_bytes(),
                );
                out.extend_from_slice(
                    &fact
                        .sequencer_authorization
                        .last_batch_number()
                        .to_be_bytes(),
                );
            }
        }
        match &self.audit_export {
            Some(value) => {
                out.push(1);
                push_bytes(&mut out, value)?;
            }
            None => out.push(0),
        }
        require_bound(out.len(), MAXIMUM_EXPORT_BYTES)?;
        Ok(out)
    }

    pub fn decode(bytes: &[u8]) -> Result<Self, ExportError> {
        require_bound(bytes.len(), MAXIMUM_EXPORT_BYTES)?;
        let mut reader = BundleReader { bytes, offset: 0 };
        let magic = reader.take(8)?;
        if magic == b"LXHEXP01" || magic == b"LXHEXP02" {
            return Err(ExportError::UnsupportedVersion { version: 1 });
        }
        if magic != BUNDLE_MAGIC {
            return Err(ExportError::UnboundProtocolEvidence);
        }
        let version = reader.byte()?;
        if version != 2 {
            return Err(ExportError::UnsupportedVersion { version });
        }
        let principal_binding = reader.array()?;
        let entry_count = reader.count()?;
        let mut entries = Vec::new();
        for _ in 0..entry_count {
            let entry_id = ActivityEntryId::new(reader.string()?).map_err(FeedError::from)?;
            let count = reader.count()?;
            let mut refs = Vec::new();
            for _ in 0..count {
                refs.push(reader.string()?);
            }
            entries.push(EvidenceEntry {
                entry_id,
                receipt_references: refs,
                tx_hash: reader.array()?,
                receipt_hash: reader.array()?,
                block_height: u64::from_be_bytes(reader.array()?),
                rung: read_rung(reader.byte()?)?,
            });
        }
        let export_count = reader.count()?;
        let mut protocol_evidence = Vec::new();
        for _ in 0..export_count {
            let count = reader.count()?;
            let mut receipts = Vec::new();
            for _ in 0..count {
                receipts.push(ReceiptFact {
                    statement: reader.string()?,
                    canonical_receipt_bytes: reader.bytes()?.to_vec(),
                    tx_hash: reader.array()?,
                    receipt_hash: reader.array()?,
                    block_height: u64::from_be_bytes(reader.array()?),
                    rung: read_rung(reader.byte()?)?,
                });
            }
            let count = reader.count()?;
            let mut inclusions = Vec::new();
            for _ in 0..count {
                let statement = reader.string()?;
                let kind = match reader.byte()? {
                    0 => InclusionKind::Activity,
                    1 => InclusionKind::State,
                    _ => return Err(ExportError::UnboundProtocolEvidence),
                };
                inclusions.push(InclusionFact {
                    statement,
                    kind,
                    canonical_leaf_bytes: reader.bytes()?.to_vec(),
                    proof: decode_proof(reader.bytes()?)
                        .map_err(|_| ExportError::UnboundProtocolEvidence)?,
                    named_root: reader.array()?,
                    canonical_header_bytes: reader.bytes()?.to_vec(),
                    header_signature: reader.array()?,
                    sequencer_authorization: SequencerAuthorization::new(
                        reader.array()?,
                        reader.array()?,
                        u64::from_be_bytes(reader.array()?),
                        u64::from_be_bytes(reader.array()?),
                    ),
                });
            }
            protocol_evidence.push(ReceiptExport {
                receipts,
                inclusions,
                checkpoints: Vec::new(),
                derived_aggregates: Vec::new(),
            });
        }
        let audit_export = match reader.byte()? {
            0 => None,
            1 => Some(reader.bytes()?.to_vec()),
            _ => return Err(ExportError::UnboundProtocolEvidence),
        };
        if reader.offset != bytes.len() {
            return Err(ExportError::UnboundProtocolEvidence);
        }
        Ok(Self {
            principal_binding,
            entries,
            protocol_evidence,
            audit_export,
            bounded_bytes: bytes.len(),
        })
    }
    #[must_use]
    pub const fn principal_binding(&self) -> [u8; 32] {
        self.principal_binding
    }
    #[must_use]
    pub fn entries(&self) -> &[EvidenceEntry] {
        &self.entries
    }
    #[must_use]
    pub fn protocol_evidence(&self) -> &[ReceiptExport] {
        &self.protocol_evidence
    }
    #[must_use]
    pub fn audit_export(&self) -> Option<&[u8]> {
        self.audit_export.as_deref()
    }
    #[must_use]
    pub const fn bounded_bytes(&self) -> usize {
        self.bounded_bytes
    }
    pub fn digest(&self) -> Result<[u8; 32], ExportError> {
        Ok(Sha256::digest(self.encode()?).into())
    }

    pub fn receipt_fact(
        entry_id: &ActivityEntryId,
        canonical_receipt: Vec<u8>,
        rung: Rung,
    ) -> Result<ReceiptFact, VerifyError> {
        let decoded = layerx_wire::receipt::decode(&canonical_receipt)
            .map_err(|_| VerifyError::Unverified(UnverifiedReason::Tampered))?;
        let receipt = decoded
            .protocol()
            .ok_or(VerifyError::Unverified(UnverifiedReason::Tampered))?;
        let batch = receipt.batch_id();
        if batch[..24] != [0; 24] {
            return Err(VerifyError::Unverified(UnverifiedReason::Tampered));
        }
        let block_height = u64::from_be_bytes(
            batch[24..]
                .try_into()
                .map_err(|_| VerifyError::Unverified(UnverifiedReason::Tampered))?,
        );
        Ok(ReceiptFact {
            statement: format!(
                "activity {} receipt {}",
                entry_id.as_str(),
                receipt_reference(&canonical_receipt)
            ),
            tx_hash: receipt.activity_id(),
            receipt_hash: Sha256::digest(&canonical_receipt).into(),
            block_height,
            rung,
            canonical_receipt_bytes: canonical_receipt,
        })
    }

    pub fn receipt(
        entry_id: ActivityEntryId,
        receipt: ReceiptFact,
        principal: &PrincipalId,
    ) -> Result<Self, VerifyError> {
        let entry = entry_for_receipt(entry_id, &receipt);
        let mut bundle = Self {
            principal_binding: principal_binding(principal.as_str()),
            entries: vec![entry],
            protocol_evidence: vec![ReceiptExport {
                receipts: vec![receipt],
                inclusions: Vec::new(),
                checkpoints: Vec::new(),
                derived_aggregates: Vec::new(),
            }],
            audit_export: None,
            bounded_bytes: 0,
        };
        bundle.bounded_bytes = bundle.encode().map_err(VerifyError::Unavailable)?.len();
        Ok(bundle)
    }

    pub fn verify(
        &self,
        expected_digest: [u8; 32],
        principal: &PrincipalId,
        receipt_keys: &[ReceiptKeyEntry],
    ) -> Result<BundleReport, VerifyError> {
        if self.digest().map_err(VerifyError::Unavailable)? != expected_digest {
            return Err(VerifyError::Unverified(UnverifiedReason::DigestMismatch));
        }
        self.verify_bindings(principal, receipt_keys)
    }

    fn verify_bindings(
        &self,
        principal: &PrincipalId,
        receipt_keys: &[ReceiptKeyEntry],
    ) -> Result<BundleReport, VerifyError> {
        if principal_binding(principal.as_str()) != self.principal_binding {
            return Err(VerifyError::Unverified(UnverifiedReason::PrincipalMismatch));
        }
        let expected = referenced_receipts(&self.entries);
        for entry in &self.entries {
            if entry.receipt_references != [hex(entry.receipt_hash)] {
                return Err(VerifyError::Unverified(UnverifiedReason::Tampered));
            }
            let matches = self
                .protocol_evidence
                .iter()
                .flat_map(|export| &export.receipts)
                .filter(|fact| fact.receipt_hash == entry.receipt_hash)
                .collect::<Vec<_>>();
            if matches.len() != 1 || entry_for_receipt(entry.entry_id.clone(), matches[0]) != *entry
            {
                return Err(VerifyError::Unverified(UnverifiedReason::Tampered));
            }
            let fact = matches[0];
            if <[u8; 32]>::from(Sha256::digest(&fact.canonical_receipt_bytes)) != fact.receipt_hash
            {
                return Err(VerifyError::Unverified(UnverifiedReason::Tampered));
            }
            let decoded = Self::receipt_fact(
                &entry.entry_id,
                fact.canonical_receipt_bytes.clone(),
                fact.rung,
            )?;
            if decoded.tx_hash != fact.tx_hash || decoded.block_height != fact.block_height {
                return Err(VerifyError::Unverified(UnverifiedReason::AuthorityMismatch));
            }
        }
        let protocol = verify_protocol_set(&self.protocol_evidence, &expected, receipt_keys)
            .map_err(VerifyError::from)?;
        let audit = self
            .audit_export
            .as_deref()
            .map(verify_audit)
            .transpose()
            .map_err(|error| VerifyError::from(ExportError::Audit(error)))?;
        if audit.as_ref().is_some_and(|report| {
            principal_binding(report.principal().as_str()) != self.principal_binding
        }) {
            return Err(VerifyError::Unverified(UnverifiedReason::PrincipalMismatch));
        }
        Ok(BundleReport {
            entries: self.entries.len(),
            verified_receipts: protocol.iter().map(|r| r.verified_receipts).sum(),
            verified_inclusions: protocol.iter().map(|r| r.verified_inclusions).sum(),
            verified_checkpoints: 0,
            audit_entries: audit.map_or(0, |report| report.entries()),
        })
    }

    pub fn verify_receipt(
        receipt: &ReceiptFact,
        receipt_keys: &[ReceiptKeyEntry],
    ) -> Result<BundleReport, VerifyError> {
        if <[u8; 32]>::from(Sha256::digest(&receipt.canonical_receipt_bytes))
            != receipt.receipt_hash
        {
            return Err(VerifyError::Unverified(UnverifiedReason::DigestMismatch));
        }
        let decoded = layerx_wire::receipt::decode(&receipt.canonical_receipt_bytes)
            .map_err(|_| VerifyError::Unverified(UnverifiedReason::Tampered))?;
        let protocol = decoded
            .protocol()
            .ok_or(VerifyError::Unverified(UnverifiedReason::Tampered))?;
        let mut batch = [0; 32];
        batch[24..].copy_from_slice(&receipt.block_height.to_be_bytes());
        if protocol.activity_id() != receipt.tx_hash
            || protocol.batch_id() != batch
            || receipt.rung == Rung::Submitted
        {
            return Err(VerifyError::Unverified(UnverifiedReason::Tampered));
        }
        let matching = receipt_keys
            .iter()
            .filter(|key| key.key_id == protocol.context_hash())
            .collect::<Vec<_>>();
        if matching.is_empty() {
            return Err(VerifyError::Unavailable(
                ExportError::AuthorityUnavailable {
                    reference: hex(receipt.receipt_hash),
                },
            ));
        }
        if matching.len() != 1
            || matching[0].key_id == [0; 32]
            || matching[0].public_key == [0; 32]
            || receipt.block_height < matching[0].first_height
            || receipt.block_height > matching[0].last_height
        {
            return Err(VerifyError::Unverified(UnverifiedReason::AuthorityMismatch));
        }
        let keys = receipt_keys
            .iter()
            .map(|key| {
                SequencerAuthorization::new(
                    key.key_id,
                    key.public_key,
                    key.first_height,
                    key.last_height,
                )
            })
            .collect::<Vec<_>>();
        verify_sequencer_signature(&receipt.canonical_receipt_bytes, keys.as_slice()).map_err(
            |failure| {
                VerifyError::Unverified(
                    if failure.check == layerx_proof::receipt::ReceiptCheck::SequencerSignature {
                        UnverifiedReason::AuthorityMismatch
                    } else {
                        UnverifiedReason::Tampered
                    },
                )
            },
        )?;
        Ok(BundleReport {
            entries: 1,
            verified_receipts: 1,
            verified_inclusions: 0,
            verified_checkpoints: 0,
            audit_entries: 0,
        })
    }
}

fn entry_for_receipt(entry_id: ActivityEntryId, receipt: &ReceiptFact) -> EvidenceEntry {
    EvidenceEntry {
        entry_id,
        receipt_references: vec![hex(receipt.receipt_hash)],
        tx_hash: receipt.tx_hash,
        receipt_hash: receipt.receipt_hash,
        block_height: receipt.block_height,
        rung: receipt.rung,
    }
}
const fn rung_byte(rung: Rung) -> u8 {
    match rung {
        Rung::Submitted => 0,
        Rung::Receipted => 1,
        Rung::Committed => 2,
    }
}
fn read_rung(byte: u8) -> Result<Rung, ExportError> {
    match byte {
        0 => Ok(Rung::Submitted),
        1 => Ok(Rung::Receipted),
        2 => Ok(Rung::Committed),
        _ => Err(ExportError::UnboundProtocolEvidence),
    }
}

fn push_u32(out: &mut Vec<u8>, value: usize) -> Result<(), ExportError> {
    out.extend_from_slice(
        &u32::try_from(value)
            .map_err(|_| ExportError::SizeOverflow)?
            .to_be_bytes(),
    );
    Ok(())
}
fn push_bytes(out: &mut Vec<u8>, value: &[u8]) -> Result<(), ExportError> {
    if value.is_empty() {
        return Err(ExportError::UnboundProtocolEvidence);
    }
    push_u32(out, value.len())?;
    out.extend_from_slice(value);
    require_bound(out.len(), MAXIMUM_EXPORT_BYTES)
}
struct BundleReader<'a> {
    bytes: &'a [u8],
    offset: usize,
}
impl<'a> BundleReader<'a> {
    fn string(&mut self) -> Result<String, ExportError> {
        String::from_utf8(self.bytes()?.to_vec()).map_err(|_| ExportError::UnboundProtocolEvidence)
    }
    fn take(&mut self, n: usize) -> Result<&'a [u8], ExportError> {
        let end = self
            .offset
            .checked_add(n)
            .ok_or(ExportError::SizeOverflow)?;
        let value = self
            .bytes
            .get(self.offset..end)
            .ok_or(ExportError::UnboundProtocolEvidence)?;
        self.offset = end;
        Ok(value)
    }
    fn byte(&mut self) -> Result<u8, ExportError> {
        Ok(self.take(1)?[0])
    }
    fn array<const N: usize>(&mut self) -> Result<[u8; N], ExportError> {
        self.take(N)?
            .try_into()
            .map_err(|_| ExportError::UnboundProtocolEvidence)
    }
    fn count(&mut self) -> Result<usize, ExportError> {
        let value = u32::from_be_bytes(self.array()?);
        usize::try_from(value).map_err(|_| ExportError::SizeOverflow)
    }
    fn bytes(&mut self) -> Result<&'a [u8], ExportError> {
        let n = self.count()?;
        if n == 0 {
            return Err(ExportError::UnboundProtocolEvidence);
        }
        self.take(n)
    }
}

/// Counts reported by one successful verifier run. It is neither cloneable
/// nor copyable, so a verified result cannot be reused for other evidence.
#[derive(Debug, Eq, PartialEq)]
pub struct BundleReport {
    entries: usize,
    verified_receipts: usize,
    verified_inclusions: usize,
    verified_checkpoints: usize,
    audit_entries: usize,
}

impl BundleReport {
    #[must_use]
    pub const fn entries(&self) -> usize {
        self.entries
    }

    #[must_use]
    pub const fn verified_receipts(&self) -> usize {
        self.verified_receipts
    }

    #[must_use]
    pub const fn verified_inclusions(&self) -> usize {
        self.verified_inclusions
    }

    #[must_use]
    pub const fn verified_checkpoints(&self) -> usize {
        self.verified_checkpoints
    }

    #[must_use]
    pub const fn audit_entries(&self) -> usize {
        self.audit_entries
    }
}

/// Typed reason the verifier refused to label evidence receipt-verified.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum UnverifiedReason {
    Tampered,
    DigestMismatch,
    PrincipalMismatch,
    AuthorityMismatch,
}

impl UnverifiedReason {
    #[must_use]
    pub const fn as_str(self) -> &'static str {
        match self {
            Self::Tampered => "tampered",
            Self::DigestMismatch => "digest-mismatch",
            Self::PrincipalMismatch => "principal-mismatch",
            Self::AuthorityMismatch => "authority-mismatch",
        }
    }
}

/// Typed outcome of a verifier run that did not verify.
#[derive(Debug)]
pub enum VerifyError {
    Unverified(UnverifiedReason),
    Unavailable(ExportError),
}

impl Display for VerifyError {
    fn fmt(&self, formatter: &mut Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::Unverified(reason) => {
                write!(formatter, "evidence is unverified: {}", reason.as_str())
            }
            Self::Unavailable(error) => write!(formatter, "evidence verifier unavailable: {error}"),
        }
    }
}

impl std::error::Error for VerifyError {}

impl From<ExportError> for VerifyError {
    fn from(value: ExportError) -> Self {
        match value {
            ExportError::PrincipalMismatch => Self::Unverified(UnverifiedReason::PrincipalMismatch),
            ExportError::Protocol(_)
            | ExportError::Audit(_)
            | ExportError::EvidenceUnavailable { .. }
            | ExportError::UnexpectedEvidence { .. }
            | ExportError::DuplicateEvidence
            | ExportError::UnboundProtocolEvidence => Self::Unverified(UnverifiedReason::Tampered),
            ExportError::UnsupportedVersion { .. } => Self::Unverified(UnverifiedReason::Tampered),
            ExportError::Unverified(reason) => Self::Unverified(reason),
            ExportError::Feed(_)
            | ExportError::InvalidSizeBound
            | ExportError::SizeBoundExceeded { .. }
            | ExportError::SizeOverflow
            | ExportError::DuplicateEntry
            | ExportError::EntryNotFound
            | ExportError::AuthorityUnavailable { .. } => Self::Unavailable(value),
        }
    }
}

/// Verification status of one evidence row as the service may label it.
///
/// The receipt-verified verdict exists only as the return path of
/// [`EvidenceBundle::verify`]: the verdict field is private and the sole
/// constructor, [`verification_status`], consumes that verifier's typed
/// result, whose success value cannot be built outside this module.
///
/// ```
/// use layerx_human_service::activity::{verification_status, UnverifiedReason, VerifyError};
///
/// let status = verification_status(Err(VerifyError::Unverified(UnverifiedReason::Tampered)));
/// assert_eq!(status.label(), "unverified");
/// assert_eq!(status.unverified_reason(), Some(UnverifiedReason::Tampered));
/// assert!(!status.is_receipt_verified());
/// ```
///
/// Outside code cannot name the verdict:
///
/// ```compile_fail
/// use layerx_human_service::activity::VerificationStatus;
///
/// let forged = VerificationStatus { verdict: () };
/// ```
///
/// Outside code cannot forge the verifier's success value either:
///
/// ```compile_fail
/// use layerx_human_service::activity::{verification_status, BundleReport};
///
/// let forged = verification_status(Ok(BundleReport {
///     entries: 0,
///     verified_receipts: 0,
///     verified_inclusions: 0,
///     verified_checkpoints: 0,
///     audit_entries: 0,
/// }));
/// ```
#[derive(Debug, Eq, PartialEq)]
pub struct VerificationStatus {
    verdict: Verdict,
}

#[derive(Debug, Eq, PartialEq)]
enum Verdict {
    ReceiptVerified(BundleReport),
    Unverified(UnverifiedReason),
    Unavailable,
}

impl VerificationStatus {
    /// Wire-level label of this status.
    #[must_use]
    pub const fn label(&self) -> &'static str {
        match self.verdict {
            Verdict::ReceiptVerified(_) => "receipt-verified",
            Verdict::Unverified(_) => "unverified",
            Verdict::Unavailable => "unavailable",
        }
    }

    #[must_use]
    pub const fn is_receipt_verified(&self) -> bool {
        matches!(self.verdict, Verdict::ReceiptVerified(_))
    }

    #[must_use]
    pub const fn is_unavailable(&self) -> bool {
        matches!(self.verdict, Verdict::Unavailable)
    }

    #[must_use]
    pub const fn unverified_reason(&self) -> Option<UnverifiedReason> {
        match &self.verdict {
            Verdict::Unverified(reason) => Some(*reason),
            Verdict::ReceiptVerified(_) | Verdict::Unavailable => None,
        }
    }

    #[must_use]
    pub const fn report(&self) -> Option<&BundleReport> {
        match &self.verdict {
            Verdict::ReceiptVerified(report) => Some(report),
            Verdict::Unverified(_) | Verdict::Unavailable => None,
        }
    }
}

/// The only constructor of a verification status: it consumes the verifier's
/// typed result and maps success to receipt-verified, a typed refusal to
/// unverified and a verifier or authority failure to unavailable.
#[must_use]
pub fn verification_status(result: Result<BundleReport, VerifyError>) -> VerificationStatus {
    let verdict = match result {
        Ok(report) => Verdict::ReceiptVerified(report),
        Err(VerifyError::Unverified(reason)) => Verdict::Unverified(reason),
        Err(VerifyError::Unavailable(_)) => Verdict::Unavailable,
    };
    VerificationStatus { verdict }
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct EvidenceExport {
    feed: Feed,
    maximum_bytes: usize,
}

impl EvidenceExport {
    /// Creates an exporter with a finite output bound.
    ///
    /// # Errors
    ///
    /// Refuses zero and unreasonably large bounds.
    pub const fn new(feed: Feed, maximum_bytes: usize) -> Result<Self, ExportError> {
        if maximum_bytes == 0 || maximum_bytes > MAXIMUM_EXPORT_BYTES {
            Err(ExportError::InvalidSizeBound)
        } else {
            Ok(Self {
                feed,
                maximum_bytes,
            })
        }
    }

    /// Exports one CSV statement from the same applied filters as the feed.
    ///
    /// # Errors
    ///
    /// Returns feed, encoding and size-bound failures.
    pub fn statement(
        self,
        scope: &PrincipalScope<'_>,
        filters: &AppliedFilters,
        now: u64,
        observed_agent_head: u64,
    ) -> Result<StatementExport, ExportError> {
        let entries = self.select(scope, filters, &[], now, observed_agent_head)?;
        let mut content = Vec::with_capacity(CSV_HEADER.len());
        append_bounded(&mut content, CSV_HEADER.as_bytes(), self.maximum_bytes)?;
        for entry in &entries {
            let row = statement_row(entry);
            append_bounded(&mut content, row.as_bytes(), self.maximum_bytes)?;
        }
        Ok(StatementExport {
            content,
            entries: entries.len(),
        })
    }

    /// Exports selected activity and its exact independently verified protocol
    /// evidence, bound to the principal and independently loaded receipt keys,
    /// together with the verifier report that
    /// bound it.
    ///
    /// # Errors
    ///
    /// Refuses missing entries, missing or extra receipt evidence, invalid
    /// proofs, an authority the service did not load, cross-scope selections
    /// and outputs over the configured bound.
    #[allow(clippy::too_many_arguments)]
    pub fn evidence(
        self,
        scope: &PrincipalScope<'_>,
        filters: &AppliedFilters,
        entry_ids: &[ActivityEntryId],
        protocol_evidence: Vec<ReceiptExport>,
        receipt_keys: &[ReceiptKeyEntry],
        now: u64,
        observed_agent_head: u64,
    ) -> Result<(EvidenceBundle, BundleReport), ExportError> {
        let selected = self.select(scope, filters, entry_ids, now, observed_agent_head)?;
        let mut entries = Vec::new();
        for entry in &selected {
            for receipt in entry.receipts() {
                let reference = receipt.reference();
                let fact = protocol_evidence
                    .iter()
                    .flat_map(|export| &export.receipts)
                    .find(|fact| hex(fact.receipt_hash) == reference)
                    .ok_or_else(|| ExportError::EvidenceUnavailable {
                        reference: reference.to_owned(),
                    })?;
                if fact.rung != receipt.rung() {
                    return Err(ExportError::Unverified(UnverifiedReason::Tampered));
                }
                entries.push(entry_for_receipt(entry.entry_id().clone(), fact));
            }
        }
        let expected = referenced_receipts(&entries);
        verify_protocol_set(&protocol_evidence, &expected, receipt_keys)?;
        let bounded_bytes = evidence_size(&entries, &protocol_evidence, None)?;
        let mut bundle = EvidenceBundle {
            principal_binding: principal_binding(scope.principal().as_str()),
            entries,
            protocol_evidence,
            audit_export: None,
            bounded_bytes,
        };
        bundle.bounded_bytes = bundle.encode()?.len();
        require_bound(bundle.bounded_bytes, self.maximum_bytes)?;
        let report = bundle.verify_bindings(scope.principal(), receipt_keys)?;
        Ok((bundle, report))
    }

    /// Exports the principal's audit chain through the same independently
    /// verifiable bundle surface.
    ///
    /// # Errors
    ///
    /// Returns audit-chain verification, encoding and size-bound failures.
    pub fn audit(
        self,
        scope: &PrincipalScope<'_>,
        audit: &AuditChain,
    ) -> Result<EvidenceBundle, ExportError> {
        let audit_export = audit.export(scope)?;
        verify_audit(&audit_export)?;
        let bounded_bytes = evidence_size(&[], &[], Some(&audit_export))?;
        require_bound(bounded_bytes, self.maximum_bytes)?;
        let bundle = EvidenceBundle {
            principal_binding: principal_binding(scope.principal().as_str()),
            entries: Vec::new(),
            protocol_evidence: Vec::new(),
            audit_export: Some(audit_export),
            bounded_bytes,
        };
        let digest = bundle.digest()?;
        bundle.verify(digest, scope.principal(), &[])?;
        Ok(bundle)
    }

    fn select(
        self,
        scope: &PrincipalScope<'_>,
        filters: &AppliedFilters,
        entry_ids: &[ActivityEntryId],
        now: u64,
        observed_agent_head: u64,
    ) -> Result<Vec<ActivityEntry>, ExportError> {
        let requested = entry_ids
            .iter()
            .map(|id| id.as_str().to_owned())
            .collect::<BTreeSet<_>>();
        if requested.len() != entry_ids.len() {
            return Err(ExportError::DuplicateEntry);
        }
        let mut cursor: Option<FeedCursor> = None;
        let mut selected = Vec::new();
        loop {
            let mut request = PageRequest::new(PAGE_SIZE, filters.clone());
            if let Some(next) = cursor {
                request = request.after(next);
            }
            let page = self.feed.page(scope, request, now, observed_agent_head)?;
            selected.extend(
                page.entries()
                    .iter()
                    .filter(|entry| {
                        requested.is_empty() || requested.contains(entry.entry_id().as_str())
                    })
                    .cloned(),
            );
            let Some(next) = page.next().cloned() else {
                break;
            };
            cursor = Some(next);
        }
        if !requested.is_empty()
            && selected
                .iter()
                .map(|entry| entry.entry_id().as_str())
                .collect::<BTreeSet<_>>()
                .len()
                != requested.len()
        {
            return Err(ExportError::EntryNotFound);
        }
        Ok(selected)
    }
}

fn statement_row(entry: &ActivityEntry) -> String {
    let verification = entry
        .receipts()
        .iter()
        .map(|receipt| crate::server::HumanComponentServer::verification_level_name(receipt.rung()))
        .collect::<Vec<_>>()
        .join(";");
    let receipts = entry
        .receipts()
        .iter()
        .map(super::ReceiptEvidence::reference)
        .collect::<Vec<_>>()
        .join(";");
    [
        entry.entry_id().as_str().to_owned(),
        activity_kind(entry.kind()).to_owned(),
        entry.status().label().to_owned(),
        entry.occurred_at().to_string(),
        entry.projected_at().to_string(),
        verification,
        receipts,
    ]
    .iter()
    .map(|field| csv_field(field))
    .collect::<Vec<_>>()
    .join(",")
        + "\r\n"
}

const fn activity_kind(kind: ActivityKind) -> &'static str {
    match kind {
        ActivityKind::Deposit => "deposit",
        ActivityKind::Withdrawal => "withdrawal",
        ActivityKind::Movement => "movement",
        ActivityKind::AgentAction => "agent-action",
        ActivityKind::Approval => "approval",
        ActivityKind::Security => "security-event",
    }
}

fn csv_field(value: &str) -> String {
    let mut safe = value.to_owned();
    if safe
        .as_bytes()
        .first()
        .is_some_and(|byte| matches!(byte, b'=' | b'+' | b'-' | b'@' | b'\t' | b'\r'))
    {
        safe.insert(0, '\'');
    }
    if safe.contains([',', '"', '\r', '\n']) {
        format!("\"{}\"", safe.replace('"', "\"\""))
    } else {
        safe
    }
}

fn receipt_reference(canonical_receipt: &[u8]) -> String {
    hex(Sha256::digest(canonical_receipt))
}

fn referenced_receipts(entries: &[EvidenceEntry]) -> BTreeSet<String> {
    entries
        .iter()
        .flat_map(|entry| entry.receipt_references.iter().cloned())
        .collect()
}

pub fn verify_receipt_export(
    artifact: &ReceiptExport,
    receipt_keys: &[ReceiptKeyEntry],
) -> Result<VerificationReport, ExportError> {
    if !artifact.checkpoints.is_empty() || !artifact.derived_aggregates.is_empty() {
        return Err(ExportError::UnboundProtocolEvidence);
    }
    for receipt in &artifact.receipts {
        if receipt.statement.trim().is_empty() {
            return Err(ExportError::Protocol(
                ExportVerificationError::EmptyStatement,
            ));
        }
        EvidenceBundle::verify_receipt(receipt, receipt_keys).map_err(|error| match error {
            VerifyError::Unverified(UnverifiedReason::DigestMismatch) => {
                ExportError::Unverified(UnverifiedReason::Tampered)
            }
            other => ExportError::from(other),
        })?;
    }
    for (index, fact) in artifact.inclusions.iter().enumerate() {
        if fact.statement.trim().is_empty() {
            return Err(ExportError::Protocol(
                ExportVerificationError::EmptyStatement,
            ));
        }
        let evidence = match fact.kind {
            InclusionKind::Activity => verify_activity(
                &fact.canonical_leaf_bytes,
                &fact.proof,
                &fact.canonical_header_bytes,
                &fact.header_signature,
                &fact.sequencer_authorization,
            ),
            InclusionKind::State => verify_state(
                &fact.canonical_leaf_bytes,
                &fact.proof,
                &fact.named_root,
                &fact.canonical_header_bytes,
                &fact.header_signature,
                &fact.sequencer_authorization,
            ),
        }
        .map_err(|error| {
            ExportError::Protocol(ExportVerificationError::Inclusion { index, error })
        })?;
        let root = match fact.kind {
            InclusionKind::Activity => evidence.header().header().activity_merkle_root(),
            InclusionKind::State => evidence.header().header().resulting_state_root(),
        };
        if root != fact.named_root {
            return Err(ExportError::Protocol(
                ExportVerificationError::InclusionRoot { index },
            ));
        }
    }
    Ok(VerificationReport {
        verified_receipts: artifact.receipts.len(),
        verified_inclusions: artifact.inclusions.len(),
        verified_checkpoints: 0,
        receipt_digests: artifact.receipts.iter().map(|r| r.receipt_hash).collect(),
        achieved_levels: vec![
            layerx_types::verify::VerificationLevel::SEQUENCER_SIGNED;
            artifact.receipts.len()
        ],
        derived_aggregates_are_protocol_facts: false,
    })
}

fn verify_protocol_set(
    protocol: &[ReceiptExport],
    expected: &BTreeSet<String>,
    receipt_keys: &[ReceiptKeyEntry],
) -> Result<Vec<VerificationReport>, ExportError> {
    let mut provided = BTreeSet::new();
    let mut reports = Vec::with_capacity(protocol.len());
    for artifact in protocol {
        if artifact.receipts.is_empty() {
            return Err(ExportError::UnboundProtocolEvidence);
        }
        let report = verify_receipt_export(artifact, receipt_keys)?;
        for receipt in &artifact.receipts {
            let reference = hex(receipt.receipt_hash);
            if !provided.insert(reference.clone()) {
                return Err(ExportError::DuplicateEvidence);
            }
            if !expected.contains(&reference) {
                return Err(ExportError::UnexpectedEvidence { reference });
            }
        }
        reports.push(report);
    }
    if let Some(reference) = expected.difference(&provided).next() {
        return Err(ExportError::EvidenceUnavailable {
            reference: reference.clone(),
        });
    }
    Ok(reports)
}

fn evidence_size(
    entries: &[EvidenceEntry],
    protocol: &[ReceiptExport],
    audit: Option<&[u8]>,
) -> Result<usize, ExportError> {
    let mut size = 96_usize;
    for entry in entries {
        size = checked_add(size, entry.entry_id.as_str().len())?;
        for reference in &entry.receipt_references {
            size = checked_add(size, reference.len())?;
        }
    }
    for artifact in protocol {
        for receipt in &artifact.receipts {
            size = checked_add(size, receipt.statement.len())?;
            size = checked_add(size, receipt.canonical_receipt_bytes.len())?;
            size = checked_add(size, 192)?;
        }
        for inclusion in &artifact.inclusions {
            size = checked_add(size, inclusion.statement.len())?;
            size = checked_add(size, inclusion.canonical_leaf_bytes.len())?;
            size = checked_add(size, encode_proof(&inclusion.proof).len())?;
            size = checked_add(size, inclusion.canonical_header_bytes.len())?;
            size = checked_add(size, 256)?;
        }
        for checkpoint in &artifact.checkpoints {
            size = checked_add(size, checkpoint.statement.len())?;
            size = checked_add(
                size,
                checkpoint.certificate.checkpoint().header_bytes().len(),
            )?;
            size = checked_add(
                size,
                checkpoint.certificate.checkpoint().validity_proof().len(),
            )?;
            size = checked_add(
                size,
                checkpoint
                    .certificate
                    .attestations()
                    .len()
                    .saturating_mul(256),
            )?;
            for (chunk, proof) in &checkpoint.availability {
                size = checked_add(size, chunk.bytes.len())?;
                size = checked_add(size, encode_proof(proof).len())?;
                size = checked_add(size, 96)?;
            }
            size = checked_add(size, checkpoint.bonded_set.len().saturating_mul(96))?;
            size = checked_add(
                size,
                checkpoint
                    .registered_settlement_reference
                    .as_ref()
                    .map_or(0, Vec::len),
            )?;
            size = checked_add(size, 128)?;
        }
        for aggregate in &artifact.derived_aggregates {
            size = checked_add(size, aggregate.label.len())?;
            size = checked_add(size, aggregate.rendered_value.len())?;
            size = checked_add(
                size,
                aggregate
                    .contributing_receipt_digests
                    .len()
                    .saturating_mul(32),
            )?;
        }
    }
    checked_add(size, audit.map_or(0, <[u8]>::len))
}

fn append_bounded(output: &mut Vec<u8>, bytes: &[u8], maximum: usize) -> Result<(), ExportError> {
    let next = checked_add(output.len(), bytes.len())?;
    require_bound(next, maximum)?;
    output.extend_from_slice(bytes);
    Ok(())
}

const fn require_bound(actual: usize, maximum: usize) -> Result<(), ExportError> {
    if actual > maximum {
        Err(ExportError::SizeBoundExceeded { maximum })
    } else {
        Ok(())
    }
}

const fn checked_add(left: usize, right: usize) -> Result<usize, ExportError> {
    match left.checked_add(right) {
        Some(value) => Ok(value),
        None => Err(ExportError::SizeOverflow),
    }
}

fn principal_binding(principal: &str) -> [u8; 32] {
    let mut digest = Sha256::new();
    digest.update(PRINCIPAL_DOMAIN);
    digest.update(principal.as_bytes());
    digest.finalize().into()
}

fn hex(bytes: impl AsRef<[u8]>) -> String {
    let bytes = bytes.as_ref();
    let mut output = String::with_capacity(bytes.len().saturating_mul(2));
    for byte in bytes {
        let _ = write!(output, "{byte:02x}");
    }
    output
}

#[derive(Debug)]
pub enum ExportError {
    Feed(FeedError),
    Audit(AuditError),
    Protocol(ExportVerificationError),
    InvalidSizeBound,
    SizeBoundExceeded { maximum: usize },
    SizeOverflow,
    DuplicateEntry,
    EntryNotFound,
    EvidenceUnavailable { reference: String },
    UnexpectedEvidence { reference: String },
    DuplicateEvidence,
    UnboundProtocolEvidence,
    UnsupportedVersion { version: u8 },
    PrincipalMismatch,
    AuthorityUnavailable { reference: String },
    Unverified(UnverifiedReason),
}

impl Display for ExportError {
    fn fmt(&self, formatter: &mut Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::Feed(error) => write!(formatter, "activity export feed failure: {error}"),
            Self::Audit(error) => write!(formatter, "activity export audit failure: {error}"),
            Self::Protocol(error) => {
                write!(
                    formatter,
                    "activity export protocol evidence failed: {error:?}"
                )
            }
            Self::InvalidSizeBound => formatter.write_str("activity export size bound is invalid"),
            Self::SizeBoundExceeded { maximum } => {
                write!(
                    formatter,
                    "activity export exceeds its {maximum}-byte bound"
                )
            }
            Self::SizeOverflow => formatter.write_str("activity export size overflowed"),
            Self::DuplicateEntry => formatter.write_str("activity export repeats an entry"),
            Self::EntryNotFound => {
                formatter.write_str("activity export entry is outside the principal or filter")
            }
            Self::EvidenceUnavailable { reference } => {
                write!(formatter, "activity evidence {reference} is unavailable")
            }
            Self::UnexpectedEvidence { reference } => {
                write!(formatter, "activity evidence {reference} was not requested")
            }
            Self::DuplicateEvidence => formatter.write_str("activity evidence is duplicated"),
            Self::UnboundProtocolEvidence => {
                formatter.write_str("protocol evidence has no selected receipt binding")
            }
            Self::PrincipalMismatch => {
                formatter.write_str("audit export principal binding does not match")
            }
            Self::AuthorityUnavailable { reference } => {
                write!(
                    formatter,
                    "activity evidence {reference} has no loaded receipt authority"
                )
            }
            Self::UnsupportedVersion { version } => {
                write!(formatter, "unsupported evidence bundle version {version}")
            }
            Self::Unverified(reason) => {
                write!(
                    formatter,
                    "activity evidence is unverified: {}",
                    reason.as_str()
                )
            }
        }
    }
}

impl std::error::Error for ExportError {}

impl From<FeedError> for ExportError {
    fn from(value: FeedError) -> Self {
        Self::Feed(value)
    }
}

impl From<AuditError> for ExportError {
    fn from(value: AuditError) -> Self {
        Self::Audit(value)
    }
}

impl From<VerifyError> for ExportError {
    fn from(value: VerifyError) -> Self {
        match value {
            VerifyError::Unverified(reason) => Self::Unverified(reason),
            VerifyError::Unavailable(error) => error,
        }
    }
}
