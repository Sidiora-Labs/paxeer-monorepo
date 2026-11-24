//! Task 22.33.594 repair record (server boundary).
//! metric-chain-cardinality -> metric_chain_cardinality.
//! compilation-admission-unbounded -> compilation_admission_unbounded (admission,
//! queue/response deadlines and cancellation accounting through the real verifier).
//! startup-requires-live-compiler-catalogues -> startup_catalogue_outage_isolated_across_restart.
//! Wiring request, domain 14: smart-contract-verifier/src/compiler/fetcher_versions.rs
//! must persist authenticated catalogue metadata, validate its origin/integrity and expiry
//! on restart, and make cached verified compilers available without a live catalogue.
//! Wiring request, domain 14: smart-contract-verifier/src/compiler/compilers.rs and
//! compiler implementations must bound wall time/output/resources and terminate/reap
//! compiler processes before releasing pool permits; a response timeout cannot do this.
//! Wiring request, domain 14: server/src/services/*_verifier.rs must bound existing raw
//! request log fields; metrics.rs rate-limits and truncates its own unknown-chain samples.
//! Baseline verification was not run: the explicit gate order forbids verification
//! before the complete implementation and requires hf-build before hf-done.
//! These tests do not claim compiler-process or authenticated warm-cache recovery proof.

use serde_json::json;
use smart_contract_verifier_proto::blockscout::smart_contract_verifier::v2::{
    health_check_response::ServingStatus, health_client::HealthClient,
    solidity_verifier_client::SolidityVerifierClient,
    sourcify_verifier_client::SourcifyVerifierClient, verify_response,
    vyper_verifier_client::VyperVerifierClient, BytecodeType, HealthCheckRequest,
    ListCompilerVersionsRequest, VerifySolidityMultiPartRequest, VerifySourcifyRequest,
    VerifyVyperMultiPartRequest,
};
use smart_contract_verifier_server::{run, Settings};
use std::{
    net::{SocketAddr, TcpListener},
    sync::{
        atomic::{AtomicBool, AtomicUsize, Ordering},
        Arc,
    },
    time::{Duration, Instant},
};
use tokio::{
    io::{AsyncReadExt, AsyncWriteExt},
    sync::{oneshot, Semaphore},
    task::JoinHandle,
    time::{sleep, timeout},
};
use tonic::{transport::Channel, Code};

const SOLIDITY: &str = "blockscout.smartContractVerifier.v2.SolidityVerifier";
const SOURCIFY: &str = "blockscout.smartContractVerifier.v2.SourcifyVerifier";

// A loopback HTTP peer serving recorded catalogue and Sourcify response transcripts.
// The production fetcher, verifier, generated gRPC client/server and metrics stay real.
struct Peer {
    address: SocketAddr,
    catalogue_stalled: Arc<AtomicBool>,
    verification_stalled: Arc<AtomicBool>,
    entered: Arc<Semaphore>,
    release: Arc<Semaphore>,
    completed: Arc<Semaphore>,
    requests: Arc<AtomicUsize>,
    server: JoinHandle<()>,
}

