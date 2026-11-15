#[path = "../src/bin/price-sidecar/main.rs"]
pub mod sidecar;
pub use sidecar::{health, now_ms, round, Config, MarketConfig};

use std::collections::BTreeMap;
use std::sync::Mutex;

use layerx_oracle_feeder::Journal;
use serde_json::{json, Value};

#[path = "support/submission_peer.rs"]
mod submission_peer;
use submission_peer::*;

fn eighteen_markets(peer: &Peer) -> Config {
    let mut config = config(peer);
    config.markets = [
        "BTC", "ETH", "SOL", "AVAX", "LINK", "TSLA", "NVDA", "NAS100", "XAU", "SPX500", "GOOGL",
        "PAX", "SID", "HYPE", "XRP", "ASTER", "TRUMP", "BNB",
    ]
    .into_iter()
    .enumerate()
    .map(|(market_id, symbol)| MarketConfig {
        market_id: market_id as u64,
        symbol: symbol.to_owned(),
    })
    .collect();
    config
}

fn committed_submission(
    config: &Config,
    peer: &Peer,
    dir: &StateDir,
    metrics: &Mutex<health::Metrics>,
) -> round::Sidecar {
    let mut sidecar = start(config, peer, dir);
    submit(config, peer, dir, &mut sidecar, metrics);
    peer.commit_submission(0, 840);
    peer.disable_transaction_lookup();
    let mut state = peer.state.lock().unwrap();
    state.account_reads.clear();
    state.observation_reads.clear();
    state.advance_height_on_account_read = true;
    sidecar
}

fn assert_committed_reads(config: &Config, peer: &Peer, dir: &StateDir) {
    let state = peer.state.lock().unwrap();
    assert_eq!(state.account_reads, [840]);
    assert_eq!(state.txs.len(), 1);
    assert!(state.mempool_pages.is_empty());
    let mut reads = BTreeMap::new();
    for &(market, height) in &state.observation_reads {
        assert_eq!(height, 840);
        *reads.entry(market).or_insert(0usize) += 1;
    }
    let mut busy = BTreeMap::new();
    for &(market, height) in &state.observation_busy_reads {
        assert_eq!(height, 840);
        *busy.entry(market).or_insert(0usize) += 1;
    }
    assert_eq!(reads.len(), config.markets.len());
    for market in &config.markets {
        assert_eq!(
            reads[&market.market_id],
            1 + busy.get(&market.market_id).copied().unwrap_or(0)
        );
    }
    let progress = dir.progress();
    assert!(progress["pending"].as_array().unwrap().is_empty());
    assert_eq!(progress["confirmed_height"], 840);
    let journal = Journal::load(&dir.0, 0).unwrap();
    assert_eq!(journal.next_account_sequence, 4);
    for market in &config.markets {
        assert_eq!(journal.markets[&market.symbol].sequence, 1);
    }
}

#[test]
fn read_bound_default_commits_eighteen_markets_with_six_reads() -> TestResult {
    let peer = Peer::new();
    let config = eighteen_markets(&peer);
    assert_eq!(config.max_committed_reads_in_flight, 6);
    let dir = StateDir::new();
    let metrics = Mutex::new(health::Metrics::default());
    let mut sidecar = committed_submission(&config, &peer, &dir, &metrics);
    peer.hold_observation_reads_until(6);

    let report = sidecar.reconcile(&config, &peer.rpc(), &metrics, &dir.0)?;

    assert!(report.could_read);
    assert!(!report.fate_open);
    assert!(peer.observation_bound_passed());
    assert_eq!(peer.highest_observation_reads_in_flight(), 6);
    assert!(peer.state.lock().unwrap().observation_busy_reads.is_empty());
    assert_committed_reads(&config, &peer, &dir);
    Ok(())
}

#[test]
fn read_bound_busy_reads_retry_without_repeating_successful_reads() -> TestResult {
    let peer = Peer::new();
    let mut config = eighteen_markets(&peer);
    config.retry_backoff_ms = 10;
    config.retry_max_attempts = 6;
    let dir = StateDir::new();
    let metrics = Mutex::new(health::Metrics::default());
    let mut sidecar = committed_submission(&config, &peer, &dir, &metrics);
    peer.hold_observation_reads_until(6);
    peer.limit_observation_reads(3);

    let report = sidecar.reconcile(&config, &peer.rpc(), &metrics, &dir.0)?;

    assert!(report.could_read);
    assert!(!report.fate_open);
    assert!(peer.observation_bound_passed());
    assert_eq!(peer.highest_observation_reads_in_flight(), 6);
    assert!(!peer.state.lock().unwrap().observation_busy_reads.is_empty());
    assert_committed_reads(&config, &peer, &dir);
    Ok(())
}

