<!--
order: 6
-->

# Events

The slashing module emits the following events/tags:

## MsgServer

### MsgUnjail

| Type    | Attribute Key | Attribute Value |
| ------- | ------------- | --------------- |
| message | module        | slashing        |
| message | sender        | {validatorAddress} |

## Keeper

### BeginBlocker: HandleValidatorSignatureConcurrent

| Type  | Attribute Key | Attribute Value             |
| ----- | ------------- | --------------------------- |
| slash | address       | {validatorConsensusAddress} |
| slash | power         | {validatorPower}            |
| slash | reason        | {slashReason}               |
| slash | jailed [0]    | {validatorConsensusAddress} |

- [0] Only included if the validator is jailed.

| Type     | Attribute Key | Attribute Value             |
| -------- | ------------- | --------------------------- |
| liveness | address       | {validatorConsensusAddress} |
| liveness | missed_blocks | {missedBlocksCounter}       |
| liveness | height        | {blockHeight}               |

### Slash

+ Same as the `slash` event from `SlashJailAndUpdateSigningInfo`, but without the `jailed` attribute.

### Jail

| Type  | Attribute Key | Attribute Value    |
| ----- | ------------- | ------------------ |
| slash | jailed        | {validatorAddress} |