impl Peer {
    async fn start() -> Self {
        let listener = tokio::net::TcpListener::bind("127.0.0.1:0").await.unwrap();
        let address = listener.local_addr().unwrap();
        let catalogue_stalled = Arc::new(AtomicBool::new(false));
        let verification_stalled = Arc::new(AtomicBool::new(false));
        let entered = Arc::new(Semaphore::new(0));
        let release = Arc::new(Semaphore::new(0));
        let completed = Arc::new(Semaphore::new(0));
        let requests = Arc::new(AtomicUsize::new(0));
        let state = (
            catalogue_stalled.clone(),
            verification_stalled.clone(),
            entered.clone(),
            release.clone(),
            completed.clone(),
            requests.clone(),
        );
        let server = tokio::spawn(async move {
            loop {
                let (mut stream, _) = listener.accept().await.unwrap();
                let (catalogue_stalled, stalled, entered, release, completed, requests) =
                    state.clone();
                tokio::spawn(async move {
                    let mut bytes = Vec::new();
                    let header_end = loop {
                        let mut byte = [0];
                        if stream.read_exact(&mut byte).await.is_err() {
                            return;
                        }
                        bytes.push(byte[0]);
                        assert!(bytes.len() <= 64 * 1024);
                        if bytes.ends_with(b"\r\n\r\n") {
                            break bytes.len();
                        }
                    };
                    let headers = std::str::from_utf8(&bytes).unwrap();
                    let catalogue = headers.starts_with("GET /list ");
                    let length = headers
                        .lines()
                        .filter_map(|line| line.split_once(':'))
                        .find(|(name, _)| name.eq_ignore_ascii_case("content-length"))
                        .map(|(_, length)| length.trim().parse::<usize>().unwrap())
                        .unwrap_or(0);
                    assert!(length <= 2 * 1024 * 1024);
                    bytes.resize(header_end + length, 0);
                    stream.read_exact(&mut bytes[header_end..]).await.unwrap();
                    if catalogue {
                        if catalogue_stalled.load(Ordering::SeqCst) {
                            // Wait for the real client's startup timeout to close the socket.
                            let mut byte = [0];
                            let _ = stream.read(&mut byte).await;
                            return;
                        }
                    } else {
                        let body: serde_json::Value =
                            serde_json::from_slice(&bytes[header_end..]).unwrap();
                        assert!(body["chain"].is_string());
                        assert_eq!(
                            body["address"],
                            "0x0000000000000000000000000000000000000001"
                        );
                        requests.fetch_add(1, Ordering::SeqCst);
                        entered.add_permits(1);
                        if stalled.load(Ordering::SeqCst) {
                            release.acquire().await.unwrap().forget();
                        }
                    }
                    let body = if catalogue {
                        r#"{"builds":[]}"#
                    } else {
                        r#"{"error":"recorded verification refusal"}"#
                    };
                    let response = format!("HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{}", body.len(), body);
                    let _ = stream.write_all(response.as_bytes()).await;
                    if !catalogue {
                        completed.add_permits(1);
                    }
                });
            }
        });
        Self {
            address,
            catalogue_stalled,
            verification_stalled,
            entered,
            release,
            completed,
            requests,
            server,
        }
    }

    async fn wait_entered(&self) {
        timeout(Duration::from_secs(5), self.entered.acquire())
            .await
            .unwrap()
            .unwrap()
            .forget();
    }
}

impl Drop for Peer {
    fn drop(&mut self) {
        self.server.abort();
    }
}

struct Server {
    channel: Channel,
    stop: Option<oneshot::Sender<()>>,
    thread: Option<std::thread::JoinHandle<()>>,
}

impl Server {
    async fn start(peer: &Peer, cache: &std::path::Path, catalogue_enabled: bool) -> Self {
        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        let address = listener.local_addr().unwrap();
        drop(listener);
        let settings: Settings = serde_json::from_value(json!({
            "server": {"http": {"enabled": false}, "grpc": {"enabled": true, "addr": address}},
            "metrics": {"enabled": false}, "tracing": {"enabled": false},
            "compilers": {"max_threads": 1},
            "solidity": {"enabled": catalogue_enabled, "compilers_dir": cache.join("solidity"),
                "fetcher": {"list": {"list_url": format!("http://{}/list", peer.address)}}},
            "vyper": {"enabled": catalogue_enabled, "compilers_dir": cache.join("vyper"),
                "fetcher": {"list": {"list_url": format!("http://{}/list", peer.address)}}},
            "zksync_solidity": {"enabled": false},
            "sourcify": {"api_url": format!("http://{}/", peer.address), "request_timeout": 120,
                "verification_attempts": 1}
        }))
        .unwrap();
        let (stop, stopped) = oneshot::channel();
        let thread = std::thread::spawn(move || {
            // Dropping this runtime stops listeners and all server tasks before reopening.
            let runtime = tokio::runtime::Builder::new_multi_thread()
                .worker_threads(2)
                .enable_all()
                .build()
                .unwrap();
            runtime.block_on(async move {
                tokio::select! {
                    result = run(settings) => panic!("server stopped unexpectedly: {result:?}"),
                    _ = stopped => {},
                }
            });
        });
        let channel = timeout(Duration::from_secs(15), async {
            loop {
                match Channel::from_shared(format!("http://{address}"))
                    .unwrap()
                    .connect()
                    .await
                {
                    Ok(channel) => break channel,
                    Err(_) => sleep(Duration::from_millis(20)).await,
                }
            }
        })
        .await
        .expect("server starts within its provider deadline");
        Self {
            channel,
            stop: Some(stop),
            thread: Some(thread),
        }
    }

