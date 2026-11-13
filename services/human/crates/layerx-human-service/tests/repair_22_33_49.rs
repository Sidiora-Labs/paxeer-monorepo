pub mod canonical_owner {
    include!("support/canonical_owner.rs");
}

use ed25519_dalek::{Signer as _, SigningKey};
use layerx_agent_api::agent_api_schema_v1;
use layerx_agent_api::identity::{AgentDid, AuthorityRef};
use layerx_agent_api::track::{ReceiptRef, SubmissionRef, TrackedSubmission};
use layerx_agent_api::SubmissionState;
use layerx_agentd::human::{decode_request, encode_response, HumanRequest, HumanResponse};
use layerx_agentd::human_receipt_frame::{ReceiptLookupAnswer, ReceiptLookupFrame};
use layerx_client::sequencer::ReceiptKeyEntry;
use layerx_human_service::agents::ProtocolEvidence;
use layerx_human_service::audit::AuditChain;
use layerx_human_service::custody::{EnvelopeKms, KeyClass, KeyEntropy, KeyId, Keystore};
use layerx_human_service::frame::{read_frame, write_frame, Limits};
use layerx_human_service::journeys::ReceiptLookup;
use layerx_human_service::onboarding::{
    AgentPreparationContext, AgentPrepareAction, AgentStageOutcome, AgentStageUpdate, Dependency,
    OnboardingError, OnboardingJourney, OnboardingStage, OnboardingStart, OnboardingState,
    ProtocolStage, RecoveryPolicy, StageState,
};
use layerx_human_service::server::agent_creation::ProductionAgentCreation;
use layerx_human_service::server::agent_runtime::AgentRuntime;
use layerx_human_service::store::{PrincipalScope, PrincipalStore, RowKey, Table, TenancyDigest};
use layerx_human_service::trace::TraceId;
use layerx_human_test_support as support;
use layerx_intents::{canonical, NativeOwnerBootstrap};
use layerx_proof::receipt::{verify_sequencer_signature, AuthorizedBatch};
use layerx_sdk::Client as AgentClient;
use layerx_types::clock::Clock;
use layerx_types::ids::Did;
use layerx_types::intent::{ApprovalThreshold, PublicKey, RecoveryRoot};
use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};
use layerx_types::rung::Rung;
use layerx_types::verify::VerificationLevel;
use sha2::{Digest as _, Sha256};
use std::fs;
use std::os::unix::net::UnixListener;
use std::sync::Arc;
use std::thread;
use std::time::Duration;
use support::{directory, install_and_open, principal, retention_uniform, tenancy};

const NETWORK_ID: u32 = 77;
const CHALLENGE_DELAY: u64 = 86_400;
const HEIGHT: u64 = 42;

struct Fixture {
    root: std::path::PathBuf,
    store: Option<PrincipalStore>,
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
                &KeyId::new("human-primary")
                    .unwrap_or_else(|error| panic!("unexpected error: {error:?}")),
                KeyClass::HumanPrimary,
                KeyEntropy::new([0x68; 32], [0x31; 16], [0x41; 24])
                    .unwrap_or_else(|error| panic!("unexpected error: {error:?}")),
            )
            .unwrap_or_else(|error| panic!("unexpected error: {error:?}"));
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
            store: Some(store),
            digest,
            keystore,
            agent,
            registry,
            trace: TraceId::mint([0x31; 16]),
        }
    }

    fn restart(&mut self) {
        drop(self.store.take().unwrap_or_else(|| panic!("missing value")));
        self.store = Some(
            PrincipalStore::open(self.root.join("store"), retention_uniform(10), self.digest)
                .unwrap_or_else(|error| panic!("unexpected error: {error:?}")),
        );
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
        sequence,
        timestamp,
        timestamp.saturating_add(30),
        10,
    )
    .unwrap_or_else(|error| panic!("agent context: {error}"))
}

fn runtime(socket: &std::path::Path, registry: &ModuleRegistry) -> AgentRuntime {
    let clock: Arc<dyn Clock> = Arc::new(support::StepClock::new(100_000));
    AgentRuntime::new_with_clock(
        socket,
        Limits {
            maximum_frame_bytes: 65_536,
            maximum_connections: 1,
            maximum_streams: 1,
            maximum_queued_bytes: 65_536,
            deadline: Duration::from_secs(5),
        },
        registry.clone(),
        clock,
    )
    .unwrap_or_else(|error| panic!("unexpected error: {error:?}"))
}

