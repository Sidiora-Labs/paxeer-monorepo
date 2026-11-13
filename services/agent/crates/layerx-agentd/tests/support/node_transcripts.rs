//! Loopback replay of the committed Go node producer, with no external node process.
use std::collections::{BTreeMap, BTreeSet};
use std::os::unix::fs::PermissionsExt;
use std::path::Path;
use std::thread::JoinHandle;
use std::time::Duration;

use layerx_agentd::config::{ReceiptKeySource, StartupConfig};
use layerx_agentd::node_rpc::NodeRpcConfig;
use layerx_client::sequencer::{decode_receipt_keys, ReceiptKeyEntry};
use layerx_types::verify::VerificationLevel;
use serde_json::{json, Value};

use super::{must, rpc_hex, serve_with_url, tenant, Exchange, SequencerTraffic};

pub fn corpus() -> Value {
    let value: Value = must(serde_json::from_str(include_str!(
        "../../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
    )));
    assert_eq!(
        value["provenance"]["generator"],
        "rpc/sequencer_receipt_vectors_test.go"
    );
    value
}

pub fn case(success: bool) -> Value {
    let corpus = corpus();
    let id = if success {
        "V1-layerxledger-pay-exact"
    } else {
        "V2-layerxledger-revert"
    };
    assert_some(
        assert_some(corpus["cases"].as_array(), "node cases")
            .iter()
            .find(|case| case["id"] == id),
        "ledger transcript",
    )
    .clone()
}

pub fn unhex(value: &Value) -> Vec<u8> {
    must(hex::decode(assert_some(
        assert_some(value.as_str(), "fixture hex").strip_prefix("0x"),
        "hex prefix",
    )))
}

pub fn key_reply(case: &Value) -> Value {
    // Preserve the producer's public key and numeric validity interval verbatim.
    json!({"keys": [case["pinnedKey"].clone()]})
}

pub fn key(case: &Value) -> ReceiptKeyEntry {
    let keys = must(decode_receipt_keys(&key_reply(case)));
    assert_eq!(keys.len(), 1);
    keys[0]
}

pub fn replay(exchanges: Vec<Exchange>) -> (NodeRpcConfig, JoinHandle<SequencerTraffic>) {
    let (url, server) = serve_with_url(exchanges);
    (
        NodeRpcConfig {
            url,
            chain_id: assert_some(corpus()["chainId"].as_u64(), "node chain"),
            deadline: Duration::from_secs(2),
        },
        server,
    )
}

pub fn startup(root: &Path, node: &NodeRpcConfig, key: ReceiptKeyEntry) -> StartupConfig {
    let path = root.join("receipt-authority.csv");
    let source = format!(
        "layerx-sequencer-authority-v1\n{},{},1,{},{},active\n",
        hex::encode(key.key_id),
        hex::encode(key.public_key),
        key.first_height,
        key.last_height
    );
    must(std::fs::write(&path, source));
    must(std::fs::set_permissions(
        &path,
        std::fs::Permissions::from_mode(0o600),
    ));
    let tenant = tenant();
    StartupConfig {
        network_id: assert_ok(u32::try_from(node.chain_id), "network id"),
        node_endpoint: node.url.clone(),
        expected_chain_id: node.chain_id,
        tenants: BTreeSet::from([tenant.clone()]),
        policy_sources: BTreeMap::from([(tenant.clone(), root.join("policy.kvx"))]),
        signer_configurations: BTreeMap::from([(tenant.clone(), root.join("signer.kvx"))]),
        verification_defaults: BTreeMap::from([(tenant, VerificationLevel::STATE_PROVEN)]),
        receipt_key_source: ReceiptKeySource::PinnedFile(path),
    }
}

pub fn readiness_exchanges(case: &Value) -> Vec<Exchange> {
    vec![
        (
            "eth_chainId",
            json!([]),
            Some(json!(format!(
                "{:#x}",
                assert_some(corpus()["chainId"].as_u64(), "expected Some value")
            ))),
        ),
        ("px_getReceiptKeys", json!([]), Some(key_reply(case))),
    ]
}

pub fn receipt_exchange(case: &Value) -> Exchange {
    let raw = unhex(&case["rawTransaction"]);
    let hash = layerx_crypto::evm_transaction::transaction_hash(&raw);
    assert_eq!(case["receiptJson"]["txHash"], rpc_hex(&hash));
    assert_eq!(case["receiptJson"]["receipt"], case["signedReceipt"]);
    (
        "pax_sequencerReceipt",
        json!([rpc_hex(&hash)]),
        Some(case["receiptJson"].clone()),
    )
}

#[track_caller]
fn assert_ok<T, E: std::fmt::Debug>(result: Result<T, E>, message: &str) -> T {
    match result {
        Ok(value) => value,
        Err(error) => panic!("{message}: {error:?}"),
    }
}

#[track_caller]
fn assert_some<T>(value: Option<T>, message: &str) -> T {
    match value {
        Some(value) => value,
        None => panic!("{message}"),
    }
}
