//! Offline verification rooted only in a finalised mirror archive.

use layerx_proof::inclusion::SequencerAuthorization;
use layerx_proof::receipt::{verify_sequencer_signature, ReceiptCheck, ReceiptKeys};
use layerx_wire::receipt::Receipt;

use crate::committed::CommittedCoordinate;

use crate::source::{MirrorLag, MirrorObservation, MirrorSourceFreshness, ObservedArchive};
use crate::{ArchiveData, ArchiveError, CommittedFreshness, MirrorFreshness};

/// Mirror coordinates displayed with every mirror-derived result.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct MirrorVerificationFreshness {
    pub latest_batch_mirrored: Option<u64>,
    pub latest_committed_mirrored: Option<CommittedCoordinate>,
    pub batch_lag: MirrorLag,
    pub committed: Option<CommittedFreshness>,
}

impl MirrorVerificationFreshness {
    #[must_use]
    pub const fn offline(
        latest_batch_mirrored: u64,
        latest_committed_mirrored: Option<CommittedCoordinate>,
    ) -> Self {
        Self {
            latest_batch_mirrored: Some(latest_batch_mirrored),
            latest_committed_mirrored,
            batch_lag: MirrorLag::Unknown,
            committed: None,
        }
    }

    #[must_use]
    pub fn relative(value: MirrorFreshness) -> Self {
        Self {
            latest_batch_mirrored: value.latest_batch_mirrored,
            latest_committed_mirrored: value.latest_committed_mirrored,
            batch_lag: MirrorLag::Known(value.batch_lag),
            committed: Some(value.committed),
        }
    }

    #[must_use]
    pub const fn from_source(value: MirrorSourceFreshness) -> Self {
        Self {
            latest_batch_mirrored: value.latest_batch,
            latest_committed_mirrored: value.latest_committed,
            batch_lag: value.batch_lag,
            committed: None,
        }
    }
}

/// Evidence established from the archive commitment and an independently
/// configured sequencer key, without a `LayerX` RPC or hosted service.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct MirrorVerification<T> {
    value: T,
    level: MirrorEvidenceLevel,
    batch_number: u64,
    freshness: MirrorVerificationFreshness,
    observation: Option<MirrorObservation>,
}

impl<T> MirrorVerification<T> {
    #[must_use]
    pub const fn value(&self) -> &T {
        &self.value
    }

    #[must_use]
    pub const fn level(&self) -> MirrorEvidenceLevel {
        self.level
    }

    #[must_use]
    pub const fn batch_number(&self) -> u64 {
        self.batch_number
    }

    #[must_use]
    pub const fn freshness(&self) -> MirrorVerificationFreshness {
        self.freshness
    }

