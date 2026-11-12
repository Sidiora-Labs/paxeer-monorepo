include!("support/service_harness.rs");
include!("support/service_repairs.rs");

use layerx_gas_station::config::{ConfigError, PaymasterRequired};
use layerx_gas_station::rpc::{JsonRpc, Rung, SequencerReceipt};
use layerx_gas_station::station::StationError;
use layerx_gas_station::tx::recover;

#[test]
fn quote_and_sponsored_submission_over_a_socket() -> TestResult {
    let harness = start("CYCLE")?;
    harness.recorded.phase("receipted");
    let (status, quote) = harness.post("/quote", &harness.quote_body())?;
    assert_eq!(status, 200);
    let expected = json!({
        "sponsor": hex(&harness.sponsor),
        "token": hex(&SIDIORA),
        "maxTokenAmount": "3200000",
        "tokenAmount": AMOUNT.to_string(),
        "deadline": "1019",
        "quoteNonce": "1",
        "gasCost": GAS_COST,
        "decimals": 6,
    });
    assert_eq!(quote["quote"], expected);
    assert_eq!(quote.as_object().map(serde_json::Map::len), Some(2));
    let signed = harness.quoted(&quote)?;
    assert_eq!(recover(signed.digest, &signed.signature)?, harness.sponsor);

    let (status, submitted) = harness.post("/submit", &harness.submission_body(&quote)?)?;
    assert_eq!(status, 200);
    let sent = harness.recorded.sent();
    assert_eq!(sent.len(), 1);
    assert_eq!(sent[0][0], 4);
    let hash = hex(&keccak(&sent[0]));
    assert_eq!(
        submitted,
        json!({"transactionHash": hash, "rung": "receipted", "seq": "42",
            "height": "17", "status": "1", "logs": [],
            "keyId": hex(&word(1)), "receipt": "0x"})
    );
    let submitted = submitted.to_string();
    assert!(!submitted.contains(hex(&sent[0]).trim_start_matches("0x")));
    for secret in &harness.secrets {
        assert!(!submitted.contains(secret.trim_start_matches("0x")));
        assert!(!quote.to_string().contains(secret.trim_start_matches("0x")));
    }
    assert_eq!(
        harness.post("/status", &harness.status_body(&quote))?,
        (
            200,
            json!({
                "state": "completed",
                "deadline": "1019",
                "submission": {"sponsorNonce": "5", "transactionHash": hash},
                "completion": {"outcome": "receipted", "rung": "receipted",
                    "transactionHash": hash, "seq": "42"},
                "budget": {
                    "key": format!("did:layerx:{}", &hex(&[0xd1; 32])[2..]),
                    "gasLimit": "1000000", "usedGas": "200000",
                    "remainingGas": "800000", "dayEnds": "86400",
                },
            })
        )
    );
    assert_eq!(harness.recorded.sent().len(), 1);
    harness.assert_log_clean(&["/quote 200", "/submit 200", "/status 200"]);
    harness.finish()
}

#[test]
fn healthz_answers_a_plain_get_and_refuses_other_methods() -> TestResult {
    let harness = start("HEALTH")?;
    let (status, body) = harness.exchange(b"GET /healthz HTTP/1.1\r\nHost: station\r\n\r\n")?;
    assert_eq!((status, body), (200, json!({"status": "ok"})));
    let (status, body) = harness.post("/healthz", &json!({}))?;
    assert_eq!(
        (status, body),
        (405, json!({"error": "method_not_allowed"}))
    );
    harness.assert_log_clean(&["/healthz 200", "/healthz 405"]);
    harness.finish()
}

#[test]
fn a_failed_did_read_answers_503_unavailable() -> TestResult {
    let harness = start("DID_UNAVAILABLE")?;
    harness.recorded.phase("did_unreachable");
    assert_eq!(
        harness.post("/quote", &harness.quote_body())?,
        (503, json!({"error": "unavailable"}))
    );
    assert!(harness.journal_is_empty()?);
    assert!(harness.recorded.sent().is_empty());
    harness.assert_log_clean(&["/quote 503"]);
    harness.finish()
}