fn peer(
    socket: &std::path::Path,
    response: HumanResponse,
    check: impl FnOnce(HumanRequest) + Send + 'static,
) -> thread::JoinHandle<()> {
    let listener =
        UnixListener::bind(socket).unwrap_or_else(|error| panic!("unexpected error: {error:?}"));
    listener
        .set_nonblocking(true)
        .unwrap_or_else(|error| panic!("unexpected error: {error:?}"));
    thread::spawn(move || {
        let mut accept_steps = 0..10_000;
        let mut stream = loop {
            match listener.accept() {
                Ok((stream, _)) => break stream,
                Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                    assert!(accept_steps.next().is_some(), "peer accept deadline");
                    thread::sleep(Duration::from_millis(1));
                }
                Err(error) => panic!("accept: {error}"),
            }
        };
        stream
            .set_read_timeout(Some(Duration::from_secs(5)))
            .unwrap_or_else(|error| panic!("unexpected error: {error:?}"));
        stream
            .set_write_timeout(Some(Duration::from_secs(5)))
            .unwrap_or_else(|error| panic!("unexpected error: {error:?}"));
        check(
            decode_request(
                read_frame(&mut stream, 65_536)
                    .unwrap_or_else(|error| panic!("unexpected error: {error:?}")),
            )
            .unwrap_or_else(|error| panic!("unexpected error: {error:?}")),
        );
        write_frame(&mut stream, &encode_response(Ok(response)), 65_536)
            .unwrap_or_else(|error| panic!("unexpected error: {error:?}"));
    })
}

fn receipt_bytes(id: [u8; 32], result: i32, signature: Option<[u8; 64]>) -> Vec<u8> {
    layerx_intents::vectors::onboarding_receipt(id, result, HEIGHT, signature)
        .unwrap_or_else(|error| panic!("unexpected error: {error:?}"))
}

fn lookup_frame(
    signed_receipt_bytes: Vec<u8>,
    protocol: &layerx_wire::receipt::ProtocolReceipt,
    signer: &SigningKey,
    owner_id: [u8; 32],
    rung: Rung,
) -> ReceiptLookupFrame {
    ReceiptLookupFrame {
        canonical_bytes: signed_receipt_bytes.clone(),
        authorised_batch: AuthorizedBatch::new(
            protocol.batch_id(),
            protocol.asset(),
            protocol.previous_state_root(),
            protocol.resulting_state_root(),
            signer.verifying_key().to_bytes(),
        ),
        verification_level: VerificationLevel::SEQUENCER_SIGNED,
        tx_hash: owner_id,
        receipt_hash: Sha256::digest(&signed_receipt_bytes).into(),
        block_height: HEIGHT,
        rung,
        signed_receipt: signed_receipt_bytes,
        receipt_key_id: [0x91; 32],
        public_key: signer.verifying_key().to_bytes(),
        first_height: HEIGHT,
        last_height: HEIGHT,
        inclusion_height: HEIGHT,
    }
}

fn owner_bootstrap(action: &AgentPrepareAction) -> NativeOwnerBootstrap {
    let did = Did::new(canonical_owner::OWNER_DID.as_bytes())
        .unwrap_or_else(|error| panic!("unexpected error: {error:?}"));
    match action.stage() {
        ProtocolStage::DidRegistration => NativeOwnerBootstrap::Identity {
            did,
            primary_key: PublicKey::new(
                SigningKey::from_bytes(&[0x68; 32])
                    .verifying_key()
                    .to_bytes(),
            ),
        },
        ProtocolStage::RecoveryRegistration => NativeOwnerBootstrap::RecoveryPolicy {
            did,
            root: RecoveryRoot::new([0x43; 32]),
            threshold: ApprovalThreshold::new(2)
                .unwrap_or_else(|error| panic!("unexpected error: {error:?}")),
            minimum_delay: CHALLENGE_DELAY,
            maximum_delay: CHALLENGE_DELAY,
        },
    }
}

