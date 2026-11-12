use std::process::Command;

#[test]
fn real_paxd_boundary() {
    let status = Command::new("python3")
        .arg(concat!(env!("CARGO_MANIFEST_DIR"), "/tests/real_chain.py"))
        .arg(env!("CARGO_BIN_EXE_layerx-paxeer-boundary"))
        .arg(env!("CARGO_BIN_EXE_paxeer-native-tool"))
        .status()
        .unwrap_or_else(|error| panic!("launch real Paxeer qualification: {error}"));
    assert!(
        status.success(),
        "real Paxeer qualification failed: {status}"
    );
}

#[test]
fn qualification_refuses_reentry() {
    for active in ["1", ""] {
        let output = Command::new("python3")
            .arg("-c")
            .arg(
                r"
import os
import runpy
import sys

process_starts = []

def audit(event, args):
    if event in ('subprocess.Popen', 'os.system', 'os.posix_spawn', 'os.exec',
                 'os.fork', 'os.forkpty'):
        process_starts.append(event)

sys.addaudithook(audit)
sys.argv = sys.argv[1:]
sys.path.insert(0, os.path.dirname(sys.argv[0]))
try:
    runpy.run_path(sys.argv[0], run_name='__main__')
finally:
    print(f'qualification child process starts: {len(process_starts)}')
",
            )
            .arg(concat!(env!("CARGO_MANIFEST_DIR"), "/tests/real_chain.py"))
            .arg(env!("CARGO_BIN_EXE_layerx-paxeer-boundary"))
            .env("LAYERX_PAXEER_QUALIFICATION_ACTIVE", active)
            .env(
                "PAXD",
                concat!(env!("CARGO_MANIFEST_DIR"), "/tests/nonexistent-paxd"),
            )
            .output()
            .unwrap_or_else(|error| panic!("launch re-entry qualification: {error}"));
        assert_eq!(output.status.code(), Some(2), "{output:?}");
        assert_eq!(
            String::from_utf8(output.stderr)
                .unwrap_or_else(|error| panic!("qualification stderr is UTF-8: {error:?}")),
            "refusing qualification re-entry: LAYERX_PAXEER_QUALIFICATION_ACTIVE is already set\n"
        );
        assert_eq!(
            String::from_utf8(output.stdout)
                .unwrap_or_else(|error| panic!("qualification stdout is UTF-8: {error:?}")),
            "qualification child process starts: 0\n",
            "qualification must refuse before launching any chain or boundary process"
        );
    }
}

#[test]
fn settlement_domains() {
    let status = Command::new("python3")
        .arg(concat!(
            env!("CARGO_MANIFEST_DIR"),
            "/tests/migrations/v1/settlement_domains.py"
        ))
        .status()
        .unwrap_or_else(|error| panic!("launch version-one migration qualification: {error}"));
    assert!(
        status.success(),
        "version-one settlement migration qualification failed"
    );
}

#[test]
fn native_settlement_evidence() {
    let status = Command::new("python3")
        .arg(concat!(
            env!("CARGO_MANIFEST_DIR"),
            "/tests/settlement_domains.py"
        ))
        .arg(env!("CARGO_BIN_EXE_paxeer-native-tool"))
        .status()
        .unwrap_or_else(|error| panic!("launch native evidence qualification: {error}"));
    assert!(
        status.success(),
        "native settlement evidence qualification failed"
    );
}
