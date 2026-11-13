# LayerX anchor

This directory holds the anchor module's retained checkpoint, guarantor bond,
availability, challenge, and genesis types, together with its state keeper and
retirement handling in `paxd`. Paxeer is one BFT network: a receipt is final when
its block commits. `SubmitCheckpoint`, `SubmitAvailabilityAttestation`, and
`SubmitEquivocation` return `ErrModuleRetired`. The registered `MsgUpdateParams`
route also returns that error when `v7.0` is active. The anchor precompile retains
checkpoint reads until `v7.0-anchor-store-delete`; these records describe retained
anchor state rather than current block finality.

## Layout

- [keeper/](keeper/) — state access, genesis import and export, bond accounting, challenges, custody read adapter, and retirement handling, with keeper tests.
- [types/](types/) — parameters, genesis and state schemas, store keys, events, errors, message codecs, generated transaction bindings, and type tests.
- [testdata/](testdata/) — `anchor_vectors.json`, read by the keeper tests.
- [module.go](module.go) — SDK module registration, genesis validation and serialization, message service registration, and invariants.
- [bringup_genesis_test.go](bringup_genesis_test.go) — tests the node's registered genesis decoder with absent, valid, and malformed anchor state.
- [proof_retirement_test.go](proof_retirement_test.go) — tests the registered message service, governance caller, upgrade retirement gate, and genesis consumer.

## Build and test

The module belongs to the root [go.mod](../../go.mod) and has no separate build
manifest or Makefile. Run these commands from the repository root:

```sh
make paxd-build
make -f chain.mk test
```

The root [Makefile](../../Makefile) defines `paxd-build`, which builds native
dependencies and the `paxd` application through [chain.mk](../../chain.mk).
The `test` target in `chain.mk` splits the root Go packages with tests into groups
and runs them with the race detector and coverage, including this module.

## Configuration

The module reads parameters from SDK state and accepts them in the `params`
field of [GenesisState](types/genesis.go). It reads no environment variables or
standalone configuration files. [Params](types/params.go) declares these JSON
field names:

- `authority`, `bond_denom`, `threshold`, `min_bond`, `permissionless_registration`.
- `unbonding_delay_seconds`, `challenge_window_seconds`, `challenge_bond`, `max_attestation_delay_ms`.
- `paxeer_chain_id`, `settlement_contract`, `network_id`.
- `slash_fraction_equivocation`, `slash_fraction_fraud`, `slash_fraction_availability`, `reporter_share`, `slash_destination`.

The message service reads upgrade activation through `UpgradeReader`; the plan
names are declared in [types/retire.go](types/retire.go). `RetireAtMerge` returns
guarantor bonds, pending unbondings, and open challenge bonds through the bank
keeper.

## Documentation

- [Chain module reference](../../docs/system/chain-modules.md#layerxanchor).
- [Anchor precompile reference](../../docs/system/precompiles.md#layerxanchor-abijson).
- [Upgrade plan](../../docs/release/7.0.0-upgrade-plan.md).
- [Fork and anchor store deletion runbook](../../docs/runbooks/paxeer-x-fork.md).
- [Documentation site: checkpoints](../../docs/site/paxeer-docs/public/markdown/checkpoints.md).
- [Documentation site: modules](../../docs/site/paxeer-docs/public/markdown/modules.md).
