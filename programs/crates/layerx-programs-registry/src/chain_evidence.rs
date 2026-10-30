use crate::protocol_evidence::PROGRAMS_MODULE_ID;

pub mod program_state {}
pub mod deployment {}
pub mod protocol_head {}
pub mod lifecycle {}
pub mod wind_down {}
pub mod freshness {}
pub mod trust_root {}

pub fn programs_state_key(kernel_key: &[u8]) -> Vec<u8> {
    let mut key = Vec::with_capacity(3 + kernel_key.len());
    key.push(0x01);
    key.extend_from_slice(&PROGRAMS_MODULE_ID.to_be_bytes());
    key.extend_from_slice(kernel_key);
    key
}
