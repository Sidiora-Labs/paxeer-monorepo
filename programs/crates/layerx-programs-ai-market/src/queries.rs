//! F10 read-only selectors, exact snapshot binding and bounded off-chain page framing.
//! Selectors read one captured common state value; they never write, emit, pay or stage blobs.
//! Host `ProgramRead` facts and finality evidence are inputs; nothing here establishes them.
#[cfg(target_arch = "wasm32")]
use crate::codec::Sha256State as Sha256;
use crate::{
    codec::{self, ChunkResponse, ReadHeader, Reader, RosterView, Writer},
    dispatch,
    errors::{
        ApplicationError, CodecResult, ARITHMETIC, BAD_VERSION, CAPACITY, CONFLICT, NON_CANONICAL,
        NOT_FOUND, WRONG_DOMAIN, WRONG_MARKET, WRONG_PROGRAM, WRONG_ROSTER,
    },
    evaluators::authority::split_identity_section,
    registry_ops::PolicySection,
    state::{self, Section, SharedState},
    types::{
        AssetId, ChainDomain, Digest32, MarketId, MetadataDigest, PolicyDigest, Presence,
        PrincipalId, ProgramId, RequestDigest, RosterDigest, StateDigest, Version,
    },
    workers::{WorkerState, WorkerTable},
    MAX_CHUNK_BYTES, MAX_STATE_BYTES, SCHEMA_VERSION,
};
#[cfg(not(target_arch = "wasm32"))]
use sha2::{Digest, Sha256};

pub const FULL_CAPTURE_CHUNKS: usize = MAX_STATE_BYTES / MAX_CHUNK_BYTES;
pub const FINALIZED_RANK: u8 = 4;
pub const PAGE_MAX_ROWS: usize = 32;
pub const PAGE_MAX_BYTES: usize = 65_536;
pub const DEFAULT_LIMIT: u8 = 16;
pub const CURSOR_MAX_BYTES: usize = 1_024;
pub const CURSOR_LIFETIME_MS: u64 = 900_000;
/// schema:u16, visibility32, market32, snapshot32, filter-hash32 precede the ordinal.
const CURSOR_ORDINAL_OFFSET: usize = 2 + 4 * 32;
const CURSOR_PAYLOAD_BYTES: usize = CURSOR_ORDINAL_OFFSET + 2 + 8 + 4;
const CURSOR_TAG_BYTES: usize = 32;
pub const CURSOR_TOKEN_BYTES: usize = 2 * (CURSOR_PAYLOAD_BYTES + CURSOR_TAG_BYTES);
/// Content-identity prefix through `roster_digest` with epoch and roster present.
const BINDING_PREFIX_MAX_BYTES: usize = 2 + 3 * 32 + 8 + 8 + 32 + 32 + 8 + 32 + 9 + 8 + 32 + 33;
pub const BINDING_MAX_BYTES: usize = BINDING_PREFIX_MAX_BYTES + 8 + 32 + 32 + 32 + 1 + 8 + 8;
const HEX: &[u8; 16] = b"0123456789abcdef";

/// Off-chain view failures keep their own names, separate from on-chain application codes.
#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub enum QueryError {
    Application(ApplicationError),
    BindingMismatch,
    FinalityUnavailable,
    SnapshotConflict,
    IntegrityFailure,
    CursorExpired,
    CursorMismatch,
    ResponseTooLarge,
}
impl From<ApplicationError> for QueryError {
    fn from(error: ApplicationError) -> Self {
        Self::Application(error)
    }
}
pub type QueryResult<T> = Result<T, QueryError>;

/// The F02 worker table prefix of the identity section; the F03 region after it is not F10's.
fn worker_table(shared: &SharedState<'_>) -> CodecResult<WorkerTable> {
    WorkerTable::decode(split_identity_section(shared.section(Section::IdentityRoster)?)?.0)
}

fn policy_of<'s>(shared: &'s SharedState<'_>) -> CodecResult<PolicySection<'s>> {
    let bytes = shared.section(Section::PolicyLifecycle)?;
    if bytes.is_empty() {
        return Err(NOT_FOUND);
    }
    PolicySection::decode(bytes)
}

/// `READ_HEADER` over the exact captured state. Epoch and roster stay absent until a
/// frozen-roster producer records them in state; absence is never reported as epoch 0.
///
/// # Errors
/// Refuses a noncanonical state, an uncreated market (`NOT_FOUND`) and a chain or program
/// other than the market's own (`WRONG_DOMAIN`, `WRONG_PROGRAM`).
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn read_header(
    state_bytes: &[u8],
    chain: ChainDomain,
    program: ProgramId,
) -> CodecResult<ReadHeader> {
    let shared = state::decode_shared_state(state_bytes)?;
    let header = policy_of(&shared)?.header;
    if header.deployment_chain_domain != chain {
        return Err(WRONG_DOMAIN);
    }
    if header.program_id != program {
        return Err(WRONG_PROGRAM);
    }
    let opened = crate::epoch::captured_roster(&shared)?;
    Ok(ReadHeader {
        revision: shared.revision,
        digest: codec::state_digest(state_bytes)?,
        total_bytes: u32::try_from(state_bytes.len()).map_err(|_| ARITHMETIC)?,
        chain,
        program,
        market: header.market_id,
        epoch: opened
            .as_ref()
            .map_or(Presence::Absent, |r| Presence::Present(r.binding.epoch)),
        config: Version::new(header.active_config_version)?,
        roster: opened
            .as_ref()
            .map_or(Presence::Absent, |r| Presence::Present(r.binding.roster)),
    })
}

/// Digest of the market's current policy in the captured state.
///
/// # Errors
/// Refuses a noncanonical state or an uncreated market (`NOT_FOUND`).
pub fn policy_digest(state_bytes: &[u8]) -> CodecResult<PolicyDigest> {
    let shared = state::decode_shared_state(state_bytes)?;
    policy_of(&shared)?.current.digest()
}

