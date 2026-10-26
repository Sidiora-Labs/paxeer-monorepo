//! Header and validator commitments matching the consensus Go implementation.

use crate::{
    merkle,
    proto::{encode_length_delimited, encode_varint_field},
};

#[derive(Clone, Copy, Debug, Default, Eq, PartialEq)]
pub struct ConsensusVersion {
    pub block: u64,
    pub app: u64,
}

#[derive(Clone, Copy, Debug, Default, Eq, PartialEq)]
pub struct Timestamp {
    pub seconds: i64,
    pub nanos: u32,
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

/// The fourteen fields committed by `consensus/types/block.go`'s `Header.Hash`.
/// A fixed-size validator hash prevents the missing-hash case Go refuses.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct Header {
    pub version: ConsensusVersion,
    pub chain_id: String,
    pub height: i64,
    pub time: Timestamp,
    pub last_block_id: BlockId,
    pub last_commit_hash: Vec<u8>,
    pub data_hash: Vec<u8>,
    pub validators_hash: [u8; 32],
    pub next_validators_hash: Vec<u8>,
    pub consensus_hash: Vec<u8>,
    pub app_hash: Vec<u8>,
    pub last_results_hash: Vec<u8>,
    pub evidence_hash: Vec<u8>,
    pub proposer_address: Vec<u8>,
}

impl Header {
    /// Hash the separate protobuf encodings in consensus field order.
    ///
    /// # Panics
    /// Panics if the timestamp is outside protobuf's permitted range.
    #[must_use]
    pub fn hash(&self) -> [u8; 32] {
        assert!((-62_135_596_800..=253_402_300_799).contains(&self.time.seconds));
        assert!(self.time.nanos < 1_000_000_000);

        let mut version = Vec::new();
        nonzero_varint(1, self.version.block, &mut version);
        nonzero_varint(2, self.version.app, &mut version);

        let mut height = Vec::new();
        nonzero_varint(1, self.height as u64, &mut height);

        let mut time = Vec::new();
        nonzero_varint(1, self.time.seconds as u64, &mut time);
        nonzero_varint(2, u64::from(self.time.nanos), &mut time);

        let mut parts = Vec::new();
        nonzero_varint(
            1,
            u64::from(self.last_block_id.part_set_header.total),
            &mut parts,
        );
        nonempty_bytes(2, &self.last_block_id.part_set_header.hash, &mut parts);
        let mut block_id = Vec::new();
        nonempty_bytes(1, &self.last_block_id.hash, &mut block_id);
        // The non-nullable embedded PartSetHeader is emitted even when empty.
        encode_length_delimited(2, &parts, &mut block_id);

        let fields = [
            version,
            wrapped_bytes(self.chain_id.as_bytes()),
            height,
            time,
            block_id,
            wrapped_bytes(&self.last_commit_hash),
            wrapped_bytes(&self.data_hash),
            wrapped_bytes(&self.validators_hash),
            wrapped_bytes(&self.next_validators_hash),
            wrapped_bytes(&self.consensus_hash),
            wrapped_bytes(&self.app_hash),
            wrapped_bytes(&self.last_results_hash),
            wrapped_bytes(&self.evidence_hash),
            wrapped_bytes(&self.proposer_address),
        ];
        let leaves: Vec<&[u8]> = fields.iter().map(Vec::as_slice).collect();
        merkle::root(&leaves)
    }
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct Validator {
    pub pub_key: [u8; 32],
    pub voting_power: i64,
}

/// Hash `SimpleValidator` encodings in the supplied consensus set order.
#[must_use]
pub fn validators_hash(set: &[Validator]) -> [u8; 32] {
    let encodings: Vec<Vec<u8>> = set
        .iter()
        .map(|validator| {
            let mut key = Vec::new();
            encode_length_delimited(1, &validator.pub_key, &mut key);
            let mut encoded = Vec::new();
            encode_length_delimited(1, &key, &mut encoded);
            nonzero_varint(2, validator.voting_power as u64, &mut encoded);
            encoded
        })
        .collect();
    let leaves: Vec<&[u8]> = encodings.iter().map(Vec::as_slice).collect();
    merkle::root(&leaves)
}

fn nonzero_varint(field: u32, value: u64, output: &mut Vec<u8>) {
    if value != 0 {
        encode_varint_field(field, value, output);
    }
}

fn nonempty_bytes(field: u32, value: &[u8], output: &mut Vec<u8>) {
    if !value.is_empty() {
        encode_length_delimited(field, value, output);
    }
}

fn wrapped_bytes(value: &[u8]) -> Vec<u8> {
    let mut encoded = Vec::new();
    nonempty_bytes(1, value, &mut encoded);
    encoded
}
