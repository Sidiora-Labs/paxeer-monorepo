//! Proves: SettlementReceipt and Grant -> public Rust settlement/payment APIs -> decode_payment_exact and read_payer_grant consumers.
//!
//! The interface inventory names the public Rust item as the crossing and this
//! integration test as its first external consumer. The receipt input is the
//! output of rpc/sequencer_receipt_vectors_test.go, authenticated below before
//! its decoded logs are consumed. No RPC transport or payment plane is replaced.
//! The stateless boundary is crossed afresh twice, then malformed results are
//! refused by the same decoders. The RPC refusal uses the real sidecar server.
//!
//! Repairs settlement.rs::decode: a submitted answer carrying signingKey must
//! be refused, just like one carrying keyId or receipt. payment.rs documents
//! the distinction between decoding an event and authenticating its receipt.
//! Outside scope: precompiles/layerxledger/payments.go::getPayerGrant currently
//! returns LXP_NO_PAYER_GRANT unconditionally; these API proofs do not claim a
//! successful live ledger grant lookup. Transport failure must remain an error,
//! never be converted into an absent grant.

use std::error::Error;
use std::path::Path;

use ed25519_dalek::{Signer as _, SigningKey};
use layerx_crypto::payments::Payment;
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_proof::receipt::{verify_sequencer_signature, ReceiptCheck};
use layerx_proof::sequencer_logs::{verify_logs, SequencerLog};
use layerx_types::payload::ModuleId;
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};
use x_websearch::payment::{
    decode_grant, decode_payment_exact, read_payer_grant, unhex, PaymentExactLog, LEDGER_PRECOMPILE,
};
use x_websearch::settlement::{
    decode, SettlementError, SettlementReceipt, SettlementRpc, SettlementRung,
};
use x_websearch::{Limits, RouteTable, Server};

type Outcome<T = ()> = Result<T, Box<dyn Error>>;

fn bytes(value: &Value) -> Outcome<Vec<u8>> {
    unhex(
        value
            .as_str()
            .ok_or("hex string")?
            .strip_prefix("0x")
            .ok_or("hex prefix")?,
    )
    .ok_or_else(|| "hex bytes".into())
}

fn word(value: &Value) -> Outcome<[u8; 32]> {
    bytes(value)?.try_into().map_err(|_| "32-byte word".into())
}

fn producer_case() -> Outcome<Value> {
    let root = Path::new(env!("CARGO_MANIFEST_DIR")).join("../../../..");
    let vectors: Value = serde_json::from_slice(&std::fs::read(
        root.join("sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"),
    )?)?;
    assert_eq!(
        vectors["provenance"]["generator"],
        "rpc/sequencer_receipt_vectors_test.go"
    );
    vectors["cases"]
        .as_array()
        .ok_or("producer cases")?
        .iter()
        .find(|case| case["id"] == "V1-layerxledger-pay-exact")
        .cloned()
        .ok_or_else(|| "producer exact payment".into())
}

fn authenticated_receipt(case: &Value) -> Outcome<SettlementReceipt> {
    // Deserialize the producer's serialized output through the public API.
    let serialized = serde_json::to_vec(&case["receiptJson"])?;
    let receipt = decode(&serde_json::from_slice(&serialized)?)?;
    let pin = &case["pinnedKey"];
    let pins = [SequencerAuthorization::new(
        word(&pin["keyId"])?,
        word(&pin["publicKey"])?,
        pin["firstHeight"].as_u64().ok_or("first height")?,
        pin["lastHeight"].as_u64().ok_or("last height")?,
    )];
    let signed = receipt.receipt.as_deref().ok_or("signed receipt")?;
    assert_eq!(signed, bytes(&case["signedReceipt"])?);
    let verified = verify_sequencer_signature(signed, &pins)
        .map_err(|failure| format!("producer signature: {failure:?}"))?;
    let logs = receipt
        .logs
        .iter()
        .map(|log| {
            SequencerLog::new(
                log.address,
                log.topics.clone(),
                log.data.clone(),
                log.log_index,
            )
        })
        .collect();
    let verified_logs =
        verify_logs(&verified, logs).map_err(|failure| format!("producer logs: {failure:?}"))?;
    assert_eq!(verified_logs.logs().len(), 1);
    assert_eq!(verified_logs.logs()[0].address(), LEDGER_PRECOMPILE);
    assert_eq!(verified_logs.logs()[0].data(), receipt.logs[0].data);
    Ok(receipt)
}

