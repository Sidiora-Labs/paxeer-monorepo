use serde::{Deserialize, Serialize};

#[derive(Clone, Debug, Eq, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct EvmTransaction {
    pub chain_id: u64,
    pub nonce: u64,
    pub max_priority_fee_per_gas: u64,
    pub max_fee_per_gas: u64,
    pub gas_limit: u64,
    pub to: [u8; 20],
    pub value: [u8; 32],
    pub calldata: Vec<u8>,
}

impl EvmTransaction {
    /// Builds the canonical bank call for an exact first-funding disclosure.
    pub fn with_bank_transfer(
        mut self,
        sender: [u8; 20],
        recipient: [u8; 20],
        denom: &str,
        amount: [u8; 32],
    ) -> Result<Self, super::CustodyError> {
        let mut bank = [0; 20];
        bank[18..].copy_from_slice(&0x1001_u16.to_be_bytes());
        if self.to != bank
            || self.value != [0; 32]
            || !self.calldata.is_empty()
            || sender == [0; 20]
            || recipient == [0; 20]
            || amount == [0; 32]
            || denom.is_empty()
            || denom.len() > 128
            || !denom.is_ascii()
        {
            return Err(super::CustodyError::InvalidEvidence);
        }
        let mut data = layerx_crypto::disclosure::call::ledger_selector(
            "send(address,address,string,uint256)",
        )
        .to_vec();
        for address in [sender, recipient] {
            data.extend_from_slice(&[0; 12]);
            data.extend_from_slice(&address);
        }
        let mut word = [0; 32];
        word[31] = 128;
        data.extend_from_slice(&word);
        data.extend_from_slice(&amount);
        word = [0; 32];
        word[24..].copy_from_slice(&(denom.len() as u64).to_be_bytes());
        data.extend_from_slice(&word);
        data.extend_from_slice(denom.as_bytes());
        data.resize(4 + 160 + denom.len().div_ceil(32) * 32, 0);
        self.calldata = data;
        Ok(self)
    }
}

#[derive(Clone, Debug, Eq, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct EvmPlanAuthorization {
    pub plan_id: [u8; 32],
    pub action_key: [u8; 32],
    pub tenant: String,
    pub principal: String,
    pub binding_digest: [u8; 32],
    pub wallet: [u8; 20],
    pub not_before: u64,
    pub not_after: u64,
    pub transaction: EvmTransaction,
}

#[derive(Clone, Debug, Eq, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct EvmAction {
    pub authorization: EvmPlanAuthorization,
    pub raw_transaction: Vec<u8>,
    pub transaction_hash: Option<[u8; 32]>,
    pub acknowledged: bool,
}

#[derive(Clone, Debug, Eq, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct EvmAcknowledgement {
    pub action_key: [u8; 32],
    pub transaction_hash: [u8; 32],
}

/// An executable bank transfer retaining the complete recoverable signature.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct SignedBankTransfer {
    raw_transaction: Vec<u8>,
    signature: [u8; 65],
    sender: [u8; 20],
}

impl SignedBankTransfer {
    pub fn raw_transaction(&self) -> &[u8] {
        &self.raw_transaction
    }

    pub const fn signature(&self) -> &[u8; 65] {
        &self.signature
    }

    pub const fn sender(&self) -> [u8; 20] {
        self.sender
    }

    pub fn transaction_hash(&self) -> [u8; 32] {
        layerx_crypto::evm_transaction::transaction_hash(&self.raw_transaction)
    }
}

impl super::CustodySigner {
    /// Authorizes first funding as an exact EIP-1559 bank transfer.
    pub fn authorize_send_transaction(
        &self,
        principal: &crate::store::PrincipalId,
        key: &super::KeyId,
        plan: &EvmPlanAuthorization,
        recipient: [u8; 20],
        denom: String,
        amount: [u8; 32],
    ) -> Result<SignedBankTransfer, super::CustodyError> {
        let raw_transaction = self
            .creation_keystore()
            .authorize_send_transaction(principal, key, plan, recipient, denom, amount)?;
        let (_, signature) = layerx_crypto::evm_transaction::decode_signed(&raw_transaction)
            .map_err(|_| super::CustodyError::Kms(super::KmsError::InvalidResponse))?;
        Ok(SignedBankTransfer {
            raw_transaction,
            signature: *signature.as_bytes(),
            sender: plan.wallet,
        })
    }
}

use super::{CustodyError, KeyId, Keystore, KmsError};
use crate::store::PrincipalId;

impl Keystore {
    /// Returns the existing opaque provider handle after binding validation.
    /// # Errors
    /// Refuses missing or mismatched custody records.
    pub fn evm_provider_reference(
        &self,
        principal: &PrincipalId,
        key: &KeyId,
    ) -> Result<super::ProviderKeyReference, CustodyError> {
        let record = self.read_record(principal, key)?;
        Self::require_record_binding(&self.binding(principal, key, record.class)?, &record)?;
        Ok(record.provider_reference)
    }

