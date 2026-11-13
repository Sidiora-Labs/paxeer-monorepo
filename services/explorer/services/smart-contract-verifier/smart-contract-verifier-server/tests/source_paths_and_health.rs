use actix_web::{http::StatusCode, test, App};
use serde_json::json;
use smart_contract_verifier::{DetailedVersion, Fetcher, ListFetcher, SolcValidator};
use smart_contract_verifier_proto::blockscout::smart_contract_verifier::v2::{
    health_actix::route_health, health_check_response::ServingStatus, health_client::HealthClient,
    health_server::HealthServer, solidity_verifier_actix::route_solidity_verifier,
    solidity_verifier_client::SolidityVerifierClient,
    solidity_verifier_server::SolidityVerifierServer,
    verify_response::Status as VerificationStatus, vyper_verifier_client::VyperVerifierClient,
    vyper_verifier_server::VyperVerifierServer, BatchVerifySolidityMultiPartRequest,
    BatchVerifySolidityStandardJsonRequest, BytecodeType, Contract, HealthCheckRequest,
    HealthCheckResponse, ListCompilerVersionsRequest, VerifyResponse,
    VerifySolidityMultiPartRequest, VerifySolidityStandardJsonRequest,
};
use smart_contract_verifier_server::{
    HealthService, Settings, SolidityVerifierService, VyperVerifierService,
};
use std::{collections::BTreeMap, net::TcpListener, sync::Arc, time::Duration};
use tempfile::TempDir;
use tokio::sync::{oneshot, Semaphore};
use tonic::{server::NamedService, transport::Server, Code};

const VERSION: &str = "v0.4.10+commit.9e8cc01b";
const COMPILER_LIST: &str =
    "https://raw.githubusercontent.com/ethereum/solc-bin/gh-pages/linux-amd64/list.json";
const SOURCE: &str = "pragma solidity ^0.4.10; contract A { function get_a() public returns (uint256) { return 1; } }";
const SOLIDITY: &str = <SolidityVerifierServer<SolidityVerifierService> as NamedService>::NAME;
const VYPER: &str = <VyperVerifierServer<VyperVerifierService> as NamedService>::NAME;

async fn solidity_service(directory: &TempDir) -> Arc<SolidityVerifierService> {
    let compilers_dir = directory.path().join("compiler-cache");
    let version: DetailedVersion = VERSION.parse().unwrap();
    // Install the checksum-verified fixture first, closing its writable handle
    // before executing it. Then apply the server's complete cache validation.
    let installer = ListFetcher::new(
        COMPILER_LIST.parse().unwrap(),
        compilers_dir.clone(),
        None,
        None,
    )
    .await
    .expect("load old-solc fixture catalogue");
    let compiler = installer
        .fetch(&version)
        .await
        .unwrap_or_else(|error| panic!("provision {VERSION} fixture: {error:#}"));
    let validator = ListFetcher::new(
        COMPILER_LIST.parse().unwrap(),
        compilers_dir.clone(),
        None,
        Some(Arc::new(SolcValidator::default())),
    )
    .await
    .expect("load catalogue for fixture authentication");
    validator
        .validate_cached(&version, &compiler)
        .await
        .unwrap_or_else(|error| panic!("authenticate executable {VERSION} fixture: {error:#}"));

    let settings: Settings = serde_json::from_value(json!({
        "solidity": {
            "compilers_dir": compilers_dir,
            "fetcher": {"list": {"list_url": COMPILER_LIST}}
        }
    }))
    .unwrap();
    Arc::new(
        SolidityVerifierService::new(settings.solidity, Arc::new(Semaphore::new(1)))
            .await
            .expect("initialize real Solidity verifier"),
    )
}

fn multipart(name: &str) -> VerifySolidityMultiPartRequest {
    VerifySolidityMultiPartRequest {
        bytecode: "0x00".into(),
        bytecode_type: BytecodeType::DeployedBytecode as i32,
        compiler_version: VERSION.into(),
        source_files: BTreeMap::from([(name.into(), SOURCE.into())]),
        ..Default::default()
    }
}

