use layerx_agentd::prepare::evm::{
    prepare_bound, AbiArgument, EvmPreparationState, PrecompileCall, PrecompileTarget,
    PreparationDefaults, PrepareRequest,
};
use layerx_human_service::custody::{
    CustodySigner, EnvelopeKms, KeyClass, KeyEntropy, KeyId, Keystore, Operation,
    SignAuthorization, SignRequest, SigningLimits,
};
use layerx_human_service::trace::TraceId;
use layerx_human_test_support::{
    directory, install_and_open, principal, retention_uniform, tenancy,
};
use layerx_intents::{compile, Intent, IntentKind, KeyRotation};
use layerx_types::activity::TimestampBound;
use layerx_types::ids::Did;
use layerx_types::intent::{PublicKey, Sequence};
use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};
use std::future::Future;
use std::sync::Arc;
use std::task::{Context, Poll, Wake, Waker};

struct Ready;
impl Wake for Ready {
    fn wake(self: Arc<Self>) {}
}
fn ready<F: Future>(future: F) -> F::Output {
    let mut future = std::pin::pin!(future);
    match future
        .as_mut()
        .poll(&mut Context::from_waker(&Waker::from(Arc::new(Ready))))
    {
        Poll::Ready(result) => result,
        Poll::Pending => panic!("synchronous envelope custody did not complete"),
    }
}

