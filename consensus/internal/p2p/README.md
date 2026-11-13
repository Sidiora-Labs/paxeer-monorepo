# p2p

The p2p package provides peer-to-peer communication for the consensus engine.
[`Router`](router.go) manages peer connections and channels, with transport
handling in [`transport.go`](transport.go). The optional Autobahn path is wired
through [`GigaRouter`](giga_router.go).

Protocol references:

- [Connection](../../spec/p2p/connection.md) for details on how connections and multiplexing work
- [Peer](../../spec/p2p/peer.md) for details on peer ID, handshakes, and peer exchange
- [Node](../../spec/p2p/node.md) for details about different types of nodes and how they should work
- [Pex](../../spec/p2p/messages/pex.md) for details on peer discovery and exchange
- [Config](../../spec/p2p/config.md) for configuration options
