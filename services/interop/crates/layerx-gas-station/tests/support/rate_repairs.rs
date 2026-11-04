mod node_recording {
    include!("node_tape.rs");
}

fn assert_publication_effects_match_node_view(receipt: &Value) -> TestResult {
    use layerx_proof::receipt::verify_sequencer_signature;
    use layerx_proof::sequencer_logs::{verify_logs, SequencerLog};
    let signed = bytes(
        receipt["receipt"]
            .as_str()
            .ok_or("missing signed receipt")?,
    )?;
    let config = node_recording::Tape::load("success_pre")?.config()?;
    let verified = verify_sequencer_signature(&signed, config.sequencer_receipt_keys.as_slice())
        .map_err(|error| format!("publication signature: {error:?}"))?;
    assert_eq!(
        encode(&verified).map_err(|error| format!("canonical receipt: {error:?}"))?,
        signed,
        "preserve canonical signed bytes"
    );
    let protocol = verified.protocol().ok_or("missing protocol receipt")?;
    let hash: Word = bytes(
        receipt["txHash"]
            .as_str()
            .ok_or("missing transaction hash")?,
    )?
    .try_into()
    .map_err(|_| "invalid hash width")?;
    assert_eq!(protocol.activity_id(), hash);
    let logs = receipt["logs"]
        .as_array()
        .ok_or("missing node logs")?
        .iter()
        .map(|log| {
            let address: Address = bytes(log["address"].as_str().ok_or("missing log address")?)?
                .try_into()
                .map_err(|_| "invalid address width")?;
            let topics = log["topics"]
                .as_array()
                .ok_or("missing topics")?
                .iter()
                .map(|topic| -> Result<Word, Box<dyn std::error::Error>> {
                    bytes(topic.as_str().ok_or("missing topic")?)?
                        .try_into()
                        .map_err(|_| "invalid topic width".into())
                })
                .collect::<Result<Vec<_>, _>>()?;
            let data = bytes(log["data"].as_str().ok_or("missing log data")?)?;
            let index = u64::try_from(layerx_gas_station::rpc::quantity(
                log["logIndex"].as_str().ok_or("missing log index")?,
            )?)?;
            Ok(SequencerLog::new(address, topics, data, index))
        })
        .collect::<Result<Vec<_>, Box<dyn std::error::Error>>>()?;
    let evidence = verify_logs(&verified, logs).map_err(|error| {
        format!("rate-fixture-effects: genuine producer execution effects required: {error:?}")
    })?;
    let summary = evidence.summary();
    for (actual, field) in [
        (summary.gas_used(), "gasUsed"),
        (summary.cumulative_gas_used(), "cumulativeGasUsed"),
        (summary.effective_gas_price(), "effectiveGasPrice"),
    ] {
        assert_eq!(
            u128::from(actual),
            layerx_gas_station::rpc::quantity(
                receipt[field]
                    .as_str()
                    .ok_or("missing execution quantity")?
            )?,
            "signed {field} must match the JSON view"
        );
    }
    let status =
        layerx_gas_station::rpc::quantity(receipt["status"].as_str().ok_or("missing status")?)?;
    assert_eq!(protocol.result_code(), if status == 1 { 0 } else { 1 });
    Ok(())
}

pub(crate) fn publication_success_commit_and_effects() -> TestResult {
    let lane = node_recording::Lane::new("effects-success", "success_pre")?;
    let expected = lane.tape.transaction("success")?;
    let mut publisher = lane.publisher()?;
    let publication = publisher.publish(&lane.file(3_400_000)?)?;
    assert_eq!(publication, expected.publication);
    assert_eq!(lane.tape.sent(), vec![expected.raw.clone()]);
    assert_eq!(publisher.journal().state().unsettled(), vec![publication]);
    let receipted = lane.tape.receipt("success_receipted", &publication.hash)?;
    assert_publication_effects_match_node_view(&receipted)?;
    lane.tape.phase("success_committed");
    publisher.recover_pending()?;
    let settled = publisher.journal().state().clone();
    let settlement = settled.publications[&publication.hash]
        .1
        .ok_or("unsettled committed publication")?;
    assert!(settlement.succeeded);
    let gas = layerx_gas_station::rpc::quantity(receipted["gasUsed"].as_str().ok_or("gas")?)?;
    let price =
        layerx_gas_station::rpc::quantity(receipted["effectiveGasPrice"].as_str().ok_or("price")?)?;
    assert_eq!(u128::from(settlement.gas_used), gas);
    assert_eq!(settlement.cost_wei, gas * price);
    assert_eq!(settled.reserved_wei(), 0);
    assert!(settled.rate_active.is_empty());
    let committed = lane.tape.receipt("success_committed", &publication.hash)?;
    assert_eq!(committed["receipt"], receipted["receipt"]);
    assert_publication_effects_match_node_view(&committed)?;
    assert_eq!(lane.tape.sent().len(), 1);
    drop(publisher);
    assert_eq!(
        Journal::open(&lane.dir.join("rate.jsonl"))?.state(),
        &settled
    );
    Ok(())
}

