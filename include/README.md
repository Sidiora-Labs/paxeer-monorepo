# LayerX kernel headers

This directory holds the C interfaces for the deterministic LayerX kernel and
its native modules. The implementations live in [src/](../src/), and
[modules/lxphost/](../modules/lxphost/) embeds the kernel in `paxd` so that native
activities execute against the same chain state as EVM transactions. Receipts
are final when their containing block commits.

## Layout

- [layerx/](layerx/) — kernel, protocol, ledger, authority, storage, cryptography
  and native module headers.
- [layerx/lxp_kernel.h](layerx/lxp_kernel.h) — activity execution, prepared
  batches, state access and host callbacks.
- [layerx/lxp_kernel_host.h](layerx/lxp_kernel_host.h) — host entry points for
  loading module state and claiming the kernel writer.
- [layerx/lxp_module.h](layerx/lxp_module.h) — module identifiers, lifecycle
  callbacks and the module interface.
- [layerx/lxp_protocol.h](layerx/lxp_protocol.h) — protocol versions, limits and
  hashing domain identifiers.
- [layerx/lxp_receipt.h](layerx/lxp_receipt.h) — execution receipts, effects,
  program outcomes and receipt verification interfaces.
- [layerx/programs.h](layerx/programs.h) — Programs activities, runtime
  interfaces, scheduling and account capabilities.

## Build and test

The headers are compiled through their consumers. Run these targets from the
repository root; they are defined in the root [Makefile](../Makefile):

```sh
make build
make test
```

`make build` builds the kernel archive, genesis builder and module registry
tool. `make test` runs the root C test targets. For focused protocol and kernel
interface tests, the same Makefile provides:

```sh
make test-protocol
make test-kernel
make test-module-ctx
make test-receipts
```

## Configuration

The headers do not read environment variables or configuration files. The
embedding host supplies network identity, batch time, authority and fee
parameters through `lxp_kernel_execution` in
[lxp_kernel.h](layerx/lxp_kernel.h).

## Documentation

- [Network architecture](../docs/wiki/UnifiedNetwork.md)
- [Protocol](../docs/wiki/Protocol.md)
- [Kernel module interfaces and activity types](../docs/system/kernel-modules.md)
- [LayerX kernel on the documentation site](../docs/site/paxeer-docs/public/markdown/layerx-kernel.md)
