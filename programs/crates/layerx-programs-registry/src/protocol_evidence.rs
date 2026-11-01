//! Cryptographic deployment and current-lifecycle evidence for Programs.

use core::fmt::{self, Display};

use layerx_programs_runtime::abi_policy::admit_abi_version;
use layerx_programs_runtime::{ProgramId, UpgradePolicy};

use crate::chain_proof::ChainProofVerifier;
use crate::{
    DeploymentRecord, InterfaceRefusal, ProgramLifecycle, ReadFreshness, VerifiedInterfaceRead,
};

pub(crate) const PROGRAMS_MODULE_ID: u16 = 9;
const PROGRAM_RECORD_BYTES: usize = 71;
pub(crate) const PROGRAM_KEY_PREFIX: &[u8] = b"program\0";
pub(crate) const STATUS_KEY_PREFIX: &[u8] = b"wind-down\0s";
const MAX_MODULE_BYTES: usize = 32 * 1024 * 1024;
const MAX_EVIDENCE_BYTES: usize = 40 * 1024 * 1024;

/// Exact cryptographic refusal returned before deployment state is trusted.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum ProtocolEvidenceError {
    ChainProof,
    Expired,
    InvalidTrustAnchor,
    TrustHistoryUnavailable,
    TrustAnchorUnavailable,
    TrustAnchorAmbiguous,
    HistoricalTrustAnchor,
    ProtocolDomain,
    SequencerRevoked,
    CanonicalActivity,
    UnsupportedActivity,
    PayloadHash,
    ActivityInclusion,
    Receipt,
    ReceiptInclusion,
    BatchMismatch,
    BatchIdentifier,
    ActivityReceiptMismatch,
    StateRoot,
    StateProof,
    ProgramRecord,
    LifecycleProof,
    DeploymentMismatch,
    Stale,
    Encoding,
}

impl Display for ProtocolEvidenceError {
    fn fmt(&self, formatter: &mut fmt::Formatter<'_>) -> fmt::Result {
        formatter.write_str(match self {
            Self::ChainProof => "chain commitment proof is invalid",
            Self::Expired => "chain trust has expired",
            Self::InvalidTrustAnchor => "deployment verifier trust anchor is invalid",
            Self::TrustHistoryUnavailable => {
                "canonical protected sequencer trust history is unavailable"
            }
            Self::TrustAnchorUnavailable => "signed batch has no configured sequencer trust anchor",
            Self::TrustAnchorAmbiguous => {
                "signed batch matches more than one sequencer trust anchor"
            }
            Self::HistoricalTrustAnchor => {
                "historical sequencer trust cannot authorize a current state head"
            }
            Self::ProtocolDomain => "evidence belongs to a different network, protocol, or epoch",
            Self::SequencerRevoked => "evidence was signed at or after sequencer-key revocation",
            Self::CanonicalActivity => "Programs activity is not canonical",
            Self::UnsupportedActivity => "activity is not a Programs deploy or upgrade",
            Self::PayloadHash => "Programs activity payload hash does not match",
            Self::ActivityInclusion => "Programs activity is not included by the signed batch",
            Self::Receipt => "Programs receipt is not a successful sequencer-signed receipt",
            Self::ReceiptInclusion => "Programs receipt is not included by the signed batch",
            Self::BatchMismatch => "activity, receipt, and state do not share one signed batch",
            Self::BatchIdentifier => {
                "receipt batch identifier does not match the core execution context"
            }
            Self::ActivityReceiptMismatch => "Programs receipt names a different activity",
            Self::StateRoot => "Programs root is not committed by the receipt state root",
            Self::StateProof => "Programs state membership proof is invalid",
            Self::ProgramRecord => "Programs registry record is non-canonical",
            Self::LifecycleProof => "Programs lifecycle proof is invalid",
            Self::DeploymentMismatch => "activity and resulting Programs record disagree",
            Self::Stale => "Programs evidence is outside the freshness bound",
            Self::Encoding => "deployment evidence encoding is non-canonical",
        })
    }
}

impl std::error::Error for ProtocolEvidenceError {}

/// Head kind carried by a Programs state proof.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum ProgramHeadKind {
    /// Final ordinary protocol receipt of a pre-maintenance signed batch.
    Ordinary,
    /// Native maintenance receipt that closes a signed batch.
    Maintenance,
}

impl ProgramHeadKind {
    #[must_use]
    pub const fn wire_byte(self) -> u8 {
        match self {
            Self::Ordinary => 0,
            Self::Maintenance => 1,
        }
    }

    #[must_use]
    pub const fn from_wire_byte(byte: u8) -> Option<Self> {
        match byte {
            0 => Some(Self::Ordinary),
            1 => Some(Self::Maintenance),
            _ => None,
        }
    }
}

/// Receipt evidence digest tagged with the digest domain that produced it.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum ReceiptEvidenceDigest {
    /// Digest of the unsigned canonical ordinary protocol receipt.
    OrdinaryUnsignedReceipt([u8; 32]),
    /// SHA-256 of the complete canonical maintenance receipt.
    MaintenanceReceiptSha256([u8; 32]),
}

