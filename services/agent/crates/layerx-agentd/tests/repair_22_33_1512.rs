//! Real audit writer/recovery proofs for audit record 22.30.91.
//! Finding/test mapping and the baseline-gate constraint are recorded in
//! src/audit/repair_22_33_1512.kvx.

use std::cell::Cell;
use std::collections::BTreeSet;
use std::fmt::Debug;
use std::fs::{self, OpenOptions};
use std::io::Write;
use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicU64, Ordering};
use std::sync::{Arc, Barrier};

use layerx_agentd::admin::{OperatorCommand, OperatorContext, Surface};
use layerx_agentd::audit::log::AppendStage;
use layerx_agentd::audit::{
    read_entries, record, verify_chain, AuditError, Coverage, Decision, Entry, EventClass, Log,
    RecordError, Redacted,
};
use layerx_agentd::store::TenantId;
use layerx_types::ids::Did;
use layerx_types::verify::VerificationLevel;

static NEXT_ROOT: AtomicU64 = AtomicU64::new(0);

fn must<T, E: Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("unexpected error: {error:?}"))
}

struct Root(PathBuf);

impl Root {
    fn new() -> Self {
        Self(std::env::temp_dir().join(format!(
            "audit-repair-22-33-1512-{}-{}",
            std::process::id(),
            NEXT_ROOT.fetch_add(1, Ordering::Relaxed)
        )))
    }

    fn open(&self) -> Log {
        must(Log::open(&self.0, &tenant()))
    }
}

impl Drop for Root {
    fn drop(&mut self) {
        let _ = fs::remove_dir_all(&self.0);
    }
}

fn tenant() -> TenantId {
    must(TenantId::new("audit-owner"))
}

fn entry(tenant: TenantId, request: u8) -> Entry {
    Entry {
        class: EventClass::Authentication,
        observed_at_ms: u64::from(request),
        tenant,
        agent: must(Did::new(b"did:layerx:audit-agent")),
        session: None,
        capability: None,
        policy_version: "audit-policy-v1".to_owned(),
        request_id: [request; 32],
        idempotency_key: None,
        decision: Decision::Allowed,
        reason: Redacted::boot_diagnostic("authenticated request"),
        resulting_activity_id: None,
        verification_level: VerificationLevel::UNVERIFIED,
        protocol_authority: None,
        submitted_bytes: None,
        receipt_id: None,
        tx_hash: None,
        nonce: None,
        rung: None,
    }
}

#[test]
fn audit_concurrent_writers_mcp_record_and_operator_handles_share_one_chain() {
    let root = Root::new();
    // MCP uses record on its independently opened Log. The operator path below
    // uses the real Surface, encoder, committed-frame writer and reader.
    let mut mcp_a = root.open();
    let mut mcp_b = root.open();
    let mut operator = must(Surface::open(&root.0, &tenant()));
    let path = mcp_a.path().to_path_buf();
    let barrier = Arc::new(Barrier::new(4));
    let receipts = std::thread::scope(|scope| {
        let writers: Vec<_> = [&mut mcp_a, &mut mcp_b]
            .into_iter()
            .enumerate()
            .map(|(writer, log)| {
                let barrier = Arc::clone(&barrier);
                scope.spawn(move || {
                    let mut coverage = Coverage::default();
                    let mut receipts = Vec::new();
                    barrier.wait();
                    for request in 0..16 {
                        let value = entry(tenant(), writer as u8 * 16 + request);
                        let (receipt, ()) = must(record(log, &mut coverage, &value, || ()));
                        receipts.push(receipt.sequence);
                    }
                    assert_eq!(coverage.count(EventClass::Authentication), 16);
                    receipts
                })
            })
            .collect();
        let operator_barrier = Arc::clone(&barrier);
        let operator_writer = scope.spawn(move || {
            operator_barrier.wait();
            for request in 0..16 {
                let context = must(OperatorContext::new("audit-operator", [request; 32]));
                must(operator.dispatch(&context, OperatorCommand::InspectUnknown([request; 32])));
            }
        });
        barrier.wait();
        // Readers must not observe the frame/anchor publication window.
        for _ in 0..24 {
            let entries = must(read_entries(&path));
            assert!(entries.len() <= 48);
        }
        must(operator_writer.join());
        writers
            .into_iter()
            .flat_map(|writer| must(writer.join()))
            .collect::<Vec<_>>()
    });
    assert_eq!(receipts.iter().copied().collect::<BTreeSet<_>>().len(), 32);
    drop(mcp_a);
    drop(mcp_b);
    let reopened = root.open();
    assert_eq!(reopened.entries(), 48);
    assert_eq!(must(verify_chain(&path)).entries, 48);
    let recovered = must(read_entries(&path));
    assert_eq!(recovered.len(), 48);
    assert!(recovered.iter().all(|value| value.tenant == tenant()));
    assert_eq!(
        Coverage::from_entries(&recovered).count(EventClass::Authentication),
        32
    );
    assert_eq!(
        Coverage::from_entries(&recovered).count(EventClass::AdministrativeAction),
        16
    );
    let request_ids: BTreeSet<_> = recovered
        .iter()
        .filter(|value| value.class == EventClass::Authentication)
        .map(|value| value.request_id)
        .collect();
    assert_eq!(request_ids.len(), 32);
}

