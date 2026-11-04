# Explorer frontend configuration contract

The maintained frontend configuration lives under
[apps/explorer/configs](../apps/explorer/configs/). It describes the explorer
for the single Paxeer network, EVM chain ID **125 (`0x7d`)**. The kernel is part
of the network and proposes blocks; finality follows committed BFT blocks.
Frontend settings select presentation and clients. They do not change the
network's consensus or provision a tenant domain.

The browser consumes the explorer backend's HTTP API and WebSocket service.
Wallet interaction uses node JSON-RPC. Configure those two destinations
separately using this page and the [backend contract](explorer-config.md).

## Configuration loading

[configs/app/index.ts](../apps/explorer/configs/app/index.ts) freezes the
assembled `app`, `chain`, `apis`, `UI`, `features`, `services`, and `meta`
configuration. [utils.ts](../apps/explorer/configs/app/utils.ts) reads
`process.env` on the server and `window.__envs` in the browser. In the `pw`
instance, browser local storage can override values for tests. The helper
replaces single quotes with double quotes before JSON parsing; invalid JSON
returns `null` in `parseEnvJson`. Feature booleans commonly require the exact
string `true`; do not assume the string `false` is false under every custom
consumer's truthiness check.

[configs/envs/paxeer-x.env](../apps/explorer/configs/envs/paxeer-x.env) is the
tracked Paxeer preset. The container preset filename is
`configs/envs/.env.paxeer-x`, selected with `ENVS_PRESET=paxeer-x`; the tracked
file describes the copy step. Validate values against the owning
[environment schema](../apps/explorer/deploy/tools/envs-validator/schema.ts)
and feature schemas before starting a deployment. Public variables must not
contain deployment keys, API secrets, or private evidence.

<a id="cfg-app"></a>

## App origin and base path

[app.ts](../apps/explorer/configs/app/app.ts) reads:

| Variable | Interpretation |
| --- | --- |
| `NEXT_PUBLIC_APP_PROTOCOL` | scheme, defaults to `https` |
| `NEXT_PUBLIC_APP_HOST` | hostname, without a scheme or path |
| `NEXT_PUBLIC_APP_PORT` | optional explicit port appended to the origin |
| `NEXT_PUBLIC_APP_ENV` | `development` and `review` select their matching flags |
| `NEXT_PUBLIC_APP_INSTANCE` | instance label; `pw` selects test behavior |
| `NEXT_PUBLIC_USE_NEXT_JS_PROXY` | exact `true` enables the proxy flag |
| `NEXT_PUBLIC_ICON_SPRITE_HASH` | sprite cache identity |

The application base path is **`/explorer`** and `baseUrl` is the assembled
origin plus that path. Include the actual external port only if users reach
it on that port; a container's listen port need not be the public port.
Private mode is read from the app-profile cookie and suppresses wallet
features in their respective configuration modules.

<a id="cfg-chain"></a>

## Chain

[chain.ts](../apps/explorer/configs/app/chain.ts) owns the wallet network:

| Variable | Mainnet-beta value / behavior |
| --- | --- |
| `NEXT_PUBLIC_NETWORK_ID` | `125` |
| `NEXT_PUBLIC_NETWORK_NAME`, `NEXT_PUBLIC_NETWORK_SHORT_NAME` | display names; preset uses Paxeer X Network / Paxeer X |
| `NEXT_PUBLIC_NETWORK_CURRENCY_NAME`, `NEXT_PUBLIC_NETWORK_CURRENCY_SYMBOL` | preset uses Paxeer / PAX |
| `NEXT_PUBLIC_NETWORK_CURRENCY_DECIMALS` | 18; parser also defaults to 18 |
| `NEXT_PUBLIC_NETWORK_RPC_URL` | one valid URL or a JSON array of URLs; absent input produces an empty list |
| `NEXT_PUBLIC_NETWORK_VERIFICATION_TYPE` | `validation` in the Paxeer preset; generic fallback is `mining` when unset and no rollup override applies |
| `NEXT_PUBLIC_IS_TESTNET` | `false` in the mainnet preset; exact `true` marks a test network |
| `NEXT_PUBLIC_NETWORK_TOKEN_STANDARD_NAME` | defaults to `ERC` |
| `NEXT_PUBLIC_NETWORK_MULTIPLE_GAS_CURRENCIES` | exact `true` enables the flag |

