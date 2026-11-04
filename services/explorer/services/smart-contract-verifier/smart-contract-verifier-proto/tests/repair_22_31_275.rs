use smart_contract_verifier_proto::{
    blockscout::smart_contract_verifier::v2 as proto,
    http_client::{health_client, solidity_verifier_client, Client, Config, Error},
};
use std::{
    net::TcpListener,
    sync::{
        atomic::{AtomicUsize, Ordering},
        Arc,
    },
};

#[path = "../../../libs/blockscout-service-launcher/src/launcher/router.rs"]
mod launcher_router;
#[path = "../../smart-contract-verifier-server/src/services/health.rs"]
mod production_health;

#[derive(Clone)]
struct HealthRouter;

impl launcher_router::HttpRouter for HealthRouter {
    fn register_routes(&self, config: &mut actix_web::web::ServiceConfig) {
        proto::health_actix::route_health(
            config,
            Arc::new(production_health::HealthService::default()),
        );
    }
}

#[actix_web::test]
async fn repair_22_31_275_launcher_health_and_probe() {
    for mount in [None, Some("/verifier"), Some("/nested/verifier")] {
        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        let address = listener.local_addr().unwrap();
        let server = actix_web::HttpServer::new(move || {
            actix_web::App::new().configure(launcher_router::configure_router(
                &HealthRouter,
                mount.map(str::to_owned),
            ))
        })
        .workers(1)
        .disable_signals()
        .listen(listener)
        .unwrap()
        .run();
        let handle = server.handle();
        let task = actix_web::rt::spawn(server);

        for trailing_slash in ["", "/"] {
            let config = Config::new(format!(
                "http://{address}{}{trailing_slash}?discard=old#discard",
                mount.unwrap_or_default(),
            ))
            .probe_url(true);
            let client = Client::new(config).await;
            let response =
                health_client::health(
                    &client,
                    proto::HealthCheckRequest {
                        service: <proto::health_server::HealthServer<
                            production_health::HealthService,
                        > as tonic::server::NamedService>::NAME
                            .to_owned(),
                    },
                )
                .await
                .unwrap();
            assert_eq!(
                response.status,
                proto::health_check_response::ServingStatus::Serving as i32,
            );

            let error = health_client::health(
                &client,
                proto::HealthCheckRequest {
                    service: "solidity".to_owned(),
                },
            )
            .await
            .unwrap_err();
            match error {
                Error::StatusCode(response) => {
                    assert_eq!(response.status(), reqwest::StatusCode::NOT_FOUND);
                    assert!(response.text().await.unwrap().contains("unknown service"));
                }
                error => panic!("Expected an unknown-service 404, got {error}"),
            }
        }

        if mount.is_some() {
            let wrong_client = Client::new(Config::new(format!("http://{address}"))).await;
            let error = health_client::health(&wrong_client, Default::default())
                .await
                .unwrap_err();
            match error {
                Error::StatusCode(response) => {
                    assert_eq!(response.status(), reqwest::StatusCode::NOT_FOUND);
                }
                error => panic!("Expected an unmounted-route 404, got {error}"),
            }
        }

        handle.stop(true).await;
        task.await.unwrap().unwrap();
    }
}

const COMPILER_VERSION: &str = "v0.8.17+commit.8df45f5f";
const RECORDED_VERSIONS: &str = r#"{"compilerVersions":["v0.8.17+commit.8df45f5f"]}"#;
const RECORDED_VERIFICATION: &str = r#"{"message":"content is not a valid standard json: EOF while parsing an object at line 1 column 1","status":"FAILURE","source":null,"extraData":null,"postActionResponses":null}"#;

#[derive(Default)]
struct RecordedCalls {
    versions: AtomicUsize,
    verification: AtomicUsize,
}

struct RecordedVerifierRouter {
    calls: actix_web::web::Data<RecordedCalls>,
}

impl launcher_router::HttpRouter for RecordedVerifierRouter {
    fn register_routes(&self, config: &mut actix_web::web::ServiceConfig) {
        HealthRouter.register_routes(config);
        config
            .app_data(self.calls.clone())
            .route(
                "/api/v2/verifier/solidity/versions",
                actix_web::web::get().to(recorded_versions),
            )
            .route(
                "/api/v2/verifier/solidity/sources:verify-standard-json",
                actix_web::web::post().to(recorded_verification),
            );
    }
}

