<!-- Source: https://docs.paxeer.app/modules/mint/ -->

# Mint Module

Scheduled native token release: a fixed amount minted daily through the epoch hook, under governance-controlled schedules.

**Source:** `Paxeer-X-Network/modules/mint/`

## Overview

The mint module creates new native tokens according to a predefined schedule: a `total_mint_amount` released between a start date and an end date. There is no block-by-block inflation rate. Minting is driven only by the epoch module's `AfterEpochEnd` hook and releases at most once per UTC date.

## Minting Mechanism

On each epoch end, the hook (`modules/mint/keeper/hooks.go`) loads or updates the current minter from the release schedule, then computes today's release from the *ending epoch's start time* — not the current block time:

```
coinsToMint := latestMinter.GetReleaseAmountToday(epoch.CurrentEpochStartTime.UTC())
```

`GetReleaseAmountToday` (`modules/mint/types/minter.go`) applies these rules, in order:

1. Before the start date, or if a mint already happened on that UTC date (`last_mint_date` matches): release `0`
2. Past the end date, or with zero days left: release **all** of `remaining_mint_amount`
3. Otherwise: release `remaining_mint_amount / days_between(today, end_date)` (integer division)

If the amount is zero or nothing remains, the hook returns without minting. Otherwise it mints the coins (`MintCoins`), sends them to the `fee_collector` account (`AddCollectedFees`), and records the mint: `remaining_mint_amount` decreases and `last_mint_amount`, `last_mint_date`, and `last_mint_height` are updated. The distribution module then handles collected fees as usual.

### Example

If `total_mint_amount = 1,000,000` tokens and the period is 100 days:

- Initial daily mint: `1,000,000 / 100 = 10,000` tokens/day

- After 50 days: `500,000 remaining / 50 days = 10,000` tokens/day (unchanged if no downtime)

- If the chain was down for 1 day at day 50: `500,000 remaining / 49 days = 10,204` tokens/day (adjusted)

- If the chain is still down past the end date, the first epoch end after recovery releases the entire remainder at once (rule 2)

## State

### Minter

The minter stores current release state. Dates use the `yyyy-mm-dd` format:

```
type Minter struct {
    StartDate           string  // The day where the mint begins
    EndDate             string  // The day where the mint ends
    Denom               string  // Must equal the bond denom (uhpx)
    TotalMintAmount     uint64  // Total amount to be minted
    RemainingMintAmount uint64  // Remaining amount to be minted
    LastMintAmount      uint64  // Last amount minted (usually from the day before)
    LastMintDate        string  // Last day minted
    LastMintHeight      uint64  // The height of the last mint
}
```

The denom is not a free choice: validation requires both the minter denom and `Params.MintDenom` to equal the default bond denom, `uhpx`.

Query the current minter and parameters:

```
paxd q mint minter
paxd q mint params
```

### Params

Parameters define the release schedules the minter is derived from:

```
type Params struct {
    MintDenom            string                   // Type of coin to mint (must be uhpx)
    TokenReleaseSchedule []ScheduledTokenRelease  // List of token release schedules
}

type ScheduledTokenRelease struct {
    StartDate          string  // The day where the mint begins
    EndDate            string  // The day where the mint ends
    TokenReleaseAmount uint64  // Total amount to be minted
}
```

Multiple release schedules can be defined, allowing sequential minting periods.

## Epoch Hook

The mint module has no begin-block minting logic of its own. Its `BeforeEpochStart` hook is a no-op; all minting happens in `AfterEpochEnd` as described above. Because epochs default to 60s, the hook runs frequently, but the same-date check limits actual mints to one per UTC date. See the [Epoch module](https://docs.paxeer.app/modules/epoch) for how boundaries are detected.

## Governance

### Update Minter Proposal

The minter can be replaced via governance to change the mint schedule. The proposal is stored *verbatim* after validation (`HandleUpdateMinterProposal` calls `ValidateMinter` then `SetMinter`), so every field must be set explicitly. In particular:

- `remaining_mint_amount` is not derived from `total_mint_amount`. Setting it to `0` means no ongoing release and no further mints

- `last_mint_date` must be a parseable `yyyy-mm-dd` date; an empty string fails validation. A fresh minter uses `0001-01-01`

- `denom` must be `uhpx`, and `last_mint_amount` must not exceed `total_mint_amount - remaining_mint_amount`

```
{
  "title": "Update Minter",
  "description": "Adjust mint schedule",
  "minter": {
    "start_date": "2023-10-05",
    "end_date": "2023-11-22",
    "denom": "uhpx",
    "total_mint_amount": "100000",
    "remaining_mint_amount": "100000",
    "last_mint_amount": "0",
    "last_mint_date": "0001-01-01",
    "last_mint_height": "0"
  }
}
```

Submit the proposal:

```
paxd tx gov submit-proposal update-minter ./minter_prop.json \
  --deposit 20pax --from admin -b block -y \
  --gas 200000 --fees 2000uhpx
```

After the proposal passes, `paxd q mint minter` returns exactly the stored minter above until the next daily release updates it.

### Update Params Proposal

Parameters can be updated to define new token release schedules:

```
{
  "title": "Param Change Proposal",
  "description": "Update token release schedule",
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
          "token_release_amount": 500,
          "start_date": "2023-10-01",
          "end_date": "2023-10-30"
        },
        {
          "token_release_amount": 1000,
          "start_date": "2023-11-01",
          "end_date": "2023-11-30"
        }
      ]
    }
  ]
}
```

Submit:

```
paxd tx gov submit-proposal param-change ./param_change_prop.json \
  --from admin -b block -y --gas 200000 --fees 200000uhpx
```

**Note:** Changes to `total_mint_amount` or `remaining_mint_amount` after the start date do not affect tokens already minted.

## Events

### Mint Event

Emitted on each successful mint:

- **mint_date:** Date of the mint

- **mint_epoch:** Epoch number when the mint occurred

- **amount:** Amount minted

## Metrics

Each successful mint records two gauges: the OpenTelemetry gauge `mint_coins_minted{denom}` and the legacy telemetry gauge `pax/mint/coins` (exported as `pax_mint_coins{denom}`).

## Next Steps

- [Understand epoch coordination](https://docs.paxeer.app/modules/epoch)

- [Learn about the oracle module](https://docs.paxeer.app/modules/oracle)
