//! Historical guarantor certificate material without chain-finality authority.
//! The compatibility types remain available to daemon consumers; both verifier
//! entry points refuse certificates with `RetiredCertificateVerifier`.

use std::convert::Infallible;

use layerx_types::settlement::SettlementError;
use layerx_wire::hash::checkpoint_id as hash_checkpoint_id;
use layerx_wire::receipt::{decode_batch_header, encode_batch_header};

const ATTESTATION_BYTES: usize = 189;

/// Canonical checkpoint body committed by guarantors.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct Checkpoint {
    header_bytes: Vec<u8>,
    validity_proof: Vec<u8>,
}

impl Checkpoint {
    /// Creates a checkpoint body from exact core-produced bytes.
    #[must_use]
    pub fn new(header_bytes: Vec<u8>, validity_proof: Vec<u8>) -> Self {
        Self {
            header_bytes,
            validity_proof,
        }
    }

    /// Borrows the exact core-produced checkpoint header bytes.
    #[must_use]
    pub fn header_bytes(&self) -> &[u8] {
        &self.header_bytes
    }

    /// Borrows the exact core-produced validity proof committed by the
    /// checkpoint identifier.
    #[must_use]
    pub fn validity_proof(&self) -> &[u8] {
        &self.validity_proof
    }
}

/// One exact replay-and-possession attestation.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct Attestation {
    protocol_version: u16,
    network_id: u32,
    paxeer_chain_id: u64,
    paxeer_settlement_contract: [u8; 20],
    epoch: u64,
    checkpoint_id: [u8; 32],
    checkpoint_hash: [u8; 32],
    guarantor_id: [u8; 32],
    batch_number: u64,
    data_availability_root: [u8; 32],
    replayed: bool,
    data_possessed: bool,
    availability_class_mask: u8,
    attested_at_ms: u64,
    signer: [u8; 20],
    signature: [u8; 64],
    signature_v: u8,
}

impl Attestation {
    /// Creates one attestation decoded from core certificate bytes.
    #[allow(clippy::too_many_arguments)]
    #[must_use]
    pub const fn new(
        protocol_version: u16,
        network_id: u32,
        paxeer_chain_id: u64,
        paxeer_settlement_contract: [u8; 20],
        epoch: u64,
        checkpoint_id: [u8; 32],
        checkpoint_hash: [u8; 32],
        guarantor_id: [u8; 32],
        batch_number: u64,
        data_availability_root: [u8; 32],
        replayed: bool,
        data_possessed: bool,
        availability_class_mask: u8,
        attested_at_ms: u64,
        signer: [u8; 20],
        signature: [u8; 64],
        signature_v: u8,
    ) -> Self {
        Self {
            protocol_version,
            network_id,
            paxeer_chain_id,
            paxeer_settlement_contract,
            epoch,
            checkpoint_id,
            checkpoint_hash,
            guarantor_id,
            batch_number,
            data_availability_root,
            replayed,
            data_possessed,
            availability_class_mask,
            attested_at_ms,
            signer,
            signature,
            signature_v,
        }
    }

    #[must_use]
    pub const fn guarantor_id(&self) -> [u8; 32] {
        self.guarantor_id
    }

    #[must_use]
    pub const fn signature(&self) -> [u8; 64] {
        self.signature
    }

    #[must_use]
    pub const fn signer(&self) -> [u8; 20] {
        self.signer
    }

    #[must_use]
    pub const fn signature_v(&self) -> u8 {
        self.signature_v
    }

    #[must_use]
    pub const fn attested_at_ms(&self) -> u64 {
        self.attested_at_ms
    }

    /// Returns every field of the exact signed canonical attestation statement.
    #[must_use]
    pub fn canonical_statement(&self) -> [u8; ATTESTATION_BYTES] {
        self.message()
    }

