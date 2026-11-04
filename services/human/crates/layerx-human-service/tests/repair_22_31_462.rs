//! Repair 22.31.462: persist one Rung::wire_rank byte and restore its level floor.
//! No finer level byte is needed: the budget observation conveys rung achievement,
//! and restarting must not infer evidence beyond the stored rung.
//! Files: agentd/src/{human_runtime.rs,managed_agent.rs}, Human src/agents/{mod.rs,recovery.rs}, this test.
//! Live consumers: Human src/server/agent_runtime.rs uses the same rung decoder.
//! Qualification: hf-build, then hf-done; exact revision and logs are retained by those gates.

#[path = "../../../../platform/hosted/runtime-clock/src/server.rs"]
mod runtime_clock_server;

use std::fmt::Debug;
use std::os::unix::net::{UnixListener, UnixStream};
use std::path::PathBuf;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::Arc;
use std::thread;
use std::time::Duration;

use layerx_agentd::human::{decode_request, encode_response, HumanRequest, HumanResponse};
use layerx_agentd::human_runtime::{CoreBudgetState, CoreKeyPolicy};
use layerx_agentd::managed_agent::{record_budget_snapshot, record_key_policy_snapshot};
use layerx_agentd::store::{key, ObjectKind, Store, StoreError, TenantId};
use layerx_client::runtime_clock::{ClockBinding, RuntimeClock};
use layerx_human_service::agents::{decode_budget_snapshot, AgentRecoveryError};
use layerx_human_service::frame::{read_frame, write_frame, Limits};
use layerx_human_service::journeys::AgentBoundaryError;
use layerx_human_service::server::agent_runtime::AgentRuntime;
use layerx_human_test_support::directory;
use layerx_types::payload::ModuleRegistry;
use layerx_types::rung::Rung;
use layerx_types::verify::VerificationLevel;

const BUDGET: [u8; 32] = [0x31; 32];
const MAX_FRAME: usize = 4096;

fn checked<T, E: Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("repair 22.31.462: {error:?}"))
}

struct Directory(PathBuf);

impl Directory {
    fn new() -> Self {
        let path = directory("b462");
        checked(std::fs::create_dir_all(&path));
        Self(path)
    }
}

impl Drop for Directory {
    fn drop(&mut self) {
        let _ = std::fs::remove_dir_all(&self.0);
    }
}

fn state(level: VerificationLevel, tag: u8) -> CoreBudgetState {
    CoreBudgetState {
        revocation_sequence: 11,
        observed_head_sequence: 13,
        verification: level.wire_rank(),
        evidence_digest: [tag; 32],
        receipt_digest: [0x42; 32],
        checkpoint_digest: [0x43; 32],
        age_sequences: 1,
        maximum_age_sequences: 10,
        remaining: (1_u128 << 100) + 17,
        asset: [0x44; 32],
    }
}

fn transport(bytes: Vec<u8>) -> Vec<u8> {
    let (mut sender, mut receiver) = checked(UnixStream::pair());
    checked(receiver.set_read_timeout(Some(Duration::from_secs(5))));
    checked(sender.set_write_timeout(Some(Duration::from_secs(5))));
    let writer = thread::spawn(move || checked(write_frame(&mut sender, &bytes, MAX_FRAME)));
    let received = checked(read_frame(&mut receiver, MAX_FRAME));
    checked(writer.join());
    received
}

#[test]
fn daemon_budget_states_cross_transport_and_reopen_at_the_expected_rung_floor() {
    let root = Directory::new();
    let tenant = checked(TenantId::new("budget-wire-owner"));
    for (tag, level, expected_rung, floor) in [
        (
            1,
            VerificationLevel::UNVERIFIED,
            Rung::Submitted,
            VerificationLevel::UNVERIFIED,
        ),
        (
            2,
            VerificationLevel::SEQUENCER_SIGNED,
            Rung::Submitted,
            VerificationLevel::UNVERIFIED,
        ),
        (
            3,
            VerificationLevel::BATCH_INCLUDED,
            Rung::Receipted,
            VerificationLevel::BATCH_INCLUDED,
        ),
        (
            4,
            VerificationLevel::STATE_PROVEN,
            Rung::Receipted,
            VerificationLevel::BATCH_INCLUDED,
        ),
        (
            5,
            VerificationLevel::COMMITTED,
            Rung::Committed,
            VerificationLevel::COMMITTED,
        ),
    ] {
        let mut expected = state(level, tag);
        let mut store = checked(Store::open(root.0.join("store")));
        let encoded = checked(record_budget_snapshot(
            &mut store, &tenant, BUDGET, &expected,
        ));
        assert_eq!(encoded.bytes().len(), 209);
        assert_eq!(encoded.bytes()[48], expected_rung.wire_rank());
        let received = transport(encoded.bytes().to_vec());
        let (budget, human, rung) = checked(decode_budget_snapshot(&received));
        expected.verification = floor.wire_rank();
        assert_eq!(budget, BUDGET);
        assert_eq!(rung, expected_rung);
        assert_eq!(human, expected);
        assert_eq!(
            checked(CoreBudgetState::decode_snapshot(&received)),
            (BUDGET, expected.clone())
        );
        drop(store);

        let reopened = checked(Store::open(root.0.join("store")));
        let mut object = b"managed-agent-observation-v1:".to_vec();
        object.push(1);
        object.extend_from_slice(&expected.evidence_digest);
        let record = checked(key(tenant.clone(), ObjectKind::Configuration, object));
        let persisted = reopened
            .get(&record)
            .expect("durable daemon budget observation");
        assert_eq!(persisted.bytes(), encoded.bytes());
        let (budget, restored) = checked(CoreBudgetState::decode_snapshot(persisted.bytes()));
        assert_eq!(restored, expected);
        assert_eq!(checked(restored.encode_snapshot(budget)), encoded.bytes());
        assert_eq!(
            checked(decode_budget_snapshot(persisted.bytes())).2,
            expected_rung
        );
    }
}

