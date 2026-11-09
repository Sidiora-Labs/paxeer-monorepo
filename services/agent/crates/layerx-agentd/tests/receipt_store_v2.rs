use std::sync::atomic::{AtomicU64, Ordering};

mod support;

use layerx_agentd::receipt::{
    serve_evidence, store_verified_if_absent, AuthenticatedTransactionReceipt, ReceiptStoreError,
};
use layerx_agentd::store::{ObjectKind, Store, TenantId, TenantKey};
use layerx_client::rpc::commit::decode_validators;
use layerx_client::rpc::receipt::{
    lookup_authenticated, lookup_authenticated_with_commit, Lookup, LookupContext,
};
use layerx_client::rpc::receipt_commit::ReceiptCommitEvidence;
use layerx_client::sequencer::decode_receipt_keys;
use layerx_programs::chain_proof::{merkle::SimpleProof, ChainProofVerifier};
use layerx_proof::merkle::{encode_proof, Proof};
use layerx_types::rung::Rung;
use serde_json::{json, Value};

static NEXT_DIRECTORY: AtomicU64 = AtomicU64::new(1);

fn directory() -> std::path::PathBuf {
    std::env::temp_dir().join(format!(
        "layerx-receipt-v2-{}-{}",
        std::process::id(),
        NEXT_DIRECTORY.fetch_add(1, Ordering::Relaxed)
    ))
}

fn tenant(name: &str) -> TenantId {
    TenantId::new(name).unwrap_or_else(|error| panic!("tenant: {error}"))
}

fn evidence_key(tenant: TenantId, hash: [u8; 32]) -> TenantKey {
    TenantKey::new(
        tenant,
        ObjectKind::Configuration,
        [b"receipt-evidence:".as_slice(), &hash].concat(),
    )
    .unwrap_or_else(|error| panic!("key: {error}"))
}

fn must<T, E: std::fmt::Debug>(value: Result<T, E>) -> T {
    value.unwrap_or_else(|error| panic!("receipt evidence: {error:?}"))
}

fn unhex(value: &Value) -> Vec<u8> {
    must(hex::decode(
        value.as_str().unwrap().trim_start_matches("0x"),
    ))
}

fn committed_case() -> Value {
    let recording: Value = must(serde_json::from_str(include_str!(
        "../../layerx-client/tests/fixtures/repair_22_33_1546_node.json"
    )));
    recording["receipt"].clone()
}

fn record(case: &Value, rung: Rung) -> AuthenticatedTransactionReceipt {
    let view = &case["view"];
    let hash: [u8; 32] = must(unhex(&view["txHash"]).try_into());
    let keys = must(decode_receipt_keys(&json!({"keys": [case["key"]]})));
    let mut exchanges = vec![
        (
            "pax_sequencerReceipt",
            json!([view["txHash"]]),
            Some(view.clone()),
        ),
        (
            "eth_getTransactionReceipt",
            json!([view["txHash"]]),
            Some(case["landing"].clone()),
        ),
    ];
    if rung == Rung::Committed {
        let height = case["height"].as_u64().unwrap();
        exchanges.extend([
            (
                "px_getCommit",
                json!([height.to_string()]),
                Some(case["material"].clone()),
            ),
            (
                "px_getValidators",
                json!([height.to_string()]),
                Some(case["validators"].clone()),
            ),
            (
                "eth_getBlockByNumber",
                json!([format!("{height:#x}"), true]),
                Some(case["block"].clone()),
            ),
        ]);
    }
    let (endpoint, server) = support::serve(exchanges);
    let context = LookupContext {
        receipt_keys: &keys,
    };
    let lookup = if rung == Rung::Committed {
        let trust = ChainProofVerifier {
            chain_id: case["material"]["signedHeader"]["header"]["chain_id"]
                .as_str()
                .unwrap()
                .to_owned(),
            validators: must(decode_validators(&case["validators"])),
        };
        let batch = unhex(&case["batch"]);
        let ids: Vec<[u8; 32]> = case["activityIds"]
            .as_array()
            .unwrap()
            .iter()
            .map(|id| must(unhex(id).try_into()))
            .collect();
        let b64 = |value: &Value| must(openssl::base64::decode_block(value.as_str().unwrap()));
        let proof = SimpleProof {
            total: must(case["proof"]["total"].as_str().unwrap().parse()),
            index: must(case["proof"]["index"].as_str().unwrap().parse()),
            leaf_hash: must(b64(&case["proof"]["leaf_hash"]).try_into()),
            aunts: case["proof"]["aunts"]
                .as_array()
                .unwrap()
                .iter()
                .map(|aunt| must(b64(aunt).try_into()))
                .collect(),
        };
        must(lookup_authenticated_with_commit(
            &endpoint,
            hash,
            context,
            &ReceiptCommitEvidence {
                verifier: &trust,
                covering_height: case["height"].as_u64().unwrap(),
                batch_transaction: &batch,
                transaction_proof: &proof,
                activity_ids: &ids,
            },
        ))
    } else {
        must(lookup_authenticated(&endpoint, hash, context))
    };
    must(server.join());
    let Lookup::Verified(verified) = lookup else {
        panic!("authenticated receipt required")
    };
    assert_eq!(verified.rung(), rung);
    must(AuthenticatedTransactionReceipt::from_authenticated(
        &verified,
    ))
}

