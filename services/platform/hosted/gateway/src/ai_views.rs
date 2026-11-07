//! Authenticated AI market views: `GET /v1/ai/markets/{market}/snapshot`,
//! `/participants` and `/epochs` served from the indexer's projection, where finalized means
//! committed in a block and the projection's freshness state is passed through unchanged.
//!
//! Every request re-authenticates the presented gateway key against the durable key store, so
//! a revoked or disabled key loses access on its next request even while it holds a cursor.
//! Rate admission happens before any projection read. Responses carry the exact snapshot
//! binding, its achieved evidence rank and observed age, and typed component statuses.

use layerx_indexer::ai_market::ViewError;
pub use layerx_indexer::ai_views::AiViews;
use layerx_indexer::codec::hex0x;
use serde_json::{json, Value};

use crate::http::{split_target, OutgoingResponse};
use crate::store::RedisStore;
use crate::{authenticate_gateway_key, AccessError};

/// Opens the indexer's durable projection with a persistent cursor signing key.
/// Configuration is all-or-nothing; a restart must never mint a replacement key.
pub fn configured() -> Result<Option<AiViews>, String> {
    const DATABASE: &str = "LAYERX_GATEWAY_AI_PROJECTION_DB";
    const SECRET: &str = "LAYERX_GATEWAY_AI_CURSOR_KEY_FILE";
    const GENERATION: &str = "LAYERX_GATEWAY_AI_CURSOR_GENERATION";
    let values = [DATABASE, SECRET, GENERATION].map(std::env::var_os);
    if values.iter().all(Option::is_none) {
        return Ok(None);
    }
    let path = values[0]
        .as_ref()
        .ok_or("AI projection database is required")?;
    let secret_path = values[1].as_ref().ok_or("AI cursor key file is required")?;
    let generation = values[2]
        .as_ref()
        .and_then(|v| v.to_str())
        .and_then(|v| v.parse::<u32>().ok())
        .filter(|v| *v > 0)
        .ok_or("AI cursor generation must be positive")?;
    let file = std::fs::File::open(secret_path).map_err(|e| e.to_string())?;
    let mut encoded = zeroize::Zeroizing::new(Vec::new());
    std::io::Read::read_to_end(&mut std::io::Read::take(file, 66), &mut encoded)
        .map_err(|e| e.to_string())?;
    let text = std::str::from_utf8(&encoded)
        .map_err(|_| "invalid AI cursor key")?
        .trim_end_matches('\n');
    if text.len() != 64 || !text.bytes().all(|b| b.is_ascii_hexdigit()) {
        return Err("AI cursor key must contain 32 hex bytes".to_owned());
    }
    let mut secret = zeroize::Zeroizing::new([0_u8; 32]);
    for (i, byte) in secret.iter_mut().enumerate() {
        *byte =
            u8::from_str_radix(&text[i * 2..i * 2 + 2], 16).map_err(|_| "invalid AI cursor key")?;
    }
    if secret.iter().all(|b| *b == 0) {
        return Err("AI cursor key must be nonzero".to_owned());
    }
    AiViews::open(std::path::Path::new(path), generation, *secret)
        .map(Some)
        .map_err(|e| format!("AI projection unavailable: {e:?}"))
}

const PREFIX: &str = "/v1/ai/markets/";

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
enum View {
    Snapshot,
    Participants,
    Epochs,
}

fn route(path: &str) -> Option<(&str, View)> {
    let (market, view) = path.strip_prefix(PREFIX)?.split_once('/')?;
    let view = match view {
        "snapshot" => View::Snapshot,
        "participants" => View::Participants,
        "epochs" => View::Epochs,
        _ => return None,
    };
    Some((market, view))
}

