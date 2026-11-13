# Native execution and Solidity interfaces

## kernel-solidity

Kernel-backed execution uses native precompiles and the node's shared host. Solidity callers use the [precompile interfaces](../../../contracts/src/precompiles), whose signatures must match each served ABI. Native precompiles have no deployed EVM bytecode; a Solidity caller must account for code-existence checks when issuing a void call.

The [ledger and auth](ledger-auth.md), [pay](pay.md), [programs](programs.md), [virtual chains](virtual-chains.md) and [web](web.md) contracts describe current authority, encoding, events and activation. A retained ABI selector or compatibility encoder does not itself establish an executable activity.

<a id="retirements-contract"></a>
