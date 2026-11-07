//! Canonical event-source and receipt-verification adapters.

use layerx_platform_gateway::{verify_activity_operation, AuthorityFacts, VerifiedOperation};
use native_tls::{Certificate, Identity};
use serde::Deserialize;
use std::collections::BTreeMap;
use std::env;
use std::fs;
use std::time::{Duration, Instant};
use subtle::ConstantTimeEq;
use x509_cert::der::asn1::ObjectIdentifier;
use x509_cert::der::Decode;
use x509_cert::ext::pkix::name::GeneralName;
use x509_cert::ext::pkix::{ExtendedKeyUsage, SubjectAltName};
use zeroize::{Zeroize, Zeroizing};

use crate::boundary::{Client, ClientIdentity, Endpoint, OutboundRequest};
use crate::encoding::{fixed_hex, hex_decode, hex_encode};
use crate::error::WebhookError;
use crate::events::{
    EventDraft, EventId, EventKind, PaymentDraft, Principal, ProtocolEvent, ProtocolFact,
    SubjectId, Verification,
};

const MAX_SOURCE_FACTS: usize = 32;

/// The URI SAN that marks an internal client leaf as an event producer.
pub const PRODUCER_ROLE: &str = "urn:layerx:webhooks:role:producer";
/// The URI SAN that marks an internal client leaf as a delivery operator.
pub const OPERATOR_ROLE: &str = "urn:layerx:webhooks:role:operator";
const ROLE_MARKER_PREFIX: &str = "urn:layerx:webhooks:role";
const SUBJECT_ALT_NAME: ObjectIdentifier = ObjectIdentifier::new_unwrap("2.5.29.17");
const EXTENDED_KEY_USAGE: ObjectIdentifier = ObjectIdentifier::new_unwrap("2.5.29.37");
const CLIENT_AUTH: ObjectIdentifier = ObjectIdentifier::new_unwrap("1.3.6.1.5.5.7.3.2");

/// The role a verified internal client leaf holds on the private ingress.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum IngressRole {
    Producer,
    Operator,
}

impl IngressRole {
    /// Reads the role of a client leaf the TLS layer already verified against
    /// the internal CA. The leaf must carry the clientAuth extended key usage
    /// and exactly one recognized role URI SAN; the subject, headers and CA
    /// membership never imply a role.
    ///
    /// # Errors
    /// Refuses malformed leaves, a missing clientAuth usage, and missing,
    /// duplicate, contradictory, unknown or malformed role markers.
    pub fn from_certificate(leaf: &[u8]) -> Result<Self, WebhookError> {
        let certificate =
            x509_cert::Certificate::from_der(leaf).map_err(|_| WebhookError::InvalidRequest)?;
        let mut usage = None;
        let mut names = None;
        for extension in certificate.tbs_certificate.extensions.iter().flatten() {
            let value = extension.extn_value.as_bytes();
            if extension.extn_id == EXTENDED_KEY_USAGE {
                let parsed =
                    ExtendedKeyUsage::from_der(value).map_err(|_| WebhookError::InvalidRequest)?;
                if usage.replace(parsed).is_some() {
                    return Err(WebhookError::InvalidRequest);
                }
            } else if extension.extn_id == SUBJECT_ALT_NAME {
                let parsed =
                    SubjectAltName::from_der(value).map_err(|_| WebhookError::InvalidRequest)?;
                if names.replace(parsed).is_some() {
                    return Err(WebhookError::InvalidRequest);
                }
            }
        }
        if !usage.is_some_and(|usage| usage.0.contains(&CLIENT_AUTH)) {
            return Err(WebhookError::InvalidRequest);
        }
        let mut role = None;
        for name in names.iter().flat_map(|names| names.0.iter()) {
            let GeneralName::UniformResourceIdentifier(uri) = name else {
                continue;
            };
            let uri = uri.to_string();
            let marker = uri
                .get(..ROLE_MARKER_PREFIX.len())
                .is_some_and(|prefix| prefix.eq_ignore_ascii_case(ROLE_MARKER_PREFIX));
            if !marker {
                continue;
            }
            let parsed = match uri.as_str() {
                PRODUCER_ROLE => Self::Producer,
                OPERATOR_ROLE => Self::Operator,
                _ => return Err(WebhookError::InvalidRequest),
            };
            if role.replace(parsed).is_some() {
                return Err(WebhookError::InvalidRequest);
            }
        }
        role.ok_or(WebhookError::InvalidRequest)
    }
}

pub struct TrustedEvent(ProtocolEvent);

impl TrustedEvent {
    pub(crate) fn from_market(event: ProtocolEvent) -> Result<Self, WebhookError> {
        event.validate()?;
        if event.kind() != EventKind::Program
            || event.verification() != Verification::BlockCommitted
        {
            return Err(WebhookError::VerificationRequired);
        }
        Ok(Self(event))
    }
    pub fn event(&self) -> &ProtocolEvent {
        &self.0
    }
}

struct Source {
    endpoint: Endpoint,
    token: Zeroizing<String>,
}