/// `READ_STATE_CHUNK`: exact 46-byte payload, pinned revision/digest after discovery, and a
/// chunk that never crosses the state end. Writes the encoded chunk response body to `out`.
///
/// # Errors
/// `NON_CANONICAL` for a malformed payload or an offset outside the state, `CONFLICT` for a
/// pin that no longer matches the captured state, `CAPACITY` for an oversized state or `out`.
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn read_state_chunk(state_bytes: &[u8], payload: &[u8], out: &mut [u8]) -> CodecResult<usize> {
    dispatch::READ_STATE_CHUNK.validate_payload_length(payload.len())?;
    let request = codec::decode_chunk_request(payload)?;
    let shared = state::decode_shared_state(state_bytes)?;
    let digest = codec::state_digest(state_bytes)?;
    if request.revision != 0
        && (request.revision != shared.revision || request.digest != Presence::Present(digest))
    {
        return Err(CONFLICT);
    }
    let offset = usize::try_from(request.offset).map_err(|_| ARITHMETIC)?;
    let remaining = state_bytes
        .len()
        .checked_sub(offset)
        .filter(|n| *n > 0)
        .ok_or(NON_CANONICAL)?;
    let end = offset + usize::from(request.requested).min(remaining);
    let response = ChunkResponse {
        revision: shared.revision,
        digest,
        total_bytes: u32::try_from(state_bytes.len()).map_err(|_| ARITHMETIC)?,
        offset: request.offset,
        bytes: state_bytes.get(offset..end).ok_or(NON_CANONICAL)?,
    };
    codec::check_chunk_response(&request, &response)?;
    codec::encode_chunk_response(&response, out)
}

/// Wraps a read body in the common application result (at most 16384 bytes).
///
/// # Errors
/// `CAPACITY` when the body or `out` exceeds the common result bounds.
pub fn frame_read_result(
    request: RequestDigest,
    revision: u64,
    body: &[u8],
    out: &mut [u8],
) -> CodecResult<usize> {
    let result =
        codec::ApplicationResult::success(codec::ResultStatus::Ok, request, revision, body)?;
    codec::encode_result(&result, out)
}

/// Outer `ProgramRead` facts returned with each chunk by the host read interface.
#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct ReadProof {
    pub chain: ChainDomain,
    pub program: ProgramId,
    pub native_state_root: Digest32,
    pub observed_sequence: u64,
    pub execution_height: u64,
    pub block_hash: Digest32,
}

/// Facts of one complete capture: every chunk proved the same root and state identity.
#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct CaptureFacts {
    pub proof: ReadProof,
    pub revision: u64,
    pub digest: StateDigest,
    pub total_bytes: u32,
    pub chunks: usize,
}

/// Whole-capture reassembly into a caller buffer. Any head change, gap or short nonfinal
/// chunk fails the whole capture permanently; a retry restarts from one new root.
pub struct StateCapture<'a> {
    buffer: &'a mut [u8],
    facts: Option<CaptureFacts>,
    next_offset: usize,
    failed: bool,
}
impl<'a> StateCapture<'a> {
    pub fn new(buffer: &'a mut [u8]) -> Self {
        Self {
            buffer,
            facts: None,
            next_offset: 0,
            failed: false,
        }
    }
    /// Accepts the next chunk in offset order.
    ///
    /// # Errors
    /// `SnapshotConflict` when root, revision, digest or size differ from the first chunk,
    /// `IntegrityFailure` for a gap, reordering, short nonfinal chunk or an earlier failure,
    /// `CAPACITY` when the declared state exceeds the caller buffer.
    pub fn accept(&mut self, proof: &ReadProof, body: &[u8]) -> QueryResult<()> {
        if self.failed {
            return Err(QueryError::IntegrityFailure);
        }
        let outcome = self.append(proof, body);
        self.failed = outcome.is_err();
        outcome
    }
    fn append(&mut self, proof: &ReadProof, body: &[u8]) -> QueryResult<()> {
        let chunk = codec::decode_chunk_response(body)?;
        let total = usize::try_from(chunk.total_bytes).map_err(|_| ARITHMETIC)?;
        let facts = match self.facts {
            None if total > self.buffer.len() => return Err(CAPACITY.into()),
            None => CaptureFacts {
                proof: *proof,
                revision: chunk.revision,
                digest: chunk.digest,
                total_bytes: chunk.total_bytes,
                chunks: 0,
            },
            Some(f)
                if f.proof != *proof
                    || f.revision != chunk.revision
                    || f.digest != chunk.digest
                    || f.total_bytes != chunk.total_bytes =>
            {
                return Err(QueryError::SnapshotConflict)
            }
            Some(f) => f,
        };
        let offset = usize::try_from(chunk.offset).map_err(|_| ARITHMETIC)?;
        let end = offset + chunk.bytes.len();
        if offset != self.next_offset || (chunk.bytes.len() != MAX_CHUNK_BYTES && end != total) {
            return Err(QueryError::IntegrityFailure);
        }
        self.buffer
            .get_mut(offset..end)
            .ok_or(CAPACITY)?
            .copy_from_slice(chunk.bytes);
        self.next_offset = end;
        self.facts = Some(CaptureFacts {
            chunks: facts.chunks + 1,
            ..facts
        });
        Ok(())
    }
    /// Completes the capture and verifies the reconstructed bytes against the state digest.
    ///
    /// # Errors
    /// `IntegrityFailure` for a failed, empty or incomplete capture or a digest mismatch.
    pub fn finish(self) -> QueryResult<(&'a [u8], CaptureFacts)> {
        let facts = self
            .facts
            .filter(|_| !self.failed)
            .ok_or(QueryError::IntegrityFailure)?;
        let total = usize::try_from(facts.total_bytes).map_err(|_| ARITHMETIC)?;
        if self.next_offset != total || facts.chunks != total.div_ceil(MAX_CHUNK_BYTES) {
            return Err(QueryError::IntegrityFailure);
        }
        let bytes = self.buffer.get(..total).ok_or(CAPACITY)?;
        if codec::state_digest(bytes)? != facts.digest {
            return Err(QueryError::IntegrityFailure);
        }
        Ok((bytes, facts))
    }
}

