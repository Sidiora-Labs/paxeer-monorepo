# Solidity API

Source: [Treasury.sol](../../contracts/protocol/Treasury.sol).

## Treasury

UUPS-upgradeable treasury with recorded token balances. `DEPOSITOR_ROLE` holders can deposit
tokens using a token allowance or record tokens already transferred into custody. Deposits credit
the actual amount received. Administrators withdraw recorded balances, recover unrecorded surplus
and authorize upgrades.

### DEPOSITOR_ROLE

```solidity
bytes32 DEPOSITOR_ROLE
```

### eventEmitter

```solidity
address eventEmitter
```

### constructor

```solidity
constructor() public
```

### initialize

```solidity
function initialize(address _eventEmitter, address _admin) external
```

### deposit

```solidity
function deposit(address token, uint256 amount) external
```

Deposit tokens into the treasury

Caller must have `DEPOSITOR_ROLE` and give the treasury sufficient token allowance. The treasury
records the positive increase in its token balance.

### withdraw

```solidity
function withdraw(address token, address to, uint256 amount) external
```

Withdraw tokens from the treasury

_Only admin (governance/timelock) can withdraw._

### getBalance

```solidity
function getBalance(address token) external view returns (uint256)
```

Returns the recorded treasury balance for a token, excluding unrecorded surplus.

### \_authorizeUpgrade

```solidity
function _authorizeUpgrade(address) internal
```

### recordDeposit

```solidity
function recordDeposit(address token, uint256 amount) external
```

Requires `DEPOSITOR_ROLE`. Credits a positive amount already held in custody above the recorded
balance.

### recoverSurplus

```solidity
function recoverSurplus(address token, address to, uint256 amount) external
```

Administrator-only transfer of a positive amount held above the recorded balance to a nonzero
recipient.
