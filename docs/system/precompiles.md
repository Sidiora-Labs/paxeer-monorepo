<!-- Preserve the lifecycle and address-selection notes when refreshing ABI signatures. -->

# Precompile ABIs

Paxeer is one BFT network, with EVM chain id **125 (`0x7d`)**. The kernel is part of the network and proposes blocks; a receipt is final when its block is committed. Kernel accounts and EVM accounts use the same chain state. Tenant sub-chains are database domains, rather than separate infrastructure.

Use `eth_call` for reads and `eth_sendRawTransaction` for signed writes through a node's root JSON-RPC endpoint, for example `https://api1.mainnet-beta.paxeer.network`. See [public JSON-RPC endpoints](../site/docs/reference/public-rpc.md) and [EVM JSON-RPC methods](rpc.md).

This catalogue preserves the ABI functions, events, errors and Solidity declarations useful for encoding calls and decoding history. **An ABI file, an address constant or an ABI-discovery result does not establish that a method is currently served.** Select the implementation at the queried block height and apply the lifecycle notes below. Function counts count declarations, including retained or disabled functions. Mutability labels reproduce the ABI declaration, including historical declarations.

Sources: [precompile registration and discovery](../../precompiles/setup.go), [node keeper wiring](../../node/precompiles.go), [activation plans](../../node/upgrades.go), [implementation switches](../../node/precompiles_v6120.go), each linked ABI, and the [Solidity interfaces](../../contracts/src/precompiles/). For module parameters, see [chain modules](chain-modules.md).

## Availability and upgrades

The node's activation gates use the applied upgrade height and module state. `v6.9` activates the fork precompile group; `v6.12.0` selects perpetuals at `0x1015`, generation 2 launchpad at `0x1017`, pricefeed at `0x1020`, and the five spot rails. The merge plan is `v7.0`: it activates the five construct precompiles at `0x101A`–`0x101E` and the eight agent precompiles at `0x0901`–`0x0908`. Constructors must be registered and their keepers supplied; the address table alone is insufficient. The current node imports those constructors and validates the merge set. These are code activation rules, not a claim that every public node has applied every plan.

Retained addresses may deliberately execute a refusal. The common retirement response is an EVM revert with `Error(string)` carrying `LXP_MODULE_DISABLED: module disabled; <name> retired`; see [retirement encoding and height checks](../../precompiles/common/retire.go).

| Surface | Lifecycle in the current implementation | Replacement or selection |
| --- | --- | --- |
| `layerxverify`, `0x1012` | Every method refuses from the merge height with `LXP_MODULE_DISABLED`, naming `layerxVerify`. Receipt inclusion, discovery proof and non-raw Ed25519 domain verification are already retired in the retained executor. | Verify the committed block/header and its native state proofs. |
| `layerxcustody`, `0x1013` | Every method, including views, deposits, withdrawal claims, forced exits and deposit-root registration, refuses from the merge height, naming `layerxCustody`. | Fund the derived sub-account with a bank send; use bank or ledger moves for native transfers and bridge `0x1016` for outward transfers. |
| `layerxanchor`, `0x1014` | Writes refuse from the merge height. Historical views remain only while the anchor store is retained. From `v7.0-anchor-store-delete`, all selectors refuse, naming `layerxAnchor`; an unwired anchor also refuses. | Read finality from the committed block. State proofs refer to the committed app hash at a height. |
| Legacy exchange ABI, `0x1015` | Replaced at `v6.12.0` by the perps ABI. Legacy keeper intent creation refuses with `ErrRetired` (module code `14`, reason `LXP_EXCHANGE_RETIRED`); the old ABI is not a second current interface at this address. | Use the perps `getOrder(uint256)` and `getPosition(uint256)` views and synchronous trading methods. |
| Oracle data queries, `0x1008` | Both methods unconditionally refuse with `oracle precompile is retired; oracle data queries are disabled`. | Select the pricefeed or agent oracle ABI appropriate to the data needed. |
| Ledger payer grants, `0x101A` | `issueGrant`, `revokeGrant`, `draw` and `getPayerGrant` retain their selectors but refuse the retired route with `LXP_NO_PAYER_GRANT` (module code `214`), subject to ordinary caller, input and pause checks. | Use the supported ledger payments and native budget operations; do not create new payer-grant flows. |
| Pay `budgetPull`, `0x101B` | Always refuses with `LXP_MODULE_DISABLED`, naming `layerxpay`. | Use the supported native funding and budget spending methods. |
| Generation 1 launchpad surface addresses | Retirement runtime reverts with `Retired(address replacement)` for any call. | Resolve the generation 2 role or pool surface from chain state. |

Historical ABI event declarations below describe decoding schemas; retired operations do not become active by retaining an event definition.

## Selecting an ABI and destination

At `0x1017`, only `precompiles/launchpad/abi.json` is the direct launchpad ABI. The role files under `precompiles/launchpad/abi/` describe distinct registered role and pool surfaces, whose deployed addresses come from chain state. Direct selector conflicts resolve in this order: Router, SidioraFactory, Quoter, PoolRegistry, FeesRouter, FeeAccumulator, ProtocolConfig. SidioraERC20 describes token contracts used in nested calls. Do not send the whole SidioraPool, SidioraNFT, Treasury or token ABI to the fixed launchpad address. See [launchpad dispatch](../../precompiles/launchpad/launchpad.go) and [role ABI selection](../../precompiles/launchpad/abi.go). The Router role is a trading contract role.

Spot ABIs map one-to-one to rails; they are not interchangeable. See [served spot set](../../precompiles/spot/spot.go) and [address constants](../../modules/spot/types/keys.go).

| ABI | Destination |
| --- | --- |
| `pecor.json` (`spot_pecor`) | `0x0000000000000000000000000000000000001021` |
| `orders.json` (`spot_orders`) | `0x0000000000000000000000000000000000001022` |
| `stoporders.json` (`spot_stoporders`) | `0x0000000000000000000000000000000000001023` |
| `quoter.json` (`spot_quoter`) | `0x0000000000000000000000000000000000001024` |
| `vault.json` (`spot_vault`) | `0x0000000000000000000000000000000000001025` |
| Share-token operations | Dynamic native ERC20 pointer returned by vault `shareTokens(address token)` |

`0x1026` and `0x1027` have no registered spot precompile. Share balances are bank balances of `spot/share/<lowercase token address>`; the spot keeper mints and burns them inside vault transactions. The retained `share.json` includes Ownable and public mint/burn declarations that the native pointer does not serve. Share pointers emit ERC20 `Transfer` and `Approval`, without `OwnershipTransferred`. See [share creation and accounting](../../modules/spot/keeper/shares.go) and [served event selection](../../precompiles/spot/events.go).

The distribution `staking_abi.json` is a test helper, not an additional ABI at `0x1007`. Send staking operations to the staking precompile at `0x1005`. The common ERC20 ABI is a contract template, with no fixed precompile address.

## Encoding and result semantics

The signatures below expand every JSON ABI tuple recursively as `(type field, ...)`, preserving array suffixes and component order. These are ABI descriptions, not Solidity source declarations. For selector hashing, remove field names and use canonical tuple types; use the linked JSON with an ABI encoder. Events mark indexed arguments explicitly so callers can distinguish topics from data. Solidity structs and data locations are defined in the linked interface sources.

Programs is fixed at `0x000000000000000000000000000000000000101D`; Chains is fixed at `0x000000000000000000000000000000000000101E`. Programs' own gate requires the merge plan's applied height, and refuses inactive calls with `LXP_MODULE_DISABLED: module disabled`.

Programs `quoteCallFee(uint64[7])` returns `(uint32 version, uint128 executionCeiling, bytes32 feeAsset)`. The ceiling is the sum of each budget word times its active schedule price, bounded to `uint128`; `feeAsset` is the schedule's asset identifier, matching `getCurrentFeeSchedule`, rather than a schedule digest. `getValueAccount(bytes32,bytes32)` returns the real `(uint128 balance, bool frozen)` flag from the host. An absent account returns `(0, false)`; the boolean does not indicate existence. See [fee quote handler](../../precompiles/layerxprograms/method_quote.go) and [account view handler](../../precompiles/layerxprograms/view_accounts.go).

Ledger `payExact` takes `expiresAtMs`, a Unix expiry in milliseconds; the keeper selects the next account sequence. It returns no ABI values and emits `PaymentExact` on success. The retired payer-grant route keeps the `issueGrant`, `revokeGrant`, `draw` and `getPayerGrant` declarations for compatibility. See [payment handlers](../../precompiles/layerxledger/payments.go).

The current `XWebProgramRequested` schema is `(bytes32 indexed programId, uint64 indexed requestId, uint8 kind, bytes payload, uint64 sequence)`. It describes program-origin requests; the old `origin/input/deadline` layout is historical. Decode current logs using [the xweb ABI](../../precompiles/xweb/abi.json) and its [emitter](../../precompiles/xweb/xweb.go).

## Summary

| Precompile | ABI | Destination | Functions | Availability / selection |
| --- | --- | --- | --- | --- |
| addr | [`precompiles/addr/abi.json`](../../precompiles/addr/abi.json) | `0x0000000000000000000000000000000000001004` | 10 | Registered; subject to module checks |
| bank | [`precompiles/bank/abi.json`](../../precompiles/bank/abi.json) | `0x0000000000000000000000000000000000001001` | 9 | Registered; subject to module checks |
| common | [`precompiles/common/erc20_abi.json`](../../precompiles/common/erc20_abi.json) | Contract-specific | 14 | ERC20 template; no fixed endpoint |
| distribution | [`precompiles/distribution/abi.json`](../../precompiles/distribution/abi.json) | `0x0000000000000000000000000000000000001007` | 5 | Registered; subject to module checks |
| distribution | [`precompiles/distribution/staking_abi.json`](../../precompiles/distribution/staking_abi.json) | Test helper; use staking `0x1005` | 3 | Not served at distribution `0x1007` |
| feetoken | [`precompiles/feetoken/abi.json`](../../precompiles/feetoken/abi.json) | `0x0000000000000000000000000000000000001018` | 3 | Registered; subject to module checks |
| gov | [`precompiles/gov/abi.json`](../../precompiles/gov/abi.json) | `0x0000000000000000000000000000000000001006` | 4 | Registered; subject to module checks |
| ibc | [`precompiles/ibc/abi.json`](../../precompiles/ibc/abi.json) | `0x0000000000000000000000000000000000001009` | 2 | Registered; subject to module checks |
| json | [`precompiles/json/abi.json`](../../precompiles/json/abi.json) | `0x0000000000000000000000000000000000001003` | 4 | Registered; subject to module checks |
| launchpad | [`precompiles/launchpad/abi/FeeAccumulator.json`](../../precompiles/launchpad/abi/FeeAccumulator.json) | Registered FeeAccumulator surface | 54 | Role ABI; resolve surface from state |
| launchpad | [`precompiles/launchpad/abi/FeesRouter.json`](../../precompiles/launchpad/abi/FeesRouter.json) | Registered FeesRouter surface | 20 | Role ABI; resolve surface from state |
| launchpad | [`precompiles/launchpad/abi/PoolRegistry.json`](../../precompiles/launchpad/abi/PoolRegistry.json) | Registered PoolRegistry surface | 22 | Role ABI; resolve surface from state |
| launchpad | [`precompiles/launchpad/abi/ProtocolConfig.json`](../../precompiles/launchpad/abi/ProtocolConfig.json) | Registered ProtocolConfig surface | 27 | Role ABI; resolve surface from state |
| launchpad | [`precompiles/launchpad/abi/Quoter.json`](../../precompiles/launchpad/abi/Quoter.json) | Registered Quoter surface | 19 | Role ABI; resolve surface from state |
| launchpad | [`precompiles/launchpad/abi/Router.json`](../../precompiles/launchpad/abi/Router.json) | Registered Router surface | 23 | Role ABI; resolve surface from state |
| launchpad | [`precompiles/launchpad/abi/SidioraERC20.json`](../../precompiles/launchpad/abi/SidioraERC20.json) | Deployment-specific token | 19 | Nested token ABI |
| launchpad | [`precompiles/launchpad/abi/SidioraFactory.json`](../../precompiles/launchpad/abi/SidioraFactory.json) | Registered SidioraFactory surface | 23 | Role ABI; resolve surface from state |
| launchpad | [`precompiles/launchpad/abi/SidioraNFT.json`](../../precompiles/launchpad/abi/SidioraNFT.json) | Registered SidioraNFT surface | 37 | Role ABI; resolve surface from state |
| launchpad | [`precompiles/launchpad/abi/SidioraPool.json`](../../precompiles/launchpad/abi/SidioraPool.json) | Registered SidioraPool surface | 42 | Role ABI; resolve surface from state |
| launchpad | [`precompiles/launchpad/abi/Treasury.json`](../../precompiles/launchpad/abi/Treasury.json) | Registered Treasury surface | 17 | Role ABI; resolve surface from state |
| launchpad | [`precompiles/launchpad/abi.json`](../../precompiles/launchpad/abi.json) | `0x0000000000000000000000000000000000001017` | 113 | Direct ABI; generation 2 from `v6.12.0` |
| layerxanchor | [`precompiles/layerxanchor/abi.json`](../../precompiles/layerxanchor/abi.json) | `0x0000000000000000000000000000000000001014` | 21 | Retained ABI; retirement rules above |
| layerxauth | [`precompiles/layerxauth/abi.json`](../../precompiles/layerxauth/abi.json) | `0x000000000000000000000000000000000000101C` | 10 | Merge-gated (`v7.0`) |
| layerxbridge | [`precompiles/layerxbridge/abi.json`](../../precompiles/layerxbridge/abi.json) | `0x0000000000000000000000000000000000001016` | 7 | Registered; subject to module checks |
| layerxchains | [`precompiles/layerxchains/abi.json`](../../precompiles/layerxchains/abi.json) | `0x000000000000000000000000000000000000101E` | 12 | Merge-gated (`v7.0`) |
| layerxcustody | [`precompiles/layerxcustody/abi.json`](../../precompiles/layerxcustody/abi.json) | `0x0000000000000000000000000000000000001013` | 20 | Retained ABI; retirement rules above |
| layerxexchange | [`precompiles/layerxexchange/abi.json`](../../precompiles/layerxexchange/abi.json) | `0x0000000000000000000000000000000000001015` | 12 | Historical; replaced by perps at `v6.12.0` |
| layerxledger | [`precompiles/layerxledger/abi.json`](../../precompiles/layerxledger/abi.json) | `0x000000000000000000000000000000000000101A` | 18 | Merge-gated (`v7.0`) |
| layerxpay | [`precompiles/layerxpay/abi.json`](../../precompiles/layerxpay/abi.json) | `0x000000000000000000000000000000000000101B` | 57 | Merge-gated (`v7.0`) |
| layerxprograms | [`precompiles/layerxprograms/abi.json`](../../precompiles/layerxprograms/abi.json) | `0x000000000000000000000000000000000000101D` | 35 | Merge-gated (`v7.0`) |
| layerxverify | [`precompiles/layerxverify/abi.json`](../../precompiles/layerxverify/abi.json) | `0x0000000000000000000000000000000000001012` | 5 | Retained ABI; retirement rules above |
| oracle | [`precompiles/oracle/abi.json`](../../precompiles/oracle/abi.json) | `0x0000000000000000000000000000000000001008` | 2 | Retained ABI; retirement rules above |
| p256 | [`precompiles/p256/abi.json`](../../precompiles/p256/abi.json) | `0x0000000000000000000000000000000000001011` | 1 | Registered; subject to module checks |
| perps | [`precompiles/perps/abi/abi.json`](../../precompiles/perps/abi/abi.json) | `0x0000000000000000000000000000000000001015` | 67 | Selected from `v6.12.0` |
| pointer | [`precompiles/pointer/abi.json`](../../precompiles/pointer/abi.json) | `0x000000000000000000000000000000000000100b` | 4 | Registered; subject to module checks |
| pointerview | [`precompiles/pointerview/abi.json`](../../precompiles/pointerview/abi.json) | `0x000000000000000000000000000000000000100A` | 4 | Registered; subject to module checks |
| pricefeed | [`precompiles/pricefeed/abi/abi.json`](../../precompiles/pricefeed/abi/abi.json) | `0x0000000000000000000000000000000000001020` | 12 | Selected from `v6.12.0` |
| solo | [`precompiles/solo/abi.json`](../../precompiles/solo/abi.json) | `0x000000000000000000000000000000000000100C` | 2 | Registered; subject to module checks |
| spot | [`precompiles/spot/abi/orders.json`](../../precompiles/spot/abi/orders.json) | `0x0000000000000000000000000000000000001022` | 40 | Single rail, gated by `v6.12.0` |
| spot | [`precompiles/spot/abi/pecor.json`](../../precompiles/spot/abi/pecor.json) | `0x0000000000000000000000000000000000001021` | 45 | Single rail, gated by `v6.12.0` |
| spot | [`precompiles/spot/abi/quoter.json`](../../precompiles/spot/abi/quoter.json) | `0x0000000000000000000000000000000000001024` | 17 | Single rail, gated by `v6.12.0` |
| spot | [`precompiles/spot/abi/share.json`](../../precompiles/spot/abi/share.json) | Dynamic native ERC20 pointer | 14 | Retained contract ABI; subset served by pointer |
| spot | [`precompiles/spot/abi/stoporders.json`](../../precompiles/spot/abi/stoporders.json) | `0x0000000000000000000000000000000000001023` | 49 | Single rail, gated by `v6.12.0` |
| spot | [`precompiles/spot/abi/vault.json`](../../precompiles/spot/abi/vault.json) | `0x0000000000000000000000000000000000001025` | 42 | Single rail, gated by `v6.12.0` |
| staking | [`precompiles/staking/abi.json`](../../precompiles/staking/abi.json) | `0x0000000000000000000000000000000000001005` | 19 | Registered; subject to module checks |
| wasmd | [`precompiles/wasmd/abi.json`](../../precompiles/wasmd/abi.json) | `0x0000000000000000000000000000000000001002` | 4 | Registered; subject to module checks |
| xweb | [`precompiles/xweb/abi.json`](../../precompiles/xweb/abi.json) | `0x0000000000000000000000000000000000001019` | 12 | Registered; subject to module checks |
| layerxagent/clearing | [`precompiles/layerxagent/clearing/clearing.go:abiJSON`](../../precompiles/layerxagent/clearing/clearing.go) | `0x0000000000000000000000000000000000000902` | 1 | Merge-gated (`v7.0`) |
| layerxagent/eip712 | [`precompiles/layerxagent/eip712/eip712.go:abiJSON`](../../precompiles/layerxagent/eip712/eip712.go) | `0x0000000000000000000000000000000000000908` | 3 | Merge-gated (`v7.0`) |
| layerxagent/oracle | [`precompiles/layerxagent/oracle/oracle.go:abiJSON`](../../precompiles/layerxagent/oracle/oracle.go) | `0x0000000000000000000000000000000000000903` | 3 | Merge-gated (`v7.0`) |
| layerxagent/orob | [`precompiles/layerxagent/orob/orob.go:abiJSON`](../../precompiles/layerxagent/orob/orob.go) | `0x0000000000000000000000000000000000000901` | 3 | Merge-gated (`v7.0`) |
| layerxagent/pofq | [`precompiles/layerxagent/pofq/pofq.go:abiJSON`](../../precompiles/layerxagent/pofq/pofq.go) | `0x0000000000000000000000000000000000000904` | 3 | Merge-gated (`v7.0`) |
| layerxagent/scheduler | [`precompiles/layerxagent/scheduler/scheduler.go:abiJSON`](../../precompiles/layerxagent/scheduler/scheduler.go) | `0x0000000000000000000000000000000000000905` | 5 | Merge-gated (`v7.0`) |
| layerxagent/streams | [`precompiles/layerxagent/streams/streams.go:abiJSON`](../../precompiles/layerxagent/streams/streams.go) | `0x0000000000000000000000000000000000000906` | 6 | Merge-gated (`v7.0`) |
| layerxagent/teeattestor | [`precompiles/layerxagent/teeattestor/teeattestor.go:abiJSON`](../../precompiles/layerxagent/teeattestor/teeattestor.go) | `0x0000000000000000000000000000000000000907` | 4 | Merge-gated (`v7.0`) |

## addr (`abi.json`)

Source: [`precompiles/addr/abi.json`](../../precompiles/addr/abi.json).

Destination: `0x0000000000000000000000000000000000001004`.

### Functions

- `associate(string v, string r, string s, string customMessage) returns (string paxAddr, address evmAddr)` `nonpayable`
- `associatePubKey(string pubKeyHex) returns (string paxAddr, address evmAddr)` `nonpayable`
- `bindLayerX(bytes32 didPublicKey, bytes signature)` `nonpayable`
- `getEvmAddr(string addr) returns (address response)` `view`
- `getEvmAddrByLayerX(bytes32 didPublicKey) returns (address evmAddr)` `view`
- `getLayerXDid(address addr) returns (bytes32 didPublicKey, string did)` `view`
- `getPaxAddr(address addr) returns (string response)` `view`
- `getUnifiedAccount(address addr) returns (address evm, string paxAddr, bytes32 didPublicKey, bytes32 layerxMainAccountId)` `view`
- `layerXBindNonce(address addr) returns (uint64 nonce)` `view`
- `unbindLayerX()` `nonpayable`

### Events

- `LayerXBound(address indexed evm, bytes32 indexed didPublicKey, uint64 nonce)`
- `LayerXUnbound(address indexed evm, bytes32 indexed didPublicKey, uint64 nonce)`

## bank (`abi.json`)

Source: [`precompiles/bank/abi.json`](../../precompiles/bank/abi.json).

Destination: `0x0000000000000000000000000000000000001001`.

### Functions

- `all_balances(address acc) returns ((uint256 amount, string denom)[] response)` `view`
- `balance(address acc, string denom) returns (uint256 amount)` `view`
- `decimals(string denom) returns (uint8 response)` `view`
- `name(string denom) returns (string response)` `view`
- `send(address fromAddress, address toAddress, string denom, uint256 amount) returns (bool success)` `nonpayable`
- `send(address fromAddress, address toAddress, string denom, uint256 amount, bytes32 contextHash) returns (bool success)` `nonpayable`
- `sendNative(string toNativeAddress) returns (bool success)` `payable`
- `supply(string denom) returns (uint256 response)` `view`
- `symbol(string denom) returns (string response)` `view`

### Events

- `LayerXReceipt(address indexed from, address indexed to, string denom, uint256 amount, bytes32 contextHash)`

## common (`erc20_abi.json`)

Source: [`precompiles/common/erc20_abi.json`](../../precompiles/common/erc20_abi.json).

Destination: Contract-specific.

ABI template for ERC20 contracts and pointers. Resolve a concrete contract address before encoding a call.

### Functions

- `BankPrecompile() returns (address)` `view`
- `allowance(address owner, address spender) returns (uint256)` `view`
- `approve(address spender, uint256 value) returns (bool)` `nonpayable`
- `balanceOf(address account) returns (uint256)` `view`
- `ddecimals() returns (uint8)` `view`
- `decimals() returns (uint8)` `view`
- `denom() returns (string)` `view`
- `name() returns (string)` `view`
- `nname() returns (string)` `view`
- `ssymbol() returns (string)` `view`
- `symbol() returns (string)` `view`
- `totalSupply() returns (uint256)` `view`
- `transfer(address to, uint256 value) returns (bool)` `nonpayable`
- `transferFrom(address from, address to, uint256 value) returns (bool)` `nonpayable`

### Events

- `Approval(address indexed owner, address indexed spender, uint256 value)`
- `Transfer(address indexed from, address indexed to, uint256 value)`

### Errors

- `ERC20InsufficientAllowance(address spender, uint256 allowance, uint256 needed)`
- `ERC20InsufficientBalance(address sender, uint256 balance, uint256 needed)`
- `ERC20InvalidApprover(address approver)`
- `ERC20InvalidReceiver(address receiver)`
- `ERC20InvalidSender(address sender)`
- `ERC20InvalidSpender(address spender)`

## distribution (`abi.json`)

Source: [`precompiles/distribution/abi.json`](../../precompiles/distribution/abi.json).

Destination: `0x0000000000000000000000000000000000001007`.

### Functions

- `rewards(address delegatorAddress) returns ((((uint256 amount, uint256 decimals, string denom)[] coins, string validator_address)[] rewards, (uint256 amount, uint256 decimals, string denom)[] total) rewards)` `view`
- `setWithdrawAddress(address withdrawAddr) returns (bool success)` `nonpayable`
- `withdrawDelegationRewards(string validator) returns (bool success)` `nonpayable`
- `withdrawMultipleDelegationRewards(string[] validators) returns (bool success)` `nonpayable`
- `withdrawValidatorCommission() returns (bool success)` `nonpayable`

### Events

- `DelegationRewardsWithdrawn(address indexed delegator, string validator, uint256 amount)`
- `MultipleDelegationRewardsWithdrawn(address indexed delegator, string[] validators, uint256[] amounts)`
- `ValidatorCommissionWithdrawn(string indexed validator, uint256 amount)`
- `WithdrawAddressSet(address indexed delegator, address withdrawAddr)`

## distribution (`staking_abi.json`)

Source: [`precompiles/distribution/staking_abi.json`](../../precompiles/distribution/staking_abi.json).

Destination: Test helper; use staking `0x1005`.

Test-only helper ABI embedded by `distribution_test.go`. These staking signatures are not methods of the distribution precompile at `0x1007`; use the staking ABI at `0x1005`.

### Functions

- `delegate(string valAddress) returns (bool success)` `payable`
- `redelegate(string srcAddress, string dstAddress, uint256 amount) returns (bool success)` `nonpayable`
- `undelegate(string valAddress, uint256 amount) returns (bool success)` `nonpayable`

## feetoken (`abi.json`)

Source: [`precompiles/feetoken/abi.json`](../../precompiles/feetoken/abi.json).

Destination: `0x0000000000000000000000000000000000001018`.

### Functions

