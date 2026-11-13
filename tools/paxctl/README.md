# Paxeer node operations (`seictl`)

This directory holds the Go module for `seictl`, the Paxeer node operations
executable. It manages Kubernetes network, node and workflow resources, runs
sidecar tasks, patches local node configuration and genesis files, and reads
shadow comparison reports. The chain daemon is `paxd`; its kernel executes inside
the node. The Go import path, `SEI_*` environment variables, `sei.io/v1alpha1`
CRDs and annotation keys retain their spelling. Select the intended Paxeer chain
ID, node home, pinned `paxd` image and Kubernetes context explicitly.

## Layout

- `.agent/` — command smoke runbook.
- `.github/` — release, container and repository check workflows.
- `internal/` — CLI output, Kubernetes clients, patching, release verification, API helpers and integration test support.
- `seinetwork/` — `SeiNetwork` commands and the embedded `genesis-chain` preset.
- `seinode/` — `SeiNode` commands and the embedded `rpc` preset.
- `sidecar/` — task API, client, persistent execution engine, task handlers, RPC, S3 and shadow comparison packages.
- `task/` — commands for submitting, reading, deleting and listing sidecar tasks and uploading snapshots.
- `workflow/` — `SeiNodeTaskWorkflow` commands, embedded presets and state-sync guidance.
- `main.go` — executable entry point, command registration and node home flag.
- `await.go` — validator condition polling.
- `config.go` — node TOML configuration patch command.
- `genesis.go` — genesis JSON patch command.
- `patch.go` — generic TOML and JSON merge patch command.
- `outputter.go` — stdout, file and in-place patch output.
- `report.go` — divergence report retrieval and rendering.
- `report_list.go` — shadow comparison object listing.
- `serve.go` — sidecar startup, task registration and signing configuration.
- `hardcut_test.go` — registered command and Paxeer chain contract tests.
- `report_test.go` — divergence report tests.
- `report_list_test.go` — report listing tests.
- `serve_test.go` — sidecar execution configuration tests.
- `Makefile` — build, installation, tests, formatting and code generation targets.
- `go.mod` — module dependencies and local replacements.
- `go.sum` — dependency checksums.
- `version.json` — release version manifest.
- `.goreleaser.yaml` — native release archives, checksums and version stamping.
- `Dockerfile` — executable build, provenance verification and distroless image.
- `.gitignore` — local build output exclusions.
- `VENDOR.kvx` — component source reference.
- `LICENSE.md` — component license.

## Build and test

Use Go 1.26.0, Git and Make with the complete monorepo checkout. The local
replacement in [go.mod](go.mod) requires [core/deps/geth](../../core/deps/geth).
Run these targets from `tools/paxctl`; they are defined in [Makefile](Makefile):

```sh
make build
make test
```

`make build` produces `build/seictl`, reads and validates `version.json`, stamps
`github.com/sei-protocol/seictl/internal/cliutil.Version`, and verifies network,
node and workflow provenance annotations using the built executable.
`make test` runs `go test ./...`. Kubernetes integration tests use a real API
server and etcd; set `KUBEBUILDER_ASSETS` to their installed binary directory.

Other Makefile targets are `make install` for installation into Go's bin
directory, `make run` for source execution, `make lint` for formatting checks,
`make fmt` for Go formatting and `make generate` for generated API code.

The [Dockerfile](Dockerfile) uses the monorepo root as its build context and
verifies the linked executable before copying it into the distroless runtime.
The container workflow targets Linux amd64 and arm64.

Native releases configured in [.goreleaser.yaml](.goreleaser.yaml) support:

| Native OS | CPU | Archive | Executable |
| --- | --- | --- | --- |
| Linux | amd64 | `seictl_Linux_x86_64.tar.gz` | `seictl` |
| Linux | arm64 | `seictl_Linux_arm64.tar.gz` | `seictl` |
| macOS | amd64 | `seictl_Darwin_x86_64.tar.gz` | `seictl` |
| macOS | arm64 | `seictl_Darwin_arm64.tar.gz` | `seictl` |

Use a pinned Paxeer release with its matching source and verify the archive's
SHA-256 entry in `checksums.txt` before extraction. Pin containers by image
digest. The release tag must match the validated version manifest, including
its `v` prefix. The [native artifact workflow](.github/workflows/release-provenance.yml)
checks archives on matching OS and CPU runners; its verifier is
[internal/release](internal/release). Renderers write the build version to the
`seictl.sei.io/version` annotation. A bare `go build` leaves that version at
`dev`; the CLI has no registered `--version` flag.

## Configuration

The sources read these environment variables; supply deployment-specific
configuration through the selected node and cluster environment:

