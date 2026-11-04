// Exercise the production server implementation with its transport crate types.
mod service {
    use sig_provider_proto::tonic;
    include!("../../sig-provider-server/src/service.rs");
}

use ethabi::{ethereum_types::H256, Contract, Event, ParamType, RawLog, Token};
use serde_json::Value;
use sig_provider::{precompiles::PrecompileSource, SignatureSource, SourceAggregator};
use sig_provider_proto::actix_web::{self, App, HttpServer};
use sig_provider_proto::blockscout::sig_provider::v1::{
    abi_service_actix::route_abi_service, Abi, GetEventAbiRequest, GetFunctionAbiRequest,
};
use std::{
    collections::{BTreeMap, BTreeSet},
    fs,
    net::TcpListener,
    path::{Path, PathBuf},
    sync::Arc,
};

fn root() -> PathBuf {
    Path::new(env!("CARGO_MANIFEST_DIR")).join("../../../../..")
}

fn producer_abi(path: &str) -> Contract {
    Contract::load(fs::File::open(root().join(path)).unwrap()).unwrap()
}

fn inline_producer_abi(path: &str, symbol: &str) -> Contract {
    let code = fs::read_to_string(root().join(path)).unwrap();
    let declaration = code.split_once(&format!("{symbol} ")).unwrap().1;
    let raw = declaration
        .split_once('`')
        .unwrap()
        .1
        .split_once('`')
        .unwrap()
        .0;
    Contract::load(raw.as_bytes()).unwrap()
}

fn signature(name: &str, inputs: impl Iterator<Item = String>) -> String {
    format!("{name}({})", inputs.collect::<Vec<_>>().join(","))
}

fn canonical_abi_entry(item: Value) -> String {
    fn ordered(value: Value) -> Value {
        match value {
            Value::Object(values) => Value::Object(
                values
                    .into_iter()
                    .map(|(key, value)| (key, ordered(value)))
                    .collect::<BTreeMap<_, _>>()
                    .into_iter()
                    .collect(),
            ),
            Value::Array(values) => Value::Array(values.into_iter().map(ordered).collect()),
            value => value,
        }
    }
    serde_json::to_string(&ordered(item)).unwrap()
}

