<!--
order: 6
-->

# Parameters

The governance keeper stores three parameter objects in the `gov` parameter
subspace. Their exact registered keys and Go types are defined in
[`types/params.go`](../types/params.go).

| Key | Type | Contents |
| --- | --- | --- |
| `depositparams` | `DepositParams` | Regular and expedited minimum deposits, and the maximum deposit period |
| `votingparams` | `VotingParams` | Regular and expedited voting periods |
| `tallyparams` | `TallyParams` | Regular and expedited quorum and approval thresholds, and the veto threshold |

## SubKeys

These are fields within the objects above, rather than separately registered
parameter-store keys. The values below are constructor defaults, not a claim
about current mainnet state. Query the live values with the
[governance client](07_client.md#params).

| Object | Field | Go type | Constructor default |
| --- | --- | --- | --- |
| `depositparams` | `min_deposit` | `sdk.Coins` | `10000000uhpx` |
| `depositparams` | `min_expedited_deposit` | `sdk.Coins` | `20000000uhpx` |
| `depositparams` | `max_deposit_period` | `time.Duration` | 48 hours |
| `votingparams` | `voting_period` | `time.Duration` | 48 hours |
| `votingparams` | `expedited_voting_period` | `time.Duration` | 24 hours |
| `tallyparams` | `quorum` | `sdk.Dec` | `0.334000000000000000` |
| `tallyparams` | `expedited_quorum` | `sdk.Dec` | `0.667000000000000000` |
| `tallyparams` | `threshold` | `sdk.Dec` | `0.500000000000000000` |
| `tallyparams` | `expedited_threshold` | `sdk.Dec` | `0.667000000000000000` |
| `tallyparams` | `veto_threshold` | `sdk.Dec` | `0.334000000000000000` |

Coins encode denominations and integer amounts as strings. Parameter-change JSON
and the CLI's legacy parameter output encode durations as nanosecond strings:
48 hours is `"172800000000000"`, and 24 hours is `"86400000000000"`.
Protobuf JSON uses duration strings such as `"172800s"` instead. The field is
`veto_threshold`, not `veto`.

## Validation and use

[`types/params.go`](../types/params.go) validates each complete parameter object.
Deposit coin sets must be valid, the expedited minimum must not be entirely less
than or equal to the regular minimum, and the deposit period must be positive.
Both voting periods must be positive, with the expedited period strictly shorter.
Quorums lie in `[0, 1]`; approval and veto thresholds lie in `(0, 1]`.
The expedited quorum and approval threshold must each exceed their regular
counterpart.

The proposal's `is_expedited` value selects its minimum deposit, voting period,
quorum and approval threshold. In [`keeper/tally.go`](../keeper/tally.go), turnout
must meet the selected quorum; the Yes share of non-abstaining voting power must
strictly exceed the selected approval threshold. The veto share of total voting
power must strictly exceed `veto_threshold` to veto. The veto threshold is shared
by regular and expedited proposals. An expedited proposal that fails its tally
is converted to a regular proposal by [`EndBlocker`](../abci.go), retaining its
votes and extending its voting end time to the original voting start plus the
regular voting period.

## Parameter changes

A parameter-change proposal identifies subspace `gov` and one of the three object
keys. Its `value` is a JSON object, for example:

```json
{
  "subspace": "gov",
  "key": "votingparams",
  "value": {
    "expedited_voting_period": "43200000000000"
  }
}
```

[`Subspace.Update`](../../params/types/subspace.go) loads the stored object,
unmarshals the supplied fields into it, validates the resulting object, and
stores it. Omitted fields retain their previous values. A field name such as
`expedited_voting_period` cannot be used as the parameter-store key itself; see
[Subspace](../../params/spec/02_subspace.md#keytable) and the
[param-change client example](07_client.md#submit-proposal).
