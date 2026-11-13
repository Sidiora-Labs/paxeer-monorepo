// Shared by the crate's unit suite and repair_22_33_305 integration gate.
// Mount the actual server implementation and generated HTTP routes.
pub mod service {
    use sig_provider_proto::tonic;
    include!("../../sig-provider-server/src/service.rs");
}

use ethabi::{ethereum_types::H256, Contract, Event, RawLog, Token};
use serde_json::{json, Value};
use sig_provider::{fourbyte, precompiles::PrecompileSource, SignatureSource, SourceAggregator};
use sig_provider_proto::{
    actix_web::{self, web, App, HttpRequest, HttpResponse, HttpServer},
    blockscout::sig_provider::v1::{
        abi_service_actix::route_abi_service, signature_service_actix::route_signature_service,
        Abi, BatchGetEventAbisRequest, GetEventAbiRequest, GetFunctionAbiRequest,
    },
};
use std::{
    net::TcpListener,
    path::Path,
    sync::{Arc, Mutex},
};

pub struct Peer {
    pub endpoint: String,
    pub server: actix_web::dev::ServerHandle,
    pub requests: Arc<Mutex<Vec<(String, Vec<u8>)>>>,
}

impl Peer {
    /// A recorded upstream HTTP transcript consumed by the production adapters.
    pub async fn reply(status: u16, body: Value) -> Self {
        let requests = Arc::new(Mutex::new(Vec::new()));
        let records = requests.clone();
        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        let endpoint = format!("http://{}", listener.local_addr().unwrap());
        let server = HttpServer::new(move || {
            let body = body.clone();
            let records = records.clone();
            App::new().default_service(web::to(move |request: HttpRequest, bytes: web::Bytes| {
                records.lock().unwrap().push((
                    format!("{} {}", request.method(), request.uri()),
                    bytes.to_vec(),
                ));
                let response =
                    HttpResponse::build(actix_web::http::StatusCode::from_u16(status).unwrap())
                        .json(body.clone());
                async { response }
            }))
        })
        .workers(1)
        .listen(listener)
        .unwrap()
        .run();
        let handle = server.handle();
        actix_web::rt::spawn(server);
        Self {
            endpoint,
            server: handle,
            requests,
        }
    }
}

pub async fn start_service(
    aggregator: Arc<SourceAggregator>,
    imports: Vec<Arc<dyn SignatureSource + Send + Sync>>,
) -> (Peer, Arc<service::Service>) {
    let service = Arc::new(service::Service::with_import_sources(aggregator, imports));
    let mounted = service.clone();
    let listener = TcpListener::bind("127.0.0.1:0").unwrap();
    let endpoint = format!("http://{}", listener.local_addr().unwrap());
    let server = HttpServer::new(move || {
        let service = mounted.clone();
        App::new().configure(move |config| {
            route_abi_service(config, service.clone());
            route_signature_service(config, service.clone());
        })
    })
    .workers(1)
    .listen(listener)
    .unwrap()
    .run();
    let handle = server.handle();
    actix_web::rt::spawn(server);
    (
        Peer {
            endpoint,
            server: handle,
            requests: Arc::default(),
        },
        service,
    )
}

pub fn producer(path: &str) -> Contract {
    let root = Path::new(env!("CARGO_MANIFEST_DIR")).join("../../../../..");
    Contract::load(std::fs::File::open(root.join(path)).unwrap()).unwrap()
}

pub fn precompile_aggregator(
    extra: Vec<Arc<dyn SignatureSource + Send + Sync>>,
) -> Arc<SourceAggregator> {
    let local = Arc::new(PrecompileSource::new());
    let mut sources: Vec<Arc<dyn SignatureSource + Send + Sync>> = vec![local.clone()];
    sources.extend(extra);
    Arc::new(SourceAggregator::new(sources, vec![local]))
}

