use std::error::Error;
use std::process::Command;

#[test]
fn full_requires_program_inputs_while_human_owner_reaches_human_checks(
) -> Result<(), Box<dyn Error>> {
    for mode in [None, Some("full"), Some("human-owner")] {
        let mut command = Command::new(env!("CARGO_BIN_EXE_layerx-agentd"));
        command
            .env_clear()
            .env("LAYERX_AGENT_PROGRAM_LISTEN", "127.0.0.1:0")
            .env("LAYERX_AGENT_PROGRAM_BEARER_TOKEN", "h".repeat(32))
            .env("LAYERX_AGENT_HUMAN_AUTHORITY_BEARER", "a".repeat(32));
        if let Some(mode) = mode {
            command.env("LAYERX_AGENT_MODE", mode);
        }
        let output = command.output()?;
        assert_eq!(output.status.code(), Some(2));
        let stderr = String::from_utf8(output.stderr)?;
        let required = if mode == Some("human-owner") {
            "LAYERX_AGENT_HUMAN_PEERS is required"
        } else {
            "LAYERX_AGENT_NODE_BEARER_TOKEN is required"
        };
        assert!(
            stderr.contains(required),
            "unexpected boot diagnostic: {stderr}"
        );
    }
    Ok(())
}

#[test]
fn full_requires_journal_and_probe_even_when_transport_inputs_are_present(
) -> Result<(), Box<dyn Error>> {
    use std::os::unix::fs::DirBuilderExt;
    let root = std::env::temp_dir().join(format!("layerx-agentd-mode-{}", std::process::id()));
    std::fs::DirBuilder::new().mode(0o700).create(&root)?;
    let ca = root.join("ca.der");
    let generated = Command::new("openssl")
        .args([
            "req",
            "-x509",
            "-newkey",
            "rsa:2048",
            "-nodes",
            "-subj",
            "/CN=mode-test",
            "-days",
            "1",
            "-outform",
            "DER",
            "-out",
        ])
        .arg(&ca)
        .arg("-keyout")
        .arg(root.join("key.pem"))
        .output()?;
    assert!(generated.status.success());
    for mode in ["full", "human-owner"] {
        let mut command = Command::new(env!("CARGO_BIN_EXE_layerx-agentd"));
        command
            .env_clear()
            .env("LAYERX_AGENT_MODE", mode)
            .env("LAYERX_AGENT_PROGRAM_LISTEN", "127.0.0.1:0")
            .env("LAYERX_AGENT_PROGRAM_BEARER_TOKEN", "h".repeat(32))
            .env("LAYERX_AGENT_HUMAN_AUTHORITY_BEARER", "a".repeat(32))
            .env("LAYERX_AGENT_NODE_BEARER_TOKEN", "n".repeat(32))
            .env("LAYERX_AGENT_AUTHORITY_BEARER_TOKEN", "r".repeat(32))
            .env("LAYERX_AGENT_PROGRAM_MAX_STALENESS_MS", "1000")
            .env("LAYERX_AGENT_NODE_ENDPOINT", "http://127.0.0.1:1")
            .env("LAYERX_AGENT_AUTHORITY_ENDPOINT", "https://localhost:2")
            .env("LAYERX_AGENT_AUTHORITY_CA_DER", &ca)
            .env("LAYERX_AGENT_AUTHORITY_REPLICA_ID", "01".repeat(32));
        if mode == "full" {
            command
                .env(
                    "LAYERX_AGENT_HUMAN_PEERS",
                    "uid=4020;tenant=beta;principal=did:layerx:beta:alice",
                )
                .env(
                    "LAYERX_POLICY_SOURCES",
                    format!("beta:{}", root.join("policy").display()),
                )
                .env(
                    "LAYERX_NATIVE_POLICY_SOURCES",
                    format!("beta:{}", root.join("native-policy").display()),
                )
                .env("LAYERX_EXPECTED_CHAIN_ID", "125")
                .env("LAYERX_AGENT_CHAIN_TRUST_ROOT", root.join("chain-trust"));
        }
        let output = command.output()?;
        assert_eq!(output.status.code(), Some(2));
        let stderr = String::from_utf8(output.stderr)?;
        let required = if mode == "full" {
            "LAYERX_AGENT_DEPLOYMENT_JOURNAL is required"
        } else {
            "LAYERX_AGENT_HUMAN_PEERS is required"
        };
        assert!(
            stderr.contains(required),
            "unexpected boot diagnostic: {stderr}"
        );
        command.env("LAYERX_AGENT_DEPLOYMENT_JOURNAL", root.join("journal"));
        let output = command.output()?;
        assert_eq!(output.status.code(), Some(2));
        let stderr = String::from_utf8(output.stderr)?;
        let required = if mode == "full" {
            "LAYERX_AGENT_PROGRAM_PROBE_ID is required"
        } else {
            "LAYERX_AGENT_HUMAN_PEERS is required"
        };
        assert!(
            stderr.contains(required),
            "unexpected boot diagnostic: {stderr}"
        );
    }
    std::fs::remove_dir_all(root)?;
    Ok(())
}

