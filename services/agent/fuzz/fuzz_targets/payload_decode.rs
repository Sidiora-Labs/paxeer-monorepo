#![no_main]

mod payload_codec;
mod support;

use libfuzzer_sys::fuzz_target;

pub fn fuzz_target_payload_decode(data: &[u8]) {
    let _ = payload_codec::payload_decode_case(data);
}

fuzz_target!(|data: &[u8]| fuzz_target_payload_decode(data));
