use std::time::Duration;

use layerx_agentd::prepare::evm::PrecompileTarget;
use layerx_crypto::evm_transaction::{
    decode_signed, recover_sender, transaction_hash, Eip1559Call,
};
use layerx_mcp::binding::Binding;
use layerx_mcp::catalogue::WEB_TOOLS;
use layerx_mcp::tools::web::{GrantTerms, WebCall, WebConfig};
use layerx_x402::model::PaymentRequirements;
use serde_json::{json, Value};

fn config() -> WebConfig {
    WebConfig::new(
        "http://127.0.0.1:9450",
        "did:layerx:payer",
        "0xabababababababababababababababababababab",
        [0xcd; 20],
        "eip155:125",
        [0x5e; 32],
        [0x2a; 32],
        1,
        100,
        Duration::from_secs(5),
        3,
    )
    .expect("valid web configuration")
}

#[test]
fn web_fetch_pays_with_pay_exact() {
    let call = WebCall::from_arguments(
        "web.fetch",
        &json!({
            "url": "https://paxeer.app/data.json", "currency": "USDC", "scheme": "exact",
            "idempotency_key": "09".repeat(32),
        }),
    )
    .expect("web.fetch invocation");
    let mut quote: PaymentRequirements = serde_json::from_value(json!({
        "scheme": "exact", "network": "eip155:125", "amount": "2",
        "asset": format!("0x{}", "ef".repeat(20)), "payTo": format!("0x{}", "cd".repeat(20)),
        "maxTimeoutSeconds": 30,
        "extra": {"layerx": {"currency": "USDC", "receiverDid": "did:layerx:receiver"}},
    }))
    .expect("exact price quote");
    let terms = call
        .exact_terms(&config(), &quote, 7)
        .expect("bind exact web price");
    let prepared = terms.prepare_pay_exact();
    let data = prepared.encode().expect("payExact ABI encoding");
    assert_eq!(prepared.target, PrecompileTarget::Pay);
    assert_eq!(
        prepared.target.address(),
        [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x10, 0x1b]
    );
    assert_eq!(
        &data[..4],
        &transaction_hash(b"payExact(bytes32,bytes32,bytes32,uint128,uint64,bytes32)")[..4]
    );
    assert_eq!(data.len(), 4 + 6 * 32);
    for (index, address) in [(0, [0xab; 20]), (1, [0xcd; 20]), (2, [0xef; 20])] {
        assert_eq!(&data[4 + index * 32..4 + index * 32 + 12], &[0; 12]);
        assert_eq!(&data[4 + index * 32 + 12..4 + (index + 1) * 32], &address);
    }
    assert_eq!(&data[100..116], &[0; 16]);
    assert_eq!(&data[116..132], &2_u128.to_be_bytes());
    assert_eq!(&data[132..156], &[0; 24]);
    assert_eq!(&data[156..164], &7_u64.to_be_bytes());
    assert_eq!(&data[164..196], &[9; 32]);
    assert_eq!(terms.amount, 2);
    assert_eq!(terms.recipient, [0xcd; 20]);
    let evm = Eip1559Call {
        chain_id: 125,
        nonce: 7,
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 3,
        gas_limit: 100_000,
        to: prepared.target.address(),
        data: data.clone(),
    };
    let signed = evm.sign(&[0xa5; 32]).expect("sign real EIP-1559 call");
    let (decoded, signature) = decode_signed(&signed).expect("decode signed call");
    assert_eq!(decoded, evm);
    assert_ne!(
        recover_sender(&decoded, &signature).expect("recover EVM signer"),
        [0; 20]
    );
    quote.pay_to = format!("0x{}", "aa".repeat(20));
    assert!(
        call.exact_terms(&config(), &quote, 7).is_err(),
        "recipient substitution must fail"
    );
    quote.pay_to = format!("0x{}", "cd".repeat(32));
    assert!(
        call.exact_terms(&config(), &quote, 7).is_err(),
        "kernel account IDs cannot be truncated into addresses"
    );

    let grant = GrantTerms {
        payer_did: terms.payer_did,
        payer_account: terms.payer_account,
        recipient: terms.recipient,
        asset: terms.asset,
        current_height: 7,
        amount: 2,
        purpose_hash: [4; 32],
        idempotency_key: [9; 32],
    };
    let budget = grant.prepare_budget();
    assert_eq!(budget.target, PrecompileTarget::Pay);
    assert_eq!(budget.encode().expect("budget calldata").len(), 4 + 12 * 32);
    let draw = grant.prepare_draw([8; 32]);
    assert_eq!(draw.target, PrecompileTarget::Pay);
    let draw_data = draw.encode().expect("draw calldata");
    assert_eq!(
        &draw_data[..4],
        &transaction_hash(b"draw(bytes32,bytes32,uint128,bytes32,uint64,bytes32,bytes32)")[..4]
    );
    assert_eq!(&draw_data[4..36], &[8; 32]);
    assert_eq!(&draw_data[52..68], &[0xcd; 16]);
    assert_eq!(&draw_data[84..100], &2_u128.to_be_bytes());
    for tool in WEB_TOOLS {
        assert_eq!(
            tool.required_scope,
            format!("write:{}", tool.name.replace('.', ":"))
        );
        assert!(tool.mutation.contains("payExact"));
    }
}

