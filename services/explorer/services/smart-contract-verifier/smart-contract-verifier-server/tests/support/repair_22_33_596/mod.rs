use actix_web::{http::StatusCode, test, App};
use serde_json::{json, Value};
use smart_contract_verifier_proto::blockscout::smart_contract_verifier::v2::{
    sourcify_verifier_actix::route_sourcify_verifier, VerifyResponse,
};
use smart_contract_verifier_server::{Settings, SourcifyVerifierService};
use std::{
    collections::BTreeMap,
    io::{Read, Write},
    net::{TcpListener, TcpStream},
    sync::{
        atomic::{AtomicBool, Ordering},
        Arc,
    },
    thread::{self, JoinHandle},
    time::{Duration, Instant},
};

pub const CHAIN: &str = "125";
pub const ADDRESS: &str = "0x6da5e8cd88641dd371f3ed7737664ea86b3c3ec8";
pub const DIRECT: &str = "/api/v2/verifier/sourcify/sources:verify";
pub const IMPORT: &str = "/api/v2/verifier/sourcify/sources:verify-from-etherscan";
pub const NIGHTLY: &str = "v0.8.14-nightly.2022.4.13+commit.25923c1f";
pub const NIGHTLY_LIST: &str = include_str!("../../fixtures/repair_22_33_596/nightly-list.json");

pub struct Exchange {
    method: &'static str,
    path: String,
    body: Option<Value>,
    status: u16,
    response: Value,
}

impl Exchange {
    pub fn new(
        method: &'static str,
        path: impl Into<String>,
        body: Option<Value>,
        status: u16,
        response: Value,
    ) -> Self {
        Self {
            method,
            path: path.into(),
            body,
            status,
            response,
        }
    }
}

/// A bounded HTTP transcript peer. Only the upstream wire protocol is replayed;
/// the server service, generated route, HTTP clients and decoders are production code.
pub struct Transcript {
    pub url: String,
    stop: Arc<AtomicBool>,
    worker: Option<JoinHandle<usize>>,
    expected: usize,
}

impl Transcript {
    pub fn start(exchanges: Vec<Exchange>) -> Self {
        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        listener.set_nonblocking(true).unwrap();
        let url = format!("http://{}/", listener.local_addr().unwrap());
        let stop = Arc::new(AtomicBool::new(false));
        let stopped = stop.clone();
        let expected = exchanges.len();
        let worker = thread::spawn(move || {
            let mut served = 0;
            let mut exchanges = exchanges.into_iter();
            let deadline = Instant::now() + Duration::from_secs(15);
            while !stopped.load(Ordering::SeqCst) && Instant::now() < deadline {
                match listener.accept() {
                    Ok((mut stream, _)) => {
                        stream
                            .set_read_timeout(Some(Duration::from_secs(3)))
                            .unwrap();
                        stream
                            .set_write_timeout(Some(Duration::from_secs(3)))
                            .unwrap();
                        let exchange = exchanges.next().expect("unexpected upstream request");
                        let (method, path, body) = read_request(&mut stream);
                        assert_eq!(method, exchange.method);
                        assert_eq!(path, exchange.path);
                        assert_eq!(body, exchange.body, "upstream request body");
                        let response = exchange.response.to_string();
                        write!(stream, "HTTP/1.1 {} Fixture\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{}", exchange.status, response.len(), response).unwrap();
                        served += 1;
                    }
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        thread::sleep(Duration::from_millis(2))
                    }
                    Err(error) => panic!("transcript accept: {error}"),
                }
            }
            served
        });
        Self {
            url,
            stop,
            worker: Some(worker),
            expected,
        }
    }

    pub fn finish(mut self) {
        self.stop.store(true, Ordering::SeqCst);
        assert_eq!(
            self.worker
                .take()
                .unwrap()
                .join()
                .expect("transcript peer assertions"),
            self.expected,
            "all upstream exchanges must execute"
        );
    }
}

