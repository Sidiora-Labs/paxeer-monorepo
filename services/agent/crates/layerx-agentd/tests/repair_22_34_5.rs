#[path = "../../layerx-client/tests/support/node_root_22_34_5.rs"]
mod support;

use layerx_agentd::config::{self, ReceiptKeySource, StartupConfig};
use layerx_agentd::read::{
    CheckpointReadError, CommittedReadError, CommittedReads, ServedCheckpoint,
};
use layerx_client::sequencer::SequencerError;
use serde_json::json;
use std::collections::BTreeMap;
use std::fs;
use std::os::unix::fs::PermissionsExt;
use std::path::PathBuf;
use std::sync::atomic::{AtomicU64, Ordering};
use support::{must, Server};

struct ConfigFile(PathBuf);
impl ConfigFile {
    fn new(endpoint: &str) -> Self {
        static NEXT: AtomicU64 = AtomicU64::new(0);
        let path = std::env::temp_dir().join(format!(
            "repair-22-34-5-{}-{}.kv",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::Relaxed)
        ));
        must(fs::write(&path, format!(
            "network_id=125\nnode_endpoint={endpoint}\nexpected_chain_id=125\ntenants=tenant-a\npolicy_sources=tenant-a:/etc/layerx/policy.kvx\nsigner_configurations=tenant-a:/etc/layerx/signer.kvx\nverification_defaults=tenant-a:state-proven\nreceipt_key_source=px_getReceiptKeys\n"
        )));
        must(fs::set_permissions(
            &path,
            fs::Permissions::from_mode(0o600),
        ));
        Self(path)
    }
    fn load(&self, endpoint: Option<&str>) -> StartupConfig {
        let environment = endpoint
            .map(|value| BTreeMap::from([("LAYERX_NODE_ENDPOINT".to_owned(), value.to_owned())]))
            .unwrap_or_default();
        must(config::load(&self.0, &environment))
    }
}
impl Drop for ConfigFile {
    fn drop(&mut self) {
        let _ = fs::remove_file(&self.0);
    }
}

#[test]
fn existing_node_setting_routes_all_served_methods_to_the_root() {
    let node = Server::recorded();
    let gateway = Server::start("/rpc", |_| {
        panic!("daemon node read reached the hosted gateway")
    });
    let file = ConfigFile::new(&gateway.url);
    let startup = file.load(Some(&node.url));
    assert_eq!(startup.node_endpoint, node.url);
    assert_eq!(startup.expected_chain_id, 125);
    assert_eq!(startup.receipt_key_source, ReceiptKeySource::NodeRpc);
    let client = must(startup.connect_px(&[]));
    support::assert_recorded_reads(&client);
    assert_eq!(node.count(), 17); // configured identity check plus the complete recording
    assert_eq!(gateway.count(), 0);

    // File configuration has the same node-root requirement as its override.
    assert!(matches!(
        file.load(None).connect_node(&[]),
        Err(SequencerError::Endpoint)
    ));
    assert_eq!(gateway.count(), 0);
}

#[test]
fn node_identity_and_required_configuration_remain_fail_closed() {
    let node = Server::recorded();
    let file = ConfigFile::new(&node.url);
    let mut startup = file.load(None);
    startup.expected_chain_id = 126;
    assert!(matches!(
        startup.connect_node(&[]),
        Err(SequencerError::Endpoint)
    ));
    assert_eq!(node.count(), 1);
    let environment = BTreeMap::from([("LAYERX_NODE_ENDPOINT".to_owned(), String::new())]);
    assert_eq!(
        config::load(&file.0, &environment).expect_err("blank node override"),
        config::ConfigError {
            setting: "node_endpoint".to_owned(),
            reason: config::RejectionReason::Empty,
        }
    );
    let environment = BTreeMap::from([("LAYERX_PX_ENDPOINT".to_owned(), node.url.clone())]);
    assert_eq!(
        config::load(&file.0, &environment).expect_err("new environment key"),
        config::ConfigError {
            setting: "LAYERX_PX_ENDPOINT".to_owned(),
            reason: config::RejectionReason::Unknown,
        }
    );
    assert_eq!(node.count(), 1);
}

