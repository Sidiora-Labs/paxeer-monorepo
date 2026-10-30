mod canonical_owner {
    use std::fmt::Debug;

    use ed25519_dalek::{Signer as _, SigningKey};
    use layerx_crypto::SignatureMessage;
    use layerx_intents::canonical;
    use layerx_types::activity::{Authority, EnvelopeBuilder, Signature, TimestampBound};
    use layerx_types::amount::Amount;
    use layerx_types::ids::{Did, IdempotencyKey};
    use layerx_types::payload::{ModuleRegistration, ModuleRegistry, Payload};

    pub struct SignedOwner {
        pub bytes: Vec<u8>,
        pub id: [u8; 32],
        pub public_key: [u8; 32],
        pub actor: Vec<u8>,
    }

    fn checked<T, E: Debug>(value: Result<T, E>) -> T {
        value.unwrap_or_else(|error| panic!("canonical owner encoding: {error:?}"))
    }

    pub fn sign(
        payload: &Payload,
        action_key: [u8; 32],
        version: u16,
        network: u32,
    ) -> SignedOwner {
        let signer = SigningKey::from_bytes(&[0x68; 32]);
        let public_key = signer.verifying_key().to_bytes();
        let actor = b"did:layerx:owner-receipt-fixture".to_vec();
        let mut builder = EnvelopeBuilder::new();
        checked(builder.protocol_version(version));
        checked(builder.network_id(network));
        checked(builder.activity_type(payload.activity_type()));
        checked(builder.actor_did(checked(Did::new(&actor))));
        checked(builder.authority(checked(Authority::owner(&public_key))));
        checked(builder.account_sequence(1));
        checked(builder.timestamp_bound(checked(TimestampBound::new(900, 2000))));
        checked(builder.idempotency_key(IdempotencyKey::new(action_key)));
        checked(builder.fee_limit(Amount::from_u128(10)));
        checked(builder.payload_hash(checked(canonical::payload_hash_for(payload))));
        checked(builder.payload(payload.clone()));
        let unsigned = checked(builder.build());
        let canonical = checked(canonical::unsigned_envelope_bytes(&unsigned));
        let digest = checked(SignatureMessage::new(
            canonical::Domain::SignaturePreimage,
            version,
            network,
            &canonical,
        ))
        .digest();
        let signature = signer.sign(&digest).to_bytes();
        let bytes = checked(canonical::signed_envelope_bytes(
            &unsigned.attach_signature(checked(Signature::new(&signature))),
        ));
        let registry = checked(ModuleRegistry::new(&[checked(ModuleRegistration::new(
            payload.activity_type().module(),
            &[payload.activity_type()],
        ))]));
        let decoded = checked(canonical::decode_signed_activity(&bytes, &registry));
        SignedOwner {
            id: checked(canonical::activity_id(&decoded)),
            bytes,
            public_key,
            actor,
        }
    }
}
mod control_fixture {
    use super::canonical_owner;
    use layerx_types::rung::Rung;

    use layerx_human_test_support as support;

    use std::collections::{BTreeMap, BTreeSet};
    use std::fs;