- `clearFeeDenom()` `nonpayable`
- `getFeeDenom(address account) returns (string denom)` `view`
- `setFeeDenom(string denom)` `nonpayable`

## gov (`abi.json`)

Source: [`precompiles/gov/abi.json`](../../precompiles/gov/abi.json).

Destination: `0x0000000000000000000000000000000000001006`.

### Functions

- `deposit(uint64 proposalID) returns (bool success)` `payable`
- `submitProposal(string proposalJSON) returns (uint64 proposalID)` `payable`
- `vote(uint64 proposalID, int32 option) returns (bool success)` `nonpayable`
- `voteWeighted(uint64 proposalID, (int32 option, string weight)[] options) returns (bool success)` `nonpayable`

## ibc (`abi.json`)

Source: [`precompiles/ibc/abi.json`](../../precompiles/ibc/abi.json).

Destination: `0x0000000000000000000000000000000000001009`.

### Functions

- `transfer(string toAddress, string port, string channel, string denom, uint256 amount, uint64 revisionNumber, uint64 revisionHeight, uint64 timeoutTimestamp, string memo) returns (bool success)` `nonpayable`
- `transferWithDefaultTimeout(string toAddress, string port, string channel, string denom, uint256 amount, string memo) returns (bool success)` `nonpayable`

## json (`abi.json`)

Source: [`precompiles/json/abi.json`](../../precompiles/json/abi.json).

Destination: `0x0000000000000000000000000000000000001003`.

### Functions

- `extractAsBytes(bytes input, string key) returns (bytes response)` `view`
- `extractAsBytesFromArray(bytes input, uint16 arrayIndex) returns (bytes response)` `view`
- `extractAsBytesList(bytes input, string key) returns (bytes[] response)` `view`
- `extractAsUint256(bytes input, string key) returns (uint256 response)` `view`

## launchpad (`FeeAccumulator.json`)

Source: [`precompiles/launchpad/abi/FeeAccumulator.json`](../../precompiles/launchpad/abi/FeeAccumulator.json).

Destination: Registered FeeAccumulator surface.

ABI of the registered FeeAccumulator surface. Its target address comes from chain state; it is not generally interchangeable with the direct `0x1017` ABI. Generation 1 surfaces retire with `Retired(address replacement)`.

### Functions

- `DEAD_ADDRESS() returns (address)` `view`
- `DEFAULT_ADMIN_ROLE() returns (bytes32)` `view`
- `FACTORY_ROLE() returns (bytes32)` `view`
- `FEES_ROUTER_ROLE() returns (bytes32)` `view`
- `OPTICAL_CLAIM_ROLE() returns (bytes32)` `view`
- `OPTICAL_GRANTER_ROLE() returns (bytes32)` `view`
- `POOL_ROLE() returns (bytes32)` `view`
- `accountingInitialized() returns (bool)` `view`
- `accountingMigrationCursor() returns (uint256)` `view`
- `airdropClaimedSupply(address, uint256) returns (uint256)` `view`
- `airdropEligibleSupply(address, uint256) returns (uint256)` `view`
- `airdropEpochRemaining(address, uint256) returns (uint256)` `view`
- `airdropSnapshotBlock(address, uint256) returns (uint256)` `view`
- `allocateLegacyAirdrop(address pool, uint256 epoch, bytes32 root, uint256 budget)` `nonpayable`
- `authorizeOptical(address optical)` `nonpayable`
- `authorizePool(address pool)` `nonpayable`
- `burn(address pool) returns (uint256 amount)` `nonpayable`
- `claim(address pool, address recipient) returns (uint256 amount)` `nonpayable`
- `claimAirdrop(address pool, address holder) returns (uint256 amount)` `nonpayable`
- `claimAirdropEpoch(address pool, address holder, uint256 epoch) returns (uint256 amount)` `nonpayable`
- `claimLegacyAirdrop(address pool, uint256 epoch, address holder, uint256 amount, bytes32[] proof)` `nonpayable`
- `claimOpticalSurplus(address pool, uint256 amount, address recipient)` `nonpayable`
- `eventEmitter() returns (address)` `view`
- `getAccumulatedFees(address pool) returns (uint256)` `view`
- `getAirdropBalance(address pool) returns (uint256)` `view`
- `getAirdropEpoch(address pool) returns (uint256)` `view`
- `getLpRewardsBalance(address pool) returns (uint256)` `view`
- `getOpticalSurplus(address pool) returns (uint256)` `view`
- `getProtocolFeesPending() returns (uint256)` `view`
- `getRoleAdmin(bytes32 role) returns (bytes32)` `view`
- `grantRole(bytes32 role, address account)` `nonpayable`
- `hasClaimedAirdrop(address pool, address holder, uint256 epoch) returns (bool)` `view`
- `hasRole(bytes32 role, address account) returns (bool)` `view`
- `initialize(address _protocolConfig, address _treasury, address _poolRegistry, address _eventEmitter, address _usdlAddress, address _admin)` `nonpayable`
- `legacyAirdropRemaining(address, uint256) returns (uint256)` `view`
- `legacyAirdropRoot(address, uint256) returns (bytes32)` `view`
- `legacyAirdropUnallocated(address) returns (uint256)` `view`
- `migrateAccounting(uint256 maxPools)` `nonpayable`
- `migrateAdditionalAccounting(address[] pools)` `nonpayable`
- `poolRegistry() returns (address)` `view`
- `protocolConfig() returns (address)` `view`
- `proxiableUUID() returns (bytes32)` `view`
- `recordFee(address pool, uint256 feeAmount)` `nonpayable`
- `recordOpticalSurplus(address pool, uint256 amount)` `nonpayable`
- `registeredDefaultAdminCount() returns (uint256)` `view`
- `renounceRole(bytes32 role, address callerConfirmation)` `nonpayable`
- `revokeRole(bytes32 role, address account)` `nonpayable`
- `sendLpRewards(address pool) returns (uint256 amount)` `nonpayable`
- `supportsAirdrop(address pool) returns (bool)` `view`
- `totalLiabilities() returns (uint256)` `view`
- `treasury() returns (address)` `view`
- `triggerAirdrop(address pool) returns (uint256 totalAmount)` `nonpayable`
- `upgradeToAndCall(address newImplementation, bytes data)` `payable`
- `usdlAddress() returns (address)` `view`

### Events

- `AccountingMigrated(uint256 liabilities)`
- `AirdropClaimed(address indexed pool, address indexed holder, uint256 amount, uint256 epoch)`
- `AirdropTriggered(address indexed pool, uint256 totalAmount, uint256 epoch)`
- `FeeRecorded(address indexed pool, uint256 feeAmount, uint256 protocolCut, uint256 poolCut)`
- `FeesBurned(address indexed pool, uint256 amount)`
- `FeesClaimed(address indexed pool, address indexed recipient, uint256 amount)`
- `Initialized(uint64 version)`
- `LegacyAirdropAllocated(address indexed pool, uint256 indexed epoch, bytes32 root, uint256 budget)`
- `LpRewardsSent(address indexed pool, uint256 amount)`
- `MirrorFailed(bytes4 indexed selector)`
- `OpticalSurplusClaimed(address indexed pool, address indexed recipient, uint256 amount)`
- `OpticalSurplusRecorded(address indexed pool, uint256 amount)`
- `ProtocolFeeSwept(uint256 amount)`
- `RoleAdminChanged(bytes32 indexed role, bytes32 indexed previousAdminRole, bytes32 indexed newAdminRole)`
- `RoleGranted(bytes32 indexed role, address indexed account, address indexed sender)`
- `RoleRevoked(bytes32 indexed role, address indexed account, address indexed sender)`
- `Upgraded(address indexed implementation)`

### Errors

- `AirdropNotTriggered()`
- `AlreadyClaimed()`
- `AlreadyInitialized()`
- `ApproveFailed()`
- `InsufficientSurplus()`
- `InvalidImplementation()`
- `InvalidRoleAccount()`
- `LastDefaultAdmin()`
- `MissingRole(address account, bytes32 role)`
- `NoFeesAccumulated()`
- `NonPayableUpgrade()`
- `NotInitializing()`
- `ReentrancyGuardReentrantCall()`
- `TransferFailed()`
- `UUPSNotThroughActiveProxy()`
- `UUPSNotThroughProxy()`
- `Unauthorized()`
- `UnauthorizedUpgrade()`
- `UnsupportedSnapshotToken()`
- `UpgradeInitializationFailed()`
- `WrongStrategy()`
- `ZeroAddress()`
- `ZeroAmount()`

## launchpad (`FeesRouter.json`)

Source: [`precompiles/launchpad/abi/FeesRouter.json`](../../precompiles/launchpad/abi/FeesRouter.json).

Destination: Registered FeesRouter surface.

ABI of the registered FeesRouter surface. Its target address comes from chain state; it is not generally interchangeable with the direct `0x1017` ABI. Generation 1 surfaces retire with `Retired(address replacement)`.

### Functions

- `DEFAULT_ADMIN_ROLE() returns (bytes32)` `view`
- `claimAirdrop(uint256 nftId) returns (uint256 amount)` `nonpayable`
- `claimAirdropEpoch(uint256 nftId, uint256 epoch) returns (uint256 amount)` `nonpayable`
- `claimFees(uint256 nftId) returns (uint256 amount)` `nonpayable`
- `executeAirdrop(uint256 nftId) returns (uint256 amount)` `nonpayable`
- `executeBurn(uint256 nftId) returns (uint256 amount)` `nonpayable`
- `executeLpRewards(uint256 nftId) returns (uint256 amount)` `nonpayable`
- `feeAccumulator() returns (address)` `view`
- `getRoleAdmin(bytes32 role) returns (bytes32)` `view`
- `grantRole(bytes32 role, address account)` `nonpayable`
- `hasRole(bytes32 role, address account) returns (bool)` `view`
- `initialize(address _nftContract, address _feeAccumulator, address _poolRegistry, address _admin)` `nonpayable`
- `nftContract() returns (address)` `view`
- `poolRegistry() returns (address)` `view`
- `proxiableUUID() returns (bytes32)` `view`
- `registeredDefaultAdminCount() returns (uint256)` `view`
- `renounceRole(bytes32 role, address callerConfirmation)` `nonpayable`
- `revokeRole(bytes32 role, address account)` `nonpayable`
- `setFeeStrategy(uint256 nftId, uint8 newStrategy)` `nonpayable`
- `upgradeToAndCall(address newImplementation, bytes data)` `payable`

### Events

- `AirdropClaimed(uint256 indexed nftId, address indexed holder, uint256 amount)`
- `AirdropExecuted(uint256 indexed nftId, uint256 amount)`
- `FeeStrategyChanged(uint256 indexed nftId, uint8 oldStrategy, uint8 newStrategy)`
- `FeesBurned(uint256 indexed nftId, uint256 amount)`
- `FeesClaimed(uint256 indexed nftId, address indexed owner, uint256 amount)`
- `Initialized(uint64 version)`
- `LpRewardsExecuted(uint256 indexed nftId, uint256 amount)`
- `MirrorFailed(bytes4 indexed selector)`
- `RoleAdminChanged(bytes32 indexed role, bytes32 indexed previousAdminRole, bytes32 indexed newAdminRole)`
- `RoleGranted(bytes32 indexed role, address indexed account, address indexed sender)`
- `RoleRevoked(bytes32 indexed role, address indexed account, address indexed sender)`
- `Upgraded(address indexed implementation)`

### Errors

- `AlreadyInitialized()`
- `InvalidImplementation()`
- `InvalidRoleAccount()`
- `LastDefaultAdmin()`
- `MissingRole(address account, bytes32 role)`
- `NonPayableUpgrade()`
- `NotInitializing()`
- `NotNftOwner()`
- `ReentrancyGuardReentrantCall()`
- `UUPSNotThroughActiveProxy()`
- `UUPSNotThroughProxy()`
- `UnauthorizedUpgrade()`
- `UnsupportedSnapshotToken()`
- `UpgradeInitializationFailed()`
- `WrongStrategy()`
- `ZeroAddress()`

## launchpad (`PoolRegistry.json`)

Source: [`precompiles/launchpad/abi/PoolRegistry.json`](../../precompiles/launchpad/abi/PoolRegistry.json).

Destination: Registered PoolRegistry surface.

ABI of the registered PoolRegistry surface. Its target address comes from chain state; it is not generally interchangeable with the direct `0x1017` ABI. Generation 1 surfaces retire with `Retired(address replacement)`.

### Functions

- `DEFAULT_ADMIN_ROLE() returns (bytes32)` `view`
- `FACTORY_ROLE() returns (bytes32)` `view`
- `eventEmitter() returns (address)` `view`
- `getAllPools(uint256 offset, uint256 limit) returns (address[])` `view`
- `getNftIdByPool(address pool) returns (uint256)` `view`
- `getPoolByToken(address token) returns (address)` `view`
- `getPoolCount() returns (uint256)` `view`
- `getPoolCountByCreator(address creator) returns (uint256)` `view`
- `getPoolMetadata(address pool) returns ((address creator, address token, address optical, uint256 nftId, uint256 createdAt, uint256 createdBlock))` `view`
- `getPoolsByCreator(address creator) returns (address[])` `view`
- `getPoolsByCreatorPage(address creator, uint256 offset, uint256 limit) returns (address[] result)` `view`
- `getRoleAdmin(bytes32 role) returns (bytes32)` `view`
- `grantRole(bytes32 role, address account)` `nonpayable`
- `hasRole(bytes32 role, address account) returns (bool)` `view`
- `initialize(address _eventEmitter, address _admin)` `nonpayable`
- `isRegisteredPool(address pool) returns (bool)` `view`
- `proxiableUUID() returns (bytes32)` `view`
- `register(address pool, address token, address creator, address optical, uint256 nftId)` `nonpayable`
- `registeredDefaultAdminCount() returns (uint256)` `view`
- `renounceRole(bytes32 role, address callerConfirmation)` `nonpayable`
- `revokeRole(bytes32 role, address account)` `nonpayable`
- `upgradeToAndCall(address newImplementation, bytes data)` `payable`

### Events

- `Initialized(uint64 version)`
- `MirrorFailed(bytes4 indexed selector)`
- `PoolRegistered(address indexed pool, address indexed token, address indexed creator, address optical, uint256 nftId, uint256 timestamp)`
- `RoleAdminChanged(bytes32 indexed role, bytes32 indexed previousAdminRole, bytes32 indexed newAdminRole)`
- `RoleGranted(bytes32 indexed role, address indexed account, address indexed sender)`
- `RoleRevoked(bytes32 indexed role, address indexed account, address indexed sender)`
- `Upgraded(address indexed implementation)`

### Errors

- `AlreadyInitialized()`
- `DuplicatePool()`
- `DuplicateToken()`
- `InvalidImplementation()`
- `InvalidRoleAccount()`
- `LastDefaultAdmin()`
- `MissingRole(address account, bytes32 role)`
- `NonPayableUpgrade()`
- `NotInitializing()`
- `PoolNotFound()`
- `UUPSNotThroughActiveProxy()`
- `UUPSNotThroughProxy()`
- `Unauthorized()`
- `UnauthorizedUpgrade()`
- `UpgradeInitializationFailed()`
- `ZeroAddress()`

## launchpad (`ProtocolConfig.json`)

Source: [`precompiles/launchpad/abi/ProtocolConfig.json`](../../precompiles/launchpad/abi/ProtocolConfig.json).

Destination: Registered ProtocolConfig surface.

ABI of the registered ProtocolConfig surface. Its target address comes from chain state; it is not generally interchangeable with the direct `0x1017` ABI. Generation 1 surfaces retire with `Retired(address replacement)`.

### Functions

- `DEFAULT_ADMIN_ROLE() returns (bytes32)` `view`
- `baseFeeBps() returns (uint256)` `view`
- `concentrationWeight() returns (uint256)` `view`
- `creationFee() returns (uint256)` `view`
- `eventEmitter() returns (address)` `view`
- `feeDecayRate() returns (uint256)` `view`
- `getRoleAdmin(bytes32 role) returns (bytes32)` `view`
- `grantRole(bytes32 role, address account)` `nonpayable`
- `hasRole(bytes32 role, address account) returns (bool)` `view`
- `initialize(address _usdlAddress, address _eventEmitter, address _admin)` `nonpayable`
- `maxFeeBps() returns (uint256)` `view`
- `minFeeBps() returns (uint256)` `view`
- `protocolFeeBps() returns (uint256)` `view`
- `proxiableUUID() returns (bytes32)` `view`
- `registeredDefaultAdminCount() returns (uint256)` `view`
- `renounceRole(bytes32 role, address callerConfirmation)` `nonpayable`
- `revokeRole(bytes32 role, address account)` `nonpayable`
- `setBaseFeeBps(uint256 newBaseFeeBps)` `nonpayable`
- `setCreationFee(uint256 newCreationFee)` `nonpayable`
- `setFeeWeights(uint256 newDecayRate, uint256 newVolWeight, uint256 newConcWeight)` `nonpayable`
- `setProtocolFeeBps(uint256 newProtocolFeeBps)` `nonpayable`
- `setVirtualDefaults(uint256 newVirtualUsdl, uint256 newVirtualToken)` `nonpayable`
- `upgradeToAndCall(address newImplementation, bytes data)` `payable`
- `usdlAddress() returns (address)` `view`
- `virtualTokenDefault() returns (uint256)` `view`
- `virtualUsdlDefault() returns (uint256)` `view`
- `volatilityWeight() returns (uint256)` `view`

### Events

- `ConfigUpdated(bytes32 indexed key, uint256 oldValue, uint256 newValue)`
- `Initialized(uint64 version)`
- `RoleAdminChanged(bytes32 indexed role, bytes32 indexed previousAdminRole, bytes32 indexed newAdminRole)`
- `RoleGranted(bytes32 indexed role, address indexed account, address indexed sender)`
- `RoleRevoked(bytes32 indexed role, address indexed account, address indexed sender)`
- `Upgraded(address indexed implementation)`

### Errors

- `AlreadyInitialized()`
- `FeeOutOfRange()`
- `InvalidImplementation()`
- `InvalidRoleAccount()`
- `LastDefaultAdmin()`
- `MissingRole(address account, bytes32 role)`
- `NonPayableUpgrade()`
- `NotInitializing()`
- `UUPSNotThroughActiveProxy()`
- `UUPSNotThroughProxy()`
- `UnauthorizedUpgrade()`
- `UpgradeInitializationFailed()`
- `ZeroAddress()`

## launchpad (`Quoter.json`)

Source: [`precompiles/launchpad/abi/Quoter.json`](../../precompiles/launchpad/abi/Quoter.json).

Destination: Registered Quoter surface.

ABI of the registered Quoter surface. Its target address comes from chain state; it is not generally interchangeable with the direct `0x1017` ABI. Generation 1 surfaces retire with `Retired(address replacement)`.

### Functions

- `DEFAULT_ADMIN_ROLE() returns (bytes32)` `view`
- `getAllPools(uint256 offset, uint256 limit) returns (address[])` `view`
- `getMarketCap(address pool) returns (uint256)` `view`
- `getPoolPrice(address pool) returns (uint256)` `view`
- `getPoolStats(address pool) returns ((uint256 virtualUsdl, uint256 realUsdl, uint256 tokenReserve, uint256 cumulativeVolume, uint256 currentFeeBps, uint256 poolAge, uint256 marketCap, uint256 price) stats)` `view`
- `getPoolsByCreator(address creator) returns (address[])` `view`
- `getRoleAdmin(bytes32 role) returns (bytes32)` `view`
- `grantRole(bytes32 role, address account)` `nonpayable`
- `hasRole(bytes32 role, address account) returns (bool)` `view`
- `initialize(address _poolRegistry, address _protocolConfig, address _admin)` `nonpayable`
- `poolRegistry() returns (address)` `view`
- `protocolConfig() returns (address)` `view`
- `proxiableUUID() returns (bytes32)` `view`
- `quoteExactInput(address pool, uint256 amountIn, bool isBuy) returns ((uint256 amountOut, uint256 feeAmount, uint256 priceImpactBps) result)` `view`
- `quoteMultihop(address tokenIn, address tokenOut, uint256 amountIn) returns ((uint256 amountOut, uint256 intermediateUsdl, uint256 sellFeeAmount, uint256 buyFeeAmount, uint256 sellPriceImpactBps, uint256 buyPriceImpactBps, uint256 combinedPriceImpactBps, address poolA, address poolB) result)` `view`
- `registeredDefaultAdminCount() returns (uint256)` `view`
- `renounceRole(bytes32 role, address callerConfirmation)` `nonpayable`
- `revokeRole(bytes32 role, address account)` `nonpayable`
- `upgradeToAndCall(address newImplementation, bytes data)` `payable`

### Events

- `Initialized(uint64 version)`
- `RoleAdminChanged(bytes32 indexed role, bytes32 indexed previousAdminRole, bytes32 indexed newAdminRole)`
- `RoleGranted(bytes32 indexed role, address indexed account, address indexed sender)`
- `RoleRevoked(bytes32 indexed role, address indexed account, address indexed sender)`
- `Upgraded(address indexed implementation)`

### Errors

- `AlreadyInitialized()`
- `DivisionByZero()`
- `InsufficientInput()`
- `InsufficientLiquidity()`
- `InvalidImplementation()`
- `InvalidRoleAccount()`
- `LastDefaultAdmin()`
- `MissingRole(address account, bytes32 role)`
- `NonPayableUpgrade()`
- `NotInitializing()`
- `OpticalQuoteUnsupported()`
- `Overflow()`
- `UUPSNotThroughActiveProxy()`
- `UUPSNotThroughProxy()`
- `UnauthorizedUpgrade()`
- `UnexecutableQuote()`
- `UpgradeInitializationFailed()`
- `ZeroAddress()`

## launchpad (`Router.json`)

Source: [`precompiles/launchpad/abi/Router.json`](../../precompiles/launchpad/abi/Router.json).

Destination: Registered Router surface.

ABI of the registered Router surface. Its target address comes from chain state; it is not generally interchangeable with the direct `0x1017` ABI. Generation 1 surfaces retire with `Retired(address replacement)`.

### Functions

- `DEFAULT_ADMIN_ROLE() returns (bytes32)` `view`
- `buy(address pool, uint256 usdlAmountIn, uint256 minTokensOut, uint256 deadline) returns (uint256 amountOut)` `nonpayable`
- `buyWithPermit(address pool, uint256 usdlAmountIn, uint256 minTokensOut, uint256 deadline, (uint256 value, uint256 deadline, uint8 v, bytes32 r, bytes32 s) permit) returns (uint256 amountOut)` `nonpayable`
- `createMarket(string name, string symbol, uint8 feeStrategy, address optical) returns (address tokenAddr, address poolAddr, uint256 nftId)` `nonpayable`
- `createMarketWithPermit(string name, string symbol, uint8 feeStrategy, address optical, (uint256 value, uint256 deadline, uint8 v, bytes32 r, bytes32 s) permit) returns (address tokenAddr, address poolAddr, uint256 nftId)` `nonpayable`
- `factory() returns (address)` `view`
- `getRoleAdmin(bytes32 role) returns (bytes32)` `view`
- `grantRole(bytes32 role, address account)` `nonpayable`
- `hasRole(bytes32 role, address account) returns (bool)` `view`
- `initialize(address _factory, address _poolRegistry, address _protocolConfig, address _usdlAddress, address _admin)` `nonpayable`
- `multicall(bytes[] data) returns (bytes[] results)` `nonpayable`
- `poolRegistry() returns (address)` `view`
- `protocolConfig() returns (address)` `view`
- `proxiableUUID() returns (bytes32)` `view`
- `registeredDefaultAdminCount() returns (uint256)` `view`
- `renounceRole(bytes32 role, address callerConfirmation)` `nonpayable`
- `revokeRole(bytes32 role, address account)` `nonpayable`
- `sell(address pool, uint256 tokenAmountIn, uint256 minUsdlOut, uint256 deadline) returns (uint256 amountOut)` `nonpayable`
- `sellWithPermit(address pool, uint256 tokenAmountIn, uint256 minUsdlOut, uint256 deadline, (uint256 value, uint256 deadline, uint8 v, bytes32 r, bytes32 s) permit) returns (uint256 amountOut)` `nonpayable`
- `swapTokenForToken(address tokenIn, address tokenOut, uint256 amountIn, uint256 minAmountOut, uint256 deadline) returns (uint256 amountOut, uint256 intermediateUsdl)` `nonpayable`
- `swapTokenForTokenWithPermit(address tokenIn, address tokenOut, uint256 amountIn, uint256 minAmountOut, uint256 deadline, (uint256 value, uint256 deadline, uint8 v, bytes32 r, bytes32 s) permit) returns (uint256 amountOut, uint256 intermediateUsdl)` `nonpayable`
- `upgradeToAndCall(address newImplementation, bytes data)` `payable`
- `usdlAddress() returns (address)` `view`

### Events

- `Buy(address indexed pool, address indexed buyer, uint256 usdlIn, uint256 tokensOut)`
- `Initialized(uint64 version)`
- `MarketCreated(address indexed token, address indexed pool, address indexed creator, uint256 nftId)`
- `MultihopSwap(address indexed sender, address indexed tokenIn, address indexed tokenOut, uint256 amountIn, uint256 intermediateUsdl, uint256 amountOut)`
- `RoleAdminChanged(bytes32 indexed role, bytes32 indexed previousAdminRole, bytes32 indexed newAdminRole)`
- `RoleGranted(bytes32 indexed role, address indexed account, address indexed sender)`
- `RoleRevoked(bytes32 indexed role, address indexed account, address indexed sender)`
- `Sell(address indexed pool, address indexed seller, uint256 tokensIn, uint256 usdlOut)`
- `Upgraded(address indexed implementation)`

### Errors

