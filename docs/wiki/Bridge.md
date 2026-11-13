# Bridge module

Paxeer is one BFT chain. Native bank and ledger transfers move value within that
chain; they do not cross a LayerX-to-Paxeer custody boundary. A committed block
establishes finality for those transfers. See [Unified network](UnifiedNetwork.md)
and [ledger payments](PaymentsQuickstart.md).

## External-chain bridge

The `layerxbridge` module and `LayerXBridge` precompile at
`0x0000000000000000000000000000000000001016` handle external-chain bridge
operations. The [ABI](../../precompiles/layerxbridge/abi.json) exposes:

- `bridgeIn` with source-chain event identity, recipient, asset, amount and
  attestor signatures.
- `bridgeOut` with destination chain, asset, amount and recipient, returning
  the withdrawal nonce.
- `getChain`, `getCap`, `getAttestors`, `isNullified` and `isPaused` views.

Chain registration, attestors, caps and pause state are governed by the
[module keeper](../../modules/layerxbridge/keeper). A configured external vault,
asset mapping and attestor policy are prerequisites; a precompile address alone
does not establish availability for an external chain. Use the registered ABI
and inspect the execution result and committed receipt.

## Kernel bridge helpers

Kernel module ID `8` (`LXP_MODULE_BRIDGE`) has no custody-credit activity.
The supported asset activity list does not include a withdrawal operation;
see [Asset](Asset.md). Funding uses current native bank or ledger operations,
rather than custody-credit or deposit-based account opening.

The retained library declarations in
[`lxp_bridge.h`](../../include/layerx/lxp_bridge.h) include withdrawal and
emergency-exit helpers. These declarations are not public activity registrations
and do not create another execution or finality network. See
[Custody](Custody.md) and [Finality](Finality.md) for the merged-chain model.

## Start here

- [Asset](Asset.md)
- [Precompile ABIs](../system/precompiles.md)
- [Unified network](UnifiedNetwork.md)
