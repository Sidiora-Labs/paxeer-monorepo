mod service_recording {
    include!("service_recording.rs");
}

fn producer_revert_harness() -> Result<Harness, Box<dyn std::error::Error>> {
    let tape = service_recording::Tape::load()?;
    let config = tape.config()?;
    let captured = &tape.fixture["service"];
    let signer = service_recording::signer(3)?;
    let account = service_recording::signer(4)?;
    let sponsor = signer.address();
    let recorded = Arc::new(Recording {
        fixture: json!({"phases": {"submit": tape.fixture["phases"]["service"], "*": []}}),
        phase: Mutex::new("submit".into()),
        bindings: Mutex::new(BTreeMap::new()),
        sent: Mutex::new(vec![]),
    });
    let listener = TcpListener::bind("127.0.0.1:0")?;
    let address = listener.local_addr()?;
    let listener_control = listener.try_clone()?;
    let config = ServiceConfig {
        station: config.station,
        listen: address,
        gas_limit: 200_000,
        max_priority_fee_per_gas: 1_000_000_000,
        registered_senders: vec![],
    };
    let journal = std::env::temp_dir().join(format!(
        "producer-reverted-{}-{}.jsonl",
        std::process::id(),
        std::time::SystemTime::now()
            .duration_since(std::time::UNIX_EPOCH)?
            .as_nanos()
    ));
    let clock = Arc::new(AtomicU64::new(
        captured["now"].as_u64().ok_or("captured service clock")?,
    ));
    let log = SharedLog(Arc::new(Mutex::new(vec![])));
    let _guard = DID_GAS_LIMIT_ENV
        .lock()
        .unwrap_or_else(PoisonError::into_inner);
    std::env::set_var("GAS_STATION_DID_GAS_LIMIT", "1000000000");
    let worker = spawn_service(
        config.clone(),
        signer,
        Arc::clone(&recorded),
        journal.clone(),
        listener,
        Arc::clone(&clock),
        log.clone(),
    )?;
    Ok(Harness {
        address,
        recorded,
        account,
        sponsor,
        secrets: vec![format!("{:064x}", 3), format!("{:064x}", 4)],
        log,
        journal,
        clock,
        config,
        listener_control,
        worker: Some(worker),
        did_gas_limit: 1_000_000_000,
    })
}

pub(crate) fn reverted_receipt_survives_http_status_and_restart() -> TestResult {
    let tape = service_recording::Tape::load()?;
    let capture = &tape.fixture["service"];
    let mut harness = producer_revert_harness()?;
    let (status, quote) = harness.post("/quote", &capture["quote_body"])?;
    assert_eq!(status, 200);
    assert_eq!(quote, capture["quote"]);
    assert_eq!(
        harness.post("/submit", &capture["submission"])?,
        (422, json!({"error": "refused"}))
    );
    let raw = harness.recorded.sent();
    assert_eq!(raw.len(), 1);
    assert_eq!(hex(&keccak(&raw[0])), capture["receipt"]["txHash"]);
    let (status, before) = harness.post("/status", &capture["status_body"])?;
    assert_eq!(status, 200);
    assert_eq!(before, capture["status"]);
    assert_eq!(
        before["completion"],
        json!({"outcome": "reverted", "rung": "receipted", "transactionHash": capture["receipt"]["txHash"]})
    );
    harness.assert_log_clean(&["/quote 200", "/submit 422", "/status 200"]);
    harness.restart()?;
    assert_eq!(
        harness.post("/status", &capture["status_body"])?,
        (200, before)
    );
    assert_eq!(
        harness.post("/submit", &capture["submission"])?,
        (422, json!({"error": "refused"}))
    );
    assert_eq!(harness.recorded.sent(), raw);
    harness.finish()
}

