include!("support/settlement_fixture.rs");

use layerx_interop_service::relay::{ChainRelay, RelayError};
use sha2::{Digest as _, Sha256};

fn assert_mounted_recovery(
    case: &Value,
    raw: Vec<u8>,
    dir: &Path,
    identity: &Identity,
    node: &Node,
    mut redis: Redis,
) -> TestResult {
    node.reset();
    let config = gateway(dir, &redis, node)?;
    let issued = IssuedKey::generate()?;
    let record = KeyRecord {
        key_id: issued.id().into(),
        principal_digest: "33".repeat(32),
        salt: "repair-salt".into(),
        secret_digest: gateway_digest(&[
            b"gateway-key-v1",
            b"repair-salt",
            issued.secret().as_bytes(),
        ]),
        signer_public_key: hex(SigningKey::from_bytes(&[7; 32]).verifying_key().as_bytes()),
        scopes: "interop:*".into(),
        quota_requests: 100,
        quota_window_seconds: 60,
        epoch: 1,
        disabled: false,
    };
    config
        .store
        .issue_key(&record, "repair integration issuance")?;
    let authorization = format!("LayerX-Key {}:{}", issued.id(), issued.secret());
    let body = visa_payment(case)?;
    let (config, status, _) = mounted(config, &body, None)?;
    assert_eq!(status, 401);
    node.fault.store(2, Ordering::SeqCst);
    let (config, status, response) = mounted(config, &body, Some(&authorization))?;
    assert_eq!(status, 503, "{response}");
    assert_eq!(response["error"]["code"], "settlement_unavailable");
    assert_eq!(node.state.lock().or_panic("node state").accepted.len(), 1);
    drop(config);
    let address = redis.listener.address;
    drop(redis);
    redis = Redis::start(dir, identity, address)?;
    let config = gateway(dir, &redis, node)?;
    let (config, status, response) = mounted(config, &body, Some(&authorization))?;
    assert_eq!(status, 200, "{response}");
    assert_eq!(response["result"]["state"], "receipt-verified");
    let operation = response["operation"].as_str().ok_or("operation scope")?;
    let stored = config
        .store
        .operation(operation)?
        .ok_or("stored operation")?;
    assert_eq!(stored.state, "completed");
    assert_eq!(
        stored.receipt,
        case["signedReceipt"]
            .as_str()
            .ok_or("receipt")?
            .trim_start_matches("0x")
    );
    let (_, status, replay) = mounted(config, &body, Some(&authorization))?;
    assert_eq!(status, 200, "{replay}");
    assert_eq!(replay["result"], response["result"]);
    let state = node.state.lock().or_panic("node state");
    assert_eq!(state.accepted.len(), 1);
    assert_eq!(state.attempts, vec![raw]);
    assert_eq!(
        state
            .methods
            .iter()
            .filter(|method| *method == "px_getReceiptKeys")
            .count(),
        2
    );
    assert!(state
        .connects
        .iter()
        .all(|target| target == "CONNECT router.railway.internal:9443 HTTP/1.1"));
    Ok(())
}

#[test]
fn relay_claim_before_send_and_private_endpoint_settlement() -> TestResult {
    let current = payment_case()?;
    let case = &current;
    let raw = bytes(case["rawTransaction"].as_str().ok_or("raw transaction")?)?;
    let (transaction, signature) = decode_signed(&raw)?;
    let sender = recover_sender(&transaction, &signature)?;
    let tx_hash = transaction_hash(&raw);
    assert!(layerx_client::sequencer::SequencerEndpoint::new(PRIVATE_URL, &[]).is_err());
    let dir = Directory::new()?;
    let identity = configure(&dir.0, case)?;
    let node = Node::start(case)?;
    let mut redis = Redis::start(&dir.0, &identity, "127.0.0.1:0".parse()?)?;
    // The TLS/RESP peer persists the actual journal commands before dropping a claim acknowledgement.
    // Dropping all relay/config instances then reopening the peer discards every volatile component.
    for scenario in 0..4 {
        node.reset();
        let key = format!("repair-recovery-{scenario}");
        let journal_key = format!("interop:relay:v1:{:x}", Sha256::digest(key.as_bytes()));
        let config = gateway(&dir.0, &redis, &node)?;
        let relay = ChainRelay::from_config(&config);
        if scenario == 0 {
            redis.lose_claim_reply.store(true, Ordering::SeqCst);
        } else {
            node.fault
                .store(if scenario == 1 { 1 } else { 2 }, Ordering::SeqCst);
        }
        let error = relay
            .relay(&key, &raw, sender)
            .err()
            .or_panic("relay refusal");
        if scenario == 0 {
            assert!(matches!(error, RelayError::Journal(_)), "{error:?}");
        } else {
            assert!(matches!(error, RelayError::Sequencer(_)), "{error:?}");
        }
        assert_eq!(
            redis.record(&journal_key),
            Some([tx_hash.to_vec(), raw.clone()].concat())
        );
        assert_eq!(redis.record(&format!("{journal_key}:submitted")), None);
        {
            let state = node.state.lock().or_panic("node state");
            assert_eq!(state.attempts.len(), usize::from(scenario != 0));
            assert_eq!(state.accepted.len(), usize::from(scenario >= 2));
        }
        drop(relay);
        drop(config);
        let address = redis.listener.address;
        drop(redis);
        redis = Redis::start(&dir.0, &identity, address)?;
        let config = gateway(&dir.0, &redis, &node)?;
        let relay = ChainRelay::from_config(&config);
        if scenario == 2 {
            node.hide_receipt.store(true, Ordering::SeqCst);
        }
        let outcome = if scenario % 2 == 0 {
            relay.recorded_evidence(&key)?.ok_or("durable claim")?
        } else {
            relay.relay(&key, &raw, sender)?
        };
        assert_executed(outcome, case)?;
        assert_eq!(
            redis.record(&format!("{journal_key}:submitted")),
            Some(tx_hash.to_vec())
        );
        assert_executed(relay.relay(&key, &raw, sender)?, case)?;
        assert_executed(
            relay.recorded_evidence(&key)?.ok_or("durable evidence")?,
            case,
        )?;
        let mut replacement = transaction.clone();
        replacement.nonce += 1;
        let replacement = replacement.sign(&[0x3b; 32])?;
        assert_eq!(
            relay.relay(&key, &replacement, sender),
            Err(RelayError::TransactionReplacementRefused)
        );
        let state = node.state.lock().or_panic("node state");
        assert_eq!(
            state.accepted.len(),
            1,
            "one accepted chain transaction after recovery"
        );
        assert_eq!(
            state.attempts.len(),
            if scenario == 1 || scenario == 2 { 2 } else { 1 }
        );
        assert!(state.attempts.iter().all(|attempt| attempt == &raw));
        assert!(state
            .methods
            .iter()
            .any(|method| method == "px_getReceiptKeys"));
        assert!(!state.connects.is_empty());
    }
    assert_mounted_recovery(case, raw, &dir.0, &identity, &node, redis)?;
    Ok(())
}
