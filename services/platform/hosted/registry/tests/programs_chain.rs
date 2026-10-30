use std::collections::HashMap;
use std::fs;
use std::io::Write as _;
use std::net::TcpListener;
use std::path::PathBuf;
use std::thread;
use std::time::{Duration, Instant};

use layerx_platform_registry::{FileDeploymentJournal, Registrar, RegistryAuthority, Request};
use layerx_programs::{hex, DeploymentJournal as _};
use layerx_programs_runtime::crypto::hash::{hash_bytes, HashAlgorithm};
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};

struct Root(PathBuf);
impl Drop for Root {
    fn drop(&mut self) {
        let _ = fs::remove_dir_all(&self.0);
    }
}

fn word(value: u64) -> Vec<u8> {
    let mut bytes = vec![0; 24];
    bytes.extend_from_slice(&value.to_be_bytes());
    bytes
}

fn abi_bytes(bytes: &[u8]) -> Vec<u8> {
    let mut encoded = word(32);
    encoded.extend_from_slice(&word(bytes.len().try_into().unwrap()));
    encoded.extend_from_slice(bytes);
    encoded.resize(64 + bytes.len().div_ceil(32) * 32, 0);
    encoded
}

#[test]
fn registry_head_reads_the_programs_state_head() {
    let root =
        Root(std::env::temp_dir().join(format!("registry-programs-chain-{}", std::process::id())));
    fs::create_dir(&root.0).unwrap();
    let journal = FileDeploymentJournal::open(root.0.join("journal")).unwrap();
    let authority =
        RegistryAuthority::new("registry-programs-chain-request-token".to_owned().into()).unwrap();
    let program = [0x72; 32];
    // The canonical empty-storage head emitted by the programs construct.
    let mut state_record = vec![0, 1, 0, 0, 0, 0];
    state_record.extend_from_slice(&Sha256::digest(&state_record));
    let expected_root = hash_bytes(HashAlgorithm::Keccak256, &state_record).unwrap();
    let state_abi = abi_bytes(&state_record);
    assert_ne!(
        expected_root,
        hash_bytes(HashAlgorithm::Keccak256, &state_abi).unwrap()
    );
    let mut program_record = word(1);
    program_record.extend_from_slice(&[0x33; 32]);
    program_record.extend_from_slice(&[0x44; 32]);
    program_record.extend_from_slice(&word(5));
    program_record.extend_from_slice(&word(1));
    program_record.extend_from_slice(&word(1));

    let listener = TcpListener::bind("127.0.0.1:0").unwrap();
    let endpoint = format!(
        "http://{}/rpc?construct=programs",
        listener.local_addr().unwrap()
    );
    listener.set_nonblocking(true).unwrap();
    let peer = thread::spawn(move || {
        let mut calls = Vec::new();
        for _ in 0..4 {
            let deadline = Instant::now() + Duration::from_secs(10);
            let mut stream = loop {
                match listener.accept() {
                    Ok((stream, _)) => break stream,
                    Err(error)
                        if error.kind() == std::io::ErrorKind::WouldBlock
                            && Instant::now() < deadline =>
                    {
                        thread::sleep(Duration::from_millis(5));
                    }
                    Err(error) => panic!("chain listener: {error}"),
                }
            };
            stream
                .set_read_timeout(Some(Duration::from_secs(5)))
                .unwrap();
            stream
                .set_write_timeout(Some(Duration::from_secs(5)))
                .unwrap();
            let request = layerx_platform_internal::http::read_http_message(
                &mut stream,
                layerx_platform_internal::http::MAX_REQUEST_BYTES,
            )
            .unwrap();
            assert_eq!(
                request.headers.get("").map(String::as_str),
                Some("POST /rpc?construct=programs HTTP/1.1")
            );
            assert!(request.headers.contains_key("host"));
            let rpc: Value = serde_json::from_slice(&request.body).unwrap();
            let result = match rpc["method"].as_str().unwrap() {
                "eth_blockNumber" => {
                    assert_eq!(rpc["params"], json!([]));
                    json!("0x20")
                }
                "eth_getBlockByNumber" => {
                    assert_eq!(rpc["params"], json!(["0x20", false]));
                    json!({"number": "0x20", "timestamp": "0x6553f100"})
                }
                "eth_call" => {
                    assert_eq!(
                        rpc["params"][0]["to"],
                        "0x000000000000000000000000000000000000101d"
                    );
                    assert_eq!(rpc["params"][1], "0x20");
                    let data = rpc["params"][0]["data"].as_str().unwrap();
                    assert_eq!(&data[10..], hex::encode(&program));
                    let get_program =
                        hash_bytes(HashAlgorithm::Keccak256, b"getProgram(bytes32)").unwrap();
                    let get_state =
                        hash_bytes(HashAlgorithm::Keccak256, b"getStateHead(bytes32)").unwrap();
                    if data[..10] == format!("0x{}", hex::encode(&get_program[..4])) {
                        json!(format!("0x{}", hex::encode(&program_record)))
                    } else {
                        assert_eq!(&data[..10], format!("0x{}", hex::encode(&get_state[..4])));
                        json!(format!("0x{}", hex::encode(&state_abi)))
                    }
                }
                method => panic!("unexpected chain method {method}"),
            };
            calls.push(rpc.clone());
            let body = json!({"jsonrpc": "2.0", "id": rpc["id"], "result": result}).to_string();
            write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{}", body.len(), body).unwrap();
        }
        calls
    });
    let deadline = Instant::now() + Duration::from_secs(10);
    let mut request = Request {
        method: "POST".to_owned(),
        path: "/__registry/head".to_owned(),
        headers: HashMap::new(),
        body: serde_json::to_vec(&json!({"program_id": hex::encode(&program)})).unwrap(),
    };
    assert_eq!(
        Registrar::route_chain_head(&request, &authority, &journal, &endpoint, deadline).status,
        401
    );
    request.headers.insert(
        "authorization".to_owned(),
        "Bearer registry-programs-chain-request-token".to_owned(),
    );
    let valid_body = request.body.clone();
    request.body = br#"{"program_id":"abcd"}"#.to_vec();
    assert_eq!(
        Registrar::route_chain_head(&request, &authority, &journal, &endpoint, deadline).status,
        400
    );
    request.body = valid_body;
    assert_eq!(
        Registrar::route_chain_head(&request, &authority, &journal, &endpoint, Instant::now())
            .status,
        503
    );
    let response = Registrar::route_chain_head(&request, &authority, &journal, &endpoint, deadline);
    assert_eq!(response.status, 200, "{}", response.body);
    let body: Value = serde_json::from_str(&response.body).unwrap();
    assert_eq!(body["observed"], true);
    assert_eq!(body["sequence"], 32);
    assert_eq!(body["observed_at"], 1_700_000_000_000_u64);
    assert_eq!(body["state_root"], hex::encode(&expected_root));
    assert!(body.get("receipt_digest").is_none());
    let retained = journal.observed_head().unwrap();
    assert_eq!(retained.sequence, 32);
    assert_eq!(retained.observed_at, 1_700_000_000_000);
    let calls = peer.join().unwrap();
    assert_eq!(
        calls
            .iter()
            .filter(|call| call["method"] == "eth_call")
            .count(),
        2
    );
    assert_eq!(
        calls
            .iter()
            .filter(|call| call["method"] == "eth_blockNumber")
            .count(),
        1
    );
}
