# Solidity API

Source: [MockReentrancy.sol](../../contracts/test/MockReentrancy.sol).

## MockReentrancyGuard

### counter

```solidity
uint256 public counter
```

### protectedIncrement

```solidity
function protectedIncrement() external nonReentrant
```

### unprotectedIncrement

```solidity
function unprotectedIncrement() external
```

### reentrantCall

```solidity
function reentrantCall() external nonReentrant
```

### crossFunctionReentrantCall

```solidity
function crossFunctionReentrantCall() external nonReentrant
```

## ReentrancyAttacker

### target

```solidity
MockReentrancyGuard public target
```

### attackCount

```solidity
uint256 public attackCount
```

### constructor

```solidity
constructor(address _target)
```

### attack

```solidity
function attack() external
```

### receive

```solidity
receive() external payable
```
