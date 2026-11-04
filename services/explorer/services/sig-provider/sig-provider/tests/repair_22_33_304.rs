mod service {
    use sig_provider_proto::tonic;
    include!("../../sig-provider-server/src/service.rs");
}

#[path = "../src/sources/abi_inventory_test_support.rs"]
mod inventory;
// These existing producer-path proofs run as part of this integration target.
#[path = "repair_22_31_267.rs"]
mod producer_path;

use serde_json::{json, Value};
use sig_provider::{fourbyte, sigeth, SignatureSource, SourceAggregator};
use sig_provider_proto::{
    actix_web::{self, web, App, HttpRequest, HttpResponse, HttpServer},
    blockscout::sig_provider::v1::signature_service_actix::route_signature_service,
};
use std::{
    net::TcpListener,
    sync::{Arc, Mutex},
    time::{Duration, Instant},
};

const ABI: &str =
    r#"[{"type":"function","name":"f","inputs":[],"outputs":[],"stateMutability":"nonpayable"}]"#;

struct Reply {
    status: u16,
    body: String,
    header: Option<(&'static str, String)>,
    delay: Duration,
}

fn reply(status: u16, body: Value) -> Reply {
    Reply {
        status,
        body: body.to_string(),
        header: None,
        delay: Duration::ZERO,
    }
}

fn accepted(fourbyte: bool) -> Value {
    if fourbyte {
        json!({"num_processed":25,"num_imported":3,"num_duplicates":18,"num_ignored":4})
    } else {
        json!({"ok":true,"result":{
            "function":{"imported":{},"duplicated":{"f()":"0x26121ff0"},"invalid":null},
            "event":{"imported":{},"duplicated":{},"invalid":null}
        }})
    }
}

type Transcript = Arc<Mutex<Vec<(String, Vec<u8>)>>>;

struct Peer {
    endpoint: String,
    transcript: Transcript,
    server: actix_web::dev::ServerHandle,
}

impl Peer {
    async fn start(handler: impl Fn(usize, &HttpRequest) -> Reply + Send + Sync + 'static) -> Self {
        let transcript: Transcript = Arc::default();
        let records = transcript.clone();
        let handler = Arc::new(handler);
        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        let endpoint = format!("http://{}", listener.local_addr().unwrap());
        let server = HttpServer::new(move || {
            let records = records.clone();
            let handler = handler.clone();
            App::new().default_service(web::to(move |request: HttpRequest, body: web::Bytes| {
                let index = {
                    let mut records = records.lock().unwrap();
                    let index = records.len();
                    records.push((
                        format!("{} {}", request.method(), request.uri()),
                        body.to_vec(),
                    ));
                    index
                };
                let response = handler(index, &request);
                async move {
                    tokio::time::sleep(response.delay).await;
                    let mut builder = HttpResponse::build(
                        actix_web::http::StatusCode::from_u16(response.status).unwrap(),
                    );
                    builder.content_type("application/json");
                    if let Some(header) = response.header {
                        builder.insert_header(header);
                    }
                    builder.body(response.body)
                }
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
            transcript,
            server: handle,
        }
    }

    fn source(&self, fourbyte: bool) -> Arc<dyn SignatureSource + Send + Sync> {
        let host = self.endpoint.parse().unwrap();
        if fourbyte {
            Arc::new(fourbyte::Source::new(host))
        } else {
            Arc::new(sigeth::Source::new(host))
        }
    }

    fn count(&self) -> usize {
        self.transcript.lock().unwrap().len()
    }

    async fn stop(self) {
        self.server.stop(false).await;
    }
}

async fn lookup(peer: &Peer) -> anyhow::Result<Vec<String>> {
    tokio::time::timeout(
        Duration::from_secs(12),
        peer.source(true).get_function_signatures("0x26121ff0"),
    )
    .await
    .expect("production pagination deadline must terminate lookup")
}

#[test]
fn fourbyte_pagination_accepts_relative_and_same_origin_pages() {
    actix_web::rt::System::new().block_on(async {
        let peer = Peer::start(|index, request| {
            assert_eq!(request.path(), "/api/v1/signatures/");
            assert_eq!(request.method(), "GET");
            let next = match index {
                0 => Some("?hex_signature=0x26121ff0&page=2".to_owned()),
                1 => Some(format!(
                    "http://{}/api/v1/signatures/?hex_signature=0x26121ff0&page=3",
                    request.connection_info().host()
                )),
                2 => None,
                _ => panic!("unexpected extra page"),
            };
            reply(
                200,
                json!({"next":next,"results":[{"text_signature":format!("f{index}()") }]}),
            )
        })
        .await;
        assert_eq!(lookup(&peer).await.unwrap(), ["f0()", "f1()", "f2()"]);
        assert_eq!(peer.count(), 3);
        peer.stop().await;
    });
}

#[test]
fn fourbyte_pagination_rejects_cycles_malformed_origins_and_redirects() {
    actix_web::rt::System::new().block_on(async {
        let foreign =
            Peer::start(|_, _| panic!("unexpected-origin URL must never be contacted")).await;
        for (next, count) in [
            ("?hex_signature=0x26121ff0".to_owned(), 1),
            ("?hex_signature=0x26121ff0#ignored".to_owned(), 1),
            ("http://[".to_owned(), 1),
            ("http://user:pass@127.0.0.1/stolen".to_owned(), 1),
            (format!("{}/stolen", foreign.endpoint), 1),
            (
                format!(
                    "//{}/stolen",
                    foreign.endpoint.trim_start_matches("http://")
                ),
                1,
            ),
            ("https://www.4byte.directory/stolen".to_owned(), 1),
        ] {
            let peer = Peer::start(move |_, _| {
                reply(
                    200,
                    json!({"next":next,"results":[{"text_signature":"partial()"}]}),
                )
            })
            .await;
            let error = lookup(&peer).await.unwrap_err().to_string();
            assert!(error.contains("retry"), "missing retry guidance: {error}");
            assert_eq!(peer.count(), count);
            peer.stop().await;
        }
        let peer = Peer::start(|index, _| reply(200, json!({
            "next": if index == 0 { "?page=2" } else { "?hex_signature=0x26121ff0" }, "results":[]
        }))).await;
        assert!(lookup(&peer)
            .await
            .unwrap_err()
            .to_string()
            .contains("cycle"));
        assert_eq!(peer.count(), 2);
        peer.stop().await;
        let endpoint = foreign.endpoint.clone();
        let peer = Peer::start(move |_, _| {
            let mut response = reply(302, json!({}));
            response.header = Some(("location", endpoint.clone()));
            response
        })
        .await;
        assert!(lookup(&peer).await.unwrap_err().to_string().contains("302"));
        assert_eq!(foreign.count(), 0);
        peer.stop().await;
        foreign.stop().await;
    });
}

#[test]
fn fourbyte_pagination_bounds_pages_and_total_deadline() {
    actix_web::rt::System::new().block_on(async {
        let peer = Peer::start(|index, _| {
            reply(
                200,
                json!({"next":format!("?page={}", index+1),"results":[]}),
            )
        })
        .await;
        assert!(lookup(&peer)
            .await
            .unwrap_err()
            .to_string()
            .contains("64 pages"));
        assert_eq!(peer.count(), 64);
        peer.stop().await;
        let peer = Peer::start(|index, _| {
            let mut response = reply(
                200,
                json!({"next":format!("?page={}",index+1),"results":[]}),
            );
            response.delay = Duration::from_millis(250);
            response
        })
        .await;
        let started = Instant::now();
        assert!(lookup(&peer)
            .await
            .unwrap_err()
            .to_string()
            .contains("total deadline"));
        assert!(started.elapsed() < Duration::from_secs(12));
        assert!(peer.count() < 64);
        peer.stop().await;
    });
}

#[test]
fn imports_require_http_success_and_declared_acceptance() {
    actix_web::rt::System::new().block_on(async {
        for fourbyte in [true, false] {
            for status in [400, 401, 403, 404, 429, 500, 502, 503, 504] {
                let peer = Peer::start(move |_, _| reply(status, accepted(fourbyte))).await;
                let error = peer.source(fourbyte).create_signatures(ABI).await.unwrap_err().to_string();
                assert!(error.contains(&status.to_string()) && error.contains("retry"), "{error}");
                assert_eq!(peer.count(), if status >= 500 || status == 429 { 4 } else { 1 });
                peer.stop().await;
            }
            let mut invalid = if fourbyte { vec![json!({"num_processed":5,"num_imported":1,"num_duplicates":1,"num_ignored":1}), json!({"num_processed":-1}), json!({"num_processed":0,"num_imported":u64::MAX,"num_duplicates":1,"num_ignored":0})] }
                else { vec![json!({"ok":false}), json!({"ok":true}), json!({"ok":true,"result":{}})] };
            if !fourbyte {
                let mut rejected = accepted(false);
                rejected["result"]["function"]["invalid"] = json!(["f(bad)"]);
                invalid.push(rejected);
            }
            invalid.extend([json!({}), Value::Null]);
            for body in invalid {
                let peer = Peer::start(move |_, _| reply(200, body.clone())).await;
                assert!(peer.source(fourbyte).create_signatures(ABI).await.is_err());
                assert_eq!(peer.count(), 1, "application rejection must not be retried");
                peer.stop().await;
            }
            let peer = Peer::start(|_, _| { let mut response = reply(200, json!({})); response.body = "not JSON".to_owned(); response }).await;
            assert!(peer.source(fourbyte).create_signatures(ABI).await.is_err());
            assert_eq!(peer.count(), 1);
            peer.stop().await;
            let peer = Peer::start(|_, _| panic!("malformed ABI must not be published")).await;
            for abi in ["not JSON", "{}", r#"[{"type":"function","inputs":[],"name":"f","outputs":[{"type":"bad"}]}]"#] {
                assert!(peer.source(fourbyte).create_signatures(abi).await.unwrap_err().to_string().contains("malformed"));
            }
            assert_eq!(peer.count(), 0);
            peer.stop().await;
        }
    });
}

#[test]
fn imports_retry_transient_failures_with_identical_production_payloads() {
    actix_web::rt::System::new().block_on(async {
        for fourbyte in [true, false] {
            for transient in [429, 502, 503] {
                let peer = Peer::start(move |index, _| {
                    let mut response = if index < 2 {
                        reply(transient, json!({}))
                    } else {
                        reply(if fourbyte { 201 } else { 200 }, accepted(fourbyte))
                    };
                    if index < 2 {
                        response.header = Some(("retry-after", "0".to_owned()));
                    }
                    response
                })
                .await;
                peer.source(fourbyte).create_signatures(ABI).await.unwrap();
                assert_eq!(peer.count(), 3);
                let expected = if fourbyte {
                    json!({"contract_abi":ABI})
                } else {
                    json!({"type":"abi","data":[serde_json::from_str::<Value>(ABI).unwrap()]})
                };
                for (route, body) in peer.transcript.lock().unwrap().iter() {
                    assert_eq!(
                        route,
                        if fourbyte {
                            "POST /api/v1/import-solidity/"
                        } else {
                            "POST /api/v1/import"
                        }
                    );
                    assert_eq!(serde_json::from_slice::<Value>(body).unwrap(), expected);
                }
                peer.stop().await;
            }
            let peer = Peer::start(|_, _| {
                let mut response = reply(429, json!({}));
                response.header = Some(("retry-after", "30".to_owned()));
                response
            })
            .await;
            let started = Instant::now();
            assert!(peer
                .source(fourbyte)
                .create_signatures(ABI)
                .await
                .unwrap_err()
                .to_string()
                .contains("total deadline"));
            assert!(started.elapsed() < Duration::from_secs(12));
            assert_eq!(
                peer.count(),
                1,
                "Retry-After must be respected within the overall deadline"
            );
            peer.stop().await;
        }
    });
}

#[test]
fn service_import_dispatches_real_adapters_to_rejecting_upstreams() {
    actix_web::rt::System::new().block_on(async {
        for fourbyte in [true, false] {
            let peer = Peer::start(|_, _| reply(400, json!({"error":"ABI rejected"}))).await;
            let source = peer.source(fourbyte);
            assert!(source
                .create_signatures(ABI)
                .await
                .unwrap_err()
                .to_string()
                .contains("400"));
            let service = Arc::new(service::Service::new(Arc::new(SourceAggregator::new(
                vec![source],
                vec![],
            ))));
            let listener = TcpListener::bind("127.0.0.1:0").unwrap();
            let endpoint = format!("http://{}", listener.local_addr().unwrap());
            let server = HttpServer::new(move || {
                let service = service.clone();
                App::new().configure(move |config| route_signature_service(config, service.clone()))
            })
            .workers(1)
            .listen(listener)
            .unwrap()
            .run();
            let handle = server.handle();
            actix_web::rt::spawn(server);
            reqwest::Client::new()
                .post(format!("{endpoint}/api/v1/signatures"))
                .json(&json!({"abi":ABI}))
                .send()
                .await
                .unwrap()
                .error_for_status()
                .unwrap();
            tokio::time::timeout(Duration::from_secs(2), async {
                while peer.count() < 2 {
                    tokio::time::sleep(Duration::from_millis(5)).await;
                }
            })
            .await
            .expect("the mounted service must dispatch the import");
            assert_eq!(
                peer.count(),
                2,
                "400 rejection must not be retried by either path"
            );
            for (_, body) in peer.transcript.lock().unwrap().iter() {
                let body: Value = serde_json::from_slice(body).unwrap();
                if fourbyte {
                    assert_eq!(body["contract_abi"], ABI);
                } else {
                    assert_eq!(body["data"][0], serde_json::from_str::<Value>(ABI).unwrap());
                }
            }
            handle.stop(false).await;
            peer.stop().await;
        }
    });
}

#[test]
fn precompile_inventory_exact_selector_signature_coverage() {
    actix_web::rt::System::new().block_on(async {
        let expected = inventory::declared_abi_inventory();
        let abi =
            ethabi::Contract::load(serde_json::to_vec(&expected).unwrap().as_slice()).unwrap();
        let source = sig_provider::precompiles::PrecompileSource::new();
        let mut functions =
            std::collections::BTreeMap::<String, std::collections::BTreeSet<String>>::new();
        let mut events =
            std::collections::BTreeMap::<String, std::collections::BTreeSet<String>>::new();
        for function in abi.functions() {
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
            functions
                .entry(hex::encode(function.short_signature()))
                .or_default()
                .insert(signature);
        }
        for event in abi.events() {
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
            events
                .entry(hex::encode(event.signature()))
                .or_default()
                .insert(signature);
        }
        for (selector, expected) in functions {
            assert_eq!(
                source
                    .get_function_signatures(&selector)
                    .await
                    .unwrap()
                    .into_iter()
                    .collect::<std::collections::BTreeSet<_>>(),
                expected
            );
        }
        for (selector, expected) in events {
            assert_eq!(
                source
                    .get_event_signatures(&selector)
                    .await
                    .unwrap()
                    .into_iter()
                    .collect::<std::collections::BTreeSet<_>>(),
                expected
            );
        }
    });
}

#[test]
fn fourbyte_lookup_retries_only_transient_status_and_preserves_event_path() {
    actix_web::rt::System::new().block_on(async {
        let peer = Peer::start(|index, request| {
            assert_eq!(request.path(), "/api/v1/event-signatures/");
            assert!(request.query_string().contains("hex_signature=0x1234"));
            if index < 2 {
                reply(502, json!({}))
            } else {
                reply(
                    200,
                    json!({"next":null,"results":[{"text_signature":"E(uint256)"}]}),
                )
            }
        })
        .await;
        assert_eq!(
            peer.source(true)
                .get_event_signatures("0x1234")
                .await
                .unwrap(),
            ["E(uint256)"]
        );
        assert_eq!(peer.count(), 3);
        peer.stop().await;
        for (status, body, count) in [
            (400, json!({}), 1),
            (503, json!({}), 4),
            (200, json!({"next":4,"results":[]}), 1),
        ] {
            let peer = Peer::start(move |_, _| reply(status, body.clone())).await;
            assert!(lookup(&peer).await.is_err());
            assert_eq!(peer.count(), count);
            peer.stop().await;
        }
        for fourbyte in [true, false] {
            let source: Arc<dyn SignatureSource + Send + Sync> = if fourbyte {
                Arc::new(fourbyte::Source::new(
                    "mailto:directory@example.com".parse().unwrap(),
                ))
            } else {
                Arc::new(sigeth::Source::new(
                    "mailto:directory@example.com".parse().unwrap(),
                ))
            };
            assert!(source
                .create_signatures(ABI)
                .await
                .unwrap_err()
                .to_string()
                .contains("URL"));
        }
    });
}
