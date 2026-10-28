use std::path::PathBuf;
use std::sync::atomic::{AtomicU64, Ordering};

use layerx_agent_api::error::{ErrorClass, RequestId, Retriability};
use layerx_agentd::idempotency::{
    EconomicResult, IdempotencyError, Outcome, RetentionPolicy, Store,
};
use layerx_agentd::store::TenantId;

static NEXT_DIRECTORY: AtomicU64 = AtomicU64::new(1);

struct Directory(PathBuf);

impl Directory {
    fn new(label: &str) -> Self {
        Self(std::env::temp_dir().join(format!(
            "layerx-idempotency-height-{label}-{}-{}",
            std::process::id(),
            NEXT_DIRECTORY.fetch_add(1, Ordering::Relaxed)
        )))
    }

    fn open(&self) -> Store {
        Store::open(
            &self.0,
            TenantId::new("height-tests").unwrap_or_else(|error| panic!("tenant: {error}")),
            RetentionPolicy::new(10, 5).unwrap_or_else(|error| panic!("retention: {error:?}")),
        )
        .unwrap_or_else(|error| panic!("open: {error:?}"))
    }
}

impl Drop for Directory {
    fn drop(&mut self) {
        if self.0.exists() {
            std::fs::remove_dir_all(&self.0)
                .unwrap_or_else(|error| panic!("remove test directory: {error}"));
        }
    }
}

fn result() -> EconomicResult {
    EconomicResult {
        response_bytes: b"original-node-result".to_vec(),
        receipt_ref: Some([0x77; 32]),
    }
}

#[test]
fn sweep_expires_records_older_than_the_retention_window_in_blocks() {
    let directory = Directory::new("sweep");
    let keys = [[1; 32], [2; 32], [3; 32], [4; 32], [5; 32]];
    {
        let store = directory.open();
        store
            .record_attempt(keys[0], b"old", 100, [0x11; 32])
            .unwrap_or_else(|error| panic!("record attempt: {error:?}"));
        store
            .execute(keys[0], b"old", 105, |attempt| {
                assert_eq!(attempt.head_height, 100);
                assert_eq!(attempt.transaction_hash, Some([0x11; 32]));
                assert_eq!(attempt.exact_request_bytes, b"old");
                assert!(attempt.retry);
                Ok(result())
            })
            .unwrap_or_else(|error| panic!("settle old: {error:?}"));
        store
            .execute(keys[1], b"boundary", 101, |_| Ok(result()))
            .unwrap_or_else(|error| panic!("settle boundary: {error:?}"));
        store
            .record_node_result(keys[2], b"node", 100, [0x22; 32], result())
            .unwrap_or_else(|error| panic!("record node result: {error:?}"));
        store
            .record_attempt(keys[3], b"pending", 100, [0x33; 32])
            .unwrap_or_else(|error| panic!("record pending: {error:?}"));
        store
            .execute(keys[4], b"future", u64::MAX - 5, |_| Ok(result()))
            .unwrap_or_else(|error| panic!("settle future: {error:?}"));
        assert_eq!(
            store
                .sweep(99)
                .unwrap_or_else(|error| panic!("regression: {error:?}")),
            0
        );
        assert_eq!(
            store
                .sweep(110)
                .unwrap_or_else(|error| panic!("at window: {error:?}")),
            0
        );
    }
    {
        let store = directory.open();
        assert_eq!(
            store
                .restore(&keys)
                .unwrap_or_else(|error| panic!("restore: {error:?}")),
            5
        );
        assert_eq!(
            store
                .sweep(111)
                .unwrap_or_else(|error| panic!("past window: {error:?}")),
            2
        );
        assert_eq!(
            store
                .sweep(111)
                .unwrap_or_else(|error| panic!("repeat sweep: {error:?}")),
            0
        );
    }
    {
        let store = directory.open();
        assert_eq!(
            store
                .restore(&keys)
                .unwrap_or_else(|error| panic!("restore after expiry: {error:?}")),
            3
        );
        assert_eq!(
            store
                .sweep(112)
                .unwrap_or_else(|error| panic!("expire boundary: {error:?}")),
            1
        );
        assert_eq!(
            store
                .sweep(u64::MAX)
                .unwrap_or_else(|error| panic!("overflow: {error:?}")),
            0
        );
        store
            .execute(keys[3], b"pending", u64::MAX, |attempt| {
                assert!(attempt.retry);
                assert_eq!(attempt.head_height, 100);
                assert_eq!(attempt.transaction_hash, Some([0x33; 32]));
                Ok(result())
            })
            .unwrap_or_else(|error| panic!("pending survives: {error:?}"));
    }
}

