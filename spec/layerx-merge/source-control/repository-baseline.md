# Source-control implementation seams

The native source-control subsystem is implemented under [tools/hyperfide](../../../tools/hyperfide/). Its typed SDK and service share the versioned source protocol.

## Service boundaries

| Component | Responsibility |
|---|---|
| [registration.ts](../../../tools/hyperfide/core/control-plane/src/source_control/registration.ts) | Composes object, metadata, workspace, query, transformation, acceptance, fleet and remote services. |
| [api.ts](../../../tools/hyperfide/core/control-plane/src/source_control/api.ts) | Dispatches typed actions with scoped authorization and admission classes. |
| [objects.ts](../../../tools/hyperfide/core/control-plane/src/source_control/objects.ts) and [trees.ts](../../../tools/hyperfide/core/control-plane/src/source_control/trees.ts) | Encode immutable objects and persistent trees. |
| [metadata.ts](../../../tools/hyperfide/core/control-plane/src/source_control/metadata.ts) | Provides transactional source metadata. |
| [workspaces.ts](../../../tools/hyperfide/core/control-plane/src/source_control/workspaces.ts) | Opens scoped views and validates checkpoint generations. |
| [fleet-adapter.ts](../../../tools/hyperfide/core/control-plane/src/source_control/fleet-adapter.ts) | Binds worker attempts, source checkpoints and submitted custody. |
| [graph-adapter.ts](../../../tools/hyperfide/core/control-plane/src/source_control/graph-adapter.ts) | Carries native source identities into graph operations. |
| [git](../../../tools/hyperfide/core/control-plane/src/source_control/git/) | Imports and projects Git objects and reconciles publication. |
| [migration](../../../tools/hyperfide/core/control-plane/src/source_control/migration/) | Inventories, consolidates and verifies source before authority cutover. |

## Client and host boundaries

The [SDK](../../../tools/hyperfide/sdk/source-control/package.json) is `@hyperfide/source-control`. Shared host code lives in [src/host](../../../tools/hyperfide/sdk/source-control/src/host/); [host-daemon adapters](../../../tools/hyperfide/core/host-daemon/src/source-control/) expose it to the host runtime. The [plugin](../../../tools/hyperfide/plugins/hyperfide-source-control/README.md) provides the Source control panel and CLI.

The [native VCS implementation](../../../tools/hyperfide/core/native/src/vcs.c) remains a distinct local source format. Native object IDs, raw Git OIDs and legacy snapshot identities have separate types and explicit import mappings.

## Verification interfaces

The [qualification runner](../../../tools/hyperfide/tests/hyperfide/qualify.py) and [manifest schema](../../../tools/hyperfide/tests/hyperfide/manifest.schema.json) support versioned evidence. Registry schema v2 accepts `V-NNN` and `SCM-NNN` identities. Source-control registrations live in [source-control/registry.json](../../../tools/hyperfide/tests/hyperfide/source-control/registry.json), and executable cases live in [cases/source_control](../../../tools/hyperfide/tests/hyperfide/cases/source_control/).

[Repository layout](repository-layout.md) describes package boundaries. [Verification](verification.md) defines the workloads and evidence associated with source-control cases.
