use std::error::Error;
use std::future::Future;
use std::pin::pin;
use std::task::{Context, Poll, Waker};

use layerx_crypto::account_derivation::derive_from_seed;
use layerx_crypto::disclosure::call::{bind_call, ledger_selector, ConstructMethod};
use layerx_crypto::disclosure::{AmountRole, CounterpartyRole, DisclosureError};
use layerx_crypto::evm_transaction::{assemble, decode_signed, recover_sender, Eip1559Call};
use layerx_crypto::signer::{sign_disclosed_call, LocalSigner, Signer};
use layerx_types::payload::{ActivityType, ModuleId};

const BUDGET: [u8; 32] = [0x31; 32];

fn ready<F: Future>(future: F) -> F::Output {
    let mut future = pin!(future);
    let mut context = Context::from_waker(Waker::noop());
    match future.as_mut().poll(&mut context) {
        Poll::Ready(value) => value,
        Poll::Pending => panic!("local signer future unexpectedly blocked"),
    }
}

fn budget_defund_call(amount: u128) -> Eip1559Call {
    let mut to = [0; 20];
    to[18..].copy_from_slice(&[0x10, 0x1b]);
    let mut data = ledger_selector("budgetDefund(bytes32,uint128)").to_vec();
    data.extend_from_slice(&BUDGET);
    data.extend_from_slice(&[0; 16]);
    data.extend_from_slice(&amount.to_be_bytes());
    Eip1559Call {
        chain_id: 125,
        nonce: 3,
        max_priority_fee_per_gas: 2,
        max_fee_per_gas: 30,
        gas_limit: 100_000,
        to,
        data,
    }
}

fn budget_fund_call(amount: u128) -> Eip1559Call {
    let mut to = [0; 20];
    to[18..].copy_from_slice(&[0x10, 0x1b]);
    let mut data = ledger_selector("budgetFund(bytes32,uint128)").to_vec();
    data.extend_from_slice(&BUDGET);
    data.extend_from_slice(&[0; 16]);
    data.extend_from_slice(&amount.to_be_bytes());
    Eip1559Call {
        chain_id: 125,
        nonce: 3,
        max_priority_fee_per_gas: 2,
        max_fee_per_gas: 30,
        gas_limit: 100_000,
        to,
        data,
    }
}

#[test]
fn bind_call_refuses_budget_create_payload_rule_violations() -> Result<(), Box<dyn Error>> {
    let sender = [0xa1; 20];
    let word = |value: u128| {
        let mut word = [0; 32];
        word[16..].copy_from_slice(&value.to_be_bytes());
        word
    };
    let budget_account = [0x32; 32];
    let asset = [0x33; 32];
    let purpose = [0x34; 32];
    let words = [
        BUDGET,
        budget_account,
        asset,
        purpose,
        word(100),
        word(1),
        word(123),
        word(1_000),
        word(2_000),
        word(3_000),
        word(7),
        word(2),
    ];
    let mut to = [0; 20];
    to[18..].copy_from_slice(&[0x10, 0x1b]);
    let mut data = ledger_selector(
        "budgetCreate(bytes32,bytes32,bytes32,bytes32,uint128,uint128,uint128,uint64,uint64,uint64,uint64,uint8)",
    )
    .to_vec();
    data.extend_from_slice(&words.concat());
    let call = Eip1559Call {
        chain_id: 125,
        nonce: 3,
        max_priority_fee_per_gas: 2,
        max_fee_per_gas: 30,
        gas_limit: 100_000,
        to,
        data,
    };
    let disclosure = bind_call(&call, sender)?;
    assert_eq!(
        disclosure.activity_type,
        ActivityType::new(ModuleId::Budget, 1).map_err(|e| format!("{e:?}"))?
    );
    assert_eq!(disclosure.idempotency_key, BUDGET);
    assert_eq!(disclosure.asset, asset);
    assert_eq!(disclosure.amounts.len(), 2);
    assert_eq!(disclosure.amounts[0].role, AmountRole::Transfer);
    assert_eq!(disclosure.amounts[0].value, 123);
    assert_eq!(disclosure.amounts[1].role, AmountRole::SpendingLimit);
    assert_eq!(disclosure.amounts[1].value, 100);
    let mut payload = 1_u16.to_be_bytes().to_vec();
    for value in [BUDGET, budget_account, asset, purpose] {
        payload.extend_from_slice(&value);
    }
    for value in [100_u128, 1, 123] {
        payload.extend_from_slice(&value.to_be_bytes());
    }
    for value in [1_000_u64, 2_000, 3_000, 7] {
        payload.extend_from_slice(&value.to_be_bytes());
    }
    payload.push(2);
    assert_eq!(
        disclosure.construct_method()?,
        Some(ConstructMethod::PayBudgetCreate { payload })
    );
    assert_eq!(disclosure.signing_digest()?, call.signing_hash());

    for (rule, index, value) in [
        ("zero budget id", 0, word(0)),
        ("zero budget account", 1, word(0)),
        ("zero asset", 2, word(0)),
        ("zero initial amount", 6, word(0)),
        ("zero per-period limit", 4, word(0)),
        ("zero period length", 7, word(0)),
        ("expiry equals period start", 9, word(2_000)),
        ("expiry before period start", 9, word(1_999)),
        ("rollover zero", 11, word(0)),
        ("rollover three", 11, word(3)),
        ("rollover one with carry cap one", 11, word(1)),
    ] {
        let mut invalid = call.clone();
        invalid.data[4 + index * 32..4 + (index + 1) * 32].copy_from_slice(&value);
        assert_eq!(
            bind_call(&invalid, sender),
            Err(DisclosureError::MalformedPayload),
            "{rule}"
        );
    }
    Ok(())
}

