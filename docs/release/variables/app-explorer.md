# Explorer application variables

The explorer application in `apps/explorer` serves the Paxeer mainnet-beta explorer at `/explorer`.
Paxeer is one BFT network with EVM chain ID `125` (`0x7d`); committed blocks have final receipts,
and public node JSON-RPC uses the root path. This page lists the environment inputs read by the
application, its startup and deployment scripts, and the keys read from its configuration assets.

`NEXT_PUBLIC_` values are public: the runtime environment script and configuration API expose them
to browsers. Public client tokens and project identifiers are not server secrets. The server-only
assistant credential and deployment Vault token must stay outside public settings. Host addresses,
account identifiers and credential values are omitted here and in the example.

Environment files use flat `NAME=value` keys with no sections. `tools/scripts/dev.sh` loads
`.env.development.local`, `.env.local`, `.env.development`, `.env` and `.env.secrets`; the secrets
file is loaded for application startup. `tools/scripts/dev.preset.sh` loads
`configs/envs/.env.<preset>` and `configs/envs/.env.secrets`. Container startup can load the
selected preset, and `deploy/scripts/make_envs_script.sh` fills only unset public inputs from the
packaged `.env`. An explicitly empty input takes precedence over a packaged value. `.env.registry`
lists public input names for startup validation.

`configs/paxeer-x/footer.json` supplies the same footer-link group fields as the prepared
`public/assets/configs/footer_links.json` asset.

Defaults below describe the source behavior when a value is absent. Feature-dependent settings apply
only when that feature is selected; configure their dependencies together. The root `.env.example`
leaves credentials and public integration identifiers empty and comments out the other assignments
so that deployments can enable the settings they use. Empty examples mean no literal default, a
derived default, or an omitted host address. Framework and shell inputs are supplied by their
runtimes. The example shows the ordinary single-chain fallback where a default depends on other
settings.

File rows use paths relative to `apps/explorer`, followed by the section and key as a dotted
JSON/YAML path: `[]` denotes each array item and `<name>` a map entry. JSON assets selected by the
public inputs are prepared under `public/assets/configs`; inline JSON and local file assets use the
same schemas. Nested JSON values stored in an environment file are listed as `.env / VARIABLE.key`;
the same keys apply when that variable comes from another env file or the process environment. The
marketplace banner content is an HTML asset. Chain caches are generated from the corresponding
explorer environment settings and loaded from the files listed below.

## Required

The app host and network name are required by both startup schemas. The API host and network ID are
required in single-chain mode. Review-deployment inputs are required only when rendering the review
Helm releases and resolving their Vault references.

| Name / file, section and key | What it sets | Source default | Secret | Source reader |
| --- | --- | --- | --- | --- |
| `GITHUB_REF_NAME_SLUG` | Names review namespaces, image tags and ingress instances; required only when rendering review Helm releases. | None | No | `apps/explorer/deploy/helmfile.yaml.gotmpl` |
| `NEXT_PUBLIC_API_HOST` | Explorer backend host, without scheme; required for single-chain API configuration. | None | No | `apps/explorer/configs/app/apis.ts`; `apps/explorer/deploy/tools/sitemap-generator/next-sitemap.config.js` |
| `NEXT_PUBLIC_APP_HOST` | Explorer frontend host used for canonical URLs, server fetching and metadata. | None | No | `apps/explorer/configs/app/app.ts`; `apps/explorer/deploy/scripts/og_image_generator.js`; `apps/explorer/deploy/tools/sitemap-generator/next-sitemap.config.js`; `apps/explorer/instrumentation.node.ts` |
| `NEXT_PUBLIC_NETWORK_ID` | Positive EVM chain identifier; required in single-chain mode; Paxeer mainnet-beta uses 125. | None | No | `apps/explorer/configs/app/chain.ts`; `apps/explorer/nextjs/getServerSideProps/handlers.ts` |
| `NEXT_PUBLIC_NETWORK_NAME` | Displayed chain name, metadata title and generated image label. | None | No | `apps/explorer/configs/app/chain.ts`; `apps/explorer/deploy/scripts/og_image_generator.js`; `apps/explorer/nextjs/getServerSideProps/handlers.ts` |
| `VAULT_TOKEN` | Authenticates Vault resolution of review deployment secret references; also read by deploy/values/review-l2/values.yaml.gotmpl. | None | Yes; server/deployment only | `apps/explorer/deploy/values/review/values.yaml.gotmpl` |

## Optional

Feature-specific requirements are stated in the setting description; asset-entry fields marked
required apply only when that asset is supplied. Rollup and alternate-chain settings select optional
presentations. For Paxeer mainnet-beta, use chain ID 125 and validation terminology.

