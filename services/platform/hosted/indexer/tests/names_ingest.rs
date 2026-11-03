use std::collections::BTreeSet;
use std::io::{Read, Write};
use std::net::{TcpListener, TcpStream};
use std::path::PathBuf;
use std::sync::atomic::{AtomicU64, Ordering};
use std::sync::Arc;
use std::thread;
use std::time::Duration;

use layerx_indexer::abi::{keccak, AbiRegistry, DecodedEvent};
use layerx_indexer::codec::{hex0x, unhex, unhex_fixed};
use layerx_indexer::follow::{FollowPolicy, StepOutcome};
use layerx_indexer::names::{names_abi, NamesConfig};
use layerx_indexer::paxeer::{AttributeEncoding, PaxeerIngester};
use layerx_indexer::store::Store;
use layerx_indexer::transport::{Endpoint, Security};
use layerx_indexer::IndexError;
use rusqlite::{Connection, OptionalExtension};
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
            "names-{}-{}.sqlite",
            std::process::id(),
            NEXT_DATABASE.fetch_add(1, Ordering::Relaxed)
        )))
    }
    fn read(&self) -> rusqlite::Result<Connection> {
        Connection::open(&self.0)
    }
}
impl Drop for Database {
    fn drop(&mut self) {
        for suffix in ["", "-wal", "-shm"] {
            let _ = std::fs::remove_file(format!("{}{suffix}", self.0.display()));
        }
    }
}

fn name(connection: &Connection, node: &str) -> Checked<Option<Value>> {
    Ok(connection.query_row(
        "SELECT label, name, owner, registrant, resolver, resolved_address, contenthash,
            registered_at, expires_at, token_id FROM names WHERE node = ?1", [node],
        |row| Ok(json!({
            "label":row.get::<_, Option<String>>(0)?, "name":row.get::<_, Option<String>>(1)?,
            "owner":row.get::<_, Option<String>>(2)?, "registrant":row.get::<_, Option<String>>(3)?,
            "resolver":row.get::<_, Option<String>>(4)?, "resolved_address":row.get::<_, Option<String>>(5)?,
            "contenthash":row.get::<_, Option<String>>(6)?, "registered_at":row.get::<_, Option<i64>>(7)?,
            "expires_at":row.get::<_, Option<String>>(8)?, "token_id":row.get::<_, Option<String>>(9)?,
        })),
    ).optional()?)
}

fn events(connection: &Connection) -> Checked<Vec<Value>> {
    let mut statement = connection.prepare(
        "SELECT chain, position, height, tx_hash, log_index, node, action, from_address, timestamp, data
         FROM name_events ORDER BY height, log_index",
    )?;
    let rows = statement.query_map([], |row| {
        Ok(json!({
            "chain":row.get::<_, String>(0)?, "position":row.get::<_, i64>(1)?,
            "height":row.get::<_, i64>(2)?, "tx_hash":row.get::<_, String>(3)?,
            "log_index":row.get::<_, i64>(4)?, "node":row.get::<_, String>(5)?,
            "action":row.get::<_, String>(6)?, "from_address":row.get::<_, String>(7)?,
            "timestamp":row.get::<_, i64>(8)?, "data":row.get::<_, String>(9)?,
        }))
    })?;
    rows.map(|row| {
        let mut row = row?;
        row["data"] = serde_json::from_str(row["data"].as_str().ok_or("data is not JSON text")?)?;
        Ok(row)
    })
    .collect()
}

