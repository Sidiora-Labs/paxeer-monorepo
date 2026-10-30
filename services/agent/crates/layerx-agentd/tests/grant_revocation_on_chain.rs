use std::collections::BTreeSet;
use std::fs;

use layerx_agentd::identity::{
    register, CoreIdentity, IdentityError, IdentityResolver, ProtocolAuthority,
};
use layerx_agentd::session::{
    invalidate_on_revocation, open, InvalidationReason, OpenRequest, RevocationEvent, SessionError,
    SessionId, SessionRef, SessionRegistry, Token,
};
use layerx_agentd::store::{Store, TenantId};
use layerx_types::ids::Did;
use layerx_types::verify::VerificationLevel;

struct BoundaryIdentity(CoreIdentity);

impl IdentityResolver for BoundaryIdentity {
    fn resolve(&mut self, _did: &Did) -> Result<Option<CoreIdentity>, IdentityError> {
        Ok(Some(self.0.clone()))
    }
}

fn open_session(
    store: &mut Store,
    sessions: &mut SessionRegistry,
    tenant: &TenantId,
    did: &Did,
    session_byte: u8,
    authority: ProtocolAuthority,
) -> Token {
    let mut boundary = BoundaryIdentity(CoreIdentity {
        canonical_bytes: b"model-identity".to_vec(),
        head_sequence: 1,
        revocation_sequence: 1,
        verification_level: VerificationLevel::STATE_PROVEN,
        frozen: false,
        authorities: vec![authority.clone()],
    });
    let identity = register(store, tenant.clone(), did.clone(), &mut boundary)
        .unwrap_or_else(|error| panic!("identity: {error:?}"));
    let request = OpenRequest {
        session_id: SessionId([session_byte; 32]),
        token_id: [session_byte.wrapping_add(1); 32],
        tenant: tenant.clone(),
        agent: did.clone(),
        authority,
        permitted_activity_types: BTreeSet::from([7]),
        scopes: BTreeSet::from(["prepare".to_owned()]),
        expiry_sequence: 100,
        expiry_seconds: None,
        opening_client: "mcp".to_owned(),
        policy_version: "v1".to_owned(),
    };
    open(store, sessions, &identity, request, 1)
        .unwrap_or_else(|error| panic!("session: {error:?}"))
}

#[test]
fn on_chain_grant_revocation_invalidates_capability_grant_sessions() {
    for (case, authority) in [
        ProtocolAuthority::SessionKey([3; 32]),
        ProtocolAuthority::CapabilityGrant([3; 32]),
    ]
    .into_iter()
    .enumerate()
    {
        let path = std::env::temp_dir().join(format!(
            "layerx-agentd-grant-revocation-on-chain-{}-{case}",
            std::process::id()
        ));
        let mut store = Store::open(&path).unwrap_or_else(|error| panic!("store: {error}"));
        let mut sessions = SessionRegistry::default();
        let tenant = TenantId::new("tenant-a").unwrap_or_else(|error| panic!("tenant: {error}"));
        let did = Did::new(b"did:layerx:model").unwrap_or_else(|error| panic!("DID: {error:?}"));
        let other_did =
            Did::new(b"did:layerx:other").unwrap_or_else(|error| panic!("other DID: {error:?}"));
        let tokens = [
            open_session(
                &mut store,
                &mut sessions,
                &tenant,
                &did,
                1,
                ProtocolAuthority::CapabilityGrant([3; 32]),
            ),
            open_session(
                &mut store,
                &mut sessions,
                &tenant,
                &did,
                2,
                ProtocolAuthority::SessionKey([3; 32]),
            ),
            open_session(
                &mut store,
                &mut sessions,
                &tenant,
                &did,
                3,
                ProtocolAuthority::CapabilityGrant([4; 32]),
            ),
            open_session(
                &mut store,
                &mut sessions,
                &tenant,
                &other_did,
                4,
                ProtocolAuthority::CapabilityGrant([3; 32]),
            ),
        ];
        for token in &tokens {
            assert_eq!(sessions.generation(&tenant, token.session_id()), Some(1));
            assert_eq!(
                token.authorize(&sessions, &tenant, token.agent(), "prepare", 1),
                Ok(token.session_id())
            );
        }

        let report = invalidate_on_revocation(
            &mut store,
            &mut sessions,
            &mut [],
            &RevocationEvent {
                did: did.clone(),
                authority: Some(authority),
                reason: InvalidationReason::GrantRevokedOnChain,
                observed_sequence: 8,
            },
        )
        .unwrap_or_else(|error| panic!("invalidation: {error:?}"));
        assert_eq!(report.reason, InvalidationReason::GrantRevokedOnChain);
        assert_eq!(report.observed_sequence, 8);
        assert_eq!(
            report.invalidated_sessions,
            vec![
                SessionRef::new(tenant.clone(), SessionId([1; 32])),
                SessionRef::new(tenant.clone(), SessionId([2; 32])),
            ]
        );
        assert_eq!(
            report.invalidated_generations,
            report
                .invalidated_sessions
                .iter()
                .cloned()
                .map(|session| (session, 1))
                .collect::<Vec<_>>()
        );
        for token in &tokens[..2] {
            assert_eq!(sessions.generation(&tenant, token.session_id()), Some(2));
            assert!(
                !sessions
                    .get(&tenant, token.session_id())
                    .unwrap_or_else(|| panic!("session missing"))
                    .open
            );
            assert_eq!(
                token.authorize(&sessions, &tenant, &did, "prepare", 8),
                Err(SessionError::Revoked)
            );
        }
        for token in &tokens[2..] {
            assert_eq!(sessions.generation(&tenant, token.session_id()), Some(1));
            assert!(
                sessions
                    .get(&tenant, token.session_id())
                    .unwrap_or_else(|| panic!("session missing"))
                    .open
            );
            assert_eq!(
                token.authorize(&sessions, &tenant, token.agent(), "prepare", 8),
                Ok(token.session_id())
            );
        }
        drop(sessions);
        drop(store);
        fs::remove_dir_all(path).unwrap_or_else(|error| panic!("remove store: {error}"));
    }
}
