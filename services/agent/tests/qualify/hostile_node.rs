use std::collections::BTreeSet;

use ed25519_dalek::{Signer as _, SigningKey};
use layerx_client::rpc::commit::{decode_validators, verify_commit_material, CommitError};
use layerx_client::rpc::receipt_commit::{verify_receipt_coverage, ReceiptCommitEvidence};
use layerx_client::sequencer::ReceiptKeyEntry;
use layerx_crypto::evm_transaction::transaction_hash;
use layerx_programs::chain_proof::{
    merkle::SimpleProof,
    validators::{TrustedValidator, ValidatorSet},
    ChainProofVerifier, VerifiedChainHead,
};
use layerx_proof::availability::{
    verify_reassembled, AvailabilityCheck, ReassembledRecords, RootCommitments,
};
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_sdk::rpc_verification::{verify_node_receipt, ReceiptPolicy};
use layerx_wire::receipt::{decode, encode_unsigned, Receipt};
use serde_json::{json, Value};

pub const OWNER_TRANSCRIPT: &str =
    include_str!("../../crates/layerx-sdk/src/repair_22_31_227_fixture/committed.json");
const RECEIPT_KEY: &str = "aed3d83b59b735683edb641862d13d6855f37284c190b7b501a46ca29085b239";
const RECEIPT_KEY_ID: &str = "ba3a5a0bba18e7ead9fd281c793daaec55218c5d084483346e45c959a92bf699";
const VALIDATOR_KEY: &str = "c614a1c91884c3c13bed586a16d51f7253072f7cc41018ea28dd864e4e548e4d";

#[derive(Clone, Copy, Debug, Eq, Ord, PartialEq, PartialOrd)]
pub enum HostileAttack {
    AlteredBalance,
    AlteredReceipt,
    ResignedReceipt,
    SubthresholdCertificate,
    TruncatedProof,
    ReorderedEvents,
    WithheldAvailability,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct Refusal {
    pub consumer: &'static str,
    pub reason: String,
}

fn refusal(consumer: &'static str, error: impl std::fmt::Debug) -> Refusal {
    Refusal {
        consumer,
        reason: format!("{error:?}"),
    }
}

fn record_error(error: impl std::fmt::Debug) -> Refusal {
    refusal("record-codec", error)
}

pub fn unhex(text: &str) -> Result<Vec<u8>, String> {
    let text = text.strip_prefix("0x").unwrap_or(text);
    if !text.len().is_multiple_of(2) {
        return Err("odd hex length".to_owned());
    }
    text.as_bytes()
        .chunks_exact(2)
        .map(|pair| {
            std::str::from_utf8(pair)
                .map_err(|error| error.to_string())
                .and_then(|pair| u8::from_str_radix(pair, 16).map_err(|error| error.to_string()))
        })
        .collect()
}

pub fn hex(bytes: &[u8]) -> String {
    format!(
        "0x{}",
        bytes
            .iter()
            .map(|byte| format!("{byte:02x}"))
            .collect::<String>()
    )
}

fn field<'a>(value: &'a Value, name: &str) -> Result<&'a str, String> {
    value[name]
        .as_str()
        .ok_or_else(|| format!("missing text field {name}"))
}

fn fixed(text: &str) -> Result<[u8; 32], String> {
    unhex(text)?
        .try_into()
        .map_err(|_| "wrong digest width".to_owned())
}

fn quantity(value: &Value, name: &str) -> Result<u64, String> {
    let text = field(value, name)?
        .strip_prefix("0x")
        .ok_or("missing quantity prefix")?;
    u64::from_str_radix(text, 16).map_err(|error| error.to_string())
}

pub fn owner_record() -> Result<Value, String> {
    serde_json::from_str(OWNER_TRANSCRIPT).map_err(|error| format!("owner transcript: {error}"))
}

pub struct VerifiedMaterial {
    pub receipt: Receipt,
    pub canonical: Vec<u8>,
    pub head: VerifiedChainHead,
}

