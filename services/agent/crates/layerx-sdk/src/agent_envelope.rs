//! Version 1 Agent operation envelope transport over the unified gateway route.
//!
//! The envelope reaches the agent daemon through `POST /v1/agent/rpc`. Gateway API-key
//! admission (`Authorization`) and the tenant session credential carried inside the
//! envelope are two distinct authorities; neither substitutes for the other. A mutating
//! operation whose outcome cannot be established after the request may have left the
//! client is reported as [`EnvelopeError::Unknown`] and is never resent automatically.

use std::fmt;
use std::path::Path;
use std::time::Duration;

use layerx_agent_api::error::{ApiError, ErrorClass, ReasonCode, RequestId, Retriability};
use layerx_agent_api::idempotency::{IdempotentMutation, Key};
use layerx_agent_api::prepare::CanonicalBytes;
use layerx_agent_api::proof::{ProofBundle, ProofBundleTarget};
use layerx_agent_api::read::{Freshness, ReadRequest, RelativeTo, VerifiedRead};
use layerx_agent_api::verify::{ApiSuccess, Rung, VerificationStatus};

use layerx_types::result::ResultCode;
use serde_json::{json, Map, Value};
use url::Url;

use crate::programs::LayerXKeyCredential;
use crate::{Call, Deployment, Operation};

/// HTTP evidence and the decoded outcome of one recorded operation.
pub type RecordedEnvelopeResponse = (
    Option<(u16, Value)>,
    Result<ApiSuccess<Value>, EnvelopeError>,
);

/// External unified-gateway route rewritten by the gateway to the daemon's `POST /rpc`.
pub const AGENT_RPC_ROUTE: &str = "/v1/agent/rpc";
/// The only envelope version this client produces and accepts.
pub const ENVELOPE_VERSION: u64 = 1;
/// Daemon body bound for one envelope.
pub const MAX_ENVELOPE_BYTES: usize = 1_048_576;
/// No bootstrap exceptions: every operation carries a session credential.
pub const BOOTSTRAP_OPERATIONS: &[Operation] = &[];
const MAX_HTTP_RESPONSE_BYTES: u64 = 9 * 1_048_576;
const MAX_TENANT_BYTES: usize = 255;
const MAX_REASON_BYTES: usize = 128;
const MAX_TRUST_ANCHOR_BYTES: u64 = 1_048_576;
const SUCCESS_FIELDS: &[&str] = &["request_id", "value", "verification_status"];
const ERROR_FIELDS: &[&str] = &[
    "class",
    "protocol_result_code",
    "retriability",
    "request_id",
    "reason",
];

/// Typed refusal or outcome of one envelope exchange.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum EnvelopeError {
    InvalidEndpoint,
    InvalidTrustAnchors,
    InvalidCredential,
    InvalidRequest,
    NotReady {
        request_id: RequestId,
        value: Value,
        status: NotReadyStatus,
    },
    /// A session credential is required for every operation.
    CredentialPresence {
        operation: Operation,
    },
    /// An idempotency key is required exactly for mutating operations.
    IdempotencyKeyPresence {
        operation: Operation,
    },
    DeploymentMismatch {
        deployment: Deployment,
    },
    Bounds,
    GatewayAuthentication,
    /// The daemon or gateway answered with the established error envelope.
    Refused(ApiError),
    /// A read failed in transport (no response, non-JSON body, or an edge/gateway
    /// answer); it had no effect and may be re-issued by the caller.
    Transport {
        operation: Operation,
    },
    /// A read returned a well-formed JSON body that violates the exact version 1
    /// success/error schema or names another request. Never re-issued.
    Decode {
        operation: Operation,
    },
    /// A mutation may or may not have taken effect. Reconcile through `track` with the
    /// same idempotency key; never resend automatically.
    Unknown {
        operation: Operation,
    },
}

/// A successful receipt lookup that has not reached its requested verification rung.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum NotReadyStatus {
    Pending { requested: Rung, achieved: Rung },
    Absent { requested: Rung },
}

/// All declared success states, including a lookup that has not reached its requested rung.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum EnvelopeOutcome {
    Ready(ApiSuccess<Value>),
    NotReady {
        request_id: RequestId,
        value: Value,
        status: NotReadyStatus,
    },
}

#[must_use]
pub fn decode_response_status(
    status: u16,
    document: &Value,
) -> Option<Result<EnvelopeOutcome, ApiError>> {
    if let Some((request_id, value, status)) = decode_not_ready_response(status, document) {
        return Some(Ok(EnvelopeOutcome::NotReady {
            request_id,
            value,
            status,
        }));
    }
    decode_response(status, document).map(|response| response.map(EnvelopeOutcome::Ready))
}

/// Preserves the daemon's exact not-ready success shapes without claiming verification.
#[must_use]
pub fn decode_not_ready_response(
    status: u16,
    document: &Value,
) -> Option<(RequestId, Value, NotReadyStatus)> {
    if status != 200 {
        return None;
    }
    let envelope = document.as_object()?;
    if !exact_fields(envelope, SUCCESS_FIELDS) {
        return None;
    }
    let fields = envelope.get("verification_status")?.as_object()?;
    let requested = rung(fields.get("requested")?)?;
    let status = match fields.get("state")?.as_str()? {
        "pending" if exact_fields(fields, &["state", "requested", "achieved"]) => {
            let achieved = rung(fields.get("achieved")?)?;
            if achieved >= requested {
                return None;
            }
            NotReadyStatus::Pending {
                requested,
                achieved,
            }
        }
        "absent" if exact_fields(fields, &["state", "requested"]) => {
            NotReadyStatus::Absent { requested }
        }
        _ => return None,
    };
    Some((
        request_id(envelope.get("request_id")?)?,
        envelope.get("value")?.clone(),
        status,
    ))
}

/// Whether the caller may re-issue an exchange that ended in an [`EnvelopeError`].
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum ClientRetriability {
    Safe,
    Never,
}

impl EnvelopeError {
    /// `Safe` only for a read transport failure; every other outcome, including
    /// [`EnvelopeError::Decode`] and [`EnvelopeError::Unknown`], is `Never`.
    #[must_use]
    pub const fn client_retriability(&self) -> ClientRetriability {
        match self {
            Self::Transport { .. } => ClientRetriability::Safe,
            _ => ClientRetriability::Never,
        }
    }
}

/// Session coordinates carried in the envelope `credential` object.
///
/// Bounds match the daemon's `SessionCredential`: tenant is 1..=255 bytes without NUL,
/// session and token identifiers are 32 bytes, generation is a `u64` sent as a canonical
/// decimal string.
#[derive(Clone, Eq, PartialEq)]
pub struct EnvelopeCredential {
    tenant: String,
    session_id: [u8; 32],
    token_id: [u8; 32],
    generation: u64,
}

impl Drop for EnvelopeCredential {
    fn drop(&mut self) {
        zeroize::Zeroize::zeroize(&mut self.token_id);
    }
}
impl fmt::Debug for EnvelopeCredential {
    fn fmt(&self, formatter: &mut fmt::Formatter<'_>) -> fmt::Result {
        formatter
            .debug_struct("EnvelopeCredential")
            .field("tenant", &self.tenant)
            .field("session_id", &hex(&self.session_id))
            .field("token_id", &"<redacted>")
            .field("generation", &self.generation)
            .finish()
    }
}

impl EnvelopeCredential {
    /// Binds the full tenant session coordinate.
    ///
    /// # Errors
    ///
    /// Refuses a tenant outside the daemon bounds and all-zero identifiers.
    pub fn new(
        tenant: impl Into<String>,
        session_id: [u8; 32],
        token_id: [u8; 32],
        generation: u64,
    ) -> Result<Self, EnvelopeError> {
        let tenant = tenant.into();
        if tenant.is_empty() || tenant.len() > MAX_TENANT_BYTES || tenant.as_bytes().contains(&0) {
            return Err(EnvelopeError::InvalidCredential);
        }
        if session_id == [0; 32] || token_id == [0; 32] {
            return Err(EnvelopeError::InvalidCredential);
        }
        Ok(Self {
            tenant,
            session_id,
            token_id,
            generation,
        })
    }

    #[must_use]
    pub fn tenant(&self) -> &str {
        &self.tenant
    }

    #[must_use]
    pub const fn session_id(&self) -> [u8; 32] {
        self.session_id
    }

    #[must_use]
    pub const fn generation(&self) -> u64 {
        self.generation
    }

    fn encode(&self) -> Value {
        json!({
            "tenant": self.tenant,
            "session_id": hex(&self.session_id),
            "token_id": hex(&self.token_id),
            "generation": self.generation.to_string(),
        })
    }
}

