# modules/mint

## Overview

The mint module creates `uhpx` according to governed token release schedules on the Paxeer network. It runs inside `paxd`, through the [epoch module](../epoch/README.md), and sends minted coins to the `fee_collector` module account for distribution with transaction fees. Mint state becomes final with the block that commits it.

The module stores one current minter and a list of scheduled releases. The default minter has zero issuance and the default schedule is empty. Once the current release is exhausted, minting resumes only when another scheduled release becomes eligible or governance replaces the minter.

### Minting Mechanism

[GetOrUpdateLatestMinter](keeper/keeper.go) keeps the current minter while its `remaining_mint_amount` is nonzero. Otherwise, it selects the first eligible scheduled release: the epoch start time must be strictly after the scheduled start date at UTC midnight, and that date must be on or after the current minter's end date. An exhausted release with the same start and end dates as the current minter is skipped.

A new minter takes its dates and total from that release, uses `mint_denom`, sets its remaining amount equal to the total, and initializes its last-mint metadata to zero with `last_mint_date: "0001-01-01"`.

### Daily Mint Calculation

[GetReleaseAmountToday](types/minter.go) uses the epoch's `current_epoch_start_time` in UTC:

- Before `start_date`, or when `last_mint_date` is already that UTC date, the release amount is zero.
- Before `end_date`, the amount is `remaining_mint_amount / days_left`, using integer division. `days_left` counts UTC calendar-day boundaries from the current date to the end date.
- On or after `end_date`, the amount is the entire remaining balance, provided nothing has already been minted on that date.

The calculation runs again at each epoch hook. A zero result does not record a mint. Rounding and missed days therefore affect later calculations; an outage does not trigger one separate mint for every missed day.

For example, with `500000uhpx` remaining and 49 days left, the next mint is `10204uhpx`, leaving `489796uhpx`. The following eligible day uses its new remaining amount and days left. Any balance still remaining on or after the end date is released at the next eligible hook.

### Minting Process

[AfterEpochEnd](keeper/hooks.go) calculates the release amount, mints it into the `mint` module account, and transfers it to `fee_collector`. Only after both bank operations succeed does it decrement the remaining amount, record the mint date, epoch height and amount, and persist the minter. A bank error panics rather than recording a successful mint.

### Updating the Minting Schedule

Governance provides two distinct updates: an `UpdateMinter` proposal replaces the complete current minter, while a parameter-change proposal updates `MintDenom` or `TokenReleaseSchedule` in the `mint` subspace. Updating the schedule does not replace a minter that still has a remaining balance.

Replacing the minter changes future issuance; it does not undo coins already minted. The handler does not derive remaining issuance from `total_mint_amount` or preserve omitted fields. An ongoing release must carry forward the intended remaining amount and last-mint metadata explicitly.

## State

### Minter

The [Minter type](types/mint.pb.go) is persisted under `MinterKey` (`0x00`) in the `mint` store. Dates use `YYYY-MM-DD` in UTC and amounts count base units of `uhpx`.

| JSON field | Go type | Meaning |
| --- | --- | --- |
| `start_date` | `string` | First date eligible for minting. |
| `end_date` | `string` | Date at which any remaining balance can be released. |
| `denom` | `string` | Minted denomination; must be `uhpx`. |
| `total_mint_amount` | `uint64` | Total issuance assigned to this release. |
| `remaining_mint_amount` | `uint64` | Balance still to mint. |
| `last_mint_amount` | `uint64` | Amount of the most recent successful mint. |
| `last_mint_date` | `string` | Date of that mint; `0001-01-01` for a fresh minter. |
| `last_mint_height` | `uint64` | Epoch's `current_epoch_height` recorded by that mint. |

[ValidateMinter](types/minter.go) requires valid start, end and last-mint dates, with the end on or after the start. Total and last-mint amounts must fit in a signed 64-bit integer; remaining must not exceed total; last-mint amount must not exceed `total_mint_amount - remaining_mint_amount`.

### Params

The [Params type](types/params.go) uses these parameter-store keys:

| JSON field | Parameter key | Meaning |
| --- | --- | --- |
| `mint_denom` | `MintDenom` | Must equal the default bond denomination, `uhpx`. |
| `token_release_schedule` | `TokenReleaseSchedule` | List of `ScheduledTokenRelease` values. |

Each scheduled release contains `start_date`, `end_date` and `token_release_amount`. Dates must parse as `YYYY-MM-DD`, the end must be on or after the start, and the amount must fit in a signed 64-bit integer. Validation sorts releases by start date and rejects overlapping periods; one release may start on the previous release's end date.

### Governance

#### Minter Governance Proposal

Inspect the current state before preparing a proposal:

```bash
paxd query mint minter
paxd query mint params
```

The node registers `update-minter` under `paxd tx gov submit-proposal` via [UpdateMinterHandler](client/cli/tx.go). The module does not expose a standalone `paxd tx mint` command. The proposal is validated by [ValidateBasic](types/gov.go) and, when executed, [HandleUpdateMinterProposal](gov.go) validates and stores the supplied minter unchanged.

For a **fresh release**, save the following complete payload as `minter_prop.json`. The dates and amounts are illustrative; choose them for the intended governance execution date. Protobuf JSON represents the `uint64` amounts and height as decimal strings.

