use layerx_mcp_test_support::option_assertion;
use layerx_mcp_test_support::result_assertion;

use layerx_crypto::disclosure::call::{bind_call, ledger_selector, ConstructMethod};
use layerx_crypto::evm_transaction::Eip1559Call;
use layerx_crypto::payments::Payment;
use layerx_crypto::send::CallOptions;
use layerx_mcp::catalogue::native_alias_target;
use layerx_mcp::tools::wallet::prepare_alias_call;
use layerx_mcp::tools::write::PaymentTool;
use layerx_types::payload::ModuleId;

fn payload(text: &str) -> Vec<u8> {
    let text = text.trim();
    assert_eq!(text.len() % 2, 0);
    text.as_bytes()
        .chunks_exact(2)
        .map(|pair| {
            result_assertion::required(
                u8::from_str_radix(
                    result_assertion::required(std::str::from_utf8(pair), "hex text"),
                    16,
                ),
                "hex byte",
            )
        })
        .collect()
}

fn options() -> CallOptions {
    CallOptions {
        from: [9; 20],
        chain_id: 125,
        nonce: 7,
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 5,
        gas_limit: 100_000,
    }
}

#[test]
fn wallet_send_targets_the_ledger_precompile() {
    let payload = payload(include_str!(
        "../../layerx-crypto/tests/fixtures/payments/native-1-5.hex"
    ));
    let mut address = [0; 20];
    address[18..].copy_from_slice(&[0x10, 0x1a]);
    for tool in [PaymentTool::Send, PaymentTool::Transfer] {
        let call = result_assertion::required(
            prepare_alias_call(tool, &payload, b"did:layerx:alice", &options()),
            "prepare ledger transfer",
        );
        assert_eq!(call.to, address);
        assert_eq!(call.chain_id, 125);
        assert_eq!(call.nonce, 7);
        assert_eq!(&call.data[..4], &ledger_selector("send(bytes)"));
        assert_eq!(
            option_assertion::required(native_alias_target(tool.name()), "alias").address,
            call.to
        );
        let disclosure =
            result_assertion::required(bind_call(&call, options().from), "ledger disclosure");
        assert_eq!(
            result_assertion::required(disclosure.construct_method(), "method"),
            Some(ConstructMethod::LedgerSend {
                payload: payload.clone()
            })
        );
        assert_eq!(disclosure.activity_type.module(), ModuleId::Asset);
        assert_eq!(disclosure.activity_type.value(), (1 << 16) | 5);
    }
    assert!(prepare_alias_call(PaymentTool::Send, &[], b"did:layerx:alice", &options()).is_err());
}

#[test]
fn grant_draw_targets_the_ledger_precompile() {
    let payload = payload(include_str!(
        "../../layerx-crypto/tests/fixtures/payments/native-1-6.hex"
    ));
    let payment = result_assertion::required(
        Payment::decode(ModuleId::Asset, 6, &payload, b"did:layerx:alice"),
        "authenticated receive",
    );
    let Payment::Receive {
        grant,
        to,
        amount,
        sequence,
        idempotency_key,
        context_hash,
        payer_grant,
        ..
    } = payment
    else {
        panic!("receive fixture");
    };
    let call: Eip1559Call = result_assertion::required(
        prepare_alias_call(
            PaymentTool::DrawGrant,
            &payload,
            b"did:layerx:alice",
            &options(),
        ),
        "prepare ledger draw",
    );
    let mut address = [0; 20];
    address[18..].copy_from_slice(&[0x10, 0x1a]);
    assert_eq!(call.to, address);
    assert_eq!(call.chain_id, 125);
    assert_eq!(call.nonce, 7);
    assert_eq!(
        option_assertion::required(native_alias_target("grant.draw"), "alias").ordinal,
        6
    );
    assert_eq!(
        option_assertion::required(native_alias_target("grant.draw"), "alias").address,
        address
    );
    assert_eq!(
        &call.data[..4],
        &ledger_selector("draw(bytes32,bytes32,uint128,bytes32,uint64,bytes32,bytes32)")
    );
    let mut amount_word = [0; 32];
    amount_word[16..].copy_from_slice(&amount.to_be_bytes());
    let mut sequence_word = [0; 32];
    sequence_word[24..].copy_from_slice(&sequence.to_be_bytes());
    assert_eq!(
        &call.data[4..],
        [
            grant,
            to,
            amount_word,
            idempotency_key,
            sequence_word,
            payer_grant.purpose_hash,
            context_hash
        ]
        .concat()
    );
    let disclosure =
        result_assertion::required(bind_call(&call, options().from), "ledger draw disclosure");
    assert_eq!(
        result_assertion::required(disclosure.construct_method(), "method"),
        Some(ConstructMethod::LedgerDraw {
            recipient: to,
            purpose_hash: payer_grant.purpose_hash,
            amount,
            grant,
            receiver_sequence: sequence,
            idempotency_key,
            context_hash,
        })
    );
    assert_eq!(disclosure.actor, options().from);
    assert_eq!(disclosure.activity_type.module(), ModuleId::Asset);
    assert_eq!(disclosure.activity_type.value(), (1 << 16) | 6);
    assert!(
        prepare_alias_call(PaymentTool::DrawGrant, &[], b"did:layerx:alice", &options()).is_err()
    );
}

#[test]
fn grant_issue_targets_the_ledger_precompile() {
    let payload = payload(include_str!(
        "../../layerx-crypto/tests/fixtures/payments/native-1-7.hex"
    ));
    let payment = result_assertion::required(
        Payment::decode(ModuleId::Asset, 7, &payload, b"did:layerx:alice"),
        "authenticated grant",
    );
    let Payment::IssueGrant(grant) = payment else {
        panic!("grant fixture");
    };
    let call: Eip1559Call = result_assertion::required(
        prepare_alias_call(
            PaymentTool::IssueGrant,
            &payload,
            b"did:layerx:alice",
            &options(),
        ),
        "prepare ledger grant",
    );
    let mut address = [0; 20];
    address[18..].copy_from_slice(&[0x10, 0x1a]);
    assert_eq!(call.to, address);
    assert_eq!(call.chain_id, 125);
    assert_eq!(call.nonce, 7);
    assert_eq!(
        option_assertion::required(native_alias_target("grant.issue"), "alias").ordinal,
        7
    );
    assert_eq!(
        option_assertion::required(native_alias_target("grant.issue"), "alias").address,
        address
    );
    assert_eq!(&call.data[..4], &ledger_selector("issueGrant(bytes)"));
    let disclosure =
        result_assertion::required(bind_call(&call, options().from), "ledger grant disclosure");
    assert_eq!(
        result_assertion::required(disclosure.construct_method(), "method"),
        Some(ConstructMethod::LedgerIssueGrant { grant })
    );
    assert_eq!(disclosure.actor, options().from);
    assert_eq!(disclosure.activity_type.module(), ModuleId::Asset);
    assert_eq!(disclosure.activity_type.value(), (1 << 16) | 7);
    assert!(prepare_alias_call(
        PaymentTool::IssueGrant,
        &[],
        b"did:layerx:alice",
        &options()
    )
    .is_err());
}

use layerx_mcp_test_support::native_alias as native_exchange;

#[test]
fn daemon_alias_accepts_producer_calls_and_external_signatures() {
    native_exchange::native_unsigned_call_prepare_disclose_sign_and_submit_binding();
}

#[test]
fn daemon_alias_refuses_altered_call_and_disclosure_fields() {
    native_exchange::native_call_rejects_every_changed_response_and_intent_field();
    native_exchange::native_legacy_owner_contract_remains_refused();
}
