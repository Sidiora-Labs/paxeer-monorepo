# IBC specification

This page describes the implemented CosmWasm IBC path in Paxeer's `paxd`
application. Contracts execute inside the merged network, and IBC delivers
messages between chains through clients, connections, channels and relayers.
The kernel participates in the same network; a receipt is final when its block
is committed. IBC delivery and acknowledgement still require the corresponding
proof-bearing transactions on the participating chains.

## General Concepts

**IBC enabled** means that the contract's Wasm code exposes the IBC entry points
recognized by the VM. During instantiation, the
[keeper](keeper/keeper.go) calls `AnalyzeCode` on the code hash and checks
`HasIBCEntryPoints`. When true, it binds the contract port and stores its name
in `ContractInfo.IBCPortID`. This detection, rather than a proposed IBC feature
flag or an assumption about every contract, controls port creation.

The same check runs on migration. Migrating a contract without a port to code
with IBC callbacks adds its port. Migrating a contract that already has a port
to code without IBC callbacks fails with `ErrMigrationFailed` and
`requires ibc callbacks`. See [Governance](Governance.md) for privileged migration
and instantiation; those operations still use this lifecycle logic.

The [local IBC implementation](../../../interchain/modules/core) owns clients,
connections, channel state and packet verification. A contract defines its
application protocol and validates the counterparty, ordering and version.

## Overview

There is one port per IBC-enabled contract. In [keeper/ibc.go](keeper/ibc.go),
`PortIDForContract` produces `wasm.<bech32-contract-address>`.
`ContractFromPortID` requires the `wasm.` prefix and parses the remaining address.
The address identifies the local receiving contract; it is not an EVM hex address.

`ensureIbcPort` reuses a port whose capability the Wasm module already holds;
otherwise it binds the port and claims its capability. The module also claims
channel capabilities during the open handshake. Outgoing packets use the
calling contract's stored port and an owned channel capability, so a contract
does not choose another contract's source port.

[node/app.go](../../../node/app.go) registers `NewIBCHandler` for the `wasm`
module. The IBC core dispatches to this module through capability ownership;
[ibc.go](ibc.go) resolves the contract address from the local port and constructs
VM messages. [keeper/relay.go](keeper/relay.go) loads that contract's code and
state, invokes native VM IBC callbacks, meters their gas and processes their
responses.

Both ordered and unordered channels reach the contract with their ordering.
The VM strings are `ORDER_ORDERED` and `ORDER_UNORDERED`. Open-init and open-try
require exactly one valid connection hop; more or fewer return
`ErrTooManyConnectionHops`. Channel IDs are parsed and a sequence greater than
`2^32 - 1` returns `ErrMaxIBCChannels`. The adapter provides no multihop channel
support.

## Workflow

### Establish clients, connections and channels

Clients and connections are established through the IBC core, normally by a
relayer. Instantiating an IBC-enabled contract supplies its port; it does not
create a connection or an open channel. Chain-level inbound and outbound IBC
policy also applies: the [core message server](../../../interchain/modules/core/keeper/msg_server.go)
checks directional enablement on connection initiation and packet reception.
A bound port alone does not establish that IBC traffic is enabled.

The channel handshake invokes these contract callbacks:

| IBC module callback | Keeper call | VM call and message variant |
| --- | --- | --- |
| `OnChanOpenInit` | `OnOpenChannel` | `IBCChannelOpen`, `open_init` |
| `OnChanOpenTry` | `OnOpenChannel` | `IBCChannelOpen`, `open_try` |
| `OnChanOpenAck` | `OnConnectChannel` | `IBCChannelConnect`, `open_ack` |
| `OnChanOpenConfirm` | `OnConnectChannel` | `IBCChannelConnect`, `open_confirm` |
| `OnChanCloseInit` | `OnCloseChannel` | `IBCChannelClose`, `close_init` |
| `OnChanCloseConfirm` | `OnCloseChannel` | `IBCChannelClose`, `close_confirm` |

The channel message contains the local endpoint, counterparty endpoint,
ordering, version and connection ID. Open-try and open-ack also carry
`counterparty_version`. The contract should reject incompatible protocols and
unexpected peers.

Version handling follows this adapter's actual interfaces:

- Open-init passes the proposed version to the contract and propagates errors,
  but discards a version returned by `OnOpenChannel`.
- Open-try accepts a non-empty version returned by the contract; an empty return
  uses the counterparty's version.
- `NegotiateAppVersion` returns the proposed version unchanged. It performs no
  application compatibility checks.
- Open-ack supplies the counterparty version to the connect callback. Before
  invoking it, the adapter updates the stored counterparty channel ID so that
  contract queries and packet sends can use that endpoint during the callback.

### Send a packet

A contract emits an `IBCMsg.SendPacket` through its response messages. This is
an example of the `CosmosMsg` JSON payload defined in
[wasm-runtime/types/msg.go](../../../wasm-runtime/types/msg.go), before wrapping
it in a response submessage:

```json
{
  "ibc": {
    "send_packet": {
      "channel_id": "channel-0",
      "data": "aGVsbG8=",
      "timeout": {
        "block": {"revision": 1, "height": 1000000}
      }
    }
  }
}
```

