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
