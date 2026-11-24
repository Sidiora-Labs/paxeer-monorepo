use actix_web::{test, web, App, HttpResponse, HttpServer};
use ethabi::Contract;
use serde_json::{json, Value};
use sig_provider_proto::blockscout::sig_provider::v1::{
    abi_service_actix::route_abi_service, health_actix::route_health,
    health_check_response::ServingStatus, health_client::HealthClient, health_server::HealthServer,
    signature_service_actix::route_signature_service, HealthCheckRequest,
};
use sig_provider_server::{health::HealthService, new_service, SourcesSettings};
use std::{
    collections::BTreeMap,
    net::TcpListener,
    sync::{
        atomic::{AtomicU8, AtomicUsize, Ordering},
        Arc, Mutex,
    },
    time::Duration,
};
use tokio::sync::{Notify, Semaphore};

const ABI: &str = r#"[
    {"type":"function","name":"set","inputs":[{"name":"value","type":"uint256"}],"outputs":[{"name":"","type":"uint256"}],"stateMutability":"nonpayable"},
    {"type":"event","name":"Set","inputs":[{"name":"value","type":"uint256","indexed":true}],"anonymous":false}
]"#;
const SERVICE_NAMES: &[&str] = &[
    "",
    "blockscout.sig_provider.v1.Health",
    "blockscout.sig_provider.v1.SignatureService",
    "blockscout.sig_provider.v1.AbiService",
];
const BEFORE_DELIVERY: u8 = 1;
const BEFORE_REPLY: u8 = 2;
const HTTP_REFUSAL: u8 = 3;
const UNCONFIRMED: u8 = 4;
const EVENT_REFUSAL: u8 = 5;

#[derive(Default)]
struct Signatures {
    functions: BTreeMap<String, String>,
    events: BTreeMap<String, String>,
}

struct Peer {
    sigeth: bool,
    mode: AtomicU8,
    imports: AtomicUsize,
    accepted: Notify,
    release: Semaphore,
    stored: Mutex<Signatures>,
}

impl Peer {
    fn store(&self, abi: &str) {
        let contract = Contract::load(abi.as_bytes()).unwrap();
        let mut stored = self.stored.lock().unwrap();
        for function in contract.functions() {
            let signature = format!(
                "{}({})",
                function.name,
                function
                    .inputs
                    .iter()
                    .map(|input| input.kind.to_string())
                    .collect::<Vec<_>>()
                    .join(",")
            );
            stored
                .functions
                .insert(hex::encode(function.short_signature()), signature);
        }
        for event in contract.events() {
            let signature = format!(
                "{}({})",
                event.name,
                event
                    .inputs
                    .iter()
                    .map(|input| input.kind.to_string())
                    .collect::<Vec<_>>()
                    .join(",")
            );
            stored
                .events
                .insert(hex::encode(event.signature()), signature);
        }
    }

    fn assert_delivered(&self) {
        let stored = self.stored.lock().unwrap();
        assert_eq!(
            stored
                .functions
                .values()
                .map(String::as_str)
                .collect::<Vec<_>>(),
            vec!["set(uint256)"]
        );
        assert_eq!(
            stored
                .events
                .values()
                .map(String::as_str)
                .collect::<Vec<_>>(),
            vec!["Set(uint256)"]
        );
    }
}

async fn import(peer: web::Data<Peer>, body: web::Json<Value>) -> HttpResponse {
    peer.imports.fetch_add(1, Ordering::SeqCst);
    let abi = if peer.sigeth {
        assert_eq!(body["type"], "abi");
        assert_eq!(body["data"].as_array().unwrap().len(), 1);
        body["data"][0].to_string()
    } else {
        body["contract_abi"].as_str().unwrap().to_owned()
    };
    assert_eq!(
        serde_json::from_str::<Value>(&abi).unwrap(),
        serde_json::from_str::<Value>(ABI).unwrap()
    );
    let mode = peer.mode.load(Ordering::SeqCst);
    if mode == HTTP_REFUSAL {
        return HttpResponse::BadRequest().json(json!({"ok": false, "error": "import refused"}));
    }
    if mode == BEFORE_DELIVERY {
        peer.accepted.notify_one();
        peer.release.acquire().await.unwrap().forget();
    }
    if mode != UNCONFIRMED {
        peer.store(&abi);
    }
    if mode == BEFORE_REPLY {
        peer.accepted.notify_one();
        peer.release.acquire().await.unwrap().forget();
    }
    if peer.sigeth {
        HttpResponse::Ok().json(json!({"ok": mode != UNCONFIRMED, "result": {}}))
    } else {
        HttpResponse::Created().json(
            json!({"num_processed": 2, "num_imported": 2, "num_duplicates": 0, "num_ignored": 0}),
        )
    }
}

