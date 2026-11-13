//! Proves: `mcp_binding_listener.daemon_environment` / `binding_environment` -> process env var -> `HumanNodeEnvironment::from_environment` / `McpHeights::from_environment` / `parse_digest` / `refuse_retired_environment`.
//!
//! The producer reads a real private environment file and execs fresh Rust consumers
//! with Gate.boot's environment builder. `main.rs` uses these exact decoders; neither
//! the producer nor the environment crossing is replaced. There is no persistent
//! anchor state: two reads and two fresh processes must observe each producer, and
//! a second producer changes its network, URL, heights and digest to expose caching.
//!
//! Repairs: `running.md` and `RunningAnAgent.md` advertised obsolete sequence keys;
//! `mcp_binding_listener.py` required and forwarded a retired web signing key instead
//! of a receipt-key index. `main.rs` now explicitly refuses legacy sequence names
//! rather than silently ignoring them. Its existing env decoders are shared through
//! `node_rpc.rs` so this target executes the host's functions without host-only deps.
//! A retired web key is intentionally never admitted, including malformed or empty
//! values; the active digest path is proved with the MCP session identifier.

use std::collections::BTreeMap;
use std::error::Error;
use std::fs::{self, DirBuilder, OpenOptions};
use std::io::Write;
use std::net::TcpListener;
use std::os::unix::fs::{DirBuilderExt, OpenOptionsExt};
use std::path::{Path, PathBuf};
use std::process::Command;
use std::time::Duration;

use layerx_agentd::node_rpc::environment::{
    human_protocol_version, parse_digest, parse_u64, refuse_retired_environment, required,
    HumanNodeEnvironment, McpHeights,
};
use layerx_agentd::node_rpc::{
    NodeRpcConfig, NodeRpcError, LAYERX_AGENT_NODE_CHAIN_ID, LAYERX_AGENT_NODE_RPC_URL,
};
use serde_json::{json, Value};

const CHILD_INPUT: &str = "PROOF_AGENTD_ENV_INPUT";
const NETWORK: &str = "LAYERX_AGENT_HUMAN_NETWORK_ID";
const PROTOCOL: &str = "LAYERX_AGENT_HUMAN_PROTOCOL_VERSION";
const CORE: &str = "LAYERX_AGENT_MCP_CORE_HEIGHT";
const EXPIRY: &str = "LAYERX_AGENT_MCP_EXPIRY_HEIGHT";
const SESSION: &str = "LAYERX_AGENT_MCP_SESSION_ID";
const RETIRED: [&str; 3] = [
    "LAYERX_AGENT_MCP_CORE_SEQUENCE",
    "LAYERX_AGENT_MCP_EXPIRY_SEQUENCE",
    "LAYERX_AGENT_MCP_WEB_SEQUENCER_PUBLIC_KEY",
];

// run_path imports the actual qualification producer without running its outer
// deployment gate. os.execve is the same OS environment transport as Gate.spawn.
const PRODUCE: &str = r"
import json, os, runpy, sys
from pathlib import Path
module, source, root, executable, encoded = sys.argv[1:]
api = runpy.run_path(module, run_name='proof_agentd_env_producer')
case = json.loads(encoded)
try:
    daemon = api['daemon_environment'](Path(source))
except api['GateFailure'] as error:
    assert case.get('producer_error') in str(error), str(error)
    print('PRODUCER_REFUSED ' + case['producer_error'])
    sys.exit(0)
assert 'producer_error' not in case, 'malformed producer file was accepted'
root = Path(root)
state = {key: root / key for key in ('store', 'human_socket', 'root', 'audit', 'socket')}
values = api['binding_environment'](daemon, state, os.geteuid(), os.getegid(),
                                    'read:receipt', web=True,
                                    overrides=case.get('overrides'))
values['PROOF_AGENTD_ENV_INPUT'] = encoded
os.execve(executable, [executable, '--exact', 'proof_agentd_env', '--nocapture'], values)
";