#[test]
fn read_bound_busy_exhaustion_preserves_the_open_submission() -> TestResult {
    let peer = Peer::new();
    let mut config = eighteen_markets(&peer);
    config.retry_max_attempts = 3;
    let dir = StateDir::new();
    let metrics = Mutex::new(health::Metrics::default());
    let mut sidecar = committed_submission(&config, &peer, &dir, &metrics);
    let progress = dir.progress();
    let journal = Journal::load(&dir.0, 0)?;
    peer.hold_observation_reads_until(6);
    peer.limit_observation_reads(0);

    let error = sidecar
        .reconcile(&config, &peer.rpc(), &metrics, &dir.0)
        .expect_err("busy observation reads exhaust their retry plan");

    assert!(error.contains("rejected due to rate limit"), "{error}");
    assert_eq!(sidecar.pending_count(), 1);
    assert_eq!(dir.progress(), progress);
    assert_eq!(Journal::load(&dir.0, 0)?, journal);
    assert_unconfirmed(&metrics);
    assert_eq!(peer.highest_observation_reads_in_flight(), 6);
    {
        let state = peer.state.lock().unwrap();
        assert_eq!(state.account_reads, [840]);
        assert_eq!(
            state.observation_reads.len(),
            config.markets.len() * config.retry_max_attempts as usize
        );
        assert_eq!(state.observation_reads, state.observation_busy_reads);
        for market in &config.markets {
            assert_eq!(
                state
                    .observation_reads
                    .iter()
                    .filter(|&&(id, height)| id == market.market_id && height == 840)
                    .count(),
                config.retry_max_attempts as usize
            );
        }
    }
    drop(sidecar);
    peer.limit_observation_reads(6);
    peer.state.lock().unwrap().advance_height_on_account_read = false;
    let mut sidecar = start(&config, &peer, &dir);
    assert_eq!(sidecar.pending_count(), 1);
    let report = sidecar.reconcile(&config, &peer.rpc(), &metrics, &dir.0)?;
    assert!(report.could_read);
    assert!(!report.fate_open);
    assert!(dir.progress()["pending"].as_array().unwrap().is_empty());
    Ok(())
}

#[test]
fn read_bound_other_errors_end_reconciliation_without_retry() -> TestResult {
    let peer = Peer::new();
    let config = eighteen_markets(&peer);
    let dir = StateDir::new();
    let metrics = Mutex::new(health::Metrics::default());
    let mut sidecar = committed_submission(&config, &peer, &dir, &metrics);
    let progress = dir.progress();
    peer.hold_observation_reads_until(6);
    peer.state.lock().unwrap().observation_error = Some(json!({
        "code": -32000, "message": "observation unavailable"
    }));

    let error = sidecar
        .reconcile(&config, &peer.rpc(), &metrics, &dir.0)
        .unwrap_err();

    assert!(error.contains("observation unavailable"), "{error}");
    assert_eq!(dir.progress(), progress);
    assert_eq!(sidecar.pending_count(), 1);
    let state = peer.state.lock().unwrap();
    assert_eq!(state.account_reads, [840]);
    assert_eq!(state.observation_reads.len(), config.markets.len());
    for market in &config.markets {
        assert_eq!(
            state
                .observation_reads
                .iter()
                .filter(|&&(id, _)| id == market.market_id)
                .count(),
            1
        );
    }
    Ok(())
}

#[test]
fn read_bound_configured_limit_keeps_sequences_with_their_markets() -> TestResult {
    let peer = Peer::new();
    let mut config = eighteen_markets(&peer);
    config.max_committed_reads_in_flight = 3;
    peer.hold_observation_reads_until(3);
    let dir = StateDir::new();
    let mut sidecar = start(&config, &peer, &dir);
    {
        let mut state = peer.state.lock().unwrap();
        state.account_reads.clear();
        state.observation_reads.clear();
        for market in &config.markets {
            state
                .observation_sequences
                .insert(market.market_id, market.market_id + 7);
        }
    }
    let metrics = Mutex::new(health::Metrics::default());

    let report = sidecar.reconcile(&config, &peer.rpc(), &metrics, &dir.0)?;

    assert!(report.could_read);
    assert!(!report.fate_open);
    assert!(peer.observation_bound_passed());
    assert_eq!(peer.highest_observation_reads_in_flight(), 3);
    assert_eq!(
        peer.state.lock().unwrap().observation_reads.len(),
        config.markets.len()
    );
    let journal = Journal::load(&dir.0, 0)?;
    for market in &config.markets {
        assert_eq!(
            journal.markets[&market.symbol].sequence,
            market.market_id + 7
        );
    }
    Ok(())
}

fn config_document() -> Value {
    json!({
        "chain_id": "test-1",
        "validator": "22".repeat(20),
        "feed_account": FEED,
        "gas_limit": 200000,
        "retry_backoff_ms": 1,
        "retry_max_attempts": 3,
        "chainflow": {
            "base_url": "https://feed.example",
            "api_key_env": "PRICE_FEED_REST_API_KEY",
            "timeout_ms": 2000,
            "max_quote_age_ms": 10000
        },
        "markets": [{ "market_id": 0, "symbol": "BTC" }]
    })
}

#[test]
fn read_bound_config_defaults_to_six_and_accepts_an_explicit_bound() -> TestResult {
    let dir = StateDir::new();
    let path = dir.0.join("config.json");
    let mut document = config_document();
    std::fs::write(&path, document.to_string()).unwrap();
    assert_eq!(Config::load(&path)?.max_committed_reads_in_flight, 6);
    document["max_committed_reads_in_flight"] = json!(3);
    std::fs::write(&path, document.to_string()).unwrap();
    assert_eq!(Config::load(&path)?.max_committed_reads_in_flight, 3);
    Ok(())
}

#[test]
fn read_bound_config_refuses_zero() {
    let dir = StateDir::new();
    let path = dir.0.join("config.json");
    let mut document = config_document();
    document["max_committed_reads_in_flight"] = json!(0);
    std::fs::write(&path, document.to_string()).unwrap();
    assert_eq!(
        Config::load(&path),
        Err("config field max_committed_reads_in_flight is zero".to_owned())
    );
}