- `AlreadyInitialized()`
- `DeadlineExpired()`
- `InsufficientBalance()`
- `InvalidImplementation()`
- `InvalidRoleAccount()`
- `LastDefaultAdmin()`
- `MissingRole(address account, bytes32 role)`
- `MulticallFailed(uint256 index, bytes reason)`
- `NonPayableUpgrade()`
- `NotInitializing()`
- `PoolNotFound()`
- `ReentrancyGuardReentrantCall()`
- `SameToken()`
- `TransferFailed()`
- `TransferFromFailed()`
- `UUPSNotThroughActiveProxy()`
- `UUPSNotThroughProxy()`
- `UnauthorizedUpgrade()`
- `UpgradeInitializationFailed()`
- `ZeroAddress()`
- `ZeroAmount()`

## launchpad (`SidioraERC20.json`)

Source: [`precompiles/launchpad/abi/SidioraERC20.json`](../../precompiles/launchpad/abi/SidioraERC20.json).

Destination: Deployment-specific token.

Token-contract ABI used by nested calls. This is not a fixed precompile endpoint.

### Functions

- `DOMAIN_SEPARATOR() returns (bytes32)` `view`
- `PERMIT_TYPEHASH() returns (bytes32)` `view`
- `allowance(address, address) returns (uint256)` `view`
- `approve(address spender, uint256 amount) returns (bool)` `nonpayable`
- `balanceOf(address) returns (uint256)` `view`
- `bindPool(address pool_)` `nonpayable`
- `decimals() returns (uint8)` `view`
- `deploymentFactory() returns (address)` `view`
- `getPastBalance(address account, uint256 blockNumber) returns (uint256)` `view`
- `getPastTotalSupply(uint256 blockNumber) returns (uint256)` `view`
- `getPastVotes(address account, uint256 blockNumber) returns (uint256)` `view`
- `name() returns (string)` `view`
- `nonces(address) returns (uint256)` `view`
- `permit(address owner_, address spender, uint256 value, uint256 deadline, uint8 v, bytes32 r, bytes32 s)` `nonpayable`
- `pool() returns (address)` `view`
- `symbol() returns (string)` `view`
- `totalSupply() returns (uint256)` `view`
- `transfer(address to, uint256 amount) returns (bool)` `nonpayable`
- `transferFrom(address from, address to, uint256 amount) returns (bool)` `nonpayable`

### Events

- `Approval(address indexed owner, address indexed spender, uint256 value)`
- `Transfer(address indexed from, address indexed to, uint256 value)`

### Errors

- `FutureLookup()`
- `InsufficientAllowance()`
- `InsufficientBalance()`
- `InvalidPermit()`
- `InvalidPoolBinding()`
- `PermitExpired()`
- `ZeroAddress()`

## launchpad (`SidioraFactory.json`)

Source: [`precompiles/launchpad/abi/SidioraFactory.json`](../../precompiles/launchpad/abi/SidioraFactory.json).

Destination: Registered SidioraFactory surface.

ABI of the registered SidioraFactory surface. Its target address comes from chain state; it is not generally interchangeable with the direct `0x1017` ABI. Generation 1 surfaces retire with `Retired(address replacement)`.

### Functions

- `DEFAULT_ADMIN_ROLE() returns (bytes32)` `view`
- `ROUTER_ROLE() returns (bytes32)` `view`
- `createMarket(string name, string symbol, uint8 feeStrategy, address optical) returns (address tokenAddr, address poolAddr, uint256 nftId)` `nonpayable`
- `createMarketFor(address creator, string name, string symbol, uint8 feeStrategy, address optical) returns (address tokenAddr, address poolAddr, uint256 nftId)` `nonpayable`
- `eventEmitter() returns (address)` `view`
- `feeAccumulator() returns (address)` `view`
- `getNonce(address creator) returns (uint256)` `view`
- `getRoleAdmin(bytes32 role) returns (bytes32)` `view`
- `grantRole(bytes32 role, address account)` `nonpayable`
- `hasRole(bytes32 role, address account) returns (bool)` `view`
- `initialize(address _poolBeacon, address _nftContract, address _poolRegistry, address _eventEmitter, address _protocolConfig, address _treasury, address _feeAccumulator, address _usdlAddress, address _admin)` `nonpayable`
- `nftContract() returns (address)` `view`
- `poolBeacon() returns (address)` `view`
- `poolRegistry() returns (address)` `view`
- `predictPoolAddress(address token, address creator, address optical) returns (address)` `view`
- `protocolConfig() returns (address)` `view`
- `proxiableUUID() returns (bytes32)` `view`
- `registeredDefaultAdminCount() returns (uint256)` `view`
- `renounceRole(bytes32 role, address callerConfirmation)` `nonpayable`
- `revokeRole(bytes32 role, address account)` `nonpayable`
- `treasury() returns (address)` `view`
- `upgradeToAndCall(address newImplementation, bytes data)` `payable`
- `usdlAddress() returns (address)` `view`

### Events

- `Initialized(uint64 version)`
- `MarketCreated(address indexed token, address indexed pool, address indexed creator, uint256 nftId, address optical)`
- `RoleAdminChanged(bytes32 indexed role, bytes32 indexed previousAdminRole, bytes32 indexed newAdminRole)`
- `RoleGranted(bytes32 indexed role, address indexed account, address indexed sender)`
- `RoleRevoked(bytes32 indexed role, address indexed account, address indexed sender)`
- `Upgraded(address indexed implementation)`

### Errors

- `AlreadyInitialized()`
- `DuplicateToken()`
- `InsufficientCreationFee()`
- `InvalidImplementation()`
- `InvalidRoleAccount()`
- `LastDefaultAdmin()`
- `MissingRole(address account, bytes32 role)`
- `NonPayableUpgrade()`
- `NotInitializing()`
- `TransferFailed()`
- `TransferFromFailed()`
- `UUPSNotThroughActiveProxy()`
- `UUPSNotThroughProxy()`
- `UnauthorizedUpgrade()`
- `UpgradeInitializationFailed()`
- `ZeroAddress()`

## launchpad (`SidioraNFT.json`)

Source: [`precompiles/launchpad/abi/SidioraNFT.json`](../../precompiles/launchpad/abi/SidioraNFT.json).

Destination: Registered SidioraNFT surface.

ABI of the registered SidioraNFT surface. Its target address comes from chain state; it is not generally interchangeable with the direct `0x1017` ABI. Generation 1 surfaces retire with `Retired(address replacement)`.

### Functions

- `DEFAULT_ADMIN_ROLE() returns (bytes32)` `view`
- `MINTER_ROLE() returns (bytes32)` `view`
- `STRATEGY_SETTER_ROLE() returns (bytes32)` `view`
- `approve(address to, uint256 tokenId)` `nonpayable`
- `balanceOf(address owner_) returns (uint256)` `view`
- `eventEmitter() returns (address)` `view`
- `getApproved(uint256 tokenId) returns (address)` `view`
- `getFeeStrategy(uint256 tokenId) returns (uint8)` `view`
- `getPoolAddress(uint256 tokenId) returns (address)` `view`
- `getRoleAdmin(bytes32 role) returns (bytes32)` `view`
- `grantRole(bytes32 role, address account)` `nonpayable`
- `hasRole(bytes32 role, address account) returns (bool)` `view`
- `initialize(string _name, string _symbol, address _eventEmitter, address _admin)` `nonpayable`
- `isApprovedForAll(address owner_, address operator) returns (bool)` `view`
- `migratePoolIndex(uint256 maxItems)` `nonpayable`
- `mint(address to, address pool, uint8 strategy) returns (uint256 tokenId)` `nonpayable`
- `name() returns (string)` `view`
- `nextTokenId() returns (uint256)` `view`
- `ownerOf(uint256 tokenId) returns (address owner_)` `view`
- `poolIndexMigrationCursor() returns (uint256)` `view`
- `poolIndexReady() returns (bool)` `view`
- `poolTokenId(address) returns (uint256)` `view`
- `proxiableUUID() returns (bytes32)` `view`
- `registeredDefaultAdminCount() returns (uint256)` `view`
- `renounceRole(bytes32 role, address callerConfirmation)` `nonpayable`
- `revokeRole(bytes32 role, address account)` `nonpayable`
- `safeTransferFrom(address from, address to, uint256 tokenId)` `nonpayable`
- `safeTransferFrom(address from, address to, uint256 tokenId, bytes data)` `nonpayable`
- `setApprovalForAll(address operator, bool approved)` `nonpayable`
- `setFeeStrategy(uint256 tokenId, uint8 newStrategy)` `nonpayable`
- `supportsInterface(bytes4 interfaceId) returns (bool)` `view`
- `symbol() returns (string)` `view`
- `tokenByIndex(uint256 index) returns (uint256)` `view`
- `tokenOfOwnerByIndex(address owner_, uint256 index) returns (uint256)` `view`
- `totalSupply() returns (uint256)` `view`
- `transferFrom(address from, address to, uint256 tokenId)` `nonpayable`
- `upgradeToAndCall(address newImplementation, bytes data)` `payable`

### Events

- `Approval(address indexed owner, address indexed approved, uint256 indexed tokenId)`
- `ApprovalForAll(address indexed owner, address indexed operator, bool approved)`
- `FeeStrategyChanged(uint256 indexed tokenId, uint8 oldStrategy, uint8 newStrategy)`
- `Initialized(uint64 version)`
- `PoolNFTMinted(uint256 indexed tokenId, address indexed creator, address indexed pool)`
- `RoleAdminChanged(bytes32 indexed role, bytes32 indexed previousAdminRole, bytes32 indexed newAdminRole)`
- `RoleGranted(bytes32 indexed role, address indexed account, address indexed sender)`
- `RoleRevoked(bytes32 indexed role, address indexed account, address indexed sender)`
- `Transfer(address indexed from, address indexed to, uint256 indexed tokenId)`
- `Upgraded(address indexed implementation)`

### Errors

- `AlreadyInitialized()`
- `AlreadyMinted()`
- `DuplicatePool()`
- `IndexOutOfBounds()`
- `InvalidImplementation()`
- `InvalidRoleAccount()`
- `InvalidStrategy()`
- `LastDefaultAdmin()`
- `MissingRole(address account, bytes32 role)`
- `NonERC721Receiver()`
- `NonPayableUpgrade()`
- `NotApproved()`
- `NotInitializing()`
- `NotTokenOwner()`
- `PoolIndexMigrationRequired()`
- `TokenNotFound()`
- `UUPSNotThroughActiveProxy()`
- `UUPSNotThroughProxy()`
- `UnauthorizedUpgrade()`
- `UnsupportedSnapshotToken()`
- `UpgradeInitializationFailed()`
- `ZeroAddress()`

## launchpad (`SidioraPool.json`)

Source: [`precompiles/launchpad/abi/SidioraPool.json`](../../precompiles/launchpad/abi/SidioraPool.json).

Destination: Registered SidioraPool surface.

ABI of the registered SidioraPool surface. Its target address comes from chain state; it is not generally interchangeable with the direct `0x1017` ABI. Generation 1 surfaces retire with `Retired(address replacement)`.

### Functions

- `PAUSER_ROLE() returns (bytes32)` `view`
- `SNAPSHOT_INTERVAL() returns (uint256)` `view`
- `SNAPSHOT_MIN_VOLUME_BPS() returns (uint256)` `view`
- `acceptGuardian()` `nonpayable`
- `accumulatedTokenFees() returns (uint256)` `view`
- `accumulatedUsdlFees() returns (uint256)` `view`
- `creationTimestamp() returns (uint256)` `view`
- `cumulativeBuyUsdlVolume() returns (uint256)` `view`
- `cumulativeSellUsdlVolume() returns (uint256)` `view`
- `cumulativeVolume() returns (uint256)` `view`
- `currentFeeBps() returns (uint256)` `view`
- `emergencyPaused() returns (bool)` `view`
- `eventEmitter() returns (address)` `view`
- `factory() returns (address)` `view`
- `feeAccumulator() returns (address)` `view`
- `getEffectiveReserves() returns (uint256 effectiveUsdl, uint256 tokenRes)` `view`
- `getPoolInfo() returns ((address tokenAddress, address opticalAddress, uint256 virtualUsdlReserve, uint256 realUsdlBalance, uint256 tokenReserve, uint256 creationTimestamp, uint256 cumulativeVolume))` `view`
- `getPrice() returns (uint256)` `view`
- `getPriceSnapshots() returns (uint256[8])` `view`
- `getReserves() returns (uint256 virtualUsdl, uint256 realUsdl, uint256 tokenRes)` `view`
- `guardian() returns (address)` `view`
- `initialize(address _tokenAddress, address _usdlAddress, address _opticalAddress, address _feeAccumulator, address _eventEmitter, address _protocolConfig, address _guardian, uint256 _virtualUsdlReserve, uint256 _tokenReserve)` `nonpayable`
- `lastSnapshotTimestamp() returns (uint256)` `view`
- `migrateFactory(address factory_)` `nonpayable`
- `opticalAddress() returns (address)` `view`
- `pause()` `nonpayable`
- `paused() returns (bool)` `view`
- `pendingGuardian() returns (address)` `view`
- `priceSnapshots(uint256) returns (uint256)` `view`
- `protocolConfig() returns (address)` `view`
- `realUsdlBalance() returns (uint256)` `view`
- `snapshotCount() returns (uint256)` `view`
- `snapshotIndex() returns (uint256)` `view`
- `swap(uint256 amountIn, uint256 minAmountOut, bool isBuy, address recipient, uint256 deadline) returns (uint256 amountOut)` `nonpayable`
- `swapFor(uint256 amountIn, uint256 minAmountOut, bool isBuy, address trader, address recipient, uint256 deadline) returns (uint256)` `nonpayable`
- `syncReserves() returns (uint256 usdlBal, uint256 tokenBal)` `nonpayable`
- `tokenAddress() returns (address)` `view`
- `tokenReserve() returns (uint256)` `view`
- `transferGuardian(address nextGuardian)` `nonpayable`
- `unpause()` `nonpayable`
- `usdlAddress() returns (address)` `view`
- `virtualUsdlReserve() returns (uint256)` `view`

### Events

- `FactoryMigrated(address indexed factory)`
- `GuardianTransferStarted(address indexed guardian, address indexed pendingGuardian)`
- `GuardianTransferred(address indexed previousGuardian, address indexed guardian)`
- `Initialized(uint64 version)`
- `InputAdjustment(uint256 paidInput, uint256 pricedInput)`
- `PauseToggled(bool paused)`

### Errors

- `AlreadyInitialized()`
- `DeadlineExpired()`
- `DivisionByZero()`
- `InsufficientInput()`
- `InsufficientLiquidity()`
- `InvalidBalance()`
- `InvalidConfiguration()`
- `InvalidHookResponse()`
- `NotInitializing()`
- `NotPaused()`
- `Overflow()`
- `Paused()`
- `ReentrancyGuardReentrantCall()`
- `SlippageExceeded()`
- `TransferFailed()`
- `Unauthorized()`
- `VirtualFloorBreached()`
- `ZeroAddress()`
- `ZeroAmount()`

## launchpad (`Treasury.json`)

Source: [`precompiles/launchpad/abi/Treasury.json`](../../precompiles/launchpad/abi/Treasury.json).

Destination: Registered Treasury surface.

ABI of the registered Treasury surface. Its target address comes from chain state; it is not generally interchangeable with the direct `0x1017` ABI. Generation 1 surfaces retire with `Retired(address replacement)`.

### Functions

- `DEFAULT_ADMIN_ROLE() returns (bytes32)` `view`
- `DEPOSITOR_ROLE() returns (bytes32)` `view`
- `deposit(address token, uint256 amount)` `nonpayable`
- `eventEmitter() returns (address)` `view`
- `getBalance(address token) returns (uint256)` `view`
- `getRoleAdmin(bytes32 role) returns (bytes32)` `view`
- `grantRole(bytes32 role, address account)` `nonpayable`
- `hasRole(bytes32 role, address account) returns (bool)` `view`
- `initialize(address _eventEmitter, address _admin)` `nonpayable`
- `proxiableUUID() returns (bytes32)` `view`
- `recordDeposit(address token, uint256 amount)` `nonpayable`
- `recoverSurplus(address token, address to, uint256 amount)` `nonpayable`
- `registeredDefaultAdminCount() returns (uint256)` `view`
- `renounceRole(bytes32 role, address callerConfirmation)` `nonpayable`
- `revokeRole(bytes32 role, address account)` `nonpayable`
- `upgradeToAndCall(address newImplementation, bytes data)` `payable`
- `withdraw(address token, address to, uint256 amount)` `nonpayable`

### Events

- `Deposited(address indexed token, address indexed from, uint256 amount)`
- `Initialized(uint64 version)`
- `RoleAdminChanged(bytes32 indexed role, bytes32 indexed previousAdminRole, bytes32 indexed newAdminRole)`
- `RoleGranted(bytes32 indexed role, address indexed account, address indexed sender)`
- `RoleRevoked(bytes32 indexed role, address indexed account, address indexed sender)`
- `SurplusRecovered(address indexed token, address indexed to, uint256 amount)`
- `Upgraded(address indexed implementation)`
- `Withdrawn(address indexed token, address indexed to, uint256 amount)`

### Errors

- `AlreadyInitialized()`
- `InsufficientBalance()`
- `InvalidImplementation()`
- `InvalidRoleAccount()`
- `InvalidToken()`
- `LastDefaultAdmin()`
- `MissingRole(address account, bytes32 role)`
- `NonPayableUpgrade()`
- `NotInitializing()`
- `ReentrantCall()`
- `TransferFailed()`
- `TransferFromFailed()`
- `UUPSNotThroughActiveProxy()`
- `UUPSNotThroughProxy()`
- `Unauthorized()`
- `UnauthorizedUpgrade()`
- `UpgradeInitializationFailed()`
- `ZeroAddress()`
- `ZeroAmount()`

## launchpad (`abi.json`)

Source: [`precompiles/launchpad/abi.json`](../../precompiles/launchpad/abi.json).

Destination: `0x0000000000000000000000000000000000001017`.

Direct `0x1017` ABI. Shared selectors follow the role precedence described above; role-specific surfaces must use their own ABI and registered destination.

### Functions

- `DEAD_ADDRESS() returns (address)` `view`
- `DEFAULT_ADMIN_ROLE() returns (bytes32)` `view`
- `FACTORY_ROLE() returns (bytes32)` `view`
- `FEES_ROUTER_ROLE() returns (bytes32)` `view`
- `OPTICAL_CLAIM_ROLE() returns (bytes32)` `view`
- `OPTICAL_GRANTER_ROLE() returns (bytes32)` `view`
- `POOL_ROLE() returns (bytes32)` `view`
- `ROUTER_ROLE() returns (bytes32)` `view`
- `accountingInitialized() returns (bool)` `view`
- `accountingMigrationCursor() returns (uint256)` `view`
- `airdropClaimedSupply(address, uint256) returns (uint256)` `view`
- `airdropEligibleSupply(address, uint256) returns (uint256)` `view`
- `airdropEpochRemaining(address, uint256) returns (uint256)` `view`
- `airdropSnapshotBlock(address, uint256) returns (uint256)` `view`
- `allocateLegacyAirdrop(address pool, uint256 epoch, bytes32 root, uint256 budget)` `nonpayable`
- `authorizeOptical(address optical)` `nonpayable`
- `authorizePool(address pool)` `nonpayable`
- `baseFeeBps() returns (uint256)` `view`
- `burn(address pool) returns (uint256 amount)` `nonpayable`
- `buy(address pool, uint256 usdlAmountIn, uint256 minTokensOut, uint256 deadline) returns (uint256 amountOut)` `nonpayable`
- `buyWithPermit(address pool, uint256 usdlAmountIn, uint256 minTokensOut, uint256 deadline, (uint256 value, uint256 deadline, uint8 v, bytes32 r, bytes32 s) permit) returns (uint256 amountOut)` `nonpayable`
- `claim(address pool, address recipient) returns (uint256 amount)` `nonpayable`
- `claimAirdrop(address pool, address holder) returns (uint256 amount)` `nonpayable`
- `claimAirdrop(uint256 nftId) returns (uint256 amount)` `nonpayable`
- `claimAirdropEpoch(address pool, address holder, uint256 epoch) returns (uint256 amount)` `nonpayable`
- `claimAirdropEpoch(uint256 nftId, uint256 epoch) returns (uint256 amount)` `nonpayable`
- `claimFees(uint256 nftId) returns (uint256 amount)` `nonpayable`
- `claimLegacyAirdrop(address pool, uint256 epoch, address holder, uint256 amount, bytes32[] proof)` `nonpayable`
- `claimOpticalSurplus(address pool, uint256 amount, address recipient)` `nonpayable`
- `concentrationWeight() returns (uint256)` `view`
- `createMarket(string name, string symbol, uint8 feeStrategy, address optical) returns (address tokenAddr, address poolAddr, uint256 nftId)` `nonpayable`
- `createMarketFor(address creator, string name, string symbol, uint8 feeStrategy, address optical) returns (address tokenAddr, address poolAddr, uint256 nftId)` `nonpayable`
- `createMarketWithPermit(string name, string symbol, uint8 feeStrategy, address optical, (uint256 value, uint256 deadline, uint8 v, bytes32 r, bytes32 s) permit) returns (address tokenAddr, address poolAddr, uint256 nftId)` `nonpayable`
- `creationFee() returns (uint256)` `view`
- `eventEmitter() returns (address)` `view`
- `executeAirdrop(uint256 nftId) returns (uint256 amount)` `nonpayable`
- `executeBurn(uint256 nftId) returns (uint256 amount)` `nonpayable`
- `executeLpRewards(uint256 nftId) returns (uint256 amount)` `nonpayable`
- `factory() returns (address)` `view`
- `feeAccumulator() returns (address)` `view`
- `feeDecayRate() returns (uint256)` `view`
- `getAccumulatedFees(address pool) returns (uint256)` `view`
- `getAirdropBalance(address pool) returns (uint256)` `view`
- `getAirdropEpoch(address pool) returns (uint256)` `view`
- `getAllPools(uint256 offset, uint256 limit) returns (address[])` `view`
- `getLpRewardsBalance(address pool) returns (uint256)` `view`
- `getMarketCap(address pool) returns (uint256)` `view`
- `getNftIdByPool(address pool) returns (uint256)` `view`
- `getNonce(address creator) returns (uint256)` `view`
- `getOpticalSurplus(address pool) returns (uint256)` `view`
- `getPoolByToken(address token) returns (address)` `view`
- `getPoolCount() returns (uint256)` `view`
- `getPoolCountByCreator(address creator) returns (uint256)` `view`
- `getPoolMetadata(address pool) returns ((address creator, address token, address optical, uint256 nftId, uint256 createdAt, uint256 createdBlock))` `view`
- `getPoolPrice(address pool) returns (uint256)` `view`
- `getPoolStats(address pool) returns ((uint256 virtualUsdl, uint256 realUsdl, uint256 tokenReserve, uint256 cumulativeVolume, uint256 currentFeeBps, uint256 poolAge, uint256 marketCap, uint256 price) stats)` `view`
- `getPoolsByCreator(address creator) returns (address[])` `view`
- `getPoolsByCreatorPage(address creator, uint256 offset, uint256 limit) returns (address[] result)` `view`
- `getProtocolFeesPending() returns (uint256)` `view`
- `getRoleAdmin(bytes32 role) returns (bytes32)` `view`
- `grantRole(bytes32 role, address account)` `nonpayable`
- `hasClaimedAirdrop(address pool, address holder, uint256 epoch) returns (bool)` `view`
- `hasRole(bytes32 role, address account) returns (bool)` `view`
- `initialize(address _eventEmitter, address _admin)` `nonpayable`
- `initialize(address _factory, address _poolRegistry, address _protocolConfig, address _usdlAddress, address _admin)` `nonpayable`
- `initialize(address _nftContract, address _feeAccumulator, address _poolRegistry, address _admin)` `nonpayable`
- `initialize(address _poolBeacon, address _nftContract, address _poolRegistry, address _eventEmitter, address _protocolConfig, address _treasury, address _feeAccumulator, address _usdlAddress, address _admin)` `nonpayable`
- `initialize(address _poolRegistry, address _protocolConfig, address _admin)` `nonpayable`
- `initialize(address _protocolConfig, address _treasury, address _poolRegistry, address _eventEmitter, address _usdlAddress, address _admin)` `nonpayable`
- `isRegisteredPool(address pool) returns (bool)` `view`
- `legacyAirdropRemaining(address, uint256) returns (uint256)` `view`
- `legacyAirdropRoot(address, uint256) returns (bytes32)` `view`
- `legacyAirdropUnallocated(address) returns (uint256)` `view`
- `maxFeeBps() returns (uint256)` `view`
- `migrateAccounting(uint256 maxPools)` `nonpayable`
- `migrateAdditionalAccounting(address[] pools)` `nonpayable`
- `minFeeBps() returns (uint256)` `view`
- `multicall(bytes[] data) returns (bytes[] results)` `nonpayable`
- `nftContract() returns (address)` `view`
- `poolBeacon() returns (address)` `view`
- `poolRegistry() returns (address)` `view`
- `predictPoolAddress(address token, address creator, address optical) returns (address)` `view`
- `protocolConfig() returns (address)` `view`
- `protocolFeeBps() returns (uint256)` `view`
- `proxiableUUID() returns (bytes32)` `view`
- `quoteExactInput(address pool, uint256 amountIn, bool isBuy) returns ((uint256 amountOut, uint256 feeAmount, uint256 priceImpactBps) result)` `view`
- `quoteMultihop(address tokenIn, address tokenOut, uint256 amountIn) returns ((uint256 amountOut, uint256 intermediateUsdl, uint256 sellFeeAmount, uint256 buyFeeAmount, uint256 sellPriceImpactBps, uint256 buyPriceImpactBps, uint256 combinedPriceImpactBps, address poolA, address poolB) result)` `view`
- `recordFee(address pool, uint256 feeAmount)` `nonpayable`
- `recordOpticalSurplus(address pool, uint256 amount)` `nonpayable`
- `register(address pool, address token, address creator, address optical, uint256 nftId)` `nonpayable`
- `registeredDefaultAdminCount() returns (uint256)` `view`
- `renounceRole(bytes32 role, address callerConfirmation)` `nonpayable`
- `revokeRole(bytes32 role, address account)` `nonpayable`
- `sell(address pool, uint256 tokenAmountIn, uint256 minUsdlOut, uint256 deadline) returns (uint256 amountOut)` `nonpayable`
- `sellWithPermit(address pool, uint256 tokenAmountIn, uint256 minUsdlOut, uint256 deadline, (uint256 value, uint256 deadline, uint8 v, bytes32 r, bytes32 s) permit) returns (uint256 amountOut)` `nonpayable`
- `sendLpRewards(address pool) returns (uint256 amount)` `nonpayable`
- `setBaseFeeBps(uint256 newBaseFeeBps)` `nonpayable`
- `setCreationFee(uint256 newCreationFee)` `nonpayable`
- `setFeeStrategy(uint256 nftId, uint8 newStrategy)` `nonpayable`
- `setFeeWeights(uint256 newDecayRate, uint256 newVolWeight, uint256 newConcWeight)` `nonpayable`
- `setProtocolFeeBps(uint256 newProtocolFeeBps)` `nonpayable`
- `setVirtualDefaults(uint256 newVirtualUsdl, uint256 newVirtualToken)` `nonpayable`
- `supportsAirdrop(address pool) returns (bool)` `view`
- `swapTokenForToken(address tokenIn, address tokenOut, uint256 amountIn, uint256 minAmountOut, uint256 deadline) returns (uint256 amountOut, uint256 intermediateUsdl)` `nonpayable`
- `swapTokenForTokenWithPermit(address tokenIn, address tokenOut, uint256 amountIn, uint256 minAmountOut, uint256 deadline, (uint256 value, uint256 deadline, uint8 v, bytes32 r, bytes32 s) permit) returns (uint256 amountOut, uint256 intermediateUsdl)` `nonpayable`
- `totalLiabilities() returns (uint256)` `view`
- `treasury() returns (address)` `view`
- `triggerAirdrop(address pool) returns (uint256 totalAmount)` `nonpayable`
- `upgradeToAndCall(address newImplementation, bytes data)` `payable`
- `usdlAddress() returns (address)` `view`
- `virtualTokenDefault() returns (uint256)` `view`
- `virtualUsdlDefault() returns (uint256)` `view`
- `volatilityWeight() returns (uint256)` `view`

