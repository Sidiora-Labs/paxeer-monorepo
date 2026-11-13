//! Task record 22.33.1493 / audit 22.30.89 / acceptance 22998.1.
//! `retired_endpoint_programs` -> `program_submission_recovery` (root deployment and receipt).
//! `program_submission_recovery` -> `program_submission_recovery` (stop/reopen after dropped reply).
//! `retired_endpoint_register` -> `retired_endpoint_register` (signed hosted registration).
//! `retired_endpoint_rpc` -> `retired_endpoint_rpc` (separate node/service routes and authorization).
//! `retired_endpoint_subscription` -> `retired_endpoint_subscription` (real WebSocket handshake).
//! `scaffold_build_host_path` -> `scaffold_build_host_path` (relocated built CLI and local SDK).
//! `retired_endpoint_toolset` -> `retired_endpoint_toolset` (node receipts and hosted faucet).
//! `tool_submit_recovery` -> `tool_submit_recovery` (both invocations stop/reopen and reconcile).
//! Baseline: not run: the ordered gate instructions forbid tests before implementation;
//! this integration target did not exist on the base. No baseline failure is claimed.
//! Prior attempt: 57/58 cases passed; only `register::tests::the_request_names_the_method_and_params_the_gateway_serves`
//! failed by requiring two schema parameters despite the gateway's optional third parameter (gotcha 3182).
//! Wiring request: tools/platform/cli/src/main.rs, domain 8: update `program_client` and RPC
//! dispatch/help to accept node roots and select `RpcClient::new_gateway` for explicit px_ service URLs.
//! Wiring request: tools/platform/cli/src/config.rs, domain 8: persist a separate hosted service
//! URL; tool faucet presently requires explicit `LAYERX_GATEWAY_RPC_URL`, registration --gateway-endpoint.
//! Evidence: hf-build/hf-done retain revision, exact command, exit and private log location.

pub use layerx_platform_cli::{config, credential, encoding, http, output, rpc};
#[path = "../src/faucet.rs"]
#[allow(dead_code)]
mod faucet;
#[path = "../src/programs.rs"]
#[allow(dead_code)]
mod programs;
#[path = "../src/register.rs"]
#[allow(dead_code)]
mod register;
#[path = "../src/scaffold.rs"]
#[allow(dead_code)]
mod scaffold;
#[path = "../src/toolset.rs"]
#[allow(dead_code)]
mod toolset;

use std::io::{BufRead as _, BufReader, Read as _, Write as _};
use std::net::{Shutdown, TcpListener, TcpStream};
use std::path::Path;
use std::thread::{self, JoinHandle};
use std::time::{Duration, Instant};

use ed25519_dalek::SigningKey;
use layerx_crypto::evm_transaction::{
    decode_signed, recover_sender, transaction_hash, Eip1559Call,
};
use layerx_mcp::server::{DeploymentMode, ToolDefinition};
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};
use zeroize::Zeroizing;

fn credential_value() -> Zeroizing<String> {
    Zeroizing::new(format!("testkey:lxp_live_{}", "a".repeat(64)))
}

fn accept(listener: &TcpListener) -> TcpStream {
    listener.set_nonblocking(true).unwrap_or_else(panic_error);
    let deadline = Instant::now() + Duration::from_secs(10);
    loop {
        match listener.accept() {
            Ok((stream, _)) => {
                stream
                    .set_read_timeout(Some(Duration::from_secs(5)))
                    .unwrap_or_else(panic_error);
                stream
                    .set_write_timeout(Some(Duration::from_secs(5)))
                    .unwrap_or_else(panic_error);
                return stream;
            }
            Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                assert!(
                    Instant::now() < deadline,
                    "loopback peer received no request"
                );
                thread::sleep(Duration::from_millis(5));
            }
            Err(error) => panic!("loopback accept: {error}"),
        }
    }
}