/// Encodes the exact version 1 envelope body.
///
/// # Errors
///
/// Refuses a non-object request, a missing session credential for any operation, and an
/// idempotency key that does not match the operation's generated mutating classification.
pub fn encode_envelope(
    operation: Operation,
    request_id: RequestId,
    request: &Value,
    credential: Option<&EnvelopeCredential>,
    idempotency_key: Option<Key>,
) -> Result<Value, EnvelopeError> {
    if !request.is_object() {
        return Err(EnvelopeError::InvalidRequest);
    }
    if BOOTSTRAP_OPERATIONS.contains(&operation) == credential.is_some() {
        return Err(EnvelopeError::CredentialPresence { operation });
    }
    if operation.mutating() != idempotency_key.is_some() {
        return Err(EnvelopeError::IdempotencyKeyPresence { operation });
    }
    Ok(json!({
        "version": ENVELOPE_VERSION,
        "request_id": request_id.0.to_string(),
        "operation": operation.name(),
        "request": request,
        "credential": credential.map_or(Value::Null, EnvelopeCredential::encode),
        "idempotency_key": idempotency_key.map_or(Value::Null, |key| Value::String(hex(&key.bytes()))),
    }))
}

/// HTTPS envelope client for the unified gateway Agent route.
pub struct AgentEnvelopeTransport {
    agent: ureq::Agent,
    endpoint: Url,
    gateway_key: Option<LayerXKeyCredential>,
    route: &'static str,
    native_authority: Option<([u8; 32], [u8; 20])>,
}

/// Internal route of the agent daemon RPC mTLS listener.
pub const DAEMON_RPC_ROUTE: &str = "/rpc";

impl AgentEnvelopeTransport {
    /// # Errors
    ///
    /// Returns `Decode` for invalid tool names or non-object arguments, and preserves envelope transport, refusal and unknown-outcome errors.
    pub fn mcp_invoke(
        &self,
        request_id: RequestId,
        credential: &EnvelopeCredential,
        tool: &str,
        arguments: &Value,
        idempotency_key: Key,
    ) -> Result<ApiSuccess<Value>, EnvelopeError> {
        if tool.is_empty()
            || tool.len() > 128
            || tool.as_bytes().contains(&0)
            || !arguments.is_object()
        {
            return Err(EnvelopeError::Decode {
                operation: Operation::McpInvoke,
            });
        }
        self.send_operation(
            Operation::McpInvoke,
            request_id,
            &json!({"tool":tool,"arguments":arguments}),
            Some(credential),
            Some(idempotency_key),
        )
    }

    /// # Errors
    ///
    /// Preserves operation errors and returns `Decode` for invalid readiness fields, recovery reasons or verification status.
    pub fn tenant_readiness(
        &self,
        request_id: RequestId,
        credential: &EnvelopeCredential,
    ) -> Result<ApiSuccess<layerx_agent_api::identity::TenantReadiness>, EnvelopeError> {
        use layerx_agent_api::identity::{TenantReadiness, TenantRecoveryReason};
        let response = self.send_operation(
            Operation::TenantReadiness,
            request_id,
            &json!({}),
            Some(credential),
            None,
        )?;
        let invalid = || EnvelopeError::Decode {
            operation: Operation::TenantReadiness,
        };
        let fields = response.value.as_object().ok_or_else(invalid)?;
        if !exact_fields(
            fields,
            &[
                "transport_ready",
                "verified_reads_ready",
                "writes_admitted",
                "recovery_reason",
            ],
        ) {
            return Err(invalid());
        }
        let recovery_reason = match fields.get("recovery_reason") {
            Some(Value::Null) => None,
            Some(Value::String(reason)) => {
                Some(TenantRecoveryReason::parse(reason).ok_or_else(invalid)?)
            }
            _ => return Err(invalid()),
        };
        let value = TenantReadiness {
            transport_ready: fields
                .get("transport_ready")
                .and_then(Value::as_bool)
                .ok_or_else(invalid)?,
            verified_reads_ready: fields
                .get("verified_reads_ready")
                .and_then(Value::as_bool)
                .ok_or_else(invalid)?,
            writes_admitted: fields
                .get("writes_admitted")
                .and_then(Value::as_bool)
                .ok_or_else(invalid)?,
            recovery_reason,
        };
        value.validate().map_err(|_| invalid())?;
        if response.verification_status != VerificationStatus::Achieved(Rung::Submitted) {
            return Err(invalid());
        }
        Ok(native_success(response, value))
    }
    /// Connects to a gateway origin. Only `https` is accepted; redirects are refused.
    ///
    /// `trust_anchors` names an explicit PEM file of trusted roots; when absent the
    /// platform roots are used. An empty or unreadable anchor file is refused.
    ///
    /// # Errors
    ///
    /// Refuses a non-HTTPS endpoint, query/fragment/credentials in the URL, and invalid
    /// or empty trust anchors.
    pub fn connect(
        endpoint: &str,
        gateway_key: Option<LayerXKeyCredential>,
        trust_anchors: Option<&Path>,
    ) -> Result<Self, EnvelopeError> {
        Self::build(endpoint, gateway_key, trust_anchors, None, AGENT_RPC_ROUTE)
    }

    /// Connects directly to the agent daemon RPC listener (`POST /rpc`). Only `https` is
    /// accepted; the client identity comes from `client_cert` and no `Authorization`
    /// header is ever sent, because the daemon refuses principal headers.
    ///
    /// # Errors
    ///
    /// Same as [`Self::connect`].
    pub fn connect_daemon(
        endpoint: &str,
        client_cert: ureq::tls::ClientCert,
        trust_anchors: &Path,
    ) -> Result<Self, EnvelopeError> {
        Self::build(
            endpoint,
            None,
            Some(trust_anchors),
            Some(client_cert),
            DAEMON_RPC_ROUTE,
        )
    }

    fn build(
        endpoint: &str,
        gateway_key: Option<LayerXKeyCredential>,
        trust_anchors: Option<&Path>,
        client_cert: Option<ureq::tls::ClientCert>,
        route: &'static str,
    ) -> Result<Self, EnvelopeError> {
        let endpoint = Url::parse(endpoint).map_err(|_| EnvelopeError::InvalidEndpoint)?;
        if endpoint.scheme() != "https"
            || endpoint.host_str().is_none_or(str::is_empty)
            || !endpoint.username().is_empty()
            || endpoint.password().is_some()
            || endpoint.query().is_some()
            || endpoint.fragment().is_some()
        {
            return Err(EnvelopeError::InvalidEndpoint);
        }
        let roots = match trust_anchors {
            Some(path) => explicit_roots(path)?,
            None => crate::tls::system_roots(endpoint.as_str())
                .map_err(|_| EnvelopeError::InvalidTrustAnchors)?,
        };
        let config = ureq::Agent::config_builder()
            .tls_config(
                ureq::tls::TlsConfig::builder()
                    .provider(ureq::tls::TlsProvider::Rustls)
                    .root_certs(roots)
                    .client_cert(client_cert)
                    .build(),
            )
            .timeout_global(Some(Duration::from_secs(30)))
            .http_status_as_error(false)
            .max_redirects(0)
            .build();
        Ok(Self {
            agent: config.into(),
            endpoint,
            gateway_key,
            route,
            native_authority: None,
        })
    }

    /// Pins the owner's independently resolved EVM signer address for unsigned preparation.
    /// An unsigned transaction cannot authenticate its own sender.
    /// # Errors
    ///
    /// Returns `InvalidCredential` for an all-zero owner or sender.
    pub fn with_native_authority(
        mut self,
        owner: [u8; 32],
        sender: [u8; 20],
    ) -> Result<Self, EnvelopeError> {
        if owner == [0; 32] || sender == [0; 20] {
            return Err(EnvelopeError::InvalidCredential);
        }
        self.native_authority = Some((owner, sender));
        Ok(self)
    }

    /// Preserves successful pending and absent lookups as typed outcomes.
    /// # Errors
    ///
    /// Preserves request, credential, transport, refusal and unknown-outcome errors from the operation.
    pub fn send_operation_status(
        &self,
        operation: Operation,
        request_id: RequestId,
        request: &Value,
        credential: Option<&EnvelopeCredential>,
        key: Option<Key>,
    ) -> Result<EnvelopeOutcome, EnvelopeError> {
        match self.send_operation(operation, request_id, request, credential, key) {
            Ok(success) => Ok(EnvelopeOutcome::Ready(success)),
            Err(EnvelopeError::NotReady {
                request_id,
                value,
                status,
            }) => Ok(EnvelopeOutcome::NotReady {
                request_id,
                value,
                status,
            }),
            Err(error) => Err(error),
        }
    }

    fn preparation_sender(
        &self,
        authority: &str,
        owner: [u8; 32],
    ) -> Result<[u8; 20], EnvelopeError> {
        let (pinned_owner, sender) = self
            .native_authority
            .ok_or(EnvelopeError::InvalidCredential)?;
        let text = authority.strip_prefix("did:layerx:").unwrap_or(authority);
        if pinned_owner != owner || text != hex(&owner) {
            return Err(EnvelopeError::InvalidCredential);
        }
        Ok(sender)
    }

