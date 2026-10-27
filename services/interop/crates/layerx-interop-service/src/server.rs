use crate::config::{
    decode_hex, parse_hex32, Ap2KeyPin, Config, FiatProviderPin, VisaAgentPin, VisaTargetPin,
};
use crate::payer_tx::{decode_pay_exact, PayExactCall};
use crate::relay::{ChainRelay, RelayError, RelayJournal, RelayOutcome};
use base64::engine::general_purpose::STANDARD;
use base64::Engine as _;
use ed25519_dalek::{Signature, Verifier as _, VerifyingKey as Ed25519Key};
use layerx_ap2::{
    authorize_payment, Ap2Error, KeyResolver, KeyUse, LayerXAssetBinding, MandateMode,
    MandateVerifier, Merchant, ProtectedHeader, VerificationContext,
};
use layerx_client::sequencer::ReceiptKeyEntry;
use layerx_fiat::{
    EvidenceClass, ExecutedFiatOutcome, ExternalId, FiatAdapter, FiatError, FiatIntent,
    FiatIntentKind, FiatJourneyState, FiatPlane, FiatPlaneResult, FiatRail, PlaneFiatOutcome,
    ProviderEvidence, ProviderVerifier, TokenReference, VerifiedProviderFacts,
};
use layerx_interop_gateway::adapter::AdapterId;
use layerx_interop_gateway::evidence::{SequencerEvidence, SettlementRung};
use layerx_interop_gateway::gateway::{TranslationKind, TranslationRequest, TranslationStatus};
use layerx_interop_gateway::principal::PrincipalId;
use layerx_interop_gateway::server::{
    interop_gateway_routes, ExternalState, HostedAdapter, IngressTransport, InteropRoute,
};
use layerx_interop_gateway::trace::TraceId;
use layerx_interop_gateway::GatewayCore;
use layerx_migrate::ramp_v2::{SourceSettlementRequestV2, SourceSettlementResponseV2};
use layerx_migrate::{ExternalHistorySink, MigrationError, SourceEvidence, SourceVerifier};
use layerx_platform_gateway::http::{IncomingRequest, OutboundRequest, OutgoingResponse};
use layerx_platform_gateway::store::{
    Completion, KeyRecord, Reservation, ReservationRequest, TapCredentialRecord,
    TapNonceConsumption,
};
use layerx_platform_gateway::{
    authenticate_gateway_key, gateway_audit_event, gateway_digest, verify_submission, AccessError,
    ModuleRegistry, VerifiedSubmission, VerifiedTransfer,
};
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_proof::receipt::verify_sequencer_signature;
use layerx_ucp::{
    Capability, CheckoutStatus, CheckoutSubmission, ExecutedUcpPayment, NegotiatedCapabilities,
    OrderMetadata, PaymentHandler, PlatformProfile, UcpAdapter, UcpError, UcpIdempotencyKey,
    UcpPaymentIntent, UcpPaymentPlane, UcpPlaneResult,
};
use layerx_visa_tap::{
    prepare_trusted_intent, AgentIntent, AgentPublicKey, CredentialBinding, CredentialBindingStore,
    KeyStatus, MerchantOperationResult, RegisteredAgentKey, TapError, TapRequest, TapVerifier,
    TrustedAgentRegistry,
};
use layerx_x402::buyer::{BuyerPaymentPlane, PaymentBuildRequest, SupportedKind};
use layerx_x402::facilitator::{
    FacilitatorPaymentRequest, FacilitatorPlane, FacilitatorRequest, FacilitatorSettlementOutcome,
    PlaneVerifyOutcome, SettlementStep,
};
use layerx_x402::model::{PaymentRequirements, X402Error};
use layerx_x402::transport::encode_payment_required;
use layerx_x402::transport::{decode_facilitator_request, TransportKind, TransportValue};
use layerx_x402::{Buyer, Facilitator, PaymentRequired, Seller};
use p256::ecdsa::VerifyingKey as P256Key;
use serde::{de::DeserializeOwned, Deserialize, Serialize};
use serde_json::{json, Value};
use sha2::{Digest, Sha256};
use std::collections::BTreeMap;
use std::time::{Duration, Instant, SystemTime, UNIX_EPOCH};
use subtle::ConstantTimeEq;
use zeroize::Zeroizing;

const MAX_BODY: usize = 512 * 1024;
const FIAT_EVIDENCE_SIGNATURE_DOMAIN: &[u8] = b"LayerX/interop/fiat/provider-evidence/v1\0";

pub fn route(config: &Config, request: &IncomingRequest) -> OutgoingResponse {
    if request.headers.contains_key("x-layerx-principal")
        || request.headers.contains_key("x-layerx-api-key")
    {
        return failure(400, "untrusted_identity_header", None);
    }
    let Ok(parsed) = interop_gateway_routes(&request.method, &request.path) else {
        return failure(404, "not_found", None);
    };
    match parsed {
        InteropRoute::Live => live(),
        InteropRoute::Ready => ready(config),
        InteropRoute::AdapterMetadata => metadata(config),
        route => authenticated(config, request, &route),
    }
}

fn authenticated(
    config: &Config,
    request: &IncomingRequest,
    route: &InteropRoute<'_>,
) -> OutgoingResponse {
    if request.body.len() > MAX_BODY {
        return failure(400, "request_too_large", None);
    }
    let Some(authorization) = request.headers.get("authorization") else {
        return failure(401, "api_key_required", None);
    };
    let record = match authenticate_gateway_key(&config.store, authorization) {
        Ok(record) => record,
        Err(AccessError::Unauthenticated) => return failure(401, "api_key_required", None),
        Err(AccessError::PersistenceUnavailable) => {
            return failure(503, "persistence_unavailable", Some(5))
        }
    };
    let trace = trace(request);
    if let InteropRoute::Resume { operation } = &route {
        return resume_operation(config, &record, authorization, &trace, operation);
    }
    reserve_operation(config, request, route, record, trace)
}

fn reserve_operation(
    config: &Config,
    request: &IncomingRequest,
    route: &InteropRoute<'_>,
    record: KeyRecord,
    trace: TraceId,
) -> OutgoingResponse {
    let observed_at = now().unwrap_or(1);
    let adapter = route.adapter().map_or("operation", HostedAdapter::surface);
    let request_digest = gateway_digest(&[
        b"interop-ingress-v1",
        request.method.as_bytes(),
        request.path.as_bytes(),
        &request.body,
    ]);
    let callback_identity = uses_callback_identity(route);
    let idempotency = if callback_identity {
        request_digest.as_str()
    } else {
        match request.headers.get("idempotency-key") {
            Some(value) if valid_identifier(value, 128) => value,
            _ => return failure(400, "idempotency_key_required", None),
        }
    };
    let scope = gateway_digest(&[
        b"interop-operation-v1",
        record.principal_digest.as_bytes(),
        adapter.as_bytes(),
        idempotency.as_bytes(),
    ]);
    let audit = gateway_audit_event(
        &record.principal_digest,
        "interop_ingress",
        adapter,
        "attempted",
        observed_at,
    );
    let Ok(continuation) = continuation(request) else {
        return failure(400, "request_too_large", None);
    };
    match config.store.reserve(
        &record,
        ReservationRequest {
            idempotency_scope: &scope,
            request_digest: &request_digest,
            now: observed_at,
            retention_seconds: config.idempotency_seconds,
            activity_id: &request_digest,
            protocol_idempotency_key: idempotency,
            principal_digest: &record.principal_digest,
            audit_event: &audit,
            continuation: &continuation,
        },
    ) {
        Ok(Reservation::Revoked) => return failure(401, "api_key_required", None),
        Ok(Reservation::RateLimited {
            retry_after_seconds,
        }) => return failure(429, "quota_exceeded", Some(retry_after_seconds)),
        Ok(Reservation::Existing {
            digest,
            state,
            response,
            principal,
            ..
        }) => {
            if principal
                .as_bytes()
                .ct_eq(record.principal_digest.as_bytes())
                .unwrap_u8()
                != 1
            {
                return failure(404, "operation_not_found", None);
            }
            if digest
                .as_bytes()
                .ct_eq(request_digest.as_bytes())
                .unwrap_u8()
                != 1
            {
                return failure(409, "idempotency_conflict", None);
            }
            if state != "pending" {
                return stored_response(&state, &response, &trace, &scope);
            }
        }
        Ok(Reservation::Reserved) => {}
        Err(_) => return failure(503, "persistence_unavailable", Some(5)),
    }
    complete_operation(
        config,
        request,
        route,
        IngressOperation {
            record,
            trace,
            scope,
            request_digest,
            adapter,
            observed_at,
        },
    )
}

fn uses_callback_identity(route: &InteropRoute<'_>) -> bool {
    matches!(
        route,
        InteropRoute::Ap2VerifyMandates
            | InteropRoute::Ap2Execute
            | InteropRoute::VisaVerifyIntent
            | InteropRoute::VisaExecuteIntent
            | InteropRoute::FiatCallback { .. }
    )
}

struct IngressOperation {
    record: KeyRecord,
    trace: TraceId,
    scope: String,
    request_digest: String,
    adapter: &'static str,
    observed_at: u64,
}

fn complete_operation(
    config: &Config,
    request: &IncomingRequest,
    route: &InteropRoute<'_>,
    operation: IngressOperation,
) -> OutgoingResponse {
    let IngressOperation {
        record,
        trace,
        scope,
        request_digest,
        adapter,
        observed_at,
    } = operation;
    let Ok(principal) = PrincipalId::new(record.principal_digest.clone()) else {
        return failure(503, "persistence_unavailable", Some(5));
    };
    let dispatched = dispatch(config, request, route, &record, &principal, &trace, &scope);
    let body = dispatched.body(&trace, &scope);
    let completion_audit = gateway_audit_event(
        &record.principal_digest,
        "interop_ingress",
        adapter,
        dispatched.durable_state,
        observed_at,
    );
    if config
        .store
        .complete(Completion {
            idempotency_scope: &scope,
            request_digest: &request_digest,
            state: dispatched.durable_state,
            response_hex: &hex(&body),
            receipt_hex: dispatched.receipt_hex.as_deref().unwrap_or(""),
            activity_id: dispatched.activity_id.as_deref(),
            principal_digest: &record.principal_digest,
            audit_event: &completion_audit,
        })
        .is_err()
    {
        return failure(503, "persistence_unavailable", Some(5));
    }
    OutgoingResponse {
        content_type: "application/json".to_owned(),
        headers: Vec::new(),
        status: dispatched.status,
        body,
        retry_after: None,
    }
}

fn resume_operation(
    config: &Config,
    record: &KeyRecord,
    authorization: &str,
    trace: &TraceId,
    operation: &str,
) -> OutgoingResponse {
    match config.store.operation(operation) {
        Ok(Some(stored))
            if stored
                .principal
                .as_bytes()
                .ct_eq(record.principal_digest.as_bytes())
                .unwrap_u8()
                == 1 =>
        {
            if stored.state == "pending" && !stored.continuation.is_empty() {
                match resumed_request(&stored.continuation, authorization) {
                    Ok(resumed) => match interop_gateway_routes(&resumed.method, &resumed.path) {
                        Ok(resumed_route)
                            if !matches!(resumed_route, InteropRoute::Resume { .. }) =>
                        {
                            authenticated(config, &resumed, &resumed_route)
                        }
                        _ => failure(503, "continuation_invalid", Some(5)),
                    },
                    Err(()) => failure(503, "continuation_invalid", Some(5)),
                }
            } else {
                stored_response(&stored.state, &stored.response, trace, operation)
            }
        }
        Ok(Some(_) | None) => failure(404, "operation_not_found", None),
        Err(_) => failure(503, "persistence_unavailable", Some(5)),
    }
}

struct Dispatch {
    status: u16,
    durable_state: &'static str,
    result: Value,
    error: Option<&'static str>,
    receipt_hex: Option<String>,
    activity_id: Option<String>,
}

#[derive(Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
struct DurableContinuation {
    method: String,
    path: String,
    content_type: String,
    idempotency_key: Option<String>,
    body: String,
}

fn continuation(request: &IncomingRequest) -> Result<String, ()> {
    serde_json::to_string(&DurableContinuation {
        method: request.method.clone(),
        path: request.path.clone(),
        content_type: request
            .headers
            .get("content-type")
            .cloned()
            .unwrap_or_default(),
        idempotency_key: request.headers.get("idempotency-key").cloned(),
        body: hex(&request.body),
    })
    .map_err(|_| ())
}

fn resumed_request(encoded: &str, authorization: &str) -> Result<IncomingRequest, ()> {
    let continuation: DurableContinuation = serde_json::from_str(encoded).map_err(|_| ())?;
    if !matches!(continuation.method.as_str(), "GET" | "POST")
        || !(continuation.path.starts_with("/v1/")
            || (continuation.method == "POST"
                && matches!(
                    continuation.path.as_str(),
                    "/v2/migration/accounts"
                        | "/v2/migration/assets"
                        | "/v2/migration/history"
                        | "/v2/migration/history/read"
                )))
        || continuation.path.starts_with("/v1/operations/")
    {
        return Err(());
    }
    let mut headers = BTreeMap::new();
    headers.insert("authorization".to_owned(), authorization.to_owned());
    headers.insert("content-type".to_owned(), continuation.content_type);
    if let Some(idempotency) = continuation.idempotency_key {
        headers.insert("idempotency-key".to_owned(), idempotency);
    }
    Ok(IncomingRequest {
        method: continuation.method,
        path: continuation.path,
        headers,
        body: decode_hex(&continuation.body, MAX_BODY).map_err(|_| ())?,
    })
}

impl Dispatch {
    fn result(status: u16, state: &'static str, result: Value) -> Self {
        Self {
            status,
            durable_state: state,
            result,
            error: None,
            receipt_hex: None,
            activity_id: None,
        }
    }

    fn error(status: u16, state: &'static str, code: &'static str) -> Self {
        Self {
            status,
            durable_state: state,
            result: Value::Null,
            error: Some(code),
            receipt_hex: None,
            activity_id: None,
        }
    }

    fn body(&self, trace: &TraceId, operation: &str) -> Vec<u8> {
        let value = self.error.map_or_else(
            || json!({ "ok": true, "operation": operation, "result": self.result, "trace": trace.as_str() }),
            |code| json!({ "ok": false, "operation": operation, "error": { "code": code }, "trace": trace.as_str() }),
        );
        value.to_string().into_bytes()
    }
}

