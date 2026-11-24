# Project definitions

Each folder holds `project.kvx` with a `[meta]` section naming `project`,
`environment` and the default repository, and one KVX file per service. The
platform, wallet, interop, explorer and sites projects use
`paxeer-x-<folder>`, `production` and `sidiora-labs/paxeer-monorepo`.
The project tool reads every `.kvx` file directly in the selected folder.

Definitions use `[service.<name>]`, `[vars.<name>]` and `[inputs.<name>]` sections.
Strings are double quoted, numbers are integers, and lists fit on one row:

```toml
[meta]
kind = "railway-project"
project = "paxeer-x-platform"
environment = "production"
repo = "sidiora-labs/paxeer-monorepo"

[service.api]
kind = "repo"
dockerfile = "docker/platform-gateway/Dockerfile"
root = "/"
watch = ["/docker/platform-gateway/**", "/docker/common/**", "/services/platform/**"]
health = "/healthz"
restart = "ALWAYS"
replicas = 1
public = 1
order = 5
needs = ["identity"]

[vars.api]
PORT = "8080"
AUTH_SECRET = "env:PLATFORM_API_AUTH_SECRET"

[inputs.api]
# Authentication secret: an operator-supplied string.
PLATFORM_API_AUTH_SECRET = "operator"
```

A `repo` row may set the repository as `owner/repository`; otherwise it uses
the `repo` value from `[meta]`, and it also sets the container file, root folder
and watch patterns. The branch defaults to `main`. An `image` row uses
`image` instead of those four fields: either a complete image reference, or a
short image name resolved under `ghcr.io/sidiora-labs` through `--digests`.
The optional JSON digest map associates image names with `sha256:<64hex>`
digests. A `database` row needs `image` and `order`; its image is used directly.
Image and repo rows require `health`, `restart` (`ALWAYS`, `ON_FAILURE` or
`NEVER`) and `replicas`. An optional `volume` gives an absolute mount path;
`public = 1` creates a public HTTP address. `railway_service` optionally gives
a remote service name different from the section name.

`order` is a positive deployment wave. Each wave waits for successful deployment
health before the next starts. `needs` lists services in the folder or
[rollout.kvx](../rollout.kvx); its order cannot be below theirs. A private store
can be another database row in the same service file. Existing shared services
are referenced with Railway expressions such as `${{store.DATABASE_URL}}`.
A row with `target = "other"` is listed in the plan and left alone during apply;
a comment above that row explains its non-HTTP port, privilege or device need.

Variables are literal strings, Railway references, or `env:<NAME>` inputs read
from the process environment. Input names join the selected folder name,
service and variable names in uppercase with underscores replacing every other
character, as in the example; they stay the same when `--project` or
`--environment` replaces the destination.
Every input has a matching row under `inputs.<name>`, with a comment describing
its meaning and format. The four input words are `generated` for a random value
shared only within the project, `provider` for an outside provider key, `chain`
for a chain account key or address, and `operator` for other supplied values.
The tool reads these inputs; operators supply or generate their values.

The dashboard API sets `PORT` to `9445`, matching `LAYERX_DASHBOARD_LISTEN`
for health requests. The dashboard web requires two operator inputs without
defaults: `PLATFORM_DASHBOARD_WEB_DASHBOARD_API_ORIGIN` supplies the API origin
compiled into the production rewrite, and
`PLATFORM_DASHBOARD_WEB_DASHBOARD_GATEWAY_ORIGIN` supplies the hosted gateway
origin the web calls at runtime for sign-in. Both values are absolute HTTP(S)
origins; the API origin remains the web container's build argument.

The dashboard API and both webhook services take these `operator` inputs for
the network label used by the project gateway. Each value is 1–64 printable
ASCII bytes without spaces (bytes `0x21`–`0x7e`):

| Service | Input | Runtime setting |
| --- | --- | --- |
| `dashboard` | `PLATFORM_DASHBOARD_LAYERX_DASHBOARD_NETWORK_ID` | `LAYERX_DASHBOARD_NETWORK_ID` |
| `webhooks-public` | `PLATFORM_WEBHOOKS_PUBLIC_LAYERX_WEBHOOKS_NETWORK_ID` | `LAYERX_WEBHOOKS_NETWORK_ID` |
| `webhooks-ingress` | `PLATFORM_WEBHOOKS_INGRESS_LAYERX_WEBHOOKS_NETWORK_ID` | `LAYERX_WEBHOOKS_NETWORK_ID` |

Their `/healthz` responses always include the package `version` and include
`network_id` when the runtime setting is present. The gateway bindings use
`/healthz` for health and identity, `/ready` equal to `true`, `/network_id`
equal to the gateway network label, and `/version` equal to the deployed
package version. Webhook `/livez` responses contain only `live` and `role`.

