#[path = "../src/bin/price-sidecar/main.rs"]
pub mod sidecar;
pub use sidecar::{health, now_ms, round, Config, MarketConfig};

use std::sync::Mutex;

use base64::engine::general_purpose::STANDARD;
use base64::Engine as _;
use layerx_oracle_feeder::Journal;

#[path = "support/submission_peer.rs"]
mod submission_peer;
use submission_peer::*;

fn bound_four(peer: &Peer) -> Config {
    peer.serve_fresh_quotes();
    let mut config = config(peer);
    config.max_pending_submissions = 4;
    config.chainflow.max_quote_age_ms = u64::MAX;
    config.markets.push(MarketConfig {
        market_id: 1,
        symbol: "ETH".to_owned(),
    });
    config
}

fn run(
    config: &Config,
    peer: &Peer,
    dir: &StateDir,
    sidecar: &mut round::Sidecar,
    metrics: &Mutex<health::Metrics>,
) -> TestResult {
    round::run_round(
        config,
        &agent(),
        "fixture-credential",
        &peer.rpc(),
        sidecar,
        metrics,
        &dir.0,
    )
}

#[test]
fn ahead_refusal_removes_entry_and_waits_without_signing_past_it() -> TestResult {
    let peer = Peer::new();
    peer.admit_only_committed_sequence();
    let config = bound_four(&peer);
    let dir = StateDir::new();
    let metrics = Mutex::new(health::Metrics::default());
    let mut sidecar = start(&config, &peer, &dir);

    for _ in 0..8 {
        run(&config, &peer, &dir, &mut sidecar, &metrics)?;
        assert_eq!(sidecar.pending_count(), 1);
        assert_eq!(dir.progress()["pending"].as_array().unwrap().len(), 1);
    }
    {
        let state = peer.state.lock().unwrap();
        assert_eq!(state.txs.len(), 2);
        assert_eq!(state.ticker_requests, 4);
        assert_eq!(state.sequence_refusals, 1);
        assert_eq!(tx_sequence(&state.txs[0]), 3);
        assert_eq!(tx_sequence(&state.txs[1]), 4);
        assert_eq!(tx_observation_sequences(&state.txs[0]), [(0, 1), (1, 1)]);
        assert_eq!(tx_observation_sequences(&state.txs[1]), [(0, 2), (1, 2)]);
        assert_eq!(
            dir.progress()["pending"][0]["tx"],
            STANDARD.encode(&state.txs[0])
        );
    }
    let journal = Journal::load(&dir.0, 0)?;
    assert_eq!(journal.next_account_sequence, 3);
    for market in &config.markets {
        assert_eq!(journal.markets[&market.symbol].sequence, 0);
    }
    assert!(health::lock(&metrics)
        .metrics_text(now_ms())
        .contains("result=\"sequence_mismatch\"} 1\n"));

    peer.commit_submission(0, 840);
    run(&config, &peer, &dir, &mut sidecar, &metrics)?;
    let state = peer.state.lock().unwrap();
    assert_eq!(state.txs.len(), 3);
    assert_eq!(tx_sequence(&state.txs[2]), 4);
    assert_eq!(tx_observation_sequences(&state.txs[2]), [(0, 2), (1, 2)]);
    assert_eq!(state.sequence_refusals, 1);
    assert_eq!(sidecar.pending_count(), 1);
    Ok(())
}

#[test]
fn ahead_refusal_committed_sequence_produces_twelve_committed_submissions() -> TestResult {
    let peer = Peer::new();
    peer.admit_only_committed_sequence();
    let config = bound_four(&peer);
    let dir = StateDir::new();
    let metrics = Mutex::new(health::Metrics::default());
    let mut sidecar = start(&config, &peer, &dir);

    for index in 0..12 {
        run(&config, &peer, &dir, &mut sidecar, &metrics)?;
        assert_eq!(sidecar.pending_count(), 1);
        {
            let state = peer.state.lock().unwrap();
            assert_eq!(state.txs.len(), index + 1);
            assert_eq!(tx_sequence(&state.txs[index]), 3 + index as u64);
            assert_eq!(
                tx_observation_sequences(&state.txs[index]),
                [(0, index as u64 + 1), (1, index as u64 + 1)]
            );
        }
        let before = Journal::load(&dir.0, 0)?;
        peer.commit_submission(index, 840 + index as u64);
        assert!(
            !sidecar
                .reconcile(&config, &peer.rpc(), &metrics, &dir.0)?
                .fate_open
        );
        let after = Journal::load(&dir.0, 0)?;
        for market in &config.markets {
            assert_eq!(
                after.markets[&market.symbol].sequence,
                before.markets[&market.symbol].sequence + 1
            );
        }
        assert_eq!(sidecar.pending_count(), 0);
        assert!(dir.progress()["pending"].as_array().unwrap().is_empty());
    }
    assert_eq!(peer.state.lock().unwrap().txs.len(), 12);
    assert_eq!(peer.state.lock().unwrap().sequence_refusals, 0);
    assert!(health::lock(&metrics)
        .metrics_text(now_ms())
        .contains("result=\"committed\"} 12\n"));
    Ok(())
}

