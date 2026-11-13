use std::fmt::Debug;

use layerx_types::payload::Payload;
use sha2::{Digest as _, Sha256};

pub const OWNER_DID: &str = "did:layerx:owner-receipt-fixture";

pub struct SignedOwner {
    pub bytes: Vec<u8>,
    pub id: [u8; 32],
    pub public_key: [u8; 32],
    pub actor: Vec<u8>,
}

fn checked<T, E: Debug>(value: Result<T, E>) -> T {
    value.unwrap_or_else(|error| panic!("canonical owner encoding: {error:?}"))
}

/// Signs the owner transaction for a supported protocol version.
///
/// # Panics
/// Panics when the version or payload violates the owner transaction contract.
#[must_use]
pub fn sign_transaction(
    payload: &Payload,
    action_key: [u8; 32],
    version: u16,
    network: u32,
) -> SignedOwner {
    assert!(layerx_wire::limits::protocol_version_supported(version));
    assert_ne!(network, 0);
    let (address, data) = match payload.activity_type().module() {
        layerx_types::payload::ModuleId::Budget => {
            let data = match payload.activity_type().ordinal() {
                1 => {
                    assert_eq!(&payload.as_bytes()[2..34], &action_key);
                    budget_create_calldata(payload)
                }
                2 => {
                    let mut funding = layerx_wire::decode::Decoder::new(payload.as_bytes(), 0);
                    assert_eq!(checked(funding.u16()), 1);
                    let mut data = layerx_crypto::disclosure::call::ledger_selector(
                        "budgetFund(bytes32,uint128)",
                    )
                    .to_vec();
                    data.extend_from_slice(checked(funding.fixed(32)));
                    let amount = checked(funding.u128());
                    assert_ne!(amount, 0);
                    checked(funding.finish());
                    data.extend_from_slice(&[0; 16]);
                    data.extend_from_slice(&amount.to_be_bytes());
                    data
                }
                ordinal => panic!("unsupported budget lifecycle ordinal: {ordinal}"),
            };
            (0x101b_u16, data)
        }
        layerx_types::payload::ModuleId::Governance => {
            let mut data =
                layerx_crypto::disclosure::call::ledger_selector("recordLifecycle(bytes,bytes32)")
                    .to_vec();
            let mut offset = [0; 32];
            offset[31] = 64;
            data.extend_from_slice(&offset);
            data.extend_from_slice(&action_key);
            let mut length = [0; 32];
            length[24..]
                .copy_from_slice(&checked(u64::try_from(payload.as_bytes().len())).to_be_bytes());
            data.extend_from_slice(&length);
            data.extend_from_slice(payload.as_bytes());
            data.resize(4 + (data.len() - 4).div_ceil(32) * 32, 0);
            (0x101c_u16, data)
        }
        module => panic!("unsupported lifecycle module: {module:?}"),
    };
    let mut to = [0; 20];
    to[18..].copy_from_slice(&address.to_be_bytes());
    let call = layerx_crypto::evm_transaction::Eip1559Call {
        chain_id: u64::from(network),
        nonce: 1,
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 10,
        gas_limit: 100_000,
        to,
        data,
    };
    let bytes = checked(call.sign(&[0x68; 32]));
    assert_eq!(bytes[0], 2);
    let (decoded, signature) = checked(layerx_crypto::evm_transaction::decode_signed(&bytes));
    assert_eq!(decoded, call);
    assert_eq!(
        layerx_crypto::evm_transaction::assemble(&decoded, &signature),
        bytes
    );
    let sender = checked(layerx_crypto::evm_transaction::recover_sender(
        &decoded, &signature,
    ));
    let disclosure = checked(layerx_crypto::disclosure::call::bind_call(&decoded, sender));
    assert_eq!(disclosure.activity_type, payload.activity_type());
    let expected_idempotency_key = match checked(disclosure.construct_method()) {
        Some(layerx_crypto::disclosure::call::ConstructMethod::PayBudgetFund { .. }) => [0; 32],
        _ => action_key,
    };
    assert_eq!(disclosure.idempotency_key, expected_idempotency_key);
    let mut public_key = [0; 32];
    public_key[12..].copy_from_slice(&sender);
    SignedOwner {
        id: layerx_crypto::evm_transaction::transaction_hash(&bytes),
        bytes,
        public_key,
        actor: disclosure.actor,
    }
}

fn budget_create_calldata(payload: &Payload) -> Vec<u8> {
    let mut budget = layerx_wire::decode::Decoder::new(payload.as_bytes(), 0);
    assert_eq!(checked(budget.u16()), 1);
    let mut data = layerx_crypto::disclosure::call::ledger_selector(
        "budgetCreate(bytes32,bytes32,bytes32,bytes32,uint128,uint128,uint128,uint64,uint64,uint64,uint64,uint8)",
    )
    .to_vec();
    for _ in 0..4 {
        data.extend_from_slice(checked(budget.fixed(32)));
    }
    for _ in 0..3 {
        data.extend_from_slice(&[0; 16]);
        data.extend_from_slice(&checked(budget.u128()).to_be_bytes());
    }
    for _ in 0..4 {
        data.extend_from_slice(&[0; 24]);
        data.extend_from_slice(&checked(budget.u64()).to_be_bytes());
    }
    data.extend_from_slice(&[0; 31]);
    data.push(checked(budget.u8()));
    checked(budget.finish());
    data
}

