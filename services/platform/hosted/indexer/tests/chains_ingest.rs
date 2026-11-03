use std::io::{Read, Write};
use std::net::{TcpListener, TcpStream};
use std::sync::atomic::{AtomicU64, Ordering};
use std::sync::Arc;
use std::thread;
use std::time::Duration;

use layerx_indexer::abi::{keccak, AbiRegistry};
use layerx_indexer::chains::{chains_abi, ChainsConfig};
use layerx_indexer::codec::{hex0x, unhex, unhex_fixed};
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
        registry()?,
        FollowPolicy {
            finality_depth: 100,
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

#[test]
fn chains_ingest_records_create_status_fund_and_operator_changes() -> Checked<()> {
    check_topics()?;
    assert_eq!(
        ChainsConfig::from_lookup(&|_| None)?,
        ChainsConfig::default()
    );
    let custom = "0x1111111111111111111111111111111111111111";
    let configured = ChainsConfig::from_lookup(&|key| {
        assert_eq!(key, "LAYERX_INDEXER_CHAINS_PRECOMPILE");
        Some(custom.to_owned())
    })?;
    assert_eq!(
        hex0x(
            &chains_abi(&configured)[0]
                .address
                .ok_or("ABI is not bound")?
        ),
        custom
    );
    for invalid in ["", "0x1234", "0xzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz"] {
        assert!(ChainsConfig::from_lookup(&|_| Some(invalid.to_owned())).is_err());
    }
    let database = Database::new();
    let store = Store::open(&database.0)?;
    let head = Arc::new(AtomicU64::new(42));
    let ingester = node(Arc::clone(&head))?;
    assert_eq!(
        ingester.step(&store)?,
        StepOutcome::Advanced {
            units: 1,
            position: 42
        }
    );
    let initial = store.chain(1)?.ok_or("chain missing")?;
    assert_eq!(
        serde_json::to_value(&initial)?,
        json!({
            "id":1,"name":"demo","operator":fixture()["alice"],"status":"active",
            "chain_id":125000001,"created_height":42,"bond":"100000000000000000000",
            "gas_allowance":500000,"byte_allowance":65536,"fee_share_bps":100
        })
    );
    assert_eq!(store.chain_by_name("demo")?, Some(initial.clone()));
    assert_eq!(
        store.chains()?.iter().map(|row| row.id).collect::<Vec<_>>(),
        vec![1, 2, 3, 4]
    );
    assert!(store.chain(999)?.is_none());
    assert!(store.chain_by_name("unknown")?.is_none());
    head.store(43, Ordering::SeqCst);
    assert_eq!(
        ingester.step(&store)?,
        StepOutcome::Advanced {
            units: 1,
            position: 43
        }
    );
    let updated = store.chain(1)?.ok_or("updated chain missing")?;
    assert_eq!(updated.bond, "100000000000000000007");
    assert_eq!(updated.operator, fixture()["bob"]);
    assert_eq!(updated.status, "paused");
    assert_eq!(updated.created_height, 42);
    assert_eq!(
        store.chain(3)?.ok_or("governed chain missing")?.status,
        "governance_paused"
    );
    drop(store);
    let store = Store::open(&database.0)?;
    assert_eq!(store.chain(1)?, Some(updated.clone()));
    assert_eq!(ingester.step(&store)?, StepOutcome::Idle);
    head.store(44, Ordering::SeqCst);
    assert_eq!(
        ingester.step(&store)?,
        StepOutcome::Advanced {
            units: 1,
            position: 44
        }
    );
    assert_eq!(store.chain(1)?.ok_or("chain missing")?.status, "active");
    assert_eq!(
        store.chain_summary(1)?,
        json!({"id":1,"tx_count":2,"gas_used":42005,"last_height":44})
    );
    assert_eq!(store.transfers("pax:1")?.len(), 4);
    assert_eq!(store.events("pax:1")?.len(), 2);
    store.rollback("paxeer", Some(43))?;
    assert_eq!(store.chain(1)?, Some(updated));
    assert_eq!(
        store.chain_summary(1)?,
        json!({"id":1,"tx_count":0,"gas_used":0,"last_height":0})
    );
    assert!(store.transfers("pax:1")?.is_empty());
    assert!(store.events("pax:1")?.is_empty());
    store.rollback("paxeer", Some(42))?;
    assert_eq!(store.chain(1)?, Some(initial));
    store.rollback("paxeer", None)?;
    assert!(store.chains()?.is_empty());
    assert_eq!(
        ingester.step(&store)?,
        StepOutcome::Advanced {
            units: 3,
            position: 44
        }
    );
    assert_eq!(
        store.chain(1)?.ok_or("chain missing")?.bond,
        "100000000000000000007"
    );
    head.store(45, Ordering::SeqCst);
    assert_eq!(
        ingester.step(&store)?,
        StepOutcome::Advanced {
            units: 1,
            position: 45
        }
    );
    assert_eq!(
        store.chain(4)?.ok_or("old chain missing")?.status,
        "retired"
    );
    Ok(())
}

#[test]
fn chains_status_codes_follow_the_registry() -> Checked<()> {
    let database = Database::new();
    let store = Store::open(&database.0)?;
    let head = Arc::new(AtomicU64::new(42));
    let ingester = node(Arc::clone(&head))?;
    for (height, statuses) in [
        (42, ["active", "active", "active", "active"]),
        (43, ["paused", "active", "governance_paused", "active"]),
        (44, ["active", "paused", "governance_paused", "active"]),
        (45, ["active", "paused", "governance_paused", "retired"]),
    ] {
        head.store(height, Ordering::SeqCst);
        assert_eq!(
            ingester.step(&store)?,
            StepOutcome::Advanced {
                units: 1,
                position: height
            }
        );
        assert_eq!(
            store.cursor("paxeer")?.ok_or("cursor missing")?.position,
            height
        );
        for (index, status) in statuses.into_iter().enumerate() {
            assert_eq!(
                store
                    .chain(index as u32 + 1)?
                    .ok_or("chain missing")?
                    .status,
                status,
                "block {height}, chain {}",
                index + 1
            );
        }
    }
    let retired = store.chain(4)?.ok_or("retired chain missing")?;
    assert_eq!(retired.status, "retired");
    drop(store);
    let store = Store::open(&database.0)?;
    assert_eq!(store.chain(4)?, Some(retired.clone()));
    head.store(46, Ordering::SeqCst);
    assert_eq!(
        ingester.step(&store)?,
        StepOutcome::Advanced {
            units: 1,
            position: 46
        }
    );
    assert_eq!(
        store.cursor("paxeer")?.ok_or("cursor missing")?.position,
        46
    );
    assert_eq!(store.chain(4)?, Some(retired));
    assert_eq!(ingester.step(&store)?, StepOutcome::Idle);
    store.rollback("paxeer", Some(44))?;
    assert_eq!(store.chain(4)?.ok_or("chain missing")?.status, "active");
    assert_eq!(
        store.cursor("paxeer")?.ok_or("cursor missing")?.position,
        44
    );
    assert_eq!(
        ingester.step(&store)?,
        StepOutcome::Advanced {
            units: 2,
            position: 46
        }
    );
    assert_eq!(store.chain(4)?.ok_or("chain missing")?.status, "retired");
    Ok(())
}

#[test]
fn chains_status_codes_refuse_zero_and_out_of_range() -> Checked<()> {
    let fixture = fixture();
    let log = &fixture["receipts"][format!("0x{:064x}", 1043)]["logs"][0];
    let emitter = unhex_fixed(log["address"].as_str().ok_or("address missing")?)?;
    let topics = log["topics"]
        .as_array()
        .ok_or("topics missing")?
        .iter()
        .map(|topic| unhex_fixed(topic.as_str().expect("topic is a string")))
        .collect::<Result<Vec<_>, _>>()?;
    let registry = registry()?;
    for code in (0_u16..=255).filter(|code| !(1..=4).contains(code)) {
        let data = unhex(&format!("0x{code:064x}"))?;
        let (_, decoded) = registry
            .decode_log(emitter, &topics, &data)?
            .ok_or("status event missing")?;
        assert!(
            layerx_indexer::chains::decode(&decoded, 43, "").is_none(),
            "code {code}"
        );
    }
    Ok(())
}
