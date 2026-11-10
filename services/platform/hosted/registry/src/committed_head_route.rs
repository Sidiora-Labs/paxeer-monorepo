//! Authenticated transport contract shared with the receipt authority listener.

use serde_json::{json, Value};

pub const HEAD_PATH: &str = "/v1/protocol/account-state/head";

/// Mounts the committed head contract before the authority's generic query refusal.
/// The replica remains the independent producer; registry consumers verify its
/// signed chain commitment and transaction inclusion proof against pinned trust.
pub fn route(
    method: &str,
    path: &str,
    query: Option<&str>,
    authenticated: impl FnOnce() -> bool,
    replica_id: [u8; 32],
    fetch: impl FnOnce(&str) -> Result<Value, String>,
) -> Option<(u16, Value)> {
    if path != HEAD_PATH {
        return None;
    }
    let refusal = |status, code| Some((status, json!({"error": {"code": code}})));
    if method != "GET" {
        return refusal(405, "method_not_allowed");
    }
    if !authenticated() {
        return refusal(401, "authentication_required");
    }
    let height = match query {
        None => None,
        Some(query) => match query.strip_prefix("height=").and_then(|height| {
            height
                .parse::<u64>()
                .ok()
                .filter(|value| *value != 0 && value.to_string() == height)
        }) {
            Some(height) => Some(height),
            None => return refusal(400, "invalid_height"),
        },
    };
    let target = height.map_or_else(
        || HEAD_PATH.to_owned(),
        |height| format!("{HEAD_PATH}?height={height}"),
    );
    let document = match fetch(&target) {
        Ok(document) => document,
        Err(_) => return refusal(503, "committed_head_unavailable"),
    };
    let replica = replica_id
        .iter()
        .map(|byte| format!("{byte:02x}"))
        .collect::<String>();
    // Keep the producer identity pinned as well as the committed evidence coordinates.
    if replica_id == [0; 32]
        || document["authority_replica_id"].as_str() != Some(replica.as_str())
        || !document["observed_sequence"]
            .as_u64()
            .is_some_and(|sequence| sequence != 0 && height.is_none_or(|height| height == sequence))
        || document["observed_at"]
            .as_u64()
            .is_none_or(|time| time == 0)
        || (height.is_none() && document["current"] != true)
        || !document["chain_evidence"].is_object()
    {
        return refusal(503, "committed_head_refused");
    }
    Some((200, document))
}