fn dispatch(
    config: &Config,
    request: &IncomingRequest,
    route: &InteropRoute<'_>,
    record: &KeyRecord,
    principal: &PrincipalId,
    trace: &TraceId,
    operation: &str,
) -> Dispatch {
    match *route {
        InteropRoute::Resume { .. } => {
            Dispatch::result(202, "pending", json!({ "state": "pending" }))
        }
        InteropRoute::X402Supported { transport } => x402_supported(config, transport),
        InteropRoute::X402BuyerBuild { transport } => {
            x402_buyer(config, request, transport, trace, operation)
        }
        InteropRoute::X402SellerOffer { transport } => x402_seller(request, transport),
        InteropRoute::X402Verify { transport } => {
            x402_verify(config, request, transport, record, principal, trace)
        }
        InteropRoute::X402Settle { transport } => {
            x402_settle(config, request, transport, record, principal, trace)
        }
        InteropRoute::Ap2VerifyMandates => {
            ap2(config, request, record, principal, trace, operation, false)
        }
        InteropRoute::Ap2Execute => ap2(config, request, record, principal, trace, operation, true),
        InteropRoute::UcpComplete => ucp(config, request, record, principal, trace, operation),
        InteropRoute::VisaVerifyIntent => {
            visa(config, request, record, principal, trace, operation, false)
        }
        InteropRoute::VisaExecuteIntent => {
            visa(config, request, record, principal, trace, operation, true)
        }
        InteropRoute::FiatCallback { adapter } => fiat(
            config, request, record, principal, trace, operation, adapter,
        ),
        InteropRoute::MigrationAccountV2 => {
            migration_account_v2(config, request, record, principal, trace)
        }
        InteropRoute::MigrationAssetV2 => migration_asset_v2(config, request, record, trace),
        InteropRoute::MigrationHistoryImportV2 => {
            migration_history_import_v2(config, request, record, principal, trace)
        }
        InteropRoute::MigrationHistoryReadV2 => {
            migration_history_read_v2(config, request, principal)
        }
        InteropRoute::Live | InteropRoute::Ready | InteropRoute::AdapterMetadata => {
            Dispatch::error(404, "refused", "not_found")
        }
    }
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct MigrationHistoryImportRequestV2 {
    chain: MigrationSourceV2,
    ownership_evidence: String,
    source_evidence: String,
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct MigrationHistoryReadRequestV2 {
    cursor: Option<String>,
    limit: usize,
}

fn history_evidence(encoded: &str) -> Result<SourceEvidence, MigrationError> {
    let decoded = STANDARD
        .decode(encoded)
        .map_err(|_| MigrationError::InvalidEvidence)?;
    if STANDARD.encode(&decoded) != encoded {
        return Err(MigrationError::InvalidEvidence);
    }
    SourceEvidence::new(decoded)
}

fn migration_history_import_v2(
    config: &Config,
    request: &IncomingRequest,
    record: &KeyRecord,
    principal: &PrincipalId,
    trace: &TraceId,
) -> Dispatch {
    let Ok(body) = direct_body::<MigrationHistoryImportRequestV2>(request) else {
        return Dispatch::error(400, "refused", "invalid_migration_request");
    };
    let Some(profile) = config.migration_v2.as_ref() else {
        return Dispatch::error(503, "refused", "migration_v2_unconfigured");
    };
    let Some(history) = profile.history.as_ref() else {
        return Dispatch::error(503, "refused", "migration_history_unconfigured");
    };
    let Ok(identity) = parse_hex32(&record.signer_public_key) else {
        return Dispatch::error(503, "refused", "authenticated_signer_invalid");
    };
    if identity == [0; 32] {
        return Dispatch::error(503, "refused", "authenticated_signer_invalid");
    }
    let imported = (|| -> Result<Value, MigrationError> {
        let ownership = history_evidence(&body.ownership_evidence)?;
        let evidence = history_evidence(&body.source_evidence)?;
        let page = match body.chain {
            MigrationSourceV2::Ethereum => profile
                .ethereum
                .as_ref()
                .ok_or(MigrationError::Configuration)?
                .verify_owned_history(&ownership, &evidence, identity, trace)?,
            MigrationSourceV2::Solana => profile
                .solana
                .as_ref()
                .ok_or(MigrationError::Configuration)?
                .verify_owned_history(&ownership, &evidence, identity, trace)?,
        };
        {
            let mut sink = history.lock().map_err(|_| MigrationError::StorageRefused)?;
            sink.store_external(principal, &page, trace)?;
        }
        match body.chain {
            MigrationSourceV2::Ethereum => profile
                .ethereum
                .as_ref()
                .ok_or(MigrationError::Configuration)?
                .commit_history(&evidence, &page, trace)?,
            MigrationSourceV2::Solana => profile
                .solana
                .as_ref()
                .ok_or(MigrationError::Configuration)?
                .commit_history(&evidence, &page, trace)?,
        }
        Ok(
            json!({"version":"external-history-v2", "state":"external-history-imported",
            "record_count":page.records().len(), "next_cursor":page.next_cursor().map(|value| hex(&value)),
            "evidence_digest":hex(&page.evidence_digest()), "provenance":"external", "layerx_receipt":false}),
        )
    })();
    match imported {
        Ok(value) => Dispatch::result(200, "external_history_imported", value),
        Err(error) => migration_history_error(error),
    }
}

fn migration_history_read_v2(
    config: &Config,
    request: &IncomingRequest,
    principal: &PrincipalId,
) -> Dispatch {
    let Ok(body) = direct_body::<MigrationHistoryReadRequestV2>(request) else {
        return Dispatch::error(400, "refused", "invalid_migration_request");
    };
    let Some(history) = config
        .migration_v2
        .as_ref()
        .and_then(|profile| profile.history.as_ref())
    else {
        return Dispatch::error(503, "refused", "migration_history_unconfigured");
    };
    let cursor = match body.cursor {
        None => None,
        Some(text) => match parse_hex32(&text) {
            Ok(value) if value != [0; 32] && hex(&value) == text => Some(value),
            _ => return Dispatch::error(400, "refused", "invalid_history"),
        },
    };
    let page = history
        .lock()
        .map_err(|_| MigrationError::StorageRefused)
        .and_then(|sink| sink.read(principal, cursor, body.limit));
    match page {
        Ok(page) => Dispatch::result(200, "external_history_read", page.json()),
        Err(error) => migration_history_error(error),
    }
}

fn migration_history_error(error: MigrationError) -> Dispatch {
    let (status, state) = match error {
        MigrationError::SourcePending => (202, "pending"),
        MigrationError::RpcUnavailable
        | MigrationError::RpcDivergence
        | MigrationError::RpcResponseMismatch => (503, "pending"),
        MigrationError::RpcRateLimited { .. } => (429, "pending"),
        MigrationError::Configuration
        | MigrationError::StorageRefused
        | MigrationError::CheckpointIntegrity => (503, "refused"),
        MigrationError::CheckpointConflict
        | MigrationError::SourceDisplaced
        | MigrationError::SourceReverted => (409, "refused"),
        _ => (400, "refused"),
    };
    Dispatch::error(status, state, error.code())
}

#[derive(Deserialize)]
#[serde(rename_all = "lowercase")]
enum MigrationSourceV2 {
    Ethereum,
    Solana,
}

impl MigrationSourceV2 {
    const fn label(&self) -> &'static str {
        match self {
            Self::Ethereum => "ethereum",
            Self::Solana => "solana",
        }
    }
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct MigrationAssetRequestV2 {
    order_digest: [u8; 32],
    chain: MigrationSourceV2,
    source_evidence: String,
}

fn migration_asset_v2(
    config: &Config,
    request: &IncomingRequest,
    record: &KeyRecord,
    trace: &TraceId,
) -> Dispatch {
    let Ok(body) = direct_body::<MigrationAssetRequestV2>(request) else {
        return Dispatch::error(400, "refused", "invalid_migration_request");
    };
    let Some(ramp) = config
        .migration_v2
        .as_ref()
        .and_then(|profile| profile.ramp_intake.as_ref())
    else {
        return Dispatch::error(503, "refused", "migration_ramp_unconfigured");
    };
    let Ok(identity) = parse_hex32(&record.signer_public_key) else {
        return Dispatch::error(503, "refused", "authenticated_signer_invalid");
    };
    if identity == [0; 32] {
        return Dispatch::error(503, "refused", "authenticated_signer_invalid");
    }
    let Some(customer_authorization) = request.headers.get("authorization") else {
        return Dispatch::error(401, "refused", "api_key_required");
    };
    let Ok(decoded) = STANDARD.decode(&body.source_evidence) else {
        return Dispatch::error(400, "refused", "invalid_evidence");
    };
    if STANDARD.encode(&decoded) != body.source_evidence {
        return Dispatch::error(400, "refused", "invalid_evidence");
    }
    let Ok(evidence) = SourceEvidence::new(decoded) else {
        return Dispatch::error(400, "refused", "invalid_evidence");
    };
    let Ok(settlement) =
        SourceSettlementRequestV2::new(body.order_digest, body.chain.label(), &evidence)
    else {
        return Dispatch::error(400, "refused", "invalid_migration_request");
    };
    let Ok(encoded) = serde_json::to_vec(&settlement) else {
        return Dispatch::error(503, "refused", "migration_encoding_invalid");
    };
    let service_authorization = zeroize::Zeroizing::new(format!("Bearer {}", ramp.token.as_str()));
    let Ok(upstream) = config.client.request_migration_source_settlement(
        &ramp.endpoint,
        service_authorization.as_str(),
        &OutboundRequest {
            method: "POST",
            path: "/internal/v2/source-settlements",
            idempotency: request.headers.get("idempotency-key").map(String::as_str),
            content_type: "application/json",
            body: &encoded,
        },
        Some(trace.as_str()),
        customer_authorization,
        &identity,
    ) else {
        return Dispatch::error(503, "pending", "migration_ramp_unavailable");
    };
    if !matches!(upstream.status, 200 | 202) {
        return Dispatch::error(
            if matches!(upstream.status, 401 | 403 | 404 | 409 | 422) {
                upstream.status
            } else {
                503
            },
            if matches!(upstream.status, 401 | 403 | 404 | 409 | 422) {
                "refused"
            } else {
                "pending"
            },
            "migration_source_settlement_refused",
        );
    }
    if upstream.content_type != "application/json" || upstream.body.len() > MAX_BODY {
        return Dispatch::error(503, "refused", "migration_response_invalid");
    }
    let Ok(response) = serde_json::from_slice::<SourceSettlementResponseV2>(&upstream.body) else {
        return Dispatch::error(503, "refused", "migration_response_invalid");
    };
    if response.validate(&settlement, &evidence).is_err() {
        return Dispatch::error(503, "refused", "migration_response_mismatch");
    }
    let durable_state = match response.state.as_str() {
        "done" => "external_source_recorded",
        "layerx_refused" => "refused",
        _ => "pending",
    };
    Dispatch::result(
        upstream.status,
        durable_state,
        json!({
            "source_settlement": response,
            "provenance": "external-custody",
            "custody_label": "External custody: this independent market maker controls the off-platform funds and payout.",
            "layerx_receipt": false,
        }),
    )
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct MigrationAccountRequestV2 {
    chain: MigrationSourceV2,
    source_evidence: String,
}

fn migration_account_v2(
    config: &Config,
    request: &IncomingRequest,
    record: &KeyRecord,
    principal: &PrincipalId,
    trace: &TraceId,
) -> Dispatch {
    let Ok(body) = direct_body::<MigrationAccountRequestV2>(request) else {
        return Dispatch::error(400, "refused", "invalid_migration_request");
    };
    let Some(profile) = config.migration_v2.as_ref() else {
        return Dispatch::error(503, "refused", "migration_v2_unconfigured");
    };
    let Ok(identity) = parse_hex32(&record.signer_public_key) else {
        return Dispatch::error(503, "refused", "authenticated_signer_invalid");
    };
    if identity == [0; 32] {
        return Dispatch::error(503, "refused", "authenticated_signer_invalid");
    }
    let Ok(decoded) = STANDARD.decode(&body.source_evidence) else {
        return Dispatch::error(400, "refused", "invalid_evidence");
    };
    if STANDARD.encode(&decoded) != body.source_evidence {
        return Dispatch::error(400, "refused", "invalid_evidence");
    }
    let Ok(evidence) = SourceEvidence::new(decoded) else {
        return Dispatch::error(400, "refused", "invalid_evidence");
    };
    let confirmed = match body.chain {
        MigrationSourceV2::Ethereum => {
            let Some(verifier) = profile.ethereum.as_ref() else {
                return Dispatch::error(503, "refused", "migration_source_unconfigured");
            };
            profile.mapping_store.confirm(
                principal,
                identity,
                &evidence,
                verifier,
                &profile.paxeer_binding,
                trace,
            )
        }
        MigrationSourceV2::Solana => {
            let Some(verifier) = profile.solana.as_ref() else {
                return Dispatch::error(503, "refused", "migration_source_unconfigured");
            };
            profile.mapping_store.confirm(
                principal,
                identity,
                &evidence,
                verifier,
                &profile.paxeer_binding,
                trace,
            )
        }
    };
    match confirmed {
        Ok(mapping) => match serde_json::to_value(mapping) {
            Ok(value) => Dispatch::result(200, "external_mapping_confirmed", value),
            Err(_) => Dispatch::error(503, "refused", "mapping_encoding_invalid"),
        },
        Err(error) => {
            let (status, state) = match error {
                MigrationError::SourcePending => (202, "pending"),
                MigrationError::RpcUnavailable
                | MigrationError::RpcDivergence
                | MigrationError::RpcResponseMismatch => (503, "pending"),
                MigrationError::RpcRateLimited { .. } => (429, "pending"),
                MigrationError::Configuration
                | MigrationError::StorageRefused
                | MigrationError::CheckpointIntegrity => (503, "refused"),
                MigrationError::CheckpointConflict
                | MigrationError::SourceDisplaced
                | MigrationError::SourceReverted => (409, "refused"),
                _ => (400, "refused"),
            };
            Dispatch::error(status, state, error.code())
        }
    }
}

#[cfg(test)]
mod migration_v2_request_tests {
    use super::{
        direct_body, history_evidence, resumed_request, DurableContinuation,
        MigrationAccountRequestV2, MigrationHistoryImportRequestV2, MigrationHistoryReadRequestV2,
    };
    use layerx_platform_gateway::http::IncomingRequest;
    use std::collections::BTreeMap;

    fn request(body: &[u8]) -> IncomingRequest {
        IncomingRequest {
            method: "POST".to_owned(),
            path: "/v2/migration/accounts".to_owned(),
            headers: BTreeMap::from([("content-type".to_owned(), "application/json".to_owned())]),
            body: body.to_vec(),
        }
    }

    #[test]
    fn mapping_body_cannot_supply_owner_or_receipt_authority() {
        assert!(direct_body::<MigrationAccountRequestV2>(&request(
            br#"{"chain":"ethereum","source_evidence":"AQ=="}"#,
        ))
        .is_ok());
        for body in [
            br#"{"chain":"ethereum","source_evidence":"AQ==","principal":"other"}"#.as_slice(),
            br#"{"chain":"ethereum","source_evidence":"AQ==","layerx_identity":"other"}"#
                .as_slice(),
            br#"{"chain":"ethereum","source_evidence":"AQ==","layerx_receipt":true}"#.as_slice(),
            br#"{"chain":"paxeer","source_evidence":"AQ=="}"#.as_slice(),
            br#"{"chain":"ethereum","chain":"solana","source_evidence":"AQ=="}"#.as_slice(),
        ] {
            assert!(direct_body::<MigrationAccountRequestV2>(&request(body)).is_err());
        }
    }

    #[test]
    fn pending_mapping_resumes_only_the_exact_authenticated_route() {
        let encoded = serde_json::to_string(&DurableContinuation {
            method: "POST".to_owned(),
            path: "/v2/migration/accounts".to_owned(),
            content_type: "application/json".to_owned(),
            idempotency_key: Some("mapping-key".to_owned()),
            body: "7b7d".to_owned(),
        })
        .unwrap_or_else(|error| panic!("continuation encoding: {error}"));
        let resumed = resumed_request(&encoded, "Bearer renewed-key")
            .unwrap_or_else(|()| panic!("exact migration continuation refused"));
        assert_eq!(resumed.path, "/v2/migration/accounts");
        assert_eq!(
            resumed.headers.get("authorization").map(String::as_str),
            Some("Bearer renewed-key")
        );
        for (method, path) in [
            ("GET", "/v2/migration/accounts"),
            ("POST", "/v2/migration/assets/invalid"),
        ] {
            let encoded = serde_json::to_string(&DurableContinuation {
                method: method.to_owned(),
                path: path.to_owned(),
                content_type: "application/json".to_owned(),
                idempotency_key: None,
                body: "7b7d".to_owned(),
            })
            .unwrap_or_else(|error| panic!("continuation encoding: {error}"));
            assert!(resumed_request(&encoded, "Bearer renewed-key").is_err());
        }
    }

    #[test]
    fn history_body_cannot_inject_owner_or_receipt_authority() {
        assert!(direct_body::<MigrationHistoryImportRequestV2>(&request(
            br#"{"chain":"ethereum","ownership_evidence":"AQ==","source_evidence":"Ag=="}"#,
        ))
        .is_ok());
        assert!(direct_body::<MigrationHistoryReadRequestV2>(&request(
            br#"{"cursor":null,"limit":1}"#,
        ))
        .is_ok());
        for body in [
            br#"{"chain":"ethereum","ownership_evidence":"AQ==","source_evidence":"Ag==","principal":"other"}"#.as_slice(),
            br#"{"chain":"ethereum","ownership_evidence":"AQ==","source_evidence":"Ag==","layerx_receipt":true}"#.as_slice(),
        ] {
            assert!(direct_body::<MigrationHistoryImportRequestV2>(&request(body)).is_err());
        }
        for body in [
            br#"{"cursor":null,"limit":1,"principal":"other"}"#.as_slice(),
            br#"{"cursor":null,"limit":1,"layerx_receipt":true}"#.as_slice(),
            br#"{"cursor":null,"limit":-1}"#.as_slice(),
        ] {
            assert!(direct_body::<MigrationHistoryReadRequestV2>(&request(body)).is_err());
        }
        for malformed in ["", "AQ", "AR==", "AQ==\n", "AQ-_", "!!!!"] {
            assert!(history_evidence(malformed).is_err());
        }
    }

    #[test]
    fn history_pending_continuations_retain_exact_route_and_reauthentication() {
        for path in ["/v2/migration/history", "/v2/migration/history/read"] {
            let encoded = serde_json::to_string(&DurableContinuation {
                method: "POST".to_owned(),
                path: path.to_owned(),
                content_type: "application/json".to_owned(),
                idempotency_key: Some("history-key".to_owned()),
                body: "7b7d".to_owned(),
            })
            .unwrap_or_else(|error| panic!("history continuation: {error}"));
            let resumed = resumed_request(&encoded, "Bearer renewed-key")
                .unwrap_or_else(|()| panic!("history continuation refused"));
            assert_eq!(resumed.path, path);
            assert_eq!(
                resumed.headers.get("authorization").map(String::as_str),
                Some("Bearer renewed-key")
            );
        }
    }
}

fn x402_supported(config: &Config, transport: IngressTransport) -> Dispatch {
    match Facilitator::new(config.manifest.x402_supported.clone()) {
        Ok(facilitator) => Dispatch::result(
            200,
            "completed",
            json!({ "transport": transport.label(), "supported": facilitator.supported() }),
        ),
        Err(_) => Dispatch::error(503, "pending", "adapter_configuration_invalid"),
    }
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct BuyerRequest {
    payment_required: PaymentRequired,
    scheme_payload: Value,
}

fn x402_buyer(
    config: &Config,
    request: &IncomingRequest,
    transport: IngressTransport,
    trace: &TraceId,
    operation: &str,
) -> Dispatch {
    let body: BuyerRequest = match typed_body(request, transport) {
        Ok(value) => value,
        Err(()) => return Dispatch::error(400, "refused", "invalid_x402_buyer_request"),
    };
    let Ok(TransportValue::HttpHeader { value: header, .. }) =
        encode_payment_required(TransportKind::Http, &body.payment_required)
    else {
        return Dispatch::error(400, "refused", "invalid_x402_offer");
    };
    let supported = config
        .manifest
        .x402_supported
        .kinds
        .iter()
        .map(|kind| SupportedKind {
            scheme: kind.scheme.clone(),
            network: kind.network.clone(),
        })
        .collect();
    let Ok(buyer) = Buyer::new(supported) else {
        return Dispatch::error(503, "pending", "adapter_configuration_invalid");
    };
    let mut plane = BuyerPlane {
        payload: Some(body.scheme_payload),
    };
    let Ok(idempotency) = parse_hex32(operation) else {
        return Dispatch::error(503, "pending", "operation_identity_invalid");
    };
    match buyer.build_payment(&header, idempotency, &mut plane, trace) {
        Ok(built) => Dispatch::result(
            200,
            "completed",
            json!({
                "transport": transport.label(),
                "payment_header": built.header,
                "payment_payload": built.payload,
                "idempotency_key": hex(&built.idempotency_key)
            }),
        ),
        Err(_) => Dispatch::error(400, "refused", "x402_buyer_refused"),
    }
}

struct BuyerPlane {
    payload: Option<Value>,
}

impl BuyerPaymentPlane for BuyerPlane {
    fn construct(
        &mut self,
        _request: PaymentBuildRequest,
    ) -> Result<Value, layerx_x402::model::X402Error> {
        self.payload
            .take()
            .filter(Value::is_object)
            .ok_or(layerx_x402::model::X402Error::InvalidPayload)
    }
}

fn x402_seller(request: &IncomingRequest, transport: IngressTransport) -> Dispatch {
    let required: PaymentRequired = match typed_body(request, transport) {
        Ok(value) => value,
        Err(()) => return Dispatch::error(400, "refused", "invalid_x402_offer"),
    };
    let Ok(seller) = Seller::new(required) else {
        return Dispatch::error(400, "refused", "invalid_x402_offer");
    };
    match seller.payment_required() {
        Ok(signal) => Dispatch::result(
            200,
            "completed",
            json!({
                "transport": transport.label(),
                "status": signal.status,
                "payment_required_header": signal.header,
                "payment_required": signal.body
            }),
        ),
        Err(_) => Dispatch::error(400, "refused", "x402_offer_encoding_refused"),
    }
}

fn x402_request(
    request: &IncomingRequest,
    transport: IngressTransport,
) -> Result<FacilitatorRequest, ()> {
    let value: Value = typed_body(request, transport)?;
    decode_facilitator_request(transport_kind(transport), &TransportValue::Json(value))
        .map_err(|_| ())
}

fn gateway_core(config: &Config, trace: &TraceId, now: u64) -> Result<GatewayCore, ()> {
    let mut gateway = GatewayCore::new();
    for registered in config.manifest.adapters.values() {
        gateway
            .register_adapter(registered.descriptor.clone(), trace, now)
            .map_err(|_| ())?;
    }
    Ok(gateway)
}

struct X402VerifyPlane<'a> {
    observed_at: u64,
    receipt_keys: &'a [SequencerAuthorization],
}

fn verified_x402_transfer(
    request: &FacilitatorPaymentRequest,
    observed_at: u64,
) -> Result<(PayExactCall, [u8; 32]), &'static str> {
    request
        .payment_payload
        .payload
        .get("layerxTransaction")
        .and_then(Value::as_str)
        .ok_or("typed_intent_required")?;
    let declared_idempotency = request
        .payment_payload
        .payload
        .get("layerxIdempotencyKey")
        .and_then(Value::as_str)
        .and_then(|value| parse_hex32(value).ok())
        .ok_or("protocol_idempotency_required")?;
    verified_x402_transfer_reference(request, observed_at, declared_idempotency)
}

fn verified_x402_transfer_reference(
    request: &FacilitatorPaymentRequest,
    observed_at: u64,
    expected_reference: [u8; 32],
) -> Result<(PayExactCall, [u8; 32]), &'static str> {
    let transaction = request
        .payment_payload
        .payload
        .get("layerxTransaction")
        .and_then(Value::as_str)
        .ok_or("typed_intent_required")?;
    let transaction = transaction.strip_prefix("0x").unwrap_or(transaction);
    let transaction = decode_hex(transaction, MAX_BODY).map_err(|_| "transaction_malformed")?;
    let (transfer, _, transaction_hash) =
        decode_pay_exact(&transaction).map_err(|error| error.code())?;
    if transfer.reference != expected_reference {
        return Err("protocol_idempotency_mismatch");
    }
    let (asset, recipient) = x402_facts(&request.payment_requirements)?;
    if transfer.asset != asset
        || transfer.to != recipient
        || transfer.amount != request.payment_requirements.amount.value()
    {
        return Err("payment_requirements_mismatch");
    }
    if observed_at > transfer.expires_at {
        return Err("activity_time_window_refused");
    }
    Ok((transfer, transaction_hash))
}

fn verified_x402_activity(
    request: &FacilitatorPaymentRequest,
    modules: &ModuleRegistry,
    protocol_version: u16,
    protocol_network_id: u32,
    expected_signer: &[u8; 32],
    observed_at: u64,
) -> Result<(VerifiedSubmission, VerifiedTransfer), &'static str> {
    let activity = request
        .payment_payload
        .payload
        .get("layerxActivity")
        .and_then(Value::as_str)
        .and_then(|value| decode_hex(value, MAX_BODY).ok())
        .filter(|value| !value.is_empty())
        .ok_or("typed_intent_required")?;
    let declared_idempotency = request
        .payment_payload
        .payload
        .get("layerxIdempotencyKey")
        .and_then(Value::as_str)
        .and_then(|value| parse_hex32(value).ok())
        .ok_or("protocol_idempotency_required")?;
    let submission = verify_submission(
        &activity,
        modules,
        protocol_version,
        protocol_network_id,
        expected_signer,
    )
    .map_err(|_| "activity_authorization_refused")?;
    if submission.idempotency_key() != declared_idempotency {
        return Err("protocol_idempotency_mismatch");
    }
    let transfer = submission.transfer().ok_or("typed_transfer_required")?;
    let (asset, recipient) = x402_facts(&request.payment_requirements)?;
    if transfer.asset() != asset
        || transfer.recipient() != recipient
        || transfer.amount() != request.payment_requirements.amount.value()
    {
        return Err("payment_requirements_mismatch");
    }
    if observed_at < transfer.not_before()
        || observed_at > transfer.not_after()
        || observed_at > transfer.expires_at()
    {
        return Err("activity_time_window_refused");
    }
    Ok((submission, transfer))
}

fn x402_facts(requirements: &PaymentRequirements) -> Result<([u8; 32], [u8; 32]), &'static str> {
    if requirements.is_layerx_network() {
        return requirements.layerx_facts().map_err(|_| "unsupported_offer");
    }
    if requirements.network != "eip155:125" {
        return Err("unsupported_offer");
    }
    let identifier = |value: &str| {
        parse_hex32(value.strip_prefix("0x").unwrap_or(value)).map_err(|_| "unsupported_offer")
    };
    Ok((
        identifier(&requirements.asset)?,
        identifier(&requirements.pay_to)?,
    ))
}

impl FacilitatorPlane for X402VerifyPlane<'_> {
    fn receipt_keys(&self) -> &[SequencerAuthorization] {
        self.receipt_keys
    }

    fn verify(
        &mut self,
        request: &FacilitatorPaymentRequest,
        _trace: &TraceId,
    ) -> Result<PlaneVerifyOutcome, X402Error> {
        match verified_x402_transfer(request, self.observed_at) {
            Ok((transfer, transaction_hash)) => Ok(PlaneVerifyOutcome::Valid {
                payer: Some(hex(&transfer.from)),
                extra: Some(json!({ "layerxTransactionHash": hex(&transaction_hash) })),
            }),
            Err(reason) => Ok(PlaneVerifyOutcome::Invalid {
                reason,
                payer: None,
            }),
        }
    }

    fn settle(
        &mut self,
        _request: FacilitatorPaymentRequest,
        _trace: &TraceId,
    ) -> Result<FacilitatorSettlementOutcome, X402Error> {
        Err(X402Error::PaymentRefused)
    }
}

fn x402_verify(
    config: &Config,
    request: &IncomingRequest,
    transport: IngressTransport,
    record: &KeyRecord,
    principal: &PrincipalId,
    trace: &TraceId,
) -> Dispatch {
    let Ok(parsed) = x402_request(request, transport) else {
        return Dispatch::error(400, "refused", "invalid_x402_request");
    };
    let Ok(facilitator) = Facilitator::new(config.manifest.x402_supported.clone()) else {
        return Dispatch::error(503, "pending", "adapter_configuration_invalid");
    };
    if parse_hex32(&record.signer_public_key).is_err() {
        return Dispatch::error(503, "pending", "authenticated_signer_invalid");
    }
    let Ok(server_now) = now() else {
        return Dispatch::error(503, "pending", "clock_unavailable");
    };
    let Ok(mut gateway) = gateway_core(config, trace, server_now) else {
        return Dispatch::error(503, "pending", "adapter_configuration_invalid");
    };
    let mut plane = X402VerifyPlane {
        observed_at: server_now,
        receipt_keys: &config.receipt_keys,
    };
    match facilitator.verify(
        &mut gateway,
        principal,
        &parsed,
        &mut plane,
        trace,
        server_now,
    ) {
        Ok(response) => Dispatch::result(200, "completed", json!(response)),
        Err(traced) => match traced.into_error() {
            X402Error::UnsupportedOffer => {
                Dispatch::error(400, "refused", "unsupported_x402_offer")
            }
            X402Error::Gateway(_) => Dispatch::error(503, "pending", "settlement_unavailable"),
            _ => Dispatch::error(400, "refused", "invalid_x402_request"),
        },
    }
}

