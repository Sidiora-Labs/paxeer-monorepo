//! Production ingestion of committed ProgramRead captures. The configured source supplies
//! whole same-root chunk responses and native membership plus signed block-commit evidence.
//! No source assertion, cached binding or elapsed time substitutes for that verification.
use std::path::Path;
use std::time::Duration;

use layerx_committed_proof::{
    commit::{BlockIdFlag, Commit, CommitSig},
    header::{BlockId, ConsensusVersion, Header, PartSetHeader, Timestamp, Validator},
};
use layerx_programs_ai_market::{
    codec,
    queries::{feature_availability, ReadProof, StateCapture},
    types::{ChainDomain, Digest32, ProgramId},
    MAX_STATE_BYTES,
};
use serde::{Deserialize, Serialize};

use crate::ai_market::{
    verify_block_commit, verify_program_state, Availability, BlockCommitProof, FinalityAuthority,
    Observation, ProjectionStore, ReadVerification, SnapshotRecord, ViewError,
};
use crate::ai_reputation::{FinalizedRead, ReputationError, ReputationProjector};
use crate::transport::{Endpoint, Security};
use crate::IndexError;

pub const COMMITTED_READ_MAX_BYTES: usize = 8 * 1024 * 1024;

#[derive(Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
struct BlockDocument {
    hash: Vec<u8>,
    total: u32,
    parts_hash: Vec<u8>,
}
impl From<&BlockId> for BlockDocument {
    fn from(value: &BlockId) -> Self {
        Self {
            hash: value.hash.clone(),
            total: value.part_set_header.total,
            parts_hash: value.part_set_header.hash.clone(),
        }
    }
}
impl From<BlockDocument> for BlockId {
    fn from(value: BlockDocument) -> Self {
        Self {
            hash: value.hash,
            part_set_header: PartSetHeader {
                total: value.total,
                hash: value.parts_hash,
            },
        }
    }
}

#[derive(Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
struct HeaderDocument {
    version: (u64, u64),
    chain_id: String,
    height: i64,
    time: (i64, u32),
    last_block_id: BlockDocument,
    last_commit_hash: Vec<u8>,
    data_hash: Vec<u8>,
    validators_hash: [u8; 32],
    next_validators_hash: Vec<u8>,
    consensus_hash: Vec<u8>,
    app_hash: Vec<u8>,
    last_results_hash: Vec<u8>,
    evidence_hash: Vec<u8>,
    proposer_address: Vec<u8>,
}
impl From<&Header> for HeaderDocument {
    fn from(h: &Header) -> Self {
        Self {
            version: (h.version.block, h.version.app),
            chain_id: h.chain_id.clone(),
            height: h.height,
            time: (h.time.seconds, h.time.nanos),
            last_block_id: (&h.last_block_id).into(),
            last_commit_hash: h.last_commit_hash.clone(),
            data_hash: h.data_hash.clone(),
            validators_hash: h.validators_hash,
            next_validators_hash: h.next_validators_hash.clone(),
            consensus_hash: h.consensus_hash.clone(),
            app_hash: h.app_hash.clone(),
            last_results_hash: h.last_results_hash.clone(),
            evidence_hash: h.evidence_hash.clone(),
            proposer_address: h.proposer_address.clone(),
        }
    }
}
impl From<HeaderDocument> for Header {
    fn from(h: HeaderDocument) -> Self {
        Self {
            version: ConsensusVersion {
                block: h.version.0,
                app: h.version.1,
            },
            chain_id: h.chain_id,
            height: h.height,
            time: Timestamp {
                seconds: h.time.0,
                nanos: h.time.1,
            },
            last_block_id: h.last_block_id.into(),
            last_commit_hash: h.last_commit_hash,
            data_hash: h.data_hash,
            validators_hash: h.validators_hash,
            next_validators_hash: h.next_validators_hash,
            consensus_hash: h.consensus_hash,
            app_hash: h.app_hash,
            last_results_hash: h.last_results_hash,
            evidence_hash: h.evidence_hash,
            proposer_address: h.proposer_address,
        }
    }
}