    fn route(&self) -> Url {
        let mut endpoint = self.endpoint.clone();
        let base = endpoint.path().trim_end_matches('/').to_owned();
        endpoint.set_path(&format!("{base}{}", self.route));
        endpoint
    }

    /// Sends one typed non-mutating catalogue call. `request` is the operation request in
    /// its schema JSON form.
    ///
    /// # Errors
    ///
    /// Refuses a mutating operation (use [`Self::send_mutation`]), a non-daemon call,
    /// returns the established error envelope, or `Transport`.
    pub fn send_query<T>(
        &self,
        call: &Call<T>,
        request_id: RequestId,
        request: &Value,
        credential: Option<&EnvelopeCredential>,
    ) -> Result<ApiSuccess<Value>, EnvelopeError> {
        daemon_call(call)?;
        if call.operation().mutating() {
            return Err(EnvelopeError::IdempotencyKeyPresence {
                operation: call.operation(),
            });
        }
        self.send_operation(call.operation(), request_id, request, credential, None)
    }

    /// Sends one typed mutating catalogue call. The envelope `request_id` and
    /// `idempotency_key` are taken from the typed [`IdempotentMutation`]; `request` is the
    /// inner operation request in its schema JSON form.
    ///
    /// # Errors
    ///
    /// Returns the established error envelope, or [`EnvelopeError::Unknown`] whenever the
    /// outcome cannot be established after sending. Reconcile with `track`; never resend
    /// automatically.
    pub fn send_mutation<T>(
        &self,
        call: &Call<IdempotentMutation<T>>,
        request: &Value,
        credential: Option<&EnvelopeCredential>,
    ) -> Result<ApiSuccess<Value>, EnvelopeError> {
        daemon_call(call)?;
        let mutation = call.request();
        self.send_operation(
            call.operation(),
            mutation.request_id,
            request,
            credential,
            Some(mutation.key),
        )
    }

    /// Sends one catalogue operation by its generated identity.
    ///
    /// # Errors
    ///
    /// See [`Self::send_query`] and [`Self::send_mutation`].
    pub fn send_operation(
        &self,
        operation: Operation,
        request_id: RequestId,
        request: &Value,
        credential: Option<&EnvelopeCredential>,
        idempotency_key: Option<Key>,
    ) -> Result<ApiSuccess<Value>, EnvelopeError> {
        let mut received = None;
        self.exchange(
            operation,
            request_id,
            request,
            credential,
            idempotency_key,
            &mut received,
        )
    }

    /// Same as [`Self::send_operation`], also returning the HTTP status and parsed JSON
    /// body when a JSON response was received, for evidence retention.
    #[must_use = "inspect the recorded HTTP evidence and operation outcome"]
    pub fn send_operation_recorded(
        &self,
        operation: Operation,
        request_id: RequestId,
        request: &Value,
        credential: Option<&EnvelopeCredential>,
        idempotency_key: Option<Key>,
    ) -> RecordedEnvelopeResponse {
        let mut received = None;
        let outcome = self.exchange(
            operation,
            request_id,
            request,
            credential,
            idempotency_key,
            &mut received,
        );
        (received, outcome)
    }

    /// # Errors
    ///
    /// Refuses invalid request or session coordinates, owner consent or signer pins; preserves operation errors and returns `Unknown` for an inconsistent preparation response.
    pub fn prepare_native(
        &self,
        request_id: RequestId,
        key: Key,
        request: &layerx_agent_api::identity::NativePrepareRequestV1,
        credential: &EnvelopeCredential,
        _registry: &layerx_types::payload::ModuleRegistry,
    ) -> Result<ApiSuccess<layerx_agent_api::identity::NativePrepareResultV1>, EnvelopeError> {
        use sha2::{Digest, Sha256};
        if key.bytes() != request.idempotency_key {
            return Err(EnvelopeError::InvalidRequest);
        }
        let body = encode_native_prepare(request)?;
        let purpose = &request.purpose.purpose;
        verify_owner_purpose(
            &purpose
                .canonical_bytes()
                .map_err(|_| EnvelopeError::InvalidRequest)?,
            &request.purpose.owner_public_key,
            &request.purpose.signature,
        )?;
        let sender =
            self.preparation_sender(&request.authority, request.purpose.owner_public_key)?;
        if purpose.tenant.as_str() != credential.tenant()
            || purpose.session_id.to_bytes().ok() != Some(credential.session_id())
            || purpose.generation != credential.generation()
        {
            return Err(EnvelopeError::InvalidCredential);
        }
        let response = self.send_operation(
            Operation::Prepare,
            request_id,
            &body,
            Some(credential),
            Some(key),
        )?;
        let unknown = || EnvelopeError::Unknown {
            operation: Operation::Prepare,
        };
        let value = decode_native_preparation(&response.value).ok_or_else(unknown)?;
        let canonical_digest: [u8; 32] = Sha256::digest(&value.canonical_bytes).into();
        let call =
            validate_unsigned_preparation(&value, purpose.preparation_id, purpose.canonical_digest)
                .map_err(|_| unknown())?;
        if value.preparation_id != purpose.preparation_id
            || canonical_digest != purpose.canonical_digest
            || value.preparation_id != canonical_digest
            || value.activity != request.activity
            || !raw_call_matches_request(
                &call,
                sender,
                request.activity,
                request.actor.as_str(),
                request.account_sequence,
                request.not_before,
                request.not_after,
                request.idempotency_key,
                request.fee_limit,
                &request.payload,
                request.payload_hash,
                &value.canonical_bytes,
            )
            || call.signing_hash() != value.signing_preimage
            || value
                .approval_id
                .is_some_and(|id| id != value.preparation_id)
        {
            return Err(unknown());
        }
        Ok(native_success(response, value))
    }

    /// # Errors
    ///
    /// Refuses missing signer authority or invalid disclosure requests; preserves operation errors and returns `Unknown` for a response that fails disclosure binding.
    pub fn disclose_native_preparation(
        &self,
        request_id: RequestId,
        key: Key,
        request: &layerx_agent_api::identity::NativeDisclosureRequestV1,
        credential: &EnvelopeCredential,
        _registry: &layerx_types::payload::ModuleRegistry,
    ) -> Result<ApiSuccess<layerx_agent_api::identity::NativeDisclosureResultV1>, EnvelopeError>
    {
        let (_, sender) = self
            .native_authority
            .ok_or(EnvelopeError::InvalidCredential)?;
        let body = crate::native_effect::encode_native_disclosure(request)?;
        let response = self.send_operation(
            Operation::Prepare,
            request_id,
            &body,
            Some(credential),
            Some(key),
        )?;
        let unknown = || EnvelopeError::Unknown {
            operation: Operation::Prepare,
        };
        let value = decode_native_disclosure(&response.value).ok_or_else(unknown)?;
        validate_call_disclosure(request, &value, sender).map_err(|_| unknown())?;
        Ok(native_success(response, value))
    }

    /// # Errors
    ///
    /// Refuses invalid effect requests, credentials, idempotency or owner-purpose bindings; preserves operation errors and refuses inconsistent preparation responses.
    pub fn prepare_native_effect(
        &self,
        request_id: RequestId,
        key: Key,
        request: &layerx_agent_api::identity::NativeEffectPrepareRequestV1,
        credential: &EnvelopeCredential,
        registry: &layerx_types::payload::ModuleRegistry,
    ) -> Result<ApiSuccess<layerx_agent_api::identity::NativePrepareResultV1>, EnvelopeError> {
        let body = crate::native_effect::encode_native_effect_prepare(request)?;
        self.prepare_native_effect_body(request_id, key, request, &body, credential, registry)
    }

    /// # Errors
    ///
    /// Refuses invalid registration requests or consent bindings and preserves native preparation operation errors.
    pub fn prepare_native_registration(
        &self,
        request_id: RequestId,
        key: Key,
        request: &layerx_agent_api::identity::NativeRegistrationPrepareRequestV1,
        credential: &EnvelopeCredential,
        registry: &layerx_types::payload::ModuleRegistry,
    ) -> Result<ApiSuccess<layerx_agent_api::identity::NativePrepareResultV1>, EnvelopeError> {
        let body = crate::native_effect::encode_native_registration_prepare(request)?;
        self.prepare_native_effect_body(
            request_id,
            key,
            &request.request,
            &body,
            credential,
            registry,
        )
    }

