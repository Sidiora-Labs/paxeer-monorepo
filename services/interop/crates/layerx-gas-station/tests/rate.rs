#[path = "support/journal_entries.rs"]
mod journal_entries;
#[path = "support/journal_remove.rs"]
mod journal_remove;

include!("support/rate_harness.rs");
include!("support/rate_repairs.rs");

fn no_secret(value: &Value, secret: &str) {
    match value {
        Value::String(text) => assert_ne!(text.strip_prefix("0x").unwrap_or(text), secret),
        Value::Array(values) => values.iter().for_each(|value| no_secret(value, secret)),
        Value::Object(values) => values.values().for_each(|value| no_secret(value, secret)),
        _ => {}
    }
}

#[test]
fn publishes_the_owner_rate_with_set_rate_and_journals_it() -> TestResult {
    let lane = node_recording::Lane::new("original-publication", "success_pre")?;
    let expected = lane.tape.transaction("success")?;
    let file = lane.file(3_400_000)?;
    let mut publisher = lane.publisher()?;
    let publication = publisher.publish(&file)?;
    assert_eq!(publication.owner, expected.publication.owner);
    assert_eq!(publication.nonce, 4);
    assert_eq!(publication.rate, word(3_400_000));
    assert_eq!(publication.gas_limit, RATE_GAS_LIMIT);
    assert_eq!(publication.signed_at, expected.publication.signed_at);
    let raw = lane.tape.sent()[0].clone();
    assert_eq!(raw, expected.raw);
    assert_eq!(publication.hash, keccak(&raw));
    assert_eq!(
        published_rate(&raw)?,
        (
            lane.tape.config()?.station.paymaster,
            word(3_400_000),
            publication.owner
        )
    );
    lane.tape.phase("success_committed");
    publisher.recover_pending()?;
    let state = publisher.journal().state().clone();
    let (journalled, settlement) = state.publications[&publication.hash];
    assert_eq!(journalled, publication);
    assert_eq!(
        settlement.map(|s| (s.gas_used, s.succeeded)),
        Some((35_098, true))
    );
    assert_eq!(
        state.publication_gas(publication.signed_at / 86_400),
        35_098
    );
    drop(publisher);
    let reopened = Journal::open(&lane.dir.join("rate.jsonl"))?;
    assert_eq!(reopened.state(), &state);
    let lines = std::fs::read_to_string(lane.dir.join("rate.jsonl"))?;
    let entries: Vec<Entry> = lines
        .lines()
        .map(journal_entries::entry)
        .collect::<Result<_, _>>()?;
    assert_eq!(entries.len(), 3);
    assert!(
        matches!(&entries[0], Entry::RatePrepared { transaction } if transaction.publication == publication && transaction.raw == raw)
    );
    assert_eq!(
        entries[1],
        Entry::RateBroadcast {
            hash: publication.hash
        }
    );
    assert!(
        matches!(&entries[2], Entry::RateFinalized { hash, settlement, .. } if *hash == publication.hash && settlement.succeeded)
    );
    // The captured producer uses test scalar 1. That public 32-byte value also
    // occurs inside ABI words and receipt signatures, so compare complete
    // persisted scalar values rather than substrings of opaque public evidence.
    for entry in &entries {
        no_secret(&serde_json::to_value(entry)?, &format!("{:064x}", 1));
    }
    Ok(())
}