#[derive(Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
struct SignatureDocument {
    flag: u8,
    validator_address: Vec<u8>,
    time: (i64, u32),
    signature: Option<Vec<u8>>,
}
#[derive(Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
struct CommitDocument {
    height: u64,
    round: i32,
    block_id: BlockDocument,
    signatures: Vec<SignatureDocument>,
}
impl From<&Commit> for CommitDocument {
    fn from(c: &Commit) -> Self {
        Self {
            height: c.height,
            round: c.round,
            block_id: (&c.block_id).into(),
            signatures: c
                .signatures
                .iter()
                .map(|s| SignatureDocument {
                    flag: match s.block_id_flag {
                        BlockIdFlag::Absent => 1,
                        BlockIdFlag::Commit => 2,
                        BlockIdFlag::Nil => 3,
                    },
                    validator_address: s.validator_address.clone(),
                    time: (s.timestamp.seconds, s.timestamp.nanos),
                    signature: s.signature.map(Vec::from),
                })
                .collect(),
        }
    }
}
impl TryFrom<CommitDocument> for Commit {
    type Error = ViewError;
    fn try_from(c: CommitDocument) -> Result<Self, Self::Error> {
        Ok(Self {
            height: c.height,
            round: c.round,
            block_id: c.block_id.into(),
            signatures: c
                .signatures
                .into_iter()
                .map(|s| {
                    Ok(CommitSig {
                        block_id_flag: match s.flag {
                            1 => BlockIdFlag::Absent,
                            2 => BlockIdFlag::Commit,
                            3 => BlockIdFlag::Nil,
                            _ => return Err(ViewError::InvalidEncoding),
                        },
                        validator_address: s.validator_address,
                        timestamp: Timestamp {
                            seconds: s.time.0,
                            nanos: s.time.1,
                        },
                        signature: s
                            .signature
                            .map(|b| b.try_into().map_err(|_| ViewError::InvalidEncoding))
                            .transpose()?,
                    })
                })
                .collect::<Result<_, ViewError>>()?,
        })
    }
}

/// Versioned capture transport. Chunks are the producer's encoded read_state_chunk
/// responses, not a separately assembled JSON representation of application state.
#[derive(Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
struct CommittedReadDocument {
    version: u16,
    chain: [u8; 32],
    program: [u8; 32],
    observed_sequence: u64,
    header: HeaderDocument,
    commit: CommitDocument,
    store_proof: Vec<u8>,
    chunks: Vec<Vec<u8>>,
}

/// Producer adapter for a native committed ProgramRead and its exact chunk responses.
/// Receivers independently authenticate every field before changing a projection.
pub fn encode_committed_read(
    chain: ChainDomain,
    program: ProgramId,
    observed_sequence: u64,
    proof: &BlockCommitProof<'_>,
    chunks: &[Vec<u8>],
) -> Result<Vec<u8>, ViewError> {
    let bytes = serde_json::to_vec(&CommittedReadDocument {
        version: 1,
        chain: chain.bytes(),
        program: program.bytes(),
        observed_sequence,
        header: proof.header.into(),
        commit: proof.commit.into(),
        store_proof: proof.store_proof.to_vec(),
        chunks: chunks.to_vec(),
    })
    .map_err(|_| ViewError::InvalidEncoding)?;
    if bytes.len() > COMMITTED_READ_MAX_BYTES {
        return Err(ViewError::CapacityExceeded);
    }
    Ok(bytes)
}

/// One configured program, with durable market and F07 projections in the same database.
/// An interrupted admission is retried from the finalized market cursor; each consumer
/// checks the same evidence and admits replay idempotently.
pub struct AiRuntime {
    pub projections: ProjectionStore,
    pub reputation: ReputationProjector,
    authority: FinalityAuthority,
    chain: ChainDomain,
    program: ProgramId,
}

impl AiRuntime {
    pub fn open(
        path: &Path,
        authority: FinalityAuthority,
        chain: ChainDomain,
        program: ProgramId,
    ) -> Result<Self, IndexError> {
        let reputation = ReputationProjector::open(path, authority.clone(), chain, program)
            .map_err(reputation_error)?;
        Ok(Self {
            projections: ProjectionStore::open(path).map_err(view_error)?,
            reputation,
            authority,
            chain,
            program,
        })
    }

