use layerx_agentd::identity::{
    register, CoreIdentity, IdentityError, IdentityResolver, ProtocolAuthority,
};
use layerx_agentd::store::{Store, TenantId};
use layerx_types::ids::Did;
use layerx_types::verify::VerificationLevel;
use std::fs;
use std::path::PathBuf;
use std::sync::atomic::{AtomicU64, Ordering};

static NEXT_DIRECTORY: AtomicU64 = AtomicU64::new(1);

struct BoundaryIdentityLedger {
    result: Result<Option<CoreIdentity>, IdentityError>,
    calls: usize,
}

impl IdentityResolver for BoundaryIdentityLedger {
    fn resolve(&mut self, _did: &Did) -> Result<Option<CoreIdentity>, IdentityError> {
        self.calls += 1;
        match &self.result {
            Ok(value) => Ok(value.clone()),
            Err(IdentityError::BoundaryUnavailable) => Err(IdentityError::BoundaryUnavailable),
            Err(IdentityError::UnknownDid) => Err(IdentityError::UnknownDid),
            Err(IdentityError::Frozen) => Err(IdentityError::Frozen),
            Err(IdentityError::Unverified) => Err(IdentityError::Unverified),
            Err(IdentityError::Store(_)) => panic!("boundary ledger cannot return a store error"),
        }
    }
}

fn directory(label: &str) -> PathBuf {
    let sequence = NEXT_DIRECTORY.fetch_add(1, Ordering::Relaxed);
    std::env::temp_dir().join(format!(
        "layerx-agentd-identity-{label}-{}-{sequence}",
        std::process::id()
    ))
}

fn did() -> Did {
    match Did::new(b"did:layerx:agent-7") {
        Ok(value) => value,
        Err(error) => panic!("DID fixture invalid: {error:?}"),
    }
}

fn tenant() -> TenantId {
    match TenantId::new("tenant-a") {
        Ok(value) => value,
        Err(error) => panic!("tenant fixture invalid: {error}"),
    }
}

fn active_identity(head_sequence: u64) -> CoreIdentity {
    CoreIdentity {
        canonical_bytes: format!("identity-at-{head_sequence}").into_bytes(),
        head_sequence,
        revocation_sequence: 1,
        verification_level: VerificationLevel::STATE_PROVEN,
        frozen: false,
        authorities: vec![ProtocolAuthority::SessionKey([7; 32])],
    }
}

#[test]
fn registration_resolves_core_state_and_persists_provenance() {
    let root = directory("register");
    let mut store = Store::open(&root).unwrap_or_else(|error| panic!("open failed: {error}"));
    let mut boundary = BoundaryIdentityLedger {
        result: Ok(Some(active_identity(41))),
        calls: 0,
    };
    let record = register(&mut store, tenant(), did(), &mut boundary)
        .unwrap_or_else(|error| panic!("registration failed: {error:?}"));
    assert_eq!(boundary.calls, 1);
    assert_eq!(record.head_sequence(), 41);
    assert_eq!(record.verification_level(), VerificationLevel::STATE_PROVEN);
    assert_eq!(
        record.authorities(),
        &[ProtocolAuthority::SessionKey([7; 32])]
    );
    assert_eq!(record.canonical_core_bytes(), b"identity-at-41");
    let _ = fs::remove_dir_all(root);
}

#[test]
fn unknown_frozen_unverified_and_unavailable_identities_are_refused() {
    let cases = [
        (Ok(None), IdentityError::UnknownDid),
        (
            Ok(Some(CoreIdentity {
                frozen: true,
                ..active_identity(1)
            })),
            IdentityError::Frozen,
        ),
        (
            Ok(Some(CoreIdentity {
                verification_level: VerificationLevel::UNVERIFIED,
                ..active_identity(1)
            })),
            IdentityError::Unverified,
        ),
        (
            Err(IdentityError::BoundaryUnavailable),
            IdentityError::BoundaryUnavailable,
        ),
    ];
    for (index, (result, expected)) in cases.into_iter().enumerate() {
        let root = directory(&format!("refusal-{index}"));
        let mut store = Store::open(&root).unwrap_or_else(|error| panic!("open failed: {error}"));
        let mut boundary = BoundaryIdentityLedger { result, calls: 0 };
        assert_eq!(
            register(&mut store, tenant(), did(), &mut boundary),
            Err(expected)
        );
        let _ = fs::remove_dir_all(root);
    }
}

