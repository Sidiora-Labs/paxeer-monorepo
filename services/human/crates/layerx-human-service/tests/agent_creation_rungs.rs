#[path = "support/canonical_owner.rs"]
mod canonical_owner;

use std::fmt::Debug;
use std::os::unix::net::UnixListener;
use std::path::PathBuf;
use std::sync::{Arc, Mutex};
use std::thread::{self, JoinHandle};
use std::time::{Duration, Instant};

use ed25519_dalek::{Signer as _, SigningKey};
use layerx_agent_api::identity::{AgentDid, AuthorityRef};
use layerx_human_service::agents::{
    AgentCreationContract, AgentEvidence, AgentFailure, CapabilityProvision, CreationStage,
};
use layerx_human_service::custody::{
    CustodySigner, EnvelopeKms, KeyClass, KeyEntropy, KeyId, Keystore, SigningLimits,
};
use layerx_human_service::frame::{read_frame, write_frame, Limits};
use layerx_human_service::server::agent_creation::{
    CreationBounds, ProductionAgentCreation, ProtocolEvidence, Rung,
};
use layerx_human_service::server::agent_runtime::AgentRuntime;
use layerx_human_service::store::{PrincipalStore, RowKey, Table};
use layerx_human_service::trace::TraceId;
use layerx_human_test_support as support;
use layerx_intents::canonical::STATE_COMMITMENT_PROTOCOL_VERSION;
use layerx_intents::{BudgetCreate, Intent, IntentKind};
use layerx_sdk::Client;
use layerx_types::account::AccountId;
use layerx_types::amount::Amount;
use layerx_types::ids::{AssetId, Did};
use layerx_types::intent::{BudgetId, PeriodLength, PurposeHash, RolloverPolicy, TimestampSeconds};
use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};
use sha2::{Digest as _, Sha256};

const ACTION: [u8; 32] = [0x61; 32];
const HEIGHT: u64 = 11;
const MAX_FRAME: usize = 8192;

fn checked<T, E: Debug>(value: Result<T, E>) -> T {
    value.unwrap_or_else(|error| panic!("production creation rungs: {error:?}"))
}

fn bytes(target: &mut Vec<u8>, value: &[u8]) {
    target.extend_from_slice(&checked(u32::try_from(value.len())).to_be_bytes());
    target.extend_from_slice(value);
}

fn frame(opcode: u8, body: &[u8]) -> Vec<u8> {
    let mut frame = b"LXHAGT01".to_vec();
    frame.push(opcode);
    frame.extend_from_slice(body);
    frame
}

fn intent() -> Intent {
    Intent::v1(IntentKind::BudgetCreate(checked(BudgetCreate::new(
        BudgetId::new(ACTION),
        checked(AccountId::parse(
            "agent:did:layerx:owner-receipt-fixture:main",
        )),
        checked(AccountId::parse(
            "agent:did:layerx:owner-receipt-fixture:budget:61616161",
        )),
        AssetId::new([0x22; 32]),
        Amount::from_u128(500),
        checked(PeriodLength::new(3600)),
        RolloverPolicy::None,
        Amount::ZERO,
        PurposeHash::new([0x23; 32]),
        TimestampSeconds::from_u64(2000),
    ))))
}

fn registry() -> ModuleRegistry {
    checked(ModuleRegistry::new(&[checked(ModuleRegistration::new(
        ModuleId::Budget,
        &[checked(ActivityType::new(ModuleId::Budget, 1))],
    ))]))
}

