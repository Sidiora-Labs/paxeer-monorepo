#[path = "balance.rs"]
mod balance_tests;

use balance_tests::{must, Node};
use layerx_agentd::read::balance;
use serde_json::json;

#[test]
fn pending_balance_reports_value_height_and_chain_head() {
    let node = Node::funded();
    let value = must(balance(&node.config, [0x11; 20], None));
    assert_eq!(value.amount.value(), 900);
    assert_eq!(value.account, [0x11; 20]);
    assert_eq!(value.canonical_bytes, 900_u128.to_be_bytes());
    assert_eq!(value.achieved_rung, None);
    assert_eq!(value.freshness.chain_head, 40);
    assert_eq!(value.freshness.value_height, 41);
    assert_eq!(value.freshness.relative_to.tag(), "pending");
    assert_eq!(
        node.rpc(
            "eth_getBalance",
            json!([format!("0x{}", "11".repeat(20)), "pending"])
        ),
        format!("{:#x}", value.amount.value())
    );
    let at_height = must(balance(&node.config, [0x11; 20], Some(40)));
    assert_eq!(at_height.amount.value(), 900);
    assert_eq!(at_height.freshness.value_height, 40);
    assert_eq!(at_height.freshness.chain_head, 40);
    assert_eq!(at_height.freshness.relative_to.tag(), "0x28");
    assert_eq!(at_height.achieved_rung, None);

    let accounts = node.rpc("eth_accounts", json!([]));
    node.rpc(
        "eth_sendTransaction",
        json!([{
            "from": accounts[0],
            "to": format!("0x{}", "11".repeat(20)),
            "value": "0x1",
            "gas": "0x5208"
        }]),
    );
    let pending = must(balance(&node.config, [0x11; 20], None));
    assert_eq!(pending.amount.value(), 901);
    assert_eq!(pending.freshness.chain_head, 40);
    assert_eq!(pending.freshness.value_height, 41);
    assert_eq!(pending.freshness.relative_to.tag(), "pending");
    assert_eq!(pending.achieved_rung, None);
    assert_eq!(
        must(balance(&node.config, [0x11; 20], Some(40)))
            .amount
            .value(),
        900
    );
}

#[test]
fn balance_at_a_height_ahead_of_head_is_refused() {
    let node = Node::funded();
    let error = balance(&node.config, [0x11; 20], Some(45))
        .expect_err("a height beyond the captured chain head must not return an amount");
    assert_eq!(error.reason.as_str(), "read.height_ahead");
    assert_eq!(node.rpc("eth_blockNumber", json!([])), "0x28");
    let error = balance(&node.config, [0x11; 20], Some(41))
        .expect_err("the pending height is also ahead when requested explicitly");
    assert_eq!(error.reason.as_str(), "read.height_ahead");

    let pruned = Node::with_options(&["--prune-history", "1"]);
    pruned.rpc("anvil_mine", json!(["0x28"]));
    let error = balance(&pruned.config, [0x11; 20], Some(1))
        .expect_err("pruned historical state must not return a balance");
    assert_eq!(error.reason.as_str(), "read.height_pruned");
}

struct RecordingNode {
    config: layerx_agentd::node_rpc::NodeRpcConfig,
    requests: std::sync::Arc<std::sync::Mutex<Vec<serde_json::Value>>>,
    stop: std::sync::mpsc::Sender<()>,
    worker: Option<std::thread::JoinHandle<()>>,
}

impl RecordingNode {
    fn start(node: &Node) -> Self {
        use std::io::{BufRead, Read, Write};
        use std::sync::{Arc, Mutex};
        use std::time::Duration;

        let listener = must(std::net::TcpListener::bind("127.0.0.1:0"));
        let mut config = node.config.clone();
        config.url = format!("http://{}/", must(listener.local_addr()));
        must(listener.set_nonblocking(true));
        let upstream = node.config.url.clone();
        let requests = Arc::new(Mutex::new(Vec::new()));
        let recorded = requests.clone();
        let (stop, stopped) = std::sync::mpsc::channel();
        let worker = std::thread::spawn(move || {
            let agent: ureq::Agent = ureq::Agent::config_builder()
                .timeout_global(Some(Duration::from_secs(2)))
                .build()
                .into();
            while stopped.try_recv().is_err() {
                let (stream, _) = match listener.accept() {
                    Ok(connection) => connection,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        std::thread::sleep(Duration::from_millis(1));
                        continue;
                    }
                    Err(error) => panic!("record node request: {error}"),
                };
                must(stream.set_read_timeout(Some(Duration::from_secs(2))));
                must(stream.set_write_timeout(Some(Duration::from_secs(2))));
                let mut reader = std::io::BufReader::new(stream);
                let mut length = None;
                loop {
                    let mut line = String::new();
                    assert_ne!(must(reader.read_line(&mut line)), 0);
                    if line == "\r\n" {
                        break;
                    }
                    if let Some((name, value)) = line.split_once(':') {
                        if name.eq_ignore_ascii_case("content-length") {
                            length = Some(must(value.trim().parse::<usize>()));
                        }
                    }
                }
                let mut body = vec![0; length.expect("JSON RPC content length")];
                must(reader.read_exact(&mut body));
                let request: serde_json::Value = must(serde_json::from_slice(&body));
                must(recorded.lock()).push(request.clone());
                let mut response = must(agent.post(&upstream).send_json(&request));
                assert_eq!(response.status().as_u16(), 200);
                let body = must(response.body_mut().read_to_vec());
                let mut stream = reader.into_inner();
                must(write!(stream,
                    "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n",
                    body.len()));
                must(stream.write_all(&body));
            }
        });
        Self {
            config,
            requests,
            stop,
            worker: Some(worker),
        }
    }
}

