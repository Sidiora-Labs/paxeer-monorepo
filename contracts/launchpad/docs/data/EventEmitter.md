# Solidity API

Source: [EventEmitter.sol](../../contracts/data/EventEmitter.sol).

## IOpticalRegistryMinimal

Minimal interface for the optional optical registry auth path.

### isApproved

```solidity
function isApproved(address optical) external view returns (bool)
```

## IMetaAGRouterMinimal

Declares the adapter-status lookup keyed by `bytes32 adapterId`. Generic router authorization
uses `setAuthorizedEmitter`.

### isAdapterActive

```solidity
function isAdapterActive(bytes32 adapterId) external view returns (bool)
```

## EventEmitter

Optional protocol event mirror for launchpad, Meta-AG/PECOR, opticals, oracle, treasury, governance
and contract lifecycle events. Source-contract logs remain authoritative. The UUPS implementation
reports `VERSION()` as `"2.0.0"`.

Generic log authorization accepts explicitly authorized emitters, the pool registry, registered
pools, approved opticals, the configured factory and registered pool tokens. Typed events also
check selector permissions or caller identity for the event being emitted.
Generic logs include caller provenance. `DEFAULT_ADMIN_ROLE` configures typed permissions and
authorizes upgrades; `EVENT_EMITTER_ROLE` manages supported emitter and registry settings.

### EMITTER_ADMIN_ROLE

```solidity
bytes32 EMITTER_ADMIN_ROLE
```

This constant grants no emitter permission; authorization uses the permissions described above.

### EVENT_EMITTER_ROLE

```solidity
bytes32 EVENT_EMITTER_ROLE
```

Authorizes emitter and registry settings where accepted by the setter. `setPoolRegistry` and
`setTypedEmitter` require `DEFAULT_ADMIN_ROLE`; `EVENT_EMITTER_ROLE` does not grant upgrade
authority.

### poolRegistry

```solidity
address poolRegistry
```

_slot 2._

### opticalRegistry

```solidity
address opticalRegistry
```

_slot 3._

### metaAGRouter

```solidity
address metaAGRouter
```

_slot 4._

### sidioraFactory

```solidity
address sidioraFactory
```

_slot 5._

### constructor

```solidity
constructor()
```

### initialize

```solidity
function initialize(address _admin) external initializer
```

Initializes default administration once. Call `reinitializeV2` to configure the emitter role.

### reinitializeV2

```solidity
function reinitializeV2(address adminWithEmitterRole) external override onlyRole(DEFAULT_ADMIN_ROLE)
```

Configures the emitter-role administrator and optionally grants the emitter role. Callable only by
`DEFAULT_ADMIN_ROLE`, once. Repeated calls revert with `Unauthorized()`.

#### Parameters

| Name                 | Type    | Description                                                                                                                                |
| -------------------- | ------- | ------------------------------------------------------------------------------------------------------------------------------------------ |
| adminWithEmitterRole | address | Address granted EVENT_EMITTER_ROLE. Pass `address(0)` to skip role grant (admin can do it later via `grantRole(EVENT_EMITTER_ROLE, ...)`). |

### \_authorizeUpgrade

```solidity
function _authorizeUpgrade(address) internal override onlyRole(DEFAULT_ADMIN_ROLE)
```

### onlyAuthorized

```solidity
modifier onlyAuthorized()
```

### isAuthorizedEmitter

```solidity
function isAuthorizedEmitter(address emitter) external view override returns (bool)
```

Public auth-check for off-chain probes / dependent contracts.

### isRegisteredToken

```solidity
function isRegisteredToken(address token) external view override returns (bool)
```

### \_isAuthorized

```solidity
function _isAuthorized(address sender) internal view returns (bool)
```

### setPoolRegistry

```solidity
function setPoolRegistry(address _poolRegistry) external onlyRole(DEFAULT_ADMIN_ROLE)
```

Sets the pool registry; requires `DEFAULT_ADMIN_ROLE`.

### setAuthorizedEmitter