fn receipt(owner: &canonical_owner::SignedOwner) -> (Vec<u8>, [[u8; 32]; 5]) {
    let signer = SigningKey::from_bytes(&[0x84; 32]);
    let previous = [0x81; 32];
    let mut batch = [0; 32];
    batch[24..].copy_from_slice(&HEIGHT.to_be_bytes());
    let mut raw = STATE_COMMITMENT_PROTOCOL_VERSION.to_be_bytes().to_vec();
    raw.extend_from_slice(&0x5201_u16.to_be_bytes());
    raw.extend_from_slice(&STATE_COMMITMENT_PROTOCOL_VERSION.to_be_bytes());
    bytes(&mut raw, &owner.id);
    raw.extend_from_slice(&HEIGHT.to_be_bytes());
    bytes(&mut raw, &previous);
    bytes(&mut raw, &[0x82; 32]);
    bytes(&mut raw, &[0x85; 32]);
    raw.extend_from_slice(&0_i32.to_be_bytes());
    raw.extend_from_slice(&0_u32.to_be_bytes());
    raw.extend_from_slice(&1_u128.to_be_bytes());
    bytes(&mut raw, &batch);
    raw.extend_from_slice(&(ModuleId::Budget as u16).to_be_bytes());
    raw.extend_from_slice(&1_u32.to_be_bytes());
    raw.extend_from_slice(&1_u32.to_be_bytes());
    raw.push(0);
    bytes(&mut raw, &[0; 32]);
    raw.extend_from_slice(&0_u128.to_be_bytes());
    bytes(&mut raw, &[0; 32]);
    raw.extend_from_slice(&0_u128.to_be_bytes());
    raw.extend_from_slice(&0_u128.to_be_bytes());
    raw.extend_from_slice(&0_u64.to_be_bytes());
    bytes(&mut raw, &[0; 32]);
    raw.extend_from_slice(&0_u128.to_be_bytes());
    raw.extend_from_slice(&0_u128.to_be_bytes());
    for _ in 0..3 {
        bytes(&mut raw, &[0; 32]);
    }
    raw.extend_from_slice(&1002_u64.to_be_bytes());
    raw.push(0);
    let unsigned = checked(layerx_wire::receipt::decode(&raw));
    assert_eq!(
        checked(layerx_wire::receipt::encode_unsigned(&unsigned)),
        raw
    );
    let mut digest = Sha256::new();
    digest.update(b"LXP/v1/receipt\0");
    digest.update(&raw);
    let signature = signer.sign(&<[u8; 32]>::from(digest.finalize()));
    *raw.last_mut().expect("signature tag") = 1;
    bytes(&mut raw, &signature.to_bytes());
    let decoded = checked(layerx_wire::receipt::decode(&raw));
    assert_eq!(checked(layerx_wire::receipt::encode(&decoded)), raw);
    (
        raw,
        [
            batch,
            [0; 32],
            previous,
            [0x82; 32],
            signer.verifying_key().to_bytes(),
        ],
    )
}

fn capability() -> CapabilityProvision {
    CapabilityProvision {
        action_key: [0x71; 32],
        did: checked(Did::new(b"did:layerx:owner-receipt-fixture")),
        capability_id: [0x72; 32],
        activity_types: [checked(ActivityType::new(ModuleId::Budget, 1)).ordinal()].into(),
        counterparties: [[0x73; 32]].into(),
        assets: [[0x22; 32]].into(),
        amount_ceiling: 500,
        rate_maximum_uses: 10,
        rate_window_sequences: 100,
        purposes: ["operations".to_owned()].into(),
        expiry_sequence: 1000,
        primary_authority: [0x74; 32],
    }
}

fn capability_exchange(rung_byte: u8) -> (Vec<u8>, Vec<u8>) {
    let request = capability();
    let mut body = request.action_key.to_vec();
    bytes(&mut body, request.did.as_bytes());
    body.extend_from_slice(&request.primary_authority);
    body.extend_from_slice(&request.capability_id);
    body.extend_from_slice(&1_u16.to_be_bytes());
    for value in request.activity_types {
        body.extend_from_slice(&value.to_be_bytes());
    }
    body.extend_from_slice(&1_u16.to_be_bytes());
    body.extend_from_slice(&[0x73; 32]);
    body.extend_from_slice(&1_u16.to_be_bytes());
    body.extend_from_slice(&[0x22; 32]);
    body.extend_from_slice(&request.amount_ceiling.to_be_bytes());
    body.extend_from_slice(&request.rate_maximum_uses.to_be_bytes());
    body.extend_from_slice(&request.rate_window_sequences.to_be_bytes());
    body.extend_from_slice(&1_u16.to_be_bytes());
    bytes(&mut body, b"operations");
    body.extend_from_slice(&request.expiry_sequence.to_be_bytes());
    let expected = AgentEvidence {
        action_key: request.action_key,
        object_id: request.capability_id,
        observed_sequence: HEIGHT,
        rung: Rung::Committed,
        receipt_digest: [0; 32],
    };
    let mut answer = expected.object_id.to_vec();
    answer.extend_from_slice(&HEIGHT.to_be_bytes());
    answer.push(rung_byte);
    answer.extend_from_slice(&expected.expected_digest(CreationStage::CapabilityNarrowing));
    (frame(30, &body), frame(0, &answer))
}