#[test]
fn refusals_never_publish() -> TestResult {
    let lane = Lane::new("refusals")?;
    let valid = "rate = 3200000\nset_at = 1899000000\n";
    let mut publisher = lane.publisher(1_000_000)?;
    for (text, refusal) in [
        (None, RateRefusal::RateFileMissing),
        (Some("rate = 3200000\n"), RateRefusal::RateFileMalformed),
        (
            Some("rate = 1\nrate = 2\nset_at = 1\n"),
            RateRefusal::RateFileMalformed,
        ),
        (
            Some("rate = 1\nset_at = 1\nsid_usd = 1\n"),
            RateRefusal::RateFileMalformed,
        ),
        (
            Some("rate = \"3200000\"\nset_at = 1\n"),
            RateRefusal::RateFileMalformed,
        ),
        (
            Some("rate = 1\nset_at = 0\n"),
            RateRefusal::RateFileMalformed,
        ),
        (
            Some("rate = 1\nset_at = 5\nnot_after = 5\n"),
            RateRefusal::RateFileMalformed,
        ),
        (Some("rate = 0\nset_at = 1\n"), RateRefusal::ZeroRate),
        (
            Some("rate = 1\nset_at = 1900000001\n"),
            RateRefusal::NotYetSet,
        ),
        (
            Some("rate = 1\nset_at = 1\nnot_after = 1900000000\n"),
            RateRefusal::Expired,
        ),
        (
            Some("rate = 3114000\nset_at = 1\n"),
            RateRefusal::Unchanged { age: 119 },
        ),
    ] {
        let path = if let Some(text) = text {
            lane.rate_file(text)?
        } else {
            let path = lane.dir.join("rate.toml");
            let _ = std::fs::remove_file(&path);
            path
        };
        if refusal == (RateRefusal::Unchanged { age: 119 }) {
            lane.chain.borrow_mut().updated_at = NOW - 119;
        }
        assert_eq!(publisher.publish(&path), Err(refusal), "{text:?}");
    }
    let file = lane.rate_file(valid)?;
    lane.chain.borrow_mut().owner = [0x99; 20];
    assert_eq!(publisher.publish(&file), Err(RateRefusal::NotOwner));
    lane.chain.borrow_mut().owner = lane.owner;
    lane.chain.borrow_mut().balance = 10_u128.pow(15) - 1;
    assert_eq!(publisher.publish(&file), Err(RateRefusal::BelowFloor));
    assert_eq!(lane.sent(), 0);
    assert!(publisher.journal().state().publications.is_empty());
    lane.chain.borrow_mut().balance = 10_u128.pow(18);
    lane.chain.borrow_mut().status = 0;
    assert_eq!(publisher.publish(&file), Err(RateRefusal::Reverted));
    assert_eq!(lane.sent(), 1);
    Ok(())
}

#[test]
fn daily_gas_budget_stops_publishing() -> TestResult {
    let lane = Lane::new("budget")?;
    let file = lane.rate_file("rate = 3200000\nset_at = 1899000000\n")?;
    let mut publisher = lane.publisher(u128::from(RATE_GAS_LIMIT) + 35_000 - 1)?;
    publisher.publish(&file)?;
    lane.chain.borrow_mut().nonce = 8;
    lane.chain.borrow_mut().clock += 120;
    assert_eq!(publisher.publish(&file), Err(RateRefusal::BudgetExhausted));
    assert_eq!(lane.sent(), 1);
    drop(publisher);
    let mut publisher = lane.publisher(u128::from(RATE_GAS_LIMIT) + 35_000)?;
    publisher.publish(&file)?;
    assert_eq!(lane.sent(), 2);
    Ok(())
}

#[test]
fn cadence_must_stay_below_max_rate_age() -> TestResult {
    for (name, value) in [
        ("rate_cadence_seconds", json!(300)),
        ("rate_cadence_seconds", json!(301)),
        ("rate_cadence_seconds", json!(0)),
        ("rate_owner_key_env", json!("GAS_STATION_RELAYER_KEY")),
        ("rate_owner_key_env", json!("sensitive-value")),
        ("rate_gas_budget_per_day", json!(RATE_GAS_LIMIT - 1)),
        ("rate_balance_floor", json!(0)),
        ("rate_max_fee_per_gas", json!(1_000_000_000)),
        ("paymaster", json!(hex(&FEE_TOKEN))),
    ] {
        let mut value_map = config_json();
        value_map[name] = value;
        assert_eq!(
            PublisherConfig::parse(&value_map.to_string()).err(),
            Some(ConfigError { field: name })
        );
    }
    let mut bounded = config_json();
    bounded["rate_cadence_seconds"] = json!(239);
    assert_eq!(
        PublisherConfig::parse(&bounded.to_string())?.cadence_seconds,
        239
    );
    bounded["rate_cadence_seconds"] = json!(240);
    assert_eq!(
        PublisherConfig::parse(&bounded.to_string()).err(),
        Some(ConfigError {
            field: "rate_confirmation_retry_seconds"
        })
    );
    bounded["rate_cadence_seconds"] = json!(299);
    assert_eq!(
        PublisherConfig::parse(&bounded.to_string()).err(),
        Some(ConfigError {
            field: "rate_confirmation_retry_seconds"
        })
    );
    bounded["max_rate_age"] = json!(120);
    bounded["rate_cadence_seconds"] = json!(120);
    assert_eq!(
        PublisherConfig::parse(&bounded.to_string()).err(),
        Some(ConfigError {
            field: "rate_cadence_seconds"
        })
    );
    let Value::Object(fields) = config_json() else {
        return Err("object required".into());
    };
    for name in fields.keys() {
        let mut incomplete = fields.clone();
        incomplete.remove(name);
        let result = PublisherConfig::parse(&Value::Object(incomplete).to_string());
        assert_eq!(result.err().map(|e| e.field.to_owned()), Some(name.clone()));
    }
    Ok(())
}

