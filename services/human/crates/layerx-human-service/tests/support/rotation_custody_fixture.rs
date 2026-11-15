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

fn assert_rotation_payload(
    compiled: &layerx_intents::CompiledIntent,
    intent: &Intent,
    kind: ActivityType,
    pending_public: [u8; 32],
) {
    let payload = compiled.payload().as_bytes();
    compiled
        .verify_disclosure(intent)
        .or_fail("intent disclosure");
    assert_eq!(compiled.activity_type(), kind);
    assert_eq!(payload.len(), 92);
    let mut decoder = layerx_wire::decode::Decoder::new(payload, 0);
    assert_eq!(decoder.u16().or_fail("rotation tag"), 0x7102);
    assert_eq!(decoder.u16().or_fail("rotation field count"), 4);
    assert_eq!(
        decoder.fixed(32).or_fail("rotation owner"),
        layerx_wire::hash::did_id(&Did::new(b"did:layerx:rotation-owner").or_fail("owner DID"))
            .or_fail("production owner digest")
    );
    assert_eq!(decoder.fixed(32).or_fail("pending key"), pending_public);
    assert_eq!(decoder.u64().or_fail("challenge begins"), 1_060);
    assert_eq!(decoder.u64().or_fail("challenge ends"), 1_120);
    assert_eq!(decoder.u64().or_fail("effective sequence"), 50);
    decoder
        .finish()
        .or_fail("complete production rotation payload");
}

fn assert_rotation_calldata(call: &PrecompileCall, payload: &[u8]) {
    let calldata = call.encode().or_fail("production ABI encoder");
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
}

fn rotation_keys(
    root: &std::path::Path,
    secret: &std::path::Path,
) -> (
    layerx_human_service::store::PrincipalId,
    Keystore,
    KeyId,
    [u8; 32],
) {
    let owner = principal("alice");
    let keys = Keystore::open_development(
        root.join("custody"),
        125,
        EnvelopeKms::new("file-kms://repair-rotation", secret).or_fail("envelope provider"),
    )
    .or_fail("real keystore");
    let current = KeyId::new("current-primary").or_fail("current key");
    let pending = KeyId::new("pending-primary").or_fail("pending key");
    keys.generate(
        &owner,
        &current,
        KeyClass::HumanPrimary,
        KeyEntropy::new([0x21; 32], [1; 16], [2; 24]).or_fail("current entropy"),
    )
    .or_fail("current custody identity");
    let pending_public = keys
        .generate(
            &owner,
            &pending,
            KeyClass::HumanPrimary,
            KeyEntropy::new([0x22; 32], [3; 16], [4; 24]).or_fail("pending entropy"),
        )
        .or_fail("pending custody identity");
    (owner, keys, current, pending_public)
}

fn rotation_call(payload: &[u8]) -> PrecompileCall {
    PrecompileCall {
        target: PrecompileTarget::Auth,
        selector: layerx_crypto::disclosure::call::ledger_selector(
            "recordLifecycle(bytes,bytes32)",
        ),
        arguments: vec![
            AbiArgument::Bytes(payload.to_vec()),
            AbiArgument::Word([0x63; 32]),
        ],
    }
}

fn assert_rotation_signature(
    raw: &[u8],
    preparation: &layerx_agentd::prepare::EvmPrepared,
    sender: [u8; 20],
) {
    let (decoded_call, signature) =
        layerx_crypto::evm_transaction::decode_signed(raw).or_fail("decode custody transaction");
    assert_eq!(decoded_call, preparation.call);
    assert_eq!(
        layerx_crypto::evm_transaction::recover_sender(&decoded_call, &signature)
            .or_fail("rotation sender"),
        sender
    );
}

// The current producer admits the compiled rotation before custody signs it.
/// # Panics
/// Panics if rotation or custody evidence is unavailable.
pub fn current_rotation_reaches_custody(label: &str) {
    let root = directory(label);
    std::fs::create_dir_all(&root).or_fail("rotation fixture directory");
    let secret = root.join("kms-root");
    std::fs::write(&secret, [0xa5; 64]).or_fail("envelope secret");
    let (owner, keys, current, pending_public) = rotation_keys(&root, &secret);
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
    let kind = ActivityType::new(ModuleId::Governance, 2).or_fail("rotation type");
    let registry = ModuleRegistry::new(&[
        ModuleRegistration::new(ModuleId::Governance, &[kind]).or_fail("governance module")
    ])
    .or_fail("registry");
    let intent = Intent::v1(IntentKind::KeyRotation(
        KeyRotation::new(
            Did::new(b"did:layerx:rotation-owner").or_fail("owner DID"),
            PublicKey::new(pending_public),
            TimestampBound::new(1_060, 1_120).or_fail("rotation challenge"),
            Sequence::from_u64(50),
        )
        .or_fail("real rotation intent"),
    ));
    let compiled = compile(&intent, &registry).or_fail("production rotation compiler");
    assert_rotation_payload(&compiled, &intent, kind, pending_public);
    let payload = compiled.payload().as_bytes();
    let call = rotation_call(payload);
    assert_rotation_calldata(&call, payload);
    let signer = CustodySigner::new(
        keys,
        sign_store,
        registry,
        SigningLimits::new(100, 10_000).or_fail("limits"),
    );
    let sender = signer
        .evm_wallet(&owner, &current)
        .or_fail("real current wallet");
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
    .unwrap_or_else(|error| {
        panic!("{}: {error:?}",
        "current production rotation preparation must be usable, without a synthetic transaction",
    )
    });
    let unsigned = preparation.call.encode_unsigned();
    let trace = TraceId::mint([0x63; 16]);
    let mut scope = store.principal(&owner).or_fail("owner scope");
    let custody_signature = ready(signer.sign_in_scope(
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
    .or_fail("actual custody rotation signature");
    // Use the public signer module used by the service; its shorter custody
    // re-export is crate-private. Signing above traverses sign_in_scope -> sign_call.
    let raw = layerx_human_service::custody::signer::attach_call_signature(
        &preparation.call,
        *custody_signature.signature(),
        sender,
    )
    .or_fail("exact rotation transaction");
    assert_rotation_signature(&raw, &preparation, sender);
    drop(scope);
    drop(store);
    drop(signer);
    std::fs::remove_dir_all(root).or_fail("remove rotation fixture");
}

use crate::require_value::RequireValue as _;