/// Answers an AI market view request, or `None` when `target` is not an AI view path.
/// `verified_authority_height` is the independently verified F02 authority height used to
/// label view freshness; `None` labels it unknown.
#[must_use]
pub fn respond(
    views: &AiViews,
    keys: &RedisStore,
    method: &str,
    target: &str,
    authorization: Option<&str>,
    verified_authority_height: Option<u64>,
    now_ms: u64,
) -> Option<OutgoingResponse> {
    let Ok((path, query)) = split_target(target) else {
        return route(target).map(|_| error(&ViewError::InvalidEncoding));
    };
    let (market, view) = route(path)?;
    if method != "GET" {
        return Some(response(
            405,
            &json!({"ok": false, "error": {"code": "method_not_allowed"}}),
        ));
    }
    Some(
        serve(
            views,
            keys,
            (view, market, query),
            authorization,
            verified_authority_height,
            now_ms,
        )
        .unwrap_or_else(|failure| failure),
    )
}

fn serve(
    views: &AiViews,
    keys: &RedisStore,
    (view, market, query): (View, &str, Option<&str>),
    authorization: Option<&str>,
    verified_authority_height: Option<u64>,
    now_ms: u64,
) -> Result<OutgoingResponse, OutgoingResponse> {
    let view = match view {
        View::Snapshot => "snapshot",
        View::Participants => "participants",
        View::Epochs => "epochs",
    };
    views
        .authenticated_query(
            || {
                authenticate_gateway_key(keys, authorization.unwrap_or_default())
                    .map(|record| record.principal_digest)
                    .map_err(|failure| match failure {
                        AccessError::Unauthenticated => response(
                            401,
                            &json!({"ok": false, "error": {"code": "api_key_required"}}),
                        ),
                        AccessError::PersistenceUnavailable => response(
                            503,
                            &json!({"ok": false, "error": {"code": "persistence_unavailable"}}),
                        ),
                    })
            },
            view,
            market,
            query,
            verified_authority_height,
            now_ms,
        )?
        .map(|value| response(200, &json!({"ok": true, "result": value})))
        .map_err(|e| error(&e))
}

fn response(status: u16, value: &Value) -> OutgoingResponse {
    OutgoingResponse {
        content_type: "application/json".to_owned(),
        headers: Vec::new(),
        status,
        body: value.to_string().into_bytes(),
        retry_after: None,
    }
}

/// HTTP status and body for a view failure: the stable category, plus the oldest retained
/// position for a pruned snapshot and the lag for stale authority. Nothing else is revealed.
fn error(failure: &ViewError) -> OutgoingResponse {
    let status = match failure {
        ViewError::InvalidEncoding | ViewError::WrongDomain | ViewError::CursorMismatch => 400,
        ViewError::AccessRefused => 404,
        ViewError::FinalityUnavailable
        | ViewError::RollbackRefused
        | ViewError::AuthorityRegression => 409,
        ViewError::SnapshotPruned { .. } | ViewError::CursorExpired => 410,
        ViewError::ResponseTooLarge => 413,
        ViewError::UnsupportedVersion => 422,
        ViewError::RateLimited => 429,
        ViewError::IntegrityFailure | ViewError::BindingMismatch | ViewError::SnapshotConflict => {
            500
        }
        ViewError::ProjectionUnavailable
        | ViewError::CapacityExceeded
        | ViewError::EndpointDisabled
        | ViewError::Quarantined
        | ViewError::AuthorityStale { .. }
        | ViewError::Store(_) => 503,
    };
    let mut detail = json!({ "code": failure.category() });
    match failure {
        ViewError::SnapshotPruned { oldest } => {
            detail["oldest"] = oldest.map_or(Value::Null, |(id, sequence)| {
                json!({"snapshot_id": hex0x(id.as_bytes()), "observed_sequence": sequence.to_string()})
            });
        }
        ViewError::AuthorityStale { lag } => detail["lag"] = json!(lag.to_string()),
        _ => {}
    }
    let mut out = response(status, &json!({"ok": false, "error": detail}));
    if *failure == ViewError::RateLimited {
        out.retry_after = Some(1);
    }
    out
}