#[test]
fn human_owner_refuses_ambiguous_peer_configuration_before_connecting() -> Result<(), Box<dyn Error>>
{
    for peers in [
        "4020:did:layerx:beta:alice:beta",
        "uid=4020;tenant=beta;principal=did:layerx:beta:alice;extra=value",
    ] {
        let output = Command::new(env!("CARGO_BIN_EXE_layerx-agentd"))
            .env_clear()
            .env("LAYERX_AGENT_MODE", "human-owner")
            .env("LAYERX_AGENT_PROGRAM_LISTEN", "127.0.0.1:0")
            .env("LAYERX_AGENT_PROGRAM_BEARER_TOKEN", "h".repeat(32))
            .env("LAYERX_AGENT_HUMAN_AUTHORITY_BEARER", "a".repeat(32))
            .env("LAYERX_AGENT_HUMAN_PEERS", peers)
            .output()?;
        assert_eq!(output.status.code(), Some(2));
        let stderr = String::from_utf8(output.stderr)?;
        assert!(
            stderr.contains("LAYERX_AGENT_HUMAN_PEERS entry 0: Fields"),
            "{stderr}"
        );
        assert!(!stderr.contains(peers));
    }
    Ok(())
}

#[test]
fn human_owner_mode_lists_node_rpc_variables() -> Result<(), Box<dyn Error>> {
    let root = std::env::temp_dir().join(format!("layerx-agentd-rpc-mode-{}", std::process::id()));
    std::fs::create_dir(&root)?;
    let unserved_socket = root.join("node.sock");
    assert!(!unserved_socket.exists());
    let mut command = Command::new(env!("CARGO_BIN_EXE_layerx-agentd"));
    command
        .env_clear()
        .env("LAYERX_AGENT_MODE", "human-owner")
        .env("LAYERX_AGENT_HUMAN_SOCKET", &unserved_socket)
        .env("LAYERX_AGENT_PROGRAM_LISTEN", "127.0.0.1:0")
        .env("LAYERX_AGENT_PROGRAM_BEARER_TOKEN", "h".repeat(32))
        .env("LAYERX_AGENT_HUMAN_AUTHORITY_BEARER", "a".repeat(32))
        .env(
            "LAYERX_AGENT_HUMAN_PEERS",
            "uid=4020;tenant=beta;principal=did:layerx:beta:alice",
        );
    for (url, chain_id, expected) in [
        (None, None, "missing LAYERX_AGENT_NODE_RPC_URL"),
        (
            Some("http://127.0.0.1:1"),
            None,
            "missing LAYERX_AGENT_NODE_CHAIN_ID",
        ),
        (
            Some("http://127.0.0.1:1"),
            Some("invalid"),
            "invalid LAYERX_AGENT_NODE_CHAIN_ID",
        ),
    ] {
        command.env_remove("LAYERX_AGENT_NODE_RPC_URL");
        command.env_remove("LAYERX_AGENT_NODE_CHAIN_ID");
        if let Some(url) = url {
            command.env("LAYERX_AGENT_NODE_RPC_URL", url);
        }
        if let Some(chain_id) = chain_id {
            command.env("LAYERX_AGENT_NODE_CHAIN_ID", chain_id);
        }
        let output = command.output()?;
        assert_eq!(output.status.code(), Some(2));
        let stderr = String::from_utf8(output.stderr)?;
        assert!(stderr.contains(expected), "{stderr}");
        assert!(stderr.contains("LAYERX_AGENT_NODE_RPC_URL"), "{stderr}");
        assert!(stderr.contains("LAYERX_AGENT_NODE_CHAIN_ID"), "{stderr}");
        assert!(!stderr.contains("LAYERX_AGENT_HUMAN_SOCKET"), "{stderr}");
        assert!(
            !stderr.contains("LAYERX_AGENT_HUMAN_PROTOCOL_VERSION"),
            "{stderr}"
        );
        assert!(
            !stderr.contains("LAYERX_AGENT_HUMAN_NETWORK_ID"),
            "{stderr}"
        );
    }
    for human_socket_value in [std::path::Path::new(""), unserved_socket.as_path()] {
        let output = command
            .env_remove("LAYERX_AGENT_NODE_RPC_URL")
            .env("LAYERX_AGENT_NODE_CHAIN_ID", "42")
            .env("LAYERX_AGENT_HUMAN_SOCKET", human_socket_value)
            .output()?;
        assert_eq!(output.status.code(), Some(2));
        let stderr = String::from_utf8(output.stderr)?;
        assert!(
            stderr.contains("missing LAYERX_AGENT_NODE_RPC_URL"),
            "{stderr}"
        );
        assert!(stderr.contains("LAYERX_AGENT_NODE_RPC_URL"), "{stderr}");
        assert!(stderr.contains("LAYERX_AGENT_NODE_CHAIN_ID"), "{stderr}");
        assert!(!stderr.contains("LAYERX_AGENT_HUMAN_SOCKET"), "{stderr}");
    }
    std::fs::remove_dir(root)?;
    Ok(())
}
