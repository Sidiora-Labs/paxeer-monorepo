//! Live EVM Programs transport. A node summary authenticates the outer transaction
//! and its logs. It must never be presented as the historical kernel/state proof.
use layerx_crypto::evm_transaction::{decode_signed, recover_sender, transaction_hash};
use layerx_types::rung::Rung;
use layerx_wire::receipt::Receipt;
use serde_json::{json, Value};

use super::{
    http::HttpProgramTransport, NativeProgramLifecycleRequest, ProgramOperationError,
    ProgramSubmission, PROGRAMS_PRECOMPILE,
};
use crate::rpc_verification::{verify_node_receipt, ReceiptPolicy};

pub struct NodeProgramExecution {
    receipt: Receipt,
    canonical_receipt: Vec<u8>,
    signed_transaction: Vec<u8>,
    program_id: [u8; 32],
    idempotency_key: [u8; 32],
    terminal_result: Option<i32>,
    terminal_kind: Option<u8>,
    response: Vec<u8>,
    guest_abi: u16,
}
impl NodeProgramExecution {
    #[must_use]
    pub const fn receipt(&self) -> &Receipt {
        &self.receipt
    }
    #[must_use]
    pub fn canonical_receipt(&self) -> &[u8] {
        &self.canonical_receipt
    }
    #[must_use]
    pub fn signed_transaction(&self) -> &[u8] {
        &self.signed_transaction
    }
    #[must_use]
    pub fn transaction_hash(&self) -> [u8; 32] {
        transaction_hash(&self.signed_transaction)
    }
    #[must_use]
    pub const fn program_id(&self) -> [u8; 32] {
        self.program_id
    }
    #[must_use]
    pub const fn idempotency_key(&self) -> [u8; 32] {
        self.idempotency_key
    }
    #[must_use]
    pub const fn terminal_result(&self) -> Option<i32> {
        self.terminal_result
    }
    #[must_use]
    pub const fn terminal_kind(&self) -> Option<u8> {
        self.terminal_kind
    }
    #[must_use]
    pub fn response(&self) -> &[u8] {
        &self.response
    }
    #[must_use]
    pub const fn guest_abi(&self) -> u16 {
        self.guest_abi
    }
    #[must_use]
    #[allow(
        clippy::unused_self,
        reason = "Preserve the public execution.rung() accessor contract for receipt evidence."
    )]
    pub const fn rung(&self) -> Rung {
        Rung::Receipted
    }
}

/// A block-consistent node read. Cryptographic state authority is not implied.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct NodeProgramRead {
    pub block_number: u64,
    pub block_hash: [u8; 32],
    pub bytes: Vec<u8>,
}

