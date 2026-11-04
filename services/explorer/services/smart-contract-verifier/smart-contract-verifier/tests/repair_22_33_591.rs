use bytes::Bytes;
use serde_json::{json, Value};
use smart_contract_verifier::{
    sourcify::api::{self, VerificationRequest, VerifyFromEtherscanRequest},
    MatchType, SolcInput, SourcifyApiClient, SourcifySuccess,
};
use std::{
    collections::BTreeMap,
    io::{Read, Write},
    net::TcpListener,
    num::NonZeroU32,
    sync::Arc,
    thread,
    time::{Duration, Instant},
};

const ADDRESS: &str = "0x5d3A6C34Ef73C557958f41A3Bd084316Edf288Cb";
const LIB_A: &str = "0xcafecafecafecafecafecafecafecafecafecafe";
const LIB_B: &str = "0xcafecafecafecafecafecafecafecafecafecaf1";
const TRANSCRIPT: &str = include_str!("../src/sourcify/fixtures/repair_22_33_591_files.json");

struct Exchange {
    method: &'static str,
    path: String,
    body: Option<Value>,
    response: Value,
}

struct FixtureServer {
    client: Arc<SourcifyApiClient>,
    thread: thread::JoinHandle<()>,
}

impl FixtureServer {
    fn start(exchanges: Vec<Exchange>) -> Self {
        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        let host = format!("http://{}/", listener.local_addr().unwrap());
        listener.set_nonblocking(true).unwrap();
        let thread = thread::spawn(move || {
            for exchange in exchanges {
                let deadline = Instant::now() + Duration::from_secs(5);
                let mut stream = loop {
                    match listener.accept() {
                        Ok((stream, _)) => break stream,
                        Err(err) if err.kind() == std::io::ErrorKind::WouldBlock => {
                            assert!(
                                Instant::now() < deadline,
                                "missing {} {}",
                                exchange.method,
                                exchange.path
                            );
                            thread::sleep(Duration::from_millis(2));
                        }
                        Err(err) => panic!("accept: {err}"),
                    }
                };
                stream
                    .set_read_timeout(Some(Duration::from_secs(3)))
                    .unwrap();
                stream
                    .set_write_timeout(Some(Duration::from_secs(3)))
                    .unwrap();
                let mut request = Vec::new();
                let header_end = loop {
                    let mut buffer = [0; 4096];
                    let count = stream.read(&mut buffer).unwrap();
                    assert_ne!(count, 0, "request ended before headers");
                    request.extend_from_slice(&buffer[..count]);
                    if let Some(index) = request.windows(4).position(|part| part == b"\r\n\r\n") {
                        break index + 4;
                    }
                };
                let headers = String::from_utf8(request[..header_end].to_vec()).unwrap();
                let mut request_line = headers.lines().next().unwrap().split_whitespace();
                assert_eq!(request_line.next(), Some(exchange.method));
                assert_eq!(request_line.next(), Some(exchange.path.as_str()));
                let length = headers
                    .lines()
                    .filter_map(|line| line.split_once(':'))
                    .find(|(name, _)| name.eq_ignore_ascii_case("content-length"))
                    .map(|(_, value)| value.trim().parse::<usize>().unwrap())
                    .unwrap_or(0);
                while request.len() < header_end + length {
                    let mut buffer = [0; 4096];
                    let count = stream.read(&mut buffer).unwrap();
                    assert_ne!(count, 0, "request ended before body");
                    request.extend_from_slice(&buffer[..count]);
                }
                if let Some(expected) = exchange.body {
                    let actual: Value =
                        serde_json::from_slice(&request[header_end..header_end + length]).unwrap();
                    assert_eq!(actual, expected);
                } else {
                    assert_eq!(length, 0);
                }
                let body = exchange.response.to_string();
                write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{}", body.len(), body).unwrap();
            }
        });
        let client = Arc::new(
            SourcifyApiClient::new(host.parse().unwrap(), 5, NonZeroU32::new(1).unwrap()).unwrap(),
        );
        Self { client, thread }
    }

    fn finish(self) {
        self.thread
            .join()
            .expect("fixture HTTP transcript must be consumed exactly");
    }
}

fn metadata(transcript: &Value) -> Value {
    let file = transcript["files"]
        .as_array()
        .unwrap()
        .iter()
        .find(|file| file["name"] == "metadata.json")
        .unwrap();
    serde_json::from_str(file["content"].as_str().unwrap()).unwrap()
}