/// Finality evidence supplied by the native block-commit verifier for one root.
#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct FinalityEvidence {
    pub native_state_root: Digest32,
    pub block_height: u64,
    pub block_hash: Digest32,
    pub app_hash: Digest32,
    pub store_proof_digest: Digest32,
    pub rank: u8,
}

/// `SnapshotBindingV2` in canonical field order.
#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct SnapshotBinding {
    pub chain: ChainDomain,
    pub program: ProgramId,
    pub market: MarketId,
    pub observed_sequence: u64,
    pub execution_height: u64,
    pub batch_id: Digest32,
    pub native_state_root: Digest32,
    pub revision: u64,
    pub state_digest: StateDigest,
    pub epoch: Presence<u64>,
    pub config: Version,
    pub policy: PolicyDigest,
    pub roster: Presence<RosterDigest>,
    pub block_height: u64,
    pub block_hash: Digest32,
    pub app_hash: Digest32,
    pub store_proof_digest: Digest32,
    pub rank: u8,
    pub publication_time_ms: u64,
}

/// Binds a verified capture to its header and the finality evidence for the SAME root.
///
/// # Errors
/// `BindingMismatch` when the evidence names another root or the state differs from the
/// capture size/count, `NON_CANONICAL` for a zero block or app hash or a rank above
/// finalized, and header refusals of [`read_header`]. Evidence rank is preserved.
pub fn bind_snapshot(
    state_bytes: &[u8],
    facts: &CaptureFacts,
    finality: &FinalityEvidence,
    publication_time_ms: u64,
) -> QueryResult<SnapshotBinding> {
    let header = read_header(state_bytes, facts.proof.chain, facts.proof.program)?;
    if header.revision != facts.revision
        || header.digest != facts.digest
        || header.total_bytes != facts.total_bytes
        || facts.chunks != state_bytes.len().div_ceil(MAX_CHUNK_BYTES)
        || finality.native_state_root != facts.proof.native_state_root
    {
        return Err(QueryError::BindingMismatch);
    }
    if finality.rank > FINALIZED_RANK
        || finality.block_hash.bytes() == [0; 32]
        || finality.app_hash.bytes() == [0; 32]
    {
        return Err(NON_CANONICAL.into());
    }
    Ok(SnapshotBinding {
        chain: header.chain,
        program: header.program,
        market: header.market,
        observed_sequence: facts.proof.observed_sequence,
        execution_height: facts.proof.execution_height,
        batch_id: facts.proof.block_hash,
        native_state_root: facts.proof.native_state_root,
        revision: header.revision,
        state_digest: header.digest,
        epoch: header.epoch,
        config: header.config,
        policy: policy_digest(state_bytes)?,
        roster: header.roster,
        block_height: finality.block_height,
        block_hash: finality.block_hash,
        app_hash: finality.app_hash,
        store_proof_digest: finality.store_proof_digest,
        rank: finality.rank,
        publication_time_ms,
    })
}

impl SnapshotBinding {
    /// Decodes the canonical transport used by clients and saved indexer bindings.
    ///
    /// # Errors
    /// `BAD_VERSION` for another schema; codec refusals for malformed, oversized,
    /// trailing or reserved bytes, revision zero, or a rank above finalized.
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn decode(bytes: &[u8]) -> CodecResult<Self> {
        let mut input = Reader::new(bytes);
        if input.u16()? != 1 {
            return Err(BAD_VERSION);
        }
        if bytes.len() > BINDING_MAX_BYTES {
            return Err(NON_CANONICAL);
        }
        let binding = SnapshotBinding {
            chain: input.chain_domain()?,
            program: input.program_id()?,
            market: input.market_id()?,
            observed_sequence: input.u64()?,
            execution_height: input.u64()?,
            batch_id: input.digest32()?,
            native_state_root: input.digest32()?,
            revision: input.u64()?,
            state_digest: input.state_digest()?,
            epoch: input.presence(Reader::u64)?,
            config: input.version()?,
            policy: input.policy_digest()?,
            roster: input.presence(|reader| reader.roster_digest())?,
            block_height: input.u64()?,
            block_hash: input.digest32()?,
            app_hash: input.digest32()?,
            store_proof_digest: input.digest32()?,
            rank: input.u8()?,
            publication_time_ms: input.u64()?,
        };
        input.reserved(8)?;
        input.finish()?;
        if binding.revision == 0 || binding.rank > FINALIZED_RANK {
            return Err(NON_CANONICAL);
        }
        Ok(binding)
    }

    /// Verifies a saved binding against a fresh capture of its exact producer state.
    /// No previous decode or snapshot identifier substitutes for the capture.
    ///
    /// # Errors
    /// Binding refusals or `BindingMismatch` if any saved content field differs.
    pub fn verify_capture(&self, state: &[u8], facts: &CaptureFacts) -> QueryResult<()> {
        let evidence = FinalityEvidence {
            native_state_root: self.native_state_root,
            block_height: self.block_height,
            block_hash: self.block_hash,
            app_hash: self.app_hash,
            store_proof_digest: self.store_proof_digest,
            rank: self.rank,
        };
        if bind_snapshot(state, facts, &evidence, self.publication_time_ms)? != *self {
            return Err(QueryError::BindingMismatch);
        }
        Ok(())
    }

