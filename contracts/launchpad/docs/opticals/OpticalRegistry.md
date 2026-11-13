# Solidity API

Source: [OpticalRegistry.sol](../../contracts/opticals/OpticalRegistry.sol).

## OpticalRegistry

UUPS-upgradeable registry of optical metadata and approved identities. Administrators register,
deregister and update opticals. Approval records the runtime code hash and hook flags;
`isRegistered` and `isApproved` require those values to match the current optical.

Registration requires contract code, a valid `getFlags()` result in the range 0–31 and a risk level
from 1 to 5. `renewReview` checks the expected code hash and flags and records a nonzero evidence
digest. The enumeration includes every distinct optical ever registered, including deregistered
entries.

### ZeroAddress

```solidity
error ZeroAddress()
```

### AlreadyRegistered

```solidity
error AlreadyRegistered()
```

### NotRegistered

```solidity
error NotRegistered()
```

### OpticalMetadata

```solidity
struct OpticalMetadata {
  string name;
  string description;
  uint8 riskLevel;
  string auditor;
  uint256 registeredAt;
}
```

### OpticalRegistered

```solidity
event OpticalRegistered(address indexed optical, string name, uint8 riskLevel, uint256 timestamp)
```

### OpticalDeregistered

```solidity
event OpticalDeregistered(address indexed optical, uint256 timestamp)
```

### OpticalMetadataUpdated

```solidity
event OpticalMetadataUpdated(address indexed optical, uint256 timestamp)
```

### eventEmitter

```solidity
address eventEmitter
```

### constructor

```solidity
constructor() public
```

### initialize

```solidity
function initialize(address _eventEmitter, address _admin) external
```

### registerOptical

```solidity
function registerOptical(address optical, string name, string description, uint8 riskLevel, string auditor) external
```

Register an optical as approved with metadata

#### Parameters

| Name        | Type    | Description                           |
| ----------- | ------- | ------------------------------------- |
| optical     | address | The optical contract address          |
| name        | string  | Human-readable name                   |
| description | string  | Description of the optical's behavior |
| riskLevel   | uint8   | Risk level 1-5 (1 = lowest)           |
| auditor     | string  | Auditor name or "unaudited"           |

### deregisterOptical

```solidity
function deregisterOptical(address optical) external
```

Deregister an optical (mark as not approved)

#### Parameters

| Name    | Type    | Description                                |
| ------- | ------- | ------------------------------------------ |
| optical | address | The optical contract address to deregister |

### updateMetadata

```solidity
function updateMetadata(address optical, string name, string description, uint8 riskLevel, string auditor) external
```

Update metadata for a registered optical

### isRegistered

```solidity
function isRegistered(address optical) external view returns (bool)
```

Check if an optical is registered and approved

### getOpticalMetadata

```solidity
function getOpticalMetadata(address optical) external view returns (struct OpticalRegistry.OpticalMetadata)
```

Get metadata for an optical

### getAllOpticals

```solidity
function getAllOpticals(uint256 offset, uint256 limit) external view returns (address[])
```

Get all distinct ever-registered opticals (paginated), including deregistered entries.

### getOpticalCount

```solidity
function getOpticalCount() external view returns (uint256)
```

Get total count of ever-registered opticals

### \_authorizeUpgrade

```solidity
function _authorizeUpgrade(address) internal
```

### isApproved

```solidity
function isApproved(address optical) external view returns (bool)
```

Returns the same current identity approval as `isRegistered`.

### approvedCodeHash

```solidity
mapping(address => bytes32) approvedCodeHash
```

Approved runtime code hash per optical.

### IdentityReview

```solidity
struct IdentityReview {
  bool initialized;
  uint8 flags;
  uint256 reviewedAt;
  bytes32 evidenceDigest;
}
```

### identityReviews

```solidity
mapping(address => struct OpticalRegistry.IdentityReview) identityReviews
```

Stored hook flags, review timestamp and evidence digest.

### renewReview

```solidity
function renewReview(address optical, bytes32 expectedCodeHash, uint8 expectedFlags, bytes32 evidenceDigest) external
```

Administrators renew an approved optical's identity using a matching code hash and flags and a
nonzero evidence digest.

### migrateIndex

```solidity
function migrateIndex(uint256 limit) external
```

Administrators process at most `limit` entries to initialize a distinct-address index. Requires a
positive limit and an uninitialized index.

### indexInitialized

```solidity
bool indexInitialized
```

True after fresh initialization or completion of index migration.

### indexMigrationCursor

```solidity
uint256 indexMigrationCursor
```

Number of enumeration entries processed during index migration.

### indexMigrationWritten

```solidity
uint256 indexMigrationWritten
```

Number of distinct entries retained during index migration.
