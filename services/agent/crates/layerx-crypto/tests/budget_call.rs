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
