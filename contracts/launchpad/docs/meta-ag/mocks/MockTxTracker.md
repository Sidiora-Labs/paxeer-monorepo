# Solidity API

Source: [MockTxTracker.sol](../../../contracts/meta-ag/mocks/MockTxTracker.sol).

## MockTxTracker

Test recorder with a `record(bytes)` method. Each call stores the caller and payload, increments
`callCount` and emits `Recorded`.

### lastCaller

```solidity
address lastCaller
```

### lastPayload

```solidity
bytes lastPayload
```

### callCount

```solidity
uint256 callCount
```

### Recorded

```solidity
event Recorded(address indexed caller, bytes payload)
```

### record

```solidity
function record(bytes payload) external
```

Stores the caller and payload and increments the call count.
