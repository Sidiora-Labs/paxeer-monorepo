//! Raw protocol evidence ingress and verifier-owned trusted protocol policy.

use std::collections::BTreeSet;

use layerx_programs::hex;
use layerx_proof::inclusion::{
    verify_receipt as verify_receipt_inclusion, verify_state, InclusionError, InclusionEvidence,
    SequencerAuthorization,
};
use layerx_proof::merkle::Proof;
use layerx_proof::receipt::{
    verify_outcome, verify_sequencer_signature, AuthorizedBatch, ReceiptCheck, VerificationFailure,
    VerifiedReceipt as ProofVerifiedReceipt,
};
use layerx_types::ids::Did;
use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};
use layerx_types::verify::VerificationLevel;
use layerx_wire::hash::receipt_execution_batch_id;
use layerx_wire::historical::activity::{decode_signed, peek_signed_activity_type};
use layerx_wire::historical::hash::activity_id;
use layerx_wire::limits::PROTOCOL_VERSION;
use layerx_wire::receipt::{decode, decode_batch_header, BatchHeader};
use sha2::{Digest, Sha256};

use crate::config::{read_protected_source, ProtectedSourceError, ReceiptKeySource, StartupConfig};
use crate::human::HumanOperationError;
use layerx_client::sequencer::SequencerEndpoint;

const MAX_AUTHORITY_SOURCE_BYTES: usize = 65_536;
const AUTHORITY_SOURCE_VERSION: &str = "layerx-sequencer-authority-v1";
const MAX_CUMULATIVE_EVIDENCE_ITEMS: usize = 4_096;
const KERNEL_MODULE_COUNT: u16 = 11;

#[derive(Debug)]
enum ActivityActorBinding {
    Did(Vec<u8>),
    Account([u8; 20]),
}

struct UseActivity {
    id: [u8; 32],
    actor: ActivityActorBinding,
    envelope_context: Option<(u16, u32)>,
}

enum UseActivityError {
    Activity,
    Identity,
}

fn decode_use_activity(
    canonical: &[u8],
    receipt_module: Option<ModuleId>,
) -> Result<UseActivity, UseActivityError> {
    if receipt_module.is_none() && canonical.first() == Some(&0x02) {
        let (call, signature) = layerx_crypto::evm_transaction::decode_signed(canonical)
            .map_err(|_| UseActivityError::Activity)?;
        let sender = layerx_crypto::evm_transaction::recover_sender(&call, &signature)
            .map_err(|_| UseActivityError::Activity)?;
        return Ok(UseActivity {
            id: layerx_crypto::evm_transaction::transaction_hash(canonical),
            actor: ActivityActorBinding::Account(sender),
            envelope_context: None,
        });
    }
    let kind = peek_signed_activity_type(canonical).map_err(|_| UseActivityError::Activity)?;
    if receipt_module.is_some_and(|module| kind.module() != module) {
        return Err(UseActivityError::Identity);
    }
    let module = receipt_module.unwrap_or(kind.module());
    let registration =
        ModuleRegistration::new(module, &[kind]).map_err(|_| UseActivityError::Activity)?;
    let registry = ModuleRegistry::new(&[registration]).map_err(|_| UseActivityError::Activity)?;
    let activity = decode_signed(canonical, &registry).map_err(|_| UseActivityError::Activity)?;
    if activity.activity_type() != kind {
        return Err(UseActivityError::Identity);
    }
    Ok(UseActivity {
        id: activity_id(&activity).map_err(|_| UseActivityError::Activity)?,
        actor: ActivityActorBinding::Did(activity.actor_did().to_vec()),
        envelope_context: Some((activity.protocol_version(), activity.network_id())),
    })
}

fn select_receipt_key(key: &layerx_client::sequencer::ReceiptKeyEntry) -> SequencerAuthorization {
    SequencerAuthorization::new(
        key.key_id,
        key.public_key,
        key.first_height,
        key.last_height,
    )
}

/// One sequencer key and validity interval installed by trusted daemon configuration.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub(crate) struct TrustedSequencer {
    sequencer_id: [u8; 32],
    public_key: [u8; 32],
    epoch: u64,
    first_batch_number: u64,
    last_batch_number: u64,
    revoked_at_batch: Option<u64>,
}

impl TrustedSequencer {
    /// Creates one configured sequencer key interval.
    ///
    /// `revoked_at_batch` is inclusive: evidence at or after that batch is refused.
    ///
    /// # Errors
    ///
    /// Refuses an empty identity or key, a reversed batch range, or a revocation
    /// point outside the configured range.
    fn new(
        sequencer_id: [u8; 32],
        public_key: [u8; 32],
        epoch: u64,
        first_batch_number: u64,
        last_batch_number: u64,
        revoked_at_batch: Option<u64>,
    ) -> Result<Self, VerifierPolicyError> {
        if sequencer_id == [0; 32] || public_key == [0; 32] {
            return Err(VerifierPolicyError::InvalidAuthorization);
        }
        if first_batch_number > last_batch_number {
            return Err(VerifierPolicyError::InvalidBatchRange);
        }
        if revoked_at_batch
            .is_some_and(|batch| batch < first_batch_number || batch > last_batch_number)
        {
            return Err(VerifierPolicyError::InvalidRevocation);
        }
        Ok(Self {
            sequencer_id,
            public_key,
            epoch,
            first_batch_number,
            last_batch_number,
            revoked_at_batch,
        })
    }

    const fn active_last_batch(self) -> Option<u64> {
        let last_batch = match self.revoked_at_batch {
            Some(0) => return None,
            Some(revoked_at) => {
                let active_last = revoked_at - 1;
                if active_last < self.last_batch_number {
                    active_last
                } else {
                    self.last_batch_number
                }
            }
            None => self.last_batch_number,
        };
        if self.first_batch_number <= last_batch {
            Some(last_batch)
        } else {
            None
        }
    }
}

/// Immutable trust policy owned by the daemon, never supplied by proof ingress.
#[derive(Clone, Debug, Eq, PartialEq)]
pub(crate) struct ProtocolEvidenceVerifier {
    expected_protocol_version: u16,
    expected_network_id: u32,
    sequencers: Vec<TrustedSequencer>,
    handshake_pin: Option<[u8; 32]>,
}

impl ProtocolEvidenceVerifier {
    /// Builds one exact-network verifier from trusted daemon configuration.
    ///
    /// # Errors
    ///
    /// Refuses zero identities, an empty authority set, and overlapping active
    /// ranges because canonical batch authority must select one sequencer.
    fn new(
        expected_protocol_version: u16,
        expected_network_id: u32,
        mut sequencers: Vec<TrustedSequencer>,
    ) -> Result<Self, VerifierPolicyError> {
        if expected_protocol_version == 0 || expected_network_id == 0 || sequencers.is_empty() {
            return Err(VerifierPolicyError::EmptyPolicy);
        }
        sequencers.sort_by_key(|entry| {
            (
                entry.first_batch_number,
                entry.last_batch_number,
                entry.epoch,
                entry.sequencer_id,
            )
        });
        let mut active_through = None;
        for entry in &sequencers {
            let Some(last_batch) = entry.active_last_batch() else {
                continue;
            };
            if active_through.is_some_and(|previous| entry.first_batch_number <= previous) {
                return Err(VerifierPolicyError::AmbiguousAuthorization);
            }
            active_through = Some(last_batch);
        }
        Ok(Self {
            expected_protocol_version,
            expected_network_id,
            sequencers,
            handshake_pin: None,
        })
    }

    /// Builds a verifier whose only trust anchor is the sequencer key the node
    /// handshake authenticated. Every signed header is admitted for its exact
    /// batch under that key, mirroring the client's pinned-key authorisation.
    fn pinned(
        expected_protocol_version: u16,
        expected_network_id: u32,
        handshake_sequencer_key: [u8; 32],
    ) -> Result<Self, VerifierPolicyError> {
        if expected_protocol_version == 0 || expected_network_id == 0 {
            return Err(VerifierPolicyError::EmptyPolicy);
        }
        if handshake_sequencer_key == [0; 32] {
            return Err(VerifierPolicyError::InvalidAuthorization);
        }
        Ok(Self {
            expected_protocol_version,
            expected_network_id,
            sequencers: Vec::new(),
            handshake_pin: Some(handshake_sequencer_key),
        })
    }

    pub(crate) fn load(config: &StartupConfig) -> Result<Self, VerifierPolicyError> {
        let path = match &config.receipt_key_source {
            ReceiptKeySource::PinnedFile(path) => path,
            ReceiptKeySource::NodeRpc => {
                return Err(VerifierPolicyError::AuthoritySourceUnavailable)
            }
        };
        let bytes =
            read_protected_source(path, MAX_AUTHORITY_SOURCE_BYTES).map_err(
                |failure| match failure {
                    ProtectedSourceError::Unavailable => {
                        VerifierPolicyError::AuthoritySourceUnavailable
                    }
                    ProtectedSourceError::TooLarge => VerifierPolicyError::AuthoritySourceMalformed,
                    ProtectedSourceError::Unprotected | ProtectedSourceError::Changed => {
                        VerifierPolicyError::AuthoritySourceUnprotected
                    }
                },
            )?;
        if bytes.is_empty() {
            return Err(VerifierPolicyError::AuthoritySourceMalformed);
        }
        let source = std::str::from_utf8(&bytes)
            .map_err(|_| VerifierPolicyError::AuthoritySourceMalformed)?;
        let mut lines = source.lines();
        if lines.next() != Some(AUTHORITY_SOURCE_VERSION) {
            return Err(VerifierPolicyError::AuthoritySourceMalformed);
        }
        let mut sequencers = Vec::new();
        for line in lines {
            if line.is_empty() || line.trim() != line {
                return Err(VerifierPolicyError::AuthoritySourceMalformed);
            }
            let mut fields = line.split(',');
            let (
                Some(sequencer_id),
                Some(public_key),
                Some(epoch),
                Some(first_batch),
                Some(last_batch),
                Some(revoked_at),
            ) = (
                fields.next(),
                fields.next(),
                fields.next(),
                fields.next(),
                fields.next(),
                fields.next(),
            )
            else {
                return Err(VerifierPolicyError::AuthoritySourceMalformed);
            };
            if fields.next().is_some() {
                return Err(VerifierPolicyError::AuthoritySourceMalformed);
            }
            let sequencer_id = hex::decode_digest(sequencer_id)
                .map_err(|_| VerifierPolicyError::AuthoritySourceMalformed)?;
            let public_key = hex::decode_digest(public_key)
                .map_err(|_| VerifierPolicyError::AuthoritySourceMalformed)?;
            let epoch = epoch
                .parse()
                .map_err(|_| VerifierPolicyError::AuthoritySourceMalformed)?;
            let first_batch = first_batch
                .parse()
                .map_err(|_| VerifierPolicyError::AuthoritySourceMalformed)?;
            let last_batch = last_batch
                .parse()
                .map_err(|_| VerifierPolicyError::AuthoritySourceMalformed)?;
            let revoked_at_batch = if revoked_at == "active" {
                None
            } else {
                Some(
                    revoked_at
                        .parse()
                        .map_err(|_| VerifierPolicyError::AuthoritySourceMalformed)?,
                )
            };
            sequencers.push(TrustedSequencer::new(
                sequencer_id,
                public_key,
                epoch,
                first_batch,
                last_batch,
                revoked_at_batch,
            )?);
        }
        Self::new(PROTOCOL_VERSION, config.network_id, sequencers)
    }

    pub(crate) fn accepts_handshake_key(&self, batch: u64, public_key: [u8; 32]) -> bool {
        if self.handshake_pin == Some(public_key) {
            return true;
        }
        self.sequencers.iter().any(|entry| {
            entry.public_key == public_key
                && batch >= entry.first_batch_number
                && batch <= entry.last_batch_number
                && entry
                    .revoked_at_batch
                    .is_none_or(|revoked_at| batch < revoked_at)
        })
    }

