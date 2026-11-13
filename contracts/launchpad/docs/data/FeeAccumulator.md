# Solidity API

Source: [FeeAccumulator.sol](../../contracts/data/FeeAccumulator.sol).

## IOpticalFee

_Minimal interface for optical hooks used by FeeAccumulator_

### beforeFeeDistribution

```solidity
function beforeFeeDistribution(address pool, uint256 feeAmount) external returns (uint256 adjustedFee)
```

### afterFeeDistribution

```solidity
function afterFeeDistribution(address pool, uint256 feeAmount) external returns (bytes4)
```

### getFlags

```solidity
function getFlags() external view returns (uint8)
```

## FeeAccumulator

Tracks accumulated fees per pool and executes fee distribution strategies

_UUPS proxy. Inspired by Aerodrome's PoolFees separation._

### POOL_ROLE

```solidity
bytes32 POOL_ROLE
```

### FEES_ROUTER_ROLE

```solidity
bytes32 FEES_ROUTER_ROLE
```

### FACTORY_ROLE

```solidity
bytes32 FACTORY_ROLE
```

### OPTICAL_CLAIM_ROLE

```solidity
bytes32 OPTICAL_CLAIM_ROLE
```

### OPTICAL_GRANTER_ROLE

```solidity
bytes32 OPTICAL_GRANTER_ROLE
```

### authorizePool

```solidity
function authorizePool(address pool) external onlyRole(FACTORY_ROLE)
```

Authorize a pool to record fees. Called by Factory on market creation.

### DEAD_ADDRESS

```solidity
address DEAD_ADDRESS
```

### protocolConfig

```solidity
address protocolConfig
```

### treasury

```solidity
address treasury
```

### poolRegistry

```solidity
address poolRegistry
```

### eventEmitter

```solidity
address eventEmitter
```

### usdlAddress

```solidity
address usdlAddress
```

### constructor

```solidity
constructor()
```

### initialize

```solidity
function initialize( address _protocolConfig, address _treasury, address _poolRegistry, address _eventEmitter, address _usdlAddress, address _admin ) external initializer
```

### recordFee

```solidity
function recordFee(address pool, uint256 feeAmount) external override onlyRole(POOL_ROLE) nonReentrant
```

Records prepaid USDL fees for the caller's own registered pool. The caller must have `POOL_ROLE`,
and the USDL balance must cover existing liabilities plus the fee. The protocol cut is deposited
in the treasury; the remaining pool cut can be reduced by the optical's fee-distribution hook,
with the difference held as optical surplus.

#### Parameters

| Name      | Type    | Description                        |
| --------- | ------- | ---------------------------------- |
| pool      | address | The pool address the fees are from |
| feeAmount | uint256 | Total fee amount in USDL           |

### claim

```solidity
function claim( address pool, address recipient ) external override onlyRole(FEES_ROUTER_ROLE) nonReentrant returns (uint256 amount)
```

Claim accumulated fees for a pool (CLAIM strategy, FeesRouter-only)

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
function burn(address pool) external override onlyRole(FEES_ROUTER_ROLE) nonReentrant returns (uint256 amount)
```

Transfers accumulated USDL fees to `DEAD_ADDRESS` (BURN strategy, FeesRouter-only).

#### Parameters

| Name | Type    | Description      |
| ---- | ------- | ---------------- |
| pool | address | The pool address |

#### Return Values

| Name   | Type    | Description                                    |
| ------ | ------- | ---------------------------------------------- |
| amount | uint256 | The amount of USDL transferred to DEAD_ADDRESS |

### triggerAirdrop

```solidity
function triggerAirdrop( address pool ) external override onlyRole(FEES_ROUTER_ROLE) nonReentrant returns (uint256 totalAmount)
```

Reserves accumulated fees for a new airdrop epoch. Requires `FEES_ROUTER_ROLE` and historical
balance and supply reads from the pool token. The snapshot uses the preceding block and excludes
balances held by the pool and `DEAD_ADDRESS` from the eligible supply.

#### Parameters

| Name | Type    | Description      |
| ---- | ------- | ---------------- |
| pool | address | The pool address |

#### Return Values

| Name        | Type    | Description                                         |
| ----------- | ------- | --------------------------------------------------- |
| totalAmount | uint256 | The USDL amount reserved for the new snapshot epoch |

### claimAirdrop

```solidity
function claimAirdrop(address pool, address holder) external override nonReentrant returns (uint256 amount)
```

Pays a holder's share of the latest airdrop epoch to that holder. Anyone can submit the claim;
the holder's snapshot balance determines the share.

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
function sendLpRewards( address pool ) external override onlyRole(FEES_ROUTER_ROLE) nonReentrant returns (uint256 amount)
```