struct X402SettlePlane {
    relay: ChainRelay,
    receipt_hex: Option<String>,
    activity_id: Option<String>,
    unavailable: bool,
    observed_at: u64,
}

fn chain_relay(config: &Config) -> Result<ChainRelay, String> {
    use std::io::Read as _;
    let read = |name: &str, maximum: usize| -> Result<Zeroizing<Vec<u8>>, String> {
        let path = std::env::var(name).map_err(|_| format!("{name} is required"))?;
        let file = std::fs::File::open(path).map_err(|_| format!("{name} unavailable"))?;
        let mut bytes = Zeroizing::new(Vec::new());
        file.take((maximum + 1) as u64)
            .read_to_end(&mut bytes)
            .map_err(|_| format!("{name} unavailable"))?;
        if bytes.is_empty() || bytes.len() > maximum {
            return Err(format!("{name} exceeds its bound"));
        }
        Ok(bytes)
    };
    let secret = |name: &str| -> Result<Zeroizing<String>, String> {
        let bytes = read(name, 4096)?;
        let text = std::str::from_utf8(&bytes)
            .map_err(|_| format!("{name} is invalid"))?
            .trim_end_matches(['\r', '\n']);
        if text.is_empty() {
            return Err(format!("{name} is empty"));
        }
        Ok(Zeroizing::new(text.to_owned()))
    };
    let ca_der = read("LAYERX_INTEROP_OUTBOUND_CA_DER", 64 * 1024)?;
    let ca = native_tls::Certificate::from_der(&ca_der).map_err(|error| error.to_string())?;
    let endpoint = layerx_client::sequencer::SequencerEndpoint::new(
        &std::env::var("LAYERX_INTEROP_SEQUENCER_URL")
            .map_err(|_| "sequencer endpoint unavailable".to_owned())?,
        &[ca_der.as_slice()],
    )
    .map_err(|error| format!("sequencer endpoint: {error:?}"))?;
    let journal = RelayJournal::new(
        &std::env::var("LAYERX_INTEROP_REDIS_URL")
            .map_err(|_| "relay journal endpoint unavailable".to_owned())?,
        ca,
        secret("LAYERX_INTEROP_REDIS_USERNAME_FILE")?,
        secret("LAYERX_INTEROP_REDIS_PASSWORD_FILE")?,
    )
    .map_err(|error| error.to_string())?;
    Ok(ChainRelay {
        endpoint: std::sync::Arc::new(endpoint),
        receipt_keys: config.receipt_keys.clone(),
        journal,
    })
}

impl X402SettlePlane {
    fn new(config: &Config, observed_at: u64) -> Result<Self, String> {
        Ok(Self {
            relay: chain_relay(config)?,
            receipt_hex: None,
            activity_id: None,
            unavailable: false,
            observed_at,
        })
    }
}

impl FacilitatorPlane for X402SettlePlane {
    fn receipt_keys(&self) -> &[SequencerAuthorization] {
        &self.relay.receipt_keys
    }

    fn verify(
        &mut self,
        _request: &FacilitatorPaymentRequest,
        _trace: &TraceId,
    ) -> Result<PlaneVerifyOutcome, X402Error> {
        Err(X402Error::PaymentRefused)
    }

    fn settle(
        &mut self,
        request: FacilitatorPaymentRequest,
        _trace: &TraceId,
    ) -> Result<FacilitatorSettlementOutcome, X402Error> {
        match verified_x402_transfer_reference(&request, self.observed_at, request.idempotency_key)
        {
            Ok(_) => (),
            Err(reason) => {
                return Ok(FacilitatorSettlementOutcome::Refused {
                    reason,
                    payer: None,
                });
            }
        };
        let raw = request
            .payment_payload
            .payload
            .get("layerxTransaction")
            .and_then(Value::as_str)
            .ok_or(X402Error::InvalidPayload)?;
        let raw = decode_hex(raw.strip_prefix("0x").unwrap_or(raw), MAX_BODY)
            .map_err(|_| X402Error::InvalidPayload)?;
        let (_, sender, _) = decode_pay_exact(&raw).map_err(|_| X402Error::PaymentRefused)?;
        match self
            .relay
            .relay(&hex(&request.idempotency_key), &raw, sender)
        {
            Ok(RelayOutcome::Pending { tx_hash }) => Ok(FacilitatorSettlementOutcome::Pending {
                transaction: format!("0x{}", hex(&tx_hash)),
            }),
            Ok(RelayOutcome::Executed(evidence)) => {
                self.receipt_hex = Some(hex(&evidence.signed_receipt));
                self.activity_id = Some(hex(&evidence.transaction_hash));
                Ok(FacilitatorSettlementOutcome::Executed(evidence))
            }
            Err(RelayError::Transaction(error)) => Ok(FacilitatorSettlementOutcome::Refused {
                reason: error.code(),
                payer: None,
            }),
            Err(RelayError::TransactionReplacementRefused) => {
                Ok(FacilitatorSettlementOutcome::Refused {
                    reason: "transaction_replacement_refused",
                    payer: None,
                })
            }
            Err(_) => {
                self.unavailable = true;
                Err(X402Error::PaymentRefused)
            }
        }
    }
}

fn x402_settle(
    config: &Config,
    request: &IncomingRequest,
    transport: IngressTransport,
    _record: &KeyRecord,
    principal: &PrincipalId,
    trace: &TraceId,
) -> Dispatch {
    let Ok(parsed) = x402_request(request, transport) else {
        return Dispatch::error(400, "refused", "invalid_x402_request");
    };
    let stable_identity = parsed
        .payment_payload
        .payload
        .get("layerxIdempotencyKey")
        .and_then(Value::as_str)
        .and_then(|value| parse_hex32(value).ok());
    let Some(stable_identity) = stable_identity else {
        return Dispatch::error(400, "refused", "protocol_idempotency_required");
    };
    if !request.headers.contains_key("authorization") {
        return Dispatch::error(401, "refused", "api_key_required");
    }
    let Ok(facilitator) = Facilitator::new(config.manifest.x402_supported.clone()) else {
        return Dispatch::error(503, "pending", "adapter_configuration_invalid");
    };
    let Ok(server_now) = now() else {
        return Dispatch::error(503, "pending", "clock_unavailable");
    };
    let Ok(mut gateway) = gateway_core(config, trace, server_now) else {
        return Dispatch::error(503, "pending", "adapter_configuration_invalid");
    };
    let Ok(mut plane) = X402SettlePlane::new(config, server_now) else {
        return Dispatch::error(503, "pending", "settlement_unavailable");
    };
    let settled = facilitator.settle(
        &mut gateway,
        principal,
        &parsed,
        stable_identity,
        SettlementStep::Single,
        &mut plane,
        trace,
        server_now,
    );
    match settled {
        Ok(response) => {
            if response.success {
                let mut result = Dispatch::result(200, "completed", json!(response));
                result.receipt_hex = plane.receipt_hex;
                result.activity_id = plane.activity_id;
                result
            } else if response.error_reason.as_deref() == Some("settlement_pending") {
                Dispatch::result(202, "pending", json!(response))
            } else {
                Dispatch::result(200, "refused", json!(response))
            }
        }
        Err(traced) => {
            if plane.unavailable {
                return Dispatch::error(503, "pending", "settlement_unavailable");
            }
            match traced.into_error() {
                X402Error::UnsupportedOffer => {
                    Dispatch::error(400, "refused", "unsupported_x402_offer")
                }
                X402Error::EvidenceMismatch | X402Error::EvidenceMissing => {
                    Dispatch::error(503, "pending", "receipt_intent_mismatch")
                }
                X402Error::Gateway(_) => Dispatch::error(503, "pending", "settlement_unavailable"),
                _ => Dispatch::error(400, "refused", "invalid_x402_request"),
            }
        }
    }
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct Ap2Request {
    checkout_presentation: String,
    payment_presentation: String,
    nonce: String,
    #[serde(default)]
    activity: String,
}

struct DispatchContext<'a> {
    config: &'a Config,
    principal: &'a PrincipalId,
    trace: &'a TraceId,
}

fn ap2(
    config: &Config,
    request: &IncomingRequest,
    record: &KeyRecord,
    principal: &PrincipalId,
    trace: &TraceId,
    _operation: &str,
    execute: bool,
) -> Dispatch {
    let body: Ap2Request = match direct_body(request) {
        Ok(value) => value,
        Err(()) => return Dispatch::error(400, "refused", "invalid_ap2_request"),
    };
    let resolver = Ap2Resolver {
        keys: &config.manifest.ap2_keys,
    };
    if parse_hex32(&record.principal_digest).is_err()
        || record.principal_digest != record.principal_digest.to_ascii_lowercase()
    {
        return Dispatch::error(503, "pending", "authenticated_principal_invalid");
    }
    let Ok(server_now) = now() else {
        return Dispatch::error(503, "pending", "clock_unavailable");
    };
    let mut matched = None;
    for binding in config
        .manifest
        .ap2_assets
        .iter()
        .filter(|binding| binding.principal_digest == record.principal_digest)
    {
        let context = VerificationContext {
            now: server_now,
            clock_skew_seconds: 0,
            expected_audience: &binding.audience,
            expected_nonce: &body.nonce,
            currency_minor_exponent: binding.minor_unit_exponent,
            usage: None,
        };
        let Ok(verified) = MandateVerifier::new(&resolver).verify(
            &body.checkout_presentation,
            &body.payment_presentation,
            &context,
        ) else {
            continue;
        };
        if verified.amount().currency() != binding.currency.as_str() {
            continue;
        }
        if matched.replace((binding, verified)).is_some() {
            return Dispatch::error(400, "refused", "asset_binding_ambiguous");
        }
    }
    let Some((binding, verified)) = matched else {
        return Dispatch::error(400, "refused", "mandate_verification_refused");
    };
    if !execute {
        let mode = match verified.mode() {
            MandateMode::Direct => "direct",
            MandateMode::Autonomous => "autonomous",
        };
        return Dispatch::result(
            200,
            "completed",
            json!({
                "state": "mandate-verified",
                "mode": mode,
                "transaction_id": verified.transaction_id(),
                "checkout_id": verified.checkout_id(),
                "currency": verified.amount().currency(),
                "minor_units": verified.amount().minor_units().to_string(),
                "execution_at": verified.execution_at()
            }),
        );
    }
    ap2_authorize_and_execute(
        &DispatchContext {
            config,
            principal,
            trace,
        },
        &body,
        binding,
        &verified,
        server_now,
    )
}

fn ap2_authorize_and_execute(
    context: &DispatchContext<'_>,
    body: &Ap2Request,
    binding: &crate::config::Ap2AssetBinding,
    verified: &layerx_ap2::VerifiedMandates,
    server_now: u64,
) -> Dispatch {
    let DispatchContext {
        config,
        principal,
        trace,
        ..
    } = *context;
    if verified.amount().currency().len() != 3 || body.activity.is_empty() {
        return Dispatch::error(400, "refused", "typed_intent_required");
    }
    let Ok(activity) = decode_hex(&body.activity, MAX_BODY) else {
        return Dispatch::error(400, "refused", "typed_intent_required");
    };
    let Ok(asset) = parse_hex32(&binding.asset) else {
        return Dispatch::error(400, "refused", "asset_binding_invalid");
    };
    let Ok(payer) = parse_hex32(&binding.payer_account) else {
        return Dispatch::error(400, "refused", "account_binding_invalid");
    };
    let Ok(recipient) = parse_hex32(&binding.payee_account) else {
        return Dispatch::error(400, "refused", "account_binding_invalid");
    };
    let Ok(atomic_units) = binding.atomic_units_per_minor_unit.parse::<u128>() else {
        return Dispatch::error(503, "pending", "adapter_configuration_invalid");
    };
    let Ok(payee_merchant) = Merchant::new(
        binding.payee_merchant_id.clone(),
        binding.payee_merchant_name.clone(),
        binding.payee_merchant_website.clone(),
    ) else {
        return Dispatch::error(503, "pending", "adapter_configuration_invalid");
    };
    let layerx_binding = LayerXAssetBinding {
        currency: binding.currency.clone(),
        minor_unit_exponent: binding.minor_unit_exponent,
        atomic_units_per_minor_unit: atomic_units,
        asset,
        payer_receipt_account: payer,
        payee_receipt_account: recipient,
        payee_merchant,
    };
    let context = VerificationContext {
        now: server_now,
        clock_skew_seconds: 0,
        expected_audience: &binding.audience,
        expected_nonce: &body.nonce,
        currency_minor_exponent: binding.minor_unit_exponent,
        usage: None,
    };
    let payment = match authorize_payment(principal, verified, &context, &layerx_binding) {
        Ok(value) => value,
        Err(Ap2Error::ConstraintViolated(_)) => {
            return Dispatch::error(400, "refused", "mandate_merchant_mismatch")
        }
        Err(_) => return Dispatch::error(400, "refused", "asset_binding_invalid"),
    };
    let Ok(relay) = chain_relay(config) else {
        return Dispatch::error(503, "pending", "settlement_unavailable");
    };
    let Ok(mut gateway) = gateway_core(config, trace, server_now) else {
        return Dispatch::error(503, "pending", "adapter_configuration_invalid");
    };
    ap2_execute(&payment, &activity, &relay, &mut gateway, trace, server_now)
}

fn ap2_execute(
    payment: &layerx_ap2::AuthorizedPayment,
    transaction: &[u8],
    relay: &ChainRelay,
    gateway: &mut GatewayCore,
    trace: &TraceId,
    server_now: u64,
) -> Dispatch {
    let (transfer, sender, _) = match decode_pay_exact(transaction) {
        Ok(value) => value,
        Err(error) => return Dispatch::error(400, "refused", error.code()),
    };
    if transfer.from != payment.payer_receipt_account()
        || transfer.to != payment.payee_receipt_account()
        || transfer.asset != payment.asset()
        || transfer.amount != payment.amount()
        || transfer.reference != payment.idempotency_key()
    {
        return Dispatch::error(400, "refused", "mandate_payment_mismatch");
    }
    if server_now > transfer.expires_at {
        return Dispatch::error(400, "refused", "activity_time_window_refused");
    }
    let Ok(adapter) = AdapterId::new("ap2") else {
        return Dispatch::error(503, "pending", "adapter_configuration_invalid");
    };
    let translation = TranslationRequest::new(
        adapter,
        TranslationKind::StateChanging,
        payment.idempotency_key(),
        payment.request_digest(),
    );
    let Ok(translation) = translation else {
        return Dispatch::error(503, "pending", "adapter_configuration_invalid");
    };
    match gateway.begin_translation(payment.principal(), &translation, trace, server_now) {
        Ok(TranslationStatus::Pending | TranslationStatus::ReceiptVerified { .. }) => (),
        Ok(TranslationStatus::Refused) => {
            return Dispatch::result(
                200,
                "refused",
                json!({"state": ExternalState::Refused.label()}),
            )
        }
        _ => return Dispatch::error(503, "pending", "settlement_unavailable"),
    }
    let executed = match relay.relay(&hex(&payment.idempotency_key()), transaction, sender) {
        Ok(RelayOutcome::Pending { tx_hash }) => return submitted_payment(tx_hash),
        Ok(RelayOutcome::Executed(evidence)) => layerx_ap2::ExecutedPayment {
            evidence,
            order_id: payment.checkout_id().to_owned(),
        },
        Err(RelayError::Transaction(error)) => {
            return Dispatch::error(400, "refused", error.code())
        }
        Err(RelayError::TransactionReplacementRefused) => {
            return Dispatch::error(409, "refused", "transaction_replacement_refused")
        }
        Err(_) => return Dispatch::error(503, "pending", "settlement_unavailable"),
    };
    ap2_executed(
        payment,
        &executed,
        &relay.receipt_keys,
        gateway,
        trace,
        server_now,
    )
}

fn submitted_payment(tx_hash: [u8; 32]) -> Dispatch {
    let mut result = Dispatch::result(
        202,
        "pending",
        json!({"state": ExternalState::Submitted.label(), "transaction": format!("0x{}", hex(&tx_hash))}),
    );
    result.activity_id = Some(hex(&tx_hash));
    result
}

fn ap2_executed(
    payment: &layerx_ap2::AuthorizedPayment,
    executed: &layerx_ap2::ExecutedPayment,
    receipt_keys: &[SequencerAuthorization],
    gateway: &mut GatewayCore,
    trace: &TraceId,
    server_now: u64,
) -> Dispatch {
    let evidence = &executed.evidence;
    if evidence.rung == SettlementRung::Submitted {
        return submitted_payment(evidence.transaction_hash);
    }
    let Ok(receipt) = verify_sequencer_signature(&evidence.signed_receipt, receipt_keys) else {
        return Dispatch::error(503, "pending", "receipt_verification_failed");
    };
    let Some(protocol) = receipt.protocol() else {
        return Dispatch::error(503, "pending", "receipt_verification_failed");
    };
    const LEDGER: [u8; 20] = [
        0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x10, 0x1a,
    ];
    const PAYMENT_EXACT_TOPIC: [u8; 32] = [
        0xc4, 0xda, 0x5d, 0x6f, 0x75, 0x7c, 0x25, 0x1c, 0xd7, 0x09, 0x47, 0xb7, 0x52, 0x91, 0x7e,
        0x56, 0x71, 0xb6, 0x77, 0x51, 0x7e, 0x3d, 0x95, 0x3f, 0x36, 0x2a, 0x1d, 0xf6, 0x64, 0x8e,
        0xf5, 0xb1,
    ];
    let topic = PAYMENT_EXACT_TOPIC;
    let mut payments = evidence
        .logs
        .iter()
        .filter(|log| log.address() == LEDGER && log.topics().first() == Some(&topic));
    let Some(log) = payments.next() else {
        return Dispatch::error(503, "pending", "receipt_intent_mismatch");
    };
    if protocol.result_code() != 0
        || protocol.activity_id() != evidence.transaction_hash
        || payments.next().is_some()
        || log.topics()
            != [
                topic,
                payment.payer_receipt_account(),
                payment.payee_receipt_account(),
                payment.idempotency_key(),
            ]
        || log.data().len() != 64
        || log.data()[..32] != payment.asset()
        || log.data()[32..48] != [0; 16]
        || log.data()[48..] != payment.amount().to_be_bytes()
    {
        return Dispatch::error(503, "pending", "receipt_intent_mismatch");
    }
    let Ok(TranslationStatus::ReceiptVerified { receipt_digest }) = gateway
        .settle_with_sequencer_evidence(
            payment.principal(),
            payment.idempotency_key(),
            evidence,
            receipt_keys,
            trace,
            server_now,
        )
    else {
        return Dispatch::error(503, "pending", "receipt_intent_mismatch");
    };
    let mut result = Dispatch::result(
        200,
        "completed",
        json!({
            "state": ExternalState::ReceiptVerified.label(),
            "transaction_id": payment.transaction_id(),
            "checkout_id": payment.checkout_id(),
            "receipt_digest": hex(&receipt_digest),
            "transaction": format!("0x{}", hex(&evidence.transaction_hash)),
            "verificationLevel": evidence.rung.label(),
        }),
    );
    result.receipt_hex = Some(hex(&evidence.signed_receipt));
    result.activity_id = Some(hex(&evidence.transaction_hash));
    result
}

struct Ap2Resolver<'a> {
    keys: &'a [Ap2KeyPin],
}

