# Solidity API

Source: [Multicall.sol](../../contracts/base/Multicall.sol).

## Multicall

Batch multiple calls in a single transaction

_Enables batching of multiple function calls to this contract_

### MulticallFailed

```solidity
error MulticallFailed(uint256 index, bytes reason)
```

### multicall

```solidity
function multicall(bytes[] calldata data) external returns (bytes[] memory results)
```

Executes multiple calls in a single transaction

#### Parameters

| Name | Type    | Description                     |
| ---- | ------- | ------------------------------- |
| data | bytes[] | Array of encoded function calls |

#### Return Values

| Name    | Type    | Description                         |
| ------- | ------- | ----------------------------------- |
| results | bytes[] | Array of return data from each call |
