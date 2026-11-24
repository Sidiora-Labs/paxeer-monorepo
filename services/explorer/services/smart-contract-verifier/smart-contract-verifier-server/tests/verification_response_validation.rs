#![allow(dead_code, unused_imports)]

// Include the production conversion modules because the server keeps `types` private.
#[path = "../src/proto.rs"]
mod proto;
#[path = "../src/types/mod.rs"]
mod types;

use actix_web::test as http_test;
use actix_web::{http::StatusCode, web, App, HttpRequest, HttpResponse, HttpServer};
use proto::{
    solidity_verifier_actix::route_solidity_verifier,
    solidity_verifier_client::SolidityVerifierClient,
    solidity_verifier_server::SolidityVerifierServer,
    sourcify_verifier_actix::route_sourcify_verifier,
    sourcify_verifier_client::SourcifyVerifierClient,
    sourcify_verifier_server::SourcifyVerifierServer,
};
use serde_json::{json, Value};
use smart_contract_verifier::{FullyQualifiedName, Language, VerifyingContract};
use smart_contract_verifier_server::{Settings, SolidityVerifierService, SourcifyVerifierService};
use std::{
    collections::BTreeMap,
    net::TcpListener,
    sync::{Arc, Mutex},
    time::Duration,
};
use tokio::sync::{oneshot, Semaphore};
use tonic::{
    transport::{Channel, Server},
    Code,
};
use verification_common::verifier_alliance::{
    self, CompilationArtifacts, CreationCodeArtifacts, RuntimeCodeArtifacts,
};

const ADDRESS: &str = "0x0123456789abcdef0123456789abcdef01234567";
const VERSION: &str = "v0.6.12+commit.27d51765";
const SOURCIFY: &str = "/api/v2/verifier/sourcify/sources:";
const SOLIDITY: &str = "/api/v2/verifier/solidity/sources:";
const CACHE_BYTES: &[u8] = b"cache failure fixture";

#[derive(Clone, Default)]
struct Exchanges(Arc<Mutex<Vec<(String, Value)>>>);

async fn fixture_response(
    request: HttpRequest,
    body: web::Bytes,
    exchanges: web::Data<Exchanges>,
) -> HttpResponse {
    let payload = if body.is_empty() {
        Value::Null
    } else {
        serde_json::from_slice(&body).expect("upstream receives JSON")
    };
    exchanges
        .0
        .lock()
        .unwrap()
        .push((request.path().to_owned(), payload));
    match request.path() {
        "/list.json" => HttpResponse::Ok().json(json!({"builds": [{
            "path": "compiler", "longVersion": VERSION,
            "sha256": "c1502e0208040e0eb8d763916c481d5e54893506e82302f2dc3437bde6577d69"
        }]})),
        "/compiler" => HttpResponse::Ok().body(CACHE_BYTES),
        "/chains" => HttpResponse::Ok().json(json!([])),
        "/" => HttpResponse::Ok().json(json!({"error": "recorded verification rejection"})),
        _ => HttpResponse::NotFound().finish(),
    }
}

struct FixturePeer {
    url: String,
    exchanges: Exchanges,
    handle: actix_web::dev::ServerHandle,
}

impl FixturePeer {
    async fn start() -> Self {
        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        let url = format!("http://{}/", listener.local_addr().unwrap());
        let exchanges = Exchanges::default();
        let data = exchanges.clone();
        let server = HttpServer::new(move || {
            App::new()
                .app_data(web::Data::new(data.clone()))
                .default_service(web::to(fixture_response))
        })
        .workers(1)
        .listen(listener)
        .unwrap()
        .run();
        let handle = server.handle();
        actix_web::rt::spawn(server);
        Self {
            url,
            exchanges,
            handle,
        }
    }

    fn requests(&self) -> Vec<(String, Value)> {
        self.exchanges.0.lock().unwrap().clone()
    }

    async fn stop(self) {
        self.handle.stop(true).await;
    }
}

async fn sourcify_service(peer: &FixturePeer) -> Arc<SourcifyVerifierService> {
    let settings: Settings = serde_json::from_value(json!({"sourcify": {
        "api_url": peer.url, "request_timeout": 2, "verification_attempts": 1
    }}))
    .unwrap();
    Arc::new(
        SourcifyVerifierService::new(settings.sourcify)
            .await
            .unwrap(),
    )
}

