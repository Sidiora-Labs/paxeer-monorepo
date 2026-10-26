use serde_json::Value;

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