// Committed loopback peer: production clients perform all request encoding and transport.
fn peer(
    path: &str,
    authorization: Option<String>,
    handle: impl FnOnce(Value) -> Option<Value> + Send + 'static,
) -> (String, JoinHandle<()>) {
    let listener = TcpListener::bind("127.0.0.1:0").unwrap_or_else(panic_error);
    let endpoint = format!(
        "http://{}",
        listener.local_addr().unwrap_or_else(panic_error)
    );
    let expected_path = path.to_owned();
    let worker = thread::spawn(move || {
        let mut reader = BufReader::new(accept(&listener));
        let mut line = String::new();
        reader.read_line(&mut line).unwrap_or_else(panic_error);
        assert_eq!(line, format!("POST {expected_path} HTTP/1.1\r\n"));
        let mut length = None;
        let mut actual_authorization = None;
        loop {
            line.clear();
            assert_ne!(reader.read_line(&mut line).unwrap_or_else(panic_error), 0);
            if line == "\r\n" {
                break;
            }
            let (name, value) = line.split_once(':').unwrap_or_else(panic_missing);
            if name.eq_ignore_ascii_case("content-length") {
                assert!(length.is_none());
                length = Some(value.trim().parse::<usize>().unwrap_or_else(panic_error));
            }
            if name.eq_ignore_ascii_case("authorization") {
                actual_authorization = Some(value.trim().to_owned());
            }
        }
        assert_eq!(actual_authorization, authorization);
        let length = length.unwrap_or_else(panic_missing);
        assert!(length <= 2 * 1024 * 1024);
        let mut body = vec![0; length];
        reader.read_exact(&mut body).unwrap_or_else(panic_error);
        let request: Value = serde_json::from_slice(&body).unwrap_or_else(panic_error);
        assert_eq!(request["jsonrpc"], "2.0");
        if let Some(response) = handle(request) {
            let body = serde_json::to_vec(&response).unwrap_or_else(panic_error);
            write!(reader.get_mut(), "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", body.len()).unwrap_or_else(panic_error);
            reader
                .get_mut()
                .write_all(&body)
                .unwrap_or_else(panic_error);
        }
        reader
            .get_ref()
            .shutdown(Shutdown::Both)
            .unwrap_or_else(panic_error);
        // A write must never be automatically retried after an ambiguous response.
        assert!(
            matches!(listener.accept(), Err(error) if error.kind() == std::io::ErrorKind::WouldBlock)
        );
    });
    (endpoint, worker)
}

fn reply(request: &Value, result: &Value) -> Value {
    json!({"jsonrpc":"2.0","id":request["id"],"result":result})
}

fn signed_deployment() -> Vec<u8> {
    let fixture: Value = serde_json::from_str(include_str!(
        "../../../../sdk/platform/conformance/fixtures/native-program-deploy-v4.json"
    ))
    .unwrap_or_else(panic_error);
    let payload = encoding::hex_decode(
        "deployment",
        fixture["payload_hex"]
            .as_str()
            .unwrap_or_else(panic_missing),
    )
    .unwrap_or_else(panic_error);
    let data = programs::lifecycle_calldata(1, &payload, "0").unwrap_or_else(panic_error);
    assert_eq!(
        format!("0x{}", encoding::hex_encode(&data)),
        fixture["evm"]["calldata"]
    );
    Eip1559Call {
        chain_id: 125,
        nonce: 7,
        max_priority_fee_per_gas: 2,
        max_fee_per_gas: 200,
        gas_limit: 1_000_000,
        to: encoding::fixed_hex(
            "Programs address",
            "000000000000000000000000000000000000101d",
        )
        .unwrap_or_else(panic_error),
        data,
    }
    .sign(&[1; 32])
    .unwrap_or_else(panic_error)
}

fn hash(raw: &[u8]) -> String {
    format!("0x{}", encoding::hex_encode(&transaction_hash(raw)))
}

fn accept_and_drop(store: &Path) -> (String, JoinHandle<()>) {
    let store = store.to_owned();
    peer("/", None, move |request| {
        assert_eq!(request["method"], "eth_sendRawTransaction");
        let bytes = encoding::hex_decode(
            "accepted transaction",
            request["params"][0]
                .as_str()
                .unwrap_or_else(panic_missing)
                .strip_prefix("0x")
                .unwrap_or_else(panic_missing),
        )
        .unwrap_or_else(panic_error);
        let (call, signature) = decode_signed(&bytes).unwrap_or_else(panic_error);
        recover_sender(&call, &signature).unwrap_or_else(panic_error);
        assert_eq!(call.chain_id, 125);
        assert_eq!(call.data[..4], [0xbd, 0xe3, 0xff, 0x66]);
        // Persist the actual received bytes before severing the accepted request's response.
        let mut file = std::fs::File::create(&store).unwrap_or_else(panic_error);
        file.write_all(&bytes).unwrap_or_else(panic_error);
        file.sync_all().unwrap_or_else(panic_error);
        None
    })
}

