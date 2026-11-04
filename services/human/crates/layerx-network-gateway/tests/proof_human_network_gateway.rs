//! Proves: decode_account / decode_account_balances / decode_assets / decode_network -> AccountJoin / AccountBalances / AssetBalance / AssetTable / AssetEntry / NetworkHead -> WalletObservation (the production core of journeys::observed::NetworkObservation).
//!
//! The crossing under contract is the public Rust result, not a separate
//! daemon. Parser inputs are built in this process; only actual decoder outputs
//! cross into the consumer. No transport or consumer is replaced, and no
//! externally provisioned producer, configuration or fixture is needed.
//!
//! Repairs services/human/crates/layerx-network-gateway/src/lib.rs:
//! decode_account_balances accepted duplicate denominations, allowing
//! NetworkObservation::balance in
//! services/human/crates/layerx-human-service/src/journeys/observed.rs to silently
//! select the first conflicting amount. Both decoded and directly supplied
//! AccountBalances now refuse that ambiguity. NetworkObservation delegates its
//! accept/balance/joined operations to the same WalletObservation tested
//! here; its service-specific address and amount wrappers remain in observed.rs.

use layerx_network_gateway::{
    decode_account, decode_account_balances, decode_assets, decode_network, AccountBalances,
    AccountIdentifier, AccountJoin, AssetBalance, AssetEntry, AssetTable, Evidence, GatewayError,
    NetworkHead, WalletObservation,
};
use serde_json::{json, Value};

/// Input state for the real public decoders. These are inputs to producers,
/// never substitutes for their typed results at the consumer boundary.
struct ProducerState {
    account: Value,
    balances: Value,
    assets: Value,
    network: Value,
}

impl ProducerState {
    fn new(height: u64, amount: u128) -> Self {
        let identities = json!({
            "evm_address": format!("0x{}", "27".repeat(20)),
            "pax_address": "pax1gatewayproof",
            "layerx_did": format!("did:layerx:{}", "31".repeat(32)),
            "layerx_account": "42".repeat(32),
            "bound": true
        });
        Self {
            account: json!({
                "account": identities.clone(),
                "paxeer": {
                    "address": identities["evm_address"],
                    "balance": amount.to_string(),
                    "nonce": "3",
                    "height": height
                }
            }),
            balances: json!({
                "account": identities,
                "height": height,
                "balances": [
                    {"denom": "upax", "amount": amount.to_string()},
                    {"denom": "uusdc", "amount": "0"}
                ]
            }),
            assets: json!({
                "height": height,
                "joined_limit": 16,
                "assets": [{
                    "asset_id": "paxeer:upax", "denom": "upax", "kind": "bank",
                    "name": "Paxeer", "symbol": "PAX", "decimals": 18,
                    "supply": u128::MAX.to_string()
                }, {
                    "asset_id": "paxeer:uusdc", "denom": "uusdc", "kind": "bank",
                    "name": null, "symbol": null, "decimals": null, "supply": null
                }]
            }),
            network: json!({
                "network_id": "gateway-proof",
                "paxeer": {"chain_id": "0x7d", "latest_block": height},
                "consensus": {
                    "latest_height": height.to_string(),
                    "latest_block_hash": "a1".repeat(32),
                    "latest_app_hash": "b2".repeat(32),
                    "catching_up": false
                }
            }),
        }
    }

    fn observe(&self) -> WalletObservation {
        let account: AccountJoin = decode_account(&self.account)
            .expect("src/lib.rs::decode_account must produce the account join");
        let balances: AccountBalances = decode_account_balances(&self.balances)
            .expect("src/lib.rs::decode_account_balances must produce bank balances");
        let assets: AssetTable = decode_assets(&self.assets)
            .expect("src/lib.rs::decode_assets must produce the asset table");
        let network: NetworkHead = decode_network(&self.network)
            .expect("src/lib.rs::decode_network must produce the network head");

        assert_eq!(account.account, balances.account);
        assert_eq!(account.evidence, Evidence::GatewayReported);
        assert_eq!(balances.evidence, Evidence::GatewayReported);
        assert_eq!(assets.evidence, Evidence::GatewayReported);
        assert_eq!(network.evidence, Evidence::GatewayReported);
        let paxeer = account
            .paxeer
            .expect("bound account must report its Paxeer half");
        assert_eq!(Some(paxeer.address), balances.account.evm_address);
        assert_eq!(paxeer.height, balances.height);
        assert_eq!(paxeer.nonce, 3);
        assert_eq!(assets.height, balances.height);
        assert_eq!(assets.joined_limit, 16);
        assert_eq!(network.network_id, "gateway-proof");
        assert_eq!(network.chain_id, 125);
        assert_eq!(network.latest_block, balances.height);
        let consensus = network.consensus.expect("producer supplied consensus");
        assert_eq!(consensus.latest_height, balances.height);
        assert_eq!(consensus.latest_block_hash, [0xa1; 32]);
        assert_eq!(consensus.latest_app_hash, [0xb2; 32]);
        assert!(!consensus.catching_up);

        let expected = balances.clone();
        // This is the production acceptance function called by the human
        // service, operating on the producer's actual public Rust result.
        let observed = WalletObservation::accept(balances)
            .expect("human consumer must accept the produced bound balance table");
        assert_eq!(observed.joined(), &expected);
        assert_eq!(observed.wallet(), paxeer.address);
        assert_eq!(observed.balance("upax"), Some(paxeer.balance));
        assert_eq!(observed.balance("uusdc"), Some(0.into()));
        assert_eq!(observed.balance("unobserved"), None);
        assert_eq!(assets.assets.len(), 2);
        for entry in &assets.assets {
            let entry: &AssetEntry = entry;
            let denom = entry.denom.as_deref().expect("bank asset denomination");
            let row: &AssetBalance = expected.asset(denom).expect("produced bank row");
            assert_eq!(observed.balance(denom), Some(row.amount));
            assert_eq!(entry.kind, "bank");
        }
        assert_eq!(assets.assets[0].supply, Some(u128::MAX.into()));
        assert_eq!(assets.assets[0].name.as_deref(), Some("Paxeer"));
        assert_eq!(assets.assets[0].symbol.as_deref(), Some("PAX"));
        assert_eq!(assets.assets[0].decimals, Some(18));
        assert_eq!(assets.assets[1].supply, None);
        assert_eq!(assets.assets[1].name, None);
        observed
    }
}

