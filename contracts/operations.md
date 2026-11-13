# Operations tool contracts

This page describes the tools-ops interfaces and preserves the anchors used by the interface registry. Start at the [contracts overview](README.md) for Solidity project boundaries. Paths below are relative links to the owning implementation; checked-in deployment examples describe inputs, not evidence that a service is deployed.

## Network and endpoint contract

Paxeer mainnet-beta is one BFT network. The kernel runs inside the node and proposes blocks, and receipts become final when their block is committed. Its EVM chain ID is `125` (`0x7d`). Sub-chains are tenant domains in the database, not additional node deployments.

Public node JSON-RPC uses HTTPS at the configured node origin's root path. The node namespaces are `debug`, `echo`, `eth`, `net`, `pax`, `px`, `test` and `txpool`, subject to node configuration. Use the [public endpoint reference](../docs/site/docs/reference/public-rpc.md) for the full list and WebSocket examples. The `px` namespace answers from node state; `px_register` and `px_requestFunds` are operations of the hosted identity and faucet services declared in [the gateway OpenRPC contract](../services/platform/hosted/gateway/openrpc.json).

Native bank balances and block commitment govern network value and finality. External bridge operations have their own [bridge contract](bridge/README.md).

A download mirror, a CometBFT endpoint, a gRPC address and an EVM JSON-RPC URL are different configuration inputs.

Hosted-service dependency examples use `OPERATOR_HOST`, retaining the owning interface's scheme, port, path and method.
Replace `OPERATOR_HOST` with the operator-supplied hostname of the hosted service or dependency being configured.

For a read-only identity check:

```bash
curl --fail --silent --show-error --max-time 10 \
  "${RPC_URL:?Set RPC_URL to the node HTTPS root origin}" \
  -H 'content-type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
```

Expect the JSON-RPC `result` to be `"0x7d"`; an HTTP success alone is not a chain-identity check. Keep the transaction hash after a broadcast timeout and query its status before resubmitting.

## Bench config

The embedded benchmark client is [`tools/benchmark/loadtest`](../tools/benchmark/loadtest), with the JSON contract in [`types.go`](../tools/benchmark/loadtest/types.go) and a local sample in [`config.json`](../tools/benchmark/loadtest/config.json). Its `--config-file` option defaults to `tools/benchmark/loadtest/config.json` under the working directory. Run it from the repository root when using that default.

| Fields | Meaning and encoding |
| --- | --- |
| `chain_id` | Textual native-chain ID for signing; distinct from numeric EVM chain ID 125. The sample's `pax-loadtest-testnet` is a fixture value. |
| `grpc_endpoints`, `evm_rpc_endpoints` | Comma-separated strings, not JSON arrays. `EVMRpcEndpoint()` selects the first EVM URL for contract deployment. |
| `blockchain_endpoint`, `node_uri`, `tls` | Native-chain HTTP endpoint, node URI and gRPC TLS selection. Sample values use local ports 26657 and 9090; EVM uses 8545. |
| `target_tps`, `max_accounts`, `msgs_per_tx`, `ticks`, `metrics_port` | Unsigned integer workload controls. Read the workload loop before choosing a run length; these are not mainnet performance guarantees. |
| `message_types` | Array of workload names checked by `ValidateMessageTypes`; unknown names fail with `unrecognized message types`. |
| `message_configs` | Map from message name to unsigned `gas` and `fee`; includes the `default` entry. |
| `price_distribution`, `quantity_distribution` | Decimal-string `min` and `max`, plus integer `number_of_distinct_values`; the latter must be positive for sampling. |
| `message_type_distribution` | `dex` has `limit_order_percentage` and `market_order_percentage`; `staking` has delegate, undelegate and begin-redelegate percentages. Each sampled distribution must sum to one. |
| `contract_distribution` | Array of `contract_address` and decimal-string `percentage`; sampled percentages must sum to one. Replace fixture addresses with contracts deployed on the selected chain. |
| `wasm_msg_types` | `wasm_mint_nft.contract_address`, `vortex.contract_address` and `num_orders_per_tx`, or `instantiate.code_id` and JSON-text `payload`. |
| `post_tx_evm_queries` | Integer counts `block_by_number`, `receipt` and `filters`. |
| `evm_use_eip1559_txs` | Boolean transaction-format selection. |

The current `Config` type does not declare `run_oracle` or `pax_tester_address`; their presence in a sample or registry row does not make them active settings. JSON decoding is not strict about unknown fields. Dynamic `EVMAddresses` are filled by the client, not an operator-supplied address map.