// A positive integration regression: the current producer must admit the real
// compiled rotation before custody or any receipt can establish acceptance.
pub fn current_rotation_reaches_custody(label: &str) {
    let root = directory(label);
    std::fs::create_dir_all(&root).expect("rotation fixture directory");
    let secret = root.join("kms-root");
    std::fs::write(&secret, [0xa5; 64]).expect("envelope secret");
    let owner = principal("alice");
    let keys = Keystore::open_development(
        root.join("custody"),
        125,
        EnvelopeKms::new("file-kms://repair-rotation", &secret).expect("envelope provider"),
    )
    .expect("real keystore");
    let current = KeyId::new("current-primary").expect("current key");
    let pending = KeyId::new("pending-primary").expect("pending key");
    keys.generate(
        &owner,
        &current,
        KeyClass::HumanPrimary,
        KeyEntropy::new([0x21; 32], [1; 16], [2; 24]).expect("current entropy"),
    )
    .expect("current custody identity");
    let pending_public = keys
        .generate(
            &owner,
            &pending,
            KeyClass::HumanPrimary,
            KeyEntropy::new([0x22; 32], [3; 16], [4; 24]).expect("pending entropy"),
        )
        .expect("pending custody identity");
    let (mut store, _) = install_and_open(
        &root.join("human"),
        &tenancy(&[("alice", "tenant-a")]),
        retention_uniform(10_000),
    );
    let (sign_store, _) = install_and_open(
        &root.join("custody-audit"),
        &tenancy(&[("alice", "tenant-a")]),
        retention_uniform(10_000),
    );
    let kind = ActivityType::new(ModuleId::Governance, 2).expect("rotation type");
    let registry = ModuleRegistry::new(&[
        ModuleRegistration::new(ModuleId::Governance, &[kind]).expect("governance module")
    ])
    .expect("registry");
    let intent = Intent::v1(IntentKind::KeyRotation(
        KeyRotation::new(
            Did::new(b"did:layerx:rotation-owner").expect("owner DID"),
            PublicKey::new(pending_public),
            TimestampBound::new(1_060, 1_120).expect("rotation challenge"),
            Sequence::from_u64(50),
        )
        .expect("real rotation intent"),
    ));
    let compiled = compile(&intent, &registry).expect("production rotation compiler");
    compiled
        .verify_disclosure(&intent)
        .expect("intent disclosure");
    let payload = compiled.payload().as_bytes();
    assert_eq!(compiled.activity_type(), kind);
    assert_eq!(payload.len(), 92);
    let mut decoder = layerx_wire::decode::Decoder::new(payload, 0);
    assert_eq!(decoder.u16().expect("rotation tag"), 0x7102);
    assert_eq!(decoder.u16().expect("rotation field count"), 4);
    assert_eq!(
        decoder.fixed(32).expect("rotation owner"),
        layerx_wire::hash::did_id(&Did::new(b"did:layerx:rotation-owner").expect("owner DID"))
            .expect("production owner digest")
    );
    assert_eq!(decoder.fixed(32).expect("pending key"), pending_public);
    assert_eq!(decoder.u64().expect("challenge begins"), 1_060);
    assert_eq!(decoder.u64().expect("challenge ends"), 1_120);
    assert_eq!(decoder.u64().expect("effective sequence"), 50);
    decoder
        .finish()
        .expect("complete production rotation payload");
    let call = PrecompileCall {
        target: PrecompileTarget::Auth,
        selector: layerx_crypto::disclosure::call::ledger_selector(
            "recordLifecycle(bytes,bytes32)",
        ),
        arguments: vec![
            AbiArgument::Bytes(payload.to_vec()),
            AbiArgument::Word([0x63; 32]),
        ],
    };
    let calldata = call.encode().expect("production ABI encoder");
    let word = |value: u64| {
        let mut word = [0; 32];
        word[24..].copy_from_slice(&value.to_be_bytes());
        word
    };
    assert_eq!(calldata.len(), 196);
    assert_eq!(&calldata[..4], call.selector);
    assert_eq!(&calldata[4..36], &word(64));
    assert_eq!(&calldata[36..68], &[0x63; 32]);
    assert_eq!(&calldata[68..100], &word(92));
    assert_eq!(&calldata[100..192], payload);
    assert_eq!(&calldata[192..], &[0; 4]);
    let signer = CustodySigner::new(
        keys,
        sign_store,
        registry,
        SigningLimits::new(100, 10_000).expect("limits"),
    );
    let sender = signer
        .evm_wallet(&owner, &current)
        .expect("real current wallet");
    let preparation = prepare_bound(
        EvmPreparationState {
            chain_id: 125,
            pending_nonce: 7,
            base_fee_per_gas: 0,
            head_height: 42,
        },
        PreparationDefaults {
            max_priority_fee_per_gas: 1,
            max_fee_per_gas: 1,
            maximum_payload_bytes: 2_048,
        },
        PrepareRequest {
            from: sender,
            call,
            expected_nonce: Some(7),
            max_priority_fee_per_gas: Some(1),
            max_fee_per_gas: Some(1),
            declared_payload_limit: 2_048,
        },
        100_000,
    )
    .expect(
        "current production rotation preparation must be usable, without a synthetic transaction",
    );
    let unsigned = preparation.call.encode_unsigned();
    let trace = TraceId::mint([0x63; 16]);
    let mut scope = store.principal(&owner).expect("owner scope");
    let signed = ready(signer.sign_in_scope(
        &mut scope,
        SignRequest::new(
            &owner,
            &current,
            &trace,
            SignAuthorization::new(Operation::ProtocolMutation, None),
            &unsigned,
            &preparation.disclosure,
            1_000,
        ),
    ))
    .expect("actual custody rotation signature");
    // Use the public signer module used by the service; its shorter custody
    // re-export is crate-private. Signing above traverses sign_in_scope -> sign_call.
    let raw = layerx_human_service::custody::signer::attach_call_signature(
        &preparation.call,
        *signed.signature(),
        sender,
    )
    .expect("exact rotation transaction");
    let (decoded, signature) =
        layerx_crypto::evm_transaction::decode_signed(&raw).expect("decode custody transaction");
    assert_eq!(decoded, preparation.call);
    assert_eq!(
        layerx_crypto::evm_transaction::recover_sender(&decoded, &signature)
            .expect("rotation sender"),
        sender
    );
    drop(scope);
    drop(store);
    drop(signer);
    std::fs::remove_dir_all(root).expect("remove rotation fixture");
}
