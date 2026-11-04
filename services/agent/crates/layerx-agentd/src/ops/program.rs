//! First-class program discovery, interface, simulation, and call operations.

use layerx_client::rpc::submit::SubmitError;
use layerx_client::sequencer::{
    ExecutedReceipt, SequencerEndpoint, SequencerError, SequencerReceipt,
};
use layerx_crypto::ed25519;
use layerx_programs::{ProgramId, ProgramInterface, ProgramLifecycle, VerifiedProgramBundle};
use layerx_programs_runtime::terminal::DecodedTerminal;
use layerx_programs_runtime::{BudgetMeterRefusal, ProgramFailure};
use layerx_proof::program::{
    verify_program_execution_with_payers, OccupancyPayer, ProgramExecutionExpectation,
};
use layerx_types::intent::{CapabilityRequest, ProgramCall, ProgramCallOutcome};
use layerx_types::payload::{ModuleId, ModuleRegistry};
use layerx_types::program_call::NativeProgramCall;
use layerx_types::result::ResultCode;
use layerx_wire::activity::decode_signed;
use layerx_wire::hash::activity_id;
use sha2::{Digest as _, Sha256};

const SIMULATION_EVIDENCE_DOMAIN: &[u8] = b"LayerX/agent/program-simulation-evidence/v1\0";

