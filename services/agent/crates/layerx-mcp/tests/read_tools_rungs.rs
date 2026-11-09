use layerx_mcp_test_support::option_assertion;
use layerx_mcp_test_support::result_assertion;

use layerx_agent_api::identity::Asset;
use layerx_agent_api::prepare::CanonicalBytes;
use layerx_agent_api::read::{AccountRef, BalanceValue, Freshness, RelativeTo};
use layerx_agent_api::verify::Rung;
use layerx_agent_api::Amount;
use layerx_mcp::tools::read::{balance, receipt, ReadToolError, ReceiptValue};
use layerx_proof::inclusion::SequencerAuthorization;
use serde_json::Value;

fn hex(value: &Value) -> Vec<u8> {
    let text = option_assertion::required(value.as_str(), "fixture hex string");
    let text = option_assertion::required(text.strip_prefix("0x"), "prefixed fixture hex");
    assert_eq!(text.len() % 2, 0);
    (0..text.len())
        .step_by(2)
        .map(|offset| {
            result_assertion::required(
                u8::from_str_radix(&text[offset..offset + 2], 16),
                "fixture byte",
            )
        })
        .collect()
}

fn digest(value: &Value) -> [u8; 32] {
    result_assertion::required(hex(value).try_into(), "fixture digest")
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
    let document: Value = result_assertion::required(
        serde_json::from_str(include_str!(
            "../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
        )),
        "shared sequencer receipt corpus",
    );
    let case = &document["cases"][0];
    assert_eq!(case["id"], "V1-pax-transfer");
    let canonical = hex(&case["signedReceipt"]);
    let key = &case["pinnedKey"];
    let keys = [SequencerAuthorization::new(
        digest(&key["keyId"]),
        digest(&key["publicKey"]),
        option_assertion::required(key["firstHeight"].as_u64(), "first key height"),
        option_assertion::required(key["lastHeight"].as_u64(), "last key height"),
    )];
    let value = result_assertion::required(
        ReceiptValue::from_sequencer_signed(
            result_assertion::required(CanonicalBytes::new(canonical.clone()), "canonical receipt"),
            &keys,
        ),
        "real sequencer-signed receipt verifies",
    );
    let height = value.height;
    assert_eq!(height, 1000);
    assert_eq!(value.evidence_ids, vec![digest(&key["keyId"])]);
    let mut tampered = canonical.clone();
    *option_assertion::required(tampered.last_mut(), "signature byte") ^= 1;
    assert_eq!(
        ReceiptValue::from_sequencer_signed(
            result_assertion::required(CanonicalBytes::new(tampered), "tampered receipt bytes"),
            &keys,
        ),
        Err(ReadToolError::Unverified)
    );
    let expected = freshness(height);
    let result = result_assertion::required(
        receipt(value.clone(), Some(Rung::Receipted), expected.clone(), 4096),
        "receipt tool result",
    );
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
        account: result_assertion::required(AccountRef::new("did:layerx:reader"), "account"),
        asset: result_assertion::required(Asset::new("LXP"), "asset"),
        amount: Amount(42),
        canonical_state: result_assertion::required(
            CanonicalBytes::new(42_u128.to_be_bytes().to_vec()),
            "canonical state",
        ),
        height,
    };
    let result = result_assertion::required(
        balance(value.clone(), None, expected.clone(), 1024),
        "ordinary balance",
    );
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