    async fn stop(mut self) {
        self.stop.take().unwrap().send(()).unwrap();
        let thread = self.thread.take().unwrap();
        tokio::task::spawn_blocking(move || thread.join().unwrap())
            .await
            .unwrap();
    }

    fn client(&self) -> SourcifyVerifierClient<Channel> {
        SourcifyVerifierClient::new(self.channel.clone())
    }
}

impl Drop for Server {
    fn drop(&mut self) {
        if let Some(stop) = self.stop.take() {
            let _ = stop.send(());
        }
        if let Some(thread) = self.thread.take() {
            thread.join().unwrap();
        }
    }
}

fn request(chain: impl Into<String>) -> VerifySourcifyRequest {
    VerifySourcifyRequest {
        address: "0x0000000000000000000000000000000000000001".into(),
        chain: chain.into(),
        files: [("metadata.json".into(), "{}".into())].into(),
        chosen_contract: None,
    }
}

fn solidity_request() -> VerifySolidityMultiPartRequest {
    VerifySolidityMultiPartRequest {
        bytecode: "0x00".into(),
        bytecode_type: BytecodeType::DeployedBytecode as i32,
        compiler_version: "v0.8.17+commit.8df45f5f".into(),
        source_files: [("A.sol".into(), "contract A {}".into())].into(),
        ..Default::default()
    }
}

#[tokio::test]
async fn metric_chain_cardinality() {
    let peer = Peer::start().await;
    let cache = tempfile::tempdir().unwrap();
    let server = Server::start(&peer, cache.path(), false).await;
    let mut client = server.client();
    for chain in (10000..10512).map(|chain| chain.to_string()).chain([
        "125".into(),
        "00125".into(),
        "0x7d".into(),
        " 125 ".into(),
        "x".repeat(4096),
        "１２５".into(),
        "18446744073709551616".into(),
    ]) {
        let response = client.verify(request(chain)).await.unwrap().into_inner();
        assert_eq!(response.status(), verify_response::Status::Failure);
        assert_eq!(response.message, "recorded verification refusal");
    }
    assert_eq!(peer.requests.load(Ordering::SeqCst), 519);
    let metric = prometheus::gather()
        .into_iter()
        .find(|family| family.get_name() == "smart_contract_verifier_verify_contract")
        .unwrap();
    let mut chains = std::collections::BTreeSet::new();
    for series in metric.get_metric() {
        if series
            .get_label()
            .iter()
            .any(|label| label.get_name() == "endpoint" && label.get_value() == "sourcify")
        {
            chains.insert(
                series
                    .get_label()
                    .iter()
                    .find(|label| label.get_name() == "chain_id")
                    .unwrap()
                    .get_value()
                    .to_owned(),
            );
        }
    }
    assert_eq!(chains, ["125".to_owned(), "unknown".to_owned()].into());
    assert_eq!(
        metric.get_metric().len(),
        2,
        "caller-controlled chains cannot create additional series"
    );
    server.stop().await;
}

