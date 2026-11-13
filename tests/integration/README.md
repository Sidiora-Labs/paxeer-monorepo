# Integration Test Framework

This directory holds YAML-driven integration tests, contract fixtures, shell
probes, and Go, Python, JavaScript and TypeScript suites for `paxd`. They exercise
the node's modules, EVM interfaces, storage, consensus and upgrades. Paxeer
mainnet-beta is one BFT network, with the kernel participating in block proposal;
a receipt is final when its block is committed, and its EVM chain ID is 125
(`0x7d`). The commands below target a disposable local cluster whose genesis
settings can differ from mainnet. YAML commands run sequentially and verifiers
check captured output; the Python and Go drivers share the files but have
different expression evaluators.

## Layout

- `authz_module/` — authorization YAML scenarios and committed-result helpers.
- `autobahn/` — Go tests for Autobahn consensus and cluster behavior.
- `bank_module/` — transfers, multisignature sends and transaction simulation.
- `chain_operation/` — snapshot and state-sync scenarios and a snapshot probe.
- `contracts/` — Wasm fixtures, deployment scripts and FlatKV recovery checks.
- `dapp_tests/` — Hardhat scenarios for Uniswap, Steak and an NFT marketplace.
- `db/` — state-store and FlatKV EVM YAML scenarios.
- `distribution_module/` — community-pool and rewards scenarios.
- `evm_module/` — RPC input/output fixtures, WebSocket tests and EVM shell suites.
- `gov_module/` — governance scenarios and proposal files.
- `mint_module/` — minting scenarios.
- `oracle_module/` — feeder, penalty and committed-vote checks.
- `rpc_tests/` — TypeScript JSON-RPC assertions with a local geth reference.
- `runner/` — Go YAML driver, suite registry and runner tests.
- `scripts/` — Python YAML driver and its tests.
- `staking_module/` — staking scenarios.
- `startup/` — cluster startup scenario.
- `template/` — YAML authoring template.
- `tokenfactory_module/` — token-factory creation scenario.
- `upgrade_module/` — upgrade scenarios, runtime helpers and merge-upgrade probe.
- `utils/` — shared transaction and receipt helpers.
- `wasm_module/` — timelocked-token delegation, administration and withdrawal scenarios.

- [scripts/runner.py](scripts/runner.py) — Python YAML driver.
- [runner/runner.go](runner/runner.go) — Go YAML driver.
- [runner/suites.go](runner/suites.go) — ordered Go suite registry.
- [dapp_tests/package.json](dapp_tests/package.json) — dApp dependencies.
- [rpc_tests/package.json](rpc_tests/package.json) — RPC dependencies and commands.

## Build and test

### Prerequisites

- Docker and Docker Compose, with the daemon running.
- Python 3 and PyYAML for the Python driver (`python3 -m pip install pyyaml`).
- The repository's Go toolchain and native dependencies for the Go driver.

### Usage

From the repository root, start the local four-node cluster:

```bash
DOCKER_DETACH=true make docker-cluster-start
```

Run a YAML file through the [Python driver](scripts/runner.py):

```bash
python3 tests/integration/scripts/runner.py tests/integration/startup/startup_test.yaml
```

Or run a registered suite through the [Go driver](runner/runner.go):

```bash
go test -tags yaml_integration -v ./tests/integration/runner/... -run '^TestBankModule$'
```

[suites.go](runner/suites.go) lists suite names and YAML files in execution order.
The Go integration wrappers require the `yaml_integration` build tag. Some
suites need additional infrastructure: `TestStateSync` requires `pax-rpc-node`,
provided by `make run-rpc-node-integration-ci`. Upgrade YAML files are driven
separately through Python by the upgrade workflow.

### Contract builds and additional suites

The directory has no separate Makefile. The root
[Makefile](../../Makefile) supplies these targets for the JavaScript modules,
including their Hardhat contract builds and static checks:

```bash
make paxeer-npm-install
make paxeer-npm-build
make paxeer-npm-static-test
```

The [RPC manifest](rpc_tests/package.json) supplies the following commands.
`rpc:ci` installs dependencies, compiles contracts, starts its geth reference,
bootstraps fixtures, runs the assertions and merges reports. It requires a
running Paxeer cluster; `test:rpc` also requires a running geth reference.

```bash
npm --prefix tests/integration/rpc_tests run compile
npm --prefix tests/integration/rpc_tests run test:rpc
npm --prefix tests/integration/rpc_tests run rpc:ci
```