fn reopen_receipt_peer(store: &Path) -> (String, JoinHandle<()>) {
    let bytes = std::fs::read(store).unwrap_or_else(panic_error);
    let expected_hash = hash(&bytes);
    peer("/", None, move |request| {
        assert_eq!(
            request["method"], "pax_sequencerReceipt",
            "reconciliation must never resubmit"
        );
        assert_eq!(request["params"], json!([expected_hash]));
        // This peer proves accepted transaction recovery, not execution or finality.
        reply(
            &request,
            &json!({"txHash":expected_hash,"rung":"submitted"}),
        )
        .into()
    })
}

#[test]
fn retired_endpoint_rpc() {
    let auth = format!("LayerX-Key {}", credential_value().as_str());
    let (node_url, worker) = peer("/", Some(auth.clone()), |request| {
        assert_eq!(request["method"], "eth_chainId");
        Some(reply(&request, &json!("0x7d")))
    });
    let node = rpc::RpcClient::new(&node_url, Some(credential_value())).unwrap_or_else(panic_error);
    assert_eq!(
        node.call("eth_chainId", &json!([]))
            .unwrap_or_else(panic_error),
        "0x7d"
    );
    assert!(node
        .call("px_register", &json!(["11".repeat(32), "22".repeat(64)]))
        .err()
        .unwrap_or_else(|| panic!("expected error"))
        .contains("gateway_endpoint_required"));
    assert!(node
        .call("lx_getAccount", &json!([]))
        .err()
        .unwrap_or_else(|| panic!("expected error"))
        .contains("switched off"));
    worker.join().unwrap_or_else(panic_error);

    let (gateway_url, worker) = peer("/services/identity", Some(auth), |request| {
        assert_eq!(request["method"], "px_register");
        Some(reply(&request, &json!({"state":"pending"})))
    });
    let gateway = rpc::RpcClient::new_gateway(
        &format!("{gateway_url}/services/identity"),
        Some(credential_value()),
    )
    .unwrap_or_else(panic_error);
    assert_eq!(
        gateway
            .call("px_register", &json!(["11".repeat(32), "22".repeat(64)]))
            .unwrap_or_else(panic_error)["state"],
        "pending"
    );
    worker.join().unwrap_or_else(panic_error);
    assert!(rpc::RpcClient::new("http://example.invalid/", None).is_err());
    assert!(rpc::RpcClient::new_gateway("http://example.invalid/service", None).is_err());
    assert!(rpc::RpcClient::new("https://example.invalid/", None).is_ok());
    assert!(rpc::RpcClient::new_gateway("https://example.invalid/service", None).is_ok());
    for endpoint in [
        "http://127.0.0.1:1/rpc",
        "http://127.0.0.1:1/rpc/ws",
        "http://127.0.0.1:1/?secret=1",
        "http://user:secret@127.0.0.1:1/",
        "http://127.0.0.1:1/#fragment",
    ] {
        assert!(rpc::RpcClient::new(endpoint, None).is_err(), "{endpoint}");
    }
}

#[test]
fn retired_endpoint_register() {
    let signer = SigningKey::from_bytes(&[5; 32]);
    let public_key = signer.verifying_key().to_bytes();
    let (url, worker) = peer(
        "/services/registration",
        Some("Bearer identity-session".into()),
        move |request| {
            assert_eq!(request["method"], "px_register");
            assert_eq!(
                request["params"]
                    .as_array()
                    .unwrap_or_else(panic_missing)
                    .len(),
                2
            );
            assert_eq!(request["params"][0], encoding::hex_encode(&public_key));
            let signature = encoding::fixed_hex::<64>(
                "binding signature",
                request["params"][1].as_str().unwrap_or_else(panic_missing),
            )
            .unwrap_or_else(panic_error);
            let tagged = |domain: &[u8]| -> [u8; 32] {
                let mut digest = Sha256::new();
                for part in [domain, b"beta".as_slice(), public_key.as_slice()] {
                    digest.update((part.len() as u64).to_be_bytes());
                    digest.update(part);
                }
                digest.finalize().into()
            };
            layerx_crypto::ed25519::verify_digest(
                &public_key,
                &signature,
                &tagged(b"layerx-register-binding-v1"),
            )
            .unwrap_or_else(panic_error);
            let sub = format!(
                "beta.{}",
                &encoding::hex_encode(&tagged(b"layerx-register-subject-v1"))[..32]
            );
            Some(reply(
                &request,
                &json!({"tenant":"beta","sub":sub,"allowed_signer_public_keys":[encoding::hex_encode(&public_key)],"account":null,"audiences":[]}),
            ))
        },
    );
    let result = register::register_principal(
        &format!("{url}/services/registration"),
        Some(Zeroizing::new("identity-session".into())),
        &signer,
    )
    .unwrap_or_else(panic_error);
    assert_eq!(
        result["allowed_signer_public_keys"],
        json!([encoding::hex_encode(&public_key)])
    );
    worker.join().unwrap_or_else(panic_error);
}