impl KeyResolver for Ap2Resolver<'_> {
    fn resolve(
        &self,
        usage: KeyUse,
        header: &ProtectedHeader,
    ) -> Result<P256Key, layerx_ap2::Ap2Error> {
        if header.certificate_chain().is_some() {
            return Err(layerx_ap2::Ap2Error::KeyResolution);
        }
        let use_case = match usage {
            KeyUse::CheckoutMandateIssuer => "checkout-mandate",
            KeyUse::PaymentMandateIssuer => "payment-mandate",
            KeyUse::MerchantCheckout => "merchant-checkout",
        };
        let kid = header.key_id().ok_or(layerx_ap2::Ap2Error::KeyResolution)?;
        let pin = self
            .keys
            .iter()
            .find(|pin| pin.use_case == use_case && pin.key_id == kid)
            .ok_or(layerx_ap2::Ap2Error::KeyResolution)?;
        let bytes = decode_hex(&pin.public_key_sec1, 65)
            .map_err(|_| layerx_ap2::Ap2Error::KeyResolution)?;
        P256Key::from_sec1_bytes(&bytes).map_err(|_| layerx_ap2::Ap2Error::KeyResolution)
    }
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct UcpCapabilityWire {
    name: String,
    version: String,
    spec: String,
    schema: String,
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct UcpHandlerWire {
    id: String,
    version: String,
    spec: String,
    schema: String,
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct UcpRequest {
    checkout_id: String,
    currency: String,
    total_minor: String,
    asset: String,
    recipient: String,
    idempotency_key: String,
    profile_url: String,
    capabilities: Vec<UcpCapabilityWire>,
    payment_handlers: Vec<UcpHandlerWire>,
    activity: String,
    order_id: String,
    permalink_url: String,
}

fn ucp_client_profile(body: &UcpRequest) -> Result<PlatformProfile, ()> {
    if body.capabilities.len() > 32 || body.payment_handlers.len() > 32 {
        return Err(());
    }
    let mut capabilities = Vec::with_capacity(body.capabilities.len());
    for capability in &body.capabilities {
        capabilities.push(
            Capability::new(
                &capability.name,
                &capability.version,
                &capability.spec,
                &capability.schema,
            )
            .map_err(|_| ())?,
        );
    }
    let mut payment_handlers = Vec::with_capacity(body.payment_handlers.len());
    for handler in &body.payment_handlers {
        payment_handlers.push(
            PaymentHandler::new(
                &handler.id,
                &handler.version,
                &handler.spec,
                &handler.schema,
            )
            .map_err(|_| ())?,
        );
    }
    Ok(PlatformProfile {
        profile_url: body.profile_url.clone(),
        capabilities,
        payment_handlers,
    })
}

struct UcpServicePlane {
    relay: ChainRelay,
    transaction: Vec<u8>,
    observed_at: u64,
    order_id: String,
    permalink_url: String,
    receipt_hex: Option<String>,
    activity_id: Option<String>,
    unavailable: bool,
    refusal: Option<&'static str>,
    submitted: Option<[u8; 32]>,
}

impl UcpPaymentPlane for UcpServicePlane {
    fn execute(
        &mut self,
        intent: &UcpPaymentIntent,
        _trace: &TraceId,
    ) -> Result<UcpPlaneResult, UcpError> {
        self.refusal = None;
        self.submitted = None;
        self.unavailable = false;
        let (transfer, sender, _) = match decode_pay_exact(&self.transaction) {
            Ok(value) => value,
            Err(error) => {
                self.refusal = Some(error.code());
                return Ok(UcpPlaneResult::Refused);
            }
        };
        if transfer.to != intent.recipient
            || transfer.asset != intent.asset
            || transfer.amount != intent.amount
            || transfer.reference != intent.idempotency_key
        {
            self.refusal = Some("checkout_payment_mismatch");
            return Ok(UcpPlaneResult::Refused);
        }
        if self.observed_at > transfer.expires_at {
            self.refusal = Some("activity_time_window_refused");
            return Ok(UcpPlaneResult::Refused);
        }
        match self
            .relay
            .relay(&hex(&intent.idempotency_key), &self.transaction, sender)
        {
            Ok(RelayOutcome::Pending { tx_hash }) => {
                self.submitted = Some(tx_hash);
                Ok(UcpPlaneResult::Pending)
            }
            Ok(RelayOutcome::Executed(evidence)) => {
                self.receipt_hex = Some(hex(&evidence.signed_receipt));
                self.activity_id = Some(hex(&evidence.transaction_hash));
                Ok(UcpPlaneResult::Executed(Box::new(ExecutedUcpPayment {
                    metadata: OrderMetadata {
                        order_id: self.order_id.clone(),
                        permalink_url: self.permalink_url.clone(),
                    },
                    evidence,
                })))
            }
            Err(RelayError::Transaction(error)) => {
                self.refusal = Some(error.code());
                Ok(UcpPlaneResult::Refused)
            }
            Err(RelayError::TransactionReplacementRefused) => {
                self.refusal = Some("transaction_replacement_refused");
                Ok(UcpPlaneResult::Refused)
            }
            Err(_) => {
                self.unavailable = true;
                Err(UcpError::PlaneRefused)
            }
        }
    }
}

fn ucp(
    config: &Config,
    request: &IncomingRequest,
    _record: &KeyRecord,
    principal: &PrincipalId,
    trace: &TraceId,
    _operation: &str,
) -> Dispatch {
    let body: UcpRequest = match direct_body(request) {
        Ok(value) => value,
        Err(()) => return Dispatch::error(400, "refused", "invalid_ucp_request"),
    };
    let Ok(client_platform) = ucp_client_profile(&body) else {
        return Dispatch::error(400, "refused", "ucp_profile_refused");
    };
    let Ok(negotiated) =
        NegotiatedCapabilities::negotiate(&client_platform, &config.manifest.ucp_payment_handler)
    else {
        return Dispatch::error(400, "refused", "ucp_capability_refused");
    };
    let currency: [u8; 3] = match body.currency.as_bytes().try_into() {
        Ok(value) => value,
        Err(_) => return Dispatch::error(400, "refused", "ucp_currency_invalid"),
    };
    let submission = CheckoutSubmission {
        checkout_id: body.checkout_id.clone(),
        currency,
        total_minor: match body.total_minor.parse::<u128>() {
            Ok(value) => value,
            Err(_) => return Dispatch::error(400, "refused", "ucp_amount_invalid"),
        },
        layerx_asset: match parse_hex32(&body.asset) {
            Ok(value) => value,
            Err(_) => return Dispatch::error(400, "refused", "ucp_asset_invalid"),
        },
        layerx_recipient: match parse_hex32(&body.recipient) {
            Ok(value) => value,
            Err(_) => return Dispatch::error(400, "refused", "ucp_recipient_invalid"),
        },
        idempotency_key: match UcpIdempotencyKey::parse(&body.idempotency_key) {
            Ok(value) => value,
            Err(_) => return Dispatch::error(400, "refused", "ucp_idempotency_invalid"),
        },
        negotiated,
    };
    if body.activity.is_empty() {
        return Dispatch::error(400, "refused", "typed_intent_required");
    }
    let Ok(activity) = decode_hex(&body.activity, MAX_BODY) else {
        return Dispatch::error(400, "refused", "typed_intent_required");
    };
    let Ok(server_now) = now() else {
        return Dispatch::error(503, "pending", "clock_unavailable");
    };
    let Ok(mut gateway) = gateway_core(config, trace, server_now) else {
        return Dispatch::error(503, "pending", "adapter_configuration_invalid");
    };
    let Ok(relay) = chain_relay(config) else {
        return Dispatch::error(503, "pending", "settlement_unavailable");
    };
    let mut plane = UcpServicePlane {
        relay,
        transaction: activity,
        observed_at: server_now,
        order_id: body.order_id,
        permalink_url: body.permalink_url,
        receipt_hex: None,
        activity_id: None,
        unavailable: false,
        refusal: None,
        submitted: None,
    };
    let completed = UcpAdapter::complete_checkout(
        &mut gateway,
        principal,
        &submission,
        &mut plane,
        &config.receipt_keys,
        trace,
        server_now,
    );
    ucp_outcome(completed, &plane)
}

fn ucp_outcome(
    completed: Result<layerx_ucp::CheckoutOutcome, layerx_interop_gateway::trace::Traced<UcpError>>,
    plane: &UcpServicePlane,
) -> Dispatch {
    if let Some(reason) = plane.refusal {
        let status = if reason == "transaction_replacement_refused" {
            409
        } else {
            400
        };
        return Dispatch::error(status, "refused", reason);
    }
    match completed {
        Ok(outcome) => match outcome.status {
            CheckoutStatus::Completed => {
                let Some(order) = outcome.order else {
                    return Dispatch::error(503, "pending", "receipt_intent_mismatch");
                };
                let mut result = Dispatch::result(
                    200,
                    "completed",
                    json!({
                        "state": ExternalState::ReceiptVerified.label(),
                        "order": {
                            "id": order.id, "checkout_id": order.checkout_id,
                            "permalink_url": order.permalink_url,
                            "currency": String::from_utf8_lossy(&order.currency),
                            "total_minor": order.total_minor.to_string(),
                            "receipt_digest": hex(&order.receipt_digest)
                        }
                    }),
                );
                result.receipt_hex = plane.receipt_hex.clone();
                result.activity_id = plane.activity_id.clone();
                result
            }
            CheckoutStatus::CompleteInProgress => match plane.submitted {
                Some(tx_hash) => submitted_payment(tx_hash),
                None => Dispatch::error(503, "pending", "settlement_unavailable"),
            },
            _ => Dispatch::result(
                200,
                "refused",
                json!({ "state": ExternalState::Refused.label() }),
            ),
        },
        Err(traced) => {
            if plane.unavailable {
                return Dispatch::error(503, "pending", "settlement_unavailable");
            }
            match traced.into_error() {
                UcpError::InvalidOrder => Dispatch::error(400, "refused", "ucp_order_invalid"),
                UcpError::ReceiptMismatch
                | UcpError::ReceiptRequired
                | UcpError::OrderEvidenceRequired => {
                    Dispatch::error(503, "pending", "receipt_intent_mismatch")
                }
                UcpError::Gateway(_) => Dispatch::error(503, "pending", "settlement_unavailable"),
                _ => Dispatch::error(400, "refused", "invalid_ucp_request"),
            }
        }
    }
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct VisaRequest {
    authority: String,
    path: String,
    signature_input: String,
    signature: String,
    #[serde(default)]
    activity: String,
}

struct VisaActivityBinding {
    canonical: Vec<u8>,
    activity_id: [u8; 32],
    transfer: PayExactCall,
    sender: [u8; 20],
}

fn visa(
    config: &Config,
    request: &IncomingRequest,
    record: &KeyRecord,
    principal: &PrincipalId,
    trace: &TraceId,
    operation: &str,
    execute: bool,
) -> Dispatch {
    let Ok(body) = direct_body::<VisaRequest>(request) else {
        return Dispatch::error(400, "refused", "invalid_visa_tap_request");
    };
    let Ok(tap) = TapRequest::parse(
        body.authority,
        body.path,
        &body.signature_input,
        &body.signature,
    ) else {
        return Dispatch::error(400, "refused", "visa_tap_refused");
    };
    let Some(target) = config
        .manifest
        .visa_targets
        .iter()
        .find(|target| target.principal_digest.as_str() == record.principal_digest)
    else {
        return Dispatch::error(400, "refused", "visa_tap_target_unavailable");
    };
    if require_visa_target(&tap, target).is_err() {
        return Dispatch::error(400, "refused", "visa_tap_target_mismatch");
    }
    let registry = VisaRegistry {
        pins: &config.manifest.visa_agents,
    };
    let Ok(observed_at) = now() else {
        return Dispatch::error(503, "pending", "server_clock_unavailable");
    };
    let Ok(verified) =
        TapVerifier::verify_credential(&tap, &registry, observed_at, config.tap_clock_skew_seconds)
    else {
        return Dispatch::error(400, "refused", "visa_tap_refused");
    };
    let Some(layerx_agent) = verified.layerx_agent else {
        return Dispatch::error(400, "refused", "layerx_agent_binding_required");
    };
    let Ok(signer_public_key) = parse_hex32(&record.signer_public_key) else {
        return Dispatch::error(503, "pending", "persistence_unavailable");
    };
    if let Err(code) = require_visa_actor(layerx_agent, signer_public_key) {
        return Dispatch::error(400, "refused", code);
    }
    if let Err(code) = require_visa_route(execute, verified.intent, &body.activity) {
        return Dispatch::error(400, "refused", code);
    }
    let activity_binding = match visa_activity(&body.activity, execute) {
        Ok(binding) => binding,
        Err(refusal) => return refusal,
    };
    if !execute {
        let Ok(binding) = layerx_visa_tap::verified_agent_binding(layerx_agent, &verified) else {
            return Dispatch::error(400, "refused", "visa_tap_refused");
        };
        return Dispatch::result(
            200,
            "completed",
            json!({
                "state": "credential-verified", "agent_id": binding.trusted_agent_id,
                "layerx_agent": hex(&binding.layerx_agent),
                "credential_evidence": hex(&binding.evidence_digest)
            }),
        );
    }
    let replay_until = match verified
        .expires_at
        .checked_add(config.tap_clock_skew_seconds)
    {
        Some(value) if value > observed_at => value,
        _ => return Dispatch::error(400, "refused", "visa_tap_refused"),
    };
    let tap_audit = gateway_audit_event(
        &record.principal_digest,
        "visa_tap_nonce",
        &verified.key_id,
        "attempted",
        observed_at,
    );
    let mut bindings = DurableVisaBinding {
        store: &config.store,
        nonce: tap.nonce(),
        intent: verified.intent,
        activity_id: activity_binding.as_ref().map(|binding| binding.activity_id),
        signer_public_key,
        target_authority: tap.authority(),
        target_path: tap.path(),
        operation_identity: operation,
        credential_expires_at: verified.expires_at,
        replay_until,
        consumed_at: observed_at,
        audit_event: &tap_audit,
    };
    match prepare_trusted_intent(principal, layerx_agent, &verified, &mut bindings, trace) {
        Ok(_) => {}
        Err(TapError::Replay) => return Dispatch::error(409, "refused", "visa_tap_replayed"),
        Err(TapError::StorageRefused) => {
            return Dispatch::error(503, "pending", "persistence_unavailable")
        }
        Err(_) => return Dispatch::error(400, "refused", "visa_tap_refused"),
    }
    let Ok(relay) = chain_relay(config) else {
        return Dispatch::error(503, "pending", "settlement_unavailable");
    };
    visa_execute(&relay, activity_binding, observed_at)
}

fn visa_execute(
    relay: &ChainRelay,
    activity_binding: Option<VisaActivityBinding>,
    observed_at: u64,
) -> Dispatch {
    let Some(activity) = activity_binding else {
        return Dispatch::error(400, "refused", "typed_intent_required");
    };
    if observed_at > activity.transfer.expires_at {
        return Dispatch::error(400, "refused", "activity_time_window_refused");
    }
    let evidence = match relay.relay(
        &hex(&activity.transfer.reference),
        &activity.canonical,
        activity.sender,
    ) {
        Ok(RelayOutcome::Pending { tx_hash }) => return submitted_payment(tx_hash),
        Ok(RelayOutcome::Executed(evidence)) => evidence,
        Err(RelayError::Transaction(error)) => {
            return Dispatch::error(400, "refused", error.code());
        }
        Err(RelayError::TransactionReplacementRefused) => {
            return Dispatch::error(409, "refused", "transaction_replacement_refused");
        }
        Err(_) => return Dispatch::error(503, "pending", "settlement_unavailable"),
    };
    if evidence.transaction_hash != activity.activity_id {
        return Dispatch::error(503, "pending", "receipt_intent_mismatch");
    }
    match MerchantOperationResult::from_receipt(
        &evidence,
        &relay.receipt_keys,
        activity.transfer.reference,
    ) {
        Ok(MerchantOperationResult::ReceiptVerified {
            receipt_digest,
            from,
            to,
            asset,
            amount,
        }) if from == activity.transfer.from
            && to == activity.transfer.to
            && asset == activity.transfer.asset
            && amount == activity.transfer.amount =>
        {
            let mut result = Dispatch::result(
                200,
                "completed",
                json!({ "state": ExternalState::ReceiptVerified.label(), "receipt_digest": hex(&receipt_digest) }),
            );
            result.receipt_hex = Some(hex(&evidence.signed_receipt));
            result.activity_id = Some(hex(&evidence.transaction_hash));
            result
        }
        _ => Dispatch::error(503, "pending", "receipt_intent_mismatch"),
    }
}

fn visa_activity(activity: &str, execute: bool) -> Result<Option<VisaActivityBinding>, Dispatch> {
    Ok(if execute {
        let canonical = match decode_hex(activity, MAX_BODY) {
            Ok(value) if !value.is_empty() => value,
            _ => return Err(Dispatch::error(400, "refused", "typed_intent_required")),
        };
        let (transfer, sender, activity_id) = match decode_pay_exact(&canonical) {
            Ok(value) => value,
            Err(error) => return Err(Dispatch::error(400, "refused", error.code())),
        };
        Some(VisaActivityBinding {
            canonical,
            activity_id,
            transfer,
            sender,
        })
    } else {
        None
    })
}

fn require_visa_actor(
    layerx_agent: [u8; 32],
    signer_public_key: [u8; 32],
) -> Result<(), &'static str> {
    if layerx_agent == signer_public_key {
        Ok(())
    } else {
        Err("layerx_agent_signer_mismatch")
    }
}

fn require_visa_target(tap: &TapRequest, target: &VisaTargetPin) -> Result<(), &'static str> {
    if tap.authority() == target.authority.as_str() && tap.path() == target.path.as_str() {
        Ok(())
    } else {
        Err("visa_tap_target_mismatch")
    }
}

fn require_visa_route(
    execute: bool,
    intent: AgentIntent,
    activity: &str,
) -> Result<(), &'static str> {
    if execute && intent != AgentIntent::Pay {
        return Err("payer_credential_required");
    }
    if execute && activity.is_empty() {
        return Err("typed_intent_required");
    }
    if !execute && !activity.is_empty() {
        return Err("unexpected_activity_decoration");
    }
    Ok(())
}

struct VisaRegistry<'a> {
    pins: &'a [VisaAgentPin],
}

impl TrustedAgentRegistry for VisaRegistry<'_> {
    fn resolve(
        &self,
        key_id: &str,
        now: u64,
    ) -> Result<RegisteredAgentKey, layerx_visa_tap::TapError> {
        let pin = self
            .pins
            .iter()
            .find(|pin| pin.key_id == key_id)
            .ok_or(layerx_visa_tap::TapError::UnknownKey)?;
        let status = match pin.status.as_str() {
            "active" => KeyStatus::Active,
            "revoked" => return Err(layerx_visa_tap::TapError::Revoked),
            _ => return Err(layerx_visa_tap::TapError::RegistryUnavailable),
        };
        if pin.expires_at <= now {
            return Err(layerx_visa_tap::TapError::ExpiredKey);
        }
        let key = match pin.algorithm.as_str() {
            "ed25519" => AgentPublicKey::Ed25519(
                parse_hex32(&pin.public_key)
                    .map_err(|_| layerx_visa_tap::TapError::RegistryUnavailable)?,
            ),
            "rsa-pss-sha256" => AgentPublicKey::RsaPssSha256Pem(
                STANDARD
                    .decode(&pin.public_key)
                    .map_err(|_| layerx_visa_tap::TapError::RegistryUnavailable)?,
            ),
            _ => return Err(layerx_visa_tap::TapError::RegistryUnavailable),
        };
        Ok(RegisteredAgentKey {
            key_id: pin.key_id.clone(),
            agent_id: pin.agent_id.clone(),
            agent_domain: pin.agent_domain.clone(),
            layerx_agent: Some(
                parse_hex32(&pin.layerx_agent)
                    .map_err(|_| layerx_visa_tap::TapError::RegistryUnavailable)?,
            ),
            key,
            status,
            expires_at: pin.expires_at,
        })
    }
}

struct DurableVisaBinding<'a> {
    store: &'a layerx_platform_gateway::store::RedisStore,
    nonce: &'a str,
    intent: AgentIntent,
    activity_id: Option<[u8; 32]>,
    signer_public_key: [u8; 32],
    target_authority: &'a str,
    target_path: &'a str,
    operation_identity: &'a str,
    credential_expires_at: u64,
    replay_until: u64,
    consumed_at: u64,
    audit_event: &'a str,
}

impl CredentialBindingStore for DurableVisaBinding<'_> {
    fn put(
        &mut self,
        principal: &PrincipalId,
        binding: &CredentialBinding,
        _trace: &TraceId,
    ) -> Result<(), TapError> {
        let record = TapCredentialRecord {
            principal_digest: principal.as_str().to_owned(),
            key_id: binding.key_id.clone(),
            layerx_agent: hex(&binding.layerx_agent),
            trusted_agent_id: binding.trusted_agent_id.clone(),
            trusted_agent_domain: binding.trusted_agent_domain.clone(),
            intent: match self.intent {
                AgentIntent::Browse => "browse",
                AgentIntent::Pay => "pay",
            }
            .to_owned(),
            evidence_digest: hex(&binding.evidence_digest),
            activity_id: self.activity_id.map(|value| hex(&value)),
            signer_public_key: hex(&self.signer_public_key),
            target_authority: self.target_authority.to_owned(),
            target_path: self.target_path.to_owned(),
            operation_identity: self.operation_identity.to_owned(),
            credential_expires_at: self.credential_expires_at,
        };
        match self.store.consume_tap_nonce(
            &binding.key_id,
            self.nonce,
            &record,
            self.consumed_at,
            self.replay_until,
            self.audit_event,
        ) {
            Ok(
                TapNonceConsumption::Consumed { .. } | TapNonceConsumption::AlreadyConsumed { .. },
            ) => Ok(()),
            Ok(TapNonceConsumption::Replay) => Err(TapError::Replay),
            Err(_) => Err(TapError::StorageRefused),
        }
    }
}

#[derive(Clone, Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
struct FiatFacts {
    provider: String,
    settlement: String,
    token_reference_sha256: String,
    rail: String,
    class: String,
    amount: String,
    asset: String,
    destination: String,
    observed_at: u64,
    hold_until: Option<u64>,
}

#[derive(Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
struct FiatEvidenceEnvelope {
    facts: FiatFacts,
    signature: String,
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct FiatCallback {
    token_reference: String,
    evidence: FiatEvidenceEnvelope,
    #[serde(default)]
    activity: String,
}

struct FiatServicePlane {
    relay: ChainRelay,
    transaction: Vec<u8>,
    observed_at: u64,
    evidence: Option<SequencerEvidence>,
    submitted: Option<[u8; 32]>,
    refusal: Option<&'static str>,
}

impl FiatPlane for FiatServicePlane {
    fn receipt_keys(&self) -> &[SequencerAuthorization] {
        &self.relay.receipt_keys
    }