The local dApp scripts install dependencies, compile their contracts and run
their selected scenarios. Run them from the repository root:

```bash
./tests/integration/dapp_tests/dapp_uniswap_tests.sh
./tests/integration/dapp_tests/dapp_nft_steak_tests.sh
```

The root target below runs the Autobahn tests, whose `TestMain` manages their
cluster. It is defined in [chain.mk](../../chain.mk).

```bash
make autobahn-integration-test
```

The YAML, JSON, Wasm, `.io` and `.iox` files are scenario data and contract
fixtures consumed by the drivers and deployment scripts.

### Writing tests

Each file is a YAML list of cases. The [template](template/template_test.yaml)
shows the field layout. Use the common expression subset below for files shared
by the Python and Go drivers.

```yaml
- name: Validator count is four
  inputs:
    - cmd: paxd q tendermint-validator-set | grep address | wc -l
      env: RESULT
  verifiers:
    - type: eval
      expr: RESULT == 4
    - type: regex
      result: RESULT
      expr: '^4$'
```

#### Explanation

| Field | Required | Meaning |
| ----- | -------- | ------- |
| `name` | Yes | Case name; becomes a named Go subtest. |
| `inputs` | Yes | Commands executed in list order. |
| `cmd` | Yes | Shell command inside the chosen container. |
| `env` | No | Capture trimmed stdout under this name for later inputs and verifiers in this case. |
| `node` | No | Container name; defaults to `pax-node-0` in both drivers. |
| `verifiers` | Yes | Checks evaluated after the inputs finish. |
| `type` | Yes | `eval` or `regex`. |
| `result` | For `regex` | Captured output name to match. |
| `expr` | Yes | Comparison expression or regular expression. |

#### Expression rules shared by both drivers

For portable `eval` checks, use one comparison per verifier:

```text
operand operator operand
```

Separate every token with spaces. Operators are `==`, `!=`, `>`, `<`, `>=`
and `<=`; operands can be captured names or canonical decimal integers
(without leading zeroes except `0`). For simple string
equality, use a captured single-token value and a double-quoted literal, such as
`STATUS == "ready"`. Keep output containing quotes, backslashes or multiple
lines out of `eval`; extract a simple value with `jq` or use `regex` instead.

The Python evaluator parses a restricted expression AST, resolves captured names
as values, and evaluates with built-ins disabled. It supports comparisons,
`and`, `or`, `not`, unary signs, and arithmetic including `//` and `%`; calls,
attribute access and indexing are rejected. Captured decimal integers remain
integers, finite numeric values become floats, and other output remains a string.

The Go evaluator uses whitespace-delimited tokens and preserves quoted string
literals. It supports comparisons, `and`, `or`, and `+`, `-`, `*`, `/`.
Multiplication and division bind before addition and subtraction; `and` binds
before `or`. Integer addition, subtraction and multiplication use `big.Int`;
division and decimal arithmetic use floating point. It compares integer values
first, then floating-point values, then strings. It does not implement the
Python evaluator's full grammar. Compute complex arithmetic in commands or use
separate verifiers when sharing a file between drivers.

For `regex`, both drivers match from the start of trimmed output. Anchor shared
patterns with `^` and `$` for a whole-output check and use syntax supported by
both; Go does not support Python lookaround or backreferences.

#### Command status and timeouts

Both drivers require a successful command exit by default and fail a case when
a command exits unexpectedly or exceeds its time limit. Their optional input
fields differ:

| Driver | Expected exit field | Timeout |
| ------ | ------------------- | ------- |
| Python | `expected_status` (integer 0–255, default 0) | `timeout` per input, or the runner's 300-second default; must be positive and finite. |
| Go | `expected_exit_code` (integer 0–255, default 0) | `WithCommandTimeout`, default two minutes; `WithContext` also supports cancellation. |

Use the matching field when asserting a deliberate command failure. The other
driver does not interpret that field. Shell commands execute through Bash by
default; captured environment variables are passed as process arguments.

#### Notes and tips

1. Captures are reset for each case and passed to later commands as environment
   variables; they are not global across cases.
2. Commands and captured environment values are passed as separate process
   arguments. Shell syntax belongs in the command; captured values are not
   interpolated into that command by the driver.
3. Both drivers capture stdout and trim surrounding whitespace. Verifiers
   check that output after the command's expected exit status is satisfied.
4. Tests mutate a running chain and may not be idempotent. Preserve suite order
   and use fresh local state when a scenario requires it.

## Configuration

