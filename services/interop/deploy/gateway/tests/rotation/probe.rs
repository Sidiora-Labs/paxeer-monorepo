#[path = "../../../../crates/layerx-interop-service/src/config.rs"]
mod config;

use layerx_platform_gateway::http::{Endpoint, OutboundRequest};
use layerx_proof::receipt::{verify_sequencer_signature, ReceiptCheck};
use serde_json::{json, Value};

fn bytes(value: &Value) -> Result<Vec<u8>, String> {
    let text = value.as_str().ok_or("hex string")?;
    let text = text.strip_prefix("0x").ok_or("hex prefix")?;
    if text.len() % 2 != 0 {
        return Err("hex width".to_owned());
    }
    (0..text.len())
        .step_by(2)
        .map(|index| u8::from_str_radix(&text[index..index + 2], 16).map_err(|e| e.to_string()))
        .collect()
}

fn run() -> Result<(), String> {
    let config = config::load()?;
    let port = std::env::var("ROTATION_RECORDING_PORT")
        .map_err(|e| e.to_string())?
        .parse::<u16>()
        .map_err(|e| e.to_string())?;
    let endpoint = Endpoint::plain("127.0.0.1".to_owned(), port);
    let mut verified = Vec::new();
    for name in std::env::args().skip(1) {
        let response = config.client.request_unauthenticated(
            &endpoint,
            &OutboundRequest {
                method: "GET",
                path: &format!("/recorded/{name}"),
                body: &[],
                idempotency: None,
                content_type: "application/json",
            },
        )?;
        if response.status != 200 {
            return Err(format!("recording returned {}", response.status));
        }
        let document: Value = serde_json::from_slice(&response.body).map_err(|e| e.to_string())?;
        let (record, receipt, expected_height) = if name == "old" {
            let record = &document["cases"][0];
            (record, bytes(&record["signedReceipt"])?, 1000)
        } else {
            let index = match name.as_str() {
                "new" => 0,
                "outside" => 2,
                "unpinned" => 1,
                _ => return Err("unknown recording".to_owned()),
            };
            let record = &document["vectors"][index];
            (
                record,
                bytes(&record["receipt"])?,
                record["height"].as_u64().ok_or("height")?,
            )
        };
        let actual = verify_sequencer_signature(&receipt, config.receipt_keys.as_slice());
        if name == "old" || name == "new" {
            let decoded = actual.map_err(|e| format!("{name}: {e:?}"))?;
            let protocol = decoded.protocol().ok_or("protocol receipt")?;
            let height =
                u64::from_be_bytes(protocol.batch_id()[24..].try_into().map_err(|_| "height")?);
            let transaction = if name == "old" {
                &record["receiptJson"]["txHash"]
            } else {
                &record["txHash"]
            };
            if height != expected_height || protocol.activity_id().as_slice() != bytes(transaction)?
            {
                return Err(format!("{name}: recorded identity or height changed"));
            }
            verified.push(json!({"recording": name, "height": height}));
        } else {
            let error = actual.err().ok_or("unauthorized receipt was accepted")?;
            if error.check != ReceiptCheck::SequencerSignature {
                return Err(format!("{name}: wrong refusal: {error:?}"));
            }
            verified.push(json!({"recording": name, "refused": "SequencerSignature"}));
        }
    }
    if verified.is_empty() {
        return Err("no receipt checked".to_owned());
    }
    println!("{}", json!({"verified": verified}));
    Ok(())
}

fn main() {
    if let Err(error) = run() {
        eprintln!("rotation probe: {error}");
        std::process::exit(1);
    }
}

#[cfg(test)]
mod repair_m109 {
    use super::{bytes, verify_sequencer_signature, ReceiptCheck, Value};
    use layerx_proof::inclusion::SequencerAuthorization;

    #[test]
    fn rendered_example_windows_use_unambiguous_proof_keys() {
        let entries: Value = serde_json::from_str(include_str!("../../receipt-keys.example.json"))
            .expect("explicit renderer receipt-key inputs");
        let keys: Vec<_> = entries
            .as_array()
            .expect("receipt-key array")
            .iter()
            .map(|entry| {
                SequencerAuthorization::new(
                    bytes(&entry["keyId"]).unwrap().try_into().unwrap(),
                    bytes(&entry["publicKey"]).unwrap().try_into().unwrap(),
                    entry["firstHeight"].as_u64().unwrap(),
                    entry["lastHeight"].as_u64().unwrap(),
                )
            })
            .collect();
        assert_eq!(keys.len(), 2);
        let old: Value = serde_json::from_str(include_str!(
            "../../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
        ))
        .unwrap();
        let new: Value = serde_json::from_str(include_str!(
            "../../../../../../sdk/platform/conformance/fixtures/receipt-sequencer-v1.json"
        ))
        .unwrap();
        for (index, (recording, expected_height)) in [
            (&old["cases"][0]["signedReceipt"], 1000_u64),
            (&new["vectors"][0]["receipt"], 1200_u64),
        ]
        .into_iter()
        .enumerate()
        {
            let receipt = bytes(recording).unwrap();
            let verified = verify_sequencer_signature(&receipt, keys.as_slice()).unwrap();
            let protocol = verified.protocol().unwrap();
            assert_eq!(protocol.context_hash(), keys[index].sequencer_id());
            assert_eq!(
                u64::from_be_bytes(protocol.batch_id()[24..].try_into().unwrap()),
                expected_height
            );
            // Even a disjoint extra window with the same id is ambiguous.
            let mut duplicate = keys.clone();
            duplicate.push(SequencerAuthorization::new(
                keys[index].sequencer_id(),
                keys[index].public_key(),
                3000,
                3999,
            ));
            assert_eq!(
                verify_sequencer_signature(&receipt, duplicate.as_slice())
                    .unwrap_err()
                    .check,
                ReceiptCheck::SequencerSignature
            );
            let outside = [SequencerAuthorization::new(
                keys[index].sequencer_id(),
                keys[index].public_key(),
                expected_height + 1,
                expected_height + 2,
            )];
            assert_eq!(
                verify_sequencer_signature(&receipt, &outside)
                    .unwrap_err()
                    .check,
                ReceiptCheck::SequencerSignature
            );
            let mut tampered = receipt;
            *tampered.last_mut().unwrap() ^= 1;
            assert_eq!(
                verify_sequencer_signature(&tampered, keys.as_slice())
                    .unwrap_err()
                    .check,
                ReceiptCheck::SequencerSignature
            );
        }
    }
}