fn standard_json(name: &str) -> String {
    json!({
        "language": "Solidity",
        "sources": BTreeMap::from([(name, json!({"content": SOURCE}))]),
        "settings": {}
    })
    .to_string()
}

#[actix_web::test]
async fn absolute_source_path_write() {
    let directory = tempfile::tempdir().unwrap();
    let sentinel = directory.path().join("sentinel.sol");
    let original = b"external service state must survive failed verification";
    std::fs::write(&sentinel, original).unwrap();
    let service = solidity_service(&directory).await;
    let app = test::init_service(
        App::new().configure(|config| route_solidity_verifier(config, service.clone())),
    )
    .await;
    let prefix = "/api/v2/verifier/solidity/sources:";

    // Compilation succeeds; the deliberately different on-chain code does not verify.
    // This exercises old-solc's CLI path with the authenticated fixture cache.
    let response = test::TestRequest::post()
        .uri(&format!("{prefix}verify-multi-part"))
        .set_json(multipart("nested/A.sol"))
        .send_request(&app)
        .await;
    let status = response.status();
    let body = test::read_body(response).await;
    assert_eq!(status, StatusCode::OK, "{}", String::from_utf8_lossy(&body));
    let response: VerifyResponse = serde_json::from_slice(&body).unwrap();
    assert_eq!(response.status(), VerificationStatus::Failure);
    assert_eq!(
        response.message,
        "No contract could be verified with provided data"
    );

    let compiler = directory
        .path()
        .join("compiler-cache")
        .join(VERSION)
        .join("solc");
    let compiler_bytes = std::fs::read(&compiler).expect("old solc is actually cached");
    assert!(compiler_bytes.starts_with(b"\x7fELF"));
    let link = directory.path().join("external-link.sol");
    std::os::unix::fs::symlink(&sentinel, &link).unwrap();
    let traversal = format!("../{}", sentinel.strip_prefix("/").unwrap().display());
    let names = [
        sentinel.to_str().unwrap().to_owned(),
        compiler.to_str().unwrap().to_owned(),
        link.to_str().unwrap().to_owned(),
        traversal,
        "nested/../../sentinel.sol".to_owned(),
        "nested/../A.sol".to_owned(),
        "C:\\sentinel.sol".to_owned(),
        "\\\\server\\sentinel.sol".to_owned(),
        "nested\\..\\sentinel.sol".to_owned(),
        "nested/invalid\0.sol".to_owned(),
        "/".to_owned(),
        String::new(),
        ".".to_owned(),
    ];
    for name in names {
        let multi = multipart(&name);
        let standard = VerifySolidityStandardJsonRequest {
            bytecode: multi.bytecode.clone(),
            bytecode_type: multi.bytecode_type,
            compiler_version: VERSION.into(),
            input: standard_json(&name),
            ..Default::default()
        };
        let contracts = vec![Contract {
            runtime_code: Some("0x00".into()),
            ..Default::default()
        }];
        let batch_multi = BatchVerifySolidityMultiPartRequest {
            contracts: contracts.clone(),
            compiler_version: VERSION.into(),
            sources: multi.source_files.clone(),
            ..Default::default()
        };
        let batch_standard = BatchVerifySolidityStandardJsonRequest {
            contracts,
            compiler_version: VERSION.into(),
            input: standard.input.clone(),
        };
        for (route, body) in [
            ("verify-multi-part", serde_json::to_value(multi).unwrap()),
            (
                "verify-standard-json",
                serde_json::to_value(standard).unwrap(),
            ),
            (
                "batch-verify-multi-part",
                serde_json::to_value(batch_multi).unwrap(),
            ),
            (
                "batch-verify-standard-json",
                serde_json::to_value(batch_standard).unwrap(),
            ),
        ] {
            let response = test::TestRequest::post()
                .uri(&format!("{prefix}{route}"))
                .set_json(body)
                .send_request(&app)
                .await;
            assert_eq!(
                response.status(),
                StatusCode::BAD_REQUEST,
                "{route}: {name:?}"
            );
            let body = test::read_body(response).await;
            assert!(
                std::str::from_utf8(&body)
                    .unwrap()
                    .contains("source paths must be relative"),
                "{route}: {name:?}: {}",
                String::from_utf8_lossy(&body)
            );
            assert_eq!(std::fs::read(&sentinel).unwrap(), original);
            assert_eq!(std::fs::read(&compiler).unwrap(), compiler_bytes);
        }
    }

    let response = test::TestRequest::post()
        .uri(&format!("{prefix}verify-standard-json"))
        .set_json(VerifySolidityStandardJsonRequest {
            bytecode: "0x00".into(),
            bytecode_type: BytecodeType::DeployedBytecode as i32,
            compiler_version: VERSION.into(),
            input: standard_json("nested/A.sol"),
            ..Default::default()
        })
        .send_request(&app)
        .await;
    let status = response.status();
    let body = test::read_body(response).await;
    assert_eq!(status, StatusCode::OK, "{}", String::from_utf8_lossy(&body));
    let response: VerifyResponse = serde_json::from_slice(&body).unwrap();
    assert_eq!(
        response.message,
        "No contract could be verified with provided data"
    );
    assert_eq!(std::fs::read(&compiler).unwrap(), compiler_bytes);
}

