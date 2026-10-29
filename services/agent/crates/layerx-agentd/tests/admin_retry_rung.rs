use std::fmt::Debug;
use std::fs;
use std::path::PathBuf;

use layerx_agentd::admin::{AdminError, OperatorContext, Surface, VerificationBacklog};
use layerx_agentd::audit::verify_chain;
use layerx_agentd::finality::{FinalityError, SubmissionProgress, WaitResult};
use layerx_agentd::store::TenantId;
use layerx_types::rung::Rung;
use layerx_types::verify::VerificationLevel;

const IDEMPOTENCY_KEY: [u8; 32] = [1; 32];
const TX_HASH: [u8; 32] = [2; 32];
const DEADLINE_MS: u64 = 1_000;

fn must<T, E: Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("{error:?}"))
}

fn surface(name: &str) -> (Surface, PathBuf) {
    let root =
        std::env::temp_dir().join(format!("layerx-admin-retry-{name}-{}", std::process::id()));
    let tenant = must(TenantId::new("admin-retry-rung"));
    (must(Surface::open(&root, &tenant)), root)
}

fn backlog(requested: VerificationLevel) -> VerificationBacklog {
    VerificationBacklog {
        idempotency_key: IDEMPOTENCY_KEY,
        observed: VerificationLevel::UNVERIFIED,
        requested,
        queued_at_ms: 900,
        attempts: 1,
    }
}

#[test]
fn settlement_anchored_retry_is_refused() {
    let (mut surface, root) = surface("settlement-anchored");
    let context = must(OperatorContext::new("retry-operator", [3; 32]));
    let mut progress = SubmissionProgress::accepted(IDEMPOTENCY_KEY, TX_HASH);

    assert!(matches!(
        surface.retry_verification(
            &context,
            &mut progress,
            backlog(VerificationLevel::SETTLEMENT_ANCHORED),
            DEADLINE_MS,
            DEADLINE_MS,
            1,
        ),
        Err(AdminError::Verification(FinalityError::ProgressUnavailable))
    ));
    assert_eq!(surface.audit_entries(), 1);
    assert_eq!(must(verify_chain(surface.audit_path())).entries, 1);
    must(fs::remove_dir_all(root));
}

#[test]
fn checkpoint_finalised_retry_requests_committed() {
    let (mut surface, root) = surface("checkpoint-finalised");
    let context = must(OperatorContext::new("retry-operator", [4; 32]));
    let mut progress = SubmissionProgress::accepted(IDEMPOTENCY_KEY, TX_HASH);

    assert_eq!(
        must(surface.retry_verification(
            &context,
            &mut progress,
            backlog(VerificationLevel::CHECKPOINT_FINALISED),
            DEADLINE_MS,
            DEADLINE_MS,
            1,
        )),
        WaitResult {
            requested: Rung::Committed,
            reached: Rung::Submitted,
            observed_at_ms: DEADLINE_MS,
            deadline_elapsed: true,
        }
    );
    assert_eq!(surface.audit_entries(), 1);
    assert_eq!(must(verify_chain(surface.audit_path())).entries, 1);
    must(fs::remove_dir_all(root));
}