| Purpose | Environment variables |
| --- | --- |
| Node home | `SEI_HOME` |
| Kubernetes configuration | `KUBECONFIG` |
| Sidecar listener and authentication | `SEI_SIDECAR_PORT`, `SEI_SIDECAR_AUTHN_MODE`, `SEI_SIDECAR_ISOLATED_DEVELOPMENT` |
| Chain and genesis storage | `SEI_CHAIN_ID`, `SEI_GENESIS_BUCKET`, `SEI_GENESIS_REGION` |
| Snapshot storage and scheduling | `SEI_SNAPSHOT_BUCKET`, `SEI_SNAPSHOT_REGION`, `SEI_SNAPSHOT_UPLOAD_INTERVAL`, `SEI_SNAPSHOT_UPLOAD_TIMEOUT` |
| Signing keyring | `SEI_KEYRING_BACKEND`, `SEI_KEYRING_DIR`, `SEI_KEYRING_PASSPHRASE` |
| Result export identity | `HOSTNAME` |
| Report storage | `SEI_RESULT_EXPORT_BUCKET`, `SEI_RESULT_EXPORT_PREFIX`, `SEI_RESULT_EXPORT_REGION` |
| Kubernetes integration tests | `KUBEBUILDER_ASSETS` |

`serve` requires the chain, genesis bucket and region, and snapshot bucket and
region variables. It binds to loopback behind the pod's TLS proxy and requires
trusted-header authentication because it registers signing and destructive
tasks. The port flag overrides the listener environment setting. Signing tasks
use the configured keyring; a file backend requires the passphrase variable.
S3 clients use the AWS SDK's default configuration and credential provider chain.

Kubernetes commands load the file selected by `--kubeconfig`, `KUBECONFIG`, the
standard `.kube/config` in the user's home, or in-cluster configuration. `-n`
selects the namespace. Task commands also read the PEM CA bundle selected by
`--proxy-ca-file`; proxy trust is configured independently of API-server trust.

Local patch commands read `config/app.toml`, `config/client.toml`,
`config/config.toml` or `config/genesis.json` under the selected node home.
`--home` overrides `SEI_HOME`; local config and genesis patching use `~/.sei`
when neither is supplied, while `serve` uses `/sei`. Pass the intended Paxeer
home explicitly. Generic patching reads the file selected by `--target`.
Patch input comes from a supplied TOML or JSON file, or stdin. Output goes to
stdout unless `-o` selects a file or `-i` requests an in-place rewrite. Genesis
patching changes JSON without validating the chain's genesis semantics.

The sidecar embeds [defaults/config.toml](sidecar/tasks/defaults/config.toml)
for initializing a missing node configuration. It persists task results in
`sidecar.db` under the node home. Build and release tools read `version.json`.

### Commands and prerequisites

| Surface | Commands | Requirements |
| --- | --- | --- |
| Genesis network | `network apply/get/list/delete/watch` | `SeiNetwork`, `genesis-chain` preset, explicit chain ID and `paxd` image |
| Follower/RPC node | `node apply/get/list/delete/watch` | `SeiNode`, `rpc` preset, explicit chain ID, image and peers |
| Controller recipe | `workflow apply/get/list/delete/watch`, `workflow state-sync` | `SeiNodeTaskWorkflow`, compatible controller and target node |
| Sidecar tasks | `task submit/get/list/delete`, `task snapshot-upload` | Pod DNS connectivity, Kubernetes credentials, proxy RBAC and `--proxy-ca-file` |
| Local editing | `patch`, `config patch`, `genesis patch` | Explicit node home or target file |
| Other tools | `serve`, `await`, `report divergence/list` | Process, RPC and storage configuration for the selected deployment |

Kubernetes credentials need RBAC for the selected resources and namespace.
`apply --dry-run` performs server-side validation and requires cluster access.
Network readiness is `Ready`; node readiness is `Running`, with `caught-up` as
a watch sentinel. Workflow `Complete` means its controller plan finished;
inspect `.status.plan` for task results. State sync resets node data and is
intended for a disposable follower; see the recovery guidance below.

Task requests traverse the pod's TLS proxy, which performs TokenReview and
SubjectAccessReview and overwrites `X-Remote-User` before forwarding to the
sidecar. Run task commands where pod DNS resolves and the proxy is reachable.
The `restart-seid` and `stop-seid` task identifiers name process operations
whose daemon target is `paxd`. Snapshot, genesis, signing and state-sync
operations require configuration for the selected Paxeer chain and controller.
The registered command set is defined in [main.go](main.go).

## Documentation

- [Release readiness matrix](../../docs/release/7.0.0-readiness-matrix.md) — component build and test entry points.
- [Run a Node](../../docs/site/paxeer-docs/src/app/run-node/page.tsx) — documentation site guide to `paxd` setup and synchronization.
- [State-sync workflow](workflow/README.md) — controller recipes, node catch-up and recovery.
- [Sidecar authentication](sidecar/docs/authn.md) — proxy and listener contract.
- [Sidecar keyring](sidecar/docs/keyring.md) — signing backend configuration.
- [Command smoke procedure](.agent/runbooks/seictl-smoke.md) — chain identity and command examples.
- [Sidecar OpenAPI contract](sidecar/api/openapi.yaml) — task API requests and responses.
