# Solidity API

Source: [Timelock.sol](../../contracts/protocol/Timelock.sol).

## Timelock

Non-upgradeable delayed executor for individual transactions and atomic batches. `minDelay` is fixed
at construction and must be greater than zero and at most 30 days. Queued operations can execute
from their ETA through the 14-day grace period. Anyone may execute a ready operation.

Only the proposer queues operations. The guardian cancels individual transactions; the guardian or
proposer may cancel batches. Proposer changes use an initial direct handoff followed by Timelock
self-calls. Guardian changes require nomination and acceptance.

### minDelay

```solidity
uint256 minDelay
```

### proposer

```solidity
address proposer
```

### guardian

```solidity
address guardian
```

### queuedTransactions

```solidity
mapping(bytes32 => bool) queuedTransactions
```

### DelayNotMet

```solidity
error DelayNotMet()
```

### TransactionNotQueued

```solidity
error TransactionNotQueued()
```

### TransactionAlreadyQueued

```solidity
error TransactionAlreadyQueued()
```

### Unauthorized

```solidity
error Unauthorized()
```

### ExecutionFailed

```solidity
error ExecutionFailed()
```

### ZeroAddress

```solidity
error ZeroAddress()
```

### InvalidDelay

```solidity
error InvalidDelay()
```

### TransactionQueued

```solidity
event TransactionQueued(bytes32 indexed txHash, address indexed target, uint256 value, bytes data, uint256 eta)
```

### TransactionExecuted

```solidity
event TransactionExecuted(bytes32 indexed txHash, address indexed target, uint256 value, bytes data, uint256 eta)
```

### TransactionCancelled

```solidity
event TransactionCancelled(bytes32 indexed txHash)
```

### ProposerChanged

```solidity
event ProposerChanged(address indexed oldProposer, address indexed newProposer)
```

### GuardianChanged

```solidity
event GuardianChanged(address indexed oldGuardian, address indexed newGuardian)
```

### onlyProposer

```solidity
modifier onlyProposer()
```

### onlyGuardian

```solidity
modifier onlyGuardian()
```

### constructor

```solidity
constructor(uint256 _minDelay, address _proposer, address _guardian) public
```

### queueTransaction

```solidity
function queueTransaction(address target, uint256 value, bytes data, uint256 eta) external returns (bytes32 txHash)
```

Proposer-only queueing. ETA must be between `block.timestamp + minDelay` and one day after that.

### executeTransaction

```solidity
function executeTransaction(address target, uint256 value, bytes data, uint256 eta) external returns (bytes result)
```

Execute a queued transaction after its delay has passed

### cancelTransaction

```solidity
function cancelTransaction(address target, uint256 value, bytes data, uint256 eta) external
```

Cancel a queued transaction (guardian only)

### setProposer

```solidity
function setProposer(address newProposer) external
```

The current proposer may make one direct handoff. Subsequent proposer changes require a Timelock
self-call. The new proposer must be nonzero.

### setGuardian

```solidity
function setGuardian(address newGuardian) external
```

Nominate a nonzero guardian, callable by the current guardian or through a Timelock self-call. The
nominee must call `acceptGuardian` to complete the change.

### getMinDelay

```solidity
function getMinDelay() external view returns (uint256)
```

### \_getTxHash

```solidity
function _getTxHash(address target, uint256 value, bytes data, uint256 eta) internal pure returns (bytes32)
```

### receive

```solidity
receive() external payable
```

### GRACE_PERIOD

```solidity
uint256 GRACE_PERIOD
```

Execution grace period: 14 days after ETA.

### pendingGuardian

```solidity
address pendingGuardian
```

Nominated guardian awaiting acceptance.

### proposerHandedOff

```solidity
bool proposerHandedOff
```

Whether the initial proposer handoff has occurred.

### batchStatus

```solidity
mapping(bytes32 => uint8) batchStatus
```

Batch status: 0 unset, 1 queued, 2 executed, 3 cancelled.

### batchEta

```solidity
mapping(bytes32 => uint256) batchEta
```

Earliest execution timestamp for each batch.

### acceptGuardian

```solidity
function acceptGuardian() external
```

Accepts the guardian nomination; callable only by the pending guardian.

### hashBatch

```solidity
function hashBatch(address[] targets, uint256[] values, bytes[] data, bytes32 salt) public pure returns (bytes32)
```

Returns the batch identifier from its actions and salt.

### queueBatch

```solidity
function queueBatch(address[] targets, uint256[] values, bytes[] data, bytes32 salt) external returns (bytes32 id, uint256 eta)
```

Proposer-only queueing of nonempty arrays of equal length. ETA is the current timestamp plus
`minDelay`.

### executeBatch

```solidity
function executeBatch(address[] targets, uint256[] values, bytes[] data, bytes32 salt) external
```

Executes a queued, ready batch atomically. A failed action reverts the entire batch.

### cancelBatch

```solidity
function cancelBatch(bytes32 id) external
```

Cancels a queued batch as guardian or proposer.

### OperationExpired

```solidity
error OperationExpired()
```

### InvalidBatch

```solidity
error InvalidBatch()
```
