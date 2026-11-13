# Solidity API

## PECOR

UUPS oracle-priced swap engine using PECORVault. It supports exact input, exact output, market and
native coin swaps. [PECOROrders](PECOROrders.md) manages limit, stop-loss and stop-limit orders.

The vault must grant PECOR `OPERATOR_ROLE`. Users approve the vault for token inputs. Exact input
swaps price the amount the vault actually receives; exact output swaps require receipt of the full
calculated input. A zero deadline means no expiry. Swap and collection methods require
`feesInitialized`.

`DEFAULT_ADMIN_ROLE` controls configuration, upgrades and unpausing. Default administration or
`PAUSER_ROLE` can pause. `setFeeCollector` rotates `FEE_COLLECTOR_ROLE`; collection sends reserved
fees to `feeCollector`.

Fees and price-impact deductions accrue in the output token and are reserved in the vault. Quotes
use available reserves after reservations. The base and two volume tiers sum to at most 200 basis
points; price impact is capped at 500 basis points. Tier thresholds are 10,000 and 100,000 USD,
scaled by `1e18`.

Initialization marks fee reservations ready. An upgraded proxy with `feesInitialized == false` must
be paused while default administration calls `migrateFeeReservations` in batches of 1 to 100
registered tokens. Trading and collection become available when all registered tokens have been
processed.

Tracker calls are optional and use a 500,000 gas limit. A failed configured call emits
`TrackerCallFailed`; swap settlement remains effective.

Source: [PECOR.sol](../../../contracts/meta-ag/engine/PECOR.sol).

### FEE_COLLECTOR_ROLE

```solidity
bytes32 FEE_COLLECTOR_ROLE
```

### PRECISION

```solidity
uint256 PRECISION
```

### BPS_DENOMINATOR

```solidity
uint256 BPS_DENOMINATOR
```

### MAX_FEE_BPS

```solidity
uint256 MAX_FEE_BPS
```

Absolute cap for fee stacking — swapFeeBps + tier1FeeBps + tier2FeeBps must never exceed this.

### MAX_IMPACT_BPS

```solidity
uint256 MAX_IMPACT_BPS
```

Maximum price impact applied to any single swap.

### TIER1_THRESHOLD

```solidity
uint256 TIER1_THRESHOLD
```

Swap volume tier thresholds (USD, 18-decimals fixed-point).

### TIER2_THRESHOLD

```solidity
uint256 TIER2_THRESHOLD
```

### ZeroAddress

```solidity
error ZeroAddress()
```

### ZeroAmount

```solidity
error ZeroAmount()
```

### SameToken

```solidity
error SameToken()
```

### Expired

```solidity
error Expired()
```

### InsufficientOutput

```solidity
error InsufficientOutput()
```

### ExcessiveInput

```solidity
error ExcessiveInput()
```

### InsufficientLiquidity

```solidity
error InsufficientLiquidity()
```

### NotAStablecoin

```solidity
error NotAStablecoin()
```

### TokenIsStablecoin

```solidity
error TokenIsStablecoin()
```

### UseWethDeposit

```solidity
error UseWethDeposit()
```

### UseWethWithdraw

```solidity
error UseWethWithdraw()
```

### NoFeesToCollect

```solidity
error NoFeesToCollect()
```

### FeeTooHigh

```solidity
error FeeTooHigh()
```

### Tier2BelowTier1

```solidity
error Tier2BelowTier1()
```

### ScalarTooHigh

```solidity
error ScalarTooHigh()
```

### NativeTransferFailed

```solidity
error NativeTransferFailed()
```

### MulticallFailed

```solidity
error MulticallFailed(uint256 index)
```

### priceOracle

```solidity
contract IPriceOracle priceOracle
```

### vault

```solidity
contract IPECORVault vault
```

### transactionTracker

```solidity
contract ITransactionTracker transactionTracker
```

### weth

```solidity
address weth
```

Wrapped native coin. Assigned once in {initialize}.

### swapFeeBps

```solidity
uint256 swapFeeBps
```

Base protocol fee in BPS, applied to every swap.

### tier1FeeBps

```solidity
uint256 tier1FeeBps
```

