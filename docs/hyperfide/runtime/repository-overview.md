# Hyperfide repository overview

Hyperfide lives in [`tools/hyperfide`](../../../tools/hyperfide), a pnpm workspace
inside this monorepo. Its `.nvmrc` pins Node 22.19.0 and its package manager is
pnpm 9.15.0. Run workspace commands from that directory.

| Directory    | Contents                                                                    |
| ------------ | --------------------------------------------------------------------------- |
| `apps`       | Web, desktop, mobile, CLI and launcher applications                         |
| `core`       | Native graph tooling, server, host daemon, persistence and provider bridges |
| `extensions` | Editor integrations                                                         |
| `lib`        | Shared configuration, contracts, UI and utilities                           |
| `plugins`    | Bundled runtime plugins                                                     |
| `sdk`        | Public SDK packages                                                         |
| `tools`      | Build, registry and workspace utilities                                     |

From the monorepo root:

```sh
cd tools/hyperfide
pnpm install --frozen-lockfile
pnpm dev
```

Package commands use workspace-relative directory filters, for example:

```sh
pnpm exec turbo run typecheck test --filter=./apps/web
```

See [system overview](system-overview.md) and [configuration](configuration.md).