pub fn encoded_event(event: &Event, values: &[Token]) -> RawLog {
    assert_eq!(event.inputs.len(), values.len());
    let mut topics = vec![event.signature()];
    let mut data = Vec::new();
    for (param, token) in event.inputs.iter().zip(values) {
        assert!(token.type_check(&param.kind));
        if param.indexed {
            let encoded = ethabi::encode(std::slice::from_ref(token));
            assert_eq!(
                encoded.len(),
                32,
                "fixture indexes static, single-word values"
            );
            topics.push(H256::from_slice(&encoded));
        } else {
            data.push(token.clone());
        }
    }
    let raw = RawLog {
        topics,
        data: ethabi::encode(&data),
    };
    let decoded = event.parse_log(raw.clone()).unwrap();
    assert_eq!(
        decoded
            .params
            .iter()
            .map(|param| param.value.clone())
            .collect::<Vec<_>>(),
        values
    );
    raw
}

pub fn event_request(raw: &RawLog) -> GetEventAbiRequest {
    GetEventAbiRequest {
        data: format!("0x{}", hex::encode(&raw.data)),
        topics: raw
            .topics
            .iter()
            .map(|topic| format!("0x{}", hex::encode(topic)))
            .collect::<Vec<_>>()
            .join(","),
    }
}