    fn message(&self) -> [u8; ATTESTATION_BYTES] {
        let mut message = [0_u8; ATTESTATION_BYTES];
        message[..2].copy_from_slice(&self.protocol_version.to_be_bytes());
        message[2..6].copy_from_slice(&self.network_id.to_be_bytes());
        message[6..14].copy_from_slice(&self.paxeer_chain_id.to_be_bytes());
        message[14..34].copy_from_slice(&self.paxeer_settlement_contract);
        message[34..42].copy_from_slice(&self.epoch.to_be_bytes());
        message[42..74].copy_from_slice(&self.checkpoint_id);
        message[74..106].copy_from_slice(&self.checkpoint_hash);
        message[106..138].copy_from_slice(&self.guarantor_id);
        message[138..146].copy_from_slice(&self.batch_number.to_be_bytes());
        message[146..178].copy_from_slice(&self.data_availability_root);
        message[178] = u8::from(self.replayed);
        message[179] = u8::from(self.data_possessed);
        message[180] = self.availability_class_mask;
        message[181..189].copy_from_slice(&self.attested_at_ms.to_be_bytes());
        message
    }
}

/// One guarantor key record from the bonded set at the checkpoint.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct GuarantorKey {
    guarantor_id: [u8; 32],
    public_key: [u8; 33],
    bonded: bool,
}

/// Settlement domain owned by the certificate verifier.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct SettlementDomain {
    paxeer_chain_id: u64,
    settlement_contract: [u8; 20],
}

impl SettlementDomain {
    /// Creates the exact Paxeer chain and `GuarantorBond` contract expected by
    /// the verifier.
    #[must_use]
    pub const fn new(paxeer_chain_id: u64, settlement_contract: [u8; 20]) -> Self {
        Self {
            paxeer_chain_id,
            settlement_contract,
        }
    }

    #[must_use]
    pub const fn paxeer_chain_id(self) -> u64 {
        self.paxeer_chain_id
    }

    #[must_use]
    pub const fn settlement_contract(self) -> [u8; 20] {
        self.settlement_contract
    }
}

impl GuarantorKey {
    /// Creates one checkpoint-relative guarantor key record.
    #[must_use]
    pub const fn new(guarantor_id: [u8; 32], public_key: [u8; 33], bonded: bool) -> Self {
        Self {
            guarantor_id,
            public_key,
            bonded,
        }
    }

    #[must_use]
    pub const fn guarantor_id(&self) -> [u8; 32] {
        self.guarantor_id
    }

    #[must_use]
    pub const fn public_key(&self) -> [u8; 33] {
        self.public_key
    }

    #[must_use]
    pub const fn bonded(&self) -> bool {
        self.bonded
    }
}

/// A checkpoint certificate plus its optional registered settlement reference.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct Certificate {
    checkpoint: Checkpoint,
    attestations: Vec<Attestation>,
    threshold: usize,
    settlement_reference: Option<Vec<u8>>,
}

impl Certificate {
    /// Creates certificate material decoded from core-produced bytes.
    #[must_use]
    pub fn new(
        checkpoint: Checkpoint,
        attestations: Vec<Attestation>,
        threshold: usize,
        settlement_reference: Option<Vec<u8>>,
    ) -> Self {
        Self {
            checkpoint,
            attestations,
            threshold,
            settlement_reference,
        }
    }

    #[must_use]
    pub const fn checkpoint(&self) -> &Checkpoint {
        &self.checkpoint
    }

    #[must_use]
    pub fn attestations(&self) -> &[Attestation] {
        &self.attestations
    }

    #[must_use]
    pub const fn threshold(&self) -> usize {
        self.threshold
    }

    #[must_use]
    pub fn settlement_reference(&self) -> Option<&[u8]> {
        self.settlement_reference.as_deref()
    }
}

/// Uninhabited compatibility result for the retired guarantor verifiers.
///
/// The private impossible value prevents construction, including inside this
/// crate. The accessors preserve source compatibility for consumers awaiting
/// migration, but this type cannot carry proof evidence or establish a level.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ThresholdReport {
    /// Historical distinct-signature count; no report can be constructed.
    pub achieved: usize,
    /// Historical threshold; no report can be constructed.
    pub required: usize,
    retired: Infallible,
}

impl ThresholdReport {
    #[must_use]
    pub const fn level(&self) -> layerx_types::verify::VerificationLevel {
        match self.retired {}
    }

    /// Preserves legacy identifier access without returning an `Evidence`.
    #[must_use]
    pub const fn evidence(&self) -> &Self {
        match self.retired {}
    }

    #[must_use]
    pub const fn checkpoint_id(&self) -> Option<[u8; 32]> {
        match self.retired {}
    }

    #[must_use]
    pub const fn settlement_reference(&self) -> Option<&[u8]> {
        match self.retired {}
    }

