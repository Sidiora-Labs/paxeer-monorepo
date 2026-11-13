# Paxeer Names shell

This directory holds the Next.js shell for published `.pax` sites on the Paxeer network. It uses `@paxeer/names` from `sdk/names` to resolve a site's manifest digest with storage-proof verification, checks the manifest and content hashes before serving files from the configured bucket, and provides a status page for checking a name's published content.

## Layout

- `src/` — App Router pages and handlers, host routing in `proxy.ts`, and content verification and serving in `lib/serve.ts`.
- `e2e/` — Shell request tests covering published content, host routing, digest checks, proof failures and malformed paths.
- `package.json` — Package dependencies, runtime requirements and development, build, start and preparation scripts.
- `pnpm-lock.yaml` — Locked dependency versions.
- `pnpm-workspace.yaml` — Local pnpm workspace definition.
- `.npmrc` — pnpm module and virtual store locations.
- `next.config.ts` — Next.js response-header and trailing-slash configuration.
- `tsconfig.json` — TypeScript options and source, test and Next.js type inputs.

## Build and test

Use Node.js 24 or later and the pnpm version declared in `package.json`. Install the declared dependencies before running the package scripts. From this directory:

```sh
pnpm run prepare
pnpm run build
```

`prepare` compiles the local `sdk/names` dependency into the shell's installed package. `build` runs `next build`. Use `pnpm run dev` for the development server and `pnpm run start` to serve the production build.

Shell tests are in `e2e/shell.test.ts` and use `node:test`; `apps/names/package.json` does not declare a test script.

## Configuration

The server reads these environment variables:

| Variable | Purpose |
| --- | --- |
| `NAMES_SHELL_DOMAIN` | Base domain used to match site hosts. |
| `NAMES_RPC_URL` | Node JSON-RPC endpoint used for name resolution and storage proofs. |
| `NAMES_RESOLVER` | Resolver contract used by the names SDK. |
| `NAMES_CONSENSUS_RPC_URL` | Consensus RPC endpoint used to retrieve headers for proof verification. |
| `NAMES_BUCKET_BASE_URL` | Base URL for published manifests and content objects. |

Build and package configuration lives in `next.config.ts`, `tsconfig.json`, `package.json`, `pnpm-workspace.yaml` and `.npmrc`.

## Documentation

- [Release readiness matrix](../../docs/release/7.0.0-readiness-matrix.md) — Build and test entry points for the names shell, SDK and publishing tools.
