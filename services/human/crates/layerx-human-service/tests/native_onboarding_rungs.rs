use ed25519_dalek::{Signer as _, SigningKey};
use layerx_agent_api::agent_api_schema_v1;
use layerx_agent_api::identity::{AgentDid, AuthorityRef};
use layerx_crypto::SignatureMessage;
use layerx_human_service::agents::ProtocolEvidence;
use layerx_human_service::custody::{EnvelopeKms, KeyClass, KeyEntropy, KeyId, Keystore};
use layerx_human_service::frame::{read_frame, write_frame, Limits};
use layerx_human_service::onboarding::{
    AgentPreparationContext, OnboardingError, OnboardingJourney, OnboardingStart, OnboardingState,
    ProtocolStage, RecoveryPolicy,
};
use layerx_human_service::server::agent_runtime::AgentRuntime;
use layerx_human_service::store::{PrincipalStore, TenancyDigest};
use layerx_human_service::trace::TraceId;
use layerx_human_test_support as support;
use layerx_intents::{canonical, owner_activity, NativeOwnerBootstrap};
use layerx_sdk::Client as AgentClient;
use layerx_types::ids::Did;
use layerx_types::intent::{ApprovalThreshold, PublicKey, RecoveryRoot};
use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};
use layerx_types::rung::Rung;
use sha2::{Digest as _, Sha256};
use std::fs;
use std::os::unix::net::UnixListener;
use std::thread;
use std::time::{Duration, Instant};
use support::{directory, install_and_open, principal, retention_uniform, tenancy};
const NETWORK_ID: u32 = 77;
const CHALLENGE_DELAY: u64 = 86_400;
const CORE_SEQUENCE: u64 = 7;

struct Fixture {
    root: std::path::PathBuf,
    store: PrincipalStore,
    digest: TenancyDigest,
    keystore: Keystore,
    agent: AgentClient,
    registry: ModuleRegistry,
    trace: TraceId,
}

impl Fixture {
    fn new(label: &str) -> Self {
        let root = directory(label);
        fs::create_dir_all(&root).unwrap_or_else(|error| panic!("fixture root: {error}"));
        let store_root = root.join("store");
        let custody_root = root.join("custody");
        let secret_path = root.join("kms-root");
        fs::write(&secret_path, [0x51_u8; 64])
            .unwrap_or_else(|error| panic!("KMS secret: {error}"));
        let map = tenancy(&[("alice", "tenant-alice")]);
        let (store, digest) = install_and_open(&store_root, &map, retention_uniform(10));
        let kms = EnvelopeKms::new("kms://onboarding-primary", secret_path.clone())
            .unwrap_or_else(|error| panic!("KMS: {error}"));
        let keystore = Keystore::open_development(&custody_root, NETWORK_ID, kms)
            .unwrap_or_else(|error| panic!("keystore: {error}"));
        keystore
            .generate(
                &principal("alice"),
                &KeyId::new("human-primary").unwrap(),
                KeyClass::HumanPrimary,
                KeyEntropy::new([0x68; 32], [0x31; 16], [0x41; 24]).unwrap(),
            )
            .unwrap();
        let did = ActivityType::new(ModuleId::Governance, 1)
            .unwrap_or_else(|error| panic!("DID activity: {error:?}"));
        let recovery = ActivityType::new(ModuleId::Governance, 3)
            .unwrap_or_else(|error| panic!("recovery activity: {error:?}"));
        let registration = ModuleRegistration::new(ModuleId::Governance, &[did, recovery])
            .unwrap_or_else(|error| panic!("governance registration: {error:?}"));
        let registry = ModuleRegistry::new(&[registration])
            .unwrap_or_else(|error| panic!("registry: {error:?}"));
        let agent = AgentClient::daemon("/run/layerx/agentd.sock", agent_api_schema_v1().version)
            .unwrap_or_else(|error| panic!("agent SDK: {error:?}"));
        Self {
            root,
            store,
            digest,
            keystore,
            agent,
            registry,
            trace: TraceId::mint([0x31; 16]),
        }
    }

    fn reopen_store(&self) -> PrincipalStore {
        PrincipalStore::open(self.root.join("store"), retention_uniform(10), self.digest)
            .unwrap_or_else(|error| panic!("reopen store: {error}"))
    }
}

impl Drop for Fixture {
    fn drop(&mut self) {
        let _ = fs::remove_dir_all(&self.root);
    }
}