fn bytes(value: &Value) -> Result<Vec<u8>, ProgramOperationError> {
    let text = value
        .as_str()
        .and_then(|s| s.strip_prefix("0x"))
        .ok_or(ProgramOperationError::Decode)?;
    if text.len() % 2 != 0
        || text.len() > super::MAX_SIGNED_ACTIVITY_BYTES * 2
        || !text.bytes().all(|b| matches!(b, b'0'..=b'9' | b'a'..=b'f'))
    {
        return Err(ProgramOperationError::Bounds);
    }
    text.as_bytes()
        .chunks_exact(2)
        .map(|p| {
            u8::from_str_radix(
                std::str::from_utf8(p).map_err(|_| ProgramOperationError::Decode)?,
                16,
            )
            .map_err(|_| ProgramOperationError::Decode)
        })
        .collect()
}
fn hex(bytes: &[u8]) -> String {
    format!("0x{}", crate::rpc::encode_hex(bytes))
}
fn fixed<const N: usize>(value: &Value) -> Result<[u8; N], ProgramOperationError> {
    bytes(value)?
        .try_into()
        .map_err(|_| ProgramOperationError::Decode)
}
fn quantity(value: &Value) -> Result<u64, ProgramOperationError> {
    let text = value
        .as_str()
        .and_then(|s| s.strip_prefix("0x"))
        .ok_or(ProgramOperationError::Decode)?;
    if text.is_empty()
        || (text.len() > 1 && text.starts_with('0'))
        || !text.bytes().all(|b| matches!(b, b'0'..=b'9' | b'a'..=b'f'))
    {
        return Err(ProgramOperationError::Decode);
    }
    u64::from_str_radix(text, 16).map_err(|_| ProgramOperationError::Decode)
}
fn uint(word: &[u8]) -> Result<u64, ProgramOperationError> {
    if word.len() != 32 || word[..24] != [0; 24] {
        return Err(ProgramOperationError::Decode);
    }
    Ok(u64::from_be_bytes(
        word[24..]
            .try_into()
            .map_err(|_| ProgramOperationError::Decode)?,
    ))
}
fn signed_result(word: &[u8]) -> Result<i32, ProgramOperationError> {
    if word.len() != 32 {
        return Err(ProgramOperationError::Decode);
    }
    let result = i32::from_be_bytes(
        word[28..]
            .try_into()
            .map_err(|_| ProgramOperationError::Decode)?,
    );
    if word[..28]
        .iter()
        .any(|b| *b != if result < 0 { 255 } else { 0 })
    {
        return Err(ProgramOperationError::Decode);
    }
    Ok(result)
}

impl HttpProgramTransport {
    /// Configures independently pinned receipt authority before contacting the node.
    /// Live operations still verify the node's network before sending a transaction or reading.
    ///
    /// # Errors
    /// Refuses an unsupported receipt protocol or an unspecified network.
    pub fn with_node_receipt_policy(
        mut self,
        policy: ReceiptPolicy,
    ) -> Result<Self, ProgramOperationError> {
        if policy.protocol_version != 2 || policy.network_id == 0 {
            return Err(ProgramOperationError::Verification);
        }
        self.node_policy = Some(policy);
        Ok(self)
    }