    fn execute(
        &mut self,
        intent: FiatIntent,
        _trace: &TraceId,
    ) -> Result<FiatPlaneResult, FiatError> {
        self.evidence = None;
        self.submitted = None;
        self.refusal = None;
        let (transfer, sender, _) = match decode_pay_exact(&self.transaction) {
            Ok(value) => value,
            Err(error) => {
                self.refusal = Some(error.code());
                return Ok(FiatPlaneResult::Open(PlaneFiatOutcome::Refused));
            }
        };
        let account_matches = match intent.kind {
            FiatIntentKind::Credit => transfer.to == intent.destination,
            FiatIntentKind::Reverse | FiatIntentKind::Chargeback => {
                transfer.from == intent.destination
            }
        };
        if transfer.reference != intent.idempotency_key
            || transfer.asset != intent.asset
            || transfer.amount != intent.amount
            || !account_matches
        {
            self.refusal = Some("provider_payment_mismatch");
            return Ok(FiatPlaneResult::Open(PlaneFiatOutcome::Refused));
        }
        if self.observed_at > transfer.expires_at {
            self.refusal = Some("activity_time_window_refused");
            return Ok(FiatPlaneResult::Open(PlaneFiatOutcome::Refused));
        }
        match self
            .relay
            .relay(&hex(&intent.idempotency_key), &self.transaction, sender)
        {
            Ok(RelayOutcome::Pending { tx_hash }) => {
                self.submitted = Some(tx_hash);
                Ok(FiatPlaneResult::Open(PlaneFiatOutcome::Pending))
            }
            Ok(RelayOutcome::Executed(evidence)) => {
                self.evidence = Some(evidence.clone());
                Ok(FiatPlaneResult::Executed(ExecutedFiatOutcome { evidence }))
            }
            Err(RelayError::Transaction(error)) => {
                self.refusal = Some(error.code());
                Ok(FiatPlaneResult::Open(PlaneFiatOutcome::Refused))
            }
            Err(RelayError::TransactionReplacementRefused) => {
                self.refusal = Some("transaction_replacement_refused");
                Ok(FiatPlaneResult::Open(PlaneFiatOutcome::Refused))
            }
            Err(_) => Err(FiatError::ReceiptRequired),
        }
    }
}

fn fiat(
    config: &Config,
    request: &IncomingRequest,
    _record: &KeyRecord,
    principal: &PrincipalId,
    trace: &TraceId,
    _operation: &str,
    adapter: HostedAdapter,
) -> Dispatch {
    let body: FiatCallback = match direct_body(request) {
        Ok(value) => value,
        Err(()) => return Dispatch::error(400, "refused", "invalid_provider_callback"),
    };
    let Ok(token) = TokenReference::new(body.token_reference.into_bytes()) else {
        return Dispatch::error(400, "refused", "provider_callback_refused");
    };
    let Ok(evidence_bytes) = serde_json::to_vec(&body.evidence) else {
        return Dispatch::error(400, "refused", "provider_callback_refused");
    };
    let Ok(evidence) = ProviderEvidence::new(evidence_bytes) else {
        return Dispatch::error(400, "refused", "provider_callback_refused");
    };
    let evidence_verifier = FiatEvidenceVerifier {
        pins: &config.manifest.fiat_providers,
        expected_rail: adapter,
    };
    let Ok(facts) = FiatAdapter::verify_evidence(&token, &evidence, &evidence_verifier, trace)
    else {
        return Dispatch::error(400, "refused", "provider_callback_refused");
    };
    match facts.class {
        EvidenceClass::Authorised => fiat_state(&FiatJourneyState::AuthorisedHold {
            until: facts.hold_until.unwrap_or(facts.observed_at),
        }),
        EvidenceClass::Clearing => fiat_state(&FiatJourneyState::ClearingHold {
            until: facts.hold_until.unwrap_or(facts.observed_at),
        }),
        EvidenceClass::Settled | EvidenceClass::Reversed | EvidenceClass::Chargeback => {
            let activity = match decode_hex(&body.activity, MAX_BODY) {
                Ok(value) if !value.is_empty() => value,
                _ => return Dispatch::error(400, "refused", "typed_intent_required"),
            };
            let Ok(observed_at) = now() else {
                return Dispatch::error(503, "pending", "server_clock_unavailable");
            };
            let Ok(mut gateway) = gateway_core(config, trace, observed_at) else {
                return Dispatch::error(503, "pending", "adapter_configuration_invalid");
            };
            let Ok(relay) = chain_relay(config) else {
                return Dispatch::error(503, "pending", "settlement_unavailable");
            };
            let mut plane = FiatServicePlane {
                relay,
                transaction: activity,
                observed_at,
                evidence: None,
                submitted: None,
                refusal: None,
            };
            let outcome = FiatAdapter::apply(
                &mut gateway,
                principal,
                &token,
                &evidence,
                &evidence_verifier,
                &mut plane,
                trace,
                observed_at,
            );
            fiat_executed(outcome, &plane)
        }
    }
}

fn fiat_executed(
    outcome: Result<FiatJourneyState, layerx_interop_gateway::trace::Traced<FiatError>>,
    plane: &FiatServicePlane,
) -> Dispatch {
    if let Some(code) = plane.refusal {
        let status = if code == "transaction_replacement_refused" {
            409
        } else {
            400
        };
        return Dispatch::error(status, "refused", code);
    }
    let state = match outcome {
        Ok(state) => state,
        Err(error) => {
            return Dispatch::error(
                503,
                "pending",
                match error.into_error() {
                    FiatError::ReceiptMismatch => "receipt_intent_mismatch",
                    _ => "settlement_unavailable",
                },
            )
        }
    };
    if let Some(tx_hash) = plane.submitted {
        return submitted_payment(tx_hash);
    }
    let mut result = fiat_state(&state);
    if matches!(
        state,
        FiatJourneyState::Credited { .. }
            | FiatJourneyState::Reversed { .. }
            | FiatJourneyState::ChargedBack { .. }
    ) {
        if let Some(evidence) = &plane.evidence {
            result.receipt_hex = Some(hex(&evidence.signed_receipt));
            result.activity_id = Some(hex(&evidence.transaction_hash));
        }
    }
    result
}

fn fiat_state(state: &FiatJourneyState) -> Dispatch {
    match state {
        FiatJourneyState::AuthorisedHold { until } => Dispatch::result(
            200,
            "completed",
            json!({ "state": "authorised-hold", "until": until }),
        ),
        FiatJourneyState::ClearingHold { until } => Dispatch::result(
            200,
            "completed",
            json!({ "state": "clearing-hold", "until": until }),
        ),
        FiatJourneyState::CreditPending => Dispatch::result(
            202,
            "pending",
            json!({ "state": ExternalState::Pending.label() }),
        ),
        FiatJourneyState::ReversalPending { hold_until } => Dispatch::result(
            202,
            "pending",
            json!({ "state": ExternalState::ReversalPending.label(), "hold_until": hold_until }),
        ),
        FiatJourneyState::ChargebackPending { hold_until } => Dispatch::result(
            202,
            "pending",
            json!({ "state": "chargeback-pending", "hold_until": hold_until }),
        ),
        FiatJourneyState::Credited { receipt_digest } => Dispatch::result(
            200,
            "completed",
            json!({ "state": ExternalState::ReceiptVerified.label(), "receipt_digest": hex(receipt_digest) }),
        ),
        FiatJourneyState::Reversed { receipt_digest } => Dispatch::result(
            200,
            "completed",
            json!({ "state": ExternalState::Reversed.label(), "receipt_digest": hex(receipt_digest) }),
        ),
        FiatJourneyState::ChargedBack { receipt_digest } => Dispatch::result(
            200,
            "completed",
            json!({ "state": "charged-back", "receipt_digest": hex(receipt_digest) }),
        ),
        FiatJourneyState::Refused => Dispatch::result(
            200,
            "refused",
            json!({ "state": ExternalState::Refused.label() }),
        ),
    }
}

struct FiatEvidenceVerifier<'a> {
    pins: &'a [FiatProviderPin],
    expected_rail: HostedAdapter,
}

impl ProviderVerifier for FiatEvidenceVerifier<'_> {
    fn verify(
        &self,
        token: &TokenReference,
        evidence: &ProviderEvidence,
        _trace: &TraceId,
    ) -> Result<VerifiedProviderFacts, layerx_fiat::FiatError> {
        let envelope: FiatEvidenceEnvelope = serde_json::from_slice(evidence.canonical())
            .map_err(|_| layerx_fiat::FiatError::InvalidEvidence)?;
        let pin = self
            .pins
            .iter()
            .find(|pin| pin.provider == envelope.facts.provider)
            .ok_or(layerx_fiat::FiatError::InvalidEvidence)?;
        let public = parse_hex32(&pin.public_key_ed25519)
            .map_err(|_| layerx_fiat::FiatError::InvalidEvidence)?;
        let key =
            Ed25519Key::from_bytes(&public).map_err(|_| layerx_fiat::FiatError::InvalidEvidence)?;
        let signature_bytes = decode_hex(&envelope.signature, 64)
            .map_err(|_| layerx_fiat::FiatError::InvalidEvidence)?;
        let signature = Signature::from_slice(&signature_bytes)
            .map_err(|_| layerx_fiat::FiatError::InvalidEvidence)?;
        let canonical = serde_json::to_vec(&envelope.facts)
            .map_err(|_| layerx_fiat::FiatError::InvalidEvidence)?;
        let mut signed = Vec::with_capacity(FIAT_EVIDENCE_SIGNATURE_DOMAIN.len() + canonical.len());
        signed.extend_from_slice(FIAT_EVIDENCE_SIGNATURE_DOMAIN);
        signed.extend_from_slice(&canonical);
        key.verify(&signed, &signature)
            .map_err(|_| layerx_fiat::FiatError::InvalidEvidence)?;
        let expected_token_digest = parse_hex32(&envelope.facts.token_reference_sha256)
            .map_err(|_| layerx_fiat::FiatError::InvalidEvidence)?;
        let actual_token_digest: [u8; 32] = Sha256::digest(token.as_bytes()).into();
        if expected_token_digest
            .ct_eq(&actual_token_digest)
            .unwrap_u8()
            != 1
        {
            return Err(layerx_fiat::FiatError::InvalidEvidence);
        }
        let rail = match envelope.facts.rail.as_str() {
            "card" if self.expected_rail == HostedAdapter::FiatCard => FiatRail::Card,
            "bank" if self.expected_rail == HostedAdapter::FiatBank => FiatRail::Bank,
            "rtp" if self.expected_rail == HostedAdapter::FiatRtp => FiatRail::RealTimePayment,
            _ => return Err(layerx_fiat::FiatError::InvalidEvidence),
        };
        let class = match envelope.facts.class.as_str() {
            "authorised" => EvidenceClass::Authorised,
            "clearing" => EvidenceClass::Clearing,
            "settled" => EvidenceClass::Settled,
            "reversed" => EvidenceClass::Reversed,
            "chargeback" => EvidenceClass::Chargeback,
            _ => return Err(layerx_fiat::FiatError::InvalidEvidence),
        };
        Ok(VerifiedProviderFacts {
            provider: ExternalId::new(envelope.facts.provider)?,
            settlement: ExternalId::new(envelope.facts.settlement)?,
            rail,
            class,
            amount: envelope
                .facts
                .amount
                .parse()
                .map_err(|_| layerx_fiat::FiatError::InvalidEvidence)?,
            asset: parse_hex32(&envelope.facts.asset)
                .map_err(|_| layerx_fiat::FiatError::InvalidEvidence)?,
            destination: parse_hex32(&envelope.facts.destination)
                .map_err(|_| layerx_fiat::FiatError::InvalidEvidence)?,
            observed_at: envelope.facts.observed_at,
            hold_until: envelope.facts.hold_until,
        })
    }
}

fn direct_body<T: DeserializeOwned>(request: &IncomingRequest) -> Result<T, ()> {
    if request.headers.get("content-type").map(String::as_str) != Some("application/json") {
        return Err(());
    }
    serde_json::from_slice(&request.body).map_err(|_| ())
}

fn typed_body<T: DeserializeOwned>(
    request: &IncomingRequest,
    _transport: IngressTransport,
) -> Result<T, ()> {
    direct_body(request)
}

fn transport_kind(transport: IngressTransport) -> TransportKind {
    match transport {
        IngressTransport::Http => TransportKind::Http,
        IngressTransport::Mcp => TransportKind::Mcp,
        IngressTransport::A2a => TransportKind::A2a,
    }
}

fn live() -> OutgoingResponse {
    json_response(
        200,
        &json!({ "status": "live", "service": "layerx-interop-gateway", "package_semver": env!("CARGO_PKG_VERSION") }),
    )
}

struct SettlementReadiness {
    durable: bool,
    sequencer: bool,
    configured: bool,
    observed_at_seconds: u64,
}

fn adapter_configured(config: &Config, adapter: &str, now: u64) -> bool {
    match adapter {
        "x402" => !config.manifest.x402_supported.kinds.is_empty(),
        "ap2" => !config.manifest.ap2_keys.is_empty() && !config.manifest.ap2_assets.is_empty(),
        "ucp" => true,
        "visa-tap" => {
            !config.manifest.visa_targets.is_empty()
                && config
                    .manifest
                    .visa_agents
                    .iter()
                    .any(|key| key.status == "active" && key.expires_at > now)
        }
        "fiat" => !config.manifest.fiat_providers.is_empty(),
        _ => false,
    }
}

impl SettlementReadiness {
    fn observe(config: &Config) -> Self {
        let observed = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .ok()
            .map(|now| now.as_secs());
        let observed_at_seconds = observed.unwrap_or(0);
        let configured = observed.is_some()
            && ["x402", "ap2", "ucp", "visa-tap", "fiat"]
                .iter()
                .all(|adapter| adapter_configured(config, adapter, observed_at_seconds));
        let durable = config.store.ready();
        let started = Instant::now();
        let sequencer = config.sequencer.receipt_keys().is_ok_and(|published| {
            receipt_keys_ready(
                &config.receipt_keys,
                &published,
                started.elapsed(),
                Duration::from_millis(config.readiness_max_age_ms),
            )
        });
        Self {
            configured,
            observed_at_seconds,
            durable,
            sequencer,
        }
    }

    fn settlement(&self) -> bool {
        self.configured && self.durable && self.sequencer
    }

    fn capabilities(&self, chain_id: u64) -> Value {
        json!({
            "chain_serving": {
                "state": readiness(self.sequencer),
                "reason": if self.sequencer { "ready" } else { "unreachable" },
                "chain_id": chain_id
            },
            "layerx_settlement": {
                "state": readiness(self.settlement()),
                "reason": if self.settlement() { "ready" } else { "unreachable" }
            }
        })
    }
}

fn receipt_keys_ready(
    pinned: &[SequencerAuthorization],
    published: &[ReceiptKeyEntry],
    age: Duration,
    max_age: Duration,
) -> bool {
    age <= max_age
        && !pinned.is_empty()
        && pinned.iter().all(|pin| {
            published.iter().any(|key| {
                key.key_id == pin.sequencer_id()
                    && key.public_key == pin.public_key()
                    && key.first_height == pin.first_batch_number()
                    && key.last_height == pin.last_batch_number()
            })
        })
}

impl SettlementReadiness {
    fn components(&self) -> Value {
        json!({
            "durable_gateway_store": readiness(self.durable),
            "sequencer": readiness(self.sequencer)
        })
    }

    fn ready_response(
        &self,
        chain_id: u64,
        network_id: &str,
        wire_version: &str,
        protocol_version: u16,
        protocol_network_id: u32,
    ) -> OutgoingResponse {
        let ready = self.settlement();
        json_response(
            if ready { 200 } else { 503 },
            &json!({
                "readiness_version": 1,
                "service": "layerx-interop-gateway",
                "status": if ready { "ready" } else { "degraded" },
                "network_id": network_id,
                "lxp_wire_version": wire_version,
                "protocol_version": protocol_version,
                "protocol_network_id": protocol_network_id,
                "capabilities": self.capabilities(chain_id),
                "components": self.components()
            }),
        )
    }
}

fn ready(config: &Config) -> OutgoingResponse {
    SettlementReadiness::observe(config).ready_response(
        config.readiness_chain_id,
        &config.network_id,
        &config.wire_version,
        config.protocol_version,
        config.protocol_network_id,
    )
}

fn metadata(config: &Config) -> OutgoingResponse {
    let state = SettlementReadiness::observe(config);
    let adapters: Vec<_> = config.manifest.adapters.values().map(|registered| {
        let descriptor = &registered.descriptor;
        let configured = adapter_configured(config, descriptor.id().as_str(), state.observed_at_seconds);
        let verification_boundary = match descriptor.id().as_str() {
            "x402" => "signed-402lxp-transfer-requirements-pre-settlement",
            "ap2" => "verified-mandate-and-layerx-receipt",
            "ucp" => "negotiated-checkout-and-layerx-receipt",
            "visa-tap" => "trusted-agent-credential-and-layerx-receipt-on-execution",
            "fiat" => "verified-provider-settlement-and-layerx-receipt",
            _ => "unavailable",
        };
        json!({
            "id": descriptor.id().as_str(), "specification": descriptor.spec().protocol().as_str(),
            "version": descriptor.spec().version().as_str(), "specification_sha256": hex(&descriptor.spec().document_digest()),
            "conformance_suite": descriptor.conformance().suite().as_str(), "conformance_vectors": descriptor.conformance().vector_count(),
            "conformance_sha256": hex(&descriptor.conformance().suite_digest()), "evidence_policy": registered.evidence.label(),
            "verification_boundary": verification_boundary,
            "readiness": {
                "configuration": readiness(configured),
                "ingress": readiness(state.durable && configured),
                "settlement": readiness(state.settlement() && configured),
                "receipt_verification": readiness(state.sequencer && configured)
            }
        })
    }).collect();
    let transports: Vec<_> = config.manifest.transports.values().map(|pin| json!({
        "id": pin.id, "version": pin.version, "specification_sha256": pin.specification_sha256,
        "conformance_sha256": pin.conformance_sha256
    })).collect();
    json_response(
        200,
        &json!({ "readiness_version": 1,
        "capabilities": state.capabilities(config.readiness_chain_id),
        "components": state.components(), "adapters": adapters, "transports": transports }),
    )
}

const fn readiness(value: bool) -> &'static str {
    if value {
        "ready"
    } else {
        "unavailable"
    }
}

fn stored_response(
    state: &str,
    stored: &str,
    trace: &TraceId,
    operation: &str,
) -> OutgoingResponse {
    match decode_hex(stored, MAX_BODY) {
        Ok(body) if !body.is_empty() => OutgoingResponse {
            content_type: "application/json".to_owned(),
            headers: Vec::new(),
            status: if state == "pending" { 202 } else { 200 },
            body,
            retry_after: None,
        },
        _ if state == "pending" => json_response(
            202,
            &json!({ "ok": true, "operation": operation, "result": { "state": "pending" }, "trace": trace.as_str() }),
        ),
        _ => failure(503, "persistence_unavailable", Some(5)),
    }
}

fn failure(status: u16, code: &str, retry_after: Option<u64>) -> OutgoingResponse {
    OutgoingResponse {
        content_type: "application/json".to_owned(),
        headers: Vec::new(),
        status,
        body: json!({ "ok": false, "error": { "code": code } })
            .to_string()
            .into_bytes(),
        retry_after,
    }
}

fn json_response(status: u16, value: &Value) -> OutgoingResponse {
    OutgoingResponse {
        content_type: "application/json".to_owned(),
        headers: Vec::new(),
        status,
        body: value.to_string().into_bytes(),
        retry_after: None,
    }
}

fn trace(request: &IncomingRequest) -> TraceId {
    let mut digest = Sha256::new();
    digest.update(request.method.as_bytes());
    digest.update([0]);
    digest.update(request.path.as_bytes());
    digest.update([0]);
    digest.update(&request.body);
    digest.update(now().unwrap_or(1).to_be_bytes());
    let output = digest.finalize();
    let mut entropy = [0_u8; 16];
    entropy.copy_from_slice(&output[..16]);
    TraceId::from_inbound(
        request.headers.get("x-trace-id").map(String::as_str),
        entropy,
    )
}

fn now() -> Result<u64, String> {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|value| value.as_secs())
        .map_err(|_| "system clock precedes Unix epoch".to_owned())
}

fn valid_identifier(value: &str, maximum: usize) -> bool {
    !value.is_empty()
        && value.len() <= maximum
        && value
            .bytes()
            .all(|byte| byte.is_ascii_alphanumeric() || matches!(byte, b'-' | b'_'))
}

fn hex(bytes: &[u8]) -> String {
    const DIGITS: &[u8; 16] = b"0123456789abcdef";
    let mut value = String::with_capacity(bytes.len() * 2);
    for byte in bytes {
        value.push(char::from(DIGITS[usize::from(byte >> 4)]));
        value.push(char::from(DIGITS[usize::from(byte & 0x0f)]));
    }
    value
}

#[cfg(test)]
mod tests {
    use super::*;
    use ed25519_dalek::{Signer as _, SigningKey};
    use layerx_interop_gateway::adapter::{AdapterId, ConformanceSuite};
    use layerx_platform_gateway::http;
    use layerx_x402::facilitator::{FacilitatorKind, SupportedResponse};
    use layerx_x402::model::AtomicAmount;
    use layerx_x402::{x402_adapter_descriptor, PaymentPayload};
    use native_tls::Certificate;
    use std::fs;
    use std::net::{TcpListener, TcpStream};
    use std::path::PathBuf;
    use std::process::{Child, Command, Stdio};
    use std::sync::Arc;
    use std::thread;
    use std::time::{Duration, Instant};

    #[test]
    fn no_route_posts_hosted_activities() {
        let source = include_str!("server.rs")
            .split("\n#[cfg(test)]\nmod tests {")
            .next()
            .expect("server implementation");
        for retired_path in ["/v1/activities", "/v1/authorized-batches"] {
            assert!(
                !source.contains(retired_path),
                "server still names retired outbound path {retired_path}"
            );
        }
    }