fn setup_evidence(
    root: &std::path::Path,
    action: &AgentPrepareAction,
    registry: &ModuleRegistry,
    result: i32,
    rung: Rung,
) -> ProtocolEvidence {
    let request = owner_bootstrap(action);
    let compiled = request
        .compile(registry)
        .unwrap_or_else(|error| panic!("unexpected error: {error:?}"));
    let owner = canonical_owner::sign_transaction(
        compiled.payload(),
        action.action_key(),
        canonical::STATE_COMMITMENT_PROTOCOL_VERSION,
        NETWORK_ID,
    );
    let signer = SigningKey::from_bytes(&[0x51; 32]);
    let unsigned = receipt_bytes(owner.id, result, None);
    let digest = layerx_wire::hash::receipt_digest(&unsigned)
        .unwrap_or_else(|error| panic!("unexpected error: {error:?}"));
    let signed_receipt_bytes =
        receipt_bytes(owner.id, result, Some(signer.sign(&digest).to_bytes()));
    let authenticated =
        verify_sequencer_signature(&signed_receipt_bytes, signer.verifying_key().to_bytes())
            .unwrap_or_else(|error| panic!("unexpected error: {error:?}"));
    assert_eq!(
        authenticated
            .protocol()
            .unwrap_or_else(|| panic!("missing value"))
            .result_code(),
        result
    );
    let protocol = authenticated
        .protocol()
        .unwrap_or_else(|| panic!("missing value"));
    let frame = lookup_frame(signed_receipt_bytes, protocol, &signer, owner.id, rung);
    let socket = root.join("evidence.sock");
    let action_key = action.action_key();
    let activity_id = owner.id;
    let server = peer(
        &socket,
        HumanResponse::new(
            ReceiptLookupAnswer::Found(frame)
                .encode()
                .unwrap_or_else(|error| panic!("unexpected error: {error:?}")),
        )
        .unwrap_or_else(|error| panic!("unexpected error: {error:?}")),
        move |request| match request {
            HumanRequest::Evidence {
                idempotency_key,
                tx_hash,
            } => {
                assert_eq!(idempotency_key, action_key);
                assert_eq!(tx_hash, activity_id);
            }
            _ => panic!("unexpected evidence request"),
        },
    );
    let mut agent = runtime(&socket, registry);
    let ReceiptLookup::Found(material) = agent
        .evidence(action_key, owner.id)
        .unwrap_or_else(|error| panic!("unexpected error: {error:?}"))
    else {
        panic!("receipt material missing");
    };
    server
        .join()
        .unwrap_or_else(|error| panic!("unexpected error: {error:?}"));
    fs::remove_file(socket).unwrap_or_else(|error| panic!("unexpected error: {error:?}"));
    let evidence = ProductionAgentCreation::protocol_evidence(
        owner.bytes,
        owner.public_key,
        registry,
        material,
    )
    .unwrap_or_else(|error| panic!("unexpected error: {error:?}"));
    assert_eq!(evidence.rung, rung);
    assert_eq!(evidence.action_key, action_key);
    if rung == Rung::Committed {
        assert_eq!(
            evidence
                .verify_outcome(compiled.activity_type())
                .unwrap_or_else(|error| panic!("unexpected error: {error:?}"))
                .protocol()
                .unwrap_or_else(|| panic!("missing value"))
                .result_code(),
            result
        );
    }
    evidence
}

fn record(scope: &PrincipalScope<'_>, stage: ProtocolStage) -> serde_json::Value {
    let row = scope
        .get(
            Table::Journeys,
            &RowKey::new("onboarding-journey")
                .unwrap_or_else(|error| panic!("unexpected error: {error:?}")),
        )
        .unwrap_or_else(|| panic!("missing value"));
    let record: serde_json::Value = serde_json::from_slice(row.bytes())
        .unwrap_or_else(|error| panic!("unexpected error: {error:?}"));
    record[match stage {
        ProtocolStage::DidRegistration => "did_registration",
        ProtocolStage::RecoveryRegistration => "recovery_registration",
    }]
    .clone()
}

fn submission(reference: String, state: SubmissionState) -> TrackedSubmission {
    TrackedSubmission {
        submission_ref: SubmissionRef::new(reference)
            .unwrap_or_else(|error| panic!("unexpected error: {error:?}")),
        state,
        evidence: Vec::new(),
        rung: Rung::Submitted,
        transitions: Vec::new(),
    }
}

fn reference(evidence: &ProtocolEvidence) -> String {
    layerx_human_service::hex::lower((evidence.activity_id).as_ref())
}

