<!-- Source: https://docs.paxeer.app/modules/tokenfactory/ -->

# Token Factory Module

Creation and admin management of native token denominations with creator-namespaced denoms.

**Source:** `modules/tokenfactory/`

## Overview

The tokenfactory module allows any account to create new native tokens: `MsgCreateDenom` has no permission gate, allow list of creators, or governance check. Tokens are namespaced by creator address as `factory/{creator address}/{subdenom}`, eliminating name collisions. A single account can create multiple denoms by providing unique subdenoms.

The creator becomes the denom's admin. The admin can mint, burn, change admin, update the denom allow list, set bank metadata, and pause or unpause the denom — subject to the sender restrictions below.

## Token Naming

Tokenfactory denoms follow the format:

```
factory/{creator_address}/{subdenom}
```

Example:

```
factory/{creator_address}/ufoo
```

- **Prefix:** Always `factory` (7 bytes)

- **Creator address:** Bech32 address (max 75 bytes)

- **Subdenom:** User-defined (max 44 bytes)

The full denom must also pass the SDK denom validation, `[a-zA-Z][a-zA-Z0-9/-]{2,127}` (`sdk/types/coin.go`). That means a subdenom containing `.` is rejected, while `-` and `/` are allowed characters in the full denom. Total denom length must not exceed 128 bytes.

## Messages

### CreateDenom

Create a new denom. Field 3 carries an optional bank allow list for the new denom:

```
message MsgCreateDenom {
    string sender = 1;
    string subdenom = 2;
    AllowList allow_list = 3;  // bank AllowList type
}
```

CLI:

```
paxd tx tokenfactory create-denom ufoo --from mylocalwallet
```

State modifications:

- Set `DenomMetadata` via bank keeper

- Set `AuthorityMetadata` with sender as admin

- Add denom to `CreatorPrefixStore`

Replace angle-bracketed arguments with your denom or file path. Creation fails if the subdenom already has bank supply or the resulting denom already has bank metadata.

### Mint

Mint tokens (admin only). The minted coins go to the sender — there is no recipient field:

```
message MsgMint {
    string sender = 1;
    cosmos.base.v1beta1.Coin amount = 2;
}
```

CLI:

```
paxd tx tokenfactory mint 100000000000<factory-denom> --from mylocalwallet
```

Checks: the denom was created via tokenfactory, the sender is the admin, and the denom is not paused. The amount is minted via the bank module to the sender's account; distributing it further is a separate bank send.

### Burn

Burn tokens (admin only). The coins are burned from the sender's own balance — the admin cannot burn from another account:

```
message MsgBurn {
    string sender = 1;
    cosmos.base.v1beta1.Coin amount = 2;
}
```

Checks: the denom was created via tokenfactory, the sender is the admin, the denom is not paused, and the sender holds the amount. The amount is sent from the sender to the module account and burned via the bank module.

### ChangeAdmin

Transfer admin privileges:

```
message MsgChangeAdmin {
    string sender = 1;
    string denom = 2;
    string newAdmin = 3;
}
```

The sender must be the current admin, and `newAdmin` must differ from the current admin. `ValidateBasic` accepts either a valid bech32 address or `""` to renounce admin authority. Renouncing leaves no admin able to perform these operations.

### UpdateDenom

Replace the denom's bank allow list:

```
message MsgUpdateDenom {
    string sender = 1;
    string denom = 2;
    AllowList allow_list = 3;  // bank AllowList type
}
```

```
paxd tx tokenfactory update-denom <denom> --allow-list <allow-list-file> --from mylocalwallet
```

The allow-list file is a JSON object with an addresses array; an empty array clears the list. The stored allow list must be defined, contain only valid bech32 addresses, and not exceed the `denom_allowlist_max_size` parameter (default `2000`).

### SetDenomMetadata

Set the denom's bank metadata from a file:

```
paxd tx tokenfactory set-denom-metadata <metadata-file> --from mylocalwallet
```

### SetDenomPaused

Pause or unpause a denom:

```
paxd tx tokenfactory set-denom-paused <denom> true --from mylocalwallet
```

While a denom is paused, tokenfactory mint and burn reject with `ErrDenomPaused`. This CLI message applies to denoms without an issuer DID; issuer-bound denoms use the issuer authorization path.

## Admin Capabilities

The admin can:

- **Mint:** Create new tokens of the denom, credited to the admin itself

- **Burn:** Destroy tokens from the admin's own balance

- **Change Admin:** Transfer admin rights to another address

- **Update allow list / metadata / pause:** Via `MsgUpdateDenom`, `MsgSetDenomMetadata`, and `MsgSetDenomPaused`

There is no force-transfer message in this module: the admin cannot move or burn tokens held by other accounts. Ordinary transfers use the bank module and require the holder's signature. Admins can share privileges via the authz module without changing the master admin.

## Queries

```
paxd query tokenfactory params
paxd query tokenfactory denom-authority-metadata <denom>
paxd query tokenfactory denom-paused <denom>
paxd query tokenfactory denoms-from-creator <creator-address>
paxd query tokenfactory denom-metadata <denom>
paxd query tokenfactory denom-allow-list <denom>
paxd query bank denom-metadata --denom <factory-denom>
```

## Parameters

- **`denom_allowlist_max_size`:** Maximum addresses in a denom allow list. Default `2000`; changeable by governance

## Restrictions

To fit within the 128-byte Cosmos SDK denom limit:

- **Max subdenom length:** 44 bytes

- **Max creator address length:** 75 bytes (bech32 prefix ≤ 16 chars)

Calculation:

```
len("factory") + 2*len("/") + len(creator_address) + len(subdenom) ≤ 128
7 + 2 + 75 + 44 = 128
```

## Use Cases

- **Wrapped assets:** Mint tokens representing off-chain assets

- **Synthetic tokens:** Create tokens backed by other on-chain assets

- **Protocol-managed denoms:** Modules create denoms for internal accounting

- **Governance tokens:** DAOs issue native governance tokens

## Integration with Bank Module

Tokenfactory denoms are native tokens, not ERC-20 contracts. They integrate with the bank module for:

- Balance queries

- Transfers via `MsgSend`

- IBC transfers

- Distribution to stakers

## Next Steps

- [Review current precompile addresses and behavior](https://docs.paxeer.app/precompiles)

- [Review other Paxeer modules](https://docs.paxeer.app/modules)
