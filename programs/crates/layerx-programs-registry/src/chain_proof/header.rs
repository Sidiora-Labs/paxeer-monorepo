use sha2::{Digest, Sha256};

use super::{
    proto::{ProtoReader, ProtoValue},
    ChainProofError,
};

#[derive(Clone, Copy, Debug, Default, Eq, PartialEq)]
pub struct HeaderVersion {
    pub block: u64,
    pub app: u64,
}

#[derive(Clone, Copy, Debug, Default, Eq, PartialEq)]
pub struct HeaderTime {
    pub seconds: i64,
    pub nanos: i32,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct HeaderBlockId {
    pub hash: Vec<u8>,
    pub part_set_total: u32,
    pub part_set_hash: Vec<u8>,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ChainHeader {
    pub version: HeaderVersion,
    pub chain_id: String,
    pub height: i64,
    pub time: HeaderTime,
    pub last_block_id: HeaderBlockId,
    pub last_commit_hash: Vec<u8>,
    pub data_hash: Vec<u8>,
    pub validators_hash: Vec<u8>,
    pub next_validators_hash: Vec<u8>,
    pub consensus_hash: Vec<u8>,
    pub app_hash: Vec<u8>,
    pub last_results_hash: Vec<u8>,
    pub evidence_hash: Vec<u8>,
    pub proposer_address: Vec<u8>,
}

impl ChainHeader {
    pub fn decode(bytes: &[u8]) -> Result<Self, ChainProofError> {
        let fields = read_fields::<14>(bytes)?;
        let version = read_fields::<2>(bytes_value(fields[0])?)?;
        let time = read_fields::<2>(bytes_value(fields[3])?)?;
        let block_id = read_fields::<2>(bytes_value(fields[4])?)?;
        let part_set = read_fields::<2>(bytes_value(block_id[1])?)?;
        let header = Self {
            version: HeaderVersion {
                block: varint_value(version[0])?,
                app: varint_value(version[1])?,
            },
            chain_id: core::str::from_utf8(bytes_value(fields[1])?)
                .map_err(|_| ChainProofError::NonCanonical)?
                .to_owned(),
            height: i64::from_le_bytes(varint_value(fields[2])?.to_le_bytes()),
            time: HeaderTime {
                seconds: i64::from_le_bytes(varint_value(time[0])?.to_le_bytes()),
                nanos: i32::try_from(varint_value(time[1])?)
                    .map_err(|_| ChainProofError::NonCanonical)?,
            },
            last_block_id: HeaderBlockId {
                hash: bytes_value(block_id[0])?.to_vec(),
                part_set_total: u32::try_from(varint_value(part_set[0])?)
                    .map_err(|_| ChainProofError::NonCanonical)?,
                part_set_hash: bytes_value(part_set[1])?.to_vec(),
            },
            last_commit_hash: bytes_value(fields[5])?.to_vec(),
            data_hash: bytes_value(fields[6])?.to_vec(),
            validators_hash: bytes_value(fields[7])?.to_vec(),
            next_validators_hash: bytes_value(fields[8])?.to_vec(),
            consensus_hash: bytes_value(fields[9])?.to_vec(),
            app_hash: bytes_value(fields[10])?.to_vec(),
            last_results_hash: bytes_value(fields[11])?.to_vec(),
            evidence_hash: bytes_value(fields[12])?.to_vec(),
            proposer_address: bytes_value(fields[13])?.to_vec(),
        };
        header.validate()?;
        Ok(header)
    }

    pub fn hash(&self) -> Result<[u8; 32], ChainProofError> {
        self.validate()?;
        let mut version = Vec::new();
        encode_scalar(1, self.version.block, &mut version);
        encode_scalar(2, self.version.app, &mut version);

        let mut time = Vec::new();
        encode_scalar(
            1,
            u64::from_le_bytes(self.time.seconds.to_le_bytes()),
            &mut time,
        );
        encode_scalar(
            2,
            u64::try_from(self.time.nanos).map_err(|_| ChainProofError::NonCanonical)?,
            &mut time,
        );

        let mut part_set = Vec::new();
        encode_scalar(
            1,
            u64::from(self.last_block_id.part_set_total),
            &mut part_set,
        );
        encode_bytes(2, &self.last_block_id.part_set_hash, &mut part_set);
        let mut block_id = Vec::new();
        encode_bytes(1, &self.last_block_id.hash, &mut block_id);
        encode_bytes(2, &part_set, &mut block_id);

        let mut height = Vec::new();
        encode_scalar(
            1,
            u64::try_from(self.height).map_err(|_| ChainProofError::NonCanonical)?,
            &mut height,
        );
        let fields = [
            version,
            wrap_bytes(self.chain_id.as_bytes()),
            height,
            time,
            block_id,
            wrap_bytes(&self.last_commit_hash),
            wrap_bytes(&self.data_hash),
            wrap_bytes(&self.validators_hash),
            wrap_bytes(&self.next_validators_hash),
            wrap_bytes(&self.consensus_hash),
            wrap_bytes(&self.app_hash),
            wrap_bytes(&self.last_results_hash),
            wrap_bytes(&self.evidence_hash),
            wrap_bytes(&self.proposer_address),
        ];
        Ok(field_merkle_root(&fields))
    }

    fn validate(&self) -> Result<(), ChainProofError> {
        if self.chain_id.is_empty()
            || self.height <= 0
            || !(-62_135_596_800..=253_402_300_799).contains(&self.time.seconds)
            || !(0..1_000_000_000).contains(&self.time.nanos)
            || self.proposer_address.len() != 20
            || [
                &self.last_block_id.hash,
                &self.last_block_id.part_set_hash,
                &self.last_commit_hash,
                &self.data_hash,
                &self.validators_hash,
                &self.next_validators_hash,
                &self.consensus_hash,
                &self.app_hash,
                &self.last_results_hash,
                &self.evidence_hash,
            ]
            .iter()
            .any(|hash| hash.len() != 32)
        {
            return Err(ChainProofError::NonCanonical);
        }
        Ok(())
    }
}

fn read_fields<const N: usize>(
    bytes: &[u8],
) -> Result<[Option<ProtoValue<'_>>; N], ChainProofError> {
    let mut fields = [None; N];
    let mut reader = ProtoReader::new(bytes);
    while let Some(field) = reader.next_field()? {
        let index = usize::try_from(field.number - 1).map_err(|_| ChainProofError::UnknownField)?;
        let slot = fields.get_mut(index).ok_or(ChainProofError::UnknownField)?;
        if slot.is_some() {
            return Err(ChainProofError::NonCanonical);
        }
        *slot = Some(field.value);
    }
    reader.finish()?;
    Ok(fields)
}

fn bytes_value(value: Option<ProtoValue<'_>>) -> Result<&[u8], ChainProofError> {
    match value {
        Some(ProtoValue::Bytes(bytes)) => Ok(bytes),
        _ => Err(ChainProofError::NonCanonical),
    }
}

fn varint_value(value: Option<ProtoValue<'_>>) -> Result<u64, ChainProofError> {
    match value {
        Some(ProtoValue::Varint(value)) => Ok(value),
        None => Ok(0),
        _ => Err(ChainProofError::NonCanonical),
    }
}

fn encode_varint(mut value: u64, output: &mut Vec<u8>) {
    while value >= 0x80 {
        output.push(value.to_le_bytes()[0] & 0x7f | 0x80);
        value >>= 7;
    }
    output.push(value.to_le_bytes()[0]);
}

fn encode_scalar(number: u32, value: u64, output: &mut Vec<u8>) {
    if value != 0 {
        encode_varint(u64::from(number) << 3, output);
        encode_varint(value, output);
    }
}

fn encode_bytes(number: u32, value: &[u8], output: &mut Vec<u8>) {
    encode_varint(u64::from(number) << 3 | 2, output);
    encode_varint(value.len() as u64, output);
    output.extend_from_slice(value);
}

fn wrap_bytes(bytes: &[u8]) -> Vec<u8> {
    let mut output = Vec::new();
    encode_bytes(1, bytes, &mut output);
    output
}

fn field_merkle_root(fields: &[Vec<u8>]) -> [u8; 32] {
    let mut hasher = Sha256::new();
    match fields.len() {
        0 => {}
        1 => {
            hasher.update([0]);
            hasher.update(&fields[0]);
        }
        length => {
            let mut split = 1;
            while split * 2 < length {
                split *= 2;
            }
            hasher.update([1]);
            hasher.update(field_merkle_root(&fields[..split]));
            hasher.update(field_merkle_root(&fields[split..]));
        }
    }
    hasher.finalize().into()
}