    pub fn public_wallet_identity(
        &self,
        principal: &PrincipalId,
        key: &KeyId,
    ) -> Result<
        (
            [u8; 20],
            super::PrincipalKeyBinding,
            super::ProviderKeyReference,
        ),
        CustodyError,
    > {
        let record = self.read_record(principal, key)?;
        let binding = self.binding(principal, key, record.class)?;
        Self::require_record_binding(&binding, &record)?;
        let wallet = self
            .provider
            .evm_operation(6, &binding, &record.provider_reference, &[])
            .map_err(CustodyError::Kms)?
            .try_into()
            .map_err(|_| CustodyError::Kms(KmsError::InvalidResponse))?;
        Ok((wallet, binding, record.provider_reference))
    }

    /// Resolves the EVM wallet attached to an existing principal custody key.
    /// # Errors
    /// Refuses absent, destroyed or mismatched custody records.
    pub fn evm_wallet(
        &self,
        principal: &PrincipalId,
        key: &KeyId,
    ) -> Result<[u8; 20], CustodyError> {
        self.evm_call(principal, key, 6, &[])?
            .try_into()
            .map_err(|_| CustodyError::Kms(KmsError::InvalidResponse))
    }
    /// Returns the custody binding required by an exact authorized plan.
    /// # Errors
    /// Refuses missing or mismatched custody records.
    pub fn evm_binding(
        &self,
        principal: &PrincipalId,
        key: &KeyId,
    ) -> Result<super::PrincipalKeyBinding, CustodyError> {
        let record = self.read_record(principal, key)?;
        let binding = self.binding(principal, key, record.class)?;
        Self::require_record_binding(&binding, &record)?;
        Ok(binding)
    }
    /// Persists an authorized exact transaction and reserves its nonce.
    /// # Errors
    /// Refuses expired plans, wrong bindings or conflicting action keys/nonces.
    pub fn authorize_evm_plan(
        &self,
        principal: &PrincipalId,
        key: &KeyId,
        authorization: &EvmPlanAuthorization,
    ) -> Result<EvmAction, CustodyError> {
        if authorization.principal != principal.as_str() {
            return Err(CustodyError::Kms(KmsError::Refused));
        }
        self.evm_json(principal, key, 7, authorization)
    }
    /// Signs a previously authorized transaction, returning the durable result.
    /// # Errors
    /// Refuses unregistered or expired actions.
    pub fn sign_evm_action(
        &self,
        principal: &PrincipalId,
        key: &KeyId,
        action: &[u8; 32],
    ) -> Result<EvmAction, CustodyError> {
        self.evm_json(principal, key, 8, action)
    }
    /// Records submission only for the exact signed transaction hash.
    /// # Errors
    /// Refuses unregistered actions or mismatched hashes.
    pub fn acknowledge_evm_action(
        &self,
        principal: &PrincipalId,
        key: &KeyId,
        ack: &EvmAcknowledgement,
    ) -> Result<EvmAction, CustodyError> {
        self.evm_json(principal, key, 9, ack)
    }
    /// Recovers the authorization, signed bytes and submission journal state.
    /// # Errors
    /// Refuses absent actions or mismatched principal custody keys.
    pub fn recover_evm_action(
        &self,
        principal: &PrincipalId,
        key: &KeyId,
        action: &[u8; 32],
    ) -> Result<EvmAction, CustodyError> {
        self.evm_json(principal, key, 10, action)
    }
    fn evm_json<T: Serialize>(
        &self,
        principal: &PrincipalId,
        key: &KeyId,
        operation: u8,
        value: &T,
    ) -> Result<EvmAction, CustodyError> {
        let payload = serde_json::to_vec(value)
            .map_err(|_| CustodyError::Kms(KmsError::InvalidConfiguration))?;
        serde_json::from_slice(&self.evm_call(principal, key, operation, &payload)?)
            .map_err(|_| CustodyError::Kms(KmsError::InvalidResponse))
    }
    fn evm_call(
        &self,
        principal: &PrincipalId,
        key: &KeyId,
        operation: u8,
        payload: &[u8],
    ) -> Result<Vec<u8>, CustodyError> {
        let record = self.read_record(principal, key)?;
        let binding = self.binding(principal, key, record.class)?;
        Self::require_record_binding(&binding, &record)?;
        self.provider
            .evm_operation(operation, &binding, &record.provider_reference, payload)
            .map_err(CustodyError::Kms)
    }
}

