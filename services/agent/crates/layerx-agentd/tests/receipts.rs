fn require_err<T: std::fmt::Debug, E>(result: Result<T, E>, message: &str) -> E {
    result
        .err()
        .unwrap_or_else(|| panic!("{message}: expected an error"))
}

use std::sync::atomic::{AtomicU64, Ordering};

mod support;

use ed25519_dalek::{Signer as _, SigningKey};
use layerx_agentd::receipt::{
    classify, serve, serve_evidence, store, store_verified_if_absent,
    AuthenticatedTransactionReceipt, ReceiptLookupKey, ReceiptStoreError,
};
use layerx_agentd::store::{ObjectKind, Store, TenantId, TenantKey};
use layerx_client::rpc::receipt::{lookup_authenticated, Lookup, LookupContext};
use layerx_client::sequencer::decode_receipt_keys;
use layerx_proof::receipt::AuthorizedBatch;
use layerx_types::result::{KnownResult, ResultCode, Retriability};
use layerx_types::rung::Rung;
use layerx_types::verify::VerificationLevel;
use layerx_wire::encode::Encoder;
use layerx_wire::hash::receipt_digest;
use serde_json::{json, Value};

static NEXT_DIRECTORY: AtomicU64 = AtomicU64::new(1);

#[derive(Clone)]
struct Fields {
    activity_id: [u8; 32],
    global_sequence: u64,
    previous_state_root: [u8; 32],
    resulting_state_root: [u8; 32],
    batch_id: [u8; 32],
    asset: [u8; 32],
    result_code: i32,
}

fn fields(result_code: i32) -> Fields {
    Fields {
        activity_id: [1; 32],
        global_sequence: 9,
        previous_state_root: [2; 32],
        resulting_state_root: [3; 32],
        batch_id: [4; 32],
        asset: [5; 32],
        result_code,
    }
}

fn encode_fields(fields: &Fields, signature: Option<[u8; 64]>) -> Vec<u8> {
    let successful = fields.result_code == 0;
    let mut encoder = Encoder::new(4096);
    assert_eq!(
        encoder.structure_header_version(0x5201, layerx_wire::limits::PROTOCOL_VERSION),
        Ok(())
    );
    assert_eq!(encoder.u16(layerx_wire::limits::PROTOCOL_VERSION), Ok(()));
    assert_eq!(encoder.bytes(&fields.activity_id, 32), Ok(()));
    assert_eq!(encoder.u64(fields.global_sequence), Ok(()));
    assert_eq!(encoder.bytes(&fields.previous_state_root, 32), Ok(()));
    assert_eq!(encoder.bytes(&fields.resulting_state_root, 32), Ok(()));
    assert_eq!(encoder.bytes(&[8; 32], 32), Ok(()));
    assert_eq!(encoder.i32(fields.result_code), Ok(()));
    assert_eq!(encoder.sequence_length(0, 512), Ok(()));
    assert_eq!(encoder.u128(1), Ok(()));
    assert_eq!(encoder.bytes(&fields.batch_id, 32), Ok(()));
    assert_eq!(encoder.u16(1), Ok(()));
    assert_eq!(encoder.u32(1), Ok(()));
    assert_eq!(encoder.u32(1), Ok(()));
    assert_eq!(encoder.u8(1), Ok(()));
    assert_eq!(encoder.bytes(&fields.asset, 32), Ok(()));
    assert_eq!(encoder.u128(25), Ok(()));
    assert_eq!(encoder.bytes(&[6; 32], 32), Ok(()));
    assert_eq!(encoder.u128(100), Ok(()));
    assert_eq!(encoder.u128(if successful { 75 } else { 100 }), Ok(()));
    assert_eq!(encoder.u64(1), Ok(()));
    assert_eq!(encoder.bytes(&[7; 32], 32), Ok(()));
    assert_eq!(encoder.u128(10), Ok(()));
    assert_eq!(encoder.u128(if successful { 35 } else { 10 }), Ok(()));
    assert_eq!(encoder.bytes(&[9; 32], 32), Ok(()));
    assert_eq!(encoder.bytes(&[10; 32], 32), Ok(()));
    assert_eq!(encoder.bytes(&[11; 32], 32), Ok(()));
    assert_eq!(encoder.u64(1_000), Ok(()));
    assert_eq!(encoder.u8(u8::from(signature.is_some())), Ok(()));
    if let Some(value) = signature {
        assert_eq!(encoder.bytes(&value, 64), Ok(()));
    }
    encoder.finish()
}

