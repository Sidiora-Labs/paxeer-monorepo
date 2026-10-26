use std::collections::BTreeMap;
use std::fmt::Debug;
use std::fs;
use std::path::PathBuf;

use k256::ecdsa::SigningKey;
use layerx_bridge_relayer::attestation::{
    ethereum_address, to_attestor_signature, uint256_from_u64, InboundAttestation,
    OutboundAttestation,
};
use layerx_bridge_relayer::attestation_api::{deposit_answer, AttestationView};
use layerx_bridge_relayer::cosign::CosignDirectory;
use layerx_bridge_relayer::hex;
use layerx_bridge_relayer::journal::{inbound_key, Entry, Journal, Observation, Position};
use serde_json::{json, Value};

fn must<T, E: Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("{error:?}"))
}

struct Directory(PathBuf);

impl Directory {
    fn new() -> Self {
        let mut unique = [0_u8; 16];
        must(openssl::rand::rand_bytes(&mut unique));
        let path = std::env::temp_dir().join(format!(
            "layerx-attestation-api-{}-{}",
            std::process::id(),
            hex::encode(&unique)
        ));
        must(fs::create_dir(&path));
        Self(path)
    }

    fn journal(&self) -> Journal {
        must(Journal::open(&self.0.join("journal.jsonl")))
    }
}

impl Drop for Directory {
    fn drop(&mut self) {
        must(fs::remove_dir_all(&self.0));
    }
}

fn deposit() -> InboundAttestation {
    let mut recipient = [0_u8; 32];
    recipient[12..].fill(0xab);
    InboundAttestation {
        chain_id: u64::MAX,
        vault: [0xcd; 20],
        tx_hash: [0xef; 32],
        log_index: u64::MAX - 1,
        recipient,
        asset: [0xbc; 20],
        amount: [0xff; 32],
    }
}

fn position() -> Position {
    Position {
        block_number: 100,
        block_hash: [0x12; 32],
    }
}

fn observe(journal: &mut Journal, attestation: &InboundAttestation) {
    let observation = Observation::inbound(attestation, position());
    must(journal.append(&Entry::Observed {
        item: observation.key(),
        observation,
    }));
}

fn sign(digest: &[u8; 32], seed: u8) -> ([u8; 20], [u8; 65]) {
    let key = must(SigningKey::from_bytes((&[seed; 32]).into()));
    let (signature, recovery) = must(key.sign_prehash_recoverable(digest));
    let mut recoverable = [0_u8; 65];
    recoverable[..64].copy_from_slice(&signature.to_bytes());
    recoverable[64] = recovery.to_byte();
    (
        ethereum_address(key.verifying_key()),
        must(to_attestor_signature(recoverable)),
    )
}

fn answer(view: &AttestationView, attestation: &InboundAttestation) -> Value {
    deposit_answer(
        view,
        attestation.chain_id,
        &attestation.tx_hash,
        attestation.log_index,
    )
    .unwrap_or_else(|| panic!("observed deposit must have an answer"))
}

#[test]
fn an_observed_deposit_answers_the_bundle_fields_the_bridge_app_reads() {
    let directory = Directory::new();
    let mut journal = directory.journal();
    let mut attestation = deposit();
    let amounts = [
        (
            [0xff; 32],
            "115792089237316195423570985008687907853269984665640564039457584007913129639935",
        ),
        ([0; 32], "0"),
        (uint256_from_u64(1), "1"),
        (uint256_from_u64(u64::MAX), "18446744073709551615"),
        (
            must(hex::fixed(
                "0x8000000000000000000000000000000000000000000000000000000000000000",
            )),
            "57896044618658097711785492504343953926634992332820282019728792003956564819968",
        ),
    ];
    for (index, (amount, decimal)) in amounts.into_iter().enumerate() {
        attestation.log_index = u64::MAX - 1 - must(u64::try_from(index));
        attestation.amount = amount;
        observe(&mut journal, &attestation);
        let digest = attestation.digest();
        let (signer, signature) = sign(&digest, 1);
        let cosign = CosignDirectory::new(directory.0.join("cosign"));
        must(cosign.publish(&digest, &signer, &signature));
        let view = AttestationView {
            items: journal.state().items.clone(),
            signatures: BTreeMap::from([(digest, cosign.collect(&digest))]),
            thresholds: BTreeMap::from([(attestation.chain_id, 2)]),
        };
        let result = answer(&view, &attestation);
        assert_eq!(
            result,
            json!({
                "chainId": "18446744073709551615",
                "vault": format!("0x{}", "cd".repeat(20)),
                "txHash": format!("0x{}", "ef".repeat(32)),
                "logIndex": attestation.log_index.to_string(),
                "recipient": format!("0x{}{}", "00".repeat(12), "ab".repeat(20)),
                "asset": format!("0x{}", "bc".repeat(20)),
                "amount": decimal,
                "digest": hex::prefixed(&digest),
                "signatures": [hex::prefixed(&signature)],
                "status": "observed",
            })
        );
        assert_eq!(result["signatures"][0].as_str().map(str::len), Some(132));
    }
    drop(journal);
    let replayed = directory.journal();
    let view = AttestationView {
        items: replayed.state().items.clone(),
        ..AttestationView::default()
    };
    assert_eq!(answer(&view, &attestation)["amount"], amounts[4].1);
}

