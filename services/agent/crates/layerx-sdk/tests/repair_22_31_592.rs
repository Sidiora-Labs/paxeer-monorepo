use layerx_sdk::rpc::{FaucetGrant, RpcClient, RpcError};
use serde_json::{json, Value};
use std::io::{Read as _, Write as _};
use std::net::TcpListener;
use std::path::Path;
use std::thread::{self, JoinHandle};
use std::time::{Duration, Instant};

const DID: &str = "did:layerx:alice";
const SIGNER_PUBLIC_KEY: [u8; 32] = [0xab; 32];
const HOSTED_PATH: &str = "/hosted/rpc";
const OPENRPC: &str = include_str!("../../../../platform/hosted/gateway/openrpc.json");

fn grant_document() -> Value {
    json!({
        "funded": true,
        "funding_id": "ef".repeat(32),
        "transaction_id": "cd".repeat(32),
        "amount": "10000000000000000000",
        "network": "layerx-testnet",
        "starter_credit": {
            "sub": "beta.alice",
            "grant_id": "ab".repeat(16),
            "asset": "PAX",
            "amount": "10000000000000000000",
            "granted_at": 1,
            "settled_at": 2
        }
    })
}

fn answering_server(result: Value) -> (RpcClient, JoinHandle<()>) {
    let listener = TcpListener::bind("127.0.0.1:0").expect("bind local hosted RPC");
    let endpoint = format!(
        "http://{}{HOSTED_PATH}",
        listener.local_addr().expect("listener address")
    );
    listener.set_nonblocking(true).expect("nonblocking accept");
    let server = thread::spawn(move || {
        let deadline = Instant::now() + Duration::from_secs(10);
        let mut socket = loop {
            match listener.accept() {
                Ok((socket, _)) => break socket,
                Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                    assert!(Instant::now() < deadline, "missing hosted RPC request");
                    thread::sleep(Duration::from_millis(5));
                }
                Err(error) => panic!("accept hosted RPC: {error}"),
            }
        };
        socket
            .set_read_timeout(Some(Duration::from_secs(5)))
            .expect("read timeout");
        socket
            .set_write_timeout(Some(Duration::from_secs(5)))
            .expect("write timeout");
        let mut header = Vec::new();
        while !header.ends_with(b"\r\n\r\n") {
            assert!(header.len() < 8192, "HTTP header bound");
            let mut byte = [0];
            socket.read_exact(&mut byte).expect("read HTTP header");
            header.push(byte[0]);
        }
        let header = std::str::from_utf8(&header).expect("UTF-8 HTTP header");
        assert_eq!(
            header.lines().next(),
            Some(format!("POST {HOSTED_PATH} HTTP/1.1").as_str()),
            "the configured hosted URL path must be preserved"
        );
        let header_value = |name: &str| {
            header.lines().find_map(|line| {
                let (key, value) = line.split_once(':')?;
                key.eq_ignore_ascii_case(name).then_some(value.trim())
            })
        };
        assert_eq!(header_value("content-type"), Some("application/json"));
        let length = header_value("content-length")
            .expect("content-length header")
            .parse::<usize>()
            .expect("content length");
        assert!(length <= 4096, "faucet request body bound");
        let mut body = vec![0; length];
        socket.read_exact(&mut body).expect("read HTTP body");
        let request: Value = serde_json::from_slice(&body).expect("JSON-RPC request");
        assert_eq!(request["jsonrpc"], "2.0");
        assert!(request["id"].is_string());
        assert_eq!(request["method"], "px_requestFunds");
        assert_eq!(request["params"], json!([DID, "ab".repeat(32)]));

        let contract: Value = serde_json::from_str(OPENRPC).expect("hosted OpenRPC contract");
        let method = contract["methods"]
            .as_array()
            .expect("OpenRPC methods")
            .iter()
            .find(|method| method["name"] == request["method"])
            .expect("requested method exists in hosted contract");
        assert_eq!(method["paramStructure"], "by-position");
        let params = method["params"].as_array().expect("declared params");
        assert_eq!(params.len(), 2);
        assert_eq!(params[0]["name"], "did");
        assert_eq!(params[1]["name"], "signer_public_key");
        assert_eq!(params[0]["required"], true);
        assert_eq!(params[1]["required"], true);

        let response = serde_json::to_vec(&json!({
            "jsonrpc": "2.0", "id": request["id"], "result": result
        }))
        .expect("encode JSON-RPC response");
        write!(
            socket,
            "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n",
            response.len()
        )
        .expect("write response header");
        socket.write_all(&response).expect("write response body");
    });
    (
        RpcClient::connect_hosted(&endpoint, None).expect("configure hosted RPC"),
        server,
    )
}

#[test]
fn request_funds_posts_hosted_px_method_and_decodes_funded_grant() {
    let document = grant_document();
    let (client, server) = answering_server(document.clone());
    let result = client.request_funds(DID, &SIGNER_PUBLIC_KEY);
    server.join().expect("hosted RPC request assertions");
    let grant: FaucetGrant = result.expect("confirmed funded grant");
    assert_eq!(grant.funding_id, "ef".repeat(32));
    assert_eq!(grant.transaction_id, "cd".repeat(32));
    assert_eq!(grant.amount, 10_000_000_000_000_000_000_u128);
    assert_eq!(grant.network, "layerx-testnet");
    assert_eq!(grant.unverified_fields()["funded"], true);
    assert_eq!(grant.into_value(), document);
}

#[test]
fn request_funds_refuses_an_unfunded_hosted_answer() {
    let mut document = grant_document();
    document["funded"] = json!(false);
    let (client, server) = answering_server(document);
    let result = client.request_funds(DID, &SIGNER_PUBLIC_KEY);
    server.join().expect("hosted RPC request assertions");
    assert!(
        matches!(result, Err(RpcError::InvalidResponse)),
        "{result:?}"
    );
}

fn assert_no_retired_faucet_method(path: &Path) -> usize {
    let mut source_count = 0;
    for entry in std::fs::read_dir(path).expect("read SDK source directory") {
        let path = entry.expect("source entry").path();
        if path.is_dir() {
            source_count += assert_no_retired_faucet_method(&path);
        } else if path.extension().is_some_and(|extension| extension == "rs") {
            let source = std::fs::read_to_string(&path).expect("read Rust source");
            assert!(
                !source.contains(concat!("lx_", "requestFunds")),
                "retired faucet method in {}",
                path.display()
            );
            source_count += 1;
        }
    }
    source_count
}

#[test]
fn crate_sources_do_not_reference_the_retired_faucet_method() {
    let source_root = Path::new(env!("CARGO_MANIFEST_DIR")).join("src");
    assert!(assert_no_retired_faucet_method(&source_root) > 0);
}
