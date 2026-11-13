# Explorer frontend config contract

The [explorer frontend configuration](../../../apps/explorer/configs) reads public environment variables and assembles the application configuration. `NEXT_PUBLIC_` settings are visible to the browser.

## cfg-apis

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.cfg-fe-apis-admin</code> | <code>config_key</code> | [apps/explorer/configs/app/apis.ts](../../../apps/explorer/configs/app/apis.ts) | <code>config.apis.admin</code> |
| <code>interface.14.cfg-fe-apis-bens</code> | <code>config_key</code> | [apps/explorer/configs/app/apis.ts](../../../apps/explorer/configs/app/apis.ts) | <code>config.apis.bens</code> |
| <code>interface.14.cfg-fe-apis-clusters</code> | <code>config_key</code> | [apps/explorer/configs/app/apis.ts](../../../apps/explorer/configs/app/apis.ts) | <code>config.apis.clusters</code> |
| <code>interface.14.cfg-fe-apis-contractinfo</code> | <code>config_key</code> | [apps/explorer/configs/app/apis.ts](../../../apps/explorer/configs/app/apis.ts) | <code>config.apis.contractInfo</code> |
| <code>interface.14.cfg-fe-apis-general</code> | <code>config_key</code> | [apps/explorer/configs/app/apis.ts](../../../apps/explorer/configs/app/apis.ts) | <code>config.apis.general</code> |
| <code>interface.14.cfg-fe-apis-interchainindexer</code> | <code>config_key</code> | [apps/explorer/configs/app/apis.ts](../../../apps/explorer/configs/app/apis.ts) | <code>config.apis.interchainIndexer</code> |
| <code>interface.14.cfg-fe-apis-metadata</code> | <code>config_key</code> | [apps/explorer/configs/app/apis.ts](../../../apps/explorer/configs/app/apis.ts) | <code>config.apis.metadata</code> |
| <code>interface.14.cfg-fe-apis-multichainaggregator</code> | <code>config_key</code> | [apps/explorer/configs/app/apis.ts](../../../apps/explorer/configs/app/apis.ts) | <code>config.apis.multichainAggregator</code> |
| <code>interface.14.cfg-fe-apis-multichainstats</code> | <code>config_key</code> | [apps/explorer/configs/app/apis.ts](../../../apps/explorer/configs/app/apis.ts) | <code>config.apis.multichainStats</code> |
| <code>interface.14.cfg-fe-apis-rewards</code> | <code>config_key</code> | [apps/explorer/configs/app/apis.ts](../../../apps/explorer/configs/app/apis.ts) | <code>config.apis.rewards</code> |
| <code>interface.14.cfg-fe-apis-stats</code> | <code>config_key</code> | [apps/explorer/configs/app/apis.ts](../../../apps/explorer/configs/app/apis.ts) | <code>config.apis.stats</code> |
| <code>interface.14.cfg-fe-apis-tac</code> | <code>config_key</code> | [apps/explorer/configs/app/apis.ts](../../../apps/explorer/configs/app/apis.ts) | <code>config.apis.tac</code> |
| <code>interface.14.cfg-fe-apis-userops</code> | <code>config_key</code> | [apps/explorer/configs/app/apis.ts](../../../apps/explorer/configs/app/apis.ts) | <code>config.apis.userOps</code> |
| <code>interface.14.cfg-fe-apis-visualize</code> | <code>config_key</code> | [apps/explorer/configs/app/apis.ts](../../../apps/explorer/configs/app/apis.ts) | <code>config.apis.visualize</code> |
| <code>interface.14.cfg-fe-apis-zetachain</code> | <code>config_key</code> | [apps/explorer/configs/app/apis.ts](../../../apps/explorer/configs/app/apis.ts) | <code>config.apis.zetachain</code> |

## cfg-app

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.cfg-fe-app-basepath</code> | <code>config_key</code> | [apps/explorer/configs/app/app.ts](../../../apps/explorer/configs/app/app.ts) | <code>config.app.basePath</code> |
| <code>interface.14.cfg-fe-app-baseurl</code> | <code>config_key</code> | [apps/explorer/configs/app/app.ts](../../../apps/explorer/configs/app/app.ts) | <code>config.app.baseUrl</code> |
| <code>interface.14.cfg-fe-app-host</code> | <code>config_key</code> | [apps/explorer/configs/app/app.ts](../../../apps/explorer/configs/app/app.ts) | <code>config.app.host</code> |
| <code>interface.14.cfg-fe-app-isdev</code> | <code>config_key</code> | [apps/explorer/configs/app/app.ts](../../../apps/explorer/configs/app/app.ts) | <code>config.app.isDev</code> |
| <code>interface.14.cfg-fe-app-isprivatemode</code> | <code>config_key</code> | [apps/explorer/configs/app/app.ts](../../../apps/explorer/configs/app/app.ts) | <code>config.app.isPrivateMode</code> |
| <code>interface.14.cfg-fe-app-ispw</code> | <code>config_key</code> | [apps/explorer/configs/app/app.ts](../../../apps/explorer/configs/app/app.ts) | <code>config.app.isPw</code> |
| <code>interface.14.cfg-fe-app-isreview</code> | <code>config_key</code> | [apps/explorer/configs/app/app.ts](../../../apps/explorer/configs/app/app.ts) | <code>config.app.isReview</code> |
| <code>interface.14.cfg-fe-app-origin</code> | <code>config_key</code> | [apps/explorer/configs/app/app.ts](../../../apps/explorer/configs/app/app.ts) | <code>config.app.origin</code> |
| <code>interface.14.cfg-fe-app-port</code> | <code>config_key</code> | [apps/explorer/configs/app/app.ts](../../../apps/explorer/configs/app/app.ts) | <code>config.app.port</code> |
| <code>interface.14.cfg-fe-app-protocol</code> | <code>config_key</code> | [apps/explorer/configs/app/app.ts](../../../apps/explorer/configs/app/app.ts) | <code>config.app.protocol</code> |
| <code>interface.14.cfg-fe-app-spritehash</code> | <code>config_key</code> | [apps/explorer/configs/app/app.ts](../../../apps/explorer/configs/app/app.ts) | <code>config.app.spriteHash</code> |
| <code>interface.14.cfg-fe-app-useproxy</code> | <code>config_key</code> | [apps/explorer/configs/app/app.ts](../../../apps/explorer/configs/app/app.ts) | <code>config.app.useProxy</code> |

## cfg-chain

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.cfg-fe-chain-additionaltokentypes</code> | <code>config_key</code> | [apps/explorer/configs/app/chain.ts](../../../apps/explorer/configs/app/chain.ts) | <code>config.chain.additionalTokenTypes</code> |
| <code>interface.14.cfg-fe-chain-currency</code> | <code>config_key</code> | [apps/explorer/configs/app/chain.ts](../../../apps/explorer/configs/app/chain.ts) | <code>config.chain.currency</code> |
| <code>interface.14.cfg-fe-chain-hasmultiplegascurrencies</code> | <code>config_key</code> | [apps/explorer/configs/app/chain.ts](../../../apps/explorer/configs/app/chain.ts) | <code>config.chain.hasMultipleGasCurrencies</code> |
| <code>interface.14.cfg-fe-chain-id</code> | <code>config_key</code> | [apps/explorer/configs/app/chain.ts](../../../apps/explorer/configs/app/chain.ts) | <code>config.chain.id</code> |
| <code>interface.14.cfg-fe-chain-istestnet</code> | <code>config_key</code> | [apps/explorer/configs/app/chain.ts](../../../apps/explorer/configs/app/chain.ts) | <code>config.chain.isTestnet</code> |
| <code>interface.14.cfg-fe-chain-name</code> | <code>config_key</code> | [apps/explorer/configs/app/chain.ts](../../../apps/explorer/configs/app/chain.ts) | <code>config.chain.name</code> |
| <code>interface.14.cfg-fe-chain-rpcurls</code> | <code>config_key</code> | [apps/explorer/configs/app/chain.ts](../../../apps/explorer/configs/app/chain.ts) | <code>config.chain.rpcUrls</code> |
| <code>interface.14.cfg-fe-chain-secondarycoin</code> | <code>config_key</code> | [apps/explorer/configs/app/chain.ts](../../../apps/explorer/configs/app/chain.ts) | <code>config.chain.secondaryCoin</code> |
| <code>interface.14.cfg-fe-chain-shortname</code> | <code>config_key</code> | [apps/explorer/configs/app/chain.ts](../../../apps/explorer/configs/app/chain.ts) | <code>config.chain.shortName</code> |
| <code>interface.14.cfg-fe-chain-tokenstandard</code> | <code>config_key</code> | [apps/explorer/configs/app/chain.ts](../../../apps/explorer/configs/app/chain.ts) | <code>config.chain.tokenStandard</code> |
| <code>interface.14.cfg-fe-chain-verificationtype</code> | <code>config_key</code> | [apps/explorer/configs/app/chain.ts](../../../apps/explorer/configs/app/chain.ts) | <code>config.chain.verificationType</code> |

## cfg-essential-dapps-chains

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.cfg-fe-essential-dapps-chains</code> | <code>config_key</code> | [apps/explorer/configs/essential-dapps-chains/index.ts](../../../apps/explorer/configs/essential-dapps-chains/index.ts) | <code>const marketplaceFeature = config.features.marketplace;</code> |

