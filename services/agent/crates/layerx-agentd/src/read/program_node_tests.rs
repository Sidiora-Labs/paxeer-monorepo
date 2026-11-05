use super::*;
use layerx_programs::chain_proof::decode::ProofOp;
use serde_json::json;

fn must<T, E: std::fmt::Debug>(value: Result<T, E>) -> T {
    value.unwrap_or_else(|error| panic!("chain head document: {error:?}"))
}

fn chain_document() -> (Value, ChainHeadEvidence) {
    let evidence = ChainHeadEvidence {
        commitment: CommitmentProof {
            signed_header: vec![0xab, 0xcd],
            commit: vec![3, 4],
            validator_set: vec![5, 6],
            store_name: "layerx".to_owned(),
            key: b"program".to_vec(),
            value: Some(vec![7, 8]),
            proof_ops: vec![ProofOp {
                op_type: "ics23:iavl".to_owned(),
                key: b"program".to_vec(),
                data: vec![9, 10],
            }],
        },
        transaction: b"chain head transaction".to_vec(),
        transaction_proof: SimpleProof {
            total: 3,
            index: 1,
            leaf_hash: [0xab; 32],
            aunts: vec![[0xcd; 32], [0xef; 32]],
        },
    };
    let document = json!({ "chain_evidence": {
        "commitment_hex": hex::encode(&must(evidence.commitment.encode())),
        "transaction_hex": hex::encode(&evidence.transaction),
        "transaction_proof": {
            "total": evidence.transaction_proof.total,
            "index": evidence.transaction_proof.index,
            "leaf_hash": hex::encode(&evidence.transaction_proof.leaf_hash),
            "aunts": evidence.transaction_proof.aunts.iter().map(|aunt| hex::encode(aunt)).collect::<Vec<_>>()
        }
    }});
    (document, evidence)
}

#[test]
fn chain_head_documents_preserve_the_commitment_and_transaction_proof() {
    let (document, evidence) = chain_document();
    assert_eq!(must(chain_head_evidence(&document)), evidence);
    let mut without_value = evidence.commitment.clone();
    without_value.value = None;
    let mut document = document;
    document["chain_evidence"]["commitment_hex"] =
        json!(hex::encode(&must(without_value.encode())));
    assert_eq!(
        must(chain_head_evidence(&document)).commitment,
        without_value
    );
}

#[test]
fn chain_head_documents_refuse_non_canonical_encodings() {
    let (document, evidence) = chain_document();
    let mut refused = Vec::new();
    for field in ["commitment_hex", "transaction_hex"] {
        let text = document["chain_evidence"][field].as_str().unwrap();
        for replacement in [text.to_uppercase(), format!("{text}0"), format!("{text}gg")] {
            let mut changed = document.clone();
            changed["chain_evidence"][field] = json!(replacement);
            refused.push(changed);
        }
    }
    let mut trailing = must(evidence.commitment.encode());
    trailing.push(0);
    let mut changed = document.clone();
    changed["chain_evidence"]["commitment_hex"] = json!(hex::encode(&trailing));
    refused.push(changed);
    for replacement in [
        json!([hex::encode(&[0xab; 31])]),
        json!(vec![hex::encode(&[0xab; 32]); 65]),
        json!(["AB".repeat(32)]),
        json!(["a".repeat(63)]),
        json!(["zz".repeat(32)]),
    ] {
        let mut changed = document.clone();
        changed["chain_evidence"]["transaction_proof"]["aunts"] = replacement;
        refused.push(changed);
    }
    for replacement in [
        hex::encode(&[0xab; 31]),
        "AB".repeat(32),
        "a".repeat(63),
        "zz".repeat(32),
    ] {
        let mut changed = document.clone();
        changed["chain_evidence"]["transaction_proof"]["leaf_hash"] = json!(replacement);
        refused.push(changed);
    }
    for field in ["total", "index"] {
        let mut changed = document.clone();
        changed["chain_evidence"]["transaction_proof"][field] = json!(-1);
        refused.push(changed);
    }
    for field in ["commitment_hex", "transaction_hex", "transaction_proof"] {
        let mut changed = document.clone();
        changed["chain_evidence"]
            .as_object_mut()
            .unwrap()
            .remove(field);
        refused.push(changed);
    }
    for field in ["total", "index", "leaf_hash", "aunts"] {
        let mut changed = document.clone();
        changed["chain_evidence"]["transaction_proof"]
            .as_object_mut()
            .unwrap()
            .remove(field);
        refused.push(changed);
    }
    refused.push(json!({}));
    for changed in refused {
        assert_eq!(
            chain_head_evidence(&changed),
            Err(ProgramReadError::NonCanonicalView),
            "{changed}"
        );
    }
}
