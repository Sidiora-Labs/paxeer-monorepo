# Perps ABI change history

The perps precompile ABI is defined in
[`precompiles/perps/abi/abi.json`](../precompiles/perps/abi/abi.json).
Callers must use its selectors, argument types and argument order.

## v6.12.0: embedded precompile ABI

Commit `3f12216b2b955ef232cb3e2c4b8b860e2c87d3a2` declares `IPerps` in
`Perps.sol` and embeds its ABI under `precompiles/perps/abi`. The
position-opening signature is `openPosition(uint256,address,uint256,uint256,bool)`:
market ID, collateral token, collateral amount, leverage and direction. Order
cancellation uses `cancelOrder(uint256)` with the order ID.

## TypeScript SDK 0.1.0: open and cancel encoding

Commit `c31ded3a33b5c890e15e40b05be99c5b763307e9` aligns the SDK's
[`perpsOpenPositionCall` and `perpsCancelOrderCall`](../services/agent/sdk/typescript/src/perps.ts)
with the embedded precompile ABI.

The open-position helper previously encoded
`openPosition(bytes32,bytes32,uint8,uint256,uint256)`, using market and
account IDs, side, size and leverage. It now encodes
`openPosition(uint256,address,uint256,uint256,bool)`, using a bigint market
ID, collateral token address, bigint collateral amount, bigint leverage and
boolean direction. The cancel helper previously encoded
`cancelOrder(bytes32,bytes32)` with order and account IDs. It now encodes
`cancelOrder(uint256)` with a single bigint order ID.

The same commit updates
[`perps-v2.test.ts`](../services/agent/sdk/typescript/test/perps-v2.test.ts)
and the precompile-call tests. They derive selectors from the embedded ABI,
assert encoded argument words and reject invalid inputs. The SDK package
version remains `0.1.0`; use the commit to distinguish the corrected encoders
from earlier code carrying the same package version.

Commit `c1f5f4d31ef0f4885db9b4203821d350fd249cb1` changes programme metadata
only. It makes no ABI or caller implementation change and is not evidence of
SDK, wallet or web compatibility.
