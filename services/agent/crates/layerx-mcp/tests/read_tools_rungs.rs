use layerx_agent_api::identity::Asset;
use layerx_agent_api::prepare::CanonicalBytes;
use layerx_agent_api::read::{AccountRef, BalanceValue, Freshness, RelativeTo};
use layerx_agent_api::verify::Rung;
use layerx_agent_api::Amount;
use layerx_mcp::tools::read::{balance, receipt, ReadToolError, ReceiptValue};
use layerx_proof::inclusion::SequencerAuthorization;
use serde_json::Value;

fn hex(value: &Value) -> Vec<u8> {
    let text = value.as_str().expect("fixture hex string");
    let text = text.strip_prefix("0x").expect("prefixed fixture hex");
    assert_eq!(text.len() % 2, 0);
    (0..text.len())
        .step_by(2)
        .map(|offset| u8::from_str_radix(&text[offset..offset + 2], 16).expect("fixture byte"))
        .collect()
}

fn digest(value: &Value) -> [u8; 32] {
    hex(value).try_into().expect("fixture digest")
}

fn freshness(height: u64) -> Freshness {
    Freshness {
        chain_head: height + 3,
        value_height: height,
        relative_to: RelativeTo::Block {
            height,
            hash: [0x41; 32],
        },
    }
}

#[test]
fn receipt_tool_reports_the_receipted_rung() {
    let document: Value = serde_json::from_str(include_str!(
        "../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
    ))
    .expect("shared sequencer receipt corpus");
    let case = &document["cases"][0];
    assert_eq!(case["id"], "V1-pax-transfer");
    let canonical = hex(&case["signedReceipt"]);
    let key = &case["pinnedKey"];
    let keys = [SequencerAuthorization::new(
        digest(&key["keyId"]),
        digest(&key["publicKey"]),
        key["firstHeight"].as_u64().expect("first key height"),
        key["lastHeight"].as_u64().expect("last key height"),
    )];
    let value = ReceiptValue::from_sequencer_signed(
        CanonicalBytes::new(canonical.clone()).expect("canonical receipt"),
        &keys,
    )
    .expect("real sequencer-signed receipt verifies");
    let height = value.height;
    assert_eq!(height, 1000);
    assert_eq!(value.evidence_ids, vec![digest(&key["keyId"])]);
    let mut tampered = canonical.clone();
    *tampered.last_mut().expect("signature byte") ^= 1;
    assert_eq!(
        ReceiptValue::from_sequencer_signed(
            CanonicalBytes::new(tampered).expect("tampered receipt bytes"),
            &keys,
        ),
        Err(ReadToolError::Unverified)
    );
    let expected = freshness(height);
    let result = receipt(value.clone(), Some(Rung::Receipted), expected.clone(), 4096)
        .expect("receipt tool result");
    assert_eq!(result.rung, Some(Rung::Receipted));
    assert_eq!(result.freshness, expected);
    assert_eq!(result.value.height, height);
    assert_eq!(result.value.canonical_receipt.as_bytes(), canonical);
    assert!(result.page.complete);
    assert_eq!(result.page.returned_bytes, canonical.len() + 32);
    for rung in [None, Some(Rung::Submitted)] {
        assert_eq!(
            receipt(value.clone(), rung, freshness(height), 4096),
            Err(ReadToolError::Unverified)
        );
    }
    assert_eq!(
        receipt(value, Some(Rung::Receipted), freshness(height + 1), 4096),
        Err(ReadToolError::HeightMismatch)
    );
}

#[test]
fn balance_tool_has_no_rung_for_ordinary_reads() {
    let height = (1_u64 << 53) + 17;
    let expected = freshness(height);
    let value = BalanceValue {
        account: AccountRef::new("did:layerx:reader").expect("account"),
        asset: Asset::new("LXP").expect("asset"),
        amount: Amount(42),
        canonical_state: CanonicalBytes::new(42_u128.to_be_bytes().to_vec())
            .expect("canonical state"),
        height,
    };
    let result = balance(value.clone(), None, expected.clone(), 1024).expect("ordinary balance");
    assert_eq!(result.rung, None);
    assert_eq!(result.freshness, expected);
    assert_eq!(result.value, value);
    assert_eq!(result.freshness.value_height, height);
    assert!(
        matches!(result.freshness.relative_to, RelativeTo::Block { height: h, .. } if h == height)
    );
    assert!(result.page.complete);
    assert!(!result.page.explicitly_truncated);
    assert_eq!(result.page.returned_bytes, 16);
    assert_eq!(
        balance(value.clone(), None, freshness(height + 1), 1024),
        Err(ReadToolError::HeightMismatch)
    );
    assert_eq!(
        balance(value, None, expected, 15),
        Err(ReadToolError::ResultTooLarge)
    );
}