fn complete_onboarding(
    root: &std::path::Path,
    registry: &ModuleRegistry,
    agent: &AgentClient,
    trace: &TraceId,
    journey: &mut OnboardingJourney,
    scope: &mut PrincipalScope<'_>,
) -> Vec<(AgentPrepareAction, ProtocolEvidence)> {
    let mut completed = Vec::new();
    for stage in [
        ProtocolStage::DidRegistration,
        ProtocolStage::RecoveryRegistration,
    ] {
        let action = journey
            .prepare_agent_action(scope, registry, agent, &context(3, 1000), 102)
            .unwrap_or_else(|error| panic!("unexpected error: {error:?}"))
            .unwrap_or_else(|| panic!("missing value"));
        assert_eq!(action.stage(), stage);
        let evidence = setup_evidence(root, &action, registry, 0, Rung::Committed);
        journey
            .accept_bootstrap_owner(scope, stage, &evidence, registry, trace, 103)
            .unwrap_or_else(|error| panic!("unexpected error: {error:?}"));
        completed.push((action, evidence));
    }
    completed
}

fn assert_delayed_updates(
    journey: &mut OnboardingJourney,
    scope: &mut PrincipalScope<'_>,
    audit: &mut AuditChain,
    trace: &TraceId,
    completed: &[(AgentPrepareAction, ProtocolEvidence)],
) {
    for (action, evidence) in completed {
        let durable = record(scope, action.stage());
        for state in [
            SubmissionState::Submitted,
            SubmissionState::Unknown,
            SubmissionState::Expired,
        ] {
            let submission = submission(reference(evidence), state);
            journey
                .apply_agent_update(
                    scope,
                    audit,
                    trace,
                    &AgentStageUpdate {
                        stage: action.stage(),
                        action_key: action.action_key(),
                        outcome: AgentStageOutcome::Tracked {
                            submission: &submission,
                        },
                    },
                    104,
                )
                .unwrap_or_else(|error| panic!("unexpected error: {error:?}"));
            assert_eq!(journey.status().state(), OnboardingState::Complete);
            assert_eq!(record(scope, action.stage()), durable);
        }
        journey
            .apply_agent_update(
                scope,
                audit,
                trace,
                &AgentStageUpdate {
                    stage: action.stage(),
                    action_key: action.action_key(),
                    outcome: AgentStageOutcome::Unavailable {
                        dependency: Dependency::AgentLayer,
                    },
                },
                105,
            )
            .unwrap_or_else(|error| panic!("unexpected error: {error:?}"));
        let conflict = submission("different-submission".into(), SubmissionState::Unknown);
        assert!(matches!(
            journey.apply_agent_update(
                scope,
                audit,
                trace,
                &AgentStageUpdate {
                    stage: action.stage(),
                    action_key: action.action_key(),
                    outcome: AgentStageOutcome::Tracked {
                        submission: &conflict
                    },
                },
                106
            ),
            Err(OnboardingError::SubmissionConflict)
        ));
    }
}

#[test]
fn finding_10_completed_journey_ignores_delayed_tracking_after_restart() {
    let mut fixture = Fixture::new("repair-22-33-49-delayed");
    let mut scope = fixture
        .store
        .as_mut()
        .unwrap_or_else(|| panic!("missing value"))
        .principal(&principal("alice"))
        .unwrap_or_else(|error| panic!("unexpected error: {error:?}"));
    let mut journey = OnboardingJourney::start(&mut scope, &start_request(0x21), 100)
        .unwrap_or_else(|error| panic!("unexpected error: {error:?}"));
    journey
        .resume_local(&mut scope, &fixture.keystore, 101)
        .unwrap_or_else(|error| panic!("unexpected error: {error:?}"));
    let completed = complete_onboarding(
        &fixture.root,
        &fixture.registry,
        &fixture.agent,
        &fixture.trace,
        &mut journey,
        &mut scope,
    );
    assert_eq!(journey.status().state(), OnboardingState::Complete);
    let activation = journey
        .activation_evidence(&scope)
        .unwrap_or_else(|error| panic!("unexpected error: {error:?}"));
    assert_eq!(activation.len(), 3);
    let mut audit =
        AuditChain::open(&scope).unwrap_or_else(|error| panic!("unexpected error: {error:?}"));
    assert_eq!(
        audit
            .entries(&scope)
            .unwrap_or_else(|error| panic!("unexpected error: {error:?}"))
            .len(),
        3
    );
    assert_delayed_updates(
        &mut journey,
        &mut scope,
        &mut audit,
        &fixture.trace,
        &completed,
    );
    drop(scope);
    drop(journey);
    fixture.restart();
    let scope = fixture
        .store
        .as_mut()
        .unwrap_or_else(|| panic!("missing value"))
        .principal(&principal("alice"))
        .unwrap_or_else(|error| panic!("unexpected error: {error:?}"));
    let journey = OnboardingJourney::load(&scope)
        .unwrap_or_else(|error| panic!("unexpected error: {error:?}"))
        .unwrap_or_else(|| panic!("missing value"));
    assert_eq!(journey.status().state(), OnboardingState::Complete);
    assert!(journey.status().account_active());
    assert_eq!(
        journey
            .activation_evidence(&scope)
            .unwrap_or_else(|error| panic!("unexpected error: {error:?}")),
        activation
    );
    assert_eq!(
        AuditChain::open(&scope)
            .unwrap_or_else(|error| panic!("unexpected error: {error:?}"))
            .entries(&scope)
            .unwrap_or_else(|error| panic!("unexpected error: {error:?}"))
            .len(),
        3
    );
    for stage in [
        ProtocolStage::DidRegistration,
        ProtocolStage::RecoveryRegistration,
    ] {
        assert_eq!(
            record(&scope, stage)["evidence_rung"],
            Rung::Committed.wire_rank()
        );
    }
}

