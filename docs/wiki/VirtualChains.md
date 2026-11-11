# Virtual chains

A virtual chain is one record in Paxeer's `layerxchains` registry. Creating one starts no process and requires no new infrastructure: it shares Paxeer's validators, security, block history and EVM state. Its record gives it an operator, a name, a chain ID, block allowances and its own base fee.

**Isolation:** EVM state is shared. A chain is a namespace for identity, fees and block space, not a trust boundary. Contracts, account state and balances are shared; choosing a chain RPC path does not create private storage or an independent execution environment.

This guide describes the feature-20 contract. Runtime and client availability follow the individual tasks in [the programme specification](../../spec/layerx-merge/spec.kvx); these instructions do not establish that a particular deployment has enabled the feature. The [normative contract](../../spec/layerx-merge/contracts/virtual-chains.md) defines the integration requirements.

## Launch a market and chain

Open **Launch a chain** in the launchpad at `/create/chain` with a wallet connected to Paxeer's root network (chain ID `125`). The merged flow creates a token, market pool, fee-right NFT and bound virtual chain in one payable `launchChain` transaction. The wallet supplies at least the current registry minimum bond plus creation fee as `msg.value`; market charges and transaction gas are separate. The receipt's `ChainLaunched` event identifies the new registry number and derived chain ID. The [chain launch guide](../launchpad/chain-launch.md) covers the transaction, home-chain trading, revenue claims and chain controls.

Choose a chain name of 3 to 32 lowercase letters, digits or hyphens, without a leading or trailing hyphen. `chainByName(name)` returns zero for an unused name; retired names remain reserved. The wizard reads the current bond, creation fee and capacity from `params()`. A size tier must fit the permitted gas range and remaining total allowance. After confirmation, use the registry number to form `<rpcBase>/chain/<id>` and set the wallet to EVM chain ID `125000000 + id` before trading the launched market.

The registry's direct `createChain` call remains available to create a chain without a launchpad market binding. Its `ChainCreated` event and `getChain` readback expose the registry record described in the ABI below.

| Tier | Gas allowance per block | Byte allowance per block |
| --- | --- | --- |
| starter | 500000 | 65536 |
| growth | 1500000 | 262144 |
| pro | 4000000 | 1048576 |

These tiers choose allowances; they do not buy separate validators. Default parameters set a minimum bond of 100 PAX and a creation fee of 10 PAX. Read `params()` for the current values rather than using those defaults as a quote. `msg.value` must cover at least `minBond + creationFee`; the creation fee goes to the fee collector and the remainder becomes the chain's bond. `fundChain` adds to that bond. Retirement returns the bond to the operator while retaining the record and its name.

## Address and chain IDs

The registry precompile is `layerxchains` at **`0x000000000000000000000000000000000000101E`**.

```text
VirtualChainIDBase = 125000000
ChainIDOf(id) = VirtualChainIDBase + id
DomainOfChainID(125) = 0 (the root network)
DomainOfChainID(VirtualChainIDBase + id) = id for id >= 1
```

Registry IDs are nonzero `uint32` values, from 1 through 4294967295. Every other value is unknown to this mapping, including `125000000` itself. The ante handler separately preserves legacy chain-ID-0 transaction behavior; that exception does not make zero a virtual chain ID.

For example, registry ID `1` has EVM chain ID `125000001` (`0x7735941`) and uses `<rpcBase>/chain/1`. The URL takes the registry ID, not the derived EVM chain ID. Derivation alone does not prove that a record exists or is active.

## Status and operator actions

| Code | Status | Meaning |
| --- | --- | --- |
| 1 | `active` | Chain transactions may be admitted within its allowances. |
| 2 | `paused` | Operator-paused; no chain transaction admission. |
| 3 | `governance_paused` | Authority-paused; the operator cannot resume it. |
| 4 | `retired` | Terminal record; no chain transaction admission and the name remains reserved. |

Only the current operator can call `pauseChain`, `resumeChain`, `retireChain` and `setOperator`. Only the `authority` returned by `params()` can call `governancePause`. Funding records the payer in `ChainFunded`; it does not grant operator rights. Changing the operator emits `ChainOperatorChanged`.

Paused records remain readable on their chain RPC path. An unknown or retired registry ID has no chain RPC path and returns HTTP 404 with `{"error":"unknown_chain"}`; its registry record remains available through the root registry interface.

