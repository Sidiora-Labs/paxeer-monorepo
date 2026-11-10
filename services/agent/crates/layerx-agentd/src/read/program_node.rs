//! Agent read route backed by the authenticated layerxd Programs service.

use std::time::Duration;

use layerx_client::sequencer::{SequencerEndpoint, SequencerError};
use layerx_programs::chain_evidence::commitment_trust_step;
#[cfg(test)]
use layerx_programs::chain_proof::decode::decode_commitment_proof;
use layerx_programs::chain_proof::{
    decode::CommitmentProof, header::ChainHeader, merkle::SimpleProof, validators::ValidatorSet,
    ChainProofVerifier, Commit, VerifiedChainHead as VerifiedHeader,
};
use layerx_programs::state_record::{
    ProtocolAdapterError as StateRecordError, ProtocolProgramStateRead,
};
use layerx_programs::{
    hex, AccountStateHead, DeploymentProof, ProgramBundleError, ProgramHeadKind, ProgramId,
    ProgramLifecycle, ProtocolDeploymentVerifier, ProtocolEvidenceError, Registry,
    VerifiedChainHead, VerifiedDeploymentEvidence, VerifiedProgramBundle,
};
#[cfg(test)]
use serde_json::Value;

use super::program_balances_impl::{
    CommittedProgramBalanceFreshness, ProgramBalanceRead, ProgramValueBalance,
};

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum ProgramReadError {
    CoreRefused(i32),
    NonCanonicalView,
    AccountState(layerx_programs::AccountStateError),
    Registry(layerx_programs::RegistryError),
    CorruptRecord,
}

impl From<StateRecordError> for ProgramReadError {
    fn from(error: StateRecordError) -> Self {
        match error {
            StateRecordError::CoreRefused(code) => Self::CoreRefused(code),
            StateRecordError::NonCanonicalView => Self::NonCanonicalView,
            StateRecordError::AccountState(error) => Self::AccountState(error),
            StateRecordError::Registry(error) => Self::Registry(error),
            StateRecordError::CorruptRecord => Self::CorruptRecord,
        }
    }
}

impl From<layerx_programs::RegistryError> for ProgramReadError {
    fn from(error: layerx_programs::RegistryError) -> Self {
        Self::Registry(error)
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
struct ChainHeadEvidence {
    commitment: CommitmentProof,
    transaction: Vec<u8>,
    transaction_proof: SimpleProof,
}

/// Refusal of the authenticated chain-head read.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum ChainHeadError {
    /// Transport, node/authority disagreement, or non-canonical head document.
    View(ProgramReadError),
    /// Protected-trust refusal of the head as kind-5 coordinates.
    Evidence(ProtocolEvidenceError),
}

impl ChainHeadError {
    /// True only for the protected verifier's freshness/superseded-head refusal.
    #[must_use]
    pub const fn is_stale(&self) -> bool {
        matches!(self, Self::Evidence(ProtocolEvidenceError::Stale))
    }
}

/// Production agent reader connected to layerxd and an independent layerxd
/// receipt-authority replica. Neither endpoint is allowed to be agent-local.
pub struct LayerxdProgramBalanceReader {
    sequencer: SequencerEndpoint,
    agent: ureq::Agent,
    endpoint: String,
    authorization: String,
    verifier: ProtocolDeploymentVerifier,
    registry: Registry,
    staleness_limit: u64,
    consensus: Option<super::NativeResultSource>,
    program_state_keys: std::collections::BTreeMap<[u8; 32], (String, Vec<u8>)>,
}

/// Explicit independent authority identity and outbound trust for both read endpoints.
pub struct ProgramAuthority<'a> {
    pub endpoint: &'a str,
    pub authorization: String,
    pub replica_id: [u8; 32],
    pub ca_der: &'a [u8],
}

pub(crate) struct ProgramArtifacts {
    pub(crate) terminal_payload: Vec<u8>,
    pub(crate) call_graph: Vec<u8>,
}

pub(crate) enum ProgramArtifactsError {
    Unavailable,
    Malformed,
}

impl ProgramArtifactsError {
    pub(crate) fn is_unavailable(&self) -> bool {
        matches!(self, Self::Unavailable)
    }
}

#[derive(serde::Deserialize)]
#[serde(deny_unknown_fields)]
struct ProgramArtifactsWire {
    activity_id: String,
    receipt_digest: String,
    terminal_payload: String,
    call_graph: String,
}

impl LayerxdProgramBalanceReader {
    pub(crate) fn read_program_artifacts(
        &self,
        activity_id: [u8; 32],
        receipt_digest: [u8; 32],
    ) -> Result<ProgramArtifacts, ProgramArtifactsError> {
        let url = format!(
            "{}/v1/programs/activities/{}/artifacts?receipt_digest={}",
            self.endpoint,
            hex::encode(&activity_id),
            hex::encode(&receipt_digest),
        );
        let mut response = self
            .agent
            .get(&url)
            .header("Authorization", &format!("Bearer {}", self.authorization))
            .call()
            .map_err(|_| ProgramArtifactsError::Unavailable)?;
        if !response.status().is_success() {
            return Err(ProgramArtifactsError::Unavailable);
        }
        let body = response
            .body_mut()
            .with_config()
            .limit(4_194_560)
            .read_to_string()
            .map_err(|_| ProgramArtifactsError::Malformed)?;
        let wire: ProgramArtifactsWire =
            serde_json::from_str(&body).map_err(|_| ProgramArtifactsError::Malformed)?;
        if wire.activity_id != hex::encode(&activity_id)
            || wire.receipt_digest != hex::encode(&receipt_digest)
        {
            return Err(ProgramArtifactsError::Malformed);
        }
        Ok(ProgramArtifacts {
            terminal_payload: program_artifact_hex(&wire.terminal_payload)?,
            call_graph: program_artifact_hex(&wire.call_graph)?,
        })
    }

