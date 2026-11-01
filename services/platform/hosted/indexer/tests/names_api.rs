use std::io::{Read, Write};
use std::net::{TcpListener, TcpStream};
use std::path::PathBuf;
use std::sync::atomic::{AtomicU64, Ordering};
use std::sync::Arc;
use std::thread;
use std::time::Duration;

use layerx_indexer::abi::AbiRegistry;
use layerx_indexer::api;
use layerx_indexer::follow::{FollowPolicy, StepOutcome};
use layerx_indexer::names::{names_abi, NamesConfig};
use layerx_indexer::paxeer::{AttributeEncoding, PaxeerIngester};
use layerx_indexer::store::Store;
use layerx_indexer::transport::{Endpoint, Security};
use serde_json::{json, Value};

type Checked<T> = Result<T, Box<dyn std::error::Error>>;

fn fixture() -> Value {
    serde_json::from_str(include_str!("../fixtures/names_blocks.json"))
        .unwrap_or_else(|error| panic!("names fixture: {error}"))
}

fn config() -> NamesConfig {
    NamesConfig::from_lookup(&|key| {
        let role = key
            .strip_prefix("LAYERX_INDEXER_NAMES_")?
            .to_ascii_lowercase();
        fixture()["addresses"][format!("names_{role}")]
            .as_str()
            .map(str::to_owned)
    })
    .unwrap_or_else(|error| panic!("{error}"))
    .unwrap_or_else(|| panic!("fixture config missing"))
}

fn registry() -> AbiRegistry {
    let mut registry = AbiRegistry::default();
    for abi in names_abi(&config()).unwrap_or_else(|error| panic!("{error}")) {
        registry.insert(abi);
    }
    registry
}

fn request(stream: &mut TcpStream) -> Option<Value> {
    let mut bytes = Vec::new();
    let mut chunk = [0; 4096];
    let header_end = loop {
        let count = stream.read(&mut chunk).ok()?;
        if count == 0 {
            return None;
        }
        bytes.extend_from_slice(&chunk[..count]);
        if let Some(index) = bytes.windows(4).position(|bytes| bytes == b"\r\n\r\n") {
            break index + 4;
        }
    };
    let headers = std::str::from_utf8(&bytes[..header_end]).ok()?;
    let length = headers
        .lines()
        .filter_map(|line| line.split_once(':'))
        .find(|(key, _)| key.eq_ignore_ascii_case("content-length"))?
        .1
        .trim()
        .parse::<usize>()
        .ok()?;
    while bytes.len() < header_end + length {
        let count = stream.read(&mut chunk).ok()?;
        if count == 0 {
            return None;
        }
        bytes.extend_from_slice(&chunk[..count]);
    }
    serde_json::from_slice(&bytes[header_end..header_end + length]).ok()
}

fn node(head: Arc<AtomicU64>) -> Checked<PaxeerIngester> {
    let listener = TcpListener::bind("127.0.0.1:0")?;
    let url = format!("http://{}", listener.local_addr()?);
    let chain = fixture();
    thread::spawn(move || {
        for incoming in listener.incoming() {
            let Ok(mut stream) = incoming else {
                continue;
            };
            let Some(request) = request(&mut stream) else {
                continue;
            };
            let result = match request["method"].as_str() {
                Some("eth_chainId") => chain["chain_id"].clone(),
                Some("eth_blockNumber") => json!(format!("0x{:x}", head.load(Ordering::SeqCst))),
                Some("eth_getBlockByNumber") => {
                    let height = u64::from_str_radix(
                        request["params"][0]
                            .as_str()
                            .unwrap_or_else(|| panic!("height missing"))
                            .trim_start_matches("0x"),
                        16,
                    )
                    .unwrap_or_else(|error| panic!("{error}"));
                    if height > head.load(Ordering::SeqCst) {
                        Value::Null
                    } else {
                        chain["blocks"][height.to_string()].clone()
                    }
                }
                Some("eth_getTransactionReceipt") => chain["receipts"][request["params"][0]
                    .as_str()
                    .unwrap_or_else(|| panic!("hash missing"))]
                .clone(),
                other => panic!("unexpected method {other:?}"),
            };
            let body = json!({"jsonrpc":"2.0", "id":request["id"], "result":result}).to_string();
            write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}", body.len())
                .unwrap_or_else(|error| panic!("response: {error}"));
        }
    });
    Ok(PaxeerIngester::new(
        Endpoint::parse(
            &url,
            Security::Plaintext {
                allow_remote: false,
            },
            Duration::from_secs(5),
        )?,
        None,
        registry(),
        FollowPolicy {
            finality_depth: 10,
            max_units_per_step: 3,
        },
        1,
        Some(125),
        AttributeEncoding::Plain,
    ))
}