async fn recorded_versions(
    request: actix_web::HttpRequest,
    calls: actix_web::web::Data<RecordedCalls>,
) -> actix_web::HttpResponse {
    assert!(request.query_string().is_empty());
    calls.versions.fetch_add(1, Ordering::SeqCst);
    actix_web::HttpResponse::Ok()
        .content_type("application/json")
        .body(RECORDED_VERSIONS)
}

async fn recorded_verification(
    request: actix_web::HttpRequest,
    body: actix_web::web::Json<proto::VerifySolidityStandardJsonRequest>,
    calls: actix_web::web::Data<RecordedCalls>,
) -> actix_web::HttpResponse {
    assert!(request.query_string().is_empty());
    assert_eq!(
        body.into_inner(),
        proto::VerifySolidityStandardJsonRequest {
            bytecode: "00".to_owned(),
            bytecode_type: proto::BytecodeType::DeployedBytecode as i32,
            compiler_version: COMPILER_VERSION.to_owned(),
            input: "{".to_owned(),
            ..Default::default()
        },
    );
    calls.verification.fetch_add(1, Ordering::SeqCst);
    actix_web::HttpResponse::Ok()
        .content_type("application/json")
        .body(RECORDED_VERIFICATION)
}

async fn production_round_trip(mount: Option<&str>) {
    let listener = TcpListener::bind("127.0.0.1:0").unwrap();
    let address = listener.local_addr().unwrap();
    let calls = actix_web::web::Data::new(RecordedCalls::default());
    let server_calls = calls.clone();
    let mount = mount.map(str::to_owned);
    let base_url = format!("http://{address}{}", mount.as_deref().unwrap_or_default());
    let server = actix_web::HttpServer::new(move || {
        let router = RecordedVerifierRouter {
            calls: server_calls.clone(),
        };
        actix_web::App::new().configure(launcher_router::configure_router(&router, mount.clone()))
    })
    .workers(1)
    .disable_signals()
    .listen(listener)
    .unwrap()
    .run();
    let handle = server.handle();
    let task = actix_web::rt::spawn(server);

    for trailing_slash in ["", "/"] {
        let client = Client::new(
            Config::new(format!("{base_url}{trailing_slash}?discard=old#discard")).probe_url(true),
        )
        .await;
        let health = health_client::health(&client, Default::default())
            .await
            .unwrap();
        assert_eq!(
            health.status,
            proto::health_check_response::ServingStatus::Serving as i32
        );
        let versions =
            solidity_verifier_client::list_compiler_versions(&client, Default::default())
                .await
                .unwrap();
        assert!(!versions.compiler_versions.is_empty());
        assert_eq!(
            versions.compiler_versions,
            vec![COMPILER_VERSION.to_owned()]
        );
        let request = proto::VerifySolidityStandardJsonRequest {
            bytecode: "00".to_owned(),
            bytecode_type: proto::BytecodeType::DeployedBytecode as i32,
            compiler_version: versions.compiler_versions[0].clone(),
            input: "{".to_owned(),
            ..Default::default()
        };
        let response = solidity_verifier_client::verify_standard_json(&client, request)
            .await
            .unwrap();
        assert_eq!(
            response.status,
            proto::verify_response::Status::Failure as i32
        );
        assert!(response.message.contains("EOF"), "{}", response.message);
        assert!(response.source.is_none());
        assert_eq!(
            response.message,
            "content is not a valid standard json: EOF while parsing an object at line 1 column 1",
        );
        assert!(response.extra_data.is_none());
        assert!(response.post_action_responses.is_none());
    }

    assert_eq!(calls.versions.load(Ordering::SeqCst), 2);
    assert_eq!(calls.verification.load(Ordering::SeqCst), 2);
    handle.stop(true).await;
    task.await.unwrap().unwrap();
}

#[actix_web::test]
async fn repair_22_31_275_production_root_round_trip() {
    production_round_trip(None).await;
}

#[actix_web::test]
async fn repair_22_31_275_production_verifier_round_trip() {
    production_round_trip(Some("/verifier")).await;
}