Here `data` is the base64 encoding of `hello`. Replace the channel and timeout
with the connected local channel and a future height on the destination chain.
A timestamp timeout, when supplied, is a decimal string of nanoseconds since the
Unix epoch. The packet schema is in
[wasm-runtime/types/ibc.go](../../../wasm-runtime/types/ibc.go).

[IBCRawPacketHandler](keeper/handler_plugin.go) fills in the source port from
`ContractInfo.IBCPortID`, gets the send sequence and counterparty endpoint from
channel state, retrieves the channel capability, and calls the core
`SendPacket`. An absent contract port returns `ErrUnsupportedForContract`; an
empty channel ID returns `ErrEmpty`. Missing sequence, channel or capability
state is also an error. Sending records a packet for relay; it does not invoke
the remote contract synchronously.

### Receive, acknowledge and time out

| IBC module callback | Contract selected from | Keeper and VM call |
| --- | --- | --- |
| `OnRecvPacket` | Destination port | `OnRecvPacket` → `IBCPacketReceive` |
| `OnAcknowledgementPacket` | Source port | `OnAckPacket` → `IBCPacketAck` |
| `OnTimeoutPacket` | Source port | `OnTimeoutPacket` → `IBCPacketTimeout` |

Packet messages contain the data, source and destination port/channel pairs,
sequence and timeout. Receive and timeout messages include the packet and
relayer address. The acknowledgement message includes the acknowledgement bytes,
`original_packet` and relayer address. The contract must interpret the
application's acknowledgement format and implement any refund or compensation
logic for negative acknowledgements and timeouts.

On receive, the keeper processes the contract's submessages, attributes and
events and returns acknowledgement bytes. A submessage reply can replace those
bytes. `ContractConfirmStateAck.Success()` returns true for a contract-produced
acknowledgement, including one encoding an application error, so its contract
state changes are retained. A keeper or VM error instead becomes a core error
acknowledgement; the core discards the failed callback's cached state. The
adapter always wraps a successful contract result in an acknowledgement; it does
not expose a deferred-acknowledgement path by returning a nil acknowledgement.

Acknowledgement and timeout callback failures propagate as transaction errors.
Their successful responses process submessages and events through the same
contract response handling path.

### Authorization and recovery after restart

Relayed messages pass through the IBC core's proof and channel checks before
packet callbacks. Receive verification recognizes an already received packet
and returns a no-op without invoking the contract again. Outgoing packet
creation requires the Wasm module's channel capability. The relayer address in
a callback identifies the transaction signer; application authorization must
also validate the channel and counterparty established by the protocol.

Contract metadata, contract state and IBC channel/packet records live in chain
stores. [node/legacyabci/begin_block.go](../../../node/legacyabci/begin_block.go)
calls the capability module's begin-block handler; on the first block after a
restart, [InitMemStore](../../../sdk/x/capability/keeper/keeper.go) reconstructs
in-memory capabilities from persisted owners. Port names and contract state are
not replaced by newly instantiated contracts. The node also restores pinned
Wasm code through `InitializePinnedCodes` after loading the latest version.

Relayers must resume delivery of pending packets, acknowledgements or timeout
proofs after restart. Contract protocols should persist pending-operation data
in contract storage so that later callbacks can settle it. These storage and
initialization paths restore the state used by subsequent callbacks.

## Design limits

The current mapping assigns a stable port to each contract. Multiple ports per
contract, reassignment of a port and transfer of channels between contracts are
not provided by this adapter. The single-port-per-module alternative is not the
implemented mapping. Likewise, a custom packet protocol is not automatically
ICS-20 compatible; interoperability depends on the contract's handshake and
packet format.

## Tests and implementation references

[ibc_reflect_test.go](ibc_reflect_test.go) exercises Wasm contracts across the
local IBC test chains. [ibc_test.go](ibc_test.go) covers connection-hop validation
and packet conversion. [keeper/relay_test.go](keeper/relay_test.go) covers the
keeper callback adapters. The [IBC testing guide](ibctesting/README.md) describes
the local chain, endpoint and coordinator helpers. For ordinary contract
operations and module configuration, see [README.md](README.md); for privileged
contract lifecycle operations, see [Governance](Governance.md).

### Local native callback example

The committed IBC3 `wasm-runtime/testdata/ibc_reflect.wasm` contract accepts
`{"reflect_code_id":1}` at instantiation (replace 1 with the uploaded reflect
code ID). Its ordered channel protocol is `ibc-reflect-v1`; incompatible
versions are refused. On connect it creates a reflect account. A packet with
`{"who_am_i":{}}` returns a JSON acknowledgement naming that account. Query
`{"list_accounts":{}}` to read the stored channel/account association.

[Native callback tests](repair_m1289_test.go) exercise this contract through
the module callback adapter, including version
refusal, receive acknowledgements, acknowledgement and timeout callbacks.
The tests commit the channel/account association, close the native VM and persistent
store, reopen them, restore capabilities and pins, and repeat the query and
receive callback against the recovered state.
