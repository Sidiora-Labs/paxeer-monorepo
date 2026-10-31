use std::fmt::Debug;

use ed25519_dalek::{Signer as _, SigningKey};
use layerx_crypto::SignatureMessage;
use layerx_intents::canonical;
use layerx_types::activity::{Authority, EnvelopeBuilder, Signature, TimestampBound};
use layerx_types::amount::Amount;
use layerx_types::ids::{Did, IdempotencyKey};
use layerx_types::payload::{ModuleRegistration, ModuleRegistry, Payload};
use sha2::{Digest as _, Sha256};

pub struct SignedOwner {
    pub bytes: Vec<u8>,
    pub id: [u8; 32],
    pub public_key: [u8; 32],
    pub actor: Vec<u8>,
}

fn checked<T, E: Debug>(value: Result<T, E>) -> T {
    value.unwrap_or_else(|error| panic!("canonical owner encoding: {error:?}"))
}

pub fn sign(payload: &Payload, action_key: [u8; 32], version: u16, network: u32) -> SignedOwner {
    let signer = SigningKey::from_bytes(&[0x68; 32]);
    let public_key = signer.verifying_key().to_bytes();
    let actor = b"did:layerx:owner-receipt-fixture".to_vec();
    let mut builder = EnvelopeBuilder::new();
    checked(builder.protocol_version(version));
    checked(builder.network_id(network));
    checked(builder.activity_type(payload.activity_type()));
    checked(builder.actor_did(checked(Did::new(&actor))));
    checked(builder.authority(checked(Authority::owner(&public_key))));
    checked(builder.account_sequence(1));
    checked(builder.timestamp_bound(checked(TimestampBound::new(900, 2000))));
    checked(builder.idempotency_key(IdempotencyKey::new(action_key)));
    checked(builder.fee_limit(Amount::from_u128(10)));
    checked(builder.payload_hash(checked(canonical::payload_hash_for(payload))));
    checked(builder.payload(payload.clone()));
    let unsigned = checked(builder.build());
    let canonical = checked(canonical::unsigned_envelope_bytes(&unsigned));
    let digest = checked(SignatureMessage::new(
        canonical::Domain::SignaturePreimage,
        version,
        network,
        &canonical,
    ))
    .digest();
    let signature = signer.sign(&digest).to_bytes();
    let bytes = checked(canonical::signed_envelope_bytes(
        &unsigned.attach_signature(checked(Signature::new(&signature))),
    ));
    let registry = checked(ModuleRegistry::new(&[checked(ModuleRegistration::new(
        payload.activity_type().module(),
        &[payload.activity_type()],
    ))]));
    let decoded = checked(canonical::decode_signed_activity(&bytes, &registry));
    SignedOwner {
        id: checked(canonical::activity_id(&decoded)),
        bytes,
        public_key,
        actor,
    }
}

pub fn sign_transaction(
    payload: &Payload,
    action_key: [u8; 32],
    version: u16,
    network: u32,
) -> SignedOwner {
    assert!(layerx_wire::limits::protocol_version_supported(version));
    assert_ne!(network, 0);
    assert_eq!(
        payload.activity_type().module(),
        layerx_types::payload::ModuleId::Budget
    );
    assert_eq!(payload.activity_type().ordinal(), 1);
    assert_eq!(&payload.as_bytes()[2..34], &action_key);
    let mut to = [0; 20];
    to[18..].copy_from_slice(&0x101b_u16.to_be_bytes());
    let call = layerx_crypto::evm_transaction::Eip1559Call {
        chain_id: u64::from(network),
        nonce: 1,
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 10,
        gas_limit: 100_000,
        to,
        data: budget_create_calldata(payload),
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
    assert_eq!(disclosure.idempotency_key, action_key);
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

pub fn assert_outcome(
    evidence: &layerx_human_service::agents::ProtocolEvidence,
    kind: layerx_types::payload::ActivityType,
    authorized_batch: &layerx_proof::receipt::AuthorizedBatch,
) {
    checked(evidence.bound_activity(kind));
    checked(layerx_proof::receipt::verify_native_owner_outcome(
        &evidence.receipt_bytes,
        authorized_batch,
        &layerx_proof::receipt::NativeOwnerOutcomeContext {
            canonical_activity: &evidence.signed_activity,
            actor: &evidence.actor,
            action_key: evidence.action_key,
            activity_type: kind,
            owner_public_key: evidence.owner_public_key,
            network_id: evidence.network_id,
        },
    ));
    let receipt = checked(evidence.verify_outcome(kind));
    let protocol = receipt.protocol().expect("protocol receipt");
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