async fn lookup(
    peer: web::Data<Peer>,
    query: web::Query<BTreeMap<String, String>>,
) -> HttpResponse {
    if peer.mode.load(Ordering::SeqCst) == HTTP_REFUSAL {
        return HttpResponse::BadRequest().finish();
    }
    let stored = peer.stored.lock().unwrap();
    if peer.sigeth {
        let (kind, key, signatures) = if let Some(key) = query.get("function") {
            ("function", key, &stored.functions)
        } else {
            ("event", query.get("event").unwrap(), &stored.events)
        };
        let names: Vec<_> = signatures
            .get(key.trim_start_matches("0x"))
            .filter(|_| kind != "event" || peer.mode.load(Ordering::SeqCst) != EVENT_REFUSAL)
            .map(|name| json!({"name": name, "filtered": false}))
            .into_iter()
            .collect();
        let mut response = json!({"ok": true, "result": {"function": {}, "event": {}}});
        response["result"][kind][key] = json!(names);
        HttpResponse::Ok().json(response)
    } else {
        let key = query.get("hex_signature").unwrap();
        let signatures = if query.contains_key("event") {
            &stored.events
        } else {
            &stored.functions
        };
        let results: Vec<_> = signatures
            .get(key)
            .map(|name| json!({"text_signature": name}))
            .into_iter()
            .collect();
        HttpResponse::Ok().json(json!({"next": null, "results": results}))
    }
}

async fn event_lookup(
    peer: web::Data<Peer>,
    query: web::Query<BTreeMap<String, String>>,
) -> HttpResponse {
    let mut query = query.into_inner();
    query.insert("event".into(), String::new());
    lookup(peer, web::Query(query)).await
}

struct Upstream {
    url: url::Url,
    peer: Arc<Peer>,
    handle: actix_web::dev::ServerHandle,
    task: tokio::task::JoinHandle<std::io::Result<()>>,
}

impl Upstream {
    async fn start(sigeth: bool) -> Self {
        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        let url = format!("http://{}/", listener.local_addr().unwrap())
            .parse()
            .unwrap();
        let peer = Arc::new(Peer {
            sigeth,
            mode: AtomicU8::new(0),
            imports: AtomicUsize::new(0),
            accepted: Notify::new(),
            release: Semaphore::new(0),
            stored: Mutex::new(Signatures::default()),
        });
        let data = web::Data::from(peer.clone());
        let server = HttpServer::new(move || {
            App::new()
                .app_data(data.clone())
                .route("/api/v1/import", web::post().to(import))
                .route("/api/v1/import-solidity/", web::post().to(import))
                .route("/api/v1/signatures", web::get().to(lookup))
                .route("/api/v1/signatures/", web::get().to(lookup))
                .route("/api/v1/event-signatures/", web::get().to(event_lookup))
        })
        .workers(1)
        .disable_signals()
        .listen(listener)
        .unwrap()
        .run();
        let handle = server.handle();
        let task = actix_web::rt::spawn(server);
        Self {
            url,
            peer,
            handle,
            task,
        }
    }

    async fn stop(self) {
        self.peer.release.add_permits(10);
        self.handle.stop(false).await;
        self.task.await.unwrap().unwrap();
    }
}

struct Upstreams {
    sigeth: Upstream,
    fourbyte: Upstream,
}

impl Upstreams {
    async fn start() -> Self {
        Self {
            sigeth: Upstream::start(true).await,
            fourbyte: Upstream::start(false).await,
        }
    }

    fn settings(&self) -> SourcesSettings {
        let mut settings = SourcesSettings::default();
        settings.precompiles.enabled = false;
        settings.eth_bytecode_db.enabled = false;
        settings.sigeth = self.sigeth.url.clone();
        settings.fourbyte = self.fourbyte.url.clone();
        settings
    }

    async fn stop(self) {
        self.sigeth.stop().await;
        self.fourbyte.stop().await;
    }
}

macro_rules! application {
    ($upstreams:expr) => {{
        let service = new_service($upstreams.settings());
        test::init_service(App::new().configure(move |config| {
            route_signature_service(config, service.clone());
            route_abi_service(config, service.clone());
            route_health(config, Arc::new(HealthService::default()));
        }))
        .await
    }};
}

fn import_request(abi: &str) -> actix_web::test::TestRequest {
    test::TestRequest::post()
        .uri("/api/v1/signatures")
        .set_json(json!({"abi": abi}))
}