#[test]
fn both_budget_decoders_refuse_every_noncontract_byte_and_malformed_length() {
    let valid = checked(state(VerificationLevel::COMMITTED, 1).encode_snapshot(BUDGET));
    for rank in 3..=u8::MAX {
        let mut invalid = valid.clone();
        invalid[48] = rank;
        let received = transport(invalid);
        assert!(
            matches!(
                CoreBudgetState::decode_snapshot(&received),
                Err(StoreError::Corrupt(_))
            ),
            "daemon accepted {rank}"
        );
        assert!(
            matches!(
                decode_budget_snapshot(&received),
                Err(AgentRecoveryError::Corrupt(_))
            ),
            "Human accepted {rank}"
        );
    }
    for length in 0..valid.len() {
        assert!(matches!(
            CoreBudgetState::decode_snapshot(&valid[..length]),
            Err(StoreError::Corrupt(_))
        ));
        assert!(matches!(
            decode_budget_snapshot(&valid[..length]),
            Err(AgentRecoveryError::Corrupt(_))
        ));
    }
    let mut trailing = valid;
    trailing.push(0);
    assert!(matches!(
        CoreBudgetState::decode_snapshot(&trailing),
        Err(StoreError::Corrupt(_))
    ));
    assert!(matches!(
        decode_budget_snapshot(&trailing),
        Err(AgentRecoveryError::Corrupt(_))
    ));
}

#[test]
fn budget_encoder_refuses_retired_and_unknown_verification_levels() {
    let root = Directory::new();
    let mut store = checked(Store::open(root.0.join("store")));
    let tenant = checked(TenantId::new("budget-wire-owner"));
    for rank in [4, 5].into_iter().chain(7..=u8::MAX) {
        let mut invalid = state(VerificationLevel::COMMITTED, 1);
        invalid.verification = rank;
        assert!(matches!(
            invalid.encode_snapshot(BUDGET),
            Err(StoreError::Corrupt(_))
        ));
        assert!(record_budget_snapshot(&mut store, &tenant, BUDGET, &invalid).is_err());
    }
    assert!(store
        .list_object_ids(&tenant, ObjectKind::Configuration)
        .is_empty());
}

struct ClockServer {
    clock: Arc<RuntimeClock>,
    stop: Arc<AtomicBool>,
    worker: Option<thread::JoinHandle<()>>,
}

impl ClockServer {
    fn start(root: &std::path::Path) -> Self {
        let mut server = checked(runtime_clock_server::Server::bind(root));
        let binding = ClockBinding {
            socket: server.socket().to_owned(),
            process: std::process::id(),
            user: rustix::process::getuid().as_raw(),
        };
        let stop = Arc::new(AtomicBool::new(false));
        let shutdown = Arc::clone(&stop);
        let worker = thread::spawn(move || {
            while !shutdown.load(Ordering::Acquire) {
                checked(server.poll());
                thread::sleep(Duration::from_millis(1));
            }
        });
        Self {
            clock: checked(RuntimeClock::connect(binding)),
            stop,
            worker: Some(worker),
        }
    }
}

impl Drop for ClockServer {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        if let Some(worker) = self.worker.take() {
            checked(worker.join());
        }
    }
}