    fn authorization_for(
        &self,
        canonical_header: &[u8],
    ) -> Result<(BatchHeader, SequencerAuthorization), VerifierPolicyError> {
        let header =
            decode_batch_header(canonical_header).map_err(|_| VerifierPolicyError::HeaderDecode)?;
        if header.protocol_version() != self.expected_protocol_version {
            return Err(VerifierPolicyError::ProtocolVersion);
        }
        if header.network_id() != self.expected_network_id {
            return Err(VerifierPolicyError::Network);
        }
        if self.sequencers.is_empty() {
            let pinned = self.handshake_pin.ok_or(VerifierPolicyError::EmptyPolicy)?;
            let batch = header.batch_number();
            let authorization =
                SequencerAuthorization::new(header.sequencer_id(), pinned, batch, batch);
            return Ok((header, authorization));
        }
        let mut saw_identity = false;
        let mut saw_epoch = false;
        let mut saw_revoked = false;
        let mut selected = None;
        for entry in &self.sequencers {
            if entry.sequencer_id != header.sequencer_id() {
                continue;
            }
            saw_identity = true;
            if entry.epoch != header.epoch() {
                continue;
            }
            saw_epoch = true;
            if header.batch_number() < entry.first_batch_number
                || header.batch_number() > entry.last_batch_number
            {
                continue;
            }
            if entry
                .revoked_at_batch
                .is_some_and(|revoked_at| header.batch_number() >= revoked_at)
            {
                saw_revoked = true;
                continue;
            }
            if selected.replace(entry).is_some() {
                return Err(VerifierPolicyError::AmbiguousAuthorization);
            }
        }
        let Some(entry) = selected else {
            return Err(if !saw_identity {
                VerifierPolicyError::UnknownSequencer
            } else if !saw_epoch {
                VerifierPolicyError::Epoch
            } else if saw_revoked {
                VerifierPolicyError::Revoked
            } else {
                VerifierPolicyError::BatchRange
            });
        };
        let active_last_batch = entry
            .active_last_batch()
            .ok_or(VerifierPolicyError::Revoked)?;
        Ok((
            header,
            SequencerAuthorization::new(
                entry.sequencer_id,
                entry.public_key,
                entry.first_batch_number,
                active_last_batch,
            ),
        ))
    }

    /// Verifies raw receipt ingress and issues an opaque receipt token.
    ///
    /// # Errors
    ///
    /// Refuses policy identity, canonical inclusion, signed batch identity, receipt
    /// signature, root-chain, and sequence failures before issuing evidence.
    fn verify_receipt(
        &self,
        raw: &RawReceiptEvidence,
    ) -> Result<VerifiedReceiptEvidence, ReceiptEvidenceError> {
        self.verify_receipt_retaining(raw)
            .map(|(verified, _)| verified)
    }

    /// Verifies raw receipt ingress and retains the authorised batch exactly
    /// where this verifier establishes it.
    ///
    /// # Errors
    ///
    /// Refuses exactly what [`Self::verify_receipt`] refuses.
    fn verify_receipt_retaining(
        &self,
        raw: &RawReceiptEvidence,
    ) -> Result<(VerifiedReceiptEvidence, AuthorizedBatch), ReceiptEvidenceError> {
        let (selected_header, authorization) = self
            .authorization_for(&raw.canonical_header)
            .map_err(ReceiptEvidenceError::Policy)?;
        let inclusion = verify_receipt_inclusion(
            &raw.canonical_receipt,
            &raw.proof,
            &raw.canonical_header,
            &raw.header_signature,
            &authorization,
        )
        .map_err(ReceiptEvidenceError::Inclusion)?;
        let decoded = decode(&raw.canonical_receipt).map_err(|_| {
            ReceiptEvidenceError::Receipt(VerificationFailure {
                check: ReceiptCheck::Decode,
            })
        })?;
        let protocol =
            decoded
                .protocol()
                .ok_or(ReceiptEvidenceError::Receipt(VerificationFailure {
                    check: ReceiptCheck::ReceiptShape,
                }))?;
        if protocol.protocol_version() != selected_header.protocol_version() {
            return Err(ReceiptEvidenceError::ProtocolVersion);
        }
        if protocol.global_sequence() < selected_header.first_sequence()
            || protocol.global_sequence() > selected_header.last_sequence()
        {
            return Err(ReceiptEvidenceError::SequenceRange);
        }
        let expected_execution_id = receipt_execution_batch_id(protocol, &selected_header)
            .map_err(|_| ReceiptEvidenceError::BatchIdentity)?;
        if protocol.batch_id() != expected_execution_id {
            return Err(ReceiptEvidenceError::BatchIdentity);
        }
        let authorised = AuthorizedBatch::new(
            expected_execution_id,
            protocol.asset(),
            selected_header.previous_state_root(),
            selected_header.resulting_state_root(),
            authorization.public_key(),
        );
        let verified = verify_outcome(&raw.canonical_receipt, &authorised)
            .map_err(ReceiptEvidenceError::Receipt)?;
        let protocol = verified
            .receipt()
            .protocol()
            .ok_or(ReceiptEvidenceError::Receipt(VerificationFailure {
                check: ReceiptCheck::ReceiptShape,
            }))?;
        let receipt_ref = Sha256::digest(verified.canonical_bytes()).into();
        Ok((
            VerifiedReceiptEvidence {
                receipt_ref,
                activity_id: protocol.activity_id(),
                global_sequence: protocol.global_sequence(),
                result_code: protocol.result_code(),
                amount: protocol.amount(),
                module_id: protocol.module_id(),
                operation: protocol.operation(),
                verified,
                inclusion,
            },
            authorised,
        ))
    }

    fn verify_signed_receipt_inclusion(
        &self,
        raw: &RawReceiptEvidence,
        receipt_keys: &[layerx_client::sequencer::ReceiptKeyEntry],
    ) -> Result<VerifiedCumulativeReceipt, ReceiptEvidenceError> {
        let (header, authorization) = self
            .authorization_for(&raw.canonical_header)
            .map_err(ReceiptEvidenceError::Policy)?;
        verify_receipt_inclusion(
            &raw.canonical_receipt,
            &raw.proof,
            &raw.canonical_header,
            &raw.header_signature,
            &authorization,
        )
        .map_err(ReceiptEvidenceError::Inclusion)?;
        let decoded = decode(&raw.canonical_receipt).map_err(|_| {
            ReceiptEvidenceError::Receipt(VerificationFailure {
                check: ReceiptCheck::Decode,
            })
        })?;
        let module_id = decoded
            .protocol()
            .ok_or(ReceiptEvidenceError::Receipt(VerificationFailure {
                check: ReceiptCheck::ReceiptShape,
            }))?
            .module_id();
        let receipt = if module_id > KERNEL_MODULE_COUNT {
            let keys: Vec<_> = receipt_keys.iter().map(select_receipt_key).collect();
            verify_sequencer_signature(&raw.canonical_receipt, keys.as_slice())
        } else {
            verify_sequencer_signature(&raw.canonical_receipt, authorization.public_key())
        }
        .map_err(ReceiptEvidenceError::Receipt)?;
        let protocol =
            receipt
                .protocol()
                .ok_or(ReceiptEvidenceError::Receipt(VerificationFailure {
                    check: ReceiptCheck::ReceiptShape,
                }))?;
        if protocol.protocol_version() != header.protocol_version() {
            return Err(ReceiptEvidenceError::ProtocolVersion);
        }
        if protocol.global_sequence() < header.first_sequence()
            || protocol.global_sequence() > header.last_sequence()
        {
            return Err(ReceiptEvidenceError::SequenceRange);
        }
        Ok(VerifiedCumulativeReceipt::from_protocol(
            protocol, header, &raw.proof,
        ))
    }

    /// Verifies raw state evidence and issues an opaque state token.
    ///
    /// A state leaf is verified as a Merkle path under the header's resulting
    /// state root. A module witness is verified through the client's module
    /// evidence verifier under the pinned handshake key, and that key must be
    /// one this policy accepts for the signed batch.
    ///
    /// # Errors
    ///
    /// Refuses policy identity, canonical header, signature, state-root, Merkle
    /// and module-witness failures before issuing evidence.
    fn verify_state(
        &self,
        raw: &RawStateEvidence,
    ) -> Result<VerifiedStateEvidence, StateEvidenceError> {
        match &raw.shape {
            StateEvidenceShape::Leaf {
                proof,
                resulting_state_root,
                canonical_header,
                header_signature,
            } => {
                let (_, authorization) = self
                    .authorization_for(canonical_header)
                    .map_err(StateEvidenceError::Policy)?;
                let inclusion = verify_state(
                    &raw.canonical_state,
                    proof,
                    resulting_state_root,
                    canonical_header,
                    header_signature,
                    &authorization,
                )
                .map_err(StateEvidenceError::Inclusion)?;
                let observed_head_sequence = inclusion.header().header().last_sequence();
                Ok(VerifiedStateEvidence {
                    canonical_state: raw.canonical_state.clone(),
                    level: inclusion.level(),
                    observed_head_sequence,
                })
            }
            StateEvidenceShape::ModuleWitness {
                committed,
                budget_id,
            } => {
                let checkpoint = committed.checkpoint();
                let verified = crate::read::checkpoint(
                    checkpoint.height,
                    &self.expected_network_id.to_string(),
                    &checkpoint.header,
                    &checkpoint.commit,
                    &checkpoint.validators,
                )
                .map_err(|_| StateEvidenceError::Committed)?;
                if verified != *checkpoint
                    || committed.canonical_bytes() != committed.repeated()
                    || raw.canonical_state != committed_state_bytes(committed, *budget_id)?
                {
                    return Err(StateEvidenceError::Committed);
                }
                Ok(VerifiedStateEvidence {
                    canonical_state: raw.canonical_state.clone(),
                    level: committed.level(),
                    observed_head_sequence: checkpoint.height,
                })
            }
        }
    }
}

/// Exact trusted-policy refusal class.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum VerifierPolicyError {
    AuthoritySourceUnavailable,
    AuthoritySourceUnprotected,
    AuthoritySourceMalformed,
    EmptyPolicy,
    InvalidAuthorization,
    InvalidBatchRange,
    InvalidRevocation,
    AmbiguousAuthorization,
    HeaderDecode,
    ProtocolVersion,
    Network,
    UnknownSequencer,
    Epoch,
    BatchRange,
    Revoked,
    HandshakeKey,
}

/// Opaque daemon boot authority retained only after trusted config and handshake agree.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct EvidenceAuthority {
    verifier: ProtocolEvidenceVerifier,
    receipt_keys: Vec<layerx_client::sequencer::ReceiptKeyEntry>,
    committed_validators: Option<(String, Vec<crate::read::CommitValidator>)>,
}

impl EvidenceAuthority {
    pub(crate) const fn new(verifier: ProtocolEvidenceVerifier) -> Self {
        Self {
            verifier,
            receipt_keys: Vec::new(),
            committed_validators: None,
        }
    }

    pub(crate) const fn verifier(&self) -> &ProtocolEvidenceVerifier {
        &self.verifier
    }

    /// Builds the authority a daemon holds when its only sequencer trust anchor
    /// is the key authenticated by the node handshake.
    ///
    /// # Errors
    ///
    /// Refuses a zero protocol version, network identity or key.
    pub fn pinned_to_handshake(
        expected_protocol_version: u16,
        expected_network_id: u32,
        handshake_sequencer_key: [u8; 32],
    ) -> Result<Self, VerifierPolicyError> {
        ProtocolEvidenceVerifier::pinned(
            expected_protocol_version,
            expected_network_id,
            handshake_sequencer_key,
        )
        .map(Self::new)
    }

    /// Verifies raw receipt ingress under the daemon's accepted startup authority.
    ///
    /// # Errors
    ///
    /// Refuses every policy, signature, inclusion, sequence, or batch-identity mismatch.
    pub fn verify_receipt(
        &self,
        raw: &RawReceiptEvidence,
    ) -> Result<VerifiedReceiptEvidence, ReceiptEvidenceError> {
        self.verifier.verify_receipt(raw)
    }

    /// Installs governed node receipt keys obtained from the trusted node RPC.
    pub fn with_receipt_keys(
        mut self,
        keys: Vec<layerx_client::sequencer::ReceiptKeyEntry>,
    ) -> Result<Self, VerifierPolicyError> {
        let mut identities = BTreeSet::new();
        if keys.is_empty()
            || keys.iter().any(|key| {
                key.key_id == [0; 32]
                    || key.public_key == [0; 32]
                    || key.first_height > key.last_height
                    || !identities.insert(key.key_id)
            })
        {
            return Err(VerifierPolicyError::InvalidAuthorization);
        }
        self.receipt_keys = keys;
        Ok(self)
    }