#[test]
fn program_submission_recovery() {
    let directory = tempfile::tempdir().unwrap_or_else(panic_error);
    let store = directory.path().join("accepted.raw");
    let raw = signed_deployment();
    let (url, worker) = accept_and_drop(&store);
    let client = http::Client::new(&url, None).unwrap_or_else(panic_error);
    let result = programs::submit_lifecycle_transaction(&client, &raw).unwrap_or_else(panic_error);
    assert_eq!(result["tx_hash"], hash(&raw));
    assert_eq!(result["txHash"], hash(&raw));
    assert_eq!(result["rung"], "unknown");
    std::fs::write(
        directory.path().join("recovery.json"),
        serde_json::to_vec(&result).unwrap_or_else(panic_error),
    )
    .unwrap_or_else(panic_error);
    drop(client);
    worker.join().unwrap_or_else(panic_error);
    assert_eq!(std::fs::read(&store).unwrap_or_else(panic_error), raw);

    let saved: Value = serde_json::from_slice(
        &std::fs::read(directory.path().join("recovery.json")).unwrap_or_else(panic_error),
    )
    .unwrap_or_else(panic_error);
    let (url, worker) = reopen_receipt_peer(&store);
    let reopened = http::Client::new(&url, None).unwrap_or_else(panic_error);
    let receipt = programs::sequencer_receipt(
        &reopened,
        saved["tx_hash"].as_str().unwrap_or_else(panic_missing),
    )
    .unwrap_or_else(panic_error);
    let recovered = programs::lifecycle_receipt_result(
        saved["tx_hash"].as_str().unwrap_or_else(panic_missing),
        &receipt,
    )
    .unwrap_or_else(panic_error);
    assert_eq!(recovered["txHash"], result["txHash"]);
    assert_eq!(recovered["rung"], "submitted");
    worker.join().unwrap_or_else(panic_error);
}

fn tool(name: &str) -> ToolDefinition {
    toolset::surface(DeploymentMode::Full)
        .unwrap_or_else(panic_error)
        .into_iter()
        .find(|tool| tool.name == name)
        .unwrap_or_else(panic_missing)
}

#[test]
fn tool_submit_recovery() {
    for task_path in [false, true] {
        let directory = tempfile::tempdir().unwrap_or_else(panic_error);
        let store = directory.path().join("accepted.raw");
        let raw = signed_deployment();
        let (url, worker) = accept_and_drop(&store);
        let runtime = toolset::Runtime::loopback(&url, 125).unwrap_or_else(panic_error);
        let arguments = json!({"raw_transaction":format!("0x{}", encoding::hex_encode(&raw))});
        let result = if task_path {
            let result = toolset::invoke_task(&runtime, tool("activity.submit"), &arguments)
                .unwrap_or_else(panic_error);
            assert_eq!(result["txHash"], hash(&raw));
            assert_eq!(result["rung"], "unknown");
            assert!(!result["detail"]
                .as_str()
                .unwrap_or_else(panic_missing)
                .is_empty());
            result
        } else {
            let error = toolset::invoke(&runtime, tool("activity.submit"), &arguments)
                .err()
                .unwrap_or_else(|| panic!("expected error"));
            let prefix = format!("submission_indeterminate: {}: ", hash(&raw));
            let detail = error
                .strip_prefix(&prefix)
                .unwrap_or_else(|| panic!("{}", "local hash in tool error"));
            assert!(!detail.is_empty());
            json!(error)
        };
        std::fs::write(
            directory.path().join("recovery.json"),
            serde_json::to_vec(&result).unwrap_or_else(panic_error),
        )
        .unwrap_or_else(panic_error);
        drop(runtime);
        worker.join().unwrap_or_else(panic_error);
        assert_eq!(std::fs::read(&store).unwrap_or_else(panic_error), raw);
        let saved: Value = serde_json::from_slice(
            &std::fs::read(directory.path().join("recovery.json")).unwrap_or_else(panic_error),
        )
        .unwrap_or_else(panic_error);
        let recovery_hash = if task_path {
            saved["txHash"].as_str().unwrap_or_else(panic_missing)
        } else {
            saved
                .as_str()
                .unwrap_or_else(panic_missing)
                .strip_prefix("submission_indeterminate: ")
                .unwrap_or_else(panic_missing)
                .split_once(": ")
                .unwrap_or_else(panic_missing)
                .0
        };
        assert_eq!(recovery_hash, hash(&raw));
        let (url, worker) = reopen_receipt_peer(&store);
        let reopened = toolset::Runtime::loopback(&url, 125).unwrap_or_else(panic_error);
        let receipt = toolset::invoke(
            &reopened,
            tool("receipt.get"),
            &json!({"activity_id":recovery_hash}),
        )
        .unwrap_or_else(panic_error);
        assert_eq!(receipt["txHash"], recovery_hash);
        assert_eq!(receipt["rung"], "submitted");
        worker.join().unwrap_or_else(panic_error);
    }
}

