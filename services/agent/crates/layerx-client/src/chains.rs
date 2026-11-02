use layerx_crypto::evm_transaction::transaction_hash;

use crate::sequencer::{SequencerEndpoint, SequencerError};

pub const CHAINS_PRECOMPILE: [u8; 20] = [
    0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x10, 0x1e,
];
pub const VIRTUAL_CHAIN_ID_BASE: u64 = 125_000_000;
pub const METHOD_SIGNATURES: [&str; 12] = [
    "createChain(string,uint64,uint64,uint16)",
    "fundChain(uint32)",
    "pauseChain(uint32)",
    "resumeChain(uint32)",
    "retireChain(uint32)",
    "governancePause(uint32,bool)",
    "setOperator(uint32,address)",
    "getChain(uint32)",
    "chainByName(string)",
    "chainCount()",
    "chainIdOf(uint32)",
    "params()",
];
pub const EVENT_SIGNATURES: [&str; 4] = [
    "ChainCreated(uint32,string,address,uint64)",
    "ChainStatusChanged(uint32,uint8)",
    "ChainFunded(uint32,address,uint256)",
    "ChainOperatorChanged(uint32,address)",
];

#[derive(Clone, Debug, PartialEq, Eq)]
pub enum ChainsError {
    Encoding,
    Rpc(SequencerError),
}

impl std::fmt::Display for ChainsError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::Encoding => f.write_str("invalid chain registry ABI encoding"),
            Self::Rpc(error) => write!(f, "chain registry RPC failed: {error:?}"),
        }
    }
}

impl std::error::Error for ChainsError {}

#[derive(Clone, Debug, PartialEq, Eq)]
pub enum ChainCall {
    CreateChain {
        name: String,
        gas_allowance: u64,
        byte_allowance: u64,
        fee_share_bps: u16,
    },
    FundChain(u32),
    PauseChain(u32),
    ResumeChain(u32),
    RetireChain(u32),
    GovernancePause {
        id: u32,
        paused: bool,
    },
    SetOperator {
        id: u32,
        operator: [u8; 20],
    },
    GetChain(u32),
    ChainByName(String),
    ChainCount,
    ChainIdOf(u32),
    Params,
}

impl ChainCall {
    pub const fn signature(&self) -> &'static str {
        METHOD_SIGNATURES[match self {
            Self::CreateChain { .. } => 0,
            Self::FundChain(_) => 1,
            Self::PauseChain(_) => 2,
            Self::ResumeChain(_) => 3,
            Self::RetireChain(_) => 4,
            Self::GovernancePause { .. } => 5,
            Self::SetOperator { .. } => 6,
            Self::GetChain(_) => 7,
            Self::ChainByName(_) => 8,
            Self::ChainCount => 9,
            Self::ChainIdOf(_) => 10,
            Self::Params => 11,
        }]
    }

    pub fn selector(&self) -> [u8; 4] {
        let hash = transaction_hash(self.signature().as_bytes());
        [hash[0], hash[1], hash[2], hash[3]]
    }
}

pub fn encode_call(call: &ChainCall) -> Vec<u8> {
    let mut bytes = call.selector().to_vec();
    let args = match call {
        ChainCall::CreateChain {
            name,
            gas_allowance,
            byte_allowance,
            fee_share_bps,
        } => with_string(
            vec![
                uint(128),
                uint(*gas_allowance),
                uint(*byte_allowance),
                uint(u64::from(*fee_share_bps)),
            ],
            name,
        ),
        ChainCall::FundChain(id)
        | ChainCall::PauseChain(id)
        | ChainCall::ResumeChain(id)
        | ChainCall::RetireChain(id)
        | ChainCall::GetChain(id)
        | ChainCall::ChainIdOf(id) => uint(u64::from(*id)).to_vec(),
        ChainCall::GovernancePause { id, paused } => {
            [uint(u64::from(*id)), uint(u64::from(*paused))].concat()
        }
        ChainCall::SetOperator { id, operator } => {
            [uint(u64::from(*id)), address_word(operator)].concat()
        }
        ChainCall::ChainByName(name) => with_string(vec![uint(32)], name),
        ChainCall::ChainCount | ChainCall::Params => Vec::new(),
    };
    bytes.extend_from_slice(&args);
    bytes
}