| Name / file, section and key | What it sets | Source default | Secret | Source reader |
| --- | --- | --- | --- | --- |
| `.env` / `NEXT_PUBLIC_ADDRESS_USERNAME_TAG.api_url_template` | Profile API request template; required when configured. | None | No | `apps/explorer/configs/app/features/addressProfileAPI.ts` |
| `.env` / `NEXT_PUBLIC_ADDRESS_USERNAME_TAG.tag_bg_color` | Profile tag background. | None | No | `apps/explorer/configs/app/features/addressProfileAPI.ts` |
| `.env` / `NEXT_PUBLIC_ADDRESS_USERNAME_TAG.tag_icon` | Profile tag icon. | None | No | `apps/explorer/configs/app/features/addressProfileAPI.ts` |
| `.env` / `NEXT_PUBLIC_ADDRESS_USERNAME_TAG.tag_link_template` | Profile tag destination template. | None | No | `apps/explorer/configs/app/features/addressProfileAPI.ts` |
| `.env` / `NEXT_PUBLIC_ADDRESS_USERNAME_TAG.tag_text_color` | Profile tag text color. | None | No | `apps/explorer/configs/app/features/addressProfileAPI.ts` |
| `.env` / `NEXT_PUBLIC_AD_ADBUTLER_CONFIG_DESKTOP.height` | Ad placement height; required for AdButler. | None | No | `apps/explorer/deploy/tools/envs-validator/schemas/features/ads.ts` |
| `.env` / `NEXT_PUBLIC_AD_ADBUTLER_CONFIG_DESKTOP.id` | Public ad placement identifier; required for AdButler. | None | No | `apps/explorer/deploy/tools/envs-validator/schemas/features/ads.ts` |
| `.env` / `NEXT_PUBLIC_AD_ADBUTLER_CONFIG_DESKTOP.width` | Ad placement width; required for AdButler. | None | No | `apps/explorer/deploy/tools/envs-validator/schemas/features/ads.ts` |
| `.env` / `NEXT_PUBLIC_AD_ADBUTLER_CONFIG_MOBILE.height` | Ad placement height; required for AdButler. | None | No | `apps/explorer/deploy/tools/envs-validator/schemas/features/ads.ts` |
| `.env` / `NEXT_PUBLIC_AD_ADBUTLER_CONFIG_MOBILE.id` | Public ad placement identifier; required for AdButler. | None | No | `apps/explorer/deploy/tools/envs-validator/schemas/features/ads.ts` |
| `.env` / `NEXT_PUBLIC_AD_ADBUTLER_CONFIG_MOBILE.width` | Ad placement width; required for AdButler. | None | No | `apps/explorer/deploy/tools/envs-validator/schemas/features/ads.ts` |
| `.env` / `NEXT_PUBLIC_BRIDGED_TOKENS_BRIDGES[].short_title` | Short bridge label; required in each configured entry. | None | No | `apps/explorer/deploy/tools/envs-validator/schemas/features/bridgedToken.ts` |
| `.env` / `NEXT_PUBLIC_BRIDGED_TOKENS_BRIDGES[].title` | Bridge label; required in each configured entry. | None | No | `apps/explorer/deploy/tools/envs-validator/schemas/features/bridgedToken.ts` |
| `.env` / `NEXT_PUBLIC_BRIDGED_TOKENS_BRIDGES[].type` | Bridge type; required in each configured entry. | None | No | `apps/explorer/deploy/tools/envs-validator/schemas/features/bridgedToken.ts` |
| `.env` / `NEXT_PUBLIC_BRIDGED_TOKENS_CHAINS[].base_url` | Chain explorer base; required in each configured entry. | None | No | `apps/explorer/deploy/tools/envs-validator/schemas/features/bridgedToken.ts` |
| `.env` / `NEXT_PUBLIC_BRIDGED_TOKENS_CHAINS[].id` | Chain identifier; required in each configured entry. | None | No | `apps/explorer/deploy/tools/envs-validator/schemas/features/bridgedToken.ts` |
| `.env` / `NEXT_PUBLIC_BRIDGED_TOKENS_CHAINS[].short_title` | Short chain label; required in each configured entry. | None | No | `apps/explorer/deploy/tools/envs-validator/schemas/features/bridgedToken.ts` |
| `.env` / `NEXT_PUBLIC_BRIDGED_TOKENS_CHAINS[].title` | Chain label; required in each configured entry. | None | No | `apps/explorer/deploy/tools/envs-validator/schemas/features/bridgedToken.ts` |
| `.env` / `NEXT_PUBLIC_COLOR_THEME_OVERRIDES.<token>` | Semantic color-token override tree. | Built-in theme tokens | No | `apps/explorer/lib/settings/colorTheme.ts` |
| `.env` / `NEXT_PUBLIC_COLOR_THEME_OVERRIDES.bg.primary._dark.value` | Dark-theme primary background override. | Built-in dark theme | No | `apps/explorer/lib/settings/colorTheme.ts` |
| `.env` / `NEXT_PUBLIC_COLOR_THEME_OVERRIDES.bg.primary._light.value` | Light-theme primary background override. | Built-in light theme | No | `apps/explorer/lib/settings/colorTheme.ts` |
| `.env` / `NEXT_PUBLIC_CONTRACT_CODE_IDES[].icon_url` | IDE icon; required in each configured entry. | None | No | `apps/explorer/deploy/tools/envs-validator/schemas/ui.ts` |
| `.env` / `NEXT_PUBLIC_CONTRACT_CODE_IDES[].title` | IDE label; required in each configured entry. | None | No | `apps/explorer/deploy/tools/envs-validator/schemas/ui.ts` |
| `.env` / `NEXT_PUBLIC_CONTRACT_CODE_IDES[].url` | IDE launch template; required in each configured entry. | None | No | `apps/explorer/deploy/tools/envs-validator/schemas/ui.ts` |
| `.env` / `NEXT_PUBLIC_DEFI_DROPDOWN_ITEMS[].dappId` | Dapp identifier; supply this or url. | None | No | `apps/explorer/deploy/tools/envs-validator/schemas/features/defiDropdown.ts` |
| `.env` / `NEXT_PUBLIC_DEFI_DROPDOWN_ITEMS[].icon` | Navigation icon name. | None | No | `apps/explorer/deploy/tools/envs-validator/schemas/features/defiDropdown.ts` |
| `.env` / `NEXT_PUBLIC_DEFI_DROPDOWN_ITEMS[].isEssentialDapp` | Marks the selected dapp as an essential dapp. | False when absent | No | `apps/explorer/deploy/tools/envs-validator/schemas/features/defiDropdown.ts` |
| `.env` / `NEXT_PUBLIC_DEFI_DROPDOWN_ITEMS[].text` | Navigation entry text; required in each configured entry. | None | No | `apps/explorer/deploy/tools/envs-validator/schemas/features/defiDropdown.ts` |
| `.env` / `NEXT_PUBLIC_DEFI_DROPDOWN_ITEMS[].url` | External target; supply this or dappId. | None | No | `apps/explorer/deploy/tools/envs-validator/schemas/features/defiDropdown.ts` |
| `.env` / `NEXT_PUBLIC_FONT_FAMILY_BODY.name` | Font family name; required when configured. | None | No | `apps/explorer/deploy/tools/envs-validator/schemas/ui.ts` |
| `.env` / `NEXT_PUBLIC_FONT_FAMILY_BODY.url` | Font stylesheet location; required when configured. | None | No | `apps/explorer/deploy/tools/envs-validator/schemas/ui.ts` |
| `.env` / `NEXT_PUBLIC_FONT_FAMILY_HEADING.name` | Font family name; required when configured. | None | No | `apps/explorer/deploy/tools/envs-validator/schemas/ui.ts` |
| `.env` / `NEXT_PUBLIC_FONT_FAMILY_HEADING.url` | Font stylesheet location; required when configured. | None | No | `apps/explorer/deploy/tools/envs-validator/schemas/ui.ts` |
| `.env` / `NEXT_PUBLIC_GAS_REFUEL_PROVIDER_CONFIG.dapp_id` | Marketplace dapp identifier. | None | No | `apps/explorer/configs/app/features/getGasButton.ts` |
| `.env` / `NEXT_PUBLIC_GAS_REFUEL_PROVIDER_CONFIG.logo` | Provider logo. | None | No | `apps/explorer/configs/app/features/getGasButton.ts` |
| `.env` / `NEXT_PUBLIC_GAS_REFUEL_PROVIDER_CONFIG.name` | Provider label; required when configured. | None | No | `apps/explorer/configs/app/features/getGasButton.ts` |
| `.env` / `NEXT_PUBLIC_GAS_REFUEL_PROVIDER_CONFIG.url_template` | Refuel link with chainId substitution; required when configured. | None | No | `apps/explorer/configs/app/features/getGasButton.ts` |
| `.env` / `NEXT_PUBLIC_HOMEPAGE_HERO_BANNER_CONFIG.background` | Light/dark hero backgrounds. | Theme styling | No | `apps/explorer/deploy/tools/envs-validator/schemas/ui.ts` |
| `.env` / `NEXT_PUBLIC_HOMEPAGE_HERO_BANNER_CONFIG.border` | Light/dark hero borders. | Theme styling | No | `apps/explorer/deploy/tools/envs-validator/schemas/ui.ts` |
| `.env` / `NEXT_PUBLIC_HOMEPAGE_HERO_BANNER_CONFIG.button._default.background` | Light/dark button backgrounds for this state. | Theme styling | No | `apps/explorer/deploy/tools/envs-validator/schemas/ui.ts` |
| `.env` / `NEXT_PUBLIC_HOMEPAGE_HERO_BANNER_CONFIG.button._default.text_color` | Light/dark button text colors for this state. | Theme styling | No | `apps/explorer/deploy/tools/envs-validator/schemas/ui.ts` |
| `.env` / `NEXT_PUBLIC_HOMEPAGE_HERO_BANNER_CONFIG.button._hover.background` | Light/dark button backgrounds for this state. | Theme styling | No | `apps/explorer/deploy/tools/envs-validator/schemas/ui.ts` |
| `.env` / `NEXT_PUBLIC_HOMEPAGE_HERO_BANNER_CONFIG.button._hover.text_color` | Light/dark button text colors for this state. | Theme styling | No | `apps/explorer/deploy/tools/envs-validator/schemas/ui.ts` |
| `.env` / `NEXT_PUBLIC_HOMEPAGE_HERO_BANNER_CONFIG.button._selected.background` | Light/dark button backgrounds for this state. | Theme styling | No | `apps/explorer/deploy/tools/envs-validator/schemas/ui.ts` |
| `.env` / `NEXT_PUBLIC_HOMEPAGE_HERO_BANNER_CONFIG.button._selected.text_color` | Light/dark button text colors for this state. | Theme styling | No | `apps/explorer/deploy/tools/envs-validator/schemas/ui.ts` |
| `.env` / `NEXT_PUBLIC_HOMEPAGE_HERO_BANNER_CONFIG.search.border_width` | Light/dark search border widths. | Theme styling | No | `apps/explorer/deploy/tools/envs-validator/schemas/ui.ts` |
| `.env` / `NEXT_PUBLIC_HOMEPAGE_HERO_BANNER_CONFIG.text_color` | Light/dark hero text colors. | Theme styling | No | `apps/explorer/deploy/tools/envs-validator/schemas/ui.ts` |
| `.env` / `NEXT_PUBLIC_MARKETPLACE_ESSENTIAL_DAPPS_CONFIG.multisend.chains` | Enabled chain ID list; at least one is required when this section is configured. | None | No | `apps/explorer/deploy/tools/essential-dapps-chains-config-generator/index.ts` |
| `.env` / `NEXT_PUBLIC_MARKETPLACE_ESSENTIAL_DAPPS_CONFIG.multisend.posthogHost` | Public analytics destination for multisend. | None | No | `apps/explorer/deploy/tools/envs-validator/schemas/features/marketplace.ts` |
| `.env` / `NEXT_PUBLIC_MARKETPLACE_ESSENTIAL_DAPPS_CONFIG.multisend.posthogKey` | Public analytics project identifier for multisend. | None | No | `apps/explorer/deploy/tools/envs-validator/schemas/features/marketplace.ts` |
| `.env` / `NEXT_PUBLIC_MARKETPLACE_ESSENTIAL_DAPPS_CONFIG.revoke.chains` | Enabled chain ID list; at least one is required when this section is configured. | None | No | `apps/explorer/deploy/tools/essential-dapps-chains-config-generator/index.ts` |
| `.env` / `NEXT_PUBLIC_MARKETPLACE_ESSENTIAL_DAPPS_CONFIG.swap.chains` | Enabled chain ID list; at least one is required when this section is configured. | None | No | `apps/explorer/deploy/tools/essential-dapps-chains-config-generator/index.ts` |
| `.env` / `NEXT_PUBLIC_MARKETPLACE_ESSENTIAL_DAPPS_CONFIG.swap.fee` | Swap provider fee; required when swap is configured. | None | No | `apps/explorer/deploy/tools/envs-validator/schemas/features/marketplace.ts` |
| `.env` / `NEXT_PUBLIC_MARKETPLACE_ESSENTIAL_DAPPS_CONFIG.swap.integrator` | Public swap integration identifier; required when swap is configured. | None | No | `apps/explorer/deploy/tools/envs-validator/schemas/features/marketplace.ts` |
| `.env` / `NEXT_PUBLIC_MARKETPLACE_ESSENTIAL_DAPPS_CONFIG.swap.url` | Swap provider location; required when swap is configured. | None | No | `apps/explorer/deploy/tools/envs-validator/schemas/features/marketplace.ts` |
| `.env` / `NEXT_PUBLIC_MARKETPLACE_TITLES.entity_name` | Singular app label. | `Dapp` | No | `apps/explorer/configs/app/features/marketplace.ts` |
| `.env` / `NEXT_PUBLIC_MARKETPLACE_TITLES.menu_item` | Navigation label. | `Dapps` | No | `apps/explorer/configs/app/features/marketplace.ts` |
| `.env` / `NEXT_PUBLIC_MARKETPLACE_TITLES.subtitle_essential_dapps` | Essential-dapps section label. | `Essential dapps` | No | `apps/explorer/configs/app/features/marketplace.ts` |
| `.env` / `NEXT_PUBLIC_MARKETPLACE_TITLES.subtitle_list` | App-list section label. | `Explore dapps` | No | `apps/explorer/configs/app/features/marketplace.ts` |
| `.env` / `NEXT_PUBLIC_MARKETPLACE_TITLES.title` | Marketplace heading. | `Dappscout` | No | `apps/explorer/configs/app/features/marketplace.ts` |
| `.env` / `NEXT_PUBLIC_MIXPANEL_CONFIG_OVERRIDES.<option>` | Mixpanel SDK option passed through to browser initialization. | SDK option default | No | `apps/explorer/lib/mixpanel/useInit.tsx` |
| `.env` / `NEXT_PUBLIC_MIXPANEL_CONFIG_OVERRIDES.debug` | Overrides browser analytics debug logging. | Derived from the debug query parameter or cookie | No | `apps/explorer/lib/mixpanel/useInit.tsx` |
| `.env` / `NEXT_PUBLIC_MIXPANEL_CONFIG_OVERRIDES.persistence` | Overrides browser analytics persistence. | `localStorage` | No | `apps/explorer/lib/mixpanel/useInit.tsx` |
| `.env` / `NEXT_PUBLIC_MULTICHAIN_BALANCE_PROVIDER_CONFIG[].dapp_id` | Marketplace dapp identifier. | None | No | `apps/explorer/configs/app/features/multichainButton.ts` |
| `.env` / `NEXT_PUBLIC_MULTICHAIN_BALANCE_PROVIDER_CONFIG[].logo` | Provider logo; required in each entry. | None | No | `apps/explorer/configs/app/features/multichainButton.ts` |
| `.env` / `NEXT_PUBLIC_MULTICHAIN_BALANCE_PROVIDER_CONFIG[].name` | Provider label; required in each entry. | None | No | `apps/explorer/configs/app/features/multichainButton.ts` |
| `.env` / `NEXT_PUBLIC_MULTICHAIN_BALANCE_PROVIDER_CONFIG[].promo` | Promotes this provider in ordering. | False when absent | No | `apps/explorer/configs/app/features/multichainButton.ts` |
| `.env` / `NEXT_PUBLIC_MULTICHAIN_BALANCE_PROVIDER_CONFIG[].url_template` | Provider link template; required in each entry. | None | No | `apps/explorer/configs/app/features/multichainButton.ts` |
| `.env` / `NEXT_PUBLIC_NAVIGATION_PROMO_BANNER_CONFIG.bg_color.dark` | Dark background for a text banner; required in the text-banner form. | None | No | `apps/explorer/deploy/tools/envs-validator/schemas/ui.ts` |
| `.env` / `NEXT_PUBLIC_NAVIGATION_PROMO_BANNER_CONFIG.bg_color.light` | Light background for a text banner; required in the text-banner form. | None | No | `apps/explorer/deploy/tools/envs-validator/schemas/ui.ts` |
| `.env` / `NEXT_PUBLIC_NAVIGATION_PROMO_BANNER_CONFIG.img_url` | Banner image string, or small/large image object in the image-only form; required. | None | No | `apps/explorer/deploy/tools/envs-validator/schemas/ui.ts` |
| `.env` / `NEXT_PUBLIC_NAVIGATION_PROMO_BANNER_CONFIG.img_url.large` | Large image in the responsive image-only form; required in that form. | None | No | `apps/explorer/deploy/tools/envs-validator/schemas/ui.ts` |
| `.env` / `NEXT_PUBLIC_NAVIGATION_PROMO_BANNER_CONFIG.img_url.small` | Small image in the responsive image-only form; required in that form. | None | No | `apps/explorer/deploy/tools/envs-validator/schemas/ui.ts` |
| `.env` / `NEXT_PUBLIC_NAVIGATION_PROMO_BANNER_CONFIG.link_url` | Banner click target; required when configured. | None | No | `apps/explorer/deploy/tools/envs-validator/schemas/ui.ts` |
| `.env` / `NEXT_PUBLIC_NAVIGATION_PROMO_BANNER_CONFIG.text` | Banner caption; required in the text-banner form. | None | No | `apps/explorer/deploy/tools/envs-validator/schemas/ui.ts` |
| `.env` / `NEXT_PUBLIC_NAVIGATION_PROMO_BANNER_CONFIG.text_color.dark` | Dark text color; required in the text-banner form. | None | No | `apps/explorer/deploy/tools/envs-validator/schemas/ui.ts` |
| `.env` / `NEXT_PUBLIC_NAVIGATION_PROMO_BANNER_CONFIG.text_color.light` | Light text color; required in the text-banner form. | None | No | `apps/explorer/deploy/tools/envs-validator/schemas/ui.ts` |
| `.env` / `NEXT_PUBLIC_NETWORK_ADDITIONAL_TOKEN_TYPES[].id` | Token type identifier; required in each configured entry. | None | No | `apps/explorer/deploy/tools/envs-validator/schemas/chain.ts` |
| `.env` / `NEXT_PUBLIC_NETWORK_ADDITIONAL_TOKEN_TYPES[].name` | Token type label; required in each configured entry. | None | No | `apps/explorer/deploy/tools/envs-validator/schemas/chain.ts` |
| `.env` / `NEXT_PUBLIC_NETWORK_EXPLORERS[].baseUrl` | Explorer origin; required in each entry. | None | No | `apps/explorer/deploy/tools/envs-validator/schemas/ui.ts` |
| `.env` / `NEXT_PUBLIC_NETWORK_EXPLORERS[].logo` | Explorer logo. | None | No | `apps/explorer/deploy/tools/envs-validator/schemas/ui.ts` |
| `.env` / `NEXT_PUBLIC_NETWORK_EXPLORERS[].paths.address` | Address path prefix. | None | No | `apps/explorer/deploy/tools/envs-validator/schemas/ui.ts` |
| `.env` / `NEXT_PUBLIC_NETWORK_EXPLORERS[].paths.blob` | Blob path prefix. | None | No | `apps/explorer/deploy/tools/envs-validator/schemas/ui.ts` |
| `.env` / `NEXT_PUBLIC_NETWORK_EXPLORERS[].paths.block` | Block path prefix. | None | No | `apps/explorer/deploy/tools/envs-validator/schemas/ui.ts` |
| `.env` / `NEXT_PUBLIC_NETWORK_EXPLORERS[].paths.token` | Token path prefix. | None | No | `apps/explorer/deploy/tools/envs-validator/schemas/ui.ts` |
| `.env` / `NEXT_PUBLIC_NETWORK_EXPLORERS[].paths.tx` | Transaction path prefix. | None | No | `apps/explorer/deploy/tools/envs-validator/schemas/ui.ts` |
| `.env` / `NEXT_PUBLIC_NETWORK_EXPLORERS[].title` | Explorer label; required in each entry. | None | No | `apps/explorer/deploy/tools/envs-validator/schemas/ui.ts` |
| `.env` / `NEXT_PUBLIC_OTHER_LINKS[].text` | Navigation link label; required in each configured entry. | None | No | `apps/explorer/deploy/tools/envs-validator/schemas/ui.ts` |
| `.env` / `NEXT_PUBLIC_OTHER_LINKS[].url` | Navigation target; required in each configured entry. | None | No | `apps/explorer/deploy/tools/envs-validator/schemas/ui.ts` |
| `.env` / `NEXT_PUBLIC_ROLLUP_PARENT_CHAIN.baseUrl` | Parent explorer base; required when configuring a rollup. | None | No | `apps/explorer/lib/web3/chains.ts` |
| `.env` / `NEXT_PUBLIC_ROLLUP_PARENT_CHAIN.currency.decimals` | Parent currency decimal places; required if currency is supplied. | None | No | `apps/explorer/lib/web3/chains.ts` |
| `.env` / `NEXT_PUBLIC_ROLLUP_PARENT_CHAIN.currency.name` | Parent currency name; required if currency is supplied. | None | No | `apps/explorer/lib/web3/chains.ts` |
| `.env` / `NEXT_PUBLIC_ROLLUP_PARENT_CHAIN.currency.symbol` | Parent currency ticker; required if currency is supplied. | None | No | `apps/explorer/lib/web3/chains.ts` |
| `.env` / `NEXT_PUBLIC_ROLLUP_PARENT_CHAIN.id` | Parent EVM chain ID; needed for the parent wallet client. | None | No | `apps/explorer/lib/web3/chains.ts` |
| `.env` / `NEXT_PUBLIC_ROLLUP_PARENT_CHAIN.isTestnet` | Parent wallet test-network flag. | None | No | `apps/explorer/lib/web3/chains.ts` |
| `.env` / `NEXT_PUBLIC_ROLLUP_PARENT_CHAIN.name` | Parent chain label; needed for the parent wallet client. | None | No | `apps/explorer/lib/web3/chains.ts` |
| `.env` / `NEXT_PUBLIC_ROLLUP_PARENT_CHAIN.rpcUrls` | Parent node URL list; needed for the parent wallet client. | None | No | `apps/explorer/lib/web3/chains.ts` |
| `.env` / `NEXT_PUBLIC_TX_EXTERNAL_TRANSACTIONS_CONFIG.chain_logo_url` | External chain logo; required when configured. | None | No | `apps/explorer/configs/app/features/externalTxs.ts` |
| `.env` / `NEXT_PUBLIC_TX_EXTERNAL_TRANSACTIONS_CONFIG.chain_name` | External chain label; required when configured. | None | No | `apps/explorer/configs/app/features/externalTxs.ts` |
| `.env` / `NEXT_PUBLIC_TX_EXTERNAL_TRANSACTIONS_CONFIG.explorer_url_template` | External explorer template; required when configured. | None | No | `apps/explorer/configs/app/features/externalTxs.ts` |
| `.env` / `NEXT_PUBLIC_VIEWS_NFT_MARKETPLACES[].collection_url` | Collection link template; supply this or instance_url. | None | No | `apps/explorer/deploy/tools/envs-validator/schemas/ui.ts` |
| `.env` / `NEXT_PUBLIC_VIEWS_NFT_MARKETPLACES[].instance_url` | Token-instance link template; supply this or collection_url. | None | No | `apps/explorer/deploy/tools/envs-validator/schemas/ui.ts` |
| `.env` / `NEXT_PUBLIC_VIEWS_NFT_MARKETPLACES[].logo_url` | Marketplace logo; required in each entry. | None | No | `apps/explorer/deploy/tools/envs-validator/schemas/ui.ts` |
| `.env` / `NEXT_PUBLIC_VIEWS_NFT_MARKETPLACES[].name` | Marketplace label; required in each entry. | None | No | `apps/explorer/deploy/tools/envs-validator/schemas/ui.ts` |
| `.env` / `NEXT_PUBLIC_ZETACHAIN_EXTERNAL_SEARCH_CONFIG[].name` | Search provider label; required in each configured entry. | None | No | `apps/explorer/deploy/tools/envs-validator/schemas/features/zetaChain.ts` |
| `.env` / `NEXT_PUBLIC_ZETACHAIN_EXTERNAL_SEARCH_CONFIG[].regex` | Input matching expression; required in each configured entry. | None | No | `apps/explorer/deploy/tools/envs-validator/schemas/features/zetaChain.ts` |
| `.env` / `NEXT_PUBLIC_ZETACHAIN_EXTERNAL_SEARCH_CONFIG[].template` | Search destination template; required in each configured entry. | None | No | `apps/explorer/deploy/tools/envs-validator/schemas/features/zetaChain.ts` |
| `BUNDLE_ANALYZER` | Enables the bundle analyzer during the Next.js build. | `false` | No | `apps/explorer/next.config.js` |
| `DISABLE_TRACKING` | Suppresses server-side tracking; private browsing mode also suppresses it. | `false` | No | `apps/explorer/nextjs/getServerSideProps/handlers.ts` |
| `ENVS_PRESET` | Selects configs/envs/.env.<preset> for container startup; empty or none leaves the supplied environment in place. | Unset; supplied environment is retained | No | `apps/explorer/deploy/scripts/entrypoint.sh` |
| `EXPLORER_ASSISTANT_API_KEY` | Bearer credential for the assistant upstream, when that upstream requires authentication; server only. | None | Yes; server/deployment only | `apps/explorer/pages/api/assistant.ts` |
| `EXPLORER_ASSISTANT_BASE_URL` | HTTP(S) assistant API base used to append /chat/completions; required to serve assistant requests. | None | No | `apps/explorer/pages/api/assistant.ts` |
| `EXPLORER_ASSISTANT_MODEL` | Model identifier sent by the assistant route. | `xai/grok-4.7` | No | `apps/explorer/pages/api/assistant.ts` |
| `EXPLORER_INFERENCE_NODE_URL` | HTTP(S) root URL used by the inference route; requires the root path with no credentials, query or fragment. | None | No | `apps/explorer/pages/api/inference.ts` |
| `FAVICON_MASTER_URL` | Overrides the favicon master image; passed to MASTER_URL. | Value of NEXT_PUBLIC_NETWORK_ICON | No | `apps/explorer/deploy/scripts/favicon_generator.sh` |
| `HOSTNAME` | Container identity in the Docker-ID response header and host used by edge configuration loaders. | Runtime supplied; header fallback is an empty string | No | `apps/explorer/configs/essential-dapps-chains/config.edge.ts`; `apps/explorer/configs/multichain/config.edge.ts`; `apps/explorer/middleware.ts` |
| `MASTER_URL` | Master image URL read by the favicon generator; required for a direct generator invocation and supplied by its wrapper. | No direct default; wrapper supplies FAVICON_MASTER_URL or NEXT_PUBLIC_NETWORK_ICON | No | `apps/explorer/deploy/tools/favicon-generator/index.js` |
| `NEXT_OPEN_TELEMETRY_ENABLED` | Imports Node telemetry during Next.js instrumentation registration. | `false` | No | `apps/explorer/instrumentation.ts` |
| `NEXT_PUBLIC_ACCOUNT_API_KEYS_BUTTON` | Shows, hides or replaces the account API-keys button with a link; configure only with account support. | `true` | No | `apps/explorer/configs/app/features/account.ts` |
| `NEXT_PUBLIC_ACCOUNT_AUTH_PROVIDER` | Chooses dynamic authentication or the reCAPTCHA-backed auth0 path when account support is enabled. | No value; auth0 path if a reCAPTCHA site identifier is supplied | No | `apps/explorer/configs/app/features/account.ts` |
| `NEXT_PUBLIC_ACCOUNT_DYNAMIC_ENVIRONMENT_ID` | Public Dynamic environment identifier; required when the account auth provider is dynamic. | None | No | `apps/explorer/configs/app/features/account.ts` |
| `NEXT_PUBLIC_ADDRESS_3RD_PARTY_WIDGETS` | JSON list of enabled widget names; requires the widget configuration asset. | None | No | `apps/explorer/configs/app/features/address3rdPartyWidgets.ts` |
| `NEXT_PUBLIC_ADDRESS_3RD_PARTY_WIDGETS_CONFIG_URL` | JSON object or asset location for address widgets, indexed by widget name. | None | No | `apps/explorer/configs/app/features/address3rdPartyWidgets.ts` |
| `NEXT_PUBLIC_ADDRESS_USERNAME_TAG` | JSON profile API configuration: api_url_template and optional tag_link_template, tag_icon, tag_bg_color, tag_text_color. | None | No | `apps/explorer/configs/app/features/addressProfileAPI.ts` |
| `NEXT_PUBLIC_ADMIN_SERVICE_API_HOST` | Full base URL for the administration service and marketplace API. | None | No | `apps/explorer/configs/app/apis.ts`; `apps/explorer/deploy/tools/sitemap-generator/next-sitemap.config.js` |
| `NEXT_PUBLIC_ADVANCED_FILTER_ENABLED` | Enables the advanced filter unless explicitly false; disable in multichain mode. | `true` | No | `apps/explorer/configs/app/features/advancedFilter.ts`; `apps/explorer/deploy/tools/sitemap-generator/next-sitemap.config.js` |
| `NEXT_PUBLIC_AD_ADBUTLER_CONFIG_DESKTOP` | Desktop AdButler JSON with id, width and height; required for the AdButler provider. | None | No | `apps/explorer/configs/app/features/adsBanner.ts` |
| `NEXT_PUBLIC_AD_ADBUTLER_CONFIG_MOBILE` | Mobile AdButler JSON with id, width and height; required for the AdButler provider. | None | No | `apps/explorer/configs/app/features/adsBanner.ts` |
| `NEXT_PUBLIC_AD_BANNER_ADDITIONAL_PROVIDER` | Adds an additional banner advertising provider. | None | No | `apps/explorer/configs/app/features/adsBanner.ts` |
| `NEXT_PUBLIC_AD_BANNER_ENABLE_SPECIFY` | Enables Specify banner advertising. | `false` | No | `apps/explorer/configs/app/features/adsBanner.ts` |
| `NEXT_PUBLIC_AD_BANNER_PROVIDER` | Selects the banner advertising provider; none disables banner advertising. | `slise` | No | `apps/explorer/configs/app/features/adsBanner.ts` |
| `NEXT_PUBLIC_AD_TEXT_PROVIDER` | Selects the text advertising provider; none disables text advertising. | `coinzilla` | No | `apps/explorer/configs/app/features/adsText.ts` |
| `NEXT_PUBLIC_API_BASE_PATH` | Path prefix added to the explorer backend API URL; trailing slash is removed. | Empty string | No | `apps/explorer/configs/app/apis.ts`; `apps/explorer/deploy/tools/sitemap-generator/next-sitemap.config.js` |
| `NEXT_PUBLIC_API_DOCS_ALERT_MESSAGE` | Alert displayed on the API documentation page. | None | No | `apps/explorer/configs/app/features/apiDocs.ts` |
| `NEXT_PUBLIC_API_DOCS_TABS` | JSON list of API documentation tab identifiers; an empty list disables the page. | `["rest_api","eth_rpc_api","rpc_api","graphql_api"]` | No | `apps/explorer/configs/app/features/apiDocs.ts`; `apps/explorer/deploy/tools/sitemap-generator/next-sitemap.config.js` |
| `NEXT_PUBLIC_API_KEYS_ALERT_MESSAGE` | Message displayed in the API keys alert. | None | No | `apps/explorer/configs/app/ui.ts` |
| `NEXT_PUBLIC_API_PORT` | Explicit backend API and socket port; otherwise the protocol's standard port applies. | None | No | `apps/explorer/configs/app/apis.ts`; `apps/explorer/deploy/tools/sitemap-generator/next-sitemap.config.js` |
| `NEXT_PUBLIC_API_PROTOCOL` | Backend API URL scheme. | `https` | No | `apps/explorer/configs/app/apis.ts`; `apps/explorer/deploy/tools/sitemap-generator/next-sitemap.config.js` |
| `NEXT_PUBLIC_API_SPEC_URL` | Overrides the core API documentation specification location. | None | No | `apps/explorer/configs/app/features/apiDocs.ts` |
| `NEXT_PUBLIC_API_WEBSOCKET_PROTOCOL` | Backend WebSocket URL scheme. | `wss` | No | `apps/explorer/configs/app/apis.ts` |
| `NEXT_PUBLIC_APP_ENV` | Selects development or review behavior and supplies the Rollbar environment. | Unset in app configuration; Rollbar uses production | No | `apps/explorer/configs/app/app.ts`; `apps/explorer/configs/app/features/rollbar.ts` |
| `NEXT_PUBLIC_APP_INSTANCE` | Instance label for Rollbar and telemetry; pw also enables browser storage overrides. | Derived from the app host; telemetry finally uses unknown_app | No | `apps/explorer/configs/app/app.ts`; `apps/explorer/configs/app/features/rollbar.ts`; `apps/explorer/configs/app/utils.ts`; `apps/explorer/instrumentation.node.ts` |
| `NEXT_PUBLIC_APP_PORT` | Frontend port; also passed to next dev by the development scripts. | Unset; edge self-fetch uses 3000 | No | `apps/explorer/configs/app/app.ts`; `apps/explorer/deploy/tools/sitemap-generator/next-sitemap.config.js` |
| `NEXT_PUBLIC_APP_PROTOCOL` | Frontend URL scheme and secure-cookie selection. | `https` | No | `apps/explorer/configs/app/app.ts`; `apps/explorer/deploy/tools/sitemap-generator/next-sitemap.config.js`; `apps/explorer/lib/cookies.ts` |
| `NEXT_PUBLIC_ASSISTANT_ENABLED` | Shows the explorer assistant interface. | `false` | No | `apps/explorer/configs/app/features/assistant.ts` |
| `NEXT_PUBLIC_BEACON_CHAIN_CURRENCY_SYMBOL` | Currency symbol for beacon-chain deposits and withdrawals. | NEXT_PUBLIC_NETWORK_CURRENCY_SYMBOL, then empty string | No | `apps/explorer/configs/app/features/beaconChain.ts` |
| `NEXT_PUBLIC_BEACON_CHAIN_VALIDATOR_URL_TEMPLATE` | Link template for beacon validators; used only with beacon-chain support. | None | No | `apps/explorer/configs/app/features/beaconChain.ts` |
| `NEXT_PUBLIC_BEACON_CHAIN_WITHDRAWALS_ONLY` | Limits beacon-chain UI to withdrawals. | `false` | No | `apps/explorer/configs/app/features/beaconChain.ts`; `apps/explorer/deploy/tools/sitemap-generator/next-sitemap.config.js` |
| `NEXT_PUBLIC_BRIDGED_TOKENS_BRIDGES` | JSON bridge records with type, title and short_title; required when bridged-token chains are configured. | None | No | `apps/explorer/configs/app/features/bridgedTokens.ts` |
| `NEXT_PUBLIC_BRIDGED_TOKENS_CHAINS` | JSON bridged-chain records with id, title, short_title and base_url. | None | No | `apps/explorer/configs/app/features/bridgedTokens.ts` |
| `NEXT_PUBLIC_CELO_ENABLED` | Enables the Celo-specific UI. | `false` | No | `apps/explorer/configs/app/features/celo.ts`; `apps/explorer/deploy/tools/sitemap-generator/next-sitemap.config.js` |
| `NEXT_PUBLIC_CLUSTERS_API_HOST` | Full base URL for the Clusters name service. | None | No | `apps/explorer/configs/app/apis.ts`; `apps/explorer/deploy/tools/sitemap-generator/next-sitemap.config.js` |
| `NEXT_PUBLIC_CLUSTERS_CDN_URL` | Overrides the Clusters CDN origin; configure only with the Clusters API. | Built-in Clusters CDN origin | No | `apps/explorer/configs/app/features/nameServices.ts` |
| `NEXT_PUBLIC_COLOR_THEME_DEFAULT` | Preferred color-theme identifier; must match a supported theme. | No configured theme; color-mode default is selected by lib/settings/colorTheme.ts | No | `apps/explorer/configs/app/ui.ts` |
| `NEXT_PUBLIC_COLOR_THEME_OVERRIDES` | JSON semantic color-token overrides, including bg.primary._light.value and bg.primary._dark.value. | `{}` | No | `apps/explorer/lib/settings/colorTheme.ts` |
| `NEXT_PUBLIC_CONTRACT_CODE_IDES` | JSON list of contract IDE links with title, url and icon_url. | `[]` | No | `apps/explorer/configs/app/ui.ts` |
| `NEXT_PUBLIC_CONTRACT_INFO_API_HOST` | Full base URL for contract information; needed by the DEX pools feature. | None | No | `apps/explorer/configs/app/apis.ts` |
| `NEXT_PUBLIC_CROSS_CHAIN_TXS_ENABLED` | Enables cross-chain transactions when the interchain indexer is configured. | `false` | No | `apps/explorer/configs/app/features/crossChainTxs.ts` |
| `NEXT_PUBLIC_DATA_AVAILABILITY_ENABLED` | Enables blob and data-availability UI and sitemap entries. | `false` | No | `apps/explorer/configs/app/features/dataAvailability.ts`; `apps/explorer/deploy/tools/sitemap-generator/next-sitemap.config.js` |
| `NEXT_PUBLIC_DEFI_DROPDOWN_ITEMS` | JSON DeFi navigation entries with text, optional icon and either url or dappId; dapp entries can set isEssentialDapp. | `[]` | No | `apps/explorer/configs/app/features/deFiDropdown.ts` |
| `NEXT_PUBLIC_DEX_POOLS_ENABLED` | Enables DEX pools with the contract-information service. | `false` | No | `apps/explorer/configs/app/features/pools.ts`; `apps/explorer/deploy/tools/sitemap-generator/next-sitemap.config.js` |
| `NEXT_PUBLIC_FAULT_PROOF_ENABLED` | Enables fault-proof UI for an optimistic rollup. | `false` | No | `apps/explorer/configs/app/features/faultProofSystem.ts` |
| `NEXT_PUBLIC_FEATURED_NETWORKS` | JSON array or asset location for the network menu. | None | No | `apps/explorer/configs/app/ui.ts` |
| `NEXT_PUBLIC_FEATURED_NETWORKS_ALL_LINK` | Link to the complete network list; configure only with featured networks. | None | No | `apps/explorer/configs/app/ui.ts` |
| `NEXT_PUBLIC_FEATURED_NETWORKS_MODE` | Network menu presentation; configure only with featured networks. | `list` | No | `apps/explorer/configs/app/ui.ts` |
| `NEXT_PUBLIC_FLASHBLOCKS_SOCKET_URL` | Flashblocks socket URL; a configured MegaETH RPC socket takes precedence. | None | No | `apps/explorer/configs/app/features/flashblocks.ts` |
| `NEXT_PUBLIC_FONT_FAMILY_BODY` | JSON body-font descriptor with name and url. | None | No | `apps/explorer/configs/app/ui.ts` |
| `NEXT_PUBLIC_FONT_FAMILY_HEADING` | JSON heading-font descriptor with name and url. | None | No | `apps/explorer/configs/app/ui.ts` |
| `NEXT_PUBLIC_FOOTER_LINKS` | JSON array or asset location for custom footer link groups. | None | No | `apps/explorer/configs/app/ui.ts` |
| `NEXT_PUBLIC_GAME_BADGE_CLAIM_LINK` | Claim link for the easter-egg badge. | None | No | `apps/explorer/configs/app/features/easterEggBadge.ts` |
| `NEXT_PUBLIC_GAS_REFUEL_PROVIDER_CONFIG` | JSON gas-refuel provider with name, url_template and optional logo, dapp_id. | None | No | `apps/explorer/configs/app/features/getGasButton.ts` |
| `NEXT_PUBLIC_GAS_TRACKER_ENABLED` | Enables the gas tracker unless explicitly false; disable in multichain mode. | `true` | No | `apps/explorer/configs/app/features/gasTracker.ts`; `apps/explorer/deploy/tools/sitemap-generator/next-sitemap.config.js` |
| `NEXT_PUBLIC_GAS_TRACKER_UNITS` | JSON list of displayed gas-price units. | `["usd","gwei"]`; `["gwei"]` on a test network | No | `apps/explorer/configs/app/features/gasTracker.ts` |
| `NEXT_PUBLIC_GIT_COMMIT_SHA` | Frontend commit label, Rollbar code version and telemetry version fallback; supplied by development scripts. | No value; telemetry finally uses unknown_version | No | `apps/explorer/configs/app/features/rollbar.ts`; `apps/explorer/configs/app/ui.ts`; `apps/explorer/instrumentation.node.ts` |
| `NEXT_PUBLIC_GIT_TAG` | Frontend release label and preferred Rollbar and telemetry version; supplied by development scripts. | No value; version consumers try NEXT_PUBLIC_GIT_COMMIT_SHA | No | `apps/explorer/configs/app/features/rollbar.ts`; `apps/explorer/configs/app/ui.ts`; `apps/explorer/instrumentation.node.ts` |
| `NEXT_PUBLIC_GOOGLE_ANALYTICS_PROPERTY_ID` | Public Google Analytics property identifier; enables analytics outside private mode. | None | No | `apps/explorer/configs/app/features/googleAnalytics.ts` |
| `NEXT_PUBLIC_GROWTH_BOOK_CLIENT_KEY` | Public GrowthBook client identifier; enables feature experiments outside private mode. | None | No | `apps/explorer/configs/app/features/growthBook.ts` |
| `NEXT_PUBLIC_HAS_BEACON_CHAIN` | Enables beacon-chain deposits and withdrawals. | `false` | No | `apps/explorer/configs/app/features/beaconChain.ts`; `apps/explorer/deploy/tools/sitemap-generator/next-sitemap.config.js` |
| `NEXT_PUBLIC_HAS_CONTRACT_AUDIT_REPORTS` | Shows contract audit reports. | `false` | No | `apps/explorer/configs/app/ui.ts` |
| `NEXT_PUBLIC_HAS_MUD_FRAMEWORK` | Enables MUD UI for an optimistic rollup. | `false` | No | `apps/explorer/configs/app/features/mudFramework.ts`; `apps/explorer/deploy/tools/sitemap-generator/next-sitemap.config.js` |
| `NEXT_PUBLIC_HAS_USER_OPS` | Enables user-operation UI; configure its indexer API for the corresponding requests. | `false` | No | `apps/explorer/configs/app/features/userOps.ts`; `apps/explorer/deploy/tools/sitemap-generator/next-sitemap.config.js` |
| `NEXT_PUBLIC_HELIA_VERIFIED_FETCH_ENABLED` | Enables verified fetching of NFT assets unless explicitly false. | `true` | No | `apps/explorer/configs/app/ui/views/nft.ts` |
| `NEXT_PUBLIC_HIDE_INDEXING_ALERT_BLOCKS` | Hides the block-indexing alert. | `false` | No | `apps/explorer/configs/app/ui.ts` |
| `NEXT_PUBLIC_HIDE_INDEXING_ALERT_INT_TXS` | Hides the internal-transaction indexing alert. | `false` | No | `apps/explorer/configs/app/ui.ts` |
| `NEXT_PUBLIC_HIDE_NATIVE_COIN_PRICE` | Hides the native coin price. | `false` | No | `apps/explorer/configs/app/ui.ts` |
| `NEXT_PUBLIC_HOMEPAGE_CHARTS` | JSON list of homepage charts; daily_operational_txs requires the stats service. | `[]` | No | `apps/explorer/configs/app/ui.ts` |
| `NEXT_PUBLIC_HOMEPAGE_HERO_BANNER_CONFIG` | JSON light/dark hero styling for background, text_color, border, button states and search border_width. | None | No | `apps/explorer/configs/app/ui.ts`; `apps/explorer/deploy/scripts/og_image_generator.js` |
| `NEXT_PUBLIC_HOMEPAGE_HIGHLIGHTS_CONFIG` | JSON array or asset location for homepage promotional banners; the validator requires at least two when configured. | None | No | `apps/explorer/configs/app/ui.ts` |
| `NEXT_PUBLIC_HOMEPAGE_STATS` | JSON list of homepage counters; total_operational_txs requires the stats service. | `["total_blocks","average_block_time","total_txs","wallet_addresses","gas_tracker"]`; qualifying rollups replace total_blocks with latest_batch | No | `apps/explorer/configs/app/ui.ts` |
| `NEXT_PUBLIC_HOT_CONTRACTS_ENABLED` | Enables hot-contract listings. | `false` | No | `apps/explorer/configs/app/features/hotContracts.ts` |
| `NEXT_PUBLIC_ICON_SPRITE_HASH` | Sprite filename hash generated by the sprite script and injected by development startup. | None | No | `apps/explorer/configs/app/app.ts` |
| `NEXT_PUBLIC_INTERCHAIN_INDEXER_API_HOST` | Full base URL for the cross-chain transaction indexer; configure with cross-chain transaction support. | None | No | `apps/explorer/configs/app/apis.ts` |
| `NEXT_PUBLIC_INTEROP_ENABLED` | Enables interop message UI for an optimistic rollup. | `false` | No | `apps/explorer/configs/app/features/rollup.ts`; `apps/explorer/deploy/tools/sitemap-generator/next-sitemap.config.js` |
| `NEXT_PUBLIC_IS_ACCOUNT_SUPPORTED` | Enables account UI when Dynamic or reCAPTCHA authentication is configured and private mode is off. | `false` | No | `apps/explorer/configs/app/features/account.ts` |
| `NEXT_PUBLIC_IS_SUAVE_CHAIN` | Enables SUAVE-specific UI. | `false` | No | `apps/explorer/configs/app/features/suave.ts` |
| `NEXT_PUBLIC_IS_TESTNET` | Marks the displayed chain as a test network. | `false` | No | `apps/explorer/configs/app/chain.ts` |
| `NEXT_PUBLIC_MAINTENANCE_ALERT_MESSAGE` | Text displayed in the maintenance alert. | None | No | `apps/explorer/configs/app/ui.ts` |
| `NEXT_PUBLIC_MARKETPLACE_BANNER_CONTENT_URL` | Location of HTML content embedded by the marketplace banner. | None | No | `apps/explorer/configs/app/features/marketplace.ts` |
| `NEXT_PUBLIC_MARKETPLACE_BANNER_LINK_URL` | Click target paired with the marketplace banner content. | None | No | `apps/explorer/configs/app/features/marketplace.ts` |
| `NEXT_PUBLIC_MARKETPLACE_CATEGORIES_URL` | JSON array or asset location listing marketplace categories. | None | No | `apps/explorer/configs/app/features/marketplace.ts` |
| `NEXT_PUBLIC_MARKETPLACE_CONFIG_URL` | JSON array or asset location listing marketplace apps; the admin API can supply apps instead. | None | No | `apps/explorer/configs/app/features/marketplace.ts` |
| `NEXT_PUBLIC_MARKETPLACE_ENABLED` | Enables marketplace UI with RPC URLs, a submit form and either an apps asset or the admin API. | `false` | No | `apps/explorer/configs/app/features/marketplace.ts`; `apps/explorer/deploy/tools/sitemap-generator/next-sitemap.config.js` |
| `NEXT_PUBLIC_MARKETPLACE_ESSENTIAL_DAPPS_AD_ENABLED` | Shows advertising beside essential dapps unless explicitly false; configure with marketplace support. | `true` | No | `apps/explorer/configs/app/features/marketplace.ts` |
| `NEXT_PUBLIC_MARKETPLACE_ESSENTIAL_DAPPS_CONFIG` | JSON swap, revoke and multisend sections, with their chains and provider settings; requires blockchain interaction. | None | No | `apps/explorer/configs/app/features/marketplace.ts`; `apps/explorer/deploy/tools/essential-dapps-chains-config-generator/index.ts` |
| `NEXT_PUBLIC_MARKETPLACE_FEATURED_APP` | Identifier of the featured marketplace app. | None | No | `apps/explorer/configs/app/features/marketplace.ts` |
| `NEXT_PUBLIC_MARKETPLACE_GRAPH_LINKS_URL` | JSON object or asset location mapping app identifiers to title/url link arrays. | None | No | `apps/explorer/configs/app/features/marketplace.ts` |
| `NEXT_PUBLIC_MARKETPLACE_SUBMIT_FORM` | Submit-form URL; required when marketplace support is enabled. | None | No | `apps/explorer/configs/app/features/marketplace.ts` |
| `NEXT_PUBLIC_MARKETPLACE_SUGGEST_IDEAS_FORM` | Ideas-form URL used by the marketplace. | None | No | `apps/explorer/configs/app/features/marketplace.ts` |
| `NEXT_PUBLIC_MARKETPLACE_TITLES` | JSON overrides for entity_name, menu_item, title, subtitle_essential_dapps and subtitle_list. | JSON override object {}; individual labels retain the built-in titles unless overridden | No | `apps/explorer/configs/app/features/marketplace.ts` |
| `NEXT_PUBLIC_MAX_CONTENT_WIDTH_ENABLED` | Constrains content width unless explicitly false. | `true` | No | `apps/explorer/configs/app/ui.ts` |
| `NEXT_PUBLIC_MEGA_ETH_SOCKET_URL_METRICS` | Socket URL for MegaETH uptime metrics. | None | No | `apps/explorer/configs/app/features/megaEth.ts`; `apps/explorer/deploy/tools/sitemap-generator/next-sitemap.config.js` |
| `NEXT_PUBLIC_MEGA_ETH_SOCKET_URL_RPC` | Socket URL for MegaETH RPC and mini-block updates. | None | No | `apps/explorer/configs/app/features/megaEth.ts` |
| `NEXT_PUBLIC_METADATA_ADDRESS_TAGS_UPDATE_ENABLED` | Allows address-tag updates unless explicitly false; configure only with the metadata API. | `true` | No | `apps/explorer/configs/app/features/addressMetadata.ts` |
| `NEXT_PUBLIC_METADATA_SERVICE_API_HOST` | Full base URL for address metadata and public tags. | None | No | `apps/explorer/configs/app/apis.ts`; `apps/explorer/deploy/tools/sitemap-generator/next-sitemap.config.js` |
| `NEXT_PUBLIC_METASUITES_ENABLED` | Enables MetaSuites integration. | `false` | No | `apps/explorer/configs/app/features/metasuites.ts` |
| `NEXT_PUBLIC_MIXPANEL_CONFIG_OVERRIDES` | JSON Mixpanel SDK options; requires a Mixpanel project token. | None | No | `apps/explorer/configs/app/features/mixpanel.ts` |
| `NEXT_PUBLIC_MIXPANEL_PROJECT_TOKEN` | Public Mixpanel project identifier; enables analytics outside private mode. | None | No | `apps/explorer/configs/app/features/mixpanel.ts` |
| `NEXT_PUBLIC_MULTICHAIN_AGGREGATOR_API_HOST` | Full base URL for the multichain aggregator; required for multichain mode and its config generator. | None | No | `apps/explorer/configs/app/apis.ts`; `apps/explorer/deploy/tools/multichain-config-generator/index.ts` |
| `NEXT_PUBLIC_MULTICHAIN_BALANCE_PROVIDER_CONFIG` | JSON balance-provider records with name, logo, url_template and optional dapp_id, promo. | None | No | `apps/explorer/configs/app/features/multichainButton.ts` |
| `NEXT_PUBLIC_MULTICHAIN_CLUSTER` | Cluster identifier used by the multichain aggregator and generator. | None | No | `apps/explorer/configs/app/apis.ts`; `apps/explorer/configs/app/features/multichain.ts`; `apps/explorer/deploy/tools/multichain-config-generator/index.ts` |
| `NEXT_PUBLIC_MULTICHAIN_ENABLED` | Enables multichain UI with a cluster, aggregator API and multichain stats API. | `false` | No | `apps/explorer/configs/app/features/multichain.ts`; `apps/explorer/deploy/tools/sitemap-generator/next-sitemap.config.js` |
| `NEXT_PUBLIC_MULTICHAIN_STATS_API_HOST` | Full base URL for multichain statistics; required for multichain mode. | None | No | `apps/explorer/configs/app/apis.ts` |
| `NEXT_PUBLIC_NAME_SERVICE_API_HOST` | Full base URL for the ENS-compatible name service. | None | No | `apps/explorer/configs/app/apis.ts`; `apps/explorer/deploy/tools/sitemap-generator/next-sitemap.config.js` |
| `NEXT_PUBLIC_NAME_SERVICE_PROTOCOLS` | JSON list of ENS-compatible protocols; configure only with the name-service API. | `["ens"]` | No | `apps/explorer/configs/app/features/nameServices.ts` |
| `NEXT_PUBLIC_NAVIGATION_HIGHLIGHTED_ROUTES` | JSON list of routes highlighted in navigation. | `[]` | No | `apps/explorer/configs/app/ui.ts` |
| `NEXT_PUBLIC_NAVIGATION_LAYOUT` | Navigation layout selected by resolveNavigationLayout. | `horizontal` | No | `apps/explorer/configs/app/ui.ts` |
| `NEXT_PUBLIC_NAVIGATION_PROMO_BANNER_CONFIG` | JSON navigation banner with img_url and link_url, plus optional text, bg_color and text_color. | None | No | `apps/explorer/configs/app/ui.ts` |
| `NEXT_PUBLIC_NETWORK_ADDITIONAL_TOKEN_TYPES` | JSON token-type records with id and name. | `[]` | No | `apps/explorer/configs/app/chain.ts` |
| `NEXT_PUBLIC_NETWORK_CURRENCY_DECIMALS` | Number of decimal places in the native currency; zero also selects the fallback. | `18` | No | `apps/explorer/configs/app/chain.ts` |
| `NEXT_PUBLIC_NETWORK_CURRENCY_GWEI_NAME` | Display name for the gwei denomination. | None | No | `apps/explorer/configs/app/chain.ts` |
| `NEXT_PUBLIC_NETWORK_CURRENCY_NAME` | Native currency display name; needed for single-chain wallet interaction. | None | No | `apps/explorer/configs/app/chain.ts` |
| `NEXT_PUBLIC_NETWORK_CURRENCY_SYMBOL` | Native currency ticker; needed for single-chain wallet interaction. | None | No | `apps/explorer/configs/app/chain.ts`; `apps/explorer/configs/app/features/beaconChain.ts` |
| `NEXT_PUBLIC_NETWORK_CURRENCY_WEI_NAME` | Display name for the wei denomination. | None | No | `apps/explorer/configs/app/chain.ts` |
| `NEXT_PUBLIC_NETWORK_EXPLORERS` | JSON explorer links with title, baseUrl, optional logo and per-entity paths. | `[]` | No | `apps/explorer/configs/app/ui.ts` |
| `NEXT_PUBLIC_NETWORK_ICON` | Network icon asset and fallback input to favicon generation. | None | No | `apps/explorer/configs/app/ui.ts` |
| `NEXT_PUBLIC_NETWORK_ICON_DARK` | Dark-mode network icon asset. | None | No | `apps/explorer/configs/app/ui.ts` |
| `NEXT_PUBLIC_NETWORK_LOGO` | Network logo asset and input to Open Graph image generation. | None | No | `apps/explorer/configs/app/ui.ts`; `apps/explorer/deploy/scripts/og_image_generator.js` |
| `NEXT_PUBLIC_NETWORK_LOGO_DARK` | Dark-mode logo asset and preferred Open Graph image logo. | None | No | `apps/explorer/configs/app/ui.ts`; `apps/explorer/deploy/scripts/og_image_generator.js` |
| `NEXT_PUBLIC_NETWORK_MULTIPLE_GAS_CURRENCIES` | Enables multiple gas currency presentation. | `false` | No | `apps/explorer/configs/app/chain.ts` |
| `NEXT_PUBLIC_NETWORK_RPC_URL` | Public node JSON-RPC URL or JSON URL array; use root endpoints for Paxeer mainnet-beta; needed for single-chain wallet interaction. | Empty URL list | No | `apps/explorer/configs/app/chain.ts`; `apps/explorer/nextjs/getServerSideProps/handlers.ts` |
| `NEXT_PUBLIC_NETWORK_SECONDARY_COIN_SYMBOL` | Secondary native coin ticker. | None | No | `apps/explorer/configs/app/chain.ts` |
| `NEXT_PUBLIC_NETWORK_SHORT_NAME` | Short display name for the chain. | None | No | `apps/explorer/configs/app/chain.ts` |
| `NEXT_PUBLIC_NETWORK_TOKEN_STANDARD_NAME` | Prefix used to name token standards. | `ERC` | No | `apps/explorer/configs/app/chain.ts` |
| `NEXT_PUBLIC_NETWORK_VERIFICATION_TYPE` | Block-author terminology; use validation for Paxeer BFT consensus. | `mining`; Arbitrum uses posting and zkEVM uses sequencing | No | `apps/explorer/configs/app/chain.ts` |
| `NEXT_PUBLIC_OG_DESCRIPTION` | Open Graph description. | Empty string | No | `apps/explorer/configs/app/meta.ts` |
| `NEXT_PUBLIC_OG_ENHANCED_DATA_ENABLED` | Adds entity-specific Open Graph metadata. | `false` | No | `apps/explorer/configs/app/meta.ts` |
| `NEXT_PUBLIC_OG_IMAGE_URL` | Open Graph image asset override; setting it suppresses generated-image creation. | App origin plus /explorer/static/og_image.png | No | `apps/explorer/configs/app/meta.ts`; `apps/explorer/deploy/scripts/og_image_generator.js` |
| `NEXT_PUBLIC_OTHER_LINKS` | JSON external navigation links with text and url. | `[]` | No | `apps/explorer/configs/app/ui.ts` |
| `NEXT_PUBLIC_PAXEER_X_ENABLED` | Enables Paxeer X navigation, lists and price-feed UI. | `false` | No | `apps/explorer/configs/app/features/paxeerX.ts`; `apps/explorer/configs/app/features/paxeerXFeed.ts`; `apps/explorer/configs/app/features/paxeerXLists.ts` |
| `NEXT_PUBLIC_PROMOTE_BLOCKSCOUT_IN_TITLE` | Includes the Blockscout attribution in the page title unless explicitly false. | `true` | No | `apps/explorer/configs/app/meta.ts` |
| `NEXT_PUBLIC_PUZZLE_GAME_BADGE_CLAIM_LINK` | Claim link for the puzzle badge. | None | No | `apps/explorer/configs/app/features/easterEggPuzzleBadge.ts` |
| `NEXT_PUBLIC_REWARDS_SERVICE_API_HOST` | Full base URL for the rewards service. | None | No | `apps/explorer/configs/app/apis.ts` |
| `NEXT_PUBLIC_RE_CAPTCHA_APP_SITE_KEY` | Public reCAPTCHA v2 site identifier; enables the auth0 account path when account support is selected. | None | No | `apps/explorer/configs/app/services.ts` |
| `NEXT_PUBLIC_ROLLBAR_CLIENT_TOKEN` | Public browser Rollbar token; enables client error reporting outside private mode. | None | No | `apps/explorer/configs/app/features/rollbar.ts` |
| `NEXT_PUBLIC_ROLLUP_DA_CELESTIA_CELENIUM_URL` | Celenium explorer base for supported rollup data-availability links. | None | No | `apps/explorer/configs/app/features/rollup.ts` |
| `NEXT_PUBLIC_ROLLUP_DA_CELESTIA_NAMESPACE` | Celestia namespace for an Arbitrum rollup. | None | No | `apps/explorer/configs/app/features/rollup.ts` |
| `NEXT_PUBLIC_ROLLUP_HOMEPAGE_SHOW_LATEST_BLOCKS` | Shows latest blocks on a rollup homepage. | `false` | No | `apps/explorer/configs/app/features/rollup.ts` |
| `NEXT_PUBLIC_ROLLUP_L2_WITHDRAWAL_URL` | Withdrawal link for an optimistic rollup. | None | No | `apps/explorer/configs/app/features/rollup.ts` |
| `NEXT_PUBLIC_ROLLUP_LAYER_NUMBER` | Layer number displayed when a rollup is configured. | `2` | No | `apps/explorer/configs/app/features/rollup.ts` |
| `NEXT_PUBLIC_ROLLUP_OUTPUT_ROOTS_ENABLED` | Enables output-root UI for an optimistic rollup. | `false` | No | `apps/explorer/configs/app/features/rollup.ts`; `apps/explorer/deploy/tools/sitemap-generator/next-sitemap.config.js` |
| `NEXT_PUBLIC_ROLLUP_PARENT_CHAIN` | JSON parent-chain descriptor; baseUrl is required when configuring a rollup. | None | No | `apps/explorer/configs/app/features/rollup.ts` |
| `NEXT_PUBLIC_ROLLUP_STAGE_INDEX` | Rollup stage indicator, restricted to 1 or 2 by the validator. | None | No | `apps/explorer/configs/app/features/rollup.ts` |
| `NEXT_PUBLIC_ROLLUP_TYPE` | Selects a supported rollup UI when a parent chain is configured. | None | No | `apps/explorer/configs/app/chain.ts`; `apps/explorer/configs/app/features/rollup.ts`; `apps/explorer/deploy/tools/sitemap-generator/next-sitemap.config.js` |
| `NEXT_PUBLIC_SAFE_TX_SERVICE_URL` | Safe transaction service origin; the source resolves /api/v1/safes against it. | None | No | `apps/explorer/configs/app/features/safe.ts` |
| `NEXT_PUBLIC_SEO_ENHANCED_DATA_ENABLED` | Adds entity-specific SEO metadata. | `false` | No | `apps/explorer/configs/app/meta.ts` |
| `NEXT_PUBLIC_STATS_API_BASE_PATH` | Path prefix for statistics API requests; trailing slash is removed. | Empty string | No | `apps/explorer/configs/app/apis.ts` |
| `NEXT_PUBLIC_STATS_API_HOST` | Full base URL for statistics and dependent homepage charts and counters. | None | No | `apps/explorer/configs/app/apis.ts`; `apps/explorer/deploy/tools/sitemap-generator/next-sitemap.config.js` |
| `NEXT_PUBLIC_TAC_OPERATION_LIFECYCLE_API_HOST` | Full base URL for the TAC operation-lifecycle service. | None | No | `apps/explorer/configs/app/apis.ts`; `apps/explorer/deploy/tools/sitemap-generator/next-sitemap.config.js` |
| `NEXT_PUBLIC_TAC_TON_EXPLORER_URL` | TON explorer link used with TAC operation-lifecycle support. | None | No | `apps/explorer/configs/app/features/tac.ts` |
| `NEXT_PUBLIC_TRANSACTION_INTERPRETATION_PROVIDER` | Transaction explanation provider; none disables interpretation. | `none` | No | `apps/explorer/configs/app/features/txInterpretation.ts` |
| `NEXT_PUBLIC_TX_EXTERNAL_TRANSACTIONS_CONFIG` | JSON external-transaction chain_name, chain_logo_url and explorer_url_template. | None | No | `apps/explorer/configs/app/features/externalTxs.ts` |
| `NEXT_PUBLIC_USER_OPS_INDEXER_API_HOST` | Full base URL for user-operation indexing. | None | No | `apps/explorer/configs/app/apis.ts` |
| `NEXT_PUBLIC_USE_NEXT_JS_PROXY` | Routes eligible client requests through the Next.js proxy. | `false` | No | `apps/explorer/configs/app/app.ts` |
| `NEXT_PUBLIC_VALIDATORS_CHAIN_TYPE` | Selects a supported validator-chain presentation. | None | No | `apps/explorer/configs/app/features/validators.ts`; `apps/explorer/deploy/tools/sitemap-generator/next-sitemap.config.js` |
| `NEXT_PUBLIC_VIEWS_ADDRESS_BECH_32_PREFIX` | Bech32 prefix, 1–83 characters; required when address formats include bech32. | None | No | `apps/explorer/configs/app/ui/views/address.ts` |
| `NEXT_PUBLIC_VIEWS_ADDRESS_FORMAT` | JSON list of supported address formats. | `["base16"]` | No | `apps/explorer/configs/app/ui/views/address.ts` |
| `NEXT_PUBLIC_VIEWS_ADDRESS_HIDDEN_VIEWS` | JSON list of hidden address views. | `[]` | No | `apps/explorer/configs/app/ui/views/address.ts` |
| `NEXT_PUBLIC_VIEWS_ADDRESS_IDENTICON_TYPE` | Address identicon style. | `blockie` | No | `apps/explorer/configs/app/ui/views/address.ts` |
| `NEXT_PUBLIC_VIEWS_ADDRESS_NATIVE_TOKEN_ADDRESS` | Native-token contract identifier used by address views. | None | No | `apps/explorer/configs/app/ui/views/address.ts` |
| `NEXT_PUBLIC_VIEWS_BLOCK_HIDDEN_FIELDS` | JSON list of hidden block fields. | `[]` | No | `apps/explorer/configs/app/ui/views/block.ts` |
| `NEXT_PUBLIC_VIEWS_BLOCK_PENDING_UPDATE_ALERT_ENABLED` | Shows the pending block-update alert unless explicitly false. | `true` | No | `apps/explorer/configs/app/ui/views/block.ts` |
| `NEXT_PUBLIC_VIEWS_CONTRACT_DECODED_BYTECODE_ENABLED` | Displays decoded contract bytecode. | `false` | No | `apps/explorer/configs/app/ui/views/address.ts` |
| `NEXT_PUBLIC_VIEWS_CONTRACT_EXTRA_VERIFICATION_METHODS` | JSON list of extra verification methods; none disables them. | `["solidity-hardhat","solidity-foundry"]` | No | `apps/explorer/configs/app/ui/views/address.ts` |
| `NEXT_PUBLIC_VIEWS_CONTRACT_SOLIDITYSCAN_ENABLED` | Enables SolidityScan contract information. | `false` | No | `apps/explorer/configs/app/ui/views/address.ts` |
| `NEXT_PUBLIC_VIEWS_NFT_MARKETPLACES` | JSON NFT marketplace links; each entry needs collection_url or instance_url. | `[]` | No | `apps/explorer/configs/app/ui/views/nft.ts` |
| `NEXT_PUBLIC_VIEWS_TOKEN_SCAM_TOGGLE_ENABLED` | Enables the scam-token visibility toggle. | `false` | No | `apps/explorer/configs/app/ui/views/token.ts` |
| `NEXT_PUBLIC_VIEWS_TX_ADDITIONAL_FIELDS` | JSON list of additional transaction fields. | `[]` | No | `apps/explorer/configs/app/ui/views/tx.ts` |
| `NEXT_PUBLIC_VIEWS_TX_GROUPED_FEES` | Groups transaction fee presentation. | `false` | No | `apps/explorer/configs/app/ui/views/tx.ts` |
| `NEXT_PUBLIC_VIEWS_TX_HIDDEN_FIELDS` | JSON list of hidden transaction fields. | `[]` | No | `apps/explorer/configs/app/ui/views/tx.ts` |
| `NEXT_PUBLIC_VISUALIZE_API_BASE_PATH` | Path prefix for visualization API requests; trailing slash is removed. | Empty string | No | `apps/explorer/configs/app/apis.ts` |
| `NEXT_PUBLIC_VISUALIZE_API_HOST` | Full base URL for the contract visualization service. | None | No | `apps/explorer/configs/app/apis.ts` |
| `NEXT_PUBLIC_WALLET_CONNECT_FEATURED_WALLET_IDS` | JSON list of featured Reown wallet identifiers. | `[]` | No | `apps/explorer/configs/app/features/blockchainInteraction.ts` |
| `NEXT_PUBLIC_WALLET_CONNECT_PROJECT_ID` | Public Reown project identifier; needed for wallet interaction unless Dynamic is selected. | None | No | `apps/explorer/configs/app/features/blockchainInteraction.ts` |
| `NEXT_PUBLIC_WEB3_DISABLE_ADD_TOKEN_TO_WALLET` | Disables adding tokens to supported wallets. | `false` | No | `apps/explorer/configs/app/features/web3Wallet.ts` |
| `NEXT_PUBLIC_WEB3_WALLETS` | JSON list of enabled wallets; none disables the wallet integration. | `["metamask","rabby","coinbase","trust","okx","token_pocket"]` | No | `apps/explorer/configs/app/features/web3Wallet.ts` |
| `NEXT_PUBLIC_XSTAR_SCORE_URL` | Base URL enabling XStar score display. | None | No | `apps/explorer/configs/app/features/xStarScore.ts` |
| `NEXT_PUBLIC_ZETACHAIN_EXTERNAL_SEARCH_CONFIG` | JSON search entries with regex, template and name; used with the ZetaChain service. | None | No | `apps/explorer/configs/app/features/zetachain.ts` |
| `NEXT_PUBLIC_ZETACHAIN_SERVICE_API_HOST` | Full base URL for ZetaChain cross-chain transactions; requires its chains configuration. | None | No | `apps/explorer/configs/app/apis.ts` |
| `NEXT_PUBLIC_ZETACHAIN_SERVICE_CHAINS_CONFIG_URL` | JSON array or asset location describing ZetaChain counterpart chains. | None | No | `apps/explorer/configs/app/features/zetachain.ts` |
| `NEXT_RUNTIME` | Next.js-supplied runtime selector for Node startup and Node/edge configuration loading. | Supplied by Next.js | No | `apps/explorer/configs/essential-dapps-chains/config.edge.ts`; `apps/explorer/configs/essential-dapps-chains/config.nodejs.ts`; `apps/explorer/configs/multichain/config.edge.ts`; `apps/explorer/configs/multichain/config.nodejs.ts`; `apps/explorer/instrumentation.ts` |
| `NODE_ENV` | Node execution mode; production selects OTLP metrics, other modes select console metrics. | No source default; set by the Next.js command | No | `apps/explorer/instrumentation.node.ts` |
| `OSTYPE` | Selects the platform-specific stat command when building the icon registry; supplied by Bash. | Bash supplied; non-Darwin values use the Linux stat form | No | `apps/explorer/deploy/scripts/build_sprite.sh` |
| `OTEL_SDK_ENABLED` | Starts the telemetry SDK after telemetry instrumentation is imported. | `false` | No | `apps/explorer/instrumentation.node.ts` |
| `PROMETHEUS_METRICS_ENABLED` | Enables metrics collection and the frontend metrics route. | `false` | No | `apps/explorer/lib/monitoring/metrics.ts`; `apps/explorer/pages/api/metrics.ts` |
| `SKIP_ENVS_VALIDATION` | Controls whether the container runs environment validation; only true bypasses validation. | `false` | No | `apps/explorer/deploy/scripts/entrypoint.sh` |
| `input` | Supplies the additional argument passed to the environment validator; --silent suppresses its progress output. | None | No | `apps/explorer/deploy/scripts/validate_envs.sh` |
| `prometheus.yml` / `global.evaluation_interval` | Rule-evaluation interval. | Configured: `15s` | No | Prometheus; launched by `apps/explorer/package.json` (monitoring:prometheus:local) |
| `prometheus.yml` / `global.scrape_interval` | Metrics scrape interval. | Configured: `15s` | No | Prometheus; launched by `apps/explorer/package.json` (monitoring:prometheus:local) |
| `prometheus.yml` / `rule_files` | Prometheus rule-file list. | No active files | No | Prometheus; launched by `apps/explorer/package.json` (monitoring:prometheus:local) |
| `prometheus.yml` / `scrape_configs` | Prometheus scrape jobs. | Configured jobs: prometheus and frontend | No | Prometheus; launched by `apps/explorer/package.json` (monitoring:prometheus:local) |
| `prometheus.yml` / `scrape_configs[].job_name` | Metrics job label. | Configured per job | No | Prometheus; launched by `apps/explorer/package.json` (monitoring:prometheus:local) |
| `prometheus.yml` / `scrape_configs[].metrics_path` | HTTP metrics route. | Configured frontend path: `/explorer/node-api/metrics` | No | Prometheus; launched by `apps/explorer/package.json` (monitoring:prometheus:local) |
| `prometheus.yml` / `scrape_configs[].scheme` | Scrape transport scheme. | Configured frontend scheme: `http` | No | Prometheus; launched by `apps/explorer/package.json` (monitoring:prometheus:local) |
| `prometheus.yml` / `scrape_configs[].static_configs[].targets` | Metrics target list. | Configured targets; addresses omitted | No | Prometheus; launched by `apps/explorer/package.json` (monitoring:prometheus:local) |
| `public/assets/configs/address_3rd_party_widgets_config.json` / `<widget>.chainIds` | Map from the explorer chain ID to a provider-specific chain ID. | Current chain ID | No | `apps/explorer/ui/address/address3rdPartyWidgets/Address3rdPartyWidgetCard.tsx`; `apps/explorer/deploy/tools/envs-validator/schemas/features/address3rdPartyWidgets.ts` |
| `public/assets/configs/address_3rd_party_widgets_config.json` / `<widget>.hint` | Tooltip text. | None | No | `apps/explorer/ui/address/address3rdPartyWidgets/Address3rdPartyWidgetCard.tsx`; `apps/explorer/deploy/tools/envs-validator/schemas/features/address3rdPartyWidgets.ts` |
| `public/assets/configs/address_3rd_party_widgets_config.json` / `<widget>.icon` | Widget icon location; required in each configured widget. | None | No | `apps/explorer/ui/address/address3rdPartyWidgets/Address3rdPartyWidgetCard.tsx`; `apps/explorer/deploy/tools/envs-validator/schemas/features/address3rdPartyWidgets.ts` |
| `public/assets/configs/address_3rd_party_widgets_config.json` / `<widget>.name` | Provider label; required in each configured widget. | None | No | `apps/explorer/ui/address/address3rdPartyWidgets/Address3rdPartyWidgetCard.tsx`; `apps/explorer/deploy/tools/envs-validator/schemas/features/address3rdPartyWidgets.ts` |
| `public/assets/configs/address_3rd_party_widgets_config.json` / `<widget>.pages` | Enabled page types: eoa, contract or token; required in each configured widget. | None | No | `apps/explorer/ui/address/address3rdPartyWidgets/Address3rdPartyWidgetCard.tsx`; `apps/explorer/deploy/tools/envs-validator/schemas/features/address3rdPartyWidgets.ts` |
| `public/assets/configs/address_3rd_party_widgets_config.json` / `<widget>.title` | Value label; required in each configured widget. | None | No | `apps/explorer/ui/address/address3rdPartyWidgets/Address3rdPartyWidgetCard.tsx`; `apps/explorer/deploy/tools/envs-validator/schemas/features/address3rdPartyWidgets.ts` |
| `public/assets/configs/address_3rd_party_widgets_config.json` / `<widget>.url` | Click-through template; required in each configured widget. | None | No | `apps/explorer/ui/address/address3rdPartyWidgets/Address3rdPartyWidgetCard.tsx`; `apps/explorer/deploy/tools/envs-validator/schemas/features/address3rdPartyWidgets.ts` |
| `public/assets/configs/address_3rd_party_widgets_config.json` / `<widget>.valuePath` | Path used to read the returned value; required in each configured widget. | None | No | `apps/explorer/ui/address/address3rdPartyWidgets/Address3rdPartyWidgetCard.tsx`; `apps/explorer/deploy/tools/envs-validator/schemas/features/address3rdPartyWidgets.ts` |
| `public/assets/configs/featured_networks.json` / `[].group` | Network-menu group for this entry; required in each configured entry. | None | No | `apps/explorer/ui/snippets/networkMenu/NetworkMenuLink.tsx`; `apps/explorer/deploy/tools/envs-validator/schemas/ui.ts`; `apps/explorer/ui/snippets/networkMenu/NetworkMenuContent.tsx` |
| `public/assets/configs/featured_networks.json` / `[].icon` | Network icon location. | None | No | `apps/explorer/ui/snippets/networkMenu/NetworkMenuLink.tsx`; `apps/explorer/deploy/tools/envs-validator/schemas/ui.ts` |
| `public/assets/configs/featured_networks.json` / `[].invertIconInDarkMode` | Inverts this icon in dark mode. | False when absent | No | `apps/explorer/ui/snippets/networkMenu/NetworkMenuLink.tsx`; `apps/explorer/deploy/tools/envs-validator/schemas/ui.ts` |
| `public/assets/configs/featured_networks.json` / `[].isActive` | Marks the current network. | Compared with the current browser origin | No | `apps/explorer/ui/snippets/networkMenu/NetworkMenuLink.tsx`; `apps/explorer/deploy/tools/envs-validator/schemas/ui.ts` |
| `public/assets/configs/featured_networks.json` / `[].title` | Network-menu label; required in each configured entry. | None | No | `apps/explorer/ui/snippets/networkMenu/NetworkMenuLink.tsx`; `apps/explorer/deploy/tools/envs-validator/schemas/ui.ts` |
| `public/assets/configs/featured_networks.json` / `[].url` | Network explorer link; required in each configured entry. | None | No | `apps/explorer/ui/snippets/networkMenu/NetworkMenuLink.tsx`; `apps/explorer/deploy/tools/envs-validator/schemas/ui.ts` |
| `public/assets/configs/footer_links.json` / `[].links` | Link array for this footer group; required in each configured group. | None | No | `apps/explorer/ui/snippets/footer/Footer.tsx`; `apps/explorer/deploy/tools/envs-validator/schemas/ui.ts` |
| `public/assets/configs/footer_links.json` / `[].links[].iconUrl` | Light/dark link-icon locations. | None | No | `apps/explorer/ui/snippets/footer/Footer.tsx`; `apps/explorer/deploy/tools/envs-validator/schemas/ui.ts` |
| `public/assets/configs/footer_links.json` / `[].links[].text` | Link label; required in each configured link. | None | No | `apps/explorer/ui/snippets/footer/Footer.tsx`; `apps/explorer/deploy/tools/envs-validator/schemas/ui.ts` |
| `public/assets/configs/footer_links.json` / `[].links[].url` | Link target; required in each configured link. | None | No | `apps/explorer/ui/snippets/footer/Footer.tsx`; `apps/explorer/deploy/tools/envs-validator/schemas/ui.ts` |
| `public/assets/configs/footer_links.json` / `[].title` | Footer group heading; required in each configured group. | None | No | `apps/explorer/ui/snippets/footer/Footer.tsx`; `apps/explorer/deploy/tools/envs-validator/schemas/ui.ts` |
| `public/assets/configs/homepage_highlights_config.json` / `[].background` | Light/dark background colors; dark falls back to light. | Theme styling | No | `apps/explorer/ui/home/PromotionalBanners.tsx`; `apps/explorer/deploy/tools/envs-validator/schemas/features/highlights.ts` |
| `public/assets/configs/homepage_highlights_config.json` / `[].description` | Banner description; required in each configured banner. | None | No | `apps/explorer/ui/home/PromotionalBanners.tsx`; `apps/explorer/deploy/tools/envs-validator/schemas/features/highlights.ts` |
| `public/assets/configs/homepage_highlights_config.json` / `[].description_color` | Light/dark description colors; dark falls back to light. | Theme styling | No | `apps/explorer/ui/home/PromotionalBanners.tsx`; `apps/explorer/deploy/tools/envs-validator/schemas/features/highlights.ts` |
| `public/assets/configs/homepage_highlights_config.json` / `[].is_pinned` | Pinned-banner flag accepted by the configuration validator. | None | No | `apps/explorer/deploy/tools/envs-validator/schemas/features/highlights.ts` |
| `public/assets/configs/homepage_highlights_config.json` / `[].page_path` | Internal page target if no redirect_url is supplied. | None | No | `apps/explorer/ui/home/PromotionalBanners.tsx`; `apps/explorer/deploy/tools/envs-validator/schemas/features/highlights.ts` |
| `public/assets/configs/homepage_highlights_config.json` / `[].redirect_url` | External click target; takes precedence over page_path. | None | No | `apps/explorer/ui/home/PromotionalBanners.tsx`; `apps/explorer/deploy/tools/envs-validator/schemas/features/highlights.ts` |
| `public/assets/configs/homepage_highlights_config.json` / `[].side_img_url` | Light/dark side-image locations; dark falls back to light. | None | No | `apps/explorer/ui/home/PromotionalBanners.tsx`; `apps/explorer/deploy/tools/envs-validator/schemas/features/highlights.ts` |
| `public/assets/configs/homepage_highlights_config.json` / `[].title` | Banner title; required in each configured banner. | None | No | `apps/explorer/ui/home/PromotionalBanners.tsx`; `apps/explorer/deploy/tools/envs-validator/schemas/features/highlights.ts` |
| `public/assets/configs/homepage_highlights_config.json` / `[].title_color` | Light/dark title colors; dark falls back to light. | Theme styling | No | `apps/explorer/ui/home/PromotionalBanners.tsx`; `apps/explorer/deploy/tools/envs-validator/schemas/features/highlights.ts` |
| `public/assets/configs/marketplace_categories.json` / `[]` | Category-name strings; categories are derived from apps if the asset has no entries. | Derived app categories | No | `apps/explorer/ui/marketplace/useMarketplaceCategories.tsx`; `apps/explorer/deploy/tools/envs-validator/schemas/features/marketplace.ts` |
| `public/assets/configs/marketplace_config.json` / `[].author` | App publisher label; required in each configured app. | None | No | `apps/explorer/ui/marketplace/useMarketplaceApps.tsx`; `apps/explorer/deploy/tools/envs-validator/schemas/features/marketplace.ts` |
| `public/assets/configs/marketplace_config.json` / `[].categories` | App category names; required in each configured app. | None | No | `apps/explorer/ui/marketplace/useMarketplaceApps.tsx`; `apps/explorer/deploy/tools/envs-validator/schemas/features/marketplace.ts` |
| `public/assets/configs/marketplace_config.json` / `[].description` | App description; required in each configured app. | None | No | `apps/explorer/ui/marketplace/useMarketplaceApps.tsx`; `apps/explorer/deploy/tools/envs-validator/schemas/features/marketplace.ts` |
| `public/assets/configs/marketplace_config.json` / `[].discord` | Discord link. | None | No | `apps/explorer/ui/marketplace/useMarketplaceApps.tsx`; `apps/explorer/deploy/tools/envs-validator/schemas/features/marketplace.ts` |
| `public/assets/configs/marketplace_config.json` / `[].external` | Opens the app as an external site. | False when absent | No | `apps/explorer/ui/marketplace/useMarketplaceApps.tsx`; `apps/explorer/deploy/tools/envs-validator/schemas/features/marketplace.ts` |
| `public/assets/configs/marketplace_config.json` / `[].github` | GitHub link or link array. | None | No | `apps/explorer/ui/marketplace/useMarketplaceApps.tsx`; `apps/explorer/deploy/tools/envs-validator/schemas/features/marketplace.ts` |
| `public/assets/configs/marketplace_config.json` / `[].id` | App identifier; required in each configured app. | None | No | `apps/explorer/ui/marketplace/useMarketplaceApps.tsx`; `apps/explorer/deploy/tools/envs-validator/schemas/features/marketplace.ts` |
| `public/assets/configs/marketplace_config.json` / `[].internalWallet` | Enables the embedded-wallet integration. | False when absent | No | `apps/explorer/ui/marketplace/useMarketplaceApps.tsx`; `apps/explorer/deploy/tools/envs-validator/schemas/features/marketplace.ts` |
| `public/assets/configs/marketplace_config.json` / `[].logo` | App logo location; required in each configured app. | None | No | `apps/explorer/ui/marketplace/useMarketplaceApps.tsx`; `apps/explorer/deploy/tools/envs-validator/schemas/features/marketplace.ts` |
| `public/assets/configs/marketplace_config.json` / `[].logoDarkMode` | Dark-mode logo location. | App logo | No | `apps/explorer/ui/marketplace/useMarketplaceApps.tsx`; `apps/explorer/deploy/tools/envs-validator/schemas/features/marketplace.ts` |
| `public/assets/configs/marketplace_config.json` / `[].priority` | App ordering priority. | 0 | No | `apps/explorer/ui/marketplace/useMarketplaceApps.tsx`; `apps/explorer/deploy/tools/envs-validator/schemas/features/marketplace.ts` |
| `public/assets/configs/marketplace_config.json` / `[].shortDescription` | List description; required in each configured app. | None | No | `apps/explorer/ui/marketplace/useMarketplaceApps.tsx`; `apps/explorer/deploy/tools/envs-validator/schemas/features/marketplace.ts` |
| `public/assets/configs/marketplace_config.json` / `[].site` | App website link. | None | No | `apps/explorer/ui/marketplace/useMarketplaceApps.tsx`; `apps/explorer/deploy/tools/envs-validator/schemas/features/marketplace.ts` |
| `public/assets/configs/marketplace_config.json` / `[].telegram` | Telegram link. | None | No | `apps/explorer/ui/marketplace/useMarketplaceApps.tsx`; `apps/explorer/deploy/tools/envs-validator/schemas/features/marketplace.ts` |
| `public/assets/configs/marketplace_config.json` / `[].title` | App title; required in each configured app. | None | No | `apps/explorer/ui/marketplace/useMarketplaceApps.tsx`; `apps/explorer/deploy/tools/envs-validator/schemas/features/marketplace.ts` |
| `public/assets/configs/marketplace_config.json` / `[].twitter` | Social link. | None | No | `apps/explorer/ui/marketplace/useMarketplaceApps.tsx`; `apps/explorer/deploy/tools/envs-validator/schemas/features/marketplace.ts` |
| `public/assets/configs/marketplace_config.json` / `[].url` | App launch URL; required in each configured app. | None | No | `apps/explorer/ui/marketplace/useMarketplaceApps.tsx`; `apps/explorer/deploy/tools/envs-validator/schemas/features/marketplace.ts` |
| `public/assets/configs/marketplace_graph_links.json` / `<app-id>[].title` | Label for an app graph link. | None | No | `apps/explorer/ui/marketplace/MarketplaceAppGraphLinks.tsx` |
| `public/assets/configs/marketplace_graph_links.json` / `<app-id>[].url` | Target for an app graph link. | None | No | `apps/explorer/ui/marketplace/MarketplaceAppGraphLinks.tsx` |
| `public/assets/configs/zetachain_service_chains_config.json` / `[].address_url_template` | Counterpart explorer address-link template. | None | No | `apps/explorer/configs/app/features/zetachain.ts`; `apps/explorer/deploy/tools/envs-validator/schemas/features/zetaChain.ts` |
| `public/assets/configs/zetachain_service_chains_config.json` / `[].chain_id` | Counterpart chain ID; required in each configured chain. | None | No | `apps/explorer/configs/app/features/zetachain.ts`; `apps/explorer/deploy/tools/envs-validator/schemas/features/zetaChain.ts` |
| `public/assets/configs/zetachain_service_chains_config.json` / `[].chain_logo` | Counterpart chain logo. | None | No | `apps/explorer/configs/app/features/zetachain.ts`; `apps/explorer/deploy/tools/envs-validator/schemas/features/zetaChain.ts` |
| `public/assets/configs/zetachain_service_chains_config.json` / `[].chain_name` | Counterpart chain label; required in each configured chain. | None | No | `apps/explorer/configs/app/features/zetachain.ts`; `apps/explorer/deploy/tools/envs-validator/schemas/features/zetaChain.ts` |
| `public/assets/configs/zetachain_service_chains_config.json` / `[].instance_url` | Counterpart explorer base URL. | None | No | `apps/explorer/configs/app/features/zetachain.ts`; `apps/explorer/deploy/tools/envs-validator/schemas/features/zetaChain.ts` |
| `public/assets/configs/zetachain_service_chains_config.json` / `[].tx_url_template` | Counterpart explorer transaction-link template. | None | No | `apps/explorer/configs/app/features/zetachain.ts`; `apps/explorer/deploy/tools/envs-validator/schemas/features/zetaChain.ts` |
| `public/assets/essential-dapps/chains.json` / `chains` | Generated chain-descriptor array, loaded by Node and the corresponding edge loader. | None | No | `apps/explorer/configs/essential-dapps-chains/config.nodejs.ts` |
| `public/assets/essential-dapps/chains.json` / `chains[].app_config` | Generated application configuration; follows the settings in the environment table. | None | No | `apps/explorer/configs/essential-dapps-chains/config.nodejs.ts` |
| `public/assets/essential-dapps/chains.json` / `chains[].app_config.apis.general.endpoint` | Explorer API base URL used by the chain-specific client. | None | No | `apps/explorer/lib/api/getResourceParams.ts` |
| `public/assets/essential-dapps/chains.json` / `chains[].app_config.app.baseUrl` | Explorer base URL used by wallet clients. | None | No | `apps/explorer/lib/web3/chains.ts` |
| `public/assets/essential-dapps/chains.json` / `chains[].app_config.chain.currency.decimals` | Native-currency decimal places. | None | No | `apps/explorer/lib/web3/chains.ts` |
| `public/assets/essential-dapps/chains.json` / `chains[].app_config.chain.currency.name` | Native-currency name. | Empty string | No | `apps/explorer/lib/web3/chains.ts` |
| `public/assets/essential-dapps/chains.json` / `chains[].app_config.chain.currency.symbol` | Native-currency ticker. | Empty string | No | `apps/explorer/lib/web3/chains.ts` |
| `public/assets/essential-dapps/chains.json` / `chains[].app_config.chain.id` | EVM chain identifier used by wallet clients. | None | No | `apps/explorer/lib/web3/chains.ts` |
| `public/assets/essential-dapps/chains.json` / `chains[].app_config.chain.isTestnet` | Wallet test-network flag. | None | No | `apps/explorer/lib/web3/chains.ts` |
| `public/assets/essential-dapps/chains.json` / `chains[].app_config.chain.name` | Chain label used by wallet clients. | Empty string | No | `apps/explorer/lib/web3/chains.ts` |
| `public/assets/essential-dapps/chains.json` / `chains[].app_config.chain.rpcUrls` | Node RPC URL array used by wallet transports. | None | No | `apps/explorer/lib/web3/chains.ts` |
| `public/assets/essential-dapps/chains.json` / `chains[].contracts` | Wallet contract metadata forwarded to the chain client. | None | No | `apps/explorer/lib/web3/chains.ts` |
| `public/assets/essential-dapps/chains.json` / `chains[].explorer_url` | Explorer link for this chain. | None | No | `apps/explorer/configs/essential-dapps-chains/config.nodejs.ts` |
| `public/assets/essential-dapps/chains.json` / `chains[].id` | Chain identifier. | None | No | `apps/explorer/configs/essential-dapps-chains/config.nodejs.ts` |
| `public/assets/essential-dapps/chains.json` / `chains[].logo` | Chain logo location. | None | No | `apps/explorer/configs/essential-dapps-chains/config.nodejs.ts` |
| `public/assets/essential-dapps/chains.json` / `chains[].name` | Displayed chain name. | None | No | `apps/explorer/configs/essential-dapps-chains/config.nodejs.ts` |
| `public/assets/multichain/config.json` / `chains` | Generated chain-descriptor array, loaded by Node and the corresponding edge loader. | None | No | `apps/explorer/configs/multichain/config.nodejs.ts` |
| `public/assets/multichain/config.json` / `chains[].app_config` | Generated application configuration; follows the settings in the environment table. | None | No | `apps/explorer/configs/multichain/config.nodejs.ts` |
| `public/assets/multichain/config.json` / `chains[].app_config.UI.views.address.decodedBytecodeEnabled` | Per-chain address view setting. | Derived from the corresponding environment inputs | No | `apps/explorer/configs/multichain/config.nodejs.ts` |
| `public/assets/multichain/config.json` / `chains[].app_config.UI.views.address.extraVerificationMethods` | Per-chain address view setting. | Derived from the corresponding environment inputs | No | `apps/explorer/configs/multichain/config.nodejs.ts` |
| `public/assets/multichain/config.json` / `chains[].app_config.UI.views.address.hashFormat.availableFormats` | Per-chain address view setting. | Derived from the corresponding environment inputs | No | `apps/explorer/configs/multichain/config.nodejs.ts` |
| `public/assets/multichain/config.json` / `chains[].app_config.UI.views.address.hashFormat.bech32Prefix` | Per-chain address view setting. | Derived from the corresponding environment inputs | No | `apps/explorer/configs/multichain/config.nodejs.ts` |
| `public/assets/multichain/config.json` / `chains[].app_config.UI.views.address.hiddenViews` | Per-chain address view setting. | Derived from the corresponding environment inputs | No | `apps/explorer/configs/multichain/config.nodejs.ts` |
| `public/assets/multichain/config.json` / `chains[].app_config.UI.views.address.identiconType` | Per-chain address view setting. | Derived from the corresponding environment inputs | No | `apps/explorer/configs/multichain/config.nodejs.ts` |
| `public/assets/multichain/config.json` / `chains[].app_config.UI.views.address.nativeTokenAddress` | Per-chain address view setting. | Derived from the corresponding environment inputs | No | `apps/explorer/configs/multichain/config.nodejs.ts` |
| `public/assets/multichain/config.json` / `chains[].app_config.UI.views.address.solidityscanEnabled` | Per-chain address view setting. | Derived from the corresponding environment inputs | No | `apps/explorer/configs/multichain/config.nodejs.ts` |
| `public/assets/multichain/config.json` / `chains[].app_config.UI.views.block.hiddenFields` | Per-chain block view setting. | Derived from the corresponding environment inputs | No | `apps/explorer/configs/multichain/config.nodejs.ts` |
| `public/assets/multichain/config.json` / `chains[].app_config.UI.views.block.pendingUpdateAlertEnabled` | Per-chain block view setting. | Derived from the corresponding environment inputs | No | `apps/explorer/configs/multichain/config.nodejs.ts` |
| `public/assets/multichain/config.json` / `chains[].app_config.UI.views.nft.marketplaces` | Per-chain nft view setting. | Derived from the corresponding environment inputs | No | `apps/explorer/configs/multichain/config.nodejs.ts` |
| `public/assets/multichain/config.json` / `chains[].app_config.UI.views.nft.verifiedFetch.isEnabled` | Per-chain nft view setting. | Derived from the corresponding environment inputs | No | `apps/explorer/configs/multichain/config.nodejs.ts` |
| `public/assets/multichain/config.json` / `chains[].app_config.UI.views.token.hideScamTokensEnabled` | Per-chain token view setting. | Derived from the corresponding environment inputs | No | `apps/explorer/configs/multichain/config.nodejs.ts` |
| `public/assets/multichain/config.json` / `chains[].app_config.UI.views.tx.additionalFields` | Per-chain tx view setting. | Derived from the corresponding environment inputs | No | `apps/explorer/configs/multichain/config.nodejs.ts` |
| `public/assets/multichain/config.json` / `chains[].app_config.UI.views.tx.groupedFees` | Per-chain tx view setting. | Derived from the corresponding environment inputs | No | `apps/explorer/configs/multichain/config.nodejs.ts` |
| `public/assets/multichain/config.json` / `chains[].app_config.UI.views.tx.hiddenFields` | Per-chain tx view setting. | Derived from the corresponding environment inputs | No | `apps/explorer/configs/multichain/config.nodejs.ts` |
| `public/assets/multichain/config.json` / `chains[].app_config.apis.<api>.basePath` | Per-service API path prefix. | Derived from the service environment inputs | No | `apps/explorer/lib/api/getResourceParams.ts` |
| `public/assets/multichain/config.json` / `chains[].app_config.apis.<api>.endpoint` | Per-service base URL. | Derived from the service environment inputs | No | `apps/explorer/lib/api/getResourceParams.ts` |
| `public/assets/multichain/config.json` / `chains[].app_config.apis.<api>.host` | General API host. | Derived from NEXT_PUBLIC_API_HOST | No | `apps/explorer/lib/api/getResourceParams.ts` |
| `public/assets/multichain/config.json` / `chains[].app_config.apis.<api>.port` | General API port. | Derived from NEXT_PUBLIC_API_PORT | No | `apps/explorer/lib/api/getResourceParams.ts` |
| `public/assets/multichain/config.json` / `chains[].app_config.apis.<api>.protocol` | General API scheme. | Derived from NEXT_PUBLIC_API_PROTOCOL | No | `apps/explorer/lib/api/getResourceParams.ts` |
| `public/assets/multichain/config.json` / `chains[].app_config.apis.<api>.socketEndpoint` | Per-service WebSocket endpoint. | Derived from the service environment inputs | No | `apps/explorer/lib/api/getResourceParams.ts` |
| `public/assets/multichain/config.json` / `chains[].app_config.apis.general.endpoint` | Explorer API base URL used by the chain-specific client. | None | No | `apps/explorer/lib/api/getResourceParams.ts` |
| `public/assets/multichain/config.json` / `chains[].app_config.app.baseUrl` | Explorer base URL used by wallet clients. | None | No | `apps/explorer/lib/web3/chains.ts` |
| `public/assets/multichain/config.json` / `chains[].app_config.chain.additionalTokenTypes` | Additional token type records. | Derived from NEXT_PUBLIC_NETWORK_ADDITIONAL_TOKEN_TYPES | No | `apps/explorer/configs/multichain/config.nodejs.ts` |
| `public/assets/multichain/config.json` / `chains[].app_config.chain.additionalTokenTypes[].id` | Additional token type ID. | None | No | `apps/explorer/configs/multichain/config.nodejs.ts` |
| `public/assets/multichain/config.json` / `chains[].app_config.chain.additionalTokenTypes[].name` | Additional token type label. | None | No | `apps/explorer/configs/multichain/config.nodejs.ts` |
| `public/assets/multichain/config.json` / `chains[].app_config.chain.currency.decimals` | Native-currency decimal places. | None | No | `apps/explorer/lib/web3/chains.ts` |
| `public/assets/multichain/config.json` / `chains[].app_config.chain.currency.gweiName` | Gwei-denomination label. | Derived from NEXT_PUBLIC_NETWORK_CURRENCY_GWEI_NAME | No | `apps/explorer/configs/multichain/config.nodejs.ts` |
| `public/assets/multichain/config.json` / `chains[].app_config.chain.currency.name` | Native-currency name. | Empty string | No | `apps/explorer/lib/web3/chains.ts` |
| `public/assets/multichain/config.json` / `chains[].app_config.chain.currency.symbol` | Native-currency ticker. | Empty string | No | `apps/explorer/lib/web3/chains.ts` |
| `public/assets/multichain/config.json` / `chains[].app_config.chain.currency.weiName` | Wei-denomination label. | Derived from NEXT_PUBLIC_NETWORK_CURRENCY_WEI_NAME | No | `apps/explorer/configs/multichain/config.nodejs.ts` |
| `public/assets/multichain/config.json` / `chains[].app_config.chain.hasMultipleGasCurrencies` | Multiple gas-currency flag. | Derived from NEXT_PUBLIC_NETWORK_MULTIPLE_GAS_CURRENCIES | No | `apps/explorer/configs/multichain/config.nodejs.ts` |
| `public/assets/multichain/config.json` / `chains[].app_config.chain.id` | EVM chain identifier used by wallet clients. | None | No | `apps/explorer/lib/web3/chains.ts` |
| `public/assets/multichain/config.json` / `chains[].app_config.chain.isTestnet` | Wallet test-network flag. | None | No | `apps/explorer/lib/web3/chains.ts` |
| `public/assets/multichain/config.json` / `chains[].app_config.chain.name` | Chain label used by wallet clients. | Empty string | No | `apps/explorer/lib/web3/chains.ts` |
| `public/assets/multichain/config.json` / `chains[].app_config.chain.rpcUrls` | Node RPC URL array used by wallet transports. | None | No | `apps/explorer/lib/web3/chains.ts` |
| `public/assets/multichain/config.json` / `chains[].app_config.chain.secondaryCoin.symbol` | Secondary coin ticker. | Derived from NEXT_PUBLIC_NETWORK_SECONDARY_COIN_SYMBOL | No | `apps/explorer/configs/multichain/config.nodejs.ts` |
| `public/assets/multichain/config.json` / `chains[].app_config.chain.shortName` | Short chain label. | Derived from NEXT_PUBLIC_NETWORK_SHORT_NAME | No | `apps/explorer/configs/multichain/config.nodejs.ts` |
| `public/assets/multichain/config.json` / `chains[].app_config.chain.tokenStandard` | Token-standard prefix. | Derived from NEXT_PUBLIC_NETWORK_TOKEN_STANDARD_NAME | No | `apps/explorer/configs/multichain/config.nodejs.ts` |
| `public/assets/multichain/config.json` / `chains[].app_config.chain.verificationType` | Block-author terminology. | Derived from NEXT_PUBLIC_NETWORK_VERIFICATION_TYPE | No | `apps/explorer/configs/multichain/config.nodejs.ts` |
| `public/assets/multichain/config.json` / `chains[].app_config.features.addressMetadata.isAddressTagsUpdateEnabled` | Tag-update feature flag. | Derived from the corresponding environment inputs | No | `apps/explorer/lib/address/useAddressMetadataInfoQuery.ts` |
| `public/assets/multichain/config.json` / `chains[].app_config.features.addressMetadata.isEnabled` | Metadata feature flag. | Derived from the corresponding environment inputs | No | `apps/explorer/lib/address/useAddressMetadataInfoQuery.ts` |
| `public/assets/multichain/config.json` / `chains[].app_config.features.gasTracker.isEnabled` | Gas-tracker feature flag. | Derived from the corresponding environment inputs | No | `apps/explorer/ui/shared/gas/GasPrice.tsx` |
| `public/assets/multichain/config.json` / `chains[].app_config.features.gasTracker.units` | Gas-price unit list. | Derived from the corresponding environment inputs | No | `apps/explorer/ui/shared/gas/GasPrice.tsx` |
| `public/assets/multichain/config.json` / `chains[].app_config.features.stats.isEnabled` | Statistics feature flag. | Derived from the corresponding environment inputs | No | `apps/explorer/ui/multichain/stats/MultichainStats.tsx` |
| `public/assets/multichain/config.json` / `chains[].app_config.features.txInterpretation.isEnabled` | Transaction interpretation flag. | Derived from the corresponding environment inputs | No | `apps/explorer/ui/tx/TxSubHeading.tsx` |
| `public/assets/multichain/config.json` / `chains[].app_config.features.txInterpretation.provider` | Transaction interpretation provider. | Derived from the corresponding environment inputs | No | `apps/explorer/ui/tx/TxSubHeading.tsx` |
| `public/assets/multichain/config.json` / `chains[].app_config.features.userOps.isEnabled` | User-operation feature flag. | Derived from the corresponding environment inputs | No | `apps/explorer/ui/multichain/userOps/MultichainUserOps.tsx` |
| `public/assets/multichain/config.json` / `chains[].explorer_url` | Explorer link for this chain. | None | No | `apps/explorer/configs/multichain/config.nodejs.ts` |
| `public/assets/multichain/config.json` / `chains[].id` | Chain identifier. | None | No | `apps/explorer/configs/multichain/config.nodejs.ts` |
| `public/assets/multichain/config.json` / `chains[].logo` | Chain logo location. | None | No | `apps/explorer/configs/multichain/config.nodejs.ts` |
| `public/assets/multichain/config.json` / `chains[].name` | Displayed chain name. | None | No | `apps/explorer/configs/multichain/config.nodejs.ts` |
| `public/assets/multichain/config.json` / `chains[].slug` | Chain route selector. | None | No | `apps/explorer/lib/contexts/multichain.tsx` |