#[tokio::test]
async fn compilation_admission_unbounded() {
    let peer = Peer::start().await;
    peer.verification_stalled.store(true, Ordering::SeqCst);
    let cache = tempfile::tempdir().unwrap();
    let server = Server::start(&peer, cache.path(), true).await;
    let mut first_client = server.client();
    let first = tokio::spawn(async move { first_client.verify(request("125")).await });
    peer.wait_entered().await;
    let mut queued_client = SolidityVerifierClient::new(server.channel.clone());
    let queued =
        tokio::spawn(async move { queued_client.verify_multi_part(solidity_request()).await });
    sleep(Duration::from_millis(100)).await;
    let error = VyperVerifierClient::new(server.channel.clone())
        .verify_multi_part(VerifyVyperMultiPartRequest {
            bytecode: "0x00".into(),
            bytecode_type: BytecodeType::DeployedBytecode as i32,
            compiler_version: "v0.3.10+commit.9136169".into(),
            source_files: [("A.vy".into(), "# @version 0.3.10".into())].into(),
            ..Default::default()
        })
        .await
        .unwrap_err();
    assert_eq!(error.code(), Code::ResourceExhausted);
    assert!(error.message().contains("bounded queue is full"));
    let error = queued.await.unwrap().unwrap_err();
    assert_eq!(error.code(), Code::DeadlineExceeded);
    assert!(error.message().contains("queue deadline"));
    assert_eq!(peer.requests.load(Ordering::SeqCst), 1);

    // The real response deadline fires without granting a new slot to a still-running worker.
    let error = timeout(Duration::from_secs(32), first)
        .await
        .unwrap()
        .unwrap()
        .unwrap_err();
    assert_eq!(error.code(), Code::DeadlineExceeded);
    assert!(error.message().contains("response deadline"));
    assert_eq!(
        server
            .client()
            .verify(request("125"))
            .await
            .unwrap_err()
            .code(),
        Code::DeadlineExceeded
    );
    assert_eq!(peer.requests.load(Ordering::SeqCst), 1);
    peer.verification_stalled.store(false, Ordering::SeqCst);
    peer.release.add_permits(1);
    timeout(Duration::from_secs(5), peer.completed.acquire())
        .await
        .unwrap()
        .unwrap()
        .forget();
    assert_eq!(
        server
            .client()
            .verify(request("125"))
            .await
            .unwrap()
            .into_inner()
            .status(),
        verify_response::Status::Failure
    );

    // Cancellation must also retain capacity until the actual operation finishes.
    peer.verification_stalled.store(true, Ordering::SeqCst);
    // Drain entry notifications from the two preceding requests.
    while let Ok(permit) = peer.entered.try_acquire() {
        permit.forget();
    }
    let mut cancelled_client = server.client();
    let cancelled = tokio::spawn(async move { cancelled_client.verify(request("125")).await });
    peer.wait_entered().await;
    cancelled.abort();
    assert!(cancelled.await.unwrap_err().is_cancelled());
    assert_eq!(
        server
            .client()
            .verify(request("125"))
            .await
            .unwrap_err()
            .code(),
        Code::DeadlineExceeded
    );
    peer.verification_stalled.store(false, Ordering::SeqCst);
    peer.release.add_permits(1);
    let response = server
        .client()
        .verify(request("125"))
        .await
        .unwrap()
        .into_inner();
    assert_eq!(response.status(), verify_response::Status::Failure);
    server.stop().await;
}

#[tokio::test]
async fn startup_catalogue_outage_isolated_across_restart() {
    let peer = Peer::start().await;
    let cache = tempfile::tempdir().unwrap();
    let server = Server::start(&peer, cache.path(), true).await;
    let mut health = HealthClient::new(server.channel.clone());
    assert_eq!(
        health
            .check(HealthCheckRequest {
                service: SOLIDITY.into()
            })
            .await
            .unwrap()
            .into_inner()
            .status(),
        ServingStatus::Serving
    );
    let versions = SolidityVerifierClient::new(server.channel.clone())
        .list_compiler_versions(ListCompilerVersionsRequest {})
        .await
        .unwrap()
        .into_inner();
    assert!(versions.compiler_versions.is_empty());
    // No compiler is invented for an empty but valid provider catalogue.
    let error = SolidityVerifierClient::new(server.channel.clone())
        .verify_multi_part(solidity_request())
        .await
        .unwrap_err();
    assert_eq!(error.code(), Code::InvalidArgument);
    server.stop().await;

    peer.catalogue_stalled.store(true, Ordering::SeqCst);
    let start = Instant::now();
    let reopened = Server::start(&peer, cache.path(), true).await;
    assert!(
        start.elapsed() < Duration::from_secs(15),
        "provider deadlines run concurrently"
    );
    let mut health = HealthClient::new(reopened.channel.clone());
    assert_eq!(
        health
            .check(HealthCheckRequest {
                service: SOLIDITY.into()
            })
            .await
            .unwrap()
            .into_inner()
            .status(),
        ServingStatus::NotServing
    );
    assert_eq!(
        health
            .check(HealthCheckRequest {
                service: SOURCIFY.into()
            })
            .await
            .unwrap()
            .into_inner()
            .status(),
        ServingStatus::Serving
    );
    assert_eq!(
        health
            .check(HealthCheckRequest {
                service: String::new()
            })
            .await
            .unwrap()
            .into_inner()
            .status(),
        ServingStatus::Serving
    );
    let error = SolidityVerifierClient::new(reopened.channel.clone())
        .list_compiler_versions(ListCompilerVersionsRequest {})
        .await
        .unwrap_err();
    assert_eq!(error.code(), Code::Unavailable);
    assert!(error.message().contains("catalogue initialization failed"));
    assert_eq!(
        reopened
            .client()
            .verify(request("125"))
            .await
            .unwrap()
            .into_inner()
            .status(),
        verify_response::Status::Failure
    );
    reopened.stop().await;
}