    fn prepare_native_effect_body(
        &self,
        request_id: RequestId,
        key: Key,
        request: &layerx_agent_api::identity::NativeEffectPrepareRequestV1,
        body: &Value,
        credential: &EnvelopeCredential,
        _registry: &layerx_types::payload::ModuleRegistry,
    ) -> Result<ApiSuccess<layerx_agent_api::identity::NativePrepareResultV1>, EnvelopeError> {
        use sha2::{Digest, Sha256};
        let purpose = &request.purpose.purpose;
        verify_owner_purpose(
            &purpose
                .canonical_bytes()
                .map_err(|_| EnvelopeError::InvalidRequest)?,
            &request.purpose.owner_public_key,
            &request.purpose.signature,
        )?;
        let sender =
            self.preparation_sender(&request.authority, request.purpose.owner_public_key)?;
        crate::native_effect::validate_request_binding(request, credential, key)?;
        let response = self.send_operation(
            Operation::Prepare,
            request_id,
            body,
            Some(credential),
            Some(key),
        )?;
        let unknown = || EnvelopeError::Unknown {
            operation: Operation::Prepare,
        };
        let value = decode_native_preparation(&response.value).ok_or_else(unknown)?;
        let canonical_digest: [u8; 32] = Sha256::digest(&value.canonical_bytes).into();
        let call =
            validate_unsigned_preparation(&value, purpose.preparation_id, purpose.canonical_digest)
                .map_err(|_| unknown())?;
        if value.preparation_id != purpose.preparation_id
            || canonical_digest != purpose.canonical_digest
            || value.preparation_id != canonical_digest
            || value.activity != request.activity
            || !raw_call_matches_request(
                &call,
                sender,
                request.activity,
                request.actor.as_str(),
                request.account_sequence,
                request.not_before,
                request.not_after,
                request.idempotency_key,
                request.fee_limit,
                &request.payload,
                request.payload_hash,
                &value.canonical_bytes,
            )
            || call.signing_hash() != value.signing_preimage
            || value
                .approval_id
                .is_some_and(|id| id != value.preparation_id)
        {
            return Err(unknown());
        }
        Ok(native_success(response, value))
    }

    /// # Errors
    ///
    /// Refuses invalid Send requests, credentials, idempotency or consent signatures and preserves native preparation operation errors.
    pub fn prepare_native_send(
        &self,
        request_id: RequestId,
        key: Key,
        request: &crate::native_effect::NativeSendPrepareRequestV2,
        credential: &EnvelopeCredential,
        registry: &layerx_types::payload::ModuleRegistry,
    ) -> Result<ApiSuccess<layerx_agent_api::identity::NativePrepareResultV1>, EnvelopeError> {
        self.prepare_native_send_surface(request_id, key, request, credential, registry, false)
    }

    /// # Errors
    ///
    /// Refuses invalid Send request or consent bindings and preserves MCP operation and preparation response validation errors.
    pub fn prepare_native_send_mcp(
        &self,
        request_id: RequestId,
        key: Key,
        request: &crate::native_effect::NativeSendPrepareRequestV2,
        credential: &EnvelopeCredential,
        registry: &layerx_types::payload::ModuleRegistry,
    ) -> Result<ApiSuccess<layerx_agent_api::identity::NativePrepareResultV1>, EnvelopeError> {
        self.prepare_native_send_surface(request_id, key, request, credential, registry, true)
    }

    fn prepare_native_send_surface(
        &self,
        request_id: RequestId,
        key: Key,
        request: &crate::native_effect::NativeSendPrepareRequestV2,
        credential: &EnvelopeCredential,
        _registry: &layerx_types::payload::ModuleRegistry,
        mcp: bool,
    ) -> Result<ApiSuccess<layerx_agent_api::identity::NativePrepareResultV1>, EnvelopeError> {
        use sha2::{Digest, Sha256};
        let body = crate::native_effect::encode_native_send_prepare(request)?;
        let purpose = &request.purpose.purpose;
        verify_owner_purpose(
            &purpose
                .canonical_bytes()
                .map_err(|_| EnvelopeError::InvalidRequest)?,
            &purpose.consent_public_key,
            &request.purpose.signature,
        )?;
        let sender = self.preparation_sender(&request.authority, purpose.consent_public_key)?;
        if sender != purpose.evm_sender {
            return Err(EnvelopeError::InvalidCredential);
        }
        crate::native_effect::validate_send_request_binding(request, credential, key)?;
        let response = if mcp {
            self.mcp_invoke(request_id, credential, "activity.prepare", &body, key)?
        } else {
            self.send_operation(
                Operation::Prepare,
                request_id,
                &body,
                Some(credential),
                Some(key),
            )?
        };
        let unknown = || EnvelopeError::Unknown {
            operation: Operation::Prepare,
        };
        let value = decode_native_preparation(&response.value).ok_or_else(unknown)?;
        let canonical_digest: [u8; 32] = Sha256::digest(&value.canonical_bytes).into();
        let call =
            validate_unsigned_preparation(&value, purpose.preparation_id, purpose.canonical_digest)
                .map_err(|_| unknown())?;
        if value.preparation_id != purpose.preparation_id
            || canonical_digest != purpose.canonical_digest
            || value.preparation_id != canonical_digest
            || value.activity != request.activity
            || !raw_call_matches_request(
                &call,
                sender,
                request.activity,
                request.actor.as_str(),
                request.account_sequence,
                request.not_before,
                request.not_after,
                request.idempotency_key,
                request.fee_limit,
                &request.payload,
                request.payload_hash,
                &value.canonical_bytes,
            )
            || call.signing_hash() != value.signing_preimage
            || call.chain_id != purpose.chain_id
            || purpose.transaction_type
                != layerx_agent_api::identity::NativeSendPurposeV2::TRANSACTION_TYPE
            || value
                .approval_id
                .is_some_and(|id| id != value.preparation_id)
        {
            return Err(unknown());
        }
        Ok(native_success(response, value))
    }

    /// # Errors
    ///
    /// Preserves operation errors and returns `Decode` for an invalid native approval list.
    pub fn approval_list_native(
        &self,
        request_id: RequestId,
        credential: &EnvelopeCredential,
    ) -> Result<ApiSuccess<layerx_agent_api::identity::NativeApprovalListResultV1>, EnvelopeError>
    {
        let response = self.send_operation(
            Operation::ApprovalList,
            request_id,
            &json!({"variant":"native_v1"}),
            Some(credential),
            None,
        )?;
        let value = decode_native_approval_list(&response.value).ok_or(EnvelopeError::Decode {
            operation: Operation::ApprovalList,
        })?;
        Ok(native_success(response, value))
    }

    /// # Errors
    ///
    /// Preserves operation errors and returns `Decode` for an invalid native approval record.
    pub fn approval_get_native(
        &self,
        request_id: RequestId,
        request: &layerx_agent_api::identity::NativeApprovalGetV1,
        credential: &EnvelopeCredential,
    ) -> Result<ApiSuccess<layerx_agent_api::identity::NativeApprovalResultV1>, EnvelopeError> {
        let response = self.send_operation(
            Operation::ApprovalGet,
            request_id,
            &json!({"variant":"native_v1", "approval_id":hex(&request.approval_id)}),
            Some(credential),
            None,
        )?;
        let value = decode_native_approval(&response.value)
            .filter(|value| value.approval_id == request.approval_id)
            .ok_or(EnvelopeError::Decode {
                operation: Operation::ApprovalGet,
            })?;
        Ok(native_success(response, value))
    }

    /// # Errors
    ///
    /// Refuses invalid decision requests and preserves operation and approval response decoding errors.
    pub fn approval_decide_native(
        &self,
        request_id: RequestId,
        key: Key,
        request: &layerx_agent_api::identity::NativeApprovalDecisionV1,
        credential: &EnvelopeCredential,
        grant: bool,
    ) -> Result<ApiSuccess<layerx_agent_api::identity::NativeApprovalResultV1>, EnvelopeError> {
        let operation = if grant {
            Operation::ApprovalApprove
        } else {
            Operation::ApprovalReject
        };
        let response = self.send_operation(operation, request_id,
            &json!({"variant":"native_v1", "approval_id":hex(&request.approval_id),
                    "held_digest":hex(&request.held_digest), "current_sequence":request.current_sequence.to_string()}),
            Some(credential), Some(key))?;
        let value = decode_native_approval(&response.value)
            .filter(|value| {
                value.approval_id == request.approval_id && value.held_digest == request.held_digest
            })
            .ok_or(EnvelopeError::Unknown { operation })?;
        Ok(native_success(response, value))
    }

    /// # Errors
    ///
    /// Refuses invalid proof bundle requests and preserves operation and verified response decoding errors.
    pub fn read_proof_bundle(
        &self,
        request_id: RequestId,
        request: &ReadRequest<CanonicalBytes>,
        credential: &EnvelopeCredential,
    ) -> Result<ApiSuccess<VerifiedRead<ProofBundle>>, EnvelopeError> {
        let body = encode_proof_bundle_request(request)?;
        let response = self.send_operation(
            Operation::ReadProofBundle,
            request_id,
            &body,
            Some(credential),
            None,
        )?;
        let value =
            decode_proof_bundle_response(request, &response).ok_or(EnvelopeError::Decode {
                operation: Operation::ReadProofBundle,
            })?;
        Ok(native_success(response, value))
    }

