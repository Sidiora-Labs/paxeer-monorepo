use std::fs;
use std::io::Write as _;
use std::net::{TcpListener, TcpStream};
use std::path::PathBuf;
use std::thread;
use std::time::{Duration, Instant, SystemTime, UNIX_EPOCH};

use layerx_crypto::evm_transaction::transaction_hash as keccak256;
use layerx_platform_registry::{
    parse_request, write_response, FileDeploymentJournal, FileProgramStateJournal, Registrar,
    RegistryAuthority,
};
use layerx_programs::{hex, DeploymentJournal as _, ProgramId};
use serde::Deserialize as _;
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};

const PROGRAM: [u8; 32] = [0x72; 32];
const TOKEN: &str = "repair-22-31-358-request-authority";
const RPC_VARIABLE: &str = "LAYERX_REGISTRY_CHAIN_RPC_URL";

struct Directory(PathBuf);

impl Directory {
    fn new() -> Self {
        let root = std::env::temp_dir().join(format!(
            "repair-22-31-358-{}-{}",
            std::process::id(),
            SystemTime::now()
                .duration_since(UNIX_EPOCH)
                .unwrap()
                .as_nanos()
        ));
        fs::create_dir(&root).unwrap();
        Self(root)
    }
}

impl Drop for Directory {
    fn drop(&mut self) {
        fs::remove_dir_all(&self.0).unwrap();
    }
}

fn deployment() -> Vec<Value> {
    serde_yaml::Deserializer::from_str(include_str!("../deployment.yaml"))
        .map(|document| Value::deserialize(document).unwrap())
        .collect()
}

fn rendered_endpoint(documents: &[Value]) -> ureq::http::Uri {
    let workload = documents
        .iter()
        .find(|document| document["kind"] == "StatefulSet")
        .unwrap();
    let registry = workload["spec"]["template"]["spec"]["containers"]
        .as_array()
        .unwrap()
        .iter()
        .find(|container| container["name"] == "registry")
        .unwrap();
    let settings: Vec<_> = registry["env"]
        .as_array()
        .unwrap()
        .iter()
        .filter(|setting| setting["name"] == RPC_VARIABLE)
        .collect();
    assert_eq!(
        settings.len(),
        1,
        "exactly one committed-chain endpoint is required"
    );
    settings[0]["value"].as_str().unwrap().parse().unwrap()
}

#[test]
fn missing_chain_rpc_deployment_configures_root_tls_and_matching_egress() {
    let documents = deployment();
    let endpoint = rendered_endpoint(&documents);
    assert_eq!(endpoint.scheme_str(), Some("https"));
    assert_eq!(endpoint.host(), Some("api1.mainnet-beta.paxeer.network"));
    assert_eq!(endpoint.path(), "/");
    assert!(endpoint.query().is_none());
    assert_eq!(endpoint.port_u16().unwrap_or(443), 443);

    let policy = documents
        .iter()
        .find(|document| {
            document["kind"] == "NetworkPolicy"
                && document["metadata"]["name"] == "layerx-program-registry"
        })
        .unwrap();
    assert_eq!(policy["metadata"]["namespace"], "layerx-testnet");
    assert_eq!(
        policy["spec"]["podSelector"]["matchLabels"]["app"],
        "layerx-program-registry"
    );
    assert!(policy["spec"]["policyTypes"]
        .as_array()
        .unwrap()
        .contains(&json!("Egress")));
    let egress = policy["spec"]["egress"].as_array().unwrap();
    for cidr in ["0.0.0.0/0", "::/0"] {
        assert!(
            egress.iter().any(|rule| {
                rule["ports"]
                    .as_array()
                    .unwrap()
                    .iter()
                    .any(|port| port["protocol"] == "TCP" && port["port"] == 443)
                    && rule["to"].as_array().unwrap().iter().any(|peer| {
                        peer["ipBlock"]["cidr"] == cidr && peer["ipBlock"].get("except").is_none()
                    })
            }),
            "the configured TLS endpoint needs {cidr} egress on TCP 443"
        );
    }
    for protocol in ["UDP", "TCP"] {
        assert!(
            egress.iter().any(|rule| {
                rule["to"]
                    .as_array()
                    .unwrap()
                    .iter()
                    .any(|peer| peer["namespaceSelector"] == json!({}))
                    && rule["ports"]
                        .as_array()
                        .unwrap()
                        .iter()
                        .any(|port| port["protocol"] == protocol && port["port"] == 53)
            }),
            "the configured hostname needs {protocol} DNS egress"
        );
    }
}