fn change_metadata(transcript: &mut Value, change: impl FnOnce(&mut Value)) {
    let mut value = metadata(transcript);
    change(&mut value);
    let file = transcript["files"]
        .as_array_mut()
        .unwrap()
        .iter_mut()
        .find(|file| file["name"] == "metadata.json")
        .unwrap();
    file["content"] = json!(value.to_string());
}

fn sources(transcript: &Value) -> BTreeMap<String, String> {
    transcript["files"]
        .as_array()
        .unwrap()
        .iter()
        .filter_map(|file| {
            file["path"]
                .as_str()
                .unwrap()
                .split_once("/sources/")
                .map(|(_, path)| {
                    (
                        path.to_string(),
                        file["content"].as_str().unwrap().to_string(),
                    )
                })
        })
        .collect()
}

fn request(transcript: &Value) -> VerificationRequest {
    let mut files = sources(transcript);
    files.insert("metadata.json".into(), metadata(transcript).to_string());
    VerificationRequest {
        address: ADDRESS.into(),
        chain: "77".into(),
        files,
        chosen_contract: None,
    }
}

fn server(transcript: &Value) -> FixtureServer {
    let req = request(transcript);
    FixtureServer::start(vec![
        Exchange {
            method: "POST",
            path: "/".into(),
            body: Some(json!({"address": ADDRESS, "chain": "77", "files": req.files})),
            response: json!({"result": [{"address": ADDRESS, "status": "perfect"}]}),
        },
        Exchange {
            method: "GET",
            path: format!("/files/any/77/{ADDRESS}"),
            body: None,
            response: transcript.clone(),
        },
        Exchange {
            method: "GET",
            path: "/chains".into(),
            body: None,
            response: json!([{"chainId": 77, "supported": true, "etherscanAPI": true}]),
        },
        Exchange {
            method: "POST",
            path: "/verify/etherscan".into(),
            body: Some(json!({"chainId": "77", "address": ADDRESS.to_lowercase()})),
            response: json!({"result": [{"address": ADDRESS, "chainId": "77", "status": "perfect", "libraryMap": {}, "immutableReferences": null}]}),
        },
        Exchange {
            method: "GET",
            path: format!("/files/any/77/{}", ADDRESS.to_lowercase()),
            body: None,
            response: transcript.clone(),
        },
    ])
}

async fn both_paths(transcript: &Value) -> (SourcifySuccess, SourcifySuccess) {
    let server = server(transcript);
    let direct = api::verify(server.client.clone(), request(transcript))
        .await
        .unwrap();
    let imported = api::verify_from_etherscan(
        server.client.clone(),
        VerifyFromEtherscanRequest {
            address: Bytes::from(hex::decode(&ADDRESS[2..]).unwrap()),
            chain: "77".into(),
        },
    )
    .await
    .unwrap();
    server.finish();
    (direct, imported)
}

fn assert_compiler_input(success: &SourcifySuccess, transcript: &Value, libraries: Value) {
    let metadata = metadata(transcript);
    let settings: Value = serde_json::from_str(&success.compiler_settings).unwrap();
    assert!(settings.get("compilationTarget").is_none());
    let mut expected = metadata["settings"].clone();
    expected
        .as_object_mut()
        .unwrap()
        .remove("compilationTarget");
    expected["libraries"] = libraries.clone();
    assert_eq!(
        settings, expected,
        "all other compiler settings must survive unchanged"
    );
    assert_eq!(success.sources, sources(transcript));
    assert_eq!(success.file_name, "contracts/Main.sol");
    assert_eq!(success.contract_name, "Main");
    assert_eq!(
        success.compiler_version,
        metadata["compiler"]["version"].as_str().unwrap()
    );
    assert_eq!(
        success.evm_version.as_deref(),
        metadata["settings"]["evmVersion"].as_str()
    );
    assert_eq!(success.optimization, Some(false));
    assert_eq!(success.optimization_runs, Some(200));
    assert_eq!(
        success.contract_libraries,
        serde_json::from_value::<BTreeMap<String, String>>(
            metadata["settings"]["libraries"].clone()
        )
        .unwrap()
    );
    assert_eq!(
        serde_json::from_str::<Value>(&success.abi).unwrap(),
        metadata["output"]["abi"]
    );
    assert_eq!(success.match_type, MatchType::Full);

    let source_input: BTreeMap<_, _> = success
        .sources
        .iter()
        .map(|(path, content)| (path, json!({"content": content})))
        .collect();
    let input: SolcInput = serde_json::from_value(
        json!({"language": "Solidity", "sources": source_input, "settings": settings}),
    )
    .expect("returned sources/settings must enter the production solc standard-JSON codec");
    assert_eq!(input.0.sources.len(), success.sources.len());
    for (path, source) in &input.0.sources {
        assert_eq!(
            source.content.as_str(),
            success.sources[path.to_str().unwrap()]
        );
    }
    assert_eq!(
        serde_json::to_value(&input.0.settings.libraries).unwrap(),
        libraries
    );
    let encoded = serde_json::to_string(&input).unwrap();
    let roundtrip: SolcInput = serde_json::from_str(&encoded).unwrap();
    assert_eq!(
        serde_json::to_value(roundtrip).unwrap(),
        serde_json::to_value(input).unwrap()
    );
}

