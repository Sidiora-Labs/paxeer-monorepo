use actix_web::{
    dev::ServerHandle, http::StatusCode, test, web, App, HttpRequest, HttpResponse, HttpServer,
};
use serde_json::{json, Value};
use smart_contract_verifier::{
    solidity, DetailedVersion, EvmCompilersPool, Fetcher, ListFetcher, SolcCompiler, SolcInput,
};
use smart_contract_verifier_proto::blockscout::smart_contract_verifier::v2::{
    self as proto,
    solidity_verifier_client::SolidityVerifierClient,
    sourcify_verifier_actix::route_sourcify_verifier,
    vyper_verifier_client::VyperVerifierClient,
    zksync::solidity::{
        self as zkproto, zk_sync_solidity_verifier_client::ZkSyncSolidityVerifierClient,
    },
    BatchVerifySolidityMultiPartRequest, BatchVerifySolidityStandardJsonRequest, BytecodeType,
    Contract, VerificationMetadata, VerifySolidityMultiPartRequest,
    VerifySolidityStandardJsonRequest,
};
use smart_contract_verifier_server::{Settings, SourcifyVerifierService};
use std::{
    collections::BTreeMap,
    net::{SocketAddr, TcpListener},
    os::unix::fs::PermissionsExt,
    path::Path,
    sync::{
        atomic::{AtomicBool, AtomicUsize, Ordering},
        Arc, Mutex,
    },
    time::{Duration, Instant},
};
use tokio::{
    io::{AsyncReadExt, AsyncWriteExt},
    sync::Semaphore,
};
use tonic::Code;

const VERSION: &str = "v0.8.20+commit.a1b79de6";
const ADDRESS: &str = "0x1111111111111111111111111111111111111111";
const SOURCE: &str = "pragma solidity ^0.8.20; contract A {}";
const SOLIDITY: &str = "/api/v2/verifier/solidity/sources:";
const SOURCIFY: &str = "/api/v2/verifier/sourcify/sources:verify-from-etherscan";
const LIMIT: i32 = 1_000_000;
// Opaque distribution bytes: the cache is the component under test. No compiler
// process or substitute compiler implementation is involved in these cache tests.
const DISTRIBUTION: &[u8] = b"recorded compiler distribution payload\n";
const DISTRIBUTION_SHA: &str = "0xfa02b6d23d8cf1c8feaab2c83f3d54ac05f2a5ac6de8f9b2b7094a6ef079bd3c";

async fn catalogue() -> (String, ServerHandle, Arc<AtomicUsize>) {
    let listener = TcpListener::bind("127.0.0.1:0").unwrap();
    let address = listener.local_addr().unwrap();
    let downloads = Arc::new(AtomicUsize::new(0));
    let observed = downloads.clone();
    let server = HttpServer::new(move || {
        let observed = observed.clone();
        App::new().default_service(web::to(move |request: HttpRequest| {
            let observed = observed.clone();
            async move {
                if request.path() == "/list.json" {
                    HttpResponse::Ok().json(json!({"builds": []}))
                } else {
                    observed.fetch_add(1, Ordering::SeqCst);
                    HttpResponse::NotFound().finish()
                }
            }
        }))
    })
    .workers(1)
    .listen(listener)
    .unwrap()
    .run();
    let handle = server.handle();
    actix_web::rt::spawn(server);
    (format!("http://{address}/list.json"), handle, downloads)
}

fn multipart(runs: i32) -> VerifySolidityMultiPartRequest {
    VerifySolidityMultiPartRequest {
        bytecode: "0x00".into(),
        bytecode_type: BytecodeType::DeployedBytecode as i32,
        compiler_version: VERSION.into(),
        source_files: BTreeMap::from([("A.sol".into(), SOURCE.into())]),
        optimization_runs: Some(runs),
        metadata: Some(VerificationMetadata {
            chain_id: Some("125".into()),
            ..Default::default()
        }),
        ..Default::default()
    }
}

fn standard_input(runs: u64) -> String {
    json!({"language": "Solidity", "sources": {"A.sol": {"content": SOURCE}},
        "settings": {"optimizer": {"enabled": true, "runs": runs}}})
    .to_string()
}