    fn send_envelope(
        &self,
        operation: Operation,
        body: &[u8],
    ) -> Result<ureq::http::Response<ureq::Body>, EnvelopeError> {
        let authorization = self
            .gateway_key
            .as_ref()
            .map(LayerXKeyCredential::authorization)
            .transpose()
            .map_err(|_| EnvelopeError::GatewayAuthentication)?;
        let ambiguous = || {
            if operation.mutating() {
                EnvelopeError::Unknown { operation }
            } else {
                EnvelopeError::Transport { operation }
            }
        };
        let route = self.route();
        let mut request_builder = self
            .agent
            .post(route.as_str())
            .header("Accept", "application/json")
            .header("Content-Type", "application/json")
            .header(
                "User-Agent",
                concat!("layerx-rust/", env!("CARGO_PKG_VERSION")),
            );
        if let Some(value) = authorization.as_deref() {
            request_builder = request_builder.header("Authorization", value);
        }
        request_builder.send(body).map_err(|_| ambiguous())
    }

    fn exchange(
        &self,
        operation: Operation,
        request_id: RequestId,
        request: &Value,
        credential: Option<&EnvelopeCredential>,
        idempotency_key: Option<Key>,
        received: &mut Option<(u16, Value)>,
    ) -> Result<ApiSuccess<Value>, EnvelopeError> {
        let proof_request = if operation == Operation::ReadProofBundle {
            Some(decode_proof_bundle_request(request).ok_or(EnvelopeError::InvalidRequest)?)
        } else {
            None
        };
        let envelope =
            encode_envelope(operation, request_id, request, credential, idempotency_key)?;
        let body = serde_json::to_vec(&envelope).map_err(|_| EnvelopeError::InvalidRequest)?;
        if body.len() > MAX_ENVELOPE_BYTES {
            return Err(EnvelopeError::Bounds);
        }
        let mut response = self.send_envelope(operation, &body)?;
        let ambiguous = || {
            if operation.mutating() {
                EnvelopeError::Unknown { operation }
            } else {
                EnvelopeError::Transport { operation }
            }
        };
        let malformed = || {
            if operation.mutating() {
                EnvelopeError::Unknown { operation }
            } else {
                EnvelopeError::Transport { operation }
            }
        };
        let status = response.status().as_u16();
        let json_body = response
            .headers()
            .get("content-type")
            .and_then(|value| value.to_str().ok())
            .and_then(|value| value.split(';').next())
            .is_some_and(|value| value.trim() == "application/json");
        if !json_body {
            return Err(malformed());
        }
        let response_limit = if proof_request.is_some() {
            MAX_ENVELOPE_BYTES as u64 + 1
        } else {
            MAX_HTTP_RESPONSE_BYTES
        };
        let encoded = response
            .body_mut()
            .with_config()
            .limit(response_limit)
            .read_to_vec()
            .map_err(|_| ambiguous())?;
        if proof_request.is_some() && encoded.len() > MAX_ENVELOPE_BYTES {
            return Err(EnvelopeError::Decode { operation });
        }
        let document: Value = serde_json::from_slice(&encoded).map_err(|_| malformed())?;
        *received = Some((status, document.clone()));
        let edge = matches!(status, 502..=504)
            || document
                .as_object()
                .is_some_and(|object| object.contains_key("ok"));
        let schema_violation = || {
            if operation.mutating() {
                EnvelopeError::Unknown { operation }
            } else if edge {
                EnvelopeError::Transport { operation }
            } else {
                EnvelopeError::Decode { operation }
            }
        };
        if let Some((received_id, value, status)) = decode_not_ready_response(status, &document) {
            if received_id != request_id {
                return Err(schema_violation());
            }
            return Err(EnvelopeError::NotReady {
                request_id: received_id,
                value,
                status,
            });
        }
        validate_operation_response(
            operation,
            request_id,
            status,
            &document,
            proof_request.as_ref(),
            edge,
        )
    }
}

fn validate_operation_response(
    operation: Operation,
    request_id: RequestId,
    status: u16,
    document: &Value,
    proof_request: Option<&ReadRequest<CanonicalBytes>>,
    edge: bool,
) -> Result<ApiSuccess<Value>, EnvelopeError> {
    let schema_violation = || {
        if operation.mutating() {
            EnvelopeError::Unknown { operation }
        } else if edge {
            EnvelopeError::Transport { operation }
        } else {
            EnvelopeError::Decode { operation }
        }
    };
    let decoded = decode_response(status, document).ok_or_else(schema_violation)?;
    let received = match &decoded {
        Ok(success) => success.request_id,
        // "0" is the daemon's request_id for an envelope it could not parse.
        Err(error) if error.request_id == RequestId(0) => request_id,
        Err(error) => error.request_id,
    };
    if received != request_id {
        return Err(if operation.mutating() {
            EnvelopeError::Unknown { operation }
        } else {
            EnvelopeError::Decode { operation }
        });
    }
    let success = decoded.map_err(EnvelopeError::Refused)?;
    if let Some(request) = proof_request {
        decode_proof_bundle_response(request, &success)
            .ok_or(EnvelopeError::Decode { operation })?;
    }
    Ok(success)
}

fn daemon_call<T>(call: &Call<T>) -> Result<(), EnvelopeError> {
    if call.deployment() == Deployment::Daemon {
        Ok(())
    } else {
        Err(EnvelopeError::DeploymentMismatch {
            deployment: call.deployment(),
        })
    }
}

fn explicit_roots(path: &Path) -> Result<ureq::tls::RootCerts, EnvelopeError> {
    let metadata = std::fs::metadata(path).map_err(|_| EnvelopeError::InvalidTrustAnchors)?;
    if !metadata.is_file() || metadata.len() == 0 || metadata.len() > MAX_TRUST_ANCHOR_BYTES {
        return Err(EnvelopeError::InvalidTrustAnchors);
    }
    let pem = std::fs::read(path).map_err(|_| EnvelopeError::InvalidTrustAnchors)?;
    let mut certificates = Vec::new();
    for item in ureq::tls::parse_pem(&pem) {
        match item.map_err(|_| EnvelopeError::InvalidTrustAnchors)? {
            ureq::tls::PemItem::Certificate(certificate) => {
                certificates.push(certificate.to_owned());
            }
            _ => return Err(EnvelopeError::InvalidTrustAnchors),
        }
    }
    if certificates.is_empty() {
        return Err(EnvelopeError::InvalidTrustAnchors);
    }
    Ok(ureq::tls::RootCerts::new_with_certs(&certificates))
}

/// Decodes a version 1 response document. `None` means the document is not a valid
/// success or error envelope for the given status.
#[must_use]
pub fn decode_response(
    status: u16,
    document: &Value,
) -> Option<Result<ApiSuccess<Value>, ApiError>> {
    if let Some((request_id, _, status)) = decode_not_ready_response(status, document) {
        let reason = match status {
            NotReadyStatus::Pending { .. } => "verification.pending",
            NotReadyStatus::Absent { .. } => "verification.absent",
        };
        return Some(Err(ApiError {
            class: ErrorClass::UnavailableCapability,
            protocol_result_code: None,
            retriability: Retriability::Retriable,
            request_id,
            reason: ReasonCode::new(reason).ok()?,
        }));
    }
    let envelope = document.as_object()?;
    if status == 200 {
        if !exact_fields(envelope, SUCCESS_FIELDS) {
            return None;
        }
        let request_id = request_id(envelope.get("request_id")?)?;
        let verification_status = verification_status(envelope.get("verification_status")?)?;
        return Some(Ok(ApiSuccess {
            request_id,
            value: envelope.get("value")?.clone(),
            verification_status,
        }));
    }
    if !exact_fields(envelope, ERROR_FIELDS) {
        return None;
    }
    let class = match envelope.get("class")?.as_str()? {
        "TransportFailure" => ErrorClass::TransportFailure,
        "Deadline" => ErrorClass::Deadline,
        "ProtocolIncompatibility" => ErrorClass::ProtocolIncompatibility,
        "UnavailableCapability" => ErrorClass::UnavailableCapability,
        "CoreRejection" => ErrorClass::CoreRejection,
        "VerificationFailure" => ErrorClass::VerificationFailure,
        "PolicyRefusal" => ErrorClass::PolicyRefusal,
        "CapabilityRefusal" => ErrorClass::CapabilityRefusal,
        "BudgetRefusal" => ErrorClass::BudgetRefusal,
        "RateLimit" => ErrorClass::RateLimit,
        "IdempotencyConflict" => ErrorClass::IdempotencyConflict,
        "InternalFault" => ErrorClass::InternalFault,
        _ => return None,
    };
    let retriability = match envelope.get("retriability")?.as_str()? {
        "Terminal" => Retriability::Terminal,
        "Retriable" => Retriability::Retriable,
        _ => return None,
    };
    let protocol_result_code = match envelope.get("protocol_result_code")? {
        Value::Null => None,
        Value::Number(number) => Some(ResultCode::from_raw(i32::try_from(number.as_i64()?).ok()?)),
        _ => return None,
    };
    Some(Err(ApiError {
        class,
        protocol_result_code,
        retriability,
        request_id: request_id(envelope.get("request_id")?)?,
        reason: reason(envelope.get("reason")?)?,
    }))
}

