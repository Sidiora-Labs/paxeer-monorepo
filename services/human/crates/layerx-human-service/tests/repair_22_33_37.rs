//! Repair 22.33.37 task record (acceptance 22998.1).
//! archive-history-mutation: archive_history_detects_replacement_and_missing_after_reopen.
//! control-state-restart: completed_limit_retry_recovers_exact_authority_after_reopen.
//! reclaim-old-key-retry: reclaim_exact_old_key_retry_survives_later_nonce_and_reopen.
//! spend-never-reconciles: spend_uses_verified_budget_and_receipts_after_reopen.
//! recovery-production-gap: production recovery remains unavailable; the current chain
//! lifecycle exposes policy registration but no supported recovery authority submission.
//! Wiring request (authentication domain): modules/layerxauth/keeper/lifecycle.go must expose
//! a supported chain recovery transition; RecordLifecycle currently admits only tags 1 and 3.
//! Attempt 2 reported control and archive cases failing; reclaim and spend passed.
//! Attempt 3 retains those cases and repairs receipt-version binding and retained-row write admission.
//! Baseline verification was not run: implementation-first and hf-done-only gate order
//! supersedes step 9's unchanged-tree verification; no baseline failures are claimed.

#[path = "support/canonical_owner.rs"]
mod canonical_owner;

