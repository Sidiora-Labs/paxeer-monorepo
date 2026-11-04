<!--
order: 7
-->

# Client

Governance queries and transactions use `paxd` and the
`cosmos.gov.v1beta1` services implemented in this tree. Paxeer accounts use the
`pax` Bech32 prefix and the base denomination is `uhpx`, as configured in
[`node/params/config.go`](../../../../node/params/config.go).

Paxeer mainnet-beta is one BFT network whose kernel proposes blocks as part of
the network. A receipt is final when its block is committed. Its public EVM
JSON-RPC uses chain ID `125` (`0x7d`) and the root path of
`https://api1.mainnet-beta.paxeer.network/` through
`https://api14.mainnet-beta.paxeer.network/`, and
`https://api-mainnet-beta.paxeer.network/`. The Cosmos CLI, gRPC and REST examples
below require the corresponding node services; these public EVM URLs do not
establish that those services are exposed there.

## CLI

Use a node configured for the intended network and a local key, here named
`validator`. Address arguments in the examples come from that key rather than
an abbreviated, invalid Bech32 string:

```bash
ACCOUNT=$(paxd keys show validator --address)
paxd query gov --help
paxd tx gov --help
```

The command implementations are in [`client/cli/query.go`](../client/cli/query.go)
and [`client/cli/tx.go`](../client/cli/tx.go). Deposit amounts below illustrate the
constructor defaults; query live [parameters](06_params.md) before submitting a
transaction. Supply the node, chain ID, fees and keyring settings required by
your deployment through the CLI configuration or transaction flags.

### Query

#### deposit

Query one depositor's stored deposit for a proposal:

```bash
paxd query gov deposit 1 "$ACCOUNT"
```

#### deposits

Query the proposal's deposits, with pagination:

```bash
paxd query gov deposits 1 --limit 20
```

#### param

Query one parameter group. The selectors are exactly `voting`, `tallying` and
`deposit`:

```bash
paxd query gov param voting
paxd query gov param tallying
paxd query gov param deposit
```

#### params

Query all three groups. This CLI command makes three `Query/Params` requests and
combines their selected results. For the constructor defaults its legacy output
contains these fields:

```bash
paxd query gov params
```

```yaml
deposit_params:
  min_deposit:
  - denom: uhpx
    amount: "10000000"
  min_expedited_deposit:
  - denom: uhpx
    amount: "20000000"
  max_deposit_period: "172800000000000"
voting_params:
  voting_period: "172800000000000"
  expedited_voting_period: "86400000000000"
tally_params:
  quorum: "0.334000000000000000"
  expedited_quorum: "0.667000000000000000"
  threshold: "0.500000000000000000"
  expedited_threshold: "0.667000000000000000"
  veto_threshold: "0.334000000000000000"
```

Durations here are nanosecond strings. These values illustrate the parameter
shape; they are not a captured response from mainnet.

#### proposal

Query one proposal, including its content, status, tally, deposit, timestamps
and `is_expedited` flag:

```bash
paxd query gov proposal 1
```

#### proposals

List proposals, optionally filtering by status, voter or depositor:

```bash
paxd query gov proposals --status VotingPeriod --limit 20
paxd query gov proposals --voter "$ACCOUNT"
paxd query gov proposals --depositor "$ACCOUNT"
```

#### proposer

Find the proposer through indexed submission transactions:

```bash
paxd query gov proposer 1
```

This command uses transaction search, not a `Query/Proposer` gRPC method. It
requires the submission transaction to remain available in the node's index.

#### tally

Query the current or stored tally:

```bash
paxd query gov tally 1
```

`TallyResult` returns an empty tally during the deposit period, the stored final
tally for passed or rejected proposals, and a calculated tally otherwise.

#### vote

Query one account's vote:

```bash
paxd query gov vote 1 "$ACCOUNT"
```

Prefer the `options` list of weighted choices. The deprecated `option` field
only represents a single choice of weight one.

#### votes

Query votes with pagination:

```bash
paxd query gov votes 1 --limit 20
```

The `votes`, `deposit` and `deposits` commands use transaction-search paths when
the proposal is outside the deposit and voting periods. They require the
proposal record and retained transaction indexes. An individual `vote` query
can still return an error when its stored vote no longer exists. The gRPC
methods below query the store directly.

### Transactions

