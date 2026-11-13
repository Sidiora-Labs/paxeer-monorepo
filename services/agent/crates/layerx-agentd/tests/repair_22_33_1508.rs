//! mcp-history-scan: real invocation producer, durable store, evidence consumer and restart.
//! Finding/test mapping and baseline constraints: `src/repair_22_33_1508.kvx`.

use std::collections::{BTreeMap, BTreeSet};
use std::fmt::Debug;
use std::io::Write;
use std::os::unix::fs::PermissionsExt;
use std::path::PathBuf;
use std::sync::atomic::{AtomicU64, Ordering};
use std::sync::{Arc, Mutex};

use layerx_agentd::budget::BudgetLimiter;
use layerx_agentd::events::subscription::Termination;
use layerx_agentd::identity::{
    self, CoreIdentity, IdentityError, IdentityResolver, ProtocolAuthority,
};
use layerx_agentd::prepare::PreparationLifecycle;
use layerx_agentd::session::{
    self, OpenRequest, ScopeRestriction, SessionId, SessionRegistry, Token,
};
use layerx_agentd::session_control::{
    mcp_store_evidence_for_tool, McpInvocationError, McpInvocationHandle, McpInvocationIdentity,
    McpInvocationStart, OperationPermit, SessionControl, MCP_RETAINED_TERMINAL_LIMIT,
};
use layerx_agentd::store::{ObjectKind, Store, TenantId, TenantKey};
use layerx_agentd::tenant::{Operation, Surface};
use layerx_types::ids::Did;
use layerx_types::verify::VerificationLevel;
use serde_json::Value;

static NEXT_ROOT: AtomicU64 = AtomicU64::new(0);
const HEIGHT: u64 = 10;
const HISTORY: usize = MCP_RETAINED_TERMINAL_LIMIT * 3 + 7;
const TOOL: &str = "recorded.read";

fn must<T, E: Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("MCP history repair: {error:?}"))
}

struct Root(PathBuf);
impl Root {
    fn new() -> Self {
        Self(std::env::temp_dir().join(format!(
            "repair-22-33-1508-{}-{}",
            std::process::id(),
            NEXT_ROOT.fetch_add(1, Ordering::Relaxed)
        )))
    }
}
impl Drop for Root {
    fn drop(&mut self) {
        let _ = std::fs::remove_dir_all(&self.0);
    }
}

// A committed core identity transcript is the peer fixture. The invocation, session,
// storage codecs, persistence, generation gates and consumers below are production code.
struct IdentityTranscript(BTreeMap<Vec<u8>, CoreIdentity>);
impl IdentityResolver for IdentityTranscript {
    fn resolve(&mut self, did: &Did) -> Result<Option<CoreIdentity>, IdentityError> {
        Ok(self.0.get(did.as_bytes()).cloned())
    }
}

fn tenant(name: &str) -> TenantId {
    must(TenantId::new(name))
}

fn enroll(
    store: &mut Store,
    sessions: &mut SessionRegistry,
    tenant: &TenantId,
    session_id: u8,
) -> Token {
    let agent = must(Did::new(b"did:layerx:history-agent"));
    let mut transcript = IdentityTranscript(BTreeMap::from([(
        agent.as_bytes().to_vec(),
        CoreIdentity {
            canonical_bytes: b"recorded-identity/history-agent/session-key-4".to_vec(),
            head_sequence: HEIGHT,
            revocation_sequence: 1,
            verification_level: VerificationLevel::STATE_PROVEN,
            frozen: false,
            authorities: vec![ProtocolAuthority::SessionKey([4; 32])],
        },
    )]));
    let identity = must(identity::register(
        store,
        tenant.clone(),
        agent.clone(),
        &mut transcript,
    ));
    must(session::open(
        store,
        sessions,
        &identity,
        OpenRequest {
            session_id: SessionId([session_id; 32]),
            token_id: [session_id.wrapping_add(1); 32],
            tenant: tenant.clone(),
            agent,
            authority: ProtocolAuthority::SessionKey([4; 32]),
            permitted_activity_types: BTreeSet::from([5]),
            scopes: BTreeSet::from(["read".to_owned(), "prepare".to_owned(), "write".to_owned()]),
            expiry_sequence: 10_000,
            expiry_seconds: None,
            opening_client: "mcp-history-repair".to_owned(),
            policy_version: "history-v1".to_owned(),
        },
        HEIGHT,
    ))
}

