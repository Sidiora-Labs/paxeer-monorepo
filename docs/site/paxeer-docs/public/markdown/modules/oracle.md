<!-- Source: https://docs.paxeer.app/modules/oracle/ -->

# Oracle Module

Navigate the retired EVM Oracle API and the native interfaces for exchange rates, voting parameters, and price observations.

**Source:** `Paxeer-X-Network/precompiles/oracle/oracle.go`, `modules/oracle/`, `node/wasmbinding/queries.go`, and `node/app.go`.

## EVM price queries are retired

The Oracle precompile remains registered at `0x0000000000000000000000000000000000001008`, but both `getExchangeRates` and `getOracleTwaps` revert with:

```
oracle precompile is retired; oracle data queries are disabled
```

For EVM applications, choose an active price-data integration for your network and configure its asset identifiers, units, and freshness checks.

## Native Oracle interfaces

The native Oracle module handles validator voting, aggregation, and queries. It also provides the keeper used by CosmWasm Oracle query plugins.

| Component | Function |
| --- | --- |
| `modules/oracle/handler.go` | Feed-consent delegation and aggregate exchange-rate vote messages. |
| `modules/oracle/abci.go` | `MidBlocker`: vote-period ballot aggregation, exchange-rate updates, and slash-window penalty processing. |
| `modules/oracle/keeper/` | Stored rates, vote targets, validator voting state, parameters, and query services. |
| `modules/oracle/client/cli/query.go` | Native exchange-rate, actives, snapshot-history, TWAP, parameter, feeder, and vote-penalty queries. |
| `node/wasmbinding/queries.go` | `HandleOracleQuery`: native `exchange_rates` and `oracle_twaps` custom-query handlers. |

Query current vote targets and observations to identify the denominations available on your network. Check observation timestamps before using a rate in application logic.

## Aggregation and parameters

Aggregation runs in the module's mid-blocker (`MidBlock` in `modules/oracle/module.go`), not in begin- or end-block. At the last block of each vote period it builds voting claims from bonded validators, organizes ballots by configured vote targets, applies threshold checks, and derives exchange rates through weighted-median processing (`WeightedMedianWithAssertion` for the reference denom, `Tally` with the reward band for cross rates). At the last block of each slash window it runs `SlashAndResetCounters` over the miss/abstain counters. Vote-period, reward-band, and slash-window behavior is controlled by module parameters.

Read the live parameters and vote targets before interpreting a network's observations or validator obligations. Do not infer a fixed voting period, reward band, slash rate, or required asset list from an example or another network's settings.

## Native query commands

These eight commands are registered by the native module. Point the CLI at the Cosmos RPC endpoint for the node you are inspecting:

```
paxd query oracle params
paxd query oracle vote-targets
paxd query oracle actives
paxd query oracle exchange-rates
paxd query oracle exchange-rates "$DENOM"
paxd query oracle price-snapshot-history
paxd query oracle twaps "$LOOKBACK_SECONDS"
paxd query oracle feeder "$VALIDATOR_ADDRESS"
paxd query oracle vote-penalty-counter "$VALIDATOR_ADDRESS"
```

The optional denomination belongs to `exchange-rates`; there is no separate singular `exchange-rate` command in this CLI. Validator operator addresses use the `paxvaloper` prefix.

## CosmWasm query boundary

The `oracle` custom-query route still calls the native module handler. A CosmWasm custom querier payload for retained exchange rates has this shape:

```
{
  "route": "oracle",
  "query_data": { "exchange_rates": {} }
}
```

Issue this query from a CosmWasm contract through [WASM bindings](https://docs.paxeer.app/wasmbinding). Check the returned denominations and observation freshness before using the results.

## Related interfaces

- [Current precompile addresses and ABI behavior](https://docs.paxeer.app/precompiles)

- [Native REST and gRPC interfaces](https://docs.paxeer.app/rest-grpc)

- [Custom CosmWasm queries and messages](https://docs.paxeer.app/wasmbinding)