#### deposit

Add tokens to an active proposal's deposit:

```bash
paxd tx gov deposit 1 10000000uhpx --from validator
```

The relevant minimum is `min_deposit` or `min_expedited_deposit`, depending on the
proposal's expedited status.

#### submit-proposal

Submit a regular text proposal using flags:

```bash
paxd tx gov submit-proposal --title="Network discussion" \
  --description="Describe the proposed policy" --type=Text \
  --deposit=10000000uhpx --from validator
```

To request expedited treatment for a text proposal, put `is_expedited` in the
proposal file. For example, save this as `proposal.json`:

```json
{
  "title": "Expedited network discussion",
  "description": "Describe the urgent policy",
  "type": "Text",
  "is_expedited": true,
  "deposit": "20000000uhpx"
}
```

```bash
paxd tx gov submit-proposal --proposal=proposal.json --from validator
```

The text-proposal command does not register an `--is-expedited` flag in this
tree. The file form reads that field directly. Do not combine `--proposal` with
`--title`, `--description`, `--type` or `--deposit`: the parser rejects that
combination.

Expedited proposals use `min_expedited_deposit`, `expedited_voting_period`,
`expedited_quorum` and `expedited_threshold`. A failed expedited tally converts
the proposal to regular voting with the existing votes; see
[parameters](06_params.md#validation-and-use).

Proposal subcommands are supplied by their owning modules and registered in
[`node/app.go`](../../../../node/app.go). Useful examples are:

```bash
paxd tx gov submit-proposal param-change params.json --from validator
paxd tx gov submit-proposal community-pool-spend spend.json --from validator
paxd tx gov submit-proposal software-upgrade v2 --upgrade-height 1000000 \
  --title="Planned upgrade" --description="Describe the release" \
  --deposit=10000000uhpx --from validator
paxd tx gov submit-proposal cancel-software-upgrade \
  --title="Cancel planned upgrade" --description="Explain the cancellation" \
  --deposit=10000000uhpx --from validator
```

The upgrade height above is illustrative; choose a future height for the
intended network. The two upgrade subcommands register `--is-expedited`.

For `param-change`, a `params.json` file can update one field of a governance
parameter object:

```json
{
  "title": "Shorten expedited voting",
  "description": "Use a twelve-hour expedited voting period",
  "is_expedited": false,
  "changes": [
    {
      "subspace": "gov",
      "key": "votingparams",
      "value": {
        "expedited_voting_period": "43200000000000"
      }
    }
  ],
  "deposit": "10000000uhpx"
}
```

The `param-change` file parser accepts a JSON value and converts it to the
parameter change's encoded value. The handler loads the existing object and
validates the merged value when applying the proposal. It requires an exact
registered key; see [Subspace](../../params/spec/02_subspace.md).

For `community-pool-spend`, `spend.json` contains `title`, `description`,
`recipient`, `amount` and `deposit`. Use a complete `pax` account address for
`recipient` and coin strings such as `10000000uhpx` for both coin fields.

#### vote

Submit one of `yes`, `no`, `no_with_veto` or `abstain`:

```bash
paxd tx gov vote 1 yes --from validator
```

#### weighted-vote

Submit weighted choices whose positive weights sum to one:

```bash
paxd tx gov weighted-vote 1 yes=0.5,no=0.5 --from validator
```

## gRPC

The service is `cosmos.gov.v1beta1.Query`, declared in the local
[`query.proto`](../../../proto/cosmos/gov/v1beta1/query.proto) and implemented by
[`keeper/grpc_query.go`](../keeper/grpc_query.go). The examples assume an enabled
local gRPC listener at `localhost:9090`; configure `grpc.enable` and
`grpc.address` in the node's `app.toml` as appropriate.

### Proposal

```bash
grpcurl -plaintext -d '{"proposal_id":"1"}' \
  localhost:9090 cosmos.gov.v1beta1.Query/Proposal
```

### Proposals

```bash
grpcurl -plaintext -d '{"pagination":{"limit":"20"}}' \
  localhost:9090 cosmos.gov.v1beta1.Query/Proposals
```

The request also accepts `proposal_status`, `voter` and `depositor` filters.

### Vote

Replace `PAX_ACCOUNT` in the JSON with a complete `pax` account address:

```bash
grpcurl -plaintext -d '{"proposal_id":"1","voter":"PAX_ACCOUNT"}' \
  localhost:9090 cosmos.gov.v1beta1.Query/Vote
```

### Votes

```bash
grpcurl -plaintext -d '{"proposal_id":"1","pagination":{"limit":"20"}}' \
  localhost:9090 cosmos.gov.v1beta1.Query/Votes
```

### Params

`Params` selects one group per request, rather than aggregating all groups:

```bash
grpcurl -plaintext -d '{"params_type":"voting"}' \
  localhost:9090 cosmos.gov.v1beta1.Query/Params
grpcurl -plaintext -d '{"params_type":"deposit"}' \
  localhost:9090 cosmos.gov.v1beta1.Query/Params
grpcurl -plaintext -d '{"params_type":"tallying"}' \
  localhost:9090 cosmos.gov.v1beta1.Query/Params
```

Read only the selected group's fields. Other groups in the response may contain
zero values because the protobuf response embeds non-nullable parameter
objects. Those values do not describe the other groups' stored parameters.
Protobuf JSON durations use seconds strings, for example `"172800s"` and
`"86400s"` for the default voting periods. Generic protobuf tools render the
schema's `bytes` tally decimals as base64; the SDK-aware CLI renders decimal
strings. Use the schema and SDK types when decoding these responses.

### Deposit

```bash
grpcurl -plaintext -d '{"proposal_id":"1","depositor":"PAX_ACCOUNT"}' \
  localhost:9090 cosmos.gov.v1beta1.Query/Deposit
```

### deposits

```bash
grpcurl -plaintext -d '{"proposal_id":"1"}' \
  localhost:9090 cosmos.gov.v1beta1.Query/Deposits
```

### TallyResult

```bash
grpcurl -plaintext -d '{"proposal_id":"1"}' \
  localhost:9090 cosmos.gov.v1beta1.Query/TallyResult
```

Proposal IDs must be nonzero for the single-proposal methods. A missing proposal
in `Proposal` or `TallyResult` returns gRPC `NotFound`. A missing vote or deposit
in `Vote` or `Deposit` returns `InvalidArgument`. An unsupported parameter
selector also returns `InvalidArgument`.

## REST

The gRPC gateway exposes HTTP GET queries when the node's API service is
enabled. These examples assume `api.enable=true` and the local API listener at
`http://localhost:1317`, configured with `api.address` in `app.toml`. They are
Cosmos API routes on that listener.

### proposal

```bash
curl http://localhost:1317/cosmos/gov/v1beta1/proposals/1
```

### proposals

```bash
curl 'http://localhost:1317/cosmos/gov/v1beta1/proposals?pagination.limit=20'
```

Optional filters are `proposal_status`, `voter` and `depositor`.

### voter vote

```bash
curl "http://localhost:1317/cosmos/gov/v1beta1/proposals/1/votes/$ACCOUNT"
```

### votes

```bash
curl 'http://localhost:1317/cosmos/gov/v1beta1/proposals/1/votes?pagination.limit=20'
```

### params

```bash
curl http://localhost:1317/cosmos/gov/v1beta1/params/voting
curl http://localhost:1317/cosmos/gov/v1beta1/params/deposit
curl http://localhost:1317/cosmos/gov/v1beta1/params/tallying
```

As with gRPC, each request selects one group. In the SDK-aware gateway JSON,
`voting_params` includes `voting_period` and `expedited_voting_period`,
`deposit_params` includes both minimum deposits and `max_deposit_period`, and
`tally_params` includes regular and expedited quorum and approval thresholds
plus `veto_threshold`. Read the selected group and ignore the zero-valued
unselected groups. See [parameters](06_params.md) for names and encodings.

### deposits

Query one depositor:

```bash
curl "http://localhost:1317/cosmos/gov/v1beta1/proposals/1/deposits/$ACCOUNT"
```

### proposal deposits

```bash
curl 'http://localhost:1317/cosmos/gov/v1beta1/proposals/1/deposits?pagination.limit=20'
```

### tally

```bash
curl http://localhost:1317/cosmos/gov/v1beta1/proposals/1/tally
```

List responses contain a `pagination` object. Follow `next_key` with the
transport's pagination-key field when more results are available. Governance
vote and deposit records can be removed after tallying; the CLI's indexed
transaction fallback does not add historical records to these store queries.
