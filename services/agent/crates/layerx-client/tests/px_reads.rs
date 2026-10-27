use std::io::{BufRead, BufReader, Read, Write};
use std::net::TcpListener;
use std::thread::JoinHandle;
use std::time::Duration;

use layerx_client::px::*;
use layerx_client::sequencer::{SequencerEndpoint, SequencerError};
use serde_json::{json, Map, Value};

trait Must<T> {
    fn must(self) -> T;
}

impl<T, E: std::fmt::Debug> Must<T> for Result<T, E> {
    fn must(self) -> T {
        self.unwrap_or_else(|error| panic!("{error:?}"))
    }
}

fn serve(exchanges: Vec<(&'static str, Value)>) -> (String, JoinHandle<Vec<Value>>) {
    let listener = TcpListener::bind("127.0.0.1:0").must();
    let url = format!("http://{}/", listener.local_addr().must());
    let handle = std::thread::spawn(move || {
        let mut seen = Vec::new();
        for (method, result) in exchanges {
            let (stream, _) = listener.accept().must();
            stream.set_read_timeout(Some(Duration::from_secs(5))).must();
            let mut reader = BufReader::new(stream);
            let mut length = 0_usize;
            loop {
                let mut line = String::new();
                assert_ne!(reader.read_line(&mut line).must(), 0);
                if line == "\r\n" {
                    break;
                }
                if let Some(value) = line.to_ascii_lowercase().strip_prefix("content-length:") {
                    length = value.trim().parse().must();
                }
            }
            let mut body = vec![0_u8; length];
            reader.read_exact(&mut body).must();
            let request: Value = serde_json::from_slice(&body).must();
            assert_eq!(request["jsonrpc"], "2.0");
            assert_eq!(request["method"], method);
            let answer =
                json!({"jsonrpc": "2.0", "id": request["id"], "result": result}).to_string();
            seen.push(request["params"].clone());
            let mut stream = reader.into_inner();
            write!(
                stream,
                "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{answer}",
                answer.len()
            ).must();
            stream.flush().must();
        }
        seen
    });
    (url, handle)
}

fn identities_json() -> Value {
    json!({
        "evm_address": format!("0x{}", "12".repeat(20)),
        "pax_address": "pax1boundaccount",
        "layerx_did": format!("did:layerx:{}", "34".repeat(32)),
        "layerx_account": "56".repeat(32),
        "bound": true
    })
}

fn expected_identities() -> PxResolvedIdentities {
    PxResolvedIdentities {
        evm_address: Some([0x12; 20]),
        pax_address: Some("pax1boundaccount".to_owned()),
        layerx_did: Some(format!("did:layerx:{}", "34".repeat(32))),
        layerx_account: Some([0x56; 32]),
        bound: true,
    }
}

fn custody_json() -> Value {
    json!({
        "asset_id": "ab".repeat(32), "denom": "upax",
        "pointer": format!("0x{}", "cd".repeat(20)),
        "enabled": true, "paused": false,
        "minimum_deposit": "7", "custody_cap": u128::MAX.to_string(),
        "custodied": "0x10000000000000001", "released": "9", "pending": 10
    })
}

fn expected_custody() -> PxCustodyAsset {
    PxCustodyAsset {
        asset_id: [0xab; 32],
        denom: "upax".to_owned(),
        pointer: [0xcd; 20],
        enabled: true,
        paused: false,
        minimum_deposit: 7,
        custody_cap: u128::MAX,
        custodied: 0x1_0000_0000_0000_0001,
        released: 9,
        pending: 10,
    }
}

fn fields(value: Value) -> Map<String, Value> {
    value.as_object().must().clone()
}

impl<T> Must<T> for Option<T> {
    fn must(self) -> T {
        self.unwrap_or_else(|| panic!("missing value"))
    }
}

#[test]
fn account_and_balances_read_px_methods_and_decode_every_field() {
    let account = format!("0x{}", "12".repeat(20));
    let native = json!({
        "account_id": "56".repeat(32), "name": "agent:main", "sequence": "17",
        "balances": [{"amount": "12345678901234567890", "asset_id": "ab".repeat(32)}],
        "metadata": {"nested": [true, null, "retained"]}
    });
    let account_result = json!({
        "account": identities_json(),
        "paxeer": {"address": account, "balance": "0xffffffffffffffffffffffffffffffff", "nonce": "0xffffffffffffffff"},
        "layerx": native
    });
    let row_native = json!({"account_id": "56".repeat(32), "amount": "123", "locked": "4"});
    let balances_result = json!({
        "account": identities_json(), "joined_limit": 16,
        "balances": [
            {"asset_id": "ab".repeat(32), "denom": "upax", "custody": custody_json(),
             "paxeer": {"denom": "upax", "amount": "340282366920938463463374607431768211455"},
             "layerx": row_native},
            {"asset_id": "ef".repeat(32), "denom": null, "custody": null, "paxeer": null, "layerx": null}
        ]
    });
    let mut exchanges = vec![
        ("px_getAccount", account_result.clone()),
        ("px_getBalances", balances_result.clone()),
    ];
    for name in ["account", "paxeer", "layerx"] {
        let mut malformed = account_result.clone();
        malformed.as_object_mut().must().remove(name);
        exchanges.push(("px_getAccount", malformed));
    }
    for name in [
        "evm_address",
        "pax_address",
        "layerx_did",
        "layerx_account",
        "bound",
    ] {
        let mut malformed = account_result.clone();
        malformed["account"].as_object_mut().must().remove(name);
        exchanges.push(("px_getAccount", malformed));
    }
    for (name, invalid) in [
        ("address", json!("0x12")),
        ("balance", json!("340282366920938463463374607431768211456")),
        ("nonce", json!("0x10000000000000000")),
    ] {
        let mut malformed = account_result.clone();
        malformed["paxeer"][name] = invalid;
        exchanges.push(("px_getAccount", malformed));
    }
    let bad_accounts = exchanges.len() - 2;
    for name in ["account", "balances", "joined_limit"] {
        let mut malformed = balances_result.clone();
        malformed.as_object_mut().must().remove(name);
        exchanges.push(("px_getBalances", malformed));
    }
    for name in ["asset_id", "denom", "custody", "paxeer", "layerx"] {
        let mut malformed = balances_result.clone();
        malformed["balances"][0].as_object_mut().must().remove(name);
        exchanges.push(("px_getBalances", malformed));
    }
    for name in fields(custody_json()).keys() {
        let mut malformed = balances_result.clone();
        malformed["balances"][0]["custody"]
            .as_object_mut()
            .must()
            .remove(name);
        exchanges.push(("px_getBalances", malformed));
    }
    for invalid in [json!("-1"), json!("01"), json!(1.5), Value::Null] {
        let mut malformed = balances_result.clone();
        malformed["balances"][0]["paxeer"]["amount"] = invalid;
        exchanges.push(("px_getBalances", malformed));
    }
    let bad_balances = exchanges.len() - 2 - bad_accounts;
    let unbound = json!({"evm_address": null, "pax_address": null, "layerx_did": null,
                         "layerx_account": null, "bound": false});
    exchanges.push((
        "px_getAccount",
        json!({"account": unbound, "paxeer": null, "layerx": null}),
    ));
    exchanges.push((
        "px_getBalances",
        json!({"account": unbound, "balances": [], "joined_limit": "16"}),
    ));

    let (url, handle) = serve(exchanges);
    let endpoint = PxEndpoint::from(SequencerEndpoint::new(&url, &[]).must());
    assert_eq!(
        endpoint.account(&account).must(),
        PxAccountJoin {
            account: expected_identities(),
            paxeer: Some(PxPaxeerAccount {
                address: [0x12; 20],
                balance: u128::MAX,
                nonce: u64::MAX
            }),
            layerx: Some(fields(native)),
        }
    );
    assert_eq!(
        endpoint.balances(&account).must(),
        PxAccountBalances {
            account: expected_identities(),
            joined_limit: 16,
            balances: vec![
                PxAssetBalance {
                    asset_id: [0xab; 32],
                    denom: Some("upax".to_owned()),
                    custody: Some(expected_custody()),
                    paxeer: Some(PxPaxeerAssetBalance {
                        denom: "upax".to_owned(),
                        amount: u128::MAX
                    }),
                    layerx: Some(fields(row_native))
                },
                PxAssetBalance {
                    asset_id: [0xef; 32],
                    denom: None,
                    custody: None,
                    paxeer: None,
                    layerx: None
                }
            ],
        }
    );
    for _ in 0..bad_accounts {
        assert_eq!(endpoint.account(&account), Err(SequencerError::Malformed));
    }
    for _ in 0..bad_balances {
        assert_eq!(endpoint.balances(&account), Err(SequencerError::Malformed));
    }
    let empty_identities = PxResolvedIdentities {
        evm_address: None,
        pax_address: None,
        layerx_did: None,
        layerx_account: None,
        bound: false,
    };
    assert_eq!(
        endpoint.account(&account).must(),
        PxAccountJoin {
            account: empty_identities.clone(),
            paxeer: None,
            layerx: None
        }
    );
    assert_eq!(
        endpoint.balances(&account).must(),
        PxAccountBalances {
            account: empty_identities,
            balances: vec![],
            joined_limit: 16
        }
    );
    let seen = handle.join().must();
    assert_eq!(seen.len(), 4 + bad_accounts + bad_balances);
    for params in seen {
        assert_eq!(params, json!([account]));
    }
}

#[test]
fn list_assets_and_network_read_px_methods_and_refuse_anchor_fields() {
    let metadata = json!({"asset_id": "ab".repeat(32), "name": "Paxeer", "symbol": "PAX",
                          "decimals": 18, "supply": "123456789", "metadata": {"tags": ["native"]}});
    let assets = json!({"assets": [
        {"asset_id": "ab".repeat(32), "layerx": metadata, "paxeer": custody_json()},
        {"asset_id": "ef".repeat(32), "layerx": null, "paxeer": null}
    ], "joined_limit": "0x10"});
    let native_head = json!({"node_info": {"version": "1", "height": "9"}});
    let network = json!({"network_id": "paxeer:mainnet", "paxeer": {"chain_id": "0x7d", "latest_block": "0x10"},
                         "layerx": native_head, "latest_committed_height": 9});
    let mut exchanges = vec![
        ("px_listAssets", assets.clone()),
        ("px_getNetwork", network.clone()),
    ];
    for anchor in [
        Value::Null,
        json!({"latest_finalized_batch": 9}),
        json!(false),
    ] {
        let mut malformed = network.clone();
        malformed["anchor"] = anchor;
        exchanges.push(("px_getNetwork", malformed));
    }
    for name in ["network_id", "paxeer", "layerx", "latest_committed_height"] {
        let mut malformed = network.clone();
        malformed.as_object_mut().must().remove(name);
        exchanges.push(("px_getNetwork", malformed));
    }
    for (path, invalid) in [
        ("/latest_committed_height", json!(17)),
        ("/paxeer/chain_id", json!("18446744073709551616")),
        ("/paxeer/latest_block", json!("0x")),
        ("/layerx", json!([])),
    ] {
        let mut malformed = network.clone();
        *malformed.pointer_mut(path).must() = invalid;
        exchanges.push(("px_getNetwork", malformed));
    }
    let bad_networks = exchanges.len() - 2;
    for name in ["assets", "joined_limit"] {
        let mut malformed = assets.clone();
        malformed.as_object_mut().must().remove(name);
        exchanges.push(("px_listAssets", malformed));
    }
    for name in ["asset_id", "layerx", "paxeer"] {
        let mut malformed = assets.clone();
        malformed["assets"][0].as_object_mut().must().remove(name);
        exchanges.push(("px_listAssets", malformed));
    }
    let mut malformed = assets.clone();
    malformed["assets"][0]["paxeer"]["enabled"] = json!("true");
    exchanges.push(("px_listAssets", malformed));
    let mut malformed = assets.clone();
    malformed["assets"][0]["layerx"] = json!([]);
    exchanges.push(("px_listAssets", malformed));
    let mut malformed = assets;
    malformed["assets"] = json!(vec![Value::Null; 1025]);
    exchanges.push(("px_listAssets", malformed));
    let bad_assets = exchanges.len() - 2 - bad_networks;
    let mut no_native = network;
    no_native["layerx"] = Value::Null;
    exchanges.push(("px_getNetwork", no_native));
    exchanges.push(("px_listAssets", json!({"assets": [], "joined_limit": 16})));

    let (url, handle) = serve(exchanges);
    let endpoint = PxEndpoint::new(&url, &[]).must();
    assert_eq!(
        endpoint.list_assets().must(),
        PxAssetTable {
            joined_limit: 16,
            assets: vec![
                PxAssetEntry {
                    asset_id: [0xab; 32],
                    layerx: Some(fields(metadata)),
                    paxeer: Some(expected_custody())
                },
                PxAssetEntry {
                    asset_id: [0xef; 32],
                    layerx: None,
                    paxeer: None
                },
            ]
        }
    );
    let expected_head = PxNetworkHead {
        network_id: "paxeer:mainnet".to_owned(),
        paxeer: PxPaxeerHead {
            chain_id: 125,
            latest_block: 16,
        },
        layerx: Some(fields(native_head)),
        latest_committed_height: 9,
    };
    assert_eq!(endpoint.network().must(), expected_head);
    for _ in 0..bad_networks {
        assert_eq!(endpoint.network(), Err(SequencerError::Malformed));
    }
    for _ in 0..bad_assets {
        assert_eq!(endpoint.list_assets(), Err(SequencerError::Malformed));
    }
    assert_eq!(
        endpoint.network().must(),
        PxNetworkHead {
            layerx: None,
            ..expected_head
        }
    );
    assert_eq!(
        endpoint.list_assets().must(),
        PxAssetTable {
            assets: vec![],
            joined_limit: 16
        }
    );
    let seen = handle.join().must();
    assert_eq!(seen.len(), 4 + bad_networks + bad_assets);
    for params in seen {
        assert_eq!(params, json!([]));
    }
}