pub fn decode_call(bytes: &[u8]) -> Result<ChainCall, ChainsError> {
    let selector = bytes.get(..4).ok_or(ChainsError::Encoding)?;
    let index = METHOD_SIGNATURES
        .iter()
        .position(|signature| selector == &transaction_hash(signature.as_bytes())[..4])
        .ok_or(ChainsError::Encoding)?;
    let args = &bytes[4..];
    let call = match index {
        0 => ChainCall::CreateChain {
            name: string(args, 4, 0)?,
            gas_allowance: number(args, 1)?,
            byte_allowance: number(args, 2)?,
            fee_share_bps: number(args, 3)?,
        },
        1 => ChainCall::FundChain(number(args, 0)?),
        2 => ChainCall::PauseChain(number(args, 0)?),
        3 => ChainCall::ResumeChain(number(args, 0)?),
        4 => ChainCall::RetireChain(number(args, 0)?),
        5 => {
            let paused: u8 = number(args, 1)?;
            if paused > 1 {
                return Err(ChainsError::Encoding);
            }
            ChainCall::GovernancePause {
                id: number(args, 0)?,
                paused: paused == 1,
            }
        }
        6 => ChainCall::SetOperator {
            id: number(args, 0)?,
            operator: address(args, 1)?,
        },
        7 => ChainCall::GetChain(number(args, 0)?),
        8 => ChainCall::ChainByName(string(args, 1, 0)?),
        9 => ChainCall::ChainCount,
        10 => ChainCall::ChainIdOf(number(args, 0)?),
        11 => ChainCall::Params,
        _ => return Err(ChainsError::Encoding),
    };
    if encode_call(&call) != bytes {
        return Err(ChainsError::Encoding);
    }
    Ok(call)
}

#[derive(Clone, Debug, PartialEq, Eq)]
pub struct ChainRecord {
    pub id: u32,
    pub name: String,
    pub operator: [u8; 20],
    pub status: u8,
    pub chain_id: u64,
    pub gas_allowance: u64,
    pub byte_allowance: u64,
    pub fee_share_bps: u16,
    pub bond: [u8; 32],
    pub base_fee_per_gas: [u8; 32],
    pub created_height: u64,
}

impl ChainRecord {
    pub fn encode(&self) -> Vec<u8> {
        with_string(
            vec![
                uint(u64::from(self.id)),
                uint(352),
                address_word(&self.operator),
                uint(u64::from(self.status)),
                uint(self.chain_id),
                uint(self.gas_allowance),
                uint(self.byte_allowance),
                uint(u64::from(self.fee_share_bps)),
                self.bond,
                self.base_fee_per_gas,
                uint(self.created_height),
            ],
            &self.name,
        )
    }
}

pub fn decode_get_chain(bytes: &[u8]) -> Result<ChainRecord, ChainsError> {
    Ok(ChainRecord {
        id: number(bytes, 0)?,
        name: string(bytes, 11, 1)?,
        operator: address(bytes, 2)?,
        status: number(bytes, 3)?,
        chain_id: number(bytes, 4)?,
        gas_allowance: number(bytes, 5)?,
        byte_allowance: number(bytes, 6)?,
        fee_share_bps: number(bytes, 7)?,
        bond: word(bytes, 8)?,
        base_fee_per_gas: word(bytes, 9)?,
        created_height: number(bytes, 10)?,
    })
}

#[derive(Clone, Debug, PartialEq, Eq)]
pub struct ChainsParams {
    pub authority: [u8; 20],
    pub min_bond: [u8; 32],
    pub creation_fee: [u8; 32],
    pub min_gas_allowance: u64,
    pub max_gas_allowance: u64,
    pub total_gas_allowance_left: u64,
    pub max_chains: u32,
}

impl ChainsParams {
    pub fn encode(&self) -> Vec<u8> {
        [
            address_word(&self.authority),
            self.min_bond,
            self.creation_fee,
            uint(self.min_gas_allowance),
            uint(self.max_gas_allowance),
            uint(self.total_gas_allowance_left),
            uint(u64::from(self.max_chains)),
        ]
        .concat()
    }
}

