#[allow(dead_code)]
mod repair_22_33_1470;

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
    encoded.extend_from_slice(&word(
        bytes
            .len()
            .try_into()
            .unwrap_or_else(|error| panic!("{error:?}")),
    ));
    encoded.extend_from_slice(bytes);
    encoded.resize(64 + bytes.len().div_ceil(32) * 32, 0);
    encoded
}

#[test]
fn registry_head_reads_the_programs_state_head() {
    let root =
        Root(std::env::temp_dir().join(format!("registry-programs-chain-{}", std::process::id())));
    fs::create_dir(&root.0).unwrap_or_else(|error| panic!("{error:?}"));
    let journal = FileDeploymentJournal::open(root.0.join("journal"))
        .unwrap_or_else(|error| panic!("{error:?}"));
    let authority =
        RegistryAuthority::new("registry-programs-chain-request-token".to_owned().into())
            .unwrap_or_else(|error| panic!("{error:?}"));
    let program = [0x72; 32];
    // The canonical empty-storage head emitted by the programs construct.
    let state_record = repair_22_33_1470::storage_head_abi()[64..102].to_vec();
    let expected_root = hash_bytes(HashAlgorithm::Keccak256, &state_record)
        .unwrap_or_else(|error| panic!("{error:?}"));
    let state_abi = abi_bytes(&state_record);
    assert_ne!(
        expected_root,
        hash_bytes(HashAlgorithm::Keccak256, &state_abi)
            .unwrap_or_else(|error| panic!("{error:?}"))
    );
    let fixture = repair_22_33_1470::admission(1);
    let program_record = repair_22_33_1470::program_abi(
        &fixture
            .proof
            .claimed_record()
            .unwrap_or_else(|error| panic!("{error:?}")),
    );

    let listener = TcpListener::bind("127.0.0.1:0").unwrap_or_else(|error| panic!("{error:?}"));
    let endpoint = format!(
        "http://{}/",
        listener
            .local_addr()
            .unwrap_or_else(|error| panic!("{error:?}"))
    );
    listener
        .set_nonblocking(true)
        .unwrap_or_else(|error| panic!("{error:?}"));
    let peer =
        thread::spawn(move || serve_head_peer(&listener, program, &program_record, &state_abi));
    let deadline = Instant::now() + Duration::from_secs(10);
    let mut request = Request {
        method: "POST".to_owned(),
        path: "/__registry/head".to_owned(),
        headers: HashMap::new(),
        body: serde_json::to_vec(&json!({"program_id": hex::encode(&program)}))
            .unwrap_or_else(|error| panic!("{error:?}")),
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
    let body: Value =
        serde_json::from_str(&response.body).unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(body["observed"], true);
    assert_eq!(body["sequence"], 32);
    assert_eq!(body["observed_at"], 1_700_000_000_000_u64);
    assert_eq!(body["state_root"], hex::encode(&expected_root));
    assert!(body.get("receipt_digest").is_none());
    let retained = journal
        .observed_head()
        .unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(retained.sequence, 32);
    assert_eq!(retained.observed_at, 1_700_000_000_000);
    let calls = peer.join().unwrap_or_else(|error| panic!("{error:?}"));
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

fn serve_head_peer(
    listener: &TcpListener,
    program: [u8; 32],
    program_record: &[u8],
    state_abi: &[u8],
) -> Vec<Value> {
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
            .unwrap_or_else(|error| panic!("{error:?}"));
        stream
            .set_write_timeout(Some(Duration::from_secs(5)))
            .unwrap_or_else(|error| panic!("{error:?}"));
        let request = layerx_platform_internal::http::read_http_message(
            &mut stream,
            layerx_platform_internal::http::MAX_REQUEST_BYTES,
        )
        .unwrap_or_else(|error| panic!("{error:?}"));
        assert_eq!(
            request.headers.get("").map(String::as_str),
            Some("POST / HTTP/1.1")
        );
        assert!(request.headers.contains_key("host"));
        let rpc: Value =
            serde_json::from_slice(&request.body).unwrap_or_else(|error| panic!("{error:?}"));
        let result = match rpc["method"]
            .as_str()
            .unwrap_or_else(|| panic!("expected a value"))
        {
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
                let data = rpc["params"][0]["data"]
                    .as_str()
                    .unwrap_or_else(|| panic!("expected a value"));
                assert_eq!(&data[10..], hex::encode(&program));
                let get_program = hash_bytes(HashAlgorithm::Keccak256, b"getProgram(bytes32)")
                    .unwrap_or_else(|error| panic!("{error:?}"));
                let get_state = hash_bytes(HashAlgorithm::Keccak256, b"getStateHead(bytes32)")
                    .unwrap_or_else(|error| panic!("{error:?}"));
                if data[..10] == format!("0x{}", hex::encode(&get_program[..4])) {
                    json!(format!("0x{}", hex::encode(program_record)))
                } else {
                    assert_eq!(&data[..10], format!("0x{}", hex::encode(&get_state[..4])));
                    json!(format!("0x{}", hex::encode(state_abi)))
                }
            }
            method => panic!("unexpected chain method {method}"),
        };
        calls.push(rpc.clone());
        let body = json!({"jsonrpc": "2.0", "id": rpc["id"], "result": result}).to_string();
        write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{}", body.len(), body).unwrap_or_else(|error| panic!("{error:?}"));
    }
    calls
}

