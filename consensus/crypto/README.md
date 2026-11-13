# crypto

crypto is the cryptographic package adapted for Tendermint's uses. In this tree it provides Ed25519 keys ([`ed25519`](./ed25519/)), SHA-256 hashing ([`tmhash`](./tmhash/)) and simple Merkle trees ([`merkle`](./merkle/)).

## Importing it

To get the interfaces,
`import "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto"`

For any specific algorithm, use its specific module e.g.
`import "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/ed25519"`

## Binary encoding

For Binary encoding, please refer to the [encoding specification](../spec/core/encoding.md).

## JSON Encoding

JSON encoding is done using the engine's internal json encoder ([`libs/json`](../libs/json/doc.go)); key types register their type tags with [`internal/jsontypes`](../internal/jsontypes/).

The type-tagged encoding wraps the base64 key bytes in `type` and `value` fields:

| Go type             | JSON type tag               |
| ------------------- | --------------------------- |
| `ed25519.SecretKey` | `tendermint/PrivKeyEd25519` |
| `ed25519.PublicKey` | `tendermint/PubKeyEd25519`  |

The key types' own `MarshalJSON` methods encode just the base64 byte string;
[`jsontypes.Marshal`](../internal/jsontypes/jsontypes.go) adds the type-tagged wrapper.