struct Fixture {
    root: PathBuf,
    store: Arc<Mutex<PrincipalStore>>,
    custody: CustodySigner,
    runtime: AgentRuntime,
    server: Option<JoinHandle<()>>,
    owner: canonical_owner::SignedOwner,
    receipt: Vec<u8>,
}

impl Fixture {
    fn new(rung_byte: u8, legacy_verification: u8) -> Self {
        let root = support::directory("agent-creation-rungs");
        checked(std::fs::create_dir_all(&root));
        let (store, _) = support::install_and_open(
            &root.join("store"),
            &support::tenancy(&[("alice", "tenant-alice")]),
            support::retention_uniform(100),
        );
        let store = Arc::new(Mutex::new(store));
        let secret = root.join("kms-root");
        checked(std::fs::write(&secret, [0x51; 64]));
        let keystore = checked(Keystore::open_development(
            root.join("custody"),
            77,
            checked(EnvelopeKms::new("kms://creation-primary", secret)),
        ));
        checked(keystore.generate(
            &support::principal("alice"),
            &checked(KeyId::new("human-primary")),
            KeyClass::HumanPrimary,
            checked(KeyEntropy::new([0x68; 32], [0x31; 16], [0x41; 24])),
        ));
        let custody = CustodySigner::new_shared(
            keystore,
            Arc::clone(&store),
            registry(),
            checked(SigningLimits::new(10, 60)),
        );
        let compiled = checked(layerx_intents::compile(&intent(), &registry()));
        let owner = canonical_owner::sign_transaction(
            compiled.payload(),
            ACTION,
            STATE_COMMITMENT_PROTOCOL_VERSION,
            77,
        );
        {
            let mut store = checked(store.lock());
            let mut scope = checked(store.principal(&support::principal("alice")));
            checked(scope.put(
                Table::Journeys,
                checked(RowKey::new(format!("protocol-signed-{}", hex(&ACTION)))),
                1000,
                owner.bytes.clone(),
            ));
        }
        let (receipt, batch) = receipt(&owner);
        let mut request = ACTION.to_vec();
        request.extend_from_slice(&owner.id);
        let mut answer = vec![1];
        bytes(&mut answer, &receipt);
        for field in batch {
            answer.extend_from_slice(&field);
        }
        answer.push(legacy_verification);
        answer.extend_from_slice(&owner.id);
        answer.extend_from_slice(&<[u8; 32]>::from(Sha256::digest(&receipt)));
        answer.extend_from_slice(&HEIGHT.to_be_bytes());
        answer.push(rung_byte);
        let exchanges = vec![
            (frame(4, &request), frame(0, &answer)),
            capability_exchange(rung_byte),
        ];
        let socket = root.join("peer.sock");
        let listener = checked(UnixListener::bind(&socket));
        checked(listener.set_nonblocking(true));
        let server = thread::spawn(move || {
            for (request, answer) in exchanges {
                let deadline = Instant::now() + Duration::from_secs(5);
                let mut stream = loop {
                    match listener.accept() {
                        Ok((stream, _)) => break stream,
                        Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                            assert!(Instant::now() < deadline, "creation peer request deadline");
                            thread::sleep(Duration::from_millis(1));
                        }
                        Err(error) => panic!("creation peer accept: {error}"),
                    }
                };
                checked(stream.set_read_timeout(Some(Duration::from_secs(5))));
                checked(stream.set_write_timeout(Some(Duration::from_secs(5))));
                assert_eq!(checked(read_frame(&mut stream, MAX_FRAME)), request);
                checked(write_frame(&mut stream, &answer, MAX_FRAME));
            }
        });
        let runtime = checked(AgentRuntime::new(
            &socket,
            Limits {
                maximum_frame_bytes: MAX_FRAME,
                maximum_connections: 1,
                maximum_streams: 1,
                maximum_queued_bytes: MAX_FRAME,
                deadline: Duration::from_secs(5),
            },
            registry(),
        ));
        Self {
            root,
            store,
            custody,
            runtime,
            server: Some(server),
            owner,
            receipt,
        }
    }

    fn evidence(
        &mut self,
    ) -> (
        Result<ProtocolEvidence, AgentFailure>,
        Result<AgentEvidence, AgentFailure>,
    ) {
        let client = checked(Client::daemon(
            "/run/layerx-agentd.sock",
            layerx_agent_api::agent_api_schema_v1().version,
        ));
        let trace = TraceId::mint([0x31; 16]);
        let mut adapter = checked(ProductionAgentCreation::new(
            &mut self.runtime,
            &client,
            &self.custody,
            &trace,
            checked(AgentDid::new(checked(std::str::from_utf8(
                &self.owner.actor,
            )))),
            checked(AuthorityRef::new(hex(&self.owner.public_key))),
            CreationBounds {
                timestamp_span: 30,
                fee_limit: 10,
            },
        ));
        let mut store = checked(self.store.lock());
        let mut scope = checked(store.principal(&support::principal("alice")));
        let evidence = adapter
            .prepare_lifecycle_intent(
                &mut scope,
                &registry(),
                intent(),
                ACTION,
                checked(KeyId::new("human-primary")),
                1000,
            )
            .and_then(|prepared| adapter.submit_prepared(prepared));
        let capability = adapter.narrow_capability(capability());
        (evidence, capability)
    }

    fn finish(mut self) {
        checked(self.server.take().expect("creation peer").join());
    }
}

