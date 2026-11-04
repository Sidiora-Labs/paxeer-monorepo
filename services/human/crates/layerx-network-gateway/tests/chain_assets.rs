use layerx_network_gateway::{decode_assets, AssetEntry, Evidence, GatewayError};
use serde_json::{json, Value};

fn recorded_chain_assets() -> Value {
    json!({
        "height": 16,
        "assets": [
            {
                "asset_id": format!("0x{}", "11".repeat(32)),
                "denom": null,
                "kind": "layerx",
                "name": "Ledger Asset",
                "symbol": "LED",
                "decimals": 18,
                "supply": "340282366920938463463374607431768211454"
            },
            {
                "asset_id": "paxeer:ulxp",
                "denom": "ulxp",
                "kind": "bank",
                "name": "LayerX",
                "symbol": "LXP",
                "decimals": 18,
                "supply": "1000000"
            }
        ],
        "joined_limit": 16
    })
}

#[test]
fn assets_decode_chain_metadata() {
    let answer = recorded_chain_assets();
    let table = decode_assets(&answer).expect("task 8.5.2 chain metadata decodes");
    assert_eq!(table.height, 16);
    assert_eq!(table.joined_limit, 16);
    assert_eq!(table.evidence, Evidence::GatewayReported);
    assert_eq!(
        table.assets,
        vec![
            AssetEntry {
                asset_id: format!("0x{}", "11".repeat(32)),
                denom: None,
                kind: "layerx".to_owned(),
                name: Some("Ledger Asset".to_owned()),
                symbol: Some("LED".to_owned()),
                decimals: Some(18),
                supply: Some((u128::MAX - 1).into()),
            },
            AssetEntry {
                asset_id: "paxeer:ulxp".to_owned(),
                denom: Some("ulxp".to_owned()),
                kind: "bank".to_owned(),
                name: Some("LayerX".to_owned()),
                symbol: Some("LXP".to_owned()),
                decimals: Some(18),
                supply: Some(1_000_000.into()),
            },
        ]
    );

    let mut boundary = answer.clone();
    boundary["height"] = json!(u64::MAX.to_string());
    boundary["assets"][0]["supply"] = json!(u128::MAX.to_string());
    boundary["assets"][0]["decimals"] = json!(255);
    let table = decode_assets(&boundary).expect("exact integer bounds decode");
    assert_eq!(table.height, u64::MAX);
    assert_eq!(table.assets[0].supply, Some(u128::MAX.into()));
    assert_eq!(table.assets[0].decimals, Some(u8::MAX));

    boundary["assets"][0]["supply"] = json!("340282366920938463463374607431768211456");
    let large = decode_assets(&boundary).expect("uint256 supply read remains available");
    let supply = large.assets[0].supply.expect("observed supply");
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

    let mut bounded = answer.clone();
    bounded["assets"] = Value::Array(vec![answer["assets"][0].clone(); 1_024]);
    assert_eq!(
        decode_assets(&bounded)
            .expect("maximum row count decodes")
            .assets
            .len(),
        1_024
    );
    bounded["assets"]
        .as_array_mut()
        .unwrap()
        .push(answer["assets"][0].clone());
    assert_eq!(decode_assets(&bounded), Err(GatewayError::MalformedAnswer));
    bounded["assets"] = json!([]);
    assert!(decode_assets(&bounded)
        .expect("empty asset page decodes")
        .assets
        .is_empty());
}

#[test]
fn assets_keep_null_metadata_rows() {
    let mut answer = recorded_chain_assets();
    answer["assets"].as_array_mut().unwrap().insert(
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
    answer["assets"].as_array_mut().unwrap().push(json!({
        "asset_id": "denom:ubroken",
        "denom": "ubroken",
        "kind": "token_factory",
        "name": null,
        "symbol": null,
        "decimals": null,
        "supply": null
    }));
    let table = decode_assets(&answer).expect("failed chain reads retain their asset rows");
    assert_eq!(table.height, 16);
    assert_eq!(table.joined_limit, 16);
    assert_eq!(table.evidence, Evidence::GatewayReported);
    assert_eq!(table.assets.len(), 4);
    assert_eq!(table.assets[0].supply, Some((u128::MAX - 1).into()));
    assert_eq!(table.assets[2].supply, Some(1_000_000.into()));
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
    let zero =
        decode_assets(&answer).expect("chain metadata keeps empty names and zero quantities");
    assert_eq!(zero.assets[1].name.as_deref(), Some(""));
    assert_eq!(zero.assets[1].symbol.as_deref(), Some(""));
    assert_eq!(zero.assets[1].decimals, Some(0));
    assert_eq!(zero.assets[1].supply, Some(0.into()));
}
