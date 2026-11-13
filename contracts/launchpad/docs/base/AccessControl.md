# Solidity API

Source: [AccessControl.sol](../../contracts/base/AccessControl.sol).

## AccessControl

Role-based access control with role admin hierarchy

_Roles are referred to by their bytes32 identifier. DEFAULT_ADMIN_ROLE is the admin for all roles by
default._

### DEFAULT_ADMIN_ROLE

```solidity
bytes32 DEFAULT_ADMIN_ROLE
```

### RoleData

```solidity
struct RoleData {
  mapping(address => bool) hasRole;
  bytes32 adminRole;
}
```

### MissingRole

```solidity
error MissingRole(address account, bytes32 role)
```

### RoleGranted

```solidity
event RoleGranted(bytes32 indexed role, address indexed account, address indexed sender)
```

### RoleRevoked

```solidity
event RoleRevoked(bytes32 indexed role, address indexed account, address indexed sender)
```

### RoleAdminChanged

```solidity
event RoleAdminChanged(bytes32 indexed role, bytes32 indexed previousAdminRole, bytes32 indexed newAdminRole)
```

### onlyRole

```solidity
modifier onlyRole(bytes32 role)
```

### hasRole

```solidity
function hasRole(bytes32 role, address account) public view returns (bool)
```

Returns true if account has the given role

### getRoleAdmin

```solidity
function getRoleAdmin(bytes32 role) public view returns (bytes32)
```

Returns the admin role for a given role

### grantRole

```solidity
function grantRole(bytes32 role, address account) external onlyRole(getRoleAdmin(role))
```

Grants a role to an account. Caller must have the role's admin role.

### revokeRole

```solidity
function revokeRole(bytes32 role, address account) external onlyRole(getRoleAdmin(role))
```

Revokes a role from an account. Caller must have the role's admin role.

### renounceRole

```solidity
function renounceRole(bytes32 role, address callerConfirmation) external
```

Account renounces a role from itself

### \_checkRole

```solidity
function _checkRole(bytes32 role, address account) internal view
```

### \_grantRole

```solidity
function _grantRole(bytes32 role, address account) internal virtual returns (bool)
```

### \_revokeRole

```solidity
function _revokeRole(bytes32 role, address account) internal virtual returns (bool)
```

### \_setRoleAdmin

```solidity
function _setRoleAdmin(bytes32 role, bytes32 adminRole) internal
```

### registeredDefaultAdminCount

```solidity
function registeredDefaultAdminCount() external view returns (uint256)
```

Returns the tracked default-admin count. Role grants reject the zero address with
`InvalidRoleAccount()`; revocation retains at least one tracked default admin or reverts with
`LastDefaultAdmin()`.
