# Solidity API

Source: [WPAX9.sol](../contracts/WPAX9.sol).

## WPAX9

Wrapped Paxeer — 1:1 wrapped version of native PAX.

_Modern Solidity 0.8.x port of canonical WETH9 behavior_

### name

```solidity
string public constant name = "Wrapped Paxeer"
```

### symbol

```solidity
string public constant symbol = "WPAX"
```

### decimals

```solidity
uint8 public constant decimals = 18
```

### Approval

```solidity
event Approval(address indexed src, address indexed guy, uint256 wad)
```

### Transfer

```solidity
event Transfer(address indexed src, address indexed dst, uint256 wad)
```

### Deposit

```solidity
event Deposit(address indexed dst, uint256 wad)
```

### Withdrawal

```solidity
event Withdrawal(address indexed src, uint256 wad)
```

### balanceOf

```solidity
mapping(address => uint256) balanceOf
```

### allowance

```solidity
mapping(address => mapping(address => uint256)) allowance
```

### receive

```solidity
receive() external payable
```

### fallback

```solidity
fallback() external payable
```

### deposit

```solidity
function deposit() public payable
```

Credits `msg.value` to the sender and emits `Deposit`. Both `receive` and
`fallback` call this function. Deposits do not emit `Transfer`.

### withdraw

```solidity
function withdraw(uint256 wad) public
```

Debits the sender's wrapped balance, transfers native PAX to the sender and emits
`Withdrawal`. Insufficient balance or a failed native transfer reverts the call.
Withdrawals do not emit `Transfer`.

### totalSupply

```solidity
function totalSupply() public view returns (uint256)
```

Returns the contract's native PAX balance.

### approve

```solidity
function approve(address guy, uint256 wad) public returns (bool)
```

### transfer

```solidity
function transfer(address dst, uint256 wad) public returns (bool)
```

### transferFrom

```solidity
function transferFrom(address src, address dst, uint256 wad) public returns (bool)
```

Moves the wrapped balance and emits `Transfer`. A caller transferring its own
balance needs no allowance; an allowance of `type(uint256).max` is not decremented.
