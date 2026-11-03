//! Proves: WalletCommand/ProgramCommand/ReceiptCommand/McpCommand -> built layerx subprocess -> common::envelope, toolset::invoke, check_cli_output and beta_driver::cli_output.
//!
//! Repairs main.rs::main emitting non-JSON argument failures, wallet.rs::run_wallet
//! contacting RPC before validating an asset selector, and toolset.rs::invoke
//! using a retired REST activity receipt route instead of pax_sequencerReceipt.
//! The existing activity_id argument name is retained; its value is a transaction hash.
//!
//! External limitation: tools/platform/emulator/src/main.rs::route has no /rpc
//! route. Valid receipt, registry, discovery, call and token-list requests therefore
//! currently refuse with HTTP 404, rather than providing successful EVM results.
//! services/agent/crates/layerx-sdk/src/rpc.rs::RpcClient::call maps the emulator's
//! non-RPC error document to RpcError::InvalidResponse; token list consequently
//! reports rpc_method_unavailable. Assert that exact current consumer behaviour
//! alongside the real route's 404, without repairing the SDK outside this task.
//! This proof asserts that behaviour through the real emulator without replacing it.
//! tools/platform/cli/src/rpc.rs::request intentionally refuses px_listAssets;
//! token listing uses layerx_sdk::rpc::RpcClient::list_assets_page instead.
//! Probe the missing /rpc route with the published pax_sequencerReceipt request,
//! and independently assert both the CLI builder's refusal and the SDK result.
//! MCP refuses absent or malformed enrolment, rather than manufacturing a session.
//! Wallet and core state are produced by real commands, snapshotted by the core,
//! stopped, reopened in a new process and observed by the actual consumers.
#![allow(dead_code)]

mod common;
#[path = "../src/config.rs"]
mod config;
#[path = "../src/credential.rs"]
mod credential;
#[path = "../src/encoding.rs"]
mod encoding;
#[path = "../src/faucet.rs"]
mod faucet;
#[path = "../src/file_store.rs"]
mod file_store;
#[path = "../src/output.rs"]
mod output;
#[path = "../src/toolset.rs"]
mod toolset;

use layerx_platform_cli::http;
use serde_json::{json, Value};
use std::io::Write as _;
use std::net::TcpListener;
use std::path::Path;
use std::process::{Child, Command, Output, Stdio};
use std::time::{Duration, Instant};

struct Process(Child);
impl Drop for Process {
    fn drop(&mut self) {
        let _ = self.0.kill();
        let _ = self.0.wait();
    }
}

fn command(root: &Path) -> Command {
    let mut command = Command::new(env!("CARGO_BIN_EXE_layerx"));
    command
        .env_clear()
        .env("LAYERX_CONFIG", root.join("config.json"))
        .env("LAYERX_INSTALL_ROOT", root)
        .env("LAYERX_CREDENTIAL_STORE", "file")
        .env(
            "LAYERX_CREDENTIAL_PASSPHRASE",
            "private command proof passphrase",
        )
        .arg("--json");
    command
}

fn run(root: &Path, args: &[&str]) -> Output {
    command(root).args(args).output().expect("built CLI runs")
}

fn success(output: &Output, kind: &str) -> Value {
    assert!(
        output.status.success(),
        "{}",
        String::from_utf8_lossy(&output.stderr)
    );
    assert!(output.stderr.is_empty());
    let value = common::envelope(output);
    assert_eq!(value["ok"], true);
    assert_eq!(value["kind"], kind);
    assert!(value["message"].is_string());
    assert!(value.get("data").is_some());
    value
}

fn refusal(output: &Output, detail: &str) -> Value {
    assert!(!output.status.success());
    assert!(output.stdout.is_empty());
    let value = common::error_envelope(output);
    assert_eq!(value["ok"], false);
    assert!(value["error"]["code"].is_string());
    assert!(
        value["error"]["detail"]
            .as_str()
            .expect("error detail")
            .contains(detail),
        "{value}"
    );
    value
}

