#[path = "../src/bin/price-sidecar/main.rs"]
mod sidecar;
pub use sidecar::{health, now_ms, round, Config, MarketConfig};

use std::sync::Mutex;
use std::thread;
use std::time::{Duration, Instant};

use serde_json::json;
use zeroize::Zeroizing;

#[path = "support/submission_peer.rs"]
mod submission_peer;
use submission_peer::*;

#[test]
fn cadence_pending_committed_cadence_keeps_advancing() -> TestResult {
    let peer = Peer::new();
    {
        let mut state = peer.state.lock().unwrap();
        state.response_delay_ms = 20;
        state.fate = Fate::Committed {
            code: 0,
            height: 840,
        };
    }
    let mut config = config(&peer);
    config.cadence_ms = 100;
    config.max_pending_submissions = 8;
    config.chainflow.max_quote_age_ms = u64::MAX;
    config.markets = (0..18)
        .map(|market_id| MarketConfig {
            market_id,
            symbol: [
                "BTC", "ETH", "SOL", "AVAX", "LINK", "TSLA", "NVDA", "NAS100", "XAU", "SPX500",
                "GOOGL", "PAX", "SID", "HYPE", "XRP", "ASTER", "TRUMP", "BNB",
            ][market_id as usize]
                .to_owned(),
        })
        .collect();
    let dir = StateDir::new();
    let metrics = Mutex::new(health::Metrics::default());
    let mut sidecar = round::start(&config, &peer.rpc(), Zeroizing::new([1; 32]), &dir.0)?;
    let started = Instant::now();
    while started.elapsed() < Duration::from_secs(5) {
        round::run_round(
            &config,
            &agent(),
            "fixture-credential",
            &peer.rpc(),
            &mut sidecar,
            &metrics,
            &dir.0,
        )?;
        thread::sleep(Duration::from_millis(100));
    }
    assert!(peer.state.lock().unwrap().observation_sequence >= 20);
    Ok(())
}

#[test]
fn cadence_pending_bound_stops_new_broadcasts() -> TestResult {
    let peer = Peer::new();
    let mut config = config(&peer);
    config.max_pending_submissions = 3;
    let dir = StateDir::new();
    let metrics = Mutex::new(health::Metrics::default());
    let mut sidecar = start(&config, &peer, &dir);
    for _ in 0..3 {
        submit(&config, &peer, &dir, &mut sidecar, &metrics);
    }
    assert_eq!(dir.progress()["pending"].as_array().map(Vec::len), Some(3));
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
    let state = peer.state.lock().unwrap();
    assert_eq!(state.ticker_requests, before);
    assert_eq!(dir.progress()["pending"].as_array().map(Vec::len), Some(3));
    Ok(())
}

#[test]
fn cadence_pending_evicted_oldest_ends_queue() -> TestResult {
    let peer = Peer::new();
    let config = config(&peer);
    let dir = StateDir::new();
    let metrics = Mutex::new(health::Metrics::default());
    let mut sidecar = start(&config, &peer, &dir);
    for _ in 0..3 {
        submit(&config, &peer, &dir, &mut sidecar, &metrics);
    }
    let proposed = dir.progress()["pending"][0]["proposed_height"]
        .as_u64()
        .ok_or("missing proposed height")?;
    {
        let mut state = peer.state.lock().unwrap();
        state.fate = Fate::Evicted;
        state.committed_height = Some(proposed + config.pending_submission_block_limit);
    }
    assert!(sidecar.reconcile(&config, &peer.rpc(), &metrics, &dir.0)?);
    assert!(dir.progress()["pending"].as_array().unwrap().is_empty());
    Ok(())
}