### Events

- `ConfigUpdated(bytes32 indexed key, uint256 oldValue, uint256 newValue, uint256 timestamp, uint256 blockNumber)`
- `FeeDistributed(bytes32 indexed poolId, uint256 nftId, uint8 strategy, uint256 amount, address recipient, uint256 timestamp, uint256 blockNumber)`
- `FeeFlow(uint8 indexed kind, address indexed pool, address indexed party, uint256 amount, uint256 protocolCut, uint256 poolCut, uint256 epoch, uint256 timestamp, uint256 blockNumber)`
- `FeeStrategyChanged(bytes32 indexed poolId, uint256 nftId, uint8 oldStrategy, uint8 newStrategy, uint256 timestamp, uint256 blockNumber)`
- `MarketCreated(bytes32 indexed poolId, address indexed token, address indexed creator, address pool, address optical, uint256 timestamp, uint256 blockNumber)`
- `NftMint(uint256 indexed tokenId, address indexed creator, address indexed pool, uint8 strategy, uint256 timestamp, uint256 blockNumber)`
- `NftTransfer(address indexed nft, address indexed from, address indexed to, uint256 tokenId, uint256 timestamp, uint256 blockNumber)`
- `PoolRegistered(address indexed pool, address indexed token, address indexed creator, address optical, uint256 nftId, uint256 timestamp, uint256 blockNumber)`
- `PoolStateUpdated(bytes32 indexed poolId, uint256 virtualReserve, uint256 realReserve, uint256 tokenReserve, uint256 price, uint256 timestamp, uint256 blockNumber)`
- `RouterTrade(uint8 indexed kind, address indexed pool, address indexed sender, address tokenIn, address tokenOut, uint256 amountIn, uint256 amountOut, uint256 intermediateUsdl, uint256 timestamp, uint256 blockNumber)`
- `Swap(bytes32 indexed poolId, address indexed sender, bool isBuy, uint256 amountIn, uint256 amountOut, uint256 fee, uint256 price, uint256 timestamp, uint256 blockNumber)`
- `TokenDeployed(address indexed token, address indexed pool, address indexed creator, bytes32 salt, string name, string symbol, uint8 decimals, uint256 totalSupply, uint256 timestamp, uint256 blockNumber)`

### Errors

- `AccessControlUnauthorizedAccount(address, bytes32)`
- `AirdropNotTriggered()`
- `AlreadyClaimed()`
- `AlreadyInitialized()`
- `AlreadyMinted()`
- `ApproveFailed()`
- `DeadlineExpired()`
- `DivisionByZero()`
- `DuplicatePool()`
- `DuplicateToken()`
- `FeeOutOfRange()`
- `FutureLookup()`
- `IndexOutOfBounds()`
- `InsufficientAllowance()`
- `InsufficientBalance()`
- `InsufficientCreationFee()`
- `InsufficientInput()`
- `InsufficientLiquidity()`
- `InsufficientSurplus()`
- `InvalidBalance()`
- `InvalidConfiguration()`
- `InvalidHookResponse()`
- `InvalidImplementation()`
- `InvalidPermit()`
- `InvalidPoolBinding()`
- `InvalidRoleAccount()`
- `InvalidStrategy()`
- `InvalidToken()`
- `LastDefaultAdmin()`
- `MissingRole(address, bytes32)`
- `MulticallFailed(uint256, bytes)`
- `NoFeesAccumulated()`
- `NonERC721Receiver()`
- `NonPayableUpgrade()`
- `NotApproved()`
- `NotInitializing()`
- `NotNftOwner()`
- `NotPaused()`
- `NotTokenOwner()`
- `OpticalQuoteUnsupported()`
- `Overflow()`
- `Paused()`
- `PermitExpired()`
- `PoolIndexMigrationRequired()`
- `PoolNotFound()`
- `ReentrancyGuardReentrantCall()`
- `ReentrantCall()`
- `Retired(address)`
- `SameToken()`
- `SlippageExceeded()`
- `TokenNotFound()`
- `TransferFailed()`
- `TransferFromFailed()`
- `UUPSNotThroughActiveProxy()`
- `UUPSNotThroughProxy()`
- `Unauthorized()`
- `UnauthorizedUpgrade()`
- `UnexecutableQuote()`
- `UnsupportedSnapshotToken()`
- `UpgradeInitializationFailed()`
- `VirtualFloorBreached()`
- `WrongStrategy()`
- `ZeroAddress()`
- `ZeroAmount()`

## layerxanchor (`abi.json`)

Source: [`precompiles/layerxanchor/abi.json`](../../precompiles/layerxanchor/abi.json).

Destination: `0x0000000000000000000000000000000000001014`.

Retained historical ABI. Writes are retired at `v7.0`; views are retired at `v7.0-anchor-store-delete`. All calls after store deletion revert with `LXP_MODULE_DISABLED: module disabled; layerxAnchor retired`. These checkpoint and bond declarations do not provide current network finality.

### Functions

- `activateGuarantor(bytes32 guarantorId) returns (bool)` `nonpayable`
- `beginUnbond(bytes32 guarantorId, uint256 amount) returns (uint64 completionTime)` `nonpayable`
- `checkpoint(uint64 batchNumber) returns ((uint64 batchNumber, bytes32 checkpointId, bytes32 headerDigest, uint64 epoch, uint64 firstSequence, uint64 lastSequence, bytes32 previousStateRoot, bytes32 stateRoot, bytes32 receiptRoot, bytes32 dataAvailabilityRoot, bytes32 sequencerId, uint64 timestampMs, uint8 status, uint8 signers, uint8 availabilityMask, uint32 openChallenges, uint64 submittedHeight, uint64 finalizedHeight))` `view`
- `checkpointBatch(bytes32 checkpointId) returns (uint64 batchNumber, uint8 status)` `view`
- `checkpointGuarantors(uint64 batchNumber) returns (bytes32[] guarantorIds)` `view`
- `completeUnbond(bytes32 guarantorId) returns (uint256 amount)` `nonpayable`
- `finalize(uint64 batchNumber) returns (bool)` `nonpayable`
- `finalizedReceiptRoot(uint64 batchNumber) returns (bytes32 receiptRoot, bool finalized)` `view`
- `finalizedStateRoot(uint64 batchNumber) returns (bytes32 stateRoot, bool finalized)` `view`
- `guarantor(bytes32 guarantorId) returns ((bytes32 guarantorId, address signer, address operator, uint256 bond, uint256 unbonding, uint8 status, bool eligible))` `view`
- `increaseBond(bytes32 guarantorId) returns (bool)` `payable`
- `latestFinalized() returns (uint64 batchNumber, bool exists)` `view`
- `openChallenge(uint64 batchNumber, uint8 kind, bytes32 evidenceHash) returns (uint64 challengeId)` `payable`
- `registerGuarantor(bytes32 guarantorId, address signer) returns (bool)` `payable`
- `resolveChallenge(uint64 challengeId, bool upheld) returns (bool)` `nonpayable`
- `setSequencerAuthorization(bytes32 sequencerId, bytes32 publicKey, uint64 firstBatchNumber, uint64 lastBatchNumber) returns (bool)` `nonpayable`
- `statusOf(uint64 batchNumber) returns (uint8)` `view`
- `submitAvailabilityAttestation(bytes attestation) returns (uint8 availabilityMask)` `nonpayable`
- `submitCheckpoint(bytes header, bytes headerSignature, bytes certificate) returns (bytes32 checkpointId, uint8 status)` `nonpayable`
- `submitEquivocation(bytes evidenceA, bytes evidenceB) returns (uint256 slashed)` `nonpayable`
- `threshold() returns (uint32)` `view`

### Events

- `AvailabilityAttested(uint64 indexed batchNumber, bytes32 indexed guarantorId, uint8 classMask, uint8 availabilityMask)`
- `BondIncreased(bytes32 indexed guarantorId, uint256 amount, uint256 bond)`
- `ChallengeOpened(uint64 indexed challengeId, uint64 indexed batchNumber, uint8 kind, bytes32 evidenceHash, address challenger)`
- `ChallengeResolved(uint64 indexed challengeId, uint64 indexed batchNumber, bool upheld)`
- `CheckpointFinalized(uint64 indexed batchNumber, bytes32 indexed checkpointId, bytes32 stateRoot, bytes32 receiptRoot)`
- `CheckpointSubmitted(uint64 indexed batchNumber, bytes32 indexed checkpointId, bytes32 stateRoot, bytes32 receiptRoot, uint8 signers)`
- `GuarantorActivated(bytes32 indexed guarantorId)`
- `GuarantorRegistered(bytes32 indexed guarantorId, address indexed signer, address operator, uint256 bond, uint8 status)`
- `GuarantorSlashed(bytes32 indexed guarantorId, uint8 reason, uint64 batchNumber, uint256 amount, address reporter, uint256 reporterReward)`
- `SequencerAuthorized(bytes32 indexed sequencerId, bytes32 publicKey, uint64 firstBatchNumber, uint64 lastBatchNumber)`
- `UnbondBegun(bytes32 indexed guarantorId, uint256 amount, uint64 completionTime)`
- `UnbondCompleted(bytes32 indexed guarantorId, uint256 amount)`

## layerxauth (`abi.json`)

Source: [`precompiles/layerxauth/abi.json`](../../precompiles/layerxauth/abi.json).

Destination: `0x000000000000000000000000000000000000101C`.

Served after merge activation with the node-supplied keepers and registered constructor.

### Functions

- `capabilityScope(bytes32 grantId) returns (uint64 moduleMask, uint16 ordinalMin, uint16 ordinalMax, bytes32 assetId, uint128 maxPerActivity, uint128 maxTotal, uint128 spentTotal, uint64 periodLength, uint128 maxPerPeriod, uint128 spentThisPeriod, uint64 periodStart, bytes32 purposeHash)` `view`
- `checkActivity(bytes32 grantId, uint32 activityType, bytes32 assetId, uint128 amount) returns (string code)` `view`
- `getGrant(bytes32 grantId) returns (bytes grant, bool revoked, uint64 revokedAt)` `view`
- `grantCapability(bytes grant) returns (bytes32 grantId)` `nonpayable`
- `grantSession(bytes grant, uint64 expirySequence, bytes32 actionKey) returns (bytes32 grantId)` `nonpayable`
- `grantSessionSuccessor(bytes grant, uint64 expirySequence, bytes32 actionKey, bytes32 predecessorId, bytes32 expectedCommitment) returns (bytes32 grantId)` `nonpayable`
- `isLive(bytes32 grantId, uint64 sequence, uint64 timestampMs) returns (bool live, string code)` `view`
- `recordLifecycle(bytes payload, bytes32 idempotencyKey) returns (uint64 globalSequence)` `nonpayable`
- `revokeGrant(bytes32 grantId, uint8 reason, uint64 sequence)` `nonpayable`
- `sessionKey(bytes32 grantId) returns (bytes summary)` `view`

### Events

- `CapabilityGranted(bytes32 indexed grantId)`
- `GrantBody(bytes32 indexed grantId, uint8 sub, uint8 chunk, bytes body)`
- `GrantRevoked(bytes32 indexed grantId, bytes record)`
- `IdentityStateUpdated(bytes32 indexed didId, bytes state)`
- `SessionGranted(bytes32 indexed grantId, bytes summary)`

## layerxbridge (`abi.json`)

Source: [`precompiles/layerxbridge/abi.json`](../../precompiles/layerxbridge/abi.json).

Destination: `0x0000000000000000000000000000000000001016`.

### Functions

- `bridgeIn(uint64 chain, address vault, bytes32 txHash, uint64 logIndex, bytes32 recipient, address asset, uint256 amount, bytes[] signatures) returns (string denom)` `nonpayable`
- `bridgeOut(uint64 chain, address asset, uint256 amount, address recipient) returns (uint64 nonce)` `nonpayable`
- `getAttestors() returns (address[] signers, uint256[] bonds, uint32 threshold)` `view`
- `getCap(uint64 chain, address asset) returns (string denom, uint256 maxInFlight, uint256 maxPerTx, uint256 inFlight)` `view`
- `getChain(uint64 chain) returns (bool registered, address vault, uint64 finalityDepth, bool enabled)` `view`
- `isNullified(uint64 chain, bytes32 txHash, uint64 logIndex) returns (bool)` `view`
- `isPaused() returns (bool)` `view`

### Events

- `BridgeIn(uint64 indexed chain, bytes32 indexed txHash, address indexed recipient, uint64 logIndex, address asset, uint256 amount, string denom)`
- `BridgeOut(uint64 indexed chain, address indexed asset, uint256 amount, address recipient, uint64 indexed nonce)`

## layerxchains (`abi.json`)

Source: [`precompiles/layerxchains/abi.json`](../../precompiles/layerxchains/abi.json).

Destination: `0x000000000000000000000000000000000000101E`.

Served after merge activation. These operations manage tenant domains in the shared database, including their allowances and lifecycle; creating one does not launch a separate node network.

### Functions

- `chainByName(string name) returns (uint32)` `view`
- `chainCount() returns (uint32)` `view`
- `chainIdOf(uint32 id) returns (uint64)` `pure`
- `createChain(string name, uint64 gasAllowance, uint64 byteAllowance, uint16 feeShareBps) returns (uint32 id)` `payable`
- `fundChain(uint32 id)` `payable`
- `getChain(uint32 id) returns (uint32 id, string name, address operator, uint8 status, uint64 chainId, uint64 gasAllowance, uint64 byteAllowance, uint16 feeShareBps, uint256 bond, uint256 baseFeePerGas, uint64 createdHeight)` `view`
- `governancePause(uint32 id, bool paused)` `nonpayable`
- `params() returns (address authority, uint256 minBond, uint256 creationFee, uint64 minGasAllowance, uint64 maxGasAllowance, uint64 totalGasAllowanceLeft, uint32 maxChains)` `view`
- `pauseChain(uint32 id)` `nonpayable`
- `resumeChain(uint32 id)` `nonpayable`
- `retireChain(uint32 id)` `nonpayable`
- `setOperator(uint32 id, address operator)` `nonpayable`

### Events

- `ChainCreated(uint32 indexed id, string name, address indexed operator, uint64 chainId)`
- `ChainFunded(uint32 indexed id, address indexed from, uint256 amount)`
- `ChainOperatorChanged(uint32 indexed id, address indexed operator)`
- `ChainStatusChanged(uint32 indexed id, uint8 status)`

## layerxcustody (`abi.json`)

Source: [`precompiles/layerxcustody/abi.json`](../../precompiles/layerxcustody/abi.json).

Destination: `0x0000000000000000000000000000000000001013`.

Retained historical ABI. Every method below, including views, deposit/depositToken, withdrawal, forced exit and deposit-root methods, is retired from the merge height and reverts with `LXP_MODULE_DISABLED: module disabled; layerxCustody retired`. Use native bank balances and moves, or the outward bridge.

### Functions

- `assetByPointer(address pointer) returns (bytes32 assetId)` `view`
- `deposit(bytes32 beneficiary) returns (bytes32 depositId)` `payable`
- `depositCount() returns (uint64 count)` `view`
- `depositNonce(address depositor, bytes32 assetId) returns (uint64 nonce)` `view`
- `depositRegistrationDigest(bytes32 checkpointId) returns (bytes32 commitment)` `view`
- `depositRootAuthority() returns (bytes32 authority)` `view`
- `depositRootRegistered(bytes32 checkpointId) returns (bool registered)` `view`
- `depositToken(address pointer, uint256 amount, bytes32 beneficiary) returns (bytes32 depositId)` `nonpayable`
- `executeForcedExit(bytes witness, uint64 batchNumber, bytes32 account, bytes32 assetId, address recipient, bytes recipientSignature) returns (bytes32 claimId)` `nonpayable`
- `exitEligible() returns (bool eligible)` `view`
- `finaliseWithdrawal(bytes receipt, bytes proof, bytes header, bytes headerSignature) returns (bytes32 claimId)` `nonpayable`
- `getAsset(bytes32 assetId) returns ((bytes32 assetId, string denom, address pointer, bool enabled, bool paused, uint256 minimumDeposit, uint256 custodyCap, uint256 custodied, uint256 released, uint256 pending) asset)` `view`
- `getClaim(bytes32 claimId) returns ((bytes32 claimId, uint8 kind, uint8 status, bytes32 nullifier, bytes32 withdrawalId, bytes32 account, bytes32 assetId, string denom, address recipient, uint256 amount, uint64 batchNumber, bytes32 anchor, uint64 availableAt) claim)` `view`
- `getDeposit(bytes32 depositId) returns ((bytes32 depositId, uint64 index, address depositor, bytes32 beneficiary, bytes32 assetId, string denom, uint256 amount, uint64 nonce, uint64 height) record)` `view`
- `getDepositByIndex(uint64 index) returns ((bytes32 depositId, uint64 index, address depositor, bytes32 beneficiary, bytes32 assetId, string denom, uint256 amount, uint64 nonce, uint64 height) record)` `view`
- `nativeAssetId() returns (bytes32 assetId)` `view`
- `nullifierStatus(bytes32 nullifier) returns (uint8 status)` `view`
- `registerDepositRoot(bytes registration, bytes signature, bytes32[] leafOrdering)` `nonpayable`
- `requestForcedExit(bytes witness, uint64 batchNumber, bytes32 account, bytes32 assetId, address recipient, bytes recipientSignature) returns (bytes32 claimId, uint64 availableAt)` `nonpayable`
- `requestWithdrawal(bytes receipt, bytes proof, bytes header, bytes headerSignature) returns (bytes32 claimId, uint64 availableAt)` `nonpayable`

### Events

- `ClaimFinalised(bytes32 indexed claimId, bytes32 indexed nullifier)`
- `ClaimQueued(bytes32 indexed claimId, bytes32 indexed nullifier, bytes32 indexed checkpointHash, bytes32 assetId, address recipient, uint256 amount, uint64 availableAt)`
- `CustodyDeposit(bytes32 indexed depositId, bytes32 indexed assetId, address indexed payer, bytes32 beneficiary, uint256 amount, uint64 nonce)`
- `CustodyRelease(bytes32 indexed claimId, bytes32 indexed assetId, address indexed recipient, uint256 amount, address settlementModule)`
- `DepositRootRegistered(bytes32 indexed checkpointId, bytes32 indexed depositRoot, bytes32 commitment, uint16 version)`
- `EmergencyExitExecuted(bytes32 indexed claimId, bytes32 indexed nullifier, bytes32 indexed checkpointHash, bytes32 account, bytes32 assetId, address recipient, uint256 amount)`

## layerxexchange (`abi.json`)

Source: [`precompiles/layerxexchange/abi.json`](../../precompiles/layerxexchange/abi.json).

Destination: `0x0000000000000000000000000000000000001015`.

Historical ABI for the implementation below `v6.12.0`. The current address serves the perps ABI instead. Old intent submission is retired with keeper `ErrRetired` (code `14`, reason `LXP_EXCHANGE_RETIRED`). Do not encode current perps reads with these batch-number/witness signatures.

### Functions

- `cancelOrder(bytes32 orderId) returns (bytes32 intentId)` `nonpayable`
- `depositMargin(bytes32 account) returns (bytes32 intentId, bytes32 depositId)` `payable`
- `depositMarginToken(address pointer, uint256 amount, bytes32 account) returns (bytes32 intentId, bytes32 depositId)` `nonpayable`
- `getIntent(bytes32 intentId) returns ((bytes32 intentId, uint8 kind, uint8 status, address owner, uint64 nonce, uint64 height, bytes32 account, bytes32 assetId, string denom, uint256 amount, bytes32 depositId, bytes32 marketId, uint8 side, uint256 price, uint256 quantity, uint8 timeInForce, bytes32 orderId, bytes32 positionId) intent)` `view`
- `getMargin(bytes32 account, bytes32 assetId, uint64 batchNumber, bytes witness) returns ((uint64 batchNumber, bytes32 stateRoot, bytes32 account, bytes32 assetId, uint256 balance, bool frozen) margin)` `view`
- `getMarket(bytes32 marketId, uint64 batchNumber, bytes witness) returns ((uint64 batchNumber, bytes32 stateRoot, bytes key, bytes value) record)` `view`
- `getOrder(bytes32 marketId, bytes32 orderId, uint64 batchNumber, bytes witness) returns ((uint64 batchNumber, bytes32 stateRoot, bytes key, bytes value) record)` `view`
- `getPosition(bytes32 marketId, bytes32 positionId, uint64 batchNumber, bytes witness) returns ((uint64 batchNumber, bytes32 stateRoot, bytes key, bytes value) record)` `view`
- `intentNonce(address owner) returns (uint64 nonce)` `view`
- `placeOrder(bytes32 market, uint8 side, uint256 price, uint256 qty, uint8 tif) returns (bytes32 intentId)` `nonpayable`
- `requestSettlement(bytes32 positionId) returns (bytes32 intentId)` `nonpayable`
- `withdrawMargin(bytes32 account, bytes32 assetId, uint256 amount) returns (bytes32 intentId)` `nonpayable`

### Events

- `MarginDeposited(bytes32 indexed intentId, bytes32 indexed account, address indexed owner, bytes32 assetId, uint256 amount, bytes32 depositId, uint64 nonce)`
- `MarginWithdrawalRequested(bytes32 indexed intentId, bytes32 indexed account, address indexed owner, bytes32 assetId, uint256 amount, uint64 nonce)`
- `OrderCancelRequested(bytes32 indexed intentId, bytes32 indexed orderId, address indexed owner, uint64 nonce)`
- `OrderPlaced(bytes32 indexed intentId, bytes32 indexed marketId, address indexed owner, uint8 side, uint256 price, uint256 quantity, uint8 timeInForce, uint64 nonce)`
- `SettlementRequested(bytes32 indexed intentId, bytes32 indexed positionId, address indexed owner, uint64 nonce)`

## layerxledger (`abi.json`)

Source: [`precompiles/layerxledger/abi.json`](../../precompiles/layerxledger/abi.json).

Destination: `0x000000000000000000000000000000000000101A`.

Served after merge activation. Payer-grant methods `issueGrant`, `revokeGrant`, `draw` and `getPayerGrant` are retained but refuse with `LXP_NO_PAYER_GRANT`; see the lifecycle table. `payExact` uses a Unix expiry in milliseconds, returns no ABI values, and emits `PaymentExact` on success.

### Functions

- `accountId(bytes32 owner, bytes32 assetId) returns (bytes32 accountId)` `view`
- `accountOwner(bytes32 accountId) returns (bytes32 owner)` `view`
- `balanceOf(bytes32 accountId, bytes32 assetId) returns (uint128 balance)` `view`
- `burn(bytes32 assetId, bytes32 accountId, uint128 amount)` `nonpayable`
- `draw(bytes32 grantId, bytes32 recipient, uint128 amount, bytes32 idempotencyKey, uint64 sequence, bytes32 purposeHash, bytes32 contextHash)` `nonpayable`
- `getAsset(bytes32 assetId) returns (bytes32 assetId, string symbol, string name, uint8 decimals, uint128 supplyCap, bytes32 issuerDid, bytes32 salt, uint8 issuerKind, uint8 custodyKind, bytes custodyReference, bool paused, uint128 totalUnits)` `view`
- `getPayerGrant(bytes32 grantId) returns (bytes grant, uint128 drawnTotal, uint128 drawnThisPeriod, uint64 windowStart, uint64 revokedAt, bool revoked, bool invoiceSettled)` `view`
- `issueGrant(bytes grant) returns (bytes32 grantId)` `nonpayable`
- `mint(bytes32 assetId, bytes32 accountId, uint128 amount)` `nonpayable`
- `nextSequence(bytes32 accountId, bytes32 assetId) returns (uint64 sequence)` `view`
- `openAccount(bytes32 assetId) returns (bytes32 accountId)` `nonpayable`
- `pause(bytes32 assetId)` `nonpayable`
- `payExact(bytes32 from, bytes32 to, bytes32 assetId, uint128 amount, uint64 expiresAtMs, bytes32 paymentReference)` `nonpayable`
- `register(bytes32 assetId, bytes32 salt, string symbol, string name, uint8 decimals, uint128 supplyCap, uint8 issuerKind, bytes custodyReference) returns (bytes32 assetId)` `nonpayable`
- `revokeGrant(bytes32 grantId, uint64 revocationSequence)` `nonpayable`
- `send(bytes envelope)` `nonpayable`
- `totalUnits(bytes32 assetId) returns (uint128 totalUnits)` `view`
- `unpause(bytes32 assetId)` `nonpayable`