#[test]
fn bind_call_discloses_budget_defund() -> Result<(), Box<dyn Error>> {
    let account = derive_from_seed(&[0x07; 64], 0)?;
    let signer = LocalSigner::from_derived_account(&account);
    let sender = signer.evm_address().ok_or("missing EVM address")?;
    for amount in [1, 123, u128::MAX] {
        let call = budget_defund_call(amount);
        let disclosure = bind_call(&call, sender)?;
        assert_eq!(
            disclosure.activity_type,
            ActivityType::new(ModuleId::Budget, 8).map_err(|e| format!("{e:?}"))?
        );
        assert_eq!(disclosure.actor, sender);
        assert_eq!(disclosure.authority, sender);
        assert_eq!(disclosure.counterparties.len(), 1);
        assert_eq!(disclosure.counterparties[0].role, CounterpartyRole::Payer);
        assert_eq!(disclosure.counterparties[0].account, BUDGET);
        assert_eq!(disclosure.amounts.len(), 1);
        assert_eq!(disclosure.amounts[0].role, AmountRole::Transfer);
        assert_eq!(disclosure.amounts[0].value, amount);
        assert_eq!(disclosure.asset, [0; 32]);
        assert_eq!(
            disclosure.construct_method()?,
            Some(ConstructMethod::PayBudgetDefund {
                budget: BUDGET,
                amount,
            })
        );
        assert_eq!(disclosure.signing_digest()?, call.signing_hash());
        let signature = ready(sign_disclosed_call(&signer, &call, &disclosure, sender))?;
        assert_eq!(signature.as_bytes().len(), 65);
        assert_eq!(recover_sender(&call, &signature)?, sender);
        let raw = assemble(&call, &signature);
        let (decoded_call, decoded_signature) = decode_signed(&raw)?;
        assert_eq!(decoded_call, call);
        assert_eq!(decoded_signature, signature);
        assert_eq!(recover_sender(&decoded_call, &decoded_signature)?, sender);
    }
    Ok(())
}