```solidity
function setAuthorizedEmitter(address emitter, bool authorized) external override
```

Sets generic emitter authorization. Requires `EVENT_EMITTER_ROLE` or `DEFAULT_ADMIN_ROLE`; the
emitter must be nonzero.

### setOpticalRegistry

```solidity
function setOpticalRegistry(address registry) external override
```

Sets the optical registry. Requires `EVENT_EMITTER_ROLE` or `DEFAULT_ADMIN_ROLE`.

### setMetaAGRouter

```solidity
function setMetaAGRouter(address router) external override
```

### setSidioraFactory

```solidity
function setSidioraFactory(address factory) external override
```

### setTokenRegistry

```solidity
function setTokenRegistry(address ) external override
```

Checks emitter-management authority without storing an external registry. Token registration uses
the local mapping through `registerToken`.

### registerToken

```solidity
function registerToken(address token, address ) external override
```

Marks a nonzero token in the local authorization mapping. Callable by the configured factory or
an account with `EVENT_EMITTER_ROLE` or `DEFAULT_ADMIN_ROLE`.

### deregisterToken

```solidity
function deregisterToken(address token) external override
```

### VERSION

```solidity
function VERSION() external pure override returns (string memory)
```

Semantic version string of the deployed implementation.

### emitMarketCreated

```solidity
function emitMarketCreated( bytes32 poolId, address token, address creator, address pool, address optical ) external override onlyAuthorized
```

### emitSwap

```solidity
function emitSwap( bytes32 poolId, address sender, bool isBuy, uint256 amountIn, uint256 amountOut, uint256 fee, uint256 price ) external override onlyAuthorized
```

### emitFeeRecorded

```solidity
function emitFeeRecorded( bytes32 poolId, uint256 feeAmount, uint256 protocolCut, uint256 poolCut ) external override onlyAuthorized
```

### emitFeeDistributed

```solidity
function emitFeeDistributed( bytes32 poolId, uint256 nftId, uint8 strategy, uint256 amount, address recipient ) external override onlyAuthorized
```

### emitFeeStrategyChanged

```solidity
function emitFeeStrategyChanged( bytes32 poolId, uint256 nftId, uint8 oldStrategy, uint8 newStrategy ) external override onlyAuthorized
```

### emitPoolStateUpdated

```solidity
function emitPoolStateUpdated( bytes32 poolId, uint256 virtualReserve, uint256 realReserve, uint256 tokenReserve, uint256 price ) external override onlyAuthorized
```

### emitOpticalExecuted

```solidity
function emitOpticalExecuted( bytes32 poolId, address optical, string calldata hookName, bytes calldata data ) external override onlyAuthorized
```

### emitConfigUpdated

```solidity
function emitConfigUpdated( bytes32 key, uint256 oldValue, uint256 newValue ) external override onlyAuthorized
```

### emitEventLog

```solidity
function emitEventLog( string calldata eventName, EventData calldata data ) external override onlyAuthorized
```

### emitEventLog1

```solidity
function emitEventLog1( string calldata eventName, bytes32 topic1, EventData calldata data ) external override onlyAuthorized
```

### emitEventLog2

```solidity
function emitEventLog2( string calldata eventName, bytes32 topic1, bytes32 topic2, EventData calldata data ) external override onlyAuthorized
```

### emitPecorSwap

```solidity
function emitPecorSwap( address user, address tokenIn, address tokenOut, uint256 amountIn, uint256 amountOut, uint256 priceIn, uint256 priceOut, uint256 volumeUSD, uint256 feeBps, uint256 feeAmount, uint256 impactBps, uint8 swapKind ) external override onlyAuthorized
```

### emitPecorOrderCreated

```solidity
function emitPecorOrderCreated( uint256 orderId, address user, uint8 orderKind, uint8 orderType, address tokenIn, address tokenOut, uint256 amount, uint256 targetPrice, uint256 stopPrice, uint256 limitPrice ) external override onlyAuthorized
```

### emitPecorOrderLifecycle

