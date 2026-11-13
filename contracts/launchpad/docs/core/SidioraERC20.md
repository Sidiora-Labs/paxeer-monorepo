# Solidity API

Source: [SidioraERC20.sol](../../contracts/core/SidioraERC20.sol).

## SidioraERC20

Minimal immutable ERC20 token deployed via CREATE2 by the Factory

_Total supply minted once in constructor to recipient. No mint/burn after creation._

### constructor

```solidity
constructor( string memory _name, string memory _symbol, uint256 _totalSupply, address _recipient ) ERC20Base(_name, _symbol, 6)
```

#### Parameters

| Name          | Type    | Description                                  |
| ------------- | ------- | -------------------------------------------- |
| \_name        | string  | Token name                                   |
| \_symbol      | string  | Token symbol                                 |
| \_totalSupply | uint256 | Total supply to mint (6 decimals)            |
| \_recipient   | address | Address to receive the entire initial supply |

### bindPool

```solidity
function bindPool(address pool_) external
```

The deployment factory binds one pool with code, once. A bound pool’s optical can enforce transfer
policy when flag bit 4 is set. `InvalidPoolBinding()` rejects other binding calls. The token
inherits permit and historical balance/supply reads from [ERC20Base](../base/ERC20Base.md).