fn listener() -> TcpListener {
    let listener = TcpListener::bind("127.0.0.1:0").unwrap();
    listener.set_nonblocking(true).unwrap();
    listener
}

fn accept(listener: &TcpListener) -> TcpStream {
    let deadline = Instant::now() + Duration::from_secs(10);
    loop {
        match listener.accept() {
            Ok((stream, _)) => {
                stream
                    .set_read_timeout(Some(Duration::from_secs(5)))
                    .unwrap();
                stream
                    .set_write_timeout(Some(Duration::from_secs(5)))
                    .unwrap();
                return stream;
            }
            Err(error)
                if error.kind() == std::io::ErrorKind::WouldBlock && Instant::now() < deadline =>
            {
                thread::sleep(Duration::from_millis(2));
            }
            Err(error) => panic!("bounded loopback accept: {error}"),
        }
    }
}

fn word(value: u64) -> Vec<u8> {
    let mut bytes = vec![0; 24];
    bytes.extend_from_slice(&value.to_be_bytes());
    bytes
}

fn native_empty_head() -> Vec<u8> {
    // Canonical emptyStateHead() from precompiles/layerxprograms/view_registry.go.
    let mut head = vec![0, 1, 0, 0, 0, 0];
    head.extend_from_slice(&Sha256::digest(&head));
    head
}

fn abi_bytes(bytes: &[u8]) -> Vec<u8> {
    let mut encoded = word(32);
    encoded.extend_from_slice(&word(bytes.len().try_into().unwrap()));
    encoded.extend_from_slice(bytes);
    encoded.resize(64 + bytes.len().div_ceil(32) * 32, 0);
    encoded
}