fn receipted_record(success: bool) -> AuthenticatedTransactionReceipt {
    let recording: Value = must(serde_json::from_str(include_str!(
        "../../layerx-agentd-host/src/repair_22_31_363_receipts.json"
    )));
    let case = &recording[if success { "success" } else { "reverted" }];
    record(
        &json!({"view": case["receiptJson"], "key": case["pinnedKey"], "landing": null}),
        Rung::Receipted,
    )
}

#[test]
fn receipt_evidence_round_trips_by_transaction_hash() {
    let root = directory();
    let owner = tenant("receipt-owner");
    let other = tenant("another-owner");
    let mut durable = Store::open(&root).unwrap_or_else(|error| panic!("store: {error}"));
    let committed = record(&committed_case(), Rung::Committed);
    let receipted = record(&committed_case(), Rung::Receipted);
    for (capability, status, rung) in [
        (receipted_record(true), true, Rung::Receipted),
        (receipted_record(false), false, Rung::Receipted),
        (committed.clone(), true, Rung::Committed),
    ] {
        let expected = capability.evidence().clone();
        let hash = expected.metadata.transaction_hash;
        assert_eq!(expected.metadata.status, status);
        assert_eq!(expected.metadata.rung, rung);
        assert_eq!(
            serve_evidence(&durable, owner.clone(), hash)
                .unwrap_or_else(|error| panic!("{error:?}")),
            None
        );
        let mut submitted = expected.clone();
        submitted.metadata.rung = Rung::Submitted;
        assert!(matches!(
            store_verified_if_absent(
                &mut durable,
                owner.clone(),
                hash,
                &submitted.canonical_receipt,
                &submitted
            ),
            Err(ReceiptStoreError::BelowReceipted)
        ));
        assert!(durable.get(&evidence_key(owner.clone(), hash)).is_none());
        for _ in 0..2 {
            let stored = store_verified_if_absent(
                &mut durable,
                owner.clone(),
                hash,
                &expected.canonical_receipt,
                &capability,
            )
            .unwrap_or_else(|error| panic!("store receipt: {error:?}"));
            assert_eq!(stored, expected);
        }
        if rung == Rung::Committed {
            let weaker = &receipted;
            assert_eq!(
                store_verified_if_absent(
                    &mut durable,
                    owner.clone(),
                    hash,
                    &weaker.evidence().canonical_receipt,
                    weaker
                )
                .unwrap_or_else(|error| panic!("{error:?}")),
                expected
            );
        }
        let key = evidence_key(owner.clone(), hash);
        let encoded = durable
            .get(&key)
            .unwrap_or_else(|| panic!("missing stored evidence"))
            .bytes()
            .to_vec();
        assert_eq!(&encoded[..5], b"LXRE\x02");
        assert_eq!(&encoded[5..10], b"LXRM\x02");
        assert_eq!(
            serve_evidence(&durable, owner.clone(), hash)
                .unwrap_or_else(|error| panic!("{error:?}")),
            Some(expected.clone())
        );
        assert_eq!(
            serve_evidence(&durable, other.clone(), hash)
                .unwrap_or_else(|error| panic!("{error:?}")),
            None
        );
        assert_eq!(
            serve_evidence(&durable, owner.clone(), [0x99; 32])
                .unwrap_or_else(|error| panic!("{error:?}")),
            None
        );

        let mut conflicting = encoded.clone();
        conflicting[59..67].copy_from_slice(&(expected.metadata.gas_used + 1).to_be_bytes());
        must(durable.put_local(key.clone(), conflicting));
        assert!(matches!(
            store_verified_if_absent(
                &mut durable,
                owner.clone(),
                hash,
                &expected.canonical_receipt,
                &capability
            ),
            Err(ReceiptStoreError::Store(_))
        ));
        must(durable.put_local(key.clone(), encoded.clone()));
        assert!(matches!(
            store_verified_if_absent(
                &mut durable,
                owner.clone(),
                [0x99; 32],
                &expected.canonical_receipt,
                &capability
            ),
            Err(ReceiptStoreError::Corrupt)
        ));
        assert!(matches!(
            store_verified_if_absent(
                &mut durable,
                owner.clone(),
                hash,
                b"altered receipt",
                &capability
            ),
            Err(ReceiptStoreError::Corrupt)
        ));
        assert!(matches!(
            store_verified_if_absent(
                &mut durable,
                owner.clone(),
                hash,
                &submitted.canonical_receipt,
                &submitted
            ),
            Err(ReceiptStoreError::BelowReceipted)
        ));
        assert_eq!(
            durable
                .get(&key)
                .unwrap_or_else(|| panic!("missing stored evidence"))
                .bytes(),
            encoded
        );
        drop(durable);
        durable = Store::open(&root).unwrap_or_else(|error| panic!("reopen: {error}"));
        let served = serve_evidence(&durable, owner.clone(), hash)
            .unwrap_or_else(|error| panic!("{error:?}"))
            .unwrap_or_else(|| panic!("missing served evidence"));
        assert_eq!(served, expected);
        assert_eq!(served.canonical_receipt, expected.canonical_receipt);
        assert_eq!(
            durable
                .get(&key)
                .unwrap_or_else(|| panic!("missing stored evidence"))
                .bytes(),
            encoded
        );

        for malformed in [
            encoded[..4].to_vec(),
            encoded[..10].to_vec(),
            encoded[..encoded.len() - 1].to_vec(),
            [encoded.as_slice(), &[0]].concat(),
        ] {
            durable
                .put_local(key.clone(), malformed)
                .unwrap_or_else(|error| panic!("{error:?}"));
            assert!(matches!(
                serve_evidence(&durable, owner.clone(), hash),
                Err(ReceiptStoreError::Corrupt)
            ));
        }
        for (offset, value) in [(10, 0), (58, 2), (388, 3)] {
            let mut malformed = encoded.clone();
            malformed[offset] = value;
            durable
                .put_local(key.clone(), malformed)
                .unwrap_or_else(|error| panic!("{error:?}"));
            assert!(matches!(
                serve_evidence(&durable, owner.clone(), hash),
                Err(ReceiptStoreError::Corrupt)
            ));
        }
        let mut submitted_bytes = encoded.clone();
        submitted_bytes[388] = Rung::Submitted.wire_rank();
        durable
            .put_local(key.clone(), submitted_bytes)
            .unwrap_or_else(|error| panic!("{error:?}"));
        assert!(matches!(
            serve_evidence(&durable, owner.clone(), hash),
            Err(ReceiptStoreError::BelowReceipted)
        ));
        durable
            .put_local(key, encoded)
            .unwrap_or_else(|error| panic!("{error:?}"));
    }
    let owner = tenant("upgrade-owner");
    let hash = receipted.evidence().metadata.transaction_hash;
    assert_eq!(
        must(store_verified_if_absent(
            &mut durable,
            owner.clone(),
            hash,
            &receipted.evidence().canonical_receipt,
            &receipted
        )),
        *receipted.evidence()
    );
    assert_eq!(
        must(store_verified_if_absent(
            &mut durable,
            owner.clone(),
            hash,
            &committed.evidence().canonical_receipt,
            &committed
        )),
        *committed.evidence()
    );
    assert_eq!(
        must(store_verified_if_absent(
            &mut durable,
            owner.clone(),
            hash,
            &receipted.evidence().canonical_receipt,
            &receipted
        )),
        *committed.evidence()
    );
    assert_eq!(
        must(serve_evidence(&durable, owner, hash)),
        Some(committed.evidence().clone())
    );
    drop(durable);
    std::fs::remove_dir_all(root).unwrap_or_else(|error| panic!("{error:?}"));
}

