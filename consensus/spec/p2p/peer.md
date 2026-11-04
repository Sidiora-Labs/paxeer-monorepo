# Peers

This page describes node identity and TCP handshakes in the Paxeer consensus
implementation. The wire definitions are
[`internal/p2p/p2p.proto`](../../internal/p2p/p2p.proto); the handshake consumers
are [`handshake.go`](../../internal/p2p/handshake.go) and the connection admission
code in [`internal/p2p`](../../internal/p2p). These connections belong to the same
network whose kernel proposes blocks.

For peer discovery, see the local [PEX implementation](../../internal/p2p/pex).
For channel payloads, see [Consensus](messages/consensus.md) and
[State Sync](messages/state-sync.md). The [RPC reference](../rpc/README.md)
describes node and peer inspection through the local consensus listener.

## Peer Identity

A node has a persistent Ed25519 key. Its `types.NodeID` is the lowercase hex
encoding of the first 20 bytes of SHA-256 of the raw public key: 40 hex digits.
See [`types/node_id.go`](../../types/node_id.go) and
[`crypto/ed25519/conv.go`](../../crypto/ed25519/conv.go). The node key authenticates
the transport; it is distinct from a validator's consensus role.

[`NodeAddress`](../../internal/p2p/address.go) carries the node ID, a hostname or
IP address, and a port. Its string form is `mconn://<ID>@<HOST>:<PORT>`; the parser
also accepts `<ID>@<HOST>:<PORT>`. IPv6 hosts use brackets. Specify the actual P2P
port explicitly: the parser's omitted-port default is 26657, while
`DefaultP2PConfig` listens on `tcp://127.0.0.1:26656`.

A node ID can have several candidate addresses. Dialing checks the authenticated
remote key against the intended node ID before accepting node information.
Advertised addresses are not proof that another node controls those identities.

## Connections

The TCP connection first establishes encryption, then authenticates a persistent
node key. Ordinary channel connections next exchange `NodeInfo`. Peers that
negotiate the PaxGiga connection mode instead enter its peer RPC transport.
These are internal transport choices, not separate networks or public API
services.

The connection APIs take contexts for cancellation. `RouterOptions.HandshakeTimeout`
bounds the authenticated handshake; the outgoing channel path also exchanges
`NodeInfo` within that timeout. Incoming `NodeInfo` exchange uses the connection's
parent context. Do not assume two independent timeout settings for the two
stages.

### Authenticated Encryption Handshake

[`conn/secret_connection.go`](../../internal/p2p/conn/secret_connection.go)
implements the ephemeral key exchange and encrypted framing.

1. Each side generates an ephemeral X25519 key pair and concurrently sends and
   receives a protobuf `Preface`. Its only field is `bytes sts_public_key = 1`.
   [`conn/protoio.go`](../../internal/p2p/conn/protoio.go) prefixes the encoded
   message with its byte length as an unsigned varint. The read limit is 1,024
   bytes; the public key must be exactly 32 bytes.
2. Sort the two public keys lexicographically. Start a Merlin transcript named
   `TENDERMINT_SECRET_CONNECTION_TRANSCRIPT_HASH`, append the lower key under
   `EPHEMERAL_LOWER_PUBLIC_KEY`, the upper key under
   `EPHEMERAL_UPPER_PUBLIC_KEY`, and the X25519 shared secret under `DH_SECRET`.
   Invalid shared-secret derivation fails the handshake.
3. Extract the 32-byte authentication challenge from the transcript using
   `SECRET_CONNECTION_MAC`. Separately derive 64 bytes of encryption key material
   with HKDF-SHA256, the shared secret as input key material, no salt, and info
   `TENDERMINT_SECRET_CONNECTION_KEY_AND_CHALLENGE_GEN`. The side with the lower
   ephemeral key uses the first 32 bytes to receive and the second 32 to send;
   the other side uses the reverse assignment.
