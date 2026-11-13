# Solidity API

Source: [MockOpticalRegistry.sol](../../contracts/test/MockOpticalRegistry.sol).

## MockOpticalRegistry

Test helper with configurable optical approval returned by `isApproved(address)`.

### setApproved

```solidity
function setApproved(address optical, bool approved) external
```

### isApproved

```solidity
function isApproved(address optical) external view returns (bool)
```
