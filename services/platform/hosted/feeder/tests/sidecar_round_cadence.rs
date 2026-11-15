#[path = "../src/bin/price-sidecar/main.rs"]
pub mod sidecar;
pub use sidecar::{health, now_ms, round, Config, MarketConfig};

use std::collections::BTreeSet;
use std::sync::Mutex;

use base64::engine::general_purpose::STANDARD;
use base64::Engine as _;
use serde_json::Value;
use zeroize::Zeroizing;

#[path = "support/submission_peer.rs"]
mod submission_peer;
use submission_peer::*;

fn two_markets(peer: &Peer) -> Config {
    let mut config = config(peer);
    config.markets.push(MarketConfig {
        market_id: 1,
        symbol: "ETH".to_owned(),
    });
    config
}

fn pending(dir: &StateDir) -> Vec<Value> {
    dir.progress()["pending"]
        .as_array()
        .cloned()
        .unwrap_or_default()
}

#[test]
fn round_cadence_open_fates_accumulate_until_the_bound() -> TestResult {
    let peer = Peer::new();
    peer.disable_transaction_lookup();
    peer.serve_fresh_quotes();
    let config = two_markets(&peer);
    let dir = StateDir::new();
    let metrics = Mutex::new(health::Metrics::default());
    let mut sidecar = round::start(&config, &peer.rpc(), Zeroizing::new([1; 32]), &dir.0)?;

    for _ in 0..3 {
        round::run_round(
            &config,
            &agent(),
            "fixture-credential",
            &peer.rpc(),
            &mut sidecar,
            &metrics,
            &dir.0,
        )?;
    }

    let entries = pending(&dir);
    assert_eq!(entries.len(), 3);
    assert_eq!(peer.state.lock().unwrap().txs.len(), 3);
    assert_eq!(
        peer.state.lock().unwrap().ticker_requests,
        3 * config.markets.len()
    );
    assert_unconfirmed(&metrics);
    assert!(health::lock(&metrics)
        .metrics_text(now_ms())
        .contains("price_sidecar_rounds_total 3\n"));
    for (index, entry) in entries.iter().enumerate() {
        let journal = &entry["journal"];
        assert_eq!(journal["next_account_sequence"], 4 + index as u64);
        assert_eq!(
            journal["markets"]["BTC"]["observation_sequence"],
            1 + index as u64
        );
        assert_eq!(
            journal["markets"]["ETH"]["observation_sequence"],
            1 + index as u64
        );
    }
    Ok(())
}

#[test]
fn round_cadence_committed_oldest_keeps_broadcasting() -> TestResult {
    let peer = Peer::new();
    peer.disable_transaction_lookup();
    peer.serve_fresh_quotes();
    let config = two_markets(&peer);
    let dir = StateDir::new();
    let metrics = Mutex::new(health::Metrics::default());
    let mut sidecar = round::start(&config, &peer.rpc(), Zeroizing::new([1; 32]), &dir.0)?;
    for _ in 0..3 {
        round::run_round(
            &config,
            &agent(),
            "fixture-credential",
            &peer.rpc(),
            &mut sidecar,
            &metrics,
            &dir.0,
        )?;
    }
    peer.commit_submission(0, 840);

    round::run_round(
        &config,
        &agent(),
        "fixture-credential",
        &peer.rpc(),
        &mut sidecar,
        &metrics,
        &dir.0,
    )?;

    let entries = pending(&dir);
    assert_eq!(peer.state.lock().unwrap().txs.len(), 4);
    assert_eq!(entries.len(), 3);
    assert_eq!(
        entries[0]["journal"]["markets"]["BTC"]["observation_sequence"],
        2
    );
    assert_eq!(entries[2]["journal"]["next_account_sequence"], 7);
    assert_eq!(
        entries[2]["journal"]["markets"]["ETH"]["observation_sequence"],
        4
    );
    Ok(())
}

#[test]
fn round_cadence_bound_waits_for_one_commit_then_broadcasts() -> TestResult {
    let peer = Peer::new();
    peer.disable_transaction_lookup();
    peer.serve_fresh_quotes();
    let mut config = config(&peer);
    config.max_pending_submissions = 3;
    let dir = StateDir::new();
    let metrics = Mutex::new(health::Metrics::default());
    let mut sidecar = round::start(&config, &peer.rpc(), Zeroizing::new([1; 32]), &dir.0)?;
    for _ in 0..3 {
        round::run_round(
            &config,
            &agent(),
            "fixture-credential",
            &peer.rpc(),
            &mut sidecar,
            &metrics,
            &dir.0,
        )?;
    }
    let ticker_requests = peer.state.lock().unwrap().ticker_requests;
    round::run_round(
        &config,
        &agent(),
        "fixture-credential",
        &peer.rpc(),
        &mut sidecar,
        &metrics,
        &dir.0,
    )?;
    assert_eq!(peer.state.lock().unwrap().ticker_requests, ticker_requests);
    assert_eq!(peer.state.lock().unwrap().txs.len(), 3);
    assert_eq!(pending(&dir).len(), 3);

    peer.commit_submission(0, 840);
    round::run_round(
        &config,
        &agent(),
        "fixture-credential",
        &peer.rpc(),
        &mut sidecar,
        &metrics,
        &dir.0,
    )?;
    assert_eq!(peer.state.lock().unwrap().txs.len(), 4);
    assert_eq!(pending(&dir).len(), 3);
    Ok(())
}