    /// Connects the running agent route to the production node pair.
    ///
    /// # Errors
    ///
    /// Refuses empty authorizations, a zero replica id, identical or insecure
    /// endpoints as a non-canonical view.
    pub fn connect(
        endpoint: &str,
        authorization: String,
        authority: ProgramAuthority<'_>,
        verifier: ProtocolDeploymentVerifier,
        registry: Registry,
    ) -> Result<Self, ProgramReadError> {
        // Retain the independent authority's configured identity validation.
        let ProgramAuthority {
            endpoint: authority_endpoint,
            authorization: authority_authorization,
            replica_id: authority_replica_id,
            ca_der,
        } = authority;
        let endpoint = endpoint.trim_end_matches('/');
        let authority_endpoint = authority_endpoint.trim_end_matches('/');
        if authorization.is_empty()
            || authority_authorization.is_empty()
            || authority_replica_id == [0; 32]
            || endpoint == authority_endpoint
            || !secure_endpoint(endpoint)
            || !secure_endpoint(authority_endpoint)
        {
            return Err(ProgramReadError::NonCanonicalView);
        }
        let tls =
            crate::outbound_tls::private_ca(ca_der).ok_or(ProgramReadError::NonCanonicalView)?;
        let config = ureq::Agent::config_builder()
            .tls_config(tls)
            .timeout_global(Some(Duration::from_secs(30)))
            .http_status_as_error(false)
            .build();
        let sequencer = SequencerEndpoint::new(endpoint, &[ca_der])
            .map_err(|_| ProgramReadError::NonCanonicalView)?;
        Ok(Self {
            sequencer,
            agent: config.into(),
            endpoint: endpoint.to_owned(),
            authorization,
            staleness_limit: verifier.staleness_limit_ms(),
            consensus: None,
            program_state_keys: std::collections::BTreeMap::new(),
            verifier,
            registry,
        })
    }

    /// Connects directly to the merged node under protected chain trust.
    /// # Errors
    /// Returns an error if the program view is noncanonical or its state or balance proof cannot
    /// be acquired or verified.
    pub fn connect_native(
        endpoint: &str,
        authorization: String,
        ca_der: Option<&[u8]>,
        source: super::NativeResultSource,
        verifier: ProtocolDeploymentVerifier,
        registry: Registry,
    ) -> Result<Self, ProgramReadError> {
        if authorization.is_empty()
            || !secure_endpoint(endpoint)
            || verifier.chain_trust().is_none()
        {
            return Err(ProgramReadError::NonCanonicalView);
        }
        let roots: Vec<_> = ca_der.into_iter().collect();
        let sequencer = SequencerEndpoint::new(endpoint, &roots)
            .map_err(|_| ProgramReadError::NonCanonicalView)?;
        let tls = match ca_der {
            Some(ca) => crate::outbound_tls::private_ca(ca),
            None => crate::outbound_tls::system(endpoint),
        }
        .ok_or(ProgramReadError::NonCanonicalView)?;
        let agent = ureq::Agent::config_builder()
            .tls_config(tls)
            .timeout_global(Some(Duration::from_secs(30)))
            .max_redirects(0)
            .build()
            .into();
        Ok(Self {
            sequencer,
            agent,
            endpoint: endpoint.trim_end_matches('/').to_owned(),
            authorization,
            staleness_limit: verifier.staleness_limit_ms(),
            verifier,
            registry,
            consensus: Some(source),
            program_state_keys: std::collections::BTreeMap::new(),
        })
    }

    #[must_use]
    pub fn with_consensus_source(mut self, source: super::NativeResultSource) -> Self {
        self.consensus = Some(source);
        self
    }

    /// Configures the Programs deployment's authenticated state-record key.
    /// # Errors
    /// Returns an error if the program view is noncanonical or its state or balance proof cannot
    /// be acquired or verified.
    pub fn with_program_state_key(
        mut self,
        program: ProgramId,
        store_name: String,
        key: Vec<u8>,
    ) -> Result<Self, ProgramReadError> {
        if key.is_empty() || key.len() > 1024 || store_name.is_empty() || store_name.len() > 128 {
            return Err(ProgramReadError::NonCanonicalView);
        }
        self.program_state_keys
            .insert(program.bytes(), (store_name, key));
        Ok(self)
    }

    /// Reads and locally re-verifies one complete current protocol state.
    ///
    /// # Errors
    ///
    /// Refuses a zero clock as a non-canonical view and surfaces transport,
    /// decode, and verification refusals from the node pair unchanged.
    pub fn read_protocol_state(
        &mut self,
        program: ProgramId,
        now: u64,
    ) -> Result<ProtocolProgramStateRead, ProgramReadError> {
        self.read_protocol_state_selected(program, now, false)
    }

    /// # Errors
    /// Returns an error if the program view is noncanonical or its state or balance proof cannot
    /// be acquired or verified.
    pub fn read_protocol_state_profile2(
        &mut self,
        program: ProgramId,
        now: u64,
    ) -> Result<ProtocolProgramStateRead, ProgramReadError> {
        self.read_protocol_state_selected(program, now, true)
    }

    fn read_protocol_state_selected(
        &mut self,
        program: ProgramId,
        now: u64,
        request_profile2: bool,
    ) -> Result<ProtocolProgramStateRead, ProgramReadError> {
        if now == 0 {
            return Err(ProgramReadError::NonCanonicalView);
        }
        let chain = self.read_chain_head(now).map_err(|error| match error {
            ChainHeadError::View(error) => error,
            ChainHeadError::Evidence(_) => ProgramReadError::NonCanonicalView,
        })?;
        let head = AccountStateHead {
            receipt_digest: chain.receipt_digest(),
            state_root: chain.state_root(),
            freshness: chain.freshness(),
        };
        let profile2 = request_profile2
            || self
                .registry
                .entry_for_wind_down(program)?
                .versions
                .last()
                .is_some_and(|version| matches!(version.abi_version, 3 | 4));
        let source = self
            .consensus
            .as_ref()
            .ok_or(ProgramReadError::NonCanonicalView)?;
        let (store_name, key) = self
            .program_state_keys
            .get(&program.bytes())
            .ok_or(ProgramReadError::NonCanonicalView)?;
        let trust = self
            .verifier
            .chain_trust()
            .ok_or(ProgramReadError::NonCanonicalView)?;
        let validators = trust
            .validators
            .validators()
            .iter()
            .map(|v| super::CommitValidator {
                address: v.address,
                public_key: v.public_key,
                voting_power: v.voting_power,
            })
            .collect::<Vec<_>>();
        let checkpoint = source
            .checkpoint(
                head.freshness.observed_sequence,
                &trust.chain_id,
                &validators,
            )
            .map_err(|_| ProgramReadError::NonCanonicalView)?;
        let bytes = source
            .program_state_record(program.bytes(), key, store_name, &checkpoint)
            .map_err(|_| ProgramReadError::CorruptRecord)?;
        if bytes.get(5..37) != Some(program.bytes().as_slice())
            || !bytes.starts_with(if profile2 { b"LXPS2" } else { b"LXPS1" })
        {
            return Err(ProgramReadError::CorruptRecord);
        }
        if profile2 {
            ProtocolProgramStateRead::restore_verified_profile2(
                &bytes,
                &mut self.registry,
                head,
                head,
                now,
                self.staleness_limit,
            )
            .map_err(ProgramReadError::from)
        } else {
            ProtocolProgramStateRead::restore_verified(
                &bytes,
                &mut self.registry,
                head,
                head,
                now,
                self.staleness_limit,
            )
            .map_err(ProgramReadError::from)
        }
    }