fn document() -> Value {
    json!({
        "mode": "full", "tenant": "tenant-a", "store": "/tmp/web-store", "audit_root": "/tmp/web-audit",
        "session_id": "07".repeat(32), "session_token_file": "/tmp/web-session-token", "session_generation": 1,
        "capability_id": "09".repeat(32), "core_sequence": 7, "deadline_ms": 5000,
        "agent": {"endpoint": "http://127.0.0.1:9500", "bearer_file": "/tmp/web-agent-bearer", "probe_program": "55".repeat(32)},
        "limit": {"id": "09".repeat(16), "name": "mcp-limit", "scope": "tenant", "scope_id": "01".repeat(32), "ceiling": "1000", "consumed": "0"},
        "web": {"endpoint": "http://127.0.0.1:9450", "payer_address": format!("0x{}", "ab".repeat(20)),
            "receiver_address": format!("0x{}", "cd".repeat(20)), "network": "eip155:125",
            "receipt_key_index": [{"keyId": "2a".repeat(32), "publicKey": "5e".repeat(32), "firstHeight": 1, "lastHeight": 100}],
            "sequencer_id": "2a".repeat(32), "first_batch_number": 1, "last_batch_number": 100,
            "timeout_ms": 5000, "pending_attempts": 3, "approval_threshold": "1000"}
    })
}

#[test]
fn web_binding_names_a_receipt_key_index() {
    let valid = document();
    assert!(
        Binding::parse(&valid.to_string()).is_ok(),
        "receipt key index binds"
    );
    let mut missing = valid.clone();
    missing["web"]
        .as_object_mut()
        .expect("web block")
        .remove("receipt_key_index");
    let error = Binding::parse(&missing.to_string()).expect_err("index is required");
    assert!(error.detail().contains("receipt_key_index"));
    let mut retired = missing.clone();
    retired["web"]["sequencer_public_key"] = json!("5e".repeat(32));
    let error = Binding::parse(&retired.to_string()).expect_err("retired key is refused");
    assert!(error.detail().contains("sequencer_public_key"));
    retired["web"]["receipt_key_index"] = valid["web"]["receipt_key_index"].clone();
    assert!(
        Binding::parse(&retired.to_string()).is_err(),
        "retired key is refused alongside an index"
    );
    for index in [
        json!([]),
        json!("5e".repeat(32)),
        json!([{"keyId": "00".repeat(32), "publicKey": "5e".repeat(32), "firstHeight": 1, "lastHeight": 100}]),
    ] {
        let mut invalid = valid.clone();
        invalid["web"]["receipt_key_index"] = index;
        assert!(
            Binding::parse(&invalid.to_string()).is_err(),
            "malformed key index refused"
        );
    }
    let mut uncovered = valid;
    uncovered["web"]["receipt_key_index"][0]["lastHeight"] = json!(6);
    assert!(
        Binding::parse(&uncovered.to_string()).is_err(),
        "key must cover the configured heights"
    );
}
