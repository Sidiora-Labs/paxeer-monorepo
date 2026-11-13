//! The emulator serves head and program routes through its live HTTP surface.

use std::error::Error;
use std::io::{Read, Write};
use std::net::{TcpListener, TcpStream};
use std::path::PathBuf;
use std::sync::atomic::{AtomicBool, AtomicU64, Ordering};
use std::sync::Arc;
use std::thread::{self, JoinHandle};
use std::time::{Duration, Instant};

use serde_json::{json, Value};

type Result<T> = std::result::Result<T, Box<dyn Error>>;
static NEXT_SEED: AtomicU64 = AtomicU64::new(0);

struct Server {
    address: String,
    shutdown: Arc<AtomicBool>,
    worker: Option<JoinHandle<std::result::Result<(), String>>>,
    seed: PathBuf,
}

impl Server {
    fn start(protocol: u16) -> Result<Self> {
        let listener = TcpListener::bind("127.0.0.1:0")?;
        let address = listener.local_addr()?.to_string();
        let seed = std::env::temp_dir().join(format!(
            "emulator-22-33-1435-{}-{}.seed",
            std::process::id(),
            NEXT_SEED.fetch_add(1, Ordering::Relaxed)
        ));
        std::fs::write(&seed, [0x42; 32])?;
        drop(listener);
        let arguments = vec![
            "up".to_owned(),
            "--listen".to_owned(),
            address.clone(),
            "--sequencer-seed-file".to_owned(),
            seed.to_string_lossy().into_owned(),
            "--protocol-version".to_owned(),
            protocol.to_string(),
        ];
        let shutdown = Arc::new(AtomicBool::new(false));
        let stop = Arc::clone(&shutdown);
        let worker = thread::Builder::new()
            .stack_size(8 * 1024 * 1024)
            .spawn(move || layerx_platform_emulator::run_until(arguments, stop))?;
        let server = Self {
            address,
            shutdown,
            worker: Some(worker),
            seed,
        };
        let deadline = Instant::now() + Duration::from_secs(15);
        loop {
            if let Ok((200, _)) = server.request("GET", "/healthz", &[]) {
                return Ok(server);
            }
            if server
                .worker
                .as_ref()
                .ok_or("missing worker")?
                .is_finished()
            {
                return Err("emulator stopped before readiness".into());
            }
            if Instant::now() >= deadline {
                return Err("emulator readiness timed out".into());
            }
            thread::sleep(Duration::from_millis(10));
        }
    }

    fn request(&self, method: &str, path: &str, body: &[u8]) -> Result<(u16, Vec<u8>)> {
        let mut stream = TcpStream::connect(&self.address)?;
        stream.set_read_timeout(Some(Duration::from_secs(10)))?;
        stream.set_write_timeout(Some(Duration::from_secs(10)))?;
        write!(stream, "{method} {path} HTTP/1.1\r\nHost: {}\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", self.address, body.len())?;
        stream.write_all(body)?;
        let mut response = Vec::new();
        stream.read_to_end(&mut response)?;
        let split = response
            .windows(4)
            .position(|part| part == b"\r\n\r\n")
            .ok_or("missing header terminator")?;
        let headers = std::str::from_utf8(&response[..split])?;
        let status = headers
            .split_whitespace()
            .nth(1)
            .ok_or("missing HTTP status")?
            .parse()?;
        assert!(headers.contains("Cache-Control: no-store"));
        Ok((status, response[split + 4..].to_vec()))
    }

    fn json(&self, method: &str, path: &str, body: &Value, status: u16) -> Result<Value> {
        let bytes = if matches!(method, "POST" | "PUT") {
            serde_json::to_vec(body)?
        } else {
            Vec::new()
        };
        let (actual, response) = self.request(method, path, &bytes)?;
        assert_eq!(
            actual,
            status,
            "{method} {path}: {}",
            String::from_utf8_lossy(&response)
        );
        Ok(serde_json::from_slice(&response)?)
    }

    fn stop(&mut self) -> Result<()> {
        self.shutdown.store(true, Ordering::Release);
        if let Some(worker) = self.worker.take() {
            worker.join().map_err(|_| "emulator worker panicked")??;
        }
        Ok(())
    }
}

impl Drop for Server {
    fn drop(&mut self) {
        let _ = self.stop();
        let _ = std::fs::remove_file(&self.seed);
    }
}

fn hex(bytes: &[u8]) -> String {
    const DIGITS: &[u8; 16] = b"0123456789abcdef";
    let mut encoded = String::with_capacity(bytes.len() * 2);
    for byte in bytes {
        encoded.push(char::from(DIGITS[usize::from(byte >> 4)]));
        encoded.push(char::from(DIGITS[usize::from(byte & 0x0f)]));
    }
    encoded
}

