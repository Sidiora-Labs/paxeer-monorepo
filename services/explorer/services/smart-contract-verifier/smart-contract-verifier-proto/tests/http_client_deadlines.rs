use actix_web::{
    body::{BodySize, MessageBody},
    web, App, HttpResponse, HttpServer,
};
use smart_contract_verifier_proto::{
    blockscout::smart_contract_verifier::v2 as proto,
    http_client::{health_client, solidity_verifier_client, Client, Config, Error},
};
use std::{
    convert::Infallible,
    future::pending,
    io::Read,
    net::TcpListener,
    pin::Pin,
    sync::{
        atomic::{AtomicBool, AtomicUsize, Ordering},
        mpsc, Arc,
    },
    task::{Context, Poll},
    time::{Duration, Instant},
};

const REQUEST_TIMEOUT: Duration = Duration::from_millis(200);
const WATCHDOG: Duration = Duration::from_secs(3);

#[derive(Clone, Copy)]
enum ResponseMode {
    HeadersStall,
    BodyStall,
    Complete,
}

struct Calls {
    mode: ResponseMode,
    verification: AtomicUsize,
    health: AtomicUsize,
    body_started: AtomicBool,
}

struct StalledBody {
    calls: web::Data<Calls>,
    started: bool,
}

impl MessageBody for StalledBody {
    type Error = Infallible;

    fn size(&self) -> BodySize {
        BodySize::Stream
    }

    fn poll_next(
        mut self: Pin<&mut Self>,
        _cx: &mut Context<'_>,
    ) -> Poll<Option<Result<web::Bytes, Self::Error>>> {
        if self.started {
            Poll::Pending
        } else {
            self.started = true;
            self.calls.body_started.store(true, Ordering::SeqCst);
            Poll::Ready(Some(Ok(web::Bytes::from_static(b"{\"status\":"))))
        }
    }
}

fn verification_request() -> proto::VerifySolidityStandardJsonRequest {
    proto::VerifySolidityStandardJsonRequest {
        bytecode: "00".to_owned(),
        bytecode_type: proto::BytecodeType::DeployedBytecode as i32,
        compiler_version: "v0.8.17+commit.8df45f5f".to_owned(),
        input: "{".to_owned(),
        ..Default::default()
    }
}

async fn respond(calls: web::Data<Calls>, complete: HttpResponse) -> HttpResponse {
    match calls.mode {
        ResponseMode::HeadersStall => pending().await,
        ResponseMode::BodyStall => HttpResponse::Ok()
            .content_type("application/json")
            .message_body(StalledBody {
                calls,
                started: false,
            })
            .unwrap()
            .map_into_boxed_body(),
        ResponseMode::Complete => complete,
    }
}

async fn verification(
    body: web::Json<proto::VerifySolidityStandardJsonRequest>,
    calls: web::Data<Calls>,
) -> HttpResponse {
    assert_eq!(body.into_inner(), verification_request());
    calls.verification.fetch_add(1, Ordering::SeqCst);
    respond(
        calls,
        HttpResponse::Ok().json(proto::VerifyResponse {
            message: "content is not a valid standard json".to_owned(),
            status: proto::verify_response::Status::Failure as i32,
            ..Default::default()
        }),
    )
    .await
}

async fn health(request: actix_web::HttpRequest, calls: web::Data<Calls>) -> HttpResponse {
    assert_eq!(request.query_string(), "service=");
    calls.health.fetch_add(1, Ordering::SeqCst);
    respond(
        calls,
        HttpResponse::Ok().json(proto::HealthCheckResponse {
            status: proto::health_check_response::ServingStatus::Serving as i32,
        }),
    )
    .await
}

