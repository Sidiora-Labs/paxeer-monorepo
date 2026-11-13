# Solidity API

## IFeeAccumulator

Interface for fee tracking and distribution across pools

Source: [IFeeAccumulator.sol](../../contracts/interfaces/IFeeAccumulator.sol).

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

### NoFeesAccumulated

```solidity
error NoFeesAccumulated()
```

### WrongStrategy

```solidity
error WrongStrategy()
```

### AlreadyClaimed

```solidity
error AlreadyClaimed()
```

### AirdropNotTriggered

```solidity
error AirdropNotTriggered()
```

### InsufficientSurplus

```solidity
error InsufficientSurplus()
```

### FeeRecorded

```solidity
event FeeRecorded(address indexed pool, uint256 feeAmount, uint256 protocolCut, uint256 poolCut)
```

### FeesClaimed

```solidity
event FeesClaimed(address indexed pool, address indexed recipient, uint256 amount)
```

### FeesBurned

```solidity
event FeesBurned(address indexed pool, uint256 amount)
```

### AirdropTriggered

```solidity
event AirdropTriggered(address indexed pool, uint256 totalAmount, uint256 epoch)
```

### AirdropClaimed

```solidity
event AirdropClaimed(address indexed pool, address indexed holder, uint256 amount, uint256 epoch)
```

### LpRewardsSent

```solidity
event LpRewardsSent(address indexed pool, uint256 amount)
```

### ProtocolFeeSwept

```solidity
event ProtocolFeeSwept(uint256 amount)
```

### OpticalSurplusRecorded

```solidity
event OpticalSurplusRecorded(address indexed pool, uint256 amount)
```

### OpticalSurplusClaimed

```solidity
event OpticalSurplusClaimed(address indexed pool, address indexed recipient, uint256 amount)
```

### recordFee

```solidity
function recordFee(address pool, uint256 feeAmount) external
```

Record fees from a swap (pool-only)

#### Parameters

| Name      | Type    | Description                        |
| --------- | ------- | ---------------------------------- |
| pool      | address | The pool address the fees are from |
| feeAmount | uint256 | Total fee amount in USDL           |

### claim

```solidity
function claim(address pool, address recipient) external returns (uint256 amount)
```

Claim accumulated fees for a pool. Requires FEES_ROUTER_ROLE; FeesRouter enforces the CLAIM
strategy.

#### Parameters

| Name      | Type    | Description                 |
| --------- | ------- | --------------------------- |
| pool      | address | The pool address            |
| recipient | address | Address to receive the fees |

#### Return Values

| Name   | Type    | Description                |
| ------ | ------- | -------------------------- |
| amount | uint256 | The amount of USDL claimed |

### burn

```solidity
function burn(address pool) external returns (uint256 amount)
```

Transfer accumulated fees to the burn address. Requires FEES_ROUTER_ROLE; FeesRouter enforces the
BURN strategy.

#### Parameters

| Name | Type    | Description      |
| ---- | ------- | ---------------- |
| pool | address | The pool address |

#### Return Values

| Name   | Type    | Description               |
| ------ | ------- | ------------------------- |
| amount | uint256 | The amount of USDL burned |

### triggerAirdrop

```solidity
function triggerAirdrop(address pool) external returns (uint256 totalAmount)
```

Start a snapshot airdrop epoch. Requires FEES_ROUTER_ROLE; FeesRouter enforces the AIRDROP strategy.
Eligibility uses the previous block’s token balances and excludes the pool and burn address.

#### Parameters

| Name | Type    | Description      |
| ---- | ------- | ---------------- |
| pool | address | The pool address |

#### Return Values

| Name        | Type    | Description                  |
| ----------- | ------- | ---------------------------- |
| totalAmount | uint256 | The total amount distributed |

### claimAirdrop

```solidity
function claimAirdrop(address pool, address holder) external returns (uint256 amount)
```

Claim the latest epoch’s snapshot share for a token holder; payment goes to holder regardless of
caller. The claim window is 180 days from triggering.

#### Parameters

| Name   | Type    | Description              |
| ------ | ------- | ------------------------ |
| pool   | address | The pool address         |
| holder | address | The token holder address |

#### Return Values

| Name   | Type    | Description                |
| ------ | ------- | -------------------------- |
| amount | uint256 | The amount of USDL claimed |

### sendLpRewards