mod control_proof {
    use super::canonical_owner;
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
    use layerx_proof::receipt::AuthorizedBatch;
    use layerx_types::account::AccountId;
    use layerx_types::ids::{AssetId, Did};
    use layerx_types::intent::PurposeHash;
    use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};
    use layerx_types::rung::Rung;
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
        active_budget_id: [u8; 32],
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
                active_budget_id: AGENT_ID,
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
                rung: layerx_types::rung::Rung::Committed,
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
                rung: layerx_types::rung::Rung::Receipted,
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
                    reason: InvalidationReason::GrantRevokedOnChain,
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
                rung: layerx_types::rung::Rung::Receipted,
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
            layerx_human_service::server::agent_creation::verify_evm_outcome(
                &receipt,
                action.compiled.activity_type(),
            )?;
            decoded_limit(action.intent.kind())?;
            let finalization = ProductionAgentCreation::finalization_evidence(
                &receipt,
                ModuleId::Budget,
                1,
                self.now,
            )?;
            if finalization.action_key != action.action_key
                || finalization.activity_id != receipt.activity_id
                || finalization.observed_sequence != 11
                || finalization.verification != layerx_types::rung::Rung::Committed.wire_rank()
            {
                return Err(AgentFailure::Refused(
                    "protocol budget finalization mismatch",
                ));
            }
            self.protocol_limit_effects = self.protocol_limit_effects.saturating_add(1);
            self.active_budget_id = action.action_key;
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
        let owner = canonical_owner::sign_transaction(
            payload,
            action_key,
            STATE_COMMITMENT_PROTOCOL_VERSION,
            77,
        );

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
        let persisted = encode_receipt(&fields, None);
        let decoded = layerx_wire::receipt::decode(&persisted).expect("decode retained receipt");
        let unsigned = layerx_wire::receipt::encode_unsigned(&decoded)
            .expect("reproduce receipt signing preimage");
        assert_eq!(
            unsigned, persisted,
            "retained signing bytes reproduce exactly"
        );
        let signature = signer
            .sign(&layerx_wire::hash::receipt_digest(&unsigned).expect("canonical receipt digest"));
        let receipt_bytes = encode_receipt(&fields, Some(signature.to_bytes()));
        let evidence = ProtocolEvidence {
            signed_activity: owner.bytes,
            actor: owner.public_key[12..].to_vec(),
            owner_public_key: owner.public_key,
            network_id: 77,
            action_key,
            activity_id: owner.id,
            receipt_hash: Sha256::digest(&receipt_bytes).into(),
            receipt_bytes,
            tx_hash: fields.activity_id,
            block_height: 11,
            rung: layerx_types::rung::Rung::Committed,
        };
        let authorized_batch = AuthorizedBatch::new(
            fields.batch_id,
            [0; 32],
            fields.previous_state_root,
            fields.resulting_state_root,
            signer.verifying_key().to_bytes(),
        );
        evidence
            .bound_activity(payload.activity_type())
            .expect("bound owner transaction");
        let keys = [layerx_proof::inclusion::SequencerAuthorization::new(
            [0x86; 32],
            signer.verifying_key().to_bytes(),
            11,
            11,
        )];
        let receipt = layerx_proof::receipt::verify_sequencer_signature(
            &evidence.receipt_bytes,
            keys.as_slice(),
        )
        .expect("owner receipt with sequencer key pinned to its context and height");
        let protocol = receipt.protocol().expect("protocol receipt");
        assert_eq!(protocol.batch_id(), authorized_batch.batch_id());
        assert_eq!(
            protocol.previous_state_root(),
            authorized_batch.previous_state_root()
        );
        assert_eq!(
            protocol.resulting_state_root(),
            authorized_batch.resulting_state_root()
        );
        let receipt = layerx_human_service::server::agent_creation::verify_evm_outcome(
            &evidence,
            payload.activity_type(),
        )
        .expect("verified owner transaction outcome");
        let protocol = receipt.protocol().expect("protocol receipt");
        assert_eq!(evidence.tx_hash, protocol.activity_id());
        assert_eq!(evidence.tx_hash, evidence.activity_id);
        assert_eq!(evidence.rung, layerx_types::rung::Rung::Committed);
        assert_eq!(
            &protocol.batch_id()[24..],
            &evidence.block_height.to_be_bytes()
        );
        assert_eq!(
            evidence.receipt_hash,
            <[u8; 32]>::from(Sha256::digest(&evidence.receipt_bytes))
        );
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
        push_u16(&mut bytes, 12);
        bytes.extend_from_slice(&1_u32.to_be_bytes());
        bytes.extend_from_slice(&1_u32.to_be_bytes());
        bytes.push(2);
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
        push_bytes(&mut bytes, &[0x86; 32]);
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

    #[test]
    fn completed_limit_retry_recovers_exact_authority_after_reopen() {
        use layerx_human_service::store::PrincipalStore;
        use std::sync::{Arc, Mutex};
        let root = directory("repair-control-record");
        let (store, tenancy_digest) = support::install_and_open(
            &root,
            &support::tenancy(&[("alice", "tenant-a")]),
            support::retention_uniform(100_000),
        );
        let store = Arc::new(Mutex::new(store));
        let (initial, _) = controls(
            "repair-control-authority",
            LimitBacking::Protocol {
                active_budget_id: AGENT_ID,
            },
        );
        let mut controls = AgentControls::open(
            initial.into_boundary(),
            profile(),
            1_000,
            LimitBacking::Protocol {
                active_budget_id: AGENT_ID,
            },
            1_000,
            store.clone(),
        )
        .expect("open durable controls");
        let first = LimitChangeRequest::new([0x71; 32], 500).expect("first limit");
        let completed = controls
            .change_limit(Rung::Committed, &registry(), first, 1_002)
            .expect("receipt-backed limit");
        assert_eq!(completed.limit.monthly, 500);
        assert_eq!(
            controls
                .boundary()
                .sessions()
                .contract()
                .protocol_limit_effects,
            1
        );
        let first_budget = controls.boundary().sessions().contract().active_budget_id;
        let authority = controls.into_boundary();
        drop(store);
        let store = Arc::new(Mutex::new(
            PrincipalStore::open(&root, support::retention_uniform(100_000), tenancy_digest)
                .expect("reopen control store"),
        ));
        let mut recovered = AgentControls::open(
            authority,
            profile(),
            1_000,
            LimitBacking::Protocol {
                active_budget_id: AGENT_ID,
            },
            1_010,
            store.clone(),
        )
        .expect("reconstruct controls");
        assert_eq!(
            recovered
                .change_limit(Rung::Committed, &registry(), first, 1_010)
                .expect("exact old retry"),
            completed
        );
        assert_eq!(
            recovered
                .boundary()
                .sessions()
                .contract()
                .protocol_limit_effects,
            1
        );
        assert_eq!(
            recovered.boundary().sessions().contract().active_budget_id,
            first_budget
        );
        let second = LimitChangeRequest::new([0x72; 32], 400).expect("second limit");
        let later = recovered
            .change_limit(Rung::Committed, &registry(), second, 1_011)
            .expect("next budget");
        assert_eq!(later.limit.monthly, 400);
        let expected = digest(&[
            b"layerx-human/agent-limit-budget/v1",
            &first_budget,
            &digest(&[
                b"layerx-human/agent-control-action/v1",
                &AGENT_ID,
                &second.idempotency_key,
                &second.monthly_limit.to_be_bytes(),
            ]),
        ]);
        assert_eq!(
            recovered.boundary().sessions().contract().active_budget_id,
            expected,
            "replacement derives from recovered active authority"
        );
        assert_eq!(
            recovered
                .change_limit(Rung::Committed, &registry(), first, 1_012)
                .expect("older completed retry"),
            completed
        );
        assert!(matches!(
            recovered.change_limit(
                Rung::Committed,
                &registry(),
                LimitChangeRequest::new(first.idempotency_key, 600).expect("conflict request"),
                1_012
            ),
            Err(AgentControlError::IdempotencyConflict)
        ));
        assert_eq!(
            recovered
                .boundary()
                .sessions()
                .contract()
                .protocol_limit_effects,
            2
        );
        let authority = recovered.into_boundary();
        drop(store);
        let store = Arc::new(Mutex::new(
            PrincipalStore::open(&root, support::retention_uniform(100_000), tenancy_digest)
                .expect("second reopen"),
        ));
        let recovered = AgentControls::open(
            authority,
            profile(),
            1_000,
            LimitBacking::Protocol {
                active_budget_id: AGENT_ID,
            },
            1_013,
            store.clone(),
        )
        .expect("second reconstruction");
        assert_eq!(recovered.view(1_013).expect("current view"), later);
        let mut foreign_profile = profile();
        foreign_profile.principal = support::principal("bob");
        assert!(AgentControls::open(
            recovered.into_boundary(),
            foreign_profile,
            1000,
            LimitBacking::Protocol {
                active_budget_id: AGENT_ID
            },
            1_013,
            store.clone()
        )
        .is_err());
        drop(store);
        fs::remove_dir_all(root).expect("remove control store");
    }

    fn digest(parts: &[&[u8]]) -> [u8; 32] {
        let mut hash = Sha256::new();
        for part in parts {
            hash.update(part);
        }
        hash.finalize().into()
    }
}

mod archive_proof {

