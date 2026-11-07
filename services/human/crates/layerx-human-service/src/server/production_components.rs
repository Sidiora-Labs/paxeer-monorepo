//! Production composition root for the privileged human component process.

use layerx_types::clock::{Clock, Deadline};
use std::env;
use std::fs;
use std::net::SocketAddr;
use std::path::PathBuf;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::{Arc, Mutex};
use std::time::Duration;

use crate::frame::Limits;
use crate::journeys::AgentBoundary as _;
use base64::{engine::general_purpose::URL_SAFE_NO_PAD, Engine as _};
use layerx_agent_api::identity::{AgentDid, AuthorityRef};
use layerx_crypto::disclosure::call::{bind_call, ledger_selector};
use layerx_crypto::evm_transaction::{
    assemble, decode_signed, recover_sender, transaction_hash, Eip1559Call,
};
use layerx_crypto::local::LocalSigner;
use layerx_crypto::payments::{ledger_payment_call, CallOptions};
use layerx_crypto::session::{issue_session_key, SessionKeyRequest};
use layerx_crypto::signer::sign_disclosed_call;
use layerx_crypto::signer::Signer as _;
use layerx_intents::{
    BudgetCreate, BudgetDefund, Intent, IntentKind, SessionGrant as ProtocolSessionGrant,
    SessionRevoke,
};
use layerx_types::account::AccountId;
use layerx_types::amount::Amount as ProtocolAmount;
use layerx_types::ids::Did;
use layerx_types::ids::{AssetId, IdempotencyKey};
use layerx_types::intent::EvmAddress;
use layerx_types::intent::{AuthorityGrantId, PublicKey, SessionRevocationReason};
use layerx_types::intent::{
    BudgetId, PeriodLength, PurposeHash, RolloverPolicy, Sequence as ProtocolSequence,
    TimestampSeconds,
};
use layerx_types::payload::{ModuleId, ModuleRegistry};
use layerx_types::rung::Rung;
use rustls::pki_types::{CertificateDer, PrivateKeyDer};
use rustls::RootCertStore;
use serde_json::json;
use sha2::{Digest as _, Sha256};

use super::agent_creation::ProductionAgentCreation;
use crate::activity::{
    verification_status, AppliedFilters, EvidenceExport, Feed, FeedCursor, FilterDraft,
    PageRequest, ReceiptExport,
};
use crate::agents::{
    CreateAgentRequest, CreationContext, CreationJourney, PurposePresetCatalog,
    ScopedAgentCreationContract, SessionProvision,
};
use crate::approvals::{
    AgentApprovalRecord, AgentApprovalState, AgentDecisionStatus, ApprovalBoundary,
};
use crate::auth::{AccountIdentity, AuthConfig, Passkeys, RateLimit};
use crate::custody::{
    CustodyError, CustodySigner, KeyClass, KeyId, Keystore, KmsError, PrincipalKeyBinding,
    ProviderDeployment, ProviderKeyDescription, ProviderKeyReference, ProviderSignRequest,
    RemoteKmsProvider, RotationState, SigningLimits,
};
use crate::frame::{Limits as KmsLimits, MutualTlsConfig};
use crate::notify::{
    ActivityEntryId, Channel, DeepLinks, Dispatcher, NotificationId, NotificationSummary,
    Preferences,
};
use crate::onboarding::OnboardingJourney;
use crate::security::{
    AuthenticatorMethod, AuthenticatorProvider, AuthenticatorStatus, RecoveryEvidenceProvider,
};
use crate::store::{
    PrincipalStore, RetentionPeriod, RetentionPolicy, RowKey, Table, TenancyDigest,
};
use crate::support::{CreateConversation, Shell, SupportService, Topic};
use crate::trace::TraceId;

use super::agent_runtime::AgentRuntime;
use super::backend::{
    ApiFailure, BackendResponse, BearerCredentials, ComponentState, HumanApiComponents,
    PrincipalContext, Readiness, ScopedRequest, SessionCredentials, SessionSecrets,
};
use super::identity_dispatch::{self, IdentityProviderConfig, RemoteIdentityProvider};
use super::production_auth::{
    authorize_bearer_execution, authorize_execution, authorize_refresh_execution, consume_context,
    AuthDiscoveryIndex, AuthorizationDisclosure, IndexAuthenticationKey, RemoteSecurityProvider,
    SecurityProviderConfig,
};
use super::schema::{ApiSchema, Operation};

pub(crate) fn verify_owner_activity(bytes: &[u8]) -> Result<(Eip1559Call, [u8; 32]), ApiFailure> {
    let (call, signature) = decode_signed(bytes).map_err(|_| ApiFailure::upstream_degraded())?;
    if call.chain_id != 125 {
        return Err(ApiFailure::upstream_degraded());
    }
    let sender = recover_sender(&call, &signature).map_err(|_| ApiFailure::upstream_degraded())?;
    let mut authority = [0; 32];
    authority[12..].copy_from_slice(&sender);
    Ok((call, authority))
}

pub const PRODUCTION_OPERATIONS: &[&str] = &[
    "account.balance",
    "account.create",
    "activity.entry",
    "activity.export.evidence",
    "activity.export.statement",
    "activity.query",
    "agent.archive",
    "agent.create",
    "agent.get",
    "agent.limit",
    "agent.list",
    "agent.pause",
    "agent.reclaim",
    "agent.recover",
    "agent.resume",
    "agent.rotate",
    "agent.rotation.start",
    "agent.rotation.disclosure",
    "approval.approve",
    "approval.get",
    "approval.list",
    "approval.program.disclosure",
    "approval.program.list",
    "approval.program.get",
    "approval.program.material",
    "approval.program.budget",
    "approval.program.approve",
    "approval.program.reject",
    "approval.reject",
    "authenticator.backup.rotate",
    "authenticator.disable",
    "authenticator.setup.begin",
    "authenticator.setup.finish",
    "authenticator.status",
    "evidence.get",
    "home.summary",
    "intent.plan",
    "intent.submit",
    "native.send.access.begin",
    "native.send.access.confirm",
    "journey.get",
    "journey.list",
    "journey.page",
    "move.commit",
    "move.quote",
    "notification.list",
    "notification.preferences.get",
    "notification.preferences.set",
    "notification.read",
    "onboarding.resume",
    "onboarding.status",
    "passkey.assert.begin",
    "passkey.assert.finish",
    "passkey.register.begin",
    "passkey.register.finish",
    "profile.get",
    "profile.update",
    "security.action",
    "security.key-export.begin",
    "security.key-export.finish",
    "security.passkey.list",
    "security.passkey.register.begin",
    "security.passkey.register.finish",
    "security.passkey.revoke",
    "security.recovery.reveal",
    "security.session.revoke",
    "security.session.revoke-all",
    "session.list",
    "session.fee-policy",
    "session.open",
    "session.refresh",
    "session.revoke",
    "session.revoke-all",
    "stepup.begin",
    "stepup.finish",
    "stream.next",
    "stream.open",
    "support.create",
    "support.feedback",
    "support.list",
    "support.read",
    "support.reply",
    "support.status",
    "version",
];
fn managed_protocol_identity(value: &str) -> Result<[u8; 32], ApiFailure> {
    let encoded = value
        .strip_prefix("agt_")
        .ok_or_else(ApiFailure::upstream_degraded)?;
    if encoded.len() != 64 {
        return Err(ApiFailure::upstream_degraded());
    }
    let mut out = [0; 32];
    for (index, byte) in out.iter_mut().enumerate() {
        *byte = u8::from_str_radix(&encoded[index * 2..index * 2 + 2], 16)
            .map_err(|_| ApiFailure::upstream_degraded())?;
    }
    Ok(out)
}
use super::component::ComponentMaintenance;

pub struct AccountBalance {
    balance: super::agent_runtime::VerifiedBalance,
    rung: Option<Rung>,
}

impl AccountBalance {
    pub fn from_agent(balance: super::agent_runtime::VerifiedBalance) -> Result<Self, ApiFailure> {
        let rung = balance.verification;
        Ok(Self { balance, rung })
    }

    pub fn evidence_material(&self) -> Result<Vec<u8>, ApiFailure> {
        let balance = &self.balance;
        let mut material = Vec::with_capacity(
            5 + 1 + balance.canonical_bytes.len() + balance.proof_material.len(),
        );
        material.extend_from_slice(b"LXHB2");
        match self.rung {
            None => material.push(0),
            Some(rung) => {
                material.push(1);
                material.push(rung.wire_rank());
            }
        }
        material.extend_from_slice(
            &u32::try_from(balance.observed_at.len())
                .map_err(|_| ApiFailure::upstream_degraded())?
                .to_be_bytes(),
        );
        material.extend_from_slice(balance.observed_at.as_bytes());
        material.extend_from_slice(&balance.age_seconds.to_be_bytes());
        material.extend_from_slice(
            &u32::try_from(balance.canonical_bytes.len())
                .map_err(|_| ApiFailure::upstream_degraded())?
                .to_be_bytes(),
        );
        material.extend_from_slice(&balance.canonical_bytes);
        material.extend_from_slice(&balance.proof_material);
        Ok(material)
    }

    pub fn project(&self, evidence_digest: [u8; 32], freshness_seconds: u64) -> serde_json::Value {
        let balance = &self.balance;
        let verification = self
            .rung
            .map(super::HumanComponentServer::verification_level_name)
            .unwrap_or("unverified");
        let mut result = json!({
            "account_id": format!("act_{}", hex_bytes(&balance.account)),
            "money": {"amount": balance.amount.to_string(), "currency": balance.currency},
            "verification": verification,
            "freshness": {"observed_at": balance.observed_at, "age_seconds": balance.age_seconds,
                "source_head": balance.observed_head_sequence.to_string(),
                "within_bound": balance.observed_head_sequence == balance.global_sequence && balance.age_seconds <= freshness_seconds,
                },
            "evidence": [{"evidence_id": format!("evd_{}", hex_bytes(&evidence_digest)),
                "class": match self.rung { Some(Rung::Committed) => "checkpoint-proof", Some(Rung::Receipted) => "layerx-receipt", _ => "submission-record" },
                "verification": verification}]
        });
        if let Some(rung) = self.rung {
            result["rung"] = json!(rung.name());
        }
        if balance.observed_checkpoint != [0; 32] {
            result["freshness"]["checkpoint"] = json!(hex_bytes(&balance.observed_checkpoint));
        }
        result
    }

    pub fn read(
        agent: &mut AgentRuntime,
        scope: &mut crate::store::PrincipalScope<'_>,
        now: u64,
        freshness_seconds: u64,
    ) -> Result<serde_json::Value, ApiFailure> {
        let balance = Self::from_agent(agent.balance().map_err(agent_failure)?)?;
        let material = balance.evidence_material()?;
        let digest: [u8; 32] = Sha256::digest(&material).into();
        scope
            .put(
                Table::Cache,
                RowKey::new(format!("state-proof-{}", hex_bytes(&digest)))
                    .map_err(|_| ApiFailure::upstream_degraded())?,
                now,
                material,
            )
            .map_err(|_| ApiFailure::unavailable())?;
        Ok(balance.project(digest, freshness_seconds))
    }

    pub fn read_home_summary(
        agent: &mut AgentRuntime,
        scope: &mut crate::store::PrincipalScope<'_>,
        now: u64,
        freshness_seconds: u64,
    ) -> Result<serde_json::Value, ApiFailure> {
        Self::read_native_home_summary(agent, scope, now, freshness_seconds)
    }

    fn read_native_home_summary(
        agent: &mut AgentRuntime,
        scope: &mut crate::store::PrincipalScope<'_>,
        now: u64,
        freshness_seconds: u64,
    ) -> Result<serde_json::Value, ApiFailure> {
        let balance_json = AccountBalance::read(agent, scope, now, freshness_seconds)?;
        let managed_page = agent.agent_list(None, 100).map_err(agent_failure)?;
        let agents = managed_page
            .agents
            .into_iter()
            .map(|value| managed_agent_json(agent, value))
            .collect::<Result<Vec<_>, _>>()?;
        let sequence = agent.head().map_err(agent_failure)?.chain_sequence;
        let (approvals, _) = approval_inventory(agent, scope, sequence, now)?;
        let (program_approvals, _) = program_approval_inventory(agent, scope, None, sequence, now)?;
        let filters = Feed::apply_filters(FilterDraft::new())
            .map_err(|error| activity_feed_failure(&error))?;
        let recent_page = Feed::new(freshness_seconds)
            .map_err(|_| ApiFailure::unavailable())?
            .page(scope, PageRequest::new(20, filters), now, sequence)
            .map_err(|error| activity_feed_failure(&error))?;
        let recent = recent_page
            .entries()
            .iter()
            .map(|entry| super::production_reads::activity_entry_json(scope, entry.entry_id()))
            .collect::<Result<Vec<_>, _>>()?;
        Ok(Self::home_summary(
            balance_json,
            agents,
            approvals,
            program_approvals,
            recent,
        ))
    }

    pub fn home_summary(
        balance: serde_json::Value,
        agents: Vec<serde_json::Value>,
        approvals: Vec<serde_json::Value>,
        program_approvals: Vec<serde_json::Value>,
        recent_activity: Vec<serde_json::Value>,
    ) -> serde_json::Value {
        let mut result = json!({"balance": balance, "agents": agents, "approvals": approvals, "program_approvals": program_approvals, "recent_activity": recent_activity});
        if let Some(rung) = result["balance"].get("rung").cloned() {
            result["rung"] = rung;
        }
        result
    }
}

fn balance_rung(rank: u8) -> Result<Rung, ApiFailure> {
    match rank {
        0 => Ok(Rung::Submitted),
        1 => Ok(Rung::Receipted),
        2 => Ok(Rung::Committed),
        _ => Err(ApiFailure::upstream_degraded()),
    }
}

/// Mandatory, bounded production settings. There are deliberately no defaults:
/// an omitted trust root, retention limit or transport bound refuses startup.
pub struct ProductionComponentsConfig {
    store_root: PathBuf,
    custody_root: PathBuf,
    auth_index_root: PathBuf,
    tenancy_digest: [u8; 32],
    auth_index_key: [u8; 32],
    stream_cursor_key: [u8; 32],
    auth: AuthConfig,
    retention: RetentionPolicy,
    capability_ttl_seconds: u64,
    agent_socket: PathBuf,
    agent_limits: Limits,
    security: SecurityProviderConfig,
    identity: IdentityProviderConfig,
    identity_binding: layerx_identity_binding::Config,
    kms: Option<RemoteKmsConfig>,
    attestor: Option<AttestorCustodyConfig>,
    network_id: u32,
    protocol_version: u16,
    signing_limits: SigningLimits,
    agent_actor: AgentDid,
    agent_authority: AuthorityRef,
    agent_timestamp_span_seconds: u64,
    agent_fee_limit: u128,
    onboarding_sponsor_principal: crate::store::PrincipalId,
    onboarding_initial_funding: u128,
    evm_gas_limit: u64,
    evm_max_fee_per_gas: u64,
    evm_max_priority_fee_per_gas: u64,
    agent_purpose_catalog: PathBuf,
    agent_owner_account: String,
    agent_recovery_root: [u8; 32],
    agent_recovery_threshold: u16,
    activity_freshness_seconds: u64,
    activity_export_maximum_bytes: usize,
    continuation_unknown_deadline_seconds: u64,
}

fn bounded_limits(prefix: &str) -> Result<Limits, String> {
    Ok(Limits {
        maximum_frame_bytes: number(&format!("{prefix}_MAX_FRAME_BYTES"))?,
        maximum_connections: number(&format!("{prefix}_MAX_CONNECTIONS"))?,
        maximum_streams: number(&format!("{prefix}_MAX_STREAMS"))?,
        maximum_queued_bytes: number(&format!("{prefix}_MAX_QUEUED_BYTES"))?,
        deadline: Duration::from_secs(number(&format!("{prefix}_DEADLINE_SECONDS"))?),
    })
}

impl ProductionComponentsConfig {
    /// # Errors
    /// Refuses incomplete, invalid, or unsupported production dependency configuration.
    pub fn from_environment() -> Result<Self, String> {
        let protocol_value = env::var("LAYERX_HUMAN_PROTOCOL_VERSION")
            .map(Some)
            .or_else(|error| match error {
                env::VarError::NotPresent => Ok(None),
                env::VarError::NotUnicode(_) => {
                    Err("LAYERX_HUMAN_PROTOCOL_VERSION is invalid".to_owned())
                }
            })?;
        let protocol_version = selected_custody_protocol(protocol_value.as_deref(), false)?;
        let attestor = AttestorCustodyConfig::from_environment()?;
        let attestor_configured = attestor.is_some();
        Ok(Self {
            store_root: absolute("LAYERX_HUMAN_STORE_ROOT")?,
            custody_root: absolute("LAYERX_HUMAN_CUSTODY_ROOT")?,
            auth_index_root: absolute("LAYERX_HUMAN_AUTH_INDEX_ROOT")?,
            tenancy_digest: secret32("LAYERX_HUMAN_TENANCY_DIGEST")?,
            auth_index_key: secret32("LAYERX_HUMAN_AUTH_INDEX_KEY")?,
            stream_cursor_key: secret32("LAYERX_HUMAN_STREAM_CURSOR_KEY")?,
            auth: production_passkey_config()?,
            retention: production_retention_config()?,
            capability_ttl_seconds: number("LAYERX_HUMAN_CAPABILITY_TTL_SECONDS")?,
            agent_socket: absolute("LAYERX_HUMAN_AGENT_SOCKET")?,
            agent_limits: bounded_limits("LAYERX_HUMAN_AGENT")?,
            security: SecurityProviderConfig {
                socket: absolute("LAYERX_HUMAN_SECURITY_SOCKET")?,
                deadline: Duration::from_secs(number("LAYERX_HUMAN_SECURITY_DEADLINE_SECONDS")?),
                maximum_frame_bytes: number("LAYERX_HUMAN_SECURITY_MAX_FRAME_BYTES")?,
            },
            identity_binding: principal_binding_configuration()?,
            identity: IdentityProviderConfig {
                socket: absolute("LAYERX_HUMAN_IDENTITY_SOCKET")?,
                deadline: Duration::from_secs(number("LAYERX_HUMAN_IDENTITY_DEADLINE_SECONDS")?),
                maximum_frame_bytes: number("LAYERX_HUMAN_IDENTITY_MAX_FRAME_BYTES")?,
                peer_uid: number("LAYERX_HUMAN_IDENTITY_PEER_UID")?,
                peer_gid: number("LAYERX_HUMAN_IDENTITY_PEER_GID")?,
            },
            kms: if attestor_configured {
                None
            } else {
                Some(RemoteKmsConfig::from_environment()?)
            },
            attestor,
            network_id: number("LAYERX_HUMAN_NETWORK_ID")?,
            protocol_version,
            signing_limits: SigningLimits::new(
                number("LAYERX_HUMAN_SIGNING_RATE_MAXIMUM")?,
                number("LAYERX_HUMAN_SIGNING_RATE_WINDOW_SECONDS")?,
            )
            .map_err(|_| "invalid custody signing limits".to_owned())?,
            agent_actor: AgentDid::new(required("LAYERX_HUMAN_AGENT_ACTOR")?)
                .map_err(|_| "LAYERX_HUMAN_AGENT_ACTOR is invalid".to_owned())?,
            agent_authority: AuthorityRef::new(required("LAYERX_HUMAN_AGENT_AUTHORITY")?)
                .map_err(|_| "LAYERX_HUMAN_AGENT_AUTHORITY is invalid".to_owned())?,
            agent_timestamp_span_seconds: number("LAYERX_HUMAN_AGENT_TIMESTAMP_SPAN_SECONDS")?,
            agent_fee_limit: number("LAYERX_HUMAN_AGENT_FEE_LIMIT")?,
            onboarding_sponsor_principal: crate::store::PrincipalId::new(required(
                "LAYERX_HUMAN_ONBOARDING_SPONSOR_PRINCIPAL",
            )?)
            .map_err(|_| "invalid onboarding sponsor principal".to_owned())?,
            onboarding_initial_funding: number("LAYERX_HUMAN_ONBOARDING_INITIAL_FUNDING")?,
            evm_gas_limit: number("LAYERX_HUMAN_EVM_GAS_LIMIT")?,
            evm_max_fee_per_gas: number("LAYERX_HUMAN_EVM_MAX_FEE_PER_GAS")?,
            evm_max_priority_fee_per_gas: number("LAYERX_HUMAN_EVM_MAX_PRIORITY_FEE_PER_GAS")?,
            agent_purpose_catalog: absolute("LAYERX_HUMAN_AGENT_PURPOSE_CATALOG")?,
            agent_owner_account: required("LAYERX_HUMAN_AGENT_OWNER_ACCOUNT")?,
            agent_recovery_root: secret32("LAYERX_HUMAN_AGENT_RECOVERY_ROOT")?,
            agent_recovery_threshold: number("LAYERX_HUMAN_AGENT_RECOVERY_THRESHOLD")?,
            activity_freshness_seconds: number("LAYERX_HUMAN_ACTIVITY_FRESHNESS_SECONDS")?,
            activity_export_maximum_bytes: number("LAYERX_HUMAN_ACTIVITY_EXPORT_MAXIMUM_BYTES")?,
            continuation_unknown_deadline_seconds: number(
                "LAYERX_HUMAN_CONTINUATION_UNKNOWN_DEADLINE_SECONDS",
            )?,
        })
    }

    /// The attestor custody backend when `LAYERX_HUMAN_ATTESTOR_NODES` selected it.
    #[must_use]
    pub const fn attestor_custody(&self) -> Option<&AttestorCustodyConfig> {
        self.attestor.as_ref()
    }

    /// The custody protocol version the loader settled on.
    #[must_use]
    pub const fn protocol_version(&self) -> u16 {
        self.protocol_version
    }
}

/// The remote KMS group, required only while no attestor custody is configured.
struct RemoteKmsConfig {
    provider_reference: String,
    endpoint: SocketAddr,
    server_name: String,
    root_certificate: PathBuf,
    client_certificate: PathBuf,
    client_private_key: PathBuf,
    limits: KmsLimits,
}

impl RemoteKmsConfig {
    fn from_environment() -> Result<Self, String> {
        Ok(Self {
            provider_reference: required("LAYERX_HUMAN_KMS_PROVIDER_REFERENCE")?,
            endpoint: required("LAYERX_HUMAN_KMS_ENDPOINT")?
                .parse()
                .map_err(|_| "LAYERX_HUMAN_KMS_ENDPOINT is invalid".to_owned())?,
            server_name: required("LAYERX_HUMAN_KMS_SERVER_NAME")?,
            root_certificate: absolute("LAYERX_HUMAN_KMS_ROOT_CERTIFICATE_DER")?,
            client_certificate: absolute("LAYERX_HUMAN_KMS_CLIENT_CERTIFICATE_DER")?,
            client_private_key: absolute("LAYERX_HUMAN_KMS_CLIENT_PRIVATE_KEY_DER")?,
            limits: KmsLimits {
                maximum_frame_bytes: number("LAYERX_HUMAN_KMS_MAX_FRAME_BYTES")?,
                maximum_connections: number("LAYERX_HUMAN_KMS_MAX_CONNECTIONS")?,
                maximum_streams: number("LAYERX_HUMAN_KMS_MAX_STREAMS")?,
                maximum_queued_bytes: number("LAYERX_HUMAN_KMS_MAX_QUEUED_BYTES")?,
                deadline: Duration::from_secs(number("LAYERX_HUMAN_KMS_DEADLINE_SECONDS")?),
            },
        })
    }
}

/// In-process owners used by the privileged component listener.
pub struct ProductionComponents {
    clock: Arc<dyn Clock>,
    store: Arc<Mutex<PrincipalStore>>,
    event_outbox: Arc<crate::event_producer::HumanOutbox>,
    passkeys: Passkeys,
    auth_index: AuthDiscoveryIndex,
    capability_ttl_seconds: u64,
    agent: Mutex<AgentRuntime>,
    agent_contract: layerx_sdk::Client,
    agent_limits: Limits,
    native_asset: [u8; 32],
    custody: Arc<CustodySigner>,
    attestor: Option<AttestorKms>,
    security: Mutex<RemoteSecurityProvider>,
    stream: super::stream_journal::StreamJournal,
    feed: Feed,
    activity_export_maximum_bytes: usize,
    identity: RemoteIdentityProvider,
    agent_actor: AgentDid,
    agent_authority: AuthorityRef,
    agent_timestamp_span_seconds: u64,
    agent_fee_limit: u128,
    onboarding_sponsor_principal: crate::store::PrincipalId,
    onboarding_initial_funding: u128,
    evm_gas_limit: u64,
    evm_max_fee_per_gas: u64,
    evm_max_priority_fee_per_gas: u64,
    network_id: u32,
    protocol_version: u16,
    activity_freshness_seconds: u64,
    agent_purpose_catalog: PurposePresetCatalog,
    agent_owner_account: String,
    agent_recovery_root: [u8; 32],
    agent_recovery_threshold: u16,
    continuation_unknown_deadline_seconds: u64,
    maintenance_healthy: AtomicBool,
}

#[path = "production_rotation.rs"]
mod owner_rotation;

#[path = "onboarding_sponsor.rs"]
mod onboarding_sponsor;
pub use onboarding_sponsor::onboarding_sponsor_command;

#[path = "production_onboarding.rs"]
mod onboarding_native;
#[path = "production_owner.rs"]
mod owner;
use owner::resolve_principal_owner;

impl ProductionComponents {
    pub async fn provision_native_send_access(
        &self,
        scope: &mut crate::store::PrincipalScope<'_>,
        authenticated: &crate::auth::StepUpEvidence,
        expected_operation: crate::auth::OperationDigest,
        request_digest: [u8; 32],
        trace: &TraceId,
        access: &super::native_send::NativeSendAccessRequest,
    ) -> Result<super::native_send::HumanOwnerNativeContextV1, ApiFailure> {
        let now = self.now()?;
        if request_digest == [0; 32] || expected_operation.bytes() == [0; 32] {
            return Err(ApiFailure::forbidden());
        }
        if expected_operation
            != super::native_send::access_operation(scope, access)
                .map_err(agent_failure_from_creation_contract)?
        {
            return Err(ApiFailure::forbidden());
        }
        self.passkeys
            .revalidate_step_up(scope, authenticated, expected_operation, now)
            .map_err(|_| ApiFailure::forbidden())?;
        let mut runtime = self.principal_agent(scope)?;
        let owner = resolve_principal_owner(self, scope, &mut runtime)?;
        if access.owner.as_bytes() != owner.actor.as_str().as_bytes()
            || owner.authority.as_str() != hex_bytes(&access.owner_public_key)
        {
            return Err(ApiFailure::forbidden());
        }
        let registry = runtime.registry().clone();
        let session = {
            let mut producer = ProductionAgentCreation::new(
                &mut runtime,
                &self.agent_contract,
                &self.custody,
                trace,
                owner.actor,
                owner.authority,
                super::agent_creation::CreationBounds {
                    timestamp_span: self.agent_timestamp_span_seconds,
                    fee_limit: self.agent_fee_limit,
                },
            )
            .map_err(|_| ApiFailure::upstream_degraded())?;
            producer
                .provision_human_send_session(scope, &registry, access)
                .map_err(agent_failure_from_creation_contract)?
        };
        super::native_send::authorize_access(
            scope,
            &mut runtime,
            &self.custody,
            &self.passkeys,
            authenticated,
            expected_operation,
            request_digest,
            trace,
            access,
            session,
            now,
        )
        .await
        .map_err(agent_failure_from_creation_contract)
    }

    pub fn preview_native_send(
        &self,
        scope: &crate::store::PrincipalScope<'_>,
        preparation: &layerx_agent_api::prepare::PrepareRequest,
        capability_id: [u8; 32],
        purpose_expires_at_ms: u64,
        commitment: [u8; 32],
    ) -> Result<super::native_send::NativeSendPreviewV1, ApiFailure> {
        let mut runtime = self.principal_agent(scope)?;
        super::native_send::preview_send(
            scope,
            &mut runtime,
            preparation,
            capability_id,
            purpose_expires_at_ms,
            commitment,
            self.now()?,
        )
        .map_err(agent_failure_from_creation_contract)
    }

    /// # Errors
    /// Refuses unverified production dependencies or invalid configuration.
    pub fn open(
        mut config: ProductionComponentsConfig,
        clock: Arc<dyn Clock>,
    ) -> Result<Self, String> {
        validate_production_configuration(&config)?;
        require_attestor_custody_protocol(config.attestor.is_some(), config.protocol_version)?;
        let provider = match (config.attestor.take(), config.kms.take()) {
            (Some(attestor), _) => ProductionKms::Attestor(
                AttestorKms::connect_durable(
                    attestor,
                    config.custody_root.join("send-replay"),
                    config.auth_index_key,
                    Arc::clone(&clock),
                )
                .map_err(|_| "attestor quorum refused startup".to_owned())?,
            ),
            (None, Some(kms)) => {
                ProductionKms::Remote(production_kms_provider(&kms, Arc::clone(&clock))?)
            }
            (None, None) => {
                return Err("neither attestor custody nor the remote KMS is configured".to_owned())
            }
        };
        let passkeys =
            Passkeys::new(config.auth).map_err(|_| "invalid passkey configuration".to_owned())?;
        let agent_purpose_catalog =
            PurposePresetCatalog::from_json(&read_nonempty(&config.agent_purpose_catalog)?)
                .map_err(|_| "agent purpose catalog was refused".to_owned())?;
        let store = production_principal_store(
            config.store_root,
            config.retention,
            config.tenancy_digest,
            config.identity_binding,
        )?;
        let auth_index = production_auth_index(config.auth_index_root, config.auth_index_key)?;
        let (agent_contract, agent, native_asset) =
            production_agent(config.agent_socket, config.agent_limits, Arc::clone(&clock))?;
        let (keystore, attestor) = match provider {
            ProductionKms::Attestor(attestor) => (
                Keystore::open_production(config.custody_root, config.network_id, attestor.clone()),
                Some(attestor),
            ),
            ProductionKms::Remote(remote) => (
                Keystore::open_production(config.custody_root, config.network_id, remote),
                None,
            ),
        };
        let keystore = keystore.map_err(|_| "KMS or custody storage refused startup".to_owned())?;
        let custody = Arc::new(CustodySigner::new_shared(
            keystore,
            Arc::clone(&store),
            agent.registry().clone(),
            config.signing_limits,
        ));
        let security = RemoteSecurityProvider::new(config.security)
            .map_err(|_| "security provider configuration was refused".to_owned())?;
        let identity = RemoteIdentityProvider::new(config.identity)
            .map_err(|_| "identity provider configuration was refused".to_owned())?;
        let event_outbox = crate::event_producer::HumanOutbox::start(Arc::clone(&store))?;
        Ok(Self {
            clock,
            store,
            event_outbox,
            passkeys,
            auth_index,
            capability_ttl_seconds: config.capability_ttl_seconds,
            agent: Mutex::new(agent),
            agent_contract,
            agent_limits: config.agent_limits,
            native_asset,
            custody,
            attestor,
            security: Mutex::new(security),
            stream: super::stream_journal::StreamJournal::new(config.stream_cursor_key),
            feed: Feed::new(config.activity_freshness_seconds)
                .map_err(|_| "activity freshness bound is invalid".to_owned())?,
            activity_export_maximum_bytes: config.activity_export_maximum_bytes,
            identity,
            agent_actor: config.agent_actor,
            agent_authority: config.agent_authority,
            agent_timestamp_span_seconds: config.agent_timestamp_span_seconds,
            agent_fee_limit: config.agent_fee_limit,
            onboarding_sponsor_principal: config.onboarding_sponsor_principal,
            onboarding_initial_funding: config.onboarding_initial_funding,
            evm_gas_limit: config.evm_gas_limit,
            evm_max_fee_per_gas: config.evm_max_fee_per_gas,
            evm_max_priority_fee_per_gas: config.evm_max_priority_fee_per_gas,
            network_id: config.network_id,
            protocol_version: config.protocol_version,
            activity_freshness_seconds: config.activity_freshness_seconds,
            agent_purpose_catalog,
            agent_owner_account: config.agent_owner_account,
            agent_recovery_root: config.agent_recovery_root,
            agent_recovery_threshold: config.agent_recovery_threshold,
            continuation_unknown_deadline_seconds: config.continuation_unknown_deadline_seconds,
            maintenance_healthy: AtomicBool::new(true),
        })
    }

    #[must_use]
    pub const fn attestor_custody(&self) -> Option<&AttestorKms> {
        self.attestor.as_ref()
    }

    fn revoke_browser_grants(
        &self,
        scope: &mut crate::store::PrincipalScope<'_>,
        request: &ScopedRequest<'_>,
        grants: &[(String, [u8; 32])],
    ) -> Result<(), ApiFailure> {
        let mut agent = self.principal_agent(scope)?;
        let owner = resolve_principal_owner(self, scope, &mut agent)?;
        let registry = agent.registry().clone();
        let trace =
            TraceId::parse(&request.trace).map_err(|_| ApiFailure::invalid_request(None))?;
        for (session_id, grant_id) in grants {
            let effective = agent
                .head()
                .map_err(agent_failure)?
                .chain_sequence
                .checked_add(1)
                .ok_or_else(ApiFailure::upstream_degraded)?;
            let intent = Intent::v1(IntentKind::SessionRevoke(
                SessionRevoke::new(
                    AuthorityGrantId::new(*grant_id),
                    SessionRevocationReason::SignedOut,
                    ProtocolSequence::from_u64(effective),
                )
                .map_err(|_| ApiFailure::upstream_degraded())?,
            ));
            let key = action_key(&format!("{}:{session_id}", required_idempotency(request)?));
            let current = self.now()?;
            let mut adapter = ProductionAgentCreation::new(
                &mut agent,
                &self.agent_contract,
                &self.custody,
                &trace,
                owner.actor.clone(),
                owner.authority.clone(),
                super::agent_creation::CreationBounds {
                    timestamp_span: self.agent_timestamp_span_seconds,
                    fee_limit: self.agent_fee_limit,
                },
            )
            .map_err(|_| ApiFailure::upstream_degraded())?;
            let evidence = adapter
                .submit_lifecycle_intent(
                    scope,
                    &registry,
                    intent,
                    key,
                    KeyId::new("human-primary").map_err(|_| ApiFailure::upstream_degraded())?,
                    current,
                )
                .map_err(|_| ApiFailure::upstream_degraded())?;
            ProductionAgentCreation::finalization_evidence(
                &evidence,
                ModuleId::Governance,
                6,
                current,
            )
            .map_err(|_| ApiFailure::upstream_degraded())?;
        }
        Ok(())
    }
}

impl HumanApiComponents for ProductionComponents {
    fn authorize(
        &self,
        operation: &Operation,
        credentials: SessionCredentials<'_>,
        trace: &str,
    ) -> Result<PrincipalContext, ApiFailure> {
        let now = self.now()?;
        let mut store = self.store.lock().map_err(|_| ApiFailure::unavailable())?;
        if operation.name == "session.refresh" {
            let csrf = credentials.csrf_token.ok_or_else(ApiFailure::forbidden)?;
            return authorize_refresh_execution(
                &mut store,
                &self.auth_index,
                credentials.access_token,
                csrf,
                AuthorizationDisclosure {
                    operation,
                    destination: credentials.intended_destination,
                    path_parameters: credentials.path_parameters,
                    body: credentials.body,
                    idempotency_key: credentials.idempotency_key,
                    trace,
                },
                now,
                self.capability_ttl_seconds,
            )
            .map_err(|error| auth_failure(&error));
        }
        let principal = self
            .auth_index
            .resolve_access_token(credentials.access_token, now)
            .map_err(|error| auth_failure(&error))?;
        let step_up_id = credentials
            .body
            .get("step_up")
            .and_then(|value| value.get("challenge_id"))
            .and_then(|value| value.as_str())
            .or_else(|| {
                credentials
                    .body
                    .get("step_up_evidence")
                    .and_then(|value| value.as_str())
            });
        let step_up = if let Some(challenge_id) = step_up_id {
            let mut scope = store
                .principal(&principal)
                .map_err(|_| ApiFailure::unavailable())?;
            Some(
                self.passkeys
                    .load_step_up_evidence(&mut scope, challenge_id, now)
                    .map_err(|_| ApiFailure::forbidden())?,
            )
        } else {
            None
        };
        let capability = authorize_execution(
            &mut store,
            &self.passkeys,
            &self.auth_index,
            super::production_auth::ExecutionCredentials {
                access: credentials.access_token,
                csrf: credentials.csrf_token,
                step_up: step_up.as_ref(),
            },
            AuthorizationDisclosure {
                operation,
                destination: credentials.intended_destination,
                path_parameters: credentials.path_parameters,
                body: credentials.body,
                idempotency_key: credentials.idempotency_key,
                trace,
            },
            now,
            self.capability_ttl_seconds,
        )
        .map_err(|error| auth_failure(&error))?;
        if capability.request_disclosure() != credentials.request_digest
            || capability.body_disclosure() != credentials.disclosure_digest
        {
            return Err(ApiFailure::forbidden());
        }
        capability.into_context()
    }

    fn admit_bearer_assertion(
        &self,
        operation: &Operation,
        credentials: BearerCredentials<'_>,
        trace: &str,
    ) -> Result<PrincipalContext, ApiFailure> {
        let now = self.now()?;
        let binding = credentials.wallet_binding;
        let account = self
            .identity
            .resolve_assertion_with_binding(credentials.assertion, binding)
            .map_err(|error| bearer_failure(&error))?;
        let did = account.did.clone().ok_or_else(ApiFailure::forbidden)?;
        let mut store = self.store.lock().map_err(|_| ApiFailure::unavailable())?;
        let capability = authorize_bearer_execution(
            &mut store,
            &self.auth_index,
            &account.principal,
            credentials.assertion,
            AuthorizationDisclosure {
                operation,
                destination: credentials.intended_destination,
                path_parameters: credentials.path_parameters,
                body: credentials.body,
                idempotency_key: credentials.idempotency_key,
                trace,
            },
            now,
            self.capability_ttl_seconds,
        )
        .map_err(|error| auth_failure(&error))?;
        if capability.request_disclosure() != credentials.request_digest
            || capability.body_disclosure() != credentials.disclosure_digest
        {
            return Err(ApiFailure::forbidden());
        }
        capability.into_bearer_context(credentials.assertion, &did)
    }

    fn execute(&self, request: ScopedRequest<'_>) -> Result<BackendResponse, ApiFailure> {
        let _trace = super::agent_runtime::TraceContext::enter(&request.trace)
            .map_err(|_| ApiFailure::invalid_request(None))?;
        if request.operation.name == "version" {
            return Ok(BackendResponse {
                result: json!({"schema": {"major": 1, "minor": 0}, "service": "layerx-human"}),
                session: None,
            });
        }
        if request.operation.is_public_bootstrap() {
            return self.execute_bootstrap(&request);
        }
        let context = request
            .principal
            .as_ref()
            .ok_or_else(ApiFailure::unauthenticated)?;
        consume_context(
            &self.auth_index,
            context,
            AuthorizationDisclosure {
                operation: request.operation,
                destination: context.destination(),
                path_parameters: &request.path_parameters,
                body: &request.body,
                idempotency_key: request.idempotency_key.as_deref(),
                trace: &request.trace,
            },
            self.now()?,
        )
        .map_err(|error| auth_failure(&error))?;
        if context.assertion().is_some() {
            if let Some(attestor) = self.attestor_custody() {
                attestor
                    .admit_context_assertion(context)
                    .map_err(|_| ApiFailure::forbidden())?;
            }
        }
        let principal = context.principal.clone();
        let session_id = context.session_id.clone();
        if request.operation.name == "onboarding.resume" {
            return self.execute_onboarding_resume(&request, &principal);
        }
        let mut store = self.store.lock().map_err(|_| ApiFailure::unavailable())?;
        let mut scope = store
            .principal(&principal)
            .map_err(|_| ApiFailure::unavailable())?;
        let result = self.execute_authorized(&request, &mut scope, &principal, &session_id);
        if request.operation.mutates() {
            super::stream_journal::changed();
        }
        result
    }

    fn stream(
        &self,
        request: ScopedRequest<'_>,
        maximum_bytes: usize,
        emit: &mut dyn FnMut(serde_json::Value) -> Result<(), ApiFailure>,
        cancelled: &dyn Fn() -> bool,
    ) -> Result<(), ApiFailure> {
        if request.operation.name != "stream.next"
            || request.idempotency_key.is_some()
            || maximum_bytes == 0
        {
            return Err(ApiFailure::invalid_request(None));
        }
        let _trace = super::agent_runtime::TraceContext::enter(&request.trace)
            .map_err(|_| ApiFailure::invalid_request(None))?;
        let context = request
            .principal
            .as_ref()
            .ok_or_else(ApiFailure::unauthenticated)?;
        let now = self.now()?;
        let lifetime = context
            .expires_at()
            .checked_sub(now)
            .filter(|value| *value > 0 && *value <= 60)
            .ok_or_else(ApiFailure::session_expired)?;
        consume_context(
            &self.auth_index,
            context,
            AuthorizationDisclosure {
                operation: request.operation,
                destination: context.destination(),
                path_parameters: &request.path_parameters,
                body: &request.body,
                idempotency_key: request.idempotency_key.as_deref(),
                trace: &request.trace,
            },
            now,
        )
        .map_err(|error| auth_failure(&error))?;
        if context.assertion().is_some() {
            return Err(ApiFailure::forbidden());
        }
        let mut deadline = Deadline::start(self.clock.as_ref(), Duration::from_secs(lifetime))
            .map_err(|_| ApiFailure::unavailable())?;
        let maximum_bytes = maximum_bytes.min(1_048_576);
        let mut used = 0_usize;
        let mut count = 0_usize;
        let mut cursor = path(&request, "cursor")?.to_owned();
        let schema = ApiSchema::v1().map_err(|_| ApiFailure::upstream_degraded())?;
        loop {
            if cancelled()
                || count == super::stream_journal::MAX_PAGE
                || deadline
                    .remaining(self.clock.as_ref())
                    .map_err(|_| ApiFailure::unavailable())?
                    .is_zero()
                || self.now()? >= context.expires_at()
            {
                return Ok(());
            }
            let observed_change = super::stream_journal::change_position()?;
            let page = {
                let mut store = self.store.lock().map_err(|_| ApiFailure::unavailable())?;
                let scope = store
                    .principal(&context.principal)
                    .map_err(|_| ApiFailure::unavailable())?;
                if scope.tenant() != &context.tenant {
                    return Err(ApiFailure::forbidden());
                }
                Passkeys::list_sessions_authorized(&scope, &context.session_id)
                    .map_err(|error| auth_api_failure(&error))?;
                self.stream.next_push(&scope, &cursor)?
            };
            let events = page
                .get("events")
                .and_then(serde_json::Value::as_array)
                .ok_or_else(ApiFailure::upstream_degraded)?;
            if events.is_empty() {
                let remaining = deadline
                    .remaining(self.clock.as_ref())
                    .map_err(|_| ApiFailure::unavailable())?;
                super::stream_journal::wait_for_change(observed_change, remaining)?;
                continue;
            }
            for event in events {
                if cancelled()
                    || count == super::stream_journal::MAX_PAGE
                    || deadline
                        .remaining(self.clock.as_ref())
                        .map_err(|_| ApiFailure::unavailable())?
                        .is_zero()
                    || self.now()? >= context.expires_at()
                {
                    return Ok(());
                }
                {
                    let mut store = self.store.lock().map_err(|_| ApiFailure::unavailable())?;
                    let scope = store
                        .principal(&context.principal)
                        .map_err(|_| ApiFailure::unavailable())?;
                    if scope.tenant() != &context.tenant {
                        return Err(ApiFailure::forbidden());
                    }
                    Passkeys::list_sessions_authorized(&scope, &context.session_id)
                        .map_err(|error| auth_api_failure(&error))?;
                }
                let next = event
                    .get("cursor")
                    .and_then(serde_json::Value::as_str)
                    .ok_or_else(ApiFailure::upstream_degraded)?;
                let single = json!({"events":[event],"next_cursor":next});
                schema
                    .encode_response(request.operation, &single)
                    .map_err(|_| ApiFailure::upstream_degraded())?;
                let bytes =
                    serde_json::to_vec(&json!({"ok":true,"trace":request.trace,"result":single}))
                        .map_err(|_| ApiFailure::upstream_degraded())?
                        .len();
                let Some(total) = used
                    .checked_add(bytes)
                    .filter(|value| *value <= maximum_bytes)
                else {
                    return Ok(());
                };
                emit(single)?;
                used = total;
                count += 1;
                cursor = next.to_owned();
            }
        }
    }

    fn readiness(&self, trace: &str) -> Result<Readiness, ApiFailure> {
        let _trace = super::agent_runtime::TraceContext::enter(trace)
            .map_err(|_| ApiFailure::invalid_request(None))?;
        let custody = self.custody.status();
        let custody_ready = matches!(custody.kms, crate::custody::Availability::Available)
            && matches!(custody.storage, crate::custody::Availability::Available)
            && matches!(
                custody.key_references,
                crate::custody::KeyReferenceIntegrity::Verified
            )
            && matches!(custody.rotation, crate::custody::RotationState::Stable);
        let agent_ready = self
            .agent
            .lock()
            .ok()
            .is_some_and(|agent| agent.probe().is_ok());
        let core_ready = self
            .agent_contract
            .daemon_endpoint()
            .is_some_and(|endpoint| {
                AgentRuntime::connect_with_clock(
                    endpoint,
                    self.agent_limits,
                    Arc::clone(&self.clock),
                )
                .and_then(|mut runtime| runtime.head())
                .is_ok()
            });
        let security_ready = self
            .security
            .lock()
            .ok()
            .is_some_and(|security| security.probe().is_ok());
        let identity_ready = self.identity.probe().is_ok();
        let store_ready = self
            .store
            .lock()
            .ok()
            .is_some_and(|store| store.probe().is_ok());
        Ok(Readiness {
            human_service: if self.event_outbox.ready()
                && store_ready
                && security_ready
                && identity_ready
                && self.maintenance_healthy.load(Ordering::Acquire)
            {
                ComponentState::Ready
            } else {
                ComponentState::Unavailable
            },
            custody: if custody_ready {
                ComponentState::Ready
            } else {
                ComponentState::Unavailable
            },
            agent: if agent_ready {
                ComponentState::Ready
            } else {
                ComponentState::Unavailable
            },
            paxeer: if core_ready {
                ComponentState::Ready
            } else {
                ComponentState::Unavailable
            },
            core: if core_ready {
                ComponentState::Ready
            } else {
                ComponentState::Unavailable
            },
        })
    }
}

impl ComponentMaintenance for ProductionComponents {
    fn maintain(&self, maximum_items: usize, observed_at: u64) -> Result<usize, ApiFailure> {
        if maximum_items == 0 {
            return Err(ApiFailure::invalid_request(None));
        }
        let mut principals = self
            .auth_index
            .active_principals(observed_at)
            .map_err(|error| auth_failure(&error))?;
        let mut store = self.store.lock().map_err(|_| ApiFailure::unavailable())?;
        for principal in store
            .known_principals()
            .map_err(|_| ApiFailure::upstream_degraded())?
        {
            if !principals.contains(&principal) {
                principals.push(principal);
            }
        }
        principals.sort();
        let mut advanced = 0usize;
        for principal in principals {
            if advanced == maximum_items {
                break;
            }
            let mut scope = store
                .principal(&principal)
                .map_err(|_| ApiFailure::unavailable())?;
            for journey in
                CreationJourney::list(&scope).map_err(|error| agent_creation_failure(&error))?
            {
                if !journey.native_resume_eligible()
                    || journey
                        .status()
                        .stages
                        .iter()
                        .any(|(_, state)| *state == crate::agents::StageState::Refused)
                {
                    continue;
                }
                let id = creation_journey_id(&journey)?;
                let key = RowKey::new(format!("continuation-agent-create-{}", id.as_str()))
                    .map_err(|_| ApiFailure::upstream_degraded())?;
                if scope.get(Table::Journeys, &key).is_none() {
                    schedule_continuation(&mut scope, "agent-create", &id, observed_at)?;
                }
            }
            let keys = scope.keys(Table::Journeys);
            for key in keys
                .into_iter()
                .filter(|key| key.as_str().starts_with("continuation-"))
            {
                if advanced == maximum_items {
                    break;
                }
                let row = scope
                    .get(Table::Journeys, &key)
                    .ok_or_else(ApiFailure::upstream_degraded)?;
                let mut continuation: Continuation = serde_json::from_slice(row.bytes())
                    .map_err(|_| ApiFailure::upstream_degraded())?;
                if continuation.next_attempt_at > observed_at || continuation.terminal {
                    continue;
                }
                let journey_id = crate::notify::JourneyId::new(continuation.journey_id.clone())
                    .map_err(|_| ApiFailure::upstream_degraded())?;
                let trace = maintenance_trace(&principal, &journey_id, continuation.attempts);
                let outcome = self.advance_continuation(
                    &mut scope,
                    &continuation.kind,
                    &journey_id,
                    &trace,
                    observed_at,
                );
                advanced = advanced.saturating_add(1);
                if let Ok(terminal) = outcome {
                    continuation.updated_at = observed_at;
                    continuation.last_error = None;
                    continuation.unknown_since = None;
                    continuation.unknown_deadline_at = None;
                    continuation.terminal = terminal;
                    continuation.next_attempt_at = if terminal {
                        u64::MAX
                    } else {
                        observed_at.saturating_add(1)
                    };
                } else {
                    continuation.attempts = continuation.attempts.saturating_add(1);
                    continuation.updated_at = observed_at;
                    continuation.last_error = Some("boundary-outcome-unknown".to_owned());
                    let unknown_since = continuation.unknown_since.get_or_insert(observed_at);
                    let deadline =
                        unknown_since.saturating_add(self.continuation_unknown_deadline_seconds);
                    continuation.unknown_deadline_at = Some(deadline);
                    let exponent = continuation.attempts.min(8);
                    let retry = 1_u64.checked_shl(exponent).unwrap_or(256).min(300);
                    continuation.next_attempt_at = if observed_at >= deadline {
                        observed_at.saturating_add(300)
                    } else {
                        observed_at.saturating_add(retry).min(deadline)
                    };
                }
                let bytes = serde_json::to_vec(&continuation)
                    .map_err(|_| ApiFailure::upstream_degraded())?;
                scope
                    .put(Table::Journeys, key, observed_at, bytes)
                    .map_err(|_| ApiFailure::unavailable())?;
            }
        }
        Ok(advanced)
    }

    fn set_maintenance_health(&self, healthy: bool) {
        self.maintenance_healthy.store(healthy, Ordering::Release);
    }
}

impl ProductionComponents {
    fn advance_continuation(
        &self,
        scope: &mut crate::store::PrincipalScope<'_>,
        kind: &str,
        id: &crate::notify::JourneyId,
        trace: &TraceId,
        observed_at: u64,
    ) -> Result<bool, ApiFailure> {
        if kind == "intent-evm" {
            return self.advance_evm_intent(scope, id, observed_at);
        }
        let mut agent = self.principal_agent(scope)?;
        let registry = agent.registry().clone();
        self.bind_native_journey_access(scope, &mut agent, id, None)?;
        match kind {
            "agent-create" => {
                let mut journey = CreationJourney::list(scope)
                    .map_err(|error| agent_creation_failure(&error))?
                    .into_iter()
                    .find(|journey| {
                        format!("jrn_{}", URL_SAFE_NO_PAD.encode(journey.agent_id())) == id.as_str()
                    })
                    .ok_or_else(ApiFailure::not_found)?;
                if !journey.native_resume_eligible() {
                    return Err(ApiFailure::forbidden());
                }
                if journey
                    .status()
                    .stages
                    .iter()
                    .any(|(_, state)| *state == crate::agents::StageState::Refused)
                {
                    return Ok(true);
                }
                let owner = resolve_principal_owner(self, scope, &mut agent)?;
                let projection = journey.projection();
                if projection.owner_account != owner.account.canonical()
                    || projection.network_id != self.network_id
                {
                    return Err(ApiFailure::forbidden());
                }
                let mut adapter = ProductionAgentCreation::new(
                    &mut agent,
                    &self.agent_contract,
                    &self.custody,
                    trace,
                    owner.actor,
                    owner.authority,
                    super::agent_creation::CreationBounds {
                        timestamp_span: self.agent_timestamp_span_seconds,
                        fee_limit: self.agent_fee_limit,
                    },
                )
                .map_err(|_| ApiFailure::upstream_degraded())?;
                let status = journey
                    .resume_native(
                        scope,
                        self.custody.creation_keystore(),
                        &registry,
                        &mut adapter,
                        observed_at,
                    )
                    .map_err(|error| agent_creation_failure(&error))?;
                if matches!(status.state, crate::agents::CreationState::Active) {
                    adapter
                        .publish_creation(&journey.projection())
                        .map_err(|_| ApiFailure::upstream_degraded())?;
                    return Ok(true);
                }
                Ok(status
                    .stages
                    .iter()
                    .any(|(_, state)| *state == crate::agents::StageState::Refused))
            }
            "intent" => {
                let mut journey = crate::journeys::JourneyEngine::load(scope, id)
                    .map_err(|_| ApiFailure::upstream_degraded())?
                    .ok_or_else(ApiFailure::not_found)?;
                let status = crate::server::poll_once_ready(journey.advance(
                    scope,
                    &self.agent_contract,
                    &mut agent,
                    &self.custody,
                    &registry,
                    trace,
                    observed_at,
                ))
                .map_err(|error| {
                    eprintln!(
                        "intent journey {} advance poll failed: {error:?}",
                        id.as_str()
                    );
                    ApiFailure::upstream_degraded()
                })?
                .map_err(|error| {
                    eprintln!("intent journey {} advance failed: {error:?}", id.as_str());
                    ApiFailure::upstream_degraded()
                })?;
                Ok(matches!(
                    status.state(),
                    crate::journeys::JourneyState::Done | crate::journeys::JourneyState::Refused
                ))
            }
            "move" => {
                let mut journey = crate::journeys::MoveJourney::load(scope, id)
                    .map_err(move_journey_failure)?
                    .ok_or_else(ApiFailure::not_found)?;
                let status = crate::server::poll_once_ready(journey.advance(
                    scope,
                    &self.agent_contract,
                    &mut agent,
                    &self.custody,
                    &registry,
                    trace,
                    observed_at,
                ))
                .map_err(|_| ApiFailure::upstream_degraded())?
                .map_err(move_journey_failure)?;
                Ok(matches!(
                    status.stage(),
                    crate::journeys::MoveStage::Done | crate::journeys::MoveStage::Refused
                ))
            }
            "agent-rotation" => {
                drop(agent);
                self.advance_owner_rotation(scope, id, trace, observed_at)
            }
            _ => Err(ApiFailure::upstream_degraded()),
        }
    }
}

#[derive(serde::Serialize, serde::Deserialize)]
struct Continuation {
    kind: String,
    journey_id: String,
    started_at: u64,
    updated_at: u64,
    next_attempt_at: u64,
    attempts: u32,
    #[serde(default)]
    unknown_since: Option<u64>,
    #[serde(default)]
    unknown_deadline_at: Option<u64>,
    #[serde(default)]
    last_error: Option<String>,
    #[serde(default)]
    terminal: bool,
}

fn maintenance_trace(
    principal: &crate::store::PrincipalId,
    id: &crate::notify::JourneyId,
    attempt: u32,
) -> TraceId {
    let digest: [u8; 32] = Sha256::digest(
        [
            b"layerx-human/continuation-trace/v1\0".as_slice(),
            principal.as_str().as_bytes(),
            id.as_str().as_bytes(),
            &attempt.to_be_bytes(),
        ]
        .concat(),
    )
    .into();
    let mut prefix = [0; 16];
    prefix.copy_from_slice(&digest[..16]);
    TraceId::mint(prefix)
}

fn response(value: impl serde::Serialize) -> Result<BackendResponse, ApiFailure> {
    Ok(BackendResponse {
        result: serde_json::to_value(value).map_err(|_| ApiFailure::upstream_degraded())?,
        session: None,
    })
}

pub fn activity_filters(body: &serde_json::Value) -> Result<AppliedFilters, ApiFailure> {
    let Some(filter) = body.get("filter") else {
        return Feed::apply_filters(FilterDraft::new())
            .map_err(|error| activity_feed_failure(&error));
    };
    let object = filter
        .as_object()
        .ok_or_else(|| ApiFailure::invalid_request(Some("filter")))?;
    let mut draft = FilterDraft::new();
    if let Some(kinds) = object.get("kinds") {
        let kinds = kinds
            .as_array()
            .ok_or_else(|| ApiFailure::invalid_request(Some("kinds")))?
            .iter()
            .map(|value| match value.as_str() {
                Some("deposit") => Ok(crate::activity::ActivityKind::Deposit),
                Some("withdrawal") => Ok(crate::activity::ActivityKind::Withdrawal),
                Some("movement") => Ok(crate::activity::ActivityKind::Movement),
                Some("agent-action") => Ok(crate::activity::ActivityKind::AgentAction),
                Some("approval") => Ok(crate::activity::ActivityKind::Approval),
                Some("security-event") => Ok(crate::activity::ActivityKind::Security),
                _ => Err(ApiFailure::invalid_request(Some("kinds"))),
            })
            .collect::<Result<Vec<_>, _>>()?;
        draft = draft.with_kinds(kinds);
    }
    if let Some(agent) = object.get("agent_id").and_then(serde_json::Value::as_str) {
        draft = draft.with_agent(agent);
    }
    let bound = |name| -> Result<Option<u64>, ApiFailure> {
        object
            .get(name)
            .map(|value| {
                value
                    .as_str()
                    .and_then(crate::time::seconds_from_rfc3339)
                    .ok_or_else(|| ApiFailure::invalid_request(Some(name)))
            })
            .transpose()
    };
    let from = bound("from")?;
    let through = bound("to")?;
    draft = draft.with_dates(from, through);
    Feed::apply_filters(draft).map_err(|error| activity_feed_failure(&error))
}

pub fn cache_activity_export(
    scope: &mut crate::store::PrincipalScope<'_>,
    bytes: Vec<u8>,
    evidence_bundle: bool,
    evidence: Vec<serde_json::Value>,
    now: u64,
) -> Result<serde_json::Value, ApiFailure> {
    let created_at = super::projection::unix_time(now)?;
    let digest: [u8; 32] = Sha256::digest(&bytes).into();
    let id = hex_bytes(&digest);
    let (prefix, kind, content_type) = if evidence_bundle {
        (
            "activity-evidence",
            "evidence-bundle",
            "application/vnd.layerx.evidence-bundle",
        )
    } else {
        ("activity-export", "statement", "text/csv; charset=utf-8")
    };
    scope
        .put(
            Table::Cache,
            RowKey::new(format!("{prefix}-{id}")).map_err(|_| ApiFailure::unavailable())?,
            now,
            bytes,
        )
        .map_err(|_| ApiFailure::unavailable())?;
    Ok(
        json!({"export_id":format!("exp_{id}"),"kind":kind,"download_path":format!("/v1/evidence/evd_{id}"),
        "content_type":content_type,"created_at":created_at,"evidence":evidence}),
    )
}

pub fn activity_page_json(
    page: &crate::activity::FeedPage,
    native_currency: &str,
) -> Result<serde_json::Value, ApiFailure> {
    let mut groups =
        std::collections::BTreeMap::<u64, (u128, u128, String, Vec<serde_json::Value>)>::new();
    for entry in page.entries() {
        let mut amount = 0u128;
        let mut currency = None;
        for receipt in entry.receipts() {
            let bytes = receipt
                .canonical()
                .ok_or_else(ApiFailure::upstream_degraded)?;
            let actual = crate::activity::detail::ReceiptActual::from_verified_journey_bytes(
                bytes,
                receipt.reference(),
            )
            .map_err(|_| ApiFailure::upstream_degraded())?;
            amount = amount
                .checked_add(actual.amount())
                .ok_or_else(ApiFailure::upstream_degraded)?;
            let asset = hex_bytes(&actual.asset());
            if currency.as_ref().is_some_and(|value| value != &asset) {
                return Err(ApiFailure::upstream_degraded());
            }
            currency = Some(asset);
        }
        let bucket = entry.occurred_at() / 2_629_800;
        let group = groups.entry(bucket).or_insert((
            0,
            0,
            currency
                .clone()
                .unwrap_or_else(|| native_currency.to_owned()),
            Vec::new(),
        ));
        if let Some(currency) = currency {
            if !group.2.is_empty() && group.2 != currency {
                return Err(ApiFailure::upstream_degraded());
            }
            group.2 = currency;
        }
        match entry.kind() {
            crate::activity::ActivityKind::Deposit => {
                group.0 = group
                    .0
                    .checked_add(amount)
                    .ok_or_else(ApiFailure::upstream_degraded)?;
            }
            crate::activity::ActivityKind::Withdrawal => {
                group.1 = group
                    .1
                    .checked_add(amount)
                    .ok_or_else(ApiFailure::upstream_degraded)?;
            }
            _ => {
                if amount != 0 {
                    return Err(ApiFailure::upstream_degraded());
                }
            }
        }
        group.3.push(json!({"entry_id":entry.entry_id().as_str(),"kind":activity_kind_label(entry.kind()),"state":activity_status_label(entry.status()),"state_copy_key":format!("activity.state.{}",activity_status_label(entry.status())),"summary_copy_key":format!("activity.summary.{}",activity_kind_label(entry.kind())),"occurred_at":super::projection::unix_time(entry.occurred_at())?}));
    }
    let groups=groups.into_iter().rev().map(|(month,(incoming,outgoing,currency,entries))|json!({"month":format!("unix-month-{month}"),"subtotal_in":{"amount":incoming.to_string(),"currency":currency},"subtotal_out":{"amount":outgoing.to_string(),"currency":currency},"entries":entries})).collect::<Vec<_>>();
    Ok(
        json!({"groups":groups,"next_cursor":page.next().map_or("cur_end", FeedCursor::as_str),"filter":activity_filter_json(page.applied_filters())?}),
    )
}

pub fn activity_filter_json(filters: &AppliedFilters) -> Result<serde_json::Value, ApiFailure> {
    let mut result = json!({});
    if !filters.kinds().is_empty() {
        result["kinds"] = json!(filters
            .kinds()
            .iter()
            .map(|kind| activity_kind_label(*kind))
            .collect::<Vec<_>>());
    }
    if let Some(agent) = filters.agent() {
        result["agent_id"] = json!(agent);
    }
    if let Some(from) = filters.from() {
        result["from"] = json!(super::projection::unix_time(from)?);
    }
    if let Some(to) = filters.through() {
        result["to"] = json!(super::projection::unix_time(to)?);
    }
    Ok(result)
}

fn activity_feed_failure(error: &crate::activity::FeedError) -> ApiFailure {
    match error {
        crate::activity::FeedError::Store(_) => ApiFailure::unavailable(),
        _ => ApiFailure::invalid_request(Some("filter")),
    }
}

fn activity_export_failure(error: crate::activity::ExportError) -> ApiFailure {
    match error {
        crate::activity::ExportError::Feed(value) => activity_feed_failure(&value),
        _ => ApiFailure::upstream_degraded(),
    }
}
fn activity_kind_label(value: crate::activity::ActivityKind) -> &'static str {
    match value {
        crate::activity::ActivityKind::Deposit => "deposit",
        crate::activity::ActivityKind::Withdrawal => "withdrawal",
        crate::activity::ActivityKind::Movement => "movement",
        crate::activity::ActivityKind::AgentAction => "agent-action",
        crate::activity::ActivityKind::Approval => "approval",
        crate::activity::ActivityKind::Security => "security-event",
    }
}
fn activity_status_label(value: crate::activity::ActivityStatus) -> &'static str {
    use crate::activity::{ActivityStatus as S, DepositStage as D, WithdrawalStage as W};
    match value {
        S::GettingReady => "getting-ready",
        S::Sending => "sending",
        S::Processing
        | S::Deposit(D::ConfirmingOnPaxeer | D::Crediting)
        | S::Withdrawal(W::Processing | W::WaitingForSettlement) => "processing",
        S::StillChecking => "still-checking",
        S::WaitingForYou | S::Deposit(D::WaitingForWallet) | S::Withdrawal(W::ReadyToClaim) => {
            "waiting-for-you"
        }
        S::Done | S::Deposit(D::Done) | S::Withdrawal(W::PaidOut) => "done",
        S::DoneFinalised => "done-finalised",
        S::DidntGoThrough { .. } => "refused",
    }
}

fn path<'a>(request: &'a ScopedRequest<'_>, name: &str) -> Result<&'a str, ApiFailure> {
    request
        .path_parameters
        .get(name)
        .map(String::as_str)
        .ok_or_else(|| ApiFailure::invalid_request(Some(name)))
}

fn text_field<'a>(body: &'a serde_json::Value, name: &str) -> Result<&'a str, ApiFailure> {
    body.get(name)
        .and_then(serde_json::Value::as_str)
        .ok_or_else(|| ApiFailure::invalid_request(Some(name)))
}

fn money_field<'a>(body: &'a serde_json::Value, name: &str) -> Result<(u128, &'a str), ApiFailure> {
    let money = body
        .get(name)
        .and_then(serde_json::Value::as_object)
        .ok_or_else(|| ApiFailure::invalid_request(Some(name)))?;
    let amount = money
        .get("amount")
        .and_then(serde_json::Value::as_str)
        .ok_or_else(|| ApiFailure::invalid_request(Some(name)))?
        .parse::<u128>()
        .map_err(|_| ApiFailure::invalid_request(Some(name)))?;
    let currency = money
        .get("currency")
        .and_then(serde_json::Value::as_str)
        .filter(|value| !value.is_empty())
        .ok_or_else(|| ApiFailure::invalid_request(Some(name)))?;
    if amount == 0 {
        return Err(ApiFailure::invalid_request(Some(name)));
    }
    Ok((amount, currency))
}

#[derive(serde::Serialize, serde::Deserialize)]
#[serde(deny_unknown_fields)]
struct BankMoveQuote {
    principal: String,
    tenant: String,
    binding_digest: [u8; 32],
    identity_digest: [u8; 32],
    wallet: [u8; 20],
    recipient: [u8; 20],
    denom: String,
    amount: u128,
    currency: String,
    not_before: u64,
    expires_at: u64,
    transaction: crate::custody::EvmTransaction,
}

fn bank_move_quote_key(quote_id: &str) -> Result<RowKey, ApiFailure> {
    let digest = quote_id
        .strip_prefix("qte_")
        .filter(|text| text.len() == 64 && text.bytes().all(|byte| byte.is_ascii_hexdigit()))
        .ok_or_else(|| ApiFailure::invalid_request(Some("quote_id")))?;
    RowKey::new(format!("bank-move-quote-{digest}"))
        .map_err(|_| ApiFailure::invalid_request(Some("quote_id")))
}

fn action_key(value: &str) -> [u8; 32] {
    sha2::Sha256::digest(
        [
            b"layerx-human/movement-action/v1\0".as_slice(),
            value.as_bytes(),
        ]
        .concat(),
    )
    .into()
}

fn intent_observation_key(
    plan_digest: &[u8; 32],
    session_id: &str,
) -> Result<RowKey, crate::store::StoreError> {
    let mut digest = Sha256::new();
    digest.update(plan_digest);
    digest.update(Sha256::digest(session_id.as_bytes()));
    RowKey::new(format!(
        "intent-observation-{}",
        hex_bytes(&digest.finalize())
    ))
}

#[derive(serde::Serialize, serde::Deserialize)]
#[serde(deny_unknown_fields)]
struct HeldIntentObservation {
    profile: u16,
    principal: String,
    tenant: String,
    session_id: String,
    plan_digest: [u8; 32],
    custody_binding_digest: [u8; 32],
    identity_authority_digest: [u8; 32],
    canonical_intent: Vec<u8>,
    canonical_plan: Vec<u8>,
    transactions: Vec<crate::custody::EvmTransaction>,
}

#[derive(serde::Serialize, serde::Deserialize)]
#[serde(deny_unknown_fields)]
struct HeldIntentOutcome {
    canonical_submission: Vec<u8>,
    session_id: String,
    plan_digest: [u8; 32],
    result: serde_json::Value,
}

struct KernelIntentContext {
    account: AccountId,
    actor: AgentDid,
    authority: AuthorityRef,
    account_sequence: u64,
    custody_key: KeyId,
    custody_binding_digest: [u8; 32],
    identity_authority_digest: [u8; 32],
    network: layerx_types::intent::NetworkId,
    protocol_version: u16,
}

fn resolve_kernel_intent_context(
    components: &ProductionComponents,
    scope: &crate::store::PrincipalScope<'_>,
    asset: AssetId,
) -> Result<KernelIntentContext, ApiFailure> {
    let mut agent = components.principal_agent(scope)?;
    let owner = resolve_principal_owner(components, scope, &mut agent)?;
    let balance = agent.balance().map_err(agent_failure)?;
    if balance.account
        != layerx_intents::canonical::account_id_for_protocol(
            &owner.account,
            components.protocol_version,
        )
        .map_err(|_| ApiFailure::forbidden())?
        || balance.asset != asset.bytes()
        || balance.global_sequence != balance.observed_head_sequence
        || balance.age_seconds > components.activity_freshness_seconds
    {
        return Err(ApiFailure::forbidden());
    }
    let custody_key = KeyId::new("human-primary").map_err(|_| ApiFailure::upstream_degraded())?;
    let binding = components
        .custody
        .evm_binding(scope.principal(), &custody_key)
        .map_err(|_| ApiFailure::forbidden())?;
    let account_sequence = agent
        .account_sequence(&owner.actor, &owner.authority)
        .map_err(agent_failure)?;
    Ok(KernelIntentContext {
        account: owner.account,
        actor: owner.actor,
        authority: owner.authority,
        account_sequence,
        custody_key,
        custody_binding_digest: binding.digest(),
        identity_authority_digest: Sha256::digest(&owner.identity.canonical_bytes).into(),
        network: layerx_types::intent::NetworkId::new(components.network_id)
            .map_err(|_| ApiFailure::upstream_degraded())?,
        protocol_version: components.protocol_version,
    })
}

#[derive(Clone, Copy)]
struct KernelSubmission<'a> {
    planned: &'a crate::journeys::UnifiedPlan,
    submitted: &'a crate::journeys::SubmitPlanRequest,
    expectation: &'a crate::journeys::BindingExpectation,
    context: &'a KernelIntentContext,
    idempotency: &'a str,
}

fn submit_failure(refusal: crate::journeys::SubmitRefusal) -> ApiFailure {
    ApiFailure {
        status: refusal.status(),
        code: refusal.code().to_owned(),
        copy_key: refusal.copy_key().to_owned(),
        retry: refusal.retry().to_owned(),
        retry_after_ms: None,
        field: refusal.field().map(str::to_owned),
    }
}

fn intent_budget_route(
    scope: &crate::store::PrincipalScope<'_>,
    agent: &mut AgentRuntime,
    context: &KernelIntentContext,
    leg: &crate::journeys::PlannedLeg,
    binding: &crate::journeys::IntentLegBinding,
    budget: &AccountId,
) -> Result<crate::journeys::RouteRequest, ApiFailure> {
    let managed = CreationJourney::list(scope).map_err(|_| ApiFailure::upstream_degraded())?;
    for journey in &managed {
        let alias = format!("agt_{}", hex_bytes(&journey.agent_id()));
        let agent_context = agent.agent_context(&alias).map_err(agent_failure)?;
        if agent_context.seed.budget_account != budget.canonical() {
            continue;
        }
        if agent_context.seed.owner_account != context.account.canonical()
            || agent_context.seed.budget_asset != leg.asset().bytes()
            || agent_context.protocol_grant_id == [0; 32]
        {
            return Err(ApiFailure::forbidden());
        }
        let state = agent
            .agent_budget_state(agent_context.active_budget_id)
            .map_err(agent_failure)?;
        if state.asset != leg.asset().bytes() || state.age_sequences > state.maximum_age_sequences {
            return Err(ApiFailure::forbidden());
        }
        return crate::journeys::RouteRequest::from_wire_parts(
            leg.source().clone(),
            leg.destination().clone(),
            crate::journeys::Relationship::ManagedBudget(crate::journeys::BudgetRoute {
                budget_id: BudgetId::new(agent_context.active_budget_id),
                idempotency_key: IdempotencyKey::new(binding.action_key),
                revocation_sequence: ProtocolSequence::from_u64(state.revocation_sequence),
                create: None,
            }),
            leg.asset(),
            leg.amount(),
        )
        .map_err(|_| ApiFailure::forbidden());
    }
    Err(ApiFailure::forbidden())
}

fn move_journey_failure(_: crate::journeys::MoveJourneyError) -> ApiFailure {
    ApiFailure::upstream_degraded()
}
fn decode_hex_32(value: &str) -> Result<[u8; 32], ()> {
    let text = value.strip_prefix("0x").ok_or(())?;
    if text.len() != 64 || !text.bytes().all(|byte| byte.is_ascii_hexdigit()) {
        return Err(());
    }
    let mut bytes = [0; 32];
    for (index, byte) in bytes.iter_mut().enumerate() {
        *byte = u8::from_str_radix(&text[index * 2..index * 2 + 2], 16).map_err(|_| ())?;
    }
    Ok(bytes)
}

fn schedule_continuation(
    scope: &mut crate::store::PrincipalScope<'_>,
    kind: &str,
    id: &crate::notify::JourneyId,
    observed_at: u64,
) -> Result<(), ApiFailure> {
    let key = RowKey::new(format!("continuation-{kind}-{}", id.as_str()))
        .map_err(|_| ApiFailure::upstream_degraded())?;
    let continuation = match scope.get(Table::Journeys, &key) {
        Some(row) => {
            let existing: Continuation =
                serde_json::from_slice(row.bytes()).map_err(|_| ApiFailure::upstream_degraded())?;
            if existing.kind != kind || existing.journey_id != id.as_str() {
                return Err(ApiFailure::upstream_degraded());
            }
            existing
        }
        None => Continuation {
            kind: kind.to_owned(),
            journey_id: id.as_str().to_owned(),
            started_at: observed_at,
            updated_at: observed_at,
            next_attempt_at: observed_at,
            attempts: 0,
            unknown_since: None,
            unknown_deadline_at: None,
            last_error: None,
            terminal: false,
        },
    };
    let bytes = serde_json::to_vec(&continuation).map_err(|_| ApiFailure::upstream_degraded())?;
    scope
        .put(Table::Journeys, key, observed_at, bytes)
        .map_err(|_| ApiFailure::unavailable())
}
fn continuation_times(
    scope: &crate::store::PrincipalScope<'_>,
    kind: &str,
    id: &crate::notify::JourneyId,
    now: u64,
) -> Result<(u64, u64), ApiFailure> {
    let key = RowKey::new(format!("continuation-{kind}-{}", id.as_str()))
        .map_err(|_| ApiFailure::upstream_degraded())?;
    let value: serde_json::Value = serde_json::from_slice(
        scope
            .get(Table::Journeys, &key)
            .ok_or_else(ApiFailure::upstream_degraded)?
            .bytes(),
    )
    .map_err(|_| ApiFailure::upstream_degraded())?;
    Ok((
        value
            .get("started_at")
            .and_then(serde_json::Value::as_u64)
            .ok_or_else(ApiFailure::upstream_degraded)?,
        value
            .get("updated_at")
            .and_then(serde_json::Value::as_u64)
            .unwrap_or(now),
    ))
}
fn public_journey(
    scope: &crate::store::PrincipalScope<'_>,
    kind: &str,
    id: &crate::notify::JourneyId,
    state: &str,
    copy: &str,
    evidence: &[serde_json::Value],
    now: u64,
) -> Result<serde_json::Value, ApiFailure> {
    let (started_at, updated_at) = continuation_times(scope, kind, id, now)?;
    Ok(
        json!({"journey_id":id.as_str(),"kind":kind,"state":state,"state_copy_key":format!("status.{state}"),
        "stages":[{"stage_id":format!("stg_{kind}"),"copy_key":copy,"state":state,"evidence":evidence}],
        "evidence":evidence,"started_at":started_at,"updated_at":updated_at}),
    )
}
fn move_public_json(
    scope: &crate::store::PrincipalScope<'_>,
    status: &crate::journeys::MoveStatus,
    now: u64,
) -> Result<serde_json::Value, ApiFailure> {
    let state = match status.stage() {
        crate::journeys::MoveStage::Committed => "getting-ready",
        crate::journeys::MoveStage::Moving => "processing",
        crate::journeys::MoveStage::StillChecking => "still-checking",
        crate::journeys::MoveStage::Done => "done",
        crate::journeys::MoveStage::Refused => "refused",
    };
    let evidence = status
        .receipt_references()
        .iter()
        .map(|receipt| {
            let verification = super::production_reads::custody_receipt_label(
                scope,
                receipt.digest(),
            )?;
            Ok(
                json!({"evidence_id":receipt.reference(),"class":"layerx-receipt","verification":verification}),
            )
        })
        .collect::<Result<Vec<_>, ApiFailure>>()?;
    public_journey(
        scope,
        "move",
        status.journey_id(),
        state,
        "movement.stage.progress",
        &evidence,
        now,
    )
}
fn managed_agent_json(
    agent: &mut AgentRuntime,
    mut value: super::agent_runtime::ManagedAgentView,
) -> Result<serde_json::Value, ApiFailure> {
    hydrate_managed_evidence(agent, &mut value)?;
    super::projection::managed_agent(&value)
}
fn hydrate_managed_evidence(
    agent: &mut AgentRuntime,
    value: &mut super::agent_runtime::ManagedAgentView,
) -> Result<(), ApiFailure> {
    for reference in &mut value.evidence {
        let digest = super::projection::digest(
            reference
                .evidence_id
                .strip_prefix("evd_")
                .unwrap_or(&reference.evidence_id),
        )?;
        let material = agent
            .managed_evidence(&value.agent_id, digest)
            .map_err(agent_failure)?;
        reference.verification = material.verification;
        reference.class = "layerx-receipt".to_owned();
    }
    Ok(())
}
fn managed_journey_json(
    value: &super::agent_runtime::ManagedAgentJourney,
) -> Result<serde_json::Value, ApiFailure> {
    let state = ["getting-ready", "processing", "done", "refused"]
        .get(usize::from(value.state))
        .ok_or_else(ApiFailure::upstream_degraded)?;
    let kind = match value.kind.as_str() {
        "reclaim" => "move",
        "archive" => "agent-retire",
        _ => return Err(ApiFailure::upstream_degraded()),
    };
    let stages = value.stages.iter().map(|stage| {
        let state = ["getting-ready", "processing", "done", "refused"].get(usize::from(stage.state)).ok_or_else(ApiFailure::upstream_degraded)?;
        Ok(json!({"stage_id":format!("stg_{}",stage.stage_id),"copy_key":stage.copy_key,"state":state,"evidence":stage.evidence.iter().map(super::projection::managed_evidence).collect::<Result<Vec<_>,_>>()?}))
    }).collect::<Result<Vec<_>,ApiFailure>>()?;
    let time = |value: &str| {
        value
            .parse::<u64>()
            .map_err(|_| ApiFailure::upstream_degraded())
            .and_then(super::projection::unix_time)
    };
    Ok(
        json!({"journey_id":value.journey_id,"kind":kind,"state":state,"state_copy_key":format!("status.{state}"),"stages":stages,"evidence":value.evidence.iter().map(super::projection::managed_evidence).collect::<Result<Vec<_>,_>>()?,"started_at":time(&value.started_at)?,"updated_at":time(&value.updated_at)?}),
    )
}
fn agent_failure(error: crate::journeys::AgentBoundaryError) -> ApiFailure {
    match error {
        crate::journeys::AgentBoundaryError::Refused => ApiFailure::not_found(),
        crate::journeys::AgentBoundaryError::Unavailable
        | crate::journeys::AgentBoundaryError::CorruptResponse => ApiFailure::upstream_degraded(),
    }
}

fn decode_field<T: serde::de::DeserializeOwned>(
    body: &serde_json::Value,
    name: &str,
) -> Result<T, ApiFailure> {
    serde_json::from_value(
        body.get(name)
            .cloned()
            .ok_or_else(|| ApiFailure::invalid_request(Some(name)))?,
    )
    .map_err(|_| ApiFailure::invalid_request(Some(name)))
}

fn optional_decode_field<T: serde::de::DeserializeOwned>(
    body: &serde_json::Value,
    name: &str,
) -> Result<Option<T>, ApiFailure> {
    body.get(name)
        .cloned()
        .map(serde_json::from_value)
        .transpose()
        .map_err(|_| ApiFailure::invalid_request(Some(name)))
}

fn required_idempotency<'a>(request: &'a ScopedRequest<'_>) -> Result<&'a str, ApiFailure> {
    request
        .idempotency_key
        .as_deref()
        .ok_or_else(|| ApiFailure::invalid_request(Some("Idempotency-Key")))
}

fn support_failure(error: &crate::support::SupportError) -> ApiFailure {
    use crate::support::SupportError::{
        Conflict, ConversationExpired, ConversationFull, ConversationLimit, ConversationResolved,
        ConversationUnknown, Corrupt, InvalidBody, InvalidIdempotencyKey, MessageUnknown, Store,
    };
    match error {
        ConversationUnknown | ConversationExpired | MessageUnknown => ApiFailure::not_found(),
        Store(_) => ApiFailure::unavailable(),
        InvalidBody => ApiFailure::invalid_request(Some("body")),
        InvalidIdempotencyKey
        | Conflict
        | ConversationFull
        | ConversationLimit
        | ConversationResolved
        | Corrupt => ApiFailure::invalid_request(None),
    }
}

fn auth_api_failure(error: &crate::auth::AuthError) -> ApiFailure {
    match error {
        crate::auth::AuthError::Unauthenticated => ApiFailure::unauthenticated(),
        crate::auth::AuthError::SessionExpired => ApiFailure::session_expired(),
        crate::auth::AuthError::SessionNotFound => ApiFailure::not_found(),
        crate::auth::AuthError::Store(_) => ApiFailure::unavailable(),
        _ => ApiFailure::forbidden(),
    }
}

fn agent_creation_failure(error: &crate::agents::AgentCreationError) -> ApiFailure {
    use crate::agents::AgentCreationError;
    match error {
        AgentCreationError::InvalidRequest
        | AgentCreationError::InvalidContext
        | AgentCreationError::UnknownPurpose => ApiFailure::invalid_request(None),
        AgentCreationError::IdempotencyConflict
        | AgentCreationError::EvidenceConflict
        | AgentCreationError::Agent(crate::agents::AgentFailure::Refused(_)) => {
            ApiFailure::forbidden()
        }
        AgentCreationError::Agent(crate::agents::AgentFailure::Unavailable) => {
            ApiFailure::upstream_degraded()
        }
        _ => ApiFailure::upstream_degraded(),
    }
}

fn agent_failure_from_creation_contract(error: crate::agents::AgentFailure) -> ApiFailure {
    match error {
        crate::agents::AgentFailure::Unavailable => ApiFailure::upstream_degraded(),
        crate::agents::AgentFailure::Refused(_) => ApiFailure::forbidden(),
    }
}

fn creation_stage_code(stage: crate::agents::CreationStage) -> u8 {
    use crate::agents::CreationStage;
    match stage {
        CreationStage::Custody => 1,
        CreationStage::DidRegistration => 2,
        CreationStage::RecoveryRegistration => 3,
        CreationStage::SessionProvision => 4,
        CreationStage::CapabilityNarrowing => 5,
        CreationStage::BudgetCreation => 6,
        CreationStage::BudgetFunding => 7,
        CreationStage::MainFunding => 8,
        CreationStage::AssetAccountOpening => 9,
        CreationStage::InitialFunding => 10,
    }
}

fn creation_journey_id(journey: &CreationJourney) -> Result<crate::notify::JourneyId, ApiFailure> {
    crate::notify::JourneyId::new(format!(
        "jrn_{}",
        URL_SAFE_NO_PAD.encode(journey.agent_id())
    ))
    .map_err(|_| ApiFailure::upstream_degraded())
}

fn creation_material(
    scope: &crate::store::PrincipalScope<'_>,
    agent: &mut AgentRuntime,
    journey: &CreationJourney,
    stage: crate::agents::CreationStage,
) -> Result<Option<serde_json::Value>, ApiFailure> {
    use crate::agents::CreationStage;
    let source_stage = if stage == CreationStage::Custody {
        CreationStage::DidRegistration
    } else {
        stage
    };
    let full_id = hex_bytes(&journey.agent_id());
    let short_id = &full_id[..32];
    let key = RowKey::new(format!("agent-create-{short_id}"))
        .map_err(|_| ApiFailure::upstream_degraded())?;
    let record: serde_json::Value = serde_json::from_slice(
        scope
            .get(Table::Journeys, &key)
            .ok_or_else(ApiFailure::upstream_degraded)?
            .bytes(),
    )
    .map_err(|_| ApiFailure::upstream_degraded())?;
    let progress = record["stages"]
        .as_array()
        .ok_or_else(ApiFailure::upstream_degraded)?
        .iter()
        .find(|value| {
            serde_json::from_value::<CreationStage>(value["stage"].clone()).ok()
                == Some(source_stage)
        })
        .ok_or_else(ApiFailure::upstream_degraded)?;
    if progress["evidence_digest"].is_null() {
        return Ok(None);
    }
    let expected: [u8; 32] = serde_json::from_value(progress["evidence_digest"].clone())
        .map_err(|_| ApiFailure::upstream_degraded())?;
    let action_key: [u8; 32] = serde_json::from_value(progress["action_key"].clone())
        .map_err(|_| ApiFailure::upstream_degraded())?;
    let object: [u8; 32] = serde_json::from_value(progress["object_id"].clone())
        .map_err(|_| ApiFailure::upstream_degraded())?;
    let code = creation_stage_code(source_stage);
    let evidence_key = RowKey::new(format!("agent-create-{short_id}-e{code}"))
        .map_err(|_| ApiFailure::upstream_degraded())?;
    let row = scope
        .get(Table::Journeys, &evidence_key)
        .ok_or_else(ApiFailure::upstream_degraded)?;
    if source_stage == CreationStage::CapabilityNarrowing {
        let (key, object_id, sequence, rung_byte, recorded): (
            [u8; 32],
            [u8; 32],
            u64,
            u8,
            [u8; 32],
        ) = serde_json::from_slice(row.bytes()).map_err(|_| ApiFailure::upstream_degraded())?;
        let observation = crate::agents::AgentEvidence {
            action_key: key,
            object_id,
            observed_sequence: sequence,
            rung: match rung_byte {
                0 => layerx_types::rung::Rung::Submitted,
                1 => layerx_types::rung::Rung::Receipted,
                2 => layerx_types::rung::Rung::Committed,
                _ => return Err(ApiFailure::upstream_degraded()),
            },
            receipt_digest: recorded,
        };
        if key != action_key
            || object_id != object
            || object_id == [0; 32]
            || sequence == 0
            || !observation.creation_complete()
            || recorded != expected
            || recorded != observation.expected_digest(source_stage)
        {
            return Err(ApiFailure::upstream_degraded());
        }
        return Ok(Some(super::projection::owned_material(
            row.bytes(),
            "local-journey-state",
            "application/vnd.layerx.agent-capability-evidence",
        )));
    }
    let native_key = RowKey::new(format!("native-create-activity-{full_id}-{code}"))
        .map_err(|_| ApiFailure::upstream_degraded())?;
    let protocol_key = RowKey::new(format!("protocol-signed-{}", hex_bytes(&action_key)))
        .map_err(|_| ApiFailure::upstream_degraded())?;
    let signed = scope
        .get(Table::Journeys, &native_key)
        .or_else(|| scope.get(Table::Journeys, &protocol_key))
        .ok_or_else(ApiFailure::upstream_degraded)?;
    let (call, authority) = verify_owner_activity(signed.bytes())?;
    let sender = authority[12..]
        .try_into()
        .map_err(|_| ApiFailure::upstream_degraded())?;
    let disclosure = bind_call(&call, sender).map_err(|_| ApiFailure::upstream_degraded())?;
    let protocol_key =
        crate::agents::ProtocolEvidence::creation_idempotency_key(&disclosure, action_key)
            .map_err(agent_failure_from_creation_contract)?;
    if disclosure.idempotency_key != protocol_key
        || call.chain_id != u64::from(journey.projection().network_id)
    {
        return Err(ApiFailure::upstream_degraded());
    }
    let activity_id = transaction_hash(signed.bytes());
    if source_stage != CreationStage::SessionProvision && activity_id != object {
        return Err(ApiFailure::upstream_degraded());
    }
    let material = match crate::journeys::AgentBoundary::receipt_by_idempotency_key(
        agent,
        protocol_key,
        activity_id,
    )
    .map_err(agent_failure)?
    {
        crate::journeys::ReceiptLookup::Found(value) => value,
        crate::journeys::ReceiptLookup::Absent
        | crate::journeys::ReceiptLookup::Submitted { .. } => {
            return Err(ApiFailure::upstream_degraded())
        }
    };
    let evidence = ProductionAgentCreation::protocol_evidence_for_action(
        signed.bytes().to_vec(),
        authority,
        action_key,
        agent.registry(),
        material,
    )
    .map_err(|_| ApiFailure::upstream_degraded())?;
    super::agent_creation::verify_evm_outcome(&evidence, disclosure.activity_type)
        .map_err(agent_failure_from_creation_contract)?;
    if <[u8; 32]>::from(Sha256::digest(evidence.receipt_bytes.as_slice())) != expected
        || (source_stage != CreationStage::SessionProvision
            && row.bytes() != evidence.receipt_bytes.as_slice())
    {
        return Err(ApiFailure::upstream_degraded());
    }
    if source_stage == CreationStage::SessionProvision {
        let (key, object_id, sequence, rung_byte, recorded): (
            [u8; 32],
            [u8; 32],
            u64,
            u8,
            [u8; 32],
        ) = serde_json::from_slice(row.bytes()).map_err(|_| ApiFailure::upstream_degraded())?;
        if key != action_key
            || object_id != object
            || object_id == [0; 32]
            || sequence == 0
            || rung_byte != layerx_types::rung::Rung::Committed.wire_rank()
            || rung_byte != evidence.rung.wire_rank()
            || recorded != expected
        {
            return Err(ApiFailure::upstream_degraded());
        }
    }
    let mut value = super::projection::owned_material(
        evidence.receipt_bytes.as_slice(),
        "layerx-receipt",
        "application/vnd.layerx.receipt",
    );
    value["verification"] = json!(super::HumanComponentServer::verification_level_name(
        evidence.rung
    ));
    Ok(Some(value))
}

fn agent_creation_json(
    scope: &crate::store::PrincipalScope<'_>,
    agent: &mut AgentRuntime,
    journey: &CreationJourney,
    status: &crate::agents::CreationStatus,
) -> Result<serde_json::Value, ApiFailure> {
    use crate::agents::{CreationStage, CreationState, StageState};
    let projection = journey.projection();
    let state = if status
        .stages
        .iter()
        .any(|(_, state)| *state == StageState::Refused)
    {
        "refused"
    } else if status
        .stages
        .iter()
        .any(|(_, state)| *state == StageState::Unavailable)
    {
        "still-checking"
    } else {
        match status.state {
            CreationState::GettingReady => "getting-ready",
            CreationState::Partial => "processing",
            CreationState::Active => "done",
        }
    };
    let mut stages = Vec::new();
    let mut evidence = Vec::new();
    for (stage, stage_state) in &status.stages {
        if *stage == CreationStage::CapabilityNarrowing {
            continue;
        }
        let mut references = Vec::new();
        if let Some(material) = creation_material(scope, agent, journey, *stage)? {
            references.push(json!({"evidence_id":material["evidence_id"],
                "class":material["class"],"verification":material["verification"]}));
        }
        let mut states = vec![*stage_state];
        if *stage == CreationStage::SessionProvision {
            if let Some((capability, capability_state)) = status
                .stages
                .iter()
                .find(|(candidate, _)| *candidate == CreationStage::CapabilityNarrowing)
            {
                states.push(*capability_state);
                if let Some(material) = creation_material(scope, agent, journey, *capability)? {
                    references.push(json!({"evidence_id":material["evidence_id"],
                        "class":material["class"],"verification":material["verification"]}));
                }
            }
        }
        let stage_state = if states.contains(&StageState::Refused) {
            "refused"
        } else if states.contains(&StageState::Unavailable) {
            "still-checking"
        } else if states
            .iter()
            .all(|state| *state == StageState::ReceiptVerified)
        {
            "done"
        } else {
            "getting-ready"
        };
        let copy = match stage {
            CreationStage::MainFunding
            | CreationStage::InitialFunding
            | CreationStage::BudgetFunding => "agent.create.stage.first-funding",
            CreationStage::RecoveryRegistration
            | CreationStage::SessionProvision
            | CreationStage::BudgetCreation => "agent.create.stage.protection",
            _ => "agent.create.stage.setting-up",
        };
        stages.push(
            json!({"stage_id":format!("stg_agent_create_{}",creation_stage_code(*stage)),
            "copy_key":copy,"state":stage_state,"evidence":references}),
        );
        for reference in references {
            if !evidence.contains(&reference) {
                evidence.push(reference);
            }
        }
    }
    let key = RowKey::new(format!(
        "agent-create-{}",
        &hex_bytes(&journey.agent_id())[..32]
    ))
    .map_err(|_| ApiFailure::upstream_degraded())?;
    let updated_at = scope
        .get(Table::Journeys, &key)
        .ok_or_else(ApiFailure::upstream_degraded)?
        .written_at();
    Ok(json!({
        "journey_id": format!("jrn_{}", URL_SAFE_NO_PAD.encode(status.agent_id)),
        "kind": "agent-create", "state": state,
        "state_copy_key": match state {
            "done" => "agent.create.ready",
            "getting-ready" => "agent.create.progress.title",
            _ => "agent.create.partial",
        },
        "stages": stages, "evidence": evidence,
        "started_at": super::projection::unix_time(projection.started_at)?,
        "updated_at": super::projection::unix_time(updated_at)?,
    }))
}

fn bearer_failure(error: &identity_dispatch::IdentityDispatchError) -> ApiFailure {
    match error {
        identity_dispatch::IdentityDispatchError::ProviderRefused => ApiFailure::unauthenticated(),
        other => identity_failure(other),
    }
}

fn assertion_subject(assertion: &str) -> Result<String, ApiFailure> {
    let payload = assertion
        .split('.')
        .nth(1)
        .and_then(|segment| URL_SAFE_NO_PAD.decode(segment).ok())
        .and_then(|bytes| serde_json::from_slice::<serde_json::Value>(&bytes).ok())
        .ok_or_else(ApiFailure::unauthenticated)?;
    payload
        .get("sub")
        .and_then(serde_json::Value::as_str)
        .filter(|subject| attestor_owner_valid(subject))
        .map(str::to_owned)
        .ok_or_else(ApiFailure::unauthenticated)
}

fn identity_failure(error: &identity_dispatch::IdentityDispatchError) -> ApiFailure {
    match error {
        identity_dispatch::IdentityDispatchError::InvalidInput => ApiFailure::invalid_request(None),
        identity_dispatch::IdentityDispatchError::NotFound => ApiFailure::not_found(),
        identity_dispatch::IdentityDispatchError::ProviderRefused
        | identity_dispatch::IdentityDispatchError::ProviderAuthentication => {
            ApiFailure::forbidden()
        }
        identity_dispatch::IdentityDispatchError::InvalidConfiguration
        | identity_dispatch::IdentityDispatchError::Corrupt
        | identity_dispatch::IdentityDispatchError::ProviderUnavailable
        | identity_dispatch::IdentityDispatchError::ProviderEvidence
        | identity_dispatch::IdentityDispatchError::Store(_) => ApiFailure::upstream_degraded(),
    }
}

fn notify_failure(error: &crate::notify::NotifyError) -> ApiFailure {
    match error {
        crate::notify::NotifyError::NotificationNotFound => ApiFailure::not_found(),
        crate::notify::NotifyError::Store(_) | crate::notify::NotifyError::Audit(_) => {
            ApiFailure::unavailable()
        }
        _ => ApiFailure::upstream_degraded(),
    }
}

fn notification_json(summary: &NotificationSummary) -> Result<serde_json::Value, ApiFailure> {
    super::stream_journal::notification_wire(summary)
}

fn preferences_json(preferences: &Preferences) -> serde_json::Value {
    let channel = |channel: Channel| {
        json!({
            "enabled": preferences.channel(channel).enabled(),
            "classes": crate::notify::NotificationClass::ALL.into_iter().map(|class| json!({
                "class": class.as_str(), "enabled": preferences.channel(channel).class_enabled(class)
            })).collect::<Vec<_>>()
        })
    };
    json!({"push": channel(Channel::Push), "email": channel(Channel::Email),
        "in_app": channel(Channel::InApp), "detail": preferences.detail().as_str()})
}

fn parse_preferences(body: &serde_json::Value) -> Result<Preferences, ApiFailure> {
    use crate::notify::{DetailLevel, NotificationClass};
    let mut preferences = Preferences::default();
    preferences.set_detail(match text_field(body, "detail")? {
        "full" => DetailLevel::Full,
        "summary" => DetailLevel::Summary,
        "minimal" => DetailLevel::Minimal,
        _ => return Err(ApiFailure::invalid_request(Some("detail"))),
    });
    for (name, channel) in [
        ("push", Channel::Push),
        ("email", Channel::Email),
        ("in_app", Channel::InApp),
    ] {
        let value = body
            .get(name)
            .and_then(serde_json::Value::as_object)
            .ok_or_else(|| ApiFailure::invalid_request(Some(name)))?;
        let enabled = value
            .get("enabled")
            .and_then(serde_json::Value::as_bool)
            .ok_or_else(|| ApiFailure::invalid_request(Some(name)))?;
        preferences.set_channel(channel, enabled);
        let classes = value
            .get("classes")
            .and_then(serde_json::Value::as_array)
            .ok_or_else(|| ApiFailure::invalid_request(Some(name)))?;
        if classes.len() != NotificationClass::ALL.len() {
            return Err(ApiFailure::invalid_request(Some(name)));
        }
        let mut seen = std::collections::BTreeSet::new();
        for entry in classes {
            let class_name = text_field(entry, "class")?;
            let class = NotificationClass::ALL
                .into_iter()
                .find(|candidate| candidate.as_str() == class_name)
                .ok_or_else(|| ApiFailure::invalid_request(Some("class")))?;
            if !seen.insert(class.as_str()) {
                return Err(ApiFailure::invalid_request(Some("class")));
            }
            let selected = entry
                .get("enabled")
                .and_then(serde_json::Value::as_bool)
                .ok_or_else(|| ApiFailure::invalid_request(Some("enabled")))?;
            preferences.set_class(channel, class, selected);
        }
    }
    Ok(preferences)
}

fn authenticator_method_json(
    method: &AuthenticatorMethod,
) -> Result<serde_json::Value, ApiFailure> {
    super::identity::IdentityProjector::authenticator_method(method)
}

fn authenticator_status_json(
    status: &AuthenticatorStatus,
) -> Result<serde_json::Value, ApiFailure> {
    Ok(
        json!({"methods": status.methods.iter().map(authenticator_method_json).collect::<Result<Vec<_>, _>>()?,
        "backup_codes_remaining": status.backup_codes_remaining}),
    )
}

fn timed_secret_json(
    secret: &crate::security::TimedSecret,
) -> Result<serde_json::Value, ApiFailure> {
    super::identity::IdentityProjector::timed_secret(secret)
}

fn auth_failure(error: &super::production_auth::ProductionAuthError) -> ApiFailure {
    use super::production_auth::ProductionAuthError::{
        Auth, CapabilityRefused, CapabilitySpent, Conflict, Entropy, IndexAuthentication,
        InvalidConfiguration, InvalidDisclosure, Io, SessionExpired, Store, Unauthenticated,
        Unavailable, UnclassifiedOperation,
    };
    match error {
        Unauthenticated => ApiFailure::unauthenticated(),
        SessionExpired => ApiFailure::session_expired(),
        InvalidDisclosure | UnclassifiedOperation => ApiFailure::invalid_request(None),
        CapabilitySpent | CapabilityRefused | Conflict => ApiFailure::forbidden(),
        InvalidConfiguration | IndexAuthentication | Entropy | Unavailable | Io(_) | Store(_)
        | Auth(_) => ApiFailure::unavailable(),
    }
}

impl ProductionComponents {
    fn now(&self) -> Result<u64, ApiFailure> {
        self.clock
            .sample(Duration::from_secs(1))
            .map(layerx_types::clock::ClockReading::unix_seconds)
            .map_err(|_| ApiFailure::unavailable())
    }
}

fn mutual_tls(
    root: &PathBuf,
    certificate: &PathBuf,
    key: &PathBuf,
) -> Result<MutualTlsConfig, String> {
    let mut roots = RootCertStore::empty();
    roots
        .add(CertificateDer::from(read_nonempty(root)?))
        .map_err(|_| "KMS trust root DER is invalid".to_owned())?;
    let certificate = CertificateDer::from(read_nonempty(certificate)?);
    let key = PrivateKeyDer::try_from(read_nonempty(key)?)
        .map_err(|_| "KMS client private key DER is invalid".to_owned())?;
    MutualTlsConfig::new(roots, vec![certificate], key)
        .map_err(|_| "KMS mutual TLS configuration is invalid".to_owned())
}

fn read_nonempty(path: &PathBuf) -> Result<Vec<u8>, String> {
    fs::read(path)
        .map_err(|_| format!("cannot read {}", path.display()))
        .and_then(|bytes| {
            if bytes.is_empty() {
                Err(format!("{} is empty", path.display()))
            } else {
                Ok(bytes)
            }
        })
}

fn required(name: &str) -> Result<String, String> {
    env::var(name)
        .ok()
        .filter(|value| !value.is_empty())
        .ok_or_else(|| format!("{name} is required"))
}

fn absolute(name: &str) -> Result<PathBuf, String> {
    let path = PathBuf::from(required(name)?);
    if path.is_absolute() {
        Ok(path)
    } else {
        Err(format!("{name} must be absolute"))
    }
}

fn number<T: std::str::FromStr>(name: &str) -> Result<T, String> {
    required(name)?
        .parse()
        .map_err(|_| format!("{name} is invalid"))
}

fn secret32(name: &str) -> Result<[u8; 32], String> {
    let bytes = URL_SAFE_NO_PAD
        .decode(required(name)?)
        .map_err(|_| format!("{name} is invalid"))?;
    bytes
        .try_into()
        .map_err(|_| format!("{name} must encode exactly 32 bytes"))
}

fn decode_hex_20(value: &str) -> Result<[u8; 20], ()> {
    let value = value.strip_prefix("0x").ok_or(())?;
    if value.len() != 40 || !value.bytes().all(|byte| byte.is_ascii_hexdigit()) {
        return Err(());
    }
    let mut out = [0; 20];
    for (index, byte) in out.iter_mut().enumerate() {
        *byte = u8::from_str_radix(&value[index * 2..index * 2 + 2], 16).map_err(|_| ())?;
    }
    Ok(out)
}

fn decode_hex(value: &str) -> Result<Vec<u8>, ()> {
    let value = value.strip_prefix("0x").ok_or(())?;
    if value.is_empty() || !value.len().is_multiple_of(2) || value.len() > 512 {
        return Err(());
    }
    (0..value.len())
        .step_by(2)
        .map(|index| u8::from_str_radix(&value[index..index + 2], 16).map_err(|_| ()))
        .collect()
}

fn journey_status_json(status: &crate::journeys::JourneyStatus) -> serde_json::Value {
    use crate::journeys::JourneyState;
    json!({"journey_id": status.journey_id().as_str(), "kind":"wallet-binding",
        "state": match status.state() { JourneyState::GettingReady => "getting-ready", JourneyState::Sending => "sending",
            JourneyState::Processing => "processing", JourneyState::StillChecking => "still-checking",
            JourneyState::Done => "done", JourneyState::Refused => "refused", JourneyState::Closed => "closed" },
        "current_leg": status.current_leg(),
        "receipt_verified": status.receipt_digests().iter().all(Option::is_some)})
}

fn decode_id(value: &str) -> Result<[u8; 32], ()> {
    let value = value.strip_prefix("apr_").unwrap_or(value);
    if value.len() == 64 {
        return super::projection::digest(value).map_err(|_| ());
    }
    URL_SAFE_NO_PAD
        .decode(value)
        .map_err(|_| ())?
        .try_into()
        .map_err(|_| ())
}

fn append_approval_stream(
    scope: &mut crate::store::PrincipalScope<'_>,
    hold: &AgentApprovalRecord,
    value: &serde_json::Value,
    observed_at: u64,
) -> Result<(), ApiFailure> {
    let (state, kind) = match hold.state {
        AgentApprovalState::AwaitingApproval => ("pending", "approval-created"),
        AgentApprovalState::Approved { .. } => ("approved", "approval-approved"),
        AgentApprovalState::Rejected => ("rejected", "approval-rejected"),
        AgentApprovalState::Expired | AgentApprovalState::Defective => {
            ("expired", "approval-expired")
        }
    };
    super::stream_journal::StreamJournal::append(
        scope,
        &format!(
            "approval:{}:{state}",
            super::projection::hex(&hold.approval_id)
        ),
        kind,
        observed_at,
        json!({"approval":value}),
    )
}

fn managed_for_actor(
    agent: &mut AgentRuntime,
    actor: &str,
) -> Result<super::agent_runtime::ManagedAgentView, ApiFailure> {
    let mut cursor = None;
    let mut seen = std::collections::BTreeSet::new();
    let mut found = None;
    loop {
        let page = agent.agent_list(cursor, 100).map_err(agent_failure)?;
        for value in page.agents {
            let context = agent
                .agent_context(&value.agent_id)
                .map_err(agent_failure)?;
            if context.seed.agent_id != value.agent_id {
                return Err(ApiFailure::upstream_degraded());
            }
            if context.agent_did == actor {
                if found.is_some() {
                    return Err(super::projection::hold_defective());
                }
                found = Some(value);
            }
        }
        match page.next_cursor {
            None => break,
            Some(next) if seen.insert(next) => cursor = Some(next),
            _ => return Err(ApiFailure::upstream_degraded()),
        }
    }
    found.ok_or_else(super::projection::hold_defective)
}

fn project_approval(
    agent: &mut AgentRuntime,
    facts: &super::agent_runtime::AgentApprovalFacts,
    sequence: u64,
) -> Result<super::projection::ApprovalProjection, ApiFailure> {
    let mut managed = managed_for_actor(agent, facts.approval.held_activity.actor.as_str())?;
    hydrate_managed_evidence(agent, &mut managed)?;
    let budget = agent
        .verified_budget_after(&facts.approval, sequence)
        .map_err(|_| ApiFailure::upstream_degraded())?;
    let evidence = managed
        .evidence
        .iter()
        .map(super::projection::managed_evidence)
        .collect::<Result<Vec<_>, _>>()?;
    super::projection::approval(facts, &managed, budget, evidence)
}

fn cache_native_material(
    scope: &mut crate::store::PrincipalScope<'_>,
    material: serde_json::Value,
    observed_at: u64,
) -> Result<serde_json::Value, ApiFailure> {
    let id = material["evidence_id"]
        .as_str()
        .ok_or_else(ApiFailure::upstream_degraded)?;
    let digest = id
        .strip_prefix("evd_")
        .ok_or_else(ApiFailure::upstream_degraded)
        .and_then(super::projection::digest)?;
    let key = RowKey::new(format!(
        "native-approval-evidence-{}",
        super::projection::hex(&digest)
    ))
    .map_err(|_| ApiFailure::upstream_degraded())?;
    scope
        .put(
            Table::Cache,
            key,
            observed_at,
            serde_json::to_vec(&material).map_err(|_| ApiFailure::upstream_degraded())?,
        )
        .map_err(|_| ApiFailure::unavailable())?;
    Ok(json!({"evidence_id":id,"class":material["class"],"verification":material["verification"]}))
}

fn project_native_approval(
    agent: &mut AgentRuntime,
    scope: &mut crate::store::PrincipalScope<'_>,
    facts: &super::agent_runtime::NativeEffectApprovalFacts,
    sequence: u64,
    observed_at: u64,
) -> Result<super::projection::ApprovalProjection, ApiFailure> {
    if !facts.requires_approval {
        return Err(ApiFailure::not_found());
    }
    let mut managed = managed_for_actor(agent, &facts.actor)?;
    let context = agent
        .agent_context(&managed.agent_id)
        .map_err(agent_failure)?;
    if context.agent_did != facts.actor
        || context.seed.agent_id != managed.agent_id
        || context.seed.budget_asset != facts.asset
    {
        return Err(super::projection::hold_defective());
    }
    let budget = agent
        .native_effect_approval_budget(facts.approval_id, facts.held_digest, sequence)
        .map_err(agent_failure)?;
    let mut selected = budget
        .rows
        .iter()
        .filter(|row| row.asset == facts.asset && row.budget_id == context.active_budget_id);
    let (Some(row), None) = (selected.next(), selected.next()) else {
        return Err(super::projection::hold_defective());
    };
    let fee_policy = agent.native_fee_policy().map_err(agent_failure)?;
    let fee_currency = if facts.fee_asset == context.seed.budget_asset {
        managed.currency.clone()
    } else if facts.fee_asset == fee_policy.asset_id {
        fee_policy.currency
    } else {
        return Err(super::projection::hold_defective());
    };
    let material = agent
        .native_effect_approval_material(facts.approval_id, facts.held_digest)
        .map_err(agent_failure)?;
    if material.owner != facts.owner
        || material.approval_id != facts.approval_id
        || material.held_digest != facts.held_digest
    {
        return Err(ApiFailure::upstream_degraded());
    }
    let proof = agent
        .agent_budget_proof(row.budget_id)
        .map_err(agent_failure)?;
    if proof.owner != facts.owner
        || proof.budget_id != row.budget_id
        || proof.asset != row.asset
        || proof.source_account != row.source_account
        || proof.observed_head_sequence != sequence
        || proof.remaining < row.remaining
        || proof.verification != row.verification
        || proof.evidence_digest != row.evidence_digest
        || proof.receipt_digest != row.receipt_digest
        || proof.checkpoint_digest != row.checkpoint_digest
        || proof.age_sequences != row.age_sequences
        || proof.maximum_age_sequences != row.maximum_age_sequences
    {
        return Err(ApiFailure::upstream_degraded());
    }
    hydrate_managed_evidence(agent, &mut managed)?;
    let mut evidence = managed
        .evidence
        .iter()
        .map(super::projection::managed_evidence)
        .collect::<Result<Vec<_>, _>>()?;
    evidence.push(cache_native_material(
        scope,
        super::projection::owned_material(
            &material.canonical_unsigned_bytes,
            "approval-hold",
            "application/vnd.layerx.activity",
        ),
        observed_at,
    )?);
    evidence.push(cache_native_material(
        scope,
        super::projection::owned_material(
            &material.immutable_carrier_bytes,
            "approval-hold",
            "application/vnd.layerx.approval-carrier",
        ),
        observed_at,
    )?);
    evidence.push(cache_native_material(
        scope,
        super::projection::owned_material(
            &material.canonical_budget_bytes,
            "local-journey-state",
            "application/vnd.layerx.budget-allocation",
        ),
        observed_at,
    )?);
    let mut exported = super::projection::owned_material(
        &proof.canonical_export_bytes,
        "checkpoint-proof",
        "application/vnd.layerx.budget-proof",
    );
    exported["verification"] = json!(super::projection::level(proof.verification)?);
    if exported["evidence_id"] != json!(format!("evd_{}", super::projection::hex(&proof.digest))) {
        return Err(ApiFailure::upstream_degraded());
    }
    evidence.push(cache_native_material(scope, exported, observed_at)?);
    super::projection::native_approval(facts, &managed, &budget, row, &fee_currency, evidence)
}

struct ProgramApprovalProjection {
    summary: serde_json::Value,
    detail: serde_json::Value,
    material: serde_json::Value,
    budget: Option<serde_json::Value>,
}

fn project_program_approval(
    agent: &mut AgentRuntime,
    scope: &mut crate::store::PrincipalScope<'_>,
    facts: &super::agent_runtime::NativeProgramApprovalFacts,
    sequence: u64,
    observed_at: u64,
) -> Result<ProgramApprovalProjection, ApiFailure> {
    let managed = managed_for_actor(agent, &facts.actor)?;
    let context = agent
        .agent_context(&managed.agent_id)
        .map_err(agent_failure)?;
    let (_, owner_account) = movement_principal_account(scope)?;
    if context.agent_did != facts.actor
        || context.seed.agent_id != managed.agent_id
        || context.seed.owner_account != owner_account.canonical()
    {
        return Err(super::projection::hold_defective());
    }
    let material = agent
        .native_program_approval_material(facts.approval_id, facts.held_digest)
        .map_err(agent_failure)?;
    material
        .validate_facts(facts, agent.registry())
        .map_err(agent_failure)?;
    let mut evidence = Vec::with_capacity(4);
    for value in [
        super::projection::owned_material(
            &material.canonical_unsigned_bytes,
            "approval-hold",
            "application/vnd.layerx.activity",
        ),
        super::projection::owned_material(
            &material.immutable_carrier_bytes,
            "approval-hold",
            "application/vnd.layerx.approval-carrier",
        ),
        super::projection::owned_material(
            &material.canonical_budget_bytes,
            "local-journey-state",
            "application/vnd.layerx.budget-allocation",
        ),
    ] {
        evidence.push(cache_native_material(scope, value, observed_at)?);
    }
    let has_budget = facts.fee_asset.is_some()
        || matches!(&facts.semantics,
        super::agent_runtime::NativeProgramApprovalSemantics::AuthorizedLimits(rows) if !rows.is_empty());
    let budget = if has_budget {
        let row = agent
            .native_program_approval_budget(facts.approval_id, facts.held_digest, sequence)
            .map_err(agent_failure)?;
        if row.owner != facts.owner
            || row.budget_id != context.active_budget_id
            || row.asset != context.seed.budget_asset
            || row.observed_at_sequence != sequence
        {
            return Err(ApiFailure::upstream_degraded());
        }
        let proof = agent
            .agent_budget_proof(row.budget_id)
            .map_err(agent_failure)?;
        if proof.owner != row.owner
            || proof.budget_id != row.budget_id
            || proof.asset != row.asset
            || proof.source_account != row.source_account
            || proof.observed_head_sequence != sequence
            || proof.remaining < row.remaining
            || proof.verification != row.verification
            || proof.evidence_digest != row.evidence_digest
            || proof.receipt_digest != row.receipt_digest
            || proof.checkpoint_digest != row.checkpoint_digest
            || proof.age_sequences != row.age_sequences
            || proof.maximum_age_sequences != row.maximum_age_sequences
            || proof.digest != row.proof_digest
            || proof.canonical_export_bytes != row.verified_proof_bytes
        {
            return Err(ApiFailure::upstream_degraded());
        }
        let mut exported = super::projection::owned_material(
            &row.verified_proof_bytes,
            "checkpoint-proof",
            "application/vnd.layerx.budget-proof",
        );
        exported["verification"] = json!(super::projection::level(row.verification)?);
        if exported["evidence_id"]
            != json!(format!("evd_{}", super::projection::hex(&row.proof_digest)))
        {
            return Err(ApiFailure::upstream_degraded());
        }
        let reference = cache_native_material(scope, exported, observed_at)?;
        evidence.push(reference.clone());
        Some(super::projection::program_budget(&row, reference)?)
    } else {
        None
    };
    let summary = super::projection::program_summary(facts, &managed, &evidence)?;
    let detail = super::projection::program_detail(facts, &managed, &evidence, budget.as_ref())?;
    Ok(ProgramApprovalProjection {
        summary,
        detail,
        material: super::projection::program_material(facts, &material)?,
        budget,
    })
}

fn program_hold_expired() -> ApiFailure {
    ApiFailure {
        status: 409,
        code: "hold-expired".to_owned(),
        copy_key: "error.approval.hold-expired".to_owned(),
        retry: "final".to_owned(),
        retry_after_ms: None,
        field: None,
    }
}

fn append_program_approval_stream(
    scope: &mut crate::store::PrincipalScope<'_>,
    facts: &super::agent_runtime::NativeProgramApprovalFacts,
    summary: &serde_json::Value,
    observed_at: u64,
) -> Result<(), ApiFailure> {
    let state = super::projection::program_state(facts.state)?;
    let kind = match state {
        "pending" => "program-approval-created",
        "approved" => "program-approval-approved",
        "rejected" => "program-approval-rejected",
        "expired" => "program-approval-expired",
        "defective" | "not-required" => return Ok(()),
        _ => return Err(ApiFailure::upstream_degraded()),
    };
    super::stream_journal::StreamJournal::append(
        scope,
        &format!(
            "program-approval:{}:{state}",
            super::projection::hex(&facts.approval_id)
        ),
        kind,
        observed_at,
        json!({"program_approval":summary}),
    )
}

fn program_approval_inventory(
    agent: &mut AgentRuntime,
    scope: &mut crate::store::PrincipalScope<'_>,
    cursor: Option<[u8; 32]>,
    sequence: u64,
    observed_at: u64,
) -> Result<(Vec<serde_json::Value>, String), ApiFailure> {
    let page = agent
        .native_program_approval_list(cursor, 100)
        .map_err(agent_failure)?;
    let mut summaries = Vec::with_capacity(page.approvals.len());
    for facts in page.approvals {
        let projected = project_program_approval(agent, scope, &facts, sequence, observed_at)?;
        append_program_approval_stream(scope, &facts, &projected.summary, observed_at)?;
        summaries.push(projected.summary);
    }
    Ok((
        summaries,
        page.next_cursor
            .map_or_else(String::new, |cursor| URL_SAFE_NO_PAD.encode(cursor)),
    ))
}

fn append_native_approval_stream(
    scope: &mut crate::store::PrincipalScope<'_>,
    facts: &super::agent_runtime::NativeEffectApprovalFacts,
    summary: &serde_json::Value,
    observed_at: u64,
) -> Result<(), ApiFailure> {
    let state = super::projection::native_state(facts.state)?;
    let kind = match state {
        "pending" => "approval-created",
        "approved" => "approval-approved",
        "rejected" => "approval-rejected",
        _ => "approval-expired",
    };
    super::stream_journal::StreamJournal::append(
        scope,
        &format!(
            "approval:{}:{state}",
            super::projection::hex(&facts.approval_id)
        ),
        kind,
        observed_at,
        json!({"approval":summary}),
    )
}

fn approval_inventory(
    agent: &mut AgentRuntime,
    scope: &mut crate::store::PrincipalScope<'_>,
    sequence: u64,
    observed_at: u64,
) -> Result<(Vec<serde_json::Value>, String), ApiFailure> {
    let legacy = agent
        .approval_list_facts(sequence, None, 100)
        .map_err(agent_failure)?;
    let native = agent
        .native_effect_approval_list_facts(None, 100)
        .map_err(agent_failure)?;
    let mut approvals = std::collections::BTreeMap::new();
    for facts in &legacy.approvals {
        let projected = project_approval(agent, facts, sequence)?;
        append_approval_stream(scope, &facts.approval, &projected.summary, observed_at)?;
        if approvals
            .insert(facts.approval.approval_id, projected.summary)
            .is_some()
        {
            return Err(ApiFailure::upstream_degraded());
        }
    }
    for facts in native
        .approvals
        .iter()
        .filter(|facts| facts.requires_approval)
    {
        let projected = project_native_approval(agent, scope, facts, sequence, observed_at)?;
        append_native_approval_stream(scope, facts, &projected.summary, observed_at)?;
        if approvals
            .insert(facts.approval_id, projected.summary)
            .is_some()
        {
            return Err(ApiFailure::upstream_degraded());
        }
    }
    if approvals.len() > 100 {
        return Err(ApiFailure::upstream_degraded());
    }
    let cursor = match (legacy.next_cursor, native.next_cursor) {
        (None, None) => String::new(),
        (Some(cursor), None) | (None, Some(cursor)) => URL_SAFE_NO_PAD.encode(cursor),
        (Some(_), Some(_)) => return Err(ApiFailure::upstream_degraded()),
    };
    Ok((approvals.into_values().collect(), cursor))
}

fn hex_bytes(bytes: &[u8]) -> String {
    const H: &[u8; 16] = b"0123456789abcdef";
    bytes
        .iter()
        .flat_map(|byte| {
            [
                char::from(H[usize::from(byte >> 4)]),
                char::from(H[usize::from(byte & 15)]),
            ]
        })
        .collect()
}

fn selected_protocol(value: Option<&str>) -> Result<u16, String> {
    let protocol = value.map_or(
        Ok(layerx_intents::canonical::STATE_COMMITMENT_PROTOCOL_VERSION),
        |value| {
            value
                .parse::<u16>()
                .map_err(|_| "LAYERX_HUMAN_PROTOCOL_VERSION is invalid".to_owned())
        },
    )?;
    if protocol != layerx_intents::canonical::STATE_COMMITMENT_PROTOCOL_VERSION {
        return Err("LAYERX_HUMAN_PROTOCOL_VERSION is unsupported".to_owned());
    }
    Ok(protocol)
}

#[cfg(test)]
mod protocol_tests {
    use super::selected_protocol;
    #[test]
    fn explicit_beta_protocol_preserves_legacy_default_and_refuses_unknown_versions() {
        assert_eq!(selected_protocol(None), Ok(3));
        assert!(selected_protocol(Some("2")).is_err());
        assert_eq!(selected_protocol(Some("3")), Ok(3));
        for invalid in ["", "0", "1", "4", "65536", "three"] {
            assert!(selected_protocol(Some(invalid)).is_err());
        }
    }
}

const ATTESTOR_CUSTODY_PROTOCOL: u16 = layerx_intents::canonical::STATE_COMMITMENT_PROTOCOL_VERSION;

fn require_attestor_custody_protocol(attestor: bool, protocol: u16) -> Result<u16, String> {
    if attestor && protocol != ATTESTOR_CUSTODY_PROTOCOL {
        return Err(format!(
            "LAYERX_HUMAN_PROTOCOL_VERSION must be {ATTESTOR_CUSTODY_PROTOCOL} when attestor custody is configured"
        ));
    }
    Ok(protocol)
}

fn selected_custody_protocol(value: Option<&str>, attestor: bool) -> Result<u16, String> {
    let protocol = match value {
        None if attestor => ATTESTOR_CUSTODY_PROTOCOL,
        value => selected_protocol(value).map_err(|error| {
            if attestor {
                format!(
                    "{error}; LAYERX_HUMAN_PROTOCOL_VERSION must be {ATTESTOR_CUSTODY_PROTOCOL} when attestor custody is configured"
                )
            } else {
                error
            }
        })?,
    };
    require_attestor_custody_protocol(attestor, protocol)
}

#[cfg(test)]
mod attestor_custody_protocol {
    use super::{require_attestor_custody_protocol, selected_custody_protocol};

    const REFUSAL: &str =
        "LAYERX_HUMAN_PROTOCOL_VERSION must be 3 when attestor custody is configured";

    #[test]
    fn attestor_custody_protocol_defaults_to_three_when_unset() {
        assert_eq!(selected_custody_protocol(None, true), Ok(3));
        assert_eq!(selected_custody_protocol(Some("3"), true), Ok(3));
        assert_eq!(require_attestor_custody_protocol(true, 3), Ok(3));
    }

    #[test]
    fn attestor_custody_protocol_refuses_any_other_value() {
        for value in ["2", "", "0", "1", "4", "65536", "three"] {
            let refused = selected_custody_protocol(Some(value), true)
                .expect_err("attestor custody accepted a protocol other than 3");
            assert!(
                refused.contains(REFUSAL),
                "refusal for {value:?} does not name the variable and value: {refused}"
            );
        }
        assert_eq!(
            require_attestor_custody_protocol(true, 2),
            Err(REFUSAL.to_owned())
        );
    }

    #[test]
    fn attestor_custody_protocol_keeps_the_existing_behaviour_without_attestors() {
        assert_eq!(selected_custody_protocol(None, false), Ok(3));
        assert_eq!(selected_custody_protocol(Some("2"), false), Ok(2));
        assert_eq!(selected_custody_protocol(Some("3"), false), Ok(3));
        for invalid in ["", "0", "1", "4", "65536", "three"] {
            assert!(selected_custody_protocol(Some(invalid), false).is_err());
        }
        assert_eq!(require_attestor_custody_protocol(false, 2), Ok(2));
    }
}

fn canonical_fee_amount(value: &serde_json::Value, name: &str) -> Result<u128, ApiFailure> {
    let text = text_field(value, name)?;
    let parsed = text
        .parse::<u128>()
        .map_err(|_| ApiFailure::invalid_request(Some("native_fee_budget")))?;
    if parsed.to_string() != text {
        return Err(ApiFailure::invalid_request(Some("native_fee_budget")));
    }
    Ok(parsed)
}

fn native_fee_consent(
    body: &serde_json::Value,
    policy: &super::agent_runtime::NativeFeePolicy,
) -> Result<Option<crate::agents::NativeFeeConsent>, ApiFailure> {
    let Some(value) = body.get("native_fee_budget") else {
        return if policy.version == 2 {
            Err(ApiFailure::invalid_request(Some("native_fee_budget")))
        } else {
            Ok(None)
        };
    };
    let object = value
        .as_object()
        .filter(|object| object.len() == 5)
        .ok_or_else(|| ApiFailure::invalid_request(Some("native_fee_budget")))?;
    let encoded = text_field(value, "asset_id")?;
    if encoded.len() != 64
        || !encoded
            .bytes()
            .all(|byte| byte.is_ascii_digit() || (b'a'..=b'f').contains(&byte))
        || object.keys().any(|key| {
            ![
                "asset_id",
                "maximum_per_activity",
                "maximum_total",
                "period_length_ms",
                "maximum_per_period",
            ]
            .contains(&key.as_str())
        })
    {
        return Err(ApiFailure::invalid_request(Some("native_fee_budget")));
    }
    let mut asset_id = [0; 32];
    for (index, byte) in asset_id.iter_mut().enumerate() {
        *byte = u8::from_str_radix(&encoded[index * 2..index * 2 + 2], 16)
            .map_err(|_| ApiFailure::invalid_request(Some("native_fee_budget")))?;
    }
    if asset_id != policy.asset_id {
        return Err(ApiFailure::invalid_request(Some("native_fee_budget")));
    }
    let consent = crate::agents::NativeFeeConsent {
        asset_id,
        maximum_per_activity: canonical_fee_amount(value, "maximum_per_activity")?,
        maximum_total: canonical_fee_amount(value, "maximum_total")?,
        period_length_ms: u64::try_from(canonical_fee_amount(value, "period_length_ms")?)
            .map_err(|_| ApiFailure::invalid_request(Some("native_fee_budget")))?,
        maximum_per_period: canonical_fee_amount(value, "maximum_per_period")?,
    };
    consent
        .budget(0)
        .map_err(|_| ApiFailure::invalid_request(Some("native_fee_budget")))?;
    Ok(Some(consent))
}

impl ProductionComponents {
    fn execute_agent_create(
        &self,
        request: &ScopedRequest<'_>,
        scope: &mut crate::store::PrincipalScope<'_>,
    ) -> Result<BackendResponse, ApiFailure> {
        let current = self.now()?;
        let limit = request
            .body
            .get("monthly_limit")
            .ok_or_else(|| ApiFailure::invalid_request(Some("monthly_limit")))?;
        let amount = text_field(limit, "amount")?
            .parse::<u128>()
            .map_err(|_| ApiFailure::invalid_request(Some("monthly_limit")))?;
        let fee_policy = self
            .principal_agent(scope)?
            .native_fee_policy()
            .map_err(agent_failure)?;
        let native_fee_budget = native_fee_consent(&request.body, &fee_policy)?;
        let purpose = text_field(&request.body, "purpose")?;
        let activity_types = self
            .agent_purpose_catalog
            .activity_types(purpose)
            .map_err(|error| agent_creation_failure(&error))?;
        let creation = CreateAgentRequest::new(
            text_field(&request.body, "name")?,
            purpose,
            amount,
            text_field(limit, "currency")?,
        )
        .and_then(|value| value.with_activity_types(activity_types))
        .and_then(|value| value.with_native_fee_budget(native_fee_budget))
        .map_err(|_| ApiFailure::invalid_request(None))?;
        let idempotency_key: [u8; 32] =
            Sha256::digest(required_idempotency(request)?.as_bytes()).into();
        let mut runtime = self.principal_agent(scope)?;
        let owner = resolve_principal_owner(self, scope, &mut runtime)?;
        let (human_recovery_root, recovery_threshold) = owner.recovery_policy()?;
        let creation_context = CreationContext {
            idempotency_key,
            owner_account: owner.account.canonical().to_owned(),
            human_recovery_root,
            recovery_threshold,
            network_id: self.network_id,
            protocol_time: current,
        };
        let mut journey = CreationJourney::start_native(
            scope,
            &creation,
            &creation_context,
            &self.agent_purpose_catalog,
            current,
            fee_policy.asset_id,
            self.agent_timestamp_span_seconds,
        )
        .map_err(|error| agent_creation_failure(&error))?;
        schedule_continuation(
            scope,
            "agent-create",
            &creation_journey_id(&journey)?,
            current,
        )?;
        let trace =
            TraceId::parse(&request.trace).map_err(|_| ApiFailure::invalid_request(None))?;
        let registry = runtime.registry().clone();
        let mut adapter = ProductionAgentCreation::new(
            &mut runtime,
            &self.agent_contract,
            &self.custody,
            &trace,
            owner.actor,
            owner.authority,
            super::agent_creation::CreationBounds {
                timestamp_span: self.agent_timestamp_span_seconds,
                fee_limit: self.agent_fee_limit,
            },
        )
        .map_err(|_| ApiFailure::upstream_degraded())?;
        let status = journey
            .resume_native(
                scope,
                self.custody.creation_keystore(),
                &registry,
                &mut adapter,
                current,
            )
            .map_err(|error| agent_creation_failure(&error))?;
        if matches!(status.state, crate::agents::CreationState::Active) {
            adapter
                .publish_creation(&journey.projection())
                .map_err(|_| ApiFailure::upstream_degraded())?;
        }
        Ok(BackendResponse {
            result: agent_creation_json(scope, &mut runtime, &journey, &status)?,
            session: None,
        })
    }

    fn execute_agent_list(
        &self,
        scope: &crate::store::PrincipalScope<'_>,
    ) -> Result<BackendResponse, ApiFailure> {
        let mut agent = self.principal_agent(scope)?;
        let page = agent.agent_list(None, 100).map_err(agent_failure)?;
        Ok(BackendResponse {
            result: json!({"agents": page.agents.into_iter().map(|value|managed_agent_json(&mut agent,value)).collect::<Result<Vec<_>,_>>()?, "next_cursor": page.next_cursor.map(|value| URL_SAFE_NO_PAD.encode(value)).unwrap_or_default()}),
            session: None,
        })
    }

    fn execute_agent_get(
        &self,
        request: &ScopedRequest<'_>,
        scope: &crate::store::PrincipalScope<'_>,
    ) -> Result<BackendResponse, ApiFailure> {
        let mut agent = self.principal_agent(scope)?;
        let value = agent
            .agent_get(path(request, "agent_id")?)
            .map_err(agent_failure)?;
        Ok(BackendResponse {
            result: managed_agent_json(&mut agent, value)?,
            session: None,
        })
    }

    fn execute_agent_pause(
        &self,
        request: &ScopedRequest<'_>,
        scope: &mut crate::store::PrincipalScope<'_>,
    ) -> Result<BackendResponse, ApiFailure> {
        if request.operation.name == "agent.resume" {
            return self.execute_agent_resume(request, scope);
        }
        let mut agent = self.principal_agent(scope)?;
        let agent_id = path(request, "agent_id")?;
        let context = agent.agent_context(agent_id).map_err(agent_failure)?;
        let session = agent
            .agent_session_snapshot(agent_id)
            .map_err(agent_failure)?;
        if session.agent_did != context.agent_did {
            return Err(ApiFailure::upstream_degraded());
        }
        let operation_key = action_key(required_idempotency(request)?);
        let effective_sequence = agent
            .head()
            .map_err(agent_failure)?
            .chain_sequence
            .checked_add(1)
            .ok_or_else(ApiFailure::upstream_degraded)?;
        let intent = Intent::v1(IntentKind::SessionRevoke(
            SessionRevoke::new(
                AuthorityGrantId::new(context.protocol_grant_id),
                SessionRevocationReason::Paused,
                ProtocolSequence::from_u64(effective_sequence),
            )
            .map_err(|_| ApiFailure::upstream_degraded())?,
        ));
        let trace =
            TraceId::parse(&request.trace).map_err(|_| ApiFailure::invalid_request(None))?;
        let registry = agent.registry().clone();
        let current = self.now()?;
        let mut adapter = ProductionAgentCreation::new(
            &mut agent,
            &self.agent_contract,
            &self.custody,
            &trace,
            AgentDid::new(context.seed.actor.clone())
                .map_err(|_| ApiFailure::upstream_degraded())?,
            AuthorityRef::new(context.seed.primary_authority.clone())
                .map_err(|_| ApiFailure::upstream_degraded())?,
            super::agent_creation::CreationBounds {
                timestamp_span: self.agent_timestamp_span_seconds,
                fee_limit: self.agent_fee_limit,
            },
        )
        .map_err(|_| ApiFailure::upstream_degraded())?;
        let receipt = adapter
            .submit_lifecycle_intent(
                scope,
                &registry,
                intent,
                operation_key,
                KeyId::new(context.seed.custody_key)
                    .map_err(|_| ApiFailure::upstream_degraded())?,
                current,
            )
            .map_err(|_| ApiFailure::upstream_degraded())?;
        let evidence = ProductionAgentCreation::finalization_evidence(
            &receipt,
            ModuleId::Governance,
            6,
            self.now()?,
        )
        .map_err(|_| ApiFailure::upstream_degraded())?;
        let observation = agent
            .agent_session_suspend(agent_id, operation_key)
            .map_err(agent_failure)?;
        let value = agent
            .agent_control(agent_id, false, observation.evidence_digest, evidence)
            .map_err(agent_failure)?;
        Ok(BackendResponse {
            result: managed_agent_json(&mut agent, value)?,
            session: None,
        })
    }

    fn execute_agent_limit(
        &self,
        request: &ScopedRequest<'_>,
        scope: &mut crate::store::PrincipalScope<'_>,
    ) -> Result<BackendResponse, ApiFailure> {
        let (amount, currency) = money_field(&request.body, "monthly_limit")?;
        let mut agent = self.principal_agent(scope)?;
        let agent_id = path(request, "agent_id")?;
        let context = agent.agent_context(agent_id).map_err(agent_failure)?;
        if context.seed.currency != currency {
            return Err(ApiFailure::invalid_request(Some("monthly_limit")));
        }
        let operation_key = action_key(required_idempotency(request)?);
        let replacement_budget_id: [u8; 32] = Sha256::digest(
            [
                b"layerx-human/agent-limit-budget/v1".as_slice(),
                context.active_budget_id.as_slice(),
                operation_key.as_slice(),
            ]
            .concat(),
        )
        .into();
        let intent = Intent::v1(IntentKind::BudgetCreate(
            BudgetCreate::new(
                BudgetId::new(replacement_budget_id),
                AccountId::parse(&context.seed.owner_account)
                    .map_err(|_| ApiFailure::upstream_degraded())?,
                AccountId::parse(&context.seed.budget_account)
                    .map_err(|_| ApiFailure::upstream_degraded())?,
                AssetId::new(context.seed.budget_asset),
                ProtocolAmount::from_u128(amount),
                PeriodLength::new(context.seed.budget_period_seconds)
                    .map_err(|_| ApiFailure::upstream_degraded())?,
                RolloverPolicy::None,
                ProtocolAmount::ZERO,
                PurposeHash::new(context.seed.purpose_hash),
                TimestampSeconds::from_u64(
                    self.now()?
                        .checked_add(context.seed.budget_expiry_seconds)
                        .ok_or_else(ApiFailure::upstream_degraded)?,
                ),
            )
            .map_err(|_| ApiFailure::upstream_degraded())?,
        ));
        let trace =
            TraceId::parse(&request.trace).map_err(|_| ApiFailure::invalid_request(None))?;
        let registry = agent.registry().clone();
        let current = self.now()?;
        let mut adapter = ProductionAgentCreation::new(
            &mut agent,
            &self.agent_contract,
            &self.custody,
            &trace,
            AgentDid::new(context.seed.actor.clone())
                .map_err(|_| ApiFailure::upstream_degraded())?,
            AuthorityRef::new(context.seed.primary_authority.clone())
                .map_err(|_| ApiFailure::upstream_degraded())?,
            super::agent_creation::CreationBounds {
                timestamp_span: self.agent_timestamp_span_seconds,
                fee_limit: self.agent_fee_limit,
            },
        )
        .map_err(|_| ApiFailure::upstream_degraded())?;
        let receipt = adapter
            .submit_lifecycle_intent(
                scope,
                &registry,
                intent,
                operation_key,
                KeyId::new(context.seed.custody_key)
                    .map_err(|_| ApiFailure::upstream_degraded())?,
                current,
            )
            .map_err(|_| ApiFailure::upstream_degraded())?;
        let evidence = ProductionAgentCreation::finalization_evidence(
            &receipt,
            ModuleId::Budget,
            1,
            self.now()?,
        )
        .map_err(|_| ApiFailure::upstream_degraded())?;
        let value = agent
            .agent_limit(agent_id, amount, currency, replacement_budget_id, evidence)
            .map_err(agent_failure)?;
        Ok(BackendResponse {
            result: managed_agent_json(&mut agent, value)?,
            session: None,
        })
    }

    fn execute_agent_reclaim(
        &self,
        request: &ScopedRequest<'_>,
        scope: &mut crate::store::PrincipalScope<'_>,
    ) -> Result<BackendResponse, ApiFailure> {
        let (amount, currency) = money_field(&request.body, "money")?;
        let mut agent = self.principal_agent(scope)?;
        let agent_id = path(request, "agent_id")?;
        let context = agent.agent_context(agent_id).map_err(agent_failure)?;
        if context.seed.currency != currency {
            return Err(ApiFailure::invalid_request(Some("money")));
        }
        let operation_key = action_key(required_idempotency(request)?);
        let budget_state = agent
            .agent_budget_state(context.active_budget_id)
            .map_err(agent_failure)?;
        if budget_state.asset != context.seed.budget_asset || budget_state.remaining < amount {
            return Err(ApiFailure::invalid_request(Some("money")));
        }
        let trace =
            TraceId::parse(&request.trace).map_err(|_| ApiFailure::invalid_request(None))?;
        if let (Some(attestor), Some(assertion)) = (
            self.attestor_custody(),
            request
                .principal
                .as_ref()
                .and_then(PrincipalContext::assertion),
        ) {
            attestor
                .admit_assertion(&assertion_subject(assertion)?, assertion)
                .map_err(|_| ApiFailure::forbidden())?;
        }
        let receipt =
            self.submit_budget_defund(scope, &mut agent, &context, amount, operation_key, &trace)?;
        let mut evidence = ProductionAgentCreation::finalization_evidence(
            &receipt,
            ModuleId::Budget,
            8,
            self.now()?,
        )
        .map_err(|_| ApiFailure::upstream_degraded())?;
        evidence.action_key = operation_key;
        let post = agent
            .agent_budget_state(context.active_budget_id)
            .map_err(agent_failure)?;
        if post.asset != budget_state.asset
            || post.observed_head_sequence < evidence.observed_sequence
        {
            return Err(ApiFailure::upstream_degraded());
        }
        let value = agent
            .agent_reclaim(
                agent_id,
                amount,
                currency,
                budget_state.evidence_digest,
                post.evidence_digest,
                evidence,
            )
            .map_err(agent_failure)?;
        Ok(BackendResponse {
            result: managed_journey_json(&value)?,
            session: None,
        })
    }

    fn execute_agent_archive(
        &self,
        request: &ScopedRequest<'_>,
        scope: &mut crate::store::PrincipalScope<'_>,
    ) -> Result<BackendResponse, ApiFailure> {
        let mut agent = self.principal_agent(scope)?;
        let agent_id = path(request, "agent_id")?;
        let confirm_name = text_field(&request.body, "confirm_name")?;
        let context = agent.agent_context(agent_id).map_err(agent_failure)?;
        let operation_key = action_key(required_idempotency(request)?);
        let pre = agent
            .agent_budget_state(context.active_budget_id)
            .map_err(agent_failure)?;
        if pre.asset != context.seed.budget_asset {
            return Err(ApiFailure::upstream_degraded());
        }
        let trace =
            TraceId::parse(&request.trace).map_err(|_| ApiFailure::invalid_request(None))?;
        let registry = agent.registry().clone();
        if pre.remaining != 0 {
            self.archive_defund(scope, &mut agent, &context, &pre, operation_key, &trace)?;
        }
        let session = agent
            .agent_session_snapshot(agent_id)
            .map_err(agent_failure)?;
        if session.agent_did != context.agent_did {
            return Err(ApiFailure::upstream_degraded());
        }
        let effective_sequence = agent
            .head()
            .map_err(agent_failure)?
            .chain_sequence
            .checked_add(1)
            .ok_or_else(ApiFailure::upstream_degraded)?;
        let intent = Intent::v1(IntentKind::SessionRevoke(
            SessionRevoke::new(
                AuthorityGrantId::new(context.protocol_grant_id),
                SessionRevocationReason::Archived,
                ProtocolSequence::from_u64(effective_sequence),
            )
            .map_err(|_| ApiFailure::upstream_degraded())?,
        ));
        let current = self.now()?;
        let mut adapter = ProductionAgentCreation::new(
            &mut agent,
            &self.agent_contract,
            &self.custody,
            &trace,
            AgentDid::new(context.seed.actor.clone())
                .map_err(|_| ApiFailure::upstream_degraded())?,
            AuthorityRef::new(context.seed.primary_authority.clone())
                .map_err(|_| ApiFailure::upstream_degraded())?,
            super::agent_creation::CreationBounds {
                timestamp_span: self.agent_timestamp_span_seconds,
                fee_limit: self.agent_fee_limit,
            },
        )
        .map_err(|_| ApiFailure::upstream_degraded())?;
        let receipt = adapter
            .submit_lifecycle_intent(
                scope,
                &registry,
                intent,
                operation_key,
                KeyId::new(context.seed.custody_key)
                    .map_err(|_| ApiFailure::upstream_degraded())?,
                current,
            )
            .map_err(|_| ApiFailure::upstream_degraded())?;
        let evidence = ProductionAgentCreation::finalization_evidence(
            &receipt,
            ModuleId::Governance,
            6,
            self.now()?,
        )
        .map_err(|_| ApiFailure::upstream_degraded())?;
        let session_observation = agent
            .agent_session_suspend(agent_id, operation_key)
            .map_err(agent_failure)?;
        let post = agent
            .agent_budget_state(context.active_budget_id)
            .map_err(agent_failure)?;
        if post.asset != pre.asset
            || post.remaining != 0
            || post.observed_head_sequence < evidence.observed_sequence
        {
            return Err(ApiFailure::upstream_degraded());
        }
        let value = agent
            .agent_archive(
                agent_id,
                confirm_name,
                pre.evidence_digest,
                post.evidence_digest,
                session_observation.evidence_digest,
                evidence,
            )
            .map_err(agent_failure)?;
        Ok(BackendResponse {
            result: managed_journey_json(&value)?,
            session: None,
        })
    }

    fn execute_security_action(
        request: &ScopedRequest<'_>,
        scope: &mut crate::store::PrincipalScope<'_>,
    ) -> Result<BackendResponse, ApiFailure> {
        let digest = identity_dispatch::security_digest(
            scope,
            text_field(&request.body, "action")?,
            request
                .body
                .get("target_id")
                .and_then(serde_json::Value::as_str),
        )
        .map_err(|error| identity_failure(&error))?;
        Ok(BackendResponse {
            result: json!({"confirms": digest.to_schema()}),
            session: None,
        })
    }

    fn execute_stepup_begin(
        &self,
        request: &ScopedRequest<'_>,
        scope: &mut crate::store::PrincipalScope<'_>,
        principal: &crate::store::PrincipalId,
    ) -> Result<BackendResponse, ApiFailure> {
        let challenge =
            super::key_export::step_up_begin(&self.passkeys, scope, &request.body, self.now()?)?;
        self.auth_index
            .bind_step_up_at(
                &challenge.challenge_id,
                principal,
                challenge.expires_at,
                self.now()?,
            )
            .map_err(|error| auth_failure(&error))?;
        Ok(BackendResponse {
            result: super::identity::IdentityProjector::checked_step_up_challenge(&challenge)?,
            session: None,
        })
    }

    fn execute_stepup_finish(
        &self,
        request: &ScopedRequest<'_>,
        scope: &mut crate::store::PrincipalScope<'_>,
    ) -> Result<BackendResponse, ApiFailure> {
        let evidence = self
            .passkeys
            .finish_step_up(
                scope,
                path(request, "challenge_id")?,
                text_field(&request.body, "credential")?,
                self.now()?,
            )
            .map_err(|error| auth_api_failure(&error))?;
        Ok(BackendResponse {
            result: super::identity::IdentityProjector::checked_step_up(&evidence)?,
            session: None,
        })
    }

    fn execute_profile_get(
        scope: &mut crate::store::PrincipalScope<'_>,
    ) -> Result<BackendResponse, ApiFailure> {
        Ok(BackendResponse {
            result: identity_dispatch::profile(scope).map_err(|error| identity_failure(&error))?,
            session: None,
        })
    }

    fn execute_profile_update(
        &self,
        request: &ScopedRequest<'_>,
        scope: &mut crate::store::PrincipalScope<'_>,
    ) -> Result<BackendResponse, ApiFailure> {
        Ok(BackendResponse {
            result: identity_dispatch::update_profile(scope, &request.body, self.now()?)
                .map_err(|error| identity_failure(&error))?,
            session: None,
        })
    }

    fn execute_onboarding_status(
        scope: &mut crate::store::PrincipalScope<'_>,
    ) -> Result<BackendResponse, ApiFailure> {
        let journey = OnboardingJourney::load(scope)
            .map_err(|_| ApiFailure::upstream_degraded())?
            .ok_or_else(ApiFailure::not_found)?;
        Ok(BackendResponse {
            result: super::identity::IdentityProjector::onboarding(&scope, &journey.status())?,
            session: None,
        })
    }

    fn execute_onboarding_resume(
        &self,
        request: &ScopedRequest<'_>,
        principal: &crate::store::PrincipalId,
    ) -> Result<BackendResponse, ApiFailure> {
        let status = self.advance_native_onboarding(principal, &request.trace, self.now()?)?;
        let mut store = self.store.lock().map_err(|_| ApiFailure::unavailable())?;
        let scope = store
            .principal(principal)
            .map_err(|_| ApiFailure::unavailable())?;
        Ok(BackendResponse {
            result: super::identity::IdentityProjector::onboarding(&scope, &status)?,
            session: None,
        })
    }

    fn intent_observed_state(
        &self,
        scope: &mut crate::store::PrincipalScope<'_>,
        intent: &crate::journeys::UnifiedIntent,
    ) -> Result<(crate::journeys::ObservedState, AccountId, String), ApiFailure> {
        let asset = intent.asset();
        let observed_at = self.now()?;
        let mut agent = self.principal_agent(scope)?;
        let owner = resolve_principal_owner(self, scope, &mut agent)?;
        let account = owner.account;
        let balance = agent.balance().map_err(agent_failure)?;
        if balance.asset != asset.bytes() {
            return Err(ApiFailure::invalid_request(Some("asset_id")));
        }
        if balance.global_sequence != balance.observed_head_sequence
            || balance.age_seconds > self.activity_freshness_seconds
        {
            return Err(ApiFailure::upstream_degraded());
        }

        let home = crate::journeys::Endpoint::human(account.clone())
            .map_err(|_| ApiFailure::upstream_degraded())?;
        let mut ledger = vec![crate::journeys::BalanceEntry::new(
            home.clone(),
            asset,
            ProtocolAmount::from_u128(balance.amount),
        )];
        let mut allowances = Vec::new();
        let mut budgets = Vec::new();
        let managed = CreationJourney::list(scope).map_err(|_| ApiFailure::upstream_degraded())?;
        for journey in &managed {
            let alias = format!("agt_{}", hex_bytes(&journey.agent_id()));
            let context = agent.agent_context(&alias).map_err(agent_failure)?;
            if context.seed.owner_account != account.canonical()
                || context.seed.budget_asset != asset.bytes()
                || context.protocol_grant_id == [0; 32]
            {
                continue;
            }
            let state = agent
                .agent_budget_state(context.active_budget_id)
                .map_err(agent_failure)?;
            if state.asset != asset.bytes() || state.age_sequences > state.maximum_age_sequences {
                return Err(ApiFailure::upstream_degraded());
            }
            let budget_account = AccountId::parse(&context.seed.budget_account)
                .map_err(|_| ApiFailure::upstream_degraded())?;
            let budget = crate::journeys::Endpoint::agent_budget(budget_account.clone())
                .map_err(|_| ApiFailure::upstream_degraded())?;
            budgets.push(
                crate::journeys::BudgetBinding::new(budget_account, account.clone())
                    .map_err(intent_failure)?,
            );
            ledger.push(crate::journeys::BalanceEntry::new(
                budget.clone(),
                asset,
                ProtocolAmount::from_u128(state.remaining),
            ));
            let headroom = context.current_monthly_limit.saturating_sub(context.spent);
            if headroom > 0 {
                allowances.push(
                    crate::journeys::SignedAllowance::new(
                        crate::journeys::AllowanceId::new(context.active_budget_id)
                            .map_err(intent_failure)?,
                        crate::journeys::AllowanceKind::BudgetAllowance,
                        crate::journeys::AllowanceScope::new(
                            home.clone(),
                            budget,
                            asset,
                            crate::journeys::LegMechanism::Protocol(
                                crate::journeys::Mechanism::BudgetFund,
                            ),
                        ),
                        ProtocolAmount::from_u128(headroom),
                        ProtocolAmount::from_u128(headroom),
                        layerx_types::intent::TimestampSeconds::from_u64(
                            context.seed.budget_expiry_seconds,
                        ),
                    )
                    .map_err(intent_failure)?,
                );
            }
        }

        supported_intent_endpoints(intent).map_err(submit_failure)?;
        let observed = crate::journeys::ObservedState::new(
            layerx_types::intent::TimestampSeconds::from_u64(observed_at),
            account.clone(),
            None,
            ledger,
            allowances,
            budgets,
            self.intent_fee_schedule()?,
        )
        .map_err(intent_failure)?;

        Ok((observed, account, balance.currency.clone()))
    }

    fn intent_fee_schedule(&self) -> Result<crate::journeys::FeeSchedule, ApiFailure> {
        use crate::journeys::{LegMechanism, Mechanism};
        let protocol = u128::from(self.evm_gas_limit)
            .checked_mul(u128::from(self.evm_max_fee_per_gas))
            .ok_or_else(ApiFailure::upstream_degraded)?;
        crate::journeys::FeeSchedule::new(vec![
            (LegMechanism::Protocol(Mechanism::Send), protocol),
            (LegMechanism::Protocol(Mechanism::BudgetFund), protocol),
            (LegMechanism::Protocol(Mechanism::BudgetDefund), protocol),
            (
                LegMechanism::PaxeerCustodyDeposit,
                u128::from(self.evm_gas_limit).saturating_mul(u128::from(self.evm_max_fee_per_gas)),
            ),
            (
                LegMechanism::PaxeerWithdrawFinalise,
                u128::from(self.evm_gas_limit).saturating_mul(u128::from(self.evm_max_fee_per_gas)),
            ),
        ])
        .map_err(intent_failure)
    }

    fn intent_from_request(
        body: &serde_json::Value,
        asset: AssetId,
    ) -> Result<crate::journeys::UnifiedIntent, ApiFailure> {
        let source = intent_endpoint(body, "source")?;
        let destination = intent_endpoint(body, "destination")?;
        let (amount, _) = money_field(body, "money")?;
        let constraints = body
            .get("constraints")
            .ok_or_else(|| ApiFailure::invalid_request(Some("constraints")))?;
        let deadline = crate::time::seconds_from_rfc3339(text_field(constraints, "deadline")?)
            .ok_or_else(|| ApiFailure::invalid_request(Some("constraints.deadline")))?;
        let (max_fee, _) = money_field(constraints, "max_fee")?;
        let allow_top_up = constraints
            .get("allow_top_up")
            .and_then(serde_json::Value::as_bool)
            .ok_or_else(|| ApiFailure::invalid_request(Some("constraints.allow_top_up")))?;
        crate::journeys::UnifiedIntent::new(
            source,
            destination,
            asset,
            ProtocolAmount::from_u128(amount),
            crate::journeys::Constraints::new(
                layerx_types::intent::TimestampSeconds::from_u64(deadline),
                max_fee,
                allow_top_up,
            ),
        )
        .map_err(intent_failure)
    }

    fn execute_intent_plan(
        &self,
        request: &ScopedRequest<'_>,
        scope: &mut crate::store::PrincipalScope<'_>,
    ) -> Result<BackendResponse, ApiFailure> {
        let asset = intent_asset(&request.body)?;
        let intent = Self::intent_from_request(&request.body, asset)?;
        let (observed, _, currency) = self.intent_observed_state(scope, &intent)?;
        if money_field(&request.body, "money")?.1 != currency
            || money_field(
                request
                    .body
                    .get("constraints")
                    .ok_or_else(|| ApiFailure::invalid_request(Some("constraints")))?,
                "max_fee",
            )?
            .1 != currency
        {
            return Err(ApiFailure::invalid_request(Some("money.currency")));
        }
        let planned = crate::journeys::plan(&intent, &observed).map_err(intent_failure)?;
        let owner = resolve_kernel_intent_context(self, scope, asset)?;
        let transactions = self.intent_transactions(scope, &planned, &owner, &currency)?;
        let memo = HeldIntentObservation {
            profile: 1,
            principal: scope.principal().as_str().to_owned(),
            tenant: scope.tenant().as_str().to_owned(),
            session_id: request
                .principal
                .as_ref()
                .ok_or_else(ApiFailure::unauthenticated)?
                .session_id
                .clone(),
            plan_digest: planned.digest(),
            custody_binding_digest: owner.custody_binding_digest,
            identity_authority_digest: owner.identity_authority_digest,
            canonical_intent: serde_json::to_vec(&request.body)
                .map_err(|_| ApiFailure::invalid_request(None))?,
            canonical_plan: planned.canonical_encode(),
            transactions: transactions.clone(),
        };
        let key = intent_observation_key(&planned.digest(), &memo.session_id)
            .map_err(|_| ApiFailure::upstream_degraded())?;
        let bytes = serde_json::to_vec(&memo).map_err(|_| ApiFailure::unavailable())?;
        match scope.get(Table::Cache, &key) {
            Some(row) if row.bytes() == bytes.as_slice() => {}
            Some(_) => return Err(ApiFailure::forbidden()),
            None => scope
                .put(Table::Cache, key, observed.observed_at().value(), bytes)
                .map_err(|_| ApiFailure::unavailable())?,
        }
        Ok(BackendResponse {
            result: intent_plan_json(&planned, &currency, &transactions),
            session: None,
        })
    }

    fn submit_intent(
        &self,
        request: &ScopedRequest<'_>,
        scope: &mut crate::store::PrincipalScope<'_>,
    ) -> Result<BackendResponse, ApiFailure> {
        let idempotency = required_idempotency(request)?.to_owned();
        let submitted =
            crate::journeys::SubmitPlanRequest::from_json(&request.body).map_err(submit_failure)?;
        let key = intent_observation_key(
            &submitted.plan_digest,
            &request
                .principal
                .as_ref()
                .ok_or_else(ApiFailure::unauthenticated)?
                .session_id,
        )
        .map_err(|_| ApiFailure::invalid_request(Some("plan_digest")))?;
        let row = scope
            .get(Table::Cache, &key)
            .ok_or_else(ApiFailure::forbidden)?;
        let memo: HeldIntentObservation =
            serde_json::from_slice(row.bytes()).map_err(|_| ApiFailure::forbidden())?;
        if memo.profile != 1
            || memo.plan_digest != submitted.plan_digest
            || memo.principal != scope.principal().as_str()
            || memo.tenant != scope.tenant().as_str()
            || memo.session_id
                != request
                    .principal
                    .as_ref()
                    .ok_or_else(ApiFailure::unauthenticated)?
                    .session_id
        {
            return Err(ApiFailure::forbidden());
        }
        let body: serde_json::Value =
            serde_json::from_slice(&memo.canonical_intent).map_err(|_| ApiFailure::forbidden())?;
        let asset = intent_asset(&body)?;
        let intent = Self::intent_from_request(&body, asset)?;
        let context = resolve_kernel_intent_context(self, scope, asset)?;
        let outcome_key = RowKey::new(format!(
            "intent-outcome-{}",
            hex_bytes(&action_key(&idempotency))
        ))
        .map_err(|_| ApiFailure::invalid_request(None))?;
        let canonical_submission =
            serde_json::to_vec(&request.body).map_err(|_| ApiFailure::invalid_request(None))?;
        if let Some(row) = scope.get(Table::Cache, &outcome_key) {
            let outcome: HeldIntentOutcome =
                serde_json::from_slice(row.bytes()).map_err(|_| ApiFailure::forbidden())?;
            if outcome.canonical_submission != canonical_submission
                || outcome.session_id != memo.session_id
                || outcome.plan_digest != memo.plan_digest
                || context.custody_binding_digest != memo.custody_binding_digest
                || context.identity_authority_digest != memo.identity_authority_digest
            {
                return Err(ApiFailure::forbidden());
            }
            return Ok(BackendResponse {
                result: outcome.result,
                session: None,
            });
        }
        let (observed, _, currency) = self.intent_observed_state(scope, &intent)?;
        let planned = crate::journeys::plan(&intent, &observed).map_err(intent_failure)?;
        if planned.canonical_encode() != memo.canonical_plan {
            return Err(ApiFailure::forbidden());
        }
        let now = self.now()?;
        if context.custody_binding_digest != memo.custody_binding_digest
            || context.identity_authority_digest != memo.identity_authority_digest
        {
            return Err(ApiFailure::forbidden());
        }
        if self.intent_transactions(scope, &planned, &context, &currency)? != memo.transactions {
            return Err(ApiFailure::forbidden());
        }
        let expectation = crate::journeys::BindingExpectation {
            actor: context.actor.clone(),
            authority: context.authority.clone(),
            account_sequence: context.account_sequence,
            currency,
            now,
        };
        crate::journeys::verify_bindings(&planned, &submitted, &expectation)
            .map_err(submit_failure)?;
        let kernel_submission = KernelSubmission {
            planned: &planned,
            submitted: &submitted,
            expectation: &expectation,
            context: &context,
            idempotency: &idempotency,
        };
        let submission = match planned.executable_shape() {
            crate::journeys::IntentShape::Kernel => {
                self.submit_kernel_intent(request, scope, &kernel_submission)?
            }
            crate::journeys::IntentShape::CustodyDeposit
            | crate::journeys::IntentShape::DepositForward
            | crate::journeys::IntentShape::WithdrawToWallet
            | crate::journeys::IntentShape::TransferThenWithdraw => {
                return Err(submit_failure(
                    crate::journeys::SubmitRefusal::UnsupportedPlan,
                ));
            }
        };
        let result = submission.to_json();
        let outcome = HeldIntentOutcome {
            canonical_submission,
            session_id: memo.session_id,
            plan_digest: memo.plan_digest,
            result: result.clone(),
        };
        scope
            .put(
                Table::Cache,
                outcome_key,
                now,
                serde_json::to_vec(&outcome).map_err(|_| ApiFailure::unavailable())?,
            )
            .map_err(|_| ApiFailure::unavailable())?;
        Ok(BackendResponse {
            result,
            session: None,
        })
    }

    fn submit_kernel_intent(
        &self,
        request: &ScopedRequest<'_>,
        scope: &mut crate::store::PrincipalScope<'_>,
        submission: &KernelSubmission<'_>,
    ) -> Result<crate::journeys::IntentSubmission, ApiFailure> {
        let KernelSubmission {
            planned,
            submitted,
            expectation,
            context,
            idempotency,
        } = *submission;
        if request.body.get("native_access_id").is_some() {
            return Err(ApiFailure::forbidden());
        }
        let transactions =
            self.intent_transactions(scope, planned, context, &expectation.currency)?;
        let mut signed = Vec::with_capacity(transactions.len());
        for (tx, binding) in transactions.iter().zip(&submitted.bindings) {
            let authorization = crate::custody::EvmPlanAuthorization {
                plan_id: planned.digest(),
                action_key: binding.action_key,
                tenant: scope.tenant().as_str().to_owned(),
                principal: scope.principal().as_str().to_owned(),
                binding_digest: context.custody_binding_digest,
                wallet: self
                    .custody
                    .evm_wallet(scope.principal(), &context.custody_key)
                    .map_err(|_| ApiFailure::forbidden())?,
                not_before: binding.not_before,
                not_after: binding.not_after,
                transaction: tx.clone(),
            };
            let row_key = RowKey::new(format!("intent-signed-{}", hex_bytes(&binding.action_key)))
                .map_err(|_| ApiFailure::unavailable())?;
            let action = if let Some(row) = scope.get(Table::Journeys, &row_key) {
                let saved: crate::custody::EvmAction =
                    serde_json::from_slice(row.bytes()).map_err(|_| ApiFailure::unavailable())?;
                if saved.authorization != authorization {
                    return Err(ApiFailure::forbidden());
                }
                saved
            } else {
                let call = evm_call(tx);
                let disclosure =
                    bind_call(&call, authorization.wallet).map_err(|_| ApiFailure::forbidden())?;
                if let Some(layerx_crypto::disclosure::call::ConstructMethod::LedgerSend {
                    payload,
                }) = disclosure
                    .construct_method()
                    .map_err(|_| ApiFailure::forbidden())?
                {
                    let encoded = ledger_payment_call(
                        &payload,
                        &CallOptions {
                            from: authorization.wallet,
                            chain_id: call.chain_id,
                            nonce: call.nonce,
                            max_priority_fee_per_gas: call.max_priority_fee_per_gas,
                            max_fee_per_gas: call.max_fee_per_gas,
                            gas_limit: call.gas_limit,
                        },
                    )
                    .map_err(|_| ApiFailure::forbidden())?;
                    if encoded.call != call {
                        return Err(ApiFailure::forbidden());
                    }
                }
                if let (Some(attestor), Some(assertion)) = (
                    self.attestor_custody(),
                    request
                        .principal
                        .as_ref()
                        .and_then(PrincipalContext::assertion),
                ) {
                    attestor
                        .admit_assertion(&assertion_subject(assertion)?, assertion)
                        .map_err(|_| ApiFailure::forbidden())?;
                }
                let raw_transaction =
                    if let Some(layerx_crypto::disclosure::call::ConstructMethod::BankSend {
                        to,
                        denom,
                        amount,
                        ..
                    }) = disclosure
                        .construct_method()
                        .map_err(|_| ApiFailure::forbidden())?
                    {
                        self.custody
                            .creation_keystore()
                            .authorize_send_transaction(
                                scope.principal(),
                                &context.custody_key,
                                &authorization,
                                to,
                                denom,
                                amount,
                            )
                            .map_err(|_| ApiFailure::forbidden())?
                    } else {
                        let trace = TraceId::parse(&request.trace)
                            .map_err(|_| ApiFailure::invalid_request(None))?;
                        let principal = scope.principal().clone();
                        let unsigned = layerx_human_kms::attestor::unsigned_evm_transaction(&call);
                        let grant = super::executor::poll_once_ready(self.custody.sign_in_scope(
                            scope,
                            crate::custody::SignRequest::new(
                                &principal,
                                &context.custody_key,
                                &trace,
                                crate::custody::SignAuthorization::new(
                                    crate::custody::Operation::ProtocolMutation,
                                    None,
                                ),
                                &unsigned,
                                &disclosure,
                                expectation.now,
                            ),
                        ))
                        .map_err(|_| ApiFailure::upstream_degraded())?
                        .map_err(|_| ApiFailure::forbidden())?;
                        let signature =
                            layerx_crypto::signer::AgentSignature::new(*grant.signature())
                                .map_err(|_| ApiFailure::upstream_degraded())?;
                        if layerx_crypto::evm_transaction::recover_sender(&call, &signature)
                            .map_err(|_| ApiFailure::forbidden())?
                            != authorization.wallet
                        {
                            return Err(ApiFailure::forbidden());
                        }
                        assemble(&call, &signature)
                    };
                let action = crate::custody::EvmAction {
                    authorization,
                    transaction_hash: Some(transaction_hash(&raw_transaction)),
                    raw_transaction,
                    acknowledged: false,
                };
                scope
                    .put(
                        Table::Journeys,
                        row_key,
                        expectation.now,
                        serde_json::to_vec(&action).map_err(|_| ApiFailure::unavailable())?,
                    )
                    .map_err(|_| ApiFailure::unavailable())?;
                action
            };
            signed.push(action);
        }
        let journey_id = crate::journeys::intent_journey_id(idempotency, planned.digest())
            .map_err(submit_failure)?;
        let result = crate::journeys::start_kernel_journey(
            scope,
            planned,
            submitted,
            expectation,
            journey_id.clone(),
            &signed,
        )
        .map_err(submit_failure)?;
        schedule_continuation(scope, "intent-evm", &journey_id, expectation.now)?;
        self.advance_evm_intent(scope, &journey_id, expectation.now)?;
        Ok(result)
    }

    fn intent_transactions(
        &self,
        scope: &crate::store::PrincipalScope<'_>,
        plan: &crate::journeys::UnifiedPlan,
        context: &KernelIntentContext,
        currency: &str,
    ) -> Result<Vec<crate::custody::EvmTransaction>, ApiFailure> {
        let wallet = self
            .custody
            .evm_wallet(scope.principal(), &context.custody_key)
            .map_err(|_| ApiFailure::forbidden())?;
        let mut agent = self.principal_agent(scope)?;
        plan.legs()
            .iter()
            .map(|leg| {
                let nonce = context
                    .account_sequence
                    .checked_add(u64::try_from(leg.index()).map_err(|_| ApiFailure::forbidden())?)
                    .ok_or_else(ApiFailure::forbidden)?;
                let (to, calldata) = match leg.mechanism() {
                    crate::journeys::LegMechanism::Protocol(crate::journeys::Mechanism::Send) => {
                        if intent_account(leg.source())? != &context.account {
                            return Err(ApiFailure::forbidden());
                        }
                        let recipient = intent_account(leg.destination())?;
                        let did = recipient
                            .canonical()
                            .strip_prefix("agent:")
                            .and_then(|name| name.strip_suffix(":main"))
                            .ok_or_else(ApiFailure::forbidden)?;
                        let identity = agent.identity_resolve(did).map_err(agent_failure)?;
                        if identity.frozen || !matches!(identity.verification, 4 | 5) {
                            return Err(ApiFailure::forbidden());
                        }
                        let public_key = identity
                            .authorities
                            .iter()
                            .find(|(kind, _)| *kind == 1)
                            .map(|(_, key)| key)
                            .ok_or_else(ApiFailure::forbidden)?;
                        let recipient = intent_node()?
                            .bound_address(public_key)
                            .map_err(|_| ApiFailure::upstream_degraded())?
                            .ok_or_else(ApiFailure::forbidden)?;
                        let gateway = layerx_network_gateway::GatewayEndpoint::from_environment()
                            .map_err(|_| ApiFailure::upstream_degraded())?;
                        let balances = gateway
                            .get_balances(layerx_network_gateway::AccountIdentifier::Evm(wallet))
                            .map_err(|_| ApiFailure::upstream_degraded())?;
                        let assets = gateway
                            .list_assets()
                            .map_err(|_| ApiFailure::upstream_degraded())?;
                        let denom = send_leg_denom(&assets, &balances, leg.asset())?;
                        (
                            precompile(0x1001),
                            bank_send_calldata(wallet, recipient, &denom, leg.amount().value()),
                        )
                    }
                    crate::journeys::LegMechanism::Protocol(
                        crate::journeys::Mechanism::BudgetDefund,
                    ) => {
                        let budget = intent_account(leg.source())?;
                        let binding = crate::journeys::IntentLegBinding {
                            leg_index: leg.index(),
                            action_key: plan.action_key(leg.index()).map_err(intent_failure)?,
                            actor: context.actor.as_str().to_owned(),
                            authority: crate::journeys::authority_label(leg.authority()).to_owned(),
                            relationship: "self".to_owned(),
                            account_sequence: nonce,
                            not_before: self.now()?,
                            not_after: plan.intent().constraints().deadline().value(),
                            fee_limit: leg.fee(),
                            fee_currency: currency.to_owned(),
                        };
                        let route =
                            intent_budget_route(scope, &mut agent, context, leg, &binding, budget)?;
                        let crate::journeys::Relationship::ManagedBudget(route) =
                            route.relationship
                        else {
                            return Err(ApiFailure::forbidden());
                        };
                        (
                            precompile(0x101b),
                            budget_defund_calldata(route.budget_id.bytes(), leg.amount().value()),
                        )
                    }
                    _ => return Err(ApiFailure::forbidden()),
                };
                let transaction = crate::custody::EvmTransaction {
                    chain_id: u64::from(self.network_id),
                    nonce,
                    max_priority_fee_per_gas: self.evm_max_priority_fee_per_gas,
                    max_fee_per_gas: self.evm_max_fee_per_gas,
                    gas_limit: self.evm_gas_limit,
                    to,
                    value: [0; 32],
                    calldata,
                };
                bind_call(&evm_call(&transaction), wallet).map_err(|_| ApiFailure::forbidden())?;
                Ok(transaction)
            })
            .collect()
    }

    fn execute_move_quote(
        &self,
        request: &ScopedRequest<'_>,
        scope: &mut crate::store::PrincipalScope<'_>,
    ) -> Result<BackendResponse, ApiFailure> {
        let now = self.now()?;
        let (amount, currency) = money_field(&request.body, "money")?;
        let key = KeyId::new("human-primary").map_err(|_| ApiFailure::unavailable())?;
        let mut agent = self.principal_agent(scope)?;
        let owner = resolve_principal_owner(self, scope, &mut agent)?;
        let wallet = self
            .custody
            .evm_wallet(scope.principal(), &key)
            .map_err(|_| ApiFailure::forbidden())?;
        let binding = self
            .custody
            .evm_binding(scope.principal(), &key)
            .map_err(|_| ApiFailure::forbidden())?;
        let account = layerx_intents::canonical::account_id_for_protocol(
            &owner.account,
            self.protocol_version,
        )
        .map_err(|_| ApiFailure::forbidden())?;
        let source = text_field(&request.body, "source")?;
        if source != owner.account.canonical()
            && source != format!("act_{}", hex_bytes(&account))
            && decode_hex_20(source).ok() != Some(wallet)
        {
            return Err(ApiFailure::forbidden());
        }
        let balance = agent.balance().map_err(agent_failure)?;
        if balance.account != account
            || balance.global_sequence != balance.observed_head_sequence
            || balance.age_seconds > self.activity_freshness_seconds
            || balance.currency != currency
        {
            return Err(ApiFailure::forbidden());
        }
        let destination = text_field(&request.body, "destination")?;
        let recipient = if destination.starts_with("0x") {
            decode_hex_20(destination)
                .map_err(|_| ApiFailure::invalid_request(Some("destination")))?
        } else {
            let account = AccountId::parse(destination)
                .map_err(|_| ApiFailure::invalid_request(Some("destination")))?;
            let did = account
                .canonical()
                .strip_prefix("agent:")
                .and_then(|name| name.strip_suffix(":main"))
                .ok_or_else(ApiFailure::forbidden)?;
            let identity = agent.identity_resolve(did).map_err(agent_failure)?;
            if identity.frozen || !matches!(identity.verification, 4 | 5) {
                return Err(ApiFailure::forbidden());
            }
            let public_key = identity
                .authorities
                .iter()
                .find(|(kind, _)| *kind == 1)
                .map(|(_, public_key)| public_key)
                .ok_or_else(ApiFailure::forbidden)?;
            intent_node()?
                .bound_address(public_key)
                .map_err(|_| ApiFailure::upstream_degraded())?
                .ok_or_else(ApiFailure::forbidden)?
        };
        if recipient == [0; 20] || recipient == wallet {
            return Err(ApiFailure::invalid_request(Some("destination")));
        }
        let gateway = layerx_network_gateway::GatewayEndpoint::from_environment()
            .map_err(|_| ApiFailure::upstream_degraded())?;
        let balances = gateway
            .get_balances(layerx_network_gateway::AccountIdentifier::Evm(wallet))
            .map_err(|_| ApiFailure::upstream_degraded())?;
        let assets = gateway
            .list_assets()
            .map_err(|_| ApiFailure::upstream_degraded())?;
        let denom = send_leg_denom(&assets, &balances, AssetId::new(balance.asset))?;
        if balances
            .asset(&denom)
            .ok_or_else(ApiFailure::forbidden)?
            .amount
            < amount.into()
        {
            return Err(ApiFailure::forbidden());
        }
        let node = intent_node()?;
        if node.chain_id() != u64::from(self.network_id) {
            return Err(ApiFailure::forbidden());
        }
        let quote = BankMoveQuote {
            principal: scope.principal().as_str().to_owned(),
            tenant: scope.tenant().as_str().to_owned(),
            binding_digest: binding.digest(),
            identity_digest: Sha256::digest(&owner.identity.canonical_bytes).into(),
            wallet,
            recipient,
            denom: denom.clone(),
            amount,
            currency: currency.to_owned(),
            not_before: now,
            expires_at: now
                .checked_add(self.agent_timestamp_span_seconds)
                .ok_or_else(ApiFailure::unavailable)?,
            transaction: crate::custody::EvmTransaction {
                chain_id: node.chain_id(),
                nonce: node
                    .pending_nonce(&wallet)
                    .map_err(|_| ApiFailure::upstream_degraded())?,
                max_priority_fee_per_gas: self.evm_max_priority_fee_per_gas,
                max_fee_per_gas: self.evm_max_fee_per_gas,
                gas_limit: self.evm_gas_limit,
                to: precompile(0x1001),
                value: [0; 32],
                calldata: bank_send_calldata(wallet, recipient, &denom, amount),
            },
        };
        bind_call(&evm_call(&quote.transaction), wallet).map_err(|_| ApiFailure::forbidden())?;
        let bytes = serde_json::to_vec(&quote).map_err(|_| ApiFailure::unavailable())?;
        let digest: [u8; 32] = Sha256::digest(&bytes).into();
        let quote_id = format!("qte_{}", hex_bytes(&digest));
        scope
            .put(Table::Journeys, bank_move_quote_key(&quote_id)?, now, bytes)
            .map_err(|_| ApiFailure::unavailable())?;
        let ceiling = u128::from(quote.transaction.gas_limit)
            .checked_mul(u128::from(quote.transaction.max_fee_per_gas))
            .ok_or_else(ApiFailure::unavailable)?;
        Ok(BackendResponse {
            result: json!({"quote_id": quote_id, "description_copy_key": "movement.review.resolved-route",
                "mechanism": "transfer", "money": {"amount": amount.to_string(), "currency": currency},
                "fee_estimate": {"amount": ceiling.to_string(), "currency": currency},
                "fee_ceiling": {"amount": ceiling.to_string(), "currency": currency},
                "arrival_estimate": crate::time::rfc3339(now.saturating_add(5)),
                "expires_at": crate::time::rfc3339(quote.expires_at),
                "irreversibility_copy_key": "movement.review.irreversible"}),
            session: None,
        })
    }

    fn execute_move_commit(
        &self,
        request: &ScopedRequest<'_>,
        scope: &mut crate::store::PrincipalScope<'_>,
    ) -> Result<BackendResponse, ApiFailure> {
        if request.body.get("native_access_id").is_some() {
            return Err(ApiFailure::forbidden());
        }
        let now = self.now()?;
        let quote_id = text_field(&request.body, "quote_id")?;
        let row = scope
            .get(Table::Journeys, &bank_move_quote_key(quote_id)?)
            .ok_or_else(ApiFailure::forbidden)?;
        let quote: BankMoveQuote =
            serde_json::from_slice(row.bytes()).map_err(|_| ApiFailure::unavailable())?;
        let plan_id: [u8; 32] = Sha256::digest(row.bytes()).into();
        if quote_id != format!("qte_{}", hex_bytes(&plan_id))
            || quote.principal != scope.principal().as_str()
            || quote.tenant != scope.tenant().as_str()
        {
            return Err(ApiFailure::forbidden());
        }
        let key = KeyId::new("human-primary").map_err(|_| ApiFailure::unavailable())?;
        let mut agent = self.principal_agent(scope)?;
        let owner = resolve_principal_owner(self, scope, &mut agent)?;
        if self
            .custody
            .evm_binding(scope.principal(), &key)
            .map_err(|_| ApiFailure::forbidden())?
            .digest()
            != quote.binding_digest
            || self
                .custody
                .evm_wallet(scope.principal(), &key)
                .map_err(|_| ApiFailure::forbidden())?
                != quote.wallet
            || <[u8; 32]>::from(Sha256::digest(&owner.identity.canonical_bytes))
                != quote.identity_digest
        {
            return Err(ApiFailure::forbidden());
        }
        let commit_key = hex_bytes(&action_key(required_idempotency(request)?));
        let id = crate::notify::JourneyId::new(format!("jrn_{}", hex_bytes(&plan_id)))
            .map_err(|_| ApiFailure::unavailable())?;
        let journey_key = RowKey::new(format!("intent-evm-{}", id.as_str()))
            .map_err(|_| ApiFailure::unavailable())?;
        if let Some(row) = scope.get(Table::Journeys, &journey_key) {
            let record: serde_json::Value =
                serde_json::from_slice(row.bytes()).map_err(|_| ApiFailure::unavailable())?;
            if record["move_commit_key"] != commit_key || record["move_quote_id"] != quote_id {
                return Err(ApiFailure::forbidden());
            }
        } else {
            if now < quote.not_before || now > quote.expires_at {
                return Err(ApiFailure::forbidden());
            }
            let authorization = crate::custody::EvmPlanAuthorization {
                plan_id,
                action_key: action_key(&format!("move:{quote_id}:{commit_key}")),
                tenant: quote.tenant.clone(),
                principal: quote.principal.clone(),
                binding_digest: quote.binding_digest,
                wallet: quote.wallet,
                not_before: quote.not_before,
                not_after: quote.expires_at,
                transaction: quote.transaction.clone(),
            };
            let signed_key = RowKey::new(format!("move-signed-{}", hex_bytes(&plan_id)))
                .map_err(|_| ApiFailure::unavailable())?;
            let action = if let Some(row) = scope.get(Table::Journeys, &signed_key) {
                let action: crate::custody::EvmAction =
                    serde_json::from_slice(row.bytes()).map_err(|_| ApiFailure::unavailable())?;
                if action.authorization != authorization {
                    return Err(ApiFailure::forbidden());
                }
                action
            } else {
                if intent_node()?
                    .pending_nonce(&quote.wallet)
                    .map_err(|_| ApiFailure::upstream_degraded())?
                    != quote.transaction.nonce
                {
                    return Err(ApiFailure::forbidden());
                }
                if let (Some(attestor), Some(assertion)) = (
                    self.attestor_custody(),
                    request
                        .principal
                        .as_ref()
                        .and_then(PrincipalContext::assertion),
                ) {
                    attestor
                        .admit_assertion(&assertion_subject(assertion)?, assertion)
                        .map_err(|_| ApiFailure::forbidden())?;
                }
                let raw_transaction = self
                    .custody
                    .creation_keystore()
                    .authorize_send_transaction(
                        scope.principal(),
                        &key,
                        &authorization,
                        quote.recipient,
                        quote.denom.clone(),
                        abi_amount(quote.amount),
                    )
                    .map_err(|_| ApiFailure::forbidden())?;
                let action = crate::custody::EvmAction {
                    authorization,
                    transaction_hash: Some(transaction_hash(&raw_transaction)),
                    raw_transaction,
                    acknowledged: false,
                };
                scope
                    .put(
                        Table::Journeys,
                        signed_key,
                        now,
                        serde_json::to_vec(&action).map_err(|_| ApiFailure::unavailable())?,
                    )
                    .map_err(|_| ApiFailure::unavailable())?;
                action
            };
            let hash = transaction_hash(&action.raw_transaction);
            let record = json!({"journey_id": id.as_str(), "plan_digest": hex_bytes(&plan_id),
                "move_quote_id": quote_id, "move_commit_key": commit_key,
                "state": "processing", "state_copy_key": "journey.state.processing",
                "started_at": crate::time::rfc3339(now), "updated_at": crate::time::rfc3339(now),
                "legs": [{"index": 0, "nonce": quote.transaction.nonce,
                    "tx_hash": format!("0x{}", hex_bytes(&hash)),
                    "action_key": hex_bytes(&action.authorization.action_key),
                    "raw_transaction": action.raw_transaction, "submitted": false, "verified": false}]});
            scope
                .put(
                    Table::Journeys,
                    journey_key.clone(),
                    now,
                    serde_json::to_vec(&record).map_err(|_| ApiFailure::unavailable())?,
                )
                .map_err(|_| ApiFailure::unavailable())?;
        }
        schedule_continuation(scope, "intent-evm", &id, now)?;
        self.advance_evm_intent(scope, &id, now)?;
        let record: serde_json::Value = serde_json::from_slice(
            scope
                .get(Table::Journeys, &journey_key)
                .ok_or_else(ApiFailure::unavailable)?
                .bytes(),
        )
        .map_err(|_| ApiFailure::unavailable())?;
        Ok(BackendResponse {
            result: super::production_reads::intent_journey_json(&record)?,
            session: None,
        })
    }

    fn execute_security_passkey_list(
        scope: &mut crate::store::PrincipalScope<'_>,
    ) -> Result<BackendResponse, ApiFailure> {
        let passkeys =
            Passkeys::list_passkeys_authorized(scope).map_err(|error| auth_api_failure(&error))?;
        Ok(BackendResponse {
            result: super::identity::IdentityProjector::checked_passkeys(&passkeys)?,
            session: None,
        })
    }

    fn execute_security_passkey_register_begin(
        &self,
        request: &ScopedRequest<'_>,
        scope: &mut crate::store::PrincipalScope<'_>,
        principal: &crate::store::PrincipalId,
    ) -> Result<BackendResponse, ApiFailure> {
        let profile =
            identity_dispatch::profile(scope).map_err(|error| identity_failure(&error))?;
        let account =
            AccountIdentity::new(principal.as_str(), text_field(&profile, "display_name")?)
                .map_err(|error| auth_api_failure(&error))?;
        let challenge = self
            .passkeys
            .begin_registration(
                scope,
                &account,
                text_field(&request.body, "label")?,
                self.now()?,
            )
            .map_err(|error| auth_api_failure(&error))?;
        self.auth_index
            .bind_registration_at(
                &challenge.registration_id,
                principal,
                challenge.expires_at,
                self.now()?,
            )
            .map_err(|error| auth_failure(&error))?;
        Ok(BackendResponse {
            result: super::identity::IdentityProjector::checked_registration_challenge(&challenge)?,
            session: None,
        })
    }

    fn execute_security_passkey_register_finish(
        &self,
        request: &ScopedRequest<'_>,
        scope: &mut crate::store::PrincipalScope<'_>,
        principal: &crate::store::PrincipalId,
    ) -> Result<BackendResponse, ApiFailure> {
        let registration_id = path(request, "registration_id")?;
        if self
            .auth_index
            .resolve_registration(registration_id, self.now()?)
            .map_err(|error| auth_failure(&error))?
            != *principal
        {
            return Err(ApiFailure::forbidden());
        }
        let passkey = self
            .passkeys
            .finish_registration(
                scope,
                registration_id,
                text_field(&request.body, "credential")?,
                self.now()?,
            )
            .map_err(|error| auth_api_failure(&error))?;
        response(super::identity::IdentityProjector::checked_passkey(
            &passkey,
        )?)
    }

    fn execute_security_passkey_revoke(
        request: &ScopedRequest<'_>,
        scope: &mut crate::store::PrincipalScope<'_>,
    ) -> Result<BackendResponse, ApiFailure> {
        let passkeys = Passkeys::revoke_passkey_authorized(scope, path(request, "passkey_id")?)
            .map_err(|error| auth_api_failure(&error))?;
        Ok(BackendResponse {
            result: super::identity::IdentityProjector::checked_passkeys(&passkeys)?,
            session: None,
        })
    }

    fn execute_session_refresh(
        &self,
        request: &ScopedRequest<'_>,
        scope: &mut crate::store::PrincipalScope<'_>,
        principal: &crate::store::PrincipalId,
    ) -> Result<BackendResponse, ApiFailure> {
        Self::refresh_browser_session(
            &self.passkeys,
            &self.auth_index,
            request,
            scope,
            principal,
            self.now()?,
        )
    }

    pub fn refresh_browser_session(
        passkeys: &Passkeys,
        auth_index: &AuthDiscoveryIndex,
        request: &ScopedRequest<'_>,
        scope: &mut crate::store::PrincipalScope<'_>,
        principal: &crate::store::PrincipalId,
        now: u64,
    ) -> Result<BackendResponse, ApiFailure> {
        let context = request
            .principal
            .as_ref()
            .ok_or_else(ApiFailure::unauthenticated)?;
        let (refresh, csrf) = context
            .refresh_credentials()
            .ok_or_else(ApiFailure::unauthenticated)?;
        let grant = passkeys
            .refresh_authorized(scope, refresh, csrf, now)
            .map_err(|error| auth_api_failure(&error))?;
        auth_index
            .bind_session(&grant, principal)
            .map_err(|error| auth_failure(&error))?;
        crate::event_producer::enroll_session(scope, &grant, now)
            .map_err(|_| ApiFailure::unavailable())?;
        let current = Passkeys::list_sessions_authorized(scope, grant.session_id())
            .map_err(|error| auth_api_failure(&error))?
            .into_iter()
            .find(|value| value.current)
            .ok_or_else(ApiFailure::unavailable)?;
        let result = super::identity::IdentityProjector::checked_session(&current)?;
        Ok(BackendResponse {
            result,
            session: Some(SessionSecrets {
                access_token: grant.access_token().expose().to_owned(),
                refresh_token: grant.refresh_token().expose().to_owned(),
                csrf_token: grant.csrf_token().expose().to_owned(),
                access_max_age_seconds: grant.access_expires_at().saturating_sub(now),
                refresh_max_age_seconds: grant.refresh_expires_at().saturating_sub(now),
            }),
        })
    }

    fn execute_authenticator_status(
        &self,
        principal: &crate::store::PrincipalId,
    ) -> Result<BackendResponse, ApiFailure> {
        let provider = self
            .security
            .lock()
            .map_err(|_| ApiFailure::unavailable())?;
        let status = provider
            .status(principal)
            .map_err(|_| ApiFailure::upstream_degraded())?;
        Ok(BackendResponse {
            result: authenticator_status_json(&status)?,
            session: None,
        })
    }

    fn execute_authenticator_setup_begin(
        &self,
        request: &ScopedRequest<'_>,
        principal: &crate::store::PrincipalId,
    ) -> Result<BackendResponse, ApiFailure> {
        let mut provider = self
            .security
            .lock()
            .map_err(|_| ApiFailure::unavailable())?;
        let challenge = provider
            .begin_setup(principal, text_field(&request.body, "label")?, self.now()?)
            .map_err(|_| ApiFailure::upstream_degraded())?;
        Ok(BackendResponse {
            result: super::identity::IdentityProjector::authenticator_setup(&challenge)?,
            session: None,
        })
    }

    fn execute_authenticator_setup_finish(
        &self,
        request: &ScopedRequest<'_>,
        principal: &crate::store::PrincipalId,
    ) -> Result<BackendResponse, ApiFailure> {
        let mut provider = self
            .security
            .lock()
            .map_err(|_| ApiFailure::unavailable())?;
        let result = provider
            .finish_setup(
                principal,
                path(request, "setup_id")?,
                text_field(&request.body, "code")?,
                self.now()?,
            )
            .map_err(|_| ApiFailure::upstream_degraded())?;
        Ok(BackendResponse {
            result: json!({"method": authenticator_method_json(&result.method)?,
                    "backup_codes": super::identity::IdentityProjector::backup_codes(&result.backup_codes)?}),
            session: None,
        })
    }

    fn execute_authenticator_disable(
        &self,
        request: &ScopedRequest<'_>,
        principal: &crate::store::PrincipalId,
    ) -> Result<BackendResponse, ApiFailure> {
        let mut provider = self
            .security
            .lock()
            .map_err(|_| ApiFailure::unavailable())?;
        let status = provider
            .disable(principal, path(request, "authenticator_id")?, self.now()?)
            .map_err(|_| ApiFailure::upstream_degraded())?;
        Ok(BackendResponse {
            result: authenticator_status_json(&status)?,
            session: None,
        })
    }

    fn execute_authenticator_backup_rotate(
        &self,
        principal: &crate::store::PrincipalId,
    ) -> Result<BackendResponse, ApiFailure> {
        let mut provider = self
            .security
            .lock()
            .map_err(|_| ApiFailure::unavailable())?;
        let codes = provider
            .rotate_backup_codes(principal, self.now()?)
            .map_err(|_| ApiFailure::upstream_degraded())?;
        Ok(BackendResponse {
            result: super::identity::IdentityProjector::backup_codes(&codes)?,
            session: None,
        })
    }

    fn execute_security_recovery_reveal(
        &self,
        request: &ScopedRequest<'_>,
        principal: &crate::store::PrincipalId,
    ) -> Result<BackendResponse, ApiFailure> {
        let provider = self
            .security
            .lock()
            .map_err(|_| ApiFailure::unavailable())?;
        let secret = provider
            .reveal_verified_receipt(
                principal,
                text_field(&request.body, "evidence_id")?,
                self.now()?,
            )
            .map_err(|_| ApiFailure::upstream_degraded())?;
        Ok(BackendResponse {
            result: timed_secret_json(&secret)?,
            session: None,
        })
    }

    fn execute_key_export_begin(
        &self,
        request: &ScopedRequest<'_>,
        scope: &mut crate::store::PrincipalScope<'_>,
    ) -> Result<BackendResponse, ApiFailure> {
        super::key_export::begin(
            self.custody.creation_keystore(),
            &self.passkeys,
            scope,
            &request.body,
            self.now()?,
        )
    }

    fn execute_key_export_finish(
        &self,
        request: &ScopedRequest<'_>,
        scope: &mut crate::store::PrincipalScope<'_>,
    ) -> Result<BackendResponse, ApiFailure> {
        super::key_export::finish(
            &self.passkeys,
            self.custody.creation_keystore(),
            scope,
            &request.body,
            &request.trace,
            self.now()?,
        )
    }

    fn execute_session_list(
        scope: &mut crate::store::PrincipalScope<'_>,
        session_id: &str,
    ) -> Result<BackendResponse, ApiFailure> {
        let sessions = Passkeys::list_sessions_authorized(scope, session_id)
            .map_err(|error| auth_api_failure(&error))?;
        let sessions = sessions
            .iter()
            .map(super::identity::IdentityProjector::checked_session)
            .collect::<Result<Vec<_>, _>>()?;
        Ok(BackendResponse {
            result: json!({"sessions": sessions}),
            session: None,
        })
    }

    fn execute_session_revoke(
        &self,
        request: &ScopedRequest<'_>,
        scope: &mut crate::store::PrincipalScope<'_>,
    ) -> Result<BackendResponse, ApiFailure> {
        let target = path(request, "session_id")?;
        let grant = Passkeys::protocol_grant_for_session(scope, target)
            .map_err(|error| auth_api_failure(&error))?;
        self.revoke_browser_grants(scope, request, &[(target.to_owned(), grant)])?;
        let revoked = Passkeys::revoke_session_authorized(scope, target, self.now()?)
            .map_err(|error| auth_api_failure(&error))?;
        Ok(BackendResponse {
            result: super::identity::IdentityProjector::checked_revocation(&revoked)?,
            session: None,
        })
    }

    fn execute_session_revoke_all(
        &self,
        request: &ScopedRequest<'_>,
        scope: &mut crate::store::PrincipalScope<'_>,
    ) -> Result<BackendResponse, ApiFailure> {
        let grants = Passkeys::active_protocol_session_grants(scope)
            .map_err(|error| auth_api_failure(&error))?;
        self.revoke_browser_grants(scope, request, &grants)?;
        let revoked = Passkeys::revoke_all_sessions_authorized(scope, self.now()?)
            .map_err(|error| auth_api_failure(&error))?;
        Ok(BackendResponse {
            result: super::identity::IdentityProjector::checked_revocation(&revoked)?,
            session: None,
        })
    }

    fn execute_program_approval_list(
        &self,
        request: &ScopedRequest<'_>,
        scope: &mut crate::store::PrincipalScope<'_>,
    ) -> Result<BackendResponse, ApiFailure> {
        let encoded = path(request, "cursor")?;
        let cursor = if encoded == "start" {
            None
        } else {
            let value: [u8; 32] = URL_SAFE_NO_PAD
                .decode(encoded)
                .map_err(|_| ApiFailure::invalid_request(Some("cursor")))?
                .try_into()
                .map_err(|_| ApiFailure::invalid_request(Some("cursor")))?;
            if value == [0; 32] || URL_SAFE_NO_PAD.encode(value) != encoded {
                return Err(ApiFailure::invalid_request(Some("cursor")));
            }
            Some(value)
        };
        let mut agent = self.principal_agent(scope)?;
        let sequence = agent.head().map_err(agent_failure)?.chain_sequence;
        let (approvals, next_cursor) =
            program_approval_inventory(&mut agent, scope, cursor, sequence, self.now()?)?;
        Ok(BackendResponse {
            result: json!({"approvals":approvals,"next_cursor":next_cursor}),
            session: None,
        })
    }

    fn execute_program_approval_read(
        &self,
        request: &ScopedRequest<'_>,
        scope: &mut crate::store::PrincipalScope<'_>,
    ) -> Result<BackendResponse, ApiFailure> {
        let encoded = path(request, "approval_id")?;
        let id = encoded
            .strip_prefix("apr_")
            .ok_or_else(|| ApiFailure::invalid_request(Some("approval_id")))
            .and_then(super::projection::digest)?;
        let mut agent = self.principal_agent(scope)?;
        let facts = agent
            .native_program_approval_get(id)
            .map_err(agent_failure)?;
        let sequence = agent.head().map_err(agent_failure)?.chain_sequence;
        let projected = project_program_approval(&mut agent, scope, &facts, sequence, self.now()?)?;
        append_program_approval_stream(scope, &facts, &projected.summary, self.now()?)?;
        let result = match request.operation.name.as_str() {
            "approval.program.get" => projected.detail,
            "approval.program.material" => projected.material,
            "approval.program.budget" => projected
                .budget
                .ok_or_else(super::projection::hold_defective)?,
            _ => return Err(ApiFailure::not_found()),
        };
        Ok(BackendResponse {
            result,
            session: None,
        })
    }

    fn execute_program_approval_disclosure(
        &self,
        request: &ScopedRequest<'_>,
        scope: &mut crate::store::PrincipalScope<'_>,
    ) -> Result<BackendResponse, ApiFailure> {
        let encoded = path(request, "approval_id")?;
        let id = encoded
            .strip_prefix("apr_")
            .ok_or_else(|| ApiFailure::invalid_request(Some("approval_id")))
            .and_then(super::projection::digest)?;
        let held_digest = super::projection::digest(text_field(&request.body, "held_digest")?)?;
        let decision = text_field(&request.body, "decision")?;
        if !matches!(decision, "approve" | "reject") {
            return Err(ApiFailure::invalid_request(Some("decision")));
        }
        let idempotency = text_field(&request.body, "idempotency_key")?;
        if idempotency.is_empty()
            || idempotency.len() > 128
            || idempotency.bytes().any(|byte| byte.is_ascii_control())
        {
            return Err(ApiFailure::invalid_request(Some("idempotency_key")));
        }
        let mut agent = self.principal_agent(scope)?;
        let facts = agent
            .native_program_approval_get(id)
            .map_err(agent_failure)?;
        if facts.held_digest != held_digest {
            return Err(super::projection::hold_defective());
        }
        if facts.state == super::agent_runtime::NativeApprovalFactState::Expired
            || self
                .now()?
                .checked_mul(1000)
                .ok_or_else(ApiFailure::unavailable)?
                >= facts.activity_expires_at_unix_milliseconds
        {
            return Err(program_hold_expired());
        }
        if facts.state != super::agent_runtime::NativeApprovalFactState::Awaiting {
            return Err(super::projection::hold_defective());
        }
        let sequence = agent.head().map_err(agent_failure)?.chain_sequence;
        let projected = project_program_approval(&mut agent, scope, &facts, sequence, self.now()?)?;
        let schema = super::schema::ApiSchema::v1().map_err(|_| ApiFailure::upstream_degraded())?;
        let operation_name = format!("approval.program.{decision}");
        let operation = schema
            .operation(&operation_name)
            .ok_or_else(ApiFailure::upstream_degraded)?;
        let destination = format!("/v1/program-approvals/{encoded}/{decision}");
        let digest = super::production_auth::step_up_digest(
            scope.principal(),
            scope.tenant(),
            operation,
            &destination,
            &request.path_parameters,
            &json!({"held_digest":text_field(&request.body,"held_digest")?}),
            Some(idempotency),
        )
        .map_err(|_| ApiFailure::upstream_degraded())?;
        Ok(BackendResponse {
            result: json!({"approval_id":encoded,"held_digest":super::projection::hex(&held_digest),
            "decision":decision,"confirms":digest.to_schema(),
            "evidence":projected.material["evidence"]}),
            session: None,
        })
    }

    fn execute_program_approval_decide(
        &self,
        request: &ScopedRequest<'_>,
        scope: &mut crate::store::PrincipalScope<'_>,
    ) -> Result<BackendResponse, ApiFailure> {
        let encoded = path(request, "approval_id")?;
        let id = encoded
            .strip_prefix("apr_")
            .ok_or_else(|| ApiFailure::invalid_request(Some("approval_id")))
            .and_then(super::projection::digest)?;
        let held_digest = super::projection::digest(text_field(&request.body, "held_digest")?)?;
        let mut agent = self.principal_agent(scope)?;
        let facts = agent
            .native_program_approval_get(id)
            .map_err(agent_failure)?;
        if facts.held_digest != held_digest {
            return Err(super::projection::hold_defective());
        }
        let grant = request.operation.name == "approval.program.approve";
        let state = super::projection::program_state(facts.state)?;
        if state == "expired" {
            return Err(program_hold_expired());
        }
        if state == "defective" || state == "not-required" {
            return Err(super::projection::hold_defective());
        }
        if (state == "approved" && !grant) || (state == "rejected" && grant) {
            return Err(ApiFailure {
                status: 409,
                code: "already-decided".to_owned(),
                copy_key: "error.approval.already-decided".to_owned(),
                retry: "final".to_owned(),
                retry_after_ms: None,
                field: None,
            });
        }
        let sequence = agent.head().map_err(agent_failure)?.chain_sequence;
        project_program_approval(&mut agent, scope, &facts, sequence, self.now()?)?;
        let idempotency = super::projection::hex(&action_key(required_idempotency(request)?));
        let decided = agent
            .native_program_approval_decide(id, held_digest, &idempotency, grant, sequence)
            .map_err(agent_failure)?;
        if super::projection::program_state(decided.state)? == "expired" {
            return Err(program_hold_expired());
        }
        let projected =
            project_program_approval(&mut agent, scope, &decided, sequence, self.now()?)?;
        append_program_approval_stream(scope, &decided, &projected.summary, self.now()?)?;
        let evidence = projected.material["evidence"]
            .as_array()
            .ok_or_else(ApiFailure::upstream_degraded)?;
        Ok(BackendResponse {
            result: super::projection::program_decision(&decided, evidence)?,
            session: None,
        })
    }

    fn execute_approval_list(
        &self,
        scope: &mut crate::store::PrincipalScope<'_>,
    ) -> Result<BackendResponse, ApiFailure> {
        let mut agent = self.principal_agent(scope)?;
        let sequence = agent.head().map_err(agent_failure)?.chain_sequence;
        let (approvals, cursor) = approval_inventory(&mut agent, scope, sequence, self.now()?)?;
        Ok(BackendResponse {
            result: json!({"approvals":approvals,"next_cursor":cursor}),
            session: None,
        })
    }

    fn execute_approval_get(
        &self,
        request: &ScopedRequest<'_>,
        scope: &mut crate::store::PrincipalScope<'_>,
    ) -> Result<BackendResponse, ApiFailure> {
        let approval_id = decode_id(path(request, "approval_id")?)
            .map_err(|()| ApiFailure::invalid_request(Some("approval_id")))?;
        let mut agent = self.principal_agent(scope)?;
        let sequence = agent.head().map_err(agent_failure)?.chain_sequence;
        match agent.native_effect_approval_get_facts(approval_id) {
            Ok(facts) => {
                let value =
                    project_native_approval(&mut agent, scope, &facts, sequence, self.now()?)?;
                append_native_approval_stream(scope, &facts, &value.summary, self.now()?)?;
                return Ok(BackendResponse {
                    result: value.detail,
                    session: None,
                });
            }
            Err(crate::journeys::AgentBoundaryError::Refused) => {}
            Err(error) => return Err(agent_failure(error)),
        }
        let facts = agent
            .approval_get_facts(approval_id, sequence)
            .map_err(agent_failure)?;
        let value = project_approval(&mut agent, &facts, sequence)?;
        append_approval_stream(scope, &facts.approval, &value.summary, self.now()?)?;
        Ok(BackendResponse {
            result: value.detail,
            session: None,
        })
    }

    fn execute_approval_approve(
        &self,
        request: &ScopedRequest<'_>,
        scope: &mut crate::store::PrincipalScope<'_>,
    ) -> Result<BackendResponse, ApiFailure> {
        let approval_id = decode_id(path(request, "approval_id")?)
            .map_err(|()| ApiFailure::invalid_request(Some("approval_id")))?;
        let mut agent = self.principal_agent(scope)?;
        let sequence = agent.head().map_err(agent_failure)?.chain_sequence;
        match agent.native_effect_approval_get_facts(approval_id) {
            Ok(facts) => {
                if super::projection::native_state(facts.state)? == "expired" {
                    return Err(ApiFailure {
                        status: 409,
                        code: "hold-expired".to_owned(),
                        copy_key: "error.approval.hold-expired".to_owned(),
                        retry: "final".to_owned(),
                        retry_after_ms: None,
                        field: None,
                    });
                }
                let current = super::projection::native_state(facts.state)?;
                let grant = request.operation.name == "approval.approve";
                if (current == "approved" && !grant) || (current == "rejected" && grant) {
                    return Err(ApiFailure {
                        status: 409,
                        code: "already-decided".to_owned(),
                        copy_key: "error.approval.already-decided".to_owned(),
                        retry: "final".to_owned(),
                        retry_after_ms: None,
                        field: None,
                    });
                }
                project_native_approval(&mut agent, scope, &facts, sequence, self.now()?)?;
                let decided = agent
                    .native_effect_approval_decide(
                        approval_id,
                        facts.held_digest,
                        required_idempotency(request)?,
                        grant,
                        sequence,
                    )
                    .map_err(agent_failure)?;
                let state = super::projection::native_state(decided.state)?;
                if state == "expired" {
                    return Err(ApiFailure {
                        status: 409,
                        code: "hold-expired".to_owned(),
                        copy_key: "error.approval.hold-expired".to_owned(),
                        retry: "final".to_owned(),
                        retry_after_ms: None,
                        field: None,
                    });
                }
                let projected =
                    project_native_approval(&mut agent, scope, &decided, sequence, self.now()?)?;
                append_native_approval_stream(scope, &decided, &projected.summary, self.now()?)?;
                let evidence = projected.detail["evidence"]
                    .as_array()
                    .ok_or_else(ApiFailure::upstream_degraded)?
                    .iter()
                    .filter(|reference| {
                        reference["verification"] == "unverified"
                            && matches!(
                                reference["class"].as_str(),
                                Some("approval-hold" | "local-journey-state")
                            )
                    })
                    .cloned()
                    .collect::<Vec<_>>();
                return Ok(BackendResponse {
                    result: json!({"approval_id":format!("apr_{}",super::projection::hex(&approval_id)),"state":state,
                    "state_copy_key":format!("approval.state.{state}"),"money_moved":false,"moved_copy_key":"approval.decision.no-money-moved","evidence":evidence}),
                    session: None,
                });
            }
            Err(crate::journeys::AgentBoundaryError::Refused) => {}
            Err(error) => return Err(agent_failure(error)),
        }
        let facts = agent
            .approval_get_facts(approval_id, sequence)
            .map_err(agent_failure)?;
        let mut projected = if matches!(facts.approval.state, AgentApprovalState::AwaitingApproval)
        {
            Some(project_approval(&mut agent, &facts, sequence)?)
        } else {
            None
        };
        let decision = agent
            .approval_decide(
                request.operation.name == "approval.approve",
                approval_id,
                facts.approval.canonical_bytes_digest,
                required_idempotency(request)?,
                sequence,
            )
            .map_err(agent_failure)?;
        if decision.resolution == crate::approvals::AgentDecisionResolution::AlreadyDecided {
            return Err(ApiFailure {
                status: 409,
                code: "already-decided".to_owned(),
                copy_key: "error.approval.already-decided".to_owned(),
                retry: "final".to_owned(),
                retry_after_ms: None,
                field: None,
            });
        }
        let state = match decision.status {
            AgentDecisionStatus::Approved { .. } => "approved",
            AgentDecisionStatus::Rejected => "rejected",
            AgentDecisionStatus::Expired => {
                return Err(ApiFailure {
                    status: 409,
                    code: "hold-expired".to_owned(),
                    copy_key: "error.approval.hold-expired".to_owned(),
                    retry: "final".to_owned(),
                    retry_after_ms: None,
                    field: None,
                })
            }
            AgentDecisionStatus::Defective => return Err(super::projection::hold_defective()),
        };
        let evidence = if let Some(projected) = &projected {
            projected.detail["evidence"].clone()
        } else {
            let mut managed =
                managed_for_actor(&mut agent, facts.approval.held_activity.actor.as_str())?;
            hydrate_managed_evidence(&mut agent, &mut managed)?;
            json!(managed
                .evidence
                .iter()
                .map(super::projection::managed_evidence)
                .collect::<Result<Vec<_>, _>>()?)
        };
        let value = json!({"approval_id":format!("apr_{}",super::projection::hex(&approval_id)),"state":state,"state_copy_key":format!("approval.state.{state}"),"money_moved":false,"moved_copy_key":"approval.decision.no-money-moved","evidence":evidence});
        if let Some(projected) = &mut projected {
            projected.summary["state"] = json!(state);
            super::stream_journal::StreamJournal::append(
                scope,
                &format!("approval:{}:{state}", super::projection::hex(&approval_id)),
                match state {
                    "approved" => "approval-approved",
                    "rejected" => "approval-rejected",
                    _ => "approval-expired",
                },
                self.now()?,
                json!({"approval":projected.summary}),
            )?;
        }
        Ok(BackendResponse {
            result: value,
            session: None,
        })
    }

    fn execute_support_list(
        scope: &mut crate::store::PrincipalScope<'_>,
    ) -> Result<BackendResponse, ApiFailure> {
        let conversations = SupportService::list(scope).map_err(|error| support_failure(&error))?;
        Ok(BackendResponse {
            result: json!({"conversations": conversations}),
            session: None,
        })
    }

    fn execute_support_create(
        &self,
        request: &ScopedRequest<'_>,
        scope: &mut crate::store::PrincipalScope<'_>,
    ) -> Result<BackendResponse, ApiFailure> {
        let mut create = CreateConversation::new(
            text_field(&request.body, "body")?,
            decode_field::<Shell>(&request.body, "shell")?,
        )
        .map_err(|_| ApiFailure::invalid_request(Some("body")))?;
        if let Some(topic) = optional_decode_field::<Topic>(&request.body, "topic")? {
            create = create.with_topic(topic);
        }
        if let Some(trace) = request
            .body
            .get("trace_id")
            .and_then(|value| value.as_str())
        {
            create = create.with_trace(
                TraceId::parse(trace).map_err(|_| ApiFailure::invalid_request(Some("trace_id")))?,
            );
        }
        let value =
            SupportService::create(scope, self.now()?, required_idempotency(request)?, &create)
                .map_err(|error| support_failure(&error))?;
        response(value)
    }

    fn execute_support_reply(
        &self,
        request: &ScopedRequest<'_>,
        scope: &mut crate::store::PrincipalScope<'_>,
    ) -> Result<BackendResponse, ApiFailure> {
        let value = SupportService::reply(
            scope,
            self.now()?,
            path(request, "conversation_id")?,
            required_idempotency(request)?,
            text_field(&request.body, "body")?,
        )
        .map_err(|error| support_failure(&error))?;
        response(value)
    }

    fn execute_support_read(
        &self,
        request: &ScopedRequest<'_>,
        scope: &mut crate::store::PrincipalScope<'_>,
    ) -> Result<BackendResponse, ApiFailure> {
        let conversation = SupportService::mark_read(
            scope,
            self.now()?,
            path(request, "conversation_id")?,
            text_field(&request.body, "through_message_id")?,
        )
        .map_err(|error| support_failure(&error))?;
        Ok(BackendResponse {
            result: json!({
                "conversation_id": conversation.conversation_id(), "state": conversation.state(),
                "unread_count": conversation.unread_count(), "updated_at": conversation.updated_at()
            }),
            session: None,
        })
    }

    fn execute_support_status(
        request: &ScopedRequest<'_>,
        scope: &mut crate::store::PrincipalScope<'_>,
    ) -> Result<BackendResponse, ApiFailure> {
        let conversation_id = path(request, "conversation_id")?;
        let (state, unread_count, updated_at) = SupportService::status(scope, conversation_id)
            .map_err(|error| support_failure(&error))?;
        Ok(BackendResponse {
            result: json!({"conversation_id": conversation_id, "state": state,
                    "unread_count": unread_count, "updated_at": updated_at}),
            session: None,
        })
    }

    fn execute_support_feedback(
        &self,
        request: &ScopedRequest<'_>,
        scope: &mut crate::store::PrincipalScope<'_>,
    ) -> Result<BackendResponse, ApiFailure> {
        let helpful = request
            .body
            .get("helpful")
            .and_then(serde_json::Value::as_bool)
            .ok_or_else(|| ApiFailure::invalid_request(Some("helpful")))?;
        let value = SupportService::feedback(
            scope,
            self.now()?,
            path(request, "conversation_id")?,
            text_field(&request.body, "message_id")?,
            helpful,
        )
        .map_err(|error| support_failure(&error))?;
        response(value)
    }

    fn execute_notification_list(
        &self,
        scope: &mut crate::store::PrincipalScope<'_>,
    ) -> Result<BackendResponse, ApiFailure> {
        let inventory =
            DeepLinks::inventory(scope, self.now()?).map_err(|error| notify_failure(&error))?;
        let groups = inventory
            .groups()
            .iter()
            .map(|group| {
                let notifications = group
                    .notifications()
                    .iter()
                    .map(notification_json)
                    .collect::<Result<Vec<_>, ApiFailure>>()?;
                Ok(json!({"recency": group.recency().as_str(), "notifications": notifications}))
            })
            .collect::<Result<Vec<_>, ApiFailure>>()?;
        Ok(BackendResponse {
            result: json!({"groups": groups, "next_cursor": "cur_end", "unread_count": inventory.unread_count()}),
            session: None,
        })
    }

    fn execute_notification_read(
        &self,
        request: &ScopedRequest<'_>,
        scope: &mut crate::store::PrincipalScope<'_>,
    ) -> Result<BackendResponse, ApiFailure> {
        let id = NotificationId::new(path(request, "notification_id")?)
            .map_err(|error| notify_failure(&error))?;
        let summary = DeepLinks::mark_read(scope, self.now()?, &id)
            .map_err(|error| notify_failure(&error))?;
        let value = notification_json(&summary)?;
        super::stream_journal::StreamJournal::append(
            scope,
            &format!("notification-read:{}", summary.notification_id().as_str()),
            "notification",
            summary.created_at(),
            json!({"notification": value.clone()}),
        )?;
        Ok(BackendResponse {
            result: value,
            session: None,
        })
    }

    fn execute_notification_preferences_get(
        scope: &mut crate::store::PrincipalScope<'_>,
    ) -> Result<BackendResponse, ApiFailure> {
        let preferences = Dispatcher::preferences(scope).map_err(|error| notify_failure(&error))?;
        Ok(BackendResponse {
            result: preferences_json(&preferences),
            session: None,
        })
    }

    fn execute_notification_preferences_set(
        &self,
        request: &ScopedRequest<'_>,
        scope: &mut crate::store::PrincipalScope<'_>,
    ) -> Result<BackendResponse, ApiFailure> {
        let preferences = parse_preferences(&request.body)?;
        Dispatcher::update_preferences(scope, self.now()?, &preferences)
            .map_err(|error| notify_failure(&error))?;
        Ok(BackendResponse {
            result: preferences_json(&preferences),
            session: None,
        })
    }

    fn execute_account_balance(
        &self,
        scope: &mut crate::store::PrincipalScope<'_>,
    ) -> Result<BackendResponse, ApiFailure> {
        let result = self.execute_account_balance_fresh(scope);
        self.verified_read_response(scope, "account-balance", false, result)
    }

    fn execute_home_summary(
        &self,
        scope: &mut crate::store::PrincipalScope<'_>,
    ) -> Result<BackendResponse, ApiFailure> {
        let result = self.execute_home_summary_fresh(scope);
        self.verified_read_response(scope, "home-summary", true, result)
    }

    fn verified_read_response(
        &self,
        scope: &mut crate::store::PrincipalScope<'_>,
        name: &str,
        home: bool,
        response: Result<BackendResponse, ApiFailure>,
    ) -> Result<BackendResponse, ApiFailure> {
        let key =
            RowKey::new(format!("verified-read-{name}")).map_err(|_| ApiFailure::unavailable())?;
        let now = self.now()?;
        match response {
            Ok(response) => {
                let schema = ApiSchema::v1().map_err(|_| ApiFailure::upstream_degraded())?;
                let operation = schema
                    .operation(if home {
                        "home.summary"
                    } else {
                        "account.balance"
                    })
                    .ok_or_else(ApiFailure::upstream_degraded)?;
                schema
                    .encode_response(operation, &response.result)
                    .map_err(|_| ApiFailure::upstream_degraded())?;
                let bytes = serde_json::to_vec(&response.result)
                    .map_err(|_| ApiFailure::upstream_degraded())?;
                let mut rows = vec![(Table::Cache, key, bytes)];
                if home {
                    let balance = response
                        .result
                        .get("balance")
                        .ok_or_else(ApiFailure::upstream_degraded)?;
                    let key = RowKey::new("verified-read-account-balance")
                        .map_err(|_| ApiFailure::unavailable())?;
                    let bytes =
                        serde_json::to_vec(balance).map_err(|_| ApiFailure::upstream_degraded())?;
                    rows.push((Table::Cache, key, bytes));
                }
                scope
                    .put_batch(now, rows)
                    .map_err(|_| ApiFailure::unavailable())?;
                Ok(response)
            }
            Err(failure) if failure.code == "upstream-degraded" => {
                let row = scope.get(Table::Cache, &key).ok_or(failure)?;
                let elapsed = now
                    .checked_sub(row.written_at())
                    .ok_or_else(ApiFailure::upstream_degraded)?;
                let mut result: serde_json::Value = serde_json::from_slice(row.bytes())
                    .map_err(|_| ApiFailure::upstream_degraded())?;
                let balance = if home {
                    result
                        .get_mut("balance")
                        .ok_or_else(ApiFailure::upstream_degraded)?
                } else {
                    &mut result
                };
                let evidence = balance
                    .get("evidence")
                    .and_then(serde_json::Value::as_array)
                    .ok_or_else(ApiFailure::upstream_degraded)?;
                if evidence.is_empty() {
                    return Err(ApiFailure::upstream_degraded());
                }
                for reference in evidence {
                    let id = reference
                        .get("evidence_id")
                        .and_then(serde_json::Value::as_str)
                        .and_then(|id| id.strip_prefix("evd_"))
                        .ok_or_else(ApiFailure::upstream_degraded)?;
                    let proof_key = RowKey::new(format!("state-proof-{id}"))
                        .map_err(|_| ApiFailure::upstream_degraded())?;
                    let proof = scope
                        .get(Table::Cache, &proof_key)
                        .ok_or_else(ApiFailure::upstream_degraded)?;
                    let digest: [u8; 32] = Sha256::digest(proof.bytes()).into();
                    let rung = balance
                        .get("rung")
                        .map(|name| {
                            name.as_str()
                                .and_then(Rung::from_name)
                                .ok_or_else(ApiFailure::upstream_degraded)
                        })
                        .transpose()?;
                    if let Some(rung) = rung {
                        if balance_rung(
                            *proof
                                .bytes()
                                .get(5)
                                .ok_or_else(ApiFailure::upstream_degraded)?,
                        )? != rung
                        {
                            return Err(ApiFailure::upstream_degraded());
                        }
                    } else if proof.bytes().get(5..9)
                        != balance
                            .pointer("/freshness/observed_at")
                            .and_then(serde_json::Value::as_str)
                            .and_then(|text| u32::try_from(text.len()).ok())
                            .map(u32::to_be_bytes)
                            .as_ref()
                            .map(|bytes| bytes.as_slice())
                    {
                        return Err(ApiFailure::upstream_degraded());
                    }
                    let level = rung
                        .map(super::HumanComponentServer::verification_level_name)
                        .unwrap_or("unverified");
                    if proof.bytes().get(..5) != Some(b"LXHB1")
                        || hex_bytes(&digest) != id
                        || balance
                            .get("verification")
                            .and_then(serde_json::Value::as_str)
                            != Some(level)
                        || reference
                            .get("verification")
                            .and_then(serde_json::Value::as_str)
                            != Some(level)
                    {
                        return Err(ApiFailure::upstream_degraded());
                    }
                }
                let freshness = balance
                    .get_mut("freshness")
                    .and_then(serde_json::Value::as_object_mut)
                    .ok_or_else(ApiFailure::upstream_degraded)?;
                let age = freshness
                    .get("age_seconds")
                    .and_then(serde_json::Value::as_u64)
                    .and_then(|age| age.checked_add(elapsed))
                    .ok_or_else(ApiFailure::upstream_degraded)?;
                freshness.insert("age_seconds".to_owned(), json!(age));
                freshness.insert("within_bound".to_owned(), json!(false));
                Ok(BackendResponse {
                    result,
                    session: None,
                })
            }
            Err(failure) => Err(failure),
        }
    }

    fn execute_home_summary_fresh(
        &self,
        scope: &mut crate::store::PrincipalScope<'_>,
    ) -> Result<BackendResponse, ApiFailure> {
        let mut agent = self.principal_agent(scope)?;
        Ok(BackendResponse {
            result: AccountBalance::read_native_home_summary(
                &mut agent,
                scope,
                self.now()?,
                self.activity_freshness_seconds,
            )?,
            session: None,
        })
    }

    fn execute_account_balance_fresh(
        &self,
        scope: &mut crate::store::PrincipalScope<'_>,
    ) -> Result<BackendResponse, ApiFailure> {
        let mut agent = self.principal_agent(scope)?;
        Ok(BackendResponse {
            result: AccountBalance::read(
                &mut agent,
                scope,
                self.now()?,
                self.activity_freshness_seconds,
            )?,
            session: None,
        })
    }

    fn execute_stream_open(
        &self,
        scope: &mut crate::store::PrincipalScope<'_>,
    ) -> Result<BackendResponse, ApiFailure> {
        Ok(BackendResponse {
            result: self.stream.open(scope)?,
            session: None,
        })
    }

    fn execute_stream_next(
        &self,
        request: &ScopedRequest<'_>,
        scope: &mut crate::store::PrincipalScope<'_>,
    ) -> Result<BackendResponse, ApiFailure> {
        Ok(BackendResponse {
            result: self.stream.next(scope, path(request, "cursor")?)?,
            session: None,
        })
    }

    fn execute_activity_export_statement(
        &self,
        request: &ScopedRequest<'_>,
        scope: &mut crate::store::PrincipalScope<'_>,
    ) -> Result<BackendResponse, ApiFailure> {
        let filters = activity_filters(&request.body)?;
        let head = self
            .principal_agent(scope)?
            .head()
            .map_err(|_| ApiFailure::upstream_degraded())?;
        let statement = EvidenceExport::new(self.feed, self.activity_export_maximum_bytes)
            .map_err(activity_export_failure)?
            .statement(scope, &filters, self.now()?, head.chain_sequence)
            .map_err(activity_export_failure)?;
        Ok(BackendResponse {
            result: cache_activity_export(
                scope,
                statement.content().to_vec(),
                false,
                Vec::new(),
                self.now()?,
            )?,
            session: None,
        })
    }

    fn execute_activity_export_evidence(
        &self,
        request: &ScopedRequest<'_>,
        scope: &mut crate::store::PrincipalScope<'_>,
    ) -> Result<BackendResponse, ApiFailure> {
        let filters = activity_filters(&request.body)?;
        let ids = request
            .body
            .get("entry_ids")
            .and_then(serde_json::Value::as_array)
            .ok_or_else(|| ApiFailure::invalid_request(Some("entry_ids")))?
            .iter()
            .map(|value| {
                ActivityEntryId::new(
                    value
                        .as_str()
                        .ok_or_else(|| ApiFailure::invalid_request(Some("entry_ids")))?
                        .to_owned(),
                )
                .map_err(|_| ApiFailure::invalid_request(Some("entry_ids")))
            })
            .collect::<Result<Vec<_>, _>>()?;
        let mut exports = Vec::new();
        for id in &ids {
            let entry = Feed::entry(scope, id)
                .map_err(|error| activity_feed_failure(&error))?
                .ok_or_else(ApiFailure::not_found)?;
            for receipt in entry.receipts() {
                let canonical = receipt
                    .canonical()
                    .ok_or_else(ApiFailure::upstream_degraded)?
                    .to_vec();
                let digest: [u8; 32] = sha2::Sha256::digest(&canonical).into();
                if hex_bytes(&digest) != receipt.reference() {
                    return Err(ApiFailure::upstream_degraded());
                }
                let fact = receipt
                    .export_fact(id)
                    .map_err(|_| ApiFailure::upstream_degraded())?;
                exports.push(ReceiptExport {
                    receipts: vec![fact],
                    inclusions: Vec::new(),
                    derived_aggregates: Vec::new(),
                });
            }
        }
        let node_configuration = layerx_agentd::node_rpc::NodeRpcConfig::from_environment()
            .map_err(|_| ApiFailure::upstream_degraded())?;
        let receipt_keys = layerx_agentd::node_rpc::NodeRpc::connect(&node_configuration)
            .and_then(|node| node.receipt_keys())
            .map_err(|_| ApiFailure::upstream_degraded())?;
        let head = self
            .principal_agent(scope)?
            .head()
            .map_err(|_| ApiFailure::upstream_degraded())?;
        let (bundle, _) = EvidenceExport::new(self.feed, self.activity_export_maximum_bytes)
            .map_err(activity_export_failure)?
            .evidence(
                scope,
                &filters,
                &ids,
                exports,
                &receipt_keys,
                self.now()?,
                head.chain_sequence,
            )
            .map_err(activity_export_failure)?;
        let bytes = bundle.encode().map_err(activity_export_failure)?;
        let digest: [u8; 32] = sha2::Sha256::digest(&bytes).into();
        let verification = super::production_reads::verification_label(&verification_status(
            bundle.verify(digest, scope.principal(), &receipt_keys),
        ))?;
        let evidence = bundle.entries().iter().flat_map(|entry|entry.receipt_references().iter())
            .map(|reference|json!({"evidence_id":format!("evd_{reference}"),"class":"layerx-receipt","verification":verification})).collect();
        Ok(BackendResponse {
            result: cache_activity_export(scope, bytes, true, evidence, self.now()?)?,
            session: None,
        })
    }

    fn execute_activity_entry(
        &self,
        request: &ScopedRequest<'_>,
        scope: &mut crate::store::PrincipalScope<'_>,
    ) -> Result<BackendResponse, ApiFailure> {
        super::production_reads::activity_entry(scope, request)
    }

    fn execute_activity_query(
        &self,
        request: &ScopedRequest<'_>,
        scope: &mut crate::store::PrincipalScope<'_>,
    ) -> Result<BackendResponse, ApiFailure> {
        let filters = activity_filters(&request.body)?;
        let limit = request
            .body
            .get("page_limit")
            .and_then(serde_json::Value::as_u64)
            .map_or(Ok(50usize), |value| {
                usize::try_from(value).map_err(|_| ApiFailure::invalid_request(Some("page_limit")))
            })?;
        let mut page_request = PageRequest::new(limit, filters);
        if let Some(cursor) = request
            .body
            .get("cursor")
            .and_then(serde_json::Value::as_str)
        {
            page_request = page_request
                .after(FeedCursor::parse(cursor).map_err(|error| activity_feed_failure(&error))?);
        }
        let head = self
            .principal_agent(scope)?
            .head()
            .map_err(|_| ApiFailure::upstream_degraded())?;
        let page = self
            .feed
            .page(scope, page_request, self.now()?, head.chain_sequence)
            .map_err(|error| activity_feed_failure(&error))?;
        let currency = self
            .principal_agent(scope)?
            .native_fee_policy()
            .map_err(agent_failure)?
            .currency;
        Ok(BackendResponse {
            result: activity_page_json(&page, &currency)?,
            session: None,
        })
    }

    fn execute_read_projection(
        &self,
        request: &ScopedRequest<'_>,
        scope: &mut crate::store::PrincipalScope<'_>,
    ) -> Result<BackendResponse, ApiFailure> {
        if matches!(
            request.operation.name.as_str(),
            "journey.list" | "journey.page"
        ) {
            let mut additional = Vec::new();
            let creations =
                CreationJourney::list(scope).map_err(|error| agent_creation_failure(&error))?;
            if !creations.is_empty() {
                let mut agent = self.principal_agent(scope)?;
                for journey in creations {
                    let key = RowKey::new(format!(
                        "agent-create-{}",
                        &hex_bytes(&journey.agent_id())[..32]
                    ))
                    .map_err(|_| ApiFailure::upstream_degraded())?;
                    let updated_at = scope
                        .get(Table::Journeys, &key)
                        .ok_or_else(ApiFailure::upstream_degraded)?
                        .written_at();
                    additional.push((
                        updated_at,
                        agent_creation_json(scope, &mut agent, &journey, &journey.status())?,
                    ));
                }
            }
            let cursor = if request.operation.name == "journey.page" {
                Some(path(request, "cursor")?)
            } else {
                None
            };
            return super::production_reads::journey_list_with(scope, cursor, additional);
        }
        if request.operation.name == "journey.get" {
            let requested = path(request, "journey_id")?;
            let evm_key = RowKey::new(format!("intent-evm-{requested}"))
                .map_err(|_| ApiFailure::invalid_request(None))?;
            if scope.get(Table::Journeys, &evm_key).is_some() {
                let id = crate::notify::JourneyId::new(requested.to_owned())
                    .map_err(|_| ApiFailure::invalid_request(None))?;
                self.advance_evm_intent(scope, &id, self.now()?)?;
                let row = scope
                    .get(Table::Journeys, &evm_key)
                    .ok_or_else(ApiFailure::unavailable)?;
                let record: serde_json::Value =
                    serde_json::from_slice(row.bytes()).map_err(|_| ApiFailure::unavailable())?;
                return Ok(BackendResponse {
                    result: super::production_reads::intent_journey_json(&record)?,
                    session: None,
                });
            }
            for journey in
                CreationJourney::list(scope).map_err(|error| agent_creation_failure(&error))?
            {
                if requested == format!("jrn_{}", URL_SAFE_NO_PAD.encode(journey.agent_id())) {
                    let mut agent = self.principal_agent(scope)?;
                    return Ok(BackendResponse {
                        result: agent_creation_json(
                            scope,
                            &mut agent,
                            &journey,
                            &journey.status(),
                        )?,
                        session: None,
                    });
                }
            }
        }
        let existing = super::production_reads::execute(scope, request)
            .unwrap_or_else(|| Err(ApiFailure::not_found()));
        if request.operation.name != "evidence.get"
            || !matches!(&existing, Err(error) if error.code == "not-found")
        {
            return existing;
        }
        let id = path(request, "evidence_id")?;
        let digest = id
            .strip_prefix("evd_")
            .ok_or_else(|| ApiFailure::invalid_request(Some("evidence_id")))
            .and_then(super::projection::digest)?;
        let key = RowKey::new(format!(
            "native-approval-evidence-{}",
            super::projection::hex(&digest)
        ))
        .map_err(|_| ApiFailure::upstream_degraded())?;
        if let Some(row) = scope.get(Table::Cache, &key) {
            let material: serde_json::Value =
                serde_json::from_slice(row.bytes()).map_err(|_| ApiFailure::upstream_degraded())?;
            let bytes = base64::engine::general_purpose::STANDARD
                .decode(
                    material["bytes_base64"]
                        .as_str()
                        .ok_or_else(ApiFailure::upstream_degraded)?,
                )
                .map_err(|_| ApiFailure::upstream_degraded())?;
            if <[u8; 32]>::from(Sha256::digest(&bytes)) != digest
                || material["evidence_id"] != id
                || !matches!(
                    (
                        material["class"].as_str(),
                        material["verification"].as_str()
                    ),
                    (
                        Some("approval-hold" | "local-journey-state"),
                        Some("unverified")
                    ) | (
                        Some("checkpoint-proof"),
                        Some("checkpoint-finalised" | "settlement-anchored")
                    )
                )
            {
                return Err(ApiFailure::upstream_degraded());
            }
            return Ok(BackendResponse {
                result: material,
                session: None,
            });
        }
        let mut agent = self.principal_agent(scope)?;
        for journey in
            CreationJourney::list(scope).map_err(|error| agent_creation_failure(&error))?
        {
            for (stage, _) in journey.status().stages {
                if let Some(material) = creation_material(scope, &mut agent, &journey, stage)? {
                    if material["evidence_id"] == id {
                        return Ok(BackendResponse {
                            result: material,
                            session: None,
                        });
                    }
                }
            }
        }
        let mut native_cursor = None;
        let mut native_seen = std::collections::BTreeSet::new();
        loop {
            let page = agent
                .native_effect_approval_list_facts(native_cursor, 100)
                .map_err(agent_failure)?;
            for facts in page
                .approvals
                .iter()
                .filter(|facts| facts.requires_approval)
            {
                let material = agent
                    .native_effect_approval_material(facts.approval_id, facts.held_digest)
                    .map_err(agent_failure)?;
                for value in [
                    super::projection::owned_material(
                        &material.canonical_unsigned_bytes,
                        "approval-hold",
                        "application/vnd.layerx.activity",
                    ),
                    super::projection::owned_material(
                        &material.immutable_carrier_bytes,
                        "approval-hold",
                        "application/vnd.layerx.approval-carrier",
                    ),
                    super::projection::owned_material(
                        &material.canonical_budget_bytes,
                        "local-journey-state",
                        "application/vnd.layerx.budget-allocation",
                    ),
                ] {
                    if value["evidence_id"] == id {
                        cache_native_material(scope, value.clone(), self.now()?)?;
                        return Ok(BackendResponse {
                            result: value,
                            session: None,
                        });
                    }
                }
            }
            match page.next_cursor {
                None => break,
                Some(next) if native_seen.insert(next) => native_cursor = Some(next),
                _ => return Err(ApiFailure::upstream_degraded()),
            }
        }
        let mut program_cursor = None;
        let mut program_seen = std::collections::BTreeSet::new();
        loop {
            let page = agent
                .native_program_approval_list(program_cursor, 100)
                .map_err(agent_failure)?;
            for facts in &page.approvals {
                let material = agent
                    .native_program_approval_material(facts.approval_id, facts.held_digest)
                    .map_err(agent_failure)?;
                material
                    .validate_facts(facts, agent.registry())
                    .map_err(agent_failure)?;
                for value in [
                    super::projection::owned_material(
                        &material.canonical_unsigned_bytes,
                        "approval-hold",
                        "application/vnd.layerx.activity",
                    ),
                    super::projection::owned_material(
                        &material.immutable_carrier_bytes,
                        "approval-hold",
                        "application/vnd.layerx.approval-carrier",
                    ),
                    super::projection::owned_material(
                        &material.canonical_budget_bytes,
                        "local-journey-state",
                        "application/vnd.layerx.budget-allocation",
                    ),
                ] {
                    if value["evidence_id"] == id {
                        cache_native_material(scope, value.clone(), self.now()?)?;
                        return Ok(BackendResponse {
                            result: value,
                            session: None,
                        });
                    }
                }
                let has_budget = facts.fee_asset.is_some()
                    || matches!(&facts.semantics,
                    super::agent_runtime::NativeProgramApprovalSemantics::AuthorizedLimits(rows) if !rows.is_empty());
                if has_budget {
                    let sequence = agent.head().map_err(agent_failure)?.chain_sequence;
                    let row = agent
                        .native_program_approval_budget(
                            facts.approval_id,
                            facts.held_digest,
                            sequence,
                        )
                        .map_err(agent_failure)?;
                    if super::projection::hex(&row.proof_digest) == super::projection::hex(&digest)
                    {
                        let mut value = super::projection::owned_material(
                            &row.verified_proof_bytes,
                            "checkpoint-proof",
                            "application/vnd.layerx.budget-proof",
                        );
                        value["verification"] = json!(super::projection::level(row.verification)?);
                        cache_native_material(scope, value.clone(), self.now()?)?;
                        return Ok(BackendResponse {
                            result: value,
                            session: None,
                        });
                    }
                }
            }
            match page.next_cursor {
                None => break,
                Some(next) if program_seen.insert(next) => program_cursor = Some(next),
                _ => return Err(ApiFailure::upstream_degraded()),
            }
        }
        let mut cursor = None;
        let mut seen = std::collections::BTreeSet::new();
        loop {
            let page = agent.agent_list(cursor, 100).map_err(agent_failure)?;
            for managed in page.agents {
                if managed.evidence.iter().any(|evidence| {
                    super::projection::digest(
                        evidence
                            .evidence_id
                            .strip_prefix("evd_")
                            .unwrap_or(&evidence.evidence_id),
                    )
                    .ok()
                        == Some(digest)
                }) {
                    let material = agent
                        .managed_evidence(&managed.agent_id, digest)
                        .map_err(agent_failure)?;
                    return Ok(BackendResponse {
                        result: super::projection::managed_export(&material)?,
                        session: None,
                    });
                }
            }
            match page.next_cursor {
                None => return existing,
                Some(next) if seen.insert(next) => cursor = Some(next),
                _ => return Err(ApiFailure::upstream_degraded()),
            }
        }
    }
}

impl ProductionComponents {
    fn execute_bootstrap(
        &self,
        request: &ScopedRequest<'_>,
    ) -> Result<BackendResponse, ApiFailure> {
        let observed_at = self.now()?;
        match request.operation.name.as_str() {
            "account.create" => self.bootstrap_account_create(request, observed_at),
            "passkey.register.begin" => self.bootstrap_passkey_register_begin(request, observed_at),
            "passkey.register.finish" => {
                self.bootstrap_passkey_register_finish(request, observed_at)
            }
            "passkey.assert.begin" => self.bootstrap_passkey_assert_begin(request, observed_at),
            "passkey.assert.finish" => self.bootstrap_passkey_assert_finish(request, observed_at),
            "session.open" => self.bootstrap_session_open(request, observed_at),
            "session.fee-policy" => {
                let policy = self
                    .agent
                    .lock()
                    .map_err(|_| ApiFailure::unavailable())?
                    .native_fee_policy()
                    .map_err(agent_failure)?;
                Ok(BackendResponse {
                    result: json!({"asset_id": hex_bytes(&policy.asset_id),
                    "currency": policy.currency, "decimals": policy.decimals}),
                    session: None,
                })
            }
            _ => Err(ApiFailure::not_found()),
        }
    }
    fn bootstrap_account_create(
        &self,
        request: &ScopedRequest<'_>,
        now: u64,
    ) -> Result<BackendResponse, ApiFailure> {
        let (_, result, session) = {
            let email = text_field(&request.body, "email")?;
            let display_name = text_field(&request.body, "display_name")?;
            let provisioned = self
                .identity
                .provision(email, display_name, required_idempotency(request)?, now)
                .map_err(|error| identity_failure(&error))?;
            self.auth_index
                .bind_account(email, &provisioned.principal)
                .map_err(|error| auth_failure(&error))?;
            let mut store = self.store.lock().map_err(|_| ApiFailure::unavailable())?;
            let mut scope = store
                .principal(&provisioned.principal)
                .map_err(|_| ApiFailure::unavailable())?;
            let mut journey = OnboardingJourney::start(&mut scope, &provisioned.onboarding, now)
                .map_err(|_| ApiFailure::upstream_degraded())?;
            self.custody
                .resume_onboarding_local(&mut journey, &mut scope, now)
                .map_err(|_| ApiFailure::upstream_degraded())?;
            identity_dispatch::update_profile(
                &mut scope,
                &json!({"display_name": display_name}),
                now,
            )
            .map_err(|error| identity_failure(&error))?;
            let result = json!({"account_id": provisioned.principal.as_str(),
                        "onboarding": super::identity::IdentityProjector::onboarding(&scope, &journey.status())?});
            (provisioned.principal, result, None)
        };
        Ok(BackendResponse { result, session })
    }

    fn bootstrap_passkey_register_begin(
        &self,
        request: &ScopedRequest<'_>,
        now: u64,
    ) -> Result<BackendResponse, ApiFailure> {
        let (_, result, session) = {
            let principal =
                crate::store::PrincipalId::new(text_field(&request.body, "account_id")?)
                    .map_err(|_| ApiFailure::invalid_request(Some("account_id")))?;
            let mut store = self.store.lock().map_err(|_| ApiFailure::unavailable())?;
            let mut scope = store
                .principal(&principal)
                .map_err(|_| ApiFailure::unauthenticated())?;
            let profile =
                identity_dispatch::profile(&scope).map_err(|error| identity_failure(&error))?;
            let account =
                AccountIdentity::new(principal.as_str(), text_field(&profile, "display_name")?)
                    .map_err(|error| auth_api_failure(&error))?;
            let challenge = self
                .passkeys
                .begin_registration(&mut scope, &account, "Primary passkey", now)
                .map_err(|error| auth_api_failure(&error))?;
            self.auth_index
                .bind_registration_at(
                    &challenge.registration_id,
                    &principal,
                    challenge.expires_at,
                    now,
                )
                .map_err(|error| auth_failure(&error))?;
            let result =
                super::identity::IdentityProjector::checked_registration_challenge(&challenge)?;
            (principal, result, None)
        };
        Ok(BackendResponse { result, session })
    }

    fn bootstrap_passkey_register_finish(
        &self,
        request: &ScopedRequest<'_>,
        now: u64,
    ) -> Result<BackendResponse, ApiFailure> {
        let (_, result, session) = {
            let registration_id = path(request, "registration_id")?;
            let principal = self
                .auth_index
                .resolve_registration(registration_id, now)
                .map_err(|error| auth_failure(&error))?;
            let mut store = self.store.lock().map_err(|_| ApiFailure::unavailable())?;
            let mut scope = store
                .principal(&principal)
                .map_err(|_| ApiFailure::unavailable())?;
            let passkey = self
                .passkeys
                .finish_registration(
                    &mut scope,
                    registration_id,
                    text_field(&request.body, "credential")?,
                    now,
                )
                .map_err(|error| auth_api_failure(&error))?;
            let result = super::identity::IdentityProjector::checked_passkey(&passkey)?;
            (principal, result, None)
        };
        Ok(BackendResponse { result, session })
    }

    fn bootstrap_passkey_assert_begin(
        &self,
        request: &ScopedRequest<'_>,
        now: u64,
    ) -> Result<BackendResponse, ApiFailure> {
        let (_, result, session) = {
            let email = text_field(&request.body, "email")?;
            let principal = self
                .auth_index
                .resolve_account(email)
                .map_err(|error| auth_failure(&error))?;
            let mut store = self.store.lock().map_err(|_| ApiFailure::unavailable())?;
            let mut scope = store
                .principal(&principal)
                .map_err(|_| ApiFailure::unauthenticated())?;
            let challenge = self
                .passkeys
                .begin_assertion(&mut scope, now)
                .map_err(|error| auth_api_failure(&error))?;
            self.auth_index
                .bind_assertion_at(
                    &challenge.assertion_id,
                    &principal,
                    challenge.expires_at,
                    now,
                )
                .map_err(|error| auth_failure(&error))?;
            let result =
                super::identity::IdentityProjector::checked_assertion_challenge(&challenge)?;
            (principal, result, None)
        };
        Ok(BackendResponse { result, session })
    }

    fn bootstrap_passkey_assert_finish(
        &self,
        request: &ScopedRequest<'_>,
        now: u64,
    ) -> Result<BackendResponse, ApiFailure> {
        let (_, result, session) = {
            let assertion_id = path(request, "assertion_id")?;
            let principal = self
                .auth_index
                .resolve_assertion(assertion_id, now)
                .map_err(|error| auth_failure(&error))?;
            let mut store = self.store.lock().map_err(|_| ApiFailure::unavailable())?;
            let mut scope = store
                .principal(&principal)
                .map_err(|_| ApiFailure::unavailable())?;
            let proof = self
                .passkeys
                .finish_assertion(
                    &mut scope,
                    assertion_id,
                    text_field(&request.body, "credential")?,
                    now,
                )
                .map_err(|error| auth_api_failure(&error))?;
            let result = super::identity::IdentityProjector::checked_assertion(&proof)?;
            (principal, result, None)
        };
        Ok(BackendResponse { result, session })
    }

    fn bootstrap_session_open(
        &self,
        request: &ScopedRequest<'_>,
        now: u64,
    ) -> Result<BackendResponse, ApiFailure> {
        let (_, result, session) = {
            let assertion_id = text_field(&request.body, "assertion_id")?;
            let principal = self
                .auth_index
                .resolve_assertion(assertion_id, now)
                .map_err(|error| auth_failure(&error))?;
            let onboarding = self.advance_native_onboarding(&principal, &request.trace, now)?;
            if onboarding.state() != crate::onboarding::OnboardingState::Complete {
                return Err(ApiFailure::forbidden());
            }
            let (device_label, device_platform) = browser_device(request)?;
            let idempotency = required_idempotency(request)?;
            let action = action_key(idempotency);
            let recovery_seed = self
                .auth_index
                .browser_session_seed(&principal, assertion_id, idempotency)
                .map_err(|error| auth_failure(&error))?;
            let mut store = self.store.lock().map_err(|_| ApiFailure::unavailable())?;
            let mut scope = store
                .principal(&principal)
                .map_err(|_| ApiFailure::unavailable())?;
            let mut prepared = self
                .passkeys
                .prepare_open_session_replayable(
                    &mut scope,
                    crate::auth::ReplayableSessionRequest {
                        assertion_id,
                        label: device_label,
                        platform: device_platform,
                        action_key: action,
                        recovery_seed,
                        now,
                    },
                )
                .map_err(|error| auth_api_failure(&error))?;
            let mut agent = self.principal_agent(&scope)?;
            let owner = owner::resolve_principal_owner(self, &scope, &mut agent)?;
            let registry = agent.registry().clone();
            let (intent, grant_id) = browser_grant_intent(
                &mut scope,
                &mut agent,
                &prepared,
                recovery_seed,
                &owner.actor,
                action,
            )?;
            let trace =
                TraceId::parse(&request.trace).map_err(|_| ApiFailure::invalid_request(None))?;
            let mut adapter = ProductionAgentCreation::new(
                &mut agent,
                &self.agent_contract,
                &self.custody,
                &trace,
                owner.actor,
                owner.authority,
                super::agent_creation::CreationBounds {
                    timestamp_span: self.agent_timestamp_span_seconds,
                    fee_limit: self.agent_fee_limit,
                },
            )
            .map_err(|_| ApiFailure::upstream_degraded())?;
            let signed = adapter
                .prepare_lifecycle_intent(
                    &mut scope,
                    &registry,
                    intent,
                    action,
                    KeyId::new("human-primary").map_err(|_| ApiFailure::upstream_degraded())?,
                    prepared.opened_at(),
                )
                .map_err(|_| ApiFailure::upstream_degraded())?;
            drop(scope);
            drop(store);
            let evidence = adapter
                .submit_prepared(signed)
                .map_err(|_| ApiFailure::upstream_degraded())?;
            ProductionAgentCreation::finalization_evidence(&evidence, ModuleId::Governance, 5, now)
                .map_err(|_| ApiFailure::upstream_degraded())?;
            prepared
                .bind_protocol_grant(grant_id)
                .map_err(|error| auth_api_failure(&error))?;
            let mut store = self.store.lock().map_err(|_| ApiFailure::unavailable())?;
            let mut scope = store
                .principal(&principal)
                .map_err(|_| ApiFailure::unavailable())?;
            let grant = Passkeys::commit_open_session(&mut scope, prepared, now)
                .map_err(|error| auth_api_failure(&error))?;
            self.auth_index
                .bind_session(&grant, &principal)
                .map_err(|error| auth_failure(&error))?;
            crate::event_producer::enroll_session(&mut scope, &grant, now)
                .map_err(|_| ApiFailure::unavailable())?;
            let session_view = grant.session();
            let result = super::identity::IdentityProjector::checked_session(&session_view)?;
            let session = session_secrets(&grant, now);
            (principal, result, Some(session))
        };
        Ok(BackendResponse { result, session })
    }
}

impl ProductionComponents {
    fn execute_authorized(
        &self,
        request: &ScopedRequest<'_>,
        scope: &mut crate::store::PrincipalScope<'_>,
        principal: &crate::store::PrincipalId,
        session_id: &str,
    ) -> Result<BackendResponse, ApiFailure> {
        match request.operation.name.as_str() {
            "agent.create" => self.execute_agent_create(request, scope),
            "agent.list" => self.execute_agent_list(scope),
            "agent.get" => self.execute_agent_get(request, scope),
            "agent.pause" | "agent.resume" => self.execute_agent_pause(request, scope),
            "agent.limit" => self.execute_agent_limit(request, scope),
            "agent.reclaim" => self.execute_agent_reclaim(request, scope),
            "agent.rotation.disclosure" => self.execute_rotation_disclosure(request, scope),
            "agent.rotate" | "agent.rotation.start" | "agent.recover" => {
                self.execute_agent_rotate(request, scope, principal)
            }
            "agent.archive" => self.execute_agent_archive(request, scope),
            "security.action" => Self::execute_security_action(request, scope),
            "stepup.begin" => self.execute_stepup_begin(request, scope, principal),
            "stepup.finish" => self.execute_stepup_finish(request, scope),
            "profile.get" => Self::execute_profile_get(scope),
            "profile.update" => self.execute_profile_update(request, scope),
            "onboarding.status" => Self::execute_onboarding_status(scope),
            "intent.plan" => self.execute_intent_plan(request, scope),
            "intent.submit" => self.submit_intent(request, scope),
            "native.send.access.begin" => self.execute_native_send_access_begin(request, scope),
            "native.send.access.confirm" => self.execute_native_send_access_confirm(request, scope),
            "move.quote" => self.execute_move_quote(request, scope),
            "move.commit" => self.execute_move_commit(request, scope),
            "security.passkey.list" => Self::execute_security_passkey_list(scope),
            "security.passkey.register.begin" => {
                self.execute_security_passkey_register_begin(request, scope, principal)
            }
            "security.passkey.register.finish" => {
                self.execute_security_passkey_register_finish(request, scope, principal)
            }
            "security.passkey.revoke" => Self::execute_security_passkey_revoke(request, scope),
            "session.refresh" => self.execute_session_refresh(request, scope, principal),
            "authenticator.status" => self.execute_authenticator_status(principal),
            "authenticator.setup.begin" => {
                self.execute_authenticator_setup_begin(request, principal)
            }
            "authenticator.setup.finish" => {
                self.execute_authenticator_setup_finish(request, principal)
            }
            "authenticator.disable" => self.execute_authenticator_disable(request, principal),
            "authenticator.backup.rotate" => self.execute_authenticator_backup_rotate(principal),
            "security.recovery.reveal" => self.execute_security_recovery_reveal(request, principal),
            "security.key-export.begin" => self.execute_key_export_begin(request, scope),
            "security.key-export.finish" => self.execute_key_export_finish(request, scope),
            "session.list" => Self::execute_session_list(scope, session_id),
            "session.revoke" | "security.session.revoke" => {
                self.execute_session_revoke(request, scope)
            }
            "session.revoke-all" | "security.session.revoke-all" => {
                self.execute_session_revoke_all(request, scope)
            }
            "approval.program.disclosure" => {
                self.execute_program_approval_disclosure(request, scope)
            }
            "approval.program.list" => self.execute_program_approval_list(request, scope),
            "approval.program.get" | "approval.program.material" | "approval.program.budget" => {
                self.execute_program_approval_read(request, scope)
            }
            "approval.program.approve" | "approval.program.reject" => {
                self.execute_program_approval_decide(request, scope)
            }
            "approval.list" => self.execute_approval_list(scope),
            "approval.get" => self.execute_approval_get(request, scope),
            "approval.approve" | "approval.reject" => self.execute_approval_approve(request, scope),
            "support.list" => Self::execute_support_list(scope),
            "support.create" => self.execute_support_create(request, scope),
            "support.reply" => self.execute_support_reply(request, scope),
            "support.read" => self.execute_support_read(request, scope),
            "support.status" => Self::execute_support_status(request, scope),
            "support.feedback" => self.execute_support_feedback(request, scope),
            "notification.list" => self.execute_notification_list(scope),
            "notification.read" => self.execute_notification_read(request, scope),
            "notification.preferences.get" => Self::execute_notification_preferences_get(scope),
            "notification.preferences.set" => {
                self.execute_notification_preferences_set(request, scope)
            }
            "home.summary" => self.execute_home_summary(scope),
            "account.balance" => self.execute_account_balance(scope),
            "stream.open" => self.execute_stream_open(scope),
            "stream.next" => self.execute_stream_next(request, scope),
            "activity.export.statement" => self.execute_activity_export_statement(request, scope),
            "activity.export.evidence" => self.execute_activity_export_evidence(request, scope),
            "activity.entry" => self.execute_activity_entry(request, scope),
            "activity.query" => self.execute_activity_query(request, scope),
            _ => self.execute_read_projection(request, scope),
        }
    }
}

impl ProductionComponents {
    fn execute_agent_resume(
        &self,
        request: &ScopedRequest<'_>,
        scope: &mut crate::store::PrincipalScope<'_>,
    ) -> Result<BackendResponse, ApiFailure> {
        let mut agent = self.principal_agent(scope)?;
        let agent_id = path(request, "agent_id")?;

        let context = agent.agent_context(agent_id).map_err(agent_failure)?;
        let identity = agent
            .identity_resolve(&context.agent_did)
            .map_err(agent_failure)?;
        if identity.frozen {
            return Err(ApiFailure::upstream_degraded());
        }
        let prior_session = agent
            .session_fee_state(context.protocol_grant_id)
            .map_err(agent_failure)?;
        validate_resumed_session(&prior_session, &context.agent_did)?;
        let operation_key = action_key(required_idempotency(request)?);
        let current = self.now()?;
        let trace =
            TraceId::parse(&request.trace).map_err(|_| ApiFailure::invalid_request(None))?;
        let registry = agent.registry().clone();
        let mut adapter = ProductionAgentCreation::new(
            &mut agent,
            &self.agent_contract,
            &self.custody,
            &trace,
            AgentDid::new(context.seed.actor.clone())
                .map_err(|_| ApiFailure::upstream_degraded())?,
            AuthorityRef::new(context.seed.primary_authority.clone())
                .map_err(|_| ApiFailure::upstream_degraded())?,
            super::agent_creation::CreationBounds {
                timestamp_span: self.agent_timestamp_span_seconds,
                fee_limit: self.agent_fee_limit,
            },
        )
        .map_err(|_| ApiFailure::upstream_degraded())?;
        let lifetime = context
            .seed
            .session_expiry_unix_seconds
            .checked_sub(context.seed.created_at)
            .ok_or_else(ApiFailure::upstream_degraded)?;
        let expires_at = current
            .checked_add(lifetime)
            .ok_or_else(ApiFailure::upstream_degraded)?;
        let evidence = adapter
            .provision_session_scoped(
                scope,
                &registry,
                SessionProvision {
                    native_fee_budget: prior_session.grant.fee_budget,
                    replacement: prior_session
                        .grant
                        .fee_budget
                        .map(|_| prior_session.clone()),
                    not_before: current,
                    action_key: operation_key,
                    did: Did::new(context.agent_did.as_bytes())
                        .map_err(|_| ApiFailure::upstream_degraded())?,
                    activity_types: context
                        .seed
                        .activity_types
                        .iter()
                        .map(|value| layerx_types::payload::ActivityType::from_u32(*value))
                        .collect::<Result<Vec<_>, _>>()
                        .map_err(|_| ApiFailure::upstream_degraded())?,
                    daemon_scopes: context.seed.session_scopes.clone(),
                    expires_at,
                    primary_authority: context.seed.custody_public_key,
                    grantor: managed_protocol_identity(&context.seed.agent_id)?,
                    custody_key: KeyId::new(context.seed.custody_key.clone())
                        .map_err(|_| ApiFailure::upstream_degraded())?,
                    revocation_sequence: identity.revocation_sequence,
                },
            )
            .map_err(agent_failure_from_creation_contract)?;
        let (token_id, generation, finalization) = adapter
            .take_latest_session_credential()
            .map_err(agent_failure_from_creation_contract)?;
        let observation = agent
            .agent_session_bind(agent_id, operation_key, token_id, operation_key)
            .map_err(agent_failure)?;
        if observation.generation != generation {
            return Err(ApiFailure::upstream_degraded());
        }
        if finalization.action_key != operation_key
            || finalization.receipt_digest != evidence.receipt_digest
            || finalization.observed_sequence != evidence.observed_sequence
            || finalization.verification != evidence.rung.wire_rank()
        {
            return Err(ApiFailure::upstream_degraded());
        }
        let value = agent
            .agent_control(agent_id, true, observation.evidence_digest, finalization)
            .map_err(agent_failure)?;
        Ok(BackendResponse {
            result: managed_agent_json(&mut agent, value)?,
            session: None,
        })
    }
}

impl ProductionComponents {
    fn archive_defund(
        &self,
        scope: &mut crate::store::PrincipalScope<'_>,
        agent: &mut AgentRuntime,
        context: &super::agent_runtime::AgentLifecycleContext,
        pre: &super::agent_runtime::AgentBudgetState,
        operation_key: [u8; 32],
        trace: &TraceId,
    ) -> Result<(), ApiFailure> {
        let defund_key: [u8; 32] = Sha256::digest(
            [
                b"layerx-human/agent-archive-defund/v1\0".as_slice(),
                operation_key.as_slice(),
            ]
            .concat(),
        )
        .into();
        let receipt =
            self.submit_budget_defund(scope, agent, context, pre.remaining, defund_key, trace)?;
        ProductionAgentCreation::finalization_evidence(&receipt, ModuleId::Budget, 8, self.now()?)
            .map_err(|_| ApiFailure::upstream_degraded())?;
        Ok(())
    }
}

pub(crate) const DEFAULT_RP_ID: &str = "paxportwallet.com";
pub(crate) const DEFAULT_WEB_ORIGINS: &str =
    "https://paxportwallet.com,https://api-mainnet-beta.paxeer.network";

/// Passkey relying party identifier and comma-separated web origin list, read
/// from `LAYERX_HUMAN_RP_ID` and `LAYERX_HUMAN_WEB_ORIGIN` with beta defaults.
pub fn web_identity(lookup: impl Fn(&str) -> Option<String>) -> (String, String) {
    let value = |name: &str, default: &str| {
        lookup(name)
            .map(|value| value.trim().to_owned())
            .filter(|value| !value.is_empty())
            .unwrap_or_else(|| default.to_owned())
    };
    (
        value("LAYERX_HUMAN_RP_ID", DEFAULT_RP_ID),
        value("LAYERX_HUMAN_WEB_ORIGIN", DEFAULT_WEB_ORIGINS),
    )
}

fn production_passkey_config() -> Result<AuthConfig, String> {
    Ok(AuthConfig {
        rp_id: web_identity(|name| env::var(name).ok()).0,
        rp_name: required("LAYERX_HUMAN_RP_NAME")?,
        origin: required("LAYERX_HUMAN_ORIGIN")?,
        ceremony_ttl_secs: number("LAYERX_HUMAN_CEREMONY_TTL_SECONDS")?,
        assertion_ttl_secs: number("LAYERX_HUMAN_ASSERTION_TTL_SECONDS")?,
        session_ttl_secs: number("LAYERX_HUMAN_SESSION_TTL_SECONDS")?,
        refresh_ttl_secs: number("LAYERX_HUMAN_REFRESH_TTL_SECONDS")?,
        step_up_ttl_secs: number("LAYERX_HUMAN_STEP_UP_TTL_SECONDS")?,
        rate_limit: RateLimit {
            attempts: number("LAYERX_HUMAN_AUTH_RATE_ATTEMPTS")?,
            window_secs: number("LAYERX_HUMAN_AUTH_RATE_WINDOW_SECONDS")?,
        },
    })
}
fn production_retention_config() -> Result<RetentionPolicy, String> {
    let retention = |name: &str| number(name).map(RetentionPeriod::new);
    Ok(RetentionPolicy {
        journeys: retention("LAYERX_HUMAN_RETENTION_JOURNEYS_SECONDS")?,
        notifications: retention("LAYERX_HUMAN_RETENTION_NOTIFICATIONS_SECONDS")?,
        audit: retention("LAYERX_HUMAN_RETENTION_AUDIT_SECONDS")?,
        telemetry: retention("LAYERX_HUMAN_RETENTION_TELEMETRY_SECONDS")?,
        cache: retention("LAYERX_HUMAN_RETENTION_CACHE_SECONDS")?,
    })
}

pub fn validate_production_configuration(
    config: &ProductionComponentsConfig,
) -> Result<(), String> {
    let schema = ApiSchema::v1().map_err(|_| "human API schema is invalid".to_owned())?;
    let operations = schema
        .operations()
        .iter()
        .filter(|operation| !operation.retired())
        .collect::<Vec<_>>();
    if operations.len() != PRODUCTION_OPERATIONS.len()
        || operations
            .iter()
            .any(|operation| !PRODUCTION_OPERATIONS.contains(&operation.name.as_str()))
    {
        return Err("human API operation dispatch is incomplete".to_owned());
    }
    if !(1..=60).contains(&config.capability_ttl_seconds) {
        return Err("LAYERX_HUMAN_CAPABILITY_TTL_SECONDS must be between 1 and 60".to_owned());
    }
    if config.evm_gas_limit == 0
        || config.evm_max_fee_per_gas == 0
        || config.evm_max_priority_fee_per_gas > config.evm_max_fee_per_gas
    {
        return Err("EVM execution gas and fee bounds are invalid".to_owned());
    }
    if config.onboarding_initial_funding == 0 {
        return Err("onboarding initial funding must be positive".to_owned());
    }
    if config.agent_timestamp_span_seconds == 0 || config.agent_fee_limit == 0 {
        return Err("agent preparation bounds must be non-zero".to_owned());
    }
    if config.continuation_unknown_deadline_seconds == 0 {
        return Err(
            "LAYERX_HUMAN_CONTINUATION_UNKNOWN_DEADLINE_SECONDS must be non-zero".to_owned(),
        );
    }
    Ok(())
}
fn production_kms_provider(
    config: &RemoteKmsConfig,
    clock: Arc<dyn Clock>,
) -> Result<RemoteKmsProvider, String> {
    let tls = mutual_tls(
        &config.root_certificate,
        &config.client_certificate,
        &config.client_private_key,
    )?;
    let provider = RemoteKmsProvider::new(
        config.provider_reference.clone(),
        config.endpoint,
        config.server_name.clone(),
        tls,
        config.limits,
        clock,
    )
    .map_err(|_| "KMS configuration was refused".to_owned())?;
    Ok(provider)
}

const ATTESTOR_PROVIDER_REFERENCE: &str = "attestor-quorum/v1";

enum ProductionKms {
    Attestor(AttestorKms),
    Remote(RemoteKmsProvider),
}

#[derive(Clone)]
pub struct AttestorCustodyConfig {
    nodes: Vec<(String, SocketAddr)>,
    signers: Vec<String>,
    root_certificate: Vec<u8>,
    client_certificate: Vec<u8>,
    client_private_key: zeroize::Zeroizing<Vec<u8>>,
    deadline: Duration,
}

impl std::fmt::Debug for AttestorCustodyConfig {
    fn fmt(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        formatter
            .debug_struct("AttestorCustodyConfig")
            .field("nodes", &self.nodes)
            .field("signers", &self.signers)
            .field("identity", &"[client identity]")
            .field("deadline", &self.deadline)
            .finish_non_exhaustive()
    }
}

impl AttestorCustodyConfig {
    /// Builds the attestor custody configuration and checks its client identity and quorum.
    ///
    /// # Errors
    ///
    /// Refuses a configuration whose client certificates or private key are rejected, or whose
    /// signers are not at least three distinct configured attestor nodes.
    pub fn new(
        nodes: Vec<(String, SocketAddr)>,
        signers: Vec<String>,
        root_certificate: Vec<u8>,
        client_certificate: Vec<u8>,
        client_private_key: Vec<u8>,
        deadline: Duration,
    ) -> Result<Self, String> {
        let config = Self {
            nodes,
            signers,
            root_certificate,
            client_certificate,
            client_private_key: zeroize::Zeroizing::new(client_private_key),
            deadline,
        };
        config
            .client()
            .map_err(|_| "attestor client configuration was refused".to_owned())?;
        let distinct: std::collections::BTreeSet<&str> =
            config.signers.iter().map(String::as_str).collect();
        if distinct.len() != config.signers.len()
            || distinct.len() < 3
            || distinct
                .iter()
                .any(|signer| !config.nodes.iter().any(|(id, _)| id == signer))
        {
            return Err("attestor signing quorum was refused".to_owned());
        }
        Ok(config)
    }

    fn from_environment() -> Result<Option<Self>, String> {
        let Some(table) = env::var("LAYERX_HUMAN_ATTESTOR_NODES")
            .ok()
            .filter(|value| !value.is_empty())
        else {
            return Ok(None);
        };
        let invalid = || "LAYERX_HUMAN_ATTESTOR_NODES is invalid".to_owned();
        let nodes = table
            .split(',')
            .map(|entry| {
                let (id, address) = entry.split_once('=').ok_or_else(invalid)?;
                Ok((id.to_owned(), address.parse().map_err(|_| invalid())?))
            })
            .collect::<Result<Vec<(String, SocketAddr)>, String>>()?;
        let signers = required("LAYERX_HUMAN_ATTESTOR_SIGNERS")?
            .split(',')
            .map(str::to_owned)
            .collect();
        Self::new(
            nodes,
            signers,
            read_nonempty(&absolute("LAYERX_HUMAN_ATTESTOR_ROOT_CERTIFICATE_DER")?)?,
            read_nonempty(&absolute("LAYERX_HUMAN_ATTESTOR_CLIENT_CERTIFICATE_DER")?)?,
            read_nonempty(&absolute("LAYERX_HUMAN_ATTESTOR_CLIENT_PRIVATE_KEY_DER")?)?,
            Duration::from_secs(number("LAYERX_HUMAN_ATTESTOR_DEADLINE_SECONDS")?),
        )
        .map(Some)
    }

    fn client(
        &self,
    ) -> Result<layerx_human_kms::attestor::AttestorClient, layerx_human_kms::attestor::AttestorError>
    {
        layerx_human_kms::attestor::AttestorClient::new(
            &self.nodes,
            std::slice::from_ref(&self.root_certificate),
            std::slice::from_ref(&self.client_certificate),
            &self.client_private_key,
            self.deadline,
        )
    }
}

struct AttestorKmsState {
    config: AttestorCustodyConfig,
    creation: Mutex<()>,
    enrolment: Mutex<Option<(String, String)>>,
    assertions: Mutex<std::collections::BTreeMap<String, zeroize::Zeroizing<String>>>,
    sends: Mutex<Option<super::attestor_send_journal::AttestorSendJournal>>,
    clock: Mutex<Option<Arc<dyn Clock>>>,
    last_refusal: Mutex<Option<String>>,
}

#[derive(Clone, Debug, Eq, PartialEq, serde::Serialize, serde::Deserialize)]
#[serde(deny_unknown_fields)]
pub(super) struct AttestorSendDisclosure {
    recipient: [u8; 20],
    denom: String,
    amount: [u8; 32],
}

#[derive(Clone, Debug, Eq, PartialEq, serde::Serialize, serde::Deserialize)]
#[serde(deny_unknown_fields)]
pub(super) struct AttestorSendAuthorization {
    pub(super) authorization: crate::custody::EvmPlanAuthorization,
    disclosure: AttestorSendDisclosure,
}

const ATTESTOR_SEND_OPERATION: u8 = 11;

#[derive(Clone)]
pub struct AttestorKms {
    inner: Arc<AttestorKmsState>,
}

impl std::fmt::Debug for AttestorKms {
    fn fmt(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        formatter
            .debug_struct("AttestorKms")
            .field("config", &self.inner.config)
            .field("assertions", &"[redacted]")
            .finish()
    }
}

fn attestor_refusal(code: &str) -> KmsError {
    match code {
        "token_invalid" | "token_missing" | "token_not_owner" => KmsError::Authentication,
        "token_unavailable" | "chain_unavailable" | "quorum_too_few_signers" => {
            KmsError::Unavailable
        }
        "key_not_found" => KmsError::KeyNotFound,
        "key_exists" => KmsError::Conflict,
        "session_timeout" => KmsError::Timeout,
        _ => KmsError::Refused,
    }
}

fn attestor_kms_failure(error: &layerx_human_kms::attestor::AttestorError) -> KmsError {
    use layerx_human_kms::attestor::AttestorError;
    match error {
        AttestorError::Configuration(_) => KmsError::InvalidConfiguration,
        AttestorError::Refused { .. } => error
            .refusal_code()
            .map_or(KmsError::Refused, attestor_refusal),
        AttestorError::Disclosure(_)
        | AttestorError::WrongNetwork { .. }
        | AttestorError::UnsupportedActivity { .. } => KmsError::Refused,
        AttestorError::Timeout { .. } => KmsError::Timeout,
        AttestorError::Unavailable { .. } => KmsError::Unavailable,
        AttestorError::Authentication { .. } => KmsError::Authentication,
        AttestorError::MalformedResponse { .. } | AttestorError::SignatureInvalid { .. } => {
            KmsError::InvalidResponse
        }
    }
}

fn attestor_custody_failure(error: layerx_human_kms::attestor::AttestorError) -> CustodyError {
    use layerx_human_kms::attestor::AttestorError;
    match error {
        AttestorError::Disclosure(error) => CustodyError::Sign(error),
        AttestorError::WrongNetwork { .. } => CustodyError::InvalidNetwork,
        AttestorError::UnsupportedActivity { .. } => CustodyError::Kms(KmsError::Refused),
        other => CustodyError::Kms(attestor_kms_failure(&other)),
    }
}

fn attestor_owner_valid(owner: &str) -> bool {
    !owner.is_empty() && owner.len() <= 1_024 && !owner.chars().any(char::is_control)
}

fn attestor_key_id(binding: &PrincipalKeyBinding) -> String {
    let digest = binding.digest();
    let mut key_id = String::from("lx-");
    for byte in &digest[..16] {
        key_id.push_str(&format!("{byte:02x}"));
    }
    key_id
}

fn attestor_reference(
    key_id: &str,
    public_key: [u8; 32],
    owner: &str,
) -> Result<ProviderKeyReference, KmsError> {
    let mut public = String::with_capacity(64);
    for byte in public_key {
        public.push_str(&format!("{byte:02x}"));
    }
    ProviderKeyReference::new(format!("{key_id}\n{public}\n{owner}").into_bytes())
}

fn attestor_reference_parts(
    reference: &ProviderKeyReference,
) -> Result<(String, [u8; 32], String), KmsError> {
    let text = std::str::from_utf8(reference.as_bytes()).map_err(|_| KmsError::InvalidReference)?;
    let mut parts = text.splitn(3, '\n');
    let (Some(key_id), Some(public), Some(owner)) = (parts.next(), parts.next(), parts.next())
    else {
        return Err(KmsError::InvalidReference);
    };
    if public.len() != 64 || !attestor_owner_valid(owner) {
        return Err(KmsError::InvalidReference);
    }
    let mut public_key = [0_u8; 32];
    for (index, byte) in public_key.iter_mut().enumerate() {
        *byte = u8::from_str_radix(&public[index * 2..index * 2 + 2], 16)
            .map_err(|_| KmsError::InvalidReference)?;
    }
    Ok((key_id.to_owned(), public_key, owner.to_owned()))
}

impl AttestorKms {
    /// Connects to the attestor quorum described by `config` and probes it.
    ///
    /// # Errors
    ///
    /// Returns the probe's KMS error when the attestor quorum does not answer.
    pub fn connect(config: AttestorCustodyConfig) -> Result<Self, CustodyError> {
        let kms = Self {
            inner: Arc::new(AttestorKmsState {
                config,
                creation: Mutex::new(()),
                enrolment: Mutex::new(None),
                assertions: Mutex::new(std::collections::BTreeMap::new()),
                sends: Mutex::new(None),
                clock: Mutex::new(None),
                last_refusal: Mutex::new(None),
            }),
        };
        crate::custody::KmsProvider::probe(&kms).map_err(CustodyError::Kms)?;
        Ok(kms)
    }

    pub fn connect_durable(
        config: AttestorCustodyConfig,
        root: impl AsRef<std::path::Path>,
        key: [u8; 32],
        clock: Arc<dyn Clock>,
    ) -> Result<Self, CustodyError> {
        let journal = super::attestor_send_journal::AttestorSendJournal::open(root, key)
            .map_err(CustodyError::Kms)?;
        let kms = Self::connect(config)?;
        *kms.inner
            .sends
            .lock()
            .map_err(|_| CustodyError::Kms(KmsError::Unavailable))? = Some(journal);
        *kms.inner
            .clock
            .lock()
            .map_err(|_| CustodyError::Kms(KmsError::Unavailable))? = Some(Arc::new(
            layerx_human_kms::attestor::AttestorClock::new(clock),
        ));
        Ok(kms)
    }

    pub fn connect_with_clock(
        config: AttestorCustodyConfig,
        clock: Arc<dyn Clock>,
    ) -> Result<Self, CustodyError> {
        let kms = Self::connect(config)?;
        *kms.inner
            .clock
            .lock()
            .map_err(|_| CustodyError::Kms(KmsError::Unavailable))? = Some(Arc::new(
            layerx_human_kms::attestor::AttestorClock::new(clock),
        ));
        Ok(kms)
    }

    pub fn admit_context_assertion(&self, context: &PrincipalContext) -> Result<(), CustodyError> {
        let assertion = context
            .assertion()
            .ok_or(CustodyError::Kms(KmsError::Authentication))?;
        if context.session_id != super::production_auth::bearer_session_id(assertion) {
            return Err(CustodyError::Kms(KmsError::Authentication));
        }
        let subject = assertion_subject(assertion)
            .map_err(|_| CustodyError::Kms(KmsError::Authentication))?;
        self.admit_assertion(&subject, assertion)
    }

    /// Records the identity assertion that authorizes signing for `subject`.
    ///
    /// # Errors
    ///
    /// Refuses an invalid subject or an empty assertion as an authentication failure, and
    /// reports the KMS unavailable when the assertion store cannot be locked.
    pub fn admit_assertion(&self, subject: &str, assertion: &str) -> Result<(), CustodyError> {
        if !attestor_owner_valid(subject)
            || assertion_subject(assertion).ok().as_deref() != Some(subject)
        {
            return Err(CustodyError::Kms(KmsError::Authentication));
        }
        self.inner
            .assertions
            .lock()
            .map_err(|_| CustodyError::Kms(KmsError::Unavailable))?
            .insert(
                subject.to_owned(),
                zeroize::Zeroizing::new(assertion.to_owned()),
            );
        Ok(())
    }

    /// Creates the attestor-held primary key for `owner` and `account` in `keystore`.
    ///
    /// # Errors
    ///
    /// Refuses an invalid owner or an empty account, returns the keystore's creation error,
    /// and reports the KMS unavailable when the enrolment state cannot be locked.
    pub fn create_owned_key(
        &self,
        keystore: &Keystore,
        principal: &crate::store::PrincipalId,
        key: &KeyId,
        owner: &str,
        account: &str,
    ) -> Result<[u8; 32], CustodyError> {
        if !attestor_owner_valid(owner) || account.is_empty() {
            return Err(CustodyError::Kms(KmsError::Refused));
        }
        let _serial = self
            .inner
            .creation
            .lock()
            .map_err(|_| CustodyError::Kms(KmsError::Unavailable))?;
        *self
            .inner
            .enrolment
            .lock()
            .map_err(|_| CustodyError::Kms(KmsError::Unavailable))? =
            Some((owner.to_owned(), account.to_owned()));
        let created = keystore.create(principal, key, KeyClass::HumanPrimary);
        let cleared = self
            .inner
            .enrolment
            .lock()
            .map(|mut enrolment| *enrolment = None);
        let public_key = created?;
        cleared.map_err(|_| CustodyError::Kms(KmsError::Unavailable))?;
        Ok(public_key)
    }

    /// Returns the attestor quorum's refusal code for the latest refused operation, or the
    /// local refusal reason when the request never reached the quorum.
    #[must_use]
    pub fn last_refusal_code(&self) -> Option<String> {
        self.inner
            .last_refusal
            .lock()
            .ok()
            .and_then(|last| last.clone())
    }

    fn refused(&self, error: &layerx_human_kms::attestor::AttestorError) {
        if let Ok(mut last) = self.inner.last_refusal.lock() {
            *last = Some(
                error
                    .refusal_code()
                    .map_or_else(|| error.to_string(), str::to_owned),
            );
        }
    }

    fn signer(
        &self,
        key_id: &str,
        public_key: [u8; 32],
        network: u32,
    ) -> Result<layerx_human_kms::attestor::AttestorSigner, layerx_human_kms::attestor::AttestorError>
    {
        let mut client = self.inner.config.client()?;
        if let Some(clock) = self
            .inner
            .clock
            .lock()
            .map_err(|_| {
                layerx_human_kms::attestor::AttestorError::Configuration("attestor clock lock")
            })?
            .as_ref()
        {
            client = client.with_clock(Arc::clone(clock));
        }
        let signers: Vec<&str> = self
            .inner
            .config
            .signers
            .iter()
            .map(String::as_str)
            .collect();
        layerx_human_kms::attestor::AttestorSigner::new(
            client, key_id, public_key, &signers, network,
        )
    }

    fn sign_send_now(
        &self,
        binding: &PrincipalKeyBinding,
        reference: &ProviderKeyReference,
        payload: &[u8],
    ) -> Result<Vec<u8>, KmsError> {
        let (key_id, public_key, owner) = attestor_reference_parts(reference)?;
        if key_id != attestor_key_id(binding) {
            return Err(KmsError::Integrity);
        }
        let request: AttestorSendAuthorization =
            serde_json::from_slice(payload).map_err(|_| KmsError::Refused)?;
        let authorization = &request.authorization;
        let transaction = &authorization.transaction;
        let now = self
            .inner
            .clock
            .lock()
            .map_err(|_| KmsError::Unavailable)?
            .as_ref()
            .ok_or(KmsError::InvalidConfiguration)?
            .sample(Duration::from_secs(1))
            .map_err(|_| KmsError::Unavailable)?
            .unix_seconds();
        let mut bank = [0; 20];
        bank[18..].copy_from_slice(&[0x10, 0x01]);
        if authorization.binding_digest != binding.digest()
            || transaction.chain_id != u64::from(binding.network_id())
            || authorization.plan_id == [0; 32]
            || authorization.action_key == [0; 32]
            || authorization.principal.is_empty()
            || authorization.principal.len() > 128
            || authorization.tenant.is_empty()
            || authorization.tenant.len() > 128
            || authorization.wallet == [0; 20]
            || authorization.not_before > now
            || authorization.not_after < now
            || authorization.not_after <= authorization.not_before
            || transaction.to != bank
            || transaction.value != [0; 32]
            || transaction.gas_limit < 21_000
            || transaction.max_fee_per_gas == 0
            || transaction.max_priority_fee_per_gas > transaction.max_fee_per_gas
            || transaction.calldata.len() > 524_288
            || request.disclosure.recipient == [0; 20]
            || request.disclosure.denom.is_empty()
            || request.disclosure.amount == [0; 32]
        {
            return Err(KmsError::Refused);
        }
        let call = layerx_crypto::evm_transaction::Eip1559Call {
            chain_id: transaction.chain_id,
            nonce: transaction.nonce,
            max_priority_fee_per_gas: transaction.max_priority_fee_per_gas.into(),
            max_fee_per_gas: transaction.max_fee_per_gas.into(),
            gas_limit: transaction.gas_limit,
            to: transaction.to,
            data: transaction.calldata.clone(),
        };
        let disclosed = layerx_crypto::disclosure::call::bind_call(&call, authorization.wallet)
            .map_err(|_| KmsError::Refused)?;
        match disclosed
            .construct_method()
            .map_err(|_| KmsError::Refused)?
        {
            Some(layerx_crypto::disclosure::call::ConstructMethod::BankSend {
                from,
                to,
                denom,
                amount,
            }) if from == authorization.wallet
                && to == request.disclosure.recipient
                && denom == request.disclosure.denom
                && amount == request.disclosure.amount => {}
            _ => return Err(KmsError::Refused),
        }
        let mut sends = self.inner.sends.lock().map_err(|_| KmsError::Unavailable)?;
        let journal = sends.as_mut().ok_or(KmsError::InvalidConfiguration)?;
        if let Some(raw) = journal.replay(payload, now)? {
            return Ok(raw);
        }
        let assertion = self
            .inner
            .assertions
            .lock()
            .map_err(|_| KmsError::Unavailable)?
            .remove(&owner)
            .ok_or(KmsError::Authentication)?;
        if assertion_subject(assertion.as_str()).ok().as_deref() != Some(owner.as_str()) {
            return Err(KmsError::Authentication);
        }
        let signer = self
            .signer(&key_id, public_key, binding.network_id())
            .and_then(|signer| signer.with_public_wallet(&owner, assertion.as_str()))
            .map_err(|error| {
                self.refused(&error);
                attestor_kms_failure(&error)
            })?;
        if signer.evm_address() != authorization.wallet {
            return Err(KmsError::Refused);
        }
        if let Some(raw) = journal.reserve(payload, now)? {
            return Ok(raw);
        }
        let transaction_signer = DisclosedAttestorSigner(signer);
        let signature = super::executor::poll_once_ready(sign_disclosed_call(
            &transaction_signer,
            &call,
            &disclosed,
            authorization.wallet,
        ))
        .map_err(|_| KmsError::Unavailable)?
        .map_err(|_| KmsError::Refused)?;
        if layerx_crypto::evm_transaction::recover_sender(&call, &signature)
            .map_err(|_| KmsError::InvalidResponse)?
            != authorization.wallet
        {
            return Err(KmsError::Integrity);
        }
        let raw_transaction = layerx_crypto::evm_transaction::assemble(&call, &signature);
        journal.complete(payload, raw_transaction)
    }

    fn sign_now(
        &self,
        binding: &PrincipalKeyBinding,
        reference: &ProviderKeyReference,
        request: ProviderSignRequest<'_>,
    ) -> Result<[u8; 65], CustodyError> {
        let (key_id, public_key, owner) =
            attestor_reference_parts(reference).map_err(CustodyError::Kms)?;
        if key_id != attestor_key_id(binding)
            || !layerx_crypto::ct::eq_fixed(&public_key, &request.expected_public_key())
        {
            return Err(CustodyError::Kms(KmsError::Integrity));
        }
        let assertion = self
            .inner
            .assertions
            .lock()
            .map_err(|_| CustodyError::Kms(KmsError::Unavailable))?
            .remove(&owner)
            .ok_or(CustodyError::Kms(KmsError::Authentication))?;
        let signer = self
            .signer(&key_id, public_key, binding.network_id())
            .map_err(attestor_custody_failure)?;
        let session = layerx_human_kms::attestor::new_session_id("activity")
            .map_err(attestor_custody_failure)?;
        let approval = layerx_human_kms::attestor::Approval {
            principal: &owner,
            session_id: &session,
            expires_at: request
                .disclosure()
                .expiry
                .not_after
                .min(request.disclosure().expiry.payload_expires_at),
        };
        let signature = signer
            .sign_activity(
                request.call(),
                request.disclosure(),
                request.from(),
                &approval,
                &assertion,
            )
            .map_err(|error| {
                self.refused(&error);
                attestor_custody_failure(error)
            })?;
        let signature = *signature.signature();
        let parsed = layerx_crypto::signer::AgentSignature::new(signature)
            .map_err(|_| CustodyError::Kms(KmsError::Integrity))?;
        if layerx_crypto::evm_transaction::recover_sender(request.call(), &parsed)
            .map_err(|_| CustodyError::Kms(KmsError::Integrity))?
            != request.from()
        {
            return Err(CustodyError::Kms(KmsError::Integrity));
        }
        Ok(signature)
    }
    fn sign_native_now(
        &self,
        binding: &PrincipalKeyBinding,
        reference: &ProviderKeyReference,
        request: crate::custody::ProviderNativeSignRequest<'_>,
    ) -> Result<[u8; 64], CustodyError> {
        let (key_id, public_key, owner) =
            attestor_reference_parts(reference).map_err(CustodyError::Kms)?;
        if key_id != attestor_key_id(binding)
            || binding.class() != KeyClass::HumanPrimary
            || !layerx_crypto::ct::eq_fixed(&public_key, &request.expected_public_key())
        {
            return Err(CustodyError::Kms(KmsError::Integrity));
        }
        request
            .consent()
            .validate_at(request.now_ms(), public_key)?;
        let assertion = self
            .inner
            .assertions
            .lock()
            .map_err(|_| CustodyError::Kms(KmsError::Unavailable))?
            .remove(&owner)
            .ok_or(CustodyError::Kms(KmsError::Authentication))?;
        if assertion_subject(assertion.as_str()).ok().as_deref() != Some(owner.as_str()) {
            return Err(CustodyError::Kms(KmsError::Authentication));
        }
        let signer = self
            .signer(&key_id, public_key, binding.network_id())
            .map_err(attestor_custody_failure)?;
        let session = layerx_human_kms::attestor::new_session_id("native-consent")
            .map_err(attestor_custody_failure)?;
        let signed = match request.consent() {
            crate::custody::NativeConsent::PreparationPurpose(purpose) => {
                signer.sign_native_preparation_purpose(purpose, &session, assertion.as_str())
            }
            crate::custody::NativeConsent::LocalGrant(grant) => {
                signer.sign_native_local_grant(grant, &session, assertion.as_str())
            }
            crate::custody::NativeConsent::SendPurpose(purpose) => {
                signer.sign_native_send_purpose(purpose, &session, assertion.as_str())
            }
        }
        .map_err(|error| {
            self.refused(&error);
            attestor_custody_failure(error)
        })?;
        Ok(*signed.signature())
    }
}

impl crate::custody::KmsProvider for AttestorKms {
    fn provider_reference(&self) -> &str {
        ATTESTOR_PROVIDER_REFERENCE
    }

    fn deployment(&self) -> ProviderDeployment {
        ProviderDeployment::Production
    }

    fn probe(&self) -> Result<(), KmsError> {
        let client = self
            .inner
            .config
            .client()
            .map_err(|error| attestor_kms_failure(&error))?;
        for signer in &self.inner.config.signers {
            let health =
                client
                    .health(signer)
                    .map_err(|error| match attestor_kms_failure(&error) {
                        KmsError::Authentication => KmsError::Authentication,
                        _ => KmsError::Unavailable,
                    })?;
            if !health.ready {
                return Err(KmsError::Unavailable);
            }
        }
        Ok(())
    }

    fn evm_operation(
        &self,
        operation: u8,
        binding: &PrincipalKeyBinding,
        reference: &ProviderKeyReference,
        payload: &[u8],
    ) -> Result<Vec<u8>, KmsError> {
        if operation == 6 {
            if !payload.is_empty() {
                return Err(KmsError::InvalidResponse);
            }
            let (key_id, public_key, owner) = attestor_reference_parts(reference)?;
            if key_id != attestor_key_id(binding) {
                return Err(KmsError::InvalidReference);
            }
            let assertion = self
                .inner
                .assertions
                .lock()
                .map_err(|_| KmsError::Unavailable)?
                .get(&owner)
                .cloned()
                .ok_or(KmsError::Authentication)?;
            return self
                .inner
                .config
                .client()
                .map_err(|error| attestor_kms_failure(&error))?
                .public_wallet_identity(&key_id, &public_key, &owner, assertion.as_str())
                .map(|address| address.to_vec())
                .map_err(|error| attestor_kms_failure(&error));
        }
        if operation != ATTESTOR_SEND_OPERATION {
            return Err(KmsError::Refused);
        }
        self.sign_send_now(binding, reference, payload)
    }

    fn create_key(
        &self,
        binding: &PrincipalKeyBinding,
    ) -> Result<ProviderKeyDescription, KmsError> {
        let (owner, account) = self
            .inner
            .enrolment
            .lock()
            .map_err(|_| KmsError::Unavailable)?
            .clone()
            .ok_or(KmsError::Refused)?;
        let key_id = attestor_key_id(binding);
        let client = self
            .inner
            .config
            .client()
            .map_err(|error| attestor_kms_failure(&error))?;
        let generated = client
            .generate_ed25519(&key_id, &owner, &account)
            .map_err(|error| attestor_kms_failure(&error))?;
        if generated.key_id != key_id {
            return Err(KmsError::InvalidResponse);
        }
        ProviderKeyDescription::new(
            attestor_reference(&key_id, generated.public_key, &owner)?,
            generated.public_key,
            binding.digest(),
            RotationState::Stable,
        )
    }

    fn describe_key(
        &self,
        binding: &PrincipalKeyBinding,
        reference: &ProviderKeyReference,
    ) -> Result<ProviderKeyDescription, KmsError> {
        let (key_id, public_key, _) = attestor_reference_parts(reference)?;
        if key_id != attestor_key_id(binding) {
            return Err(KmsError::Integrity);
        }
        ProviderKeyDescription::new(
            reference.clone(),
            public_key,
            binding.digest(),
            RotationState::Stable,
        )
    }

    fn rotate_key(
        &self,
        _binding: &PrincipalKeyBinding,
        _reference: &ProviderKeyReference,
    ) -> Result<ProviderKeyDescription, KmsError> {
        Err(KmsError::Refused)
    }

    fn destroy_key(
        &self,
        _binding: &PrincipalKeyBinding,
        _reference: &ProviderKeyReference,
    ) -> Result<(), KmsError> {
        Err(KmsError::Refused)
    }

    fn sign_native<'a>(
        &'a self,
        binding: &'a PrincipalKeyBinding,
        reference: &'a ProviderKeyReference,
        request: crate::custody::ProviderNativeSignRequest<'a>,
    ) -> std::pin::Pin<
        Box<dyn std::future::Future<Output = Result<[u8; 64], CustodyError>> + Send + 'a>,
    > {
        Box::pin(async move { self.sign_native_now(binding, reference, request) })
    }

    fn sign<'a>(
        &'a self,
        binding: &'a PrincipalKeyBinding,
        reference: &'a ProviderKeyReference,
        request: ProviderSignRequest<'a>,
    ) -> std::pin::Pin<
        Box<dyn std::future::Future<Output = Result<[u8; 65], CustodyError>> + Send + 'a>,
    > {
        Box::pin(async move { self.sign_now(binding, reference, request) })
    }
}

fn finality_host(url: &str) -> Result<String, String> {
    let refused = || "Paxeer finality requires valid independent HTTPS hosts".to_owned();
    let authority = url
        .strip_prefix("https://")
        .and_then(|rest| rest.split('/').next())
        .filter(|value| !value.is_empty() && !value.contains(['@', '?', '#']))
        .ok_or_else(refused)?;
    let (host, port) = if let Some(bracketed) = authority.strip_prefix('[') {
        let (host, suffix) = bracketed.split_once(']').ok_or_else(refused)?;
        host.parse::<std::net::Ipv6Addr>().map_err(|_| refused())?;
        (
            host,
            if suffix.is_empty() {
                None
            } else {
                Some(suffix.strip_prefix(':').ok_or_else(refused)?)
            },
        )
    } else {
        match authority.split_once(':') {
            Some((host, port)) => (host, Some(port)),
            None => (authority, None),
        }
    };
    if let Some(port) = port {
        if port.parse::<u16>().map_err(|_| refused())? == 0 {
            return Err(refused());
        }
    }
    let canonical = host.parse::<std::net::IpAddr>().map_or_else(
        |_| host.trim_end_matches('.').to_ascii_lowercase(),
        |address| address.to_string(),
    );
    if canonical.is_empty()
        || host.chars().any(char::is_whitespace)
        || host.contains(['%', '/', '\\'])
    {
        return Err(refused());
    }
    Ok(canonical)
}

#[cfg(test)]
mod finality_host_tests {
    use super::finality_host;

    #[test]
    fn endpoint_variants_are_not_independent_votes() -> Result<(), String> {
        for alias in [
            "https://RPC.EXAMPLE/one",
            "https://rpc.example.:443/two",
            "https://rpc.example:8443/three",
        ] {
            assert_eq!(finality_host(alias)?, "rpc.example");
        }
        assert_eq!(
            finality_host("https://[2001:0db8::1]:443/a")?,
            finality_host("https://[2001:db8::1]:8443/b")?
        );
        assert_ne!(
            finality_host("https://one.example/")?,
            finality_host("https://two.example/")?
        );
        for invalid in [
            "http://one.example",
            "https://user@one.example",
            "https://one.example:0",
            "https://[invalid]:443",
            "https://one.example:bad",
        ] {
            assert!(finality_host(invalid).is_err());
        }
        Ok(())
    }
}

fn movement_principal_account(
    scope: &crate::store::PrincipalScope<'_>,
) -> Result<(AgentDid, AccountId), ApiFailure> {
    let onboarding = OnboardingJourney::load(scope)
        .map_err(|_| ApiFailure::upstream_degraded())?
        .ok_or_else(ApiFailure::forbidden)?;
    let did = onboarding
        .did()
        .map_err(|_| ApiFailure::upstream_degraded())?;
    let did = std::str::from_utf8(did.as_bytes()).map_err(|_| ApiFailure::upstream_degraded())?;
    let actor = AgentDid::new(did).map_err(|_| ApiFailure::upstream_degraded())?;
    let account = AccountId::parse(&format!("agent:{did}:main"))
        .map_err(|_| ApiFailure::upstream_degraded())?;
    Ok((actor, account))
}

#[derive(serde::Serialize, serde::Deserialize)]
struct BrowserGrantPreparation {
    registration_payload: Vec<u8>,
    expiry_sequence: u64,
}

fn browser_grant_intent(
    scope: &mut crate::store::PrincipalScope<'_>,
    agent: &mut AgentRuntime,
    prepared: &crate::auth::PreparedBrowserSession,
    recovery_seed: [u8; 32],
    actor: &AgentDid,
    action: [u8; 32],
) -> Result<(Intent, [u8; 32]), ApiFailure> {
    let mut session_seed: [u8; 32] = Sha256::digest(
        [
            b"layerx-human/browser-session-signer/v1\0".as_slice(),
            recovery_seed.as_slice(),
        ]
        .concat(),
    )
    .into();
    let session_public_key = LocalSigner::new(session_seed).public_key();
    session_seed.fill(0);
    let did = layerx_types::ids::Did::new(actor.as_str().as_bytes())
        .map_err(|_| ApiFailure::upstream_degraded())?;
    let grantor = layerx_intents::canonical::did_id_for_protocol(&did, 3)
        .map_err(|_| ApiFailure::upstream_degraded())?;
    let not_before = prepared
        .opened_at()
        .checked_mul(1000)
        .ok_or_else(ApiFailure::upstream_degraded)?;
    let expires_at = prepared
        .refresh_expires_at()
        .checked_mul(1000)
        .ok_or_else(ApiFailure::upstream_degraded)?;
    let row_key = crate::store::RowKey::new(format!("browser-grant-{}", hex_bytes(&action)))
        .map_err(|_| ApiFailure::upstream_degraded())?;
    let plan: BrowserGrantPreparation =
        if let Some(row) = scope.get(crate::store::Table::Journeys, &row_key) {
            serde_json::from_slice(row.bytes()).map_err(|_| ApiFailure::upstream_degraded())?
        } else {
            let identity = agent
                .identity_resolve(actor.as_str())
                .map_err(agent_failure)?;
            let issued = issue_session_key(&SessionKeyRequest {
                fee_budget: None,
                purpose: layerx_crypto::session::SessionPurpose::Authentication,
                grantor,
                session_public_key,
                not_before,
                expires_at: Some(expires_at),
                permitted_activity_types: Vec::new(),
                revocation_sequence: Some(identity.revocation_sequence),
            })
            .map_err(|_| ApiFailure::upstream_degraded())?;
            let plan = BrowserGrantPreparation {
                registration_payload: issued.registration_payload,
                expiry_sequence: agent
                    .head()
                    .map_err(agent_failure)?
                    .chain_sequence
                    .checked_add(1024)
                    .ok_or_else(ApiFailure::upstream_degraded)?,
            };
            scope
                .put(
                    crate::store::Table::Journeys,
                    row_key,
                    prepared.opened_at(),
                    serde_json::to_vec(&plan).map_err(|_| ApiFailure::upstream_degraded())?,
                )
                .map_err(|_| ApiFailure::unavailable())?;
            plan
        };
    let issued = layerx_crypto::session::decode_session_key(&plan.registration_payload)
        .map_err(|_| ApiFailure::upstream_degraded())?;
    if issued.purpose != layerx_crypto::session::SessionPurpose::Authentication
        || issued.grantor != grantor
        || issued.session_public_key != session_public_key
        || issued.not_before != not_before
        || issued.expires_at != expires_at
        || plan.expiry_sequence == 0
    {
        return Err(ApiFailure::upstream_degraded());
    }
    let intent = Intent::v3(IntentKind::SessionGrant(
        ProtocolSessionGrant::new(plan.registration_payload, plan.expiry_sequence, action)
            .map_err(|_| ApiFailure::upstream_degraded())?,
    ));
    Ok((intent, issued.grant_id))
}

fn session_secrets(grant: &crate::auth::SessionGrant, now: u64) -> SessionSecrets {
    SessionSecrets {
        access_token: grant.access_token().expose().to_owned(),
        refresh_token: grant.refresh_token().expose().to_owned(),
        csrf_token: grant.csrf_token().expose().to_owned(),
        access_max_age_seconds: grant.access_expires_at().saturating_sub(now),
        refresh_max_age_seconds: grant.refresh_expires_at().saturating_sub(now),
    }
}

fn production_agent(
    socket: PathBuf,
    limits: Limits,
    clock: Arc<dyn Clock>,
) -> Result<(layerx_sdk::Client, AgentRuntime, [u8; 32]), String> {
    let agent_contract = layerx_sdk::Client::daemon(
        socket.clone(),
        layerx_agent_api::agent_api_schema_v1().version,
    )
    .map_err(|_| "agent SDK contract refused startup".to_owned())?;
    let mut agent = AgentRuntime::connect_with_clock(socket, limits, clock)
        .map_err(|_| "agent boundary refused startup".to_owned())?;
    let native_asset = agent
        .native_fee_policy()
        .map_err(|_| "authenticated native asset refused startup".to_owned())?
        .asset_id;
    Ok((agent_contract, agent, native_asset))
}

fn production_principal_store(
    root: PathBuf,
    retention: RetentionPolicy,
    tenancy_digest: [u8; 32],
    binding: layerx_identity_binding::Config,
) -> Result<Arc<Mutex<PrincipalStore>>, String> {
    let clock = layerx_client::runtime_clock::RuntimeClock::from_environment()
        .map_err(|_| "principal clock capability unavailable".to_owned())?;
    let provider = layerx_identity_binding::Client::new(binding, clock)
        .map_err(|_| "identity binding provider configuration refused".to_owned())?;
    PrincipalStore::open_with_authority(
        root,
        retention,
        TenancyDigest::new(tenancy_digest),
        Arc::new(IdentityTenancy(provider)),
    )
    .map(|store| Arc::new(Mutex::new(store)))
    .map_err(|_| "principal store refused startup".to_owned())
}

fn production_auth_index(root: PathBuf, key: [u8; 32]) -> Result<AuthDiscoveryIndex, String> {
    AuthDiscoveryIndex::open(
        root,
        IndexAuthenticationKey::new(key)
            .map_err(|_| "authentication index key is invalid".to_owned())?,
    )
    .map_err(|_| "authentication index refused startup".to_owned())
}

fn browser_device<'a>(request: &'a ScopedRequest<'_>) -> Result<(&'a str, &'a str), ApiFailure> {
    let device = request
        .body
        .get("device")
        .and_then(serde_json::Value::as_object)
        .ok_or_else(|| ApiFailure::invalid_request(Some("device")))?;
    let device_label = device
        .get("label")
        .and_then(serde_json::Value::as_str)
        .ok_or_else(|| ApiFailure::invalid_request(Some("device.label")))?;
    let device_platform = device
        .get("platform")
        .and_then(serde_json::Value::as_str)
        .ok_or_else(|| ApiFailure::invalid_request(Some("device.platform")))?;
    Ok((device_label, device_platform))
}

fn validate_resumed_session(
    prior_session: &layerx_crypto::session::SessionFeeState,
    did: &str,
) -> Result<(), ApiFailure> {
    if prior_session.revoked_at_sequence == 0
        || prior_session.grant.grantor
            != layerx_intents::canonical::did_id_for_protocol(
                &Did::new(did.as_bytes()).map_err(|_| ApiFailure::upstream_degraded())?,
                3,
            )
            .map_err(|_| ApiFailure::upstream_degraded())?
    {
        return Err(ApiFailure::upstream_degraded());
    }
    Ok(())
}

#[derive(Debug)]
struct IdentityTenancy(layerx_identity_binding::Client);

impl crate::store::PrincipalTenancyAuthority for IdentityTenancy {
    fn tenant_for(
        &self,
        principal: &crate::store::PrincipalId,
    ) -> Result<crate::store::AgentTenantId, crate::store::StoreError> {
        let binding = self.0.lookup(principal.as_str())?;
        if binding.principal() != principal.as_str() {
            return Err(crate::store::StoreError::InvalidPrincipal);
        }
        crate::store::AgentTenantId::new(binding.agent_tenant())
    }
}

fn principal_binding_configuration() -> Result<layerx_identity_binding::Config, String> {
    Ok(layerx_identity_binding::Config {
        socket: absolute("LAYERX_HUMAN_IDENTITY_BINDING_SOCKET")?,
        tenant: required("LAYERX_HUMAN_IDENTITY_BINDING_TENANT")?,
        peer_uid: number("LAYERX_HUMAN_IDENTITY_BINDING_PEER_UID")?,
        peer_gid: number("LAYERX_HUMAN_IDENTITY_BINDING_PEER_GID")?,
        deadline: Duration::from_secs(number("LAYERX_HUMAN_IDENTITY_BINDING_DEADLINE_SECONDS")?),
    })
}

fn intent_failure(refusal: crate::journeys::Refusal) -> ApiFailure {
    match refusal {
        crate::journeys::Refusal::PlanDigestMismatch
        | crate::journeys::Refusal::LegMismatch { .. }
        | crate::journeys::Refusal::UnboundLegs { .. } => ApiFailure::forbidden(),
        crate::journeys::Refusal::ZeroAmount
        | crate::journeys::Refusal::EndpointsIdentical
        | crate::journeys::Refusal::InvalidOwner
        | crate::journeys::Refusal::InvalidAllowance
        | crate::journeys::Refusal::InvalidAnnotation => ApiFailure::invalid_request(None),
        _ => ApiFailure::forbidden(),
    }
}

fn observation_failure(error: &crate::journeys::ObservationError) -> ApiFailure {
    match error {
        crate::journeys::ObservationError::Invalid(refusal) => intent_failure(refusal.clone()),
        crate::journeys::ObservationError::WalletNotBound
        | crate::journeys::ObservationError::IdentityMismatch => ApiFailure::forbidden(),
        _ => ApiFailure::upstream_degraded(),
    }
}

fn intent_endpoint(
    value: &serde_json::Value,
    field: &str,
) -> Result<crate::journeys::Endpoint, ApiFailure> {
    let document = value
        .get(field)
        .ok_or_else(|| ApiFailure::invalid_request(Some(field)))?;
    let kind = document
        .get("kind")
        .and_then(serde_json::Value::as_str)
        .ok_or_else(|| ApiFailure::invalid_request(Some(field)))?;
    if kind == "paxeer-wallet" {
        return Ok(crate::journeys::Endpoint::PaxeerWallet);
    }
    let canonical = document
        .get("account")
        .and_then(serde_json::Value::as_str)
        .ok_or_else(|| ApiFailure::invalid_request(Some(field)))?;
    let account =
        AccountId::parse(canonical).map_err(|_| ApiFailure::invalid_request(Some(field)))?;
    match kind {
        "human" => crate::journeys::Endpoint::human(account),
        "agent" => crate::journeys::Endpoint::agent(account),
        "agent-budget" => crate::journeys::Endpoint::agent_budget(account),
        _ => return Err(ApiFailure::invalid_request(Some(field))),
    }
    .map_err(|_| ApiFailure::invalid_request(Some(field)))
}

fn intent_endpoint_json(endpoint: &crate::journeys::Endpoint) -> serde_json::Value {
    match endpoint {
        crate::journeys::Endpoint::PaxeerWallet => json!({"kind": "paxeer-wallet"}),
        crate::journeys::Endpoint::Human(account) => {
            json!({"kind": "human", "account": account.canonical()})
        }
        crate::journeys::Endpoint::Agent(account) => {
            json!({"kind": "agent", "account": account.canonical()})
        }
        crate::journeys::Endpoint::AgentBudget(account) => {
            json!({"kind": "agent-budget", "account": account.canonical()})
        }
    }
}

fn intent_asset(value: &serde_json::Value) -> Result<AssetId, ApiFailure> {
    let text = text_field(value, "asset_id")?;
    if text.len() != 64 {
        return Err(ApiFailure::invalid_request(Some("asset_id")));
    }
    let mut asset = [0_u8; 32];
    for (slot, pair) in asset.iter_mut().zip(text.as_bytes().chunks_exact(2)) {
        let nibble = |byte: u8| match byte {
            b'0'..=b'9' => Some(byte - b'0'),
            b'a'..=b'f' => Some(byte - b'a' + 10),
            _ => None,
        };
        let high = nibble(pair[0]).ok_or_else(|| ApiFailure::invalid_request(Some("asset_id")))?;
        let low = nibble(pair[1]).ok_or_else(|| ApiFailure::invalid_request(Some("asset_id")))?;
        *slot = (high << 4) | low;
    }
    Ok(AssetId::new(asset))
}

fn intent_money_json(amount: u128, currency: &str) -> serde_json::Value {
    json!({"amount": amount.to_string(), "currency": currency})
}

fn supported_intent_endpoints(
    intent: &crate::journeys::UnifiedIntent,
) -> Result<(), crate::journeys::SubmitRefusal> {
    if intent.source() == &crate::journeys::Endpoint::PaxeerWallet
        || intent.destination() == &crate::journeys::Endpoint::PaxeerWallet
    {
        return Err(crate::journeys::SubmitRefusal::UnsupportedPlan);
    }
    Ok(())
}

fn intent_plan_json(
    plan: &crate::journeys::UnifiedPlan,
    currency: &str,
    transactions: &[crate::custody::EvmTransaction],
) -> serde_json::Value {
    let legs = plan
        .legs()
        .iter()
        .enumerate()
        .map(|(index, leg)| {
            json!({
                "index": index,
                "to": format!("0x{}", hex_bytes(&transactions[index].to)),
                "value": "0",
                "calldata": format!("0x{}", hex_bytes(&transactions[index].calldata)),
                "nonce": transactions[index].nonce,
                "chain_id": transactions[index].chain_id,
                "gas_limit": transactions[index].gas_limit,
                "max_fee_per_gas": transactions[index].max_fee_per_gas.to_string(),
                "max_priority_fee_per_gas": transactions[index].max_priority_fee_per_gas.to_string(),
                "mechanism": leg.mechanism().label(),
                "domain": match leg.mechanism().domain() {
                    crate::journeys::Domain::Paxeer => "paxeer",
                    crate::journeys::Domain::LayerX => "layerx",
                },
                "source": intent_endpoint_json(leg.source()),
                "destination": intent_endpoint_json(leg.destination()),
                "money": intent_money_json(leg.amount().value(), currency),
                "fee": intent_money_json(leg.fee(), currency),
            })
        })
        .collect::<Vec<_>>();
    let requirements = plan.signing_requirements().map_or_else(
        |_| Vec::new(),
        |list| {
            list.iter()
                .map(|requirement| {
                    json!({
                        "leg_index": requirement.leg_index(),
                        "action_key": hex_bytes(&requirement.action_key()),
                        "signing_context": hex_bytes(&requirement.signing_context()),
                        "authority": intent_authority_label(requirement.authority()),
                    })
                })
                .collect::<Vec<_>>()
        },
    );
    json!({
        "plan_digest": hex_bytes(&plan.digest()),
        "journey_kind": intent_journey_kind_label(plan.journey_kind()),
        "total_fee": intent_money_json(plan.total_fee(), currency),
        "legs": legs,
        "signing_requirements": requirements,
    })
}

const fn intent_authority_label(authority: crate::journeys::RequiredAuthority) -> &'static str {
    crate::journeys::authority_label(authority)
}

const fn intent_journey_kind_label(kind: crate::journeys::JourneyKind) -> &'static str {
    match kind {
        crate::journeys::JourneyKind::Onboarding => "onboarding",
        crate::journeys::JourneyKind::Move => "move",
        crate::journeys::JourneyKind::AgentCreate => "agent-create",
        crate::journeys::JourneyKind::AgentFund => "agent-fund",
        crate::journeys::JourneyKind::AgentPause => "agent-pause",
        crate::journeys::JourneyKind::AgentRetire => "agent-retire",
    }
}

fn native_access_row(body: &serde_json::Value) -> Result<RowKey, ApiFailure> {
    let id = text_field(body, "access_id")?;
    native_access_hex(id)?;
    RowKey::new(format!("native-send-public-disclosure-{id}"))
        .map_err(|_| ApiFailure::unavailable())
}
fn native_access_hex(text: &str) -> Result<[u8; 32], ApiFailure> {
    if text.len() != 64
        || !text
            .bytes()
            .all(|b| b.is_ascii_digit() || (b'a'..=b'f').contains(&b))
    {
        return Err(ApiFailure::invalid_request(None));
    }
    let mut bytes = [0; 32];
    for (i, byte) in bytes.iter_mut().enumerate() {
        *byte = u8::from_str_radix(&text[i * 2..i * 2 + 2], 16)
            .map_err(|_| ApiFailure::invalid_request(None))?;
    }
    if bytes == [0; 32] {
        return Err(ApiFailure::invalid_request(None));
    }
    Ok(bytes)
}
fn native_access_public_body(body: &serde_json::Value) -> Result<serde_json::Value, ApiFailure> {
    let mut body = body.clone();
    let object = body
        .as_object_mut()
        .ok_or_else(|| ApiFailure::invalid_request(None))?;
    object.remove("step_up");
    Ok(body)
}
pub(super) fn native_access_request(
    scope: &crate::store::PrincipalScope<'_>,
    body: &serde_json::Value,
) -> Result<super::native_send::NativeSendAccessRequest, ApiFailure> {
    let row = scope
        .get(Table::Journeys, &native_access_row(body)?)
        .ok_or_else(ApiFailure::forbidden)?;
    let retained: serde_json::Value =
        serde_json::from_slice(row.bytes()).map_err(|_| ApiFailure::unavailable())?;
    if retained["body"] != native_access_public_body(body)? {
        return Err(ApiFailure::forbidden());
    }
    let public = &retained["body"];
    let number = |name| -> Result<u64, ApiFailure> {
        u64::try_from(canonical_fee_amount(public, name)?)
            .map_err(|_| ApiFailure::invalid_request(Some(name)))
    };
    let fee = if let Some(value) = public.get("native_fee_budget") {
        Some(layerx_crypto::authority_grant::NativeFeeBudget {
            asset: native_access_hex(text_field(value, "asset_id")?)?,
            maximum_per_activity: canonical_fee_amount(value, "maximum_per_activity")?,
            maximum_total: canonical_fee_amount(value, "maximum_total")?,
            maximum_per_period: canonical_fee_amount(value, "maximum_per_period")?,
            period_length: u64::try_from(canonical_fee_amount(value, "period_length_ms")?)
                .map_err(|_| ApiFailure::invalid_request(None))?,
            period_start: retained["issued_at_ms"]
                .as_u64()
                .ok_or_else(ApiFailure::unavailable)?,
        })
    } else {
        None
    };
    let request = super::native_send::NativeSendAccessRequest {
        action_key: native_access_hex(text_field(public, "access_id")?)?,
        owner: Did::new(text_field(&retained, "owner")?.as_bytes())
            .map_err(|_| ApiFailure::forbidden())?,
        owner_public_key: native_access_hex(text_field(&retained, "owner_public_key")?)?,
        not_before: number("not_before")?,
        expires_at: number("expires_at")?,
        native_fee_budget: fee,
        counterparty: native_access_hex(text_field(public, "counterparty")?)?,
        asset: native_access_hex(text_field(public, "asset")?)?,
        maximum_amount: canonical_fee_amount(public, "maximum_amount")?,
        rate_window_ms: number("rate_window_ms")?,
        maximum_uses: number("maximum_uses")?,
        commitment: native_access_hex(text_field(public, "commitment")?)?,
    };
    request
        .validate()
        .map_err(agent_failure_from_creation_contract)?;
    Ok(request)
}
impl ProductionComponents {
    fn execute_native_send_access_begin(
        &self,
        request: &ScopedRequest<'_>,
        scope: &mut crate::store::PrincipalScope<'_>,
    ) -> Result<BackendResponse, ApiFailure> {
        let now = self.now()?;
        let mut runtime = self.principal_agent(scope)?;
        let owner = resolve_principal_owner(self, scope, &mut runtime)?;
        let fee_policy = runtime.native_fee_policy().map_err(agent_failure)?;
        native_fee_consent(&request.body, &fee_policy)?;
        let owner_key = native_access_hex(owner.authority.as_str())?;
        let row_key = native_access_row(&request.body)?;
        let public = native_access_public_body(&request.body)?;
        if let Some(row) = scope.get(Table::Journeys, &row_key) {
            let stored: serde_json::Value =
                serde_json::from_slice(row.bytes()).map_err(|_| ApiFailure::unavailable())?;
            if stored["body"] != public
                || stored["owner"] != owner.actor.as_str()
                || stored["owner_public_key"] != hex_bytes(&owner_key)
            {
                return Err(ApiFailure::forbidden());
            }
        } else {
            let retained = json!({"body":public,"owner":owner.actor.as_str(),"owner_public_key":hex_bytes(&owner_key),"issued_at_ms":now.checked_mul(1000).ok_or_else(ApiFailure::unavailable)?});
            scope
                .put(
                    Table::Journeys,
                    row_key,
                    now,
                    serde_json::to_vec(&retained).map_err(|_| ApiFailure::unavailable())?,
                )
                .map_err(|_| ApiFailure::unavailable())?;
        }
        let access = native_access_request(scope, &request.body)?;
        if now < access.not_before || now >= access.expires_at {
            return Err(ApiFailure::forbidden());
        }
        let digest = super::native_send::access_operation(scope, &access)
            .map_err(agent_failure_from_creation_contract)?;
        Ok(BackendResponse {
            result: json!({"access_id":text_field(&request.body,"access_id")?,"confirms":digest.to_schema()}),
            session: None,
        })
    }
    fn execute_native_send_access_confirm(
        &self,
        request: &ScopedRequest<'_>,
        scope: &mut crate::store::PrincipalScope<'_>,
    ) -> Result<BackendResponse, ApiFailure> {
        let now = self.now()?;
        let access = native_access_request(scope, &request.body)?;
        let operation = super::native_send::access_operation(scope, &access)
            .map_err(agent_failure_from_creation_contract)?;
        let challenge = request
            .body
            .get("step_up")
            .and_then(|v| v.get("challenge_id"))
            .and_then(serde_json::Value::as_str)
            .ok_or_else(ApiFailure::forbidden)?;
        let evidence = self
            .passkeys
            .load_step_up_evidence(scope, challenge, now)
            .map_err(|_| ApiFailure::forbidden())?;
        let context = request
            .principal
            .as_ref()
            .ok_or_else(ApiFailure::forbidden)?;
        self.passkeys
            .revalidate_step_up(scope, &evidence, operation, now)
            .map_err(|_| ApiFailure::forbidden())?;
        let retained_key = RowKey::new(format!(
            "human-native-send-access-{}",
            hex_bytes(&access.action_key)
        ))
        .map_err(|_| ApiFailure::unavailable())?;
        if scope.get(Table::Journeys, &retained_key).is_some() {
            let mut runtime = self.principal_agent(scope)?;
            let retained =
                super::native_send::load_access(scope, &mut runtime, access.action_key, now)
                    .map_err(agent_failure_from_creation_contract)?;
            if retained.commitment != access.commitment
                || retained.session.owner.as_bytes() != access.owner.as_bytes()
                || retained.session.owner_public_key != access.owner_public_key
                || retained.capability_id != access.action_key
            {
                return Err(ApiFailure::forbidden());
            }
            return Ok(BackendResponse {
                result: json!({"native_access_id":hex_bytes(&retained.capability_id)}),
                session: None,
            });
        }
        let trace =
            TraceId::parse(&request.trace).map_err(|_| ApiFailure::invalid_request(None))?;
        let result = super::executor::poll_once_ready(self.provision_native_send_access(
            scope,
            &evidence,
            operation,
            context.request_digest(),
            &trace,
            &access,
        ))
        .map_err(|_| ApiFailure::upstream_degraded())??;
        Ok(BackendResponse {
            result: json!({"native_access_id":hex_bytes(&result.capability_id)}),
            session: None,
        })
    }
}

impl ProductionComponents {
    fn bind_native_journey_access(
        &self,
        scope: &mut crate::store::PrincipalScope<'_>,
        agent: &mut AgentRuntime,
        journey: &crate::notify::JourneyId,
        selected: Option<&str>,
    ) -> Result<(), ApiFailure> {
        let key = RowKey::new(format!("native-send-journey-access-{}", journey.as_str()))
            .map_err(|_| ApiFailure::unavailable())?;
        let retained = scope
            .get(Table::Journeys, &key)
            .map(|row| row.bytes().to_vec());
        let (access_id, expiry) = if let Some(bytes) = retained {
            let saved: serde_json::Value =
                serde_json::from_slice(&bytes).map_err(|_| ApiFailure::unavailable())?;
            let id = text_field(&saved, "access_id")?.to_owned();
            if selected.is_some_and(|actual| actual != id) {
                return Err(ApiFailure::forbidden());
            }
            (
                id,
                saved["purpose_expires_at_ms"]
                    .as_u64()
                    .ok_or_else(ApiFailure::unavailable)?,
            )
        } else if let Some(id) = selected {
            let context =
                super::native_send::load_access(scope, agent, native_access_hex(id)?, self.now()?)
                    .map_err(agent_failure_from_creation_contract)?;
            let expiry = context.expires_at_ms;
            let bytes = serde_json::to_vec(&json!({"access_id":id,"purpose_expires_at_ms":expiry}))
                .map_err(|_| ApiFailure::unavailable())?;
            scope
                .put(Table::Journeys, key, self.now()?, bytes)
                .map_err(|_| ApiFailure::unavailable())?;
            (id.to_owned(), expiry)
        } else {
            return Ok(());
        };
        let context = super::native_send::load_access(
            scope,
            agent,
            native_access_hex(&access_id)?,
            self.now()?,
        )
        .map_err(agent_failure_from_creation_contract)?;
        agent
            .bind_native_journey_context(context, expiry)
            .map_err(agent_failure)
    }
}

fn intent_node() -> Result<layerx_agentd::node_rpc::NodeRpc, ApiFailure> {
    let config = layerx_agentd::node_rpc::NodeRpcConfig::from_environment()
        .map_err(|_| ApiFailure::upstream_degraded())?;
    layerx_agentd::node_rpc::NodeRpc::connect(&config).map_err(|_| ApiFailure::upstream_degraded())
}
fn precompile(id: u16) -> [u8; 20] {
    let mut address = [0; 20];
    address[18..].copy_from_slice(&id.to_be_bytes());
    address
}
fn evm_call(tx: &crate::custody::EvmTransaction) -> Eip1559Call {
    Eip1559Call {
        chain_id: tx.chain_id,
        nonce: tx.nonce,
        max_priority_fee_per_gas: tx.max_priority_fee_per_gas.into(),
        max_fee_per_gas: tx.max_fee_per_gas.into(),
        gas_limit: tx.gas_limit,
        to: tx.to,
        data: tx.calldata.clone(),
    }
}
fn abi_amount(amount: u128) -> [u8; 32] {
    let mut word = [0; 32];
    word[16..].copy_from_slice(&amount.to_be_bytes());
    word
}
fn budget_defund_calldata(budget: [u8; 32], amount: u128) -> Vec<u8> {
    [
        ledger_selector("budgetDefund(bytes32,uint128)").to_vec(),
        budget.to_vec(),
        abi_amount(amount).to_vec(),
    ]
    .concat()
}
fn send_leg_denom(
    assets: &layerx_network_gateway::AssetTable,
    balances: &layerx_network_gateway::AccountBalances,
    asset: AssetId,
) -> Result<String, ApiFailure> {
    let mut matches = assets.assets.iter().filter_map(|entry| {
        let denom = entry.denom.as_ref()?;
        let identifier: [u8; 32] = Sha256::digest(denom.as_bytes()).into();
        (entry.asset_id == format!("denom:{denom}") && identifier == asset.bytes()).then_some(denom)
    });
    let denom = matches.next().ok_or_else(ApiFailure::forbidden)?;
    if matches.next().is_some() {
        return Err(ApiFailure::forbidden());
    }
    balances.asset(denom).ok_or_else(ApiFailure::forbidden)?;
    Ok(denom.clone())
}

fn bank_send_calldata(from: [u8; 20], to: [u8; 20], denom: &str, amount: u128) -> Vec<u8> {
    let mut source = [0; 32];
    source[12..].copy_from_slice(&from);
    let mut recipient = [0; 32];
    recipient[12..].copy_from_slice(&to);
    let mut data = [
        vec![0x5c, 0x05, 0x96, 0x1b],
        source.to_vec(),
        recipient.to_vec(),
        abi_amount(128).to_vec(),
        abi_amount(amount).to_vec(),
        abi_amount(denom.len() as u128).to_vec(),
        denom.as_bytes().to_vec(),
    ]
    .concat();
    data.resize(4 + (data.len() - 4).div_ceil(32) * 32, 0);
    data
}

fn intent_account(endpoint: &crate::journeys::Endpoint) -> Result<&AccountId, ApiFailure> {
    match endpoint {
        crate::journeys::Endpoint::Human(account)
        | crate::journeys::Endpoint::Agent(account)
        | crate::journeys::Endpoint::AgentBudget(account) => Ok(account),
        _ => Err(ApiFailure::forbidden()),
    }
}

#[derive(Debug)]
struct DisclosedAttestorSigner(layerx_human_kms::attestor::AttestorSigner);
impl layerx_crypto::signer::Signer for DisclosedAttestorSigner {
    fn public_key(&self) -> [u8; 32] {
        self.0.public_key()
    }
    fn evm_address(&self) -> Option<[u8; 20]> {
        Some(self.0.evm_address())
    }
    fn sign<'a>(
        &'a self,
        _: layerx_crypto::signer::SigningRequest<'a>,
    ) -> layerx_crypto::signer::SignFuture<'a> {
        Box::pin(std::future::ready(Err(
            layerx_crypto::signer::SignError::RemoteRefused,
        )))
    }
    fn sign_transaction<'a>(
        &'a self,
        call: &'a Eip1559Call,
    ) -> layerx_crypto::signer::TransactionSignFuture<'a> {
        Box::pin(async move {
            let mut signature = self
                .0
                .sign_evm_transaction(
                    &layerx_human_kms::attestor::unsigned_evm_transaction(call),
                    call.chain_id,
                )
                .map_err(|_| layerx_crypto::signer::SignError::RemoteRefused)?;
            signature[64] += 27;
            layerx_crypto::signer::AgentSignature::new(signature)
                .map_err(|_| layerx_crypto::signer::SignError::ReturnedSignatureInvalid)
        })
    }
}

impl ProductionComponents {
    fn submit_budget_defund(
        &self,
        scope: &mut crate::store::PrincipalScope<'_>,
        agent: &mut AgentRuntime,
        context: &super::agent_runtime::AgentLifecycleContext,
        amount: u128,
        action_key: [u8; 32],
        trace: &TraceId,
    ) -> Result<crate::agents::ProtocolEvidence, ApiFailure> {
        let key =
            KeyId::new(context.seed.custody_key.clone()).map_err(|_| ApiFailure::forbidden())?;
        let wallet = self
            .custody
            .evm_wallet(scope.principal(), &key)
            .map_err(|_| ApiFailure::forbidden())?;
        let node = intent_node()?;
        let row_key = RowKey::new(format!("defund-signed-{}", hex_bytes(&action_key)))
            .map_err(|_| ApiFailure::unavailable())?;
        let raw = if let Some(row) = scope.get(Table::Journeys, &row_key) {
            row.bytes().to_vec()
        } else {
            let nonce = node
                .pending_nonce(&wallet)
                .map_err(|_| ApiFailure::upstream_degraded())?;
            let call = Eip1559Call {
                chain_id: u64::from(self.network_id),
                nonce,
                max_priority_fee_per_gas: self.evm_max_priority_fee_per_gas.into(),
                max_fee_per_gas: self.evm_max_fee_per_gas.into(),
                gas_limit: self.evm_gas_limit,
                to: precompile(0x101b),
                data: budget_defund_calldata(context.active_budget_id, amount),
            };
            let disclosure = bind_call(&call, wallet).map_err(|_| ApiFailure::forbidden())?;
            let unsigned = layerx_human_kms::attestor::unsigned_evm_transaction(&call);
            let principal = scope.principal().clone();
            let signed = super::executor::poll_once_ready(self.custody.sign_in_scope(
                scope,
                crate::custody::SignRequest::new(
                    &principal,
                    &key,
                    trace,
                    crate::custody::SignAuthorization::new(
                        crate::custody::Operation::ProtocolMutation,
                        None,
                    ),
                    &unsigned,
                    &disclosure,
                    self.now()?,
                ),
            ))
            .map_err(|_| ApiFailure::upstream_degraded())?
            .map_err(|_| ApiFailure::forbidden())?;
            let signature = layerx_crypto::signer::AgentSignature::new(*signed.signature())
                .map_err(|_| ApiFailure::forbidden())?;
            let raw = assemble(&call, &signature);
            scope
                .put(Table::Journeys, row_key, self.now()?, raw.clone())
                .map_err(|_| ApiFailure::unavailable())?;
            raw
        };
        let (call, signature) = layerx_crypto::evm_transaction::decode_signed(&raw)
            .map_err(|_| ApiFailure::forbidden())?;
        if call.data != budget_defund_calldata(context.active_budget_id, amount)
            || call.to != precompile(0x101b)
            || call.chain_id != u64::from(self.network_id)
            || layerx_crypto::evm_transaction::recover_sender(&call, &signature)
                .map_err(|_| ApiFailure::forbidden())?
                != wallet
        {
            return Err(ApiFailure::forbidden());
        }
        let hash = transaction_hash(&raw);
        let material = match agent
            .receipt_by_idempotency_key([0; 32], hash)
            .map_err(agent_failure)?
        {
            crate::journeys::ReceiptLookup::Found(material) => material,
            crate::journeys::ReceiptLookup::Absent
            | crate::journeys::ReceiptLookup::Submitted { .. } => {
                match node.send_raw_transaction(&raw) {
                    Ok(accepted) if accepted == hash => {}
                    Err(layerx_agentd::node_rpc::NodeRpcError::Sequencer(
                        layerx_client::sequencer::SequencerError::Rpc(refusal),
                    )) if refusal.code == -32000
                        && refusal
                            .message
                            .starts_with("same nonce already receipted or held:") => {}
                    _ => return Err(ApiFailure::upstream_degraded()),
                }
                match agent
                    .receipt_by_idempotency_key([0; 32], hash)
                    .map_err(agent_failure)?
                {
                    crate::journeys::ReceiptLookup::Found(material) => material,
                    crate::journeys::ReceiptLookup::Absent
                    | crate::journeys::ReceiptLookup::Submitted { .. } => {
                        return Err(ApiFailure::upstream_degraded())
                    }
                }
            }
        };
        let mut owner = [0; 32];
        owner[12..].copy_from_slice(&wallet);
        bind_call(&call, wallet).map_err(|_| ApiFailure::forbidden())?;
        ProductionAgentCreation::protocol_evidence(raw, owner, agent.registry(), material)
            .map_err(|_| ApiFailure::upstream_degraded())
    }
}

fn verify_move_evm_outcome(
    evidence: &crate::agents::ProtocolEvidence,
    expected: layerx_types::payload::ActivityType,
) -> Result<layerx_wire::receipt::Receipt, ApiFailure> {
    super::agent_creation::verify_evm_outcome(evidence, expected)
        .map_err(|_| ApiFailure::upstream_degraded())
}

impl ProductionComponents {
    fn advance_evm_intent(
        &self,
        scope: &mut crate::store::PrincipalScope<'_>,
        id: &crate::notify::JourneyId,
        now: u64,
    ) -> Result<bool, ApiFailure> {
        let key = RowKey::new(format!("intent-evm-{}", id.as_str()))
            .map_err(|_| ApiFailure::unavailable())?;
        let mut record: serde_json::Value = serde_json::from_slice(
            scope
                .get(Table::Journeys, &key)
                .ok_or_else(ApiFailure::not_found)?
                .bytes(),
        )
        .map_err(|_| ApiFailure::unavailable())?;
        if matches!(record["state"].as_str(), Some("done" | "refused")) {
            return Ok(true);
        }
        let node = intent_node()?;
        let mut agent = self.principal_agent(scope)?;
        let mut complete = true;
        let mut refused = false;
        let leg_count = record["legs"]
            .as_array()
            .ok_or_else(ApiFailure::unavailable)?
            .len();
        for index in 0..leg_count {
            let leg = &mut record["legs"][index];
            if leg["verified"] == true {
                continue;
            }
            let raw: Vec<u8> = serde_json::from_value(leg["raw_transaction"].clone())
                .map_err(|_| ApiFailure::unavailable())?;
            let hash = transaction_hash(&raw);
            if leg["submitted"] != true {
                let retained_hash = leg["tx_hash"] == format!("0x{}", hex_bytes(&hash));
                match node.send_raw_transaction(&raw) {
                    Ok(accepted) if accepted == hash => {}
                    Err(layerx_agentd::node_rpc::NodeRpcError::Sequencer(
                        layerx_client::sequencer::SequencerError::Rpc(refusal),
                    )) if retained_hash
                        && refusal.code == -32000
                        && refusal
                            .message
                            .starts_with("same nonce already receipted or held:") => {}
                    _ => return Err(ApiFailure::upstream_degraded()),
                }
                let nonce = match leg.get("nonce") {
                    Some(nonce) => nonce.as_u64().ok_or_else(ApiFailure::unavailable)?,
                    None => {
                        decode_signed(&raw)
                            .map_err(|_| ApiFailure::unavailable())?
                            .0
                            .nonce
                    }
                };
                leg["submitted"] = json!(true);
                leg["tx_hash"] = json!(format!("0x{}", hex_bytes(&hash)));
                leg["nonce"] = json!(nonce);
                record["state"] = json!("still-checking");
                record["state_copy_key"] = json!("journey.state.still-checking");
                record["updated_at"] = json!(crate::time::rfc3339(now));
                scope
                    .put(
                        Table::Journeys,
                        key.clone(),
                        now,
                        serde_json::to_vec(&record).map_err(|_| ApiFailure::unavailable())?,
                    )
                    .map_err(|_| ApiFailure::unavailable())?;
            }
            let leg = &mut record["legs"][index];
            let (call, signature) = layerx_crypto::evm_transaction::decode_signed(&raw)
                .map_err(|_| ApiFailure::unavailable())?;
            if leg
                .get("nonce")
                .is_some_and(|nonce| nonce.as_u64() != Some(call.nonce))
            {
                return Err(ApiFailure::upstream_degraded());
            }
            leg["nonce"] = json!(call.nonce);
            let wallet = layerx_crypto::evm_transaction::recover_sender(&call, &signature)
                .map_err(|_| ApiFailure::unavailable())?;
            let disclosure = bind_call(&call, wallet).map_err(|_| ApiFailure::unavailable())?;
            let material = match agent.receipt_by_idempotency_key(disclosure.idempotency_key, hash)
            {
                Ok(crate::journeys::ReceiptLookup::Found(material))
                    if material.rung == Rung::Committed =>
                {
                    material
                }
                Ok(crate::journeys::ReceiptLookup::Submitted { tx_hash, nonce }) => {
                    if tx_hash != hash || nonce != call.nonce {
                        return Err(ApiFailure::upstream_degraded());
                    }
                    leg["tx_hash"] = json!(format!("0x{}", hex_bytes(&tx_hash)));
                    complete = false;
                    break;
                }
                Ok(_) | Err(crate::journeys::AgentBoundaryError::Unavailable) => {
                    complete = false;
                    break;
                }
                Err(error) => {
                    eprintln!("intent EVM receipt lookup failed: {error:?}");
                    return Err(agent_failure(error));
                }
            };
            let mut owner = [0; 32];
            owner[12..].copy_from_slice(&wallet);
            let evidence =
                ProductionAgentCreation::protocol_evidence(raw, owner, agent.registry(), material)
                    .map_err(|_| ApiFailure::upstream_degraded())?;
            let receipt = verify_move_evm_outcome(&evidence, disclosure.activity_type)?;
            if receipt
                .protocol()
                .ok_or_else(ApiFailure::upstream_degraded)?
                .result_code()
                != 0
            {
                leg["refused"] = json!(true);
                leg["receipt_hash"] = json!(format!("0x{}", hex_bytes(&evidence.receipt_hash)));
                leg["receipt"] = json!(evidence.receipt_bytes);
                refused = true;
                break;
            }
            leg["verified"] = json!(true);
            leg["receipt_hash"] = json!(format!("0x{}", hex_bytes(&evidence.receipt_hash)));
            leg["receipt"] = json!(evidence.receipt_bytes);
        }
        let state = if refused {
            "refused"
        } else if complete {
            "done"
        } else {
            "still-checking"
        };
        record["state"] = json!(state);
        record["state_copy_key"] = json!(format!("journey.state.{state}"));
        record["updated_at"] = json!(crate::time::rfc3339(now));
        scope
            .put(
                Table::Journeys,
                key,
                now,
                serde_json::to_vec(&record).map_err(|_| ApiFailure::unavailable())?,
            )
            .map_err(|_| ApiFailure::unavailable())?;
        Ok(complete || refused)
    }
}

#[cfg(test)]
mod intent_bank_golden_tests {
    use super::*;
    use crate::journeys::{
        BalanceEntry, Endpoint, FeeSchedule, LegMechanism, Mechanism, ObservedState,
    };
    use layerx_crypto::evm_transaction::decode_unsigned;
    use serde_json::Value;

    macro_rules! golden {
        ($name:literal) => {
            serde_json::from_str::<Value>(include_str!(concat!(
                env!("CARGO_MANIFEST_DIR"),
                "/../../schema/human-api/golden/",
                $name
            )))
            .expect("golden JSON")
        };
    }

    #[test]
    fn intent_bank_golden_request_produces_exact_eip1559_response() {
        let request = golden!("intent.plan.request.json");
        let expected = golden!("intent.plan.response.json");
        let context = golden!("producer/intent.plan.context.json");
        let body: Value = serde_json::from_slice(
            &serde_json::to_vec(&request["body"]).expect("request transport"),
        )
        .expect("request consumer");
        let asset = intent_asset(&body).expect("asset");
        let intent = ProductionComponents::intent_from_request(&body, asset).expect("intent");
        supported_intent_endpoints(&intent).expect("supported bank endpoints");
        let owner = AccountId::parse(context["owner_account"].as_str().expect("owner"))
            .expect("owner account");
        let transaction: crate::custody::EvmTransaction =
            serde_json::from_value(context["transaction"].clone()).expect("transaction");
        let observed = ObservedState::new(
            TimestampSeconds::from_u64(
                crate::time::seconds_from_rfc3339(
                    context["observed_at"].as_str().expect("observation time"),
                )
                .expect("observation timestamp"),
            ),
            owner.clone(),
            None,
            vec![BalanceEntry::new(
                Endpoint::human(owner).expect("home endpoint"),
                asset,
                ProtocolAmount::from_u128(
                    context["balance"]["amount"]
                        .as_str()
                        .expect("balance")
                        .parse()
                        .expect("amount"),
                ),
            )],
            Vec::new(),
            Vec::new(),
            FeeSchedule::new(vec![(
                LegMechanism::Protocol(Mechanism::Send),
                u128::from(transaction.gas_limit) * u128::from(transaction.max_fee_per_gas),
            )])
            .expect("fee schedule"),
        )
        .expect("observed bank balance");
        let plan = crate::journeys::plan(&intent, &observed).expect("production plan");
        assert_eq!(plan.legs().len(), 1);
        let sender =
            decode_hex_20(context["sender"].as_str().expect("sender")).expect("sender address");
        let recipient = decode_hex_20(context["recipient"].as_str().expect("recipient"))
            .expect("recipient address");
        assert_eq!(transaction.chain_id, 125);
        assert_eq!(transaction.to, precompile(0x1001));
        assert_eq!(transaction.value, [0; 32]);
        assert_eq!(
            transaction.calldata,
            bank_send_calldata(
                sender,
                recipient,
                context["denom"].as_str().expect("denom"),
                plan.legs()[0].amount().value()
            )
        );
        assert_eq!(
            context["nonce_observation"]["params"],
            json!([context["sender"], "pending"])
        );
        assert_eq!(
            transaction.nonce,
            u64::from_str_radix(
                context["nonce_observation"]["result"]
                    .as_str()
                    .expect("pending nonce")
                    .strip_prefix("0x")
                    .expect("hex nonce"),
                16
            )
            .expect("nonce")
        );
        let call = evm_call(&transaction);
        let decoded = decode_unsigned(&call.encode_unsigned()).expect("EIP-1559 consumer");
        assert_eq!(decoded, call);
        bind_call(&decoded, sender).expect("bank signing disclosure");
        let actual = intent_plan_json(&plan, "LXP", &[transaction]);
        let transported: Value =
            serde_json::from_slice(&serde_json::to_vec(&actual).expect("response transport"))
                .expect("response consumer");
        assert_eq!(transported, expected["body"]["result"]);
        assert_eq!(
            actual["signing_requirements"]
                .as_array()
                .expect("signatures")
                .len(),
            1
        );
    }

    #[test]
    fn intent_bank_golden_retired_wallet_is_unsupported_in_both_directions() {
        let request = golden!("negative/intent.plan.paxeer-wallet.request.json");
        let expected = golden!("negative/intent.plan.paxeer-wallet.response.json");
        let mut body = request["body"].clone();
        for reverse in [false, true] {
            if reverse {
                let source = body["source"].clone();
                body["source"] = body["destination"].clone();
                body["destination"] = source;
            }
            let intent = ProductionComponents::intent_from_request(
                &body,
                intent_asset(&body).expect("historical asset"),
            )
            .expect("historical request decoding");
            let refusal = supported_intent_endpoints(&intent).expect_err("retired endpoint");
            assert_eq!(refusal, crate::journeys::SubmitRefusal::UnsupportedPlan);
            let error = submit_failure(refusal);
            assert_eq!(json!(error.status), expected["status"]);
            assert_eq!(
                json!({"code": error.code, "copy_key": error.copy_key,
                "retry": error.retry}),
                expected["body"]["error"]
            );
            assert_eq!(error.field, None);
            assert_eq!(error.retry_after_ms, None);
        }
    }

    #[test]
    fn intent_bank_golden_journey_uses_the_route_projection() {
        let record = golden!("producer/journey.get.record.json");
        let expected = golden!("journey.get.response.json");
        let stored: Value =
            serde_json::from_slice(&serde_json::to_vec(&record).expect("stored projection input"))
                .expect("stored record consumer");
        let actual = super::super::production_reads::intent_journey_json(&stored)
            .expect("production journey route projection");
        assert_eq!(actual, expected["body"]["result"]);
        for (verified, refused, class, verification) in [
            (false, false, "submission-record", "unverified"),
            (true, false, "layerx-receipt", "receipt-verified"),
            (false, true, "layerx-receipt", "receipt-verified"),
        ] {
            let mut input = stored.clone();
            input["legs"][0]["verified"] = json!(verified);
            input["legs"][0]["refused"] = json!(refused);
            let projected = super::super::production_reads::intent_journey_json(&input)
                .expect("current evidence projection");
            assert_eq!(projected["evidence"][0]["class"], class);
            assert_eq!(projected["evidence"][0]["verification"], verification);
            let wire = serde_json::to_string(&projected).expect("journey transport");
            for retired in ["paxeer-finality", "paxeer-finalised", "settlement-anchored"] {
                assert!(!wire.contains(retired));
            }
            assert!(projected.get("raw_transaction").is_none());
        }
        for invalid in [
            Value::Null,
            json!("0x12"),
            json!(format!("0x{}", "gg".repeat(32))),
        ] {
            let mut input = stored.clone();
            input["legs"][0]["receipt_hash"] = invalid;
            assert_eq!(
                super::super::production_reads::intent_journey_json(&input),
                Err(ApiFailure::upstream_degraded())
            );
        }
    }

    #[test]
    fn intent_bank_golden_historical_payloads_still_decode_separately() {
        let plan = golden!("historical/intent.plan.response.json");
        let journey = golden!("historical/journey.get.response.json");
        for payload in [&plan, &journey] {
            let decoded: Value =
                serde_json::from_slice(&serde_json::to_vec(payload).expect("historical wire"))
                    .expect("historical JSON decoding");
            assert_eq!(&decoded, payload);
            assert_eq!(
                decoded["body"]["result"]["kind"]
                    .as_str()
                    .or_else(|| decoded["body"]["result"]["journey_kind"].as_str()),
                Some("deposit")
            );
        }
        assert_eq!(
            plan["body"]["result"]["legs"][0]["mechanism"],
            "paxeer-custody-deposit"
        );
        assert_eq!(
            journey["body"]["result"]["stages"][1]["evidence"][0]["verification"],
            "paxeer-finalised"
        );
    }
}

#[cfg(test)]
mod send_leg_denom_tests {
    use super::{bank_send_calldata, hex_bytes, send_leg_denom, ApiFailure, AssetId};
    use layerx_network_gateway::{
        AccountBalances, AssetBalance, AssetEntry, AssetTable, Evidence, ResolvedIdentities,
    };
    use sha2::{Digest as _, Sha256};

    fn asset_and_balances() -> (AssetTable, AccountBalances) {
        (
            AssetTable {
                height: 16,
                assets: vec![AssetEntry {
                    asset_id: "denom:ulxp".to_owned(),
                    denom: Some("ulxp".to_owned()),
                    kind: "bank".to_owned(),
                    name: None,
                    symbol: None,
                    decimals: None,
                    supply: None,
                }],
                joined_limit: 16,
                evidence: Evidence::GatewayReported,
            },
            AccountBalances {
                account: ResolvedIdentities {
                    lookup_selector: None,
                    evm_address: Some([0x11; 20]),
                    pax_address: None,
                    layerx_did: None,
                    layerx_account: None,
                    bound: false,
                    evidence: Evidence::GatewayReported,
                },
                height: 16,
                balances: vec![AssetBalance {
                    denom: "ulxp".to_owned(),
                    amount: 100.into(),
                }],
                evidence: Evidence::GatewayReported,
            },
        )
    }

    #[test]
    fn send_leg_denom_resolves_through_the_asset_table() {
        let (assets, balances) = asset_and_balances();
        assert_eq!(
            send_leg_denom(
                &assets,
                &balances,
                AssetId::new(Sha256::digest(b"ulxp").into())
            ),
            Ok("ulxp".to_owned())
        );
    }

    #[test]
    fn send_leg_denom_resolves_a_producer_shaped_bank_asset() {
        let (mut assets, mut balances) = asset_and_balances();
        assets.assets[0].asset_id = "denom:ubank".to_owned();
        assets.assets[0].denom = Some("ubank".to_owned());
        balances.balances[0].denom = "ubank".to_owned();
        let asset = AssetId::new(Sha256::digest(b"ubank").into());
        let denom = send_leg_denom(&assets, &balances, asset).expect("bank asset resolves");
        assert_eq!(denom, "ubank");

        let calldata = bank_send_calldata([0x11; 20], [0x22; 20], &denom, 42);
        assert_eq!(calldata.len(), 4 + 6 * 32);
        let tail = &calldata[4 + 5 * 32..];
        assert_eq!(&tail[..5], b"ubank");
        assert_eq!(&tail[5..], &[0; 27]);

        assets.assets[0].asset_id = format!("0x{}", hex_bytes(&asset.bytes()));
        assert_eq!(
            send_leg_denom(&assets, &balances, asset),
            Err(ApiFailure::forbidden())
        );
    }

    #[test]
    fn send_leg_denom_refuses_an_asset_without_denom_or_balance() {
        let (mut assets, mut balances) = asset_and_balances();
        let asset = AssetId::new(Sha256::digest(b"ulxp").into());
        assert_eq!(
            send_leg_denom(&assets, &balances, AssetId::new([0xcd; 32])),
            Err(ApiFailure::forbidden())
        );

        assets.assets[0].denom = None;
        assert_eq!(
            send_leg_denom(&assets, &balances, asset),
            Err(ApiFailure::forbidden())
        );

        assets.assets[0].denom = Some("ulxp".to_owned());
        assets.assets[0].asset_id = "denom:uother".to_owned();
        assert_eq!(
            send_leg_denom(&assets, &balances, asset),
            Err(ApiFailure::forbidden())
        );

        assets.assets[0].asset_id = "denom:ulxp".to_owned();
        assets.assets.push(assets.assets[0].clone());
        assert_eq!(
            send_leg_denom(&assets, &balances, asset),
            Err(ApiFailure::forbidden())
        );
        assets.assets.pop();

        balances.balances[0].denom = "uother".to_owned();
        assert_eq!(
            send_leg_denom(&assets, &balances, asset),
            Err(ApiFailure::forbidden())
        );
    }
}