impl ReceiptEvidenceDigest {
    #[must_use]
    pub const fn bytes(self) -> [u8; 32] {
        match self {
            Self::OrdinaryUnsignedReceipt(digest) | Self::MaintenanceReceiptSha256(digest) => {
                digest
            }
        }
    }

    #[must_use]
    pub const fn head_kind(self) -> ProgramHeadKind {
        match self {
            Self::OrdinaryUnsignedReceipt(_) => ProgramHeadKind::Ordinary,
            Self::MaintenanceReceiptSha256(_) => ProgramHeadKind::Maintenance,
        }
    }
}

/// Exact refusal of a Programs state proof bound to a chain head.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum ProgramBundleError {
    Evidence(ProtocolEvidenceError),
    Interface(InterfaceRefusal),
    HeadKind,
    ChainHeadMismatch,
    SignerMismatch,
    ProgramMismatch,
}

impl Display for ProgramBundleError {
    fn fmt(&self, formatter: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::Evidence(error) => Display::fmt(error, formatter),
            Self::Interface(refusal) => Display::fmt(refusal, formatter),
            Self::HeadKind => {
                formatter.write_str("Programs state proof names a different head kind")
            }
            Self::ChainHeadMismatch => {
                formatter.write_str("Programs state proof is not bound to the verified chain head")
            }
            Self::SignerMismatch => formatter
                .write_str("protected sequencer trust and the authenticated signer disagree"),
            Self::ProgramMismatch => {
                formatter.write_str("Programs state proof names a different program")
            }
        }
    }
}

impl std::error::Error for ProgramBundleError {}

/// Verifier bound to a persisted chain validator set and a header freshness limit.
#[derive(Clone, Debug)]
pub struct ProtocolDeploymentVerifier {
    pub(crate) anchors: Vec<core::convert::Infallible>,
    pub(crate) current_anchor: usize,
    pub(crate) staleness_limit_ms: u64,
    pub(crate) signed_history: Option<core::convert::Infallible>,
    pub(crate) chain_verifier: Option<ChainProofVerifier>,
}

impl ProtocolDeploymentVerifier {
    pub fn chain_trust(&self) -> Option<&ChainProofVerifier> {
        self.chain_verifier.as_ref()
    }

    #[must_use]
    pub const fn staleness_limit_ms(&self) -> u64 {
        self.staleness_limit_ms
    }
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub(crate) struct ProgramRecord {
    pub(crate) program: ProgramId,
    pub(crate) version: u32,
    pub(crate) code_hash: [u8; 32],
    pub(crate) abi_version: u16,
    pub(crate) policy: UpgradePolicy,
}

fn decode_program_record_value(
    key: &[u8],
    value: &[u8],
) -> Result<ProgramRecord, ProtocolEvidenceError> {
    if key.len() != PROGRAM_KEY_PREFIX.len() + 32
        || key.get(..PROGRAM_KEY_PREFIX.len()) != Some(PROGRAM_KEY_PREFIX)
        || value.len() != PROGRAM_RECORD_BYTES
    {
        return Err(ProtocolEvidenceError::ProgramRecord);
    }
    let program = ProgramId::new(
        key[PROGRAM_KEY_PREFIX.len()..]
            .try_into()
            .map_err(|_| ProtocolEvidenceError::ProgramRecord)?,
    )
    .map_err(|_| ProtocolEvidenceError::ProgramRecord)?;
    let authority: [u8; 32] = value[1..33]
        .try_into()
        .map_err(|_| ProtocolEvidenceError::ProgramRecord)?;
    let policy = match value[0] {
        0 if authority == [0; 32] => UpgradePolicy::Immutable,
        1 if authority != [0; 32] => UpgradePolicy::Authority(authority),
        _ => return Err(ProtocolEvidenceError::ProgramRecord),
    };
    let code_hash = value[33..65]
        .try_into()
        .map_err(|_| ProtocolEvidenceError::ProgramRecord)?;
    let abi_version = u16::from_be_bytes(
        value[65..67]
            .try_into()
            .map_err(|_| ProtocolEvidenceError::ProgramRecord)?,
    );
    let version = u32::from_be_bytes(
        value[67..71]
            .try_into()
            .map_err(|_| ProtocolEvidenceError::ProgramRecord)?,
    );
    if code_hash == [0; 32] || admit_abi_version(abi_version).is_err() || version == 0 {
        return Err(ProtocolEvidenceError::ProgramRecord);
    }
    Ok(ProgramRecord {
        program,
        version,
        code_hash,
        abi_version,
        policy,
    })
}

fn put_bytes(bytes: &mut Vec<u8>, value: &[u8]) {
    bytes.extend_from_slice(&u32::try_from(value.len()).unwrap_or(u32::MAX).to_be_bytes());
    bytes.extend_from_slice(value);
}

fn take_bytes(
    bytes: &[u8],
    cursor: &mut usize,
    limit: usize,
) -> Result<Vec<u8>, ProtocolEvidenceError> {
    let length = usize::try_from(u32::from_be_bytes(take_array::<4>(bytes, cursor)?))
        .map_err(|_| ProtocolEvidenceError::Encoding)?;
    if length > limit {
        return Err(ProtocolEvidenceError::Encoding);
    }
    let end = cursor
        .checked_add(length)
        .ok_or(ProtocolEvidenceError::Encoding)?;
    let value = bytes
        .get(*cursor..end)
        .ok_or(ProtocolEvidenceError::Encoding)?
        .to_vec();
    *cursor = end;
    Ok(value)
}

fn take_array<const N: usize>(
    bytes: &[u8],
    cursor: &mut usize,
) -> Result<[u8; N], ProtocolEvidenceError> {
    let end = cursor
        .checked_add(N)
        .ok_or(ProtocolEvidenceError::Encoding)?;
    let value = bytes
        .get(*cursor..end)
        .and_then(|slice| slice.try_into().ok())
        .ok_or(ProtocolEvidenceError::Encoding)?;
    *cursor = end;
    Ok(value)
}

use crate::chain_evidence::{
    deployment::ChainDeploymentProof, freshness::chain_freshness, lifecycle::ChainLifecycleProof,
    program_state, program_state::ChainProgramStateProof, protocol_head::ChainProtocolHeadProof,
    wind_down, wind_down::ChainWindDownProof,
};
use crate::chain_proof::{
    decode::{CommitmentProof, ProofOp},
    header::ChainHeader,
    merkle::SimpleProof,
    validators::ValidatorSet,
    ChainProofError, Commit, VerifiedChainHead as VerifiedHeader,
};

#[path = "chain_evidence/verify_head.rs"]
mod head_helpers;
#[path = "chain_evidence/verify_program.rs"]
mod program_helpers;
mod bundle_helpers {
    include!("chain_evidence/verify_bundle.rs");

