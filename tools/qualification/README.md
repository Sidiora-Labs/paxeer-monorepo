# Qualification tools

This directory holds qualification runners, integration helpers and tests for
the Paxeer X Network and its applications. They exercise node, program, wallet,
Human, platform and interoperability behavior and validate evidence against its
source and artifacts. The network runs through `paxd`, with the kernel taking
part in block proposal and execution; receipts are final when their blocks are
committed.

## Layout

- `audit_repairs/` — focused source and behavior tests with associated KVX data.
- `localnode/` — disposable node and perps gateway scripts, Go integration tests and Python precompile checks.
- `paxeer-x/` — network and application qualification clients, shared ABI and candidate helpers, fixtures and tests.
- `tests/` — focused tests for qualification inputs and application behavior.
- `__init__.py` — Python package declaration.
- `release_runner.py` — runs component targets and validates external case inventories, source identities and artifact digests.
- `beta_driver.py` — executes adoption, program, interoperability and multichain cases and collects committed-node evidence.
- `paxeer_x_release.py` — prepares candidate-bound requests, communicates with the qualification controller and validates completed evidence.
- `requirements.txt` — Python dependencies for the qualification tools and tests.
- `conftest.py` — declares the `native` pytest marker for tests that compile against node libraries.
- `test_release_runner.py` — runner and evidence validation tests.
- `test_beta_driver.py` — beta driver, sample and committed-evidence tests.
- `test_native_program_human_peer.py` and `test_program_market_attestation.py` — program integration checks.
- `test_registry_router_bootstrap.py` — registry bootstrap checks.
- `test_retired_callers_1.py` through `test_retired_callers_18.py` — checks for callers of disabled interfaces.
- `test_service_catalogue_1.py`, `test_service_catalogue_2.py` and `test_service_catalogue_3.py` — service catalogue checks.

## Build and test

Run commands from the repository root. These Python entry points have no
standalone compiled build. The root `Makefile` supplies their test targets and
the component builds used by the release runner.

| Command | Purpose |
| --- | --- |
| `python3 -m pip install -r tools/qualification/requirements.txt` | Install the dependencies used by the Python qualification suites, as in the CI workflows. |
| `make test-qualification-python` | Run the directory's pytest suite excluding cases marked `native`. |
| `make qualification-runner-test` | Run the MCP binding receipt-key prerequisite and release runner unit tests. |
| `make beta-driver-test` | Run the release runner and beta driver unittest modules. |
| `make beta-qualify` | Run the beta component builds and checks and validate external qualification evidence. |
| `make human-qualify` | Run Human builds, checks, journeys and external evidence validation. |
| `make platform-qualify` | Run the platform component builds, tests and external evidence validation. |
| `make programs-qualify` | Run program tests, conservation checks and external evidence validation. |

The qualification targets that consume external evidence require the configured
driver and services. `release_runner.py` validates the driver's digest, case
coverage and artifact contents before accepting its evidence. The local node
helpers also use the node's native libraries; the Python-only target above
does not run those native cases.

## Configuration

The main runner reads these environment variables:

- `LAYERX_QUALIFICATION_DRIVER` and `LAYERX_QUALIFICATION_DRIVER_SHA256` select and bind the external driver.
- `LAYERX_QUALIFICATION_REAL_STACK` declares the use of running services.
- `LAYERX_QUALIFICATION_NODE_URL`, `LAYERX_QUALIFICATION_AGENT_URL` and `LAYERX_QUALIFICATION_HUMAN_URL` identify the service endpoints.
- `LAYERX_QUALIFICATION_ARTIFACT_DIR` selects the artifact directory.

The beta driver's case inputs are declared in `beta_driver.py`:

- Network trust: `LAYERX_QUALIFICATION_NETWORK_TRUST`, `LAYERX_PAXEER_CHAIN_ID`.
- Samples: `LAYERX_API_TOKEN`, `LAYERX_SOURCE`, `LAYERX_DESTINATION`, `LAYERX_AMOUNT`, `LAYERX_CURRENCY`.
- CLI: `LAYERX_NETWORK_ID`, `LAYERX_SEQUENCER_TRUST_ANCHOR`, `LAYERX_CLI_ENVIRONMENT`, `LAYERX_BIN`, `LAYERX_CREDENTIAL_STORE`.
- Toolchains: `PLATFORM_PYTHON_ENV`, `PLATFORM_MAVEN_REPOSITORY`, `CARGO_TARGET_DIR`.
- Mirrors: `LAYERX_MIRROR_VERIFY_CONFIG`, `LAYERX_MIRROR_CANONICAL_REQUEST`, `LAYERX_MIRROR_FAILOVER_REQUEST`, `LAYERX_MIRROR_DIVERGENCE_REQUEST`, `LAYERX_MIRROR_TAMPER_REQUEST`, `LAYERX_MIRROR_TS_CONFORMANCE`, `LAYERX_MIRROR_PYTHON_CONFORMANCE`, `LAYERX_MIRROR_GO_CONFORMANCE`, `LAYERX_MIRROR_JVM_CONFORMANCE`, `LAYERX_MIRROR_SWIFT_CONFORMANCE`, `LAYERX_MIRROR_DOTNET_CONFORMANCE`.
- Migration: `LAYERX_ETHEREUM_CONFIG`, `LAYERX_ETHEREUM_ASSET_EVIDENCE`, `LAYERX_ETHEREUM_HISTORY_EVIDENCE`, `LAYERX_ETHEREUM_OWNERSHIP_EVIDENCE`, `LAYERX_SOLANA_CONFIG`, `LAYERX_SOLANA_ASSET_EVIDENCE`, `LAYERX_SOLANA_HISTORY_EVIDENCE`, `LAYERX_SOLANA_OWNERSHIP_EVIDENCE`, `LAYERX_MIGRATION_SECRET_DIR`.
- Ramps: `LAYERX_RAMP_URL`, `LAYERX_RAMP_CA_PEM`, `LAYERX_RAMP_CUSTOMER_TOKEN`, `LAYERX_RAMP_OPERATOR_URL`, `LAYERX_RAMP_OPERATOR_TOKEN`, `LAYERX_RAMP_ON_QUOTE_ID`, `LAYERX_RAMP_OFF_QUOTE_ID`, `LAYERX_RAMP_OFF_GRANT_JSON`, `LAYERX_RAMP_ON_ACCOUNT_SEQUENCE`, `LAYERX_RAMP_OFF_RECEIVER_SEQUENCE`.

The network trust input names a JSON file. Mirror and migration configuration
and request files are selected by their case inputs. The driver also reads the
component `Cargo.toml` manifests under `services/agent`, `services/human`,
`services/platform`, `services/interop` and `programs`.

`paxeer_x_release.py` accepts candidate, contract, request, trust and completed
evidence files through its command-line options. It reads `PAXEER_X_MAINLINE`,
`PAXEER_X_REQUEST`, `PAXEER_X_REQUEST_ID`, `PAXEER_X_CANDIDATE_JSON`,
`PAXEER_X_CONTRACT_JSON`, `RUNNER_TEMP`, `GITHUB_SHA`, `GITHUB_RUN_ID` and
`GITHUB_RUN_ATTEMPT`. Its service catalogue is
`spec/layerx-merge/spec.kvx`, and it binds requests to
`.github/workflows/paxeer-x-qualification.yml`.

The local node helpers use `TMPDIR` and a generated `localnode.env` file;
`perps-gateway.sh` accepts an environment file with `--env`. Local integration
inputs include `LOCAL_NODE_RPC`, `LOCAL_NODE_HOME`, `LOCAL_NODE_KEY`,
`LOCAL_NODE_ADDRESS`, `LOCAL_NODE_PROFILE_RPC`, `LOCAL_NODE_SEQUENCER_RPC`,
`PERPS_DATA_TEST_RPC`, `PERPS_DATA_TEST_COMET`,
`PERPS_DATA_TEST_DATABASE_URL` and `NEXT_PUBLIC_PERPS_GATEWAY_URL`.

## Documentation

- [Network qualification](../../docs/QUALIFICATION.md)
- [Release readiness matrix](../../docs/release/7.0.0-readiness-matrix.md)
- [Network upgrade plan](../../docs/release/7.0.0-upgrade-plan.md)
- [Program workspace](../../docs/wiki/ProgramsWorkspace.md)
- [Network operations on the documentation site](../../docs/site/paxeer-docs/src/app/network-operations/page.tsx)