    /// # Errors
    /// Returns an error if the program view is noncanonical or its state or balance proof cannot
    /// be acquired or verified.
    pub fn read_verified_balances(
        &mut self,
        program: ProgramId,
        now: u64,
    ) -> Result<layerx_programs::VerifiedProgramBalanceRead, ProgramReadError> {
        self.read_protocol_state(program, now)
            .map(ProtocolProgramStateRead::into_balances)
    }

    pub(crate) fn value_account_ids(
        &self,
        program: ProgramId,
    ) -> Result<Vec<[u8; 32]>, ProgramReadError> {
        self.registry
            .value_account_bindings(program)
            .map(|bindings| bindings.iter().map(|binding| binding.account_id).collect())
            .map_err(ProgramReadError::AccountState)
    }

    /// Reads registered program accounts at the requested committed height.
    ///
    /// # Errors
    /// Refuses unknown programs and every block, transport or ABI refusal.
    pub fn read(
        &mut self,
        program: ProgramId,
        height: u64,
    ) -> Result<ProgramBalanceRead<CommittedProgramBalanceFreshness>, ProgramReadError> {
        let accounts: Vec<_> = self
            .registry
            .value_account_bindings(program)
            .map_err(|_| ProgramReadError::NonCanonicalView)?
            .iter()
            .map(|binding| binding.account_id)
            .collect();
        Self::read_at(&self.sequencer, program, &accounts, height)
            .map_err(|_| ProgramReadError::NonCanonicalView)
    }

    /// Reads an explicit set of accounts, validating their on-chain identities.
    ///
    /// # Errors
    /// Propagates RPC refusals and rejects absent or changing committed blocks,
    /// duplicate accounts, foreign accounts and non-canonical ABI answers.
    pub fn read_at(
        endpoint: &SequencerEndpoint,
        program: ProgramId,
        accounts: &[[u8; 32]],
        height: u64,
    ) -> Result<ProgramBalanceRead<CommittedProgramBalanceFreshness>, SequencerError> {
        read_committed_program_balances(endpoint, program, accounts, height)
    }

    /// Reads the current chain head through the same independent
    /// receipt-authority cross-check as the balances path and returns it as the
    /// opaque authenticated kind-5 coordinate head.
    ///
    /// # Errors
    ///
    /// Refuses a zero clock, transport, node/authority and coordinate
    /// disagreement as a non-canonical view, and surfaces protected-trust
    /// refusals of the head (including a superseded ordinary head) unchanged.
    pub fn read_chain_head(&self, now: u64) -> Result<VerifiedChainHead, ChainHeadError> {
        if now == 0 {
            return Err(ChainHeadError::View(ProgramReadError::NonCanonicalView));
        }
        let height = self
            .sequencer
            .block_number()
            .map_err(|_| ChainHeadError::View(ProgramReadError::NonCanonicalView))?;
        let evidence = self.native_head_evidence(height)?;
        let head = self
            .verifier
            .verify_current_chain_head(
                ProgramHeadKind::Ordinary,
                &evidence.commitment,
                &evidence.transaction,
                &evidence.transaction_proof,
                now,
            )
            .map_err(ChainHeadError::Evidence)?;
        if self
            .sequencer
            .block_number()
            .map_err(|_| ChainHeadError::View(ProgramReadError::NonCanonicalView))?
            != height
        {
            return Err(ChainHeadError::Evidence(ProtocolEvidenceError::Stale));
        }
        Ok(head)
    }

    /// Verifies retained coordinates at an explicit native block height.
    /// # Errors
    /// Returns an error if the chain head or deployment evidence does not match its verified view
    /// or trust anchor.
    pub fn read_chain_head_at(
        &self,
        height: u64,
        now: u64,
    ) -> Result<VerifiedChainHead, ChainHeadError> {
        if height == 0 || now == 0 {
            return Err(ChainHeadError::View(ProgramReadError::NonCanonicalView));
        }
        let evidence = self.native_head_evidence(height)?;
        self.verifier
            .verify_current_chain_head(
                ProgramHeadKind::Ordinary,
                &evidence.commitment,
                &evidence.transaction,
                &evidence.transaction_proof,
                now,
            )
            .map_err(ChainHeadError::Evidence)
    }

    fn native_head_evidence(&self, height: u64) -> Result<ChainHeadEvidence, ChainHeadError> {
        let view = || ChainHeadError::View(ProgramReadError::NonCanonicalView);
        let trust = self.verifier.chain_trust().ok_or(ChainHeadError::Evidence(
            ProtocolEvidenceError::InvalidTrustAnchor,
        ))?;
        let source = self.consensus.as_ref().ok_or_else(view)?;
        // A published set supplies proof material, never a new trust anchor.
        // The protected verifier below authenticates it before admitting a head.
        let validators = source.validators(height).map_err(|_| view())?;
        let checkpoint = source
            .checkpoint(height, &trust.chain_id, &validators)
            .map_err(|_| view())?;
        let block = self
            .sequencer
            .block_by_number(height)
            .map_err(|_| view())?
            .ok_or_else(view)?;
        if checkpoint.block_hash != block.hash {
            return Err(view());
        }
        let (transaction, transaction_proof) = source
            .sequence_batch(height, checkpoint.header.data_hash.ok_or_else(view)?)
            .map_err(|_| view())?;
        Ok(ChainHeadEvidence {
            commitment: source.host_commitment(&checkpoint).map_err(|_| view())?,
            transaction,
            transaction_proof,
        })
    }

