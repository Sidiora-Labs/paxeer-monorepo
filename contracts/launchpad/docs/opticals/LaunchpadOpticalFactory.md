# Solidity API

Source: [LaunchpadOpticalFactory.sol](../../contracts/opticals/LaunchpadOpticalFactory.sol).

## LaunchpadOpticalFactory

UUPS-upgradeable factory for `LaunchpadOptical` instances. The caller becomes the optical's owner
and creator. Team wallets, vesting and capital-raise settings are supplied at creation.

The factory calls `FeeAccumulator.authorizeOptical` to grant the new instance `OPTICAL_CLAIM_ROLE`.
The factory must hold `OPTICAL_GRANTER_ROLE` on the accumulator.

### ZeroAddress

```solidity
error ZeroAddress()
```

### ArrayTooLong

```solidity
error ArrayTooLong()
```

### LaunchpadOpticalCreated

```solidity
event LaunchpadOpticalCreated(address optical, address creator, address teamClaimAddress, uint256 cliffDuration, uint256 vestingDuration, uint256 capitalRaiseBps, uint256 capitalRaiseDuration)
```

### poolRegistry

```solidity
address poolRegistry
```

PoolRegistry address (passed to each LaunchpadOptical)

### feeAccumulator

```solidity
address feeAccumulator
```

FeeAccumulator address (passed to each LaunchpadOptical + role granting)

### opticalRegistry

```solidity
address opticalRegistry
```

Stored OpticalRegistry address supplied at initialization.

### constructor

```solidity
constructor() public
```

### initialize

```solidity
function initialize(address _poolRegistry, address _feeAccumulator, address _opticalRegistry, address _admin) external
```

### createLaunchpadOptical

```solidity
function createLaunchpadOptical(address[] teamWallets, uint256 cliffDuration, uint256 vestingDuration, uint256 capitalRaiseBps, uint256 capitalRaiseDuration, address teamClaimAddress) external returns (address optical)
```

Deploy a new LaunchpadOptical for a project

#### Parameters

| Name                 | Type      | Description                                              |
| -------------------- | --------- | -------------------------------------------------------- |
| teamWallets          | address[] | Additional team wallets to vest (creator is auto-vested) |
| cliffDuration        | uint256   | Hard lock period in seconds                              |
| vestingDuration      | uint256   | Linear unlock period after cliff in seconds              |
| capitalRaiseBps      | uint256   | Fee percentage diverted to team (max 1000 = 10%)         |
| capitalRaiseDuration | uint256   | How long the capital raise fee is active in seconds      |
| teamClaimAddress     | address   | Where accumulated USDL is claimed to                     |

#### Return Values

| Name    | Type    | Description                                        |
| ------- | ------- | -------------------------------------------------- |
| optical | address | The address of the newly deployed LaunchpadOptical |

### getOpticalsByCreator

```solidity
function getOpticalsByCreator(address creator) external view returns (address[])
```

Get all LaunchpadOpticals deployed by a specific creator

### getDeployedCount

```solidity
function getDeployedCount() external view returns (uint256)
```

Get total number of deployed LaunchpadOpticals

### getAllOpticals

```solidity
function getAllOpticals(uint256 offset, uint256 limit) external view returns (address[])
```

Get paginated list of all deployed opticals

### \_authorizeUpgrade

```solidity
function _authorizeUpgrade(address) internal
```