Set node URLs to [root JSON-RPC endpoints](evm-rpc.md#http-endpoint), such as
`https://api1.mainnet-beta.paxeer.network/`. The public pool consists of
`api1` through `api14.mainnet-beta.paxeer.network` and
`api-mainnet-beta.paxeer.network`. The tracked preset does not set
`NEXT_PUBLIC_NETWORK_RPC_URL`; a deployment wanting contract write interaction
must supply it alongside the other required wallet settings.
`node.hyperpaxeer.com` is a library mirror, not an RPC endpoint.
The pre-merge router and its /rpc, /rpc/ws and /rpc/schema paths are retired.
There is no separate LayerX interface or separate sequencer service to expose.

Rollup-specific frontend modules support other upstream chain types; enabling
them is not a representation of Paxeer's merged network. Tenant sub-chains
are database domains; a multichain display setting alone does not launch one.

<a id="cfg-apis"></a>

## APIs

[apis.ts](../apps/explorer/configs/app/apis.ts) assembles the general backend
API only when `NEXT_PUBLIC_API_HOST` is provided:

| Variable | Interpretation |
| --- | --- |
| `NEXT_PUBLIC_API_HOST` | backend hostname without scheme, port or path |
| `NEXT_PUBLIC_API_PROTOCOL` | defaults to `https` |
| `NEXT_PUBLIC_API_PORT` | optional port, also used in the socket origin |
| `NEXT_PUBLIC_API_BASE_PATH` | backend path prefix, trailing slash stripped; defaults to empty |
| `NEXT_PUBLIC_API_WEBSOCKET_PROTOCOL` | defaults to `wss` |

The general API endpoint is `protocol://host[:port]`; its socket endpoint is
`websocketProtocol://host[:port]`. The API base path is independent of the
frontend's `/explorer` path. The preset names `api.paxscan.io` as the API host;
it is an explorer backend, not a public node endpoint.

Optional service variables such as `NEXT_PUBLIC_STATS_API_HOST`,
`NEXT_PUBLIC_METADATA_SERVICE_API_HOST`, `NEXT_PUBLIC_CONTRACT_INFO_API_HOST`,
and `NEXT_PUBLIC_USER_OPS_INDEXER_API_HOST` are used directly as service
endpoints. Despite the `HOST` suffix, these readers do not assemble their
scheme the way the general API reader does. The multichain aggregator requires
both `NEXT_PUBLIC_MULTICHAIN_AGGREGATOR_API_HOST` and
`NEXT_PUBLIC_MULTICHAIN_CLUSTER`; it constructs
`/api/v1/clusters/<cluster>` and a WSS origin from the parsed URL. Malformed
URLs disable that optional API configuration.

<a id="cfg-features"></a>

## Features

[features/paxeerXFeed.ts](../apps/explorer/configs/app/features/paxeerXFeed.ts)
and [features/paxeerXLists.ts](../apps/explorer/configs/app/features/paxeerXLists.ts)
enable their frontend modules only when `NEXT_PUBLIC_PAXEER_X_ENABLED` is
exactly `true`. Runtime availability comes from
`/api/v2/paxeer-x/capabilities`; the price-feed UI also consults its
`price_feed` flag. Backend probing and native feed indexing are controlled by
[backend settings](explorer-config.md#paxeer-capabilities), not this flag.
Keep absent or unavailable data visible as such; a frontend switch is not
proof that an address is active at the selected height.

[blockchainInteraction.ts](../apps/explorer/configs/app/features/blockchainInteraction.ts)
requires the network ID, name, currency name, symbol, decimals and at least one
RPC URL for single-chain wallet interaction, or enabled multichain config.
It also requires either enabled Dynamic account configuration with its
environment ID or `NEXT_PUBLIC_WALLET_CONNECT_PROJECT_ID` for Reown.
Private mode suppresses it. `NEXT_PUBLIC_WALLET_CONNECT_FEATURED_WALLET_IDS`
is an optional JSON array.

[web3Wallet.ts](../apps/explorer/configs/app/features/web3Wallet.ts) separately
controls add-network/add-token wallet integrations. `NEXT_PUBLIC_WEB3_WALLETS`
is a JSON list filtered against supported wallet names; `none` disables it.
`NEXT_PUBLIC_WEB3_DISABLE_ADD_TOKEN_TO_WALLET=true` disables add-token actions.
This feature does not bypass the contract-write prerequisites above.

Each optional module under `features/` owns its enable conditions, required
service URLs, and payload shape. Configure its backend counterpart as well;
feature names in the source index do not imply that every upstream feature is
enabled on Paxeer.

<a id="cfg-ui"></a>
<a id="cfg-meta"></a>
<a id="cfg-services"></a>
<a id="cfg-paxeer-x"></a>
<a id="cfg-essential-dapps-chains"></a>
<a id="cfg-multichain"></a>

## UI, metadata and supporting configs

[ui.ts](../apps/explorer/configs/app/ui.ts) and
[ui/views](../apps/explorer/configs/app/ui/views/) own navigation, branding,
homepage sections and per-resource display choices. The Paxeer preset uses a
horizontal navigation layout, disables native coin price display without a
market source, and disables third-party ad providers. These are presentation
choices, independent of ledger balances and receipt validity.

[meta.ts](../apps/explorer/configs/app/meta.ts) owns social/SEO metadata;
[services.ts](../apps/explorer/configs/app/services.ts) owns supporting service
configuration. [configs/paxeer-x/footer.json](../apps/explorer/configs/paxeer-x/footer.json)
contains the maintained footer link data.
[essential-dapps-chains](../apps/explorer/configs/essential-dapps-chains/)
and [multichain](../apps/explorer/configs/multichain/) own their separate
catalogs. They do not redefine Paxeer's chain ID or its domain registry.

Use [address activation](address-inventory.md),
[bridge deployment records](bridge-deploy.md#app-env), and
[bridge vectors](bridge-vectors.md) for those contracts. Bridge
`NEXT_PUBLIC_BRIDGE_*` deployment exports belong to the bridge app and are
not substitutes for this explorer configuration.

## Registry source index

These link targets preserve interface-registry references. Each row points
to the current source owner for its declaration, ABI, message, configuration
family or helper. It does not assert that a method is registered, a feature
is enabled, or a contract is deployed. Rows marked retired identify source
references absent from this tree and must not be used as deployment targets.

| Link targets | Source owner |
| --- | --- |
| <a id="env-account"></a>`env-account` | [apps/explorer/configs/app/features/account.ts](../apps/explorer/configs/app/features/account.ts) |
| <a id="env-ad"></a>`env-ad` | [apps/explorer/configs/app/features/adsBanner.ts](../apps/explorer/configs/app/features/adsBanner.ts); [apps/explorer/configs/app/features/adsText.ts](../apps/explorer/configs/app/features/adsText.ts) |
| <a id="env-address"></a>`env-address` | [apps/explorer/configs/app/features/address3rdPartyWidgets.ts](../apps/explorer/configs/app/features/address3rdPartyWidgets.ts); [apps/explorer/configs/app/features/addressProfileAPI.ts](../apps/explorer/configs/app/features/addressProfileAPI.ts) |
| <a id="env-admin"></a>`env-admin` | [apps/explorer/configs/app/apis.ts](../apps/explorer/configs/app/apis.ts) |
| <a id="env-advanced"></a>`env-advanced` | [apps/explorer/configs/app/features/advancedFilter.ts](../apps/explorer/configs/app/features/advancedFilter.ts) |
| <a id="env-api"></a>`env-api` | [apps/explorer/configs/app/apis.ts](../apps/explorer/configs/app/apis.ts); [apps/explorer/configs/app/features/apiDocs.ts](../apps/explorer/configs/app/features/apiDocs.ts); [apps/explorer/configs/app/ui.ts](../apps/explorer/configs/app/ui.ts) |
| <a id="env-app"></a>`env-app` | [apps/explorer/configs/app/app.ts](../apps/explorer/configs/app/app.ts) |
| <a id="env-beacon"></a>`env-beacon` | [apps/explorer/configs/app/features/beaconChain.ts](../apps/explorer/configs/app/features/beaconChain.ts) |
| <a id="env-bridged"></a>`env-bridged` | [apps/explorer/configs/app/features/bridgedTokens.ts](../apps/explorer/configs/app/features/bridgedTokens.ts) |
| <a id="env-celo"></a>`env-celo` | [apps/explorer/configs/app/features/celo.ts](../apps/explorer/configs/app/features/celo.ts) |
| <a id="env-clusters"></a>`env-clusters` | [apps/explorer/configs/app/apis.ts](../apps/explorer/configs/app/apis.ts); [apps/explorer/configs/app/features/nameServices.ts](../apps/explorer/configs/app/features/nameServices.ts) |
| <a id="env-color"></a>`env-color` | [apps/explorer/configs/app/ui.ts](../apps/explorer/configs/app/ui.ts) |
| <a id="env-contract"></a>`env-contract` | [apps/explorer/configs/app/apis.ts](../apps/explorer/configs/app/apis.ts); [apps/explorer/configs/app/ui.ts](../apps/explorer/configs/app/ui.ts) |
| <a id="env-cross"></a>`env-cross` | [apps/explorer/configs/app/features/crossChainTxs.ts](../apps/explorer/configs/app/features/crossChainTxs.ts) |
| <a id="env-data"></a>`env-data` | [apps/explorer/configs/app/features/dataAvailability.ts](../apps/explorer/configs/app/features/dataAvailability.ts) |
| <a id="env-defi"></a>`env-defi` | [apps/explorer/configs/app/features/deFiDropdown.ts](../apps/explorer/configs/app/features/deFiDropdown.ts) |
| <a id="env-dex"></a>`env-dex` | [apps/explorer/configs/app/features/pools.ts](../apps/explorer/configs/app/features/pools.ts) |
| <a id="env-fault"></a>`env-fault` | [apps/explorer/configs/app/features/faultProofSystem.ts](../apps/explorer/configs/app/features/faultProofSystem.ts) |
| <a id="env-featured"></a>`env-featured` | [apps/explorer/configs/app/ui.ts](../apps/explorer/configs/app/ui.ts) |
| <a id="env-flashblocks"></a>`env-flashblocks` | [apps/explorer/configs/app/features/flashblocks.ts](../apps/explorer/configs/app/features/flashblocks.ts) |
| <a id="env-font"></a>`env-font` | [apps/explorer/configs/app/ui.ts](../apps/explorer/configs/app/ui.ts) |
| <a id="env-footer"></a>`env-footer` | [apps/explorer/configs/app/ui.ts](../apps/explorer/configs/app/ui.ts) |
| <a id="env-game"></a>`env-game` | [apps/explorer/configs/app/features/easterEggBadge.ts](../apps/explorer/configs/app/features/easterEggBadge.ts) |
| <a id="env-gas"></a>`env-gas` | [apps/explorer/configs/app/features/gasTracker.ts](../apps/explorer/configs/app/features/gasTracker.ts); [apps/explorer/configs/app/features/getGasButton.ts](../apps/explorer/configs/app/features/getGasButton.ts) |
| <a id="env-git"></a>`env-git` | [apps/explorer/configs/app/features/rollbar.ts](../apps/explorer/configs/app/features/rollbar.ts) |
| <a id="env-google"></a>`env-google` | [apps/explorer/configs/app/features/googleAnalytics.ts](../apps/explorer/configs/app/features/googleAnalytics.ts) |
| <a id="env-growth"></a>`env-growth` | [apps/explorer/configs/app/features/growthBook.ts](../apps/explorer/configs/app/features/growthBook.ts) |
| <a id="env-has"></a>`env-has` | [apps/explorer/configs/app/features/beaconChain.ts](../apps/explorer/configs/app/features/beaconChain.ts); [apps/explorer/configs/app/features/mudFramework.ts](../apps/explorer/configs/app/features/mudFramework.ts); [apps/explorer/configs/app/features/userOps.ts](../apps/explorer/configs/app/features/userOps.ts); [apps/explorer/configs/app/ui.ts](../apps/explorer/configs/app/ui.ts) |
| <a id="env-helia"></a>`env-helia` | [apps/explorer/configs/app/ui/views/nft.ts](../apps/explorer/configs/app/ui/views/nft.ts) |
| <a id="env-hide"></a>`env-hide` | [apps/explorer/configs/app/ui.ts](../apps/explorer/configs/app/ui.ts) |
| <a id="env-homepage"></a>`env-homepage` | [apps/explorer/configs/app/ui.ts](../apps/explorer/configs/app/ui.ts) |
| <a id="env-hostname"></a>`env-hostname` | [apps/explorer/configs/essential-dapps-chains/config.edge.ts](../apps/explorer/configs/essential-dapps-chains/config.edge.ts) |
| <a id="env-hot"></a>`env-hot` | [apps/explorer/configs/app/features/hotContracts.ts](../apps/explorer/configs/app/features/hotContracts.ts) |
| <a id="env-icon"></a>`env-icon` | [apps/explorer/configs/app/app.ts](../apps/explorer/configs/app/app.ts) |
| <a id="env-interchain"></a>`env-interchain` | [apps/explorer/configs/app/apis.ts](../apps/explorer/configs/app/apis.ts) |
| <a id="env-interop"></a>`env-interop` | [apps/explorer/configs/app/features/rollup.ts](../apps/explorer/configs/app/features/rollup.ts) |
| <a id="env-is"></a>`env-is` | [apps/explorer/configs/app/chain.ts](../apps/explorer/configs/app/chain.ts); [apps/explorer/configs/app/features/account.ts](../apps/explorer/configs/app/features/account.ts); [apps/explorer/configs/app/features/suave.ts](../apps/explorer/configs/app/features/suave.ts) |
| <a id="env-maintenance"></a>`env-maintenance` | [apps/explorer/configs/app/ui.ts](../apps/explorer/configs/app/ui.ts) |
| <a id="env-marketplace"></a>`env-marketplace` | [apps/explorer/configs/app/features/marketplace.ts](../apps/explorer/configs/app/features/marketplace.ts) |
| <a id="env-max"></a>`env-max` | [apps/explorer/configs/app/ui.ts](../apps/explorer/configs/app/ui.ts) |
| <a id="env-mega"></a>`env-mega` | [apps/explorer/configs/app/features/megaEth.ts](../apps/explorer/configs/app/features/megaEth.ts) |
| <a id="env-metadata"></a>`env-metadata` | [apps/explorer/configs/app/apis.ts](../apps/explorer/configs/app/apis.ts); [apps/explorer/configs/app/features/addressMetadata.ts](../apps/explorer/configs/app/features/addressMetadata.ts) |
| <a id="env-metasuites"></a>`env-metasuites` | [apps/explorer/configs/app/features/metasuites.ts](../apps/explorer/configs/app/features/metasuites.ts) |
| <a id="env-mixpanel"></a>`env-mixpanel` | [apps/explorer/configs/app/features/mixpanel.ts](../apps/explorer/configs/app/features/mixpanel.ts) |
| <a id="env-multichain"></a>`env-multichain` | [apps/explorer/configs/app/apis.ts](../apps/explorer/configs/app/apis.ts); [apps/explorer/configs/app/features/multichain.ts](../apps/explorer/configs/app/features/multichain.ts); [apps/explorer/configs/app/features/multichainButton.ts](../apps/explorer/configs/app/features/multichainButton.ts) |
| <a id="env-name"></a>`env-name` | [apps/explorer/configs/app/apis.ts](../apps/explorer/configs/app/apis.ts); [apps/explorer/configs/app/features/nameServices.ts](../apps/explorer/configs/app/features/nameServices.ts) |
| <a id="env-navigation"></a>`env-navigation` | [apps/explorer/configs/app/ui.ts](../apps/explorer/configs/app/ui.ts) |
| <a id="env-network"></a>`env-network` | [apps/explorer/configs/app/chain.ts](../apps/explorer/configs/app/chain.ts); [apps/explorer/configs/app/ui.ts](../apps/explorer/configs/app/ui.ts) |
| <a id="env-next"></a>`env-next` | [apps/explorer/configs/essential-dapps-chains/config.edge.ts](../apps/explorer/configs/essential-dapps-chains/config.edge.ts) |
| <a id="env-og"></a>`env-og` | [apps/explorer/configs/app/meta.ts](../apps/explorer/configs/app/meta.ts) |
| <a id="env-other"></a>`env-other` | [apps/explorer/configs/app/ui.ts](../apps/explorer/configs/app/ui.ts) |
| <a id="env-paxeer"></a>`env-paxeer` | [apps/explorer/configs/app/features/paxeerXFeed.ts](../apps/explorer/configs/app/features/paxeerXFeed.ts) |
| <a id="env-promote"></a>`env-promote` | [apps/explorer/configs/app/meta.ts](../apps/explorer/configs/app/meta.ts) |
| <a id="env-puzzle"></a>`env-puzzle` | [apps/explorer/configs/app/features/easterEggPuzzleBadge.ts](../apps/explorer/configs/app/features/easterEggPuzzleBadge.ts) |
| <a id="env-re"></a>`env-re` | [apps/explorer/configs/app/services.ts](../apps/explorer/configs/app/services.ts) |
| <a id="env-rewards"></a>`env-rewards` | [apps/explorer/configs/app/apis.ts](../apps/explorer/configs/app/apis.ts) |
| <a id="env-rollbar"></a>`env-rollbar` | [apps/explorer/configs/app/features/rollbar.ts](../apps/explorer/configs/app/features/rollbar.ts) |
| <a id="env-rollup"></a>`env-rollup` | [apps/explorer/configs/app/chain.ts](../apps/explorer/configs/app/chain.ts); [apps/explorer/configs/app/features/rollup.ts](../apps/explorer/configs/app/features/rollup.ts) |
| <a id="env-safe"></a>`env-safe` | [apps/explorer/configs/app/features/safe.ts](../apps/explorer/configs/app/features/safe.ts) |
| <a id="env-seo"></a>`env-seo` | [apps/explorer/configs/app/meta.ts](../apps/explorer/configs/app/meta.ts) |
| <a id="env-stats"></a>`env-stats` | [apps/explorer/configs/app/apis.ts](../apps/explorer/configs/app/apis.ts) |
| <a id="env-tac"></a>`env-tac` | [apps/explorer/configs/app/apis.ts](../apps/explorer/configs/app/apis.ts); [apps/explorer/configs/app/features/tac.ts](../apps/explorer/configs/app/features/tac.ts) |
| <a id="env-transaction"></a>`env-transaction` | [apps/explorer/configs/app/features/txInterpretation.ts](../apps/explorer/configs/app/features/txInterpretation.ts) |
| <a id="env-tx"></a>`env-tx` | [apps/explorer/configs/app/features/externalTxs.ts](../apps/explorer/configs/app/features/externalTxs.ts) |
| <a id="env-use"></a>`env-use` | [apps/explorer/configs/app/app.ts](../apps/explorer/configs/app/app.ts) |
| <a id="env-user"></a>`env-user` | [apps/explorer/configs/app/apis.ts](../apps/explorer/configs/app/apis.ts) |
| <a id="env-validators"></a>`env-validators` | [apps/explorer/configs/app/features/validators.ts](../apps/explorer/configs/app/features/validators.ts) |
| <a id="env-views"></a>`env-views` | [apps/explorer/configs/app/ui/views/address.ts](../apps/explorer/configs/app/ui/views/address.ts); [apps/explorer/configs/app/ui/views/block.ts](../apps/explorer/configs/app/ui/views/block.ts); [apps/explorer/configs/app/ui/views/nft.ts](../apps/explorer/configs/app/ui/views/nft.ts); [apps/explorer/configs/app/ui/views/token.ts](../apps/explorer/configs/app/ui/views/token.ts); [apps/explorer/configs/app/ui/views/tx.ts](../apps/explorer/configs/app/ui/views/tx.ts) |
| <a id="env-visualize"></a>`env-visualize` | [apps/explorer/configs/app/apis.ts](../apps/explorer/configs/app/apis.ts) |
| <a id="env-wallet"></a>`env-wallet` | [apps/explorer/configs/app/features/blockchainInteraction.ts](../apps/explorer/configs/app/features/blockchainInteraction.ts) |
| <a id="env-web3"></a>`env-web3` | [apps/explorer/configs/app/features/web3Wallet.ts](../apps/explorer/configs/app/features/web3Wallet.ts) |
| <a id="env-xstar"></a>`env-xstar` | [apps/explorer/configs/app/features/xStarScore.ts](../apps/explorer/configs/app/features/xStarScore.ts) |
| <a id="env-zetachain"></a>`env-zetachain` | [apps/explorer/configs/app/apis.ts](../apps/explorer/configs/app/apis.ts); [apps/explorer/configs/app/features/zetachain.ts](../apps/explorer/configs/app/features/zetachain.ts) |
