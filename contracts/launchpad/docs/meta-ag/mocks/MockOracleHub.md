# Solidity API

## MockOracleHub

Configurable price and availability contract used by MetaAGRouter unit tests. It supplies the
`isPriceAvailable` and `getPrice` queries consumed by `MetaAGRouter._oracleSanityCheck`.

`getPrice` returns the stored price independently of availability. `isPriceAvailable` returns the
stored availability flag and confidence 5000 when available, or zero confidence otherwise.

Source: [MockOracleHub.sol](../../../contracts/meta-ag/mocks/MockOracleHub.sol).

### CONFIDENCE

```solidity
uint256 CONFIDENCE
```

Confidence returned by `isPriceAvailable` when the token is available: 5000.

### setPrice

```solidity
function setPrice(address token, uint256 price) external
```

Sets price and marks it available exactly when the price is nonzero. `setPrice(token, 0)` clears
both values; `setAvailable` overrides availability without changing the price.

### setAvailable

```solidity
function setAvailable(address token, bool flag) external
```

Override availability without changing the stored price.

### isPriceAvailable

```solidity
function isPriceAvailable(address token) external view returns (bool available, uint256 bestConfidence)
```

### getPrice

```solidity
function getPrice(address token) external view returns (uint256)
```
