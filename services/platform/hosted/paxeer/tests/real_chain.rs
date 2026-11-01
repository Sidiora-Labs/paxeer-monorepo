use std::process::Command;

#[test]
fn real_paxd_boundary() {
    let status = Command::new("python3")
        .arg(concat!(env!("CARGO_MANIFEST_DIR"), "/tests/real_chain.py"))
        .arg(env!("CARGO_BIN_EXE_layerx-paxeer-boundary"))
        .arg(std::env::current_exe().unwrap_or_else(|error| panic!("test executable: {error}")))
        .status()
        .unwrap_or_else(|error| panic!("launch real Paxeer qualification: {error}"));
    assert!(
        status.success(),
        "real Paxeer qualification failed: {status}"
    );
}

#[test]
fn settlement_domains() {
    let status = Command::new("python3")
        .arg(concat!(
            env!("CARGO_MANIFEST_DIR"),
            "/tests/settlement_domains.py"
        ))
        .status()
        .unwrap_or_else(|error| panic!("launch domain qualification: {error}"));
    assert!(status.success(), "settlement domain qualification failed");
}
