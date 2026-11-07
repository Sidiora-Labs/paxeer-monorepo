use std::io::{Read, Write};
use std::net::{TcpListener, TcpStream};
use std::sync::atomic::{AtomicU64, Ordering};
use std::sync::Arc;
use std::thread;
use std::time::Duration;

use layerx_indexer::abi::{keccak, AbiRegistry};
use layerx_indexer::chains::{chains_abi, ChainsConfig};
use layerx_indexer::codec::hex0x;
use layerx_indexer::follow::{FollowPolicy, StepOutcome};
use layerx_indexer::paxeer::{AttributeEncoding, PaxeerIngester};
use layerx_indexer::store::Store;
use layerx_indexer::transport::{Endpoint, Security};
use serde_json::{json, Value};

type Checked<T> = Result<T, Box<dyn std::error::Error>>;

fn fixture() -> Value {
    serde_json::from_str(include_str!("fixtures/chains_blocks.json"))
        .unwrap_or_else(|error| panic!("chains fixture: {error}"))
}

fn registry() -> Checked<AbiRegistry> {
    let mut registry = AbiRegistry::default();
    for abi in chains_abi(&ChainsConfig::from_lookup(&|_| None)?) {
        registry.insert(abi);
    }
    Ok(registry)
}

fn check_topics() -> Checked<()> {
    let fixture = fixture();
    let receipts = fixture["receipts"].as_object().ok_or("receipts missing")?;
    let mut signatures = std::collections::BTreeSet::new();
    for receipt in receipts.values() {
        for log in receipt["logs"].as_array().ok_or("logs missing")? {
            let signature = log["signature"].as_str().ok_or("signature missing")?;
            assert_eq!(log["topics"][0], hex0x(&keccak(signature.as_bytes())));
            signatures.insert(signature);
        }
    }
    assert_eq!(signatures.len(), 4);
    Ok(())
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

fn node(head: Arc<AtomicU64>, finality_depth: u64) -> Checked<PaxeerIngester> {
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
        registry()?,
        FollowPolicy {
            finality_depth,
            max_units_per_step: 10,
        },
        42,
        Some(125),
        AttributeEncoding::Plain,
    ))
}