fn sign(fields: &Fields, signing_key: &SigningKey) -> Vec<u8> {
    let unsigned = encode_fields(fields, None);
    let digest = receipt_digest(&unsigned)
        .unwrap_or_else(|error| panic!("receipt hashing failed: {error:?}"));
    encode_fields(fields, Some(signing_key.sign(&digest).to_bytes()))
}

fn authorised(fields: &Fields, signing_key: &SigningKey) -> AuthorizedBatch {
    AuthorizedBatch::new(
        fields.batch_id,
        fields.asset,
        fields.previous_state_root,
        fields.resulting_state_root,
        signing_key.verifying_key().to_bytes(),
    )
}

fn directory(label: &str) -> std::path::PathBuf {
    let sequence = NEXT_DIRECTORY.fetch_add(1, Ordering::Relaxed);
    std::env::temp_dir().join(format!(
        "layerx-receipts-{label}-{}-{sequence}",
        std::process::id()
    ))
}

fn tenant() -> TenantId {
    TenantId::new("tenant-a").unwrap_or_else(|error| panic!("tenant: {error}"))
}

fn transaction_receipt() -> AuthenticatedTransactionReceipt {
    let recording: Value = serde_json::from_str(include_str!(
        "../../layerx-agentd-host/src/repair_22_31_363_receipts.json"
    ))
    .unwrap_or_else(|error| panic!("producer recording: {error}"));
    let case = &recording["success"];
    let view = &case["receiptJson"];
    let hash: [u8; 32] = hex::decode(
        view["txHash"]
            .as_str()
            .unwrap_or_else(|| panic!("producer transaction hash required"))
            .trim_start_matches("0x"),
    )
    .unwrap_or_else(|error| panic!("transaction hash: {error}"))
    .try_into()
    .unwrap_or_else(|_| panic!("transaction hash must be 32 bytes"));
    let keys = decode_receipt_keys(&json!({"keys": [case["pinnedKey"]]}))
        .unwrap_or_else(|error| panic!("pinned receipt key: {error:?}"));
    let (endpoint, server) = support::serve(vec![
        (
            "pax_sequencerReceipt",
            json!([view["txHash"]]),
            Some(view.clone()),
        ),
        (
            "eth_getTransactionReceipt",
            json!([view["txHash"]]),
            Some(Value::Null),
        ),
    ]);
    let lookup = lookup_authenticated(
        &endpoint,
        hash,
        LookupContext {
            receipt_keys: &keys,
        },
    )
    .unwrap_or_else(|error| panic!("authenticate producer receipt: {error:?}"));
    let traffic = server
        .join()
        .unwrap_or_else(|error| panic!("receipt transport: {error:?}"));
    assert_eq!(traffic.lookups, 1);
    let Lookup::Verified(verified) = lookup else {
        panic!("authenticated receipt required")
    };
    assert_eq!(verified.rung(), Rung::Receipted);
    let capability = AuthenticatedTransactionReceipt::from_authenticated(&verified)
        .unwrap_or_else(|error| panic!("receipt capability: {error:?}"));
    assert_eq!(capability.evidence().metadata.transaction_hash, hash);
    assert_eq!(
        capability.evidence().canonical_receipt,
        verified.canonical_bytes()
    );
    capability
}

fn transaction_keys(owner: &TenantId, hash: [u8; 32]) -> [TenantKey; 2] {
    [
        (ObjectKind::Receipt, b"receipt:transaction:".as_slice()),
        (ObjectKind::Configuration, b"receipt-evidence:".as_slice()),
    ]
    .map(|(kind, prefix)| {
        TenantKey::new(owner.clone(), kind, [prefix, &hash].concat())
            .unwrap_or_else(|error| panic!("transaction key: {error}"))
    })
}

fn transaction_bytes(durable: &Store, keys: &[TenantKey; 2]) -> [Option<Vec<u8>>; 2] {
    keys.each_ref()
        .map(|key| durable.get(key).map(|value| value.bytes().to_vec()))
}

