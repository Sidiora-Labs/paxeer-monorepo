# Solidity API

## TokenAddress

Computes deterministic SidioraERC20 token addresses from CREATE2 parameters

Computes the CREATE2 formula from the token constructor bytecode and arguments. The factory deploys
`SidioraERC20` with the factory itself as the initial token recipient before transferring the supply
to the pool.

Source: [TokenAddress.sol](../../contracts/libraries/TokenAddress.sol).

### computeAddress

```solidity
function computeAddress(address factory, address creator, string name, string symbol, uint256 nonce, bytes creationCode, uint256 totalSupply, address recipient) internal pure returns (address token)
```

Computes the CREATE2 address for a SidioraERC20 token

#### Parameters

| Name         | Type    | Description                                                         |
| ------------ | ------- | ------------------------------------------------------------------- |
| factory      | address | The factory contract address (deployer)                             |
| creator      | address | The market creator address                                          |
| name         | string  | Token name                                                          |
| symbol       | string  | Token symbol                                                        |
| nonce        | uint256 | Creator's nonce for uniqueness                                      |
| creationCode | bytes   | The SidioraERC20 creation code (without constructor args)           |
| totalSupply  | uint256 | Token total supply                                                  |
| recipient    | address | Initial token recipient (the factory for SidioraFactory deployment) |

#### Return Values

| Name  | Type    | Description                     |
| ----- | ------- | ------------------------------- |
| token | address | The deterministic token address |

### computeSalt

```solidity
function computeSalt(address creator, string name, string symbol, uint256 nonce) internal pure returns (bytes32)
```

Computes the salt for token CREATE2 deployment

#### Parameters

| Name    | Type    | Description        |
| ------- | ------- | ------------------ |
| creator | address | The market creator |
| name    | string  | Token name         |
| symbol  | string  | Token symbol       |
| nonce   | uint256 | Creator's nonce    |

#### Return Values

| Name | Type    | Description           |
| ---- | ------- | --------------------- |
| [0]  | bytes32 | salt The salt bytes32 |