    #[test]
    fn readyz_reports_the_sequencer_component() -> X402TestResult {
        let public_key = SigningKey::from_bytes(&[0x73; 32])
            .verifying_key()
            .to_bytes();
        let pin = SequencerAuthorization::new([0x72; 32], public_key, 1000, 1999);
        let key = ReceiptKeyEntry {
            key_id: pin.sequencer_id(),
            public_key,
            first_height: 1000,
            last_height: 1999,
        };
        let max_age = Duration::from_millis(100);
        let mut state = SettlementReadiness {
            durable: true,
            sequencer: receipt_keys_ready(&[pin], &[key], max_age, max_age),
            configured: true,
            observed_at_seconds: 1_700_000_000,
        };
        let response = state.ready_response(2025, "testnet", "1", 1, 2025);
        assert_eq!(response.status, 200);
        let document: Value = serde_json::from_slice(&response.body)?;
        assert_eq!(document["status"], "ready");
        assert_eq!(
            document["components"],
            json!({
                "durable_gateway_store": "ready",
                "sequencer": "ready"
            })
        );
        assert_eq!(
            document["capabilities"]["layerx_settlement"]["state"],
            "ready"
        );
        assert!(document["components"].get("hosted").is_none());
        assert!(document["components"].get("authority").is_none());
        assert!(document["components"].get("hosted_gateway").is_none());
        assert!(document["components"].get("receipt_authority").is_none());

        for published in [
            Vec::new(),
            vec![ReceiptKeyEntry {
                key_id: [0x74; 32],
                ..key
            }],
            vec![ReceiptKeyEntry {
                public_key: SigningKey::from_bytes(&[0x75; 32])
                    .verifying_key()
                    .to_bytes(),
                ..key
            }],
            vec![ReceiptKeyEntry {
                first_height: 999,
                ..key
            }],
            vec![ReceiptKeyEntry {
                last_height: 2000,
                ..key
            }],
        ] {
            state.sequencer = receipt_keys_ready(&[pin], &published, Duration::ZERO, max_age);
            assert!(!state.sequencer);
            let response = state.ready_response(2025, "testnet", "1", 1, 2025);
            assert_eq!(response.status, 503);
            let document: Value = serde_json::from_slice(&response.body)?;
            assert_eq!(document["components"]["sequencer"], "unavailable");
            assert_eq!(
                document["capabilities"]["layerx_settlement"]["state"],
                "unavailable"
            );
        }
        assert!(!receipt_keys_ready(&[], &[key], Duration::ZERO, max_age));
        assert!(!receipt_keys_ready(
            &[pin],
            &[key],
            max_age + Duration::from_nanos(1),
            max_age
        ));
        state.sequencer = true;
        state.durable = false;
        assert_eq!(
            state.ready_response(2025, "testnet", "1", 1, 2025).status,
            503
        );
        state.durable = true;
        state.configured = false;
        assert_eq!(
            state.ready_response(2025, "testnet", "1", 1, 2025).status,
            503
        );
        Ok(())
    }

    fn parsed_tap(authority: &str, path: &str) -> TapRequest {
        TapRequest::parse(
            authority,
            path,
            "sig2=(\"@authority\" \"@path\");created=1;keyid=\"tap-key-1\";alg=\"Ed25519\";expires=100;nonce=\"target-nonce\";tag=\"agent-payer-auth\"",
            "sig2=:AA==:",
        )
        .unwrap_or_else(|error| panic!("TAP target fixture must parse: {error}"))
    }

    fn visa_pin(status: &str, expires_at: u64) -> VisaAgentPin {
        VisaAgentPin {
            key_id: "tap-key-1".to_owned(),
            agent_id: "trusted-agent-1".to_owned(),
            agent_domain: "https://agent.example".to_owned(),
            layerx_agent: "11".repeat(32),
            algorithm: "ed25519".to_owned(),
            public_key: "22".repeat(32),
            status: status.to_owned(),
            expires_at,
        }
    }

    fn visa_wire() -> serde_json::Value {
        serde_json::json!({
            "authority": "shop.example",
            "path": "/checkout",
            "signature_input": "sig2=(\"@authority\" \"@path\")",
            "signature": "sig2=:AA==:"
        })
    }

    fn signed_fiat_evidence(signing: &SigningKey, token: &TokenReference) -> ProviderEvidence {
        let facts = FiatFacts {
            provider: "provider-001".to_owned(),
            settlement: "settlement-001".to_owned(),
            token_reference_sha256: hex(&Sha256::digest(token.as_bytes())),
            rail: "card".to_owned(),
            class: "settled".to_owned(),
            amount: "10000".to_owned(),
            asset: "41".repeat(32),
            destination: "42".repeat(32),
            observed_at: 1_700_000_000,
            hold_until: None,
        };
        let canonical = serde_json::to_vec(&facts)
            .unwrap_or_else(|error| panic!("fiat facts serialize: {error:?}"));
        let mut signed = FIAT_EVIDENCE_SIGNATURE_DOMAIN.to_vec();
        signed.extend_from_slice(&canonical);
        let signature = signing.sign(&signed);
        ProviderEvidence::new(
            serde_json::to_vec(&FiatEvidenceEnvelope {
                facts,
                signature: hex(&signature.to_bytes()),
            })
            .unwrap_or_else(|error| panic!("fiat evidence serializes: {error:?}")),
        )
        .unwrap_or_else(|error| panic!("fiat evidence is bounded: {error:?}"))
    }

    #[test]
    fn visa_wire_has_no_caller_time_or_trust_override() {
        assert!(serde_json::from_value::<VisaRequest>(visa_wire()).is_ok());
        let mut caller_time = visa_wire();
        caller_time["now"] = serde_json::json!(u64::MAX);
        assert!(serde_json::from_value::<VisaRequest>(caller_time).is_err());
        let mut trust_override = visa_wire();
        trust_override["verified"] = serde_json::json!(true);
        assert!(serde_json::from_value::<VisaRequest>(trust_override).is_err());
    }

    #[test]
    fn unrelated_activity_decoration_and_browse_execution_are_refused() {
        assert_eq!(
            require_visa_route(false, AgentIntent::Browse, "00"),
            Err("unexpected_activity_decoration")
        );
        assert_eq!(
            require_visa_route(true, AgentIntent::Browse, "00"),
            Err("payer_credential_required")
        );
    }

    #[test]
    fn visa_target_refuses_cross_authority_and_cross_path_credentials() {
        let target = VisaTargetPin {
            principal_digest: "33".repeat(32),
            authority: "shop.example".to_owned(),
            path: "/checkout".to_owned(),
        };
        assert!(require_visa_target(&parsed_tap("shop.example", "/checkout"), &target).is_ok());
        assert!(require_visa_target(&parsed_tap("other.example", "/checkout"), &target).is_err());
        assert!(require_visa_target(&parsed_tap("shop.example", "/other"), &target).is_err());
    }

    #[test]
    fn visa_registry_preserves_revocation_and_expiry_instead_of_synthesizing_active() {
        let revoked = vec![visa_pin("revoked", 100)];
        let revoked_registry = VisaRegistry { pins: &revoked };
        assert_eq!(
            TapVerifier::verify_credential(
                &parsed_tap("shop.example", "/checkout"),
                &revoked_registry,
                1,
                0,
            ),
            Err(TapError::Revoked)
        );
        let expired = vec![visa_pin("active", 1)];
        let expired_registry = VisaRegistry { pins: &expired };
        assert_eq!(
            TapVerifier::verify_credential(
                &parsed_tap("shop.example", "/checkout"),
                &expired_registry,
                2,
                0,
            ),
            Err(TapError::ExpiredKey)
        );
        assert_eq!(
            revoked_registry.resolve("unknown-key", 1),
            Err(TapError::UnknownKey)
        );
    }

    #[test]
    fn tap_agent_must_be_the_authenticated_activity_signer() {
        assert!(require_visa_actor([0x41; 32], [0x41; 32]).is_ok());
        assert_eq!(
            require_visa_actor([0x41; 32], [0x42; 32]),
            Err("layerx_agent_signer_mismatch")
        );
    }

    #[test]
    fn receipt_must_match_the_exact_submitted_activity() -> X402TestResult {
        let (raw, answer, keys) = relay_fixture([0x51; 32])?;
        let (_, _, submitted) = decode_pay_exact(&raw)?;
        let receipt = decode_hex(
            answer["receipt"]
                .as_str()
                .and_then(|value| value.strip_prefix("0x"))
                .ok_or("signed receipt")?,
            MAX_BODY,
        )?;
        let verified = verify_sequencer_signature(&receipt, keys.as_slice())
            .map_err(|error| format!("sequencer receipt verification: {error:?}"))?;
        let receipted = verified.protocol().ok_or("protocol receipt")?.activity_id();
        assert_eq!(submitted, receipted);
        let (replacement, _, _) = relay_fixture([0x52; 32])?;
        let (_, _, replacement_id) = decode_pay_exact(&replacement)?;
        assert_ne!(replacement_id, receipted);
        Ok(())
    }

    #[test]
    fn fiat_provider_signature_binds_the_opaque_token_reference() {
        let signing = SigningKey::from_bytes(&[0x61; 32]);
        let token = TokenReference::new(b"provider-token-a".to_vec())
            .unwrap_or_else(|error| panic!("opaque provider token is valid: {error:?}"));
        let substituted = TokenReference::new(b"provider-token-b".to_vec())
            .unwrap_or_else(|error| panic!("substitute provider token is valid: {error:?}"));
        let evidence = signed_fiat_evidence(&signing, &token);
        let pins = [FiatProviderPin {
            provider: "provider-001".to_owned(),
            public_key_ed25519: hex(signing.verifying_key().as_bytes()),
        }];
        let evidence_verifier = FiatEvidenceVerifier {
            pins: &pins,
            expected_rail: HostedAdapter::FiatCard,
        };
        let trace = TraceId::mint([0x62; 16]);
        assert!(evidence_verifier.verify(&token, &evidence, &trace).is_ok());
        assert_eq!(
            evidence_verifier.verify(&substituted, &evidence, &trace),
            Err(layerx_fiat::FiatError::InvalidEvidence)
        );
    }

    #[test]
    fn fiat_callback_refuses_caller_selected_economic_identity() {
        let callback = serde_json::json!({
            "token_reference": "provider-token-a",
            "evidence": {
                "facts": {
                    "provider": "provider-001",
                    "settlement": "settlement-001",
                    "token_reference_sha256": "11".repeat(32),
                    "rail": "card",
                    "class": "settled",
                    "amount": "10000",
                    "asset": "41".repeat(32),
                    "destination": "42".repeat(32),
                    "observed_at": 1_700_000_000,
                    "hold_until": null
                },
                "signature": "22".repeat(64)
            },
            "activity": "00",
            "activity_idempotency_key": "33".repeat(32)
        });
        assert!(serde_json::from_value::<FiatCallback>(callback).is_err());
    }

    #[test]
    fn visa_execute_refuses_a_transaction_to_another_contract() -> X402TestResult {
        let fixtures: Value = serde_json::from_str(include_str!(
            "../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
        ))?;
        let transfer = fixtures["cases"]
            .as_array()
            .ok_or("fixture cases")?
            .iter()
            .find(|case| case["id"] == "V1-pax-transfer")
            .ok_or("recorded PAX transfer")?;
        let transaction = transfer["rawTransaction"]
            .as_str()
            .and_then(|raw| raw.strip_prefix("0x"))
            .ok_or("transaction hex")?;
        let refusal = match visa_activity(transaction, true) {
            Err(refusal) => refusal,
            Ok(_) => panic!("Visa must refuse the recorded transaction to another contract"),
        };
        assert_eq!(refusal.status, 400);
        assert_eq!(refusal.durable_state, "refused");
        assert_eq!(refusal.error, Some("transaction_target_refused"));
        assert!(refusal.receipt_hex.is_none());

        let (raw, receipt, keys) = relay_fixture([0x71; 32])?;
        let (_, _, hash) = decode_pay_exact(&raw)?;
        let redis = X402RedisProcess::start()?;
        let (relay, server) = payment_relay(&redis, &raw, &receipt, keys)?;
        let binding = visa_activity(&hex(&raw), true)
            .map_err(|dispatch| format!("valid Visa transaction: {:?}", dispatch.error))?;
        let pending = visa_execute(&relay, binding, 1_700_000_000);
        assert_eq!(pending.status, 202);
        assert_eq!(pending.result["state"], "submitted");
        assert_eq!(pending.result["transaction"], format!("0x{}", hex(&hash)));
        assert!(pending.receipt_hex.is_none());
        let binding = visa_activity(&hex(&raw), true)
            .map_err(|dispatch| format!("valid Visa retry: {:?}", dispatch.error))?;
        let completed = visa_execute(&relay, binding, 1_700_000_000);
        server
            .join()
            .map_err(|_| "Visa sequencer fixture panicked")??;
        let signed_receipt = decode_hex(
            receipt["receipt"]
                .as_str()
                .and_then(|text| text.strip_prefix("0x"))
                .ok_or("receipt hex")?,
            MAX_BODY,
        )?;
        assert_eq!(completed.status, 200);
        assert_eq!(completed.durable_state, "completed");
        assert_eq!(completed.result["state"], "receipt-verified");
        assert_eq!(completed.receipt_hex, Some(hex(&signed_receipt)));
        assert_eq!(completed.activity_id, Some(hex(&hash)));
        assert_eq!(
            completed.result["receipt_digest"],
            hex(&layerx_interop_gateway::evidence::canonical_receipt_digest(
                &signed_receipt
            )?)
        );
        Ok(())
    }

    #[test]
    fn fiat_callback_settles_through_the_relay() -> X402TestResult {
        use layerx_interop_gateway::adapter::{PinnedSpec, SpecVersion};
        let (recorded, _) = recorded_x402_request("V1-layerxledger-pay-exact")?;
        let raw = recorded.payment_payload.payload["layerxTransaction"]
            .as_str()
            .and_then(|text| text.strip_prefix("0x"))
            .ok_or("raw transaction")?;
        let (payment, _, _) = decode_pay_exact(&decode_hex(raw, MAX_BODY)?)?;
        let token_text = "provider-relay-token";
        let facts = FiatFacts {
            provider: "provider-relay".into(),
            settlement: "settlement-relay".into(),
            token_reference_sha256: hex(&Sha256::digest(token_text.as_bytes())),
            rail: "card".into(),
            class: "settled".into(),
            amount: payment.amount.to_string(),
            asset: hex(&payment.asset),
            destination: hex(&payment.to),
            observed_at: 1_700_000_000,
            hold_until: None,
        };
        let signing = SigningKey::from_bytes(&[0x72; 32]);
        let mut signed = FIAT_EVIDENCE_SIGNATURE_DOMAIN.to_vec();
        signed.extend_from_slice(&serde_json::to_vec(&facts)?);
        let envelope = FiatEvidenceEnvelope {
            facts,
            signature: hex(&signing.sign(&signed).to_bytes()),
        };
        let pins = [FiatProviderPin {
            provider: "provider-relay".into(),
            public_key_ed25519: hex(signing.verifying_key().as_bytes()),
        }];
        let verifier = FiatEvidenceVerifier {
            pins: &pins,
            expected_rail: HostedAdapter::FiatCard,
        };
        let trace = TraceId::mint([0x73; 16]);
        let token = TokenReference::new(token_text.as_bytes().to_vec())?;
        let evidence = ProviderEvidence::new(serde_json::to_vec(&envelope)?)?;
        let verified = FiatAdapter::verify_evidence(&token, &evidence, &verifier, &trace)
            .map_err(|error| format!("provider evidence: {error:?}"))?;
        let reference = FiatAdapter::idempotency_key(&verified);
        let (raw, receipt, keys) = relay_fixture(reference)?;
        let (_, _, hash) = decode_pay_exact(&raw)?;
        let request = IncomingRequest {
            method: "POST".into(),
            path: "/v1/fiat/card/callbacks".into(),
            headers: BTreeMap::from([("content-type".into(), "application/json".into())]),
            body: serde_json::to_vec(&json!({
                "token_reference": token_text, "evidence": envelope, "activity": hex(&raw),
            }))?,
        };
        let callback: FiatCallback = direct_body(&request).map_err(|_| "callback body")?;
        let token = TokenReference::new(callback.token_reference.into_bytes())?;
        let evidence = ProviderEvidence::new(serde_json::to_vec(&callback.evidence)?)?;
        let principal = PrincipalId::new("fiat-relay-principal")?;
        let mut gateway = GatewayCore::new();
        let spec = PinnedSpec::new(
            AdapterId::new("fiat")?,
            SpecVersion::parse("1")?,
            [0x74; 32],
        )?;
        let suite = ConformanceSuite::new(AdapterId::new("fiat-relay-vectors")?, 1, [0x75; 32])?;
        gateway
            .register_adapter(
                layerx_fiat::fiat_adapter_descriptor(spec, suite)?,
                &trace,
                1,
            )
            .map_err(|error| format!("register fiat: {error:?}"))?;
        let redis = X402RedisProcess::start()?;
        let (relay, server) = payment_relay(&redis, &raw, &receipt, keys)?;
        let mut plane = FiatServicePlane {
            relay,
            transaction: decode_hex(&callback.activity, MAX_BODY)?,
            observed_at: 1_700_000_000,
            evidence: None,
            submitted: None,
            refusal: None,
        };
        let pending = FiatAdapter::apply(
            &mut gateway,
            &principal,
            &token,
            &evidence,
            &verifier,
            &mut plane,
            &trace,
            1_700_000_000,
        );
        let pending = fiat_executed(pending, &plane);
        assert_eq!(pending.status, 202);
        assert_eq!(pending.result["state"], "submitted");
        assert_eq!(pending.result["transaction"], format!("0x{}", hex(&hash)));
        assert!(pending.receipt_hex.is_none());
        let completed = FiatAdapter::apply(
            &mut gateway,
            &principal,
            &token,
            &evidence,
            &verifier,
            &mut plane,
            &trace,
            1_700_000_000,
        );
        let completed = fiat_executed(completed, &plane);
        server
            .join()
            .map_err(|_| "fiat sequencer fixture panicked")??;
        let signed_receipt = decode_hex(
            receipt["receipt"]
                .as_str()
                .and_then(|text| text.strip_prefix("0x"))
                .ok_or("receipt hex")?,
            MAX_BODY,
        )?;
        let digest = layerx_interop_gateway::evidence::canonical_receipt_digest(&signed_receipt)?;
        assert_eq!(completed.status, 200);
        assert_eq!(completed.durable_state, "completed");
        assert_eq!(completed.result["state"], "receipt-verified");
        assert_eq!(completed.result["receipt_digest"], hex(&digest));
        assert_eq!(completed.receipt_hex, Some(hex(&signed_receipt)));
        assert_eq!(completed.activity_id, Some(hex(&hash)));
        assert_eq!(
            gateway.translation(&principal, reference),
            Some(TranslationStatus::ReceiptVerified {
                receipt_digest: digest
            })
        );
        Ok(())
    }

    fn relay_fixture(
        reference: [u8; 32],
    ) -> X402TestResult<(Vec<u8>, Value, Vec<SequencerAuthorization>)> {
        fn replace(bytes: &mut [u8], old: &[u8], new: &[u8]) {
            assert_eq!(old.len(), new.len());
            let positions: Vec<_> = bytes
                .windows(old.len())
                .enumerate()
                .filter_map(|(index, value)| (value == old).then_some(index))
                .collect();
            assert_eq!(positions.len(), 1);
            bytes[positions[0]..positions[0] + old.len()].copy_from_slice(new);
        }
        let fixture: Value = serde_json::from_str(include_str!(
            "../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
        ))?;
        let case = fixture["cases"]
            .as_array()
            .ok_or("cases")?
            .iter()
            .find(|case| case["id"] == "V1-layerxledger-pay-exact")
            .ok_or("payExact fixture")?;
        let bytes = |value: &Value| -> X402TestResult<Vec<u8>> {
            Ok(decode_hex(
                value
                    .as_str()
                    .and_then(|text| text.strip_prefix("0x"))
                    .ok_or("hex")?,
                MAX_BODY,
            )?)
        };
        let pin = &case["pinnedKey"];
        let keys = vec![SequencerAuthorization::new(
            bytes(&pin["keyId"])?
                .try_into()
                .map_err(|_| "key id width")?,
            bytes(&pin["publicKey"])?
                .try_into()
                .map_err(|_| "key width")?,
            pin["firstHeight"].as_u64().ok_or("first height")?,
            pin["lastHeight"].as_u64().ok_or("last height")?,
        )];
        let raw = bytes(&case["rawTransaction"])?;
        let (transfer, old_sender, old_hash) = decode_pay_exact(&raw)?;
        let (mut tx, _) = layerx_crypto::evm_transaction::decode_signed(&raw)?;
        replace(&mut tx.data, &transfer.reference, &reference);
        let raw = tx.sign(&[0x3b; 32])?;
        let (_, sender, hash) = decode_pay_exact(&raw)?;
        let mut receipt = bytes(&case["signedReceipt"])?;
        let verified = verify_sequencer_signature(&receipt, keys.as_slice())
            .map_err(|error| format!("recorded receipt: {error:?}"))?;
        let protocol = verified.protocol().ok_or("protocol receipt")?;
        let old_root = protocol.resulting_state_root();
        let mut sequence = Sha256::new();
        sequence.update(b"PAXEER/v1/sequence\0");
        sequence.update(protocol.previous_state_root());
        sequence.update(protocol.global_sequence().to_be_bytes());
        sequence.update(hash);
        let new_root: [u8; 32] = sequence.finalize().into();
        replace(&mut receipt, &transfer.reference, &reference);
        replace(&mut receipt, &old_hash, &hash);
        replace(&mut receipt, &old_root, &new_root);
        if old_sender != sender {
            replace(&mut receipt, &old_sender, &sender);
        }
        let key = SigningKey::from_bytes(&[0x5e; 32]);
        assert_eq!(key.verifying_key().to_bytes(), keys[0].public_key());
        let digest = layerx_interop_gateway::evidence::canonical_receipt_digest(&receipt)?;
        let signature_start = receipt.len() - 64;
        receipt[signature_start..].copy_from_slice(&key.sign(&digest).to_bytes());
        let mut answer = case["receiptJson"].clone();
        answer["txHash"] = json!(format!("0x{}", hex(&hash)));
        answer["from"] = json!(format!("0x{}", hex(&sender)));
        answer["receipt"] = json!(format!("0x{}", hex(&receipt)));
        answer["logs"][0]["topics"][3] = json!(format!("0x{}", hex(&reference)));
        Ok((raw, answer, keys))
    }

