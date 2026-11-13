# Solidity API

Source: [MockPoolRegistry.sol](../../contracts/test/MockPoolRegistry.sol).

## MockPoolRegistry

Test helper with configurable pool registration returned by `isRegisteredPool(address)`.

### setRegistered

```solidity
function setRegistered(address pool, bool registered) external
```

### isRegisteredPool

```solidity
function isRegisteredPool(address pool) external view returns (bool)
```
