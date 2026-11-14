//! Task 22.33.1412: real sidecar, signed Cosmos codec, durable store and loopback peers.
#[path = "../src/bin/price-sidecar/main.rs"]
pub mod sidecar;
pub use sidecar::{health, now_ms, round, Config, MarketConfig};

use std::sync::Mutex;

use layerx_oracle_feeder::{hex_encode, Journal, MarketRecord};
use sha2::{Digest, Sha256};

#[path = "support/submission_peer.rs"]
mod submission_peer;
use submission_peer::*;

#[test]
fn finding_031_deployed_https_and_isolated_loopback_transport() {
    let peer = Peer::new();
    let mut config = config(&peer);
    assert!(config.validate().is_err());
    assert!(config
        .fetch_ticker(&agent(), "BTC", "fixture-credential", || NOW)
        .is_err());
    assert_eq!(
        peer.state
            .lock()
            .unwrap_or_else(|error| panic!("{error:?}"))
            .ticker_requests,
        0
    );
    let quote = ticker(&config);
    assert_eq!(quote.index_price, "65000500000000000000000");
    assert_eq!(
        peer.state
            .lock()
            .unwrap_or_else(|error| panic!("{error:?}"))
            .api_keys,
        ["fixture-credential"]
    );
    for url in [
        "http://feed.example",
        "http://127.0.0.1",
        "http://[::1]",
        "ftp://feed.example",
        "https://",
        "https://user:password@feed.example",
    ] {
        config.chainflow.base_url = url.to_owned();
        assert!(
            config.validate().is_err(),
            "deployed configuration admitted {url}"
        );
    }
    for url in [
        "http://feed.example",
        "http://localhost",
        "http://127.0.0.1.example",
        "http://127.0.0.1@feed.example",
        "http://192.0.2.1",
    ] {
        config.chainflow.base_url = url.to_owned();
        assert!(config
            .fetch_ticker_loopback(&agent(), "BTC", "fixture-credential", || NOW)
            .is_err());
    }
    config.chainflow.base_url = "https://feed.example".to_owned();
    assert_eq!(config.validate(), Ok(()));
}

#[test]
fn finding_031_authenticated_quote_redirect_is_not_followed() {
    let peer = Peer::new();
    let destination = Peer::new();
    peer.state
        .lock()
        .unwrap_or_else(|error| panic!("{error:?}"))
        .redirect = Some(format!("{}/api/markets/ticker/BTC", destination.url));
    let config = config(&peer);
    // Even a caller's redirect-enabled agent cannot forward the credential.
    let redirecting_agent: ureq::Agent = ureq::Agent::config_builder()
        .max_redirects(5)
        .build()
        .into();
    assert!(config
        .fetch_ticker_loopback(&redirecting_agent, "BTC", "fixture-credential", || NOW)
        .is_err());
    assert_eq!(
        peer.state
            .lock()
            .unwrap_or_else(|error| panic!("{error:?}"))
            .ticker_requests,
        1
    );
    assert_eq!(
        destination
            .state
            .lock()
            .unwrap_or_else(|error| panic!("{error:?}"))
            .ticker_requests,
        0
    );
}

