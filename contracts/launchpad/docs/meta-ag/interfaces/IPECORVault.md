# Solidity API

## IPECORVault

Interface for the UUPS multi-asset PECORVault. Registered tokens hold reserves for oracle-priced
swaps. `OPERATOR_ROLE` gates token pulls, pushes, native withdrawals and reservation changes.
`DEFAULT_ADMIN_ROLE` controls token registration, operator membership and upgrades.

To use USDL in vault swaps, register it with `registerToken(USDL, true)` before supplying USDL
liquidity. Reservations protect order escrow and accrued fees. Use `getAvailableReserves` for
spendable liquidity and `getReservedBalance` for total reservations; `getReserves` reports the
accounted reserve before subtracting reservations.

Source: [IPECORVault.sol](../../../contracts/meta-ag/interfaces/IPECORVault.sol).

### TokenInfo

```solidity
struct TokenInfo {
  bool isRegistered;
  bool isStablecoin;
  uint8 decimals;
  uint256 reserves;
  uint256 totalDeposited;
  uint256 totalWithdrawn;
}
```

### initialize

```solidity
function initialize(address weth, address tracker, address admin) external
```

Initialize the UUPS proxy

#### Parameters

| Name    | Type    | Description                                            |
| ------- | ------- | ------------------------------------------------------ |
| weth    | address | Wrapped native coin address (WPAX on Paxeer)           |
| tracker | address | ITransactionTracker address (may be zero at bootstrap) |
| admin   | address | Timelock address — granted DEFAULT_ADMIN_ROLE          |

### registerToken

```solidity
function registerToken(address token, bool isStablecoin) external
```

### setStablecoinStatus

```solidity
function setStablecoinStatus(address token, bool isStablecoin) external
```

### setOperator

```solidity
function setOperator(address operator, bool authorized) external
```

### setTransactionTracker

```solidity
function setTransactionTracker(address tracker) external
```

### emergencyWithdraw

```solidity
function emergencyWithdraw(address token, uint256 amount, address recipient) external
```

### syncReserves

```solidity
function syncReserves(address token) external
```

### syncAllReserves

```solidity
function syncAllReserves() external
```

### deposit

```solidity
function deposit(address token, uint256 amount) external
```

Donate inventory to the vault. The credited amount is measured on receipt; deposits create no
depositor shares or withdrawal entitlement.

### depositBatch

```solidity
function depositBatch(address[] tokenList, uint256[] amounts) external
```

### depositNative

```solidity
function depositNative() external payable
```

Wrap native coin into WETH and credit reserves (WETH must be registered).

### pullTokens

```solidity
function pullTokens(address token, address from, uint256 amount) external returns (uint256 actualAmount)
```

Pull tokens from `from` into the vault (fee-on-transfer safe).

#### Return Values

| Name         | Type    | Description                                         |
| ------------ | ------- | --------------------------------------------------- |
| actualAmount | uint256 | Amount actually received after any token-side fees. |

### pushTokens

```solidity
function pushTokens(address token, address to, uint256 amount) external
```

Transfer spendable inventory to the recipient. The vault's balance must decrease by exactly
`amount` and the recipient's balance must increase by exactly `amount`; otherwise the call reverts.

### withdrawNative

```solidity
function withdrawNative(uint256 amount, address to) external
```

Unwrap WETH to native coin and forward to recipient.

### updateReserves

```solidity
function updateReserves(address tokenIn, uint256 amountIn, address tokenOut, uint256 amountOut) external
```

The implementation reverts with DeprecatedBookMutation after its operator check. Token transfers and
reserve synchronization maintain the ledger.

### getReserves

```solidity
function getReserves(address token) external view returns (uint256)
```

### getTokenInfo

```solidity
function getTokenInfo(address token) external view returns (bool isRegistered, bool isStablecoin, uint8 decimals, uint256 reserves, uint256 totalDeposited, uint256 totalWithdrawn)
```

Returns registration, stablecoin status, decimals and reserve totals as a flat tuple.

### isStablecoin

```solidity
function isStablecoin(address token) external view returns (bool)
```

### getTokenDecimals

```solidity
function getTokenDecimals(address token) external view returns (uint8)
```

### getRegisteredTokens

```solidity
function getRegisteredTokens() external view returns (address[])
```

### getRegisteredStablecoins

```solidity
function getRegisteredStablecoins() external view returns (address[])
```

### getRegisteredTokenCount

```solidity
function getRegisteredTokenCount() external view returns (uint256)
```

### hasLiquidity

```solidity
function hasLiquidity(address token, uint256 amount) external view returns (bool)
```

### getAllReserves

```solidity
function getAllReserves() external view returns (address[] memory, uint256[] memory)
```

### getUntrackedFunds

```solidity
function getUntrackedFunds(address token) external view returns (uint256)
```

### TokenRegistered

```solidity
event TokenRegistered(address indexed token, uint8 decimals, bool isStablecoin)
```

### StablecoinStatusUpdated

```solidity
event StablecoinStatusUpdated(address indexed token, bool isStablecoin)
```

### Deposit

```solidity
event Deposit(address indexed token, address indexed depositor, uint256 amount, uint256 newReserve)
```

### Withdrawal

```solidity
event Withdrawal(address indexed token, address indexed recipient, uint256 amount, uint256 newReserve)
```

### NativeDeposit

```solidity
event NativeDeposit(address indexed depositor, uint256 amount, uint256 newReserve)
```

### NativeWithdrawal

```solidity
event NativeWithdrawal(address indexed recipient, uint256 amount, uint256 newReserve)
```

### OperatorUpdated

```solidity
event OperatorUpdated(address indexed operator, bool authorized)
```

### TransactionTrackerUpdated

```solidity
event TransactionTrackerUpdated(address indexed tracker)
```

### ReservesUpdated

```solidity
event ReservesUpdated(address indexed token, uint256 oldReserve, uint256 newReserve)
```

### ReservesSync

```solidity
event ReservesSync(address indexed token, uint256 oldReserves, uint256 newReserves, uint256 recovered)
```

### EmergencyWithdraw

```solidity
event EmergencyWithdraw(address indexed token, address indexed recipient, uint256 amount, uint256 newReserve)
```

### reserveTokens

```solidity
function reserveTokens(address token, uint256 amount) external
```

Requires OPERATOR_ROLE and sufficient spendable reserves. Adds a reservation owned by the caller.

### releaseTokens

```solidity
function releaseTokens(address token, uint256 amount) external
```

Requires OPERATOR_ROLE, initialized liabilities and enough of the caller’s own reservations.
Releases the caller’s reservation without transferring tokens.

### getAvailableReserves

```solidity
function getAvailableReserves(address token) external view returns (uint256)
```

Returns zero during liability initialization or token quarantine. Otherwise returns the smaller of
live custody and accounted reserves, less reservations, floored at zero.

### getReservedBalance

```solidity
function getReservedBalance(address token) external view returns (uint256)
```

Returns total reserved liabilities for the token across operators.