fn emulator(root: &Path) -> (Process, String) {
    let listener = TcpListener::bind("127.0.0.1:0").expect("loopback port");
    let address = listener.local_addr().expect("address");
    drop(listener);
    let endpoint = format!("http://{address}");
    let child = command(root)
        .args([
            "emulator",
            "up",
            "--listen",
            &address.to_string(),
            "--sequencer-seed-file",
        ])
        .arg(root.join("sequencer.seed"))
        .stdout(Stdio::null())
        .stderr(Stdio::piped())
        .spawn()
        .expect("real core emulator starts");
    let mut process = Process(child);
    let deadline = Instant::now() + Duration::from_secs(20);
    loop {
        if let Ok(response) = ureq::get(&format!("{endpoint}/healthz")).call() {
            if response.status().is_success() {
                break;
            }
        }
        assert!(
            process.0.try_wait().expect("process status").is_none(),
            "emulator exited"
        );
        assert!(Instant::now() < deadline, "emulator readiness deadline");
        std::thread::sleep(Duration::from_millis(25));
    }
    success(
        &run(
            root,
            &[
                "environment",
                "use",
                "emulator",
                "--endpoint",
                &endpoint,
                "--network-id",
                "402",
                "--sequencer-trust-anchor",
                &common::emulator_trust_anchor(),
            ],
        ),
        "environment.selected",
    );
    (process, endpoint)
}

fn consume_python(output: &Output, kind: &str, expected: Option<&Value>) {
    let script = r#"
import json, sys
from pathlib import Path
from tools.qualification.beta_driver import Execution, cli_output, CaseFailure
text = sys.stdin.read()
execution = Execution('real CLI', [], Path.cwd(), int(sys.argv[2]), Path('/dev/null'), text, 0)
try:
    data = cli_output(execution, sys.argv[1])
except CaseFailure:
    sys.exit(2)
print(json.dumps(data))
"#;
    let mut child = Command::new("python3")
        .args([
            "-B",
            "-c",
            script,
            kind,
            &output.status.code().expect("exit code").to_string(),
        ])
        .current_dir(Path::new(env!("CARGO_MANIFEST_DIR")).join("../../.."))
        .stdin(Stdio::piped())
        .stdout(Stdio::piped())
        .stderr(Stdio::piped())
        .spawn()
        .expect("qualification consumer starts");
    child
        .stdin
        .take()
        .expect("stdin")
        .write_all(&output.stdout)
        .expect("producer output");
    let observed = child.wait_with_output().expect("consumer exits");
    match expected {
        Some(expected) => {
            assert!(
                observed.status.success(),
                "{}",
                String::from_utf8_lossy(&observed.stderr)
            );
            assert_eq!(
                serde_json::from_slice::<Value>(&observed.stdout).expect("consumer JSON"),
                *expected
            );
        }
        None => assert_eq!(observed.status.code(), Some(2)),
    }
}

fn consume_shell(output: &Output, kind: &str, expected_success: bool) {
    let mut child = Command::new("bash")
        .arg(Path::new(env!("CARGO_MANIFEST_DIR")).join("tests/install-journey.sh"))
        .args(["--check-cli-output", kind])
        .stdin(Stdio::piped())
        .stdout(Stdio::piped())
        .stderr(Stdio::piped())
        .spawn()
        .expect("installation consumer starts");
    let bytes = if output.status.success() {
        &output.stdout
    } else {
        &output.stderr
    };
    child
        .stdin
        .take()
        .expect("stdin")
        .write_all(bytes)
        .expect("producer output");
    let observed = child.wait_with_output().expect("consumer exits");
    assert_eq!(
        observed.status.success(),
        expected_success,
        "{}",
        String::from_utf8_lossy(&observed.stderr)
    );
}

