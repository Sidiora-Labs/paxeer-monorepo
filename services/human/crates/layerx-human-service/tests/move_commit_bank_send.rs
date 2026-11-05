#[path = "support/production_fixture.rs"]
mod production_fixture;

use layerx_crypto::disclosure::call::{bind_call, ConstructMethod};
use layerx_crypto::evm_transaction::{decode_signed, recover_sender, transaction_hash};
use production_fixture::*;
use serde_json::json;

#[test]
fn move_commit_submits_a_bank_transfer_without_a_movement_provider() -> Result<()> {
    let _guard = ENVIRONMENT
        .lock()
        .unwrap_or_else(|poisoned| poisoned.into_inner());
    for source in [
        include_str!("../src/server/production_components.rs"),
        include_str!("support/production_fixture.rs"),
        include_str!("move_commit_bank_send.rs"),
    ] {
        for retired in [
            concat!("movement", ".sock"),
            concat!("LAYERX_HUMAN_", "MOVEMENT_"),
            concat!("layerx_", "paxeer_client"),
        ] {
            assert!(!source.contains(retired), "retired dependency: {retired}");
        }
    }
    let mut fixture = Fixture::open()?;
    let wallet = {
        let peer = fixture.peer_state.lock().map_err(|_| "peer state")?;
        let actor = fixture.document["actor"].as_str().ok_or("owner actor")?;
        assert!(peer.submitted.is_empty());
        peer.identities.get(actor).ok_or("owner identity")?.1
    };
    let recipient = [0x42; 20];
    assert_ne!(recipient, wallet);
    let body = json!({
        "source": fixture.document["intent"]["source"]["account"],
        "destination": format!("0x{}", cluster::hex(&recipient)),
        "money": {"amount": "7", "currency": "LXP"}
    });
    let quote = fixture.request("POST", "/v1/moves/quote", &body, "move-bank-quote")?;
    assert_eq!(quote["money"], body["money"]);
    let amount: u128 = quote["money"]["amount"]
        .as_str()
        .ok_or("quoted amount")?
        .parse()?;
    assert_ne!(amount, 0);
    assert_eq!(quote["mechanism"], "transfer");
    assert!(fixture
        .peer_state
        .lock()
        .map_err(|_| "peer state")?
        .submitted
        .is_empty());

    let commit = json!({"quote_id": quote["quote_id"]});
    let started = fixture.request("POST", "/v1/moves", &commit, "move-bank-commit")?;
    assert_eq!(started["kind"], "move");
    let journey_id = started["journey_id"].as_str().ok_or("journey id")?;
    let record = fixture.journey_record(journey_id)?;
    assert_eq!(record["move_quote_id"], quote["quote_id"]);
    let legs = record["legs"].as_array().ok_or("record legs")?;
    assert_eq!(legs.len(), 1);
    let stages = started["stages"].as_array().ok_or("journey stages")?;
    assert_eq!(stages.len(), 1);

    let (hash, raw) = {
        let peer = fixture.peer_state.lock().map_err(|_| "peer state")?;
        assert_eq!(peer.submitted.len(), 1);
        assert_eq!(
            peer.rpc_served
                .iter()
                .filter(|method| *method == "eth_sendRawTransaction")
                .count(),
            1
        );
        assert!(!peer.pending_nonces.is_empty());
        assert!(peer
            .pending_nonces
            .iter()
            .all(|answer| *answer == (wallet, NONCE)));
        let (hash, raw) = peer.submitted.iter().next().ok_or("bank transaction")?;
        (*hash, raw.clone())
    };
    assert_eq!(raw.first(), Some(&2), "EIP-1559 envelope");
    assert_eq!(transaction_hash(&raw), hash);
    let (call, signature) =
        decode_signed(&raw).map_err(|error| format!("signed transfer: {error:?}"))?;
    assert_eq!(
        recover_sender(&call, &signature).map_err(|error| format!("sender: {error:?}"))?,
        wallet
    );
    assert_eq!(call.nonce, NONCE);
    assert_eq!(call.chain_id, 125);
    let mut bank = [0; 20];
    bank[18..].copy_from_slice(&0x1001_u16.to_be_bytes());
    assert_eq!(call.to, bank);
    assert_eq!(
        layerx_crypto::evm_transaction::assemble(&call, &signature),
        raw,
        "canonical zero-value EIP-1559 call"
    );
    let mut amount_word = [0; 32];
    amount_word[16..].copy_from_slice(&amount.to_be_bytes());
    assert_eq!(
        bind_call(&call, wallet)
            .map_err(|error| format!("bank disclosure: {error:?}"))?
            .construct_method()
            .map_err(|error| format!("bank method: {error:?}"))?,
        Some(ConstructMethod::BankSend {
            from: wallet,
            to: recipient,
            denom: "alxp".to_owned(),
            amount: amount_word
        })
    );
    assert_eq!(legs[0]["tx_hash"], format!("0x{}", cluster::hex(&hash)));
    assert_eq!(legs[0]["nonce"], call.nonce);
    assert_eq!(legs[0]["submitted"], true);
    assert_eq!(stages[0]["tx_hash"], legs[0]["tx_hash"]);
    assert_eq!(
        serde_json::from_value::<Vec<u8>>(legs[0]["raw_transaction"].clone())?,
        raw
    );

    fixture.reopen()?;
    let repeated = fixture.request("POST", "/v1/moves", &commit, "move-bank-commit")?;
    assert_eq!(repeated["journey_id"], started["journey_id"]);
    assert_eq!(repeated["stages"][0]["tx_hash"], stages[0]["tx_hash"]);
    let recovered = fixture.journey_record(journey_id)?;
    assert_eq!(recovered["legs"][0]["tx_hash"], legs[0]["tx_hash"]);
    let peer = fixture.peer_state.lock().map_err(|_| "peer state")?;
    assert_eq!(peer.submitted.len(), 1);
    assert_eq!(peer.submitted.get(&hash), Some(&raw));
    assert_eq!(
        peer.rpc_served
            .iter()
            .filter(|method| *method == "eth_sendRawTransaction")
            .count(),
        1
    );
    Ok(())
}

