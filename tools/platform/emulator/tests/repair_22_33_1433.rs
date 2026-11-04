//! Real listener coverage for the retired payment surface and its discovery.

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
            "emulator-22-33-1433-{}-{}.seed",
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

fn surface(server: &Server) -> Result<Value> {
    let health = server.json("GET", "/healthz", &Value::Null, 200)?;
    assert_eq!(health["result"]["surface"], "/v1/surface");
    assert_eq!(health["result"]["payments_supported"], false);
    let response = server.json("GET", "/v1/surface", &Value::Null, 200)?;
    let surface = response["result"].clone();
    assert_eq!(surface["payments"]["supported"], false);
    assert_eq!(surface["payments"]["settlement"], "retired");
    assert_eq!(surface["payments"]["settlement_http_status"], 410);
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
    assert_eq!(
        surface["payments"]["replacement"]["authority"],
        "ReceiptPayloadAuthority"
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
    let routes = surface["routes"]
        .as_array()
        .ok_or("missing supported routes")?;
    assert_eq!(routes.len(), 15);
    for route in routes {
        let path = route["path"].as_str().ok_or("missing route path")?;
        assert!(!path.starts_with("/v1/activities"));
        assert!(!path.starts_with("/v1/moves"));
        assert!(!path.starts_with("/v1/programs"));
        assert_ne!(path, "/v1/settle");
    }
    Ok(surface)
}

fn assert_settlement_retired(server: &Server, payment: &Value) -> Result<()> {
    for body in [serde_json::to_vec(payment)?, b"not-json".to_vec()] {
        let (status, response) = server.request("POST", "/v1/settle", &body)?;
        assert_eq!(status, 410);
        let response: Value = serde_json::from_slice(&response)?;
        assert_eq!(response["ok"], false);
        assert_eq!(response["error"]["code"], "settlement_authority_retired");
        assert_eq!(response["error"]["retry"], "never");
        assert!(response.get("result").is_none());
    }
    Ok(())
}

#[test]
fn emulator_settlement_receipt_wiring_declares_retirement_after_payment_attempt_and_reopen(
) -> Result<()> {
    // Recorded transaction and signed receipt from the real merged-node producer.
    let transcript: Value = serde_json::from_str(include_str!(
        "../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
    ))?;
    assert_eq!(
        transcript["provenance"]["generator"],
        "rpc/sequencer_receipt_vectors_test.go"
    );
    let payment = transcript["cases"]
        .as_array()
        .ok_or("missing vectors")?
        .iter()
        .find(|case| case["id"] == "V1-layerxledger-pay-exact")
        .ok_or("missing payment vector")?;
    assert!(payment["rawTransaction"]
        .as_str()
        .ok_or("missing transaction")?
        .starts_with("0x02"));
    assert_eq!(payment["receiptJson"]["status"], "0x1");
    for protocol in [2, 3] {
        let mut server = Server::start(protocol)?;
        let before = surface(&server)?;
        server.json("POST", "/__emulator/accounts/prefund", &json!({
            "did": "did:layerx:retirement-payer", "public_key": "11".repeat(32), "amount_lo": 731
        }), 200)?;
        let state = server.json("GET", "/v1/state", &Value::Null, 200)?["result"].clone();
        let submission = json!({"jsonrpc":"2.0", "id":1, "method":"eth_sendRawTransaction", "params":[payment["rawTransaction"]]});
        let refused = server.json("POST", "/", &submission, 404)?;
        assert_eq!(refused["error"]["code"], "not_found");
        assert_settlement_retired(&server, payment)?;
        assert_eq!(
            server.json("GET", "/v1/state", &Value::Null, 200)?["result"],
            state
        );
        let (status, snapshot) = server.request("GET", "/__emulator/snapshot", &[])?;
        assert_eq!(status, 200);
        assert!(!snapshot.is_empty());
        // Persist the real export, stop the listener and destroy its core.
        let snapshot_path = server.seed.with_extension("snapshot");
        std::fs::write(&snapshot_path, &snapshot)?;
        let old_address = server.address.clone();
        server.stop()?;
        let listener = TcpListener::bind(&old_address)?;
        drop(listener);
        drop(server);
        let server = Server::start(protocol)?;
        let (status, _) = server.request(
            "PUT",
            "/__emulator/snapshot",
            &std::fs::read(&snapshot_path)?,
        )?;
        std::fs::remove_file(snapshot_path)?;
        assert_eq!(status, 200);
        assert_eq!(
            server.json("GET", "/v1/state", &Value::Null, 200)?["result"],
            state
        );
        assert_eq!(surface(&server)?, before);
        assert_settlement_retired(&server, payment)?;
        let missing = server.json(
            "GET",
            &format!("/v1/receipts/{}", "a".repeat(64)),
            &Value::Null,
            404,
        )?;
        assert_eq!(missing["error"]["code"], "not_found");
    }
    Ok(())
}

#[test]
fn emulator_retired_route_registry_publishes_only_supported_inspection_routes() -> Result<()> {
    for protocol in [2, 3] {
        let server = Server::start(protocol)?;
        let discovery = surface(&server)?;
        assert_eq!(discovery["retired_ingress"]["http_status"], 404);
        let id = "a".repeat(64);
        let paths = [
            "/v1/activities".to_owned(),
            "/v1/moves".to_owned(),
            "/v1/moves/quote".to_owned(),
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
        for path in paths {
            for method in ["GET", "POST", "PUT", "DELETE"] {
                let response = server.json(method, &path, &Value::Null, 404)?;
                assert_eq!(response["error"]["code"], "not_found", "{method} {path}");
            }
        }
        assert_eq!(
            server.json("POST", "/v1/surface", &json!({}), 405)?["error"]["code"],
            "method_not_allowed"
        );
        for path in ["/v1/state", "/v1/sequencer", "/__emulator/snapshot"] {
            assert_eq!(server.request("GET", path, &[])?.0, 200, "{path}");
        }
    }
    Ok(())
}
