use super::{merkle::SimpleProof, ChainProofError, VerifiedChainHead};

#[derive(Clone, Debug, Default, Eq, PartialEq)]
pub struct TxResult {
    pub code: u32,
    pub data: Vec<u8>,
    pub gas_wanted: i64,
    pub gas_used: i64,
}

pub fn verify_tx_result(
    next: &VerifiedChainHead,
    result: &TxResult,
    proof: &SimpleProof,
) -> Result<(), ChainProofError> {
    let mut leaf = Vec::new();
    if result.code != 0 {
        leaf.push(0x08);
        encode_varint(u64::from(result.code), &mut leaf);
    }
    if !result.data.is_empty() {
        leaf.push(0x12);
        encode_varint(result.data.len() as u64, &mut leaf);
        leaf.extend_from_slice(&result.data);
    }
    if result.gas_wanted != 0 {
        leaf.push(0x28);
        encode_varint(
            u64::from_le_bytes(result.gas_wanted.to_le_bytes()),
            &mut leaf,
        );
    }
    if result.gas_used != 0 {
        leaf.push(0x30);
        encode_varint(u64::from_le_bytes(result.gas_used.to_le_bytes()), &mut leaf);
    }
    proof.verify(&next.last_results_hash(), &leaf)
}

fn encode_varint(mut value: u64, output: &mut Vec<u8>) {
    while value >= 0x80 {
        output.push(value.to_le_bytes()[0] & 0x7f | 0x80);
        value >>= 7;
    }
    output.push(value.to_le_bytes()[0]);
}
