# Solidity API

Source: [PoolBeacon.sol](../../contracts/core/PoolBeacon.sol).

## PoolBeacon

Stores the SidioraPool implementation address. All pool proxies read from this.

A wrapper around [UpgradeableBeacon](../base/UpgradeableBeacon.md). The constructor sets the
supplied initial owner; deployment proposes ownership to Timelock, which becomes owner after
acceptance. Updating the beacon changes the implementation used by every pool attached to that
beacon.

### constructor

```solidity
constructor( address initialImplementation, address initialOwner ) UpgradeableBeacon(initialImplementation, initialOwner)
```