#[test]
fn rate_file_reads_the_owner_rate() {
    assert_eq!(
        RateFile::parse("# SID base units per whole PAX\nrate = 3_114_000\nset_at = 1899000000\nnot_after = 1999000000\n"),
        Ok(RateFile {
            rate: 3_114_000,
            set_at: 1_899_000_000,
            not_after: Some(1_999_000_000),
        })
    );
}

/// The most the owner pays for one publication at a 1 gwei base fee and a
/// 1 gwei tip: (2 * 1 gwei + 1 gwei) * 60000.
const RESERVE: u128 = 3_000_000_000 * RATE_GAS_LIMIT as u128;

#[test]
fn fee_above_the_ceiling_is_refused_unsigned() -> TestResult {
    let lane = Lane::new("ceiling")?;
    let file = lane.rate_file("rate = 3200000\nset_at = 1899000000\n")?;
    let mut publisher = lane.publisher_with(1_000_000, 2_999_999_999)?;
    assert_eq!(
        publisher.publish(&file),
        Err(RateRefusal::FeeAboveCeiling {
            required: 3_000_000_000,
            ceiling: 2_999_999_999,
        })
    );
    lane.chain.borrow_mut().balance = u128::MAX;
    assert!(matches!(
        publisher.publish(&file),
        Err(RateRefusal::FeeAboveCeiling { .. })
    ));
    assert_eq!(lane.sent(), 0);
    assert!(publisher.journal().state().publications.is_empty());
    drop(publisher);
    let mut publisher = lane.publisher_with(1_000_000, 3_000_000_000)?;
    assert_eq!(publisher.publish(&file)?.max_fee_per_gas, 3_000_000_000);
    assert_eq!(lane.sent(), 1);
    Ok(())
}

#[test]
fn unsettled_publications_reserve_their_most_against_the_floor() -> TestResult {
    let lane = Lane::new("reserve")?;
    let file = lane.rate_file("rate = 3200000\nset_at = 1899000000\n")?;
    let legacy = Publication {
        owner: lane.owner,
        nonce: 7,
        hash: word(0xabc),
        rate: word(3_114_000),
        gas_limit: RATE_GAS_LIMIT,
        max_fee_per_gas: 4_000_000_000,
        signed_at: NOW - 60,
    };
    let mut journal = Journal::open(&lane.dir.join("rate.jsonl"))?;
    journal.append(&Entry::RatePublished {
        publication: legacy,
    })?;
    assert_eq!(journal.state().reserved_wei(), 4_000_000_000 * 60_000);
    drop(journal);
    let refused = lane
        .publisher(1_000_000)
        .err()
        .ok_or("legacy journal accepted")?;
    assert_eq!(
        refused.downcast_ref::<RateRefusal>(),
        Some(&RateRefusal::LegacyUnresolved)
    );
    journal_remove::remove(&lane.dir.join("rate.jsonl"))?;
    let floor = 10_u128.pow(15);
    lane.chain.borrow_mut().rung = "receipted";
    lane.chain.borrow_mut().balance = floor + RESERVE - 1;
    let mut publisher = lane.publisher(1_000_000)?;
    assert_eq!(publisher.publish(&file), Err(RateRefusal::BelowFloor));
    assert_eq!(lane.sent(), 0);
    lane.chain.borrow_mut().balance = floor + RESERVE;
    let publication = publisher.publish(&file)?;
    assert_eq!(publication.nonce, 7);
    let state = publisher.journal().state();
    assert_eq!(state.unsettled(), vec![publication]);
    assert_eq!(state.reserved_wei(), RESERVE);
    assert_eq!(
        state.publication_gas(NOW / 86_400),
        u128::from(RATE_GAS_LIMIT)
    );
    assert_eq!(state.publications[&publication.hash].1, None);
    assert_eq!(
        (lane.chain.borrow().rate, lane.chain.borrow().updated_at),
        (3_200_000, NOW)
    );
    lane.chain.borrow_mut().rung = "committed";
    assert_eq!(
        publisher.publish(&file),
        Err(RateRefusal::Unchanged { age: 0 })
    );
    assert_eq!(lane.sent(), 1);
    let state = publisher.journal().state();
    assert_eq!(state.publications[&publication.hash].0, publication);
    assert!(state.unsettled().is_empty());
    assert_eq!(state.reserved_wei(), 0);
    assert_eq!(
        state.publications[&publication.hash].1.map(|s| s.cost_wei),
        Some(1_500_000_000 * 35_000)
    );
    Ok(())
}