#[tokio::test]
async fn sourcify_settings_shape_legacy_libraries_both_paths() {
    let transcript: Value = serde_json::from_str(TRANSCRIPT).unwrap();
    let (direct, imported) = both_paths(&transcript).await;
    assert_eq!(direct, imported);
    for success in [&direct, &imported] {
        assert_compiler_input(
            success,
            &transcript,
            json!({"": {"LibA": LIB_A, "LibB": LIB_B}}),
        );
    }
}

#[tokio::test]
async fn sourcify_settings_shape_qualified_libraries_both_paths() {
    let mut transcript: Value = serde_json::from_str(TRANSCRIPT).unwrap();
    change_metadata(&mut transcript, |metadata| {
        metadata["settings"]["libraries"] = json!({"contracts/LibA.sol:LibA": LIB_A, "contracts/LibB.sol:LibB": LIB_B, "contracts/LibA.sol:Other": LIB_B});
        metadata["settings"]["viaIR"] = json!(true);
        metadata["settings"]["optimizer"]["details"] = json!({"yul": true});
        metadata["settings"]["remappings"] = json!(["@contracts/=contracts/"]);
    });
    let (direct, imported) = both_paths(&transcript).await;
    assert_eq!(direct, imported);
    for success in [&direct, &imported] {
        assert_compiler_input(
            success,
            &transcript,
            json!({"contracts/LibA.sol": {"LibA": LIB_A, "Other": LIB_B}, "contracts/LibB.sol": {"LibB": LIB_B}}),
        );
    }
}

#[tokio::test]
async fn sourcify_settings_shape_empty_libraries_and_constructor_arguments() {
    let mut transcript: Value = serde_json::from_str(TRANSCRIPT).unwrap();
    change_metadata(&mut transcript, |metadata| {
        metadata["settings"]["libraries"] = json!({})
    });
    transcript["files"].as_array_mut().unwrap().push(json!({"name": "constructor-args.txt", "path": format!("/home/data/repository/contracts/full_match/77/{ADDRESS}/constructor-args.txt"), "content": "000000000000000000000000000000000000000000000000000000000000002a"}));
    let (direct, mut imported) = both_paths(&transcript).await;
    assert_eq!(direct.constructor_arguments, None);
    assert_eq!(
        imported.constructor_arguments.take(),
        Some(Bytes::from(
            hex::decode("000000000000000000000000000000000000000000000000000000000000002a")
                .unwrap()
        ))
    );
    assert_eq!(direct, imported);
    for success in [&direct, &imported] {
        assert_compiler_input(success, &transcript, json!({}));
    }
}

#[tokio::test]
async fn sourcify_settings_shape_invalid_metadata_is_rejected_by_both_paths() {
    for mutation in [json!({}), json!({"contracts/Main.sol": "Main"})] {
        let mut transcript: Value = serde_json::from_str(TRANSCRIPT).unwrap();
        change_metadata(&mut transcript, |metadata| {
            metadata["settings"]["compilationTarget"] = mutation.clone();
            if !mutation.as_object().unwrap().is_empty() {
                metadata["settings"]["libraries"] = json!({"LibA": {"invalid": LIB_A}});
            }
        });
        let server = server(&transcript);
        assert!(api::verify(server.client.clone(), request(&transcript))
            .await
            .is_err());
        assert!(api::verify_from_etherscan(
            server.client.clone(),
            VerifyFromEtherscanRequest {
                address: Bytes::from(hex::decode(&ADDRESS[2..]).unwrap()),
                chain: "77".into(),
            }
        )
        .await
        .is_err());
        server.finish();
    }
}
