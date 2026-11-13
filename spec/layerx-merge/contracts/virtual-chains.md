# Virtual chains contract

Feature `20` defines a virtual chain as one registry record with shared Paxeer security and EVM state. This document freezes the contract cited by the feature's acceptance blocks; task status and qualification remain in [spec.kvx](../spec.kvx). The [operator and builder guide](../../../docs/wiki/VirtualChains.md) explains the user journey. Defining this contract does not assert that its pending runtime or client tasks are deployed.

## registry

Requirement `20001.1`; task `20.1.1`.

The `layerxchains` store contains records of type:

```text
Domain {
  Id uint32; Name string; Operator common.Address; Status uint8;
  GasAllowance uint64; ByteAllowance uint64; FeeShareBps uint16;
  Bond sdk.Int; TargetGasPerBlock uint64; CreatedHeight int64;
}
```

Status codes are `1 active`, `2 paused`, `3 governance_paused`, `4 retired`. IDs start at 1 and fit `uint32`. Records are never deleted. Names contain 3 to 32 characters of lowercase `a-z`, digits and hyphen, with no leading or trailing hyphen. Names remain unique among every record, including retired ones; `chainByName` returns zero only for an unused name.

```text
VirtualChainIDBase = 125000000
ChainIDOf(id) = VirtualChainIDBase + id
DomainOfChainID(125) = 0 (the root network)
DomainOfChainID(VirtualChainIDBase + id) = id for id >= 1
```

The virtual mapping is limited to nonzero `uint32` IDs; every other value is unknown. In particular the base value itself is not a virtual chain. The legacy chain-ID-0 ante exception is separate from this mapping. A derived ID requires a registered, active record for transaction admission.

`Create`, `Fund`, `Pause`, `Resume`, `GovernancePause`, `Retire` and `SetOperator` update the record through the keeper. Create validates name, uniqueness, parameter ranges, remaining active allowance, maximum records and payment. The creation fee moves to the fee collector, the remaining value becomes bond held by the module account, and `TargetGasPerBlock = GasAllowance / 2`. Funding increases the bond. Retirement returns the bond to the operator and preserves the record and name. An operator cannot resume a governance-paused record.

The sum of active gas allowances never exceeds `MaxTotalGasAllowance`. Defaults are `MinBond = 100 PAX`, `CreationFee = 10 PAX`, gas range 250000 to 4000000, total gas allowance 6250000, byte range 16384 to 1048576, `MaxChains = 1024`, and `MaxFeeShareBps = 5000`. Monetary parameters use base units. Live parameters govern admission and payment; defaults are not a fixed launch quote.
The keeper stores `MinBond` and `CreationFee` in bank base units (10^6 per PAX), and the precompile reports them in wei.

Genesis imports and exports parameters, all records, the next ID and next base fees. Validation rejects duplicate IDs or names, a next ID not above every record, and excessive active allowance. Round-trip qualification must preserve retired records as well as active ones.

## wiring

Requirement `20001.2`; task `20.1.2`.

The application mounts `LayerXChainsStoreKey = layerxchains`, including `MemIAVLStoreKeys`, constructs `LayerXChainsKeeper` with bank access and the EVM base-denomination source, and registers the basic and application modules. The module appears after `layerxpay` in init-genesis order and participates in the begin-block, end-block and export orders. The merge store upgrade adds its store and initializes default genesis. Application-level export followed by import must reproduce a created chain; a keeper-only round trip is insufficient evidence for wiring.

## abi

Requirement `20002.2`; task `20.2.1`.

The `layerxchains` precompile address is **`0x000000000000000000000000000000000000101E`**. Its complete `ILayerXChains` ABI is:

```solidity
function createChain(string name,uint64 gasAllowance,uint64 byteAllowance,uint16 feeShareBps) external payable returns (uint32 id);
function fundChain(uint32 id) external payable;
function pauseChain(uint32 id) external;
function resumeChain(uint32 id) external;
function retireChain(uint32 id) external;
function governancePause(uint32 id,bool paused) external;
function setOperator(uint32 id,address operator) external;
function getChain(uint32 id) external view returns (uint32 id,string name,address operator,uint8 status,uint64 chainId,uint64 gasAllowance,uint64 byteAllowance,uint16 feeShareBps,uint256 bond,uint256 baseFeePerGas,uint64 createdHeight);
function chainByName(string name) external view returns (uint32); // Zero for an unused name; retained ID for any existing record, including retired chains.
function chainCount() external view returns (uint32);
function chainIdOf(uint32 id) external pure returns (uint64);
function params() external view returns (address authority,uint256 minBond,uint256 creationFee,uint64 minGasAllowance,uint64 maxGasAllowance,uint64 totalGasAllowanceLeft,uint32 maxChains);

event ChainCreated(uint32 indexed id,string name,address indexed operator,uint64 chainId);
event ChainStatusChanged(uint32 indexed id,uint8 status);
event ChainFunded(uint32 indexed id,address indexed from,uint256 amount);
event ChainOperatorChanged(uint32 indexed id,address indexed operator);
```