struct Producer {
    root: PathBuf,
    listener: TcpListener,
    values: BTreeMap<String, String>,
}

impl Producer {
    fn new(chain: u32, core: u64, expiry: u64, seed: u8) -> Result<Self, Box<dyn Error>> {
        let mut nonce = [0; 16];
        getrandom::fill(&mut nonce).map_err(|error| std::io::Error::other(error.to_string()))?;
        let root = std::env::temp_dir().join(format!("proof-agentd-env-{}", hex::encode(nonce)));
        DirBuilder::new().mode(0o700).create(&root)?;
        let listener = TcpListener::bind("127.0.0.1:0")?;
        let key = ed25519_dalek::SigningKey::from_bytes(&[seed; 32]);
        let mut values = BTreeMap::new();
        for (key, value) in [
            (NETWORK, chain.to_string()),
            (
                PROTOCOL,
                layerx_wire::limits::STATE_COMMITMENT_PROTOCOL_VERSION.to_string(),
            ),
            (LAYERX_AGENT_NODE_CHAIN_ID, format!("0x{chain:x}")),
            (
                LAYERX_AGENT_NODE_RPC_URL,
                format!("http://{}/", listener.local_addr()?),
            ),
            (CORE, core.to_string()),
            (EXPIRY, expiry.to_string()),
            (SESSION, hex::encode(key.verifying_key().to_bytes())),
            ("LAYERX_AGENT_MCP_CAPABILITY_ID", hex::encode([seed; 32])),
            (
                "LAYERX_AGENT_HUMAN_STORE",
                root.join("original-store").display().to_string(),
            ),
            (
                "LAYERX_AGENT_HUMAN_SOCKET",
                root.join("original-socket").display().to_string(),
            ),
            ("LAYERX_AGENT_MCP_MODE", "full".to_owned()),
            ("LAYERX_AGENT_MCP_SCOPES", "read:receipt".to_owned()),
            (
                "LAYERX_AGENT_PROGRAM_LISTEN",
                listener.local_addr()?.to_string(),
            ),
            (
                "LAYERX_AGENT_MCP_WEB_ENDPOINT",
                format!("http://{}/", listener.local_addr()?),
            ),
            ("LAYERX_AGENT_MCP_WEB_NETWORK", chain.to_string()),
            ("LAYERX_AGENT_MCP_WEB_RECEIPT_KEY_INDEX", "0".to_owned()),
            ("LAYERX_AGENT_MCP_WEB_TIMEOUT_MS", "1000".to_owned()),
            ("LAYERX_AGENT_MCP_WEB_PENDING_ATTEMPTS", "2".to_owned()),
            ("LAYERX_AGENT_MCP_WEB_APPROVAL_THRESHOLD", "1".to_owned()),
        ] {
            values.insert(key.to_owned(), value);
        }
        Ok(Self {
            root,
            listener,
            values,
        })
    }

    fn write(&self, suffix: &str) -> Result<PathBuf, Box<dyn Error>> {
        let path = self.root.join("daemon.env");
        let mut file = OpenOptions::new()
            .write(true)
            .create(true)
            .truncate(true)
            .mode(0o600)
            .open(&path)?;
        for (key, value) in &self.values {
            writeln!(file, "{key}={value}")?;
        }
        write!(file, "{suffix}")?;
        Ok(path)
    }