#[test]
fn quote_refusals_are_4xx_and_unavailability_is_5xx() -> TestResult {
    let harness = start("SPLIT")?;
    let mut lines = Vec::new();
    for (field, value) in [
        ("chainId", json!("1")),
        ("token", json!(hex(&[0x66; 20]))),
        ("decimals", json!(18)),
        ("gasCost", json!("1000000000000000001")),
        ("maxTokenAmount", json!("1")),
    ] {
        let mut body = harness.quote_body();
        body[field] = value;
        let (status, answer) = harness.post("/quote", &body)?;
        assert_eq!((status, answer), (422, json!({"error": "refused"})));
        lines.push("/quote 422");
    }
    let mut body = harness.quote_body();
    body["extra"] = json!("field");
    assert_eq!(harness.post("/quote", &body)?.0, 400);
    lines.push("/quote 400");
    assert_eq!(
        harness
            .exchange(b"POST /quote HTTP/1.1\r\nContent-Length: 1\r\n\r\n{")?
            .0,
        400
    );
    lines.push("/quote 400");
    assert_eq!(harness.exchange(b"GET /quote HTTP/1.1\r\n\r\n")?.0, 405);
    lines.push("/quote 405");
    assert_eq!(
        harness.exchange(b"POST /elsewhere HTTP/1.1\r\nContent-Length: 0\r\n\r\n")?,
        (404, json!({"error": "not_found"}))
    );
    lines.push("- 404");
    assert_eq!(
        harness.exchange(b"POST /submit HTTP/1.1\r\nHost: station\r\n\r\n")?,
        (411, json!({"error": "length_required"}))
    );
    lines.push("/submit 411");
    assert_eq!(
        harness.exchange(b"POST /quote HTTP/1.1\r\nContent-Length: 65537\r\n\r\n")?,
        (413, json!({"error": "too_large"}))
    );
    lines.push("/quote 413");
    assert_eq!(
        harness.exchange(b"POST /quote HTTP/1.1\r\nContent-Len")?,
        (408, json!({"error": "timeout"}))
    );
    lines.push("- 408");
    assert!(harness.journal_is_empty()?);

    harness.recorded.phase("stale_rate");
    assert_eq!(
        harness.post("/quote", &harness.quote_body())?,
        (503, json!({"error": "stale_rate"}))
    );
    lines.push("/quote 503");
    harness.recorded.phase("unreachable");
    assert_eq!(harness.post("/quote", &harness.quote_body())?.0, 503);
    lines.push("/quote 503");
    assert!(harness.journal_is_empty()?);
    harness.assert_log_clean(&lines);
    harness.finish()
}

#[test]
fn submission_refusals_are_4xx_and_an_unreachable_node_is_5xx() -> TestResult {
    let harness = start("SUBMIT")?;
    let mut lines = Vec::new();
    harness.recorded.phase("refused");
    let (status, quote) = harness.post("/quote", &harness.quote_body())?;
    assert_eq!(status, 200);
    lines.push("/quote 200");
    let submission = harness.submission_body(&quote)?;
    let mut unknown = submission.clone();
    unknown["batch"]["quote"]["quoteNonce"] = json!("99");
    assert_eq!(harness.post("/submit", &unknown)?.0, 422);
    lines.push("/submit 422");
    let mut tampered = submission.clone();
    tampered["call"]["data"] = json!("0x00");
    assert_eq!(harness.post("/submit", &tampered)?.0, 422);
    lines.push("/submit 422");
    let mut raised = submission.clone();
    raised["batch"]["quote"]["tokenAmount"] = json!("3145141");
    assert_eq!(harness.post("/submit", &raised)?.0, 422);
    lines.push("/submit 422");
    assert!(harness.recorded.sent().is_empty());

    assert_eq!(
        harness.post("/submit", &submission)?,
        (422, json!({"error": "refused"}))
    );
    lines.push("/submit 422");
    assert_eq!(harness.recorded.sent().len(), 1);
    let hash = hex(&keccak(&harness.recorded.sent()[0]));
    assert_eq!(
        harness.post("/submit", &submission)?,
        (422, json!({"error": "refused"}))
    );
    lines.push("/submit 422");
    assert_eq!(harness.recorded.sent().len(), 1);
    harness.recorded.phase("unreachable");
    assert_eq!(
        harness.post("/submit", &submission)?,
        (503, json!({"error": "unavailable"}))
    );
    lines.push("/submit 503");
    assert_eq!(harness.recorded.sent().len(), 1);
    assert_eq!(
        harness.post("/status", &harness.status_body(&quote))?,
        (
            200,
            json!({
                "state": "refused",
                "deadline": "1019",
                "submission": {"sponsorNonce": "5", "transactionHash": hash},
                "completion": null,
                "budget": {
                    "key": format!("did:layerx:{}", &hex(&[0xd1; 32])[2..]),
                    "gasLimit": "1000000", "usedGas": "200000",
                    "remainingGas": "800000", "dayEnds": "86400",
                },
            })
        )
    );
    lines.push("/status 200");
    harness.assert_log_clean(&lines);
    harness.finish()
}