impl Drop for RecordingNode {
    fn drop(&mut self) {
        let _ = self.stop.send(());
        if let Some(worker) = self.worker.take() {
            worker.join().expect("recording node worker");
        }
    }
}

#[test]
fn ledger_balance_reads_the_requested_asset_through_the_ledger_precompile() {
    use layerx_agentd::human_runtime::{balance_operation_value, read::ledger_balance};

    let node = Node::start();
    let ledger = "0x000000000000000000000000000000000000101a";
    let account = [0x11; 32];
    let asset = [0x22; 32];
    let other_asset = [0x33; 32];
    let other_account = [0x44; 32];
    let overflow_asset = [0x55; 32];
    let served = u128::MAX - 17;
    let selector = layerx_crypto::evm_transaction::transaction_hash(b"balanceOf(bytes32,bytes32)");
    let selector = hex::encode(&selector[..4]);
    assert_eq!(selector, "b1ffc03e");
    let getter = format!(
        "0x36604414600a575f5ffd5b5f3560e01c63{selector}14601c575f5ffd5b604060045f3760405f20545f5260205ff3"
    );
    node.rpc("anvil_setCode", json!([ledger, getter]));
    for (owner, token, word) in [
        (account, asset, format!("{served:064x}")),
        (account, other_asset, format!("{:064x}", 1234)),
        (other_account, asset, format!("{:064x}", 5678)),
        (account, overflow_asset, format!("{:032x}{:032x}", 1, 0)),
    ] {
        let key = layerx_crypto::evm_transaction::transaction_hash(&[owner, token].concat());
        node.rpc(
            "anvil_setStorageAt",
            json!([
                ledger,
                format!("0x{}", hex::encode(key)),
                format!("0x{word}")
            ]),
        );
    }
    node.rpc(
        "anvil_setBalance",
        json!([format!("0x{}", "11".repeat(20)), "0x384"]),
    );
    node.rpc("anvil_mine", json!(["0x28"]));
    let recorded = RecordingNode::start(&node);

    for (owner, token, height, expected) in [
        (account, asset, None, served),
        (account, asset, Some(40), served),
        (account, other_asset, None, 1234),
        (other_account, asset, None, 5678),
        (account, [0x66; 32], None, 0),
    ] {
        let value = must(ledger_balance(&recorded.config, owner, token, height));
        assert_eq!(value.account, owner);
        assert_eq!(value.asset, token);
        assert_eq!(value.amount.value(), expected);
        assert_eq!(value.canonical_bytes, expected.to_be_bytes());
        assert_eq!(value.achieved_rung, None);
        assert_eq!(value.freshness.chain_head, 40);
        assert_eq!(value.freshness.value_height, height.unwrap_or(41));
        assert_eq!(
            value.freshness.relative_to.tag(),
            if height.is_some() { "0x28" } else { "pending" }
        );
        let response = balance_operation_value(
            owner,
            token,
            value.amount,
            &value.canonical_bytes,
            value.freshness.chain_head,
            value.freshness.value_height,
        );
        assert_eq!(response["value"]["asset"], hex::encode(token));
        assert_eq!(response["value"]["amount"], expected.to_string());
    }
    let error = ledger_balance(&recorded.config, account, overflow_asset, None)
        .expect_err("a uint256 exceeding Amount must be refused");
    assert_eq!(error.reason.as_str(), "read.amount_overflow");
    let error = ledger_balance(&recorded.config, account, asset, Some(45))
        .expect_err("a ledger balance ahead of the captured head must be refused");
    assert_eq!(error.reason.as_str(), "read.height_ahead");

    let requests = must(recorded.requests.lock());
    assert!(requests
        .iter()
        .all(|request| request["method"] != "eth_getBalance"));
    let calls: Vec<_> = requests
        .iter()
        .filter(|request| request["method"] == "eth_call")
        .collect();
    assert_eq!(calls.len(), 6);
    for (call, (owner, token, tag)) in calls.iter().zip([
        (account, asset, "pending"),
        (account, asset, "0x28"),
        (account, other_asset, "pending"),
        (other_account, asset, "pending"),
        (account, [0x66; 32], "pending"),
        (account, overflow_asset, "pending"),
    ]) {
        assert_eq!(
            call["params"],
            json!([{
            "to": ledger,
            "data": format!("0x{selector}{}{}", hex::encode(owner), hex::encode(token)),
        }, tag])
        );
    }
}