#[tokio::test]
async fn precompile_table_coverage_matches_versioned_node_inventory() {
    let embedded: Value =
        serde_json::from_str(include_str!("../src/sources/precompiles.json")).unwrap();
    let inventory = embedded["inventory"].as_array().unwrap();
    let setup = fs::read_to_string(root().join("precompiles/setup.go")).unwrap();
    let registry = setup
        .split_once("var precompileInfoSources =")
        .unwrap()
        .1
        .split_once("\n}")
        .unwrap()
        .0;
    for row in registry
        .lines()
        .filter(|line| line.trim_start().starts_with('"'))
    {
        let fields: Vec<_> = row.split('"').collect();
        let origin = format!("precompiles/{}", fields[3]);
        assert!(
            inventory
                .iter()
                .any(|entry| entry["name"] == fields[1] && entry["origin"] == origin),
            "missing versioned producer {row}"
        );
    }
    let merge = setup
        .split_once("var MergeAddressTable =")
        .unwrap()
        .1
        .split_once("\n}")
        .unwrap()
        .0;
    for row in merge.lines().filter(|line| line.contains("HexToAddress")) {
        let name = row.split('"').nth(3).unwrap();
        assert!(
            inventory
                .iter()
                .any(|entry| entry["name"] == name && entry["status"] == "merge"),
            "missing merged producer {name}"
        );
    }
    let mut expected = BTreeSet::new();
    for entry in inventory {
        let origin = entry["origin"].as_str().unwrap();
        let raw = if let Some((path, symbol)) = origin.split_once('#') {
            let code = fs::read_to_string(root().join(path)).unwrap();
            code.split_once(&format!("{symbol} "))
                .unwrap()
                .1
                .split_once('`')
                .unwrap()
                .1
                .split_once('`')
                .unwrap()
                .0
                .to_owned()
        } else {
            let path = if origin == "precompiles_historical.json" {
                Path::new(env!("CARGO_MANIFEST_DIR"))
                    .join("src/sources")
                    .join(origin)
            } else {
                root().join(origin)
            };
            fs::read_to_string(path).unwrap()
        };
        let abi: Vec<Value> = serde_json::from_str(&raw).unwrap();
        expected.extend(
            abi.into_iter()
                .filter(|item| item["type"] == "function" || item["type"] == "event")
                .map(canonical_abi_entry),
        );
    }
    let actual: BTreeSet<_> = embedded["abi"]
        .as_array()
        .unwrap()
        .iter()
        .cloned()
        .map(canonical_abi_entry)
        .collect();
    assert_eq!(
        actual, expected,
        "the embedded ABI must exactly equal its real producer and historical ABI union"
    );
    for name in [
        "perps",
        "pricefeed",
        "spot_pecor",
        "spot_orders",
        "spot_stoporders",
        "spot_quoter",
        "spot_vault",
        "layerxledger",
        "layerxpay",
        "layerxauth",
        "layerxprograms",
        "layerxchains",
        "orob",
        "clearing",
        "oracle aggregator",
        "pofq",
        "scheduler",
        "streams",
        "tee attestor",
        "eip712",
    ] {
        assert!(
            embedded["inventory"]
                .as_array()
                .unwrap()
                .iter()
                .any(|source| source["name"] == name),
            "missing active producer {name}"
        );
    }
    let source = PrecompileSource::new();
    let abi = Contract::load(serde_json::to_vec(&embedded["abi"]).unwrap().as_slice()).unwrap();
    let historical =
        Contract::load(include_str!("../src/sources/precompiles_historical.json").as_bytes())
            .unwrap();
    for abi in [&abi, &historical] {
        for function in abi.functions() {
            let expected = signature(
                &function.name,
                function.inputs.iter().map(|input| input.kind.to_string()),
            );
            assert!(
                source
                    .get_function_signatures(&hex::encode(function.short_signature()))
                    .await
                    .unwrap()
                    .contains(&expected),
                "missing {expected}"
            );
        }
        for event in abi.events() {
            let descriptions = sig_provider::CompleteSignatureSource::get_event_signatures(
                &source,
                &hex::encode(event.signature()),
            )
            .await
            .unwrap();
            assert!(
                descriptions.iter().any(|description| {
                    let decoded: Event =
                        serde_json::from_value(serde_json::to_value(description).unwrap()).unwrap();
                    decoded == *event
                }),
                "complete event metadata must preserve every producer layout: {event:?}"
            );
            let expected = signature(
                &event.name,
                event.inputs.iter().map(|input| input.kind.to_string()),
            );
            assert!(
                source
                    .get_event_signatures(&hex::encode(event.signature()))
                    .await
                    .unwrap()
                    .contains(&expected),
                "missing {expected}"
            );
        }
    }
}

fn sample(kind: &ParamType) -> Token {
    match kind {
        ParamType::Address => Token::Address([0x24; 20].into()),
        ParamType::Uint(_) => Token::Uint(42.into()),
        ParamType::Int(_) => Token::Int(17.into()),
        ParamType::Bool => Token::Bool(true),
        ParamType::FixedBytes(length) => Token::FixedBytes(vec![0xab; *length]),
        ParamType::Bytes => Token::Bytes(vec![0x12, 0x34, 0xab]),
        ParamType::String => Token::String("native ABI".to_owned()),
        ParamType::Array(item) => Token::Array(vec![sample(item)]),
        ParamType::FixedArray(item, length) => Token::FixedArray(vec![sample(item); *length]),
        ParamType::Tuple(items) => Token::Tuple(items.iter().map(sample).collect()),
    }
}

async fn start_server() -> (String, actix_web::dev::ServerHandle) {
    let service = Arc::new(service::Service::new(Arc::new(SourceAggregator::new(
        vec![Arc::new(PrecompileSource::new())],
        vec![Arc::new(PrecompileSource::new())],
    ))));
    let listener = TcpListener::bind("127.0.0.1:0").unwrap();
    let endpoint = format!("http://{}", listener.local_addr().unwrap());
    let server = HttpServer::new(move || {
        let service = service.clone();
        App::new().configure(move |config| route_abi_service(config, service.clone()))
    })
    .workers(1)
    .listen(listener)
    .unwrap()
    .run();
    let handle = server.handle();
    actix_web::rt::spawn(server);
    (endpoint, handle)
}