## cfg-features

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.cfg-fe-feature-account</code> | <code>config_key</code> | [apps/explorer/configs/app/features/account.ts](../../../apps/explorer/configs/app/features/account.ts) | <code>config.features.account {title, isEnabled, authProvider, dynamic, apiKeysButton}</code> |
| <code>interface.14.cfg-fe-feature-address3rdpartywidgets</code> | <code>config_key</code> | [apps/explorer/configs/app/features/address3rdPartyWidgets.ts](../../../apps/explorer/configs/app/features/address3rdPartyWidgets.ts) | <code>config.features.address3rdPartyWidgets {title, isEnabled, widgets, configUrl}</code> |
| <code>interface.14.cfg-fe-feature-addressmetadata</code> | <code>config_key</code> | [apps/explorer/configs/app/features/addressMetadata.ts](../../../apps/explorer/configs/app/features/addressMetadata.ts) | <code>config.features.addressMetadata {title, isEnabled, isAddressTagsUpdateEnabled}</code> |
| <code>interface.14.cfg-fe-feature-addressprofileapi</code> | <code>config_key</code> | [apps/explorer/configs/app/features/addressProfileAPI.ts](../../../apps/explorer/configs/app/features/addressProfileAPI.ts) | <code>config.features.addressProfileAPI {title, isEnabled, apiUrlTemplate, tagLinkTemplate, tagIcon, tagBgColor, tagTextColor}</code> |
| <code>interface.14.cfg-fe-feature-addressverification</code> | <code>config_key</code> | [apps/explorer/configs/app/features/addressVerification.ts](../../../apps/explorer/configs/app/features/addressVerification.ts) | <code>config.features.addressVerification {title, isEnabled}</code> |
| <code>interface.14.cfg-fe-feature-adsbanner</code> | <code>config_key</code> | [apps/explorer/configs/app/features/adsBanner.ts](../../../apps/explorer/configs/app/features/adsBanner.ts) | <code>config.features.adsBanner {title, isEnabled, provider, adButler, isSpecifyEnabled, additionalProvider}</code> |
| <code>interface.14.cfg-fe-feature-adstext</code> | <code>config_key</code> | [apps/explorer/configs/app/features/adsText.ts](../../../apps/explorer/configs/app/features/adsText.ts) | <code>config.features.adsText {title, isEnabled, provider, sevio}</code> |
| <code>interface.14.cfg-fe-feature-advancedfilter</code> | <code>config_key</code> | [apps/explorer/configs/app/features/advancedFilter.ts](../../../apps/explorer/configs/app/features/advancedFilter.ts) | <code>config.features.advancedFilter {title, isEnabled}</code> |
| <code>interface.14.cfg-fe-feature-apidocs</code> | <code>config_key</code> | [apps/explorer/configs/app/features/apiDocs.ts](../../../apps/explorer/configs/app/features/apiDocs.ts) | <code>config.features.apiDocs {title, isEnabled, tabs, alertMessage}</code> |
| <code>interface.14.cfg-fe-feature-beaconchain</code> | <code>config_key</code> | [apps/explorer/configs/app/features/beaconChain.ts](../../../apps/explorer/configs/app/features/beaconChain.ts) | <code>config.features.beaconChain {title, isEnabled, currency, validatorUrlTemplate, withdrawalsOnly}</code> |
| <code>interface.14.cfg-fe-feature-blockchaininteraction</code> | <code>config_key</code> | [apps/explorer/configs/app/features/blockchainInteraction.ts](../../../apps/explorer/configs/app/features/blockchainInteraction.ts) | <code>config.features.blockchainInteraction {title, isEnabled, connectorType, dynamic, reown}</code> |
| <code>interface.14.cfg-fe-feature-bridgedtokens</code> | <code>config_key</code> | [apps/explorer/configs/app/features/bridgedTokens.ts](../../../apps/explorer/configs/app/features/bridgedTokens.ts) | <code>config.features.bridgedTokens {title, isEnabled, chains, bridges}</code> |
| <code>interface.14.cfg-fe-feature-celo</code> | <code>config_key</code> | [apps/explorer/configs/app/features/celo.ts](../../../apps/explorer/configs/app/features/celo.ts) | <code>config.features.celo {title, isEnabled}</code> |
| <code>interface.14.cfg-fe-feature-crosschaintxs</code> | <code>config_key</code> | [apps/explorer/configs/app/features/crossChainTxs.ts](../../../apps/explorer/configs/app/features/crossChainTxs.ts) | <code>config.features.crossChainTxs {title, isEnabled}</code> |
| <code>interface.14.cfg-fe-feature-csvexport</code> | <code>config_key</code> | [apps/explorer/configs/app/features/csvExport.ts](../../../apps/explorer/configs/app/features/csvExport.ts) | <code>config.features.csvExport {title, isEnabled, reCaptcha}</code> |
| <code>interface.14.cfg-fe-feature-dataavailability</code> | <code>config_key</code> | [apps/explorer/configs/app/features/dataAvailability.ts](../../../apps/explorer/configs/app/features/dataAvailability.ts) | <code>config.features.dataAvailability {title, isEnabled}</code> |
| <code>interface.14.cfg-fe-feature-defidropdown</code> | <code>config_key</code> | [apps/explorer/configs/app/features/deFiDropdown.ts](../../../apps/explorer/configs/app/features/deFiDropdown.ts) | <code>config.features.deFiDropdown {title, isEnabled, items}</code> |
| <code>interface.14.cfg-fe-feature-eastereggbadge</code> | <code>config_key</code> | [apps/explorer/configs/app/features/easterEggBadge.ts](../../../apps/explorer/configs/app/features/easterEggBadge.ts) | <code>config.features.easterEggBadge {title, isEnabled, badgeClaimLink}</code> |
| <code>interface.14.cfg-fe-feature-eastereggpuzzlebadge</code> | <code>config_key</code> | [apps/explorer/configs/app/features/easterEggPuzzleBadge.ts](../../../apps/explorer/configs/app/features/easterEggPuzzleBadge.ts) | <code>config.features.easterEggPuzzleBadge {title, isEnabled, badgeClaimLink}</code> |
| <code>interface.14.cfg-fe-feature-externaltxs</code> | <code>config_key</code> | [apps/explorer/configs/app/features/externalTxs.ts](../../../apps/explorer/configs/app/features/externalTxs.ts) | <code>config.features.externalTxs {title, isEnabled, chainName, chainLogoUrl, explorerUrlTemplate}</code> |
| <code>interface.14.cfg-fe-feature-faultproofsystem</code> | <code>config_key</code> | [apps/explorer/configs/app/features/faultProofSystem.ts](../../../apps/explorer/configs/app/features/faultProofSystem.ts) | <code>config.features.faultProofSystem {title, isEnabled}</code> |
| <code>interface.14.cfg-fe-feature-flashblocks</code> | <code>config_key</code> | [apps/explorer/configs/app/features/flashblocks.ts](../../../apps/explorer/configs/app/features/flashblocks.ts) | <code>config.features.flashblocks {title, isEnabled, socketUrl, type, name}</code> |
| <code>interface.14.cfg-fe-feature-gastracker</code> | <code>config_key</code> | [apps/explorer/configs/app/features/gasTracker.ts](../../../apps/explorer/configs/app/features/gasTracker.ts) | <code>config.features.gasTracker {title, isEnabled, units}</code> |
| <code>interface.14.cfg-fe-feature-getgasbutton</code> | <code>config_key</code> | [apps/explorer/configs/app/features/getGasButton.ts](../../../apps/explorer/configs/app/features/getGasButton.ts) | <code>config.features.getGasButton {title, isEnabled, name, logoUrl, url, dappId}</code> |
| <code>interface.14.cfg-fe-feature-googleanalytics</code> | <code>config_key</code> | [apps/explorer/configs/app/features/googleAnalytics.ts](../../../apps/explorer/configs/app/features/googleAnalytics.ts) | <code>config.features.googleAnalytics {title, isEnabled, propertyId}</code> |
| <code>interface.14.cfg-fe-feature-growthbook</code> | <code>config_key</code> | [apps/explorer/configs/app/features/growthBook.ts](../../../apps/explorer/configs/app/features/growthBook.ts) | <code>config.features.growthBook {title, isEnabled, clientKey}</code> |
| <code>interface.14.cfg-fe-feature-hotcontracts</code> | <code>config_key</code> | [apps/explorer/configs/app/features/hotContracts.ts](../../../apps/explorer/configs/app/features/hotContracts.ts) | <code>config.features.hotContracts {title, isEnabled}</code> |
| <code>interface.14.cfg-fe-feature-marketplace</code> | <code>config_key</code> | [apps/explorer/configs/app/features/marketplace.ts](../../../apps/explorer/configs/app/features/marketplace.ts) | <code>config.features.marketplace {title, isEnabled, configUrl, props, api}</code> |
| <code>interface.14.cfg-fe-feature-megaeth</code> | <code>config_key</code> | [apps/explorer/configs/app/features/megaEth.ts](../../../apps/explorer/configs/app/features/megaEth.ts) | <code>config.features.megaEth {title, isEnabled, socketUrl}</code> |
| <code>interface.14.cfg-fe-feature-metasuites</code> | <code>config_key</code> | [apps/explorer/configs/app/features/metasuites.ts](../../../apps/explorer/configs/app/features/metasuites.ts) | <code>config.features.metasuites {title, isEnabled}</code> |
| <code>interface.14.cfg-fe-feature-mixpanel</code> | <code>config_key</code> | [apps/explorer/configs/app/features/mixpanel.ts](../../../apps/explorer/configs/app/features/mixpanel.ts) | <code>config.features.mixpanel {title, isEnabled, projectToken, configOverrides}</code> |
| <code>interface.14.cfg-fe-feature-mudframework</code> | <code>config_key</code> | [apps/explorer/configs/app/features/mudFramework.ts](../../../apps/explorer/configs/app/features/mudFramework.ts) | <code>config.features.mudFramework {title, isEnabled}</code> |
| <code>interface.14.cfg-fe-feature-multichain</code> | <code>config_key</code> | [apps/explorer/configs/app/features/multichain.ts](../../../apps/explorer/configs/app/features/multichain.ts) | <code>config.features.multichain {title, isEnabled, cluster}</code> |
| <code>interface.14.cfg-fe-feature-multichainbutton</code> | <code>config_key</code> | [apps/explorer/configs/app/features/multichainButton.ts](../../../apps/explorer/configs/app/features/multichainButton.ts) | <code>config.features.multichainButton {title, isEnabled, providers}</code> |
| <code>interface.14.cfg-fe-feature-nameservices</code> | <code>config_key</code> | [apps/explorer/configs/app/features/nameServices.ts](../../../apps/explorer/configs/app/features/nameServices.ts) | <code>config.features.nameServices {title, isEnabled, ens, clusters}</code> |
| <code>interface.14.cfg-fe-feature-paxeerxfeed</code> | <code>config_key</code> | [apps/explorer/configs/app/features/paxeerXFeed.ts](../../../apps/explorer/configs/app/features/paxeerXFeed.ts) | <code>config.features.paxeerXFeed {title, isEnabled}</code> |
| <code>interface.14.cfg-fe-feature-paxeerxlists</code> | <code>config_key</code> | [apps/explorer/configs/app/features/paxeerXLists.ts](../../../apps/explorer/configs/app/features/paxeerXLists.ts) | <code>config.features.paxeerXLists {title, isEnabled}</code> |
| <code>interface.14.cfg-fe-feature-pools</code> | <code>config_key</code> | [apps/explorer/configs/app/features/pools.ts](../../../apps/explorer/configs/app/features/pools.ts) | <code>config.features.pools {title, isEnabled}</code> |
| <code>interface.14.cfg-fe-feature-publictagssubmission</code> | <code>config_key</code> | [apps/explorer/configs/app/features/publicTagsSubmission.ts](../../../apps/explorer/configs/app/features/publicTagsSubmission.ts) | <code>config.features.publicTagsSubmission {title, isEnabled}</code> |
| <code>interface.14.cfg-fe-feature-rewards</code> | <code>config_key</code> | [apps/explorer/configs/app/features/rewards.ts](../../../apps/explorer/configs/app/features/rewards.ts) | <code>config.features.rewards {title, isEnabled}</code> |
| <code>interface.14.cfg-fe-feature-rollbar</code> | <code>config_key</code> | [apps/explorer/configs/app/features/rollbar.ts](../../../apps/explorer/configs/app/features/rollbar.ts) | <code>config.features.rollbar {title, isEnabled, clientToken, environment, instance, codeVersion}</code> |
| <code>interface.14.cfg-fe-feature-rollup</code> | <code>config_key</code> | [apps/explorer/configs/app/features/rollup.ts](../../../apps/explorer/configs/app/features/rollup.ts) | <code>config.features.rollup {title, isEnabled, type, stageIndex, layerNumber, L2WithdrawalUrl, outputRootsEnabled, interopEnabled, homepage, parentChain, DA}</code> |
| <code>interface.14.cfg-fe-feature-safe</code> | <code>config_key</code> | [apps/explorer/configs/app/features/safe.ts](../../../apps/explorer/configs/app/features/safe.ts) | <code>config.features.safe {title, isEnabled, apiUrl}</code> |
| <code>interface.14.cfg-fe-feature-sol2uml</code> | <code>config_key</code> | [apps/explorer/configs/app/features/sol2uml.ts](../../../apps/explorer/configs/app/features/sol2uml.ts) | <code>config.features.sol2uml {title, isEnabled}</code> |
| <code>interface.14.cfg-fe-feature-stats</code> | <code>config_key</code> | [apps/explorer/configs/app/features/stats.ts](../../../apps/explorer/configs/app/features/stats.ts) | <code>config.features.stats {title, isEnabled}</code> |
| <code>interface.14.cfg-fe-feature-suave</code> | <code>config_key</code> | [apps/explorer/configs/app/features/suave.ts](../../../apps/explorer/configs/app/features/suave.ts) | <code>config.features.suave {title, isEnabled}</code> |
| <code>interface.14.cfg-fe-feature-tac</code> | <code>config_key</code> | [apps/explorer/configs/app/features/tac.ts](../../../apps/explorer/configs/app/features/tac.ts) | <code>config.features.tac {title, isEnabled, tonExplorerUrl}</code> |
| <code>interface.14.cfg-fe-feature-txinterpretation</code> | <code>config_key</code> | [apps/explorer/configs/app/features/txInterpretation.ts](../../../apps/explorer/configs/app/features/txInterpretation.ts) | <code>config.features.txInterpretation {title, provider, isEnabled}</code> |
| <code>interface.14.cfg-fe-feature-userops</code> | <code>config_key</code> | [apps/explorer/configs/app/features/userOps.ts](../../../apps/explorer/configs/app/features/userOps.ts) | <code>config.features.userOps {title, isEnabled}</code> |
| <code>interface.14.cfg-fe-feature-validators</code> | <code>config_key</code> | [apps/explorer/configs/app/features/validators.ts](../../../apps/explorer/configs/app/features/validators.ts) | <code>config.features.validators {title, isEnabled, chainType}</code> |
| <code>interface.14.cfg-fe-feature-verifiedtokens</code> | <code>config_key</code> | [apps/explorer/configs/app/features/verifiedTokens.ts](../../../apps/explorer/configs/app/features/verifiedTokens.ts) | <code>config.features.verifiedTokens {title, isEnabled}</code> |
| <code>interface.14.cfg-fe-feature-web3wallet</code> | <code>config_key</code> | [apps/explorer/configs/app/features/web3Wallet.ts](../../../apps/explorer/configs/app/features/web3Wallet.ts) | <code>config.features.web3Wallet {title, isEnabled, wallets, addToken, addNetwork}</code> |
| <code>interface.14.cfg-fe-feature-xstarscore</code> | <code>config_key</code> | [apps/explorer/configs/app/features/xStarScore.ts](../../../apps/explorer/configs/app/features/xStarScore.ts) | <code>config.features.xStarScore {title, url, isEnabled}</code> |
| <code>interface.14.cfg-fe-feature-zetachain</code> | <code>config_key</code> | [apps/explorer/configs/app/features/zetachain.ts](../../../apps/explorer/configs/app/features/zetachain.ts) | <code>config.features.zetachain {title, isEnabled, chainsConfigUrl, externalSearchConfig}</code> |