#[test]
fn submission_failures_remain_distinct() {
    let client = http::Client::new("http://127.0.0.1:1", None).unwrap_or_else(panic_error);
    assert!(programs::submit_program_transaction(&client, &[0x02])
        .err()
        .unwrap_or_else(|| panic!("expected error"))
        .starts_with("submission_unsent:"));
    let raw = signed_deployment();
    let expected = hash(&raw);
    for task_path in [false, true] {
        let runtime =
            toolset::Runtime::loopback("http://127.0.0.1:1", 125).unwrap_or_else(panic_error);
        let result = if task_path {
            toolset::invoke_task(
                &runtime,
                tool("activity.submit"),
                &json!({"raw_transaction":"0x02"}),
            )
        } else {
            toolset::invoke(
                &runtime,
                tool("activity.submit"),
                &json!({"raw_transaction":"0x02"}),
            )
        };
        assert!(result
            .err()
            .unwrap_or_else(|| panic!("expected error"))
            .starts_with("submission_unsent:"));
        let (url, worker) = peer("/", None, |request| {
            Some(
                json!({"jsonrpc":"2.0","id":request["id"],"error":{"code":-32000,"message":"transaction rejected"}}),
            )
        });
        let runtime = toolset::Runtime::loopback(&url, 125).unwrap_or_else(panic_error);
        let arguments = json!({"raw_transaction":format!("0x{}", encoding::hex_encode(&raw))});
        let result = if task_path {
            toolset::invoke_task(&runtime, tool("activity.submit"), &arguments)
        } else {
            toolset::invoke(&runtime, tool("activity.submit"), &arguments)
        };
        assert!(result
            .err()
            .unwrap_or_else(|| panic!("expected error"))
            .starts_with("submission_refused:"));
        worker.join().unwrap_or_else(panic_error);
    }
    let (url, worker) = peer("/", None, |request| {
        Some(
            json!({"jsonrpc":"2.0","id":request["id"],"error":{"code":-32000,"message":"transaction rejected"}}),
        )
    });
    assert!(programs::submit_program_transaction(
        &http::Client::new(&url, None).unwrap_or_else(panic_error),
        &raw
    )
    .err()
    .unwrap_or_else(|| panic!("expected error"))
    .starts_with("submission_refused:"));
    worker.join().unwrap_or_else(panic_error);
    let successful_hash = expected.clone();
    let (url, worker) = peer("/", None, move |request| {
        Some(reply(&request, &json!(successful_hash)))
    });
    let accepted = programs::submit_program_transaction(
        &http::Client::new(&url, None).unwrap_or_else(panic_error),
        &raw,
    )
    .unwrap_or_else(panic_error);
    assert_eq!(accepted["tx_hash"], expected);
    assert_eq!(accepted["rung"], "submitted");
    worker.join().unwrap_or_else(panic_error);
    let wrong_network =
        toolset::Runtime::loopback("http://127.0.0.1:1", 402).unwrap_or_else(panic_error);
    assert!(toolset::invoke(
        &wrong_network,
        tool("activity.submit"),
        &json!({"raw_transaction":format!("0x{}", encoding::hex_encode(&raw))})
    )
    .err()
    .unwrap_or_else(|| panic!("expected error"))
    .starts_with("submission_unsent:"));
    for response in [
        json!({"jsonrpc":"2.0","id":2,"error":{"code":-32000,"message":"unbound"}}),
        json!({"jsonrpc":"2.0","id":1,"result":format!("0x{}", "ff".repeat(32))}),
        json!({"jsonrpc":"2.0","id":1,"result":null}),
    ] {
        let (url, worker) = peer("/", None, move |_| Some(response));
        let result = programs::submit_program_transaction(
            &http::Client::new(&url, None).unwrap_or_else(panic_error),
            &raw,
        )
        .unwrap_or_else(panic_error);
        assert_eq!(result["tx_hash"], expected);
        assert_eq!(result["rung"], "unknown");
        worker.join().unwrap_or_else(panic_error);
    }
}

