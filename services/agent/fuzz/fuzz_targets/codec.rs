use layerx_crypto::evm_transaction::{self, Eip1559Call, EvmTransactionError};
use layerx_crypto::signer::AgentSignature;
use layerx_wire::receipt::{
    decode, decode_batch_header, decode_checkpoint, decode_merkle_proof, encode,
    encode_batch_header, encode_checkpoint, encode_merkle_proof,
};

pub fn decode_signed(data: &[u8]) -> Result<(Eip1559Call, AgentSignature), EvmTransactionError> {
    evm_transaction::decode_signed(data)
}

pub fn assemble(call: &Eip1559Call, signature: &AgentSignature) -> Vec<u8> {
    evm_transaction::assemble(call, signature)
}

pub fn envelope_round_trip(data: &[u8]) {
    if let Ok((call, signature)) = decode_signed(data) {
        assert_eq!(assemble(&call, &signature), data);
    }
}

pub fn roundtrip_case(data: &[u8]) {
    let Some((&selector, bytes)) = data.split_first() else {
        return;
    };
    match selector % 6 {
        0 | 1 => envelope_round_trip(bytes),
        2 => {
            if let Ok(value) = decode(bytes) {
                assert_eq!(encode(&value).as_deref(), Ok(bytes));
            }
        }
        3 => {
            if let Ok(value) = decode_merkle_proof(bytes) {
                assert_eq!(encode_merkle_proof(&value).as_deref(), Ok(bytes));
            }
        }
        4 => {
            if let Ok(value) = decode_batch_header(bytes) {
                assert_eq!(encode_batch_header(&value).as_deref(), Ok(bytes));
            }
        }
        _ => {
            if let Ok(value) = decode_checkpoint(bytes) {
                assert_eq!(encode_checkpoint(&value).as_deref(), Ok(bytes));
            }
        }
    }
}
