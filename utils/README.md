# Paxeer node utilities

This directory holds shared Go helpers used by `paxd` and the chain modules in the merged Paxeer network. They support EVM and Pax address recovery and association, transaction prioritization, asset metadata, slice operations, concurrent collections, panic handling, logging, and chain metrics.

## Layout

- [`datastructures/`](datastructures/) — concurrent sets and typed maps, including ordered iteration and nested maps.
- [`helpers/`](helpers/) — signature recovery, public-key conversion, address association, and balance migration; `legacy/` contains version-specific helpers for historical replay.
- [`logging/`](logging/) — reports operations that exceed an expected duration and counts those reports with OpenTelemetry.
- [`metrics/`](metrics/) — chain counters, gauges, histograms, processing labels, and the OpenTelemetry Prometheus provider.
- [`constants.go`](constants.go) — shared big-integer and SDK integer values.
- [`constants_test.go`](constants_test.go) — tests the shared integer values.
- [`interface.go`](interface.go) — the `IAsset` interface for native, CW20, and CW721 assets.
- [`metadata.go`](metadata.go) — the `ERCMetadata` name, symbol, and decimals fields.
- [`metadata_test.go`](metadata_test.go) — metadata field and comparison tests.
- [`panic.go`](panic.go) — panic recovery callbacks, stack-trace logging, and errors that must propagate as panics.
- [`panic_test.go`](panic_test.go) — tests error decoration and detection for panic propagation.
- [`prioritized_txs.go`](prioritized_txs.go) — classifies pricefeed submissions and oracle vote and delegation transactions for prioritization.
- [`prioritized_txs_test.go`](prioritized_txs_test.go) — transaction prioritization tests.
- [`slice.go`](slice.go) — generic mapping, filtering, reduction, and copying helpers.
- [`slice_test.go`](slice_test.go) — slice filtering, mapping, and copying tests.

## Build and test

These packages belong to the root Go module declared in [`go.mod`](../go.mod). The directory has no separate manifest or Makefile. Run the covering targets from the repository root:

```sh
make paxeer-build
make paxeer-test
```

The root [`Makefile`](../Makefile) delegates these targets to [`chain.mk`](../chain.mk). The build compiles `paxd` with its native prerequisites. The test target discovers Go packages with tests and runs them in groups with race detection and coverage, including the utility tests.

## Configuration

The utility sources read no environment variables or configuration files directly. Callers supply contexts, keepers, durations, and metric labels. `SetupOtelMetricsProvider` installs the Prometheus-backed OpenTelemetry provider; application telemetry configuration belongs to the node.

## Documentation

- [Node layout and build commands](../docs/paxeer-network.md).
- [EVM execution and address association](../docs/site/paxeer-docs/public/markdown/evm.md) in the documentation site.
- [Node configuration](../docs/site/paxeer-docs/public/markdown/configuration.md) in the documentation site, including application telemetry.
- [Chain metrics](metrics/README.md).