fn assert_receipted_rejected(
    journey: &mut OnboardingJourney,
    scope: &mut PrincipalScope<'_>,
    stage: ProtocolStage,
    action: &AgentPrepareAction,
    evidence: &ProtocolEvidence,
    registry: &ModuleRegistry,
    trace: &TraceId,
) -> layerx_human_service::onboarding::OnboardingStatus {
    let before = journey.status();
    let audit_count = AuditChain::open(scope)
        .unwrap_or_else(|error| panic!("unexpected error: {error:?}"))
        .entries(scope)
        .unwrap_or_else(|error| panic!("unexpected error: {error:?}"))
        .len();
    let mut audit =
        AuditChain::open(scope).unwrap_or_else(|error| panic!("unexpected error: {error:?}"));
    let tracked = submission(
        reference(evidence),
        SubmissionState::Executed {
            receipt_ref: ReceiptRef::new("receipt")
                .unwrap_or_else(|error| panic!("unexpected error: {error:?}")),
        },
    );
    let receipt_key = ReceiptKeyEntry {
        key_id: [0x91; 32],
        public_key: SigningKey::from_bytes(&[0x51; 32])
            .verifying_key()
            .to_bytes(),
        first_height: HEIGHT,
        last_height: HEIGHT,
    };
    assert!(matches!(
        journey.apply_agent_update(
            scope,
            &mut audit,
            trace,
            &AgentStageUpdate {
                stage,
                action_key: action.action_key(),
                outcome: AgentStageOutcome::Executed {
                    submission: &tracked,
                    activity_id: evidence.activity_id,
                    receipt_bytes: &evidence.receipt_bytes,
                    tx_hash: evidence.tx_hash,
                    rung: evidence.rung,
                    receipt_key: &receipt_key,
                },
            },
            103
        ),
        Err(OnboardingError::NativeOutcomeRequired)
    ));
    assert!(matches!(
        journey.accept_bootstrap_owner(scope, stage, evidence, registry, trace, 104),
        Err(OnboardingError::EvidenceConflict)
    ));
    assert_eq!(journey.status(), before);
    assert_eq!(
        AuditChain::open(scope)
            .unwrap_or_else(|error| panic!("unexpected error: {error:?}"))
            .entries(scope)
            .unwrap_or_else(|error| panic!("unexpected error: {error:?}"))
            .len(),
        audit_count
    );
    assert!(record(scope, stage)["receipt_digest"].is_null());
    before
}

fn assert_completed_registration(fixture: &mut Fixture) {
    let scope = fixture
        .store
        .as_mut()
        .unwrap_or_else(|| panic!("missing value"))
        .principal(&principal("alice"))
        .unwrap_or_else(|error| panic!("unexpected error: {error:?}"));
    assert_eq!(
        OnboardingJourney::load(&scope)
            .unwrap_or_else(|error| panic!("unexpected error: {error:?}"))
            .unwrap_or_else(|| panic!("missing value"))
            .status()
            .state(),
        OnboardingState::Complete
    );
}