#[test]
fn bind_call_refuses_noncanonical_budget_defund() {
    let sender = [0xa1; 20];
    let call = budget_defund_call(123);
    assert!(bind_call(&call, sender).is_ok());

    let mut trailing = call.clone();
    trailing.data.push(0);
    assert_eq!(
        bind_call(&trailing, sender),
        Err(DisclosureError::MalformedPayload)
    );

    for index in 36..52 {
        let mut padding = call.clone();
        padding.data[index] = 1;
        assert_eq!(
            bind_call(&padding, sender),
            Err(DisclosureError::MalformedPayload)
        );
    }

    assert_eq!(
        bind_call(&budget_defund_call(0), sender),
        Err(DisclosureError::MalformedPayload)
    );

    let mut unknown = call.clone();
    unknown.data[..4].copy_from_slice(&ledger_selector("openAccount(bytes32)"));
    assert_eq!(
        bind_call(&unknown, sender),
        Err(DisclosureError::MalformedPayload)
    );

    for length in 0..call.data.len() {
        let mut truncated = call.clone();
        truncated.data.truncate(length);
        assert_eq!(
            bind_call(&truncated, sender),
            Err(DisclosureError::MalformedPayload)
        );
    }

    let mut wrong_target = call;
    wrong_target.to[0] = 1;
    assert_eq!(
        bind_call(&wrong_target, sender),
        Err(DisclosureError::MalformedPayload)
    );
}

#[test]
fn bind_call_discloses_budget_fund() -> Result<(), Box<dyn Error>> {
    let account = derive_from_seed(&[0x07; 64], 0)?;
    let signer = LocalSigner::from_derived_account(&account);
    let sender = signer.evm_address().ok_or("missing EVM address")?;
    for amount in [1, 123, u128::MAX] {
        let call = budget_fund_call(amount);
        let disclosure = bind_call(&call, sender)?;
        assert_eq!(
            disclosure.activity_type,
            ActivityType::new(ModuleId::Budget, 2).map_err(|e| format!("{e:?}"))?
        );
        assert_eq!(disclosure.actor, sender);
        assert_eq!(disclosure.authority, sender);
        assert_eq!(disclosure.counterparties.len(), 1);
        assert_eq!(disclosure.counterparties[0].role, CounterpartyRole::Payer);
        assert_eq!(disclosure.counterparties[0].account, BUDGET);
        assert_eq!(disclosure.amounts.len(), 1);
        assert_eq!(disclosure.amounts[0].role, AmountRole::Transfer);
        assert_eq!(disclosure.amounts[0].value, amount);
        assert_eq!(disclosure.asset, [0; 32]);
        assert_eq!(disclosure.idempotency_key, [0; 32]);
        assert_eq!(
            disclosure.construct_method()?,
            Some(ConstructMethod::PayBudgetFund {
                budget: BUDGET,
                amount,
            })
        );
        assert_eq!(disclosure.signing_digest()?, call.signing_hash());
        let signature = ready(sign_disclosed_call(&signer, &call, &disclosure, sender))?;
        assert_eq!(signature.as_bytes().len(), 65);
        assert_eq!(recover_sender(&call, &signature)?, sender);
        let raw = assemble(&call, &signature);
        let (decoded_call, decoded_signature) = decode_signed(&raw)?;
        assert_eq!(decoded_call, call);
        assert_eq!(decoded_signature, signature);
        assert_eq!(recover_sender(&decoded_call, &decoded_signature)?, sender);
    }
    Ok(())
}

#[test]
fn bind_call_refuses_noncanonical_budget_fund() {
    let sender = [0xa1; 20];
    let call = budget_fund_call(123);
    assert!(bind_call(&call, sender).is_ok());

    let mut trailing = call.clone();
    trailing.data.push(0);
    assert_eq!(
        bind_call(&trailing, sender),
        Err(DisclosureError::MalformedPayload)
    );

    for index in 36..52 {
        let mut padding = call.clone();
        padding.data[index] = 1;
        assert_eq!(
            bind_call(&padding, sender),
            Err(DisclosureError::MalformedPayload)
        );
    }

    assert_eq!(
        bind_call(&budget_fund_call(0), sender),
        Err(DisclosureError::MalformedPayload)
    );

    let mut unknown = call.clone();
    unknown.data[..4].copy_from_slice(&ledger_selector("openAccount(bytes32)"));
    assert_eq!(
        bind_call(&unknown, sender),
        Err(DisclosureError::MalformedPayload)
    );

    for length in 0..call.data.len() {
        let mut truncated = call.clone();
        truncated.data.truncate(length);
        assert_eq!(
            bind_call(&truncated, sender),
            Err(DisclosureError::MalformedPayload)
        );
    }

    let mut wrong_target = call;
    wrong_target.to[0] = 1;
    assert_eq!(
        bind_call(&wrong_target, sender),
        Err(DisclosureError::MalformedPayload)
    );
}
