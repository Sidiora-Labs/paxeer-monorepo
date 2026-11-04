use layerx_human_service::journeys::{
    Endpoint, FeeSchedule, NetworkObservation, ObservationError, ObservedStateBuilder,
};
use layerx_network_gateway::{
    AccountBalances, AssetBalance, Evidence, LookupSelector, Quantity, ResolvedIdentities,
};
use layerx_types::account::AccountId;
use layerx_types::amount::Amount;
use layerx_types::ids::AssetId;
use layerx_types::intent::{EvmAddress, TimestampSeconds};
use sha2::{Digest as _, Sha256};

#[test]
fn observed_balances_use_denom_amount_and_height() -> Result<(), Box<dyn std::error::Error>> {
    let native = AssetId::new(Sha256::digest(b"upax").into());
    let other = AssetId::new(Sha256::digest(b"uother").into());
    let wallet = [7; 20];
    let zero = Quantity::from_be_bytes([0; 32]);
    let zero_amount = Amount::from_u128(zero.for_operation("intent-planning")?);
    for (height, amount) in [(42, 4_096), (43, u128::MAX)] {
        let mut word = [0; 32];
        word[16..].copy_from_slice(&amount.to_be_bytes());
        let quantity = Quantity::from_be_bytes(word);
        let expected_amount = Amount::from_u128(quantity.for_operation("intent-planning")?);
        assert_eq!(expected_amount, Amount::from_u128(amount));
        assert_eq!(zero_amount, Amount::from_u128(0));
        let answer = AccountBalances {
            account: ResolvedIdentities {
                lookup_selector: Some(LookupSelector::Evm(wallet)),
                evm_address: Some(wallet),
                pax_address: None,
                layerx_did: None,
                layerx_account: None,
                bound: true,
                evidence: Evidence::GatewayReported,
            },
            height,
            balances: vec![
                AssetBalance {
                    denom: "uother".to_owned(),
                    amount: zero,
                },
                AssetBalance {
                    denom: "upax".to_owned(),
                    amount: quantity,
                },
            ],
            evidence: Evidence::GatewayReported,
        };
        let network = NetworkObservation::accept(answer.clone())?;
        assert_eq!(network.wallet(), EvmAddress::new(wallet));
        assert_eq!(network.height(), height);
        assert_eq!(network.joined(), &answer);
        assert_eq!(network.balance("upax"), Some(quantity));
        assert_eq!(network.balance("uother"), Some(zero));
        assert_eq!(network.balance("unobserved"), None);
        assert_eq!(
            network.joined().asset("upax").map(|row| row.amount),
            Some(quantity)
        );
        assert_eq!(
            network.joined().asset("uother").map(|row| row.amount),
            Some(zero)
        );
        assert_eq!(network.wallet_balance(native)?, expected_amount);
        assert_eq!(network.wallet_balance(other)?, zero_amount);
        assert_eq!(
            network.wallet_balance(AssetId::new([6; 32])),
            Err(ObservationError::AssetNotJoined {
                asset: AssetId::new([6; 32])
            })
        );

        let observed = ObservedStateBuilder::new(
            TimestampSeconds::from_u64(1_000),
            AccountId::parse("agent:did:layerx:alice:main")
                .unwrap_or_else(|error| panic!("owner: {error:?}")),
            AccountId::parse("system:treasury")
                .unwrap_or_else(|error| panic!("reserve: {error:?}")),
            AccountId::parse("system:paxeer-withdrawals")
                .unwrap_or_else(|error| panic!("withdrawals: {error:?}")),
        )
        .with_network(network)
        .with_ledger_balances(Vec::new())
        .with_allowances(Vec::new())
        .with_budget_bindings(Vec::new())
        .with_fees(FeeSchedule::new(Vec::new())?)
        .requiring(native)
        .requiring(other)
        .build()?;
        assert_eq!(
            observed.available(&Endpoint::PaxeerWallet, native),
            expected_amount
        );
        assert_eq!(
            observed.available(&Endpoint::PaxeerWallet, other),
            zero_amount
        );

        let mut missing = answer;
        missing.balances.retain(|row| row.denom != "upax");
        let missing = NetworkObservation::accept(missing)?;
        assert_eq!(missing.height(), height);
        assert_eq!(
            missing.wallet_balance(native),
            Err(ObservationError::AssetNotJoined { asset: native })
        );
    }
    Ok(())
}