Additional fee in BPS for volumeUSD >= TIER1_THRESHOLD.

### tier2FeeBps

```solidity
uint256 tier2FeeBps
```

Additional fee in BPS for volumeUSD >= TIER2_THRESHOLD.

### priceImpactEnabled

```solidity
bool priceImpactEnabled
```

Toggle for price-impact deduction on net swap output.

### priceImpactScalarBps

```solidity
uint256 priceImpactScalarBps
```

Scalar used to scale impact by swap-to-reserve ratio.

### feeCollector

```solidity
address feeCollector
```

Destination for {collectFees}. Also holds FEE_COLLECTOR_ROLE.

### accruedFees

```solidity
mapping(address => uint256) accruedFees
```

Per-token accrued fees (tracked here; actual tokens live in the vault and are moved via
vault.pushTokens at collect time).

### ensure

```solidity
modifier ensure(uint256 deadline)
```

### constructor

```solidity
constructor() public
```

### initialize

```solidity
function initialize(address priceOracle_, address vault_, address weth_, address tracker_, address admin_) external
```

Initialize the UUPS proxy

#### Parameters

| Name          | Type    | Description |
| ------------- | ------- | ----------- |
| priceOracle\_ | address |             |
| vault\_       | address |             |
| weth\_        | address |             |
| tracker\_     | address |             |
| admin\_       | address |             |

### \_authorizeUpgrade

```solidity
function _authorizeUpgrade(address) internal
```

### setPriceOracle

```solidity
function setPriceOracle(address oracle) external
```

### setTransactionTracker

```solidity
function setTransactionTracker(address tracker) external
```

### setSwapFee

```solidity
function setSwapFee(uint256 feeBps) external
```

The sum of swapFeeBps, tier1FeeBps and tier2FeeBps must not exceed MAX_FEE_BPS.

### setTieredFees

```solidity
function setTieredFees(uint256 tier1FeeBps_, uint256 tier2FeeBps_) external
```

The sum of the base and tier fees must not exceed MAX_FEE_BPS, and tier2FeeBps must be at least
tier1FeeBps.

### setPriceImpact

```solidity
function setPriceImpact(bool enabled, uint256 scalarBps) external
```

### setFeeCollector

```solidity
function setFeeCollector(address collector) external
```

_Rotates FEE_COLLECTOR_ROLE from the previous collector to the new one._

### pause

```solidity
function pause() external
```

### unpause

```solidity
function unpause() external
```

### collectFees

```solidity
function collectFees(address token) external
```

_Moves accrued fees out of the vault into {feeCollector}. The vault treats PECOR as an OPERATOR_ROLE
holder, so this succeeds after vault.setOperator(PECOR, true)._

### swapExactIn

```solidity
function swapExactIn(address tokenIn, address tokenOut, uint256 amountIn, uint256 amountOutMin, uint256 deadline) external returns (uint256 amountOut)
```

### swapExactOut

```solidity
function swapExactOut(address tokenIn, address tokenOut, uint256 amountOut, uint256 amountInMax, uint256 deadline) external returns (uint256 amountIn)
```

### marketBuy

```solidity
function marketBuy(address stablecoin, address token, uint256 stablecoinAmount, uint256 minTokenAmount, uint256 deadline) external returns (uint256 tokenAmount)
```

### marketSell

```solidity
function marketSell(address token, address stablecoin, uint256 tokenAmount, uint256 minStablecoinAmount, uint256 deadline) external returns (uint256 stablecoinAmount)
```

### swapExactInNative

```solidity
function swapExactInNative(address tokenOut, uint256 amountOutMin, uint256 deadline) external payable returns (uint256 amountOut)
```

### swapExactInToNative

```solidity
function swapExactInToNative(address tokenIn, uint256 amountIn, uint256 amountOutMin, uint256 deadline) external returns (uint256 amountOut)
```

### getQuoteExactIn

```solidity
function getQuoteExactIn(address tokenIn, address tokenOut, uint256 amountIn) external view returns (uint256 amountOut)
```

### getQuoteExactOut

```solidity
function getQuoteExactOut(address tokenIn, address tokenOut, uint256 amountOut) external view returns (uint256 amountIn)
```