#[test]
fn settlement_records_the_actual_cost() -> TestResult {
    let lane = Lane::new("cost")?;
    let file = lane.rate_file("rate = 3200000\nset_at = 1899000000\n")?;
    let mut publisher = lane.publisher(1_000_000)?;
    let publication = publisher.publish(&file)?;
    let state = publisher.journal().state().clone();
    let settlement = state.publications[&publication.hash]
        .1
        .ok_or("publication unsettled")?;
    assert_eq!(settlement.cost_wei, 1_500_000_000 * 35_000);
    assert_eq!(state.publication_wei(NOW / 86_400), 1_500_000_000 * 35_000);
    assert_eq!(state.publication_gas(NOW / 86_400), 35_000);
    assert_eq!(state.reserved_wei(), 0);
    drop(publisher);
    assert_eq!(Journal::open(&lane.dir.join("rate.jsonl"))?.state(), &state);
    lane.chain.borrow_mut().effective_gas_price = 3_000_000_001;
    lane.chain.borrow_mut().nonce = 8;
    lane.chain.borrow_mut().updated_at = NOW - 300;
    let mut publisher = lane.publisher(1_000_000)?;
    assert_eq!(
        publisher.publish(&file),
        Err(RateRefusal::Rpc(RpcFault::Malformed))
    );
    assert_eq!(publisher.journal().state().unsettled().len(), 1);
    Ok(())
}

#[test]
fn cadence_equal_to_max_rate_age_is_refused_at_load() -> TestResult {
    use std::os::unix::fs::PermissionsExt as _;
    let lane = Lane::new("load")?;
    let path = lane.dir.join("publisher.json");
    let mut value = config_json();
    value["rate_cadence_seconds"] = json!(300);
    std::fs::write(&path, value.to_string())?;
    std::fs::set_permissions(&path, std::fs::Permissions::from_mode(0o600))?;
    assert_eq!(
        PublisherConfig::load(&path),
        Err(ConfigError {
            field: "rate_cadence_seconds"
        })
    );
    value["rate_cadence_seconds"] = json!(120);
    value["max_rate_age"] = json!(301);
    std::fs::write(&path, value.to_string())?;
    assert_eq!(
        PublisherConfig::load(&path),
        Err(ConfigError {
            field: "max_rate_age"
        })
    );
    value["max_rate_age"] = json!(300);
    value["rate_cadence_seconds"] = json!(240);
    std::fs::write(&path, value.to_string())?;
    assert_eq!(
        PublisherConfig::load(&path),
        Err(ConfigError {
            field: "rate_confirmation_retry_seconds"
        })
    );
    value["rate_cadence_seconds"] = json!(298);
    value["rate_confirmation_retry_seconds"] = json!(1);
    std::fs::write(&path, value.to_string())?;
    assert_eq!(PublisherConfig::load(&path)?.cadence_seconds, 298);
    Ok(())
}