## cfg-meta

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.cfg-fe-meta-og</code> | <code>config_key</code> | [apps/explorer/configs/app/meta.ts](../../../apps/explorer/configs/app/meta.ts) | <code>config.meta.og</code> |
| <code>interface.14.cfg-fe-meta-promoteblockscoutintitle</code> | <code>config_key</code> | [apps/explorer/configs/app/meta.ts](../../../apps/explorer/configs/app/meta.ts) | <code>config.meta.promoteBlockscoutInTitle</code> |
| <code>interface.14.cfg-fe-meta-seo</code> | <code>config_key</code> | [apps/explorer/configs/app/meta.ts](../../../apps/explorer/configs/app/meta.ts) | <code>config.meta.seo</code> |

## cfg-multichain

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.cfg-fe-multichain</code> | <code>config_key</code> | [apps/explorer/configs/multichain/index.ts](../../../apps/explorer/configs/multichain/index.ts) | <code>const multichainConfig: () =&gt; MultichainConfig &#124; undefined</code> |

## cfg-paxeer-x

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.cfg-fe-paxeer-x-footer</code> | <code>config_key</code> | [apps/explorer/configs/paxeer-x/footer.json](../../../apps/explorer/configs/paxeer-x/footer.json) | <code>configs/paxeer-x/footer.json</code> |

## cfg-services

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.cfg-fe-services-recaptchav2</code> | <code>config_key</code> | [apps/explorer/configs/app/services.ts](../../../apps/explorer/configs/app/services.ts) | <code>config.services.reCaptchaV2</code> |

## cfg-ui

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.cfg-fe-ui-apikeysalert</code> | <code>config_key</code> | [apps/explorer/configs/app/ui.ts](../../../apps/explorer/configs/app/ui.ts) | <code>config.UI.apiKeysAlert</code> |
| <code>interface.14.cfg-fe-ui-colortheme</code> | <code>config_key</code> | [apps/explorer/configs/app/ui.ts](../../../apps/explorer/configs/app/ui.ts) | <code>config.UI.colorTheme</code> |
| <code>interface.14.cfg-fe-ui-explorers</code> | <code>config_key</code> | [apps/explorer/configs/app/ui.ts](../../../apps/explorer/configs/app/ui.ts) | <code>config.UI.explorers</code> |
| <code>interface.14.cfg-fe-ui-featurednetworks</code> | <code>config_key</code> | [apps/explorer/configs/app/ui.ts](../../../apps/explorer/configs/app/ui.ts) | <code>config.UI.featuredNetworks</code> |
| <code>interface.14.cfg-fe-ui-fonts</code> | <code>config_key</code> | [apps/explorer/configs/app/ui.ts](../../../apps/explorer/configs/app/ui.ts) | <code>config.UI.fonts</code> |
| <code>interface.14.cfg-fe-ui-footer</code> | <code>config_key</code> | [apps/explorer/configs/app/ui.ts](../../../apps/explorer/configs/app/ui.ts) | <code>config.UI.footer</code> |
| <code>interface.14.cfg-fe-ui-hascontractauditreports</code> | <code>config_key</code> | [apps/explorer/configs/app/ui.ts](../../../apps/explorer/configs/app/ui.ts) | <code>config.UI.hasContractAuditReports</code> |
| <code>interface.14.cfg-fe-ui-homepage</code> | <code>config_key</code> | [apps/explorer/configs/app/ui.ts](../../../apps/explorer/configs/app/ui.ts) | <code>config.UI.homepage</code> |
| <code>interface.14.cfg-fe-ui-ides</code> | <code>config_key</code> | [apps/explorer/configs/app/ui.ts](../../../apps/explorer/configs/app/ui.ts) | <code>config.UI.ides</code> |
| <code>interface.14.cfg-fe-ui-indexingalert</code> | <code>config_key</code> | [apps/explorer/configs/app/ui.ts](../../../apps/explorer/configs/app/ui.ts) | <code>config.UI.indexingAlert</code> |
| <code>interface.14.cfg-fe-ui-maintenancealert</code> | <code>config_key</code> | [apps/explorer/configs/app/ui.ts](../../../apps/explorer/configs/app/ui.ts) | <code>config.UI.maintenanceAlert</code> |
| <code>interface.14.cfg-fe-ui-maxcontentwidth</code> | <code>config_key</code> | [apps/explorer/configs/app/ui.ts](../../../apps/explorer/configs/app/ui.ts) | <code>config.UI.maxContentWidth</code> |
| <code>interface.14.cfg-fe-ui-nativecoinprice</code> | <code>config_key</code> | [apps/explorer/configs/app/ui.ts](../../../apps/explorer/configs/app/ui.ts) | <code>config.UI.nativeCoinPrice</code> |
| <code>interface.14.cfg-fe-ui-navigation</code> | <code>config_key</code> | [apps/explorer/configs/app/ui.ts](../../../apps/explorer/configs/app/ui.ts) | <code>config.UI.navigation</code> |
| <code>interface.14.cfg-fe-ui-views</code> | <code>config_key</code> | [apps/explorer/configs/app/ui.ts](../../../apps/explorer/configs/app/ui.ts) | <code>config.UI.views</code> |
| <code>interface.14.cfg-fe-ui-views-address</code> | <code>config_key</code> | [apps/explorer/configs/app/ui/views/address.ts](../../../apps/explorer/configs/app/ui/views/address.ts) | <code>config.UI.views.address {identiconType, hashFormat, hiddenViews, nativeTokenAddress, solidityscanEnabled, extraVerificationMethods, decodedBytecodeEnabled}</code> |
| <code>interface.14.cfg-fe-ui-views-block</code> | <code>config_key</code> | [apps/explorer/configs/app/ui/views/block.ts](../../../apps/explorer/configs/app/ui/views/block.ts) | <code>config.UI.views.block {hiddenFields, pendingUpdateAlertEnabled}</code> |
| <code>interface.14.cfg-fe-ui-views-nft</code> | <code>config_key</code> | [apps/explorer/configs/app/ui/views/nft.ts](../../../apps/explorer/configs/app/ui/views/nft.ts) | <code>config.UI.views.nft {marketplaces, verifiedFetch}</code> |
| <code>interface.14.cfg-fe-ui-views-token</code> | <code>config_key</code> | [apps/explorer/configs/app/ui/views/token.ts](../../../apps/explorer/configs/app/ui/views/token.ts) | <code>config.UI.views.token {hideScamTokensEnabled}</code> |
| <code>interface.14.cfg-fe-ui-views-tx</code> | <code>config_key</code> | [apps/explorer/configs/app/ui/views/tx.ts](../../../apps/explorer/configs/app/ui/views/tx.ts) | <code>config.UI.views.tx {hiddenFields, additionalFields, groupedFees}</code> |

## env-account

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-fe-next-public-account-api-keys-button</code> | <code>env_var</code> | [apps/explorer/configs/app/features/account.ts](../../../apps/explorer/configs/app/features/account.ts) | <code>NEXT_PUBLIC_ACCOUNT_API_KEYS_BUTTON</code> |
| <code>interface.14.env-fe-next-public-account-auth-provider</code> | <code>env_var</code> | [apps/explorer/configs/app/features/account.ts](../../../apps/explorer/configs/app/features/account.ts) | <code>NEXT_PUBLIC_ACCOUNT_AUTH_PROVIDER</code> |
| <code>interface.14.env-fe-next-public-account-dynamic-environment-id</code> | <code>env_var</code> | [apps/explorer/configs/app/features/account.ts](../../../apps/explorer/configs/app/features/account.ts) | <code>NEXT_PUBLIC_ACCOUNT_DYNAMIC_ENVIRONMENT_ID</code> |

## env-ad

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-fe-next-public-ad-adbutler-config-desktop</code> | <code>env_var</code> | [apps/explorer/configs/app/features/adsBanner.ts](../../../apps/explorer/configs/app/features/adsBanner.ts) | <code>const desktopConfig = parseEnvJson&lt;AdButlerConfig&gt;(getEnvValue('NEXT_PUBLIC_AD_ADBUTLER_CONFIG_DESKTOP'));</code> |
| <code>interface.14.env-fe-next-public-ad-adbutler-config-mobile</code> | <code>env_var</code> | [apps/explorer/configs/app/features/adsBanner.ts](../../../apps/explorer/configs/app/features/adsBanner.ts) | <code>const mobileConfig = parseEnvJson&lt;AdButlerConfig&gt;(getEnvValue('NEXT_PUBLIC_AD_ADBUTLER_CONFIG_MOBILE'));</code> |
| <code>interface.14.env-fe-next-public-ad-banner-additional-provider</code> | <code>env_var</code> | [apps/explorer/configs/app/features/adsBanner.ts](../../../apps/explorer/configs/app/features/adsBanner.ts) | <code>NEXT_PUBLIC_AD_BANNER_ADDITIONAL_PROVIDER</code> |
| <code>interface.14.env-fe-next-public-ad-banner-enable-specify</code> | <code>env_var</code> | [apps/explorer/configs/app/features/adsBanner.ts](../../../apps/explorer/configs/app/features/adsBanner.ts) | <code>NEXT_PUBLIC_AD_BANNER_ENABLE_SPECIFY</code> |
| <code>interface.14.env-fe-next-public-ad-banner-provider</code> | <code>env_var</code> | [apps/explorer/configs/app/features/adsBanner.ts](../../../apps/explorer/configs/app/features/adsBanner.ts) | <code>NEXT_PUBLIC_AD_BANNER_PROVIDER</code> |
| <code>interface.14.env-fe-next-public-ad-text-provider</code> | <code>env_var</code> | [apps/explorer/configs/app/features/adsText.ts](../../../apps/explorer/configs/app/features/adsText.ts) | <code>NEXT_PUBLIC_AD_TEXT_PROVIDER</code> |