    use ed25519_dalek::{Signer as _, SigningKey};
    use layerx_agentd::store::{Store as AgentStore, TenantId};
    use layerx_human_service::store::{PrincipalId, PrincipalScope, PrincipalStore, Table};
    use layerx_human_test_support::{
        directory, install_and_open, principal, retention_uniform, tenancy,
    };
    use layerx_types::ids::{AssetId, Did};
    use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};
    use sha2::{Digest as _, Sha256};
    use std::collections::{BTreeMap, BTreeSet};
    fn registry() -> ModuleRegistry {
        ModuleRegistry::new(
            &[ModuleRegistration::new(ModuleId::Asset, &[activity_type()])
                .expect("asset registration")],
        )
        .expect("registry")
    }
    fn activity_type() -> ActivityType {
        ActivityType::new(ModuleId::Asset, 5).expect("activity type")
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
        ArchiveRequest, ArchiveStage, FundsDispositionEvidence, ReclaimStatus,
        SessionArchiveAdapter, ARCHIVE_ACTION_LABEL, ARCHIVE_CONFIRMATION_TONE,
        ARCHIVE_IRREVERSIBILITY_NOTICE,
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
    use layerx_human_service::store::{EvidenceRef, RowKey};
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
                    rung: layerx_types::rung::Rung::Committed,
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
                rung: layerx_types::rung::Rung::Committed,
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
                rung: layerx_types::rung::Rung::Receipted,
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
                    reason: InvalidationReason::GrantRevokedOnChain,
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
                rung: layerx_types::rung::Rung::Receipted,
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
    fn archive_history_detects_replacement_and_missing_after_reopen() {
        let root = directory("repair-archive-history");
        let (mut store, digest) = install_and_open(
            &root.join("human"),
            &tenancy(&[("alice", "tenant-a")]),
            retention_uniform(100_000),
        );
        let principal = principal("alice");
        let did = Did::new(b"did:layerx:worker").expect("agent DID");
        let mut authority = AuthorityLayer::new(
            &root.join("authority"),
            principal.clone(),
            did.clone(),
            [0x11; 32],
        );
        authority.funds.before[0].amount = 0;
        authority.funds.after[0].amount = 0;
        let mut sessions =
            SessionKeyProvisioner::new(authority, ArchiveEntropy, archive_policy(), registry());
        sessions
            .provision(&principal, &did, vec![activity_type()])
            .expect("real session authority");
        let mut boundary = SessionArchiveAdapter::new(sessions);
        let history = RowKey::new("retained-agent-history").expect("history key");
        let mut scope = store.principal(&principal).expect("scope");
        scope
            .put(
                Table::Cache,
                history.clone(),
                1_000,
                b"original receipt timeline".to_vec(),
            )
            .expect("history producer");
        let request = ArchiveRequest {
            idempotency_key: [0x76; 32],
            agent_id: AgentId::new("agt_worker").expect("agent id"),
            agent_name: "worker".to_owned(),
            did,
            history: vec![EvidenceRef::new(Table::Cache, history.clone())],
        };
        let mut journey = ArchiveJourney::start(&mut scope, &mut boundary, &request, &[], 1_001)
            .expect("start actual archive");
        let passkeys = Passkeys::new(auth_config()).expect("passkeys");
        let mut authenticator = ArchiveAuthenticator::new();
        let session = open_session(&passkeys, &mut scope, &mut authenticator, 1_002);
        let ceremony = passkeys
            .begin_step_up(
                &mut scope,
                session.access_token().expose(),
                session.csrf_token().expose(),
                journey.operation_digest(),
                1_007,
            )
            .expect("step-up begin");
        let response = authenticator.assert(&ceremony.ceremony);
        let evidence = passkeys
            .finish_step_up(&mut scope, &ceremony.challenge_id, &response, 1_008)
            .expect("real passkey step-up");
        let trace =
            layerx_human_service::trace::TraceId::parse("trc_00112233445566778899aabbccddeeff")
                .expect("trace");
        scope
            .put(
                Table::Cache,
                history.clone(),
                1_000,
                b"changed receipt timeline".to_vec(),
            )
            .expect("replace unpinned history");
        assert!(matches!(
            journey.confirm(
                &mut scope,
                &passkeys,
                session.access_token().expose(),
                session.csrf_token().expose(),
                &evidence,
                "worker",
                &mut boundary,
                &trace,
                1_009,
            ),
            Err(ArchiveError::HistoryChanged)
        ));
        assert_eq!(boundary.sessions().contract().revocation_effects, 0);
        scope
            .remove(Table::Cache, &history)
            .expect("remove unpinned history");
        assert!(matches!(
            journey.confirm(
                &mut scope,
                &passkeys,
                session.access_token().expose(),
                session.csrf_token().expose(),
                &evidence,
                "worker",
                &mut boundary,
                &trace,
                1_009,
            ),
            Err(ArchiveError::HistoryMissing)
        ));
        assert_eq!(boundary.sessions().contract().revocation_effects, 0);
        scope
            .put(
                Table::Cache,
                history.clone(),
                1_000,
                b"original receipt timeline".to_vec(),
            )
            .expect("restore exact history");
        let completed = journey
            .confirm(
                &mut scope,
                &passkeys,
                session.access_token().expose(),
                session.csrf_token().expose(),
                &evidence,
                "worker",
                &mut boundary,
                &trace,
                1_009,
            )
            .expect("archive actual authority");
        assert_eq!(completed.stage(), ArchiveStage::Archived);
        assert_eq!(
            journey.history(&scope).expect("original history")[0].bytes(),
            b"original receipt timeline"
        );
        drop(scope);
        drop(journey);
        drop(store);
        let mut store =
            PrincipalStore::open(&root.join("human"), retention_uniform(100_000), digest)
                .expect("reopen archived store");
        let mut scope = store.principal(&principal).expect("reopened scope");
        let journey = ArchiveJourney::load(&scope, request.idempotency_key)
            .expect("load")
            .expect("archive");
        assert_eq!(journey.status().expect("durable status"), completed);
        assert_eq!(
            journey.history(&scope).expect("recovered history")[0].bytes(),
            b"original receipt timeline"
        );
        let replacement = scope.put(
            Table::Cache,
            history.clone(),
            1_000,
            b"replacement at the exact same key".to_vec(),
        );
        assert!(
            matches!(
                replacement,
                Err(layerx_human_service::store::StoreError::ArchiveHistoryChanged)
            ),
            "retained row replacement refusal: {replacement:?}"
        );
        let timestamp_change = scope.put(
            Table::Cache,
            history.clone(),
            1_001,
            b"original receipt timeline".to_vec(),
        );
        assert!(
            matches!(
                timestamp_change,
                Err(layerx_human_service::store::StoreError::ArchiveHistoryChanged)
            ),
            "retained row timestamp refusal: {timestamp_change:?}"
        );
        let identical = scope.put(
            Table::Cache,
            history.clone(),
            1_000,
            b"original receipt timeline".to_vec(),
        );
        assert!(
            matches!(
                identical,
                Err(layerx_human_service::store::StoreError::EvidencePinned)
            ),
            "unchanged evidence remains pinned: {identical:?}"
        );
        let unrelated = RowKey::new("unrelated-history-batch-row").expect("batch key");
        let batch = scope.put_batch(
            1_000,
            vec![
                (
                    Table::Cache,
                    unrelated.clone(),
                    b"must not be written".to_vec(),
                ),
                (
                    Table::Cache,
                    history.clone(),
                    b"replacement at the exact same key".to_vec(),
                ),
            ],
        );
        assert!(
            matches!(
                batch,
                Err(layerx_human_service::store::StoreError::ArchiveHistoryChanged)
            ),
            "retained row batch refusal: {batch:?}"
        );
        assert!(
            scope.get(Table::Cache, &unrelated).is_none(),
            "refused batch is atomic"
        );
        assert_eq!(
            journey
                .history(&scope)
                .expect("history after refused writes")[0]
                .bytes(),
            b"original receipt timeline"
        );
        assert!(matches!(
            scope.remove(Table::Cache, &history),
            Err(layerx_human_service::store::StoreError::EvidencePinned)
        ));
        drop(scope);
        drop(store);

        let path = root.join("human/principals/alice/store.bin");
        let original = std::fs::read(&path).expect("read durable store");
        let payload = b"original receipt timeline";
        let positions: Vec<_> = original
            .windows(payload.len())
            .enumerate()
            .filter_map(|(offset, bytes)| (bytes == payload).then_some(offset))
            .collect();
        assert_eq!(
            positions.len(),
            1,
            "the history row has one durable payload"
        );
        let offset = positions[0];
        assert_eq!(&original[offset - 12..offset - 4], &1_000_u64.to_be_bytes());
        let mut altered = original.clone();
        altered[offset] ^= 1;
        std::fs::write(&path, &altered).expect("simulate out-of-band row replacement");
        let mut store =
            PrincipalStore::open(&root.join("human"), retention_uniform(100_000), digest)
                .expect("reopen altered store");
        let scope = store
            .principal(&principal)
            .expect("decode replaced history row");
        let journey = ArchiveJourney::load(&scope, request.idempotency_key)
            .expect("load archived transition")
            .expect("archive exists");
        assert!(matches!(
            journey.history(&scope),
            Err(ArchiveError::HistoryChanged)
        ));
        drop(scope);
        drop(store);

        let mut altered = original.clone();
        altered[offset - 12..offset - 4].copy_from_slice(&1_001_u64.to_be_bytes());
        std::fs::write(&path, &altered).expect("simulate changed row timestamp");
        let mut store =
            PrincipalStore::open(&root.join("human"), retention_uniform(100_000), digest)
                .expect("reopen changed timestamp");
        let scope = store
            .principal(&principal)
            .expect("decode timestamp change");
        let journey = ArchiveJourney::load(&scope, request.idempotency_key)
            .expect("load archived transition")
            .expect("archive exists");
        assert!(matches!(
            journey.history(&scope),
            Err(ArchiveError::HistoryChanged)
        ));
        drop(scope);
        drop(store);
        drop(boundary);
        std::fs::remove_dir_all(root).expect("remove archive state");
    }
}