pub(crate) fn publication_pending_restart_and_commit() -> TestResult {
    let lane = node_recording::Lane::new("effects-pending", "success_pre")?;
    let expected = lane.tape.transaction("success")?;
    lane.tape.withhold(true);
    let mut publisher = lane.publisher()?;
    assert_eq!(
        publisher.publish(&lane.file(3_400_000)?),
        Err(RateRefusal::ReceiptTimeout)
    );
    let saved = publisher.journal().state().clone();
    let p = expected.publication;
    assert_eq!(saved.rate_active.get(&(p.owner, p.nonce)), Some(&p.hash));
    assert_eq!(saved.rate_transactions[&p.hash].raw, expected.raw);
    assert!(saved.rate_broadcast.contains(&p.hash));
    drop(publisher);
    lane.tape.withhold(false);
    let mut publisher = lane.publisher()?;
    assert_eq!(publisher.journal().state(), &saved);
    publisher.recover_pending()?;
    assert_eq!(lane.tape.sent(), vec![expected.raw.clone()]);
    assert_eq!(
        publisher.journal().state().rate_transactions[&p.hash].raw,
        expected.raw
    );
    assert_eq!(publisher.journal().state().unsettled().len(), 1);
    assert_publication_effects_match_node_view(&lane.tape.receipt("success_receipted", &p.hash)?)?;
    lane.tape.phase("success_committed");
    publisher.recover_pending()?;
    let settled = publisher.journal().state().clone();
    assert!(settled.unsettled().is_empty());
    assert!(settled.rate_active.is_empty());
    assert_eq!(settled.reserved_wei(), 0);
    assert!(
        settled.publications[&p.hash]
            .1
            .ok_or("settlement")?
            .succeeded
    );
    drop(publisher);
    assert_eq!(
        Journal::open(&lane.dir.join("rate.jsonl"))?.state(),
        &settled
    );
    assert_eq!(lane.tape.sent().len(), 1);
    Ok(())
}

pub(crate) fn publication_revert_and_effects() -> TestResult {
    let lane = node_recording::Lane::new("effects-revert", "revert_pre")?;
    let expected = lane.tape.transaction("revert")?;
    let mut publisher = lane.publisher()?;
    assert_eq!(
        publisher.publish(&lane.file(3_500_000)?),
        Err(RateRefusal::Reverted)
    );
    assert_eq!(lane.tape.sent(), vec![expected.raw.clone()]);
    let p = expected.publication;
    let receipt = lane.tape.receipt("revert_receipted", &p.hash)?;
    assert_eq!(receipt["status"], "0x0");
    assert_eq!(receipt["logs"], json!([]));
    assert_publication_effects_match_node_view(&receipt)?;
    assert_eq!(publisher.journal().state().unsettled().len(), 1);
    drop(publisher);
    lane.tape.phase("revert_committed");
    let mut publisher = lane.publisher()?;
    assert_eq!(publisher.recover_pending(), Err(RateRefusal::Reverted));
    let settlement = publisher.journal().state().publications[&p.hash]
        .1
        .ok_or("failed settlement")?;
    assert!(!settlement.succeeded);
    assert_eq!(
        u128::from(settlement.gas_used),
        layerx_gas_station::rpc::quantity(receipt["gasUsed"].as_str().ok_or("gas")?)?
    );
    assert_eq!(publisher.journal().state().reserved_wei(), 0);
    assert_eq!(lane.tape.sent().len(), 1);
    Ok(())
}

pub(crate) fn publication_view_only_tampering_is_refused() -> TestResult {
    for field in ["gas", "status", "time"] {
        let lane = node_recording::Lane::new(&format!("view-{field}"), "success_pre")?;
        let mut publisher = lane.publisher()?;
        let publication = publisher.publish(&lane.file(3_400_000)?)?;
        let before = lane.tape.receipt("success_receipted", &publication.hash)?;
        lane.tape.tamper(Some(field));
        let rpc = ConfiguredRpc::new(&lane.tape.config()?.station, lane.tape.clone())?;
        let after = layerx_gas_station::rpc::JsonRpc::call(
            &rpc,
            "pax_sequencerReceipt",
            json!([hex(&publication.hash)]),
        )?;
        assert_eq!(
            after["receipt"], before["receipt"],
            "{field}: preserve signed bytes"
        );
        assert_ne!(after, before);
        assert_eq!(
            publisher.recover_pending(),
            Err(RateRefusal::Rpc(RpcFault::Malformed)),
            "{field}: refuse JSON-only forgery"
        );
        assert_eq!(publisher.journal().state().unsettled(), vec![publication]);
        assert_eq!(lane.tape.sent().len(), 1);
    }
    Ok(())
}