```solidity
function emitPecorOrderLifecycle( uint256 orderId, address user, uint8 orderKind, uint8 phase, uint256 price, bytes calldata payload ) external override onlyAuthorized
```

### emitBestRouteSwap

```solidity
function emitBestRouteSwap( address user, address tokenIn, address tokenOut, bytes32 routeId, uint256 amountIn, uint256 amountOut, address[] calldata hops, uint256 protocolFeeBps ) external override onlyAuthorized
```

### emitPriceUpdated

```solidity
function emitPriceUpdated( address token, uint256 roundId, address relayer, uint256 price, uint256 confidence, bytes32 sourceId ) external override onlyAuthorized
```

### emitCircuitBreaker

```solidity
function emitCircuitBreaker( address token, bytes32 sourceId, uint256 reportedPrice, uint256 referencePrice, uint256 deviationBps ) external override onlyAuthorized
```

### emitOracleAdapterLifecycle

```solidity
function emitOracleAdapterLifecycle( bytes32 sourceId, address adapter, uint8 phase, uint256 priority ) external override onlyAuthorized
```

### emitVaultFlow

```solidity
function emitVaultFlow( uint8 flowType, address token, address party, uint256 amount, uint256 newReserve ) external override onlyAuthorized
```

### emitGovernance

```solidity
function emitGovernance( uint8 action, bytes32 id, address actor, bytes calldata payload ) external override onlyAuthorized
```

### emitTreasuryFlow

```solidity
function emitTreasuryFlow( uint8 direction, address token, address party, uint256 amount ) external override onlyAuthorized
```

### emitOpticalLifecycle

```solidity
function emitOpticalLifecycle( uint8 action, address optical, address pool, bytes32 name, bytes calldata payload ) external override onlyAuthorized
```

### emitTokenTransfer

```solidity
function emitTokenTransfer( address token, address from, address to, uint256 value ) external override onlyAuthorized
```

### emitNftTransfer

```solidity
function emitNftTransfer( address nft, address from, address to, uint256 tokenId ) external override onlyAuthorized
```

### emitAssetApproval

```solidity
function emitAssetApproval( address asset, address owner, address spender, uint256 valueOrTokenId, bool isNft ) external override onlyAuthorized
```

### emitRoleChange

```solidity
function emitRoleChange( uint8 action, bytes32 role, address account, address sender, bytes32 previousAdminRole ) external override onlyAuthorized
```

### emitUpgraded

```solidity
function emitUpgraded( address proxy, address newImplementation, uint8 kind ) external override onlyAuthorized
```

### emitPauseToggle

```solidity
function emitPauseToggle( address pausedContract, bool paused ) external override onlyAuthorized
```

### emitFeeFlow

```solidity
function emitFeeFlow( uint8 kind, address pool, address party, uint256 amount, uint256 protocolCut, uint256 poolCut, uint256 epoch ) external override onlyAuthorized
```

### emitRouterTrade

```solidity
function emitRouterTrade( uint8 kind, address pool, address sender, address tokenIn, address tokenOut, uint256 amountIn, uint256 amountOut, uint256 intermediateUsdl ) external override onlyAuthorized
```

### emitNftMint

```solidity
function emitNftMint( uint256 tokenId, address creator, address pool, uint8 strategy ) external override onlyAuthorized
```

### emitPoolRegistered

```solidity
function emitPoolRegistered( address pool, address token, address creator, address optical, uint256 nftId ) external override onlyAuthorized
```

### emitTokenDeployed

```solidity
function emitTokenDeployed( address token, address pool, address creator, bytes32 salt, string calldata name, string calldata symbol, uint8 decimals, uint256 totalSupply ) external override onlyAuthorized
```

### setTypedEmitter

```solidity
function setTypedEmitter(address emitter, bytes4 selector, bool allowed) external override onlyRole(DEFAULT_ADMIN_ROLE)
```

Sets a selector-specific permission and emits `TypedEmitterPermission`.

### isTypedEmitter

```solidity
function isTypedEmitter(address emitter, bytes4 selector) external view returns (bool)
```

Reads the explicit selector permission.
