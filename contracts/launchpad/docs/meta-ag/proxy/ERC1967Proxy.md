# Solidity API

Source: [ERC1967Proxy.sol](../../../contracts/meta-ag/proxy/ERC1967Proxy.sol).

## ERC1967Proxy

ERC-1967 proxy that delegates calls through the in-house `Proxy` base. Construction stores the
implementation address and, when `data` is nonempty, delegatecalls that data to initialize the
proxy's storage. Empty data skips initialization.

### InitializationFailed

```solidity
error InitializationFailed()
```

### constructor

```solidity
constructor(address implementation, bytes data) public payable
```

_On initializer failure we bubble up the implementation's revert data verbatim (standard ERC-1967
pattern) so callers observe the real custom error (e.g. `ZeroAddress`) instead of a generic
`InitializationFailed`. The latter is retained only as the explicit fallback when the initializer
reverted without any return data._

#### Parameters

| Name           | Type    | Description                                                      |
| -------------- | ------- | ---------------------------------------------------------------- |
| implementation | address | UUPS implementation contract                                     |
| data           | bytes   | ABI-encoded initializer call. Pass `0x` for empty bytes to skip. |

### \_implementation

```solidity
function _implementation() internal view returns (address)
```

Returns the current implementation address