    /// The exact committed head consumed by retention and webhook publication.
    ///
    /// # Errors
    /// `FinalityUnavailable` without rank 4; `BindingMismatch` when the commit
    /// names another height, block or app root, or the head has no height.
    pub fn finalized_head(&self) -> QueryResult<(u64, Digest32)> {
        self.require_finalized()?;
        if self.execution_height == 0
            || self.block_height != self.execution_height
            || self.block_hash != self.batch_id
            || self.app_hash != self.native_state_root
        {
            return Err(QueryError::BindingMismatch);
        }
        Ok((self.block_height, self.block_hash))
    }

    #[cfg_attr(target_arch = "wasm32", inline(never))]
    fn write_prefix(&self, w: &mut Writer<'_>) -> CodecResult<()> {
        w.u16(SCHEMA_VERSION)?;
        w.put(self.chain.as_bytes())?;
        w.put(self.program.as_bytes())?;
        w.put(self.market.as_bytes())?;
        w.u64(self.observed_sequence)?;
        w.u64(self.execution_height)?;
        w.put(self.batch_id.as_bytes())?;
        w.put(self.native_state_root.as_bytes())?;
        w.u64(self.revision)?;
        w.put(self.state_digest.as_bytes())?;
        w.presence(&self.epoch, |w, e| w.u64(*e))?;
        w.u64(self.config.get())?;
        w.put(self.policy.as_bytes())?;
        w.presence(&self.roster, |w, r| w.put(r.as_bytes()))
    }
    /// Canonical binding bytes; returns the encoded length.
    ///
    /// # Errors
    /// `NON_CANONICAL` for revision 0 or a rank above 4, `CAPACITY` for a short `out`.
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn encode(&self, out: &mut [u8]) -> CodecResult<usize> {
        if self.revision == 0 || self.rank > FINALIZED_RANK {
            return Err(NON_CANONICAL);
        }
        let mut w = Writer::new(out);
        self.write_prefix(&mut w)?;
        w.u64(self.block_height)?;
        w.put(self.block_hash.as_bytes())?;
        w.put(self.app_hash.as_bytes())?;
        w.put(self.store_proof_digest.as_bytes())?;
        w.u8(self.rank)?;
        w.u64(self.publication_time_ms)?;
        w.put(&[0; 8])?;
        Ok(w.len())
    }
    /// Content identity; excludes block-commit evidence, rank and publication time.
    ///
    /// # Errors
    /// Propagates prefix encoding failures.
    pub fn snapshot_id(&self) -> CodecResult<Digest32> {
        let mut prefix = [0u8; BINDING_PREFIX_MAX_BYTES];
        let mut w = Writer::new(&mut prefix);
        self.write_prefix(&mut w)?;
        let n = w.len();
        codec::domain_hash("PAXAI/view/v1", prefix.get(..n).ok_or(CAPACITY)?)
    }
    /// Finalized endpoints and webhook publication require actual rank-4 evidence.
    ///
    /// # Errors
    /// `FinalityUnavailable` unless rank is 4 and block and app hashes are nonzero.
    pub fn require_finalized(&self) -> QueryResult<()> {
        if self.rank == FINALIZED_RANK
            && self.block_hash.bytes() != [0; 32]
            && self.app_hash.bytes() != [0; 32]
        {
            Ok(())
        } else {
            Err(QueryError::FinalityUnavailable)
        }
    }
}

#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
#[repr(u8)]
pub enum Availability {
    Available = 1,
    NotEnabled = 2,
    NotYetProduced = 3,
    ContentUnavailable = 4,
    UnsupportedVersion = 5,
}

/// Availability of content actually decoded from the exact capture, in F01..F11 order.
/// Unknown codec versions and malformed content remain explicit to consumers.
pub fn feature_availability(state_bytes: &[u8]) -> CodecResult<[Availability; 11]> {
    fn decoded<T>(bytes: &[u8], decode: impl FnOnce() -> CodecResult<T>) -> Availability {
        if bytes.is_empty() {
            return Availability::NotYetProduced;
        }
        match decode() {
            Ok(_) => Availability::Available,
            Err(BAD_VERSION) => Availability::UnsupportedVersion,
            Err(_) => Availability::ContentUnavailable,
        }
    }
    let shared = state::decode_shared_state(state_bytes)?;
    let mut out = [Availability::NotYetProduced; 11];
    let policy = shared.section(Section::PolicyLifecycle)?;
    out[0] = decoded(policy, || policy_of(&shared));
    out[10] = match out[0] {
        Availability::Available => Availability::NotEnabled,
        availability => availability,
    };
    out[1] = decoded(shared.section(Section::IdentityRoster)?, || {
        worker_table(&shared)
    });
    let identity = split_identity_section(shared.section(Section::IdentityRoster)?);
    out[2] = match identity {
        Ok((_, bytes)) => decoded(bytes, || {
            crate::evaluators::authority::EvaluatorRegion::decode(bytes)
        }),
        Err(BAD_VERSION) => Availability::UnsupportedVersion,
        Err(_) => Availability::ContentUnavailable,
    };
    let reports = crate::evaluators::admission::ReportRegion::decode(
        shared.section(Section::CurrentReports)?,
    );
    out[3] = match reports {
        Ok(reports) => decoded(reports.rest(), || {
            crate::commit_reveal::CommitRegion::decode(reports.rest())
        }),
        Err(BAD_VERSION) => Availability::UnsupportedVersion,
        Err(_) => Availability::ContentUnavailable,
    };
    let settlement = shared.section(Section::SettlementClaims)?;
    if !settlement.is_empty() {
        out[5] = decoded(settlement, || {
            settlement
                .get(..crate::rewards::REWARD_STATE_BYTES)
                .ok_or(NON_CANONICAL)
                .and_then(crate::rewards::decode_reward_state)
        });
        if settlement.len() > crate::rewards::REWARD_STATE_BYTES {
            out[4] = decoded(&settlement[crate::rewards::REWARD_STATE_BYTES..], || {
                crate::aggregation::progress(state_bytes)
            });
        }
    }
    match crate::reputation_transition::split_joint(shared.section(Section::ReputationAdmission)?) {
        Ok((region, table)) => {
            out[6] = if region.is_some() {
                Availability::Available
            } else {
                Availability::NotYetProduced
            };
            out[7] = decoded(table, || crate::admission::AdmissionTable::decode(table));
        }
        Err(BAD_VERSION) => {
            out[6] = Availability::UnsupportedVersion;
            out[7] = Availability::ContentUnavailable;
        }
        Err(_) => {
            out[6] = Availability::ContentUnavailable;
            out[7] = Availability::ContentUnavailable;
        }
    }
    out[8] = decoded(shared.control.feature_bytes, || {
        crate::evidence::SealRegion::decode(shared.control.feature_bytes)
    });
    if !policy.is_empty() {
        out[9] = Availability::Available;
    }
    Ok(out)
}