```solidity
function sendLpRewards(address pool) external returns (uint256 amount)
```

Donate accumulated fees to pool liquidity. Requires FEES_ROUTER_ROLE; FeesRouter enforces LP_REWARDS
and synchronizes pool reserves.

#### Parameters

| Name | Type    | Description      |
| ---- | ------- | ---------------- |
| pool | address | The pool address |

#### Return Values

| Name   | Type    | Description             |
| ------ | ------- | ----------------------- |
| amount | uint256 | The amount of USDL sent |

### authorizePool

```solidity
function authorizePool(address pool) external
```

Authorize a pool to record fees (Factory-only via FACTORY_ROLE)

#### Parameters

| Name | Type    | Description                   |
| ---- | ------- | ----------------------------- |
| pool | address | The pool address to authorize |

### getAccumulatedFees

```solidity
function getAccumulatedFees(address pool) external view returns (uint256)
```

Get accumulated fees for a pool

### getProtocolFeesPending

```solidity
function getProtocolFeesPending() external view returns (uint256)
```

Get pending protocol fees waiting to be swept

### getAirdropBalance

```solidity
function getAirdropBalance(address pool) external view returns (uint256)
```

Get airdrop balance for a pool

### getLpRewardsBalance

```solidity
function getLpRewardsBalance(address pool) external view returns (uint256)
```

Get cumulative USDL donated to pool liquidity.

### getAirdropEpoch

```solidity
function getAirdropEpoch(address pool) external view returns (uint256)
```

Get current airdrop epoch for a pool

### hasClaimedAirdrop

```solidity
function hasClaimedAirdrop(address pool, address holder, uint256 epoch) external view returns (bool)
```

Check if holder has claimed for a specific epoch

### authorizeOptical

```solidity
function authorizeOptical(address optical) external
```

Grant OPTICAL_CLAIM_ROLE to an optical; requires OPTICAL_GRANTER_ROLE. claimOpticalSurplus
authorizes the pool’s attached optical through registry metadata.

#### Parameters

| Name    | Type    | Description                               |
| ------- | ------- | ----------------------------------------- |
| optical | address | The optical contract address to authorize |

### recordOpticalSurplus

```solidity
function recordOpticalSurplus(address pool, uint256 amount) external
```

Record additional USDL-backed surplus for the calling registered pool. Requires POOL_ROLE and
sufficient unaccounted USDL to back the amount.

#### Parameters

| Name   | Type    | Description                  |
| ------ | ------- | ---------------------------- |
| pool   | address | The pool address             |
| amount | uint256 | The surplus amount to record |

### claimOpticalSurplus

```solidity
function claimOpticalSurplus(address pool, uint256 amount, address recipient) external
```

Claim surplus as the optical contract currently attached to the pool in PoolRegistry. The caller
must contain contract code.

#### Parameters

| Name      | Type    | Description                 |
| --------- | ------- | --------------------------- |
| pool      | address | The pool address            |
| amount    | uint256 | Amount to claim             |
| recipient | address | Address to receive the USDL |

### getOpticalSurplus

```solidity
function getOpticalSurplus(address pool) external view returns (uint256)
```

Get optical surplus balance for a pool

### AirdropClaimWindowOpen

```solidity
error AirdropClaimWindowOpen()
```

### AirdropClaimWindowClosed

```solidity
error AirdropClaimWindowClosed()
```

### UnsupportedSnapshotToken

```solidity
error UnsupportedSnapshotToken()
```

### AirdropExpired

```solidity
event AirdropExpired(address indexed pool, uint256 indexed epoch, uint256 amount)
```

### supportsAirdrop

```solidity
function supportsAirdrop(address pool) external view returns (bool)
```

Checks whether the pool’s token exposes previous-block total supply and balance checkpoints.

### claimAirdropEpoch

```solidity
function claimAirdropEpoch(address pool, address holder, uint256 epoch) external returns (uint256 amount)
```

Claims a specified snapshot epoch for holder and sends payment to that holder. Claims close 180 days
after the epoch is triggered; the final eligible claimant receives rounding residue.

### sweepExpiredAirdrop

```solidity
function sweepExpiredAirdrop(address pool, uint256 epoch) external returns (uint256 amount)
```

After the 180-day claim window, anyone may return the epoch’s unclaimed amount to the pool’s
accumulated fees.
