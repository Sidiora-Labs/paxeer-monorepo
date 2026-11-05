#![no_main]

#[path = "codec.rs"]
mod codec;
mod support;

use libfuzzer_sys::fuzz_target;

pub fn fuzz_target_roundtrip(data: &[u8]) {
    let materialized = support::input_bytes(data);
    codec::roundtrip_case(&materialized);
}

fuzz_target!(|data: &[u8]| fuzz_target_roundtrip(data));