async fn grpc_channel(address: std::net::SocketAddr) -> Channel {
    tokio::time::timeout(Duration::from_secs(5), async {
        loop {
            if let Ok(channel) =
                tonic::transport::Endpoint::from_shared(format!("http://{address}"))
                    .unwrap()
                    .connect()
                    .await
            {
                break channel;
            }
            tokio::time::sleep(Duration::from_millis(10)).await;
        }
    })
    .await
    .expect("real gRPC listener starts")
}

async fn sourcify_grpc(
    service: Arc<SourcifyVerifierService>,
) -> (
    SourcifyVerifierClient<Channel>,
    oneshot::Sender<()>,
    tokio::task::JoinHandle<()>,
) {
    let listener = TcpListener::bind("127.0.0.1:0").unwrap();
    let address = listener.local_addr().unwrap();
    drop(listener);
    let (send, receive) = oneshot::channel();
    let server = tokio::spawn(async move {
        Server::builder()
            .add_service(SourcifyVerifierServer::from_arc(service))
            .serve_with_shutdown(address, async {
                let _ = receive.await;
            })
            .await
            .unwrap();
    });
    (
        SourcifyVerifierClient::new(grpc_channel(address).await),
        send,
        server,
    )
}

fn sourcify_request(address: &str, index: Option<i32>) -> proto::VerifySourcifyRequest {
    proto::VerifySourcifyRequest {
        address: address.into(),
        chain: "1".into(),
        files: BTreeMap::new(),
        chosen_contract: index,
    }
}

#[actix_web::test]
async fn negative_sourcify_contract_index_http_and_grpc() {
    let peer = FixturePeer::start().await;
    let service = sourcify_service(&peer).await;
    let app = http_test::init_service(
        App::new().configure(|config| route_sourcify_verifier(config, service.clone())),
    )
    .await;
    let (mut grpc, shutdown, server) = sourcify_grpc(service).await;
    for index in [-1, i32::MIN] {
        let request = sourcify_request(ADDRESS, Some(index));
        let response = http_test::TestRequest::post()
            .uri(&format!("{SOURCIFY}verify"))
            .set_json(&request)
            .send_request(&app)
            .await;
        assert_eq!(response.status(), StatusCode::BAD_REQUEST);
        let body: Value = http_test::read_body_json(response).await;
        assert_eq!(body["code"], 3);
        assert_eq!(
            body["message"],
            "chosen_contract must be between 0 and 2147483647"
        );
        let error = grpc.verify(request).await.unwrap_err();
        assert_eq!(error.code(), Code::InvalidArgument);
        assert_eq!(
            error.message(),
            "chosen_contract must be between 0 and 2147483647"
        );
        assert!(
            peer.requests().is_empty(),
            "invalid index must not reach Sourcify"
        );
    }
    for index in [Some(0), Some(i32::MAX), None] {
        let request = sourcify_request(ADDRESS, index);
        let response = http_test::TestRequest::post()
            .uri(&format!("{SOURCIFY}verify"))
            .set_json(&request)
            .send_request(&app)
            .await;
        assert_eq!(response.status(), StatusCode::OK);
        let response: proto::VerifyResponse = http_test::read_body_json(response).await;
        assert_eq!(response.status(), proto::verify_response::Status::Failure);
        assert_eq!(response.message, "recorded verification rejection");
        let response = grpc.verify(request).await.unwrap().into_inner();
        assert_eq!(response.status(), proto::verify_response::Status::Failure);
        assert_eq!(response.message, "recorded verification rejection");
        let requests = peer.requests();
        for (path, payload) in requests.iter().rev().take(2) {
            assert_eq!(path, "/");
            assert_eq!(payload["address"], ADDRESS);
            assert_eq!(payload["chain"], "1");
            assert_eq!(
                payload["chosenContract"],
                index
                    .map(|index| Value::String(index.to_string()))
                    .unwrap_or(Value::Null)
            );
        }
    }
    assert_eq!(peer.requests().len(), 6);
    shutdown.send(()).unwrap();
    server.await.unwrap();
    peer.stop().await;
}

