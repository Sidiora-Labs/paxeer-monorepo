use super::{hex, ok, parse_hex32, refusal, Config, Request, Response};

pub(super) fn proof(config: &Config, transaction: &str) -> Response {
    let Some(id) = parse_hex32(transaction.strip_prefix("0x").unwrap_or(transaction))
        .filter(|id| *id != [0; 32])
    else {
        return refusal(400, "invalid_activity_id", None);
    };
    match config.chain_receipts.lookup_receipt(id) {
        Ok(Some(receipt)) => ok(serde_json::json!({
            "transaction_hash": hex(&receipt.transaction_hash),
            "receipt": hex(&receipt.bytes),
        })
        .to_string()),
        Ok(None) => refusal(404, "receipt_not_found", None),
        Err(_) => refusal(503, "node_unavailable", Some(5)),
    }
}

pub(super) fn route(
    config: &Config,
    request: &Request,
    path: &str,
    query: Option<&str>,
    plane: super::Plane,
) -> Option<Response> {
    if !path.starts_with("/internal/v1/deployment-proof/")
        && !matches!(
            path,
            "/internal/v1/programs/deploy" | "/internal/v1/programs/upgrade"
        )
    {
        return None;
    }
    if plane != super::Plane::Registry {
        return Some(refusal(403, "entitlement_denied", None));
    }
    if query.is_some() {
        return Some(refusal(404, "not_found", None));
    }
    Some(match (request.method.as_str(), path) {
        ("GET", target) if target.starts_with("/internal/v1/deployment-proof/") => {
            proof(config, &target["/internal/v1/deployment-proof/".len()..])
        }
        _ => refusal(404, "not_found", None),
    })
}
