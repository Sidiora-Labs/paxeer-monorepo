#[path = "../src/bin/price-sidecar/main.rs"]
pub mod sidecar;
pub use sidecar::{health, now_ms, round, Config, MarketConfig};

use std::sync::Mutex;

use base64::engine::general_purpose::STANDARD;
use base64::Engine as _;
use layerx_oracle_feeder::cosmos_tx::ACCOUNT_PREFIX;
use layerx_oracle_feeder::{hex_decode, Journal};
use serde_json::{json, Value};
use zeroize::Zeroizing;

#[path = "support/submission_peer.rs"]
mod submission_peer;
use submission_peer::*;

fn run(
    config: &Config,
    rpc: &round::Rpc,
    dir: &StateDir,
    sidecar: &mut round::Sidecar,
    metrics: &Mutex<health::Metrics>,
) -> TestResult {
    round::run_round(
        config,
        &agent(),
        "fixture-credential",
        rpc,
        sidecar,
        metrics,
        &dir.0,
    )
}

#[test]
fn consensus_observation_decides_fate_without_evm_read() -> TestResult {
    let peer = Peer::new();
    let evm = Peer::new();
    peer.disable_transaction_lookup();
    let mut config = config(&peer);
    config.max_pending_submissions = 1;
    config.markets.push(MarketConfig {
        market_id: 1,
        symbol: "ETH".to_owned(),
    });
    let rpc = round::Rpc::new(agent(), peer.url.clone(), evm.url.clone())?;
    let dir = StateDir::new();
    let metrics = Mutex::new(health::Metrics::default());
    let mut sidecar = round::start(&config, &rpc, Zeroizing::new([1; 32]), &dir.0)?;
    submit(&config, &peer, &dir, &mut sidecar, &metrics);
    peer.serve_fresh_quotes();
    let pending = dir.progress();
    assert_eq!(sidecar.pending_count(), 1);
    drop(sidecar);
    peer.commit_submission(0, 840);
    peer.state.lock().unwrap().advance_height_on_account_read = true;
    let mut sidecar = round::start(&config, &rpc, Zeroizing::new([1; 32]), &dir.0)?;
    assert_eq!(dir.progress(), pending);

    run(&config, &rpc, &dir, &mut sidecar, &metrics)?;

    let state = peer.state.lock().unwrap();
    assert_eq!(state.txs.len(), 2);
    assert_eq!(tx_sequence(&state.txs[1]), 4);
    assert_eq!(tx_observation_sequences(&state.txs[1]), [(0, 2), (1, 2)]);
    assert_eq!(sidecar.pending_count(), 1);
    let progress = dir.progress();
    assert_eq!(progress["confirmed_height"], 840);
    assert_eq!(progress["pending"].as_array().unwrap().len(), 1);
    assert_ne!(
        progress["pending"][0]["hash"],
        pending["pending"][0]["hash"]
    );
    let journal = Journal::load(&dir.0, 0)?;
    assert_eq!(journal.next_account_sequence, 4);
    for symbol in ["BTC", "ETH"] {
        assert_eq!(journal.markets[symbol].sequence, 1);
        assert_eq!(
            journal.markets[symbol].activity_id,
            pending["pending"][0]["hash"].as_str().unwrap()
        );
    }
    assert!(health::lock(&metrics)
        .metrics_text(now_ms())
        .contains("result=\"committed\"} 1\n"));
    assert_eq!(state.tx_lookups, 1);
    assert!(state.mempool_pages.is_empty());
    assert!(state.evm_observation_reads.is_empty());
    assert!(evm.state.lock().unwrap().evm_observation_reads.is_empty());
    assert_eq!(state.observation_requests.len(), 4);
    for (request, &(market_id, height)) in state
        .observation_requests
        .iter()
        .zip(&state.observation_reads)
    {
        assert_eq!(request["path"], round::OBSERVATION_QUERY_PATH);
        assert_eq!(request["height"], height.to_string());
        assert!(state.account_reads.contains(&height));
        assert!([0, 1].contains(&market_id));
        assert_eq!(request["prove"], false);
    }
    assert_eq!(
        state
            .observation_reads
            .iter()
            .filter(|&&(_, height)| height == 840)
            .count(),
        2
    );
    Ok(())
}

