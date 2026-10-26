# Explorer frontend config contract

Source revision: `2a1033aa2`. This contract freezes the cited interface and acceptance blocks of the specification. Statuses include the specification overrides. A source marked planned has line 0; a positive source line is verified in the pinned revision.

The context and decision tables reproduce the rules linked by interface identifiers. An anchor without a linked context gains no additional storage, gas, activation or pause rule here. Acceptance qualification remains UNRUN until its declared gate succeeds.

## cfg-apis

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.cfg-fe-apis-admin</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/apis.ts:231</code> | <code>config.apis.admin</code> |
| <code>interface.14.cfg-fe-apis-bens</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/apis.ts:232</code> | <code>config.apis.bens</code> |
| <code>interface.14.cfg-fe-apis-clusters</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/apis.ts:233</code> | <code>config.apis.clusters</code> |
| <code>interface.14.cfg-fe-apis-contractinfo</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/apis.ts:234</code> | <code>config.apis.contractInfo</code> |
| <code>interface.14.cfg-fe-apis-general</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/apis.ts:230</code> | <code>config.apis.general</code> |
| <code>interface.14.cfg-fe-apis-interchainindexer</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/apis.ts:235</code> | <code>config.apis.interchainIndexer</code> |
| <code>interface.14.cfg-fe-apis-metadata</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/apis.ts:236</code> | <code>config.apis.metadata</code> |
| <code>interface.14.cfg-fe-apis-multichainaggregator</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/apis.ts:237</code> | <code>config.apis.multichainAggregator</code> |
| <code>interface.14.cfg-fe-apis-multichainstats</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/apis.ts:238</code> | <code>config.apis.multichainStats</code> |
| <code>interface.14.cfg-fe-apis-rewards</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/apis.ts:239</code> | <code>config.apis.rewards</code> |
| <code>interface.14.cfg-fe-apis-stats</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/apis.ts:240</code> | <code>config.apis.stats</code> |
| <code>interface.14.cfg-fe-apis-tac</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/apis.ts:241</code> | <code>config.apis.tac</code> |
| <code>interface.14.cfg-fe-apis-userops</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/apis.ts:242</code> | <code>config.apis.userOps</code> |
| <code>interface.14.cfg-fe-apis-visualize</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/apis.ts:243</code> | <code>config.apis.visualize</code> |
| <code>interface.14.cfg-fe-apis-zetachain</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/apis.ts:244</code> | <code>config.apis.zetachain</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.cfg-fe-apis-admin</code> | notes | 0 reader site(s) outside configs/app |
| <code>interface.14.cfg-fe-apis-bens</code> | notes | 3 reader site(s) outside configs/app |
| <code>interface.14.cfg-fe-apis-clusters</code> | notes | 0 reader site(s) outside configs/app |
| <code>interface.14.cfg-fe-apis-contractinfo</code> | notes | 0 reader site(s) outside configs/app |
| <code>interface.14.cfg-fe-apis-general</code> | notes | 16 reader site(s) outside configs/app |
| <code>interface.14.cfg-fe-apis-interchainindexer</code> | notes | 3 reader site(s) outside configs/app |
| <code>interface.14.cfg-fe-apis-metadata</code> | notes | 0 reader site(s) outside configs/app |
| <code>interface.14.cfg-fe-apis-multichainaggregator</code> | notes | 0 reader site(s) outside configs/app |
| <code>interface.14.cfg-fe-apis-multichainstats</code> | notes | 0 reader site(s) outside configs/app |
| <code>interface.14.cfg-fe-apis-rewards</code> | notes | 6 reader site(s) outside configs/app |
| <code>interface.14.cfg-fe-apis-stats</code> | notes | 4 reader site(s) outside configs/app |
| <code>interface.14.cfg-fe-apis-tac</code> | notes | 6 reader site(s) outside configs/app |
| <code>interface.14.cfg-fe-apis-userops</code> | notes | 3 reader site(s) outside configs/app |
| <code>interface.14.cfg-fe-apis-visualize</code> | notes | 0 reader site(s) outside configs/app |
| <code>interface.14.cfg-fe-apis-zetachain</code> | notes | 8 reader site(s) outside configs/app |

## cfg-app

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.cfg-fe-app-basepath</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/app.ts:30</code> | <code>config.app.basePath</code> |
| <code>interface.14.cfg-fe-app-baseurl</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/app.ts:28</code> | <code>config.app.baseUrl</code> |
| <code>interface.14.cfg-fe-app-host</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/app.ts:26</code> | <code>config.app.host</code> |
| <code>interface.14.cfg-fe-app-isdev</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/app.ts:22</code> | <code>config.app.isDev</code> |
| <code>interface.14.cfg-fe-app-isprivatemode</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/app.ts:33</code> | <code>config.app.isPrivateMode</code> |
| <code>interface.14.cfg-fe-app-ispw</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/app.ts:24</code> | <code>config.app.isPw</code> |
| <code>interface.14.cfg-fe-app-isreview</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/app.ts:23</code> | <code>config.app.isReview</code> |
| <code>interface.14.cfg-fe-app-origin</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/app.ts:29</code> | <code>config.app.origin</code> |
| <code>interface.14.cfg-fe-app-port</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/app.ts:27</code> | <code>config.app.port</code> |
| <code>interface.14.cfg-fe-app-protocol</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/app.ts:25</code> | <code>config.app.protocol</code> |
| <code>interface.14.cfg-fe-app-spritehash</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/app.ts:32</code> | <code>config.app.spriteHash</code> |
| <code>interface.14.cfg-fe-app-useproxy</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/app.ts:31</code> | <code>config.app.useProxy</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.cfg-fe-app-basepath</code> | notes | 0 reader site(s) outside configs/app |
| <code>interface.14.cfg-fe-app-baseurl</code> | notes | 18 reader site(s) outside configs/app |
| <code>interface.14.cfg-fe-app-host</code> | notes | 13 reader site(s) outside configs/app |
| <code>interface.14.cfg-fe-app-isdev</code> | notes | 5 reader site(s) outside configs/app |
| <code>interface.14.cfg-fe-app-isprivatemode</code> | notes | 8 reader site(s) outside configs/app |
| <code>interface.14.cfg-fe-app-ispw</code> | notes | 2 reader site(s) outside configs/app |
| <code>interface.14.cfg-fe-app-isreview</code> | notes | 1 reader site(s) outside configs/app |
| <code>interface.14.cfg-fe-app-origin</code> | notes | 3 reader site(s) outside configs/app |
| <code>interface.14.cfg-fe-app-port</code> | notes | 2 reader site(s) outside configs/app |
| <code>interface.14.cfg-fe-app-protocol</code> | notes | 2 reader site(s) outside configs/app |
| <code>interface.14.cfg-fe-app-spritehash</code> | notes | 1 reader site(s) outside configs/app |
| <code>interface.14.cfg-fe-app-useproxy</code> | notes | 1 reader site(s) outside configs/app |

## cfg-chain

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.cfg-fe-chain-additionaltokentypes</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/chain.ts:52</code> | <code>config.chain.additionalTokenTypes</code> |
| <code>interface.14.cfg-fe-chain-currency</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/chain.ts:40</code> | <code>config.chain.currency</code> |
| <code>interface.14.cfg-fe-chain-hasmultiplegascurrencies</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/chain.ts:50</code> | <code>config.chain.hasMultipleGasCurrencies</code> |
| <code>interface.14.cfg-fe-chain-id</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/chain.ts:37</code> | <code>config.chain.id</code> |
| <code>interface.14.cfg-fe-chain-istestnet</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/chain.ts:54</code> | <code>config.chain.isTestnet</code> |
| <code>interface.14.cfg-fe-chain-name</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/chain.ts:38</code> | <code>config.chain.name</code> |
| <code>interface.14.cfg-fe-chain-rpcurls</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/chain.ts:53</code> | <code>config.chain.rpcUrls</code> |
| <code>interface.14.cfg-fe-chain-secondarycoin</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/chain.ts:47</code> | <code>config.chain.secondaryCoin</code> |
| <code>interface.14.cfg-fe-chain-shortname</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/chain.ts:39</code> | <code>config.chain.shortName</code> |
| <code>interface.14.cfg-fe-chain-tokenstandard</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/chain.ts:51</code> | <code>config.chain.tokenStandard</code> |
| <code>interface.14.cfg-fe-chain-verificationtype</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/chain.ts:55</code> | <code>config.chain.verificationType</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.cfg-fe-chain-additionaltokentypes</code> | notes | 9 reader site(s) outside configs/app |
| <code>interface.14.cfg-fe-chain-currency</code> | notes | 48 reader site(s) outside configs/app; 8 further callers omitted |
| <code>interface.14.cfg-fe-chain-hasmultiplegascurrencies</code> | notes | 1 reader site(s) outside configs/app |
| <code>interface.14.cfg-fe-chain-id</code> | notes | 88 reader site(s) outside configs/app; 48 further callers omitted |
| <code>interface.14.cfg-fe-chain-istestnet</code> | notes | 4 reader site(s) outside configs/app |
| <code>interface.14.cfg-fe-chain-name</code> | notes | 43 reader site(s) outside configs/app; 3 further callers omitted |
| <code>interface.14.cfg-fe-chain-rpcurls</code> | notes | 17 reader site(s) outside configs/app |
| <code>interface.14.cfg-fe-chain-secondarycoin</code> | notes | 4 reader site(s) outside configs/app |
| <code>interface.14.cfg-fe-chain-shortname</code> | notes | 1 reader site(s) outside configs/app |
| <code>interface.14.cfg-fe-chain-tokenstandard</code> | notes | 6 reader site(s) outside configs/app |
| <code>interface.14.cfg-fe-chain-verificationtype</code> | notes | 1 reader site(s) outside configs/app |

## cfg-essential-dapps-chains

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.cfg-fe-essential-dapps-chains</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/essential-dapps-chains/index.ts:6</code> | <code>const marketplaceFeature = config.features.marketplace;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.cfg-fe-essential-dapps-chains</code> | notes | 11 importer(s) |

