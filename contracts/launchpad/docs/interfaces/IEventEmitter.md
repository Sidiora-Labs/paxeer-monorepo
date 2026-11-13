# Solidity API

## IEventEmitter

[IEventEmitter.sol](../../contracts/interfaces/IEventEmitter.sol) declares the central event API;
[EventEmitter.sol](../../contracts/data/EventEmitter.sol) implements authorization and emission.
This reference accompanies the
[launchpad overview](../../README.md#data-and-fee-accounting) and
[operations runbook](../../operations/README.md).

These are EVM contract events on Paxeer mainnet-beta (chain ID 125 / `0x7d`). Read receipts and logs
through root-path node JSON-RPC. Receipts are final when their block is committed on the single BFT
network, whose kernel proposes blocks. Discover the hub proxy and source contracts in
`deployments/<network>-addresses.json`.

The hub is an optional mirror, with coverage determined by each producer and the deployed wiring.
Native source logs remain authoritative. Some producers catch mirror failures and emit
`MirrorFailed`; other configured hub calls propagate errors. Following only the hub can miss
activity. Most activity events append `block.timestamp` and `block.number` inside the emitter rather
than accepting those values from callers. `TypedEmitterPermission` is an administrative event
without those fields.

Payloads are event-specific. Core market, swap and pool-state events use `poolId`, encoded by
current callers as `bytes32(uint256(uint160(pool)))`. `TokenTransfer` carries `token`, `from`, `to`
and `value`, with no pool ID; use PoolRegistry to resolve pool metadata. Generic `EventData` has no
enforced application schema, so consumers must agree on keys and value types with their producer.

All emission entrypoints require `isAuthorizedEmitter(msg.sender)`. This generic gate admits
explicit allowlisted addresses, the wired pool registry, registered pools, approved opticals, the
wired Sidiora factory and locally registered tokens. Failed registry probes do not authorize a
caller. Holding `EVENT_EMITTER_ROLE` alone does not authorize emission.

Typed emission has additional checks. Most typed methods require a selector-specific permission set
by `setTypedEmitter`; a generic whitelist entry or optical approval alone is insufficient. Factory
creation methods also accept the wired factory. Pool swap/state/optical methods also accept
registered pool callers with a matching address-derived pool ID; the optical must match registry
metadata. Transfer, approval, upgrade and pause mirrors require the caller to be the asset, proxy or
paused contract named in the payload. Pool registration mirrors validate the wired registry and
stored metadata unless explicitly permitted for that selector. See the implementation for each
method’s exact gate.

`DEFAULT_ADMIN_ROLE` controls upgrades, role administration, pool-registry wiring and typed
permissions. `EVENT_EMITTER_ROLE` or default administration permits generic emitter authorization
and the optional wiring setters below. Neither role guarantees that all producer contracts have been
wired. For existing proxies, follow the
[upgrade and migration guidance](../../README.md#operator-safety-and-upgrade-qualification) before
changing implementation or permissions.

### Unauthorized

```solidity
error Unauthorized()
```

Raised when an emission gate, self-identity check or one-time migration guard fails. Administrative
role failures may instead use the inherited `MissingRole(address,bytes32)` error, which is not
declared by this interface.

### MarketCreated

```solidity
event MarketCreated(bytes32 indexed poolId, address indexed token, address indexed creator, address pool, address optical, uint256 timestamp, uint256 blockNumber)
```

### Swap

```solidity
event Swap(bytes32 indexed poolId, address indexed sender, bool isBuy, uint256 amountIn, uint256 amountOut, uint256 fee, uint256 price, uint256 timestamp, uint256 blockNumber)
```

### FeeRecorded

```solidity
event FeeRecorded(bytes32 indexed poolId, uint256 feeAmount, uint256 protocolCut, uint256 poolCut, uint256 timestamp, uint256 blockNumber)
```

### FeeDistributed

```solidity
event FeeDistributed(bytes32 indexed poolId, uint256 nftId, uint8 strategy, uint256 amount, address recipient, uint256 timestamp, uint256 blockNumber)
```

### FeeStrategyChanged

```solidity
event FeeStrategyChanged(bytes32 indexed poolId, uint256 nftId, uint8 oldStrategy, uint8 newStrategy, uint256 timestamp, uint256 blockNumber)
```

### OpticalExecuted

```solidity
event OpticalExecuted(bytes32 indexed poolId, address indexed optical, string hookName, bytes data, uint256 timestamp, uint256 blockNumber)
```

### PoolStateUpdated

```solidity
event PoolStateUpdated(bytes32 indexed poolId, uint256 virtualReserve, uint256 realReserve, uint256 tokenReserve, uint256 price, uint256 timestamp, uint256 blockNumber)
```

### ConfigUpdated

```solidity
event ConfigUpdated(bytes32 indexed key, uint256 oldValue, uint256 newValue, uint256 timestamp, uint256 blockNumber)
```

### emitMarketCreated

```solidity
function emitMarketCreated(bytes32 poolId, address token, address creator, address pool, address optical) external
```

### emitSwap

```solidity
function emitSwap(bytes32 poolId, address sender, bool isBuy, uint256 amountIn, uint256 amountOut, uint256 fee, uint256 price) external
```

### emitFeeRecorded

```solidity
function emitFeeRecorded(bytes32 poolId, uint256 feeAmount, uint256 protocolCut, uint256 poolCut) external
```

### emitFeeDistributed

```solidity
function emitFeeDistributed(bytes32 poolId, uint256 nftId, uint8 strategy, uint256 amount, address recipient) external
```

### emitFeeStrategyChanged

```solidity
function emitFeeStrategyChanged(bytes32 poolId, uint256 nftId, uint8 oldStrategy, uint8 newStrategy) external
```

### emitPoolStateUpdated

```solidity
function emitPoolStateUpdated(bytes32 poolId, uint256 virtualReserve, uint256 realReserve, uint256 tokenReserve, uint256 price) external
```

### emitOpticalExecuted

```solidity
function emitOpticalExecuted(bytes32 poolId, address optical, string hookName, bytes data) external
```

### emitConfigUpdated

```solidity
function emitConfigUpdated(bytes32 key, uint256 oldValue, uint256 newValue) external
```

### isAuthorizedEmitter

```solidity
function isAuthorizedEmitter(address emitter) external view returns (bool)
```

Returns the generic authorization result. It does not check a typed method’s selector-specific
permission or validate its payload.

### setAuthorizedEmitter

```solidity
function setAuthorizedEmitter(address emitter, bool authorized) external
```

Sets the explicit generic whitelist flag for a nonzero address. Requires default administration or
the emitter role. Revoking this flag does not revoke independent dynamic authorization or typed
permissions.

### AddressKeyValue

```solidity
struct AddressKeyValue {
  string key;
  address value;
}
```

### AddressArrayKeyValue

```solidity
struct AddressArrayKeyValue {
  string key;
  address[] value;
}
```

### UintKeyValue

```solidity
struct UintKeyValue {
  string key;
  uint256 value;
}
```

### UintArrayKeyValue

```solidity
struct UintArrayKeyValue {
  string key;
  uint256[] value;
}
```

### IntKeyValue

```solidity
struct IntKeyValue {
  string key;
  int256 value;
}
```

### IntArrayKeyValue

```solidity
struct IntArrayKeyValue {
  string key;
  int256[] value;
}
```

### BoolKeyValue

```solidity
struct BoolKeyValue {
  string key;
  bool value;
}
```

### BoolArrayKeyValue

```solidity
struct BoolArrayKeyValue {
  string key;
  bool[] value;
}
```

### Bytes32KeyValue

```solidity
struct Bytes32KeyValue {
  string key;
  bytes32 value;
}
```

### Bytes32ArrayKeyValue

```solidity
struct Bytes32ArrayKeyValue {
  string key;
  bytes32[] value;
}
```

### BytesKeyValue

```solidity
struct BytesKeyValue {
  string key;
  bytes value;
}
```

### BytesArrayKeyValue

```solidity
struct BytesArrayKeyValue {
  string key;
  bytes[] value;
}
```

### StringKeyValue

```solidity
struct StringKeyValue {
  string key;
  string value;
}
```

### StringArrayKeyValue

```solidity
struct StringArrayKeyValue {
  string key;
  string[] value;
}
```

### AddressItems

```solidity
struct AddressItems {
  struct IEventEmitter.AddressKeyValue[] items;
  struct IEventEmitter.AddressArrayKeyValue[] arrayItems;
}
```

### UintItems

```solidity
struct UintItems {
  struct IEventEmitter.UintKeyValue[] items;
  struct IEventEmitter.UintArrayKeyValue[] arrayItems;
}
```

### IntItems

```solidity
struct IntItems {
  struct IEventEmitter.IntKeyValue[] items;
  struct IEventEmitter.IntArrayKeyValue[] arrayItems;
}
```

### BoolItems

```solidity
struct BoolItems {
  struct IEventEmitter.BoolKeyValue[] items;
  struct IEventEmitter.BoolArrayKeyValue[] arrayItems;
}
```

### Bytes32Items

```solidity
struct Bytes32Items {
  struct IEventEmitter.Bytes32KeyValue[] items;
  struct IEventEmitter.Bytes32ArrayKeyValue[] arrayItems;
}
```

### BytesItems

```solidity
struct BytesItems {
  struct IEventEmitter.BytesKeyValue[] items;
  struct IEventEmitter.BytesArrayKeyValue[] arrayItems;
}
```

### StringItems

```solidity
struct StringItems {
  struct IEventEmitter.StringKeyValue[] items;
  struct IEventEmitter.StringArrayKeyValue[] arrayItems;
}
```

### EventData

```solidity
struct EventData {
  struct IEventEmitter.AddressItems addressItems;
  struct IEventEmitter.UintItems uintItems;
  struct IEventEmitter.IntItems intItems;
  struct IEventEmitter.BoolItems boolItems;
  struct IEventEmitter.Bytes32Items bytes32Items;
  struct IEventEmitter.BytesItems bytesItems;
  struct IEventEmitter.StringItems stringItems;
}
```

### EventLog

```solidity
event EventLog(address indexed msgSender, bytes32 indexed eventNameHash, string eventName, EventData eventData, uint256 timestamp, uint256 blockNumber)
```

Generic payload with caller provenance. For these non-anonymous events, log topic 0 is the ABI event
signature, topic 1 is indexed `msgSender`, and topic 2 is
`eventNameHash = keccak256(bytes(eventName))`. Filter the name hash at topic 2, not topic 1.

### EventLog1

```solidity
event EventLog1(address indexed msgSender, bytes32 indexed eventNameHash, bytes32 indexed topic1, string eventName, EventData eventData, uint256 timestamp, uint256 blockNumber)
```

Generic payload with caller and name hash, plus indexed `topic1` at log topic 3 (for example, a
user, pool or token identifier).

### EventLog2

```solidity
event EventLog2(address indexed msgSender, bytes32 indexed eventNameHash, bytes32 indexed topic1, bytes32 topic2, string eventName, EventData eventData, uint256 timestamp, uint256 blockNumber)
```

Generic payload with two application identifiers. Only `topic1` is indexed (log topic 3); `topic2`
is unindexed and must be decoded from event data. The caller and name hash occupy log topics 1
and 2.

### emitEventLog

```solidity
function emitEventLog(string eventName, struct IEventEmitter.EventData data) external
```

### emitEventLog1

```solidity
function emitEventLog1(string eventName, bytes32 topic1, struct IEventEmitter.EventData data) external
```

### emitEventLog2

```solidity
function emitEventLog2(string eventName, bytes32 topic1, bytes32 topic2, struct IEventEmitter.EventData data) external
```

### PecorSwap

```solidity
event PecorSwap(address indexed user, address indexed tokenIn, address indexed tokenOut, uint256 amountIn, uint256 amountOut, uint256 priceIn, uint256 priceOut, uint256 volumeUSD, uint256 feeBps, uint256 feeAmount, uint256 impactBps, uint8 swapKind, uint256 timestamp, uint256 blockNumber)
```

### PecorOrderCreated

```solidity
event PecorOrderCreated(uint256 indexed orderId, address indexed user, uint8 orderKind, uint8 orderType, address tokenIn, address tokenOut, uint256 amount, uint256 targetPrice, uint256 stopPrice, uint256 limitPrice, uint256 timestamp, uint256 blockNumber)
```

### PecorOrderLifecycle

```solidity
event PecorOrderLifecycle(uint256 indexed orderId, address indexed user, uint8 orderKind, uint8 phase, uint256 price, bytes payload, uint256 timestamp, uint256 blockNumber)
```

### BestRouteSwap

```solidity
event BestRouteSwap(address indexed user, address indexed tokenIn, address indexed tokenOut, bytes32 routeId, uint256 amountIn, uint256 amountOut, address[] hops, uint256 protocolFeeBps, uint256 timestamp, uint256 blockNumber)
```

### emitPecorSwap

```solidity
function emitPecorSwap(address user, address tokenIn, address tokenOut, uint256 amountIn, uint256 amountOut, uint256 priceIn, uint256 priceOut, uint256 volumeUSD, uint256 feeBps, uint256 feeAmount, uint256 impactBps, uint8 swapKind) external
```

### emitPecorOrderCreated

```solidity
function emitPecorOrderCreated(uint256 orderId, address user, uint8 orderKind, uint8 orderType, address tokenIn, address tokenOut, uint256 amount, uint256 targetPrice, uint256 stopPrice, uint256 limitPrice) external
```

### emitPecorOrderLifecycle

```solidity
function emitPecorOrderLifecycle(uint256 orderId, address user, uint8 orderKind, uint8 phase, uint256 price, bytes payload) external
```

### emitBestRouteSwap

```solidity
function emitBestRouteSwap(address user, address tokenIn, address tokenOut, bytes32 routeId, uint256 amountIn, uint256 amountOut, address[] hops, uint256 protocolFeeBps) external
```

### PriceUpdated

```solidity
event PriceUpdated(address indexed token, uint256 indexed roundId, address indexed relayer, uint256 price, uint256 confidence, bytes32 sourceId, uint256 timestamp, uint256 blockNumber)
```

### CircuitBreakerTriggered

```solidity
event CircuitBreakerTriggered(address indexed token, bytes32 indexed sourceId, uint256 reportedPrice, uint256 referencePrice, uint256 deviationBps, uint256 timestamp, uint256 blockNumber)
```

### OracleAdapterLifecycle

```solidity
event OracleAdapterLifecycle(bytes32 indexed sourceId, address indexed adapter, uint8 phase, uint256 priority, uint256 timestamp, uint256 blockNumber)
```

### emitPriceUpdated

```solidity
function emitPriceUpdated(address token, uint256 roundId, address relayer, uint256 price, uint256 confidence, bytes32 sourceId) external
```

### emitCircuitBreaker

```solidity
function emitCircuitBreaker(address token, bytes32 sourceId, uint256 reportedPrice, uint256 referencePrice, uint256 deviationBps) external
```

### emitOracleAdapterLifecycle

```solidity
function emitOracleAdapterLifecycle(bytes32 sourceId, address adapter, uint8 phase, uint256 priority) external
```

### VaultFlow

```solidity
event VaultFlow(uint8 indexed flowType, address indexed token, address indexed party, uint256 amount, uint256 newReserve, uint256 timestamp, uint256 blockNumber)
```

### emitVaultFlow

```solidity
function emitVaultFlow(uint8 flowType, address token, address party, uint256 amount, uint256 newReserve) external
```

### Governance

```solidity
event Governance(uint8 indexed action, bytes32 indexed id, address indexed actor, bytes payload, uint256 timestamp, uint256 blockNumber)
```

### emitGovernance

```solidity
function emitGovernance(uint8 action, bytes32 id, address actor, bytes payload) external
```

### TreasuryFlow

```solidity
event TreasuryFlow(uint8 indexed direction, address indexed token, address indexed party, uint256 amount, uint256 timestamp, uint256 blockNumber)
```

### emitTreasuryFlow

```solidity
function emitTreasuryFlow(uint8 direction, address token, address party, uint256 amount) external
```

### OpticalLifecycle

```solidity
event OpticalLifecycle(uint8 indexed action, address indexed optical, address indexed pool, bytes32 name, bytes payload, uint256 timestamp, uint256 blockNumber)
```

### emitOpticalLifecycle

```solidity
function emitOpticalLifecycle(uint8 action, address optical, address pool, bytes32 name, bytes payload) external
```

An authorized caller may emit for itself when `msg.sender == optical`. Emitting for another optical
also requires the selector-specific permission for this method.

### TokenTransfer

```solidity
event TokenTransfer(address indexed token, address indexed from, address indexed to, uint256 value, uint256 timestamp, uint256 blockNumber)
```

### NftTransfer

```solidity
event NftTransfer(address indexed nft, address indexed from, address indexed to, uint256 tokenId, uint256 timestamp, uint256 blockNumber)
```

### AssetApproval

```solidity
event AssetApproval(address indexed asset, address indexed owner, address indexed spender, uint256 valueOrTokenId, bool isNft, uint256 timestamp, uint256 blockNumber)
```

### emitTokenTransfer

```solidity
function emitTokenTransfer(address token, address from, address to, uint256 value) external
```

### emitNftTransfer

```solidity
function emitNftTransfer(address nft, address from, address to, uint256 tokenId) external
```

### emitAssetApproval

```solidity
function emitAssetApproval(address asset, address owner, address spender, uint256 valueOrTokenId, bool isNft) external
```

### RoleChange

```solidity
event RoleChange(uint8 indexed action, bytes32 indexed role, address indexed account, address sender, bytes32 previousAdminRole, uint256 timestamp, uint256 blockNumber)
```

### ContractUpgraded

```solidity
event ContractUpgraded(address indexed proxy, address indexed newImplementation, uint8 kind, uint256 timestamp, uint256 blockNumber)
```

### PauseToggle

```solidity
event PauseToggle(address indexed pausedContract, bool paused, uint256 timestamp, uint256 blockNumber)
```

### emitRoleChange

```solidity
function emitRoleChange(uint8 action, bytes32 role, address account, address sender, bytes32 previousAdminRole) external
```

### emitUpgraded

```solidity
function emitUpgraded(address proxy, address newImplementation, uint8 kind) external
```

### emitPauseToggle

```solidity
function emitPauseToggle(address pausedContract, bool paused) external
```

### FeeFlow

```solidity
event FeeFlow(uint8 indexed kind, address indexed pool, address indexed party, uint256 amount, uint256 protocolCut, uint256 poolCut, uint256 epoch, uint256 timestamp, uint256 blockNumber)
```

Typed channel for FeeAccumulator's mirrored fee activity. The implementation uses best-effort
emission for this channel; native fee events and liabilities remain authoritative. For example,
`AirdropExpired` is a native event without a corresponding `FeeFlow` emission in the current
implementation.

### RouterTrade

```solidity
event RouterTrade(uint8 indexed kind, address indexed pool, address indexed sender, address tokenIn, address tokenOut, uint256 amountIn, uint256 amountOut, uint256 intermediateUsdl, uint256 timestamp, uint256 blockNumber)
```

### NftMint

```solidity
event NftMint(uint256 indexed tokenId, address indexed creator, address indexed pool, uint8 strategy, uint256 timestamp, uint256 blockNumber)
```

### PoolRegistered

```solidity
event PoolRegistered(address indexed pool, address indexed token, address indexed creator, address optical, uint256 nftId, uint256 timestamp, uint256 blockNumber)
```

### TokenDeployed

```solidity
event TokenDeployed(address indexed token, address indexed pool, address indexed creator, bytes32 salt, string name, string symbol, uint8 decimals, uint256 totalSupply, uint256 timestamp, uint256 blockNumber)
```

### emitFeeFlow

```solidity
function emitFeeFlow(uint8 kind, address pool, address party, uint256 amount, uint256 protocolCut, uint256 poolCut, uint256 epoch) external
```

### emitRouterTrade

```solidity
function emitRouterTrade(uint8 kind, address pool, address sender, address tokenIn, address tokenOut, uint256 amountIn, uint256 amountOut, uint256 intermediateUsdl) external
```

### emitNftMint

```solidity
function emitNftMint(uint256 tokenId, address creator, address pool, uint8 strategy) external
```

### emitPoolRegistered

```solidity
function emitPoolRegistered(address pool, address token, address creator, address optical, uint256 nftId) external
```

### emitTokenDeployed

```solidity
function emitTokenDeployed(address token, address pool, address creator, bytes32 salt, string name, string symbol, uint8 decimals, uint256 totalSupply) external
```

### TypedEmitterPermission

```solidity
event TypedEmitterPermission(address indexed emitter, bytes4 indexed selector, bool allowed)
```

Emitted when default administration changes a selector-specific permission. This administrative
event has no timestamp or block-number fields; use receipt metadata.

### setTypedEmitter

```solidity
function setTypedEmitter(address emitter, bytes4 selector, bool allowed) external
```

Requires `DEFAULT_ADMIN_ROLE` and a nonzero emitter. The selector identifies the `emit...` function
being permitted, not the emitted event’s topic hash. The generic emission gate still applies, so
configure and verify both paths for a producer.

### isTypedEmitter

```solidity
function isTypedEmitter(address emitter, bytes4 selector) external view returns (bool)
```

Returns the explicit selector permission. It does not evaluate generic or method-specific
authorization.

### VERSION

```solidity
function VERSION() external pure returns (string)
```

The current implementation returns `"2.0.0"`. This is a version label, not proof of a particular
deployed runtime or permission configuration.

### EVENT_EMITTER_ROLE

```solidity
function EVENT_EMITTER_ROLE() external view returns (bytes32)
```

`keccak256("EVENT_EMITTER_ROLE")`. Allows generic emitter authorization and optional
registry/factory wiring, but does not grant upgrade authority, role administration or
selector-specific typed permissions. Those remain with `DEFAULT_ADMIN_ROLE`.

### setOpticalRegistry

```solidity
function setOpticalRegistry(address registry) external
```

Sets the optical registry used by the generic authorization gate. Requires `DEFAULT_ADMIN_ROLE` or
`EVENT_EMITTER_ROLE`. This setter can be called again to change or clear the pointer; it is not a
one-time initializer.

### setMetaAGRouter

```solidity
function setMetaAGRouter(address router) external
```

Stores the Meta-AG contract address. The current `_isAuthorized` implementation does not consult
this pointer or authorize its adapters; generic and typed permissions must be configured separately.
Requires default administration or the emitter role.

### setSidioraFactory

```solidity
function setSidioraFactory(address factory) external
```

Sets the factory identity used by generic authorization and factory-specific creation checks.
Requires default administration or the emitter role.

### setTokenRegistry

```solidity
function setTokenRegistry(address registry) external
```

Compatibility setter with a role check only. The current implementation does not store or consult an
external token-registry pointer; it uses the local token mapping. Requires default administration or
the emitter role.

### registerToken

```solidity
function registerToken(address token, address pool) external
```

Registers a nonzero token in the local generic authorization mapping. The wired factory,
`DEFAULT_ADMIN_ROLE` or `EVENT_EMITTER_ROLE` may call it. The `pool` argument is currently ignored;
no token/pool binding is stored here. Registration does not add transfer mirrors to a token that
does not call the hub.

### deregisterToken

```solidity
function deregisterToken(address token) external
```

Clears local token registration. Requires default administration or the emitter role. Other
independent authorization paths, such as the explicit allowlist, can still authorize that address.

### isRegisteredToken

```solidity
function isRegisteredToken(address token) external view returns (bool)
```

Returns the local token-registration flag, not the result of the full authorization gate.

### reinitializeV2

```solidity
function reinitializeV2(address adminWithEmitterRole) external
```

One-time migration on an initialized proxy, callable only by `DEFAULT_ADMIN_ROLE`. It sets the v2
migration flag, sets the administrator of `EVENT_EMITTER_ROLE` to `DEFAULT_ADMIN_ROLE`, and grants
the emitter role to `adminWithEmitterRole` unless that address is zero. A second call reverts with
`Unauthorized()`; it is not idempotent. A caller without the default admin role fails the inherited
`MissingRole(address,bytes32)` check.

Reconcile the recorded receipt and role state before retrying an ambiguous migration transaction. Do
not schedule this call again merely because a deployment or upgrade is being resumed.