pub fn assert_producer_event(abis: &[Abi], event: &Event, values: &[Token]) {
    assert_eq!(
        abis.len(),
        1,
        "producer metadata must exclude guessed layouts"
    );
    assert_eq!(abis[0].name, event.name);
    assert_eq!(abis[0].inputs.len(), event.inputs.len());
    for ((actual, expected), token) in abis[0].inputs.iter().zip(&event.inputs).zip(values) {
        assert_eq!(actual.name, expected.name);
        assert_eq!(actual.r#type, expected.kind.to_string());
        assert_eq!(actual.indexed, Some(expected.indexed));
        assert_eq!(actual.value, token.to_string());
    }
}

#[test]
fn precompile_signature_ranks_first() {
    actix_web::rt::System::new().block_on(async {
        let producer = producer("precompiles/spot/abi/stoporders.json");
        let local = producer.function("executeStopLossOrder").unwrap();
        // Committed ABI fixture, indexed by its actual canonical Keccak selector.
        let collision = Contract::load(br#"[{"type":"function","name":"collisionFixture762890","inputs":[{"name":"orderId","type":"uint256"}],"outputs":[],"stateMutability":"nonpayable"}]"#.as_slice()).unwrap();
        let collision = collision.function("collisionFixture762890").unwrap();
        assert_ne!(local.name, collision.name);
        assert_eq!(local.short_signature(), collision.short_signature());
        assert_eq!(hex::encode(local.short_signature()), "1a617148");
        let values = [Token::Uint(42.into())];
        let calldata = local.encode_input(&values).unwrap();
        assert_eq!(collision.encode_input(&values).unwrap(), calldata);
        assert_eq!(local.decode_input(&calldata[4..]).unwrap(), values);
        assert_eq!(collision.decode_input(&calldata[4..]).unwrap(), values);
        let local_signature = "executeStopLossOrder(uint256)";
        let collision_signature = "collisionFixture762890(uint256)";
        let remote = Peer::reply(200, json!({"next":null,"results":[
            {"text_signature":collision_signature}, {"text_signature":local_signature}
        ]})).await;
        let aggregator = precompile_aggregator(vec![Arc::new(fourbyte::Source::new(remote.endpoint.parse().unwrap()))]);
        let signatures = aggregator.get_function_signatures(&hex::encode(local.short_signature())).await.unwrap();
        assert_eq!(signatures, vec![local_signature, collision_signature]);
        let decoded = aggregator.get_function_abi(&calldata).await.unwrap();
        let (http, _) = start_service(aggregator, vec![]).await;
        let response: Vec<Abi> = reqwest::Client::new().get(format!("{}/api/v1/abi/function", http.endpoint))
            .query(&GetFunctionAbiRequest { tx_input: format!("0x{}", hex::encode(&calldata)) })
            .send().await.unwrap().error_for_status().unwrap().json().await.unwrap();
        assert_eq!(response, decoded);
        assert_eq!(response.len(), 2);
        assert_eq!(response[0].name, local.name);
        assert_eq!(response[1].name, collision.name);
        for candidate in response {
            assert_eq!(candidate.inputs.len(), 1);
            assert_eq!(candidate.inputs[0].r#type, "uint256");
            assert_eq!(candidate.inputs[0].value, values[0].to_string());
        }
        let requests = remote.requests.lock().unwrap().clone();
        assert_eq!(requests.len(), 3);
        assert!(requests.iter().all(|(path, _)| path == "GET /api/v1/signatures/?hex_signature=1a617148"));
        http.server.stop(true).await;
        remote.server.stop(true).await;
    });
}

#[test]
fn producer_backed_swap_preserves_layout_over_http() {
    actix_web::rt::System::new().block_on(async {
        let producer = producer("precompiles/launchpad/abi.json");
        let event = producer.event("Swap").unwrap();
        let values = vec![
            Token::FixedBytes(vec![0x11; 32]),
            Token::Address([0x22; 20].into()),
            Token::Bool(true),
            Token::Uint(0x33.into()),
            Token::Uint(0x44.into()),
            Token::Uint(0x55.into()),
            Token::Uint(0x66.into()),
            Token::Uint(0x77.into()),
            Token::Uint(0x88.into()),
        ];
        let raw = encoded_event(event, &values);
        let aggregator = precompile_aggregator(vec![]);
        let report = aggregator
            .get_event_abi_with_coverage(raw.clone())
            .await
            .unwrap();
        assert_eq!(report.value.len(), 1);
        assert_eq!(report.value[0].layout, sig_provider::EventLayout::Exact);
        assert_producer_event(&[report.value[0].abi.clone()], event, &values);
        let (http, _) = start_service(aggregator, vec![]).await;
        let client = reqwest::Client::new();
        let request = event_request(&raw);
        let decoded: Vec<Abi> = client
            .get(format!("{}/api/v1/abi/event", http.endpoint))
            .query(&request)
            .send()
            .await
            .unwrap()
            .error_for_status()
            .unwrap()
            .json()
            .await
            .unwrap();
        assert_producer_event(&decoded, event, &values);
        let batch: Vec<sig_provider_proto::blockscout::sig_provider::v1::GetEventAbiResponse> =
            client
                .post(format!("{}/api/v1/abi/events:batch-get", http.endpoint))
                .json(&BatchGetEventAbisRequest {
                    requests: vec![request],
                })
                .send()
                .await
                .unwrap()
                .error_for_status()
                .unwrap()
                .json()
                .await
                .unwrap();
        assert_eq!(batch.len(), 1);
        assert_eq!(batch[0].abi, decoded);
        http.server.stop(true).await;
    });
}

#[test]
fn producer_backed_chain_launched_preserves_layout() {
    actix_web::rt::System::new().block_on(async {
        let producer = producer("precompiles/launchpad/abi.json");
        let event = producer.event("ChainLaunched").unwrap();
        let values = vec![
            Token::Uint(37.into()),
            Token::Address([0x11; 20].into()),
            Token::Address([0x22; 20].into()),
            Token::Uint(125.into()),
            Token::Uint(42.into()),
            Token::Address([0x33; 20].into()),
            Token::Uint(1_700_000_000_u64.into()),
            Token::Uint(99.into()),
        ];
        let raw = encoded_event(event, &values);
        let report = precompile_aggregator(vec![])
            .get_event_abi_with_coverage(raw)
            .await
            .unwrap();
        assert_eq!(report.value.len(), 1);
        assert_eq!(report.value[0].layout, sig_provider::EventLayout::Exact);
        assert_producer_event(&[report.value[0].abi.clone()], event, &values);
    });
}