### getDetailedQuote

```solidity
function getDetailedQuote(address tokenIn, address tokenOut, uint256 amountIn) external view returns (uint256 grossOut, uint256 netOut, uint256 priceImpactBps, uint256 feeBps, uint256 feeAmount)
```

Full oracle-priced quote with price impact and fee breakdown.

#### Return Values

| Name           | Type    | Description                                     |
| -------------- | ------- | ----------------------------------------------- |
| grossOut       | uint256 | Raw amountOut before fee / impact.              |
| netOut         | uint256 | amountOut user actually receives.               |
| priceImpactBps | uint256 | Price impact applied (BPS).                     |
| feeBps         | uint256 | Effective fee rate after tiered stacking (BPS). |
| feeAmount      | uint256 | Fee deducted from grossOut.                     |

### multicall

```solidity
function multicall(bytes[] data) external returns (bytes[] results)
```

Batch multiple calls atomically via delegatecall.

Bubbles the first failing call’s revert data. An empty revert becomes `MulticallFailed(index)`.

### receive

```solidity
receive() external payable
```

Accept native transfers only from WETH unwraps initiated by PECOR.

### \_calculateSwapOutput

```solidity
function _calculateSwapOutput(address tokenIn, address tokenOut, uint256 amountIn, uint256 priceIn, uint256 priceOut) internal view returns (uint256)
```

_Compute gross output respecting token decimals and oracle prices._

### \_calculateSwapInput

```solidity
function _calculateSwapInput(address tokenIn, address tokenOut, uint256 amountOut, uint256 priceIn, uint256 priceOut) internal view returns (uint256)
```

_Compute required input for a target output, rounding up._

### \_applyFeeAndImpact

```solidity
function _applyFeeAndImpact(address token, uint256 grossAmount, uint256 volumeUSD, uint256 reserveUSD) internal returns (uint256 netAmount)
```

_Apply tiered fee + price impact, credit accrued fees._

### \_calcNetOutput

```solidity
function _calcNetOutput(uint256 grossAmount, uint256 volumeUSD, uint256 reserveUSD) internal view returns (uint256 netAmount)
```

_Pure view form of {\_applyFeeAndImpact} — no state or events._

### \_getEffectiveFeeBps

```solidity
function _getEffectiveFeeBps(uint256 volumeUSD) internal view returns (uint256)
```

Returns the fee rate for the input USD volume tier.

### \_recordTrade

```solidity
function _recordTrade(address user, address tokenIn, address tokenOut, uint256 amountIn, uint256 amountOut, uint256 volumeUSD) internal
```

_Gated call to the transaction tracker; silently skipped if unset. When set, the tracker enforces
EMITTER_ROLE on its side._

### \_recordMarketTrade

```solidity
function _recordMarketTrade(address user, address stablecoin, address token, uint256 stablecoinAmount, uint256 tokenAmount, bool isBuy, uint256 executionPrice) internal
```

_Gated market-order record; silently skipped if tracker unset._

### UnsupportedInputTransfer

```solidity
error UnsupportedInputTransfer()
```

### LiabilityMigrationRequired

```solidity
error LiabilityMigrationRequired()
```

### MigrationAlreadyComplete

```solidity
error MigrationAlreadyComplete()
```

### TrackerCallFailed

```solidity
event TrackerCallFailed(address indexed tracker, bytes4 indexed selector)
```

### liabilitiesInitialized

```solidity
modifier liabilitiesInitialized()
```

### \_quoteExactOut

```solidity
function _quoteExactOut(address tokenIn, address tokenOut, uint256 target, uint256 priceIn, uint256 priceOut) internal view returns (uint256)
```

### \_track

```solidity
function _track(bytes memory payload) internal
```

### liabilitiesReady

```solidity
function liabilitiesReady() external view returns (bool)
```

Returns feesInitialized.

### migrateFeeReservations

```solidity
function migrateFeeReservations(uint256 maxCount) external
```

### feesInitialized

```solidity
bool feesInitialized
```

### feeMigrationCursor

```solidity
uint256 feeMigrationCursor
```
