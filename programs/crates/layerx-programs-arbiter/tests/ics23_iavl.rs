use layerx_programs_arbiter::ics23::{decode_existence, verify_existence, ProofError};
use serde_json::Value;

fn vectors() -> Vec<Value> {
    let fixture: Value = serde_json::from_str(include_str!(
        "../../../tests/vectors/ics23-iavl-existence.json"
    ))
    .expect("Go ICS23 fixture JSON");
    fixture["entries"].as_array().expect("four entries").clone()
}

fn hex(entry: &Value, name: &str) -> Vec<u8> {
    let text = entry[name].as_str().expect("hex field");
    assert_eq!(text.len() % 2, 0);
    (0..text.len())
        .step_by(2)
        .map(|i| u8::from_str_radix(&text[i..i + 2], 16).expect("hex byte"))
        .collect()
}

#[test]
fn iavl_existence_proof_matches_go_ics23_vector() {
    let entries = vectors();
    assert_eq!(entries.len(), 4);
    let mut left = false;
    let mut right = false;
    let mut long_key = false;
    for entry in &entries {
        let key = hex(entry, "key");
        let value = hex(entry, "value");
        let root: [u8; 32] = hex(entry, "root").try_into().expect("SHA256 root");
        let proof = decode_existence(&hex(entry, "proof")).expect("Go existence protobuf");
        assert_eq!(proof.key, key);
        assert_eq!(proof.value, value);
        assert_eq!(key[0], 1);
        let module = u16::from_be_bytes([key[1], key[2]]);
        assert_eq!(
            u64::from(module),
            entry["module"].as_u64().expect("module id")
        );
        assert_eq!(&key[3..], hex(entry, "module_key"));
        assert_eq!(proof.leaf.prefix, [0, 2, 0x80, 2]);
        long_key |= key.len() >= 128;
        assert!(!proof.path.is_empty());
        for op in &proof.path {
            left |= op.suffix.len() == 33;
            right |= op.suffix.is_empty();
        }
        assert_eq!(verify_existence(&proof, root, &key, &value), Ok(()));
    }
    assert!(left && right, "exercise both IAVL child positions");
    assert!(long_key, "exercise multibyte protobuf key lengths");
}

#[test]
fn iavl_existence_proof_refuses_tampered_value() {
    for entry in vectors() {
        let key = hex(&entry, "key");
        let value = hex(&entry, "value");
        let root: [u8; 32] = hex(&entry, "root").try_into().expect("SHA256 root");
        let encoded = hex(&entry, "proof");
        let proof = decode_existence(&encoded).expect("Go existence protobuf");
        let mut bad_value = value.clone();
        bad_value[0] ^= 1;
        assert_eq!(
            verify_existence(&proof, root, &key, &bad_value),
            Err(ProofError::RootMismatch)
        );
        let mut bad_key = key.clone();
        bad_key[0] ^= 1;
        assert_eq!(
            verify_existence(&proof, root, &bad_key, &value),
            Err(ProofError::RootMismatch)
        );
        let mut bad = proof.clone();
        bad.value[0] ^= 1;
        assert_eq!(
            verify_existence(&bad, root, &key, &bad_value),
            Err(ProofError::RootMismatch)
        );
        let mut bad = proof.clone();
        bad.key[0] ^= 1;
        assert_eq!(
            verify_existence(&bad, root, &bad_key, &value),
            Err(ProofError::RootMismatch)
        );
        let mut bad = proof.clone();
        let inner = &mut bad.path[0];
        if inner.suffix.is_empty() {
            let sibling_byte = inner.prefix.len() - 2;
            inner.prefix[sibling_byte] ^= 1;
        } else {
            inner.suffix[1] ^= 1;
        }
        assert_eq!(
            verify_existence(&bad, root, &key, &value),
            Err(ProofError::RootMismatch)
        );
        let mut bad_root = root;
        bad_root[0] ^= 1;
        assert_eq!(
            verify_existence(&proof, bad_root, &key, &value),
            Err(ProofError::RootMismatch)
        );
        for hash in [0, 2, 3, 4, 5, 6] {
            let mut bad = proof.clone();
            bad.leaf.hash = hash;
            assert_eq!(
                verify_existence(&bad, root, &key, &value),
                Err(ProofError::Malformed)
            );
        }
        for field in 0..3 {
            let mut bad = proof.clone();
            match field {
                0 => bad.leaf.prehash_key = 1,
                1 => bad.leaf.prehash_value = 0,
                _ => bad.leaf.length = 0,
            }
            assert_eq!(
                verify_existence(&bad, root, &key, &value),
                Err(ProofError::Malformed)
            );
        }
        let mut bad = proof.clone();
        bad.leaf.prefix.push(0);
        assert_eq!(
            verify_existence(&bad, root, &key, &value),
            Err(ProofError::Malformed)
        );
        let mut bad = proof.clone();
        bad.path[0].hash = 2;
        assert_eq!(
            verify_existence(&bad, root, &key, &value),
            Err(ProofError::Malformed)
        );
        let mut bad = proof.clone();
        bad.path[0].prefix[0] = 1;
        assert_eq!(
            verify_existence(&bad, root, &key, &value),
            Err(ProofError::Malformed)
        );
        let mut bad = proof.clone();
        if bad.path[0].suffix.is_empty() {
            let marker = bad.path[0].prefix.len() - 1;
            bad.path[0].prefix[marker] = 31;
        } else {
            bad.path[0].suffix[0] = 31;
        }
        assert_eq!(
            verify_existence(&bad, root, &key, &value),
            Err(ProofError::Malformed)
        );
        for malformed in [
            vec![],
            vec![0],
            vec![0x0a, 0x80],
            vec![0x0a, 0xff, 0xff, 0xff, 0xff, 0x7f],
            vec![0x0a, 0x80, 0],
            vec![0x0b],
            vec![0x08, 1],
        ] {
            assert_eq!(decode_existence(&malformed), Err(ProofError::Malformed));
        }
        let mut duplicate = encoded.clone();
        duplicate.extend_from_slice(&[0x0a, 1, 1]);
        assert_eq!(decode_existence(&duplicate), Err(ProofError::Malformed));
        let mut unknown = encoded;
        unknown.extend_from_slice(&[0xa0, 6, 1]);
        assert_eq!(decode_existence(&unknown), Ok(proof));
    }
}
