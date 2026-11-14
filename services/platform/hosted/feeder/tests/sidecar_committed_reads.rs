#[path = "../src/bin/price-sidecar/main.rs"]
pub mod sidecar;
pub use sidecar::{health, now_ms, round, Config, MarketConfig};

use std::sync::Mutex;

use serde_json::json;
use zeroize::Zeroizing;

#[path = "support/submission_peer.rs"]
mod submission_peer;
use submission_peer::*;

fn disable_lookup(peer: &Peer) {
    peer.state.lock().unwrap().lookup_error = Some(json!({
        "code": -32603,
        "message": "Internal error",
        "data": "transaction querying is disabled",
    }));
}

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

#[test]
fn committed_reads_decided_fate_reads_each_market_once_in_parallel() -> TestResult {
    let peer = Peer::new();
    peer.state.lock().unwrap().fate = Fate::Committed {
        code: 0,
        height: 840,
    };
    let config = eighteen_markets(&peer);
    let dir = StateDir::new();
    let metrics = Mutex::new(health::Metrics::default());
    let mut sidecar = round::start(&config, &peer.rpc(), Zeroizing::new([1; 32]), &dir.0)?;
    submit(&config, &peer, &dir, &mut sidecar, &metrics);
    disable_lookup(&peer);
    {
        let mut state = peer.state.lock().unwrap();
        state.account_sequence = 5;
        state.account_reads.clear();
        state.observation_reads.clear();
    }
    peer.hold_observation_reads_until(config.markets.len());

    assert!(sidecar.reconcile(&config, &peer.rpc(), &metrics, &dir.0)?);
    let state = peer.state.lock().unwrap();
    assert!(peer.observation_bound_passed());
    assert_eq!(state.account_reads.len(), 1);
    assert_eq!(state.observation_reads.len(), config.markets.len());
    let mut markets: Vec<_> = state
        .observation_reads
        .iter()
        .map(|(market_id, _)| *market_id)
        .collect();
    markets.sort_unstable();
    assert_eq!(
        markets,
        (0..config.markets.len() as u64).collect::<Vec<_>>()
    );
    assert!(dir.progress()["pending"].as_array().unwrap().is_empty());
    Ok(())
}

#[test]
fn committed_reads_open_fate_skips_observations_until_commit() -> TestResult {
    let peer = Peer::new();
    let config = config(&peer);
    let dir = StateDir::new();
    let metrics = Mutex::new(health::Metrics::default());
    let mut sidecar = round::start(&config, &peer.rpc(), Zeroizing::new([1; 32]), &dir.0)?;
    submit(&config, &peer, &dir, &mut sidecar, &metrics);
    disable_lookup(&peer);
    {
        let mut state = peer.state.lock().unwrap();
        state.account_reads.clear();
        state.observation_reads.clear();
    }
    peer.hold_observation_reads_until(config.markets.len());

    assert!(!sidecar.reconcile(&config, &peer.rpc(), &metrics, &dir.0)?);
    {
        let state = peer.state.lock().unwrap();
        assert_eq!(state.account_reads.len(), 1);
        assert!(state.observation_reads.is_empty());
    }
    assert!(!peer.observation_bound_passed());

    {
        let mut state = peer.state.lock().unwrap();
        state.fate = Fate::Committed {
            code: 0,
            height: 840,
        };
        state.account_sequence = 4;
        state.observation_sequence = 1;
        state.account_reads.clear();
        state.observation_reads.clear();
    }
    assert!(sidecar.reconcile(&config, &peer.rpc(), &metrics, &dir.0)?);
    let state = peer.state.lock().unwrap();
    assert!(peer.observation_bound_passed());
    assert_eq!(state.observation_reads.len(), config.markets.len());
    assert!(dir.progress()["pending"].as_array().unwrap().is_empty());
    Ok(())
}