fn controller(store: &Arc<Mutex<Store>>, sessions: SessionRegistry) -> SessionControl {
    SessionControl::new(
        Arc::clone(store),
        sessions,
        Arc::new(PreparationLifecycle::default()),
        Arc::new(must(BudgetLimiter::new(Vec::new()))),
    )
}

fn permit(control: &SessionControl, token: &Token) -> OperationPermit {
    must(control.authorize(
        &token.credential(),
        Operation::ReadBalance,
        Surface::Mcp,
        HEIGHT,
        None,
    ))
}

fn identity(id: usize, tool: &'static str) -> McpInvocationIdentity {
    let mut key = [7; 32];
    key[..8].copy_from_slice(&(id as u64).to_be_bytes());
    let mut digest = key;
    digest[31] = 9;
    McpInvocationIdentity {
        tool_name: tool,
        idempotency_key: key,
        request_digest: digest,
    }
}

fn begin(permit: &OperationPermit, control: &SessionControl, id: usize) -> McpInvocationHandle {
    match must(permit.mcp_begin(control, identity(id, TOOL))) {
        McpInvocationStart::Started(handle) => handle,
        _ => panic!("new invocation did not start"),
    }
}

fn settle(
    permit: &OperationPermit,
    control: &SessionControl,
    id: usize,
    tool: &'static str,
) -> Vec<u8> {
    let McpInvocationStart::Started(handle) = must(permit.mcp_begin(control, identity(id, tool)))
    else {
        panic!("new invocation did not start")
    };
    let response = must(serde_json::to_vec(
        &serde_json::json!({"id": id, "tool": tool}),
    ));
    let result: Result<Vec<u8>, std::io::Error> =
        must(permit.mcp_effect(control, &handle, || Ok(response.clone())));
    assert_eq!(must(result), response);
    let mut wire = Vec::new();
    must(must(
        permit.mcp_release(control, &handle, || wire.write_all(&response)),
    ));
    let consumed: Value = must(serde_json::from_slice(&wire));
    assert_eq!(consumed["id"], id);
    assert_eq!(consumed["tool"], tool);
    response
}

fn evidence(store: &Arc<Mutex<Store>>, tenant: &TenantId, id: usize, tool: &str) -> Value {
    let guard = must(store.lock());
    must(mcp_store_evidence_for_tool(
        &guard,
        tenant,
        identity(id, TOOL).idempotency_key,
        Some(tool),
    ))
    .unwrap_or_else(|| panic!("missing evidence {id}/{tool}"))
}

fn events(record: &Value, kind: &str) -> usize {
    record["events"]
        .as_array()
        .unwrap_or_else(|| panic!("missing events"))
        .iter()
        .filter(|event| event["kind"] == kind)
        .count()
}