## env-address

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-fe-next-public-address-3rd-party-widgets</code> | <code>env_var</code> | [apps/explorer/configs/app/features/address3rdPartyWidgets.ts](../../../apps/explorer/configs/app/features/address3rdPartyWidgets.ts) | <code>const widgets = parseEnvJson&lt;Array&lt;string&gt;&gt;(getEnvValue('NEXT_PUBLIC_ADDRESS_3RD_PARTY_WIDGETS'));</code> |
| <code>interface.14.env-fe-next-public-address-3rd-party-widgets-config-url</code> | <code>env_var</code> | [apps/explorer/configs/app/features/address3rdPartyWidgets.ts](../../../apps/explorer/configs/app/features/address3rdPartyWidgets.ts) | <code>NEXT_PUBLIC_ADDRESS_3RD_PARTY_WIDGETS_CONFIG_URL</code> |
| <code>interface.14.env-fe-next-public-address-username-tag</code> | <code>env_var</code> | [apps/explorer/configs/app/features/addressProfileAPI.ts](../../../apps/explorer/configs/app/features/addressProfileAPI.ts) | <code>const value = parseEnvJson&lt;AddressProfileAPIConfig&gt;(getEnvValue('NEXT_PUBLIC_ADDRESS_USERNAME_TAG'));</code> |

## env-admin

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-fe-next-public-admin-service-api-host</code> | <code>env_var</code> | [apps/explorer/configs/app/apis.ts](../../../apps/explorer/configs/app/apis.ts) | <code>NEXT_PUBLIC_ADMIN_SERVICE_API_HOST</code> |

## env-advanced

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-fe-next-public-advanced-filter-enabled</code> | <code>env_var</code> | [apps/explorer/configs/app/features/advancedFilter.ts](../../../apps/explorer/configs/app/features/advancedFilter.ts) | <code>NEXT_PUBLIC_ADVANCED_FILTER_ENABLED</code> |

