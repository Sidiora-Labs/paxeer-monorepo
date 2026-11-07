<!--
order: 9
-->

# Client

The `staking` module is part of `paxd` on the merged Paxeer network. The kernel
participates in the same network and proposes blocks; a receipt is final when its
block is committed. This page describes the native Cosmos staking CLI, gRPC
service and REST gateway.

The public EVM JSON-RPC uses the root URL, for example
`https://api1.mainnet-beta.paxeer.network/`, with EVM chain ID `125` (`0x7d`).
The Cosmos services below require a node exposing the corresponding listeners.
The EVM URL is not a substitute for a Cosmos consensus RPC, gRPC or REST address.

The command definitions are in [query.go](../client/cli/query.go) and
[tx.go](../client/cli/tx.go). The service methods and HTTP routes are declared in
[query.proto](../../../proto/cosmos/staking/v1beta1/query.proto), and their
behavior is implemented in [grpc_query.go](../keeper/grpc_query.go).

## CLI

Use `paxd` with the home and keyring of your node. Paxeer account addresses start
with `pax1`, validator operator addresses with `paxvaloper1`, and consensus
addresses with `paxvalcons1`; these prefixes are configured in
[node/params/config.go](../../../../node/params/config.go). Obtain addresses from
the keyring and node responses instead of changing the prefix of an address from
another network: its Bech32 checksum must also match.

The examples use a local consensus RPC and select real operator addresses from
its validator response. Choose the source and destination appropriate to your
delegations; listing a validator does not imply that your account delegates to it.

```bash
PAX_KEY=operator
PAX_NODE=tcp://127.0.0.1:26657
PAX_DELEGATOR=$(paxd keys show "$PAX_KEY" --address)
PAX_VALIDATOR=$(paxd query staking validators --node "$PAX_NODE" --output json \
  | jq -er '.validators[0].operator_address')
PAX_SRC_VALIDATOR="$PAX_VALIDATOR"
PAX_DST_VALIDATOR=$(paxd query staking validators --node "$PAX_NODE" --output json \
  | jq -er '.validators[1].operator_address')
PAX_BOND_DENOM=$(paxd query staking params --node "$PAX_NODE" --output json \
  | jq -er '.bond_denom')
```

The destination selection requires at least two validators. A query for a specific
record also requires that record to exist. The configured base denomination is
`uhpx`, with `1000000uhpx = 1hpx`; read `bond_denom` before constructing a staking
transaction. Query results, balances and parameters come from your selected node
and height. The examples below do not claim a captured mainnet snapshot.

### Query

All commands below support `--node "$PAX_NODE" --output json`. Use `--height H`
for a retained application-state height. List commands expose pagination flags
such as `--limit` and `--page-key`; continue with the returned `pagination.next_key`
until it is empty. Keep the query filters and height unchanged between pages.

#### delegation

Query one delegator/validator pair. The CLI prints the `DelegationResponse`
directly: `delegation` contains addresses and decimal `shares`, and `balance`
contains the current coin `denom` and integer-string `amount`.

```bash
paxd query staking delegation "$PAX_DELEGATOR" "$PAX_VALIDATOR" \
  --node "$PAX_NODE" --output json
```

#### delegations

List the account's delegations. The result contains `delegation_responses` and
`pagination`.

```bash
paxd query staking delegations "$PAX_DELEGATOR" --limit 20 \
  --node "$PAX_NODE" --output json
```

#### delegations-to

List delegations to one validator, returning `delegation_responses` and
`pagination`.

```bash
paxd query staking delegations-to "$PAX_VALIDATOR" --limit 20 \
  --node "$PAX_NODE" --output json
```

#### historical-info

Query a retained historical header and validator set. The positional height must
be nonnegative; an absent or pruned record returns an error. The CLI prints the
`hist` object directly, with `header` and `valset`.

```bash
paxd query staking historical-info 1 --node "$PAX_NODE" --output json
```

#### params

