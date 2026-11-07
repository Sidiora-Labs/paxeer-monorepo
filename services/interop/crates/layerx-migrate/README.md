# layerx-migrate

Ethereum and Solana source-chain verifiers for the `migration` interop adapter.
This crate imports provenance against pinned source deployments. Network credits must cross
the authenticated payment boundary and be checked against network evidence.
This crate is distinct from the C17 genesis cutover. Paxeer mainnet-beta is one
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

The connected funded-operator test requires `LAYERX_MIGRATION_FUNDED_OPERATOR_FIXTURE`, an absolute path to a genuine protected funded-operator fixture. The connected migration V2 test requires `LAYERX_MIGRATION_V2_FIXTURE`, an absolute path to a genuine protected migration V2 fixture. These tests cannot establish connected behaviour without those protected
producer inputs. The inventory repair gate excludes them by name
while compiling both test targets; that exclusion is not a connected
qualification result.