impl Drop for Transcript {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::SeqCst);
        if let Some(worker) = self.worker.take() {
            let _ = worker.join();
        }
    }
}

fn read_request(stream: &mut TcpStream) -> (String, String, Option<Value>) {
    let mut bytes = Vec::new();
    let mut buffer = [0; 4096];
    let header_end = loop {
        let size = stream.read(&mut buffer).unwrap();
        assert!(size > 0, "request ended before headers");
        bytes.extend_from_slice(&buffer[..size]);
        assert!(bytes.len() < 1_048_576, "bounded fixture request");
        if let Some(position) = bytes.windows(4).position(|v| v == b"\r\n\r\n") {
            break position + 4;
        }
    };
    let headers = std::str::from_utf8(&bytes[..header_end]).unwrap();
    let mut request = headers.lines().next().unwrap().split_whitespace();
    let method = request.next().unwrap().to_owned();
    let path = request.next().unwrap().to_owned();
    let length: usize = headers
        .lines()
        .filter_map(|line| line.split_once(':'))
        .find(|(name, _)| name.eq_ignore_ascii_case("content-length"))
        .map(|(_, value)| value.trim().parse().unwrap())
        .unwrap_or(0);
    assert!(length < 1_048_576);
    while bytes.len() < header_end + length {
        let size = stream.read(&mut buffer).unwrap();
        assert!(size > 0, "request ended before body");
        bytes.extend_from_slice(&buffer[..size]);
    }
    let body = (length > 0)
        .then(|| serde_json::from_slice(&bytes[header_end..header_end + length]).unwrap());
    (method, path, body)
}

pub fn fixture(import: bool) -> Value {
    serde_json::from_str(if import {
        include_str!("../../fixtures/repair_22_33_596/c1.json")
    } else {
        include_str!("../../fixtures/repair_22_33_596/storage.json")
    })
    .unwrap()
}

pub fn files_response(import: bool, full: bool) -> Value {
    let fixture = fixture(import);
    let file_name = if import {
        "C1.sol"
    } else {
        "contracts/1_Storage.sol"
    };
    let prefix = format!(
        "contracts/{}_match/{CHAIN}/{ADDRESS}",
        if full { "full" } else { "partial" }
    );
    json!({"status": if full { "full" } else { "partial" }, "files": [
        {"name":"metadata.json", "path":format!("{prefix}/metadata.json"), "content":fixture["metadata"].to_string()},
        {"name":file_name, "path":format!("{prefix}/sources/{file_name}"), "content":fixture["source"]}
    ]})
}

pub fn direct_request() -> Value {
    let fixture = fixture(false);
    json!({"address":ADDRESS, "chain":CHAIN, "files":{
        "source.sol":fixture["source"], "metadata.json":fixture["metadata"].to_string()
    }})
}

pub fn chains(supported: bool, etherscan_api: bool) -> Exchange {
    Exchange::new(
        "GET",
        "/chains",
        None,
        200,
        json!([
            {"chainId":125,"supported":supported,"etherscanAPI":etherscan_api},
            {"chainId":2221,"supported":true,"etherscanAPI":false}
        ]),
    )
}

pub fn source_exchange(import: bool, full: bool) -> Exchange {
    Exchange::new(
        "GET",
        format!("/files/any/{CHAIN}/{ADDRESS}"),
        None,
        200,
        files_response(import, full),
    )
}

pub async fn call(route: &str, request: Value, exchanges: Vec<Exchange>) -> (StatusCode, Vec<u8>) {
    let transcript = Transcript::start(exchanges);
    let mut settings = Settings::default();
    settings.sourcify.api_url = transcript.url.parse().unwrap();
    settings.sourcify.request_timeout = 2;
    settings.sourcify.verification_attempts = std::num::NonZeroU32::new(1).unwrap();
    let service = Arc::new(
        SourcifyVerifierService::new(settings.sourcify)
            .await
            .unwrap(),
    );
    let app = test::init_service(
        App::new().configure(|config| route_sourcify_verifier(config, service.clone())),
    )
    .await;
    let response = test::TestRequest::post()
        .uri(route)
        .set_json(request)
        .send_request(&app)
        .await;
    let status = response.status();
    let body = test::read_body(response).await.to_vec();
    transcript.finish();
    (status, body)
}

