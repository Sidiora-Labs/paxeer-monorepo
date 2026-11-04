# P2P Multiplex Connection

The node has two peer wire paths. The compatibility path carries reactor
messages over `MConnection`; Autobahn peers use the streaming multiplexer and
RPC registrations under `internal/p2p/giga`. Both run inside `paxd`. These are
node transport implementations on the same network.

## MConnection

[`MConnection`](../../internal/p2p/conn/oldmux.go) multiplexes ordered channel
messages over a `conn.Conn`. The transport uses authenticated, encrypted peer
connections before starting this multiplexer. The channel type `ChannelID` is
`uint16`, but the compatibility transport's send routine rejects IDs above 255.
Reactor channel IDs used on this wire therefore remain byte-sized.

A channel descriptor supplies its ID, priority, protobuf message type and queue
and receive capacities. If omitted, the low-level descriptor defaults are a
priority of 1, send queue capacity of 1, receive buffer capacity of 4096 bytes
and maximum received message size of 22020096 bytes. Reactors supply their own
capacities where necessary.

### Packet Framing

Each packet is a protobuf `p2p.Packet` from
[`p2p.proto`](../../internal/p2p/p2p.proto), prefixed by its encoded byte length as
an unsigned varint. [`WriteSizedMsg` and `ReadSizedMsg`](../../internal/p2p/conn/protoio.go)
write and read this framing. The receiver rejects invalid lengths and lengths
above the configured packet bound before allocating the body.

| `Packet.sum` variant | Protobuf field number | Contents |
| --- | --- | --- |
| `packet_ping` | 1 | Empty `PacketPing` message |
| `packet_pong` | 2 | Empty `PacketPong` message |
| `packet_msg` | 3 | `PacketMsg` fragment |

### Ping and Pong

Ping and pong are length-prefixed protobuf packet envelopes, not raw single-byte
0x01/0x02 messages. `pingRoutine` queues a ping, waits for an explicit pong within
`PongTimeout`, then waits `PingInterval` before repeating. Both durations default
to 10 seconds in `DefaultMConnConfig`. Ordinary data does not satisfy the pong
wait. Receiving a ping queues a pong, and control packets have priority over
data fragments. A missed pong returns a timeout error and ends the connection's
run scope.

### Msg

Reactor messages are serialized as protobuf before entering `MConnection`.
The multiplexer fragments these bytes into `PacketMsg` messages:

| Field | Protobuf type | Field number | Meaning |
| --- | --- | --- | --- |
| `channel_id` | `int32` | 1 | Channel ID; must fit the receiver's `ChannelID` and name a configured channel |
| `eof` | `bool` | 2 | Ends the current message on that channel |
| `data` | `bytes` | 3 | Fragment of the serialized reactor message |

Fragments on each channel are appended in order. On `eof = true`, the complete
byte slice is returned by `Recv`. Accumulated bytes may not exceed that
channel's `RecvMessageCapacity`. Unknown packet variants, invalid channel IDs,
malformed protobuf and oversized messages end the receive routine with an error.

`MaxPacketMsgPayloadSize` controls outgoing fragmentation; the receiver's packet
bound is calculated from a serialized packet containing a payload of that size.
`DefaultMConnConfig` uses 1400 bytes, while node setup overrides it from
`[p2p].max-packet-msg-payload-size`, whose current default is 1000000 bytes.
Peers must use compatible packet bounds. Packet size and whole-message capacity
are different limits.

### Multiplexing

The send queue has a bounded FIFO per channel. After pending ping and pong
packets, the scheduler picks the channel with the lowest ratio of recently sent
data bytes to priority. It emits one fragment at a time, preserving order within
a channel. Sent-byte statistics decay periodically. Writes are flushed using
`FlushThrottle`; send and receive rate limiters apply to serialized packets.

## Sending Messages

The actual lower-level API is context-aware and accepts serialized bytes:

```go
func (c *MConnection) Run(ctx context.Context) error
func (c *MConnection) Send(ctx context.Context, chID ChannelID, msgBytes []byte) error
func (c *MConnection) Recv(ctx context.Context) (ChannelID, []byte, error)
func (c *MConnection) Close()
```

These are API signatures, not a standalone Go program. `Send` waits for space in
the selected channel's queue and returns an error on cancellation or an unknown
channel. It takes ownership of the byte slice. Success means queued locally,
not delivered to the peer. There is no `MConnection.TrySend` method in this tree.

`Run` owns the ping, send, receive and statistics routines through structured
concurrency. Cancellation or a routine error ends the scope. The peer transport
runs its send and receive routines in the same connection scope and closes the
underlying connection when the connection handler exits. `Close` delegates to
that underlying connection. The `conn.Conn` read, write and flush operations
also take contexts and return errors.

## Peer

[`transport.go`](../../internal/p2p/transport.go) constructs a `ConnV2` with one
`MConnection`, a peer send queue and peer information. A successful compatibility
handshake exchanges `NodeInfo`, including advertised channel support. Peer
identity, dialed address and self-declared address belong to this connection
record; the peer manager owns dialing, connection admission and reconnection.
See [Config](./config.md) for persistent and private peers.

## Switch/Reactor

The old `Switch`/`Peer.Send(interface{})` example is retired. Reactors use typed
[`Channel[T]`](../../internal/p2p/channel.go) values opened with a channel
descriptor. `Send(msg T, to types.NodeID)` and `Broadcast(msg T)` queue protobuf
messages; `Recv(ctx)` returns a `RecvMsg[T]` and an error. The transport marshals
outgoing messages and unmarshals incoming bytes into the descriptor's message
type, applying its pre-decode scanner when provided.

These channel methods have different queue semantics from `MConnection.Send`:
sending to a disconnected peer or one that lacks the channel drops the message,
and queue pressure can prune messages. They do not return delivery acknowledgments.
The [message index](./messages/README.md) identifies the reactor protocols.

## Autobahn Streaming Connections

The authenticated handshake's `pax_giga_connection` field selects the Autobahn
path when both sides support it. Configured validator connections use this path;
compatibility connections can still serve classic peers. The code's
`GigaRouter` is an internal peer transport type, not a separate public service.

[`mux`](../../internal/p2p/mux/mux.go) uses its own framing: a one-byte protobuf
header length, the protobuf header, then the frame payload. Its OPEN, RESIZE, MSG
and CLOSE operations manage independent streams, message size bounds and
receive windows. These frames are distinct from `MConnection` packets.

The registered [Autobahn RPCs](./messages/README.md#autobahn-streaming-rpcs) use
stream-kind numbers and bounded protobuf messages. Their `Ping` RPC is also
distinct from compatibility `PacketPing`/`PacketPong`; the service sends it every
10 seconds with a 5-second timeout. Use the registered stream API and certificate
consumers when implementing an Autobahn peer.