#[test]
fn replay_settled_on_the_node_maps_to_idempotent_replay() {
    let directory = Directory::new("node-replay");
    let key = [6; 32];
    {
        let store = directory.open();
        let local_key = [7; 32];
        assert_eq!(
            store
                .execute(local_key, b"local-request", 100, |attempt| {
                    assert!(!attempt.retry);
                    assert_eq!(attempt.head_height, 100);
                    assert_eq!(attempt.transaction_hash, None);
                    Ok(result())
                })
                .unwrap_or_else(|error| panic!("local execution: {error:?}")),
            Outcome::First(result())
        );
        assert_eq!(
            store
                .execute(local_key, b"local-request", 101, |_| {
                    panic!("a locally settled transaction must never be resubmitted")
                })
                .unwrap_or_else(|error| panic!("local replay: {error:?}")),
            Outcome::RepeatedOriginal(result())
        );
        assert!(matches!(
            store.execute(local_key, b"changed-local-request", 101, |_| {
                panic!("conflicting request bytes must never execute")
            }),
            Err(IdempotencyError::Conflict(conflict)) if conflict.key == local_key
        ));
        store
            .record_attempt(key, b"original-request", 100, [0x44; 32])
            .unwrap_or_else(|error| panic!("record attempt: {error:?}"));
        assert!(matches!(
            store.execute(key, b"original-request", 101, |_| Err(
                "transport lost".to_owned()
            )),
            Err(IdempotencyError::Operation(_))
        ));
        store
            .record_node_result(key, b"original-request", 105, [0x44; 32], result())
            .unwrap_or_else(|error| panic!("record resultByIdempotency result: {error:?}"));
    }
    let store = directory.open();
    assert_eq!(
        store
            .restore(&[key])
            .unwrap_or_else(|error| panic!("restore: {error:?}")),
        1
    );
    let repeated = store
        .execute_with_request_id(RequestId(8), key, b"original-request", 106, |_| {
            panic!("a node-settled transaction must never be resubmitted")
        })
        .unwrap_or_else(|error| panic!("repeat original: {error:?}"));
    assert_eq!(repeated, Outcome::RepeatedOriginal(result()));
    let replay = store.execute_with_request_id(RequestId(9), key, b"changed-request", 106, |_| {
        panic!("a node-settled key must not execute different request bytes")
    });
    let Err(IdempotencyError::Api(error)) = replay else {
        panic!("expected node replay refusal, got {replay:?}");
    };
    assert_eq!(error.class, ErrorClass::CoreRejection);
    assert_eq!(error.request_id, RequestId(9));
    assert_eq!(
        error.protocol_result_code.map(|code| code.raw()),
        Some(-302)
    );
    assert_eq!(error.retriability, Retriability::Terminal);
    assert_eq!(error.reason.as_str(), "idempotent_replay");
    assert_ne!(error.class, ErrorClass::IdempotencyConflict);
    assert_eq!(
        store
            .sweep(110)
            .unwrap_or_else(|error| panic!("original height: {error:?}")),
        0
    );
    assert_eq!(
        store
            .sweep(111)
            .unwrap_or_else(|error| panic!("expire node result: {error:?}")),
        1
    );
}