## cfg-features

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.cfg-fe-feature-account</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/account.ts:21</code> | <code>config.features.account {title, isEnabled, authProvider, dynamic, apiKeysButton}</code> |
| <code>interface.14.cfg-fe-feature-address3rdpartywidgets</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/address3rdPartyWidgets.ts:7</code> | <code>config.features.address3rdPartyWidgets {title, isEnabled, widgets, configUrl}</code> |
| <code>interface.14.cfg-fe-feature-addressmetadata</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/addressMetadata.ts:8</code> | <code>config.features.addressMetadata {title, isEnabled, isAddressTagsUpdateEnabled}</code> |
| <code>interface.14.cfg-fe-feature-addressprofileapi</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/addressProfileAPI.ts:21</code> | <code>config.features.addressProfileAPI {title, isEnabled, apiUrlTemplate, tagLinkTemplate, tagIcon, tagBgColor, tagTextColor}</code> |
| <code>interface.14.cfg-fe-feature-addressverification</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/addressVerification.ts:10</code> | <code>config.features.addressVerification {title, isEnabled}</code> |
| <code>interface.14.cfg-fe-feature-adsbanner</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/adsBanner.ts:45</code> | <code>config.features.adsBanner {title, isEnabled, provider, adButler, isSpecifyEnabled, additionalProvider}</code> |
| <code>interface.14.cfg-fe-feature-adstext</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/adsText.ts:27</code> | <code>config.features.adsText {title, isEnabled, provider, sevio}</code> |
| <code>interface.14.cfg-fe-feature-advancedfilter</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/advancedFilter.ts:9</code> | <code>config.features.advancedFilter {title, isEnabled}</code> |
| <code>interface.14.cfg-fe-feature-apidocs</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/apiDocs.ts:16</code> | <code>config.features.apiDocs {title, isEnabled, tabs, alertMessage}</code> |
| <code>interface.14.cfg-fe-feature-beaconchain</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/beaconChain.ts:7</code> | <code>config.features.beaconChain {title, isEnabled, currency, validatorUrlTemplate, withdrawalsOnly}</code> |
| <code>interface.14.cfg-fe-feature-blockchaininteraction</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/blockchainInteraction.ts:21</code> | <code>config.features.blockchainInteraction {title, isEnabled, connectorType, dynamic, reown}</code> |
| <code>interface.14.cfg-fe-feature-bridgedtokens</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/bridgedTokens.ts:8</code> | <code>config.features.bridgedTokens {title, isEnabled, chains, bridges}</code> |
| <code>interface.14.cfg-fe-feature-celo</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/celo.ts:7</code> | <code>config.features.celo {title, isEnabled}</code> |
| <code>interface.14.cfg-fe-feature-crosschaintxs</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/crossChainTxs.ts:8</code> | <code>config.features.crossChainTxs {title, isEnabled}</code> |
| <code>interface.14.cfg-fe-feature-csvexport</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/csvExport.ts:7</code> | <code>config.features.csvExport {title, isEnabled, reCaptcha}</code> |
| <code>interface.14.cfg-fe-feature-dataavailability</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/dataAvailability.ts:7</code> | <code>config.features.dataAvailability {title, isEnabled}</code> |
| <code>interface.14.cfg-fe-feature-defidropdown</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/deFiDropdown.ts:11</code> | <code>config.features.deFiDropdown {title, isEnabled, items}</code> |
| <code>interface.14.cfg-fe-feature-eastereggbadge</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/easterEggBadge.ts:9</code> | <code>config.features.easterEggBadge {title, isEnabled, badgeClaimLink}</code> |
| <code>interface.14.cfg-fe-feature-eastereggpuzzlebadge</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/easterEggPuzzleBadge.ts:9</code> | <code>config.features.easterEggPuzzleBadge {title, isEnabled, badgeClaimLink}</code> |
| <code>interface.14.cfg-fe-feature-externaltxs</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/externalTxs.ts:10</code> | <code>config.features.externalTxs {title, isEnabled, chainName, chainLogoUrl, explorerUrlTemplate}</code> |
| <code>interface.14.cfg-fe-feature-faultproofsystem</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/faultProofSystem.ts:8</code> | <code>config.features.faultProofSystem {title, isEnabled}</code> |
| <code>interface.14.cfg-fe-feature-flashblocks</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/flashblocks.ts:10</code> | <code>config.features.flashblocks {title, isEnabled, socketUrl, type, name}</code> |
| <code>interface.14.cfg-fe-feature-gastracker</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/gasTracker.ts:26</code> | <code>config.features.gasTracker {title, isEnabled, units}</code> |
| <code>interface.14.cfg-fe-feature-getgasbutton</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/getGasButton.ts:12</code> | <code>config.features.getGasButton {title, isEnabled, name, logoUrl, url, dappId}</code> |
| <code>interface.14.cfg-fe-feature-googleanalytics</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/googleAnalytics.ts:10</code> | <code>config.features.googleAnalytics {title, isEnabled, propertyId}</code> |
| <code>interface.14.cfg-fe-feature-growthbook</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/growthBook.ts:10</code> | <code>config.features.growthBook {title, isEnabled, clientKey}</code> |
| <code>interface.14.cfg-fe-feature-hotcontracts</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/hotContracts.ts:7</code> | <code>config.features.hotContracts {title, isEnabled}</code> |
| <code>interface.14.cfg-fe-feature-marketplace</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/marketplace.ts:20</code> | <code>config.features.marketplace {title, isEnabled, configUrl, props, api}</code> |
| <code>interface.14.cfg-fe-feature-megaeth</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/megaEth.ts:10</code> | <code>config.features.megaEth {title, isEnabled, socketUrl}</code> |
| <code>interface.14.cfg-fe-feature-metasuites</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/metasuites.ts:7</code> | <code>config.features.metasuites {title, isEnabled}</code> |
| <code>interface.14.cfg-fe-feature-mixpanel</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/mixpanel.ts:9</code> | <code>config.features.mixpanel {title, isEnabled, projectToken, configOverrides}</code> |
| <code>interface.14.cfg-fe-feature-mudframework</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/mudFramework.ts:8</code> | <code>config.features.mudFramework {title, isEnabled}</code> |
| <code>interface.14.cfg-fe-feature-multichain</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/multichain.ts:11</code> | <code>config.features.multichain {title, isEnabled, cluster}</code> |
| <code>interface.14.cfg-fe-feature-multichainbutton</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/multichainButton.ts:11</code> | <code>config.features.multichainButton {title, isEnabled, providers}</code> |
| <code>interface.14.cfg-fe-feature-nameservices</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/nameServices.ts:8</code> | <code>config.features.nameServices {title, isEnabled, ens, clusters}</code> |
| <code>interface.14.cfg-fe-feature-paxeerxfeed</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/paxeerXFeed.ts:9</code> | <code>config.features.paxeerXFeed {title, isEnabled}</code> |
| <code>interface.14.cfg-fe-feature-paxeerxlists</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/paxeerXLists.ts:7</code> | <code>config.features.paxeerXLists {title, isEnabled}</code> |
| <code>interface.14.cfg-fe-feature-pools</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/pools.ts:10</code> | <code>config.features.pools {title, isEnabled}</code> |
| <code>interface.14.cfg-fe-feature-publictagssubmission</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/publicTagsSubmission.ts:10</code> | <code>config.features.publicTagsSubmission {title, isEnabled}</code> |
| <code>interface.14.cfg-fe-feature-rewards</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/rewards.ts:10</code> | <code>config.features.rewards {title, isEnabled}</code> |
| <code>interface.14.cfg-fe-feature-rollbar</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/rollbar.ts:20</code> | <code>config.features.rollbar {title, isEnabled, clientToken, environment, instance, codeVersion}</code> |
| <code>interface.14.cfg-fe-feature-rollup</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/rollup.ts:30</code> | <code>config.features.rollup {title, isEnabled, type, stageIndex, layerNumber, L2WithdrawalUrl, outputRootsEnabled, interopEnabled, homepage, parentChain, DA}</code> |
| <code>interface.14.cfg-fe-feature-safe</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/safe.ts:16</code> | <code>config.features.safe {title, isEnabled, apiUrl}</code> |
| <code>interface.14.cfg-fe-feature-sol2uml</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/sol2uml.ts:7</code> | <code>config.features.sol2uml {title, isEnabled}</code> |
| <code>interface.14.cfg-fe-feature-stats</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/stats.ts:8</code> | <code>config.features.stats {title, isEnabled}</code> |
| <code>interface.14.cfg-fe-feature-suave</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/suave.ts:7</code> | <code>config.features.suave {title, isEnabled}</code> |
| <code>interface.14.cfg-fe-feature-tac</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/tac.ts:10</code> | <code>config.features.tac {title, isEnabled, tonExplorerUrl}</code> |
| <code>interface.14.cfg-fe-feature-txinterpretation</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/txInterpretation.ts:19</code> | <code>config.features.txInterpretation {title, provider, isEnabled}</code> |
| <code>interface.14.cfg-fe-feature-userops</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/userOps.ts:7</code> | <code>config.features.userOps {title, isEnabled}</code> |
| <code>interface.14.cfg-fe-feature-validators</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/validators.ts:14</code> | <code>config.features.validators {title, isEnabled, chainType}</code> |
| <code>interface.14.cfg-fe-feature-verifiedtokens</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/verifiedTokens.ts:7</code> | <code>config.features.verifiedTokens {title, isEnabled}</code> |
| <code>interface.14.cfg-fe-feature-web3wallet</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/web3Wallet.ts:25</code> | <code>config.features.web3Wallet {title, isEnabled, wallets, addToken, addNetwork}</code> |
| <code>interface.14.cfg-fe-feature-xstarscore</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/xStarScore.ts:8</code> | <code>config.features.xStarScore {title, url, isEnabled}</code> |
| <code>interface.14.cfg-fe-feature-zetachain</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/zetachain.ts:13</code> | <code>config.features.zetachain {title, isEnabled, chainsConfigUrl, externalSearchConfig}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.cfg-fe-feature-account</code> | notes | 21 reader site(s) |
| <code>interface.14.cfg-fe-feature-address3rdpartywidgets</code> | notes | 2 reader site(s) |
| <code>interface.14.cfg-fe-feature-addressmetadata</code> | notes | 5 reader site(s) |
| <code>interface.14.cfg-fe-feature-addressprofileapi</code> | notes | 3 reader site(s) |
| <code>interface.14.cfg-fe-feature-addressverification</code> | notes | 4 reader site(s) |
| <code>interface.14.cfg-fe-feature-adsbanner</code> | notes | 7 reader site(s) |
| <code>interface.14.cfg-fe-feature-adstext</code> | notes | 2 reader site(s) |
| <code>interface.14.cfg-fe-feature-advancedfilter</code> | notes | 9 reader site(s) |
| <code>interface.14.cfg-fe-feature-apidocs</code> | notes | 16 reader site(s) |
| <code>interface.14.cfg-fe-feature-beaconchain</code> | notes | 22 reader site(s) |
| <code>interface.14.cfg-fe-feature-blockchaininteraction</code> | notes | 26 reader site(s) |
| <code>interface.14.cfg-fe-feature-bridgedtokens</code> | notes | 6 reader site(s) |
| <code>interface.14.cfg-fe-feature-celo</code> | notes | 4 reader site(s) |
| <code>interface.14.cfg-fe-feature-crosschaintxs</code> | notes | 14 reader site(s) |
| <code>interface.14.cfg-fe-feature-csvexport</code> | notes | 2 reader site(s) |
| <code>interface.14.cfg-fe-feature-dataavailability</code> | notes | 13 reader site(s) |
| <code>interface.14.cfg-fe-feature-defidropdown</code> | notes | 2 reader site(s) |
| <code>interface.14.cfg-fe-feature-eastereggbadge</code> | notes | 1 reader site(s) |
| <code>interface.14.cfg-fe-feature-eastereggpuzzlebadge</code> | notes | 1 reader site(s) |
| <code>interface.14.cfg-fe-feature-externaltxs</code> | notes | 4 reader site(s) |
| <code>interface.14.cfg-fe-feature-faultproofsystem</code> | notes | 3 reader site(s) |
| <code>interface.14.cfg-fe-feature-flashblocks</code> | notes | 6 reader site(s) |
| <code>interface.14.cfg-fe-feature-gastracker</code> | notes | 7 reader site(s) |
| <code>interface.14.cfg-fe-feature-getgasbutton</code> | notes | 1 reader site(s) |
| <code>interface.14.cfg-fe-feature-googleanalytics</code> | notes | 2 reader site(s) |
| <code>interface.14.cfg-fe-feature-growthbook</code> | notes | 2 reader site(s) |
| <code>interface.14.cfg-fe-feature-hotcontracts</code> | notes | 2 reader site(s) |
| <code>interface.14.cfg-fe-feature-marketplace</code> | notes | 31 reader site(s) |
| <code>interface.14.cfg-fe-feature-megaeth</code> | notes | 4 reader site(s) |
| <code>interface.14.cfg-fe-feature-metasuites</code> | notes | 6 reader site(s) |
| <code>interface.14.cfg-fe-feature-mixpanel</code> | notes | 5 reader site(s) |
| <code>interface.14.cfg-fe-feature-mudframework</code> | notes | 8 reader site(s) |
| <code>interface.14.cfg-fe-feature-multichain</code> | notes | 39 reader site(s) |
| <code>interface.14.cfg-fe-feature-multichainbutton</code> | notes | 4 reader site(s) |
| <code>interface.14.cfg-fe-feature-nameservices</code> | notes | 19 reader site(s) |
| <code>interface.14.cfg-fe-feature-paxeerxfeed</code> | notes | 2 reader site(s) |
| <code>interface.14.cfg-fe-feature-paxeerxlists</code> | notes | 8 reader site(s) |
| <code>interface.14.cfg-fe-feature-pools</code> | notes | 2 reader site(s) |
| <code>interface.14.cfg-fe-feature-publictagssubmission</code> | notes | 3 reader site(s) |
| <code>interface.14.cfg-fe-feature-rewards</code> | notes | 18 reader site(s) |
| <code>interface.14.cfg-fe-feature-rollbar</code> | notes | 3 reader site(s) |
| <code>interface.14.cfg-fe-feature-rollup</code> | notes | 90 reader site(s); 50 further callers omitted |
| <code>interface.14.cfg-fe-feature-safe</code> | notes | 2 reader site(s) |
| <code>interface.14.cfg-fe-feature-sol2uml</code> | notes | 0 reader site(s) |
| <code>interface.14.cfg-fe-feature-stats</code> | notes | 14 reader site(s) |
| <code>interface.14.cfg-fe-feature-suave</code> | notes | 3 reader site(s) |
| <code>interface.14.cfg-fe-feature-tac</code> | notes | 7 reader site(s) |
| <code>interface.14.cfg-fe-feature-txinterpretation</code> | notes | 7 reader site(s) |
| <code>interface.14.cfg-fe-feature-userops</code> | notes | 13 reader site(s) |
| <code>interface.14.cfg-fe-feature-validators</code> | notes | 13 reader site(s) |
| <code>interface.14.cfg-fe-feature-verifiedtokens</code> | notes | 3 reader site(s) |
| <code>interface.14.cfg-fe-feature-web3wallet</code> | notes | 3 reader site(s) |
| <code>interface.14.cfg-fe-feature-xstarscore</code> | notes | 2 reader site(s) |
| <code>interface.14.cfg-fe-feature-zetachain</code> | notes | 15 reader site(s) |

