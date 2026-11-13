# Hyperfide native source control

Hyperfide source control serves immutable source history through an online API. Source objects use S3 storage; PostgreSQL stores revision, operation, workspace and acceptance metadata. Clients use scoped APIs and bounded caches, with sparse, virtual or explicitly materialized source views. Git import and export run at the remote boundary.

Hyperfide owns task assignment, attempts, custody and qualification policy. Source control owns source identities, revisions, workspace mutations and acceptance. Saving source, completing a task, accepting a candidate and publishing to a remote have separate receipts.

## Implementation

| Component | Location |
|---|---|
| Typed contracts and client | [SDK](../../../tools/hyperfide/sdk/source-control/src/) |
| Service composition | [registration.ts](../../../tools/hyperfide/core/control-plane/src/source_control/registration.ts) |
| JSON HTTP transport | [http.ts](../../../tools/hyperfide/core/control-plane/src/source_control/http.ts) |
| Service configuration | [server.ts](../../../tools/hyperfide/core/control-plane/src/source_control/server.ts) |
| Host runtime | [host adapters](../../../tools/hyperfide/core/host-daemon/src/source-control/) |
| App and CLI integration | [source-control plugin](../../../tools/hyperfide/plugins/hyperfide-source-control/README.md) |
| Deployment packaging | [source-control deployment](../../../tools/hyperfide/deploy/hyperfide/source-control/) |

The private `@hyperfide/source-control` package exports the client, `./contracts`, `./host` and `./signed`. Protocol version 1 uses JSON POST requests at `/api/v1/source-control`, with typed `source.*` actions. The standalone listener authenticates bearer tokens; the control-plane transport authenticates signed requests. See [protocol](protocol.md) for the action families.

The service reads `HF_SOURCE_CONTROL_CONFIG`. The client uses `HF_SOURCE_CONFIG` or an explicit `--config` file. The app and CLI use signed configuration containing `schema_version`, `endpoint`, `scope`, `credential_id`, `ca_file` and `key_file`, optionally placed in `transport` alongside a separate `assignment` in a versioned worker configuration. The signing key stays in a protected file. Public source provenance contains author name/email, message and timestamp.

## Source access

The Source control panel and `hyperfide platform source` provide history, workspace, checkpoint, candidate and publication operations. The [plugin guide](../../../tools/hyperfide/plugins/hyperfide-source-control/README.md) documents the supported verbs and configuration shape.

The CLI returns JSON. An unknown mutation outcome returns exit 75; reconciliation uses the original operation identity. Text checkpoint input is UTF-8 and bounded at 64 KiB. Read and write scopes are explicit, and a workspace checkpoint carries workspace, scope and overlay generations.

## Contracts

Read [design](design.body.md), [storage](storage.md), [protocol](protocol.md) and [workspaces](workspaces.md) for the source model. [Security](identity-security.md) defines capability boundaries; [Git interoperability](git-interop.md) defines projections; [migration](migration.md) defines single-writer cutover. [Verification](verification.md) separates component checks from backend, fleet and capacity campaigns.

Capacity profiles are acceptance targets. Repository size, imported history, newly authored revisions, workspace opens, concurrent workers and downloaded bytes are separate measurements.