#[test]
fn retired_endpoint_toolset() {
    let tx_hash = hash(&signed_deployment());
    let expected = tx_hash.clone();
    let (url, worker) = peer("/", None, move |request| {
        assert_eq!(request["method"], "pax_sequencerReceipt");
        assert_eq!(request["params"], json!([expected]));
        Some(reply(
            &request,
            &json!({"txHash":expected,"rung":"submitted"}),
        ))
    });
    let runtime = toolset::Runtime::loopback(&url, 125).unwrap_or_else(panic_error);
    assert_eq!(
        toolset::invoke(
            &runtime,
            tool("receipt.get"),
            &json!({"activity_id":tx_hash})
        )
        .unwrap_or_else(panic_error)["txHash"],
        tx_hash
    );
    worker.join().unwrap_or_else(panic_error);
    let (url, worker) = peer(
        "/services/faucet",
        Some("Bearer identity-session".into()),
        |request| {
            assert_eq!(request["method"], "px_requestFunds");
            assert_eq!(
                request["params"],
                json!(["did:layerx:test", "11".repeat(32)])
            );
            Some(reply(
                &request,
                &json!({"funded":true,"funding_id":"aa","transaction_id":format!("0x{}", "bb".repeat(32)),"amount":"100","network":"beta"}),
            ))
        },
    );
    let claim = toolset::claim_faucet_at(
        &format!("{url}/services/faucet"),
        Zeroizing::new("identity-session".into()),
        "did:layerx:test",
        &"11".repeat(32),
    )
    .unwrap_or_else(panic_error);
    assert_eq!(claim["claim"]["funded"], true);
    worker.join().unwrap_or_else(panic_error);
}

#[test]
fn retired_endpoint_subscription() {
    for path in ["/", "/services/events"] {
        let listener = TcpListener::bind("127.0.0.1:0").unwrap_or_else(panic_error);
        let url = format!(
            "http://{}{}",
            listener.local_addr().unwrap_or_else(panic_error),
            path
        );
        let worker = thread::spawn(move || {
            let stream = accept(&listener);
            let mut socket = tungstenite::accept_hdr(
                stream,
                move |request: &tungstenite::handshake::server::Request, response| {
                    assert_eq!(request.uri().path(), path);
                    assert_eq!(
                        request.headers()["authorization"],
                        format!("LayerX-Key {}", credential_value().as_str())
                    );
                    Ok(response)
                },
            )
            .unwrap_or_else(panic_error);
            let command: Value = serde_json::from_str(
                socket
                    .read()
                    .unwrap_or_else(panic_error)
                    .to_text()
                    .unwrap_or_else(panic_error),
            )
            .unwrap_or_else(panic_error);
            assert_eq!(command["method"], "eth_subscribe");
            assert_eq!(
                command["params"],
                json!(["logs", {"address":format!("0x{}", "11".repeat(20)),"topics":[]}])
            );
            socket
                .send(tungstenite::Message::Text(
                    reply(&command, &json!("subscription-1")).to_string().into(),
                ))
                .unwrap_or_else(panic_error);
            socket.send(tungstenite::Message::Text(json!({"jsonrpc":"2.0","method":"eth_subscription","params":{"subscription":"subscription-1","result":{"address":format!("0x{}", "11".repeat(20)),"data":"0x","topics":[]}}}).to_string().into())).unwrap_or_else(panic_error);
        });
        let client = if path == "/" {
            rpc::RpcClient::new(&url, Some(credential_value()))
        } else {
            rpc::RpcClient::new_gateway(&url, Some(credential_value()))
        }
        .unwrap_or_else(panic_error);
        let event = client
            .subscribe(
                &json!(["account", format!("0x{}", "11".repeat(20))]),
                Duration::from_secs(5),
            )
            .unwrap_or_else(panic_error);
        assert_eq!(event["subscription"], "subscription-1");
        assert_eq!(
            event["notification"]["address"],
            format!("0x{}", "11".repeat(20))
        );
        assert_eq!(event["verified"], false);
        worker.join().unwrap_or_else(panic_error);
    }
}

