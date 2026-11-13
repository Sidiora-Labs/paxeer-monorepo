# Solidity API

Source: [MaxWalletOptical.sol](../../../contracts/opticals/presets/MaxWalletOptical.sol).

## MaxWalletOptical

Limits recipient holdings on transfers from an attached pool. Uses the token's `beforeTokenTransfer`
extension, flag bit 4 (16). The check requires the registered pool's token as caller and rejects a
transfer when the recipient's balance plus the amount exceeds `totalSupply * maxWalletBps / 10000`.

Transfers to the pool and exempt recipients are allowed. The cap is fixed at construction; only the
owner may change exemptions.

### MaxWalletExceeded

```solidity
error MaxWalletExceeded()
```

### maxWalletBps

```solidity
uint256 maxWalletBps
```

Maximum wallet holding in basis points of total supply (e.g., 200 = 2%)

### exempt

```solidity
mapping(address => bool) exempt
```

Addresses exempt from max wallet check

### constructor

```solidity
constructor(address _poolRegistry, address _owner, uint256 _maxWalletBps) public
```

#### Parameters

| Name           | Type    | Description                              |
| -------------- | ------- | ---------------------------------------- |
| \_poolRegistry | address | PoolRegistry address for pool validation |
| \_owner        | address | Deployer/owner address                   |
| \_maxWalletBps | uint256 | Maximum holding in bps of total supply   |

### setExempt

```solidity
function setExempt(address account, bool isExempt) external
```

Set an address as exempt from max wallet check

_Only owner can set exemptions (e.g., pool address, treasury)_

### getFlags

```solidity
function getFlags() external pure returns (uint8)
```

### beforeTokenTransfer

```solidity
function beforeTokenTransfer(address pool, address from, address to, uint256 amount) external view
```

Checks transfers from the attached pool before recipient balances change.

### ExemptionUpdated

```solidity
event ExemptionUpdated(address indexed account, bool isExempt)
```