    pub(super) fn commitment_head(
        verifier: &ChainProofVerifier,
        proof: &CommitmentProof,
    ) -> Result<(ChainHeader, Commit, VerifiedChainHead), ProtocolEvidenceError> {
        let header = ChainHeader::decode(&proof.signed_header)
            .map_err(|_| ProtocolEvidenceError::Encoding)?;
        let commit = decode_commit(&proof.commit).map_err(|_| ProtocolEvidenceError::Encoding)?;
        let validators =
            decode_validators(&proof.validator_set).map_err(|_| ProtocolEvidenceError::Encoding)?;
        if validators != verifier.validators {
            return Err(ProtocolEvidenceError::InvalidTrustAnchor);
        }
        let head = verifier
            .verify_head(&header, &commit)
            .map_err(super::chain_error)?;
        if proof.store_name != "layerx" {
            return Err(ProtocolEvidenceError::StateProof);
        }
        match &proof.value {
            Some(value) => verify_state(
                &head,
                &proof.store_name,
                &proof.key,
                value,
                &proof.proof_ops,
            ),
            None => verify_absent(&head, &proof.store_name, &proof.key, &proof.proof_ops),
        }
        .map_err(super::chain_error)?;
        Ok((header, commit, head))
    }
}

/// An ICS23 leaf and the proof operations that authenticate it.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct StateLeafWitness {
    pub key: Vec<u8>,
    pub value: Vec<u8>,
    pub proof: Vec<ProofOp>,
}

