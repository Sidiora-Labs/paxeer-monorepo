use super::rpc::error;
use super::rpc_register::{
    configured_tenant, principal_request, principal_result, register_params, subject, tagged_parts,
    REGISTRATIONS_PER_MINUTE,
};
use super::{Config, IncomingRequest, KernelBackend};
use serde_json::{json, Value};
use std::collections::BTreeSet;
use std::sync::Mutex;

pub(super) const PX_OPEN_SESSION: &str = "px_openSession";
const SESSION_DOMAIN: &[u8] = b"layerx-session-open-v1";
const SESSION_WINDOW_SECONDS: u64 = 120;

#[derive(Clone, Copy, Debug, Eq, Ord, PartialEq, PartialOrd)]
struct AcceptedPair {
    key: [u8; 32],
    time: u64,
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
enum PairAdmission {
    Accepted,
    Replay,
    Limit,
}

static ACCEPTED_PAIRS: Mutex<BTreeSet<AcceptedPair>> = Mutex::new(BTreeSet::new());

fn refusal(id: &Value, code: i32, message: &str, reason: &str) -> Value {
    let mut refused = error(id, code, message);
    refused["error"]["data"] = json!({ "code": reason });
    refused
}

fn canonical_time(value: &str) -> Option<u64> {
    if value.is_empty()
        || (value.len() > 1 && value.starts_with('0'))
        || !value.bytes().all(|byte| byte.is_ascii_digit())
    {
        return None;
    }
    value
        .parse::<u64>()
        .ok()
        .filter(|time| time.to_string() == value)
}

fn session_params(params: Option<&Value>) -> Result<([u8; 32], u64, [u8; 64]), i32> {
    let Some(Value::Array(args)) = params else {
        return Err(-32602);
    };
    let [Value::String(signer_public_key), Value::String(time), Value::String(signature)] =
        args.as_slice()
    else {
        return Err(-32602);
    };
    let (signer_public_key, signature) =
        register_params(Some(&json!([signer_public_key, signature])))?;
    let time = canonical_time(time).ok_or(-32602)?;
    Ok((signer_public_key, time, signature))
}

fn session_digest(tenant: &str, signer_public_key: &[u8; 32], time: u64) -> [u8; 32] {
    let decimal = time.to_string();
    tagged_parts(
        SESSION_DOMAIN,
        &[
            tenant.as_bytes(),
            signer_public_key.as_slice(),
            decimal.as_bytes(),
        ],
    )
}

fn within_span(now: u64, time: u64) -> bool {
    time <= now.saturating_add(SESSION_WINDOW_SECONDS)
        && now <= time.saturating_add(SESSION_WINDOW_SECONDS)
}

fn admit_pair(
    pairs: &mut BTreeSet<AcceptedPair>,
    now: u64,
    key: [u8; 32],
    time: u64,
) -> PairAdmission {
    pairs.retain(|pair| within_span(now, pair.time));
    let pair = AcceptedPair { key, time };
    if pairs.contains(&pair) {
        return PairAdmission::Replay;
    }
    if pairs.len() >= usize::try_from(REGISTRATIONS_PER_MINUTE).unwrap_or(usize::MAX) {
        return PairAdmission::Limit;
    }
    pairs.insert(pair);
    PairAdmission::Accepted
}

fn accept_pair(now: u64, key: [u8; 32], time: u64) -> PairAdmission {
    let Ok(mut pairs) = ACCEPTED_PAIRS.lock() else {
        return PairAdmission::Limit;
    };
    admit_pair(&mut pairs, now, key, time)
}

fn proof_is_valid(tenant: &str, key: &[u8; 32], time: u64, signature: &[u8; 64]) -> bool {
    layerx_crypto::ed25519::verify_digest(key, signature, &session_digest(tenant, key, time))
        .is_ok()
}

fn with_reason(mut answer: Value, reason: &str) -> Value {
    answer["error"]["data"] = json!({ "code": reason });
    answer
}

fn session_result(
    id: &Value,
    tenant: &str,
    sub: &str,
    status: u16,
    content_type: &str,
    body: &[u8],
) -> Value {
    if status != 200 || content_type != "application/json" {
        return refusal(
            id,
            -32603,
            "Invalid upstream session response",
            "identity_session_invalid",
        );
    }
    let Ok(document) = serde_json::from_slice::<Value>(body) else {
        return refusal(
            id,
            -32603,
            "Invalid upstream session response",
            "identity_session_invalid",
        );
    };
    let valid = document["tenant"] == json!(tenant)
        && document["sub"] == json!(sub)
        && document["token"]
            .as_str()
            .is_some_and(|token| !token.is_empty() && token.len() <= 4096)
        && document["csrf_token"]
            .as_str()
            .is_some_and(|csrf| !csrf.is_empty() && csrf.len() <= 4096)
        && document["expires_at"].as_u64().is_some();
    if !valid {
        return refusal(
            id,
            -32603,
            "Invalid upstream session response",
            "identity_session_invalid",
        );
    }
    json!({
        "jsonrpc": "2.0",
        "id": id,
        "result": {
            "tenant": tenant,
            "sub": sub,
            "token": document["token"].as_str().unwrap_or_default(),
            "csrf_token": document["csrf_token"].as_str().unwrap_or_default(),
            "expires_at": document["expires_at"],
        }
    })
}

fn open_session(
    config: &Config,
    _request: &IncomingRequest,
    id: &Value,
    params: Option<&Value>,
) -> Value {
    let (signer_public_key, time, signature) = match session_params(params) {
        Ok(parsed) => parsed,
        Err(_) => return refusal(id, -32602, "Invalid params", "session_params_invalid"),
    };
    let Some(registration) = &config.registration_token else {
        return refusal(
            id,
            -32001,
            "Session unavailable",
            "registration_not_configured",
        );
    };
    let Ok(tenant) = configured_tenant() else {
        return refusal(
            id,
            -32001,
            "Session unavailable",
            "registration_tenant_not_configured",
        );
    };
    let Ok(now) = (config.state_clock)() else {
        return refusal(id, -32001, "Session unavailable", "clock_unavailable");
    };
    if !within_span(now, time) {
        return refusal(id, -32002, "Session refused", "session_time_out_of_range");
    }
    if !proof_is_valid(&tenant, &signer_public_key, time, &signature) {
        return refusal(id, -32002, "Session refused", "session_proof_invalid");
    }
    match accept_pair(now, signer_public_key, time) {
        PairAdmission::Replay => {
            return refusal(id, -32002, "Session refused", "session_proof_replay")
        }
        PairAdmission::Limit => {
            return refusal(id, -32005, "Session unavailable", "session_rate_limit")
        }
        PairAdmission::Accepted => {}
    }
    let (identity, _) = match config.backend(KernelBackend::Identity) {
        Ok(target) => target,
        Err(unavailable) => return unavailable.rpc(id),
    };
    let signer = super::hex(&signer_public_key);
    let sub = subject(&tenant, &signer_public_key);
    let principal = super::upstream_json(
        config,
        identity,
        registration.as_str(),
        "POST",
        "/v1/principals",
        None,
        principal_request(&tenant, &sub, &signer).as_bytes(),
    );
    let principal = match principal {
        Ok(upstream) => principal_result(
            &tenant,
            id,
            &sub,
            &signer,
            upstream.status,
            &upstream.content_type,
            &upstream.body,
        ),
        Err(_) => return refusal(id, -32001, "Session unavailable", "identity_unavailable"),
    };
    if principal.get("error").is_some() {
        return if principal["error"]["code"] == -32603 {
            with_reason(principal, "registration_principal_mismatch")
        } else {
            principal
        };
    }
    let body = json!({ "tenant": tenant, "sub": sub }).to_string();
    let session = match super::upstream_json(
        config,
        identity,
        registration.as_str(),
        "POST",
        "/v1/sessions",
        None,
        body.as_bytes(),
    ) {
        Ok(upstream) => session_result(
            id,
            &tenant,
            &sub,
            upstream.status,
            &upstream.content_type,
            &upstream.body,
        ),
        Err(_) => return refusal(id, -32001, "Session unavailable", "identity_unavailable"),
    };
    session
}

pub(super) fn dispatch(
    config: &Config,
    request: &IncomingRequest,
    method: &str,
    id: &Value,
    params: Option<&Value>,
) -> Option<Value> {
    (method == PX_OPEN_SESSION).then(|| open_session(config, request, id, params))
}

#[cfg(test)]
mod tests {
    use super::*;
    use ed25519_dalek::{Signer as _, SigningKey};