## cfg-meta

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.cfg-fe-meta-og</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/meta.ts:8</code> | <code>config.meta.og</code> |
| <code>interface.14.cfg-fe-meta-promoteblockscoutintitle</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/meta.ts:7</code> | <code>config.meta.promoteBlockscoutInTitle</code> |
| <code>interface.14.cfg-fe-meta-seo</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/meta.ts:13</code> | <code>config.meta.seo</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.cfg-fe-meta-og</code> | notes | 7 reader site(s) outside configs/app |
| <code>interface.14.cfg-fe-meta-promoteblockscoutintitle</code> | notes | 1 reader site(s) outside configs/app |
| <code>interface.14.cfg-fe-meta-seo</code> | notes | 17 reader site(s) outside configs/app |

## cfg-multichain

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.cfg-fe-multichain</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/multichain/index.ts:7</code> | <code>const multichainConfig: () =&gt; MultichainConfig &#124; undefined = () =&gt; {</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.cfg-fe-multichain</code> | notes | 49 importer(s) |

## cfg-paxeer-x

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.cfg-fe-paxeer-x-footer</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/paxeer-x/footer.json:1</code> | <code>configs/paxeer-x/footer.json</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.cfg-fe-paxeer-x-footer</code> | notes | 4 referencing site(s) |

## cfg-services

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.cfg-fe-services-recaptchav2</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/services.ts:4</code> | <code>config.services.reCaptchaV2</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.cfg-fe-services-recaptchav2</code> | notes | 7 reader site(s) outside configs/app |

## cfg-ui

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.cfg-fe-ui-apikeysalert</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/ui.ts:88</code> | <code>config.UI.apiKeysAlert</code> |
| <code>interface.14.cfg-fe-ui-colortheme</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/ui.ts:98</code> | <code>config.UI.colorTheme</code> |
| <code>interface.14.cfg-fe-ui-explorers</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/ui.ts:91</code> | <code>config.UI.explorers</code> |
| <code>interface.14.cfg-fe-ui-featurednetworks</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/ui.ts:60</code> | <code>config.UI.featuredNetworks</code> |
| <code>interface.14.cfg-fe-ui-fonts</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/ui.ts:102</code> | <code>config.UI.fonts</code> |
| <code>interface.14.cfg-fe-ui-footer</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/ui.ts:65</code> | <code>config.UI.footer</code> |
| <code>interface.14.cfg-fe-ui-hascontractauditreports</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/ui.ts:97</code> | <code>config.UI.hasContractAuditReports</code> |
| <code>interface.14.cfg-fe-ui-homepage</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/ui.ts:70</code> | <code>config.UI.homepage</code> |
| <code>interface.14.cfg-fe-ui-ides</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/ui.ts:94</code> | <code>config.UI.ides</code> |
| <code>interface.14.cfg-fe-ui-indexingalert</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/ui.ts:77</code> | <code>config.UI.indexingAlert</code> |
| <code>interface.14.cfg-fe-ui-maintenancealert</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/ui.ts:85</code> | <code>config.UI.maintenanceAlert</code> |
| <code>interface.14.cfg-fe-ui-maxcontentwidth</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/ui.ts:106</code> | <code>config.UI.maxContentWidth</code> |
| <code>interface.14.cfg-fe-ui-nativecoinprice</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/ui.ts:107</code> | <code>config.UI.nativeCoinPrice</code> |
| <code>interface.14.cfg-fe-ui-navigation</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/ui.ts:46</code> | <code>config.UI.navigation</code> |
| <code>interface.14.cfg-fe-ui-views</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/ui.ts:76</code> | <code>config.UI.views</code> |
| <code>interface.14.cfg-fe-ui-views-address</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/ui/views/address.ts:66</code> | <code>config.UI.views.address {identiconType, hashFormat, hiddenViews, nativeTokenAddress, solidityscanEnabled, extraVerificationMethods, decodedBytecodeEnabled}</code> |
| <code>interface.14.cfg-fe-ui-views-block</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/ui/views/block.ts:21</code> | <code>config.UI.views.block {hiddenFields, pendingUpdateAlertEnabled}</code> |
| <code>interface.14.cfg-fe-ui-views-nft</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/ui/views/nft.ts:16</code> | <code>config.UI.views.nft {marketplaces, verifiedFetch}</code> |
| <code>interface.14.cfg-fe-ui-views-token</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/ui/views/token.ts:3</code> | <code>config.UI.views.token {hideScamTokensEnabled}</code> |
| <code>interface.14.cfg-fe-ui-views-tx</code> | <code>config_key</code> | <code>stable</code> | <code>apps/explorer/configs/app/ui/views/tx.ts:36</code> | <code>config.UI.views.tx {hiddenFields, additionalFields, groupedFees}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.cfg-fe-ui-apikeysalert</code> | notes | 1 reader site(s) outside configs/app |
| <code>interface.14.cfg-fe-ui-colortheme</code> | notes | 3 reader site(s) outside configs/app |
| <code>interface.14.cfg-fe-ui-explorers</code> | notes | 2 reader site(s) outside configs/app |
| <code>interface.14.cfg-fe-ui-featurednetworks</code> | notes | 8 reader site(s) outside configs/app |
| <code>interface.14.cfg-fe-ui-fonts</code> | notes | 6 reader site(s) outside configs/app |
| <code>interface.14.cfg-fe-ui-footer</code> | notes | 4 reader site(s) outside configs/app |
| <code>interface.14.cfg-fe-ui-hascontractauditreports</code> | notes | 1 reader site(s) outside configs/app |
| <code>interface.14.cfg-fe-ui-homepage</code> | notes | 10 reader site(s) outside configs/app |
| <code>interface.14.cfg-fe-ui-ides</code> | notes | 1 reader site(s) outside configs/app |
| <code>interface.14.cfg-fe-ui-indexingalert</code> | notes | 5 reader site(s) outside configs/app |
| <code>interface.14.cfg-fe-ui-maintenancealert</code> | notes | 1 reader site(s) outside configs/app |
| <code>interface.14.cfg-fe-ui-maxcontentwidth</code> | notes | 2 reader site(s) outside configs/app |
| <code>interface.14.cfg-fe-ui-nativecoinprice</code> | notes | 2 reader site(s) outside configs/app |
| <code>interface.14.cfg-fe-ui-navigation</code> | notes | 23 reader site(s) outside configs/app |
| <code>interface.14.cfg-fe-ui-views</code> | notes | 118 reader site(s) outside configs/app; 78 further callers omitted |
| <code>interface.14.cfg-fe-ui-views-address</code> | notes | 34 reader site(s) |
| <code>interface.14.cfg-fe-ui-views-block</code> | notes | 35 reader site(s) |
| <code>interface.14.cfg-fe-ui-views-nft</code> | notes | 7 reader site(s) |
| <code>interface.14.cfg-fe-ui-views-token</code> | notes | 4 reader site(s) |
| <code>interface.14.cfg-fe-ui-views-tx</code> | notes | 38 reader site(s) |

## env-account

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-fe-next-public-account-api-keys-button</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/account.ts:11</code> | <code>const value = getEnvValue('NEXT_PUBLIC_ACCOUNT_API_KEYS_BUTTON');</code> |
| <code>interface.14.env-fe-next-public-account-auth-provider</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/account.ts:34</code> | <code>const authProvider = getEnvValue('NEXT_PUBLIC_ACCOUNT_AUTH_PROVIDER');</code> |
| <code>interface.14.env-fe-next-public-account-dynamic-environment-id</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/account.ts:35</code> | <code>const dynamicEnvironmentId = getEnvValue('NEXT_PUBLIC_ACCOUNT_DYNAMIC_ENVIRONMENT_ID');</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-fe-next-public-account-api-keys-button</code> | notes | read 1x under configs; 3 other sites |
| <code>interface.14.env-fe-next-public-account-auth-provider</code> | notes | read 1x under configs; 4 other sites |
| <code>interface.14.env-fe-next-public-account-dynamic-environment-id</code> | notes | read 1x under configs; 3 other sites |

## env-ad

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-fe-next-public-ad-adbutler-config-desktop</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/adsBanner.ts:54</code> | <code>const desktopConfig = parseEnvJson&lt;AdButlerConfig&gt;(getEnvValue('NEXT_PUBLIC_AD_ADBUTLER_CONFIG_DESKTOP'));</code> |
| <code>interface.14.env-fe-next-public-ad-adbutler-config-mobile</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/adsBanner.ts:55</code> | <code>const mobileConfig = parseEnvJson&lt;AdButlerConfig&gt;(getEnvValue('NEXT_PUBLIC_AD_ADBUTLER_CONFIG_MOBILE'));</code> |
| <code>interface.14.env-fe-next-public-ad-banner-additional-provider</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/adsBanner.ts:15</code> | <code>const additionalProvider = getEnvValue('NEXT_PUBLIC_AD_BANNER_ADDITIONAL_PROVIDER') as AdBannerAdditionalProviders;</code> |
| <code>interface.14.env-fe-next-public-ad-banner-enable-specify</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/adsBanner.ts:16</code> | <code>const isSpecifyEnabled = getEnvValue('NEXT_PUBLIC_AD_BANNER_ENABLE_SPECIFY') === 'true';</code> |
| <code>interface.14.env-fe-next-public-ad-banner-provider</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/adsBanner.ts:10</code> | <code>const envValue = getEnvValue('NEXT_PUBLIC_AD_BANNER_PROVIDER') as AdBannerProviders;</code> |
| <code>interface.14.env-fe-next-public-ad-text-provider</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/adsText.ts:21</code> | <code>const envValue = getEnvValue('NEXT_PUBLIC_AD_TEXT_PROVIDER') as AdTextProviders;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-fe-next-public-ad-adbutler-config-desktop</code> | notes | read 2x under configs; 1 other sites |
| <code>interface.14.env-fe-next-public-ad-adbutler-config-mobile</code> | notes | read 2x under configs; 1 other sites |
| <code>interface.14.env-fe-next-public-ad-banner-additional-provider</code> | notes | read 1x under configs; 2 other sites |
| <code>interface.14.env-fe-next-public-ad-banner-enable-specify</code> | notes | read 1x under configs; 1 other sites |
| <code>interface.14.env-fe-next-public-ad-banner-provider</code> | notes | read 1x under configs; 6 other sites |
| <code>interface.14.env-fe-next-public-ad-text-provider</code> | notes | read 1x under configs; 4 other sites |

## env-address

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-fe-next-public-address-3rd-party-widgets</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/address3rdPartyWidgets.ts:6</code> | <code>const widgets = parseEnvJson&lt;Array&lt;string&gt;&gt;(getEnvValue('NEXT_PUBLIC_ADDRESS_3RD_PARTY_WIDGETS'));</code> |
| <code>interface.14.env-fe-next-public-address-3rd-party-widgets-config-url</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/address3rdPartyWidgets.ts:7</code> | <code>const configUrl = getExternalAssetFilePath('NEXT_PUBLIC_ADDRESS_3RD_PARTY_WIDGETS_CONFIG_URL');</code> |
| <code>interface.14.env-fe-next-public-address-username-tag</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/addressProfileAPI.ts:7</code> | <code>const value = parseEnvJson&lt;AddressProfileAPIConfig&gt;(getEnvValue('NEXT_PUBLIC_ADDRESS_USERNAME_TAG'));</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-fe-next-public-address-3rd-party-widgets</code> | notes | read 1x under configs; 4 other sites |
| <code>interface.14.env-fe-next-public-address-3rd-party-widgets-config-url</code> | notes | read 1x under configs; 10 other sites |
| <code>interface.14.env-fe-next-public-address-username-tag</code> | notes | read 1x under configs; 2 other sites |

## env-admin

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-fe-next-public-admin-service-api-host</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/apis.ts:54</code> | <code>const apiHost = getEnvValue('NEXT_PUBLIC_ADMIN_SERVICE_API_HOST');</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-fe-next-public-admin-service-api-host</code> | notes | read 1x under configs; 4 other sites |

## env-advanced

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-fe-next-public-advanced-filter-enabled</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/advancedFilter.ts:5</code> | <code>const isDisabled = getEnvValue('NEXT_PUBLIC_ADVANCED_FILTER_ENABLED') === 'false';</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-fe-next-public-advanced-filter-enabled</code> | notes | read 1x under configs; 3 other sites |

