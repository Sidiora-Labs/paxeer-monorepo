# Solidity API

## ITreasury

Interface for the protocol treasury contract

Source: [ITreasury.sol](../../contracts/interfaces/ITreasury.sol).

### InsufficientBalance

```solidity
error InsufficientBalance()
```

### Unauthorized

```solidity
error Unauthorized()
```

### ZeroAddress

```solidity
error ZeroAddress()
```

### ZeroAmount

```solidity
error ZeroAmount()
```

### Deposited

```solidity
event Deposited(address indexed token, address indexed from, uint256 amount)
```

### Withdrawn

```solidity
event Withdrawn(address indexed token, address indexed to, uint256 amount)
```

### deposit

```solidity
function deposit(address token, uint256 amount) external
```

Requires DEPOSITOR_ROLE and token approval to the treasury; credits the amount actually received.

### withdraw

```solidity
function withdraw(address token, address to, uint256 amount) external
```

Requires DEFAULT_ADMIN_ROLE, a nonzero recipient and sufficient accounted balance.

### getBalance

```solidity
function getBalance(address token) external view returns (uint256)
```

### recordDeposit

```solidity
function recordDeposit(address token, uint256 amount) external
```

Accounts for tokens already transferred to the treasury. Requires DEPOSITOR_ROLE, a nonzero amount
and enough unaccounted token balance; call atomically with the transfer.