fn start_request(key: u8) -> OnboardingStart {
    let recovery = RecoveryPolicy::new(
        RecoveryRoot::new([0x43; 32]),
        ApprovalThreshold::new(2).unwrap_or_else(|error| panic!("threshold: {error:?}")),
        CHALLENGE_DELAY,
    )
    .unwrap_or_else(|error| panic!("recovery policy: {error}"));
    OnboardingStart::new(
        [key; 32],
        Did::new(b"did:layerx:owner-receipt-fixture")
            .unwrap_or_else(|error| panic!("DID: {error:?}")),
        recovery,
    )
    .unwrap_or_else(|error| panic!("start request: {error}"))
}

fn context(sequence: u64, timestamp: u64) -> AgentPreparationContext {
    AgentPreparationContext::new(
        AgentDid::new("did:layerx:registrar").unwrap_or_else(|error| panic!("actor: {error:?}")),
        AuthorityRef::new("custody:human-primary")
            .unwrap_or_else(|error| panic!("authority: {error:?}")),
        sequence,
        timestamp,
        timestamp.saturating_add(30),
        10,
    )
    .unwrap_or_else(|error| panic!("agent context: {error}"))
}

#[derive(Clone)]
struct ReceiptFields {
    activity_id: [u8; 32],
    previous_state_root: [u8; 32],
    resulting_state_root: [u8; 32],
    batch_id: [u8; 32],
    asset: [u8; 32],
    operation: u8,
    module: u16,
}

fn encode_receipt(fields: &ReceiptFields, signature: Option<[u8; 64]>) -> Vec<u8> {
    let mut bytes = Vec::new();
    push_u16(
        &mut bytes,
        layerx_intents::canonical::STATE_COMMITMENT_PROTOCOL_VERSION,
    );
    push_u16(&mut bytes, 0x5201);
    push_u16(
        &mut bytes,
        layerx_intents::canonical::STATE_COMMITMENT_PROTOCOL_VERSION,
    );
    push_bytes(&mut bytes, &fields.activity_id);
    push_u64(&mut bytes, CORE_SEQUENCE);
    push_bytes(&mut bytes, &fields.previous_state_root);
    push_bytes(&mut bytes, &fields.resulting_state_root);
    push_bytes(&mut bytes, &[0x81; 32]);
    bytes.extend_from_slice(&0_i32.to_be_bytes());
    bytes.extend_from_slice(&0_u32.to_be_bytes());
    bytes.extend_from_slice(&1_u128.to_be_bytes());
    push_bytes(&mut bytes, &fields.batch_id);
    push_u16(&mut bytes, fields.module);
    bytes.extend_from_slice(&1_u32.to_be_bytes());
    bytes.extend_from_slice(&1_u32.to_be_bytes());
    bytes.push(fields.operation);
    push_bytes(&mut bytes, &fields.asset);
    bytes.extend_from_slice(&0_u128.to_be_bytes());
    push_bytes(&mut bytes, &[0; 32]);
    bytes.extend_from_slice(&0_u128.to_be_bytes());
    bytes.extend_from_slice(&0_u128.to_be_bytes());
    push_u64(&mut bytes, 0);
    push_bytes(&mut bytes, &[0; 32]);
    bytes.extend_from_slice(&0_u128.to_be_bytes());
    bytes.extend_from_slice(&0_u128.to_be_bytes());
    push_bytes(&mut bytes, &[0; 32]);
    push_bytes(&mut bytes, &[0; 32]);
    push_bytes(&mut bytes, &[0; 32]);
    push_u64(&mut bytes, 1_000);
    bytes.push(u8::from(signature.is_some()));
    if let Some(signature) = signature {
        push_bytes(&mut bytes, &signature);
    }
    bytes
}

fn push_u16(bytes: &mut Vec<u8>, value: u16) {
    bytes.extend_from_slice(&value.to_be_bytes());
}

fn push_u64(bytes: &mut Vec<u8>, value: u64) {
    bytes.extend_from_slice(&value.to_be_bytes());
}

fn push_bytes(output: &mut Vec<u8>, value: &[u8]) {
    let length = u32::try_from(value.len()).unwrap_or_else(|_| panic!("receipt field overflow"));
    output.extend_from_slice(&length.to_be_bytes());
    output.extend_from_slice(value);
}

fn frame(opcode: u8, body: &[u8]) -> Vec<u8> {
    let mut frame = b"LXHAGT01".to_vec();
    frame.push(opcode);
    frame.extend_from_slice(body);
    frame
}