### Events

- `AccountOpened(bytes32 indexed accountId)`
- `AssetPaused(bytes32 indexed assetId)`
- `AssetRegistered(bytes32 indexed assetId)`
- `AssetUnpaused(bytes32 indexed assetId)`
- `PayerGrantIssued(bytes32 indexed grantId)`
- `PayerGrantRevoked(bytes32 indexed grantId)`
- `PaymentDrawn(bytes32 indexed grantId, bytes32 indexed from, bytes32 indexed to, bytes32 assetId, uint128 amount, bytes32 idempotencyKey, bytes32 contextHash)`
- `PaymentExact(bytes32 indexed paymentReference, bytes32 indexed from, bytes32 indexed to, bytes32 assetId, uint128 amount)`
- `Transfer(bytes32 indexed from, bytes32 indexed to, bytes32 indexed assetId, uint128 amount, uint8 kind)`

## layerxpay (`abi.json`)

Source: [`precompiles/layerxpay/abi.json`](../../precompiles/layerxpay/abi.json).

Destination: `0x000000000000000000000000000000000000101B`.

Served after merge activation. `budgetPull` is always retired and reverts with `LXP_MODULE_DISABLED: module disabled; layerxpay retired`. Other budget, escrow, service and stream declarations are subject to their normal keeper checks.

### Functions

- `budgetAmend(bytes32 budgetId, uint128 perPeriodLimit, uint128 carryCap, uint64 expiryMs, uint8 rolloverPolicy)` `nonpayable`
- `budgetClose(bytes32 budgetId, uint64 revocationSequence)` `nonpayable`
- `budgetCreate(bytes32 budgetId, bytes32 budgetAccount, bytes32 assetId, bytes32 purposeHash, uint128 perPeriodLimit, uint128 carryCap, uint128 amount, uint64 periodLengthMs, uint64 periodStartMs, uint64 expiryMs, uint64 revocationSequence, uint8 rolloverPolicy)` `nonpayable`
- `budgetCreateNative(bytes32 budgetId, bytes32 budgetAccount, bytes32 assetId, bytes32 purposeHash, uint128 perPeriodLimit, uint128 carryCap, uint128 amount, uint64 periodLengthMs, uint64 periodStartMs, uint64 expiryMs, uint64 revocationSequence, uint8 rolloverPolicy, bytes32 sourceAccount, uint64 sourceSequence)` `nonpayable`
- `budgetDefund(bytes32 budgetId, uint128 amount)` `nonpayable`
- `budgetDelegateAdd(bytes32 budgetId, bytes32 delegateActor)` `nonpayable`
- `budgetDelegateRemove(bytes32 budgetId, bytes32 delegateActor)` `nonpayable`
- `budgetDelegateSpend(bytes32 budgetId, bytes32 recipient, uint128 amount)` `nonpayable`
- `budgetFund(bytes32 budgetId, uint128 amount)` `nonpayable`
- `budgetFundNative(bytes32 budgetId, uint128 amount, uint64 sourceSequence)` `nonpayable`
- `budgetPull(bytes32 budgetId, bytes32 recipient, uint128 amount, bytes payerGrant)` `nonpayable`
- `budgetRemaining(bytes32 budgetId) returns (uint128 remaining)` `view`
- `budgetRevoke(bytes32 budgetId, uint64 revocationSequence)` `nonpayable`
- `budgetSpend(bytes32 budgetId, bytes32 recipient, uint128 amount)` `nonpayable`
- `escrowCapture(bytes32 escrowId, uint128 amount, bytes32 idempotencyKey)` `nonpayable`
- `escrowDisputeOpen(bytes32 escrowId)` `nonpayable`
- `escrowDisputeResolve(bytes32 escrowId, uint32 beneficiaryBps, bytes32 idempotencyKey)` `nonpayable`
- `escrowOpen(bytes32 escrowId, bytes32 owner, bytes32 escrowAccount, bytes32 beneficiary, bytes32 arbiter, bytes32 assetId, uint128 amount, uint64 expiryMs, uint64 disputeWindowMs, bytes32 termsHash, bytes32 agreementReference)` `nonpayable`
- `escrowPartialCapture(bytes32 escrowId, uint128 amount, bytes32 idempotencyKey)` `nonpayable`
- `escrowRelease(bytes32 escrowId, bytes32 idempotencyKey)` `nonpayable`
- `escrowRemaining(bytes32 escrowId) returns (uint128 remaining)` `view`
- `escrowTimeout(bytes32 escrowId, bytes32 idempotencyKey)` `nonpayable`
- `getAgreement(bytes32 agreementId) returns ((bytes32 agreementId, bytes32 offerId, bytes32 provider, bytes32 buyer, bytes32 termsHash, bytes32 escrowId, uint64 deliveryDeadlineMs, uint64 acceptanceWindowEndMs, uint64 disputeWindowEndMs, uint8 defaultOutcome, uint8 state, uint64 acceptedSequence, uint16 rejectionReason, bytes32[] contestedHashes, bool defaultApplied, uint64 outcomeSequence, uint64 outcomeTimestampMs) agreement)` `view`
- `getBudget(bytes32 budgetId) returns ((bytes32 owner, bytes32 budgetAccount, bytes32 assetId, bytes32 purposeHash, uint128 perPeriodLimit, uint128 configuredPeriodLimit, uint128 carryCap, uint128 spentThisPeriod, uint128 carried, uint64 periodLengthMs, uint64 periodStartMs, uint64 expiryMs, uint64 revocationSequence, uint8 rolloverPolicy, bool closed, bool revoked, bytes32[] delegates, bytes32 sourceAccount) budget)` `view`
- `getCommitment(bytes32 commitmentId) returns ((bytes32 commitmentId, bytes32 activityId, bytes32 provider, bytes32 agreementId, bytes32 taskHash, uint64 deadlineMs, uint64 resourceBound, bytes32 escrowId, uint64 globalSequence, bool abandoned, uint16 abandonReason) commitment)` `view`
- `getDelivery(bytes32 deliveryId) returns ((bytes32 agreementId, bytes32 provider, (bytes32 hash, uint64 artifactSizeBytes, bytes32 availabilityReference)[] deliverables, uint64 globalSequence) delivery)` `view`
- `getDispute(bytes32 disputeId) returns ((bytes32 disputeId, bytes32 activityId, bytes32 agreementId, bytes32 raiser, bytes32[] evidenceHashes, uint64 globalSequence, bool resolved, uint16 ruling, uint32 providerBps, bytes32 escrowResolutionId, uint64 resolutionSequence) dispute)` `view`
- `getEscrow(bytes32 escrowId) returns ((bytes32 owner, bytes32 escrowAccount, bytes32 beneficiary, bytes32 arbiter, bytes32 assetId, uint128 lockedAmount, uint128 capturedAmount, uint8 state, uint64 expiryMs, uint64 disputeWindowMs, bytes32 termsHash, bytes32 agreementReference) escrow)` `view`
- `getEscrowResult(bytes32 idempotencyKey) returns (bytes result)` `view`
- `getExecution(bytes32 attestationId) returns (bytes execution)` `view`
- `getOffer(bytes32 offerId) returns ((bytes32 offerId, bytes32 activityId, bytes32 offeringAgent, bytes32 assetId, uint128 price, bytes32 termsHash, bytes32 deliverableSpecHash, uint64 deliveryDeadlineMs, uint64 acceptanceWindowMs, uint64 disputeWindowMs, uint8 defaultOutcome, uint64 offerExpiryMs, uint64 globalSequence, bool withdrawn, bool accepted) offer)` `view`
- `getProgress(bytes32 reportId) returns ((bytes32 reportId, bytes32 activityId, bytes32 commitmentId, bytes32 agreementId, bytes32 provider, bytes32 noteHash, bytes32 availabilityReference, uint32 progressBps, uint64 reportedAtMs, uint64 globalSequence) progress)` `view`
- `getStream(bytes32 streamId) returns ((bytes32 payer, bytes32 streamAccount, bytes32 recipient, bytes32 assetId, uint8 mode, uint128 rate, uint64 rateUnit, uint64 startMs, uint64 lastAccrualMs, uint64 endMs, uint128 totalCap, uint128 accruedTotal, uint128 settledTotal, uint128 remainderCarry, uint64 cumulativeMeter, bytes32[] meterAuthorities, bool underfunded, bool paused, bool closed) stream)` `view`
- `getStreamResult(bytes32 idempotencyKey) returns ((bytes32 streamId, bytes32 transferSetRoot, uint128 paid, uint128 refunded, uint16 ordinal, uint8 legCount) result)` `view`
- `latestDelivery(bytes32 agreementId) returns ((bytes32 agreementId, bytes32 provider, (bytes32 hash, uint64 artifactSizeBytes, bytes32 availabilityReference)[] deliverables, uint64 globalSequence) delivery)` `view`
- `progressHighWater(bytes32 commitmentId) returns (uint32 progressBps)` `view`
- `serviceAccept(bytes32 agreementId)` `nonpayable`
- `serviceAgreementAccept(bytes32 agreementId)` `nonpayable`
- `serviceAgreementPropose(bytes32 agreementId, bytes32 offerId, bytes32 termsHash, bytes32 escrowId)` `nonpayable`
- `serviceCommitAbandon(bytes32 commitmentId, uint16 reason)` `nonpayable`
- `serviceCommitTask(bytes32 commitmentId, bytes32 agreementId, bytes32 taskHash, bytes32 escrowId, uint64 deadlineMs, uint64 resourceBound)` `nonpayable`
- `serviceDeliver(bytes32 deliveryId, bytes32 agreementId, (bytes32 hash, uint64 artifactSizeBytes, bytes32 availabilityReference)[] deliverables)` `nonpayable`
- `serviceDisputeOpen(bytes32 disputeId, bytes32 agreementId, bytes32[] evidenceHashes)` `nonpayable`
- `serviceDisputeResolve(bytes32 disputeId, uint16 ruling, uint32 providerBps, bytes32 escrowResolutionId)` `nonpayable`
- `serviceOfferPublish(bytes32 offerId, bytes32 assetId, uint128 price, bytes32 termsHash, bytes32 deliverableSpecHash, uint64 deliveryDeadlineMs, uint64 acceptanceWindowMs, uint64 disputeWindowMs, uint8 defaultOutcome, uint64 offerExpiryMs)` `nonpayable`
- `serviceOfferWithdraw(bytes32 offerId)` `nonpayable`
- `serviceProgressReport(bytes32 reportId, bytes32 commitmentId, bytes32 noteHash, bytes32 availabilityReference, uint32 progressBps)` `nonpayable`
- `serviceReject(bytes32 agreementId, uint16 reason, bytes32[] contestedHashes)` `nonpayable`
- `serviceToolExecAttest(bytes execution)` `nonpayable`
- `streamAccrued(bytes32 streamId) returns (uint128 accruedTotal, uint128 unsettled)` `view`
- `streamClose(bytes32 streamId, bytes32 idempotencyKey)` `nonpayable`
- `streamMeter(bytes32 streamId, uint64 cumulativeReading, bytes32 authorityKey, bytes signature)` `nonpayable`
- `streamOpen(bytes32 streamId, bytes32 streamAccount, bytes32 recipient, bytes32 assetId, uint8 mode, uint128 rate, uint64 rateUnit, uint64 startMs, uint64 endMs, uint128 totalCap, uint128 initialFunding, bytes32[] meterAuthorities)` `nonpayable`
- `streamPause(bytes32 streamId)` `nonpayable`
- `streamResume(bytes32 streamId)` `nonpayable`
- `streamSettle(bytes32 streamId, bytes32 idempotencyKey)` `nonpayable`
- `streamTopUp(bytes32 streamId, uint128 amount)` `nonpayable`

### Events

- `AgreementAccepted(bytes32 indexed primary, bytes32 indexed secondary, uint8 code, uint64 sequence)`
- `AgreementProposed(bytes32 indexed primary, bytes32 indexed secondary, uint8 code, uint64 sequence)`
- `BudgetAmended(bytes32 indexed budgetId, uint128 perPeriodLimit, uint64 expiryMs)`
- `BudgetClosed(bytes32 indexed budgetId, uint128 drained, uint64 revocationSequence)`
- `BudgetCreated(bytes32 indexed budgetId, bytes32 assetId, uint128 amount)`
- `BudgetDefunded(bytes32 indexed budgetId, uint128 amount, uint128 perPeriodLimit)`
- `BudgetDelegateAdded(bytes32 indexed budgetId, bytes32 delegateActor)`
- `BudgetDelegateRemoved(bytes32 indexed budgetId, bytes32 delegateActor)`
- `BudgetFunded(bytes32 indexed budgetId, uint128 amount)`
- `BudgetRevoked(bytes32 indexed budgetId, uint128 drained, uint64 revocationSequence)`
- `BudgetSpent(bytes32 indexed budgetId, bytes32 recipient, uint128 amount)`
- `CommitAbandoned(bytes32 indexed primary, bytes32 indexed secondary, uint8 code, uint64 sequence)`
- `DefaultOutcomeApplied(bytes32 indexed agreementId, uint8 state, bool defaultApplied, uint64 outcomeSequence, uint64 outcomeTimestampMs)`
- `Delivered(bytes32 indexed primary, bytes32 indexed secondary, uint8 code, uint64 sequence)`
- `DisputeOpened(bytes32 indexed primary, bytes32 indexed secondary, uint8 code, uint64 sequence)`
- `DisputeResolved(bytes32 indexed primary, bytes32 indexed secondary, uint8 code, uint64 sequence)`
- `EscrowCaptured(bytes32 indexed escrowId, uint16 ordinal, uint8 state, uint128 capturedAmount, uint128 lockedAmount)`
- `EscrowDisputed(bytes32 indexed escrowId, uint16 ordinal, uint8 state, uint128 capturedAmount, uint128 lockedAmount)`
- `EscrowOpened(bytes32 indexed escrowId, uint16 ordinal, uint8 state, uint128 capturedAmount, uint128 lockedAmount)`
- `EscrowPartiallyCaptured(bytes32 indexed escrowId, uint16 ordinal, uint8 state, uint128 capturedAmount, uint128 lockedAmount)`
- `EscrowReleased(bytes32 indexed escrowId, uint16 ordinal, uint8 state, uint128 capturedAmount, uint128 lockedAmount)`
- `EscrowResolved(bytes32 indexed escrowId, uint16 ordinal, uint8 state, uint128 capturedAmount, uint128 lockedAmount)`
- `EscrowTimedOut(bytes32 indexed escrowId, uint16 ordinal, uint8 state, uint128 capturedAmount, uint128 lockedAmount)`
- `ExecutionAttested(bytes32 indexed primary, bytes32 indexed secondary, uint8 code, uint64 sequence)`
- `OfferPublished(bytes32 indexed primary, bytes32 indexed secondary, uint8 code, uint64 sequence)`
- `OfferWithdrawn(bytes32 indexed primary, bytes32 indexed secondary, uint8 code, uint64 sequence)`
- `OutcomeAccepted(bytes32 indexed primary, bytes32 indexed secondary, uint8 code, uint64 sequence)`
- `OutcomeRejected(bytes32 indexed primary, bytes32 indexed secondary, uint8 code, uint64 sequence)`
- `ProgressReported(bytes32 indexed primary, bytes32 indexed secondary, uint8 code, uint64 sequence)`
- `StreamClosed(bytes32 indexed streamId, uint128 paid, uint128 refunded)`
- `StreamMetered(bytes32 indexed streamId, uint64 cumulativeMeter, uint128 accrued, uint128 accruedTotal)`
- `StreamOpened(bytes32 indexed streamId, bytes32 payer, bytes32 streamAccount, bytes32 recipient, bytes32 assetId, uint8 mode, uint128 funding)`
- `StreamPaused(bytes32 indexed streamId, uint64 lastAccrualMs, uint128 accruedTotal)`
- `StreamRateUpdated(bytes32 indexed streamId, uint128 oldRate, uint128 newRate, uint128 paid)`
- `StreamResumed(bytes32 indexed streamId, uint64 lastAccrualMs, uint128 accruedTotal)`
- `StreamSettled(bytes32 indexed streamId, uint128 paid, uint128 settledTotal, bool underfunded)`
- `StreamToppedUp(bytes32 indexed streamId, uint128 amount, bool wasUnderfunded)`
- `TaskCommitted(bytes32 indexed primary, bytes32 indexed secondary, uint8 code, uint64 sequence)`

## layerxprograms (`abi.json`)

Source: [`precompiles/layerxprograms/abi.json`](../../precompiles/layerxprograms/abi.json).

Destination: `0x000000000000000000000000000000000000101D`.

Served at this fixed address after merge activation. `feeAsset` is the active schedule asset identifier; `frozen` is the actual host freeze flag, with `(0, false)` for an absent account. See the result semantics above.

### Functions

- `call(bytes32 programId, uint16 abiVersion, string entrypoint, bytes callData, bytes capabilities, bytes accessDeclaration, uint32 responseCapacity, uint64[7] budget, uint128 feeLimit) returns (int32 terminalResult, uint8 terminalKind, bytes response)` `nonpayable`
- `deploy(bytes32 programId, uint16 abiVersion, uint8 policy, bytes32 authority, bytes32 codeHash, bytes interfaceEncoding, bytes wasm)` `nonpayable`
- `deprecate(bytes32 programId, bytes32 exitProgram, uint64 deadline)` `nonpayable`
- `exitValue(bytes32 programId, bytes32 account)` `nonpayable`
- `exitValueBounded(bytes32 programId, bytes32 account, uint128 maxAmount)` `nonpayable`
- `getAccountProfile(bytes32 programId) returns (uint8 profile, bytes32 principal)` `view`
- `getCurrentFeeSchedule() returns (uint32 version, uint64[7] prices, bytes32 feeAsset)` `view`
- `getCurrentMeteringSchedule() returns (uint32 version, uint64[9] coefficients)` `view`
- `getExitRoute(bytes32 programId, bytes32 accountId) returns (bytes32 asset, bytes32 destination, bytes seed)` `view`
- `getFeeSchedule(uint32 version) returns (uint64[7] prices, bytes32 feeAsset)` `view`
- `getInterface(bytes32 programId) returns (uint32 version, bytes32 digest, bytes encoding)` `view`
- `getMeteringSchedule(uint32 version) returns (uint64[9] coefficients, uint8 authorityKind)` `view`
- `getPendingFeeSchedule() returns (uint64[7] prices, bytes32 asset, uint64[6] demand, uint64 activationBatch)` `view`
- `getProgram(bytes32 programId) returns (uint8 policy, bytes32 authority, bytes32 codeHash, uint16 abiVersion, uint32 version, bool active)` `view`
- `getProgramAccount(bytes32 programId, bytes32 assetId, bytes seed) returns (bytes32 accountId, bytes32 assetId, uint64 registeredSequence)` `view`
- `getProgramAccountById(bytes32 accountId) returns (bytes32 programId, bytes32 assetId, bytes seed)` `view`
- `getProgramOwner(bytes32 programId) returns (bytes32 owner)` `view`
- `getRetirementControl() returns (bytes control)` `view`
- `getSandbox(bytes32 leaseId, uint8 kind) returns (bytes state)` `view`
- `getStateHead(bytes32 programId) returns (bytes stateHead)` `view`
- `getValueAccount(bytes32 programId, bytes32 accountId) returns (uint128 balance, bool frozen)` `view`
- `getWebRequest(bytes32 programId, uint64 requestId) returns (bytes record)` `view`
- `getWindDown(bytes32 programId) returns (uint8 status, bytes32 exitProgram, uint64 deadline, uint64 effectiveSequence, uint16 accountCount, uint16 liveCount)` `view`
- `quoteCallFee(uint64[7] budget) returns (uint32 version, uint128 executionCeiling, bytes32 feeAsset)` `view`
- `registerAccount(bytes32 programId, uint8 profile, bytes32 assetId, bytes seed)` `nonpayable`
- `registryRead(bytes32 programId, bytes body)` `nonpayable`
- `retireStorage(bytes request)` `nonpayable`
- `sandboxActivate(bytes32 leaseId, bytes32 expectedLeaseDigest, uint64 expectedSequence)` `nonpayable`
- `sandboxExecute(bytes32 leaseId, uint64 expectedSequence, bytes32 expectedLeaseDigest, bytes32 escrowAccount, bytes32 asset, bytes32 feeDestination, uint32 feeScheduleVersion, uint64[7] feeSchedule, bytes callPayload, uint128 feeLimit) returns (int32 terminalResult, uint8 terminalKind, bytes response)` `nonpayable`
- `sandboxFund(bytes32 leaseId, bytes32 tenant, bytes32 hostProgram, bytes32 escrowAccount, bytes32 asset, uint128 amount, uint64 expiry, uint32 feeScheduleVersion, uint64[7] feeSchedule, bytes leaseRequest, bytes32 transferProgramId, (bytes32 from, bytes32 asset, bytes32 to, uint128 amount)[] fundingLegs)` `nonpayable`
- `setExitRoute(bytes32 programId, bytes32 account, bytes32 asset, bytes32 destination, bytes seed)` `nonpayable`
- `simulateCall(bytes32 programId, uint16 abiVersion, string entrypoint, bytes callData, bytes capabilities, bytes accessDeclaration, uint32 responseCapacity, uint64[7] budget, uint128 feeLimit) returns (int32 terminalResult, uint8 terminalKind, bytes response)` `view`
- `tombstone(bytes32 programId)` `nonpayable`
- `transfer(bytes32 programId, (bytes32 from, bytes32 asset, bytes32 to, uint128 amount)[] legs)` `nonpayable`
- `upgrade(bytes32 programId, uint16 abiVersion, uint8 flags, bytes32 oldCodeHash, bytes32 newCodeHash, bytes migrationHook, bytes interfaceEncoding, bytes wasm, uint128 feeLimit)` `nonpayable`

### Events

- `ExitRouteSet(bytes32 indexed programId, bytes32 accountId, bytes32 assetId, bytes32 destination)`
- `ProgramAccountRegistered(bytes32 indexed programId, bytes32 accountId, bytes32 assetId, bytes32 seedDigest, uint16 seedLength, uint64 registeredSequence)`
- `ProgramCallOutcome(bytes32 indexed programId, bytes32 principal, bytes32 activityId, uint64 framePath, uint8 depth, uint16 runtimeVersion, uint16 abiVersion, uint32 feeScheduleVersion, uint32 meteringVersion, int32 terminalResult, bytes32 transferSetRoot, bytes32 callGraphDigest, bytes32 terminalDetailDigest, bytes32 eventEnvelopeDigest)`
- `ProgramDeployed(bytes32 indexed programId, bytes32 codeHash)`
- `ProgramDeprecated(bytes32 indexed programId, uint8 priorStatus, uint8 targetStatus, bytes32 principal, bytes32 exitProgram, uint64 deadline, uint64 sequence, uint16 accountCount, uint16 liveCount, bytes32 accountRoot)`
- `ProgramGuestEvent(bytes32 indexed programId, bytes32 principal, bytes32 activityId, uint64 framePath, uint8 depth, uint32 eventIndex, uint16 topicLength, bytes32 topicDigest, uint32 dataLength, bytes32 dataDigest)`
- `ProgramTombstoned(bytes32 indexed programId, uint8 priorStatus, uint8 targetStatus, bytes32 principal, bytes32 exitProgram, uint64 deadline, uint64 sequence, uint16 accountCount, uint16 liveCount, bytes32 accountRoot)`
- `ProgramTransferred(bytes32 indexed programId, bytes32 transferSetRoot)`
- `ProgramUpgraded(bytes32 indexed programId, bytes32 oldCodeHash, bytes32 newCodeHash)`
- `ProgramValueExited(bytes32 indexed programId, bytes32 accountId, bytes32 transferSetRoot)`
- `RegistryRead(bytes32 indexed programId)`
- `SandboxDestroyed(bytes32 indexed leaseId, uint8 part, bytes terminalFrame)`
- `SandboxUsageReceipt(bytes32 indexed activityId, uint64 expectedSequence, uint16 chunkIndex, uint16 chunkCount, uint32 receiptLength, bytes chunk)`
- `StorageRetired(bytes32 indexed activityId, uint8 frameIndex, bytes frame)`

## layerxverify (`abi.json`)

Source: [`precompiles/layerxverify/abi.json`](../../precompiles/layerxverify/abi.json).

Destination: `0x0000000000000000000000000000000000001012`.

Retained historical ABI. Every method is retired from the merge height and reverts with `LXP_MODULE_DISABLED: module disabled; layerxVerify retired`, even if the ABI declares it pure. Before the merge, receipt-inclusion and discovery methods and non-raw Ed25519 domains already refuse as retired.

### Functions

