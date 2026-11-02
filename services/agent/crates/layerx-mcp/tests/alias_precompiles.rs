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
            u8::from_str_radix(std::str::from_utf8(pair).expect("hex text"), 16).expect("hex byte")
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
        let call = prepare_alias_call(tool, &payload, b"did:layerx:alice", &options())
            .expect("prepare ledger transfer");
        assert_eq!(call.to, address);
        assert_eq!(call.chain_id, 125);
        assert_eq!(call.nonce, 7);
        assert_eq!(&call.data[..4], &ledger_selector("send(bytes)"));
        assert_eq!(
            native_alias_target(tool.name()).expect("alias").address,
            call.to
        );
        let disclosure = bind_call(&call, options().from).expect("ledger disclosure");
        assert_eq!(
            disclosure.construct_method().expect("method"),
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
    let payment = Payment::decode(ModuleId::Asset, 6, &payload, b"did:layerx:alice")
        .expect("authenticated receive");
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
    let call: Eip1559Call = prepare_alias_call(
        PaymentTool::DrawGrant,
        &payload,
        b"did:layerx:alice",
        &options(),
    )
    .expect("prepare ledger draw");
    let mut address = [0; 20];
    address[18..].copy_from_slice(&[0x10, 0x1a]);
    assert_eq!(call.to, address);
    assert_eq!(call.chain_id, 125);
    assert_eq!(call.nonce, 7);
    assert_eq!(native_alias_target("grant.draw").expect("alias").ordinal, 6);
    assert_eq!(
        native_alias_target("grant.draw").expect("alias").address,
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
    let disclosure = bind_call(&call, options().from).expect("ledger draw disclosure");
    assert_eq!(
        disclosure.construct_method().expect("method"),
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
    let payment = Payment::decode(ModuleId::Asset, 7, &payload, b"did:layerx:alice")
        .expect("authenticated grant");
    let Payment::IssueGrant(grant) = payment else {
        panic!("grant fixture");
    };
    let call: Eip1559Call = prepare_alias_call(
        PaymentTool::IssueGrant,
        &payload,
        b"did:layerx:alice",
        &options(),
    )
    .expect("prepare ledger grant");
    let mut address = [0; 20];
    address[18..].copy_from_slice(&[0x10, 0x1a]);
    assert_eq!(call.to, address);
    assert_eq!(call.chain_id, 125);
    assert_eq!(call.nonce, 7);
    assert_eq!(
        native_alias_target("grant.issue").expect("alias").ordinal,
        7
    );
    assert_eq!(
        native_alias_target("grant.issue").expect("alias").address,
        address
    );
    assert_eq!(&call.data[..4], &ledger_selector("issueGrant(bytes)"));
    let disclosure = bind_call(&call, options().from).expect("ledger grant disclosure");
    assert_eq!(
        disclosure.construct_method().expect("method"),
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
