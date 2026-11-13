# Solidity API

Source: [MockWETH9.sol](../../../contracts/meta-ag/mocks/MockWETH9.sol).

## MockWETH9

Test implementation of `IWETH` with 18 decimals, the name `Mock Wrapped PAX` and symbol `mWPAX`.
Deposits mint wrapped tokens equal to the native amount received. Withdrawals burn wrapped tokens
and send the same native amount to the caller, emitting `Withdrawal`.

Used by `PECORVault` tests for `depositNative`, `withdrawNative` and the `WethOnly` receive guard.

### name

```solidity
string name
```

### symbol

```solidity
string symbol
```

### decimals

```solidity
uint8 decimals
```

### InsufficientBalance

```solidity
error InsufficientBalance()
```

### InsufficientAllowance

```solidity
error InsufficientAllowance()
```

### totalSupply

```solidity
function totalSupply() external view returns (uint256)
```

### balanceOf

```solidity
function balanceOf(address owner) external view returns (uint256)
```

### allowance

```solidity
function allowance(address owner, address spender) external view returns (uint256)
```

### deposit

```solidity
function deposit() external payable
```

### withdraw

```solidity
function withdraw(uint256 wad) external
```

### approve

```solidity
function approve(address spender, uint256 value) external returns (bool)
```

### transfer

```solidity
function transfer(address to, uint256 value) external returns (bool)
```

### transferFrom

```solidity
function transferFrom(address from, address to, uint256 value) external returns (bool)
```

### \_transfer

```solidity
function _transfer(address from, address to, uint256 value) internal returns (bool)
```

### receive

```solidity
receive() external payable
```

Receiving native coin credits the sender with the same amount of wrapped tokens and emits `Deposit`.
