# Paxeer X explorer release notes

## Current tree — unreleased

These notes describe the explorer source based on monorepo revision
`5c65ff650b571b5294048762896e896aa35d11ae`. They record the configuration and
interfaces in this tree; they do not certify a deployed release or an API version
compatibility matrix.

## Features and network behavior

The frontend serves the Paxeer X explorer under `/explorer`, as configured in
[next.config.js](./next.config.js). It reads indexed explorer data from the
[Blockscout backend](../../services/explorer/backend), with API hosts configured
in [configs/app/apis.ts](./configs/app/apis.ts).

Paxeer mainnet-beta is one BFT network with EVM chain ID **125** (`0x7d`) and
native currency **PAX** (18 decimals). Its kernel is part of the network and
proposes blocks; a receipt is final when its block is committed. Sub-chains are
tenant domains in the database, launchable without separate infrastructure.

Public node JSON-RPC uses HTTPS at the root path of `api1.mainnet-beta.paxeer.network`
through `api14.mainnet-beta.paxeer.network`, or
`https://api-mainnet-beta.paxeer.network/`. For example, a wallet can use
`https://api1.mainnet-beta.paxeer.network/`. The explorer backend API and node
JSON-RPC have separate configuration inputs.

## UI and account behavior

- The local UI toolkit uses Chakra UI 3. Its provider accepts `value={theme}`;
  see the [toolkit setup](./toolkit/package/README.md).
- Desktop and mobile headers select Auth0 or Dynamic when accounts are enabled,
  and otherwise show the wallet controls when blockchain interaction is enabled.
  Dynamic supports an email account without a connected wallet. See the
  [profile and wallet components](./ui/snippets/user/README.md) for the exact
  feature gates and shared content.

## Dependencies and tooling

The [frontend manifest](./package.json) declares Next.js `15.5.28`, React and
React DOM `19.1.4`, and Chakra UI `3.33.0`. The Node requirement in
[.nvmrc](./.nvmrc) and the manifest is `>=22.14.0`; the manifest also requires
npm `>=10.9.0`. The imported frontend uses Yarn and carries a `yarn.lock`.
The [toolkit package manifest](./toolkit/package/package.json) identifies the
upstream package as `@blockscout/ui-toolkit`, version `2.4.0-alpha`.

## Deployment configuration

The tracked [public preset](./configs/envs/paxeer-x.env) sets
`NEXT_PUBLIC_APP_HOST=paxscan.io`, `NEXT_PUBLIC_API_HOST=api.paxscan.io`,
`NEXT_PUBLIC_NETWORK_ID=125`, and `NEXT_PUBLIC_PAXEER_X_ENABLED=true`.
It uses HTTPS for the app/API and WSS for the API socket connection. The
[development preset](./configs/envs/paxeer-x-dev.env) uses HTTP/WS and requires
replacement of its app and API host placeholders before use. Both presets
disable marketplace and ad providers and hide the native coin price.

For wallet interactions on this network, supply `NEXT_PUBLIC_NETWORK_RPC_URL`
with a root-path public node URL such as
`https://api1.mainnet-beta.paxeer.network/`. Neither tracked preset supplies this
value. Account and wallet availability also depends on the
[provider configuration](./ui/snippets/user/README.md#configuration).

The [environment reference](./docs/ENVS.md) documents the available inputs;
[the app README](./README.md#paxeer-x) explains preset loading.

## Compatibility and provenance

The [import record](../../services/explorer/UPSTREAM.md) identifies the frontend
baseline as Blockscout `v2.7.2` and the backend baseline as Blockscout `v10.2.6`.
These are import versions. Current API resource paths and payload types are
maintained in [lib/api](./lib/api); deployments must provide the APIs used by
their enabled features. Optional service configuration lives in
[configs/app/apis.ts](./configs/app/apis.ts).

The documentation gate checks these pages only. Application builds, backend
compatibility and live deployment health require their own qualification.