The YAML files configure commands, captures, containers and verifiers. The Go
driver also accepts `WithContainer`, `WithExtraPath`, `WithShell`, `WithContext`
and `WithCommandTimeout` options. The cluster startup target reads
`DOCKER_DETACH` and uses [docker/docker-compose.yml](../../docker/docker-compose.yml).

The nested suites read these environment variables; names are listed without
values:

| Consumer | Environment variables |
| -------- | --------------------- |
| dApp suites | `DAPP_TEST_ENV`, `DAPP_TESTS_MNEMONIC`, `PAXEER_LOCAL_EVM_RPC_URL`, `PAXEER_DEVNET_EVM_RPC_URL`, `PAXEER_TESTNET_EVM_RPC_URL` |
| RPC endpoints and accounts | `PAX_EVM_RPC`, `PAX_EVM_WS`, `PAX_COSMOS_RPC`, `PAX_REST`, `RPC_ETH_GETH`, `RPC_ETH_FORK`, `RPC_ACCOUNTLESS`, `PAX_ADMIN_MNEMONIC` |
| RPC execution | `RPC_TESTS_RUNTIME_STATE`, `RPC_POLLING_INTERVAL_MS`, `RPC_EXPECTED_EVM_CHAIN_ID`, `RPC_NETWORK_MODE`, `RPC_TESTS_WASM_MODE`, `PAX_TIMEOUT`, `GETH_TIMEOUT`, `SKIP_NPM_CI`, `GETH_VERSION` |
| Optional Hardhat reference fork | `ETH_MAINNET_UPSTREAM`, `ETH_MAINNET_FORK_BLOCK` |
| EVM input/output tests | `PAX_EVM_IO_RUN_INTEGRATION`, `PAX_EVM_RPC_URL`, `PAX_EVM_IO_DEBUG_FILES`, `PAX_EVM_IO_SEED_BLOCK`, `PAX_EVM_IO_DEPLOY_TX_HASH`, `PAX_EVM_IO_REVERTER_ADDRESS`, `PAX_EVM_IO_TX_CONTAINER`, `PAX_EVM_IO_TX_PASSWORD`, `PAX_EVM_IO_TX_FROM`, `PAX_EVM_IO_TX_RECIPIENT`, `PAX_EVM_IO_PROJECT_ROOT`, `PAX_EVM_IO_KEYRING_BACKEND` |
| WebSocket tests | `PAX_EVM_WS_RUN_INTEGRATION`, `PAX_EVM_WS_URL`, `PAX_EVM_HTTP_URL` |
| Transaction helpers | `TX_WAIT_TIMEOUT`, `TX_WAIT_INTERVAL` |
| Upgrade runtime | `PAXD_BIN`, `PAX_HOME`, `PAXD_RPC_URL`, `PAX_RUN_DIR`, `ID`, `UPGRADE_VERSION_LIST`, `INVARIANT_CHECK_INTERVAL`, `UPGRADE_LAUNCH_TIMEOUT`, `PAXD_PROGRESS_TIMEOUT_SECONDS`, `PAXD_TARGET_SAMPLES`, `PAXD_TARGET_SAMPLE_WINDOW_SECONDS`, `PAXD_RESTART_RETRY_SECONDS`, `PAXD_RESTART_MAX_ATTEMPTS` |
| Upgrade height polling | `WAIT_FOR_HEIGHT_STATUS_TIMEOUT_SECONDS`, `WAIT_FOR_HEIGHT_MAX_WAIT_SECONDS`, `WAIT_FOR_HEIGHT_MAX_NO_PROGRESS_SECONDS` |

The FlatKV deployment and recovery scripts accept these additional variables:

| Script in `contracts/` | Environment variables |
| ---------------------- | --------------------- |
| [deploy_flatkv_evm_fixture.sh](contracts/deploy_flatkv_evm_fixture.sh) | `EVM_RPC_URL`, `FLATKV_EVM_BULK_STORAGE_KEYS`, `FLATKV_EVM_BULK_STORAGE_KEYS_PER_CONTRACT`, `FLATKV_EVM_FIXTURE_CHAIN_ID`, `FLATKV_EVM_FIXTURE_FROM`, `FLATKV_EVM_FIXTURE_KEYRING_BACKEND`, `FLATKV_EVM_FIXTURE_MISSING`, `FLATKV_EVM_FIXTURE_PASSWORD`, `FLATKV_EVM_FIXTURE_RECIPIENT`, `FLATKV_EVM_FIXTURE_TRANSFER_VALUE_WEI` |
| [verify_cross_validator_flatkv_digest.sh](contracts/verify_cross_validator_flatkv_digest.sh) | `FLATKV_DIGEST_COMPARE_BUFFER`, `FLATKV_DIGEST_MIN_HEIGHT`, `FLATKV_DIGEST_NODE_COUNT`, `FLATKV_DIGEST_WAIT_TIMEOUT`, `FLATKV_DIR`, `GOPROXY`, `GO_BIN` |
| [verify_flatkv_crash_recovery.sh](contracts/verify_flatkv_crash_recovery.sh) | `FLATKV_CRASH_CATCHUP_TIMEOUT`, `FLATKV_CRASH_COMPARE_BUFFER`, `FLATKV_CRASH_DOWN_SECS`, `FLATKV_CRASH_NODE_COUNT`, `FLATKV_CRASH_NODE_INDEX`, `FLATKV_CRASH_RESTART_PROBE_SECS`, `FLATKV_CRASH_SURVIVOR`, `FLATKV_CRASH_SURVIVOR_TIMEOUT`, `FLATKV_DIR`, `GOPROXY`, `GO_BIN` |
| [verify_flatkv_evm_migrate.sh](contracts/verify_flatkv_evm_migrate.sh) | `APP_CONFIG`, `FLATKV_DIR`, `GOPROXY`, `GO_BIN`, `MIGRATE_COMPARE_BUFFER`, `MIGRATE_COMPLETION_TIMEOUT`, `MIGRATE_FIXTURE_HEIGHT_FILE`, `MIGRATE_HALT_TIMEOUT`, `MIGRATE_KEYS_PER_BLOCK`, `MIGRATE_MIN_HEIGHT_AFTER`, `MIGRATE_MIN_KEYS_MIGRATED`, `MIGRATE_NODE_COUNT`, `MIGRATE_PREFLIP_HALT_BLOCKS`, `MIGRATE_PREFLIP_SETTLE_BLOCKS`, `MIGRATE_PREFLIP_SYNC_TIMEOUT`, `MIGRATE_RESTART_PROBE_SECS`, `MIGRATE_STOP_TIMEOUT` |
| [verify_flatkv_evm_store.sh](contracts/verify_flatkv_evm_store.sh) | `FLATKV_DIR`, `FLATKV_EVM_DUMP_DIR`, `GOPROXY` |
| [verify_flatkv_only_statesync.sh](contracts/verify_flatkv_only_statesync.sh) | `APP_CONFIG`, `FLATKV_DIR`, `FLATKV_ONLY_NODE_COUNT`, `FLATKV_ONLY_STATESYNC_CATCHUP_TIMEOUT`, `FLATKV_ONLY_STATESYNC_CATCHUP_TOLERANCE`, `FLATKV_ONLY_STATESYNC_COMPARE_BUFFER`, `FLATKV_ONLY_STATESYNC_DONOR`, `FLATKV_ONLY_STATESYNC_KILL_TIMEOUT`, `FLATKV_ONLY_STATESYNC_MIN_DONOR_HEIGHT`, `FLATKV_ONLY_STATESYNC_SECOND_RPC`, `FLATKV_ONLY_STATESYNC_SMOKE_TIMEOUT`, `FLATKV_ONLY_STATESYNC_SNAPSHOT_WAIT_TIMEOUT`, `FLATKV_ONLY_STATESYNC_STOP_TIMEOUT`, `FLATKV_ONLY_STATESYNC_THIRD_PEER`, `FLATKV_ONLY_STATESYNC_TRUST_LAG`, `FLATKV_ONLY_STATESYNC_VICTIM_INDEX`, `GOPROXY`, `GO_BIN` |
| [verify_flatkv_partial_loss_fails_loudly.sh](contracts/verify_flatkv_partial_loss_fails_loudly.sh) | `FLATKV_DIR`, `FLATKV_PARTIAL_LOSS_CATCHUP_TIMEOUT`, `FLATKV_PARTIAL_LOSS_CATCHUP_TOLERANCE`, `FLATKV_PARTIAL_LOSS_COMPARE_BUFFER`, `FLATKV_PARTIAL_LOSS_NODE_COUNT`, `FLATKV_PARTIAL_LOSS_RESTART_OBSERVE_SECS`, `FLATKV_PARTIAL_LOSS_VICTIM_INDEX`, `GOPROXY`, `GO_BIN` |
| [verify_flatkv_statesync_crash_recovery.sh](contracts/verify_flatkv_statesync_crash_recovery.sh) | `FLATKV_DIR`, `FLATKV_IMPORT_HEIGHT_FILE`, `FLATKV_STATESYNC_CATCHUP_TIMEOUT`, `FLATKV_STATESYNC_CATCHUP_TOLERANCE`, `FLATKV_STATESYNC_DONOR`, `FLATKV_STATESYNC_KILL_WINDOW_SECS`, `FLATKV_STATESYNC_MIN_DONOR_HEIGHT`, `FLATKV_STATESYNC_MIN_SNAPSHOT_HEIGHT`, `FLATKV_STATESYNC_RECOVERY_SCENARIO`, `FLATKV_STATESYNC_SECOND_RPC`, `FLATKV_STATESYNC_SNAPSHOT_WAIT_TIMEOUT`, `FLATKV_STATESYNC_TRUST_LAG`, `FLATKV_STATESYNC_VICTIM_INDEX`, `GOPROXY`, `GO_BIN` |
| [verify_flatkv_total_loss_recovery.sh](contracts/verify_flatkv_total_loss_recovery.sh) | `FLATKV_DIR`, `FLATKV_IMPORT_HEIGHT_FILE`, `FLATKV_TOTAL_LOSS_CATCHUP_TIMEOUT`, `FLATKV_TOTAL_LOSS_CATCHUP_TOLERANCE`, `FLATKV_TOTAL_LOSS_DONOR`, `FLATKV_TOTAL_LOSS_MIN_DONOR_HEIGHT`, `FLATKV_TOTAL_LOSS_MIN_SNAPSHOT_HEIGHT`, `FLATKV_TOTAL_LOSS_SECOND_RPC`, `FLATKV_TOTAL_LOSS_SNAPSHOT_WAIT_TIMEOUT`, `FLATKV_TOTAL_LOSS_STOP_TIMEOUT`, `FLATKV_TOTAL_LOSS_TRUST_LAG`, `FLATKV_TOTAL_LOSS_VICTIM_INDEX`, `GOPROXY`, `GO_BIN` |
| [verify_statesync_flatkv_digest.sh](contracts/verify_statesync_flatkv_digest.sh) | `FLATKV_DIGEST_COMPARE_BUFFER`, `FLATKV_DIGEST_DONOR`, `FLATKV_DIGEST_MIN_HEIGHT`, `FLATKV_DIGEST_RECEIVER`, `FLATKV_DIGEST_WAIT_TIMEOUT`, `FLATKV_DIR`, `GOPROXY`, `GO_BIN` |