## Complete registry ABI

The following declarations are the `ILayerXChains` interface. Tuple ordering and integer widths are part of the contract.

```solidity
function createChain(string name,uint64 gasAllowance,uint64 byteAllowance,uint16 feeShareBps) external payable returns (uint32 id);
function fundChain(uint32 id) external payable;
function pauseChain(uint32 id) external;
function resumeChain(uint32 id) external;
function retireChain(uint32 id) external;
function governancePause(uint32 id,bool paused) external;
function setOperator(uint32 id,address operator) external;
function getChain(uint32 id) external view returns (uint32 id,string name,address operator,uint8 status,uint64 chainId,uint64 gasAllowance,uint64 byteAllowance,uint16 feeShareBps,uint256 bond,uint256 baseFeePerGas,uint64 createdHeight);
function chainByName(string name) external view returns (uint32);
function chainCount() external view returns (uint32);
function chainIdOf(uint32 id) external pure returns (uint64);
function params() external view returns (address authority,uint256 minBond,uint256 creationFee,uint64 minGasAllowance,uint64 maxGasAllowance,uint64 totalGasAllowanceLeft,uint32 maxChains);

event ChainCreated(uint32 indexed id,string name,address indexed operator,uint64 chainId);
event ChainStatusChanged(uint32 indexed id,uint8 status);
event ChainFunded(uint32 indexed id,address indexed from,uint256 amount);
event ChainOperatorChanged(uint32 indexed id,address indexed operator);
```

Read calls use `eth_call` against the precompile. Writes use signed EVM transactions, with value for the two payable methods. `chainByName` also finds retired records, `chainCount` includes retained records, and `chainIdOf` is pure arithmetic rather than an existence check. Use `getChain` to inspect the current status, allowances, bond and base fee.

## Allowances and fees

Admission counts gas and encoded bytes for each chain in each shared block. The chain's accumulated gas usage plus the incoming transaction's gas limit must fit `gasAllowance`; its bytes must fit `byteAllowance`. Admission refuses unknown, paused, governance-paused and retired chains. A gas refusal identifies the chain, gas wanted and allowance. The sum of active gas allowances cannot exceed `MaxTotalGasAllowance`; normal shared block limits still apply. Root transactions retain the whole `Block.MaxGas` limit.

Each chain's base fee uses its own executed gas usage and the EIP-1559 adjustment in `modules/evm/keeper/fee.go`, with its gas allowance as the limit and `TargetGasPerBlock = gasAllowance / 2` as the target. The next fee stays within the EVM minimum and maximum. A chain transaction must meet that chain's fee, even if the root fee is lower. Chain traffic does not drive the root base fee. The registry, next base fee and per-height transaction index persist across a node restart.

At end of block the operator receives the truncated share of base fees:

```text
operator share = floor(baseFeePerGas * gasUsed * feeShareBps / 10000)
```

The calculation uses the chain's fee before the next-block adjustment and transfers from the fee collector in the base denomination. A share of `100` basis points is 1%; zero pays no share. The default maximum is `5000` basis points. The launch draft accepts 0 to 5000; live module parameters also constrain creation. This share concerns base fees, not the creation fee or a promise of revenue from unused allowance.

## RPC scope

Use `/chain/<id>` on both the HTTP and WebSocket RPC base URLs. A trailing slash is accepted. On `/chain/1`, `eth_chainId` returns `0x7735941` and `net_version` returns `125000001`; the root path retains chain ID `125`.

| Surface | View on the chain path |
| --- | --- |
| Blocks and block receipts | Only this chain's transactions and receipts; shared block hash, number, parent and header identity are preserved. |
| Block transaction counts | Count only this chain's transactions, by number or hash. |
| `eth_getTransactionByHash`, `eth_getTransactionReceipt` | Return only this chain's transactions; another chain's transaction returns `null`. |
| `eth_getLogs`, installed log filters and WebSocket log subscriptions | Return only logs from this chain's transactions. |
| WebSocket `newHeads` | Shared block headers, unchanged. |
| `eth_sendRawTransaction` | Same submission behavior as the root path; the signed chain ID determines admission and accounting. |
| Account, balance, code and storage reads | Shared EVM state; the path does not create a state boundary. |

