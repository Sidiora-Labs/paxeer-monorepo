# Solidity API

Source: [MockEventEmitter.sol](../../contracts/test/MockEventEmitter.sol).

## MockEventEmitter

Simplified mock EventEmitter for testing contracts that emit events through it

### lastConfigKey

```solidity
bytes32 lastConfigKey
```

### lastConfigOldValue

```solidity
uint256 lastConfigOldValue
```

### lastConfigNewValue

```solidity
uint256 lastConfigNewValue
```

### configUpdateCount

```solidity
uint256 configUpdateCount
```

### authorizedEmitters

```solidity
mapping(address => bool) authorizedEmitters
```

### ConfigUpdated

```solidity
event ConfigUpdated(bytes32 indexed key, uint256 oldValue, uint256 newValue, uint256 timestamp, uint256 blockNumber)
```

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

### PoolStateUpdated

```solidity
event PoolStateUpdated(bytes32 indexed poolId, uint256 virtualReserve, uint256 realReserve, uint256 tokenReserve, uint256 price, uint256 timestamp, uint256 blockNumber)
```

### setAuthorizedEmitter

```solidity
function setAuthorizedEmitter(address emitter, bool authorized) external
```

### isAuthorizedEmitter

```solidity
function isAuthorizedEmitter(address emitter) external view returns (bool)
```

### emitConfigUpdated

```solidity
function emitConfigUpdated(bytes32 key, uint256 oldValue, uint256 newValue) external
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

### NftMint

```solidity
event NftMint(uint256 indexed tokenId, address indexed creator, address indexed pool, uint8 strategy, uint256 timestamp, uint256 blockNumber)
```

### NftTransfer

```solidity
event NftTransfer(address indexed nft, address indexed from, address indexed to, uint256 tokenId, uint256 timestamp, uint256 blockNumber)
```

### AssetApproval

```solidity
event AssetApproval(address indexed asset, address indexed owner, address indexed spender, uint256 valueOrTokenId, bool isNft, uint256 timestamp, uint256 blockNumber)
```

### TokenDeployed

```solidity
event TokenDeployed(address indexed token, address indexed pool, address indexed creator, bytes32 salt, string name, string symbol, uint8 decimals, uint256 totalSupply, uint256 timestamp, uint256 blockNumber)
```

### RouterTrade

```solidity
event RouterTrade(uint8 indexed kind, address indexed pool, address indexed sender, address tokenIn, address tokenOut, uint256 amountIn, uint256 amountOut, uint256 intermediateUsdl, uint256 timestamp, uint256 blockNumber)
```

### FeeFlow

```solidity
event FeeFlow(uint8 indexed kind, address indexed pool, address indexed party, uint256 amount, uint256 protocolCut, uint256 poolCut, uint256 epoch, uint256 timestamp, uint256 blockNumber)
```

### PoolRegistered

```solidity
event PoolRegistered(address indexed pool, address indexed token, address indexed creator, address optical, uint256 nftId, uint256 timestamp, uint256 blockNumber)
```

### emitNftMint

```solidity
function emitNftMint(uint256 id, address creator, address pool, uint8 strategy) external
```

### emitNftTransfer

```solidity
function emitNftTransfer(address nft, address from, address to, uint256 id) external
```

### emitAssetApproval

```solidity
function emitAssetApproval(address asset, address owner, address spender, uint256 id, bool nft) external
```

### emitTokenDeployed

```solidity
function emitTokenDeployed(address token, address pool, address creator, bytes32 salt, string calldata name, string calldata symbol, uint8 decimals, uint256 supply) external
```

### emitRouterTrade

```solidity
function emitRouterTrade(uint8 kind, address pool, address sender, address tokenIn, address tokenOut, uint256 amountIn, uint256 amountOut, uint256 middle) external
```

### emitFeeFlow

```solidity
function emitFeeFlow(uint8 kind, address pool, address party, uint256 amount, uint256 protocolCut, uint256 poolCut, uint256 epoch) external
```

### emitPoolRegistered

```solidity
function emitPoolRegistered(address pool, address token, address creator, address optical, uint256 id) external
```
