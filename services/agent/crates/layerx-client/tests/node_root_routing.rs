#[path = "support/node_root_22_34_5.rs"]
mod support;

use layerx_client::px::{PxEndpoint, DEFAULT_NODE_ENDPOINT, NODE_READ_METHODS};
use layerx_client::rpc::{commit, history};
use layerx_client::sequencer::{SequencerError, SequencerReceipt};
use serde_json::{json, Value};
use support::{must, Server};

#[test]
fn every_served_method_uses_node_root_and_preserves_recorded_refusals() {
    let node = Server::recorded();
    let gateway = Server::start("/rpc", |_| panic!("node read reached the hosted gateway"));
    let client = must(PxEndpoint::with_gateway(&node.url, &[], &gateway.url, &[]));
    support::assert_recorded_reads(&client);
    assert_eq!(node.count(), 16);
    assert_eq!(gateway.count(), 0);
    assert_eq!(
        must(node.requests.lock())
            .iter()
            .skip(1)
            .map(|request| (match request["method"].as_str() {
                Some(required_value) => required_value,
                None => panic!("method"),
            })
            .to_owned())
            .collect::<Vec<_>>(),
        NODE_READ_METHODS
    );
}

#[test]
fn only_registration_and_funding_use_the_explicit_hosted_endpoint() {
    let node = Server::start("/", |_| panic!("hosted operation reached the node"));
    // Closing the peer after accepting the request exercises actual transport
    // failure, without fabricating successful registration or funding results.
    let gateway = Server::start("/rpc", |request| {
        assert!(matches!(
            request["method"].as_str(),
            Some("px_register" | "px_requestFunds")
        ));
        assert_eq!(request["params"], json!([]));
        None
    });
    let client = must(PxEndpoint::with_gateway(&node.url, &[], &gateway.url, &[]));
    for method in ["px_register", "px_requestFunds"] {
        assert_eq!(
            client.call(method, &json!([])),
            Err(SequencerError::Unavailable)
        );
    }
    assert_eq!(gateway.count(), 2);
    assert_eq!(node.count(), 0);
    let node_only = must(PxEndpoint::new(&node.url, &[]));
    for method in [
        "px_register",
        "px_requestFunds",
        "px_typo",
        "eth_sendRawTransaction",
    ] {
        assert_eq!(
            node_only.call(method, &json!([])),
            Err(SequencerError::Endpoint)
        );
    }
    assert_eq!(node.count(), 0);
    assert_eq!(gateway.count(), 2);
}

#[test]
fn node_urls_require_the_root_and_keep_transport_security() {
    assert_eq!(
        DEFAULT_NODE_ENDPOINT,
        "https://api1.mainnet-beta.paxeer.network/"
    );
    let node = support::storage_server();
    for suffix in ["rpc", "evm", "?key=value", "#fragment", " "] {
        assert!(matches!(
            PxEndpoint::new(&format!("{}{suffix}", node.url), &[]),
            Err(SequencerError::Endpoint)
        ));
    }
    for url in [
        "http://api1.mainnet-beta.paxeer.network/",
        "https://api1.mainnet-beta.paxeer.network/",
        "http://user:pass@127.0.0.1/",
        "file:///node",
    ] {
        assert!(matches!(
            PxEndpoint::new(url, &[]),
            Err(SequencerError::Endpoint)
        ));
    }
    assert_eq!(node.count(), 0);
}

fn typed_reads_node(served: Value, unified: Value, receipt: Value) -> Server {
    Server::start("/", move |request| {
        let Some(method) = request["method"].as_str() else {
            panic!("method")
        };
        let result = match method {
            "px_getAccount" | "px_getBalances" => {
                assert_eq!(
                    request["params"],
                    json!([served["account"]["account"]["evm_address"]])
                );
                served[if method == "px_getAccount" {
                    "account"
                } else {
                    "balances"
                }]
                .clone()
            }
            "px_listAssets" => {
                assert_eq!(request["params"], json!([]));
                served["assets"].clone()
            }
            "px_getHistory" => {
                assert_eq!(
                    request["params"],
                    json!([served["history"]["account"], null, 50])
                );
                served["history"].clone()
            }
            "px_resolveAccount" => {
                assert_eq!(request["params"], json!([unified["account"]["layerx_did"]]));
                unified["account"].clone()
            }
            "px_getUnifiedHistory" => {
                assert_eq!(
                    request["params"],
                    json!([unified["account"]["layerx_did"], null, 25])
                );
                unified.clone()
            }
            "px_getCommit" | "px_getValidators" => {
                assert_eq!(
                    request["params"],
                    json!([(match receipt["height"].as_u64() {
                        Some(required_value) => required_value,
                        None => panic!("height"),
                    })
                    .to_string()])
                );
                receipt[if method == "px_getCommit" {
                    "material"
                } else {
                    "validators"
                }]
                .clone()
            }
            "px_getReceiptKeys" => {
                assert_eq!(request["params"], json!([]));
                json!({"keys": [receipt["key"]]})
            }
            "pax_sequencerReceipt" => {
                assert_eq!(request["params"], json!([receipt["view"]["txHash"]]));
                receipt["view"].clone()
            }
            other => panic!("unrecorded producer method {other}"),
        };
        Some(json!({"result": result}))
    })
}

