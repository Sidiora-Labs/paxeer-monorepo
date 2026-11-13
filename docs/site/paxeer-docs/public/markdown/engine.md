<!-- Source: https://docs.paxeer.app/engine/ -->

# Engine

Paxeer's EVM execution engine, transaction processing, and state transition management.

**Source:** `Paxeer-X-Network/engine/`

## Overview

The engine package provides EVM transaction execution for Paxeer. It wraps go-ethereum's EVM with Paxeer-specific precompiles, state management, and fee handling. The engine sits between the consensus layer (which orders transactions) and the EVM module (which manages on-chain state).

## Executor

**Source:** `Paxeer-X-Network/engine/executor/`

The executor is the core EVM execution wrapper. It provides two entry points:

### Executor Types

The `Executor` struct in `engine/executor/executor.go` supports two backends:

- **Geth Executor:** Uses go-ethereum's native EVM interpreter

- **Evmone Executor:** Uses the evmone C++ VM via EVMC bindings for performance

```
// From engine/executor/executor.go
type Executor struct {
    evm *vm.EVM
}

func NewGethExecutor(blockCtx vm.BlockContext, stateDB vm.StateDB,
    chainConfig *params.ChainConfig, config vm.Config,
    customPrecompiles map[common.Address]vm.PrecompiledContract) *Executor

func NewEvmoneExecutor(evmoneVM *evmc.VM, blockCtx vm.BlockContext,
    stateDB vm.StateDB, chainConfig *params.ChainConfig, config vm.Config,
    customPrecompiles map[common.Address]vm.PrecompiledContract) *Executor
```

### Transaction Execution

The executor provides two execution modes:

#### Standard Execution

```
func (e *Executor) ExecuteTransaction(tx *types.Transaction,
    sender common.Address, baseFee *big.Int,
    gasPool *core.GasPool) (*core.ExecutionResult, error)
```

Standard execution charges gas fees from the sender, executes the transaction, and refunds unused gas. This path is used for direct EVM transaction submission.

#### Fee-Already-Charged Execution

```
func (e *Executor) ExecuteTransactionFeeCharged(tx *types.Transaction,
    sender common.Address, baseFee *big.Int,
    gasPool *core.GasPool) (*core.ExecutionResult, error)
```

This mode assumes fees were already charged before execution. It skips fee deduction and refund. This is the mode the node Giga execution path uses for `MsgEVMTransaction` transactions.

### Internal Components

**Source:** `Paxeer-X-Network/engine/executor/internal/`

Internal execution utilities:

- **Host context** (`host_context.go`): EVMC host context for evmone integration

- **Interpreter** (`interpreter.go`): EVM opcode interpreter wrapper

- **Message** (`message.go`): internal execution message handling

## Precompiles

**Source:** `Paxeer-X-Network/precompiles/`; `engine/executor/precompiles/failfast.go`

Paxeer-specific precompiled contracts live in the top-level `precompiles/` package and are registered with the executor at initialization. The executor passes the `customPrecompiles` map to the EVM, which routes calls to the registered precompile addresses. The `engine/executor/precompiles/` directory itself contains only the fail-fast wrapper (`failfast.go`) and its tests.

See [Precompiles documentation](https://docs.paxeer.app/precompiles) for the full list of Paxeer precompiles (bank, staking, oracle, pointer, etc.).

## Dependencies (xbank, xevm)

**Source:** `Paxeer-X-Network/engine/deps/`

The engine depends on Cosmos SDK modules via thin wrappers in `engine/deps/`:

### xbank

`engine/deps/xbank/` wraps the SDK bank module for balance operations:

- **Send:** Transfer tokens between accounts

- **View:** Query balances and supply

- **Deferred Cache:** Batch balance updates before commit

### xevm

`engine/deps/xevm/` wraps the EVM module keeper:

- **State:** EVM storage access (accounts, code, storage slots)

- **Code:** Contract bytecode storage and retrieval

- **Receipt:** Transaction receipt generation

- **Address:** Cosmos ↔ EVM address association

- **Nonce:** Account nonce management

- **Fee:** Gas price and fee collection

- **Coinbase:** Block reward recipient

- **Precompile:** Precompile address registration

## Configuration

**Source:** `Paxeer-X-Network/engine/executor/config/`

The Giga executor configuration carries two flags, both defaulting to `true`:

- **`giga_executor.enabled`:** whether the Giga executor is used

- **`giga_executor.occ_enabled`:** whether optimistic concurrency control (OCC) is used with the Giga executor

```
# app.toml
[giga_executor]
enabled = true
occ_enabled = true
```

Chain configuration (chain ID 125, fork activation) and the custom precompile address map are supplied to the executor constructors by the caller. They are not fields of this executor config.

## Testing

**Source:** `Paxeer-X-Network/engine/tests/`

The engine includes comprehensive test suites:

- **State Tests:** `state_test.go` runs Ethereum state transition tests

- **Harness:** `tests/harness/` provides test fixtures and builders

- **Giga Test:** `giga_test.go` for large-scale execution

## Chain ID

The chain configuration maps the Cosmos identifier `hyperpax_125-1` to **EVM chain ID 125**. Transactions on that network must use chain ID 125 for EIP-155 replay protection. Unknown Cosmos identifiers use the separate development fallback chain ID and must not be treated as Paxeer mainnet.

## Utilities

**Source:** `Paxeer-X-Network/engine/executor/utils/`

The utils package (`errors.go`) defines the Giga abort sentinel (`giga_fallback_to_v2`) and `ShouldExecutionAbort`, which reports whether an error should abort Giga execution and fall back to the standard (V2) execution path.

## Integration with EVM Module

EVM transactions arrive as `MsgEVMTransaction` messages. On the Giga execution path, the node application (`node/app.go`) constructs a geth executor with `NewGethExecutor` and calls `ExecuteTransactionFeeCharged`, because fees have already been charged before execution. The flow is:

1. Consensus orders transactions in a block
2. Block processing routes `MsgEVMTransaction` to the EVM execution path
3. Ante processing validates the transaction and charges fees
4. The Giga path constructs the executor and calls `ExecuteTransactionFeeCharged`
5. The executor runs EVM bytecode against the state DB
6. The EVM module generates the receipt and emits events

See [EVM Module documentation](https://docs.paxeer.app/evm) for the full message handling flow.

## Next Steps

[Design for Parallel Execution](https://docs.paxeer.app/parallelization) explains how application storage layout affects contention and how to measure a change without confusing gas usage with network throughput.

- [Understand the EVM module](https://docs.paxeer.app/evm)

- [Review Paxeer precompiles](https://docs.paxeer.app/precompiles)

- [Use the JSON-RPC API](https://docs.paxeer.app/json-rpc)
