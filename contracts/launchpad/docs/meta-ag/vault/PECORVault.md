# Solidity API

Source: [PECORVault.sol](../../../contracts/meta-ag/vault/PECORVault.sol).

## PECORVault

UUPS-upgradeable multi-asset vault with open deposits and operator-authorized transfers and
reservations. Administrators register tokens, manage operators, reconcile reserves, quarantine
tokens and authorize upgrades.

Deposits measure actual received balances, including transfer fees. Outbound token transfers require
exact vault and recipient balance changes. Available reserves are the smaller of custody and
recorded reserves, minus reserved liabilities; quarantined tokens report no available reserves.
Native deposits and withdrawals wrap and unwrap the configured wrapped-native token.

### OPERATOR_ROLE

```solidity
bytes32 OPERATOR_ROLE
```

### ZeroAddress

```solidity
error ZeroAddress()
```

### ZeroAmount

```solidity
error ZeroAmount()
```

### InvalidArrayLength

```solidity
error InvalidArrayLength()
```

### TokenAlreadyRegistered

```solidity
error TokenAlreadyRegistered()
```

### TokenNotRegistered

```solidity
error TokenNotRegistered()
```

### ReservesMismatch

```solidity
error ReservesMismatch()
```

### NativeTransferFailed

```solidity
error NativeTransferFailed()
```

### WethOnly

```solidity
error WethOnly()
```

### weth

```solidity
address weth
```

Canonical wrapped-native token. Semantically immutable — assigned exactly once in `initialize`,
never mutated afterwards.

### transactionTracker

```solidity
address transactionTracker
```

Configured transaction-tracker address. May be zero; administrators update it with
`setTransactionTracker`.

### authorizedOperators

```solidity
mapping(address => bool) authorizedOperators
```

Mirror of `OPERATOR_ROLE` membership. Kept for O(1) view by integrations that prefer a boolean over
`hasRole` staticcalls.

### constructor

```solidity
constructor() public
```

### initialize

```solidity
function initialize(address weth_, address tracker_, address admin_) external
```

Initialize the UUPS proxy

#### Parameters

| Name      | Type    | Description |
| --------- | ------- | ----------- |
| weth\_    | address |             |
| tracker\_ | address |             |
| admin\_   | address |             |

### \_authorizeUpgrade

```solidity
function _authorizeUpgrade(address) internal
```

### registerToken

```solidity
function registerToken(address token, bool isStablecoin_) external
```

### setStablecoinStatus

```solidity
function setStablecoinStatus(address token, bool isStablecoin_) external
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

### withdrawNative

```solidity
function withdrawNative(uint256 amount, address to) external
```

Unwrap WETH to native coin and forward to recipient.

### updateReserves

```solidity
function updateReserves(address tokenIn, uint256 amountIn, address tokenOut, uint256 amountOut) external
```

Reverts with `DeprecatedBookMutation` for authorized operator calls. Reserve accounting follows
actual token movements and reconciliation.

### getReserves

```solidity
function getReserves(address token) external view returns (uint256)
```

### getTokenInfo

```solidity
function getTokenInfo(address token) external view returns (bool, bool, uint8, uint256, uint256, uint256)
```

Returns `(isRegistered, isStablecoin, decimals, reserves, totalDeposited, totalWithdrawn)` in that
order.

### isStablecoin

```solidity
function isStablecoin(address token) external view returns (bool)
```

### getTokenDecimals

```solidity
function getTokenDecimals(address token) external view returns (uint8)
```

### hasLiquidity

```solidity
function hasLiquidity(address token, uint256 amount) external view returns (bool)
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

### getAllReserves

```solidity
function getAllReserves() external view returns (address[] tokens, uint256[] reserves)
```

### getUntrackedFunds

```solidity
function getUntrackedFunds(address token) external view returns (uint256)
```

### receive

```solidity
receive() external payable
```

Accepts native transfers only from the configured wrapped-native contract. Other senders revert with
`WethOnly`.

### \_pullAndMeasure

```solidity
function _pullAndMeasure(address token, address from, uint256 amount) internal returns (uint256 received)
```

_Pull tokens via `transferFrom` and return the actually credited amount. Handles fee-on-transfer
tokens by comparing the vault's balance delta across the call._

### \_syncOne

```solidity
function _syncOne(address token) internal
```

Reconciles recorded reserves with custody. Positive deltas also increase `totalDeposited`. A deficit
reduces reserves only when custody still covers reserved liabilities. No change on parity.

### \_removeStablecoin

```solidity
function _removeStablecoin(address token) internal
```

_Compact 1-indexed removal from `_registeredStablecoins`._

### \_fetchDecimals

```solidity
function _fetchDecimals(address token) internal view returns (uint8)
```

Requires a token contract with a successful 32-byte `decimals()` response in the range 0–36;
otherwise reverts with `UnsupportedToken`.

### reserveTokens

```solidity
function reserveTokens(address token, uint256 amount) external
```

Operators reserve available tokens against their own reservation balance.

### releaseTokens

```solidity
function releaseTokens(address token, uint256 amount) external
```

Operators release their own reservations when custody covers the reserved liabilities.

### getAvailableReserves

```solidity
function getAvailableReserves(address token) external view returns (uint256)
```

Returns spendable custody after reservations, or zero while quarantined or liabilities are not
initialized.

### getReservedBalance

```solidity
function getReservedBalance(address token) external view returns (uint256)
```

Returns the total reservation balance for the token.

### operatorReservations

```solidity
mapping(address => mapping(address => uint256)) operatorReservations
```

Reservations by operator and token.

### quarantined

```solidity
mapping(address => bool) quarantined
```

Per-token quarantine status.

### liabilitiesInitialized

```solidity
bool liabilitiesInitialized
```

Whether liability accounting is initialized.

### setTokenQuarantine

```solidity
function setTokenQuarantine(address token, bool value) external
```

Administrators set quarantine for a registered token, blocking deposits and new reservations.

### syncReservesSubset

```solidity
function syncReservesSubset(address[] tokens) external
```

Administrators reconcile up to 100 tokens in one call.

### getRegisteredTokensPage

```solidity
function getRegisteredTokensPage(uint256 cursor, uint256 count) external view returns (address[] page, uint256 next)
```

Returns up to 100 registered tokens and the next cursor.

### recoverNativeSurplus

```solidity
function recoverNativeSurplus(address payable recipient) external
```

Administrators transfer the vault's native balance to a nonzero recipient.

### finalizeLiabilityMigration

```solidity
function finalizeLiabilityMigration(address[] liabilityOperators) external
```

Initializes liability accounting once after checking each supplied operator's role and
`liabilitiesReady()` response. Fresh vault initialization sets this state directly.
