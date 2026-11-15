use layerx_platform_gateway::store::{
    Completion, KeyIssuance, KeyRecord, Reservation, ReservationRequest, TapCredentialRecord,
    TapNonceConsumption,
};
use layerx_platform_gateway::{gateway_audit_event, gateway_digest};
use serde_json::Value;
use std::time::{SystemTime, UNIX_EPOCH};

#[path = "support/redis_process.rs"]
mod redis_process;
use redis_process::RedisProcess;

fn record(key_id: &str, principal: &str) -> KeyRecord {
    KeyRecord {
        key_id: key_id.to_owned(),
        principal_digest: principal.to_owned(),
        salt: "audit-key-salt".to_owned(),
        secret_digest: gateway_digest(&[b"audit-key-secret"]),
        signer_public_key: "22".repeat(32),
        scopes: "receipt:read".to_owned(),
        quota_requests: 1,
        quota_window_seconds: 120,
        epoch: 1,
        disabled: false,
    }
}

fn principal_entries(redis: &RedisProcess, principal: &str) -> Vec<Value> {
    redis
        .command(&["XRANGE", "gateway:audit", "-", "+"])
        .as_array()
        .expect("audit entries")
        .iter()
        .filter(|entry| {
            entry[1]
                .as_array()
                .expect("audit fields")
                .chunks_exact(2)
                .any(|pair| {
                    pair[0] == "event"
                        && pair[1]
                            .as_str()
                            .expect("event")
                            .starts_with(&format!("{principal}:"))
                })
        })
        .cloned()
        .collect()
}

fn assert_index(redis: &RedisProcess, principal: &str) -> Vec<Value> {
    let index = format!("gateway:principal:{principal}:audit");
    let entries = principal_entries(redis, principal);
    assert!(
        !entries.is_empty(),
        "principal {principal} must have common audit entries: {}",
        redis.command(&["XRANGE", "gateway:audit", "-", "+"])
    );
    assert_eq!(
        redis.command(&["XRANGE", &index, "-", "+"]),
        Value::Array(entries.clone())
    );
    assert_eq!(redis.command(&["GET", &format!("{index}:version")]), "1");
    entries
}

#[test]
fn principal_audit_index_copies_all_producers_and_idempotent_transitions_once() {
    let mut redis = RedisProcess::start();
    let store = redis.store();
    let principal = "11".repeat(32);
    let other = "99".repeat(32);
    let now = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .expect("clock")
        .as_secs();
    let event =
        |action, outcome| gateway_audit_event(&principal, action, "audit-key", outcome, now);
    let key = record("audit-key", &principal);
    store
        .issue_key(&key, &event("issue", "issued"))
        .expect("issue");
    assert_eq!(
        store
            .issue_key_outcome(&key, &event("issue", "issued"))
            .expect("issue retry"),
        KeyIssuance::Conflict
    );
    store
        .issue_key(
            &record("foreign-key", &other),
            &gateway_audit_event(&other, "issue", "foreign-key", "issued", now),
        )
        .expect("other principal");
    let reserve_event = event("request", "pending");
    let request = ReservationRequest {
        idempotency_scope: "audit-operation",
        request_digest: "request-digest",
        now,
        retention_seconds: 3600,
        activity_id: "audit-activity",
        protocol_idempotency_key: "protocol-key",
        principal_digest: &principal,
        audit_event: &reserve_event,
        continuation: "retained-continuation",
    };
    assert!(matches!(
        store.reserve(&key, request).expect("reserve"),
        Reservation::Reserved
    ));
    assert!(matches!(
        store.reserve(&key, request).expect("reserve retry"),
        Reservation::Existing { .. }
    ));
    let completion_event = event("request", "completed");
    let completion = Completion {
        idempotency_scope: request.idempotency_scope,
        request_digest: request.request_digest,
        state: "completed",
        response_hex: "7b7d",
        receipt_hex: "",
        activity_id: Some(request.activity_id),
        principal_digest: &principal,
        audit_event: &completion_event,
    };
    store.complete(completion).expect("complete");
    store.complete(completion).expect("complete retry");
    assert!(matches!(
        store
            .reserve(
                &key,
                ReservationRequest {
                    idempotency_scope: "limited-operation",
                    activity_id: "limited-activity",
                    ..request
                }
            )
            .expect("limit"),
        Reservation::RateLimited { .. }
    ));
    let replacement = record("replacement-key", &principal);
    store
        .rotate_key(&key, &replacement, &event("rotate", "rotated"))
        .expect("rotate");
    assert_eq!(
        store
            .consume_read(&replacement, now, &event("read", "consumed"))
            .expect("read"),
        None
    );
    assert!(store
        .consume_read(&replacement, now, &event("read", "rate_limited"))
        .expect("limited read")
        .is_some());
    assert!(store
        .revoke_key(&replacement.key_id, &principal, &event("revoke", "revoked"))
        .expect("revoke"));
    assert!(store
        .revoke_key(&replacement.key_id, &principal, &event("revoke", "revoked"))
        .expect("revoke retry"));
    let tap = TapCredentialRecord {
        principal_digest: principal.clone(),
        key_id: "tap-key".to_owned(),
        layerx_agent: "22".repeat(32),
        trusted_agent_id: "trusted-agent".to_owned(),
        trusted_agent_domain: "https://agent.example".to_owned(),
        intent: "pay".to_owned(),
        evidence_digest: "33".repeat(32),
        activity_id: Some("44".repeat(32)),
        signer_public_key: "22".repeat(32),
        target_authority: "shop.example".to_owned(),
        target_path: "/checkout".to_owned(),
        operation_identity: "55".repeat(32),
        credential_expires_at: now + 300,
    };
    assert!(matches!(
        store
            .consume_tap_nonce(
                &tap.key_id,
                "audit-nonce",
                &tap,
                now,
                now + 360,
                &event("tap", "consumed")
            )
            .expect("tap"),
        TapNonceConsumption::Consumed { .. }
    ));
    assert!(matches!(
        store
            .consume_tap_nonce(
                &tap.key_id,
                "audit-nonce",
                &tap,
                now,
                now + 360,
                &event("tap", "existing")
            )
            .expect("tap retry"),
        TapNonceConsumption::AlreadyConsumed { .. }
    ));
    let altered = TapCredentialRecord {
        target_path: "/other".to_owned(),
        ..tap.clone()
    };
    assert_eq!(
        store
            .consume_tap_nonce(
                &tap.key_id,
                "audit-nonce",
                &altered,
                now,
                now + 360,
                &event("tap", "replay")
            )
            .expect("tap replay"),
        TapNonceConsumption::Replay
    );
    let entries = assert_index(&redis, &principal);
    assert_eq!(entries.len(), 11);
    assert_eq!(assert_index(&redis, &other).len(), 1);
    drop(store);
    redis.restart();
    redis
        .store()
        .reconcile_principal_audit_indexes()
        .expect("reconcile current indexes");
    assert_eq!(assert_index(&redis, &principal), entries);
}

