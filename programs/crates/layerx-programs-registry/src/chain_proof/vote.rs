use crate::chain_proof::{
    proto::{ProtoReader, ProtoValue},
    ChainProofError,
};

pub const SIGNED_MSG_TYPE_PRECOMMIT: u64 = 2;

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
#[repr(u8)]
pub enum BlockIdFlag {
    Absent = 1,
    Commit = 2,
    Nil = 3,
}

impl TryFrom<u64> for BlockIdFlag {
    type Error = ChainProofError;

    fn try_from(value: u64) -> Result<Self, Self::Error> {
        match value {
            1 => Ok(Self::Absent),
            2 => Ok(Self::Commit),
            3 => Ok(Self::Nil),
            _ => Err(ChainProofError::NonCanonical),
        }
    }
}

#[derive(Clone, Copy, Debug, Default, Eq, PartialEq)]
pub struct Timestamp {
    pub seconds: i64,
    pub nanos: i32,
}

impl Timestamp {
    pub const ZERO: Self = Self {
        seconds: -62_135_596_800,
        nanos: 0,
    };

    fn validate(self) -> Result<(), ChainProofError> {
        if !(-62_135_596_800..=253_402_300_799).contains(&self.seconds)
            || !(0..1_000_000_000).contains(&self.nanos)
        {
            return Err(ChainProofError::NonCanonical);
        }
        Ok(())
    }

    pub fn decode(bytes: &[u8]) -> Result<Self, ChainProofError> {
        let mut timestamp = Self::default();
        let mut reader = ProtoReader::new(bytes);
        let mut previous = 0;
        while let Some(field) = reader.next_field()? {
            if field.number == previous {
                return Err(ChainProofError::NonCanonical);
            }
            previous = field.number;
            match (field.number, field.value) {
                (1, ProtoValue::Varint(value)) => {
                    timestamp.seconds = i64::from_le_bytes(value.to_le_bytes());
                }
                (2, ProtoValue::Varint(value)) => {
                    timestamp.nanos =
                        i32::try_from(value).map_err(|_| ChainProofError::NonCanonical)?;
                }
                (1 | 2, _) => return Err(ChainProofError::NonCanonical),
                _ => return Err(ChainProofError::UnknownField),
            }
        }
        reader.finish()?;
        timestamp.validate()?;
        Ok(timestamp)
    }

    fn encode(self) -> Result<Vec<u8>, ChainProofError> {
        self.validate()?;
        let mut bytes = Vec::new();
        if self.seconds != 0 {
            bytes.push(0x08);
            write_varint(u64::from_le_bytes(self.seconds.to_le_bytes()), &mut bytes);
        }
        if self.nanos != 0 {
            bytes.push(0x10);
            write_varint(u64::from(self.nanos.unsigned_abs()), &mut bytes);
        }
        Ok(bytes)
    }
}

#[derive(Clone, Debug, Default, Eq, PartialEq)]
pub struct PartSetHeader {
    pub total: u32,
    pub hash: Vec<u8>,
}

#[derive(Clone, Debug, Default, Eq, PartialEq)]
pub struct BlockId {
    pub hash: Vec<u8>,
    pub part_set_header: PartSetHeader,
}

impl BlockId {
    fn is_nil(&self) -> bool {
        self.hash.is_empty()
            && self.part_set_header.total == 0
            && self.part_set_header.hash.is_empty()
    }

    fn encode(&self) -> Result<Vec<u8>, ChainProofError> {
        if !matches!(self.hash.len(), 0 | 32)
            || !matches!(self.part_set_header.hash.len(), 0 | 32)
            || self.part_set_header.total > 101
        {
            return Err(ChainProofError::NonCanonical);
        }
        let mut parts = Vec::new();
        if self.part_set_header.total != 0 {
            parts.push(0x08);
            write_varint(u64::from(self.part_set_header.total), &mut parts);
        }
        if !self.part_set_header.hash.is_empty() {
            write_bytes(0x12, &self.part_set_header.hash, &mut parts);
        }
        let mut bytes = Vec::new();
        if !self.hash.is_empty() {
            write_bytes(0x0a, &self.hash, &mut bytes);
        }
        write_bytes(0x12, &parts, &mut bytes);
        Ok(bytes)
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct CommitSig {
    pub flag: BlockIdFlag,
    pub validator_address: Vec<u8>,
    pub timestamp: Timestamp,
    pub signature: Vec<u8>,
}

impl CommitSig {
    pub fn decode(bytes: &[u8]) -> Result<Self, ChainProofError> {
        let mut flag = None;
        let mut validator_address = Vec::new();
        let mut timestamp = Timestamp::ZERO;
        let mut signature = Vec::new();
        let mut reader = ProtoReader::new(bytes);
        let mut previous = 0;
        while let Some(field) = reader.next_field()? {
            if field.number == previous {
                return Err(ChainProofError::NonCanonical);
            }
            previous = field.number;
            match (field.number, field.value) {
                (1, ProtoValue::Varint(value)) => flag = Some(BlockIdFlag::try_from(value)?),
                (2, ProtoValue::Bytes(value)) => validator_address = value.to_vec(),
                (3, ProtoValue::Bytes(value)) => timestamp = Timestamp::decode(value)?,
                (4, ProtoValue::Bytes(value)) => signature = value.to_vec(),
                (1..=4, _) => return Err(ChainProofError::NonCanonical),
                _ => return Err(ChainProofError::UnknownField),
            }
        }
        reader.finish()?;
        let flag = flag.ok_or(ChainProofError::NonCanonical)?;
        match flag {
            BlockIdFlag::Absent => {
                if !validator_address.is_empty()
                    || timestamp != Timestamp::ZERO
                    || !signature.is_empty()
                {
                    return Err(ChainProofError::NonCanonical);
                }
            }
            BlockIdFlag::Commit | BlockIdFlag::Nil => {
                if validator_address.len() != 20 || signature.len() != 64 {
                    return Err(ChainProofError::NonCanonical);
                }
            }
        }
        Ok(Self {
            flag,
            validator_address,
            timestamp,
            signature,
        })
    }
}

pub fn vote_sign_bytes(
    chain_id: &str,
    height: i64,
    round: i32,
    block_id: Option<&BlockId>,
    timestamp: Timestamp,
) -> Result<Vec<u8>, ChainProofError> {
    let mut vote = vec![0x08, SIGNED_MSG_TYPE_PRECOMMIT as u8];
    if height != 0 {
        vote.push(0x11);
        vote.extend_from_slice(&height.to_le_bytes());
    }
    if round != 0 {
        vote.push(0x19);
        vote.extend_from_slice(&i64::from(round).to_le_bytes());
    }
    if let Some(block_id) = block_id.filter(|block_id| !block_id.is_nil()) {
        write_bytes(0x22, &block_id.encode()?, &mut vote);
    }
    write_bytes(0x2a, &timestamp.encode()?, &mut vote);
    if !chain_id.is_empty() {
        write_bytes(0x32, chain_id.as_bytes(), &mut vote);
    }
    let mut bytes = Vec::new();
    write_varint(vote.len() as u64, &mut bytes);
    bytes.extend_from_slice(&vote);
    Ok(bytes)
}

fn write_varint(mut value: u64, bytes: &mut Vec<u8>) {
    while value >= 0x80 {
        bytes.push((value & 0x7f) as u8 | 0x80);
        value >>= 7;
    }
    bytes.push(value as u8);
}

fn write_bytes(key: u8, value: &[u8], bytes: &mut Vec<u8>) {
    bytes.push(key);
    write_varint(value.len() as u64, bytes);
    bytes.extend_from_slice(value);
}
