# Solidity API

Source: [BuybackBurnOptical.sol](../../../contracts/opticals/presets/BuybackBurnOptical.sol).

## BuybackBurnOptical

Diverts up to 50% of the pool's fee share to buyback accounting through `beforeFeeDistribution`. The
fee accumulator holds the corresponding optical surplus.

The owner calls `executeBuybackBurn(pool, minTokensOut, deadline)` to claim that surplus, transfer
USDL into the pool and buy tokens directly to `DEAD_ADDRESS`. A positive minimum token output is
required. The buyback rate is fixed at construction.

### NothingToExecute

```solidity
error NothingToExecute()
```

### DEAD_ADDRESS

```solidity
address DEAD_ADDRESS
```

Dead address for burning tokens

### buybackBps

```solidity
uint256 buybackBps
```

Percentage of fees redirected to buyback+burn (in bps, e.g., 2000 = 20%)

### accumulatedUsdl

```solidity
mapping(address => uint256) accumulatedUsdl
```

Accumulated USDL per pool for buyback execution

### BuybackAccumulated

```solidity
event BuybackAccumulated(address pool, uint256 amount)
```

### BuybackExecuted

```solidity
event BuybackExecuted(address pool, uint256 usdlUsed)
```

### constructor

```solidity
constructor(address _poolRegistry, address _owner, uint256 _buybackBps) public
```

#### Parameters

| Name           | Type    | Description                                             |
| -------------- | ------- | ------------------------------------------------------- |
| \_poolRegistry | address | PoolRegistry address for pool validation                |
| \_owner        | address | Deployer/owner address                                  |
| \_buybackBps   | uint256 | Percentage of fees to redirect (in bps, max 5000 = 50%) |

### getFlags

```solidity
function getFlags() external pure returns (uint8)
```

### beforeFeeDistribution

```solidity
function beforeFeeDistribution(address pool, uint256 feeAmount) external returns (uint256 adjustedFee)
```

### getAccumulatedUsdl

```solidity
function getAccumulatedUsdl(address pool) external view returns (uint256)
```

Get the accumulated USDL available for buyback for a pool

### executeBuybackBurn

```solidity
function executeBuybackBurn(address pool, uint256 minTokensOut, uint256 deadline) external returns (uint256 tokensBurned)
```

Owner-only buyback for a registered pool attached to this optical. Claims accumulated optical
surplus and executes a pool buy with the supplied deadline and positive minimum output.