#[test]
fn cadence_pending_failed_middle_preserves_later_order() -> TestResult {
    let peer = Peer::new();
    {
        let mut state = peer.state.lock().unwrap();
        state.tx_fates = vec![
            Fate::Committed {
                code: 0,
                height: 840,
            },
            Fate::Committed {
                code: 12,
                height: 841,
            },
            Fate::Committed {
                code: 0,
                height: 842,
            },
        ];
    }
    let config = config(&peer);
    let dir = StateDir::new();
    let metrics = Mutex::new(health::Metrics::default());
    let mut sidecar = start(&config, &peer, &dir);
    for _ in 0..3 {
        submit(&config, &peer, &dir, &mut sidecar, &metrics);
    }
    assert!(sidecar.reconcile(&config, &peer.rpc(), &metrics, &dir.0)?);
    assert!(dir.progress()["pending"].as_array().unwrap().is_empty());
    let text = health::lock(&metrics).metrics_text(now_ms());
    assert!(text.contains("result=\"failed\"} 1"));
    assert_eq!(peer.state.lock().unwrap().txs.len(), 3);
    Ok(())
}

#[test]
fn cadence_pending_restart_decides_each_entry() -> TestResult {
    let peer = Peer::new();
    let config = config(&peer);
    let dir = StateDir::new();
    let metrics = Mutex::new(health::Metrics::default());
    let mut sidecar = start(&config, &peer, &dir);
    submit(&config, &peer, &dir, &mut sidecar, &metrics);
    submit(&config, &peer, &dir, &mut sidecar, &metrics);
    drop(sidecar);
    peer.state.lock().unwrap().fate = Fate::Committed {
        code: 0,
        height: 850,
    };
    let mut reopened = start(&config, &peer, &dir);
    assert!(reopened.reconcile(&config, &peer.rpc(), &metrics, &dir.0)?);
    assert!(dir.progress()["pending"].as_array().unwrap().is_empty());
    Ok(())
}

#[test]
fn cadence_pending_legacy_progress_loads_as_list() -> TestResult {
    let peer = Peer::new();
    let config = config(&peer);
    let dir = StateDir::new();
    let metrics = Mutex::new(health::Metrics::default());
    let mut sidecar = start(&config, &peer, &dir);
    submit(&config, &peer, &dir, &mut sidecar, &metrics);
    let mut progress = dir.progress();
    let pending = progress["pending"][0].clone();
    progress["pending"] = pending;
    std::fs::write(
        dir.0.join("price-sidecar-progress.json"),
        serde_json::to_vec(&progress).map_err(|error| error.to_string())?,
    )
    .map_err(|error| error.to_string())?;
    drop(sidecar);
    peer.state.lock().unwrap().fate = Fate::Committed {
        code: 0,
        height: 851,
    };
    let mut reopened = start(&config, &peer, &dir);
    assert!(reopened.reconcile(&config, &peer.rpc(), &metrics, &dir.0)?);
    assert!(dir.progress()["pending"].as_array().unwrap().is_empty());
    Ok(())
}

#[test]
fn cadence_pending_one_reconcile_reads_each_market_once() -> TestResult {
    let peer = Peer::new();
    let config = config(&peer);
    let dir = StateDir::new();
    let metrics = Mutex::new(health::Metrics::default());
    let mut sidecar = start(&config, &peer, &dir);
    submit(&config, &peer, &dir, &mut sidecar, &metrics);
    peer.state.lock().unwrap().lookup_error = Some(json!({
        "code": -32603,
        "message": "Internal error",
        "data": "transaction querying is disabled",
    }));
    peer.state.lock().unwrap().account_sequence = 3;
    peer.state.lock().unwrap().observation_sequence = 0;
    peer.state.lock().unwrap().account_reads.clear();
    peer.state.lock().unwrap().observation_reads.clear();
    assert!(!sidecar.reconcile(&config, &peer.rpc(), &metrics, &dir.0)?);
    let state = peer.state.lock().unwrap();
    assert_eq!(state.account_reads.len(), 1);
    assert_eq!(state.observation_reads.len(), config.markets.len());
    Ok(())
}