#[test]
fn finding_8_receipt_only_adapter_is_retired_and_native_activation_requires_commit() {
    let mut fixture = Fixture::new("repair-22-33-49-rungs");
    let mut scope = fixture
        .store
        .as_mut()
        .unwrap_or_else(|| panic!("missing value"))
        .principal(&principal("alice"))
        .unwrap_or_else(|error| panic!("unexpected error: {error:?}"));
    let mut journey = OnboardingJourney::start(&mut scope, &start_request(0x22), 100)
        .unwrap_or_else(|error| panic!("unexpected error: {error:?}"));
    journey
        .resume_local(&mut scope, &fixture.keystore, 101)
        .unwrap_or_else(|error| panic!("unexpected error: {error:?}"));
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
            .unwrap_or_else(|error| panic!("unexpected error: {error:?}"))
            .unwrap_or_else(|| panic!("missing value"));
        assert_eq!(action.stage(), stage);
        let evidence = setup_evidence(
            &fixture.root,
            &action,
            &fixture.registry,
            0,
            Rung::Receipted,
        );
        let before = assert_receipted_rejected(
            &mut journey,
            &mut scope,
            stage,
            &action,
            &evidence,
            &fixture.registry,
            &fixture.trace,
        );
        drop(scope);
        drop(journey);
        fixture.restart();
        scope = fixture
            .store
            .as_mut()
            .unwrap_or_else(|| panic!("missing value"))
            .principal(&principal("alice"))
            .unwrap_or_else(|error| panic!("unexpected error: {error:?}"));
        journey = OnboardingJourney::load(&scope)
            .unwrap_or_else(|error| panic!("unexpected error: {error:?}"))
            .unwrap_or_else(|| panic!("missing value"));
        assert_eq!(journey.status(), before);
        let evidence = setup_evidence(
            &fixture.root,
            &action,
            &fixture.registry,
            0,
            Rung::Committed,
        );
        journey
            .accept_bootstrap_owner(
                &mut scope,
                stage,
                &evidence,
                &fixture.registry,
                &fixture.trace,
                105,
            )
            .unwrap_or_else(|error| panic!("unexpected error: {error:?}"));
        assert_eq!(
            record(&scope, stage)["evidence_rung"],
            Rung::Committed.wire_rank()
        );
        assert_eq!(
            AuditChain::open(&scope)
                .unwrap_or_else(|error| panic!("unexpected error: {error:?}"))
                .entries(&scope)
                .unwrap_or_else(|error| panic!("unexpected error: {error:?}"))
                .len(),
            if stage == ProtocolStage::DidRegistration {
                2
            } else {
                3
            }
        );
    }
    assert_eq!(journey.status().state(), OnboardingState::Complete);
    drop(scope);
    drop(journey);
    fixture.restart();
    assert_completed_registration(&mut fixture);
}

fn fail_registration(
    sources: (&std::path::Path, &ModuleRegistry, &AgentClient, &TraceId),
    journey: &mut OnboardingJourney,
    scope: &mut PrincipalScope<'_>,
    failed_stage: ProtocolStage,
    failure: i32,
) -> (AgentPrepareAction, ProtocolEvidence) {
    let (root, registry, agent, trace) = sources;
    let mut failed_action = None;
    for stage in [
        ProtocolStage::DidRegistration,
        ProtocolStage::RecoveryRegistration,
    ] {
        let action = journey
            .prepare_agent_action(scope, registry, agent, &context(3, 1000), 102)
            .unwrap_or_else(|error| panic!("unexpected error: {error:?}"))
            .unwrap_or_else(|| panic!("missing value"));
        let result = if stage == failed_stage { failure } else { 0 };
        let evidence = setup_evidence(root, &action, registry, result, Rung::Committed);
        journey
            .accept_bootstrap_owner(scope, stage, &evidence, registry, trace, 103)
            .unwrap_or_else(|error| panic!("unexpected error: {error:?}"));
        if stage == failed_stage {
            failed_action = Some((action, evidence));
            break;
        }
    }
    let (action, evidence) = failed_action.unwrap_or_else(|| panic!("missing value"));
    (action, evidence)
}