#[test]
fn finding_029_checktx_is_pending_and_restart_confirms_actual_height() -> TestResult {
    let peer = Peer::new();
    let config = config(&peer);
    let dir = StateDir::new();
    let metrics = Mutex::new(health::Metrics::default());
    let mut sidecar = start(&config, &peer, &dir);
    let submission = submit(&config, &peer, &dir, &mut sidecar, &metrics);
    assert_eq!(submission.sequence, 3);
    assert_eq!(submission.observations[0].sequence, 1);
    assert_unconfirmed(&metrics);
    let text = health::lock(&metrics).metrics_text(now_ms());
    assert!(text.contains("result=\"submitted\"} 1\n"));
    assert!(text.contains("price_sidecar_proposed_target_height 831\n"));
    assert!(text.contains("price_sidecar_pending_submission{hash=\""));
    let journal = Journal::load(&dir.0, 0)?;
    assert_eq!(journal.next_account_sequence, 3);
    assert_eq!(journal.markets["BTC"].sequence, 0);
    assert!(!sidecar.reconcile(&config, &peer.rpc(), &metrics, &dir.0)?);
    let requests = peer
        .state
        .lock()
        .unwrap_or_else(|error| panic!("{error:?}"))
        .ticker_requests;
    round::run_round(
        &config,
        &agent(),
        "fixture-credential",
        &peer.rpc(),
        &mut sidecar,
        &metrics,
        &dir.0,
    )?;
    assert_eq!(
        peer.state
            .lock()
            .unwrap_or_else(|error| panic!("{error:?}"))
            .ticker_requests,
        requests
    );
    assert_eq!(
        peer.state
            .lock()
            .unwrap_or_else(|error| panic!("{error:?}"))
            .txs
            .len(),
        1
    );
    drop(sidecar);
    let mut sidecar = start(&config, &peer, &dir);
    let metrics = Mutex::new(health::Metrics::default());
    assert!(!sidecar.reconcile(&config, &peer.rpc(), &metrics, &dir.0)?);
    assert_unconfirmed(&metrics);
    {
        let mut state = peer
            .state
            .lock()
            .unwrap_or_else(|error| panic!("{error:?}"));
        state.fate = Fate::Committed {
            code: 0,
            height: 840,
        };
        state.account_sequence = 4;
        state.observation_sequence = 1;
    }
    assert!(sidecar.reconcile(&config, &peer.rpc(), &metrics, &dir.0)?);
    let journal = Journal::load(&dir.0, 0)?;
    assert_eq!(journal.next_account_sequence, 4);
    assert_eq!(journal.markets["BTC"].sequence, 1);
    assert_eq!(
        journal.markets["BTC"].activity_id,
        hex_encode(&Sha256::digest(
            &peer
                .state
                .lock()
                .unwrap_or_else(|error| panic!("{error:?}"))
                .txs[0]
        ))
    );
    assert_eq!(dir.progress()["confirmed_height"], 840);
    assert!(dir.progress()["pending"].is_null());
    let confirmed_at = dir.progress()["confirmed_ms"]
        .as_u64()
        .unwrap_or_else(|| panic!("required protocol value is absent"));
    assert!(health::lock(&metrics).is_ready(confirmed_at, 1000));
    assert!(!health::lock(&metrics).is_ready(confirmed_at + 3001, 1000));
    assert!(health::lock(&metrics)
        .metrics_text(now_ms())
        .contains("price_sidecar_last_accepted_height 840\n"));
    drop(sidecar);
    let mut reopened = start(&config, &peer, &dir);
    let metrics = Mutex::new(health::Metrics::default());
    reopened.reconcile(&config, &peer.rpc(), &metrics, &dir.0)?;
    assert!(health::lock(&metrics)
        .metrics_text(now_ms())
        .contains("price_sidecar_confirmed_height 840\n"));
    assert_eq!(dir.progress()["confirmed_ms"], confirmed_at);
    Ok(())
}

fn recovery_after_failure(fate: Fate, expected_account: u8, label: &str) -> TestResult {
    let peer = Peer::new();
    let config = config(&peer);
    let dir = StateDir::new();
    let metrics = Mutex::new(health::Metrics::default());
    let mut sidecar = start(&config, &peer, &dir);
    submit(&config, &peer, &dir, &mut sidecar, &metrics);
    drop(sidecar);
    {
        let mut state = peer
            .state
            .lock()
            .unwrap_or_else(|error| panic!("{error:?}"));
        state.fate = fate;
        state.account_sequence = expected_account;
    }
    let mut sidecar = start(&config, &peer, &dir);
    assert!(sidecar.reconcile(&config, &peer.rpc(), &metrics, &dir.0)?);
    assert_unconfirmed(&metrics);
    assert!(dir.progress()["pending"].is_null());
    let journal = Journal::load(&dir.0, 0)?;
    assert_eq!(journal.next_account_sequence, u64::from(expected_account));
    assert_eq!(journal.markets["BTC"].sequence, 0);
    assert!(journal.markets["BTC"].activity_id.is_empty());
    assert!(health::lock(&metrics)
        .metrics_text(now_ms())
        .contains(&format!("result=\"{label}\"}} 1\n")));
    peer.state
        .lock()
        .unwrap_or_else(|error| panic!("{error:?}"))
        .fate = Fate::Pending;
    let retry = submit(&config, &peer, &dir, &mut sidecar, &metrics);
    assert_eq!(retry.sequence, u64::from(expected_account));
    assert_eq!(retry.observations[0].sequence, 1);
    {
        let mut state = peer
            .state
            .lock()
            .unwrap_or_else(|error| panic!("{error:?}"));
        state.fate = Fate::Committed {
            code: 0,
            height: 845,
        };
        state.account_sequence += 1;
        state.observation_sequence = 1;
    }
    assert!(sidecar.reconcile(&config, &peer.rpc(), &metrics, &dir.0)?);
    assert_eq!(dir.progress()["confirmed_height"], 845);
    assert!(health::lock(&metrics).is_ready(now_ms(), 1000));
    Ok(())
}