#[test]
fn publication_waits_for_the_sequencer_receipt_and_is_never_resent_at_its_nonce() -> TestResult {
    let lane = Lane::new("unconfirmed")?;
    let file = lane.rate_file("rate = 3200000\nset_at = 1899000000\n")?;
    lane.chain.borrow_mut().rung = "submitted";
    let mut publisher = lane.publisher_confirming_within(1)?;
    assert_eq!(publisher.publish(&file), Err(RateRefusal::ReceiptTimeout));
    assert_eq!(lane.sent(), 1);
    let raw = lane.chain.borrow().sent[0].clone();
    let hash = keccak(&raw);
    let state = publisher.journal().state();
    assert_eq!(state.rate_active.get(&(lane.owner, 7)), Some(&hash));
    assert_eq!(state.rate_transactions.len(), 1);
    assert_eq!(state.rate_broadcast.len(), 1);
    assert_eq!(state.unsettled().len(), 1);
    assert_eq!(publisher.publish(&file), Err(RateRefusal::ReceiptTimeout));
    assert_eq!(lane.sent(), 1);
    assert_eq!(publisher.journal().state().rate_transactions.len(), 1);
    lane.chain.borrow_mut().rung = "committed";
    assert_eq!(
        publisher.publish(&file),
        Err(RateRefusal::Unchanged { age: 0 })
    );
    assert_eq!(lane.sent(), 1);
    let state = publisher.journal().state();
    let publication = state.publications[&hash].0;
    assert_eq!((publication.nonce, publication.hash), (7, hash));
    assert!(state.rate_active.is_empty());
    assert_eq!(
        state.publications[&hash]
            .1
            .map(|s| (s.gas_used, s.succeeded)),
        Some((35_000, true))
    );
    lane.chain.borrow_mut().clock += 120;
    let next = publisher.publish(&file)?;
    assert_eq!(next.nonce, 8);
    assert_eq!(lane.sent(), 2);
    Ok(())
}

#[test]
fn publication_unknown_to_the_sequencer_is_resent_as_its_exact_bytes() -> TestResult {
    let lane = Lane::new("unknown")?;
    let file = lane.rate_file("rate = 3200000\nset_at = 1899000000\n")?;
    lane.chain.borrow_mut().rung = "unknown";
    let mut publisher = lane.publisher_confirming_within(1)?;
    assert_eq!(
        publisher.publish(&file),
        Err(RateRefusal::Rpc(RpcFault::Rejected { code: -32000 }))
    );
    assert_eq!(lane.sent(), 1);
    assert!(publisher.journal().state().rate_broadcast.is_empty());
    lane.chain.borrow_mut().rung = "receipted";
    assert_eq!(
        publisher.publish(&file),
        Err(RateRefusal::Unchanged { age: 0 })
    );
    let sent = lane.chain.borrow().sent.clone();
    assert_eq!(sent.len(), 2);
    assert_eq!(sent[0], sent[1]);
    let state = publisher.journal().state();
    let publication = state.publications[&keccak(&sent[0])].0;
    assert_eq!(publication.hash, keccak(&sent[0]));
    assert_eq!(publication.nonce, 7);
    assert_eq!(state.rate_transactions.len(), 1);
    assert!(state.rate_broadcast.contains(&publication.hash));
    assert_eq!(state.unsettled(), vec![publication]);
    Ok(())
}

#[test]
fn publisher_targets_only_the_paymaster_never_the_fee_token() -> TestResult {
    let lane = Lane::new("targets")?;
    let file = lane.rate_file("rate = 3114000\nset_at = 1899000000\n")?;
    let mut fee_token = config_json();
    fee_token["paymaster"] = json!(hex(&FEE_TOKEN));
    assert_eq!(
        PublisherConfig::parse(&fee_token.to_string()).err(),
        Some(ConfigError { field: "paymaster" })
    );
    lane.chain.borrow_mut().updated_at = NOW - 120;
    let mut publisher = lane.publisher(1_000_000)?;
    let publication = publisher.publish(&file)?;
    assert_eq!(publication.rate, word(3_114_000));
    let sent = lane.chain.borrow().sent.clone();
    assert_eq!(sent.len(), 1);
    for raw in &sent {
        let (to, rate, from) = published_rate(raw)?;
        assert_eq!((to, rate, from), (PAYMASTER, word(3_114_000), lane.owner));
        assert_ne!(to, FEE_TOKEN);
    }
    Ok(())
}

