# Solidity API

Source: [TokenAddressWrapper.sol](../../contracts/test/TokenAddressWrapper.sol).

## TokenAddressWrapper

Wrapper to expose TokenAddress library functions for testing

Both methods delegate to [TokenAddress](../../contracts/libraries/TokenAddress.sol)
and compute values without deploying a token or changing state.

### computeAddress

```solidity
function computeAddress(address factory, address creator, string name, string symbol, uint256 nonce, bytes creationCode, uint256 totalSupply, address recipient) external pure returns (address)
```

Returns the CREATE2 address for `factory`, using the computed salt and the hash
of `creationCode` followed by ABI-encoded `name`, `symbol`, `totalSupply` and
`recipient` constructor arguments.

### computeSalt

```solidity
function computeSalt(address creator, string name, string symbol, uint256 nonce) external pure returns (bytes32)
```

Returns `keccak256(abi.encodePacked(creator, name, symbol, nonce))`.