    /// # Errors
    /// Returns an error if the chain head or deployment evidence does not match its verified view
    /// or trust anchor.
    pub fn verify_journal_admission(
        &self,
        proof: &DeploymentProof,
        now: u64,
    ) -> Result<VerifiedDeploymentEvidence, ChainHeadError> {
        if now == 0 {
            return Err(ChainHeadError::View(ProgramReadError::NonCanonicalView));
        }
        let height = self
            .sequencer
            .block_number()
            .map_err(|_| ChainHeadError::View(ProgramReadError::NonCanonicalView))?;
        let head = self.native_head_evidence(height)?;
        let trust = self.verifier.chain_trust().ok_or(ChainHeadError::Evidence(
            ProtocolEvidenceError::InvalidTrustAnchor,
        ))?;
        let admission_step = commitment_trust_step(&proof.state.commitment)
            .map_err(|_| ChainHeadError::Evidence(ProtocolEvidenceError::Encoding))?;
        let current_step = commitment_trust_step(&head.commitment)
            .map_err(|_| ChainHeadError::Evidence(ProtocolEvidenceError::Encoding))?;
        let history = (admission_step.0.height..current_step.0.height)
            .skip(1)
            .map(|height| {
                let refused =
                    |_| ChainHeadError::Evidence(ProtocolEvidenceError::HistoricalTrustAnchor);
                let height = u64::try_from(height).map_err(|_| {
                    ChainHeadError::Evidence(ProtocolEvidenceError::HistoricalTrustAnchor)
                })?;
                let block = self
                    .sequencer
                    .block_by_number(height)
                    .map_err(|_| ChainHeadError::View(ProgramReadError::NonCanonicalView))?
                    .ok_or(ChainHeadError::View(ProgramReadError::NonCanonicalView))?;
                let checkpoint = if let Some(source) = &self.consensus {
                    let validators = source.validators(height).map_err(refused)?;
                    let checkpoint = source
                        .checkpoint(height, &trust.chain_id, &validators)
                        .map_err(refused)?;
                    if checkpoint.block_hash != block.hash {
                        return Err(ChainHeadError::View(ProgramReadError::NonCanonicalView));
                    }
                    checkpoint
                } else {
                    super::ServedCheckpoint::acquire_at_committed_block(
                        &self.sequencer,
                        height,
                        Some(block.hash),
                        &trust.chain_id,
                    )
                    .map_err(refused)?
                };
                checkpoint_trust_step(checkpoint)
            });
        let bridge = historical_trust_chain(trust, admission_step, current_step, history)?;
        let current_verifier = ProtocolDeploymentVerifier::from_chain_trust(
            &bridge.verifier.chain_id,
            bridge.verifier.validators.validators().to_vec(),
            self.staleness_limit,
        )
        .map_err(ChainHeadError::Evidence)?;
        current_verifier
            .verify_current_chain_head(
                ProgramHeadKind::Ordinary,
                &head.commitment,
                &head.transaction,
                &head.transaction_proof,
                now,
            )
            .map_err(ChainHeadError::Evidence)?;
        self.verifier
            .verify_historical_deployment(proof, &bridge.head, &bridge.steps, now)
            .map_err(ChainHeadError::Evidence)
    }

    /// Decodes one kind-5 Programs state answer and binds it to the one
    /// authenticated chain head under the refreshed protected trust.
    ///
    /// # Errors
    ///
    /// Refuses non-canonical bytes and every head, signer, program, state,
    /// lifecycle, freshness and interface refusal of the bundle verifier.
    pub fn verify_program_bundle(
        &self,
        payload: &[u8],
        chain: &VerifiedChainHead,
        program: ProgramId,
        now: u64,
    ) -> Result<VerifiedProgramBundle, ProgramBundleError> {
        self.verifier
            .verify_current_program_bundle_bytes(payload, chain, program, now)
    }

    #[must_use]
    pub const fn staleness_limit(&self) -> u64 {
        self.staleness_limit
    }
}

type TrustStep = (ChainHeader, Commit, ValidatorSet);

fn checkpoint_trust_step(checkpoint: super::ServedCheckpoint) -> Result<TrustStep, ChainHeadError> {
    use layerx_programs::chain_proof::{
        header::{HeaderBlockId, HeaderTime, HeaderVersion},
        validators::TrustedValidator,
        vote::{BlockId, BlockIdFlag, CommitSig, PartSetHeader, Timestamp},
    };

    let refused = ChainHeadError::Evidence(ProtocolEvidenceError::Encoding);
    let h = checkpoint.header;
    let parent = h.last_block_id.ok_or(refused)?;
    let header = ChainHeader {
        version: HeaderVersion {
            block: h.version_block,
            app: h.version_app,
        },
        chain_id: h.chain_id,
        height: i64::try_from(h.height).map_err(|_| refused)?,
        time: HeaderTime {
            seconds: h.time.seconds,
            nanos: i32::try_from(h.time.nanos).map_err(|_| refused)?,
        },
        last_block_id: HeaderBlockId {
            hash: parent.hash.to_vec(),
            part_set_total: parent.part_set_total,
            part_set_hash: parent.part_set_hash.to_vec(),
        },
        last_commit_hash: h.last_commit_hash.ok_or(refused)?.to_vec(),
        data_hash: h.data_hash.ok_or(refused)?.to_vec(),
        validators_hash: h.validators_hash.to_vec(),
        next_validators_hash: h.next_validators_hash.to_vec(),
        consensus_hash: h.consensus_hash.to_vec(),
        app_hash: h.app_hash.to_vec(),
        last_results_hash: h.last_results_hash.ok_or(refused)?.to_vec(),
        evidence_hash: h.evidence_hash.ok_or(refused)?.to_vec(),
        proposer_address: h.proposer_address.to_vec(),
    };
    let commit = Commit {
        height: i64::try_from(checkpoint.commit.height).map_err(|_| refused)?,
        round: i32::try_from(checkpoint.commit.round).map_err(|_| refused)?,
        block_id: BlockId {
            hash: checkpoint.commit.block_id.hash.to_vec(),
            part_set_header: PartSetHeader {
                total: checkpoint.commit.block_id.part_set_total,
                hash: checkpoint.commit.block_id.part_set_hash.to_vec(),
            },
        },
        signatures: checkpoint
            .commit
            .votes
            .into_iter()
            .map(|vote| {
                Ok(CommitSig {
                    flag: BlockIdFlag::Commit,
                    validator_address: vote.validator_address.to_vec(),
                    timestamp: Timestamp {
                        seconds: vote.timestamp.seconds,
                        nanos: i32::try_from(vote.timestamp.nanos).map_err(|_| refused)?,
                    },
                    signature: vote.signature.to_vec(),
                })
            })
            .collect::<Result<_, ChainHeadError>>()?,
    };
    let validators = ValidatorSet::new(
        checkpoint
            .validators
            .into_iter()
            .map(|validator| TrustedValidator {
                address: validator.address,
                public_key: validator.public_key,
                voting_power: validator.voting_power,
            })
            .collect(),
    )
    .map_err(|_| refused)?;
    Ok((header, commit, validators))
}

struct HistoricalTrustChain {
    steps: Vec<TrustStep>,
    verifier: ChainProofVerifier,
    head: VerifiedHeader,
}

impl HistoricalTrustChain {
    fn push(&mut self, step: TrustStep) -> Result<(), ChainHeadError> {
        let refused = |_| ChainHeadError::Evidence(ProtocolEvidenceError::HistoricalTrustAnchor);
        self.verifier = self
            .verifier
            .advance(&self.head, &step.0, &step.1, step.2.clone())
            .map_err(refused)?;
        self.head = self
            .verifier
            .verify_head(&step.0, &step.1)
            .map_err(refused)?;
        self.steps.push(step);
        Ok(())
    }
}