#[test]
fn quote_refused_when_the_rate_is_older_than_max_rate_age() -> TestResult {
    let harness = start_with_rate_age("STALE", 120)?;
    harness.recorded.phase("rate_age_150");
    assert_eq!(
        harness.post("/quote", &harness.quote_body())?,
        (503, json!({"error": "stale_rate"}))
    );
    assert!(harness.journal_is_empty()?);
    harness.recorded.phase("rate_age_120");
    let (status, quote) = harness.post("/quote", &harness.quote_body())?;
    assert_eq!(status, 200);
    assert_eq!(quote["quote"]["tokenAmount"], json!(AMOUNT.to_string()));
    assert_eq!(quote["quote"]["quoteNonce"], json!("1"));
    assert!(harness.recorded.sent().is_empty());
    harness.assert_log_clean(&["/quote 503", "/quote 200"]);
    harness.finish()
}

#[test]
fn a_held_future_nonce_send_answers_at_rung_submitted_and_stays_pending() -> TestResult {
    let (harness, quote, answer) = sequencer_endpoint_submission("HELD", "held")?;
    let hash = hex(&keccak(&harness.recorded.sent()[0]));
    assert_eq!(
        answer,
        (200, json!({"transactionHash": hash, "rung": "submitted"}))
    );
    assert_eq!(
        harness.post("/status", &harness.status_body(&quote))?,
        (
            200,
            json!({
                "state": "pending",
                "deadline": "1019",
                "submission": {"sponsorNonce": "5", "transactionHash": hash},
                "completion": null,
                "budget": {
                    "key": format!("did:layerx:{}", &hex(&[0xd1; 32])[2..]),
                    "gasLimit": "1000000", "usedGas": "200000",
                    "remainingGas": "800000", "dayEnds": "86400",
                },
            })
        )
    );
    harness.assert_log_clean(&["/quote 200", "/submit 200", "/status 200"]);
    harness.finish()
}

#[test]
fn a_reverted_receipt_is_refused_and_reported_reverted() -> TestResult {
    reverted_receipt_survives_http_status_and_restart()
}

#[test]
fn a_receipt_naming_another_sponsor_is_refused_as_unavailable() -> TestResult {
    let (harness, _, answer) = sequencer_endpoint_submission("FOREIGN", "foreign_sponsor")?;
    assert_eq!(answer, (503, json!({"error": "unavailable"})));
    harness.assert_log_clean(&["/quote 200", "/submit 503"]);
    harness.finish()
}

/// A served configuration matching `station_config`, with `registered` as its
/// registered senders.
fn service_json(registered: &Value) -> Value {
    json!({"listen": "127.0.0.1:8545", "gas_limit": 200_000,
        "max_priority_fee_per_gas": 1_000_000_000, "chain_id": CHAIN_ID,
        "endpoints": ["https://paxeer.app"], "paymaster": hex(&PAYMASTER), "token": hex(&SIDIORA),
        "decimals": 6, "max_rate_age": 300, "spread_bps": 500, "margin_bps": 100,
        "per_account_limit": 8_000_000, "per_interval_limit": 16_000_000,
        "per_quote_limit": 4_000_000, "interval_seconds": 60, "balance_floor": 100,
        "relayer_key_env": "PAXEER_SERVICE_PARSE_KEY", "registered_senders": registered})
}

