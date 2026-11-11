use std::collections::BTreeMap;
use std::sync::Arc;
use std::thread;
use std::time::Duration;

use layerx_indexer::abi::AbiRegistry;
use layerx_indexer::api;
use layerx_indexer::chains::{chains_abi, ChainsConfig};
use layerx_indexer::paxeer::{decode_block, AttributeEncoding, BlockInputs, CHAIN};
use layerx_indexer::store::{Store, Unit};
use layerx_indexer::transport::{Endpoint, Security};
use serde_json::{json, Value};

type Checked<T> = Result<T, Box<dyn std::error::Error>>;

struct Database(std::path::PathBuf);

impl Drop for Database {
    fn drop(&mut self) {
        for suffix in ["", "-wal", "-shm"] {
            let _ = std::fs::remove_file(format!("{}{suffix}", self.0.display()));
        }
    }
}

fn block(store: &Store, height: u64) -> Checked<Unit> {
    let fixture: Value = serde_json::from_str(include_str!("fixtures/chains_blocks.json"))?;
    fixture_block(store, &fixture["scope"], height)
}

fn fixture_block(store: &Store, fixture: &Value, height: u64) -> Checked<Unit> {
    let receipts: BTreeMap<String, Value> = serde_json::from_value(fixture["receipts"].clone())?;
    let mut registry = AbiRegistry::default();
    for abi in chains_abi(&ChainsConfig::default()) {
        registry.insert(abi);
    }
    Ok(decode_block(
        &fixture["blocks"][height.to_string()],
        &BlockInputs {
            registry: &registry,
            receipts: &receipts,
            cosmos_txs: &[],
            encoding: AttributeEncoding::Plain,
            is_pointer: &|address| store.is_pointer(address),
        },
    )?)
}

fn endpoint(store: Arc<Store>) -> Checked<Endpoint> {
    let listener = api::bind("127.0.0.1:0".parse()?, false)?;
    let endpoint = Endpoint::parse(
        &format!("http://{}", listener.local_addr()?),
        Security::Plaintext {
            allow_remote: false,
        },
        Duration::from_secs(5),
    )?;
    thread::spawn(move || api::serve(&listener, &store, None));
    Ok(endpoint)
}

fn answer(endpoint: &Endpoint, target: &str) -> Checked<(u16, Value)> {
    let (status, bytes) = endpoint.get(target)?;
    Ok((status, serde_json::from_slice(&bytes)?))
}

fn summary(endpoint: &Endpoint, count: u64, gas: u64, height: u64) -> Checked<()> {
    assert_eq!(
        answer(endpoint, "/v1/chains/1/summary")?,
        (
            200,
            json!({"id":1,"tx_count":count,"gas_used":gas,"last_height":height})
        )
    );
    Ok(())
}

