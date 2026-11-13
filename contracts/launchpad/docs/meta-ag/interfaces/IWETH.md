# Solidity API

## IWETH

Interface for Wrapped Native Coin (WETH / WPAX)

WETH9-compatible deposit, withdrawal and ERC20 operations. Meta-AG contracts receive their wrapped
native token address through initialization; on Paxeer, that token is WPAX.

Source: [IWETH.sol](../../../contracts/meta-ag/interfaces/IWETH.sol).

### deposit

```solidity
function deposit() external payable
```

### withdraw

```solidity
function withdraw(uint256 wad) external
```

### totalSupply

```solidity
function totalSupply() external view returns (uint256)
```

### balanceOf

```solidity
function balanceOf(address owner) external view returns (uint256)
```

### transfer

```solidity
function transfer(address to, uint256 value) external returns (bool)
```

### transferFrom

```solidity
function transferFrom(address from, address to, uint256 value) external returns (bool)
```

### approve

```solidity
function approve(address spender, uint256 value) external returns (bool)
```

### allowance

```solidity
function allowance(address owner, address spender) external view returns (uint256)
```

### Deposit

```solidity
event Deposit(address indexed dst, uint256 wad)
```

### Withdrawal

```solidity
event Withdrawal(address indexed src, uint256 wad)
```
