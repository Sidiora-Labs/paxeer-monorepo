# Solidity API

Source: [SidioraFactory.sol](../../contracts/core/SidioraFactory.sol).

## SidioraFactory

Market creation orchestrator. One transaction creates token + pool + NFT.

_UUPS proxy. CREATE2 deploys SidioraERC20, BeaconProxy deploys SidioraPool._

### ROUTER_ROLE

```solidity
bytes32 ROUTER_ROLE
```

### poolBeacon

```solidity
address poolBeacon
```

Get the pool beacon address

### nftContract

```solidity
address nftContract
```

Get the NFT contract address

### poolRegistry

```solidity
address poolRegistry
```

Get the pool registry address

### eventEmitter

```solidity
address eventEmitter
```

Get the event emitter address

### protocolConfig

```solidity
address protocolConfig
```

Get the protocol config address

### treasury

```solidity
address treasury
```

Get the treasury address

### feeAccumulator

```solidity
address feeAccumulator
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
function initialize( address _poolBeacon, address _nftContract, address _poolRegistry, address _eventEmitter, address _protocolConfig, address _treasury, address _feeAccumulator, address _usdlAddress, address _admin ) external initializer
```

### createMarket

```solidity
function createMarket( string calldata name, string calldata symbol, uint8 feeStrategy, address optical ) external override returns (address tokenAddr, address poolAddr, uint256 nftId)
```

Create a new market (token + pool + NFT)

#### Parameters

| Name        | Type    | Description                                                                   |
| ----------- | ------- | ----------------------------------------------------------------------------- |
| name        | string  | Token name                                                                    |
| symbol      | string  | Token symbol                                                                  |
| feeStrategy | uint8   | Initial fee strategy for the pool NFT (0=CLAIM,1=BURN,2=AIRDROP,3=LP_REWARDS) |
| optical     | address | Optional optical hook contract (address(0) for none)                          |

#### Return Values

| Name      | Type    | Description                |
| --------- | ------- | -------------------------- |
| tokenAddr | address | The deployed token address |
| poolAddr  | address | The deployed pool address  |
| nftId     | uint256 | The minted NFT token ID    |

### createMarketFor

```solidity
function createMarketFor( address creator, string calldata name, string calldata symbol, uint8 feeStrategy, address optical ) external override onlyRole(ROUTER_ROLE) returns (address tokenAddr, address poolAddr, uint256 nftId)
```

Create a new market on behalf of a creator (Router-only via ROUTER_ROLE)

#### Parameters

| Name        | Type    | Description                                                 |
| ----------- | ------- | ----------------------------------------------------------- |
| creator     | address | The actual creator address (receives NFT, becomes guardian) |
| name        | string  | Token name                                                  |
| symbol      | string  | Token symbol                                                |
| feeStrategy | uint8   | Initial fee strategy for the pool NFT                       |
| optical     | address | Optional optical hook contract (address(0) for none)        |

#### Return Values

| Name      | Type    | Description                |
| --------- | ------- | -------------------------- |
| tokenAddr | address | The deployed token address |
| poolAddr  | address | The deployed pool address  |
| nftId     | uint256 | The minted NFT token ID    |

### getNonce

```solidity
function getNonce(address creator) external view override returns (uint256)
```

Get the nonce for a creator (used for CREATE2 determinism)

### \_createMarket

```solidity
function _createMarket( address creator, string calldata name, string calldata symbol, uint8 feeStrategy, address optical ) internal returns (address tokenAddr, address poolAddr, uint256 nftId)
```

### \_deployBeaconProxy

```solidity
function _deployBeaconProxy(bytes32 salt, bytes memory initData) internal returns (address proxy)
```

### \_authorizeUpgrade

```solidity
function _authorizeUpgrade(address) internal override onlyRole(DEFAULT_ADMIN_ROLE)
```

### predictPoolAddress

```solidity
function predictPoolAddress(address token, address creator, address optical) external view returns (address)
```

Predicts the pool CREATE2 address from the current configuration and supplied identities.

## BeaconProxyDeployer

_Minimal beacon proxy for CREATE2 deployment by factory. Reads implementation from beacon, delegates
all calls._

### BeaconCallFailed

```solidity
error BeaconCallFailed()
```

### constructor

```solidity
constructor(address beacon, bytes memory data)
```

### fallback

```solidity
fallback() external payable
```

### receive

```solidity
receive() external payable
```

### \_beacon

```solidity
function _beacon() internal view returns (address b)
```

### \_getImpl

```solidity
function _getImpl(address beacon) internal view returns (address impl)
```