    fn run(&self, mut case: Value, suffix: &str) -> Result<(), Box<dyn Error>> {
        let repo = Path::new(env!("CARGO_MANIFEST_DIR")).join("../../../..");
        case["expected"] = json!({
            "network": self.values[NETWORK], "protocol": self.values[PROTOCOL],
            "url": self.values[LAYERX_AGENT_NODE_RPC_URL], "core": self.values[CORE],
            "expiry": self.values[EXPIRY], "session": self.values[SESSION],
        });
        // Keep the real bound URL alive until the consumer exits.
        assert!(self.listener.local_addr()?.port() > 0);
        let output = Command::new("python3")
            .args(["-I", "-c", PRODUCE])
            .arg(repo.join("tools/qualification/paxeer-x/mcp_binding_listener.py"))
            .arg(self.write(suffix)?)
            .arg(&self.root)
            .arg(std::env::current_exe()?)
            .arg(serde_json::to_string(&case)?)
            .output()?;
        let stdout = String::from_utf8(output.stdout)?;
        let stderr = String::from_utf8(output.stderr)?;
        assert!(output.status.success(), "case={case}\n{stdout}\n{stderr}");
        if let Some(error) = case.get("producer_error").and_then(Value::as_str) {
            assert!(
                stdout.contains(&format!("PRODUCER_REFUSED {error}")),
                "{stdout}"
            );
        } else {
            assert!(
                stdout.contains("1 passed"),
                "child did not execute: {stdout}"
            );
        }
        Ok(())
    }
}

impl Drop for Producer {
    fn drop(&mut self) {
        let _ = fs::remove_dir_all(&self.root);
    }
}

fn observe(case: &Value) -> Result<(), Box<dyn Error>> {
    let kind = case["kind"].as_str().ok_or("missing consumer case")?;
    let name = case["name"].as_str().unwrap_or("");
    // Re-decode twice in the same process as well as testing fresh process startup.
    for _ in 0..2 {
        match kind {
            "happy" => {
                refuse_retired_environment()?;
                let expected = &case["expected"];
                let environment = HumanNodeEnvironment::from_environment()?;
                assert_eq!(
                    environment.expected_network_id.to_string(),
                    expected["network"]
                );
                assert_eq!(
                    environment.protocol_version.to_string(),
                    expected["protocol"]
                );
                assert_eq!(
                    human_protocol_version()?,
                    layerx_wire::limits::STATE_COMMITMENT_PROTOCOL_VERSION
                );
                assert_eq!(environment.rpc.url, expected["url"]);
                assert_eq!(
                    environment.rpc.chain_id,
                    u64::from(environment.expected_network_id)
                );
                assert_eq!(environment.rpc.deadline, Duration::from_secs(30));
                assert_eq!(NodeRpcConfig::from_environment()?, environment.rpc);
                assert_eq!(required(NETWORK)?, expected["network"]);
                let heights = McpHeights::from_environment()?;
                assert_eq!(heights.core_height.to_string(), expected["core"]);
                assert_eq!(heights.expiry_height.to_string(), expected["expiry"]);
                assert_eq!(parse_u64(CORE)?, heights.core_height);
                assert_eq!(hex::encode(parse_digest(SESSION)?), expected["session"]);
                assert_eq!(required("LAYERX_AGENT_MCP_WEB_RECEIPT_KEY_INDEX")?, "0");
            }
            "human" => {
                let error = lint_error(
                    HumanNodeEnvironment::from_environment(),
                    "malformed Human configuration was accepted",
                );
                assert!(
                    error.contains(case["error"].as_str().ok_or("missing error")?),
                    "{error}"
                );
            }
            "heights" => {
                let error = lint_error(
                    McpHeights::from_environment(),
                    "malformed height was accepted",
                );
                assert!(error.contains(name), "{error}");
            }
            "digest" => assert!(parse_digest(SESSION).is_err()),
            "rpc" => {
                assert_eq!(
                    NodeRpcConfig::from_environment(),
                    Err(NodeRpcError::InvalidVariable(
                        if name == LAYERX_AGENT_NODE_RPC_URL {
                            LAYERX_AGENT_NODE_RPC_URL
                        } else {
                            LAYERX_AGENT_NODE_CHAIN_ID
                        }
                    ))
                );
            }
            "missing" => {
                assert!(required(name).is_err(), "missing {name} was accepted");
            }
            "retired" => {
                assert_eq!(
                    refuse_retired_environment(),
                    Err(format!("{name} is retired"))
                );
            }
            _ => panic!("unknown consumer case: {kind}"),
        }
    }
    Ok(())
}

