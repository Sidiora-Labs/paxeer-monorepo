use ed25519_dalek::SigningKey;
use layerx_agentd::boot::{Gate, GateError};
use layerx_agentd::node_rpc::{NodeRpc, NodeRpcConfig};
use layerx_programs::hex;
use std::fmt::Debug;
use std::io::{BufRead, BufReader, Read, Write};
use std::net::TcpListener;
use std::path::{Path, PathBuf};
use std::thread;
use std::time::{Duration, Instant};

fn must<T, E: Debug>(result: Result<T, E>, what: &str) -> T {
    result.unwrap_or_else(|error| panic!("{what}: {error:?}"))
}

pub(crate) fn node_authority(
    config: &layerx_agentd::config::StartupConfig,
    node: &NodeRpc,
) -> layerx_agentd::protocol_evidence::EvidenceAuthority {
    let mut gate = must(Gate::new(config), "protected authority");
    assert!(gate.guard_write(|| ()).is_err());
    let status = must(gate.readiness_gate(node), "current node readiness");
    assert_eq!(status.chain_id, config.expected_chain_id);
    let keys = status.receipt_keys.clone();
    let authority = must(
        must(gate.evidence_authority(), "ready authority")
            .clone()
            .with_receipt_keys(keys),
        "governed receipt authority",
    );
    assert_eq!(gate.guard_write(|| ()), Ok(()));
    authority
}

fn repository_root() -> PathBuf {
    let manifest = PathBuf::from(env!("CARGO_MANIFEST_DIR"));
    manifest.ancestors().nth(4).map_or_else(
        || panic!("repository root above {}", manifest.display()),
        Path::to_path_buf,
    )
}

#[test]
fn repository_root_holds_migrations_and_agent_workspace() {
    let repository = repository_root();
    assert!(repository.join("migrations").is_dir());
    assert!(repository.join("services/agent/Cargo.toml").is_file());
}

struct Fixture {
    sequencer_key: [u8; 32],
    sequencer_id: [u8; 32],
}

fn fixture(
    policy: super::TestAuthorityPolicy<'_>,
    expected_sequencer_id: Option<[u8; 32]>,
) -> Fixture {
    let sequencer_key = SigningKey::from_bytes(&policy.handshake_signing_seed)
        .verifying_key()
        .to_bytes();
    Fixture {
        sequencer_key,
        sequencer_id: expected_sequencer_id.unwrap_or(sequencer_key),
    }
}

pub(super) struct ReadinessNode {
    pub(super) config: NodeRpcConfig,
    listener: TcpListener,
}

impl ReadinessNode {
    pub(super) fn new() -> Self {
        // This peer provides focused decoder coverage; node transcripts below
        // provide the production-interface proof without external configuration.
        let chain_id = 125;
        let listener = must(TcpListener::bind("127.0.0.1:0"), "readiness listener");
        let config = NodeRpcConfig {
            url: format!(
                "http://{}/",
                must(listener.local_addr(), "readiness address")
            ),
            chain_id,
            deadline: Duration::from_secs(2),
        };
        must(
            listener.set_nonblocking(true),
            "readiness nonblocking listener",
        );
        Self { config, listener }
    }
}

