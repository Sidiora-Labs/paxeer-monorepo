use std::collections::BTreeMap;

use serde_json::{json, Value};

use crate::attestation::recover_signer;
use crate::hex;
use crate::journal::{inbound_key, outbound_key, Item};

#[derive(Clone, Debug, Default)]
pub struct AttestationView {
    pub items: BTreeMap<String, Item>,
    pub signatures: BTreeMap<[u8; 32], Vec<[u8; 65]>>,
    pub thresholds: BTreeMap<u64, usize>,
}

#[must_use]
pub fn deposit_answer(
    view: &AttestationView,
    chain_id: u64,
    tx_hash: &[u8; 32],
    log_index: u64,
) -> Option<Value> {
    let item = view.items.get(&inbound_key(chain_id, tx_hash, log_index))?;
    let attestation = item.observation.inbound_attestation()?;
    let digest = attestation.digest();
    let signatures = signatures(view, item, &digest);
    let status = status(view, item, chain_id, signatures.len());
    Some(json!({
        "chainId": attestation.chain_id.to_string(),
        "vault": hex::prefixed(&attestation.vault),
        "txHash": hex::prefixed(&attestation.tx_hash),
        "logIndex": attestation.log_index.to_string(),
        "recipient": hex::prefixed(&attestation.recipient),
        "asset": hex::prefixed(&attestation.asset),
        "amount": decimal(&attestation.amount),
        "digest": hex::prefixed(&digest),
        "signatures": signatures,
        "status": status,
    }))
}

#[must_use]
pub fn release_answer(
    view: &AttestationView,
    chain_id: u64,
    paxeer_tx_hash: &[u8; 32],
    paxeer_nonce: u64,
) -> Option<Value> {
    let item = view
        .items
        .get(&outbound_key(chain_id, paxeer_tx_hash, paxeer_nonce))?;
    let attestation = item.observation.outbound_attestation()?;
    let digest = attestation.digest();
    let signatures = signatures(view, item, &digest);
    let status = status(view, item, chain_id, signatures.len());
    Some(json!({
        "chainId": attestation.chain_id.to_string(),
        "vault": hex::prefixed(&attestation.vault),
        "paxeerTxHash": hex::prefixed(&attestation.paxeer_tx_hash),
        "paxeerNonce": attestation.paxeer_nonce.to_string(),
        "recipient": hex::prefixed(&attestation.recipient),
        "asset": hex::prefixed(&attestation.asset),
        "amount": decimal(&attestation.amount),
        "digest": hex::prefixed(&digest),
        "signatures": signatures,
        "status": status,
    }))
}

fn signatures(view: &AttestationView, item: &Item, digest: &[u8; 32]) -> Vec<String> {
    let mut by_signer = BTreeMap::new();
    for signature in item
        .signature
        .iter()
        .chain(view.signatures.get(digest).into_iter().flatten())
    {
        if let Ok(signer) = recover_signer(digest, signature) {
            by_signer.entry(signer).or_insert(*signature);
        }
    }
    by_signer
        .values()
        .map(|signature| hex::prefixed(signature))
        .collect()
}

fn status(view: &AttestationView, item: &Item, chain_id: u64, signatures: usize) -> &'static str {
    if item.signature.is_none() {
        "observed"
    } else if view
        .thresholds
        .get(&chain_id)
        .is_some_and(|threshold| *threshold > 0 && signatures >= *threshold)
    {
        "ready"
    } else {
        "attesting"
    }
}

fn decimal(bytes: &[u8; 32]) -> String {
    let mut digits = vec![0_u16];
    for byte in bytes {
        let mut carry = u16::from(*byte);
        for digit in &mut digits {
            carry += *digit * 256;
            *digit = carry % 10;
            carry /= 10;
        }
        while carry != 0 {
            digits.push(carry % 10);
            carry /= 10;
        }
    }
    digits
        .iter()
        .rev()
        .map(|digit| char::from(b"0123456789"[usize::from(*digit)]))
        .collect()
}