fn assert_nonmonetary_receipt(protocol: &layerx_wire::receipt::ProtocolReceipt) {
    assert_eq!(protocol.operation(), 0);
    assert_eq!(protocol.asset(), [0; 32]);
    assert_eq!(protocol.amount(), 0);
    assert_eq!(protocol.from(), [0; 32]);
    assert_eq!(protocol.to(), [0; 32]);
    assert_eq!(protocol.debit_sequence(), 0);
    assert_eq!(protocol.debit_balance_before(), 0);
    assert_eq!(protocol.debit_balance_after(), 0);
    assert_eq!(protocol.credit_balance_before(), 0);
    assert_eq!(protocol.credit_balance_after(), 0);
    assert_eq!(protocol.transfer_set_root(), [0; 32]);
    assert_eq!(protocol.authorization_hash(), [0; 32]);
    assert_eq!(protocol.context_hash(), [0; 32]);
    assert!(protocol.program_outcome().is_none());
    assert!(protocol.total_units().is_none());
    assert!(!protocol
        .effects()
        .iter()
        .any(layerx_wire::receipt::Effect::monetary));
}

/// Asserts that the owner evidence matches the authorized batch.
///
/// # Panics
/// Panics when a lifecycle binding differs from the authorized batch.
pub fn assert_outcome(
    evidence: &layerx_human_service::agents::ProtocolEvidence,
    kind: layerx_types::payload::ActivityType,
    authorized_batch: &layerx_proof::receipt::AuthorizedBatch,
) {
    let (call, signature) = checked(layerx_crypto::evm_transaction::decode_signed(
        &evidence.signed_activity,
    ));
    let sender = checked(layerx_crypto::evm_transaction::recover_sender(
        &call, &signature,
    ));
    let disclosure = checked(layerx_crypto::disclosure::call::bind_call(&call, sender));
    let mut authority = [0; 32];
    authority[12..].copy_from_slice(&sender);
    let transaction_hash =
        layerx_crypto::evm_transaction::transaction_hash(&evidence.signed_activity);
    assert_eq!(
        disclosure.activity_type, kind,
        "lifecycle activity_type binding"
    );
    assert_eq!(disclosure.actor, evidence.actor, "lifecycle actor binding");
    assert_eq!(
        authority, evidence.owner_public_key,
        "lifecycle owner_public_key authority binding"
    );
    assert_eq!(
        call.chain_id,
        u64::from(evidence.network_id),
        "lifecycle chain_id network_id binding"
    );
    assert_ne!(
        evidence.network_id, 0,
        "lifecycle network_id must be non-zero"
    );
    let expected_idempotency_key = match checked(disclosure.construct_method()) {
        Some(layerx_crypto::disclosure::call::ConstructMethod::PayBudgetCreate { payload }) => {
            checked(payload[2..34].try_into())
        }
        Some(layerx_crypto::disclosure::call::ConstructMethod::PayBudgetFund { .. }) => [0; 32],
        _ => evidence.action_key,
    };
    assert_eq!(
        disclosure.idempotency_key, expected_idempotency_key,
        "lifecycle idempotency_key action_key binding"
    );
    assert_eq!(
        transaction_hash, evidence.activity_id,
        "lifecycle transaction hash activity_id binding"
    );
    assert_eq!(
        transaction_hash, evidence.tx_hash,
        "lifecycle transaction hash tx_hash binding"
    );
    let activity = checked(evidence.bound_activity(kind));
    assert_ne!(evidence.action_key, [0; 32]);
    assert!((2..=7).contains(&(kind.module() as u16)));
    assert_eq!(activity.protocol_version(), 3);
    assert_eq!(
        layerx_crypto::evm_transaction::assemble(&call, &signature),
        evidence.signed_activity
    );
    let authenticated = checked(layerx_proof::receipt::verify_sequencer_signature(
        &evidence.receipt_bytes,
        authorized_batch.sequencer_public_key(),
    ));
    let receipt = checked(evidence.verify_outcome(kind));
    assert_eq!(
        checked(layerx_wire::receipt::encode(&authenticated)),
        checked(layerx_wire::receipt::encode(&receipt))
    );
    let protocol = receipt.protocol().or_fail("protocol receipt");
    assert_eq!(protocol.batch_id(), authorized_batch.batch_id());
    assert_eq!(protocol.asset(), authorized_batch.asset());
    assert_eq!(
        protocol.previous_state_root(),
        authorized_batch.previous_state_root()
    );
    assert_eq!(
        protocol.resulting_state_root(),
        authorized_batch.resulting_state_root()
    );
    assert_nonmonetary_receipt(protocol);
    assert_eq!(evidence.tx_hash, protocol.activity_id());
    assert_eq!(evidence.tx_hash, evidence.activity_id);
    assert_eq!(evidence.rung, layerx_types::rung::Rung::Committed);
    assert_eq!(
        &protocol.batch_id()[24..],
        &evidence.block_height.to_be_bytes()
    );
    assert_eq!(
        evidence.receipt_hash,
        <[u8; 32]>::from(Sha256::digest(&evidence.receipt_bytes))
    );
}

use crate::require_value::RequireValue as _;