    use ed25519_dalek::{Signer as _, SigningKey};
    use layerx_agentd::budget::{
        reserve, BudgetLimiter, LimitConfig, LimitId, LimitRefusal, LimitScope, LocalLimit,
        ReservationRequest,
    };
    use layerx_agentd::identity::{
        register, CoreIdentity, IdentityError, IdentityResolver, ProtocolAuthority,
    };
    use layerx_agentd::session::{
        close, invalidate_on_revocation, open, InvalidationReason, OpenRequest, RevocationEvent,
        SessionId, SessionRegistry, Token,
    };
    use layerx_agentd::sign::ProvisionedSessionKey;
    use layerx_agentd::store::{Store, TenantId};
    use layerx_crypto::local::LocalSigner;
    use layerx_crypto::session::IssuedSessionKey;
    use layerx_crypto::signer::Signer as _;
    use layerx_human_service::agents::{
        AgentControlAgent, AgentControlError, AgentControlProfile, AgentControlState,
        AgentControls, AppLimitEvidence, AppLimitRequest, CapabilityProvision, LimitBacking,
        LimitChangeRequest, LimitEnforcement, ProtocolAction, ProtocolEvidence,
        SessionControlAdapter, APP_LIMIT_COPY_KEY, APP_LIMIT_EXPLANATION, PAUSE_CONSEQUENCE,
        PAUSE_CONSEQUENCE_COPY_KEY, PROTOCOL_LIMIT_COPY_KEY,
    };
    use layerx_human_service::agents::{
        AgentCreationContract, AgentEvidence, AgentFailure, SessionProvision,
    };
    use layerx_human_service::custody::{
        AgentContractError, AgentSessionContract, AgentSessionProvision, ProtocolIdentitySnapshot,
        ProvisionEvidence, RevocationEvidence, RevocationReason, RotationEvidence,
        RotationObservation, RotationSubmission, SessionEntropySource, SessionKeyEntropy,
        SessionKeyError, SessionKeyProvisioner, SessionPolicy, SessionTarget, SuspensionEvidence,
    };
    use layerx_human_service::server::agent_creation::ProductionAgentCreation;
    use layerx_human_service::store::PrincipalId;
    use layerx_intents::canonical::STATE_COMMITMENT_PROTOCOL_VERSION;
    use layerx_intents::{DisclosureCheck, IntentKind};
    use layerx_types::account::AccountId;
    use layerx_types::ids::{AssetId, Did};
    use layerx_types::intent::PurposeHash;
    use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};
    use layerx_types::verify::VerificationLevel;
    use sha2::{Digest as _, Sha256};

    use support::directory;

    const ASSET: [u8; 32] = [0x22; 32];
    const AGENT_ID: [u8; 32] = [0x31; 32];

    struct IdentityBoundary(CoreIdentity);

    impl IdentityResolver for IdentityBoundary {
        fn resolve(&mut self, _did: &Did) -> Result<Option<CoreIdentity>, IdentityError> {
            Ok(Some(self.0.clone()))
        }
    }

    struct InstalledSession {
        _issued: IssuedSessionKey,
        _signer: ProvisionedSessionKey,
        token: Token,
    }

    /// In-process agent contract over the actual agentd identity, session,
    /// revocation, protocol-budget, local-limiter, and durable store code paths.
    struct RealAgentLayer {
        root: std::path::PathBuf,
        store: Store,
        sessions: SessionRegistry,
        tenant: TenantId,
        principal: PrincipalId,
        did: Did,
        primary_public_key: [u8; 32],
        protocol_identity: [u8; 32],
        revocation_sequence: u64,
        now: u64,
        core_sequence: u64,
        installed: BTreeMap<[u8; 32], InstalledSession>,
        protocol_cache: BTreeMap<[u8; 32], ProtocolEvidence>,
        protocol_limit_effects: usize,
        app_limit_effects: usize,
        app_limiter: Option<(BudgetLimiter, LimitId)>,
        local_limit: Option<LocalLimit>,
    }

    impl RealAgentLayer {
        fn new(
            root: std::path::PathBuf,
            principal: PrincipalId,
            did: Did,
            primary_public_key: [u8; 32],
        ) -> Self {
            fs::create_dir_all(&root).unwrap_or_else(|error| panic!("agent root: {error}"));
            Self {
                store: Store::open(&root).unwrap_or_else(|error| panic!("agent store: {error}")),
                sessions: SessionRegistry::default(),
                tenant: TenantId::new("tenant-controls")
                    .unwrap_or_else(|error| panic!("tenant: {error}")),
                principal,
                protocol_identity: Sha256::digest(did.as_bytes()).into(),
                did,
                primary_public_key,
                revocation_sequence: 1,
                now: 1_000,
                core_sequence: 10,
                installed: BTreeMap::new(),
                protocol_cache: BTreeMap::new(),
                protocol_limit_effects: 0,
                app_limit_effects: 0,
                app_limiter: None,
                local_limit: None,
                root,
            }
        }

        fn require_scope(
            &self,
            principal: &PrincipalId,
            did: &Did,
        ) -> Result<(), AgentContractError> {
            if principal == &self.principal && did == &self.did {
                Ok(())
            } else {
                Err(AgentContractError::Refused("wrong principal"))
            }
        }

        fn daemon_open(&self, grant_id: [u8; 32]) -> bool {
            self.sessions
                .get(&self.tenant, SessionId(grant_id))
                .is_some_and(|record| record.open)
        }

        fn authorizes(&self, grant_id: [u8; 32], scope: &str) -> bool {
            self.installed.get(&grant_id).is_some_and(|installed| {
                installed
                    .token
                    .authorize(
                        &self.sessions,
                        &self.tenant,
                        &self.did,
                        scope,
                        self.core_sequence,
                    )
                    .is_ok()
            })
        }

        fn app_refuses(&self, amount: u128) -> bool {
            let Some((limiter, id)) = &self.app_limiter else {
                return false;
            };
            matches!(
                reserve(
                    limiter,
                    &ReservationRequest {
                        id: [0x91; 32],
                        amount,
                        expiry_sequence: self.core_sequence.saturating_add(10),
                        current_sequence: self.core_sequence,
                        applicable_limits: vec![*id],
                    },
                ),
                Err(LimitRefusal::Exceeded { .. })
            )
        }
    }

    impl Drop for RealAgentLayer {
        fn drop(&mut self) {
            let _ = fs::remove_dir_all(&self.root);
        }
    }

    impl AgentSessionContract for RealAgentLayer {
        fn identity_snapshot(
            &mut self,
            principal: &PrincipalId,
            did: &Did,
        ) -> Result<ProtocolIdentitySnapshot, AgentContractError> {
            self.require_scope(principal, did)?;
            Ok(ProtocolIdentitySnapshot {
                protocol_identity: self.protocol_identity,
                primary_public_key: self.primary_public_key,
                revocation_sequence: self.revocation_sequence,
                protocol_time: self.now,
                core_sequence: self.core_sequence,
                rung: Rung::Committed,
            })
        }

        fn provision_session(
            &mut self,
            request: AgentSessionProvision,
        ) -> Result<ProvisionEvidence, AgentContractError> {
            let (principal, did, issued, scopes, policy_version, secret) = request.into_parts();
            self.require_scope(&principal, &did)?;
            let grant_id = issued.grant_id;
            let session_public_key = issued.session_public_key;
            let signer = ProvisionedSessionKey::new(secret.into_seed(), issued.clone())
                .map_err(|_| AgentContractError::Refused("session key mismatch"))?;
            let authorities = self
                .installed
                .keys()
                .copied()
                .map(ProtocolAuthority::SessionKey)
                .chain(std::iter::once(ProtocolAuthority::SessionKey(grant_id)))
                .collect();
            let mut resolver = IdentityBoundary(CoreIdentity {
                canonical_bytes: self.protocol_identity.to_vec(),
                head_sequence: self.core_sequence,
                revocation_sequence: 1,
                verification_level: VerificationLevel::STATE_PROVEN,
                frozen: false,
                authorities,
            });
            let identity = register(
                &mut self.store,
                self.tenant.clone(),
                self.did.clone(),
                &mut resolver,
            )
            .map_err(|_| AgentContractError::Refused("identity registration failed"))?;
            let token = open(
                &mut self.store,
                &mut self.sessions,
                &identity,
                OpenRequest {
                    session_id: SessionId(grant_id),
                    token_id: Sha256::digest(grant_id).into(),
                    tenant: self.tenant.clone(),
                    agent: self.did.clone(),
                    authority: ProtocolAuthority::SessionKey(grant_id),
                    permitted_activity_types: issued
                        .permitted_activity_types
                        .iter()
                        .map(|activity| activity.ordinal())
                        .collect::<BTreeSet<_>>(),
                    scopes: scopes.into_iter().collect(),
                    expiry_sequence: issued.expires_at,
                    expiry_seconds: None,
                    opening_client: "layerx-human-service".to_owned(),
                    policy_version,
                },
                self.core_sequence,
            )
            .map_err(|_| AgentContractError::Refused("daemon session open failed"))?;
            self.installed.insert(
                grant_id,
                InstalledSession {
                    _issued: issued,
                    _signer: signer,
                    token,
                },
            );
            let mut evidence = ProvisionEvidence {
                grant_id,
                session_public_key,
                daemon_session_id: grant_id,
                protocol_sequence: self.core_sequence,
                observed_at: self.now,
                rung: Rung::Receipted,
                receipt_digest: [0; 32],
            };
            evidence.receipt_digest = evidence.expected_digest();
            Ok(evidence)
        }

        fn suspend_permissions(
            &mut self,
            target: &SessionTarget,
            reason: RevocationReason,
            requested_at: u64,
        ) -> Result<SuspensionEvidence, AgentContractError> {
            self.require_scope(&target.principal, &target.agent_did)?;
            close(
                &mut self.store,
                &mut self.sessions,
                &self.tenant,
                SessionId(target.daemon_session_id),
            )
            .map_err(|_| AgentContractError::Refused("daemon close failed"))?;
            self.now = self.now.max(requested_at);
            let mut evidence = SuspensionEvidence {
                grant_id: target.grant_id,
                daemon_session_id: target.daemon_session_id,
                reason,
                observed_at: self.now,
                receipt_digest: [0; 32],
            };
            evidence.receipt_digest = evidence.expected_digest();
            Ok(evidence)
        }

        fn revoke_protocol_session(
            &mut self,
            target: &SessionTarget,
            reason: RevocationReason,
            requested_at: u64,
        ) -> Result<RevocationEvidence, AgentContractError> {
            self.require_scope(&target.principal, &target.agent_did)?;
            self.core_sequence = self.core_sequence.saturating_add(1);
            self.revocation_sequence = self.revocation_sequence.saturating_add(1);
            invalidate_on_revocation(
                &mut self.store,
                &mut self.sessions,
                &mut [],
                &RevocationEvent {
                    did: self.did.clone(),
                    authority: Some(ProtocolAuthority::SessionKey(target.grant_id)),
                    reason: InvalidationReason::SessionKeyRevoked,
                    observed_sequence: self.core_sequence,
                },
            )
            .map_err(|_| AgentContractError::Refused("protocol invalidation failed"))?;
            self.now = self.now.max(requested_at).saturating_add(1);
            let mut evidence = RevocationEvidence {
                grant_id: target.grant_id,
                reason,
                observed_sequence: self.core_sequence,
                observed_at: self.now,
                rung: Rung::Receipted,
                receipt_digest: [0; 32],
            };
            evidence.receipt_digest = evidence.expected_digest();
            Ok(evidence)
        }

        fn announce_rotation(
            &mut self,
            _submission: RotationSubmission,
        ) -> Result<RotationEvidence, AgentContractError> {
            Err(AgentContractError::Refused(
                "rotation is outside this contract",
            ))
        }

        fn rotation_observation(
            &mut self,
            _principal: &PrincipalId,
            _did: &Did,
        ) -> Result<RotationObservation, AgentContractError> {
            Err(AgentContractError::Refused(
                "rotation is outside this contract",
            ))
        }
    }

    impl AgentCreationContract for RealAgentLayer {
        fn submit_protocol(
            &mut self,
            action: ProtocolAction,
        ) -> Result<ProtocolEvidence, AgentFailure> {
            if let Some(existing) = self.protocol_cache.get(&action.action_key) {
                return Ok(existing.clone());
            }
            if !matches!(action.intent.kind(), IntentKind::BudgetCreate(_))
                || DisclosureCheck::verify(&action.intent, &action.compiled)
                    != Ok(action.disclosure.clone())
                || action.compiled.payload().as_bytes() != action.disclosure.canonical_payload()
            {
                return Err(AgentFailure::Refused("typed limit intent mismatch"));
            }
            let receipt =
                protocol_receipt_for_payload(action.action_key, action.compiled.payload());
            receipt
                .verify_outcome(action.compiled.activity_type())
                .map_err(|_| AgentFailure::Refused("receipt verification failed"))?;
            decoded_limit(action.intent.kind())?;
            let finalization = ProductionAgentCreation::finalization_evidence(
                &receipt,
                ModuleId::Budget,
                1,
                self.now,
            )?;
            if finalization.action_key != action.action_key
                || finalization.activity_id != receipt.tx_hash
                || finalization.observed_sequence != 11
                || finalization.verification != Rung::Committed.wire_rank()
            {
                return Err(AgentFailure::Refused(
                    "protocol budget finalization mismatch",
                ));
            }
            self.protocol_limit_effects = self.protocol_limit_effects.saturating_add(1);
            self.protocol_cache
                .insert(action.action_key, receipt.clone());
            Ok(receipt)
        }

        fn provision_session(
            &mut self,
            _request: SessionProvision,
        ) -> Result<AgentEvidence, AgentFailure> {
            Err(AgentFailure::Refused(
                "creation session path is not a control",
            ))
        }

        fn narrow_capability(
            &mut self,
            _request: CapabilityProvision,
        ) -> Result<AgentEvidence, AgentFailure> {
            Err(AgentFailure::Refused(
                "capability creation is not a control",
            ))
        }
    }

    impl layerx_human_service::agents::ScopedAgentCreationContract for RealAgentLayer {}

    impl AgentControlAgent for RealAgentLayer {
        fn apply_app_limit(
            &mut self,
            request: AppLimitRequest,
        ) -> Result<AppLimitEvidence, AgentFailure> {
            let mut id_bytes = [0_u8; 16];
            id_bytes.copy_from_slice(&request.action_key[..16]);
            let id = LimitId(id_bytes);
            let local = LocalLimit::new(
                self.tenant.clone(),
                request.action_key,
                request.asset,
                request.ceiling,
            );
            let limiter = BudgetLimiter::new(vec![LimitConfig {
                id,
                name: "managed agent monthly app limit".to_owned(),
                scope: LimitScope::Agent(request.agent_id),
                ceiling: request.ceiling,
                consumed: 0,
            }])
            .map_err(|_| AgentFailure::Refused("app limit configuration failed"))?;
            self.local_limit = Some(local);
            self.app_limiter = Some((limiter, id));
            self.app_limit_effects = self.app_limit_effects.saturating_add(1);
            let mut evidence = AppLimitEvidence {
                action_key: request.action_key,
                agent_id: request.agent_id,
                asset: request.asset,
                ceiling: request.ceiling,
                observed_sequence: self.core_sequence,
                configuration_digest: [0; 32],
            };
            evidence.configuration_digest = evidence.expected_digest();
            Ok(evidence)
        }
    }

    fn decoded_limit(intent: &IntentKind) -> Result<u128, AgentFailure> {
        let IntentKind::BudgetCreate(value) = intent else {
            return Err(AgentFailure::Refused("limit intent missing"));
        };
        Ok(value.per_period_limit().value())
    }

    struct RealEntropy;

    impl SessionEntropySource for RealEntropy {
        fn next_session_entropy(&mut self) -> Result<SessionKeyEntropy, SessionKeyError> {
            let mut seed = [0_u8; 32];
            getrandom::fill(&mut seed).map_err(|_| SessionKeyError::EntropyUnavailable)?;
            SessionKeyEntropy::new(seed)
        }
    }

    fn principal() -> PrincipalId {
        PrincipalId::new("alice").unwrap_or_else(|error| panic!("principal: {error}"))
    }

    fn did() -> Did {
        Did::new(b"did:layerx:controlled")
            .unwrap_or_else(|error| panic!("controlled DID: {error:?}"))
    }

    fn activity(module: ModuleId, ordinal: u16) -> ActivityType {
        ActivityType::new(module, ordinal).unwrap_or_else(|error| panic!("activity: {error:?}"))
    }

    fn registry() -> ModuleRegistry {
        let asset = ModuleRegistration::new(ModuleId::Asset, &[activity(ModuleId::Asset, 5)])
            .unwrap_or_else(|error| panic!("asset registry: {error:?}"));
        let budget = ModuleRegistration::new(ModuleId::Budget, &[activity(ModuleId::Budget, 1)])
            .unwrap_or_else(|error| panic!("budget registry: {error:?}"));
        ModuleRegistry::new(&[asset, budget])
            .unwrap_or_else(|error| panic!("module registry: {error:?}"))
    }

    fn session_policy() -> SessionPolicy {
        SessionPolicy::new(
            1_000,
            100,
            3,
            vec!["prepare".to_owned(), "submit".to_owned()],
            "managed-agent-controls-v1",
        )
        .unwrap_or_else(|error| panic!("session policy: {error}"))
    }

    fn profile() -> AgentControlProfile {
        AgentControlProfile {
            principal: principal(),
            agent_id: AGENT_ID,
            did: did(),
            owner: AccountId::parse("agent:did:layerx:alice:main")
                .unwrap_or_else(|error| panic!("owner: {error:?}")),
            budget_account: AccountId::parse("agent:did:layerx:controlled:budget:31313131")
                .unwrap_or_else(|error| panic!("budget account: {error:?}")),
            asset: AssetId::new(ASSET),
            purpose: PurposeHash::new([0x51; 32]),
            period_seconds: 2_592_000,
            budget_lifetime_seconds: 31_536_000,
            freshness_seconds: 5,
            initial_limit_evidence_digest: [0x52; 32],
        }
    }

    type Controls = AgentControls<SessionControlAdapter<RealAgentLayer, RealEntropy>>;

    fn controls(label: &str, backing: LimitBacking) -> (Controls, [u8; 32]) {
        let root = directory(label);
        let primary_public = LocalSigner::new([0x11; 32]).public_key();
        let layer = RealAgentLayer::new(root, principal(), did(), primary_public);
        let mut sessions =
            SessionKeyProvisioner::new(layer, RealEntropy, session_policy(), registry());
        let lease = sessions
            .provision(&principal(), &did(), vec![activity(ModuleId::Asset, 5)])
            .unwrap_or_else(|error| panic!("initial session: {error}"));
        let controls = AgentControls::new(
            SessionControlAdapter::new(sessions),
            profile(),
            1_000,
            backing,
            1_000,
        )
        .unwrap_or_else(|error| panic!("controls: {error}"));
        (controls, lease.grant_id)
    }

    #[test]
    fn pause_revokes_real_authority_promptly_and_resume_is_receipt_gated() {
        let (mut controls, first_grant) = controls(
            "agent-controls-authority",
            LimitBacking::Protocol {
                active_budget_id: AGENT_ID,
            },
        );
        assert!(controls
            .boundary()
            .sessions()
            .contract()
            .authorizes(first_grant, "prepare"));

        let paused = controls
            .pause(1_005)
            .unwrap_or_else(|error| panic!("pause: {error}"));
        assert_eq!(paused.state, AgentControlState::Paused);
        assert_eq!(
            paused.pause_consequence_copy_key,
            Some(PAUSE_CONSEQUENCE_COPY_KEY)
        );
        assert_eq!(paused.pause_consequence, Some(PAUSE_CONSEQUENCE));
        let outcome_lease = controls
            .boundary()
            .sessions()
            .session(&principal(), &did())
            .unwrap_or_else(|| panic!("paused lease missing"));
        let latency = match outcome_lease.state {
            layerx_human_service::custody::SessionLeaseState::Revoked {
                suspended_at,
                revoked_at,
                ..
            } => revoked_at.saturating_sub(suspended_at),
            _ => panic!("pause did not revoke the session"),
        };
        assert!(latency <= session_policy().maximum_revocation_latency_seconds());
        assert!(!controls
            .boundary()
            .sessions()
            .contract()
            .daemon_open(first_grant));
        let resumed = controls
            .resume(1_007)
            .unwrap_or_else(|error| panic!("resume: {error}"));
        assert_eq!(resumed.state, AgentControlState::Active);
        assert!(resumed.pause_consequence.is_none());
        let replacement = controls
            .boundary()
            .sessions()
            .session(&principal(), &did())
            .unwrap_or_else(|| panic!("replacement lease missing"));
        assert_ne!(replacement.grant_id, first_grant);
        assert!(controls
            .boundary()
            .sessions()
            .contract()
            .authorizes(replacement.grant_id, "submit"));
    }

    #[test]
    fn protocol_and_app_limit_changes_keep_their_real_enforcement_labels() {
        let mut unfinalized = protocol_receipt([0x60; 32]);
        unfinalized.rung = Rung::Receipted;
        assert!(matches!(
            ProductionAgentCreation::finalization_evidence(
                &unfinalized,
                ModuleId::Budget,
                1,
                1_002,
            ),
            Err(AgentFailure::Refused(
                "lifecycle receipt is not checkpoint-finalized"
            ))
        ));
        let finalized = protocol_receipt([0x62; 32]);
        assert!(matches!(
            ProductionAgentCreation::finalization_evidence(
                &finalized,
                ModuleId::Governance,
                1,
                1_002,
            ),
            Err(AgentFailure::Refused(
                "lifecycle receipt does not prove successful action"
            ))
        ));
        assert!(matches!(
            ProductionAgentCreation::finalization_evidence(&finalized, ModuleId::Budget, 7, 1_002,),
            Err(AgentFailure::Refused(
                "lifecycle receipt does not prove successful action"
            ))
        ));

        let (mut protocol, _) = controls(
            "agent-controls-protocol-limit",
            LimitBacking::Protocol {
                active_budget_id: AGENT_ID,
            },
        );
        let request = LimitChangeRequest::new([0x61; 32], 750)
            .unwrap_or_else(|error| panic!("protocol request: {error}"));
        let changed = protocol
            .change_limit(Rung::Committed, &registry(), request, 1_002)
            .unwrap_or_else(|error| panic!("protocol limit: {error}"));
        assert_eq!(changed.limit.monthly, 750);
        assert_eq!(changed.limit.enforcement, LimitEnforcement::Protocol);
        assert_eq!(changed.limit.enforcement_copy_key, PROTOCOL_LIMIT_COPY_KEY);
        assert_eq!(changed.limit.verification_level, Rung::Committed);
        assert_eq!(
            protocol
                .boundary()
                .sessions()
                .contract()
                .protocol_limit_effects,
            1
        );
        let repeated = protocol
            .change_limit(Rung::Committed, &registry(), request, 1_003)
            .unwrap_or_else(|error| panic!("repeat protocol limit: {error}"));
        assert_eq!(repeated, changed);
        assert_eq!(
            protocol
                .boundary()
                .sessions()
                .contract()
                .protocol_limit_effects,
            1
        );

        let (mut app, _) = controls("agent-controls-app-limit", LimitBacking::App);
        let changed = app
            .change_limit(
                Rung::Committed,
                &registry(),
                LimitChangeRequest::new([0x71; 32], 100)
                    .unwrap_or_else(|error| panic!("app request: {error}")),
                1_002,
            )
            .unwrap_or_else(|error| panic!("app limit: {error}"));
        assert_eq!(changed.limit.enforcement, LimitEnforcement::App);
        assert_eq!(changed.limit.enforcement_copy_key, APP_LIMIT_COPY_KEY);
        assert_eq!(changed.limit.explanation, APP_LIMIT_EXPLANATION);
        assert!(!changed.limit.explanation.contains("protocol-enforced"));
        assert!(changed
            .limit
            .explanation
            .contains("not a protocol guarantee"));
        let layer = app.boundary().sessions().contract();
        assert_eq!(layer.app_limit_effects, 1);
        assert!(layer.app_refuses(101));
        let local = layer
            .local_limit
            .as_ref()
            .unwrap_or_else(|| panic!("local limit missing"));
        assert_eq!(local.enforcement, "daemon-enforced");
        assert!(local
            .bypass_statement
            .contains("bypassing layerx-agentd bypasses"));

        assert!(matches!(
            app.view(1_008),
            Err(AgentControlError::Stale { .. })
        ));
    }

    #[derive(Clone)]
    struct ReceiptFields {
        activity_id: [u8; 32],
        previous_state_root: [u8; 32],
        resulting_state_root: [u8; 32],
        batch_id: [u8; 32],
    }

    fn protocol_receipt(action_key: [u8; 32]) -> ProtocolEvidence {
        let profile = profile();
        let intent = layerx_intents::Intent::v1(IntentKind::BudgetCreate(
            layerx_intents::BudgetCreate::new(
                layerx_types::intent::BudgetId::new(action_key),
                profile.owner,
                profile.budget_account,
                profile.asset,
                layerx_types::amount::Amount::from_u128(500),
                layerx_types::intent::PeriodLength::new(profile.period_seconds)
                    .unwrap_or_else(|error| panic!("period: {error:?}")),
                layerx_types::intent::RolloverPolicy::None,
                layerx_types::amount::Amount::ZERO,
                profile.purpose,
                layerx_types::intent::TimestampSeconds::from_u64(2000),
            )
            .unwrap_or_else(|error| panic!("budget intent: {error:?}")),
        ));
        let compiled = layerx_intents::compile(&intent, &registry())
            .unwrap_or_else(|error| panic!("budget encoding: {error:?}"));
        protocol_receipt_for_payload(action_key, compiled.payload())
    }

    fn protocol_receipt_for_payload(
        action_key: [u8; 32],
        payload: &layerx_types::payload::Payload,
    ) -> ProtocolEvidence {
        let owner =
            canonical_owner::sign(payload, action_key, STATE_COMMITMENT_PROTOCOL_VERSION, 77);

        let previous_state_root = [0x81; 32];
        let fields = ReceiptFields {
            activity_id: owner.id,
            previous_state_root,
            resulting_state_root: [0x82; 32],
            batch_id: {
                let mut height = [0; 32];
                height[24..].copy_from_slice(&11_u64.to_be_bytes());
                height
            },
        };
        let signer = SigningKey::from_bytes(&[0x84; 32]);
        let unsigned = encode_receipt(&fields, None);
        let mut hasher = Sha256::new();
        hasher.update(b"LXP/v1/receipt\0");
        hasher.update(&unsigned);
        let signature = signer.sign(&<[u8; 32]>::from(hasher.finalize()));
        let receipt_bytes = encode_receipt(&fields, Some(signature.to_bytes()));
        let evidence = ProtocolEvidence {
            signed_activity: owner.bytes,
            actor: owner.actor,
            owner_public_key: owner.public_key,
            network_id: 77,
            action_key,
            activity_id: owner.id,
            receipt_bytes: receipt_bytes.clone(),
            tx_hash: fields.activity_id,
            receipt_hash: Sha256::digest(&receipt_bytes).into(),
            block_height: 11,
            rung: Rung::Committed,
        };
        evidence
            .verify_outcome(payload.activity_type())
            .expect("bound committed transaction receipt");
        evidence
    }

    fn encode_receipt(fields: &ReceiptFields, signature: Option<[u8; 64]>) -> Vec<u8> {
        let mut bytes = Vec::new();
        push_u16(&mut bytes, STATE_COMMITMENT_PROTOCOL_VERSION);
        push_u16(&mut bytes, 0x5201);
        push_u16(&mut bytes, STATE_COMMITMENT_PROTOCOL_VERSION);
        push_bytes(&mut bytes, &fields.activity_id);
        push_u64(&mut bytes, 11);
        push_bytes(&mut bytes, &fields.previous_state_root);
        push_bytes(&mut bytes, &fields.resulting_state_root);
        push_bytes(&mut bytes, &[0x85; 32]);
        bytes.extend_from_slice(&0_i32.to_be_bytes());
        bytes.extend_from_slice(&0_u32.to_be_bytes());
        bytes.extend_from_slice(&1_u128.to_be_bytes());
        push_bytes(&mut bytes, &fields.batch_id);
        push_u16(&mut bytes, ModuleId::Budget as u16);
        bytes.extend_from_slice(&1_u32.to_be_bytes());
        bytes.extend_from_slice(&1_u32.to_be_bytes());
        bytes.push(0);
        push_bytes(&mut bytes, &[0; 32]);
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
        push_u64(&mut bytes, 1_002);
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

    fn push_bytes(bytes: &mut Vec<u8>, value: &[u8]) {
        let length = u32::try_from(value.len()).unwrap_or_else(|_| panic!("field overflow"));
        bytes.extend_from_slice(&length.to_be_bytes());
        bytes.extend_from_slice(value);
    }

    pub fn refuse_uncommitted_reads() {
        protocol_and_app_limit_changes_keep_their_real_enforcement_labels();
        for backing in [
            LimitBacking::App,
            LimitBacking::Protocol {
                active_budget_id: AGENT_ID,
            },
        ] {
            let (mut controls, _) = controls("rung-authority", backing);
            let before = controls.view(1_000).unwrap();
            let request = LimitChangeRequest::new([0x88; 32], 750).unwrap();
            for rung in [Rung::Submitted, Rung::Receipted] {
                assert!(matches!(
                    controls.change_limit(rung, &registry(), request, 1_002),
                    Err(AgentControlError::EvidenceConflict)
                ));
                assert_eq!(controls.view(1_000).unwrap(), before);
                assert_eq!(
                    controls.boundary().sessions().contract().app_limit_effects,
                    0
                );
                assert_eq!(
                    controls
                        .boundary()
                        .sessions()
                        .contract()
                        .protocol_limit_effects,
                    0
                );
            }
            let committed = controls
                .change_limit(Rung::Committed, &registry(), request, 1_002)
                .unwrap();
            assert_eq!(committed.limit.monthly, request.monthly_limit);
            let layer = controls.boundary().sessions().contract();
            let effects = (layer.app_limit_effects, layer.protocol_limit_effects);
            assert_eq!(
                effects,
                match backing {
                    LimitBacking::App => (1, 0),
                    LimitBacking::Protocol { .. } => (0, 1),
                }
            );
            for rung in [Rung::Submitted, Rung::Receipted] {
                assert!(matches!(
                    controls.change_limit(rung, &registry(), request, 1_003),
                    Err(AgentControlError::EvidenceConflict)
                ));
                assert_eq!(controls.view(1_003).unwrap(), committed);
                let layer = controls.boundary().sessions().contract();
                assert_eq!(
                    (layer.app_limit_effects, layer.protocol_limit_effects),
                    effects
                );
            }
        }
        let (mut controls, _) = controls("rung-committed", LimitBacking::App);
        let changed = controls
            .change_limit(
                Rung::Committed,
                &registry(),
                LimitChangeRequest::new([0x89; 32], 750).unwrap(),
                1_002,
            )
            .unwrap();
        assert_eq!(changed.limit.monthly, 750);
        assert_eq!(
            controls.boundary().sessions().contract().app_limit_effects,
            1
        );
    }
}
mod archive_fixture {
    use layerx_human_test_support as support;
    use layerx_types::rung::Rung;