async fn assert_function(client: &reqwest::Client, endpoint: &str, abi: &Contract, name: &str) {
    let function = abi.function(name).unwrap();
    let tokens: Vec<_> = function
        .inputs
        .iter()
        .map(|input| sample(&input.kind))
        .collect();
    let calldata = function.encode_input(&tokens).unwrap();
    assert_eq!(function.decode_input(&calldata[4..]).unwrap(), tokens);
    let decoded: Vec<Abi> = client
        .get(format!("{endpoint}/api/v1/abi/function"))
        .query(&GetFunctionAbiRequest {
            tx_input: format!("0x{}", hex::encode(calldata)),
        })
        .send()
        .await
        .unwrap()
        .error_for_status()
        .unwrap()
        .json()
        .await
        .unwrap();
    assert!(
        decoded.iter().any(|candidate| {
            candidate.name == function.name
                && candidate.inputs.len() == tokens.len()
                && candidate
                    .inputs
                    .iter()
                    .zip(&function.inputs)
                    .zip(&tokens)
                    .all(|((actual, param), token)| {
                        actual.r#type == param.kind.to_string() && actual.value == token.to_string()
                    })
        }),
        "HTTP consumer failed to decode {}: {decoded:?}",
        function.signature()
    );
}

#[test]
fn precompile_table_coverage_current_and_historical_calldata_over_http() {
    actix_web::rt::System::new().block_on(async {
        let (endpoint, server) = start_server().await;
        let client = reqwest::Client::new();
        for (path, name) in [
            ("precompiles/perps/abi/abi.json", "cancelOrder"),
            ("precompiles/layerxexchange/abi.json", "cancelOrder"),
            ("precompiles/launchpad/legacy/v610/abi.json", "getConfig"),
            ("precompiles/layerxledger/abi.json", "balanceOf"),
            ("precompiles/layerxpay/abi.json", "budgetAmend"),
            ("precompiles/layerxauth/abi.json", "grantSession"),
            ("precompiles/layerxprograms/abi.json", "getProgram"),
            ("precompiles/layerxchains/abi.json", "chainCount"),
            ("precompiles/pricefeed/abi/abi.json", "getPrices"),
        ] {
            assert_function(&client, &endpoint, &producer_abi(path), name).await;
        }
        for role in ["pecor", "orders", "stoporders", "quoter", "vault"] {
            let abi = producer_abi(&format!("precompiles/spot/abi/{role}.json"));
            let function = abi.functions().next().unwrap();
            assert_function(&client, &endpoint, &abi, &function.name).await;
        }
        for (package, name) in [
            ("orob", "resolveOffsetBatch"),
            ("clearing", "computeClearing"),
            ("oracle", "aggregate"),
            ("pofq", "scoreBatch"),
            ("scheduler", "schedule"),
            ("streams", "updateRate"),
            ("teeattestor", "verify"),
            ("eip712", "domainSeparator"),
        ] {
            let abi = inline_producer_abi(
                &format!("precompiles/layerxagent/{package}/{package}.go"),
                "abiJSON",
            );
            assert_function(&client, &endpoint, &abi, name).await;
        }
        let unknown: Vec<Abi> = client
            .get(format!("{endpoint}/api/v1/abi/function"))
            .query(&GetFunctionAbiRequest {
                tx_input: "0xffffffff".to_owned(),
            })
            .send()
            .await
            .unwrap()
            .error_for_status()
            .unwrap()
            .json()
            .await
            .unwrap();
        assert!(unknown.is_empty());
        server.stop(true).await;
    });
}

fn event_request(event: &Event) -> (GetEventAbiRequest, Vec<Token>) {
    let values: Vec<_> = event
        .inputs
        .iter()
        .map(|input| sample(&input.kind))
        .collect();
    let mut topics = vec![event.signature()];
    let mut data = Vec::new();
    for (param, token) in event.inputs.iter().zip(&values) {
        if param.indexed {
            // These producer events index only static, single-word types.
            let encoded = ethabi::encode(std::slice::from_ref(token));
            assert_eq!(encoded.len(), 32);
            topics.push(H256::from_slice(&encoded));
        } else {
            data.push(token.clone());
        }
    }
    let data = ethabi::encode(&data);
    let parsed = event
        .parse_log(RawLog {
            topics: topics.clone(),
            data: data.clone(),
        })
        .unwrap();
    assert_eq!(
        parsed
            .params
            .iter()
            .map(|param| param.value.clone())
            .collect::<Vec<_>>(),
        values
    );
    (
        GetEventAbiRequest {
            data: format!("0x{}", hex::encode(data)),
            topics: topics
                .iter()
                .map(|topic| format!("0x{}", hex::encode(topic)))
                .collect::<Vec<_>>()
                .join(","),
        },
        values,
    )
}