#[test]
fn verified_bytes_survive_all_three_indexes_and_restart_unchanged() {
    let root = directory("indexes");
    let signing_key = SigningKey::from_bytes(&[3; 32]);
    let fields = fields(0);
    let exact = sign(&fields, &signing_key);
    let idem = [0x44; 32];
    let mut durable = Store::open(&root).unwrap_or_else(|error| panic!("store: {error}"));
    let metadata = store(
        &mut durable,
        tenant(),
        idem,
        &exact,
        &authorised(&fields, &signing_key),
    )
    .unwrap_or_else(|error| panic!("record: {error:?}"));
    assert_eq!(
        metadata.verification_level,
        VerificationLevel::SEQUENCER_SIGNED
    );
    assert_eq!(metadata.result.code.raw(), 0);
    drop(durable);

    let reopened = Store::open(&root).unwrap_or_else(|error| panic!("reopen: {error}"));
    for lookup in [
        ReceiptLookupKey::Activity(fields.activity_id),
        ReceiptLookupKey::Idempotency(idem),
        ReceiptLookupKey::GlobalSequence(fields.global_sequence),
    ] {
        let served =
            serve(&reopened, tenant(), lookup).unwrap_or_else(|error| panic!("serve: {error:?}"));
        assert_eq!(served.canonical_bytes, exact);
        assert_eq!(served.metadata, metadata);
    }
    let _ = std::fs::remove_dir_all(root);
}

#[test]
fn first_verified_ingress_is_stored_before_it_is_served() {
    let root = directory("first-ingress");
    let capability = transaction_receipt();
    let record = capability.evidence();
    let hash = record.metadata.transaction_hash;
    let keys = transaction_keys(&tenant(), hash);
    let other = TenantId::new("tenant-b").unwrap_or_else(|error| panic!("tenant: {error}"));
    let other_keys = transaction_keys(&other.clone(), hash);
    let mut durable = Store::open(&root).unwrap_or_else(|error| panic!("store: {error}"));
    assert_eq!(
        serve_evidence(&durable, tenant(), hash).unwrap_or_else(|error| panic!("{error:?}")),
        None
    );
    assert_eq!(transaction_bytes(&durable, &keys), [None, None]);
    let served = store_verified_if_absent(
        &mut durable,
        tenant(),
        hash,
        &record.canonical_receipt,
        &capability,
    )
    .unwrap_or_else(|error| panic!("first ingress: {error:?}"));
    assert_eq!(served, *record);
    assert_eq!(served.metadata.rung, Rung::Receipted);
    assert_eq!(
        serve_evidence(&durable, tenant(), hash).unwrap_or_else(|error| panic!("{error:?}")),
        Some(record.clone())
    );
    let stored = transaction_bytes(&durable, &keys);
    assert_eq!(
        stored[0].as_deref(),
        Some(record.canonical_receipt.as_slice())
    );
    assert!(stored[1].is_some());
    assert_eq!(transaction_bytes(&durable, &other_keys), [None, None]);
    drop(durable);

    let reopened = Store::open(&root).unwrap_or_else(|error| panic!("reopen: {error}"));
    assert_eq!(transaction_bytes(&reopened, &keys), stored);
    assert_eq!(
        serve_evidence(&reopened, tenant(), hash).unwrap_or_else(|error| panic!("{error:?}")),
        Some(record.clone())
    );
    assert_eq!(
        serve_evidence(&reopened, other, hash).unwrap_or_else(|error| panic!("{error:?}")),
        None
    );
    assert_eq!(transaction_bytes(&reopened, &other_keys), [None, None]);
    drop(reopened);
    std::fs::remove_dir_all(root).unwrap_or_else(|error| panic!("cleanup: {error}"));
}