#[test]
fn real_signed_v2_publication_replays_exactly_and_refuses_identity_corruption() -> TestResult {
    use layerx_gas_station::journal::PublicationTransaction;
    use layerx_gas_station::tx::{sign_call, Fees};
    use std::io::Write as _;
    use std::os::unix::fs::{MetadataExt as _, OpenOptionsExt as _, PermissionsExt as _};

    let directory =
        std::env::temp_dir().join(format!("paxeer-rate-authenticated-{}", std::process::id()));
    std::fs::create_dir(&directory)?;
    std::fs::set_permissions(&directory, std::fs::Permissions::from_mode(0o700))?;
    let owner = signer(&SigningKey::random(&mut OsRng).to_bytes())?;
    let fees = Fees {
        gas_limit: RATE_GAS_LIMIT,
        max_fee_per_gas: 5_000_000_000,
        max_priority_fee_per_gas: 1_000_000_000,
    };
    let rate = word(3_200_000);
    let data = [&keccak(b"setRate(uint256)")[..4], rate.as_slice()].concat();
    let signed = sign_call(125, 7, fees, PAYMASTER, &data, &owner)?;
    let generated = PublicationTransaction {
        version: 2,
        publication: Publication {
            owner: owner.address(),
            nonce: 7,
            hash: signed.hash,
            rate,
            gas_limit: fees.gas_limit,
            max_fee_per_gas: fees.max_fee_per_gas,
            signed_at: NOW,
        },
        chain_id: 125,
        paymaster: PAYMASTER,
        max_priority_fee_per_gas: fees.max_priority_fee_per_gas,
        raw: signed.raw,
        cancellation: false,
    };
    let input = directory.join("transaction.json");
    let mut fixture = std::fs::OpenOptions::new()
        .write(true)
        .create_new(true)
        .mode(0o600)
        .open(&input)?;
    fixture.write_all(&serde_json::to_vec(&generated)?)?;
    drop(fixture);
    let metadata = std::fs::symlink_metadata(&input)?;
    if !input.is_absolute()
        || !metadata.is_file()
        || metadata.mode() & 0o077 != 0
        || metadata.nlink() != 1
        || metadata.len() > 16_384
        || metadata.uid() != std::fs::metadata("/proc/self")?.uid()
        || input.components().any(|component| {
            component
                .as_os_str()
                .to_str()
                .is_some_and(|part| part == ".env" || part.starts_with(".env."))
        })
    {
        return Err("protected genuine publisher transaction required".into());
    }
    let transaction: PublicationTransaction = serde_json::from_slice(&std::fs::read(&input)?)?;
    assert_eq!(transaction, generated);
    let path = directory.join("rate.jsonl");
    let mut journal = Journal::open(&path)?;
    let entry = Entry::RatePrepared {
        transaction: transaction.clone(),
    };
    journal.append(&entry)?;
    let durable = std::fs::read(&path)?;
    journal.append(&entry)?;
    assert_eq!(std::fs::read(&path)?, durable);
    let state = journal.state().clone();
    for field in ["chain", "paymaster", "owner", "nonce", "rate", "fee", "raw"] {
        let mut changed = transaction.clone();
        match field {
            "chain" => changed.chain_id ^= 1,
            "paymaster" => changed.paymaster[0] ^= 1,
            "owner" => changed.publication.owner[0] ^= 1,
            "nonce" => changed.publication.nonce ^= 1,
            "rate" => changed.publication.rate[0] ^= 1,
            "fee" => changed.publication.max_fee_per_gas ^= 1,
            _ => changed.raw[0] ^= 1,
        }
        assert!(
            journal
                .append(&Entry::RatePrepared {
                    transaction: changed
                })
                .is_err(),
            "{field}"
        );
        assert_eq!(journal.state(), &state);
        assert_eq!(std::fs::read(&path)?, durable);
    }
    drop(journal);
    assert_eq!(Journal::open(&path)?.state(), &state);
    journal_remove::remove(&path)?;
    std::fs::remove_file(input)?;
    std::fs::remove_dir(directory)?;
    Ok(())
}