#[test]
fn names_ingest_builds_the_view_and_events() -> Checked<()> {
    assert_eq!(NamesConfig::from_lookup(&|_| None)?, None);
    assert!(matches!(
        NamesConfig::from_lookup(&|key| (key.ends_with("REGISTRY"))
            .then(|| "0x0101010101010101010101010101010101010101".to_owned())),
        Err(IndexError::Config(_))
    ));
    for invalid in [
        "",
        "0x1234",
        "0xzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz",
        "0x111111111111111111111111111111111111111111",
    ] {
        assert!(matches!(
            NamesConfig::from_lookup(&|_| Some(invalid.to_owned())),
            Err(IndexError::Config(_))
        ));
    }
    let chain = fixture();
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
    let connection = database.read()?;
    let row =
        name(&connection, chain["node"].as_str().ok_or("node missing")?)?.ok_or("name missing")?;
    assert_eq!(
        row,
        json!({
            "label":"alice", "name":"alice.pax", "owner":chain["bob"], "registrant":chain["bob"],
            "resolver":null, "resolved_address":chain["alice"], "contenthash":chain["contenthash"],
            "registered_at":1800000001, "expires_at":"2100000000", "token_id":chain["token_id"],
        })
    );
    let first_seven = events(&connection)?;
    assert_eq!(first_seven.len(), 7);
    let expected = [
        (1, 0, "register", "names_registrar", "NameRegistered"),
        (1, 1, "register", "names_controller", "NameRegistered"),
        (1, 2, "transfer", "names_registry", "NewOwner"),
        (1, 3, "set_addr", "names_resolver", "AddrChanged"),
        (
            1,
            4,
            "set_contenthash",
            "names_resolver",
            "ContenthashChanged",
        ),
        (2, 0, "renew", "names_controller", "NameRenewed"),
        (2, 1, "transfer", "names_registrar", "Transfer"),
    ];
    for (row, (height, log_index, action, contract, event)) in first_seven.iter().zip(expected) {
        let block = &chain["blocks"][height.to_string()];
        assert_eq!(row["chain"], "paxeer");
        assert_eq!(row["height"], height);
        assert_eq!(row["position"], height);
        assert_eq!(row["tx_hash"], block["transactions"][0]["hash"]);
        assert_eq!(row["log_index"], log_index);
        assert_eq!(row["node"], chain["node"]);
        assert_eq!(row["action"], action);
        assert_eq!(row["from_address"], block["transactions"][0]["from"]);
        assert_eq!(row["timestamp"], 1800000000 + height);
        assert_eq!(row["data"]["contract"], contract);
        assert_eq!(row["data"]["event"], event);
    }
    assert_eq!(
        unhex(row["contenthash"].as_str().ok_or("contenthash missing")?)?.len(),
        36
    );
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
    let all = events(&connection)?;
    assert_eq!(all.len(), 9);
    assert_eq!(&all[..7], first_seven.as_slice());
    assert_eq!(all[7]["action"], "set_text");
    assert_eq!(all[7]["data"]["args"]["key"], "url");
    assert_eq!(all[7]["data"]["args"]["value"], "https://alice.pax");
    assert_eq!(all[8]["action"], "reverse_name_set");
    assert_eq!(all[8]["node"], chain["reverse_node"]);
    assert_eq!(all[8]["data"]["contract"], "names_resolver");
    assert_eq!(all[8]["data"]["event"], "NameChanged");
    assert_eq!(all[8]["data"]["args"]["name"], "alice.pax");
    assert_eq!(
        all[8]["from_address"],
        chain["blocks"]["3"]["transactions"][0]["from"]
    );
    assert_eq!(all[8]["timestamp"], 1800000003_u64);
    let reverse_sender = chain["blocks"]["3"]["transactions"][0]["from"]
        .as_str()
        .ok_or("reverse sender missing")?;
    assert_eq!(
        layerx_indexer::names::reverse_node(reverse_sender),
        Some(unhex_fixed(
            chain["reverse_node"]
                .as_str()
                .ok_or("reverse node missing")?
        )?)
    );
    assert_eq!(
        layerx_indexer::names::reverse_node(&reverse_sender.to_ascii_uppercase()),
        layerx_indexer::names::reverse_node(reverse_sender)
    );
    assert_eq!(
        name(
            &connection,
            chain["reverse_node"]
                .as_str()
                .ok_or("reverse node missing")?
        )?
        .ok_or("reverse name missing")?["name"],
        "alice.pax"
    );
    assert_eq!(
        name(&connection, chain["node"].as_str().ok_or("node missing")?)?,
        Some(row)
    );

    let registry = registry();
    for (index, action) in ["transfer", "set_resolver", "name_changed"]
        .into_iter()
        .enumerate()
    {
        let log = &chain["other_logs"][index];
        let emitter = unhex_fixed(log["address"].as_str().ok_or("emitter missing")?)?;
        let topics = log["topics"]
            .as_array()
            .ok_or("topics missing")?
            .iter()
            .map(|topic| {
                unhex_fixed(
                    topic
                        .as_str()
                        .ok_or_else(|| IndexError::Decode("topic missing".to_owned()))?,
                )
            })
            .collect::<Result<Vec<_>, _>>()?;
        let data = unhex(log["data"].as_str().ok_or("data missing")?)?;
        assert!(registry.decode_log([0xee; 20], &topics, &data)?.is_none());
        assert!(registry
            .decode_log(config().reverse, &topics, &data)?
            .is_none());
        let (_, decoded) = registry
            .decode_log(emitter, &topics, &data)?
            .ok_or("event missing")?;
        let decoded = layerx_indexer::names::decode(
            &decoded,
            chain["sender"].as_str().ok_or("sender missing")?,
            42,
        )
        .ok_or("name event missing")?;
        assert_eq!(decoded.action, action);
        assert_eq!(decoded.timestamp, 42);
        assert_eq!(decoded.from_address, chain["sender"]);
    }
    Ok(())
}

