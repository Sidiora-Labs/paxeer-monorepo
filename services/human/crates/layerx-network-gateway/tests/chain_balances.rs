use layerx_network_gateway::{
    decode_account, decode_account_balances, AssetBalance, Evidence, GatewayError,
    WalletObservation,
};
use serde_json::{json, Value};

#[path = "support/hosted_gateway.rs"]
mod hosted_gateway;

fn account_document() -> Value {
    json!({
        "evm_address": "0x102132435465768798a9bacbdcedfe0f1e2d3c4b",
        "pax_address": "pax1exampleaccount",
        "layerx_did": format!("did:layerx:{}", "61".repeat(32)),
        "layerx_account": "457d8ff18e80b2c0671cb8dd37042120b6bfcfa2f2477389636a6d62e2e64de0",
        "bound": true
    })
}

fn balance_answer() -> Value {
    json!({
        "account": account_document(),
        "height": 812,
        "balances": [
            {"denom": "upax", "amount": "5000"},
            {"denom": "uusdc", "amount": "7"}
        ]
    })
}

#[test]
fn balances_decode_bank_coins_at_one_height() {
    let mut answer = balance_answer();
    let balances = decode_account_balances(&answer).expect("chain balances decode");
    assert_eq!(balances.height, 812);
    assert_eq!(balances.evidence, Evidence::GatewayReported);
    assert!(balances.account.bound);
    assert_eq!(
        balances.account.evm_address,
        Some([
            0x10, 0x21, 0x32, 0x43, 0x54, 0x65, 0x76, 0x87, 0x98, 0xa9, 0xba, 0xcb, 0xdc, 0xed,
            0xfe, 0x0f, 0x1e, 0x2d, 0x3c, 0x4b,
        ])
    );
    assert_eq!(
        balances.balances,
        vec![
            AssetBalance {
                denom: "upax".to_owned(),
                amount: 5000.into()
            },
            AssetBalance {
                denom: "uusdc".to_owned(),
                amount: 7.into()
            },
        ]
    );
    assert_eq!(balances.asset("upax"), balances.balances.first());
    assert_eq!(balances.asset("uusdc"), balances.balances.get(1));
    assert_eq!(balances.asset("missing"), None);

    answer = hosted_gateway::balances();
    let recorded = decode_account_balances(&answer).expect("recorded uint256 balances decode");
    let bank = hosted_gateway::vector("bank.all_balances");
    assert_eq!(recorded.height, 16);
    assert_eq!(recorded.balances.len(), 2);
    for (row, expected) in recorded
        .balances
        .iter()
        .zip(bank["out"]["response"].as_array().unwrap())
    {
        assert_eq!(row.denom, expected["denom"].as_str().unwrap());
        assert_eq!(row.amount.to_string(), expected["amount"].as_str().unwrap());
    }
    assert_eq!(
        recorded.balances[1].amount.for_operation("native-send"),
        Err(GatewayError::AmountOutOfRange {
            operation: "native-send"
        })
    );

    let observed = WalletObservation::accept(recorded).expect("real production wallet consumer");
    let denom = &hosted_gateway::coins()[1].0;
    assert_eq!(
        observed.balance(denom).unwrap().to_string(),
        hosted_gateway::TWO_TO_200
    );
    assert_eq!(observed.joined().balances.len(), 2);
    assert_eq!(observed.balance("ulxp"), Some(123_456_789.into()));
    assert_eq!(observed.balance("missing"), None);
    assert_eq!(
        observed.joined().account,
        decode_account(&hosted_gateway::account()).unwrap().account
    );

    answer["balances"][1]["amount"] = json!("340282366920938463463374607431768211456");
    let large = decode_account_balances(&answer).expect("valid 2^128 balance decodes");
    assert_eq!(
        large.balances[1].amount.to_string(),
        "340282366920938463463374607431768211456"
    );
    assert_eq!(
        large.balances[1].amount.for_operation("native-send"),
        Err(GatewayError::AmountOutOfRange {
            operation: "native-send"
        })
    );

    answer["height"] = json!(u64::MAX.to_string());
    answer["balances"] = json!([{"denom": "upax", "amount": u128::MAX.to_string()}]);
    let maximum = decode_account_balances(&answer).expect("exact integer bounds decode");
    assert_eq!(maximum.height, u64::MAX);
    assert_eq!(maximum.balances[0].amount, u128::MAX.into());
    answer["balances"] = json!([]);
    let empty = decode_account_balances(&answer).expect("empty bank answer decodes");
    assert_eq!(empty.height, u64::MAX);
    assert!(empty.balances.is_empty());
}

