<!--
order: 5
-->

# Client

The Paxeer node binary is `paxd`. Its authz commands are registered by
[the authz module](../module/module.go) and implemented in
[query.go](../client/cli/query.go) and [tx.go](../client/cli/tx.go).
Accounts use `pax1...` Bech32 addresses; validators use `paxvaloper1...`.
`uhpx` is the native base denomination. Protobuf type URLs retain their
`/cosmos.*` spelling.

These SDK commands and gRPC queries use the node's configured SDK services.
Paxeer mainnet-beta's public EVM JSON-RPC uses chain id 125 (`0x7d`) at the root of
`https://api1.mainnet-beta.paxeer.network` through
`https://api14.mainnet-beta.paxeer.network`, and
`https://api-mainnet-beta.paxeer.network`. Those addresses do not imply public
exposure of the SDK gRPC or REST services. Configure the CLI's node connection and
SDK chain-id string for your network; the EVM numeric chain id is not a substitute
for that string.

## CLI

The examples assume local keyring entries named `granter`, `grantee`, and
`recipient`. Use the same keyring and home options throughout if yours differ.
Set `CHAIN_ID` to your node's SDK chain-id string before the transaction examples.

```bash
GRANTER=$(paxd keys show granter -a)
GRANTEE=$(paxd keys show grantee -a)
RECIPIENT=$(paxd keys show recipient -a)
EXPIRATION=$(date -u -d '+1 year' +%s)
paxd query authz --help
paxd tx authz --help
```

### Query

#### grants

Query all grants for a granter and grantee, or select one message type:

```bash
paxd query authz grants "$GRANTER" "$GRANTEE"
paxd query authz grants "$GRANTER" "$GRANTEE" /cosmos.bank.v1beta1.MsgSend
```

#### grants-by-granter

```bash
paxd query authz grants-by-granter "$GRANTER"
```

The alias is `granter-grants`.

#### grants-by-grantee

```bash
paxd query authz grants-by-grantee "$GRANTEE"
```

The alias is `grantee-grants`. All three query commands accept pagination flags;
use their `--help` output for supported flags. Pair queries return `Grant` entries
with `authorization` and `expiration`; granter/grantee queries return
`GrantAuthorization` entries that also include both account addresses.

A query filtered by message type returns gRPC `NotFound` if the grant is missing
or expired. List queries do not filter expired entries. Querying does not commit
grant cleanup to chain state. See [Messages](03_messages.md#expiration-and-cleanup).

### Transactions

#### grant

`grant` takes exactly two positional arguments: the grantee and authorization type.
The granter is selected by `--from`.

```bash
paxd tx authz grant "$GRANTEE" send --spend-limit=100uhpx \
  --expiration="$EXPIRATION" --from=granter --chain-id="$CHAIN_ID"
```

The supported authorization types and additional flags are:

| Type | Flags and behavior |
| --- | --- |
| `send` | `--spend-limit` must contain positive coins, such as `100uhpx`. Authorizes `MsgSend`. |
| `generic` | `--msg-type` selects the exact registered message type URL. No authorization-specific spend limit. |
| `delegate` | Authorizes `MsgDelegate`; requires a validator policy. |
| `unbond` | Authorizes `MsgUndelegate`; requires a validator policy. |
| `redelegate` | Authorizes `MsgBeginRedelegate`; policy checks the destination validator. |

For each staking type, supply exactly one non-empty comma-separated
`--allowed-validators` or `--deny-validators` list of `paxvaloper1...` addresses.
An optional `--spend-limit` supplies the staking token limit; pass one denomination,
since this CLI uses the first parsed coin for `max_tokens`. See
[Concepts](01_concepts.md#stakeauthorization) for validator policy semantics.

`--expiration` is an integer Unix timestamp in seconds, not an RFC 3339 string.
Its default is one year from command construction. The grant handler does not
reject a past expiration. Reissuing a grant for the same pair and message type
replaces the prior grant.

For example, a generic grant for the bank send message is:

```bash
paxd tx authz grant "$GRANTEE" generic \
  --msg-type=/cosmos.bank.v1beta1.MsgSend --expiration="$EXPIRATION" \
  --from=granter --chain-id="$CHAIN_ID"
```

This replaces the earlier send grant for that same tuple with an unrestricted
generic authorization for `MsgSend`; the bank handler still enforces its rules.

#### exec

Generate an unsigned transaction whose message signer is the granter, then submit
its messages inside `MsgExec` signed by the grantee:

```bash
paxd tx bank send "$GRANTER" "$RECIPIENT" 10uhpx \
  --chain-id="$CHAIN_ID" --generate-only > tx.json
paxd tx authz exec tx.json --from=grantee --chain-id="$CHAIN_ID"
```

The file is a transaction JSON document, not a bare message array. `exec` reads its
messages and constructs a new transaction; it does not broadcast the file's
original signatures. This command rejects `--offline`. Authorization does not
bypass [bank message checks](../../bank/spec/03_messages.md).

#### revoke

```bash
paxd tx authz revoke "$GRANTEE" /cosmos.bank.v1beta1.MsgSend \
  --from=granter --chain-id="$CHAIN_ID"
```

Revocation fails if the specified grant does not exist.

## gRPC

The query contract is [query.proto](../../../proto/cosmos/authz/v1beta1/query.proto).
Use your node's configured `grpc.address` with `grpc.enable` enabled. The SDK
default listener is port 9090; the following plaintext example is for a local node:

### Grants

```bash
grpcurl -plaintext \
  -d "{\"granter\":\"$GRANTER\",\"grantee\":\"$GRANTEE\",\"msg_type_url\":\"/cosmos.bank.v1beta1.MsgSend\"}" \
  localhost:9090 cosmos.authz.v1beta1.Query/Grants
```

| Method | Request fields apart from optional `pagination` |
| --- | --- |
| `cosmos.authz.v1beta1.Query/Grants` | `granter`, `grantee`, optional `msg_type_url` |
| `cosmos.authz.v1beta1.Query/GranterGrants` | `granter` |
| `cosmos.authz.v1beta1.Query/GranteeGrants` | `grantee` |

A send authorization is encoded with type URL
`/cosmos.bank.v1beta1.SendAuthorization` and a `spend_limit` coin list (`spendLimit`
in protobuf JSON). Coin amounts are decimal strings; expiration is a timestamp.

## REST

The authz query service registers these gRPC gateway GET routes:

| Route | Parameters |
| --- | --- |
| `/cosmos/authz/v1beta1/grants` | Query: `granter`, `grantee`, optional `msg_type_url` |
| `/cosmos/authz/v1beta1/grants/granter/{granter}` | Granter in path |
| `/cosmos/authz/v1beta1/grants/grantee/{grantee}` | Grantee in path |

Enable the node's SDK API with `api.enable` and use `api.address`. The SDK default
API address is `tcp://0.0.0.0:1317`, with the API disabled by default. For a local
node where it is enabled:

```bash
curl --get 'http://localhost:1317/cosmos/authz/v1beta1/grants' \
  --data-urlencode "granter=$GRANTER" \
  --data-urlencode "grantee=$GRANTEE" \
  --data-urlencode 'msg_type_url=/cosmos.bank.v1beta1.MsgSend'
```

These are query routes. Submit grant, revoke, and execution transactions through
the SDK transaction facilities, such as the CLI commands above.
