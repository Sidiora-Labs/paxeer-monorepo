# Solidity API

## PoolAddress

Computes deterministic pool addresses from BeaconProxy CREATE2 parameters

Used by `SidioraFactory.predictPoolAddress`. The address depends on the factory, token-derived salt,
beacon, exact proxy creation code and initialization calldata. The factory supplies
`BeaconProxyDeployer` creation code.

Source: [PoolAddress.sol](../../contracts/libraries/PoolAddress.sol).

### computeAddress

```solidity
function computeAddress(address factory, address beacon, address token, bytes memory creationCode, bytes memory initData) internal pure returns (address pool)
```

Computes the CREATE2 address for a pool BeaconProxy

#### Parameters

| Name         | Type    | Description                                                                    |
| ------------ | ------- | ------------------------------------------------------------------------------ |
| factory      | address | The factory contract address (deployer)                                        |
| beacon       | address | The PoolBeacon address                                                         |
| token        | address | The SidioraERC20 token address (used in salt)                                  |
| creationCode | bytes   | Exact creation code used for deployment, without constructor arguments         |
| initData     | bytes   | Exact initialization calldata encoded with the beacon as constructor arguments |

#### Return Values

| Name | Type    | Description                    |
| ---- | ------- | ------------------------------ |
| pool | address | The deterministic pool address |

### computeSalt

```solidity
function computeSalt(address token) internal pure returns (bytes32)
```

Computes the salt used for pool CREATE2 deployment

#### Parameters

| Name  | Type    | Description       |
| ----- | ------- | ----------------- |
| token | address | The token address |

#### Return Values

| Name | Type    | Description           |
| ---- | ------- | --------------------- |
| [0]  | bytes32 | salt The salt bytes32 |