fn verification_status(value: &Value) -> Option<VerificationStatus> {
    let status = value.as_object()?;
    match status.get("state")?.as_str()? {
        "achieved" if exact_fields(status, &["state", "rung"]) => {
            Some(VerificationStatus::Achieved(rung(status.get("rung")?)?))
        }
        "unverified" if exact_fields(status, &["state", "requested", "achieved", "reason"]) => {
            let requested = rung(status.get("requested")?)?;
            let achieved = rung(status.get("achieved")?)?;
            if achieved >= requested {
                return None;
            }
            Some(VerificationStatus::Below {
                requested,
                achieved,
                reason: reason(status.get("reason")?)?,
            })
        }
        _ => None,
    }
}

fn rung(value: &Value) -> Option<Rung> {
    Rung::from_name(value.as_str()?)
}

fn request_id(value: &Value) -> Option<RequestId> {
    canonical_u64(value.as_str()?).map(RequestId)
}

fn reason(value: &Value) -> Option<ReasonCode> {
    let text = value.as_str()?;
    if text.len() > MAX_REASON_BYTES {
        return None;
    }
    ReasonCode::new(text).ok()
}

/// Parses a canonical decimal `u64`: digits only, no sign, no leading zero except `0`.
#[must_use]
pub fn canonical_u64(text: &str) -> Option<u64> {
    if text.is_empty()
        || !text.bytes().all(|byte| byte.is_ascii_digit())
        || (text.len() > 1 && text.starts_with('0'))
    {
        return None;
    }
    text.parse().ok()
}

fn exact_fields(value: &Map<String, Value>, fields: &[&str]) -> bool {
    value.len() == fields.len() && fields.iter().all(|field| value.contains_key(*field))
}

fn hex(bytes: &[u8]) -> String {
    use std::fmt::Write as _;
    bytes
        .iter()
        .fold(String::with_capacity(bytes.len() * 2), |mut out, byte| {
            let _ = write!(out, "{byte:02x}");
            out
        })
}

/// # Errors
///
/// Returns `InvalidRequest` when the native preparation request fails validation.
pub fn encode_native_prepare(
    request: &layerx_agent_api::identity::NativePrepareRequestV1,
) -> Result<Value, EnvelopeError> {
    request
        .clone()
        .validate()
        .map_err(|_| EnvelopeError::InvalidRequest)?;
    let signed = &request.purpose;
    let purpose = &signed.purpose;
    Ok(json!({
        "variant": "native_v1",
        "activity": native_activity_json(request.activity),
        "actor": request.actor.as_str(), "authority": request.authority,
        "account_sequence": request.account_sequence.to_string(),
        "not_before": request.not_before.to_string(), "not_after": request.not_after.to_string(),
        "idempotency_key": hex(&request.idempotency_key), "fee_limit": request.fee_limit.to_string(),
        "payload": hex(&request.payload), "payload_hash": hex(&request.payload_hash),
        "capability_id": request.capability_id.as_str(),
        "purpose": {
            "purpose": {"version":"1", "tenant":purpose.tenant.as_str(),
                "agent_did":purpose.agent_did.as_str(), "session_id":purpose.session_id.as_str(),
                "generation":purpose.generation.to_string(), "expires_at_ms":purpose.expires_at_ms.to_string(),
                "capability_id":purpose.capability_id.as_str(), "preparation_id":hex(&purpose.preparation_id),
                "canonical_digest":hex(&purpose.canonical_digest), "commitment":hex(&purpose.commitment)},
            "owner_public_key":hex(&signed.owner_public_key), "signature":hex(&signed.signature)
        },
        "local_grant": request.local_grant.as_ref().map(|grant| json!({
            "version":"1", "capability":hex(&grant.capability), "session_scope":hex(&grant.session_scope),
            "expires_at_ms":grant.expires_at_ms.to_string(), "owner_public_key":hex(&grant.owner_public_key),
            "signature":hex(&grant.signature)
        }))
    }))
}

fn native_activity_json(activity: layerx_agent_api::identity::NativeActivity) -> Value {
    json!({"version":"1", "module":activity.module.to_string(), "ordinal":activity.ordinal.to_string()})
}

fn native_activity(value: &Value) -> Option<layerx_agent_api::identity::NativeActivity> {
    let value = value.as_object()?;
    if !exact_fields(value, &["version", "module", "ordinal"])
        || value.get("version")?.as_str()? != "1"
    {
        return None;
    }
    layerx_agent_api::identity::NativeActivity::new(
        u16::try_from(canonical_u64(value.get("module")?.as_str()?)?).ok()?,
        u16::try_from(canonical_u64(value.get("ordinal")?.as_str()?)?).ok()?,
    )
    .ok()
}

fn native_hex(value: &Value, maximum: usize) -> Option<Vec<u8>> {
    let text = value.as_str()?;
    if text.is_empty()
        || text.len() % 2 != 0
        || text.len() / 2 > maximum
        || !text
            .bytes()
            .all(|byte| matches!(byte, b'0'..=b'9' | b'a'..=b'f'))
    {
        return None;
    }
    text.as_bytes()
        .chunks_exact(2)
        .map(|pair| u8::from_str_radix(std::str::from_utf8(pair).ok()?, 16).ok())
        .collect()
}

fn native_id(value: &Value) -> Option<[u8; 32]> {
    native_hex(value, 32)?.try_into().ok()
}

fn native_optional_id(value: &Value) -> Result<Option<[u8; 32]>, ()> {
    if value.is_null() {
        Ok(None)
    } else {
        native_id(value).map(Some).ok_or(())
    }
}

#[must_use]
pub fn decode_native_disclosure(
    value: &Value,
) -> Option<layerx_agent_api::identity::NativeDisclosureResultV1> {
    let value = value.as_object()?;
    if !exact_fields(
        value,
        &[
            "version",
            "preparation_id",
            "canonical_bytes",
            "disclosure_digest",
            "activity_type",
            "actor",
            "authority",
            "asset",
            "fee_limit",
            "not_before",
            "not_after",
            "payload_expires_at",
            "idempotency_key",
        ],
    ) || value.get("version")?.as_str()? != "1"
    {
        return None;
    }
    let number = |field: &str| canonical_u64(value.get(field)?.as_str()?);
    let amount = value.get("fee_limit")?.as_str()?;
    if amount.is_empty()
        || !amount.bytes().all(|b| b.is_ascii_digit())
        || (amount.len() > 1 && amount.starts_with('0'))
    {
        return None;
    }
    Some(layerx_agent_api::identity::NativeDisclosureResultV1 {
        preparation_id: native_id(value.get("preparation_id")?)?,
        canonical_bytes: native_hex(
            value.get("canonical_bytes")?,
            layerx_wire::limits::MAX_MESSAGE_BYTES,
        )?,
        disclosure_digest: native_id(value.get("disclosure_digest")?)?,
        activity_type: u32::try_from(number("activity_type")?).ok()?,
        actor: native_hex(value.get("actor")?, layerx_types::limits::MAX_DID_BYTES)?,
        authority: native_hex(
            value.get("authority")?,
            layerx_types::limits::MAX_AUTHORITY_BYTES,
        )?,
        asset: native_id(value.get("asset")?)?,
        fee_limit: amount.parse().ok()?,
        not_before: number("not_before")?,
        not_after: number("not_after")?,
        payload_expires_at: number("payload_expires_at")?,
        idempotency_key: native_id(value.get("idempotency_key")?)?,
    })
}

/// Validates the producer's canonical unsigned call, preparation identity and signing hash.
/// The owner purpose is authenticated separately, and the sender is pinned by the caller.
/// # Errors
///
/// Returns `InvalidRequest` when the canonical unsigned transaction cannot be decoded or its preparation ID, digest or signing preimage differs.
pub fn validate_unsigned_preparation(
    value: &layerx_agent_api::identity::NativePrepareResultV1,
    preparation_id: [u8; 32],
    canonical_digest: [u8; 32],
) -> Result<layerx_crypto::evm_transaction::Eip1559Call, EnvelopeError> {
    use sha2::{Digest, Sha256};
    let call = layerx_crypto::evm_transaction::decode_unsigned(&value.canonical_bytes)
        .map_err(|_| EnvelopeError::InvalidRequest)?;
    if call.encode_unsigned() != value.canonical_bytes
        || value.signing_preimage != call.signing_hash()
        || value.preparation_id != preparation_id
        || preparation_id != canonical_digest
        || canonical_digest != <[u8; 32]>::from(Sha256::digest(&value.canonical_bytes))
        || value.approval_id.is_some_and(|id| id != preparation_id)
    {
        return Err(EnvelopeError::InvalidRequest);
    }
    Ok(call)
}

fn verify_owner_purpose(
    canonical: &[u8],
    key: &[u8; 32],
    signature: &[u8; 64],
) -> Result<(), EnvelopeError> {
    use sha2::{Digest, Sha256};
    layerx_crypto::ed25519::verify_digest(key, signature, &Sha256::digest(canonical).into())
        .map_err(|_| EnvelopeError::InvalidCredential)
}