#[derive(Clone, Debug, Eq, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct SendPlanAuthorization {
    pub plan_id: [u8; 32],
    pub action_key: [u8; 32],
    pub principal: String,
    pub tenant: String,
    pub binding_digest: [u8; 32],
    pub from: [u8; 32],
    pub to: [u8; 32],
    pub asset: [u8; 32],
    pub amount: u128,
    pub sequence: u64,
    pub idempotency_key: [u8; 32],
    pub expires_at: u64,
    pub context: [u8; 32],
    pub network: u32,
    pub protocol: u16,
    pub not_before: u64,
    pub not_after: u64,
}
#[derive(Clone, Debug, Eq, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub(super) struct EvmSendAuthorization {
    pub(super) authorization: EvmPlanAuthorization,
    pub(super) disclosure: SendDisclosure,
}

#[derive(Clone, Debug, Eq, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub(super) struct SendDisclosure {
    pub(super) recipient: [u8; 20],
    pub(super) denom: String,
    pub(super) amount: [u8; 32],
}

impl Keystore {
    /// Signs an exact bank transfer with its recipient and amount disclosure.
    /// # Errors
    /// Refuses a mismatched binding, wallet, transaction or signed response.
    pub fn authorize_send_transaction(
        &self,
        principal: &PrincipalId,
        key: &KeyId,
        plan: &EvmPlanAuthorization,
        recipient: [u8; 20],
        denom: String,
        amount: [u8; 32],
    ) -> Result<Vec<u8>, CustodyError> {
        let binding = self.evm_binding(principal, key)?;
        if plan.principal != principal.as_str()
            || plan.binding_digest != binding.digest()
            || plan.transaction.chain_id != u64::from(binding.network_id())
            || plan.wallet != self.evm_wallet(principal, key)?
            || plan.transaction.value != [0; 32]
        {
            return Err(CustodyError::Kms(KmsError::Refused));
        }
        let tx = &plan.transaction;
        let call = layerx_crypto::evm_transaction::Eip1559Call {
            chain_id: tx.chain_id,
            nonce: tx.nonce,
            max_priority_fee_per_gas: tx.max_priority_fee_per_gas.into(),
            max_fee_per_gas: tx.max_fee_per_gas.into(),
            gas_limit: tx.gas_limit,
            to: tx.to,
            data: tx.calldata.clone(),
        };
        let disclosure = layerx_crypto::disclosure::call::bind_call(&call, plan.wallet)
            .map_err(|_| CustodyError::Kms(KmsError::Refused))?;
        match disclosure
            .construct_method()
            .map_err(|_| CustodyError::Kms(KmsError::Refused))?
        {
            Some(layerx_crypto::disclosure::call::ConstructMethod::BankSend {
                from,
                to,
                denom: disclosed_denom,
                amount: disclosed_amount,
            }) if from == plan.wallet
                && to == recipient
                && disclosed_denom == denom
                && disclosed_amount == amount => {}
            _ => return Err(CustodyError::Kms(KmsError::Refused)),
        }
        let authorization = EvmSendAuthorization {
            authorization: plan.clone(),
            disclosure: SendDisclosure {
                recipient,
                denom,
                amount,
            },
        };
        let payload = serde_json::to_vec(&authorization)
            .map_err(|_| CustodyError::Kms(KmsError::InvalidConfiguration))?;
        let raw = self.evm_call(principal, key, 11, &payload)?;
        let (call, signature) = layerx_crypto::evm_transaction::decode_signed(&raw)
            .map_err(|_| CustodyError::Kms(KmsError::InvalidResponse))?;
        let tx = &authorization.authorization.transaction;
        if call.chain_id != tx.chain_id
            || call.nonce != tx.nonce
            || call.to != tx.to
            || call.data != tx.calldata
            || call.gas_limit != tx.gas_limit
            || call.max_fee_per_gas != u128::from(tx.max_fee_per_gas)
            || call.max_priority_fee_per_gas != u128::from(tx.max_priority_fee_per_gas)
            || tx.value != [0; 32]
            || layerx_crypto::evm_transaction::recover_sender(&call, &signature)
                .map_err(|_| CustodyError::Kms(KmsError::InvalidResponse))?
                != authorization.authorization.wallet
        {
            return Err(CustodyError::Kms(KmsError::Integrity));
        }
        Ok(raw)
    }

    /// Refuses the retired kernel SEND authorization, which has no EIP-1559 call.
    /// Callers must supply an exact transaction to `authorize_send_transaction`.
    pub fn authorize_send(
        &self,
        _principal: &PrincipalId,
        _key: &KeyId,
        _authorization: &SendPlanAuthorization,
    ) -> Result<[u8; 64], CustodyError> {
        Err(CustodyError::Sign(
            layerx_crypto::signer::SignError::InvalidDisclosure,
        ))
    }
}

#[derive(Clone, Debug, Eq, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct EvmExternalSignature {
    pub action_key: [u8; 32],
    pub signature: Vec<u8>,
}
