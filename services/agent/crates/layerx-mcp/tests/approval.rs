use layerx_mcp_test_support::approval_fixture::{context, limiter, prepared, reserved_hold};
use layerx_mcp_test_support::option_assertion;
use layerx_mcp_test_support::result_assertion;

use layerx_mcp_test_support::test_clock;

use layerx_agent_api::Amount;
use std::fs;
use std::sync::atomic::{AtomicU64, Ordering};

use layerx_agentd::policy::approval::{
    ApprovalError as DaemonApprovalError, ApprovalRegistry, ApprovalState, ApproverId,
};
use layerx_mcp::approval::{approve, expire, require, ApprovalError, ApprovalPolicy, Requirement};

static NEXT_DIRECTORY: AtomicU64 = AtomicU64::new(1);

fn directory(label: &str) -> std::path::PathBuf {
    let sequence = NEXT_DIRECTORY.fetch_add(1, Ordering::Relaxed);
    std::env::temp_dir().join(format!(
        "layerx-mcp-approval-{label}-{}-{}-{sequence}",
        module_path!().replace("::", "-"),
        std::process::id()
    ))
}

#[test]
fn amount_at_threshold_proceeds_and_one_unit_over_is_held() {
    let registry = ApprovalRegistry::default();
    let at = require(
        &registry,
        ApprovalPolicy {
            amount_threshold: 100,
        },
        context(1),
        prepared(100, 1),
        50,
        &test_clock::TestClock::new(50),
    )
    .unwrap_or_else(|error| panic!("threshold: {error:?}"));
    assert!(matches!(
        at,
        Requirement::NotRequired {
            disclosed_amount: 100,
            ..
        }
    ));
    let above = require(
        &registry,
        ApprovalPolicy {
            amount_threshold: 100,
        },
        context(2),
        prepared(101, 2),
        50,
        &test_clock::TestClock::new(50),
    )
    .unwrap_or_else(|error| panic!("above threshold: {error:?}"));
    assert!(matches!(above, Requirement::Required(_)));
}

#[test]
fn approver_sees_the_disclosure_and_cannot_approve_an_altered_request() {
    let registry = ApprovalRegistry::default();
    let Requirement::Required(ticket) = require(
        &registry,
        ApprovalPolicy {
            amount_threshold: 100,
        },
        context(3),
        prepared(101, 3),
        50,
        &test_clock::TestClock::new(50),
    )
    .unwrap_or_else(|error| panic!("hold: {error:?}")) else {
        panic!("approval was not required");
    };
    let mut altered = ticket.disclosure.clone();
    altered.fee_limit = Amount(1);
    let approver =
        ApproverId::new("human:alice").unwrap_or_else(|error| panic!("approver: {error:?}"));
    assert_eq!(
        approve(
            &registry,
            ticket.hold_id,
            approver,
            &altered,
            60,
            &test_clock::TestClock::new(60)
        ),
        Err(ApprovalError::DisclosureChanged)
    );
    let audit = approve(
        &registry,
        ticket.hold_id,
        ApproverId::new("human:alice").unwrap_or_else(|error| panic!("approver: {error:?}")),
        &ticket.disclosure,
        60,
        &test_clock::TestClock::new(60),
    )
    .unwrap_or_else(|error| panic!("approval: {error:?}"));
    assert_eq!(audit.decision, ApprovalState::Approved);
    assert_eq!(
        audit
            .approver
            .as_ref()
            .unwrap_or_else(|| panic!("approver absent"))
            .as_str(),
        "human:alice"
    );
    assert_eq!(audit.disclosure_digest, ticket.disclosure_digest);
}

#[test]
fn expiry_at_the_declared_sequence_is_terminal_and_never_auto_approves() {
    let root = directory("expiry");
    fs::create_dir_all(&root).unwrap_or_else(|error| panic!("directory: {error}"));
    let registry = ApprovalRegistry::default();
    let Requirement::Required(ticket) = require(
        &registry,
        ApprovalPolicy {
            amount_threshold: 1,
        },
        context(4),
        prepared(2, 4),
        50,
        &test_clock::TestClock::new(50),
    )
    .unwrap_or_else(|error| panic!("hold: {error:?}")) else {
        panic!("approval was not required");
    };
    assert!(expire(&registry, 119, &test_clock::TestClock::new(119))
        .unwrap_or_else(|error| panic!("early expiry: {error:?}"))
        .is_empty());
    let expired = expire(&registry, 120, &test_clock::TestClock::new(120))
        .unwrap_or_else(|error| panic!("expiry: {error:?}"));
    assert_eq!(expired.len(), 1);
    assert_eq!(expired[0].decision, ApprovalState::Expired);
    assert!(expired[0].approver.is_none());
    assert_eq!(
        approve(
            &registry,
            ticket.hold_id,
            ApproverId::new("human:late").unwrap_or_else(|error| panic!("approver: {error:?}")),
            &ticket.disclosure,
            120,
            &test_clock::TestClock::new(120),
        ),
        Err(ApprovalError::Daemon(DaemonApprovalError::AlreadyDecided(
            ApprovalState::Expired
        )))
    );
    let _ = fs::remove_dir_all(root);
}

