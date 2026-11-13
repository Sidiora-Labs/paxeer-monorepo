# Solidity API

Source: [UpgradeableBeacon.sol](../../contracts/base/UpgradeableBeacon.sol).

## UpgradeableBeacon

Stores and upgrades the implementation address for beacon proxies

_Owner can upgrade. All BeaconProxy instances read implementation from here._

### InvalidImplementation

```solidity
error InvalidImplementation()
```

### Unauthorized

```solidity
error Unauthorized()
```

### ZeroAddress

```solidity
error ZeroAddress()
```

### Upgraded

```solidity
event Upgraded(address indexed implementation)
```

### OwnershipTransferred

```solidity
event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
```

### constructor

```solidity
constructor(address initialImplementation, address initialOwner)
```

### onlyOwner

```solidity
modifier onlyOwner()
```

### implementation

```solidity
function implementation() public view returns (address)
```

Returns the current implementation address

### owner

```solidity
function owner() public view returns (address)
```

Returns the current owner

### upgradeTo

```solidity
function upgradeTo(address newImplementation) external onlyOwner
```

Upgrades the implementation to a new address

### transferOwnership

```solidity
function transferOwnership(address newOwner) external onlyOwner
```

Proposes a nonzero pending owner. The current owner retains authority until acceptance.

### pendingOwner

```solidity
address public pendingOwner
```

The proposed owner, cleared after acceptance.

### OwnershipTransferStarted

```solidity
event OwnershipTransferStarted(address indexed previousOwner, address indexed pendingOwner)
```

### acceptOwnership

```solidity
function acceptOwnership() external
```

Only the pending owner can accept the transfer. Emits `OwnershipTransferred` and clears
`pendingOwner`.