Sending a transaction signed with `125` through `/chain/1` still submits a root transaction; the path does not rewrite its signature or domain. Standard Ethereum clients, including go-ethereum `ethclient`, use the chain URL without a client fork. The root path retains the full transaction view.

## Indexer and explorer

The hosted indexer decodes `ChainCreated`, `ChainStatusChanged`, `ChainFunded` and `ChainOperatorChanged` from the registry address into its `chains` table.

| Indexer entry point | Response |
| --- | --- |
| `GET /v1/chains` | `{"chains":[...]}`, sorted by registry ID. |
| `GET /v1/chains/{id}` | One chain record. |
| `GET /v1/chains/by-name/{name}` | One chain record, including a retained name. |
| `GET /v1/chains/{id}/summary` | `id`, `tx_count`, `gas_used` and `last_height`. |
| `GET /v1/chains/{id}/transfers` | A history page restricted to `pax:<id>` rows. |
| `GET /v1/chains/{id}/events` | A history page restricted to `pax:<id>` rows. |

Records expose `id`, `name`, `operator`, `status`, `chain_id`, `created_height`, `bond`, `gas_allowance`, `byte_allowance` and `fee_share_bps`; status strings match the table above and bond is a base-unit decimal string. Unknown single-record lookups return 404 with `{"error":"unknown_chain"}`. The indexer derives transaction namespaces from signed chain IDs and records receipt gas usage; root rows keep their existing namespace.

In the explorer, **Chains** opens `/chains`. The list shows name, chain ID, operator, status, allowances and creation height, with active records first and then ID order. `/chains/<id>` shows the record, summary, recent transfers and events, copyable RPC path, launch link and **Add to wallet** action.

## Wallet and SDK entry points

The launchpad and explorer call EIP-3085 `wallet_addEthereumChain` with the hexadecimal derived chain ID, `<name> on Paxeer`, `rpcUrls: [<rpcBase>/chain/<id>]`, PAX as the native currency (18 decimals), and `blockExplorerUrls: [<explorerBase>/chains/<id>]`.

The wallet network switcher in settings keeps the root network and up to 32 saved virtual networks. Add by registry ID or explorer chain URL, switch networks, or remove a saved entry. The wallet checks the endpoint's `eth_chainId` and rejects a mismatch. `wallet_switchEthereumChain` selects a saved network; the active network supplies the RPC URL and transfer explorer links. Saved custom network URLs must use HTTPS.

Feature 20 supplies the following SDK modules (repository paths):

| SDK | Entry point | Helpers |
| --- | --- | --- |
| TypeScript | `services/agent/sdk/typescript/src/chains.ts` | `ChainsClient`, `virtualChainId`, `chainRpcUrl` |
| Python | `services/agent/sdk/python/layerx_sdk/chains.py` | `ChainsClient`, `virtual_chain_id`, `chain_rpc_url` |
| Go | `sdk/platform/go/chains.go` | `ChainsClient`, `VirtualChainID`, `ChainRPCURL` |
| JVM | `sdk/platform/jvm/src/main/java/com/sidiora/layerx/sdk/Chains.java` | `encodeCall`, `decodeGetChain`, `decodeParams`, `decodeEvent`, `virtualChainId`, `chainRpcUrl` |
| .NET | `sdk/platform/dotnet/Chains.cs` | `EncodeCall`, `DecodeGetChain`, `DecodeParams`, `DecodeEvent`, `VirtualChainId`, `ChainRpcUrl` |
| Swift | `sdk/platform/swift/Sources/LayerXSDK/Chains.swift` | `encodeCall`, `decodeGetChain`, `decodeParams`, `decodeEvent`, `virtualChainId`, `chainRpcUrl` |
| Rust | `services/agent/crates/layerx-client/src/chains.rs` | `ChainsReads`, `encode_call`, `decode_get_chain`, `decode_params`, `decode_event`, `virtual_chain_id`, `chain_rpc_url` |

Every SDK encodes all twelve methods, decodes both return tuples and all four events, and checks selectors, calldata and event topics against `tests/vectors/chains/abi-vectors.json`. That vector file is generated from the precompile ABI, so all languages share one wire contract.

The platform CLI entry points are `lx chains list`, `lx chains show <id|name>` and `lx chains create --name demo --tier starter --fee-share-bps 100`. Creation reads current parameters, signs with the configured wallet and reports the receipt's registry ID, derived chain ID and RPC URL.
