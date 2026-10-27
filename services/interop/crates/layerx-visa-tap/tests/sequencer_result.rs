use layerx_interop_gateway::evidence::{SequencerEvidence, SettlementRung};
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_proof::sequencer_logs::SequencerLog;
use layerx_visa_tap::{MerchantOperationResult, TapError};
use serde_json::Value;

fn bytes(value: &Value) -> Vec<u8> {
    let hex = value
        .as_str()
        .and_then(|value| value.strip_prefix("0x"))
        .expect("fixture hex");
    assert_eq!(hex.len() % 2, 0);
    hex.as_bytes()
        .chunks_exact(2)
        .map(|pair| {
            u8::from_str_radix(std::str::from_utf8(pair).expect("hex digits"), 16)
                .expect("hex byte")
        })
        .collect()
}

fn fixed<const N: usize>(value: &Value) -> [u8; N] {
    bytes(value).try_into().expect("fixture field length")
}

fn recorded_payment() -> (SequencerEvidence, [SequencerAuthorization; 1], [u8; 32]) {
    let fixture: Value = serde_json::from_str(include_str!(
        "../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
    ))
    .expect("recorded sequencer fixture");
    let case = fixture["cases"]
        .as_array()
        .expect("fixture cases")
        .iter()
        .find(|case| case["id"] == "V1-layerxledger-pay-exact")
        .expect("recorded payExact case");
    let receipt = &case["receiptJson"];
    assert_eq!(receipt["rung"], "receipted");
    let logs = receipt["logs"]
        .as_array()
        .expect("recorded logs")
        .iter()
        .map(|log| {
            SequencerLog::new(
                fixed(&log["address"]),
                log["topics"]
                    .as_array()
                    .expect("log topics")
                    .iter()
                    .map(fixed)
                    .collect(),
                bytes(&log["data"]),
                u64::from_str_radix(
                    log["logIndex"]
                        .as_str()
                        .and_then(|index| index.strip_prefix("0x"))
                        .expect("log index"),
                    16,
                )
                .expect("hex log index"),
            )
        })
        .collect();
    let key = &case["pinnedKey"];
    (
        SequencerEvidence {
            transaction_hash: fixed(&receipt["txHash"]),
            signed_receipt: bytes(&case["signedReceipt"]),
            logs,
            rung: SettlementRung::Receipted,
        },
        [SequencerAuthorization::new(
            fixed(&key["keyId"]),
            fixed(&key["publicKey"]),
            key["firstHeight"].as_u64().expect("first pinned height"),
            key["lastHeight"].as_u64().expect("last pinned height"),
        )],
        fixed(&case["receiptDigest"]),
    )
}

#[test]
fn from_receipt_reads_the_pay_exact_log() {
    let (evidence, keys, receipt_digest) = recorded_payment();
    assert_eq!(evidence.logs.len(), 1);
    let reference = evidence.logs[0].topics()[3];
    let result = MerchantOperationResult::from_receipt(&evidence, &keys, reference);
    assert_eq!(
        result,
        Ok(MerchantOperationResult::ReceiptVerified {
            receipt_digest,
            from: fixed(&Value::from(
                "0x1fb0082aa4aa4ce1eb67568a7d8f67798f3b008c254e0929dfe5eabb1dc0d769"
            )),
            to: fixed(&Value::from(
                "0x84ddb3e2af095e5f24fbe8a6d3e161eb9ebae5d2292861f3287f14e0db317985"
            )),
            asset: fixed(&Value::from(
                "0x4e366c1f82bbcb0cfc80b6168346024ca11db454d4b5e93dc7cac57bc5292f28"
            )),
            amount: 1_000_000,
        })
    );

    for field in 0..4 {
        let mut tampered = evidence.clone();
        let log = &evidence.logs[0];
        let mut topics = log.topics().to_vec();
        let mut data = log.data().to_vec();
        match field {
            0 => topics[1][0] ^= 1,
            1 => topics[2][0] ^= 1,
            2 => data[0] ^= 1,
            3 => data[63] ^= 1,
            _ => unreachable!(),
        }
        tampered.logs[0] = SequencerLog::new(log.address(), topics, data, log.log_index());
        assert_eq!(
            MerchantOperationResult::from_receipt(&tampered, &keys, reference),
            Err(TapError::ReceiptMismatch),
            "served payment field {field} must match the signed log"
        );
    }
}

#[test]
fn from_receipt_refuses_another_reference() {
    let (evidence, keys, _) = recorded_payment();
    let reference = evidence.logs[0].topics()[3];
    assert!(MerchantOperationResult::from_receipt(&evidence, &keys, reference).is_ok());
    let mut other_reference = reference;
    other_reference[0] ^= 1;
    assert_eq!(
        MerchantOperationResult::from_receipt(&evidence, &keys, other_reference),
        Err(TapError::ReceiptMismatch)
    );
}