#[test]
fn durable_call_reopens_with_reservation_and_refuses_changed_bytes_and_sender() {
    use layerx_agentd::store::Store;
    use layerx_types::payload::ModuleRegistry;
    use std::sync::{Arc, Mutex};
    for corruption in [None, Some("bytes"), Some("sender")] {
        let root = directory("replay");
        let store = Arc::new(Mutex::new(result_assertion::required(
            Store::open(&root),
            "durable store",
        )));
        let registry = ApprovalRegistry::with_store(Arc::clone(&store));
        let original = limiter();
        let ticket = reserved_hold(&registry, &original, 19);
        result_assertion::required(
            registry.validate_registry(
                &context(19).tenant,
                &result_assertion::required(ModuleRegistry::new(&[]), "modules"),
            ),
            "valid held call",
        );
        assert_eq!(
            result_assertion::required(
                original.held_exposure(layerx_agentd::budget::LimitId([1; 16])),
                "reserved exposure"
            ),
            101
        );
        if let Some(field) = corruption {
            corrupt_held_call(&store, field);
        }
        drop(registry);
        drop(original);
        drop(store);
        let reopened = Arc::new(Mutex::new(result_assertion::required(
            Store::open(&root),
            "reopen stopped store",
        )));
        let registry = ApprovalRegistry::with_store(reopened);
        let recovered = limiter();
        let replay = registry.replay_tenant(&context(19).tenant, &recovered);
        if corruption.is_some() {
            assert!(
                replay.is_err()
                    || registry
                        .validate_registry(
                            &context(19).tenant,
                            &result_assertion::required(ModuleRegistry::new(&[]), "modules")
                        )
                        .is_err(),
                "changed retained {corruption:?} accepted"
            );
        } else {
            assert_eq!(replay, Ok(1));
            result_assertion::required(
                registry.validate_registry(
                    &context(19).tenant,
                    &result_assertion::required(ModuleRegistry::new(&[]), "modules"),
                ),
                "rebind recovered sender and call",
            );
            assert_eq!(
                result_assertion::required(registry.ticket(ticket.hold_id), "ticket"),
                Some(ticket)
            );
            assert_eq!(
                result_assertion::required(
                    recovered.held_exposure(layerx_agentd::budget::LimitId([1; 16])),
                    "recovered reservation"
                ),
                101
            );
        }
        drop(registry);
        result_assertion::required(fs::remove_dir_all(root), "remove replay state");
    }
}

fn corrupt_held_call(store: &std::sync::Mutex<layerx_agentd::store::Store>, field: &str) {
    use layerx_agentd::store::{ObjectKind, TenantKey};
    let mut durable = result_assertion::required(store.lock(), "store");
    let id = option_assertion::required(
        durable
            .list_object_ids(&context(19).tenant, ObjectKind::PreparedActivity)
            .into_iter()
            .find(|id| id.starts_with(b"approval-hold-v1:")),
        "hold key",
    );
    let key = result_assertion::required(
        TenantKey::new(context(19).tenant, ObjectKind::PreparedActivity, id),
        "key",
    );
    let mut bytes = option_assertion::required(durable.get(&key), "record")
        .bytes()
        .to_vec();
    let needle = if field == "bytes" {
        prepared(101, 19)
            .unsigned_canonical_bytes
            .as_bytes()
            .to_vec()
    } else {
        vec![4; 20]
    };
    let offset = option_assertion::required(
        bytes.windows(needle.len()).position(|w| w == needle),
        "retained field",
    );
    bytes[offset] ^= 1;
    result_assertion::required(durable.put_local(key, bytes), "persist corruption");
}
