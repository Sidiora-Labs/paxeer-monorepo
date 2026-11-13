# Solidity API

Source: [GovernanceModule.sol](../../contracts/protocol/GovernanceModule.sol).

## GovernanceModule

UUPS-upgradeable governance with checkpoint-based voting and delayed Timelock execution.
`initialize` configures voting and enables an administrative queueing mode. `initializeAdmin`
configures administrative queueing only, without voting.

Proposals use prior-block voting power for admission and the block before voting starts for vote
weights. After voting succeeds, `execute` queues an atomic Timelock batch; `executeQueued` executes
it after the delay. Only the voting configuration allows administrative mode to be deactivated.

### ADMIN_ROLE

```solidity
bytes32 ADMIN_ROLE
```

### ProposalState

```solidity
enum ProposalState {
  Pending,
  Active,
  Defeated,
  Succeeded,
  Queued,
  Executed,
  Cancelled,
  Expired
}
```

### Proposal

```solidity
struct Proposal {
  address proposer;
  address[] targets;
  uint256[] values;
  bytes[] calldatas;
  string description;
  uint256 startBlock;
  uint256 endBlock;
  uint256 forVotes;
  uint256 againstVotes;
  bool executed;
  bool cancelled;
}
```

### votingToken

```solidity
address votingToken
```

### timelock

```solidity
address timelock
```

### proposalThreshold

```solidity
uint256 proposalThreshold
```

### votingPeriod

```solidity
uint256 votingPeriod
```

### quorumVotes

```solidity
uint256 quorumVotes
```

### proposalCount

```solidity
uint256 proposalCount
```

### proposals

```solidity
mapping(uint256 => struct GovernanceModule.Proposal) proposals
```

### hasVoted

```solidity
mapping(uint256 => mapping(address => bool)) hasVoted
```

### voteSnapshot

```solidity
mapping(uint256 => mapping(address => uint256)) voteSnapshot
```

### adminModeActive

```solidity
bool adminModeActive
```

### BelowProposalThreshold

```solidity
error BelowProposalThreshold()
```

### AlreadyVoted

```solidity
error AlreadyVoted()
```

### VotingClosed

```solidity
error VotingClosed()
```

### QuorumNotMet

```solidity
error QuorumNotMet()
```

### ProposalNotPassed

```solidity
error ProposalNotPassed()
```

### ProposalAlreadyExecuted

```solidity
error ProposalAlreadyExecuted()
```

### ProposalIsCancelled

```solidity
error ProposalIsCancelled()
```

### InvalidProposal

```solidity
error InvalidProposal()
```

### Unauthorized

```solidity
error Unauthorized()
```

### AdminModeNotActive

```solidity
error AdminModeNotActive()
```

### ArrayLengthMismatch

```solidity
error ArrayLengthMismatch()
```

### ProposalCreated

```solidity
event ProposalCreated(uint256 indexed proposalId, address indexed proposer, string description)
```

### VoteCast

```solidity
event VoteCast(uint256 indexed proposalId, address indexed voter, bool support, uint256 weight)
```

### ProposalExecuted

```solidity
event ProposalExecuted(uint256 indexed proposalId)
```

### ProposalCancelled

```solidity
event ProposalCancelled(uint256 indexed proposalId)
```

### AdminModeDeactivated

```solidity
event AdminModeDeactivated()
```

### constructor

```solidity
constructor() public
```

### initialize

```solidity
function initialize(address _votingToken, address _timelock, address _admin, uint256 _proposalThreshold, uint256 _votingPeriod, uint256 _quorumVotes) external
```

### propose

```solidity
function propose(address[] targets, uint256[] values, bytes[] calldatas, string description) external returns (uint256 proposalId)
```

Create a proposal using `getPastVotes(msg.sender, block.number - 1) >= proposalThreshold`.
Unavailable in administrative-only mode.

### castVote

```solidity
function castVote(uint256 proposalId, bool support) external
```

Vote on a proposal

### execute

```solidity
function execute(uint256 proposalId, uint256) external
```

Queue a passed proposal as an atomic Timelock batch. The second argument is unused; the Timelock
sets the execution time from its minimum delay.

### cancel

```solidity
function cancel(uint256 proposalId) external
```

Cancel a proposal as its proposer, or as an administrator while administrative mode is active. A
queued batch is cancelled in the Timelock; an executed batch cannot be cancelled.

### adminExecute

```solidity
function adminExecute(address target, uint256 value, bytes data, uint256 eta) external
```

Admin bypasses governance and queues directly to Timelock

_Only works while adminModeActive is true_

### deactivateAdminMode

```solidity
function deactivateAdminMode() external
```

Deactivate administrative queueing and administrative proposal cancellation in the voting
configuration. Requires `ADMIN_ROLE`; reverts in administrative-only mode. Role membership remains
unchanged, and `DEFAULT_ADMIN_ROLE` continues to authorize upgrades.

### getProposalState

```solidity
function getProposalState(uint256 proposalId) external view returns (enum GovernanceModule.ProposalState)
```

Get the state of a proposal

### \_authorizeUpgrade

```solidity
function _authorizeUpgrade(address) internal
```

### initializeAdmin

```solidity
function initializeAdmin(address _timelock, address _admin) external
```

Initializes administrative-only governance and grants both administrative roles.

### adminOnlyMode

```solidity
bool adminOnlyMode
```

True for the administrative-only configuration.

### proposalOperation

```solidity
mapping(uint256 => bytes32) proposalOperation
```

Timelock batch identifier per queued proposal.

### proposalEta

```solidity
mapping(uint256 => uint256) proposalEta
```

Timelock execution timestamp per queued proposal.

### executeQueued

```solidity
function executeQueued(uint256 proposalId) external
```

Executes the proposal's queued batch and records execution.

### getProposalActions

```solidity
function getProposalActions(uint256 proposalId) external view returns (address[], uint256[], bytes[])
```

Returns proposal targets, values and calldata in matching order.

### AdminOnlyGovernance

```solidity
error AdminOnlyGovernance()
```

### ProposalQueued

```solidity
event ProposalQueued(uint256 indexed proposalId, bytes32 indexed operationId, uint256 eta)
```

## ITimelockQueue

### queueTransaction

```solidity
function queueTransaction(address target, uint256 value, bytes data, uint256 eta) external returns (bytes32)
```

### queueBatch

```solidity
function queueBatch(address[], uint256[], bytes[], bytes32) external returns (bytes32, uint256)
```

### executeBatch

```solidity
function executeBatch(address[], uint256[], bytes[], bytes32) external
```

### cancelBatch

```solidity
function cancelBatch(bytes32) external
```

### batchStatus

```solidity
function batchStatus(bytes32) external view returns (uint8)
```

### getMinDelay

```solidity
function getMinDelay() external view returns (uint256)
```

### GRACE_PERIOD

```solidity
function GRACE_PERIOD() external view returns (uint256)
```

## IVotingCheckpoints

### getPastVotes

```solidity
function getPastVotes(address account, uint256 blockNumber) external view returns (uint256)
```

Returns the checkpointed voting power used for proposals and voting.
