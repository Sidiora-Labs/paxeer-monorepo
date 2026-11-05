include!("../src/main.rs");

#[cfg(test)]
mod repair_22_31_485 {
    use super::*;
    use layerx_client::px::{
        PxAccountJoin, PxAssetEntry, PxAssetTable, PxNetworkHead, PxPaxeerAccount, PxPaxeerHead,
        PxResolvedIdentities, Uint256,
    };
    use layerx_client::sequencer::ReceiptKeyEntry;
    use serde_json::{json, Value};

    const MAX256: &str =
        "115792089237316195423570985008687907853269984665640564039457584007913129639935";

    fn encoded(response: Response) -> Value {
        assert_eq!(response.status, 200, "{}", response.body);
        assert_eq!(response.retry_after, None);
        let mut body: Value = serde_json::from_str(&response.body).unwrap();
        assert_eq!(body.as_object().unwrap().len(), 3);
        assert_eq!(body["ok"], true);
        let trace = body["trace"].as_str().unwrap();
        assert!(trace.strip_prefix("core-").unwrap().parse::<u64>().unwrap() > 0);
        body.as_object_mut().unwrap().remove("trace");
        body.as_object_mut().unwrap().remove("ok");
        body
    }

    fn account(balance: &str) -> PxAccountJoin {
        PxAccountJoin {
            account: PxResolvedIdentities {
                evm_address: Some([0x12; 20]),
                pax_address: None,
                layerx_did: None,
                layerx_account: None,
                bound: false,
            },
            paxeer: Some(PxPaxeerAccount {
                address: [0x12; 20],
                balance: Uint256::from_decimal(balance).unwrap(),
                nonce: 7,
                height: 42,
            }),
            layerx: Some(serde_json::Map::from_iter([
                ("name".to_owned(), json!("Treasury")),
                ("asset_id".to_owned(), json!("ab".repeat(32))),
                ("frozen".to_owned(), json!(false)),
                ("unpublished".to_owned(), json!(true)),
            ])),
        }
    }

    #[test]
    fn public_account_read_encodes_all_256_balance_bits_and_refuses_mismatches() {
        let id = format!("0x{}", "12".repeat(20));
        for amount in ["0", "340282366920938463463374607431768211456", MAX256] {
            let joined = account(amount);
            let balance = joined.paxeer.as_ref().unwrap().balance.bytes();
            assert_eq!(
                encoded(public_reads::native_account_result(
                    joined.clone(),
                    &id,
                    42,
                    balance,
                    7
                )),
                json!({"result": {
                    "name": "Treasury", "asset_id": "ab".repeat(32), "frozen": false,
                    "account_id": id, "balance": amount, "balance_unit": "wei",
                    "balance_decimals": 18, "height": "42", "next_sequence": "7",
                    "batch_number": "42"
                }})
            );
            for bit in [0, 15, 16, 31] {
                let mut different = balance;
                different[bit] ^= 1;
                let response =
                    public_reads::native_account_result(joined.clone(), &id, 42, different, 7);
                assert_eq!(response.status, 502);
                let body: Value = serde_json::from_str(&response.body).unwrap();
                assert_eq!(body["error"]["code"], "account_snapshot_mismatch");
            }
            let response = public_reads::native_account_result(joined, &id, 42, balance, 8);
            assert_eq!(response.status, 502);
        }
    }

    fn asset() -> PxAssetEntry {
        PxAssetEntry {
            asset_id: "ab".repeat(32),
            kind: "ledger".to_owned(),
            denom: None,
            name: Some("LayerX Points".to_owned()),
            symbol: Some("LXP".to_owned()),
            decimals: Some(6),
            supply: Uint256::from_decimal(MAX256),
            paused: Some(false),
            verification: Some("authenticated_committed_snapshot".to_owned()),
        }
    }

    #[test]
    fn public_asset_reads_encode_flat_metadata_without_changing_the_envelope() {
        let asset = asset();
        let expected = json!({
            "asset_id": "ab".repeat(32), "symbol": "LXP", "name": "LayerX Points",
            "decimals": 6, "total_units": MAX256, "paused": false
        });
        assert_eq!(
            encoded(public_reads::native_asset_result(&asset)),
            json!({"result":{"asset":expected}})
        );
        let absent = PxAssetEntry {
            asset_id: "bank:unavailable".to_owned(),
            kind: "bank".to_owned(),
            denom: Some("unavailable".to_owned()),
            name: None,
            symbol: None,
            decimals: None,
            supply: None,
            paused: None,
            verification: None,
        };
        let table = PxAssetTable {
            assets: vec![asset, absent],
            height: 42,
            joined_limit: 16,
        };
        assert_eq!(
            encoded(public_reads::native_assets_result(&table)),
            json!({"result":{"assets":[expected, {"asset_id":"bank:unavailable"}]}})
        );
        for symbol in ["", "ABCDEFGHIJKLMNOPQ", "é"] {
            let mut invalid = table.clone();
            invalid.assets[0].symbol = Some(symbol.to_owned());
            for response in [
                public_reads::native_asset_result(&invalid.assets[0]),
                public_reads::native_assets_result(&invalid),
            ] {
                assert_eq!(response.status, 502);
                let body: Value = serde_json::from_str(&response.body).unwrap();
                assert_eq!(body["error"]["code"], "invalid_asset_symbol");
            }
        }
    }

    #[test]
    fn public_node_read_encodes_the_current_head_and_optional_capabilities() {
        let key = ReceiptKeyEntry {
            key_id: [1; 32],
            public_key: [2; 32],
            first_height: 1,
            last_height: 43,
        };
        for capabilities in [
            None,
            Some(serde_json::Map::from_iter([(
                "read".to_owned(),
                json!(true),
            )])),
        ] {
            let head = PxNetworkHead {
                network_id: "repair-network".to_owned(),
                paxeer: PxPaxeerHead {
                    chain_id: 125,
                    latest_block: 43,
                },
                layerx: capabilities.clone(),
                latest_committed_height: 42,
            };
            assert_eq!(
                encoded(public_reads::native_node_info_result(head, &key, [3; 32])),
                json!({"result": {
                    "protocol_version": layerx_wire::limits::STATE_COMMITMENT_PROTOCOL_VERSION,
                    "network_id": "repair-network", "chain_head_sequence": "42", "latest_sealed_batch": "43",
                    "latest_finalised_checkpoint": "03".repeat(32), "authorised_sequencer_key": "02".repeat(32),
                    "capabilities": capabilities.unwrap_or_default()
                }})
            );
        }
    }
}