#[test]
fn balances_refuse_row_without_denom() {
    let mut answer = balance_answer();
    answer["balances"][1]
        .as_object_mut()
        .unwrap()
        .remove("denom");
    assert_eq!(
        decode_account_balances(&answer),
        Err(GatewayError::MalformedAnswer)
    );
    for denom in [Value::Null, json!(""), json!(7)] {
        answer["balances"][1]["denom"] = denom;
        assert_eq!(
            decode_account_balances(&answer),
            Err(GatewayError::MalformedAnswer)
        );
    }
    answer = balance_answer();
    answer["balances"][1]
        .as_object_mut()
        .unwrap()
        .remove("amount");
    assert_eq!(
        decode_account_balances(&answer),
        Err(GatewayError::MalformedAnswer)
    );
    for invalid in [
        json!(hosted_gateway::OVERFLOW),
        json!(format!("0x1{}", "0".repeat(64))),
        Value::Null,
        json!(true),
        json!(-1),
        json!(1.5),
        json!("-1"),
        json!("1e2"),
        json!("0xgg"),
    ] {
        answer = balance_answer();
        answer["balances"][1]["amount"] = invalid;
        assert_eq!(
            decode_account_balances(&answer),
            Err(GatewayError::MalformedAnswer)
        );
    }
    for height in [Value::Null, json!("18446744073709551616"), json!(-1)] {
        answer = balance_answer();
        answer["height"] = height;
        assert_eq!(
            decode_account_balances(&answer),
            Err(GatewayError::MalformedAnswer)
        );
    }
}

#[test]
fn account_decodes_paxeer_account_with_height() {
    let mut answer = json!({
        "account": account_document(),
        "paxeer": {
            "address": "0x102132435465768798a9bacbdcedfe0f1e2d3c4b",
            "balance": "9",
            "nonce": "3",
            "height": 812
        }
    });
    let account = decode_account(&answer).expect("chain account decodes");
    assert_eq!(account.evidence, Evidence::GatewayReported);
    let paxeer = account.paxeer.expect("Paxeer chain account");
    assert_eq!(Some(paxeer.address), account.account.evm_address);
    assert_eq!(paxeer.balance, 9.into());
    assert_eq!(paxeer.nonce, 3);
    assert_eq!(paxeer.height, 812);

    answer["paxeer"]["balance"] = json!("0x123456789abcdef");
    answer["paxeer"]["nonce"] = json!("0x2a");
    answer["paxeer"]["height"] = json!(16);
    let recorded = decode_account(&answer).expect("task 8.5.1 account answer decodes");
    let recorded = recorded.paxeer.expect("recorded Paxeer account");
    assert_eq!(recorded.balance, 0x123456789abcdef.into());
    assert_eq!(recorded.nonce, 42);
    assert_eq!(recorded.height, 16);

    for height in [Value::Null, json!("18446744073709551616"), json!(-1)] {
        answer["paxeer"]["height"] = height;
        assert_eq!(decode_account(&answer), Err(GatewayError::MalformedAnswer));
    }
    answer["paxeer"] = Value::Null;
    answer["account"]["evm_address"] = Value::Null;
    answer["account"]["pax_address"] = Value::Null;
    answer["account"]["bound"] = json!(false);
    let unbound = decode_account(&answer).expect("unbound account decodes");
    assert_eq!(unbound.paxeer, None);
    assert_eq!(unbound.account.evm_address, None);
    assert!(!unbound.account.bound);
    assert_eq!(unbound.evidence, Evidence::GatewayReported);
}