- `verifyDiscoveryProof(bytes payload, bytes proofMaterial, bytes32 programId, uint64 stalenessMs, bytes32 sequencerPublicKey) returns ((bytes32 digest, uint32 version, bytes32 codeHash, uint16 abiVersion, uint64 observedSequence, uint64 observedAt, uint64 validThrough, bytes32 stateRoot, bytes32 headReceiptDigest) head)` `pure`
- `verifyEd25519(bytes32 publicKey, uint8 domain, bytes message, bytes signature) returns (bool valid)` `pure`
- `verifyReceipt(bytes receipt, bytes32 sequencerPublicKey) returns ((bytes32 receiptDigest, bytes32 activityId, uint64 globalSequence, int32 resultCode, uint16 moduleId, uint8 operation, bytes32 asset, uint128 amount, bytes32 from, bytes32 to, bytes32 previousStateRoot, bytes32 resultingStateRoot, uint64 timestamp) facts)` `pure`
- `verifyReceiptInclusion(bytes receipt, bytes proof, bytes batchHeader, bytes headerSignature, bytes32 sequencerId, bytes32 sequencerPublicKey, uint64 firstBatchNumber, uint64 lastBatchNumber) returns ((bytes32 receiptDigest, bytes32 activityId, uint64 globalSequence, int32 resultCode, uint16 moduleId, uint8 operation, bytes32 asset, uint128 amount, bytes32 from, bytes32 to, bytes32 previousStateRoot, bytes32 resultingStateRoot, uint64 timestamp) facts, (bytes32 headerDigest, uint32 networkId, uint64 epoch, uint64 batchNumber, uint64 firstSequence, uint64 lastSequence, bytes32 previousStateRoot, bytes32 resultingStateRoot, bytes32 receiptRoot, bytes32 sequencerId, uint64 timestampMs) batch)` `pure`
- `verifyStateProof(bytes witness, bytes32 stateRoot) returns (uint16 moduleId, bytes key, bytes value)` `pure`

## oracle (`abi.json`)

Source: [`precompiles/oracle/abi.json`](../../precompiles/oracle/abi.json).

Destination: `0x0000000000000000000000000000000000001008`.

Retained ABI only. Both methods always refuse with `oracle precompile is retired; oracle data queries are disabled`.

### Functions

- `getExchangeRates() returns ((string denom, (string exchangeRate, string lastUpdate, int64 lastUpdateTimestamp) oracleExchangeRateVal)[])` `view`
- `getOracleTwaps(uint64 lookback_seconds) returns ((string denom, string twap, int64 lookbackSeconds)[])` `view`

## p256 (`abi.json`)

Source: [`precompiles/p256/abi.json`](../../precompiles/p256/abi.json).

Destination: `0x0000000000000000000000000000000000001011`.

### Functions

- `verify(bytes signature) returns (bytes response)` `view`

## perps (`abi.json`)

Source: [`precompiles/perps/abi/abi.json`](../../precompiles/perps/abi/abi.json).

Destination: `0x0000000000000000000000000000000000001015`.

Served at `0x1015` from `v6.12.0`, replacing the legacy exchange ABI. Orders and positions are read directly from current module state.

### Functions

- `addCollateral(uint256 positionId, uint256 amount)` `nonpayable`
- `addSize(uint256 positionId, uint256 additionalCollateral, uint256 leverage)` `nonpayable`
- `cancelByCloid(uint256 marketId, bytes16 cloid)` `nonpayable`
- `cancelOrder(uint256 orderId)` `nonpayable`
- `checkLiquidatable(uint256 positionId) returns (bool liquidatable, uint256 marginBps)` `view`
- `closePosition(uint256 positionId)` `nonpayable`
- `closePositionWithParams(uint256 positionId, uint256 closeSizeUsd, uint256 acceptablePrice, uint64 deadline)` `nonpayable`
- `createVault() returns (address vault)` `nonpayable`
- `deposit(address token, uint256 amount)` `nonpayable`
- `emergencyWithdraw(address token)` `nonpayable`
- `executeOrder(uint256 orderId) returns (uint256 positionId)` `nonpayable`
- `getADLRank(uint256 positionId) returns (uint256 rankScore, bool profitable)` `view`
- `getAccount(address account) returns ((uint256 collateral, int256 equityUsd, uint256 initialMarginUsd, uint256 maintenanceMarginUsd, uint256 withdrawable, uint8 feeTier, uint256 volume14dUsd, uint256 positionCount, uint256 openOrderCount) state)` `view`
- `getAccountSummary(address account, address token) returns ((uint256 idle, uint256 totalLocked, uint256 positionCount) summary)` `view`
- `getAccountSummary(address token) returns ((uint256 idle, uint256 totalLocked, uint256 positionCount) summary)` `view`
- `getActiveMarketIds() returns (uint256[] marketIds)` `view`
- `getBalance(address account, address token) returns (uint256 balance)` `view`
- `getBalance(address token) returns (uint256 balance)` `view`
- `getCurrentFundingRate(uint256 marketId) returns (int256 ratePerSecond)` `view`
- `getFeeTier(address account) returns (uint8 tier, uint256 takerFeePpm, int256 makerFeePpm, uint256 volume14dUsd)` `view`
- `getFees() returns (uint256 takerFeePpm, uint256 makerFeePpm, uint256 liquidationFeePpm, uint256 insuranceSharePpm)` `view`
- `getFundingDetails(uint256 marketId) returns (int256 cumulativeFundingPerUnit, uint256 lastSettleTime, int256 lastHourlyRate, int256 premiumSampleSum, uint256 premiumSampleCount)` `view`
- `getFundingState(uint256 marketId) returns (int256 cumulativeLong, int256 cumulativeShort, uint256 lastUpdate, int256 ratePerSecond)` `view`
- `getInsuranceBalance(address token) returns (uint256 balance)` `view`
- `getLiquidationPrice(uint256 positionId) returns (uint256 liquidationPrice)` `view`
- `getLockedBalance(address account, address token) returns (uint256 locked)` `view`
- `getLockedBalance(address token) returns (uint256 locked)` `view`
- `getMarginMode() returns (uint8 mode)` `view`
- `getMarginMode(address account) returns (uint8 mode)` `view`
- `getMarginTiers(uint256 marketId) returns ((uint256 lowerBoundUsd, uint256 maxLeverage, uint256 maintenanceBps, uint256 deductionUsd)[] tiers)` `view`
- `getMarkPrice(uint256 marketId) returns (uint256 markPrice)` `view`
- `getMarkPriceDetails(uint256 marketId) returns (uint256 markPrice, uint256 indexPrice, uint256 midPrice, bool fresh)` `view`
- `getMarket(uint256 marketId) returns (string name, string symbol, uint256 maxLeverage, uint256 maintenanceMarginBps, uint256 maxOpenInterest, bool enabled)` `view`
- `getMarketConfig(uint256 marketId) returns ((uint256 marketId, string symbol, string name, string oracleSymbol, uint8 szDecimals, uint8 pxDecimals, uint256 tickSize, uint256 lotSize, uint256 maxLeverage, uint8 tierClass, uint256 impactNotionalUsd, uint256 oiCapUsd, uint256 fundingCapPpmPerHour, bool enabled, uint64 listedAt) config)` `view`
- `getOpenInterest(uint256 marketId) returns (uint256 longOI, uint256 shortOI)` `view`
- `getOrder(uint256 orderId) returns (address user, uint256 marketId, bool isLong, uint8 orderType, uint256 triggerPrice, uint256 limitPrice, uint256 sizeUsd, uint256 leverage, address collateralToken, uint256 collateralAmount, bool active)` `view`
- `getOrderDetails(uint256 orderId) returns ((uint256 orderId, address user, uint256 marketId, bool isLong, uint8 orderType, uint8 timeInForce, uint8 status, bool reduceOnly, uint256 limitPrice, uint256 triggerPrice, uint256 acceptablePrice, uint256 sizeUsd, uint256 sizeBase, uint256 filledBase, uint256 leverage, uint64 deadline, bytes16 cloid, uint64 sequence) details)` `view`
- `getPosition(uint256 positionId) returns (address user, uint256 marketId, bool isLong, uint256 sizeUsd, uint256 collateralUsd, address collateralToken, uint256 collateralAmount, uint256 entryPrice, uint256 timestamp, bool active)` `view`
- `getPositionDetails(uint256 positionId) returns ((uint256 positionId, address user, uint256 marketId, bool isLong, uint256 sizeBase, uint256 sizeUsdAtMark, uint256 entryPrice, uint256 leverage, int256 unrealizedPnlUsd, int256 fundingOwedUsd, uint256 liquidationPrice, uint256 takeProfitPrice, uint256 stopLossPrice, uint256 openedAt) details)` `view`
- `getPositionFunding(uint256 positionId) returns (int256 fundingOwedUsd)` `view`
- `getPositionMargin(uint256 positionId) returns ((address token, uint256 lockedAmount, uint256 marketId, bool isLong) margin)` `view`
- `getProtocolAccounts() returns (address feeRecipient, address insuranceFund)` `view`
- `getUserMarketPosition(address user, uint256 marketId) returns (uint256 positionId)` `view`
- `getUserOrderIds(address user) returns (uint256[] orderIds)` `view`
- `getUserPositionIds(address user) returns (uint256[] positionIds)` `view`
- `getVault(address user) returns (address vault)` `view`
- `isMarketActive(uint256 marketId) returns (bool active)` `view`
- `liquidate(uint256 positionId)` `nonpayable`
- `openPosition(uint256 marketId, address collateralToken, uint256 collateralAmount, uint256 leverage, bool isLong) returns (uint256 positionId)` `nonpayable`
- `partialClose(uint256 positionId, uint256 closeSizeUsd)` `nonpayable`
- `placeLimitOrder(uint256 marketId, bool isLong, uint256 triggerPrice, uint256 sizeUsd, uint256 leverage, address collateralToken, uint256 collateralAmount) returns (uint256 orderId)` `nonpayable`
- `placeOrder((uint256 marketId, bool isLong, uint8 orderType, uint8 timeInForce, uint256 sizeUsd, uint256 leverage, uint256 limitPrice, uint256 triggerPrice, uint256 acceptablePrice, uint256 collateralAmount, bool reduceOnly, uint256 takeProfitPrice, uint256 stopLossPrice, uint64 deadline, bytes16 cloid) p) returns (uint256 orderId)` `nonpayable`
- `placeStopLimitOrder(uint256 marketId, bool isLong, uint256 triggerPrice, uint256 limitPrice, uint256 sizeUsd, uint256 leverage, address collateralToken, uint256 collateralAmount) returns (uint256 orderId)` `nonpayable`
- `placeStopLossOrder(uint256 positionId, uint256 triggerPrice) returns (uint256 orderId)` `nonpayable`
- `placeTakeProfitOrder(uint256 positionId, uint256 triggerPrice) returns (uint256 orderId)` `nonpayable`
- `quoteClosePosition(uint256 positionId) returns ((uint256 exitPrice, int256 unrealizedPnl, uint256 tradingFee, uint256 tradingFeeUsd, int256 fundingOwed, int256 netPnl, uint256 estimatedPayout) quote)` `view`
- `quoteMarket(uint256 marketId) returns ((uint256 indexPrice, uint256 markPrice, uint256 oracleTWAP, int256 fundingRatePerSecond, int256 fundingRate24h, uint256 longOI, uint256 shortOI, uint256 maxLeverage, uint256 maintenanceMarginBps, bool enabled, bool priceStale) quote)` `view`
- `quoteOpenPosition(uint256 marketId, address collateralToken, uint256 collateralAmount, uint256 leverage, bool isLong) returns ((uint256 entryPrice, uint256 sizeUsd, uint256 collateralUsd, uint256 leverage, uint256 tradingFee, uint256 tradingFeeUsd, uint256 priceImpact, uint256 liquidationPrice, int256 estimatedFunding24h, uint256 maintenanceMarginBps) quote)` `view`
- `quotePartialClose(uint256 positionId, uint256 closeSizeUsd) returns (uint256 exitPrice, int256 closedPnl, uint256 fee, uint256 estimatedPayout)` `view`
- `removeCollateral(uint256 positionId, uint256 amount)` `nonpayable`
- `setMarginMode(uint8 mode)` `nonpayable`
- `setPositionTpSl(uint256 positionId, uint256 takeProfitPrice, uint256 stopLossPrice)` `nonpayable`
- `topUpPosition(uint256 positionId, uint256 amount)` `nonpayable`
- `totalMarkets() returns (uint256 count)` `view`
- `transferCollateral(address to, uint256 amount)` `nonpayable`
- `transferMargin(uint256 fromPositionId, uint256 toPositionId, uint256 amount)` `nonpayable`
- `withdraw(address token, uint256 amount)` `nonpayable`

### Events

- `ADLExecuted(uint256 indexed positionId, uint256 deleveragedSizeUsd)`
- `ADLMatched(uint256 indexed marketId, uint256 indexed positionId, address indexed account, uint256 sizeUsd, uint256 price, uint256 rankScore)`
- `BackstopLiquidation(address indexed account, uint256 indexed marketId, uint256 sizeUsd, uint256 markPrice, uint256 marginTakenUsd)`
- `CollateralDeposited(address indexed account, uint256 amount, uint256 newBalance)`
- `CollateralTransferred(address indexed from, address indexed to, uint256 amount)`
- `CollateralWithdrawn(address indexed account, uint256 amount, uint256 newBalance)`
- `FeeCollected(uint256 indexed positionId, address indexed user, uint256 indexed marketId, uint8 feeType, uint256 feeUsd, uint256 feeTokens, address token)`
- `Fill(uint256 indexed marketId, uint256 indexed makerOrderId, uint256 indexed takerOrderId, address maker, address taker, bool takerIsLong, uint256 price, uint256 sizeUsd, int256 makerFeeUsd, uint256 takerFeeUsd)`
- `FundingRateUpdated(uint256 indexed marketId, int256 newRatePerSecond, int256 fundingRate24h)`
- `FundingSettled(uint256 indexed marketId, int256 fundingRate, int256 longPayment, int256 shortPayment)`
- `InsuranceContribution(address indexed token, uint256 amount, uint8 source)`
- `InsuranceDraw(uint256 indexed marketId, uint256 amount, uint256 newBalance)`
- `Liquidation(uint256 indexed positionId, address indexed user, uint256 indexed marketId, uint256 liquidationPrice, uint256 penalty, address keeper)`
- `MarkPriceUpdated(uint256 indexed marketId, uint256 markPrice, uint256 indexPrice, uint256 midPrice)`
- `OpenInterestChanged(uint256 indexed marketId, uint256 longOI, uint256 shortOI, uint256 deltaUsd, bool isIncrease)`
- `OrderCancelled(uint256 indexed orderId, address indexed user)`
- `OrderExecuted(uint256 indexed orderId, uint256 indexed positionId, uint256 executionPrice)`
- `OrderExpired(uint256 indexed orderId, address indexed user, uint256 indexed marketId)`
- `OrderOptions(uint256 indexed orderId, uint8 timeInForce, bool reduceOnly, uint256 limitPrice, uint256 acceptablePrice, uint64 deadline, bytes16 cloid)`
- `OrderPlaced(uint256 indexed orderId, address indexed user, uint256 indexed marketId, uint8 orderType, bool isLong, uint256 triggerPrice, uint256 sizeUsd)`
- `OrderTriggered(uint256 indexed orderId, uint256 indexed marketId, uint256 markPrice)`
- `PositionClosed(uint256 indexed positionId, address indexed user, uint256 indexed marketId, uint256 closedSizeUsd, uint256 exitPrice, int256 realizedPnl, bool isFullClose)`
- `PositionEntryPriceChanged(uint256 indexed positionId, uint256 oldEntryPrice, uint256 newEntryPrice)`
- `PositionFundingApplied(uint256 indexed positionId, address indexed user, uint256 indexed marketId, int256 fundingPaymentUsd, uint256 newCollateralUsd, uint256 newCollateralAmount)`
- `PositionModified(uint256 indexed positionId, uint256 newSizeUsd, uint256 newCollateralUsd, uint256 newCollateralAmount)`
- `PositionOpened(uint256 indexed positionId, address indexed user, uint256 indexed marketId, bool isLong, uint256 sizeUsd, uint256 leverage, uint256 entryPrice, address collateralToken, uint256 collateralAmount)`
- `PositionTpSlSet(uint256 indexed positionId, uint256 takeProfitPrice, uint256 stopLossPrice)`
- `TradeSettled(uint256 indexed positionId, address indexed user, uint256 indexed marketId, uint8 tradeType, uint256 sizeUsd, uint256 executionPrice, int256 grossPnl, uint256 totalFeesUsd, int256 fundingPaidUsd, uint256 netPayoutTokens)`
- `VaultCreated(address indexed user, address indexed vault)`
- `VaultDeficit(address indexed token, uint256 deficit)`

### Errors

- `PerpsCloseSizeExceedsPosition()`
- `PerpsCrossMarginOnly()`
- `PerpsDeadlineExpired()`
- `PerpsDelegatecall()`
- `PerpsDuplicateCloid()`
- `PerpsFokNotFilled()`
- `PerpsInsufficientCollateral()`
- `PerpsInsufficientMargin()`
- `PerpsInvalidGenesis()`
- `PerpsInvalidOrderType()`
- `PerpsInvalidParams()`
- `PerpsInvalidTimeInForce()`
- `PerpsInvalidTriggerPrice()`
- `PerpsLeverageTooHigh()`
- `PerpsLiquidationCooldown()`
- `PerpsLotSize()`
- `PerpsMarketDisabled()`
- `PerpsMaxOrderValue()`
- `PerpsMinOrderValue()`
- `PerpsNoAccount()`
- `PerpsNoLiquidity()`
- `PerpsNotLiquidatable()`
- `PerpsNotOrderHolder()`
- `PerpsNotPositionHolder()`
- `PerpsOpenInterestCap()`
- `PerpsOverflow()`
- `PerpsPaused()`
- `PerpsPostOnlyWouldCross()`
- `PerpsPriceStale()`
- `PerpsReduceOnlyViolation()`
- `PerpsSelfTransfer()`
- `PerpsSlippageExceeded()`
- `PerpsStaticCall()`
- `PerpsTickSize()`
- `PerpsTokenTransferFailed()`
- `PerpsTooManyOpenOrders()`
- `PerpsTriggerNotReached()`
- `PerpsUnauthorized()`
- `PerpsUnknownMarket()`
- `PerpsUnknownOrder()`
- `PerpsUnknownPosition()`
- `PerpsUnsupportedCollateral()`
- `PerpsZeroAddress()`
- `PerpsZeroAmount()`

## pointer (`abi.json`)

Source: [`precompiles/pointer/abi.json`](../../precompiles/pointer/abi.json).

Destination: `0x000000000000000000000000000000000000100b`.

### Functions

- `addCW1155Pointer(string cwAddr) returns (address ret)` `payable`
- `addCW20Pointer(string cwAddr) returns (address ret)` `payable`
- `addCW721Pointer(string cwAddr) returns (address ret)` `payable`
- `addNativePointer(string token) returns (address ret)` `payable`

## pointerview (`abi.json`)

Source: [`precompiles/pointerview/abi.json`](../../precompiles/pointerview/abi.json).

Destination: `0x000000000000000000000000000000000000100A`.

### Functions

- `getCW1155Pointer(string cwAddr) returns (address addr, uint16 version, bool exists)` `view`
- `getCW20Pointer(string cwAddr) returns (address addr, uint16 version, bool exists)` `view`
- `getCW721Pointer(string cwAddr) returns (address addr, uint16 version, bool exists)` `view`
- `getNativePointer(string token) returns (address addr, uint16 version, bool exists)` `view`

## pricefeed (`abi.json`)

Source: [`precompiles/pricefeed/abi/abi.json`](../../precompiles/pricefeed/abi/abi.json).

Destination: `0x0000000000000000000000000000000000001020`.

Served from `v6.12.0`; this is distinct from the retired oracle-data ABI at `0x1008` and the agent oracle at `0x0903`.

### Functions

- `getFeedKey(address validator) returns (bytes pubKey, address feedAccount)` `view`
- `getMarket(uint256 marketId) returns ((uint64 marketId, string symbol, string chainflowSymbol, uint256 minPrice, uint256 maxPrice, bool enabled))` `view`
- `getMarketCount() returns (uint256)` `view`
- `getObservation(uint256 marketId, address validator) returns ((uint64 sequence, uint256 indexPrice, uint256 markPrice, uint64 sourceTimestampMs, int64 height))` `view`
- `getParams() returns ((uint64 stalenessWindowMs, uint64 maxClockSkewMs, uint32 minSources, uint32 maxDivergenceBps, uint32 maxDeviationBps, uint32 maxObservationsPerSubmission, uint32 maxSubmissionBytes, uint64 markEmaTauMs, uint32 maxMarkBasisBps, uint64 maxPriceAgeMs))` `view`
- `getPrice(uint256 marketId) returns ((uint256 indexPrice, uint256 markPrice, uint64 timestampMs, int64 height, uint32 sources, bool fresh))` `view`
- `getPriceBySymbol(string symbol) returns ((uint256 indexPrice, uint256 markPrice, uint64 timestampMs, int64 height, uint32 sources, bool fresh))` `view`
- `getPrices(uint256[] marketIds) returns ((uint256 indexPrice, uint256 markPrice, uint64 timestampMs, int64 height, uint32 sources, bool fresh)[])` `view`
- `getValidatorByFeedAccount(address feedAccount) returns (address validator)` `view`
- `marketIdOf(string symbol) returns (uint256)` `view`
- `registerFeedKey(bytes pubKey) returns (address feedAccount)` `nonpayable`
- `revokeFeedKey()` `nonpayable`

### Events

- `FeedKeyRegistered(address indexed validator, address indexed feedAccount, bytes pubKey, address previousFeedAccount)`
- `FeedKeyRevoked(address indexed validator, address indexed feedAccount)`

## solo (`abi.json`)

Source: [`precompiles/solo/abi.json`](../../precompiles/solo/abi.json).

Destination: `0x000000000000000000000000000000000000100C`.

### Functions

- `claim(bytes payload) returns (bool response)` `nonpayable`
- `claimSpecific(bytes payload) returns (bool response)` `nonpayable`

## spot (`orders.json`)

Source: [`precompiles/spot/abi/orders.json`](../../precompiles/spot/abi/orders.json).

Destination: `0x0000000000000000000000000000000000001022`.

Served only at this rail address. Select this ABI for calls and logs at this address.

### Functions

- `BPS_DENOMINATOR() returns (uint256)` `view`
- `MAX_ORDER_FEE_BPS() returns (uint256)` `view`
- `accumulatedFees(address) returns (uint256)` `view`
- `activeLimitOrderIds(uint256) returns (uint256)` `view`
- `batchExecuteLimitOrders(uint256[] orderIds) returns (uint256 executedCount)` `nonpayable`
- `batchExpireOrders(uint256[] orderIds) returns (uint256 expiredCount)` `nonpayable`
- `canExecuteLimitOrder(uint256 orderId) returns (bool, string)` `view`
- `cancelLimitOrder(uint256 orderId)` `nonpayable`
- `collectFees(address token)` `nonpayable`
- `executeLimitOrder(uint256 orderId)` `nonpayable`
- `feeCollector() returns (address)` `view`
- `fillQualityTwapPeriod() returns (uint256)` `view`
- `getAccumulatedFees(address token) returns (uint256)` `view`
- `getActiveLimitOrderCount() returns (uint256)` `view`
- `getExecutableLimitOrders(uint256 maxCount) returns (uint256[] orderIds)` `view`
- `getLimitOrder(uint256 orderId) returns ((uint256 id, address user, address stablecoin, address token, uint256 amount, uint256 targetPrice, uint8 orderType, uint8 status, uint256 createdAt, uint256 expiresAt))` `view`
- `getUserLimitOrders(address user) returns (uint256[])` `view`
- `keepers(address) returns (bool)` `view`
- `limitOrders(uint256) returns (uint256 id, address user, address stablecoin, address token, uint256 amount, uint256 targetPrice, uint8 orderType, uint8 status, uint256 createdAt, uint256 expiresAt)` `view`
- `modifyLimitOrder(uint256 orderId, uint256 newTargetPrice, uint256 newExpiresAt)` `nonpayable`
- `nextOrderId() returns (uint256)` `view`
- `orderFeeBps() returns (uint256)` `view`
- `owner() returns (address)` `view`
- `pause()` `nonpayable`
- `paused() returns (bool)` `view`
- `placeLimitBuy(address stablecoin, address token, uint256 stablecoinAmount, uint256 targetPrice, uint256 expiresAt) returns (uint256 orderId)` `nonpayable`
- `placeLimitSell(address token, address stablecoin, uint256 tokenAmount, uint256 targetPrice, uint256 expiresAt) returns (uint256 orderId)` `nonpayable`
- `priceOracle() returns (address)` `view`
- `renounceOwnership()` `nonpayable`
- `setFeeCollector(address _collector)` `nonpayable`
- `setFillQualityTwapPeriod(uint256 period)` `nonpayable`
- `setKeeper(address keeper, bool authorized)` `nonpayable`
- `setOrderFee(uint256 _feeBps)` `nonpayable`
- `setPriceOracle(address _oracle)` `nonpayable`
- `setTransactionTracker(address _t)` `nonpayable`
- `transactionTracker() returns (address)` `view`
- `transferOwnership(address newOwner)` `nonpayable`
- `unpause()` `nonpayable`
- `userLimitOrders(address, uint256) returns (uint256)` `view`
- `vault() returns (address)` `view`

### Events

- `LimitOrderCancelled(uint256 indexed orderId)`
- `LimitOrderCreated(uint256 indexed orderId, address indexed user, uint8 orderType, uint256 amount, uint256 targetPrice)`
- `LimitOrderExecuted(uint256 indexed orderId, uint256 executionPrice)`
- `LimitOrderExpired(uint256 indexed orderId)`
- `LimitOrderModified(uint256 indexed orderId, uint256 targetPrice, uint256 expiresAt)`
- `OwnershipTransferred(address indexed previousOwner, address indexed newOwner)`
- `Paused(address account)`
- `Unpaused(address account)`

### Errors

- `CannotCancel()`
- `EnforcedPause()`
- `ExpectedPause()`
- `FeeExceedsMax()`
- `InsufficientLiquidity()`
- `InsufficientOutput()`
- `InvalidExpiry()`
- `InvalidOracle()`
- `InvalidPrice()`
- `InvalidTWAPPeriod()`
- `InvalidVault()`
- `NoFeesToCollect()`
- `NotAStablecoin()`
- `NotAuthorized()`
- `NotFeeCollector()`
- `NotOrderOwner()`
- `OrderBelowMinimumNotional()`
- `OrderExpired()`
- `OrderNotPending()`
- `OwnableInvalidOwner(address owner)`
- `OwnableUnauthorizedAccount(address account)`
- `PriceAboveTarget()`
- `PriceBelowTarget()`
- `ReentrancyGuardReentrantCall()`
- `SafeERC20FailedOperation(address token)`
- `SameToken()`
- `StalePrice()`
- `TokenIsStablecoin()`
- `TokenPaused()`
- `TooManyActiveOrders()`
- `UnsupportedInputTransfer()`
- `UnsupportedTransfer()`
- `ZeroAmount()`

