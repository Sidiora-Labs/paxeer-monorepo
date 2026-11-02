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

#[test]
fn registry_deployment_reads_the_program_record() {
    use layerx_programs_runtime::{Deploy, Lifecycle, ProgramId, UpgradePolicy, ABI_VERSION};

    for policy in [
        UpgradePolicy::Immutable,
        UpgradePolicy::Authority([0x53; 32]),
    ] {
        let (policy_number, upgrade_authority) = match policy {
            UpgradePolicy::Immutable => (0_u64, [0; 32]),
            UpgradePolicy::Authority(authority) => (1, authority),
        };
        let root = Root(std::env::temp_dir().join(format!(
            "registry-chain-deployment-{}-{policy_number}",
            std::process::id()
        )));
        fs::create_dir(&root.0).unwrap();
        let program = ProgramId::new([0x71; 32]).unwrap();
        let wasm = layerx_programs_runtime::test_support::add_module();
        let code_hash: [u8; 32] = Sha256::digest(&wasm).into();
        let activity = Deploy {
            program,
            code_hash,
            wasm,
            abi_version: ABI_VERSION,
            upgrade_policy: policy,
        };
        let mut lifecycle = Lifecycle::declared().unwrap();
        let deployed = lifecycle.deploy(activity.clone()).unwrap();
        assert!(lifecycle.deploy(activity).is_err());
        let transaction = [0x81; 32];
        let failed_transaction = [0x82; 32];
        let block_hash = [0x91; 32];
        let deployed_topic = hash_bytes(
            HashAlgorithm::Keccak256,
            b"ProgramDeployed(bytes32,bytes32)",
        )
        .unwrap();
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
        let mut failed = receipt.clone();
        failed["status"] = json!("0x0");
        failed["transactionHash"] = json!(format!("0x{}", hex::encode(&failed_transaction)));
        failed["logs"] = json!([]);
        let mut wrong_transaction = receipt.clone();
        wrong_transaction["transactionHash"] = failed["transactionHash"].clone();
        let mut missing_event = receipt.clone();
        missing_event["logs"][0]["address"] = json!("0x000000000000000000000000000000000000101c");
        let mut bad_record = record.clone();
        bad_record[64] ^= 1;
        let receipts = vec![
            receipt.clone(),
            receipt.clone(),
            failed,
            Value::Null,
            wrong_transaction,
            missing_event,
            receipt,
        ];
        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        let endpoint = format!("http://{}/rpc", listener.local_addr().unwrap());
        listener.set_nonblocking(true).unwrap();
        let peer = thread::spawn(move || {
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
                            thread::sleep(Duration::from_millis(5))
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
                let rpc: Value = serde_json::from_slice(&request.body).unwrap();
                let result = match rpc["method"].as_str().unwrap() {
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
                        receipts.next().unwrap()
                    }
                    "eth_call" => {
                        let selector =
                            hash_bytes(HashAlgorithm::Keccak256, b"getProgram(bytes32)").unwrap();
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
                write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{}", body.len(), body).unwrap();
            }
            assert!(receipts.next().is_none());
            calls
        });
        let authority =
            RegistryAuthority::new("registry-deployment-chain-token".to_owned().into()).unwrap();
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
                serde_json::from_str::<Value>(&response.body).unwrap()["error"]["code"],
                "invalid_argument"
            );
        }
        request.body = transaction.to_vec();
        assert_eq!(route(&request, Instant::now()).status, 503);
        let response = route(&request, deadline);
        assert_eq!(response.status, 200, "{}", response.body);
        let body: Value = serde_json::from_str(&response.body).unwrap();
        assert_eq!(body["transaction_hash"], hex::encode(&transaction));
        assert_eq!(body["program_id"], hex::encode(&program.bytes()));
        assert_eq!(body["code_hash"], hex::encode(&code_hash));
        assert_eq!(body["abi_version"], ABI_VERSION);
        assert_eq!(body["version"], deployed.version());
        assert_eq!(body["policy"], policy_number);
        assert_eq!(body["authority"], hex::encode(&upgrade_authority));
        let path = root
            .0
            .join("chain-deployments")
            .join(format!("{}.json", hex::encode(&transaction)));
        let bytes = fs::read(&path).unwrap();
        let stored: Value = serde_json::from_slice(&bytes).unwrap();
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
        request.body = serde_json::to_vec(
            &json!({"transaction_hash": format!("0x{}", hex::encode(&transaction))}),
        )
        .unwrap();
        assert_eq!(route(&request, deadline).status, 200);
        assert_eq!(fs::read(&path).unwrap(), bytes);
        for tx in [
            failed_transaction,
            transaction,
            transaction,
            transaction,
            transaction,
        ] {
            request.body = tx.to_vec();
            let response = route(&request, deadline);
            assert_eq!(response.status, 422, "{}", response.body);
            assert_eq!(
                serde_json::from_str::<Value>(&response.body).unwrap()["error"]["code"],
                "deployment_proof_refused"
            );
            assert_eq!(fs::read(&path).unwrap(), bytes);
        }
        assert!(!root
            .0
            .join("chain-deployments")
            .join(format!("{}.json", hex::encode(&failed_transaction)))
            .exists());
        let calls = peer.join().unwrap();
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
}

