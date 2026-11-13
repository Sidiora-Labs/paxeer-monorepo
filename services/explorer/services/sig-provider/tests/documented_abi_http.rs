use actix_web::{http::StatusCode, test, web, App, HttpRequest, HttpResponse, HttpServer};
use ethabi::{Contract, Token};
use serde_json::{json, Value};
use sig_provider::{precompiles::PrecompileSource, SignatureSource};
use sig_provider_proto::blockscout::sig_provider::v1::{abi_service_actix::route_abi_service, Abi};
use sig_provider_server::{new_service, Service, SourcesSettings};
use std::{net::TcpListener, sync::Arc};

// Recorded directory responses are peers of the real service and adapters.
async fn directory(refuse: bool) -> (SourcesSettings, actix_web::dev::ServerHandle) {
    let listener = TcpListener::bind("127.0.0.1:0").unwrap();
    let endpoint = format!("http://{}", listener.local_addr().unwrap());
    let server = HttpServer::new(move || {
        App::new().default_service(web::to(move |request: HttpRequest| async move {
            if refuse {
                HttpResponse::BadRequest().json(json!({"error": "recorded directory refusal"}))
            } else if request.path() == "/api/v1/signatures" {
                HttpResponse::Ok().json(json!({"result": {"function": {}, "event": {}}}))
            } else {
                HttpResponse::Ok().json(json!({"next": null, "results": []}))
            }
        }))
    })
    .workers(1)
    .listen(listener)
    .unwrap()
    .run();
    let handle = server.handle();
    actix_web::rt::spawn(server);
    let mut settings = SourcesSettings::default();
    settings.fourbyte = endpoint.parse().unwrap();
    settings.sigeth = endpoint.parse().unwrap();
    settings.eth_bytecode_db.enabled = false;
    (settings, handle)
}

async fn request(service: Arc<Service>, uri: &str) -> (StatusCode, Vec<u8>) {
    let app = test::init_service(App::new().configure(move |config| {
        route_abi_service(config, service.clone());
    }))
    .await;
    let response = test::call_service(&app, test::TestRequest::get().uri(uri).to_request()).await;
    let status = response.status();
    (status, test::read_body(response).await.to_vec())
}

fn producer(relative: &str) -> Contract {
    let root = std::path::Path::new(env!("CARGO_MANIFEST_DIR")).join("../../../../..");
    Contract::load(std::fs::File::open(root.join(relative)).unwrap()).unwrap()
}

#[actix_web::test]
async fn documented_precompile_switch_controls_real_http_decoding() {
    let readme = include_str!("../README.md");
    assert!(readme.contains("SIG_PROVIDER__SOURCES__PRECOMPILES__ENABLED=false"));
    let function = producer("precompiles/spot/abi/stoporders.json")
        .function("executeStopLossOrder")
        .unwrap()
        .clone();
    let values = [Token::Uint(42.into())];
    let calldata = function.encode_input(&values).unwrap();
    let uri = format!("/api/v1/abi/function?txInput=0x{}", hex::encode(calldata));
    let (settings, directory) = directory(false).await;
    assert!(settings.precompiles.enabled);
    let (status, body) = request(new_service(settings.clone()), &uri).await;
    assert_eq!(status, StatusCode::OK);
    let decoded: Vec<Abi> = serde_json::from_slice(&body).unwrap();
    assert_eq!(decoded.len(), 1);
    assert_eq!(decoded[0].name, function.name);
    assert_eq!(decoded[0].inputs.len(), 1);
    assert_eq!(decoded[0].inputs[0].value, values[0].to_string());
    let mut disabled = settings;
    disabled.precompiles.enabled = false;
    let (status, body) = request(new_service(disabled), &uri).await;
    assert_eq!(status, StatusCode::OK);
    assert_eq!(serde_json::from_slice::<Vec<Abi>>(&body).unwrap(), vec![]);
    directory.stop(true).await;
}

#[actix_web::test]
async fn documented_unknown_selector_differs_from_unavailable_source() {
    let local = PrecompileSource::new();
    assert!(local
        .get_function_signatures("ffffffff")
        .await
        .unwrap()
        .is_empty());
    for refuse in [false, true] {
        let (settings, directory) = directory(refuse).await;
        let service = new_service(settings);
        let (status, body) =
            request(service.clone(), "/api/v1/abi/function?txInput=0xffffffff").await;
        if refuse {
            assert_eq!(status, StatusCode::INTERNAL_SERVER_ERROR);
            assert!(String::from_utf8(body)
                .unwrap()
                .contains("signature lookup unavailable"));
            // A capable local producer still supplies a usable partial result.
            let function = producer("precompiles/spot/abi/stoporders.json")
                .function("executeStopLossOrder")
                .unwrap()
                .clone();
            let calldata = function.encode_input(&[Token::Uint(7.into())]).unwrap();
            let uri = format!("/api/v1/abi/function?txInput=0x{}", hex::encode(calldata));
            let (status, body) = request(service, &uri).await;
            assert_eq!(status, StatusCode::OK);
            let decoded: Vec<Abi> = serde_json::from_slice(&body).unwrap();
            assert_eq!(decoded.len(), 1);
            assert_eq!(decoded[0].name, function.name);
        } else {
            assert_eq!(status, StatusCode::OK);
            assert_eq!(serde_json::from_slice::<Vec<Abi>>(&body).unwrap(), vec![]);
        }
        directory.stop(true).await;
    }
}

#[actix_web::test]
async fn documented_complete_event_layout_survives_http() {
    let contract = producer("precompiles/launchpad/abi.json");
    let event = contract.event("Swap").unwrap();
    let values = vec![
        Token::FixedBytes(vec![0x11; 32]),
        Token::Address([0x22; 20].into()),
        Token::Bool(true),
        Token::Uint(3.into()),
        Token::Uint(4.into()),
        Token::Uint(5.into()),
        Token::Uint(6.into()),
        Token::Uint(7.into()),
        Token::Uint(8.into()),
    ];
    assert_eq!(event.inputs.len(), values.len());
    let mut topics = vec![format!("0x{}", hex::encode(event.signature()))];
    let mut data = Vec::new();
    for (param, value) in event.inputs.iter().zip(&values) {
        assert!(value.type_check(&param.kind));
        if param.indexed {
            let encoded = ethabi::encode(std::slice::from_ref(value));
            assert_eq!(encoded.len(), 32);
            topics.push(format!("0x{}", hex::encode(encoded)));
        } else {
            data.push(value.clone());
        }
    }
    let uri = format!(
        "/api/v1/abi/event?topics={}&data=0x{}",
        topics.join(","),
        hex::encode(ethabi::encode(&data))
    );
    let (settings, directory) = directory(true).await;
    let (status, body) = request(new_service(settings), &uri).await;
    assert_eq!(status, StatusCode::OK);
    let decoded: Vec<Abi> = serde_json::from_slice(&body).unwrap();
    assert_eq!(decoded.len(), 1);
    assert_eq!(decoded[0].name, event.name);
    assert_eq!(decoded[0].inputs.len(), event.inputs.len());
    for ((actual, expected), value) in decoded[0].inputs.iter().zip(&event.inputs).zip(&values) {
        assert_eq!(actual.name, expected.name);
        assert_eq!(actual.r#type, expected.kind.to_string());
        assert_eq!(actual.indexed, Some(expected.indexed));
        assert_eq!(actual.value, value.to_string());
    }
    let wire: Value = serde_json::from_slice(&body).unwrap();
    assert!(wire.is_array());
    directory.stop(true).await;
}