#[test]
fn registry_deployment_reads_the_program_record() {
    use layerx_programs_runtime::{UpgradePolicy, ABI_VERSION};

    for policy in [
        UpgradePolicy::Immutable,
        UpgradePolicy::Authority([0x53; 32]),
    ] {
        let DeploymentCase {
            root,
            endpoint,
            peer,
            transaction,
            failed_transaction,
            program,
            code_hash,
            version,
            policy_number,
            upgrade_authority,
        } = deployment_case(policy);
        let authority = RegistryAuthority::new("registry-deployment-chain-token".to_owned().into())
            .unwrap_or_else(|error| panic!("{error:?}"));
        let deadline = Instant::now() + Duration::from_secs(10);
        let mut request = Request {
            method: "POST".to_owned(),
            path: "/__registry/deployments".to_owned(),
            headers: HashMap::new(),
            body: transaction.to_vec(),
        };
        let route = |request: &Request, deadline| {
            Registrar::route_chain_deployment(request, &authority, &root.0, &endpoint, deadline)
        };
        assert_eq!(route(&request, deadline).status, 401);
        request.headers.insert(
            "authorization".to_owned(),
            "Bearer registry-deployment-chain-token".to_owned(),
        );
        for body in [
            Vec::new(),
            vec![0; 31],
            vec![0; 33],
            br#"{"transaction_hash":"abcd"}"#.to_vec(),
            br#"{"program_id":"71","code_hash":"44","version":1}"#.to_vec(),
        ] {
            request.body = body;
            let response = route(&request, deadline);
            assert_eq!(response.status, 400, "{}", response.body);
            assert_eq!(
                serde_json::from_str::<Value>(&response.body)
                    .unwrap_or_else(|error| panic!("{error:?}"))["error"]["code"],
                "invalid_argument"
            );
        }
        request.body = transaction.to_vec();
        assert_eq!(route(&request, Instant::now()).status, 503);
        let response = route(&request, deadline);
        assert_eq!(response.status, 200, "{}", response.body);
        let body: Value =
            serde_json::from_str(&response.body).unwrap_or_else(|error| panic!("{error:?}"));
        assert_eq!(body["transaction_hash"], hex::encode(&transaction));
        assert_eq!(body["program_id"], hex::encode(&program.bytes()));
        assert_eq!(body["code_hash"], hex::encode(&code_hash));
        assert_eq!(body["abi_version"], ABI_VERSION);
        assert_eq!(body["version"], version);
        assert_eq!(body["policy"], policy_number);
        assert_eq!(body["authority"], hex::encode(&upgrade_authority));
        let path = root
            .0
            .join("chain-deployments")
            .join(format!("{}.json", hex::encode(&transaction)));
        let bytes = fs::read(&path).unwrap_or_else(|error| panic!("{error:?}"));
        let stored: Value =
            serde_json::from_slice(&bytes).unwrap_or_else(|error| panic!("{error:?}"));
        for field in [
            "transaction_hash",
            "program_id",
            "code_hash",
            "abi_version",
            "version",
            "policy",
            "authority",
            "block_hash",
            "block_number",
        ] {
            assert_eq!(stored[field], body[field], "{field}");
        }
        assert_eq!(stored["active"], true);
        assert_deployment_refusals(
            &route,
            &mut request,
            &root,
            &path,
            &bytes,
            (transaction, failed_transaction),
            deadline,
        );
        assert_deployment_rpc_calls(peer);
    }
}

fn assert_deployment_rpc_calls(peer: std::thread::JoinHandle<Vec<Value>>) {
    let calls = peer.join().unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(
        calls
            .iter()
            .filter(|call| call["method"] == "eth_call")
            .count(),
        3
    );
    assert_eq!(
        calls
            .iter()
            .filter(|call| call["method"] == "eth_getTransactionReceipt")
            .count(),
        7
    );
}