#[test]
fn proof_agentd_env() -> Result<(), Box<dyn Error>> {
    if let Ok(encoded) = std::env::var(CHILD_INPUT) {
        return observe(&serde_json::from_str::<Value>(&encoded)?);
    }
    let first = Producer::new(125, 42, 99, 7)?;
    let second = Producer::new(126, 43, 100, 8)?;
    for producer in [&first, &second] {
        for _ in 0..2 {
            producer.run(json!({"kind": "happy"}), "")?;
        }
    }
    for (name, value, error) in [
        (NETWORK, "wrong-network", "human network id is invalid"),
        (NETWORK, "4294967296", "human network id is invalid"),
        (NETWORK, "126", "human node RPC network id mismatch"),
        (PROTOCOL, "malformed", "human protocol version is invalid"),
        (
            PROTOCOL,
            "1",
            "human protocol version is not the current beta protocol",
        ),
    ] {
        first.run(
            json!({"kind": "human", "error": error, "overrides": {name: value}}),
            "",
        )?;
    }
    for name in [CORE, EXPIRY] {
        for value in ["-1", "18446744073709551616", "not-a-height"] {
            first.run(
                json!({"kind": "heights", "name": name, "overrides": {name: value}}),
                "",
            )?;
        }
    }
    for value in ["not-a-digest", &"g".repeat(64)] {
        first.run(json!({"kind": "digest", "overrides": {SESSION: value}}), "")?;
    }
    for (name, value) in [
        (LAYERX_AGENT_NODE_RPC_URL, " http://127.0.0.1/"),
        (LAYERX_AGENT_NODE_CHAIN_ID, "0x07d"),
        (LAYERX_AGENT_NODE_CHAIN_ID, "wrong-chain"),
    ] {
        first.run(
            json!({"kind": "rpc", "name": name, "overrides": {name: value}}),
            "",
        )?;
    }
    for name in [NETWORK, PROTOCOL, CORE, EXPIRY, SESSION] {
        for value in [Value::Null, json!("")] {
            first.run(
                json!({"kind": "missing", "name": name, "overrides": {name: value}}),
                "",
            )?;
        }
    }
    for name in RETIRED {
        for value in ["", "malformed-private-producer-result"] {
            first.run(
                json!({"kind": "retired", "name": name, "overrides": {name: value}}),
                "",
            )?;
            first.run(
                json!({"producer_error": format!("{name} is retired")}),
                &format!("{name}={value}\n"),
            )?;
        }
    }
    first.run(
        json!({"producer_error": "repeats"}),
        &format!("{NETWORK}=125\n"),
    )?;
    first.run(
        json!({"producer_error": "not KEY=VALUE"}),
        "malformed input\n",
    )?;

    verify_environment_documentation();
    Ok(())
}

#[track_caller]
fn lint_error<T: std::fmt::Debug, E>(result: Result<T, E>, context: &str) -> E {
    match result {
        Err(error) => error,
        Ok(value) => panic!("{context}: unexpected success {value:?}"),
    }
}

fn verify_environment_documentation() {
    for document in [
        include_str!("../../../../../docs/site/docs/agents/running.md"),
        include_str!("../../../../../docs/wiki/RunningAnAgent.md"),
    ] {
        for name in [
            NETWORK,
            PROTOCOL,
            CORE,
            EXPIRY,
            LAYERX_AGENT_NODE_CHAIN_ID,
            LAYERX_AGENT_NODE_RPC_URL,
        ] {
            assert!(
                document.contains(&format!("| `{name}` |")),
                "undocumented active environment key {name}"
            );
        }
        for name in RETIRED {
            assert!(document.contains(name), "undocumented retirement {name}");
            assert!(
                !document.contains(&format!("| `{name}` |")),
                "retired key is still advertised: {name}"
            );
        }
    }
}
