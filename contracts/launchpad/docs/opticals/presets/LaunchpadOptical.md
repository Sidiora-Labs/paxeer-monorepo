# Solidity API

Source: [LaunchpadOptical.sol](../../../contracts/opticals/presets/LaunchpadOptical.sol).

## LaunchpadOptical

Non-upgradeable optical with cliff and linear vesting for the creator and team wallets, plus a
time-limited diversion of the pool's fee share to a team claim address.

Vesting is enforced by `beforeTokenTransfer`, called by the registered pool's token. It checks
outbound transfers from vested wallets and records cumulative transferred amounts in `tokensSold`.
Cliff, vesting and capital-raise periods start at PoolRegistry's `createdAt` timestamp. The pool's
registered creator must match the configured creator when recording its start.

`beforeSwap` records the start time and `afterSwap` returns its selector. `beforeFeeDistribution`
records capital-raise amounts, with the fee accumulator holding the corresponding optical surplus.
Only `teamClaimAddress` may call `claimCapital(pool)`; the accumulator transfers the claim there.

### CliffActive

```solidity
error CliffActive()
```

### VestingExceeded

```solidity
error VestingExceeded()
```

### CapitalRaiseTooHigh

```solidity
error CapitalRaiseTooHigh()
```

### DurationTooLong

```solidity
error DurationTooLong()
```

### NoTeamWallets

```solidity
error NoTeamWallets()
```

### TooManyTeamWallets

```solidity
error TooManyTeamWallets()
```

### NothingToClaim

```solidity
error NothingToClaim()
```

### NotTeam

```solidity
error NotTeam()
```

### PoolStartTimeRecorded

```solidity
event PoolStartTimeRecorded(address indexed pool, uint256 startTime)
```

### SellBlockedCliff

```solidity
event SellBlockedCliff(address indexed pool, address indexed sender, uint256 amountIn)
```

### SellBlockedVesting

```solidity
event SellBlockedVesting(address indexed pool, address indexed sender, uint256 amountIn, uint256 maxSellable)
```

### CapitalRaiseAccumulated

```solidity
event CapitalRaiseAccumulated(address indexed pool, uint256 amount, uint256 totalRaisedAmount)
```

### CapitalRaiseClaimed

```solidity
event CapitalRaiseClaimed(address indexed pool, uint256 amount, address indexed recipient)
```

### MAX_CAPITAL_RAISE_BPS

```solidity
uint256 MAX_CAPITAL_RAISE_BPS
```

### MAX_CLIFF_DURATION

```solidity
uint256 MAX_CLIFF_DURATION
```

### MAX_VESTING_DURATION

```solidity
uint256 MAX_VESTING_DURATION
```

### MAX_RAISE_DURATION

```solidity
uint256 MAX_RAISE_DURATION
```

### MAX_TEAM_WALLETS

```solidity
uint256 MAX_TEAM_WALLETS
```

### creator

```solidity
address creator
```

Pool creator wallet (auto-vested)

### teamClaimAddress

```solidity
address teamClaimAddress
```

Address where accumulated USDL capital is claimed to

### feeAccumulator

```solidity
address feeAccumulator
```

FeeAccumulator address for claiming optical surplus

### cliffDuration

```solidity
uint256 cliffDuration
```

Hard lock period in seconds before any vesting begins

### vestingDuration

```solidity
uint256 vestingDuration
```

Linear unlock period in seconds after the cliff

### capitalRaiseBps

```solidity
uint256 capitalRaiseBps
```

Fee percentage diverted to team (in bps, max 1000 = 10%)

### capitalRaiseDuration

```solidity
uint256 capitalRaiseDuration
```

How long the capital raise fee is active (in seconds)

### teamWalletCount

```solidity
uint256 teamWalletCount
```

Number of entries supplied in `_teamWallets`, including zero or repeated entries. The creator and
each nonzero entry are marked as vested.

### isVested

```solidity
mapping(address => bool) isVested
```