#[test]
fn principal_audit_backfill_pages_resume_after_restart_and_do_not_duplicate_entries() {
    let mut redis = RedisProcess::start();
    let principal = "11".repeat(32);
    let other = "99".repeat(32);
    for (digest, key) in [(&principal, "owner-key"), (&other, "other-key")] {
        redis
            .store()
            .issue_key(
                &record(key, digest),
                &gateway_audit_event(digest, "issue", key, "issued", 1),
            )
            .expect("issue");
        let index = format!("gateway:principal:{digest}:audit");
        assert_eq!(
            redis.command(&["DEL", &index, &format!("{index}:version")]),
            2
        );
    }
    // Retained common-stream rows preserve additional fields through recovery.
    let script = "for i=1,600 do local p=ARGV[1]; if i%2==0 then p=ARGV[2] end; redis.call('XADD', KEYS[1], '*', 'event', p .. ':retained-' .. i, 'previous', 'retained-head', 'chain', 'retained-chain', 'outcome', 'completed', 'detail', 'retained') end; return 600";
    assert_eq!(
        redis.command(&["EVAL", script, "1", "gateway:audit", &principal, &other]),
        600
    );
    redis.restart();
    let store = redis.store();
    store
        .reconcile_principal_audit_indexes()
        .expect("recover pages");
    let entries = assert_index(&redis, &principal);
    assert_eq!(entries.len(), 301);
    assert_eq!(assert_index(&redis, &other).len(), 301);
    let cursor = redis.command(&["GET", "gateway:audit:principal-index:cursor"]);
    let last = redis.command(&["XREVRANGE", "gateway:audit", "+", "-", "COUNT", "1"]);
    assert_eq!(cursor, last[0][0]);
    drop(store);
    redis.restart();
    redis
        .store()
        .reconcile_principal_audit_indexes()
        .expect("recovered cursor");
    assert_eq!(assert_index(&redis, &principal), entries);
    assert_eq!(
        redis.command(&["GET", "gateway:audit:principal-index:cursor"]),
        cursor
    );
    let next = redis.command(&[
        "XADD",
        "gateway:audit",
        "*",
        "event",
        &gateway_audit_event(&principal, "read", "receipt", "completed", 2),
        "outcome",
        "completed",
    ]);
    redis
        .store()
        .reconcile_principal_audit_indexes()
        .expect("resume cursor");
    let resumed = assert_index(&redis, &principal);
    assert_eq!(resumed.len(), 302);
    assert_eq!(resumed.last().expect("last entry")[0], next);
    // An index ahead of a retained entry is left intact while its version is set.
    let index = format!("gateway:principal:{principal}:audit");
    assert_eq!(
        redis.command(&[
            "DEL",
            "gateway:audit:principal-index:cursor",
            &format!("{index}:version")
        ]),
        2
    );
    redis
        .store()
        .reconcile_principal_audit_indexes()
        .expect("existing index");
    assert_eq!(assert_index(&redis, &principal), resumed);
}

#[test]
fn principal_audit_backfill_reports_its_stage_and_retains_serving_and_progress() {
    let redis = RedisProcess::start();
    let principal = "11".repeat(32);
    let event = gateway_audit_event(&principal, "read", "receipt", "completed", 1);
    let first = redis.command(&["XADD", "gateway:audit", "*", "event", &event]);
    let invalid = redis.command(&["XADD", "gateway:audit", "*", "outcome", "completed"]);
    let store = redis.store();
    assert_eq!(
        store.reconcile_principal_audit_indexes(),
        Err("decode_entry: missing event".to_owned())
    );
    assert!(store.ready());
    assert_eq!(
        redis.command(&["GET", "gateway:audit:principal-index:cursor"]),
        first
    );
    assert_eq!(assert_index(&redis, &principal).len(), 1);
    assert_eq!(
        redis.command(&[
            "XDEL",
            "gateway:audit",
            invalid.as_str().expect("stream id")
        ]),
        1
    );
    store
        .reconcile_principal_audit_indexes()
        .expect("resume retained cursor");
    assert_eq!(assert_index(&redis, &principal).len(), 1);
}