`precompiles/layerxchains/abi.json` and `LayerXChains.sol` must agree on every signature, selector and event topic. Every method must have a gas-table entry and every gas-table entry must name a method: create 60000, fund 30000, other writes 20000 each, and reads including the pure derivation 3000 each.

`tests/vectors/chains/abi-vectors.json` is generated from that ABI. It carries the address; every method's name, signature, selector, arguments and calldata; every event's name, signature and topic0; and encoded plus decoded `getChain` and `params` examples. Example calls include `createChain('demo', 500000, 65536, 100)`, ID `1` for ID-taking methods, `governancePause(1, true)`, `chainByName('demo')` and operator `0x1111111111111111111111111111111111111111`. The `getChain` example returns ID 1, name demo, that operator, status 1, chain ID 125000001, gas 500000, bytes 65536, share 100, bond 100000000000000000000, base fee 1000000000 and height 42. SDKs reproduce the generated bytes rather than defining independent hex fixtures.

## precompile

Requirement `20002.1`; task `20.2.2`.

The precompile is the only external write path and the complete registry read path. `RegisterMergeConstructor` registers its constructor; `MergeAddressTable` lists its address and `layerxchains`; `mergePrecompileGate` gates on the module name. Application keepers expose `LayerXChainsK` to the constructor. Execution applies the method gas table and maps keeper failures to LXP reasons.

`createChain` takes `msg.value` as bond plus creation fee and refuses less than `minBond + creationFee`. `fundChain` takes its payable value as additional bond. `pauseChain`, `resumeChain`, `retireChain` and `setOperator` accept only the current operator; `governancePause` accepts only the parameter authority. Every write emits its corresponding event: creation, funding, status change or operator change. The served EVM-call path must prove create, emitted receipt logs, readback and permission refusals, rather than calling the keeper directly for the assertions.

## admission

Requirements `20003.1` and `20003.2`; tasks `20.3.1` and `20.3.2`.

The ante handler resolves signed chain IDs, permits registered active records, and places the domain ID in context for the fee decorator. Unknown IDs refuse with `ErrInvalidChainID`; paused, governance-paused and retired records refuse with `ErrDomainNotActive`. Root chain ID 125 and legacy zero retain their existing behavior. Chain fee caps are checked against that chain's next base fee.

Block admission tracks gas and bytes per domain. Accumulated chain gas plus the incoming transaction's gas limit must stay within `GasAllowance`, and encoded bytes within `ByteAllowance`. A refusal identifies the chain, gas wanted and allowance. Shared block totals still apply; root transactions retain the whole `Block.MaxGas` without a virtual-chain allowance. Unknown or inactive domains are not admitted.

EVM state is shared. A virtual chain is a namespace for identity, fees and block space, not a trust boundary. Neither admission nor the RPC path creates an independent state tree, contract deployment namespace, private account balance or validator set.

## fees

Requirements `20004.1` and `20004.2`; tasks `20.3.3` and `20.3.4`.

Every executed chain transaction records its hash and actual gas used in that chain's per-height index. End-block adjusts each active chain's next base fee with the EIP-1559 rule of `modules/evm/keeper/fee.go`, using `GasAllowance` as its limit and `TargetGasPerBlock` as target, bounded by the EVM minimum and maximum. Busy-chain utilization raises its fee, idle utilization lowers its fee within bounds, and the root fee follows root traffic alone.

The operator share is `floor(baseFeePerGas * gasUsed * FeeShareBps / 10000)`, using the chain's base fee before adjustment. It transfers in the base denomination from the fee collector to the operator when the collector holds the funds. Zero basis points transfers nothing; 100 basis points is 1%. This is a share of base fees, separate from the bond and creation fee.

Qualification executes root and chain transactions in one block, refuses an over-allowance chain transaction, observes the affected fees and operator bank balance, and reopens the same database to prove preservation of the registry record, next base fee and per-height transaction index.

## rpc

Requirements `20005.1` and `20005.2`; tasks `20.4.1` through `20.4.4`.

Both HTTP and WebSocket endpoints serve `/chain/<id>` for a decimal registry ID, with an optional trailing slash. A resolver uses the latest chain record to place `DomainScope` in the request context. Unknown and retired IDs return 404 `{"error":"unknown_chain"}`; paused records remain readable. The root path remains unchanged and reports chain ID 125.