#[derive(Clone, Copy, Eq, PartialEq, Ord, PartialOrd)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
#[repr(u8)]
pub enum ParticipantKind {
    Worker = 1,
    Evaluator = 2,
}

#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
#[repr(u8)]
pub enum ScoreStatus {
    Present = 1,
    NoAdmissibleScore = 2,
    InsufficientCoverage = 3,
    NotProduced = 4,
    Unavailable = 5,
    Unsupported = 6,
}

/// Present carries an actual number (zero included); every other status carries none.
#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct ScoreField {
    epoch: Presence<u64>,
    status: ScoreStatus,
    ppm: Presence<u32>,
}
impl ScoreField {
    /// # Errors
    /// `NON_CANONICAL` above 1000000 ppm.
    pub fn present(epoch: u64, ppm: u32) -> CodecResult<Self> {
        if ppm > 1_000_000 {
            return Err(NON_CANONICAL);
        }
        Ok(Self {
            epoch: Presence::Present(epoch),
            status: ScoreStatus::Present,
            ppm: Presence::Present(ppm),
        })
    }
    /// # Errors
    /// `NON_CANONICAL` for the present status, which must carry a number.
    pub fn absent(epoch: Presence<u64>, status: ScoreStatus) -> CodecResult<Self> {
        if status == ScoreStatus::Present {
            return Err(NON_CANONICAL);
        }
        Ok(Self {
            epoch,
            status,
            ppm: Presence::Absent,
        })
    }
    #[must_use]
    pub const fn status(&self) -> ScoreStatus {
        self.status
    }
    #[must_use]
    pub const fn ppm(&self) -> Presence<u32> {
        self.ppm
    }
    #[must_use]
    pub const fn epoch(&self) -> Presence<u64> {
        self.epoch
    }
}

/// F06-backed entitlement. Only an F06 producer may supply available values.
#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct RewardField {
    status: Availability,
    asset: Presence<AssetId>,
    earned: Presence<u128>,
    claimed: Presence<u128>,
}
impl RewardField {
    /// # Errors
    /// `NON_CANONICAL` when claimed exceeds earned.
    pub fn entitlement(asset: AssetId, earned: u128, claimed: u128) -> CodecResult<Self> {
        if claimed > earned {
            return Err(NON_CANONICAL);
        }
        Ok(Self {
            status: Availability::Available,
            asset: Presence::Present(asset),
            earned: Presence::Present(earned),
            claimed: Presence::Present(claimed),
        })
    }
    /// # Errors
    /// `NON_CANONICAL` for the available status, which must carry values.
    pub fn unavailable(status: Availability) -> CodecResult<Self> {
        if status == Availability::Available {
            return Err(NON_CANONICAL);
        }
        Ok(Self {
            status,
            asset: Presence::Absent,
            earned: Presence::Absent,
            claimed: Presence::Absent,
        })
    }
    #[must_use]
    pub const fn status(&self) -> Availability {
        self.status
    }
    #[must_use]
    pub const fn asset(&self) -> Presence<AssetId> {
        self.asset
    }
    #[must_use]
    pub const fn earned(&self) -> Presence<u128> {
        self.earned
    }
    #[must_use]
    pub const fn claimed(&self) -> Presence<u128> {
        self.claimed
    }
}

pub const ELIGIBLE_SERVING: u8 = 1;
pub const ELIGIBLE_NEW_WORK: u8 = 2;
/// Evaluator rows come from the frozen roster; no current evaluator record exists in state.
pub const IDENTITY_STATE_UNAVAILABLE: u8 = 0;

/// One participant row. For evaluators, `generation` is the frozen grant version.
#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct ParticipantRow {
    pub kind: ParticipantKind,
    pub id: [u8; 32],
    pub owner: PrincipalId,
    pub generation: u64,
    pub identity_state: u8,
    pub frozen_member: bool,
    pub frozen_generation: Presence<u64>,
    pub eligibility: u8,
    pub metadata: Presence<MetadataDigest>,
    pub metadata_revision: u64,
    pub score: ScoreField,
    pub reward: RewardField,
    pub history_status: Availability,
    pub history: Presence<Digest32>,
}
impl ParticipantRow {
    /// # Errors
    /// `CAPACITY` when the writer is full.
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn encode(&self, w: &mut Writer<'_>) -> CodecResult<()> {
        w.u8(self.kind as u8)?;
        w.put(&self.id)?;
        w.put(self.owner.as_bytes())?;
        w.u64(self.generation)?;
        w.u8(self.identity_state)?;
        w.boolean(self.frozen_member)?;
        w.presence(&self.frozen_generation, |w, g| w.u64(*g))?;
        w.u8(self.eligibility)?;
        w.presence(&self.metadata, |w, m| w.put(m.as_bytes()))?;
        w.u64(self.metadata_revision)?;
        w.presence(&self.score.epoch, |w, e| w.u64(*e))?;
        w.u8(self.score.status as u8)?;
        w.presence(&self.score.ppm, |w, s| w.u32(*s))?;
        w.u8(self.reward.status as u8)?;
        w.presence(&self.reward.asset, |w, a| w.put(a.as_bytes()))?;
        w.presence(&self.reward.earned, |w, v| w.u128(*v))?;
        w.presence(&self.reward.claimed, |w, v| w.u128(*v))?;
        w.u8(self.history_status as u8)?;
        w.presence(&self.history, |w, h| w.put(h.as_bytes()))
    }
    const fn active(&self) -> bool {
        self.eligibility & ELIGIBLE_SERVING != 0
    }
}