fn assert_deployment_refusals(
    route: &impl Fn(&Request, Instant) -> layerx_platform_registry::Response,
    request: &mut Request,
    root: &Root,
    path: &std::path::Path,
    bytes: &[u8],
    transactions: ([u8; 32], [u8; 32]),
    deadline: Instant,
) {
    let (transaction, failed_transaction) = transactions;
    request.body = serde_json::to_vec(
        &json!({"transaction_hash": format!("0x{}", hex::encode(&transaction))}),
    )
    .unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(route(request, deadline).status, 200);
    assert_eq!(
        fs::read(path).unwrap_or_else(|error| panic!("{error:?}")),
        bytes
    );
    for tx in [
        failed_transaction,
        transaction,
        transaction,
        transaction,
        transaction,
    ] {
        request.body = tx.to_vec();
        let response = route(request, deadline);
        assert_eq!(response.status, 422, "{}", response.body);
        assert_eq!(
            serde_json::from_str::<Value>(&response.body)
                .unwrap_or_else(|error| panic!("{error:?}"))["error"]["code"],
            "deployment_proof_refused"
        );
        assert_eq!(
            fs::read(path).unwrap_or_else(|error| panic!("{error:?}")),
            bytes
        );
    }
    assert!(!root
        .0
        .join("chain-deployments")
        .join(format!("{}.json", hex::encode(&failed_transaction)))
        .exists());
}

struct DeploymentCase {
    root: Root,
    endpoint: String,
    peer: std::thread::JoinHandle<Vec<Value>>,
    transaction: [u8; 32],
    failed_transaction: [u8; 32],
    program: layerx_programs_runtime::ProgramId,
    code_hash: [u8; 32],
    version: u32,
    policy_number: u64,
    upgrade_authority: [u8; 32],
}

fn deployment_receipt_variants(
    receipt: &Value,
    failed_transaction: [u8; 32],
    record: &[u8],
) -> (Vec<Value>, Vec<u8>) {
    let mut failed = receipt.clone();
    failed["status"] = json!("0x0");
    failed["transactionHash"] = json!(format!("0x{}", hex::encode(&failed_transaction)));
    failed["logs"] = json!([]);
    let mut wrong_transaction = receipt.clone();
    wrong_transaction["transactionHash"] = failed["transactionHash"].clone();
    let mut missing_event = receipt.clone();
    missing_event["logs"][0]["address"] = json!("0x000000000000000000000000000000000000101c");
    let mut bad_record = record.to_vec();
    bad_record[64] ^= 1;
    let receipts = vec![
        receipt.clone(),
        receipt.clone(),
        failed,
        Value::Null,
        wrong_transaction,
        missing_event,
        receipt.clone(),
    ];
    (receipts, bad_record)
}

fn deployment_case(policy: layerx_programs_runtime::UpgradePolicy) -> DeploymentCase {
    use layerx_programs_runtime::{Deploy, Lifecycle, ProgramId, UpgradePolicy, ABI_VERSION};
    let (policy_number, upgrade_authority) = match policy {
        UpgradePolicy::Immutable => (0_u64, [0; 32]),
        UpgradePolicy::Authority(authority) => (1, authority),
    };
    let root = Root(std::env::temp_dir().join(format!(
        "registry-chain-deployment-{}-{policy_number}",
        std::process::id()
    )));
    fs::create_dir(&root.0).unwrap_or_else(|error| panic!("{error:?}"));
    let program = ProgramId::new([0x71; 32]).unwrap_or_else(|error| panic!("{error:?}"));
    let wasm = layerx_programs_runtime::test_support::add_module();
    let code_hash: [u8; 32] = Sha256::digest(&wasm).into();
    let activity = Deploy {
        program,
        code_hash,
        wasm,
        abi_version: ABI_VERSION,
        upgrade_policy: policy,
    };
    let mut lifecycle = Lifecycle::declared().unwrap_or_else(|error| panic!("{error:?}"));
    let deployed = lifecycle
        .deploy(activity.clone())
        .unwrap_or_else(|error| panic!("{error:?}"));
    assert!(lifecycle.deploy(activity).is_err());
    let transaction = [0x81; 32];
    let failed_transaction = [0x82; 32];
    let block_hash = [0x91; 32];
    let deployed_topic = hash_bytes(
        HashAlgorithm::Keccak256,
        b"ProgramDeployed(bytes32,bytes32)",
    )
    .unwrap_or_else(|error| panic!("{error:?}"));
    let mut record = word(policy_number);
    record.extend_from_slice(&upgrade_authority);
    record.extend_from_slice(&deployed.new_code_hash());
    record.extend_from_slice(&word(u64::from(ABI_VERSION)));
    record.extend_from_slice(&word(u64::from(deployed.version())));
    record.extend_from_slice(&word(1));
    let receipt = json!({
        "transactionHash": format!("0x{}", hex::encode(&transaction)),
        "blockHash": format!("0x{}", hex::encode(&block_hash)),
        "blockNumber": "0x20",
        "status": "0x1",
        "logs": [{
            "address": "0x000000000000000000000000000000000000101d",
            "topics": [format!("0x{}", hex::encode(&deployed_topic)),
                format!("0x{}", hex::encode(&deployed.program().bytes()))],
            "data": format!("0x{}", hex::encode(&deployed.new_code_hash())),
            "transactionHash": format!("0x{}", hex::encode(&transaction)),
            "blockHash": format!("0x{}", hex::encode(&block_hash)),
            "blockNumber": "0x20",
            "removed": false,
        }],
    });
    let (receipts, bad_record) = deployment_receipt_variants(&receipt, failed_transaction, &record);
    let listener = TcpListener::bind("127.0.0.1:0").unwrap_or_else(|error| panic!("{error:?}"));
    let endpoint = format!(
        "http://{}/",
        listener
            .local_addr()
            .unwrap_or_else(|error| panic!("{error:?}"))
    );
    listener
        .set_nonblocking(true)
        .unwrap_or_else(|error| panic!("{error:?}"));
    let peer_case = DeploymentPeerCase {
        program,
        block_hash,
        bad_record,
        record,
        receipts,
        transaction,
        failed_transaction,
    };
    let peer = thread::spawn(move || serve_deployment_peer(&listener, peer_case));
    DeploymentCase {
        root,
        endpoint,
        peer,
        transaction,
        failed_transaction,
        program,
        code_hash,
        version: deployed.version(),
        policy_number,
        upgrade_authority,
    }
}