## spot (`pecor.json`)

Source: [`precompiles/spot/abi/pecor.json`](../../precompiles/spot/abi/pecor.json).

Destination: `0x0000000000000000000000000000000000001021`.

Served only at this rail address. Select this ABI for calls and logs at this address.

### Functions

- `BPS_DENOMINATOR() returns (uint256)` `view`
- `MAX_FEE_BPS() returns (uint256)` `view`
- `MAX_IMPACT_BPS() returns (uint256)` `view`
- `PRECISION() returns (uint256)` `view`
- `TIER1_THRESHOLD() returns (uint256)` `view`
- `TIER2_THRESHOLD() returns (uint256)` `view`
- `WETH() returns (address)` `view`
- `accumulatedFees(address) returns (uint256)` `view`
- `collectFees(address token)` `nonpayable`
- `feeCollector() returns (address)` `view`
- `getAccumulatedFees(address token) returns (uint256)` `view`
- `getDetailedQuote(address tokenIn, address tokenOut, uint256 amountIn) returns (uint256 grossOut, uint256 netOut, uint256 priceImpactBps, uint256 feeBps, uint256 feeAmount)` `view`
- `getQuoteExactIn(address tokenIn, address tokenOut, uint256 amountIn) returns (uint256 amountOut)` `view`
- `getQuoteExactOut(address tokenIn, address tokenOut, uint256 amountOut) returns (uint256 amountIn)` `view`
- `impactScalarBps() returns (uint256)` `view`
- `marketBuy(address stablecoin, address token, uint256 stablecoinAmount, uint256 minTokenAmount, uint256 deadline) returns (uint256 tokenAmount)` `nonpayable`
- `marketSell(address token, address stablecoin, uint256 tokenAmount, uint256 minStablecoinAmount, uint256 deadline) returns (uint256 stablecoinAmount)` `nonpayable`
- `maxTwapDeviationBps() returns (uint256)` `view`
- `multicall(bytes[] data) returns (bytes[] results)` `nonpayable`
- `owner() returns (address)` `view`
- `pause()` `nonpayable`
- `paused() returns (bool)` `view`
- `priceImpactEnabled() returns (bool)` `view`
- `priceOracle() returns (address)` `view`
- `renounceOwnership()` `nonpayable`
- `setFeeCollector(address _collector)` `nonpayable`
- `setPriceImpact(bool _enabled, uint256 _scalarBps)` `nonpayable`
- `setPriceOracle(address _oracle)` `nonpayable`
- `setSwapFee(uint256 _feeBps)` `nonpayable`
- `setTieredFees(uint256 _tier1FeeBps, uint256 _tier2FeeBps)` `nonpayable`
- `setTransactionTracker(address _tracker)` `nonpayable`
- `setTwapSanity(bool enabled, uint256 period, uint256 maxDeviationBps_)` `nonpayable`
- `swapExactIn(address tokenIn, address tokenOut, uint256 amountIn, uint256 amountOutMin, uint256 deadline) returns (uint256 amountOut)` `nonpayable`
- `swapExactInNative(address tokenOut, uint256 amountOutMin, uint256 deadline) returns (uint256 amountOut)` `payable`
- `swapExactInToNative(address tokenIn, uint256 amountIn, uint256 amountOutMin, uint256 deadline) returns (uint256 amountOut)` `nonpayable`
- `swapExactOut(address tokenIn, address tokenOut, uint256 amountOut, uint256 amountInMax, uint256 deadline) returns (uint256 amountIn)` `nonpayable`
- `swapFeeBps() returns (uint256)` `view`
- `tier1FeeBps() returns (uint256)` `view`
- `tier2FeeBps() returns (uint256)` `view`
- `transactionTracker() returns (address)` `view`
- `transferOwnership(address newOwner)` `nonpayable`
- `twapSanityEnabled() returns (bool)` `view`
- `twapSanityPeriod() returns (uint256)` `view`
- `unpause()` `nonpayable`
- `vault() returns (address)` `view`

### Events

- `MarketOrderExecuted(address indexed user, address indexed stablecoin, address indexed token, uint256 stablecoinAmount, uint256 tokenAmount, bool isBuy)`
- `NativeSwap(address indexed user, address indexed tokenOther, uint256 nativeAmount, uint256 tokenAmount, bool nativeIsInput)`
- `OwnershipTransferred(address indexed previousOwner, address indexed newOwner)`
- `Paused(address account)`
- `PriceImpactApplied(address indexed user, address indexed tokenOut, uint256 impactBps, uint256 deductedAmount)`
- `SimpleSwap(address indexed user, address indexed tokenIn, address indexed tokenOut, uint256 amountIn, uint256 amountOut, uint256 executionPrice)`
- `TieredFeeApplied(address indexed user, address indexed tokenOut, uint256 volumeUSD, uint256 feeBps, uint256 feeAmount)`
- `TwapSanityUpdated(bool enabled, uint256 period, uint256 maxDeviationBps)`
- `Unpaused(address account)`

### Errors

- `EnforcedPause()`
- `ExcessiveInput()`
- `ExpectedPause()`
- `Expired()`
- `InsufficientLiquidity()`
- `InsufficientOutput()`
- `InvalidVault()`
- `MulticallFailed(uint256 index)`
- `NativeTransferFailed()`
- `NoFeesToCollect()`
- `NotAStablecoin()`
- `OwnableInvalidOwner(address owner)`
- `OwnableUnauthorizedAccount(address account)`
- `ReentrancyGuardReentrantCall()`
- `SafeERC20FailedOperation(address token)`
- `SameToken()`
- `StalePrice()`
- `TokenIsStablecoin()`
- `TokenPaused()`
- `UnsupportedInputTransfer()`
- `UnsupportedTransfer()`
- `UseWethDeposit()`
- `UseWethWithdraw()`
- `ZeroAmount()`

## spot (`quoter.json`)

Source: [`precompiles/spot/abi/quoter.json`](../../precompiles/spot/abi/quoter.json).

Destination: `0x0000000000000000000000000000000000001024`.

Served only at this rail address. Select this ABI for calls and logs at this address.

### Functions

- `WETH() returns (address)` `view`
- `batchQuote((address tokenIn, address tokenOut, uint256 amount, bool isExactIn)[] requests) returns ((uint256 amountIn, uint256 amountOut, uint256 grossAmountOut, uint256 executionPrice, uint256 spotPriceIn, uint256 spotPriceOut, uint256 feeAmount, uint256 feeBps, bool sufficientLiquidity, uint256 availableLiquidity, uint256 priceTimestampIn, uint256 priceTimestampOut, bool priceStaleIn, bool priceStaleOut, uint256 utilizationCapBps, uint256 tradeUtilizationBps, bool cappedByUtilization, uint256 twapOut, uint256 twapDeviationBps, uint256 effectiveFeeBps, uint256 tier, uint256 priceImpactBps, uint256 priceImpactDeduction)[] results)` `view`
- `getAllLiquidityInfo() returns (address[] tokens, uint256[] reserves, uint256[] prices, bool[] stale)` `view`
- `getLiquidityInfo(address token) returns (uint256 available, uint256 tokenPrice, bool isStale)` `view`
- `getTWAP(address token, uint256 period) returns (uint256)` `view`
- `getTokenPrice(address token) returns (uint256 price, uint256 timestamp, bool isStale)` `view`
- `getTokenPrices(address[] tokens) returns (uint256[] prices, uint256[] timestamps, bool[] stale)` `view`
- `pecor() returns (address)` `view`
- `priceOracle() returns (address)` `view`
- `quoteExactIn(address tokenIn, address tokenOut, uint256 amountIn) returns ((uint256 amountIn, uint256 amountOut, uint256 grossAmountOut, uint256 executionPrice, uint256 spotPriceIn, uint256 spotPriceOut, uint256 feeAmount, uint256 feeBps, bool sufficientLiquidity, uint256 availableLiquidity, uint256 priceTimestampIn, uint256 priceTimestampOut, bool priceStaleIn, bool priceStaleOut, uint256 utilizationCapBps, uint256 tradeUtilizationBps, bool cappedByUtilization, uint256 twapOut, uint256 twapDeviationBps, uint256 effectiveFeeBps, uint256 tier, uint256 priceImpactBps, uint256 priceImpactDeduction) result)` `view`
- `quoteExactInNative(address tokenOut, uint256 nativeAmountIn) returns ((uint256 amountIn, uint256 amountOut, uint256 grossAmountOut, uint256 executionPrice, uint256 spotPriceIn, uint256 spotPriceOut, uint256 feeAmount, uint256 feeBps, bool sufficientLiquidity, uint256 availableLiquidity, uint256 priceTimestampIn, uint256 priceTimestampOut, bool priceStaleIn, bool priceStaleOut, uint256 utilizationCapBps, uint256 tradeUtilizationBps, bool cappedByUtilization, uint256 twapOut, uint256 twapDeviationBps, uint256 effectiveFeeBps, uint256 tier, uint256 priceImpactBps, uint256 priceImpactDeduction) result)` `view`
- `quoteExactInToNative(address tokenIn, uint256 amountIn) returns ((uint256 amountIn, uint256 amountOut, uint256 grossAmountOut, uint256 executionPrice, uint256 spotPriceIn, uint256 spotPriceOut, uint256 feeAmount, uint256 feeBps, bool sufficientLiquidity, uint256 availableLiquidity, uint256 priceTimestampIn, uint256 priceTimestampOut, bool priceStaleIn, bool priceStaleOut, uint256 utilizationCapBps, uint256 tradeUtilizationBps, bool cappedByUtilization, uint256 twapOut, uint256 twapDeviationBps, uint256 effectiveFeeBps, uint256 tier, uint256 priceImpactBps, uint256 priceImpactDeduction) result)` `view`
- `quoteExactOut(address tokenIn, address tokenOut, uint256 amountOut) returns ((uint256 amountIn, uint256 amountOut, uint256 grossAmountOut, uint256 executionPrice, uint256 spotPriceIn, uint256 spotPriceOut, uint256 feeAmount, uint256 feeBps, bool sufficientLiquidity, uint256 availableLiquidity, uint256 priceTimestampIn, uint256 priceTimestampOut, bool priceStaleIn, bool priceStaleOut, uint256 utilizationCapBps, uint256 tradeUtilizationBps, bool cappedByUtilization, uint256 twapOut, uint256 twapDeviationBps, uint256 effectiveFeeBps, uint256 tier, uint256 priceImpactBps, uint256 priceImpactDeduction) result)` `view`
- `quoteMarketBuy(address stablecoin, address token, uint256 stablecoinAmount) returns ((uint256 amountIn, uint256 amountOut, uint256 grossAmountOut, uint256 executionPrice, uint256 spotPriceIn, uint256 spotPriceOut, uint256 feeAmount, uint256 feeBps, bool sufficientLiquidity, uint256 availableLiquidity, uint256 priceTimestampIn, uint256 priceTimestampOut, bool priceStaleIn, bool priceStaleOut, uint256 utilizationCapBps, uint256 tradeUtilizationBps, bool cappedByUtilization, uint256 twapOut, uint256 twapDeviationBps, uint256 effectiveFeeBps, uint256 tier, uint256 priceImpactBps, uint256 priceImpactDeduction) result)` `view`
- `quoteMarketSell(address token, address stablecoin, uint256 tokenAmount) returns ((uint256 amountIn, uint256 amountOut, uint256 grossAmountOut, uint256 executionPrice, uint256 spotPriceIn, uint256 spotPriceOut, uint256 feeAmount, uint256 feeBps, bool sufficientLiquidity, uint256 availableLiquidity, uint256 priceTimestampIn, uint256 priceTimestampOut, bool priceStaleIn, bool priceStaleOut, uint256 utilizationCapBps, uint256 tradeUtilizationBps, bool cappedByUtilization, uint256 twapOut, uint256 twapDeviationBps, uint256 effectiveFeeBps, uint256 tier, uint256 priceImpactBps, uint256 priceImpactDeduction) result)` `view`
- `setPecor(address _pecor)` `nonpayable`
- `vault() returns (address)` `view`

### Errors

- `OwnableUnauthorizedAccount(address account)`
- `StalePrice()`
- `TwapWindowInvalid()`

## spot (`share.json`)

Source: [`precompiles/spot/abi/share.json`](../../precompiles/spot/abi/share.json).

Destination: Dynamic native ERC20 pointer.

Retained share contract ABI, not a fixed rail ABI. Resolve the actual native ERC20 pointer with vault `shareTokens(token)`. Public `mint`, `burn`, `owner`, `renounceOwnership` and `transferOwnership`, and the `OwnershipTransferred` event, are not served by that pointer. Mint/burn accounting belongs to the spot keeper; only ERC20 Transfer/Approval logs describe the pointer.

### Functions

- `allowance(address owner, address spender) returns (uint256)` `view`
- `approve(address spender, uint256 value) returns (bool)` `nonpayable`
- `balanceOf(address account) returns (uint256)` `view`
- `burn(address from, uint256 amount)` `nonpayable`
- `decimals() returns (uint8)` `view`
- `mint(address to, uint256 amount)` `nonpayable`
- `name() returns (string)` `view`
- `owner() returns (address)` `view`
- `renounceOwnership()` `nonpayable`
- `symbol() returns (string)` `view`
- `totalSupply() returns (uint256)` `view`
- `transfer(address to, uint256 value) returns (bool)` `nonpayable`
- `transferFrom(address from, address to, uint256 value) returns (bool)` `nonpayable`
- `transferOwnership(address newOwner)` `nonpayable`

### Events

- `Approval(address indexed owner, address indexed spender, uint256 value)`
- `OwnershipTransferred(address indexed previousOwner, address indexed newOwner)`
- `Transfer(address indexed from, address indexed to, uint256 value)`

### Errors

- `ERC20InsufficientAllowance(address spender, uint256 allowance, uint256 needed)`
- `ERC20InsufficientBalance(address sender, uint256 balance, uint256 needed)`
- `ERC20InvalidApprover(address approver)`
- `ERC20InvalidReceiver(address receiver)`
- `ERC20InvalidSender(address sender)`
- `ERC20InvalidSpender(address spender)`
- `OwnableInvalidOwner(address owner)`
- `OwnableUnauthorizedAccount(address account)`

## spot (`stoporders.json`)

Source: [`precompiles/spot/abi/stoporders.json`](../../precompiles/spot/abi/stoporders.json).

Destination: `0x0000000000000000000000000000000000001023`.

Served only at this rail address. Select this ABI for calls and logs at this address.

### Functions

- `BPS_DENOMINATOR() returns (uint256)` `view`
- `MAX_ORDER_FEE_BPS() returns (uint256)` `view`
- `accumulatedFees(address) returns (uint256)` `view`
- `activeStopLimitOrderIds(uint256) returns (uint256)` `view`
- `activeStopLossOrderIds(uint256) returns (uint256)` `view`
- `batchCheckAndExecuteStopLimits(uint256[] orderIds) returns (uint256 activatedCount, uint256 executedCount)` `nonpayable`
- `batchExecuteStopLossOrders(uint256[] orderIds) returns (uint256 executedCount)` `nonpayable`
- `cancelStopLimitOrder(uint256 orderId)` `nonpayable`
- `cancelStopLossOrder(uint256 orderId)` `nonpayable`
- `checkStopLimitActivation(uint256 orderId)` `nonpayable`
- `collectFees(address token)` `nonpayable`
- `executeStopLimitOrder(uint256 orderId)` `nonpayable`
- `executeStopLossOrder(uint256 orderId)` `nonpayable`
- `feeCollector() returns (address)` `view`
- `fillQualityTwapPeriod() returns (uint256)` `view`
- `getAccumulatedFees(address token) returns (uint256)` `view`
- `getActivatableStopLimits(uint256 maxCount) returns (uint256[] orderIds)` `view`
- `getActiveStopLimitOrderCount() returns (uint256)` `view`
- `getActiveStopLossOrderCount() returns (uint256)` `view`
- `getExecutableStopLimits(uint256 maxCount) returns (uint256[] orderIds)` `view`
- `getStopLimitOrder(uint256 orderId) returns ((uint256 id, address user, address stablecoin, address token, uint256 amount, uint256 stopPrice, uint256 limitPrice, uint8 orderType, uint8 status, uint256 createdAt, uint256 expiresAt))` `view`
- `getStopLossOrder(uint256 orderId) returns ((uint256 id, address user, address stablecoin, address token, uint256 amount, uint256 triggerPrice, uint8 status, uint256 createdAt, uint256 expiresAt))` `view`
- `getUserStopLimitOrders(address user) returns (uint256[])` `view`
- `getUserStopLossOrders(address user) returns (uint256[])` `view`
- `keepers(address) returns (bool)` `view`
- `nextOrderId() returns (uint256)` `view`
- `orderFeeBps() returns (uint256)` `view`
- `owner() returns (address)` `view`
- `pause()` `nonpayable`
- `paused() returns (bool)` `view`
- `placeStopLimitBuy(address stablecoin, address token, uint256 amount, uint256 stopPrice, uint256 limitPrice, uint256 expiresAt) returns (uint256 orderId)` `nonpayable`
- `placeStopLimitSell(address token, address stablecoin, uint256 amount, uint256 stopPrice, uint256 limitPrice, uint256 expiresAt) returns (uint256 orderId)` `nonpayable`
- `placeStopLoss(address token, address stablecoin, uint256 tokenAmount, uint256 triggerPrice, uint256 expiresAt) returns (uint256 orderId)` `nonpayable`
- `priceOracle() returns (address)` `view`
- `renounceOwnership()` `nonpayable`
- `setFeeCollector(address _collector)` `nonpayable`
- `setFillQualityTwapPeriod(uint256 period)` `nonpayable`
- `setKeeper(address keeper, bool authorized)` `nonpayable`
- `setOrderFee(uint256 _feeBps)` `nonpayable`
- `setPriceOracle(address _oracle)` `nonpayable`
- `setTransactionTracker(address _t)` `nonpayable`
- `stopLimitOrders(uint256) returns (uint256 id, address user, address stablecoin, address token, uint256 amount, uint256 stopPrice, uint256 limitPrice, uint8 orderType, uint8 status, uint256 createdAt, uint256 expiresAt)` `view`
- `stopLossOrders(uint256) returns (uint256 id, address user, address stablecoin, address token, uint256 amount, uint256 triggerPrice, uint8 status, uint256 createdAt, uint256 expiresAt)` `view`
- `transactionTracker() returns (address)` `view`
- `transferOwnership(address newOwner)` `nonpayable`
- `unpause()` `nonpayable`
- `userStopLimitOrders(address, uint256) returns (uint256)` `view`
- `userStopLossOrders(address, uint256) returns (uint256)` `view`
- `vault() returns (address)` `view`

### Events

- `OwnershipTransferred(address indexed previousOwner, address indexed newOwner)`
- `Paused(address account)`
- `StopLimitActivated(uint256 indexed orderId, uint256 activationPrice)`
- `StopLimitCancelled(uint256 indexed orderId)`
- `StopLimitExecuted(uint256 indexed orderId, uint256 executionPrice)`
- `StopLimitOrderCreated(uint256 indexed orderId, address indexed user, uint8 orderType, uint256 amount, uint256 stopPrice, uint256 limitPrice)`
- `StopLossOrderCancelled(uint256 indexed orderId)`
- `StopLossOrderCreated(uint256 indexed orderId, address indexed user, uint256 amount, uint256 triggerPrice)`
- `StopLossOrderExecuted(uint256 indexed orderId, uint256 executionPrice)`
- `Unpaused(address account)`

### Errors

- `CannotCancel()`
- `EnforcedPause()`
- `ExpectedPause()`
- `FeeExceedsMax()`
- `InsufficientLiquidity()`
- `InsufficientOutput()`
- `InvalidExpiry()`
- `InvalidOracle()`
- `InvalidPrice()`
- `InvalidPrices()`
- `InvalidTWAPPeriod()`
- `InvalidVault()`
- `LimitMustBeAboveStopBuy()`
- `LimitMustBeBelowStopSell()`
- `NoFeesToCollect()`
- `NotAStablecoin()`
- `NotActivated()`
- `NotAuthorized()`
- `NotFeeCollector()`
- `NotOrderOwner()`
- `NotPending()`
- `OrderBelowMinimumNotional()`
- `OrderExpired()`
- `OrderNotPending()`
- `OwnableInvalidOwner(address owner)`
- `OwnableUnauthorizedAccount(address account)`
- `PriceAboveLimit()`
- `PriceAboveTrigger()`
- `PriceBelowLimit()`
- `ReentrancyGuardReentrantCall()`
- `SafeERC20FailedOperation(address token)`
- `StalePrice()`
- `TokenIsStablecoin()`
- `TokenPaused()`
- `TooManyActiveOrders()`
- `UnsupportedInputTransfer()`
- `UnsupportedTransfer()`
- `ZeroAmount()`

## spot (`vault.json`)

Source: [`precompiles/spot/abi/vault.json`](../../precompiles/spot/abi/vault.json).

Destination: `0x0000000000000000000000000000000000001025`.

Served only at this rail address. Select this ABI for calls and logs at this address.

### Functions

- `BPS_DENOMINATOR() returns (uint256)` `view`
- `DEFAULT_MAX_UTILIZATION_BPS() returns (uint256)` `view`
- `WETH() returns (address)` `view`
- `authorizedOperators(address) returns (bool)` `view`
- `deposit(address token, uint256 amount)` `nonpayable`
- `depositBatch(address[] tokenList, uint256[] amounts)` `nonpayable`
- `depositNative()` `payable`
- `emergencyWithdraw(address token, uint256 amount, address recipient)` `nonpayable`
- `getAllReserves() returns (address[], uint256[])` `view`
- `getRegisteredStablecoins() returns (address[])` `view`
- `getRegisteredTokenCount() returns (uint256)` `view`
- `getRegisteredTokens() returns (address[])` `view`
- `getReserves(address token) returns (uint256)` `view`
- `getTokenDecimals(address token) returns (uint8)` `view`
- `getTokenInfo(address token) returns (bool isRegistered, bool isStablecoin_, uint8 decimals, uint256 reserves, uint256 totalDeposited, uint256 totalWithdrawn)` `view`
- `getUntrackedFunds(address token) returns (uint256)` `view`
- `hasLiquidity(address token, uint256 amount) returns (bool)` `view`
- `isStablecoin(address token) returns (bool)` `view`
- `isTokenPaused(address token) returns (bool)` `view`
- `maxUtilizationBps(address) returns (uint256)` `view`
- `owner() returns (address)` `view`
- `pausedTokens(address) returns (bool)` `view`
- `pullTokens(address token, address from, uint256 amount) returns (uint256)` `nonpayable`
- `pushTokens(address token, address to, uint256 amount)` `nonpayable`
- `registerToken(address token, bool _isStablecoin)` `nonpayable`
- `registeredStablecoins(uint256) returns (address)` `view`
- `registeredTokens(uint256) returns (address)` `view`
- `renounceOwnership()` `nonpayable`
- `setMaxUtilizationBps(address token, uint256 capBps)` `nonpayable`
- `setOperator(address operator, bool authorized)` `nonpayable`
- `setStablecoinStatus(address token, bool _isStablecoin)` `nonpayable`
- `setTokenPaused(address token, bool paused)` `nonpayable`
- `setTransactionTracker(address _tracker)` `nonpayable`
- `shareTokens(address) returns (address)` `view`
- `syncAllReserves()` `nonpayable`
- `syncReserves(address token)` `nonpayable`
- `tokens(address) returns (bool isRegistered, bool isStablecoin, uint8 decimals, uint256 reserves, uint256 totalDeposited, uint256 totalWithdrawn)` `view`
- `transactionTracker() returns (address)` `view`
- `transferOwnership(address newOwner)` `nonpayable`
- `updateReserves(address tokenIn, uint256 amountIn, address tokenOut, uint256 amountOut)` `nonpayable`
- `withdraw(address token, uint256 shares) returns (uint256 amount)` `nonpayable`
- `withdrawNative(uint256 amount, address to)` `nonpayable`

### Events

- `Deposit(address indexed token, address indexed depositor, uint256 amount, uint256 newReserve)`
- `LPWithdrawal(address indexed token, address indexed provider, uint256 shares, uint256 amount)`
- `NativeDeposit(address indexed depositor, uint256 amount, uint256 newReserve)`
- `NativeWithdrawal(address indexed recipient, uint256 amount, uint256 newReserve)`
- `OperatorUpdated(address indexed operator, bool authorized)`
- `OwnershipTransferred(address indexed previousOwner, address indexed newOwner)`
- `ReservesSync(address indexed token, uint256 oldReserves, uint256 newReserves, uint256 recovered)`
- `ReservesUpdated(address indexed token, uint256 oldReserve, uint256 newReserve)`
- `ShareTokenCreated(address indexed token, address indexed shareToken)`
- `TokenPauseUpdated(address indexed token, bool paused)`
- `TokenRegistered(address indexed token, uint8 decimals, bool isStablecoin)`
- `TransactionTrackerUpdated(address indexed tracker)`
- `UtilizationCapUpdated(address indexed token, uint256 maxUtilizationBps)`
- `Withdrawal(address indexed token, address indexed recipient, uint256 amount, uint256 newReserve)`

### Errors

- `InsufficientLiquidity()`
- `InvalidArrayLength()`
- `OwnableInvalidOwner(address owner)`
- `OwnableUnauthorizedAccount(address account)`
- `ReentrancyGuardReentrantCall()`
- `SafeERC20FailedOperation(address token)`
- `TokenNotRegistered()`
- `TokenPaused()`
- `UnsupportedInputTransfer()`
- `UnsupportedTransfer()`
- `WethOnly()`
- `ZeroAmount()`