The dApp configuration is in [hardhat.config.js](dapp_tests/hardhat.config.js)
and [constants.js](dapp_tests/constants.js). Native CLI and pointer operations
use the latter's maps separately from the Hardhat provider settings.
The RPC suite reads [config/endpoints.ts](rpc_tests/config/endpoints.ts),
[hardhat.config.ts](rpc_tests/hardhat.config.ts),
[hardhat/hardhat.config.ts](rpc_tests/hardhat/hardhat.config.ts),
[tsconfig.json](rpc_tests/tsconfig.json),
[.mocharc.bootstrap.json](rpc_tests/.mocharc.bootstrap.json) and
[.mocharc.run.json](rpc_tests/.mocharc.run.json). Bootstrap produces runtime
state subsequently read by the RPC assertions.

Snapshot probes read the node home's `config/genesis.json`, `config/config.toml`
and `config/app.toml`; upgrade helpers read `config/client.toml`. FlatKV scripts
read their application configuration through `APP_CONFIG` and select storage and
tool locations through `FLATKV_DIR`, `GO_BIN` and `GOPROXY`.

## Documentation

- [dApp scenarios](dapp_tests/README.md): EVM, CosmWasm and pointer interactions.
- [EVM RPC fixtures](evm_module/rpc_io_test/RPC_IO_README.md): `.io` / `.iox` method schemas and deterministic response checks.
- [RPC parity suite](rpc_tests/README.md): TypeScript assertions against a local geth reference.
- [EVM JSON-RPC behavior](../../docs/evm_jsonrpc_unsupported.md): method refusals and integration fixture coverage.
- [Documentation site JSON-RPC reference](../../docs/site/paxeer-docs/public/markdown/json-rpc.md).
- [Documentation site public RPC reference](../../docs/site/paxeer-docs/public/markdown/public-rpc.md): public JSON-RPC uses the node endpoint's root path.