/// Checks the unsigned call and disclosure against the sender selected by the external signer.
/// # Errors
///
/// Refuses a malformed canonical transaction, signer or preparation mismatch, invalid disclosure, or a disclosure that does not match the requested call.
pub fn validate_call_disclosure(
    request: &layerx_agent_api::identity::NativeDisclosureRequestV1,
    value: &layerx_agent_api::identity::NativeDisclosureResultV1,
    expected_sender: [u8; 20],
) -> Result<layerx_crypto::disclosure::Disclosure, EnvelopeError> {
    use sha2::{Digest, Sha256};
    let call = layerx_crypto::evm_transaction::decode_unsigned(&request.canonical_bytes)
        .map_err(|_| EnvelopeError::InvalidRequest)?;
    let disclosure = layerx_crypto::disclosure::call::bind_call(&call, expected_sender)
        .map_err(|_| EnvelopeError::InvalidRequest)?;
    if value.canonical_bytes != request.canonical_bytes
        || value.preparation_id != <[u8; 32]>::from(Sha256::digest(&request.canonical_bytes))
        || value.disclosure_digest
            != disclosure
                .signing_digest()
                .map_err(|_| EnvelopeError::InvalidRequest)?
        || value.activity_type != disclosure.activity_type.value()
        || value.actor != disclosure.actor
        || value.authority != disclosure.authority
        || value.asset != disclosure.asset
        || value.fee_limit != disclosure.fee_limit
        || value.not_before != disclosure.expiry.not_before
        || value.not_after != disclosure.expiry.not_after
        || value.payload_expires_at != disclosure.expiry.payload_expires_at
        || value.idempotency_key != disclosure.idempotency_key
    {
        return Err(EnvelopeError::InvalidRequest);
    }
    Ok(disclosure)
}

fn decode_raw_call(
    raw: &[u8],
) -> Result<(layerx_crypto::evm_transaction::Eip1559Call, [u8; 20]), EnvelopeError> {
    use layerx_crypto::evm_transaction::{decode_signed, recover_sender};
    let (call, signature) = decode_signed(raw).map_err(|_| EnvelopeError::InvalidRequest)?;
    let sender = recover_sender(&call, &signature).map_err(|_| EnvelopeError::InvalidRequest)?;
    Ok((call, sender))
}

/// Decodes the signed transaction carried by an envelope and binds its sender.
///
/// # Errors
/// Refuses malformed transaction bytes or a signature from another sender.
pub fn decode_signed_envelope_call(
    value: &Value,
    expected_sender: [u8; 20],
) -> Result<layerx_crypto::evm_transaction::Eip1559Call, EnvelopeError> {
    let raw = native_hex(
        value
            .get("canonical_bytes")
            .ok_or(EnvelopeError::InvalidRequest)?,
        MAX_ENVELOPE_BYTES,
    )
    .ok_or(EnvelopeError::InvalidRequest)?;
    let (call, sender) = decode_raw_call(&raw)?;
    if sender != expected_sender {
        return Err(EnvelopeError::InvalidCredential);
    }
    Ok(call)
}

#[allow(clippy::too_many_arguments)]
fn raw_call_matches_request(
    call: &layerx_crypto::evm_transaction::Eip1559Call,
    sender: [u8; 20],
    activity: layerx_agent_api::identity::NativeActivity,
    actor: &str,
    nonce: u64,
    not_before: u64,
    not_after: u64,
    idempotency_key: [u8; 32],
    fee_limit: u128,
    payload: &[u8],
    payload_hash: [u8; 32],
    raw: &[u8],
) -> bool {
    use layerx_types::payload::ModuleId;
    use sha2::{Digest, Sha256};
    let target = match activity
        .activity_type()
        .map(layerx_types::payload::ActivityType::module)
    {
        Ok(ModuleId::Programs) => [0x10, 0x1d],
        Ok(ModuleId::Asset) => [0x10, 0x1a],
        Ok(ModuleId::Budget | ModuleId::Escrow | ModuleId::Stream) => [0x10, 0x1b],
        _ => return false,
    };
    let expected_hash: [u8; 32] = Sha256::new()
        .chain_update(layerx_wire::hash::Domain::PayloadHash.tag())
        .chain_update(payload)
        .finalize()
        .into();
    let Ok(disclosure) = layerx_crypto::disclosure::call::bind_call(call, sender) else {
        return false;
    };
    let Ok(kind) = activity.activity_type() else {
        return false;
    };
    let expected_payload = match kind.module() {
        ModuleId::Programs => layerx_types::program_call::NativeProgramCall::decode(payload)
            .map(|native| layerx_crypto::disclosure::call::encode_program_call(&native, fee_limit))
            .ok(),
        ModuleId::Asset if kind.ordinal() == 5 => layerx_crypto::send::ledger_payment_call(
            payload,
            &layerx_crypto::send::CallOptions {
                from: sender,
                chain_id: call.chain_id,
                nonce: call.nonce,
                max_priority_fee_per_gas: call.max_priority_fee_per_gas,
                max_fee_per_gas: call.max_fee_per_gas,
                gas_limit: call.gas_limit,
            },
        )
        .map(|disclosed| disclosed.call.data)
        .ok(),
        _ => layerx_crypto::payments::Payment::decode(
            kind.module(),
            kind.ordinal(),
            payload,
            actor.as_bytes(),
        )
        .and_then(|payment| {
            layerx_crypto::disclosure::call::encode_payment_call(&payment, actor.as_bytes())
        })
        .ok(),
    };
    call.to[..18] == [0; 18]
        && call.to[18..] == target
        && call.nonce == nonce
        && disclosure.activity_type == kind
        // Ledger send carries its expiry in the signed payload, without an activity window.
        && ((kind.module() == ModuleId::Asset && kind.ordinal() == 5)
            || (disclosure.expiry.not_before == not_before
                && disclosure.expiry.not_after == not_after))
        && (if disclosure.idempotency_key == [0; 32] {
            <[u8; 32]>::from(Sha256::digest(raw)) == idempotency_key
        } else {
            disclosure.idempotency_key == idempotency_key
        })
        && disclosure.fee_limit == fee_limit
        && expected_payload.as_deref() == Some(call.data.as_slice())
        && call.encode_unsigned() == raw
        && payload_hash == expected_hash
}

#[must_use]
pub fn decode_native_preparation(
    value: &Value,
) -> Option<layerx_agent_api::identity::NativePrepareResultV1> {
    let value = value.as_object()?;
    if !exact_fields(
        value,
        &[
            "version",
            "preparation_id",
            "canonical_bytes",
            "signing_preimage",
            "activity",
            "approval_required",
            "approval_id",
        ],
    ) || value.get("version")?.as_str()? != "1"
    {
        return None;
    }
    let approval_required = value.get("approval_required")?.as_bool()?;
    let approval_id = native_optional_id(value.get("approval_id")?).ok()?;
    if approval_required != approval_id.is_some() {
        return None;
    }
    Some(layerx_agent_api::identity::NativePrepareResultV1 {
        preparation_id: native_id(value.get("preparation_id")?)?,
        canonical_bytes: native_hex(
            value.get("canonical_bytes")?,
            layerx_wire::limits::MAX_MESSAGE_BYTES,
        )?,
        signing_preimage: native_id(value.get("signing_preimage")?)?,
        activity: native_activity(value.get("activity")?)?,
        approval_required,
        approval_id,
    })
}

#[must_use]
pub fn decode_native_approval(
    value: &Value,
) -> Option<layerx_agent_api::identity::NativeApprovalResultV1> {
    let value = value.as_object()?;
    if !exact_fields(
        value,
        &[
            "version",
            "approval_id",
            "held_digest",
            "activity",
            "state",
            "submission_ref",
        ],
    ) || value.get("version")?.as_str()? != "1"
    {
        return None;
    }
    let state = value.get("state")?.as_str()?;
    if !matches!(
        state,
        "Awaiting" | "Granted" | "Rejected" | "Expired" | "Defective" | "NotRequired"
    ) {
        return None;
    }
    Some(layerx_agent_api::identity::NativeApprovalResultV1 {
        approval_id: native_id(value.get("approval_id")?)?,
        held_digest: native_id(value.get("held_digest")?)?,
        activity: native_activity(value.get("activity")?)?,
        state: state.to_owned(),
        submission_ref: native_optional_id(value.get("submission_ref")?).ok()?,
    })
}

#[must_use]
pub fn decode_native_approval_list(
    value: &Value,
) -> Option<layerx_agent_api::identity::NativeApprovalListResultV1> {
    let value = value.as_object()?;
    if !exact_fields(value, &["version", "approvals"]) || value.get("version")?.as_str()? != "1" {
        return None;
    }
    let approvals = value.get("approvals")?.as_array()?;
    if approvals.len() > 100 {
        return None;
    }
    Some(layerx_agent_api::identity::NativeApprovalListResultV1 {
        approvals: approvals
            .iter()
            .map(decode_native_approval)
            .collect::<Option<Vec<_>>>()?,
    })
}