#[actix_web::test]
async fn sourcify_address_length_http_and_grpc() {
    let peer = FixturePeer::start().await;
    let service = sourcify_service(&peer).await;
    let app = http_test::init_service(
        App::new().configure(|config| route_sourcify_verifier(config, service.clone())),
    )
    .await;
    let (mut grpc, shutdown, server) = sourcify_grpc(service).await;
    let invalid = [
        String::new(),
        "0x01".into(),
        format!("0x{}", "11".repeat(19)),
        format!("0x{}", "11".repeat(21)),
        format!("0x{}gg", "11".repeat(19)),
        format!("0x{}1", "11".repeat(19)),
    ];
    for address in invalid {
        let direct = sourcify_request(&address, Some(0));
        let import = proto::VerifyFromEtherscanSourcifyRequest {
            address,
            chain: "1".into(),
        };
        for (route, payload) in [
            ("verify", serde_json::to_value(&direct).unwrap()),
            (
                "verify-from-etherscan",
                serde_json::to_value(&import).unwrap(),
            ),
        ] {
            let response = http_test::TestRequest::post()
                .uri(&format!("{SOURCIFY}{route}"))
                .set_json(payload)
                .send_request(&app)
                .await;
            assert_eq!(response.status(), StatusCode::BAD_REQUEST);
            let body: Value = http_test::read_body_json(response).await;
            assert_eq!(body["code"], 3);
            assert_eq!(
                body["message"],
                "address must be a hex-encoded 20-byte EVM address"
            );
        }
        for error in [
            grpc.verify(direct).await.unwrap_err(),
            grpc.verify_from_etherscan(import).await.unwrap_err(),
        ] {
            assert_eq!(error.code(), Code::InvalidArgument);
            assert_eq!(
                error.message(),
                "address must be a hex-encoded 20-byte EVM address"
            );
        }
        assert!(
            peer.requests().is_empty(),
            "invalid address must not reach either upstream route"
        );
    }
    let direct = sourcify_request(ADDRESS, Some(0));
    let response = http_test::TestRequest::post()
        .uri(&format!("{SOURCIFY}verify"))
        .set_json(&direct)
        .send_request(&app)
        .await;
    assert_eq!(response.status(), StatusCode::OK);
    grpc.verify(direct).await.unwrap();
    let import = proto::VerifyFromEtherscanSourcifyRequest {
        address: ADDRESS.into(),
        chain: "1".into(),
    };
    let response = http_test::TestRequest::post()
        .uri(&format!("{SOURCIFY}verify-from-etherscan"))
        .set_json(&import)
        .send_request(&app)
        .await;
    assert_eq!(response.status(), StatusCode::OK);
    let response: proto::VerifyResponse = http_test::read_body_json(response).await;
    assert_eq!(
        response.message,
        "Chain 1 is not supported for importing from Etherscan"
    );
    let response = grpc
        .verify_from_etherscan(import)
        .await
        .unwrap()
        .into_inner();
    assert_eq!(
        response.message,
        "Chain 1 is not supported for importing from Etherscan"
    );
    assert_eq!(
        peer.requests()
            .iter()
            .map(|(path, _)| path.as_str())
            .collect::<Vec<_>>(),
        ["/", "/", "/chains", "/chains"]
    );
    shutdown.send(()).unwrap();
    server.await.unwrap();
    peer.stop().await;
}