    /// Re-verifies a persisted current receipt without inventing batch evidence.
    pub fn verify_current_receipt(
        &self,
        raw: &crate::receipt::TransactionReceiptEvidence,
    ) -> Result<VerifiedSpendReceipt, ReceiptEvidenceError> {
        let keys: Vec<_> = self.receipt_keys.iter().map(select_receipt_key).collect();
        let decoded = verify_sequencer_signature(&raw.canonical_receipt, keys.as_slice())
            .map_err(ReceiptEvidenceError::Receipt)?;
        let protocol =
            decoded
                .protocol()
                .ok_or(ReceiptEvidenceError::Receipt(VerificationFailure {
                    check: ReceiptCheck::ReceiptShape,
                }))?;
        if protocol.protocol_version() != self.verifier.expected_protocol_version {
            return Err(ReceiptEvidenceError::ProtocolVersion);
        }
        let mut height = [0; 32];
        height[24..].copy_from_slice(&raw.metadata.block_height.to_be_bytes());
        let signature = protocol
            .sequencer_signature()
            .ok_or(ReceiptEvidenceError::Receipt(VerificationFailure {
                check: ReceiptCheck::MissingSignature,
            }))?;
        if protocol.batch_id() != height
            || protocol.activity_id() != raw.metadata.transaction_hash
            || (protocol.result_code() == 0) != raw.metadata.status
            || raw.metadata.receipt_signature[..64] != signature
            || raw.metadata.receipt_signature[64] != crate::receipt::NODE_ED25519_SIGNATURE_TAG
            || raw.metadata.rung < layerx_types::rung::Rung::Receipted
        {
            return Err(ReceiptEvidenceError::Receipt(VerificationFailure {
                check: ReceiptCheck::ReceiptShape,
            }));
        }
        Ok(VerifiedSpendReceipt {
            receipt_ref: Sha256::digest(&raw.canonical_receipt).into(),
            activity_id: protocol.activity_id(),
            global_sequence: protocol.global_sequence(),
            result_code: protocol.result_code(),
            amount: protocol.amount(),
            module_id: protocol.module_id(),
            operation: protocol.operation(),
        })
    }

    /// Verifies raw receipt ingress for offline export and retains the
    /// authorised batch, inclusion proof and signed header it was verified under.
    ///
    /// # Errors
    ///
    /// Refuses every policy, signature, inclusion, sequence, or batch-identity mismatch.
    pub fn verify_receipt_for_export(
        &self,
        raw: &RawReceiptEvidence,
    ) -> Result<ExportReceiptEvidence, ReceiptEvidenceError> {
        let (verified, authorised) = self.verifier.verify_receipt_retaining(raw)?;
        Ok(ExportReceiptEvidence {
            verified,
            authorised,
            raw: raw.clone(),
        })
    }

    /// Verifies raw state ingress under the daemon's accepted startup authority.
    ///
    /// # Errors
    ///
    /// Refuses every policy, signature, root, or inclusion mismatch.
    pub fn verify_state(
        &self,
        raw: &RawStateEvidence,
    ) -> Result<VerifiedStateEvidence, StateEvidenceError> {
        self.verifier.verify_state(raw)
    }

    fn verify_cumulative_window(
        &self,
        window: CumulativeUseWindow,
        entries: &[RawActivityReceiptEvidence],
    ) -> Result<Vec<VerifiedCumulativeReceipt>, CumulativeUseError> {
        let length = window
            .last
            .checked_sub(window.first)
            .and_then(|distance| distance.checked_add(1))
            .and_then(|length| usize::try_from(length).ok())
            .ok_or(CumulativeUseError::InvalidWindow)?;
        if length == 0 || length > MAX_CUMULATIVE_EVIDENCE_ITEMS {
            return Err(CumulativeUseError::InvalidWindow);
        }
        if entries.is_empty() || entries.len() > length {
            return Err(CumulativeUseError::IncompleteWindow);
        }
        let mut receipt_refs = BTreeSet::new();
        let mut activity_ids = BTreeSet::new();
        let mut previous_header: Option<BatchHeader> = None;
        let mut current_header: Option<BatchHeader> = None;
        let mut expected_leaf = 0_u32;
        let mut activity_count = 0_u32;
        let mut covered_until = window.first;
        let mut verified = Vec::with_capacity(entries.len());
        for entry in entries {
            let receipt = match &entry.receipt {
                CumulativeReceiptEvidence::Outcome(raw) => self
                    .verify_receipt(raw)
                    .map(|value| VerifiedCumulativeReceipt::from_verified(&value, raw.proof()))
                    .map_err(CumulativeUseError::Receipt)?,
                CumulativeReceiptEvidence::SignedInclusion(raw) => self
                    .verifier
                    .verify_signed_receipt_inclusion(raw, &self.receipt_keys)
                    .map_err(CumulativeUseError::Receipt)?,
            };
            let receipt_ref: [u8; 32] = Sha256::digest(entry.receipt().canonical_receipt()).into();
            if !receipt_refs.insert(receipt_ref) || !activity_ids.insert(receipt.activity_id()) {
                return Err(CumulativeUseError::Duplicate);
            }
            let header = receipt.header();
            if current_header.as_ref() != Some(header) {
                if expected_leaf != activity_count || header.first_sequence() != covered_until {
                    return Err(CumulativeUseError::IncompleteWindow);
                }
                if previous_header.as_ref().is_some_and(|previous| {
                    previous.batch_number().checked_add(1) != Some(header.batch_number())
                        || header.previous_state_root() != previous.resulting_state_root()
                }) {
                    return Err(CumulativeUseError::BatchChain);
                }
                activity_count = header
                    .last_sequence()
                    .checked_sub(header.first_sequence())
                    .and_then(|value| u32::try_from(value).ok())
                    .filter(|value| *value > 0)
                    .ok_or(CumulativeUseError::Sequence)?;
                expected_leaf = 0;
                current_header = Some(header.clone());
            }
            let expected_leaf_count = activity_count
                .checked_add(1)
                .ok_or(CumulativeUseError::InvalidWindow)?;
            if receipt.leaf_count() != expected_leaf_count
                || receipt.leaf_index() != expected_leaf
                || receipt.global_sequence()
                    != header
                        .first_sequence()
                        .checked_add(u64::from(expected_leaf))
                        .ok_or(CumulativeUseError::Sequence)?
            {
                return Err(CumulativeUseError::IncompleteWindow);
            }
            expected_leaf = expected_leaf
                .checked_add(1)
                .ok_or(CumulativeUseError::InvalidWindow)?;
            if expected_leaf == activity_count {
                covered_until = header
                    .last_sequence()
                    .checked_add(1)
                    .ok_or(CumulativeUseError::InvalidWindow)?;
                previous_header = Some(header.clone());
            }
            verified.push(receipt);
        }
        if expected_leaf != activity_count
            || covered_until
                != window
                    .last
                    .checked_add(1)
                    .ok_or(CumulativeUseError::InvalidWindow)?
        {
            return Err(CumulativeUseError::IncompleteWindow);
        }
        Ok(verified)
    }

    /// Authenticates a complete protocol-sequence window and derives cumulative
    /// successful activity use for one actor from the exact executed activities.
    ///
    /// # Errors
    ///
    /// Refuses empty, oversized, incomplete, duplicated, noncanonical, forked,
    /// or receipt/activity-mismatched windows before issuing cumulative facts.
    pub fn authenticate_cumulative_use(
        &self,
        actor: &Did,
        window: CumulativeUseWindow,
        entries: &[RawActivityReceiptEvidence],
    ) -> Result<AuthenticatedCumulativeUse, CumulativeUseError> {
        self.authenticate_cumulative_use_bound(actor, None, window, entries)
    }

    /// Authenticates cumulative use bound to the actor DID or the supplied EVM account.
    ///
    /// # Errors
    /// Refuses invalid evidence and incomplete or mismatched activity windows.
    pub fn authenticate_account_cumulative_use(
        &self,
        actor: &Did,
        account: [u8; 20],
        window: CumulativeUseWindow,
        entries: &[RawActivityReceiptEvidence],
    ) -> Result<AuthenticatedCumulativeUse, CumulativeUseError> {
        self.authenticate_cumulative_use_bound(actor, Some(account), window, entries)
    }

    fn authenticate_cumulative_use_bound(
        &self,
        actor: &Did,
        account: Option<[u8; 20]>,
        window: CumulativeUseWindow,
        entries: &[RawActivityReceiptEvidence],
    ) -> Result<AuthenticatedCumulativeUse, CumulativeUseError> {
        let receipts = self.verify_cumulative_window(window, entries)?;
        let mut amount = 0_u128;
        let mut count = 0_u64;
        for (entry, receipt) in entries.iter().zip(receipts) {
            let receipt_module = if receipt.module_id() > KERNEL_MODULE_COUNT {
                None
            } else {
                Some(
                    ModuleId::from_u16(receipt.module_id())
                        .map_err(|_| CumulativeUseError::Activity)?,
                )
            };
            let activity = decode_use_activity(entry.canonical_activity(), receipt_module)
                .map_err(|error| match error {
                    UseActivityError::Activity => CumulativeUseError::Activity,
                    UseActivityError::Identity => CumulativeUseError::ActivityReceiptMismatch,
                })?;
            if activity.id != receipt.activity_id() {
                return Err(CumulativeUseError::ActivityReceiptMismatch);
            }
            let matches_actor = match activity.actor {
                ActivityActorBinding::Did(did) => did == actor.as_bytes(),
                ActivityActorBinding::Account(sender) => {
                    sender == account.ok_or(CumulativeUseError::AccountBinding)?
                }
            };
            if receipt.result_code() == 0 && matches_actor {
                amount = amount
                    .checked_add(receipt.amount())
                    .ok_or(CumulativeUseError::AmountOverflow)?;
                count = count
                    .checked_add(1)
                    .ok_or(CumulativeUseError::CountOverflow)?;
            }
        }
        Ok(AuthenticatedCumulativeUse {
            actor: actor.clone(),
            window,
            amount,
            count,
        })
    }

