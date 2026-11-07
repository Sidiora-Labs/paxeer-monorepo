#[path = "differential.rs"]
mod differential;

use layerx_intents::{golden, inspect_intent, IntentHeader, IntentKindTag, RejectReason};
use layerx_wire::encode::Encoder;

fn header(version: u16, kind: u16) -> Vec<u8> {
    let mut encoder = Encoder::new(6);
    encoder.u16(version).expect("version");
    encoder.u16(kind).expect("kind");
    encoder.fixed(&[0xde, 0xad]).expect("trailing bytes");
    encoder.finish()
}

fn assert_retired_tag(tag: u16) {
    for version in [1, 2] {
        let input = header(version, tag);
        for bytes in [&input[..4], input.as_slice()] {
            let rejected = inspect_intent(bytes).expect_err("retired kind accepted");
            assert_eq!(rejected.reason(), RejectReason::RetiredShape, "tag {tag}");
            assert_eq!(rejected.input(), bytes);
            assert_eq!(rejected.input().as_ptr(), bytes.as_ptr());
        }
    }
}

#[test]
fn retired_bridge_tags_answer_retired_shape() {
    for tag in [4, 11, 12] {
        assert_retired_tag(tag);
    }
    for (tag, kind) in [
        (1, IntentKindTag::DidRegistration),
        (2, IntentKindTag::KeyRotation),
        (3, IntentKindTag::RecoveryRegistration),
        (5, IntentKindTag::LxpSend),
        (6, IntentKindTag::LxpReceive),
        (7, IntentKindTag::PayerGrantRegistration),
        (8, IntentKindTag::BudgetCreate),
        (9, IntentKindTag::BudgetFund),
        (10, IntentKindTag::BudgetDefund),
        (13, IntentKindTag::SessionGrant),
        (14, IntentKindTag::SessionRevoke),
        (15, IntentKindTag::AuthorityGrant),
    ] {
        assert_eq!(kind as u16, tag);
        assert_eq!(
            inspect_intent(&header(1, tag)),
            Ok(IntentHeader { version: 1, kind })
        );
    }
    let input = header(1, 18);
    assert_eq!(
        inspect_intent(&input)
            .expect_err("unknown kind accepted")
            .reason(),
        RejectReason::UnknownKind(18)
    );
}

#[test]
fn retired_custody_credit_tag_answers_retired_shape() {
    assert_retired_tag(16);
    assert_eq!(IntentKindTag::NativeReceive as u16, 17);
    assert_eq!(
        inspect_intent(&header(1, 17)),
        Ok(IntentHeader {
            version: 1,
            kind: IntentKindTag::NativeReceive,
        })
    );
}

#[test]
fn golden_vectors_hold_no_bridge_entries() {
    for (version, source) in [
        (golden::V1_VERSION, golden::V1_SOURCE),
        (golden::V2_VERSION, golden::V2_SOURCE),
    ] {
        differential::assert_golden_vectors(source, version);
    }
}