#[test]
fn tls_routes_do_not_downgrade() {
    for websocket in [false, true] {
        let listener = TcpListener::bind("127.0.0.1:0").unwrap_or_else(panic_error);
        let url = format!(
            "https://{}/",
            listener.local_addr().unwrap_or_else(panic_error)
        );
        let worker = thread::spawn(move || {
            let mut stream = accept(&listener);
            let mut header = [0; 5];
            stream.read_exact(&mut header).unwrap_or_else(panic_error);
            assert_eq!(header[0], 0x16, "HTTPS must start a TLS handshake");
            assert_eq!(header[1], 3);
            stream.shutdown(Shutdown::Both).unwrap_or_else(panic_error);
        });
        let client =
            rpc::RpcClient::new(&url, Some(credential_value())).unwrap_or_else(panic_error);
        if websocket {
            assert!(client
                .subscribe(
                    &json!(["account", format!("0x{}", "11".repeat(20))]),
                    Duration::from_secs(5)
                )
                .is_err());
        } else {
            assert!(client.call("eth_chainId", &json!([])).is_err());
        }
        worker.join().unwrap_or_else(panic_error);
    }
}

#[test]
fn scaffold_build_host_path() {
    let directory = tempfile::tempdir().unwrap_or_else(panic_error);
    let relocated = directory.path().join("installed-layerx");
    std::fs::copy(env!("CARGO_BIN_EXE_layerx"), &relocated).unwrap_or_else(panic_error);
    let sdk = Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("../../../programs/sdk/rust")
        .canonicalize()
        .unwrap_or_else(panic_error);
    // The built CLI is the component under test; no external toolchain or live service is used.
    let output = std::process::Command::new(&relocated)
        .env_clear()
        .current_dir(directory.path())
        .env("LAYERX_PROGRAM_SDK", &sdk)
        .args(["--json", "new", "relocated-program"])
        .output()
        .unwrap_or_else(panic_error);
    assert!(
        output.status.success(),
        "{}",
        String::from_utf8_lossy(&output.stderr)
    );
    let manifest = std::fs::read_to_string(directory.path().join("relocated-program/Cargo.toml"))
        .unwrap_or_else(panic_error)
        .parse::<toml_edit::DocumentMut>()
        .unwrap_or_else(panic_error);
    assert_eq!(
        manifest["dependencies"]["layerx-program-sdk"]["path"].as_str(),
        sdk.to_str()
    );
    assert!(directory
        .path()
        .join("relocated-program/src/lib.rs")
        .is_file());
    let output = std::process::Command::new(&relocated)
        .env_clear()
        .current_dir(directory.path())
        .env("LAYERX_PROGRAM_SDK", directory.path().join("absent-sdk"))
        .args(["--json", "new", "invalid-program"])
        .output()
        .unwrap_or_else(panic_error);
    assert!(!output.status.success());
    assert!(!directory.path().join("invalid-program").exists());
    let output = std::process::Command::new(&relocated)
        .env_clear()
        .current_dir(directory.path())
        .args(["--json", "new", "missing-sdk"])
        .output()
        .unwrap_or_else(panic_error);
    assert!(!output.status.success());
    assert!(String::from_utf8_lossy(&output.stderr).contains("LAYERX_PROGRAM_SDK"));
    assert!(!directory.path().join("missing-sdk").exists());
    assert!(scaffold::create_with_sdk("wrong-sdk", directory.path(), directory.path()).is_err());
    assert!(!directory.path().join("wrong-sdk").exists());
}

fn panic_error<T, E: std::fmt::Debug>(error: E) -> T {
    panic!("{error:?}")
}

fn panic_missing<T>() -> T {
    panic!("expected value")
}
