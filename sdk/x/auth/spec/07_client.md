<!--
order: 7
-->

# Client

Use `paxd` for SDK account queries and vesting transactions. Account addresses
use the `pax` Bech32 prefix. In the examples, `ACCOUNT` and `RECIPIENT` stand for
valid full Paxeer addresses, and `SENDER` identifies a local signing key. Query
commands require a configured SDK node connection.

The public EVM JSON-RPC is available at the root of
`https://api1.mainnet-beta.paxeer.network` through
`https://api14.mainnet-beta.paxeer.network` and
`https://api-mainnet-beta.paxeer.network`. These nodes serve EVM chain ID `125`
(`0x7d`). SDK CLI, gRPC and REST queries below use the node's corresponding SDK
services; do not assume the public EVM endpoint also exposes them. A local node
must enable gRPC or the REST API before those interfaces can be used.

## Auth

Auth queries return identity and signing metadata. Current coin balances are
queried through `x/bank`, as explained in [State](02_state.md).

### CLI

The [auth CLI registration](../client/cli/query.go) provides these queries.

#### Query

```bash
paxd query auth --help
```

#### account

```bash
paxd query auth account "$ACCOUNT" --output json
```

This prints the account `Any` itself, including its concrete `@type`, rather
than a balance. A base account contains `address`, an optional public key,
`account_number` and `sequence`. A module or vesting account has a different
concrete type and nested fields; clients must decode its type rather than assume
every result is a `BaseAccount`.

#### accounts

```bash
paxd query auth accounts --limit 20 --output json
```

The result contains `accounts` and `pagination`. Use the returned `next_key` with
the CLI pagination options to continue; the first page need not contain every
account. Module accounts may appear alongside user and vesting accounts. Account
numbers and sequences are encoded as strings in protobuf JSON.

#### params

```bash
paxd query auth params --output json
```

This command prints the params object directly. The code defaults are:

```json
{
  "max_memo_characters": "256",
  "tx_sig_limit": "7",
  "tx_size_cost_per_byte": "10",
  "sig_verify_cost_ed25519": "590",
  "sig_verify_cost_secp256k1": "1000",
  "disable_seqno_check": false
}
```

Zero/default fields may be omitted by some serializers. These are defaults, not
an observation of current network settings. See [Parameters](07_params.md) for
all stored keys and the effect of `disable_seqno_check`.

### gRPC

The [query protobuf](../../../proto/cosmos/auth/v1beta1/query.proto) defines the
`cosmos.auth.v1beta1.Query` service. The following examples use a local gRPC
listener on `localhost:9090`; adjust the address and TLS options for your node.

#### Account

```bash
grpcurl -plaintext -d "{\"address\":\"$ACCOUNT\"}" \
  localhost:9090 cosmos.auth.v1beta1.Query/Account
```

The response is `{"account": <typed account>}`. For a base account, the typed
object has `@type: /cosmos.auth.v1beta1.BaseAccount` and its identity fields.
Standard gRPC protobuf JSON uses names such as `pubKey` and `accountNumber`;
SDK CLI JSON uses the protobuf field spellings shown elsewhere on this page.
The [query implementation](../keeper/grpc_query.go) returns gRPC `NotFound` for
an absent account, `InvalidArgument` for an empty address, and an address parsing
error for invalid Bech32 input.

#### Accounts

```bash
grpcurl -plaintext -d '{"pagination":{"limit":"20"}}' \
  localhost:9090 cosmos.auth.v1beta1.Query/Accounts
```

The response contains a list of typed `accounts` and `pagination`. Its
`pagination.nextKey`, when nonempty, is a base64 page key for the next request.

#### Params

```bash
grpcurl -plaintext -d '{}' \
  localhost:9090 cosmos.auth.v1beta1.Query/Params
```

Unlike the CLI's direct params output, the response has a `params` wrapper:

```json
{
  "params": {
    "maxMemoCharacters": "256",
    "txSigLimit": "7",
    "txSizeCostPerByte": "10",
    "sigVerifyCostEd25519": "590",
    "sigVerifyCostSecp256k1": "1000",
    "disableSeqnoCheck": false
  }
}
```

This illustrates code defaults; default-valued fields may be omitted.

### REST

Use an enabled SDK REST/gRPC-gateway listener, represented here by `REST_URL`.
The generated bindings in [`query.pb.gw.go`](../types/query.pb.gw.go) expose:

#### Account

```text
GET /cosmos/auth/v1beta1/accounts/{address}
```

```bash
curl "$REST_URL/cosmos/auth/v1beta1/accounts/$ACCOUNT"
```

The address is a path component. The response contains `account`, a typed
protobuf account without a bank balance.

#### Accounts

```text
GET /cosmos/auth/v1beta1/accounts
```

```bash
curl "$REST_URL/cosmos/auth/v1beta1/accounts?pagination.limit=20"
```

The response contains `accounts` and `pagination`. Continue with the returned
page key, URL-encoding it when placing it in `pagination.key`.

#### Params

```text
GET /cosmos/auth/v1beta1/params
```

```bash
curl "$REST_URL/cosmos/auth/v1beta1/params"
```

The response contains a `params` object. For current coin balances, use the
[bank query routes](../../../proto/cosmos/bank/v1beta1/query.proto), including
`GET /cosmos/bank/v1beta1/balances/{address}` and
`GET /cosmos/bank/v1beta1/spendable_balances/{address}`. Auth account metadata,
total bank coins and spendable bank coins answer different questions.

## Vesting

### CLI

The [vesting CLI](../vesting/client/cli/tx.go) registers one transaction
subcommand:

```bash
paxd tx vesting --help
```

#### Transactions

##### create-vesting-account

```text
paxd tx vesting create-vesting-account [to_address] [amount] [end_time] [flags]
```

Set `RECIPIENT` to an address without an existing auth account, `SENDER` to the
funding key, `SDK_CHAIN_ID` to the node's configured SDK chain ID, and `END_TIME`
to the desired absolute Unix timestamp in seconds. Use an end time after the
creation block for a future release schedule. For example, with those values set:

```bash
paxd tx vesting create-vesting-account "$RECIPIENT" 1000000uhpx "$END_TIME" \
  --from "$SENDER" --chain-id "$SDK_CHAIN_ID" --gas auto \
  --gas-adjustment 1.3 --gas-prices 0.02uhpx
```

This creates a continuous vesting account starting at the creation block's time.
Add `--delayed` to release the full allocation only at `END_TIME`. The example
amount is one `hpx` (`1000000uhpx`); gas prices are illustrative and must satisfy
the current global and node-local requirements described in
[Concepts](01_concepts.md#gas--fees). The sender must cover the allocation and fees.

For delayed creation with an administrator, use:

```bash
paxd tx vesting create-vesting-account "$RECIPIENT" 1000000uhpx "$END_TIME" \
  --delayed --admin "$ADMIN" --from "$SENDER" --chain-id "$SDK_CHAIN_ID" \
  --gas auto --gas-adjustment 1.3 --gas-prices 0.02uhpx
```

The optional `--admin "$ADMIN"` flag stores an administrator, where `ADMIN` is a
valid address with an existing auth account. This records metadata; the current
module has no cancellation message or administrator cancellation implementation.
Account validation requires the reserved `cancelled_time` field to remain zero.
The transaction cannot convert an already-existing recipient account into a
vesting account. Bank send-enabled, blocked-address and denomination allowlist
checks also apply.

There is no registered `create-periodic-vesting-account` subcommand or periodic
creation message in this tree. Periodic and permanently locked accounts are
supported account representations for genesis/import, not additional commands
of this live creation service. Query an existing vesting account through the auth
account query and its spendable coins through bank. See
[Vesting](05_vesting.md) for schedules, delegation and export/import details.