/// Complete projection from the captured F02/F03/F05/F06/F07/F08 producers.
/// An optional transported roster must bind exactly to the persisted opening.
pub fn participant_rows(
    state_bytes: &[u8],
    supplied: Option<&RosterView<'_>>,
    out: &mut [ParticipantRow],
) -> CodecResult<usize> {
    let shared = state::decode_shared_state(state_bytes)?;
    let policy = policy_of(&shared)?;
    let opened = crate::epoch::captured_roster(&shared)?;
    if let Some(view) = supplied {
        if view.market != policy.header.market_id {
            return Err(WRONG_MARKET);
        }
        let captured = opened.as_ref().ok_or(WRONG_ROSTER)?;
        if view.epoch != captured.binding.epoch
            || view.config != captured.binding.config
            || view.digest()? != captured.binding.roster
        {
            return Err(WRONG_ROSTER);
        }
    }
    let workers = worker_table(&shared)?;
    let evaluators =
        crate::evaluators::authority::evaluator_region(shared.section(Section::IdentityRoster)?)?;
    let (history, _) =
        crate::reputation_transition::split_joint(shared.section(Section::ReputationAdmission)?)?;
    let settlement = shared.section(Section::SettlementClaims)?;
    let rewards = if settlement.is_empty() {
        None
    } else {
        Some(crate::rewards::decode_reward_state(
            settlement
                .get(..crate::rewards::REWARD_STATE_BYTES)
                .ok_or(NON_CANONICAL)?,
        )?)
    };
    let aggregate = if opened.is_some() {
        crate::aggregation::captured_aggregation(state_bytes)?
    } else {
        None
    };
    let total = workers.len() + evaluators.len();
    let slots = out.get_mut(..total).ok_or(CAPACITY)?;
    let (worker_slots, evaluator_slots) = slots.split_at_mut(workers.len());
    for (slot, record) in worker_slots.iter_mut().zip(workers.iter()) {
        let frozen = opened
            .as_ref()
            .and_then(|r| r.workers().iter().find(|w| w.worker == record.worker));
        let epoch = opened
            .as_ref()
            .map_or(Presence::Absent, |r| Presence::Present(r.binding.epoch));
        let mut score = ScoreField::absent(epoch, ScoreStatus::NotProduced)?;
        if let (Some(roster), Some(aggregate)) = (&opened, &aggregate) {
            if aggregate.phase() == crate::aggregation_codec::AggregationPhase::Terminal {
                for i in 0..aggregate.output_count() {
                    let output = aggregate.output(i)?;
                    if output.worker() == record.worker {
                        score = match output.quality() {
                            Presence::Present(value) => {
                                ScoreField::present(roster.binding.epoch, value.get())?
                            }
                            Presence::Absent => {
                                ScoreField::absent(epoch, ScoreStatus::InsufficientCoverage)?
                            }
                        };
                    }
                }
            }
        }
        let mut reward = RewardField::unavailable(Availability::NotYetProduced)?;
        if let (Some(roster), Some(rewards)) = (&opened, &rewards) {
            let row = rewards.row(roster.binding.epoch)?;
            if matches!(
                row.status,
                crate::rewards::EpochStatus::Terminal | crate::rewards::EpochStatus::Expired
            ) {
                if row.outcome == crate::rewards::RewardOutcome::NoEligibleScore && frozen.is_some()
                {
                    reward = RewardField::entitlement(rewards.ledger()?.asset, 0, 0)?;
                }
                let dictionary = rewards.dictionary();
                for entry in row.entries() {
                    if dictionary.slot(entry.slot)?.worker == record.worker {
                        let claimed = if entry.disposition == crate::rewards::Disposition::Claimed {
                            entry.entitlement
                        } else {
                            0
                        };
                        reward = RewardField::entitlement(
                            rewards.ledger()?.asset,
                            entry.entitlement,
                            claimed,
                        )?;
                    }
                }
            }
        }
        let standing = history
            .as_ref()
            .and_then(|region| region.state.records().find(|r| r.worker == record.worker));
        *slot = ParticipantRow {
            kind: ParticipantKind::Worker,
            id: record.worker.bytes(),
            owner: record.owner,
            generation: record.generation,
            identity_state: record.state as u8,
            frozen_member: frozen.is_some(),
            frozen_generation: frozen
                .map_or(Presence::Absent, |w| Presence::Present(w.generation.get())),
            eligibility: match record.state {
                WorkerState::Available => ELIGIBLE_SERVING | ELIGIBLE_NEW_WORK,
                WorkerState::Enrolled | WorkerState::Draining => ELIGIBLE_SERVING,
                _ => 0,
            },
            metadata: Presence::Present(record.metadata),
            metadata_revision: record.metadata_revision,
            score,
            reward,
            history_status: if standing.is_some() {
                Availability::Available
            } else {
                Availability::NotYetProduced
            },
            history: standing.map_or(Presence::Absent, |r| Presence::Present(r.segment)),
        };
    }
    for (slot, record) in evaluator_slots.iter_mut().zip(evaluators.records()) {
        let grant = &record.grant;
        let frozen = opened.as_ref().and_then(|r| {
            r.evaluators()
                .iter()
                .find(|e| e.evaluator == grant.evaluator)
        });
        *slot = ParticipantRow {
            kind: ParticipantKind::Evaluator,
            id: grant.evaluator.bytes(),
            owner: grant.principal,
            generation: grant.grant_version.get(),
            identity_state: grant.status as u8,
            frozen_member: frozen.is_some(),
            frozen_generation: frozen
                .map_or(Presence::Absent, |e| Presence::Present(e.grant.get())),
            eligibility: 0,
            metadata: Presence::Absent,
            metadata_revision: 0,
            score: ScoreField::absent(Presence::Absent, ScoreStatus::NotProduced)?,
            reward: RewardField::unavailable(Availability::NotEnabled)?,
            history_status: Availability::NotEnabled,
            history: Presence::Absent,
        };
    }
    Ok(total)
}