### Bench CLI

[`benchmark.sh`](../tools/benchmark/benchmark.sh) initializes and starts a dedicated benchmark node. `BENCHMARK_PHASE` selects `init`, `start` or `all` (default). Initialization uses benchmark build support and test accounts; use a disposable chain home. It invokes the [home validator](../tools/benchmark/home), which requires an exact-path `BENCHMARK_RESET_HOME` confirmation for existing state.

### Bench env

| Variable | Default / purpose |
| --- | --- |
| `PAX_HOME` | `$HOME/.pax-benchmark`; dedicated node state. |
| `BENCHMARK_RESET_HOME` | Empty; explicit reset input to the home validator. |
| `PAXD_BIN` | Empty; a supplied binary skips compilation and is copied to `$HOME/go/bin/paxd`. |
| `PORT_OFFSET`, `LOG_FILE` | `0` and empty; offset the node's ports and optionally redirect node output. |
| `BENCHMARK_CONFIG` | Script-relative `scenarios/evm.json`. |
| `BENCHMARK_TXS_PER_BATCH` | `1000`. |
| `GIGA_EXECUTOR`, `GIGA_OCC` | Both `false` in the single-node script. |
| `MOCK_BALANCES`, `DISABLE_INDEXER` | Both `true` for this local benchmark. |
| `DB_BACKEND`, `DEBUG` | `goleveldb` and `false`; backend choices also include `memdb`, `cleveldb` and `rocksdb`. |

### Bench compare

[`benchmark-compare.sh`](../tools/benchmark/benchmark-compare.sh) accepts at least two `label=commit` arguments. It checks out revisions to build binaries and clears its results directory, so run it in a disposable checkout with a dedicated `BASE_DIR` (default `/tmp/pax-bench`). `DURATION` defaults to 600 seconds, both Giga switches default to `true`, and `MAX_REGRESSION_PCT` defaults to 5. `RESULTS_ONLY=true` processes retained results without rebuilding and running nodes. [`tps_stats.py`](../tools/benchmark/tps_stats.py) owns comparison statistics and the regression result propagated by the wrapper.

### Bench contracts

The [benchmark EVM project](../tools/benchmark/loadtest/contracts/evm/foundry.toml) pins Solidity `0.8.20`, optimizer runs 1,000, `src`, `out` and its own `lib` directory. Its [vendoring record](../tools/benchmark/loadtest/contracts/evm/VENDORING.md) pins OpenZeppelin and Solmate. [`setup.sh`](../tools/benchmark/loadtest/contracts/evm/setup.sh) verifies their tree digests offline; it does not download dependencies. The chain project's [`bootstrap-libs.sh`](bootstrap-libs.sh) does not manage this directory.

The [ERC20 deploy wrapper](../tools/benchmark/loadtest/contracts/deploy_erc20.sh) takes the EVM endpoint as its first argument and requires `BENCH_DEPLOYER_KEY`. It can fund the derived deployer using the local `admin` account and broadcasts a Foundry deployment. These are local workload tools, not mainnet deployment instructions. Contract addresses must come from actual deployments, not the sample workload JSON.

### Bench formats

