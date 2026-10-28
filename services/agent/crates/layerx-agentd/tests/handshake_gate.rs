use std::collections::{BTreeMap, BTreeSet};
use std::fs;
use std::io::{BufRead, BufReader, Read, Write};
use std::net::TcpListener;
use std::os::unix::fs::{symlink, PermissionsExt};
use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicU64, Ordering};
use std::thread::{self, JoinHandle};
use std::time::{Duration, Instant};

use layerx_agentd::boot::{Gate, GateError, Status, WriteGateError};
use layerx_agentd::compat::{
    self, CompatibilityError, CONTRACT_VERSION, DAEMON_VERSION, SDK_VERSION,
};
use layerx_agentd::config::StartupConfig;
use layerx_agentd::node_rpc::{NodeRpc, NodeRpcConfig, NodeRpcError};
use layerx_agentd::store::TenantId;
use layerx_programs::hex;
use layerx_types::verify::VerificationLevel;
use serde_json::{json, Value};

static NEXT_SOCKET: AtomicU64 = AtomicU64::new(1);

fn socket_path(label: &str) -> PathBuf {
    let sequence = NEXT_SOCKET.fetch_add(1, Ordering::Relaxed);
    std::env::temp_dir().join(format!(
        "layerx-readiness-gate-{label}-{}-{sequence}",
        std::process::id()
    ))
}

pub fn must<T, E: std::fmt::Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("{error:?}"))
}