#[test]
fn receipt_signature_is_verified_against_pinned_receipt_keys() -> TestResult {
    let refused = Some(ConfigError {
        field: "sequencer_receipt_keys",
    });
    let mut identity = [0_u8; 32];
    identity[0] = 1;
    let mut above_prime = [0xff_u8; 32];
    above_prime[0] = 0xef;
    above_prime[31] = 0x7f;
    let pinned = receipt_key_entry(RECEIPT_KEY_ID, 1, 256);
    let with = |name: &str, value: Value| {
        let mut entry = pinned.clone();
        entry[name] = value;
        entry
    };
    for keys in [
        json!([]),
        json!({}),
        json!([pinned.clone(), receipt_key_entry(RECEIPT_KEY_ID, 300, 400)]),
        json!([receipt_key_entry(RECEIPT_KEY_ID, 257, 256)]),
        json!([receipt_key_entry([0; 32], 1, 256)]),
        json!([with("publicKey", json!(STANDARD.encode(identity)))]),
        json!([with("publicKey", json!(STANDARD.encode(above_prime)))]),
        json!([with("publicKey", json!(STANDARD.encode([0x42_u8; 31])))]),
        json!([with("keyId", json!(hex(&RECEIPT_KEY_ID)))]),
        json!([with("firstHeight", json!("01"))]),
        json!([with("lastHeight", json!(-1))]),
        json!([with("sequencerId", json!(STANDARD.encode(RECEIPT_KEY_ID)))]),
    ] {
        let mut value = config_json();
        value["sequencer_receipt_keys"] = keys.clone();
        assert_eq!(
            PublisherConfig::parse(&value.to_string()).err(),
            refused,
            "{keys}"
        );
    }
    let mut rotated = config_json();
    rotated["sequencer_receipt_keys"] = json!([
        receipt_key_entry([0x5b; 32], 1, 15),
        receipt_key_entry(RECEIPT_KEY_ID, 16, 256)
    ]);
    let keys = PublisherConfig::parse(&rotated.to_string())?.sequencer_receipt_keys;
    assert_eq!(
        keys.iter()
            .map(|key| (
                key.sequencer_id(),
                key.public_key(),
                key.first_batch_number(),
                key.last_batch_number()
            ))
            .collect::<Vec<_>>(),
        vec![
            ([0x5b; 32], receipt_key().verifying_key().to_bytes(), 1, 15),
            (
                RECEIPT_KEY_ID,
                receipt_key().verifying_key().to_bytes(),
                16,
                256
            )
        ]
    );

    let lane = Lane::new("receipt-keys")?;
    let file = lane.rate_file("rate = 3200000\nset_at = 1899000000\n")?;
    lane.chain.borrow_mut().rung = "receipted";
    let mut publisher = lane.publisher(1_000_000)?;
    for tamper in [
        Tamper::Signature,
        Tamper::UnpinnedKeyId,
        Tamper::HeightOutsideRange,
        Tamper::OtherActivity,
        Tamper::Missing,
    ] {
        lane.chain.borrow_mut().tamper = tamper;
        assert_eq!(
            publisher.publish(&file),
            Err(RateRefusal::Rpc(RpcFault::Malformed)),
            "{tamper:?}"
        );
        assert_eq!(lane.sent(), 1, "{tamper:?}");
        let hash = keccak(&lane.chain.borrow().sent[0]);
        let state = publisher.journal().state();
        assert_eq!(state.rate_active.get(&(lane.owner, 7)), Some(&hash));
        assert_eq!(state.unsettled().len(), 1);
        assert_eq!(state.publications[&hash].1, None);
    }
    lane.chain.borrow_mut().tamper = Tamper::None;
    assert_eq!(
        publisher.publish(&file),
        Err(RateRefusal::Unchanged { age: 0 })
    );
    assert_eq!(lane.sent(), 1);
    let hash = keccak(&lane.chain.borrow().sent[0]);
    let state = publisher.journal().state();
    let publication = state.publications[&hash].0;
    assert_eq!((publication.nonce, publication.hash), (7, hash));
    assert_eq!(state.unsettled(), vec![publication]);
    Ok(())
}