#[test]
fn concurrent_first_open_serializes_initialization_and_appends() {
    let root = Root::new();
    let barrier = Barrier::new(6);
    let mut sequences = std::thread::scope(|scope| {
        let threads: Vec<_> = (0..6)
            .map(|_| {
                let root = &root;
                let barrier = &barrier;
                scope.spawn(move || {
                    barrier.wait();
                    let mut log = root.open();
                    must(log.before_operation(b"concurrent first open", || ()))
                        .0
                        .sequence
                })
            })
            .collect();
        threads
            .into_iter()
            .map(|thread| must(thread.join()))
            .collect::<Vec<_>>()
    });
    sequences.sort_unstable();
    assert_eq!(sequences, (0..6).collect::<Vec<_>>());
    assert_eq!(root.open().entries(), 6);
}

#[test]
fn audit_record_tenant_binding_refuses_before_all_mutation() {
    let root = Root::new();
    let mut log = root.open();
    let mut coverage = Coverage::default();
    let local = entry(tenant(), 1);
    must(record(&mut log, &mut coverage, &local, || ()));
    let before = must(fs::read(log.path()));
    let anchor = must(fs::read(log.path().with_file_name("audit.anchor")));
    let before_coverage = coverage.clone();
    let side_effects = Cell::new(0);
    let mut foreign = entry(must(TenantId::new("foreign-tenant")), 2);
    for _ in 0..2 {
        assert!(matches!(
            record(&mut log, &mut coverage, &foreign, || side_effects.set(1)),
            Err(RecordError::Audit(AuditError::TenantMismatch))
        ));
        // Tenant binding must be checked even before entry validation.
        foreign.policy_version.clear();
    }
    assert_eq!(side_effects.get(), 0);
    assert_eq!(coverage, before_coverage);
    assert_eq!(log.entries(), 1);
    assert_eq!(must(fs::read(log.path())), before);
    assert_eq!(
        must(fs::read(log.path().with_file_name("audit.anchor"))),
        anchor
    );
    assert!(!log.path().with_file_name("audit.intent").exists());
    drop(log);
    let reopened = root.open();
    assert_eq!(must(read_entries(reopened.path())), vec![local]);
}

struct Interrupted {
    root: Root,
    path: PathBuf,
    prefix: Vec<u8>,
    complete: Vec<u8>,
}

fn interrupt(stage: AppendStage) -> Interrupted {
    let root = Root::new();
    let mut log = root.open();
    must(log.before_operation(b"committed history", || ()));
    let path = log.path().to_path_buf();
    let prefix = must(fs::read(&path));
    let operation_ran = Cell::new(false);
    let interrupted = std::panic::catch_unwind(std::panic::AssertUnwindSafe(|| {
        let _ = log.before_operation_observed(
            b"interrupted append with bounded recovery",
            |observed| {
                if observed == stage {
                    panic!("stop at {stage:?}");
                }
            },
            || operation_ran.set(true),
        );
    }));
    assert!(interrupted.is_err());
    assert!(!operation_ran.get());
    drop(log);
    let complete = must(fs::read(&path));
    assert!(path.with_file_name("audit.intent").exists());
    Interrupted {
        root,
        path,
        prefix,
        complete,
    }
}