pub fn decode_params(bytes: &[u8]) -> Result<ChainsParams, ChainsError> {
    exact_words(bytes, 7)?;
    Ok(ChainsParams {
        authority: address(bytes, 0)?,
        min_bond: word(bytes, 1)?,
        creation_fee: word(bytes, 2)?,
        min_gas_allowance: number(bytes, 3)?,
        max_gas_allowance: number(bytes, 4)?,
        total_gas_allowance_left: number(bytes, 5)?,
        max_chains: number(bytes, 6)?,
    })
}

#[derive(Clone, Debug, PartialEq, Eq)]
pub enum ChainEvent {
    Created {
        id: u32,
        name: String,
        operator: [u8; 20],
        chain_id: u64,
    },
    StatusChanged {
        id: u32,
        status: u8,
    },
    Funded {
        id: u32,
        from: [u8; 20],
        amount: [u8; 32],
    },
    OperatorChanged {
        id: u32,
        operator: [u8; 20],
    },
}

impl ChainEvent {
    pub const fn signature(&self) -> &'static str {
        EVENT_SIGNATURES[match self {
            Self::Created { .. } => 0,
            Self::StatusChanged { .. } => 1,
            Self::Funded { .. } => 2,
            Self::OperatorChanged { .. } => 3,
        }]
    }

    pub fn topic0(&self) -> [u8; 32] {
        transaction_hash(self.signature().as_bytes())
    }

    pub fn encode(&self) -> (Vec<[u8; 32]>, Vec<u8>) {
        let mut topics = vec![self.topic0()];
        let data = match self {
            Self::Created {
                id,
                name,
                operator,
                chain_id,
            } => {
                topics.extend([uint(u64::from(*id)), address_word(operator)]);
                with_string(vec![uint(64), uint(*chain_id)], name)
            }
            Self::StatusChanged { id, status } => {
                topics.push(uint(u64::from(*id)));
                uint(u64::from(*status)).to_vec()
            }
            Self::Funded { id, from, amount } => {
                topics.extend([uint(u64::from(*id)), address_word(from)]);
                amount.to_vec()
            }
            Self::OperatorChanged { id, operator } => {
                topics.extend([uint(u64::from(*id)), address_word(operator)]);
                Vec::new()
            }
        };
        (topics, data)
    }
}

pub fn decode_event(topics: &[[u8; 32]], data: &[u8]) -> Result<ChainEvent, ChainsError> {
    let topic0 = topics.first().ok_or(ChainsError::Encoding)?;
    let index = EVENT_SIGNATURES
        .iter()
        .position(|signature| *topic0 == transaction_hash(signature.as_bytes()))
        .ok_or(ChainsError::Encoding)?;
    let expected_topics = if index == 1 { 2 } else { 3 };
    if topics.len() != expected_topics {
        return Err(ChainsError::Encoding);
    }
    let id = number(&topics[1], 0)?;
    let event = match index {
        0 => ChainEvent::Created {
            id,
            name: string(data, 2, 0)?,
            operator: address(&topics[2], 0)?,
            chain_id: number(data, 1)?,
        },
        1 => {
            exact_words(data, 1)?;
            ChainEvent::StatusChanged {
                id,
                status: number(data, 0)?,
            }
        }
        2 => {
            exact_words(data, 1)?;
            ChainEvent::Funded {
                id,
                from: address(&topics[2], 0)?,
                amount: word(data, 0)?,
            }
        }
        3 => {
            exact_words(data, 0)?;
            ChainEvent::OperatorChanged {
                id,
                operator: address(&topics[2], 0)?,
            }
        }
        _ => return Err(ChainsError::Encoding),
    };
    Ok(event)
}

pub fn virtual_chain_id(id: u32) -> u64 {
    VIRTUAL_CHAIN_ID_BASE + u64::from(id)
}

pub fn chain_rpc_url(base: &str, id: u32) -> String {
    format!("{}/chain/{id}", base.trim_end_matches('/'))
}

#[derive(Debug)]
pub struct ChainsReads<'a> {
    endpoint: &'a SequencerEndpoint,
}

impl<'a> ChainsReads<'a> {
    pub const fn new(endpoint: &'a SequencerEndpoint) -> Self {
        Self { endpoint }
    }

    pub fn get_chain(&self, id: u32) -> Result<ChainRecord, ChainsError> {
        decode_get_chain(&self.read(&ChainCall::GetChain(id))?)
    }