fn discovery(server: &Server) -> Result<Value> {
    let health = server.json("GET", "/healthz", &Value::Null, 200)?;
    assert_eq!(health["result"]["surface"], "/v1/surface");
    assert_eq!(health["result"]["payments_supported"], false);
    let surface = server.json("GET", "/v1/surface", &Value::Null, 200)?["result"].clone();
    assert_eq!(surface["payments"]["supported"], false);
    assert_eq!(
        surface["payments"]["replacement"]["transport"],
        "merged_node_json_rpc"
    );
    assert_eq!(
        surface["payments"]["replacement"]["submit"],
        "eth_sendRawTransaction"
    );
    assert_eq!(
        surface["payments"]["replacement"]["receipt"],
        "pax_sequencerReceipt"
    );
    assert_eq!(surface["historical_imports"]["mode"], "inspection_only");
    assert_eq!(
        surface["historical_imports"]["produces_new_payments"],
        false
    );
    assert_eq!(
        surface["historical_imports"]["establishes_merged_finality"],
        false
    );
    assert_eq!(surface["retired_ingress"]["http_status"], 404);
    assert_eq!(
        surface["retired_ingress"]["replacement"],
        "merged-node transactions via eth_sendRawTransaction"
    );
    let retired = surface["retired_ingress"]["paths"]
        .as_array()
        .ok_or("missing retired ingress")?;
    for path in [
        "/v1/activities",
        "/v1/moves",
        "/v1/moves/quote",
        "/v1/programs",
        "/v1/programs/*",
    ] {
        assert!(retired.contains(&json!(path)), "missing retirement: {path}");
    }
    let routes = surface["routes"]
        .as_array()
        .ok_or("missing supported routes")?;
    assert_eq!(routes.len(), 15);
    for route in routes {
        let path = route["path"].as_str().ok_or("missing route path")?;
        let method = route["method"].as_str().ok_or("missing route method")?;
        assert!(!path.starts_with("/v1/activities"));
        assert!(!path.starts_with("/v1/moves"));
        assert!(!path.starts_with("/v1/programs"));
        assert!(
            method == "GET" || path.starts_with("/__emulator/"),
            "execution ingress advertised: {route}"
        );
    }
    Ok(surface)
}

fn assert_head_unavailable(server: &Server) -> Result<()> {
    let head = server.json("GET", "/v1/protocol/account-state/head", &Value::Null, 503)?;
    assert_eq!(head["ok"], false);
    assert_eq!(head["error"]["code"], "principal_state_proof_unavailable");
    assert!(
        head.get("result").is_none(),
        "unavailable head must not carry successful evidence"
    );
    Ok(())
}

fn recorded_transaction(id: &str) -> Result<Value> {
    let transcript: Value = serde_json::from_str(include_str!(
        "../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
    ))?;
    assert_eq!(
        transcript["provenance"]["generator"],
        "rpc/sequencer_receipt_vectors_test.go"
    );
    let case = transcript["cases"]
        .as_array()
        .ok_or("missing recorded cases")?
        .iter()
        .find(|case| case["id"] == id)
        .ok_or("missing recorded transaction")?;
    assert!(case["rawTransaction"]
        .as_str()
        .ok_or("missing raw transaction")?
        .starts_with("0x02"));
    assert_eq!(case["receiptJson"]["status"], "0x1");
    Ok(case.clone())
}

fn assert_transaction_refused(server: &Server, case: &Value) -> Result<()> {
    let submission = json!({"jsonrpc":"2.0", "id":1, "method":"eth_sendRawTransaction", "params":[case["rawTransaction"]]});
    let refused = server.json("POST", "/", &submission, 404)?;
    assert_eq!(refused["ok"], false);
    assert_eq!(refused["error"]["code"], "not_found");
    assert!(refused.get("result").is_none());
    let tx_hash = case["receiptJson"]["txHash"]
        .as_str()
        .ok_or("missing transaction hash")?;
    let receipt = server.json(
        "GET",
        &format!("/v1/receipts/{}", tx_hash.trim_start_matches("0x")),
        &Value::Null,
        404,
    )?;
    assert_eq!(receipt["error"]["code"], "not_found");
    assert!(receipt.get("result").is_none());
    Ok(())
}