mod reclaim_proof {
    use layerx_agent_api::identity::{AgentDid, AuthorityRef};
    use layerx_human_service::agents::{
        Reclaim, ReclaimAgentContext, ReclaimError, ReclaimMechanism, ReclaimRequest, ReclaimStage,
    };
    use layerx_human_service::custody::KeyId;
    use layerx_human_service::journeys::JourneyEngine;
    use layerx_human_service::notify::JourneyId;
    use layerx_human_service::store::{PrincipalScope, PrincipalStore, Table};
    use layerx_human_test_support::{directory, principal, retention_uniform, tenancy};
    use layerx_types::account::AccountId;
    use layerx_types::amount::Amount;
    use layerx_types::ids::AssetId;
    use layerx_types::intent::{BudgetId, Sequence};
    use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};

    fn registry() -> ModuleRegistry {
        ModuleRegistry::new(&[ModuleRegistration::new(
            ModuleId::Budget,
            &[ActivityType::new(ModuleId::Budget, 8).expect("defund activity")],
        )
        .expect("budget registration")])
        .expect("module registry")
    }

    fn request(key: u8, nonce: u64, agent: &str) -> ReclaimRequest {
        ReclaimRequest {
            journey_id: JourneyId::new(format!("jrn_reclaimnonce{key}")).expect("journey id"),
            idempotency_key: [key; 32],
            owner: AccountId::parse("agent:did:layerx:human:main").expect("owner account"),
            agent_account: AccountId::parse(&format!("agent:did:layerx:{agent}:main"))
                .expect("agent account"),
            asset: AssetId::new([0x33; 32]),
            amount: Amount::from_u128(1),
            mechanism: ReclaimMechanism::BudgetDefund {
                budget_account: AccountId::parse(&format!(
                    "agent:did:layerx:{agent}:budget:operations"
                ))
                .expect("budget account"),
                budget_id: BudgetId::new([0x41; 32]),
                revocation_sequence: Sequence::from_u64(4),
            },
            agent: ReclaimAgentContext {
                label: "worker".to_owned(),
                actor: AgentDid::new(format!("did:layerx:{agent}")).expect("agent DID"),
                authority: AuthorityRef::new("custody-human-primary").expect("authority"),
                custody_key: KeyId::new("human-primary").expect("custody key"),
                account_sequence: nonce,
                nonce,
                not_before: 995,
                not_after: 1_010,
                fee_limit: 7,
            },
        }
    }

    #[test]
    fn reclaim_exact_old_key_retry_survives_later_nonce_and_reopen() {
        let root = directory("repair-reclaim-old-key");
        let digest = tenancy(&[("alice", "tenant-a"), ("bob", "tenant-b")])
            .install(&root)
            .expect("tenancy");
        let owner = principal("alice");
        let first = request(1, 40, "worker");
        let registry = registry();
        let mut store =
            PrincipalStore::open(&root, retention_uniform(10_000), digest).expect("store");
        let mut scope = store.principal(&owner).expect("scope");
        let initial = Reclaim::start(&mut scope, &first, &registry, 1_000).expect("first reclaim");
        Reclaim::start(&mut scope, &request(2, 41, "worker"), &registry, 1_001)
            .expect("next reclaim");
        drop(scope);
        drop(store);
        let mut store =
            PrincipalStore::open(&root, retention_uniform(10_000), digest).expect("reopened store");
        let mut scope = store.principal(&owner).expect("reopened scope");
        let keys = scope.keys(Table::Journeys);
        assert_eq!(
            Reclaim::start(&mut scope, &first, &registry, 1_002).expect("exact older retry"),
            initial
        );
        assert_eq!(
            scope.keys(Table::Journeys),
            keys,
            "retry creates no journey or authority"
        );
        let mut conflict = first.clone();
        conflict.amount = Amount::from_u128(2);
        assert!(matches!(
            Reclaim::start(&mut scope, &conflict, &registry, 1_002),
            Err(ReclaimError::IdempotencyConflict)
        ));
        assert!(matches!(
            Reclaim::start(&mut scope, &request(3, 40, "worker"), &registry, 1_002),
            Err(ReclaimError::StaleNonce {
                nonce: 40,
                last_used: 41
            })
        ));
        assert!(Reclaim::load(&scope, &request(3, 40, "worker").journey_id)
            .expect("absent stale journey")
            .is_none());
        assert!(
            matches!(
                Reclaim::start(&mut scope, &request(4, 40, "worker"), &registry, 1_003),
                Err(ReclaimError::StaleNonce { last_used: 41, .. })
            ),
            "old retry must not lower the high-water mark"
        );
        assert_eq!(scope.keys(Table::Journeys), keys);
        drop(scope);
        let scope = store
            .principal(&principal("bob"))
            .expect("foreign principal scope");
        assert!(Reclaim::load(&scope, &first.journey_id)
            .expect("isolated lookup")
            .is_none());
        drop(scope);
        drop(store);
        std::fs::remove_dir_all(root).expect("remove reclaim state");
    }
}