#[actix_web::test]
async fn health_service_name_http() {
    let app = test::init_service(App::new().configure(|config| {
        route_health(config, Arc::new(HealthService::default()));
    }))
    .await;
    for name in SERVICE_NAMES {
        let uri = format!("/health?service={name}");
        let response =
            test::call_service(&app, test::TestRequest::get().uri(&uri).to_request()).await;
        assert_eq!(response.status(), actix_web::http::StatusCode::OK);
        let body: Value = test::read_body_json(response).await;
        assert_eq!(body, json!({"status": "SERVING"}));
    }
    let response =
        test::call_service(&app, test::TestRequest::get().uri("/health").to_request()).await;
    assert_eq!(response.status(), actix_web::http::StatusCode::OK);
    for name in [
        "unknown",
        "sig_provider",
        "blockscout.sig_provider.v1.AbiService.Typo",
    ] {
        let response = test::call_service(
            &app,
            test::TestRequest::get()
                .uri(&format!("/health?service={name}"))
                .to_request(),
        )
        .await;
        assert_eq!(response.status(), actix_web::http::StatusCode::NOT_FOUND);
        let body: Value = test::read_body_json(response).await;
        assert_eq!(body["code"], tonic::Code::NotFound as i32);
    }
}

#[tokio::test]
async fn health_service_name_grpc() {
    let listener = tokio::net::TcpListener::bind("127.0.0.1:0").await.unwrap();
    let address = listener.local_addr().unwrap();
    let incoming = futures::stream::unfold(listener, |listener| async move {
        let result = listener.accept().await.map(|(stream, _)| stream);
        Some((result, listener))
    });
    let (stop, stopped) = tokio::sync::oneshot::channel();
    let server = tokio::spawn(
        tonic::transport::Server::builder()
            .add_service(HealthServer::new(HealthService::default()))
            .serve_with_incoming_shutdown(incoming, async {
                stopped.await.unwrap();
            }),
    );
    let mut client = HealthClient::connect(format!("http://{address}"))
        .await
        .unwrap();
    for name in SERVICE_NAMES {
        let response = client
            .check(HealthCheckRequest {
                service: (*name).into(),
            })
            .await
            .unwrap();
        assert_eq!(response.into_inner().status, ServingStatus::Serving as i32);
    }
    for name in [
        "unknown",
        "sig_provider",
        "blockscout.sig_provider.v1.SignatureService.Typo",
    ] {
        let error = client
            .check(HealthCheckRequest {
                service: name.into(),
            })
            .await
            .unwrap_err();
        assert_eq!(error.code(), tonic::Code::NotFound);
    }
    drop(client);
    stop.send(()).unwrap();
    server.await.unwrap().unwrap();
}

#[actix_web::test]
async fn signature_import_invalid_abi() {
    let upstreams = Upstreams::start().await;
    let app = application!(upstreams);
    for abi in [
        "not JSON",
        "{}",
        r#"[{"type":"function","name":"bad","inputs":[{"name":"x","type":"not-a-solidity-type"}],"outputs":[]}]"#,
        r#"[{"type":"function","name":"bad","inputs":[{"name":"x","type":"]"}],"outputs":[]}]"#,
        r#"[{"type":"function","name":"bad","inputs":[],"outputs":[{"name":"x","type":"uint7"}]}]"#,
        r#"[{"type":"event","name":"Bad","inputs":[{"name":"x","type":"tuple[]","components":[{"name":"y","type":"bytes33"}],"indexed":true}],"anonymous":false}]"#,
        r#"[{"type":"function","name":"bad","inputs":[{"name":"x","type":"uint256[0]"}],"outputs":[]}]"#,
    ] {
        let response = test::call_service(&app, import_request(abi).to_request()).await;
        assert_eq!(response.status(), actix_web::http::StatusCode::BAD_REQUEST);
        let body: Value = test::read_body_json(response).await;
        assert_eq!(body["code"], tonic::Code::InvalidArgument as i32);
    }
    assert_eq!(upstreams.sigeth.peer.imports.load(Ordering::SeqCst), 0);
    assert_eq!(upstreams.fourbyte.peer.imports.load(Ordering::SeqCst), 0);
    drop(app);
    upstreams.stop().await;
}

#[actix_web::test]
async fn signature_import_upstream_failure() {
    let upstreams = Upstreams::start().await;
    for peer in [&upstreams.sigeth.peer, &upstreams.fourbyte.peer] {
        peer.mode.store(HTTP_REFUSAL, Ordering::SeqCst);
    }
    let app = application!(upstreams);
    let response = test::call_service(&app, import_request(ABI).to_request()).await;
    assert_eq!(
        response.status(),
        actix_web::http::StatusCode::SERVICE_UNAVAILABLE
    );
    let body: Value = test::read_body_json(response).await;
    assert_eq!(body["code"], tonic::Code::Unavailable as i32);
    for peer in [&upstreams.sigeth.peer, &upstreams.fourbyte.peer] {
        assert_eq!(peer.imports.load(Ordering::SeqCst), 1);
        assert!(peer.stored.lock().unwrap().functions.is_empty());
        assert!(peer.stored.lock().unwrap().events.is_empty());
    }
    drop(app);
    upstreams.stop().await;
}

