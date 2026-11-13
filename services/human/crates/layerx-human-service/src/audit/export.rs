use std::collections::BTreeMap;

use layerx_proof::merkle::leaf_hash;

use crate::store::{PrincipalId, PrincipalScope, RowKey, Table};

use super::wire::{push_bytes, push_length, Reader};
use super::{
    decode_entry, evidence_digest, genesis_link, table_code, table_from_code, AuditChain,
    AuditError, ChainHead,
};

const EXPORT_MAGIC: &[u8; 4] = b"LXAX";
const EXPORT_VERSION: u8 = 1;

/// Maximum encoded bytes accepted by either audit export verifier (16 MiB).
pub const EXPORT_IMPORT_MAX_BYTES: usize = 16 * 1024 * 1024;
/// Maximum entry frames accepted by either audit export verifier.
pub const EXPORT_IMPORT_MAX_ENTRIES: usize = 65_536;
/// Maximum evidence rows accepted by either audit export verifier.
pub const EXPORT_IMPORT_MAX_EVIDENCE_ROWS: usize = 65_536;

struct ImportedEvidence<'a> {
    key: (Table, RowKey),
    written_at: u64,
    bytes: &'a [u8],
    used: bool,
}

#[derive(Clone, Debug, Eq, PartialEq)]
struct EvidenceRow {
    written_at: u64,
    bytes: Vec<u8>,
}

/// An internally consistent audit export summary.
///
/// A report from [`verify_export`] is consistency-only: its principal and head
/// are self-declared. Authenticity and completeness require
/// [`verify_export_anchored`] with an independently authenticated commitment.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ExportReport {
    principal: PrincipalId,
    head: ChainHead,
    entries: usize,
    evidence_rows: usize,
}

impl ExportReport {
    /// Returns the principal declared by the consistent bundle.
    #[must_use]
    pub const fn principal(&self) -> &PrincipalId {
        &self.principal
    }

    /// Returns the internally verified head declared by the bundle.
    #[must_use]
    pub const fn head(&self) -> ChainHead {
        self.head
    }

    /// Returns the number of verified audit entries.
    #[must_use]
    pub const fn entries(&self) -> usize {
        self.entries
    }

    /// Returns the number of distinct evidence rows verified by the bundle.
    #[must_use]
    pub const fn evidence_rows(&self) -> usize {
        self.evidence_rows
    }
}

pub(super) fn build(scope: &PrincipalScope<'_>, chain: &AuditChain) -> Result<Vec<u8>, AuditError> {
    let entries = chain.entries(scope)?;
    let mut evidence = BTreeMap::new();
    for entry in &entries {
        for binding in entry.evidence() {
            let row = scope
                .get(binding.table(), binding.key())
                .ok_or(AuditError::Corrupt("dangling export evidence"))?;
            let exported = EvidenceRow {
                written_at: row.written_at(),
                bytes: row.bytes().to_vec(),
            };
            let key = (binding.table(), binding.key().clone());
            if evidence
                .insert(key, exported.clone())
                .is_some_and(|old| old != exported)
            {
                return Err(AuditError::Corrupt("conflicting export evidence"));
            }
        }
    }

    let mut output = Vec::new();
    output.extend_from_slice(EXPORT_MAGIC);
    output.push(EXPORT_VERSION);
    push_bytes(&mut output, scope.principal().as_str().as_bytes())?;
    push_bytes(&mut output, &chain.head().encode())?;
    push_length(&mut output, entries.len())?;
    for entry in &entries {
        push_bytes(&mut output, entry.bytes())?;
    }
    push_length(&mut output, evidence.len())?;
    for ((table, key), row) in evidence {
        output.push(table_code(table));
        push_bytes(&mut output, key.as_str().as_bytes())?;
        output.extend_from_slice(&row.written_at.to_be_bytes());
        push_bytes(&mut output, &row.bytes)?;
    }
    Ok(output)
}

/// Checks internal consistency of an exported chain and its referenced evidence.
///
/// This consistency-only check does not authenticate the principal or establish
/// completeness: a replacement chain or an older valid prefix can pass. Use
/// [`verify_export_anchored`] for verification against an external commitment.
///
/// # Errors
///
/// Refuses malformed bundles, missing or extraneous evidence, broken chain
/// links, altered bytes, inconsistent heads and imports exceeding the explicit
/// byte, entry or evidence budgets.
pub fn verify_export(bytes: &[u8]) -> Result<ExportReport, AuditError> {
    verify(bytes, None)
}

/// Verifies authenticity and completeness against an independently authenticated
/// principal and chain head, as well as internal consistency and evidence.
///
/// The caller must retain or authenticate both anchor values independently of
/// the bundle and its producer; taking them from the bundle provides no trust.
/// The declared principal and head must match before entry allocation or acceptance.
///
/// # Errors
///
/// Returns the same refusals as [`verify_export`], and refuses a different
/// principal, a replacement head or a truncated chain relative to the anchor.
pub fn verify_export_anchored(
    bytes: &[u8],
    expected_principal: &PrincipalId,
    expected_head: ChainHead,
) -> Result<ExportReport, AuditError> {
    verify(bytes, Some((expected_principal, expected_head)))
}

