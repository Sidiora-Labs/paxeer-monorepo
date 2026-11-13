# Paxeer paxctl component guide

This directory ships the `seictl` executable from the pinned Paxeer monorepo.
The inherited module path, `SEI_*` settings and `sei.io` API names remain wire
compatibility names. Read [README.md](README.md) for installation and chain
configuration; [go.mod](go.mod) declares its module dependencies.

## Architecture and routing

`main.go` registers local `config`, `genesis`, `patch`, `await`, `serve` and
`report` commands plus four operational trees:

- `seinetwork/`: `network apply/get/list/delete/watch`, rendering `SeiNetwork`.
- `seinode/`: `node apply/get/list/delete/watch`, rendering `SeiNode` followers.
- `workflow/`: `workflow apply/get/list/delete/watch` and `workflow state-sync`,
  rendering `SeiNodeTaskWorkflow`. The controller adopts and executes the recipe;
  the CLI renders/applies/watches it. It does not execute the controller plan.
- `task/`: `task submit/get/list/delete` and `task snapshot-upload`, calling the
  addressed pod's sidecar through its authenticated TLS proxy.

`internal/seiapi` binds the current `sei.io/v1alpha1` GVK/GVRs and performs
server-side apply. `internal/cliutil` resolves kubeconfig/namespace, prints
Kubernetes objects and stamps `seictl.sei.io/version`. `--dry-run` is an API-server
request, not offline rendering. Presets and flags feed the production renderers;
renderers stamp the build version in `seictl.sei.io/version`.

`serve.go` opens `<home>/sidecar.db` using `server.NewTaskStore`, installs it as
both `engine.ResultStore` and `ExecutionConfig.Checkpointer`, registers real
handlers from `sidecar/tasks`, then calls `RehydrateStaleTasks` before listening.
HTTP POST `/v0/tasks` decodes exact JSON parameters, admits them through the
durable TaskStore queue, and dispatches to the engine. HTTP admission limits
queued/active tasks to 32 and retained identities to 256 and runs one node task
at a time; overload/unavailability is explicit (429/503).

The engine executes one-shot tasks. There is no cron scheduler or automatic
periodic retry of completed/failed work. Controller/CronJob producers own any
schedule, fresh operation identity and polling. The store retains parameters,
results, generations, cancellations and transaction checkpoints. Startup
re-executes stale `running` work and drains persisted HTTP queue entries after
runtime dependencies are installed; handlers must support idempotent recovery.
Completed results survive reopening and are not dispatched again. Readiness
hold/release is durable; stale mark-not-ready recovery runs before mark-ready.
DELETE cancels active work and generation checks prevent obsolete completions
from resurrecting deleted results. Keep the database with the correct node home
and chain; deleting it discards operation/recovery evidence.

`sidecar/api/openapi.yaml` and generated `sidecar/client/sidecar.gen.go` define
the typed client surface. Hand-written `client.go`/`tasks.go` wrap it. Runtime
admission can also return 413/429/503 and `queued` task status.
`config-reload` reports restart-required.

## Authentication and listener boundary

`server.AuthnMode()` defaults to `trusted-header`. `BindAddress` returns
`127.0.0.1:7777` for that mode; `--port` / `SEI_SIDECAR_PORT` changes the port.
Unknown modes fail startup. Explicit `unauthenticated` additionally requires
`SEI_SIDECAR_ISOLATED_DEVELOPMENT=true`, and its low-level listener binds
`:7777` on all interfaces. That development server rejects task writes.
The production `serve` command rejects unauthenticated mode entirely because
its handler set includes signing and destructive operations.

External task callers use TLS :8443 on the in-pod kube-rbac-proxy. It validates
Kubernetes TokenReview and SubjectAccessReview, strips/overwrites caller
identity headers, then forwards to the loopback sidecar with exactly one
non-empty `X-Remote-User`. The sidecar checks that header, not bearer tokens.
`task` commands use kubeconfig credentials but require `--proxy-ca-file` for the
proxy's CA and verify its pod DNS identity; redirects are refused. API-server
CA/insecure settings must not weaken this proxy boundary.

Only `/v0/healthz`, `/v0/startupz`, `/v0/livez` and `/v0/metrics` bypass the
identity header. Keep proxy allow-paths aligned with `server.BypassPaths()`.
The sidecar and proxy share the pod network with trusted colocated containers.
Configure `hostNetwork`, `hostPID` and `hostIPC` as false and give the sidecar
the paxd process visibility and home required by its task handlers.

## Build and release

Use Go 1.26.0 and the complete pinned monorepo, because local replacements in
`go.mod` resolve sibling modules. `make build` builds `build/seictl` with the
validated version and verifies all three real rendered annotations locally.
GoReleaser validates the tag against the same version file and stamps the same
symbol. Docker requires monorepo-root build context and verifies each target
executable before copying it into the distroless image. The native artifact
workflow verifies all four supported OS/CPU archives. A bare build retains
`dev`; do not publish it as a qualified release.

`make test` runs the Go package tests. `make generate` runs the module's Go
generators, including the API client generator configured by
`sidecar/api/generate.go` and `sidecar/api/codegen.yaml`.