    pub(crate) fn receipt_replay_guard() -> ReceiptReplayGuard {
        ReceiptReplayGuard::default()
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct RawTimeWindowBatch<A = RawActivityReceiptEvidence, M = RawReceiptEvidence> {
    activities: Vec<A>,
    maintenance: M,
}

impl RawTimeWindowBatch {
    #[must_use]
    pub fn new(
        activities: Vec<RawActivityReceiptEvidence>,
        maintenance: RawReceiptEvidence,
    ) -> Self {
        Self {
            activities,
            maintenance,
        }
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct AuthenticatedCoreTime {
    checkpoint: crate::read::ServedCheckpoint,
    timestamp_ms: u64,
}

impl AuthenticatedCoreTime {
    #[must_use]
    pub const fn observed_core_ms(&self) -> u64 {
        self.timestamp_ms
    }
    #[must_use]
    pub const fn observed_batch_id(&self) -> [u8; 32] {
        self.checkpoint.block_hash
    }
    #[must_use]
    pub const fn through_sequence(&self) -> u64 {
        self.checkpoint.height
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct AuthenticatedReceiptAmount {
    asset: [u8; 32],
    source_account: [u8; 32],
    amount: u128,
}

impl AuthenticatedReceiptAmount {
    #[must_use]
    pub const fn asset(&self) -> [u8; 32] {
        self.asset
    }
    #[must_use]
    pub const fn source_account(&self) -> [u8; 32] {
        self.source_account
    }
    #[must_use]
    pub const fn amount(&self) -> u128 {
        self.amount
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct AuthenticatedTimeWindowUse {
    actor: Did,
    observed: AuthenticatedCoreTime,
    window_seconds: u64,
    first_covered_sequence: u64,
    count: u64,
    successful_count: u64,
    activity_ids: Vec<[u8; 32]>,
    receipt_amounts: Vec<AuthenticatedReceiptAmount>,
}

impl AuthenticatedTimeWindowUse {
    #[must_use]
    pub const fn actor(&self) -> &Did {
        &self.actor
    }
    #[must_use]
    pub const fn observed_core_ms(&self) -> u64 {
        self.observed.observed_core_ms()
    }
    #[must_use]
    pub const fn observed_batch_id(&self) -> [u8; 32] {
        self.observed.observed_batch_id()
    }
    #[must_use]
    pub const fn through_sequence(&self) -> u64 {
        self.observed.through_sequence()
    }
    #[must_use]
    pub const fn first_covered_sequence(&self) -> u64 {
        self.first_covered_sequence
    }
    #[must_use]
    pub const fn window_seconds(&self) -> u64 {
        self.window_seconds
    }
    #[must_use]
    pub const fn count(&self) -> u64 {
        self.count
    }
    #[must_use]
    pub const fn successful_count(&self) -> u64 {
        self.successful_count
    }
    #[must_use]
    pub fn receipt_amounts(&self) -> &[AuthenticatedReceiptAmount] {
        &self.receipt_amounts
    }
    #[must_use]
    pub fn activity_ids(&self) -> &[[u8; 32]] {
        &self.activity_ids
    }
}

#[derive(Debug)]
pub enum TimeWindowUseError {
    Window,
    Bounds,
    Observation,
    Coverage,
    Chain,
    Maintenance,
    Receipt(ReceiptEvidenceError),
    Inclusion(InclusionError),
    Policy(VerifierPolicyError),
    Activity,
    AccountBinding,
    Identity,
    Duplicate,
    Overflow,
}

impl EvidenceAuthority {
    pub fn pin_committed_validators(
        mut self,
        chain_id: String,
        validators: Vec<crate::read::CommitValidator>,
    ) -> Result<Self, TimeWindowUseError> {
        let mut addresses = BTreeSet::new();
        if chain_id.is_empty()
            || chain_id.len() > 50
            || validators.is_empty()
            || validators.iter().any(|v| {
                v.voting_power == 0
                    || v.address != crate::read::CommitValidator::address_for_key(&v.public_key)
                    || !addresses.insert(v.address)
            })
        {
            return Err(TimeWindowUseError::Observation);
        }
        self.committed_validators = Some((chain_id, validators));
        Ok(self)
    }

    pub fn authenticate_committed_time(
        &self,
        checkpoint: &crate::read::ServedCheckpoint,
    ) -> Result<AuthenticatedCoreTime, TimeWindowUseError> {
        let (chain_id, validators) = self
            .committed_validators
            .as_ref()
            .ok_or(TimeWindowUseError::Observation)?;
        if *chain_id != self.verifier.expected_network_id.to_string() {
            return Err(TimeWindowUseError::Observation);
        }
        let verified = crate::read::checkpoint(
            checkpoint.height,
            chain_id,
            &checkpoint.header,
            &checkpoint.commit,
            validators,
        )
        .map_err(|_| TimeWindowUseError::Observation)?;
        if verified != *checkpoint || checkpoint.height == u64::MAX {
            return Err(TimeWindowUseError::Observation);
        }
        let timestamp_ms = u64::try_from(checkpoint.header.time.seconds)
            .ok()
            .and_then(|seconds| seconds.checked_mul(1_000))
            .and_then(|ms| ms.checked_add(u64::from(checkpoint.header.time.nanos / 1_000_000)))
            .filter(|ms| *ms > 0)
            .ok_or(TimeWindowUseError::Observation)?;
        Ok(AuthenticatedCoreTime {
            checkpoint: verified,
            timestamp_ms,
        })
    }

    pub fn authenticate_cumulative_time_use(
        &self,
        actor: &Did,
        window_seconds: u64,
        observed: &AuthenticatedCoreTime,
        batches: &[RawTimeWindowBatch],
    ) -> Result<AuthenticatedTimeWindowUse, TimeWindowUseError> {
        self.authenticate_cumulative_time_use_bound(actor, None, window_seconds, observed, batches)
    }

    /// Authenticates time-window use bound to the actor DID or supplied EVM account.
    ///
    /// # Errors
    /// Refuses invalid observations, incomplete coverage and mismatched activity evidence.
    pub fn authenticate_account_cumulative_time_use(
        &self,
        actor: &Did,
        account: [u8; 20],
        window_seconds: u64,
        observed: &AuthenticatedCoreTime,
        batches: &[RawTimeWindowBatch],
    ) -> Result<AuthenticatedTimeWindowUse, TimeWindowUseError> {
        self.authenticate_cumulative_time_use_bound(
            actor,
            Some(account),
            window_seconds,
            observed,
            batches,
        )
    }

    fn authenticate_cumulative_time_use_bound(
        &self,
        actor: &Did,
        account: Option<[u8; 20]>,
        window_seconds: u64,
        observed: &AuthenticatedCoreTime,
        batches: &[RawTimeWindowBatch],
    ) -> Result<AuthenticatedTimeWindowUse, TimeWindowUseError> {
        let window_ms = window_seconds
            .checked_mul(1_000)
            .filter(|value| *value > 0)
            .ok_or(TimeWindowUseError::Window)?;
        if batches.is_empty() || batches.len() > MAX_CUMULATIVE_EVIDENCE_ITEMS {
            return Err(TimeWindowUseError::Bounds);
        }
        let pinned = self.authenticate_committed_time(&observed.checkpoint)?;
        if pinned != *observed {
            return Err(TimeWindowUseError::Observation);
        }
        let lower = observed.observed_core_ms().checked_sub(window_ms);
        let mut previous: Option<BatchHeader> = None;
        let mut first_covered_sequence = 0;
        let mut covered = 0_usize;
        let mut bytes = 0_usize;
        let mut seen = BTreeSet::new();
        let mut count = 0_u64;
        let mut successful_count = 0_u64;
        let mut activity_ids = Vec::new();
        let mut amounts = std::collections::BTreeMap::<([u8; 32], [u8; 32]), u128>::new();
        for (position, batch) in batches.iter().enumerate() {
            let raw = &batch.maintenance;
            covered = covered
                .checked_add(batch.activities.len())
                .and_then(|n| n.checked_add(1))
                .filter(|n| *n <= MAX_CUMULATIVE_EVIDENCE_ITEMS)
                .ok_or(TimeWindowUseError::Bounds)?;
            for receipt in std::iter::once(raw).chain(
                batch
                    .activities
                    .iter()
                    .map(RawActivityReceiptEvidence::receipt),
            ) {
                bytes = bytes
                    .checked_add(receipt.canonical_receipt.len())
                    .and_then(|n| n.checked_add(receipt.canonical_header.len()))
                    .filter(|n| *n <= 16 * 1_024 * 1_024)
                    .ok_or(TimeWindowUseError::Bounds)?;
            }
            for activity in &batch.activities {
                bytes = bytes
                    .checked_add(activity.canonical_activity.len())
                    .filter(|n| *n <= 16 * 1_024 * 1_024)
                    .ok_or(TimeWindowUseError::Bounds)?;
            }
            let (header, authorization) = self
                .verifier
                .authorization_for(raw.canonical_header())
                .map_err(TimeWindowUseError::Policy)?;
            let count_a = header
                .last_sequence()
                .checked_sub(header.first_sequence())
                .and_then(|n| u32::try_from(n).ok())
                .ok_or(TimeWindowUseError::Coverage)?;
            let count_all = count_a.checked_add(1).ok_or(TimeWindowUseError::Bounds)?;
            if header.first_sequence() == 0
                || header.timestamp_ms() == 0
                || batch.activities.len()
                    != usize::try_from(count_a).map_err(|_| TimeWindowUseError::Bounds)?
                || raw.proof.leaf_index() != count_a
                || raw.proof.leaf_count() != count_all
            {
                return Err(TimeWindowUseError::Coverage);
            }
            verify_receipt_inclusion(
                raw.canonical_receipt(),
                raw.proof(),
                raw.canonical_header(),
                &raw.header_signature(),
                &authorization,
            )
            .map_err(TimeWindowUseError::Inclusion)?;
            let maintenance =
                layerx_wire::batch_maintenance::decode_maintenance(raw.canonical_receipt())
                    .map_err(|_| TimeWindowUseError::Maintenance)?;
            maintenance
                .verify_header(&header)
                .map_err(|_| TimeWindowUseError::Maintenance)?;
            if position == 0 {
                first_covered_sequence = header.first_sequence();
                let genesis = header.batch_number() == 1 && header.first_sequence() == 1;
                if !genesis && !lower.is_some_and(|bound| header.timestamp_ms() <= bound) {
                    return Err(TimeWindowUseError::Coverage);
                }
            }
            if previous.as_ref().is_some_and(|prior| {
                prior.batch_number().checked_add(1) != Some(header.batch_number())
                    || prior.last_sequence().checked_add(1) != Some(header.first_sequence())
                    || prior.resulting_state_root() != header.previous_state_root()
                    || prior.timestamp_ms() >= header.timestamp_ms()
            }) {
                return Err(TimeWindowUseError::Chain);
            }
            if header.timestamp_ms() > observed.observed_core_ms() {
                return Err(TimeWindowUseError::Observation);
            }
            for (index, entry) in batch.activities.iter().enumerate() {
                let receipt_raw = entry.receipt();
                if receipt_raw.canonical_header() != raw.canonical_header()
                    || receipt_raw.header_signature() != raw.header_signature()
                    || receipt_raw.proof.leaf_index()
                        != u32::try_from(index).map_err(|_| TimeWindowUseError::Bounds)?
                    || receipt_raw.proof.leaf_count() != count_all
                {
                    return Err(TimeWindowUseError::Coverage);
                }
                let receipt = self
                    .verifier
                    .verify_signed_receipt_inclusion(receipt_raw, &self.receipt_keys)
                    .map_err(TimeWindowUseError::Receipt)?;
                let decoded = decode(receipt_raw.canonical_receipt())
                    .map_err(|_| TimeWindowUseError::Identity)?;
                let protocol = decoded.protocol().ok_or(TimeWindowUseError::Identity)?;
                let batch_id = layerx_wire::hash::receipt_execution_batch_id_maintenance(
                    protocol,
                    &header,
                    maintenance.occupancy(),
                    count_a,
                )
                .map_err(|_| TimeWindowUseError::Identity)?;
                let batch_id = if receipt.module_id() > KERNEL_MODULE_COUNT {
                    let mut height = [0; 32];
                    height[24..].copy_from_slice(&header.batch_number().to_be_bytes());
                    height
                } else {
                    batch_id
                };
                if protocol.batch_id() != batch_id
                    || protocol.timestamp() != header.timestamp_ms()
                    || header
                        .first_sequence()
                        .checked_add(u64::try_from(index).map_err(|_| TimeWindowUseError::Bounds)?)
                        != Some(receipt.global_sequence())
                {
                    return Err(TimeWindowUseError::Identity);
                }
                if !seen.insert(receipt.activity_id()) {
                    return Err(TimeWindowUseError::Duplicate);
                }
                let receipt_module = if receipt.module_id() > KERNEL_MODULE_COUNT {
                    None
                } else {
                    Some(
                        ModuleId::from_u16(receipt.module_id())
                            .map_err(|_| TimeWindowUseError::Activity)?,
                    )
                };
                let activity = decode_use_activity(entry.canonical_activity(), receipt_module)
                    .map_err(|error| match error {
                        UseActivityError::Activity => TimeWindowUseError::Activity,
                        UseActivityError::Identity => TimeWindowUseError::Identity,
                    })?;
                if activity.id != receipt.activity_id()
                    || activity.envelope_context.is_some_and(|context| {
                        context != (header.protocol_version(), header.network_id())
                    })
                {
                    return Err(TimeWindowUseError::Identity);
                }
                let matches_actor = match activity.actor {
                    ActivityActorBinding::Did(did) => did == actor.as_bytes(),
                    ActivityActorBinding::Account(sender) => {
                        sender == account.ok_or(TimeWindowUseError::AccountBinding)?
                    }
                };
                if matches_actor && lower.is_none_or(|bound| header.timestamp_ms() > bound) {
                    activity_ids.push(receipt.activity_id());
                    count = count.checked_add(1).ok_or(TimeWindowUseError::Overflow)?;
                    if receipt.result_code() == 0 {
                        successful_count = successful_count
                            .checked_add(1)
                            .ok_or(TimeWindowUseError::Overflow)?;
                        let amount = amounts
                            .entry((protocol.asset(), protocol.from()))
                            .or_default();
                        *amount = amount
                            .checked_add(protocol.amount())
                            .ok_or(TimeWindowUseError::Overflow)?;
                    }
                }
            }
            if position + 1 == batches.len()
                && (header.last_sequence() != observed.through_sequence()
                    || header.resulting_state_root() != observed.checkpoint.app_hash)
            {
                return Err(TimeWindowUseError::Observation);
            }
            previous = Some(header);
        }
        Ok(AuthenticatedTimeWindowUse {
            actor: actor.clone(),
            observed: observed.clone(),
            window_seconds,
            first_covered_sequence,
            count,
            successful_count,
            activity_ids,
            receipt_amounts: amounts
                .into_iter()
                .map(
                    |((asset, source_account), amount)| AuthenticatedReceiptAmount {
                        asset,
                        source_account,
                        amount,
                    },
                )
                .collect(),
        })
    }
}

#[derive(Clone, Debug, Default, Eq, PartialEq)]
pub(crate) struct ReceiptReplayGuard {
    receipt_refs: BTreeSet<[u8; 32]>,
    activity_ids: BTreeSet<[u8; 32]>,
}

impl ReceiptReplayGuard {
    pub(crate) fn admit(
        &mut self,
        receipt: &VerifiedReceiptEvidence,
    ) -> Result<(), ReceiptReplayError> {
        self.admit_identity(receipt.receipt_ref(), receipt.activity_id())
    }

    pub(crate) fn admit_identity(
        &mut self,
        receipt_ref: [u8; 32],
        activity_id: [u8; 32],
    ) -> Result<(), ReceiptReplayError> {
        if self.receipt_refs.contains(&receipt_ref) {
            return Err(ReceiptReplayError::DuplicateReceipt);
        }
        if self.activity_ids.contains(&activity_id) {
            return Err(ReceiptReplayError::DuplicateActivity);
        }
        self.receipt_refs.insert(receipt_ref);
        self.activity_ids.insert(activity_id);
        Ok(())
    }
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub(crate) enum ReceiptReplayError {
    DuplicateReceipt,
    DuplicateActivity,
}

/// Raw receipt bytes, Merkle path, and signature returned by a core boundary.
///
/// Trusted keys, ranges, network identity, and protocol identity are deliberately
/// absent. They come only from `ProtocolEvidenceVerifier` configuration.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct RawReceiptEvidence {
    canonical_receipt: Vec<u8>,
    proof: Proof,
    canonical_header: Vec<u8>,
    header_signature: [u8; 64],
}

impl RawReceiptEvidence {
    #[must_use]
    pub fn new(
        canonical_receipt: Vec<u8>,
        proof: Proof,
        canonical_header: Vec<u8>,
        header_signature: [u8; 64],
    ) -> Self {
        Self {
            canonical_receipt,
            proof,
            canonical_header,
            header_signature,
        }
    }

    #[must_use]
    pub fn canonical_receipt(&self) -> &[u8] {
        &self.canonical_receipt
    }

    #[must_use]
    pub const fn proof(&self) -> &Proof {
        &self.proof
    }

    #[must_use]
    pub fn canonical_header(&self) -> &[u8] {
        &self.canonical_header
    }

    #[must_use]
    pub const fn header_signature(&self) -> [u8; 64] {
        self.header_signature
    }
}

/// Exact verification stage which refused raw receipt evidence.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum ReceiptEvidenceError {
    Policy(VerifierPolicyError),
    Inclusion(InclusionError),
    Receipt(VerificationFailure),
    ProtocolVersion,
    SequenceRange,
    BatchIdentity,
}

/// Spend facts issued only after receipt authority verification.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct VerifiedSpendReceipt {
    receipt_ref: [u8; 32],
    activity_id: [u8; 32],
    global_sequence: u64,
    result_code: i32,
    amount: u128,
    module_id: u16,
    operation: u8,
}

impl VerifiedSpendReceipt {
    pub const fn receipt_ref(&self) -> [u8; 32] {
        self.receipt_ref
    }
    pub const fn activity_id(&self) -> [u8; 32] {
        self.activity_id
    }
    pub const fn global_sequence(&self) -> u64 {
        self.global_sequence
    }
    pub const fn result_code(&self) -> i32 {
        self.result_code
    }
    pub const fn amount(&self) -> u128 {
        self.amount
    }
    pub const fn module_id(&self) -> u16 {
        self.module_id
    }
    pub const fn operation(&self) -> u8 {
        self.operation
    }
}

impl From<VerifiedReceiptEvidence> for VerifiedSpendReceipt {
    fn from(receipt: VerifiedReceiptEvidence) -> Self {
        Self {
            receipt_ref: receipt.receipt_ref(),
            activity_id: receipt.activity_id(),
            global_sequence: receipt.global_sequence(),
            result_code: receipt.result_code(),
            amount: receipt.amount(),
            module_id: receipt.module_id(),
            operation: receipt.operation(),
        }
    }
}

/// Inclusive global-sequence window covered by cumulative-use evidence.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct CumulativeUseWindow {
    pub first: u64,
    pub last: u64,
}

/// One exact signed activity paired with its independently authenticated receipt.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct RawActivityReceiptEvidence<R = RawReceiptEvidence, A = Vec<u8>> {
    canonical_activity: A,
    receipt: CumulativeReceiptEvidence<R>,
}

#[derive(Clone, Debug, Eq, PartialEq)]
enum CumulativeReceiptEvidence<R = RawReceiptEvidence> {
    Outcome(R),
    SignedInclusion(R),
}

impl RawActivityReceiptEvidence {
    #[must_use]
    pub fn new(canonical_activity: Vec<u8>, receipt: RawReceiptEvidence) -> Self {
        Self {
            canonical_activity,
            receipt: CumulativeReceiptEvidence::Outcome(receipt),
        }
    }

    /// Admits the gateway proof shape: exact sequencer-signed receipt bytes
    /// included in an independently authenticated signed batch header.
    #[must_use]
    pub fn from_signed_inclusion(canonical_activity: Vec<u8>, receipt: RawReceiptEvidence) -> Self {
        Self {
            canonical_activity,
            receipt: CumulativeReceiptEvidence::SignedInclusion(receipt),
        }
    }

    #[must_use]
    pub fn canonical_activity(&self) -> &[u8] {
        &self.canonical_activity
    }

    #[must_use]
    pub const fn receipt(&self) -> &RawReceiptEvidence {
        match &self.receipt {
            CumulativeReceiptEvidence::Outcome(receipt)
            | CumulativeReceiptEvidence::SignedInclusion(receipt) => receipt,
        }
    }
}

struct VerifiedCumulativeReceipt {
    activity_id: [u8; 32],
    global_sequence: u64,
    result_code: i32,
    amount: u128,
    module_id: u16,
    operation: u8,
    header: BatchHeader,
    leaf_index: u32,
    leaf_count: u32,
}

impl VerifiedCumulativeReceipt {
    fn from_verified(receipt: &VerifiedReceiptEvidence, proof: &Proof) -> Self {
        let header = receipt.inclusion.header().header().clone();
        Self {
            activity_id: receipt.activity_id(),
            global_sequence: receipt.global_sequence(),
            result_code: receipt.result_code(),
            amount: receipt.amount(),
            module_id: receipt.module_id(),
            operation: receipt.operation(),
            header,
            leaf_index: proof.leaf_index(),
            leaf_count: proof.leaf_count(),
        }
    }

    fn from_protocol(
        protocol: &layerx_wire::receipt::ProtocolReceipt,
        header: BatchHeader,
        proof: &Proof,
    ) -> Self {
        Self {
            activity_id: protocol.activity_id(),
            global_sequence: protocol.global_sequence(),
            result_code: protocol.result_code(),
            amount: protocol.amount(),
            module_id: protocol.module_id(),
            operation: protocol.operation(),
            header,
            leaf_index: proof.leaf_index(),
            leaf_count: proof.leaf_count(),
        }
    }

    const fn activity_id(&self) -> [u8; 32] {
        self.activity_id
    }

    const fn global_sequence(&self) -> u64 {
        self.global_sequence
    }

    const fn result_code(&self) -> i32 {
        self.result_code
    }

    const fn amount(&self) -> u128 {
        self.amount
    }

    const fn module_id(&self) -> u16 {
        self.module_id
    }

    const fn operation(&self) -> u8 {
        self.operation
    }

    const fn header(&self) -> &BatchHeader {
        &self.header
    }

    const fn leaf_index(&self) -> u32 {
        self.leaf_index
    }

    const fn leaf_count(&self) -> u32 {
        self.leaf_count
    }
}

/// Opaque cumulative facts issued only for a complete authenticated window.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct AuthenticatedCumulativeUse {
    actor: Did,
    window: CumulativeUseWindow,
    amount: u128,
    count: u64,
}

impl AuthenticatedCumulativeUse {
    pub(crate) const fn actor(&self) -> &Did {
        &self.actor
    }

    pub(crate) const fn window(&self) -> CumulativeUseWindow {
        self.window
    }

    pub const fn amount(&self) -> u128 {
        self.amount
    }

    pub const fn count(&self) -> u64 {
        self.count
    }
}

/// Exact refusal while producing authenticated cumulative-use facts.
#[derive(Debug)]
pub enum CumulativeUseError {
    InvalidWindow,
    IncompleteWindow,
    Receipt(ReceiptEvidenceError),
    Duplicate,
    Sequence,
    Activity,
    AccountBinding,
    ActivityReceiptMismatch,
    BatchChain,
    AmountOverflow,
    CountOverflow,
}

/// Receipt facts available only after configured policy and canonical proof verification.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct VerifiedReceiptEvidence {
    verified: ProofVerifiedReceipt,
    inclusion: InclusionEvidence,
    receipt_ref: [u8; 32],
    activity_id: [u8; 32],
    global_sequence: u64,
    result_code: i32,
    amount: u128,
    module_id: u16,
    operation: u8,
}

impl VerifiedReceiptEvidence {
    pub(crate) const fn execution_receipt(&self) -> &ProofVerifiedReceipt {
        &self.verified
    }

    /// Verifies an outcome against the complete authenticated native batch transition.
    ///
    /// # Errors
    /// Refuses incomplete history, altered maintenance, roots, signatures or network binding.
    pub fn verify_authorized_maintained(
        raw: &RawReceiptEvidence,
        activity_batch: &AuthorizedBatch,
        evidence: &layerx_proof::receipt::MaintainedOutcomeEvidence<'_>,
        receipts: &[Vec<u8>],
        protocol_version: u16,
        network_id: u32,
    ) -> Result<Self, ReceiptEvidenceError> {
        let header = decode_batch_header(raw.canonical_header())
            .map_err(|_| ReceiptEvidenceError::Policy(VerifierPolicyError::HeaderDecode))?;
        if header.protocol_version() != protocol_version
            || header.network_id() != network_id
            || evidence.header != raw.canonical_header()
            || evidence.header_signature != &raw.header_signature
            || evidence.activity_proof != &raw.proof
            || evidence.authorization.public_key() != activity_batch.sequencer_public_key()
        {
            return Err(ReceiptEvidenceError::BatchIdentity);
        }
        let sealed_batch = AuthorizedBatch::new(
            activity_batch.batch_id(),
            activity_batch.asset(),
            header.previous_state_root(),
            header.resulting_state_root(),
            activity_batch.sequencer_public_key(),
        );
        let authenticated = layerx_proof::receipt::authorized_maintained_activity_batch_chain(
            &raw.canonical_receipt,
            &sealed_batch,
            evidence,
            receipts,
        )
        .map_err(|_| ReceiptEvidenceError::BatchIdentity)?;
        if authenticated != *activity_batch {
            return Err(ReceiptEvidenceError::BatchIdentity);
        }
        let verified = verify_outcome(&raw.canonical_receipt, &authenticated)
            .map_err(ReceiptEvidenceError::Receipt)?;
        let inclusion = verify_receipt_inclusion(
            &raw.canonical_receipt,
            &raw.proof,
            &raw.canonical_header,
            &raw.header_signature,
            evidence.authorization,
        )
        .map_err(ReceiptEvidenceError::Inclusion)?;
        let protocol = verified
            .receipt()
            .protocol()
            .ok_or(ReceiptEvidenceError::BatchIdentity)?;
        Ok(Self {
            receipt_ref: Sha256::digest(verified.canonical_bytes()).into(),
            activity_id: protocol.activity_id(),
            global_sequence: protocol.global_sequence(),
            result_code: protocol.result_code(),
            amount: protocol.amount(),
            module_id: protocol.module_id(),
            operation: protocol.operation(),
            verified,
            inclusion,
        })
    }
    pub(crate) fn verify_authorized(
        raw: &RawReceiptEvidence,
        batch: &AuthorizedBatch,
        protocol_version: u16,
        network_id: u32,
    ) -> Result<Self, ReceiptEvidenceError> {
        let header = decode_batch_header(raw.canonical_header())
            .map_err(|_| ReceiptEvidenceError::Policy(VerifierPolicyError::HeaderDecode))?;
        if header.previous_state_root() != batch.previous_state_root()
            || header.resulting_state_root() != batch.resulting_state_root()
        {
            return Err(ReceiptEvidenceError::BatchIdentity);
        }
        let sequencer = TrustedSequencer::new(
            header.sequencer_id(),
            batch.sequencer_public_key(),
            header.epoch(),
            header.batch_number(),
            header.batch_number(),
            None,
        )
        .map_err(ReceiptEvidenceError::Policy)?;
        let policy = ProtocolEvidenceVerifier::new(protocol_version, network_id, vec![sequencer])
            .map_err(ReceiptEvidenceError::Policy)?;
        let verified = policy.verify_receipt(raw)?;
        let protocol = verified
            .verified
            .receipt()
            .protocol()
            .ok_or(ReceiptEvidenceError::BatchIdentity)?;
        if protocol.batch_id() != batch.batch_id() || protocol.asset() != batch.asset() {
            return Err(ReceiptEvidenceError::BatchIdentity);
        }
        Ok(verified)
    }
    #[must_use]
    pub const fn receipt_ref(&self) -> [u8; 32] {
        self.receipt_ref
    }

    #[must_use]
    pub fn canonical_receipt(&self) -> &[u8] {
        self.verified.canonical_bytes()
    }

    #[must_use]
    pub const fn level(&self) -> layerx_types::verify::VerificationLevel {
        self.inclusion.level()
    }

    #[must_use]
    pub fn activity_id(&self) -> [u8; 32] {
        self.activity_id
    }

    #[must_use]
    pub fn result_code(&self) -> i32 {
        self.result_code
    }

    #[must_use]
    pub const fn global_sequence(&self) -> u64 {
        self.global_sequence
    }

    #[must_use]
    pub fn amount(&self) -> u128 {
        self.amount
    }

    #[must_use]
    pub const fn module_id(&self) -> u16 {
        self.module_id
    }

    #[must_use]
    pub const fn operation(&self) -> u8 {
        self.operation
    }
}

/// Receipt evidence verified under daemon authority with the exact material
/// an offline export carries: the authorised batch established by the
/// verifier, the receipt inclusion proof and the authenticated signed header.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ExportReceiptEvidence {
    verified: VerifiedReceiptEvidence,
    authorised: AuthorizedBatch,
    raw: RawReceiptEvidence,
}

impl ExportReceiptEvidence {
    #[must_use]
    pub const fn verified(&self) -> &VerifiedReceiptEvidence {
        &self.verified
    }