fn proxy_match() -> VerifyingContract {
    let fixture: Value = serde_json::from_str(include_str!(
        "test_cases_solidity/both_partial_and_full_verification_sources.json"
    ))
    .unwrap();
    let decode =
        |key: &str| hex::decode(fixture[key].as_str().unwrap().trim_start_matches("0x")).unwrap();
    let creation_code_artifacts: CreationCodeArtifacts =
        serde_json::from_value(fixture["expected_creation_input_artifacts"].clone()).unwrap();
    let runtime_code_artifacts: RuntimeCodeArtifacts =
        serde_json::from_value(fixture["expected_deployed_bytecode_artifacts"].clone()).unwrap();
    let compilation_artifacts: CompilationArtifacts =
        serde_json::from_value(fixture["expected_compiler_artifacts"].clone()).unwrap();
    let recorded_creation_code = decode("creation_bytecode");
    let runtime_code = decode("deployed_bytecode");
    // This fixture appends a raw 20-byte implementation address. The compiled
    // creation code also has a revert string AFTER CBOR; preserve that code and
    // ABI-encode the address as a 32-byte constructor word for the creation proof.
    let (compiled_creation, implementation) =
        recorded_creation_code.split_at(recorded_creation_code.len() - 20);
    assert_eq!(compiled_creation.len(), 2109);
    assert!(
        compiled_creation.ends_with(b"Cannot set a proxy implementation to a non-contract address")
    );
    let compiled_creation_code = compiled_creation.to_vec();
    let mut creation_code = compiled_creation_code.clone();
    creation_code.extend_from_slice(&[0u8; 12]);
    creation_code.extend_from_slice(implementation);
    let creation_match = verifier_alliance::verify_creation_code(
        &creation_code,
        compiled_creation_code.clone(),
        &creation_code_artifacts,
        &compilation_artifacts,
    )
    .unwrap()
    .unwrap();
    let runtime_match = verifier_alliance::verify_runtime_code(
        &runtime_code,
        runtime_code.clone(),
        &runtime_code_artifacts,
    )
    .unwrap()
    .unwrap();
    assert!(creation_match.metadata_match && runtime_match.metadata_match);
    VerifyingContract {
        fully_qualified_name: FullyQualifiedName::from_file_and_contract_names(
            fixture["file_name"].as_str().unwrap(),
            fixture["contract_name"].as_str().unwrap(),
        ),
        language: Language::Solidity,
        compiler_version: VERSION.into(),
        compiler_settings: fixture["input"]["settings"].clone(),
        sources: fixture["input"]["sources"]
            .as_object()
            .unwrap()
            .iter()
            .map(|(name, source)| (name.clone(), source["content"].as_str().unwrap().into()))
            .collect(),
        creation_code: compiled_creation_code,
        runtime_code,
        compilation_artifacts,
        creation_code_artifacts,
        runtime_code_artifacts,
        creation_match: Some(creation_match),
        runtime_match: Some(runtime_match),
        is_blueprint: false,
    }
}

#[test]
fn batch_selects_best_match_through_response_codec() {
    let full = proxy_match();
    let mut partial = full.clone();
    // A different compilation's metadata still verifies partially against the real deployed proxy.
    let auxdata = partial
        .runtime_code_artifacts
        .cbor_auxdata
        .as_mut()
        .unwrap()
        .values_mut()
        .next()
        .unwrap();
    let offset = auxdata.offset as usize;
    let mut changed_metadata = auxdata.value.to_vec();
    changed_metadata[10] ^= 1;
    partial.runtime_code[offset..offset + changed_metadata.len()]
        .copy_from_slice(&changed_metadata);
    auxdata.value = changed_metadata.into();
    partial.runtime_match = verifier_alliance::verify_runtime_code(
        &full.runtime_code,
        partial.runtime_code.clone(),
        &partial.runtime_code_artifacts,
    )
    .unwrap();
    partial.creation_match = None;
    assert!(!partial.runtime_match.as_ref().unwrap().metadata_match);
    assert!(!partial
        .runtime_match
        .as_ref()
        .unwrap()
        .transformations
        .is_empty());
    for matches in [
        vec![partial.clone(), full.clone()],
        vec![full.clone(), partial.clone()],
    ] {
        let solo =
            types::verification_result::process_verification_result(matches.clone(), &[]).unwrap();
        let solo: proto::VerifyResponse =
            serde_json::from_slice(&serde_json::to_vec(&solo).unwrap()).unwrap();
        let solo = solo.source.unwrap();
        let batch =
            types::batch_verification::process_verification_results(vec![matches, vec![]]).unwrap();
        let batch: proto::BatchVerifyResponse =
            serde_json::from_slice(&serde_json::to_vec(&batch).unwrap()).unwrap();
        let Some(proto::batch_verify_response::VerificationResult::ContractVerificationResults(
            results,
        )) = batch.verification_result
        else {
            panic!("contract results expected")
        };
        let Some(proto::contract_verification_result::VerificationResult::Success(success)) =
            &results.items[0].verification_result
        else {
            panic!("verified proxy expected")
        };
        assert_eq!(success.file_name, "contracts/v1/FiatTokenProxy.sol");
        assert_eq!(success.contract_name, "FiatTokenProxy");
        assert_eq!(success.file_name, solo.file_name);
        assert_eq!(success.sources, solo.source_files);
        assert_eq!(solo.match_type(), proto::source::MatchType::Full);
        assert_eq!(
            success.runtime_code,
            format!("0x{}", hex::encode(&full.runtime_code))
        );
        for (details, original) in [
            (&success.creation_match_details, &full.creation_match),
            (&success.runtime_match_details, &full.runtime_match),
        ] {
            let details = details.as_ref().unwrap();
            let original = original.as_ref().unwrap();
            assert_eq!(
                details.match_type(),
                proto::contract_verification_success::MatchType::Full
            );
            assert_eq!(
                serde_json::from_str::<Value>(&details.transformations).unwrap(),
                Value::from(original.transformations.clone())
            );
            assert_eq!(
                serde_json::from_str::<Value>(&details.values).unwrap(),
                Value::from(original.values.clone())
            );
        }
        assert!(matches!(
            &results.items[1].verification_result,
            Some(proto::contract_verification_result::VerificationResult::Failure(_))
        ));
    }
}