fn setup_evidence(
    stage: ProtocolStage,
    action_key: [u8; 32],
    registry: &ModuleRegistry,
) -> ProtocolEvidence {
    let did = Did::new(b"did:layerx:owner-receipt-fixture").unwrap();
    let public_key = SigningKey::from_bytes(&[0x68; 32])
        .verifying_key()
        .to_bytes();
    let request = match stage {
        ProtocolStage::DidRegistration => NativeOwnerBootstrap::Identity {
            did,
            primary_key: PublicKey::new(public_key),
        },
        ProtocolStage::RecoveryRegistration => NativeOwnerBootstrap::RecoveryPolicy {
            did,
            root: RecoveryRoot::new([0x43; 32]),
            threshold: ApprovalThreshold::new(2).unwrap(),
            minimum_delay: CHALLENGE_DELAY,
            maximum_delay: CHALLENGE_DELAY,
        },
    };
    let compiled = request.compile(registry).unwrap();
    let (unsigned, _) = owner_activity::unsigned_native(
        &compiled,
        &owner_activity::OwnerEnvelopeContext {
            actor: Did::new(b"did:layerx:owner-receipt-fixture").unwrap(),
            owner_public_key: public_key,
            network_id: NETWORK_ID,
            account_sequence: 1,
            not_before_ms: 900,
            not_after_ms: 2000,
            action_key,
            fee_limit: 10,
        },
        registry,
    )
    .unwrap();
    let message = SignatureMessage::new(
        canonical::Domain::SignaturePreimage,
        3,
        NETWORK_ID,
        &unsigned,
    )
    .unwrap();
    let signature = SigningKey::from_bytes(&[0x68; 32])
        .sign(&message.digest())
        .to_bytes();
    let signed_activity =
        owner_activity::attach_signature(&unsigned, signature, public_key, registry).unwrap();
    let activity = owner_activity::verify(&signed_activity, registry).unwrap();
    let activity_id = canonical::activity_id(&activity).unwrap();
    let block_height = 42_u64;
    let mut batch_id = [0; 32];
    batch_id[24..].copy_from_slice(&block_height.to_be_bytes());
    let fields = ReceiptFields {
        activity_id,
        previous_state_root: [0x31; 32],
        resulting_state_root: [0x32; 32],
        batch_id,
        asset: [0; 32],
        operation: 0,
        module: ModuleId::Governance as u16,
    };
    let signer = SigningKey::from_bytes(&[0x51; 32]);
    let mut digest = Sha256::new();
    digest.update(b"LXP/v1/receipt\0");
    digest.update(encode_receipt(&fields, None));
    let signature = signer.sign(&<[u8; 32]>::from(digest.finalize())).to_bytes();
    let receipt_bytes = encode_receipt(&fields, Some(signature));
    let evidence = ProtocolEvidence {
        signed_activity,
        actor: activity.actor_did().to_vec(),
        owner_public_key: public_key,
        network_id: NETWORK_ID,
        action_key,
        activity_id,
        receipt_hash: Sha256::digest(&receipt_bytes).into(),
        receipt_bytes,
        tx_hash: activity_id,
        block_height,
        rung: Rung::Committed,
    };
    evidence.verify_outcome(compiled.activity_type()).unwrap();
    evidence
}

#[test]
fn native_onboarding_completes_only_at_committed() {
    let mut fixture = Fixture::new("native-onboarding-rungs");
    let mut scope = fixture.store.principal(&principal("alice")).unwrap();
    let mut journey = OnboardingJourney::start(&mut scope, &start_request(0x21), 100).unwrap();
    journey
        .resume_local(&mut scope, &fixture.keystore, 101)
        .unwrap();
    for stage in [
        ProtocolStage::DidRegistration,
        ProtocolStage::RecoveryRegistration,
    ] {
        let action = journey
            .prepare_agent_action(
                &mut scope,
                &fixture.registry,
                &fixture.agent,
                &context(3, 1000),
                102,
            )
            .unwrap()
            .unwrap();
        assert_eq!(action.stage(), stage);
        let mut evidence = setup_evidence(stage, action.action_key(), &fixture.registry);
        for rung in [Rung::Submitted, Rung::Receipted] {
            evidence.rung = rung;
            assert!(matches!(
                journey.accept_bootstrap_owner(
                    &mut scope,
                    stage,
                    &evidence,
                    &fixture.registry,
                    &fixture.trace,
                    103
                ),
                Err(OnboardingError::EvidenceConflict)
            ));
            assert_ne!(journey.status().state(), OnboardingState::Complete);
            let retained = OnboardingJourney::load(&scope).unwrap().unwrap();
            assert_ne!(retained.status().state(), OnboardingState::Complete);
        }
        evidence.rung = Rung::Committed;
        journey
            .accept_bootstrap_owner(
                &mut scope,
                stage,
                &evidence,
                &fixture.registry,
                &fixture.trace,
                104,
            )
            .unwrap();
        if stage == ProtocolStage::DidRegistration {
            assert_ne!(journey.status().state(), OnboardingState::Complete);
        }
    }
    assert_eq!(journey.status().state(), OnboardingState::Complete);
    assert_eq!(
        OnboardingJourney::load(&scope)
            .unwrap()
            .unwrap()
            .status()
            .state(),
        OnboardingState::Complete
    );
}