use crate::read::LayerxdProgramBalanceReader;

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum ProgramOperationError<E = SubmitError> {
    InvalidRequest,
    UnknownProgram,
    InactiveProgram,
    Stale,
    UnverifiedReceipt,
    Submit(E),
    /// The pinned authenticated chain head advanced before the read completed.
    HeadAdvanced,
    /// The kind-5 answer refused the program: record or interface not present.
    ProgramStateAbsent,
    /// The node does not serve authenticated kind-5 Programs state.
    Unavailable,
    /// Any other exact core refusal of the kind-5 request.
    CoreRefusal {
        class: u8,
        result: ResultCode,
    },
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ProgramAccountBalance {
    pub account: [u8; 32],
    pub balance: u128,
    pub exists: bool,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ProgramDiscovery {
    pub program: ProgramId,
    pub lifecycle: ProgramLifecycle,
    pub observed_sequence: u64,
    pub observed_at: u64,
    pub valid_through: u64,
    pub receipt_digest: [u8; 32],
    pub state_root: [u8; 32],
    pub version: u32,
    pub abi_version: u16,
    pub code_hash: [u8; 32],
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ProgramInterfaceRead {
    pub discovery: ProgramDiscovery,
    pub version: u32,
    pub interface: ProgramInterface,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ProgramExecution {
    committed: bool,
    result_code: i32,
    metered_cost: u128,
    fee_units: u128,
    terminal_payload_root: [u8; 32],
    cpu_fuel: u64,
    memory_bytes: u64,
    storage_read_bytes: u64,
    storage_write_bytes: u64,
    output_values: u32,
    output_bytes: u64,
    outcome: Option<ProgramCallOutcome>,
    authenticated_failure: Option<ProgramFailure>,
    authenticated_resource: Option<BudgetMeterRefusal>,
    terminal: DecodedTerminal,
    call_graph: Vec<u8>,
    receipt: Vec<u8>,
}

impl ProgramExecution {
    #[must_use]
    pub const fn committed(&self) -> bool {
        self.committed
    }
    #[must_use]
    pub const fn result_code(&self) -> i32 {
        self.result_code
    }
    #[must_use]
    pub const fn metered_cost(&self) -> u128 {
        self.metered_cost
    }
    #[must_use]
    pub const fn fee_units(&self) -> u128 {
        self.fee_units
    }
    #[must_use]
    pub const fn terminal_payload_root(&self) -> [u8; 32] {
        self.terminal_payload_root
    }
    #[must_use]
    pub const fn cpu_fuel(&self) -> u64 {
        self.cpu_fuel
    }
    #[must_use]
    pub const fn memory_bytes(&self) -> u64 {
        self.memory_bytes
    }
    #[must_use]
    pub const fn storage_read_bytes(&self) -> u64 {
        self.storage_read_bytes
    }
    #[must_use]
    pub const fn storage_write_bytes(&self) -> u64 {
        self.storage_write_bytes
    }
    #[must_use]
    pub const fn output_values(&self) -> u32 {
        self.output_values
    }
    #[must_use]
    pub const fn output_bytes(&self) -> u64 {
        self.output_bytes
    }
    #[must_use]
    pub const fn outcome(&self) -> Option<&ProgramCallOutcome> {
        self.outcome.as_ref()
    }
    #[must_use]
    pub const fn authenticated_failure(&self) -> Option<&ProgramFailure> {
        self.authenticated_failure.as_ref()
    }
    #[must_use]
    pub const fn authenticated_resource(&self) -> Option<&BudgetMeterRefusal> {
        self.authenticated_resource.as_ref()
    }
    #[must_use]
    pub const fn terminal(&self) -> &DecodedTerminal {
        &self.terminal
    }
    #[must_use]
    pub fn call_graph(&self) -> &[u8] {
        &self.call_graph
    }
    #[must_use]
    pub fn receipt(&self) -> &[u8] {
        &self.receipt
    }
}

pub struct RawProgramSimulation {
    pub receipt: Vec<u8>,
    pub terminal_payload: Vec<u8>,
    pub call_graph: Vec<u8>,
    pub evidence: ProgramSimulationEvidence,
    pub evidence_signature: [u8; 64],
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum ProgramSimulationEvidence {
    SequencerSigned {
        boundary_id: [u8; 32],
        activity_id: [u8; 32],
        previous_state_root: [u8; 32],
        hypothetical_state_root: [u8; 32],
        observed_sequence: u64,
        observed_at: u64,
        committed: bool,
    },
    EvmPending {
        activity_id: [u8; 32],
        sender: [u8; 20],
        to: [u8; 20],
        gas_estimate: u64,
        result: ProgramCallResult,
    },
}

impl ProgramSimulationEvidence {
    #[must_use]
    pub fn signing_digest(&self) -> Option<[u8; 32]> {
        let Self::SequencerSigned {
            boundary_id,
            activity_id,
            previous_state_root,
            hypothetical_state_root,
            observed_sequence,
            observed_at,
            committed,
        } = self
        else {
            return None;
        };
        let mut bytes = Vec::with_capacity(SIMULATION_EVIDENCE_DOMAIN.len() + 137);
        bytes.extend_from_slice(SIMULATION_EVIDENCE_DOMAIN);
        bytes.extend_from_slice(boundary_id);
        bytes.extend_from_slice(activity_id);
        bytes.extend_from_slice(previous_state_root);
        bytes.extend_from_slice(hypothetical_state_root);
        bytes.extend_from_slice(&observed_sequence.to_be_bytes());
        bytes.extend_from_slice(&observed_at.to_be_bytes());
        bytes.push(u8::from(*committed));
        Some(Sha256::digest(bytes).into())
    }

    fn matches_context(
        &self,
        boundary_id: [u8; 32],
        activity_id: [u8; 32],
        previous_state_root: [u8; 32],
        hypothetical_state_root: [u8; 32],
        observed_sequence: u64,
        observed_at: u64,
    ) -> bool {
        matches!(self, Self::SequencerSigned {
            boundary_id: boundary, activity_id: activity, previous_state_root: previous,
            hypothetical_state_root: hypothetical, observed_sequence: sequence,
            observed_at: at, committed: false,
        } if *boundary == boundary_id && *activity == activity_id && *previous == previous_state_root
            && *hypothetical == hypothetical_state_root && *sequence == observed_sequence && *at == observed_at)
    }
}

pub trait ProgramSimulationTransport {
    ///
    /// # Errors
    ///
    /// Returns an error if the request, program metadata, or execution evidence is invalid, or transport fails.
    fn simulate_exact(
        &mut self,
        call: &ProgramCall,
        signed_activity: &[u8],
    ) -> Result<RawProgramSimulation, ProgramOperationError>;
}

pub struct EmulatorProgramSimulationTransport {
    agent: ureq::Agent,
    endpoint: String,
}

#[cfg(test)]
use crate::outbound_tls::tls_boundary;

#[test]
fn simulation_tls_checks_the_actual_server_identity() {
    tls_boundary::qualify(
        "ops::program::simulation_tls_checks_the_actual_server_identity",
        |endpoint| {
            let uppercase = endpoint.replacen("https://", "HTTPS://", 1);
            let client = EmulatorProgramSimulationTransport::connect(&uppercase)
                .map_err(|error| format!("{error:?}"))?;
            client
                .agent
                .get(format!("{endpoint}/livez"))
                .call()
                .map_err(|error| error.to_string())?
                .body_mut()
                .read_to_vec()
                .map_err(|error| error.to_string())
        },
    );
}

impl EmulatorProgramSimulationTransport {
    /// # Errors
    /// Refuses unavailable or invalid system trust roots for HTTPS.
    pub fn connect(endpoint: &str) -> Result<Self, ProgramOperationError> {
        let config = ureq::Agent::config_builder()
            .tls_config(
                crate::outbound_tls::system(endpoint)
                    .ok_or(ProgramOperationError::InvalidRequest)?,
            )
            .http_status_as_error(false)
            .build();
        Ok(Self {
            agent: config.into(),
            endpoint: endpoint.trim_end_matches('/').to_owned(),
        })
    }
}

impl ProgramSimulationTransport for EmulatorProgramSimulationTransport {
    fn simulate_exact(
        &mut self,
        call: &ProgramCall,
        signed_activity: &[u8],
    ) -> Result<RawProgramSimulation, ProgramOperationError> {
        self.simulate_document(&program_call_request(call, signed_activity))
    }
}

pub trait NativeProgramSimulationTransport {
    ///
    /// # Errors
    ///
    /// Returns an error if the request, program metadata, or execution evidence is invalid, or transport fails.
    fn simulate_native_exact(
        &mut self,
        call: NativeProgramCall<'_>,
        fee_limit: u128,
        signed_activity: &[u8],
    ) -> Result<RawProgramSimulation, ProgramOperationError>;
}

impl EmulatorProgramSimulationTransport {
    fn simulate_document(
        &mut self,
        request: &serde_json::Value,
    ) -> Result<RawProgramSimulation, ProgramOperationError> {
        let url = format!("{}/v1/programs/simulate", self.endpoint);
        let signed = decode_hex_json(request, "signed_activity")?;
        if signed.is_empty() || signed.len() > 1_048_576 {
            return Err(ProgramOperationError::InvalidRequest);
        }
        let mut response = self
            .agent
            .post(&url)
            .header("Content-Type", "application/octet-stream")
            .send(signed.as_slice())
            .map_err(|_| ProgramOperationError::InvalidRequest)?;
        if !response.status().is_success() {
            return Err(ProgramOperationError::InvalidRequest);
        }
        let text = response
            .body_mut()
            .read_to_string()
            .map_err(|_| ProgramOperationError::InvalidRequest)?;
        let document: serde_json::Value =
            serde_json::from_str(&text).map_err(|_| ProgramOperationError::InvalidRequest)?;
        let envelope = document
            .as_object()
            .ok_or(ProgramOperationError::UnverifiedReceipt)?;
        let verification = envelope
            .get("verification_status")
            .and_then(serde_json::Value::as_object)
            .ok_or(ProgramOperationError::UnverifiedReceipt)?;
        if verification
            .get("state")
            .and_then(serde_json::Value::as_str)
            != Some("Achieved")
            || verification
                .get("level")
                .and_then(serde_json::Value::as_str)
                != Some("SequencerSigned")
        {
            return Err(ProgramOperationError::UnverifiedReceipt);
        }
        let result = envelope
            .get("value")
            .ok_or(ProgramOperationError::UnverifiedReceipt)?;
        if result.get("committed").and_then(serde_json::Value::as_bool) != Some(false) {
            return Err(ProgramOperationError::UnverifiedReceipt);
        }
        let execution = result
            .get("execution")
            .ok_or(ProgramOperationError::UnverifiedReceipt)?;
        let evidence = result
            .get("simulation_evidence")
            .ok_or(ProgramOperationError::UnverifiedReceipt)?;
        Ok(RawProgramSimulation {
            receipt: decode_hex_json(execution, "receipt")?,
            terminal_payload: decode_hex_json(execution, "terminal_payload")?,
            call_graph: decode_hex_json(execution, "call_graph")?,
            evidence: ProgramSimulationEvidence::SequencerSigned {
                boundary_id: decode_fixed_json(evidence, "boundary_id")?,
                activity_id: decode_fixed_json(evidence, "activity_id")?,
                previous_state_root: decode_fixed_json(evidence, "previous_state_root")?,
                hypothetical_state_root: decode_fixed_json(evidence, "hypothetical_state_root")?,
                observed_sequence: decode_decimal_u64_json(evidence, "observed_sequence")?,
                observed_at: decode_decimal_u64_json(evidence, "observed_at")?,
                committed: evidence
                    .get("committed")
                    .and_then(serde_json::Value::as_bool)
                    .ok_or(ProgramOperationError::UnverifiedReceipt)?,
            },
            evidence_signature: decode_fixed_json(evidence, "signature")?,
        })
    }
}

impl NativeProgramSimulationTransport for EmulatorProgramSimulationTransport {
    fn simulate_native_exact(
        &mut self,
        call: NativeProgramCall<'_>,
        fee_limit: u128,
        signed_activity: &[u8],
    ) -> Result<RawProgramSimulation, ProgramOperationError> {
        call.encode()
            .map_err(|_| ProgramOperationError::InvalidRequest)?;
        self.simulate_document(&serde_json::json!({
            "payload_encoding":"native-v1", "program_id":encode_hex(&call.program_id.bytes()), "calldata":encode_hex(call.calldata),
            "budget":{"fuel":call.resources.0[0].to_string(),"fee_limit":fee_limit.to_string()}, "signed_activity":encode_hex(signed_activity),
            "native_call":{"guest_abi":call.guest_abi,"entrypoint":std::str::from_utf8(call.entrypoint).map_err(|_| ProgramOperationError::InvalidRequest)?,
                "capabilities_hex":encode_hex(call.capabilities),"access_declaration_hex":encode_hex(call.access_declaration),
                "response_capacity":call.response_capacity,"resources":call.resources.0.map(|value|value.to_string())}
        }))
    }
}

pub struct NodeProgramSimulationTransport<'a> {
    client: &'a mut layerx_client::sequencer::SequencerEndpoint,
    registry: ModuleRegistry,
    correlation_id: u64,
}

impl<'a> NodeProgramSimulationTransport<'a> {
    #[must_use]
    pub fn new(
        client: &'a mut layerx_client::sequencer::SequencerEndpoint,
        registry: ModuleRegistry,
        correlation_id: u64,
    ) -> Self {
        Self {
            client,
            registry,
            correlation_id,
        }
    }

    fn simulate_signed_activity(
        &mut self,
        signed_activity: &[u8],
    ) -> Result<RawProgramSimulation, ProgramOperationError> {
        let (call, signature) = layerx_crypto::evm_transaction::decode_signed(signed_activity)
            .map_err(|_| ProgramOperationError::InvalidRequest)?;
        let sender = layerx_crypto::evm_transaction::recover_sender(&call, &signature)
            .map_err(|_| ProgramOperationError::InvalidRequest)?;
        validate_program_transaction(&call)?;
        let gas_estimate = self
            .client
            .estimate_gas(&layerx_client::sequencer::CallRequest {
                from: Some(sender),
                to: Some(call.to),
                value: 0,
                data: call.data.clone(),
            })
            .map_err(node_program_error)?;
        let bytes = self
            .client
            .call_pending_from(&sender, &call.to, &call.data)
            .map_err(node_program_error)?;
        let result = decode_program_call_result(&bytes, program_response_capacity(&call.data)?)
            .map_err(legacy_program_error)?;
        Ok(RawProgramSimulation {
            receipt: Vec::new(),
            terminal_payload: bytes,
            call_graph: Vec::new(),
            evidence: ProgramSimulationEvidence::EvmPending {
                activity_id: layerx_crypto::evm_transaction::transaction_hash(signed_activity),
                sender,
                to: call.to,
                gas_estimate,
                result,
            },
            evidence_signature: [0; 64],
        })
    }
}

impl ProgramSimulationTransport for NodeProgramSimulationTransport<'_> {
    fn simulate_exact(
        &mut self,
        _call: &ProgramCall,
        signed_activity: &[u8],
    ) -> Result<RawProgramSimulation, ProgramOperationError> {
        self.simulate_signed_activity(signed_activity)
    }
}

impl NativeProgramSimulationTransport for NodeProgramSimulationTransport<'_> {
    fn simulate_native_exact(
        &mut self,
        call: NativeProgramCall<'_>,
        fee_limit: u128,
        signed_activity: &[u8],
    ) -> Result<RawProgramSimulation, ProgramOperationError> {
        validate_native_call_activity(&self.registry, call, fee_limit, signed_activity)?;
        self.simulate_signed_activity(signed_activity)
    }
}

fn encode_hex(bytes: &[u8]) -> String {
    bytes
        .iter()
        .flat_map(|byte| {
            let digits = b"0123456789abcdef";
            [
                char::from(digits[usize::from(byte >> 4)]),
                char::from(digits[usize::from(byte & 15)]),
            ]
        })
        .collect::<String>()
}
fn program_call_request(call: &ProgramCall, signed_activity: &[u8]) -> serde_json::Value {
    serde_json::json!({
        "program_id": encode_hex(&call.callee().bytes()),
        "calldata": encode_hex(call.calldata().as_bytes()),
        "budget": {
            "fuel": call.budget().fuel().to_string(),
            "fee_limit": call.budget().fee_limit().value().to_string(),
        },
        "capabilities": call.capabilities().as_slice().iter().map(|capability| match capability {
            CapabilityRequest::StorageRead => "storage_read",
            CapabilityRequest::StorageWrite => "storage_write",
            CapabilityRequest::Transfer => "transfer",
            CapabilityRequest::EmitEvent => "emit_event",
            CapabilityRequest::Compose => "compose",
        }).collect::<Vec<_>>(),
        "signed_activity": encode_hex(signed_activity),
    })
}
fn decode_hex_json(
    value: &serde_json::Value,
    field: &str,
) -> Result<Vec<u8>, ProgramOperationError> {
    let text = value
        .get(field)
        .and_then(serde_json::Value::as_str)
        .ok_or(ProgramOperationError::UnverifiedReceipt)?;
    decode_program_hex(text, layerx_wire::limits::MAX_MESSAGE_BYTES)
}

fn decode_program_hex(text: &str, maximum: usize) -> Result<Vec<u8>, ProgramOperationError> {
    let bytes = text.as_bytes();
    if bytes.len() % 2 != 0 || bytes.len() / 2 > maximum || !bytes.iter().all(u8::is_ascii_hexdigit)
    {
        return Err(ProgramOperationError::UnverifiedReceipt);
    }
    let nibble = |byte: u8| {
        if byte.is_ascii_digit() {
            byte - b'0'
        } else {
            byte.to_ascii_lowercase() - b'a' + 10
        }
    };
    Ok(bytes
        .chunks_exact(2)
        .map(|pair| (nibble(pair[0]) << 4) | nibble(pair[1]))
        .collect())
}
fn decode_fixed_json<const N: usize>(
    value: &serde_json::Value,
    field: &str,
) -> Result<[u8; N], ProgramOperationError> {
    let text = value
        .get(field)
        .and_then(serde_json::Value::as_str)
        .ok_or(ProgramOperationError::UnverifiedReceipt)?;
    decode_program_hex(text, N)?
        .try_into()
        .map_err(|_| ProgramOperationError::UnverifiedReceipt)
}
fn decode_decimal_u64_json(
    value: &serde_json::Value,
    field: &str,
) -> Result<u64, ProgramOperationError> {
    let text = value
        .get(field)
        .and_then(serde_json::Value::as_str)
        .ok_or(ProgramOperationError::UnverifiedReceipt)?;
    if text.is_empty()
        || !text.bytes().all(|byte| byte.is_ascii_digit())
        || (text.len() > 1 && text.starts_with('0'))
    {
        return Err(ProgramOperationError::UnverifiedReceipt);
    }
    text.parse()
        .map_err(|_| ProgramOperationError::UnverifiedReceipt)
}

pub struct ReceiptVerifiedProgramSimulator<T> {
    transport: T,
    registry: ModuleRegistry,
    expected_abi_version: u16,
    expected_program: ProgramId,
    expected_version: u32,
    expected_code_hash: [u8; 32],
    simulation_public_key: [u8; 32],
    boundary_id: [u8; 32],
    receipt_digest: [u8; 32],
    observed_sequence: u64,
    observed_at: u64,
    trusted_previous_state_root: [u8; 32],
}

impl<T: ProgramSimulationTransport> ReceiptVerifiedProgramSimulator<T> {
    /// Binds the simulation boundary to one verified program bundle; the
    /// trusted previous state root is the chain head's resulting root.
    ///
    /// # Errors
    ///
    /// Refuses a program head whose receipt digest, state root or freshness
    /// differs from the bound chain head.
    pub fn new(
        transport: T,
        registry: ModuleRegistry,
        bound: &VerifiedProgramBundle,
        simulation_public_key: [u8; 32],
    ) -> Result<Self, ProgramOperationError> {
        let program_head = bound.program_head();
        let chain = bound.chain_head();
        if program_head.receipt_digest() != chain.receipt_digest()
            || program_head.state_root() != chain.state_root()
            || program_head.freshness() != chain.freshness()
        {
            return Err(ProgramOperationError::UnverifiedReceipt);
        }
        let mut boundary = b"LayerX/emulator/simulation-boundary/v1\0".to_vec();
        boundary.extend_from_slice(&simulation_public_key);
        Ok(Self {
            transport,
            registry,
            expected_abi_version: program_head.abi_version(),
            expected_program: program_head.program(),
            expected_version: program_head.version(),
            expected_code_hash: program_head.code_hash(),
            simulation_public_key,
            boundary_id: Sha256::digest(boundary).into(),
            receipt_digest: chain.receipt_digest(),
            observed_sequence: chain.freshness().observed_sequence,
            observed_at: chain.freshness().observed_at,
            trusted_previous_state_root: chain.state_root(),
        })
    }
}

trait ProgramSimulationBoundary {
    fn simulate_signed(
        &mut self,
        call: &ProgramCall,
        signed_activity: &[u8],
    ) -> Result<ProgramExecution, ProgramOperationError>;
}

impl<T: ProgramSimulationTransport> ProgramSimulationBoundary
    for ReceiptVerifiedProgramSimulator<T>
{
    fn simulate_signed(
        &mut self,
        call: &ProgramCall,
        signed_activity: &[u8],
    ) -> Result<ProgramExecution, ProgramOperationError> {
        let raw = self.transport.simulate_exact(call, signed_activity)?;
        self.verify_simulation(&raw, signed_activity)
    }
}

impl<T> ReceiptVerifiedProgramSimulator<T> {
    fn verify_simulation(
        &self,
        raw: &RawProgramSimulation,
        signed_activity: &[u8],
    ) -> Result<ProgramExecution, ProgramOperationError> {
        if raw.receipt.is_empty() {
            return Err(ProgramOperationError::UnverifiedReceipt);
        }
        let activity = decode_signed(signed_activity, &self.registry)
            .map_err(|_| ProgramOperationError::InvalidRequest)?;
        let expected = activity_id(&activity).map_err(|_| ProgramOperationError::InvalidRequest)?;
        let verified = verify_program_execution_with_payers(
            &raw.receipt,
            &raw.terminal_payload,
            &raw.call_graph,
            ProgramExecutionExpectation {
                sequencer_public_key: self.simulation_public_key,
                previous_state_root: self.trusted_previous_state_root,
                activity_id: expected,
                payload_hash: layerx_wire::hash::payload_hash(&activity)
                    .map_err(|_| ProgramOperationError::InvalidRequest)?,
                program_id: self.expected_program.bytes(),
                guest_abi_version: self.expected_abi_version,
            },
            &[OccupancyPayer {
                did: activity.actor_did(),
                account: None,
            }],
        )
        .map_err(|_| ProgramOperationError::UnverifiedReceipt)?;
        let protocol = verified
            .receipt()
            .receipt()
            .protocol()
            .ok_or(ProgramOperationError::UnverifiedReceipt)?;
        if protocol.protocol_version() != activity.protocol_version() {
            return Err(ProgramOperationError::UnverifiedReceipt);
        }
        if !raw.evidence.matches_context(
            self.boundary_id,
            expected,
            self.trusted_previous_state_root,
            protocol.resulting_state_root(),
            self.observed_sequence,
            self.observed_at,
        ) || ed25519::verify_digest(
            &self.simulation_public_key,
            &raw.evidence_signature,
            &raw.evidence
                .signing_digest()
                .ok_or(ProgramOperationError::UnverifiedReceipt)?,
        )
        .is_err()
        {
            return Err(ProgramOperationError::UnverifiedReceipt);
        }
        if self.observed_sequence.checked_add(1) != Some(protocol.global_sequence()) {
            return Err(ProgramOperationError::UnverifiedReceipt);
        }
        Ok(ProgramExecution {
            committed: false,
            result_code: verified.result_code(),
            metered_cost: verified.fee_units(),
            fee_units: verified.fee_units(),
            terminal_payload_root: verified.terminal_payload_root(),
            cpu_fuel: verified.cpu_fuel(),
            memory_bytes: verified.memory_bytes(),
            storage_read_bytes: verified.storage_read_bytes(),
            storage_write_bytes: verified.storage_write_bytes(),
            output_values: verified.output_values(),
            output_bytes: verified.output_bytes(),
            outcome: Some(verified.outcome().clone()),
            authenticated_failure: verified.authenticated_failure().cloned(),
            authenticated_resource: verified.authenticated_resource().copied(),
            terminal: verified.terminal().clone(),
            call_graph: verified.call_graph().to_vec(),
            receipt: verified.receipt().canonical_bytes().to_vec(),
        })
    }
}

#[derive(Clone, Copy)]
pub struct ProgramSubmission<'a> {
    pub signer_public_key: [u8; 32],
    pub correlation_id: u64,
    pub attempt: u32,
    pub signed_activity: &'a [u8],
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ProgramTransactionSubmission {
    pub submission: layerx_client::rpc::submit::Submission,
    pub execution: Option<ExecutedReceipt>,
}

#[cfg(test)]
mod node_simulation_transport_contract {
    use super::ProgramSimulationTransport;

    #[test]
    fn node_simulation_transport_holds_a_sequencer_endpoint() {
        let mut endpoint =
            layerx_client::sequencer::SequencerEndpoint::new("http://127.0.0.1:1", &[])
                .expect("loopback sequencer endpoint");
        let registry =
            layerx_types::payload::ModuleRegistry::new(&[]).expect("empty module registry");
        let endpoint_ptr = &endpoint as *const _;
        let mut transport = super::NodeProgramSimulationTransport::new(&mut endpoint, registry, 42);
        assert_eq!(transport.correlation_id, 42);
        assert!(std::ptr::eq(transport.client, endpoint_ptr));
        assert!(matches!(
            transport.simulate_signed_activity(&[]),
            Err(super::ProgramOperationError::InvalidRequest)
        ));
    }
}

pub struct ProgramOperations<R = LayerxdProgramBalanceReader> {
    reader: R,
}

pub(crate) struct ProgramActivityExecution {
    pub(crate) program_id: [u8; 32],
    pub(crate) guest_abi_version: u16,
    pub(crate) execution: ProgramExecution,
    pub(crate) terminal_payload: Vec<u8>,
}

impl ProgramOperations {
    pub fn current_balances(
        config: &crate::node_rpc::NodeRpcConfig,
        program: ProgramId,
        accounts: &[[u8; 32]],
    ) -> Result<(u64, Vec<ProgramAccountBalance>), ProgramOperationError> {
        let node = crate::node_rpc::NodeRpc::connect(config)
            .map_err(|_| ProgramOperationError::Unavailable)?;
        let certificates = if config.url.starts_with("https://") {
            let loaded = rustls_native_certs::load_native_certs();
            if loaded.certs.is_empty() || !loaded.errors.is_empty() {
                return Err(ProgramOperationError::Unavailable);
            }
            loaded.certs
        } else {
            Vec::new()
        };
        let roots: Vec<&[u8]> = certificates.iter().map(AsRef::as_ref).collect();
        let endpoint = layerx_client::sequencer::SequencerEndpoint::new(&config.url, &roots)
            .map_err(|_| ProgramOperationError::Unavailable)?;
        let height = endpoint
            .block_number()
            .map_err(|_| ProgramOperationError::Unavailable)?;
        let balances = Self::value_accounts_at(&node, height, program, accounts)?;
        Ok((height, balances))
    }

    pub fn value_accounts_at(
        node: &crate::node_rpc::NodeRpc,
        height: u64,
        program: ProgramId,
        accounts: &[[u8; 32]],
    ) -> Result<Vec<ProgramAccountBalance>, ProgramOperationError> {
        let selector =
            layerx_crypto::evm_transaction::transaction_hash(b"getValueAccount(bytes32,bytes32)");
        let mut address = [0; 20];
        address[18..].copy_from_slice(&[0x10, 0x1d]);
        let mut balances = Vec::with_capacity(accounts.len());
        for account in accounts {
            let mut data = selector[..4].to_vec();
            data.extend_from_slice(&program.bytes());
            data.extend_from_slice(account);
            let result = node
                .call_at(height, &address, &data)
                .map_err(|_| ProgramOperationError::Unavailable)?;
            if result.len() != 64
                || result[..16] != [0; 16]
                || result[32..63] != [0; 31]
                || result[63] > 1
            {
                return Err(ProgramOperationError::UnverifiedReceipt);
            }
            let balance = u128::from_be_bytes(
                result[16..32]
                    .try_into()
                    .map_err(|_| ProgramOperationError::UnverifiedReceipt)?,
            );
            let exists = result[63] == 1;
            if !exists && balance != 0 {
                return Err(ProgramOperationError::UnverifiedReceipt);
            }
            balances.push(ProgramAccountBalance {
                account: *account,
                balance,
                exists,
            });
        }
        Ok(balances)
    }

    pub(crate) fn activity_execution(
        &self,
        registry: &ModuleRegistry,
        signed_activity: &[u8],
        receipt: &[u8],
        authority: layerx_proof::receipt::AuthorizedBatch,
    ) -> Result<ProgramActivityExecution, ProgramOperationError> {
        use layerx_proof::program::{
            verify_authorized_program_execution_with_payers, AuthorizedProgramExecutionExpectation,
        };
        let activity = decode_signed(signed_activity, registry)
            .map_err(|_| ProgramOperationError::InvalidRequest)?;
        if activity.activity_type().module() != ModuleId::Programs
            || activity.activity_type().ordinal() != 3
        {
            return Err(ProgramOperationError::InvalidRequest);
        }
        let decoded = layerx_wire::receipt::decode(receipt)
            .map_err(|_| ProgramOperationError::UnverifiedReceipt)?;
        let protocol = decoded
            .protocol()
            .ok_or(ProgramOperationError::UnverifiedReceipt)?;
        if protocol.protocol_version() != activity.protocol_version() {
            return Err(ProgramOperationError::UnverifiedReceipt);
        }
        let (program_id, guest_abi_version) = if activity
            .payload()
            .starts_with(layerx_types::intent::PROGRAM_CALL_PAYLOAD_DOMAIN)
        {
            let call = ProgramCall::from_canonical_payload(activity.payload())
                .map_err(|_| ProgramOperationError::InvalidRequest)?;
            (
                call.callee().bytes(),
                protocol
                    .program_outcome()
                    .ok_or(ProgramOperationError::UnverifiedReceipt)?
                    .abi_version(),
            )
        } else {
            if activity.protocol_version() != 3 {
                return Err(ProgramOperationError::InvalidRequest);
            }
            let call = NativeProgramCall::decode(activity.payload())
                .map_err(|_| ProgramOperationError::InvalidRequest)?;
            (call.program_id.bytes(), call.guest_abi)
        };
        let activity_id =
            activity_id(&activity).map_err(|_| ProgramOperationError::InvalidRequest)?;
        let unsigned = layerx_wire::receipt::encode_unsigned(&decoded)
            .map_err(|_| ProgramOperationError::UnverifiedReceipt)?;
        let receipt_digest = layerx_wire::hash::receipt_digest(&unsigned)
            .map_err(|_| ProgramOperationError::UnverifiedReceipt)?;
        let artifacts = self
            .reader
            .read_program_artifacts(activity_id, receipt_digest)
            .map_err(|error| {
                if error.is_unavailable() {
                    ProgramOperationError::Unavailable
                } else {
                    ProgramOperationError::UnverifiedReceipt
                }
            })?;
        let verified = verify_authorized_program_execution_with_payers(
            receipt,
            &artifacts.terminal_payload,
            &artifacts.call_graph,
            &AuthorizedProgramExecutionExpectation {
                authority,
                activity_id,
                payload_hash: layerx_wire::hash::payload_hash(&activity)
                    .map_err(|_| ProgramOperationError::InvalidRequest)?,
                program_id,
                guest_abi_version,
            },
            &[OccupancyPayer {
                did: activity.actor_did(),
                account: None,
            }],
        )
        .map_err(|_| ProgramOperationError::UnverifiedReceipt)?;
        Ok(ProgramActivityExecution {
            program_id,
            guest_abi_version,
            execution: ProgramExecution {
                committed: true,
                result_code: verified.result_code(),
                metered_cost: verified.fee_units(),
                fee_units: verified.fee_units(),
                terminal_payload_root: verified.terminal_payload_root(),
                cpu_fuel: verified.cpu_fuel(),
                memory_bytes: verified.memory_bytes(),
                storage_read_bytes: verified.storage_read_bytes(),
                storage_write_bytes: verified.storage_write_bytes(),
                output_values: verified.output_values(),
                output_bytes: verified.output_bytes(),
                outcome: Some(verified.outcome().clone()),
                authenticated_failure: verified.authenticated_failure().cloned(),
                authenticated_resource: verified.authenticated_resource().copied(),
                terminal: verified.terminal().clone(),
                call_graph: verified.call_graph().to_vec(),
                receipt: verified.receipt().canonical_bytes().to_vec(),
            },
            terminal_payload: artifacts.terminal_payload,
        })
    }

    #[must_use]
    pub const fn new(reader: LayerxdProgramBalanceReader) -> Self {
        Self { reader }
    }

    pub fn current_value_accounts(
        &mut self,
        program: ProgramId,
        now: u64,
    ) -> Result<layerx_programs::VerifiedProgramBalanceRead, ProgramOperationError> {
        self.reader
            .read_verified_balances(program, now)
            .map_err(|error| match error {
                crate::read::ProgramReadError::AccountState(
                    layerx_programs::AccountStateError::StaleRead,
                )
                | crate::read::ProgramReadError::Registry(
                    layerx_programs::RegistryError::StaleRead,
                ) => ProgramOperationError::Stale,
                _ => ProgramOperationError::UnverifiedReceipt,
            })
    }

    /// Reads the one authenticated chain head, requests the kind-5 Programs
    /// state pinned to exactly that head and binds the answer to it.
    ///
    /// # Errors
    ///
    /// Returns `HeadAdvanced` when the pinned head is no longer current,
    /// `ProgramStateAbsent` when the program record or its interface is not
    /// present, `Stale` outside the freshness bound, `UnknownProgram` for a
    /// different program, and `UnverifiedReceipt` for every other refusal.
    pub fn current_program(
        &mut self,
        client: &mut layerx_client::sequencer::SequencerEndpoint,
        program: ProgramId,
        now: u64,
        correlation_id: u64,
    ) -> Result<VerifiedProgramBundle, ProgramOperationError> {
        // Decision 2196: task 9.25.4 ports program state to the EVM precompile.
        let _ = (client, program, now, correlation_id);
        Err(ProgramOperationError::Unavailable)
    }

    ///
    /// # Errors
    ///
    /// Returns an error if the request, program metadata, or execution evidence is invalid, or transport fails.
    pub fn discover(
        &mut self,
        program: ProgramId,
        now: u64,
        bound: &VerifiedProgramBundle,
    ) -> Result<ProgramDiscovery, ProgramOperationError> {
        let head = bound.program_head();
        let chain = bound.chain_head();
        let freshness = chain.freshness();
        let valid_through = freshness
            .observed_at
            .checked_add(self.reader.staleness_limit())
            .ok_or(ProgramOperationError::Stale)?;
        bound
            .verify_discovery(program, now, self.reader.staleness_limit())
            .map_err(|error| match error {
                layerx_programs::ProgramBundleError::ProgramMismatch => {
                    ProgramOperationError::UnknownProgram
                }
                layerx_programs::ProgramBundleError::Evidence(
                    layerx_programs::ProtocolEvidenceError::Stale,
                ) => ProgramOperationError::Stale,
                layerx_programs::ProgramBundleError::Evidence(
                    layerx_programs::ProtocolEvidenceError::LifecycleProof,
                ) => ProgramOperationError::InactiveProgram,
                _ => ProgramOperationError::UnverifiedReceipt,
            })?;
        let accounts = self
            .reader
            .value_account_ids(program)
            .map_err(|_| ProgramOperationError::UnknownProgram)?;
        let config = crate::node_rpc::NodeRpcConfig::from_environment()
            .map_err(|_| ProgramOperationError::Unavailable)?;
        let (_, balances) = Self::current_balances(&config, program, &accounts)?;
        if balances.iter().any(|balance| !balance.exists) {
            return Err(ProgramOperationError::ProgramStateAbsent);
        }
        Ok(ProgramDiscovery {
            program,
            lifecycle: head.lifecycle(),
            observed_sequence: freshness.observed_sequence,
            observed_at: freshness.observed_at,
            valid_through,
            receipt_digest: chain.receipt_digest(),
            state_root: chain.state_root(),
            version: head.version(),
            abi_version: head.abi_version(),
            code_hash: head.code_hash(),
        })
    }

    ///
    /// # Errors
    ///
    /// Returns an error if the request, program metadata, or execution evidence is invalid, or transport fails.
    pub fn interface(
        &mut self,
        program: ProgramId,
        now: u64,
        bound: &VerifiedProgramBundle,
    ) -> Result<ProgramInterfaceRead, ProgramOperationError> {
        let discovery = self.discover(program, now, bound)?;
        let verified = bound.interface();
        if verified.program != program
            || verified.receipt_digest != discovery.receipt_digest
            || verified.state_root != discovery.state_root
            || verified.freshness.observed_sequence != discovery.observed_sequence
            || verified.freshness.observed_at != discovery.observed_at
        {
            return Err(ProgramOperationError::UnverifiedReceipt);
        }
        Ok(ProgramInterfaceRead {
            discovery,
            version: verified.version,
            interface: verified.interface.clone(),
        })
    }

    ///
    /// # Errors
    ///
    /// Returns an error if the request, program metadata, or execution evidence is invalid, or transport fails.
    pub fn simulate(
        &mut self,
        boundary: &mut ReceiptVerifiedProgramSimulator<impl ProgramSimulationTransport>,
        call: &ProgramCall,
        signed_activity: &[u8],
        now: u64,
        bound: &VerifiedProgramBundle,
    ) -> Result<ProgramExecution, ProgramOperationError> {
        let program = ProgramId::new(call.callee().bytes())
            .map_err(|_| ProgramOperationError::InvalidRequest)?;
        let discovery = self.discover(program, now, bound)?;
        if boundary.trusted_previous_state_root != discovery.state_root
            || boundary.receipt_digest != discovery.receipt_digest
            || boundary.expected_abi_version != discovery.abi_version
            || boundary.expected_program != discovery.program
            || boundary.expected_version != discovery.version
            || boundary.expected_code_hash != discovery.code_hash
            || boundary.observed_sequence != discovery.observed_sequence
            || boundary.observed_at != discovery.observed_at
        {
            return Err(ProgramOperationError::UnverifiedReceipt);
        }
        validate_call_activity(boundary.registry(), call, signed_activity)?;
        let execution = boundary.simulate_signed(call, signed_activity)?;
        if execution.committed() || execution.receipt().is_empty() {
            return Err(ProgramOperationError::UnverifiedReceipt);
        }
        Ok(execution)
    }

    ///
    /// # Errors
    ///
    /// Returns an error if the request, program metadata, or execution evidence is invalid, or transport fails.
    pub fn submit(
        &mut self,
        client: &mut layerx_client::sequencer::SequencerEndpoint,
        registry: &ModuleRegistry,
        call: &ProgramCall,
        submission: ProgramSubmission<'_>,
        now: u64,
        bound: &VerifiedProgramBundle,
    ) -> Result<ProgramTransactionSubmission, ProgramOperationError> {
        let ProgramSubmission {
            signer_public_key,
            correlation_id,
            attempt,
            signed_activity,
        } = submission;
        let program = ProgramId::new(call.callee().bytes())
            .map_err(|_| ProgramOperationError::InvalidRequest)?;
        self.discover(program, now, bound)?;
        validate_call_activity(registry, call, signed_activity)?;
        let _ = (signer_public_key, correlation_id, attempt);
        submit_program_transaction(client, signed_activity).map_err(legacy_program_error)
    }
    ///
    /// # Errors
    ///
    /// Returns an error if the request, program metadata, or execution evidence is invalid, or transport fails.
    pub fn simulate_native(
        &mut self,
        boundary: &mut ReceiptVerifiedProgramSimulator<
            impl ProgramSimulationTransport + NativeProgramSimulationTransport,
        >,
        call: NativeProgramCall<'_>,
        fee_limit: u128,
        signed_activity: &[u8],
        now: u64,
        bound: &VerifiedProgramBundle,
    ) -> Result<ProgramExecution, ProgramOperationError> {
        let program = ProgramId::new(call.program_id.bytes())
            .map_err(|_| ProgramOperationError::InvalidRequest)?;
        let discovery = self.discover(program, now, bound)?;
        if boundary.trusted_previous_state_root != discovery.state_root
            || boundary.receipt_digest != discovery.receipt_digest
            || boundary.expected_abi_version != discovery.abi_version
            || boundary.expected_program != discovery.program
            || boundary.expected_version != discovery.version
            || boundary.expected_code_hash != discovery.code_hash
            || boundary.observed_sequence != discovery.observed_sequence
            || boundary.observed_at != discovery.observed_at
        {
            return Err(ProgramOperationError::UnverifiedReceipt);
        }
        validate_native_call_activity(boundary.registry(), call, fee_limit, signed_activity)?;
        let raw = boundary
            .transport
            .simulate_native_exact(call, fee_limit, signed_activity)?;
        let execution = boundary.verify_simulation(&raw, signed_activity)?;
        if execution.committed() || execution.receipt().is_empty() {
            return Err(ProgramOperationError::UnverifiedReceipt);
        }
        Ok(execution)
    }

    ///
    /// # Errors
    ///
    /// Returns an error if the request, program metadata, or execution evidence is invalid, or transport fails.
    pub fn submit_native(
        &mut self,
        client: &mut layerx_client::sequencer::SequencerEndpoint,
        registry: &ModuleRegistry,
        call_with_fee: (NativeProgramCall<'_>, u128),
        submission: ProgramSubmission<'_>,
        now: u64,
        bound: &VerifiedProgramBundle,
    ) -> Result<ProgramTransactionSubmission, ProgramOperationError> {
        let ProgramSubmission {
            signer_public_key,
            correlation_id,
            attempt,
            signed_activity,
        } = submission;
        let (call, fee_limit) = call_with_fee;
        let program = ProgramId::new(call.program_id.bytes())
            .map_err(|_| ProgramOperationError::InvalidRequest)?;
        self.discover(program, now, bound)?;
        validate_native_call_activity(registry, call, fee_limit, signed_activity)?;
        let _ = (signer_public_key, correlation_id, attempt);
        submit_program_transaction(client, signed_activity).map_err(legacy_program_error)
    }
}

impl<T> ReceiptVerifiedProgramSimulator<T> {
    const fn registry(&self) -> &ModuleRegistry {
        &self.registry
    }
}

fn validate_call_activity(
    registry: &ModuleRegistry,
    call: &ProgramCall,
    signed_activity: &[u8],
) -> Result<(), ProgramOperationError> {
    let activity = decode_signed(signed_activity, registry)
        .map_err(|_| ProgramOperationError::InvalidRequest)?;
    let kind = activity.activity_type();
    if kind.module() != ModuleId::Programs
        || kind.ordinal() != 3
        || activity.payload() != call.canonical_payload()
    {
        return Err(ProgramOperationError::InvalidRequest);
    }
    Ok(())
}

/// # Errors
/// Refuses invalid code hashes, payloads, or signed lifecycle activity bindings.
pub fn validate_deploy_activity(
    registry: &ModuleRegistry,
    value: layerx_types::program_lifecycle::NativeProgramDeploy<'_>,
    signed: &[u8],
) -> Result<(), ProgramOperationError> {
    let digest: [u8; 32] = Sha256::digest(value.wasm).into();
    if digest != value.new_hash {
        return Err(ProgramOperationError::InvalidRequest);
    }
    validate_lifecycle_activity(
        registry,
        1,
        &value
            .encode()
            .map_err(|_| ProgramOperationError::InvalidRequest)?,
        signed,
    )
}

/// # Errors
/// Refuses invalid code hashes, upgrade flags, or signed activity bindings.
pub fn validate_upgrade_activity(
    registry: &ModuleRegistry,
    value: layerx_types::program_lifecycle::NativeProgramUpgrade<'_>,
    signed: &[u8],
) -> Result<(), ProgramOperationError> {
    let digest: [u8; 32] = Sha256::digest(value.wasm).into();
    if digest != value.new_hash {
        return Err(ProgramOperationError::InvalidRequest);
    }
    validate_lifecycle_activity(
        registry,
        2,
        &value
            .encode()
            .map_err(|_| ProgramOperationError::InvalidRequest)?,
        signed,
    )
}

/// # Errors
/// Refuses invalid wind-down payloads or signed activity bindings.
pub fn validate_wind_down_activity(
    registry: &ModuleRegistry,
    value: layerx_types::program_lifecycle::NativeProgramWindDown<'_>,
    signed: &[u8],
) -> Result<(), ProgramOperationError> {
    validate_lifecycle_activity(
        registry,
        7,
        &value
            .encode()
            .map_err(|_| ProgramOperationError::InvalidRequest)?,
        signed,
    )
}

fn validate_lifecycle_activity(
    registry: &ModuleRegistry,
    ordinal: u16,
    payload: &[u8],
    signed: &[u8],
) -> Result<(), ProgramOperationError> {
    if signed.is_empty() || signed.len() > 1_048_576 {
        return Err(ProgramOperationError::InvalidRequest);
    }
    let activity =
        decode_signed(signed, registry).map_err(|_| ProgramOperationError::InvalidRequest)?;
    if activity.protocol_version() != 3
        || activity.activity_type().module() != ModuleId::Programs
        || activity.activity_type().ordinal() != ordinal
        || activity.payload() != payload
        || activity.payload_hash()
            != layerx_wire::hash::payload_hash(&activity)
                .map_err(|_| ProgramOperationError::InvalidRequest)?
    {
        return Err(ProgramOperationError::InvalidRequest);
    }
    Ok(())
}

fn validate_native_call_activity(
    registry: &ModuleRegistry,
    call: NativeProgramCall<'_>,
    fee_limit: u128,
    signed_activity: &[u8],
) -> Result<(), ProgramOperationError> {
    if signed_activity.first() == Some(&2) {
        let (transaction, signature) =
            layerx_crypto::evm_transaction::decode_signed(signed_activity)
                .map_err(|_| ProgramOperationError::InvalidRequest)?;
        layerx_crypto::evm_transaction::recover_sender(&transaction, &signature)
            .map_err(|_| ProgramOperationError::InvalidRequest)?;
        let expected = ProgramOperations::<SequencerEndpoint>::call_precompile(call, fee_limit)
            .map_err(legacy_program_error)?;
        if transaction.to != expected.target.address()
            || transaction.data
                != expected
                    .encode()
                    .map_err(|_| ProgramOperationError::InvalidRequest)?
        {
            return Err(ProgramOperationError::InvalidRequest);
        }
        return Ok(());
    }
    let activity = decode_signed(signed_activity, registry)
        .map_err(|_| ProgramOperationError::InvalidRequest)?;
    let payload = call
        .encode()
        .map_err(|_| ProgramOperationError::InvalidRequest)?;
    if activity.protocol_version() != 3
        || activity.activity_type().module() != ModuleId::Programs
        || activity.activity_type().ordinal() != 3
        || activity.fee_limit() != fee_limit
        || activity.payload() != payload
    {
        return Err(ProgramOperationError::InvalidRequest);
    }
    Ok(())
}

#[cfg(test)]
mod native_call_tests {
    use super::*;
    use layerx_types::payload::{ActivityType, ModuleRegistration};

    #[test]
    fn lifecycle_bindings_consume_c_signed_fixtures() -> Result<(), String> {
        use layerx_types::program_lifecycle::{
            NativeProgramDeploy, NativeProgramUpgrade, NativeProgramWindDown,
        };
        let activities = [1, 2, 3, 7]
            .map(|ordinal| ActivityType::new(ModuleId::Programs, ordinal))
            .into_iter()
            .collect::<Result<Vec<_>, _>>()
            .map_err(|error| format!("{error:?}"))?;
        let registration = ModuleRegistration::new(ModuleId::Programs, &activities)
            .map_err(|error| format!("{error:?}"))?;
        let registry =
            ModuleRegistry::new(&[registration]).map_err(|error| format!("{error:?}"))?;
        for (name, ordinal) in [
            ("deploy", 1),
            ("upgrade", 2),
            ("wind-down-route", 7),
            ("wind-down-deprecate", 7),
            ("wind-down-tombstone", 7),
            ("wind-down-exit", 7),
        ] {
            let path = std::path::Path::new(env!("CARGO_MANIFEST_DIR"))
                .join("../../../../sdk/platform/conformance/fixtures")
                .join(format!("native-program-{name}-v3.json"));
            let fixture: serde_json::Value =
                serde_json::from_slice(&std::fs::read(path).map_err(|error| error.to_string())?)
                    .map_err(|error| error.to_string())?;
            let payload =
                decode_hex_json(&fixture, "payload_hex").map_err(|error| format!("{error:?}"))?;
            let signed = decode_hex_json(&fixture, "signed_activity_hex")
                .map_err(|error| format!("{error:?}"))?;
            let result = match ordinal {
                1 => validate_deploy_activity(
                    &registry,
                    NativeProgramDeploy::decode(&payload).map_err(|error| format!("{error:?}"))?,
                    &signed,
                ),
                2 => validate_upgrade_activity(
                    &registry,
                    NativeProgramUpgrade::decode(&payload).map_err(|error| format!("{error:?}"))?,
                    &signed,
                ),
                _ => validate_wind_down_activity(
                    &registry,
                    NativeProgramWindDown::decode(&payload)
                        .map_err(|error| format!("{error:?}"))?,
                    &signed,
                ),
            };
            assert!(result.is_ok());
            let mut wrong_hash = signed.clone();
            let hash_offset = signed.len() - 69 - payload.len() - 5 - 32;
            wrong_hash[hash_offset] ^= 1;
            assert!(decode_signed(&wrong_hash, &registry).is_ok());
            assert!(
                validate_lifecycle_activity(&registry, ordinal, &payload, &wrong_hash).is_err()
            );
            assert!(validate_lifecycle_activity(
                &registry,
                ordinal,
                &payload,
                &signed[..signed.len() - 1]
            )
            .is_err());
            assert!(validate_lifecycle_activity(&registry, 3, &payload, &signed).is_err());
            let mut altered = payload;
            altered[0] ^= 1;
            assert!(validate_lifecycle_activity(&registry, ordinal, &altered, &signed).is_err());
        }
        Ok(())
    }

    #[test]
    fn native_binding_preserves_fee_and_payload() -> Result<(), Box<dyn std::error::Error>> {
        let fixture: serde_json::Value = serde_json::from_str(include_str!(
            "../../../../../../sdk/platform/conformance/fixtures/native-program-call-v3.json"
        ))?;
        let signed = decode_hex_json(&fixture, "signed_activity_hex")
            .map_err(|_| "signed fixture missing")?;
        let payload =
            decode_hex_json(&fixture, "payload_hex").map_err(|_| "payload fixture missing")?;
        let native = NativeProgramCall::decode(&payload).map_err(|_| "invalid fixture")?;
        let registry = ModuleRegistry::new(&[ModuleRegistration::new(
            ModuleId::Programs,
            &[ActivityType::new(ModuleId::Programs, 3).map_err(|_| "activity type invalid")?],
        )
        .map_err(|_| "registration invalid")?])
        .map_err(|_| "registry invalid")?;
        assert!(validate_native_call_activity(&registry, native, 1000, &signed).is_ok());
        assert!(validate_native_call_activity(&registry, native, 999, &signed).is_err());
        assert!(validate_native_call_activity(
            &registry,
            NativeProgramCall {
                response_capacity: 17,
                ..native
            },
            1000,
            &signed
        )
        .is_err());
        Ok(())
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum ProgramRpcFailure {
    ExecutionReverted(layerx_client::sequencer::RpcRefusal),
    Sequencer(layerx_client::sequencer::SequencerError),
    Submission(layerx_client::rpc::submit::SubmitError),
    Signing(crate::sign::SigningError),
}

pub type ProgramRpcOperationError = ProgramOperationError<ProgramRpcFailure>;

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ProgramCallResult {
    pub terminal_result: i32,
    pub terminal_kind: u8,
    pub response: Vec<u8>,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ProgramRegistryRecord {
    pub program: ProgramId,
    pub pin: ProgramStatePin,
    pub policy: u8,
    pub authority: [u8; 32],
    pub code_hash: [u8; 32],
    pub abi_version: u16,
    pub version: u32,
    pub active: bool,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ProgramRegistryInterface {
    pub pin: ProgramStatePin,
    pub version: u32,
    pub digest: [u8; 32],
    pub interface: ProgramInterface,
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct ProgramStatePin {
    pub height: u64,
    pub block_hash: [u8; 32],
    pub observed_at: u64,
}

impl ProgramStatePin {
    fn check_freshness(
        &self,
        now: u64,
        minimum_height: u64,
        maximum_age: u64,
    ) -> Result<(), ProgramRpcOperationError> {
        if self.height < minimum_height
            || now < self.observed_at
            || now
                > self
                    .observed_at
                    .checked_add(maximum_age)
                    .ok_or(ProgramOperationError::Stale)?
        {
            return Err(ProgramOperationError::Stale);
        }
        Ok(())
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct PinnedProgramState {
    pub program: ProgramId,
    pub record: ProgramRegistryRecord,
    pub interface: ProgramRegistryInterface,
    pub state_head: Vec<u8>,
}

fn simulation_error(error: layerx_client::sequencer::SequencerError) -> ProgramRpcOperationError {
    use layerx_client::sequencer::{SequencerError, REVERT_CODE};
    ProgramOperationError::Submit(match error {
        SequencerError::Rpc(refusal) if refusal.code == REVERT_CODE => {
            ProgramRpcFailure::ExecutionReverted(refusal)
        }
        other => ProgramRpcFailure::Sequencer(other),
    })
}

fn node_program_error(error: SequencerError) -> ProgramOperationError {
    match error {
        SequencerError::Unavailable | SequencerError::Tls | SequencerError::Endpoint => {
            ProgramOperationError::Unavailable
        }
        SequencerError::Rpc(refusal) => {
            if refusal.code == -32601 {
                return ProgramOperationError::Unavailable;
            }
            if let Some(data) = refusal.data.as_ref() {
                if let (Some(class), Some(result)) = (
                    data.get("class").and_then(serde_json::Value::as_u64),
                    data.get("result").and_then(serde_json::Value::as_i64),
                ) {
                    if let (Ok(class), Ok(result)) = (u8::try_from(class), i32::try_from(result)) {
                        return ProgramOperationError::CoreRefusal {
                            class,
                            result: ResultCode::from_raw(result),
                        };
                    }
                }
            }
            let reason = refusal
                .data
                .as_ref()
                .and_then(serde_json::Value::as_str)
                .and_then(program_revert_reason);
            match reason.as_deref() {
                Some(reason) if reason.starts_with("LXP_NOT_FOUND: ") => {
                    ProgramOperationError::ProgramStateAbsent
                }
                Some(reason) if reason.starts_with("LXP_HEAD_ADVANCED: ") => {
                    ProgramOperationError::HeadAdvanced
                }
                Some(reason) if reason.starts_with("LXP_STALE: ") => ProgramOperationError::Stale,
                _ if refusal.code == -32602 => ProgramOperationError::InvalidRequest,
                _ => ProgramOperationError::UnverifiedReceipt,
            }
        }
        _ => ProgramOperationError::UnverifiedReceipt,
    }
}

fn program_revert_reason(text: &str) -> Option<String> {
    if text.starts_with("LXP_") {
        return Some(text.to_owned());
    }
    let encoded = text.strip_prefix("0x")?;
    let bytes = decode_program_hex(encoded, layerx_wire::limits::MAX_MESSAGE_BYTES).ok()?;
    let bytes = bytes.strip_prefix(&[0x08, 0xc3, 0x79, 0xa0])?;
    if program_uint(bytes.get(..32)?).ok()? != 32 {
        return None;
    }
    String::from_utf8(program_dynamic(bytes, 32).ok()?).ok()
}

fn legacy_program_error(error: ProgramRpcOperationError) -> ProgramOperationError {
    match error {
        ProgramOperationError::InvalidRequest => ProgramOperationError::InvalidRequest,
        ProgramOperationError::UnknownProgram => ProgramOperationError::UnknownProgram,
        ProgramOperationError::InactiveProgram => ProgramOperationError::InactiveProgram,
        ProgramOperationError::Stale => ProgramOperationError::Stale,
        ProgramOperationError::UnverifiedReceipt => ProgramOperationError::UnverifiedReceipt,
        ProgramOperationError::HeadAdvanced => ProgramOperationError::HeadAdvanced,
        ProgramOperationError::ProgramStateAbsent => ProgramOperationError::ProgramStateAbsent,
        ProgramOperationError::Unavailable => ProgramOperationError::Unavailable,
        ProgramOperationError::CoreRefusal { class, result } => {
            ProgramOperationError::CoreRefusal { class, result }
        }
        ProgramOperationError::Submit(ProgramRpcFailure::ExecutionReverted(refusal)) => {
            node_program_error(SequencerError::Rpc(refusal))
        }
        ProgramOperationError::Submit(ProgramRpcFailure::Sequencer(error)) => {
            node_program_error(error)
        }
        ProgramOperationError::Submit(ProgramRpcFailure::Submission(
            layerx_client::rpc::submit::SubmitError::Refused { refusal, .. },
        )) => node_program_error(SequencerError::Rpc(refusal)),
        ProgramOperationError::Submit(ProgramRpcFailure::Submission(
            layerx_client::rpc::submit::SubmitError::Sequencer(error),
        )) => node_program_error(error),
        ProgramOperationError::Submit(ProgramRpcFailure::Signing(_)) => {
            ProgramOperationError::InvalidRequest
        }
    }
}

fn program_response_capacity(data: &[u8]) -> Result<u32, ProgramOperationError> {
    let word = data
        .get(4 + 6 * 32..4 + 7 * 32)
        .ok_or(ProgramOperationError::InvalidRequest)?;
    u32::try_from(program_uint(word).map_err(legacy_program_error)?)
        .map_err(|_| ProgramOperationError::InvalidRequest)
}

fn validate_program_transaction(
    call: &layerx_crypto::evm_transaction::Eip1559Call,
) -> Result<(), ProgramOperationError> {
    let selector =
        program_selector("call(bytes32,uint16,string,bytes,bytes,bytes,uint32,uint64[7],uint128)");
    if call.to != crate::prepare::evm::PrecompileTarget::Programs.address()
        || call.data.len() < 4 + 15 * 32
        || call.data[..4] != selector
    {
        return Err(ProgramOperationError::InvalidRequest);
    }
    let capacity = program_response_capacity(&call.data)?;
    if capacity > 1_048_576 {
        return Err(ProgramOperationError::InvalidRequest);
    }
    Ok(())
}

fn decode_program_call_result(
    bytes: &[u8],
    capacity: u32,
) -> Result<ProgramCallResult, ProgramRpcOperationError> {
    if bytes.len() < 128 {
        return Err(ProgramOperationError::UnverifiedReceipt);
    }
    let result_bytes = bytes[28..32]
        .try_into()
        .map_err(|_| ProgramOperationError::UnverifiedReceipt)?;
    let terminal_result = i32::from_be_bytes(result_bytes);
    let sign = if terminal_result < 0 { 0xff } else { 0 };
    if bytes[..28].iter().any(|b| *b != sign) || program_uint(&bytes[64..96])? != 96 {
        return Err(ProgramOperationError::UnverifiedReceipt);
    }
    let terminal_kind = u8::try_from(program_uint(&bytes[32..64])?)
        .map_err(|_| ProgramOperationError::UnverifiedReceipt)?;
    let response = program_dynamic(bytes, 96)?;
    if response.len() > capacity as usize {
        return Err(ProgramOperationError::UnverifiedReceipt);
    }
    Ok(ProgramCallResult {
        terminal_result,
        terminal_kind,
        response,
    })
}

fn submit_program_transaction(
    endpoint: &SequencerEndpoint,
    raw: &[u8],
) -> Result<ProgramTransactionSubmission, ProgramRpcOperationError> {
    let (call, signature) = layerx_crypto::evm_transaction::decode_signed(raw)
        .map_err(|_| ProgramOperationError::InvalidRequest)?;
    let sender = layerx_crypto::evm_transaction::recover_sender(&call, &signature)
        .map_err(|_| ProgramOperationError::InvalidRequest)?;
    validate_program_transaction(&call).map_err(|_| ProgramOperationError::InvalidRequest)?;
    let submission = layerx_client::rpc::submit::submit_signed(endpoint, raw)
        .map_err(|e| ProgramOperationError::Submit(ProgramRpcFailure::Submission(e)))?;
    let execution = match &submission {
        layerx_client::rpc::submit::Submission::Submitted { tx_hash, .. } => {
            if *tx_hash != layerx_crypto::evm_transaction::transaction_hash(raw) {
                return Err(ProgramOperationError::UnverifiedReceipt);
            }
            match endpoint
                .sequencer_receipt(tx_hash)
                .map_err(simulation_error)?
            {
                Some(SequencerReceipt::Receipted(receipt))
                | Some(SequencerReceipt::Committed { receipt, .. }) => {
                    if receipt.from != sender
                        || receipt.to != call.to
                        || receipt.nonce != call.nonce
                    {
                        return Err(ProgramOperationError::UnverifiedReceipt);
                    }
                    Some(receipt)
                }
                Some(SequencerReceipt::Submitted { nonce, .. }) if nonce != call.nonce => {
                    return Err(ProgramOperationError::UnverifiedReceipt)
                }
                Some(SequencerReceipt::Submitted { .. }) | None => None,
            }
        }
        layerx_client::rpc::submit::Submission::Unknown(_) => None,
    };
    Ok(ProgramTransactionSubmission {
        submission,
        execution,
    })
}

fn program_selector(signature: &str) -> [u8; 4] {
    let hash = layerx_crypto::evm_transaction::transaction_hash(signature.as_bytes());
    [hash[0], hash[1], hash[2], hash[3]]
}

fn program_word(value: u128) -> [u8; 32] {
    let mut word = [0; 32];
    word[16..].copy_from_slice(&value.to_be_bytes());
    word
}

fn program_uint(word: &[u8]) -> Result<u128, ProgramRpcOperationError> {
    if word.len() != 32 || word[..16].iter().any(|byte| *byte != 0) {
        return Err(ProgramOperationError::UnverifiedReceipt);
    }
    let bytes = word[16..]
        .try_into()
        .map_err(|_| ProgramOperationError::UnverifiedReceipt)?;
    Ok(u128::from_be_bytes(bytes))
}

fn program_dynamic(bytes: &[u8], head_length: usize) -> Result<Vec<u8>, ProgramRpcOperationError> {
    let length_end = head_length
        .checked_add(32)
        .ok_or(ProgramOperationError::UnverifiedReceipt)?;
    let length = usize::try_from(program_uint(
        bytes
            .get(head_length..length_end)
            .ok_or(ProgramOperationError::UnverifiedReceipt)?,
    )?)
    .map_err(|_| ProgramOperationError::UnverifiedReceipt)?;
    let padded = length
        .checked_add(31)
        .map(|n| n & !31)
        .and_then(|n| length_end.checked_add(n))
        .ok_or(ProgramOperationError::UnverifiedReceipt)?;
    let end = length_end
        .checked_add(length)
        .ok_or(ProgramOperationError::UnverifiedReceipt)?;
    if bytes.len() != padded
        || bytes
            .get(end..)
            .ok_or(ProgramOperationError::UnverifiedReceipt)?
            .iter()
            .any(|b| *b != 0)
    {
        return Err(ProgramOperationError::UnverifiedReceipt);
    }
    Ok(bytes
        .get(length_end..end)
        .ok_or(ProgramOperationError::UnverifiedReceipt)?
        .to_vec())
}

impl ProgramOperations<layerx_client::sequencer::SequencerEndpoint> {
    pub fn from_endpoint(endpoint: layerx_client::sequencer::SequencerEndpoint) -> Self {
        Self { reader: endpoint }
    }

    pub fn connect(url: &str, trust_anchors: &[&[u8]]) -> Result<Self, ProgramRpcOperationError> {
        layerx_client::sequencer::SequencerEndpoint::new(url, trust_anchors)
            .map(Self::from_endpoint)
            .map_err(simulation_error)
    }

    pub fn call_precompile(
        call: NativeProgramCall<'_>,
        fee_limit: u128,
    ) -> Result<crate::prepare::evm::PrecompileCall, ProgramRpcOperationError> {
        use crate::prepare::evm::{AbiArgument, PrecompileCall, PrecompileTarget};
        call.encode()
            .map_err(|_| ProgramOperationError::InvalidRequest)?;
        let mut arguments = vec![
            AbiArgument::Word(call.program_id.bytes()),
            AbiArgument::Word(program_word(u128::from(call.guest_abi))),
            AbiArgument::Bytes(call.entrypoint.to_vec()),
            AbiArgument::Bytes(call.calldata.to_vec()),
            AbiArgument::Bytes(call.capabilities.to_vec()),
            AbiArgument::Bytes(call.access_declaration.to_vec()),
            AbiArgument::Word(program_word(u128::from(call.response_capacity))),
        ];
        arguments.extend(
            call.resources
                .0
                .into_iter()
                .map(|n| AbiArgument::Word(program_word(u128::from(n)))),
        );
        arguments.push(AbiArgument::Word(program_word(fee_limit)));
        Ok(PrecompileCall {
            target: PrecompileTarget::Programs,
            selector: program_selector(
                "call(bytes32,uint16,string,bytes,bytes,bytes,uint32,uint64[7],uint128)",
            ),
            arguments,
        })
    }

    pub fn simulate(
        &self,
        actor: [u8; 20],
        call: NativeProgramCall<'_>,
        fee_limit: u128,
    ) -> Result<ProgramCallResult, ProgramRpcOperationError> {
        if actor == [0; 20] {
            return Err(ProgramOperationError::InvalidRequest);
        }
        let precompile = Self::call_precompile(call, fee_limit)?;
        let data = precompile
            .encode()
            .map_err(|_| ProgramOperationError::InvalidRequest)?;
        let bytes = self
            .reader
            .call_pending_from(&actor, &precompile.target.address(), &data)
            .map_err(simulation_error)?;
        decode_program_call_result(&bytes, call.response_capacity)
    }

    pub fn simulate_native(
        &self,
        actor: [u8; 20],
        call: NativeProgramCall<'_>,
        fee_limit: u128,
    ) -> Result<ProgramCallResult, ProgramRpcOperationError> {
        self.simulate(actor, call, fee_limit)
    }

    fn pin(&self) -> Result<ProgramStatePin, ProgramRpcOperationError> {
        let height = self.reader.block_number().map_err(simulation_error)?;
        let block = self
            .reader
            .block_by_number(height)
            .map_err(simulation_error)?
            .ok_or(ProgramOperationError::HeadAdvanced)?;
        Ok(ProgramStatePin {
            height,
            block_hash: block.hash,
            observed_at: block
                .timestamp
                .checked_mul(1000)
                .ok_or(ProgramOperationError::UnverifiedReceipt)?,
        })
    }

    fn check_pin(&self, pin: &ProgramStatePin) -> Result<(), ProgramRpcOperationError> {
        let block = self
            .reader
            .block_by_number(pin.height)
            .map_err(simulation_error)?
            .ok_or(ProgramOperationError::HeadAdvanced)?;
        if block.hash != pin.block_hash
            || block.timestamp.checked_mul(1000) != Some(pin.observed_at)
        {
            return Err(ProgramOperationError::HeadAdvanced);
        }
        Ok(())
    }

    fn registry_call_at(
        &self,
        signature: &str,
        program: ProgramId,
        pin: &ProgramStatePin,
    ) -> Result<Vec<u8>, ProgramRpcOperationError> {
        self.check_pin(pin)?;
        let mut data = program_selector(signature).to_vec();
        data.extend_from_slice(&program.bytes());
        let result = self
            .reader
            .call_at(
                pin.height,
                &crate::prepare::evm::PrecompileTarget::Programs.address(),
                &data,
            )
            .map_err(simulation_error)?;
        self.check_pin(pin)?;
        Ok(result)
    }

    pub fn discover(
        &self,
        program: ProgramId,
    ) -> Result<ProgramRegistryRecord, ProgramRpcOperationError> {
        let pin = self.pin()?;
        self.discover_at(program, &pin)
    }

    pub fn discover_at(
        &self,
        program: ProgramId,
        pin: &ProgramStatePin,
    ) -> Result<ProgramRegistryRecord, ProgramRpcOperationError> {
        let bytes = self.registry_call_at("getProgram(bytes32)", program, pin)?;
        if bytes.len() != 192 {
            return Err(ProgramOperationError::UnverifiedReceipt);
        }
        let narrow = || -> Result<_, ProgramRpcOperationError> {
            Ok((
                u8::try_from(program_uint(&bytes[..32])?)
                    .map_err(|_| ProgramOperationError::UnverifiedReceipt)?,
                u16::try_from(program_uint(&bytes[96..128])?)
                    .map_err(|_| ProgramOperationError::UnverifiedReceipt)?,
                u32::try_from(program_uint(&bytes[128..160])?)
                    .map_err(|_| ProgramOperationError::UnverifiedReceipt)?,
            ))
        };
        let (policy, abi_version, version) = narrow()?;
        let active = match program_uint(&bytes[160..192])? {
            0 => false,
            1 => true,
            _ => return Err(ProgramOperationError::UnverifiedReceipt),
        };
        Ok(ProgramRegistryRecord {
            program,
            pin: *pin,
            policy,
            authority: bytes[32..64]
                .try_into()
                .map_err(|_| ProgramOperationError::UnverifiedReceipt)?,
            code_hash: bytes[64..96]
                .try_into()
                .map_err(|_| ProgramOperationError::UnverifiedReceipt)?,
            abi_version,
            version,
            active,
        })
    }

    pub fn interface(
        &self,
        program: ProgramId,
    ) -> Result<ProgramRegistryInterface, ProgramRpcOperationError> {
        let record = self.discover(program)?;
        self.interface_at(program, &record)
    }

    pub fn interface_at(
        &self,
        program: ProgramId,
        record: &ProgramRegistryRecord,
    ) -> Result<ProgramRegistryInterface, ProgramRpcOperationError> {
        if record.program != program {
            return Err(ProgramOperationError::UnknownProgram);
        }
        let bytes = self.registry_call_at("getInterface(bytes32)", program, &record.pin)?;
        if bytes.len() < 128 || program_uint(&bytes[64..96])? != 96 {
            return Err(ProgramOperationError::UnverifiedReceipt);
        }
        let version = u32::try_from(program_uint(&bytes[..32])?)
            .map_err(|_| ProgramOperationError::UnverifiedReceipt)?;
        let digest: [u8; 32] = bytes[32..64]
            .try_into()
            .map_err(|_| ProgramOperationError::UnverifiedReceipt)?;
        let encoding = program_dynamic(&bytes, 96)?;
        let actual: [u8; 32] = Sha256::digest(&encoding).into();
        if actual != digest {
            return Err(ProgramOperationError::UnverifiedReceipt);
        }
        let interface = ProgramInterface::decode(&encoding)
            .map_err(|_| ProgramOperationError::UnverifiedReceipt)?;
        if version != record.version
            || interface.code_hash() != record.code_hash
            || interface.abi_version() != record.abi_version
        {
            return Err(ProgramOperationError::UnverifiedReceipt);
        }
        Ok(ProgramRegistryInterface {
            pin: record.pin,
            version,
            digest,
            interface,
        })
    }

    pub fn current_program(
        &self,
        program: ProgramId,
        discovery: &ProgramRegistryRecord,
        now: u64,
        minimum_height: u64,
        maximum_age: u64,
    ) -> Result<PinnedProgramState, ProgramRpcOperationError> {
        discovery
            .pin
            .check_freshness(now, minimum_height, maximum_age)?;
        let record = self.discover_at(program, &discovery.pin)?;
        if &record != discovery {
            return Err(ProgramOperationError::UnverifiedReceipt);
        }
        if !record.active {
            return Err(ProgramOperationError::InactiveProgram);
        }
        let interface = self.interface_at(program, &record)?;
        let bytes = self.registry_call_at("getStateHead(bytes32)", program, &record.pin)?;
        if bytes.len() < 64 || program_uint(&bytes[..32])? != 32 {
            return Err(ProgramOperationError::UnverifiedReceipt);
        }
        let state_head = program_dynamic(&bytes, 32)?;
        self.check_pin(&record.pin)?;
        Ok(PinnedProgramState {
            program,
            record,
            interface,
            state_head,
        })
    }

    pub fn submit(
        &self,
        call: NativeProgramCall<'_>,
        fee_limit: u128,
        prepared: &crate::sign::SigningPreparation,
        signed: &[u8],
        registry: &ModuleRegistry,
    ) -> Result<ProgramTransactionSubmission, ProgramRpcOperationError> {
        let expected = Self::call_precompile(call, fee_limit)?;
        let data = expected
            .encode()
            .map_err(|_| ProgramOperationError::InvalidRequest)?;
        if prepared.transaction.call.to != expected.target.address()
            || prepared.transaction.call.data != data
        {
            return Err(ProgramOperationError::InvalidRequest);
        }
        let verified =
            crate::sign::verify_before_submit(signed, prepared, &prepared.actor_address, registry)
                .map_err(|e| ProgramOperationError::Submit(ProgramRpcFailure::Signing(e)))?;
        submit_program_transaction(&self.reader, verified.exact_bytes())
    }

    pub fn submit_native(
        &self,
        call: NativeProgramCall<'_>,
        fee_limit: u128,
        prepared: &crate::sign::SigningPreparation,
        signed: &[u8],
        registry: &ModuleRegistry,
    ) -> Result<ProgramTransactionSubmission, ProgramRpcOperationError> {
        self.submit(call, fee_limit, prepared, signed, registry)
    }
}

#[cfg(test)]
mod evm_program_tests {
    use super::*;
    use layerx_types::payload::{ActivityType, ModuleRegistration};
    use serde_json::{json, Value};
    use std::io::{BufRead, BufReader, Read, Write};
    use std::net::TcpListener;
    use std::thread::JoinHandle;
    use std::time::{Duration, Instant};

    type Exchange = (&'static str, Value);

    fn serve(exchanges: Vec<Exchange>) -> (SequencerEndpoint, JoinHandle<Vec<Value>>) {
        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        listener.set_nonblocking(true).unwrap();
        let endpoint =
            SequencerEndpoint::new(&format!("http://{}", listener.local_addr().unwrap()), &[])
                .unwrap();
        let worker = std::thread::spawn(move || {
            let mut requests = Vec::new();
            for (method, response) in exchanges {
                let deadline = Instant::now() + Duration::from_secs(10);
                let stream = loop {
                    match listener.accept() {
                        Ok((stream, _)) => break stream,
                        Err(error)
                            if error.kind() == std::io::ErrorKind::WouldBlock
                                && Instant::now() < deadline =>
                        {
                            std::thread::sleep(Duration::from_millis(2))
                        }
                        other => panic!("RPC double did not receive {method}: {other:?}"),
                    }
                };
                stream
                    .set_read_timeout(Some(Duration::from_secs(5)))
                    .unwrap();
                stream
                    .set_write_timeout(Some(Duration::from_secs(5)))
                    .unwrap();
                let mut reader = BufReader::new(stream);
                let mut length = None;
                loop {
                    let mut line = String::new();
                    assert_ne!(reader.read_line(&mut line).unwrap(), 0);
                    if line == "\r\n" {
                        break;
                    }
                    if let Some(value) = line.to_ascii_lowercase().strip_prefix("content-length:") {
                        length = Some(value.trim().parse::<usize>().unwrap());
                    }
                }
                let mut body = vec![0; length.unwrap()];
                reader.read_exact(&mut body).unwrap();
                let request: Value = serde_json::from_slice(&body).unwrap();
                assert_eq!(request["jsonrpc"], "2.0");
                assert_eq!(request["method"], method);
                let mut answer = response.as_object().unwrap().clone();
                answer.insert("jsonrpc".to_owned(), json!("2.0"));
                answer.insert("id".to_owned(), request["id"].clone());
                let answer = serde_json::to_string(&answer).unwrap();
                let mut stream = reader.into_inner();
                write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{answer}", answer.len()).unwrap();
                requests.push(request);
            }
            requests
        });
        (endpoint, worker)
    }

    fn hex(bytes: &[u8]) -> String {
        format!("0x{}", encode_hex(bytes))
    }
    fn result(bytes: &[u8]) -> Value {
        json!({"result": hex(bytes)})
    }
    fn registry() -> ModuleRegistry {
        ModuleRegistry::new(&[ModuleRegistration::new(
            ModuleId::Programs,
            &[ActivityType::new(ModuleId::Programs, 3).unwrap()],
        )
        .unwrap()])
        .unwrap()
    }
    fn fixture() -> (Vec<u8>, Vec<u8>) {
        let value: Value = serde_json::from_str(include_str!(
            "../../../../../../sdk/platform/conformance/fixtures/native-program-call-v3.json"
        ))
        .unwrap();
        (
            decode_hex_json(&value, "payload_hex").unwrap(),
            decode_hex_json(&value, "signed_activity_hex").unwrap(),
        )
    }
    fn transaction(
        payload: &[u8],
    ) -> (
        layerx_crypto::evm_transaction::Eip1559Call,
        Vec<u8>,
        [u8; 20],
    ) {
        let call = NativeProgramCall::decode(payload).unwrap();
        let precompile =
            ProgramOperations::<SequencerEndpoint>::call_precompile(call, 1000).unwrap();
        let transaction = layerx_crypto::evm_transaction::Eip1559Call {
            chain_id: 125,
            nonce: 7,
            max_priority_fee_per_gas: 1,
            max_fee_per_gas: 10,
            gas_limit: 1_000_000,
            to: precompile.target.address(),
            data: precompile.encode().unwrap(),
        };
        let raw = transaction.sign(&[0xa5; 32]).unwrap();
        let (_, signature) = layerx_crypto::evm_transaction::decode_signed(&raw).unwrap();
        let sender =
            layerx_crypto::evm_transaction::recover_sender(&transaction, &signature).unwrap();
        (transaction, raw, sender)
    }
    fn call_result(code: i32, kind: u8, response: &[u8]) -> Vec<u8> {
        let mut bytes = vec![if code < 0 { 0xff } else { 0 }; 28];
        bytes.extend_from_slice(&code.to_be_bytes());
        bytes.extend_from_slice(&program_word(u128::from(kind)));
        bytes.extend_from_slice(&program_word(96));
        bytes.extend_from_slice(&program_word(response.len() as u128));
        bytes.extend_from_slice(response);
        bytes.resize((bytes.len() + 31) & !31, 0);
        bytes
    }
    fn block(hash: [u8; 32]) -> Value {
        json!({"result": {"number": "0x7", "hash": hex(&hash), "parentHash": hex(&[4;32]),
            "timestamp": "0x64", "transactionsRoot": hex(&[5;32]), "receiptsRoot": hex(&[6;32]), "transactions": []}})
    }
    fn interface() -> ProgramInterface {
        use layerx_programs::{InterfaceEntryPoint, ValueSchema, ValueType};
        let wasm = [
            0, 97, 115, 109, 1, 0, 0, 0, 1, 12, 2, 96, 2, 127, 127, 1, 127, 96, 1, 127, 1, 127, 3,
            3, 2, 0, 1, 5, 3, 1, 0, 1, 7, 34, 3, 4, b'c', b'a', b'l', b'l', 0, 0, 14, b'l', b'a',
            b'y', b'e', b'r', b'x', b'_', b'r', b'e', b's', b'e', b'r', b'v', b'e', 0, 1, 6, b'm',
            b'e', b'm', b'o', b'r', b'y', 2, 0, 10, 11, 2, 4, 0, 65, 0, 11, 4, 0, 65, 0, 11,
        ];
        ProgramInterface::bind(
            &wasm,
            1,
            vec![InterfaceEntryPoint {
                name: "call".to_owned(),
                discriminator: [1, 2, 3, 4],
                calldata: ValueSchema::layerx(ValueType::Bytes { max_len: 64 }),
                response: ValueSchema::layerx(ValueType::Bytes { max_len: 64 }),
                capabilities: Vec::new(),
                event_topics: Vec::new(),
                failures: Vec::new(),
            }],
        )
        .unwrap()
    }
    fn record_bytes(interface: &ProgramInterface) -> Vec<u8> {
        let mut bytes = program_word(1).to_vec();
        bytes.extend_from_slice(&[0x31; 32]);
        bytes.extend_from_slice(&interface.code_hash());
        bytes.extend_from_slice(&program_word(u128::from(interface.abi_version())));
        bytes.extend_from_slice(&program_word(3));
        bytes.extend_from_slice(&program_word(1));
        bytes
    }
    fn dynamic_output(mut head: Vec<u8>, bytes: &[u8]) -> Vec<u8> {
        let offset = head.len() + 32;
        head.extend_from_slice(&program_word(offset as u128));
        head.extend_from_slice(&program_word(bytes.len() as u128));
        head.extend_from_slice(bytes);
        head.resize((head.len() + 31) & !31, 0);
        head
    }
    fn interface_bytes(interface: &ProgramInterface) -> Vec<u8> {
        let mut head = program_word(3).to_vec();
        head.extend_from_slice(&Sha256::digest(interface.canonical_encoding()));
        dynamic_output(head, interface.canonical_encoding())
    }
    fn read_exchanges(bytes: &[u8], after_hash: [u8; 32]) -> Vec<Exchange> {
        vec![
            ("eth_getBlockByNumber", block([7; 32])),
            ("eth_call", result(bytes)),
            ("eth_getBlockByNumber", block(after_hash)),
        ]
    }
    fn discovery_exchanges(interface: &ProgramInterface) -> Vec<Exchange> {
        let mut exchanges = vec![
            ("eth_blockNumber", json!({"result":"0x7"})),
            ("eth_getBlockByNumber", block([7; 32])),
        ];
        exchanges.extend(read_exchanges(&record_bytes(interface), [7; 32]));
        exchanges
    }

    #[test]
    fn committed_lie_is_refused_before_signature_authority() {
        let (payload, _) = fixture();
        let (transaction, raw, sender) = transaction(&payload);
        let bytes = call_result(0, 1, b"answer");
        let (mut endpoint, server) = serve(vec![
            ("eth_estimateGas", json!({"result":"0x5208"})),
            ("eth_call", result(&bytes)),
        ]);
        let mut transport = NodeProgramSimulationTransport::new(&mut endpoint, registry(), 42);
        let simulation = transport
            .simulate_native_exact(NativeProgramCall::decode(&payload).unwrap(), 1000, &raw)
            .unwrap();
        assert!(simulation.receipt.is_empty());
        assert_eq!(simulation.terminal_payload, bytes);
        assert_eq!(simulation.evidence.signing_digest(), None);
        assert!(!simulation
            .evidence
            .matches_context([0; 32], [0; 32], [0; 32], [0; 32], 0, 0));
        match simulation.evidence {
            ProgramSimulationEvidence::EvmPending {
                activity_id,
                sender: actual,
                to,
                gas_estimate,
                result,
            } => {
                assert_eq!(
                    activity_id,
                    layerx_crypto::evm_transaction::transaction_hash(&raw)
                );
                assert_eq!(actual, sender);
                assert_eq!(to, transaction.to);
                assert_eq!(gas_estimate, 21_000);
                assert_eq!(
                    result,
                    ProgramCallResult {
                        terminal_result: 0,
                        terminal_kind: 1,
                        response: b"answer".to_vec()
                    }
                );
            }
            other => panic!("pending call acquired signature authority: {other:?}"),
        }
        let requests = server.join().unwrap();
        let expected = json!({"from": hex(&sender), "to": hex(&transaction.to), "data": hex(&transaction.data)});
        assert_eq!(requests[0]["params"], json!([expected, "pending"]));
        assert_eq!(requests[1]["params"], requests[0]["params"]);
        let mut lie = bytes;
        lie.extend_from_slice(&program_word(1));
        let (mut endpoint, server) = serve(vec![
            ("eth_estimateGas", json!({"result":"0x5208"})),
            ("eth_call", result(&lie)),
        ]);
        let mut transport = NodeProgramSimulationTransport::new(&mut endpoint, registry(), 42);
        assert!(matches!(
            transport.simulate_signed_activity(&raw),
            Err(ProgramOperationError::UnverifiedReceipt)
        ));
        assert_eq!(server.join().unwrap().len(), 2);
    }

    #[test]
    fn stale_discovery_root_is_refused() {
        let interface = interface();
        let mut exchanges = discovery_exchanges(&interface);
        exchanges.extend(read_exchanges(&interface_bytes(&interface), [8; 32]));
        let (endpoint, server) = serve(exchanges);
        let programs = ProgramOperations::from_endpoint(endpoint);
        let program = ProgramId::new([0x11; 32]).unwrap();
        let discovery = programs.discover(program).unwrap();
        assert_eq!(discovery.pin.block_hash, [7; 32]);
        assert_eq!(
            programs.interface_at(program, &discovery),
            Err(ProgramOperationError::HeadAdvanced)
        );
        let requests = server.join().unwrap();
        let reads: Vec<_> = requests
            .iter()
            .filter(|r| r["method"] == "eth_call")
            .collect();
        assert_eq!(reads.len(), 2);
        for request in &reads {
            assert_eq!(request["params"][1], "0x7");
            assert_eq!(
                request["params"][0]["to"],
                hex(&crate::prepare::evm::PrecompileTarget::Programs.address())
            );
        }
        assert_eq!(
            reads[1]["params"][0]["data"],
            hex(&[
                program_selector("getInterface(bytes32)").as_slice(),
                &program.bytes()
            ]
            .concat())
        );
    }

    #[test]
    fn stale_sequence_and_freshness_are_refused() {
        let (endpoint, server) = serve(discovery_exchanges(&interface()));
        let programs = ProgramOperations::from_endpoint(endpoint);
        let program = ProgramId::new([0x11; 32]).unwrap();
        let discovery = programs.discover(program).unwrap();
        assert_eq!(discovery.pin.observed_at, 100_000);
        for (now, minimum) in [(100_000, 8), (99_999, 7), (105_001, 7), (u64::MAX, 7)] {
            assert_eq!(
                programs.current_program(program, &discovery, now, minimum, 5000),
                Err(ProgramOperationError::Stale)
            );
        }
        assert_eq!(discovery.pin.check_freshness(105_000, 7, 5000), Ok(()));
        assert_eq!(
            discovery.pin.check_freshness(100_000, 7, u64::MAX),
            Err(ProgramOperationError::Stale)
        );
        assert_eq!(server.join().unwrap().len(), 5);
    }

    #[test]
    fn node_simulation_refusals_keep_their_existing_paths() {
        let (payload, _) = fixture();
        let (_, raw, _) = transaction(&payload);
        for (error, expected) in [
            (
                json!({"code":-32601,"message":"method unavailable"}),
                ProgramOperationError::Unavailable,
            ),
            (
                json!({"code":-32602,"message":"invalid parameters"}),
                ProgramOperationError::InvalidRequest,
            ),
            (
                json!({"code":3,"message":"refused","data":{"class":5,"result":-736}}),
                ProgramOperationError::CoreRefusal {
                    class: 5,
                    result: ResultCode::from_raw(-736),
                },
            ),
            (
                json!({"code":3,"message":"execution reverted","data":"LXP_NOT_FOUND: program"}),
                ProgramOperationError::ProgramStateAbsent,
            ),
            (
                json!({"code":3,"message":"execution reverted","data":"LXP_HEAD_ADVANCED: read"}),
                ProgramOperationError::HeadAdvanced,
            ),
            (
                json!({"code":3,"message":"execution reverted","data":"LXP_STALE: read"}),
                ProgramOperationError::Stale,
            ),
            (
                json!({"code":3,"message":"opaque refusal","data":"0x1234"}),
                ProgramOperationError::UnverifiedReceipt,
            ),
        ] {
            for estimate in [true, false] {
                let exchanges = if estimate {
                    vec![("eth_estimateGas", json!({"error":error}))]
                } else {
                    vec![
                        ("eth_estimateGas", json!({"result":"0x5208"})),
                        ("eth_call", json!({"error":error})),
                    ]
                };
                let (mut endpoint, server) = serve(exchanges);
                let mut transport =
                    NodeProgramSimulationTransport::new(&mut endpoint, registry(), 42);
                assert!(
                    matches!(transport.simulate_signed_activity(&raw), Err(actual) if actual==expected)
                );
                assert_eq!(server.join().unwrap().len(), if estimate { 1 } else { 2 });
            }
        }
        let mut revert = vec![0x08, 0xc3, 0x79, 0xa0];
        revert.extend(dynamic_output(Vec::new(), b"LXP_NOT_FOUND: program"));
        let (mut endpoint, server) = serve(vec![(
            "eth_estimateGas",
            json!({"error":{"code":3,"message":"execution reverted","data":hex(&revert)}}),
        )]);
        let mut transport = NodeProgramSimulationTransport::new(&mut endpoint, registry(), 42);
        assert!(matches!(
            transport.simulate_signed_activity(&raw),
            Err(ProgramOperationError::ProgramStateAbsent)
        ));
        assert_eq!(server.join().unwrap().len(), 1);
    }

    fn executed_receipt(raw: &[u8], sender: &[u8; 20], success: bool, committed: bool) -> Value {
        let mut value = json!({"rung":"receipted", "txHash":hex(&layerx_crypto::evm_transaction::transaction_hash(raw)),
            "nonce":"0x7", "seq":"0x2a", "height":"0x7", "index":"0x0", "timestamp":"0x64",
            "status":if success {"0x1"} else {"0x0"}, "returnData":"0xaabb", "logs":[],
            "gasUsed":"0x5208", "cumulativeGasUsed":"0x5208", "effectiveGasPrice":"0x1", "feeDenom":"upax", "feeAmount":"0x5208",
            "sponsor":hex(&[0;20]), "from":hex(sender), "to":hex(&crate::prepare::evm::PrecompileTarget::Programs.address()),
            "contractAddress":hex(&[0;20]), "vmError":if success {""} else {"execution reverted"},
            "keyId":hex(&[1;32]), "receipt":"0x01", "signingKey":hex(&[2;32])});
        if committed {
            value["rung"] = json!("committed");
            value["blockNumber"] = json!("0x7");
            value["blockHash"] = json!(hex(&[7; 32]));
        }
        value
    }

    #[test]
    fn program_submission_reads_its_receipt_at_the_submitted_rung() {
        let (payload, _) = fixture();
        let (transaction, raw, sender) = transaction(&payload);
        let registry = registry();
        let disclosure = layerx_crypto::disclosure::call::bind_call(&transaction, sender).unwrap();
        let prepared = crate::sign::SigningPreparation {
            transaction: crate::prepare::EvmPrepared {
                signing_hash: transaction.signing_hash(),
                call: transaction.clone(),
                from: sender,
                disclosure_digest: disclosure.signing_digest().unwrap(),
                disclosure,
                observed_head_height: 7,
            },
            actor_address: sender,
        };
        for (success, committed) in [(true, false), (false, false), (false, true)] {
            let hash = layerx_crypto::evm_transaction::transaction_hash(&raw);
            let receipt = executed_receipt(&raw, &sender, success, committed);
            let (endpoint, server) = serve(vec![
                ("eth_sendRawTransaction", json!({"result":hex(&hash)})),
                ("pax_sequencerReceipt", json!({"result":receipt})),
            ]);
            let programs = ProgramOperations::from_endpoint(endpoint);
            let submitted = programs
                .submit(
                    NativeProgramCall::decode(&payload).unwrap(),
                    1000,
                    &prepared,
                    &raw,
                    &registry,
                )
                .unwrap();
            assert_eq!(
                submitted.submission,
                layerx_client::rpc::submit::Submission::Submitted {
                    tx_hash: hash,
                    rung: layerx_types::rung::Rung::Submitted
                }
            );
            let execution = submitted.execution.unwrap();
            assert_eq!(execution.success, success);
            assert_eq!(execution.return_data, [0xaa, 0xbb]);
            assert_eq!(
                execution.vm_error,
                if success { "" } else { "execution reverted" }
            );
            assert_eq!(execution.tx_hash, hash);
            assert_eq!(execution.from, sender);
            let requests = server.join().unwrap();
            assert_eq!(requests[0]["params"], json!([hex(&raw)]));
            assert_eq!(requests[1]["params"], json!([hex(&hash)]));
        }
    }

    #[test]
    fn program_state_methods_share_the_discovery_pin() {
        let interface = interface();
        let mut exchanges = discovery_exchanges(&interface);
        exchanges.extend(read_exchanges(&record_bytes(&interface), [7; 32]));
        exchanges.extend(read_exchanges(&interface_bytes(&interface), [7; 32]));
        let mut state_head = vec![0, 1, 0, 0, 0, 0];
        state_head.extend_from_slice(&Sha256::digest(&state_head));
        exchanges.extend(read_exchanges(
            &dynamic_output(Vec::new(), &state_head),
            [7; 32],
        ));
        exchanges.push(("eth_getBlockByNumber", block([7; 32])));
        let (endpoint, server) = serve(exchanges);
        let programs = ProgramOperations::from_endpoint(endpoint);
        let program = ProgramId::new([0x11; 32]).unwrap();
        let discovery = programs.discover(program).unwrap();
        let read = programs
            .current_program(program, &discovery, 105_000, 7, 5000)
            .unwrap();
        assert_eq!(read.record, discovery);
        assert_eq!(read.interface.interface, interface);
        assert_eq!(read.interface.pin, discovery.pin);
        assert_eq!(read.state_head, state_head);
        let requests = server.join().unwrap();
        let calls: Vec<_> = requests
            .iter()
            .filter(|r| r["method"] == "eth_call")
            .collect();
        assert_eq!(calls.len(), 4);
        for (call, signature) in calls.iter().zip([
            "getProgram(bytes32)",
            "getProgram(bytes32)",
            "getInterface(bytes32)",
            "getStateHead(bytes32)",
        ]) {
            assert_eq!(call["params"][1], "0x7");
            assert_eq!(
                call["params"][0]["data"],
                hex(&[program_selector(signature).as_slice(), &program.bytes()].concat())
            );
        }
    }
}