#[test]
fn registered_senders_parse_resolve_their_own_budget_and_are_quoted() -> TestResult {
    let feeder = [0x0f; 20];
    let config = ServiceConfig::parse(
        &service_json(&json!([{"address": hex(&feeder), "budget": 5_000_000}])).to_string(),
    )?;
    assert_eq!(
        config.registered_senders,
        vec![RegisteredSender {
            address: feeder,
            budget: 5_000_000
        }]
    );
    assert_eq!(config.account_budget(&feeder), 5_000_000);
    assert_eq!(config.account_budget(&[0x0e; 20]), 8_000_000);
    let mut unregistered = service_json(&json!([]));
    assert!(ServiceConfig::parse(&unregistered.to_string())?
        .registered_senders
        .is_empty());
    if let Some(fields) = unregistered.as_object_mut() {
        fields.remove("registered_senders");
    }
    assert!(ServiceConfig::parse(&unregistered.to_string())?
        .registered_senders
        .is_empty());
    assert_eq!(
        ServiceConfig::parse(
            &service_json(&json!([{"address": hex(&feeder), "budget": 15_999_999}])).to_string()
        )?
        .account_budget(&feeder),
        15_999_999
    );
    for refused in [
        json!([{"address": hex(&feeder), "budget": 0}]),
        json!([{"address": hex(&feeder), "budget": 16_000_000}]),
        json!([{"address": hex(&feeder), "budget": -1}]),
        json!([{"address": hex(&[0; 20]), "budget": 1}]),
        json!([{"address": "0x0f", "budget": 1}]),
        json!([{"address": hex(&feeder)}]),
        json!([{"budget": 1}]),
        json!([{"address": hex(&feeder), "budget": 1, "key": "sensitive-value"}]),
        json!([{"address": hex(&feeder), "budget": 1}, {"address": hex(&feeder), "budget": 2}]),
        json!({"address": hex(&feeder), "budget": 1}),
        json!(null),
    ] {
        assert_eq!(
            ServiceConfig::parse(&service_json(&refused).to_string()).err(),
            Some(ConfigError {
                field: "registered_senders"
            })
        );
    }

    let harness = start_configured("REGISTERED", |config, account| {
        config.registered_senders = vec![RegisteredSender {
            address: account,
            budget: 12_000_000,
        }];
    })?;
    harness.recorded.phase("receipted");
    let (status, quote) = harness.post("/quote", &harness.quote_body())?;
    assert_eq!(status, 200);
    assert_eq!(quote["quote"]["tokenAmount"], json!(AMOUNT.to_string()));
    assert_eq!(quote["quote"]["quoteNonce"], json!("1"));
    assert!(harness.recorded.sent().is_empty());
    harness.assert_log_clean(&["/quote 200"]);
    harness.finish()?;
    registered_budgets_cross_caps_and_survive_restart()
}

#[test]
fn status_reports_the_did_gas_budget() -> TestResult {
    let harness = start("DID_STATUS")?;
    let budget_key = format!("did:layerx:{}", &hex(&[0xd1; 32])[2..]);
    let (status, quote) = harness.post("/quote", &harness.quote_body())?;
    assert_eq!(status, 200);
    let identity = harness.status_body(&quote);
    harness.recorded.phase("did_unreachable");
    let mut lines = vec!["/quote 200"];
    for now in [1000, 1020, 86_399, 86_400, 172_800] {
        harness.clock.store(now, Ordering::SeqCst);
        let (status, report) = harness.post("/status", &identity)?;
        assert_eq!(status, 200);
        let used: u64 = if now < 86_400 { 200_000 } else { 0 };
        assert_eq!(
            report,
            json!({
                "state": "quoted",
                "deadline": "1019",
                "submission": null,
                "completion": null,
                "budget": {
                    "key": budget_key,
                    "gasLimit": "1000000",
                    "usedGas": used.to_string(),
                    "remainingGas": (1_000_000 - used).to_string(),
                    "dayEnds": ((now / 86_400 + 1) * 86_400).to_string(),
                },
            }),
            "budget at service time {now}"
        );
        lines.push("/status 200");
    }
    assert!(harness.recorded.sent().is_empty());
    harness.assert_log_clean(&lines);
    harness.finish()
}

