# Solidity API

## FeeLib

Dynamic fee calculation for Sidiora pools

_feeBps = baseFee + ageFactor + volatilityFactor + concentrationFactor Clamped to [minFeeBps,
maxFeeBps]_

Source: [FeeLib.sol](../../contracts/libraries/FeeLib.sol).

### calculateDynamicFee

```solidity
function calculateDynamicFee(uint256 baseFee, uint256 minFee, uint256 maxFee, uint256 feeDecayRate, uint256 volatilityWeight, uint256 concentrationWeight, uint256 poolAgeSeconds, uint256 volatility, uint256 topHolderBps) internal pure returns (uint256 feeBps)
```

Calculates the dynamic fee for a swap

Requires `minFee <= maxFee` and `maxFee < 10000`. The calculation saturates at maxFee as components
are added and raises a smaller result to minFee.

#### Parameters

| Name                | Type    | Description                                             |
| ------------------- | ------- | ------------------------------------------------------- |
| baseFee             | uint256 | Base fee in basis points                                |
| minFee              | uint256 | Minimum fee in basis points                             |
| maxFee              | uint256 | Maximum fee in basis points                             |
| feeDecayRate        | uint256 | Controls how fast age factor decays                     |
| volatilityWeight    | uint256 | Weight of volatility component                          |
| concentrationWeight | uint256 | Weight of concentration component                       |
| poolAgeSeconds      | uint256 | Age of pool in seconds                                  |
| volatility          | uint256 | Root mean square relative price movement, scaled by 1e6 |
| topHolderBps        | uint256 | Top holder's percentage in basis points (0-10000)       |

#### Return Values

| Name   | Type    | Description                        |
| ------ | ------- | ---------------------------------- |
| feeBps | uint256 | The calculated fee in basis points |

### calculateAgeFactor

```solidity
function calculateAgeFactor(uint256 feeDecayRate, uint256 poolAgeSeconds) internal pure returns (uint256 factor)
```

Age factor: higher fee for younger pools, decays over time

_ageFactor = feeDecayRate / (1 + poolAgeInHours)_, where poolAgeInHours is poolAgeSeconds divided
by 3600 with integer truncation.

#### Parameters

| Name           | Type    | Description                   |
| -------------- | ------- | ----------------------------- |
| feeDecayRate   | uint256 | Decay rate parameter (in bps) |
| poolAgeSeconds | uint256 | Pool age in seconds           |

#### Return Values

| Name   | Type    | Description                             |
| ------ | ------- | --------------------------------------- |
| factor | uint256 | Age-based fee component in basis points |

### calculateVolatilityFactor

```solidity
function calculateVolatilityFactor(uint256 volatilityWeight, uint256 volatility) internal pure returns (uint256 factor)
```

Volatility factor: higher fee when price swings are large

_volatilityFactor = volatilityWeight \* volatility / 1e6_

#### Parameters

| Name             | Type    | Description                                             |
| ---------------- | ------- | ------------------------------------------------------- |
| volatilityWeight | uint256 | Weight parameter                                        |
| volatility       | uint256 | Root mean square relative price movement, scaled by 1e6 |

#### Return Values

| Name   | Type    | Description                                    |
| ------ | ------- | ---------------------------------------------- |
| factor | uint256 | Volatility-based fee component in basis points |

### calculateConcentrationFactor

```solidity
function calculateConcentrationFactor(uint256 concentrationWeight, uint256 topHolderBps) internal pure returns (uint256 factor)
```

Concentration factor: increases with the top holder's share

_concentrationFactor = concentrationWeight \* topHolderBps / 10000_

#### Parameters

| Name                | Type    | Description                        |
| ------------------- | ------- | ---------------------------------- |
| concentrationWeight | uint256 | Weight parameter                   |
| topHolderBps        | uint256 | Top holder's share in basis points |

#### Return Values

| Name   | Type    | Description                                       |
| ------ | ------- | ------------------------------------------------- |
| factor | uint256 | Concentration-based fee component in basis points |

### calculateVolatility

```solidity
function calculateVolatility(uint256[8] snapshots, uint256 count) internal pure returns (uint256 volatility)
```

Calculates price volatility from a snapshot buffer

Computes the square root of the mean squared relative movements between chronological observations,
scaled by `1e6`. Each absolute movement is capped at 100%. Counts above eight revert; fewer than two
observations return zero.

#### Parameters

| Name      | Type       | Description                        |
| --------- | ---------- | ---------------------------------- |
| snapshots | uint256[8] | Array of price snapshots (up to 8) |
| count     | uint256    | Number of valid snapshots          |

#### Return Values

| Name       | Type    | Description                                             |
| ---------- | ------- | ------------------------------------------------------- |
| volatility | uint256 | Root mean square relative price movement, scaled by 1e6 |
