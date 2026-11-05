#[path = "../build.rs"]
mod build_script;

use std::{fs, path::Path, process::Command};

#[test]
fn pay2_lockfile_is_tracked() {
    let manifest = Path::new(env!("CARGO_MANIFEST_DIR"));
    let lockfile = manifest.join("Cargo.lock");
    assert!(lockfile.is_file(), "PAY2's standalone lockfile must exist");
    let bytes = fs::read(&lockfile).expect("read PAY2's standalone lockfile");

    let tracked = Command::new("git")
        .current_dir(manifest)
        .args(["ls-files", "--error-unmatch", "--", "Cargo.lock"])
        .output()
        .expect("invoke Git to check PAY2's tracked lockfile");
    assert_eq!(
        tracked.status.code(),
        Some(0),
        "PAY2's standalone lockfile must be tracked: {}",
        String::from_utf8_lossy(&tracked.stderr)
    );

    let committed = Command::new("git")
        .current_dir(manifest)
        .args(["show", "HEAD:./Cargo.lock"])
        .output()
        .expect("invoke Git to read PAY2's committed lockfile");
    assert_eq!(
        committed.status.code(),
        Some(0),
        "PAY2's standalone lockfile must exist in the checkout's revision: {}",
        String::from_utf8_lossy(&committed.stderr)
    );
    assert_eq!(
        bytes, committed.stdout,
        "PAY2's standalone lockfile must match its tracked bytes"
    );

    let ignored = Command::new("git")
        .current_dir(manifest)
        .args(["check-ignore", "--no-index", "--quiet", "--", "Cargo.lock"])
        .output()
        .expect("invoke Git to check PAY2's lockfile ignore policy");
    assert_eq!(
        ignored.status.code(),
        Some(1),
        "PAY2's standalone lockfile must not be ignored: {}",
        String::from_utf8_lossy(&ignored.stderr)
    );
}

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
    let inputs = build_script::pay2_build_inputs(manifest);
    assert_eq!(
        inputs,
        [
            "Cargo.toml",
            "required.rs",
            "pay2e_test.rs",
            "funded_inputs.rs",
            "funding.rs",
            "sequence.rs",
            "tests/support/registered_scenario.rs",
        ]
        .map(|input| manifest.join(input)),
        "PAY2 must retain exactly its live build inputs"
    );
    build_script::pay2_check_build_inputs(manifest).expect("registered scenario inputs");
    for input in inputs {
        assert!(
            !fs::read_to_string(&input).unwrap().is_empty(),
            "live build input {} must remain readable",
            input.display()
        );
    }
}