#[test]
fn repeated_verified_ingress_refuses_conflicts_without_replacing_the_first_receipt() {
    let root = directory("ingress-conflict");
    let capability = transaction_receipt();
    let original = capability.evidence();
    let hash = original.metadata.transaction_hash;
    let keys = transaction_keys(&tenant(), hash);
    let mut durable = Store::open(&root).unwrap_or_else(|error| panic!("store: {error}"));
    for _ in 0..2 {
        assert_eq!(
            store_verified_if_absent(
                &mut durable,
                tenant(),
                hash,
                &original.canonical_receipt,
                &capability,
            )
            .unwrap_or_else(|error| panic!("authenticated ingress: {error:?}")),
            *original
        );
    }
    let first = transaction_bytes(&durable, &keys);
    let mut conflicting = first[1]
        .clone()
        .unwrap_or_else(|| panic!("missing evidence"));
    conflicting[59..67].copy_from_slice(&(original.metadata.gas_used + 1).to_be_bytes());
    durable
        .put_local(keys[1].clone(), conflicting.clone())
        .unwrap_or_else(|error| panic!("seed conflicting durable record: {error}"));
    let mut conflicting_evidence = original.clone();
    conflicting_evidence.metadata.gas_used += 1;
    assert!(matches!(
        store_verified_if_absent(
            &mut durable,
            tenant(),
            hash,
            &original.canonical_receipt,
            &capability,
        ),
        Err(ReceiptStoreError::Store(_))
    ));
    assert_eq!(
        transaction_bytes(&durable, &keys),
        [first[0].clone(), Some(conflicting.clone())]
    );
    drop(durable);

    let mut reopened = Store::open(&root).unwrap_or_else(|error| panic!("reopen: {error}"));
    assert_eq!(
        serve_evidence(&reopened, tenant(), hash)
            .unwrap_or_else(|error| panic!("serve conflict: {error:?}")),
        Some(conflicting_evidence)
    );
    assert_eq!(
        transaction_bytes(&reopened, &keys),
        [first[0].clone(), Some(conflicting)]
    );
    reopened
        .put_local(
            keys[1].clone(),
            first[1]
                .clone()
                .unwrap_or_else(|| panic!("missing first evidence")),
        )
        .unwrap_or_else(|error| panic!("restore first evidence: {error}"));
    assert_eq!(
        store_verified_if_absent(
            &mut reopened,
            tenant(),
            hash,
            &original.canonical_receipt,
            &capability
        )
        .unwrap_or_else(|error| panic!("repeat after reopen: {error:?}")),
        *original
    );
    assert_eq!(transaction_bytes(&reopened, &keys), first);
    assert_eq!(
        serve_evidence(&reopened, tenant(), hash)
            .unwrap_or_else(|error| panic!("serve original: {error:?}")),
        Some(original.clone())
    );
    drop(reopened);
    std::fs::remove_dir_all(root).unwrap_or_else(|error| panic!("cleanup: {error}"));
}

#[test]
fn raw_transaction_evidence_is_refused_without_writing() {
    let root = directory("raw-ingress");
    let capability = transaction_receipt();
    let record = capability.evidence();
    let hash = record.metadata.transaction_hash;
    let keys = transaction_keys(&tenant(), hash);
    let other = TenantId::new("tenant-b").unwrap_or_else(|error| panic!("tenant: {error}"));
    let other_keys = transaction_keys(&other.clone(), hash);
    let mut durable = Store::open(&root).unwrap_or_else(|error| panic!("store: {error}"));
    let mut authenticated_bytes = None;
    for stage in 0..3 {
        let before = transaction_bytes(&durable, &keys);
        if stage == 0 {
            assert_eq!(before, [None, None]);
        } else {
            assert_eq!(Some(before.clone()), authenticated_bytes);
        }
        for rung in [Rung::Receipted, Rung::Committed] {
            let mut raw = record.clone();
            raw.metadata.rung = rung;
            for (owner, owner_keys) in [(tenant(), &keys), (other.clone(), &other_keys)] {
                let unchanged = transaction_bytes(&durable, owner_keys);
                let error = require_err(
                    store_verified_if_absent(
                        &mut durable,
                        owner,
                        hash,
                        &raw.canonical_receipt,
                        &raw,
                    ),
                    "required test value",
                );
                assert!(matches!(error, ReceiptStoreError::UnauthenticatedIngress));
                assert_eq!(error.reason(), "receipt.unauthenticated_ingress");
                assert_eq!(transaction_bytes(&durable, owner_keys), unchanged);
            }
        }
        assert_eq!(transaction_bytes(&durable, &keys), before);
        assert_eq!(transaction_bytes(&durable, &other_keys), [None, None]);
        assert_eq!(
            serve_evidence(&durable, tenant(), hash)
                .unwrap_or_else(|error| panic!("serve: {error:?}")),
            if stage == 0 {
                None
            } else {
                Some(record.clone())
            }
        );
        assert_eq!(
            serve_evidence(&durable, other.clone(), hash)
                .unwrap_or_else(|error| panic!("serve other tenant: {error:?}")),
            None
        );
        if stage == 0 {
            assert_eq!(
                store_verified_if_absent(
                    &mut durable,
                    tenant(),
                    hash,
                    &record.canonical_receipt,
                    &capability
                )
                .unwrap_or_else(|error| panic!("valid ingress: {error:?}")),
                *record
            );
            authenticated_bytes = Some(transaction_bytes(&durable, &keys));
        } else if stage == 1 {
            drop(durable);
            durable = Store::open(&root).unwrap_or_else(|error| panic!("reopen: {error}"));
        }
    }
    drop(durable);
    std::fs::remove_dir_all(root).unwrap_or_else(|error| panic!("cleanup: {error}"));
}

