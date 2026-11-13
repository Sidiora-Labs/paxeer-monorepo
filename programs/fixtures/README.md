# Programs fixtures

This directory holds canonical WebAssembly artifacts, payment authorization vectors,
and native execution fixtures for Programs in Paxeer X. They exercise program
deployment, account registration, calls, receipts, and authenticated native
transfers through the kernel that runs inside `paxd` on the merged network.

## Layout

- [`pay5/`](pay5/): token, merchant, and swap artifacts, interfaces, registry values,
  authorization vectors, and native replay sources.
- [`test_workspace_graph.py`](test_workspace_graph.py): checks the resolved Cargo
  dependencies of the Programs registry and sandbox.
- [`pay5/native-token/`](pay5/native-token/): canonical token activities, receipts,
  terminal payloads, call graphs, and the deployed interface registry value.
- [`pay5/native-merchant/`](pay5/native-merchant/): canonical merchant activities,
  receipts, terminal payloads, and call graphs.
- [`pay5/native_roundtrip.c`](pay5/native_roundtrip.c) and
  [`pay5/native_roundtrip.h`](pay5/native_roundtrip.h): native token and merchant
  execution harness, including balance, fee, authorization, and receipt checks.
- [`pay5/run_native_roundtrip.py`](pay5/run_native_roundtrip.py): builds the native
  harness and compares its output byte for byte with the stored fixtures.
- [`pay5/test_native_roundtrip.py`](pay5/test_native_roundtrip.py): pytest entrypoint
  for the native replay.
- [`pay5/generate_sdk_account_fixtures.py`](pay5/generate_sdk_account_fixtures.py):
  generates account and destination authorization vectors from native encoder output.
- [`pay5/account-authorization-vectors.json`](pay5/account-authorization-vectors.json),
  [`pay5/destination-authorization-vectors.json`](pay5/destination-authorization-vectors.json),
  and [`pay5/receipt-account-bound-v4.json`](pay5/receipt-account-bound-v4.json):
  authorization acceptance and rejection cases and account-bound receipt evidence.

## Build and test

Run commands from the repository root. The directory has no separate build
manifest or Makefile. The native replay script builds its dependencies with:

```sh
make -j16 build/liblayerx.a programs-build
```

Build and run the token and merchant replay, comparing all generated files with
the stored fixtures:

```sh
python3 programs/fixtures/pay5/run_native_roundtrip.py
```

The pytest files cover native replay and the locked, offline workspace dependency
graph:

```sh
python3 -m pytest programs/fixtures/pay5/test_native_roundtrip.py programs/fixtures/test_workspace_graph.py
```

The root target `make programs-reference-fixtures` invokes
[`program-fixtures.sh`](../../scripts/programs/program-fixtures.sh) to rebuild
reference artifacts, including the swap artifact, interface, and registry value
in `pay5/`. The native replay script's `--write-fixtures` option replaces the
stored token and merchant execution fixtures.

## Configuration

- `PAY5_QUAL_DIR`: output directory read by the replay script and native harness.
- `PAY5_FIXTURE_OUT`: fixture output directory read by the native harness; the
  replay script sets it for each case.
- `CARGO_BUILD_JOBS` and `MAKEFLAGS`: build parallelism set by the replay script.

The workspace dependency test reads [`../Cargo.toml`](../Cargo.toml) and the locked
dependency graph in [`../Cargo.lock`](../Cargo.lock). Cargo builds entered through
the Programs workspace use [`../.cargo/config.toml`](../.cargo/config.toml).

## Documentation

- [Programs](../../docs/wiki/Programs.md): execution, token behavior, and payment
  authorization.
- [Porting](../../docs/wiki/Porting.md): native asset mapping and fixture replay.
- [Programs site page](../../docs/site/paxeer-docs/public/markdown/programs.md).
- [Programs Developer Workspace site page](../../docs/site/paxeer-docs/public/markdown/program-workspace.md).