    /// Decode, authenticate and durably publish one complete capture. Invalid consensus,
    /// namespace membership, domain or chunks cannot publish even an observed outbox row.
    pub fn ingest(&mut self, bytes: &[u8], now_ms: u64) -> Result<SnapshotRecord, IndexError> {
        if bytes.len() > COMMITTED_READ_MAX_BYTES {
            return Err(view_error(ViewError::CapacityExceeded));
        }
        let document: CommittedReadDocument =
            serde_json::from_slice(bytes).map_err(|_| view_error(ViewError::InvalidEncoding))?;
        if document.version != 1 {
            return Err(view_error(ViewError::UnsupportedVersion));
        }
        if document.observed_sequence == 0 {
            return Err(view_error(ViewError::InvalidEncoding));
        }
        if document.chain != self.chain.bytes() || document.program != self.program.bytes() {
            return Err(view_error(ViewError::WrongDomain));
        }
        let header: Header = document.header.into();
        let commit: Commit = document.commit.try_into().map_err(view_error)?;
        let evidence = BlockCommitProof {
            header: &header,
            commit: &commit,
            store_proof: &document.store_proof,
        };
        // Validate the timestamp and header before any call that hashes the header.
        let verified = verify_block_commit(&evidence, &self.authority)
            .map_err(|_| view_error(ViewError::FinalityUnavailable))?;
        let proof = ReadProof {
            chain: self.chain,
            program: self.program,
            observed_sequence: document.observed_sequence,
            execution_height: verified.height,
            native_state_root: Digest32::new(verified.app_hash)
                .map_err(|_| view_error(ViewError::InvalidEncoding))?,
            block_hash: Digest32::new(verified.block_hash)
                .map_err(|_| view_error(ViewError::InvalidEncoding))?,
        };
        let mut buffer = vec![0; MAX_STATE_BYTES];
        let mut capture = StateCapture::new(&mut buffer);
        for chunk in &document.chunks {
            capture
                .accept(&proof, chunk)
                .map_err(|e| view_error(e.into()))?;
        }
        let (state, facts) = capture.finish().map_err(|e| view_error(e.into()))?;
        verify_program_state(&evidence, &self.authority, self.chain, self.program, state)
            .map_err(|_| view_error(ViewError::FinalityUnavailable))?;
        let market =
            codec::derive_market(self.chain, self.program).map_err(|e| view_error(e.into()))?;
        if self
            .projections
            .stream(&market)
            .map_err(view_error)?
            .and_then(|s| s.finalized_sequence)
            .is_some_and(|s| proof.observed_sequence < s)
        {
            return Err(view_error(ViewError::RollbackRefused));
        }
        let availability = feature_availability(state).map_err(|e| view_error(e.into()))?;
        if availability[6] == Availability::Available {
            self.reputation
                .admit(&FinalizedRead {
                    proof,
                    chunks: &document.chunks,
                    commit: evidence,
                    publication_time_ms: now_ms,
                })
                .map_err(reputation_error)?;
        }
        let activity = codec::domain_hash("PAXAI/committed-read/v1", &verified.block_hash)
            .map_err(|e| view_error(e.into()))?;
        let record = self
            .projections
            .observe(&Observation {
                state,
                facts,
                read: ReadVerification::SequencerSigned,
                source_activity: activity,
                observed_at_ms: now_ms,
            })
            .map_err(view_error)?;
        self.projections
            .promote(
                &record.snapshot_id,
                &BlockCommitProof {
                    header: &header,
                    commit: &commit,
                    store_proof: &document.store_proof,
                },
                &self.authority,
                now_ms,
            )
            .map_err(view_error)
    }

    /// One daemon iteration, including fresh source observation and durable readiness.
    pub fn daemon_poll(
        &mut self,
        observations: &crate::store::Store,
        source: &Endpoint,
        now_ms: u64,
    ) -> Result<Option<SnapshotRecord>, IndexError> {
        let result = self.poll(source, now_ms);
        match &result {
            Ok(_) => {
                let market = codec::derive_market(self.chain, self.program)
                    .map_err(|e| view_error(e.into()))?;
                let position = self
                    .projections
                    .stream(&market)
                    .map_err(view_error)?
                    .and_then(|s| s.finalized_sequence);
                if position.is_none() {
                    observations.record_source_failure("ai", "committed_projection_unavailable")?;
                    return Err(IndexError::Source(
                        "AI source idle before its first committed capture".to_owned(),
                    ));
                }
                observations.record_source_head("ai", position)?;
                observations.record_source_success("ai", position, position)?;
            }
            Err(_) => observations.record_source_failure("ai", "committed_read_unavailable")?,
        }
        result
    }