#[test]
fn producer_receipt_crosses_the_public_api_into_the_exact_payment_consumer_twice() -> Outcome {
    let expected = PaymentExactLog {
        payer: word(&json!(
            "0x1fb0082aa4aa4ce1eb67568a7d8f67798f3b008c254e0929dfe5eabb1dc0d769"
        ))?,
        payee: word(&json!(
            "0x112b929c0d6cb01c57eaad307e6d0a094c6c6e58f2e0ce3c62856cf815c0fba4"
        ))?,
        reference: word(&json!(
            "0x26fde3dedb14662268bff5bb3ab6560550a0dc3938b0c9d5ed717932c75b9839"
        ))?,
        asset: word(&json!(
            "0x4e366c1f82bbcb0cfc80b6168346024ca11db454d4b5e93dc7cac57bc5292f28"
        ))?,
        amount: 1_000_000,
    };
    for _ in 0..2 {
        // Reload and decode, rather than retaining the first observed result.
        let case = producer_case()?;
        let receipt: SettlementReceipt = authenticated_receipt(&case)?;
        assert_eq!(receipt.rung, SettlementRung::Receipted);
        assert_eq!(receipt.tx_hash, word(&case["receiptJson"]["txHash"])?);
        assert_eq!(receipt.nonce, 2);
        assert_eq!(receipt.seq, Some(3));
        assert_eq!(receipt.height, Some(1001));
        assert_eq!(receipt.status, Some(1));
        assert_eq!(receipt.key_id, Some([0x6b; 32]));
        assert_eq!(receipt.logs.len(), 1);
        assert_eq!(receipt.logs[0].log_index, 0);
        assert_eq!(
            decode_payment_exact(&receipt.logs[0]),
            Some(expected.clone())
        );

        let mut foreign = receipt.logs[0].clone();
        foreign.address[0] ^= 1;
        assert_eq!(decode_payment_exact(&foreign), None);
        let mut overflow = receipt.logs[0].clone();
        overflow.data[32] = 1;
        assert_eq!(decode_payment_exact(&overflow), None);
        let mut truncated = receipt.logs[0].clone();
        truncated.data.pop();
        assert_eq!(decode_payment_exact(&truncated), None);
    }
    Ok(())
}

#[test]
fn malformed_producer_receipts_and_an_untrusted_signer_are_refused() -> Outcome {
    let case = producer_case()?;
    let original = &case["receiptJson"];
    for (field, bad_value) in [
        ("status", json!("0x2")),
        ("seq", json!("0x03")),
        ("keyId", json!("0x01")),
        ("receipt", json!("0x")),
        ("logs", Value::Null),
    ] {
        let mut malformed = original.clone();
        malformed[field] = bad_value;
        assert_eq!(
            decode(&malformed),
            Err(SettlementError::Malformed),
            "{field}"
        );
        assert!(decode(original).is_ok(), "no cached refusal after {field}");
    }
    let receipt = authenticated_receipt(&case)?;
    let wrong_pins = [SequencerAuthorization::new(
        [0x6b; 32], [0x55; 32], 1000, 1999,
    )];
    let refused = verify_sequencer_signature(
        receipt.receipt.as_deref().ok_or("signed receipt")?,
        &wrong_pins,
    )
    .err()
    .ok_or("wrong signer accepted")?;
    assert_eq!(refused.check, ReceiptCheck::SequencerSignature);

    // A submitted answer is produced by removing the receipt view, exactly as
    // rpc/sequencer_receipt.go::SequencerReceiptResult serializes that rung.
    let submitted = json!({
        "rung": "submitted", "txHash": original["txHash"], "nonce": original["nonce"],
    });
    assert_eq!(decode(&submitted)?.rung, SettlementRung::Submitted);
    for field in ["signingKey", "keyId", "receipt"] {
        let mut malformed = submitted.clone();
        malformed[field] = original[field].clone();
        assert_eq!(
            decode(&malformed),
            Err(SettlementError::Malformed),
            "{field}"
        );
    }
    Ok(())
}