fn shared_transfer_stability(
    store: &Store,
    boundary: Option<u64>,
    chains: &[(&str, usize)],
) -> Checked<()> {
    assert_eq!(
        store
            .cursor(CHAIN)?
            .ok_or("shared cursor missing")?
            .finalized_boundary,
        boundary
    );
    for &(chain, count) in chains {
        let rows = store.transfers(chain)?;
        assert_eq!(rows.len(), count, "{chain}");
        for row in &rows {
            assert_eq!(row["chain"], chain);
            let height = row["height_or_seq"]
                .as_str()
                .ok_or("height missing")?
                .parse::<u64>()?;
            let final_row = boundary.is_some_and(|boundary| height <= boundary);
            assert_eq!(row["final"], final_row);
            assert_eq!(row["final_basis"], "block_commit");
            assert_eq!(
                row["stability"],
                json!({
                    "level": if final_row { "depth_stable" } else { "reversible" },
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
            let account = row["account"].as_str().ok_or("account missing")?;
            let history = store.history(account, None, 100, None)?;
            assert_eq!(
                history.items.iter().find(|item| item["id"] == row["id"]),
                Some(row)
            );
        }
        if let Some(id) = chain.strip_prefix("pax:") {
            assert!(store.cursor(chain)?.is_none());
            assert_eq!(
                store.chain_page(id.parse()?, false, None, 100, None)?.items,
                rows
            );
        }
    }
    Ok(())
}

#[test]
fn chains_scope_virtual_transfers_follow_the_shared_cursor() -> Checked<()> {
    let fixture: Value = serde_json::from_str(include_str!("fixtures/chains_blocks.json"))?;
    for depth in [0, 1] {
        let database = Database(std::path::PathBuf::from(env!("CARGO_TARGET_TMPDIR")).join(
            format!("chains-stability-{}-{depth}.sqlite", std::process::id()),
        ));
        let store = Store::open(&database.0)?;
        for height in 42..=44 {
            let unit = fixture_block(&store, &fixture, height)?;
            assert_eq!(unit.chain, CHAIN);
            store.commit(&unit, depth)?;
        }
        drop(store);
        let store = Store::open(&database.0)?;
        shared_transfer_stability(
            &store,
            Some(44 - depth),
            &[(CHAIN, 2), ("pax:1", 4), ("pax:2", 2)],
        )?;
    }

    let store = Arc::new(Store::open_in_memory()?);
    let endpoint = endpoint(Arc::clone(&store))?;
    store.commit(&block(&store, 42)?, 1)?;
    shared_transfer_stability(&store, None, &[(CHAIN, 2), ("pax:1", 4)])?;
    store.commit(&block(&store, 43)?, 1)?;
    shared_transfer_stability(&store, Some(42), &[(CHAIN, 4), ("pax:1", 4)])?;
    let (status, before) = answer(&endpoint, "/v1/chains/1/transfers")?;
    assert_eq!(status, 200);
    assert_eq!(before["items"], json!(store.transfers("pax:1")?));
    store.rollback(CHAIN, Some(42))?;
    shared_transfer_stability(&store, None, &[(CHAIN, 2), ("pax:1", 4)])?;
    let (status, after) = answer(&endpoint, "/v1/chains/1/transfers")?;
    assert_eq!(status, 200);
    assert_eq!(after["items"], json!(store.transfers("pax:1")?));
    Ok(())
}

fn assert_namespace_rows(database: &Database, chain_hashes: &[&str]) -> Checked<()> {
    let sql = rusqlite::Connection::open(&database.0)?;
    for table in ["transfers", "events"] {
        let mut statement = sql.prepare(&format!("SELECT chain, tx_id FROM {table}"))?;
        let rows = statement.query_map([], |row| {
            Ok((row.get::<_, String>(0)?, row.get::<_, String>(1)?))
        })?;
        let mut namespaces = std::collections::BTreeSet::new();
        for row in rows {
            let (chain, tx_id) = row?;
            assert_eq!(
                chain,
                if chain_hashes.contains(&tx_id.as_str()) {
                    "pax:1"
                } else {
                    CHAIN
                }
            );
            namespaces.insert(chain);
        }
        assert_eq!(
            namespaces,
            [CHAIN.to_owned(), "pax:1".to_owned()].into_iter().collect()
        );
    }
    let activity: (String, i64, i64, i64) = sql.query_row(
        "SELECT chain, tx_count, gas_used, last_height FROM chain_activity",
        [],
        |row| Ok((row.get(0)?, row.get(1)?, row.get(2)?, row.get(3)?)),
    )?;
    assert_eq!(activity, ("pax:1".to_owned(), 2, 42005, 42));
    drop(sql);
    Ok(())
}

fn assert_scoped_feeds(endpoint: &Endpoint, store: &Store) -> Checked<()> {
    for (feed, expected) in [
        ("transfers", store.transfers("pax:1")?),
        ("events", store.events("pax:1")?),
    ] {
        let target = format!("/v1/chains/1/{feed}");
        let (status, full) = answer(endpoint, &target)?;
        assert_eq!(status, 200);
        assert_eq!(full["version"], 1);
        assert_eq!(full["indexedHeight"], 42);
        assert!(full.get("chainHeight").is_some());
        assert!(full.get("freshness").is_some());
        assert!(full["next_cursor"].is_null());
        let all = full["items"].as_array().ok_or("items missing")?;
        assert_eq!(all.len(), expected.len());
        assert!(all.iter().all(|row| row["chain"] == "pax:1"));
        let documents: Vec<_> = all
            .iter()
            .cloned()
            .map(|mut row| {
                if feed == "events" {
                    let fields = row
                        .as_object_mut()
                        .unwrap_or_else(|| panic!("event is an object"));
                    assert!(fields.remove("id").is_some());
                    assert!(fields.remove("chain").is_some());
                }
                row
            })
            .collect();
        assert_eq!(documents, expected);
        let mut paged = Vec::new();
        let mut cursor = None;
        for _ in 0..=expected.len() {
            let query = cursor.as_ref().map_or("?limit=1".to_owned(), |cursor| {
                format!("?limit=1&cursor={cursor}")
            });
            let (status, page) = answer(endpoint, &format!("{target}{query}"))?;
            assert_eq!(status, 200);
            let items = page["items"].as_array().ok_or("page items missing")?;
            assert_eq!(items.len(), 1);
            assert_eq!(items[0]["chain"], "pax:1");
            paged.extend(items.iter().cloned());
            cursor = page["next_cursor"].as_str().map(str::to_owned);
            if cursor.is_none() {
                break;
            }
        }
        assert!(cursor.is_none(), "pagination must terminate");
        assert_eq!(&paged, all);
    }
    Ok(())
}

#[test]
fn chains_scope_namespaces_rows_and_serves_them() -> Checked<()> {
    let database = Database(
        std::path::PathBuf::from(env!("CARGO_TARGET_TMPDIR"))
            .join(format!("chains-scope-{}.sqlite", std::process::id())),
    );
    let store = Store::open(&database.0)?;
    let unit = block(&store, 42)?;
    store.commit(&unit, 100)?;
    assert_eq!(store.chain(1)?.ok_or("chain missing")?.status, "paused");
    let chain_hashes: Vec<_> = unit
        .events
        .iter()
        .filter(|event| event.source == "chain-transaction")
        .map(|event| event.tx_id.as_str())
        .collect();
    assert_eq!(chain_hashes.len(), 2);
    assert_eq!(store.transfers(CHAIN)?.len(), 2);
    assert_eq!(store.events(CHAIN)?.len(), 2);
    for rows in [store.transfers(CHAIN)?, store.events(CHAIN)?] {
        assert!(rows
            .iter()
            .all(|row| !chain_hashes.contains(&row["tx_id"].as_str().unwrap_or_default())));
    }
    for (rows, count) in [(store.transfers("pax:1")?, 4), (store.events("pax:1")?, 2)] {
        assert_eq!(rows.len(), count);
        assert!(rows
            .iter()
            .all(|row| chain_hashes.contains(&row["tx_id"].as_str().unwrap_or_default())));
        for hash in &chain_hashes {
            assert!(rows.iter().any(|row| row["tx_id"] == *hash));
        }
    }
    assert_namespace_rows(&database, &chain_hashes)?;
    drop(store);
    let store = Arc::new(Store::open(&database.0)?);
    let endpoint = endpoint(Arc::clone(&store))?;
    summary(&endpoint, 2, 42005, 42)?;
    for feed in ["summary", "transfers", "events"] {
        assert_eq!(
            answer(&endpoint, &format!("/v1/chains/2/{feed}"))?,
            (404, json!({"error":"unknown_chain"}))
        );
    }
    assert_scoped_feeds(&endpoint, &store)?;
    let next = block(&store, 43)?;
    store.commit(&next, 100)?;
    summary(&endpoint, 3, 63008, 43)?;
    assert_eq!(store.transfers(CHAIN)?.len(), 4);
    assert_eq!(store.events(CHAIN)?.len(), 3);
    assert!(store.transfers("pax:99")?.is_empty());
    assert!(store.events("pax:99")?.is_empty());
    assert_eq!(store.transfers("pax:1")?.len(), 4);
    assert_eq!(store.events("pax:1")?.len(), 2);
    store.rollback(CHAIN, Some(42))?;
    summary(&endpoint, 2, 42005, 42)?;
    assert_eq!(store.transfers(CHAIN)?.len(), 2);
    assert_eq!(store.events(CHAIN)?.len(), 2);
    store.commit(&next, 100)?;
    summary(&endpoint, 3, 63008, 43)?;
    store.rollback(CHAIN, None)?;
    for chain in [CHAIN, "pax:1", "pax:99"] {
        assert!(store.transfers(chain)?.is_empty());
        assert!(store.events(chain)?.is_empty());
    }
    assert_eq!(
        answer(&endpoint, "/v1/chains/1/summary")?,
        (404, json!({"error":"unknown_chain"}))
    );
    let sql = rusqlite::Connection::open(&database.0)?;
    assert_eq!(
        sql.query_row("SELECT COUNT(*) FROM chain_activity", [], |row| row
            .get::<_, i64>(0))?,
        0
    );
    Ok(())
}
