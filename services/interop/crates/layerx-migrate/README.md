# layerx-migrate

Ethereum and Solana source-chain verifiers for the `migration` interop adapter.
This crate imports provenance against pinned source deployments. Network credits must cross
the authenticated payment boundary and be checked against network evidence.
Paxeer mainnet-beta is one
network (chain ID 125), with the kernel participating in block proposal and
finality at block commit.

Operator contract: [`OPERATIONS.md`](OPERATIONS.md). Workspace index: [`../../README.md`](../../README.md).

Wiki page: [`docs/wiki/Migration.md`](../../../../docs/wiki/Migration.md). LayerX kernel genesis import sections (different surface): [`migrations/`](../../../../migrations/README.md).

The crate also builds the `layerx-migrate` binary. Its commands are
`verify-ownership`, `verify-asset`, `confirm-mapping`, `migrate-asset`,
`import-history` and `read-history`. `confirm-mapping` checks an existing Paxeer
DID/EVM binding and stores external mapping provenance; `migrate-asset` submits
source evidence to the authenticated gateway. Neither a source-verification
result nor a history record is a network receipt. See
[PORTABILITY.md](../../../../docs/interop/PORTABILITY.md) for receipt verification.

The authenticated journals require an independent rollback authority through
`layerx_getMigrationJournalHead` and `layerx_advanceMigrationJournalHead`. This
tree supplies their client contract, not a production authority server; see
[the operations contract](OPERATIONS.md#journal-and-rollback-authority).

For development from the monorepo root: `make interop-test-migration`. The ignored live-testnet suite is `make interop-test-migration-testnets`; it needs an operator environment and runs in CI only through the manually dispatched `.github/workflows/interop-migration-testnets.yml`.

The [funded operator settlement test](tests/funded_operator.rs),
`funded_operator_runs_real_gateway_source_and_independent_funded_receipts`,
requires `LAYERX_MIGRATION_FUNDED_OPERATOR_FIXTURE`, an absolute path to a
genuine protected funded-operator fixture. The
[authenticated mapping and funded settlement test](tests/migration_v2_connected.rs),
`migration_v2_connected_runs_authenticated_mapping_and_funded_settlement`,
requires `LAYERX_MIGRATION_V2_FIXTURE`, an absolute path to a genuine protected
migration V2 fixture. Both require genuine Ethereum and Solana coverage with
zero skipped cases. Without those producer inputs they refuse to run;
compiling the targets alone does not establish connected behaviour.