    const TENANT: &str = "beta";

    fn signing_key(seed: u8) -> SigningKey {
        SigningKey::from_bytes(&[seed; 32])
    }

    #[test]
    fn session_digest_is_domain_separated_from_registration() {
        let key = [0xab_u8; 32];
        assert_eq!(
            super::super::hex(&session_digest(TENANT, &key, 1_700_000_000)),
            "03716e9e25f25c00c2fc0b1db517acba61c0dbc17e30463bee7f49733a2069e1"
        );
        assert_ne!(
            session_digest(TENANT, &key, 1_700_000_000),
            super::super::rpc_register::binding(TENANT, &key)
        );
    }

    #[test]
    fn session_span_accepts_both_edges_only() {
        assert!(within_span(1_000, 880));
        assert!(within_span(1_000, 1_120));
        assert!(!within_span(1_000, 879));
        assert!(!within_span(1_000, 1_121));
    }

    #[test]
    fn session_pair_replay_and_bound_are_refused() {
        let mut pairs = BTreeSet::new();
        let key = [7; 32];
        assert_eq!(
            admit_pair(&mut pairs, 1_000, key, 1_000),
            PairAdmission::Accepted
        );
        assert_eq!(
            admit_pair(&mut pairs, 1_000, key, 1_000),
            PairAdmission::Replay
        );
        for index in 1..usize::try_from(REGISTRATIONS_PER_MINUTE).unwrap_or_default() {
            assert_eq!(
                admit_pair(
                    &mut pairs,
                    1_000,
                    [u8::try_from(index + 20).unwrap_or_default(); 32],
                    1_000
                ),
                PairAdmission::Accepted
            );
        }
        assert_eq!(
            admit_pair(&mut pairs, 1_000, [0xff; 32], 1_000),
            PairAdmission::Limit
        );
        assert_eq!(
            admit_pair(&mut pairs, 1_241, key, 1_000),
            PairAdmission::Accepted
        );
    }

    #[test]
    fn session_signature_uses_the_fresh_digest() {
        let key = signing_key(7);
        let public_key = key.verifying_key().to_bytes();
        let signature = key
            .sign(&session_digest(TENANT, &public_key, 42))
            .to_bytes();
        assert!(proof_is_valid(TENANT, &public_key, 42, &signature));
        assert!(!proof_is_valid(TENANT, &public_key, 43, &signature));
    }
}
