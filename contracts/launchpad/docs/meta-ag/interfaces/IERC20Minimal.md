# Solidity API

## IERC20Minimal

Minimal ERC20 interface for the Meta-AG contracts. It provides balance, supply, allowance, transfer,
approval and decimal queries without an external package dependency.

Source: [IERC20Minimal.sol](../../../contracts/meta-ag/interfaces/IERC20Minimal.sol).

### balanceOf

```solidity
function balanceOf(address account) external view returns (uint256)
```

### totalSupply

```solidity
function totalSupply() external view returns (uint256)
```

### transfer

```solidity
function transfer(address to, uint256 amount) external returns (bool)
```

### transferFrom

```solidity
function transferFrom(address from, address to, uint256 amount) external returns (bool)
```

### approve

```solidity
function approve(address spender, uint256 amount) external returns (bool)
```

### allowance

```solidity
function allowance(address owner, address spender) external view returns (uint256)
```

### decimals

```solidity
function decimals() external view returns (uint8)
```
