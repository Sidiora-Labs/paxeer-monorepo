# Solidity API

Source: [MockPausable.sol](../../contracts/test/MockPausable.sol).

## MockPausable

### value

```solidity
uint256 public value
```

### protectedAction

```solidity
function protectedAction(uint256 _val) external whenNotPaused
```

### emergencyAction

```solidity
function emergencyAction() external whenPaused returns (bool)
```

### pause

```solidity
function pause() external
```

### unpause

```solidity
function unpause() external
```

### paused

```solidity
function paused() external view returns (bool)
```