#[test]
fn rejected_receipt_preserves_exact_code_and_canonical_taxonomy() {
    let root = directory("rejection");
    let signing_key = SigningKey::from_bytes(&[3; 32]);
    let fields = fields(KnownResult::BadSignature.raw());
    let exact = sign(&fields, &signing_key);
    let mut durable = Store::open(&root).unwrap_or_else(|error| panic!("store: {error}"));
    let metadata = store(
        &mut durable,
        tenant(),
        [0x55; 32],
        &exact,
        &authorised(&fields, &signing_key),
    )
    .unwrap_or_else(|error| panic!("record rejection: {error:?}"));
    assert_eq!(metadata.result.code.raw(), KnownResult::BadSignature.raw());
    assert_eq!(metadata.result.canonical, Some(KnownResult::BadSignature));
    assert_eq!(metadata.result.retriability, Retriability::Terminal);
    assert!(!metadata.result.retry_permitted);
    let served = serve(
        &durable,
        tenant(),
        ReceiptLookupKey::Idempotency([0x55; 32]),
    )
    .unwrap_or_else(|error| panic!("serve rejection: {error:?}"));
    assert_eq!(served.canonical_bytes, exact);
    assert_eq!(served.metadata.result.code.raw(), -201);
    let _ = std::fs::remove_dir_all(root);
}

#[test]
fn every_terminal_protocol_code_class_is_never_retried() {
    for known in KnownResult::ALL {
        let result = classify(ResultCode::from(*known));
        assert_eq!(result.code.raw(), known.raw());
        assert_eq!(result.canonical, Some(*known));
        if known.retriability() == Retriability::Terminal {
            assert!(!result.retry_permitted, "terminal code {known:?} retried");
        } else {
            assert!(result.retry_permitted, "retriable code {known:?} blocked");
        }
    }
    let future_unknown = classify(ResultCode::from_raw(-7_777));
    assert_eq!(future_unknown.code.raw(), -7_777);
    assert_eq!(future_unknown.canonical, None);
    assert!(!future_unknown.retry_permitted);
}

#[test]
fn unverifiable_or_conflicting_receipts_are_never_recorded() {
    let root = directory("invalid");
    let signing_key = SigningKey::from_bytes(&[3; 32]);
    let fields = fields(0);
    let exact = sign(&fields, &signing_key);
    let mut altered = exact.clone();
    let last = altered.len() - 1;
    altered[last] ^= 1;
    let mut durable = Store::open(&root).unwrap_or_else(|error| panic!("store: {error}"));
    assert!(matches!(
        store(
            &mut durable,
            tenant(),
            [0x66; 32],
            &altered,
            &authorised(&fields, &signing_key),
        ),
        Err(ReceiptStoreError::Verification(_))
    ));
    assert!(matches!(
        serve(
            &durable,
            tenant(),
            ReceiptLookupKey::Idempotency([0x66; 32])
        ),
        Err(ReceiptStoreError::Missing)
    ));

    store(
        &mut durable,
        tenant(),
        [0x66; 32],
        &exact,
        &authorised(&fields, &signing_key),
    )
    .unwrap_or_else(|error| panic!("first record: {error:?}"));
    let mut other = fields.clone();
    other.activity_id = [9; 32];
    let other_exact = sign(&other, &signing_key);
    assert!(matches!(
        store(
            &mut durable,
            tenant(),
            [0x66; 32],
            &other_exact,
            &authorised(&other, &signing_key),
        ),
        Err(ReceiptStoreError::Store(_))
    ));
    let _ = std::fs::remove_dir_all(root);
}
