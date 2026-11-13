# Hyperfide repository layout

The Hyperfide codebase lives under [tools/hyperfide](../../../tools/hyperfide/). Its packages are grouped by purpose. Public package names, CLI commands and module identities are defined by the manifests within that tree.

## Categories

Paths in this table are relative to `tools/hyperfide/`.

| Directory | Contents |
|---|---|
| `core/native/src/` | Native C command line and graph implementation |
| `core/control-plane/` | Independent control-services npm package, migrations and source-control service |
| `core/server/`, `core/host-daemon/` | Server and host runtime |
| `core/authority-client/` | Authority client and persistence support |
| `core/kvx/` | KVX module and language support |
| `sdk/` | Client and plugin SDKs, including `sdk/source-control/` |
| `contracts/` | Cross-process schemas, including fleet contracts |
| `lib/` | Shared internal libraries |
| `apps/` | Web, desktop, mobile, CLI and launcher applications |
| `plugins/` | Product plugins and providers |
| `extensions/` | Editor extensions |
| `tools/` | Build, workspace and repository tooling |
| `examples/`, `tests/` | Examples, cases and fixtures |
| `runtime/`, `deploy/` | Runtime entry points and deployment packaging |
| `vendor/`, `licenses/` | Third-party sources and attribution |

Product documentation also lives in [docs/hyperfide](../../../docs/hyperfide/). The [layout mapping](repository-layout.json) describes source-to-category mappings.

## Package and toolchain boundaries

The [workspace manifest](../../../tools/hyperfide/pnpm-workspace.yaml) includes category packages and explicitly excludes the independent control-plane package, KVX module and separately packaged extensions. The [root package](../../../tools/hyperfide/package.json) pins pnpm 9.15.0 and requires Node at least 22.19.0. Its build script uses Turbo.

The [control-plane package](../../../tools/hyperfide/core/control-plane/package.json) pins Node 24.22.0 and npm 11.19.1. It retains its own lockfile, TypeScript configuration and migrations. From `tools/hyperfide/core/control-plane/`, `npm run build` synchronizes contracts, compiles the service and writes its distribution stamp. `npm run source-control` starts the built source-control server.

The native build uses the [Makefile](../../../tools/hyperfide/Makefile). The public CLI discovers the platform front door at [apps/cli/bin/hyperfide-platform](../../../tools/hyperfide/apps/cli/bin/hyperfide-platform).

## Source-control packages

The service is [core/control-plane/src/source_control](../../../tools/hyperfide/core/control-plane/src/source_control/). The client package is [sdk/source-control](../../../tools/hyperfide/sdk/source-control/), and host integration is [core/host-daemon/src/source-control](../../../tools/hyperfide/core/host-daemon/src/source-control/). The [source-control plugin](../../../tools/hyperfide/plugins/hyperfide-source-control/README.md) provides product and CLI integration.

Package-local migrations, tests and assets remain with their owning package. Installed artifact paths are defined by deployment packaging and can differ from checkout paths.