#[test]
fn ahead_refusal_admitting_ahead_sequences_fills_bound_four() -> TestResult {
    let peer = Peer::new();
    let config = bound_four(&peer);
    let dir = StateDir::new();
    let metrics = Mutex::new(health::Metrics::default());
    let mut sidecar = start(&config, &peer, &dir);
    for count in 1..=4 {
        run(&config, &peer, &dir, &mut sidecar, &metrics)?;
        assert_eq!(sidecar.pending_count(), count);
    }
    run(&config, &peer, &dir, &mut sidecar, &metrics)?;
    let state = peer.state.lock().unwrap();
    assert_eq!(state.txs.len(), 4);
    assert_eq!(state.ticker_requests, 8);
    assert_eq!(state.sequence_refusals, 0);
    assert_eq!(dir.progress()["pending"].as_array().unwrap().len(), 4);
    Ok(())
}

fn retained_sequence_refusal(log: &str) -> TestResult {
    let peer = Peer::new();
    peer.state.lock().unwrap().broadcast_refusal = Some(log.to_owned());
    let config = bound_four(&peer);
    let dir = StateDir::new();
    let metrics = Mutex::new(health::Metrics::default());
    let mut sidecar = start(&config, &peer, &dir);
    run(&config, &peer, &dir, &mut sidecar, &metrics)?;
    let state = peer.state.lock().unwrap();
    assert_eq!(state.txs.len(), 2);
    assert_eq!(state.txs[0], state.txs[1]);
    assert_eq!(state.sequence_refusals, 2);
    assert_eq!(sidecar.pending_count(), 1);
    assert_eq!(
        dir.progress()["pending"][0]["tx"],
        STANDARD.encode(&state.txs[0])
    );
    assert!(health::lock(&metrics)
        .metrics_text(now_ms())
        .contains("result=\"sequence_mismatch\"} 2\n"));
    Ok(())
}

#[test]
fn ahead_refusal_expected_above_sent_keeps_pending_and_retry() -> TestResult {
    retained_sequence_refusal(
        "account sequence mismatch, expected 4, got 3: incorrect account sequence",
    )
}

#[test]
fn ahead_refusal_missing_numbers_keeps_pending_and_retry() -> TestResult {
    for log in [
        "account sequence mismatch",
        "account sequence mismatch, expected 2",
        "account sequence mismatch, expected 2, got unknown",
        "account sequence mismatch, expected unknown, got 3",
    ] {
        retained_sequence_refusal(log)?;
    }
    Ok(())
}

#[test]
fn ahead_refusal_without_other_pending_retries_from_committed_state() -> TestResult {
    let peer = Peer::new();
    peer.admit_only_committed_sequence();
    let config = bound_four(&peer);
    let dir = StateDir::new();
    let metrics = Mutex::new(health::Metrics::default());
    let mut sidecar = start(&config, &peer, &dir);
    let quote = config
        .fetch_ticker_loopback(&agent(), "BTC", "fixture-credential", now_ms)
        .map_err(|error| format!("{error:?}"))?;
    let results: Vec<_> = config
        .markets
        .iter()
        .map(|market| (market, Ok(quote.clone())))
        .collect();
    let observations = sidecar
        .collect(&results, &mut health::lock(&metrics))
        .unwrap();
    peer.state.lock().unwrap().account_sequence = 2;
    sidecar.submit(&config, &peer.rpc(), &observations, &metrics, &dir.0)?;
    let state = peer.state.lock().unwrap();
    assert_eq!(state.txs.len(), 2);
    assert_eq!(tx_sequence(&state.txs[0]), 3);
    assert_eq!(tx_sequence(&state.txs[1]), 2);
    assert_eq!(tx_observation_sequences(&state.txs[1]), [(0, 1), (1, 1)]);
    assert_eq!(state.sequence_refusals, 1);
    assert_eq!(sidecar.pending_count(), 1);
    Ok(())
}

#[test]
fn ahead_refusal_restart_reconciles_remaining_pending_submission() -> TestResult {
    let peer = Peer::new();
    peer.admit_only_committed_sequence();
    let config = bound_four(&peer);
    let dir = StateDir::new();
    let metrics = Mutex::new(health::Metrics::default());
    let mut sidecar = start(&config, &peer, &dir);
    run(&config, &peer, &dir, &mut sidecar, &metrics)?;
    run(&config, &peer, &dir, &mut sidecar, &metrics)?;
    assert_eq!(sidecar.pending_count(), 1);
    let progress = dir.progress();
    assert_eq!(progress.as_object().unwrap().len(), 3);
    drop(sidecar);

    let mut reopened = start(&config, &peer, &dir);
    assert!(
        reopened
            .reconcile(&config, &peer.rpc(), &metrics, &dir.0)?
            .fate_open
    );
    assert_eq!(dir.progress(), progress);
    peer.commit_submission(0, 840);
    assert!(
        !reopened
            .reconcile(&config, &peer.rpc(), &metrics, &dir.0)?
            .fate_open
    );
    assert_eq!(reopened.pending_count(), 0);
    assert_eq!(Journal::load(&dir.0, 0)?.markets["BTC"].sequence, 1);
    assert_eq!(Journal::load(&dir.0, 0)?.markets["ETH"].sequence, 1);
    run(&config, &peer, &dir, &mut reopened, &metrics)?;
    let state = peer.state.lock().unwrap();
    assert_eq!(state.txs.len(), 3);
    assert_eq!(tx_sequence(&state.txs[2]), 4);
    assert_eq!(tx_observation_sequences(&state.txs[2]), [(0, 2), (1, 2)]);
    Ok(())
}