pub fn assert_success(body: &[u8], import: bool, full: bool) {
    let response: VerifyResponse = serde_json::from_slice(body).unwrap();
    assert_eq!(response.status().as_str_name(), "SUCCESS");
    assert_eq!(response.message, "OK");
    let source = response.source.unwrap();
    let fixture = fixture(import);
    let file = if import {
        "C1.sol"
    } else {
        "contracts/1_Storage.sol"
    };
    assert_eq!(source.file_name, file);
    assert_eq!(source.contract_name, if import { "C1" } else { "Storage" });
    assert_eq!(source.compiler_version, "0.8.7+commit.e28d00a7");
    assert_eq!(source.source_type().as_str_name(), "SOLIDITY");
    assert_eq!(
        source.match_type().as_str_name(),
        if full { "FULL" } else { "PARTIAL" }
    );
    assert_eq!(
        source.source_files,
        BTreeMap::from([(
            file.to_owned(),
            fixture["source"].as_str().unwrap().to_owned()
        )])
    );
    assert_eq!(
        serde_json::from_str::<Value>(source.abi.as_ref().unwrap()).unwrap(),
        fixture["metadata"]["output"]["abi"]
    );
    // compilationTarget identifies the selected contract; the public compiler settings
    // omit it, as required by the current production metadata conversion.
    let settings = json!({"evmVersion":"london","libraries":{},"metadata":{"bytecodeHash":"ipfs"},"optimizer":{"enabled":false,"runs":200},"remappings":[]});
    assert_eq!(
        serde_json::from_str::<Value>(&source.compiler_settings).unwrap(),
        settings
    );
    assert!(source.constructor_arguments.is_none());
    assert!(source.compilation_artifacts.is_none());
    assert!(source.creation_input_artifacts.is_none());
    assert!(source.deployed_bytecode_artifacts.is_none());
    assert!(source.libraries.is_empty());
    assert!(!source.is_blueprint);
    let extra = response.extra_data.unwrap();
    assert!(extra.local_creation_input_parts.is_empty());
    assert!(extra.local_deployed_bytecode_parts.is_empty());
    assert!(response
        .post_action_responses
        .unwrap()
        .lookup_methods
        .is_none());
}

pub fn assert_failure(body: &[u8], message: &str) {
    let response: VerifyResponse = serde_json::from_slice(body).unwrap();
    assert_eq!(response.status().as_str_name(), "FAILURE");
    assert!(response.message.contains(message), "{}", response.message);
    assert!(response.source.is_none());
    assert!(response.extra_data.is_none());
    assert!(response.post_action_responses.is_none());
}

pub async fn nightly_service() -> (
    Arc<smart_contract_verifier_server::SolidityVerifierService>,
    tempfile::TempDir,
) {
    let catalogue = Transcript::start(vec![Exchange::new(
        "GET",
        "/list.json",
        None,
        200,
        serde_json::from_str(NIGHTLY_LIST).unwrap(),
    )]);
    let directory = tempfile::tempdir().unwrap();
    let settings: Settings = serde_json::from_value(json!({"solidity":{
        "compilers_dir":directory.path(),
        "fetcher":{"list":{"list_url":format!("{}list.json", catalogue.url)}},
        "refresh_versions_schedule":"0 0 0 1 1 * 2099"
    }}))
    .unwrap();
    let service = Arc::new(
        smart_contract_verifier_server::SolidityVerifierService::new(
            settings.solidity,
            Arc::new(tokio::sync::Semaphore::new(1)),
        )
        .await
        .unwrap(),
    );
    catalogue.finish();
    (service, directory)
}