    fn payment_relay(
        redis: &X402RedisProcess,
        raw: &[u8],
        receipt: &Value,
        keys: Vec<SequencerAuthorization>,
    ) -> X402TestResult<(ChainRelay, thread::JoinHandle<Result<(), String>>)> {
        let listener = TcpListener::bind("127.0.0.1:0")?;
        listener.set_nonblocking(true)?;
        let endpoint = layerx_client::sequencer::SequencerEndpoint::new(
            &format!("http://{}", listener.local_addr()?),
            &[],
        )
        .map_err(|error| format!("sequencer endpoint: {error:?}"))?;
        let raw = format!("0x{}", hex(raw));
        let receipt = receipt.clone();
        let server = thread::spawn(move || -> Result<(), String> {
            let deadline = Instant::now() + Duration::from_secs(8);
            for (method, params, result) in [
                (
                    "eth_sendRawTransaction",
                    json!([raw]),
                    receipt["txHash"].clone(),
                ),
                (
                    "pax_sequencerReceipt",
                    json!([receipt["txHash"]]),
                    json!({
                        "rung": "submitted", "txHash": receipt["txHash"], "nonce": receipt["nonce"],
                    }),
                ),
                (
                    "pax_sequencerReceipt",
                    json!([receipt["txHash"]]),
                    receipt.clone(),
                ),
            ] {
                let mut stream = loop {
                    match listener.accept() {
                        Ok((stream, _)) => break stream,
                        Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                            if Instant::now() >= deadline {
                                return Err(format!("timed out waiting for {method}"));
                            }
                            thread::sleep(Duration::from_millis(5));
                        }
                        Err(error) => return Err(error.to_string()),
                    }
                };
                stream
                    .set_read_timeout(Some(Duration::from_secs(5)))
                    .map_err(|error| error.to_string())?;
                stream
                    .set_write_timeout(Some(Duration::from_secs(5)))
                    .map_err(|error| error.to_string())?;
                let request = http::read_request(&mut stream, 64 * 1024)?;
                let rpc: Value =
                    serde_json::from_slice(&request.body).map_err(|error| error.to_string())?;
                assert_eq!(request.method, "POST");
                assert_eq!(rpc["jsonrpc"], "2.0");
                assert_eq!(rpc["method"], method);
                assert_eq!(rpc["params"], params);
                http::write_response(
                    &mut stream,
                    &OutgoingResponse {
                        content_type: "application/json".to_owned(),
                        headers: vec![],
                        status: 200,
                        body: serde_json::to_vec(
                            &json!({"jsonrpc": "2.0", "id": rpc["id"], "result": result}),
                        )
                        .map_err(|error| error.to_string())?,
                        retry_after: None,
                    },
                )?;
            }
            Ok(())
        });
        Ok((
            ChainRelay {
                endpoint: Arc::new(endpoint),
                receipt_keys: keys,
                journal: redis.journal()?,
            },
            server,
        ))
    }

    fn ap2_relay_payment() -> X402TestResult<layerx_ap2::AuthorizedPayment> {
        use base64::engine::general_purpose::URL_SAFE_NO_PAD;
        use p256::ecdsa::{Signature, SigningKey as P256SigningKey};
        let issuer = P256SigningKey::from_bytes((&[7; 32]).into())?;
        let merchant = P256SigningKey::from_bytes((&[11; 32]).into())?;
        let jws = |header: Value, payload: Value, key: &P256SigningKey| -> X402TestResult<String> {
            let input = format!(
                "{}.{}",
                URL_SAFE_NO_PAD.encode(serde_json::to_vec(&header)?),
                URL_SAFE_NO_PAD.encode(serde_json::to_vec(&payload)?)
            );
            let signature: Signature = key.sign(input.as_bytes());
            Ok(format!(
                "{input}.{}",
                URL_SAFE_NO_PAD.encode(signature.to_bytes())
            ))
        };
        let checkout = jws(
            json!({"alg":"ES256","typ":"JWT","kid":"merchant"}),
            json!({
                "id":"checkout-12345", "merchant":{"id":"merchant-001","name":"Test Merchant"},
                "line_items":[{"id":"line-001","item":{"id":"sku-001","title":"Node credit","price":1000000},
                    "quantity":1,"totals":[{"type":"total","amount":1000000}]}],
                "status":"completed","currency":"USD","totals":[{"type":"total","amount":1000000}],"links":[]
            }),
            &merchant,
        )?;
        let reference = URL_SAFE_NO_PAD.encode(Sha256::digest(checkout.as_bytes()));
        let root = |mandate| -> X402TestResult<String> {
            Ok(format!(
                "{}~",
                jws(
                    json!({"alg":"ES256","typ":"dc+sd-jwt","kid":"issuer"}),
                    json!({"iat":1699999940_u64,"exp":1700003600_u64,"delegate_payload":[mandate]}),
                    &issuer
                )?
            ))
        };
        let checkout = root(json!({"vct":"mandate.checkout.1","checkout_jwt":checkout,
            "checkout_hash":reference,"iat":1699999970_u64,"exp":1700001800_u64}))?;
        let payment = root(json!({"vct":"mandate.payment.1","transaction_id":reference,
            "payee":{"id":"merchant-001","name":"Test Merchant"},
            "payment_amount":{"amount":1000000,"currency":"USD"},
            "payment_instrument":{"id":"card-001","type":"card"},"iat":1699999970_u64,"exp":1700001800_u64}))?;
        let pins = vec![
            Ap2KeyPin {
                use_case: "checkout-mandate".into(),
                key_id: "issuer".into(),
                public_key_sec1: hex(issuer.verifying_key().to_encoded_point(false).as_bytes()),
            },
            Ap2KeyPin {
                use_case: "payment-mandate".into(),
                key_id: "issuer".into(),
                public_key_sec1: hex(issuer.verifying_key().to_encoded_point(false).as_bytes()),
            },
            Ap2KeyPin {
                use_case: "merchant-checkout".into(),
                key_id: "merchant".into(),
                public_key_sec1: hex(merchant.verifying_key().to_encoded_point(false).as_bytes()),
            },
        ];
        let resolver = Ap2Resolver { keys: &pins };
        let context = VerificationContext {
            now: 1_700_000_000,
            clock_skew_seconds: 0,
            expected_audience: "merchant-api",
            expected_nonce: "nonce",
            currency_minor_exponent: 2,
            usage: None,
        };
        let verified = MandateVerifier::new(&resolver).verify(&checkout, &payment, &context)?;
        let (recorded, _) = recorded_x402_request("V1-layerxledger-pay-exact")?;
        let raw = recorded.payment_payload.payload["layerxTransaction"]
            .as_str()
            .ok_or("raw")?;
        let (transfer, _, _) = decode_pay_exact(&decode_hex(
            raw.strip_prefix("0x").ok_or("prefix")?,
            MAX_BODY,
        )?)?;
        Ok(authorize_payment(
            &PrincipalId::new("ap2-relay-principal")?,
            &verified,
            &context,
            &LayerXAssetBinding {
                currency: "USD".into(),
                minor_unit_exponent: 2,
                atomic_units_per_minor_unit: 1,
                asset: transfer.asset,
                payer_receipt_account: transfer.from,
                payee_receipt_account: transfer.to,
                payee_merchant: Merchant::new("merchant-001", "Test Merchant", None)?,
            },
        )?)
    }

    #[test]
    fn ap2_execute_relays_the_mandate_transaction() -> X402TestResult {
        let payment = ap2_relay_payment()?;
        let (raw, receipt, keys) = relay_fixture(payment.idempotency_key())?;
        let (_, _, hash) = decode_pay_exact(&raw)?;
        let redis = X402RedisProcess::start()?;
        let (relay, server) = payment_relay(&redis, &raw, &receipt, keys)?;
        let trace = TraceId::mint([0x45; 16]);
        let suite = ConformanceSuite::new(AdapterId::new("ap2-relay-vectors")?, 1, [0x46; 32])?;
        let mut gateway = GatewayCore::new();
        gateway
            .register_adapter(layerx_ap2::ap2_adapter_descriptor(suite)?, &trace, 1)
            .map_err(|error| format!("register: {error:?}"))?;
        let pending = ap2_execute(&payment, &raw, &relay, &mut gateway, &trace, 1_700_000_000);
        assert_eq!(pending.status, 202);
        assert_eq!(pending.result["state"], "submitted");
        assert_eq!(pending.result["transaction"], format!("0x{}", hex(&hash)));
        assert!(pending.receipt_hex.is_none());
        let mut replacement = layerx_crypto::evm_transaction::decode_signed(&raw)?.0;
        replacement.max_fee_per_gas += 1;
        let refused = ap2_execute(
            &payment,
            &replacement.sign(&[0x3b; 32])?,
            &relay,
            &mut gateway,
            &trace,
            1_700_000_000,
        );
        assert_eq!(refused.status, 409);
        assert_eq!(refused.error, Some("transaction_replacement_refused"));
        let wrong = relay_fixture([0x64; 32])?.0;
        let refused = ap2_execute(
            &payment,
            &wrong,
            &relay,
            &mut gateway,
            &trace,
            1_700_000_000,
        );
        assert_eq!(refused.status, 400);
        assert_eq!(refused.error, Some("mandate_payment_mismatch"));
        let completed = ap2_execute(&payment, &raw, &relay, &mut gateway, &trace, 1_700_000_000);
        server.join().map_err(|_| "sequencer fixture panicked")??;
        assert_eq!(completed.status, 200);
        assert_eq!(completed.durable_state, "completed");
        assert_eq!(completed.result["state"], "receipt-verified");
        let receipt = decode_hex(
            receipt["receipt"]
                .as_str()
                .and_then(|text| text.strip_prefix("0x"))
                .ok_or("receipt")?,
            MAX_BODY,
        )?;
        assert_eq!(completed.receipt_hex, Some(hex(&receipt)));
        assert_eq!(completed.activity_id, Some(hex(&hash)));
        assert_eq!(
            completed.result["receipt_digest"],
            hex(&layerx_interop_gateway::evidence::canonical_receipt_digest(
                &receipt
            )?)
        );
        Ok(())
    }

    #[test]
    fn ucp_complete_relays_the_checkout_transaction() -> X402TestResult {
        use layerx_interop_gateway::adapter::{PinnedSpec, SpecVersion};
        let key = UcpIdempotencyKey::parse("123e4567-e89b-12d3-a456-426614174000")?;
        let (raw, receipt, keys) = relay_fixture(key.gateway_key())?;
        let (transfer, _, hash) = decode_pay_exact(&raw)?;
        let handler = PaymentHandler::new(
            "layerx-402",
            "2026-04-08",
            "https://layerx.dev/2026-04-08/specification/402",
            "https://layerx.dev/2026-04-08/schemas/402.json",
        )?;
        let merchant =
            layerx_ucp::MerchantProfile::layerx("https://merchant.example/ucp", handler.clone())?;
        let platform = PlatformProfile {
            profile_url: "https://buyer.example/profile".into(),
            capabilities: merchant.capabilities().to_vec(),
            payment_handlers: vec![handler.clone()],
        };
        let submission = CheckoutSubmission {
            checkout_id: "checkout-relay".into(),
            currency: *b"USD",
            total_minor: transfer.amount,
            layerx_asset: transfer.asset,
            layerx_recipient: transfer.to,
            idempotency_key: key,
            negotiated: NegotiatedCapabilities::negotiate(&platform, &handler)?,
        };
        let trace = TraceId::mint([0x47; 16]);
        let principal = PrincipalId::new("ucp-relay-principal")?;
        let mut gateway = GatewayCore::new();
        let spec = PinnedSpec::new(
            AdapterId::new("ucp")?,
            SpecVersion::parse("20260408")?,
            layerx_ucp::UCP_CHECKOUT_SPEC_SHA256,
        )?;
        let suite = ConformanceSuite::new(AdapterId::new("ucp-relay-vectors")?, 1, [0x48; 32])?;
        gateway
            .register_adapter(layerx_ucp::ucp_adapter_descriptor(spec, suite)?, &trace, 1)
            .map_err(|error| format!("register: {error:?}"))?;
        let redis = X402RedisProcess::start()?;
        let (relay, server) = payment_relay(&redis, &raw, &receipt, keys.clone())?;
        let mut plane = UcpServicePlane {
            relay,
            transaction: raw.clone(),
            observed_at: 1_700_000_000,
            order_id: "order-relay".into(),
            permalink_url: "https://merchant.example/orders/relay".into(),
            receipt_hex: None,
            activity_id: None,
            unavailable: false,
            refusal: None,
            submitted: None,
        };
        let pending = UcpAdapter::complete_checkout(
            &mut gateway,
            &principal,
            &submission,
            &mut plane,
            &keys,
            &trace,
            1_700_000_000,
        )
        .map_err(|error| format!("pending checkout: {error:?}"))?;
        assert_eq!(pending.status, CheckoutStatus::CompleteInProgress);
        let pending = ucp_outcome(Ok(pending), &plane);
        assert_eq!(pending.status, 202);
        assert_eq!(pending.result["state"], "submitted");
        assert_eq!(pending.result["transaction"], format!("0x{}", hex(&hash)));
        assert!(pending.receipt_hex.is_none());

        let intent = UcpPaymentIntent {
            checkout_id: submission.checkout_id.clone(),
            currency: submission.currency,
            amount: transfer.amount + 1,
            asset: transfer.asset,
            recipient: transfer.to,
            idempotency_key: key.gateway_key(),
        };
        assert!(matches!(
            plane.execute(&intent, &trace)?,
            UcpPlaneResult::Refused
        ));
        assert_eq!(plane.refusal, Some("checkout_payment_mismatch"));
        let mut replacement = layerx_crypto::evm_transaction::decode_signed(&raw)?.0;
        replacement.max_fee_per_gas += 1;
        plane.transaction = replacement.sign(&[0x3b; 32])?;
        let valid_intent = UcpPaymentIntent {
            amount: transfer.amount,
            ..intent
        };
        assert!(matches!(
            plane.execute(&valid_intent, &trace)?,
            UcpPlaneResult::Refused
        ));
        assert_eq!(plane.refusal, Some("transaction_replacement_refused"));
        plane.transaction = raw;
        let completed = UcpAdapter::complete_checkout(
            &mut gateway,
            &principal,
            &submission,
            &mut plane,
            &keys,
            &trace,
            1_700_000_000,
        )
        .map_err(|error| format!("completed checkout: {error:?}"))?;
        server.join().map_err(|_| "sequencer fixture panicked")??;
        assert_eq!(completed.status, CheckoutStatus::Completed);
        let order = completed.order.as_ref().ok_or("receipt-backed order")?;
        assert_eq!(order.id, "order-relay");
        assert_eq!(order.transaction_hash, hash);
        let signed_receipt = decode_hex(
            receipt["receipt"]
                .as_str()
                .and_then(|text| text.strip_prefix("0x"))
                .ok_or("receipt")?,
            MAX_BODY,
        )?;
        assert_eq!(
            order.receipt_digest,
            layerx_interop_gateway::evidence::canonical_receipt_digest(&signed_receipt)?
        );
        let completed = ucp_outcome(Ok(completed), &plane);
        assert_eq!(completed.status, 200);
        assert_eq!(completed.result["state"], "receipt-verified");
        assert_eq!(completed.receipt_hex, Some(hex(&signed_receipt)));
        assert_eq!(completed.activity_id, Some(hex(&hash)));
        Ok(())
    }

    const X402_FIXTURE_ACCOUNT: &str = "agent:did:layerx:interop-merchant:main";

    fn x402_fixture_pay_to() -> String {
        const DIGITS: &[u8; 16] = b"0123456789abcdef";
        let identifiers = layerx_x402::model::account_identifiers(X402_FIXTURE_ACCOUNT)
            .unwrap_or_else(|error| panic!("fixture account has identifiers: {error:?}"));
        let mut text = String::with_capacity(64);
        for byte in identifiers[0] {
            text.push(char::from(DIGITS[usize::from(byte >> 4)]));
            text.push(char::from(DIGITS[usize::from(byte & 0x0f)]));
        }
        text
    }

    fn x402_fixture_request(payload: Value) -> FacilitatorRequest {
        let requirements = PaymentRequirements {
            scheme: "exact".to_owned(),
            network: "layerx:beta".to_owned(),
            amount: AtomicAmount::parse("1000")
                .unwrap_or_else(|error| panic!("fixture amount is canonical: {error:?}")),
            asset: "44".repeat(32),
            pay_to: x402_fixture_pay_to(),
            max_timeout_seconds: 60,
            extra: Some(serde_json::json!({
                "layerx": {
                    "commitment": "executed",
                    "account": X402_FIXTURE_ACCOUNT,
                    "currency": "LXP"
                }
            })),
        };
        FacilitatorRequest {
            x402_version: 2,
            payment_payload: PaymentPayload {
                x402_version: 2,
                resource: None,
                payload,
                accepted: requirements.clone(),
                extensions: BTreeMap::new(),
            },
            payment_requirements: requirements,
        }
    }

    fn x402_fixture_gateway(trace: &TraceId) -> GatewayCore {
        let conformance = ConformanceSuite::new(
            AdapterId::new("x402-v2-local-matrix")
                .unwrap_or_else(|error| panic!("suite id is valid: {error:?}")),
            1,
            [0x11; 32],
        )
        .unwrap_or_else(|error| panic!("conformance suite is declared: {error:?}"));
        let mut gateway = GatewayCore::new();
        gateway
            .register_adapter(
                x402_adapter_descriptor(conformance)
                    .unwrap_or_else(|error| panic!("x402 descriptor is valid: {error:?}")),
                trace,
                10,
            )
            .unwrap_or_else(|error| panic!("x402 adapter registers: {error:?}"));
        gateway
    }

    fn recorded_x402_request(
        case_id: &str,
    ) -> Result<(FacilitatorPaymentRequest, Value), Box<dyn std::error::Error>> {
        let fixtures: Value = serde_json::from_str(include_str!(
            "../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
        ))?;
        let case = fixtures["cases"]
            .as_array()
            .ok_or("fixture cases must be an array")?
            .iter()
            .find(|case| case["id"] == case_id)
            .ok_or("recorded transaction must exist")?;
        let reference = "26fde3dedb14662268bff5bb3ab6560550a0dc3938b0c9d5ed717932c75b9839";
        let requirements = PaymentRequirements {
            scheme: "exact".to_owned(),
            network: "layerx:beta".to_owned(),
            amount: AtomicAmount::parse("1000000")?,
            asset: "4e366c1f82bbcb0cfc80b6168346024ca11db454d4b5e93dc7cac57bc5292f28".to_owned(),
            pay_to: "84ddb3e2af095e5f24fbe8a6d3e161eb9ebae5d2292861f3287f14e0db317985".to_owned(),
            max_timeout_seconds: 60,
            extra: None,
        };
        let request = FacilitatorPaymentRequest {
            payment_payload: PaymentPayload {
                x402_version: 2,
                resource: None,
                payload: json!({
                    "layerxTransaction": case["rawTransaction"],
                    "layerxIdempotencyKey": reference,
                }),
                accepted: requirements.clone(),
                extensions: BTreeMap::new(),
            },
            payment_requirements: requirements,
            idempotency_key: parse_hex32(reference)?,
            request_digest: parse_hex32(
                case["receiptJson"]["txHash"]
                    .as_str()
                    .and_then(|hash| hash.strip_prefix("0x"))
                    .ok_or("recorded transaction has a prefixed hash")?,
            )?,
            settlement_step: SettlementStep::Single,
        };
        Ok((request, case["receiptJson"].clone()))
    }

    type X402TestResult<T = ()> = Result<T, Box<dyn std::error::Error>>;

    struct X402RedisProcess {
        child: Child,
        directory: PathBuf,
        url: String,
        certificate: Certificate,
    }

    impl X402RedisProcess {
        fn start() -> X402TestResult<Self> {
            let directory = std::env::temp_dir().join(format!(
                "interop-relay-{}-{}",
                std::process::id(),
                SystemTime::now().duration_since(UNIX_EPOCH)?.as_nanos(),
            ));
            fs::create_dir(&directory)?;
            let pem = directory.join("server.pem");
            let key = directory.join("server.key");
            let output = Command::new("openssl")
                .args([
                    "req",
                    "-x509",
                    "-newkey",
                    "rsa:2048",
                    "-nodes",
                    "-days",
                    "1",
                    "-subj",
                    "/CN=localhost",
                    "-addext",
                    "subjectAltName=DNS:localhost",
                    "-keyout",
                ])
                .arg(&key)
                .arg("-out")
                .arg(&pem)
                .output()?;
            if !output.status.success() {
                return Err(format!(
                    "test certificate: {}",
                    String::from_utf8_lossy(&output.stderr)
                )
                .into());
            }
            let certificate = Certificate::from_pem(&fs::read(&pem)?)?;
            let listener = TcpListener::bind("127.0.0.1:0")?;
            let port = listener.local_addr()?.port();
            let acl = directory.join("users.acl");
            fs::write(
                &acl,
                "user default off\nuser relay on >relay-test-secret ~* &* +@all\n",
            )?;
            let config = directory.join("redis.conf");
            fs::write(&config, format!(
                "bind 127.0.0.1\nport 0\ntls-port {port}\ntls-cert-file {}\ntls-key-file {}\ntls-ca-cert-file {}\ntls-auth-clients no\naclfile {}\nappendonly yes\nappendfsync always\ndir {}\nprotected-mode yes\n",
                pem.display(), key.display(), pem.display(), acl.display(), directory.display(),
            ))?;
            drop(listener);
            let child = Command::new("redis-server")
                .arg(&config)
                .stdout(Stdio::null())
                .stderr(Stdio::null())
                .spawn()?;
            let mut process = Self {
                child,
                directory,
                url: format!("rediss://localhost:{port}"),
                certificate,
            };
            let deadline = Instant::now() + Duration::from_secs(5);
            loop {
                if TcpStream::connect(("127.0.0.1", port)).is_ok() {
                    return Ok(process);
                }
                if let Some(status) = process.child.try_wait()? {
                    return Err(format!("test Redis exited: {status}").into());
                }
                if Instant::now() >= deadline {
                    return Err("test Redis did not start".into());
                }
                thread::sleep(Duration::from_millis(10));
            }
        }

        fn journal(&self) -> Result<RelayJournal, RelayError> {
            RelayJournal::new(
                &self.url,
                self.certificate.clone(),
                Zeroizing::new("relay".to_owned()),
                Zeroizing::new("relay-test-secret".to_owned()),
            )
        }
    }

    impl Drop for X402RedisProcess {
        fn drop(&mut self) {
            let _ = self.child.kill();
            let _ = self.child.wait();
            let _ = fs::remove_dir_all(&self.directory);
        }
    }

    fn x402_settlement_fixture(
        principal: &PrincipalId,
        stable_identity: [u8; 32],
    ) -> X402TestResult<(FacilitatorRequest, Value, Vec<SequencerAuthorization>)> {
        use layerx_interop_gateway::evidence::canonical_receipt_digest;
        use layerx_x402::facilitator::SettlementIdentity;

        fn replace(bytes: &mut [u8], old: &[u8], new: &[u8]) {
            assert_eq!(old.len(), new.len());
            let positions: Vec<_> = bytes
                .windows(old.len())
                .enumerate()
                .filter_map(|(index, value)| (value == old).then_some(index))
                .collect();
            assert_eq!(positions.len(), 1);
            bytes[positions[0]..positions[0] + old.len()].copy_from_slice(new);
        }
        let (recorded, mut receipt_json) = recorded_x402_request("V1-layerxledger-pay-exact")?;
        let mut request = x402_fixture_request(json!({}));
        request.payment_requirements.network = "eip155:125".to_owned();
        request.payment_requirements.asset = recorded.payment_requirements.asset.clone();
        request.payment_requirements.amount = recorded.payment_requirements.amount;
        request
            .payment_requirements
            .extra
            .as_mut()
            .ok_or("profile required")?["layerx"]["commitment"] = json!("receipted");
        request.payment_payload.accepted = request.payment_requirements.clone();
        let identity = SettlementIdentity::derive(
            principal,
            &request,
            stable_identity,
            SettlementStep::Single,
        )?;
        let raw_text = recorded.payment_payload.payload["layerxTransaction"]
            .as_str()
            .ok_or("raw transaction")?;
        let raw = decode_hex(raw_text.strip_prefix("0x").ok_or("raw prefix")?, MAX_BODY)?;
        let (mut transaction, _) = layerx_crypto::evm_transaction::decode_signed(&raw)?;
        let recipient = parse_hex32(&request.payment_requirements.pay_to)?;
        let (old_transfer, _, old_hash) = decode_pay_exact(&raw)?;
        replace(&mut transaction.data, &old_transfer.to, &recipient);
        replace(
            &mut transaction.data,
            &old_transfer.reference,
            &identity.idempotency_key,
        );
        let raw = transaction.sign(&[0x3b; 32])?;
        let (_, sender, tx_hash) = decode_pay_exact(&raw)?;
        let old_sender = decode_hex(
            receipt_json["from"]
                .as_str()
                .ok_or("sender")?
                .strip_prefix("0x")
                .ok_or("sender prefix")?,
            20,
        )?;
        let mut receipt = decode_hex(
            receipt_json["receipt"]
                .as_str()
                .ok_or("receipt")?
                .strip_prefix("0x")
                .ok_or("receipt prefix")?,
            MAX_BODY,
        )?;
        replace(&mut receipt, &old_transfer.to, &recipient);
        replace(
            &mut receipt,
            &old_transfer.reference,
            &identity.idempotency_key,
        );
        replace(&mut receipt, &old_hash, &tx_hash);
        if old_sender != sender {
            replace(&mut receipt, &old_sender, &sender);
        }
        let key = SigningKey::from_bytes(&[0x72; 32]);
        let digest = canonical_receipt_digest(&receipt)?;
        let signature_start = receipt.len() - 64;
        receipt[signature_start..].copy_from_slice(&key.sign(&digest).to_bytes());
        receipt_json["txHash"] = json!(format!("0x{}", hex(&tx_hash)));
        receipt_json["from"] = json!(format!("0x{}", hex(&sender)));
        receipt_json["receipt"] = json!(format!("0x{}", hex(&receipt)));
        receipt_json["logs"][0]["topics"][2] = json!(format!("0x{}", hex(&recipient)));
        receipt_json["logs"][0]["topics"][3] =
            json!(format!("0x{}", hex(&identity.idempotency_key)));
        request.payment_payload.payload = json!({
            "layerxTransaction": format!("0x{}", hex(&raw)),
            "layerxIdempotencyKey": hex(&stable_identity),
        });
        let key_id = parse_hex32(
            receipt_json["keyId"]
                .as_str()
                .ok_or("key id")?
                .strip_prefix("0x")
                .ok_or("key prefix")?,
        )?;
        let keys = vec![SequencerAuthorization::new(
            key_id,
            key.verifying_key().to_bytes(),
            1000,
            1999,
        )];
        Ok((request, receipt_json, keys))
    }

    fn x402_loopback_settlement(
        submitted: bool,
    ) -> X402TestResult<(
        layerx_x402::model::SettlementResponse,
        X402SettlePlane,
        FacilitatorRequest,
    )> {
        let trace = TraceId::mint([0x75; 16]);
        let principal = PrincipalId::new("principal-x402-settle")?;
        let stable_identity = [0x76; 32];
        let (request, receipt, keys) = x402_settlement_fixture(&principal, stable_identity)?;
        let redis = X402RedisProcess::start()?;
        let listener = TcpListener::bind("127.0.0.1:0")?;
        listener.set_nonblocking(true)?;
        let endpoint = layerx_client::sequencer::SequencerEndpoint::new(
            &format!("http://{}", listener.local_addr()?),
            &[],
        )
        .map_err(|error| format!("loopback sequencer endpoint: {error:?}"))?;
        let raw = request.payment_payload.payload["layerxTransaction"].clone();
        let answer = if submitted {
            json!({"rung": "submitted", "txHash": receipt["txHash"], "nonce": receipt["nonce"]})
        } else {
            receipt.clone()
        };
        let server = thread::spawn(move || -> Result<(), String> {
            let deadline = Instant::now() + Duration::from_secs(5);
            for (method, params, result) in [
                (
                    "eth_sendRawTransaction",
                    json!([raw]),
                    receipt["txHash"].clone(),
                ),
                ("pax_sequencerReceipt", json!([receipt["txHash"]]), answer),
            ] {
                let mut stream = loop {
                    match listener.accept() {
                        Ok((stream, _)) => break stream,
                        Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                            if Instant::now() >= deadline {
                                return Err(format!("timed out waiting for {method}"));
                            }
                            thread::sleep(Duration::from_millis(5));
                        }
                        Err(error) => return Err(error.to_string()),
                    }
                };
                stream
                    .set_read_timeout(Some(Duration::from_secs(5)))
                    .map_err(|error| error.to_string())?;
                stream
                    .set_write_timeout(Some(Duration::from_secs(5)))
                    .map_err(|error| error.to_string())?;
                let incoming = http::read_request(&mut stream, 64 * 1024)?;
                let rpc: Value =
                    serde_json::from_slice(&incoming.body).map_err(|error| error.to_string())?;
                assert_eq!(incoming.method, "POST");
                assert_eq!(rpc["jsonrpc"], "2.0");
                assert_eq!(rpc["method"], method);
                assert_eq!(rpc["params"], params);
                http::write_response(
                    &mut stream,
                    &OutgoingResponse {
                        content_type: "application/json".to_owned(),
                        headers: vec![],
                        status: 200,
                        body: serde_json::to_vec(
                            &json!({"jsonrpc": "2.0", "id": rpc["id"], "result": result}),
                        )
                        .map_err(|error| error.to_string())?,
                        retry_after: None,
                    },
                )?;
            }
            Ok(())
        });
        let mut plane = X402SettlePlane {
            relay: ChainRelay {
                endpoint: Arc::new(endpoint),
                receipt_keys: keys,
                journal: redis.journal()?,
            },
            receipt_hex: None,
            activity_id: None,
            unavailable: false,
            observed_at: 1_750_000_000_000,
        };
        let facilitator = Facilitator::new(SupportedResponse {
            kinds: vec![FacilitatorKind {
                x402_version: 2,
                scheme: "exact".to_owned(),
                network: "eip155:125".to_owned(),
                extra: None,
            }],
            extensions: vec![],
            signers: BTreeMap::new(),
        })?;
        let mut gateway = x402_fixture_gateway(&trace);
        let settled = facilitator.settle(
            &mut gateway,
            &principal,
            &request,
            stable_identity,
            SettlementStep::Single,
            &mut plane,
            &trace,
            10,
        );
        server.join().map_err(|_| "loopback sequencer panicked")??;
        let response = settled.map_err(|error| format!("settlement: {error:?}"))?;
        Ok((response, plane, request))
    }

    #[test]
    fn x402_settle_pending_carries_the_transaction_hash() -> X402TestResult {
        let (response, plane, request) = x402_loopback_settlement(true)?;
        let raw = request.payment_payload.payload["layerxTransaction"]
            .as_str()
            .ok_or("raw")?;
        let (_, _, tx_hash) = decode_pay_exact(&decode_hex(
            raw.strip_prefix("0x").ok_or("raw prefix")?,
            MAX_BODY,
        )?)?;
        assert!(!response.success);
        assert_eq!(response.error_reason.as_deref(), Some("settlement_pending"));
        assert_eq!(response.transaction, format!("0x{}", hex(&tx_hash)));
        assert_ne!(
            response.transaction,
            request.payment_payload.payload["layerxIdempotencyKey"]
        );
        assert_ne!(
            response.transaction,
            format!(
                "0x{}",
                request.payment_payload.payload["layerxIdempotencyKey"]
                    .as_str()
                    .ok_or("idempotency")?
            )
        );
        assert_eq!(
            response.extensions["layerx"]["verificationLevel"],
            "submitted"
        );
        assert!(plane.receipt_hex.is_none());
        assert!(plane.activity_id.is_none());
        assert!(!plane.unavailable);
        response.validate_wire()?;
        Ok(())
    }

    #[test]
    fn x402_settle_succeeds_on_the_receipted_rung() -> X402TestResult {
        use layerx_interop_gateway::evidence::canonical_receipt_digest;
        let (response, plane, request) = x402_loopback_settlement(false)?;
        let raw = request.payment_payload.payload["layerxTransaction"]
            .as_str()
            .ok_or("raw")?;
        let (transfer, _, tx_hash) = decode_pay_exact(&decode_hex(
            raw.strip_prefix("0x").ok_or("raw prefix")?,
            MAX_BODY,
        )?)?;
        assert!(response.success);
        assert_eq!(response.error_reason, None);
        assert_eq!(response.transaction, format!("0x{}", hex(&tx_hash)));
        assert_eq!(response.payer, Some(hex(&transfer.from)));
        assert_eq!(response.amount, Some(request.payment_requirements.amount));
        assert_eq!(response.network, "eip155:125");
        assert_eq!(
            response.extensions["layerx"]["verificationLevel"],
            "receipted"
        );
        let receipt_hex = plane.receipt_hex.ok_or("executed receipt required")?;
        let receipt = decode_hex(&receipt_hex, MAX_BODY)?;
        assert_eq!(
            response.extensions["layerx"]["receipt"],
            STANDARD.encode(&receipt)
        );
        assert_eq!(
            response.extensions["layerx"]["receiptDigest"],
            hex(&canonical_receipt_digest(&receipt)?)
        );
        assert_eq!(plane.activity_id, Some(hex(&tx_hash)));
        assert!(!plane.unavailable);
        response.validate_wire()?;
        Ok(())
    }

    #[test]
    fn x402_verify_accepts_the_recorded_pay_exact_transaction(
    ) -> Result<(), Box<dyn std::error::Error>> {
        let (request, receipt) = recorded_x402_request("V1-layerxledger-pay-exact")?;
        let observed_at = 1_750_000_000_000;
        let (transfer, transaction_hash) = verified_x402_transfer(&request, observed_at)?;
        let payer = "1fb0082aa4aa4ce1eb67568a7d8f67798f3b008c254e0929dfe5eabb1dc0d769";
        assert_eq!(transfer.from, parse_hex32(payer)?);
        assert_eq!(
            transfer.asset,
            parse_hex32(&request.payment_requirements.asset)?
        );
        assert_eq!(
            transfer.to,
            parse_hex32(&request.payment_requirements.pay_to)?
        );
        assert_eq!(transfer.amount, 1_000_000);
        assert_eq!(transfer.expires_at, 1_750_003_600_000);
        assert_eq!(transfer.reference, request.idempotency_key);
        assert_eq!(
            transaction_hash,
            parse_hex32(
                receipt["txHash"]
                    .as_str()
                    .and_then(|hash| hash.strip_prefix("0x"))
                    .ok_or("recorded prefixed hash")?,
            )?,
        );
        let trace = TraceId::mint([0x72; 16]);
        let mut plane = X402VerifyPlane {
            observed_at,
            receipt_keys: &[],
        };
        let outcome = plane.verify(&request, &trace)?;
        match outcome {
            PlaneVerifyOutcome::Valid {
                payer: actual,
                extra,
            } => {
                assert_eq!(actual.as_deref(), Some(payer));
                assert_eq!(
                    extra,
                    Some(json!({ "layerxTransactionHash": hex(&transaction_hash) })),
                );
            }
            PlaneVerifyOutcome::Invalid { reason, .. } => {
                panic!("recorded payExact must verify: {reason}");
            }
        }
        for field in ["asset", "pay_to", "amount"] {
            let mut mismatched = request.clone();
            match field {
                "asset" => mismatched.payment_requirements.asset = "44".repeat(32),
                "pay_to" => mismatched.payment_requirements.pay_to = "45".repeat(32),
                "amount" => {
                    mismatched.payment_requirements.amount = AtomicAmount::parse("1000001")?
                }
                _ => unreachable!(),
            }
            assert_eq!(
                verified_x402_transfer(&mismatched, observed_at),
                Err("payment_requirements_mismatch"),
            );
        }
        let mut mismatched = request.clone();
        mismatched.payment_payload.payload["layerxIdempotencyKey"] = json!("00".repeat(32));
        assert_eq!(
            verified_x402_transfer(&mismatched, observed_at),
            Err("protocol_idempotency_mismatch"),
        );
        assert!(verified_x402_transfer(&request, transfer.expires_at).is_ok());
        assert_eq!(
            verified_x402_transfer(&request, transfer.expires_at + 1),
            Err("activity_time_window_refused"),
        );
        Ok(())
    }

    #[test]
    fn x402_verify_refuses_a_transaction_to_another_contract(
    ) -> Result<(), Box<dyn std::error::Error>> {
        let (request, _) = recorded_x402_request("V1-pax-transfer")?;
        assert_eq!(
            verified_x402_transfer(&request, 1_750_000_000_000),
            Err("transaction_target_refused"),
        );
        let mut plane = X402VerifyPlane {
            observed_at: 1_750_000_000_000,
            receipt_keys: &[],
        };
        let outcome = plane.verify(&request, &TraceId::mint([0x73; 16]))?;
        match outcome {
            PlaneVerifyOutcome::Invalid { reason, payer } => {
                assert_eq!(reason, "transaction_target_refused");
                assert_eq!(payer, None);
            }
            PlaneVerifyOutcome::Valid { .. } => {
                panic!("recorded PAX transfer must be refused");
            }
        }
        Ok(())
    }

    #[test]
    fn x402_verify_refuses_unsigned_and_unverifiable_payments_through_the_facilitator() {
        let facilitator = Facilitator::new(SupportedResponse {
            kinds: vec![FacilitatorKind {
                x402_version: 2,
                scheme: "exact".to_owned(),
                network: "layerx:beta".to_owned(),
                extra: None,
            }],
            extensions: vec![],
            signers: BTreeMap::new(),
        })
        .unwrap_or_else(|error| panic!("facilitator supports the layerx exact kind: {error:?}"));
        let trace = TraceId::mint([0x71; 16]);
        let principal = PrincipalId::new("principal-x402-verify")
            .unwrap_or_else(|error| panic!("principal id is valid: {error:?}"));
        let mut plane = X402VerifyPlane {
            observed_at: 10,
            receipt_keys: &[],
        };
        let mut gateway = x402_fixture_gateway(&trace);
        let unsigned = facilitator
            .verify(
                &mut gateway,
                &principal,
                &x402_fixture_request(serde_json::json!({})),
                &mut plane,
                &trace,
                10,
            )
            .unwrap_or_else(|error| {
                panic!(
                    "verify renders a refusal for a payment without a typed transaction: {error:?}"
                )
            });
        assert!(!unsigned.is_valid);
        assert_eq!(
            unsigned.invalid_reason.as_deref(),
            Some("typed_intent_required")
        );
        let unverifiable = facilitator
            .verify(
                &mut gateway,
                &principal,
                &x402_fixture_request(serde_json::json!({
                    "layerxTransaction": "deadbeef",
                    "layerxIdempotencyKey": "00".repeat(32),
                })),
                &mut plane,
                &trace,
                10,
            )
            .unwrap_or_else(|error| {
                panic!("verify renders a refusal for unverifiable transaction bytes: {error:?}")
            });
        assert!(!unverifiable.is_valid);
        assert_eq!(
            unverifiable.invalid_reason.as_deref(),
            Some("transaction_malformed")
        );
    }

    fn ucp_wire(capabilities: Value, payment_handlers: Value) -> UcpRequest {
        let mut wire = serde_json::json!({
            "checkout_id": "checkout-1",
            "currency": "USD",
            "total_minor": "1000",
            "asset": "44".repeat(32),
            "recipient": "45".repeat(32),
            "idempotency_key": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
            "profile_url": "https://client.example/profile",
            "capabilities": [],
            "payment_handlers": [],
            "activity": "00",
            "order_id": "order-1",
            "permalink_url": "https://merchant.example/orders/1"
        });
        wire["capabilities"] = capabilities;
        wire["payment_handlers"] = payment_handlers;
        serde_json::from_value(wire)
            .unwrap_or_else(|error| panic!("UCP wire request parses: {error:?}"))
    }

    #[test]
    fn ucp_negotiation_refuses_incompatible_client_profiles() {
        let platform_handler = PaymentHandler::new(
            "dev.layerx.payment",
            "2026-04-08",
            "https://interchain.paxeer.network/specs/payment-handler",
            "https://interchain.paxeer.network/schemas/payment-handler.json",
        )
        .unwrap_or_else(|error| panic!("platform payment handler is valid: {error:?}"));
        let handler_wire = serde_json::json!([{
            "id": "dev.layerx.payment",
            "version": "2026-04-08",
            "spec": "https://interchain.paxeer.network/specs/payment-handler",
            "schema": "https://interchain.paxeer.network/schemas/payment-handler.json"
        }]);
        let checkout_wire = serde_json::json!([{
            "name": "dev.ucp.shopping.checkout",
            "version": "2026-04-08",
            "spec": "https://ucp.dev/2026-04-08/specification/checkout",
            "schema": "https://ucp.dev/2026-04-08/schemas/shopping/checkout.json"
        }]);
        let without_checkout =
            ucp_client_profile(&ucp_wire(serde_json::json!([]), handler_wire.clone()))
                .unwrap_or_else(|error| {
                    panic!("client profile without checkout parses: {error:?}")
                });
        assert_eq!(
            NegotiatedCapabilities::negotiate(&without_checkout, &platform_handler),
            Err(UcpError::CapabilityUnavailable)
        );
        let foreign_handler_wire = serde_json::json!([{
            "id": "dev.other.payment",
            "version": "2026-04-08",
            "spec": "https://other.example/specs/payment-handler",
            "schema": "https://other.example/schemas/payment-handler.json"
        }]);
        let foreign_handler =
            ucp_client_profile(&ucp_wire(checkout_wire.clone(), foreign_handler_wire))
                .unwrap_or_else(|error| {
                    panic!("client profile with a foreign handler parses: {error:?}")
                });
        assert_eq!(
            NegotiatedCapabilities::negotiate(&foreign_handler, &platform_handler),
            Err(UcpError::PaymentHandlerUnavailable)
        );
        let compatible = ucp_client_profile(&ucp_wire(checkout_wire, handler_wire))
            .unwrap_or_else(|error| panic!("compatible client profile parses: {error:?}"));
        let negotiated = NegotiatedCapabilities::negotiate(&compatible, &platform_handler)
            .unwrap_or_else(|error| panic!("compatible client profile negotiates: {error:?}"));
        assert!(negotiated.checkout());
    }

    #[test]
    fn ap2_request_refuses_caller_clock_audience_and_economic_identity() {
        let mut request = serde_json::json!({
            "checkout_presentation": "checkout",
            "payment_presentation": "payment",
            "nonce": "merchant-issued-nonce",
            "activity": "00"
        });
        assert!(serde_json::from_value::<Ap2Request>(request.clone()).is_ok());
        for field in [
            "now",
            "clock_skew_seconds",
            "audience",
            "currency_minor_exponent",
            "activity_idempotency_key",
        ] {
            request[field] = serde_json::json!(1);
            assert!(serde_json::from_value::<Ap2Request>(request.clone()).is_err());
            request
                .as_object_mut()
                .unwrap_or_else(|| panic!("AP2 request is an object"))
                .remove(field);
        }
    }
}