    /// Connects to live node JSON-RPC with independently configured network and height-ranged receipt trust.
    /// # Errors
    ///
    /// Refuses a policy outside protocol version 2 or with network ID zero, invalid connection configuration, or a node whose chain ID differs from the policy.
    pub fn connect_node_with_policy(
        endpoint: &str,
        credential: Option<super::LayerXKeyCredential>,
        policy: ReceiptPolicy,
        clock: fn() -> Result<u64, ProgramOperationError>,
    ) -> Result<Self, ProgramOperationError> {
        if policy.protocol_version != 2 || policy.network_id == 0 {
            return Err(ProgramOperationError::Verification);
        }
        let transport = Self::connect(endpoint, credential, policy.sequencer.public_key(), clock)?
            .with_node_receipt_policy(policy)?;
        transport.check_node_network()?;
        Ok(transport)
    }
    fn check_node_network(&self) -> Result<&ReceiptPolicy, ProgramOperationError> {
        let policy = self
            .node_policy
            .as_ref()
            .ok_or(ProgramOperationError::MissingNodeReadAuthority)?;
        if quantity(&self.node_rpc("eth_chainId", &json!([]))?)? != u64::from(policy.network_id) {
            return Err(ProgramOperationError::Verification);
        }
        Ok(policy)
    }
    pub(super) fn send_node_transaction(
        &self,
        raw: &[u8],
        expected: [u8; 32],
    ) -> Result<(), ProgramOperationError> {
        let policy = self.check_node_network()?;
        let (call, signature) = decode_signed(raw).map_err(|_| ProgramOperationError::Decode)?;
        recover_sender(&call, &signature).map_err(|_| ProgramOperationError::Verification)?;
        if call.to != PROGRAMS_PRECOMPILE
            || call.chain_id != u64::from(policy.network_id)
            || transaction_hash(raw) != expected
        {
            return Err(ProgramOperationError::IdentityMismatch);
        }
        let result = self.node_rpc("eth_sendRawTransaction", &json!([hex(raw)]))?;
        if fixed::<32>(&result)? != expected {
            return Err(ProgramOperationError::IdentityMismatch);
        }
        Ok(())
    }
    pub(super) fn node_simulate(&self, raw: &[u8]) -> Result<Vec<u8>, ProgramOperationError> {
        let policy = self.check_node_network()?;
        let (call, signature) = decode_signed(raw).map_err(|_| ProgramOperationError::Decode)?;
        let from =
            recover_sender(&call, &signature).map_err(|_| ProgramOperationError::Verification)?;
        if call.to != PROGRAMS_PRECOMPILE || call.chain_id != u64::from(policy.network_id) {
            return Err(ProgramOperationError::IdentityMismatch);
        }
        bytes(&self.node_rpc("eth_call",&json!([{"from":hex(&from),"to":hex(&call.to),"data":hex(&call.data),"gas":format!("0x{:x}",call.gas_limit)},"pending"]))?)
    }
    /// Reads a currently served registry method at a fixed, rechecked committed block.
    /// # Errors
    ///
    /// Refuses a zero program or unsupported method, missing network authority, malformed RPC results, transport errors, or a block that changes during the read.
    pub fn node_read(
        &self,
        program: [u8; 32],
        method: &str,
    ) -> Result<NodeProgramRead, ProgramOperationError> {
        if program == [0; 32]
            || !matches!(
                method,
                "getProgram(bytes32)" | "getInterface(bytes32)" | "getStateHead(bytes32)"
            )
        {
            return Err(ProgramOperationError::IdentityMismatch);
        }
        self.check_node_network()?;
        let block = self.node_rpc("eth_getBlockByNumber", &json!(["latest", false]))?;
        let height = quantity(&block["number"])?;
        let hash = fixed::<32>(&block["hash"])?;
        let mut data = layerx_crypto::disclosure::call::ledger_selector(method).to_vec();
        data.extend_from_slice(&program);
        let selector = format!("0x{height:x}");
        let result = bytes(&self.node_rpc(
            "eth_call",
            &json!([{"to":hex(&PROGRAMS_PRECOMPILE),"data":hex(&data)},selector]),
        )?)?;
        let again = self.node_rpc("eth_getBlockByNumber", &json!([selector, false]))?;
        if quantity(&again["number"])? != height || fixed::<32>(&again["hash"])? != hash {
            return Err(ProgramOperationError::Verification);
        }
        Ok(NodeProgramRead {
            block_number: height,
            block_hash: hash,
            bytes: result,
        })
    }
    pub(super) fn node_lifecycle_receipt(
        &self,
        request: &NativeProgramLifecycleRequest,
    ) -> Result<Receipt, ProgramOperationError> {
        let policy = self.check_node_network()?;
        let hash = request.bound_activity_id();
        let raw = bytes(&self.node_rpc("eth_getRawTransactionByHash", &json!([hex(&hash)]))?)?;
        if raw != request.signed_activity() {
            return Err(ProgramOperationError::IdentityMismatch);
        }
        let value = self.node_rpc("pax_sequencerReceipt", &json!([hex(&hash)]))?;
        let (receipt, canonical) = verify_node_receipt(&value, hash, policy)
            .map_err(|_| ProgramOperationError::Verification)?;
        bind_transaction(&receipt, &raw)?;
        crate::program_lifecycle::verify_lifecycle_receipt(
            &canonical,
            hash,
            std::slice::from_ref(&policy.sequencer),
        )?;
        if receipt
            .protocol()
            .ok_or(ProgramOperationError::Verification)?
            .result_code()
            == 0
        {
            let (call, _) = decode_signed(&raw).map_err(|_| ProgramOperationError::Decode)?;
            let topic = crate::program_lifecycle::lifecycle_event_topic(&call.data)?;
            let logs = value["logs"]
                .as_array()
                .ok_or(ProgramOperationError::Decode)?;
            if !logs.iter().any(|log| {
                log["address"] == hex(&PROGRAMS_PRECOMPILE)
                    && log["topics"].as_array().map(Vec::len) == Some(2)
                    && log["topics"][0] == format!("0x{topic}")
                    && log["topics"][1] == hex(&call.data[4..36])
            }) {
                return Err(ProgramOperationError::Verification);
            }
        }
        Ok(receipt)
    }
    pub(super) fn node_recover(
        &self,
        hash: [u8; 32],
        expected_key: Option<[u8; 32]>,
    ) -> Result<ProgramSubmission, ProgramOperationError> {
        let policy = self.check_node_network()?;
        if hash == [0; 32] {
            return Err(ProgramOperationError::IdentityMismatch);
        }
        let raw = self.node_rpc("eth_getRawTransactionByHash", &json!([hex(&hash)]))?;
        if raw.is_null() {
            return Err(ProgramOperationError::UnknownOutcome);
        }
        let raw = bytes(&raw)?;
        let transaction = super::http::signed_program_transaction(&raw)?;
        if transaction.activity_id != hash
            || expected_key.is_some_and(|key| key != transaction.idempotency_key)
        {
            return Err(ProgramOperationError::IdentityMismatch);
        }
        let value = self.node_rpc("pax_sequencerReceipt", &json!([hex(&hash)]))?;
        if value.is_null() || value["rung"] == "submitted" {
            if !value.is_null() {
                let (call, _) = decode_signed(&raw).map_err(|_| ProgramOperationError::Decode)?;
                if fixed::<32>(&value["txHash"])? != hash
                    || quantity(&value["nonce"])? != call.nonce
                {
                    return Err(ProgramOperationError::IdentityMismatch);
                }
            }
            return Ok(ProgramSubmission::Submitted {
                activity_id: hash,
                idempotency_key: transaction.idempotency_key,
                retained_signed_activity: raw,
            });
        }
        let execution = verify_node_program_execution(&value, &raw, policy)?;
        Ok(ProgramSubmission::Receipted(execution))
    }
}