Send accumulated fees to pool as LP rewards (LP_REWARDS strategy, FeesRouter-only)

#### Parameters

| Name | Type    | Description      |
| ---- | ------- | ---------------- |
| pool | address | The pool address |

#### Return Values

| Name   | Type    | Description             |
| ------ | ------- | ----------------------- |
| amount | uint256 | The amount of USDL sent |

### getAccumulatedFees

```solidity
function getAccumulatedFees(address pool) external view override returns (uint256)
```

Get accumulated fees for a pool

### getProtocolFeesPending

```solidity
function getProtocolFeesPending() external view override returns (uint256)
```

Get pending protocol fees waiting to be swept

### getAirdropBalance

```solidity
function getAirdropBalance(address pool) external view override returns (uint256)
```

Get airdrop balance for a pool

### getLpRewardsBalance

```solidity
function getLpRewardsBalance(address pool) external view override returns (uint256)
```

Returns cumulative USDL donations sent to the pool, rather than a claimable balance.

### getAirdropEpoch

```solidity
function getAirdropEpoch(address pool) external view override returns (uint256)
```

Get current airdrop epoch for a pool

### hasClaimedAirdrop

```solidity
function hasClaimedAirdrop(address pool, address holder, uint256 epoch) external view override returns (bool)
```

Check if holder has claimed for a specific epoch

### \_getTokenBalance

```solidity
function _getTokenBalance(address token, address account) internal view returns (uint256)
```

### \_getTokenTotalSupply

```solidity
function _getTokenTotalSupply(address token) internal view returns (uint256)
```

### authorizeOptical

```solidity
function authorizeOptical(address optical) external onlyRole(OPTICAL_GRANTER_ROLE)
```

Grants `OPTICAL_CLAIM_ROLE` to an optical. Requires `OPTICAL_GRANTER_ROLE`, which is used by
LaunchpadOpticalFactory. Surplus claims check the pool's optical attachment in the registry.

### recordOpticalSurplus

```solidity
function recordOpticalSurplus(address pool, uint256 amount) external override onlyRole(POOL_ROLE) nonReentrant
```

Records additional prepaid USDL as optical surplus for the caller's own registered pool. Requires
`POOL_ROLE` and sufficient USDL backing beyond existing liabilities. A zero amount returns
without changing balances.

#### Parameters

| Name   | Type    | Description                  |
| ------ | ------- | ---------------------------- |
| pool   | address | The pool address             |
| amount | uint256 | The surplus amount to record |

### claimOpticalSurplus

```solidity
function claimOpticalSurplus(address pool, uint256 amount, address recipient) external override nonReentrant
```

Claims optical surplus for a pool. The caller must be the optical registered for that pool and have
code; the recipient must be nonzero.

#### Parameters

| Name      | Type    | Description                 |
| --------- | ------- | --------------------------- |
| pool      | address | The pool address            |
| amount    | uint256 | Amount to claim             |
| recipient | address | Address to receive the USDL |

### getOpticalSurplus

```solidity
function getOpticalSurplus(address pool) external view override returns (uint256)
```

Get optical surplus balance for a pool

### \_authorizeUpgrade

```solidity
function _authorizeUpgrade(address) internal override onlyRole(DEFAULT_ADMIN_ROLE)
```

### supportsAirdrop

```solidity
function supportsAirdrop(address pool) public view override returns (bool)
```

Checks historical balance and supply read support on the pool token.

### claimAirdropEpoch

```solidity
function claimAirdropEpoch(address pool, address holder, uint256 epoch) external override nonReentrant returns (uint256 amount)
```

Pays the holder's share of a specified snapshot epoch to that holder. Snapshot balances exclude the
pool and dead address; claims close 180 days after the epoch is triggered. Each holder can claim
once per epoch. The final eligible claimant receives any rounding remainder.

### sweepExpiredAirdrop

```solidity
function sweepExpiredAirdrop(address pool, uint256 epoch) external override nonReentrant returns (uint256 amount)
```

Returns unclaimed expired-epoch USDL to the pool’s accumulated fees.