fn readiness_gate(
    authority_gate: &mut Gate,
    fixture: &Fixture,
    batch: u64,
    readiness_node: &ReadinessNode,
) -> Result<(), GateError> {
    let config = &readiness_node.config;
    let chain_id = config.chain_id;
    let listener = must(
        readiness_node.listener.try_clone(),
        "readiness listener clone",
    );
    let keys = serde_json::json!({"keys": [{
        "keyId": format!("0x{}", hex::encode(&fixture.sequencer_id)),
        "publicKey": format!("0x{}", hex::encode(&fixture.sequencer_key)),
        "firstHeight": batch,
        "lastHeight": batch,
    }]});
    let server = thread::spawn(move || {
        for (method, result) in [
            ("eth_chainId", serde_json::json!(format!("0x{chain_id:x}"))),
            ("px_getReceiptKeys", keys),
        ] {
            let expires = Instant::now() + Duration::from_secs(5);
            let stream = loop {
                match listener.accept() {
                    Ok((stream, _)) => break stream,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        assert!(Instant::now() < expires, "missing {method} request");
                        thread::sleep(Duration::from_millis(1));
                    }
                    Err(error) => panic!("readiness accept: {error}"),
                }
            };
            must(
                stream.set_read_timeout(Some(Duration::from_secs(2))),
                "readiness read timeout",
            );
            must(
                stream.set_write_timeout(Some(Duration::from_secs(2))),
                "readiness write timeout",
            );
            let mut reader = BufReader::new(stream);
            let mut length = None;
            loop {
                let mut line = String::new();
                assert_ne!(must(reader.read_line(&mut line), "readiness header"), 0);
                if line == "\r\n" {
                    break;
                }
                if let Some(value) = line.to_ascii_lowercase().strip_prefix("content-length:") {
                    length = Some(must(
                        value.trim().parse::<usize>(),
                        "readiness content length",
                    ));
                }
            }
            let length = length.expect("readiness content length required");
            assert!(length <= 64 * 1024);
            let mut body = vec![0; length];
            must(reader.read_exact(&mut body), "readiness request body");
            let request: serde_json::Value =
                must(serde_json::from_slice(&body), "readiness request JSON");
            assert_eq!(request["jsonrpc"], "2.0");
            assert_eq!(request["method"], method);
            assert_eq!(request["params"], serde_json::json!([]));
            let answer =
                serde_json::json!({"jsonrpc": "2.0", "id": request["id"], "result": result})
                    .to_string();
            let mut stream = reader.into_inner();
            must(write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{answer}", answer.len()), "readiness response");
            must(stream.flush(), "readiness flush");
        }
    });
    let result = NodeRpc::connect(config)
        .map_err(GateError::from)
        .and_then(|node| {
            authority_gate.readiness_gate(&node).map(|status| {
                assert_eq!(status.chain_id, chain_id);
                assert!(status.receipt_keys.iter().any(|key| {
                    key.key_id == fixture.sequencer_id
                        && key.public_key == fixture.sequencer_key
                        && key.first_height <= batch
                        && batch <= key.last_height
                }));
            })
        });
    must(server.join(), "readiness server");
    result?;
    assert_eq!(authority_gate.guard_write(|| ()), Ok(()));
    assert!(authority_gate.evidence_authority().is_ok());
    Ok(())
}

fn restart_authority(
    fixture: &Fixture,
    authority_gate: &mut Gate,
    config: &layerx_agentd::config::StartupConfig,
    batch: u64,
    readiness_node: &ReadinessNode,
) -> Result<(), GateError> {
    authority_gate.disconnected();
    assert!(authority_gate.guard_write(|| ()).is_err());
    assert!(authority_gate.evidence_authority().is_err());
    *authority_gate = Gate::new(config)?;
    assert!(authority_gate.guard_write(|| ()).is_err());
    assert!(authority_gate.evidence_authority().is_err());
    readiness_gate(authority_gate, fixture, batch, readiness_node)?;
    assert!(authority_gate.evidence_authority().is_ok());
    Ok(())
}

pub(super) fn authorize(
    authority_gate: &mut Gate,
    policy: super::TestAuthorityPolicy<'_>,
    expected_sequencer_id: Option<[u8; 32]>,
    restart_config: Option<&layerx_agentd::config::StartupConfig>,
    readiness_node: &ReadinessNode,
) -> Result<(), GateError> {
    let fixture = fixture(policy, expected_sequencer_id);
    let result = readiness_gate(
        authority_gate,
        &fixture,
        policy.handshake_batch,
        readiness_node,
    );
    if let Some(config) = restart_config {
        result.clone()?;
        restart_authority(
            &fixture,
            authority_gate,
            config,
            policy.handshake_batch,
            readiness_node,
        )?;
    }
    result
}