fn decode_bech32(address: &str) -> Result<(String, Vec<u8>), String> {
    if !(8..=90).contains(&address.len())
        || !address.bytes().all(|byte| (33..=126).contains(&byte))
        || (address.bytes().any(|byte| byte.is_ascii_uppercase())
            && address.bytes().any(|byte| byte.is_ascii_lowercase()))
    {
        return Err("invalid bech32 text".to_owned());
    }
    let address = address.to_ascii_lowercase();
    let separator = address.rfind('1').ok_or("missing bech32 separator")?;
    let (hrp, tail) = address.split_at(separator);
    if hrp.is_empty() || tail.len() < 7 {
        return Err("invalid bech32 length".to_owned());
    }
    let charset = b"qpzry9x8gf2tvdw0s3jn54khce6mua7l";
    let words = tail.as_bytes()[1..]
        .iter()
        .map(|byte| {
            charset
                .iter()
                .position(|candidate| candidate == byte)
                .map(|value| value as u8)
                .ok_or_else(|| "invalid bech32 character".to_owned())
        })
        .collect::<Result<Vec<_>, _>>()?;
    let expanded = hrp
        .bytes()
        .map(|byte| byte >> 5)
        .chain([0])
        .chain(hrp.bytes().map(|byte| byte & 31));
    let mut checksum = 1u32;
    for word in expanded.chain(words.iter().copied()) {
        let top = checksum >> 25;
        checksum = ((checksum & 0x1ff_ffff) << 5) ^ u32::from(word);
        for (bit, generator) in [0x3b6a57b2, 0x26508e6d, 0x1ea119fa, 0x3d4233dd, 0x2a1462b3]
            .iter()
            .enumerate()
        {
            if (top >> bit) & 1 != 0 {
                checksum ^= generator;
            }
        }
    }
    if checksum != 1 {
        return Err("invalid bech32 checksum".to_owned());
    }
    let mut bytes = Vec::new();
    let mut accumulator = 0u32;
    let mut bits = 0;
    for word in &words[..words.len() - 6] {
        accumulator = ((accumulator << 5) | u32::from(*word)) & 0xfff;
        bits += 5;
        if bits >= 8 {
            bits -= 8;
            bytes.push((accumulator >> bits) as u8);
        }
    }
    if bits >= 5 || ((accumulator << (8 - bits)) & 0xff) != 0 {
        return Err("invalid bech32 padding".to_owned());
    }
    Ok((hrp.to_owned(), bytes))
}

#[test]
fn consensus_observation_request_bytes() -> TestResult {
    let peer = Peer::new();
    let validator: [u8; 20] = std::array::from_fn(|index| index as u8);
    {
        let mut state = peer.state.lock().unwrap();
        state.validator = validator;
        state.observation_sequences.insert(300, 300);
        state.observation_sequences.insert(0, u64::MAX);
    }
    let mut config = config(&peer);
    config.markets = vec![
        MarketConfig {
            market_id: 300,
            symbol: "BTC".to_owned(),
        },
        MarketConfig {
            market_id: 0,
            symbol: "ETH".to_owned(),
        },
    ];
    let dir = StateDir::new();
    let _sidecar = round::start(&config, &peer.rpc(), Zeroizing::new([1; 32]), &dir.0)?;
    let journal = Journal::load(&dir.0, 0)?;
    assert_eq!(journal.markets["BTC"].sequence, 300);
    assert_eq!(journal.markets["ETH"].sequence, u64::MAX);
    let valoper = "paxvaloper1qqqsyqcyq5rqwzqfpg9scrgwpugpzysn55yk8j";
    let mut expected = vec![0x08, 0xac, 0x02, 0x12, 0x31];
    expected.extend_from_slice(b"paxvaloper1qqqsyqcyq5rqwzqfpg9scrgwpugpzysn55yk8j");
    let mut zero = vec![0x12, 0x31];
    zero.extend_from_slice(b"paxvaloper1qqqsyqcyq5rqwzqfpg9scrgwpugpzysn55yk8j");
    let state = peer.state.lock().unwrap();
    assert_eq!(state.observation_requests.len(), 2);
    for request in &state.observation_requests {
        let bytes = hex_decode(request["data"].as_str().unwrap())?;
        if bytes[0] == 0x08 {
            assert_eq!(bytes, expected);
        } else {
            assert_eq!(bytes, zero);
        }
        assert_eq!(request["height"], "830");
        assert_eq!(request["prove"], false);
    }
    let mut markets: Vec<_> = state
        .observation_reads
        .iter()
        .map(|&(market, _)| market)
        .collect();
    markets.sort_unstable();
    assert_eq!(markets, [0, 300]);
    let encoded_validator =
        std::str::from_utf8(&expected[5..]).map_err(|error| error.to_string())?;
    assert_eq!(encoded_validator, valoper);
    assert!(encoded_validator.starts_with(&format!("{ACCOUNT_PREFIX}valoper1")));
    let (hrp, decoded) = decode_bech32(encoded_validator)?;
    assert_eq!(hrp, format!("{ACCOUNT_PREFIX}valoper"));
    assert_eq!(decoded, validator);
    assert!(decode_bech32(&format!("{}q", &valoper[..valoper.len() - 1])).is_err());
    assert!(state.evm_observation_reads.is_empty());
    Ok(())
}

fn response(fields: Value) -> String {
    json!({"jsonrpc":"2.0","id":1,"result":{"response":fields}}).to_string()
}