fn exercise_accepted_submission_recovery(
    reopen: bool,
    retained_only: bool,
    submitted: bool,
) -> Result<()> {
    let _guard = ENVIRONMENT
        .lock()
        .unwrap_or_else(|poisoned| poisoned.into_inner());
    let mut fixture = Fixture::open()?;
    let body = json!({
        "source": fixture.document["intent"]["source"]["account"],
        "destination": format!("0x{}", cluster::hex(&[0x42; 20])),
        "money": {"amount": "7", "currency": "LXP"}
    });
    let quote = fixture.request("POST", "/v1/moves/quote", &body, "recovery-quote")?;
    {
        let mut peer = fixture.peer_state.lock().map_err(|_| "peer state")?;
        peer.fail_next_receipt_lookup = true;
        peer.reject_repeated_send = true;
    }
    let commit = json!({"quote_id": quote["quote_id"]});
    let started = fixture.request("POST", "/v1/moves", &commit, "recovery-commit")?;
    assert_eq!(started["state"], "still-checking");
    let journey_id = started["journey_id"].as_str().ok_or("journey id")?;
    let (hash, raw) = {
        let peer = fixture.peer_state.lock().map_err(|_| "peer state")?;
        assert_eq!(peer.failed_receipt_lookups, 1);
        assert!(!peer.fail_next_receipt_lookup);
        assert_eq!(peer.submitted.len(), 1);
        assert_eq!(peer.receipt_lookups.len(), 1);
        let (hash, raw) = peer.submitted.iter().next().ok_or("accepted transaction")?;
        (*hash, raw.clone())
    };
    let hash_text = format!("0x{}", cluster::hex(&hash));
    let (call, signature) =
        decode_signed(&raw).map_err(|error| format!("signed leg: {error:?}"))?;
    let sender = recover_sender(&call, &signature).map_err(|error| format!("sender: {error:?}"))?;
    let disclosure = bind_call(&call, sender).map_err(|error| format!("disclosure: {error:?}"))?;
    let record = fixture.journey_record(journey_id)?;
    assert_eq!(record["state"], "still-checking");
    assert_eq!(record["legs"][0]["submitted"], true);
    assert_eq!(record["legs"][0]["verified"], false);
    assert_eq!(record["legs"][0]["tx_hash"], hash_text);
    assert_eq!(record["legs"][0]["nonce"], call.nonce);
    assert_eq!(started["stages"][0]["tx_hash"], hash_text);

    if reopen {
        fixture.reopen()?;
        let recovered = fixture.journey_record(journey_id)?;
        assert_eq!(recovered, record);
        fixture.edit_journey_record(journey_id, |record| {
            record["legs"][0]
                .as_object_mut()
                .expect("submitted leg")
                .remove("nonce");
        })?;
    }
    if submitted {
        let waiting = fixture.request("POST", "/v1/moves", &commit, "recovery-commit")?;
        assert_eq!(waiting["state"], "still-checking");
        let waiting_record = fixture.journey_record(journey_id)?;
        assert_eq!(waiting_record["legs"][0]["submitted"], true);
        assert_eq!(waiting_record["legs"][0]["tx_hash"], hash_text);
        assert_eq!(waiting_record["legs"][0]["nonce"], call.nonce);
        let peer = fixture.peer_state.lock().map_err(|_| "peer state")?;
        assert_eq!(
            peer.rpc_served
                .iter()
                .filter(|method| *method == "eth_sendRawTransaction")
                .count(),
            1
        );
    }
    if retained_only {
        fixture.edit_journey_record(journey_id, |record| {
            record["legs"][0]["submitted"] = json!(false);
            record["legs"][0]
                .as_object_mut()
                .expect("retained leg")
                .remove("nonce");
        })?;
    }
    fixture
        .peer_state
        .lock()
        .map_err(|_| "peer state")?
        .receipt_rung = Some(layerx_types::rung::Rung::Committed);
    let completed = fixture.request("POST", "/v1/moves", &commit, "recovery-commit")?;
    assert_eq!(completed["journey_id"], started["journey_id"]);
    assert_eq!(completed["state"], "done");
    assert_eq!(completed["stages"][0]["tx_hash"], hash_text);
    let record = fixture.journey_record(journey_id)?;
    assert_eq!(record["state"], "done");
    assert_eq!(record["legs"][0]["submitted"], true);
    assert_eq!(record["legs"][0]["verified"], true);
    assert_eq!(record["legs"][0]["tx_hash"], hash_text);
    assert_eq!(record["legs"][0]["nonce"], call.nonce);
    let receipt_bytes: Vec<u8> = serde_json::from_value(record["legs"][0]["receipt"].clone())?;
    let receipt = layerx_wire::receipt::decode(&receipt_bytes)
        .map_err(|error| format!("committed receipt: {error:?}"))?;
    let protocol = receipt.protocol().ok_or("protocol receipt")?;
    assert_eq!(protocol.activity_id(), hash);
    assert_eq!(
        protocol.protocol_version(),
        layerx_wire::limits::PROTOCOL_VERSION
    );
    assert_eq!(protocol.module_id(), 12, "node EVMReceiptModuleID");
    let peer = fixture.peer_state.lock().map_err(|_| "peer state")?;
    assert_eq!(peer.submitted.len(), 1);
    assert_eq!(peer.submitted.get(&hash), Some(&raw));
    assert_eq!(
        peer.rpc_served
            .iter()
            .filter(|method| *method == "eth_sendRawTransaction")
            .count(),
        if retained_only { 2 } else { 1 }
    );
    assert_eq!(peer.receipt_lookups.len(), if submitted { 3 } else { 2 });
    assert!(peer
        .receipt_lookups
        .iter()
        .all(|lookup| *lookup == (disclosure.idempotency_key, hash)));
    Ok(())
}

#[test]
fn move_commit_retains_the_accepted_submission_when_the_receipt_read_fails() -> Result<()> {
    exercise_accepted_submission_recovery(false, false, false)
}

#[test]
fn move_commit_recovers_the_accepted_submission_after_reopening() -> Result<()> {
    exercise_accepted_submission_recovery(true, false, false)
}

#[test]
fn move_commit_reconciles_a_consumed_nonce_with_a_retained_hash() -> Result<()> {
    exercise_accepted_submission_recovery(false, true, false)
}

#[test]
fn move_commit_keeps_a_submitted_receipt_pending_without_resending() -> Result<()> {
    exercise_accepted_submission_recovery(false, false, true)
}
