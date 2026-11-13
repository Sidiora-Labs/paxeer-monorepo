# Hyperfide platform

Hyperfide runs agent work in live threads across the browser, desktop, mobile,
and command line. The server coordinates state; host daemons provision local
workspaces and run providers. Plugins add providers, tools, and workspace panels.

![Hyperfide](../../../tools/hyperfide/assets/hyperfide-logo.svg)

## Run from source

The Hyperfide workspace is `tools/hyperfide`. Its `.nvmrc` pins Node 22.19.0,
and its package manager is pnpm 9.15.0. From the monorepo root:

```sh
cd tools/hyperfide
pnpm install --frozen-lockfile
pnpm dev
```

Use `hyperfide platform --help` for installed platform commands. Source commands
use `pnpm hyperfide-platform` for production mode or
`pnpm hyperfide-platform:dev` for development mode from that workspace.
Runtime data defaults to `~/.hyperfide` in production; configuration environment
variables use `HYPERFIDE_`.

See [repository overview](repository-overview.md),
[configuration](configuration.md), and the
[platform integration guide](../hyperfide/platform.md).