#[test]
fn emulator_head_proof_coverage_retires_local_transfer_promise_without_fabricating_head(
) -> Result<()> {
    let transfer = recorded_transaction("V1-layerxledger-pay-exact")?;
    let public = ed25519_dalek::SigningKey::from_bytes(&[0x42; 32])
        .verifying_key()
        .to_bytes();
    for protocol in [2, 3] {
        let mut server = Server::start(protocol)?;
        let surface = discovery(&server)?;
        let initial_state = server.json("GET", "/v1/state", &Value::Null, 200)?["result"].clone();
        let initial_accounts = initial_state["accounts"]
            .as_array()
            .ok_or("missing genesis accounts")?;
        assert_head_unavailable(&server)?;
        let identity = server.json("GET", "/v1/sequencer", &Value::Null, 200)?["result"].clone();
        assert_eq!(identity["sequencer_public_key"], hex(&public));
        assert_eq!(
            identity["sequencer_id"],
            hex(&<sha2::Sha256 as sha2::Digest>::digest(public))
        );
        for (did, amount) in [("source", 1000), ("destination", 0)] {
            let response = server.json("POST", "/__emulator/accounts/prefund", &json!({
                "did": format!("did:layerx:head-{did}"), "public_key": hex(&public), "amount_lo": amount
            }), 200)?;
            assert_eq!(response["result"]["prefunded"], true);
        }
        let state = server.json("GET", "/v1/state", &Value::Null, 200)?["result"].clone();
        let accounts = state["accounts"]
            .as_array()
            .ok_or("missing funded accounts")?;
        assert_eq!(accounts.len(), initial_accounts.len() + 2);
        for genesis_account in initial_accounts {
            let preserved = accounts
                .iter()
                .find(|account| account["id"] == genesis_account["id"])
                .ok_or("genesis account missing after prefund")?;
            assert_eq!(preserved, genesis_account);
        }
        for (did, amount) in [("source", 1000), ("destination", 0)] {
            let name = format!("agent:did:layerx:head-{did}:main");
            let account = accounts
                .iter()
                .find(|account| account["name"] == name)
                .ok_or("prefunded account missing")?;
            assert_eq!(account["balance_hi"], 0);
            assert_eq!(account["balance_lo"], amount);
        }
        assert_head_unavailable(&server)?;
        for path in ["/v1/activities", "/v1/moves", "/v1/moves/quote"] {
            let refused = server.json("POST", path, &transfer, 404)?;
            assert_eq!(refused["error"]["code"], "not_found");
            assert!(refused.get("result").is_none());
        }
        assert_transaction_refused(&server, &transfer)?;
        assert_head_unavailable(&server)?;
        assert_eq!(
            server.json("GET", "/v1/state", &Value::Null, 200)?["result"],
            state
        );

        let (status, snapshot) = server.request("GET", "/__emulator/snapshot", &[])?;
        assert_eq!(status, 200);
        assert!(!snapshot.is_empty());
        let address = server.address.clone();
        server.stop()?;
        let released = TcpListener::bind(address)?;
        drop(released);
        drop(server);
        let server = Server::start(protocol)?;
        assert_eq!(
            server.json("GET", "/v1/state", &Value::Null, 200)?["result"],
            initial_state
        );
        let (status, imported) = server.request("PUT", "/__emulator/snapshot", &snapshot)?;
        assert_eq!(status, 200);
        assert_eq!(
            serde_json::from_slice::<Value>(&imported)?["result"]["imported"],
            true
        );
        assert_eq!(
            server.json("GET", "/v1/state", &Value::Null, 200)?["result"],
            state
        );
        assert_eq!(
            server.json("GET", "/v1/sequencer", &Value::Null, 200)?["result"],
            identity
        );
        assert_eq!(discovery(&server)?, surface);
        assert_head_unavailable(&server)?;
        assert_transaction_refused(&server, &transfer)?;
        assert_eq!(
            server.json("GET", "/v1/state", &Value::Null, 200)?["result"],
            state
        );
    }
    Ok(())
}

#[test]
fn emulator_program_path_coverage_retires_local_lifecycle_echo_and_keeps_8323_refusals(
) -> Result<()> {
    let program_call = recorded_transaction("V1-layerxprograms-call")?;
    for protocol in [2, 3] {
        let server = Server::start(protocol)?;
        discovery(&server)?;
        let state = server.json("GET", "/v1/state", &Value::Null, 200)?["result"].clone();
        let id = "a".repeat(64);
        let routes = [
            "/v1/programs".to_owned(),
            "/v1/programs/read".to_owned(),
            "/v1/programs/call".to_owned(),
            "/v1/programs/deploy".to_owned(),
            "/v1/programs/upgrade".to_owned(),
            "/v1/programs/wind-down".to_owned(),
            "/v1/programs/simulate".to_owned(),
            "/v1/programs/registry".to_owned(),
            format!("/v1/programs/registry/{id}/source"),
            format!("/v1/programs/registry/{id}"),
            format!("/v1/programs/registry/{id}/interface"),
            format!("/v1/programs/receipts/by-idempotency/{id}"),
            format!("/v1/programs/activities/{id}"),
            "/v1/programs/unknown".to_owned(),
        ];
        for path in routes {
            for method in ["GET", "POST", "PUT", "DELETE"] {
                let refused = server.json(method, &path, &program_call, 404)?;
                assert_eq!(refused["ok"], false, "{method} {path}");
                assert_eq!(refused["error"]["code"], "not_found", "{method} {path}");
                assert!(refused.get("result").is_none(), "{method} {path}");
            }
        }
        assert_transaction_refused(&server, &program_call)?;
        assert_eq!(
            server.json("GET", "/v1/state", &Value::Null, 200)?["result"],
            state
        );
        assert_head_unavailable(&server)?;
        assert_eq!(
            server.json("GET", "/healthz", &Value::Null, 200)?["result"]["status"],
            "ready"
        );
    }
    Ok(())
}
