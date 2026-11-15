# Bridge regression tests

This directory holds Python regression tests for retired custody contracts and bridge consumers, a C file-reading helper, and Python dependency declarations. The tests check repository files and their callers against the merged Paxeer network: the LayerX kernel inside `paxd` executes and orders each transaction and builds each block; validators check and commit it, and receipts become final when the validators commit their blocks.

## Layout

- [retired/](retired/) contains the retirement inventory, reference scanner, scanner tests, and assertions for custody contracts, deployment scripts, credit consumers, and settlement-document readers.
- [files.h](files.h) defines `read_file`, which reads a bounded regular file and optionally requires private file permissions.
- [requirements.txt](requirements.txt) declares the cryptography, pycryptodome, eth-account, and eth-utils dependencies used by repository Python tests.

## Build and test

The directory has no standalone build target. Its Python tests run directly with pytest; `files.h` is a header without a standalone executable. Python and Git are required, and the daemon-consumer test also requires Make.

From the repository root, use the pytest invocation used by the repository's [Python suite runner](../ci/run_python_suites.py), selecting this directory:

```sh
python3 -m pytest -q tests/bridge/retired
```

The daemon-consumer test inspects the root [Makefile](../../Makefile) recipes with `make -n build test`. The settlement test checks the retained document's schema and its exact set of readers.

Related native bridge tests are defined separately in the root Makefile:

```sh
make test-bridge-deposit
make test-bridge-withdraw
```

These targets build and run `tests/test_bridge_deposit.c` and `tests/test_bridge_withdraw.c` with the kernel library and programs runtime.

## Configuration

The sources in this directory read no environment variables. The settlement helper reads [contracts/config/checkpoint-settlement.json](../../contracts/config/checkpoint-settlement.json). The retirement scanner reads the Git index and tracked source files; its repository root defaults to the checkout containing the helper.

## Documentation

- [Kernel modules](../../docs/system/kernel-modules.md) describes the native bridge helpers and module registrations.
- [Bridging and balances](../../docs/site/paxeer-docs/public/markdown/bridge.md) describes the external-chain bridge; its [documentation-site page](../../docs/site/paxeer-docs/src/app/bridge/page.tsx) contains the rendered content.
- [Account funding and key custody](../../docs/site/paxeer-docs/public/markdown/custody.md) describes account funding and custody behavior on the merged network.