fn historical_trust_chain(
    trust: &ChainProofVerifier,
    admission: TrustStep,
    current: TrustStep,
    history: impl IntoIterator<Item = Result<TrustStep, ChainHeadError>>,
) -> Result<HistoricalTrustChain, ChainHeadError> {
    const TRUST_PERIOD_MS: i128 = 1_209_600_000;
    let refused = ChainHeadError::Evidence(ProtocolEvidenceError::HistoricalTrustAnchor);
    if admission.2 != trust.validators || admission.0.height > current.0.height {
        return Err(refused);
    }
    let head = trust
        .verify_head(&admission.0, &admission.1)
        .map_err(|_| refused)?;
    let mut bridge = HistoricalTrustChain {
        steps: vec![admission.clone()],
        verifier: trust.clone(),
        head,
    };
    if admission.0.height == current.0.height {
        if admission != current {
            return Err(refused);
        }
        return Ok(bridge);
    }
    let time_ms = |step: &TrustStep| {
        i128::from(step.0.time.seconds) * 1_000 + i128::from(step.0.time.nanos / 1_000_000)
    };
    let current_height = current.0.height;
    let mut previous = admission;
    for step in history.into_iter().chain(std::iter::once(Ok(current))) {
        let step = step?;
        if previous.0.height.checked_add(1) != Some(step.0.height)
            || step.0.height > current_height
            || step.0.chain_id != trust.chain_id
            || time_ms(&step) <= time_ms(&previous)
        {
            return Err(refused);
        }
        let changed = step.2 != previous.2;
        if (changed || time_ms(&step) - i128::from(bridge.head.time_ms()) > TRUST_PERIOD_MS)
            && previous.0.height > bridge.head.height()
        {
            bridge.push(previous.clone())?;
        }
        if changed || step.0.height == current_height {
            bridge.push(step.clone())?;
        }
        previous = step;
    }
    Ok(bridge)
}

fn program_artifact_hex(text: &str) -> Result<Vec<u8>, ProgramArtifactsError> {
    if text.len() > 2_097_152
        || !text.len().is_multiple_of(2)
        || !text
            .bytes()
            .all(|byte| matches!(byte, b'0'..=b'9' | b'a'..=b'f'))
    {
        return Err(ProgramArtifactsError::Malformed);
    }
    text.as_bytes()
        .chunks_exact(2)
        .map(|pair| {
            let pair = std::str::from_utf8(pair).map_err(|_| ProgramArtifactsError::Malformed)?;
            u8::from_str_radix(pair, 16).map_err(|_| ProgramArtifactsError::Malformed)
        })
        .collect()
}

/// Agent service route that always refreshes from layerxd at request time.
pub struct ProgramBalanceReadRoute {
    reader: LayerxdProgramBalanceReader,
}

impl ProgramBalanceReadRoute {
    #[must_use]
    pub const fn new(reader: LayerxdProgramBalanceReader) -> Self {
        Self { reader }
    }

    /// Serves one committed balance read through the connected reader.
    ///
    /// # Errors
    ///
    /// Propagates every reader refusal unchanged.
    pub fn read(
        &mut self,
        program: ProgramId,
        height: u64,
    ) -> Result<ProgramBalanceRead<CommittedProgramBalanceFreshness>, ProgramReadError> {
        self.reader.read(program, height)
    }
}

#[cfg(test)]
fn chain_head_evidence(value: &Value) -> Result<ChainHeadEvidence, ProgramReadError> {
    let value = &value["chain_evidence"];
    let commitment = decode_commitment_proof(&chain_hex(field(value, "commitment_hex")?)?)
        .map_err(|_| ProgramReadError::NonCanonicalView)?;
    let transaction = chain_hex(field(value, "transaction_hex")?)?;
    let proof = &value["transaction_proof"];
    let total = proof["total"]
        .as_i64()
        .filter(|total| *total >= 0)
        .ok_or(ProgramReadError::NonCanonicalView)?;
    let index = proof["index"]
        .as_i64()
        .filter(|index| *index >= 0)
        .ok_or(ProgramReadError::NonCanonicalView)?;
    let leaf_hash = chain_hex(field(proof, "leaf_hash")?)?
        .try_into()
        .map_err(|_| ProgramReadError::NonCanonicalView)?;
    let aunts = proof["aunts"]
        .as_array()
        .filter(|aunts| aunts.len() <= 64)
        .ok_or(ProgramReadError::NonCanonicalView)?;
    let aunts = aunts
        .iter()
        .map(|aunt| {
            chain_hex(aunt.as_str().ok_or(ProgramReadError::NonCanonicalView)?)?
                .try_into()
                .map_err(|_| ProgramReadError::NonCanonicalView)
        })
        .collect::<Result<Vec<[u8; 32]>, _>>()?;
    Ok(ChainHeadEvidence {
        commitment,
        transaction,
        transaction_proof: SimpleProof {
            total,
            index,
            leaf_hash,
            aunts,
        },
    })
}

#[cfg(test)]
fn chain_hex(text: &str) -> Result<Vec<u8>, ProgramReadError> {
    if !text.len().is_multiple_of(2)
        || !text
            .bytes()
            .all(|byte| matches!(byte, b'0'..=b'9' | b'a'..=b'f'))
    {
        return Err(ProgramReadError::NonCanonicalView);
    }
    hex::decode(text).map_err(|_| ProgramReadError::NonCanonicalView)
}

#[cfg(test)]
fn field<'a>(value: &'a Value, name: &str) -> Result<&'a str, ProgramReadError> {
    value[name]
        .as_str()
        .ok_or(ProgramReadError::NonCanonicalView)
}

#[cfg(test)]
fn digest(bytes: &[u8]) -> [u8; 32] {
    use sha2::{Digest as _, Sha256};
    Sha256::digest(bytes).into()
}

fn secure_endpoint(endpoint: &str) -> bool {
    endpoint.starts_with("https://")
        || endpoint
            .strip_prefix("http://")
            .and_then(|value| value.split('/').next())
            .is_some_and(|host| {
                host == "localhost"
                    || host.starts_with("localhost:")
                    || host == "127.0.0.1"
                    || host.starts_with("127.0.0.1:")
                    || host == "[::1]"
                    || host.starts_with("[::1]:")
            })
}

#[cfg(test)]
#[path = "program_node_tests.rs"]
mod tests;

#[cfg(test)]
#[allow(dead_code)]
#[path = "../../../../../../programs/crates/layerx-programs-registry/tests/support/mod.rs"]
mod historical_deployment_support;