pub(crate) fn decode_program_record(
    witness: &StateLeafWitness,
) -> Result<ProgramRecord, ProtocolEvidenceError> {
    decode_program_record_value(&witness.key, &witness.value)
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ProgramLifecycleProof(pub ChainLifecycleProof);

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ProgramStateProof {
    pub commitment: CommitmentProof,
    pub state: ChainProgramStateProof,
    pub lifecycle: ProgramLifecycleProof,
    pub wind_down: ChainWindDownProof,
    pub transaction: Vec<u8>,
    pub transaction_proof: SimpleProof,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct DeploymentProof {
    pub state: ProgramStateProof,
    pub deployment: ChainDeploymentProof,
}

pub struct ProtocolHeadProof<'a> {
    pub commitment: &'a CommitmentProof,
    pub chain: ChainProtocolHeadProof<'a>,
    pub transaction: &'a [u8],
    pub transaction_proof: &'a SimpleProof,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct VerifiedDeploymentEvidence {
    record: DeploymentRecord,
    interface: Option<crate::ProgramInterface>,
    lifecycle: ProgramLifecycle,
    header: VerifiedHeader,
    transaction_hash: [u8; 32],
    freshness: ReadFreshness,
    proof: DeploymentProof,
}

impl VerifiedDeploymentEvidence {
    pub(crate) fn into_record(self) -> DeploymentRecord {
        self.record
    }
    pub const fn program(&self) -> ProgramId {
        self.record.program
    }
    pub const fn version(&self) -> u32 {
        self.record.version
    }
    pub const fn code_hash(&self) -> [u8; 32] {
        self.record.new_code_hash
    }
    pub const fn abi_version(&self) -> u16 {
        self.record.abi_version
    }
    pub const fn interface_present(&self) -> bool {
        self.interface.is_some()
    }
    pub const fn interface(&self) -> Option<&crate::ProgramInterface> {
        self.interface.as_ref()
    }
    pub const fn receipt_digest(&self) -> [u8; 32] {
        self.transaction_hash
    }
    pub const fn activity_id(&self) -> [u8; 32] {
        self.transaction_hash
    }
    pub const fn state_root(&self) -> [u8; 32] {
        self.header.app_hash()
    }
    pub const fn programs_root(&self) -> [u8; 32] {
        self.header.app_hash()
    }
    pub const fn freshness(&self) -> ReadFreshness {
        self.freshness
    }
    pub const fn lifecycle(&self) -> ProgramLifecycle {
        self.lifecycle
    }
    pub fn module(&self) -> &[u8] {
        &self.record.module
    }
    pub const fn record(&self) -> &DeploymentRecord {
        &self.record
    }
    pub const fn proof(&self) -> &DeploymentProof {
        &self.proof
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct VerifiedProgramHead {
    program: ProgramId,
    version: u32,
    code_hash: [u8; 32],
    abi_version: u16,
    policy: UpgradePolicy,
    lifecycle: ProgramLifecycle,
    header: VerifiedHeader,
    transaction_hash: [u8; 32],
    freshness: ReadFreshness,
    valid_until_ms: u64,
}
impl VerifiedProgramHead {
    pub const fn program(&self) -> ProgramId {
        self.program
    }
    pub const fn version(&self) -> u32 {
        self.version
    }
    pub const fn code_hash(&self) -> [u8; 32] {
        self.code_hash
    }
    pub const fn abi_version(&self) -> u16 {
        self.abi_version
    }
    pub const fn policy(&self) -> UpgradePolicy {
        self.policy
    }
    pub const fn lifecycle(&self) -> ProgramLifecycle {
        self.lifecycle
    }
    pub const fn receipt_digest(&self) -> [u8; 32] {
        self.transaction_hash
    }
    pub const fn state_root(&self) -> [u8; 32] {
        self.header.app_hash()
    }
    pub const fn programs_root(&self) -> [u8; 32] {
        self.header.app_hash()
    }
    pub const fn freshness(&self) -> ReadFreshness {
        self.freshness
    }
    pub const fn valid_until_ms(&self) -> u64 {
        self.valid_until_ms
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct VerifiedProtocolHead {
    header: VerifiedHeader,
    transaction_hash: [u8; 32],
    freshness: ReadFreshness,
}
impl VerifiedProtocolHead {
    pub const fn activity_id(&self) -> [u8; 32] {
        self.transaction_hash
    }
    pub const fn receipt_digest(&self) -> [u8; 32] {
        self.transaction_hash
    }
    pub const fn state_root(&self) -> [u8; 32] {
        self.header.app_hash()
    }
    pub const fn programs_root(&self) -> [u8; 32] {
        self.header.app_hash()
    }
    pub const fn freshness(&self) -> ReadFreshness {
        self.freshness
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct VerifiedChainHead {
    protocol: VerifiedProtocolHead,
    evidence_digest: ReceiptEvidenceDigest,
}
impl VerifiedChainHead {
    pub const fn head_kind(&self) -> ProgramHeadKind {
        self.evidence_digest.head_kind()
    }
    pub const fn receipt_evidence_digest(&self) -> ReceiptEvidenceDigest {
        self.evidence_digest
    }
    pub const fn receipt_digest(&self) -> [u8; 32] {
        self.protocol.receipt_digest()
    }
    pub const fn state_root(&self) -> [u8; 32] {
        self.protocol.state_root()
    }
    pub const fn programs_root(&self) -> [u8; 32] {
        self.protocol.programs_root()
    }
    pub const fn freshness(&self) -> ReadFreshness {
        self.protocol.freshness()
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ProgramStateBundle {
    pub commitment: CommitmentProof,
    pub program: ProgramId,
    pub program_record_ops: Vec<ProofOp>,
    pub interface_ops: Vec<ProofOp>,
    pub wind_down: Option<ChainWindDownProof>,
}
impl ProgramStateBundle {
    fn chain_bundle(&self) -> bundle_helpers::ProgramStateBundle {
        bundle_helpers::ProgramStateBundle {
            commitment: self.commitment.clone(),
            program: self.program,
            program_record_ops: self.program_record_ops.clone(),
            interface_ops: self.interface_ops.clone(),
            wind_down: self.wind_down.clone(),
        }
    }
    pub fn canonical_encoding(&self) -> Result<Vec<u8>, ProgramBundleError> {
        self.chain_bundle().canonical_encoding()
    }
    pub fn decode(bytes: &[u8]) -> Result<Self, ProgramBundleError> {
        let bundle = bundle_helpers::ProgramStateBundle::decode(bytes)?;
        Ok(Self {
            commitment: bundle.commitment,
            program: bundle.program,
            program_record_ops: bundle.program_record_ops,
            interface_ops: bundle.interface_ops,
            wind_down: bundle.wind_down,
        })
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct VerifiedProgramBundle {
    program: VerifiedProgramHead,
    chain: VerifiedChainHead,
    interface: VerifiedInterfaceRead,
}
impl VerifiedProgramBundle {
    pub const fn program_head(&self) -> &VerifiedProgramHead {
        &self.program
    }
    pub const fn chain_head(&self) -> &VerifiedChainHead {
        &self.chain
    }
    pub const fn interface(&self) -> &VerifiedInterfaceRead {
        &self.interface
    }
}

fn chain_error(error: ChainProofError) -> ProtocolEvidenceError {
    match error {
        ChainProofError::Expired => ProtocolEvidenceError::Expired,
        _ => ProtocolEvidenceError::ChainProof,
    }
}

impl ProtocolDeploymentVerifier {
    fn require_chain_trust(&self) -> Result<&ChainProofVerifier, ProtocolEvidenceError> {
        self.chain_trust()
            .ok_or(ProtocolEvidenceError::InvalidTrustAnchor)
    }

    fn state_head(
        &self,
        proof: &ProgramStateProof,
    ) -> Result<VerifiedHeader, ProtocolEvidenceError> {
        let (_, _, head) =
            bundle_helpers::commitment_head(self.require_chain_trust()?, &proof.commitment)?;
        Ok(head)
    }

    pub fn verify_deployment(
        &self,
        proof: &DeploymentProof,
        now_ms: u64,
    ) -> Result<VerifiedDeploymentEvidence, ProtocolEvidenceError> {
        let head = self.state_head(&proof.state)?;
        if head.chain_id() != self.require_chain_trust()?.chain_id {
            return Err(ProtocolEvidenceError::ProtocolDomain);
        }
        chain_freshness(&head, now_ms, self.staleness_limit_ms)?;
        let deployment = proof.deployment.verify(&head)?;
        self.bind_chain_deployment(proof, head, deployment, now_ms)
    }

    pub fn verify_historical_deployment(
        &self,
        proof: &DeploymentProof,
        current: &VerifiedHeader,
        trust_chain: &[(ChainHeader, Commit, ValidatorSet)],
        now_ms: u64,
    ) -> Result<VerifiedDeploymentEvidence, ProtocolEvidenceError> {
        let head = self.state_head(&proof.state)?;
        let mut trust = self.require_chain_trust()?.clone();
        if head.chain_id() != trust.chain_id || current.chain_id() != trust.chain_id {
            return Err(ProtocolEvidenceError::ProtocolDomain);
        }
        chain_freshness(current, now_ms, self.staleness_limit_ms)?;
        let ((header, commit, validators), remaining) = trust_chain
            .split_first()
            .ok_or(ProtocolEvidenceError::HistoricalTrustAnchor)?;
        if validators != &trust.validators {
            return Err(ProtocolEvidenceError::InvalidTrustAnchor);
        }
        let mut trusted_head = trust
            .verify_head(header, commit)
            .map_err(|_| ProtocolEvidenceError::HistoricalTrustAnchor)?;
        if trusted_head != head {
            return Err(ProtocolEvidenceError::HistoricalTrustAnchor);
        }
        for (header, commit, validators) in remaining {
            trust = trust
                .advance(&trusted_head, header, commit, validators.clone())
                .map_err(|_| ProtocolEvidenceError::HistoricalTrustAnchor)?;
            trusted_head = trust
                .verify_head(header, commit)
                .map_err(|_| ProtocolEvidenceError::HistoricalTrustAnchor)?;
        }
        if &trusted_head != current {
            return Err(ProtocolEvidenceError::HistoricalTrustAnchor);
        }
        let deployment = proof.deployment.verify(&head)?;
        let historical_time =
            u64::try_from(head.time_ms()).map_err(|_| ProtocolEvidenceError::Stale)?;
        self.bind_chain_deployment(proof, head, deployment, historical_time)
    }

    fn bind_chain_deployment(
        &self,
        proof: &DeploymentProof,
        header: VerifiedHeader,
        deployment: crate::chain_evidence::deployment::VerifiedDeploymentEvidence,
        now_ms: u64,
    ) -> Result<VerifiedDeploymentEvidence, ProtocolEvidenceError> {
        if proof.state.state != proof.deployment.program_state {
            return Err(ProtocolEvidenceError::DeploymentMismatch);
        }
        let program = program_helpers::current_program_at(
            &header,
            &proof.state.state,
            &proof.state.lifecycle.0,
            &proof.state.wind_down,
            deployment.program(),
            now_ms,
            self.staleness_limit_ms,
        )?;
        if program.lifecycle() != ProgramLifecycle::Active {
            return Err(ProtocolEvidenceError::LifecycleProof);
        }
        let transaction_hash = crate::verify_tx(
            &header,
            &proof.state.transaction,
            &proof.state.transaction_proof,
        )
        .map_err(chain_error)?;
        let freshness = chain_freshness(&header, now_ms, self.staleness_limit_ms)?;
        let record = DeploymentRecord {
            program: deployment.program(),
            version: deployment.version(),
            abi_version: deployment.abi_version(),
            upgrade_policy: deployment.upgrade_policy(),
            old_code_hash: None,
            new_code_hash: deployment.code_hash(),
            sequence: freshness.observed_sequence,
            observed_at: freshness.observed_at,
            module: deployment.module().to_vec(),
            migration: None,
        };
        Ok(VerifiedDeploymentEvidence {
            record,
            interface: deployment.interface().cloned(),
            lifecycle: program.lifecycle(),
            header,
            transaction_hash,
            freshness,
            proof: proof.clone(),
        })
    }

    pub fn verify_current_program(
        &self,
        proof: &ProgramStateProof,
        expected_program: ProgramId,
        now_ms: u64,
    ) -> Result<VerifiedProgramHead, ProtocolEvidenceError> {
        let header = self.state_head(proof)?;
        let verified = program_helpers::current_program_at(
            &header,
            &proof.state,
            &proof.lifecycle.0,
            &proof.wind_down,
            expected_program,
            now_ms,
            self.staleness_limit_ms,
        )?;
        let transaction_hash =
            crate::verify_tx(&header, &proof.transaction, &proof.transaction_proof)
                .map_err(chain_error)?;
        Ok(VerifiedProgramHead {
            program: verified.program(),
            version: verified.version(),
            code_hash: verified.code_hash(),
            abi_version: verified.abi_version(),
            policy: verified.policy(),
            lifecycle: verified.lifecycle(),
            header,
            transaction_hash,
            freshness: verified.freshness(),
            valid_until_ms: verified.valid_until_ms(),
        })
    }

    pub fn verify_current_protocol_head(
        &self,
        proof: &ProtocolHeadProof<'_>,
        now_ms: u64,
    ) -> Result<VerifiedProtocolHead, ProtocolEvidenceError> {
        let (_, _, header) =
            bundle_helpers::commitment_head(self.require_chain_trust()?, proof.commitment)?;
        let verified = head_helpers::protocol_head_at(
            self.require_chain_trust()?,
            &proof.chain,
            now_ms,
            self.staleness_limit_ms,
        )?;
        if verified.header_hash() != header.header_hash() {
            return Err(ProtocolEvidenceError::StateRoot);
        }
        let transaction_hash =
            crate::verify_tx(&header, proof.transaction, proof.transaction_proof)
                .map_err(chain_error)?;
        Ok(VerifiedProtocolHead {
            header,
            transaction_hash,
            freshness: verified.freshness(),
        })
    }

    pub fn verify_current_protocol_head_proof(
        &self,
        proof: &ProtocolHeadProof<'_>,
        now_ms: u64,
    ) -> Result<VerifiedProtocolHead, ProtocolEvidenceError> {
        self.verify_current_protocol_head(proof, now_ms)
    }

    pub fn verify_historical_protocol_head(
        &self,
        proof: &ProtocolHeadProof<'_>,
        trusted_header: &ChainHeader,
        trusted_commit: &Commit,
        history: &[(ChainHeader, Commit, ValidatorSet)],
    ) -> Result<VerifiedProtocolHead, ProtocolEvidenceError> {
        let steps: Vec<_> = history
            .iter()
            .map(
                |(header, commit, validators)| head_helpers::ChainTrustStep {
                    header,
                    commit,
                    validators,
                },
            )
            .collect();
        let verified = head_helpers::historical_protocol_head_at(
            self.require_chain_trust()?,
            trusted_header,
            trusted_commit,
            &steps,
            &proof.chain,
        )?;
        let (_, _, header) =
            bundle_helpers::commitment_head(proof.chain.verifier, proof.commitment)?;
        if verified.header_hash() != header.header_hash() {
            return Err(ProtocolEvidenceError::StateRoot);
        }
        let transaction_hash =
            crate::verify_tx(&header, proof.transaction, proof.transaction_proof)
                .map_err(chain_error)?;
        Ok(VerifiedProtocolHead {
            header,
            transaction_hash,
            freshness: verified.freshness(),
        })
    }

    pub fn verify_historical_protocol_head_proof(
        &self,
        proof: &ProtocolHeadProof<'_>,
        trusted_header: &ChainHeader,
        trusted_commit: &Commit,
        history: &[(ChainHeader, Commit, ValidatorSet)],
    ) -> Result<VerifiedProtocolHead, ProtocolEvidenceError> {
        self.verify_historical_protocol_head(proof, trusted_header, trusted_commit, history)
    }

    pub fn verify_current_chain_head(
        &self,
        kind: ProgramHeadKind,
        proof: &CommitmentProof,
        transaction: &[u8],
        transaction_proof: &SimpleProof,
        now_ms: u64,
    ) -> Result<VerifiedChainHead, ProtocolEvidenceError> {
        let (raw_header, commit, header) =
            bundle_helpers::commitment_head(self.require_chain_trust()?, proof)?;
        let verified = head_helpers::chain_head_at(
            self.require_chain_trust()?,
            kind,
            &raw_header,
            &commit,
            now_ms,
            self.staleness_limit_ms,
        )?;
        let transaction_hash =
            crate::verify_tx(&header, transaction, transaction_proof).map_err(chain_error)?;
        let evidence_digest = match kind {
            ProgramHeadKind::Ordinary => {
                ReceiptEvidenceDigest::OrdinaryUnsignedReceipt(transaction_hash)
            }
            ProgramHeadKind::Maintenance => {
                ReceiptEvidenceDigest::MaintenanceReceiptSha256(transaction_hash)
            }
        };
        Ok(VerifiedChainHead {
            protocol: VerifiedProtocolHead {
                header,
                transaction_hash,
                freshness: verified.freshness(),
            },
            evidence_digest,
        })
    }

    pub fn verify_current_program_bundle(
        &self,
        bundle: &ProgramStateBundle,
        chain: &VerifiedChainHead,
        expected_program: ProgramId,
        now_ms: u64,
    ) -> Result<VerifiedProgramBundle, ProgramBundleError> {
        let verified = bundle_helpers::program_bundle_at(
            self.require_chain_trust()
                .map_err(ProgramBundleError::Evidence)?,
            &bundle.chain_bundle(),
            expected_program,
        )?;
        if verified.chain_head() != &chain.protocol.header {
            return Err(ProgramBundleError::ChainHeadMismatch);
        }
        let freshness = chain_freshness(verified.chain_head(), now_ms, self.staleness_limit_ms)
            .map_err(ProgramBundleError::Evidence)?;
        let valid_until_ms = freshness
            .observed_at
            .checked_add(self.staleness_limit_ms)
            .ok_or(ProgramBundleError::Evidence(ProtocolEvidenceError::Stale))?;
        let status = verified.wind_down().ok_or(ProgramBundleError::Evidence(
            ProtocolEvidenceError::LifecycleProof,
        ))?;
        let lifecycle = match status {
            None => ProgramLifecycle::Active,
            Some(value) if value[1] == 2 => ProgramLifecycle::Deprecated,
            _ => {
                return Err(ProgramBundleError::Evidence(
                    ProtocolEvidenceError::LifecycleProof,
                ))
            }
        };
        let record = verified.program_head();
        let program = VerifiedProgramHead {
            program: record.program(),
            version: record.version(),
            code_hash: record.code_hash(),
            abi_version: record.abi_version(),
            policy: record.upgrade_policy(),
            lifecycle,
            header: verified.chain_head().clone(),
            transaction_hash: chain.receipt_digest(),
            freshness,
            valid_until_ms,
        };
        Ok(VerifiedProgramBundle {
            program,
            chain: chain.clone(),
            interface: verified.interface().clone(),
        })
    }
}

impl DeploymentProof {
    pub fn canonical_encoding(&self) -> Result<Vec<u8>, ProtocolEvidenceError> {
        let mut bytes = b"LXDP\x02".to_vec();
        put_chain_field(
            &mut bytes,
            &self.state.commitment.encode().map_err(chain_error)?,
        )?;
        bytes.extend_from_slice(&self.state.state.program.bytes());
        put_chain_ops(&mut bytes, &self.state.state.ops)?;
        let lifecycle = &self.state.lifecycle.0;
        bytes.extend_from_slice(&lifecycle.program.bytes());
        put_chain_option(&mut bytes, lifecycle.record.as_deref())?;
        put_chain_ops(&mut bytes, &lifecycle.record_ops)?;
        put_chain_option(&mut bytes, lifecycle.status.as_deref())?;
        put_chain_ops(&mut bytes, &lifecycle.status_ops)?;
        bytes.extend_from_slice(&self.state.wind_down.program.bytes());
        put_chain_option(&mut bytes, self.state.wind_down.value.as_deref())?;
        put_chain_ops(&mut bytes, &self.state.wind_down.ops)?;
        put_chain_field(&mut bytes, &self.state.transaction)?;
        let proof = &self.state.transaction_proof;
        if proof.aunts.len() > 64 {
            return Err(ProtocolEvidenceError::Encoding);
        }
        bytes.extend_from_slice(&proof.total.to_be_bytes());
        bytes.extend_from_slice(&proof.index.to_be_bytes());
        bytes.extend_from_slice(&proof.leaf_hash);
        bytes.push(proof.aunts.len() as u8);
        for aunt in &proof.aunts {
            bytes.extend_from_slice(aunt);
        }
        bytes.extend_from_slice(&self.deployment.program_state.program.bytes());
        put_chain_ops(&mut bytes, &self.deployment.program_state.ops)?;
        if self.deployment.code.len() > MAX_MODULE_BYTES {
            return Err(ProtocolEvidenceError::Encoding);
        }
        put_chain_field(&mut bytes, &self.deployment.code)?;
        put_chain_option(&mut bytes, self.deployment.interface.as_deref())?;
        if bytes.len() > MAX_EVIDENCE_BYTES {
            return Err(ProtocolEvidenceError::Encoding);
        }
        Ok(bytes)
    }

    pub fn decode(bytes: &[u8]) -> Result<Self, ProtocolEvidenceError> {
        if bytes.len() > MAX_EVIDENCE_BYTES || bytes.get(..5) != Some(b"LXDP\x02") {
            return Err(ProtocolEvidenceError::Encoding);
        }
        let mut cursor = 5;
        let commitment = crate::chain_proof::decode::decode_commitment_proof(&take_bytes(
            bytes,
            &mut cursor,
            MAX_EVIDENCE_BYTES,
        )?)
        .map_err(chain_error)?;
        let state = ChainProgramStateProof {
            program: take_chain_program(bytes, &mut cursor)?,
            ops: take_chain_ops(bytes, &mut cursor)?,
        };
        let lifecycle = ProgramLifecycleProof(ChainLifecycleProof {
            program: take_chain_program(bytes, &mut cursor)?,
            record: take_chain_option(bytes, &mut cursor)?,
            record_ops: take_chain_ops(bytes, &mut cursor)?,
            status: take_chain_option(bytes, &mut cursor)?,
            status_ops: take_chain_ops(bytes, &mut cursor)?,
        });
        let wind_down = ChainWindDownProof {
            program: take_chain_program(bytes, &mut cursor)?,
            value: take_chain_option(bytes, &mut cursor)?,
            ops: take_chain_ops(bytes, &mut cursor)?,
        };
        let transaction = take_bytes(bytes, &mut cursor, MAX_EVIDENCE_BYTES)?;
        let total = i64::from_be_bytes(take_array(bytes, &mut cursor)?);
        let index = i64::from_be_bytes(take_array(bytes, &mut cursor)?);
        let leaf_hash = take_array(bytes, &mut cursor)?;
        let count = take_array::<1>(bytes, &mut cursor)?[0];
        if count > 64 {
            return Err(ProtocolEvidenceError::Encoding);
        }
        let mut aunts = Vec::with_capacity(usize::from(count));
        for _ in 0..count {
            aunts.push(take_array(bytes, &mut cursor)?);
        }
        let transaction_proof = SimpleProof {
            total,
            index,
            leaf_hash,
            aunts,
        };
        let program_state = ChainProgramStateProof {
            program: take_chain_program(bytes, &mut cursor)?,
            ops: take_chain_ops(bytes, &mut cursor)?,
        };
        let code = take_bytes(bytes, &mut cursor, MAX_MODULE_BYTES)?;
        let interface = take_chain_option(bytes, &mut cursor)?;
        let proof = Self {
            state: ProgramStateProof {
                commitment,
                state,
                lifecycle,
                wind_down,
                transaction,
                transaction_proof,
            },
            deployment: ChainDeploymentProof {
                program_state,
                code,
                interface,
            },
        };
        if cursor != bytes.len() || proof.canonical_encoding()? != bytes {
            return Err(ProtocolEvidenceError::Encoding);
        }
        Ok(proof)
    }

    pub fn claimed_receipt_digest(&self) -> Result<[u8; 32], ProtocolEvidenceError> {
        if self.state.transaction.is_empty() || self.state.transaction.len() > MAX_EVIDENCE_BYTES {
            return Err(ProtocolEvidenceError::Encoding);
        }
        Ok(crate::hash::sha256(&self.state.transaction))
    }
}

fn put_chain_field(bytes: &mut Vec<u8>, field: &[u8]) -> Result<(), ProtocolEvidenceError> {
    if field.len() > MAX_EVIDENCE_BYTES {
        return Err(ProtocolEvidenceError::Encoding);
    }
    put_bytes(bytes, field);
    Ok(())
}
fn put_chain_option(
    bytes: &mut Vec<u8>,
    field: Option<&[u8]>,
) -> Result<(), ProtocolEvidenceError> {
    match field {
        None => bytes.push(0),
        Some(value) => {
            bytes.push(1);
            put_chain_field(bytes, value)?;
        }
    }
    Ok(())
}
fn take_chain_option(
    bytes: &[u8],
    cursor: &mut usize,
) -> Result<Option<Vec<u8>>, ProtocolEvidenceError> {
    match take_array::<1>(bytes, cursor)?[0] {
        0 => Ok(None),
        1 => Ok(Some(take_bytes(bytes, cursor, MAX_EVIDENCE_BYTES)?)),
        _ => Err(ProtocolEvidenceError::Encoding),
    }
}
fn put_chain_ops(bytes: &mut Vec<u8>, ops: &[ProofOp]) -> Result<(), ProtocolEvidenceError> {
    if ops.len() > crate::chain_proof::decode::MAX_PROOF_OPS {
        return Err(ProtocolEvidenceError::Encoding);
    }
    bytes.push(ops.len() as u8);
    for op in ops {
        for value in [op.op_type.as_bytes(), &op.key, &op.data] {
            if value.len() > crate::chain_proof::decode::MAX_FIELD_BYTES {
                return Err(ProtocolEvidenceError::Encoding);
            }
            put_chain_field(bytes, value)?;
        }
    }
    Ok(())
}
fn take_chain_ops(bytes: &[u8], cursor: &mut usize) -> Result<Vec<ProofOp>, ProtocolEvidenceError> {
    let count = take_array::<1>(bytes, cursor)?[0];
    if usize::from(count) > crate::chain_proof::decode::MAX_PROOF_OPS {
        return Err(ProtocolEvidenceError::Encoding);
    }
    let mut ops = Vec::with_capacity(usize::from(count));
    for _ in 0..count {
        let op_type = String::from_utf8(take_bytes(
            bytes,
            cursor,
            crate::chain_proof::decode::MAX_FIELD_BYTES,
        )?)
        .map_err(|_| ProtocolEvidenceError::Encoding)?;
        ops.push(ProofOp {
            op_type,
            key: take_bytes(bytes, cursor, crate::chain_proof::decode::MAX_FIELD_BYTES)?,
            data: take_bytes(bytes, cursor, crate::chain_proof::decode::MAX_FIELD_BYTES)?,
        });
    }
    Ok(ops)
}
fn take_chain_program(
    bytes: &[u8],
    cursor: &mut usize,
) -> Result<ProgramId, ProtocolEvidenceError> {
    ProgramId::new(take_array(bytes, cursor)?).map_err(|_| ProtocolEvidenceError::Encoding)
}