fn bind_transaction(receipt: &Receipt, raw: &[u8]) -> Result<(), ProgramOperationError> {
    let (call, signature) = decode_signed(raw).map_err(|_| ProgramOperationError::Decode)?;
    let sender =
        recover_sender(&call, &signature).map_err(|_| ProgramOperationError::Verification)?;
    let protocol = receipt
        .protocol()
        .ok_or(ProgramOperationError::Verification)?;
    let mut from = [0; 32];
    from[12..].copy_from_slice(&sender);
    let mut to = [0; 32];
    to[12..].copy_from_slice(&call.to);
    if call.to != PROGRAMS_PRECOMPILE
        || protocol.module_id() != 12
        || protocol.from() != from
        || protocol.to() != to
        || protocol.debit_sequence() != call.nonce
        || protocol.activity_id() != transaction_hash(raw)
    {
        return Err(ProgramOperationError::IdentityMismatch);
    }
    Ok(())
}

/// Verifies a real node summary, signed Programs outcome log, canonical ABI return,
/// requested ABI and response capacity. No kernel proof is inferred from JSON.
/// # Errors
///
/// Refuses an unauthenticated receipt, mismatched transaction or Programs outcome, malformed ABI return, or response exceeding the signed capacity.
pub fn verify_node_program_execution(
    value: &Value,
    raw: &[u8],
    policy: &ReceiptPolicy,
) -> Result<NodeProgramExecution, ProgramOperationError> {
    let transaction = super::http::signed_program_transaction(raw)?;
    let (call, _fee) = layerx_crypto::disclosure::call::decode_program_call(&transaction.data)
        .map_err(|_| ProgramOperationError::Decode)?;
    let (outer, _) = decode_signed(raw).map_err(|_| ProgramOperationError::Decode)?;
    if outer.chain_id != u64::from(policy.network_id) {
        return Err(ProgramOperationError::IdentityMismatch);
    }
    let (receipt, canonical_receipt) = verify_node_receipt(value, transaction.activity_id, policy)
        .map_err(|_| ProgramOperationError::Verification)?;
    bind_transaction(&receipt, raw)?;
    let mut result = NodeProgramExecution {
        receipt,
        canonical_receipt,
        signed_transaction: raw.to_vec(),
        program_id: call.program_id.bytes(),
        idempotency_key: transaction.idempotency_key,
        terminal_result: None,
        terminal_kind: None,
        response: Vec::new(),
        guest_abi: call.guest_abi,
    };
    if result
        .receipt
        .protocol()
        .ok_or(ProgramOperationError::Verification)?
        .result_code()
        != 0
    {
        return Ok(result);
    }
    let returned = bytes(&value["returnData"])?;
    if returned.len() < 128 || uint(&returned[64..96])? != 96 {
        return Err(ProgramOperationError::Decode);
    }
    let terminal = signed_result(&returned[..32])?;
    let kind = u8::try_from(uint(&returned[32..64])?).map_err(|_| ProgramOperationError::Decode)?;
    if !matches!(kind, 1..=3) || (kind == 1) != (terminal == 0) {
        return Err(ProgramOperationError::Verification);
    }
    let length =
        usize::try_from(uint(&returned[96..128])?).map_err(|_| ProgramOperationError::Bounds)?;
    let end = 128_usize
        .checked_add(length)
        .ok_or(ProgramOperationError::Bounds)?;
    let padded = end.checked_add(31).ok_or(ProgramOperationError::Bounds)? & !31;
    if length
        > usize::try_from(call.response_capacity).map_err(|_| ProgramOperationError::Decode)?
        || padded != returned.len()
        || returned[end..].iter().any(|b| *b != 0)
    {
        return Err(ProgramOperationError::Bounds);
    }
    let topic = hex(&layerx_crypto::evm_transaction::transaction_hash(b"ProgramCallOutcome(bytes32,bytes32,bytes32,uint64,uint8,uint16,uint16,uint32,uint32,int32,bytes32,bytes32,bytes32,bytes32)"));
    let logs = value["logs"]
        .as_array()
        .ok_or(ProgramOperationError::Decode)?;
    let mut outcomes = logs
        .iter()
        .filter(|log| log["address"] == hex(&PROGRAMS_PRECOMPILE) && log["topics"][0] == topic);
    let outcome = outcomes.next().ok_or(ProgramOperationError::Verification)?;
    if outcomes.next().is_some()
        || outcome["topics"].as_array().map(Vec::len) != Some(2)
        || fixed::<32>(&outcome["topics"][1])? != call.program_id.bytes()
    {
        return Err(ProgramOperationError::Verification);
    }
    let data = bytes(&outcome["data"])?;
    if data.len() != 13 * 32 {
        return Err(ProgramOperationError::Decode);
    }
    let word = |i: usize| &data[i * 32..(i + 1) * 32];
    // Root frame, registered runtime/guest, nonzero schedule versions, exact terminal result.
    if uint(word(2))? != 0
        || uint(word(3))? != 0
        || uint(word(4))? != 1
        || uint(word(5))? != u64::from(call.guest_abi)
        || !layerx_types::guest_abi::supported(call.guest_abi)
        || uint(word(6))? == 0
        || uint(word(6))? > u64::from(u32::MAX)
        || uint(word(7))? == 0
        || uint(word(7))? > u64::from(u32::MAX)
        || signed_result(word(8))? != terminal
        || word(0) == [0; 32]
        || word(1) == [0; 32]
    {
        return Err(ProgramOperationError::Verification);
    }
    result.terminal_result = Some(terminal);
    result.terminal_kind = Some(kind);
    result.response = returned[128..end].to_vec();
    Ok(result)
}
