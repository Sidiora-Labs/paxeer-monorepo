use layerx_proof::inclusion::SequencerAuthorization;
use layerx_proof::receipt::{verify_sequencer_signature, ReceiptKeys};
use serde::Deserialize;
use serde_json::{json, Value};

use crate::rpc::{RpcCluster, RpcError, RpcQuorumConfig};

#[derive(Clone, Debug, Deserialize, Eq, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct ReceiptKeyConfig {
    pub sequencer_id_hex: String,
    pub public_key_hex: String,
    pub first_height: u64,
    pub last_height: u64,
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct CommittedSourceConfig {
    pub rpc: RpcQuorumConfig,
    pub chain_id: u64,
    pub genesis_hash_hex: String,
    pub receipt_keys: Vec<ReceiptKeyConfig>,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct CommittedBlock {
    pub header: CommittedHeader,
    pub receipts: Vec<Vec<u8>>,
}

pub struct CommittedBlockSource {
    rpc: RpcCluster,
    receipt_keys: Vec<SequencerAuthorization>,
}

impl CommittedBlockSource {
    pub fn new(config: &CommittedSourceConfig) -> Result<Self, CommittedRefusal> {
        Self::open(config)
    }

    pub fn open(config: &CommittedSourceConfig) -> Result<Self, CommittedRefusal> {
        let pin = |text: &str| {
            parse_hash(&Value::String(format!(
                "0x{}",
                text.strip_prefix("0x").unwrap_or(text)
            )))
        };
        let genesis_hash = pin(&config.genesis_hash_hex)?;
        if config.receipt_keys.is_empty() {
            return Err(CommittedRefusal::ReceiptSignature);
        }
        let receipt_keys = config
            .receipt_keys
            .iter()
            .map(|key| {
                if key.first_height > key.last_height {
                    return Err(CommittedRefusal::ReceiptSignature);
                }
                Ok(SequencerAuthorization::new(
                    pin(&key.sequencer_id_hex)?,
                    pin(&key.public_key_hex)?,
                    key.first_height,
                    key.last_height,
                ))
            })
            .collect::<Result<Vec<_>, _>>()?;
        let rpc = RpcCluster::new(&config.rpc).map_err(rpc_refusal)?;
        let chain = rpc.call("eth_chainId", json!([])).map_err(rpc_refusal)?;
        if parse_quantity(&chain)? != config.chain_id {
            return Err(CommittedRefusal::ChainMismatch);
        }
        let genesis = rpc
            .call("eth_getBlockByNumber", json!(["0x0", false]))
            .map_err(rpc_refusal)?;
        if parse_quantity(&genesis["number"])? != 0 || parse_hash(&genesis["hash"])? != genesis_hash
        {
            return Err(CommittedRefusal::GenesisMismatch);
        }
        Ok(Self { rpc, receipt_keys })
    }

    pub fn latest_committed(
        &self,
        previous: Option<CommittedCoordinate>,
    ) -> Result<CommittedCoordinate, CommittedRefusal> {
        let block = self
            .rpc
            .call("eth_getBlockByNumber", json!(["finalized", false]))
            .map_err(rpc_refusal)?;
        if block.is_null() {
            return Err(CommittedRefusal::Unavailable);
        }
        let coordinate = CommittedCoordinate {
            height: parse_quantity(&block["number"])?,
            block_hash: parse_hash(&block["hash"])?,
        };
        if previous.is_some_and(|previous| coordinate.height < previous.height) {
            return Err(CommittedRefusal::HeightRegressed);
        }
        Ok(coordinate)
    }

    pub fn acquire_next(
        &self,
        previous: CommittedCoordinate,
    ) -> Result<CommittedBlock, CommittedRefusal> {
        let height = previous
            .height
            .checked_add(1)
            .ok_or(CommittedRefusal::Malformed)?;
        let block = self.acquire(height)?;
        if block.header.parent_hash != previous.block_hash {
            return Err(CommittedRefusal::ParentMismatch);
        }
        Ok(block)
    }

    pub fn acquire(&self, height: u64) -> Result<CommittedBlock, CommittedRefusal> {
        let block = self
            .rpc
            .call(
                "eth_getBlockByNumber",
                json!([format!("0x{height:x}"), false]),
            )
            .map_err(rpc_refusal)?;
        if block.is_null() {
            return Err(CommittedRefusal::Unavailable);
        }
        let header = CommittedHeader::from_block_json(&block)?;
        if header.height != height {
            return Err(CommittedRefusal::NotCommitted);
        }
        let mut receipts = Vec::with_capacity(header.transactions.len());
        for (index, transaction) in header.transactions.iter().enumerate() {
            let answer = self
                .rpc
                .call(
                    "pax_sequencerReceipt",
                    json!([block["transactions"][index]]),
                )
                .map_err(rpc_refusal)?;
            if answer["rung"].as_str() != Some("committed")
                || parse_quantity(&answer["blockNumber"]).ok() != Some(header.height)
                || parse_hash(&answer["blockHash"]).ok() != Some(header.block_hash)
            {
                return Err(CommittedRefusal::NotCommitted);
            }
            let bytes = parse_bytes(&answer["receipt"])?;
            let verified =
                verify_sequencer_signature(&bytes, ReceiptKeys::HeightRanges(&self.receipt_keys))
                    .map_err(|_| CommittedRefusal::ReceiptSignature)?;
            let protocol = verified
                .protocol()
                .ok_or(CommittedRefusal::ReceiptSignature)?;
            let receipt_height = u64::from_be_bytes(
                protocol.batch_id()[24..]
                    .try_into()
                    .map_err(|_| CommittedRefusal::Malformed)?,
            );
            if protocol.activity_id() != *transaction
                || parse_hash(&answer["txHash"])? != *transaction
                || receipt_height > height
            {
                return Err(CommittedRefusal::NotCommitted);
            }
            receipts.push(bytes);
        }
        Ok(CommittedBlock { header, receipts })
    }
}

fn rpc_refusal(error: RpcError) -> CommittedRefusal {
    match error {
        RpcError::Divergence => CommittedRefusal::Divergence,
        RpcError::Unavailable | RpcError::RateLimited { .. } | RpcError::Rejected { .. } => {
            CommittedRefusal::Unavailable
        }
        RpcError::Configuration | RpcError::ResponseMismatch => CommittedRefusal::Malformed,
    }
}

fn parse_bytes(value: &Value) -> Result<Vec<u8>, CommittedRefusal> {
    let digits = value
        .as_str()
        .and_then(|text| text.strip_prefix("0x"))
        .ok_or(CommittedRefusal::Malformed)?;
    if digits.len() % 2 != 0 || !digits.bytes().all(|byte| byte.is_ascii_hexdigit()) {
        return Err(CommittedRefusal::Malformed);
    }
    (0..digits.len())
        .step_by(2)
        .map(|index| {
            u8::from_str_radix(&digits[index..index + 2], 16)
                .map_err(|_| CommittedRefusal::Malformed)
        })
        .collect()
}

#[derive(Clone, Copy, Debug, Eq, PartialEq, Ord, PartialOrd)]
pub struct CommittedCoordinate {
    pub height: u64,
    pub block_hash: [u8; 32],
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct CommittedHeader {
    pub height: u64,
    pub block_hash: [u8; 32],
    pub parent_hash: [u8; 32],
    pub app_hash: [u8; 32],
    pub timestamp: u64,
    pub transactions: Vec<[u8; 32]>,
}

impl CommittedHeader {
    pub fn from_block_json(block: &Value) -> Result<Self, CommittedRefusal> {
        let transactions = block
            .get("transactions")
            .and_then(Value::as_array)
            .ok_or(CommittedRefusal::Malformed)?
            .iter()
            .map(parse_hash)
            .collect::<Result<Vec<_>, _>>()?;
        Ok(Self {
            height: parse_quantity(&block["number"])?,
            block_hash: parse_hash(&block["hash"])?,
            parent_hash: parse_hash(&block["parentHash"])?,
            app_hash: parse_hash(&block["stateRoot"])?,
            timestamp: parse_quantity(&block["timestamp"])?,
            transactions,
        })
    }
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum CommittedRefusal {
    Unavailable,
    ChainMismatch,
    GenesisMismatch,
    Divergence,
    NotCommitted,
    ReceiptSignature,
    ParentMismatch,
    HeightRegressed,
    Malformed,
}

impl CommittedRefusal {
    #[must_use]
    pub const fn code(self) -> &'static str {
        match self {
            Self::Unavailable => "committed_unavailable",
            Self::ChainMismatch => "committed_chain_mismatch",
            Self::GenesisMismatch => "committed_genesis_mismatch",
            Self::Divergence => "committed_divergence",
            Self::NotCommitted => "committed_not_committed",
            Self::ReceiptSignature => "committed_receipt_signature",
            Self::ParentMismatch => "committed_parent_mismatch",
            Self::HeightRegressed => "committed_height_regressed",
            Self::Malformed => "committed_malformed",
        }
    }
}

fn parse_quantity(value: &Value) -> Result<u64, CommittedRefusal> {
    let digits = value
        .as_str()
        .and_then(|text| text.strip_prefix("0x"))
        .ok_or(CommittedRefusal::Malformed)?;
    if digits.is_empty()
        || (digits.len() > 1 && digits.starts_with('0'))
        || !digits.bytes().all(|byte| byte.is_ascii_hexdigit())
    {
        return Err(CommittedRefusal::Malformed);
    }
    u64::from_str_radix(digits, 16).map_err(|_| CommittedRefusal::Malformed)
}

fn parse_hash(value: &Value) -> Result<[u8; 32], CommittedRefusal> {
    let digits = value
        .as_str()
        .and_then(|text| text.strip_prefix("0x"))
        .ok_or(CommittedRefusal::Malformed)?;
    if digits.len() != 64 || !digits.bytes().all(|byte| byte.is_ascii_hexdigit()) {
        return Err(CommittedRefusal::Malformed);
    }
    let mut hash = [0; 32];
    for (index, byte) in hash.iter_mut().enumerate() {
        *byte = u8::from_str_radix(&digits[index * 2..index * 2 + 2], 16)
            .map_err(|_| CommittedRefusal::Malformed)?;
    }
    Ok(hash)
}