#[actix_web::test]
async fn health_unknown_service() {
    let directory = tempfile::tempdir().unwrap();
    let solidity = solidity_service(&directory).await;
    let health = Arc::new(HealthService::new([SOLIDITY]));
    let listener = TcpListener::bind("127.0.0.1:0").unwrap();
    let address = listener.local_addr().unwrap();
    drop(listener);
    let (shutdown_tx, shutdown_rx) = oneshot::channel();
    let grpc_health = health.clone();
    let server = tokio::spawn(async move {
        Server::builder()
            .add_service(HealthServer::from_arc(grpc_health))
            .add_optional_service(Some(SolidityVerifierServer::from_arc(solidity)))
            .add_optional_service(None::<VyperVerifierServer<VyperVerifierService>>)
            .serve_with_shutdown(address, async {
                let _ = shutdown_rx.await;
            })
            .await
            .unwrap();
    });
    let endpoint = format!("http://{address}");
    let channel = tokio::time::timeout(Duration::from_secs(5), async {
        loop {
            if let Ok(channel) = tonic::transport::Endpoint::from_shared(endpoint.clone())
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
    .expect("gRPC listener starts");
    let mut client = HealthClient::new(channel.clone());
    for name in [
        "",
        <HealthServer<HealthService> as NamedService>::NAME,
        SOLIDITY,
    ] {
        let response = client
            .check(HealthCheckRequest {
                service: name.into(),
            })
            .await
            .unwrap();
        assert_eq!(response.into_inner().status(), ServingStatus::Serving);
    }
    let versions = SolidityVerifierClient::new(channel.clone())
        .list_compiler_versions(ListCompilerVersionsRequest {})
        .await
        .unwrap()
        .into_inner()
        .compiler_versions;
    assert!(versions.contains(&VERSION.to_string()));
    for name in [VYPER, "nonexistent.verifier", "SolidityVerifier"] {
        let error = client
            .check(HealthCheckRequest {
                service: name.into(),
            })
            .await
            .unwrap_err();
        assert_eq!(error.code(), Code::NotFound, "{name}");
    }
    let error = VyperVerifierClient::new(channel)
        .list_compiler_versions(ListCompilerVersionsRequest {})
        .await
        .unwrap_err();
    assert_eq!(error.code(), Code::Unimplemented);

    let app =
        test::init_service(App::new().configure(|config| route_health(config, health.clone())))
            .await;
    for uri in ["/health", &format!("/health?service={SOLIDITY}")] {
        let response = test::TestRequest::get().uri(uri).send_request(&app).await;
        assert_eq!(response.status(), StatusCode::OK);
        let body: HealthCheckResponse = test::read_body_json(response).await;
        assert_eq!(body.status(), ServingStatus::Serving);
    }
    for name in [VYPER, "nonexistent.verifier"] {
        let response = test::TestRequest::get()
            .uri(&format!("/health?service={name}"))
            .send_request(&app)
            .await;
        assert_eq!(response.status(), StatusCode::NOT_FOUND);
    }
    shutdown_tx.send(()).unwrap();
    server.await.unwrap();
}