#[test]
fn consensus_observation_absent_is_sequence_zero() -> TestResult {
    assert_eq!(
        round::parse_observation(
            &response(json!({
                "code":22,"codespace":"sdk","log":"not found","height":"830"
            })),
            830
        )?,
        0
    );
    for code in [None, Some(json!(0)), Some(json!("0"))] {
        let mut fields = json!({"height":"830","value":STANDARD.encode([0x12, 1, b'1', 0x1a, 1, b'2', 0x20, 1, 0x28, 2])});
        if let Some(code) = code {
            fields["code"] = code;
        }
        assert_eq!(round::parse_observation(&response(fields), 830)?, 0);
    }
    let peer = Peer::new();
    peer.state
        .lock()
        .unwrap()
        .observation_absent_markets
        .insert(0);
    let config = config(&peer);
    let dir = StateDir::new();
    let metrics = Mutex::new(health::Metrics::default());
    let mut sidecar = round::start(&config, &peer.rpc(), Zeroizing::new([1; 32]), &dir.0)?;
    assert_eq!(Journal::load(&dir.0, 0)?.markets["BTC"].sequence, 0);
    let submission = submit(&config, &peer, &dir, &mut sidecar, &metrics);
    assert_eq!(submission.observations[0].sequence, 1);
    let state = peer.state.lock().unwrap();
    assert_eq!(state.observation_reads, [(0, 830)]);
    assert!(state.evm_observation_reads.is_empty());
    Ok(())
}

fn assert_failed_read(overrides: Value, expected: &str) -> TestResult {
    let peer = Peer::new();
    peer.disable_transaction_lookup();
    let config = config(&peer);
    let dir = StateDir::new();
    let metrics = Mutex::new(health::Metrics::default());
    let mut sidecar = start(&config, &peer, &dir);
    submit(&config, &peer, &dir, &mut sidecar, &metrics);
    peer.serve_fresh_quotes();
    peer.commit_submission(0, 840);
    let pending = dir.progress();
    let journal = Journal::load(&dir.0, 0)?;
    {
        let mut state = peer.state.lock().unwrap();
        state.observation_response = Some(overrides);
        state.observation_reads.clear();
        state.observation_requests.clear();
    }
    let error = sidecar
        .reconcile(&config, &peer.rpc(), &metrics, &dir.0)
        .unwrap_err();
    assert!(error.contains(expected), "{error}");
    assert_eq!(sidecar.pending_count(), 1);
    assert_eq!(dir.progress(), pending);
    assert_eq!(Journal::load(&dir.0, 0)?, journal);
    let requests = peer.state.lock().unwrap().ticker_requests;

    run(&config, &peer.rpc(), &dir, &mut sidecar, &metrics)?;

    assert_eq!(sidecar.pending_count(), 1);
    assert_eq!(dir.progress(), pending);
    assert_eq!(Journal::load(&dir.0, 0)?, journal);
    assert_unconfirmed(&metrics);
    let state = peer.state.lock().unwrap();
    assert_eq!(state.txs.len(), 1);
    assert_eq!(state.ticker_requests, requests);
    assert_eq!(state.observation_reads, [(0, 840), (0, 840)]);
    assert!(state.observation_busy_reads.is_empty());
    assert!(state.evm_observation_reads.is_empty());
    Ok(())
}

#[test]
fn consensus_observation_other_code_fails_the_read() -> TestResult {
    assert_failed_read(
        json!({"code":17,"codespace":"pricefeed","log":"observation unavailable"}),
        "code 17 pricefeed: observation unavailable",
    )?;
    let error = round::parse_observation(
        &response(json!({"height":"840","code":22,"codespace":"pricefeed","log":"not found"})),
        840,
    )
    .unwrap_err();
    assert!(error.contains("code 22 pricefeed: not found"), "{error}");
    let rpc_error =
        json!({"code":-32000,"message":"observation unavailable","data":"query refused"});
    let error = round::parse_observation(&json!({"error":rpc_error}).to_string(), 840).unwrap_err();
    assert!(error.contains(&rpc_error.to_string()), "{error}");
    for code in [json!("invalid"), json!(null), json!(-1)] {
        assert!(round::parse_observation(
            &response(json!({"height":"840","code":code,"value":""})),
            840
        )
        .is_err());
    }
    for value in [
        None,
        Some(json!("invalid base64")),
        Some(json!(STANDARD.encode([0x08]))),
    ] {
        let mut fields = json!({"height":"840","code":0});
        if let Some(value) = value {
            fields["value"] = value;
        }
        assert!(round::parse_observation(&response(fields), 840).is_err());
    }
    Ok(())
}

#[test]
fn consensus_observation_other_height_fails_the_read() -> TestResult {
    assert_failed_read(
        json!({"height":"841"}),
        "observation height 841, requested 840",
    )?;
    for code in [0, 22] {
        for height in [
            None,
            Some(json!(null)),
            Some(json!("invalid")),
            Some(json!(-1)),
            Some(json!(841)),
        ] {
            let mut fields = json!({"code":code,"codespace":"sdk","value":""});
            if let Some(height) = height {
                fields["height"] = height;
            }
            assert!(round::parse_observation(&response(fields), 840).is_err());
        }
    }
    Ok(())
}
