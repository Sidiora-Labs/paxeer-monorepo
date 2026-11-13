# Solidity API

Source: [AntiSnipeOptical.sol](../../../contracts/opticals/presets/AntiSnipeOptical.sol).

## AntiSnipeOptical

Caps buy input during the protection period measured from PoolRegistry's recorded creation block.
The maximum USDL input is `(virtualUsdl + realUsdl) * maxBuyBps / 10000`, when token reserves are
nonzero. Sells and buys after the protection period are allowed.

Uses `beforeSwap`; configuration is fixed at construction. `maxBuyBps` must be from 1 to 10000.

### ProtectionActive

```solidity
error ProtectionActive()
```

### maxBuyBps

```solidity
uint256 maxBuyBps
```

Maximum buy input as a fraction of effective USDL reserves, in BPS (100 = 1%).

### protectionBlocks

```solidity
uint256 protectionBlocks
```

Number of blocks after pool creation during which protection is active

### poolCreationBlock

```solidity
mapping(address => uint256) poolCreationBlock
```

Mapping of pool address to the block number at which it was created

### constructor

```solidity
constructor(address _poolRegistry, address _owner, uint256 _maxBuyBps, uint256 _protectionBlocks) public
```

#### Parameters

| Name               | Type    | Description                                                    |
| ------------------ | ------- | -------------------------------------------------------------- |
| \_poolRegistry     | address | PoolRegistry address for pool validation                       |
| \_owner            | address | Deployer/owner address                                         |
| \_maxBuyBps        | uint256 | Maximum buy input in BPS of effective USDL reserves (100 = 1%) |
| \_protectionBlocks | uint256 | Number of blocks the protection lasts                          |

### registerPool

```solidity
function registerPool(address pool) external
```

Cache a registered, attached pool's creation block from PoolRegistry metadata. Anyone may call this;
the swap check reads the registry creation block directly.

### getFlags

```solidity
function getFlags() external pure returns (uint8)
```

### beforeSwap

```solidity
function beforeSwap(address pool, address, bool isBuy, uint256 amountIn) external returns (bool proceed, int256 amountDelta)
```
