#[path = "../src/bin/price-sidecar/main.rs"]
pub mod sidecar;
pub use sidecar::{health, now_ms, round, start_report, Config, MarketConfig};

use std::sync::Mutex;

use serde_json::json;

#[path = "support/submission_peer.rs"]
mod submission_peer;
use submission_peer::*;

fn config_without_pending_key(peer: &Peer) -> Config {
    let configured = config(peer);
    let value = json!({
        "chain_id": configured.chain_id,
        "validator": configured.validator,
        "feed_account": configured.feed_account,
        "gas_limit": configured.gas_limit,
        "cadence_ms": configured.cadence_ms,
        "max_committed_reads_in_flight": configured.max_committed_reads_in_flight,
        "pending_submission_block_limit": configured.pending_submission_block_limit,
        "retry_backoff_ms": configured.retry_backoff_ms,
        "retry_max_attempts": configured.retry_max_attempts,
        "chainflow": {
            "base_url": configured.chainflow.base_url,
            "api_key_env": configured.chainflow.api_key_env,
            "timeout_ms": configured.chainflow.timeout_ms,
            "max_quote_age_ms": configured.chainflow.max_quote_age_ms,
        },
        "markets": configured
            .markets
            .iter()
            .map(|market| json!({"market_id": market.market_id, "symbol": market.symbol}))
            .collect::<Vec<_>>(),
    });
    serde_json::from_value(value).unwrap()
}

#[test]
fn pending_default_start_report_names_one_submission() -> TestResult {
    let peer = Peer::new();
    let config = config_without_pending_key(&peer);
    assert_eq!(config.max_pending_submissions, 1);
    assert!(start_report(&config).contains("max_pending_submissions 1"));
    Ok(())
}

#[test]
fn pending_default_committed_sequence_admits_twelve_rounds() -> TestResult {
    let peer = Peer::new();
    peer.admit_only_committed_sequence();
    peer.serve_fresh_quotes();
    {
        peer.state.lock().unwrap().fate = Fate::Committed {
            code: 0,
            height: 840,
        };
    }
    let mut config = config_without_pending_key(&peer);
    config.chainflow.max_quote_age_ms = u64::MAX;
    let dir = StateDir::new();
    let metrics = Mutex::new(health::Metrics::default());
    let mut sidecar = start(&config, &peer, &dir);

    for _ in 0..12 {
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

    let state = peer.state.lock().unwrap();
    assert_eq!(state.txs.len(), 12);
    assert_eq!(state.sequence_refusals, 0);
    assert_eq!(state.account_sequence, 15);
    assert_eq!(dir.progress()["pending"].as_array().unwrap().len(), 1);
    Ok(())
}

#[test]
fn pending_default_bound_four_removes_ahead_refusal_and_waits_for_committed_sequence() -> TestResult
{
    let peer = Peer::new();
    peer.admit_only_committed_sequence();
    peer.serve_fresh_quotes();
    let mut config = config_without_pending_key(&peer);
    config.max_pending_submissions = 4;
    config.chainflow.max_quote_age_ms = u64::MAX;
    let dir = StateDir::new();
    let metrics = Mutex::new(health::Metrics::default());
    let mut sidecar = start(&config, &peer, &dir);

    for _ in 0..4 {
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
    assert_eq!(sidecar.pending_count(), 1);
    assert_eq!(dir.progress()["pending"].as_array().unwrap().len(), 1);
    assert_eq!(peer.state.lock().unwrap().sequence_refusals, 1);

    let before = peer.state.lock().unwrap().ticker_requests;
    round::run_round(
        &config,
        &agent(),
        "fixture-credential",
        &peer.rpc(),
        &mut sidecar,
        &metrics,
        &dir.0,
    )?;
    assert_eq!(sidecar.pending_count(), 1);
    assert_eq!(peer.state.lock().unwrap().ticker_requests, before);
    Ok(())
}
