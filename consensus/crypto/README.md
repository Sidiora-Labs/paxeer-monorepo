# crypto

The crypto package provides Ed25519 keys ([`ed25519`](./ed25519/)), SHA-256 hashing ([`tmhash`](./tmhash/)) and simple Merkle trees ([`merkle`](./merkle/)).

## Importing it

The root package exposes `PubKey`, `PrivKey`, `Sig` and `BatchVerifier` as aliases
for the Ed25519 types in [`crypto.go`](crypto.go):
`import "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto"`

To use the Ed25519 package directly,
`import "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/ed25519"`

## Binary encoding

For Binary encoding, please refer to the [encoding specification](../spec/core/encoding.md).

## JSON Encoding

[`libs/json`](../libs/json/doc.go) encodes concrete values using the engine's
JSON rules and honors each type's JSON methods. It does not add type tags.
The Ed25519 key types register their tags with the separate
[`internal/jsontypes`](../internal/jsontypes/) package, which is available to
callers within the consensus tree.

The type-tagged encoding wraps the base64 key bytes in `type` and `value` fields:

| Go type             | JSON type tag               |
| ------------------- | --------------------------- |
| `ed25519.SecretKey` | `tendermint/PrivKeyEd25519` |
| `ed25519.PublicKey` | `tendermint/PubKeyEd25519`  |

The key types' own `MarshalJSON` methods encode just the base64 byte string;
[`jsontypes.Marshal`](../internal/jsontypes/jsontypes.go) adds the type-tagged wrapper.
