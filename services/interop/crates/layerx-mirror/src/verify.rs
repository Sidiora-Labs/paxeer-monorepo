//! Offline verification rooted only in a finalised mirror archive.

use layerx_crypto::ed25519;
use layerx_proof::inclusion::{verify_state, InclusionError, SequencerAuthorization};
use layerx_proof::merkle::{build_proof, verify_path, MerkleError, Proof};
use layerx_proof::receipt::{
    authorized_maintained_activity_batch_chain, verify_outcome, verify_sequencer_signature,
    AuthorizedBatch, MaintainedOutcomeEvidence, ReceiptCheck, ReceiptKeys, VerifiedReceipt,
};
use layerx_wire::hash::batch_header_digest;
use layerx_wire::receipt::{decode, decode_batch_header, Receipt};

use crate::committed::CommittedCoordinate;

use crate::source::{MirrorLag, MirrorObservation, MirrorSourceFreshness, ObservedArchive};
use crate::{ArchiveData, ArchiveError, CommittedFreshness, MirrorFreshness};

/// Authority configured independently of mirror payloads. The public key and
/// range are never accepted from an archive.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct SignedHeaderTrust {
    pub sequencer_id: [u8; 32],
    pub sequencer_public_key: [u8; 32],
    pub first_batch_number: u64,
    pub last_batch_number: u64,
}

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
    signed_header_digest: [u8; 32],
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

    /// Returns zero for committed evidence, which carries no signed batch header.
    #[must_use]
    pub const fn signed_header_digest(&self) -> [u8; 32] {
        self.signed_header_digest
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
    BatchIncluded,
    StateProven,
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
    ReceiptInclusion(MerkleError),
    State(InclusionError),
    SourceCommitment,
    SourceBatch,
    SourceReorged,
    CheckpointTrustUnavailable,
    CommittedUnavailable,
}

enum MirrorAuthority {
    SignedHeader(SignedHeaderTrust),
    Committed,
}

/// Offline verifier over untrusted archive bytes and separately configured authority.
pub struct MirrorVerifier {
    archive: ArchiveData,
    authority: MirrorAuthority,
    header_digest: [u8; 32],
    freshness: MirrorVerificationFreshness,
    observation: Option<MirrorObservation>,
}

impl MirrorVerifier {
    /// Admits one sealed source observation. Archive, chain identity,
    /// canonical position and freshness remain one indivisible source fact.
    ///
    /// # Errors
    /// Returns an error for a mismatched commitment, invalid archive, header authority or signature.
    pub fn from_source(
        observed: ObservedArchive,
        trust: SignedHeaderTrust,
    ) -> Result<Self, MirrorVerifyError> {
        let (archive_bytes, observation) = observed.into_parts();
        if crate::archive_commitment(&archive_bytes) != observation.commitment {
            return Err(MirrorVerifyError::SourceCommitment);
        }
        let freshness = MirrorVerificationFreshness::from_source(observation.freshness);
        Self::admit(&archive_bytes, trust, freshness, Some(observation))
    }

    /// Admits a committed archive without requiring a signed batch header.
    ///
    /// # Errors
    /// Refuses mismatched source commitments or heights and archives without committed evidence.
    pub fn from_committed_source(observed: ObservedArchive) -> Result<Self, MirrorVerifyError> {
        let (archive_bytes, observation) = observed.into_parts();
        Self::admit_committed(&archive_bytes, observation)
    }

