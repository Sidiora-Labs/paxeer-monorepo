use std::fmt::Debug;

use layerx_crypto::disclosure::call::{bind_call, ledger_selector};
use layerx_crypto::evm_transaction::{
    decode_signed, recover_sender, transaction_hash, Eip1559Call,
};
use layerx_human_service::agents::{AgentFailure, ProtocolEvidence};
use layerx_human_service::server::agent_creation::ProductionAgentCreation;
use layerx_intents::NativeOwnerBootstrap;
use layerx_proof::receipt::verify_sequencer_signature;
use layerx_types::ids::Did;
use layerx_types::intent::PublicKey;
use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};
use layerx_types::rung::Rung;
use sha2::{Digest as _, Sha256};

fn checked<T, E: Debug>(value: Result<T, E>) -> T {
    value.unwrap_or_else(|error| panic!("original native lifecycle evidence: {error:?}"))
}

fn evidence() -> ProtocolEvidence {
    let receipt = include_bytes!(
        "../../../../../tests/fixtures/authority/native-sessions/authentication/grant.receipt"
    );
    let sequencer = *include_bytes!("../../../../../tests/fixtures/authority/native-sessions/authentication/grant.sequencer-public");
    let kind = checked(ActivityType::new(ModuleId::Governance, 1));
    let registry = checked(ModuleRegistry::new(&[checked(ModuleRegistration::new(
        ModuleId::Governance,
        &[kind],
    ))]));
    let authenticated = checked(verify_sequencer_signature(receipt, sequencer));
    let protocol = authenticated
        .protocol()
        .unwrap_or_else(|| panic!("missing native receipt"));
    assert_eq!(protocol.operation(), 0);
    let compiled = checked(
        NativeOwnerBootstrap::Identity {
            did: checked(Did::new(b"did:layerx:native-lifecycle")),
            primary_key: PublicKey::new([0x11; 32]),
        }
        .compile(&registry),
    );
    let action_key = [0x42; 32];
    let mut data = ledger_selector("recordLifecycle(bytes,bytes32)").to_vec();
    let mut offset = [0; 32];
    offset[31] = 64;
    data.extend_from_slice(&offset);
    data.extend_from_slice(&action_key);
    let mut length = [0; 32];
    length[24..].copy_from_slice(
        &checked(u64::try_from(compiled.payload().as_bytes().len())).to_be_bytes(),
    );
    data.extend_from_slice(&length);
    data.extend_from_slice(compiled.payload().as_bytes());
    data.resize(4 + (data.len() - 4).div_ceil(32) * 32, 0);
    let mut to = [0; 20];
    to[18..].copy_from_slice(&0x101c_u16.to_be_bytes());
    let call = Eip1559Call {
        chain_id: 77,
        nonce: 1,
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 10,
        gas_limit: 100_000,
        to,
        data,
    };
    let signed = checked(call.sign(&[0x68; 32]));
    let (decoded, signature) = checked(decode_signed(&signed));
    assert_eq!(decoded, call);
    let sender = checked(recover_sender(&decoded, &signature));
    let disclosure = checked(bind_call(&decoded, sender));
    let mut public = [0; 32];
    public[12..].copy_from_slice(&sender);
    let tx_hash = transaction_hash(&signed);
    ProtocolEvidence {
        signed_activity: signed,
        owner_public_key: public,
        actor: disclosure.actor.clone(),
        network_id: 77,
        action_key,
        activity_id: tx_hash,
        receipt_bytes: receipt.to_vec(),
        tx_hash,
        receipt_hash: Sha256::digest(receipt).into(),
        block_height: u64::from_be_bytes(
            protocol.batch_id()[24..].try_into().or_fail("block height"),
        ),
        rung: Rung::Receipted,
    }
}

#[test]
fn native_lifecycle_binds_distinct_action_and_activity_without_inventing_finality() {
    let evidence = evidence();
    let kind = checked(ActivityType::new(ModuleId::Governance, 1));
    assert_ne!(evidence.action_key, evidence.activity_id);
    let original = checked(evidence.bound_activity(kind));
    assert_eq!(original.activity_type(), kind);
    assert_eq!(original.idempotency_key(), evidence.action_key);
    assert_eq!(original.fee_limit(), original.disclosure.fee_limit);
    let method = checked(original.disclosure.construct_method());
    let Some(layerx_crypto::disclosure::call::ConstructMethod::AuthLifecycle { payload, .. }) =
        method
    else {
        panic!("expected an authentication lifecycle call");
    };
    assert_eq!(original.payload(), payload);
    assert_ne!(original.payload(), original.call.data);
    assert_eq!(
        checked(ProtocolEvidence::creation_idempotency_key(
            &original.disclosure,
            evidence.action_key
        )),
        original.disclosure.idempotency_key
    );
    assert_eq!(
        original.sender().as_slice(),
        &evidence.owner_public_key[12..]
    );
    assert_eq!(evidence.rung, Rung::Receipted);
    assert!(matches!(
        evidence.verify_outcome(kind),
        Err(AgentFailure::Unavailable)
    ));
    assert!(matches!(
        ProductionAgentCreation::finalization_evidence(&evidence, ModuleId::Governance, 1, 1000),
        Err(AgentFailure::Refused("lifecycle receipt is not committed"))
    ));
    let mutations: &[fn(&mut ProtocolEvidence)] = &[
        |value| value.activity_id = value.action_key,
        |value| value.action_key = value.activity_id,
        |value| value.actor[0] ^= 1,
        |value| value.owner_public_key[0] ^= 1,
        |value| value.network_id += 1,
        |value| value.signed_activity[20] ^= 1,
    ];
    for mutate in mutations {
        let mut changed = evidence.clone();
        mutate(&mut changed);
        assert!(changed.bound_activity(kind).is_err());
        assert!(changed.verify_outcome(kind).is_err());
    }
    assert!(evidence
        .bound_activity(checked(ActivityType::new(ModuleId::Governance, 6)))
        .is_err());
}

use crate::require_value::RequireValue as _;

pub mod require_value {
    include!("support/require_value.rs");
}
