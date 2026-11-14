# tools/launchpad

tools/launchpad contains Python and shell utilities for qualifying launchpad services and SDKs, checking delivery and storage behavior, measuring cache and chart capacity, and enforcing coverage inventories. These checks support components that run on the single Paxeer mainnet-beta network, whose EVM chain ID is 125: the LayerX kernel inside `paxd` executes and orders each transaction and builds each block; validators check and commit it, and a receipt is final when the validators commit its block.

## Layout

- `cache/` — cache policy checks, Redis integration checks, and capacity measurement.
- `delivery/` — delivery qualification, archive and stream manifest validation, measurement helpers, and fixtures.
- `production/` — bootstrap, chart, dispatcher, feature-acceptance, integration, interface, load, serial-gate, and production test tooling.
- `storage/` — PostgreSQL storage fixture, recovery, and durability checks.
- `coverage-gate.sh` — checks run-bound Go coverage profiles against the configured thresholds.
- `coverage_inventory.py` — creates, records, finishes, and checks a coverage run manifest and its profiles.
- `requirements.txt` — Python dependency declaration for `jsonschema`.
- `test_coverage_gate.py` — Python tests for the coverage gate and its profile validation.

## Build and test

The directory has no standalone build target. The `services/launchpad/Makefile` drives the launchpad modules and uses the coverage helpers here. From the repository root, run:

```sh
make -C services/launchpad build
make -C services/launchpad test
make -C services/launchpad cover-check
```

The chart build and test scripts cover the trading-charts HTTP API and web chart datafeed:

```sh
bash tools/launchpad/production/charts-realtime/build.sh
bash tools/launchpad/production/charts-realtime/check.sh
```

The cache and delivery scripts also invoke these focused Python checks:

```sh
python3 tools/launchpad/cache/test_capacity.py
python3 -m unittest discover -s tools/launchpad/delivery -p 'test_measure_phase4.py'
```

The chart scripts require the web application's installed TypeScript and Vitest dependencies. Runtime cache, delivery, storage, integration, and load checks require their configured services and inputs; their entry points live in the corresponding subdirectories.

## Configuration

Sources read these environment variable names; values are supplied by the invoking qualification or fixture:

- Coverage: `COVER_DIR`, `DEFAULT_MIN`, `MONEY_MIN`, `MONEY`, `MODULES`, `RUN_MANIFEST`, `COVER_RUN_ID`.
- Cache: `L09_REDIS_URL`, `L09_DATABASE_URL`, `L09_NAMESPACE`, `L09_PHASE`, `L09_CAPACITY_CONFIG`, `L09_CAPACITY_CONFIG_SHA256`, `L09_CAPACITY_OUTPUT`, `L09_IDENTITY_ENDPOINT`, `L09_IDENTITY_SERVICE_TOKEN`, `L09_WALLET_A_ACCESS`, `L09_WALLET_B_ACCESS`, `L09_EXCLUSIVE_REDIS_DB`.
- Storage: `L07_POSTGRES_IMAGE`, `L07_OLD_READER_BIN`, `L07_OLD_READER_SHA256`, `L07_NEW_READER_BIN`, `L07_NEW_READER_SHA256`.
- Delivery: `DELIVERY_TEST_DSN`, `DELIVERY_TEST_SCHEMA`, `DELIVERY_ARCHIVE_ENDPOINT`, `DELIVERY_ARCHIVE_BUCKET`, `DELIVERY_ARCHIVE_ACCESS_KEY`, `DELIVERY_ARCHIVE_SECRET_KEY`, `DELIVERY_ARCHIVE_FROM`, `DELIVERY_ARCHIVE_TO`, `DELIVERY_ARCHIVE_STREAM`, `DELIVERY_ARCHIVE_DEPLOYMENT`, `DELIVERY_ARCHIVE_DECODER_SHA`, `DELIVERY_ARCHIVE_ABI_SHA`, `DELIVERY_ARCHIVE_CODE_SHA`, `DELIVERY_PHASE4_LEDGER`, `DELIVERY_PHASE4_LEDGER_SHA256`, `DELIVERY_PHASE4_CANDIDATE_SHA256`.
- Charts: `CHART_MEASUREMENT_CONFIG`, `CHART_MEASUREMENT_CONFIG_SHA256`, `CHART_PEER_TOKEN`, `CHART_PEER_URL`, `L10_TEST_POSTGRES`, `L11_BROWSER_SOURCE_REVISION`, `L11_CHART_LICENSE_MANIFEST`.
- Build and contract fixtures: `CLIENT_DEPLOYMENT_HELPER_OUTPUT`, `CLIENT_SOURCE_ROOT`, `CONTRACT_FREEZE_OUTPUT`, `CONTRACT_SOURCE_ROOT`, `DOCKER_HOST`, `FIXTURE_CHAIN_ID`, `FIXTURE_OUTPUT`, `ROUTER_ADDRESS`.
- Load checks: `LOAD_PHASE2_CONFIG_FILE`, `LOAD_PHASE2_CONFIG_SHA256`, `LOAD_PHASE2_OUTPUT_DIR`, `LOAD_PHASE3_CONFIG_FILE`, `LOAD_PHASE3_CONFIG_SHA256`, `LOAD_PHASE3_OUTPUT_DIR`, `LOAD_PHASE4_CONFIG_FILE`, `LOAD_PHASE4_CONFIG_SHA256`, `LOAD_PHASE4_OUTPUT_DIR`.
- Integration checks: `PHASE2_CONFIG_FILE`, `PHASE2_CONFIG_SHA256`, `PHASE2_OUTPUT_DIR`, `PHASE2_BROWSER_PHASE`, `PHASE3_CONFIG_FILE`, `PHASE3_CONFIG_SHA256`, `PHASE3_OUTPUT_DIR`, `PHASE3_BROWSER_PHASE`, `PHASE4_CONFIG_FILE`, `PHASE4_CONFIG_SHA256`, `PHASE4_OUTPUT_DIR`.
- Qualification and dispatcher: `PRODUCTION_DISPATCHER_SOURCE`, `PRODUCTION_DISPATCHER_SPEC_ROOT`, `QUALIFICATION_ACCEPTANCE_ID`, `QUALIFICATION_CANDIDATE_SHA256`, `QUALIFICATION_CONTRACT_ROOT`, `QUALIFICATION_EVIDENCE_DIR`, `QUALIFICATION_SOURCE_ROOT`, `QUALIFICATION_STARTED_AT`.

Configuration files are kept beside the tools that consume them. Notable inputs are `delivery/stream-manifest.json`, `production/bootstrap/build/manifest.json`, `production/bootstrap/check-registry.json`, `production/bootstrap/security/manifest.json`, `production/bootstrap/tests/source-catalog.json`, the phase manifests under `production/integration/`, `production/load/`, and `production/feature-acceptance/`, and `production/charts-realtime/phase4-config.schema.json`.

## Documentation

- [Launchpad production checklist](../../docs/launchpad/production-checklist.md)
- [Launch a market and chain](../../docs/launchpad/chain-launch.md)
- [Launchpad documentation page](../../docs/site/paxeer-docs/public/markdown/launchpad.md)
