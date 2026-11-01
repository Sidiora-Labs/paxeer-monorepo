use std::{fs, path::Path};

#[test]
fn real_node_prepare_builds_no_kernel_daemon() {
    let script = fs::read_to_string(
        Path::new(env!("CARGO_MANIFEST_DIR")).join("../../tools/run-real-node-tests.sh"),
    )
    .expect("read the real-node test script");

    for forbidden in [
        "layerxd",
        "layerx-genesis-build",
        "lxp_test_daemon_finality_authority",
        "ed25519",
    ] {
        assert!(
            !script.contains(forbidden),
            "real-node preparation must not depend on {forbidden}"
        );
    }

    for mode in [
        "test) test_target=agent-test ;;",
        "sanitizers) test_target=agent-test-sanitize ;;",
        "prepare) test_target= ;;",
    ] {
        assert!(script.contains(mode), "retain real-node mode {mode}");
    }

    let (setup, prepare) = script
        .split_once("prepare() {\n")
        .expect("declare the prepare function");
    assert!(setup.contains("make \"$test_target\"\n    exit 0"));
    assert!(script.trim_end().ends_with("\nprepare"));
    for required in [
        "tests/lxp_test_program_admission",
        "tests/lxp_test_metered_allowance",
        "tests/bridge/sign-credit",
        "custody-proof-build",
        "make paxeer-build",
        "export PAXD=$repo_root/build/paxd",
        "sha256sum \"$PAXD\"",
    ] {
        assert!(prepare.contains(required), "prepare must retain {required}");
    }

    let python_check = setup
        .lines()
        .find(|line| line.starts_with("python3 -c "))
        .expect("check Python prerequisites");
    for required in [
        "from Crypto.Hash import keccak",
        "from eth_account import Account",
        "from eth_utils import to_checksum_address",
    ] {
        assert!(
            python_check.contains(required),
            "Python prerequisite check must retain {required}"
        );
    }
}