fn contracts() -> Vec<Contract> {
    [Some("125"), Some("caller-controlled-chain"), None]
        .into_iter()
        .map(|chain| Contract {
            runtime_code: Some("0x00".into()),
            metadata: chain.map(|chain| VerificationMetadata {
                chain_id: Some(chain.into()),
                ..Default::default()
            }),
            ..Default::default()
        })
        .collect()
}

async fn http(address: SocketAddr, method: &str, path: &str, body: &str) -> (u16, String) {
    tokio::time::timeout(Duration::from_secs(5), async {
        let mut stream = tokio::net::TcpStream::connect(address).await.unwrap();
        let request = format!("{method} {path} HTTP/1.1\r\nHost: {address}\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}", body.len());
        stream.write_all(request.as_bytes()).await.unwrap();
        let mut response = Vec::new();
        stream.read_to_end(&mut response).await.unwrap();
        let response = String::from_utf8(response).unwrap();
        let (headers, body) = response.split_once("\r\n\r\n").unwrap();
        let status = headers.split_whitespace().nth(1).unwrap().parse().unwrap();
        // Actix streams the metrics response with chunked encoding; metric lines
        // still appear verbatim and are the only data this helper reads from it.
        (status, body.to_string())
    }).await.expect("loopback HTTP response deadline")
}

fn counter(chain: &str, language: &str, endpoint: &str, status: &str) -> u64 {
    prometheus::gather()
        .into_iter()
        .filter(|family| family.get_name() == "smart_contract_verifier_verify_contract")
        .flat_map(|family| family.get_metric().to_vec())
        .filter(|metric| {
            [
                ("chain_id", chain),
                ("language", language),
                ("endpoint", endpoint),
                ("status", status),
            ]
            .iter()
            .all(|(key, value)| {
                metric
                    .get_label()
                    .iter()
                    .any(|label| label.get_name() == *key && label.get_value() == *value)
            })
        })
        .map(|metric| metric.get_counter().get_value() as u64)
        .sum()
}

