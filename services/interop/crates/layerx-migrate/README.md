# layerx-migrate

Ethereum and Solana source-chain verifiers for the `migration` interop adapter. This crate imports provenance against pinned source deployments. It does not write LayerX balances and it is not the C17 genesis cutover.

Operator contract: [`OPERATIONS.md`](OPERATIONS.md). Workspace index: [`../../README.md`](../../README.md).

Wiki page: [`docs/wiki/Migration.md`](../../../../docs/wiki/Migration.md). LayerX kernel genesis import sections (different surface): [`migrations/`](../../../../migrations/README.md).

The crate also builds the `layerx-migrate` binary.

From the monorepo root: `make interop-test-migration`. The ignored live-testnet suite is `make interop-test-migration-testnets`; it needs an operator environment and runs in CI only through the manually dispatched `.github/workflows/interop-migration-testnets.yml`.

The connected funded-operator test requires `LAYERX_MIGRATION_FUNDED_OPERATOR_FIXTURE`, an absolute path to a genuine protected funded-operator fixture. The connected migration V2 test requires `LAYERX_MIGRATION_V2_FIXTURE`, an absolute path to a genuine protected migration V2 fixture. Decision 2496 qualifies these two tests through their producer inputs; the inventory repair gate excludes them by name while compiling both test targets.
