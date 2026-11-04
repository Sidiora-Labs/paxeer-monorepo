# Interop committed proof contract

Paxeer mainnet-beta is one BFT network. Its kernel participates in the network and proposes blocks; a receipt is final when its block is committed. The proof library authenticates receipt bytes and consensus evidence for that network. It does not introduce a separate execution network or a later settlement service. Explorer WebSocket messages are indexer notifications; see the [explorer ws contract](explorer-ws.md) for their delivery and address-encoding rules.

This page describes the implemented Rust API in [layerx-committed-proof](../../../services/interop/crates/layerx-committed-proof/src/lib.rs), replacing the old pinned snapshot that marked these APIs planned. The Cargo package is `layerx-committed-proof`; its Rust import path is `layerx_committed_proof`. Public modules are `proto`, `merkle`, `header`, `commit` and `committed`. Receipt decoding and Ed25519 verification are implemented in `lib.rs`; there are no public `validators`, `ed25519`, `results` or `batch` modules.

## rs-committed-proof-crate

### Public signatures

The two functions named `verify_committed` have different arguments and guarantees. Use the fully qualified name when describing or calling them.

```rust
// layerx_committed_proof::merkle
pub fn root(items: &[&[u8]]) -> [u8; 32];
pub fn verify_proof(
    root: [u8; 32], total: u64, index: u64,
    leaf: &[u8], aunts: &[[u8; 32]],
) -> Result<(), ProofError>;

// layerx_committed_proof::header
impl Header {
    pub fn hash(&self) -> [u8; 32];
}
pub fn validators_hash(set: &[Validator]) -> [u8; 32];

// layerx_committed_proof::commit
pub fn verify_commit(
    chain_id: &str, set: &[Validator], header_hash: [u8; 32],
    height: u64, commit: &Commit,
) -> Result<u64, ProofError>;

// layerx_committed_proof (crate root)
pub fn verify_header(
    header: &header::Header, set: &[header::Validator],
) -> Result<[u8; 32], ProofError>;
pub fn verify_receipted(
    raw_transaction: &[u8], signed_receipt: &[u8], receipt_json: &[u8],
    prev_seq_hash: [u8; 32],
    keys: &[([u8; 32], [u8; 32], u64, u64)],
) -> Result<[u8; 32], ProofError>;
pub fn verify_committed(
    raw_transaction: &[u8], signed_receipt: &[u8], receipt_json: &[u8],
    prev_seq_hash: [u8; 32],
    keys: &[([u8; 32], [u8; 32], u64, u64)], block_hash: [u8; 32],
) -> Result<[u8; 32], ProofError>;

// layerx_committed_proof::committed
pub fn verify_committed(
    receipt_digest: [u8; 32], proof: &CommittedProof,
    set: &[Validator], chain_id: &str,
) -> Result<CommittedBlock, ProofError>;
```

`Header` and `Validator` come from [header.rs](../../../services/interop/crates/layerx-committed-proof/src/header.rs); `Commit` comes from [commit.rs](../../../services/interop/crates/layerx-committed-proof/src/commit.rs). `CommittedProof` and `CommittedBlock` are defined at the crate root and re-exported by `committed`:

```rust
pub struct CommittedProof {
    pub header: header::Header,
    pub commit: commit::Commit,
    pub execution_header: header::Header,
    pub batch_transaction: Vec<u8>,
    pub batch_total: u64,
    pub batch_index: u64,
    pub batch_aunts: Vec<[u8; 32]>,
    pub signed_receipt: Vec<u8>,
    pub activity_ids: Vec<[u8; 32]>,
}
pub struct CommittedBlock {
    pub number: u64,
    pub hash: [u8; 32],
}
```

### Receipt authentication and JSON views

`verify_receipted` authenticates the canonical signed receipt, its JSON view, raw transaction and previous sequence hash, then returns the receipt digest. Each key pin is `(key_id, ed25519_public_key, first_height, last_height)`: both byte arrays are 32 bytes, and both height bounds are inclusive. The signed `keyId` must be nonzero and match the pin; an unpinned public key is insufficient.