#[test]
fn renewal_tick_is_not_spent_on_finalizing_the_previous_publication() -> TestResult {
    const CADENCE: u64 = 239;
    const MAX_RATE_AGE: u64 = 300;
    let rate_age = |lane: &Lane| {
        let chain = lane.chain.borrow();
        chain.clock - chain.updated_at
    };
    for (name, renewed) in [("renewal", 3_114_000_u128), ("renewal-changed", 3_200_000)] {
        let lane = Lane::new(name)?;
        let file = lane.rate_file("rate = 3_114_000\nset_at = 1_899_000_000\n")?;
        let mut value = config_json();
        value["rate_cadence_seconds"] = json!(CADENCE);
        value["rate_confirmation_retry_seconds"] = json!(60);
        value["max_rate_age"] = json!(MAX_RATE_AGE);
        let mut publisher = lane.publisher_from(&value, Duration::ZERO)?;
        lane.chain.borrow_mut().rung = "receipted";
        lane.chain.borrow_mut().updated_at = NOW - CADENCE;
        assert!(rate_age(&lane) < MAX_RATE_AGE, "{name}");

        let tick = publisher.publish(&file);
        let wait = next_wait(CADENCE, &tick);
        let first = tick?;
        assert_eq!(wait, CADENCE, "{name}");
        assert_eq!(
            (first.nonce, first.signed_at, first.rate),
            (7, NOW, word(3_114_000)),
            "{name}"
        );
        assert_eq!(lane.sent(), 1, "{name}");
        let state = publisher.journal().state();
        assert_eq!(state.unsettled(), vec![first], "{name}");
        assert_eq!(state.publications[&first.hash].1, None, "{name}");
        assert_eq!(state.reserved_wei(), RESERVE, "{name}");
        assert_eq!(rate_age(&lane), 0, "{name}");

        lane.chain.borrow_mut().rung = "committed";
        lane.chain.borrow_mut().clock += wait;
        if renewed != 3_114_000 {
            lane.rate_file(&format!("rate = {renewed}\nset_at = 1_899_000_000\n"))?;
        }
        assert!(rate_age(&lane) < MAX_RATE_AGE, "{name}");
        let tick = publisher.publish(&file);
        assert_eq!(next_wait(CADENCE, &tick), CADENCE, "{name}");
        let second = tick?;
        assert_eq!(
            (second.nonce, second.signed_at, second.rate),
            (8, NOW + CADENCE, word(renewed)),
            "{name}"
        );
        assert_eq!(lane.sent(), 2, "{name}");
        assert!(second.signed_at - first.signed_at <= CADENCE, "{name}");
        let raw = lane.chain.borrow().sent[1].clone();
        assert_eq!(second.hash, keccak(&raw), "{name}");
        assert_eq!(
            published_rate(&raw)?,
            (PAYMASTER, word(renewed), lane.owner),
            "{name}"
        );
        let state = publisher.journal().state();
        assert!(state.unsettled().is_empty(), "{name}");
        assert_eq!(state.reserved_wei(), 0, "{name}");
        assert_eq!(
            state.publications[&first.hash]
                .1
                .map(|s| (s.cost_wei, s.succeeded)),
            Some((1_500_000_000 * 35_000, true)),
            "{name}"
        );
        assert_eq!(lane.chain.borrow().rate, renewed, "{name}");
        assert_eq!(rate_age(&lane), 0, "{name}");
    }
    Ok(())
}

#[test]
fn publication_receipts_carry_node_execution_effects() -> TestResult {
    publication_success_commit_and_effects()
}

#[test]
fn persisted_pending_publication_recovers_without_resigning() -> TestResult {
    publication_pending_restart_and_commit()
}

#[test]
fn failed_publication_preserves_node_effects() -> TestResult {
    publication_revert_and_effects()
}

#[test]
fn publication_refuses_changes_to_the_unsigned_execution_view() -> TestResult {
    publication_view_only_tampering_is_refused()
}