#[derive(Clone, Default)]
struct ErrorLogs(Arc<Mutex<Vec<BTreeMap<String, String>>>>);

impl tracing::Subscriber for ErrorLogs {
    fn enabled(&self, _: &tracing::Metadata<'_>) -> bool {
        true
    }
    fn new_span(&self, _: &tracing::span::Attributes<'_>) -> tracing::span::Id {
        tracing::span::Id::from_u64(1)
    }
    fn record(&self, _: &tracing::span::Id, _: &tracing::span::Record<'_>) {}
    fn record_follows_from(&self, _: &tracing::span::Id, _: &tracing::span::Id) {}
    fn event(&self, event: &tracing::Event<'_>) {
        struct Fields(BTreeMap<String, String>);
        impl tracing::field::Visit for Fields {
            fn record_debug(&mut self, field: &tracing::field::Field, value: &dyn std::fmt::Debug) {
                self.0.insert(field.name().into(), format!("{value:?}"));
            }
        }
        let mut fields = Fields(BTreeMap::new());
        event.record(&mut fields);
        if fields.0.contains_key("error_id") {
            self.0.lock().unwrap().push(fields.0);
        }
    }
    fn enter(&self, _: &tracing::span::Id) {}
    fn exit(&self, _: &tracing::span::Id) {}
}

fn assert_private_error(message: &str, cache: &std::path::Path, logs: &ErrorLogs) -> String {
    let id = message
        .strip_prefix("Internal verification error; reference: ")
        .expect("stable public message");
    assert!(id.starts_with("verification-"));
    assert_eq!(id.len(), "verification-".len() + 16);
    for private in [
        cache.to_str().unwrap(),
        "fetching evm compiler",
        "AlreadyExists",
        "File(",
        "os error",
        "cache failure fixture",
    ] {
        assert!(
            !message.contains(private),
            "private detail leaked: {private}"
        );
    }
    let records = logs.0.lock().unwrap();
    let record = records
        .iter()
        .find(|record| record["error_id"] == id)
        .expect("public reference identifies structured server event");
    assert!(record["error"].contains("fetching evm compiler"));
    assert!(record["error"].contains("File"));
    assert!(record["error"].contains("exists") || record["error"].contains("Not a directory"));
    id.into()
}

#[actix_web::test]
async fn internal_errors_exposed_real_cache_failure_http_and_grpc() {
    let logs = ErrorLogs::default();
    tracing::subscriber::set_global_default(logs.clone()).expect("install structured log consumer");
    let peer = FixturePeer::start().await;
    let directory = tempfile::tempdir().unwrap();
    let cache = directory.path().join("private-compiler-cache");
    std::fs::write(&cache, b"cache root is deliberately a file").unwrap();
    let settings: Settings = serde_json::from_value(json!({"solidity": {
        "compilers_dir": cache, "fetcher": {"list": {"list_url": format!("{}list.json", peer.url)}}
    }}))
    .unwrap();
    let service = Arc::new(
        SolidityVerifierService::new(settings.solidity, Arc::new(Semaphore::new(1)))
            .await
            .unwrap(),
    );
    let app = http_test::init_service(
        App::new().configure(|config| route_solidity_verifier(config, service.clone())),
    )
    .await;
    let listener = TcpListener::bind("127.0.0.1:0").unwrap();
    let address = listener.local_addr().unwrap();
    drop(listener);
    let (shutdown, receive) = oneshot::channel();
    let server = tokio::spawn(async move {
        Server::builder()
            .add_service(SolidityVerifierServer::from_arc(service))
            .serve_with_shutdown(address, async {
                let _ = receive.await;
            })
            .await
            .unwrap();
    });
    let mut grpc = SolidityVerifierClient::new(grpc_channel(address).await);
    let sources = BTreeMap::from([(
        "A.sol".to_owned(),
        "pragma solidity ^0.6.12; contract A {}".to_owned(),
    )]);
    let input = json!({"language": "Solidity", "sources": {"A.sol": {"content": sources["A.sol"]}}, "settings": {}}).to_string();
    let solo_multi = proto::VerifySolidityMultiPartRequest {
        bytecode: "0x00".into(),
        bytecode_type: proto::BytecodeType::DeployedBytecode.into(),
        compiler_version: VERSION.into(),
        source_files: sources.clone(),
        ..Default::default()
    };
    let solo_json = proto::VerifySolidityStandardJsonRequest {
        bytecode: "0x00".into(),
        bytecode_type: proto::BytecodeType::DeployedBytecode.into(),
        compiler_version: VERSION.into(),
        input: input.clone(),
        ..Default::default()
    };
    let batch_multi = proto::BatchVerifySolidityMultiPartRequest {
        contracts: vec![proto::Contract {
            runtime_code: Some("0x00".into()),
            ..Default::default()
        }],
        compiler_version: VERSION.into(),
        sources,
        ..Default::default()
    };
    let batch_json = proto::BatchVerifySolidityStandardJsonRequest {
        contracts: batch_multi.contracts.clone(),
        compiler_version: VERSION.into(),
        input,
    };
    let mut ids = Vec::new();
    for (route, payload) in [
        (
            "verify-multi-part",
            serde_json::to_value(&solo_multi).unwrap(),
        ),
        (
            "verify-standard-json",
            serde_json::to_value(&solo_json).unwrap(),
        ),
        (
            "batch-verify-multi-part",
            serde_json::to_value(&batch_multi).unwrap(),
        ),
        (
            "batch-verify-standard-json",
            serde_json::to_value(&batch_json).unwrap(),
        ),
    ] {
        let response = http_test::TestRequest::post()
            .uri(&format!("{SOLIDITY}{route}"))
            .set_json(payload)
            .send_request(&app)
            .await;
        assert_eq!(response.status(), StatusCode::INTERNAL_SERVER_ERROR);
        let body: Value = http_test::read_body_json(response).await;
        assert_eq!(body["code"], 13);
        assert_eq!(
            body.as_object().unwrap().len(),
            2,
            "transport body has only code and message"
        );
        ids.push(assert_private_error(
            body["message"].as_str().unwrap(),
            &cache,
            &logs,
        ));
    }
    let errors = [
        grpc.verify_multi_part(solo_multi).await.unwrap_err(),
        grpc.verify_standard_json(solo_json).await.unwrap_err(),
        grpc.batch_verify_multi_part(batch_multi).await.unwrap_err(),
        grpc.batch_verify_standard_json(batch_json)
            .await
            .unwrap_err(),
    ];
    for error in errors {
        assert_eq!(error.code(), Code::Internal);
        assert!(error.details().is_empty());
        ids.push(assert_private_error(error.message(), &cache, &logs));
    }
    let unique: std::collections::BTreeSet<_> = ids.iter().collect();
    assert_eq!(
        unique.len(),
        8,
        "each failed request has a distinct reference"
    );
    assert_eq!(
        peer.requests()
            .iter()
            .filter(|(path, _)| path == "/compiler")
            .count(),
        8,
        "all failures reach the production fetch/cache path"
    );
    assert_eq!(
        std::fs::read(&cache).unwrap(),
        b"cache root is deliberately a file"
    );
    shutdown.send(()).unwrap();
    server.await.unwrap();
    peer.stop().await;
}