/// Verify the recorded node's actual receipt, Comet commit and sequence-batch
/// inclusion under pins retained independently of the served record.
pub fn verify_owner_record(record: &Value) -> Result<VerifiedMaterial, Refusal> {
    let receipt_key = ReceiptKeyEntry {
        key_id: fixed(RECEIPT_KEY_ID).map_err(record_error)?,
        public_key: fixed(RECEIPT_KEY).map_err(record_error)?,
        first_height: 10,
        last_height: 10,
    };
    let raw =
        unhex(field(record, "rawTransaction").map_err(record_error)?).map_err(record_error)?;
    let activity = transaction_hash(&raw);
    let policy = ReceiptPolicy {
        protocol_version: 2,
        network_id: 125,
        sequencer: SequencerAuthorization::new(receipt_key.key_id, receipt_key.public_key, 10, 10),
        trusted_checkpoint_context_digest: None,
    };
    let (receipt, canonical) = verify_node_receipt(&record["receiptJson"], activity, &policy)
        .map_err(|error| refusal("receipt-verifier", error))?;
    let verifier = ChainProofVerifier {
        chain_id: "committed-fixture".to_owned(),
        validators: ValidatorSet::new(vec![TrustedValidator::new(
            fixed(VALIDATOR_KEY).map_err(record_error)?,
            7_000_000_000,
        )])
        .map_err(|error| refusal("validator-pins", error))?,
    };
    let supplied = record["validators"]["validators"]
        .as_array()
        .ok_or_else(|| record_error("served validators"))?;
    let supplied = json!({"validators": supplied.iter().map(|entry| json!({
        "address":entry["address"], "pubKey":entry["pub_key"], "votingPower":entry["voting_power"],
    })).collect::<Vec<_>>()});
    let supplied =
        decode_validators(&supplied).map_err(|error| refusal("validator-pins", error))?;
    if supplied != verifier.validators {
        return Err(refusal(
            "validator-pins",
            "served validators differ from independent pins",
        ));
    }
    let head = verify_commit_material(&record["commit"]["signed_header"], &verifier)
        .map_err(|error| refusal("commit-verifier", error))?;
    let served = &record["receiptJson"];
    if served["rung"] != "committed"
        || quantity(served, "blockNumber").map_err(record_error)? != head.height() as u64
        || fixed(field(served, "blockHash").map_err(record_error)?).map_err(record_error)?
            != head.header_hash()
    {
        return Err(refusal(
            "block-binding",
            "receipt and authenticated block differ",
        ));
    }
    let path = &record["batch"]["proof"];
    let proof = SimpleProof {
        total: path["total"]
            .as_i64()
            .ok_or_else(|| record_error("proof total"))?,
        index: path["index"]
            .as_i64()
            .ok_or_else(|| record_error("proof index"))?,
        leaf_hash: fixed(field(path, "leafHash").map_err(record_error)?).map_err(record_error)?,
        aunts: path["aunts"]
            .as_array()
            .ok_or_else(|| record_error("proof aunts"))?
            .iter()
            .map(|v| fixed(v.as_str().ok_or_else(|| "proof aunt".to_owned())?))
            .collect::<Result<_, String>>()
            .map_err(record_error)?,
    };
    let batch =
        unhex(field(&record["batch"], "tx").map_err(record_error)?).map_err(record_error)?;
    let activities = [activity];
    let evidence = ReceiptCommitEvidence {
        verifier: &verifier,
        covering_height: head.height() as u64,
        batch_transaction: &batch,
        transaction_proof: &proof,
        activity_ids: &activities,
    };
    verify_receipt_coverage(
        receipt
            .protocol()
            .ok_or_else(|| record_error("protocol receipt"))?,
        quantity(served, "height").map_err(record_error)?,
        &head,
        &evidence,
        &[receipt_key],
    )
    .map_err(|error| refusal("receipt-coverage", error))?;
    Ok(VerifiedMaterial {
        receipt,
        canonical,
        head,
    })
}

/// Each mutation starts from the same accepted producer transcript.
pub fn attack_record(attack: HostileAttack) -> Result<Value, String> {
    let mut record = owner_record()?;
    match attack {
        HostileAttack::AlteredBalance => {
            let mut data = unhex(field(&record["receiptJson"]["logs"][0], "data")?)?;
            *data.last_mut().ok_or("missing payment value")? ^= 1;
            record["receiptJson"]["logs"][0]["data"] = json!(hex(&data));
        }
        HostileAttack::AlteredReceipt | HostileAttack::ResignedReceipt => {
            let mut bytes = unhex(field(&record["receiptJson"], "receipt")?)?;
            if attack == HostileAttack::AlteredReceipt {
                *bytes.last_mut().ok_or("missing signature")? ^= 1;
            } else {
                let decoded =
                    decode(&bytes).map_err(|error| format!("receipt decode: {error:?}"))?;
                let unsigned = encode_unsigned(&decoded)
                    .map_err(|error| format!("receipt encode: {error:?}"))?;
                let digest = layerx_wire::hash::receipt_digest(&unsigned)
                    .map_err(|error| format!("receipt digest: {error:?}"))?;
                let attacker = SigningKey::from_bytes(&[0x57; 32]);
                let signature = attacker.sign(&digest).to_bytes();
                layerx_crypto::ed25519::verify_digest(
                    &attacker.verifying_key().to_bytes(),
                    &signature,
                    &digest,
                )
                .map_err(|error| format!("attacker signature: {error:?}"))?;
                let offset = bytes
                    .len()
                    .checked_sub(64)
                    .ok_or("missing signature bytes")?;
                bytes[offset..].copy_from_slice(&signature);
                record["receiptJson"]["signingKey"] =
                    json!(hex(&attacker.verifying_key().to_bytes()));
            }
            record["receiptJson"]["receipt"] = json!(hex(&bytes));
        }
        HostileAttack::SubthresholdCertificate => {
            record["commit"]["signed_header"]["commit"]["signatures"][0] = json!({
                "block_id_flag":1, "validator_address":"",
                "timestamp":"0001-01-01T00:00:00Z", "signature":null,
            });
        }
        HostileAttack::TruncatedProof => {
            record["batch"]["proof"]["aunts"]
                .as_array_mut()
                .ok_or("missing inclusion path")?
                .pop()
                .ok_or("empty inclusion path")?;
        }
        HostileAttack::ReorderedEvents => {
            let logs = record["receiptJson"]["logs"]
                .as_array_mut()
                .ok_or("missing logs")?;
            if logs.len() < 2 {
                return Err("producer needs two events".to_owned());
            }
            logs.swap(0, 1);
        }
        HostileAttack::WithheldAvailability => {
            return Err("availability uses its own consumer".to_owned())
        }
    }
    Ok(record)
}