#[test]
fn committed_reader_restarts_and_resumes_signed_checkpoint_and_storage_from_node() {
    let gateway = Server::start("/rpc", |_| {
        panic!("committed reader reached hosted gateway")
    });
    let node = support::storage_server();
    let file = ConfigFile::new(&node.url);
    let storage = support::producer()["storage"].clone();
    let height = storage["height"].as_u64().expect("height");
    let hash = support::hex_bytes::<32>(&storage["hash"]);
    let address = support::hex_bytes::<20>(&storage["address"]);
    let slot = support::hex_bytes::<32>(&storage["slots"][0]);
    let startup = file.load(None);
    let endpoint = must(startup.connect_node(&[]));
    let saved = must(ServedCheckpoint::acquire_at_committed_block(
        &endpoint,
        height,
        Some(hash),
        "hyperpax_125-1",
    ));
    assert_eq!(saved.height, height);
    assert_eq!(saved.block_hash, hash);
    assert_eq!(
        saved.app_hash,
        support::hex_bytes::<32>(&storage["appHash"])
    );
    let pinned = must(layerx_client::rpc::commit::decode_validators(
        &storage["validators"],
    ));
    assert_eq!(saved.validators.len(), pinned.validators().len());
    for (validator, trusted) in saved.validators.iter().zip(pinned.validators()) {
        assert_eq!(validator.public_key, trusted.public_key);
        assert_eq!(validator.voting_power, trusted.voting_power);
    }
    let reader = must(CommittedReads::at_checkpoint(&endpoint, &saved));
    let witness = must(reader.storage(address, slot));
    assert_eq!(
        witness.value,
        support::hex_bytes::<32>(&storage["values"][0])
    );
    drop(reader);
    drop(endpoint);
    assert_eq!(node.count(), 8);
    drop(node);

    // Restart both the transport and reader. Only the authenticated checkpoint
    // is retained by the caller; the old node listener no longer exists.
    let restarted_node = support::storage_server();
    let restarted_config = file.load(Some(&restarted_node.url));
    let restarted_endpoint = must(restarted_config.connect_node(&[]));
    let resumed = must(CommittedReads::resume(&restarted_endpoint, &saved));
    assert_eq!(resumed.checkpoint(), &saved);
    assert_eq!(resumed.checkpoint().header.chain_id, "hyperpax_125-1");
    assert_eq!(must(resumed.storage(address, slot)), witness);
    assert_eq!(restarted_node.count(), 8);
    assert_eq!(gateway.count(), 0);
    let requests = must(restarted_node.requests.lock());
    assert_eq!(
        requests
            .iter()
            .map(|r| r["method"].as_str().expect("method"))
            .collect::<Vec<_>>(),
        [
            "eth_chainId",
            "eth_getBlockByNumber",
            "px_getCommit",
            "px_getValidators",
            "eth_getBlockByNumber",
            "eth_getProof",
            "eth_getBlockByNumber",
            "eth_getBlockByNumber",
        ]
    );
    assert!(requests
        .iter()
        .filter(|r| r["method"] == "eth_getBlockByNumber")
        .all(|r| r["params"][0] == format!("{height:#x}")));
    drop(requests);

    let count = restarted_node.count();
    let mut invalid = saved.clone();
    invalid.commit.votes[0].signature[0] ^= 1;
    assert!(matches!(
        CommittedReads::resume(&restarted_endpoint, &invalid),
        Err(CommittedReadError::Checkpoint(CheckpointReadError::Commit))
    ));
    assert_eq!(
        restarted_node.count(),
        count,
        "invalid saved trust must fail before transport"
    );
    assert_eq!(gateway.count(), 0);
}

#[test]
fn resumed_reader_propagates_node_unavailability_without_reusing_stale_material() {
    let node = support::storage_server();
    let file = ConfigFile::new(&node.url);
    let endpoint = must(file.load(None).connect_node(&[]));
    let storage = support::producer()["storage"].clone();
    let height = storage["height"].as_u64().expect("height");
    let saved = must(ServedCheckpoint::acquire_at_committed_block(
        &endpoint,
        height,
        Some(support::hex_bytes::<32>(&storage["hash"])),
        "hyperpax_125-1",
    ));
    let block = storage["block"].clone();
    let recording = support::recording();
    let refusal = recording["exchanges"]
        .as_array()
        .expect("exchanges")
        .iter()
        .find(|entry| entry["request"]["method"] == "px_getCommit")
        .expect("recorded refusal")["response"]
        .clone();
    let failing_node = Server::start("/", move |request| {
        match request["method"].as_str().expect("method") {
            "eth_getBlockByNumber" => Some(json!({"result": block})),
            "px_getCommit" => Some(refusal.clone()),
            method => panic!("read continued after unavailable commit: {method}"),
        }
    });
    let failed_endpoint = must(layerx_client::sequencer::SequencerEndpoint::new_node(
        &failing_node.url,
        &[],
    ));
    assert!(
        matches!(CommittedReads::resume(&failed_endpoint, &saved), Err(CommittedReadError::Checkpoint(CheckpointReadError::Rpc(SequencerError::Rpc(refusal)))) if refusal.code == -32601)
    );
    assert_eq!(failing_node.count(), 2);
}
