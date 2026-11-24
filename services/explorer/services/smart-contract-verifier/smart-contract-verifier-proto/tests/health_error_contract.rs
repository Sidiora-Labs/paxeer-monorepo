use smart_contract_verifier_proto::blockscout::smart_contract_verifier::v2 as proto;
use std::{sync::Arc, time::Duration};

#[path = "../../smart-contract-verifier-server/src/services/health.rs"]
mod health;

const HEALTH_SERVICE: &str = "blockscout.smartContractVerifier.v2.Health";
const UNKNOWN_SERVICES: &[&str] = &[
    "nonexistent.service",
    "blockscout.smartContractVerifier.v2.Health.Check",
    "blockscout.smartContractVerifier.v2.health",
    " ",
];

#[tokio::test]
async fn health_error_contract_grpc() {
    let listener = tokio::net::TcpListener::bind("127.0.0.1:0").await.unwrap();
    let address = listener.local_addr().unwrap();
    let incoming = tonic::codegen::tokio_stream::wrappers::TcpListenerStream::new(listener);
    let (shutdown, shutdown_received) = tokio::sync::oneshot::channel();
    let server = tokio::spawn(async move {
        tonic::transport::Server::builder()
            .add_service(proto::health_server::HealthServer::new(
                health::HealthService::default(),
            ))
            .serve_with_incoming_shutdown(incoming, async {
                shutdown_received.await.unwrap();
            })
            .await
            .unwrap();
    });
    let channel = tonic::transport::Endpoint::from_shared(format!("http://{address}"))
        .unwrap()
        .connect_timeout(Duration::from_secs(5))
        .timeout(Duration::from_secs(5))
        .connect()
        .await
        .unwrap();
    let mut client = proto::health_client::HealthClient::new(channel);

    for service in ["", HEALTH_SERVICE] {
        let response = client
            .check(proto::HealthCheckRequest {
                service: service.to_owned(),
            })
            .await
            .unwrap()
            .into_inner();
        assert_eq!(
            response.status(),
            proto::health_check_response::ServingStatus::Serving
        );
    }
    for service in UNKNOWN_SERVICES {
        let error = client
            .check(proto::HealthCheckRequest {
                service: (*service).to_owned(),
            })
            .await
            .expect_err("unknown services must not be reported healthy");
        assert_eq!(error.code(), tonic::Code::NotFound, "{service:?}");
        assert_eq!(error.message(), "unknown service");
    }
    let response = client
        .check(proto::HealthCheckRequest::default())
        .await
        .unwrap();
    assert_eq!(
        response.into_inner().status(),
        proto::health_check_response::ServingStatus::Serving
    );

    drop(client);
    shutdown.send(()).unwrap();
    tokio::time::timeout(Duration::from_secs(5), server)
        .await
        .unwrap()
        .unwrap();
}

#[actix_web::test]
async fn health_error_contract_http() {
    let listener = std::net::TcpListener::bind("127.0.0.1:0").unwrap();
    let address = listener.local_addr().unwrap();
    let health = Arc::new(health::HealthService::default());
    let server = actix_web::HttpServer::new(move || {
        let health = health.clone();
        actix_web::App::new()
            .configure(move |config| proto::health_actix::route_health(config, health.clone()))
    })
    .workers(1)
    .disable_signals()
    .listen(listener)
    .unwrap()
    .run();
    let handle = server.handle();
    let server = tokio::spawn(server);
    let client = reqwest::Client::builder()
        .no_proxy()
        .timeout(Duration::from_secs(5))
        .build()
        .unwrap();
    let url = format!("http://{address}/health");

    // An omitted query uses the generated request's serde default.
    let response = client.get(&url).send().await.unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::OK);
    let response: proto::HealthCheckResponse = response.json().await.unwrap();
    assert_eq!(
        response.status(),
        proto::health_check_response::ServingStatus::Serving
    );

    for service in ["", HEALTH_SERVICE] {
        let response = client
            .get(&url)
            .query(&[("service", service)])
            .send()
            .await
            .unwrap();
        assert_eq!(response.status(), reqwest::StatusCode::OK, "{service:?}");
        let response: proto::HealthCheckResponse = response.json().await.unwrap();
        assert_eq!(
            response.status(),
            proto::health_check_response::ServingStatus::Serving
        );
    }
    for service in UNKNOWN_SERVICES {
        let response = client
            .get(&url)
            .query(&[("service", service)])
            .send()
            .await
            .unwrap();
        assert_eq!(
            response.status(),
            reqwest::StatusCode::NOT_FOUND,
            "{service:?}"
        );
        let body = response.text().await.unwrap();
        assert!(body.contains("unknown service"), "{body}");
        assert!(!body.contains("SERVING"), "{body}");
    }
    let response = client.get(&url).send().await.unwrap();
    assert_eq!(response.status(), reqwest::StatusCode::OK);
    let response: proto::HealthCheckResponse = response.json().await.unwrap();
    assert_eq!(
        response.status(),
        proto::health_check_response::ServingStatus::Serving
    );

    handle.stop(true).await;
    tokio::time::timeout(Duration::from_secs(5), server)
        .await
        .unwrap()
        .unwrap()
        .unwrap();
}