fn truncate(path: &Path, length: usize) {
    let file = must(OpenOptions::new().write(true).open(path));
    must(file.set_len(length as u64));
    must(file.sync_all());
}

fn recover(interrupted: &Interrupted, expected_entries: u64, expected: &[u8]) {
    let mut reopened = interrupted.root.open();
    assert_eq!(reopened.entries(), expected_entries);
    assert_eq!(
        must(verify_chain(&interrupted.path)).entries,
        expected_entries
    );
    assert_eq!(must(fs::read(&interrupted.path)), expected);
    assert!(!interrupted.path.with_file_name("audit.intent").exists());
    assert_eq!(
        must(reopened.before_operation(b"next append", || 7))
            .0
            .sequence,
        expected_entries
    );
    drop(reopened);
    assert_eq!(interrupted.root.open().entries(), expected_entries + 1);
}

#[test]
fn audit_append_crash_window_before_frame_write_recovers_old_tail() {
    let interrupted = interrupt(AppendStage::IntentSynced);
    assert_eq!(interrupted.complete, interrupted.prefix);
    recover(&interrupted, 1, &interrupted.prefix);
}

#[test]
fn audit_append_crash_window_before_frame_sync_recovers_only_exact_partial_tails() {
    // Simulate loss of an unsynced suffix of the frame emitted by the real
    // producer, at marker, sequence, previous hash, length, payload and hash.
    for surviving in [0, 1, 4, 9, 43, 48, 50, 90, 118] {
        let interrupted = interrupt(AppendStage::FrameWritten);
        assert!(interrupted.complete.len() > interrupted.prefix.len() + surviving);
        truncate(&interrupted.path, interrupted.prefix.len() + surviving);
        recover(&interrupted, 1, &interrupted.prefix);
    }
}

#[test]
fn audit_append_crash_window_complete_unsynced_frame_rolls_forward() {
    let interrupted = interrupt(AppendStage::FrameWritten);
    recover(&interrupted, 2, &interrupted.complete);
}

#[test]
fn audit_append_crash_window_before_anchor_replacement_rolls_forward() {
    let interrupted = interrupt(AppendStage::FrameSynced);
    assert!(verify_chain(&interrupted.path).is_err());
    recover(&interrupted, 2, &interrupted.complete);
}

#[test]
fn audit_append_crash_window_after_anchor_directory_sync_is_idempotent() {
    let interrupted = interrupt(AppendStage::AnchorSynced);
    assert_eq!(must(verify_chain(&interrupted.path)).entries, 2);
    recover(&interrupted, 2, &interrupted.complete);
}

fn assert_rejected_without_mutation(interrupted: &Interrupted) {
    let chain = must(fs::read(&interrupted.path));
    let anchor_path = interrupted.path.with_file_name("audit.anchor");
    let intent_path = interrupted.path.with_file_name("audit.intent");
    let anchor = must(fs::read(&anchor_path));
    let intent = must(fs::read(&intent_path));
    assert!(matches!(
        Log::open(&interrupted.root.0, &tenant()),
        Err(AuditError::Invalid(_))
    ));
    assert_eq!(must(fs::read(&interrupted.path)), chain);
    assert_eq!(must(fs::read(anchor_path)), anchor);
    assert_eq!(must(fs::read(intent_path)), intent);
}

#[test]
fn recovery_refuses_corruption_of_committed_prefix_or_pending_frame() {
    for offset in [8, 40 + 48, 40 + 80 + b"committed history".len() + 49] {
        let interrupted = interrupt(AppendStage::FrameSynced);
        let mut bytes = interrupted.complete.clone();
        bytes[offset] ^= 1;
        must(fs::write(&interrupted.path, bytes));
        assert_rejected_without_mutation(&interrupted);
    }
}