#[test]
fn round_cadence_eviction_resets_to_committed_sequences() -> TestResult {
    let peer = Peer::new();
    peer.disable_transaction_lookup();
    peer.serve_fresh_quotes();
    let config = config(&peer);
    let dir = StateDir::new();
    let metrics = Mutex::new(health::Metrics::default());
    let mut sidecar = round::start(&config, &peer.rpc(), Zeroizing::new([1; 32]), &dir.0)?;
    for _ in 0..3 {
        round::run_round(
            &config,
            &agent(),
            "fixture-credential",
            &peer.rpc(),
            &mut sidecar,
            &metrics,
            &dir.0,
        )?;
    }
    let proposed = pending(&dir)[0]["proposed_height"]
        .as_u64()
        .ok_or("missing proposed height")?;
    peer.state.lock().unwrap().committed_height =
        Some(proposed + config.pending_submission_block_limit);

    round::run_round(
        &config,
        &agent(),
        "fixture-credential",
        &peer.rpc(),
        &mut sidecar,
        &metrics,
        &dir.0,
    )?;
    let entries = pending(&dir);
    assert_eq!(peer.state.lock().unwrap().txs.len(), 4);
    assert_eq!(entries.len(), 1);
    assert_eq!(entries[0]["journal"]["next_account_sequence"], 4);
    assert_eq!(
        entries[0]["journal"]["markets"]["BTC"]["observation_sequence"],
        1
    );
    assert!(entries[0]["journal"]["markets"]["BTC"]["activity_id"]
        .as_str()
        .is_some_and(|activity_id| !activity_id.is_empty()));
    Ok(())
}

#[test]
fn round_cadence_restart_decides_pending_entries_then_submits() -> TestResult {
    let peer = Peer::new();
    peer.disable_transaction_lookup();
    peer.serve_fresh_quotes();
    let config = two_markets(&peer);
    let dir = StateDir::new();
    let metrics = Mutex::new(health::Metrics::default());
    let mut sidecar = round::start(&config, &peer.rpc(), Zeroizing::new([1; 32]), &dir.0)?;
    for _ in 0..3 {
        round::run_round(
            &config,
            &agent(),
            "fixture-credential",
            &peer.rpc(),
            &mut sidecar,
            &metrics,
            &dir.0,
        )?;
    }
    let mut committed = pending(&dir);
    drop(sidecar);
    peer.commit_submission(0, 840);
    peer.commit_submission(1, 841);
    peer.commit_submission(2, 842);

    let mut reopened = round::start(&config, &peer.rpc(), Zeroizing::new([1; 32]), &dir.0)?;
    round::run_round(
        &config,
        &agent(),
        "fixture-credential",
        &peer.rpc(),
        &mut reopened,
        &metrics,
        &dir.0,
    )?;
    let entries = pending(&dir);
    assert_eq!(peer.state.lock().unwrap().txs.len(), 4);
    assert_eq!(entries.len(), 1);
    assert_eq!(entries[0]["journal"]["next_account_sequence"], 7);
    assert_eq!(
        entries[0]["journal"]["markets"]["BTC"]["observation_sequence"],
        4
    );
    assert_eq!(
        entries[0]["journal"]["markets"]["ETH"]["observation_sequence"],
        4
    );
    committed.extend(entries);
    peer.commit_submission(3, 843);
    let state = peer.state.lock().unwrap();
    let mut accounts = BTreeSet::new();
    let mut observations = BTreeSet::new();
    for (index, entry) in committed.iter().enumerate() {
        assert!(matches!(
            state.tx_fates[index],
            Fate::Committed { code: 0, .. }
        ));
        assert_eq!(entry["tx"], STANDARD.encode(&state.txs[index]));
        assert!(accounts.insert(entry["journal"]["next_account_sequence"].as_u64().unwrap() - 1));
        for symbol in ["BTC", "ETH"] {
            assert!(observations.insert((
                symbol,
                entry["journal"]["markets"][symbol]["observation_sequence"]
                    .as_u64()
                    .unwrap(),
            )));
        }
    }
    assert_eq!(accounts, BTreeSet::from([3, 4, 5, 6]));
    assert_eq!(observations.len(), 8);
    Ok(())
}