#[test]
fn an_empty_or_sidiora_paymaster_is_refused_at_start() {
    for paymaster in [hex(&[0; 20]), hex(&SIDIORA)] {
        let mut value = service_json(&json!([]));
        value["paymaster"] = json!(paymaster);
        assert_eq!(
            ServiceConfig::parse(&value.to_string()).err(),
            Some(ConfigError { field: "paymaster" })
        );
    }
    let required = ConfigError {
        field: "GAS_STATION_PAYMASTER",
    };
    for raw in [
        None,
        Some(""),
        Some("0x"),
        Some("0x0000000000000000000000000000000000000000"),
        Some("0x21f7b20a555199fa73A238B1a91FD0f549068fEe"),
    ] {
        assert_eq!(PaymasterRequired::try_from(raw), Err(required.clone()));
    }
    assert_eq!(
        PaymasterRequired::try_from(Some(hex(&PAYMASTER).as_str())),
        Ok(PaymasterRequired(PAYMASTER))
    );
    for (name, paymaster) in [("EMPTY_PAYMASTER", [0; 20]), ("SID_PAYMASTER", SIDIORA)] {
        let refused = start_configured(name, |config, _| config.station.paymaster = paymaster);
        assert!(refused.is_err());
    }
}

#[test]
fn sequencer_receipt_is_requested_by_transaction_hash() -> TestResult {
    let (harness, _, answer) = sequencer_endpoint_submission("BY_HASH", "held")?;
    let hash = keccak(&harness.recorded.sent()[0]);
    assert_eq!(
        answer,
        (
            200,
            json!({"transactionHash": hex(&hash), "rung": "submitted"})
        )
    );
    let rpc = ConfiguredRpc::new(
        &station_config("BY_HASH"),
        ReplayExchange(Arc::clone(&harness.recorded)),
    )?;
    assert_eq!(
        rpc.sequencer_receipt(&hash)?,
        Some(SequencerReceipt {
            nonce: 5,
            rung: Rung::Submitted
        })
    );
    harness.recorded.phase("refused");
    assert_eq!(rpc.sequencer_receipt(&hash)?, None);
    harness.assert_log_clean(&["/quote 200", "/submit 200"]);
    harness.finish()
}

#[test]
fn a_committed_submission_answers_the_full_sequencer_receipt() -> TestResult {
    let (harness, _, answer) = sequencer_endpoint_submission("FULL_COMMITTED", "full_committed")?;
    let fixture: Value = serde_json::from_str(include_str!("fixtures/sequencer_receipts.json"))?;
    let receipt = &fixture["committed"];
    let mut logs = receipt["logs"].clone();
    logs[0]["logIndex"] = json!("0");
    assert_eq!(
        answer,
        (
            200,
            json!({
                "transactionHash": hex(&keccak(&harness.recorded.sent()[0])),
                "rung": "committed", "seq": "2", "height": "1000", "status": "1",
                "logs": logs, "keyId": receipt["keyId"], "receipt": receipt["receipt"],
                "blockNumber": "1000", "blockHash": receipt["blockHash"],
            })
        )
    );
    harness.assert_log_clean(&["/quote 200", "/submit 200"]);
    harness.finish()
}

#[test]
fn a_receipted_submission_answers_without_block_fields() -> TestResult {
    let (harness, _, answer) = sequencer_endpoint_submission("FULL_RECEIPTED", "full_receipted")?;
    let fixture: Value = serde_json::from_str(include_str!("fixtures/sequencer_receipts.json"))?;
    let receipt = &fixture["receipted"];
    let mut logs = receipt["logs"].clone();
    logs[0]["logIndex"] = json!("0");
    assert_eq!(
        answer,
        (
            200,
            json!({
                "transactionHash": hex(&keccak(&harness.recorded.sent()[0])),
                "rung": "receipted", "seq": "2", "height": "1000", "status": "1",
                "logs": logs, "keyId": receipt["keyId"], "receipt": receipt["receipt"],
            })
        )
    );
    assert!(answer.1.get("blockNumber").is_none());
    assert!(answer.1.get("blockHash").is_none());
    harness.assert_log_clean(&["/quote 200", "/submit 200"]);
    harness.finish()
}