mod spend_proof {
    use ed25519_dalek::SigningKey;
    use layerx_agentd::budget::{
        verify_budget_proof, BudgetProofTrust, ProtocolBudgetState, SpendReceiptEvidence,
    };
    use layerx_agentd::read::ServedCheckpoint;
    use layerx_client::sequencer::SequencerEndpoint;
    use layerx_human_service::agents::{
        ReconciliationDirection, SpendError, SpendProfile, SpendReconciliation,
        SpendReconciliationStatus,
    };
    use layerx_human_service::store::PrincipalStore;
    use layerx_human_test_support::{
        directory, install_and_open, principal, retention_uniform, tenancy,
    };
    use layerx_types::ids::Did;
    use layerx_types::rung::Rung;
    use serde_json::{json, Value};
    use std::io::{Read as _, Write as _};
    use std::sync::{
        atomic::{AtomicBool, Ordering},
        Arc,
    };

    const ANSWERS: &str =
        include_str!("../../../../../tests/vectors/budget-proof-v2/node-answers.json");
    const EXPORT: &[u8] =
        include_bytes!("../../../../../tests/vectors/budget-proof-v2/export.json");
    const OWNER: &str =
        "did:layerx:0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a";
    fn budget_id() -> [u8; 32] {
        let mut id = [0; 32];
        id[31] = 7;
        id
    }
    fn profile() -> SpendProfile {
        SpendProfile {
            principal: principal("operator"),
            agent_id: [0x31; 32],
            agent_account: [2; 32],
            budget_id: budget_id(),
            asset: [4; 32],
        }
    }
    fn hex(text: &str) -> [u8; 32] {
        let bytes: Vec<_> = text
            .as_bytes()
            .chunks_exact(2)
            .map(|b| {
                u8::from_str_radix(std::str::from_utf8(b).expect("hex"), 16).expect("hex byte")
            })
            .collect();
        bytes.try_into().expect("digest width")
    }
    use ed25519_dalek::Signer as _;
    use layerx_agentd::protocol_evidence::RawReceiptEvidence;
    use layerx_human_service::journeys::ReceiptMaterial;
    use layerx_human_test_support as support;
    use layerx_proof::receipt::AuthorizedBatch;
    use layerx_types::payload::ModuleId;
    use sha2::{Digest as _, Sha256};
    #[derive(Clone)]
    struct SpendReceipt {
        material: ReceiptMaterial,
        authorized_batch: AuthorizedBatch,
        evidence: RawReceiptEvidence,
    }