#[test]
fn names_registry_owner_and_resolver_changes_are_not_registrations() -> Checked<()> {
    let parent = [0x11; 32];
    let label = [0x22; 32];
    let node = hex0x(&parent);
    let labelhash = hex0x(&label);
    let owner = hex0x(&[0x33; 20]);
    let resolver = hex0x(&[0x44; 20]);
    let sender = hex0x(&[0x55; 20]);
    for (name, signature, args, expected) in [
        (
            "NewOwner",
            "NewOwner(bytes32,bytes32,address)",
            json!({"node":node, "label":labelhash, "owner":owner}),
            Some(("transfer", json!({"owner":owner}))),
        ),
        (
            "Transfer",
            "Transfer(bytes32,address)",
            json!({"node":node, "owner":owner}),
            Some(("transfer", json!({"owner":owner}))),
        ),
        (
            "NewResolver",
            "NewResolver(bytes32,address)",
            json!({"node":node, "resolver":resolver}),
            Some(("set_resolver", json!({"resolver":resolver}))),
        ),
        (
            "NewTTL",
            "NewTTL(bytes32,uint64)",
            json!({"node":node, "ttl":"3600"}),
            None,
        ),
        (
            "ApprovalForAll",
            "ApprovalForAll(address,address,bool)",
            json!({"owner":owner, "operator":sender, "approved":true}),
            None,
        ),
    ] {
        let event = DecodedEvent {
            contract: "names_registry".to_owned(),
            name: name.to_owned(),
            signature: signature.to_owned(),
            args: args.as_object().ok_or("event args missing")?.clone(),
        };
        let row = layerx_indexer::names::decode(&event, &sender, 42);
        if let Some((action, changes)) = expected {
            let row = row.ok_or("registry name event missing")?;
            assert_eq!(row.action, action, "{name}");
            assert_ne!(row.action, "register", "{name}");
            assert_ne!(row.action, "set_addr", "{name}");
            assert_eq!(row.data["changes"], changes, "{name}");
            let expected_node = if name == "NewOwner" {
                hex0x(&layerx_indexer::names::child_node(parent, label))
            } else {
                node.clone()
            };
            assert_eq!(row.node, expected_node, "{name}");
        } else {
            assert!(row.is_none(), "unexpected registry name event: {name}");
        }
    }
    for (contract, signature, args) in [
        (
            "names_registrar",
            "NameRegistered(uint256,address,uint256)",
            json!({"id":"1", "owner":owner, "expires":"2000000000"}),
        ),
        (
            "names_controller",
            "NameRegistered(string,bytes32,address,uint256,uint256,bytes32)",
            json!({"label":"alice", "labelhash":labelhash, "owner":owner,
                "cost":"1", "expires":"2000000000", "referrer":hex0x(&[0; 32])}),
        ),
    ] {
        let event = DecodedEvent {
            contract: contract.to_owned(),
            name: "NameRegistered".to_owned(),
            signature: signature.to_owned(),
            args: args.as_object().ok_or("registration args missing")?.clone(),
        };
        let row = layerx_indexer::names::decode(&event, &sender, 42)
            .ok_or("registration event missing")?;
        assert_eq!(row.action, "register", "{contract}");
        assert_eq!(row.data["changes"]["owner"], owner, "{contract}");
    }
    Ok(())
}