#[test]
fn mcp_history_scan_archives_reopens_and_revokes_only_the_affected_working_set() {
    let root = Root::new();
    let owner = tenant("history-owner");
    let mut durable = must(Store::open(&root.0));
    let mut sessions = SessionRegistry::default();
    let token = enroll(&mut durable, &mut sessions, &owner, 1);
    let sibling = enroll(&mut durable, &mut sessions, &owner, 3);
    let store = Arc::new(Mutex::new(durable));
    let control = controller(&store, sessions);
    let auth = permit(&control, &token);
    let sibling_auth = permit(&control, &sibling);
    let unknown = begin(&auth, &control, 1);
    must(auth.mcp_start_effect(&control, &unknown));
    must(auth.mcp_settle_effect(&control, &unknown, None));
    let begun = begin(&auth, &control, 2);
    let sibling_unknown = begin(&sibling_auth, &control, 3);
    must(sibling_auth.mcp_start_effect(&control, &sibling_unknown));
    for id in 10..10 + HISTORY {
        settle(&auth, &control, id, TOOL);
    }
    let first = evidence(&store, &owner, 10, TOOL);
    let unresolved = evidence(&store, &owner, 1, TOOL);
    let stats = must(store.lock()).mcp_history_stats(&owner);
    assert_eq!(stats.retained_terminal, MCP_RETAINED_TERMINAL_LIMIT);
    assert_eq!(
        stats.archived_terminal,
        HISTORY - MCP_RETAINED_TERMINAL_LIMIT
    );
    assert_eq!(stats.unresolved, 3);
    assert_eq!(stats.last_transition_candidates, 0);
    assert_eq!(unresolved["phase"], "effect_started");
    assert_eq!(events(&unresolved, "effect_unknown"), 1);
    drop(auth);
    drop(sibling_auth);
    drop(control);
    drop(store);

    let durable = must(Store::open(&root.0));
    assert_eq!(
        durable.mcp_history_stats(&owner).retained_terminal,
        stats.retained_terminal
    );
    assert_eq!(
        durable.mcp_history_stats(&owner).archived_terminal,
        stats.archived_terminal
    );
    assert_eq!(
        durable.mcp_history_stats(&owner).unresolved,
        stats.unresolved
    );
    let mut recovered = SessionRegistry::default();
    must(recovered.restore_tenant(&durable, &owner));
    let store = Arc::new(Mutex::new(durable));
    let control = controller(&store, recovered);
    assert_eq!(evidence(&store, &owner, 10, TOOL), first);
    assert_eq!(evidence(&store, &owner, 1, TOOL), unresolved);
    let auth = permit(&control, &token);
    let sibling_auth = permit(&control, &sibling);
    let archived = match must(auth.mcp_begin(&control, identity(10, TOOL))) {
        McpInvocationStart::Settled(handle, response) => {
            assert_eq!(must(serde_json::from_slice::<Value>(&response))["id"], 10);
            handle
        }
        _ => panic!("archived evidence was executed again"),
    };
    assert!(matches!(
        must(auth.mcp_begin(&control, identity(1, TOOL))),
        McpInvocationStart::Unknown(_)
    ));
    assert!(matches!(
        must(auth.mcp_begin(&control, identity(2, TOOL))),
        McpInvocationStart::Unknown(_)
    ));
    let mut changed = identity(10, TOOL);
    changed.request_digest[0] ^= 1;
    assert!(matches!(
        auth.mcp_begin(&control, changed),
        Err(McpInvocationError::Conflict)
    ));

    must(control.close(&owner, SessionId([3; 32]), HEIGHT + 1));
    assert_eq!(
        must(store.lock())
            .mcp_history_stats(&owner)
            .last_transition_candidates,
        1
    );
    assert_history_revocation_and_reopen(HistoryRevocation {
        root,
        owner,
        token,
        begun,
        first,
        unresolved,
        store,
        control,
        auth,
        sibling_auth,
        archived,
    });
}

struct HistoryRevocation {
    root: Root,
    owner: TenantId,
    token: Token,
    begun: McpInvocationHandle,
    first: Value,
    unresolved: Value,
    store: Arc<Mutex<Store>>,
    control: SessionControl,
    auth: OperationPermit,
    sibling_auth: OperationPermit,
    archived: McpInvocationHandle,
}

fn assert_history_revocation_and_reopen(context: HistoryRevocation) {
    let HistoryRevocation {
        root,
        owner,
        token,
        begun,
        first,
        unresolved,
        store,
        control,
        auth,
        sibling_auth,
        archived,
    } = context;

    assert_eq!(
        sibling_auth.stop().reason(),
        Some(Termination::SessionRevoked)
    );
    assert_eq!(
        events(&evidence(&store, &owner, 3, TOOL), "session_close"),
        1
    );
    assert_eq!(evidence(&store, &owner, 1, TOOL), unresolved);
    assert_eq!(auth.stop().reason(), None);
    must(control.close(&owner, SessionId([1; 32]), HEIGHT + 2));
    assert_eq!(
        must(store.lock())
            .mcp_history_stats(&owner)
            .last_transition_candidates,
        MCP_RETAINED_TERMINAL_LIMIT + 2
    );
    assert_eq!(auth.stop().reason(), Some(Termination::SessionRevoked));
    assert!(control
        .authorize(
            &token.credential(),
            Operation::ReadBalance,
            Surface::Mcp,
            HEIGHT + 2,
            None
        )
        .is_err());
    assert_eq!(
        events(&evidence(&store, &owner, 1, TOOL), "session_close"),
        1
    );
    assert_eq!(evidence(&store, &owner, 10, TOOL), first);
    let mut wrote = false;
    assert!(matches!(
        auth.mcp_release(&control, &archived, || {
            wrote = true;
            Ok(())
        }),
        Err(McpInvocationError::Control(_))
    ));
    assert!(!wrote);
    assert!(matches!(
        auth.mcp_start_effect(&control, &begun),
        Err(McpInvocationError::Control(_))
    ));
    assert_eq!(evidence(&store, &owner, 1, TOOL)["phase"], "effect_started");
    drop(auth);
    drop(sibling_auth);
    drop(control);
    drop(store);

    let durable = must(Store::open(&root.0));
    let mut recovered = SessionRegistry::default();
    must(recovered.restore_tenant(&durable, &owner));
    assert!(
        !recovered
            .get(&owner, SessionId([1; 32]))
            .unwrap_or_else(|| panic!("missing recovered session"))
            .open
    );
    assert_eq!(durable.mcp_history_stats(&owner).unresolved, 3);
    let value = must(mcp_store_evidence_for_tool(
        &durable,
        &owner,
        identity(1, TOOL).idempotency_key,
        Some(TOOL),
    ))
    .unwrap_or_else(|| panic!("lost unresolved effect"));
    assert_eq!(value["phase"], "effect_started");
    assert_eq!(events(&value, "effect_unknown"), 1);
    assert_eq!(events(&value, "session_close"), 1);
}