fn set_documented_env(
    relayer_secret: &str,
    owner_secret: &str,
) -> Result<Vec<&'static str>, Box<dyn std::error::Error>> {
    let mut fills: BTreeMap<&str, String> = [
        ("GAS_STATION_ENDPOINTS", "https://sequencer.test".to_owned()),
        ("GAS_STATION_PAYMASTER", hex(&PAYMASTER)),
        ("GAS_STATION_MAX_RATE_AGE", "300".to_owned()),
        ("GAS_STATION_MARGIN_BPS", "100".to_owned()),
        ("GAS_STATION_PER_QUOTE_LIMIT", "4000000".to_owned()),
        ("GAS_STATION_PER_ACCOUNT_LIMIT", "8000000".to_owned()),
        ("GAS_STATION_PER_INTERVAL_LIMIT", "16000000".to_owned()),
        ("GAS_STATION_INTERVAL_SECONDS", "60".to_owned()),
        ("GAS_STATION_BALANCE_FLOOR", "100".to_owned()),
        ("GAS_STATION_GAS_LIMIT", "200000".to_owned()),
        (
            "GAS_STATION_MAX_PRIORITY_FEE_PER_GAS",
            "1000000000".to_owned(),
        ),
        ("GAS_STATION_DID_GAS_LIMIT", "1000000".to_owned()),
        ("GAS_STATION_RATE_GAS_BUDGET_PER_DAY", "2000000".to_owned()),
        ("GAS_STATION_RATE_BALANCE_FLOOR", "100".to_owned()),
        ("GAS_STATION_BROWSER_ORIGINS", "https://app.test".to_owned()),
        ("GAS_STATION_RELAYER_KEY", relayer_secret.to_owned()),
        ("GAS_STATION_RATE_OWNER_KEY", owner_secret.to_owned()),
    ]
    .into_iter()
    .collect();
    let mut listed = Vec::new();
    for line in include_str!("../../../deploy/gas-station/railway.env.example").lines() {
        if line.is_empty() || line.starts_with('#') {
            continue;
        }
        let (key, value) = line.split_once('=').ok_or("env line without '='")?;
        let value = if value.is_empty() || value == "SEQUENCER_RPC_URL" {
            fills
                .remove(key)
                .ok_or_else(|| format!("no test value for {key}"))?
        } else {
            value.to_owned()
        };
        std::env::set_var(key, value);
        listed.push(key);
    }
    assert_eq!(fills.into_keys().collect::<Vec<_>>(), Vec::<&str>::new());
    assert!(listed.contains(&"GAS_STATION_DID_GAS_LIMIT"));
    Ok(listed)
}

fn render_documented_config() -> Result<String, std::env::VarError> {
    let var = |key: &str| std::env::var(key);
    let endpoints = var("GAS_STATION_ENDPOINTS")?
        .split(' ')
        .map(|url| format!("\"{url}\""))
        .collect::<Vec<_>>()
        .join(",");
    Ok(format!(
        r#"{{"listen":"{}","gas_limit":{},"max_priority_fee_per_gas":{},"chain_id":{},"endpoints":[{}],"paymaster":"{}","token":"{}","decimals":{},"max_rate_age":{},"spread_bps":{},"margin_bps":{},"per_account_limit":{},"per_interval_limit":{},"per_quote_limit":{},"interval_seconds":{},"balance_floor":{},"relayer_key_env":"{}"}}"#,
        var("GAS_STATION_LISTEN")?,
        var("GAS_STATION_GAS_LIMIT")?,
        var("GAS_STATION_MAX_PRIORITY_FEE_PER_GAS")?,
        var("GAS_STATION_CHAIN_ID")?,
        endpoints,
        var("GAS_STATION_PAYMASTER")?,
        var("GAS_STATION_TOKEN")?,
        var("GAS_STATION_DECIMALS")?,
        var("GAS_STATION_MAX_RATE_AGE")?,
        var("GAS_STATION_SPREAD_BPS")?,
        var("GAS_STATION_MARGIN_BPS")?,
        var("GAS_STATION_PER_ACCOUNT_LIMIT")?,
        var("GAS_STATION_PER_INTERVAL_LIMIT")?,
        var("GAS_STATION_PER_QUOTE_LIMIT")?,
        var("GAS_STATION_INTERVAL_SECONDS")?,
        var("GAS_STATION_BALANCE_FLOOR")?,
        var("GAS_STATION_RELAYER_KEY_ENV")?,
    ))
}