The verifier checks Keccak-256 of the raw transaction against the receipt's activity id, verifies the Ed25519 signature over the domain-separated SHA-256 receipt digest, and checks the sequence chain against `prev_seq_hash`. It validates every JSON field against the signed representation, including execution summary fields, payment and fee denominations, return data, VM error and logs. Quantities are canonical `0x` hex strings; byte fields are rendered as lowercase `0x` hex. Missing or additional JSON fields fail the exact view comparison.

The crate-root `verify_committed` performs the same receipt authentication with an expected `"committed"` view. It additionally binds `blockHash` to its supplied `block_hash` and `blockNumber` to the signed receipt's height. It returns a receipt digest, not a `CommittedBlock`. Neither root receipt verifier establishes consensus inclusion. `verify_header` checks the validator hash and timestamp before returning a header hash; it also does not verify a consensus commit by itself. Callers must establish the trust and commitment of supplied block evidence separately.

The parser accepts a nonempty raw transaction up to `1_048_576` bytes, a signed receipt up to `1_048_576` bytes and a JSON view up to `4_194_304` bytes. It requires 1–512 effects, at most 256 body bytes per effect, positive receipt height and timestamp, a valid result code and no trailing receipt bytes. These are library input bounds, not network deployment payload limits.

### Consensus inclusion

`committed::verify_committed` requires a digest already authenticated by receipt verification. It verifies a batch transaction and receipt sequence against consensus evidence and returns the **execution block** number and hash:

1. Both headers must match the supplied nonempty consensus `chain_id` (at most 50 bytes). `header.height` must be exactly `execution_header.height + 1`, and the execution header's `next_validators_hash` must match the successor header's `validators_hash`.
2. `verify_header` binds the successor header to the supplied validator set. `commit::verify_commit` verifies the successor's signatures and requires strictly more than two thirds of total voting power. The successor's `last_block_id.hash` must equal the execution header hash.
3. The batch must be at index zero, `batch_total - 1` must equal `activity_ids.len()`, and the Merkle proof must reconstruct the execution header's `data_hash`. The Merkle leaf input is `SHA256(batch_transaction)`, matching the consensus transaction-hash leaf encoding.
4. The batch begins with `PAXEER/v1/sequence-batch\0` and has a 216-byte body. Its height, sequence range, activity digest and resulting sequence hash must match the supplied activities and signed receipt position. The receipt's height must be positive and no greater than the execution block height.

The `header` and `execution_header` fields are successive blocks of the same chain. The successor supplies commitment evidence; this does not describe two networks or a second finality mechanism. The caller supplies a trusted validator set and consensus chain id; accepting a self-consistent header and an arbitrary validator set is not a trust bootstrap. The consensus string `chain_id` is distinct from the EVM JSON-RPC numeric chain id `125`.