static NEXT_DATABASE: AtomicU64 = AtomicU64::new(0);
struct Database(PathBuf);
impl Database {
    fn new() -> Self {
        Self(PathBuf::from(env!("CARGO_TARGET_TMPDIR")).join(format!(
            "names-api-{}-{}.sqlite",
            std::process::id(),
            NEXT_DATABASE.fetch_add(1, Ordering::Relaxed)
        )))
    }
}
impl Drop for Database {
    fn drop(&mut self) {
        for suffix in ["", "-wal", "-shm"] {
            let _ = std::fs::remove_file(format!("{}{suffix}", self.0.display()));
        }
    }
}

fn api_endpoint(store: Store) -> Checked<Endpoint> {
    let listener = api::bind("127.0.0.1:0".parse()?, false)?;
    let endpoint = Endpoint::parse(
        &format!("http://{}", listener.local_addr()?),
        Security::Plaintext {
            allow_remote: false,
        },
        Duration::from_secs(5),
    )?;
    let store = Arc::new(store);
    thread::spawn(move || api::serve(&listener, &store, None));
    Ok(endpoint)
}

fn answer(endpoint: &Endpoint, target: &str) -> Checked<(u16, Value)> {
    let (status, bytes) = endpoint.get(target)?;
    Ok((status, serde_json::from_slice(&bytes)?))
}

#[test]
fn names_api_answers_match_the_committed_vector() -> Checked<()> {
    let database = Database::new();
    let store = Store::open(&database.0)?;
    let head = Arc::new(AtomicU64::new(2));
    let ingester = node(Arc::clone(&head))?;
    assert_eq!(
        ingester.step(&store)?,
        StepOutcome::Advanced {
            units: 2,
            position: 2
        }
    );
    let chain = fixture();
    let resolved = chain["alice"].as_str().ok_or("resolved address missing")?;
    let fallback = api::route(&store, "GET", &format!("/api/v1/addresses/{resolved}"));
    assert_eq!(fallback.status, 200);
    let fallback: Value = serde_json::from_str(&fallback.body)?;
    assert_eq!(fallback["domain"]["id"], chain["node"]);
    assert_eq!(fallback["resolved_domains_count"], 1);
    drop(store);

    let store = Store::open(&database.0)?;
    head.store(3, Ordering::SeqCst);
    assert_eq!(
        ingester.step(&store)?,
        StepOutcome::Advanced {
            units: 1,
            position: 3
        }
    );
    drop(store);
    let endpoint = api_endpoint(Store::open(&database.0)?)?;
    let vector: Value = serde_json::from_str(include_str!(
        "../../../../../tests/vectors/names/indexer_api_v1.json"
    ))?;
    let entries = vector.as_object().ok_or("route vector must be an object")?;
    assert_eq!(entries.len(), 6);
    for (target, expected) in entries {
        let (status, body) = answer(&endpoint, target)?;
        assert_eq!(status, 200, "{target}: {body}");
        assert_eq!(&body, expected, "{target}");
    }
    let detail = &vector["/api/v1/domains/alice.pax"];
    for name in ["alice", "ALICE", "ALICE.PAX", "%61lice.PAX"] {
        let target = format!("/api/v1/domains/{name}");
        let (status, body) = answer(&endpoint, &target)?;
        assert_eq!(status, 200, "{target}");
        assert_eq!(&body, detail, "{target}");
    }
    let summary = &vector["/api/v1/domains:lookup?name=alice.pax&only_active=true"];
    for name in ["alice", "ALICE", "ALICE.PAX", "alice%2Epax"] {
        let (status, body) = answer(
            &endpoint,
            &format!("/api/v1/domains:lookup?name={name}&only_active=false"),
        )?;
        assert_eq!(status, 200);
        assert_eq!(&body, summary);
    }
    let (status, ascending) = answer(&endpoint, "/api/v1/domains/ALICE/events?order=ASC")?;
    assert_eq!(status, 200);
    let mut descending = vector["/api/v1/domains/alice.pax/events?order=DESC"]["items"]
        .as_array()
        .ok_or("event vector missing")?
        .clone();
    descending.reverse();
    assert_eq!(
        ascending,
        json!({"items": descending, "next_page_params": null})
    );
    let (status, resolved_lookup) = answer(&endpoint, &format!(
        "/api/v1/addresses:lookup?address={resolved}&owned_by=false&only_active=true&sort=registration_date&order=ASC"
    ))?;
    assert_eq!(status, 200);
    assert_eq!(&resolved_lookup, summary);
    let (status, old_owner) = answer(
        &endpoint,
        &format!("/api/v1/addresses:lookup?address={resolved}&owned_by=true"),
    )?;
    assert_eq!(status, 200);
    assert_eq!(old_owner, json!({"items": [], "next_page_params": null}));
    let owner = chain["bob"]
        .as_str()
        .ok_or("owner missing")?
        .to_ascii_uppercase();
    let (status, owned) = answer(
        &endpoint,
        &format!("/api/v1/addresses:lookup?address={owner}&only_active=false&order=ASC"),
    )?;
    assert_eq!(status, 200);
    assert_eq!(&owned, summary);
    Ok(())
}