static NEXT_DATABASE: AtomicU64 = AtomicU64::new(0);
struct Database(std::path::PathBuf);
impl Database {
    fn new() -> Self {
        Self(
            std::path::PathBuf::from(env!("CARGO_TARGET_TMPDIR")).join(format!(
                "chains-{}-{}.sqlite",
                std::process::id(),
                NEXT_DATABASE.fetch_add(1, Ordering::Relaxed)
            )),
        )
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
    let listener = layerx_indexer::api::bind("127.0.0.1:0".parse()?, false)?;
    let endpoint = Endpoint::parse(
        &format!("http://{}", listener.local_addr()?),
        Security::Plaintext {
            allow_remote: false,
        },
        Duration::from_secs(5),
    )?;
    let store = Arc::new(store);
    thread::spawn(move || layerx_indexer::api::serve(&listener, &store, None));
    Ok(endpoint)
}

fn answer(endpoint: &Endpoint, target: &str) -> Checked<(u16, Value)> {
    let (status, bytes) = endpoint.get(target)?;
    Ok((status, serde_json::from_slice(&bytes)?))
}

#[test]
fn chains_api_lists_and_reads_one_chain() -> Checked<()> {
    check_topics()?;
    let database = Database::new();
    let store = Store::open(&database.0)?;
    let ingester = node(Arc::new(AtomicU64::new(45)), 100)?;
    assert_eq!(
        ingester.step(&store)?,
        StepOutcome::Advanced {
            units: 4,
            position: 45
        }
    );
    drop(store);
    let endpoint = api_endpoint(Store::open(&database.0)?)?;
    let expected = json!({"id":1,"name":"demo","operator":fixture()["bob"],"status":"active",
        "chain_id":125000001,"created_height":42,"bond":"100000000000000000007",
        "gas_allowance":500000,"byte_allowance":65536,"fee_share_bps":100});
    let (status, list) = answer(&endpoint, "/v1/chains")?;
    assert_eq!(status, 200);
    let chains = list["chains"].as_array().ok_or("chains missing")?;
    assert_eq!(
        chains
            .iter()
            .map(|chain| chain["id"].as_u64())
            .collect::<Vec<_>>(),
        vec![Some(1), Some(2), Some(3), Some(4)]
    );
    assert_eq!(chains[0], expected);
    assert_eq!(chains[1]["status"], "paused");
    assert_eq!(chains[2]["status"], "governance_paused");
    assert_eq!(chains[3]["status"], "retired");
    for target in [
        "/v1/chains/1",
        "/v1/chains/by-name/demo",
        "/v1/chains/by-name/%64emo",
    ] {
        let (status, body) = answer(&endpoint, target)?;
        assert_eq!(status, 200, "{target}: {body}");
        assert_eq!(body, expected, "{target}");
    }
    for target in [
        "/v1/chains/999",
        "/v1/chains/by-name/unknown",
        "/v1/chains/999/summary",
        "/v1/chains/999/transfers",
        "/v1/chains/999/events",
    ] {
        assert_eq!(
            answer(&endpoint, target)?,
            (404, json!({"error":"unknown_chain"})),
            "{target}"
        );
    }
    assert_eq!(
        answer(&endpoint, "/v1/chains/1/summary")?,
        (
            200,
            json!({"id":1,"tx_count":2,"gas_used":42005,"last_height":44})
        )
    );
    assert_eq!(
        answer(&endpoint, "/v1/chains/2/summary")?,
        (
            200,
            json!({"id":2,"tx_count":1,"gas_used":21002,"last_height":44})
        )
    );
    assert_eq!(
        answer(&endpoint, "/v1/chains/3/summary")?,
        (
            200,
            json!({"id":3,"tx_count":0,"gas_used":0,"last_height":0})
        )
    );
    for (feed, count) in [("transfers", 4), ("events", 2)] {
        let target = format!("/v1/chains/1/{feed}");
        let (status, full) = answer(&endpoint, &target)?;
        assert_eq!(status, 200);
        assert_eq!(full["version"], 1);
        assert_eq!(full["indexedHeight"], 45);
        assert!(full["next_cursor"].is_null());
        let all = full["items"].as_array().ok_or("items missing")?;
        assert_eq!(all.len(), count);
        assert!(all.iter().all(|row| row["chain"] == "pax:1"));
        let mut paged = Vec::new();
        let mut cursor = None;
        for _ in 0..=count {
            let query = cursor.as_ref().map_or("?limit=1".to_owned(), |cursor| {
                format!("?limit=1&cursor={cursor}")
            });
            let (status, body) = answer(&endpoint, &format!("{target}{query}"))?;
            assert_eq!(status, 200);
            paged.extend(
                body["items"]
                    .as_array()
                    .ok_or("page items missing")?
                    .iter()
                    .cloned(),
            );
            cursor = body["next_cursor"].as_str().map(str::to_owned);
            if cursor.is_none() {
                break;
            }
        }
        assert!(cursor.is_none(), "pagination did not terminate");
        assert_eq!(&paged, all);
        let (status, other) = answer(&endpoint, &format!("/v1/chains/2/{feed}"))?;
        assert_eq!(status, 200);
        assert!(other["items"]
            .as_array()
            .ok_or("other items missing")?
            .iter()
            .all(|row| row["chain"] == "pax:2"));
        let (status, empty) = answer(&endpoint, &format!("/v1/chains/3/{feed}"))?;
        assert_eq!(status, 200);
        assert_eq!(empty["items"], json!([]));
    }
    for target in [
        "/v1/chains?unknown=1",
        "/v1/chains/1?unknown=1",
        "/v1/chains/1/events?cursor=bad",
        "/v1/chains/1/transfers?limit=0",
    ] {
        assert_eq!(answer(&endpoint, target)?.0, 400, "{target}");
    }
    Ok(())
}

#[test]
fn chains_api_virtual_transfers_follow_the_shared_cursor() -> Checked<()> {
    for depth in [0, 1] {
        let database = Database::new();
        let store = Store::open(&database.0)?;
        let ingester = node(Arc::new(AtomicU64::new(44)), depth)?;
        assert_eq!(
            ingester.step(&store)?,
            StepOutcome::Advanced {
                units: 3,
                position: 44
            }
        );
        let boundary = store
            .cursor(layerx_indexer::paxeer::CHAIN)?
            .ok_or("shared cursor missing")?
            .finalized_boundary;
        assert_eq!(boundary, Some(44 - depth));
        let endpoint = api_endpoint(Store::open(&database.0)?)?;
        for (id, count) in [(1, 4), (2, 2)] {
            let target = format!("/v1/chains/{id}/transfers");
            let (status, full) = answer(&endpoint, &target)?;
            assert_eq!(status, 200);
            assert_eq!(full["indexedHeight"], 44);
            let rows = full["items"].as_array().ok_or("transfers missing")?;
            assert_eq!(rows.len(), count);
            assert_eq!(rows, &store.transfers(&format!("pax:{id}"))?);
            for row in rows {
                assert_eq!(row["chain"], format!("pax:{id}"));
                assert_eq!(row["height_or_seq"], "44");
                assert_eq!(row["final"], depth == 0);
                assert_eq!(row["final_basis"], "block_commit");
                assert_eq!(
                    row["stability"],
                    json!({
                        "level": if depth == 0 { "depth_stable" } else { "reversible" },
                        "source": "block_commit",
                        "finalized_boundary": boundary.map(|height| height.to_string()),
                    })
                );
                assert_eq!(
                    row["settlement"],
                    json!({
                        "level": "unverified", "source": null,
                        "reason": "no_verified_checkpoint_evidence",
                    })
                );
            }
            let (status, page) = answer(&endpoint, &format!("{target}?limit=1"))?;
            assert_eq!(status, 200);
            assert_eq!(page["items"], json!([rows[0]]));
            assert!(page["next_cursor"].is_string());
        }
    }
    Ok(())
}

#[test]
fn chains_api_status_codes_follow_the_registry_and_rollback() -> Checked<()> {
    let database = Database::new();
    let store = Store::open(&database.0)?;
    let head = Arc::new(AtomicU64::new(42));
    let ingester = node(Arc::clone(&head), 100)?;
    let endpoint = api_endpoint(Store::open(&database.0)?)?;
    for (height, statuses) in [
        (42, ["active", "active", "active", "active"]),
        (43, ["paused", "active", "governance_paused", "active"]),
        (44, ["active", "paused", "governance_paused", "active"]),
        (45, ["active", "paused", "governance_paused", "retired"]),
        (46, ["active", "paused", "governance_paused", "retired"]),
    ] {
        head.store(height, Ordering::SeqCst);
        assert_eq!(
            ingester.step(&store)?,
            StepOutcome::Advanced {
                units: 1,
                position: height
            }
        );
        for (index, expected) in statuses.into_iter().enumerate() {
            let id = index as u32 + 1;
            let (status, body) = answer(&endpoint, &format!("/v1/chains/{id}"))?;
            assert_eq!(status, 200);
            assert_eq!(body["status"], expected, "block {height}, chain {id}");
            assert_eq!(
                body["status"],
                store.chain(id)?.ok_or("chain missing")?.status
            );
        }
    }
    store.rollback("paxeer", Some(44))?;
    let (status, body) = answer(&endpoint, "/v1/chains/4")?;
    assert_eq!(status, 200);
    assert_eq!(body["status"], "active");
    assert_eq!(
        ingester.step(&store)?,
        StepOutcome::Advanced {
            units: 2,
            position: 46
        }
    );
    let (status, body) = answer(&endpoint, "/v1/chains/4")?;
    assert_eq!(status, 200);
    assert_eq!(body["status"], "retired");
    Ok(())
}