#[test]
fn an_unobserved_deposit_answers_nothing() {
    let directory = Directory::new();
    let mut journal = directory.journal();
    let attestation = deposit();
    let view = AttestationView::default();
    assert_eq!(
        deposit_answer(
            &view,
            attestation.chain_id,
            &attestation.tx_hash,
            attestation.log_index
        ),
        None
    );
    observe(&mut journal, &attestation);
    let outbound = OutboundAttestation {
        chain_id: 7,
        vault: attestation.vault,
        paxeer_tx_hash: attestation.tx_hash,
        paxeer_nonce: attestation.log_index,
        recipient: [0xab; 20],
        asset: attestation.asset,
        amount: attestation.amount,
    };
    let observation = Observation::outbound(&outbound, position());
    must(journal.append(&Entry::Observed {
        item: observation.key(),
        observation,
    }));
    let view = AttestationView {
        items: journal.state().items.clone(),
        ..AttestationView::default()
    };
    for (chain, hash, index) in [
        (
            attestation.chain_id - 1,
            attestation.tx_hash,
            attestation.log_index,
        ),
        (attestation.chain_id, [0xee; 32], attestation.log_index),
        (
            attestation.chain_id,
            attestation.tx_hash,
            attestation.log_index - 1,
        ),
        (
            outbound.chain_id,
            outbound.paxeer_tx_hash,
            outbound.paxeer_nonce,
        ),
    ] {
        assert_eq!(deposit_answer(&view, chain, &hash, index), None);
    }
    assert_eq!(answer(&view, &attestation)["status"], "observed");
}

#[test]
fn deposit_status_follows_the_collected_signatures() {
    let directory = Directory::new();
    let mut journal = directory.journal();
    let attestation = deposit();
    observe(&mut journal, &attestation);
    let digest = attestation.digest();
    let key = inbound_key(
        attestation.chain_id,
        &attestation.tx_hash,
        attestation.log_index,
    );
    let cosign = CosignDirectory::new(directory.0.join("cosign"));
    let mut view = AttestationView {
        items: journal.state().items.clone(),
        signatures: BTreeMap::from([(digest, cosign.collect(&digest))]),
        thresholds: BTreeMap::from([(attestation.chain_id, 2)]),
    };
    assert_eq!(answer(&view, &attestation)["status"], "observed");
    assert_eq!(answer(&view, &attestation)["signatures"], json!([]));

    let (own_signer, own_signature) = sign(&digest, 1);
    must(journal.append(&Entry::Signed {
        item: key,
        signature: own_signature,
    }));
    view.items = journal.state().items.clone();
    assert_eq!(answer(&view, &attestation)["status"], "attesting");
    assert_eq!(
        answer(&view, &attestation)["signatures"],
        json!([hex::prefixed(&own_signature)])
    );

    must(cosign.publish(&digest, &own_signer, &own_signature));
    must(cosign.publish(&digest, &own_signer, &own_signature));
    let mut collected = cosign.collect(&digest);
    collected.push(own_signature);
    view.signatures.insert(digest, collected);
    assert_eq!(answer(&view, &attestation)["status"], "attesting");
    assert_eq!(
        answer(&view, &attestation)["signatures"],
        json!([hex::prefixed(&own_signature)])
    );

    let (peer, peer_signature) = sign(&digest, 2);
    must(cosign.publish(&digest, &peer, &peer_signature));
    view.signatures.insert(digest, cosign.collect(&digest));
    let ready = answer(&view, &attestation);
    assert_eq!(ready["status"], "ready");
    let mut expected = BTreeMap::from([(own_signer, own_signature), (peer, peer_signature)]);
    let encoded: Vec<_> = expected
        .values()
        .map(|signature| hex::prefixed(signature))
        .collect();
    assert_eq!(ready["signatures"], json!(encoded));

    let (third, third_signature) = sign(&digest, 3);
    must(cosign.publish(&digest, &third, &third_signature));
    expected.insert(third, third_signature);
    view.signatures.insert(digest, cosign.collect(&digest));
    assert_eq!(answer(&view, &attestation)["status"], "ready");
    let encoded: Vec<_> = expected
        .values()
        .map(|signature| hex::prefixed(signature))
        .collect();
    assert_eq!(answer(&view, &attestation)["signatures"], json!(encoded));

    view.thresholds.insert(attestation.chain_id, 4);
    assert_eq!(answer(&view, &attestation)["status"], "attesting");
    view.thresholds.insert(attestation.chain_id, 0);
    assert_eq!(answer(&view, &attestation)["status"], "attesting");
    view.thresholds.remove(&attestation.chain_id);
    assert_eq!(answer(&view, &attestation)["status"], "attesting");
    view.thresholds.insert(attestation.chain_id, 1);
    view.signatures.clear();
    assert_eq!(answer(&view, &attestation)["status"], "ready");

    drop(journal);
    view.items = directory.journal().state().items.clone();
    assert_eq!(answer(&view, &attestation)["status"], "ready");
}