    #[must_use]
    pub const fn authorized_batch(&self) -> &AuthorizedBatch {
        &self.authorised
    }

    #[must_use]
    pub const fn raw(&self) -> &RawReceiptEvidence {
        &self.raw
    }

    /// Returns the outcome digest of the verified receipt, absent for a receipt
    /// whose verified evidence names none.
    #[must_use]
    pub const fn receipt_digest(&self) -> Option<[u8; 32]> {
        self.verified.verified.evidence().receipt_digest()
    }

    /// Returns the resulting state root committed by the verified receipt.
    #[must_use]
    pub fn resulting_state_root(&self) -> Option<[u8; 32]> {
        self.verified
            .verified
            .receipt()
            .protocol()
            .map(|protocol| protocol.resulting_state_root())
    }
}

/// Raw state and signed inclusion material returned by a core boundary.
///
/// Either an exact state leaf with its Merkle path under a signed batch header,
/// or the module-state witness material a node point read returns.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct RawStateEvidence {
    canonical_state: Vec<u8>,
    shape: StateEvidenceShape,
}

#[derive(Clone, Debug, Eq, PartialEq)]
enum StateEvidenceShape {
    Leaf {
        proof: Proof,
        resulting_state_root: [u8; 32],
        canonical_header: Vec<u8>,
        header_signature: [u8; 64],
    },
    ModuleWitness {
        committed: crate::read::committed::CommittedCall,
        budget_id: Option<[u8; 32]>,
    },
}