fn native_success<T>(response: ApiSuccess<Value>, value: T) -> ApiSuccess<T> {
    ApiSuccess {
        request_id: response.request_id,
        value,
        verification_status: response.verification_status,
    }
}

/// # Errors
///
/// Returns `InvalidRequest` when the proof-bundle request does not use a valid block selector or canonical identifier.
pub fn encode_proof_bundle_request(
    request: &ReadRequest<CanonicalBytes>,
) -> Result<Value, EnvelopeError> {
    ProofBundleTarget::decode(request.selector.as_bytes())
        .map_err(|_| EnvelopeError::InvalidRequest)?;
    Ok(json!({
        "target": hex(request.selector.as_bytes()),
        "requested_rung": request.requested_rung.name(),
    }))
}

#[must_use]
pub fn decode_proof_bundle_request(value: &Value) -> Option<ReadRequest<CanonicalBytes>> {
    let request = value.as_object()?;
    if !exact_fields(request, &["target", "requested_rung"]) {
        return None;
    }
    let bytes = native_hex(request.get("target")?, 67)?;
    ProofBundleTarget::decode(&bytes).ok()?;
    Some(ReadRequest {
        selector: CanonicalBytes::new(bytes).ok()?,
        requested_rung: Rung::from_name(request.get("requested_rung")?.as_str()?)?,
    })
}

#[must_use]
pub fn decode_proof_bundle_response(
    request: &ReadRequest<CanonicalBytes>,
    response: &ApiSuccess<Value>,
) -> Option<VerifiedRead<ProofBundle>> {
    let operation = Operation::ReadProofBundle;
    let read = response.value.as_object()?;
    if !exact_fields(read, &["value", "achieved_verification_level", "freshness"]) {
        return None;
    }
    let achieved = Rung::from_name(read.get("achieved_verification_level")?.as_str()?)?;
    if !matches!(response.verification_status, VerificationStatus::Achieved(rung) if rung == achieved)
    {
        return None;
    }
    let freshness = Freshness::decode_json(&read.get("freshness")?.to_string()).ok()?;
    let RelativeTo::Block { height, hash } = freshness.relative_to;
    if freshness.value_height > freshness.chain_head
        || height != freshness.value_height
        || hash == [0; 32]
    {
        return None;
    }
    let result = VerifiedRead::new(
        crate::rpc_export::decode_bundle(read.get("value")?, operation).ok()?,
        achieved,
        freshness,
    );
    let target = ProofBundleTarget::decode(request.selector.as_bytes()).ok()?;
    let typed_request = ReadRequest {
        selector: target,
        requested_rung: request.requested_rung,
    };
    ProofBundle::check_response(&typed_request, &result).ok()?;
    let receipt = layerx_wire::receipt::decode(result.value.signed_receipt.as_bytes()).ok()?;
    if receipt.protocol()?.activity_id() != target.activity_id() {
        return None;
    }
    Some(result)
}

#[cfg(test)]
mod rung_contract_tests {
    use super::*;

    #[test]
    fn idempotent_replay_remains_a_terminal_core_rejection() {
        let expected = ApiError::idempotent_replay(RequestId(9));
        let wire = json!({"class":"CoreRejection", "protocol_result_code":-302, "retriability":"Terminal", "request_id":"9", "reason":"idempotent_replay"});
        assert_eq!(decode_response(409, &wire), Some(Err(expected.clone())));
        assert_eq!(expected.class, ErrorClass::CoreRejection);
        assert_eq!(
            expected.protocol_result_code,
            Some(ResultCode::from_raw(-302))
        );
        assert_eq!(expected.retriability, Retriability::Terminal);
        assert_eq!(expected.reason.as_str(), "idempotent_replay");
        assert_ne!(expected.class, ErrorClass::IdempotencyConflict);
    }
}

#[cfg(test)]
mod native_disclosure_tests {
    use super::*;
    use layerx_agent_api::identity::{NativeDisclosureRequestV1, NativeDisclosureResultV1};
    use layerx_agentd::prepare::evm;
    use sha2::{Digest, Sha256};

    fn served_call() -> (NativeDisclosureRequestV1, NativeDisclosureResultV1) {
        let payload = hex::decode(
            include_str!("../../layerx-crypto/tests/fixtures/payments/native-1-5.hex").trim(),
        )
        .unwrap_or_else(|error| panic!("Ledger transfer payload: {error:?}"));
        let prepared = evm::prepare_bound(
            evm::EvmPreparationState {
                chain_id: 125,
                pending_nonce: 7,
                base_fee_per_gas: 1,
                head_height: 50,
            },
            evm::PreparationDefaults {
                max_priority_fee_per_gas: 1,
                max_fee_per_gas: 5,
                maximum_payload_bytes: 65_536,
            },
            evm::PrepareRequest {
                from: [0x11; 20],
                call: evm::PrecompileCall::ledger_transfer(payload),
                expected_nonce: Some(7),
                max_priority_fee_per_gas: None,
                max_fee_per_gas: None,
                declared_payload_limit: 65_536,
            },
            100_000,
        )
        .unwrap_or_else(|error| panic!("daemon Ledger call producer: {error:?}"));
        assert_eq!(&prepared.call.data[..4], &[0x2b, 0xb1, 0xae, 0x7c]);
        let canonical_bytes = prepared.call.encode_unsigned();
        let d = prepared.disclosure;
        let value = NativeDisclosureResultV1 {
            preparation_id: Sha256::digest(&canonical_bytes).into(),
            canonical_bytes: canonical_bytes.clone(),
            disclosure_digest: prepared.disclosure_digest,
            activity_type: d.activity_type.value(),
            actor: d.actor,
            authority: d.authority,
            asset: d.asset,
            fee_limit: d.fee_limit,
            not_before: d.expiry.not_before,
            not_after: d.expiry.not_after,
            payload_expires_at: d.expiry.payload_expires_at,
            idempotency_key: d.idempotency_key,
        };
        (NativeDisclosureRequestV1 { canonical_bytes }, value)
    }

    #[test]
    fn native_disclosure_binds_unsigned_ledger_call_and_signer() {
        let (request, value) = served_call();
        let disclosure = validate_call_disclosure(&request, &value, [0x11; 20])
            .unwrap_or_else(|error| panic!("served unsigned call and signer agree: {error:?}"));
        let call = layerx_crypto::evm_transaction::decode_unsigned(&request.canonical_bytes)
            .unwrap_or_else(|error| panic!("unsigned EIP-1559: {error:?}"));
        assert_eq!(
            disclosure
                .signing_digest()
                .unwrap_or_else(|error| panic!("expected a successful fixture result: {error:?}")),
            call.signing_hash()
        );
        assert_eq!(value.disclosure_digest, call.signing_hash());
        assert!(validate_call_disclosure(&request, &value, [0x22; 20]).is_err());
        let mut audit = value.clone();
        audit.disclosure_digest = disclosure
            .audit_digest()
            .unwrap_or_else(|error| panic!("expected a successful fixture result: {error:?}"));
        assert_ne!(audit.disclosure_digest, value.disclosure_digest);
        assert!(validate_call_disclosure(&request, &audit, [0x11; 20]).is_err());
        let mut signed = request.clone();
        signed.canonical_bytes = call
            .sign(&[3; 32])
            .unwrap_or_else(|error| panic!("signed transaction: {error:?}"));
        assert!(validate_call_disclosure(&signed, &value, [0x11; 20]).is_err());
    }

    #[test]
    fn native_disclosure_rejects_every_changed_answer_field() {
        let (request, value) = served_call();
        for field in [
            "preparation_id",
            "canonical_bytes",
            "disclosure_digest",
            "activity_type",
            "actor",
            "authority",
            "asset",
            "fee_limit",
            "not_before",
            "not_after",
            "payload_expires_at",
            "idempotency_key",
        ] {
            let mut changed = value.clone();
            match field {
                "preparation_id" => changed.preparation_id[0] ^= 1,
                "canonical_bytes" => changed.canonical_bytes[0] ^= 1,
                "disclosure_digest" => changed.disclosure_digest[0] ^= 1,
                "activity_type" => changed.activity_type ^= 1,
                "actor" => changed.actor[0] ^= 1,
                "authority" => changed.authority[0] ^= 1,
                "asset" => changed.asset[0] ^= 1,
                "fee_limit" => changed.fee_limit ^= 1,
                "not_before" => changed.not_before ^= 1,
                "not_after" => changed.not_after ^= 1,
                "payload_expires_at" => changed.payload_expires_at ^= 1,
                "idempotency_key" => changed.idempotency_key[0] ^= 1,
                _ => unreachable!(),
            }
            assert!(
                validate_call_disclosure(&request, &changed, [0x11; 20]).is_err(),
                "{field}"
            );
        }
    }
}