A downstream example is [layerx-x402's buyer](../../../services/interop/crates/layerx-x402/src/buyer.rs): `verify_committed_evidence` parses the public proof types, checks the evidence against `TrustedConsensusAnchor`, calls `committed::verify_committed`, then compares the returned execution block number and hash with the claimed values. The [seller](../../../services/interop/crates/layerx-x402/src/seller.rs) also calls the namespaced inclusion verifier.

### Merkle, header and commit rules

[merkle.rs](../../../services/interop/crates/layerx-committed-proof/src/merkle.rs) implements RFC 6962 hashing compatible with `consensus/crypto/merkle/{tree,proof}.go`: the empty root is SHA-256 of empty bytes, leaves use the `0x00` prefix, inner nodes use `0x01`, and aunts run from leaf toward root. Invalid positions and inexact aunt counts are refused. For block data trees, items are SHA-256 transaction digests, not raw transaction bytes.

`Header::hash` hashes the fourteen consensus header fields in protobuf encoding order. It panics on a timestamp outside protobuf's permitted range; `verify_header` and the inclusion verifier return `InvalidCommit` for an invalid timestamp instead. `header::validators_hash` preserves the supplied consensus validator order, so callers must not reorder the set.

`commit::verify_commit` validates the commit height, round, block id, part-set header and signature slots against the supplied set. Validators must have positive power and distinct public keys, with total power at most `i64::MAX / 8`. Every present precommit signature is verified using the canonical consensus vote encoding; only votes for the block count toward signed power. The returned `u64` is signed voting power. Exactly two thirds is insufficient.

### Errors

The public Rust error is `ProofError`, with these variants:

| Variants | Meaning |
|---|---|
| `InvalidProof`, `RootMismatch` | Malformed inclusion/batch evidence or mismatched Merkle/activity/sequence-root commitment |
| `ValidatorsHashMismatch` | Supplied set does not match the header commitment |
| `InvalidCommit`, `InvalidValidatorSet`, `InvalidCommitSignature` | Invalid commit/header metadata, validator powers/keys or precommit signature |
| `InsufficientVotingPower` | Block votes do not strictly exceed two thirds of total power |
| `InvalidReceipt` | Malformed or noncanonical receipt encoding |
| `ReceiptViewMismatch` | JSON fields or execution outputs disagree with the signed receipt |
| `ReceiptActivityMismatch` | Raw transaction hash differs from the signed activity id |
| `ReceiptSequenceMismatch` | Receipt sequence link differs from the supplied chain or batch position |
| `UnpinnedReceiptSignature` | No eligible key-id/public-key pin verifies within its inclusive height range |

These are library errors, not JSON-RPC error codes. In particular, the current voting-power variant is `InsufficientVotingPower`.

### Existing acceptance coverage

The following implemented tests retain the useful acceptance mapping from the former snapshot. Their existence is not a claim that their gates ran in this documentation task.

| Requirement | Test source and assertions |
|---|---|
| `11010.1`, `11010.2` | [tests/merkle.rs](../../../services/interop/crates/layerx-committed-proof/tests/merkle.rs): `roots_match_the_reference_tree` compares the recorded 0-, 1-, 2- and 5-item roots; `a_flipped_aunt_is_refused` checks proof tampering |
| `11016.1`, `11016.2` | [tests/header.rs](../../../services/interop/crates/layerx-committed-proof/tests/header.rs): `header_hash_matches_the_reference_header` and `validators_hash_matches_the_reference_set` compare reference commitments, including validator order |
| `11029.1`, `11029.2` | [tests/commit.rs](../../../services/interop/crates/layerx-committed-proof/tests/commit.rs): `a_three_of_four_commit_verifies` accepts sufficient signed power; `a_two_of_four_commit_is_refused` rejects insufficient power |
| `11040.1` | [tests/committed.rs](../../../services/interop/crates/layerx-committed-proof/tests/committed.rs): `committed_vectors_verify_end_to_end` checks accepting execution block identities and the recorded refusal variants against [vector-v4-committed.json](../../../services/interop/specs/conformance/committed/vector-v4-committed.json) |
| `11049.1` | [tests/receipted.rs](../../../services/interop/crates/layerx-committed-proof/tests/receipted.rs): `receipted_fixture_cases_verify_or_refuse` checks V1 acceptance and V3 refusal cases from [sequencer-receipt-v1.json](../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json); the filename identifies the receipt fixture, not a separate service |

`committed_fixture_view_verifies_and_binds_block_fields_and_rung` in `tests/receipted.rs` covers the root committed-view verifier. [tests/proof_rs_committed_proof_crate.rs](../../../services/interop/crates/layerx-committed-proof/tests/proof_rs_committed_proof_crate.rs) exercises the public producer-to-verifier boundary, including repeated decoding, refusal cases and invalid timestamps. This task's declared qualification checks documentation only; it does not execute the Rust test suite.