#[cfg(test)]
mod historical_deployment_tests {
    use super::*;
    use ed25519_dalek::{Signer, SigningKey};
    use historical_deployment_support::{verifier_for_fixture, NOW, WASM_V1};
    use layerx_programs::chain_proof::{
        merkle::hash_from_byte_slices,
        results::TxResult,
        validators::TrustedValidator,
        vote::{vote_sign_bytes, BlockIdFlag, CommitSig, Timestamp},
    };
    use layerx_programs::{
        chain_evidence::{deployment::ChainDeploymentProof, programs_state_key},
        DeploymentExecutionProof, ProgramInterface,
    };

    const SEEDS: [[u8; 32]; 4] = [[1; 32], [2; 32], [3; 32], [4; 32]];
    const ROTATED: [[u8; 32]; 4] = [[11; 32], [12; 32], [13; 32], [14; 32]];
    const DAY_SECONDS: i64 = 86_400;

    fn sign_fixture(chain: &mut historical_deployment_support::chain::ChainFixture) {
        chain.commit.height = chain.header.height;
        chain.commit.block_id.hash = chain.header.hash().expect("canonical header").to_vec();
        let timestamp = Timestamp {
            seconds: chain.header.time.seconds,
            nanos: chain.header.time.nanos,
        };
        let bytes = vote_sign_bytes(
            &chain.header.chain_id,
            chain.commit.height,
            chain.commit.round,
            Some(&chain.commit.block_id),
            timestamp,
        )
        .expect("canonical precommit");
        for signature in &mut chain.commit.signatures {
            let key = SEEDS
                .iter()
                .map(SigningKey::from_bytes)
                .find(|key| {
                    TrustedValidator::new(key.verifying_key().to_bytes(), 1)
                        .address
                        .as_slice()
                        == signature.validator_address
                })
                .expect("validator signing key");
            signature.timestamp = timestamp;
            signature.signature = key.sign(&bytes).to_bytes().to_vec();
        }
    }

    fn historical_deployment_fixture() -> historical_deployment_support::ProtocolFixture {
        let transcript = include_str!(
            "../../../../../../programs/crates/layerx-programs-registry/src/repair_22_31_256.hex"
        );
        assert_eq!(
            hex::encode(&digest(transcript.as_bytes())),
            "5798c9304ce5dd774f153da4f4f3208f1205f4cbfd1854732a49636dc9d76c6e"
        );
        let producer = transcript
            .lines()
            .map(|line| {
                let (key, value) = line.split_once('=').expect("producer recording row");
                (key, hex::decode(value).expect("producer recording bytes"))
            })
            .collect::<std::collections::BTreeMap<_, _>>();
        assert_eq!(producer["wasm.1"], WASM_V1);
        let interface =
            ProgramInterface::decode(&producer["interface.1"]).expect("producer interface");
        let mut fixture = historical_deployment_support::deploy_fixture_with_interface(
            WASM_V1,
            layerx_programs::UpgradePolicy::Authority(historical_deployment_support::AUTHORITY),
            100,
            NOW,
            &interface,
        );
        let program_key = fixture.proof.state.commitment.key.clone();
        let interface_key = programs_state_key(&layerx_programs::interface_state_key(
            fixture.proof.state.state.program,
        ));
        let transaction_root = hash_from_byte_slices(&[&digest(&producer["deploy"])]);
        let account = &producer["host.account"];
        let mut previous = historical_deployment_support::chain::ChainFixture::new(
            &fixture.chain.verifier.chain_id,
            &SEEDS,
            vec![(account[..33].to_vec(), account[33..].to_vec())],
        )
        .expect("producer predecessor state");
        previous.header.height = 100;
        previous.header.time.seconds = (NOW / 1_000) as i64 - 1;
        previous.header.time.nanos = 0;
        previous.header.data_hash = transaction_root.to_vec();
        sign_fixture(&mut previous);
        let result_root = hash_from_byte_slices(&[&producer["success.leaf"]]);
        assert_eq!(result_root.as_slice(), producer["success.root"]);
        fixture.chain.header.last_block_id.hash = previous.commit.block_id.hash.clone();
        fixture.chain.header.last_block_id.part_set_total =
            previous.commit.block_id.part_set_header.total;
        fixture.chain.header.last_block_id.part_set_hash =
            previous.commit.block_id.part_set_header.hash.clone();
        fixture.chain.header.last_results_hash = result_root.to_vec();
        fixture.chain.header.data_hash = transaction_root.to_vec();
        sign_fixture(&mut fixture.chain);
        fixture.proof.state.commitment = decode_commitment_proof(
            &fixture
                .chain
                .encode_key(&program_key)
                .expect("program commitment"),
        )
        .expect("decode program commitment");
        fixture.proof.deployment = ChainDeploymentProof::from_state_proofs(
            fixture.proof.state.state.clone(),
            WASM_V1.to_vec(),
            &decode_commitment_proof(
                &fixture
                    .chain
                    .encode_key(&interface_key)
                    .expect("interface commitment"),
            )
            .expect("decode interface commitment"),
        )
        .expect("authenticated deployment interface");
        fixture.proof.state.transaction_proof = SimpleProof {
            total: 1,
            index: 0,
            leaf_hash: transaction_root,
            aunts: Vec::new(),
        };
        fixture
            .proof
            .bind_execution(&DeploymentExecutionProof {
                transaction: producer["deploy"].clone(),
                previous: decode_commitment_proof(
                    &previous
                        .encode_key(&program_key)
                        .expect("predecessor commitment"),
                )
                .expect("decode predecessor commitment"),
                result: TxResult {
                    code: 0,
                    data: producer["success"].clone(),
                    gas_wanted: 0,
                    gas_used: 0,
                },
                result_proof: SimpleProof {
                    total: 1,
                    index: 0,
                    leaf_hash: result_root,
                    aunts: Vec::new(),
                },
                interface_ops: fixture
                    .chain
                    .prove(&interface_key)
                    .expect("interface proof"),
                migration: None,
                migration_ops: Vec::new(),
                migration_previous_ops: Vec::new(),
            })
            .expect("bind producer execution");
        fixture
    }

    #[test]
    fn historical_checkpoint_preserves_signed_header_commit_and_validators() {
        let checkpoint = super::super::ServedCheckpoint::test_fixture(100, 1_700_000_000, 0);
        let step = checkpoint_trust_step(checkpoint.clone()).expect("historical checkpoint step");
        let trust = ChainProofVerifier {
            chain_id: step.0.chain_id.clone(),
            validators: step.2.clone(),
        };
        let head = trust
            .verify_head(&step.0, &step.1)
            .expect("real precommit quorum");
        assert_eq!(head.header_hash(), checkpoint.block_hash);
        assert_eq!(head.height(), 100);
        assert_eq!(head.time_ms(), 1_700_000_000_000);
        assert_eq!(head.app_hash(), checkpoint.app_hash);
        assert_eq!(step.2.hash(), checkpoint.header.validators_hash);
        let mut missing_hash = checkpoint;
        missing_hash.header.data_hash = None;
        assert_eq!(
            checkpoint_trust_step(missing_hash).err(),
            Some(ChainHeadError::Evidence(ProtocolEvidenceError::Encoding))
        );
    }

