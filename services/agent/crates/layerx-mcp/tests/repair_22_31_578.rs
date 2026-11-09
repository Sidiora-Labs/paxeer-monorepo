use layerx_crypto::disclosure::call::{bind_call, ConstructMethod};
use layerx_crypto::disclosure::{AmountRole, CounterpartyRole};
use layerx_crypto::payments::Payment;
use layerx_crypto::send::CallOptions;
use layerx_mcp::tools::wallet::{
    draw_policy_dimensions, prepare_alias_call, prepare_alias_call_with_receive, WalletToolError,
};
use layerx_mcp::tools::write::PaymentTool;
use layerx_types::payload::ModuleId;

const ACTOR: &[u8] = b"did:layerx:alice";

fn receive() -> Payment {
    let text = include_str!("../../layerx-crypto/tests/fixtures/payments/native-1-6.hex").trim();
    let payload: Vec<u8> = text
        .as_bytes()
        .chunks_exact(2)
        .map(|pair| {
            u8::from_str_radix(std::str::from_utf8(pair).expect("hex text"), 16).expect("hex byte")
        })
        .collect();
    Payment::decode(ModuleId::Asset, 6, &payload, ACTOR).expect("authenticated Receive")
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
fn draw_dimensions_follow_the_encoded_receive_and_bound_ledger_draw() {
    let payment = receive();
    let payload = payment.encode(ACTOR).expect("payments encoder");
    let (call, decoded) =
        prepare_alias_call_with_receive(PaymentTool::DrawGrant, &payload, ACTOR, &options())
            .expect("prepare and retain decoded Receive");
    assert_eq!(decoded.as_ref(), Some(&payment));
    assert_eq!(
        prepare_alias_call(PaymentTool::DrawGrant, &payload, ACTOR, &options())
            .expect("existing alias entry point"),
        call
    );
    let disclosure = bind_call(&call, options().from).expect("bound LedgerDraw");
    let Payment::Receive {
        from,
        to,
        asset,
        amount,
        grant,
        idempotency_key,
        ..
    } = &payment
    else {
        panic!("Receive fixture");
    };
    assert!(matches!(
        disclosure.construct_method().expect("validated method"),
        Some(ConstructMethod::LedgerDraw {
            recipient,
            amount: draw_amount,
            grant: draw_grant,
            idempotency_key: draw_key,
            ..
        }) if recipient == *to && draw_amount == *amount
            && draw_grant == *grant && draw_key == *idempotency_key
    ));
    assert!(disclosure.payment.is_none());
    assert_eq!(disclosure.counterparties.len(), 1);
    assert_eq!(
        disclosure.counterparties[0].role,
        CounterpartyRole::Recipient
    );
    assert_eq!(disclosure.counterparties[0].account, *to);
    assert_eq!(disclosure.amounts.len(), 1);
    assert_eq!(disclosure.amounts[0].role, AmountRole::Transfer);
    assert_eq!(disclosure.amounts[0].value, *amount);
    assert_eq!(disclosure.idempotency_key, *idempotency_key);
    assert_ne!(disclosure.asset, *asset);
    assert_ne!(from, to);
    assert_eq!(
        draw_policy_dimensions(decoded.as_ref(), &disclosure).expect("DrawGrant dimensions"),
        (*amount, *asset, *from)
    );
}

#[test]
fn draw_dimensions_refuse_each_receive_mismatch_and_missing_receive() {
    let payment = receive();
    let payload = payment.encode(ACTOR).expect("payments encoder");
    let (call, decoded) =
        prepare_alias_call_with_receive(PaymentTool::DrawGrant, &payload, ACTOR, &options())
            .expect("prepared Receive");
    let disclosure = bind_call(&call, options().from).expect("bound LedgerDraw");
    for field in ["grant", "idempotency_key", "amount", "recipient"] {
        let mut mismatched = decoded.clone().expect("retained Receive");
        let Payment::Receive {
            grant,
            idempotency_key,
            amount,
            to,
            ..
        } = &mut mismatched
        else {
            panic!("Receive fixture");
        };
        match field {
            "grant" => grant[0] ^= 1,
            "idempotency_key" => idempotency_key[0] ^= 1,
            "amount" => *amount += 1,
            "recipient" => to[0] ^= 1,
            _ => unreachable!("declared mismatch"),
        }
        assert!(
            matches!(
                draw_policy_dimensions(Some(&mismatched), &disclosure),
                Err(WalletToolError::Arguments)
            ),
            "mismatched {field} must be refused"
        );
    }
    assert!(matches!(
        draw_policy_dimensions(None, &disclosure),
        Err(WalletToolError::Arguments)
    ));
    assert!(matches!(
        draw_policy_dimensions(
            Some(&Payment::RevokeGrant {
                grant: [1; 32],
                revocation_sequence: 0,
            }),
            &disclosure
        ),
        Err(WalletToolError::Arguments)
    ));
    assert!(matches!(
        prepare_alias_call_with_receive(PaymentTool::DrawGrant, &[], ACTOR, &options()),
        Err(WalletToolError::Arguments)
    ));
}

#[test]
fn draw_dimensions_refuse_other_methods_and_altered_disclosures() {
    let payment = receive();
    let payload = payment.encode(ACTOR).expect("payments encoder");
    let (call, decoded) =
        prepare_alias_call_with_receive(PaymentTool::DrawGrant, &payload, ACTOR, &options())
            .expect("prepared Receive");
    let disclosure = bind_call(&call, options().from).expect("bound LedgerDraw");
    let mut altered = disclosure.clone();
    altered.amounts[0].value += 1;
    assert!(matches!(
        draw_policy_dimensions(decoded.as_ref(), &altered),
        Err(WalletToolError::Arguments)
    ));

    let Payment::Receive { payer_grant, .. } = payment else {
        panic!("Receive fixture");
    };
    let issue = Payment::IssueGrant(*payer_grant);
    let payload = issue.encode(ACTOR).expect("grant encoder");
    let (call, receive) =
        prepare_alias_call_with_receive(PaymentTool::IssueGrant, &payload, ACTOR, &options())
            .expect("prepared IssueGrant");
    assert!(receive.is_none());
    let disclosure = bind_call(&call, options().from).expect("bound IssueGrant");
    assert_eq!(disclosure.payment.as_ref(), Some(&issue));
    assert!(matches!(
        draw_policy_dimensions(decoded.as_ref(), &disclosure),
        Err(WalletToolError::Arguments)
    ));
}