#[test]
fn finding_029_failed_execution_recovers_account_and_observation_sequences() -> TestResult {
    recovery_after_failure(
        Fate::Committed {
            code: 12,
            height: 840,
        },
        4,
        "failed",
    )
}
#[test]
fn finding_029_eviction_recovers_without_spending_sequences() -> TestResult {
    recovery_after_failure(Fate::Evicted, 3, "evicted")
}

#[test]
fn finding_029_unknown_fate_and_incomplete_mempool_preserve_durable_pending() {
    let peer = Peer::new();
    let config = config(&peer);
    let dir = StateDir::new();
    let metrics = Mutex::new(health::Metrics::default());
    let mut sidecar = start(&config, &peer, &dir);
    submit(&config, &peer, &dir, &mut sidecar, &metrics);
    let pending = dir.progress();
    for fate in [
        Fate::LookupUnavailable,
        Fate::IncompleteMempool,
        Fate::MalformedExecutionCode,
        Fate::WrongTransaction,
        Fate::CommitDuringLookup,
    ] {
        let mut state = peer
            .state
            .lock()
            .unwrap_or_else(|error| panic!("{error:?}"));
        state.fate = fate;
        state.tx_lookups = 0;
        drop(state);
        assert!(sidecar
            .reconcile(&config, &peer.rpc(), &metrics, &dir.0)
            .is_err());
        assert_eq!(dir.progress(), pending);
        assert_unconfirmed(&metrics);
        assert_eq!(
            peer.state
                .lock()
                .unwrap_or_else(|error| panic!("{error:?}"))
                .txs
                .len(),
            1
        );
    }
}

#[test]
fn finding_029_lost_acknowledgement_retries_identical_signed_bytes_and_recovers() -> TestResult {
    let peer = Peer::new();
    peer.state
        .lock()
        .unwrap_or_else(|error| panic!("{error:?}"))
        .lose_ack = true;
    let config = config(&peer);
    let dir = StateDir::new();
    let metrics = Mutex::new(health::Metrics::default());
    let mut sidecar = start(&config, &peer, &dir);
    submit(&config, &peer, &dir, &mut sidecar, &metrics);
    {
        let state = peer
            .state
            .lock()
            .unwrap_or_else(|error| panic!("{error:?}"));
        assert_eq!(state.txs.len(), 2);
        assert_eq!(state.txs[0], state.txs[1]);
    }
    assert_unconfirmed(&metrics);
    drop(sidecar);
    {
        let mut state = peer
            .state
            .lock()
            .unwrap_or_else(|error| panic!("{error:?}"));
        state.fate = Fate::Committed {
            code: 0,
            height: 840,
        };
        state.account_sequence = 4;
        state.observation_sequence = 1;
    }
    let mut reopened = start(&config, &peer, &dir);
    assert!(reopened.reconcile(&config, &peer.rpc(), &metrics, &dir.0)?);
    assert_eq!(dir.progress()["confirmed_height"], 840);
    assert_eq!(
        peer.state
            .lock()
            .unwrap_or_else(|error| panic!("{error:?}"))
            .txs
            .len(),
        2
    );
    Ok(())
}

#[test]
fn finding_029_legacy_optimistic_journal_reopens_at_committed_sequences() -> TestResult {
    let peer = Peer::new();
    let config = config(&peer);
    let dir = StateDir::new();
    let mut journal = Journal {
        next_account_sequence: 99,
        ..Journal::default()
    };
    journal.markets.insert(
        "BTC".to_owned(),
        MarketRecord {
            sequence: 99,
            activity_id: "unconfirmed legacy hash".to_owned(),
            ..MarketRecord::default()
        },
    );
    journal.store(&dir.0)?;
    let mut sidecar = start(&config, &peer, &dir);
    let metrics = Mutex::new(health::Metrics::default());
    let submission = submit(&config, &peer, &dir, &mut sidecar, &metrics);
    assert_eq!(submission.sequence, 3);
    assert_eq!(submission.observations[0].sequence, 1);
    assert_unconfirmed(&metrics);
    Ok(())
}
