<!-- Source: https://docs.paxeer.app/evm/ -->

# EVM Module

Native EVM execution, address association, receipts, pointers, and precompile integration on chain ID 125.

**Source:** `Paxeer-X-Network/modules/evm/`

## Overview

The EVM module (`modules/evm/`) provides full Ethereum Virtual Machine execution on Paxeer. It runs as a Cosmos SDK module and integrates with the [execution engine](https://docs.paxeer.app/engine) to process EVM transactions with chain ID **125**.

The module handles:

- **EVM Transaction Processing:** Execute Ethereum-format transactions

- **Address Association:** Map Cosmos bech32 addresses to EVM hex addresses

- **Receipt Generation:** Create transaction receipts with logs and status

- **Pointers:** Link ERC-20 tokens to native Cosmos denoms

- **Precompiles:** Expose Cosmos modules via EVM precompiled contracts

## Message Types

The EVM module handles `MsgEVMTransaction`, which wraps a standard Ethereum transaction:

- Legacy transactions (type 0)

- EIP-2930 access list transactions (type 1)

- EIP-1559 dynamic fee transactions (type 2)

- EIP-4844 blob transactions (type 3), accepted from Cancun

- EIP-7702 set-code transactions (type 4), accepted from Prague

Cancun and Prague both activate at time 0 in the chain configuration, so all five types are accepted. `MsgEVMTransaction` also has an associate-transaction variant (`AssociateTx`, reported by `IsAssociateTx`) used for address association.

Transactions are submitted via the JSON-RPC `eth_sendRawTransaction` endpoint or the Cosmos SDK `MsgEVMTransaction` message.

**Source:** `Paxeer-X-Network/modules/evm/ante/preprocess.go`, `modules/evm/types/codec.go`, `modules/evm/types/message_evm_transaction.go`, `modules/evm/types/config.go`

## Address Association

**Source:** `Paxeer-X-Network/modules/evm/keeper/address.go`

Paxeer addresses exist in two formats:

- **Cosmos:** `pax1...` (bech32)

- **EVM:** `0x...` (hex, 20 bytes)

The EVM module maintains a bidirectional mapping between these formats. When a Cosmos account sends an EVM transaction, the module associates its `pax1...` address with the derived `0x...` address. This allows:

- Cosmos accounts to interact with EVM contracts

- EVM accounts to interact with native Cosmos denominations

- Unified balance queries across both address formats

## State Management

**Source:** `Paxeer-X-Network/modules/evm/keeper/state.go`

The EVM module stores:

- **Accounts:** Nonce, balance (via bank module)

- **Code:** Contract bytecode (`keeper/code.go`)

- **Storage:** Contract storage slots (`keeper/state.go`)

- **Receipts:** Transaction receipts and logs (`keeper/receipt.go`)

EVM state is stored in the module's KVStore. Account balances are managed by the bank module to maintain consistency with native token operations.

## Transaction Execution Flow

When an EVM transaction is submitted:

1. RPC receives `eth_sendRawTransaction` or SDK receives `MsgEVMTransaction`
2. Ante decorators under `modules/evm/ante/` validate signatures, nonces, and transaction fees
3. Msg server routes to EVM module
4. Execution runs through the [engine executor](https://docs.paxeer.app/engine); on the node Giga path this is `ExecuteTransactionFeeCharged`, with fees already charged
5. Executor runs EVM bytecode against state DB
6. Module generates receipt (`keeper/receipt.go`) with logs, gas used, status
7. Module emits events for indexing

## Receipts

**Source:** `Paxeer-X-Network/modules/evm/keeper/receipt.go`

Transaction receipts include:

- **Status:** 1 (success) or 0 (revert)

- **Gas Used:** Total gas consumed

- **Logs:** Event logs emitted during execution

- **Contract Address:** For contract creation transactions

- **Bloom Filter:** For efficient log filtering

Execution stages receipts in a transient store and flushes them to the configured receipt store. `GetReceipt` delegates to that store; JSON-RPC exposes them through `eth_getTransactionReceipt`.

## Pointers

Pointers are a Paxeer-specific feature that links ERC-20 token contracts to native Cosmos denoms. A pointer allows:

- ERC-20 `transfer` to move native tokens via bank module

- Native token transfers to appear as ERC-20 events

- Unified balance view across EVM and Cosmos APIs

Pointer contracts are deployed via the [pointer precompile](https://docs.paxeer.app/precompiles).

## Precompile Integration

**Source:** `Paxeer-X-Network/modules/evm/keeper/precompile.go`, `Paxeer-X-Network/precompiles/`

The EVM module registers custom precompiled contracts that expose Cosmos SDK modules to EVM callers. Paxeer-specific addresses are registered by the precompile package, including bank at `0x1001` and address conversion at `0x1004`. Standard Ethereum precompiles occupy a separate address range.

See [Precompiles documentation](https://docs.paxeer.app/precompiles) for the full list of Paxeer precompiles (bank, staking, oracle, IBC, etc.).

## Fee Collection

**Source:** `Paxeer-X-Network/modules/evm/keeper/fee.go`, `Paxeer-X-Network/precompiles/feetoken/feetoken.go`

EVM transaction fees are:

- Charged in PAX (native gas token) by default. An account can select an allowed alternative fee denom, such as SID, through the fee-token precompile at `0x0000000000000000000000000000000000001018`. See [Sidiora](https://docs.paxeer.app/sidiora)

- Reported using receipt gas usage and the effective gas price; estimation and charge settlement depend on the configured execution path

- Sent to the fee collector module

- Distributed to validators and stakers

EIP-1559 dynamic fees are supported with `baseFee` and priority fees.

## Coinbase

**Source:** `Paxeer-X-Network/modules/evm/keeper/coinbase.go`, `keeper.go`

The EVM block context sets `COINBASE` to the fee collector module’s associated EVM address through `GetFeeCollectorAddress`.

## Chain ID

Paxeer uses **EVM chain ID 125**. This must match the `chainId` field in EIP-155 transaction signatures for replay protection. The Cosmos chain identifier is `hyperpax_125-1`.

## Configuration

**Source:** `Paxeer-X-Network/modules/evm/types/params.go`, `types/config.go`, `keeper/params.go`

EVM module parameters include:

- **Native units:** PAX is the display currency; the implementation retains `uhpx` as its Cosmos base denomination and converts it to 18-decimal EVM units

- **Fee bounds:** Minimum and maximum fees per gas, target block gas, and dynamic adjustment parameters

- **Storage gas:** The default SSTORE set cost is 20,000 gas. Query the network’s EVM parameters when calculating costs for a deployment

- **Chain Config:** Fork activation times, including Cancun and Prague, and a configurable SSTORE cost

## Genesis

**Source:** `Paxeer-X-Network/modules/evm/keeper/genesis.go`

Genesis state for the EVM module includes:

- Module parameters

- Pre-deployed contract code and storage

- Address associations

## Logging

**Source:** `Paxeer-X-Network/modules/evm/keeper/log.go`

EVM event logs are emitted during transaction execution via the `LOG0`, `LOG1`, `LOG2`, `LOG3`, `LOG4` opcodes. Logs are included in receipts and indexed for querying via `eth_getLogs`.

## Deferred Operations

**Source:** `Paxeer-X-Network/modules/evm/keeper/deferred.go`

Some EVM operations (like balance updates from precompiles) are deferred and batched for commit. This avoids redundant state writes during execution and improves performance.

## Next Steps

- [Use the JSON-RPC API](https://docs.paxeer.app/json-rpc)

- [Explore Paxeer precompiles](https://docs.paxeer.app/precompiles)

- [Deploy contracts](https://docs.paxeer.app/contracts)