On `/chain/1`, `eth_chainId` reports hexadecimal `0x7735941` and `net_version` reports decimal `125000001`. Blocks retain the shared hash, number, parent and header fields while listing only the scoped chain's transactions and receipts. Block transaction counts by hash or number count only those transactions. `eth_getTransactionByHash` and `eth_getTransactionReceipt` return `null` for another chain's transaction. `eth_getLogs`, installed filters and WebSocket log subscriptions restrict results to the chain's transactions. `newHeads` remains the shared header stream. Root reads retain the full transaction view.

`eth_sendRawTransaction` keeps root submission semantics. The signed chain ID chooses the execution domain, so a transaction signed with 125 sent through `/chain/1` is still a root transaction. Account, balance, code and storage reads remain shared EVM reads. A standard go-ethereum `ethclient` must be able to dial the scoped path, submit a signed chain transaction, receive its receipt and filter its logs without a client fork.

## indexer

Requirement `20006.1`; tasks `20.5.1` and `20.5.2`.

The hosted indexer loads the four registry events from the precompile address; `LAYERX_INDEXER_CHAINS_PRECOMPILE` defaults to that address. Creation inserts a chain; status, funding and operator events update it. The `chains` table retains ID, unique name, operator, status, derived chain ID, creation height, bond, gas allowance, byte allowance and fee share. Status strings are `active`, `paused`, `governance_paused` and `retired`.

`GET /v1/chains` returns records sorted by ID in this shape:

```json
{"chains":[{"id":1,"name":"demo","operator":"0x1111111111111111111111111111111111111111","status":"active","chain_id":125000001,"created_height":42,"bond":"100000000000000000000","gas_allowance":500000,"byte_allowance":65536,"fee_share_bps":100}]}
```

`GET /v1/chains/{id}` and `GET /v1/chains/by-name/{name}` return one record or 404 `{"error":"unknown_chain"}`. Bond uses a base-unit decimal string. The signed transaction chain ID determines `pax:<id>` on transfers and events; root rows keep their existing chain value. Receipt gas usage feeds per-chain activity accounting.

`GET /v1/chains/{id}/summary` returns `{"id":1,"tx_count":N,"gas_used":N,"last_height":N}` with numeric values in place of `N`. `/v1/chains/{id}/transfers` and `/v1/chains/{id}/events` return pages with the `/v1/history` shape restricted to `pax:<id>`. Unknown chain summaries return 404. Qualification covers receipt-log ingestion through the HTTP response and signed chain ID through row namespace to query results.

## explorer

Requirement `20007.1`; tasks `20.6.1` and `20.6.2`.

The explorer's **Chains** navigation opens `/chains`, backed by the indexer API. It validates records, labels every lifecycle status and shows name, chain ID, operator, status, allowance and created height. Its display ordering is active first, then ID; malformed records are refused.

`/chains/<id>` shows the chain record, summary, recent transfers and events, copyable `<rpcBase>/chain/<id>`, a launchpad link and **Add to wallet**. The action invokes `wallet_addEthereumChain` with the derived hexadecimal chain ID, chain name `<name> on Paxeer`, native currency `{name: PAX, symbol: PAX, decimals: 18}`, the scoped RPC URL and `<explorerBase>/chains/<id>` as its explorer URL.

## wallet

Requirement `20007.1`; tasks `20.8.1` and `20.8.2`.

Networks carry chain ID, name, RPC URL, explorer URL and kind (`root` or `virtual`). The root comes from `PAXEER_CONFIG`. Virtual IDs satisfy `125000000 < chainId <= 125000000 + 4294967295`; their registry ID is `chainId - VirtualChainIDBase`. Saved preferences accept at most 32 virtual networks with validated HTTPS URLs and an optional active chain ID; older preference records migrate without losing existing values.

`wallet_addEthereumChain` saves a valid virtual network; `wallet_switchEthereumChain` selects a saved network. Settings expose the network switcher to add by registry ID or explorer chain URL, switch, and remove saved networks. Adding checks `eth_chainId` on the derived RPC URL and refuses a mismatch. Switching stores the active ID and fires `RPC_CHANGED_EVENT`; `getActiveRpcUrl` and transfer explorer links follow the selected network. Absent a saved active virtual network, root selection follows the existing configuration.

## launch

Requirement `20008.1`; tasks `20.7.1` through `20.7.3`.

`/create/chain` presents **Launch a chain** in three clicks: name the chain, pick a size tier, confirm one wallet transaction. Nothing else is installed, hosted or configured. Name validation enforces the registry rule and checks `chainByName(name) == 0`.