#[test]
fn native_onboarding_uses_the_pending_nonce() {
    let mut fixture = Fixture::new("native-onboarding-pending-nonce");
    let mut runtime = AgentRuntime::new(
        fixture.root.join("unavailable-agent.sock"),
        layerx_human_service::frame::Limits {
            maximum_frame_bytes: 65_536,
            maximum_connections: 1,
            maximum_streams: 1,
            maximum_queued_bytes: 65_536,
            deadline: Duration::from_secs(1),
        },
        fixture.registry.clone(),
    )
    .unwrap();
    let mut scope = fixture.store.principal(&principal("alice")).unwrap();
    let mut journey = OnboardingJourney::start(&mut scope, &start_request(0x22), 100).unwrap();
    journey
        .resume_local(&mut scope, &fixture.keystore, 101)
        .unwrap();
    assert!(matches!(
        journey.prepare_native_agent_action(
            &mut scope,
            &fixture.registry,
            &fixture.agent,
            &mut runtime,
            &context(99, 1000),
            102,
        ),
        Err(OnboardingError::InvalidAgentContext)
    ));
    let pending_nonce = 7_u64;
    let stale_nonce = 99;
    let mut nonce_request = Vec::new();
    push_bytes(&mut nonce_request, b"did:layerx:registrar");
    push_bytes(&mut nonce_request, b"custody:human-primary");
    let nonce_request = frame(13, &nonce_request);
    let mut nonce_answer = pending_nonce.to_be_bytes().to_vec();
    nonce_answer.push(Rung::Receipted.wire_rank());
    let nonce_answer = frame(0, &nonce_answer);
    let socket = fixture.root.join("peer.sock");
    let listener = UnixListener::bind(&socket).unwrap();
    listener.set_nonblocking(true).unwrap();
    let server = thread::spawn(move || {
        let deadline = Instant::now() + Duration::from_secs(5);
        let mut stream = loop {
            match listener.accept() {
                Ok((stream, _)) => break stream,
                Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                    assert!(Instant::now() < deadline, "peer request deadline");
                    thread::sleep(Duration::from_millis(1));
                }
                Err(error) => panic!("peer accept: {error}"),
            }
        };
        stream
            .set_read_timeout(Some(Duration::from_secs(5)))
            .unwrap();
        stream
            .set_write_timeout(Some(Duration::from_secs(5)))
            .unwrap();
        assert_eq!(read_frame(&mut stream, 4096).unwrap(), nonce_request);
        write_frame(&mut stream, &nonce_answer, 4096).unwrap();
    });
    let mut pending_runtime = AgentRuntime::new(
        &socket,
        Limits {
            maximum_frame_bytes: 4096,
            maximum_connections: 1,
            maximum_streams: 1,
            maximum_queued_bytes: 4096,
            deadline: Duration::from_secs(5),
        },
        fixture.registry.clone(),
    )
    .unwrap();
    let action = journey
        .prepare_native_agent_action(
            &mut scope,
            &fixture.registry,
            &fixture.agent,
            &mut pending_runtime,
            &context(stale_nonce, 1000),
            102,
        )
        .unwrap()
        .unwrap();
    server.join().unwrap();
    assert_eq!(
        action.call().request().operation.account_sequence.0,
        pending_nonce
    );
    assert_ne!(
        action.call().request().operation.account_sequence.0,
        stale_nonce
    );
    drop(journey);
    let mut journey = OnboardingJourney::load(&scope).unwrap().unwrap();
    let repeated = journey
        .prepare_native_agent_action(
            &mut scope,
            &fixture.registry,
            &fixture.agent,
            &mut runtime,
            &context(9, 1100),
            103,
        )
        .unwrap()
        .unwrap();
    assert_eq!(
        repeated.call().request().operation.account_sequence.0,
        pending_nonce
    );
    assert_eq!(repeated.call(), action.call());
}