pub fn reject_attack(attack: HostileAttack) -> Result<Refusal, String> {
    let original = owner_record()?;
    let verified = verify_owner_record(&original)
        .map_err(|error| format!("positive owner control failed: {error:?}"))?;
    if attack == HostileAttack::WithheldAvailability {
        let empty: [&[u8]; 0] = [];
        let receipts = [verified.canonical.as_slice()];
        let records = ReassembledRecords {
            activities: &empty,
            receipts: &receipts,
            events: &empty,
            oracle_inputs: &empty,
        };
        let result = verify_reassembled(
            &[],
            &records,
            RootCommitments {
                activity: verified.head.data_hash(),
                receipt: verified
                    .receipt
                    .protocol()
                    .ok_or("protocol receipt")?
                    .resulting_state_root(),
                event: verified.head.last_results_hash(),
                oracle: verified.head.app_hash(),
            },
        );
        return match result {
            Err(failure) if failure.check == AvailabilityCheck::MissingClass => {
                Ok(refusal("availability-verifier", failure.check))
            }
            other => Err(format!("withheld availability was not refused: {other:?}")),
        };
    }
    let damaged = attack_record(attack)?;
    let rejected = match verify_owner_record(&damaged) {
        Err(rejected) => rejected,
        Ok(_) => return Err(format!("hostile {attack:?} passed verification")),
    };
    let expected = match attack {
        HostileAttack::SubthresholdCertificate => "commit-verifier",
        HostileAttack::TruncatedProof => "receipt-coverage",
        _ => "receipt-verifier",
    };
    if rejected.consumer != expected {
        return Err(format!(
            "{attack:?} failed before its real consumer: {rejected:?}"
        ));
    }
    if attack == HostileAttack::SubthresholdCertificate
        && rejected.reason
            != format!(
                "{:?}",
                CommitError::Proof(
                    layerx_programs::chain_proof::ChainProofError::InsufficientPower,
                )
            )
    {
        return Err(format!(
            "certificate never reached quorum verification: {rejected:?}"
        ));
    }
    Ok(rejected)
}

/// Recorded receipt-key corpus includes a real signature outside the active
/// key table and a previously authorized key used after its height interval.
pub fn verify_receipt_authority_cases() -> Result<BTreeSet<String>, String> {
    use layerx_proof::receipt::verify_sequencer_signature;
    let corpus: Value = serde_json::from_str(include_str!(
        "../../../../sdk/platform/conformance/fixtures/receipt-sequencer-v1.json"
    ))
    .map_err(|error| error.to_string())?;
    let mut observed = BTreeSet::new();
    for case in corpus["vectors"].as_array().ok_or("receipt vectors")? {
        let pins = case["receiptKeys"]
            .as_array()
            .ok_or("receipt keys")?
            .iter()
            .map(|key| {
                Ok(SequencerAuthorization::new(
                    fixed(field(key, "keyId")?)?,
                    fixed(field(key, "publicKey")?)?,
                    key["firstHeight"].as_u64().ok_or("first key height")?,
                    key["lastHeight"].as_u64().ok_or("last key height")?,
                ))
            })
            .collect::<Result<Vec<_>, String>>()?;
        let canonical = unhex(field(case, "receipt")?)?;
        let result = verify_sequencer_signature(&canonical, pins.as_slice());
        let expected = field(&case["expected"], "verifier")?;
        match result {
            Ok(_) if expected == "accepts" => {}
            Err(error) if format!("{:?}", error.check) == expected => {}
            other => {
                return Err(format!(
                    "receipt authority corpus {}: {other:?}",
                    field(case, "id")?
                ))
            }
        }
        observed.insert(field(case, "id")?.to_owned());
    }
    if observed.len() != 7 {
        return Err(format!("receipt authority coverage drift: {observed:?}"));
    }
    Ok(observed)
}

/// Reports coverage only after each real verification consumer refuses its attack.
pub fn agent_hostile_node_harness() -> Result<BTreeSet<HostileAttack>, String> {
    verify_receipt_authority_cases()?;
    let mut rejected = BTreeSet::new();
    for attack in [
        HostileAttack::AlteredBalance,
        HostileAttack::AlteredReceipt,
        HostileAttack::ResignedReceipt,
        HostileAttack::SubthresholdCertificate,
        HostileAttack::TruncatedProof,
        HostileAttack::ReorderedEvents,
        HostileAttack::WithheldAvailability,
    ] {
        reject_attack(attack)?;
        rejected.insert(attack);
    }
    Ok(rejected)
}