#[test]
fn proof_human_network_gateway_observes_fresh_producer_results() {
    let producer = ProducerState::new(812, u128::MAX - 1);
    let first = producer.observe();
    let second = producer.observe();
    assert_eq!(first, second);
    // Fresh producer state must reach a fresh observation, while the earlier
    // snapshot remains intact. Neither producer nor consumer has a cache.
    let next = ProducerState::new(813, 4_096).observe();
    assert_eq!(next.joined().height, 813);
    assert_eq!(next.balance("upax"), Some(4_096.into()));
    assert_eq!(first.joined().height, 812);
    assert_eq!(first.balance("upax"), Some((u128::MAX - 1).into()));
    assert_ne!(first, next);
}

#[test]
fn proof_human_network_gateway_refuses_malformed_and_ambiguous_balances() {
    let producer = ProducerState::new(812, 4_096);
    let mut malformed = producer.balances.clone();
    malformed["balances"][0]
        .as_object_mut()
        .unwrap()
        .remove("amount");
    assert_eq!(
        decode_account_balances(&malformed).and_then(WalletObservation::accept),
        Err(GatewayError::MalformedAnswer)
    );

    let mut duplicate = producer.balances.clone();
    duplicate["balances"][1]["denom"] = json!("upax");
    assert_eq!(
        decode_account_balances(&duplicate).and_then(WalletObservation::accept),
        Err(GatewayError::MalformedAnswer)
    );
    // Public types also permit a caller to supply rows directly, as the
    // human service's existing accept callers do. Guard that boundary too.
    let mut direct = decode_account_balances(&producer.balances).unwrap();
    direct.balances.push(AssetBalance {
        denom: "upax".to_owned(),
        amount: 1.into(),
    });
    assert_eq!(
        WalletObservation::accept(direct),
        Err(GatewayError::MalformedAnswer)
    );
}

#[test]
fn proof_human_network_gateway_refuses_unbound_wallet_and_wrong_identity() {
    let producer = ProducerState::new(812, 4_096);
    let mut unbound = producer.balances.clone();
    unbound["account"]["bound"] = json!(false);
    unbound["account"]["evm_address"] = Value::Null;
    unbound["account"]["layerx_did"] = Value::Null;
    unbound["account"]["layerx_account"] = Value::Null;
    assert_eq!(
        decode_account_balances(&unbound).and_then(WalletObservation::accept),
        Err(GatewayError::Unbound)
    );
    let mut identities = decode_account(&producer.account).unwrap().account;
    assert_eq!(
        identities.retain_lookup(AccountIdentifier::Evm([0x28; 20])),
        Err(GatewayError::Unbound)
    );
    assert!(identities
        .retain_lookup(AccountIdentifier::Evm([0x27; 20]))
        .is_ok());

    let mut malformed_account = producer.account.clone();
    malformed_account["paxeer"]["nonce"] = json!("invalid");
    assert_eq!(
        decode_account(&malformed_account),
        Err(GatewayError::MalformedAnswer)
    );
    let mut malformed_network = producer.network.clone();
    malformed_network["consensus"]["latest_app_hash"] = json!("invalid");
    assert_eq!(
        decode_network(&malformed_network),
        Err(GatewayError::MalformedAnswer)
    );
    let mut malformed_assets = producer.assets.clone();
    malformed_assets["assets"][0]["supply"] = json!("340282366920938463463374607431768211456");
    let wide = decode_assets(&malformed_assets).expect("uint256 supply reaches the consumer");
    let supply = wide.assets[0].supply.expect("observed supply");
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
    malformed_assets["assets"][0]["supply"] =
        json!("115792089237316195423570985008687907853269984665640564039457584007913129639936");
    assert_eq!(
        decode_assets(&malformed_assets),
        Err(GatewayError::MalformedAnswer)
    );
}
