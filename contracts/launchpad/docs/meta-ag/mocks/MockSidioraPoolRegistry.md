# Solidity API

Source: [MockSidioraPoolRegistry.sol](../../../contracts/meta-ag/mocks/MockSidioraPoolRegistry.sol).

## MockSidioraPoolRegistry

Configurable token-to-pool lookup used by `SidioraFeedAdapter` tests. Implements
`getPoolByToken(address)` and can be configured to revert on lookup.

### revertOnLookup

```solidity
bool revertOnLookup
```

### setPoolByToken

```solidity
function setPoolByToken(address token, address pool) external
```

### setRevertOnLookup

```solidity
function setRevertOnLookup(bool v) external
```

### getPoolByToken

```solidity
function getPoolByToken(address token) external view returns (address)
```