/// The indexer's complete projection, pinned to the transported captured binding.
/// Used both when observing content and when serving a reopened persisted capture.
pub fn project_capture(
    state_bytes: &[u8],
    binding: &SnapshotBinding,
    out: &mut [ParticipantRow],
) -> QueryResult<([Availability; 11], usize)> {
    let header = read_header(state_bytes, binding.chain, binding.program)?;
    if header.market != binding.market
        || header.revision != binding.revision
        || header.digest != binding.state_digest
        || policy_digest(state_bytes)? != binding.policy
        || header.config != binding.config
        || header.epoch != binding.epoch
        || header.roster != binding.roster
    {
        return Err(QueryError::BindingMismatch);
    }
    Ok((
        feature_availability(state_bytes)?,
        participant_rows(state_bytes, None, out)?,
    ))
}

#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
#[repr(u8)]
pub enum KindFilter {
    All = 0,
    Worker = 1,
    Evaluator = 2,
}
impl KindFilter {
    /// # Errors
    /// `NON_CANONICAL` for an unknown filter.
    pub fn from_u8(value: u8) -> CodecResult<Self> {
        Ok(match value {
            0 => Self::All,
            1 => Self::Worker,
            2 => Self::Evaluator,
            _ => return Err(NON_CANONICAL),
        })
    }
    fn admits(self, row: &ParticipantRow, active_only: bool) -> bool {
        let kind = match self {
            Self::All => true,
            Self::Worker => row.kind == ParticipantKind::Worker,
            Self::Evaluator => row.kind == ParticipantKind::Evaluator,
        };
        kind && (!active_only || row.active())
    }
}

/// Exactly 64 lower-case hexadecimal characters.
///
/// # Errors
/// `NON_CANONICAL` for any other length or character.
pub fn parse_hex32(text: &str) -> CodecResult<[u8; 32]> {
    let mut out = [0u8; 32];
    hex_decode(text.as_bytes(), &mut out)?;
    Ok(out)
}
fn nibble(c: u8) -> CodecResult<u8> {
    match c {
        b'0'..=b'9' => Ok(c - b'0'),
        b'a'..=b'f' => Ok(c - b'a' + 10),
        _ => Err(NON_CANONICAL),
    }
}
fn hex_decode(text: &[u8], out: &mut [u8]) -> CodecResult<()> {
    if text.len() != out.len() * 2 {
        return Err(NON_CANONICAL);
    }
    for (o, pair) in out.iter_mut().zip(text.chunks_exact(2)) {
        *o = (nibble(pair[0])? << 4) | nibble(pair[1])?;
    }
    Ok(())
}

/// Canonical decimal limit, default 16, range 1..=32.
///
/// # Errors
/// `NON_CANONICAL` for a noncanonical decimal or a value outside 1..=32.
pub fn parse_limit(text: Option<&str>) -> CodecResult<u8> {
    let Some(text) = text else {
        return Ok(DEFAULT_LIMIT);
    };
    let limit = codec::decimal_u64(text)?;
    if limit == 0 || limit > PAGE_MAX_ROWS as u64 {
        return Err(NON_CANONICAL);
    }
    u8::try_from(limit).map_err(|_| ARITHMETIC)
}

/// Server key for cursor authentication; never placed in a snapshot.
#[derive(Clone, Copy)]
pub struct CursorKey<'k> {
    pub secret: &'k [u8; 32],
    pub generation: u32,
}
/// Visibility scope, snapshot and exact normalized filters a cursor is bound to.
#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct CursorScope {
    pub visibility: Digest32,
    pub market: MarketId,
    pub snapshot: Digest32,
    pub filter: KindFilter,
    pub active_only: bool,
}
fn hmac(secret: &[u8; 32], payload: &[u8]) -> [u8; 32] {
    let mut ipad = [0x36u8; 64];
    let mut opad = [0x5cu8; 64];
    for ((i, o), k) in ipad.iter_mut().zip(opad.iter_mut()).zip(secret) {
        *i ^= k;
        *o ^= k;
    }
    let mut inner = Sha256::new();
    inner.update(&ipad);
    inner.update(payload);
    let inner = inner.finalize();
    let mut outer = Sha256::new();
    outer.update(&opad);
    outer.update(&inner);
    outer.finalize().into()
}
fn cursor_payload(
    key: &CursorKey<'_>,
    scope: &CursorScope,
    next_ordinal: u16,
    expiry_ms: u64,
) -> CodecResult<[u8; CURSOR_PAYLOAD_BYTES]> {
    let filter = codec::domain_hash(
        "PAXAI/view-filter/v1",
        &[scope.filter as u8, u8::from(scope.active_only)],
    )?;
    let mut p = [0u8; CURSOR_PAYLOAD_BYTES];
    let mut w = Writer::new(&mut p);
    w.u16(SCHEMA_VERSION)?;
    w.put(scope.visibility.as_bytes())?;
    w.put(scope.market.as_bytes())?;
    w.put(scope.snapshot.as_bytes())?;
    w.put(filter.as_bytes())?;
    w.u16(next_ordinal)?;
    w.u64(expiry_ms)?;
    w.u32(key.generation)?;
    Ok(p)
}