    fn admit_committed(
        archive_bytes: &[u8],
        observation: MirrorObservation,
    ) -> Result<Self, MirrorVerifyError> {
        if crate::archive_commitment(archive_bytes) != observation.commitment {
            return Err(MirrorVerifyError::SourceCommitment);
        }
        let archive = ArchiveData::decode(archive_bytes).map_err(MirrorVerifyError::Archive)?;
        if archive.committed.is_none() {
            return Err(MirrorVerifyError::CommittedUnavailable);
        }
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
            authority: MirrorAuthority::Committed,
            header_digest: [0; 32],
            freshness,
            observation: Some(observation),
        })
    }

    /// Returns the committed block coordinate retained in a v4 archive.
    ///
    /// # Errors
    /// Refuses batch-era archives without a committed coordinate.
    pub fn committed(&self) -> Result<CommittedCoordinate, MirrorVerifyError> {
        let committed = self
            .archive
            .committed
            .as_ref()
            .ok_or(MirrorVerifyError::CommittedUnavailable)?;
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
        let committed = self
            .archive
            .committed
            .as_ref()
            .ok_or(MirrorVerifyError::CommittedUnavailable)?;
        let canonical = committed
            .receipts
            .get(index)
            .ok_or(MirrorVerifyError::ReceiptMissing)?;
        let value = verify_sequencer_signature(canonical, ReceiptKeys::HeightRanges(keys))
            .map_err(|failure| MirrorVerifyError::Receipt(failure.check))?;
        Ok(self.report(value, MirrorEvidenceLevel::Committed))
    }

    fn signed_header_trust(&self) -> Result<SignedHeaderTrust, MirrorVerifyError> {
        match self.authority {
            MirrorAuthority::SignedHeader(trust) => Ok(trust),
            MirrorAuthority::Committed => Err(MirrorVerifyError::HeaderAuthority),
        }
    }

    fn admit(
        archive_bytes: &[u8],
        trust: SignedHeaderTrust,
        freshness: MirrorVerificationFreshness,
        observation: Option<MirrorObservation>,
    ) -> Result<Self, MirrorVerifyError> {
        let archive = ArchiveData::decode(archive_bytes).map_err(MirrorVerifyError::Archive)?;
        let header = decode_batch_header(&archive.canonical_batch_header)
            .map_err(|_| MirrorVerifyError::Header)?;
        if header.sequencer_id() != trust.sequencer_id
            || header.batch_number() < trust.first_batch_number
            || header.batch_number() > trust.last_batch_number
            || archive.batch_authorization.sequencer_id != trust.sequencer_id
            || archive.batch_authorization.sequencer_public_key != trust.sequencer_public_key
            || archive.batch_authorization.first_batch_number != trust.first_batch_number
            || archive.batch_authorization.last_batch_number != trust.last_batch_number
        {
            return Err(MirrorVerifyError::HeaderAuthority);
        }
        let header_digest = batch_header_digest(&archive.canonical_batch_header)
            .map_err(|_| MirrorVerifyError::Header)?;
        ed25519::verify_digest(
            &trust.sequencer_public_key,
            &archive.batch_authorization.header_signature,
            &header_digest,
        )
        .map_err(|_| MirrorVerifyError::HeaderSignature)?;
        if freshness
            .latest_batch_mirrored
            .is_some_and(|latest| latest < archive.batch_number)
        {
            return Err(MirrorVerifyError::HeaderAuthority);
        }
        if observation
            .as_ref()
            .is_some_and(|value| value.batch_number != archive.batch_number)
        {
            return Err(MirrorVerifyError::SourceBatch);
        }
        Ok(Self {
            archive,
            authority: MirrorAuthority::SignedHeader(trust),
            header_digest,
            freshness,
            observation,
        })
    }

    /// Verifies a receipt retained by the archive against both its sequencer
    /// signature and the receipt root in the independently signed header.
    ///
    /// # Errors
    ///
    /// Refuses absent or non-included receipts and any canonical, invariant,
    /// or sequencer-signature failure.
    pub fn receipt(
        &self,
        canonical_receipt: &[u8],
    ) -> Result<MirrorVerification<VerifiedReceipt>, MirrorVerifyError> {
        let index = self
            .archive
            .records
            .receipts
            .iter()
            .position(|record| record == canonical_receipt)
            .ok_or(MirrorVerifyError::ReceiptMissing)?;
        let leaves = self
            .archive
            .records
            .receipts
            .iter()
            .map(Vec::as_slice)
            .collect::<Vec<_>>();
        let (proof, root) =
            build_proof(&leaves, index).map_err(MirrorVerifyError::ReceiptInclusion)?;
        if root != self.archive.record_roots.receipt {
            return Err(MirrorVerifyError::ReceiptInclusion(
                MerkleError::RootMismatch,
            ));
        }
        verify_path(canonical_receipt, &proof, &root)
            .map_err(MirrorVerifyError::ReceiptInclusion)?;
        let header = decode_batch_header(&self.archive.canonical_batch_header)
            .map_err(|_| MirrorVerifyError::Header)?;
        let authorised = self.authorize_receipt(canonical_receipt, &proof, &header)?;
        let value = verify_outcome(canonical_receipt, &authorised)
            .map_err(|failure| MirrorVerifyError::Receipt(failure.check))?;
        Ok(self.report(value, MirrorEvidenceLevel::BatchIncluded))
    }

    fn authorize_receipt(
        &self,
        canonical: &[u8],
        proof: &Proof,
        header: &layerx_wire::receipt::BatchHeader,
    ) -> Result<AuthorizedBatch, MirrorVerifyError> {
        let trust = self.signed_header_trust()?;
        let decoded = decode(canonical).map_err(|_| MirrorVerifyError::ReceiptDecode)?;
        let receipt = decoded.protocol().ok_or(MirrorVerifyError::ReceiptDecode)?;
        let last = self
            .archive
            .records
            .receipts
            .last()
            .ok_or(MirrorVerifyError::ReceiptMissing)?;
        let maintenance = layerx_wire::maintenance::decode_occupancy_maintenance(last).ok();
        if maintenance.is_none() {
            decode(last).map_err(|_| MirrorVerifyError::ReceiptDecode)?;
        }
        let batch_id = layerx_wire::hash::receipt_execution_batch_id_for_evidence(
            receipt,
            header,
            maintenance.as_ref(),
        )
        .map_err(|_| MirrorVerifyError::ReceiptBatchMismatch)?;
        if batch_id != receipt.batch_id() {
            return Err(MirrorVerifyError::ReceiptBatchMismatch);
        }
        let authorised = AuthorizedBatch::new(
            batch_id,
            receipt.asset(),
            header.previous_state_root(),
            header.resulting_state_root(),
            trust.sequencer_public_key,
        );
        if maintenance.is_some() {
            let leaves = self
                .archive
                .records
                .receipts
                .iter()
                .map(Vec::as_slice)
                .collect::<Vec<_>>();
            let (maintenance_proof, _) = build_proof(&leaves, leaves.len() - 1)
                .map_err(MirrorVerifyError::ReceiptInclusion)?;
            let authorization = SequencerAuthorization::new(
                trust.sequencer_id,
                trust.sequencer_public_key,
                trust.first_batch_number,
                trust.last_batch_number,
            );
            return authorized_maintained_activity_batch_chain(
                canonical,
                &authorised,
                &MaintainedOutcomeEvidence {
                    header: &self.archive.canonical_batch_header,
                    header_signature: &self.archive.batch_authorization.header_signature,
                    activity_proof: proof,
                    maintenance: last,
                    maintenance_proof: &maintenance_proof,
                    authorization: &authorization,
                },
                &self.archive.records.receipts[..leaves.len() - 1],
            )
            .map_err(|_| MirrorVerifyError::ReceiptBatchMismatch);
        }
        if receipt.previous_state_root() != header.previous_state_root()
            || receipt.resulting_state_root() != header.resulting_state_root()
            || receipt.global_sequence() < header.first_sequence()
            || receipt.global_sequence() > header.last_sequence()
        {
            return Err(MirrorVerifyError::ReceiptBatchMismatch);
        }
        Ok(authorised)
    }

    /// Verifies caller-supplied state inclusion against this archive's signed
    /// header. State bytes and proof remain untrusted inputs.
    ///
    /// # Errors
    ///
    /// Refuses invalid state proofs and any signed-header authority mismatch.
    pub fn state(
        &self,
        canonical_state: &[u8],
        proof: &Proof,
    ) -> Result<MirrorVerification<layerx_proof::inclusion::InclusionEvidence>, MirrorVerifyError>
    {
        let header = decode_batch_header(&self.archive.canonical_batch_header)
            .map_err(|_| MirrorVerifyError::Header)?;
        let trust = self.signed_header_trust()?;
        let authority = SequencerAuthorization::new(
            trust.sequencer_id,
            trust.sequencer_public_key,
            trust.first_batch_number,
            trust.last_batch_number,
        );
        let value = verify_state(
            canonical_state,
            proof,
            &header.resulting_state_root(),
            &self.archive.canonical_batch_header,
            &self.archive.batch_authorization.header_signature,
            &authority,
        )
        .map_err(MirrorVerifyError::State)?;
        Ok(self.report(value, MirrorEvidenceLevel::StateProven))
    }

    fn report<T>(&self, value: T, level: MirrorEvidenceLevel) -> MirrorVerification<T> {
        MirrorVerification {
            value,
            level,
            batch_number: self.archive.batch_number,
            signed_header_digest: self.header_digest,
            freshness: self.freshness,
            observation: self.observation.clone(),
        }
    }

    /// The v2 archive carries canonical checkpoint bytes but omits the
    /// attestation replay/possession timestamps needed by the protocol-owned
    /// checkpoint verifier. It must never be promoted to checkpoint level.
    ///
    /// # Errors
    /// Returns `MirrorVerifyError::CheckpointTrustUnavailable` because the archive does not retain independent checkpoint authority.
    pub const fn checkpoint_level() -> Result<(), MirrorVerifyError> {
        Err(MirrorVerifyError::CheckpointTrustUnavailable)
    }
}