#[test]
fn typed_reads_decode_committed_producer_results_on_the_node_connection() {
    let reads = support::gateway_reads();
    let served = reads.clone();
    let history_record = support::unified_history();
    let unified = history_record.clone();
    let producer = support::producer()["receipt"].clone();
    let receipt = producer.clone();
    let node = typed_reads_node(served, unified, receipt);
    let gateway = Server::start("/rpc", |_| panic!("producer read reached the gateway"));
    let px = must(PxEndpoint::with_gateway(&node.url, &[], &gateway.url, &[]));
    let Some(address) = reads["account"]["account"]["evm_address"].as_str() else {
        panic!("address")
    };
    let account = must(px.account(address));
    assert!(account.account.bound);
    let Some(paxeer) = account.paxeer else {
        panic!("Paxeer account")
    };
    assert_eq!(paxeer.height, 16);
    assert_eq!(paxeer.nonce, 42);
    assert_eq!(paxeer.balance.decimal(), "81985529216486895");
    let balances = must(px.balances(address));
    assert_eq!(balances.height, 16);
    assert_eq!(balances.account, account.account);
    assert_eq!(balances.balances.len(), 2);
    assert_eq!(
        balances.balances[1].amount.decimal(),
        match reads["balances"]["balances"][1]["amount"].as_str() {
            Some(required_value) => required_value,
            None => panic!("amount"),
        }
    );
    let assets = must(px.list_assets());
    assert_eq!(assets.height, 16);
    assert_eq!(
        assets.assets.len(),
        (match reads["assets"]["assets"].as_array() {
            Some(required_value) => required_value,
            None => panic!("assets"),
        })
        .len()
    );
    let page = must(history::history(
        px.node(),
        match reads["history"]["account"].as_str() {
            Some(required_value) => required_value,
            None => panic!("history account"),
        },
        None,
        None,
    ));
    assert_eq!(
        page.items,
        *match reads["history"]["items"].as_array() {
            Some(required_value) => required_value,
            None => panic!("items"),
        }
    );
    assert_eq!(page.freshness, reads["history"]["freshness"]);
    let did = &history_record["account"]["layerx_did"];
    assert_eq!(
        must(px.call("px_resolveAccount", &json!([did]))),
        history_record["account"]
    );
    assert_eq!(
        must(px.call("px_getUnifiedHistory", &json!([did, null, 25]))),
        history_record
    );
    assert_committed_receipt_reads(&px, &producer);
    assert_eq!(gateway.count(), 0);
}

fn assert_committed_receipt_reads(px: &PxEndpoint, producer: &Value) {
    let Some(height) = producer["height"].as_u64() else {
        panic!("height")
    };
    assert_eq!(
        must(px.node().commit_material(height)),
        producer["material"]
    );
    let validators = must(commit::decode_validators(&producer["validators"]));
    let verifier = layerx_programs_registry::chain_proof::ChainProofVerifier {
        chain_id: "hyperpax_125-1".to_owned(),
        validators,
    };
    let head = must(commit::acquire_committed_header(
        px.node(),
        height,
        &verifier,
    ));
    assert_eq!(
        head.header_hash(),
        support::hex_bytes::<32>(&producer["hash"])
    );
    let keys = must(px.node().receipt_keys());
    assert_eq!(keys.len(), 1);
    assert_eq!(
        keys[0].public_key,
        support::hex_bytes::<32>(&producer["key"]["publicKey"])
    );
    let receipt = must(
        px.node()
            .sequencer_receipt(&support::hex_bytes::<32>(&producer["view"]["txHash"])),
    );
    assert!(
        matches!(receipt, Some(SequencerReceipt::Receipted(value)) if value.height == 1000 && value.seq == 1)
    );
}

#[test]
fn node_network_reads_keep_snapshot_and_unavailability_checks() {
    let records = support::gateway_reads();
    for (record, expected) in [
        (records["network"].clone(), SequencerError::Malformed),
        (records["unavailable"].clone(), SequencerError::Unavailable),
    ] {
        let node = Server::start("/", move |request| {
            assert_eq!(request["method"], "px_getNetwork");
            assert_eq!(request["params"], json!([]));
            Some(json!({"result": record}))
        });
        let gateway = Server::start("/rpc", |_| {
            panic!("network refusal retried on hosted gateway")
        });
        let px = must(PxEndpoint::with_gateway(&node.url, &[], &gateway.url, &[]));
        assert_eq!(px.network(), Err(expected));
        assert_eq!(node.count(), 1);
        assert_eq!(gateway.count(), 0);
    }
}