    fn validators(seeds: &[[u8; 32]]) -> ValidatorSet {
        let mut validators = seeds
            .iter()
            .map(|seed| {
                TrustedValidator::new(SigningKey::from_bytes(seed).verifying_key().to_bytes(), 1)
            })
            .collect::<Vec<_>>();
        validators.sort_by_key(|validator| validator.address);
        ValidatorSet::new(validators).expect("real validator set")
    }

    fn next_step(
        previous: &TrustStep,
        seeds: &[[u8; 32]],
        seconds: i64,
        next: &ValidatorSet,
    ) -> TrustStep {
        let set = validators(seeds);
        let mut header = previous.0.clone();
        header.height += 1;
        header.time.seconds += seconds;
        header.last_block_id.hash = previous.1.block_id.hash.clone();
        header.last_block_id.part_set_total = previous.1.block_id.part_set_header.total;
        header.last_block_id.part_set_hash = previous.1.block_id.part_set_header.hash.clone();
        header.validators_hash = set.hash().to_vec();
        header.next_validators_hash = next.hash().to_vec();
        header.proposer_address = set.validators()[0].address.to_vec();
        let mut commit = previous.1.clone();
        commit.height = header.height;
        commit.block_id.hash = header.hash().expect("canonical header").to_vec();
        let timestamp = Timestamp {
            seconds: header.time.seconds,
            nanos: header.time.nanos,
        };
        let bytes = vote_sign_bytes(
            &header.chain_id,
            commit.height,
            commit.round,
            Some(&commit.block_id),
            timestamp,
        )
        .expect("canonical precommit");
        commit.signatures = set
            .validators()
            .iter()
            .map(|validator| {
                let key = seeds
                    .iter()
                    .map(SigningKey::from_bytes)
                    .find(|key| key.verifying_key().to_bytes() == validator.public_key)
                    .expect("validator signing key");
                CommitSig {
                    flag: BlockIdFlag::Commit,
                    validator_address: validator.address.to_vec(),
                    timestamp,
                    signature: key.sign(&bytes).to_bytes().to_vec(),
                }
            })
            .collect();
        (header, commit, set)
    }

    fn bridge(verifier: &ProtocolDeploymentVerifier, steps: &[TrustStep]) -> HistoricalTrustChain {
        historical_trust_chain(
            verifier.chain_trust().expect("pinned chain trust"),
            steps[0].clone(),
            steps.last().expect("current head").clone(),
            steps[1..steps.len() - 1].iter().cloned().map(Ok),
        )
        .unwrap_or_else(|error| panic!("historical trust bridge: {error:?}"))
    }

    #[test]
    fn historical_deployment_bridges_a_twenty_day_old_admission() {
        let fixture = historical_deployment_fixture();
        let verifier = verifier_for_fixture(&fixture, 5_000);
        let original = verifier
            .verify_deployment(&fixture.proof, NOW)
            .expect("admission");
        let admission =
            commitment_trust_step(&fixture.proof.state.commitment).expect("admission step");
        let mut steps = vec![admission];
        for _ in 0..20 {
            steps.push(next_step(
                steps.last().unwrap(),
                &SEEDS,
                DAY_SECONDS,
                &validators(&SEEDS),
            ));
        }
        let chain = bridge(&verifier, &steps);
        assert_eq!(
            chain
                .steps
                .iter()
                .map(|step| step.0.height)
                .collect::<Vec<_>>(),
            vec![101, 115, 121]
        );
        let now = NOW + 20 * DAY_SECONDS as u64 * 1_000;
        assert_eq!(
            verifier.verify_historical_deployment(
                &fixture.proof,
                &chain.head,
                &[steps[0].clone(), steps[20].clone()],
                now
            ),
            Err(ProtocolEvidenceError::HistoricalTrustAnchor)
        );
        assert_eq!(
            verifier
                .verify_historical_deployment(&fixture.proof, &chain.head, &chain.steps, now)
                .expect("twenty-day historical deployment"),
            original
        );
        assert_eq!(
            verifier.verify_historical_deployment(
                &fixture.proof,
                &chain.head,
                &chain.steps,
                now + 5_001
            ),
            Err(ProtocolEvidenceError::Stale)
        );
    }

    #[test]
    fn historical_deployment_bridges_a_full_non_adjacent_validator_rotation() {
        let fixture = historical_deployment_fixture();
        let verifier = verifier_for_fixture(&fixture, 5_000);
        let original = verifier
            .verify_deployment(&fixture.proof, NOW)
            .expect("admission");
        let mut steps =
            vec![commitment_trust_step(&fixture.proof.state.commitment).expect("admission step")];
        let rotated = validators(&ROTATED);
        assert!(rotated
            .validators()
            .iter()
            .all(|validator| !steps[0].2.validators().contains(validator)));
        for offset in 1..=20 {
            let seeds = if offset <= 10 { &SEEDS } else { &ROTATED };
            let next = if offset < 10 {
                validators(&SEEDS)
            } else {
                rotated.clone()
            };
            steps.push(next_step(steps.last().unwrap(), seeds, 1, &next));
        }
        let chain = bridge(&verifier, &steps);
        assert_eq!(
            chain
                .steps
                .iter()
                .map(|step| step.0.height)
                .collect::<Vec<_>>(),
            vec![101, 111, 112, 121]
        );
        let now = NOW + 20_000;
        assert_eq!(
            verifier.verify_historical_deployment(
                &fixture.proof,
                &chain.head,
                &[steps[0].clone(), steps[20].clone()],
                now
            ),
            Err(ProtocolEvidenceError::HistoricalTrustAnchor)
        );
        assert_eq!(
            verifier
                .verify_historical_deployment(&fixture.proof, &chain.head, &chain.steps, now)
                .expect("deployment across a full rotation"),
            original
        );
        let current_verifier = ProtocolDeploymentVerifier::from_chain_trust(
            &chain.verifier.chain_id,
            chain.verifier.validators.validators().to_vec(),
            5_000,
        )
        .expect("trust reached by the bridge");
        let mut current = fixture.chain;
        current.header = steps[20].0.clone();
        current.commit = steps[20].1.clone();
        current.verifier = chain.verifier;
        let commitment = decode_commitment_proof(&current.encode().expect("current commitment"))
            .expect("decode current commitment");
        assert_eq!(
            current_verifier
                .verify_current_chain_head(
                    ProgramHeadKind::Ordinary,
                    &commitment,
                    &DeploymentExecutionProof::decode(&fixture.proof.state.transaction)
                        .expect("deployment execution")
                        .transaction,
                    &fixture.proof.state.transaction_proof,
                    now
                )
                .expect("current head under advanced trust")
                .freshness()
                .observed_sequence,
            steps[20].0.height as u64
        );
        let mut forged = steps.clone();
        forged[10].1.signatures[0].signature[0] ^= 1;
        assert!(historical_trust_chain(
            verifier.chain_trust().unwrap(),
            forged[0].clone(),
            forged[20].clone(),
            forged[1..20].iter().cloned().map(Ok)
        )
        .is_err());
        assert!(historical_trust_chain(
            verifier.chain_trust().unwrap(),
            steps[0].clone(),
            steps[20].clone(),
            steps[1..20]
                .iter()
                .filter(|step| step.0.height != 111)
                .cloned()
                .map(Ok)
        )
        .is_err());
    }
}

