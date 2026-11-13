# Cryptographic package API

The consensus crypto packages use the repository's root [Go module](../../go.mod).

- [`crypto.go`](crypto.go) aliases `PubKey`, `PrivKey`, `Sig` and `BatchVerifier`
  to the Ed25519 types. `Checksum` returns a 32-byte SHA-256 hash;
  `AddressHash` returns its first 20 bytes as an `Address`.
- [`ed25519`](ed25519/) provides `SecretKey`, `PublicKey` and `Signature`.
  `SecretKey.Public()` returns the public key and `SecretKey.Sign()` signs a
  message. `PublicKey.Verify()` and `BatchVerifier.Verify()` return an error
  when verification does not succeed.
- [`tmhash`](tmhash/hash.go) exposes SHA-256 through `New()` and `Sum()`.
- [`merkle`](merkle/) constructs ordered Merkle trees and membership proofs
  using full SHA-256 hashes, with distinct leaf and inner-node prefixes.

[The package README](README.md) describes imports and JSON key encodings.
