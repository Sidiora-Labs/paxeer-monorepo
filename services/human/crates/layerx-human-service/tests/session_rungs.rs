use layerx_human_test_support as support;

use std::collections::{BTreeMap, BTreeSet};
use std::fs;

use ed25519_dalek::{Signature, Signer as _, SigningKey};
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
use layerx_human_service::custody::{
    AgentContractError, AgentSessionContract, AgentSessionProvision, EnvelopeKms, KeyClass,
    KeyEntropy, KeyId, Keystore, ProtocolIdentitySnapshot, ProvisionEvidence, RevocationEvidence,
    RevocationReason, RotationEvidence, RotationJourneyState, RotationObservation, RotationSubject,
    RotationSubmission, SessionEntropySource, SessionKeyEntropy, SessionKeyError,
    SessionKeyProvisioner, SessionPolicy, SessionTarget, SuspensionEvidence,
};
use layerx_human_service::store::PrincipalId;
use layerx_intents::DisclosureCheck;
use layerx_types::ids::Did;
use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};
use layerx_types::rung::Rung;
use layerx_types::verify::VerificationLevel;
use sha2::{Digest as _, Sha256};

use support::directory;

const NETWORK_ID: u32 = 77;

fn principal() -> PrincipalId {
    PrincipalId::new("alice").unwrap_or_else(|error| panic!("principal: {error}"))
}

fn did(label: &str) -> Did {
    Did::new(format!("did:layerx:{label}").as_bytes())
        .unwrap_or_else(|error| panic!("DID: {error:?}"))
}

fn send_type() -> ActivityType {
    ActivityType::new(ModuleId::Asset, 5).unwrap_or_else(|error| panic!("send activity: {error:?}"))
}

fn rotation_type() -> ActivityType {
    ActivityType::new(ModuleId::Governance, 2)
        .unwrap_or_else(|error| panic!("rotation activity: {error:?}"))
}

fn registry() -> ModuleRegistry {
    let asset = ModuleRegistration::new(ModuleId::Asset, &[send_type()])
        .unwrap_or_else(|error| panic!("asset registry: {error:?}"));
    let governance = ModuleRegistration::new(ModuleId::Governance, &[rotation_type()])
        .unwrap_or_else(|error| panic!("governance registry: {error:?}"));
    ModuleRegistry::new(&[asset, governance])
        .unwrap_or_else(|error| panic!("module registry: {error:?}"))
}

fn policy() -> SessionPolicy {
    SessionPolicy::new(
        100,
        20,
        3,
        vec!["prepare".to_owned(), "submit".to_owned()],
        "managed-agent-v1",
    )
    .unwrap_or_else(|error| panic!("session policy: {error}"))
}

struct OperatingEntropy;

impl SessionEntropySource for OperatingEntropy {
    fn next_session_entropy(&mut self) -> Result<SessionKeyEntropy, SessionKeyError> {
        let mut seed = [0_u8; 32];
        getrandom::fill(&mut seed).map_err(|_| SessionKeyError::EntropyUnavailable)?;
        SessionKeyEntropy::new(seed)
    }
}

struct IdentityBoundary(CoreIdentity);

impl IdentityResolver for IdentityBoundary {
    fn resolve(&mut self, _did: &Did) -> Result<Option<CoreIdentity>, IdentityError> {
        Ok(Some(self.0.clone()))
    }
}

struct InstalledSession {
    issued: IssuedSessionKey,
    token: Token,
    current_revocation_sequence: u64,
}

struct CoreRotation {
    tx_hash: [u8; 32],
    current: [u8; 32],
    pending: [u8; 32],
    effective_at: u64,
    lapse_at: u64,
    effective_sequence: u64,
    committed: bool,
}