#[test]
fn recovery_refuses_missing_committed_bytes_or_extra_tail() {
    let missing = interrupt(AppendStage::FrameSynced);
    truncate(&missing.path, missing.prefix.len() - 1);
    assert_rejected_without_mutation(&missing);
    let extra = interrupt(AppendStage::FrameSynced);
    let mut file = must(OpenOptions::new().append(true).open(&extra.path));
    must(file.write_all(b"unplanned bytes"));
    must(file.sync_all());
    assert_rejected_without_mutation(&extra);
}

#[test]
fn recovery_never_rolls_back_a_published_anchor() {
    let interrupted = interrupt(AppendStage::AnchorSynced);
    truncate(&interrupted.path, interrupted.prefix.len());
    assert_rejected_without_mutation(&interrupted);
}

#[test]
fn recovery_requires_intact_intent_and_an_expected_anchor() {
    for corrupt_intent in [true, false] {
        let interrupted = interrupt(AppendStage::FrameSynced);
        let path = interrupted.path.with_file_name(if corrupt_intent {
            "audit.intent"
        } else {
            "audit.anchor"
        });
        let mut bytes = must(fs::read(&path));
        let last = bytes.len() - 1;
        bytes[last] ^= 1;
        must(fs::write(path, bytes));
        assert_rejected_without_mutation(&interrupted);
    }
}

#[test]
fn absent_intent_never_authorizes_tail_repair() {
    for partial in [false, true] {
        let interrupted = interrupt(AppendStage::FrameSynced);
        must(fs::remove_file(
            interrupted.path.with_file_name("audit.intent"),
        ));
        if partial {
            truncate(&interrupted.path, interrupted.prefix.len() + 9);
        }
        let before = must(fs::read(&interrupted.path));
        assert!(Log::open(&interrupted.root.0, &tenant()).is_err());
        assert_eq!(must(fs::read(&interrupted.path)), before);
    }
}

#[test]
fn successful_append_clears_intent_before_side_effects_and_detects_later_tampering() {
    let root = Root::new();
    let mut log = root.open();
    let path = log.path().to_path_buf();
    let (receipt, output) = must(log.before_operation(b"durable", || {
        assert!(!path.with_file_name("audit.intent").exists());
        assert_eq!(must(verify_chain(&path)).entries, 1);
        42
    }));
    assert_eq!(receipt.sequence, 0);
    assert_eq!(output, 42);
    drop(log);
    truncate(&path, 40);
    assert!(root.open_result().is_err());
}

impl Root {
    fn open_result(&self) -> Result<Log, AuditError> {
        Log::open(&self.0, &tenant())
    }
}

#[test]
fn committed_record_recovers_after_real_anchor_publication_io_failure() {
    let root = Root::new();
    let mut log = root.open();
    let mut coverage = Coverage::default();
    let first = entry(tenant(), 1);
    let interrupted = entry(tenant(), 2);
    must(record(&mut log, &mut coverage, &first, || ()));
    let path = log.path().to_path_buf();
    // A filesystem obstruction forces a real I/O failure after the committed
    // frame sync and before anchor replacement. No writer or codec is doubled.
    let temporary = path.with_file_name("audit.anchor.tmp");
    must(fs::create_dir(&temporary));
    let operation_ran = Cell::new(false);
    let before_coverage = coverage.clone();
    assert!(matches!(
        record(&mut log, &mut coverage, &interrupted, || operation_ran
            .set(true)),
        Err(RecordError::Audit(AuditError::Io(_)))
    ));
    assert!(!operation_ran.get());
    assert_eq!(coverage, before_coverage);
    assert!(path.with_file_name("audit.intent").exists());
    drop(log);
    must(fs::remove_dir(temporary));
    let mut reopened = root.open();
    assert_eq!(reopened.entries(), 2);
    assert_eq!(
        must(read_entries(&path)),
        vec![first.clone(), interrupted.clone()]
    );
    assert!(!path.with_file_name("audit.intent").exists());
    let next = entry(tenant(), 3);
    let (receipt, ()) = must(record(&mut reopened, &mut coverage, &next, || ()));
    assert_eq!(receipt.sequence, 2);
    drop(reopened);
    assert_eq!(
        must(read_entries(root.open().path())),
        vec![first, interrupted, next]
    );
}