#[cfg(test)]
mod tests {
    use super::{
        MirrorEvidenceLevel, MirrorVerificationFreshness, MirrorVerifier, MirrorVerifyError,
        SignedHeaderTrust,
    };
    use crate::{ArchiveData, ArchiveError};

    const ARCHIVE: &[u8] =
        include_bytes!("../../../contracts/solana-mirror/tests/fixtures/native.archive");
    const SEQUENCER_ID: &[u8; 32] =
        include_bytes!("../tests/fixtures/native-publisher/sequencer.id");
    const SEQUENCER_KEY: &[u8; 32] =
        include_bytes!("../tests/fixtures/native-publisher/sequencer.public");

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

    fn batch_era_archive() -> crate::Archive {
        use layerx_proof::availability::{reassemble, verify_chunk, Chunk};
        use layerx_proof::merkle::build_leaf_hash_proof;

        let data = ArchiveData::decode(ARCHIVE)
            .unwrap_or_else(|error| panic!("native archive: {error:?}"));
        let authorization = data.batch_authorization;
        let trust = SignedHeaderTrust {
            sequencer_id: *SEQUENCER_ID,
            sequencer_public_key: *SEQUENCER_KEY,
            first_batch_number: authorization.first_batch_number,
            last_batch_number: authorization.last_batch_number,
        };
        let batch =
            crate::NodeBatch::verify(data.canonical_batch_header.clone(), authorization, &trust)
                .unwrap_or_else(|error| panic!("native batch: {error:?}"));
        let hashes = data
            .chunks
            .iter()
            .map(|chunk| chunk.claimed_hash)
            .collect::<Vec<_>>();
        let verified = data
            .chunks
            .iter()
            .enumerate()
            .map(|(index, chunk)| {
                let (proof, root) = build_leaf_hash_proof(&hashes, index)
                    .unwrap_or_else(|error| panic!("chunk proof: {error:?}"));
                assert_eq!(root, data.data_availability_root);
                verify_chunk(
                    Chunk {
                        batch_number: data.batch_number,
                        index: chunk.index,
                        class: chunk.class,
                        class_offset: chunk.class_offset,
                        claimed_hash: chunk.claimed_hash,
                        bytes: chunk.bytes.clone(),
                    },
                    &proof,
                    data.batch_number,
                    &root,
                )
                .unwrap_or_else(|error| panic!("chunk verification: {error:?}"))
            })
            .collect::<Vec<_>>();
        let (records, _) = reassemble(&verified, data.record_roots)
            .unwrap_or_else(|error| panic!("native records: {error:?}"));
        records
            .verify(&verified, data.record_roots)
            .unwrap_or_else(|error| panic!("availability: {error:?}"));
        crate::Archive::from_node(
            &batch,
            &verified,
            None,
            crate::NodeHead {
                latest_sealed_batch: data.batch_number,
                latest_committed: None,
            },
        )
        .unwrap_or_else(|error| panic!("batch-era archive: {error:?}"))
    }