#[test]
fn live_human_budget_boundary_accepts_the_daemon_committed_snapshot() {
    let root = Directory::new();
    let clock = ClockServer::start(&root.0);
    let socket = root.0.join("agent.sock");
    let listener = checked(UnixListener::bind(&socket));
    let store_path = root.0.join("store");
    let producer = thread::spawn(move || {
        let (mut stream, _) = checked(listener.accept());
        checked(stream.set_read_timeout(Some(Duration::from_secs(5))));
        checked(stream.set_write_timeout(Some(Duration::from_secs(5))));
        let request = checked(decode_request(checked(read_frame(&mut stream, MAX_FRAME))));
        let HumanRequest::AgentBudgetState { active_budget_id } = request else {
            panic!("expected the production budget-state request");
        };
        assert_eq!(active_budget_id, BUDGET);
        let tenant = checked(TenantId::new("budget-wire-owner"));
        let mut store = checked(Store::open(store_path));
        let response: HumanResponse = checked(record_budget_snapshot(
            &mut store,
            &tenant,
            active_budget_id,
            &state(VerificationLevel::COMMITTED, 1),
        ));
        checked(write_frame(
            &mut stream,
            &encode_response(Ok(response)),
            MAX_FRAME,
        ));
    });
    let mut runtime = checked(AgentRuntime::new_with_clock(
        &socket,
        Limits {
            maximum_frame_bytes: MAX_FRAME,
            maximum_connections: 1,
            maximum_streams: 1,
            maximum_queued_bytes: MAX_FRAME,
            deadline: Duration::from_secs(5),
        },
        checked(ModuleRegistry::new(&[])),
        clock.clock.clone(),
    ));
    let result = runtime.agent_budget_state(BUDGET);
    checked(producer.join());
    let decoded = checked(result);
    assert_eq!(decoded.active_budget_id, BUDGET);
    assert_eq!(decoded.verification, Rung::Committed.wire_rank());
    assert_eq!(
        decoded.remaining,
        state(VerificationLevel::COMMITTED, 1).remaining
    );
}

#[test]
fn live_human_rank_checks_accept_exactly_the_shared_budget_contract() {
    let root = Directory::new();
    let clock = ClockServer::start(&root.0);
    let socket = root.0.join("agent.sock");
    let listener = checked(UnixListener::bind(&socket));
    let store_path = root.0.join("store");
    let producer = thread::spawn(move || {
        let tenant = checked(TenantId::new("budget-wire-owner"));
        let mut store = checked(Store::open(store_path));
        for rank in 0..=u8::MAX {
            let level = match rank {
                0 => VerificationLevel::UNVERIFIED,
                1 => VerificationLevel::BATCH_INCLUDED,
                _ => VerificationLevel::COMMITTED,
            };
            let mut evidence_digest = [0x41; 32];
            evidence_digest[0] = rank;
            let mut observation = state(level, 1);
            observation.evidence_digest = evidence_digest;
            let response = checked(record_budget_snapshot(
                &mut store,
                &tenant,
                BUDGET,
                &observation,
            ));
            let mut bytes = response.bytes().to_vec();
            if rank > Rung::Committed.wire_rank() {
                bytes[48] = rank;
            }
            assert_eq!(bytes[48], rank);
            let (mut stream, _) = checked(listener.accept());
            checked(stream.set_read_timeout(Some(Duration::from_secs(5))));
            checked(stream.set_write_timeout(Some(Duration::from_secs(5))));
            assert!(matches!(
                checked(decode_request(checked(read_frame(&mut stream, MAX_FRAME)))),
                HumanRequest::AgentBudgetState { active_budget_id } if active_budget_id == BUDGET
            ));
            checked(write_frame(
                &mut stream,
                &encode_response(Ok(checked(HumanResponse::new(bytes)))),
                MAX_FRAME,
            ));

            let (mut stream, _) = checked(listener.accept());
            checked(stream.set_read_timeout(Some(Duration::from_secs(5))));
            checked(stream.set_write_timeout(Some(Duration::from_secs(5))));
            assert!(matches!(
                checked(decode_request(checked(read_frame(&mut stream, MAX_FRAME)))),
                HumanRequest::AgentKeyPolicy { agent_did, recovery }
                    if agent_did == "did:layerx:budget-owner" && !recovery
            ));
            let response = checked(record_key_policy_snapshot(
                &mut store,
                &tenant,
                "did:layerx:budget-owner",
                false,
                &CoreKeyPolicy {
                    policy_revision: 1,
                    required_delay_seconds: 10,
                    maximum_delay_seconds: 20,
                    effective_sequence: 11,
                    observed_head_sequence: 13,
                    verification: rank,
                    evidence_digest,
                    checkpoint_digest: [0x43; 32],
                    age_sequences: 1,
                    maximum_age_sequences: 10,
                },
            ));
            checked(write_frame(
                &mut stream,
                &encode_response(Ok(response)),
                MAX_FRAME,
            ));
        }
    });
    let mut runtime = checked(AgentRuntime::new_with_clock(
        &socket,
        Limits {
            maximum_frame_bytes: MAX_FRAME,
            maximum_connections: 1,
            maximum_streams: 1,
            maximum_queued_bytes: MAX_FRAME,
            deadline: Duration::from_secs(5),
        },
        checked(ModuleRegistry::new(&[])),
        clock.clock.clone(),
    ));
    for rank in 0..=u8::MAX {
        let budget = runtime.agent_budget_state(BUDGET);
        let policy = runtime.agent_key_policy("did:layerx:budget-owner", false);
        if rank <= Rung::Committed.wire_rank() {
            assert_eq!(checked(budget).verification, rank);
            assert_eq!(checked(policy).verification, rank);
        } else {
            assert!(
                matches!(budget, Err(AgentBoundaryError::CorruptResponse)),
                "live budget boundary accepted {rank}"
            );
            assert!(
                matches!(policy, Err(AgentBoundaryError::CorruptResponse)),
                "live key-policy boundary accepted {rank}"
            );
        }
    }
    checked(producer.join());
}
