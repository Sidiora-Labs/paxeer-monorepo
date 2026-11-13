# Solidity API

Source: [PoolAddressWrapper.sol](../../contracts/test/PoolAddressWrapper.sol).

## PoolAddressWrapper

Wrapper to expose PoolAddress library functions for testing

### computeAddress

```solidity
function computeAddress(address factory, address beacon, address token, bytes memory creationCode, bytes memory initData) external pure returns (address)
```

### computeSalt

```solidity
function computeSalt(address token) external pure returns (bytes32)
```
