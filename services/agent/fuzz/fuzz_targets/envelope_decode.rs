#![no_main]

#[path = "codec.rs"]
mod codec;
mod support;

use libfuzzer_sys::fuzz_target;

pub fn fuzz_target_envelope_decode(data: &[u8]) {
    let materialized = support::input_bytes(data);
    codec::envelope_round_trip(&materialized);
}

fuzz_target!(|data: &[u8]| fuzz_target_envelope_decode(data));