fn peer(mode: ResponseMode) -> (String, web::Data<Calls>, actix_web::dev::Server) {
    let listener = TcpListener::bind("127.0.0.1:0").unwrap();
    let url = format!("http://{}", listener.local_addr().unwrap());
    let calls = web::Data::new(Calls {
        mode,
        verification: AtomicUsize::new(0),
        health: AtomicUsize::new(0),
        body_started: AtomicBool::new(false),
    });
    let server_calls = calls.clone();
    let server = HttpServer::new(move || {
        App::new()
            .app_data(server_calls.clone())
            .route("/health", web::get().to(health))
            .route(
                "/api/v2/verifier/solidity/sources:verify-standard-json",
                web::post().to(verification),
            )
    })
    .workers(1)
    .disable_signals()
    .listen(listener)
    .unwrap()
    .run();
    (url, calls, server)
}

fn bounded_config(url: String) -> Config {
    Config::new(url)
        .with_request_timeout(REQUEST_TIMEOUT)
        .with_connect_timeout(Duration::from_millis(100))
}

fn assert_timeout(error: Error) {
    match error {
        Error::Reqwest(error) => assert!(error.is_timeout(), "{error:?}"),
        error => panic!("Expected a usable reqwest timeout, got {error:?}"),
    }
}

async fn verification_stall(mode: ResponseMode) {
    let (url, calls, server) = peer(mode);
    let handle = server.handle();
    let task = actix_web::rt::spawn(server);
    let client = Client::new(bounded_config(url)).await;
    let started = Instant::now();
    let result = actix_web::rt::time::timeout(
        WATCHDOG,
        solidity_verifier_client::verify_standard_json(&client, verification_request()),
    )
    .await;
    let elapsed = started.elapsed();
    handle.stop(false).await;
    task.await.unwrap().unwrap();

    assert_timeout(
        result
            .expect("Verification exceeded its deadline")
            .unwrap_err(),
    );
    assert!(elapsed < WATCHDOG);
    assert_eq!(calls.verification.load(Ordering::SeqCst), 1);
    assert_eq!(
        calls.body_started.load(Ordering::SeqCst),
        matches!(mode, ResponseMode::BodyStall)
    );
}

#[actix_web::test]
async fn verifier_request_deadline_stalled_headers() {
    verification_stall(ResponseMode::HeadersStall).await;
}

#[actix_web::test]
async fn verifier_request_deadline_stalled_body() {
    verification_stall(ResponseMode::BodyStall).await;
}

#[actix_web::test]
async fn verifier_health_request_deadline() {
    for mode in [ResponseMode::HeadersStall, ResponseMode::BodyStall] {
        let (url, calls, server) = peer(mode);
        let handle = server.handle();
        let task = actix_web::rt::spawn(server);
        let client = Client::new(bounded_config(url)).await;
        let result = actix_web::rt::time::timeout(
            WATCHDOG,
            health_client::health(&client, Default::default()),
        )
        .await;
        handle.stop(false).await;
        task.await.unwrap().unwrap();

        assert_timeout(
            result
                .expect("Health request exceeded its deadline")
                .unwrap_err(),
        );
        assert_eq!(calls.health.load(Ordering::SeqCst), 1);
        assert_eq!(
            calls.body_started.load(Ordering::SeqCst),
            matches!(mode, ResponseMode::BodyStall)
        );
    }
}

fn panic_message(payload: Box<dyn std::any::Any + Send>) -> String {
    if let Some(message) = payload.downcast_ref::<String>() {
        message.clone()
    } else if let Some(message) = payload.downcast_ref::<&str>() {
        (*message).to_owned()
    } else {
        panic!("Expected a diagnostic panic message");
    }
}

#[actix_web::test]
async fn verifier_startup_probe_deadline() {
    for mode in [ResponseMode::HeadersStall, ResponseMode::BodyStall] {
        let (url, calls, server) = peer(mode);
        let handle = server.handle();
        let task = actix_web::rt::spawn(server);
        let mut probe = actix_web::rt::spawn(Client::new(bounded_config(url).probe_url(true)));
        let result = actix_web::rt::time::timeout(WATCHDOG, &mut probe).await;
        probe.abort();
        handle.stop(false).await;
        task.await.unwrap().unwrap();

        let error = result
            .expect("Startup probe exceeded its deadline")
            .unwrap_err();
        assert!(error.is_panic());
        let message = panic_message(error.into_panic());
        assert!(
            message.contains("Cannot establish a connection"),
            "{message}"
        );
        assert!(message.contains("timed out"), "{message}");
        assert_eq!(calls.health.load(Ordering::SeqCst), 1);
    }
}

