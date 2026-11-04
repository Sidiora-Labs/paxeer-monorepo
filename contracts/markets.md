# Native market contracts

Paxeer mainnet-beta is one BFT network running `paxd`, with EVM chain ID
125 (`0x7d`). The kernel is part of the network and proposes blocks. A
transaction receipt is final when its block commits. Perpetuals, spot and
price feeds use chain state; the [data services](perps-data.md) index that
state for the [trading client](perps-client.md).

Public JSON-RPC uses HTTPS at the root of `api1.mainnet-beta.paxeer.network`
through `api14.mainnet-beta.paxeer.network`, and
`api-mainnet-beta.paxeer.network`. The node namespaces are `debug`, `echo`,
`eth`, `net`, `pax`, `test` and `txpool`. Hosted `px_` methods have their own
[OpenRPC contract](../services/platform/hosted/gateway/openrpc.json).
`node.hyperpaxeer.com` is the hpx library mirror, not an RPC endpoint.
Sub-chains are tenant domains in the database.

## Addresses and ABI sources

The short addresses below are left-padded with zeroes to 20 bytes. Registration
and upgrade selection live in [precompiles/setup.go](../precompiles/setup.go).
Use the ABI for the active implementation at the queried height.

| Address | Surface | ABI |
| --- | --- | --- |
| `0x1008` | Denomination oracle | [oracle](../precompiles/oracle/abi.json) |
| `0x1015` | Native perpetuals | [perps](../precompiles/perps/abi/abi.json) |
| `0x1020` | Validator price feed | [pricefeed](../precompiles/pricefeed/abi/abi.json) |
| `0x1021` | PECOR swaps | [PECOR](../precompiles/spot/abi/pecor.json) |
| `0x1022` | PECOR limit orders | [orders](../precompiles/spot/abi/orders.json) |
| `0x1023` | PECOR stop orders | [stop orders](../precompiles/spot/abi/stoporders.json) |
| `0x1024` | PECOR quotes | [quoter](../precompiles/spot/abi/quoter.json) |
| `0x1025` | PECOR liquidity vault | [vault](../precompiles/spot/abi/vault.json) |

## Perps 0x1015

The [dispatcher](../precompiles/perps/perps.go) uses the
[perps keeper](../modules/perps/keeper). It rejects value, delegate calls,
writes under static calls and calls without a wired keeper. `GasTable` defines
the fixed method charges and token-call surcharges.

| Operation | Methods |
| --- | --- |
| Account and collateral | `createVault`, `getVault`, `deposit`, `withdraw`, `emergencyWithdraw`, `transferCollateral`, `getAccount`, `getAccountSummary`, `getBalance`, `getLockedBalance` |
| Margin | `setMarginMode`, `getMarginMode`, `topUpPosition`, `transferMargin`, `getPositionMargin`, `getMarginTiers` |
| Orders | `placeOrder`, `placeLimitOrder`, `placeStopLimitOrder`, `cancelOrder`, `cancelByCloid`, `getOrder`, `getOrderDetails`, `getUserOrderIds`, `executeOrder` |
| Positions | `openPosition`, `addSize`, `addCollateral`, `removeCollateral`, `closePosition`, `partialClose`, `closePositionWithParams`, `getPosition`, `getPositionDetails`, `getUserPositionIds`, `getUserMarketPosition` |
| Position triggers | `placeTakeProfitOrder`, `placeStopLossOrder`, `setPositionTpSl` |
| Markets and fees | `getMarket`, `getMarketConfig`, `getActiveMarketIds`, `totalMarkets`, `isMarketActive`, `getFees`, `getFeeTier`, `getProtocolAccounts` |
| Prices and funding | `getMarkPrice`, `getMarkPriceDetails`, `quoteMarket`, `getCurrentFundingRate`, `getFundingState`, `getFundingDetails`, `getPositionFunding`, `getOpenInterest` |
| Risk and quotes | `liquidate`, `checkLiquidatable`, `getLiquidationPrice`, `getInsuranceBalance`, `getADLRank`, `quoteOpenPosition`, `quoteClosePosition`, `quotePartialClose` |