fn serve(exchanges: Vec<(&'static str, Value)>) -> (NodeRpcConfig, JoinHandle<()>) {
    let listener = must(TcpListener::bind("127.0.0.1:0"));
    let config = NodeRpcConfig {
        url: format!("http://{}/", must(listener.local_addr())),
        chain_id: 125,
        deadline: Duration::from_secs(2),
    };
    must(listener.set_nonblocking(true));
    let server = thread::spawn(move || {
        for (method, result) in exchanges {
            let expires = Instant::now() + Duration::from_secs(5);
            let stream = loop {
                match listener.accept() {
                    Ok((stream, _)) => break stream,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        assert!(Instant::now() < expires, "missing {method} request");
                        thread::sleep(Duration::from_millis(1));
                    }
                    Err(error) => panic!("accept: {error}"),
                }
            };
            must(stream.set_read_timeout(Some(Duration::from_secs(2))));
            must(stream.set_write_timeout(Some(Duration::from_secs(2))));
            let mut reader = BufReader::new(stream);
            let mut length = None;
            loop {
                let mut line = String::new();
                assert_ne!(must(reader.read_line(&mut line)), 0);
                if line == "\r\n" {
                    break;
                }
                if let Some(value) = line.to_ascii_lowercase().strip_prefix("content-length:") {
                    length = Some(must(value.trim().parse::<usize>()));
                }
            }
            let length = length.unwrap_or_else(|| panic!("content length required"));
            assert!(length <= 64 * 1024);
            let mut body = vec![0; length];
            must(reader.read_exact(&mut body));
            let request: Value = must(serde_json::from_slice(&body));
            assert_eq!(request["jsonrpc"], "2.0");
            assert_eq!(request["method"], method);
            assert_eq!(request["params"], json!([]));
            let answer =
                json!({"jsonrpc": "2.0", "id": request["id"], "result": result}).to_string();
            let mut stream = reader.into_inner();
            must(write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{answer}", answer.len()));
            must(stream.flush());
        }
    });
    (config, server)
}

pub fn empty_node_config(
    chain_checks: usize,
    receipt_checks: usize,
) -> (NodeRpcConfig, JoinHandle<()>) {
    let mut exchanges = vec![("eth_chainId", json!("0x7d")); chain_checks];
    exchanges.extend(vec![("px_getReceiptKeys", json!([])); receipt_checks]);
    serve(exchanges)
}

fn receipt_public_key() -> [u8; 32] {
    ed25519_dalek::SigningKey::from_bytes(&[8; 32])
        .verifying_key()
        .to_bytes()
}

fn ready_node(receipt_checks: usize) -> (NodeRpc, JoinHandle<()>) {
    let keys = json!([{
        "keyId": format!("0x{}", hex::encode(&receipt_public_key())),
        "publicKey": format!("0x{}", hex::encode(&receipt_public_key())),
        "firstHeight": "0x1",
        "lastHeight": "0x64",
    }]);
    let mut exchanges = vec![("eth_chainId", json!("0x7d"))];
    exchanges.extend(vec![("px_getReceiptKeys", keys); receipt_checks]);
    let (config, server) = serve(exchanges);
    (must(NodeRpc::connect(&config)), server)
}

fn tenant() -> TenantId {
    TenantId::new("tenant-a").unwrap_or_else(|error| panic!("tenant: {error}"))
}

pub fn config() -> StartupConfig {
    let authority_source = socket_path("authority").with_extension("csv");
    let key = receipt_public_key();
    fs::write(
        &authority_source,
        format!(
            "layerx-sequencer-authority-v1\n{},{},0,1,100,active\n",
            hex::encode(&key),
            hex::encode(&key)
        ),
    )
    .unwrap_or_else(|error| panic!("authority source: {error}"));
    fs::set_permissions(&authority_source, fs::Permissions::from_mode(0o600))
        .unwrap_or_else(|error| panic!("authority source permissions: {error}"));
    config_with_authority(authority_source)
}

fn config_with_authority(authority_source: PathBuf) -> StartupConfig {
    let tenant = tenant();
    StartupConfig {
        network_id: 42,
        node_endpoint: PathBuf::from("/run/layerx/layerxd.sock"),
        expected_protocol_version: layerx_wire::limits::PROTOCOL_VERSION,
        tenants: BTreeSet::from([tenant.clone()]),
        policy_sources: BTreeMap::from([(
            tenant.clone(),
            PathBuf::from("/etc/layerx/policy-a.kvx"),
        )]),
        signer_configurations: BTreeMap::from([(
            tenant.clone(),
            PathBuf::from("/etc/layerx/signer-a.kvx"),
        )]),
        verification_defaults: BTreeMap::from([(tenant, VerificationLevel::STATE_PROVEN)]),
        sequencer_authority_source: authority_source,
    }
}

fn protected_authority(path: &Path) {
    let key = [8; 32];
    fs::write(
        path,
        format!(
            "layerx-sequencer-authority-v1\n{},{},0,0,100,active\n",
            hex::encode(&key),
            hex::encode(&key)
        ),
    )
    .unwrap_or_else(|error| panic!("authority source: {error}"));
    fs::set_permissions(path, fs::Permissions::from_mode(0o600))
        .unwrap_or_else(|error| panic!("authority source permissions: {error}"));
}

#[test]
fn authority_source_rejects_leaf_and_ancestor_symlinks() {
    let leaf_target = socket_path("authority-leaf-target").with_extension("csv");
    let leaf_link = socket_path("authority-leaf-link").with_extension("csv");
    protected_authority(&leaf_target);
    symlink(&leaf_target, &leaf_link).unwrap_or_else(|error| panic!("leaf symlink: {error}"));
    assert!(matches!(
        Gate::new(&config_with_authority(leaf_link.clone())),
        Err(GateError::Evidence(
            layerx_agentd::protocol_evidence::VerifierPolicyError::AuthoritySourceUnprotected
        ))
    ));

    let root = socket_path("authority-ancestor-root").with_extension("directory");
    let real = root.join("real");
    let linked = root.join("linked");
    fs::create_dir(&root).unwrap_or_else(|error| panic!("authority root: {error}"));
    fs::create_dir(&real).unwrap_or_else(|error| panic!("authority real directory: {error}"));
    symlink(&real, &linked).unwrap_or_else(|error| panic!("ancestor symlink: {error}"));
    let real_source = real.join("authorities.csv");
    protected_authority(&real_source);
    assert!(matches!(
        Gate::new(&config_with_authority(linked.join("authorities.csv"))),
        Err(GateError::Evidence(
            layerx_agentd::protocol_evidence::VerifierPolicyError::AuthoritySourceUnprotected
        ))
    ));

    let _ = fs::remove_file(leaf_link);
    let _ = fs::remove_file(leaf_target);
    let _ = fs::remove_file(linked);
    let _ = fs::remove_file(real_source);
    let _ = fs::remove_dir(real);
    let _ = fs::remove_dir(root);
}

#[test]
fn authority_source_rejects_permissive_and_non_regular_inputs() {
    let permissive = socket_path("authority-permissive").with_extension("csv");
    protected_authority(&permissive);
    fs::set_permissions(&permissive, fs::Permissions::from_mode(0o640))
        .unwrap_or_else(|error| panic!("permissive authority permissions: {error}"));
    assert!(matches!(
        Gate::new(&config_with_authority(permissive.clone())),
        Err(GateError::Evidence(
            layerx_agentd::protocol_evidence::VerifierPolicyError::AuthoritySourceUnprotected
        ))
    ));

    let directory = socket_path("authority-directory").with_extension("directory");
    fs::create_dir(&directory).unwrap_or_else(|error| panic!("authority directory: {error}"));
    assert!(matches!(
        Gate::new(&config_with_authority(directory.clone())),
        Err(GateError::Evidence(
            layerx_agentd::protocol_evidence::VerifierPolicyError::AuthoritySourceUnprotected
        ))
    ));

    let _ = fs::remove_file(permissive);
    let _ = fs::remove_dir(directory);
}

#[test]
fn startup_is_not_ready_until_real_node_reports_receipt_keys() {
    let mut gate = must(Gate::new(&config()));
    let operation_ran = std::cell::Cell::new(false);
    assert_eq!(gate.status(), &Status::NotReady);
    assert_eq!(
        gate.guard_write(|| operation_ran.set(true)),
        Err(WriteGateError::NotReady)
    );
    assert!(!operation_ran.get());

    let (node, server) = ready_node(2);
    let keys = must(node.receipt_keys());
    assert!(!keys.is_empty());
    let status = must(gate.readiness_gate(&node));
    assert_eq!(status.chain_id, node.chain_id());
    assert_eq!(status.receipt_keys, keys);
    assert_eq!(status.generation, 1);
    assert_eq!(gate.guard_write(|| 7), Ok(7));
    must(server.join());
}

#[test]
fn empty_receipt_keys_do_not_open_the_write_gate() {
    let mut gate = must(Gate::new(&config()));
    let (node_config, server) = empty_node_config(1, 2);
    let node = must(NodeRpc::connect(&node_config));
    assert!(must(node.receipt_keys()).is_empty());
    assert_eq!(gate.readiness_gate(&node), Err(GateError::NoReceiptKeys));
    assert_eq!(gate.status(), &Status::NotReady);
    assert_eq!(gate.guard_write(|| 1), Err(WriteGateError::NotReady));
    assert!(matches!(
        gate.evidence_authority(),
        Err(WriteGateError::NotReady)
    ));
    must(server.join());
}

#[test]
fn reconnect_rechecks_receipt_keys_and_invalidates_stale_readiness() {
    let mut gate = must(Gate::new(&config()));
    let (first, first_server) = ready_node(3);
    assert_eq!(must(gate.readiness_gate(&first)).generation, 1);
    gate.disconnected();
    assert_eq!(gate.status(), &Status::NotReady);
    assert_eq!(gate.guard_write(|| 1), Err(WriteGateError::NotReady));
    assert_eq!(must(gate.readiness_gate(&first)).generation, 2);

    let (second_config, second_server) = empty_node_config(1, 2);
    let second = must(NodeRpc::connect(&second_config));
    assert!(must(second.receipt_keys()).is_empty());
    assert_eq!(gate.readiness_gate(&second), Err(GateError::NoReceiptKeys));
    assert_eq!(gate.status(), &Status::NotReady);
    assert_eq!(gate.guard_write(|| 1), Err(WriteGateError::NotReady));
    assert!(matches!(
        gate.evidence_authority(),
        Err(WriteGateError::NotReady)
    ));
    assert_eq!(must(gate.readiness_gate(&first)).generation, 3);
    must(first_server.join());
    must(second_server.join());
}

#[test]
fn chain_contract_and_client_major_mismatches_refuse_operation() {
    let mut gate = must(Gate::new(&config()));
    let (mut config, server) = empty_node_config(2, 0);
    let observed = must(NodeRpc::connect(&config)).chain_id();
    config.chain_id = observed
        .checked_add(1)
        .unwrap_or_else(|| panic!("chain id overflow"));
    assert_eq!(
        NodeRpc::connect(&config).err(),
        Some(NodeRpcError::ChainIdMismatch {
            expected: config.chain_id,
            observed,
        })
    );
    must(server.join());
    gate.disconnected();
    assert_eq!(gate.guard_write(|| 1), Err(WriteGateError::NotReady));
    let major = compat::matrix()[0].node_interface_major;
    assert_eq!(
        compat::verify(
            DAEMON_VERSION,
            &format!("paxd/v{major}.0.0"),
            1,
            SDK_VERSION
        ),
        Err(CompatibilityError::UnsupportedContract)
    );
    assert_eq!(
        compat::verify(
            DAEMON_VERSION,
            &format!("paxd/v{}.0.0", major + 1),
            CONTRACT_VERSION,
            SDK_VERSION
        ),
        Err(CompatibilityError::UnsupportedNodeInterface)
    );
}
