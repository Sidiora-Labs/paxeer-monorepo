//! Audit 22.30.95 findings 17, 14, 13, 29, 23, 30, 32 and 20.
//! Closure mapping and out-of-path wiring requests: `repair_22_33_1537.kvx`.
//! The startup fixture authenticates an exact spend of 25 through node RPC.
//! Restart recovery consumes the current evidence inventory through the production consumer.
fn require_ok<T, E: std::fmt::Debug>(result: Result<T, E>, message: &str) -> T {
    result.unwrap_or_else(|error| panic!("{message}: {error:?}"))
}

fn require_some<T>(value: Option<T>, message: &str) -> T {
    value.unwrap_or_else(|| panic!("{message}"))
}

#[path = "native_routes_rpc.rs"]
mod native_routes_rpc;
#[path = "suites/outbox_recover.rs"]
mod outbox_recover;
#[path = "suites/prepare_expiry.rs"]
mod prepare_expiry;
#[path = "suites/protocol_evidence.rs"]
mod protocol_evidence;
#[path = "suites/recovery.rs"]
mod recovery;
mod support;

use layerx_agentd::boot::readiness_gate;
use layerx_agentd::node_rpc::NodeRpc;
use serde_json::json;

#[test]
fn finding_13_real_node_transport_accepts_current_keys_and_refuses_empty_keys() {
    for answer in [
        support::current_node::keys(8, 100, 199),
        json!({"keys": []}),
    ] {
        let (config, server) = support::serve_node(vec![
            ("eth_chainId", json!([]), Some(json!("0x7d"))),
            ("px_getReceiptKeys", json!([]), Some(answer.clone())),
        ]);
        let node = require_ok(NodeRpc::connect(&config), "chain checked real node client");
        let observed = readiness_gate(&node);
        if require_some(answer["keys"].as_array(), "keys array").is_empty() {
            assert_eq!(observed, Err(layerx_agentd::boot::GateError::NoReceiptKeys));
        } else {
            let status = require_ok(observed, "current node key response");
            assert_eq!(status.chain_id, 125);
            assert_eq!(status.receipt_keys.len(), 1);
            assert_eq!(status.receipt_keys[0].key_id, [1; 32]);
            assert_eq!(status.receipt_keys[0].first_height, 100);
            assert_eq!(status.receipt_keys[0].last_height, 199);
        }
        require_ok(server.join(), "node method and parameter assertions");
    }
}

#[test]
fn finding_30_loaded_startup_restarts_and_refuses_malformed_producer() {
    support::current_node::startup_restart();
}

use support::send_authorization;