#[actix_web::test]
async fn verifier_connect_deadline_stalled_tls_handshake() {
    let listener = TcpListener::bind("127.0.0.1:0").unwrap();
    listener.set_nonblocking(true).unwrap();
    let url = format!("https://{}", listener.local_addr().unwrap());
    let received = Arc::new(AtomicBool::new(false));
    let server_received = received.clone();
    let (stop, stopping) = mpsc::channel();
    let server = std::thread::spawn(move || {
        let started = Instant::now();
        let mut stream = loop {
            match listener.accept() {
                Ok((stream, _)) => break stream,
                Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                    assert!(started.elapsed() < WATCHDOG, "TLS connection never arrived");
                    std::thread::sleep(Duration::from_millis(5));
                }
                Err(error) => panic!("TLS accept failed: {error}"),
            }
        };
        stream.set_read_timeout(Some(WATCHDOG)).unwrap();
        let mut handshake = [0u8; 1];
        stream.read_exact(&mut handshake).unwrap();
        assert_eq!(handshake[0], 22, "Expected a TLS handshake record");
        server_received.store(true, Ordering::SeqCst);
        stopping.recv_timeout(WATCHDOG).unwrap();
    });
    let client = Client::new(
        Config::new(url)
            .with_request_timeout(Duration::from_secs(10))
            .with_connect_timeout(REQUEST_TIMEOUT),
    )
    .await;
    let started = Instant::now();
    let result =
        actix_web::rt::time::timeout(WATCHDOG, health_client::health(&client, Default::default()))
            .await;
    let elapsed = started.elapsed();
    stop.send(()).unwrap();
    server.join().unwrap();

    assert_timeout(
        result
            .expect("Connect deadline was not applied")
            .unwrap_err(),
    );
    assert!(elapsed < WATCHDOG);
    assert!(received.load(Ordering::SeqCst));
}

#[actix_web::test]
async fn verifier_rejects_invalid_deadlines() {
    for timeout in [Duration::ZERO, Duration::MAX] {
        for (name, config) in [
            (
                "request_timeout",
                Config::new("http://127.0.0.1:1".to_owned()).with_request_timeout(timeout),
            ),
            (
                "connect_timeout",
                Config::new("http://127.0.0.1:1".to_owned()).with_connect_timeout(timeout),
            ),
        ] {
            let error = actix_web::rt::spawn(Client::new(config)).await.unwrap_err();
            assert!(error.is_panic());
            let message = panic_message(error.into_panic());
            assert!(
                message.contains("Invalid client configuration"),
                "{message}"
            );
            assert!(message.contains(name), "{message}");
        }
    }
}

#[actix_web::test]
async fn verifier_deadlines_preserve_successful_round_trip() {
    let (url, calls, server) = peer(ResponseMode::Complete);
    let handle = server.handle();
    let task = actix_web::rt::spawn(server);
    for config in [Config::new(url.clone()), bounded_config(url)] {
        let client = Client::new(config.probe_url(true)).await;
        let response =
            solidity_verifier_client::verify_standard_json(&client, verification_request())
                .await
                .unwrap();
        assert_eq!(
            response.status,
            proto::verify_response::Status::Failure as i32
        );
        assert_eq!(response.message, "content is not a valid standard json");
        let health = health_client::health(&client, Default::default())
            .await
            .unwrap();
        assert_eq!(
            health.status,
            proto::health_check_response::ServingStatus::Serving as i32
        );
    }
    handle.stop(false).await;
    task.await.unwrap().unwrap();
    assert_eq!(calls.verification.load(Ordering::SeqCst), 2);
    assert_eq!(calls.health.load(Ordering::SeqCst), 4);
}