fn assert_refused_before_restart(
    journey: &mut OnboardingJourney,
    scope: &mut PrincipalScope<'_>,
    failed_stage: ProtocolStage,
    action: &AgentPrepareAction,
    evidence: &ProtocolEvidence,
    failure: i32,
    context: (&ModuleRegistry, &TraceId),
) -> (serde_json::Value, bool, usize) {
    let (registry, trace) = context;
    assert_eq!(journey.status().state(), OnboardingState::Refused);
    let active = failed_stage == ProtocolStage::RecoveryRegistration;
    assert_eq!(journey.status().account_active(), active);
    let expected_events = if active { 2 } else { 0 };
    assert_eq!(
        AuditChain::open(scope)
            .unwrap_or_else(|error| panic!("unexpected error: {error:?}"))
            .entries(scope)
            .unwrap_or_else(|error| panic!("unexpected error: {error:?}"))
            .len(),
        expected_events
    );
    let failed_record = record(scope, failed_stage);
    assert_eq!(failed_record["refusal_code"], failure);
    assert_eq!(failed_record["evidence_rung"], Rung::Committed.wire_rank());
    assert!(failed_record["receipt_digest"].is_array());
    let stage_status = journey
        .status()
        .stages()
        .iter()
        .find(|status| {
            status.stage()
                == if active {
                    OnboardingStage::RecoveryRegistration
                } else {
                    OnboardingStage::DidRegistration
                }
        })
        .unwrap_or_else(|| panic!("missing value"))
        .state();
    assert_eq!(
        stage_status,
        StageState::Refused {
            result_code: Some(failure)
        }
    );
    let mut audit =
        AuditChain::open(scope).unwrap_or_else(|error| panic!("unexpected error: {error:?}"));
    let delayed = submission(reference(evidence), SubmissionState::Submitted);
    journey
        .apply_agent_update(
            scope,
            &mut audit,
            trace,
            &AgentStageUpdate {
                stage: failed_stage,
                action_key: action.action_key(),
                outcome: AgentStageOutcome::Tracked {
                    submission: &delayed,
                },
            },
            104,
        )
        .unwrap_or_else(|error| panic!("unexpected error: {error:?}"));
    assert_eq!(record(scope, failed_stage), failed_record);
    journey
        .accept_bootstrap_owner(scope, failed_stage, evidence, registry, trace, 105)
        .unwrap_or_else(|error| panic!("unexpected error: {error:?}"));
    (failed_record, active, expected_events)
}

#[test]
fn finding_9_failed_did_and_recovery_registrations_stay_refused_after_restart() {
    for failed_stage in [
        ProtocolStage::DidRegistration,
        ProtocolStage::RecoveryRegistration,
    ] {
        let mut fixture = Fixture::new("repair-22-33-49-refused");
        let mut scope = fixture
            .store
            .as_mut()
            .unwrap_or_else(|| panic!("missing value"))
            .principal(&principal("alice"))
            .unwrap_or_else(|error| panic!("unexpected error: {error:?}"));
        let mut journey = OnboardingJourney::start(&mut scope, &start_request(0x23), 100)
            .unwrap_or_else(|error| panic!("unexpected error: {error:?}"));
        journey
            .resume_local(&mut scope, &fixture.keystore, 101)
            .unwrap_or_else(|error| panic!("unexpected error: {error:?}"));
        let failure = -32001;
        let (action, evidence) = fail_registration(
            (
                &fixture.root,
                &fixture.registry,
                &fixture.agent,
                &fixture.trace,
            ),
            &mut journey,
            &mut scope,
            failed_stage,
            failure,
        );
        let (failed_record, active, expected_events) = assert_refused_before_restart(
            &mut journey,
            &mut scope,
            failed_stage,
            &action,
            &evidence,
            failure,
            (&fixture.registry, &fixture.trace),
        );
        drop(scope);
        drop(journey);
        fixture.restart();
        let mut scope = fixture
            .store
            .as_mut()
            .unwrap_or_else(|| panic!("missing value"))
            .principal(&principal("alice"))
            .unwrap_or_else(|error| panic!("unexpected error: {error:?}"));
        let mut journey = OnboardingJourney::load(&scope)
            .unwrap_or_else(|error| panic!("unexpected error: {error:?}"))
            .unwrap_or_else(|| panic!("missing value"));
        assert_eq!(journey.status().state(), OnboardingState::Refused);
        assert_eq!(record(&scope, failed_stage), failed_record);
        assert_eq!(journey.status().account_active(), active);
        assert_eq!(
            journey
                .activation_evidence(&scope)
                .unwrap_or_else(|error| panic!("unexpected error: {error:?}"))
                .len(),
            expected_events
        );
        assert_eq!(
            AuditChain::open(&scope)
                .unwrap_or_else(|error| panic!("unexpected error: {error:?}"))
                .entries(&scope)
                .unwrap_or_else(|error| panic!("unexpected error: {error:?}"))
                .len(),
            expected_events
        );
        let changed = setup_evidence(
            &fixture.root,
            &action,
            &fixture.registry,
            0,
            Rung::Committed,
        );
        assert!(matches!(
            journey.accept_bootstrap_owner(
                &mut scope,
                failed_stage,
                &changed,
                &fixture.registry,
                &fixture.trace,
                106
            ),
            Err(OnboardingError::EvidenceConflict)
        ));
        assert_eq!(record(&scope, failed_stage), failed_record);
    }
}

