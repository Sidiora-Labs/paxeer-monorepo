use sha2::{Digest, Sha256};

use crate::replay::{append, ReplayWitnessError};

pub const REPLAY_MEMORY_CHUNK_DOMAIN: &[u8] = b"LXP/program-replay-memory-chunk/v1\0";
pub const REPLAY_MEMORY_NODE_DOMAIN: &[u8] = b"LXP/program-replay-memory-node/v1\0";
pub const REPLAY_MEMORY_ROOT_DOMAIN: &[u8] = b"LXP/program-replay-memory-root/v1\0";

pub const PROGRAM_REPLAY_RECORD_DOMAIN: &[u8] = b"LXP/program-replay-record/v1\0";
pub const PROGRAM_REPLAY_WITNESS_DOMAIN: &[u8] = b"LXP/program-replay-witness/v1\0";
pub const MAX_PROGRAM_REPLAY_BYTES: u32 = 1024 * 1024;
pub const MAX_PROGRAM_REPLAY_BOUNDARIES: u32 = 4096;

pub(crate) struct ReplayCaptureError {
    pub error: ReplayWitnessError,
    pub check: &'static str,
    pub observed: u128,
    pub permitted: u128,
    pub leaf_length: usize,
}

impl std::fmt::Debug for ReplayCaptureError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(
            f,
            "{:?}: check={} observed={} permitted={} leaf_length={}",
            self.error, self.check, self.observed, self.permitted, self.leaf_length
        )
    }
}

impl From<ReplayCaptureError> for ReplayWitnessError {
    fn from(error: ReplayCaptureError) -> Self {
        error.error
    }
}