## staking (`abi.json`)

Source: [`precompiles/staking/abi.json`](../../precompiles/staking/abi.json).

Destination: `0x0000000000000000000000000000000000001005`.

### Functions

- `createValidator(string pubKeyHex, string moniker, string commissionRate, string commissionMaxRate, string commissionMaxChangeRate, uint256 minSelfDelegation) returns (bool success)` `payable`
- `delegate(string valAddress) returns (bool success)` `payable`
- `delegation(address delegator, string valAddress) returns (((uint256 amount, string denom) balance, (string delegator_address, uint256 shares, uint256 decimals, string validator_address) delegation) delegation)` `view`
- `delegatorDelegations(address delegator, bytes nextKey) returns ((((uint256 amount, string denom) balance, (string delegator_address, uint256 shares, uint256 decimals, string validator_address) delegation)[] delegations, bytes nextKey) response)` `view`
- `delegatorUnbondingDelegations(address delegator, bytes nextKey) returns (((string delegatorAddress, string validatorAddress, (int64 creationHeight, int64 completionTime, string initialBalance, string balance)[] entries)[] unbondingDelegations, bytes nextKey) response)` `view`
- `delegatorValidator(address delegator, string validatorAddress) returns ((string operatorAddress, bytes consensusPubkey, bool jailed, int32 status, string tokens, string delegatorShares, string description, int64 unbondingHeight, int64 unbondingTime, string commissionRate, string commissionMaxRate, string commissionMaxChangeRate, int64 commissionUpdateTime, string minSelfDelegation) validator)` `view`
- `delegatorValidators(address delegator, bytes nextKey) returns (((string operatorAddress, bytes consensusPubkey, bool jailed, int32 status, string tokens, string delegatorShares, string description, int64 unbondingHeight, int64 unbondingTime, string commissionRate, string commissionMaxRate, string commissionMaxChangeRate, int64 commissionUpdateTime, string minSelfDelegation)[] validators, bytes nextKey) response)` `view`
- `editValidator(string moniker, string commissionRate, uint256 minSelfDelegation) returns (bool success)` `nonpayable`
- `historicalInfo(int64 height) returns ((int64 height, (string operatorAddress, bytes consensusPubkey, bool jailed, int32 status, string tokens, string delegatorShares, string description, int64 unbondingHeight, int64 unbondingTime, string commissionRate, string commissionMaxRate, string commissionMaxChangeRate, int64 commissionUpdateTime, string minSelfDelegation)[] validators) historicalInfo)` `view`
- `params() returns ((uint64 unbondingTime, uint32 maxValidators, uint32 maxEntries, uint32 historicalEntries, string bondDenom, string minCommissionRate, string maxVotingPowerRatio, string maxVotingPowerEnforcementThreshold) params)` `view`
- `pool() returns ((string notBondedTokens, string bondedTokens) pool)` `view`
- `redelegate(string srcAddress, string dstAddress, uint256 amount) returns (bool success)` `nonpayable`
- `redelegations(string delegator, string srcValidator, string dstValidator, bytes nextKey) returns (((string delegatorAddress, string validatorSrcAddress, string validatorDstAddress, (int64 creationHeight, int64 completionTime, string initialBalance, string sharesDst)[] entries)[] redelegations, bytes nextKey) response)` `view`
- `unbondingDelegation(address delegator, string validatorAddress) returns ((string delegatorAddress, string validatorAddress, (int64 creationHeight, int64 completionTime, string initialBalance, string balance)[] entries) unbondingDelegation)` `view`
- `undelegate(string valAddress, uint256 amount) returns (bool success)` `nonpayable`
- `validator(string validatorAddress) returns ((string operatorAddress, bytes consensusPubkey, bool jailed, int32 status, string tokens, string delegatorShares, string description, int64 unbondingHeight, int64 unbondingTime, string commissionRate, string commissionMaxRate, string commissionMaxChangeRate, int64 commissionUpdateTime, string minSelfDelegation) validator)` `view`
- `validatorDelegations(string validatorAddress, bytes nextKey) returns ((((uint256 amount, string denom) balance, (string delegator_address, uint256 shares, uint256 decimals, string validator_address) delegation)[] delegations, bytes nextKey) response)` `view`
- `validatorUnbondingDelegations(string validatorAddress, bytes nextKey) returns (((string delegatorAddress, string validatorAddress, (int64 creationHeight, int64 completionTime, string initialBalance, string balance)[] entries)[] unbondingDelegations, bytes nextKey) response)` `view`
- `validators(string status, bytes nextKey) returns (((string operatorAddress, bytes consensusPubkey, bool jailed, int32 status, string tokens, string delegatorShares, string description, int64 unbondingHeight, int64 unbondingTime, string commissionRate, string commissionMaxRate, string commissionMaxChangeRate, int64 commissionUpdateTime, string minSelfDelegation)[] validators, bytes nextKey) response)` `view`

### Events

- `Delegate(address indexed delegator, string validator, uint256 amount)`
- `DelegationRewardsWithdrawn(address indexed delegator, string validator, uint256 amount)`
- `Redelegate(address indexed delegator, string srcValidator, string dstValidator, uint256 amount)`
- `Undelegate(address indexed delegator, string validator, uint256 amount)`
- `ValidatorCreated(address indexed creator, string validatorAddress, string moniker)`
- `ValidatorEdited(address indexed editor, string validatorAddress, string moniker)`

## wasmd (`abi.json`)

Source: [`precompiles/wasmd/abi.json`](../../precompiles/wasmd/abi.json).

Destination: `0x0000000000000000000000000000000000001002`.

### Functions

- `execute(string contractAddress, bytes msg, bytes coins) returns (bytes response)` `payable`
- `execute_batch((string contractAddress, bytes msg, bytes coins)[] executeMsgs) returns (bytes[] responses)` `payable`
- `instantiate(uint64 codeID, string admin, bytes msg, string label, bytes coins) returns (string contractAddr, bytes data)` `payable`
- `query(string contractAddress, bytes req) returns (bytes response)` `view`

## xweb (`abi.json`)

Source: [`precompiles/xweb/abi.json`](../../precompiles/xweb/abi.json).

Destination: `0x0000000000000000000000000000000000001019`.

Current program-origin events use `kind`, `payload` and `sequence`. The catalogue below reflects the current ABI, including indexed fields; the historical origin/input/deadline schema must not be used to decode these logs.

### Functions

- `fee() returns (uint256)` `view`
- `fulfil(uint64 requestId, bytes response, bytes32 contentDigest, uint32 fullLength, bytes[] signatures) returns (uint8 callback, uint64 callbackGasUsed)` `nonpayable`
- `fulfilProgram(bytes observation)` `nonpayable`
- `getAttestors() returns ((address signer, string payout, bytes publicKey)[] attestors, uint32 required)` `view`
- `getKernelNetworkId() returns (uint32 kernelNetworkId)` `view`
- `getParams() returns ((uint256 fee, uint32 maxPayloadBytes, uint64 maxCallbackGas, uint64 timeoutBlocks, bool paused))` `view`
- `getProgramResult(bytes32 programId, uint64 requestId) returns (bytes32 contentDigest, uint32 fullLength, uint32 responseLength, bytes response)` `view`
- `getRequest(uint64 requestId) returns ((uint64 id, address requester, uint8 kind, bytes32 payloadHash, uint64 callbackGas, uint256 fee, uint64 height, uint64 timeoutHeight, uint8 status, uint8 level, address attestor))` `view`
- `getResult(uint64 requestId) returns ((uint64 requestId, bytes response, bytes32 contentDigest, uint32 fullLength, address[] signers, uint64 height, uint8 callback, uint64 callbackGasUsed, uint8 level))` `view`
- `refund(uint64 requestId)` `nonpayable`
- `request(uint8 kind, bytes payload, uint64 callbackGas) returns (uint64 requestId)` `payable`
- `threshold() returns (uint32)` `view`

### Events

- `XWebFulfilled(uint64 indexed requestId, address indexed requester, bytes32 contentDigest, uint32 fullLength, uint8 level, uint8 callback, uint64 callbackGasUsed)`
- `XWebProgramRequested(bytes32 indexed programId, uint64 indexed requestId, uint8 kind, bytes payload, uint64 sequence)`
- `XWebRefunded(uint64 indexed requestId, address indexed requester, uint256 refunded)`
- `XWebRequested(uint64 indexed requestId, address indexed requester, uint8 kind, bytes payload, uint64 callbackGas, uint256 paid, uint64 timeoutHeight)`

## layerxagent/clearing (`abiJSON`)

Source: [`precompiles/layerxagent/clearing/clearing.go:abiJSON`](../../precompiles/layerxagent/clearing/clearing.go).

Destination: `0x0000000000000000000000000000000000000902`.

Served after merge activation with the node-supplied keepers and registered constructor.

### Functions

- `computeClearing(int256 oraclePrice, (int16 offsetBps, uint128 size)[] buyOrders, (int16 offsetBps, uint128 size)[] sellOrders) returns (int16 clearingOffsetBps, int256 clearingPrice, uint256 matchedVolume)` `pure`

## layerxagent/eip712 (`abiJSON`)

Source: [`precompiles/layerxagent/eip712/eip712.go:abiJSON`](../../precompiles/layerxagent/eip712/eip712.go).

Destination: `0x0000000000000000000000000000000000000908`.

Served after merge activation with the node-supplied keepers and registered constructor.

### Functions

- `domainSeparator(string name, string version, uint256 chainId, address verifyingContract) returns (bytes32 separator)` `pure`
- `hashTypedData(bytes32 domainSeparator, bytes32 structHash) returns (bytes32 digest)` `pure`
- `recoverTypedSigner(bytes32 domainSeparator, bytes32 structHash, bytes signature) returns (address signer)` `view`

## layerxagent/oracle (`abiJSON`)

Source: [`precompiles/layerxagent/oracle/oracle.go:abiJSON`](../../precompiles/layerxagent/oracle/oracle.go).

Destination: `0x0000000000000000000000000000000000000903`.

Served after merge activation with the node-supplied keepers and registered constructor.

### Functions

- `aggregate((int256 price, uint256 confidence, uint256 timestamp)[] feeds) returns (int256 price, uint256 confidence)` `pure`
- `getValidatorPrice(bytes32 feedID) returns (int256 price, uint256 quorum, uint256 timestamp)` `view`
- `submitPrice(bytes32 feedID, int256 price, uint256 confidence)` `nonpayable`

## layerxagent/orob (`abiJSON`)

Source: [`precompiles/layerxagent/orob/orob.go:abiJSON`](../../precompiles/layerxagent/orob/orob.go).

Destination: `0x0000000000000000000000000000000000000901`.

Served after merge activation with the node-supplied keepers and registered constructor.

### Functions

- `resolveOffset(int256 oraclePrice, int16 offsetBps) returns (int256 absolutePrice)` `pure`
- `resolveOffsetBatch(int256 oraclePrice, int16[] offsetsBps) returns (int256[] absolutePrices)` `pure`
- `toOffset(int256 oraclePrice, int256 absolutePrice) returns (int16 offsetBps)` `pure`

## layerxagent/pofq (`abiJSON`)

Source: [`precompiles/layerxagent/pofq/pofq.go:abiJSON`](../../precompiles/layerxagent/pofq/pofq.go).

Destination: `0x0000000000000000000000000000000000000904`.

Served after merge activation with the node-supplied keepers and registered constructor.

### Functions

- `scoreBatch((int256 fillPrice, int256 oraclePrice, uint128 size)[] fills) returns (uint256 avgScore, uint256 totalVolume)` `pure`
- `scoreFill(int256 fillPrice, int256 oraclePrice) returns (uint256 score)` `pure`
- `updateRollingScore(uint256 currentScore, uint256 currentWeight, uint256 newScore, uint256 newWeight, uint16 decayBps) returns (uint256 updatedScore, uint256 updatedWeight)` `pure`

## layerxagent/scheduler (`abiJSON`)

Source: [`precompiles/layerxagent/scheduler/scheduler.go:abiJSON`](../../precompiles/layerxagent/scheduler/scheduler.go).

Destination: `0x0000000000000000000000000000000000000905`.

Served after merge activation with the node-supplied keepers and registered constructor.

### Functions

- `cancel(uint256 jobId)` `nonpayable`
- `getJob(uint256 jobId) returns (uint256 id, address creator, address target, bytes callData, uint64 executeAtBlock, uint64 gasLimit, uint256 deposit, bool active)` `view`
- `pending(address creator) returns (uint256[] jobIds)` `view`
- `reschedule(uint256 jobId, uint64 newBlock)` `nonpayable`
- `schedule(address target, bytes callData, uint64 executeAtBlock, uint64 gasLimit) returns (uint256 jobId)` `payable`

### Events

- `JobCancelled(uint256 indexed jobId, address indexed creator, uint256 refund)`
- `JobRescheduled(uint256 indexed jobId, uint64 newBlock)`
- `JobScheduled(uint256 indexed jobId, address indexed creator, address target, uint64 executeAtBlock, uint64 gasLimit, uint256 deposit)`

## layerxagent/streams (`abiJSON`)

Source: [`precompiles/layerxagent/streams/streams.go:abiJSON`](../../precompiles/layerxagent/streams/streams.go).

Destination: `0x0000000000000000000000000000000000000906`.

Served after merge activation with the node-supplied keepers and registered constructor.

### Functions

- `accrued(uint256 streamId) returns (uint256 amount)` `view`
- `close(uint256 streamId) returns (uint256 finalPaid)` `nonpayable`
- `getStream(uint256 streamId) returns ((uint256 id, address payer, address payee, address token, uint256 ratePerSecond, uint256 cap, uint64 startTime, uint64 stopTime, uint256 settled, bool active) stream)` `view`
- `open(address payee, address token, uint256 ratePerSecond, uint64 startTime, uint64 stopTime, uint256 cap) returns (uint256 streamId)` `nonpayable`
- `settle(uint256 streamId) returns (uint256 paid)` `nonpayable`
- `updateRate(uint256 streamId, uint256 newRate)` `nonpayable`

### Events

- `StreamClosed(uint256 indexed streamId, uint256 finalPaid, uint256 refund)`
- `StreamOpened(uint256 indexed streamId, address indexed payer, address indexed payee, address token, uint256 ratePerSecond, uint256 cap)`
- `StreamRateUpdated(uint256 indexed streamId, uint256 oldRate, uint256 newRate)`
- `StreamSettled(uint256 indexed streamId, address indexed payee, uint256 paid, uint256 committed)`

## layerxagent/teeattestor (`abiJSON`)

Source: [`precompiles/layerxagent/teeattestor/teeattestor.go:abiJSON`](../../precompiles/layerxagent/teeattestor/teeattestor.go).

Destination: `0x0000000000000000000000000000000000000907`.

Served after merge activation with the node-supplied keepers and registered constructor.

### Functions

- `rootCount(uint8 family) returns (uint256 count)` `view`
- `rootOf(uint8 family, uint256 index) returns (bytes root)` `view`
- `verify(uint8 family, bytes quote) returns ((uint8 family, bytes32 mrtd, bytes32 reportData, uint64 timestamp, bool debug) att)` `view`
- `verifyAndExpect(uint8 family, bytes quote, bytes32 expectedReportData) returns ((uint8 family, bytes32 mrtd, bytes32 reportData, uint64 timestamp, bool debug) att)` `view`

### Events

- `AttestationVerified(uint8 indexed family, bytes32 indexed mrtd, bytes32 reportData, uint64 timestamp, bool debug)`

## Solidity interfaces (`contracts/src/precompiles/`)

These source declarations are retained for contract bindings and historical decoding. The availability rules above also apply to these interfaces. Struct definitions are available in each linked source. Use the JSON ABI when an interface exposes only part of a package ABI.

### IAddr

Source: [IAddr.sol](../../contracts/src/precompiles/IAddr.sol).

- `bindLayerX(bytes32 didPublicKey, bytes memory signature) external`
- `unbindLayerX() external`
- `getPaxAddr(address addr) external view returns (string memory response)`
- `getEvmAddr(string memory addr) external view returns (address response)`
- `getLayerXDid(address addr) external view returns (bytes32 didPublicKey, string memory did)`
- `getEvmAddrByLayerX(bytes32 didPublicKey) external view returns (address evmAddr)`
- `layerXBindNonce(address addr) external view returns (uint64 nonce)`
- `getUnifiedAccount(address addr) external view returns ( address evm, string memory paxAddr, bytes32 didPublicKey, bytes32 layerxMainAccountId )`
- constant `ADDR_PRECOMPILE_ADDRESS` = `0x0000000000000000000000000000000000001004`

### IBank

Source: [IBank.sol](../../contracts/src/precompiles/IBank.sol).

- `send(address fromAddress, address toAddress, string memory denom, uint256 amount) external returns (bool success)`
- `balance(address acc, string memory denom) external view returns (uint256 amount)`
- `name(string memory denom) external view returns (string memory response)`
- `symbol(string memory denom) external view returns (string memory response)`
- `decimals(string memory denom) external view returns (uint8 response)`
- `supply(string memory denom) external view returns (uint256 response)`
- constant `BANK_PRECOMPILE_ADDRESS` = `0x0000000000000000000000000000000000001001`

### IJson

Source: [IJson.sol](../../contracts/src/precompiles/IJson.sol).

- `extractAsBytes(bytes memory input, string memory key) external view returns (bytes memory response)`
- `extractAsBytesList(bytes memory input, string memory key) external view returns (bytes[] memory response)`
- `extractAsUint256(bytes memory input, string memory key) external view returns (uint256 response)`
- constant `JSON_PRECOMPILE_ADDRESS` = `0x0000000000000000000000000000000000001003`

### ILayerXAnchor

Source: [ILayerXAnchor.sol](../../contracts/src/precompiles/ILayerXAnchor.sol).

Historical interface. Anchor writes retire at the merge; all selectors retire at the store-deletion plan. Calls then revert with `LXP_MODULE_DISABLED` as described above.

- `submitCheckpoint(bytes calldata header, bytes calldata headerSignature, bytes calldata certificate) external returns (bytes32 checkpointId, uint8 status)`
- `submitAvailabilityAttestation(bytes calldata attestation) external returns (uint8 availabilityMask)`
- `finalize(uint64 batchNumber) external returns (bool)`
- `registerGuarantor(bytes32 guarantorId, address signer) external payable returns (bool)`
- `increaseBond(bytes32 guarantorId) external payable returns (bool)`
- `beginUnbond(bytes32 guarantorId, uint256 amount) external returns (uint64 completionTime)`
- `completeUnbond(bytes32 guarantorId) external returns (uint256 amount)`
- `submitEquivocation(bytes calldata evidenceA, bytes calldata evidenceB) external returns (uint256 slashed)`
- `openChallenge(uint64 batchNumber, uint8 kind, bytes32 evidenceHash) external payable returns (uint64 challengeId)`
- `resolveChallenge(uint64 challengeId, bool upheld) external returns (bool)`
- `activateGuarantor(bytes32 guarantorId) external returns (bool)`
- `setSequencerAuthorization(bytes32 sequencerId, bytes32 publicKey, uint64 firstBatchNumber, uint64 lastBatchNumber) external returns (bool)`
- `latestFinalized() external view returns (uint64 batchNumber, bool exists)`
- `checkpoint(uint64 batchNumber) external view returns (Checkpoint memory)`
- `finalizedStateRoot(uint64 batchNumber) external view returns (bytes32 stateRoot, bool finalized)`
- `finalizedReceiptRoot(uint64 batchNumber) external view returns (bytes32 receiptRoot, bool finalized)`
- `guarantor(bytes32 guarantorId) external view returns (Guarantor memory)`
- `threshold() external view returns (uint32)`
- `statusOf(uint64 batchNumber) external view returns (uint8)`
- `checkpointBatch(bytes32 checkpointId) external view returns (uint64 batchNumber, uint8 status)`
- `checkpointGuarantors(uint64 batchNumber) external view returns (bytes32[] memory guarantorIds)`
- constant `LAYERX_ANCHOR_PRECOMPILE_ADDRESS` = `0x0000000000000000000000000000000000001014`

### ILayerXCustody

Source: [ILayerXCustody.sol](../../contracts/src/precompiles/ILayerXCustody.sol).

Historical interface. All selectors retire at the merge, including deposits, views, withdrawal claims, forced exits and deposit-root methods. Calls then revert with `LXP_MODULE_DISABLED` as described above.

- `deposit(bytes32 beneficiary) external payable returns (bytes32 depositId)`
- `depositToken(address pointer, uint256 amount, bytes32 beneficiary) external returns (bytes32 depositId)`
- `requestWithdrawal(bytes calldata receipt, bytes calldata proof, bytes calldata header, bytes calldata headerSignature) external returns (bytes32 claimId, uint64 availableAt)`
- `finaliseWithdrawal(bytes calldata receipt, bytes calldata proof, bytes calldata header, bytes calldata headerSignature) external returns (bytes32 claimId)`
- `requestForcedExit(bytes calldata witness, uint64 batchNumber, bytes32 account, bytes32 assetId, address recipient, bytes calldata recipientSignature) external returns (bytes32 claimId, uint64 availableAt)`
- `executeForcedExit(bytes calldata witness, uint64 batchNumber, bytes32 account, bytes32 assetId, address recipient, bytes calldata recipientSignature) external returns (bytes32 claimId)`
- `depositCount() external view returns (uint64 count)`
- `depositNonce(address depositor, bytes32 assetId) external view returns (uint64 nonce)`
- `getDeposit(bytes32 depositId) external view returns (DepositRecord memory record)`
- `getDepositByIndex(uint64 index) external view returns (DepositRecord memory record)`
- `getClaim(bytes32 claimId) external view returns (Claim memory claim)`
- `nullifierStatus(bytes32 nullifier) external view returns (uint8 status)`
- `getAsset(bytes32 assetId) external view returns (Asset memory asset)`
- `assetByPointer(address pointer) external view returns (bytes32 assetId)`
- `nativeAssetId() external view returns (bytes32 assetId)`
- `exitEligible() external view returns (bool eligible)`
- `registerDepositRoot(bytes calldata registration, bytes calldata signature, bytes32[] calldata leafOrdering) external`
- `depositRootAuthority() external view returns (bytes32 authority)`
- `depositRootRegistered(bytes32 checkpointId) external view returns (bool registered)`
- `depositRegistrationDigest(bytes32 checkpointId) external view returns (bytes32 commitment)`
- constant `LAYERX_CUSTODY_PRECOMPILE_ADDRESS` = `0x0000000000000000000000000000000000001013`

### ILayerXVerify

Source: [ILayerXVerify.sol](../../contracts/src/precompiles/ILayerXVerify.sol).

Historical interface. All selectors retire at the merge; the pure declarations below do not imply availability. Calls then revert with `LXP_MODULE_DISABLED` as described above.

- `verifyEd25519(bytes32 publicKey, uint8 domain, bytes calldata message, bytes calldata signature) external pure returns (bool valid)`
- `verifyReceipt(bytes calldata receipt, bytes32 sequencerPublicKey) external pure returns (ReceiptFacts memory facts)`
- `verifyReceiptInclusion(bytes calldata receipt, bytes calldata proof, bytes calldata batchHeader, bytes calldata headerSignature, bytes32 sequencerId, bytes32 sequencerPublicKey, uint64 firstBatchNumber, uint64 lastBatchNumber) external pure returns (ReceiptFacts memory facts, BatchFacts memory batch)`
- `verifyStateProof(bytes calldata witness, bytes32 stateRoot) external pure returns (uint16 moduleId, bytes memory key, bytes memory value)`
- `verifyDiscoveryProof(bytes calldata payload, bytes calldata proofMaterial, bytes32 programId, uint64 stalenessMs, bytes32 sequencerPublicKey) external pure returns (DiscoveryFacts memory head)`
- constant `LAYERX_VERIFY_PRECOMPILE_ADDRESS` = `0x0000000000000000000000000000000000001012`

### IWasmd

Source: [IWasmd.sol](../../contracts/src/precompiles/IWasmd.sol).

- `instantiate(uint64 codeID, string memory admin, bytes memory payload, string memory label, bytes memory coins) external returns (string memory contractAddr, bytes memory data)`
- `execute(string memory contractAddress, bytes memory payload, bytes memory coins) external returns (bytes memory response)`
- `query(string memory contractAddress, bytes memory req) external view returns (bytes memory response)`
- constant `WASMD_PRECOMPILE_ADDRESS` = `0x0000000000000000000000000000000000001002`

### IXWeb

Source: [IXWeb.sol](../../contracts/src/precompiles/IXWeb.sol).

`onXWebResponse` is the requester callback interface; it is implemented by the consumer, not served as an xweb precompile method.

- `request(uint8 kind, bytes calldata payload, uint64 callbackGas) external payable returns (uint64 requestId)`
- `fulfil(uint64 requestId, bytes calldata response, bytes32 contentDigest, uint32 fullLength, bytes[] calldata signatures) external returns (uint8 callback, uint64 callbackGasUsed)`
- `fulfilProgram(bytes calldata observation) external`
- `refund(uint64 requestId) external`
- `getRequest(uint64 requestId) external view returns (Request memory)`
- `getResult(uint64 requestId) external view returns (Result memory)`
- `getAttestors() external view returns (Attestor[] memory attestors, uint32 required)`
- `threshold() external view returns (uint32)`
- `fee() external view returns (uint256)`
- `getParams() external view returns (Params memory)`
- `getProgramResult(bytes32 programId, uint64 requestId) external view returns (bytes32 contentDigest, uint32 fullLength, uint32 responseLength, bytes memory response)`
- `getKernelNetworkId() external view returns (uint32 kernelNetworkId)`
- `onXWebResponse(uint64 requestId, bytes32 contentDigest, uint32 fullLength, bytes calldata response) external`
- constant `XWEB_PRECOMPILE_ADDRESS` = `0x0000000000000000000000000000000000001019`