/// Reads Programs ABI views at one committed block; never uses pending state.
///
/// # Errors
/// Propagates RPC failures and refuses malformed results, identity mismatches,
/// duplicate accounts, missing blocks and a changed block during the calls.
pub fn read_committed_program_balances(
    endpoint: &SequencerEndpoint,
    program: ProgramId,
    accounts: &[[u8; 32]],
    height: u64,
) -> Result<ProgramBalanceRead<CommittedProgramBalanceFreshness>, SequencerError> {
    crate::reject_if(
        height == 0
            || program.bytes() == [0; 32]
            || accounts.len() > layerx_programs::MAX_PROGRAM_VALUE_ACCOUNTS
            || accounts.contains(&[0; 32])
            || accounts
                .iter()
                .collect::<std::collections::BTreeSet<_>>()
                .len()
                != accounts.len(),
        || SequencerError::Malformed,
    )?;
    let block = endpoint
        .block_by_number(height)?
        .ok_or(SequencerError::Malformed)?;
    crate::reject_if(block.hash == [0; 32], || SequencerError::Malformed)?;
    let wind_down = programs_call(endpoint, height, "getWindDown(bytes32)", &[program.bytes()])?;
    crate::reject_if(wind_down.len() != 192, || SequencerError::Malformed)?;
    let lifecycle = match abi_integer(&wind_down[..32], 1)? {
        1 => ProgramLifecycle::Active,
        2 => ProgramLifecycle::Deprecated,
        3 => ProgramLifecycle::Tombstoned,
        _ => return Err(SequencerError::Malformed),
    };
    abi_integer(&wind_down[64..96], 8)?;
    abi_integer(&wind_down[96..128], 8)?;
    let count = abi_integer(&wind_down[128..160], 2)?;
    let live_count = abi_integer(&wind_down[160..192], 2)?;
    crate::reject_if(live_count > count, || SequencerError::Malformed)?;
    let mut values = Vec::with_capacity(accounts.len());
    for account in accounts {
        values.push(read_committed_program_balance(
            endpoint, program, account, height,
        )?);
    }
    let confirmed = endpoint
        .block_by_number(height)?
        .ok_or(SequencerError::Malformed)?;
    crate::reject_if(confirmed != block, || SequencerError::Malformed)?;
    Ok(ProgramBalanceRead {
        program: program.bytes(),
        lifecycle,
        accounts: values,
        freshness: CommittedProgramBalanceFreshness {
            height: block.number,
            block_hash: block.hash,
            timestamp: block.timestamp,
        },
    })
}
fn read_committed_program_balance(
    endpoint: &SequencerEndpoint,
    program: ProgramId,
    account: &[u8; 32],
    height: u64,
) -> Result<ProgramValueBalance, SequencerError> {
    let identity = programs_call(
        endpoint,
        height,
        "getProgramAccountById(bytes32)",
        &[*account],
    )?;
    // (bytes32 programId, bytes32 assetId, bytes seed), with one canonical tail.
    crate::reject_if(
        identity.len() < 128
            || identity.len() % 32 != 0
            || identity[..32] != program.bytes()
            || abi_integer(&identity[64..96], 8)? != 96,
        || SequencerError::Malformed,
    )?;
    let seed_len = usize::try_from(abi_integer(&identity[96..128], 8)?)
        .map_err(|_| SequencerError::Malformed)?;
    crate::reject_if(
        seed_len > layerx_programs_runtime::MAX_PROGRAM_ACCOUNT_SEED_BYTES
            || identity.len() != 128 + seed_len.div_ceil(32) * 32
            || identity[128 + seed_len..].iter().any(|byte| *byte != 0),
        || SequencerError::Malformed,
    )?;
    let derived =
        layerx_programs_runtime::derive_program_account(program, &identity[128..128 + seed_len])
            .map_err(|_| SequencerError::Malformed)?;
    crate::reject_if(!derived.matches(account), || SequencerError::Malformed)?;
    let asset = identity[32..64]
        .try_into()
        .map_err(|_| SequencerError::Malformed)?;
    let result = programs_call(
        endpoint,
        height,
        "getValueAccount(bytes32,bytes32)",
        &[program.bytes(), *account],
    )?;
    crate::reject_if(result.len() != 64, || SequencerError::Malformed)?;
    let amount = abi_integer(&result[..32], 16)?;
    let exists = match abi_integer(&result[32..], 1)? {
        0 => false,
        1 => true,
        _ => return Err(SequencerError::Malformed),
    };
    crate::reject_if(!exists && amount != 0, || SequencerError::Malformed)?;
    Ok(ProgramValueBalance {
        account: *account,
        asset,
        amount,
        frozen: None,
    })
}

fn programs_call(
    endpoint: &SequencerEndpoint,
    height: u64,
    signature: &str,
    words: &[[u8; 32]],
) -> Result<Vec<u8>, SequencerError> {
    let selector = layerx_crypto::evm_transaction::transaction_hash(signature.as_bytes());
    let mut data = selector[..4].to_vec();
    for word in words {
        data.extend_from_slice(word);
    }
    let mut address = [0; 20];
    address[18..].copy_from_slice(&[0x10, 0x1d]);
    endpoint.call_at(height, &address, &data)
}

fn abi_integer(word: &[u8], width: usize) -> Result<u128, SequencerError> {
    if word.len() != 32 || word[..32 - width].iter().any(|byte| *byte != 0) {
        return Err(SequencerError::Malformed);
    }
    let bytes = word[16..]
        .try_into()
        .map_err(|_| SequencerError::Malformed)?;
    Ok(u128::from_be_bytes(bytes))
}
