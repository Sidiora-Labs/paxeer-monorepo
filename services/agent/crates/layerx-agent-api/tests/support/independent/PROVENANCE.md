# Verification source provenance

The field arithmetic, Edwards group operations, SHA-512 and error enum are the verification-only source modules from ed25519-compact 2.1.1, licensed under MIT. Source archive: https://crates.io/api/v1/crates/ed25519-compact/2.1.1/download
Archive SHA-256: e9b3460f44bea8cd47f45a0c70892f1eff856d97cd55358b2f73f663789f6190
Only the field module's crate-relative error import is made module-relative. No signing, randomness, FFI, executable or external crate is used.

The chain_proof modules are source-derived from programs/crates/layerx-programs-registry/src/chain_proof. Their hash/signature imports use the independent test crypto implementations; vote.rs uses a module-relative import. This preserves the native protobuf, Merkle and voting-power rules. API request, response and export codecs use the production crate implementations.