    struct ReceiptFields {
        activity_id: [u8; 32],
        sequence: u64,
        previous_state_root: [u8; 32],
        resulting_state_root: [u8; 32],
        batch_id: [u8; 32],
        asset: [u8; 32],
        amount: u128,
        counterparty: [u8; 32],
    }

    fn signed_receipt(
        sequence: u64,
        activity_id: [u8; 32],
        amount: u128,
        asset: [u8; 32],
        counterparty: [u8; 32],
    ) -> SpendReceipt {
        let previous_state_root: [u8; 32] =
            Sha256::digest([b"before".as_slice(), &activity_id].concat()).into();
        let fields = ReceiptFields {
            activity_id,
            sequence,
            previous_state_root,
            resulting_state_root: Sha256::digest([b"after".as_slice(), &activity_id].concat())
                .into(),
            batch_id: support::committed_execution_batch_id(
                previous_state_root,
                [0x81; 32],
                sequence,
            ),
            asset,
            amount,
            counterparty,
        };
        let signer = SigningKey::from_bytes(&[0x84; 32]);
        let unsigned = encode_receipt(&fields, None);
        let mut digest = Sha256::new();
        digest.update(b"LXP/v1/receipt\0");
        digest.update(&unsigned);
        let signature = signer.sign(&<[u8; 32]>::from(digest.finalize()));
        let canonical_receipt = encode_receipt(&fields, Some(signature.to_bytes()));
        let authorized_batch = AuthorizedBatch::new(
            fields.batch_id,
            fields.asset,
            fields.previous_state_root,
            fields.resulting_state_root,
            signer.verifying_key().to_bytes(),
        );
        let decoded = layerx_intents::canonical::decode_receipt(&canonical_receipt)
            .unwrap_or_else(|error| panic!("spend producer receipt: {error:?}"));
        let protocol = decoded.protocol().expect("spend protocol receipt");
        assert_eq!(protocol.previous_state_root(), fields.previous_state_root);
        assert_eq!(protocol.resulting_state_root(), fields.resulting_state_root);
        let evidence = support::raw_receipt_evidence(
            canonical_receipt.clone(),
            signer.verifying_key().to_bytes(),
            fields.previous_state_root,
            fields.resulting_state_root,
            sequence,
            &signer,
        );
        let material = ReceiptMaterial {
            authorised_batch: authorized_batch,
            verification_level: layerx_types::verify::VerificationLevel::SEQUENCER_SIGNED,
            receipt_hash: Sha256::digest(&canonical_receipt).into(),
            signed_receipt: canonical_receipt.clone(),
            canonical_bytes: canonical_receipt,
            tx_hash: activity_id,
            block_height: sequence,
            rung: layerx_types::rung::Rung::Receipted,
            receipt_key: layerx_human_service::journeys::ReceiptKeyEntry {
                key_id: [0x94; 32],
                public_key: signer.verifying_key().to_bytes(),
                first_height: 1,
                last_height: u64::MAX,
            },
            inclusion_height: sequence,
        };
        let receipt = layerx_intents::canonical::decode_receipt(&material.canonical_bytes)
            .unwrap_or_else(|error| panic!("spend receipt: {error:?}"));
        let protocol = receipt.protocol().expect("protocol spend receipt");
        assert_eq!(material.tx_hash, protocol.activity_id());
        assert_eq!(material.block_height, protocol.global_sequence());
        assert_eq!(
            material.receipt_hash,
            <[u8; 32]>::from(Sha256::digest(&material.canonical_bytes))
        );
        assert_eq!(material.rung, layerx_types::rung::Rung::Receipted);
        SpendReceipt {
            material,
            authorized_batch,
            evidence,
        }
    }

    fn spend_activity_id(sequence: u64, idempotency_key: [u8; 32]) -> [u8; 32] {
        Sha256::digest(
            [
                b"layerx-human-spend-activity/v1".as_slice(),
                &sequence.to_be_bytes(),
                &idempotency_key,
            ]
            .concat(),
        )
        .into()
    }