#[test]
fn version_one_receipt_evidence_is_refused() {
    let root = directory();
    let owner = tenant("receipt-owner");
    let capability = receipted_record(true);
    let current = capability.evidence();
    let hash = current.metadata.transaction_hash;
    let mut legacy = b"LXRE\x01".to_vec();
    legacy.extend_from_slice(&hash);
    legacy.extend_from_slice(&[0x66; 32]);
    legacy.extend_from_slice(&9_u64.to_be_bytes());
    let proof = encode_proof(&Proof::new(0, 1, vec![]).unwrap_or_else(|error| panic!("{error:?}")));
    for section in [
        current.canonical_receipt.as_slice(),
        proof.as_slice(),
        b"retained signed header",
    ] {
        legacy.extend_from_slice(
            &u32::try_from(section.len())
                .unwrap_or_else(|error| panic!("{error:?}"))
                .to_be_bytes(),
        );
        legacy.extend_from_slice(section);
    }
    legacy.extend_from_slice(&[0x77; 64]);
    let key = evidence_key(owner.clone(), hash);
    let mut durable = Store::open(&root).unwrap_or_else(|error| panic!("{error:?}"));
    durable
        .put_local(key.clone(), legacy.clone())
        .unwrap_or_else(|error| panic!("{error:?}"));
    drop(durable);
    let mut durable = Store::open(&root).unwrap_or_else(|error| panic!("{error:?}"));
    let error = serve_evidence(&durable, owner.clone(), hash).unwrap_err();
    assert!(matches!(error, ReceiptStoreError::EvidenceVersion));
    assert_eq!(error.reason(), "receipt.evidence_version");
    let error = store_verified_if_absent(
        &mut durable,
        owner.clone(),
        hash,
        &current.canonical_receipt,
        &capability,
    )
    .unwrap_err();
    assert_eq!(error.reason(), "receipt.evidence_version");
    assert_eq!(
        durable
            .get(&key)
            .unwrap_or_else(|| panic!("missing stored evidence"))
            .bytes(),
        legacy
    );
    assert!(durable
        .list_object_ids(&owner, ObjectKind::Receipt)
        .is_empty());
    drop(durable);
    std::fs::remove_dir_all(root).unwrap_or_else(|error| panic!("{error:?}"));
}