struct AgentLayer {
    observation_rung: Rung,
    announcement_rung: Rung,
    root: std::path::PathBuf,
    store: Store,
    registry: SessionRegistry,
    tenant: TenantId,
    principal: PrincipalId,
    did: Did,
    protocol_identity: [u8; 32],
    primary_public_key: [u8; 32],
    revocation_sequence: u64,
    now: u64,
    core_sequence: u64,
    installed: BTreeMap<[u8; 32], InstalledSession>,
    rotation: Option<CoreRotation>,
    events: Vec<(&'static str, [u8; 32])>,
    last_rotation_payload_hash: Option<[u8; 32]>,
}

impl AgentLayer {
    fn new(
        root: std::path::PathBuf,
        principal: PrincipalId,
        did: Did,
        primary_public_key: [u8; 32],
    ) -> Self {
        fs::create_dir_all(&root).unwrap_or_else(|error| panic!("agent root: {error}"));
        let store = Store::open(&root).unwrap_or_else(|error| panic!("agent store: {error}"));
        let protocol_identity = Sha256::digest(did.as_bytes()).into();
        Self {
            observation_rung: Rung::Receipted,
            announcement_rung: Rung::Receipted,
            root,
            store,
            registry: SessionRegistry::default(),
            tenant: TenantId::new("tenant-a").unwrap_or_else(|error| panic!("tenant: {error}")),
            principal,
            did,
            protocol_identity,
            primary_public_key,
            revocation_sequence: 1,
            now: 1_000,
            core_sequence: 10,
            installed: BTreeMap::new(),
            rotation: None,
            events: Vec::new(),
            last_rotation_payload_hash: None,
        }
    }

    fn advance(&mut self, now: u64, core_sequence: u64) {
        assert!(now >= self.now, "protocol time must not move backwards");
        assert!(
            core_sequence >= self.core_sequence,
            "core sequence must not move backwards"
        );
        self.now = now;
        self.core_sequence = core_sequence;
    }

    fn daemon_open(&self, grant_id: [u8; 32]) -> bool {
        self.registry
            .get(&self.tenant, SessionId(grant_id))
            .is_some_and(|record| record.open)
    }

    fn token_authorizes(&self, grant_id: [u8; 32], scope: &str) -> bool {
        self.installed.get(&grant_id).is_some_and(|installed| {
            installed
                .token
                .authorize(
                    &self.registry,
                    &self.tenant,
                    &self.did,
                    scope,
                    self.core_sequence,
                )
                .is_ok()
        })
    }