#[test]
fn mcp_history_scan_lookup_preserves_tool_tenant_and_archive_identity() {
    let root = Root::new();
    let owner = tenant("lookup-owner");
    let other = tenant("lookup-other");
    let mut durable = must(Store::open(&root.0));
    let mut sessions = SessionRegistry::default();
    let token = enroll(&mut durable, &mut sessions, &owner, 1);
    let other_token = enroll(&mut durable, &mut sessions, &other, 1);
    let store = Arc::new(Mutex::new(durable));
    let control = controller(&store, sessions);
    let auth = permit(&control, &token);
    let other_auth = permit(&control, &other_token);
    settle(&auth, &control, 1, "recorded.alpha");
    settle(&auth, &control, 1, "recorded.beta");
    settle(&other_auth, &control, 1, "recorded.alpha");
    let alpha = evidence(&store, &owner, 1, "recorded.alpha");
    let beta = evidence(&store, &owner, 1, "recorded.beta");
    let other_alpha = evidence(&store, &other, 1, "recorded.alpha");
    for id in 10..10 + HISTORY {
        settle(&auth, &control, id, TOOL);
    }
    assert_eq!(
        must(store.lock())
            .mcp_history_stats(&owner)
            .archived_terminal,
        HISTORY + 2 - MCP_RETAINED_TERMINAL_LIMIT
    );
    assert!(matches!(
        control.mcp_evidence(&owner, identity(1, TOOL).idempotency_key),
        Err(McpInvocationError::Conflict)
    ));
    let guard = must(store.lock());
    assert!(must(mcp_store_evidence_for_tool(
        &guard,
        &owner,
        identity(1, TOOL).idempotency_key,
        Some("absent.tool")
    ))
    .is_none());
    drop(guard);
    assert!(must(
        control.mcp_evidence(&tenant("absent-tenant"), identity(1, TOOL).idempotency_key)
    )
    .is_none());
    drop(auth);
    drop(other_auth);
    drop(control);
    drop(store);
    let durable = must(Store::open_read_only(&root.0));
    for (tenant, tool, expected) in [
        (&owner, "recorded.alpha", alpha),
        (&owner, "recorded.beta", beta),
        (&other, "recorded.alpha", other_alpha),
    ] {
        assert_eq!(
            must(mcp_store_evidence_for_tool(
                &durable,
                tenant,
                identity(1, TOOL).idempotency_key,
                Some(tool)
            )),
            Some(expected)
        );
    }
    assert!(matches!(
        layerx_agentd::session_control::mcp_store_evidence(
            &durable,
            &owner,
            identity(1, TOOL).idempotency_key
        ),
        Err(McpInvocationError::Conflict)
    ));
    assert_eq!(durable.mcp_history_stats(&other).retained_terminal, 1);
}