#[test]
fn legacy_signed_grant_codec_is_observed_twice_and_corrupt_signatures_are_refused() -> Outcome {
    for _ in 0..2 {
        let recording: Value = serde_json::from_slice(&std::fs::read(
            Path::new(env!("CARGO_MANIFEST_DIR")).join("tests/fixtures/gateway/metered.json"),
        )?)?;
        let entry = &recording["ledger"]["grants"][0];
        let canonical =
            unhex(entry["canonical"].as_str().ok_or("canonical grant")?).ok_or("grant bytes")?;
        let Payment::IssueGrant(mut grant) =
            Payment::decode(ModuleId::Asset, 7, &canonical, b"")
                .map_err(|error| format!("grant producer: {error:?}"))?
        else {
            return Err("expected IssueGrant".into());
        };
        // Recreate the producer signature with the recording's public test key,
        // then invoke the production encoder; no replacement grant codec.
        let seed: [u8; 32] = Sha256::digest(b"x-websearch-test/key/payer").into();
        let signer = SigningKey::from_bytes(&seed);
        assert_eq!(grant.public_key, signer.verifying_key().to_bytes());
        grant.id = grant
            .signing_digest()
            .map_err(|error| format!("grant digest: {error:?}"))?;
        grant.signature = signer.sign(&grant.id).to_bytes();
        let emitted = Payment::IssueGrant(grant.clone())
            .encode(b"")
            .map_err(|error| format!("grant encode: {error:?}"))?;
        assert_eq!(emitted, canonical);
        let answer = bytes(&entry["result"])?;
        assert_eq!(decode_grant(&answer, ""), Some(grant));

        let mut corrupt = answer.clone();
        // ABI head (7 words), dynamic length word, canonical signature's last byte.
        corrupt[8 * 32 + emitted.len() - 1] ^= 1;
        assert_eq!(decode_grant(&corrupt, ""), None);
        assert_eq!(decode_grant(&answer[..answer.len() - 1], ""), None);
        assert!(decode_grant(&answer, "").is_some());
    }
    Ok(())
}

#[test]
fn real_transport_refusals_reach_the_grant_and_receipt_consumers_twice() -> Outcome {
    assert_eq!(
        SettlementRpc::new("http://127.0.0.1:0/rpc?secret=forbidden", 125).err(),
        Some(SettlementError::Malformed)
    );
    // The real sidecar is an HTTP service, not a settlement JSON-RPC endpoint.
    // Its actual method refusal must reach consumers as Malformed, not None.
    let server =
        Server::bind("127.0.0.1:0".parse()?, Limits::default(), RouteTable::new())?.spawn()?;
    let rpc = SettlementRpc::new(&format!("http://{}/health", server.local_addr()), 125)?;
    assert!(rpc.endpoint().ends_with("/health"));
    for _ in 0..2 {
        assert_eq!(
            read_payer_grant(&rpc, &[0x71; 32]),
            Err(SettlementError::Malformed)
        );
        assert_eq!(
            rpc.sequencer_receipt(&[0x72; 32]),
            Err(SettlementError::Malformed)
        );
    }
    // Shutdown keeps the listener reserved until the service really stops;
    // neither a transport implementation nor an RPC result is supplied here.
    server.shutdown()?;
    assert_eq!(
        read_payer_grant(&rpc, &[0x71; 32]),
        Err(SettlementError::Transport)
    );
    assert_eq!(
        rpc.sequencer_receipt(&[0x72; 32]),
        Err(SettlementError::Transport)
    );
    Ok(())
}
