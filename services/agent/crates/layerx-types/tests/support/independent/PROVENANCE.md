# Verification source provenance

The field arithmetic, Edwards group operations, SHA-512 and error enum derive
from ed25519-compact 2.1.1, licensed under MIT. Attribution is retained in
`LICENSE.ed25519-compact`.
Archive SHA-256: e9b3460f44bea8cd47f45a0c70892f1eff856d97cd55358b2f73f663789f6190
The field module uses a module-relative error import. These modules provide
verification without signing, randomness, FFI or external crate dependencies.

The `chain_proof` modules derive from
`programs/crates/layerx-programs-registry/src/chain_proof`. Their hash and
signature imports use the independent test crypto implementations; `vote.rs`
uses a module-relative import. These modules implement protobuf decoding,
Merkle hashing and voting-power verification. API request, response and export
codecs use the production crate implementations.

This crate uses the independent field, scalar and proof implementations in its test targets, with MIT attribution. Production API consumers are imported from their original source paths.