    fn encode_receipt(fields: &ReceiptFields, signature: Option<[u8; 64]>) -> Vec<u8> {
        let debit_before = 10_000_u128;
        let credit_before = 20_000_u128;
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
        push_u16(&mut bytes, ModuleId::Asset as u16);
        bytes.extend_from_slice(&1_u32.to_be_bytes());
        bytes.extend_from_slice(&1_u32.to_be_bytes());
        bytes.push(1);
        push_bytes(&mut bytes, &fields.asset);
        bytes.extend_from_slice(&fields.amount.to_be_bytes());
        push_bytes(&mut bytes, &[2; 32]);
        bytes.extend_from_slice(&debit_before.to_be_bytes());
        bytes.extend_from_slice(&(debit_before - fields.amount).to_be_bytes());
        push_u64(&mut bytes, fields.sequence - 1_u64 + 1);
        push_bytes(&mut bytes, &fields.counterparty);
        bytes.extend_from_slice(&credit_before.to_be_bytes());
        bytes.extend_from_slice(&(credit_before + fields.amount).to_be_bytes());
        push_bytes(&mut bytes, &[0x91; 32]);
        push_bytes(&mut bytes, &[0x92; 32]);
        push_bytes(&mut bytes, &[0x93; 32]);
        push_u64(&mut bytes, 1_700_000_000_000_u64 + fields.sequence);
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
            u32::try_from(value.len()).unwrap_or_else(|_| panic!("receipt field overflow"));
        output.extend_from_slice(&length.to_be_bytes());
        output.extend_from_slice(value);
    }

