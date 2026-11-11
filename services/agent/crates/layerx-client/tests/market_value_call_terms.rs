use layerx_client::ai_market::{market_program, AiClientError};
use layerx_client::{market_value_call_terms, MarketSpendingGrant};
use layerx_programs_runtime::abi::{Capability, CapabilitySet};
use layerx_programs_runtime::access::AccessDeclaration;
use layerx_programs_runtime::accounts::derive_program_account;
use layerx_programs_runtime::storage::ProgramId;

const PROGRAM: [u8; 32] = [1; 32];
const PRINCIPAL: [u8; 32] = [2; 32];
const ASSET: [u8; 32] = [3; 32];
const ACCOUNT: [u8; 32] = [4; 32];
const RECIPIENT: [u8; 32] = [5; 32];
const AMOUNT: u128 = 79;
const SEED: &[u8] = b"paxai/rewards/v1";

fn deposit() -> MarketSpendingGrant {
    MarketSpendingGrant::Deposit {
        asset: ASSET,
        rewards_account: ACCOUNT,
        amount: AMOUNT,
    }
}

fn payment() -> Result<MarketSpendingGrant, String> {
    let program = ProgramId::new(PROGRAM).map_err(|error| error.to_string())?;
    let source_account = derive_program_account(program, SEED)
        .map_err(|error| error.to_string())?
        .bytes();
    Ok(MarketSpendingGrant::Payment {
        owner_program: PROGRAM,
        seed: SEED.to_vec(),
        source_account,
        asset: ASSET,
        recipient: RECIPIENT,
        amount: AMOUNT,
    })
}

fn assert_terms(
    operation: market_program::dispatch::Operation,
    grant: MarketSpendingGrant,
    spending: Capability,
) -> Result<(), String> {
    let terms = market_value_call_terms(operation, PROGRAM, PRINCIPAL, grant)
        .map_err(|error| format!("{error:?}"))?;
    let decoded = if matches!(spending, Capability::ProgramSpend { .. }) {
        CapabilitySet::decode_v2_canonical(&terms.capabilities)
            .map_err(|error| error.to_string())?
    } else {
        CapabilitySet::decode_canonical(&terms.capabilities).map_err(|error| error.to_string())?
    };
    assert_eq!(
        decoded,
        vec![
            Capability::EmitEvent,
            spending,
            Capability::SharedStorageRead,
            Capability::SharedStorageWrite,
        ]
    );
    assert!(!decoded
        .iter()
        .any(|grant| matches!(grant, Capability::BalanceView { .. })));
    assert_eq!(
        CapabilitySet::new(decoded)
            .map_err(|error| error.to_string())?
            .canonical_encoding(),
        terms.capabilities
    );
    assert_eq!(
        AccessDeclaration::canonical_decode(&terms.access_declaration),
        Ok(AccessDeclaration::Absent),
    );
    Ok(())
}

#[test]
fn market_value_deposit_terms_cover_every_deposit_operation() -> Result<(), String> {
    for operation in [
        market_program::dispatch::FUND,
        market_program::dispatch::LockBond,
        market_program::dispatch::ChallengeLap,
    ] {
        assert_terms(
            operation,
            deposit(),
            Capability::Transfer402 {
                asset: ASSET,
                to: ACCOUNT,
                maximum_amount: AMOUNT,
            },
        )?;
    }
    Ok(())
}

#[test]
fn market_value_payment_terms_cover_every_payment_operation() -> Result<(), String> {
    let program = ProgramId::new(PROGRAM).map_err(|error| error.to_string())?;
    let source_account = derive_program_account(program, SEED)
        .map_err(|error| error.to_string())?
        .bytes();
    for operation in [
        market_program::dispatch::CLAIM,
        market_program::dispatch::REFUND_FREE,
        market_program::dispatch::ADVANCE_CLOSE,
        market_program::dispatch::RefundBond,
        market_program::dispatch::ResolveChallenge,
    ] {
        assert_terms(
            operation,
            payment()?,
            Capability::ProgramSpend {
                owner_program: program,
                seed: SEED.to_vec(),
                source_account,
                asset: ASSET,
                to: RECIPIENT,
                maximum_amount: AMOUNT,
            },
        )?;
    }
    Ok(())
}

#[test]
fn market_value_terms_refuse_nonvalue_zero_and_wrong_direction() -> Result<(), String> {
    let nonvalue = market_program::dispatch::RecordDeal;
    assert_eq!(
        market_value_call_terms(nonvalue, PROGRAM, PRINCIPAL, deposit()),
        Err(AiClientError::MarketNonValueOperation {
            operation: nonvalue.metadata().name
        }),
    );
    let zero_deposit = MarketSpendingGrant::Deposit {
        asset: ASSET,
        rewards_account: ACCOUNT,
        amount: 0,
    };
    assert_eq!(
        market_value_call_terms(
            market_program::dispatch::FUND,
            PROGRAM,
            PRINCIPAL,
            zero_deposit
        ),
        Err(AiClientError::MarketValueAmountZero),
    );
    let zero_payment = match payment()? {
        MarketSpendingGrant::Payment {
            owner_program,
            seed,
            source_account,
            asset,
            recipient,
            ..
        } => MarketSpendingGrant::Payment {
            owner_program,
            seed,
            source_account,
            asset,
            recipient,
            amount: 0,
        },
        MarketSpendingGrant::Deposit { .. } => unreachable!(),
    };
    assert_eq!(
        market_value_call_terms(
            market_program::dispatch::CLAIM,
            PROGRAM,
            PRINCIPAL,
            zero_payment
        ),
        Err(AiClientError::MarketValueAmountZero),
    );
    for (operation, grant) in [
        (market_program::dispatch::FUND, payment()?),
        (market_program::dispatch::CLAIM, deposit()),
    ] {
        assert_eq!(
            market_value_call_terms(operation, PROGRAM, PRINCIPAL, grant),
            Err(AiClientError::MarketValueGrantDirection {
                operation: operation.metadata().name
            }),
        );
    }
    Ok(())
}
