# LayerX custody

This directory contains the native `layerxcustody` module in the merged Paxeer network: custody records, genesis support, governance dispatch and merge retirement handling. At `v7.0` activation, its message handlers and proposal handler return `ErrModuleRetired`; deposits and deposit-root registration also refuse writes. Withdrawal inclusion and forced-exit proof paths refuse independently of activation height. The merge release cancels pending claims and transfers custody balances to the configured custody authority, while retaining asset, deposit, claim and checkpoint records.

## Layout

- [keeper/](keeper/) — persistent custody state, asset mappings, deposits, claims, genesis import/export, migrations, solvency invariant, retirement release and tests.
- [types/](types/) — parameter and genesis validation, message and proposal types, codecs, store keys, identifiers, errors, generated protobuf types and tests.
- [module.go](module.go) — SDK module registration, message service, migration, invariant and genesis adapters.
- [handler.go](handler.go) — custody governance proposal dispatch through the message service, with activation and retirement checks.
- [handler_test.go](handler_test.go) — governance dispatch, validation and atomic execution tests.
- [handler_retire_test.go](handler_retire_test.go) — proposal refusal at merge activation.
- [module_retire_test.go](module_retire_test.go) — checks that the module exposes no transaction or query CLI command.

## Build and test

The module belongs to the root [Go module](../../go.mod) and has no separate manifest or Makefile. From the repository root, use the targets in the root [Makefile](../../Makefile), which delegate to [chain.mk](../../chain.mk):

```sh
make paxeer-build
make paxeer-test
```

`paxeer-build` builds `paxd` with its native libraries. `paxeer-test` runs the chain Go test groups, including this module's packages, with race detection and coverage. Both targets use the locally available Go dependencies with `GOPROXY=off`.

## Configuration

The module reads no environment variables or standalone configuration files. Its parameters are loaded through genesis and stored in the SDK key-value store. The fields declared in [types/params.pb.go](types/params.pb.go) and validated in [types/params.go](types/params.go) are:

- `authority`
- `network_id`
- `withdrawal_delay_seconds`
- `forced_exit_delay_seconds`
- `liveness_bound_seconds`
- `sequencer_authorizations`
- `deposit_root_authority`

These fields remain part of the retained custody schema. Retirement is determined by the upgrade keeper or an imported retirement marker; parameter updates through the message service refuse after retirement.

## Documentation

- [Chain modules and custody parameters](../../docs/system/chain-modules.md#layerxcustody)
- [Governance](../../docs/system/governance.md)
- [Custody precompile behavior](../../docs/system/precompiles.md)
- [Network upgrade and custody release](../../docs/release/7.0.0-upgrade-plan.md)
- [Documentation site: account funding and key custody](../../docs/site/paxeer-docs/public/markdown/custody.md)