impl Drop for Fixture {
    fn drop(&mut self) {
        let _ = std::fs::remove_dir_all(&self.root);
    }
}

fn hex(value: &[u8]) -> String {
    value.iter().map(|byte| format!("{byte:02x}")).collect()
}

#[test]
fn finalization_evidence_reports_the_runtime_rung() {
    for legacy_verification in 1..=5 {
        let mut fixture = Fixture::new(2, legacy_verification);
        let (protocol, capability) = fixture.evidence();
        let protocol = checked(protocol);
        assert_eq!(protocol.rung, Rung::Committed);
        assert_eq!(protocol.tx_hash, fixture.owner.id);
        assert_eq!(
            protocol.receipt_hash,
            <[u8; 32]>::from(Sha256::digest(&fixture.receipt))
        );
        assert_eq!(protocol.block_height, HEIGHT);
        assert_eq!(protocol.receipt_bytes, fixture.receipt);
        assert_eq!(protocol.signed_activity, fixture.owner.bytes);
        let finalized = checked(ProductionAgentCreation::finalization_evidence(
            &protocol,
            ModuleId::Budget,
            1,
            1002,
        ));
        assert_eq!(finalized.verification, Rung::Committed.wire_rank());
        assert_eq!(finalized.action_key, ACTION);
        assert_eq!(finalized.activity_id, fixture.owner.id);
        assert_eq!(finalized.receipt_digest, protocol.receipt_hash);
        assert_eq!(finalized.observed_sequence, HEIGHT);
        assert_eq!(finalized.finalized_at, 1002);
        let capability = checked(capability);
        assert_eq!(capability.rung, Rung::Committed);
        assert!(capability.creation_complete());
        assert_eq!(
            capability.receipt_digest,
            capability.expected_digest(CreationStage::CapabilityNarrowing)
        );
        fixture.finish();
    }
}

#[test]
fn finalization_evidence_refuses_rung_byte_three() {
    for legacy_verification in 4..=5 {
        let mut fixture = Fixture::new(3, legacy_verification);
        let (protocol, capability) = fixture.evidence();
        assert_eq!(
            protocol.as_ref().err(),
            Some(&AgentFailure::Refused(
                "agent refused or corrupted creation operation"
            ))
        );
        let finalized = protocol.and_then(|evidence| {
            ProductionAgentCreation::finalization_evidence(&evidence, ModuleId::Budget, 1, 1002)
        });
        assert!(matches!(
            finalized,
            Err(AgentFailure::Refused(
                "agent refused or corrupted creation operation"
            ))
        ));
        assert_eq!(
            capability,
            Err(AgentFailure::Refused("capability rung is invalid"))
        );
        fixture.finish();
    }
}
