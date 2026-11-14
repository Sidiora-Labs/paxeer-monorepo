#[path = "../src/bin/price-sidecar/main.rs"]
pub mod sidecar;
pub use sidecar::{health, now_ms, round, Config, MarketConfig};

use std::sync::Mutex;

use layerx_oracle_feeder::Journal;
use serde_json::{json, Value};

#[path = "support/submission_peer.rs"]
mod submission_peer;
use submission_peer::*;

fn disable_lookup(peer: &Peer) {
    peer.state.lock().unwrap().lookup_error = Some(json!({
        "code": -32603,
        "message": "Internal error",
        "data": "transaction querying is disabled due to no kvEventSink"
    }));
}

fn two_markets(peer: &Peer) -> Config {
    let mut config = config(peer);
    config.markets.push(MarketConfig {
        market_id: 1,
        symbol: "ETH".to_owned(),
    });
    config
}

fn assert_reads(peer: &Peer, height: u64, markets: &[u64], reads: usize) {
    let state = peer.state.lock().unwrap();
    assert_eq!(state.account_reads, vec![height; reads]);
    let expected: Vec<_> = (0..reads)
        .flat_map(|_| markets.iter().map(|market| (*market, height)))
        .collect();
    assert_eq!(state.observation_reads, expected);
    assert!(state.mempool_pages.is_empty());
}

fn clear_reads(peer: &Peer) {
    let mut state = peer.state.lock().unwrap();
    state.account_reads.clear();
    state.observation_reads.clear();
}

#[test]
fn committed_state_committed_submission_recovers_after_restart() -> TestResult {
    let peer = Peer::new();
    let config = two_markets(&peer);
    let dir = StateDir::new();
    let metrics = Mutex::new(health::Metrics::default());
    let mut sidecar = start(&config, &peer, &dir);
    let submission = submit(&config, &peer, &dir, &mut sidecar, &metrics);
    let pending = dir.progress()["pending"].clone();
    assert_unconfirmed(&metrics);
    drop(sidecar);
    disable_lookup(&peer);
    {
        let mut state = peer.state.lock().unwrap();
        state.account_sequence = 4;
        state.observation_sequence = 1;
        state.committed_height = Some(840);
        state.advance_height_on_account_read = true;
    }
    clear_reads(&peer);
    let metrics = Mutex::new(health::Metrics::default());
    let mut sidecar = start(&config, &peer, &dir);
    assert!(sidecar.reconcile(&config, &peer.rpc(), &metrics, &dir.0)?);
    {
        let state = peer.state.lock().unwrap();
        assert_eq!(state.account_reads, [840, 841]);
        assert_eq!(
            state.observation_reads,
            [(0, 840), (1, 840), (0, 841), (1, 841)]
        );
        assert!(state.mempool_pages.is_empty());
    }
    let journal = Journal::load(&dir.0, 0)?;
    assert_eq!(journal.next_account_sequence, submission.sequence + 1);
    for (index, symbol) in ["BTC", "ETH"].iter().enumerate() {
        assert_eq!(
            journal.markets[*symbol].sequence,
            submission.observations[index].sequence
        );
        assert_eq!(
            journal.markets[*symbol].price,
            submission.observations[index].index_price
        );
        assert_eq!(
            journal.markets[*symbol].activity_id,
            pending["hash"].as_str().unwrap()
        );
    }
    let progress = dir.progress();
    assert!(progress["pending"].is_null());
    assert_eq!(progress["confirmed_height"], 840);
    assert!(health::lock(&metrics).is_ready(now_ms(), 1000));
    assert!(health::lock(&metrics)
        .metrics_text(now_ms())
        .contains("result=\"committed\"} 1\n"));
    drop(sidecar);
    let mut sidecar = start(&config, &peer, &dir);
    let metrics = Mutex::new(health::Metrics::default());
    assert!(sidecar.reconcile(&config, &peer.rpc(), &metrics, &dir.0)?);
    assert_eq!(dir.progress(), progress);
    assert_eq!(Journal::load(&dir.0, 0)?, journal);
    assert!(health::lock(&metrics)
        .metrics_text(now_ms())
        .contains("price_sidecar_confirmed_height 840\n"));
    assert_eq!(peer.state.lock().unwrap().txs.len(), 1);
    Ok(())
}

