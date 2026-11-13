# FlyCI

This directory holds the Go controller and runner entrypoint for ephemeral GitHub Actions runners on Fly Machines. It supports the Paxeer network's build and test infrastructure: the controller selects queued jobs by runner labels, provisions machines with just-in-time runner configuration, and reconciles their lifecycle against GitHub job assignments. The controller also supports persistent qualification requests, encrypted results, signed completion records, and an optional authenticated readiness endpoint.

## Layout

- [controller/](controller/) — Go executable, GitHub and Fly API clients, job reconciliation, qualification storage and evidence handling, readiness service, tests, and the controller's `fly.toml` deployment configuration.
- [runner/](runner/) — `entrypoint.sh` starts the Actions runner as an unprivileged user, enforces the idle timeout, and provides `--selfcheck`; `fly.toml` selects the runner image build.

## Build and test

The controller belongs to the root [Go module](../../go.mod). Run commands from the repository root.

The [qualification workflow](../../.github/workflows/paxeer-x-qualification.yml) builds the controller as an evidence helper with the following command; GitHub Actions supplies `RUNNER_TEMP`:

```sh
go build -o "$RUNNER_TEMP/flyci-evidence" ./tools/flyci/controller
```

The container builds are defined in the [controller Dockerfile](../../docker/flyci-controller/Dockerfile) and [runner Dockerfile](../../docker/flyci-runner/Dockerfile). The controller image build requires the `SOURCE_REVISION` build argument and embeds it for readiness checks. The runner image installs the Actions runner and its build tools; its entrypoint's `--selfcheck` checks the installed tools and writable tool cache inside that image.

The [root Makefile](../../Makefile) provides the Go test target, which delegates to [chain.mk](../../chain.mk) and includes the controller package:

```sh
make paxeer-test
```

For the controller's focused job-assignment and cleanup tests, the tree provides this [test script](../paxeer-x/gates/25.2.sh):

```sh
tools/paxeer-x/gates/25.2.sh
```

## Configuration

The controller reads these environment variables in [config.go](controller/config.go):

| Purpose | Names |
| --- | --- |
| Required repository, authentication, and image settings | `GITHUB_REPOSITORY`, `GITHUB_TOKEN`, `FLY_API_TOKEN`, `RUNNER_IMAGE` |
| Runner placement and labels | `FLY_RUNNER_APP`, `FLY_REGION`, `RUNNER_LABELS` |
| Machine resources and capacity | `MACHINE_CPU_KIND`, `MACHINE_CPUS`, `MACHINE_MEMORY_MB`, `MAX_MACHINES` |
| Polling and lifecycle timing | `POLL_INTERVAL`, `RUNNER_IDLE_TIMEOUT`, `ORPHAN_TTL` |
| Provider API overrides | `GITHUB_API_URL`, `FLY_API_URL` |
| Persistent qualification | `QUALIFICATION_STATE_DIR`, `QUALIFICATION_CONTRACT_SHA256` |
| Readiness configuration file | `CI_READINESS_CONFIG_FILE` |

[controller/fly.toml](controller/fly.toml) declares the controller process, deployment environment, machine resources, and qualification volume mount. [runner/fly.toml](runner/fly.toml) declares the runner image build.

When qualification storage is configured, the controller reads `artifact-key.der` and `signing-key` from `QUALIFICATION_STATE_DIR` and requires protected storage on a mounted volume. `QUALIFICATION_CONTRACT_SHA256` pins the accepted qualification contract.

The optional JSON file selected by `CI_READINESS_CONFIG_FILE` is read by [readiness.go](controller/readiness.go). Its fields are `listen`, `certificate_file`, `key_file`, `client_ca_file`, `client_spki_sha256`, `chain_id`, `network_id`, `wire_version`, `repository`, `runner_app`, `runner_image`, `source_revision`, `qualification_contract`, and `max_age_seconds`. The certificate, key, and client CA fields name protected files; client public-key pins authenticate readiness callers.

The [runner entrypoint](runner/entrypoint.sh) reads `RUNNER_JITCONFIG` and `RUNNER_IDLE_TIMEOUT` when starting a job, and `RUNNER_TOOL_CACHE` during `--selfcheck`. The controller supplies the just-in-time configuration and idle timeout when creating each machine.

## Documentation

- [Workflow secrets, variables and runners](../../docs/ci/secrets-and-runners.md) describes Actions configuration and runner selection.