    #[test]
    fn a_v4_archive_receipt_verifies_under_the_configured_keys() -> Result<(), MirrorVerifyError> {
        let (archive, keys) = committed_archive();
        let observed = observation(&archive);
        let verifier = MirrorVerifier::admit_committed(archive.bytes(), observed.clone())?;
        assert!(archive.data().canonical_batch_header.is_empty());
        let receipts = &archive
            .data()
            .committed
            .as_ref()
            .ok_or(MirrorVerifyError::CommittedUnavailable)?
            .receipts;
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
        assert!(matches!(
            verifier.signed_header_trust(),
            Err(MirrorVerifyError::HeaderAuthority)
        ));
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
        let legacy = batch_era_archive();
        assert_eq!(&legacy.bytes()[8..10], &2_u16.to_be_bytes());
        assert!(matches!(
            MirrorVerifier::admit_committed(legacy.bytes(), observation(&legacy)),
            Err(MirrorVerifyError::CommittedUnavailable)
        ));
        let data = legacy.data();
        let trust = SignedHeaderTrust {
            sequencer_id: *SEQUENCER_ID,
            sequencer_public_key: *SEQUENCER_KEY,
            first_batch_number: data.batch_authorization.first_batch_number,
            last_batch_number: data.batch_authorization.last_batch_number,
        };
        let verifier = MirrorVerifier::admit(
            legacy.bytes(),
            trust,
            MirrorVerificationFreshness::offline(data.batch_number, None),
            None,
        )?;
        assert_eq!(
            verifier.committed(),
            Err(MirrorVerifyError::CommittedUnavailable)
        );
        assert_eq!(
            verifier.committed_receipt(0, &[]),
            Err(MirrorVerifyError::CommittedUnavailable)
        );
        Ok(())
    }

