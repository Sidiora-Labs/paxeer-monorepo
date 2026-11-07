#![no_main]

mod intent_harness;

use libfuzzer_sys::fuzz_target;

fuzz_target!(|data: &[u8]| {
    intent_harness::fuzz_intent(data);
});
