use layerx_crypto::disclosure::call::{ledger_selector, ConstructMethod};
use layerx_crypto::disclosure::DisclosureError;
use layerx_crypto::evm_transaction::Eip1559Call;
use layerx_crypto::payments::bind_call;
use layerx_crypto::signer::{LocalSigner, Signer as _};
use layerx_intents::NativeOwnerBootstrap;
use layerx_types::activity::TimestampBound;
use layerx_types::ids::Did;
use layerx_types::intent::{ApprovalThreshold, PublicKey, RecoveryRoot};
use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};

fn registry(ordinals: &[u16]) -> ModuleRegistry {
    let activities: Vec<_> = ordinals
        .iter()
        .map(|ordinal| {
            ActivityType::new(ModuleId::Governance, *ordinal)
                .unwrap_or_else(|error| panic!("native Governance ordinal: {error:?}"))
        })
        .collect();
    ModuleRegistry::new(&[ModuleRegistration::new(ModuleId::Governance, &activities)
        .unwrap_or_else(|error| panic!("native Governance registration: {error:?}"))])
    .unwrap_or_else(|error| panic!("native module registry: {error:?}"))
}

fn lifecycle_call(compiled: &layerx_intents::CompiledIntent) -> Eip1559Call {
    let payload = compiled.payload().as_bytes();
    let mut data = ledger_selector("recordLifecycle(bytes,bytes32)").to_vec();
    let mut word = [0; 32];
    word[31] = 64;
    data.extend_from_slice(&word);
    data.extend_from_slice(&[1; 32]);
    word.fill(0);
    word[24..].copy_from_slice(
        &u64::try_from(payload.len())
            .expect("payload length")
            .to_be_bytes(),
    );
    data.extend_from_slice(&word);
    data.extend_from_slice(payload);
    data.resize(4 + (data.len() - 4).div_ceil(32) * 32, 0);
    let mut to = [0; 20];
    to[18..].copy_from_slice(&[0x10, 0x1c]);
    Eip1559Call {
        chain_id: 125,
        nonce: 10,
        max_priority_fee_per_gas: 2,
        max_fee_per_gas: 30,
        gas_limit: 100_000,
        to,
        data,
    }
}

#[test]
fn native_identity_compiles_to_auth_lifecycle_call() -> Result<(), String> {
    let owner =
        Did::new(b"did:layerx:owner-bootstrap").map_err(|error| format!("owner DID: {error:?}"))?;
    let subject = layerx_wire::hash::did_id_for_protocol(&owner, 3)
        .map_err(|error| format!("subject: {error:?}"))?;
    let public = LocalSigner::new([71; 32]).public_key();
    let registry = registry(&[1]);
    let identity = NativeOwnerBootstrap::Identity {
        did: owner,
        primary_key: PublicKey::new(public),
    }
    .compile(&registry)
    .map_err(|error| format!("native identity: {error:?}"))?;
    let payload = identity.payload().as_bytes();
    assert_eq!(payload.len(), 68);
    assert_eq!(&payload[..4], &[0x71, 1, 0, 2]);
    assert_eq!(&payload[4..36], &subject);
    assert_eq!(&payload[36..], &public);
    let call = lifecycle_call(&identity);
    let from = [0x42; 20];
    let disclosure = bind_call(&call, from).map_err(|error| format!("disclosure: {error:?}"))?;
    assert_eq!(disclosure.activity_type, identity.activity_type());
    assert_eq!(disclosure.actor, from);
    assert_eq!(disclosure.authority, from);
    assert_eq!(disclosure.envelope_sequence(), call.nonce);
    assert_eq!(
        disclosure.fee_limit,
        u128::from(call.gas_limit) * call.max_fee_per_gas
    );
    assert_eq!(disclosure.idempotency_key, [1; 32]);
    assert_eq!(disclosure.canonical_payload(), call.data);
    assert_eq!(
        disclosure
            .signing_digest()
            .map_err(|error| format!("digest: {error:?}"))?,
        call.signing_hash()
    );
    assert_eq!(
        disclosure
            .construct_method()
            .map_err(|error| format!("method: {error:?}"))?,
        Some(ConstructMethod::AuthLifecycle {
            payload: identity.payload().as_bytes().to_vec(),
            idempotency_key: [1; 32],
        })
    );
    assert!(disclosure.amounts.is_empty());
    assert!(disclosure.counterparties.is_empty());
    assert_eq!(disclosure.asset, [0; 32]);
    for offset in [100, 101, 102, 103] {
        let mut malformed = call.clone();
        malformed.data[offset] = 0xff;
        assert_eq!(
            bind_call(&malformed, from),
            Err(DisclosureError::MalformedPayload)
        );
    }
    let mut invalid_offset = call.clone();
    invalid_offset.data[35] = 32;
    assert_eq!(
        bind_call(&invalid_offset, from),
        Err(DisclosureError::MalformedPayload)
    );
    let mut invalid_padding = call.clone();
    *invalid_padding.data.last_mut().expect("ABI padding") = 1;
    assert_eq!(
        bind_call(&invalid_padding, from),
        Err(DisclosureError::MalformedPayload)
    );
    let mut trailing = call;
    trailing.data.push(0);
    assert_eq!(
        bind_call(&trailing, from),
        Err(DisclosureError::MalformedPayload)
    );
    Ok(())
}

#[test]
fn native_bootstrap_refuses_missing_module_and_invalid_authority_policy() {
    let did =
        Did::new(b"did:layerx:owner-bootstrap").unwrap_or_else(|error| panic!("DID: {error:?}"));
    let key = PublicKey::new(LocalSigner::new([71; 32]).public_key());
    let identity = NativeOwnerBootstrap::Identity {
        did: did.clone(),
        primary_key: key,
    };
    assert!(identity.compile(&registry(&[2, 3])).is_err());
    assert!(NativeOwnerBootstrap::Identity {
        did: did.clone(),
        primary_key: PublicKey::new([0; 32])
    }
    .compile(&registry(&[1, 2, 3]))
    .is_err());
    assert!(NativeOwnerBootstrap::RotationPolicy {
        did: did.clone(),
        pending_key: key,
        challenge: TimestampBound::new(1, 2).unwrap_or_else(|error| panic!("challenge: {error:?}")),
        effective_sequence: 0
    }
    .compile(&registry(&[1, 2, 3]))
    .is_err());
    for (root, minimum_delay, maximum_delay) in
        [([0; 32], 30, 60), ([19; 32], 0, 60), ([19; 32], 60, 30)]
    {
        assert!(NativeOwnerBootstrap::RecoveryPolicy {
            did: did.clone(),
            root: RecoveryRoot::new(root),
            threshold: ApprovalThreshold::new(2)
                .unwrap_or_else(|error| panic!("threshold: {error:?}")),
            minimum_delay,
            maximum_delay
        }
        .compile(&registry(&[1, 2, 3]))
        .is_err());
    }
}