#[test]
fn missing_chain_rpc_deployment_serves_committed_program_reads() {
    let directory = Directory::new();
    let configured = rendered_endpoint(&deployment());
    let chain = listener();
    // Render only the remote origin onto an allowed loopback peer; preserve the shipped path.
    let endpoint = format!(
        "http://{}{}",
        chain.local_addr().unwrap(),
        configured.path_and_query().unwrap()
    );
    let state = native_empty_head();
    let expected_root = hex::encode(&keccak256(&state));
    let expected_state = state.clone();
    let peer = thread::spawn(move || {
        let mut program = word(0); // Immutable program, no upgrade authority.
        program.extend_from_slice(&[0; 32]);
        program.extend_from_slice(&[0x44; 32]);
        program.extend_from_slice(&word(5));
        program.extend_from_slice(&word(1));
        program.extend_from_slice(&word(1));
        let selectors = [
            keccak256(b"getProgram(bytes32)"),
            keccak256(b"getStateHead(bytes32)"),
        ];
        let mut calls = Vec::new();
        for step in 0..6 {
            let mut stream = accept(&chain);
            let message = layerx_platform_internal::http::read_http_message(
                &mut stream,
                layerx_platform_internal::http::MAX_REQUEST_BYTES,
            )
            .unwrap();
            assert_eq!(message.headers.get("").unwrap(), "POST / HTTP/1.1");
            let rpc: Value = serde_json::from_slice(&message.body).unwrap();
            assert_eq!(rpc["jsonrpc"], "2.0");
            assert_eq!(rpc["id"], 1);
            let result = match step {
                0 | 5 => {
                    assert_eq!(rpc["method"], "eth_blockNumber");
                    assert_eq!(rpc["params"], json!([]));
                    json!("0x20")
                }
                1 => {
                    assert_eq!(rpc["method"], "eth_getBlockByNumber");
                    assert_eq!(rpc["params"], json!(["0x20", false]));
                    json!({"number": "0x20", "timestamp": "0x6553f100"})
                }
                2..=4 => {
                    assert_eq!(rpc["method"], "eth_call");
                    let selector = if step == 2 {
                        selectors[0]
                    } else {
                        selectors[1]
                    };
                    assert_eq!(
                        rpc["params"],
                        json!([{
                        "to": "0x000000000000000000000000000000000000101d",
                        "data": format!("0x{}{}", hex::encode(&selector[..4]), hex::encode(&PROGRAM)),
                    }, "0x20"])
                    );
                    let encoded = if step == 2 {
                        program.clone()
                    } else {
                        abi_bytes(&state)
                    };
                    json!(format!("0x{}", hex::encode(&encoded)))
                }
                _ => unreachable!(),
            };
            let response = if step == 5 {
                json!({"jsonrpc": "2.0", "id": 1, "error": {"code": -32000, "message": "committed head unavailable"}})
            } else {
                json!({"jsonrpc": "2.0", "id": 1, "result": result})
            }.to_string();
            write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{}", response.len(), response).unwrap();
            calls.push(rpc);
        }
        calls
    });

    let journal_path = directory.0.join("journal");
    let journal = FileDeploymentJournal::open(journal_path.clone()).unwrap();
    let authority = RegistryAuthority::new(TOKEN.to_owned().into()).unwrap();
    let service = listener();
    let service_url = format!("http://{}/__registry/head", service.local_addr().unwrap());
    let service_endpoint = endpoint.clone();
    let server = thread::spawn(move || {
        for _ in 0..3 {
            let mut stream = accept(&service);
            let request = parse_request(&mut stream).unwrap();
            let response = Registrar::route_chain_head(
                &request,
                &authority,
                &journal,
                &service_endpoint,
                Instant::now() + Duration::from_secs(5),
            );
            write_response(&mut stream, &response).unwrap();
        }
    });
    let agent: ureq::Agent = ureq::Agent::config_builder()
        .http_status_as_error(false)
        .timeout_global(Some(Duration::from_secs(8)))
        .build()
        .into();
    let body = json!({"program_id": hex::encode(&PROGRAM)});
    let response = agent.post(&service_url).send_json(&body).unwrap();
    assert_eq!(response.status(), 401);
    let mut response = agent
        .post(&service_url)
        .header("authorization", format!("Bearer {TOKEN}"))
        .send_json(&body)
        .unwrap();
    assert_eq!(response.status(), 200);
    let observed: Value = response.body_mut().read_json().unwrap();
    assert_eq!(observed["observed"], true);
    assert_eq!(observed["sequence"], 32);
    assert_eq!(observed["observed_at"], 1_700_000_000_000_u64);
    assert_eq!(observed["state_root"], expected_root);

    let cache_path = directory.0.join("program-state");
    let cache = FileProgramStateJournal::open(cache_path.clone()).unwrap();
    assert_eq!(
        cache
            .store(
                &endpoint,
                ProgramId::new(PROGRAM).unwrap(),
                32,
                Instant::now() + Duration::from_secs(5)
            )
            .unwrap(),
        expected_state
    );
    cache.audit().unwrap();
    assert_eq!(fs::read_dir(&cache_path).unwrap().count(), 1);

    let mut response = agent
        .post(&service_url)
        .header("authorization", format!("Bearer {TOKEN}"))
        .send_json(&body)
        .unwrap();
    assert_eq!(response.status(), 503);
    let refusal: Value = response.body_mut().read_json().unwrap();
    assert!(refusal.to_string().contains("protocol_state_unavailable"));
    server.join().unwrap();
    assert_eq!(peer.join().unwrap().len(), 6);
    let reopened = FileDeploymentJournal::open(journal_path).unwrap();
    let retained = reopened.observed_head().unwrap();
    assert_eq!(retained.sequence, 32);
    assert_eq!(retained.observed_at, 1_700_000_000_000_u64);
    FileProgramStateJournal::open(cache_path)
        .unwrap()
        .audit()
        .unwrap();
}