    struct Replay {
        endpoint: SequencerEndpoint,
        stop: Arc<AtomicBool>,
        worker: Option<std::thread::JoinHandle<usize>>,
    }
    impl Replay {
        fn new(answers: Value) -> Self {
            let listener = std::net::TcpListener::bind("127.0.0.1:0").expect("bind RPC");
            listener
                .set_nonblocking(true)
                .expect("nonblocking listener");
            let address = listener.local_addr().expect("RPC address");
            let stop = Arc::new(AtomicBool::new(false));
            let stopped = Arc::clone(&stop);
            let worker = std::thread::spawn(move || {
                let mut calls = 0;
                while !stopped.load(Ordering::SeqCst) {
                    let mut stream = match listener.accept() {
                        Ok((stream, _)) => stream,
                        Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                            std::thread::sleep(std::time::Duration::from_millis(1));
                            continue;
                        }
                        Err(error) => panic!("RPC accept: {error}"),
                    };
                    stream
                        .set_read_timeout(Some(std::time::Duration::from_secs(5)))
                        .expect("read deadline");
                    let mut headers = Vec::new();
                    while !headers.ends_with(b"\r\n\r\n") {
                        let mut byte = [0];
                        stream.read_exact(&mut byte).expect("HTTP header");
                        headers.push(byte[0]);
                    }
                    let headers = String::from_utf8(headers).expect("headers");
                    let length: usize = headers
                        .lines()
                        .find_map(|line| {
                            let (name, value) = line.split_once(':')?;
                            name.eq_ignore_ascii_case("content-length")
                                .then(|| value.trim().parse().expect("length"))
                        })
                        .expect("content length");
                    let mut body = vec![0; length];
                    stream.read_exact(&mut body).expect("RPC body");
                    let request: Value = serde_json::from_slice(&body).expect("RPC JSON");
                    let answer = answers
                        .as_array()
                        .expect("answers")
                        .iter()
                        .find(|answer| {
                            answer["method"] == request["method"]
                                && answer["params"] == request["params"]
                        })
                        .expect("recorded method and params");
                    if request["method"] == "eth_call" {
                        calls += 1;
                    }
                    let body =
                        json!({"jsonrpc":"2.0", "id":request["id"], "result":answer["result"]})
                            .to_string();
                    write!(stream,"HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{}",body.len(),body).expect("RPC reply");
                }
                calls
            });
            Self {
                endpoint: SequencerEndpoint::new(&format!("http://{address}"), &[])
                    .expect("endpoint"),
                stop,
                worker: Some(worker),
            }
        }
        fn finish(mut self) -> usize {
            self.stop.store(true, Ordering::SeqCst);
            self.worker
                .take()
                .expect("worker")
                .join()
                .expect("RPC replay")
        }
    }
    impl Drop for Replay {
        fn drop(&mut self) {
            self.stop.store(true, Ordering::SeqCst);
            if let Some(worker) = self.worker.take() {
                let _ = worker.join();
            }
        }
    }

    #[test]
    fn spend_uses_verified_budget_and_receipts_after_reopen() {
        let root = directory("repair-spend-projection");
        let (mut store, digest) = install_and_open(
            &root,
            &tenancy(&[("operator", "tenant"), ("bob", "foreign")]),
            retention_uniform(100_000),
        );
        let mut scope = store
            .principal(&principal("operator"))
            .expect("principal scope");
        let mut service =
            SpendReconciliation::open(&scope, profile()).expect("empty spend service");
        assert!(matches!(
            service.both_shells(&principal("operator")),
            Err(SpendError::ProtocolBudgetStateUnavailable)
        ));
        assert!(matches!(
            service.both_shells(&principal("bob")),
            Err(SpendError::WrongPrincipal)
        ));
        let replay = Replay::new(serde_json::from_str(ANSWERS).expect("recorded RPC transcript"));
        let document: Value = serde_json::from_slice(EXPORT).expect("production authority export");
        let hash = hex(document["head"]["block_hash"].as_str().expect("block hash"));
        let checkpoint =
            ServedCheckpoint::acquire_at_committed_block(&replay.endpoint, 8, Some(hash), "125")
                .expect("actual committed checkpoint consumer");
        let owner = Did::new(OWNER.as_bytes()).expect("owner DID");
        let trust = BudgetProofTrust {
            tenant: "tenant",
            principal: "operator",
            owner: &owner,
            protocol_version: 3,
            network_id: 125,
            checkpoint: &checkpoint,
            endpoint: &replay.endpoint,
            current_height: 8,
            now_ms: 1_700_000_000_000,
            maximum_age_seconds: 60,
        };
        let proof = verify_budget_proof(EXPORT, budget_id(), &trust)
            .expect("real budget authority producer/transport/verifier");
        let verifier =
            layerx_human_test_support::evidence_verifier(&SigningKey::from_bytes(&[0x84; 32]));
        let view = service
            .observe_budget_proof(&mut scope, &proof, &[], &verifier, 1_700_000_000_000)
            .expect("adopt actual network budget");
        assert_eq!(view.spent(), 23);
        assert_eq!(view.limit(), 100);
        assert_eq!(view.remaining(), 77);
        assert_eq!(view.receipt_spent(), 0);
        assert_eq!(view.verification_level(), Rung::Committed);
        assert_eq!(view.observed_head_sequence(), 8);
        assert_eq!(
            view.reconciliation(),
            SpendReconciliationStatus::ProtocolAdopted {
                direction: ReconciliationDirection::ProtocolHigher,
                difference: 23
            }
        );
        assert_eq!(view.evidence_digests(), &[proof.digest()]);
        let receipt = signed_receipt(7, [0x45; 32], 20, [4; 32], [0x55; 32]);
        let observations = [SpendReceiptEvidence {
            expected_activity_id: [0x45; 32],
            evidence: receipt.evidence,
        }];
        let reconciled = service
            .observe_budget_proof(
                &mut scope,
                &proof,
                &observations,
                &verifier,
                1_700_000_000_007,
            )
            .expect("verified spend receipt observation");
        assert_eq!(reconciled.spent(), 23);
        assert_eq!(reconciled.receipt_spent(), 20);
        assert_eq!(reconciled.receipt_count(), 1);
        assert_eq!(
            reconciled.reconciliation(),
            SpendReconciliationStatus::ProtocolAdopted {
                direction: ReconciliationDirection::ProtocolHigher,
                difference: 3
            }
        );
        assert_eq!(
            reconciled.verification_level(),
            Rung::Receipted,
            "receipt projection does not inherit a stronger rung from the budget"
        );
        let duplicate = [observations[0].clone(), observations[0].clone()];
        assert!(matches!(
            service.observe_budget_proof(
                &mut scope,
                &proof,
                &duplicate,
                &verifier,
                1_700_000_000_007
            ),
            Err(SpendError::EvidenceConflict)
        ));
        let mut substituted = observations.clone();
        substituted[0].expected_activity_id = [0xff; 32];
        assert!(matches!(
            service.observe_budget_proof(
                &mut scope,
                &proof,
                &substituted,
                &verifier,
                1_700_000_000_007
            ),
            Err(SpendError::EvidenceConflict)
        ));
        let surfaces = service
            .both_shells(&principal("operator"))
            .expect("atomic native projections");
        assert_eq!(surfaces.mobile().spend(), surfaces.desktop().spend());
        let mut changed = document.clone();
        changed["budget_state"]["remaining"] = json!("78");
        assert!(verify_budget_proof(
            &serde_json::to_vec(&changed).expect("tampered export"),
            budget_id(),
            &trust
        )
        .is_err());
        let mut wrong_profile = profile();
        wrong_profile.agent_account = [0xff; 32];
        let mut wrong = SpendReconciliation::new(wrong_profile).expect("other account profile");
        assert!(matches!(
            wrong.observe_budget_proof(&mut scope, &proof, &[], &verifier, 1_700_000_000_000),
            Err(SpendError::EvidenceConflict)
        ));
        drop(scope);
        drop(service);
        drop(store);
        let mut store = PrincipalStore::open(&root, retention_uniform(100_000), digest)
            .expect("reopen spend store");
        let scope = store
            .principal(&principal("operator"))
            .expect("reopened principal");
        let recovered =
            SpendReconciliation::open(&scope, profile()).expect("recover verified projection");
        assert_eq!(
            recovered
                .both_shells(&principal("operator"))
                .expect("recovered surfaces"),
            surfaces
        );
        drop(scope);
        let mut scope = store.principal(&principal("bob")).expect("foreign scope");
        assert!(matches!(
            SpendReconciliation::open(&scope, profile()),
            Err(SpendError::WrongPrincipal)
        ));
        let mut foreign = profile();
        foreign.principal = principal("bob");
        let mut service = SpendReconciliation::new(foreign).expect("foreign service");
        assert!(
            matches!(
                service.observe_budget_proof(&mut scope, &proof, &[], &verifier, 1_700_000_000_000),
                Err(SpendError::WrongPrincipal)
            ),
            "verified proof principal cannot be replayed into a foreign scope"
        );
        assert!(matches!(
            service.both_shells(&principal("bob")),
            Err(SpendError::ProtocolBudgetStateUnavailable)
        ));
        drop(scope);
        drop(store);
        assert!(
            replay.finish() >= 2,
            "real verifier read both getBudget and budgetRemaining"
        );
        std::fs::remove_dir_all(root).expect("remove spend state");
    }
}
