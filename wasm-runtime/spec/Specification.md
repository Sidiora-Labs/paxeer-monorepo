# Specification

This page describes the Go API in [`lib.go`](../lib.go) and the JSON types in
[`types`](../types). The [architecture](Architecture.md) explains how the Wasm
keeper supplies chain context and processes responses.

## Code and contract instances

`WasmCode` is a byte slice containing WebAssembly bytecode. `Checksum` identifies
that bytecode. `VM.StoreCode` stores code in the native cache and returns its
checksum. `CreateChecksum` computes the SHA-256 checksum after checking the Wasm
magic bytes; it does not perform the VM's full code validation.

The chain module assigns numeric code IDs and contract addresses. A VM call
selects code by checksum and receives the storage and environment for the
particular contract instance. Multiple instances can share code while retaining
separate state.

## VM entry points

Contract execution requires cgo. Create a VM with:

```go
func NewVM(dataDir string, supportedCapabilities string, memoryLimit uint32,
    printDebug bool, cacheSize uint32) (*VM, error)
```

`dataDir` holds Wasm blobs and caches. `supportedCapabilities` is a
comma-separated list. `memoryLimit` bounds each execution's memory in MiB;
`cacheSize` sets the in-memory module cache size in MiB, with zero disabling it.
`printDebug` controls contract debug output. Call `Cleanup` when the VM is no
longer used.

Code and cache methods include:

| Method | Result |
| --- | --- |
| `StoreCode(code WasmCode)` | `(Checksum, error)` |
| `GetCode(checksum Checksum)` | `(WasmCode, error)` |
| `RemoveCode(checksum Checksum)` | `error` |
| `AnalyzeCode(checksum Checksum)` | `(*types.AnalysisReport, error)` |
| `Pin(checksum Checksum)` | `error` |
| `Unpin(checksum Checksum)` | `error` |
| `GetMetrics()` | `(*types.Metrics, error)` |

The execution methods take a checksum, `types.Env`, their operation-specific
message, `KVStore`, `GoAPI`, `Querier`, `GasMeter`, a `uint64` VM gas limit and
`types.UFraction` deserialization cost. `Instantiate` and `Execute` also take
`types.MessageInfo` immediately after the environment.

| Method | Message argument | Result |
| --- | --- | --- |
| `Instantiate` | `initMsg []byte` | `(*types.Response, uint64, error)` |
| `Execute` | `executeMsg []byte` | `(*types.Response, uint64, error)` |
| `Query` | `queryMsg []byte` | `([]byte, uint64, error)` |
| `Migrate` | `migrateMsg []byte` | `(*types.Response, uint64, error)` |
| `Sudo` | `sudoMsg []byte` | `(*types.Response, uint64, error)` |
| `Reply` | `reply types.Reply` | `(*types.Response, uint64, error)` |

The second return value reports gas used internally by the VM. Host callbacks
account for external work through the gas meter. Response deserialization is
also charged. The keeper translates VM gas into SDK gas. `Query` returns the
contract's response bytes; it takes a query message without a separate path
argument.

## Serialization and environment

The wrapper JSON-encodes the environment, message information and typed reply
before calling the native bridge. Contract-defined messages are supplied as
JSON bytes. The definitions in [`env.go`](../types/env.go) are:

```go
type Env struct {
    Block       BlockInfo        `json:"block"`
    Transaction *TransactionInfo `json:"transaction"`
    Contract    ContractInfo     `json:"contract"`
}

type BlockInfo struct {
    Height  uint64 `json:"height"`
    Time    uint64 `json:"time,string"`
    ChainID string `json:"chain_id"`
}

type TransactionInfo struct {
    Index uint32 `json:"index"`
}

type ContractInfo struct {
    Address HumanAddress `json:"address"`
}

type MessageInfo struct {
    Sender HumanAddress `json:"sender"`
    Funds  Coins        `json:"funds"`
}
```

Block time is a decimal string of nanoseconds since the Unix epoch. Transaction
indices start at zero. An unavailable transaction index is represented by a
null `transaction`. `ChainID` is the Cosmos chain identifier from the SDK
context. `HumanAddress` is a string; the module supplies Bech32 addresses.

[`Coin`](../types/types.go) contains `denom` and `amount`, both strings.
`denom` names the denomination and `amount` is its integer quantity. `Coins`
encodes an empty collection as `[]`. Contract balances are available through
bank queries.

## Results and dispatched messages

[`ContractResult`](../types/msg.go) contains either an `ok` response or an
`error` string. A successful response has `messages`, `data`, `attributes` and
`events`. `data` is a byte slice, represented as base64 in JSON; attributes
have string `key` and `value` fields. Custom events add a string `type` and
attributes.

Each response message is a [`SubMsg`](../types/submessages.go):

```go
type SubMsg struct {
    ID       uint64    `json:"id"`
    Msg      CosmosMsg `json:"msg"`
    GasLimit *uint64   `json:"gas_limit,omitempty"`
    ReplyOn  replyOn   `json:"reply_on"`
}
```

The `replyOn` type is private to the types package; exported constants
`ReplyAlways`, `ReplySuccess`, `ReplyError` and `ReplyNever` encode as
`"always"`, `"success"`, `"error"` and `"never"`. `Reply` carries the original
`id` and a result containing either `ok` with events and optional data, or
`error`. The keeper dispatches the submessage and invokes `reply` when its
policy matches the result.

`CosmosMsg` defines the variants `bank`, `custom`, `distribution`, `gov`, `ibc`,
`staking`, `stargate` and `wasm`. Populate the variant for the intended action.
For example, bank sends use `bank.send` with `to_address` and `amount`; contract
execution uses `wasm.execute` with `contract_addr`, `msg` and `funds`. Support
for a variant depends on the chain's registered message handlers.

## State access and host callbacks

[`KVStore`](../types/store.go) exposes `Get`, `Set`, `Delete`, `Iterator` and
`ReverseIterator`. Iterators use an inclusive start and exclusive end. Callers
must check `Valid`, close iterators and treat returned keys and values as
read-only. The keeper supplies storage prefixed for the selected contract.

[`GoAPI`](../types/api.go) contains `HumanAddress` and `CanonicalAddress`
callbacks. They convert between printable and canonical addresses and return
the gas cost with the value or error. [`GasMeter`](../types/gas.go) exposes
`GasConsumed() uint64` to the native bridge.

## Queries

[`Querier`](../types/queries.go) exposes:

```go
Query(request QueryRequest, gasLimit uint64) ([]byte, error)
GasConsumed() uint64
```

`QueryRequest` defines the `bank`, `custom`, `ibc`, `staking`, `distribution`,
`stargate` and `wasm` variants. Responses use the selected query's schema.
For example, a bank balance request contains `bank.balance.address` and
`bank.balance.denom`; its response has `amount` containing a `Coin`.

The module's [query plugins](../../wasm/x/wasm/keeper/query_plugins.go) determine
which variants are handled. The default bank plugin supports balance and
all-balances queries. Default IBC, staking and Wasm plugins provide their
implemented query variants. The default custom and Stargate handlers return
unsupported-request errors. Queries run with a gas bound and discard state
changes in their cached SDK context.

## IBC callbacks

The wrapper also exposes `IBCChannelOpen`, `IBCChannelConnect`,
`IBCChannelClose`, `IBCPacketReceive`, `IBCPacketAck` and `IBCPacketTimeout`.
Their message and result types are declared in [`ibc.go`](../types/ibc.go).
See the [IBC guide](../../wasm/x/wasm/IBC.md) for channel versions, packet
encoding and acknowledgement handling.