    fn commit_rotation(&mut self) -> Result<(), AgentContractError> {
        let rotation = self
            .rotation
            .as_mut()
            .ok_or(AgentContractError::Refused("rotation missing"))?;
        if self.now < rotation.effective_at {
            return Err(AgentContractError::Refused("challenge delay open"));
        }
        if self.now > rotation.lapse_at {
            return Err(AgentContractError::Refused("rotation lapsed"));
        }
        if rotation.committed {
            return Err(AgentContractError::Refused("rotation already committed"));
        }
        rotation.committed = true;
        self.primary_public_key = rotation.pending;
        self.revocation_sequence = self.revocation_sequence.saturating_add(1);
        let event = RevocationEvent {
            did: self.did.clone(),
            authority: None,
            reason: InvalidationReason::PrimaryKeyRotated,
            observed_sequence: self.core_sequence,
        };
        invalidate_on_revocation(&mut self.store, &mut self.registry, &mut [], &event)
            .map_err(|_| AgentContractError::Refused("agent invalidation failed"))?;
        for installed in self.installed.values_mut() {
            installed.current_revocation_sequence =
                installed.current_revocation_sequence.saturating_add(1);
        }
        Ok(())
    }
}

impl Drop for AgentLayer {
    fn drop(&mut self) {
        let _ = fs::remove_dir_all(&self.root);
    }
}

impl AgentSessionContract for AgentLayer {
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
        if scopes.is_empty() || policy_version.is_empty() {
            return Err(AgentContractError::Refused("daemon bounds missing"));
        }
        let grant_id = issued.grant_id;
        let session_public_key = issued.session_public_key;
        let seed = secret.into_seed();
        let _signer = ProvisionedSessionKey::new(seed, issued.clone())
            .map_err(|_| AgentContractError::Refused("session secret does not match grant"))?;
        let authorities = self
            .installed
            .keys()
            .copied()
            .map(ProtocolAuthority::SessionKey)
            .chain(std::iter::once(ProtocolAuthority::SessionKey(grant_id)))
            .collect();
        let mut boundary = IdentityBoundary(CoreIdentity {
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
            &mut boundary,
        )
        .map_err(|_| AgentContractError::Refused("identity registration failed"))?;
        let token = open(
            &mut self.store,
            &mut self.registry,
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
                current_revocation_sequence: issued.revocation_sequence,
                issued,
                token,
            },
        );
        self.events.push(("provision", grant_id));
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
        if target.grant_id != target.daemon_session_id {
            return Err(AgentContractError::Refused("session target mismatch"));
        }
        close(
            &mut self.store,
            &mut self.registry,
            &self.tenant,
            SessionId(target.daemon_session_id),
        )
        .map_err(|_| AgentContractError::Refused("daemon session close failed"))?;
        self.now = self.now.max(requested_at);
        self.events.push(("suspend", target.grant_id));
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
        let installed = self
            .installed
            .get_mut(&target.grant_id)
            .ok_or(AgentContractError::Refused("session signer missing"))?;
        installed.current_revocation_sequence =
            installed.current_revocation_sequence.saturating_add(1);
        let event = RevocationEvent {
            did: self.did.clone(),
            authority: Some(ProtocolAuthority::SessionKey(target.grant_id)),
            reason: InvalidationReason::SessionKeyRevoked,
            observed_sequence: self.core_sequence.saturating_add(1),
        };
        invalidate_on_revocation(&mut self.store, &mut self.registry, &mut [], &event)
            .map_err(|_| AgentContractError::Refused("protocol invalidation failed"))?;
        self.core_sequence = self.core_sequence.saturating_add(1);
        self.now = self.now.max(requested_at).saturating_add(1);
        self.events.push(("revoke", target.grant_id));
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
        submission: RotationSubmission,
    ) -> Result<RotationEvidence, AgentContractError> {
        self.require_scope(&submission.principal, &submission.did)?;
        if self.rotation.is_some() {
            return Err(AgentContractError::Refused("competing rotation"));
        }
        if DisclosureCheck::verify(&submission.intent, &submission.compiled)
            != Ok(submission.disclosure.clone())
            || submission.compiled.activity_type() != rotation_type()
            || submission.current_public_key != self.primary_public_key
        {
            return Err(AgentContractError::Refused("rotation intent mismatch"));
        }
        self.last_rotation_payload_hash = Some(submission.compiled.payload_hash());
        let signing_key = SigningKey::from_bytes(&[0x91; 32]);
        let message = submission.compiled.payload_hash();
        let signature = signing_key.sign(&message).to_bytes();
        let transaction = [message.as_slice(), signature.as_slice()].concat();
        let tx_hash = Sha256::digest(&transaction).into();
        fs::write(self.root.join("rotation-transaction"), transaction)
            .map_err(|_| AgentContractError::Unavailable)?;
        self.rotation = Some(CoreRotation {
            tx_hash,
            current: submission.current_public_key,
            pending: submission.pending_public_key,
            effective_at: submission.effective_at,
            lapse_at: submission.lapse_at,
            effective_sequence: submission.effective_sequence,
            committed: false,
        });
        self.core_sequence = self.core_sequence.saturating_add(1);
        let mut evidence = RotationEvidence {
            tx_hash,
            payload_hash: submission.compiled.payload_hash(),
            pending_public_key: submission.pending_public_key,
            effective_at: submission.effective_at,
            lapse_at: submission.lapse_at,
            effective_sequence: submission.effective_sequence,
            observed_sequence: self.core_sequence,
            observed_at: self.now,
            rung: self.announcement_rung,
            receipt_digest: [0; 32],
        };
        evidence.receipt_digest = evidence.expected_digest();
        Ok(evidence)
    }

    fn rotation_observation(
        &mut self,
        principal: &PrincipalId,
        did: &Did,
    ) -> Result<RotationObservation, AgentContractError> {
        self.require_scope(principal, did)?;
        let rotation = self
            .rotation
            .as_ref()
            .ok_or(AgentContractError::Refused("rotation missing"))?;
        let transaction = fs::read(self.root.join("rotation-transaction"))
            .map_err(|_| AgentContractError::Unavailable)?;
        if transaction.len() != 96 {
            return Err(AgentContractError::Refused("invalid signed rotation"));
        }
        let signature = Signature::from_slice(&transaction[32..])
            .map_err(|_| AgentContractError::Refused("invalid rotation signature"))?;
        SigningKey::from_bytes(&[0x91; 32])
            .verifying_key()
            .verify_strict(&transaction[..32], &signature)
            .map_err(|_| AgentContractError::Refused("rotation signature refused"))?;
        Ok(RotationObservation {
            tx_hash: Sha256::digest(&transaction).into(),
            primary_public_key: if rotation.committed {
                rotation.pending
            } else {
                rotation.current
            },
            pending_public_key: (!rotation.committed).then_some(rotation.pending),
            superseded_public_key: rotation.committed.then_some(rotation.current),
            effective_at: rotation.effective_at,
            lapse_at: rotation.lapse_at,
            effective_sequence: rotation.effective_sequence,
            observed_at: self.now,
            observed_sequence: self.core_sequence,
            rung: self.observation_rung,
        })
    }
}

