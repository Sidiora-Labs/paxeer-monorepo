# Interop committed proof contract

Source revision: `2a1033aa2`. This contract freezes the cited interface and acceptance blocks of the specification. Statuses include the specification overrides. A source marked planned has line 0; a positive source line is verified in the pinned revision.

The context and decision tables reproduce the rules linked by interface identifiers. An anchor without a linked context gains no additional storage, gas, activation or pause rule here. Acceptance qualification remains UNRUN until its declared gate succeeds.

## rs-committed-proof-crate

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-committed-proof-crate</code> | <code>rust_pub_item</code> | <code>new</code> | <code>services/interop/crates/layerx-committed-proof</code> (planned) | <code>crate layerx-committed-proof: proto, merkle, header, validators, commit, ed25519, results, batch modules</code> |
| <code>interface.11.rs-committed-proof-header-hash-j47</code> | <code>rust_pub_item</code> | <code>new</code> | <code>services/interop/crates/layerx-committed-proof/src/header.rs</code> (planned) | <code>pub fn hash(&amp;self) -&gt; [u8; 32]</code> |
| <code>interface.11.rs-committed-proof-merkle-j47</code> | <code>rust_pub_item</code> | <code>new</code> | <code>services/interop/crates/layerx-committed-proof/src/merkle.rs</code> (planned) | <code>pub fn root(items: &amp;[&amp;[u8]]) -&gt; [u8; 32]</code> |
| <code>interface.11.rs-committed-proof-verify-commit-j47</code> | <code>rust_pub_item</code> | <code>new</code> | <code>services/interop/crates/layerx-committed-proof/src/commit.rs</code> (planned) | <code>pub fn verify_commit(chain_id: &amp;str, set: &amp;[Validator], header_hash: [u8; 32], height: u64, commit: &amp;Commit) -&gt; Result&lt;u64, ProofError&gt;</code> |
| <code>interface.11.rs-committed-proof-verify-committed-j47</code> | <code>rust_pub_item</code> | <code>new</code> | <code>services/interop/crates/layerx-committed-proof/src/lib.rs</code> (planned) | <code>pub fn verify_committed(receipt_digest: [u8; 32], proof: &amp;CommittedProof, set: &amp;[Validator], chain_id: &amp;str) -&gt; Result&lt;CommittedBlock, ProofError&gt;</code> |
| <code>interface.11.rs-committed-proof-verify-receipted-j47</code> | <code>rust_pub_item</code> | <code>new</code> | <code>services/interop/crates/layerx-committed-proof/src/lib.rs</code> (planned) | <code>pub fn verify_receipted(raw_transaction: &amp;[u8], signed_receipt: &amp;[u8], prev_seq_hash: [u8; 32], keys: &amp;[([u8; 32], u64, u64)]) -&gt; Result&lt;[u8; 32], ProofError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-committed-proof-crate</code> | notes | stale task 0.50 do_1 creates the crate (absent at HEAD); consumed by layerx-x402 for committed-rung inclusion evidence (0.101 do_4) |
| <code>interface.11.rs-committed-proof-header-hash-j47</code> | notes | Task 11.2.3. |
| <code>interface.11.rs-committed-proof-merkle-j47</code> | notes | Task 11.2.2. |
| <code>interface.11.rs-committed-proof-verify-commit-j47</code> | notes | Task 11.2.4. |
| <code>interface.11.rs-committed-proof-verify-committed-j47</code> | notes | Task 11.2.5. |
| <code>interface.11.rs-committed-proof-verify-receipted-j47</code> | notes | Task 11.2.6. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.11.10.1</code> | req | 11010.1 |
| <code>acceptance.11.10.1</code> | scenario | Given the four recorded item lists, when merkle::root runs, then each root equals the recorded Go root. |
| <code>acceptance.11.10.1</code> | test | services/interop/crates/layerx-committed-proof/tests/merkle.rs::roots_match_the_reference_tree (planned) |
| <code>acceptance.11.10.1</code> | qualification | UNRUN |
| <code>req.11010</code> | ac_1 | merkle::root equals HashFromByteSlices (consensus/crypto/merkle/tree.go:11) for 0, 1, 2 and 5 items (planned). |
| <code>acceptance.11.10.2</code> | req | 11010.2 |
| <code>acceptance.11.10.2</code> | scenario | Given a valid proof of item 3 of 5, when one bit of its first aunt is flipped, then verify_proof returns ProofError and the unmodified proof verifies. |
| <code>acceptance.11.10.2</code> | test | services/interop/crates/layerx-committed-proof/tests/merkle.rs::a_flipped_aunt_is_refused (planned) |
| <code>acceptance.11.10.2</code> | qualification | UNRUN |
| <code>req.11010</code> | ac_2 | merkle::verify_proof refuses a proof with one flipped aunt bit (planned) (consensus/crypto/merkle/proof.go:35). |
| <code>acceptance.11.16.1</code> | req | 11016.1 |
| <code>acceptance.11.16.1</code> | scenario | Given the recorded header fields, when Header::hash runs, then it equals the recorded Go hash. |
| <code>acceptance.11.16.1</code> | test | services/interop/crates/layerx-committed-proof/tests/header.rs::header_hash_matches_the_reference_header (planned) |
| <code>acceptance.11.16.1</code> | qualification | UNRUN |
| <code>req.11016</code> | ac_1 | Header::hash equals Header.Hash (consensus/types/block.go:513) for the recorded header (planned). |
| <code>acceptance.11.16.2</code> | req | 11016.2 |
| <code>acceptance.11.16.2</code> | scenario | Given the recorded four-validator set, when validators_hash runs, then it equals the recorded Go hash, and reordering two validators changes it. |
| <code>acceptance.11.16.2</code> | test | services/interop/crates/layerx-committed-proof/tests/header.rs::validators_hash_matches_the_reference_set (planned) |
| <code>acceptance.11.16.2</code> | qualification | UNRUN |
| <code>req.11016</code> | ac_2 | validators_hash equals ValidatorSet.Hash (consensus/types/validator_set.go:364) for four validators (planned). |
| <code>acceptance.11.29.1</code> | req | 11029.1 |
| <code>acceptance.11.29.1</code> | scenario | Given the recorded header hash and three valid precommits of four, when verify_commit runs, then it returns power 3. |
| <code>acceptance.11.29.1</code> | test | services/interop/crates/layerx-committed-proof/tests/commit.rs::a_three_of_four_commit_verifies (planned) |
| <code>acceptance.11.29.1</code> | qualification | UNRUN |
| <code>req.11029</code> | ac_1 | verify_commit accepts a commit signed by three of four equal-power validators (planned) (consensus/types/block.go:853). |
| <code>acceptance.11.29.2</code> | req | 11029.2 |
| <code>acceptance.11.29.2</code> | scenario | Given the same commit with one of the three signatures removed, when verify_commit runs, then it returns ProofError::InsufficientPower. |
| <code>acceptance.11.29.2</code> | test | services/interop/crates/layerx-committed-proof/tests/commit.rs::a_two_of_four_commit_is_refused (planned) |
| <code>acceptance.11.29.2</code> | qualification | UNRUN |
| <code>req.11029</code> | ac_2 | verify_commit refuses a commit holding exactly half the power (planned) (consensus/types/vote.go:123). |
| <code>acceptance.11.40.1</code> | req | 11040.1 |
| <code>acceptance.11.40.1</code> | scenario | Given every case of vector-v4-committed.json, when verify_committed runs, then accepting cases return their block number and hash and refusal cases return their ProofError. |
| <code>acceptance.11.40.1</code> | test | services/interop/crates/layerx-committed-proof/tests/committed.rs::committed_vectors_verify_end_to_end (planned) |
| <code>acceptance.11.40.1</code> | qualification | UNRUN |
| <code>req.11040</code> | ac_1 | verify_committed accepts the accepting committed vectors and refuses the refusal cases (planned) (consensus/crypto/merkle/proof.go:35). |
| <code>acceptance.11.49.1</code> | req | 11049.1 |
| <code>acceptance.11.49.1</code> | scenario | Given each case of sequencer-receipt-v1.json, when verify_receipted runs, then V1 cases return their receiptDigest and V3 cases return their expected refusal. |
| <code>acceptance.11.49.1</code> | test | services/interop/crates/layerx-committed-proof/tests/receipted.rs::receipted_fixture_cases_verify_or_refuse (planned) |
| <code>acceptance.11.49.1</code> | qualification | UNRUN |
| <code>req.11049</code> | ac_1 | verify_receipted accepts every V1 case and refuses every V3 case of sdk/platform/conformance/fixtures/sequencer-receipt-v1.json:263 (planned). |
| <code>capability.11.2.1</code> | title | Add the RFC 6962 merkle root and inclusion proof to layerx-committed-proof |
| <code>capability.11.2.1</code> | reqs | 11010.1<br>11010.2 |
| <code>capability.11.2.2</code> | title | Hash committed block headers and validator sets in layerx-committed-proof |
| <code>capability.11.2.2</code> | reqs | 11016.1<br>11016.2 |
| <code>capability.11.2.3</code> | title | Verify a block commit carries more than two thirds of the validator power |
| <code>capability.11.2.3</code> | reqs | 11029.1<br>11029.2 |
| <code>capability.11.2.4</code> | title | Verify a receipt batch is included in a committed block |
| <code>capability.11.2.4</code> | reqs | 11040.1 |
| <code>capability.11.2.5</code> | title | Verify the receipted rung rules R4 to R6 against the recorded receipt fixture |
| <code>capability.11.2.5</code> | reqs | 11049.1 |
| <code>resolution.17.885</code> | title | decision.1108 binds: J47 owns the layerx-committed-proof crate |
| <code>resolution.17.885</code> | applies_to | decision.1108<br>task.11.2.2<br>task.11.14.3<br>task.11.15.1 |
| <code>resolution.17.885</code> | source | decision.1108 in fragments/J47/decision.kvx; tasks from affects |
