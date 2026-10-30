use std::io::{BufRead, BufReader};
use std::process::{Child, Command, Stdio};
use std::time::{Duration, Instant};

use layerx_agentd::node_rpc::NodeRpcConfig;
use layerx_agentd::read::balance;
use serde_json::{json, Value};

pub fn must<T, E: std::fmt::Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("{error:?}"))
}

pub struct Node {
    child: Child,
    pub config: NodeRpcConfig,
}

impl Node {
    pub fn start() -> Self {
        Self::with_options(&[])
    }

    pub fn with_options(options: &[&str]) -> Self {
        let executable = std::env::var_os("LAYERX_TEST_ANVIL").unwrap_or_else(|| "anvil".into());
        let mut child = must(
            Command::new(executable)
                .args([
                    "--host",
                    "127.0.0.1",
                    "--port",
                    "0",
                    "--chain-id",
                    "125",
                    "--no-mining",
                ])
                .args(options)
                .stdout(Stdio::piped())
                .stderr(Stdio::inherit())
                .spawn(),
        );
        let stdout = child
            .stdout
            .take()
            .unwrap_or_else(|| panic!("anvil stdout"));
        let (sender, receiver) = std::sync::mpsc::sync_channel(1);
        std::thread::spawn(move || {
            let mut sent = false;
            for line in BufReader::new(stdout).lines() {
                let line = must(line);
                if !sent {
                    if let Some(address) = line.strip_prefix("Listening on ") {
                        let _ = sender.send(format!("http://{address}/"));
                        sent = true;
                    }
                }
            }
        });
        let config = NodeRpcConfig {
            url: String::new(),
            chain_id: 125,
            deadline: Duration::from_secs(2),
        };
        let mut node = Self { child, config };
        node.config.url = must(receiver.recv_timeout(Duration::from_secs(10)));
        node
    }

    pub fn rpc(&self, method: &str, params: Value) -> Value {
        let agent: ureq::Agent = ureq::Agent::config_builder()
            .timeout_global(Some(Duration::from_secs(2)))
            .build()
            .into();
        let mut response = must(
            agent
                .post(&self.config.url)
                .send_json(json!({"jsonrpc": "2.0", "id": 1, "method": method, "params": params})),
        );
        let response: Value = must(response.body_mut().read_json());
        assert!(response.get("error").is_none(), "{method}: {response}");
        response
            .get("result")
            .unwrap_or_else(|| panic!("{method}: {response}"))
            .clone()
    }

    pub fn funded() -> Self {
        let node = Self::start();
        node.rpc(
            "anvil_setBalance",
            json!([format!("0x{}", "11".repeat(20)), "0x384"]),
        );
        node.rpc("anvil_mine", json!(["0x28"]));
        assert_eq!(node.rpc("eth_blockNumber", json!([])), "0x28");
        node
    }
}

impl Drop for Node {
    fn drop(&mut self) {
        let _ = self.child.kill();
        let _ = self.child.wait();
    }
}

#[test]
fn proven_balance_carries_identity_bytes_level_and_all_freshness_coordinates() {
    let node = Node::funded();
    let value = must(balance(&node.config, [0x11; 20], Some(40)));
    assert_eq!(value.account, [0x11; 20]);
    assert_eq!(value.amount.value(), 900);
    assert_eq!(value.canonical_bytes, 900_u128.to_be_bytes());
    assert_eq!(value.achieved_rung, None);
    assert_eq!(value.freshness.chain_head, 40);
    assert_eq!(value.freshness.value_height, 40);
    assert_eq!(value.freshness.relative_to.tag(), "0x28");
    node.rpc(
        "anvil_setBalance",
        json!([format!("0x{}", "11".repeat(20)), "0x385"]),
    );
    node.rpc("anvil_mine", json!(["0x1"]));
    assert_eq!(
        must(balance(&node.config, [0x11; 20], Some(40)))
            .amount
            .value(),
        900
    );
    assert_eq!(
        must(balance(&node.config, [0x11; 20], Some(41)))
            .amount
            .value(),
        901
    );
}

#[test]
fn hostile_asserted_level_cannot_raise_evidence_and_missing_proof_is_named() {
    let node = Node::funded();
    let mut wrong_chain = node.config.clone();
    wrong_chain.chain_id = 126;
    let error = balance(&wrong_chain, [0x11; 20], None).expect_err("another chain must be refused");
    assert_eq!(error.reason.as_str(), "read.chain_id_mismatch");
    node.rpc(
        "anvil_setBalance",
        json!([
            format!("0x{}", "11".repeat(20)),
            "0x100000000000000000000000000000000"
        ]),
    );
    let started = Instant::now();
    let error = balance(&node.config, [0x11; 20], None)
        .expect_err("amount exceeding the protocol width must be refused");
    assert_eq!(error.reason.as_str(), "read.amount_overflow");
    assert!(started.elapsed() < Duration::from_secs(10));
}