#[test]
fn restored_binding_is_revalidated_and_freeze_is_observed() {
    let root = directory("revalidate");
    let peer = crate::repair_support::peer();
    let authorities = [ProtocolAuthority::SessionKey([7; 32])];
    {
        let mut store = lint_success(Store::open(&root), "initial store");
        let (mut authority, worker) = crate::repair_support::authority_peer(vec![(
            200,
            crate::repair_support::identity_document(10, false, &authorities),
        )]);
        let mut resolver = crate::repair_support::AuthorityIdentity {
            authority: &mut authority,
            peer: &peer,
        };
        let record = lint_success(
            register(&mut store, tenant(), did(), &mut resolver),
            "startup registration",
        );
        assert_eq!(record.head_sequence(), 10);
        lint_success(worker.join(), "initial authority");
    } // No pre-restart record, resolver, authority or Store survives.
    {
        let mut store = lint_success(Store::open(&root), "restart store");
        let (mut authority, worker) = crate::repair_support::authority_peer(vec![(
            200,
            crate::repair_support::identity_document(11, false, &authorities),
        )]);
        let mut resolver = crate::repair_support::AuthorityIdentity {
            authority: &mut authority,
            peer: &peer,
        };
        // main.rs::enrol_mcp_agent uses current core_identity followed by register at startup.
        let refreshed = lint_success(
            register(&mut store, tenant(), did(), &mut resolver),
            "fresh startup registration",
        );
        assert_eq!(refreshed.head_sequence(), 11);
        assert_eq!(refreshed.canonical_core_bytes(), b"verified-identity-at-11");
        assert_eq!(refreshed.authorities(), &authorities);
        assert_eq!(refreshed.verification_level(), VerificationLevel::COMMITTED);
        lint_success(worker.join(), "refreshed authority");
    }
    for (status, document, expected) in [
        (
            200,
            crate::repair_support::identity_document(12, true, &authorities),
            IdentityError::Frozen,
        ),
        (
            404,
            serde_json::json!({"error":"unknown_identity"}),
            IdentityError::Unverified,
        ),
    ] {
        let mut store = lint_success(Store::open(&root), "restart refusal store");
        let key = lint_success(
            layerx_agentd::store::TenantKey::new(
                tenant(),
                layerx_agentd::store::ObjectKind::Identity,
                did().as_bytes().to_vec(),
            ),
            "identity key",
        );
        let before = lint_present(store.get(&key), "durable binding")
            .bytes()
            .to_vec();
        let (mut authority, worker) =
            crate::repair_support::authority_peer(vec![(status, document)]);
        let mut resolver = crate::repair_support::AuthorityIdentity {
            authority: &mut authority,
            peer: &peer,
        };
        assert_eq!(
            register(&mut store, tenant(), did(), &mut resolver),
            Err(expected)
        );
        assert_eq!(
            lint_present(store.get(&key), "unchanged binding").bytes(),
            before
        );
        lint_success(worker.join(), "refused authority");
    }
    lint_success(fs::remove_dir_all(root), "remove identity store");
}

#[track_caller]
fn lint_success<T, E: std::fmt::Debug>(result: Result<T, E>, context: &str) -> T {
    match result {
        Ok(value) => value,
        Err(error) => panic!("{context}: {error:?}"),
    }
}

#[track_caller]
fn lint_present<T>(value: Option<T>, context: &str) -> T {
    match value {
        Some(value) => value,
        None => panic!("{context}: required value is absent"),
    }
}
