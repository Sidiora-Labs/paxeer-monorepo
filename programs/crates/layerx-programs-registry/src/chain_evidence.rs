use crate::protocol_evidence::PROGRAMS_MODULE_ID;

pub mod deployment;
pub mod freshness;
pub mod lifecycle;
pub mod program_state;
pub mod protocol_head;
pub mod wind_down;
pub mod trust_root {}

pub(crate) mod verify_bundle;

pub fn programs_state_key(kernel_key: &[u8]) -> Vec<u8> {
    let mut key = Vec::with_capacity(3 + kernel_key.len());
    key.push(0x01);
    key.extend_from_slice(&PROGRAMS_MODULE_ID.to_be_bytes());
    key.extend_from_slice(kernel_key);
    key
}