Whether an address is subject to vesting restrictions

### poolStartTime

```solidity
mapping(address => uint256) poolStartTime
```

PoolRegistry creation timestamp, cached on first interaction.

### tokensSold

```solidity
mapping(address => mapping(address => uint256)) tokensSold
```

Cumulative outbound token transfers from each vested wallet per pool, used in vesting calculations.

_pool → wallet → amount sold_

### accumulatedUsdl

```solidity
mapping(address => uint256) accumulatedUsdl
```

USDL accumulated per pool for team capital raise (accounting only)

### totalRaised

```solidity
mapping(address => uint256) totalRaised
```

Lifetime total USDL raised per pool

### constructor

```solidity
constructor(address _poolRegistry, address _owner, address _creator, address[] _teamWallets, uint256 _cliffDuration, uint256 _vestingDuration, uint256 _capitalRaiseBps, uint256 _capitalRaiseDuration, address _teamClaimAddress, address _feeAccumulator) public
```

#### Parameters

| Name                   | Type      | Description                                         |
| ---------------------- | --------- | --------------------------------------------------- |
| \_poolRegistry         | address   | PoolRegistry address for pool validation            |
| \_owner                | address   | Deployer/owner address                              |
| \_creator              | address   | Pool creator wallet (auto-vested)                   |
| \_teamWallets          | address[] | Additional team wallets to vest                     |
| \_cliffDuration        | uint256   | Hard lock period in seconds                         |
| \_vestingDuration      | uint256   | Linear unlock period after cliff in seconds         |
| \_capitalRaiseBps      | uint256   | Fee percentage diverted to team (max 1000 = 10%)    |
| \_capitalRaiseDuration | uint256   | How long the capital raise fee is active in seconds |
| \_teamClaimAddress     | address   | Where accumulated USDL is claimed to                |
| \_feeAccumulator       | address   | FeeAccumulator contract address                     |

### getFlags

```solidity
function getFlags() external pure returns (uint8)
```

### beforeSwap

```solidity
function beforeSwap(address pool, address sender, bool isBuy, uint256 amountIn) external returns (bool proceed, int256 amountDelta)
```

### afterSwap

```solidity
function afterSwap(address pool, address, bool, uint256, uint256) external returns (bytes4)
```

### beforeFeeDistribution

```solidity
function beforeFeeDistribution(address pool, uint256 feeAmount) external returns (uint256 adjustedFee)
```

### claimCapital

```solidity
function claimCapital(address pool) external
```

Claim accumulated USDL capital for a pool

#### Parameters

| Name | Type    | Description                            |
| ---- | ------- | -------------------------------------- |
| pool | address | The pool address to claim capital from |

### getVestingInfo

```solidity
function getVestingInfo(address pool, address wallet) external view returns (bool vested, uint256 cliffEnd, uint256 vestEnd, uint256 vestedAmount, uint256 sold, uint256 maxSellableNow)
```

Get vesting info for a wallet in a specific pool

### getCapitalRaiseInfo

```solidity
function getCapitalRaiseInfo(address pool) external view returns (uint256 accumulated, uint256 totalRaisedAmount, uint256 raiseEndTime, bool isActive)
```

Get capital raise info for a pool

### isTeamWallet

```solidity
function isTeamWallet(address wallet) external view returns (bool)
```

Check if an address is a team wallet (vested)

### \_getTokenBalance

```solidity
function _getTokenBalance(address pool, address wallet) internal view returns (uint256)
```

_Read token balance of a wallet via the pool's token address_

### beforeTokenTransfer

```solidity
function beforeTokenTransfer(address pool, address from, address to, uint256 amount) external
```

Requires the registered pool's token as caller. Enforces cliff and vesting on transfers from vested
wallets, excluding minting, self-transfers and zero amounts. Uses flag bit 4 (16).

### \_recordStart

```solidity
function _recordStart(address pool) internal returns (uint256 start)
```

Validates the pool creator and caches the registry creation timestamp.