#[test]
#[allow(non_snake_case)]
fn Call() -> Result<(), Box<dyn std::error::Error>> {
    let temp = tempfile::tempdir()?;
    let root = temp.path();
    {
        use std::os::unix::fs::OpenOptionsExt as _;
        let mut seed = std::fs::OpenOptions::new()
            .write(true)
            .create_new(true)
            .mode(0o600)
            .open(root.join("sequencer.seed"))?;
        seed.write_all(&[0x42; 32])?;
        seed.sync_all()?;
    }
    let (process, endpoint) = emulator(root);
    success(
        &run(
            root,
            &["wallet", "create", "alice", "--did", "did:layerx:alice"],
        ),
        "wallet.created",
    );
    let listed = run(root, &["wallet", "list"]);
    let keys = success(&listed, "key.list");
    assert_eq!(keys["data"].as_array().expect("keys").len(), 1);
    let public_key = keys["data"][0]["public_key"].as_str().expect("public key");
    assert_eq!(public_key.len(), 64);
    assert_eq!(keys["data"][0]["did"], "did:layerx:alice");
    consume_shell(&listed, "key.list", true);
    let balances = run(root, &["wallet", "balance"]);
    let before = success(&balances, "wallet.balance");
    assert_eq!(
        before["data"]["accounts"]
            .as_array()
            .expect("accounts")
            .len(),
        1
    );
    assert_eq!(
        before["data"]["accounts"][0]["name"],
        "agent:did:layerx:alice:main"
    );
    consume_python(&balances, "wallet.balance", Some(&before["data"]));
    consume_shell(&balances, "wallet.balance", true);
    consume_shell(&balances, "receipt.read", false);

    // The recovery bytes come from the running producer, never a constructed snapshot.
    let snapshot = ureq::get(&format!("{endpoint}/__emulator/snapshot"))
        .call()?
        .body_mut()
        .read_to_vec()?;
    assert!(!snapshot.is_empty());
    let state: Value = ureq::get(&format!("{endpoint}/v1/state"))
        .call()?
        .body_mut()
        .read_json()?;
    std::fs::write(root.join("recovery.snapshot"), &snapshot)?;
    drop(process);
    let (_reopened, endpoint) = emulator(root);
    let empty: Value = ureq::get(&format!("{endpoint}/v1/state"))
        .call()?
        .body_mut()
        .read_json()?;
    assert_eq!(empty["result"]["accounts"], json!([]));
    let response = ureq::put(&format!("{endpoint}/__emulator/snapshot"))
        .header("Content-Type", "application/vnd.layerx.emulator-snapshot")
        .send(std::fs::read(root.join("recovery.snapshot"))?)?;
    assert!(response.status().is_success());
    let restored: Value = ureq::get(&format!("{endpoint}/v1/state"))
        .call()?
        .body_mut()
        .read_json()?;
    assert_eq!(restored["result"], state["result"]);
    let after_output = run(root, &["wallet", "balance"]);
    let after = success(&after_output, "wallet.balance");
    assert_eq!(after["data"], before["data"]);
    assert_eq!(
        success(&run(root, &["wallet", "list"]), "key.list")["data"],
        keys["data"]
    );
    consume_python(&after_output, "wallet.balance", Some(&before["data"]));
    consume_shell(&after_output, "wallet.balance", true);
    refusal(
        &run(
            root,
            &["wallet", "create", "alice", "--did", "did:layerx:alice"],
        ),
        "wallet registration failed",
    );
    assert_eq!(
        success(&run(root, &["wallet", "balance"]), "wallet.balance")["data"],
        before["data"]
    );
    assert_eq!(
        success(&run(root, &["wallet", "list"]), "key.list")["data"],
        keys["data"]
    );

    // Derivation emits the real owner-only EVM export consumed by ProgramCommand::Call.
    let export = root.join("evm.json");
    let mut derive = command(root)
        .args(["wallet", "derive", "--export-private-keys"])
        .arg(&export)
        .stdin(Stdio::piped())
        .stdout(Stdio::piped())
        .stderr(Stdio::piped())
        .spawn()?;
    derive.stdin.take().ok_or("derive stdin")?.write_all(
        b"abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about\n",
    )?;
    success(&derive.wait_with_output()?, "wallet.derived");
    let id = "11".repeat(32);
    let hash = format!("0x{}", "22".repeat(32));
    let rpc = format!("{endpoint}/rpc");
    for args in [
        vec!["receipt", "get", hash.as_str()],
        vec!["program", "discover", id.as_str()],
        vec!["program", "registry", "get", id.as_str()],
        vec!["program", "registry", "list"],
        vec![
            "program",
            "call",
            id.as_str(),
            "--fuel",
            "100000",
            "--evm-key-file",
            export.to_str().ok_or("export path")?,
        ],
    ] {
        let mut arguments = vec!["--rpc", rpc.as_str()];
        arguments.extend(args);
        let observed = run(root, &arguments);
        refusal(&observed, "404");
        consume_shell(&observed, "wallet.balance", false);
        consume_python(&observed, "wallet.balance", None);
    }

    let agent: ureq::Agent = ureq::Agent::config_builder()
        .http_status_as_error(false)
        .build()
        .into();
    assert_eq!(
        layerx_platform_cli::rpc::request("px_listAssets", &json!([null, 1]))
            .expect_err("asset listing uses the SDK, not the CLI request builder"),
        "rpc_method_unavailable: px_listAssets is absent from the published OpenRPC contract"
    );
    let request = layerx_platform_cli::rpc::request("pax_sequencerReceipt", &json!([hash]))?;
    let absent_rpc = agent.post(&rpc).send_json(&request)?;
    assert_eq!(absent_rpc.status().as_u16(), 404);
    let tokens = run(root, &["--rpc", &rpc, "token", "list", "--limit", "1"]);
    assert_eq!(
        refusal(&tokens, "InvalidResponse"),
        json!({
            "ok": false,
            "error": {
                "code": "rpc_method_unavailable",
                "detail": "rpc_method_unavailable: wallet RPC failed: InvalidResponse"
            }
        })
    );
    consume_shell(&tokens, "token.read", false);
    consume_python(&tokens, "token.read", None);

    // Consumer toolset must make the same surviving RPC read as receipt get.
    let runtime = toolset::Runtime::loopback(&endpoint, 402)?;
    let tool = toolset::surface(layerx_mcp::server::DeploymentMode::ReadOnly)?[0];
    assert_eq!(tool.name, "receipt.get");
    assert_eq!(
        toolset::schema(tool.name)["properties"]["activity_id"]["pattern"],
        "^0x[0-9a-fA-F]{64}$"
    );
    let detail = toolset::invoke(&runtime, tool, &json!({"activity_id":hash}))
        .expect_err("no emulator RPC route");
    assert!(detail.contains("POST /rpc returned HTTP 404"), "{detail}");
    assert!(toolset::invoke(&runtime, tool, &json!({"activity_id":id}))
        .expect_err("unprefixed hash")
        .contains("0x-prefixed"));
    assert!(toolset::invoke(&runtime, tool, &json!({"activity_id":hash,"extra":true})).is_err());

    for args in [
        vec!["receipt", "get", "bad id"],
        vec!["program", "discover", "bad id"],
        vec!["program", "registry", "get", "bad id"],
        vec![
            "program",
            "call",
            "bad id",
            "--fuel",
            "100000",
            "--evm-key-file",
            export.to_str().ok_or("export path")?,
        ],
        vec!["token", "list", "--cursor", "bad id"],
        vec!["wallet", "balance", "--asset", "bad id"],
    ] {
        refusal(&run(root, &args), "hex");
    }
    let invalid = run(root, &["token", "list", "--limit", "0"]);
    assert_eq!(invalid.status.code(), Some(2));
    assert_eq!(
        refusal(&invalid, "--limit")["error"]["code"],
        "invalid_arguments"
    );
    let missing = root.join("mcp/binding.json");
    refusal(
        &run(
            root,
            &[
                "mcp",
                "serve",
                "--daemon-binding",
                missing.to_str().ok_or("binding path")?,
            ],
        ),
        "unreadable",
    );
    std::fs::create_dir_all(missing.parent().ok_or("binding parent")?)?;
    std::fs::write(&missing, b"{")?;
    refusal(
        &run(
            root,
            &[
                "mcp",
                "serve",
                "--daemon-binding",
                missing.to_str().ok_or("binding path")?,
            ],
        ),
        "binding",
    );
    refusal(
        &run(root, &["install", "mcp", "--host", "layerx"]),
        "binding",
    );
    assert!(!root.join("mcp.json").exists());
    assert_eq!(
        success(&run(root, &["wallet", "balance"]), "wallet.balance")["data"],
        before["data"]
    );
    Ok(())
}
