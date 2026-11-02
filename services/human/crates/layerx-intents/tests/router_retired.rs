use std::ffi::OsString;
use std::fs;
use std::path::Path;

use layerx_intents::{IntentHeader, RejectReason, RejectedIntent};

fn inspect_intent(input: &[u8]) -> Result<IntentHeader, RejectedIntent<'_>> {
    layerx_intents::inspect_intent(input)
}

#[test]
fn evm_log_bytes_are_not_an_intent() {
    let fuzz_target = layerx_intents::fuzz_targets::INTENT_PARSING_AND_COMPILATION;
    assert_eq!(fuzz_target, "intent");

    let fuzz_directory = Path::new(env!("CARGO_MANIFEST_DIR")).join("fuzz/fuzz_targets");
    let mut committed_targets = fs::read_dir(fuzz_directory)
        .unwrap_or_else(|error| panic!("read committed fuzz targets: {error}"))
        .map(|entry| {
            entry
                .unwrap_or_else(|error| panic!("read fuzz target entry: {error}"))
                .file_name()
        })
        .collect::<Vec<_>>();
    committed_targets.sort();
    assert_eq!(
        committed_targets,
        [OsString::from(format!("{fuzz_target}.rs"))]
    );

    let manifest = include_str!("../fuzz/Cargo.toml");
    assert_eq!(manifest.matches("[[bin]]").count(), 1);
    assert!(manifest.contains(&format!("name = \"{fuzz_target}\"")));
    assert!(manifest.contains(&format!("path = \"fuzz_targets/{fuzz_target}.rs\"")));

    let evm_log_body = [];
    let rejected = inspect_intent(&evm_log_body).err().unwrap_or_else(|| {
        panic!("EVM event body with only indexed arguments accepted as an intent")
    });
    assert_eq!(rejected.input(), evm_log_body);
    assert!(
        matches!(rejected.reason(), RejectReason::Malformed(_)),
        "EVM log body must be malformed, got {:?}",
        rejected.reason()
    );
}