4. Encrypt subsequent traffic with ChaCha20-Poly1305. A plaintext frame contains
   a four-byte little-endian data length followed by a 1,024-byte data area,
   padded when necessary. The 1,028-byte frame plus the 16-byte tag produces
   1,044 bytes on the wire. Each direction has an independent counter starting
   at zero. The 12-byte nonce has four zero bytes followed by the little-endian
   64-bit counter. The implementation rejects counter overflow.
5. Over this encrypted channel, exchange a varint-length-prefixed protobuf
   `Handshake`, with a receive limit of 1,048,576 bytes. Each side signs the
   **Merlin challenge**, not the ephemeral public key or HKDF output, with its
   persistent Ed25519 key.

The `Handshake` fields are:

| Name | Type | Field Number | Meaning |
| --- | --- | --- | --- |
| node_auth_key | NodePublicKey | 1 | Persistent public key; nested `bytes ed25519 = 1` is 32 bytes |
| node_auth_sig | bytes | 2 | 64-byte Ed25519 signature over the session challenge |
| pax_giga_connection | bool | 3 | Requests the PaxGiga peer RPC connection mode |
| self_addr | optional string | 4 | Address at which this node advertises it can be dialed |
| pex_addrs | repeated string | 5 | Initial peer-discovery addresses; at most 100 |

[`conv.go`](../../internal/p2p/conv.go) requires the public key and checks key and
signature encodings. `handshake` verifies the signature, requires any
`self_addr` to name the authenticated node, and rejects an oversized peer list.
The dialer additionally requires the authenticated ID to equal its dial target.
Both sides verify the remote signature; the dialer also knows which identity it
intended to reach. Encryption alone does not establish that intended identity.

### Peer Filter

For ordinary connections, admission runs after the identity and `NodeInfo`
checks. The peer manager rejects a connection to self and applies its connection
pool policy. Duplicate connections may be rejected or replace an existing
connection according to that policy; there is no unconditional rule that every
second connection is disconnected. This path does not call the historical ABCI
address/public-key whitelist described by older peer guides.

### Tendermint Version Handshake

On the ordinary channel path, `exchangeNodeInfo` sends and receives the
varint-length-prefixed `tendermint.p2p.NodeInfo` protobuf defined in
[`proto/tendermint/p2p/types.proto`](../../proto/tendermint/p2p/types.proto).
The receive limit is 10,240 bytes.

| Name | Type | Field Number |
| --- | --- | --- |
| protocol_version | ProtocolVersion (`uint64 p2p = 1`, `block = 2`, `app = 3`) | 1 |
| node_id | string | 2 |
| listen_addr | string | 3 |
| network | string | 4 |
| version | string | 5 |
| channels | bytes | 6 |
| moniker | string | 7 |
| other | NodeInfoOther (`string tx_index = 1`, `rpc_address = 2`) | 8 |

`node_id` must match the ID derived from the authenticated key. Then
[`NodeInfo.Validate`](../../types/node_info.go) checks the listen address
(including DNS resolution), ASCII text fields, a nonempty moniker, at most 16
channel bytes with no duplicates, and `tx_index` equal to `on`, `off` or empty.
The network string and block protocol version must match the local node.
`CompatibleWith` currently does not require matching P2P or application versions
or an intersection of advertised channels. Advertised channels determine which
channel messages a peer can receive.

When both sides negotiate `pax_giga_connection`, the inbound transport branches
to `RunInboundConn` before this ordinary `NodeInfo` exchange. Its RPC definitions
are in [`internal/p2p/giga/api.proto`](../../internal/p2p/giga/api.proto). Do not
send classic channel envelopes on that RPC connection.

## Connection Activity

After admission, ordinary connections use `conn.NewMConnection` and its protobuf
`Packet` envelope. Channel receive loops deliver messages to the subscribed
consumers; send queues carry messages back to the peer. The current API uses
opened channels and peer updates rather than the historical switch's `AddPeer`
and reactor `Receive` callbacks. Connection cancellation or a transport error
ends the loops and notifies the peer manager of disconnection.
