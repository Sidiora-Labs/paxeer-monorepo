#[path = "support/repair_22_33_1335.rs"]
pub mod support;

use serde_json::json;
use std::sync::atomic::Ordering;
use std::sync::{Arc, Mutex};
use support::Fixture;

#[test]
fn readiness_contract_conflict() {
    let fixture = Fixture::start();
    let trace = Arc::new(Mutex::new(Vec::new()));
    let first = fixture.node(false, Arc::clone(&trace));
    let second = fixture.node(false, Arc::clone(&trace));
    let consensus = fixture.consensus();
    let gateway = fixture.gateway(&[&first, &second], Some(&consensus), false);
    let product = gateway.readiness("/readyz");
    let core = gateway.readiness("/readyz/core");
    drop(gateway);
    let gateway = fixture.gateway(&[&first, &second], Some(&consensus), true);
    assert_eq!(gateway.readiness("/readyz"), product);
    assert_eq!(gateway.readiness("/readyz/core"), core);
}

#[test]
fn chain_read_router_path() {
    let fixture = Fixture::start();
    let consensus = fixture.consensus();
    let gateway = fixture.gateway(&[], Some(&consensus), false);
    gateway.retired();
    let result = gateway.consumer().commit_material(10).unwrap_or_else(|error| panic!("{}: {error:?}", "root commit read"));
    assert_eq!(result["signedHeader"], support::committed()["commit"]["signed_header"]);
    assert_eq!(gateway.rpc("px_getCommit", json!(["0"]))["error"]["code"], -32602);
}

#[test]
fn degraded_router_path() {
    let fixture = Fixture::start();
    let gateway = fixture.gateway(&[], None, false);
    gateway.retired();
    let chain = gateway.rpc("eth_chainId", json!([]));
    assert_eq!(chain["error"]["data"]["code"], "paxeer_rpc_not_configured");
    assert!(chain.get("result").is_none());
    let consensus = gateway.rpc("px_getCommit", json!(["10"]));
    assert_eq!(consensus["error"]["data"]["code"], "consensus_unavailable");
    assert!(consensus.get("result").is_none());
    let funds = gateway.rpc("px_requestFunds", json!([]));
    assert_eq!(funds["error"]["data"]["code"], "kernel_unavailable");
    assert_eq!(funds["error"]["data"]["backend"], "identity");
}

#[test]
fn one_source_router_path() {
    let fixture = Fixture::start();
    let trace = Arc::new(Mutex::new(Vec::new()));
    let first = fixture.node(false, Arc::clone(&trace));
    let second = fixture.node(false, Arc::clone(&trace));
    let gateway = fixture.gateway(&[&first, &second], None, false);
    gateway.retired();
    assert_eq!(gateway.consumer().block_number(), Ok(10));
    assert_eq!(gateway.consumer().chain_id(), Ok(125));
    assert_eq!(trace.lock().unwrap_or_else(|error| panic!("{}: {error:?}", "node trace")).len(), 4);
    let refused = gateway.rpc("lx_getAccount", json!([]));
    assert_eq!(refused["error"]["code"], -32601);
}

#[test]
fn failover_upstream_path_shape() {
    let fixture = Fixture::start();
    let trace = Arc::new(Mutex::new(Vec::new()));
    let first = fixture.node(false, Arc::clone(&trace));
    let second = fixture.node(false, Arc::new(Mutex::new(Vec::new())));
    let gateway = fixture.gateway(&[&first, &second], None, false);
    let hash = support::committed()["receipt"]["txHash"].clone();
    let answer = gateway.rpc("pax_sequencerReceipt", json!([hash]));
    assert_eq!(answer["result"], support::committed()["receipt"]);
    assert_eq!(answer["id"], 9_007_199_254_740_993_u64);
    let calls = trace.lock().unwrap_or_else(|error| panic!("{}: {error:?}", "node trace"));
    assert_eq!(
        calls.as_slice(),
        [
            json!({"jsonrpc":"2.0", "id":"route-network", "method":"eth_chainId", "params":[]}),
            json!({"jsonrpc":"2.0", "id":9_007_199_254_740_993_u64,"method":"pax_sequencerReceipt","params":[hash]})
        ]
    );
    drop(calls);
    let call = json!({"jsonrpc":"2.0", "id":"exact-bytes", "method":"pax_sequencerReceipt", "params":[hash]});
    let (status, body) = gateway.exchange("POST", "/", call.to_string().as_bytes()).unwrap_or_else(|error| panic!("{}: {error:?}", "receipt bytes over HTTP"));
    assert_eq!(status, 200);
    assert_eq!(
        body,
        serde_json::to_vec(&json!({"jsonrpc":"2.0", "id":"exact-bytes", "result":support::committed()["receipt"]})).unwrap_or_else(|error| panic!("{}: {error:?}", "canonical producer response"))
    );
    assert_eq!(gateway.consumer().pending_nonce(&[0x42; 20]), Ok(9_007_199_254_740_993));
}

#[test]
fn failover_router_path() {
    let fixture = Fixture::start();
    let trace = Arc::new(Mutex::new(Vec::new()));
    let first = fixture.node(true, Arc::clone(&trace));
    let second = fixture.node(false, Arc::clone(&trace));
    let gateway = fixture.gateway(&[&first, &second], None, false);
    gateway.retired();
    assert!(trace.lock().unwrap_or_else(|error| panic!("{}: {error:?}", "node trace")).is_empty(), "retired paths cannot reach an upstream");
    assert_eq!(gateway.consumer().block_number(), Ok(10));
    let calls = std::mem::take(&mut *trace.lock().unwrap_or_else(|error| panic!("{}: {error:?}", "node trace")));
    assert_eq!(calls.len(), 3);
    assert_eq!(calls[0]["id"], "route-network");
    assert_eq!(calls[1]["id"], "route-network");
    assert_eq!(calls[2]["method"], "eth_blockNumber");
    second.failed.store(true, Ordering::Release);
    let refused = gateway.rpc("eth_blockNumber", json!([]));
    assert_eq!(refused["error"]["data"]["code"], "paxeer_node_unavailable");
    assert!(refused.get("result").is_none());
    assert_eq!(trace.lock().unwrap_or_else(|error| panic!("{}: {error:?}", "node trace")).len(), 2);
    first.failed.store(false, Ordering::Release);
    assert_eq!(gateway.consumer().block_number(), Ok(10));
}

#[test]
fn gateway_method_proof_success_gap() {
    support::proof_success_and_recovery();
}

#[test]
fn proof_router_path() {
    let fixture = Fixture::start();
    let gateway = fixture.gateway(&[], None, false);
    gateway.retired();
    let keys = gateway.consumer().receipt_keys().unwrap_or_else(|error| panic!("{}: {error:?}", "root producer to SDK"));
    assert_eq!(keys.len(), 1);
    assert_eq!(keys[0].key_id.as_slice(), support::unhex(&support::committed()["pinnedKey"]["keyId"]));
    assert_eq!(gateway.rpc("px_getReceiptKeys", json!(["unexpected"]))["error"]["code"], -32602);
}