#[test]
pub(crate) fn producer_revert_retains_signed_effects_and_malformed_checks() -> TestResult {
    use layerx_gas_station::rpc::decode_full_sequencer_receipt;
    let tape = service_recording::Tape::load()?;
    let answer = &tape.fixture["service"]["receipt"];
    let hash = bytes(answer["txHash"].as_str().ok_or("missing node hash")?)?
        .try_into()
        .map_err(|_| "hash width")?;
    let decoded = decode_full_sequencer_receipt(answer, &hash)?;
    assert_eq!(decoded.status, Some(0));
    assert_eq!(decoded.sponsor, None);
    assert_eq!(answer["sponsor"], hex(&[0; 20]));
    assert_eq!(decoded.vm_error.as_deref(), Some("execution reverted"));
    let config = tape.config()?;
    let wire = layerx_proof::receipt::verify_sequencer_signature(
        &decoded.receipt,
        config.sequencer_receipt_keys.as_slice(),
    )
    .map_err(|e| format!("node signature: {e:?}"))?;
    assert_eq!(
        layerx_wire::receipt::encode(&wire).map_err(|e| format!("canonical receipt: {e:?}"))?,
        decoded.receipt
    );
    let logs = layerx_proof::sequencer_logs::verify_logs(&wire, vec![])
        .map_err(|e| format!("node effects: {e:?}"))?;
    assert_eq!(
        logs.summary().gas_used(),
        decoded.gas_used.ok_or("gas used")?
    );
    let protocol = wire.protocol().ok_or("missing protocol")?;
    assert_eq!(protocol.activity_id(), hash);
    assert_eq!(protocol.effects().len(), 1);
    assert_eq!(protocol.effects()[0].kind(), 1);
    assert_eq!(protocol.effects()[0].module_id(), protocol.module_id());
    assert!(!layerx_gas_station::rate::verify_sequencer_view(
        answer,
        &hash,
        &config.sequencer_receipt_keys
    )?);
    for sponsor in [Value::Null, json!("0x00"), json!("0xzz")] {
        let mut invalid = answer.clone();
        invalid["sponsor"] = sponsor;
        assert_eq!(
            decode_full_sequencer_receipt(&invalid, &hash),
            Err(RpcFault::Malformed)
        );
    }
    Ok(())
}

pub(crate) fn registered_budgets_cross_caps_and_survive_restart(prefix: &str) -> TestResult {
    let mut observations = Vec::new();
    // The same token cost straddles each cap only cumulatively. Ordinary
    // accounts admit two quotes; the lower cap admits one, the higher three.
    for (name, custom, admitted) in [
        ("BUDGET_ORDINARY", None, 2_usize),
        ("BUDGET_SMALL", Some(5_000_000), 1),
        ("BUDGET_LARGE", Some(12_000_000), 3),
    ] {
        let mut harness = start_configured(&format!("{prefix}_{name}"), |config, account| {
            if let Some(budget) = custom {
                config.registered_senders = vec![RegisteredSender {
                    address: account,
                    budget,
                }];
            }
        })?;
        harness.recorded.phase("receipted");
        harness.recorded.bind(
            "$balance",
            json!(format!("{:#x}", 100_u128 * 10_u128.pow(18))),
        );
        let mut actual = Vec::new();
        for index in 0..4 {
            harness
                .recorded
                .bind("$batch_nonce", json!(hex(&word(index))));
            let mut body = harness.quote_body();
            body["nonce"] = json!(index.to_string());
            if index == 1 {
                // Keep the journal, signer, node transcript, interval and DID;
                // only the real station and HTTP service are stopped/reopened.
                harness.restart()?;
            }
            let durable_before_request = std::fs::read(&harness.journal)?;
            let (status, quote) = harness.post("/quote", &body)?;
            actual.push(status);
            if status == 200 {
                assert_eq!(quote["quote"]["tokenAmount"], AMOUNT.to_string());
                assert_eq!(quote["quote"]["quoteNonce"], (index + 1).to_string());
            } else {
                assert_eq!(quote, json!({"error": "refused"}));
            }
            if status != 200 {
                assert_eq!(std::fs::read(&harness.journal)?, durable_before_request);
            }
        }
        assert!(
            harness.recorded.sent().is_empty(),
            "admissions never broadcast transactions"
        );
        observations.push((name, actual, admitted));
        harness.finish()?;
    }
    // Complete every control and both custom-cap paths before reporting a
    // mismatch, so a production wiring gap cannot hide the ordinary control.
    for (name, actual, admitted) in observations {
        let expected: Vec<u16> = (0..4)
            .map(|index| if index < admitted { 200 } else { 422 })
            .collect();
        assert_eq!(
            actual, expected,
            "{name}: cumulative account budget across restart"
        );
    }
    Ok(())
}