#[actix_web::test]
async fn negative_optimization_runs_and_verification_counter_misses_routes() {
    let directory = tempfile::tempdir().unwrap();
    let (list, upstream, downloads) = catalogue().await;
    // Reserve every server port together before releasing them for the production launcher.
    let listeners: Vec<_> = (0..3)
        .map(|_| TcpListener::bind("127.0.0.1:0").unwrap())
        .collect();
    let addresses: Vec<_> = listeners
        .iter()
        .map(|listener| listener.local_addr().unwrap())
        .collect();
    let settings: Settings = serde_json::from_value(json!({
        "server": {"http": {"addr": addresses[0]}, "grpc": {"enabled": true, "addr": addresses[1]}},
        "metrics": {"enabled": true, "addr": addresses[2]},
        "tracing": {"enabled": false}, "jaeger": {"enabled": false},
        "sourcify": {"enabled": false},
        "solidity": {"compilers_dir": directory.path().join("solidity"), "fetcher": {"list": {"list_url": list}}},
        "vyper": {"compilers_dir": directory.path().join("vyper"), "fetcher": {"list": {"list_url": list}}},
        "zksync_solidity": {"evm_compilers_dir": directory.path().join("evm"), "zk_compilers_dir": directory.path().join("zk"),
            "evm_fetcher": {"list": {"list_url": list}}, "era_evm_fetcher": {"list": {"list_url": list}}, "zk_fetcher": {"list": {"list_url": list}}}
    })).unwrap();
    drop(listeners);
    let base = format!("http://{}", addresses[0]).parse().unwrap();
    let server = blockscout_service_launcher::test_server::init_server(
        || smart_contract_verifier_server::run(settings),
        &base,
    )
    .await;
    let channel = tokio::time::timeout(Duration::from_secs(5), async {
        loop {
            if let Ok(channel) =
                tonic::transport::Endpoint::from_shared(format!("http://{}", addresses[1]))
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
    .expect("production gRPC listener starts");
    let mut client = SolidityVerifierClient::new(channel.clone());

    for runs in [-1, LIMIT + 1, i32::MAX] {
        let before = counter("125", "solidity", "multi-part", "INVALID_ARGUMENT");
        let (status, body) = http(
            addresses[0],
            "POST",
            &format!("{SOLIDITY}verify-multi-part"),
            &serde_json::to_string(&multipart(runs)).unwrap(),
        )
        .await;
        assert_eq!(status, 400, "{body}");
        assert!(body.contains("optimization_runs"), "{body}");
        assert_eq!(
            counter("125", "solidity", "multi-part", "INVALID_ARGUMENT"),
            before + 1
        );
        let error = client.verify_multi_part(multipart(runs)).await.unwrap_err();
        assert_eq!(error.code(), Code::InvalidArgument);
        assert!(error.message().contains("optimization_runs"), "{error}");
        assert_eq!(
            counter("125", "solidity", "multi-part", "INVALID_ARGUMENT"),
            before + 2
        );
    }

    for runs in [0, LIMIT] {
        let request = multipart(runs);
        let wire = serde_json::to_vec(&request).unwrap();
        let decoded: VerifySolidityMultiPartRequest = serde_json::from_slice(&wire).unwrap();
        let parsed = solidity::multi_part::VerificationRequest::try_from(decoded).unwrap();
        let inputs: Vec<SolcInput> = parsed.content.into();
        assert_eq!(inputs.len(), 1);
        assert_eq!(inputs[0].0.settings.optimizer.runs, Some(runs as usize));
        assert_eq!(inputs[0].0.settings.optimizer.enabled, Some(true));
        let (status, body) = http(
            addresses[0],
            "POST",
            &format!("{SOLIDITY}verify-multi-part"),
            std::str::from_utf8(&wire).unwrap(),
        )
        .await;
        assert_eq!(status, 400, "{body}");
        assert!(
            body.contains("not found") && !body.contains("optimization_runs"),
            "{body}"
        );
        let error = client.verify_multi_part(request).await.unwrap_err();
        assert_eq!(error.code(), Code::InvalidArgument);
        assert!(
            error.message().contains("not found") && !error.message().contains("optimization_runs")
        );

        let standard = VerifySolidityStandardJsonRequest {
            bytecode: "0x00".into(),
            bytecode_type: BytecodeType::DeployedBytecode as i32,
            compiler_version: VERSION.into(),
            input: standard_input(runs as u64),
            ..Default::default()
        };
        let parsed = solidity::standard_json::VerificationRequest::try_from(standard).unwrap();
        assert_eq!(
            parsed.content.0.settings.optimizer.runs,
            Some(runs as usize)
        );
        let batch = BatchVerifySolidityMultiPartRequest {
            contracts: contracts(),
            compiler_version: VERSION.into(),
            sources: multipart(runs).source_files,
            optimization_runs: Some(runs as u32),
            ..Default::default()
        };
        let parsed = solidity::multi_part::BatchVerificationRequest::try_from(batch).unwrap();
        let inputs: Vec<SolcInput> = parsed.content.into();
        assert_eq!(inputs[0].0.settings.optimizer.runs, Some(runs as usize));
        let parsed = solidity::standard_json::BatchVerificationRequest::try_from(
            BatchVerifySolidityStandardJsonRequest {
                contracts: contracts(),
                compiler_version: VERSION.into(),
                input: standard_input(runs as u64),
            },
        )
        .unwrap();
        assert_eq!(
            parsed.content.0.settings.optimizer.runs,
            Some(runs as usize)
        );
    }

    for runs in [(LIMIT as u64) + 1, u32::MAX as u64] {
        let standard = VerifySolidityStandardJsonRequest {
            bytecode: "0x00".into(),
            bytecode_type: BytecodeType::DeployedBytecode as i32,
            compiler_version: VERSION.into(),
            input: standard_input(runs),
            ..Default::default()
        };
        let error = client.verify_standard_json(standard).await.unwrap_err();
        assert_eq!(error.code(), Code::InvalidArgument);
        assert!(error.message().contains("optimization_runs"));
        let batch = BatchVerifySolidityMultiPartRequest {
            contracts: contracts(),
            compiler_version: VERSION.into(),
            sources: multipart(0).source_files,
            optimization_runs: Some(runs as u32),
            ..Default::default()
        };
        let before: Vec<_> = ["125", "other", "unknown"]
            .map(|chain| counter(chain, "solidity", "batch-multi-part", "INVALID_ARGUMENT"))
            .into();
        let error = client.batch_verify_multi_part(batch).await.unwrap_err();
        assert_eq!(error.code(), Code::InvalidArgument);
        assert!(error.message().contains("optimization_runs"));
        for (index, chain) in ["125", "other", "unknown"].iter().enumerate() {
            assert_eq!(
                counter(chain, "solidity", "batch-multi-part", "INVALID_ARGUMENT"),
                before[index] + 1
            );
        }
        let error = client
            .batch_verify_standard_json(BatchVerifySolidityStandardJsonRequest {
                contracts: contracts(),
                compiler_version: VERSION.into(),
                input: standard_input(runs),
            })
            .await
            .unwrap_err();
        assert_eq!(error.code(), Code::InvalidArgument);
        assert!(error.message().contains("optimization_runs"));
    }

    // HTTP-success compilation failures count once per submitted batch contract.
    let before = ["125", "other", "unknown"]
        .map(|chain| counter(chain, "solidity", "batch-standard-json", "FAILURE"));
    let batch = client
        .batch_verify_standard_json(BatchVerifySolidityStandardJsonRequest {
            contracts: contracts(),
            compiler_version: VERSION.into(),
            input: "{".into(),
        })
        .await
        .unwrap()
        .into_inner();
    assert!(matches!(
        batch.verification_result,
        Some(proto::batch_verify_response::VerificationResult::CompilationFailure(_))
    ));
    for (index, chain) in ["125", "other", "unknown"].iter().enumerate() {
        assert_eq!(
            counter(chain, "solidity", "batch-standard-json", "FAILURE"),
            before[index] + 1
        );
    }
    let before = counter("unknown", "solidity", "standard-json", "FAILURE");
    let response = client
        .verify_standard_json(VerifySolidityStandardJsonRequest {
            bytecode: "0x00".into(),
            bytecode_type: BytecodeType::DeployedBytecode as i32,
            compiler_version: VERSION.into(),
            input: "{".into(),
            ..Default::default()
        })
        .await
        .unwrap()
        .into_inner();
    assert_eq!(response.status(), proto::verify_response::Status::Failure);
    assert_eq!(
        counter("unknown", "solidity", "standard-json", "FAILURE"),
        before + 1
    );

    let mut zk = ZkSyncSolidityVerifierClient::new(channel.clone());
    let before = counter("unknown", "solidity", "zksync-standard-json", "FAILURE");
    let response = zk
        .verify_standard_json(zkproto::VerifyStandardJsonRequest {
            code: "0x00".into(),
            solc_compiler: VERSION.into(),
            zk_compiler: "v1.5.0".into(),
            input: "{".into(),
            ..Default::default()
        })
        .await
        .unwrap()
        .into_inner();
    assert!(matches!(
        response.verify_response,
        Some(zkproto::verify_response::VerifyResponse::CompilationFailure(_))
    ));
    assert_eq!(
        counter("unknown", "solidity", "zksync-standard-json", "FAILURE"),
        before + 1
    );
    let before = counter(
        "unknown",
        "solidity",
        "zksync-standard-json",
        "INVALID_ARGUMENT",
    );
    let error = zk
        .verify_standard_json(zkproto::VerifyStandardJsonRequest::default())
        .await
        .unwrap_err();
    assert_eq!(error.code(), Code::InvalidArgument);
    assert_eq!(
        counter(
            "unknown",
            "solidity",
            "zksync-standard-json",
            "INVALID_ARGUMENT"
        ),
        before + 1
    );
    let mut vyper = VyperVerifierClient::new(channel);
    for endpoint in ["multi-part", "standard-json"] {
        let before = counter("unknown", "vyper", endpoint, "INVALID_ARGUMENT");
        let error = if endpoint == "multi-part" {
            vyper
                .verify_multi_part(proto::VerifyVyperMultiPartRequest::default())
                .await
                .unwrap_err()
        } else {
            vyper
                .verify_standard_json(proto::VerifyVyperStandardJsonRequest::default())
                .await
                .unwrap_err()
        };
        assert_eq!(error.code(), Code::InvalidArgument);
        assert_eq!(
            counter("unknown", "vyper", endpoint, "INVALID_ARGUMENT"),
            before + 1
        );
    }
    let (status, exported) = http(addresses[2], "GET", "/metrics", "").await;
    assert_eq!(status, 200);
    for endpoint in [
        "multi-part",
        "standard-json",
        "batch-multi-part",
        "batch-standard-json",
    ] {
        assert!(
            exported.contains(&format!("endpoint=\"{endpoint}\"")),
            "{exported}"
        );
    }
    assert!(exported.contains("endpoint=\"zksync-standard-json\""));
    assert!(exported.contains("status=\"INVALID_ARGUMENT\""));
    assert!(exported.contains("status=\"FAILURE\""));
    assert!(!exported.contains("caller-controlled-chain"));
    assert_eq!(
        downloads.load(Ordering::SeqCst),
        0,
        "no native compiler is needed by the gate"
    );
    server.abort();
    let _ = server.await;
    upstream.stop(false).await;
}

async fn cache_pool(
    list: &str,
    root: &Path,
) -> (
    EvmCompilersPool<SolcCompiler>,
    Arc<ListFetcher<DetailedVersion>>,
) {
    let fetcher = Arc::new(
        ListFetcher::new(list.parse().unwrap(), root.to_path_buf(), None, None)
            .await
            .unwrap(),
    );
    let pool = EvmCompilersPool::new(fetcher.clone(), Arc::new(Semaphore::new(1)));
    pool.load_from_dir(&root.to_path_buf()).await;
    (pool, fetcher)
}

#[actix_web::test]
async fn compiler_cache_trust_after_restart() {
    let root = tempfile::tempdir().unwrap();
    let listener = TcpListener::bind("127.0.0.1:0").unwrap();
    let address = listener.local_addr().unwrap();
    let corrupted_download = Arc::new(AtomicBool::new(false));
    let downloads = Arc::new(AtomicUsize::new(0));
    let bad = corrupted_download.clone();
    let count = downloads.clone();
    let server = HttpServer::new(move || {
        let bad = bad.clone();
        let count = count.clone();
        App::new().default_service(web::to(move |request: HttpRequest| {
            let bad = bad.clone();
            let count = count.clone();
            async move {
                match request.path() {
                    "/list.json" => HttpResponse::Ok().json(json!({"builds": [{
                        "path": "compiler", "longVersion": VERSION, "sha256": DISTRIBUTION_SHA
                    }]})),
                    "/compiler" => {
                        count.fetch_add(1, Ordering::SeqCst);
                        HttpResponse::Ok().body(if bad.load(Ordering::SeqCst) {
                            &b"altered download"[..]
                        } else {
                            DISTRIBUTION
                        })
                    }
                    _ => HttpResponse::NotFound().finish(),
                }
            }
        }))
    })
    .workers(1)
    .listen(listener)
    .unwrap()
    .run();
    let upstream = server.handle();
    actix_web::rt::spawn(server);
    let list = format!("http://{address}/list.json");
    let version: DetailedVersion = VERSION.parse().unwrap();
    let (pool, fetcher) = cache_pool(&list, root.path()).await;
    let executable = pool.fetch_compiler(&version).await.unwrap();
    assert_eq!(std::fs::read(&executable).unwrap(), DISTRIBUTION);
    for path in [root.path(), executable.parent().unwrap(), &executable] {
        assert_eq!(
            std::fs::metadata(path).unwrap().permissions().mode() & 0o777,
            0o700
        );
    }
    assert_eq!(downloads.load(Ordering::SeqCst), 1);
    drop(pool);
    drop(fetcher);

    // A complete reopen reuses an intact, authenticated entry without a download.
    let (pool, fetcher) = cache_pool(&list, root.path()).await;
    assert_eq!(pool.fetch_compiler(&version).await.unwrap(), executable);
    assert_eq!(downloads.load(Ordering::SeqCst), 1);
    drop(pool);
    drop(fetcher);
    std::fs::write(&executable, b"altered persisted executable").unwrap();
    corrupted_download.store(true, Ordering::SeqCst);

    let (pool, fetcher) = cache_pool(&list, root.path()).await;
    let error = fetcher
        .validate_cached(&version, &executable)
        .await
        .unwrap_err();
    assert!(
        format!("{error:?}").starts_with("HashMismatch("),
        "{error:?}"
    );
    let error = pool.fetch_compiler(&version).await.unwrap_err();
    let smart_contract_verifier::Error::Internal(error) = error else {
        panic!("corrupt replacement must fail compiler fetching: {error:#?}");
    };
    assert_eq!(error.to_string(), "fetching evm compiler");
    assert!(
        format!(
            "{:?}",
            error.chain().nth(1).expect("compiler fetch error source")
        )
        .starts_with("HashMismatch("),
        "{error:#?}"
    );
    assert_eq!(downloads.load(Ordering::SeqCst), 2);
    assert_eq!(
        std::fs::read(&executable).unwrap(),
        b"altered persisted executable"
    );
    corrupted_download.store(false, Ordering::SeqCst);
    assert_eq!(pool.fetch_compiler(&version).await.unwrap(), executable);
    assert_eq!(std::fs::read(&executable).unwrap(), DISTRIBUTION);
    assert_eq!(downloads.load(Ordering::SeqCst), 3);
    drop(pool);
    drop(fetcher);

    let (pool, fetcher) = cache_pool(&list, root.path()).await;
    assert_eq!(pool.fetch_compiler(&version).await.unwrap(), executable);
    assert_eq!(downloads.load(Ordering::SeqCst), 3);
    // A parseable directory cannot invent a catalogue version identity.
    let other: DetailedVersion = "v0.8.19+commit.7dd6d404".parse().unwrap();
    let other_dir = root.path().join(other.to_string());
    std::fs::create_dir(&other_dir).unwrap();
    std::fs::set_permissions(&other_dir, std::fs::Permissions::from_mode(0o700)).unwrap();
    std::fs::copy(&executable, other_dir.join("solc")).unwrap();
    drop(pool);
    drop(fetcher);
    let (pool, _) = cache_pool(&list, root.path()).await;
    assert!(pool.fetch_compiler(&other).await.is_err());
    assert_eq!(downloads.load(Ordering::SeqCst), 3);
    upstream.stop(false).await;
}

fn source_files() -> Value {
    let metadata = json!({
        "compiler": {"version": "0.8.20+commit.a1b79de6"}, "language": "Solidity", "output": {"abi": []},
        "settings": {"compilationTarget": {"A.sol": "A"}, "optimizer": {"enabled": true, "runs": 200}, "libraries": {}},
        "sources": {"A.sol": {}}, "version": 1
    });
    let prefix = format!("contracts/full_match/125/{ADDRESS}");
    json!({"status": "full", "files": [
        {"name": "A.sol", "path": format!("{prefix}/sources/A.sol"), "content": SOURCE},
        {"name": "metadata.json", "path": format!("{prefix}/metadata.json"), "content": metadata.to_string()}
    ]})
}

#[actix_web::test]
async fn sourcify_import_timeout_drift() {
    for stalled_stage in [0, 1, 2, 3, 4] {
        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        let address = listener.local_addr().unwrap();
        let calls = Arc::new(Mutex::new(Vec::new()));
        let observed = calls.clone();
        let server = HttpServer::new(move || {
            let observed = observed.clone();
            App::new().default_service(web::to(move |request: HttpRequest, body: web::Bytes| {
                let observed = observed.clone();
                async move {
                    let path = request.path();
                    observed.lock().unwrap().push(path.to_owned());
                    let stage = if path == "/chains" { 1 } else if path == "/verify/etherscan" { 2 } else { 3 };
                    if stage == stalled_stage { tokio::time::sleep(Duration::from_secs(4)).await; }
                    if stalled_stage == 4 { tokio::time::sleep(Duration::from_millis(450)).await; }
                    match path {
                        "/" => HttpResponse::Ok().json(json!({"result": [{"address": ADDRESS, "status": "perfect"}]})),
                        "/chains" => HttpResponse::Ok().json(json!([{"chainId": 125, "supported": true, "etherscanAPI": true}])),
                        "/verify/etherscan" => {
                            assert_eq!(request.method(), "POST");
                            let body: Value = serde_json::from_slice(&body).unwrap();
                            assert_eq!(body, json!({"chainId": "125", "address": ADDRESS}));
                            HttpResponse::Ok().json(json!({"result": [{"address": ADDRESS, "chainId": "125", "status": "perfect", "libraryMap": {}}]}))
                        },
                        path if path == format!("/files/any/125/{ADDRESS}") => HttpResponse::Ok().json(source_files()),
                        _ => HttpResponse::NotFound().finish(),
                    }
                }
            }))
        }).workers(1).listen(listener).unwrap().run();
        let upstream = server.handle();
        actix_web::rt::spawn(server);
        let settings: Settings = serde_json::from_value(json!({"sourcify": {
            "api_url": format!("http://{address}/"), "request_timeout": 1, "verification_attempts": 3
        }})).unwrap();
        let service = Arc::new(
            SourcifyVerifierService::new(settings.sourcify)
                .await
                .unwrap(),
        );
        let app = test::init_service(
            App::new().configure(|config| route_sourcify_verifier(config, service.clone())),
        )
        .await;
        let started = Instant::now();
        let before = counter(
            "125",
            "solidity",
            "sourcify-from-etherscan",
            if stalled_stage == 0 {
                "SUCCESS"
            } else {
                "DEADLINE_EXCEEDED"
            },
        );
        let response = tokio::time::timeout(
            Duration::from_secs(2),
            test::TestRequest::post()
                .uri(SOURCIFY)
                .set_json(json!({"chain": "125", "address": ADDRESS}))
                .send_request(&app),
        )
        .await
        .expect("every upstream stage honors the configured one-second deadline");
        let elapsed = started.elapsed();
        let status = response.status();
        let body = test::read_body(response).await;
        if stalled_stage == 0 {
            assert_eq!(status, StatusCode::OK, "{}", String::from_utf8_lossy(&body));
            let result: proto::VerifyResponse = serde_json::from_slice(&body).unwrap();
            assert_eq!(result.status(), proto::verify_response::Status::Success);
            let source = result.source.unwrap();
            assert_eq!(source.source_files.get("A.sol").unwrap(), SOURCE);
            let settings: Value = serde_json::from_str(&source.compiler_settings).unwrap();
            assert_eq!(settings["optimizer"]["runs"], 200);
            assert_eq!(
                counter("125", "solidity", "sourcify-from-etherscan", "SUCCESS"),
                before + 1
            );
        } else {
            assert_eq!(
                status,
                StatusCode::GATEWAY_TIMEOUT,
                "{}",
                String::from_utf8_lossy(&body)
            );
            assert!(String::from_utf8_lossy(&body).contains("Sourcify operation deadline exceeded"));
            assert!(
                elapsed >= Duration::from_millis(900) && elapsed < Duration::from_secs(2),
                "{elapsed:?}"
            );
            assert_eq!(
                counter(
                    "125",
                    "solidity",
                    "sourcify-from-etherscan",
                    "DEADLINE_EXCEEDED"
                ),
                before + 1
            );
        }
        let expected = if stalled_stage == 0 || stalled_stage == 4 {
            3
        } else {
            stalled_stage
        };
        let expected_paths = [
            "/chains".to_owned(),
            "/verify/etherscan".to_owned(),
            format!("/files/any/125/{ADDRESS}"),
        ];
        assert_eq!(*calls.lock().unwrap(), expected_paths[..expected]);
        if stalled_stage == 0 {
            calls.lock().unwrap().clear();
            let before = counter("125", "solidity", "sourcify", "SUCCESS");
            let response = test::TestRequest::post()
                .uri("/api/v2/verifier/sourcify/sources:verify")
                .set_json(proto::VerifySourcifyRequest {
                    address: ADDRESS.into(),
                    chain: "125".into(),
                    files: BTreeMap::from([("A.sol".into(), SOURCE.into())]),
                    ..Default::default()
                })
                .send_request(&app)
                .await;
            assert_eq!(response.status(), StatusCode::OK);
            let response: proto::VerifyResponse = test::read_body_json(response).await;
            assert_eq!(response.status(), proto::verify_response::Status::Success);
            assert_eq!(
                counter("125", "solidity", "sourcify", "SUCCESS"),
                before + 1
            );
            assert_eq!(
                *calls.lock().unwrap(),
                ["/".to_owned(), format!("/files/any/125/{ADDRESS}")]
            );
        }
        upstream.stop(false).await;
    }
    let mut settings = Settings::default();
    settings.sourcify.request_timeout = 0;
    assert!(SourcifyVerifierService::new(settings.sourcify)
        .await
        .is_err());
}