| Tier | Gas per block | Bytes per block |
| --- | --- | --- |
| starter | 500000 | 65536 |
| growth | 1500000 | 262144 |
| pro | 4000000 | 1048576 |

The draft records its creator, root chain ID 125, name, tier and fee share (0 to 5000 basis points), and expires after 30 days. Cost derives bond and fee from `params()`, rejects tiers outside the permitted gas range or remaining capacity, and separates transaction gas from payable bond plus fee.

The adapter checks wallet account and chain 125, submits exactly one `createChain` transaction, waits for its receipt, decodes `ChainCreated`, then reads the registry record through `getChain`. A taken name or changed wallet blocks launch. Progress covers wallet confirmation, waiting for the block and readiness. The ready panel includes registry ID, derived chain ID, copyable RPC URL, explorer link and the EIP-3085 add-to-wallet action described above. Draft persistence and server rendering followed by hydration preserve the journey.

The launchpad's combined market and chain flow is specified in [chain-launchpad.md](chain-launchpad.md). Its registry number uses this contract's chain ID and scoped RPC rules; direct `createChain` remains the registry call described above.

## sdk

Requirement `20009.1`; tasks `20.9.1` through `20.9.8`.

Every SDK encodes all twelve ABI methods, decodes the `getChain` and `params` tuples and all four events, and derives `virtualChainId(id) = 125000000 + id` and `chainRpcUrl(base, id) = <base>/chain/<id>`. Qualification reads the precompile-generated `tests/vectors/chains/abi-vectors.json` and compares selectors, example calldata, topic0 and decoded tuples byte for byte, then round-trips encoding and decoding.

| SDK | Repository entry point | Read/client entry |
| --- | --- | --- |
| TypeScript | `services/agent/sdk/typescript/src/chains.ts`, re-exported from `index.ts` | `ChainsClient` over `JsonRpcClient` |
| Python | `services/agent/sdk/python/layerx_sdk/chains.py`, exported through `__init__.py` and its stub | `ChainsClient` over the JSON-RPC helper used by `px.py` |
| Go | `sdk/platform/go/chains.go` | `ChainsClient`, `EncodeChainsCall`, `DecodeGetChain`, `DecodeChainsParams`, `DecodeChainsEvent` |
| JVM | `sdk/platform/jvm/src/main/java/com/sidiora/layerx/sdk/Chains.java` | `encodeCall`, `decodeGetChain`, `decodeParams`, `decodeEvent` |
| .NET | `sdk/platform/dotnet/Chains.cs` | `EncodeCall`, `DecodeGetChain`, `DecodeParams`, `DecodeEvent` |
| Swift | `sdk/platform/swift/Sources/LayerXSdk/Chains.swift` | `encodeCall`, `decodeGetChain`, `decodeParams`, `decodeEvent` |
| Rust | `services/agent/crates/layerx-client/src/chains.rs`, declared in `lib.rs` | `ChainsReads`, `encode_call`, `decode_get_chain`, `decode_params`, `decode_event` |

Read clients use `eth_call` to `getChain`, `chainByName`, `chainCount` and `params`. Creation uses the existing signed type-2 transaction builders. The platform CLI module `tools/platform/cli/src/chains.rs` exposes `lx chains list`, `lx chains show <id|name>` and `lx chains create --name <name> --tier starter|growth|pro --fee-share-bps <bps>`. It reads current parameters, signs bond-plus-fee creation with the configured wallet and prints the `ChainCreated` registry ID, derived chain ID and RPC URL.

## closure

Requirement `20010.1`; tasks `20.0.1` and `20.10.1`.

Feature 20 contains 31 tasks, including its closure and documentation tasks. `chains_closure(spec_path)` in `spec/layerx-merge/programme/chains_closure.py` must parse every `[task.20.*]` block except `20.0.1` and return the sorted IDs whose status is not `done`. Its command-line entry prints that list and exits 1 while it is non-empty, or 0 when it is empty. Any status other than `done` keeps this feature open. Implementing that executable and its closure tests belongs to task `20.0.1`; documentation qualification alone does not close the feature.

`test_virtual_chains_docs_name_every_contract` reads the operator guide and this contract and requires both to name `0x000000000000000000000000000000000000101E`, `125000000`, `/chain/`, every ABI method and every event. It also requires the contracts index to list this file. The guide is linked from the wiki Home and the Paxeer network overview. This is the documentation evidence for acceptance `20.10.1.1`; runtime, client, persistence and programme closure evidence remain the responsibility of their declared tasks.