    #[test]
    fn native_fixture_receipt_verifies_and_altered_bytes_are_refused(
    ) -> Result<(), MirrorVerifyError> {
        let archive = ArchiveData::decode(ARCHIVE).map_err(MirrorVerifyError::Archive)?;
        let trust = SignedHeaderTrust {
            sequencer_id: *SEQUENCER_ID,
            sequencer_public_key: *SEQUENCER_KEY,
            first_batch_number: 1,
            last_batch_number: u64::MAX,
        };
        let freshness = MirrorVerificationFreshness::offline(
            archive.batch_number,
            archive.committed_coordinate(),
        );
        let verifier = MirrorVerifier::admit(ARCHIVE, trust, freshness, None)?;
        let canonical = archive
            .records
            .receipts
            .first()
            .ok_or(MirrorVerifyError::ReceiptMissing)?;
        let verified = verifier.receipt(canonical)?;
        assert_eq!(verified.level(), MirrorEvidenceLevel::BatchIncluded);
        assert_eq!(verified.batch_number(), archive.batch_number);
        assert!(verified.value().evidence().receipt_digest().is_some());
        assert!(matches!(
            MirrorVerifier::checkpoint_level(),
            Err(MirrorVerifyError::CheckpointTrustUnavailable)
        ));

        let mut changed_receipt = canonical.clone();
        let last = changed_receipt
            .last_mut()
            .ok_or(MirrorVerifyError::ReceiptMissing)?;
        *last ^= 1;
        assert!(matches!(
            verifier.receipt(&changed_receipt),
            Err(MirrorVerifyError::ReceiptMissing)
        ));

        let mut changed_archive = ARCHIVE.to_vec();
        let signature_offset = 24 + 4 + archive.canonical_batch_header.len() + 32 + 32 + 8 + 8;
        changed_archive[signature_offset] ^= 1;
        assert!(matches!(
            MirrorVerifier::admit(&changed_archive, trust, freshness, None),
            Err(MirrorVerifyError::Archive(ArchiveError::BatchAuthorization))
        ));

        let mut changed_trust = trust;
        changed_trust.sequencer_public_key[0] ^= 1;
        assert!(matches!(
            MirrorVerifier::admit(ARCHIVE, changed_trust, freshness, None),
            Err(MirrorVerifyError::HeaderAuthority)
        ));
        Ok(())
    }
}
