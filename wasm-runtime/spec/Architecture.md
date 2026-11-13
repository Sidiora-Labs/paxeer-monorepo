# Architecture

The [Go wrapper](../lib.go) exposes the CosmWasm VM to the
[Wasm keeper](../../wasm/x/wasm/keeper/keeper.go). The keeper owns the chain
integration: it selects the contract's code and storage, checks permissions,
constructs the execution environment and processes the contract's response.

## Contract context and state

[`NewEnv` and `NewInfo`](../../wasm/x/wasm/types/types.go) translate SDK context
and message data into the VM's JSON types. `Env` contains block height, block
time, the Cosmos chain identifier, the contract address and, when available,
the transaction's index within the block. `MessageInfo` supplies the sender and
funds for instantiation and execution. The contract can query balances through
the host query interface.

Uploaded bytecode is identified by its checksum in the VM. The chain module
stores a numeric code ID and maps each contract address to its code and
metadata. Multiple contract instances can use the same bytecode while keeping
separate state. Each invocation receives the contract's prefixed `KVStore`.

The wrapper supplies host callbacks for storage, address conversion,
queries and gas accounting. The VM receives these interfaces and
serialized data rather than the full SDK context. See the
[specification](Specification.md) for their definitions.

## Calling other modules

Contracts return a `Response` containing submessages, attributes, events and
optional data. The keeper dispatches those submessages after the VM call.
[`handler_plugin.go`](../../wasm/x/wasm/keeper/handler_plugin.go) provides
SDK message dispatch, IBC packet handling and bank burn handling. SDK messages
are validated and their signers must match the calling contract.

[`msg_dispatcher.go`](../../wasm/x/wasm/keeper/msg_dispatcher.go) executes each
submessage in a cached context. Successful submessage state is committed to the
parent context; failed submessage state is discarded. An optional gas limit
bounds a submessage. The `reply_on` policy chooses whether success, error or
both results invoke the original contract's `reply` entry point. An unhandled
submessage error or a failed reply propagates to the enclosing operation.

Contracts can therefore call other contracts through response messages and
receive their results through replies. The chain processes these calls within
the enclosing transaction.

## Querying other modules

The wrapper's `Querier` callback takes a typed `QueryRequest`, a gas limit and
returns bytes or an error. The
[query handler](../../wasm/x/wasm/keeper/query_plugins.go) creates a cached
context with a query gas meter, charges the parent context for the work and
discards query state changes.

Default query plugins handle bank balances, IBC ports and channels, staking
and Wasm queries. The default custom querier returns an unsupported-request
error, and the default Stargate querier rejects Stargate queries. Applications
can supply query handlers through the keeper options. A field in the VM query
schema alone does not establish support in a chain's query handler.

## Exposing contract queries

The module supports raw state reads and smart queries. Raw queries read a key
from the selected contract's storage. Smart queries invoke the contract's
`query` entry point with a contract-defined JSON message. The
[query service](../../wasm/x/wasm/keeper/querier.go) validates requests and
applies the configured smart-query gas limit.

Contract-to-contract Wasm queries use the same keeper query paths. Their
message and response schemas belong to the queried contract.

## Contract upgrades

Uploaded bytecode and mutable contract state are separate. Migration changes
the code ID associated with an existing contract and invokes the new code's
`migrate` entry point against its storage. Ordinary migration requires the
contract admin; enabled governance proposals use the privileged authorization
policy. See [Governance](../../wasm/x/wasm/Governance.md).

Contract messages and host query formats are JSON interfaces. The adapter
translates between those types and SDK types, so contract interfaces can stay
independent of SDK representations. A migrated contract must interpret its
existing state and the messages used by its callers.

## Genesis and recovery

[`genesis.go`](../../wasm/x/wasm/keeper/genesis.go) exports parameters,
uploaded code, contract metadata, state and sequence values.
Initialization restores those values and checks their consistency. Contract
state is stored in chain stores; the VM manages bytecode and compiled caches.
On loading the latest chain state, the node calls `InitializePinnedCodes` to
restore the pinned code cache.

IBC-enabled contracts also have a persistent port association. See the
[IBC guide](../../wasm/x/wasm/IBC.md) for capability recovery and packet
callbacks.