    use std::collections::{BTreeMap, BTreeSet};
    use std::fs;
    use std::future::Future;
    use std::pin::pin;
    use std::sync::Arc;
    use std::task::{Context, Poll, Wake, Waker};

    use ed25519_dalek::{Signer as _, SigningKey};
    use layerx_agent_api::idempotency::IdempotentMutation;
    use layerx_agent_api::identity::{AgentDid, AuthorityRef};
    use layerx_agent_api::prepare::{PreparationRef, PrepareRequest as ApiPrepareRequest};
    use layerx_agent_api::submit::SubmitRequest;
    use layerx_agent_api::track::{
        EvidenceRef as AgentEvidenceRef, ReceiptRef, SubmissionRef, SubmissionState, TrackRequest,
        TrackedSubmission,
    };
    use layerx_agentd::outbox::{Outbox, OutboxError, SubmissionState as OutboxState};
    use layerx_agentd::prepare::{
        prepare_activity, CorePreparationBoundary, CorePreparationState, CoreStateError,
        PreparationDefaults, PrepareRequest, Prepared,
    };
    use layerx_agentd::receipt::{self as daemon_receipt, ReceiptLookupKey as DaemonReceiptKey};
    use layerx_agentd::sign::{attach_external_signature, verify_before_submit};
    use layerx_agentd::store::{Store as AgentStore, TenantId};
    use layerx_human_service::custody::{
        CustodySigner, EnvelopeKms, KeyClass, KeyEntropy, KeyId, Keystore, Operation, SigningLimits,
    };
    use layerx_human_service::journeys::{
        AgentBoundary, AgentBoundaryError, AgentObservation, AgentPreparation, JourneyEngine,
        JourneyLeg, JourneyPhase, JourneyPlan, JourneyProgress, JourneyState, ReceiptLookup,
        ReceiptMaterial,
    };
    use layerx_human_service::notify::JourneyId;
    use layerx_human_service::store::{PrincipalId, PrincipalStore, TenancyDigest};
    use layerx_human_service::trace::TraceId;
    use layerx_intents::{Intent, IntentKind, LxpSend};
    use layerx_proof::receipt::AuthorizedBatch;
    use layerx_sdk::{Call, Client as AgentClient};
    use layerx_types::account::AccountId;
    use layerx_types::activity::{Authority, TimestampBound};
    use layerx_types::amount::Amount;
    use layerx_types::ids::{AssetId, Did, IdempotencyKey};
    use layerx_types::intent::{
        AuthorizationSignature, ContextHash, NetworkId, ProtocolVersion, PublicKey,
        SendAuthorization, SendAuthorizationKind, Sequence, TimestampSeconds,
    };
    use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};
    use sha2::{Digest as _, Sha256};

    use support::{directory, principal, retention_uniform, tenancy};

    const NETWORK_ID: u32 = 77;
    const ACCOUNT_SEQUENCE: u64 = 7;

    struct NoopWake;

    impl Wake for NoopWake {
        fn wake(self: Arc<Self>) {}
    }

    fn ready<F: Future>(future: F) -> F::Output {
        let mut future = pin!(future);
        let waker = Waker::from(Arc::new(NoopWake));
        let mut context = Context::from_waker(&waker);
        match future.as_mut().poll(&mut context) {
            Poll::Ready(value) => value,
            Poll::Pending => panic!("journey future unexpectedly blocked"),
        }
    }

    fn activity_type() -> ActivityType {
        ActivityType::new(ModuleId::Asset, 5)
            .unwrap_or_else(|error| panic!("activity type: {error:?}"))
    }

    fn registry() -> ModuleRegistry {
        let asset = ModuleRegistration::new(
            ModuleId::Asset,
            &[
                activity_type(),
                ActivityType::new(ModuleId::Asset, 6)
                    .unwrap_or_else(|error| panic!("receive activity type: {error:?}")),
            ],
        )
        .unwrap_or_else(|error| panic!("asset registration: {error:?}"));
        let budget = ModuleRegistration::new(
            ModuleId::Budget,
            &[ActivityType::new(ModuleId::Budget, 7)
                .unwrap_or_else(|error| panic!("defund activity type: {error:?}"))],
        )
        .unwrap_or_else(|error| panic!("budget registration: {error:?}"));
        ModuleRegistry::new(&[asset, budget])
            .unwrap_or_else(|error| panic!("module registry: {error:?}"))
    }

    fn payload_activity_type(payload: &[u8]) -> Result<ActivityType, AgentBoundaryError> {
        let tag = payload
            .get(..2)
            .and_then(|bytes| <[u8; 2]>::try_from(bytes).ok())
            .map(u16::from_be_bytes)
            .ok_or(AgentBoundaryError::CorruptResponse)?;
        match tag {
            0x5301 => ActivityType::new(ModuleId::Asset, 5),
            0x5201 => ActivityType::new(ModuleId::Asset, 6),
            0x4207 => ActivityType::new(ModuleId::Budget, 7),
            _ => return Err(AgentBoundaryError::Refused),
        }
        .map_err(|_| AgentBoundaryError::CorruptResponse)
    }

    fn account(value: &str) -> AccountId {
        AccountId::parse(value).unwrap_or_else(|error| panic!("account: {error:?}"))
    }

    fn send_intent(public_key: [u8; 32], amount: u128, key: u8) -> Intent {
        let signer = layerx_crypto::local::LocalSigner::new([0x51; 32]);
        assert_eq!(
            layerx_crypto::signer::Signer::public_key(&signer),
            public_key
        );
        let debit = layerx_crypto::send::SendDebit {
            from: layerx_intents::canonical::account_id_for_protocol(
                &account("agent:did:layerx:alice:main"),
                layerx_intents::canonical::PROTOCOL_VERSION,
            )
            .unwrap_or_else(|error| panic!("source account: {error:?}")),
            to: layerx_intents::canonical::account_id_for_protocol(
                &account("agent:did:layerx:recipient:main"),
                layerx_intents::canonical::PROTOCOL_VERSION,
            )
            .unwrap_or_else(|error| panic!("destination account: {error:?}")),
            asset: [0x33; 32],
            amount,
            source_sequence: ACCOUNT_SEQUENCE,
            idempotency_key: [key; 32],
            expires_at: 1_010,
            context_hash: [0x55; 32],
            conditions: Vec::new(),
            authorization_kind: SendAuthorizationKind::Owner as u8,
            network_id: NETWORK_ID,
            protocol_version: layerx_intents::canonical::PROTOCOL_VERSION,
        };

        let send = LxpSend::new(
            account("agent:did:layerx:alice:main"),
            account("agent:did:layerx:recipient:main"),
            AssetId::new([0x33; 32]),
            Amount::from_u128(amount),
            Sequence::from_u64(ACCOUNT_SEQUENCE),
            IdempotencyKey::new([key; 32]),
            TimestampSeconds::from_u64(1_010),
            ContextHash::new([0x55; 32]),
            support::sign_send(
                &signer,
                &debit,
                SendAuthorization::new(
                    SendAuthorizationKind::Owner,
                    PublicKey::new(public_key),
                    AuthorizationSignature::new([0x77; 64]),
                ),
            ),
            NetworkId::new(NETWORK_ID).unwrap_or_else(|error| panic!("network: {error:?}")),
            ProtocolVersion::new(layerx_intents::canonical::PROTOCOL_VERSION)
                .unwrap_or_else(|error| panic!("protocol: {error:?}")),
        )
        .unwrap_or_else(|error| panic!("send intent: {error:?}"));
        Intent::v1(IntentKind::LxpSend(send))
    }

    #[derive(Clone, Copy, Debug, Eq, PartialEq)]
    enum DeliveryMode {
        TrackThenReceipt,
        UnknownThenLookup,
    }

    #[derive(Clone, Copy, Debug, Eq, PartialEq)]
    enum FaultPoint {
        Prepare,
        Submit,
        Track,
        Lookup,
    }

    struct RecordedCore(CorePreparationState);

    impl CorePreparationBoundary for RecordedCore {
        fn preparation_state(
            &mut self,
            _actor: &Did,
        ) -> Result<CorePreparationState, CoreStateError> {
            Ok(self.0.clone())
        }
    }

    /// Test adapter over the real agentd prepare, signature-verification, durable
    /// outbox, and receipt store. Faults are injected only after the named real
    /// operation has completed, reproducing a service crash in the acknowledgement
    /// gap without replacing any production component.
    struct RealAgentLayer {
        store: AgentStore,
        outbox: Outbox,
        tenant: TenantId,
        registry: ModuleRegistry,
        preparations: BTreeMap<[u8; 32], Prepared>,
        preparation_bodies: BTreeMap<[u8; 32], [u8; 32]>,
        observations: BTreeMap<[u8; 32], AgentObservation>,
        receipt_material: BTreeMap<[u8; 32], ReceiptMaterial>,
        submission_keys: BTreeMap<String, [u8; 32]>,
        modes: BTreeMap<[u8; 32], DeliveryMode>,
        effects: BTreeMap<[u8; 32], u32>,
        prepare_calls: BTreeMap<[u8; 32], u32>,
        submit_calls: BTreeMap<[u8; 32], u32>,
        track_calls: BTreeMap<[u8; 32], u32>,
        lookup_calls: BTreeMap<[u8; 32], u32>,
        fault: Option<FaultPoint>,
        fault_fired: bool,
    }

    impl RealAgentLayer {
        fn new(root: &std::path::Path, modes: BTreeMap<[u8; 32], DeliveryMode>) -> Self {
            Self {
                store: AgentStore::open(root)
                    .unwrap_or_else(|error| panic!("agent store: {error}")),
                outbox: Outbox::default(),
                tenant: TenantId::new("tenant-a").unwrap_or_else(|error| panic!("tenant: {error}")),
                registry: registry(),
                preparations: BTreeMap::new(),
                preparation_bodies: BTreeMap::new(),
                observations: BTreeMap::new(),
                receipt_material: BTreeMap::new(),
                submission_keys: BTreeMap::new(),
                modes,
                effects: BTreeMap::new(),
                prepare_calls: BTreeMap::new(),
                submit_calls: BTreeMap::new(),
                track_calls: BTreeMap::new(),
                lookup_calls: BTreeMap::new(),
                fault: None,
                fault_fired: false,
            }
        }

        fn inject(&mut self, fault: FaultPoint) {
            self.fault = Some(fault);
            self.fault_fired = false;
        }

        fn fail_after(&mut self, point: FaultPoint) -> Result<(), AgentBoundaryError> {
            if self.fault == Some(point) && !self.fault_fired {
                self.fault_fired = true;
                Err(AgentBoundaryError::Unavailable)
            } else {
                Ok(())
            }
        }

        fn count(map: &mut BTreeMap<[u8; 32], u32>, key: [u8; 32]) {
            let count = map.entry(key).or_default();
            *count = count.saturating_add(1);
        }

        fn tracked(
            key: [u8; 32],
            state: SubmissionState,
            receipt_digest: Option<[u8; 32]>,
        ) -> TrackedSubmission {
            let evidence = receipt_digest.map_or_else(Vec::new, |digest| {
                vec![AgentEvidenceRef {
                    kind: "sequencer-receipt".to_owned(),
                    digest,
                }]
            });
            let rung = if evidence.is_empty() {
                layerx_types::rung::Rung::Submitted
            } else {
                layerx_types::rung::Rung::Receipted
            };
            TrackedSubmission {
                submission_ref: SubmissionRef::new(format!("sub-{}", hex(&key)))
                    .unwrap_or_else(|error| panic!("submission ref: {error:?}")),
                state,
                evidence,
                rung,
                transitions: Vec::new(),
            }
        }

        fn executed_observation(&self, key: [u8; 32], activity_id: [u8; 32]) -> AgentObservation {
            let material = self
                .receipt_material
                .get(&key)
                .cloned()
                .unwrap_or_else(|| panic!("receipt material missing"));
            let digest: [u8; 32] = Sha256::digest(&material.canonical_bytes).into();
            AgentObservation {
                submission: Self::tracked(
                    key,
                    SubmissionState::Executed {
                        receipt_ref: ReceiptRef::new(format!("rcp-{}", hex(&key)))
                            .unwrap_or_else(|error| panic!("receipt ref: {error:?}")),
                    },
                    Some(digest),
                ),
                activity_id,
                receipt: Some(material),
            }
        }
    }

    impl AgentBoundary for RealAgentLayer {
        fn prepare(
            &mut self,
            call: &Call<IdempotentMutation<ApiPrepareRequest>>,
        ) -> Result<AgentPreparation, AgentBoundaryError> {
            let mutation = call.request();
            let key = mutation.key.bytes();
            Self::count(&mut self.prepare_calls, key);
            if let Some(body) = self.preparation_bodies.get(&key) {
                if *body != mutation.body_digest.0 {
                    return Err(AgentBoundaryError::Refused);
                }
            } else {
                let request = &mutation.operation;
                let activity = payload_activity_type(request.payload.as_bytes())?;
                let mut core = RecordedCore(CorePreparationState {
                    network_id: NETWORK_ID,
                    account_sequence: request.account_sequence.get(),
                    protocol_timestamp: request.timestamp_bound.not_before.get().saturating_add(5),
                    observed_head_sequence: 88,
                    module_registry: self.registry.clone(),
                });
                let prepared = prepare_activity(
                    &mut core,
                    PreparationDefaults {
                        timestamp_span: request
                            .timestamp_bound
                            .not_after
                            .get()
                            .saturating_sub(request.timestamp_bound.not_before.get()),
                        fee_limit: Amount::from_u128(request.fee_limit.get()),
                        maximum_payload_bytes: 1_024,
                    },
                    PrepareRequest {
                        actor: Did::new(request.actor.as_str().as_bytes())
                            .map_err(|_| AgentBoundaryError::CorruptResponse)?,
                        authority: Authority::owner(request.authority.as_str().as_bytes())
                            .map_err(|_| AgentBoundaryError::CorruptResponse)?,
                        activity_type: activity,
                        expected_account_sequence: Some(request.account_sequence.get()),
                        timestamp_bound: Some(
                            TimestampBound::new(
                                request.timestamp_bound.not_before.get(),
                                request.timestamp_bound.not_after.get(),
                            )
                            .map_err(|_| AgentBoundaryError::CorruptResponse)?,
                        ),
                        fee_limit: Some(Amount::from_u128(request.fee_limit.get())),
                        idempotency_key: IdempotencyKey::new(key),
                        payload: request.payload.as_bytes().to_vec(),
                        declared_payload_limit: 1_024,
                    },
                )
                .map_err(|_| AgentBoundaryError::Refused)?;
                self.preparation_bodies.insert(key, mutation.body_digest.0);
                self.preparations.insert(key, prepared);
            }
            let prepared = self
                .preparations
                .get(&key)
                .cloned()
                .ok_or(AgentBoundaryError::CorruptResponse)?;
            let request = &mutation.operation;
            let result = AgentPreparation {
                preparation_ref: PreparationRef::new(format!("prep-{}", hex(&key)))
                    .map_err(|_| AgentBoundaryError::CorruptResponse)?,
                unsigned_canonical_bytes: prepared.canonical_bytes.clone(),
                signing_preimage: prepared.signing_preimage.to_vec(),
                disclosure: prepared.disclosure.clone(),
                actor: request.actor.clone(),
                authority: request.authority.clone(),
                account_sequence: request.account_sequence.get(),
                not_before: request.timestamp_bound.not_before.get(),
                not_after: request.timestamp_bound.not_after.get(),
                fee_limit: request.fee_limit.get(),
                activity_type: prepared.envelope.activity_type(),
                payload: prepared.envelope.payload().as_bytes().to_vec(),
                payload_hash: prepared.envelope.payload_hash(),
                idempotency_key: prepared.envelope.idempotency_key().bytes(),
            };
            self.fail_after(FaultPoint::Prepare)?;
            Ok(result)
        }

        fn submit(
            &mut self,
            call: &Call<IdempotentMutation<SubmitRequest>>,
            signer_public_key: [u8; 32],
        ) -> Result<AgentObservation, AgentBoundaryError> {
            let mutation = call.request();
            let key = mutation.key.bytes();
            Self::count(&mut self.submit_calls, key);
            if let Some(observation) = self.observations.get(&key).cloned() {
                self.fail_after(FaultPoint::Submit)?;
                return Ok(observation);
            }
            let prepared = self
                .preparations
                .get(&key)
                .cloned()
                .ok_or(AgentBoundaryError::CorruptResponse)?;
            if mutation.operation.preparation_ref.as_str() != format!("prep-{}", hex(&key)) {
                return Err(AgentBoundaryError::Refused);
            }
            let signature: [u8; 64] = mutation
                .operation
                .signature
                .as_bytes()
                .try_into()
                .map_err(|_| AgentBoundaryError::Refused)?;
            let signed = attach_external_signature(&prepared, signature)
                .map_err(|_| AgentBoundaryError::Refused)?;
            let verified =
                verify_before_submit(&signed, &prepared, &signer_public_key, &self.registry)
                    .map_err(|_| AgentBoundaryError::Refused)?;
            let activity_id = verified.activity_id();
            match self
                .outbox
                .enqueue(&mut self.store, self.tenant.clone(), key, verified)
            {
                Ok(()) => {}
                Err(OutboxError::Duplicate) => return Err(AgentBoundaryError::CorruptResponse),
                Err(_) => return Err(AgentBoundaryError::Refused),
            }
            self.outbox
                .transition(
                    &mut self.store,
                    key,
                    OutboxState::Submitted,
                    "real transport accepted exact bytes",
                    None,
                )
                .map_err(|_| AgentBoundaryError::Refused)?;
            let receipt = receipt(activity_id, key[0], prepared.envelope.activity_type());
            self.receipt_material.insert(key, receipt.clone());
            Self::count(&mut self.effects, key);
            let mode = self
                .modes
                .get(&key)
                .copied()
                .ok_or(AgentBoundaryError::CorruptResponse)?;
            let observation = match mode {
                DeliveryMode::TrackThenReceipt => {
                    daemon_receipt::store(
                        &mut self.store,
                        self.tenant.clone(),
                        key,
                        &receipt.canonical_bytes,
                        &receipt.authorised_batch,
                    )
                    .map_err(|_| AgentBoundaryError::CorruptResponse)?;
                    self.outbox
                        .transition(
                            &mut self.store,
                            key,
                            OutboxState::Acknowledged,
                            "core acknowledged",
                            None,
                        )
                        .map_err(|_| AgentBoundaryError::Refused)?;
                    AgentObservation {
                        submission: Self::tracked(key, SubmissionState::Submitted, None),
                        activity_id,
                        receipt: None,
                    }
                }
                DeliveryMode::UnknownThenLookup => {
                    self.outbox
                        .transition(
                            &mut self.store,
                            key,
                            OutboxState::Unknown,
                            "transport outcome unavailable",
                            None,
                        )
                        .map_err(|_| AgentBoundaryError::Refused)?;
                    AgentObservation {
                        submission: Self::tracked(key, SubmissionState::Unknown, None),
                        activity_id,
                        receipt: None,
                    }
                }
            };
            self.submission_keys.insert(
                observation.submission.submission_ref.as_str().to_owned(),
                key,
            );
            self.observations.insert(key, observation.clone());
            self.fail_after(FaultPoint::Submit)?;
            Ok(observation)
        }

        fn track(
            &mut self,
            call: &Call<TrackRequest>,
        ) -> Result<AgentObservation, AgentBoundaryError> {
            let reference = call.request().submission_ref.as_str();
            let key = *self
                .submission_keys
                .get(reference)
                .ok_or(AgentBoundaryError::CorruptResponse)?;
            Self::count(&mut self.track_calls, key);
            if self.modes.get(&key) == Some(&DeliveryMode::UnknownThenLookup) {
                return Err(AgentBoundaryError::CorruptResponse);
            }
            let activity_id = self
                .outbox
                .status(key)
                .map(|status| status.activity_id)
                .ok_or(AgentBoundaryError::CorruptResponse)?;
            let current = self
                .outbox
                .status(key)
                .map(|status| status.state)
                .ok_or(AgentBoundaryError::CorruptResponse)?;
            if current == OutboxState::Acknowledged {
                let material = self
                    .receipt_material
                    .get(&key)
                    .ok_or(AgentBoundaryError::CorruptResponse)?;
                let signer = SigningKey::from_bytes(&[key[0].saturating_add(4); 32]);
                let raw = support::raw_receipt_evidence(
                    material.canonical_bytes.clone(),
                    material.authorised_batch,
                    u64::from(key[0]),
                    &signer,
                );
                let verified = support::evidence_verifier(&signer)
                    .verify_receipt(&raw)
                    .map_err(|_| AgentBoundaryError::CorruptResponse)?;
                self.outbox
                    .transition(
                        &mut self.store,
                        key,
                        OutboxState::Executed,
                        "verified receipt attached",
                        Some(verified),
                    )
                    .map_err(|_| AgentBoundaryError::Refused)?;
            }
            let observation = self.executed_observation(key, activity_id);
            self.observations.insert(key, observation.clone());
            self.fail_after(FaultPoint::Track)?;
            Ok(observation)
        }

        fn receipt_by_idempotency_key(
            &mut self,
            idempotency_key: [u8; 32],
            expected_activity_id: [u8; 32],
        ) -> Result<ReceiptLookup, AgentBoundaryError> {
            Self::count(&mut self.lookup_calls, idempotency_key);
            if self.lookup_calls.get(&idempotency_key) == Some(&1) {
                return Ok(ReceiptLookup::Absent);
            }
            if self.lookup_calls.get(&idempotency_key) == Some(&2) {
                let material = self
                    .receipt_material
                    .get(&idempotency_key)
                    .ok_or(AgentBoundaryError::CorruptResponse)?;
                daemon_receipt::store(
                    &mut self.store,
                    self.tenant.clone(),
                    idempotency_key,
                    &material.canonical_bytes,
                    &material.authorised_batch,
                )
                .map_err(|_| AgentBoundaryError::CorruptResponse)?;
            }
            let served = daemon_receipt::serve(
                &self.store,
                self.tenant.clone(),
                DaemonReceiptKey::Idempotency(idempotency_key),
            )
            .map_err(|_| AgentBoundaryError::Unavailable)?;
            if served.metadata.activity_id != expected_activity_id {
                return Err(AgentBoundaryError::CorruptResponse);
            }
            let material = self
                .receipt_material
                .get(&idempotency_key)
                .cloned()
                .ok_or(AgentBoundaryError::CorruptResponse)?;
            if served.canonical_bytes != material.canonical_bytes {
                return Err(AgentBoundaryError::CorruptResponse);
            }
            self.fail_after(FaultPoint::Lookup)?;
            Ok(ReceiptLookup::Found(material))
        }
    }

    #[derive(Clone)]
    struct ReceiptFields {
        activity_id: [u8; 32],
        previous_state_root: [u8; 32],
        resulting_state_root: [u8; 32],
        batch_id: [u8; 32],
        asset: [u8; 32],
        module: ModuleId,
        operation: u8,
        sequence: u64,
    }

    fn receipt(activity_id: [u8; 32], marker: u8, activity: ActivityType) -> ReceiptMaterial {
        let fields = ReceiptFields {
            activity_id,
            previous_state_root: [marker.saturating_add(1); 32],
            resulting_state_root: [marker.saturating_add(2); 32],
            batch_id: support::committed_execution_batch_id(
                [marker.saturating_add(1); 32],
                [0x81; 32],
                u64::from(marker),
            ),
            asset: [0x33; 32],
            module: activity.module(),
            operation: u8::try_from(activity.ordinal())
                .unwrap_or_else(|_| panic!("activity ordinal is outside receipt range")),
            sequence: u64::from(marker),
        };
        let signer = SigningKey::from_bytes(&[marker.saturating_add(4); 32]);
        let unsigned = encode_receipt(&fields, None);
        let mut digest = Sha256::new();
        digest.update(b"LXP/v1/receipt\0");
        digest.update(&unsigned);
        let signature = signer.sign(&<[u8; 32]>::from(digest.finalize()));
        ReceiptMaterial {
            canonical_bytes: encode_receipt(&fields, Some(signature.to_bytes())),
            tx_hash: activity_id,
            receipt_hash: Sha256::digest(encode_receipt(&fields, Some(signature.to_bytes())))
                .into(),
            block_height: u64::from(marker),
            rung: Rung::Receipted,
            authorised_batch: AuthorizedBatch::new(
                fields.batch_id,
                fields.asset,
                fields.previous_state_root,
                fields.resulting_state_root,
                signer.verifying_key().to_bytes(),
            ),
            verification_level: layerx_types::verify::VerificationLevel::SEQUENCER_SIGNED,
        }
    }

    fn encode_receipt(fields: &ReceiptFields, signature: Option<[u8; 64]>) -> Vec<u8> {
        let mut bytes = Vec::new();
        push_u16(&mut bytes, layerx_intents::canonical::PROTOCOL_VERSION);
        push_u16(&mut bytes, 0x5201);
        push_u16(&mut bytes, layerx_intents::canonical::PROTOCOL_VERSION);
        push_bytes(&mut bytes, &fields.activity_id);
        push_u64(&mut bytes, fields.sequence);
        push_bytes(&mut bytes, &fields.previous_state_root);
        push_bytes(&mut bytes, &fields.resulting_state_root);
        push_bytes(&mut bytes, &[0x81; 32]);
        bytes.extend_from_slice(&0_i32.to_be_bytes());
        bytes.extend_from_slice(&0_u32.to_be_bytes());
        bytes.extend_from_slice(&1_u128.to_be_bytes());
        push_bytes(&mut bytes, &fields.batch_id);
        push_u16(&mut bytes, u16::from(fields.module as u8));
        bytes.extend_from_slice(&1_u32.to_be_bytes());
        bytes.extend_from_slice(&1_u32.to_be_bytes());
        bytes.push(fields.operation);
        push_bytes(&mut bytes, &fields.asset);
        bytes.extend_from_slice(&1_u128.to_be_bytes());
        push_bytes(&mut bytes, &[0x91; 32]);
        bytes.extend_from_slice(&10_u128.to_be_bytes());
        bytes.extend_from_slice(&9_u128.to_be_bytes());
        push_u64(&mut bytes, 1);
        push_bytes(&mut bytes, &[0x92; 32]);
        bytes.extend_from_slice(&20_u128.to_be_bytes());
        bytes.extend_from_slice(&21_u128.to_be_bytes());
        push_bytes(&mut bytes, &[0x93; 32]);
        push_bytes(&mut bytes, &[0x94; 32]);
        push_bytes(&mut bytes, &[0x95; 32]);
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
        let length =
            u32::try_from(value.len()).unwrap_or_else(|_| panic!("receipt field length overflow"));
        output.extend_from_slice(&length.to_be_bytes());
        output.extend_from_slice(value);
    }

    struct Fixture {
        root: std::path::PathBuf,
        store_root: std::path::PathBuf,
        custody_root: std::path::PathBuf,
        agent_root: std::path::PathBuf,
        secret_path: std::path::PathBuf,
        tenancy_digest: TenancyDigest,
        principal: PrincipalId,
        public_key: [u8; 32],
        signer: CustodySigner,
        agent_contract: AgentClient,
        trace: TraceId,
    }

    impl Fixture {
        fn new(label: &str) -> Self {
            let root = directory(label);
            fs::create_dir_all(&root).unwrap_or_else(|error| panic!("fixture root: {error}"));
            let store_root = root.join("human-store");
            let secret_path = root.join("kms-mounted-root");
            fs::write(&secret_path, [0x42; 64]).unwrap_or_else(|error| panic!("KMS root: {error}"));
            let map = tenancy(&[("alice", "tenant-a")]);
            let tenancy_digest = map
                .install(&store_root)
                .unwrap_or_else(|error| panic!("tenancy: {error}"));
            let principal = principal("alice");
            let provider = EnvelopeKms::new("file-kms://human-primary", &secret_path)
                .unwrap_or_else(|error| panic!("KMS provider: {error}"));
            let keystore = Keystore::open_development(root.join("custody"), NETWORK_ID, provider)
                .unwrap_or_else(|error| panic!("keystore: {error}"));
            let key = KeyId::new("human-primary").unwrap_or_else(|error| panic!("key id: {error}"));
            let public_key = keystore
                .generate(
                    &principal,
                    &key,
                    KeyClass::HumanPrimary,
                    KeyEntropy::new([0x51; 32], [0x52; 16], [0x53; 24])
                        .unwrap_or_else(|error| panic!("entropy: {error}")),
                )
                .unwrap_or_else(|error| panic!("generate key: {error}"));
            let signer_store =
                PrincipalStore::open(&store_root, retention_uniform(10_000), tenancy_digest)
                    .unwrap_or_else(|error| panic!("signer store: {error}"));
            let signer = CustodySigner::new(
                keystore,
                signer_store,
                registry(),
                SigningLimits::new(1_000, 10_000).unwrap_or_else(|error| panic!("limits: {error}")),
            );
            let schema = layerx_agent_api::agent_api_schema_v1();
            let agent_contract = AgentClient::daemon("/run/layerx-agentd.sock", schema.version)
                .unwrap_or_else(|error| panic!("agent SDK: {error:?}"));
            Self {
                store_root,
                custody_root: root.join("custody"),
                agent_root: root.join("agent-store"),
                secret_path,
                tenancy_digest,
                principal,
                public_key,
                signer,
                agent_contract,
                trace: TraceId::mint([0x44; 16]),
                root,
            }
        }

        fn store(&self) -> PrincipalStore {
            PrincipalStore::open(
                &self.store_root,
                retention_uniform(10_000),
                self.tenancy_digest,
            )
            .unwrap_or_else(|error| panic!("principal store: {error}"))
        }

        fn plan(&self) -> JourneyPlan {
            let legs = [0x21_u8, 0x22_u8]
                .into_iter()
                .map(|key| {
                    JourneyLeg::new(
                        send_intent(self.public_key, u128::from(key), key),
                        [key; 32],
                        AgentDid::new("did:layerx:alice")
                            .unwrap_or_else(|error| panic!("actor: {error:?}")),
                        AuthorityRef::new("custody-human-primary")
                            .unwrap_or_else(|error| panic!("authority: {error:?}")),
                        ACCOUNT_SEQUENCE,
                        995,
                        1_010,
                        7,
                    )
                    .unwrap_or_else(|error| panic!("leg: {error}"))
                })
                .collect();
            JourneyPlan::new(
                JourneyId::new("jrn_crashjourney")
                    .unwrap_or_else(|error| panic!("journey id: {error}")),
                layerx_human_service::journeys::JourneyKind::Move,
                [0x31; 32],
                KeyId::new("human-primary").unwrap_or_else(|error| panic!("key: {error}")),
                Operation::ProtocolMutation,
                legs,
            )
            .unwrap_or_else(|error| panic!("plan: {error}"))
        }
    }

    impl Drop for Fixture {
        fn drop(&mut self) {
            let _ = (&self.custody_root, &self.secret_path);
            let _ = fs::remove_dir_all(&self.root);
        }
    }

    fn modes() -> BTreeMap<[u8; 32], DeliveryMode> {
        BTreeMap::from([
            ([0x21; 32], DeliveryMode::TrackThenReceipt),
            ([0x22; 32], DeliveryMode::UnknownThenLookup),
        ])
    }

    fn reopen(fixture: &Fixture) -> (PrincipalStore, JourneyEngine) {
        let mut store = fixture.store();
        let scope = store
            .principal(&fixture.principal)
            .unwrap_or_else(|error| panic!("reopen scope: {error}"));
        let id = JourneyId::new("jrn_crashjourney")
            .unwrap_or_else(|error| panic!("journey id: {error}"));
        let engine = JourneyEngine::load(&scope, &id)
            .unwrap_or_else(|error| panic!("load journey: {error}"))
            .unwrap_or_else(|| panic!("journey missing"));
        drop(scope);
        (store, engine)
    }

    fn drive_with_restart(
        fixture: &Fixture,
        agent: &mut RealAgentLayer,
        mut store: PrincipalStore,
        mut engine: JourneyEngine,
        start_time: u64,
    ) -> JourneyEngine {
        let registry = registry();
        for offset in 0..40_u64 {
            let mut scope = store
                .principal(&fixture.principal)
                .unwrap_or_else(|error| panic!("drive scope: {error}"));
            let result = ready(engine.advance(
                &mut scope,
                &fixture.agent_contract,
                agent,
                &fixture.signer,
                &registry,
                &fixture.trace,
                start_time.saturating_add(offset),
            ));
            drop(scope);
            drop(store);
            if let Err(error) = result {
                assert!(
                    error.to_string().contains("agent boundary failure"),
                    "unexpected journey error: {error}"
                );
            }
            let reopened = reopen(fixture);
            store = reopened.0;
            engine = reopened.1;
            if engine
                .status()
                .unwrap_or_else(|error| panic!("status: {error}"))
                .state()
                == JourneyState::Done
            {
                return engine;
            }
        }
        panic!("journey did not complete within the bounded driver")
    }

    #[test]
    fn crashes_after_every_durable_stage_resume_with_one_effect_per_leg() {
        let fixture = Fixture::new("journey-stage-crashes");
        let registry = registry();
        let mut store = fixture.store();
        let mut scope = store
            .principal(&fixture.principal)
            .unwrap_or_else(|error| panic!("scope: {error}"));
        let plan = fixture.plan();
        let engine = JourneyEngine::start(&mut scope, &plan, &registry, 100)
            .unwrap_or_else(|error| panic!("start: {error}"));
        let repeated = JourneyEngine::start(&mut scope, &plan, &registry, 100)
            .unwrap_or_else(|error| panic!("repeat: {error}"));
        assert_eq!(
            repeated
                .status()
                .unwrap_or_else(|error| panic!("repeat status: {error}")),
            engine
                .status()
                .unwrap_or_else(|error| panic!("initial status: {error}"))
        );
        drop(scope);

        let mut agent = RealAgentLayer::new(&fixture.agent_root, modes());
        let engine = drive_with_restart(&fixture, &mut agent, store, engine, 101);
        let status = engine
            .status()
            .unwrap_or_else(|error| panic!("final status: {error}"));
        assert_eq!(status.state(), JourneyState::Done);
        assert_eq!(
            status.phases(),
            [JourneyPhase::ReceiptVerified, JourneyPhase::ReceiptVerified]
        );
        assert!(status.receipt_digests().iter().all(Option::is_some));
        assert_eq!(agent.effects.get(&[0x21; 32]), Some(&1));
        assert_eq!(agent.effects.get(&[0x22; 32]), Some(&1));
        assert_eq!(agent.submit_calls.get(&[0x21; 32]), Some(&1));
        assert_eq!(agent.submit_calls.get(&[0x22; 32]), Some(&1));
        assert_eq!(agent.track_calls.get(&[0x21; 32]), Some(&1));
        assert!(!agent.track_calls.contains_key(&[0x22; 32]));
        assert_eq!(agent.lookup_calls.get(&[0x22; 32]), Some(&2));

        let mut final_store = fixture.store();
        let final_scope = final_store
            .principal(&fixture.principal)
            .unwrap_or_else(|error| panic!("final scope: {error}"));
        let id = JourneyId::new("jrn_crashjourney")
            .unwrap_or_else(|error| panic!("journey id: {error}"));
        let stream = JourneyEngine::stream_events(&final_scope, &id)
            .unwrap_or_else(|error| panic!("stream: {error}"));
        let notifications = JourneyEngine::notification_events(&final_scope, &id)
            .unwrap_or_else(|error| panic!("notifications: {error}"));
        assert_eq!(stream, notifications);
        assert!(stream
            .windows(2)
            .all(|pair| pair[1].sequence() == pair[0].sequence() + 1));
        let cursors = stream
            .iter()
            .map(JourneyProgress::cursor)
            .collect::<BTreeSet<_>>();
        assert_eq!(cursors.len(), stream.len());
    }

    #[test]
    fn acknowledgement_gap_faults_converge_without_resubmitting_unknown() {
        for fault in [
            FaultPoint::Prepare,
            FaultPoint::Submit,
            FaultPoint::Track,
            FaultPoint::Lookup,
        ] {
            let label = format!("journey-gap-{fault:?}").to_ascii_lowercase();
            let fixture = Fixture::new(&label);
            let registry = registry();
            let mut store = fixture.store();
            let mut scope = store
                .principal(&fixture.principal)
                .unwrap_or_else(|error| panic!("scope: {error}"));
            let engine = JourneyEngine::start(&mut scope, &fixture.plan(), &registry, 200)
                .unwrap_or_else(|error| panic!("start: {error}"));
            drop(scope);
            let mut agent = RealAgentLayer::new(&fixture.agent_root, modes());
            agent.inject(fault);
            let engine = drive_with_restart(&fixture, &mut agent, store, engine, 201);
            assert_eq!(
                engine
                    .status()
                    .unwrap_or_else(|error| panic!("status: {error}"))
                    .state(),
                JourneyState::Done
            );
            assert_eq!(agent.effects.get(&[0x21; 32]), Some(&1));
            assert_eq!(agent.effects.get(&[0x22; 32]), Some(&1));
            assert!(agent.fault_fired);
            if fault == FaultPoint::Submit {
                assert_eq!(agent.submit_calls.get(&[0x21; 32]), Some(&2));
                assert_eq!(agent.effects.get(&[0x21; 32]), Some(&1));
            }
            assert!(!agent.track_calls.contains_key(&[0x22; 32]));
            assert_eq!(agent.submit_calls.get(&[0x22; 32]), Some(&1));
            if fault == FaultPoint::Lookup {
                assert_eq!(agent.lookup_calls.get(&[0x22; 32]), Some(&3));
            } else {
                assert_eq!(agent.lookup_calls.get(&[0x22; 32]), Some(&2));
            }
        }
    }

    fn hex(bytes: &[u8]) -> String {
        let mut text = String::with_capacity(bytes.len().saturating_mul(2));
        for byte in bytes {
            use std::fmt::Write as _;
            let _ = write!(text, "{byte:02x}");
        }
        text
    }

    #[test]
    fn repeated_start_repairs_initial_progress_after_restart() {
        use layerx_human_service::store::Table;
        let fixture = Fixture::new("initial-progress-repair");
        let plan = fixture.plan();
        let mut store = fixture.store();
        let mut scope = store
            .principal(&fixture.principal)
            .unwrap_or_else(|error| panic!("scope: {error}"));
        let original = JourneyEngine::start(&mut scope, &plan, &registry(), 100)
            .unwrap_or_else(|error| panic!("start: {error}"));
        let id = JourneyId::new("jrn_crashjourney").unwrap_or_else(|error| panic!("id: {error}"));
        let expected = JourneyEngine::stream_events(&scope, &id)
            .unwrap_or_else(|error| panic!("events: {error}"));
        assert_eq!(expected.len(), 1);
        for key in scope.keys(Table::Journeys) {
            if key.as_str().starts_with("jstream-") || key.as_str().starts_with("jnotify-") {
                assert!(scope
                    .remove(Table::Journeys, &key)
                    .unwrap_or_else(|error| panic!("remove progress: {error}")));
            }
        }
        drop(scope);
        drop(store);
        let mut store = fixture.store();
        let mut scope = store
            .principal(&fixture.principal)
            .unwrap_or_else(|error| panic!("reopen scope: {error}"));
        let repeated = JourneyEngine::start(&mut scope, &plan, &registry(), 200)
            .unwrap_or_else(|error| panic!("retry: {error}"));
        assert_eq!(repeated.status().ok(), original.status().ok());
        assert_eq!(
            JourneyEngine::stream_events(&scope, &id).ok(),
            Some(expected.clone())
        );
        assert_eq!(
            JourneyEngine::notification_events(&scope, &id).ok(),
            Some(expected)
        );
    }

    use layerx_human_service::activity::{ActivityKind, Feed, FilterDraft, PageRequest};
    use layerx_human_service::agents::{
        Reclaim, ReclaimAgentBoundary, ReclaimAgentContext, ReclaimError, ReclaimMechanism,
        ReclaimRequest, ReclaimStage,
    };
    use layerx_human_service::journeys::{MovementTerm, PayerGrantRoute, SendRoute};
    use layerx_types::intent::BudgetId;

    impl ReclaimAgentBoundary for RealAgentLayer {
        fn reclaim_receipt(
            &mut self,
            action_key: [u8; 32],
            expected_activity_id: [u8; 32],
        ) -> Result<ReceiptMaterial, AgentBoundaryError> {
            let material = self
                .receipt_material
                .get(&action_key)
                .cloned()
                .ok_or(AgentBoundaryError::CorruptResponse)?;
            let verified = layerx_proof::receipt::verify(
                &material.canonical_bytes,
                &material.authorised_batch,
            )
            .map_err(|_| AgentBoundaryError::CorruptResponse)?;
            let protocol = verified
                .receipt()
                .protocol()
                .ok_or(AgentBoundaryError::CorruptResponse)?;
            if protocol.activity_id() != expected_activity_id {
                return Err(AgentBoundaryError::CorruptResponse);
            }
            Ok(material)
        }
    }

    fn send_route(public_key: [u8; 32], key: u8) -> SendRoute {
        let signer = layerx_crypto::local::LocalSigner::new([0x51; 32]);
        assert_eq!(
            layerx_crypto::signer::Signer::public_key(&signer),
            public_key
        );
        let debit = layerx_crypto::send::SendDebit {
            from: layerx_intents::canonical::account_id_for_protocol(
                &account("agent:did:layerx:worker:main"),
                layerx_intents::canonical::PROTOCOL_VERSION,
            )
            .unwrap_or_else(|error| panic!("source account: {error:?}")),
            to: layerx_intents::canonical::account_id_for_protocol(
                &account("agent:did:layerx:human:main"),
                layerx_intents::canonical::PROTOCOL_VERSION,
            )
            .unwrap_or_else(|error| panic!("destination account: {error:?}")),
            asset: [0x33; 32],
            amount: 1,
            source_sequence: 7,
            idempotency_key: [key; 32],
            expires_at: 1_010,
            context_hash: [0x61; 32],
            conditions: Vec::new(),
            authorization_kind: SendAuthorizationKind::Owner as u8,
            network_id: NETWORK_ID,
            protocol_version: layerx_intents::canonical::PROTOCOL_VERSION,
        };

        SendRoute {
            account_sequence: Sequence::from_u64(ACCOUNT_SEQUENCE),
            idempotency_key: IdempotencyKey::new([key; 32]),
            expires_at: TimestampSeconds::from_u64(1_010),
            context_hash: ContextHash::new([0x61; 32]),
            authorization: support::sign_send(
                &signer,
                &debit,
                SendAuthorization::new(
                    SendAuthorizationKind::Owner,
                    PublicKey::new(public_key),
                    AuthorizationSignature::new([0x62; 64]),
                ),
            ),
            network_id: NetworkId::new(NETWORK_ID)
                .unwrap_or_else(|error| panic!("network: {error:?}")),
            protocol_version: ProtocolVersion::new(layerx_intents::canonical::PROTOCOL_VERSION)
                .unwrap_or_else(|error| panic!("protocol: {error:?}")),
        }
    }

    fn reclaim_request(journey: &str, key: u8, mechanism: ReclaimMechanism) -> ReclaimRequest {
        ReclaimRequest {
            journey_id: JourneyId::new(journey)
                .unwrap_or_else(|error| panic!("reclaim journey: {error}")),
            idempotency_key: [key; 32],
            owner: account("agent:did:layerx:human:main"),
            agent_account: account("agent:did:layerx:worker:main"),
            asset: AssetId::new([0x33; 32]),
            amount: Amount::from_u128(1),
            mechanism,
            agent: ReclaimAgentContext {
                label: "worker".to_owned(),
                actor: AgentDid::new("did:layerx:worker")
                    .unwrap_or_else(|error| panic!("actor: {error:?}")),
                authority: AuthorityRef::new("custody-human-primary")
                    .unwrap_or_else(|error| panic!("authority: {error:?}")),
                custody_key: KeyId::new("human-primary")
                    .unwrap_or_else(|error| panic!("custody key: {error}")),
                nonce: ACCOUNT_SEQUENCE,
                not_before: 995,
                not_after: 1_010,
                fee_limit: 7,
            },
        }
    }

    fn drive_reclaim(
        fixture: &Fixture,
        agent: &mut RealAgentLayer,
        journey_id: &JourneyId,
        start_time: u64,
    ) -> Reclaim {
        for offset in 0..24_u64 {
            let mut store = fixture.store();
            let mut scope = store
                .principal(&fixture.principal)
                .unwrap_or_else(|error| panic!("reclaim scope: {error}"));
            let mut reclaim = Reclaim::load(&scope, journey_id)
                .unwrap_or_else(|error| panic!("load reclaim: {error}"))
                .unwrap_or_else(|| panic!("reclaim disappeared"));
            let status = ready(reclaim.advance(
                &mut scope,
                &fixture.agent_contract,
                agent,
                &fixture.signer,
                &registry(),
                &fixture.trace,
                start_time.saturating_add(offset),
            ))
            .unwrap_or_else(|error| {
                panic!(
                    "advance reclaim {} at transition {offset}: {error}",
                    journey_id.as_str()
                )
            });
            drop(scope);
            if matches!(status.stage(), ReclaimStage::Done | ReclaimStage::Refused) {
                return reclaim;
            }
        }
        panic!("reclaim did not reach a receipt-gated terminal state")
    }

    #[test]
    fn every_reclaim_mechanism_uses_real_agent_receipts_and_projects_activity() {
        let fixture = Fixture::new("agent-reclaim-mechanisms");
        let requests = [
            reclaim_request(
                "jrn_reclaimbudget",
                0x71,
                ReclaimMechanism::BudgetDefund {
                    budget_account: account("agent:did:layerx:worker:budget:operations"),
                    budget_id: BudgetId::new([0x41; 32]),
                    revocation_sequence: Sequence::from_u64(4),
                },
            ),
            reclaim_request(
                "jrn_reclaimagent",
                0x72,
                ReclaimMechanism::AgentAuthorized(send_route(fixture.public_key, 0x72)),
            ),
            reclaim_request(
                "jrn_reclaimgrant",
                0x73,
                ReclaimMechanism::ReceiveUnderPayerGrant(PayerGrantRoute {
                    receive: signed_reclaim_receive(),
                }),
            ),
        ];
        let mut store = fixture.store();
        let mut scope = store
            .principal(&fixture.principal)
            .unwrap_or_else(|error| panic!("start scope: {error}"));
        for request in &requests {
            let first = Reclaim::start(&mut scope, request, &registry(), 300)
                .unwrap_or_else(|error| panic!("start reclaim: {error}"));
            let repeated = Reclaim::start(&mut scope, request, &registry(), 300)
                .unwrap_or_else(|error| panic!("repeat reclaim: {error}"));
            assert_eq!(first, repeated);
            assert_eq!(
                layerx_human_service::agents::ReclaimStatus::movement_term(),
                MovementTerm::Return
            );
            assert_eq!(
                layerx_human_service::agents::ReclaimStatus::user_action(),
                "Move money"
            );
        }
        drop(scope);

        let modes = BTreeMap::from([
            ([0x71; 32], DeliveryMode::TrackThenReceipt),
            ([0x72; 32], DeliveryMode::TrackThenReceipt),
            ([0x73; 32], DeliveryMode::TrackThenReceipt),
        ]);
        let mut agent = RealAgentLayer::new(&fixture.agent_root, modes);
        for (offset, request) in requests.iter().enumerate() {
            let reclaim = drive_reclaim(
                &fixture,
                &mut agent,
                &request.journey_id,
                301 + u64::try_from(offset).unwrap_or(0) * 30,
            );
            let status = reclaim
                .status()
                .unwrap_or_else(|error| panic!("reclaim status: {error}"));
            assert_eq!(status.stage(), ReclaimStage::Done);
            let result = status
                .result()
                .unwrap_or_else(|| panic!("receipt result missing"));
            assert_eq!(result.asset(), [0x33; 32]);
            assert_eq!(result.amount(), 1);
            assert_eq!(result.fee_charged(), 1);
            assert_ne!(result.activity_id(), [0; 32]);
            assert_ne!(result.receipt_digest(), [0; 32]);
            assert_eq!(agent.effects.get(&request.idempotency_key), Some(&1));
            assert_eq!(agent.submit_calls.get(&request.idempotency_key), Some(&1));
        }

        let mut final_store = fixture.store();
        let final_scope = final_store
            .principal(&fixture.principal)
            .unwrap_or_else(|error| panic!("activity scope: {error}"));
        let filters = Feed::apply_filters(
            FilterDraft::new()
                .with_kinds([ActivityKind::Movement])
                .with_agent("worker"),
        )
        .unwrap_or_else(|error| panic!("activity filters: {error}"));
        let page = Feed::new(120)
            .unwrap_or_else(|error| panic!("activity feed: {error}"))
            .page(&final_scope, PageRequest::new(10, filters), 400, 0)
            .unwrap_or_else(|error| panic!("activity page: {error}"));
        assert_eq!(page.entries().len(), 3);
        assert!(page.entries().iter().all(|entry| {
            entry.kind() == ActivityKind::Movement
                && entry.agent() == Some("worker")
                && entry.receipts().len() == 1
        }));
    }

    #[test]
    fn reclaim_contract_is_closed_to_returns_and_rejects_conflicting_reuse() {
        assert_eq!(Reclaim::signing_operation(), Operation::ProtocolMutation);
        assert_eq!(Reclaim::signing_operation().label(), "protocol-mutation");
        assert!(!Reclaim::signing_operation().label().contains("sweep"));
        let fixture = Fixture::new("agent-reclaim-closed-contract");
        let request = reclaim_request(
            "jrn_reclaimclosed",
            0x74,
            ReclaimMechanism::AgentAuthorized(send_route(fixture.public_key, 0x74)),
        );
        let mut store = fixture.store();
        let mut scope = store
            .principal(&fixture.principal)
            .unwrap_or_else(|error| panic!("closed scope: {error}"));
        Reclaim::start(&mut scope, &request, &registry(), 500)
            .unwrap_or_else(|error| panic!("closed start: {error}"));
        let mut conflict = request;
        conflict.amount = Amount::from_u128(2);
        assert!(matches!(
            Reclaim::start(&mut scope, &conflict, &registry(), 500),
            Err(ReclaimError::IdempotencyConflict)
        ));
    }

    fn signed_reclaim_receive() -> layerx_intents::NativeReceive {
        layerx_intents::NativeReceive::new(&support::receive::signed_receive(
            &support::receive::SignedReceiveRequest {
                from: &account("agent:did:layerx:worker:main"),
                to: &account("agent:did:layerx:human:main"),
                asset: [0x33; 32],
                amount: 1,
                sequence: ACCOUNT_SEQUENCE,
                idempotency_key: [0x73; 32],
                network_id: NETWORK_ID,
                protocol_version: layerx_intents::canonical::PROTOCOL_VERSION,
            },
        ))
        .unwrap_or_else(|error| panic!("signed receive: {error:?}"))
    }

    use base64::engine::general_purpose::URL_SAFE_NO_PAD;
    use base64::Engine as _;
    use ciborium::value::Value as CborValue;
    use layerx_agentd::identity::{
        register, CoreIdentity, IdentityError, IdentityResolver, ProtocolAuthority,
    };
    use layerx_agentd::session::{
        close, invalidate_on_revocation, open, InvalidationReason, OpenRequest, RevocationEvent,
        SessionId, SessionRegistry, Token,
    };
    use layerx_agentd::sign::ProvisionedSessionKey;
    use layerx_crypto::session::IssuedSessionKey;
    use layerx_human_service::agents::{
        AgentBalance, ArchiveAgentContract, ArchiveBoundary, ArchiveError, ArchiveJourney,
        ArchiveRequest, ArchiveStage, FundsDispositionEvidence, SessionArchiveAdapter,
        ARCHIVE_ACTION_LABEL, ARCHIVE_CONFIRMATION_TONE, ARCHIVE_IRREVERSIBILITY_NOTICE,
    };
    use layerx_human_service::audit::{AuditChain, AuditEvent, SecurityChangeKind};
    use layerx_human_service::auth::{
        AccountIdentity, AuthConfig, Device, Passkeys, RateLimit, SessionGrant,
    };
    use layerx_human_service::custody::{
        AgentContractError, AgentSessionContract, AgentSessionProvision, ProtocolIdentitySnapshot,
        ProvisionEvidence, RevocationEvidence, RevocationReason, RotationEvidence,
        RotationObservation, RotationSubmission, SessionEntropySource, SessionKeyEntropy,
        SessionKeyError, SessionKeyProvisioner, SessionLeaseState, SessionPolicy, SessionTarget,
        SuspensionEvidence,
    };
    use layerx_human_service::notify::AgentId;
    use layerx_human_service::store::{EvidenceRef, PrincipalScope, RowKey, Table};
    use layerx_types::verify::VerificationLevel;
    use serde_json::{json, Value};

    const ARCHIVE_ASSET: [u8; 32] = [0x33; 32];
    const ARCHIVE_RP_ID: &str = "paxportwallet.com";
    const ARCHIVE_ORIGIN: &str = "https://paxportwallet.com";
    const FLAG_UP: u8 = 1;
    const FLAG_UV: u8 = 1 << 2;
    const FLAG_AT: u8 = 1 << 6;

    struct IdentityBoundary(CoreIdentity);

    impl IdentityResolver for IdentityBoundary {
        fn resolve(&mut self, _did: &Did) -> Result<Option<CoreIdentity>, IdentityError> {
            Ok(Some(self.0.clone()))
        }
    }

    struct InstalledAuthority {
        _issued: IssuedSessionKey,
        _signer: ProvisionedSessionKey,
        token: Token,
    }

    struct AuthorityLayer {
        store: AgentStore,
        sessions: SessionRegistry,
        tenant: TenantId,
        principal: PrincipalId,
        did: Did,
        primary_public_key: [u8; 32],
        protocol_identity: [u8; 32],
        revocation_sequence: u64,
        now: u64,
        core_sequence: u64,
        installed: BTreeMap<[u8; 32], InstalledAuthority>,
        funds: FundsDispositionEvidence,
        suspension_effects: usize,
        revocation_effects: usize,
    }

    impl AuthorityLayer {
        fn new(root: &std::path::Path, principal: PrincipalId, did: Did, key: [u8; 32]) -> Self {
            Self {
                store: AgentStore::open(root)
                    .unwrap_or_else(|error| panic!("archive agent store: {error}")),
                sessions: SessionRegistry::default(),
                tenant: TenantId::new("tenant-archive")
                    .unwrap_or_else(|error| panic!("archive tenant: {error}")),
                protocol_identity: Sha256::digest(did.as_bytes()).into(),
                principal,
                did,
                primary_public_key: key,
                revocation_sequence: 1,
                now: 1_000,
                core_sequence: 10,
                installed: BTreeMap::new(),
                funds: FundsDispositionEvidence {
                    before: vec![AgentBalance {
                        asset: AssetId::new(ARCHIVE_ASSET),
                        amount: 1,
                    }],
                    after: vec![AgentBalance {
                        asset: AssetId::new(ARCHIVE_ASSET),
                        amount: 1,
                    }],
                    protocol_state_digest: [0x81; 32],
                    observed_sequence: 10,
                    observed_at: 1_000,
                    rung: Rung::Committed,
                },
                suspension_effects: 0,
                revocation_effects: 0,
            }
        }

        fn require_scope(
            &self,
            principal: &PrincipalId,
            did: &Did,
        ) -> Result<(), AgentContractError> {
            if principal == &self.principal && did == &self.did {
                Ok(())
            } else {
                Err(AgentContractError::Refused("archive scope mismatch"))
            }
        }

        fn daemon_open(&self, grant: [u8; 32]) -> bool {
            self.sessions
                .get(&self.tenant, SessionId(grant))
                .is_some_and(|session| session.open)
        }

        fn authorizes(&self, grant: [u8; 32]) -> bool {
            self.installed.get(&grant).is_some_and(|installed| {
                installed
                    .token
                    .authorize(
                        &self.sessions,
                        &self.tenant,
                        &self.did,
                        "prepare",
                        self.core_sequence,
                    )
                    .is_ok()
            })
        }
    }

    impl AgentSessionContract for AuthorityLayer {
        fn identity_snapshot(
            &mut self,
            principal: &PrincipalId,
            did: &Did,
        ) -> Result<ProtocolIdentitySnapshot, AgentContractError> {
            self.require_scope(principal, did)?;
            Ok(ProtocolIdentitySnapshot {
                protocol_identity: self.protocol_identity,
                primary_public_key: self.primary_public_key,
                revocation_sequence: self.revocation_sequence,
                protocol_time: self.now,
                core_sequence: self.core_sequence,
                rung: Rung::Committed,
            })
        }

        fn provision_session(
            &mut self,
            request: AgentSessionProvision,
        ) -> Result<ProvisionEvidence, AgentContractError> {
            let (principal, did, issued, scopes, policy_version, secret) = request.into_parts();
            self.require_scope(&principal, &did)?;
            let grant_id = issued.grant_id;
            let session_public_key = issued.session_public_key;
            let signer = ProvisionedSessionKey::new(secret.into_seed(), issued.clone())
                .map_err(|_| AgentContractError::Refused("archive session key mismatch"))?;
            let authorities = self
                .installed
                .keys()
                .copied()
                .map(ProtocolAuthority::SessionKey)
                .chain(std::iter::once(ProtocolAuthority::SessionKey(grant_id)))
                .collect();
            let mut resolver = IdentityBoundary(CoreIdentity {
                canonical_bytes: self.protocol_identity.to_vec(),
                head_sequence: self.core_sequence,
                revocation_sequence: 1,
                verification_level: VerificationLevel::STATE_PROVEN,
                frozen: false,
                authorities,
            });
            let identity = register(
                &mut self.store,
                self.tenant.clone(),
                self.did.clone(),
                &mut resolver,
            )
            .map_err(|_| AgentContractError::Refused("archive identity registration failed"))?;
            let token = open(
                &mut self.store,
                &mut self.sessions,
                &identity,
                OpenRequest {
                    session_id: SessionId(grant_id),
                    token_id: Sha256::digest(grant_id).into(),
                    tenant: self.tenant.clone(),
                    agent: self.did.clone(),
                    authority: ProtocolAuthority::SessionKey(grant_id),
                    permitted_activity_types: issued
                        .permitted_activity_types
                        .iter()
                        .map(|activity| activity.ordinal())
                        .collect(),
                    scopes: scopes.into_iter().collect(),
                    expiry_sequence: issued.expires_at,
                    expiry_seconds: None,
                    opening_client: "layerx-human-service".to_owned(),
                    policy_version,
                },
                self.core_sequence,
            )
            .map_err(|_| AgentContractError::Refused("archive daemon session failed"))?;
            self.installed.insert(
                grant_id,
                InstalledAuthority {
                    _issued: issued,
                    _signer: signer,
                    token,
                },
            );
            let mut evidence = ProvisionEvidence {
                grant_id,
                session_public_key,
                daemon_session_id: grant_id,
                protocol_sequence: self.core_sequence,
                observed_at: self.now,
                rung: Rung::Receipted,
                receipt_digest: [0; 32],
            };
            evidence.receipt_digest = evidence.expected_digest();
            Ok(evidence)
        }

        fn suspend_permissions(
            &mut self,
            target: &SessionTarget,
            reason: RevocationReason,
            requested_at: u64,
        ) -> Result<SuspensionEvidence, AgentContractError> {
            self.require_scope(&target.principal, &target.agent_did)?;
            close(
                &mut self.store,
                &mut self.sessions,
                &self.tenant,
                SessionId(target.daemon_session_id),
            )
            .map_err(|_| AgentContractError::Refused("archive daemon close failed"))?;
            self.suspension_effects = self.suspension_effects.saturating_add(1);
            self.now = self.now.max(requested_at);
            let mut evidence = SuspensionEvidence {
                grant_id: target.grant_id,
                daemon_session_id: target.daemon_session_id,
                reason,
                observed_at: self.now,
                receipt_digest: [0; 32],
            };
            evidence.receipt_digest = evidence.expected_digest();
            Ok(evidence)
        }

        fn revoke_protocol_session(
            &mut self,
            target: &SessionTarget,
            reason: RevocationReason,
            requested_at: u64,
        ) -> Result<RevocationEvidence, AgentContractError> {
            self.require_scope(&target.principal, &target.agent_did)?;
            self.core_sequence = self.core_sequence.saturating_add(1);
            self.revocation_sequence = self.revocation_sequence.saturating_add(1);
            invalidate_on_revocation(
                &mut self.store,
                &mut self.sessions,
                &mut [],
                &RevocationEvent {
                    did: self.did.clone(),
                    authority: Some(ProtocolAuthority::SessionKey(target.grant_id)),
                    reason: InvalidationReason::SessionKeyRevoked,
                    observed_sequence: self.core_sequence,
                },
            )
            .map_err(|_| AgentContractError::Refused("archive protocol invalidation failed"))?;
            self.installed.remove(&target.grant_id);
            self.revocation_effects = self.revocation_effects.saturating_add(1);
            self.now = self.now.max(requested_at).saturating_add(1);
            let mut evidence = RevocationEvidence {
                grant_id: target.grant_id,
                reason,
                observed_sequence: self.core_sequence,
                observed_at: self.now,
                rung: Rung::Receipted,
                receipt_digest: [0; 32],
            };
            evidence.receipt_digest = evidence.expected_digest();
            Ok(evidence)
        }

        fn announce_rotation(
            &mut self,
            _submission: RotationSubmission,
        ) -> Result<RotationEvidence, AgentContractError> {
            Err(AgentContractError::Refused("archive is not rotation"))
        }

        fn rotation_observation(
            &mut self,
            _principal: &PrincipalId,
            _did: &Did,
        ) -> Result<RotationObservation, AgentContractError> {
            Err(AgentContractError::Refused("archive is not rotation"))
        }
    }

    impl ArchiveAgentContract for AuthorityLayer {
        fn funds_disposition(
            &mut self,
            principal: &PrincipalId,
            did: &Did,
            agent_id: &AgentId,
        ) -> Result<FundsDispositionEvidence, AgentContractError> {
            self.require_scope(principal, did)?;
            if agent_id.as_str() != "agt_worker" {
                return Err(AgentContractError::Refused("archive agent mismatch"));
            }
            Ok(self.funds.clone())
        }
    }

    struct ArchiveEntropy;

    impl SessionEntropySource for ArchiveEntropy {
        fn next_session_entropy(&mut self) -> Result<SessionKeyEntropy, SessionKeyError> {
            SessionKeyEntropy::new([0x91; 32])
        }
    }

    struct AckGap<B> {
        inner: B,
        fired: bool,
    }

    impl<B: ArchiveBoundary> ArchiveBoundary for AckGap<B> {
        fn funds_disposition(
            &mut self,
            principal: &PrincipalId,
            did: &Did,
            agent_id: &AgentId,
        ) -> Result<FundsDispositionEvidence, ArchiveError> {
            self.inner.funds_disposition(principal, did, agent_id)
        }

        fn archive_authority(
            &mut self,
            principal: &PrincipalId,
            did: &Did,
            requested_at: u64,
        ) -> Result<layerx_human_service::custody::RevocationOutcome, ArchiveError> {
            let outcome = self.inner.archive_authority(principal, did, requested_at)?;
            if self.fired {
                Ok(outcome)
            } else {
                self.fired = true;
                Err(ArchiveError::Agent(AgentContractError::Unavailable))
            }
        }
    }

    fn archive_policy() -> SessionPolicy {
        SessionPolicy::new(
            1_000,
            100,
            3,
            vec!["prepare".to_owned(), "submit".to_owned()],
            "managed-agent-archive-v1",
        )
        .unwrap_or_else(|error| panic!("archive policy: {error}"))
    }

    fn auth_config() -> AuthConfig {
        AuthConfig {
            rp_id: ARCHIVE_RP_ID.to_owned(),
            rp_name: "LayerX".to_owned(),
            origin: ARCHIVE_ORIGIN.to_owned(),
            ceremony_ttl_secs: 300,
            assertion_ttl_secs: 30,
            session_ttl_secs: 300,
            refresh_ttl_secs: 600,
            step_up_ttl_secs: 30,
            rate_limit: RateLimit {
                attempts: 100,
                window_secs: 60,
            },
        }
    }

    fn required<T, E: std::fmt::Display>(result: Result<T, E>, label: &str) -> T {
        result.unwrap_or_else(|error| panic!("{label}: {error}"))
    }

    fn decode_ceremony(value: &str) -> Value {
        let bytes = required(URL_SAFE_NO_PAD.decode(value), "decode ceremony");
        required(serde_json::from_slice(&bytes), "parse ceremony")
    }

    fn required_text<'a>(value: &'a Value, pointer: &str) -> &'a str {
        value
            .pointer(pointer)
            .and_then(Value::as_str)
            .unwrap_or_else(|| panic!("missing ceremony field {pointer}"))
    }

    fn encode_response(value: &Value) -> String {
        URL_SAFE_NO_PAD.encode(required(serde_json::to_vec(value), "serialize response"))
    }

    struct ArchiveAuthenticator {
        signing_key: SigningKey,
        credential_id: Vec<u8>,
        counter: u32,
        user_handle: Option<String>,
    }

    impl ArchiveAuthenticator {
        fn new() -> Self {
            Self {
                signing_key: SigningKey::from_bytes(&[0x92; 32]),
                credential_id: vec![0x93; 32],
                counter: 0,
                user_handle: None,
            }
        }

        fn register(&mut self, ceremony: &str) -> String {
            let options = decode_ceremony(ceremony);
            self.user_handle = Some(required_text(&options, "/user/id").to_owned());
            let client_data = client_data("webauthn.create", required_text(&options, "/challenge"));
            encode_response(&json!({
                "id": URL_SAFE_NO_PAD.encode(&self.credential_id),
                "transports": ["internal"],
                "attestationObject": URL_SAFE_NO_PAD.encode(self.attestation_object()),
                "clientDataJSON": URL_SAFE_NO_PAD.encode(client_data),
            }))
        }

        fn assert(&mut self, ceremony: &str) -> String {
            let options = decode_ceremony(ceremony);
            self.counter = self.counter.saturating_add(1);
            let authenticator_data = self.authenticator_data(self.counter, false);
            let client_data = client_data("webauthn.get", required_text(&options, "/challenge"));
            let mut signed = authenticator_data.clone();
            signed.extend_from_slice(&Sha256::digest(&client_data));
            let signature = self.signing_key.sign(&signed).to_bytes();
            encode_response(&json!({
                "id": URL_SAFE_NO_PAD.encode(&self.credential_id),
                "authenticatorData": URL_SAFE_NO_PAD.encode(authenticator_data),
                "signature": URL_SAFE_NO_PAD.encode(signature),
                "clientDataJSON": URL_SAFE_NO_PAD.encode(client_data),
                "userHandle": self.user_handle,
            }))
        }

        fn attestation_object(&self) -> Vec<u8> {
            let map = CborValue::Map(vec![
                (
                    CborValue::Text("fmt".to_owned()),
                    CborValue::Text("none".to_owned()),
                ),
                (
                    CborValue::Text("attStmt".to_owned()),
                    CborValue::Map(Vec::new()),
                ),
                (
                    CborValue::Text("authData".to_owned()),
                    CborValue::Bytes(self.authenticator_data(0, true)),
                ),
            ]);
            let mut bytes = Vec::new();
            required(
                ciborium::ser::into_writer(&map, &mut bytes),
                "encode attestation",
            );
            bytes
        }

        fn authenticator_data(&self, counter: u32, attested: bool) -> Vec<u8> {
            let mut bytes = Vec::new();
            bytes.extend_from_slice(&Sha256::digest(ARCHIVE_RP_ID.as_bytes()));
            bytes.push(FLAG_UP | FLAG_UV | if attested { FLAG_AT } else { 0 });
            bytes.extend_from_slice(&counter.to_be_bytes());
            if attested {
                bytes.extend_from_slice(&[0; 16]);
                bytes.extend_from_slice(
                    &u16::try_from(self.credential_id.len())
                        .unwrap_or_else(|_| panic!("credential identifier too long"))
                        .to_be_bytes(),
                );
                bytes.extend_from_slice(&self.credential_id);
                bytes.extend_from_slice(&self.cose_public_key());
            }
            bytes
        }

        fn cose_public_key(&self) -> Vec<u8> {
            let map = CborValue::Map(vec![
                (CborValue::Integer(1.into()), CborValue::Integer(1.into())),
                (
                    CborValue::Integer(3.into()),
                    CborValue::Integer((-8).into()),
                ),
                (
                    CborValue::Integer((-1).into()),
                    CborValue::Integer(6.into()),
                ),
                (
                    CborValue::Integer((-2).into()),
                    CborValue::Bytes(self.signing_key.verifying_key().to_bytes().to_vec()),
                ),
            ]);
            let mut bytes = Vec::new();
            required(
                ciborium::ser::into_writer(&map, &mut bytes),
                "encode public key",
            );
            bytes
        }
    }

    fn client_data(kind: &str, challenge: &str) -> Vec<u8> {
        required(
            serde_json::to_vec(&json!({
                "type": kind,
                "challenge": challenge,
                "origin": ARCHIVE_ORIGIN,
                "crossOrigin": false,
            })),
            "encode client data",
        )
    }

    fn open_session(
        passkeys: &Passkeys,
        scope: &mut PrincipalScope<'_>,
        authenticator: &mut ArchiveAuthenticator,
        now: u64,
    ) -> SessionGrant {
        let identity = required(
            AccountIdentity::new("alice@example.com", "Alice"),
            "identity",
        );
        let registration = required(
            passkeys.begin_registration(scope, &identity, "Archive passkey", now),
            "begin registration",
        );
        let response = authenticator.register(&registration.ceremony);
        required(
            passkeys.finish_registration(scope, &registration.registration_id, &response, now + 1),
            "finish registration",
        );
        let assertion = required(passkeys.begin_assertion(scope, now + 2), "begin assertion");
        let response = authenticator.assert(&assertion.ceremony);
        required(
            passkeys.finish_assertion(scope, &assertion.assertion_id, &response, now + 3),
            "finish assertion",
        );
        let device = required(
            Device::new("dev_aabbccddeeff00112233445566778899", "Phone", "ios"),
            "device",
        );
        required(
            passkeys.open_session(scope, &assertion.assertion_id, device, now + 4),
            "open session",
        )
    }

    #[test]
    #[allow(clippy::too_many_lines)]
    pub fn archive_requires_committed_disposition_and_step_up_then_retires_authority_once() {
        assert_eq!(ARCHIVE_CONFIRMATION_TONE, "danger");
        assert_eq!(ARCHIVE_ACTION_LABEL, "Archive agent");
        assert!(ARCHIVE_IRREVERSIBILITY_NOTICE.contains("never"));
        let fixture = Fixture::new("agent-archive-real");
        let archive_did =
            Did::new(b"did:layerx:worker").unwrap_or_else(|error| panic!("archive DID: {error:?}"));
        let authority = AuthorityLayer::new(
            &fixture.root.join("archive-authority-agentd"),
            fixture.principal.clone(),
            archive_did.clone(),
            fixture.public_key,
        );
        let mut sessions =
            SessionKeyProvisioner::new(authority, ArchiveEntropy, archive_policy(), registry());
        let lease = sessions
            .provision(&fixture.principal, &archive_did, vec![activity_type()])
            .unwrap_or_else(|error| panic!("archive session: {error}"));
        assert!(sessions.contract().daemon_open(lease.grant_id));
        assert!(sessions.contract().authorizes(lease.grant_id));
        let adapter = SessionArchiveAdapter::new(sessions);
        let mut boundary = AckGap {
            inner: adapter,
            fired: false,
        };

        let history_key = RowKey::new("agent-worker-receipt-history")
            .unwrap_or_else(|error| panic!("history key: {error}"));
        let mut store = fixture.store();
        let mut scope = store
            .principal(&fixture.principal)
            .unwrap_or_else(|error| panic!("archive scope: {error}"));
        scope
            .put(
                Table::Cache,
                history_key.clone(),
                590,
                b"immutable worker receipt history".to_vec(),
            )
            .unwrap_or_else(|error| panic!("history row: {error}"));
        let request = ArchiveRequest {
            idempotency_key: [0x76; 32],
            agent_id: AgentId::new("agt_worker")
                .unwrap_or_else(|error| panic!("agent ID: {error}")),
            agent_name: "worker".to_owned(),
            did: archive_did.clone(),
            history: vec![EvidenceRef::new(Table::Cache, history_key.clone())],
        };
        assert!(matches!(
            ArchiveJourney::start(&mut scope, &mut boundary, &request, &[], 600),
            Err(ArchiveError::FundsRemain(remaining)) if remaining[0].amount == 1
        ));
        drop(scope);

        let funds = &mut boundary.inner.sessions_mut().contract_mut().funds;
        funds.before[0].amount = 0;
        funds.after[0].amount = 0;

        let mut store = fixture.store();
        let mut scope = store
            .principal(&fixture.principal)
            .unwrap_or_else(|error| panic!("archive start scope: {error}"));
        for rung in [Rung::Submitted, Rung::Receipted] {
            boundary.inner.sessions_mut().contract_mut().funds.rung = rung;
            assert!(matches!(
                ArchiveJourney::start(&mut scope, &mut boundary, &request, &[], 630),
                Err(ArchiveError::UnverifiedDisposition)
            ));
            assert!(ArchiveJourney::load(&scope, request.idempotency_key)
                .unwrap()
                .is_none());
            assert_eq!(boundary.inner.sessions().contract().suspension_effects, 0);
            assert_eq!(boundary.inner.sessions().contract().revocation_effects, 0);
            assert!(boundary
                .inner
                .sessions()
                .contract()
                .daemon_open(lease.grant_id));
            assert!(boundary
                .inner
                .sessions()
                .contract()
                .authorizes(lease.grant_id));
        }
        boundary.inner.sessions_mut().contract_mut().funds.rung = Rung::Committed;
        let mut archive = ArchiveJourney::start(&mut scope, &mut boundary, &request, &[], 630)
            .unwrap_or_else(|error| panic!("archive start: {error}"));
        assert_eq!(
            required(archive.status(), "archive status after name mismatch").stage(),
            ArchiveStage::AwaitingConfirmation
        );

        let passkeys = required(Passkeys::new(auth_config()), "passkeys");
        let mut authenticator = ArchiveAuthenticator::new();
        let session = open_session(&passkeys, &mut scope, &mut authenticator, 631);
        let challenge = required(
            passkeys.begin_step_up(
                &mut scope,
                session.access_token().expose(),
                session.csrf_token().expose(),
                archive.operation_digest(),
                636,
            ),
            "begin archive step-up",
        );
        let assertion = authenticator.assert(&challenge.ceremony);
        let evidence = required(
            passkeys.finish_step_up(&mut scope, &challenge.challenge_id, &assertion, 637),
            "finish archive step-up",
        );
        assert!(matches!(
            archive.confirm(
                &mut scope,
                &passkeys,
                session.access_token().expose(),
                session.csrf_token().expose(),
                &evidence,
                "Worker",
                &mut boundary,
                &fixture.trace,
                638,
            ),
            Err(ArchiveError::ConfirmationMismatch)
        ));
        for rung in [Rung::Submitted, Rung::Receipted] {
            boundary.inner.sessions_mut().contract_mut().funds.rung = rung;
            assert!(matches!(
                archive.confirm(
                    &mut scope,
                    &passkeys,
                    session.access_token().expose(),
                    session.csrf_token().expose(),
                    &evidence,
                    "worker",
                    &mut boundary,
                    &fixture.trace,
                    638,
                ),
                Err(ArchiveError::UnverifiedDisposition)
            ));
            assert_ne!(archive.status().unwrap().stage(), ArchiveStage::Archived);
            assert_eq!(boundary.inner.sessions().contract().suspension_effects, 0);
            assert_eq!(boundary.inner.sessions().contract().revocation_effects, 0);
            assert!(boundary
                .inner
                .sessions()
                .contract()
                .authorizes(lease.grant_id));
            archive = ArchiveJourney::load(&scope, request.idempotency_key)
                .unwrap()
                .expect("pending archive survives reload");
        }
        boundary.inner.sessions_mut().contract_mut().funds.rung = Rung::Committed;
        let lost_ack = archive.confirm(
            &mut scope,
            &passkeys,
            session.access_token().expose(),
            session.csrf_token().expose(),
            &evidence,
            "worker",
            &mut boundary,
            &fixture.trace,
            638,
        );
        assert!(matches!(
            lost_ack,
            Err(ArchiveError::Agent(AgentContractError::Unavailable))
        ));
        drop(scope);

        let mut store = fixture.store();
        let mut scope = store
            .principal(&fixture.principal)
            .unwrap_or_else(|error| panic!("archive retry scope: {error}"));
        let mut archive = ArchiveJourney::load(&scope, [0x76; 32])
            .unwrap_or_else(|error| panic!("archive reload: {error}"))
            .unwrap_or_else(|| panic!("archive missing"));
        let status = archive
            .confirm(
                &mut scope,
                &passkeys,
                session.access_token().expose(),
                session.csrf_token().expose(),
                &evidence,
                "worker",
                &mut boundary,
                &fixture.trace,
                639,
            )
            .unwrap_or_else(|error| panic!("archive retry: {error}"));
        assert_eq!(status.stage(), ArchiveStage::Archived);
        assert!(status.irreversible());
        assert_eq!(status.history_entries(), 1);
        assert!(status.suspension_receipt_digest().is_some());
        assert!(status.revocation_receipt_digest().is_some());
        let sessions = boundary.inner.sessions_mut();
        assert_eq!(sessions.contract().suspension_effects, 1);
        assert_eq!(sessions.contract().revocation_effects, 1);
        assert!(!sessions.contract().daemon_open(lease.grant_id));
        assert!(!sessions.contract().authorizes(lease.grant_id));
        assert!(matches!(
            sessions.resume(&fixture.principal, &archive_did),
            Err(SessionKeyError::Archived)
        ));
        let suspension_receipt_digest = status
            .suspension_receipt_digest()
            .unwrap_or_else(|| panic!("archive suspension receipt missing"));
        let revocation_receipt_digest = status
            .revocation_receipt_digest()
            .unwrap_or_else(|| panic!("archive revocation receipt missing"));
        assert!(matches!(
            sessions
                .session(&fixture.principal, &archive_did)
                .map(|lease| &lease.state),
            Some(SessionLeaseState::Revoked {
                reason: RevocationReason::Archived,
                suspension_receipt_digest: stored_suspension_digest,
                revocation_receipt_digest: stored_revocation_digest,
                ..
            }) if *stored_suspension_digest == suspension_receipt_digest
                && *stored_revocation_digest == revocation_receipt_digest
        ));

        let retained = archive
            .history(&scope)
            .unwrap_or_else(|error| panic!("archived history: {error}"));
        assert_eq!(retained[0].bytes(), b"immutable worker receipt history");
        let audit =
            AuditChain::open(&scope).unwrap_or_else(|error| panic!("archive audit: {error}"));
        let entries = audit
            .entries(&scope)
            .unwrap_or_else(|error| panic!("archive audit entries: {error}"));
        assert!(entries.iter().any(|entry| matches!(
            entry.event(),
            AuditEvent::SecurityChange {
                change: SecurityChangeKind::AgentArchive,
                ..
            }
        )));
        assert!(scope
            .keys(Table::Notifications)
            .iter()
            .any(|key| key.as_str().starts_with("agent-archive-notification-")));
        let report = scope
            .expire(10_000_000)
            .unwrap_or_else(|error| panic!("archive retention: {error}"));
        assert!(report.pinned_evidence_retained > 0);
        assert_eq!(
            archive
                .history(&scope)
                .unwrap_or_else(|error| panic!("retained archive history: {error}"))[0]
                .bytes(),
            b"immutable worker receipt history"
        );

        let repeated = archive
            .confirm(
                &mut scope,
                &passkeys,
                session.access_token().expose(),
                session.csrf_token().expose(),
                &evidence,
                "worker",
                &mut boundary,
                &fixture.trace,
                1_002,
            )
            .unwrap_or_else(|error| panic!("archive terminal retry: {error}"));
        assert_eq!(repeated, status);
        assert_eq!(boundary.inner.sessions().contract().suspension_effects, 1);
        assert_eq!(boundary.inner.sessions().contract().revocation_effects, 1);
    }
}
#[test]
fn archive_waits_for_committed_fund_disposition() {
    archive_fixture::archive_requires_committed_disposition_and_step_up_then_retires_authority_once(
    );
}
#[test]
fn agent_controls_refuse_a_change_read_below_committed() {
    control_fixture::refuse_uncommitted_reads();
}