pub struct AuthorityReceiptVerifier {
    client: Client,
    component: Endpoint,
    component_token: Zeroizing<String>,
    authority: Endpoint,
    authority_token: Zeroizing<String>,
    sequencer_authorization: layerx_platform_gateway::SequencerAuthorization,
    network_id: String,
    wire_version: String,
    protocol_network_id: u32,
}

pub struct TrustedSources {
    client: Client,
    sources: BTreeMap<EventKind, Source>,
    verifier: ReceiptVerifier,
}

pub struct DeveloperIdentity {
    client: Client,
    endpoint: Endpoint,
    token: Zeroizing<String>,
}

pub struct SourceTrigger {
    token: Zeroizing<String>,
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct SessionResponse {
    active: bool,
    sub: String,
    #[serde(default)]
    csrf_token: String,
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct SourceRecord {
    id: String,
    principal: String,
    subject: String,
    subject_sequence: u64,
    occurred_at: u64,
    facts: Vec<SourceFact>,
    #[serde(default)]
    activity_id: Option<String>,
    #[serde(default)]
    amount: Option<String>,
    #[serde(default)]
    asset: Option<String>,
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct SourceFact {
    name: String,
    value: String,
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct ComponentReceipt {
    activity_id: String,
    receipt: String,
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct AuthorityResponse {
    activity_id: String,
    receipt: String,
    protocol_network_id: u32,
    batch_id: String,
    asset: String,
    previous_state_root: String,
    resulting_state_root: String,
    sequencer_public_key: String,
    network_id: String,
    wire_version: String,
    #[serde(
        default,
        deserialize_with = "layerx_platform_gateway::authority_evidence::present_maintained"
    )]
    batch_evidence: Option<layerx_platform_gateway::authority_evidence::MaintainedBatchDocument>,
}

impl TrustedSources {
    /// # Errors
    /// Refuses missing or invalid configuration, secret files, endpoints or TLS material.
    pub fn from_environment() -> Result<Self, String> {
        let ca = Certificate::from_der(
            &fs::read(
                env::var("LAYERX_WEBHOOKS_INTERNAL_CA_DER")
                    .map_err(|_| "LAYERX_WEBHOOKS_INTERNAL_CA_DER is required")?,
            )
            .map_err(|error| error.to_string())?,
        )
        .map_err(|error| error.to_string())?;
        let password = read_secret("LAYERX_WEBHOOKS_CLIENT_IDENTITY_PASSWORD_FILE")?;
        let identity = Identity::from_pkcs12(
            &fs::read(
                env::var("LAYERX_WEBHOOKS_CLIENT_IDENTITY_PKCS12")
                    .map_err(|_| "LAYERX_WEBHOOKS_CLIENT_IDENTITY_PKCS12 is required")?,
            )
            .map_err(|error| error.to_string())?,
            password.as_str(),
        )
        .map_err(|error| error.to_string())?;
        let client_identity = ClientIdentity::new(ca, Some(identity));
        let mut sources = BTreeMap::new();
        for (kind, stem) in [
            (EventKind::Journey, "JOURNEY"),
            (EventKind::Payment, "PAYMENT"),
            (EventKind::Approval, "APPROVAL"),
            (EventKind::Program, "PROGRAM"),
        ] {
            sources.insert(
                kind,
                Source {
                    endpoint: Endpoint::parse(
                        &env::var(format!("LAYERX_WEBHOOKS_{stem}_SOURCE_URL"))
                            .map_err(|_| format!("{stem} event source URL is required"))?,
                    )?,
                    token: read_secret(&format!("LAYERX_WEBHOOKS_{stem}_SOURCE_TOKEN_FILE"))?,
                },
            );
        }
        let verifier = ReceiptVerifier::from_environment(client_identity.clone())?;
        Ok(Self {
            client: Client::trusted(client_identity),
            sources,
            verifier,
        })
    }

    #[must_use]
    pub fn ready(&self) -> bool {
        Instant::now()
            .checked_add(Duration::from_secs(5))
            .is_some_and(|deadline| self.ready_until(deadline))
    }

    pub fn ready_until(&self, deadline: Instant) -> bool {
        self.verifier.ready_until(deadline)
            && self.sources.values().all(|source| {
                self.client
                    .request_until(
                        &OutboundRequest {
                            endpoint: &source.endpoint,
                            method: "GET",
                            path: "/readyz",
                            bearer: Some(source.token.as_str()),
                            idempotency: None,
                            headers: &[],
                            body: &[],
                        },
                        deadline,
                    )
                    .is_ok_and(|response| response.status == 200)
            })
    }

    pub fn trigger_admission(
        &self,
        kind: EventKind,
        deadline: Instant,
    ) -> Result<(u64, usize), WebhookError> {
        let source = self
            .sources
            .get(&kind)
            .ok_or(WebhookError::InvalidRequest)?;
        let response = self
            .client
            .request_until(
                &OutboundRequest {
                    endpoint: &source.endpoint,
                    method: "GET",
                    path: "/internal/v1/reader-readiness",
                    bearer: Some(source.token.as_str()),
                    idempotency: None,
                    headers: &[],
                    body: &[],
                },
                deadline,
            )
            .map_err(|_| WebhookError::Unavailable)?;
        if response.status != 200
            || !response.content_type.starts_with("application/json")
            || response.body.len() > 4096
        {
            return Err(WebhookError::Unavailable);
        }
        #[derive(Deserialize)]
        #[serde(deny_unknown_fields)]
        struct Admission {
            schema: String,
            role: String,
            kind: String,
            ready: bool,
            generation: u64,
            principals: usize,
            principal_digest: bool,
            fresh_for_ms: u64,
        }
        let value: Admission =
            serde_json::from_slice(&response.body).map_err(|_| WebhookError::Unavailable)?;
        if value.schema != "layerx.event-admission.v1"
            || value.role != "source-reader"
            || value.kind != kind.as_str()
            || !value.ready
            || value.generation == 0
            || value.principals == 0
            || value.principal_digest
            || value.fresh_for_ms != 10_000
            || !self.verifier.ready_until(deadline)
        {
            return Err(WebhookError::Unavailable);
        }
        Ok((value.generation, value.principals))
    }

    /// # Errors
    /// Refuses invalid identifiers, unavailable sources, malformed events and unverified receipts.
    pub fn fetch(
        &self,
        kind: EventKind,
        source_event_id: &str,
    ) -> Result<TrustedEvent, WebhookError> {
        if !valid_identifier(source_event_id, 128) {
            return Err(WebhookError::InvalidRequest);
        }
        let source = self
            .sources
            .get(&kind)
            .ok_or(WebhookError::InvalidRequest)?;
        let path = format!("/internal/v1/events/{source_event_id}");
        let response = self
            .client
            .request(&OutboundRequest {
                endpoint: &source.endpoint,
                method: "GET",
                path: &path,
                bearer: Some(source.token.as_str()),
                idempotency: None,
                headers: &[],
                body: &[],
            })
            .map_err(|_| WebhookError::Unavailable)?;
        if response.status != 200 || !response.content_type.starts_with("application/json") {
            return Err(WebhookError::Unavailable);
        }
        let record: SourceRecord =
            serde_json::from_slice(&response.body).map_err(|_| WebhookError::Unavailable)?;
        if record.id != source_event_id || record.facts.len() > MAX_SOURCE_FACTS {
            return Err(WebhookError::InvalidRequest);
        }
        self.event(kind, record).map(TrustedEvent)
    }

    fn event(&self, kind: EventKind, record: SourceRecord) -> Result<ProtocolEvent, WebhookError> {
        let id = EventId::new(record.id)?;
        let principal = Principal::new(record.principal)?;
        let subject = SubjectId::new(record.subject)?;
        let operation = record
            .activity_id
            .as_deref()
            .map(|activity| self.verifier.verify(activity))
            .transpose()?;
        if kind == EventKind::Payment {
            let operation = operation
                .as_ref()
                .ok_or(WebhookError::VerificationRequired)?;
            let amount = record.amount.ok_or(WebhookError::InvalidRequest)?;
            let asset = record.asset.ok_or(WebhookError::InvalidRequest)?;
            return crate::events::settled_payment_at(
                PaymentDraft {
                    id,
                    principal,
                    subject,
                    subject_sequence: record.subject_sequence,
                    occurred_at: record.occurred_at,
                    operation: &operation.operation,
                    amount,
                    asset,
                },
                operation.verification,
            );
        }
        let mut facts = Vec::with_capacity(record.facts.len().saturating_add(2));
        for fact in record.facts {
            facts.push(ProtocolFact::unverified(fact.name, fact.value)?);
        }
        if let Some(operation) = operation.as_ref() {
            let verification = operation.verification;
            let operation = &operation.operation;
            let receipt = hex_encode(&operation.receipt_digest());
            facts.push(ProtocolFact::verified(
                "activity_id",
                hex_encode(&operation.activity_id()),
                verification,
                receipt.as_str(),
            )?);
            facts.push(ProtocolFact::verified(
                "result_code",
                operation.result_code().to_string(),
                verification,
                receipt,
            )?);
        }
        ProtocolEvent::new(EventDraft {
            id,
            kind,
            principal,
            subject,
            subject_sequence: record.subject_sequence,
            occurred_at: record.occurred_at,
            facts,
        })
    }
}

impl DeveloperIdentity {
    /// # Errors
    /// Refuses missing or invalid configuration, secret files, endpoints or TLS material.
    pub fn from_environment() -> Result<Self, String> {
        let ca = Certificate::from_der(
            &fs::read(
                env::var("LAYERX_WEBHOOKS_INTERNAL_CA_DER")
                    .map_err(|_| "LAYERX_WEBHOOKS_INTERNAL_CA_DER is required")?,
            )
            .map_err(|error| error.to_string())?,
        )
        .map_err(|error| error.to_string())?;
        let password = read_secret("LAYERX_WEBHOOKS_CLIENT_IDENTITY_PASSWORD_FILE")?;
        let identity = Identity::from_pkcs12(
            &fs::read(
                env::var("LAYERX_WEBHOOKS_CLIENT_IDENTITY_PKCS12")
                    .map_err(|_| "LAYERX_WEBHOOKS_CLIENT_IDENTITY_PKCS12 is required")?,
            )
            .map_err(|error| error.to_string())?,
            password.as_str(),
        )
        .map_err(|error| error.to_string())?;
        Ok(Self {
            client: Client::trusted(ClientIdentity::new(ca, Some(identity))),
            endpoint: Endpoint::parse(
                &env::var("LAYERX_WEBHOOKS_IDENTITY_URL")
                    .map_err(|_| "LAYERX_WEBHOOKS_IDENTITY_URL is required")?,
            )?,
            token: read_secret("LAYERX_WEBHOOKS_IDENTITY_TOKEN_FILE")?,
        })
    }

    /// # Errors
    /// Refuses missing or invalid configuration, secret files, endpoints or TLS material.
    pub fn from_dashboard_environment() -> Result<Self, String> {
        let ca = Certificate::from_der(
            &fs::read(
                env::var("LAYERX_DASHBOARD_INTERNAL_CA_DER")
                    .map_err(|_| "LAYERX_DASHBOARD_INTERNAL_CA_DER is required")?,
            )
            .map_err(|error| error.to_string())?,
        )
        .map_err(|error| error.to_string())?;
        let password = read_secret("LAYERX_DASHBOARD_CLIENT_IDENTITY_PASSWORD_FILE")?;
        let identity = Identity::from_pkcs12(
            &fs::read(
                env::var("LAYERX_DASHBOARD_CLIENT_IDENTITY_PKCS12")
                    .map_err(|_| "LAYERX_DASHBOARD_CLIENT_IDENTITY_PKCS12 is required")?,
            )
            .map_err(|error| error.to_string())?,
            password.as_str(),
        )
        .map_err(|error| error.to_string())?;
        Ok(Self {
            client: Client::trusted(ClientIdentity::new(ca, Some(identity))),
            endpoint: Endpoint::parse(
                &env::var("LAYERX_DASHBOARD_IDENTITY_URL")
                    .map_err(|_| "LAYERX_DASHBOARD_IDENTITY_URL is required")?,
            )?,
            token: read_secret("LAYERX_DASHBOARD_IDENTITY_TOKEN_FILE")?,
        })
    }

    /// # Errors
    /// Refuses invalid or inactive sessions, failed introspection, invalid principals
    /// and missing or mismatched anti-forgery tokens for cookie mutations.
    pub fn authenticate(
        &self,
        authorization: Option<&str>,
        cookie: Option<&str>,
        anti_forgery: Option<&str>,
        mutation: bool,
    ) -> Result<Principal, WebhookError> {
        let (session, cookie_auth) = match authorization {
            Some(value) => (
                value
                    .strip_prefix("Bearer ")
                    .filter(|token| !token.is_empty() && token.len() <= 4096)
                    .ok_or(WebhookError::InvalidRequest)?,
                false,
            ),
            None => (
                session_cookie(cookie.ok_or(WebhookError::InvalidRequest)?)?,
                true,
            ),
        };
        let body = Zeroizing::new(
            serde_json::to_vec(&serde_json::json!({ "token": session }))
                .map_err(|_| WebhookError::Unavailable)?,
        );
        let response = self
            .client
            .request(&OutboundRequest {
                endpoint: &self.endpoint,
                method: "POST",
                path: "/v1/sessions/introspect",
                bearer: Some(self.token.as_str()),
                idempotency: None,
                headers: &[],
                body: &body,
            })
            .map_err(|_| WebhookError::Unavailable)?;
        if response.status != 200 || !response.content_type.starts_with("application/json") {
            return Err(WebhookError::InvalidRequest);
        }
        let session: SessionResponse =
            serde_json::from_slice(&response.body).map_err(|_| WebhookError::Unavailable)?;
        if !session.active {
            return Err(WebhookError::InvalidRequest);
        }
        if cookie_auth && mutation {
            let presented = anti_forgery.ok_or(WebhookError::InvalidRequest)?;
            if session.csrf_token.is_empty()
                || session.csrf_token.len() > 256
                || session
                    .csrf_token
                    .as_bytes()
                    .ct_eq(presented.as_bytes())
                    .unwrap_u8()
                    != 1
            {
                return Err(WebhookError::InvalidRequest);
            }
        }
        Principal::new(session.sub)
    }
}

impl SourceTrigger {
    #[must_use]
    pub fn shares_secret(&self, other: &Self) -> bool {
        self.token
            .as_bytes()
            .ct_eq(other.token.as_bytes())
            .unwrap_u8()
            == 1
    }

    /// # Errors
    /// Refuses missing or invalid configuration, secret files, endpoints or TLS material.
    pub fn from_environment() -> Result<Self, String> {
        Ok(Self {
            token: read_secret("LAYERX_WEBHOOKS_SOURCE_TRIGGER_TOKEN_FILE")?,
        })
    }

    /// # Errors
    /// Refuses missing or invalid configuration, secret files, endpoints or TLS material.
    pub fn operator_from_environment() -> Result<Self, String> {
        Ok(Self {
            token: read_secret("LAYERX_WEBHOOKS_OPERATOR_TOKEN_FILE")?,
        })
    }

    #[must_use]
    pub fn authorizes(&self, authorization: Option<&str>) -> bool {
        authorization
            .and_then(|value| value.strip_prefix("Bearer "))
            .is_some_and(|value| self.token.as_bytes().ct_eq(value.as_bytes()).unwrap_u8() == 1)
    }
}

impl AuthorityReceiptVerifier {
    pub fn new(
        component_url: &str,
        authority_url: &str,
        component_token: String,
        authority_token: String,
        ca_der: &[u8],
        identity: Identity,
        authorization: layerx_platform_gateway::SequencerAuthorization,
        network_id: String,
        wire_version: String,
        protocol_network_id: u32,
    ) -> Result<Self, String> {
        Ok(Self {
            client: Client::trusted(ClientIdentity::new(
                Certificate::from_der(ca_der).map_err(|e| e.to_string())?,
                Some(identity),
            )),
            component: Endpoint::parse(component_url)?,
            component_token: Zeroizing::new(component_token),
            authority: Endpoint::parse(authority_url)?,
            authority_token: Zeroizing::new(authority_token),
            sequencer_authorization: authorization,
            network_id,
            wire_version,
            protocol_network_id,
        })
    }

    fn ready_until(&self, deadline: Instant) -> bool {
        [
            (&self.component, self.component_token.as_str()),
            (&self.authority, self.authority_token.as_str()),
        ]
        .into_iter()
        .all(|(endpoint, token)| {
            let request = OutboundRequest {
                endpoint,
                method: "GET",
                path: "/readyz",
                bearer: Some(token),
                idempotency: None,
                headers: &[],
                body: &[],
            };
            self.client
                .request_until(&request, deadline)
                .is_ok_and(|response| response.status == 200)
        })
    }

    pub fn verify(&self, activity: &str) -> Result<VerifiedOperation, WebhookError> {
        fixed_hex::<32>(activity)?;
        let receipt_path = format!("/internal/v1/receipts/{activity}");
        let authority_path = format!("/internal/v1/activities/{activity}/authority");
        let receipt_response = self
            .client
            .request(&OutboundRequest {
                endpoint: &self.component,
                method: "GET",
                path: &receipt_path,
                bearer: Some(self.component_token.as_str()),
                idempotency: None,
                headers: &[],
                body: &[],
            })
            .map_err(|_| WebhookError::Unavailable)?;
        let authority_response = self
            .client
            .request(&OutboundRequest {
                endpoint: &self.authority,
                method: "GET",
                path: &authority_path,
                bearer: Some(self.authority_token.as_str()),
                idempotency: None,
                headers: &[],
                body: &[],
            })
            .map_err(|_| WebhookError::Unavailable)?;
        if receipt_response.status != 200
            || authority_response.status != 200
            || !receipt_response
                .content_type
                .starts_with("application/json")
            || !authority_response
                .content_type
                .starts_with("application/json")
        {
            return Err(WebhookError::VerificationRequired);
        }
        self.verify_documents(activity, &receipt_response.body, &authority_response.body)
    }

    pub fn verify_documents(
        &self,
        activity: &str,
        receipt_body: &[u8],
        authority_body: &[u8],
    ) -> Result<VerifiedOperation, WebhookError> {
        let expected = fixed_hex::<32>(activity)?;
        let receipt: ComponentReceipt =
            serde_json::from_slice(receipt_body).map_err(|_| WebhookError::VerificationRequired)?;
        let authority: AuthorityResponse = serde_json::from_slice(authority_body)
            .map_err(|_| WebhookError::VerificationRequired)?;
        let raw: serde_json::Value = serde_json::from_slice(authority_body)
            .map_err(|_| WebhookError::VerificationRequired)?;
        if let Some(header) = raw
            .get("batch_evidence")
            .and_then(|v| v.get("header_hex"))
            .and_then(serde_json::Value::as_str)
        {
            let bytes = hex_decode(header)?;
            let signature = raw["batch_evidence"]["header_signature"]
                .as_str()
                .ok_or(WebhookError::VerificationRequired)?;
            let header = layerx_proof::inclusion::verify_header(
                &bytes,
                &fixed_hex::<64>(signature)?,
                &self.sequencer_authorization,
            )
            .map_err(|_| WebhookError::VerificationRequired)?;
            if header.header().network_id() != self.protocol_network_id {
                return Err(WebhookError::VerificationRequired);
            }
        }
        if receipt.activity_id != activity
            || authority.activity_id != activity
            || authority.network_id != self.network_id
            || authority.wire_version != self.wire_version
            || authority.protocol_network_id != self.protocol_network_id
            || hex_decode(&authority.receipt)? != hex_decode(&receipt.receipt)?
        {
            return Err(WebhookError::VerificationRequired);
        }
        let sequencer = fixed_hex::<32>(&authority.sequencer_public_key)?;
        if sequencer != self.sequencer_authorization.public_key() {
            return Err(WebhookError::VerificationRequired);
        }
        let receipt_bytes = hex_decode(&receipt.receipt)?;
        let facts = AuthorityFacts::new(
            fixed_hex(&authority.batch_id)?,
            fixed_hex(&authority.asset)?,
            fixed_hex(&authority.previous_state_root)?,
            fixed_hex(&authority.resulting_state_root)?,
            sequencer,
        );
        let facts = match authority.batch_evidence {
            None => facts,
            Some(maintained) => {
                let verified = maintained
                    .authorize(
                        &receipt_bytes,
                        &facts.authorized(),
                        &self.sequencer_authorization,
                    )
                    .map_err(|_| WebhookError::VerificationRequired)?;
                AuthorityFacts::new(
                    verified.batch_id(),
                    verified.asset(),
                    verified.previous_state_root(),
                    verified.resulting_state_root(),
                    verified.sequencer_public_key(),
                )
            }
        };
        verify_activity_operation(
            &receipt_bytes,
            facts,
            &self.sequencer_authorization.public_key(),
            Some(expected),
        )
        .map_err(WebhookError::from)
    }
}

fn read_secret(name: &str) -> Result<Zeroizing<String>, String> {
    let path = env::var(name).map_err(|_| format!("{name} is required"))?;
    read_secret_file(std::path::Path::new(&path), name)
}

fn read_secret_file(path: &std::path::Path, name: &str) -> Result<Zeroizing<String>, String> {
    let mut value = fs::read_to_string(path).map_err(|error| error.to_string())?;
    while matches!(value.as_bytes().last(), Some(b'\r' | b'\n')) {
        value.pop();
    }
    if value.is_empty() || value.len() > 4096 {
        value.zeroize();
        return Err(format!("{name} does not contain a bounded secret"));
    }
    Ok(Zeroizing::new(value))
}

fn bounded_env(name: &str, maximum: usize) -> Result<String, String> {
    let value = env::var(name).map_err(|_| format!("{name} is required"))?;
    if !valid_identifier(&value, maximum) {
        return Err(format!("{name} is invalid"));
    }
    Ok(value)
}

fn valid_identifier(value: &str, maximum: usize) -> bool {
    !value.is_empty()
        && value.len() <= maximum
        && value
            .bytes()
            .all(|byte| byte.is_ascii_alphanumeric() || matches!(byte, b'-' | b'_' | b'.'))
}

fn session_cookie(header: &str) -> Result<&str, WebhookError> {
    let mut selected = None;
    for part in header.split(';') {
        let (name, value) = part
            .trim()
            .split_once('=')
            .ok_or(WebhookError::InvalidRequest)?;
        if name == "__Host-layerx-session" {
            if selected.is_some() || value.is_empty() || value.len() > 4096 {
                return Err(WebhookError::InvalidRequest);
            }
            selected = Some(value);
        }
    }
    selected.ok_or(WebhookError::InvalidRequest)
}

#[cfg(test)]
#[path = "repair_22_33_1328_authority.rs"]
mod authority_path_support;

#[cfg(test)]
mod authority_shape_tests {
    #[test]
    fn real_authority_shape_selects_attachment_without_null_or_unknown_fallback() {
        super::authority_path_support::authority_path();
    }
}

#[cfg(test)]
mod credential_tests {
    use super::read_secret_file;
    use std::fs;
    use std::os::unix::fs::PermissionsExt;
    use std::path::Path;
    use std::process::Command;

    #[test]
    fn consumes_the_cluster_generated_webhook_component_credential() {
        let mut nonce = [0_u8; 16];
        getrandom::fill(&mut nonce).unwrap_or_else(|error| panic!("nonce: {error}"));
        let root = std::env::temp_dir().join(format!(
            "webhook-component-{}",
            crate::encoding::hex_encode(&nonce)
        ));
        let script = Path::new(env!("CARGO_MANIFEST_DIR")).join("../tests/beta-cluster.sh");
        let status = Command::new("bash")
            .arg("-c")
            .arg("source \"$1\"; component_secrets_generate \"$2\"")
            .arg("component-secrets")
            .arg(script)
            .arg(&root)
            .status()
            .unwrap_or_else(|error| panic!("credential producer: {error}"));
        assert!(status.success());
        let path = root.join("webhook-component.token");
        let token = read_secret_file(&path, "LAYERX_WEBHOOKS_COMPONENT_TOKEN_FILE")
            .unwrap_or_else(|error| panic!("credential consumer: {error}"));
        assert!(token.len() == 64 && token.bytes().all(|byte| byte.is_ascii_hexdigit()));
        let bytes = fs::read(&path).unwrap_or_else(|error| panic!("material: {error}"));
        assert!(bytes.len() == 65 && bytes[64] == b'\n');
        assert!(token.as_bytes() == &bytes[..64]);
        let metadata = fs::metadata(&path).unwrap_or_else(|error| panic!("permissions: {error}"));
        assert_eq!(metadata.permissions().mode() & 0o777, 0o600);
        for name in ["gateway-component.token", "registry-node.token"] {
            let other = read_secret_file(&root.join(name), name)
                .unwrap_or_else(|error| panic!("other component: {error}"));
            assert!(token.as_str() != other.as_str());
        }
        fs::write(&path, format!("{}\r\n", token.as_str()))
            .unwrap_or_else(|error| panic!("CRLF: {error}"));
        let crlf = read_secret_file(&path, "component")
            .unwrap_or_else(|error| panic!("CRLF consumer: {error}"));
        assert!(token.as_str() == crlf.as_str());
        fs::write(&path, b"\r\n").unwrap_or_else(|error| panic!("empty material: {error}"));
        assert!(read_secret_file(&path, "component").is_err());
        fs::write(&path, vec![b'a'; 4097])
            .unwrap_or_else(|error| panic!("oversized material: {error}"));
        assert!(read_secret_file(&path, "component").is_err());
        fs::remove_dir_all(&root).unwrap_or_else(|error| panic!("cleanup: {error}"));
        assert!(read_secret_file(&path, "component").is_err());
    }
}

/// The live source uses node receipts and authenticated governance/commit evidence.
pub struct ReceiptVerifier {
    client: Client,
    node: Endpoint,
    comet: Endpoint,
    token: Zeroizing<String>,
    policy: crate::chain::ChainPolicy,
}
pub struct VerifiedSourceOperation {
    operation: VerifiedOperation,
    verification: Verification,
}
impl VerifiedSourceOperation {
    pub fn operation(&self) -> &VerifiedOperation {
        &self.operation
    }
    pub fn verification(&self) -> Verification {
        self.verification
    }
}
impl ReceiptVerifier {
    fn from_environment(identity: ClientIdentity) -> Result<Self, String> {
        let policy = fs::read(
            env::var("LAYERX_WEBHOOKS_CHAIN_POLICY_FILE")
                .map_err(|_| "LAYERX_WEBHOOKS_CHAIN_POLICY_FILE is required")?,
        )
        .map_err(|e| e.to_string())?;
        let policy: crate::chain::ChainPolicy =
            serde_json::from_slice(&policy).map_err(|e| e.to_string())?;
        policy.validate().map_err(|e| e.to_string())?;
        Ok(Self {
            client: Client::trusted(identity),
            node: Endpoint::parse(
                &env::var("LAYERX_WEBHOOKS_NODE_URL")
                    .map_err(|_| "LAYERX_WEBHOOKS_NODE_URL is required")?,
            )?,
            comet: Endpoint::parse(
                &env::var("LAYERX_WEBHOOKS_COMET_URL")
                    .map_err(|_| "LAYERX_WEBHOOKS_COMET_URL is required")?,
            )?,
            token: read_secret("LAYERX_WEBHOOKS_NODE_TOKEN_FILE")?,
            policy,
        })
    }
    pub fn new(
        url: &str,
        comet_url: &str,
        token: String,
        ca_der: &[u8],
        identity: Identity,
        policy: crate::chain::ChainPolicy,
    ) -> Result<Self, String> {
        policy.validate().map_err(|e| e.to_string())?;
        Ok(Self {
            client: Client::trusted(ClientIdentity::new(
                Certificate::from_der(ca_der).map_err(|e| e.to_string())?,
                Some(identity),
            )),
            node: Endpoint::parse(url)?,
            comet: Endpoint::parse(comet_url)?,
            token: Zeroizing::new(token),
            policy,
        })
    }
    fn ready_until(&self, deadline: Instant) -> bool {
        [(&self.node, "eth_chainId"), (&self.comet, "status")]
            .into_iter()
            .all(|(endpoint, method)| {
                let body =
                    serde_json::json!({"jsonrpc":"2.0", "id":1, "method":method, "params":[]})
                        .to_string();
                self.client
                    .request_until(
                        &OutboundRequest {
                            endpoint,
                            method: "POST",
                            path: "/",
                            bearer: Some(self.token.as_str()),
                            idempotency: None,
                            headers: &[],
                            body: body.as_bytes(),
                        },
                        deadline,
                    )
                    .is_ok_and(|r| {
                        r.status == 200
                            && serde_json::from_slice::<serde_json::Value>(&r.body).is_ok_and(|v| {
                                v["jsonrpc"] == "2.0"
                                    && v["id"] == 1
                                    && v.get("error").is_none()
                                    && !v["result"].is_null()
                                    && (method != "status"
                                        || v["result"]["node_info"]["network"]
                                            == self.policy.chain_id)
                            })
                    })
            })
    }
    fn rpc(
        &self,
        endpoint: &Endpoint,
        method: &str,
        params: serde_json::Value,
    ) -> Result<serde_json::Value, WebhookError> {
        let body = serde_json::to_vec(
            &serde_json::json!({"jsonrpc":"2.0", "id":1, "method":method, "params":params}),
        )
        .map_err(|_| WebhookError::InvalidRequest)?;
        let response = self
            .client
            .request(&OutboundRequest {
                endpoint,
                method: "POST",
                path: "/",
                bearer: Some(self.token.as_str()),
                idempotency: None,
                headers: &[],
                body: &body,
            })
            .map_err(|_| WebhookError::Unavailable)?;
        if response.status != 200 || !response.content_type.starts_with("application/json") {
            return Err(WebhookError::Unavailable);
        }
        let mut value: serde_json::Value = serde_json::from_slice(&response.body)
            .map_err(|_| WebhookError::VerificationRequired)?;
        if value["jsonrpc"] != "2.0" || value["id"] != 1 || value.get("error").is_some() {
            return Err(WebhookError::VerificationRequired);
        }
        value
            .as_object_mut()
            .and_then(|v| v.remove("result"))
            .filter(|v| !v.is_null())
            .ok_or(WebhookError::VerificationRequired)
    }
    pub fn verify(&self, activity: &str) -> Result<VerifiedSourceOperation, WebhookError> {
        let expected = fixed_hex::<32>(activity)?;
        let receipt_json = self.rpc(
            &self.node,
            "pax_sequencerReceipt",
            serde_json::json!([format!("0x{activity}")]),
        )?;
        if receipt_json["rung"] != "committed" {
            return Err(WebhookError::VerificationRequired);
        }
        let receipt = hex_decode(
            receipt_json["receipt"]
                .as_str()
                .and_then(|v| v.strip_prefix("0x"))
                .ok_or(WebhookError::VerificationRequired)?,
        )?;
        // The node's height selects evidence to fetch; only the signed receipt
        // and authenticated commit can establish that height below.
        let height = u64::from_str_radix(
            receipt_json["height"]
                .as_str()
                .and_then(|value| value.strip_prefix("0x"))
                .ok_or(WebhookError::VerificationRequired)?,
            16,
        )
        .map_err(|_| WebhookError::VerificationRequired)?;
        if height <= 1 {
            return Err(WebhookError::VerificationRequired);
        }
        let commit_json = self.rpc(
            &self.comet,
            "commit",
            serde_json::json!({"height":height.to_string()}),
        )?;
        let commit = crate::chain::CommitCertificate::from_comet(&commit_json)?;
        if commit.height()? != height {
            return Err(WebhookError::VerificationRequired);
        }
        let block = self.rpc(
            &self.comet,
            "block",
            serde_json::json!({"height":height.to_string()}),
        )?;
        let transactions = block["block"]["data"]["txs"]
            .as_array()
            .ok_or(WebhookError::VerificationRequired)?
            .iter()
            .map(|v| {
                crate::encoding::base64_decode(
                    v.as_str().ok_or(WebhookError::VerificationRequired)?,
                )
            })
            .collect::<Result<Vec<_>, _>>()?;
        let ethereum = self.rpc(
            &self.node,
            "eth_getBlockByNumber",
            serde_json::json!([format!("0x{height:x}"), false]),
        )?;
        let activity_ids = ethereum["transactions"]
            .as_array()
            .ok_or(WebhookError::VerificationRequired)?
            .iter()
            .map(|v| {
                fixed_hex(
                    v.as_str()
                        .and_then(|v| v.strip_prefix("0x"))
                        .ok_or(WebhookError::VerificationRequired)?,
                )
            })
            .collect::<Result<Vec<[u8; 32]>, _>>()?;
        let proof = self.rpc(&self.comet, "abci_query", serde_json::json!({"path":"/store/sequencer/key", "data":"0x03", "height":(height-1).to_string(), "prove":true}))?;
        let response = &proof["response"];
        if response["height"]
            .as_str()
            .and_then(|v| v.parse::<u64>().ok())
            != Some(height - 1)
        {
            return Err(WebhookError::VerificationRequired);
        }
        let governed_keys = crate::chain::Membership::from_abci(response)?;
        let document = crate::chain::NodeReceiptEvidence {
            receipt,
            commit,
            governed_keys,
            transactions,
            activity_ids,
        };
        let served_hash = receipt_json["blockHash"]
            .as_str()
            .and_then(|v| v.strip_prefix("0x"))
            .ok_or(WebhookError::VerificationRequired)?;
        self.verify_evidence(&document, expected, fixed_hex::<32>(served_hash)?)
    }
    /// Verifies transported node evidence before granting a committed source label.
    pub fn verify_evidence(
        &self,
        document: &crate::chain::NodeReceiptEvidence,
        activity: [u8; 32],
        served_hash: [u8; 32],
    ) -> Result<VerifiedSourceOperation, WebhookError> {
        let operation = document.verify(&self.policy, activity)?;
        let committed = document
            .commit
            .verify(&self.policy, &document.governed_keys.proof)?;
        if served_hash != committed.block_hash {
            return Err(WebhookError::VerificationRequired);
        }
        Ok(VerifiedSourceOperation {
            operation,
            verification: Verification::BlockCommitted,
        })
    }
}
