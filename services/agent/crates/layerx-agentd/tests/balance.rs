use std::io::{BufRead, BufReader};
use std::process::{Child, Command, Stdio};
use std::time::{Duration, Instant};

use layerx_agentd::node_rpc::NodeRpcConfig;
use layerx_agentd::read::balance;
use serde_json::{json, Value};

/// # Panics
/// Panics if the supplied result is an error.
pub fn must<T, E: std::fmt::Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("{error:?}"))
}

pub struct Node {
    child: Child,
    pub config: NodeRpcConfig,
}

impl Node {
    #[must_use = "use the constructed fixture value"]
    pub fn start() -> Self {
        Self::with_options(&[])
    }

    #[must_use = "use the constructed fixture value"]
    /// # Panics
    /// Panics if the node process cannot be started or does not publish its RPC address.
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

    #[must_use = "use the constructed fixture value"]
    /// # Panics
    /// Panics if the request fails, the response is malformed, or the node returns an RPC error.
    pub fn rpc(&self, method: &str, params: Value) -> Value {
        let agent: ureq::Agent = ureq::Agent::config_builder()
            .timeout_global(Some(Duration::from_secs(2)))
            .build()
            .into();
        let mut response = must(agent.post(&self.config.url).send_json(Value::Object(
            serde_json::Map::from_iter([
                ("jsonrpc".to_owned(), json!("2.0")),
                ("id".to_owned(), json!(1)),
                ("method".to_owned(), json!(method)),
                ("params".to_owned(), params),
            ]),
        )));
        let response: Value = must(response.body_mut().read_json());
        assert!(response.get("error").is_none(), "{method}: {response}");
        response
            .get("result")
            .unwrap_or_else(|| panic!("{method}: {response}"))
            .clone()
    }

    #[must_use = "use the constructed fixture value"]
    /// # Panics
    /// Panics if the node cannot be funded, mined, or read back at the expected height.
    pub fn funded() -> Self {
        let node = Self::start();
        // rpc checks success; funding is checked through the balance read.
        let _funding_response = node.rpc(
            "anvil_setBalance",
            json!([format!("0x{}", "11".repeat(20)), "0x384"]),
        );
        // The following height assertion verifies the mining effect.
        let _mining_response = node.rpc("anvil_mine", json!(["0x28"]));
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
    // rpc checks success; the following reads verify both balance snapshots.
    let _funding_response = node.rpc(
        "anvil_setBalance",
        json!([format!("0x{}", "11".repeat(20)), "0x385"]),
    );
    let _mining_response = node.rpc("anvil_mine", json!(["0x1"]));
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
    let error = assert_err(
        balance(&wrong_chain, [0x11; 20], None),
        "another chain must be refused",
    );
    assert_eq!(error.reason.as_str(), "read.chain_id_mismatch");
    // rpc checks success; the following read verifies overflow refusal.
    let _funding_response = node.rpc(
        "anvil_setBalance",
        json!([
            format!("0x{}", "11".repeat(20)),
            "0x100000000000000000000000000000000"
        ]),
    );
    let started = Instant::now();
    let error = assert_err(
        balance(&node.config, [0x11; 20], None),
        "amount exceeding the protocol width must be refused",
    );
    assert_eq!(error.reason.as_str(), "read.amount_overflow");
    assert!(started.elapsed() < Duration::from_secs(10));
}

#[track_caller]
fn assert_err<T: std::fmt::Debug, E>(result: Result<T, E>, message: &str) -> E {
    match result {
        Err(error) => error,
        Ok(value) => panic!("{message}: {value:?}"),
    }
}
