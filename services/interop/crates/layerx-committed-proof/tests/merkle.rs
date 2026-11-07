use layerx_committed_proof::{
    merkle::{root, verify_proof},
    ProofError,
};

// Reference vectors: a Go one-off calling HashFromByteSlices in
// consensus/crypto/merkle/tree.go:11 on [], ["a"], ["a", "b"], and
// ["a", "b", "c", "d", "e"]. Aunts come from ProofsFromByteSlices in
// consensus/crypto/merkle/proof.go:35 on the same five-item list.
const ITEMS: [&[u8]; 5] = [b"a", b"b", b"c", b"d", b"e"];
const FIVE_ROOT: &str = "fe14a5426fbd70c0fa73f52342afed0da0bd23c4838662ccf6b88a3070ead97b";

fn hash(hex: &str) -> [u8; 32] {
    assert_eq!(hex.len(), 64);
    let mut bytes = [0; 32];
    for (index, byte) in bytes.iter_mut().enumerate() {
        *byte = u8::from_str_radix(&hex[2 * index..2 * index + 2], 16)
            .unwrap_or_else(|error| panic!("invalid reference hash: {error}"));
    }
    bytes
}

#[test]
fn roots_match_the_reference_tree() {
    for (count, expected) in [
        (
            0,
            "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
        ),
        (
            1,
            "022a6979e6dab7aa5ae4c3e5e45f7e977112a7e63593820dbec1ec738a24f93c",
        ),
        (
            2,
            "b137985ff484fb600db93107c77b0365c80d78f5b429ded0fd97361d077999eb",
        ),
        (5, FIVE_ROOT),
    ] {
        assert_eq!(root(&ITEMS[..count]), hash(expected), "{count} leaves");
    }
    // An empty leaf is domain separated from an empty tree.
    assert_ne!(root(&[b""]), root(&[]));
}

#[test]
fn a_flipped_aunt_is_refused() {
    let proofs: &[&[&str]] = &[
        &[
            "57eb35615d47f34ec714cacdf5fd74608a5e8e102724e80b24b287c0c27b6a31",
            "dbbd68c325614a73dacb4e7a87a2b7b4ae9724b489e5629ee83151fe8f0eafd7",
            "2824a7ccda2caa720c85c9fba1e8b5b735eecfdb03878e4f8dfe6c3625030bc4",
        ],
        &[
            "022a6979e6dab7aa5ae4c3e5e45f7e977112a7e63593820dbec1ec738a24f93c",
            "dbbd68c325614a73dacb4e7a87a2b7b4ae9724b489e5629ee83151fe8f0eafd7",
            "2824a7ccda2caa720c85c9fba1e8b5b735eecfdb03878e4f8dfe6c3625030bc4",
        ],
        &[
            "d070dc5b8da9aea7dc0f5ad4c29d89965200059c9a0ceca3abd5da2492dcb71d",
            "b137985ff484fb600db93107c77b0365c80d78f5b429ded0fd97361d077999eb",
            "2824a7ccda2caa720c85c9fba1e8b5b735eecfdb03878e4f8dfe6c3625030bc4",
        ],
        &[
            "597fcb31282d34654c200d3418fca5705c648ebf326ec73d8ddef11841f876d8",
            "b137985ff484fb600db93107c77b0365c80d78f5b429ded0fd97361d077999eb",
            "2824a7ccda2caa720c85c9fba1e8b5b735eecfdb03878e4f8dfe6c3625030bc4",
        ],
        &["33376a3bd63e9993708a84ddfe6c28ae58b83505dd1fed711bd924ec5a6239f0"],
    ];
    for (index, proof) in proofs.iter().enumerate() {
        let aunts: Vec<_> = proof.iter().map(|value| hash(value)).collect();
        let index = index as u64;
        let leaf = ITEMS[index as usize];
        assert_eq!(
            verify_proof(hash(FIVE_ROOT), 5, index, leaf, &aunts),
            Ok(())
        );

        // This includes item 3 of 5 and exercises both directions at each level.
        for position in 0..aunts.len() {
            let mut flipped = aunts.clone();
            flipped[position][0] ^= 1;
            assert_eq!(
                verify_proof(hash(FIVE_ROOT), 5, index, leaf, &flipped),
                Err(ProofError::RootMismatch)
            );
        }
        assert_eq!(
            verify_proof(hash(FIVE_ROOT), 5, index, b"changed", &aunts),
            Err(ProofError::RootMismatch)
        );
        assert_eq!(
            verify_proof(hash(FIVE_ROOT), 5, index, leaf, &aunts[1..]),
            Err(ProofError::InvalidProof)
        );
        let mut extra = aunts.clone();
        extra.push([0; 32]);
        assert_eq!(
            verify_proof(hash(FIVE_ROOT), 5, index, leaf, &extra),
            Err(ProofError::InvalidProof)
        );
        assert_eq!(
            verify_proof(hash(FIVE_ROOT), 5, 5, leaf, &aunts),
            Err(ProofError::InvalidProof)
        );
    }
    assert_eq!(
        verify_proof(root(&[]), 0, 0, b"", &[]),
        Err(ProofError::InvalidProof)
    );
    assert_eq!(verify_proof(root(&[b"a"]), 1, 0, b"a", &[]), Ok(()));
    assert_eq!(verify_proof(root(&[b""]), 1, 0, b"", &[]), Ok(()));
    assert_eq!(
        verify_proof(root(&[b"a"]), 1, 0, b"a", &[[0; 32]]),
        Err(ProofError::InvalidProof)
    );
    assert_eq!(
        verify_proof([0; 32], u64::MAX, u64::MAX - 1, b"a", &[]),
        Err(ProofError::InvalidProof)
    );
}