#[test]
fn names_rollback_replays_the_view() -> Checked<()> {
    let chain = fixture();
    let database = Database::new();
    let store = Store::open(&database.0)?;
    let head = Arc::new(AtomicU64::new(3));
    let ingester = node(Arc::clone(&head))?;
    assert_eq!(
        ingester.step(&store)?,
        StepOutcome::Advanced {
            units: 3,
            position: 3
        }
    );
    drop(store);
    let store = Store::open(&database.0)?;
    head.store(1, Ordering::SeqCst);
    assert_eq!(
        ingester.step(&store)?,
        StepOutcome::RolledBack { fork: Some(1) }
    );
    let connection = database.read()?;
    let row =
        name(&connection, chain["node"].as_str().ok_or("node missing")?)?.ok_or("name missing")?;
    assert_eq!(row["owner"], chain["alice"]);
    assert_eq!(row["registrant"], chain["alice"]);
    assert_eq!(row["expires_at"], "2000000000");
    assert_eq!(row["resolved_address"], chain["alice"]);
    assert_eq!(row["contenthash"], chain["contenthash"]);
    assert_eq!(row["registered_at"], 1800000001);
    assert_eq!(events(&connection)?.len(), 5);
    assert!(name(
        &connection,
        chain["reverse_node"]
            .as_str()
            .ok_or("reverse node missing")?
    )?
    .is_none());
    head.store(3, Ordering::SeqCst);
    assert_eq!(
        ingester.step(&store)?,
        StepOutcome::Advanced {
            units: 2,
            position: 3
        }
    );
    assert_eq!(events(&connection)?.len(), 9);
    assert_eq!(
        name(&connection, chain["node"].as_str().ok_or("node missing")?)?.ok_or("name missing")?
            ["owner"],
        chain["bob"]
    );
    store.rollback("paxeer", None)?;
    assert!(events(&connection)?.is_empty());
    assert!(name(&connection, chain["node"].as_str().ok_or("node missing")?)?.is_none());
    for height in 1..=3 {
        let (block, receipts) = ingester
            .block_with_receipts(height)?
            .ok_or("block missing")?;
        let unit = ingester.decode(&block, &receipts, &[], &|_| Ok(false))?;
        store.commit_backfill(&unit, 10)?;
    }
    assert_eq!(events(&connection)?.len(), 9);
    assert_eq!(
        name(&connection, chain["node"].as_str().ok_or("node missing")?)?
            .ok_or("backfilled name missing")?["expires_at"],
        "2100000000"
    );
    store.rollback("paxeer", Some(1))?;
    assert_eq!(events(&connection)?.len(), 5);
    assert_eq!(
        name(&connection, chain["node"].as_str().ok_or("node missing")?)?,
        Some(row)
    );
    Ok(())
}

fn declarations(source: &str) -> Vec<String> {
    source
        .split("event ")
        .skip(1)
        .map(|declaration| {
            let (name, rest) = declaration
                .split_once('(')
                .unwrap_or_else(|| panic!("event has no inputs"));
            let params = rest
                .split_once(')')
                .unwrap_or_else(|| panic!("event has no closing parenthesis"))
                .0;
            let types = params
                .split(',')
                .map(|param| {
                    let kind = param
                        .split_whitespace()
                        .next()
                        .unwrap_or_else(|| panic!("event parameter missing"));
                    match kind {
                        "uint" => "uint256",
                        "int" => "int256",
                        other => other,
                    }
                })
                .collect::<Vec<_>>();
            format!("{}({})", name.trim(), types.join(","))
        })
        .collect()
}

#[test]
fn names_abi_topics_match_the_interface() -> Checked<()> {
    let root = PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("../../../..");
    let abis = names_abi(&config())?;
    assert_eq!(abis.len(), 4);
    assert_eq!(
        abis.iter()
            .map(|abi| (abi.contract.as_str(), abi.address))
            .collect::<Vec<_>>(),
        vec![
            ("names_registry", Some(config().registry)),
            ("names_registrar", Some(config().registrar)),
            ("names_controller", Some(config().controller)),
            ("names_resolver", Some(config().resolver)),
        ]
    );
    let events: Vec<_> = abis.iter().flat_map(|abi| &abi.events).collect();
    let interface = std::fs::read_to_string(root.join("contracts/names/IPaxNames.sol"))?;
    let interface_signatures = declarations(&interface);
    assert_eq!(interface_signatures.len(), 7);
    for signature in &interface_signatures {
        let event = events
            .iter()
            .find(|event| event.signature == *signature)
            .unwrap_or_else(|| panic!("ABI omits interface event {signature}"));
        assert_eq!(hex0x(&event.topic), hex0x(&keccak(signature.as_bytes())));
    }
    let mut declared: BTreeSet<_> = interface_signatures.into_iter().collect();
    for file in ["PaxBaseRegistrar.sol", "PaxPublicResolver.sol"] {
        declared.extend(declarations(&std::fs::read_to_string(
            root.join("contracts/names").join(file),
        )?));
    }
    for event in events {
        assert!(
            declared.contains(&event.signature),
            "ABI event {} has no contract/interface declaration",
            event.signature
        );
        assert_eq!(event.topic, keccak(event.signature.as_bytes()));
    }
    Ok(())
}