## env-api

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-fe-next-public-api-base-path</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/apis.ts:45</code> | <code>basePath: stripTrailingSlash(getEnvValue('NEXT_PUBLIC_API_BASE_PATH') &#124;&#124; ''),</code> |
| <code>interface.14.env-fe-next-public-api-docs-alert-message</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/apiDocs.ts:25</code> | <code>alertMessage: getEnvValue('NEXT_PUBLIC_API_DOCS_ALERT_MESSAGE'),</code> |
| <code>interface.14.env-fe-next-public-api-docs-tabs</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/apiDocs.ts:8</code> | <code>const value = (parseEnvJson&lt;Array&lt;ApiDocsTabId&gt;&gt;(getEnvValue('NEXT_PUBLIC_API_DOCS_TABS')) &#124;&#124; API_DOCS_TABS)</code> |
| <code>interface.14.env-fe-next-public-api-host</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/apis.ts:21</code> | <code>const apiHost = getEnvValue('NEXT_PUBLIC_API_HOST');</code> |
| <code>interface.14.env-fe-next-public-api-keys-alert-message</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/ui.ts:89</code> | <code>message: getEnvValue('NEXT_PUBLIC_API_KEYS_ALERT_MESSAGE'),</code> |
| <code>interface.14.env-fe-next-public-api-port</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/apis.ts:27</code> | <code>const apiPort = getEnvValue('NEXT_PUBLIC_API_PORT');</code> |
| <code>interface.14.env-fe-next-public-api-protocol</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/apis.ts:26</code> | <code>const apiSchema = getEnvValue('NEXT_PUBLIC_API_PROTOCOL') &#124;&#124; 'https';</code> |
| <code>interface.14.env-fe-next-public-api-websocket-protocol</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/apis.ts:35</code> | <code>const socketSchema = getEnvValue('NEXT_PUBLIC_API_WEBSOCKET_PROTOCOL') &#124;&#124; 'wss';</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-fe-next-public-api-base-path</code> | notes | read 1x under configs; 7 other sites |
| <code>interface.14.env-fe-next-public-api-docs-alert-message</code> | notes | read 1x under configs; 1 other sites |
| <code>interface.14.env-fe-next-public-api-docs-tabs</code> | notes | read 1x under configs; 3 other sites |
| <code>interface.14.env-fe-next-public-api-host</code> | notes | read 1x under configs; 13 other sites |
| <code>interface.14.env-fe-next-public-api-keys-alert-message</code> | notes | read 1x under configs; 1 other sites |
| <code>interface.14.env-fe-next-public-api-port</code> | notes | read 1x under configs; 5 other sites |
| <code>interface.14.env-fe-next-public-api-protocol</code> | notes | read 1x under configs; 9 other sites |
| <code>interface.14.env-fe-next-public-api-websocket-protocol</code> | notes | read 1x under configs; 8 other sites |

## env-app

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-fe-next-public-app-env</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/app.ts:15</code> | <code>const isDev = getEnvValue('NEXT_PUBLIC_APP_ENV') === 'development';</code> |
| <code>interface.14.env-fe-next-public-app-host</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/app.ts:7</code> | <code>const appHost = getEnvValue('NEXT_PUBLIC_APP_HOST');</code> |
| <code>interface.14.env-fe-next-public-app-instance</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/app.ts:17</code> | <code>const isPw = getEnvValue('NEXT_PUBLIC_APP_INSTANCE') === 'pw';</code> |
| <code>interface.14.env-fe-next-public-app-port</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/app.ts:5</code> | <code>const appPort = getEnvValue('NEXT_PUBLIC_APP_PORT');</code> |
| <code>interface.14.env-fe-next-public-app-protocol</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/app.ts:6</code> | <code>const appSchema = getEnvValue('NEXT_PUBLIC_APP_PROTOCOL');</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-fe-next-public-app-env</code> | notes | read 3x under configs; 12 other sites |
| <code>interface.14.env-fe-next-public-app-host</code> | notes | read 1x under configs; 15 other sites |
| <code>interface.14.env-fe-next-public-app-instance</code> | notes | read 2x under configs; 7 other sites |
| <code>interface.14.env-fe-next-public-app-port</code> | notes | read 1x under configs; 12 other sites |
| <code>interface.14.env-fe-next-public-app-protocol</code> | notes | read 1x under configs; 11 other sites |

## env-beacon

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-fe-next-public-beacon-chain-currency-symbol</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/beaconChain.ts:16</code> | <code>getEnvValue('NEXT_PUBLIC_BEACON_CHAIN_CURRENCY_SYMBOL') &#124;&#124;</code> |
| <code>interface.14.env-fe-next-public-beacon-chain-validator-url-template</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/beaconChain.ts:9</code> | <code>const validatorUrlTemplate = getEnvValue('NEXT_PUBLIC_BEACON_CHAIN_VALIDATOR_URL_TEMPLATE');</code> |
| <code>interface.14.env-fe-next-public-beacon-chain-withdrawals-only</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/beaconChain.ts:10</code> | <code>const withdrawalsOnly = getEnvValue('NEXT_PUBLIC_BEACON_CHAIN_WITHDRAWALS_ONLY') === 'true';</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-fe-next-public-beacon-chain-currency-symbol</code> | notes | read 1x under configs; 2 other sites |
| <code>interface.14.env-fe-next-public-beacon-chain-validator-url-template</code> | notes | read 1x under configs; 3 other sites |
| <code>interface.14.env-fe-next-public-beacon-chain-withdrawals-only</code> | notes | read 1x under configs; 3 other sites |

## env-bridged

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-fe-next-public-bridged-tokens-bridges</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/bridgedTokens.ts:10</code> | <code>const bridges = parseEnvJson&lt;Array&lt;TokenBridge&gt;&gt;(getEnvValue('NEXT_PUBLIC_BRIDGED_TOKENS_BRIDGES'));</code> |
| <code>interface.14.env-fe-next-public-bridged-tokens-chains</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/bridgedTokens.ts:9</code> | <code>const chains = parseEnvJson&lt;Array&lt;BridgedTokenChain&gt;&gt;(getEnvValue('NEXT_PUBLIC_BRIDGED_TOKENS_CHAINS'));</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-fe-next-public-bridged-tokens-bridges</code> | notes | read 1x under configs; 3 other sites |
| <code>interface.14.env-fe-next-public-bridged-tokens-chains</code> | notes | read 1x under configs; 4 other sites |

## env-celo

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-fe-next-public-celo-enabled</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/celo.ts:9</code> | <code>if (getEnvValue('NEXT_PUBLIC_CELO_ENABLED') === 'true') {</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-fe-next-public-celo-enabled</code> | notes | read 1x under configs; 3 other sites |

## env-clusters

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-fe-next-public-clusters-api-host</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/apis.ts:197</code> | <code>const apiHost = getEnvValue('NEXT_PUBLIC_CLUSTERS_API_HOST');</code> |
| <code>interface.14.env-fe-next-public-clusters-cdn-url</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/nameServices.ts:19</code> | <code>cdnUrl: getEnvValue('NEXT_PUBLIC_CLUSTERS_CDN_URL') &#124;&#124; '&lt;url&gt;',</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-fe-next-public-clusters-api-host</code> | notes | read 1x under configs; 7 other sites |
| <code>interface.14.env-fe-next-public-clusters-cdn-url</code> | notes | read 1x under configs; 6 other sites |

## env-color

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-fe-next-public-color-theme-default</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/ui.ts:36</code> | <code>const envValue = getEnvValue('NEXT_PUBLIC_COLOR_THEME_DEFAULT') as ColorThemeId &#124; undefined;</code> |
| <code>interface.14.env-fe-next-public-color-theme-overrides</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/ui.ts:100</code> | <code>overrides: parseEnvJson&lt;Record&lt;string, unknown&gt;&gt;(getEnvValue('NEXT_PUBLIC_COLOR_THEME_OVERRIDES')) &#124;&#124; {},</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-fe-next-public-color-theme-default</code> | notes | read 1x under configs; 4 other sites |
| <code>interface.14.env-fe-next-public-color-theme-overrides</code> | notes | read 1x under configs; 7 other sites |

## env-contract

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-fe-next-public-contract-code-ides</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/ui.ts:95</code> | <code>items: parseEnvJson&lt;Array&lt;ContractCodeIde&gt;&gt;(getEnvValue('NEXT_PUBLIC_CONTRACT_CODE_IDES')) &#124;&#124; [],</code> |
| <code>interface.14.env-fe-next-public-contract-info-api-host</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/apis.ts:76</code> | <code>const apiHost = getEnvValue('NEXT_PUBLIC_CONTRACT_INFO_API_HOST');</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-fe-next-public-contract-code-ides</code> | notes | read 1x under configs; 2 other sites |
| <code>interface.14.env-fe-next-public-contract-info-api-host</code> | notes | read 1x under configs; 5 other sites |

## env-cross

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-fe-next-public-cross-chain-txs-enabled</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/crossChainTxs.ts:9</code> | <code>if (getEnvValue('NEXT_PUBLIC_CROSS_CHAIN_TXS_ENABLED') === 'true' &amp;&amp; apis.interchainIndexer) {</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-fe-next-public-cross-chain-txs-enabled</code> | notes | read 1x under configs; 6 other sites |

## env-data

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-fe-next-public-data-availability-enabled</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/dataAvailability.ts:8</code> | <code>if (getEnvValue('NEXT_PUBLIC_DATA_AVAILABILITY_ENABLED') === 'true') {</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-fe-next-public-data-availability-enabled</code> | notes | read 1x under configs; 5 other sites |

## env-defi

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-fe-next-public-defi-dropdown-items</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/deFiDropdown.ts:7</code> | <code>const items = parseEnvJson&lt;Array&lt;DeFiDropdownItem&gt;&gt;(getEnvValue('NEXT_PUBLIC_DEFI_DROPDOWN_ITEMS')) &#124;&#124; [];</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-fe-next-public-defi-dropdown-items</code> | notes | read 1x under configs; 4 other sites |

## env-dex

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-fe-next-public-dex-pools-enabled</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/pools.ts:6</code> | <code>const dexPoolsEnabled = getEnvValue('NEXT_PUBLIC_DEX_POOLS_ENABLED') === 'true';</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-fe-next-public-dex-pools-enabled</code> | notes | read 1x under configs; 3 other sites |

## env-fault

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-fe-next-public-fault-proof-enabled</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/faultProofSystem.ts:9</code> | <code>if (rollup.isEnabled &amp;&amp; rollup.type === 'optimistic' &amp;&amp; getEnvValue('NEXT_PUBLIC_FAULT_PROOF_ENABLED') === 'true') {</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-fe-next-public-fault-proof-enabled</code> | notes | read 1x under configs; 3 other sites |

## env-featured

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-fe-next-public-featured-networks</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/ui.ts:61</code> | <code>items: getExternalAssetFilePath('NEXT_PUBLIC_FEATURED_NETWORKS'),</code> |
| <code>interface.14.env-fe-next-public-featured-networks-all-link</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/ui.ts:62</code> | <code>allLink: getEnvValue('NEXT_PUBLIC_FEATURED_NETWORKS_ALL_LINK'),</code> |
| <code>interface.14.env-fe-next-public-featured-networks-mode</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/ui.ts:63</code> | <code>mode: (getEnvValue('NEXT_PUBLIC_FEATURED_NETWORKS_MODE') &#124;&#124; 'list') as 'tabs' &#124; 'list',</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-fe-next-public-featured-networks</code> | notes | read 1x under configs; 14 other sites |
| <code>interface.14.env-fe-next-public-featured-networks-all-link</code> | notes | read 1x under configs; 5 other sites |
| <code>interface.14.env-fe-next-public-featured-networks-mode</code> | notes | read 1x under configs; 3 other sites |

## env-flashblocks

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-fe-next-public-flashblocks-socket-url</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/flashblocks.ts:8</code> | <code>const socketUrl = getEnvValue('NEXT_PUBLIC_FLASHBLOCKS_SOCKET_URL');</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-fe-next-public-flashblocks-socket-url</code> | notes | read 1x under configs; 1 other sites |

## env-font

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-fe-next-public-font-family-body</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/ui.ts:104</code> | <code>body: parseEnvJson&lt;FontFamily&gt;(getEnvValue('NEXT_PUBLIC_FONT_FAMILY_BODY')),</code> |
| <code>interface.14.env-fe-next-public-font-family-heading</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/ui.ts:103</code> | <code>heading: parseEnvJson&lt;FontFamily&gt;(getEnvValue('NEXT_PUBLIC_FONT_FAMILY_HEADING')),</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-fe-next-public-font-family-body</code> | notes | read 1x under configs; 6 other sites |
| <code>interface.14.env-fe-next-public-font-family-heading</code> | notes | read 1x under configs; 6 other sites |

## env-footer

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-fe-next-public-footer-links</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/ui.ts:66</code> | <code>links: getExternalAssetFilePath('NEXT_PUBLIC_FOOTER_LINKS'),</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-fe-next-public-footer-links</code> | notes | read 1x under configs; 10 other sites |

## env-game

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-fe-next-public-game-badge-claim-link</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/easterEggBadge.ts:5</code> | <code>const badgeClaimLink = getEnvValue('NEXT_PUBLIC_GAME_BADGE_CLAIM_LINK');</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-fe-next-public-game-badge-claim-link</code> | notes | read 1x under configs; 1 other sites |

## env-gas

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-fe-next-public-gas-refuel-provider-config</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/getGasButton.ts:8</code> | <code>const value = parseEnvJson&lt;GasRefuelProviderConfig&gt;(getEnvValue('NEXT_PUBLIC_GAS_REFUEL_PROVIDER_CONFIG'));</code> |
| <code>interface.14.env-fe-next-public-gas-tracker-enabled</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/gasTracker.ts:8</code> | <code>const isDisabled = getEnvValue('NEXT_PUBLIC_GAS_TRACKER_ENABLED') === 'false';</code> |
| <code>interface.14.env-fe-next-public-gas-tracker-units</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/gasTracker.ts:11</code> | <code>const envValue = getEnvValue('NEXT_PUBLIC_GAS_TRACKER_UNITS');</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-fe-next-public-gas-refuel-provider-config</code> | notes | read 1x under configs; 3 other sites |
| <code>interface.14.env-fe-next-public-gas-tracker-enabled</code> | notes | read 1x under configs; 3 other sites |
| <code>interface.14.env-fe-next-public-gas-tracker-units</code> | notes | read 1x under configs; 3 other sites |

## env-git

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-fe-next-public-git-commit-sha</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/rollbar.ts:16</code> | <code>const codeVersion = getEnvValue('NEXT_PUBLIC_GIT_TAG') &#124;&#124; getEnvValue('NEXT_PUBLIC_GIT_COMMIT_SHA');</code> |
| <code>interface.14.env-fe-next-public-git-tag</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/rollbar.ts:16</code> | <code>const codeVersion = getEnvValue('NEXT_PUBLIC_GIT_TAG') &#124;&#124; getEnvValue('NEXT_PUBLIC_GIT_COMMIT_SHA');</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-fe-next-public-git-commit-sha</code> | notes | read 2x under configs; 9 other sites |
| <code>interface.14.env-fe-next-public-git-tag</code> | notes | read 2x under configs; 10 other sites |

## env-google

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-fe-next-public-google-analytics-property-id</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/googleAnalytics.ts:6</code> | <code>const propertyId = getEnvValue('NEXT_PUBLIC_GOOGLE_ANALYTICS_PROPERTY_ID');</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-fe-next-public-google-analytics-property-id</code> | notes | read 1x under configs; 4 other sites |

## env-growth

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-fe-next-public-growth-book-client-key</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/growthBook.ts:6</code> | <code>const clientKey = getEnvValue('NEXT_PUBLIC_GROWTH_BOOK_CLIENT_KEY');</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-fe-next-public-growth-book-client-key</code> | notes | read 1x under configs; 3 other sites |

## env-has

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-fe-next-public-has-beacon-chain</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/beaconChain.ts:8</code> | <code>if (getEnvValue('NEXT_PUBLIC_HAS_BEACON_CHAIN') === 'true') {</code> |
| <code>interface.14.env-fe-next-public-has-contract-audit-reports</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/ui.ts:97</code> | <code>hasContractAuditReports: getEnvValue('NEXT_PUBLIC_HAS_CONTRACT_AUDIT_REPORTS') === 'true' ? true : false,</code> |
| <code>interface.14.env-fe-next-public-has-mud-framework</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/mudFramework.ts:9</code> | <code>if (rollup.isEnabled &amp;&amp; rollup.type === 'optimistic' &amp;&amp; getEnvValue('NEXT_PUBLIC_HAS_MUD_FRAMEWORK') === 'true') {</code> |
| <code>interface.14.env-fe-next-public-has-user-ops</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/userOps.ts:8</code> | <code>if (getEnvValue('NEXT_PUBLIC_HAS_USER_OPS') === 'true') {</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-fe-next-public-has-beacon-chain</code> | notes | read 1x under configs; 12 other sites |
| <code>interface.14.env-fe-next-public-has-contract-audit-reports</code> | notes | read 1x under configs; 4 other sites |
| <code>interface.14.env-fe-next-public-has-mud-framework</code> | notes | read 1x under configs; 3 other sites |
| <code>interface.14.env-fe-next-public-has-user-ops</code> | notes | read 1x under configs; 6 other sites |

## env-helia

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-fe-next-public-helia-verified-fetch-enabled</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/ui/views/nft.ts:19</code> | <code>isEnabled: getEnvValue('NEXT_PUBLIC_HELIA_VERIFIED_FETCH_ENABLED') === 'false' ? false : true,</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-fe-next-public-helia-verified-fetch-enabled</code> | notes | read 1x under configs; 1 other sites |

## env-hide

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-fe-next-public-hide-indexing-alert-blocks</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/ui.ts:79</code> | <code>isHidden: getEnvValue('NEXT_PUBLIC_HIDE_INDEXING_ALERT_BLOCKS') === 'true' ? true : false,</code> |
| <code>interface.14.env-fe-next-public-hide-indexing-alert-int-txs</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/ui.ts:82</code> | <code>isHidden: getEnvValue('NEXT_PUBLIC_HIDE_INDEXING_ALERT_INT_TXS') === 'true' ? true : false,</code> |
| <code>interface.14.env-fe-next-public-hide-native-coin-price</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/ui.ts:108</code> | <code>isHidden: getEnvValue('NEXT_PUBLIC_HIDE_NATIVE_COIN_PRICE') === 'true' ? true : false,</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-fe-next-public-hide-indexing-alert-blocks</code> | notes | read 1x under configs; 2 other sites |
| <code>interface.14.env-fe-next-public-hide-indexing-alert-int-txs</code> | notes | read 1x under configs; 2 other sites |
| <code>interface.14.env-fe-next-public-hide-native-coin-price</code> | notes | read 1x under configs; 3 other sites |

## env-homepage

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-fe-next-public-homepage-charts</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/ui.ts:71</code> | <code>charts: parseEnvJson&lt;Array&lt;ChainIndicatorId&gt;&gt;(getEnvValue('NEXT_PUBLIC_HOMEPAGE_CHARTS')) &#124;&#124; [],</code> |
| <code>interface.14.env-fe-next-public-homepage-hero-banner-config</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/ui.ts:73</code> | <code>heroBanner: parseEnvJson&lt;HeroBannerConfig&gt;(getEnvValue('NEXT_PUBLIC_HOMEPAGE_HERO_BANNER_CONFIG')),</code> |
| <code>interface.14.env-fe-next-public-homepage-highlights-config</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/ui.ts:74</code> | <code>highlights: getExternalAssetFilePath('NEXT_PUBLIC_HOMEPAGE_HIGHLIGHTS_CONFIG'),</code> |
| <code>interface.14.env-fe-next-public-homepage-stats</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/ui.ts:15</code> | <code>const parsedValue = parseEnvJson&lt;Array&lt;HomeStatsWidgetId&gt;&gt;(getEnvValue('NEXT_PUBLIC_HOMEPAGE_STATS'));</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-fe-next-public-homepage-charts</code> | notes | read 1x under configs; 9 other sites |
| <code>interface.14.env-fe-next-public-homepage-hero-banner-config</code> | notes | read 1x under configs; 6 other sites |
| <code>interface.14.env-fe-next-public-homepage-highlights-config</code> | notes | read 1x under configs; 7 other sites |
| <code>interface.14.env-fe-next-public-homepage-stats</code> | notes | read 1x under configs; 7 other sites |

## env-hostname

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-fe-hostname</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/essential-dapps-chains/config.edge.ts:7</code> | <code>return process.env.HOSTNAME !== undefined;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-fe-hostname</code> | notes | read 4x under configs; 14 other sites; runtime-provided, not a NEXT_PUBLIC setting |

## env-hot

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-fe-next-public-hot-contracts-enabled</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/hotContracts.ts:8</code> | <code>if (getEnvValue('NEXT_PUBLIC_HOT_CONTRACTS_ENABLED') === 'true') {</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-fe-next-public-hot-contracts-enabled</code> | notes | read 1x under configs; 1 other sites |

## env-icon

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-fe-next-public-icon-sprite-hash</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/app.ts:18</code> | <code>const spriteHash = getEnvValue('NEXT_PUBLIC_ICON_SPRITE_HASH');</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-fe-next-public-icon-sprite-hash</code> | notes | read 1x under configs; 4 other sites |

## env-interchain

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-fe-next-public-interchain-indexer-api-host</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/apis.ts:87</code> | <code>const apiHost = getEnvValue('NEXT_PUBLIC_INTERCHAIN_INDEXER_API_HOST');</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-fe-next-public-interchain-indexer-api-host</code> | notes | read 1x under configs; 5 other sites |

## env-interop

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-fe-next-public-interop-enabled</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/rollup.ts:55</code> | <code>interopEnabled: type === 'optimistic' &amp;&amp; getEnvValue('NEXT_PUBLIC_INTEROP_ENABLED') === 'true',</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-fe-next-public-interop-enabled</code> | notes | read 1x under configs; 4 other sites |

## env-is

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-fe-next-public-is-account-supported</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/account.ts:32</code> | <code>getEnvValue('NEXT_PUBLIC_IS_ACCOUNT_SUPPORTED') === 'true'</code> |
| <code>interface.14.env-fe-next-public-is-suave-chain</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/suave.ts:8</code> | <code>if (getEnvValue('NEXT_PUBLIC_IS_SUAVE_CHAIN') === 'true') {</code> |
| <code>interface.14.env-fe-next-public-is-testnet</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/chain.ts:54</code> | <code>isTestnet: getEnvValue('NEXT_PUBLIC_IS_TESTNET') === 'true',</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-fe-next-public-is-account-supported</code> | notes | read 1x under configs; 28 other sites |
| <code>interface.14.env-fe-next-public-is-suave-chain</code> | notes | read 1x under configs; 1 other sites |
| <code>interface.14.env-fe-next-public-is-testnet</code> | notes | read 1x under configs; 8 other sites |

## env-maintenance

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-fe-next-public-maintenance-alert-message</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/ui.ts:86</code> | <code>message: getEnvValue('NEXT_PUBLIC_MAINTENANCE_ALERT_MESSAGE'),</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-fe-next-public-maintenance-alert-message</code> | notes | read 1x under configs; 2 other sites |

## env-marketplace

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-fe-next-public-marketplace-banner-content-url</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/marketplace.ts:25</code> | <code>const bannerContentUrl = getExternalAssetFilePath('NEXT_PUBLIC_MARKETPLACE_BANNER_CONTENT_URL');</code> |
| <code>interface.14.env-fe-next-public-marketplace-banner-link-url</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/marketplace.ts:26</code> | <code>const bannerLinkUrl = getEnvValue('NEXT_PUBLIC_MARKETPLACE_BANNER_LINK_URL');</code> |
| <code>interface.14.env-fe-next-public-marketplace-categories-url</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/marketplace.ts:23</code> | <code>const categoriesUrl = getExternalAssetFilePath('NEXT_PUBLIC_MARKETPLACE_CATEGORIES_URL');</code> |
| <code>interface.14.env-fe-next-public-marketplace-config-url</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/marketplace.ts:20</code> | <code>const configUrl = getExternalAssetFilePath('NEXT_PUBLIC_MARKETPLACE_CONFIG_URL');</code> |
| <code>interface.14.env-fe-next-public-marketplace-enabled</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/marketplace.ts:19</code> | <code>const enabled = getEnvValue('NEXT_PUBLIC_MARKETPLACE_ENABLED');</code> |
| <code>interface.14.env-fe-next-public-marketplace-essential-dapps-ad-enabled</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/marketplace.ts:29</code> | <code>const essentialDappsAdEnabled = getEnvValue('NEXT_PUBLIC_MARKETPLACE_ESSENTIAL_DAPPS_AD_ENABLED') !== 'false';</code> |
| <code>interface.14.env-fe-next-public-marketplace-essential-dapps-config</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/marketplace.ts:28</code> | <code>const essentialDappsConfig = parseEnvJson&lt;EssentialDappsConfig&gt;(getEnvValue('NEXT_PUBLIC_MARKETPLACE_ESSENTIAL_DAPPS_CONFIG'));</code> |
| <code>interface.14.env-fe-next-public-marketplace-featured-app</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/marketplace.ts:24</code> | <code>const featuredApp = getEnvValue('NEXT_PUBLIC_MARKETPLACE_FEATURED_APP');</code> |
| <code>interface.14.env-fe-next-public-marketplace-graph-links-url</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/marketplace.ts:27</code> | <code>const graphLinksUrl = getExternalAssetFilePath('NEXT_PUBLIC_MARKETPLACE_GRAPH_LINKS_URL');</code> |
| <code>interface.14.env-fe-next-public-marketplace-submit-form</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/marketplace.ts:21</code> | <code>const submitFormUrl = getEnvValue('NEXT_PUBLIC_MARKETPLACE_SUBMIT_FORM');</code> |
| <code>interface.14.env-fe-next-public-marketplace-suggest-ideas-form</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/marketplace.ts:22</code> | <code>const suggestIdeasFormUrl = getEnvValue('NEXT_PUBLIC_MARKETPLACE_SUGGEST_IDEAS_FORM');</code> |
| <code>interface.14.env-fe-next-public-marketplace-titles</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/marketplace.ts:30</code> | <code>const customTitles = parseEnvJson(getEnvValue('NEXT_PUBLIC_MARKETPLACE_TITLES')) &#124;&#124; {};</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-fe-next-public-marketplace-banner-content-url</code> | notes | read 1x under configs; 7 other sites |
| <code>interface.14.env-fe-next-public-marketplace-banner-link-url</code> | notes | read 1x under configs; 6 other sites |
| <code>interface.14.env-fe-next-public-marketplace-categories-url</code> | notes | read 1x under configs; 4 other sites |
| <code>interface.14.env-fe-next-public-marketplace-config-url</code> | notes | read 1x under configs; 28 other sites |
| <code>interface.14.env-fe-next-public-marketplace-enabled</code> | notes | read 1x under configs; 36 other sites |
| <code>interface.14.env-fe-next-public-marketplace-essential-dapps-ad-enabled</code> | notes | read 1x under configs; 2 other sites |
| <code>interface.14.env-fe-next-public-marketplace-essential-dapps-config</code> | notes | read 1x under configs; 10 other sites |
| <code>interface.14.env-fe-next-public-marketplace-featured-app</code> | notes | read 1x under configs; 4 other sites |
| <code>interface.14.env-fe-next-public-marketplace-graph-links-url</code> | notes | read 1x under configs; 4 other sites |
| <code>interface.14.env-fe-next-public-marketplace-submit-form</code> | notes | read 1x under configs; 3 other sites |
| <code>interface.14.env-fe-next-public-marketplace-suggest-ideas-form</code> | notes | read 1x under configs; 2 other sites |
| <code>interface.14.env-fe-next-public-marketplace-titles</code> | notes | read 1x under configs; 3 other sites |

## env-max

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-fe-next-public-max-content-width-enabled</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/ui.ts:106</code> | <code>maxContentWidth: getEnvValue('NEXT_PUBLIC_MAX_CONTENT_WIDTH_ENABLED') === 'false' ? false : true,</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-fe-next-public-max-content-width-enabled</code> | notes | read 1x under configs; 1 other sites |

## env-mega

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-fe-next-public-mega-eth-socket-url-metrics</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/megaEth.ts:7</code> | <code>const socketUrlMetrics = getEnvValue('NEXT_PUBLIC_MEGA_ETH_SOCKET_URL_METRICS');</code> |
| <code>interface.14.env-fe-next-public-mega-eth-socket-url-rpc</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/megaEth.ts:8</code> | <code>const socketUrlRpc = getEnvValue('NEXT_PUBLIC_MEGA_ETH_SOCKET_URL_RPC');</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-fe-next-public-mega-eth-socket-url-metrics</code> | notes | read 1x under configs; 3 other sites |
| <code>interface.14.env-fe-next-public-mega-eth-socket-url-rpc</code> | notes | read 1x under configs; 1 other sites |

## env-metadata

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-fe-next-public-metadata-address-tags-update-enabled</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/addressMetadata.ts:13</code> | <code>isAddressTagsUpdateEnabled: getEnvValue('NEXT_PUBLIC_METADATA_ADDRESS_TAGS_UPDATE_ENABLED') !== 'false',</code> |
| <code>interface.14.env-fe-next-public-metadata-service-api-host</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/apis.ts:98</code> | <code>const apiHost = getEnvValue('NEXT_PUBLIC_METADATA_SERVICE_API_HOST');</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-fe-next-public-metadata-address-tags-update-enabled</code> | notes | read 1x under configs; 2 other sites |
| <code>interface.14.env-fe-next-public-metadata-service-api-host</code> | notes | read 1x under configs; 6 other sites |

## env-metasuites

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-fe-next-public-metasuites-enabled</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/metasuites.ts:8</code> | <code>if (getEnvValue('NEXT_PUBLIC_METASUITES_ENABLED') === 'true') {</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-fe-next-public-metasuites-enabled</code> | notes | read 1x under configs; 1 other sites |

## env-mixpanel

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-fe-next-public-mixpanel-config-overrides</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/mixpanel.ts:10</code> | <code>const value = getEnvValue('NEXT_PUBLIC_MIXPANEL_CONFIG_OVERRIDES');</code> |
| <code>interface.14.env-fe-next-public-mixpanel-project-token</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/mixpanel.ts:8</code> | <code>const projectToken = getEnvValue('NEXT_PUBLIC_MIXPANEL_PROJECT_TOKEN');</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-fe-next-public-mixpanel-config-overrides</code> | notes | read 1x under configs; 2 other sites |
| <code>interface.14.env-fe-next-public-mixpanel-project-token</code> | notes | read 1x under configs; 6 other sites |

## env-multichain

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-fe-next-public-multichain-aggregator-api-host</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/apis.ts:120</code> | <code>const apiHost = getEnvValue('NEXT_PUBLIC_MULTICHAIN_AGGREGATOR_API_HOST');</code> |
| <code>interface.14.env-fe-next-public-multichain-balance-provider-config</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/multichainButton.ts:7</code> | <code>const value = parseEnvJson&lt;Array&lt;MultichainProviderConfig&gt;&gt;(getEnvValue('NEXT_PUBLIC_MULTICHAIN_BALANCE_PROVIDER_CONFIG'));</code> |
| <code>interface.14.env-fe-next-public-multichain-cluster</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/apis.ts:121</code> | <code>const cluster = getEnvValue('NEXT_PUBLIC_MULTICHAIN_CLUSTER');</code> |
| <code>interface.14.env-fe-next-public-multichain-enabled</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/multichain.ts:6</code> | <code>const isEnabled = getEnvValue('NEXT_PUBLIC_MULTICHAIN_ENABLED') === 'true';</code> |
| <code>interface.14.env-fe-next-public-multichain-stats-api-host</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/apis.ts:140</code> | <code>const apiHost = getEnvValue('NEXT_PUBLIC_MULTICHAIN_STATS_API_HOST');</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-fe-next-public-multichain-aggregator-api-host</code> | notes | read 1x under configs; 5 other sites |
| <code>interface.14.env-fe-next-public-multichain-balance-provider-config</code> | notes | read 1x under configs; 5 other sites |
| <code>interface.14.env-fe-next-public-multichain-cluster</code> | notes | read 2x under configs; 3 other sites |
| <code>interface.14.env-fe-next-public-multichain-enabled</code> | notes | read 1x under configs; 5 other sites |
| <code>interface.14.env-fe-next-public-multichain-stats-api-host</code> | notes | read 1x under configs; 2 other sites |

## env-name

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-fe-next-public-name-service-api-host</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/apis.ts:65</code> | <code>const apiHost = getEnvValue('NEXT_PUBLIC_NAME_SERVICE_API_HOST');</code> |
| <code>interface.14.env-fe-next-public-name-service-protocols</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/nameServices.ts:15</code> | <code>protocols: parseEnvJson&lt;Array&lt;string&gt;&gt;(getEnvValue('NEXT_PUBLIC_NAME_SERVICE_PROTOCOLS')) &#124;&#124; [ 'ens' ],</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-fe-next-public-name-service-api-host</code> | notes | read 1x under configs; 6 other sites |
| <code>interface.14.env-fe-next-public-name-service-protocols</code> | notes | read 1x under configs; 3 other sites |

## env-navigation

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-fe-next-public-navigation-highlighted-routes</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/ui.ts:31</code> | <code>const parsedValue = parseEnvJson&lt;Array&lt;string&gt;&gt;(getEnvValue('NEXT_PUBLIC_NAVIGATION_HIGHLIGHTED_ROUTES'));</code> |
| <code>interface.14.env-fe-next-public-navigation-layout</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/ui.ts:57</code> | <code>layout: (getEnvValue('NEXT_PUBLIC_NAVIGATION_LAYOUT') &#124;&#124; 'vertical') as NavigationLayout,</code> |
| <code>interface.14.env-fe-next-public-navigation-promo-banner-config</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/ui.ts:41</code> | <code>const envValue = parseEnvJson&lt;NavigationPromoBannerConfig&gt;(getEnvValue('NEXT_PUBLIC_NAVIGATION_PROMO_BANNER_CONFIG'));</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-fe-next-public-navigation-highlighted-routes</code> | notes | read 1x under configs; 4 other sites |
| <code>interface.14.env-fe-next-public-navigation-layout</code> | notes | read 1x under configs; 24 other sites |
| <code>interface.14.env-fe-next-public-navigation-promo-banner-config</code> | notes | read 1x under configs; 4 other sites |

## env-network

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-fe-next-public-network-additional-token-types</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/chain.ts:52</code> | <code>additionalTokenTypes: parseEnvJson&lt;Array&lt;AdditionalTokenType&gt;&gt;(getEnvValue('NEXT_PUBLIC_NETWORK_ADDITIONAL_TOKEN_TYPES')) &#124;&#124; [],</code> |
| <code>interface.14.env-fe-next-public-network-currency-decimals</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/chain.ts:45</code> | <code>decimals: Number(getEnvValue('NEXT_PUBLIC_NETWORK_CURRENCY_DECIMALS')) &#124;&#124; DEFAULT_CURRENCY_DECIMALS,</code> |
| <code>interface.14.env-fe-next-public-network-currency-gwei-name</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/chain.ts:43</code> | <code>gweiName: getEnvValue('NEXT_PUBLIC_NETWORK_CURRENCY_GWEI_NAME'),</code> |
| <code>interface.14.env-fe-next-public-network-currency-name</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/chain.ts:41</code> | <code>name: getEnvValue('NEXT_PUBLIC_NETWORK_CURRENCY_NAME'),</code> |
| <code>interface.14.env-fe-next-public-network-currency-symbol</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/chain.ts:44</code> | <code>symbol: getEnvValue('NEXT_PUBLIC_NETWORK_CURRENCY_SYMBOL'),</code> |
| <code>interface.14.env-fe-next-public-network-currency-wei-name</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/chain.ts:42</code> | <code>weiName: getEnvValue('NEXT_PUBLIC_NETWORK_CURRENCY_WEI_NAME'),</code> |
| <code>interface.14.env-fe-next-public-network-explorers</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/ui.ts:92</code> | <code>items: parseEnvJson&lt;Array&lt;NetworkExplorer&gt;&gt;(getEnvValue('NEXT_PUBLIC_NETWORK_EXPLORERS')) &#124;&#124; [],</code> |
| <code>interface.14.env-fe-next-public-network-icon</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/ui.ts:52</code> | <code>'default': getExternalAssetFilePath('NEXT_PUBLIC_NETWORK_ICON'),</code> |
| <code>interface.14.env-fe-next-public-network-icon-dark</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/ui.ts:53</code> | <code>dark: getExternalAssetFilePath('NEXT_PUBLIC_NETWORK_ICON_DARK'),</code> |
| <code>interface.14.env-fe-next-public-network-id</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/chain.ts:37</code> | <code>id: getEnvValue('NEXT_PUBLIC_NETWORK_ID'),</code> |
| <code>interface.14.env-fe-next-public-network-logo</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/ui.ts:48</code> | <code>'default': getExternalAssetFilePath('NEXT_PUBLIC_NETWORK_LOGO'),</code> |
| <code>interface.14.env-fe-next-public-network-logo-dark</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/ui.ts:49</code> | <code>dark: getExternalAssetFilePath('NEXT_PUBLIC_NETWORK_LOGO_DARK'),</code> |
| <code>interface.14.env-fe-next-public-network-multiple-gas-currencies</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/chain.ts:50</code> | <code>hasMultipleGasCurrencies: getEnvValue('NEXT_PUBLIC_NETWORK_MULTIPLE_GAS_CURRENCIES') === 'true',</code> |
| <code>interface.14.env-fe-next-public-network-name</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/chain.ts:38</code> | <code>name: getEnvValue('NEXT_PUBLIC_NETWORK_NAME'),</code> |
| <code>interface.14.env-fe-next-public-network-rpc-url</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/chain.ts:24</code> | <code>const envValue = getEnvValue('NEXT_PUBLIC_NETWORK_RPC_URL');</code> |
| <code>interface.14.env-fe-next-public-network-secondary-coin-symbol</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/chain.ts:48</code> | <code>symbol: getEnvValue('NEXT_PUBLIC_NETWORK_SECONDARY_COIN_SYMBOL'),</code> |
| <code>interface.14.env-fe-next-public-network-short-name</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/chain.ts:39</code> | <code>shortName: getEnvValue('NEXT_PUBLIC_NETWORK_SHORT_NAME'),</code> |
| <code>interface.14.env-fe-next-public-network-token-standard-name</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/chain.ts:51</code> | <code>tokenStandard: getEnvValue('NEXT_PUBLIC_NETWORK_TOKEN_STANDARD_NAME') &#124;&#124; 'ERC',</code> |
| <code>interface.14.env-fe-next-public-network-verification-type</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/chain.ts:20</code> | <code>return getEnvValue('NEXT_PUBLIC_NETWORK_VERIFICATION_TYPE') as NetworkVerificationTypeEnvs &#124;&#124; 'mining';</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-fe-next-public-network-additional-token-types</code> | notes | read 1x under configs; 3 other sites |
| <code>interface.14.env-fe-next-public-network-currency-decimals</code> | notes | read 1x under configs; 8 other sites |
| <code>interface.14.env-fe-next-public-network-currency-gwei-name</code> | notes | read 1x under configs; 1 other sites |
| <code>interface.14.env-fe-next-public-network-currency-name</code> | notes | read 1x under configs; 6 other sites |
| <code>interface.14.env-fe-next-public-network-currency-symbol</code> | notes | read 2x under configs; 8 other sites |
| <code>interface.14.env-fe-next-public-network-currency-wei-name</code> | notes | read 1x under configs; 1 other sites |
| <code>interface.14.env-fe-next-public-network-explorers</code> | notes | read 1x under configs; 3 other sites |
| <code>interface.14.env-fe-next-public-network-icon</code> | notes | read 1x under configs; 16 other sites |
| <code>interface.14.env-fe-next-public-network-icon-dark</code> | notes | read 1x under configs; 10 other sites |
| <code>interface.14.env-fe-next-public-network-id</code> | notes | read 1x under configs; 17 other sites |
| <code>interface.14.env-fe-next-public-network-logo</code> | notes | read 1x under configs; 15 other sites |
| <code>interface.14.env-fe-next-public-network-logo-dark</code> | notes | read 1x under configs; 10 other sites |
| <code>interface.14.env-fe-next-public-network-multiple-gas-currencies</code> | notes | read 1x under configs; 1 other sites |
| <code>interface.14.env-fe-next-public-network-name</code> | notes | read 1x under configs; 15 other sites |
| <code>interface.14.env-fe-next-public-network-rpc-url</code> | notes | read 1x under configs; 11 other sites |
| <code>interface.14.env-fe-next-public-network-secondary-coin-symbol</code> | notes | read 1x under configs; 3 other sites |
| <code>interface.14.env-fe-next-public-network-short-name</code> | notes | read 1x under configs; 7 other sites |
| <code>interface.14.env-fe-next-public-network-token-standard-name</code> | notes | read 1x under configs; 1 other sites |
| <code>interface.14.env-fe-next-public-network-verification-type</code> | notes | read 1x under configs; 5 other sites |

## env-next

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-fe-next-runtime</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/essential-dapps-chains/config.edge.ts:13</code> | <code>if (process.env.NEXT_RUNTIME !== 'edge') {</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-fe-next-runtime</code> | notes | read 4x under configs; 4 other sites; runtime-provided, not a NEXT_PUBLIC setting |

## env-og

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-fe-next-public-og-description</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/meta.ts:9</code> | <code>description: getEnvValue('NEXT_PUBLIC_OG_DESCRIPTION') &#124;&#124; '',</code> |
| <code>interface.14.env-fe-next-public-og-enhanced-data-enabled</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/meta.ts:11</code> | <code>enhancedDataEnabled: getEnvValue('NEXT_PUBLIC_OG_ENHANCED_DATA_ENABLED') === 'true',</code> |
| <code>interface.14.env-fe-next-public-og-image-url</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/meta.ts:10</code> | <code>imageUrl: app.origin + (getExternalAssetFilePath('NEXT_PUBLIC_OG_IMAGE_URL') &#124;&#124; defaultImageUrl),</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-fe-next-public-og-description</code> | notes | read 1x under configs; 6 other sites |
| <code>interface.14.env-fe-next-public-og-enhanced-data-enabled</code> | notes | read 1x under configs; 2 other sites |
| <code>interface.14.env-fe-next-public-og-image-url</code> | notes | read 1x under configs; 5 other sites |

## env-other

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-fe-next-public-other-links</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/ui.ts:56</code> | <code>otherLinks: parseEnvJson&lt;Array&lt;NavItemExternal&gt;&gt;(getEnvValue('NEXT_PUBLIC_OTHER_LINKS')) &#124;&#124; [],</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-fe-next-public-other-links</code> | notes | read 1x under configs; 6 other sites |

## env-paxeer

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-fe-next-public-paxeer-x-enabled</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/paxeerXFeed.ts:10</code> | <code>if (getEnvValue('NEXT_PUBLIC_PAXEER_X_ENABLED') === 'true') {</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-fe-next-public-paxeer-x-enabled</code> | notes | read 2x under configs; 14 other sites |

## env-promote

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-fe-next-public-promote-blockscout-in-title</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/meta.ts:7</code> | <code>promoteBlockscoutInTitle: getEnvValue('NEXT_PUBLIC_PROMOTE_BLOCKSCOUT_IN_TITLE') === 'false' ? false : true,</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-fe-next-public-promote-blockscout-in-title</code> | notes | read 1x under configs; 3 other sites |

## env-puzzle

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-fe-next-public-puzzle-game-badge-claim-link</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/easterEggPuzzleBadge.ts:5</code> | <code>const badgeClaimLink = getEnvValue('NEXT_PUBLIC_PUZZLE_GAME_BADGE_CLAIM_LINK');</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-fe-next-public-puzzle-game-badge-claim-link</code> | notes | read 1x under configs; 1 other sites |

## env-re

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-fe-next-public-re-captcha-app-site-key</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/services.ts:5</code> | <code>siteKey: getEnvValue('NEXT_PUBLIC_RE_CAPTCHA_APP_SITE_KEY'),</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-fe-next-public-re-captcha-app-site-key</code> | notes | read 1x under configs; 9 other sites |

## env-rewards

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-fe-next-public-rewards-service-api-host</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/apis.ts:109</code> | <code>const apiHost = getEnvValue('NEXT_PUBLIC_REWARDS_SERVICE_API_HOST');</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-fe-next-public-rewards-service-api-host</code> | notes | read 1x under configs; 2 other sites |

## env-rollbar

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-fe-next-public-rollbar-client-token</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/rollbar.ts:6</code> | <code>const clientToken = getEnvValue('NEXT_PUBLIC_ROLLBAR_CLIENT_TOKEN');</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-fe-next-public-rollbar-client-token</code> | notes | read 1x under configs; 3 other sites |

## env-rollup

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-fe-next-public-rollup-da-celestia-celenium-url</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/rollup.ts:63</code> | <code>celeniumUrl: getEnvValue('NEXT_PUBLIC_ROLLUP_DA_CELESTIA_CELENIUM_URL'),</code> |
| <code>interface.14.env-fe-next-public-rollup-da-celestia-namespace</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/rollup.ts:62</code> | <code>namespace: type === 'arbitrum' ? getEnvValue('NEXT_PUBLIC_ROLLUP_DA_CELESTIA_NAMESPACE') : undefined,</code> |
| <code>interface.14.env-fe-next-public-rollup-homepage-show-latest-blocks</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/rollup.ts:57</code> | <code>showLatestBlocks: getEnvValue('NEXT_PUBLIC_ROLLUP_HOMEPAGE_SHOW_LATEST_BLOCKS') === 'true',</code> |
| <code>interface.14.env-fe-next-public-rollup-l2-withdrawal-url</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/rollup.ts:14</code> | <code>const L2WithdrawalUrl = getEnvValue('NEXT_PUBLIC_ROLLUP_L2_WITHDRAWAL_URL');</code> |
| <code>interface.14.env-fe-next-public-rollup-layer-number</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/rollup.ts:52</code> | <code>layerNumber: Number(getEnvValue('NEXT_PUBLIC_ROLLUP_LAYER_NUMBER') &#124;&#124; 2),</code> |
| <code>interface.14.env-fe-next-public-rollup-output-roots-enabled</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/rollup.ts:54</code> | <code>outputRootsEnabled: type === 'optimistic' &amp;&amp; getEnvValue('NEXT_PUBLIC_ROLLUP_OUTPUT_ROOTS_ENABLED') === 'true',</code> |
| <code>interface.14.env-fe-next-public-rollup-parent-chain</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/rollup.ts:17</code> | <code>const envValue = parseEnvJson&lt;ParentChain&gt;(getEnvValue('NEXT_PUBLIC_ROLLUP_PARENT_CHAIN'));</code> |
| <code>interface.14.env-fe-next-public-rollup-stage-index</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/rollup.ts:51</code> | <code>stageIndex: getEnvValue('NEXT_PUBLIC_ROLLUP_STAGE_INDEX'),</code> |
| <code>interface.14.env-fe-next-public-rollup-type</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/chain.ts:11</code> | <code>const rollupType = getEnvValue('NEXT_PUBLIC_ROLLUP_TYPE') as RollupType;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-fe-next-public-rollup-da-celestia-celenium-url</code> | notes | read 1x under configs; 4 other sites |
| <code>interface.14.env-fe-next-public-rollup-da-celestia-namespace</code> | notes | read 1x under configs; 3 other sites |
| <code>interface.14.env-fe-next-public-rollup-homepage-show-latest-blocks</code> | notes | read 1x under configs; 2 other sites |
| <code>interface.14.env-fe-next-public-rollup-l2-withdrawal-url</code> | notes | read 1x under configs; 5 other sites |
| <code>interface.14.env-fe-next-public-rollup-layer-number</code> | notes | read 1x under configs; 2 other sites |
| <code>interface.14.env-fe-next-public-rollup-output-roots-enabled</code> | notes | read 1x under configs; 3 other sites |
| <code>interface.14.env-fe-next-public-rollup-parent-chain</code> | notes | read 1x under configs; 9 other sites |
| <code>interface.14.env-fe-next-public-rollup-stage-index</code> | notes | read 1x under configs; 2 other sites |
| <code>interface.14.env-fe-next-public-rollup-type</code> | notes | read 2x under configs; 35 other sites |

## env-safe

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-fe-next-public-safe-tx-service-url</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/safe.ts:7</code> | <code>const envValue = getEnvValue('NEXT_PUBLIC_SAFE_TX_SERVICE_URL');</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-fe-next-public-safe-tx-service-url</code> | notes | read 1x under configs; 1 other sites |

## env-seo

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-fe-next-public-seo-enhanced-data-enabled</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/meta.ts:14</code> | <code>enhancedDataEnabled: getEnvValue('NEXT_PUBLIC_SEO_ENHANCED_DATA_ENABLED') === 'true',</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-fe-next-public-seo-enhanced-data-enabled</code> | notes | read 1x under configs; 2 other sites |

## env-stats

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-fe-next-public-stats-api-base-path</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/apis.ts:158</code> | <code>basePath: stripTrailingSlash(getEnvValue('NEXT_PUBLIC_STATS_API_BASE_PATH') &#124;&#124; ''),</code> |
| <code>interface.14.env-fe-next-public-stats-api-host</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/apis.ts:151</code> | <code>const apiHost = getEnvValue('NEXT_PUBLIC_STATS_API_HOST');</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-fe-next-public-stats-api-base-path</code> | notes | read 1x under configs; 1 other sites |
| <code>interface.14.env-fe-next-public-stats-api-host</code> | notes | read 1x under configs; 24 other sites |

## env-tac

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-fe-next-public-tac-operation-lifecycle-api-host</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/apis.ts:163</code> | <code>const apiHost = getEnvValue('NEXT_PUBLIC_TAC_OPERATION_LIFECYCLE_API_HOST');</code> |
| <code>interface.14.env-fe-next-public-tac-ton-explorer-url</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/tac.ts:8</code> | <code>const tonExplorerUrl = getEnvValue('NEXT_PUBLIC_TAC_TON_EXPLORER_URL');</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-fe-next-public-tac-operation-lifecycle-api-host</code> | notes | read 1x under configs; 5 other sites |
| <code>interface.14.env-fe-next-public-tac-ton-explorer-url</code> | notes | read 1x under configs; 3 other sites |

## env-transaction

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-fe-next-public-transaction-interpretation-provider</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/txInterpretation.ts:10</code> | <code>const value = getEnvValue('NEXT_PUBLIC_TRANSACTION_INTERPRETATION_PROVIDER');</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-fe-next-public-transaction-interpretation-provider</code> | notes | read 1x under configs; 4 other sites |

## env-tx

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-fe-next-public-tx-external-transactions-config</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/externalTxs.ts:6</code> | <code>const externalTransactionsConfig = parseEnvJson&lt;TxExternalTxsConfig&gt;(getEnvValue('NEXT_PUBLIC_TX_EXTERNAL_TRANSACTIONS_CONFIG'));</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-fe-next-public-tx-external-transactions-config</code> | notes | read 1x under configs; 3 other sites |

## env-use

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-fe-next-public-use-next-js-proxy</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/app.ts:31</code> | <code>useProxy: getEnvValue('NEXT_PUBLIC_USE_NEXT_JS_PROXY') === 'true',</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-fe-next-public-use-next-js-proxy</code> | notes | read 1x under configs; 4 other sites |

## env-user

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-fe-next-public-user-ops-indexer-api-host</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/apis.ts:174</code> | <code>const apiHost = getEnvValue('NEXT_PUBLIC_USER_OPS_INDEXER_API_HOST');</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-fe-next-public-user-ops-indexer-api-host</code> | notes | read 1x under configs; 3 other sites |

## env-validators

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-fe-next-public-validators-chain-type</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/validators.ts:8</code> | <code>const envValue = getEnvValue('NEXT_PUBLIC_VALIDATORS_CHAIN_TYPE') as ValidatorsChainType &#124; undefined;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-fe-next-public-validators-chain-type</code> | notes | read 1x under configs; 5 other sites |

## env-views

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-fe-next-public-views-address-bech-32-prefix</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/ui/views/address.ts:26</code> | <code>const value = getEnvValue('NEXT_PUBLIC_VIEWS_ADDRESS_BECH_32_PREFIX');</code> |
| <code>interface.14.env-fe-next-public-views-address-format</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/ui/views/address.ts:15</code> | <code>const value = (parseEnvJson&lt;Array&lt;AddressFormat&gt;&gt;(getEnvValue('NEXT_PUBLIC_VIEWS_ADDRESS_FORMAT')) &#124;&#124; [])</code> |
| <code>interface.14.env-fe-next-public-views-address-hidden-views</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/ui/views/address.ts:37</code> | <code>const parsedValue = parseEnvJson&lt;Array&lt;AddressViewId&gt;&gt;(getEnvValue('NEXT_PUBLIC_VIEWS_ADDRESS_HIDDEN_VIEWS')) &#124;&#124; [];</code> |
| <code>interface.14.env-fe-next-public-views-address-identicon-type</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/ui/views/address.ts:9</code> | <code>const value = getEnvValue('NEXT_PUBLIC_VIEWS_ADDRESS_IDENTICON_TYPE');</code> |
| <code>interface.14.env-fe-next-public-views-address-native-token-address</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/ui/views/address.ts:73</code> | <code>nativeTokenAddress: getEnvValue('NEXT_PUBLIC_VIEWS_ADDRESS_NATIVE_TOKEN_ADDRESS'),</code> |
| <code>interface.14.env-fe-next-public-views-block-hidden-fields</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/ui/views/block.ts:7</code> | <code>const parsedValue = parseEnvJson&lt;Array&lt;BlockFieldId&gt;&gt;(getEnvValue('NEXT_PUBLIC_VIEWS_BLOCK_HIDDEN_FIELDS')) &#124;&#124; [];</code> |
| <code>interface.14.env-fe-next-public-views-block-pending-update-alert-enabled</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/ui/views/block.ts:23</code> | <code>pendingUpdateAlertEnabled: getEnvValue('NEXT_PUBLIC_VIEWS_BLOCK_PENDING_UPDATE_ALERT_ENABLED') !== 'false',</code> |
| <code>interface.14.env-fe-next-public-views-contract-decoded-bytecode-enabled</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/ui/views/address.ts:76</code> | <code>decodedBytecodeEnabled: getEnvValue('NEXT_PUBLIC_VIEWS_CONTRACT_DECODED_BYTECODE_ENABLED') === 'true',</code> |
| <code>interface.14.env-fe-next-public-views-contract-extra-verification-methods</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/ui/views/address.ts:52</code> | <code>const envValue = getEnvValue('NEXT_PUBLIC_VIEWS_CONTRACT_EXTRA_VERIFICATION_METHODS');</code> |
| <code>interface.14.env-fe-next-public-views-contract-solidityscan-enabled</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/ui/views/address.ts:74</code> | <code>solidityscanEnabled: getEnvValue('NEXT_PUBLIC_VIEWS_CONTRACT_SOLIDITYSCAN_ENABLED') === 'true',</code> |
| <code>interface.14.env-fe-next-public-views-nft-marketplaces</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/ui/views/nft.ts:6</code> | <code>const marketplaces = parseEnvJson&lt;Array&lt;NftMarketplaceItem&gt;&gt;(getEnvValue('NEXT_PUBLIC_VIEWS_NFT_MARKETPLACES')) &#124;&#124; [];</code> |
| <code>interface.14.env-fe-next-public-views-token-scam-toggle-enabled</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/ui/views/token.ts:4</code> | <code>hideScamTokensEnabled: getEnvValue('NEXT_PUBLIC_VIEWS_TOKEN_SCAM_TOGGLE_ENABLED') === 'true',</code> |
| <code>interface.14.env-fe-next-public-views-tx-additional-fields</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/ui/views/tx.ts:22</code> | <code>const parsedValue = parseEnvJson&lt;Array&lt;TxAdditionalFieldsId&gt;&gt;(getEnvValue('NEXT_PUBLIC_VIEWS_TX_ADDITIONAL_FIELDS')) &#124;&#124; [];</code> |
| <code>interface.14.env-fe-next-public-views-tx-grouped-fees</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/ui/views/tx.ts:39</code> | <code>groupedFees: getEnvValue('NEXT_PUBLIC_VIEWS_TX_GROUPED_FEES') === 'true',</code> |
| <code>interface.14.env-fe-next-public-views-tx-hidden-fields</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/ui/views/tx.ts:7</code> | <code>const parsedValue = parseEnvJson&lt;Array&lt;TxFieldsId&gt;&gt;(getEnvValue('NEXT_PUBLIC_VIEWS_TX_HIDDEN_FIELDS')) &#124;&#124; [];</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-fe-next-public-views-address-bech-32-prefix</code> | notes | read 1x under configs; 3 other sites |
| <code>interface.14.env-fe-next-public-views-address-format</code> | notes | read 1x under configs; 3 other sites |
| <code>interface.14.env-fe-next-public-views-address-hidden-views</code> | notes | read 1x under configs; 2 other sites |
| <code>interface.14.env-fe-next-public-views-address-identicon-type</code> | notes | read 1x under configs; 3 other sites |
| <code>interface.14.env-fe-next-public-views-address-native-token-address</code> | notes | read 1x under configs; 5 other sites |
| <code>interface.14.env-fe-next-public-views-block-hidden-fields</code> | notes | read 1x under configs; 2 other sites |
| <code>interface.14.env-fe-next-public-views-block-pending-update-alert-enabled</code> | notes | read 1x under configs; 3 other sites |
| <code>interface.14.env-fe-next-public-views-contract-decoded-bytecode-enabled</code> | notes | read 1x under configs; 2 other sites |
| <code>interface.14.env-fe-next-public-views-contract-extra-verification-methods</code> | notes | read 1x under configs; 3 other sites |
| <code>interface.14.env-fe-next-public-views-contract-solidityscan-enabled</code> | notes | read 1x under configs; 2 other sites |
| <code>interface.14.env-fe-next-public-views-nft-marketplaces</code> | notes | read 1x under configs; 3 other sites |
| <code>interface.14.env-fe-next-public-views-token-scam-toggle-enabled</code> | notes | read 1x under configs; 6 other sites |
| <code>interface.14.env-fe-next-public-views-tx-additional-fields</code> | notes | read 1x under configs; 3 other sites |
| <code>interface.14.env-fe-next-public-views-tx-grouped-fees</code> | notes | read 1x under configs; 4 other sites |
| <code>interface.14.env-fe-next-public-views-tx-hidden-fields</code> | notes | read 1x under configs; 5 other sites |

## env-visualize

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-fe-next-public-visualize-api-base-path</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/apis.ts:192</code> | <code>basePath: stripTrailingSlash(getEnvValue('NEXT_PUBLIC_VISUALIZE_API_BASE_PATH') &#124;&#124; ''),</code> |
| <code>interface.14.env-fe-next-public-visualize-api-host</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/apis.ts:185</code> | <code>const apiHost = getEnvValue('NEXT_PUBLIC_VISUALIZE_API_HOST');</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-fe-next-public-visualize-api-base-path</code> | notes | read 1x under configs; 1 other sites |
| <code>interface.14.env-fe-next-public-visualize-api-host</code> | notes | read 1x under configs; 3 other sites |

## env-wallet

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-fe-next-public-wallet-connect-featured-wallet-ids</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/blockchainInteraction.ts:56</code> | <code>featuredWalletIds: parseEnvJson&lt;Array&lt;string&gt;&gt;(getEnvValue('NEXT_PUBLIC_WALLET_CONNECT_FEATURED_WALLET_IDS')) ?? [],</code> |
| <code>interface.14.env-fe-next-public-wallet-connect-project-id</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/blockchainInteraction.ts:9</code> | <code>const walletConnectProjectId = getEnvValue('NEXT_PUBLIC_WALLET_CONNECT_PROJECT_ID');</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-fe-next-public-wallet-connect-featured-wallet-ids</code> | notes | read 1x under configs; 1 other sites |
| <code>interface.14.env-fe-next-public-wallet-connect-project-id</code> | notes | read 1x under configs; 6 other sites |

## env-web3

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-fe-next-public-web3-disable-add-token-to-wallet</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/web3Wallet.ts:32</code> | <code>isDisabled: getEnvValue('NEXT_PUBLIC_WEB3_DISABLE_ADD_TOKEN_TO_WALLET') === 'true',</code> |
| <code>interface.14.env-fe-next-public-web3-wallets</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/web3Wallet.ts:9</code> | <code>const envValue = getEnvValue('NEXT_PUBLIC_WEB3_WALLETS');</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-fe-next-public-web3-disable-add-token-to-wallet</code> | notes | read 1x under configs; 1 other sites |
| <code>interface.14.env-fe-next-public-web3-wallets</code> | notes | read 1x under configs; 2 other sites |

## env-xstar

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-fe-next-public-xstar-score-url</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/xStarScore.ts:6</code> | <code>const url = getEnvValue('NEXT_PUBLIC_XSTAR_SCORE_URL');</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-fe-next-public-xstar-score-url</code> | notes | read 1x under configs; 1 other sites |

## env-zetachain

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.env-fe-next-public-zetachain-external-search-config</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/zetachain.ts:10</code> | <code>getEnvValue('NEXT_PUBLIC_ZETACHAIN_EXTERNAL_SEARCH_CONFIG'),</code> |
| <code>interface.14.env-fe-next-public-zetachain-service-api-host</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/apis.ts:208</code> | <code>const apiHost = getEnvValue('NEXT_PUBLIC_ZETACHAIN_SERVICE_API_HOST');</code> |
| <code>interface.14.env-fe-next-public-zetachain-service-chains-config-url</code> | <code>env_var</code> | <code>stable</code> | <code>apps/explorer/configs/app/features/zetachain.ts:8</code> | <code>const chainsConfigUrl = getExternalAssetFilePath('NEXT_PUBLIC_ZETACHAIN_SERVICE_CHAINS_CONFIG_URL');</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.env-fe-next-public-zetachain-external-search-config</code> | notes | read 1x under configs; 3 other sites |
| <code>interface.14.env-fe-next-public-zetachain-service-api-host</code> | notes | read 1x under configs; 9 other sites |
| <code>interface.14.env-fe-next-public-zetachain-service-chains-config-url</code> | notes | read 1x under configs; 12 other sites |