#[test]
fn mcp_history_scan_failed_settlement_keeps_unresolved_index_until_durable_retry() {
    let root = Root::new();
    let owner = tenant("rollback-owner");
    let mut durable = must(Store::open(&root.0));
    let mut sessions = SessionRegistry::default();
    let token = enroll(&mut durable, &mut sessions, &owner, 1);
    let store = Arc::new(Mutex::new(durable));
    let control = controller(&store, sessions);
    let auth = permit(&control, &token);
    let handle = begin(&auth, &control, 1);
    must(auth.mcp_start_effect(&control, &handle));
    let before = evidence(&store, &owner, 1, TOOL);
    must(std::fs::set_permissions(
        &root.0,
        std::fs::Permissions::from_mode(0o777),
    ));
    assert!(control
        .close(&owner, SessionId([1; 32]), HEIGHT + 1)
        .is_err());
    assert_eq!(auth.stop().reason(), None);
    must(auth.boundary(&control));
    assert!(matches!(
        auth.mcp_settle_effect(&control, &handle, Some(b"settled")),
        Err(McpInvocationError::Unavailable)
    ));
    assert_eq!(evidence(&store, &owner, 1, TOOL), before);
    assert_eq!(must(store.lock()).mcp_history_stats(&owner).unresolved, 1);
    assert_eq!(
        must(store.lock())
            .mcp_history_stats(&owner)
            .retained_terminal,
        0
    );
    must(std::fs::set_permissions(
        &root.0,
        std::fs::Permissions::from_mode(0o700),
    ));
    must(auth.mcp_settle_effect(&control, &handle, Some(b"settled")));
    assert_eq!(must(store.lock()).mcp_history_stats(&owner).unresolved, 0);
    assert_eq!(
        must(store.lock())
            .mcp_history_stats(&owner)
            .retained_terminal,
        1
    );
    drop(auth);
    drop(control);
    drop(store);
    let durable = must(Store::open(&root.0));
    let recovered = must(mcp_store_evidence_for_tool(
        &durable,
        &owner,
        identity(1, TOOL).idempotency_key,
        Some(TOOL),
    ))
    .unwrap_or_else(|| panic!("missing retried settlement"));
    assert_eq!(recovered["phase"], "settled");
    assert_eq!(events(&recovered, "effect_settled"), 1);
    assert_eq!(durable.mcp_history_stats(&owner).unresolved, 0);
}

#[test]
fn mcp_history_scan_generation_index_pins_old_effects_without_revisiting_them() {
    let root = Root::new();
    let owner = tenant("generation-owner");
    let mut durable = must(Store::open(&root.0));
    let mut sessions = SessionRegistry::default();
    let token = enroll(&mut durable, &mut sessions, &owner, 1);
    let coordinate = must(TenantKey::new(
        owner.clone(),
        ObjectKind::Configuration,
        b"history-coordinate".to_vec(),
    ));
    must(durable.put_local(coordinate.clone(), b"generation-1".to_vec()));
    let store = Arc::new(Mutex::new(durable));
    let control = controller(&store, sessions);
    let auth = permit(&control, &token);
    let old_effect = begin(&auth, &control, 1);
    must(auth.mcp_start_effect(&control, &old_effect));
    let companion = must(TenantKey::new(
        owner.clone(),
        ObjectKind::Configuration,
        b"history-transition".to_vec(),
    ));
    let (new_token, _) = must(control.restrict_with_companion(
        &owner,
        ScopeRestriction {
            session_id: SessionId([1; 32]),
            token_id: [8; 32],
            scopes: BTreeSet::from(["read".to_owned()]),
            permitted_activity_types: BTreeSet::from([5]),
        },
        HEIGHT + 1,
        (coordinate, b"generation-2".to_vec()),
        (companion, b"narrowed".to_vec()),
    ));
    assert_eq!(
        must(store.lock())
            .mcp_history_stats(&owner)
            .last_transition_candidates,
        1
    );
    let old_evidence = evidence(&store, &owner, 1, TOOL);
    assert_eq!(events(&old_evidence, "session_narrow"), 1);
    let new_auth = permit(&control, &new_token);
    let new_effect = begin(&new_auth, &control, 2);
    must(new_auth.mcp_start_effect(&control, &new_effect));
    must(control.close(&owner, SessionId([1; 32]), HEIGHT + 2));
    assert_eq!(
        must(store.lock())
            .mcp_history_stats(&owner)
            .last_transition_candidates,
        1
    );
    assert_eq!(evidence(&store, &owner, 1, TOOL), old_evidence);
    assert_eq!(
        events(&evidence(&store, &owner, 2, TOOL), "session_close"),
        1
    );
    assert_eq!(must(store.lock()).mcp_history_stats(&owner).unresolved, 2);
    drop(auth);
    drop(new_auth);
    drop(control);
    drop(store);
    let durable = must(Store::open(&root.0));
    assert_eq!(durable.mcp_history_stats(&owner).unresolved, 2);
    assert_eq!(
        must(mcp_store_evidence_for_tool(
            &durable,
            &owner,
            identity(1, TOOL).idempotency_key,
            Some(TOOL)
        )),
        Some(old_evidence)
    );
}