struct DeploymentPeerCase {
    program: layerx_programs_runtime::ProgramId,
    block_hash: [u8; 32],
    bad_record: Vec<u8>,
    record: Vec<u8>,
    receipts: Vec<Value>,
    transaction: [u8; 32],
    failed_transaction: [u8; 32],
}

fn serve_deployment_peer(listener: &TcpListener, case: DeploymentPeerCase) -> Vec<Value> {
    let DeploymentPeerCase {
        program,
        block_hash,
        bad_record,
        record,
        receipts,
        transaction,
        failed_transaction,
    } = case;

    let mut receipts = receipts.into_iter();
    let mut calls = Vec::new();
    let mut views = 0;
    for _ in 0..10 {
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
            .unwrap_or_else(|error| panic!("{error:?}"));
        stream
            .set_write_timeout(Some(Duration::from_secs(5)))
            .unwrap_or_else(|error| panic!("{error:?}"));
        let request = layerx_platform_internal::http::read_http_message(
            &mut stream,
            layerx_platform_internal::http::MAX_REQUEST_BYTES,
        )
        .unwrap_or_else(|error| panic!("{error:?}"));
        let rpc: Value =
            serde_json::from_slice(&request.body).unwrap_or_else(|error| panic!("{error:?}"));
        let result = match rpc["method"]
            .as_str()
            .unwrap_or_else(|| panic!("expected a value"))
        {
            "eth_getTransactionReceipt" => {
                let expected = if calls.len() == 4 {
                    failed_transaction
                } else {
                    transaction
                };
                assert_eq!(
                    rpc["params"],
                    json!([format!("0x{}", hex::encode(&expected))])
                );
                receipts
                    .next()
                    .unwrap_or_else(|| panic!("expected a value"))
            }
            "eth_call" => {
                let selector = hash_bytes(HashAlgorithm::Keccak256, b"getProgram(bytes32)")
                    .unwrap_or_else(|error| panic!("{error:?}"));
                assert_eq!(
                    rpc["params"],
                    json!([
                        {"to": "0x000000000000000000000000000000000000101d",
                         "data": format!("0x{}{}", hex::encode(&selector[..4]), hex::encode(&program.bytes()))},
                        {"blockHash": format!("0x{}", hex::encode(&block_hash)), "requireCanonical": true}
                    ])
                );
                views += 1;
                json!(format!(
                    "0x{}",
                    hex::encode(if views == 3 { &bad_record } else { &record })
                ))
            }
            method => panic!("unexpected chain method {method}"),
        };
        calls.push(rpc.clone());
        let body = json!({"jsonrpc": "2.0", "id": rpc["id"], "result": result}).to_string();
        write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{}", body.len(), body).unwrap_or_else(|error| panic!("{error:?}"));
    }
    assert!(receipts.next().is_none());
    calls
}

#[test]
fn registry_starts_without_an_lni_socket() {
    repair_22_33_1470::registry_http_without_socket_recovers_the_same_view();
}

#[test]
fn registry_programs_root_authenticated_deployment_head_and_refusals() {
    repair_22_33_1470::programs_root_transport_admits_and_refuses_transactions();
}