fn assert_event(decoded: &[Abi], event: &Event, values: &[Token]) {
    assert!(
        decoded.iter().any(|candidate| {
            candidate.name == event.name
                && candidate.inputs.len() == values.len()
                && candidate.inputs.iter().zip(&event.inputs).zip(values).all(
                    |((actual, param), token)| {
                        actual.r#type == param.kind.to_string()
                            && actual.value == token.to_string()
                            && actual.indexed == Some(param.indexed)
                    },
                )
        }),
        "HTTP consumer failed to decode event {}: {decoded:?}",
        event.name
    );
    assert_eq!(
        decoded.len(),
        1,
        "exact producer metadata must not expose guessed layouts: {decoded:?}"
    );
    assert!(decoded[0]
        .inputs
        .iter()
        .zip(&event.inputs)
        .all(|(actual, param)| actual.name == param.name));
}

#[test]
fn precompile_table_coverage_current_events_and_batch_over_http() {
    actix_web::rt::System::new().block_on(async {
        let (endpoint, server) = start_server().await;
        let client = reqwest::Client::new();
        let perps = producer_abi("precompiles/perps/abi/abi.json");
        let ledger = producer_abi("precompiles/layerxledger/abi.json");
        let chains = producer_abi("precompiles/layerxchains/abi.json");
        let scheduler = inline_producer_abi(
            "precompiles/layerxagent/scheduler/merge.go",
            "jobOutcomeABI",
        );
        let events = [
            perps.event("OrderCancelled").unwrap(),
            ledger.event("Transfer").unwrap(),
            chains.event("ChainCreated").unwrap(),
            scheduler.event("JobFailed").unwrap(),
        ];
        let mut requests = Vec::new();
        let mut expected = Vec::new();
        for event in events {
            let (request, values) = event_request(event);
            let decoded: Vec<Abi> = client
                .get(format!("{endpoint}/api/v1/abi/event"))
                .query(&request)
                .send()
                .await
                .unwrap()
                .error_for_status()
                .unwrap()
                .json()
                .await
                .unwrap();
            assert_event(&decoded, event, &values);
            let malformed = GetEventAbiRequest {
                data: format!("{}{}", request.data, "00".repeat(32)),
                topics: request.topics.clone(),
            };
            let decoded: Vec<Abi> = client
                .get(format!("{endpoint}/api/v1/abi/event"))
                .query(&malformed)
                .send()
                .await
                .unwrap()
                .error_for_status()
                .unwrap()
                .json()
                .await
                .unwrap();
            assert!(
                decoded.is_empty(),
                "trailing event data must be rejected: {decoded:?}"
            );
            requests.push(request);
            expected.push((event, values));
        }
        let batch: Value = client
            .post(format!("{endpoint}/api/v1/abi/events:batch-get"))
            .json(&serde_json::json!({"requests": requests}))
            .send()
            .await
            .unwrap()
            .error_for_status()
            .unwrap()
            .json()
            .await
            .unwrap();
        let responses = batch.as_array().unwrap();
        assert_eq!(responses.len(), expected.len());
        for (response, (event, values)) in responses.iter().zip(expected) {
            let decoded: Vec<Abi> = serde_json::from_value(response["abi"].clone()).unwrap();
            assert_event(&decoded, event, &values);
        }
        let malformed_requests: Vec<_> = requests
            .iter()
            .map(|request| GetEventAbiRequest {
                data: request.data.clone(),
                topics: request.topics.rsplit_once(',').unwrap().0.to_owned(),
            })
            .collect();
        let batch: Value = client
            .post(format!("{endpoint}/api/v1/abi/events:batch-get"))
            .json(&serde_json::json!({"requests": malformed_requests}))
            .send()
            .await
            .unwrap()
            .error_for_status()
            .unwrap()
            .json()
            .await
            .unwrap();
        let responses = batch.as_array().unwrap();
        assert_eq!(responses.len(), requests.len());
        for response in responses {
            let decoded: Vec<Abi> = serde_json::from_value(response["abi"].clone()).unwrap();
            assert!(
                decoded.is_empty(),
                "missing indexed topic must be rejected: {decoded:?}"
            );
        }
        server.stop(true).await;
    });
}