The platform [gateway definition](platform/gateway.kvx) sets `LAYERX_ROLE=identity`
and `LAYERX_FILES_TABLE=/etc/layerx/native-files.tsv` for developer registration
and sessions. `LAYERX_GATEWAY_IDENTITY_URL` reads the operator input
`PLATFORM_GATEWAY_LAYERX_GATEWAY_IDENTITY_URL`, an absolute HTTPS address of
the identity service. `LAYERX_GATEWAY_TENANT` reads the operator input
`PLATFORM_GATEWAY_LAYERX_GATEWAY_TENANT`, a tenant of 1–95 lowercase letters,
digits, dots, underscores or hyphens. `LAYERX_GATEWAY_INDEXER_URL` is the
private HTTPS origin of the `indexer` service on its listener port.
`LAYERX_GATEWAY_CONSENSUS_RPC_URL` reads the operator input
`PLATFORM_GATEWAY_LAYERX_GATEWAY_CONSENSUS_RPC_URL`, an absolute HTTPS address
with a DNS host that may hold a path; calls go to exactly that path.

The native table adds these six `identity` rows, each with mode `0600`.
Each file input reads `PLATFORM_GATEWAY_` followed by its name from the launch
environment; for example, `PLATFORM_GATEWAY_ENDPOINT_IDENTITY_TOKEN` supplies
`ENDPOINT_IDENTITY_TOKEN`.

| File input | Destination | Gateway file variable | Input category and format |
| --- | --- | --- | --- |
| `ENDPOINT_ROUTE_BINDINGS` | `/run/secrets/route-bindings.json` | `LAYERX_GATEWAY_ROUTE_BINDINGS_FILE` | `operator`: product route bindings JSON document encoded as padded base64 |
| `ENDPOINT_IDENTITY_TOKEN` | `/run/secrets/identity-token` | `LAYERX_GATEWAY_IDENTITY_TOKEN_FILE` | `operator`: base64 of token text issued by the identity service |
| `ENDPOINT_IDENTITY_PROVISIONING_TOKEN` | `/run/secrets/identity-provisioning-token` | `LAYERX_GATEWAY_IDENTITY_PROVISIONING_TOKEN_FILE` | `operator`: base64 of provisioning token text issued by the identity service |
| `ENDPOINT_KEY_PROVISIONING_KEY` | `/run/secrets/key-provisioning-key` | `LAYERX_GATEWAY_KEY_PROVISIONING_KEY_FILE` | `generated`: base64 of 64 hexadecimal characters, generated once and equal on every replica |
| `ENDPOINT_CLIENT_P12` | `/run/secrets/client.p12` | `LAYERX_GATEWAY_CLIENT_IDENTITY_PKCS12` | `generated`: base64 PKCS#12 client bundle written by the launcher |
| `ENDPOINT_CLIENT_PASSWORD` | `/run/secrets/client-password` | `LAYERX_GATEWAY_CLIENT_IDENTITY_PASSWORD_FILE` | `generated`: base64 client bundle password written by the launcher |

`PLATFORM_GATEWAY_ENDPOINT_ROUTE_BINDINGS` supplies the complete bindings
document. Its service predicates, HTTPS trust, supported transports and empty
document behavior are described in the
[native gateway inputs](../../docs/wiki/HostedGateway.md#native-deployment-inputs).

The identity role selects ten native rows: these six plus the outbound CA,
Redis username, Redis password and receipt keys marked `*`. A native start
without the role and identity settings selects only those four chain files,
serves the chain methods, and answers `px_register` and `px_openSession` with
`registration_not_configured`.

From the repository root, set `RAILWAY_API_TOKEN`, `RAILWAY_WORKSPACE_ID` and
the named inputs, then start a plan or apply:

```sh
python3 tools/deploy/platform-project.py deploy/projects/platform --plan
python3 tools/deploy/platform-project.py deploy/projects/platform --apply --branch main
```

`--project` and `--environment` replace the destination names; input names
continue to follow the project's definition. `--digests <file.json>` pins image
rows. Only image rows carrying `credentials = 1` require and send
`GHCR_USERNAME` and `GHCR_TOKEN`; public images omit credentials. Plans show
names and changes without input values. Missing inputs yield plan exit code 3
and prevent apply writes. `ROLLOUT_TIMEOUT` and `ROLLOUT_POLL` set deployment
wait and polling seconds, defaulting to 1800 and 10. `--replay <file.json>`
selects an ordered recording of operation names, request variables and responses
instead of the HTTP transport.
