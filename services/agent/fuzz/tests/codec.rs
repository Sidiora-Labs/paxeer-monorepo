use std::fs;
use std::path::{Path, PathBuf};

use layerx_crypto::evm_transaction::{self, Eip1559Call};

#[path = "../fuzz_targets/codec.rs"]
mod codec;
#[path = "../fuzz_targets/support.rs"]
mod support;

fn corpus_seeds(target: &str) -> Vec<(PathBuf, Vec<u8>)> {
    let directory = Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("corpus")
        .join(target);
    let mut paths: Vec<_> = fs::read_dir(directory)
        .expect("corpus directory must exist")
        .map(|entry| entry.expect("corpus entry must be readable").path())
        .collect();
    paths.sort();
    assert!(!paths.is_empty(), "corpus must contain seeds");
    paths
        .into_iter()
        .map(|path| {
            let contents = fs::read(&path).expect("corpus seed must be readable");
            (path, support::input_bytes(&contents).into_owned())
        })
        .collect()
}

fn signed_transfer() -> Vec<u8> {
    Eip1559Call {
        chain_id: 125,
        nonce: 0,
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 2,
        gas_limit: 21_000,
        to: [0x11; 20],
        data: Vec::new(),
    }
    .sign(&[1; 32])
    .expect("transfer must be signed by the fixture key")
}

#[test]
fn envelope_decode_round_trips_signed_eip1559_transactions() {
    let produced = signed_transfer();
    let mut found_transfer = false;
    for (path, bytes) in corpus_seeds("envelope_decode") {
        let (call, signature) = evm_transaction::decode_signed(&bytes)
            .unwrap_or_else(|error| panic!("{}: {error}", path.display()));
        assert_eq!(evm_transaction::assemble(&call, &signature), bytes);
        codec::envelope_round_trip(&bytes);
        if path.file_name().and_then(|name| name.to_str()) == Some("eip1559-transfer-1") {
            assert_eq!(bytes, produced);
            found_transfer = true;
        }
    }
    assert!(found_transfer, "signed transfer seed must be present");
}

#[test]
fn roundtrip_corpus_seeds_round_trip_through_their_codecs() {
    let produced = signed_transfer();
    let mut found_transfer = false;
    for (path, bytes) in corpus_seeds("roundtrip") {
        codec::roundtrip_case(&bytes);
        let (&selector, transaction) = bytes
            .split_first()
            .expect("roundtrip seed must include a selector");
        if matches!(selector % 6, 0 | 1) {
            let (call, signature) = codec::decode_signed(transaction)
                .unwrap_or_else(|error| panic!("{}: {error}", path.display()));
            assert_eq!(codec::assemble(&call, &signature), transaction);
        }
        if path.file_name().and_then(|name| name.to_str()) == Some("eip1559-transfer-1") {
            assert_eq!(selector, 0);
            assert_eq!(transaction, produced);
            for selector in [0, 1, 6, 7] {
                let mut input = vec![selector];
                input.extend_from_slice(transaction);
                codec::roundtrip_case(&input);
            }
            found_transfer = true;
        }
    }
    assert!(found_transfer, "selector-0 transfer seed must be present");
}