#[test]
fn committed_state_failed_execution_resynchronises_every_market() -> TestResult {
    let peer = Peer::new();
    let config = two_markets(&peer);
    let dir = StateDir::new();
    let metrics = Mutex::new(health::Metrics::default());
    let mut sidecar = start(&config, &peer, &dir);
    submit(&config, &peer, &dir, &mut sidecar, &metrics);
    drop(sidecar);
    disable_lookup(&peer);
    {
        let mut state = peer.state.lock().unwrap();
        state.account_sequence = 4;
        state.observation_sequence = 0;
        state.observation_sequences.insert(0, 1);
        state.committed_height = Some(840);
    }
    clear_reads(&peer);
    let mut sidecar = start(&config, &peer, &dir);
    assert!(sidecar.reconcile(&config, &peer.rpc(), &metrics, &dir.0)?);
    assert_reads(&peer, 840, &[0, 1], 2);
    assert_unconfirmed(&metrics);
    assert!(dir.progress()["pending"].is_null());
    let journal = Journal::load(&dir.0, 0)?;
    assert_eq!(journal.next_account_sequence, 4);
    assert_eq!(journal.markets["BTC"].sequence, 1);
    assert_eq!(journal.markets["ETH"].sequence, 0);
    assert!(journal
        .markets
        .values()
        .all(|record| record.activity_id.is_empty()));
    assert!(health::lock(&metrics)
        .metrics_text(now_ms())
        .contains("result=\"failed\"} 1\n"));
    let retry = submit(&config, &peer, &dir, &mut sidecar, &metrics);
    assert_eq!(retry.sequence, 4);
    assert_eq!(retry.observations[0].sequence, 2);
    assert_eq!(retry.observations[1].sequence, 1);
    Ok(())
}

#[test]
fn committed_state_unused_sequence_stays_pending_within_bound() -> TestResult {
    let peer = Peer::new();
    let config = config(&peer);
    let dir = StateDir::new();
    let metrics = Mutex::new(health::Metrics::default());
    let mut sidecar = start(&config, &peer, &dir);
    submit(&config, &peer, &dir, &mut sidecar, &metrics);
    let pending = dir.progress();
    let journal = Journal::load(&dir.0, 0)?;
    let proposed = pending["pending"]["proposed_height"].as_u64().unwrap();
    drop(sidecar);
    disable_lookup(&peer);
    let mut sidecar = start(&config, &peer, &dir);
    for height in [
        proposed - 1,
        proposed,
        proposed + round::PENDING_SUBMISSION_BLOCK_LIMIT - 1,
    ] {
        peer.state.lock().unwrap().committed_height = Some(height);
        clear_reads(&peer);
        assert!(!sidecar.reconcile(&config, &peer.rpc(), &metrics, &dir.0)?);
        assert_reads(&peer, height, &[0], 1);
        assert_eq!(dir.progress(), pending);
        assert_eq!(Journal::load(&dir.0, 0)?, journal);
        assert_unconfirmed(&metrics);
    }
    let requests = peer.state.lock().unwrap().ticker_requests;
    round::run_round(
        &config,
        &agent(),
        "fixture-credential",
        &peer.rpc(),
        &mut sidecar,
        &metrics,
        &dir.0,
    )?;
    let state = peer.state.lock().unwrap();
    assert_eq!(state.txs.len(), 1);
    assert_eq!(state.ticker_requests, requests);
    Ok(())
}