## env-api

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-fe-next-public-api-base-path</code> | <code>env_var</code> | [apps/explorer/configs/app/apis.ts](../../../apps/explorer/configs/app/apis.ts) | <code>NEXT_PUBLIC_API_BASE_PATH</code> |
| <code>interface.14.env-fe-next-public-api-docs-alert-message</code> | <code>env_var</code> | [apps/explorer/configs/app/features/apiDocs.ts](../../../apps/explorer/configs/app/features/apiDocs.ts) | <code>NEXT_PUBLIC_API_DOCS_ALERT_MESSAGE</code> |
| <code>interface.14.env-fe-next-public-api-docs-tabs</code> | <code>env_var</code> | [apps/explorer/configs/app/features/apiDocs.ts](../../../apps/explorer/configs/app/features/apiDocs.ts) | <code>const value = (parseEnvJson&lt;Array&lt;ApiDocsTabId&gt;&gt;(getEnvValue('NEXT_PUBLIC_API_DOCS_TABS')) &#124;&#124; API_DOCS_TABS)</code> |
| <code>interface.14.env-fe-next-public-api-host</code> | <code>env_var</code> | [apps/explorer/configs/app/apis.ts](../../../apps/explorer/configs/app/apis.ts) | <code>NEXT_PUBLIC_API_HOST</code> |
| <code>interface.14.env-fe-next-public-api-keys-alert-message</code> | <code>env_var</code> | [apps/explorer/configs/app/ui.ts](../../../apps/explorer/configs/app/ui.ts) | <code>NEXT_PUBLIC_API_KEYS_ALERT_MESSAGE</code> |
| <code>interface.14.env-fe-next-public-api-port</code> | <code>env_var</code> | [apps/explorer/configs/app/apis.ts](../../../apps/explorer/configs/app/apis.ts) | <code>NEXT_PUBLIC_API_PORT</code> |
| <code>interface.14.env-fe-next-public-api-protocol</code> | <code>env_var</code> | [apps/explorer/configs/app/apis.ts](../../../apps/explorer/configs/app/apis.ts) | <code>NEXT_PUBLIC_API_PROTOCOL</code> |
| <code>interface.14.env-fe-next-public-api-websocket-protocol</code> | <code>env_var</code> | [apps/explorer/configs/app/apis.ts](../../../apps/explorer/configs/app/apis.ts) | <code>NEXT_PUBLIC_API_WEBSOCKET_PROTOCOL</code> |

## env-app

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-fe-next-public-app-env</code> | <code>env_var</code> | [apps/explorer/configs/app/app.ts](../../../apps/explorer/configs/app/app.ts) | <code>NEXT_PUBLIC_APP_ENV</code> |
| <code>interface.14.env-fe-next-public-app-host</code> | <code>env_var</code> | [apps/explorer/configs/app/app.ts](../../../apps/explorer/configs/app/app.ts) | <code>NEXT_PUBLIC_APP_HOST</code> |
| <code>interface.14.env-fe-next-public-app-instance</code> | <code>env_var</code> | [apps/explorer/configs/app/app.ts](../../../apps/explorer/configs/app/app.ts) | <code>NEXT_PUBLIC_APP_INSTANCE</code> |
| <code>interface.14.env-fe-next-public-app-port</code> | <code>env_var</code> | [apps/explorer/configs/app/app.ts](../../../apps/explorer/configs/app/app.ts) | <code>NEXT_PUBLIC_APP_PORT</code> |
| <code>interface.14.env-fe-next-public-app-protocol</code> | <code>env_var</code> | [apps/explorer/configs/app/app.ts](../../../apps/explorer/configs/app/app.ts) | <code>NEXT_PUBLIC_APP_PROTOCOL</code> |

## env-beacon

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-fe-next-public-beacon-chain-currency-symbol</code> | <code>env_var</code> | [apps/explorer/configs/app/features/beaconChain.ts](../../../apps/explorer/configs/app/features/beaconChain.ts) | <code>NEXT_PUBLIC_BEACON_CHAIN_CURRENCY_SYMBOL</code> |
| <code>interface.14.env-fe-next-public-beacon-chain-validator-url-template</code> | <code>env_var</code> | [apps/explorer/configs/app/features/beaconChain.ts](../../../apps/explorer/configs/app/features/beaconChain.ts) | <code>NEXT_PUBLIC_BEACON_CHAIN_VALIDATOR_URL_TEMPLATE</code> |
| <code>interface.14.env-fe-next-public-beacon-chain-withdrawals-only</code> | <code>env_var</code> | [apps/explorer/configs/app/features/beaconChain.ts](../../../apps/explorer/configs/app/features/beaconChain.ts) | <code>NEXT_PUBLIC_BEACON_CHAIN_WITHDRAWALS_ONLY</code> |

## env-bridged

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-fe-next-public-bridged-tokens-bridges</code> | <code>env_var</code> | [apps/explorer/configs/app/features/bridgedTokens.ts](../../../apps/explorer/configs/app/features/bridgedTokens.ts) | <code>const bridges = parseEnvJson&lt;Array&lt;TokenBridge&gt;&gt;(getEnvValue('NEXT_PUBLIC_BRIDGED_TOKENS_BRIDGES'));</code> |
| <code>interface.14.env-fe-next-public-bridged-tokens-chains</code> | <code>env_var</code> | [apps/explorer/configs/app/features/bridgedTokens.ts](../../../apps/explorer/configs/app/features/bridgedTokens.ts) | <code>const chains = parseEnvJson&lt;Array&lt;BridgedTokenChain&gt;&gt;(getEnvValue('NEXT_PUBLIC_BRIDGED_TOKENS_CHAINS'));</code> |

## env-celo

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-fe-next-public-celo-enabled</code> | <code>env_var</code> | [apps/explorer/configs/app/features/celo.ts](../../../apps/explorer/configs/app/features/celo.ts) | <code>NEXT_PUBLIC_CELO_ENABLED</code> |

## env-clusters

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-fe-next-public-clusters-api-host</code> | <code>env_var</code> | [apps/explorer/configs/app/apis.ts](../../../apps/explorer/configs/app/apis.ts) | <code>NEXT_PUBLIC_CLUSTERS_API_HOST</code> |
| <code>interface.14.env-fe-next-public-clusters-cdn-url</code> | <code>env_var</code> | [apps/explorer/configs/app/features/nameServices.ts](../../../apps/explorer/configs/app/features/nameServices.ts) | <code>cdnUrl: getEnvValue('NEXT_PUBLIC_CLUSTERS_CDN_URL') &#124;&#124; '&lt;url&gt;',</code> |

## env-color

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-fe-next-public-color-theme-default</code> | <code>env_var</code> | [apps/explorer/configs/app/ui.ts](../../../apps/explorer/configs/app/ui.ts) | <code>NEXT_PUBLIC_COLOR_THEME_DEFAULT</code> |
| <code>interface.14.env-fe-next-public-color-theme-overrides</code> | <code>env_var</code> | [apps/explorer/lib/settings/colorTheme.ts](../../../apps/explorer/lib/settings/colorTheme.ts) | <code>NEXT_PUBLIC_COLOR_THEME_OVERRIDES</code> |

## env-contract

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-fe-next-public-contract-code-ides</code> | <code>env_var</code> | [apps/explorer/configs/app/ui.ts](../../../apps/explorer/configs/app/ui.ts) | <code>items: parseEnvJson&lt;Array&lt;ContractCodeIde&gt;&gt;(getEnvValue('NEXT_PUBLIC_CONTRACT_CODE_IDES')) &#124;&#124; [],</code> |
| <code>interface.14.env-fe-next-public-contract-info-api-host</code> | <code>env_var</code> | [apps/explorer/configs/app/apis.ts](../../../apps/explorer/configs/app/apis.ts) | <code>NEXT_PUBLIC_CONTRACT_INFO_API_HOST</code> |

## env-cross

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-fe-next-public-cross-chain-txs-enabled</code> | <code>env_var</code> | [apps/explorer/configs/app/features/crossChainTxs.ts](../../../apps/explorer/configs/app/features/crossChainTxs.ts) | <code>NEXT_PUBLIC_CROSS_CHAIN_TXS_ENABLED</code> |

## env-data

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-fe-next-public-data-availability-enabled</code> | <code>env_var</code> | [apps/explorer/configs/app/features/dataAvailability.ts](../../../apps/explorer/configs/app/features/dataAvailability.ts) | <code>NEXT_PUBLIC_DATA_AVAILABILITY_ENABLED</code> |

## env-defi

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-fe-next-public-defi-dropdown-items</code> | <code>env_var</code> | [apps/explorer/configs/app/features/deFiDropdown.ts](../../../apps/explorer/configs/app/features/deFiDropdown.ts) | <code>const items = parseEnvJson&lt;Array&lt;DeFiDropdownItem&gt;&gt;(getEnvValue('NEXT_PUBLIC_DEFI_DROPDOWN_ITEMS')) &#124;&#124; [];</code> |

## env-dex

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-fe-next-public-dex-pools-enabled</code> | <code>env_var</code> | [apps/explorer/configs/app/features/pools.ts](../../../apps/explorer/configs/app/features/pools.ts) | <code>NEXT_PUBLIC_DEX_POOLS_ENABLED</code> |

## env-fault

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-fe-next-public-fault-proof-enabled</code> | <code>env_var</code> | [apps/explorer/configs/app/features/faultProofSystem.ts](../../../apps/explorer/configs/app/features/faultProofSystem.ts) | <code>NEXT_PUBLIC_FAULT_PROOF_ENABLED</code> |

## env-featured

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-fe-next-public-featured-networks</code> | <code>env_var</code> | [apps/explorer/configs/app/ui.ts](../../../apps/explorer/configs/app/ui.ts) | <code>NEXT_PUBLIC_FEATURED_NETWORKS</code> |
| <code>interface.14.env-fe-next-public-featured-networks-all-link</code> | <code>env_var</code> | [apps/explorer/configs/app/ui.ts](../../../apps/explorer/configs/app/ui.ts) | <code>NEXT_PUBLIC_FEATURED_NETWORKS_ALL_LINK</code> |
| <code>interface.14.env-fe-next-public-featured-networks-mode</code> | <code>env_var</code> | [apps/explorer/configs/app/ui.ts](../../../apps/explorer/configs/app/ui.ts) | <code>NEXT_PUBLIC_FEATURED_NETWORKS_MODE</code> |

## env-flashblocks

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-fe-next-public-flashblocks-socket-url</code> | <code>env_var</code> | [apps/explorer/configs/app/features/flashblocks.ts](../../../apps/explorer/configs/app/features/flashblocks.ts) | <code>NEXT_PUBLIC_FLASHBLOCKS_SOCKET_URL</code> |

## env-font

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-fe-next-public-font-family-body</code> | <code>env_var</code> | [apps/explorer/configs/app/ui.ts](../../../apps/explorer/configs/app/ui.ts) | <code>body: parseEnvJson&lt;FontFamily&gt;(getEnvValue('NEXT_PUBLIC_FONT_FAMILY_BODY')),</code> |
| <code>interface.14.env-fe-next-public-font-family-heading</code> | <code>env_var</code> | [apps/explorer/configs/app/ui.ts](../../../apps/explorer/configs/app/ui.ts) | <code>heading: parseEnvJson&lt;FontFamily&gt;(getEnvValue('NEXT_PUBLIC_FONT_FAMILY_HEADING')),</code> |

## env-footer

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-fe-next-public-footer-links</code> | <code>env_var</code> | [apps/explorer/configs/app/ui.ts](../../../apps/explorer/configs/app/ui.ts) | <code>NEXT_PUBLIC_FOOTER_LINKS</code> |

## env-game

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-fe-next-public-game-badge-claim-link</code> | <code>env_var</code> | [apps/explorer/configs/app/features/easterEggBadge.ts](../../../apps/explorer/configs/app/features/easterEggBadge.ts) | <code>NEXT_PUBLIC_GAME_BADGE_CLAIM_LINK</code> |

## env-gas

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-fe-next-public-gas-refuel-provider-config</code> | <code>env_var</code> | [apps/explorer/configs/app/features/getGasButton.ts](../../../apps/explorer/configs/app/features/getGasButton.ts) | <code>const value = parseEnvJson&lt;GasRefuelProviderConfig&gt;(getEnvValue('NEXT_PUBLIC_GAS_REFUEL_PROVIDER_CONFIG'));</code> |
| <code>interface.14.env-fe-next-public-gas-tracker-enabled</code> | <code>env_var</code> | [apps/explorer/configs/app/features/gasTracker.ts](../../../apps/explorer/configs/app/features/gasTracker.ts) | <code>NEXT_PUBLIC_GAS_TRACKER_ENABLED</code> |
| <code>interface.14.env-fe-next-public-gas-tracker-units</code> | <code>env_var</code> | [apps/explorer/configs/app/features/gasTracker.ts](../../../apps/explorer/configs/app/features/gasTracker.ts) | <code>NEXT_PUBLIC_GAS_TRACKER_UNITS</code> |

## env-git

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-fe-next-public-git-commit-sha</code> | <code>env_var</code> | [apps/explorer/configs/app/features/rollbar.ts](../../../apps/explorer/configs/app/features/rollbar.ts) | <code>NEXT_PUBLIC_GIT_COMMIT_SHA</code> |
| <code>interface.14.env-fe-next-public-git-tag</code> | <code>env_var</code> | [apps/explorer/configs/app/features/rollbar.ts](../../../apps/explorer/configs/app/features/rollbar.ts) | <code>NEXT_PUBLIC_GIT_TAG</code> |

## env-google

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-fe-next-public-google-analytics-property-id</code> | <code>env_var</code> | [apps/explorer/configs/app/features/googleAnalytics.ts](../../../apps/explorer/configs/app/features/googleAnalytics.ts) | <code>NEXT_PUBLIC_GOOGLE_ANALYTICS_PROPERTY_ID</code> |

## env-growth

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-fe-next-public-growth-book-client-key</code> | <code>env_var</code> | [apps/explorer/configs/app/features/growthBook.ts](../../../apps/explorer/configs/app/features/growthBook.ts) | <code>NEXT_PUBLIC_GROWTH_BOOK_CLIENT_KEY</code> |

## env-has

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-fe-next-public-has-beacon-chain</code> | <code>env_var</code> | [apps/explorer/configs/app/features/beaconChain.ts](../../../apps/explorer/configs/app/features/beaconChain.ts) | <code>NEXT_PUBLIC_HAS_BEACON_CHAIN</code> |
| <code>interface.14.env-fe-next-public-has-contract-audit-reports</code> | <code>env_var</code> | [apps/explorer/configs/app/ui.ts](../../../apps/explorer/configs/app/ui.ts) | <code>NEXT_PUBLIC_HAS_CONTRACT_AUDIT_REPORTS</code> |
| <code>interface.14.env-fe-next-public-has-mud-framework</code> | <code>env_var</code> | [apps/explorer/configs/app/features/mudFramework.ts](../../../apps/explorer/configs/app/features/mudFramework.ts) | <code>NEXT_PUBLIC_HAS_MUD_FRAMEWORK</code> |
| <code>interface.14.env-fe-next-public-has-user-ops</code> | <code>env_var</code> | [apps/explorer/configs/app/features/userOps.ts](../../../apps/explorer/configs/app/features/userOps.ts) | <code>NEXT_PUBLIC_HAS_USER_OPS</code> |

## env-helia

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-fe-next-public-helia-verified-fetch-enabled</code> | <code>env_var</code> | [apps/explorer/configs/app/ui/views/nft.ts](../../../apps/explorer/configs/app/ui/views/nft.ts) | <code>NEXT_PUBLIC_HELIA_VERIFIED_FETCH_ENABLED</code> |

## env-hide

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-fe-next-public-hide-indexing-alert-blocks</code> | <code>env_var</code> | [apps/explorer/configs/app/ui.ts](../../../apps/explorer/configs/app/ui.ts) | <code>NEXT_PUBLIC_HIDE_INDEXING_ALERT_BLOCKS</code> |
| <code>interface.14.env-fe-next-public-hide-indexing-alert-int-txs</code> | <code>env_var</code> | [apps/explorer/configs/app/ui.ts](../../../apps/explorer/configs/app/ui.ts) | <code>NEXT_PUBLIC_HIDE_INDEXING_ALERT_INT_TXS</code> |
| <code>interface.14.env-fe-next-public-hide-native-coin-price</code> | <code>env_var</code> | [apps/explorer/configs/app/ui.ts](../../../apps/explorer/configs/app/ui.ts) | <code>NEXT_PUBLIC_HIDE_NATIVE_COIN_PRICE</code> |

## env-homepage

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-fe-next-public-homepage-charts</code> | <code>env_var</code> | [apps/explorer/configs/app/ui.ts](../../../apps/explorer/configs/app/ui.ts) | <code>charts: parseEnvJson&lt;Array&lt;ChainIndicatorId&gt;&gt;(getEnvValue('NEXT_PUBLIC_HOMEPAGE_CHARTS')) &#124;&#124; [],</code> |
| <code>interface.14.env-fe-next-public-homepage-hero-banner-config</code> | <code>env_var</code> | [apps/explorer/configs/app/ui.ts](../../../apps/explorer/configs/app/ui.ts) | <code>heroBanner: parseEnvJson&lt;HeroBannerConfig&gt;(getEnvValue('NEXT_PUBLIC_HOMEPAGE_HERO_BANNER_CONFIG')),</code> |
| <code>interface.14.env-fe-next-public-homepage-highlights-config</code> | <code>env_var</code> | [apps/explorer/configs/app/ui.ts](../../../apps/explorer/configs/app/ui.ts) | <code>NEXT_PUBLIC_HOMEPAGE_HIGHLIGHTS_CONFIG</code> |
| <code>interface.14.env-fe-next-public-homepage-stats</code> | <code>env_var</code> | [apps/explorer/configs/app/ui.ts](../../../apps/explorer/configs/app/ui.ts) | <code>const parsedValue = parseEnvJson&lt;Array&lt;HomeStatsWidgetId&gt;&gt;(getEnvValue('NEXT_PUBLIC_HOMEPAGE_STATS'));</code> |

## env-hostname

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-fe-hostname</code> | <code>env_var</code> | [apps/explorer/configs/essential-dapps-chains/config.edge.ts](../../../apps/explorer/configs/essential-dapps-chains/config.edge.ts) | <code>return process.env.HOSTNAME !== undefined;</code> |

## env-hot

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-fe-next-public-hot-contracts-enabled</code> | <code>env_var</code> | [apps/explorer/configs/app/features/hotContracts.ts](../../../apps/explorer/configs/app/features/hotContracts.ts) | <code>NEXT_PUBLIC_HOT_CONTRACTS_ENABLED</code> |

## env-icon

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-fe-next-public-icon-sprite-hash</code> | <code>env_var</code> | [apps/explorer/configs/app/app.ts](../../../apps/explorer/configs/app/app.ts) | <code>NEXT_PUBLIC_ICON_SPRITE_HASH</code> |

## env-interchain

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-fe-next-public-interchain-indexer-api-host</code> | <code>env_var</code> | [apps/explorer/configs/app/apis.ts](../../../apps/explorer/configs/app/apis.ts) | <code>NEXT_PUBLIC_INTERCHAIN_INDEXER_API_HOST</code> |

## env-interop

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-fe-next-public-interop-enabled</code> | <code>env_var</code> | [apps/explorer/configs/app/features/rollup.ts](../../../apps/explorer/configs/app/features/rollup.ts) | <code>NEXT_PUBLIC_INTEROP_ENABLED</code> |

## env-is

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-fe-next-public-is-account-supported</code> | <code>env_var</code> | [apps/explorer/configs/app/features/account.ts](../../../apps/explorer/configs/app/features/account.ts) | <code>NEXT_PUBLIC_IS_ACCOUNT_SUPPORTED</code> |
| <code>interface.14.env-fe-next-public-is-suave-chain</code> | <code>env_var</code> | [apps/explorer/configs/app/features/suave.ts](../../../apps/explorer/configs/app/features/suave.ts) | <code>NEXT_PUBLIC_IS_SUAVE_CHAIN</code> |
| <code>interface.14.env-fe-next-public-is-testnet</code> | <code>env_var</code> | [apps/explorer/configs/app/chain.ts](../../../apps/explorer/configs/app/chain.ts) | <code>NEXT_PUBLIC_IS_TESTNET</code> |

## env-maintenance

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-fe-next-public-maintenance-alert-message</code> | <code>env_var</code> | [apps/explorer/configs/app/ui.ts](../../../apps/explorer/configs/app/ui.ts) | <code>NEXT_PUBLIC_MAINTENANCE_ALERT_MESSAGE</code> |

## env-marketplace

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-fe-next-public-marketplace-banner-content-url</code> | <code>env_var</code> | [apps/explorer/configs/app/features/marketplace.ts](../../../apps/explorer/configs/app/features/marketplace.ts) | <code>NEXT_PUBLIC_MARKETPLACE_BANNER_CONTENT_URL</code> |
| <code>interface.14.env-fe-next-public-marketplace-banner-link-url</code> | <code>env_var</code> | [apps/explorer/configs/app/features/marketplace.ts](../../../apps/explorer/configs/app/features/marketplace.ts) | <code>NEXT_PUBLIC_MARKETPLACE_BANNER_LINK_URL</code> |
| <code>interface.14.env-fe-next-public-marketplace-categories-url</code> | <code>env_var</code> | [apps/explorer/configs/app/features/marketplace.ts](../../../apps/explorer/configs/app/features/marketplace.ts) | <code>NEXT_PUBLIC_MARKETPLACE_CATEGORIES_URL</code> |
| <code>interface.14.env-fe-next-public-marketplace-config-url</code> | <code>env_var</code> | [apps/explorer/configs/app/features/marketplace.ts](../../../apps/explorer/configs/app/features/marketplace.ts) | <code>NEXT_PUBLIC_MARKETPLACE_CONFIG_URL</code> |
| <code>interface.14.env-fe-next-public-marketplace-enabled</code> | <code>env_var</code> | [apps/explorer/configs/app/features/marketplace.ts](../../../apps/explorer/configs/app/features/marketplace.ts) | <code>NEXT_PUBLIC_MARKETPLACE_ENABLED</code> |
| <code>interface.14.env-fe-next-public-marketplace-essential-dapps-ad-enabled</code> | <code>env_var</code> | [apps/explorer/configs/app/features/marketplace.ts](../../../apps/explorer/configs/app/features/marketplace.ts) | <code>NEXT_PUBLIC_MARKETPLACE_ESSENTIAL_DAPPS_AD_ENABLED</code> |
| <code>interface.14.env-fe-next-public-marketplace-essential-dapps-config</code> | <code>env_var</code> | [apps/explorer/configs/app/features/marketplace.ts](../../../apps/explorer/configs/app/features/marketplace.ts) | <code>const essentialDappsConfig = parseEnvJson&lt;EssentialDappsConfig&gt;(getEnvValue('NEXT_PUBLIC_MARKETPLACE_ESSENTIAL_DAPPS_CONFIG'));</code> |
| <code>interface.14.env-fe-next-public-marketplace-featured-app</code> | <code>env_var</code> | [apps/explorer/configs/app/features/marketplace.ts](../../../apps/explorer/configs/app/features/marketplace.ts) | <code>NEXT_PUBLIC_MARKETPLACE_FEATURED_APP</code> |
| <code>interface.14.env-fe-next-public-marketplace-graph-links-url</code> | <code>env_var</code> | [apps/explorer/configs/app/features/marketplace.ts](../../../apps/explorer/configs/app/features/marketplace.ts) | <code>NEXT_PUBLIC_MARKETPLACE_GRAPH_LINKS_URL</code> |
| <code>interface.14.env-fe-next-public-marketplace-submit-form</code> | <code>env_var</code> | [apps/explorer/configs/app/features/marketplace.ts](../../../apps/explorer/configs/app/features/marketplace.ts) | <code>NEXT_PUBLIC_MARKETPLACE_SUBMIT_FORM</code> |
| <code>interface.14.env-fe-next-public-marketplace-suggest-ideas-form</code> | <code>env_var</code> | [apps/explorer/configs/app/features/marketplace.ts](../../../apps/explorer/configs/app/features/marketplace.ts) | <code>NEXT_PUBLIC_MARKETPLACE_SUGGEST_IDEAS_FORM</code> |
| <code>interface.14.env-fe-next-public-marketplace-titles</code> | <code>env_var</code> | [apps/explorer/configs/app/features/marketplace.ts](../../../apps/explorer/configs/app/features/marketplace.ts) | <code>NEXT_PUBLIC_MARKETPLACE_TITLES</code> |

## env-max

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-fe-next-public-max-content-width-enabled</code> | <code>env_var</code> | [apps/explorer/configs/app/ui.ts](../../../apps/explorer/configs/app/ui.ts) | <code>NEXT_PUBLIC_MAX_CONTENT_WIDTH_ENABLED</code> |

## env-mega

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-fe-next-public-mega-eth-socket-url-metrics</code> | <code>env_var</code> | [apps/explorer/configs/app/features/megaEth.ts](../../../apps/explorer/configs/app/features/megaEth.ts) | <code>NEXT_PUBLIC_MEGA_ETH_SOCKET_URL_METRICS</code> |
| <code>interface.14.env-fe-next-public-mega-eth-socket-url-rpc</code> | <code>env_var</code> | [apps/explorer/configs/app/features/megaEth.ts](../../../apps/explorer/configs/app/features/megaEth.ts) | <code>NEXT_PUBLIC_MEGA_ETH_SOCKET_URL_RPC</code> |

## env-metadata

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-fe-next-public-metadata-address-tags-update-enabled</code> | <code>env_var</code> | [apps/explorer/configs/app/features/addressMetadata.ts](../../../apps/explorer/configs/app/features/addressMetadata.ts) | <code>NEXT_PUBLIC_METADATA_ADDRESS_TAGS_UPDATE_ENABLED</code> |
| <code>interface.14.env-fe-next-public-metadata-service-api-host</code> | <code>env_var</code> | [apps/explorer/configs/app/apis.ts](../../../apps/explorer/configs/app/apis.ts) | <code>NEXT_PUBLIC_METADATA_SERVICE_API_HOST</code> |

## env-metasuites

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-fe-next-public-metasuites-enabled</code> | <code>env_var</code> | [apps/explorer/configs/app/features/metasuites.ts](../../../apps/explorer/configs/app/features/metasuites.ts) | <code>NEXT_PUBLIC_METASUITES_ENABLED</code> |

## env-mixpanel

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-fe-next-public-mixpanel-config-overrides</code> | <code>env_var</code> | [apps/explorer/configs/app/features/mixpanel.ts](../../../apps/explorer/configs/app/features/mixpanel.ts) | <code>NEXT_PUBLIC_MIXPANEL_CONFIG_OVERRIDES</code> |
| <code>interface.14.env-fe-next-public-mixpanel-project-token</code> | <code>env_var</code> | [apps/explorer/configs/app/features/mixpanel.ts](../../../apps/explorer/configs/app/features/mixpanel.ts) | <code>NEXT_PUBLIC_MIXPANEL_PROJECT_TOKEN</code> |

## env-multichain

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-fe-next-public-multichain-aggregator-api-host</code> | <code>env_var</code> | [apps/explorer/configs/app/apis.ts](../../../apps/explorer/configs/app/apis.ts) | <code>NEXT_PUBLIC_MULTICHAIN_AGGREGATOR_API_HOST</code> |
| <code>interface.14.env-fe-next-public-multichain-balance-provider-config</code> | <code>env_var</code> | [apps/explorer/configs/app/features/multichainButton.ts](../../../apps/explorer/configs/app/features/multichainButton.ts) | <code>const value = parseEnvJson&lt;Array&lt;MultichainProviderConfig&gt;&gt;(getEnvValue('NEXT_PUBLIC_MULTICHAIN_BALANCE_PROVIDER_CONFIG'));</code> |
| <code>interface.14.env-fe-next-public-multichain-cluster</code> | <code>env_var</code> | [apps/explorer/configs/app/apis.ts](../../../apps/explorer/configs/app/apis.ts) | <code>NEXT_PUBLIC_MULTICHAIN_CLUSTER</code> |
| <code>interface.14.env-fe-next-public-multichain-enabled</code> | <code>env_var</code> | [apps/explorer/configs/app/features/multichain.ts](../../../apps/explorer/configs/app/features/multichain.ts) | <code>NEXT_PUBLIC_MULTICHAIN_ENABLED</code> |
| <code>interface.14.env-fe-next-public-multichain-stats-api-host</code> | <code>env_var</code> | [apps/explorer/configs/app/apis.ts](../../../apps/explorer/configs/app/apis.ts) | <code>NEXT_PUBLIC_MULTICHAIN_STATS_API_HOST</code> |

## env-name

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-fe-next-public-name-service-api-host</code> | <code>env_var</code> | [apps/explorer/configs/app/apis.ts](../../../apps/explorer/configs/app/apis.ts) | <code>NEXT_PUBLIC_NAME_SERVICE_API_HOST</code> |
| <code>interface.14.env-fe-next-public-name-service-protocols</code> | <code>env_var</code> | [apps/explorer/configs/app/features/nameServices.ts](../../../apps/explorer/configs/app/features/nameServices.ts) | <code>protocols: parseEnvJson&lt;Array&lt;string&gt;&gt;(getEnvValue('NEXT_PUBLIC_NAME_SERVICE_PROTOCOLS')) &#124;&#124; [ 'ens' ],</code> |

## env-navigation

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-fe-next-public-navigation-highlighted-routes</code> | <code>env_var</code> | [apps/explorer/configs/app/ui.ts](../../../apps/explorer/configs/app/ui.ts) | <code>const parsedValue = parseEnvJson&lt;Array&lt;string&gt;&gt;(getEnvValue('NEXT_PUBLIC_NAVIGATION_HIGHLIGHTED_ROUTES'));</code> |
| <code>interface.14.env-fe-next-public-navigation-layout</code> | <code>env_var</code> | [apps/explorer/configs/app/ui.ts](../../../apps/explorer/configs/app/ui.ts) | <code>NEXT_PUBLIC_NAVIGATION_LAYOUT</code> |
| <code>interface.14.env-fe-next-public-navigation-promo-banner-config</code> | <code>env_var</code> | [apps/explorer/configs/app/ui.ts](../../../apps/explorer/configs/app/ui.ts) | <code>const envValue = parseEnvJson&lt;NavigationPromoBannerConfig&gt;(getEnvValue('NEXT_PUBLIC_NAVIGATION_PROMO_BANNER_CONFIG'));</code> |

## env-network

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-fe-next-public-network-additional-token-types</code> | <code>env_var</code> | [apps/explorer/configs/app/chain.ts](../../../apps/explorer/configs/app/chain.ts) | <code>additionalTokenTypes: parseEnvJson&lt;Array&lt;AdditionalTokenType&gt;&gt;(getEnvValue('NEXT_PUBLIC_NETWORK_ADDITIONAL_TOKEN_TYPES')) &#124;&#124; [],</code> |
| <code>interface.14.env-fe-next-public-network-currency-decimals</code> | <code>env_var</code> | [apps/explorer/configs/app/chain.ts](../../../apps/explorer/configs/app/chain.ts) | <code>NEXT_PUBLIC_NETWORK_CURRENCY_DECIMALS</code> |
| <code>interface.14.env-fe-next-public-network-currency-gwei-name</code> | <code>env_var</code> | [apps/explorer/configs/app/chain.ts](../../../apps/explorer/configs/app/chain.ts) | <code>NEXT_PUBLIC_NETWORK_CURRENCY_GWEI_NAME</code> |
| <code>interface.14.env-fe-next-public-network-currency-name</code> | <code>env_var</code> | [apps/explorer/configs/app/chain.ts](../../../apps/explorer/configs/app/chain.ts) | <code>NEXT_PUBLIC_NETWORK_CURRENCY_NAME</code> |
| <code>interface.14.env-fe-next-public-network-currency-symbol</code> | <code>env_var</code> | [apps/explorer/configs/app/chain.ts](../../../apps/explorer/configs/app/chain.ts) | <code>NEXT_PUBLIC_NETWORK_CURRENCY_SYMBOL</code> |
| <code>interface.14.env-fe-next-public-network-currency-wei-name</code> | <code>env_var</code> | [apps/explorer/configs/app/chain.ts](../../../apps/explorer/configs/app/chain.ts) | <code>NEXT_PUBLIC_NETWORK_CURRENCY_WEI_NAME</code> |
| <code>interface.14.env-fe-next-public-network-explorers</code> | <code>env_var</code> | [apps/explorer/configs/app/ui.ts](../../../apps/explorer/configs/app/ui.ts) | <code>items: parseEnvJson&lt;Array&lt;NetworkExplorer&gt;&gt;(getEnvValue('NEXT_PUBLIC_NETWORK_EXPLORERS')) &#124;&#124; [],</code> |
| <code>interface.14.env-fe-next-public-network-icon</code> | <code>env_var</code> | [apps/explorer/configs/app/ui.ts](../../../apps/explorer/configs/app/ui.ts) | <code>NEXT_PUBLIC_NETWORK_ICON</code> |
| <code>interface.14.env-fe-next-public-network-icon-dark</code> | <code>env_var</code> | [apps/explorer/configs/app/ui.ts](../../../apps/explorer/configs/app/ui.ts) | <code>NEXT_PUBLIC_NETWORK_ICON_DARK</code> |
| <code>interface.14.env-fe-next-public-network-id</code> | <code>env_var</code> | [apps/explorer/configs/app/chain.ts](../../../apps/explorer/configs/app/chain.ts) | <code>NEXT_PUBLIC_NETWORK_ID</code> |
| <code>interface.14.env-fe-next-public-network-logo</code> | <code>env_var</code> | [apps/explorer/configs/app/ui.ts](../../../apps/explorer/configs/app/ui.ts) | <code>NEXT_PUBLIC_NETWORK_LOGO</code> |
| <code>interface.14.env-fe-next-public-network-logo-dark</code> | <code>env_var</code> | [apps/explorer/configs/app/ui.ts](../../../apps/explorer/configs/app/ui.ts) | <code>NEXT_PUBLIC_NETWORK_LOGO_DARK</code> |
| <code>interface.14.env-fe-next-public-network-multiple-gas-currencies</code> | <code>env_var</code> | [apps/explorer/configs/app/chain.ts](../../../apps/explorer/configs/app/chain.ts) | <code>NEXT_PUBLIC_NETWORK_MULTIPLE_GAS_CURRENCIES</code> |
| <code>interface.14.env-fe-next-public-network-name</code> | <code>env_var</code> | [apps/explorer/configs/app/chain.ts](../../../apps/explorer/configs/app/chain.ts) | <code>NEXT_PUBLIC_NETWORK_NAME</code> |
| <code>interface.14.env-fe-next-public-network-rpc-url</code> | <code>env_var</code> | [apps/explorer/configs/app/chain.ts](../../../apps/explorer/configs/app/chain.ts) | <code>NEXT_PUBLIC_NETWORK_RPC_URL</code> |
| <code>interface.14.env-fe-next-public-network-secondary-coin-symbol</code> | <code>env_var</code> | [apps/explorer/configs/app/chain.ts](../../../apps/explorer/configs/app/chain.ts) | <code>NEXT_PUBLIC_NETWORK_SECONDARY_COIN_SYMBOL</code> |
| <code>interface.14.env-fe-next-public-network-short-name</code> | <code>env_var</code> | [apps/explorer/configs/app/chain.ts](../../../apps/explorer/configs/app/chain.ts) | <code>NEXT_PUBLIC_NETWORK_SHORT_NAME</code> |
| <code>interface.14.env-fe-next-public-network-token-standard-name</code> | <code>env_var</code> | [apps/explorer/configs/app/chain.ts](../../../apps/explorer/configs/app/chain.ts) | <code>NEXT_PUBLIC_NETWORK_TOKEN_STANDARD_NAME</code> |
| <code>interface.14.env-fe-next-public-network-verification-type</code> | <code>env_var</code> | [apps/explorer/configs/app/chain.ts](../../../apps/explorer/configs/app/chain.ts) | <code>NEXT_PUBLIC_NETWORK_VERIFICATION_TYPE</code> |

## env-next

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-fe-next-runtime</code> | <code>env_var</code> | [apps/explorer/configs/essential-dapps-chains/config.edge.ts](../../../apps/explorer/configs/essential-dapps-chains/config.edge.ts) | <code>NEXT_RUNTIME</code> |

## env-og

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-fe-next-public-og-description</code> | <code>env_var</code> | [apps/explorer/configs/app/meta.ts](../../../apps/explorer/configs/app/meta.ts) | <code>NEXT_PUBLIC_OG_DESCRIPTION</code> |
| <code>interface.14.env-fe-next-public-og-enhanced-data-enabled</code> | <code>env_var</code> | [apps/explorer/configs/app/meta.ts](../../../apps/explorer/configs/app/meta.ts) | <code>NEXT_PUBLIC_OG_ENHANCED_DATA_ENABLED</code> |
| <code>interface.14.env-fe-next-public-og-image-url</code> | <code>env_var</code> | [apps/explorer/configs/app/meta.ts](../../../apps/explorer/configs/app/meta.ts) | <code>NEXT_PUBLIC_OG_IMAGE_URL</code> |

## env-other

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-fe-next-public-other-links</code> | <code>env_var</code> | [apps/explorer/configs/app/ui.ts](../../../apps/explorer/configs/app/ui.ts) | <code>otherLinks: parseEnvJson&lt;Array&lt;NavItemExternal&gt;&gt;(getEnvValue('NEXT_PUBLIC_OTHER_LINKS')) &#124;&#124; [],</code> |

## env-paxeer

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-fe-next-public-paxeer-x-enabled</code> | <code>env_var</code> | [apps/explorer/configs/app/features/paxeerXFeed.ts](../../../apps/explorer/configs/app/features/paxeerXFeed.ts) | <code>NEXT_PUBLIC_PAXEER_X_ENABLED</code> |

## env-promote

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-fe-next-public-promote-blockscout-in-title</code> | <code>env_var</code> | [apps/explorer/configs/app/meta.ts](../../../apps/explorer/configs/app/meta.ts) | <code>NEXT_PUBLIC_PROMOTE_BLOCKSCOUT_IN_TITLE</code> |

## env-puzzle

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-fe-next-public-puzzle-game-badge-claim-link</code> | <code>env_var</code> | [apps/explorer/configs/app/features/easterEggPuzzleBadge.ts](../../../apps/explorer/configs/app/features/easterEggPuzzleBadge.ts) | <code>NEXT_PUBLIC_PUZZLE_GAME_BADGE_CLAIM_LINK</code> |

## env-re

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-fe-next-public-re-captcha-app-site-key</code> | <code>env_var</code> | [apps/explorer/configs/app/services.ts](../../../apps/explorer/configs/app/services.ts) | <code>NEXT_PUBLIC_RE_CAPTCHA_APP_SITE_KEY</code> |

## env-rewards

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-fe-next-public-rewards-service-api-host</code> | <code>env_var</code> | [apps/explorer/configs/app/apis.ts](../../../apps/explorer/configs/app/apis.ts) | <code>NEXT_PUBLIC_REWARDS_SERVICE_API_HOST</code> |

## env-rollbar

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-fe-next-public-rollbar-client-token</code> | <code>env_var</code> | [apps/explorer/configs/app/features/rollbar.ts](../../../apps/explorer/configs/app/features/rollbar.ts) | <code>NEXT_PUBLIC_ROLLBAR_CLIENT_TOKEN</code> |

## env-rollup

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-fe-next-public-rollup-da-celestia-celenium-url</code> | <code>env_var</code> | [apps/explorer/configs/app/features/rollup.ts](../../../apps/explorer/configs/app/features/rollup.ts) | <code>NEXT_PUBLIC_ROLLUP_DA_CELESTIA_CELENIUM_URL</code> |
| <code>interface.14.env-fe-next-public-rollup-da-celestia-namespace</code> | <code>env_var</code> | [apps/explorer/configs/app/features/rollup.ts](../../../apps/explorer/configs/app/features/rollup.ts) | <code>NEXT_PUBLIC_ROLLUP_DA_CELESTIA_NAMESPACE</code> |
| <code>interface.14.env-fe-next-public-rollup-homepage-show-latest-blocks</code> | <code>env_var</code> | [apps/explorer/configs/app/features/rollup.ts](../../../apps/explorer/configs/app/features/rollup.ts) | <code>NEXT_PUBLIC_ROLLUP_HOMEPAGE_SHOW_LATEST_BLOCKS</code> |
| <code>interface.14.env-fe-next-public-rollup-l2-withdrawal-url</code> | <code>env_var</code> | [apps/explorer/configs/app/features/rollup.ts](../../../apps/explorer/configs/app/features/rollup.ts) | <code>NEXT_PUBLIC_ROLLUP_L2_WITHDRAWAL_URL</code> |
| <code>interface.14.env-fe-next-public-rollup-layer-number</code> | <code>env_var</code> | [apps/explorer/configs/app/features/rollup.ts](../../../apps/explorer/configs/app/features/rollup.ts) | <code>NEXT_PUBLIC_ROLLUP_LAYER_NUMBER</code> |
| <code>interface.14.env-fe-next-public-rollup-output-roots-enabled</code> | <code>env_var</code> | [apps/explorer/configs/app/features/rollup.ts](../../../apps/explorer/configs/app/features/rollup.ts) | <code>NEXT_PUBLIC_ROLLUP_OUTPUT_ROOTS_ENABLED</code> |
| <code>interface.14.env-fe-next-public-rollup-parent-chain</code> | <code>env_var</code> | [apps/explorer/configs/app/features/rollup.ts](../../../apps/explorer/configs/app/features/rollup.ts) | <code>const envValue = parseEnvJson&lt;ParentChain&gt;(getEnvValue('NEXT_PUBLIC_ROLLUP_PARENT_CHAIN'));</code> |
| <code>interface.14.env-fe-next-public-rollup-stage-index</code> | <code>env_var</code> | [apps/explorer/configs/app/features/rollup.ts](../../../apps/explorer/configs/app/features/rollup.ts) | <code>NEXT_PUBLIC_ROLLUP_STAGE_INDEX</code> |
| <code>interface.14.env-fe-next-public-rollup-type</code> | <code>env_var</code> | [apps/explorer/configs/app/chain.ts](../../../apps/explorer/configs/app/chain.ts) | <code>NEXT_PUBLIC_ROLLUP_TYPE</code> |

## env-safe

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-fe-next-public-safe-tx-service-url</code> | <code>env_var</code> | [apps/explorer/configs/app/features/safe.ts](../../../apps/explorer/configs/app/features/safe.ts) | <code>NEXT_PUBLIC_SAFE_TX_SERVICE_URL</code> |

## env-seo

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-fe-next-public-seo-enhanced-data-enabled</code> | <code>env_var</code> | [apps/explorer/configs/app/meta.ts](../../../apps/explorer/configs/app/meta.ts) | <code>NEXT_PUBLIC_SEO_ENHANCED_DATA_ENABLED</code> |

## env-stats

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-fe-next-public-stats-api-base-path</code> | <code>env_var</code> | [apps/explorer/configs/app/apis.ts](../../../apps/explorer/configs/app/apis.ts) | <code>NEXT_PUBLIC_STATS_API_BASE_PATH</code> |
| <code>interface.14.env-fe-next-public-stats-api-host</code> | <code>env_var</code> | [apps/explorer/configs/app/apis.ts](../../../apps/explorer/configs/app/apis.ts) | <code>NEXT_PUBLIC_STATS_API_HOST</code> |

## env-tac

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-fe-next-public-tac-operation-lifecycle-api-host</code> | <code>env_var</code> | [apps/explorer/configs/app/apis.ts](../../../apps/explorer/configs/app/apis.ts) | <code>NEXT_PUBLIC_TAC_OPERATION_LIFECYCLE_API_HOST</code> |
| <code>interface.14.env-fe-next-public-tac-ton-explorer-url</code> | <code>env_var</code> | [apps/explorer/configs/app/features/tac.ts](../../../apps/explorer/configs/app/features/tac.ts) | <code>NEXT_PUBLIC_TAC_TON_EXPLORER_URL</code> |

## env-transaction

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-fe-next-public-transaction-interpretation-provider</code> | <code>env_var</code> | [apps/explorer/configs/app/features/txInterpretation.ts](../../../apps/explorer/configs/app/features/txInterpretation.ts) | <code>NEXT_PUBLIC_TRANSACTION_INTERPRETATION_PROVIDER</code> |

## env-tx

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-fe-next-public-tx-external-transactions-config</code> | <code>env_var</code> | [apps/explorer/configs/app/features/externalTxs.ts](../../../apps/explorer/configs/app/features/externalTxs.ts) | <code>const externalTransactionsConfig = parseEnvJson&lt;TxExternalTxsConfig&gt;(getEnvValue('NEXT_PUBLIC_TX_EXTERNAL_TRANSACTIONS_CONFIG'));</code> |

## env-use

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-fe-next-public-use-next-js-proxy</code> | <code>env_var</code> | [apps/explorer/configs/app/app.ts](../../../apps/explorer/configs/app/app.ts) | <code>NEXT_PUBLIC_USE_NEXT_JS_PROXY</code> |

## env-user

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-fe-next-public-user-ops-indexer-api-host</code> | <code>env_var</code> | [apps/explorer/configs/app/apis.ts](../../../apps/explorer/configs/app/apis.ts) | <code>NEXT_PUBLIC_USER_OPS_INDEXER_API_HOST</code> |

## env-validators

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-fe-next-public-validators-chain-type</code> | <code>env_var</code> | [apps/explorer/configs/app/features/validators.ts](../../../apps/explorer/configs/app/features/validators.ts) | <code>NEXT_PUBLIC_VALIDATORS_CHAIN_TYPE</code> |

## env-views

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-fe-next-public-views-address-bech-32-prefix</code> | <code>env_var</code> | [apps/explorer/configs/app/ui/views/address.ts](../../../apps/explorer/configs/app/ui/views/address.ts) | <code>NEXT_PUBLIC_VIEWS_ADDRESS_BECH_32_PREFIX</code> |
| <code>interface.14.env-fe-next-public-views-address-format</code> | <code>env_var</code> | [apps/explorer/configs/app/ui/views/address.ts](../../../apps/explorer/configs/app/ui/views/address.ts) | <code>const value = (parseEnvJson&lt;Array&lt;AddressFormat&gt;&gt;(getEnvValue('NEXT_PUBLIC_VIEWS_ADDRESS_FORMAT')) &#124;&#124; [])</code> |
| <code>interface.14.env-fe-next-public-views-address-hidden-views</code> | <code>env_var</code> | [apps/explorer/configs/app/ui/views/address.ts](../../../apps/explorer/configs/app/ui/views/address.ts) | <code>const parsedValue = parseEnvJson&lt;Array&lt;AddressViewId&gt;&gt;(getEnvValue('NEXT_PUBLIC_VIEWS_ADDRESS_HIDDEN_VIEWS')) &#124;&#124; [];</code> |
| <code>interface.14.env-fe-next-public-views-address-identicon-type</code> | <code>env_var</code> | [apps/explorer/configs/app/ui/views/address.ts](../../../apps/explorer/configs/app/ui/views/address.ts) | <code>NEXT_PUBLIC_VIEWS_ADDRESS_IDENTICON_TYPE</code> |
| <code>interface.14.env-fe-next-public-views-address-native-token-address</code> | <code>env_var</code> | [apps/explorer/configs/app/ui/views/address.ts](../../../apps/explorer/configs/app/ui/views/address.ts) | <code>NEXT_PUBLIC_VIEWS_ADDRESS_NATIVE_TOKEN_ADDRESS</code> |
| <code>interface.14.env-fe-next-public-views-block-hidden-fields</code> | <code>env_var</code> | [apps/explorer/configs/app/ui/views/block.ts](../../../apps/explorer/configs/app/ui/views/block.ts) | <code>const parsedValue = parseEnvJson&lt;Array&lt;BlockFieldId&gt;&gt;(getEnvValue('NEXT_PUBLIC_VIEWS_BLOCK_HIDDEN_FIELDS')) &#124;&#124; [];</code> |
| <code>interface.14.env-fe-next-public-views-block-pending-update-alert-enabled</code> | <code>env_var</code> | [apps/explorer/configs/app/ui/views/block.ts](../../../apps/explorer/configs/app/ui/views/block.ts) | <code>NEXT_PUBLIC_VIEWS_BLOCK_PENDING_UPDATE_ALERT_ENABLED</code> |
| <code>interface.14.env-fe-next-public-views-contract-decoded-bytecode-enabled</code> | <code>env_var</code> | [apps/explorer/configs/app/ui/views/address.ts](../../../apps/explorer/configs/app/ui/views/address.ts) | <code>NEXT_PUBLIC_VIEWS_CONTRACT_DECODED_BYTECODE_ENABLED</code> |
| <code>interface.14.env-fe-next-public-views-contract-extra-verification-methods</code> | <code>env_var</code> | [apps/explorer/configs/app/ui/views/address.ts](../../../apps/explorer/configs/app/ui/views/address.ts) | <code>NEXT_PUBLIC_VIEWS_CONTRACT_EXTRA_VERIFICATION_METHODS</code> |
| <code>interface.14.env-fe-next-public-views-contract-solidityscan-enabled</code> | <code>env_var</code> | [apps/explorer/configs/app/ui/views/address.ts](../../../apps/explorer/configs/app/ui/views/address.ts) | <code>NEXT_PUBLIC_VIEWS_CONTRACT_SOLIDITYSCAN_ENABLED</code> |
| <code>interface.14.env-fe-next-public-views-nft-marketplaces</code> | <code>env_var</code> | [apps/explorer/configs/app/ui/views/nft.ts](../../../apps/explorer/configs/app/ui/views/nft.ts) | <code>const marketplaces = parseEnvJson&lt;Array&lt;NftMarketplaceItem&gt;&gt;(getEnvValue('NEXT_PUBLIC_VIEWS_NFT_MARKETPLACES')) &#124;&#124; [];</code> |
| <code>interface.14.env-fe-next-public-views-token-scam-toggle-enabled</code> | <code>env_var</code> | [apps/explorer/configs/app/ui/views/token.ts](../../../apps/explorer/configs/app/ui/views/token.ts) | <code>NEXT_PUBLIC_VIEWS_TOKEN_SCAM_TOGGLE_ENABLED</code> |
| <code>interface.14.env-fe-next-public-views-tx-additional-fields</code> | <code>env_var</code> | [apps/explorer/configs/app/ui/views/tx.ts](../../../apps/explorer/configs/app/ui/views/tx.ts) | <code>const parsedValue = parseEnvJson&lt;Array&lt;TxAdditionalFieldsId&gt;&gt;(getEnvValue('NEXT_PUBLIC_VIEWS_TX_ADDITIONAL_FIELDS')) &#124;&#124; [];</code> |
| <code>interface.14.env-fe-next-public-views-tx-grouped-fees</code> | <code>env_var</code> | [apps/explorer/configs/app/ui/views/tx.ts](../../../apps/explorer/configs/app/ui/views/tx.ts) | <code>NEXT_PUBLIC_VIEWS_TX_GROUPED_FEES</code> |
| <code>interface.14.env-fe-next-public-views-tx-hidden-fields</code> | <code>env_var</code> | [apps/explorer/configs/app/ui/views/tx.ts](../../../apps/explorer/configs/app/ui/views/tx.ts) | <code>const parsedValue = parseEnvJson&lt;Array&lt;TxFieldsId&gt;&gt;(getEnvValue('NEXT_PUBLIC_VIEWS_TX_HIDDEN_FIELDS')) &#124;&#124; [];</code> |

## env-visualize

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-fe-next-public-visualize-api-base-path</code> | <code>env_var</code> | [apps/explorer/configs/app/apis.ts](../../../apps/explorer/configs/app/apis.ts) | <code>NEXT_PUBLIC_VISUALIZE_API_BASE_PATH</code> |
| <code>interface.14.env-fe-next-public-visualize-api-host</code> | <code>env_var</code> | [apps/explorer/configs/app/apis.ts](../../../apps/explorer/configs/app/apis.ts) | <code>NEXT_PUBLIC_VISUALIZE_API_HOST</code> |

## env-wallet

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-fe-next-public-wallet-connect-featured-wallet-ids</code> | <code>env_var</code> | [apps/explorer/configs/app/features/blockchainInteraction.ts](../../../apps/explorer/configs/app/features/blockchainInteraction.ts) | <code>featuredWalletIds: parseEnvJson&lt;Array&lt;string&gt;&gt;(getEnvValue('NEXT_PUBLIC_WALLET_CONNECT_FEATURED_WALLET_IDS')) ?? [],</code> |
| <code>interface.14.env-fe-next-public-wallet-connect-project-id</code> | <code>env_var</code> | [apps/explorer/configs/app/features/blockchainInteraction.ts](../../../apps/explorer/configs/app/features/blockchainInteraction.ts) | <code>NEXT_PUBLIC_WALLET_CONNECT_PROJECT_ID</code> |

## env-web3

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-fe-next-public-web3-disable-add-token-to-wallet</code> | <code>env_var</code> | [apps/explorer/configs/app/features/web3Wallet.ts](../../../apps/explorer/configs/app/features/web3Wallet.ts) | <code>NEXT_PUBLIC_WEB3_DISABLE_ADD_TOKEN_TO_WALLET</code> |
| <code>interface.14.env-fe-next-public-web3-wallets</code> | <code>env_var</code> | [apps/explorer/configs/app/features/web3Wallet.ts](../../../apps/explorer/configs/app/features/web3Wallet.ts) | <code>NEXT_PUBLIC_WEB3_WALLETS</code> |

## env-xstar

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-fe-next-public-xstar-score-url</code> | <code>env_var</code> | [apps/explorer/configs/app/features/xStarScore.ts](../../../apps/explorer/configs/app/features/xStarScore.ts) | <code>NEXT_PUBLIC_XSTAR_SCORE_URL</code> |

## env-zetachain

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.env-fe-next-public-zetachain-external-search-config</code> | <code>env_var</code> | [apps/explorer/configs/app/features/zetachain.ts](../../../apps/explorer/configs/app/features/zetachain.ts) | <code>NEXT_PUBLIC_ZETACHAIN_EXTERNAL_SEARCH_CONFIG</code> |
| <code>interface.14.env-fe-next-public-zetachain-service-api-host</code> | <code>env_var</code> | [apps/explorer/configs/app/apis.ts](../../../apps/explorer/configs/app/apis.ts) | <code>NEXT_PUBLIC_ZETACHAIN_SERVICE_API_HOST</code> |
| <code>interface.14.env-fe-next-public-zetachain-service-chains-config-url</code> | <code>env_var</code> | [apps/explorer/configs/app/features/zetachain.ts](../../../apps/explorer/configs/app/features/zetachain.ts) | <code>NEXT_PUBLIC_ZETACHAIN_SERVICE_CHAINS_CONFIG_URL</code> |