fn capture_append(
    out: &mut Vec<u8>,
    bytes: &[u8],
    maximum: usize,
    check: &'static str,
    leaf_length: usize,
) -> Result<(), ReplayCaptureError> {
    let observed = out.len() as u128 + bytes.len() as u128;
    append(out, bytes, maximum).map_err(|error| ReplayCaptureError {
        error,
        check,
        observed,
        permitted: maximum as u128,
        leaf_length,
    })
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct ProgramReplayProfile {
    maximum_boundaries: u32,
    maximum_bytes: u32,
    policy: crate::TracePolicy,
}

impl ProgramReplayProfile {
    /// # Errors
    ///
    /// Returns a replay refusal when bounds, record framing, versioned identity, or witness commitments are invalid.
    pub fn new(maximum_boundaries: u32, maximum_bytes: u32) -> Result<Self, ReplayWitnessError> {
        if !(2..=MAX_PROGRAM_REPLAY_BOUNDARIES).contains(&maximum_boundaries)
            || !(512..=MAX_PROGRAM_REPLAY_BYTES).contains(&maximum_bytes)
        {
            return Err(ReplayWitnessError::Bounds);
        }
        let policy = crate::TracePolicy::new(1, maximum_boundaries)
            .map_err(|_| ReplayWitnessError::Bounds)?;
        Ok(Self {
            maximum_boundaries,
            maximum_bytes,
            policy,
        })
    }
    #[must_use]
    pub const fn maximum_boundaries(self) -> u32 {
        self.maximum_boundaries
    }
    #[must_use]
    pub const fn maximum_bytes(self) -> u32 {
        self.maximum_bytes
    }
    #[must_use]
    pub const fn trace_policy(self) -> crate::TracePolicy {
        self.policy
    }
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ReplayRecordHeader {
    pub identity: crate::ArbitrationExecutionIdentity,
    pub execution_parameters_digest: [u8; 32],
    pub storage_baseline: Vec<u8>,
    pub semantic_constants: Vec<u8>,
    pub composition_graph: Option<Vec<u8>>,
    pub response_capacity: Option<u64>,
}

impl ReplayRecordHeader {
    /// # Errors
    ///
    /// Returns a replay refusal when bounds, record framing, versioned identity, or witness commitments are invalid.
    pub fn from_boundary(
        arbitration: &crate::ArbitrationExecutionState,
        semantic: &[u8],
        maximum: usize,
    ) -> Result<Self, ReplayWitnessError> {
        let header = Self {
            identity: arbitration.identity,
            execution_parameters_digest: arbitration.legacy.execution_parameters_digest,
            storage_baseline: crate::replay::semantic_storage_baseline(semantic, maximum)?,
            semantic_constants: crate::replay::semantic_record_constants(semantic, maximum)?,
            composition_graph: crate::replay::semantic_composition_graph(semantic, maximum)?,
            response_capacity: crate::replay::semantic_response_capacity(semantic, maximum)?,
        };
        if arbitration.legacy.module_code_hash != header.identity.module_code_hash
            || arbitration.legacy.input_digest != header.identity.input_digest
        {
            return Err(ReplayWitnessError::Binding);
        }
        header.validate()?;
        Ok(header)
    }
    fn validate(&self) -> Result<(), ReplayWitnessError> {
        let identity = self.identity;
        let mut parameters = b"LXP/program-trace-parameters/v1\0".to_vec();
        parameters.extend(identity.runtime_version.to_be_bytes());
        parameters.extend(identity.abi_version.to_be_bytes());
        parameters.extend(identity.fee_schedule_version.to_be_bytes());
        parameters.extend(identity.metering_schedule_version.to_be_bytes());
        parameters.extend(identity.trace_policy.canonical_bytes());
        if <[u8; 32]>::from(Sha256::digest(parameters)) != self.execution_parameters_digest {
            return Err(ReplayWitnessError::Binding);
        }
        let baseline = crate::Storage::decode_untrusted_replay_state(
            &self.storage_baseline,
            MAX_PROGRAM_REPLAY_BYTES as usize,
        )?;
        if crate::replay::market_sandbox_baseline_root(&baseline)? != identity.host_base_state_root
        {
            return Err(ReplayWitnessError::Binding);
        }
        if let Some(graph) = &self.composition_graph {
            crate::CallGraph::decode_untrusted_replay_state(
                graph,
                MAX_PROGRAM_REPLAY_BYTES as usize,
            )?;
        }
        if self
            .response_capacity
            .is_some_and(|capacity| capacity > crate::abi::MAX_CALL_RESPONSE_BYTES as u64)
        {
            return Err(ReplayWitnessError::Bounds);
        }
        Ok(())
    }
    /// # Errors
    ///
    /// Returns a replay refusal when bounds, record framing, versioned identity, or witness commitments are invalid.
    pub fn canonical_bytes(&self, maximum: usize) -> Result<Vec<u8>, ReplayWitnessError> {
        self.validate()?;
        let mut out = Vec::new();
        let i = self.identity;
        append(&mut out, &i.module_code_hash, maximum)?;
        append(&mut out, &i.input_digest, maximum)?;
        append(&mut out, &i.runtime_version.to_be_bytes(), maximum)?;
        append(&mut out, &i.abi_version.to_be_bytes(), maximum)?;
        append(&mut out, &i.fee_schedule_version.to_be_bytes(), maximum)?;
        append(
            &mut out,
            &i.metering_schedule_version.to_be_bytes(),
            maximum,
        )?;
        append(&mut out, &self.execution_parameters_digest, maximum)?;
        append(&mut out, &i.trace_policy.canonical_bytes(), maximum)?;
        for root in [
            i.host_base_state_root,
            i.receipt_oracle_root,
            i.balance_oracle_root,
        ] {
            append(&mut out, &root, maximum)?;
        }
        append(
            &mut out,
            &u32::try_from(self.storage_baseline.len())
                .map_err(|_| ReplayWitnessError::Bounds)?
                .to_be_bytes(),
            maximum,
        )?;
        append(&mut out, &self.storage_baseline, maximum)?;
        append(
            &mut out,
            &u32::try_from(self.semantic_constants.len())
                .map_err(|_| ReplayWitnessError::Bounds)?
                .to_be_bytes(),
            maximum,
        )?;
        append(&mut out, &self.semantic_constants, maximum)?;
        append(
            &mut out,
            &[u8::from(self.composition_graph.is_some())],
            maximum,
        )?;
        if let Some(graph) = &self.composition_graph {
            append(
                &mut out,
                &u32::try_from(graph.len())
                    .map_err(|_| ReplayWitnessError::Bounds)?
                    .to_be_bytes(),
                maximum,
            )?;
            append(&mut out, graph, maximum)?;
        }
        append(
            &mut out,
            &[u8::from(self.response_capacity.is_some())],
            maximum,
        )?;
        if let Some(capacity) = self.response_capacity {
            append(&mut out, &capacity.to_be_bytes(), maximum)?;
        }
        Ok(out)
    }
    /// # Errors
    ///
    /// Returns a replay refusal when bounds, record framing, versioned identity, or witness commitments are invalid.
    pub fn decode(bytes: &[u8], maximum: usize) -> Result<Self, ReplayWitnessError> {
        if maximum == 0 || maximum > MAX_PROGRAM_REPLAY_BYTES as usize || bytes.len() > maximum {
            return Err(ReplayWitnessError::Bounds);
        }
        let mut c = crate::replay::ReplayCursor::new(bytes);
        let module_code_hash = c.array()?;
        let input_digest = c.array()?;
        let runtime_version = c.u16()?;
        let abi_version = c.u16()?;
        let fee_schedule_version = c.u32()?;
        let metering_schedule_version = c.u32()?;
        let execution_parameters_digest = c.array()?;
        let trace_policy =
            crate::TracePolicy::new(c.u64()?, c.u32()?).map_err(|_| ReplayWitnessError::Bounds)?;
        let host_base_state_root = c.array()?;
        let receipt_oracle_root = c.array()?;
        let balance_oracle_root = c.array()?;
        let storage_baseline = c.owned_field(maximum)?;
        let semantic_constants = c.owned_field(maximum)?;
        let composition_graph = if c.boolean()? {
            Some(c.owned_field(maximum)?)
        } else {
            None
        };
        let response_capacity = if c.boolean()? { Some(c.u64()?) } else { None };
        if !c.done() {
            return Err(ReplayWitnessError::Encoding);
        }
        let header = Self {
            identity: crate::ArbitrationExecutionIdentity {
                module_code_hash,
                input_digest,
                runtime_version,
                abi_version,
                fee_schedule_version,
                metering_schedule_version,
                trace_policy,
                host_base_state_root,
                receipt_oracle_root,
                balance_oracle_root,
            },
            execution_parameters_digest,
            storage_baseline,
            semantic_constants,
            composition_graph,
            response_capacity,
        };
        header.validate()?;
        Ok(header)
    }
    /// # Errors
    ///
    /// Returns a replay refusal when bounds, record framing, versioned identity, or witness commitments are invalid.
    pub fn digest(&self) -> Result<[u8; 32], ReplayWitnessError> {
        let mut hash = Sha256::new();
        hash.update(b"LXP/program-replay-header/v2\0");
        hash.update(self.canonical_bytes(MAX_PROGRAM_REPLAY_BYTES as usize)?);
        Ok(hash.finalize().into())
    }
    /// # Errors
    ///
    /// Returns a replay refusal when bounds, record framing, versioned identity, or witness commitments are invalid.
    pub fn boundary_envelope(
        &self,
        leaf: &[u8],
        maximum: usize,
    ) -> Result<Vec<u8>, ReplayWitnessError> {
        let mut out = Vec::new();
        append(
            &mut out,
            b"LXP/program-replay-boundary-context/v2\0",
            maximum,
        )?;
        let header = self.canonical_bytes(maximum)?;
        append(
            &mut out,
            &u32::try_from(header.len())
                .map_err(|_| ReplayWitnessError::Bounds)?
                .to_be_bytes(),
            maximum,
        )?;
        append(&mut out, &header, maximum)?;
        append(&mut out, leaf, maximum)?;
        Ok(out)
    }
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ProgramReplayRecord {
    header: ReplayRecordHeader,
    profile: ProgramReplayProfile,
    code_hash: [u8; 32],
    input_digest: [u8; 32],
    runtime_version: u16,
    abi_version: u16,
    fee_schedule_version: u32,
    metering_schedule_version: u32,
    terminal_status: u8,
    boundary_count: u32,
    boundary_root: [u8; 32],
    witness_digest: [u8; 32],
    replay_fuel: u64,
    bytes: Vec<u8>,
}

#[derive(Clone, Copy)]
pub(crate) struct CapturedRecordParameters {
    pub(crate) profile: ProgramReplayProfile,
    pub(crate) code_hash: [u8; 32],
    pub(crate) input_digest: [u8; 32],
    pub(crate) runtime_version: u16,
    pub(crate) abi_version: u16,
    pub(crate) fee_schedule_version: u32,
    pub(crate) metering_schedule_version: u32,
    pub(crate) terminal_status: u8,
}
fn capture_diagnostic(
    error: ReplayWitnessError,
    check: &'static str,
    observed: u128,
    permitted: u128,
    leaf_length: usize,
) -> ReplayCaptureError {
    ReplayCaptureError {
        error,
        check,
        observed,
        permitted,
        leaf_length,
    }
}

fn append_captured_leaf(
    witness: &mut Vec<u8>,
    leaf: &[u8],
    index: usize,
    header: &ReplayRecordHeader,
    maximum: usize,
    replay_fuel: &mut u64,
) -> Result<[u8; 32], ReplayCaptureError> {
    let length = u32::try_from(leaf.len()).map_err(|_| {
        capture_diagnostic(
            ReplayWitnessError::Bounds,
            "from_captured leaf length",
            leaf.len() as u128,
            u128::from(u32::MAX),
            leaf.len(),
        )
    })?;
    capture_append(
        witness,
        &length.to_be_bytes(),
        maximum,
        "from_captured witness bytes",
        leaf.len(),
    )?;
    capture_append(
        witness,
        leaf,
        maximum,
        "from_captured witness bytes",
        leaf.len(),
    )?;
    let boundary =
        crate::portable_replay::PortableBoundary::decode_with_header(leaf, maximum, header)
            .map_err(|error| {
                capture_diagnostic(
                    error,
                    "from_captured leaf decode",
                    leaf.len() as u128,
                    maximum as u128,
                    leaf.len(),
                )
            })?;
    let supplement = &boundary.replay.snapshot.supplement;
    let fuels = crate::commit::replay_step_commitment_fuels(
        supplement.canonical_state_bytes,
        supplement.arbitration_engine_canonical_bytes,
        supplement.arbitration_host_state_bytes,
    )
    .map_err(|_| {
        capture_diagnostic(
            ReplayWitnessError::Bounds,
            "from_captured commitment fuel",
            u128::from(supplement.canonical_state_bytes)
                + u128::from(supplement.arbitration_engine_canonical_bytes)
                + u128::from(supplement.arbitration_host_state_bytes),
            u128::from(u64::MAX),
            leaf.len(),
        )
    })?;
    if fuels
        != (
            supplement.commitment_fuel,
            supplement.arbitration_commitment_fuel,
        )
    {
        return Err(capture_diagnostic(
            ReplayWitnessError::Binding,
            "from_captured commitment fuel binding",
            u128::from(fuels.0) + u128::from(fuels.1),
            u128::from(supplement.commitment_fuel)
                + u128::from(supplement.arbitration_commitment_fuel),
            leaf.len(),
        ));
    }
    *replay_fuel = replay_fuel
        .checked_add(fuels.0)
        .and_then(|total| total.checked_add(fuels.1))
        .ok_or_else(|| {
            capture_diagnostic(
                ReplayWitnessError::Bounds,
                "from_captured fuel sum",
                u128::from(*replay_fuel) + u128::from(fuels.0) + u128::from(fuels.1),
                u128::from(u64::MAX),
                leaf.len(),
            )
        })?;
    let mut hash = Sha256::new();
    hash.update(b"LXP/program-replay-leaf/v1\0");
    hash.update(
        u32::try_from(index)
            .map_err(|_| {
                capture_diagnostic(
                    ReplayWitnessError::Bounds,
                    "from_captured leaf index",
                    index as u128,
                    u128::from(u32::MAX),
                    0,
                )
            })?
            .to_be_bytes(),
    );
    hash.update(length.to_be_bytes());
    hash.update(leaf);
    Ok(<[u8; 32]>::from(hash.finalize()))
}

type CapturedWitness = (Vec<u8>, [u8; 32], u64);
fn encode_captured_witness(
    profile: ProgramReplayProfile,
    leaves: &[Vec<u8>],
    header: &ReplayRecordHeader,
    boundary_count: u32,
) -> Result<CapturedWitness, ReplayCaptureError> {
    let maximum = profile.maximum_bytes as usize;
    let mut witness = Vec::new();
    capture_append(
        &mut witness,
        PROGRAM_REPLAY_WITNESS_DOMAIN,
        maximum,
        "from_captured witness bytes",
        0,
    )?;
    let header_digest = header.digest().map_err(|error| {
        capture_diagnostic(error, "from_captured header digest", 0, maximum as u128, 0)
    })?;
    capture_append(
        &mut witness,
        &header_digest,
        maximum,
        "from_captured witness header binding",
        0,
    )?;
    capture_append(
        &mut witness,
        &boundary_count.to_be_bytes(),
        maximum,
        "from_captured witness bytes",
        0,
    )?;
    let mut hashes = Vec::new();
    hashes.try_reserve_exact(leaves.len()).map_err(|_| {
        capture_diagnostic(
            ReplayWitnessError::Allocation,
            "from_captured hash allocation",
            leaves.len() as u128,
            u128::from(profile.maximum_boundaries),
            0,
        )
    })?;
    let mut replay_fuel = 0u64;
    for (index, leaf) in leaves.iter().enumerate() {
        hashes.push(append_captured_leaf(
            &mut witness,
            leaf,
            index,
            header,
            maximum,
            &mut replay_fuel,
        )?);
    }
    while hashes.len() > 1 {
        let mut next = Vec::new();
        next.try_reserve_exact(hashes.len().div_ceil(2))
            .map_err(|_| {
                capture_diagnostic(
                    ReplayWitnessError::Allocation,
                    "from_captured hash allocation",
                    leaves.len() as u128,
                    u128::from(profile.maximum_boundaries),
                    0,
                )
            })?;
        for pair in hashes.chunks(2) {
            let mut hash = Sha256::new();
            hash.update(b"LXP/program-replay-node/v1\0");
            hash.update(pair[0]);
            hash.update(*pair.get(1).unwrap_or(&pair[0]));
            next.push(<[u8; 32]>::from(hash.finalize()));
        }
        hashes = next;
    }
    Ok((witness, hashes[0], replay_fuel))
}

fn encode_captured_record(
    parameters: CapturedRecordParameters,
    header: &ReplayRecordHeader,
    boundary_count: u32,
    boundary_root: [u8; 32],
    witness_digest: [u8; 32],
    witness: &[u8],
) -> Result<Vec<u8>, ReplayCaptureError> {
    let profile = parameters.profile;
    let maximum = profile.maximum_bytes as usize;
    let header_bytes = header.canonical_bytes(maximum).map_err(|error| {
        capture_diagnostic(error, "from_captured header bytes", 0, maximum as u128, 0)
    })?;
    let header_length = u32::try_from(header_bytes.len()).map_err(|_| {
        capture_diagnostic(
            ReplayWitnessError::Bounds,
            "from_captured header length",
            header_bytes.len() as u128,
            u128::from(u32::MAX),
            0,
        )
    })?;
    let witness_length = u32::try_from(witness.len()).map_err(|_| {
        capture_diagnostic(
            ReplayWitnessError::Bounds,
            "from_captured witness length",
            witness.len() as u128,
            u128::from(u32::MAX),
            0,
        )
    })?;
    let fields: &[(&[u8], &str)] = &[
        (PROGRAM_REPLAY_RECORD_DOMAIN, "from_captured record bytes"),
        (&2u16.to_be_bytes(), "from_captured record bytes"),
        (&header_length.to_be_bytes(), "from_captured record header"),
        (&header_bytes, "from_captured record header"),
        (
            &profile.maximum_boundaries.to_be_bytes(),
            "from_captured record bytes",
        ),
        (
            &profile.maximum_bytes.to_be_bytes(),
            "from_captured record bytes",
        ),
        (&[parameters.terminal_status], "from_captured record bytes"),
        (&boundary_count.to_be_bytes(), "from_captured record bytes"),
        (&boundary_root, "from_captured record bytes"),
        (&witness_digest, "from_captured record bytes"),
        (&witness_length.to_be_bytes(), "from_captured record bytes"),
        (witness, "from_captured record bytes"),
    ];
    let mut bytes = Vec::new();
    for (value, check) in fields {
        capture_append(&mut bytes, value, maximum, check, 0)?;
    }
    Ok(bytes)
}

impl ProgramReplayRecord {
    pub(crate) fn from_captured(
        parameters: CapturedRecordParameters,
        leaves: &[Vec<u8>],
        header: ReplayRecordHeader,
    ) -> Result<Self, ReplayCaptureError> {
        let CapturedRecordParameters {
            profile,
            code_hash,
            input_digest,
            runtime_version,
            abi_version,
            fee_schedule_version,
            metering_schedule_version,
            terminal_status,
        } = parameters;

        if header.identity.module_code_hash != code_hash
            || header.identity.input_digest != input_digest
            || header.identity.runtime_version != runtime_version
            || header.identity.abi_version != abi_version
            || header.identity.fee_schedule_version != fee_schedule_version
            || header.identity.metering_schedule_version != metering_schedule_version
            || header.identity.trace_policy != profile.trace_policy()
        {
            return Err(capture_diagnostic(
                ReplayWitnessError::Binding,
                "from_captured record header",
                1,
                0,
                0,
            ));
        }
        if terminal_status > 2 {
            return Err(capture_diagnostic(
                ReplayWitnessError::Bounds,
                "from_captured terminal status",
                u128::from(terminal_status),
                2,
                0,
            ));
        }
        if leaves.is_empty() || leaves.len() > profile.maximum_boundaries as usize {
            return Err(capture_diagnostic(
                ReplayWitnessError::Bounds,
                "from_captured leaves (minimum 1)",
                leaves.len() as u128,
                u128::from(profile.maximum_boundaries),
                0,
            ));
        }
        let boundary_count = u32::try_from(leaves.len()).map_err(|_| {
            capture_diagnostic(
                ReplayWitnessError::Bounds,
                "from_captured boundary count",
                leaves.len() as u128,
                u128::from(u32::MAX),
                0,
            )
        })?;
        let (witness, boundary_root, replay_fuel) =
            encode_captured_witness(profile, leaves, &header, boundary_count)?;
        let witness_digest = <[u8; 32]>::from(Sha256::digest(&witness));
        let bytes = encode_captured_record(
            parameters,
            &header,
            boundary_count,
            boundary_root,
            witness_digest,
            &witness,
        )?;
        Ok(Self {
            header,
            profile,
            code_hash,
            input_digest,
            runtime_version,
            abi_version,
            fee_schedule_version,
            metering_schedule_version,
            terminal_status,
            boundary_count,
            boundary_root,
            witness_digest,
            replay_fuel,
            bytes,
        })
    }
    #[must_use]
    pub fn header(&self) -> &ReplayRecordHeader {
        &self.header
    }
    #[must_use]
    pub const fn profile(&self) -> ProgramReplayProfile {
        self.profile
    }
    #[must_use]
    pub const fn code_hash(&self) -> [u8; 32] {
        self.code_hash
    }
    #[must_use]
    pub const fn input_digest(&self) -> [u8; 32] {
        self.input_digest
    }
    #[must_use]
    pub const fn runtime_version(&self) -> u16 {
        self.runtime_version
    }
    #[must_use]
    pub const fn abi_version(&self) -> u16 {
        self.abi_version
    }
    #[must_use]
    pub const fn fee_schedule_version(&self) -> u32 {
        self.fee_schedule_version
    }
    #[must_use]
    pub const fn metering_schedule_version(&self) -> u32 {
        self.metering_schedule_version
    }
    #[must_use]
    pub const fn terminal_status(&self) -> u8 {
        self.terminal_status
    }
    #[must_use]
    pub const fn boundary_count(&self) -> u32 {
        self.boundary_count
    }
    #[must_use]
    pub const fn boundary_root(&self) -> [u8; 32] {
        self.boundary_root
    }
    #[must_use]
    pub const fn witness_digest(&self) -> [u8; 32] {
        self.witness_digest
    }
    /// Total replay work, reconstructed from the committed boundary fuel fields
    /// when a captured or portable record is read.
    #[must_use]
    pub const fn replay_fuel(&self) -> u64 {
        self.replay_fuel
    }
    #[must_use]
    pub fn canonical_bytes(&self) -> &[u8] {
        &self.bytes
    }
}

pub(crate) fn captured_leaf(
    replay: &wasmi::ExecutionReplaySnapshot,
    arbitration: &crate::ArbitrationExecutionState,
    semantic: &crate::replay::CapturedSemanticReplayV2,
    memory: &crate::replay::ReplayMemory,
    maximum: usize,
) -> Result<Vec<u8>, ReplayCaptureError> {
    let semantic_length = semantic.encoded_len().map_err(|error| ReplayCaptureError {
        error,
        check: "captured_leaf semantic length sum",
        observed: usize::MAX as u128 + 1,
        permitted: usize::MAX as u128,
        leaf_length: 0,
    })?;
    let semantic_bytes = semantic
        .canonical_bytes(maximum)
        .map_err(|error| ReplayCaptureError {
            error,
            check: "captured_leaf semantic bytes",
            observed: semantic_length as u128,
            permitted: maximum as u128,
            leaf_length: 0,
        })?;
    captured_leaf_bytes(replay, arbitration, &semantic_bytes, memory, maximum)
}

fn capture_leaf_append(
    bytes: &mut Vec<u8>,
    value: &[u8],
    maximum: usize,
) -> Result<(), ReplayCaptureError> {
    let leaf_length = bytes.len().saturating_add(value.len());
    capture_append(bytes, value, maximum, "captured_leaf bytes", leaf_length)
}
fn capture_leaf_field(
    bytes: &mut Vec<u8>,
    value: &[u8],
    maximum: usize,
) -> Result<(), ReplayCaptureError> {
    let length = u32::try_from(value.len()).map_err(|_| ReplayCaptureError {
        error: ReplayWitnessError::Bounds,
        check: "captured_leaf field length",
        observed: value.len() as u128,
        permitted: u128::from(u32::MAX),
        leaf_length: bytes.len(),
    })?;
    capture_leaf_append(bytes, &length.to_be_bytes(), maximum)?;
    capture_leaf_append(bytes, value, maximum)
}

fn captured_leaf_state(
    replay: &wasmi::ExecutionReplaySnapshot,
    arbitration: &crate::ArbitrationExecutionState,
    memory: &crate::replay::ReplayMemory,
    maximum: usize,
) -> Result<Vec<u8>, ReplayCaptureError> {
    let diagnostic = |error| ReplayCaptureError {
        error,
        check: "captured_leaf canonical state",
        observed: replay.snapshot.linear_memory.len() as u128,
        permitted: crate::MAX_ARBITRATION_STATE_BYTES as u128,
        leaf_length: 0,
    };
    let mut portable = arbitration.clone();
    let mut snapshot = replay.snapshot.as_ref().clone();
    for instance in &mut snapshot.arbitration_instances {
        if instance.instance_index == 0 {
            for engine_memory in &mut instance.memories {
                if engine_memory.memory_index == 0 {
                    engine_memory.bytes.clear();
                }
            }
        }
    }
    portable.engine_state = crate::execute::arbitration_engine_state_bytes(&snapshot)
        .map_err(|_| diagnostic(ReplayWitnessError::Binding))?;
    std::sync::Arc::make_mut(&mut portable.legacy).linear_memory =
        memory.canonical_bytes().map_err(diagnostic)?;
    // Canonical legacy state without identity, overlay or meter: those have one source.
    portable
        .canonical_bytes()
        .map_err(|_| diagnostic(ReplayWitnessError::Binding))?;
    let legacy = portable
        .legacy
        .canonical_bytes()
        .map_err(|_| diagnostic(ReplayWitnessError::Binding))?;
    let memory_length = portable.legacy.linear_memory.len();
    let globals_length = 4 + portable
        .legacy
        .globals
        .iter()
        .map(|g| {
            5 + match g.value {
                crate::ExecutionValue::I32(_) => 5,
                crate::ExecutionValue::I64(_) => 9,
            }
        })
        .sum::<usize>();
    let overlay_length = 4 + portable
        .legacy
        .storage_overlay
        .iter()
        .map(|entry| match entry {
            crate::StorageOverlayEntry::Write { key, value } => 9 + key.len() + value.len(),
            crate::StorageOverlayEntry::Delete { key } => 5 + key.len(),
        })
        .sum::<usize>();
    let usage_length = 8 + 8 + 8 + 8 + 4 + 8 + 16 + 16 + 16;
    let legacy_end = legacy
        .len()
        .checked_sub(overlay_length + 8 + usage_length)
        .ok_or_else(|| diagnostic(ReplayWitnessError::Encoding))?;
    if legacy_end < 98 + memory_length + globals_length {
        return Err(diagnostic(ReplayWitnessError::Encoding));
    }
    let mut state = Vec::new();
    capture_leaf_append(
        &mut state,
        &crate::ARBITRATION_STEP_COMMITMENT_VERSION.to_be_bytes(),
        maximum,
    )?;
    capture_leaf_field(&mut state, &legacy[98..legacy_end], maximum)?;
    capture_leaf_field(&mut state, &portable.engine_state, maximum)?;
    capture_leaf_append(&mut state, &portable.host_state_root, maximum)?;
    capture_leaf_append(
        &mut state,
        &portable.host_state_bytes.to_be_bytes(),
        maximum,
    )?;
    Ok(state)
}

pub(crate) fn captured_leaf_bytes(
    replay: &wasmi::ExecutionReplaySnapshot,
    arbitration: &crate::ArbitrationExecutionState,
    semantic_bytes: &[u8],
    memory: &crate::replay::ReplayMemory,
    maximum: usize,
) -> Result<Vec<u8>, ReplayCaptureError> {
    let diagnostic = |error| ReplayCaptureError {
        error,
        check: "captured_leaf canonical state",
        observed: replay.snapshot.linear_memory.len() as u128,
        permitted: crate::MAX_ARBITRATION_STATE_BYTES as u128,
        leaf_length: 0,
    };
    let header = ReplayRecordHeader::from_boundary(arbitration, semantic_bytes, maximum)
        .map_err(diagnostic)?;
    let mut bytes = Vec::new();
    let state = captured_leaf_state(replay, arbitration, memory, maximum)?;
    capture_leaf_field(&mut bytes, &state, maximum)?;
    let frame_count = u32::try_from(replay.frames.len()).map_err(|_| ReplayCaptureError {
        error: ReplayWitnessError::Bounds,
        check: "captured_leaf frame count",
        observed: replay.frames.len() as u128,
        permitted: u128::from(u32::MAX),
        leaf_length: bytes.len(),
    })?;
    capture_leaf_append(&mut bytes, &frame_count.to_be_bytes(), maximum)?;
    for frame in &replay.frames {
        capture_leaf_append(
            &mut bytes,
            &frame.module_function_index.to_be_bytes(),
            maximum,
        )?;
        capture_leaf_append(&mut bytes, &frame.instruction_offset.to_be_bytes(), maximum)?;
        capture_leaf_append(&mut bytes, &frame.value_base.to_be_bytes(), maximum)?;
        let operand_count =
            u32::try_from(frame.operand_types.len()).map_err(|_| ReplayCaptureError {
                error: ReplayWitnessError::Bounds,
                check: "captured_leaf operand count",
                observed: frame.operand_types.len() as u128,
                permitted: u128::from(u32::MAX),
                leaf_length: bytes.len(),
            })?;
        capture_leaf_append(&mut bytes, &operand_count.to_be_bytes(), maximum)?;
        for value_type in &frame.operand_types {
            capture_leaf_append(
                &mut bytes,
                &[match value_type {
                    wasmi::ExecutionValueType::I32 => 0,
                    wasmi::ExecutionValueType::I64 => 1,
                }],
                maximum,
            )?;
        }
    }
    capture_leaf_field(&mut bytes, &replay.snapshot.canonical_instruction, maximum)?;
    capture_leaf_append(
        &mut bytes,
        &replay.snapshot.instruction_fuel.to_be_bytes(),
        maximum,
    )?;
    capture_leaf_append(
        &mut bytes,
        &replay.snapshot.memory_expansion_bytes.to_be_bytes(),
        maximum,
    )?;
    let supplement = &replay.snapshot.supplement;
    for value in [
        supplement.canonical_state_bytes,
        supplement.commitment_fuel,
        supplement.arbitration_engine_canonical_bytes,
        supplement.arbitration_instance_retained_bytes,
        supplement.arbitration_canonical_state_bytes,
        supplement.arbitration_commitment_fuel,
    ] {
        capture_leaf_append(&mut bytes, &value.to_be_bytes(), maximum)?;
    }
    let semantic =
        crate::replay::compact_semantic(semantic_bytes, &header, maximum).map_err(diagnostic)?;
    capture_leaf_field(&mut bytes, &semantic, maximum)?;
    Ok(bytes)
}

pub(crate) fn captured_leaf_breakdown(
    leaf: &[u8],
    header: &ReplayRecordHeader,
    maximum: usize,
) -> Result<String, ReplayWitnessError> {
    use crate::replay::ReplayCursor;
    fn present(cursor: &mut ReplayCursor<'_>) -> Result<usize, ReplayWitnessError> {
        Ok(if cursor.boolean()? {
            4 + cursor.field()?.len()
        } else {
            0
        })
    }
    let boundary =
        crate::portable_replay::PortableBoundary::decode_with_header(leaf, maximum, header)?;
    let mut cursor = ReplayCursor::new(leaf);
    let state = cursor.field()?;
    let mut arbitration = ReplayCursor::new(state);
    arbitration.u16()?;
    let legacy = arbitration.field()?.len();
    let engine = arbitration.field()?.len();
    let frames = 4 + boundary
        .replay
        .frames
        .iter()
        .map(|frame| 16 + frame.operand_types.len())
        .sum::<usize>();
    for _ in 0..cursor.count(16)? {
        cursor.take(12)?;
        let count = cursor.count(1)?;
        cursor.take(count)?;
    }
    let instruction = cursor.field()?.len() + 4;
    cursor.take(16 + 48)?;
    let semantic = cursor.field()?;
    let mut parts = ReplayCursor::new(semantic);
    let host_bytes = parts.field()?;
    let host = host_bytes.len();

    let mut host_parts = ReplayCursor::new(host_bytes);
    let code_hash = 1 + present(&mut host_parts)?;
    let abi = 1 + present(&mut host_parts)?;
    let runtime = host_parts.field()?;
    let meter = host_parts.field()?.len();
    let baseline = 1 + present(&mut host_parts)?;
    let mut runtime_parts = ReplayCursor::new(runtime);
    if runtime_parts.boolean()? {
        runtime_parts.u64()?;
    }
    runtime_parts.u64()?;
    runtime_parts.boolean()?;
    if runtime_parts.boolean()? {
        runtime_parts.take(24)?;
    }
    let composition_code_hash = 1 + present(&mut runtime_parts)?;
    let mut graph_bytes = 2;
    let mut revision = 0;
    if runtime_parts.boolean()? {
        revision = 1 + present(&mut runtime_parts)?;
        graph_bytes += 1 + present(&mut runtime_parts)?;
    }
    if runtime_parts.boolean()? {
        graph_bytes += 4 + runtime_parts.field()?.len();
    }
    let storage = parts.field()?.len();
    let composition = parts.field()?.len();
    Ok(format!(
        "step={} arbitration={} arbitration_framing=4 identity=0 legacy={} engine={} host_commitment=40 frames={} instruction={} fuel_expansion=16 supplement=48 semantic={} semantic_framing=4 semantic_host={} semantic_storage={} semantic_composition={} memory={} changed_chunks={} leaf_total={} header_total={} host_code_hash={} host_abi={} host_runtime={} host_meter={} host_storage_baseline={} host_framing=8 runtime_composition_code_hash={} runtime_composition_revision={} runtime_composition_graph={}",
        boundary.replay.snapshot.step_index,
        state.len(), legacy, engine, frames, instruction, semantic.len(), host, storage,
        composition, 40 + boundary.memory.changed_chunks.len() * 4100,
        boundary.memory.changed_chunks.len(), leaf.len(), header.canonical_bytes(maximum)?.len(), code_hash, abi, runtime.len(), meter, baseline, composition_code_hash, revision, graph_bytes,
    ))
}