impl AgentLayer {
    fn require_scope(&self, principal: &PrincipalId, did: &Did) -> Result<(), AgentContractError> {
        if principal == &self.principal && did == &self.did {
            Ok(())
        } else {
            Err(AgentContractError::Refused("wrong principal"))
        }
    }
}

fn keystore(
    root: &std::path::Path,
    principal: &PrincipalId,
    class: KeyClass,
    current_seed: [u8; 32],
    pending_seed: [u8; 32],
) -> (Keystore, KeyId, KeyId, [u8; 32], [u8; 32]) {
    fs::create_dir_all(root).unwrap_or_else(|error| panic!("custody root: {error}"));
    let secret_path = root.join("kms-root");
    fs::write(&secret_path, [0xa5; 64]).unwrap_or_else(|error| panic!("KMS root: {error}"));
    let provider = EnvelopeKms::new("file-kms://rotation", secret_path)
        .unwrap_or_else(|error| panic!("KMS provider: {error}"));
    let keystore = Keystore::open_development(root.join("sealed"), NETWORK_ID, provider)
        .unwrap_or_else(|error| panic!("keystore: {error}"));
    let current =
        KeyId::new("current-primary").unwrap_or_else(|error| panic!("current key id: {error}"));
    let pending =
        KeyId::new("pending-primary").unwrap_or_else(|error| panic!("pending key id: {error}"));
    let current_public = keystore
        .generate(
            principal,
            &current,
            class,
            KeyEntropy::new(current_seed, [1; 16], [2; 24])
                .unwrap_or_else(|error| panic!("current entropy: {error}")),
        )
        .unwrap_or_else(|error| panic!("current key: {error}"));
    let pending_public = keystore
        .generate(
            principal,
            &pending,
            class,
            KeyEntropy::new(pending_seed, [3; 16], [4; 24])
                .unwrap_or_else(|error| panic!("pending entropy: {error}")),
        )
        .unwrap_or_else(|error| panic!("pending key: {error}"));
    (keystore, current, pending, current_public, pending_public)
}

fn rotation_fixture(
    label: &str,
) -> (
    SessionKeyProvisioner<AgentLayer, OperatingEntropy>,
    Keystore,
    KeyId,
    KeyId,
    PrincipalId,
    Did,
) {
    let root = directory(label);
    let principal = principal();
    let did = did(label);
    let (keys, current, pending, current_public, _) = keystore(
        &root.join("custody"),
        &principal,
        KeyClass::AgentPrimary,
        [0x21; 32],
        [0x22; 32],
    );
    let layer = AgentLayer::new(
        root.join("agent"),
        principal.clone(),
        did.clone(),
        current_public,
    );
    let mut provisioner = SessionKeyProvisioner::new(layer, OperatingEntropy, policy(), registry());
    provisioner
        .provision(&principal, &did, vec![send_type()])
        .unwrap_or_else(|error| panic!("initial session: {error}"));
    (provisioner, keys, current, pending, principal, did)
}