impl RawStateEvidence {
    #[must_use]
    pub fn new(
        canonical_state: Vec<u8>,
        proof: Proof,
        resulting_state_root: [u8; 32],
        canonical_header: Vec<u8>,
        header_signature: [u8; 64],
    ) -> Self {
        Self {
            canonical_state,
            shape: StateEvidenceShape::Leaf {
                proof,
                resulting_state_root,
                canonical_header,
                header_signature,
            },
        }
    }

    /// Decodes the getBudget result, retaining the call that establishes each record field.
    pub fn budget_witness(
        committed: crate::read::committed::CommittedCall,
        budget_id: [u8; 32],
    ) -> Result<Self, StateEvidenceError> {
        let canonical_state = committed_state_bytes(&committed, Some(budget_id))?;
        Ok(Self {
            canonical_state,
            shape: StateEvidenceShape::ModuleWitness {
                committed,
                budget_id: Some(budget_id),
            },
        })
    }

    #[must_use]
    pub fn canonical_state(&self) -> &[u8] {
        &self.canonical_state
    }

    /// Returns the Merkle path of state-leaf evidence; module witnesses carry none.
    #[must_use]
    pub const fn proof(&self) -> Option<&Proof> {
        match &self.shape {
            StateEvidenceShape::Leaf { proof, .. } => Some(proof),
            StateEvidenceShape::ModuleWitness { .. } => None,
        }
    }

    /// Returns the named resulting state root of state-leaf evidence.
    #[must_use]
    pub const fn resulting_state_root(&self) -> Option<[u8; 32]> {
        match &self.shape {
            StateEvidenceShape::Leaf {
                resulting_state_root,
                ..
            } => Some(*resulting_state_root),
            StateEvidenceShape::ModuleWitness { .. } => None,
        }
    }

    /// Returns the canonical signed header of state-leaf evidence.
    #[must_use]
    pub fn canonical_header(&self) -> Option<&[u8]> {
        match &self.shape {
            StateEvidenceShape::Leaf {
                canonical_header, ..
            } => Some(canonical_header),
            StateEvidenceShape::ModuleWitness { .. } => None,
        }
    }

    /// Returns the header signature of state-leaf evidence.
    #[must_use]
    pub const fn header_signature(&self) -> Option<[u8; 64]> {
        match &self.shape {
            StateEvidenceShape::Leaf {
                header_signature, ..
            } => Some(*header_signature),
            StateEvidenceShape::ModuleWitness { .. } => None,
        }
    }

    /// Committed calls carry checkpoint coordinates rather than a serialized transport witness.
    #[must_use]
    pub fn proof_material(&self) -> Option<&[u8]> {
        match &self.shape {
            StateEvidenceShape::ModuleWitness { .. } => None,
            StateEvidenceShape::Leaf { .. } => None,
        }
    }
}

fn committed_state_bytes(
    committed: &crate::read::committed::CommittedCall,
    budget_id: Option<[u8; 32]>,
) -> Result<Vec<u8>, StateEvidenceError> {
    let Some(budget_id) = budget_id else {
        return Ok(committed.canonical_bytes().to_vec());
    };
    let mut pay = [0; 20];
    pay[18..].copy_from_slice(&[0x10, 0x1b]);
    let mut calldata =
        layerx_crypto::evm_transaction::transaction_hash(b"getBudget(bytes32)")[..4].to_vec();
    calldata.extend(budget_id);
    if committed.precompile() != pay || committed.calldata() != calldata {
        return Err(StateEvidenceError::Committed);
    }
    let budget = crate::budget::ProtocolBudgetState::decode_get_budget(
        budget_id,
        committed.checkpoint().height,
        committed.canonical_bytes(),
    )
    .map_err(|_| StateEvidenceError::Committed)?;
    crate::human_runtime::canonical_budget_record(budget.record())
        .map_err(|_| StateEvidenceError::Committed)
}

/// Exact verification stage which refused raw state evidence.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum StateEvidenceError {
    Committed,
    Policy(VerifierPolicyError),
    Inclusion(InclusionError),
}

/// State bytes available only after configured policy and canonical proof verification.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct VerifiedStateEvidence {
    canonical_state: Vec<u8>,
    level: VerificationLevel,
    observed_head_sequence: u64,
}

impl VerifiedStateEvidence {
    #[must_use]
    pub fn canonical_state(&self) -> &[u8] {
        &self.canonical_state
    }

    #[must_use]
    pub const fn level(&self) -> VerificationLevel {
        self.level
    }

    #[must_use]
    pub const fn observed_head_sequence(&self) -> u64 {
        self.observed_head_sequence
    }
}

#[cfg(test)]
#[path = "protocol_evidence_native_tests.rs"]
mod native_terminal_tests;

pub(crate) fn verified_program_owner(
    node: &SequencerEndpoint,
    checkpoint: &crate::read::ServedCheckpoint,
    program_id: &[u8; 32],
) -> Result<[u8; 32], HumanOperationError> {
    let selector = layerx_crypto::evm_transaction::transaction_hash(b"getProgramOwner(bytes32)");
    let mut calldata = selector[..4].to_vec();
    calldata.extend_from_slice(program_id);
    let verified = verified_program_record(node, checkpoint, &calldata)?;
    let owner: [u8; 32] = verified
        .canonical_state()
        .try_into()
        .map_err(|_| HumanOperationError::Refused)?;
    if owner == [0; 32] {
        return Err(HumanOperationError::Refused);
    }
    Ok(owner)
}

pub(crate) fn verified_program_record(
    node: &SequencerEndpoint,
    checkpoint: &crate::read::ServedCheckpoint,
    calldata: &[u8],
) -> Result<VerifiedStateEvidence, HumanOperationError> {
    let reads = crate::read::CommittedReads::at_checkpoint(node, checkpoint)
        .map_err(committed_read_error)?;
    let mut precompile = [0; 20];
    precompile[18..].copy_from_slice(&[0x10, 0x1d]);
    let value = reads
        .budget_state(precompile, calldata)
        .map_err(committed_read_error)?;
    Ok(VerifiedStateEvidence {
        canonical_state: value.canonical_bytes().to_vec(),
        level: value.level(),
        observed_head_sequence: value.freshness().observed_head_sequence,
    })
}