The ABI declares overloads for account reads; select by argument types rather
than the Go ABI parser's numeric overload suffix. `placeOrder` takes one tuple;
copy its named components from the ABI. Client order IDs are `bytes16`.
`closePositionWithParams(positionId, closeSizeUsd, acceptablePrice, deadline)`
uses a zero close size for a full close. See the
[client call builders](perps-client.md#core) for bounded order construction.

[Market types](../modules/perps/types/market.go) distinguish six-decimal USDL
collateral units from 18-decimal USD prices and notionals. The default collateral
is `0x85FcD13735F4309833A503EE804ea32395851479`. Leverage arguments use 18-decimal
multiples; governance rate parameters use parts per million (`1_000_000` is one).
Query the market's tick, lot, tier and leverage constraints before submitting.

### Perps governance

[Params](../modules/perps/types/params.go) cover collateral, volume fee tiers,
maker rebates, funding, open-interest caps, withdrawal floors, liquidation and
bounded block processing. Changes are authority checked in the
[message handlers](../modules/perps/keeper/msg_server.go); market and pause
handlers live beside them. Genesis defaults are initialization values, not a
promise that governance has left the running values unchanged.

The [pause check](../modules/perps/keeper/pause.go) permits `closePosition`,
`partialClose`, `closePositionWithParams`, `cancelOrder`, `cancelByCloid`,
`deposit`, `addCollateral` and `topUpPosition` while paused. Other writes fail
with `ErrPaused`; reads remain available.

### Perps errors

[Registered module errors](../modules/perps/types/errors.go) use codespace
`perps`, codes 2–45 and `LXP_PERPS_` names. Examples include
`LXP_PERPS_PRICE_STALE`, `LXP_PERPS_INSUFFICIENT_MARGIN`,
`LXP_PERPS_SLIPPAGE_EXCEEDED` and `LXP_PERPS_PAUSED`.
The [EVM mapping](../precompiles/perps/errors.go) converts these identities to
parameterless custom errors such as `PerpsPriceStale()` and `PerpsPaused()`:
the entire revert payload is their four-byte selector. Unmapped errors use
`Error(string)`. Decode with the perps ABI; do not interpret the selector as a
module's numeric ABCI code.

### Perps events

The perps ABI declares the log fields and indexed arguments. Account events
include `VaultCreated`, `CollateralDeposited`, `CollateralWithdrawn` and
`CollateralTransferred`. Order events include `OrderPlaced`, `OrderOptions`,
`Fill`, `OrderExecuted`, `OrderCancelled`, `OrderExpired` and `OrderTriggered`.
Position, mark, funding, open-interest, liquidation, insurance and ADL events
are also declared there. [Module event names](../modules/perps/types/events.go)
and EVM log signatures are distinct encodings. The
[indexer](perps-data.md#decode) handles transaction and synthetic block receipts.

## Pricefeed 0x1020

[The ABI](../precompiles/pricefeed/abi/abi.json) serves `registerFeedKey(bytes)`,
`revokeFeedKey()`, `getPrice`, `getPriceBySymbol`, `getPrices`, `getMarket`,
`getMarketCount`, `marketIdOf`, `getObservation`, `getFeedKey`,
`getValidatorByFeedAccount` and `getParams`.

A feed key is a 33-byte compressed secp256k1 public key registered for a bonded
validator. The [state types](../modules/pricefeed/types/state.go) store the
validator/account mapping, per-market observations and aggregates. Prices are
18-decimal integers; source timestamps and aggregate timestamps are milliseconds.
Read freshness explicitly rather than treating any stored price as current.
[Parameter defaults and validation](../modules/pricefeed/types/params.go) define
staleness, clock skew, source count, divergence, deviation, submission size,
mark EMA and maximum price age.

### Pricefeed query

The [query schema](../api/pricefeed/query.proto) declares `Params`, `Price`,
`Prices`, `FeedKey`, `ValidatorByFeedAccount` and `Observation`. Its HTTP
annotations are under `/paxprotocol/paxchain/pricefeed/` on a configured Cosmos
API listener. These are distinct from the node's EVM JSON-RPC listener.
`Price` returns `index`, `mark`, `timestamp_ms`, `height`, `sources` and `fresh`.

The [CLI](../modules/pricefeed/client/cli/query.go) exposes:

```text
paxd query pricefeed params
paxd query pricefeed price <market-id>
paxd query pricefeed prices <comma-separated-market-ids>
paxd query pricefeed feed-key <validator>
paxd query pricefeed validator-by-feed-account <feed-account>
paxd query pricefeed observation <market-id> <validator>
```

`prices` accepts at most 64 IDs. CLI validator and feed-account arguments use
Bech32; the EVM ABI uses addresses. `MsgSubmitPrices` and `MsgSetParams` are
specified in [tx.proto](../api/pricefeed/tx.proto).

### Pricefeed errors

[errors.go](../modules/pricefeed/types/errors.go) registers codes 2–16 in
`pricefeed` and maps them to `LXP_PRICEFEED_*` reasons. In particular,
`INVALID_KEY`, `NOT_VALIDATOR`, `UNREGISTERED_FEEDER`, `RATE_LIMITED`,
`TOO_MANY_OBSERVATIONS`, `SUBMISSION_TOO_LARGE`, `DUPLICATE_MARKET`,
`SUBMISSION_ALONE` and `INVALID_OBSERVATION` distinguish input and authorization
refusals. The full names carry the `LXP_PRICEFEED_` prefix.

### Pricefeed events

The EVM ABI declares `FeedKeyRegistered` and `FeedKeyRevoked`.
[Module events](../modules/pricefeed/types/events.go) also carry pricefeed
state transitions; use the indexer's receipt classification to interpret them.

## Spot served set

[The spot implementation](../precompiles/spot/spot.go) registers five rails at
`0x1021`–`0x1025`; it has no entries at `0x1026` or `0x1027`.
Amounts use each token's registered decimals; execution prices use 18 decimals.
[Opening token parameters](../modules/spot/types/params.go) pin USDL, PAX and SID
to six decimals, with PAX and SID pricefeed market IDs 11 and 12.

### Spot 0x1021

PECOR serves market buys/sells, exact-input and exact-output swaps, native
swap variants, quote reads, fee and price-impact configuration, pause and
ownership methods. Consult the [ABI](../precompiles/spot/abi/pecor.json) and
[executor](../precompiles/spot/pecor.go) for argument order and authorization.

### Spot 0x1022

The [orders ABI](../precompiles/spot/abi/orders.json) defines limit buys/sells,
modification, cancellation, execution, expiry, batch operations and order reads.

### Spot 0x1023

The [stop ABI](../precompiles/spot/abi/stoporders.json) defines stop-loss and
stop-limit placement, cancellation, activation, execution and batch operations.
Stop activation and filling are separate state transitions.

### Spot 0x1024

The [quoter ABI](../precompiles/spot/abi/quoter.json) provides exact-input/output,
market, native and batch quotes, token prices, TWAP and liquidity reads.

### Spot 0x1025

The [vault ABI](../precompiles/spot/abi/vault.json) defines token registration,
reserves, utilization caps, deposits, withdrawals, native variants, operators
and reserve synchronization. Governance messages are specified in
[spot/tx.proto](../api/spot/tx.proto).

### Spot share token

Liquidity share pointers use the [share ABI](../precompiles/spot/abi/share.json).
Discover a token's pointer with `shareTokens(token)` on the vault; it is not a
sixth fixed rail. `ShareTokenCreated` supplies the token and pointer addresses.

[CreateShareToken](../modules/spot/keeper/shares.go) deploys a
`NativePaxTokensERC20` pointer with an immutable share-specific delegate boundary.
Its bank denomination is `spot/share/<lowercase token address>`. `balanceOf`
and `totalSupply` read bank state; the pointer's decimals match the underlying
registered token. Names are `Spot Vault Share <symbol>` and symbols are
`s<symbol>`. A missing pointer is the zero address; do not send share calls to it.
The pointer mapping and balances are chain-store state, not client cache state.
After reconnect, read the vault mapping and balances again at the selected
committed height.

The deployed interface serves `allowance`, `approve`, `balanceOf`, `decimals`,
`name`, `symbol`, `totalSupply`, `transfer` and `transferFrom`, plus native
getters `BankPrecompile`, `ddecimals`, `denom`, `nname` and `ssymbol`.
`approve`, `transfer` and `transferFrom` return a Solidity `bool`.
Only keeper `MintShares` and `BurnShares` change supply, through the spot bank
module account. Their `Transfer` logs use the pointer as emitter and the zero
address as mint source or burn destination. The other served event is `Approval`.

`mint(address,uint256)`, `burn(address,uint256)`, `owner()`,
`renounceOwnership()` and `transferOwnership(address)` revert with empty data
for every caller, including governance and the vault. There is no Solidity
owner. [share-solidity.json](../precompiles/spot/abi/share-solidity.json) is a
historical snapshot, not the deployed interface. The exact deployment and
refusal contract is [share-deployment.kvx](../precompiles/spot/abi/share-deployment.kvx).

An otherwise valid transfer exceeding the bank balance returns
`ERC20InsufficientBalance(address,uint256,uint256)` (`0xe450d38c`), with sender,
current balance and needed amount. Native allowance and invalid-address checks
retain their error precedence. A refusal preserves balances, supply and
allowance; decode it with the deployed share ABI.

### Spot resting

[End-block processing](../modules/spot/keeper/execute.go) handles resting
orders within configured scan, fill and expiry budgets. Placement does not
promise an immediate fill. Index order creation, activation, execution,
cancellation and expiry separately.

### Spot params

[Params](../modules/spot/types/params.go) include swap and order fees, impact,
pauses, keeper policy, TWAP checks, minimum notional, user order limits,
processing budgets and registered tokens. Fees use basis points (`10_000` is
one). `MsgUpdateParams` accepts the governance module account.

### Spot errors

[AbiError](../modules/spot/types/errors.go) carries both registered module
identity and Solidity revert data. The revert is a four-byte selector followed
by packed arguments, for example `ERC20InsufficientAllowance(address,uint256,uint256)`.
Parameterless examples include `Expired()`, `StalePrice()`,
`InsufficientLiquidity()`, `NotOrderOwner()` and `OrderBelowMinimumNotional()`.
Decode against the relevant rail ABI; governance parameter errors have a
separate `ParamsError` representation.

### Spot events

The rail ABIs define swap, order, vault and administrative logs, including
`SimpleSwap`, `MarketOrderExecuted`, `LimitOrderCreated`, `StopLimitActivated`,
`ReservesSync` and `ShareTokenCreated`. See the
[data decoder](perps-data.md#decode) for emitter-aware decoding.

### Spot storage

[Keeper storage](../modules/spot/keeper) holds reserves, shares, fees and order
state in the chain store. Its keys and typed records are defined in
[modules/spot/types](../modules/spot/types). Client balances and approvals must
follow token custody kind rather than assuming every pointer transfers by ERC20.

## Oracle 0x1008

The [denomination oracle](../precompiles/oracle/oracle.go) serves
`getExchangeRates()` and `getOracleTwaps(uint64 lookback_seconds)` using its
[ABI](../precompiles/oracle/abi.json). It is distinct from market-indexed
pricefeed at `0x1020`.

### Oracle errors

[Oracle module errors](../modules/oracle/types/errors.go) are the registered
identities used by the oracle message and query handlers. Use those codes and
the oracle ABI, rather than the perps custom-error table.

### Oracle events

[Oracle event constants](../modules/oracle/types/events.go) describe SDK events;
the `0x1008` ABI declares no EVM events.

## Layerxexchange retirement

The legacy `layerxexchange` intent/proof workflow is retired from `v6.12.0`.
Its keeper checks the upgrade activation before accepting writes, including
parameter and listing updates. Historical types remain for decoding and genesis
export; they do not describe a second execution or settlement network.

### Exchange legacy 0x1015

The historical [exchange ABI](../precompiles/layerxexchange/abi.json) belongs
to the retired implementation at `0x1015`. At native-perps activation use the
perps ABI above. Never decode a current receipt with the old intent ABI merely
because the address matches.

### Layerxexchange errors

[ErrorCode](../modules/layerxexchange/types/errors.go) maps legacy module
identities to `LXP_EXCHANGE_*` strings. `LXP_EXCHANGE_RETIRED` is the retirement
refusal. Proof-related legacy names such as `LXP_EXCHANGE_NOT_FINALIZED` are
historical; current receipt finality is BFT block commitment.

### Layerxexchange events

Legacy event protobufs live in [api/layerxexchange](../api/layerxexchange).
Use them only for the historical workflow and its exported records.

### Layerxexchange genesis

[Genesis types](../modules/layerxexchange/types/genesis.go) retain historical
markets, intents and owner nonces. Import/export validation does not reactivate
retired writes.

### Layerxexchange storage

[State and keys](../modules/layerxexchange/types) identify the legacy store;
[keeper retirement checks](../modules/layerxexchange/keeper/keeper.go) decide
whether a write is admissible at the current height.

## Supporting modules and CLI

These commands use the configured Cosmos client connection and normal query or
transaction flags. Their CLI constructors are linked below.

| Command | Source and behavior |
| --- | --- |
| <a id="cli-epoch-query-root"></a>`paxd query epoch` | [query.go](../modules/epoch/client/cli/query.go): Lists epoch queries. |
| <a id="cli-epoch-query-epoch"></a>`paxd query epoch epoch` | [query_epoch.go](../modules/epoch/client/cli/query_epoch.go): Reads current epoch state. |
| <a id="cli-epoch-query-params"></a>`paxd query epoch params` | [query_params.go](../modules/epoch/client/cli/query_params.go): Reads epoch parameters. |
| <a id="cli-epoch-tx-root"></a>`paxd tx epoch` | [tx.go](../modules/epoch/client/cli/tx.go): Root constructor; no transaction subcommands are added here. |
| <a id="cli-mint-query-root"></a>`paxd query mint` | [query.go](../modules/mint/client/cli/query.go): Lists mint queries. |
| <a id="cli-mint-query-minter"></a>`paxd query mint minter` | [query.go](../modules/mint/client/cli/query.go): Reads minter state. |
| <a id="cli-mint-query-params"></a>`paxd query mint params` | [query.go](../modules/mint/client/cli/query.go): Reads mint parameters. |
| <a id="cli-mint-tx-root"></a>`paxd tx mint` | [tx.go](../modules/mint/client/cli/tx.go): Lists mint transactions. |
| <a id="cli-mint-gov-update-minter"></a>`paxd tx mint update-minter <proposal-file>` | [tx.go](../modules/mint/client/cli/tx.go): Submits the minter governance proposal. |
| <a id="cli-oracle-query-root"></a>`paxd query oracle` | [query.go](../modules/oracle/client/cli/query.go): Lists denomination-oracle queries. |
| <a id="cli-oracle-query-actives"></a>`paxd query oracle actives` | [query.go](../modules/oracle/client/cli/query.go): Reads active denominations. |
| <a id="cli-oracle-query-exchange-rates"></a>`paxd query oracle exchange-rates [denom]` | [query.go](../modules/oracle/client/cli/query.go): Reads one denomination or all rates. |
| <a id="cli-oracle-query-feeder"></a>`paxd query oracle feeder <validator>` | [query.go](../modules/oracle/client/cli/query.go): Reads feeder delegation. |
| <a id="cli-oracle-query-params"></a>`paxd query oracle params` | [query.go](../modules/oracle/client/cli/query.go): Reads oracle parameters. |
| <a id="cli-oracle-query-price-snapshot-history"></a>`paxd query oracle price-snapshot-history` | [query.go](../modules/oracle/client/cli/query.go): Reads stored price snapshots. |
| <a id="cli-oracle-query-twaps"></a>`paxd query oracle twaps <lookback-seconds>` | [query.go](../modules/oracle/client/cli/query.go): Reads time-weighted rates. |
| <a id="cli-oracle-query-vote-penalty-counter"></a>`paxd query oracle vote-penalty-counter <validator>` | [query.go](../modules/oracle/client/cli/query.go): Reads vote penalties. |
| <a id="cli-oracle-query-vote-targets"></a>`paxd query oracle vote-targets` | [query.go](../modules/oracle/client/cli/query.go): Reads vote-target denominations. |
| <a id="cli-oracle-tx-root"></a>`paxd tx oracle` | [tx.go](../modules/oracle/client/cli/tx.go): Lists oracle transactions. |
| <a id="cli-oracle-tx-aggregate-vote"></a>`paxd tx oracle aggregate-vote <exchange-rates> <validator>` | [tx.go](../modules/oracle/client/cli/tx.go): Submits an aggregate exchange-rate vote. |
| <a id="cli-oracle-tx-set-feeder"></a>`paxd tx oracle set-feeder <feeder>` | [tx.go](../modules/oracle/client/cli/tx.go): Delegates feeder consent. |

### Epoch genesis

[Epoch validation](../modules/epoch/types/epoch.go) requires nonzero genesis
time, positive duration, a current epoch start at or after genesis and a
nonnegative current epoch height. [Genesis schemas](../api/epoch/genesis.proto)
define the imported/exported state.

### Epoch storage

[Epoch keeper](../modules/epoch/keeper/epoch.go) reads and writes the current
epoch in the module store. This clock state does not establish a separate
market settlement domain.

### Epoch events

[events.go](../modules/epoch/types/events.go) defines the epoch SDK event names
and attributes.

### Epoch errors

[errors.go](../modules/epoch/types/errors.go) registers epoch parsing, retrieval,
encoding and unknown-query errors with codes 2, 3, 4 and 6 respectively.

### Mint params

[Mint Params](../modules/mint/types/params.go) contains `mint_denom` and
`token_release_schedule`; defaults use the chain bond denomination and an empty
release schedule. Validate schedules through the module's parameter validator.

### Mint storage

[Mint keeper](../modules/mint/keeper/keeper.go) and
[keys](../modules/mint/types/keys.go) own minter state and parameters.

### Mint events

[Mint event constants](../modules/mint/types/events.go) define SDK event names.

### Mint REST

[Legacy REST registration](../modules/mint/client/rest/rest.go) defines minter
and parameter queries on a configured Cosmos REST service. The schema lives in
[api/mint](../api/mint); it is separate from EVM JSON-RPC.

## Message and source catalog

The tables below preserve interface-registry anchors for message schemas and
Go package exports. They identify source declarations, not additional public
HTTP endpoints. Legacy `layerxexchange` entries describe retired state and
compatibility code. Query schemas, message schemas and exported Go types must
be interpreted through their corresponding keeper and active upgrade.

| Registry anchor | Declaration sources |
| --- | --- |
| <a id="go-exported-modules-epoch"></a>`go-exported-modules-epoch` | [module.go](../modules/epoch/module.go) |
| <a id="go-exported-modules-epoch-client-cli"></a>`go-exported-modules-epoch-client-cli` | [query.go](../modules/epoch/client/cli/query.go), [tx.go](../modules/epoch/client/cli/tx.go) |
| <a id="go-exported-modules-epoch-client-wasm-bindings"></a>`go-exported-modules-epoch-client-wasm-bindings` | [queries.go](../modules/epoch/client/wasm/bindings/queries.go) |
| <a id="go-exported-modules-epoch-client-wasm"></a>`go-exported-modules-epoch-client-wasm` | [query.go](../modules/epoch/client/wasm/query.go) |
| <a id="go-exported-modules-epoch-keeper"></a>`go-exported-modules-epoch-keeper` | [abci.go](../modules/epoch/keeper/abci.go), [epoch.go](../modules/epoch/keeper/epoch.go), [grpc_query_epoch.go](../modules/epoch/keeper/grpc_query_epoch.go), [keeper.go](../modules/epoch/keeper/keeper.go), [params.go](../modules/epoch/keeper/params.go) |
| <a id="go-exported-modules-epoch-simulation"></a>`go-exported-modules-epoch-simulation` | [simap.go](../modules/epoch/simulation/simap.go) |
| <a id="go-exported-modules-epoch-types"></a>`go-exported-modules-epoch-types` | [codec.go](../modules/epoch/types/codec.go), [epoch.go](../modules/epoch/types/epoch.go), [errors.go](../modules/epoch/types/errors.go), [events.go](../modules/epoch/types/events.go), [expected_keepers.go](../modules/epoch/types/expected_keepers.go), [genesis.go](../modules/epoch/types/genesis.go), [hooks.go](../modules/epoch/types/hooks.go), [keys.go](../modules/epoch/types/keys.go), [params.go](../modules/epoch/types/params.go) |
| <a id="go-exported-modules-layerxexchange"></a>`go-exported-modules-layerxexchange` | [module.go](../modules/layerxexchange/module.go) |
| <a id="go-exported-modules-layerxexchange-keeper"></a>`go-exported-modules-layerxexchange-keeper` | [genesis.go](../modules/layerxexchange/keeper/genesis.go), [intents.go](../modules/layerxexchange/keeper/intents.go), [keeper.go](../modules/layerxexchange/keeper/keeper.go), [msg_server.go](../modules/layerxexchange/keeper/msg_server.go), [proofs.go](../modules/layerxexchange/keeper/proofs.go) |
| <a id="go-exported-modules-layerxexchange-types"></a>`go-exported-modules-layerxexchange-types` | [codec.go](../modules/layerxexchange/types/codec.go), [errors.go](../modules/layerxexchange/types/errors.go), [expected_keepers.go](../modules/layerxexchange/types/expected_keepers.go), [genesis.go](../modules/layerxexchange/types/genesis.go), [ids.go](../modules/layerxexchange/types/ids.go), [keys.go](../modules/layerxexchange/types/keys.go), [msgs.go](../modules/layerxexchange/types/msgs.go), [params.go](../modules/layerxexchange/types/params.go), [state.go](../modules/layerxexchange/types/state.go) |
| <a id="go-exported-modules-mint"></a>`go-exported-modules-mint` | [module.go](../modules/mint/module.go) |
| <a id="go-exported-modules-mint-client-cli"></a>`go-exported-modules-mint-client-cli` | [query.go](../modules/mint/client/cli/query.go), [tx.go](../modules/mint/client/cli/tx.go) |
| <a id="go-exported-modules-mint-client-rest"></a>`go-exported-modules-mint-client-rest` | [rest.go](../modules/mint/client/rest/rest.go) |
| <a id="go-exported-modules-mint-keeper"></a>`go-exported-modules-mint-keeper` | [genesis.go](../modules/mint/keeper/genesis.go), [grpc_query.go](../modules/mint/keeper/grpc_query.go), [hooks.go](../modules/mint/keeper/hooks.go), [keeper.go](../modules/mint/keeper/keeper.go), [migrations.go](../modules/mint/keeper/migrations.go) |
| <a id="go-exported-modules-mint-simulation"></a>`go-exported-modules-mint-simulation` | [decoder.go](../modules/mint/simulation/decoder.go), [genesis.go](../modules/mint/simulation/genesis.go) |
| <a id="go-exported-modules-mint-types"></a>`go-exported-modules-mint-types` | [codec.go](../modules/mint/types/codec.go), [expected_keepers.go](../modules/mint/types/expected_keepers.go), [genesis.go](../modules/mint/types/genesis.go), [gov.go](../modules/mint/types/gov.go), [hooks.go](../modules/mint/types/hooks.go), [keys.go](../modules/mint/types/keys.go), [minter.go](../modules/mint/types/minter.go), [params.go](../modules/mint/types/params.go) |
| <a id="go-exported-modules-oracle"></a>`go-exported-modules-oracle` | [abci.go](../modules/oracle/abci.go), [ante.go](../modules/oracle/ante.go), [module.go](../modules/oracle/module.go) |
| <a id="go-exported-modules-oracle-client-rest"></a>`go-exported-modules-oracle-client-rest` | [rest.go](../modules/oracle/client/rest/rest.go) |
| <a id="go-exported-modules-oracle-client-wasm"></a>`go-exported-modules-oracle-client-wasm` | [query.go](../modules/oracle/client/wasm/query.go) |
| <a id="go-exported-modules-oracle-keeper"></a>`go-exported-modules-oracle-keeper` | [alias_functions.go](../modules/oracle/keeper/alias_functions.go), [ballot.go](../modules/oracle/keeper/ballot.go), [keeper.go](../modules/oracle/keeper/keeper.go), [migrations.go](../modules/oracle/keeper/migrations.go), [msg_server.go](../modules/oracle/keeper/msg_server.go), [params.go](../modules/oracle/keeper/params.go), [querier.go](../modules/oracle/keeper/querier.go), [slash.go](../modules/oracle/keeper/slash.go) |
| <a id="go-exported-modules-oracle-simulation"></a>`go-exported-modules-oracle-simulation` | [decoder.go](../modules/oracle/simulation/decoder.go), [genesis.go](../modules/oracle/simulation/genesis.go), [operations.go](../modules/oracle/simulation/operations.go), [params.go](../modules/oracle/simulation/params.go) |
| <a id="go-exported-modules-oracle-types"></a>`go-exported-modules-oracle-types` | [ballot.go](../modules/oracle/types/ballot.go), [codec.go](../modules/oracle/types/codec.go), [denom.go](../modules/oracle/types/denom.go), [events.go](../modules/oracle/types/events.go), [expected_keeper.go](../modules/oracle/types/expected_keeper.go), [genesis.go](../modules/oracle/types/genesis.go), [keys.go](../modules/oracle/types/keys.go), [msgs.go](../modules/oracle/types/msgs.go), [params.go](../modules/oracle/types/params.go), [querier.go](../modules/oracle/types/querier.go), [snapshots.go](../modules/oracle/types/snapshots.go), [vote.go](../modules/oracle/types/vote.go) |
| <a id="go-exported-modules-oracle-utils"></a>`go-exported-modules-oracle-utils` | [assets.go](../modules/oracle/utils/assets.go), [period.go](../modules/oracle/utils/period.go) |
| <a id="go-exported-precompiles-layerxexchange"></a>`go-exported-precompiles-layerxexchange` | [layerxexchange.go](../precompiles/layerxexchange/layerxexchange.go), [setup.go](../precompiles/layerxexchange/setup.go) |
| <a id="go-exported-precompiles-spot-abi"></a>`go-exported-precompiles-spot-abi` | [abi.go](../precompiles/spot/abi/abi.go) |
| <a id="go-exported-precompiles-spot"></a>`go-exported-precompiles-spot` | [spot.go](../precompiles/spot/spot.go) |
| <a id="go-exported-modules-perps"></a>`go-exported-modules-perps` | [abci.go](../modules/perps/abci.go), [module.go](../modules/perps/module.go) |
| <a id="go-exported-modules-perps-keeper"></a>`go-exported-modules-perps-keeper` | [account.go](../modules/perps/keeper/account.go), [book.go](../modules/perps/keeper/book.go), [endblock.go](../modules/perps/keeper/endblock.go), [fees.go](../modules/perps/keeper/fees.go), [funding.go](../modules/perps/keeper/funding.go), [genesis.go](../modules/perps/keeper/genesis.go), [keeper.go](../modules/perps/keeper/keeper.go), [liquidation.go](../modules/perps/keeper/liquidation.go), [margin.go](../modules/perps/keeper/margin.go), [mark.go](../modules/perps/keeper/mark.go), [openinterest.go](../modules/perps/keeper/openinterest.go), [orders.go](../modules/perps/keeper/orders.go), [positions.go](../modules/perps/keeper/positions.go), [triggers.go](../modules/perps/keeper/triggers.go) |
| <a id="go-exported-modules-perps-types"></a>`go-exported-modules-perps-types` | [errors.go](../modules/perps/types/errors.go), [events.go](../modules/perps/types/events.go), [expected_keepers.go](../modules/perps/types/expected_keepers.go), [genesis.go](../modules/perps/types/genesis.go), [keys.go](../modules/perps/types/keys.go), [market.go](../modules/perps/types/market.go), [math.go](../modules/perps/types/math.go), [params.go](../modules/perps/types/params.go), [state.go](../modules/perps/types/state.go), [tiers.go](../modules/perps/types/tiers.go) |
| <a id="go-exported-precompiles-oracle"></a>`go-exported-precompiles-oracle` | [oracle.go](../precompiles/oracle/oracle.go) |
| <a id="go-exported-precompiles-perps-abi"></a>`go-exported-precompiles-perps-abi` | [abi.go](../precompiles/perps/abi/abi.go), [pack.go](../precompiles/perps/abi/pack.go) |
| <a id="go-exported-precompiles-pricefeed-abi"></a>`go-exported-precompiles-pricefeed-abi` | [abi.go](../precompiles/pricefeed/abi/abi.go) |
| <a id="go-exported-modules-pricefeed-keeper"></a>`go-exported-modules-pricefeed-keeper` | [aggregate.go](../modules/pricefeed/keeper/aggregate.go), [genesis.go](../modules/pricefeed/keeper/genesis.go), [msg_server.go](../modules/pricefeed/keeper/msg_server.go), [observation.go](../modules/pricefeed/keeper/observation.go), [params.go](../modules/pricefeed/keeper/params.go), [registry.go](../modules/pricefeed/keeper/registry.go) |
| <a id="go-exported-modules-pricefeed"></a>`go-exported-modules-pricefeed` | [module.go](../modules/pricefeed/module.go) |
| <a id="go-exported-modules-pricefeed-types"></a>`go-exported-modules-pricefeed-types` | [ante.go](../modules/pricefeed/types/ante.go), [codec.go](../modules/pricefeed/types/codec.go), [errors.go](../modules/pricefeed/types/errors.go), [events.go](../modules/pricefeed/types/events.go), [expected_keepers.go](../modules/pricefeed/types/expected_keepers.go), [msgs.go](../modules/pricefeed/types/msgs.go), [params.go](../modules/pricefeed/types/params.go), [state.go](../modules/pricefeed/types/state.go) |
| <a id="go-exported-modules-spot"></a>`go-exported-modules-spot` | [abci.go](../modules/spot/abci.go), [module.go](../modules/spot/module.go) |
| <a id="go-exported-modules-spot-keeper"></a>`go-exported-modules-spot-keeper` | [custody.go](../modules/spot/keeper/custody.go), [execute.go](../modules/spot/keeper/execute.go), [fees.go](../modules/spot/keeper/fees.go), [genesis.go](../modules/spot/keeper/genesis.go), [init_v6120.go](../modules/spot/keeper/init_v6120.go), [invariants.go](../modules/spot/keeper/invariants.go), [keeper.go](../modules/spot/keeper/keeper.go), [msg_server.go](../modules/spot/keeper/msg_server.go), [orders.go](../modules/spot/keeper/orders.go), [pricing.go](../modules/spot/keeper/pricing.go), [quote.go](../modules/spot/keeper/quote.go), [shares.go](../modules/spot/keeper/shares.go), [stoporders.go](../modules/spot/keeper/stoporders.go), [swap.go](../modules/spot/keeper/swap.go), [twap.go](../modules/spot/keeper/twap.go), [vault.go](../modules/spot/keeper/vault.go) |
| <a id="go-exported-modules-spot-types"></a>`go-exported-modules-spot-types` | [errors.go](../modules/spot/types/errors.go), [genesis.go](../modules/spot/types/genesis.go), [keys.go](../modules/spot/types/keys.go), [msgs.go](../modules/spot/types/msgs.go), [orders.go](../modules/spot/types/orders.go), [params.go](../modules/spot/types/params.go) |
| <a id="msg-layerxexchange-setmarket"></a>`msg-layerxexchange-setmarket` | [tx.proto](../api/layerxexchange/tx.proto) |
| <a id="msg-layerxexchange-updateparams"></a>`msg-layerxexchange-updateparams` | [tx.proto](../api/layerxexchange/tx.proto) |
| <a id="msg-oracle-aggregateexchangeratevote"></a>`msg-oracle-aggregateexchangeratevote` | [tx.proto](../api/oracle/tx.proto) |
| <a id="msg-oracle-delegatefeedconsent"></a>`msg-oracle-delegatefeedconsent` | [tx.proto](../api/oracle/tx.proto) |
| <a id="msg-pricefeed-setparams"></a>`msg-pricefeed-setparams` | [tx.proto](../api/pricefeed/tx.proto) |
| <a id="msg-pricefeed-submitprices"></a>`msg-pricefeed-submitprices` | [tx.proto](../api/pricefeed/tx.proto) |
| <a id="msg-spot-syncreserves"></a>`msg-spot-syncreserves` | [tx.proto](../api/spot/tx.proto) |
| <a id="msg-spot-updateparams"></a>`msg-spot-updateparams` | [tx.proto](../api/spot/tx.proto) |
| <a id="proto-epoch-epoch"></a>`proto-epoch-epoch` | [epoch.proto](../api/epoch/epoch.proto) |
| <a id="proto-epoch-genesisstate"></a>`proto-epoch-genesisstate` | [genesis.proto](../api/epoch/genesis.proto) |
| <a id="proto-epoch-params"></a>`proto-epoch-params` | [params.proto](../api/epoch/params.proto) |
| <a id="proto-epoch-queryepochrequest"></a>`proto-epoch-queryepochrequest` | [query.proto](../api/epoch/query.proto) |
| <a id="proto-epoch-queryepochresponse"></a>`proto-epoch-queryepochresponse` | [query.proto](../api/epoch/query.proto) |
| <a id="proto-epoch-queryparamsrequest"></a>`proto-epoch-queryparamsrequest` | [query.proto](../api/epoch/query.proto) |
| <a id="proto-epoch-queryparamsresponse"></a>`proto-epoch-queryparamsresponse` | [query.proto](../api/epoch/query.proto) |
| <a id="proto-layerxexchange-eventdeleveraged"></a>`proto-layerxexchange-eventdeleveraged` | [events.proto](../api/layerxexchange/events.proto) |
| <a id="proto-layerxexchange-eventfundingsettled"></a>`proto-layerxexchange-eventfundingsettled` | [events.proto](../api/layerxexchange/events.proto) |
| <a id="proto-layerxexchange-eventliquidated"></a>`proto-layerxexchange-eventliquidated` | [events.proto](../api/layerxexchange/events.proto) |
| <a id="proto-layerxexchange-eventmargindeposited"></a>`proto-layerxexchange-eventmargindeposited` | [events.proto](../api/layerxexchange/events.proto) |
| <a id="proto-layerxexchange-eventmarginwithdrawalrequested"></a>`proto-layerxexchange-eventmarginwithdrawalrequested` | [events.proto](../api/layerxexchange/events.proto) |
| <a id="proto-layerxexchange-eventmarginwithdrawn"></a>`proto-layerxexchange-eventmarginwithdrawn` | [events.proto](../api/layerxexchange/events.proto) |
| <a id="proto-layerxexchange-eventoracleaccepted"></a>`proto-layerxexchange-eventoracleaccepted` | [events.proto](../api/layerxexchange/events.proto) |
| <a id="proto-layerxexchange-eventordercancelled"></a>`proto-layerxexchange-eventordercancelled` | [events.proto](../api/layerxexchange/events.proto) |
| <a id="proto-layerxexchange-eventordercancelrequested"></a>`proto-layerxexchange-eventordercancelrequested` | [events.proto](../api/layerxexchange/events.proto) |
| <a id="proto-layerxexchange-eventorderplaced"></a>`proto-layerxexchange-eventorderplaced` | [events.proto](../api/layerxexchange/events.proto) |
| <a id="proto-layerxexchange-eventperpsmarketcreated"></a>`proto-layerxexchange-eventperpsmarketcreated` | [events.proto](../api/layerxexchange/events.proto) |
| <a id="proto-layerxexchange-eventperpsmarkethalted"></a>`proto-layerxexchange-eventperpsmarkethalted` | [events.proto](../api/layerxexchange/events.proto) |
| <a id="proto-layerxexchange-eventpositionclosed"></a>`proto-layerxexchange-eventpositionclosed` | [events.proto](../api/layerxexchange/events.proto) |
| <a id="proto-layerxexchange-eventpositionincreased"></a>`proto-layerxexchange-eventpositionincreased` | [events.proto](../api/layerxexchange/events.proto) |
| <a id="proto-layerxexchange-eventpositionopened"></a>`proto-layerxexchange-eventpositionopened` | [events.proto](../api/layerxexchange/events.proto) |
| <a id="proto-layerxexchange-eventsettlementrequested"></a>`proto-layerxexchange-eventsettlementrequested` | [events.proto](../api/layerxexchange/events.proto) |
| <a id="proto-layerxexchange-eventspotmarketcreated"></a>`proto-layerxexchange-eventspotmarketcreated` | [events.proto](../api/layerxexchange/events.proto) |
| <a id="proto-layerxexchange-eventspotmarkethalted"></a>`proto-layerxexchange-eventspotmarkethalted` | [events.proto](../api/layerxexchange/events.proto) |
| <a id="proto-layerxexchange-eventspotmarketresumed"></a>`proto-layerxexchange-eventspotmarketresumed` | [events.proto](../api/layerxexchange/events.proto) |
| <a id="proto-layerxexchange-eventspotordercancelled"></a>`proto-layerxexchange-eventspotordercancelled` | [events.proto](../api/layerxexchange/events.proto) |
| <a id="proto-layerxexchange-eventspotorderplaced"></a>`proto-layerxexchange-eventspotorderplaced` | [events.proto](../api/layerxexchange/events.proto) |
| <a id="proto-layerxexchange-genesisstate"></a>`proto-layerxexchange-genesisstate` | [genesis.proto](../api/layerxexchange/genesis.proto) |
| <a id="proto-layerxexchange-intent"></a>`proto-layerxexchange-intent` | [state.proto](../api/layerxexchange/state.proto) |
| <a id="proto-layerxexchange-intentkind"></a>`proto-layerxexchange-intentkind` | [state.proto](../api/layerxexchange/state.proto) |
| <a id="proto-layerxexchange-intentstatus"></a>`proto-layerxexchange-intentstatus` | [state.proto](../api/layerxexchange/state.proto) |
| <a id="proto-layerxexchange-market"></a>`proto-layerxexchange-market` | [params.proto](../api/layerxexchange/params.proto) |
| <a id="proto-layerxexchange-msgsetmarket"></a>`proto-layerxexchange-msgsetmarket` | [tx.proto](../api/layerxexchange/tx.proto) |
| <a id="proto-layerxexchange-msgsetmarketresponse"></a>`proto-layerxexchange-msgsetmarketresponse` | [tx.proto](../api/layerxexchange/tx.proto) |
| <a id="proto-layerxexchange-msgupdateparams"></a>`proto-layerxexchange-msgupdateparams` | [tx.proto](../api/layerxexchange/tx.proto) |
| <a id="proto-layerxexchange-msgupdateparamsresponse"></a>`proto-layerxexchange-msgupdateparamsresponse` | [tx.proto](../api/layerxexchange/tx.proto) |
| <a id="proto-layerxexchange-ownernonce"></a>`proto-layerxexchange-ownernonce` | [state.proto](../api/layerxexchange/state.proto) |
| <a id="proto-layerxexchange-params"></a>`proto-layerxexchange-params` | [params.proto](../api/layerxexchange/params.proto) |
| <a id="proto-mint-genesisstate"></a>`proto-mint-genesisstate` | [genesis.proto](../api/mint/v1beta1/genesis.proto) |
| <a id="proto-mint-minter"></a>`proto-mint-minter` | [mint.proto](../api/mint/v1beta1/mint.proto) |
| <a id="proto-mint-params"></a>`proto-mint-params` | [mint.proto](../api/mint/v1beta1/mint.proto) |
| <a id="proto-mint-queryminterrequest"></a>`proto-mint-queryminterrequest` | [query.proto](../api/mint/v1beta1/query.proto) |
| <a id="proto-mint-queryminterresponse"></a>`proto-mint-queryminterresponse` | [query.proto](../api/mint/v1beta1/query.proto) |
| <a id="proto-mint-queryparamsrequest"></a>`proto-mint-queryparamsrequest` | [query.proto](../api/mint/v1beta1/query.proto) |
| <a id="proto-mint-queryparamsresponse"></a>`proto-mint-queryparamsresponse` | [query.proto](../api/mint/v1beta1/query.proto) |
| <a id="proto-mint-scheduledtokenrelease"></a>`proto-mint-scheduledtokenrelease` | [mint.proto](../api/mint/v1beta1/mint.proto) |
| <a id="proto-mint-updateminterproposal"></a>`proto-mint-updateminterproposal` | [gov.proto](../api/mint/v1beta1/gov.proto) |
| <a id="proto-mint-version2minter"></a>`proto-mint-version2minter` | [mint.proto](../api/mint/v1beta1/mint.proto) |
| <a id="proto-mint-version2params"></a>`proto-mint-version2params` | [mint.proto](../api/mint/v1beta1/mint.proto) |
| <a id="proto-mint-version2scheduledtokenrelease"></a>`proto-mint-version2scheduledtokenrelease` | [mint.proto](../api/mint/v1beta1/mint.proto) |
| <a id="proto-oracle-aggregateexchangeratevote"></a>`proto-oracle-aggregateexchangeratevote` | [oracle.proto](../api/oracle/oracle.proto) |
| <a id="proto-oracle-denom"></a>`proto-oracle-denom` | [oracle.proto](../api/oracle/oracle.proto) |
| <a id="proto-oracle-denomoracleexchangeratepair"></a>`proto-oracle-denomoracleexchangeratepair` | [query.proto](../api/oracle/query.proto) |
| <a id="proto-oracle-exchangeratetuple"></a>`proto-oracle-exchangeratetuple` | [oracle.proto](../api/oracle/oracle.proto) |
| <a id="proto-oracle-feederdelegation"></a>`proto-oracle-feederdelegation` | [genesis.proto](../api/oracle/genesis.proto) |
| <a id="proto-oracle-genesisstate"></a>`proto-oracle-genesisstate` | [genesis.proto](../api/oracle/genesis.proto) |
| <a id="proto-oracle-msgaggregateexchangeratevote"></a>`proto-oracle-msgaggregateexchangeratevote` | [tx.proto](../api/oracle/tx.proto) |
| <a id="proto-oracle-msgaggregateexchangeratevoteresponse"></a>`proto-oracle-msgaggregateexchangeratevoteresponse` | [tx.proto](../api/oracle/tx.proto) |
| <a id="proto-oracle-msgdelegatefeedconsent"></a>`proto-oracle-msgdelegatefeedconsent` | [tx.proto](../api/oracle/tx.proto) |
| <a id="proto-oracle-msgdelegatefeedconsentresponse"></a>`proto-oracle-msgdelegatefeedconsentresponse` | [tx.proto](../api/oracle/tx.proto) |
| <a id="proto-oracle-oracleexchangerate"></a>`proto-oracle-oracleexchangerate` | [oracle.proto](../api/oracle/oracle.proto) |
| <a id="proto-oracle-oracletwap"></a>`proto-oracle-oracletwap` | [oracle.proto](../api/oracle/oracle.proto) |
| <a id="proto-oracle-params"></a>`proto-oracle-params` | [oracle.proto](../api/oracle/oracle.proto) |
| <a id="proto-oracle-penaltycounter"></a>`proto-oracle-penaltycounter` | [genesis.proto](../api/oracle/genesis.proto) |
| <a id="proto-oracle-pricesnapshot"></a>`proto-oracle-pricesnapshot` | [oracle.proto](../api/oracle/oracle.proto) |
| <a id="proto-oracle-pricesnapshotitem"></a>`proto-oracle-pricesnapshotitem` | [oracle.proto](../api/oracle/oracle.proto) |
| <a id="proto-oracle-queryactivesrequest"></a>`proto-oracle-queryactivesrequest` | [query.proto](../api/oracle/query.proto) |
| <a id="proto-oracle-queryactivesresponse"></a>`proto-oracle-queryactivesresponse` | [query.proto](../api/oracle/query.proto) |
| <a id="proto-oracle-queryexchangeraterequest"></a>`proto-oracle-queryexchangeraterequest` | [query.proto](../api/oracle/query.proto) |
| <a id="proto-oracle-queryexchangerateresponse"></a>`proto-oracle-queryexchangerateresponse` | [query.proto](../api/oracle/query.proto) |
| <a id="proto-oracle-queryexchangeratesrequest"></a>`proto-oracle-queryexchangeratesrequest` | [query.proto](../api/oracle/query.proto) |
| <a id="proto-oracle-queryexchangeratesresponse"></a>`proto-oracle-queryexchangeratesresponse` | [query.proto](../api/oracle/query.proto) |
| <a id="proto-oracle-queryfeederdelegationrequest"></a>`proto-oracle-queryfeederdelegationrequest` | [query.proto](../api/oracle/query.proto) |
| <a id="proto-oracle-queryfeederdelegationresponse"></a>`proto-oracle-queryfeederdelegationresponse` | [query.proto](../api/oracle/query.proto) |
| <a id="proto-oracle-queryparamsrequest"></a>`proto-oracle-queryparamsrequest` | [query.proto](../api/oracle/query.proto) |
| <a id="proto-oracle-queryparamsresponse"></a>`proto-oracle-queryparamsresponse` | [query.proto](../api/oracle/query.proto) |
| <a id="proto-oracle-querypricesnapshothistoryrequest"></a>`proto-oracle-querypricesnapshothistoryrequest` | [query.proto](../api/oracle/query.proto) |
| <a id="proto-oracle-querypricesnapshothistoryresponse"></a>`proto-oracle-querypricesnapshothistoryresponse` | [query.proto](../api/oracle/query.proto) |
| <a id="proto-oracle-queryslashwindowrequest"></a>`proto-oracle-queryslashwindowrequest` | [query.proto](../api/oracle/query.proto) |
| <a id="proto-oracle-queryslashwindowresponse"></a>`proto-oracle-queryslashwindowresponse` | [query.proto](../api/oracle/query.proto) |
| <a id="proto-oracle-querytwapsrequest"></a>`proto-oracle-querytwapsrequest` | [query.proto](../api/oracle/query.proto) |
| <a id="proto-oracle-querytwapsresponse"></a>`proto-oracle-querytwapsresponse` | [query.proto](../api/oracle/query.proto) |
| <a id="proto-oracle-queryvotepenaltycounterrequest"></a>`proto-oracle-queryvotepenaltycounterrequest` | [query.proto](../api/oracle/query.proto) |
| <a id="proto-oracle-queryvotepenaltycounterresponse"></a>`proto-oracle-queryvotepenaltycounterresponse` | [query.proto](../api/oracle/query.proto) |
| <a id="proto-oracle-queryvotetargetsrequest"></a>`proto-oracle-queryvotetargetsrequest` | [query.proto](../api/oracle/query.proto) |
| <a id="proto-oracle-queryvotetargetsresponse"></a>`proto-oracle-queryvotetargetsresponse` | [query.proto](../api/oracle/query.proto) |
| <a id="proto-oracle-votepenaltycounter"></a>`proto-oracle-votepenaltycounter` | [oracle.proto](../api/oracle/oracle.proto) |
| <a id="proto-pricefeed-market"></a>`proto-pricefeed-market` | [tx.proto](../api/pricefeed/tx.proto) |
| <a id="proto-pricefeed-msgsetparams"></a>`proto-pricefeed-msgsetparams` | [tx.proto](../api/pricefeed/tx.proto) |
| <a id="proto-pricefeed-msgsetparamsresponse"></a>`proto-pricefeed-msgsetparamsresponse` | [tx.proto](../api/pricefeed/tx.proto) |
| <a id="proto-pricefeed-msgsubmitprices"></a>`proto-pricefeed-msgsubmitprices` | [tx.proto](../api/pricefeed/tx.proto) |
| <a id="proto-pricefeed-msgsubmitpricesresponse"></a>`proto-pricefeed-msgsubmitpricesresponse` | [tx.proto](../api/pricefeed/tx.proto) |
| <a id="proto-pricefeed-observation"></a>`proto-pricefeed-observation` | [tx.proto](../api/pricefeed/tx.proto) |
| <a id="proto-spot-msgsyncreserves"></a>`proto-spot-msgsyncreserves` | [tx.proto](../api/spot/tx.proto) |
| <a id="proto-spot-msgsyncreservesresponse"></a>`proto-spot-msgsyncreservesresponse` | [tx.proto](../api/spot/tx.proto) |
| <a id="proto-spot-msgupdateparams"></a>`proto-spot-msgupdateparams` | [tx.proto](../api/spot/tx.proto) |
| <a id="proto-spot-msgupdateparamsresponse"></a>`proto-spot-msgupdateparamsresponse` | [tx.proto](../api/spot/tx.proto) |