fn read_evidence<'a>(
    reader: &mut Reader<'a>,
) -> Result<(usize, Vec<ImportedEvidence<'a>>), AuditError> {
    let evidence_count = reader.length()?;
    if evidence_count > EXPORT_IMPORT_MAX_EVIDENCE_ROWS {
        return Err(AuditError::Corrupt(
            "audit export exceeds import evidence budget",
        ));
    }
    // Table byte, key frame, timestamp and data frame, even with empty payloads.
    if evidence_count > reader.remaining() / (1 + 4 + 8 + 4) {
        return Err(AuditError::Corrupt(
            "export evidence count exceeds framed bytes",
        ));
    }
    let mut evidence = Vec::new();
    evidence
        .try_reserve_exact(evidence_count)
        .map_err(|_| AuditError::SizeOverflow)?;
    for _ in 0..evidence_count {
        let table = table_from_code(reader.byte()?)?;
        let key_text = std::str::from_utf8(reader.bytes()?)
            .map_err(|_| AuditError::Corrupt("export evidence key is not UTF-8"))?;
        let key = RowKey::new(key_text)
            .map_err(|_| AuditError::Corrupt("invalid export evidence key"))?;
        evidence.push(ImportedEvidence {
            key: (table, key),
            written_at: reader.u64()?,
            bytes: reader.bytes()?,
            used: false,
        });
    }
    if !reader.is_empty() {
        return Err(AuditError::Corrupt("trailing audit export bytes"));
    }

    // Sorting is in-place; unlike a tree it needs no infallible per-row nodes.
    evidence.sort_unstable_by(|left, right| left.key.cmp(&right.key));
    if evidence.windows(2).any(|rows| rows[0].key == rows[1].key) {
        return Err(AuditError::Corrupt("duplicate export evidence"));
    }
    Ok((evidence_count, evidence))
}

fn verify_anchor(
    principal: &PrincipalId,
    head: ChainHead,
    anchor: Option<(&PrincipalId, ChainHead)>,
) -> Result<(), AuditError> {
    if let Some((expected_principal, expected_head)) = anchor {
        if principal != expected_principal {
            return Err(AuditError::Corrupt(
                "export principal does not match anchor",
            ));
        }
        if head != expected_head {
            return Err(AuditError::HeadMismatch {
                expected: expected_head,
                found: head,
            });
        }
    }
    Ok(())
}

fn verify(
    bytes: &[u8],
    anchor: Option<(&PrincipalId, ChainHead)>,
) -> Result<ExportReport, AuditError> {
    if bytes.len() > EXPORT_IMPORT_MAX_BYTES {
        return Err(AuditError::Corrupt(
            "audit export exceeds import byte budget",
        ));
    }
    let mut reader = Reader::new(bytes);
    if reader.take(4)? != EXPORT_MAGIC {
        return Err(AuditError::Corrupt("invalid audit export header"));
    }
    if reader.byte()? != EXPORT_VERSION {
        return Err(AuditError::Corrupt("unknown audit export version"));
    }
    let principal_text = std::str::from_utf8(reader.bytes()?)
        .map_err(|_| AuditError::Corrupt("export principal is not UTF-8"))?;
    let principal = PrincipalId::new(principal_text)
        .map_err(|_| AuditError::Corrupt("invalid export principal"))?;
    let head = ChainHead::decode(reader.bytes()?)?;

    verify_anchor(&principal, head, anchor)?;

    let entry_count = reader.length()?;
    // Every entry needs its four-byte frame, followed by the evidence count.
    if entry_count > EXPORT_IMPORT_MAX_ENTRIES {
        return Err(AuditError::Corrupt(
            "audit export exceeds import entry budget",
        ));
    }
    let entry_frames = reader
        .remaining()
        .checked_sub(4)
        .ok_or(AuditError::Corrupt("missing export evidence count"))?;
    if entry_count > entry_frames / 4 {
        return Err(AuditError::Corrupt(
            "export entry count exceeds framed bytes",
        ));
    }
    let mut entry_bytes = Vec::new();
    entry_bytes
        .try_reserve_exact(entry_count)
        .map_err(|_| AuditError::SizeOverflow)?;
    for _ in 0..entry_count {
        entry_bytes.push(reader.bytes()?);
    }

    let (evidence_count, mut evidence) = read_evidence(&mut reader)?;

    if u64::try_from(entry_count).map_err(|_| AuditError::SizeOverflow)? != head.length() {
        return Err(AuditError::HeadMismatch {
            expected: head,
            found: ChainHead {
                length: u64::try_from(entry_count).map_err(|_| AuditError::SizeOverflow)?,
                link: head.link(),
            },
        });
    }

    let mut link = genesis_link(&principal)?;
    for (position, encoded) in entry_bytes.iter().enumerate() {
        let sequence = u64::try_from(position).map_err(|_| AuditError::SizeOverflow)?;
        let body = decode_entry(encoded)?;
        if body.sequence != sequence {
            return Err(AuditError::SequenceMismatch { sequence });
        }
        if body.prev_link != link {
            return Err(AuditError::LinkMismatch { sequence });
        }
        for binding in body.evidence {
            let position = evidence
                .binary_search_by(|row| {
                    row.key
                        .0
                        .cmp(&binding.table())
                        .then_with(|| row.key.1.cmp(binding.key()))
                })
                .map_err(|_| AuditError::EvidenceMismatch { sequence })?;
            let row = &mut evidence[position];
            if evidence_digest(row.written_at, row.bytes)? != binding.digest() {
                return Err(AuditError::EvidenceDigestMismatch { sequence });
            }
            row.used = true;
        }
        link = leaf_hash(encoded).map_err(|_| AuditError::Unhashable)?;
    }
    if link != head.link() {
        return Err(AuditError::HeadMismatch {
            expected: head,
            found: ChainHead {
                length: head.length(),
                link,
            },
        });
    }
    if evidence.iter().any(|row| !row.used) {
        return Err(AuditError::Corrupt("unreferenced export evidence"));
    }

    Ok(ExportReport {
        principal,
        head,
        entries: entry_count,
        evidence_rows: evidence_count,
    })
}