The benchmark JSON uses snake_case fields and native decimal strings as above. The Solidity [remappings](../tools/benchmark/loadtest/contracts/evm/remappings.txt), ABI/binding directories and Foundry TOML belong to this benchmark project; they are independent of `contracts/src` and `modules/evm/artifacts` described in the [contracts overview](README.md#updating-pointer-contracts).

### Bench routes

[`metrics.go`](../tools/benchmark/loadtest/metrics.go) registers `/healthz` and `/metrics` on the metrics HTTP server. It listens on all IPv4 interfaces at `metrics_port`, falling back to 9696 when the value is zero; the checked-in sample selects 9695. These routes describe the load generator, not chain finality or readiness.

### Bench metrics

The same implementation declares OpenTelemetry instruments `produce`, `consume` and `tps` under meter `loadtest`. Scraped names follow the telemetry exporter. A produced or sent transaction count is not proof of committed throughput.

## Paxload config

[`tools/loadtest`](../tools/loadtest/README.md) is the separate `paxload` client. [`LoadConfig`](../tools/loadtest/config/config.go) accepts numeric `chainId`, textual `paxChainID`, an `endpoints` array, account configuration, weighted `scenarios`, optional `settings` and `bounds`. Duration values in settings are strings such as `"10s"`. Use the selected chain's EVM identity and endpoints rather than copying a fixture profile unchanged.

### Paxload CLI

The [Cobra entry point](../tools/loadtest/main.go) requires `--config` (`-c`). Operational flags include `--workers`, `--tps`, `--nodes`, `--track-receipts`, `--track-blocks`, `--track-user-latency`, `--report-path` and `--metricsListenAddr`. Explicit CLI flags override profile settings, which override defaults in [`settings.go`](../tools/loadtest/config/settings.go). The default is one worker, unlimited TPS (`0`), a 1,000-entry buffer and a 10-second statistics interval. Track flags default to false. `--dry-run` simulates deployment and requests; it cannot qualify live chain behavior.

### Paxload bounds

Profile `bounds` contains `minAverageTps`, `maxP99LatencyMs` and `minTotalTxs`. These fill zero-valued corresponding settings; explicit flags `--min-tps`, `--max-p99-latency` and `--min-total-txs` take precedence, including an explicit zero. Zero disables that bound. [`Bounds.Check`](../tools/loadtest/stats/bounds.go) compares overall average TPS, total transactions and each endpoint's p99 latency. Out-of-range millisecond durations fail decoding. Violations return `performance bounds violated` through the command's error path, which exits 1.

### Paxload report

[`stats/logger.go`](../tools/loadtest/stats/logger.go) emits final statistics and writes JSON when a report path is configured. A report write failure is an error, and bounds are checked against final statistics after shutdown. Preserve the profile, command, revision, report and exit code together when using the run as evidence.

### Paxload routes

The metrics server exposes `/metrics`; `--metricsListenAddr` defaults to all IPv4 interfaces on port 9090. It is independent of both the node JSON-RPC server and the older benchmark metrics listener.

### Paxload metrics

The exporter uses the `paxload` Prometheus namespace. Instrument definitions live in [`sender/metrics.go`](../tools/loadtest/sender/metrics.go) and [`stats/metrics.go`](../tools/loadtest/stats/metrics.go); report fields and counters must be interpreted according to those producers.

### Paxload formats

Profiles are JSON decoded into `LoadConfig` and `Settings`; scenario names resolve through the load generator. The newer camelCase format is not interchangeable with the embedded benchmark's snake_case format. See [profiles](../tools/loadtest/profiles) and the [client guide](../tools/loadtest/README.md).

### Paxload make

[`tools/loadtest/Makefile`](../tools/loadtest/Makefile) owns build, generation, tool installation, test and lint targets for this nested project. Generated bindings and scenario code belong to its own directories.

## Docker env

[`docker/common/env-files.sh`](../docker/common/env-files.sh) converts injected base64 environment values into files before executing the container command. `LAYERX_FILES_TABLE` defaults to `/etc/layerx/files.tsv`; `LAYERX_ROLE` selects rows.

### Docker formats

Each non-comment table row has four tab-separated columns: variable name, absolute guest path, octal mode and roles. Roles are comma-separated or `*`. For an applicable row the variable must be set; its value is base64-decoded to a temporary file in the destination directory, chmod is applied, and a rename installs it. Mode defaults to `0600` when empty. Invalid columns, names, paths, modes or base64 fail startup. An optional `LAYERX_FILES_OWNER_<VARIABLE>` overrides `LAYERX_FILES_OWNER`; ownership is applied when running as root. Listed variables are unset before `exec`, including rows for other roles. Without a table, the wrapper proceeds to the command; an empty command fails.

### Docker config

[`docker/docker-compose.yml`](../docker/docker-compose.yml), [local node TOML](../docker/localnode/config/app.toml), [kernel environment example](../docker/kernel/kernel.env.example) and [systemd unit](../docker/kernel/layerx-kernel.service) have different scopes. Read the specific entry point before selecting environment keys or volume paths.

### Docker CLI

Container startup is owned by each image's entry point. The shared env-files wrapper finishes with `exec "$@"`, so the command's exit status becomes the container result after materialization succeeds. Build arguments, runtime environment values and mounted configuration files must be supplied in the form that image consumes; see the [Docker directory](../docker).

## Paxctl CLI

[`tools/paxctl/main.go`](../tools/paxctl/main.go) still declares the CLI name `seictl`. Its verbs include `config`, `genesis`, `patch`, `await`, `serve`, `report` and the registered network/node/workflow/task subcommands. The directory name does not rename its flags or `SEI_*` variables. Main propagates `cli.ExitCoder` status; other errors exit 1.

### Paxctl env

`--home` reads `SEI_HOME`. [`serve.go`](../tools/paxctl/serve.go) defaults an empty home to `/sei`, and reads `SEI_SIDECAR_PORT` (default 7777), `SEI_CHAIN_ID`, `SEI_GENESIS_BUCKET`, `SEI_GENESIS_REGION`, `SEI_SNAPSHOT_BUCKET` and `SEI_SNAPSHOT_REGION`. The five chain/bucket/region values are required. Snapshot interval/timeout variables use Go duration strings. Keyring configuration uses `SEI_KEYRING_BACKEND`, `SEI_KEYRING_DIR` and `SEI_KEYRING_PASSPHRASE`; the passphrase variable is unset while constructing execution dependencies.

### Paxctl config

Task configuration and defaults live in [`sidecar/tasks`](../tools/paxctl/sidecar/tasks). [`outputter.go`](../tools/paxctl/outputter.go) writes transformed configuration to stdout unless `--output` or `--in-place-rewrite` is selected; these output options are mutually exclusive. File writes use the atomic patch writer. The task store is `sidecar.db` under the serving home. Startup installs execution dependencies and rehydrates stale tasks from that store; retain it across restarts rather than treating HTTP acceptance as completion.

### Paxctl routes

[`server.go`](../tools/paxctl/sidecar/server/server.go) registers the following HTTP API. These are sidecar routes, not public chain JSON-RPC methods.

| Request | Contract |
| --- | --- |
| `GET /v0/healthz`, `GET /v0/startupz`, `GET /v0/livez` | Process/engine probes; unhealthy responses use 503. |
| `GET /v0/metrics` | Prometheus scrape. |
| `GET /v0/status`, `GET /v0/node-id` | Engine status or `nodeId`. |
| `POST /v0/tasks` | One JSON object with optional `id`, required `type` and optional `params` object; returns 201 with `{"id":"..."}` after submission. |
| `GET /v0/tasks`, `GET /v0/tasks/{id}` | List results or read a task; missing tasks return 404. |
| `DELETE /v0/tasks/{id}` | Returns 204 on deletion, 404 when absent, or 503 if deletion fails. |

JSON errors use `{"error":"..."}`. Malformed task requests return 400. The [`auth.go`](../tools/paxctl/sidecar/server/auth.go) default is `SEI_SIDECAR_AUTHN_MODE=trusted-header`, binding literal loopback and requiring exactly one non-empty `X-Remote-User`. Missing, empty or duplicate values return 401. Only the three probes and metrics bypass this check. The proxy must authenticate callers and overwrite that header; the trust boundary includes every container in the pod. `serve` refuses unauthenticated mode because it registers signing and destructive handlers.

### Paxctl metrics

`seictl_sidecar_authn_rejections_total` labels rejected requests by reason. Task/engine and snapshot metrics are owned by [`sidecar/engine`](../tools/paxctl/sidecar/engine) and [`sidecar/tasks`](../tools/paxctl/sidecar/tasks); collect them through `/v0/metrics`.

### Paxctl formats

The CLI consumes native node configuration/genesis and sidecar task JSON. The task request preserves raw JSON values in `params` for task-specific decoding. Read the owning handler for its accepted fields and recovery rules before retrying a signing or state-changing task; a submission ID is not a guarantee of network commitment.

## Other tools and registered anchors

The following source map preserves the remaining registry anchors. CLI rows identify entry points; environment/configuration rows identify input declarations; format rows identify serialized artifacts. These links identify each tool’s owning implementation.

| Interface | Owning sources and contract boundary |
| --- | --- |
| <a id="bringup-cli"></a>Bringup CLI | [tools/bringup](../tools/bringup): CA, naming, edge and installation scripts; use each script's argument parser. |
| <a id="bringup-config"></a>Bringup config | [railway-names.env](../tools/bringup/railway-names.env): service-name inputs used by bringup tooling. Historical role names do not define current block production. |
| <a id="bringup-env"></a>Bringup env | [install-archive.sh](../tools/bringup/install-archive.sh), [ca.sh](../tools/bringup/ca.sh): installer and certificate inputs. |
| <a id="bringup-formats"></a>Bringup formats | [archive proxy location](../tools/bringup/archive-comet-location.conf), [edge example](../tools/bringup/edge-apps.example): proxy snippets and edge inventory. |
| <a id="deploy-cli"></a>Deploy CLI | [launchpad deploy](../deploy/launchpad/deploy.sh), [production bootstrap](../deploy/launchpad/production/bootstrap/README.md), [recovery](../deploy/launchpad/production/recovery/README.md): separate deployment and recovery operations. |
| <a id="deploy-config"></a>Deploy config | [launchpad deployment guide](../deploy/launchpad/README.md) and [Railway definitions](../deploy/launchpad/railway): deployment inputs, not live readiness. |
| <a id="deploy-env"></a>Deploy env | [service image](../deploy/launchpad/Dockerfile.svc), [web image](../deploy/launchpad/Dockerfile.web), [native image](../deploy/launchpad/Dockerfile.native): image-specific build/runtime variables. |
| <a id="deploy-formats"></a>Deploy formats | [bootstrap](../deploy/launchpad/production/bootstrap), [observability](../deploy/launchpad/production/observability/README.md), [recovery](../deploy/launchpad/production/recovery/README.md): Compose, JSON and SQL contracts. |
| <a id="deploy-metrics"></a>Deploy metrics | [prometheus-rules.json](../deploy/launchpad/production/observability/prometheus-rules.json): generated/consumed alert-rule format. |
| <a id="flyci-cli"></a>Fly CI CLI | [controller](../tools/flyci/controller/main.go), [runner entry point](../tools/flyci/runner/entrypoint.sh): controller and job runner lifecycles. |
| <a id="flyci-config"></a>Fly CI config | [controller Fly config](../tools/flyci/controller/fly.toml), [runner Fly config](../tools/flyci/runner/fly.toml): distinct application definitions. |
| <a id="flyci-env"></a>Fly CI env | [runner entry point](../tools/flyci/runner/entrypoint.sh): job environment inputs. |
| <a id="flyci-formats"></a>Fly CI formats | [tools/flyci](../tools/flyci): controller/runner TOML and qualification-control implementation. |
| <a id="ci-workflows"></a>CI workflows | [.github/workflows](../.github/workflows): workflow triggers, commands and permissions. A workflow definition alone is not passing evidence. |
| <a id="ci-config"></a>CI config | [Dependabot](../.github/dependabot.yml), [labeler](../.github/labeler.yml): dependency and label automation inputs. |
| <a id="ci-env"></a>CI env | [.github/workflows](../.github/workflows): per-job environment and secret references. |
| <a id="ci-formats"></a>CI formats | [.github](../.github): workflow YAML, ownership rules and issue templates. |
| <a id="hpx-cli"></a>hpx CLI | [hpx](../tools/hpx/hpx), [installer](../tools/hpx/get-hpx.sh): native node management and library downloads. |
| <a id="hpx-config"></a>hpx config | [hosting Fly config](../tools/hpx/hosting/fly.toml): library-mirror hosting. |
| <a id="hpx-env"></a>hpx env | [get-hpx.sh](../tools/hpx/get-hpx.sh): `HPX_MIRROR` selects the download mirror. |
| <a id="hpx-formats"></a>hpx formats | [hosting](../tools/hpx/hosting): HTTP server templates, registry assets and checksums. |
| <a id="hpx-routes"></a>hpx routes | [registry](../tools/hpx/registry/main.go): library registry HTTP handlers, independent of node JSON-RPC. |
| <a id="jev-cli"></a>jev CLI | [Python entry point](../tools/jev/__main__.py): command parsing and dispatch to the owning checks. |
| <a id="jev-formats"></a>jev formats | [decision response fixture](../tools/jev/tests/fixtures/decision_response.json): serialized decision-response example. |
| <a id="ops-cli"></a>Ops CLI | [backup](../tools/ops/backup/backup.sh), [restore](../tools/ops/backup/restore.sh), [keyring migration](../tools/ops/keyring-migrate.sh): state-changing maintenance operations. |
| <a id="ops-config"></a>Ops config | [backup.env.example](../tools/ops/backup/backup.env.example), [monitoring hosts](../tools/ops/monitoring/hosts.env.example): operator-owned inventory and archive inputs. |
| <a id="ops-env"></a>Ops env | [backup helpers](../tools/ops/backup/lib.sh): `BACKUP_ENV` defaults to `/etc/paxeer/backup.env`; `BACKUP_DIR` is required, retention defaults to 14 and must be positive. |
| <a id="ops-formats"></a>Ops formats | [backup directory](../tools/ops/backup), [monitoring directory](../tools/ops/monitoring): systemd units/timer, shell environment, Compose and Prometheus YAML. |
| <a id="ops-metrics"></a>Ops metrics | [monitoring rules](../tools/ops/monitoring/rules.yml): alert expressions and their source series. |
| <a id="pricesc-config"></a>Price sidecar config | [config.template.json](../tools/price-sidecar/config.template.json): price-sidecar settings template. |
| <a id="pricesc-formats"></a>Price sidecar formats | [service unit](../tools/price-sidecar/price-sidecar.service): process launch contract alongside the JSON template. |
| <a id="pxx-cli"></a>Paxeer X CLI | [tools/paxeer-x](../tools/paxeer-x): candidate, builders and per-task gate entry points. |
| <a id="pxx-config"></a>Paxeer X config | [route catalogue](../tools/paxeer-x/route-catalogue.json), [retired callers](../tools/paxeer-x/retired_callers.json): inventory data, not endpoint deployment declarations. |
| <a id="pxx-env"></a>Paxeer X env | [candidate tooling](../tools/paxeer-x/candidate.py), [gates](../tools/paxeer-x/gates): command-specific inputs. |
| <a id="pxx-formats"></a>Paxeer X formats | [governance templates](../tools/paxeer-x/gov/templates): proposal JSON; historical templates require checking against current module interfaces. |
| <a id="qual-cli"></a>Qualification CLI | [tools/qualification](../tools/qualification): bounded qualification drivers, each with its own command contract. |
| <a id="qual-env"></a>Qualification env | [Paxeer X qualification](../tools/qualification/paxeer-x): driver-specific artifact and environment inputs; no shared global readiness guarantee. |
| <a id="railway-cli"></a>Railway CLI | [fly-files-to-table.py](../tools/railway/fly-files-to-table.py): file-table conversion; runtime decoding follows [Docker env](#docker-env). |
| <a id="release-cli"></a>Release CLI | [tools/release](../tools/release): build, fleet check, rehearsal and binary-swap entry points. |
| <a id="release-config"></a>Release config | [toolchain](../tools/release/toolchain.env), [version](../tools/release/version.json), [environment examples](../tools/release/env): release inputs. |
| <a id="release-env"></a>Release env | [build-paxd.sh](../tools/release/build-paxd.sh), [swap.sh](../tools/release/swap.sh): build/swap environment contracts. |
| <a id="release-formats"></a>Release formats | [fleet-views.json](../tools/release/fleet-views.json), [version.json](../tools/release/version.json): fleet and version documents. |
| <a id="rollout-cli"></a>Rollout CLI | [rollout.py](../tools/rollout/rollout.py), [ledger.sh](../tools/rollout/ledger.sh): rollout execution and ledger handling. |
| <a id="rollout-env"></a>Rollout env | [box-apply.sh](../tools/rollout/box-apply.sh): box-specific application inputs. |
| <a id="rollout-formats"></a>Rollout formats | [replay.json](../tools/rollout/tests/replay.json): replay fixture, not a live rollout result. |
| <a id="scripts-cli"></a>Scripts CLI | [scripts](../scripts): each entry point owns its arguments; choose the task's declared command rather than an aggregate suite. |
| <a id="scripts-env"></a>Scripts env | [release gate](../scripts/ci/release-gate.sh), [artifact record](../scripts/ci/release-artifact-record.sh): task/release evidence inputs. |
| <a id="scripts-formats"></a>Scripts formats | [version template](../scripts/bump_version/setup.go.tmpl): version-generation input. |
| <a id="tchain-cli"></a>Chain CLI | [chain tool](../tools/chain/cmd.go), [upgrade replay](../tools/chain/upgrade-replay): chain inspection and replay entry points. |
| <a id="tchain-env"></a>Chain env | [upgrade replay wrapper](../tools/chain/upgrade-replay/run.sh): replay-specific paths and environment. |
| <a id="tci-cli"></a>CI report CLI | [beta-report.sh](../tools/ci/beta-report.sh): beta evidence reporting. |
| <a id="tdeploy-cli"></a>Deployment tools CLI | [tools/deploy](../tools/deploy): image publication and Railway variable/project operations. |
| <a id="tdeploy-env"></a>Deployment tools env | [railway-apply-variables.py](../tools/deploy/railway-apply-variables.py), [offchain-images.sh](../tools/deploy/offchain-images.sh): command-specific credentials and deployment inputs. |
| <a id="tdeploy-formats"></a>Deployment tools formats | [chart image Dockerfile](../tools/deploy/charting-library.Dockerfile), [variable replay fixture](../tools/deploy/tests/fixtures/railway-apply-variables-replay.json): build and test inputs. |
| <a id="tsh-cli"></a>Shell tool CLI | [arithmetic proof](../tools/lxp_arith_proof.sh), [replay matrix](../tools/lxp_qual_replay_matrix.sh): dedicated proof drivers. |
| <a id="tsh-env"></a>Shell tool env | [lxp_arith_proof.sh](../tools/lxp_arith_proof.sh): proof-tool environment contract. |
| <a id="txscan-cli"></a>Transaction scanner CLI | [scan.go](../tools/tx-scanner/cmd/scan.go): scanner argument parsing and dispatch. |
| <a id="schemas"></a>Schemas | [qualification drivers](../tools/qualification/paxeer-x), [kernel image](../docker/kernel/Dockerfile): schema producers/consumers remain owned by their implementations; a common anchor does not imply a common wire format. |
| <a id="makefile-env"></a>Make environment | [Makefile](../Makefile), [chain.mk](../chain.mk): build variables and included target fragments. |
| <a id="makefile-targets"></a>Make targets | [root Makefile](../Makefile), [chain targets](../chain.mk), [paxctl Makefile](../tools/paxctl/Makefile), [loadtest Makefile](../tools/loadtest/Makefile): target definitions at their respective roots. |

## Retained qualification manifest contracts

These contracts supplement [Qualification CLI](#qual-cli) and [Qualification env](#qual-env). The merged node executable is `paxd`. The root `make paxd-build` target builds the native kernel archive, the release Programs sandbox archive and the node through `chain.mk`. The CGO host links `build/liblayerx.a` and the Programs sandbox from `programs/target/release`; qualification must bind these exact archives together with the node executable.

Evidence belongs in a caller-owned private directory outside the checkout (directory mode `0700`, manifest mode `0600`). A successful build is an input to admission, not evidence that behavioral cases ran. Keep the revision, command, exit code, logs and manifest together. A changed source inventory, executable, archive, build log or required case inventory invalidates admission. Rebuild and publish a new manifest rather than editing a prior successful record. Missing inputs, malformed JSON, changed digests and failed or empty suites fail closed with a nonzero exit.

| Producer / consumer | Inputs and published contract | Lifecycle and refusals |
| --- | --- | --- |
| [`native-programs-link.py`](../tools/qualification/paxeer-x/native-programs-link.py) | `--build` builds the retained `paxd`, genesis builder and module registry in the checkout's actual link locations. `PAXEER_X_NATIVE_PROGRAMS_LINK_EVIDENCE` selects the private directory. `native-programs-artifacts.json` uses `layerx.native-programs-link.v1`: `task`, `revision`, exact `inputs`, `production` and `runtime` maps. Each artifact has an absolute `path` and `sha256`; `runtime` contains `kernel` and `programs`. | Running without `--build` admits the current revision and source inventory, checks every executable and archive digest, inspects ELF links and requires resolved dependencies and the retained kernel/Programs symbols in the node. `native-programs-result.json` records the revision and link results. Retired daemon entries or a missing `paxd` cannot qualify. |
| [`native-terminal-outcome.py`](../tools/qualification/paxeer-x/native-terminal-outcome.py) | `--node-rpc URL --transactions FILE --receipt-keys FILE --output FILE` reads all four role names (`serial-empty-0`, `scheduled-0`, `scheduled-1`, `terminal-0`) from a transaction-hash map. `receipt_keys` is an array of pinned `key_id` and `public_key` hex strings without `0x`, plus numeric `first_height` and `last_height`. The `paxeer-x.committed-terminal-outcomes.v1` output contains `chain_id: 125` and captures with the original signed receipt, raw transaction, committed block, pins and terminal digest. | JSON-RPC 2.0 POST uses the root endpoint and `Content-Type: application/json`. The retained SDK verifies signed receipt bytes against the transaction, key pins, return data and logs. Finality admission requires `rung: committed`, matching nonzero block identity and transaction membership. Native terminal bytes in the ABI return value must match the committed `ProgramCallOutcome` digest. The refusal role preserves empty return data/logs and its signed execution failure. Submitted/receipted observations, signature changes and block substitutions are refused; separate signed-batch headers and maintenance proofs are not capture inputs. This is admission from a trusted node; it is not an independent BFT light client. |
| [`native_send_purpose.py`](../tools/qualification/paxeer-x/native_send_purpose.py) | `--build` publishes `build.json` with schema `layerx-human-native-send-purpose-build.v1`, `source_files`, `source_revision`, `source_digest` and the `attestor`, `go_corpus`, `rust_corpus` artifacts. Production roots include `services/human`, `services/agent`, `services/wallet`, local Rust path dependencies and the root Go module's production trees. The sorted source digest includes path names and file digests, so additions, removals and uncommitted edits change it. | `--candidate-manifest` admits the immutable candidate and prebuilt artifacts selected by `PAXEER_X_NATIVE_SEND_BUILD_MANIFEST`; the original task deadline and candidate pin remain mandatory. It runs the existing native custody suites and checks required real-process result markers. Source and revision must remain unchanged through qualification. Cached artifacts from the former source roots cannot qualify. |
| [`program-interface-producer.py`](../tools/qualification/paxeer-x/program-interface-producer.py) / [`program-interface.py`](../tools/qualification/paxeer-x/program-interface.py) | Producer `--output DIR` publishes `manifest.json` using `paxeer-x.program-interface-artifacts.v1`. The single inventory is four roles: `native-library`, `sandbox-staticlib`, `protocol-tests`, `legacy-tests`, and three explicit build logs. `source` contains `revision`, `tree`, `dirty: false`; required protocol/legacy cases, profiles and features are shared declarations. Artifact records have absolute `path`, `sha256` and `bytes`. | Consumer `--manifest FILE --native-fixtures FILE` admits producer output unchanged. Native fixtures remain separately mandatory under `paxeer-x.program-interface-native-evidence.v1`, with the same source identity and every `NATIVE_FIXTURE_CASES` entry's `module.wasm`, `interface.bin`, `evidence.kvx`. Missing native evidence never falls back to an empty suite. The producer refuses an incompatible required-case inventory before publication. |
| [`router-authority-schema.py`](../tools/qualification/paxeer-x/router-authority-schema.py) | This checks two surfaces: the maintained hosted gateway OpenRPC document and node `eth_`/`pax_` dispatch at `/`. `PAXEER_X_SCHEMA_BUILD_MANIFEST` supplies the revision/tree-bound successful gateway test artifact; `PAXEER_X_NODE_RPC_URL` supplies the node root URL. | The existing exact hosted schema test must run once. Hosted `px_` methods and the receipt relay are checked against their actual document. Node probes check `rpc_modules`, chain 125, `eth_call`, raw-transaction parameter rejection and the receipt lookup contract. The raw transaction probe sends empty invalid bytes and never broadcasts a valid transaction. A method-not-found response or a missing namespace fails qualification. |
| [`runtime-boundary-witness.py`](../tools/qualification/paxeer-x/runtime-boundary-witness.py) | `LAYERX_BOUNDARY_EVIDENCE` selects the directory containing `source.json`, schema `paxeer-x.runtime-boundary-artifacts.v1`. The build owner calls `publish_manifest` with `source: {revision, tree}`, exact `sources`, the executable, build log and `build_exit: 0`. The manifest binds both artifact digests and the complete `required_cases` list. Inventory includes the Programs workspace, SDK, patched vendor sources, toolchain pin, Cargo configuration and this consumer. | Admission compares the entire current inventory, not merely listed entries, and rejects stale revisions and changed binaries/logs. The executable is selected from the manifest, never parsed from build output. All eight declared boundary cases must report individual passes with one consistent successful summary: no failed, ignored, measured or filtered cases. Exit zero without those results cannot qualify. |
| [`storage-recovery-producer.py`](../tools/qualification/paxeer-x/storage-recovery-producer.py) / [`storage-recovery.py`](../tools/qualification/paxeer-x/storage-recovery.py) | Producer `--output DIR` retains schema `paxeer-x.storage-recovery-artifacts.v1` and publishes the actual make command, source revision/tree, scoped source digests, build log digest and three suite artifacts. The retained inventory is `lxp_test_recovery`, `lxp_test_log_durability`, `lxp_test_rebuild`; it binds retained log, snapshot and projection implementations and the chain host/context and restart tests. | The real consumer derives its inventory and suite commands from the producer. It can reopen the manifest and recover identical admitted evidence. Native `lxp_test_recovery` owns canonical/activity log close-and-reopen, checkpoint replay and snapshot crash/retry cases; rebuild owns projection recovery. The retired feed store and daemon batch WAL cannot contribute release evidence. |