#[test]
fn finding_11_runtime_nonce_matches_both_durable_preparation_fields_after_restart() {
    let mut fixture = Fixture::new("repair-22-33-49-nonce");
    let socket = fixture.root.join("nonce.sock");
    let pending_nonce = 17;
    let mut payload = HumanResponse::pending_nonce(pending_nonce).bytes().to_vec();
    payload.push(Rung::Receipted.wire_rank());
    let server = peer(
        &socket,
        HumanResponse::new(payload).unwrap_or_else(|error| panic!("unexpected error: {error:?}")),
        |request| match request {
            HumanRequest::AccountSequence { actor, authority } => {
                assert_eq!(actor, "did:layerx:registrar");
                assert_eq!(authority, "custody:human-primary");
            }
            _ => panic!("unexpected nonce request"),
        },
    );
    let mut agent_runtime = runtime(&socket, &fixture.registry);
    let mut scope = fixture
        .store
        .as_mut()
        .unwrap_or_else(|| panic!("missing value"))
        .principal(&principal("alice"))
        .unwrap_or_else(|error| panic!("unexpected error: {error:?}"));
    let mut journey = OnboardingJourney::start(&mut scope, &start_request(0x24), 100)
        .unwrap_or_else(|error| panic!("unexpected error: {error:?}"));
    journey
        .resume_local(&mut scope, &fixture.keystore, 101)
        .unwrap_or_else(|error| panic!("unexpected error: {error:?}"));
    let stale = AgentPreparationContext::new(
        AgentDid::new("did:layerx:registrar")
            .unwrap_or_else(|error| panic!("unexpected error: {error:?}")),
        AuthorityRef::new("custody:human-primary")
            .unwrap_or_else(|error| panic!("unexpected error: {error:?}")),
        2,
        4,
        1000,
        1030,
        10,
    )
    .unwrap_or_else(|error| panic!("unexpected error: {error:?}"));
    let action = journey
        .prepare_native_agent_action(
            &mut scope,
            &fixture.registry,
            &fixture.agent,
            &mut agent_runtime,
            &stale,
            102,
        )
        .unwrap_or_else(|error| panic!("unexpected error: {error:?}"))
        .unwrap_or_else(|| panic!("missing value"));
    server
        .join()
        .unwrap_or_else(|error| panic!("unexpected error: {error:?}"));
    assert_eq!(action.pending_nonce(), pending_nonce);
    assert_eq!(
        action.call().request().operation.account_sequence.0,
        pending_nonce
    );
    assert_eq!(
        record(&scope, action.stage())["preparation"]["pending_nonce"],
        pending_nonce
    );
    assert_eq!(
        record(&scope, action.stage())["preparation"]["account_sequence"],
        pending_nonce
    );
    drop(scope);
    drop(journey);
    drop(agent_runtime);
    fs::remove_file(&socket).unwrap_or_else(|error| panic!("unexpected error: {error:?}"));
    fixture.restart();
    // The peer is stopped: durable replay must not query a newer runtime nonce.
    let mut agent_runtime = runtime(&socket, &fixture.registry);
    let mut scope = fixture
        .store
        .as_mut()
        .unwrap_or_else(|| panic!("missing value"))
        .principal(&principal("alice"))
        .unwrap_or_else(|error| panic!("unexpected error: {error:?}"));
    let mut journey = OnboardingJourney::load(&scope)
        .unwrap_or_else(|error| panic!("unexpected error: {error:?}"))
        .unwrap_or_else(|| panic!("missing value"));
    let replay = journey
        .prepare_native_agent_action(
            &mut scope,
            &fixture.registry,
            &fixture.agent,
            &mut agent_runtime,
            &context(99, 1100),
            103,
        )
        .unwrap_or_else(|error| panic!("unexpected error: {error:?}"))
        .unwrap_or_else(|| panic!("missing value"));
    assert_eq!(replay.pending_nonce(), pending_nonce);
    assert_eq!(
        replay.call().request().operation.account_sequence.0,
        pending_nonce
    );
    assert_eq!(replay, action);
}

pub mod require_value {
    include!("support/require_value.rs");
}
