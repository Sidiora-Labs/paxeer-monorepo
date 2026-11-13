# Paxeer X explorer release notes

## Features and network behavior

The frontend serves the Paxeer X explorer under `/explorer`, as configured in
[next.config.js](../../apps/explorer/next.config.js). It reads indexed explorer data from the
[Blockscout backend](../../services/explorer/backend), with API hosts configured
in [configs/app/apis.ts](../../apps/explorer/configs/app/apis.ts).

Paxeer mainnet-beta is one BFT network with EVM chain ID **125** (`0x7d`) and
native currency **PAX** (18 decimals). Its kernel is part of the network and
proposes blocks; a receipt is final when its block is committed. Sub-chains are
tenant domains in the database, launchable without separate infrastructure.

The node JSON-RPC URL uses HTTPS at the root path and is configured separately from the explorer backend API.

## UI and account behavior

- The local UI toolkit uses Chakra UI 3. Its provider accepts `value={theme}`;
  see the [toolkit setup](../../apps/explorer/toolkit/package/README.md).
- Desktop and mobile headers select Auth0 or Dynamic when accounts are enabled,
  and otherwise show the wallet controls when blockchain interaction is enabled.
  Dynamic supports an email account without a connected wallet. See the
  [profile and wallet components](../../apps/explorer/ui/snippets/user/README.md) for the exact
  feature gates and shared content.

## Dependencies and tooling

The [frontend manifest](../../apps/explorer/package.json) declares Next.js `15.5.28`, React and
React DOM `19.1.4`, and Chakra UI `3.33.0`. The Node requirement in
[.nvmrc](../../apps/explorer/.nvmrc) and the manifest is `>=22.14.0`; the manifest also requires
npm `>=10.9.0`. The imported frontend uses Yarn and carries a `yarn.lock`.
The [toolkit package manifest](../../apps/explorer/toolkit/package/package.json) identifies the
upstream package as `@blockscout/ui-toolkit`, version `2.4.0-alpha`.

## Deployment configuration

The tracked [public preset](../../apps/explorer/configs/envs/paxeer-x.env) sets
the app and backend API hosts,
`NEXT_PUBLIC_NETWORK_ID=125`, and `NEXT_PUBLIC_PAXEER_X_ENABLED=true`.
It uses HTTPS for the app/API and WSS for the API socket connection. The
[development preset](../../apps/explorer/configs/envs/paxeer-x-dev.env) uses HTTP/WS and requires
replacement of its app and API host placeholders before use. Both presets
disable marketplace and ad providers. The public preset shows the native coin price; the development preset hides it.

For wallet interactions on this network, supply `NEXT_PUBLIC_NETWORK_RPC_URL`
with a root-path public node URL. Neither tracked preset supplies this
value. Account and wallet availability also depends on the
[provider configuration](../../apps/explorer/ui/snippets/user/README.md#configuration).

The [environment reference](./ENVS.md) documents the available inputs;
[the app README](../../apps/explorer/README.md#paxeer-x) explains preset loading.

## API compatibility

Current API resource paths and payload types are
maintained in [lib/api](../../apps/explorer/lib/api); deployments must provide the APIs used by
their enabled features. Optional service configuration lives in
[configs/app/apis.ts](../../apps/explorer/configs/app/apis.ts).