/// Issues an opaque lower-case hex token: payload followed by HMAC-SHA256(secret, payload).
///
/// # Errors
/// `ARITHMETIC` when the expiry overflows.
pub fn issue_cursor<'o>(
    key: &CursorKey<'_>,
    scope: &CursorScope,
    next_ordinal: u16,
    now_ms: u64,
    out: &'o mut [u8; CURSOR_TOKEN_BYTES],
) -> CodecResult<&'o str> {
    let expiry = now_ms.checked_add(CURSOR_LIFETIME_MS).ok_or(ARITHMETIC)?;
    let payload = cursor_payload(key, scope, next_ordinal, expiry)?;
    let tag = hmac(key.secret, &payload);
    for (pair, b) in out.chunks_exact_mut(2).zip(payload.iter().chain(&tag)) {
        pair[0] = HEX[usize::from(b >> 4)];
        pair[1] = HEX[usize::from(b & 15)];
    }
    core::str::from_utf8(out).map_err(|_| NON_CANONICAL)
}

/// Authenticates a cursor before any row work and returns its next ordinal.
///
/// # Errors
/// `NON_CANONICAL` for a token that is not exactly one canonical cursor encoding,
/// `CursorMismatch` for a bad tag, another scope/key generation or an ordinal outside the
/// rows, `CursorExpired` at or after its expiry.
pub fn open_cursor(
    key: &CursorKey<'_>,
    scope: &CursorScope,
    token: &str,
    now_ms: u64,
    total_rows: usize,
) -> QueryResult<u16> {
    let mut raw = [0u8; CURSOR_PAYLOAD_BYTES + CURSOR_TAG_BYTES];
    hex_decode(token.as_bytes(), &mut raw)?;
    let (payload, tag) = raw.split_at(CURSOR_PAYLOAD_BYTES);
    let expected = hmac(key.secret, payload);
    if expected.iter().zip(tag).fold(0u8, |a, (x, y)| a | (x ^ y)) != 0 {
        return Err(QueryError::CursorMismatch);
    }
    let mut r = Reader::new(payload);
    r.take(CURSOR_ORDINAL_OFFSET)?;
    let next = r.u16()?;
    let expiry = r.u64()?;
    r.u32()?;
    r.finish()?;
    if payload[..] != cursor_payload(key, scope, next, expiry)?[..] {
        return Err(QueryError::CursorMismatch);
    }
    if expiry <= now_ms {
        return Err(QueryError::CursorExpired);
    }
    if next == 0 || usize::from(next) >= total_rows {
        return Err(QueryError::CursorMismatch);
    }
    Ok(next)
}

/// Selects one page from rows in kind-then-ID order. Returns rows written and the next
/// ordinal when more rows remain.
///
/// # Errors
/// `NON_CANONICAL` for a limit outside 1..=32 or rows not strictly in kind-then-ID order.
pub fn select_page(
    rows: &[ParticipantRow],
    filter: KindFilter,
    active_only: bool,
    start: u16,
    limit: u8,
    out: &mut [ParticipantRow; PAGE_MAX_ROWS],
) -> CodecResult<(usize, Option<u16>)> {
    if limit == 0 || usize::from(limit) > PAGE_MAX_ROWS {
        return Err(NON_CANONICAL);
    }
    if rows
        .windows(2)
        .any(|p| (p[0].kind, p[0].id) >= (p[1].kind, p[1].id))
    {
        return Err(NON_CANONICAL);
    }
    let mut written = 0usize;
    let admitted = rows
        .iter()
        .filter(|r| filter.admits(r, active_only))
        .enumerate()
        .skip(usize::from(start));
    for (ordinal, row) in admitted {
        let Some(slot) = out
            .get_mut(written)
            .filter(|_| written < usize::from(limit))
        else {
            return Ok((
                written,
                Some(u16::try_from(ordinal).map_err(|_| ARITHMETIC)?),
            ));
        };
        *slot = *row;
        written += 1;
    }
    Ok((written, None))
}

/// Off-chain page bound.
///
/// # Errors
/// `ResponseTooLarge` above 65536 bytes.
pub fn bound_response(length: usize) -> QueryResult<usize> {
    if length > PAGE_MAX_BYTES {
        Err(QueryError::ResponseTooLarge)
    } else {
        Ok(length)
    }
}

/// Page frame: schema, snapshot id, row count, rows, optional cursor token.
///
/// # Errors
/// `ResponseTooLarge` above 32 rows, a cursor above 1024 bytes or a frame above 65536 bytes;
/// `CAPACITY` for a short `out`.
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn encode_page(
    snapshot: Digest32,
    rows: &[ParticipantRow],
    cursor: Option<&str>,
    out: &mut [u8],
) -> QueryResult<usize> {
    if rows.len() > PAGE_MAX_ROWS || cursor.is_some_and(|c| c.len() > CURSOR_MAX_BYTES) {
        return Err(QueryError::ResponseTooLarge);
    }
    let mut w = Writer::new(out);
    w.u16(SCHEMA_VERSION)?;
    w.put(snapshot.as_bytes())?;
    w.u8(u8::try_from(rows.len()).map_err(|_| ARITHMETIC)?)?;
    for row in rows {
        row.encode(&mut w)?;
    }
    match cursor {
        None => w.boolean(false)?,
        Some(c) => {
            w.boolean(true)?;
            w.u16(u16::try_from(c.len()).map_err(|_| ARITHMETIC)?)?;
            w.put(c.as_bytes())?;
        }
    }
    bound_response(w.len())
}