    /// Poll the committed-read source at the last durable finalized sequence. HTTP 204
    /// alone means idle; all other unavailable or malformed responses remain errors.
    pub fn poll(
        &mut self,
        source: &Endpoint,
        now_ms: u64,
    ) -> Result<Option<SnapshotRecord>, IndexError> {
        let market =
            codec::derive_market(self.chain, self.program).map_err(|e| view_error(e.into()))?;
        let after = self
            .projections
            .stream(&market)
            .map_err(view_error)?
            .and_then(|s| s.finalized_sequence)
            .unwrap_or(0);
        let path = format!(
            "/v1/ai/programs/{}/committed-read?after={after}",
            crate::codec::hex(&self.program.bytes())
        );
        let (status, body) = source.get(&path)?;
        match status {
            204 if body.is_empty() => Ok(None),
            200 => {
                let record = self.ingest(&body, now_ms)?;
                if record.observed_sequence <= after {
                    return Err(IndexError::Integrity(
                        "AI source did not advance its finalized cursor".to_owned(),
                    ));
                }
                Ok(Some(record))
            }
            _ => Err(IndexError::Source(format!(
                "AI committed read answered {status}"
            ))),
        }
    }
}

fn view_error(error: ViewError) -> IndexError {
    match error {
        ViewError::Store(detail) => IndexError::Store(detail),
        _ => IndexError::Integrity(format!("AI projection refused: {}", error.category())),
    }
}
fn reputation_error(error: ReputationError) -> IndexError {
    match error {
        ReputationError::Store(detail) => IndexError::Store(detail),
        _ => IndexError::Integrity(format!("AI reputation refused: {error:?}")),
    }
}

/// Operator-pinned trust and source configuration. Credentials do not confer consensus
/// authority: the configured validator set and exact native namespace proof do.
#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
pub struct RuntimeConfig {
    chain: [u8; 32],
    program: [u8; 32],
    consensus_chain_id: String,
    validators: Vec<ValidatorConfig>,
    source: String,
    ca_der: Option<std::path::PathBuf>,
    #[serde(default)]
    allow_remote_plaintext: bool,
}
#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct ValidatorConfig {
    pub_key: [u8; 32],
    voting_power: i64,
}
impl RuntimeConfig {
    pub fn read(path: &Path) -> Result<Self, IndexError> {
        let bytes = std::fs::read(path)
            .map_err(|e| IndexError::Config(format!("AI configuration: {e}")))?;
        serde_json::from_slice(&bytes)
            .map_err(|e| IndexError::Config(format!("AI configuration: {e}")))
    }
    pub fn open(self, database: &Path) -> Result<(AiRuntime, Endpoint), IndexError> {
        let chain = ChainDomain::new(self.chain)
            .map_err(|_| IndexError::Config("invalid AI chain".to_owned()))?;
        let program = ProgramId::new(self.program)
            .map_err(|_| IndexError::Config("invalid AI program".to_owned()))?;
        let authority = FinalityAuthority {
            chain_id: self.consensus_chain_id,
            validators: self
                .validators
                .into_iter()
                .map(|v| Validator {
                    pub_key: v.pub_key,
                    voting_power: v.voting_power,
                })
                .collect(),
        };
        let security = match self.ca_der {
            Some(path) => Security::PinnedTls(
                std::fs::read(path)
                    .map_err(|e| IndexError::Config(format!("AI trust anchor: {e}")))?,
            ),
            None => Security::Plaintext {
                allow_remote: self.allow_remote_plaintext,
            },
        };
        let endpoint = Endpoint::parse(&self.source, security, Duration::from_secs(10))?;
        Ok((
            AiRuntime::open(database, authority, chain, program)?,
            endpoint,
        ))
    }
}