#[test]
fn session_rotation_reconciles_only_at_committed() {
    let (mut provisioner, keys, current, pending, principal, did) =
        rotation_fixture("rotation-rungs");
    let original = provisioner.session(&principal, &did).unwrap().clone();
    let announced = provisioner
        .announce_rotation(
            &keys,
            &principal,
            RotationSubject::Agent,
            &did,
            &current,
            &pending,
            60,
            50,
        )
        .unwrap_or_else(|error| panic!("announce: {error}"));
    assert_eq!(announced.state, RotationJourneyState::ChallengeOpen);
    assert_eq!(
        announced.tx_hash,
        provisioner.contract().rotation.as_ref().unwrap().tx_hash
    );
    let (receipted, replacement) = provisioner.reconcile_rotation(&principal, &did).unwrap();
    assert_eq!(receipted, announced);
    assert!(replacement.is_none());
    assert!(provisioner.contract().daemon_open(original.grant_id));
    assert_eq!(provisioner.session(&principal, &did), Some(&original));

    provisioner.contract_mut().advance(1_060, 50);
    let (receipted, replacement) = provisioner.reconcile_rotation(&principal, &did).unwrap();
    assert_eq!(receipted, announced);
    assert!(replacement.is_none());
    assert_eq!(provisioner.contract().installed.len(), 1);

    provisioner.contract_mut().commit_rotation().unwrap();
    assert!(!provisioner.contract().daemon_open(original.grant_id));
    provisioner.contract_mut().observation_rung = Rung::Submitted;
    assert!(matches!(
        provisioner.reconcile_rotation(&principal, &did),
        Err(SessionKeyError::InvalidEvidence)
    ));
    provisioner.contract_mut().observation_rung = Rung::Receipted;

    let (receipted, replacement) = provisioner.reconcile_rotation(&principal, &did).unwrap();
    assert_eq!(receipted, announced);
    assert!(replacement.is_none());
    assert_eq!(provisioner.session(&principal, &did), Some(&original));
    assert_eq!(provisioner.contract().installed.len(), 1);

    provisioner.contract_mut().observation_rung = Rung::Committed;
    let (committed, replacement) = provisioner.reconcile_rotation(&principal, &did).unwrap();
    assert_eq!(
        committed.state,
        RotationJourneyState::Effective {
            superseded_key_usable_before_sequence: 50,
            observed_sequence: 50,
        }
    );
    let replacement = replacement.expect("committed rotation restores agent authority");
    assert_ne!(replacement.grant_id, original.grant_id);
    assert!(provisioner.contract().daemon_open(replacement.grant_id));
    assert!(provisioner
        .contract()
        .token_authorizes(replacement.grant_id, "prepare"));
    let (again, extra) = provisioner.reconcile_rotation(&principal, &did).unwrap();
    assert_eq!(again, committed);
    assert!(extra.is_none());
    assert_eq!(provisioner.contract().installed.len(), 2);
}

#[test]
fn rotation_observation_for_another_transaction_is_refused() {
    let (mut provisioner, keys, current, pending, principal, did) =
        rotation_fixture("rotation-transaction");
    let original = provisioner.session(&principal, &did).unwrap().clone();
    let announced = provisioner
        .announce_rotation(
            &keys,
            &principal,
            RotationSubject::Agent,
            &did,
            &current,
            &pending,
            60,
            50,
        )
        .unwrap_or_else(|error| panic!("announce: {error}"));
    provisioner.contract_mut().advance(1_060, 50);
    provisioner.contract_mut().commit_rotation().unwrap();
    let path = provisioner.contract().root.join("rotation-transaction");
    let matching_transaction = fs::read(&path).unwrap();
    let message = Sha256::digest(b"another signed rotation transaction");
    let signature = SigningKey::from_bytes(&[0x91; 32])
        .sign(&message)
        .to_bytes();
    fs::write(&path, [message.as_slice(), signature.as_slice()].concat()).unwrap();
    for rung in [Rung::Receipted, Rung::Committed] {
        provisioner.contract_mut().observation_rung = rung;
        assert!(matches!(
            provisioner.reconcile_rotation(&principal, &did),
            Err(SessionKeyError::InvalidEvidence)
        ));
        assert_eq!(provisioner.session(&principal, &did), Some(&original));
        assert_eq!(provisioner.contract().installed.len(), 1);
    }
    fs::write(&path, matching_transaction).unwrap();
    provisioner.contract_mut().observation_rung = Rung::Receipted;
    let (still_pending, replacement) = provisioner.reconcile_rotation(&principal, &did).unwrap();
    assert_eq!(still_pending, announced);
    assert!(replacement.is_none());
    provisioner.contract_mut().observation_rung = Rung::Committed;
    let (committed, replacement) = provisioner.reconcile_rotation(&principal, &did).unwrap();
    assert!(matches!(
        committed.state,
        RotationJourneyState::Effective { .. }
    ));
    assert!(replacement.is_some());
}