    #[must_use]
    pub fn observation(&self) -> Option<&MirrorObservation> {
        self.observation.as_ref()
    }
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum MirrorEvidenceLevel {
    Committed,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum MirrorVerifyError {
    Archive(ArchiveError),
    Header,
    HeaderAuthority,
    HeaderSignature,
    ReceiptMissing,
    ReceiptDecode,
    ReceiptBatchMismatch,
    Receipt(ReceiptCheck),
    SourceCommitment,
    SourceBatch,
    SourceReorged,
    CheckpointTrustUnavailable,
    CommittedUnavailable,
}

/// Offline verifier over untrusted archive bytes and separately configured authority.
pub struct MirrorVerifier {
    archive: ArchiveData,
    freshness: MirrorVerificationFreshness,
    observation: Option<MirrorObservation>,
}

impl MirrorVerifier {
    /// Admits a committed archive without requiring a signed batch header.
    ///
    /// # Errors
    /// Refuses mismatched source commitments or heights and archives without committed evidence.
    pub fn from_committed_source(observed: ObservedArchive) -> Result<Self, MirrorVerifyError> {
        let (archive_bytes, observation) = observed.into_parts();
        Self::admit_committed(&archive_bytes, observation)
    }

    pub fn admit_committed(
        archive_bytes: &[u8],
        observation: MirrorObservation,
    ) -> Result<Self, MirrorVerifyError> {
        if crate::archive_commitment(archive_bytes) != observation.commitment {
            return Err(MirrorVerifyError::SourceCommitment);
        }
        let archive = ArchiveData::decode(archive_bytes).map_err(MirrorVerifyError::Archive)?;
        if observation.batch_number != archive.batch_number {
            return Err(MirrorVerifyError::SourceBatch);
        }
        let freshness = MirrorVerificationFreshness::from_source(observation.freshness);
        if freshness
            .latest_batch_mirrored
            .is_some_and(|latest| latest < archive.batch_number)
        {
            return Err(MirrorVerifyError::HeaderAuthority);
        }
        Ok(Self {
            archive,
            freshness,
            observation: Some(observation),
        })
    }

    #[must_use]
    pub fn observation(&self) -> Option<&MirrorObservation> {
        self.observation.as_ref()
    }

    /// Returns the committed block coordinate retained in a v4 archive.
    ///
    /// # Errors
    /// Refuses batch-era archives without a committed coordinate.
    pub fn committed(&self) -> Result<CommittedCoordinate, MirrorVerifyError> {
        let committed = &self.archive.committed;
        Ok(CommittedCoordinate {
            height: self.archive.batch_number,
            block_hash: committed.block_hash,
        })
    }

    /// Verifies an archived receipt under independently configured receipt key windows.
    ///
    /// # Errors
    /// Refuses absent committed evidence or receipt indices and preserves the failing receipt check.
    pub fn committed_receipt(
        &self,
        index: usize,
        keys: &[SequencerAuthorization],
    ) -> Result<MirrorVerification<Receipt>, MirrorVerifyError> {
        let committed = &self.archive.committed;
        let canonical = committed
            .receipts
            .get(index)
            .ok_or(MirrorVerifyError::ReceiptMissing)?;
        let value = verify_sequencer_signature(canonical, ReceiptKeys::HeightRanges(keys))
            .map_err(|failure| MirrorVerifyError::Receipt(failure.check))?;
        Ok(self.report(value, MirrorEvidenceLevel::Committed))
    }

    fn report<T>(&self, value: T, level: MirrorEvidenceLevel) -> MirrorVerification<T> {
        MirrorVerification {
            value,
            level,
            batch_number: self.archive.batch_number,
            freshness: self.freshness,
            observation: self.observation.clone(),
        }
    }
}

#[cfg(test)]
mod tests {
    use super::{
        MirrorEvidenceLevel, MirrorVerificationFreshness, MirrorVerifier, MirrorVerifyError,
        ReceiptCheck,
    };
    use crate::{ArchiveData, ArchiveError};

    const ARCHIVE: &[u8] =
        include_bytes!("../../../contracts/solana-mirror/tests/fixtures/native.archive");
    fn fixture_bytes(value: &serde_json::Value) -> Vec<u8> {
        let digits = value
            .as_str()
            .and_then(|text| text.strip_prefix("0x"))
            .unwrap_or_else(|| panic!("fixture hex string"));
        (0..digits.len())
            .step_by(2)
            .map(|index| {
                u8::from_str_radix(&digits[index..index + 2], 16)
                    .unwrap_or_else(|error| panic!("fixture hex: {error}"))
            })
            .collect()
    }

    fn committed_archive() -> (
        crate::Archive,
        Vec<layerx_proof::inclusion::SequencerAuthorization>,
    ) {
        let fixture: serde_json::Value =
            serde_json::from_str(include_str!("../tests/fixtures/committed_blocks.json"))
                .unwrap_or_else(|error| panic!("recorded committed block: {error}"));
        let block = crate::committed::CommittedBlock {
            header: crate::committed::CommittedHeader::from_block_json(&fixture["committedBlock"])
                .unwrap_or_else(|error| panic!("committed header: {error:?}")),
            receipts: fixture["receipts"]
                .as_array()
                .unwrap_or_else(|| panic!("recorded receipts"))
                .iter()
                .map(|receipt| fixture_bytes(&receipt["receipt"]))
                .collect(),
        };
        let key = &fixture["receiptKey"];
        let keys = vec![layerx_proof::inclusion::SequencerAuthorization::new(
            fixture_bytes(&key["keyId"])
                .try_into()
                .unwrap_or_else(|_| panic!("key id length")),
            fixture_bytes(&key["publicKey"])
                .try_into()
                .unwrap_or_else(|_| panic!("public key length")),
            key["firstHeight"]
                .as_u64()
                .unwrap_or_else(|| panic!("first height")),
            key["lastHeight"]
                .as_u64()
                .unwrap_or_else(|| panic!("last height")),
        )];
        let archive = crate::Archive::from_committed(
            &block,
            125,
            crate::NodeHead {
                latest_sealed_batch: block.header.height,
                latest_committed: None,
            },
        )
        .unwrap_or_else(|error| panic!("v4 archive: {error:?}"));
        (archive, keys)
    }

    fn observation(archive: &crate::Archive) -> crate::source::MirrorObservation {
        use crate::source::{
            MirrorCanonicalPosition, MirrorLag, MirrorObservation, MirrorProvenance,
            MirrorSourceFreshness, MirrorSourceId, MirrorTargetIdentity,
        };
        MirrorObservation {
            source: MirrorSourceId::new("recorded-archive")
                .unwrap_or_else(|error| panic!("source id: {error:?}")),
            target: MirrorTargetIdentity::Ethereum {
                chain_id: 125,
                genesis_hash: [1; 32],
                contract: [2; 20],
                code_hash: [3; 32],
                publisher: [4; 20],
            },
            commitment: archive.commitment(),
            batch_number: archive.data().batch_number,
            committed: archive.data().committed_coordinate(),
            position: MirrorCanonicalPosition::Ethereum {
                block_number: 50,
                block_hash: [5; 32],
                reference_head_number: 52,
                reference_head_hash: [6; 32],
            },
            provenance: MirrorProvenance::Canonical,
            freshness: MirrorSourceFreshness {
                latest_batch: Some(archive.data().batch_number),
                latest_committed: archive.data().committed_coordinate(),
                batch_lag: MirrorLag::Known(0),
                committed_lag: MirrorLag::Unknown,
            },
            failover_count: 0,
            agreeing_sources: 1,
        }
    }

    #[test]
    fn a_v4_archive_receipt_verifies_under_the_configured_keys() -> Result<(), MirrorVerifyError> {
        let (archive, keys) = committed_archive();
        let observed = observation(&archive);
        let verifier = MirrorVerifier::admit_committed(archive.bytes(), observed.clone())?;
        let receipts = &archive.data().committed.receipts;
        for (index, canonical) in receipts.iter().enumerate() {
            let result = verifier.committed_receipt(index, &keys)?;
            assert_eq!(
                result.value(),
                &layerx_wire::receipt::decode(canonical)
                    .map_err(|_| MirrorVerifyError::ReceiptDecode)?
            );
            assert_eq!(result.level(), MirrorEvidenceLevel::Committed);
            assert_eq!(result.batch_number(), 1000);
            assert_eq!(result.observation(), Some(&observed));
            assert_eq!(
                result.freshness(),
                MirrorVerificationFreshness::from_source(observed.freshness)
            );
        }
        assert_eq!(
            verifier.committed_receipt(receipts.len(), &keys),
            Err(MirrorVerifyError::ReceiptMissing)
        );
        let mut wrong_commitment = observed.clone();
        wrong_commitment.commitment = crate::ArchiveCommitment::from_bytes([0; 32]);
        assert!(matches!(
            MirrorVerifier::admit_committed(archive.bytes(), wrong_commitment),
            Err(MirrorVerifyError::SourceCommitment)
        ));
        let mut wrong_height = observed.clone();
        wrong_height.batch_number += 1;
        assert!(matches!(
            MirrorVerifier::admit_committed(archive.bytes(), wrong_height),
            Err(MirrorVerifyError::SourceBatch)
        ));
        let mut stale_head = observed;
        stale_head.freshness.latest_batch = Some(999);
        assert!(matches!(
            MirrorVerifier::admit_committed(archive.bytes(), stale_head),
            Err(MirrorVerifyError::HeaderAuthority)
        ));
        Ok(())
    }

    #[test]
    fn a_v4_receipt_signed_outside_the_key_windows_is_refused() -> Result<(), MirrorVerifyError> {
        use layerx_proof::inclusion::SequencerAuthorization;
        use layerx_proof::receipt::ReceiptCheck;
        let (archive, keys) = committed_archive();
        let verifier = MirrorVerifier::admit_committed(archive.bytes(), observation(&archive))?;
        let key = &keys[0];
        let outside = [SequencerAuthorization::new(
            key.sequencer_id(),
            key.public_key(),
            1,
            999,
        )];
        assert_eq!(
            verifier.committed_receipt(0, &outside),
            Err(MirrorVerifyError::Receipt(ReceiptCheck::SequencerSignature))
        );
        let after = [SequencerAuthorization::new(
            key.sequencer_id(),
            key.public_key(),
            1001,
            1999,
        )];
        assert_eq!(
            verifier.committed_receipt(0, &after),
            Err(MirrorVerifyError::Receipt(ReceiptCheck::SequencerSignature))
        );
        let exact = [SequencerAuthorization::new(
            key.sequencer_id(),
            key.public_key(),
            1000,
            1000,
        )];
        assert_eq!(
            verifier.committed_receipt(0, &exact)?.level(),
            MirrorEvidenceLevel::Committed
        );
        Ok(())
    }

    #[test]
    fn the_committed_coordinate_is_read_from_a_v4_archive() -> Result<(), MirrorVerifyError> {
        let (archive, _) = committed_archive();
        let verifier = MirrorVerifier::admit_committed(archive.bytes(), observation(&archive))?;
        assert_eq!(
            verifier.committed()?,
            crate::committed::CommittedCoordinate {
                height: 1000,
                block_hash: [0x11; 32],
            }
        );
        assert_eq!(ArchiveData::decode(ARCHIVE), Err(ArchiveError::Version));
        assert!(matches!(
            MirrorVerifier::admit_committed(ARCHIVE, observation(&archive)),
            Err(MirrorVerifyError::SourceCommitment)
        ));
        let mut legacy_observation = observation(&archive);
        legacy_observation.commitment = crate::archive_commitment(ARCHIVE);
        assert!(matches!(
            MirrorVerifier::admit_committed(ARCHIVE, legacy_observation),
            Err(MirrorVerifyError::Archive(ArchiveError::Version))
        ));
        Ok(())
    }

    #[test]
    fn native_fixture_receipt_verifies_and_altered_bytes_are_refused(
    ) -> Result<(), MirrorVerifyError> {
        let (archive, keys) = committed_archive();
        let observed = observation(&archive);
        let verifier = MirrorVerifier::admit_committed(archive.bytes(), observed.clone())?;
        let verified = verifier.committed_receipt(0, &keys)?;
        assert_eq!(verified.level(), MirrorEvidenceLevel::Committed);
        assert_eq!(verified.batch_number(), archive.data().batch_number);
        let unsigned = layerx_wire::receipt::encode_unsigned(verified.value())
            .map_err(|_| MirrorVerifyError::ReceiptDecode)?;
        assert!(layerx_wire::hash::receipt_digest(&unsigned).is_ok());

        let mut changed = archive.data().clone();
        let canonical = &mut changed.committed.receipts[0];
        let last = canonical
            .last_mut()
            .ok_or(MirrorVerifyError::ReceiptMissing)?;
        *last ^= 1;
        let altered = crate::Archive::from_committed(
            &crate::committed::CommittedBlock {
                header: crate::committed::CommittedHeader {
                    height: changed.batch_number,
                    block_hash: changed.committed.block_hash,
                    parent_hash: changed.committed.parent_hash,
                    app_hash: changed.committed.app_hash,
                    timestamp: changed.committed.timestamp,
                    transactions: Vec::new(),
                },
                receipts: changed.committed.receipts,
            },
            changed.network_id,
            archive.node_head(),
        )
        .map_err(|_| MirrorVerifyError::ReceiptDecode)?;
        let altered_verifier =
            MirrorVerifier::admit_committed(altered.bytes(), observation(&altered))?;
        assert_eq!(
            altered_verifier.committed_receipt(0, &keys),
            Err(MirrorVerifyError::Receipt(ReceiptCheck::SequencerSignature))
        );
        assert!(matches!(
            MirrorVerifier::admit_committed(altered.bytes(), observed),
            Err(MirrorVerifyError::SourceCommitment)
        ));
        let key = &keys[0];
        let mut public_key = key.public_key();
        public_key[0] ^= 1;
        let wrong_keys = [layerx_proof::inclusion::SequencerAuthorization::new(
            key.sequencer_id(),
            public_key,
            key.first_batch_number(),
            key.last_batch_number(),
        )];
        assert!(matches!(
            verifier.committed_receipt(0, &wrong_keys),
            Err(MirrorVerifyError::Receipt(ReceiptCheck::SequencerSignature))
        ));
        Ok(())
    }
}
