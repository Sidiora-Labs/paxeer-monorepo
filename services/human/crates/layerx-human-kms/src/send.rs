use crate::evm_types::EvmPlanAuthorization;
use crate::wire::{Error, Result};
use layerx_crypto::disclosure::call::{bind_call, ConstructMethod};
use layerx_crypto::evm_transaction::Eip1559Call;
use serde::{Deserialize, Serialize};

#[derive(Clone, Debug, Eq, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct SendDisclosure {
    pub recipient: [u8; 20],
    pub denom: String,
    pub amount: [u8; 32],
}

#[derive(Clone, Debug, Eq, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct SendAuthorization {
    pub authorization: EvmPlanAuthorization,
    pub disclosure: SendDisclosure,
}

pub(crate) fn digest(
    value: &SendAuthorization,
    binding: [u8; 32],
    network: u32,
    protocol: u16,
    now: u64,
) -> Result<[u8; 32]> {
    let plan = &value.authorization;
    let tx = &plan.transaction;
    let mut bank = [0; 20];
    bank[18..].copy_from_slice(&[0x10, 0x01]);
    if plan.binding_digest != binding
        || tx.chain_id != u64::from(network)
        || protocol != 3
        || plan.plan_id == [0; 32]
        || plan.action_key == [0; 32]
        || plan.principal.is_empty()
        || plan.principal.len() > 128
        || plan.tenant.is_empty()
        || plan.tenant.len() > 128
        || plan.wallet == [0; 20]
        || plan.not_before > now
        || plan.not_after < now
        || plan.not_after <= plan.not_before
        || tx.to != bank
        || tx.value != [0; 32]
        || tx.gas_limit < 21_000
        || tx.max_fee_per_gas == 0
        || tx.max_priority_fee_per_gas > tx.max_fee_per_gas
        || tx.calldata.len() > 524_288
        || value.disclosure.recipient == [0; 20]
        || value.disclosure.denom.is_empty()
        || value.disclosure.amount == [0; 32]
    {
        return Err(Error::Refused);
    }
    let call = Eip1559Call {
        chain_id: tx.chain_id,
        nonce: tx.nonce,
        max_priority_fee_per_gas: tx.max_priority_fee_per_gas.into(),
        max_fee_per_gas: tx.max_fee_per_gas.into(),
        gas_limit: tx.gas_limit,
        to: tx.to,
        data: tx.calldata.clone(),
    };
    let disclosed = bind_call(&call, plan.wallet).map_err(|_| Error::Refused)?;
    match disclosed.construct_method().map_err(|_| Error::Refused)? {
        Some(ConstructMethod::BankSend {
            from,
            to,
            denom,
            amount,
        }) if from == plan.wallet
            && to == value.disclosure.recipient
            && denom == value.disclosure.denom
            && amount == value.disclosure.amount =>
        {
            Ok(call.signing_hash())
        }
        _ => Err(Error::Refused),
    }
}