#[test]
fn names_api_refuses_unknown_parameters_and_names() -> Checked<()> {
    let store = Store::open_in_memory()?;
    let ingester = node(Arc::new(AtomicU64::new(3)))?;
    assert_eq!(
        ingester.step(&store)?,
        StepOutcome::Advanced {
            units: 3,
            position: 3
        }
    );
    let endpoint = api_endpoint(store)?;
    for target in [
        "/api/v1/addresses:lookup?address=0x2222222222222222222222222222222222222222&unknown=true",
        "/api/v1/domains/alice.pax?unknown=true",
        "/api/v1/domains/alice.pax/events?order=ASC&unknown=true",
        "/api/v1/addresses/0x1111111111111111111111111111111111111111?unknown=true",
        "/api/v1/domains:lookup?name=alice.pax&unknown=true",
        "/api/v1/domains:lookup?name=alice.pax&un%6bnown=true",
    ] {
        let (status, body) = answer(&endpoint, target)?;
        assert_eq!(status, 400, "{target}: {body}");
        assert_eq!(
            body,
            json!({"error": {"code": "unknown_query_parameter", "retry": "never"}}),
            "{target}"
        );
    }
    for target in [
        "/api/v1/domains/bob.pax",
        "/api/v1/domains/bob.pax/events?order=DESC",
        "/api/v1/domains:lookup?name=bob.pax&only_active=true",
    ] {
        let (status, body) = answer(&endpoint, target)?;
        assert_eq!(status, 404, "{target}: {body}");
        assert_eq!(
            body,
            json!({"error": {"code": "domain_not_found", "retry": "never"}}),
            "{target}"
        );
    }
    for (target, code) in [
        ("/api/v1/addresses:lookup", "missing_address"),
        ("/api/v1/domains:lookup", "missing_name"),
        ("/api/v1/addresses:lookup?address=0x1234", "invalid_address"),
        ("/api/v1/addresses/0x1234", "invalid_address"),
        (
            "/api/v1/domains/alice.pax/events?order=random",
            "invalid_order",
        ),
        (
            "/api/v1/domains:lookup?name=alice&name=bob",
            "duplicate_query_parameter",
        ),
        (
            "/api/v1/domains:lookup?name=alice&only_active=yes",
            "invalid_query",
        ),
        ("/api/v1/domains:lookup?name=", "invalid_name"),
    ] {
        let (status, body) = answer(&endpoint, target)?;
        assert_eq!(status, 400, "{target}: {body}");
        assert_eq!(body["error"]["code"], code, "{target}");
    }
    Ok(())
}
