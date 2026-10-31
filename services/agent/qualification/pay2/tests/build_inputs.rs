#[path = "../build.rs"]
mod build_script;

use std::{fs, path::Path};

#[test]
fn pay2_build_reads_no_deleted_sources() {
    let source = include_str!("../build.rs");
    for deleted in ["lifecycle.rs", "tests/boundary.rs"] {
        assert!(
            !source.contains(deleted),
            "PAY2 build script still names deleted source {deleted}"
        );
    }
    assert!(
        !source.contains("LAYERX_TEST_CORE_BIN"),
        "PAY2 build must not require the retired core fixture binary"
    );

    let manifest = Path::new(env!("CARGO_MANIFEST_DIR"));
    let local = manifest
        .join("../../../../services/platform/hosted/gateway/tests/local")
        .canonicalize()
        .unwrap();
    let inputs = build_script::pay2_build_inputs(manifest);
    assert_eq!(
        inputs,
        [
            local.join("required.rs"),
            local.join("verifier.rs"),
            manifest.join("pay2e_test.rs"),
        ],
        "PAY2 must retain exactly its live build inputs"
    );
    for input in inputs {
        assert!(
            !fs::read_to_string(&input).unwrap().is_empty(),
            "live build input {} must remain readable",
            input.display()
        );
    }
}