/// Starts the real service from exactly the inputs
/// services/interop/deploy/gas-station/railway.env.example documents: every key it
/// lists is set, the blanks and the sequencer endpoint reference with test
/// values, the configuration is rendered the way docker/gas-station/init.sh
/// renders it, and the relayer key comes from the variable the file names.
/// Without `GAS_STATION_DID_GAS_LIMIT` the same station refuses to start.
#[test]
fn documented_deploy_inputs_start_service() -> TestResult {
    let _did_env = DID_GAS_LIMIT_ENV
        .lock()
        .unwrap_or_else(PoisonError::into_inner);
    let relayer_secret = hex(&SigningKey::random(&mut OsRng).to_bytes());
    let owner_secret = hex(&SigningKey::random(&mut OsRng).to_bytes());
    let listed = set_documented_env(&relayer_secret, &owner_secret)?;

    let var = |key: &str| std::env::var(key);
    let rendered = render_documented_config()?;
    let config = ServiceConfig::parse(&rendered)?;
    assert_eq!(config.station.relayer_key_env, "GAS_STATION_RELAYER_KEY");

    let signer = LocalSigner::from_config(&config.station)?;
    let sponsor = signer.address();
    let (account, account_secret) = ephemeral_signer(&station_config("DEPLOY"))?;
    let mut fixture =
        with_sequencer_endpoint_rules(serde_json::from_str(include_str!("fixtures/cycle.json"))?)?;
    fixture["phases"]["submit"]
        .as_array_mut()
        .ok_or("fixture without a submit phase")?
        .insert(
            0,
            json!({"method": "eth_chainId", "params": [],
                "result": format!("{:#x}", config.station.chain_id)}),
        );
    let recorded = recording(sponsor, account.address(), fixture)?;
    let journal_path = |role: &str| {
        std::env::temp_dir().join(format!(
            "paxeer-service-deploy-{role}-{}.jsonl",
            std::process::id()
        ))
    };
    let (journal, refused_journal) = (journal_path("served"), journal_path("refused"));
    for path in [&journal, &refused_journal] {
        if path.exists() {
            std::fs::remove_file(path)?;
        }
    }
    let listener = TcpListener::bind("127.0.0.1:0")?;
    let address = listener.local_addr()?;
    let log = SharedLog(Arc::new(Mutex::new(Vec::new())));
    let clock = Arc::new(AtomicU64::new(1000));
    let listener_control = listener.try_clone()?;
    let worker = spawn_service(
        config.clone(),
        signer,
        Arc::clone(&recorded),
        journal.clone(),
        listener,
        Arc::clone(&clock),
        log.clone(),
    )?;
    let harness = Harness {
        address,
        recorded,
        account,
        sponsor,
        secrets: vec![relayer_secret, owner_secret, account_secret],
        log,
        journal,
        clock,
        config: config.clone(),
        listener_control,
        worker: Some(worker),
        did_gas_limit: 1_000_000,
    };
    let (status, body) = harness.exchange(b"GET /healthz HTTP/1.1\r\nHost: station\r\n\r\n")?;
    assert_eq!((status, body), (200, json!({"status": "ok"})));
    harness.assert_log_clean(&["/healthz 200"]);

    let station = || -> Result<_, Box<dyn std::error::Error>> {
        Ok(GasStation::new(
            config.station.clone(),
            LocalSigner::from_config(&config.station)?,
            ConfiguredRpc::new(
                &config.station,
                ReplayExchange(Arc::clone(&harness.recorded)),
            )?,
            PaymasterRateSource::new(
                ConfiguredRpc::new(
                    &config.station,
                    ReplayExchange(Arc::clone(&harness.recorded)),
                )?,
                config.station.paymaster,
            ),
            Journal::open(&refused_journal)?,
        ))
    };
    let documented = var("GAS_STATION_DID_GAS_LIMIT")?;
    std::env::remove_var("GAS_STATION_DID_GAS_LIMIT");
    assert!(matches!(station()?, Err(StationError::Invalid)));
    std::env::set_var("GAS_STATION_DID_GAS_LIMIT", documented);
    assert!(station()?.is_ok());

    for key in listed {
        std::env::remove_var(key);
    }
    std::fs::remove_file(&refused_journal)?;
    harness.finish()
}