```json
{
  "title": "Start a token release",
  "description": "Release 100000uhpx according to the specified dates",
  "minter": {
    "start_date": "2030-01-01",
    "end_date": "2030-01-11",
    "denom": "uhpx",
    "total_mint_amount": "100000",
    "remaining_mint_amount": "100000",
    "last_mint_amount": "0",
    "last_mint_date": "0001-01-01",
    "last_mint_height": "0"
  }
}
```

For a configured `paxd` client, set `PROPOSER` to a funded signing key, `DEPOSIT` to an appropriate governance deposit coin string, and `FEES` to a sufficient fee coin string, both denominated in `uhpx`. Submit with:

```bash
paxd tx gov submit-proposal update-minter ./minter_prop.json \
  --from "$PROPOSER" --deposit "$DEPOSIT" \
  --gas auto --gas-adjustment 1.3 --fees "$FEES" --broadcast-mode block
```

Submission does not execute the proposal. After it passes and executes, query `paxd query mint minter` again. Immediately after replacement, before any subsequent mint, its fields match the submitted object: the fresh example has `100000` remaining and `0001-01-01` as its last-mint date. Subsequent epoch hooks can change that state, including releasing the whole balance if the proposal executes after its end date.

For an **ongoing release**, preserve the issuance already made and the latest mint metadata. For example, if the queried state is:

```yaml
denom: uhpx
start_date: "2030-01-01"
end_date: "2030-01-11"
total_mint_amount: "100000"
remaining_mint_amount: "60000"
last_mint_amount: "10000"
last_mint_date: "2030-01-04"
last_mint_height: "12345"
```

then `40000uhpx` has already been issued. To raise the release total to `150000` and extend its end date, replace `minter_prop.json` with this complete proposal and use the same `update-minter` command:

```json
{
  "title": "Extend the ongoing token release",
  "description": "Add 50000uhpx while preserving the 40000uhpx already issued",
  "minter": {
    "start_date": "2030-01-01",
    "end_date": "2030-01-16",
    "denom": "uhpx",
    "total_mint_amount": "150000",
    "remaining_mint_amount": "110000",
    "last_mint_amount": "10000",
    "last_mint_date": "2030-01-04",
    "last_mint_height": "12345"
  }
}
```

Immediately after execution, before a further mint, that is the resulting state. Keeping the last-mint date prevents another mint on the same date. Governance stores a snapshot rather than recomputing it at execution: account for mints during the voting period when choosing the replacement values. Resetting an ongoing minter to fresh metadata can permit additional issuance.

Omitting `last_mint_date` supplies an empty date and fails validation. Omitting `remaining_mint_amount` supplies zero, so the proposed minter has no remaining issuance; setting only `total_mint_amount` does not initialize it.

### Params Governance Proposal

Parameter changes use the `mint` subspace and the exact keys `MintDenom` and `TokenReleaseSchedule`. For example, save this as `param_change_prop.json`, adjusting the dates, amounts and deposit to the intended update:

```json
{
  "title": "Update the mint release schedule",
  "description": "Set two future token releases",
  "deposit": "1000000uhpx",
  "changes": [
    {
      "subspace": "mint",
      "key": "MintDenom",
      "value": "uhpx"
    },
    {
      "subspace": "mint",
      "key": "TokenReleaseSchedule",
      "value": [
        {
          "start_date": "2030-02-01",
          "end_date": "2030-02-11",
          "token_release_amount": "500"
        },
        {
          "start_date": "2030-03-01",
          "end_date": "2030-03-11",
          "token_release_amount": "1000"
        }
      ]
    }
  ]
}
```

Unlike `update-minter`, the [param-change CLI](../../sdk/x/params/client/cli/tx.go) reads its initial deposit from the JSON file. Using the same configured client, signing key and fee settings:

```bash
paxd tx gov submit-proposal param-change ./param_change_prop.json \
  --from "$PROPOSER" --gas auto --gas-adjustment 1.3 \
  --fees "$FEES" --broadcast-mode block
```

This replaces the schedule parameter. It does not top up the current minter or restart an exhausted release with the same date pair. Query `paxd query mint params` after execution to inspect the stored schedule.

## Epoch hook

Epoch duration defaults to 60 seconds in [epoch genesis](../epoch/types/genesis.go). The mint hook uses the ending epoch's `current_epoch_start_time`, rather than the wall clock or the next epoch's start. For a normal ongoing release, only the first successful mint on a UTC date records that day's share; later hooks on the same date return zero. The remaining state and last-mint date persist across restarts.

### Minting events

#### Type: mint

[RecordSuccessfulMint](types/minter.go) emits the lowercase event type `mint` with these string attributes:

- `mint_date`: UTC date recorded for the mint.
- `mint_epoch`: epoch number, in decimal.
- `amount`: minted base-unit amount, in decimal without a denomination suffix.

### Metrics

Each successful mint records the telemetry gauge `pax_mint_coins{denom}` and the OpenTelemetry gauge `mint_coins_minted` with attribute `denom`. These gauges report the latest mint amount, rather than cumulative issuance. See [mint metrics](types/metrics.go) and the [telemetry helper](../../utils/metrics/metrics_util.go).