pub(crate) fn committed_read_error(error: crate::read::CommittedReadError) -> HumanOperationError {
    match error {
        crate::read::CommittedReadError::Rpc(_)
        | crate::read::CommittedReadError::Checkpoint(crate::read::CheckpointReadError::Rpc(_)) => {
            HumanOperationError::Unavailable
        }
        _ => HumanOperationError::Refused,
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[track_caller]
    fn checked<T, E: std::fmt::Debug>(result: Result<T, E>) -> T {
        result.unwrap_or_else(|error| panic!("native time-window evidence: {error:?}"))
    }

    fn captured_batch() -> (
        EvidenceAuthority,
        Did,
        RawTimeWindowBatch,
        AuthenticatedCoreTime,
    ) {
        let root = std::path::Path::new(env!("CARGO_MANIFEST_DIR"))
            .join("../../../../tests/fixtures/custody/daemon-light-credit-receipt");
        let read = |name: &str| checked(std::fs::read(root.join(name)));
        let header_bytes = read("header");
        let header = checked(decode_batch_header(&header_bytes));
        let signature = checked(read("header.signature").try_into());
        let public = checked(read("sequencer.public").try_into());
        let authority = checked(EvidenceAuthority::pinned_to_handshake(
            header.protocol_version(),
            header.network_id(),
            public,
        ));
        let kind = checked(ActivityType::new(ModuleId::Bridge, 1));
        let registration = checked(ModuleRegistration::new(ModuleId::Bridge, &[kind]));
        let registry = checked(ModuleRegistry::new(&[registration]));
        let activity_bytes = read("activity");
        let activity = checked(decode_signed(&activity_bytes, &registry));
        let actor = checked(Did::new(activity.actor_did()));
        let receipt = RawReceiptEvidence::new(
            read("credit.receipt"),
            checked(layerx_proof::merkle::decode_proof(&read("receipt.proof"))),
            header_bytes.clone(),
            signature,
        );
        let maintenance = RawReceiptEvidence::new(
            read("maintenance.receipt"),
            checked(layerx_proof::merkle::decode_proof(&read(
                "maintenance.proof",
            ))),
            header_bytes,
            signature,
        );
        let mut checkpoint = crate::read::ServedCheckpoint::test_fixture(
            header.last_sequence(),
            (header.timestamp_ms() / 1_000) as i64,
            (header.timestamp_ms() % 1_000) as u32 * 1_000_000,
        );
        checkpoint.header.chain_id = header.network_id().to_string();
        checkpoint.header.app_hash = header.resulting_state_root();
        checkpoint.commit.block_id.hash = checkpoint.header.block_hash();
        use ed25519_dalek::Signer as _;
        for index in 0..checkpoint.commit.votes.len() {
            let vote = checkpoint.commit.votes[index];
            let key = ed25519_dalek::SigningKey::from_bytes(&[index as u8 + 1; 32]);
            checkpoint.commit.votes[index].signature = key
                .sign(
                    &checkpoint
                        .commit
                        .vote_sign_bytes(&checkpoint.header.chain_id, vote.timestamp),
                )
                .to_bytes();
        }
        checkpoint = checked(crate::read::checkpoint(
            checkpoint.height,
            &checkpoint.header.chain_id,
            &checkpoint.header,
            &checkpoint.commit,
            &checkpoint.validators,
        ));
        let authority = checked(authority.pin_committed_validators(
            checkpoint.header.chain_id.clone(),
            checkpoint.validators.clone(),
        ));
        let observed = checked(authority.authenticate_committed_time(&checkpoint));
        (
            authority,
            actor,
            RawTimeWindowBatch::new(
                vec![RawActivityReceiptEvidence::from_signed_inclusion(
                    activity_bytes,
                    receipt,
                )],
                maintenance,
            ),
            observed,
        )
    }

    #[test]
    fn committed_time_native_batch_proves_count_and_exact_receipt_denomination() {
        let (authority, actor, batch, observed) = captured_batch();
        let header = checked(decode_batch_header(batch.maintenance.canonical_header()));
        assert_eq!(header.batch_number(), 1);
        assert_eq!(header.first_sequence(), 1);
        let seconds = observed.observed_core_ms() / 1_000 + 1;
        let facts = checked(authority.authenticate_cumulative_time_use(
            &actor,
            seconds,
            &observed,
            std::slice::from_ref(&batch),
        ));
        assert_eq!(facts.actor(), &actor);
        assert_eq!(facts.count(), 1);
        assert_eq!(facts.successful_count(), 1);
        assert_eq!(facts.observed_core_ms(), header.timestamp_ms());
        assert_eq!(facts.observed_batch_id(), observed.observed_batch_id());
        assert_eq!(facts.first_covered_sequence(), 1);
        assert_eq!(facts.through_sequence(), header.last_sequence());
        assert_eq!(facts.window_seconds(), seconds);
        let decoded = checked(decode(batch.activities[0].receipt().canonical_receipt()));
        let receipt = decoded
            .protocol()
            .unwrap_or_else(|| panic!("native protocol receipt"));
        assert_eq!(facts.receipt_amounts().len(), 1);
        assert_eq!(facts.receipt_amounts()[0].asset(), receipt.asset());
        assert_eq!(facts.receipt_amounts()[0].source_account(), receipt.from());
        assert_eq!(facts.receipt_amounts()[0].amount(), receipt.amount());
        let other = checked(Did::new(b"did:layerx:time-window-other"));
        let empty = checked(authority.authenticate_cumulative_time_use(
            &other,
            seconds,
            &observed,
            &[batch],
        ));
        assert_eq!(empty.count(), 0);
        assert_eq!(empty.successful_count(), 0);
        assert!(empty.receipt_amounts().is_empty());
    }

    #[test]
    fn committed_time_native_window_refuses_missing_duplicate_and_altered_material() {
        let (authority, actor, batch, observed) = captured_batch();
        let seconds = observed.observed_core_ms() / 1_000 + 1;
        let refused = |batches: &[RawTimeWindowBatch]| {
            assert!(authority
                .authenticate_cumulative_time_use(&actor, seconds, &observed, batches)
                .is_err());
        };
        refused(&[]);
        refused(&[batch.clone(), batch.clone()]);
        let mut missing = batch.clone();
        missing.activities.clear();
        refused(&[missing]);
        let mut duplicate = batch.clone();
        duplicate.activities.push(duplicate.activities[0].clone());
        refused(&[duplicate]);
        let mut bad = batch.clone();
        bad.maintenance.canonical_receipt[0] ^= 1;
        refused(&[bad]);
        let mut bad = batch.clone();
        bad.maintenance.header_signature[0] ^= 1;
        refused(&[bad]);
        let mut bad = batch.clone();
        bad.activities[0].canonical_activity[0] ^= 1;
        refused(&[bad]);
        let mut bad = batch.clone();
        match &mut bad.activities[0].receipt {
            CumulativeReceiptEvidence::SignedInclusion(raw)
            | CumulativeReceiptEvidence::Outcome(raw) => {
                raw.canonical_receipt[0] ^= 1;
            }
        }
        refused(&[bad]);
        let mut bad = batch.clone();
        bad.maintenance.proof = checked(Proof::new(
            0,
            batch.maintenance.proof.leaf_count(),
            batch.maintenance.proof.siblings().to_vec(),
        ));
        refused(&[bad]);
        assert!(authority
            .authenticate_cumulative_time_use(&actor, 0, &observed, std::slice::from_ref(&batch))
            .is_err());
        assert!(authority
            .authenticate_cumulative_time_use(&actor, u64::MAX, &observed, &[batch])
            .is_err());
    }

    fn authority(checkpoint: &crate::read::ServedCheckpoint) -> EvidenceAuthority {
        checked(
            checked(EvidenceAuthority::pinned_to_handshake(3, 125, [7; 32]))
                .pin_committed_validators("125".into(), checkpoint.validators.clone()),
        )
    }

    #[test]
    fn committed_time_is_the_signed_block_timestamp_in_milliseconds() {
        let checkpoint = crate::read::ServedCheckpoint::test_fixture(8, 1_700_000_000, 123_456_789);
        let observed = checked(authority(&checkpoint).authenticate_committed_time(&checkpoint));
        assert_eq!(observed.observed_core_ms(), 1_700_000_000_123);
        assert_eq!(observed.observed_batch_id(), checkpoint.header.block_hash());
        assert_eq!(observed.through_sequence(), 8);
    }

    #[test]
    fn committed_time_requires_the_handshake_network_and_receipt_authority() {
        let (authority, actor, batch, observed) = captured_batch();
        let header = checked(decode_batch_header(batch.maintenance.canonical_header()));
        let wrong_network = checked(
            checked(EvidenceAuthority::pinned_to_handshake(
                header.protocol_version(),
                header.network_id() + 1,
                authority.verifier.handshake_pin.expect("fixture pin"),
            ))
            .pin_committed_validators(
                observed.checkpoint.header.chain_id.clone(),
                observed.checkpoint.validators.clone(),
            ),
        );
        assert!(wrong_network
            .authenticate_committed_time(&observed.checkpoint)
            .is_err());
        let wrong_key = checked(
            checked(EvidenceAuthority::pinned_to_handshake(
                header.protocol_version(),
                header.network_id(),
                [1; 32],
            ))
            .pin_committed_validators(
                observed.checkpoint.header.chain_id.clone(),
                observed.checkpoint.validators.clone(),
            ),
        );
        assert!(wrong_key
            .authenticate_cumulative_time_use(&actor, 1, &observed, &[batch])
            .is_err());
    }

    #[test]
    fn committed_time_requires_pinned_validators_and_an_unaltered_quorum() {
        let checkpoint = crate::read::ServedCheckpoint::test_fixture(8, 1_700_000_000, 0);
        let unpinned = checked(EvidenceAuthority::pinned_to_handshake(3, 125, [7; 32]));
        assert!(unpinned.authenticate_committed_time(&checkpoint).is_err());
        let mut wrong_set = checkpoint.validators.clone();
        wrong_set[0].voting_power += 1;
        let wrong = checked(
            unpinned
                .clone()
                .pin_committed_validators("125".into(), wrong_set),
        );
        assert!(wrong.authenticate_committed_time(&checkpoint).is_err());
        let wrong = checked(
            unpinned.pin_committed_validators("other-chain".into(), checkpoint.validators.clone()),
        );
        assert!(wrong.authenticate_committed_time(&checkpoint).is_err());
        let verifier = authority(&checkpoint);
        for changed in [
            "timestamp",
            "signature",
            "quorum",
            "hash",
            "height",
            "app",
            "rung",
            "validators",
        ] {
            let mut bad = checkpoint.clone();
            match changed {
                "timestamp" => bad.header.time.seconds += 1,
                "signature" => bad.commit.votes[0].signature[0] ^= 1,
                "quorum" => {
                    bad.commit.votes.pop();
                }
                "hash" => bad.block_hash[0] ^= 1,
                "height" => bad.height += 1,
                "app" => bad.app_hash[0] ^= 1,
                "rung" => bad.rung = layerx_types::rung::Rung::Receipted,
                "validators" => bad.validators.reverse(),
                _ => unreachable!(),
            }
            assert!(
                verifier.authenticate_committed_time(&bad).is_err(),
                "{changed}"
            );
        }
        for (seconds, nanos) in [(-1, 0), (0, 0)] {
            let bad = crate::read::ServedCheckpoint::test_fixture(8, seconds, nanos);
            assert!(authority(&bad).authenticate_committed_time(&bad).is_err());
        }
    }

    #[test]
    fn committed_time_refuses_budget_expiry_before_or_at_the_committed_timestamp() {
        use crate::budget::{
            create_daemon_limit, daemon_limit_id, BudgetLimiter, CoreTimestampMs, DaemonLimitError,
            DaemonLimitRecord,
        };
        use crate::store::{Store, TenantId};
        let checkpoint = crate::read::ServedCheckpoint::test_fixture(8, 1_700_000_000, 123_000_000);
        let observed = checked(authority(&checkpoint).authenticate_committed_time(&checkpoint));
        let now = observed.observed_core_ms();
        let directory =
            std::env::temp_dir().join(format!("committed-expiry-{}-{}", std::process::id(), now));
        let mut store = checked(Store::open(&directory));
        let limiter = checked(BudgetLimiter::new(Vec::new()));
        let record = DaemonLimitRecord {
            tenant: checked(TenantId::new("committed-clock")),
            budget_id: [1; 32],
            limit_id: daemon_limit_id([1; 32]),
            agent_digest: [2; 32],
            asset: [3; 32],
            ceiling: 100,
            consumed: 0,
            expiry_ms: now - 1,
            revoked: false,
            mutation_key: [4; 32],
            body_digest: [5; 32],
            revoke_key: [0; 32],
        };
        for expiry_ms in [now - 1, now] {
            let mut expired = record.clone();
            expired.expiry_ms = expiry_ms;
            assert!(matches!(
                create_daemon_limit(&mut store, &limiter, expired, CoreTimestampMs(now)),
                Err(DaemonLimitError::Expired)
            ));
        }
        let mut live = record;
        live.expiry_ms = now + 1;
        let installed = checked(create_daemon_limit(
            &mut store,
            &limiter,
            live.clone(),
            CoreTimestampMs(now),
        ));
        assert_eq!(installed, live);
        drop(store);
        checked(std::fs::remove_dir_all(directory));
    }
}

pub(crate) type HistoryActivityEvidence = RawActivityReceiptEvidence<CommittedHistoryReceipt, Did>;
pub(crate) type HistoryWindowBatch =
    RawTimeWindowBatch<HistoryActivityEvidence, AuthenticatedCoreTime>;

impl HistoryActivityEvidence {
    pub(crate) fn from_committed_history(
        actor: &Did,
        source: [u8; 32],
        receipt: CommittedHistoryReceipt,
    ) -> Result<Self, CumulativeUseError> {
        let decoded = decode(&receipt.canonical).map_err(|_| CumulativeUseError::Activity)?;
        let protocol = decoded.protocol().ok_or(CumulativeUseError::Activity)?;
        if protocol.from() != source {
            return Err(CumulativeUseError::ActivityReceiptMismatch);
        }
        Ok(Self {
            canonical_activity: actor.clone(),
            receipt: CumulativeReceiptEvidence::Outcome(receipt),
        })
    }

    fn history_receipt(&self) -> &CommittedHistoryReceipt {
        match &self.receipt {
            CumulativeReceiptEvidence::Outcome(receipt)
            | CumulativeReceiptEvidence::SignedInclusion(receipt) => receipt,
        }
    }
}

/// Constructed only by a complete committed-range scan, never from pagination metadata.
#[derive(Clone, Debug)]
pub(crate) struct VerifiedHistoryCoverage {
    pub(super) actor: Did,
    pub(super) checkpoint: [u8; 32],
    pub(super) first: u64,
    pub(super) last: u64,
    pub(super) entries_digest: [u8; 32],
    pub(super) time_seconds: Option<u64>,
}
pub(crate) fn history_entries_digest(entries: &[HistoryActivityEvidence]) -> [u8; 32] {
    let mut hash = Sha256::new();
    hash.update(b"layerx/policy-history/coverage/v1\0");
    for entry in entries {
        let receipt = entry.history_receipt();
        hash.update((receipt.canonical.len() as u64).to_be_bytes());
        hash.update(&receipt.canonical);
        hash.update(receipt.position.block_height.to_be_bytes());
        hash.update(receipt.position.tx_index.to_be_bytes());
        hash.update(receipt.timestamp.to_be_bytes());
    }
    hash.finalize().into()
}

impl CumulativeUseWindow {
    pub(crate) fn authenticate_history(
        self,
        actor: &Did,
        checkpoint: &crate::read::ServedCheckpoint,
        entries: &[HistoryActivityEvidence],
        coverage: &VerifiedHistoryCoverage,
    ) -> Result<AuthenticatedCumulativeUse, CumulativeUseError> {
        if self.first == 0
            || self.first > self.last
            || self.last > checkpoint.height
            || self.last - self.first >= MAX_CUMULATIVE_EVIDENCE_ITEMS as u64
            || entries.len() > MAX_CUMULATIVE_EVIDENCE_ITEMS
        {
            return Err(CumulativeUseError::InvalidWindow);
        }
        if coverage.time_seconds.is_some()
            || coverage.actor != *actor
            || coverage.checkpoint != checkpoint.block_hash
            || coverage.first > self.first
            || coverage.last < self.last
            || coverage.entries_digest != history_entries_digest(entries)
        {
            return Err(CumulativeUseError::InvalidWindow);
        }
        let mut seen = BTreeSet::new();
        let mut amount = 0u128;
        let mut count = 0u64;
        let mut bytes = 0;
        for entry in entries {
            let raw = entry.history_receipt();
            admit_bytes(&mut bytes, raw.canonical.len())
                .map_err(|_| CumulativeUseError::InvalidWindow)?;
            let decoded = decode(&raw.canonical).map_err(|_| CumulativeUseError::Activity)?;
            let receipt = decoded.protocol().ok_or(CumulativeUseError::Activity)?;
            if entry.canonical_activity != *actor
                || raw.checkpoint_hash != checkpoint.block_hash
                || !(self.first..=self.last).contains(&raw.position.block_height)
            {
                return Err(CumulativeUseError::Sequence);
            }
            if !seen.insert(receipt.activity_id()) {
                return Err(CumulativeUseError::Duplicate);
            }
            if receipt.result_code() == 0 {
                amount = amount
                    .checked_add(receipt.amount())
                    .ok_or(CumulativeUseError::AmountOverflow)?;
                count = count
                    .checked_add(1)
                    .ok_or(CumulativeUseError::CountOverflow)?;
            }
        }
        Ok(AuthenticatedCumulativeUse {
            actor: actor.clone(),
            window: self,
            amount,
            count,
        })
    }
}

impl HistoryWindowBatch {
    pub(crate) fn from_committed_history(
        activities: Vec<HistoryActivityEvidence>,
        observed: AuthenticatedCoreTime,
    ) -> Self {
        Self {
            activities,
            maintenance: observed,
        }
    }

    pub(crate) fn authenticate_history(
        &self,
        actor: &Did,
        window_seconds: u64,
        first_covered_sequence: u64,
        coverage: &VerifiedHistoryCoverage,
    ) -> Result<AuthenticatedTimeWindowUse, TimeWindowUseError> {
        let observed = &self.maintenance;
        if coverage.time_seconds != Some(window_seconds)
            || coverage.actor != *actor
            || coverage.checkpoint != observed.observed_batch_id()
            || coverage.first != first_covered_sequence
            || coverage.last != observed.through_sequence()
            || coverage.entries_digest != history_entries_digest(&self.activities)
        {
            return Err(TimeWindowUseError::Bounds);
        }
        let window_ms = window_seconds
            .checked_mul(1000)
            .filter(|n| *n > 0)
            .ok_or(TimeWindowUseError::Window)?;
        let lower = observed.observed_core_ms().saturating_sub(window_ms);
        if self.activities.len() > MAX_CUMULATIVE_EVIDENCE_ITEMS
            || first_covered_sequence == 0
            || first_covered_sequence > observed.through_sequence()
        {
            return Err(TimeWindowUseError::Bounds);
        }
        let mut count = 0u64;
        let mut successful_count = 0u64;
        let mut seen = BTreeSet::new();
        let mut activity_ids = Vec::new();
        let mut amounts = std::collections::BTreeMap::<([u8; 32], [u8; 32]), u128>::new();
        let mut bytes = 0;
        for entry in &self.activities {
            let raw = entry.history_receipt();
            admit_bytes(&mut bytes, raw.canonical.len()).map_err(|_| TimeWindowUseError::Bounds)?;
            let decoded = decode(&raw.canonical).map_err(|_| TimeWindowUseError::Identity)?;
            let receipt = decoded.protocol().ok_or(TimeWindowUseError::Identity)?;
            let timestamp = raw.timestamp;
            if entry.canonical_activity != *actor
                || raw.checkpoint_hash != observed.observed_batch_id()
                || raw.position.block_height > observed.through_sequence()
                || timestamp > observed.observed_core_ms()
            {
                return Err(TimeWindowUseError::Identity);
            }
            if !seen.insert(receipt.activity_id()) {
                return Err(TimeWindowUseError::Duplicate);
            }
            if timestamp <= lower {
                continue;
            }
            count = count.checked_add(1).ok_or(TimeWindowUseError::Overflow)?;
            activity_ids.push(receipt.activity_id());
            if receipt.result_code() == 0 {
                successful_count = successful_count
                    .checked_add(1)
                    .ok_or(TimeWindowUseError::Overflow)?;
                let amount = amounts
                    .entry((receipt.asset(), receipt.from()))
                    .or_default();
                *amount = amount
                    .checked_add(receipt.amount())
                    .ok_or(TimeWindowUseError::Overflow)?;
            }
        }
        Ok(AuthenticatedTimeWindowUse {
            actor: actor.clone(),
            observed: observed.clone(),
            window_seconds,
            first_covered_sequence,
            count,
            successful_count,
            activity_ids,
            receipt_amounts: amounts
                .into_iter()
                .map(
                    |((asset, source_account), amount)| AuthenticatedReceiptAmount {
                        asset,
                        source_account,
                        amount,
                    },
                )
                .collect(),
        })
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct CommittedHistoryReceipt {
    pub(crate) canonical: Vec<u8>,
    pub(crate) position: crate::read::Cursor,
    pub(crate) timestamp: u64,
    pub(crate) checkpoint_hash: [u8; 32],
}

pub struct PolicyHistorySource {
    pub(crate) endpoint: SequencerEndpoint,
    pub(crate) results: crate::read::NativeResultSource,
    chain_id: String,
    landing: crate::read::NativeResultSource,
}

impl PolicyHistorySource {
    pub fn new(
        config: &crate::node_rpc::NodeRpcConfig,
        result_url: &str,
    ) -> Result<Self, crate::read::HistoryReadError> {
        let certificates = if config.url.starts_with("https://") {
            let roots = rustls_native_certs::load_native_certs();
            if !roots.errors.is_empty() || roots.certs.is_empty() {
                return Err(crate::read::HistoryReadError::Malformed);
            }
            roots.certs
        } else {
            Vec::new()
        };
        let anchors: Vec<_> = certificates.iter().map(|c| c.as_ref()).collect();
        let endpoint = SequencerEndpoint::new(&config.url, &anchors)
            .map_err(crate::read::HistoryReadError::Sequencer)?;
        if endpoint
            .chain_id()
            .map_err(crate::read::HistoryReadError::Sequencer)?
            != config.chain_id
        {
            return Err(crate::read::HistoryReadError::Malformed);
        }
        let results = crate::read::NativeResultSource::connect(result_url, None)
            .map_err(|_| crate::read::HistoryReadError::ItemUnproven)?;
        let landing = crate::read::NativeResultSource::connect(&config.url, None)
            .map_err(|_| crate::read::HistoryReadError::ItemUnproven)?;
        Ok(Self {
            endpoint,
            results,
            landing,
            chain_id: config.chain_id.to_string(),
        })
    }

    /// Follows every authenticated predecessor from the captured checkpoint.
    pub(crate) fn committed_at(
        &self,
        checkpoint: &crate::read::ServedCheckpoint,
        height: u64,
    ) -> Result<crate::read::ServedCheckpoint, crate::read::HistoryReadError> {
        let bad = || crate::read::HistoryReadError::ItemUnproven;
        if height == 0 || height > checkpoint.height || checkpoint.height - height > 4096 {
            return Err(bad());
        }
        let mut current = crate::read::checkpoint(
            checkpoint.height,
            &self.chain_id,
            &checkpoint.header,
            &checkpoint.commit,
            &checkpoint.validators,
        )
        .map_err(|_| bad())?;
        if current != *checkpoint {
            return Err(bad());
        }
        while current.height > height {
            let parent = self
                .results
                .checkpoint(current.height - 1, &self.chain_id, &checkpoint.validators)
                .map_err(|_| bad())?;
            if current.header.last_block_id.map(|id| id.hash) != Some(parent.block_hash)
                || parent.header.time.seconds > current.header.time.seconds
            {
                return Err(bad());
            }
            current = parent;
        }
        Ok(current)
    }

    pub(crate) fn successor(
        &self,
        checkpoint: &crate::read::ServedCheckpoint,
        block: &crate::read::ServedCheckpoint,
    ) -> Result<crate::read::ServedCheckpoint, crate::read::HistoryReadError> {
        let next = if block.height < checkpoint.height {
            self.committed_at(checkpoint, block.height + 1)?
        } else {
            self.results
                .checkpoint(
                    block
                        .height
                        .checked_add(1)
                        .ok_or(crate::read::HistoryReadError::ItemUnproven)?,
                    &self.chain_id,
                    &checkpoint.validators,
                )
                .map_err(|_| crate::read::HistoryReadError::ItemUnproven)?
        };
        if next.header.last_block_id.map(|id| id.hash) != Some(block.block_hash) {
            return Err(crate::read::HistoryReadError::ItemUnproven);
        }
        Ok(next)
    }

    pub(crate) fn page(
        &self,
        account: &str,
        cursor: Option<crate::read::Cursor>,
        limits: crate::read::HistoryLimits,
    ) -> Result<crate::read::HistoryPage, crate::read::HistoryReadError> {
        crate::read::HistoryPage::read_newest(&self.endpoint, account, cursor, limits)
    }

    pub fn prove(
        &self,
        canonical: &[u8],
        position: crate::read::Cursor,
        checkpoint: &crate::read::ServedCheckpoint,
        keys: &[layerx_client::sequencer::ReceiptKeyEntry],
        bytes: &mut usize,
    ) -> Result<CommittedHistoryReceipt, crate::read::HistoryReadError> {
        use layerx_client::rpc::receipt::{lookup_authenticated, Lookup, LookupContext};
        let bad = || crate::read::HistoryReadError::ItemUnproven;
        let decoded = layerx_wire::receipt::decode(canonical).map_err(|_| bad())?;
        let protocol = decoded.protocol().ok_or_else(bad)?;
        let hash = protocol.activity_id();
        let Lookup::Verified(receipt) =
            lookup_authenticated(&self.endpoint, hash, LookupContext { receipt_keys: keys })
                .map_err(|_| bad())?
        else {
            return Err(bad());
        };
        let landing = receipt.landing().ok_or_else(bad)?;
        if receipt.canonical_bytes() != canonical
            || receipt.height() != position.block_height
            || receipt.execution().index != u64::from(position.tx_index)
            || position.log_index != 0
            || landing.block_number != receipt.height()
            || landing.transaction_index != receipt.execution().index
        {
            return Err(bad());
        }
        let block = self.committed_at(checkpoint, position.block_height)?;
        let next = self.successor(checkpoint, &block)?;
        let raw_landing = self
            .landing
            .rpc(
                "eth_getTransactionReceipt",
                serde_json::json!([format!("0x{}", layerx_programs::hex::encode(&hash))]),
            )
            .map_err(|_| bad())?;
        let landing_hash = raw_landing["blockHash"]
            .as_str()
            .and_then(|text| text.strip_prefix("0x"))
            .and_then(|text| layerx_programs::hex::decode_digest(text).ok());
        if landing_hash != Some(block.block_hash)
            || crate::policy_history::checkpoint_timestamp_ms(&block).ok()
                != Some(receipt.execution().timestamp)
        {
            return Err(bad());
        }
        let inclusion = self
            .results
            .acquire(position.block_height, receipt.execution().index)
            .map_err(|_| bad())?;
        inclusion
            .verify_committed(
                position.block_height,
                receipt.execution().index,
                &next,
                &self.chain_id,
                &checkpoint.validators,
            )
            .map_err(|_| bad())?;
        crate::read::abci_result::verify_native_receipt(&inclusion.result, &receipt)
            .map_err(|_| bad())?;
        admit_bytes(bytes, canonical.len())?;
        admit_bytes(
            bytes,
            inclusion.result.len() + inclusion.proof.aunts.len() * 32,
        )?;
        Ok(CommittedHistoryReceipt {
            canonical: canonical.to_vec(),
            position,
            timestamp: receipt.execution().timestamp,
            checkpoint_hash: checkpoint.block_hash,
        })
    }
}

pub(crate) fn admit_bytes(
    total: &mut usize,
    additional: usize,
) -> Result<(), crate::read::HistoryReadError> {
    *total = total
        .checked_add(additional)
        .filter(|n| *n <= 16 * 1024 * 1024)
        .ok_or(crate::read::HistoryReadError::Oversized)?;
    Ok(())
}