    #[must_use]
    pub const fn protocol_version(&self) -> u16 {
        match self.retired {}
    }

    #[must_use]
    pub const fn network_id(&self) -> u32 {
        match self.retired {}
    }

    #[must_use]
    pub const fn batch_number(&self) -> u64 {
        match self.retired {}
    }

    #[must_use]
    pub const fn first_sequence(&self) -> u64 {
        match self.retired {}
    }

    #[must_use]
    pub const fn last_sequence(&self) -> u64 {
        match self.retired {}
    }

    #[must_use]
    pub const fn data_availability_root(&self) -> [u8; 32] {
        match self.retired {}
    }

    #[must_use]
    pub const fn record_roots(&self) -> crate::availability::RootCommitments {
        match self.retired {}
    }

    #[must_use]
    pub const fn resulting_state_root(&self) -> [u8; 32] {
        match self.retired {}
    }
}

/// Exact certificate verification failure.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum CheckpointError {
    /// Guarantor certificates cannot authenticate a committed BFT block.
    RetiredCertificateVerifier,
    /// Canonical header decoding or re-encoding failed.
    Header,
    /// The certificate refers to a different checkpoint identifier.
    CheckpointIdentifier,
    /// An attestation's committed checkpoint fields do not match its body.
    CheckpointFields,
    /// One guarantor appears more than once.
    DuplicateSigner([u8; 32]),
    /// One signer is absent from, or unbonded in, the supplied set.
    SignerMembership([u8; 32]),
    /// One included signature is invalid.
    Signature([u8; 32]),
    /// The achieved count is below the configured threshold.
    Threshold { achieved: usize, required: usize },
    /// The settlement reference is absent from or differs from registration.
    Settlement,
    /// One attestation precedes the checkpoint header timestamp.
    AttestationNotYetValid {
        guarantor_id: [u8; 32],
        attested_at_ms: u64,
        header_timestamp_ms: u64,
    },
    /// One attestation follows the header timestamp plus the declared delay.
    AttestationExpired {
        guarantor_id: [u8; 32],
        attested_at_ms: u64,
        deadline_ms: u64,
    },
    /// The declared settlement configuration failed validation.
    Configuration(SettlementError),
}

/// Recomputes the checkpoint identifier from its canonical body.
///
/// # Errors
///
/// Returns a header failure when canonical header validation or hashing fails.
pub fn checkpoint_id(checkpoint: &Checkpoint) -> Result<[u8; 32], CheckpointError> {
    let header =
        decode_batch_header(&checkpoint.header_bytes).map_err(|_| CheckpointError::Header)?;
    let encoded = encode_batch_header(&header).map_err(|_| CheckpointError::Header)?;
    if encoded != checkpoint.header_bytes {
        return Err(CheckpointError::Header);
    }
    hash_checkpoint_id(&encoded, &checkpoint.validity_proof).map_err(|_| CheckpointError::Header)
}

/// Refuses the retired guarantor verification interface.
///
/// # Errors
///
/// Always returns [`CheckpointError::RetiredCertificateVerifier`]. Guarantor
/// signatures and registration identifiers cannot establish chain commitment.
#[deprecated(note = "guarantor verification is retired; use receipt or inclusion verification")]
pub fn verify_certificate(
    _certificate: &Certificate,
    _bonded_set: &[GuarantorKey],
    _registered_checkpoint_id: &[u8; 32],
    _expected_settlement_domain: SettlementDomain,
    _registered_settlement_reference: Option<&[u8]>,
) -> Result<ThresholdReport, CheckpointError> {
    Err(CheckpointError::RetiredCertificateVerifier)
}

/// Refuses the retired declared-domain guarantor verification interface.
///
/// # Errors
///
/// Always returns [`CheckpointError::RetiredCertificateVerifier`], including
/// for declared domains with a valid guarantor threshold and registration.
#[deprecated(note = "guarantor verification is retired; use receipt or inclusion verification")]
pub fn verify_declared_certificate(
    _certificate: &Certificate,
    _domain_name: &str,
    _registered_checkpoint_id: &[u8; 32],
    _registered_settlement_reference: Option<&[u8]>,
) -> Result<ThresholdReport, CheckpointError> {
    Err(CheckpointError::RetiredCertificateVerifier)
}
