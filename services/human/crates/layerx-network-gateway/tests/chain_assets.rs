include!("support/required.rs");

use layerx_network_gateway::{decode_assets, AssetEntry, Evidence, GatewayError};
use serde_json::{json, Value};

#[allow(dead_code)]
mod hosted_gateway {
    include!("support/hosted_gateway.rs");
}

fn recorded_chain_assets() -> Value {
    hosted_gateway::assets()
}

fn assert_malformed_asset_fields(answer: &Value) {
    for (field, invalid) in [
        ("asset_id", Value::Null),
        ("asset_id", json!("")),
        ("denom", json!(7)),
        ("kind", Value::Null),
        ("kind", json!("")),
        ("name", json!(false)),
        ("symbol", json!({})),
        ("decimals", json!(256)),
        ("decimals", json!(-1)),
        ("decimals", json!(1.5)),
        (
            "supply",
            json!("115792089237316195423570985008687907853269984665640564039457584007913129639936"),
        ),
        ("supply", json!("-1")),
        ("supply", json!("invalid")),
        ("supply", json!(true)),
        ("supply", json!([])),
        ("supply", json!(1.5)),
    ] {
        let mut invalid_answer = answer.clone();
        invalid_answer["assets"][0][field] = invalid;
        assert_eq!(
            decode_assets(&invalid_answer),
            Err(GatewayError::MalformedAnswer),
            "malformed {field} must be refused"
        );
    }
    for field in ["height", "joined_limit"] {
        for invalid in [Value::Null, json!(-1), json!("18446744073709551616")] {
            let mut invalid_answer = answer.clone();
            invalid_answer[field] = invalid;
            assert_eq!(
                decode_assets(&invalid_answer),
                Err(GatewayError::MalformedAnswer)
            );
        }
    }
}

#[test]
fn assets_decode_chain_metadata() {
    let answer = recorded_chain_assets();
    let table = decode_assets(&answer).required_with("task 8.5.2 chain metadata decodes");
    assert_eq!(table.height, 16);
    assert_eq!(table.joined_limit, 16);
    assert_eq!(table.evidence, Evidence::GatewayReported);
    assert_eq!(
        table.assets,
        vec![
            AssetEntry {
                asset_id: "paxeer:ulxp".to_owned(),
                denom: Some("ulxp".to_owned()),
                kind: "bank".to_owned(),
                name: Some("LayerX".to_owned()),
                symbol: Some("LXP".to_owned()),
                decimals: Some(18),
                supply: Some(layerx_network_gateway::Quantity::from_be_bytes([0xff; 32])),
            },
            AssetEntry {
                asset_id: format!("denom:{}", hosted_gateway::coins()[1].0),
                denom: Some(hosted_gateway::coins()[1].0.clone()),
                kind: "bank".to_owned(),
                name: None,
                symbol: None,
                decimals: None,
                supply: None,
            },
        ]
    );

    let mut boundary = answer.clone();
    boundary["height"] = json!(u64::MAX.to_string());
    boundary["assets"][0]["supply"] = json!(u128::MAX.to_string());
    boundary["assets"][0]["decimals"] = json!(255);
    let table = decode_assets(&boundary).required_with("exact integer bounds decode");
    assert_eq!(table.height, u64::MAX);
    assert_eq!(table.assets[0].supply, Some(u128::MAX.into()));
    assert_eq!(table.assets[0].decimals, Some(u8::MAX));

    boundary["assets"][0]["supply"] = json!("340282366920938463463374607431768211456");
    let large = decode_assets(&boundary).required_with("uint256 supply read remains available");
    let supply = large.assets[0].supply.required_with("observed supply");
    assert_eq!(
        supply.to_string(),
        "340282366920938463463374607431768211456"
    );
    assert_eq!(
        supply.for_operation("asset-issuance"),
        Err(GatewayError::AmountOutOfRange {
            operation: "asset-issuance"
        })
    );

    assert_malformed_asset_fields(&answer);

    let mut bounded = hosted_gateway::joined_page();
    assert_eq!(
        decode_assets(&bounded)
            .required_with("hosted producer joins sixteen rows")
            .assets
            .len(),
        16
    );
    assert_eq!(decode_assets(&bounded).required().joined_limit, 16);
    // Preserve the client's independent absolute safety-bound refusal.
    bounded["assets"] = Value::Array(vec![answer["assets"][0].clone(); 1_025]);
    assert_eq!(decode_assets(&bounded), Err(GatewayError::MalformedAnswer));
    bounded["assets"] = json!([]);
    assert!(decode_assets(&bounded)
        .required_with("empty asset page decodes")
        .assets
        .is_empty());
}

#[test]
fn assets_keep_null_metadata_rows() {
    let mut answer = recorded_chain_assets();
    answer["assets"].as_array_mut().required().insert(
        1,
        json!({
            "asset_id": format!("0x{}", "22".repeat(32)),
            "denom": null,
            "kind": "layerx",
            "name": null,
            "symbol": null,
            "decimals": null,
            "supply": null
        }),
    );
    answer["assets"].as_array_mut().required().push(json!({
        "asset_id": "denom:ubroken",
        "denom": "ubroken",
        "kind": "token_factory",
        "name": null,
        "symbol": null,
        "decimals": null,
        "supply": null
    }));
    let table = decode_assets(&answer).required_with("failed chain reads retain their asset rows");
    assert_eq!(table.height, 16);
    assert_eq!(table.joined_limit, 16);
    assert_eq!(table.evidence, Evidence::GatewayReported);
    assert_eq!(table.assets.len(), 4);
    assert_eq!(
        table.assets[0].supply.required().to_string(),
        hosted_gateway::MAXIMUM
    );
    assert_eq!(table.assets[2].supply, None);
    assert_eq!(
        table.assets[1],
        AssetEntry {
            asset_id: format!("0x{}", "22".repeat(32)),
            denom: None,
            kind: "layerx".to_owned(),
            name: None,
            symbol: None,
            decimals: None,
            supply: None,
        }
    );
    assert_eq!(
        table.assets[3],
        AssetEntry {
            asset_id: "denom:ubroken".to_owned(),
            denom: Some("ubroken".to_owned()),
            kind: "token_factory".to_owned(),
            name: None,
            symbol: None,
            decimals: None,
            supply: None,
        }
    );

    answer["assets"][1]["name"] = json!("");
    answer["assets"][1]["symbol"] = json!("");
    answer["assets"][1]["decimals"] = json!(0);
    answer["assets"][1]["supply"] = json!("0");
    let zero = decode_assets(&answer)
        .required_with("chain metadata keeps empty names and zero quantities");
    assert_eq!(zero.assets[1].name.as_deref(), Some(""));
    assert_eq!(zero.assets[1].symbol.as_deref(), Some(""));
    assert_eq!(zero.assets[1].decimals, Some(0));
    assert_eq!(zero.assets[1].supply, Some(0.into()));
}
