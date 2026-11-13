# Solidity API

Source: [TransferHelperWrapper.sol](../../contracts/test/TransferHelperWrapper.sol).

## TransferHelperWrapper

Wrapper to expose TransferHelper library functions for testing

Each method delegates to [TransferHelper](../../contracts/libraries/TransferHelper.sol).
The token must have code, its call must succeed, and its return data must be
empty or exactly 32 bytes encoding the integer one. Other results revert with
`TransferFailed()`, `TransferFromFailed()` or `ApproveFailed()`, respectively.
These methods return no value.

### safeTransfer

```solidity
function safeTransfer(address token, address to, uint256 value) external
```

Transfers tokens held by this wrapper.

### safeTransferFrom

```solidity
function safeTransferFrom(address token, address from, address to, uint256 value) external
```

Calls the token with this wrapper as the spender; `from` must authorize it
according to the token's allowance rules.

### safeApprove

```solidity
function safeApprove(address token, address spender, uint256 value) external
```

Sets the spender's allowance over tokens held by this wrapper.
