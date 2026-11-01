use std::process::{Command, Output};

const MODES: [&str; 4] = [
    "--production-required",
    "--production-recovery",
    "--production-duplicate",
    "--production-peer-refusal",
];

fn command() -> Command {
    let mut command = Command::new(env!("CARGO_BIN_EXE_agent-boundary-conformance"));
    command.env_clear();
    command
}

fn assert_refusal(output: &Output, name: &str) -> String {
    let stderr = String::from_utf8_lossy(&output.stderr);
    assert!(!output.status.success(), "{name} was accepted: {output:?}");
    assert!(
        stderr.contains(name),
        "refusal did not name {name}: {stderr}"
    );
    stderr.into_owned()
}

#[test]
fn production_mode_requires_the_node_rpc_url() {
    for mode in MODES {
        let output = command()
            .arg(mode)
            .output()
            .expect("run boundary conformance binary");
        assert_refusal(&output, "LAYERX_QUALIFY_NODE_RPC_URL");
    }
    let output = command()
        .output()
        .expect("run default boundary conformance mode");
    assert_refusal(&output, "LAYERX_QUALIFY_NODE_RPC_URL");
}

#[test]
fn retired_lni_socket_variable_is_refused() {
    for name in [
        "LAYERX_QUALIFY_LNI_SOCKET",
        "LAYERX_QUALIFY_SIGNED_ACTIVITY",
        "LAYERX_QUALIFY_LNI_DAEMON_UID",
        "LAYERX_QUALIFY_LNI_CLIENT_GID",
        "LAYERX_QUALIFY_MAX_SIGNED_ACTIVITY",
        "LAYERX_QUALIFY_LNI_DEADLINE_MS",
        "LAYERX_QUALIFY_UNKNOWN_SIGNED_ACTIVITY",
    ] {
        for mode in MODES {
            for value in ["", "retired-value"] {
                let output = command()
                    .arg(mode)
                    .env("LAYERX_QUALIFY_NODE_RPC_URL", "http://127.0.0.1:1")
                    .env(name, value)
                    .output()
                    .expect("run boundary conformance binary with retired variable");
                let stderr = assert_refusal(&output, name);
                assert!(
                    stderr.contains("retired"),
                    "{name} was not identified as retired: {stderr}"
                );
            }
        }
        let output = command()
            .env(name, "retired-value")
            .output()
            .expect("run default boundary conformance mode with retired variable");
        let stderr = assert_refusal(&output, name);
        assert!(
            stderr.contains("retired"),
            "{name} was not identified as retired: {stderr}"
        );
    }
}