Read all eight current staking parameters. The CLI prints the parameter object
itself, without the `params` wrapper used by gRPC and REST. See [Params](#params-1)
for the complete response schema and the distinction between defaults and state.

```bash
paxd query staking params --node "$PAX_NODE" --output json
```

#### pool

Read `not_bonded_tokens` and `bonded_tokens`, both integer strings in the bond
denomination. The CLI prints the pool object without a `pool` wrapper.

```bash
paxd query staking pool --node "$PAX_NODE" --output json
```

#### redelegation

Query a specific delegator/source/destination triplet. The keeper checks that the
triplet exists and returns matching `redelegation_responses` plus `pagination`.
A missing triplet is an error, not an empty successful result.

```bash
paxd query staking redelegation "$PAX_DELEGATOR" \
  "$PAX_SRC_VALIDATOR" "$PAX_DST_VALIDATOR" --node "$PAX_NODE" --output json
```

#### redelegations

List redelegations for one delegator. The result contains
`redelegation_responses` and `pagination`. Use [gRPC Redelegations](#redelegations-1)
or [REST Redelegations](#redelegations-2) to combine optional validator filters.

```bash
paxd query staking redelegations "$PAX_DELEGATOR" --limit 20 \
  --node "$PAX_NODE" --output json
```

#### redelegations-from

List outgoing redelegations from a source validator across delegators. This CLI
command supplies only `src_validator_addr` to the service.

```bash
paxd query staking redelegations-from "$PAX_SRC_VALIDATOR" --limit 20 \
  --node "$PAX_NODE" --output json
```

#### unbonding-delegation

Query one account/validator unbonding record. The CLI prints the unbonding object
directly, including its `entries` with `creation_height`, `completion_time`,
`initial_balance` and remaining `balance`.

```bash
paxd query staking unbonding-delegation "$PAX_DELEGATOR" "$PAX_VALIDATOR" \
  --node "$PAX_NODE" --output json
```

#### unbonding-delegations

List an account's unbonding records as `unbonding_responses` and `pagination`.

```bash
paxd query staking unbonding-delegations "$PAX_DELEGATOR" --limit 20 \
  --node "$PAX_NODE" --output json
```

#### unbonding-delegations-from

List unbonding records for a validator as `unbonding_responses` and `pagination`.

```bash
paxd query staking unbonding-delegations-from "$PAX_VALIDATOR" --limit 20 \
  --node "$PAX_NODE" --output json
```

#### validator

Query a validator by operator address. The CLI prints the validator object itself.
It includes `operator_address`, `consensus_pubkey`, `jailed`, `status`, `tokens`,
`delegator_shares`, `description`, `unbonding_height`, `unbonding_time`,
`commission` and `min_self_delegation`.

```bash
paxd query staking validator "$PAX_VALIDATOR" --node "$PAX_NODE" --output json
```

#### validators

List validators as `validators` and `pagination`. The CLI deliberately leaves the
service's `status` filter empty and queries every bond status. To filter by status,
use gRPC or REST with `BOND_STATUS_BONDED`, `BOND_STATUS_UNBONDING` or
`BOND_STATUS_UNBONDED`.

```bash
paxd query staking validators --limit 20 --node "$PAX_NODE" --output json
```

#### hex-address

Find a validator by its consensus address in hexadecimal form. The command scans
the requested validator page; it returns a `validator` wrapper when found. Use
pagination flags if the validator is outside that page. The hexadecimal consensus
address is distinct from an EVM account address.

```bash
paxd query staking hex-address "$(paxd tendermint show-address)" \
  --node "$PAX_NODE" --output json
```

### Transactions

The signer is selected with `--from`. Set `PAX_CHAIN_ID` to the Cosmos chain ID
from your node's genesis configuration; the EVM chain ID `125` does not specify
this CLI flag. The examples use an amount of `1000000` base units and leave
transaction confirmation enabled. Configure fees or gas prices for your node's
policy using the standard transaction flags.

```bash
: "${PAX_CHAIN_ID:?Set the Cosmos chain ID from the node genesis configuration}"
PAX_AMOUNT="1000000${PAX_BOND_DENOM}"
```

A successful broadcast must still be checked for transaction execution success
and inclusion. Voting-power and commission limits are enforced by the keeper.
[Upgrade concepts](../../upgrade/spec/01_concepts.md) describes how a scheduled
upgrade affects node processing at its activation height.

#### create-validator

Create a validator with an initial self-delegation. The consensus public key must
be the node's Protobuf JSON key, as emitted by `paxd tendermint show-validator`.
The account selected by `--from` becomes the operator. The example commission
values are explicit choices, not defaults or a mainnet validator snapshot; ensure
that the rate meets the current `min_commission_rate`.

```bash
paxd tx staking create-validator \
  --amount "$PAX_AMOUNT" \
  --pubkey "$(paxd tendermint show-validator)" \
  --moniker "my-validator" \
  --commission-rate "0.10" \
  --commission-max-rate "0.20" \
  --commission-max-change-rate "0.01" \
  --min-self-delegation "1" \
  --from "$PAX_KEY" --chain-id "$PAX_CHAIN_ID" --node "$PAX_NODE"
```

#### delegate

Delegate liquid coins to a validator. Above the enforcement threshold, delegation
can fail with staking code `41`, `ErrExceedMaxVotingPowerRatio`, if it would exceed
the validator voting-power ratio. See the [parameter fields](#params-1).

```bash
paxd tx staking delegate "$PAX_VALIDATOR" "$PAX_AMOUNT" \
  --from "$PAX_KEY" --chain-id "$PAX_CHAIN_ID" --node "$PAX_NODE"
```

#### edit-validator

Edit the signing operator's description, commission rate or minimum
self-delegation. The name flag is `--new-moniker`. Commission changes obey the
existing maximum rate, maximum daily change and 24-hour update interval; minimum
self-delegation cannot decrease.

```bash
paxd tx staking edit-validator --new-moniker "my-updated-validator" \
  --from "$PAX_KEY" --chain-id "$PAX_CHAIN_ID" --node "$PAX_NODE"
```

#### redelegate

Move delegated tokens from one validator to another, subject to the redelegation
entry and transitive-redelegation restrictions. Source and destination must differ.

```bash
paxd tx staking redelegate "$PAX_SRC_VALIDATOR" "$PAX_DST_VALIDATOR" "$PAX_AMOUNT" \
  --from "$PAX_KEY" --chain-id "$PAX_CHAIN_ID" --node "$PAX_NODE"
```

#### unbond

Begin undelegating a coin amount. Funds become available after the applicable
unbonding period and queue processing, subject to slashing. The command takes a
coin amount, not a decimal share count.

```bash
paxd tx staking unbond "$PAX_VALIDATOR" "$PAX_AMOUNT" \
  --from "$PAX_KEY" --chain-id "$PAX_CHAIN_ID" --node "$PAX_NODE"
```

## gRPC

The service name remains `cosmos.staking.v1beta1.Query`. It is the native module's
Protobuf namespace. Connect `grpcurl` to your node's gRPC listener, typically
`localhost:9090` for the default local configuration. These examples use
`-plaintext` for that local listener; use TLS when the server requires it.
List requests accept `pagination` with `key` (base64), `offset`, `limit`,
`count_total` and `reverse`. Do not combine a continuation key with an offset.

```bash
PAX_GRPC=localhost:9090
grpcurl -plaintext -d '{}' "$PAX_GRPC" cosmos.staking.v1beta1.Query/Params
```

The sections below specify request fields and response fields. Numeric coin
amounts, heights and shares follow their Protobuf JSON types; preserve integer
strings rather than converting large balances through floating-point numbers.

### Validators

Call `cosmos.staking.v1beta1.Query/Validators` with optional `status` and
`pagination`; the response has `validators` and `pagination`.

```bash
grpcurl -plaintext -d '{"status":"BOND_STATUS_BONDED","pagination":{"limit":"20"}}' \
  "$PAX_GRPC" cosmos.staking.v1beta1.Query/Validators
```

An empty status selects all validators. An unsupported status returns
`InvalidArgument` (gRPC code `3`).

### Validator

Call `cosmos.staking.v1beta1.Query/Validator` with `validator_addr`; the response
has `validator`. A valid operator address absent from state returns `NotFound`
(gRPC code `5`).

```bash
grpcurl -plaintext \
  -d "$(jq -nc --arg v "$PAX_VALIDATOR" '{validator_addr:$v}')" \
  "$PAX_GRPC" cosmos.staking.v1beta1.Query/Validator
```

### ValidatorDelegations

Call `cosmos.staking.v1beta1.Query/ValidatorDelegations` with `validator_addr`
and optional `pagination`. The response has `delegation_responses` and
`pagination`; each delegation response contains `delegation` and `balance`.

### ValidatorUnbondingDelegations

Call `cosmos.staking.v1beta1.Query/ValidatorUnbondingDelegations` with
`validator_addr` and optional `pagination`. The response has `unbonding_responses`
and `pagination`.

### Delegation

Call `cosmos.staking.v1beta1.Query/Delegation` with `delegator_addr` and
`validator_addr`. The response wraps the pair's delegation and balance under
`delegation_response`.

### UnbondingDelegation

Call `cosmos.staking.v1beta1.Query/UnbondingDelegation` with `delegator_addr` and
`validator_addr`. The response wraps the record under `unbond`.

### DelegatorDelegations

Call `cosmos.staking.v1beta1.Query/DelegatorDelegations` with `delegator_addr`
and optional `pagination`. The response has `delegation_responses` and
`pagination`.

### DelegatorUnbondingDelegations

Call `cosmos.staking.v1beta1.Query/DelegatorUnbondingDelegations` with
`delegator_addr` and optional `pagination`. The response has `unbonding_responses`
and `pagination`.

### Redelegations

Call `cosmos.staking.v1beta1.Query/Redelegations` with optional `delegator_addr`,
`src_validator_addr`, `dst_validator_addr` and `pagination`. Every supplied address
filter applies. A source-only request uses the source-validator index; a request
with all three addresses also requires that exact triplet to exist. Other filter
combinations return matching records, possibly an empty list.

```bash
grpcurl -plaintext \
  -d "$(jq -nc --arg d "$PAX_DELEGATOR" --arg s "$PAX_SRC_VALIDATOR" \
    '{delegator_addr:$d,src_validator_addr:$s,pagination:{limit:"20",count_total:true}}')" \
  "$PAX_GRPC" cosmos.staking.v1beta1.Query/Redelegations
```

To constrain the destination too, add `dst_validator_addr` with
`$PAX_DST_VALIDATOR`. To query all outgoing records for a source, omit
`delegator_addr` and `dst_validator_addr`. The response has
`redelegation_responses` and `pagination`. Each response contains `redelegation`
(addresses with a null nested `entries` field in REST JSON) and a separate top-level `entries`
list (each `redelegation_entry` and its current `balance`). The stored entries
are transported in that top-level list; they are not repeated inside
`redelegation.entries`.
A returned entry's `creation_height`, `completion_time`, `initial_balance` and
`shares_dst` describe the pending redelegation.

For filtered queries, `pagination.total` counts matches when total counting is
requested on the initial page. Pass `pagination.next_key` back as `pagination.key`
with the same filters to continue. The keeper enforces bounded scans and returns
an error if a scan exceeds its limit. Errors from the redelegation query helpers,
including a missing exact triplet, are wrapped as `Internal` (gRPC code `13`) by
the current producer.

### DelegatorValidators

Call `cosmos.staking.v1beta1.Query/DelegatorValidators` with `delegator_addr`
and optional `pagination`. The response has `validators` and `pagination`.

### DelegatorValidator

Call `cosmos.staking.v1beta1.Query/DelegatorValidator` with `delegator_addr`
and `validator_addr`. The response has `validator` for a validator to which the
account delegates.

### HistoricalInfo

Call `cosmos.staking.v1beta1.Query/HistoricalInfo` with `height`. The response has
`hist`, containing `header` and `valset`. Negative heights return
`InvalidArgument`; missing historical records return `NotFound`.

### Pool

Call `cosmos.staking.v1beta1.Query/Pool` with `{}`. The response has `pool` with
`not_bonded_tokens` and `bonded_tokens` in the current bond denomination.

### Params

Call `cosmos.staking.v1beta1.Query/Params` with `{}` to retrieve staking parameters.
All eight fields are declared in
[staking.proto](../../../proto/cosmos/staking/v1beta1/staking.proto).
The following is the complete REST parameter response shape populated with
[module defaults](../types/params.go), including the constants from
[sdk/types/staking.go](../../../types/staking.go). It is a source reference, not a
capture of current mainnet configuration. Governance and migrations can change
the stored values; use the query commands to capture the full current response.

<!-- repair-m690:params -->
```json
{
  "params": {
    "unbonding_time": "1814400s",
    "max_validators": 35,
    "max_entries": 7,
    "historical_entries": 10000,
    "bond_denom": "uhpx",
    "min_commission_rate": "0.050000000000000000",
    "max_voting_power_ratio": "0.200000000000000000",
    "max_voting_power_enforcement_threshold": "1000000"
  }
}
```

| Field | Meaning |
| --- | --- |
| `unbonding_time` | Duration used for unbonding. The module default is three weeks. |
| `max_validators` | Maximum number of bonded validators. |
| `max_entries` | Maximum pending entries for an unbonding pair or redelegation triplet. |
| `historical_entries` | Number of historical staking records retained. |
| `bond_denom` | Coin denomination accepted for staking. |
| `min_commission_rate` | Minimum commission rate validators may charge. |
| `max_voting_power_ratio` | Maximum validator share of voting power once enforcement applies. |
| `max_voting_power_enforcement_threshold` | Minimum total voting power at which the ratio is enforced, in consensus-power units. |

The delegation path computes power using `Keeper.PowerReduction`, checks the
post-delegation validator power against the total and enforces the ratio when the
total is positive and at least the threshold. The default reduction is
`1000000` base units per unit of consensus power; the threshold is not a raw coin
balance. A node's gRPC JSON formatting can differ from the REST presentation;
the Protobuf field types and queried values are authoritative.

## REST

The REST routes are registered by `AppModuleBasic.RegisterGRPCGatewayRoutes` in
[module.go](../module.go). Enable `[api].enable` in your node's `app.toml` to serve
them; the SDK default disables the API and sets its listener to
`tcp://0.0.0.0:1317`. The examples assume that API is enabled on a local node.
The separate `[grpc]` configuration controls the gRPC listener.

```bash
PAX_REST=http://localhost:1317
curl --fail-with-body --silent --show-error "$PAX_REST/cosmos/staking/v1beta1/params"
```

All routes below use `GET` and the `/cosmos/staking/v1beta1` prefix. Their response
fields match the corresponding [gRPC methods](#grpc). List routes accept query
parameters such as `pagination.limit`, `pagination.key` and
`pagination.count_total`. Use `curl --get --data-urlencode` for base64 continuation
keys and address filters.

### DelegatorDelegations

`/cosmos/staking/v1beta1/delegations/{delegator_addr}` returns
`delegation_responses` and `pagination`.

```bash
curl --fail-with-body --silent --show-error \
  "$PAX_REST/cosmos/staking/v1beta1/delegations/$PAX_DELEGATOR"
```

### Redelegations

`/cosmos/staking/v1beta1/delegators/{delegator_addr}/redelegations` takes the
delegator in the path and optional `src_validator_addr` and `dst_validator_addr`
in the query string. The response has `redelegation_responses` and `pagination`.
The filters and exact-triplet behavior are described under
[gRPC Redelegations](#redelegations-1).

```bash
curl --fail-with-body --silent --show-error --get \
  "$PAX_REST/cosmos/staking/v1beta1/delegators/$PAX_DELEGATOR/redelegations" \
  --data-urlencode "src_validator_addr=$PAX_SRC_VALIDATOR" \
  --data-urlencode 'pagination.limit=20' \
  --data-urlencode 'pagination.count_total=true'
```

Add `--data-urlencode "dst_validator_addr=$PAX_DST_VALIDATOR"` for an exact
triplet. For a source-only query across delegators, use the CLI
`redelegations-from` command or gRPC, whose request can omit the delegator.

### DelegatorUnbondingDelegations

`/cosmos/staking/v1beta1/delegators/{delegator_addr}/unbonding_delegations`
returns `unbonding_responses` and `pagination`.

### DelegatorValidators

`/cosmos/staking/v1beta1/delegators/{delegator_addr}/validators` returns
`validators` and `pagination`.

### DelegatorValidator

`/cosmos/staking/v1beta1/delegators/{delegator_addr}/validators/{validator_addr}`
returns `validator`.

### HistoricalInfo

`/cosmos/staking/v1beta1/historical_info/{height}` returns `hist`. The record must
still exist in staking state.

### Parameters

`/cosmos/staking/v1beta1/params` returns `params`, including commission and both
voting-power enforcement fields. See [Params](#params-1) for all fields.
To capture the producer's complete current response, use the unfiltered `curl`
command at the start of this section.

### Pool

`/cosmos/staking/v1beta1/pool` returns `pool` with `not_bonded_tokens` and
`bonded_tokens`.

### Validators

`/cosmos/staking/v1beta1/validators` returns `validators` and `pagination`.
Supply `status=BOND_STATUS_BONDED` to select bonded validators:

```bash
curl --fail-with-body --silent --show-error --get \
  "$PAX_REST/cosmos/staking/v1beta1/validators" \
  --data-urlencode 'status=BOND_STATUS_BONDED' \
  --data-urlencode 'pagination.limit=20'
```

### Validator

`/cosmos/staking/v1beta1/validators/{validator_addr}` returns `validator`.

### ValidatorDelegations

`/cosmos/staking/v1beta1/validators/{validator_addr}/delegations` returns
`delegation_responses` and `pagination`.

### Delegation

`/cosmos/staking/v1beta1/validators/{validator_addr}/delegations/{delegator_addr}`
returns `delegation_response` with `delegation` and `balance`.

### UnbondingDelegation

`/cosmos/staking/v1beta1/validators/{validator_addr}/delegations/{delegator_addr}/unbonding_delegation`
returns `unbond` with the pair's pending unbonding entries.

### ValidatorUnbondingDelegations

`/cosmos/staking/v1beta1/validators/{validator_addr}/unbonding_delegations`
returns `unbonding_responses` and `pagination`.

## Executable response examples

The Params JSON above and the following redelegation example are checked by
`TestRepairM_690_ClientExamples` against the production keeper, Protobuf query
router, REST gateway and CLI. They use a local committed fixture, not a mainnet
snapshot. The fixture stores all eight code-default parameters, delegates
`10000000uhpx`, and redelegates `5000000uhpx` at block 10 from an unbonding
source whose height-9 deadline is `2023-11-14T23:13:20Z`. Its second redelegation
to a different destination proves the destination filter excludes other records.
The addresses below are checksum-valid Paxeer addresses for that fixture;
replace them with addresses for your own account and validators when querying
another node.

```bash
PAX_EXAMPLE_DELEGATOR="pax1tfd95kj6tfd95kj6tfd95kj6tfd95kj6zc4ht8"
PAX_EXAMPLE_SRC="paxvaloper1xvenxvenxvenxvenxvenxvenxvenxventus4ep"
PAX_EXAMPLE_DST="paxvaloper1xs6rgdp5xs6rgdp5xs6rgdp5xs6rgdp5tvhs7a"
paxd query staking redelegation "$PAX_EXAMPLE_DELEGATOR" \
  "$PAX_EXAMPLE_SRC" "$PAX_EXAMPLE_DST" --node "$PAX_NODE" --output json
curl --fail-with-body --silent --show-error --get \
  "$PAX_REST/cosmos/staking/v1beta1/delegators/$PAX_EXAMPLE_DELEGATOR/redelegations" \
  --data-urlencode "src_validator_addr=$PAX_EXAMPLE_SRC" \
  --data-urlencode "dst_validator_addr=$PAX_EXAMPLE_DST" \
  --data-urlencode 'pagination.limit=1' \
  --data-urlencode 'pagination.count_total=true'
```

The equivalent request to `cosmos.staking.v1beta1.Query/Redelegations` is:

<!-- repair-m690:redelegations-request -->
```json
{
  "delegator_addr": "pax1tfd95kj6tfd95kj6tfd95kj6tfd95kj6zc4ht8",
  "src_validator_addr": "paxvaloper1xvenxvenxvenxvenxvenxvenxvenxventus4ep",
  "dst_validator_addr": "paxvaloper1xs6rgdp5xs6rgdp5xs6rgdp5xs6rgdp5tvhs7a",
  "pagination": {
    "limit": "1",
    "count_total": true
  }
}
```

The gateway returns the complete response below. Integer balances are strings,
and the creation height is a JSON number; `shares_dst` is a decimal string. The terminal
`next_key` is null, and `total` counts the single match after applying all three
address filters. The CLI also prints a null nested `entries` and numeric creation
height: the production `RedelegationResponse.MarshalJSON` method controls these
fields even within protobuf JSON. Its exact-triplet command does not accept
pagination flags, so it does not request a count and returns `total: "0"`.

<!-- repair-m690:redelegations -->
```json
{
  "redelegation_responses": [
    {
      "redelegation": {
        "delegator_address": "pax1tfd95kj6tfd95kj6tfd95kj6tfd95kj6zc4ht8",
        "validator_src_address": "paxvaloper1xvenxvenxvenxvenxvenxvenxvenxventus4ep",
        "validator_dst_address": "paxvaloper1xs6rgdp5xs6rgdp5xs6rgdp5xs6rgdp5tvhs7a",
        "entries": null
      },
      "entries": [
        {
          "redelegation_entry": {
            "creation_height": 9,
            "completion_time": "2023-11-14T23:13:20Z",
            "initial_balance": "5000000",
            "shares_dst": "5000000.000000000000000000"
          },
          "balance": "5000000"
        }
      ]
    }
  ],
  "pagination": {
    "next_key": null,
    "total": "1"
  }
}
```

The CLI response examples for the same keeper fixture are below.
`TestRepairM_690_ClientExamples` compares them field for field with the real
client output and records that output in its private verification log.
The Params CLI prints the object directly, without the REST
`params` envelope.

<!-- repair-m690:params-cli -->
```json
{
  "unbonding_time": "1814400s",
  "max_validators": 35,
  "max_entries": 7,
  "historical_entries": 10000,
  "bond_denom": "uhpx",
  "min_commission_rate": "0.050000000000000000",
  "max_voting_power_ratio": "0.200000000000000000",
  "max_voting_power_enforcement_threshold": "1000000"
}
```

<!-- repair-m690:redelegations-cli -->
```json
{
  "redelegation_responses": [
    {
      "redelegation": {
        "delegator_address": "pax1tfd95kj6tfd95kj6tfd95kj6tfd95kj6zc4ht8",
        "validator_src_address": "paxvaloper1xvenxvenxvenxvenxvenxvenxvenxventus4ep",
        "validator_dst_address": "paxvaloper1xs6rgdp5xs6rgdp5xs6rgdp5xs6rgdp5tvhs7a",
        "entries": null
      },
      "entries": [
        {
          "redelegation_entry": {
            "creation_height": 9,
            "completion_time": "2023-11-14T23:13:20Z",
            "initial_balance": "5000000",
            "shares_dst": "5000000.000000000000000000"
          },
          "balance": "5000000"
        }
      ]
    }
  ],
  "pagination": {
    "next_key": null,
    "total": "0"
  }
}
```