#[test]
fn committed_state_unused_sequence_is_evicted_at_bound() -> TestResult {
    for elapsed in [
        round::PENDING_SUBMISSION_BLOCK_LIMIT,
        round::PENDING_SUBMISSION_BLOCK_LIMIT + 1,
    ] {
        let peer = Peer::new();
        let config = config(&peer);
        let dir = StateDir::new();
        let metrics = Mutex::new(health::Metrics::default());
        let mut sidecar = start(&config, &peer, &dir);
        let submission = submit(&config, &peer, &dir, &mut sidecar, &metrics);
        let proposed = dir.progress()["pending"]["proposed_height"]
            .as_u64()
            .unwrap();
        drop(sidecar);
        disable_lookup(&peer);
        let height = proposed + elapsed;
        peer.state.lock().unwrap().committed_height = Some(height);
        clear_reads(&peer);
        let mut sidecar = start(&config, &peer, &dir);
        assert!(sidecar.reconcile(&config, &peer.rpc(), &metrics, &dir.0)?);
        assert_reads(&peer, height, &[0], 2);
        assert!(dir.progress()["pending"].is_null());
        assert_unconfirmed(&metrics);
        let journal = Journal::load(&dir.0, 0)?;
        assert_eq!(journal.next_account_sequence, submission.sequence);
        assert_eq!(journal.markets["BTC"].sequence, 0);
        assert!(journal.markets["BTC"].activity_id.is_empty());
        assert!(health::lock(&metrics)
            .metrics_text(now_ms())
            .contains("result=\"evicted\"} 1\n"));
        let retry = submit(&config, &peer, &dir, &mut sidecar, &metrics);
        assert_eq!(retry.sequence, submission.sequence);
        assert_eq!(
            retry.observations[0].sequence,
            submission.observations[0].sequence
        );
    }
    Ok(())
}

#[test]
fn committed_state_served_lookup_reads_pool_pages_until_total() -> TestResult {
    let peer = Peer::new();
    let config = config(&peer);
    let dir = StateDir::new();
    let metrics = Mutex::new(health::Metrics::default());
    let mut sidecar = start(&config, &peer, &dir);
    submit(&config, &peer, &dir, &mut sidecar, &metrics);
    let pending = dir.progress();
    let journal = Journal::load(&dir.0, 0)?;
    let other_dir = StateDir::new();
    let mut other = start(&config, &peer, &other_dir);
    peer.state.lock().unwrap().account_sequence = 4;
    submit(&config, &peer, &other_dir, &mut other, &metrics);
    {
        let mut state = peer.state.lock().unwrap();
        let previous = state.txs[0].clone();
        let current = state.txs[1].clone();
        state.mempool = Some(vec![current; 201]);
        state.mempool.as_mut().unwrap()[200] = previous;
        state.fate = Fate::Evicted;
    }
    assert!(!sidecar.reconcile(&config, &peer.rpc(), &metrics, &dir.0)?);
    assert_eq!(peer.state.lock().unwrap().mempool_pages, [1, 2, 3]);
    assert_eq!(dir.progress(), pending);
    assert_eq!(Journal::load(&dir.0, 0)?, journal);
    assert_unconfirmed(&metrics);
    {
        let mut state = peer.state.lock().unwrap();
        let replacement = state.mempool.as_ref().unwrap()[0].clone();
        state.mempool.as_mut().unwrap()[200] = replacement;
        state.mempool_pages.clear();
        state.tx_lookups = 0;
    }
    assert!(sidecar.reconcile(&config, &peer.rpc(), &metrics, &dir.0)?);
    let state = peer.state.lock().unwrap();
    assert_eq!(state.mempool_pages, [1, 2, 3]);
    assert_eq!(state.tx_lookups, 2);
    assert!(dir.progress()["pending"].is_null());
    assert_eq!(Journal::load(&dir.0, 0)?.next_account_sequence, 4);
    assert!(health::lock(&metrics)
        .metrics_text(now_ms())
        .contains("result=\"evicted\"} 1\n"));
    Ok(())
}

#[test]
fn committed_state_other_lookup_errors_preserve_pending() -> TestResult {
    let peer = Peer::new();
    let config = config(&peer);
    let dir = StateDir::new();
    let metrics = Mutex::new(health::Metrics::default());
    let mut sidecar = start(&config, &peer, &dir);
    submit(&config, &peer, &dir, &mut sidecar, &metrics);
    let pending = dir.progress();
    clear_reads(&peer);
    for error in [
        json!({"code": -32602, "data": "transaction querying is disabled"}),
        json!({"code": -32603, "data": "transaction indexing is disabled"}),
        json!({"code": -32603, "message": "transaction querying is disabled"}),
        json!({"code": -32603, "data": "backend unavailable"}),
        json!({"code": -32603, "data": Value::Null}),
    ] {
        peer.state.lock().unwrap().lookup_error = Some(error);
        assert!(sidecar
            .reconcile(&config, &peer.rpc(), &metrics, &dir.0)
            .is_err());
        assert_eq!(dir.progress(), pending);
        assert_unconfirmed(&metrics);
    }
    assert_reads(&peer, 830, &[0], 0);
    Ok(())
}
