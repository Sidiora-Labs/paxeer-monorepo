//! Authenticated endpoint routes shared by the listener and cursor-path qualification.
use crate::error::WebhookError;
use crate::events::{EndpointId, Principal};
use crate::hosted::HostedService;
use crate::http::{Reply, Request};
use serde::{Deserialize, Serialize};
const DEFAULT_PAGE: usize = 50;
#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct SuspendBody {
    reason: String,
}

pub fn refusal(error: &WebhookError) -> Reply {
    let (status, code, retry) = match error {
        WebhookError::InvalidRequest => (400, "invalid_request", None),
        WebhookError::UnknownEndpoint => (404, "unknown_endpoint", None),
        WebhookError::UnknownDelivery => (404, "unknown_delivery", None),
        WebhookError::NotDeadLettered => (409, "not_dead_lettered", None),
        WebhookError::EndpointSuspended => (409, "endpoint_suspended", None),
        WebhookError::EventConflict => (409, "conflict", None),
        WebhookError::OrderViolation => (409, "order_violation", None),
        WebhookError::InvalidCursor => (400, "invalid_cursor", None),
        WebhookError::CursorExpired => (410, "cursor_expired", None),
        WebhookError::VerificationRequired => (422, "verification_required", None),
        WebhookError::SignatureRejected => (401, "signature_rejected", None),
        WebhookError::ReplayRejected => (409, "replay_rejected", None),
        WebhookError::StaleTimestamp => (400, "stale_timestamp", None),
        WebhookError::ReplayCapacity => (503, "replay_capacity", Some(10)),
        WebhookError::Entropy => (503, "entropy_unavailable", Some(5)),
        WebhookError::CorruptStore | WebhookError::Unavailable | WebhookError::Io(_) => {
            (503, "dependency_unavailable", Some(5))
        }
        WebhookError::Gateway(_) => (422, "verification_refused", None),
    };
    Reply::refusal(status, code, retry)
}

pub fn encoded<T: Serialize>(status: u16, value: &T) -> Reply {
    serde_json::to_string(value).map_or_else(
        |_| Reply::refusal(503, "encoding_failed", Some(5)),
        |body| Reply::json(status, body),
    )
}

pub fn body<T: for<'de> Deserialize<'de>>(request: &Request) -> Result<T, WebhookError> {
    serde_json::from_slice(&request.body).map_err(|_| WebhookError::InvalidRequest)
}

pub fn page(request: &Request) -> usize {
    request
        .parameter("limit")
        .and_then(|value| value.parse::<usize>().ok())
        .unwrap_or(DEFAULT_PAGE)
        .clamp(1, 200)
}

pub fn endpoint_route(
    service: &HostedService,
    request: &Request,
    principal: &Principal,
    endpoint: &str,
    action: &str,
    at: u64,
) -> Reply {
    let endpoint = match EndpointId::new(endpoint) {
        Ok(value) => value,
        Err(error) => return refusal(&error),
    };
    match (request.method.as_str(), action) {
        ("GET", "events") => service
            .events_since(
                principal,
                &endpoint,
                request.parameter("cursor"),
                page(request),
            )
            .map_or_else(|error| refusal(&error), |value| encoded(200, &value)),
        ("GET", "keys") => service
            .signing_keys(principal, &endpoint, at)
            .map_or_else(|error| refusal(&error), |value| encoded(200, &value)),
        ("POST", "keys") => match request.header("idempotency-key") {
            Some(idempotency) => service
                .rotate_key(principal, &endpoint, idempotency, at)
                .map_or_else(|error| refusal(&error), |value| encoded(201, &value)),
            None => Reply::refusal(400, "idempotency_key_required", None),
        },
        ("POST", "redeliveries") => {
            let Some(idempotency) = request.header("idempotency-key") else {
                return Reply::refusal(400, "idempotency_key_required", None);
            };
            service
                .redeliver(
                    principal,
                    &endpoint,
                    request.parameter("cursor"),
                    page(request),
                    idempotency,
                    at,
                )
                .map_or_else(|error| refusal(&error), |value| encoded(202, &value))
        }
        ("POST", "suspensions") => body::<SuspendBody>(request)
            .and_then(|body| service.suspend(principal, &endpoint, &body.reason, at))
            .map_or_else(
                |error| refusal(&error),
                |()| Reply::json(200, "{\"suspended\":true}".to_owned()),
            ),
        ("POST", "resumptions") => service.resume(principal, &endpoint).map_or_else(
            |error| refusal(&error),
            |()| Reply::json(200, "{\"suspended\":false}".to_owned()),
        ),
        _ => Reply::refusal(404, "not_found", None),
    }
}
