# Solidity API

Source: [ReentrancyGuard.sol](../../contracts/base/ReentrancyGuard.sol).

## ReentrancyGuard

Prevents reentrant calls to protected functions

_Uses a storage slot flag to detect and block reentrancy_

### ReentrancyGuardReentrantCall

```solidity
error ReentrancyGuardReentrantCall()
```

### constructor

```solidity
constructor()
```

### nonReentrant

```solidity
modifier nonReentrant()
```

### \_initReentrancyGuard

```solidity
function _initReentrancyGuard() internal
```

_Initialize reentrancy guard for proxied contracts (no constructor)_