#[actix_web::test]
async fn signature_import_requires_function_and_event_confirmation() {
    let upstreams = Upstreams::start().await;
    let app = application!(upstreams);
    for mode in [UNCONFIRMED, EVENT_REFUSAL] {
        upstreams.sigeth.peer.mode.store(mode, Ordering::SeqCst);
        let response = test::call_service(&app, import_request(ABI).to_request()).await;
        assert_eq!(
            response.status(),
            actix_web::http::StatusCode::SERVICE_UNAVAILABLE
        );
        let body: Value = test::read_body_json(response).await;
        assert_eq!(body["code"], tonic::Code::Unavailable as i32);
    }
    drop(app);
    upstreams.stop().await;
}

async fn restart_import(mode: u8) {
    let upstreams = Upstreams::start().await;
    upstreams.sigeth.peer.mode.store(mode, Ordering::SeqCst);
    let app = application!(upstreams);
    let mut response = Box::pin(test::call_service(&app, import_request(ABI).to_request()));
    tokio::select! {
        biased;
        _ = &mut response => panic!("import was acknowledged before confirmed delivery"),
        received = tokio::time::timeout(Duration::from_secs(5), upstreams.sigeth.peer.accepted.notified()) => {
            received.expect("upstream must receive the production import request");
        },
    }
    assert!(
        tokio::time::timeout(Duration::from_millis(100), &mut response)
            .await
            .is_err(),
        "import must remain pending while delivery or its reply is blocked"
    );
    if mode == BEFORE_REPLY {
        upstreams.sigeth.peer.assert_delivered();
    } else {
        assert!(upstreams
            .sigeth
            .peer
            .stored
            .lock()
            .unwrap()
            .functions
            .is_empty());
        assert!(upstreams
            .sigeth
            .peer
            .stored
            .lock()
            .unwrap()
            .events
            .is_empty());
    }
    // Dropping the app and its request stops the in-flight service. There is
    // no acknowledged job to lose; a retry uses fresh service/source clients.
    drop(response);
    drop(app);
    upstreams.sigeth.peer.mode.store(0, Ordering::SeqCst);
    upstreams.sigeth.peer.release.add_permits(10);

    let reopened = application!(upstreams);
    let response = test::call_service(&reopened, import_request(ABI).to_request()).await;
    assert_eq!(response.status(), actix_web::http::StatusCode::OK);
    let body: Value = test::read_body_json(response).await;
    assert_eq!(body, json!({}));
    upstreams.sigeth.peer.assert_delivered();
    upstreams.fourbyte.peer.assert_delivered();
    drop(reopened);

    let reopened = application!(upstreams);
    let contract = Contract::load(ABI.as_bytes()).unwrap();
    let function = contract.function("set").unwrap();
    let calldata = function
        .encode_input(&[ethabi::Token::Uint(7.into())])
        .unwrap();
    let response = test::call_service(
        &reopened,
        test::TestRequest::get()
            .uri(&format!(
                "/api/v1/abi/function?txInput=0x{}",
                hex::encode(calldata)
            ))
            .to_request(),
    )
    .await;
    assert_eq!(response.status(), actix_web::http::StatusCode::OK);
    let body: Value = test::read_body_json(response).await;
    assert_eq!(body[0]["name"], "set");
    assert_eq!(body[0]["inputs"][0]["value"], "7");
    let event = contract.event("Set").unwrap();
    let response = test::call_service(
        &reopened,
        test::TestRequest::get()
            .uri(&format!(
                "/api/v1/abi/event?data=0x&topics=0x{},0x{}",
                hex::encode(event.signature()),
                hex::encode(ethabi::encode(&[ethabi::Token::Uint(7.into())]))
            ))
            .to_request(),
    )
    .await;
    assert_eq!(response.status(), actix_web::http::StatusCode::OK);
    let body: Value = test::read_body_json(response).await;
    assert_eq!(body[0]["name"], "Set");
    assert_eq!(body[0]["inputs"][0]["value"], "7");
    drop(reopened);
    upstreams.stop().await;
}

#[actix_web::test]
async fn signature_import_restart_before_delivery() {
    restart_import(BEFORE_DELIVERY).await;
}

#[actix_web::test]
async fn signature_import_restart_after_upstream_acceptance() {
    restart_import(BEFORE_REPLY).await;
}
