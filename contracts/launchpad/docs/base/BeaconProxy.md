# Solidity API

Source: [BeaconProxy.sol](../../contracts/base/BeaconProxy.sol).

## BeaconProxy

Proxy that reads its implementation from a beacon contract

_Multiple BeaconProxy instances share one UpgradeableBeacon. Upgrading the beacon upgrades all
proxies atomically._

### BeaconCallFailed

```solidity
error BeaconCallFailed()
```

### constructor

```solidity
constructor(address beacon, bytes memory data)
```

#### Parameters

| Name   | Type    | Description                      |
| ------ | ------- | -------------------------------- |
| beacon | address | The UpgradeableBeacon address    |
| data   | bytes   | Optional initialization calldata |

### \_implementation

```solidity
function _implementation() internal view override returns (address)
```

Returns the current implementation by querying the beacon

### \_getBeaconImplementation

```solidity
function _getBeaconImplementation(address beacon) internal view returns (address impl)
```

_Reads the implementation() function from the beacon_

### BeaconInitializationFailed

```solidity
error BeaconInitializationFailed()
```

Returned when initialization delegatecall fails without revert data; nonempty revert data is
propagated.