#[allow(dead_code)]
#[path = "../../../../../programs/crates/layerx-programs-registry/tests/support/chain.rs"]
mod chain;

#[test]
fn registry_starts_without_an_lni_socket() {
    const CHILD: &str = "LAYERX_TEST_REGISTRY_WITHOUT_LNI";
    if std::env::var_os(CHILD).is_none() {
        let output = std::process::Command::new(std::env::current_exe().unwrap())
            .args([
                "--exact",
                "registry_starts_without_an_lni_socket",
                "--nocapture",
            ])
            .env_remove("LAYERX_REGISTRY_LNI_SOCKET")
            .env(CHILD, "1")
            .output()
            .unwrap();
        assert!(
            output.status.success(),
            "{}\n{}",
            String::from_utf8_lossy(&output.stdout),
            String::from_utf8_lossy(&output.stderr)
        );
        return;
    }
    assert!(std::env::var_os("LAYERX_REGISTRY_LNI_SOCKET").is_none());
    use layerx_programs::{
        chain_evidence::programs_state_key, chain_proof::trust_store::TrustRoot, ObservedHead,
        ProgramId,
    };
    use std::os::unix::fs::PermissionsExt as _;

    let root =
        Root(std::env::temp_dir().join(format!("registry-without-lni-{}", std::process::id())));
    fs::create_dir(&root.0).unwrap();
    let program = ProgramId::new([0x73; 32]).unwrap();
    let wasm = layerx_programs_runtime::test_support::add_module();
    let code_hash: [u8; 32] = Sha256::digest(&wasm).into();
    let mut key = b"program\0".to_vec();
    key.extend_from_slice(&program.bytes());
    let key = programs_state_key(&key);
    let mut record = vec![0];
    record.extend_from_slice(&[0; 32]);
    record.extend_from_slice(&code_hash);
    record.extend_from_slice(&1_u16.to_be_bytes());
    record.extend_from_slice(&1_u32.to_be_bytes());
    let fixture = chain::ChainFixture::new(
        "registry-without-lni",
        &[[1; 32], [2; 32], [3; 32], [4; 32]],
        vec![(key.clone(), record)],
    )
    .unwrap();
    let verified_head = fixture
        .verifier
        .verify_head(&fixture.header, &fixture.commit)
        .unwrap();
    let trust = TrustRoot::from_head(&verified_head, fixture.verifier.validators.clone()).unwrap();
    let trust_path = root.0.join("chain-trust");
    fs::write(&trust_path, trust.canonical_bytes().unwrap()).unwrap();
    fs::set_permissions(&trust_path, fs::Permissions::from_mode(0o600)).unwrap();
    let journal_path = root.0.join("journal");
    let journal =
        FileDeploymentJournal::open_from_chain_trust(journal_path.clone(), &trust_path, 1_000)
            .unwrap();
    let now = u64::try_from(verified_head.time_ms()).unwrap();
    let head = ObservedHead {
        sequence: u64::try_from(verified_head.height()).unwrap(),
        observed_at: now,
    };
    journal.refresh_head(head).unwrap();
    let package = serde_json::to_vec(&json!({
        "commitment": fixture.encode_key(&key).unwrap(),
        "code": wasm,
        "interface": null,
    }))
    .unwrap();
    let deployment = journal.append_chain_deployment(&package, now).unwrap();
    assert_eq!(deployment.program(), program);
    assert_eq!(deployment.code_hash(), code_hash);
    assert_eq!(deployment.version(), 1);
    assert_eq!(deployment.state_root(), verified_head.app_hash());
    assert_eq!(journal.observed_head().unwrap(), head);
    drop(journal);
    let reopened =
        FileDeploymentJournal::open_from_chain_trust(journal_path.clone(), &trust_path, 1_000)
            .unwrap();
    assert_eq!(reopened.observed_head().unwrap(), head);
    assert_eq!(
        reopened.append_chain_deployment(&package, now).unwrap(),
        deployment
    );
    let saved = journal_path.join("chain-proofs").join(format!(
        "{}.chain-deployment",
        hex::encode(&Sha256::digest(&package))
    ));
    assert_eq!(fs::read(saved).unwrap(), package);
}