    pub fn chain_by_name(&self, name: &str) -> Result<u32, ChainsError> {
        self.read_number(&ChainCall::ChainByName(name.to_owned()))
    }

    pub fn chain_count(&self) -> Result<u32, ChainsError> {
        self.read_number(&ChainCall::ChainCount)
    }

    pub fn chain_id_of(&self, id: u32) -> Result<u64, ChainsError> {
        self.read_number(&ChainCall::ChainIdOf(id))
    }

    pub fn params(&self) -> Result<ChainsParams, ChainsError> {
        decode_params(&self.read(&ChainCall::Params)?)
    }

    fn read(&self, call: &ChainCall) -> Result<Vec<u8>, ChainsError> {
        self.endpoint
            .call_pending(&CHAINS_PRECOMPILE, &encode_call(call))
            .map_err(ChainsError::Rpc)
    }

    fn read_number<T: TryFrom<u64>>(&self, call: &ChainCall) -> Result<T, ChainsError> {
        let bytes = self.read(call)?;
        exact_words(&bytes, 1)?;
        number(&bytes, 0)
    }
}

fn uint(value: u64) -> [u8; 32] {
    let mut word = [0; 32];
    word[24..].copy_from_slice(&value.to_be_bytes());
    word
}

fn address_word(value: &[u8; 20]) -> [u8; 32] {
    let mut word = [0; 32];
    word[12..].copy_from_slice(value);
    word
}

fn with_string(head: Vec<[u8; 32]>, text: &str) -> Vec<u8> {
    let mut bytes = head.concat();
    bytes.extend_from_slice(&uint(text.len() as u64));
    bytes.extend_from_slice(text.as_bytes());
    bytes.resize(bytes.len().next_multiple_of(32), 0);
    bytes
}

fn word(bytes: &[u8], index: usize) -> Result<[u8; 32], ChainsError> {
    let start = index.checked_mul(32).ok_or(ChainsError::Encoding)?;
    let end = start.checked_add(32).ok_or(ChainsError::Encoding)?;
    bytes
        .get(start..end)
        .and_then(|value| value.try_into().ok())
        .ok_or(ChainsError::Encoding)
}

fn number<T: TryFrom<u64>>(bytes: &[u8], index: usize) -> Result<T, ChainsError> {
    let word = word(bytes, index)?;
    if word[..24].iter().any(|byte| *byte != 0) {
        return Err(ChainsError::Encoding);
    }
    let mut low = [0; 8];
    low.copy_from_slice(&word[24..]);
    T::try_from(u64::from_be_bytes(low)).map_err(|_| ChainsError::Encoding)
}

fn address(bytes: &[u8], index: usize) -> Result<[u8; 20], ChainsError> {
    let word = word(bytes, index)?;
    if word[..12].iter().any(|byte| *byte != 0) {
        return Err(ChainsError::Encoding);
    }
    let mut address = [0; 20];
    address.copy_from_slice(&word[12..]);
    Ok(address)
}

fn exact_words(bytes: &[u8], words: usize) -> Result<(), ChainsError> {
    if bytes.len() != words * 32 {
        return Err(ChainsError::Encoding);
    }
    Ok(())
}

fn string(bytes: &[u8], head_words: usize, index: usize) -> Result<String, ChainsError> {
    let offset: usize = number(bytes, index)?;
    if offset != head_words * 32 {
        return Err(ChainsError::Encoding);
    }
    let length: usize = number(bytes, head_words)?;
    let start = offset.checked_add(32).ok_or(ChainsError::Encoding)?;
    let end = start.checked_add(length).ok_or(ChainsError::Encoding)?;
    let padded_end = end.checked_add(31).ok_or(ChainsError::Encoding)? / 32 * 32;
    if bytes.len() != padded_end {
        return Err(ChainsError::Encoding);
    }
    let text = bytes.get(start..end).ok_or(ChainsError::Encoding)?;
    if bytes
        .get(end..padded_end)
        .ok_or(ChainsError::Encoding)?
        .iter()
        .any(|byte| *byte != 0)
    {
        return Err(ChainsError::Encoding);
    }
    String::from_utf8(text.to_vec()).map_err(|_| ChainsError::Encoding)
}
