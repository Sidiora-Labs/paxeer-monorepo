# Sidecar HTTP API authentication

The sidecar exposes its task API over plain HTTP behind the pod's TLS proxy.
Authentication defaults to `trusted-header`; the runtime trust
boundary is the pod, not the loopback interface.

## Environment contract

| Env | Values | Default | Behavior |
|---|---|---|---|
| `SEI_SIDECAR_AUTHN_MODE` | `trusted-header` \| `unauthenticated` | `trusted-header` | Unset selects `trusted-header`: bind loopback and require `X-Remote-User` on non-probe paths. |
| `SEI_SIDECAR_ISOLATED_DEVELOPMENT` | `true` | unset | Required to select the low-level `unauthenticated` development mode. |

The `serve` command requires `trusted-header` because it registers signing and
destructive handlers. The low-level development server can bind all interfaces
in `unauthenticated` mode only with the explicit development setting; it rejects
task writes. See [the authentication implementation](../server/auth.go) and
[the production command](../../serve.go).

Parsing trims surrounding whitespace and lowercases the mode. An empty value
selects `trusted-header`; any other value besides `unauthenticated` and
`trusted-header` is a startup error.

## `trusted-header` mode

The sidecar is paired with an in-pod `kube-rbac-proxy` container on
TLS `:8443`. The proxy performs TokenReview + a single coarse
`create seinodetasks.sei.io` SubjectAccessReview against the K8s API,
then forwards passed requests to `127.0.0.1:7777` with `X-Remote-User`
naming the authenticated identity.

The sidecar requires exactly one non-empty `X-Remote-User` value per
request. The proxy overwrites that header with the authenticated identity.

### Bypass paths

Four paths skip the `X-Remote-User` check because their callers do
not carry auth headers:

| Path | Caller |
|---|---|
| `/v0/healthz` | kubelet readiness probe |
| `/v0/startupz` | kubelet startup probe |
| `/v0/livez` | kubelet liveness probe |
| `/v0/metrics` | Prometheus scrape |

The `kube-rbac-proxy` `--allow-paths` flag must include all four so
probes and scrapes traverse the proxy without TokenReview. The
authoritative list is logged at sidecar startup in trusted-header
mode and is exposed programmatically via `server.BypassPaths()`.

Rejected requests are counted in the
`seictl_sidecar_authn_rejections_total{reason}` metric — labels
`missing_header`, `duplicate_header`, `empty_header` distinguish the
common misconfigurations.

### Pod-level isolation requirements

The trust boundary is the pod. The sidecar, proxy and any colocated containers
share its network namespace and belong to the same trusted deployment.
Configure `hostNetwork`, `hostPID` and `hostIPC` as false. Process visibility
within the pod must support the sidecar's node-management tasks.

## Controller-side contract

The contract this sidecar exposes to `sei-k8s-controller`:

- Env var: `SEI_SIDECAR_AUTHN_MODE=trusted-header`
- Internal port: `7777`, bound to `127.0.0.1`
- Probe paths: `/v0/healthz` (readiness), `/v0/startupz` (startup), `/v0/livez` (liveness)
- Prometheus path: `/v0/metrics`
- `kube-rbac-proxy --allow-paths`: `/v0/healthz,/v0/startupz,/v0/livez,/v0/metrics`
- Forwarded header: `X-Remote-User` — proxy MUST overwrite, not
  append
- Pod isolation: `hostNetwork`/`hostPID`/`hostIPC` all false

Kubelet probes hit the pod IP, not loopback. With the sidecar bound
loopback-only, probes must be routed through the proxy port.
