use layerx_platform_gateway::http::{self, Client, Endpoint};
use serde::Deserialize;

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct AuthorityReadinessResponse {
    ready: bool,
    network_id: String,
    protocol_network_id: u32,
    wire_version: String,
}

#[derive(Debug, PartialEq, Eq)]
pub(crate) enum AuthorityUnready {
    Transport,
    InvalidSchema,
    IdentityMismatch,
    Unavailable,
}

pub(crate) fn decode_authority_readiness(
    status: u16,
    content_type: &str,
    body: &[u8],
    network_id: &str,
    protocol_network_id: u32,
    wire_version: &str,
) -> Result<(), AuthorityUnready> {
    let readiness: AuthorityReadinessResponse =
        serde_json::from_slice(body).map_err(|_| AuthorityUnready::InvalidSchema)?;
    if readiness.network_id != network_id
        || readiness.protocol_network_id != protocol_network_id
        || readiness.wire_version != wire_version
    {
        return Err(AuthorityUnready::IdentityMismatch);
    }
    if status != 200 || content_type != "application/json" || !readiness.ready {
        return Err(AuthorityUnready::Unavailable);
    }
    Ok(())
}

pub(crate) fn probe_authority_readiness(
    client: &Client,
    endpoint: &Endpoint,
    token: &str,
    network_id: &str,
    protocol_network_id: u32,
    wire_version: &str,
) -> Result<(), AuthorityUnready> {
    let upstream = client
        .request(
            endpoint,
            token,
            &http::OutboundRequest {
                method: "GET",
                path: "/readyz",
                idempotency: None,
                content_type: "application/json",
                body: &[],
            },
        )
        .map_err(|_| AuthorityUnready::Transport)?;
    decode_authority_readiness(
        upstream.status,
        &upstream.content_type,
        &upstream.body,
        network_id,
        protocol_network_id,
        wire_version,
    )
}
