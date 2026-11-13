# Paxeer X Network documentation

Paxeer mainnet-beta is one BFT network with EVM chain ID `125` (`0x7d`). The kernel runs inside `paxd`, and the documentation describes the network, its systems, operations and reference material.

Public node JSON-RPC is served at the root of each node endpoint listed in the [public RPC reference](site/paxeer-docs/src/app/public-rpc/page.tsx). The node registers `px` methods for account, inference, history and kernel reads; `px_register` and `px_requestFunds` belong to the hosted gateway contract.

## Systems

Each row names the inventory directory, its README page when present, the inventory documentation pages, and the documentation-site route.

| System | Directory | Directory README | Pages | Site route |
| --- | --- | --- | --- | --- |
| Paxeer chain node (chain-node) | [daemon/paxd](../daemon/paxd/) | README.md not present | [docs/release/7.0.0-upgrade-plan.md](release/7.0.0-upgrade-plan.md), [docs/release/7.0.0-validator-runbook.md](release/7.0.0-validator-runbook.md), [docs/site/paxeer-docs/src/app/run-node/page.tsx](site/paxeer-docs/src/app/run-node/page.tsx) | `/run-node` |
| LayerX kernel host (kernel) | [modules/lxphost](../modules/lxphost/) | [modules/lxphost/README.md](../modules/lxphost/README.md) | [docs/system/kernel-modules.md](system/kernel-modules.md), [docs/site/paxeer-docs/src/app/layerx-kernel/page.tsx](site/paxeer-docs/src/app/layerx-kernel/page.tsx) | `/layerx-kernel` |
| Epoch module (module-epoch) | [modules/epoch](../modules/epoch/) | [modules/epoch/README.md](../modules/epoch/README.md) | [docs/system/chain-modules.md](system/chain-modules.md), [docs/site/paxeer-docs/src/app/modules/page.tsx](site/paxeer-docs/src/app/modules/page.tsx) | `/modules` |
| EVM module (module-evm) | [modules/evm](../modules/evm/) | [modules/evm/README.md](../modules/evm/README.md) | [docs/system/chain-modules.md](system/chain-modules.md), [docs/site/paxeer-docs/src/app/modules/page.tsx](site/paxeer-docs/src/app/modules/page.tsx) | `/modules` |
| Launchpad module (module-launchpad) | [modules/launchpad](../modules/launchpad/) | [modules/launchpad/README.md](../modules/launchpad/README.md) | [docs/system/chain-modules.md](system/chain-modules.md), [docs/site/paxeer-docs/src/app/modules/page.tsx](site/paxeer-docs/src/app/modules/page.tsx) | `/modules` |
| LayerX agent module (module-layerx-agent) | [modules/layerxagent](../modules/layerxagent/) | [modules/layerxagent/README.md](../modules/layerxagent/README.md) | [docs/system/chain-modules.md](system/chain-modules.md), [docs/site/paxeer-docs/src/app/modules/page.tsx](site/paxeer-docs/src/app/modules/page.tsx) | `/modules` |
| LayerX anchor module (module-layerx-anchor) | [modules/layerxanchor](../modules/layerxanchor/) | [modules/layerxanchor/README.md](../modules/layerxanchor/README.md) | [docs/system/chain-modules.md](system/chain-modules.md), [docs/site/paxeer-docs/src/app/modules/page.tsx](site/paxeer-docs/src/app/modules/page.tsx) | `/modules` |
| LayerX auth module (module-layerx-auth) | [modules/layerxauth](../modules/layerxauth/) | [modules/layerxauth/README.md](../modules/layerxauth/README.md) | [docs/system/chain-modules.md](system/chain-modules.md), [docs/site/paxeer-docs/src/app/modules/page.tsx](site/paxeer-docs/src/app/modules/page.tsx) | `/modules` |
| LayerX bridge module (module-layerx-bridge) | [modules/layerxbridge](../modules/layerxbridge/) | [modules/layerxbridge/README.md](../modules/layerxbridge/README.md) | [docs/system/chain-modules.md](system/chain-modules.md), [docs/site/paxeer-docs/src/app/modules/page.tsx](site/paxeer-docs/src/app/modules/page.tsx) | `/modules` |
| LayerX chains module (module-layerx-chains) | [modules/layerxchains](../modules/layerxchains/) | [modules/layerxchains/README.md](../modules/layerxchains/README.md) | [docs/system/chain-modules.md](system/chain-modules.md), [docs/site/paxeer-docs/src/app/modules/page.tsx](site/paxeer-docs/src/app/modules/page.tsx) | `/modules` |
| LayerX custody module (module-layerx-custody) | [modules/layerxcustody](../modules/layerxcustody/) | [modules/layerxcustody/README.md](../modules/layerxcustody/README.md) | [docs/system/chain-modules.md](system/chain-modules.md), [docs/site/paxeer-docs/src/app/modules/page.tsx](site/paxeer-docs/src/app/modules/page.tsx) | `/modules` |
| LayerX exchange module (module-layerx-exchange) | [modules/layerxexchange](../modules/layerxexchange/) | [modules/layerxexchange/README.md](../modules/layerxexchange/README.md) | [docs/system/chain-modules.md](system/chain-modules.md), [docs/site/paxeer-docs/src/app/modules/page.tsx](site/paxeer-docs/src/app/modules/page.tsx) | `/modules` |
| LayerX governance module (module-layerx-gov) | [modules/layerxgov](../modules/layerxgov/) | [modules/layerxgov/README.md](../modules/layerxgov/README.md) | [docs/system/chain-modules.md](system/chain-modules.md), [docs/site/paxeer-docs/src/app/modules/page.tsx](site/paxeer-docs/src/app/modules/page.tsx) | `/modules` |
| LayerX ledger module (module-layerx-ledger) | [modules/layerxledger](../modules/layerxledger/) | [modules/layerxledger/README.md](../modules/layerxledger/README.md) | [docs/system/chain-modules.md](system/chain-modules.md), [docs/site/paxeer-docs/src/app/modules/page.tsx](site/paxeer-docs/src/app/modules/page.tsx) | `/modules` |
| LayerX pay module (module-layerx-pay) | [modules/layerxpay](../modules/layerxpay/) | [modules/layerxpay/README.md](../modules/layerxpay/README.md) | [docs/system/chain-modules.md](system/chain-modules.md), [docs/site/paxeer-docs/src/app/modules/page.tsx](site/paxeer-docs/src/app/modules/page.tsx) | `/modules` |
| Mint module (module-mint) | [modules/mint](../modules/mint/) | [modules/mint/README.md](../modules/mint/README.md) | [docs/system/chain-modules.md](system/chain-modules.md), [docs/site/paxeer-docs/src/app/modules/page.tsx](site/paxeer-docs/src/app/modules/page.tsx) | `/modules` |
| Oracle module (module-oracle) | [modules/oracle](../modules/oracle/) | [modules/oracle/README.md](../modules/oracle/README.md) | [docs/system/chain-modules.md](system/chain-modules.md), [docs/site/paxeer-docs/src/app/modules/page.tsx](site/paxeer-docs/src/app/modules/page.tsx) | `/modules` |
| Perpetuals module (module-perps) | [modules/perps](../modules/perps/) | [modules/perps/README.md](../modules/perps/README.md) | [docs/system/chain-modules.md](system/chain-modules.md), [docs/site/paxeer-docs/src/app/modules/page.tsx](site/paxeer-docs/src/app/modules/page.tsx) | `/modules` |
| Price feed module (module-pricefeed) | [modules/pricefeed](../modules/pricefeed/) | [modules/pricefeed/README.md](../modules/pricefeed/README.md) | [docs/system/chain-modules.md](system/chain-modules.md), [docs/site/paxeer-docs/src/app/modules/page.tsx](site/paxeer-docs/src/app/modules/page.tsx) | `/modules` |
| Embedded sequencer module (module-sequencer) | [modules/sequencer](../modules/sequencer/) | [modules/sequencer/README.md](../modules/sequencer/README.md) | [docs/system/chain-modules.md](system/chain-modules.md), [docs/site/paxeer-docs/src/app/modules/page.tsx](site/paxeer-docs/src/app/modules/page.tsx) | `/modules` |
| Spot module (module-spot) | [modules/spot](../modules/spot/) | [modules/spot/README.md](../modules/spot/README.md) | [docs/system/chain-modules.md](system/chain-modules.md), [docs/site/paxeer-docs/src/app/modules/page.tsx](site/paxeer-docs/src/app/modules/page.tsx) | `/modules` |
| Store module (module-store) | [modules/store](../modules/store/) | README.md not present | [docs/system/chain-modules.md](system/chain-modules.md), [docs/site/paxeer-docs/src/app/modules/page.tsx](site/paxeer-docs/src/app/modules/page.tsx) | `/modules` |
| Token factory module (module-tokenfactory) | [modules/tokenfactory](../modules/tokenfactory/) | [modules/tokenfactory/README.md](../modules/tokenfactory/README.md) | [docs/system/chain-modules.md](system/chain-modules.md), [docs/site/paxeer-docs/src/app/modules/page.tsx](site/paxeer-docs/src/app/modules/page.tsx) | `/modules` |
| XWeb module (module-xweb) | [modules/xweb](../modules/xweb/) | [modules/xweb/README.md](../modules/xweb/README.md) | [docs/system/chain-modules.md](system/chain-modules.md), [docs/site/paxeer-docs/src/app/modules/page.tsx](site/paxeer-docs/src/app/modules/page.tsx) | `/modules` |
| Address binding precompiles (precompile-addr) | [precompiles/addr](../precompiles/addr/) | [precompiles/addr/README.md](../precompiles/addr/README.md) | [docs/system/precompiles.md](system/precompiles.md), [docs/site/paxeer-docs/src/app/precompiles/page.tsx](site/paxeer-docs/src/app/precompiles/page.tsx) | `/precompiles` |
| Bank precompile (precompile-bank) | [precompiles/bank](../precompiles/bank/) | [precompiles/bank/README.md](../precompiles/bank/README.md) | [docs/system/precompiles.md](system/precompiles.md), [docs/site/paxeer-docs/src/app/precompiles/page.tsx](site/paxeer-docs/src/app/precompiles/page.tsx) | `/precompiles` |
| Common token precompiles (precompile-common) | [precompiles/common](../precompiles/common/) | [precompiles/common/README.md](../precompiles/common/README.md) | [docs/system/precompiles.md](system/precompiles.md), [docs/site/paxeer-docs/src/app/precompiles/page.tsx](site/paxeer-docs/src/app/precompiles/page.tsx) | `/precompiles` |
| Distribution precompiles (precompile-distribution) | [precompiles/distribution](../precompiles/distribution/) | [precompiles/distribution/README.md](../precompiles/distribution/README.md) | [docs/system/precompiles.md](system/precompiles.md), [docs/site/paxeer-docs/src/app/precompiles/page.tsx](site/paxeer-docs/src/app/precompiles/page.tsx) | `/precompiles` |
| Fee token precompile (precompile-feetoken) | [precompiles/feetoken](../precompiles/feetoken/) | [precompiles/feetoken/README.md](../precompiles/feetoken/README.md) | [docs/system/precompiles.md](system/precompiles.md), [docs/site/paxeer-docs/src/app/precompiles/page.tsx](site/paxeer-docs/src/app/precompiles/page.tsx) | `/precompiles` |
| Governance precompile (precompile-gov) | [precompiles/gov](../precompiles/gov/) | [precompiles/gov/README.md](../precompiles/gov/README.md) | [docs/system/precompiles.md](system/precompiles.md), [docs/site/paxeer-docs/src/app/precompiles/page.tsx](site/paxeer-docs/src/app/precompiles/page.tsx) | `/precompiles` |
| IBC precompiles (precompile-ibc) | [precompiles/ibc](../precompiles/ibc/) | [precompiles/ibc/README.md](../precompiles/ibc/README.md) | [docs/system/precompiles.md](system/precompiles.md), [docs/site/paxeer-docs/src/app/precompiles/page.tsx](site/paxeer-docs/src/app/precompiles/page.tsx) | `/precompiles` |
| JSON precompile (precompile-json) | [precompiles/json](../precompiles/json/) | [precompiles/json/README.md](../precompiles/json/README.md) | [docs/system/precompiles.md](system/precompiles.md), [docs/site/paxeer-docs/src/app/precompiles/page.tsx](site/paxeer-docs/src/app/precompiles/page.tsx) | `/precompiles` |
| Launchpad precompiles (precompile-launchpad) | [precompiles/launchpad](../precompiles/launchpad/) | [precompiles/launchpad/README.md](../precompiles/launchpad/README.md) | [docs/system/precompiles.md](system/precompiles.md), [docs/site/paxeer-docs/src/app/precompiles/page.tsx](site/paxeer-docs/src/app/precompiles/page.tsx) | `/precompiles` |
| LayerX agent precompiles (precompile-layerx-agent) | [precompiles/layerxagent](../precompiles/layerxagent/) | [precompiles/layerxagent/README.md](../precompiles/layerxagent/README.md) | [docs/system/precompiles.md](system/precompiles.md), [docs/site/paxeer-docs/src/app/precompiles/page.tsx](site/paxeer-docs/src/app/precompiles/page.tsx) | `/precompiles` |
| LayerX anchor precompile (precompile-layerx-anchor) | [precompiles/layerxanchor](../precompiles/layerxanchor/) | [precompiles/layerxanchor/README.md](../precompiles/layerxanchor/README.md) | [docs/system/precompiles.md](system/precompiles.md), [docs/site/paxeer-docs/src/app/precompiles/page.tsx](site/paxeer-docs/src/app/precompiles/page.tsx) | `/precompiles` |
| LayerX auth precompile (precompile-layerx-auth) | [precompiles/layerxauth](../precompiles/layerxauth/) | [precompiles/layerxauth/README.md](../precompiles/layerxauth/README.md) | [docs/system/precompiles.md](system/precompiles.md), [docs/site/paxeer-docs/src/app/precompiles/page.tsx](site/paxeer-docs/src/app/precompiles/page.tsx) | `/precompiles` |
| LayerX bridge precompile (precompile-layerx-bridge) | [precompiles/layerxbridge](../precompiles/layerxbridge/) | [precompiles/layerxbridge/README.md](../precompiles/layerxbridge/README.md) | [docs/system/precompiles.md](system/precompiles.md), [docs/site/paxeer-docs/src/app/precompiles/page.tsx](site/paxeer-docs/src/app/precompiles/page.tsx) | `/precompiles` |
| LayerX chains precompile (precompile-layerx-chains) | [precompiles/layerxchains](../precompiles/layerxchains/) | [precompiles/layerxchains/README.md](../precompiles/layerxchains/README.md) | [docs/system/precompiles.md](system/precompiles.md), [docs/site/paxeer-docs/src/app/precompiles/page.tsx](site/paxeer-docs/src/app/precompiles/page.tsx) | `/precompiles` |
| LayerX custody precompile (precompile-layerx-custody) | [precompiles/layerxcustody](../precompiles/layerxcustody/) | [precompiles/layerxcustody/README.md](../precompiles/layerxcustody/README.md) | [docs/system/precompiles.md](system/precompiles.md), [docs/site/paxeer-docs/src/app/precompiles/page.tsx](site/paxeer-docs/src/app/precompiles/page.tsx) | `/precompiles` |
| LayerX exchange precompile (precompile-layerx-exchange) | [precompiles/layerxexchange](../precompiles/layerxexchange/) | [precompiles/layerxexchange/README.md](../precompiles/layerxexchange/README.md) | [docs/system/precompiles.md](system/precompiles.md), [docs/site/paxeer-docs/src/app/precompiles/page.tsx](site/paxeer-docs/src/app/precompiles/page.tsx) | `/precompiles` |
| LayerX ledger precompile (precompile-layerx-ledger) | [precompiles/layerxledger](../precompiles/layerxledger/) | [precompiles/layerxledger/README.md](../precompiles/layerxledger/README.md) | [docs/system/precompiles.md](system/precompiles.md), [docs/site/paxeer-docs/src/app/precompiles/page.tsx](site/paxeer-docs/src/app/precompiles/page.tsx) | `/precompiles` |
| LayerX pay precompile (precompile-layerx-pay) | [precompiles/layerxpay](../precompiles/layerxpay/) | [precompiles/layerxpay/README.md](../precompiles/layerxpay/README.md) | [docs/system/precompiles.md](system/precompiles.md), [docs/site/paxeer-docs/src/app/precompiles/page.tsx](site/paxeer-docs/src/app/precompiles/page.tsx) | `/precompiles` |
| LayerX programs precompile (precompile-layerx-programs) | [precompiles/layerxprograms](../precompiles/layerxprograms/) | [precompiles/layerxprograms/README.md](../precompiles/layerxprograms/README.md) | [docs/system/precompiles.md](system/precompiles.md), [docs/site/paxeer-docs/src/app/precompiles/page.tsx](site/paxeer-docs/src/app/precompiles/page.tsx) | `/precompiles` |
| LayerX verify precompile (precompile-layerx-verify) | [precompiles/layerxverify](../precompiles/layerxverify/) | [precompiles/layerxverify/README.md](../precompiles/layerxverify/README.md) | [docs/system/precompiles.md](system/precompiles.md), [docs/site/paxeer-docs/src/app/precompiles/page.tsx](site/paxeer-docs/src/app/precompiles/page.tsx) | `/precompiles` |
| Oracle precompile (precompile-oracle) | [precompiles/oracle](../precompiles/oracle/) | [precompiles/oracle/README.md](../precompiles/oracle/README.md) | [docs/system/precompiles.md](system/precompiles.md), [docs/site/paxeer-docs/src/app/precompiles/page.tsx](site/paxeer-docs/src/app/precompiles/page.tsx) | `/precompiles` |
| P-256 precompile (precompile-p256) | [precompiles/p256](../precompiles/p256/) | [precompiles/p256/README.md](../precompiles/p256/README.md) | [docs/system/precompiles.md](system/precompiles.md), [docs/site/paxeer-docs/src/app/precompiles/page.tsx](site/paxeer-docs/src/app/precompiles/page.tsx) | `/precompiles` |
| Perpetuals precompile (precompile-perps) | [precompiles/perps](../precompiles/perps/) | [precompiles/perps/README.md](../precompiles/perps/README.md) | [docs/system/precompiles.md](system/precompiles.md), [docs/site/paxeer-docs/src/app/precompiles/page.tsx](site/paxeer-docs/src/app/precompiles/page.tsx) | `/precompiles` |
| Pointer precompile (precompile-pointer) | [precompiles/pointer](../precompiles/pointer/) | [precompiles/pointer/README.md](../precompiles/pointer/README.md) | [docs/system/precompiles.md](system/precompiles.md), [docs/site/paxeer-docs/src/app/precompiles/page.tsx](site/paxeer-docs/src/app/precompiles/page.tsx) | `/precompiles` |
| Pointer view precompile (precompile-pointerview) | [precompiles/pointerview](../precompiles/pointerview/) | [precompiles/pointerview/README.md](../precompiles/pointerview/README.md) | [docs/system/precompiles.md](system/precompiles.md), [docs/site/paxeer-docs/src/app/precompiles/page.tsx](site/paxeer-docs/src/app/precompiles/page.tsx) | `/precompiles` |
| Price feed precompile (precompile-pricefeed) | [precompiles/pricefeed](../precompiles/pricefeed/) | [precompiles/pricefeed/README.md](../precompiles/pricefeed/README.md) | [docs/system/precompiles.md](system/precompiles.md), [docs/site/paxeer-docs/src/app/precompiles/page.tsx](site/paxeer-docs/src/app/precompiles/page.tsx) | `/precompiles` |
| Solo precompile (precompile-solo) | [precompiles/solo](../precompiles/solo/) | [precompiles/solo/README.md](../precompiles/solo/README.md) | [docs/system/precompiles.md](system/precompiles.md), [docs/site/paxeer-docs/src/app/precompiles/page.tsx](site/paxeer-docs/src/app/precompiles/page.tsx) | `/precompiles` |
| Spot precompiles (precompile-spot) | [precompiles/spot](../precompiles/spot/) | [precompiles/spot/README.md](../precompiles/spot/README.md) | [docs/system/precompiles.md](system/precompiles.md), [docs/site/paxeer-docs/src/app/precompiles/page.tsx](site/paxeer-docs/src/app/precompiles/page.tsx) | `/precompiles` |
| Staking precompile (precompile-staking) | [precompiles/staking](../precompiles/staking/) | [precompiles/staking/README.md](../precompiles/staking/README.md) | [docs/system/precompiles.md](system/precompiles.md), [docs/site/paxeer-docs/src/app/precompiles/page.tsx](site/paxeer-docs/src/app/precompiles/page.tsx) | `/precompiles` |
| Wasm precompile (precompile-wasmd) | [precompiles/wasmd](../precompiles/wasmd/) | [precompiles/wasmd/README.md](../precompiles/wasmd/README.md) | [docs/system/precompiles.md](system/precompiles.md), [docs/site/paxeer-docs/src/app/precompiles/page.tsx](site/paxeer-docs/src/app/precompiles/page.tsx) | `/precompiles` |
| XWeb precompile (precompile-xweb) | [precompiles/xweb](../precompiles/xweb/) | [precompiles/xweb/README.md](../precompiles/xweb/README.md) | [docs/system/precompiles.md](system/precompiles.md), [docs/site/paxeer-docs/src/app/precompiles/page.tsx](site/paxeer-docs/src/app/precompiles/page.tsx) | `/precompiles` |
| Hosted gateway (gateway) | [services/platform/hosted/gateway](../services/platform/hosted/gateway/) | [services/platform/hosted/gateway/README.md](../services/platform/hosted/gateway/README.md) | [docs/release/7.0.0-service-operator-runbook.md](release/7.0.0-service-operator-runbook.md), [docs/site/paxeer-docs/src/app/services/page.tsx](site/paxeer-docs/src/app/services/page.tsx) | `/services` |
| Explorer backend (explorer-backend) | [services/explorer/backend](../services/explorer/backend/) | [services/explorer/backend/README.md](../services/explorer/backend/README.md) | [docs/release/7.0.0-service-operator-runbook.md](release/7.0.0-service-operator-runbook.md), [docs/explorer/ENVS.md](explorer/ENVS.md) | No route listed |
| Agent daemon (agent-daemon) | [services/agent](../services/agent/) | [services/agent/README.md](../services/agent/README.md) | [docs/release/7.0.0-service-operator-runbook.md](release/7.0.0-service-operator-runbook.md), [docs/site/paxeer-docs/src/app/agent-runtime/page.tsx](site/paxeer-docs/src/app/agent-runtime/page.tsx) | `/agent-runtime` |
| Interop workspace (interop) | [services/interop](../services/interop/) | [services/interop/README.md](../services/interop/README.md) | [services/interop/README.md](../services/interop/README.md), [docs/site/paxeer-docs/src/app/interop/page.tsx](site/paxeer-docs/src/app/interop/page.tsx) | `/interop` |
| Bridge relayer (bridge-relayer) | [services/interop/crates/layerx-bridge-relayer](../services/interop/crates/layerx-bridge-relayer/) | README.md not present | [docs/release/7.0.0-service-operator-runbook.md](release/7.0.0-service-operator-runbook.md), [contracts/bridge/README.md](../contracts/bridge/README.md) | No route listed |
| Launchpad indexer (launchpad-indexer) | [services/launchpad/indexer](../services/launchpad/indexer/) | README.md not present | [docs/release/7.0.0-service-operator-runbook.md](release/7.0.0-service-operator-runbook.md), [docs/site/paxeer-docs/src/app/launchpad/page.tsx](site/paxeer-docs/src/app/launchpad/page.tsx) | `/launchpad` |
| Wallet gateway (wallet-gateway) | [services/wallet/gateway](../services/wallet/gateway/) | README.md not present | [docs/wallet/service/cutover.md](wallet/service/cutover.md) | `/accounts` |
| Human service (human-service) | [services/human](../services/human/) | [services/human/README.md](../services/human/README.md) | [services/human/README.md](../services/human/README.md), [docs/site/paxeer-docs/src/app/human-interface/page.tsx](site/paxeer-docs/src/app/human-interface/page.tsx) | `/human-interface` |
| Bridge deployment service (service-bridge) | [services/bridge](../services/bridge/) | [services/bridge/README.md](../services/bridge/README.md) | [docs/release/7.0.0-service-operator-runbook.md](release/7.0.0-service-operator-runbook.md), [contracts/bridge/README.md](../contracts/bridge/README.md) | No route listed |
| Launchpad service workspace (service-launchpad) | [services/launchpad](../services/launchpad/) | [services/launchpad/README.md](../services/launchpad/README.md) | [docs/release/7.0.0-service-operator-runbook.md](release/7.0.0-service-operator-runbook.md), [docs/site/paxeer-docs/src/app/launchpad/page.tsx](site/paxeer-docs/src/app/launchpad/page.tsx) | `/launchpad` |
| Platform hosted services (service-platform) | [services/platform](../services/platform/) | [services/platform/README.md](../services/platform/README.md) | [docs/site/paxeer-docs/src/app/services/page.tsx](site/paxeer-docs/src/app/services/page.tsx), [docs/release/7.0.0-service-operator-runbook.md](release/7.0.0-service-operator-runbook.md) | `/services` |
| Bridge application (bridge) | [apps/bridge](../apps/bridge/) | [apps/bridge/README.md](../apps/bridge/README.md) | [docs/release/7.0.0-service-operator-runbook.md](release/7.0.0-service-operator-runbook.md), [docs/site/paxeer-docs/src/app/bridge/page.tsx](site/paxeer-docs/src/app/bridge/page.tsx) | `/bridge` |
| Explorer application (explorer) | [apps/explorer](../apps/explorer/) | [apps/explorer/README.md](../apps/explorer/README.md) | [docs/release/7.0.0-service-operator-runbook.md](release/7.0.0-service-operator-runbook.md) | `/services` |
| Inference peer (inference-peer) | [apps/inference-peer](../apps/inference-peer/) | [apps/inference-peer/README.md](../apps/inference-peer/README.md) | [docs/site/paxeer-docs/src/app/agents/page.tsx](site/paxeer-docs/src/app/agents/page.tsx) | `/agents` |
| Launchpad web application (launchpad) | [apps/launchpad/web](../apps/launchpad/web/) | README.md not present | [docs/release/7.0.0-service-operator-runbook.md](release/7.0.0-service-operator-runbook.md), [docs/site/paxeer-docs/src/app/launchpad/page.tsx](site/paxeer-docs/src/app/launchpad/page.tsx) | `/launchpad` |
| Names application (names) | [apps/names](../apps/names/) | [apps/names/README.md](../apps/names/README.md) | [docs/system/sdk.md](system/sdk.md) | `/sdk` |
| Perpetuals application (perps) | [apps/perps](../apps/perps/) | [apps/perps/README.md](../apps/perps/README.md) | [docs/site/paxeer-docs/src/app/perpetuals/page.tsx](site/paxeer-docs/src/app/perpetuals/page.tsx), [docs/perps/data-api.md](perps/data-api.md) | `/perpetuals` |
| Wallet application (wallet) | [apps/wallet](../apps/wallet/) | [apps/wallet/README.md](../apps/wallet/README.md) | [docs/release/7.0.0-service-operator-runbook.md](release/7.0.0-service-operator-runbook.md) | `/accounts` |
| Human web application (human-web) | [apps/web](../apps/web/) | [apps/web/README.md](../apps/web/README.md) | [docs/site/paxeer-docs/src/app/human-interface/page.tsx](site/paxeer-docs/src/app/human-interface/page.tsx), [docs/platform/content/index.md](platform/content/index.md) | `/human-interface` |
| Bridge application (app-bridge) | [apps/bridge](../apps/bridge/) | [apps/bridge/README.md](../apps/bridge/README.md) | [docs/release/7.0.0-service-operator-runbook.md](release/7.0.0-service-operator-runbook.md), [docs/site/paxeer-docs/src/app/bridge/page.tsx](site/paxeer-docs/src/app/bridge/page.tsx) | `/bridge` |
| Explorer application (app-explorer) | [apps/explorer](../apps/explorer/) | [apps/explorer/README.md](../apps/explorer/README.md) | [docs/release/7.0.0-service-operator-runbook.md](release/7.0.0-service-operator-runbook.md), [docs/site/paxeer-docs/src/app/services/page.tsx](site/paxeer-docs/src/app/services/page.tsx) | `/services` |
| Launchpad web application (app-launchpad) | [apps/launchpad/web](../apps/launchpad/web/) | README.md not present | [docs/release/7.0.0-service-operator-runbook.md](release/7.0.0-service-operator-runbook.md), [docs/site/paxeer-docs/src/app/launchpad/page.tsx](site/paxeer-docs/src/app/launchpad/page.tsx) | `/launchpad` |
| Wallet application (app-wallet) | [apps/wallet](../apps/wallet/) | [apps/wallet/README.md](../apps/wallet/README.md) | [docs/release/7.0.0-service-operator-runbook.md](release/7.0.0-service-operator-runbook.md), [docs/site/paxeer-docs/src/app/accounts/page.tsx](site/paxeer-docs/src/app/accounts/page.tsx) | `/accounts` |
| Human web application (app-web) | [apps/web](../apps/web/) | [apps/web/README.md](../apps/web/README.md) | [docs/platform/content/index.md](platform/content/index.md), [docs/site/paxeer-docs/src/app/human-interface/page.tsx](site/paxeer-docs/src/app/human-interface/page.tsx) | `/human-interface` |
| Inference peer (app-inference-peer) | [apps/inference-peer](../apps/inference-peer/) | [apps/inference-peer/README.md](../apps/inference-peer/README.md) | [docs/site/paxeer-docs/src/app/agents/page.tsx](site/paxeer-docs/src/app/agents/page.tsx) | `/agents` |
| Names application (app-names) | [apps/names](../apps/names/) | [apps/names/README.md](../apps/names/README.md) | [docs/system/sdk.md](system/sdk.md) | `/names` |
| Perpetuals application (app-perps) | [apps/perps](../apps/perps/) | [apps/perps/README.md](../apps/perps/README.md) | [docs/perps/data-api.md](perps/data-api.md), [docs/site/paxeer-docs/src/app/perpetuals/page.tsx](site/paxeer-docs/src/app/perpetuals/page.tsx) | `/perpetuals` |
| Wallet SDK (wallet-sdk) | [sdk/wallet](../sdk/wallet/) | [sdk/wallet/README.md](../sdk/wallet/README.md) | [docs/system/sdk.md](system/sdk.md), [docs/site/paxeer-docs/src/app/sdk/page.tsx](site/paxeer-docs/src/app/sdk/page.tsx) | `/sdk` |
| Cosmos TypeScript SDK (cosmos-sdk) | [sdk/js](../sdk/js/) | [sdk/js/README.md](../sdk/js/README.md) | [docs/system/sdk.md](system/sdk.md), [docs/site/paxeer-docs/src/app/sdk/page.tsx](site/paxeer-docs/src/app/sdk/page.tsx) | `/sdk` |
| Names SDK (names-sdk) | [sdk/names](../sdk/names/) | README.md not present | [docs/site/paxeer-docs/src/app/sdk/page.tsx](site/paxeer-docs/src/app/sdk/page.tsx) | `/sdk` |
| Agent TypeScript SDK (agent-sdk) | [services/agent/sdk/typescript](../services/agent/sdk/typescript/) | README.md not present | [docs/system/sdk.md](system/sdk.md), [docs/site/paxeer-docs/src/app/sdk/page.tsx](site/paxeer-docs/src/app/sdk/page.tsx) | `/sdk` |
| Chainflow data SDK (chainflow-sdk) | [sdk/chainflow/typescript](../sdk/chainflow/typescript/) | [sdk/chainflow/typescript/README.md](../sdk/chainflow/typescript/README.md) | [docs/site/paxeer-docs/src/app/sdk/page.tsx](site/paxeer-docs/src/app/sdk/page.tsx) | `/sdk` |
| Programs Rust SDK (program-rust-sdk) | [programs/sdk/rust](../programs/sdk/rust/) | README.md not present | [docs/system/sdk.md](system/sdk.md), [docs/site/paxeer-docs/src/app/programs/page.tsx](site/paxeer-docs/src/app/programs/page.tsx) | `/programs` |
| Programs AssemblyScript SDK (program-assemblyscript-sdk) | [programs/sdk/assemblyscript](../programs/sdk/assemblyscript/) | README.md not present | [docs/site/paxeer-docs/src/app/programs/page.tsx](site/paxeer-docs/src/app/programs/page.tsx) | `/programs` |
| Agent Python SDK (agent-python-sdk) | [services/agent/sdk/python](../services/agent/sdk/python/) | README.md not present | [docs/system/sdk.md](system/sdk.md) | `/sdk` |
| Agent JVM SDK (agent-jvm-sdk) | [sdk/platform/jvm](../sdk/platform/jvm/) | [sdk/platform/jvm/README.md](../sdk/platform/jvm/README.md) | [docs/site/paxeer-docs/src/app/sdk/page.tsx](site/paxeer-docs/src/app/sdk/page.tsx), [sdk/platform/jvm/README.md](../sdk/platform/jvm/README.md) | `/sdk` |
| Agent .NET SDK (agent-dotnet-sdk) | [sdk/platform/dotnet](../sdk/platform/dotnet/) | README.md not present | [docs/platform/content/quickstart/csharp.md](platform/content/quickstart/csharp.md) | `/sdk` |
| Agent Swift SDK (agent-swift-sdk) | [sdk/platform/swift](../sdk/platform/swift/) | README.md not present | [docs/platform/content/quickstart/swift.md](platform/content/quickstart/swift.md) | `/sdk` |
| Chainflow Go SDK (chainflow-go-sdk) | [sdk/chainflow/go](../sdk/chainflow/go/) | [sdk/chainflow/go/README.md](../sdk/chainflow/go/README.md) | [sdk/chainflow/go/README.md](../sdk/chainflow/go/README.md) | No route listed |
| Chainflow Python SDK (chainflow-python-sdk) | [sdk/chainflow/python](../sdk/chainflow/python/) | [sdk/chainflow/python/README.md](../sdk/chainflow/python/README.md) | [docs/perps/data-api.md](perps/data-api.md), [sdk/chainflow/python/README.md](../sdk/chainflow/python/README.md) | No route listed |
| Chainflow Rust SDK (chainflow-rust-sdk) | [sdk/chainflow/rust](../sdk/chainflow/rust/) | [sdk/chainflow/rust/README.md](../sdk/chainflow/rust/README.md) | [docs/perps/data-api.md](perps/data-api.md), [sdk/chainflow/rust/README.md](../sdk/chainflow/rust/README.md) | No route listed |
| Wallet SDK (sdk-wallet) | [sdk/wallet](../sdk/wallet/) | [sdk/wallet/README.md](../sdk/wallet/README.md) | [docs/system/sdk.md](system/sdk.md), [docs/site/paxeer-docs/src/app/sdk/page.tsx](site/paxeer-docs/src/app/sdk/page.tsx) | `/sdk` |
| Cosmos TypeScript SDK (sdk-cosmos) | [sdk/js](../sdk/js/) | [sdk/js/README.md](../sdk/js/README.md) | [docs/system/sdk.md](system/sdk.md), [docs/site/paxeer-docs/src/app/sdk/page.tsx](site/paxeer-docs/src/app/sdk/page.tsx) | `/sdk` |
| Names SDK (sdk-names) | [sdk/names](../sdk/names/) | README.md not present | [docs/site/paxeer-docs/src/app/sdk/page.tsx](site/paxeer-docs/src/app/sdk/page.tsx) | `/sdk` |
| Agent TypeScript SDK (sdk-agent-typescript) | [services/agent/sdk/typescript](../services/agent/sdk/typescript/) | README.md not present | [docs/system/sdk.md](system/sdk.md), [docs/site/paxeer-docs/src/app/sdk/page.tsx](site/paxeer-docs/src/app/sdk/page.tsx) | `/sdk` |
| Programs Rust SDK (sdk-program-rust) | [programs/sdk/rust](../programs/sdk/rust/) | README.md not present | [docs/system/sdk.md](system/sdk.md), [docs/site/paxeer-docs/src/app/programs/page.tsx](site/paxeer-docs/src/app/programs/page.tsx) | `/programs` |
| Programs AssemblyScript SDK (sdk-program-assemblyscript) | [programs/sdk/assemblyscript](../programs/sdk/assemblyscript/) | README.md not present | [docs/site/paxeer-docs/src/app/programs/page.tsx](site/paxeer-docs/src/app/programs/page.tsx) | `/programs` |
| Chainflow TypeScript SDK (sdk-chainflow-typescript) | [sdk/chainflow/typescript](../sdk/chainflow/typescript/) | [sdk/chainflow/typescript/README.md](../sdk/chainflow/typescript/README.md) | [docs/site/paxeer-docs/src/app/sdk/page.tsx](site/paxeer-docs/src/app/sdk/page.tsx) | `/sdk` |
| Chainflow Go SDK (sdk-chainflow-go) | [sdk/chainflow/go](../sdk/chainflow/go/) | [sdk/chainflow/go/README.md](../sdk/chainflow/go/README.md) | [sdk/chainflow/go/README.md](../sdk/chainflow/go/README.md) | No route listed |
| Chainflow Python SDK (sdk-chainflow-python) | [sdk/chainflow/python](../sdk/chainflow/python/) | [sdk/chainflow/python/README.md](../sdk/chainflow/python/README.md) | [docs/perps/data-api.md](perps/data-api.md), [sdk/chainflow/python/README.md](../sdk/chainflow/python/README.md) | No route listed |
| Chainflow Rust SDK (sdk-chainflow-rust) | [sdk/chainflow/rust](../sdk/chainflow/rust/) | [sdk/chainflow/rust/README.md](../sdk/chainflow/rust/README.md) | [docs/perps/data-api.md](perps/data-api.md), [sdk/chainflow/rust/README.md](../sdk/chainflow/rust/README.md) | No route listed |
| Agent JVM SDK (sdk-agent-jvm) | [sdk/platform/jvm](../sdk/platform/jvm/) | [sdk/platform/jvm/README.md](../sdk/platform/jvm/README.md) | [docs/site/paxeer-docs/src/app/sdk/page.tsx](site/paxeer-docs/src/app/sdk/page.tsx) | `/sdk` |
| Agent .NET SDK (sdk-agent-dotnet) | [sdk/platform/dotnet](../sdk/platform/dotnet/) | README.md not present | [docs/site/paxeer-docs/src/app/sdk/page.tsx](site/paxeer-docs/src/app/sdk/page.tsx) | `/sdk` |
| Agent Swift SDK (sdk-agent-swift) | [sdk/platform/swift](../sdk/platform/swift/) | README.md not present | [docs/site/paxeer-docs/src/app/sdk/page.tsx](site/paxeer-docs/src/app/sdk/page.tsx) | `/sdk` |
| Agent Go SDK (sdk-agent-go) | [sdk/platform/go](../sdk/platform/go/) | README.md not present | [docs/site/paxeer-docs/src/app/sdk/page.tsx](site/paxeer-docs/src/app/sdk/page.tsx) | `/sdk` |
| Release tools (release-tools) | [tools/release](../tools/release/) | [tools/release/README.md](../tools/release/README.md) | [docs/release/7.0.0-upgrade-plan.md](release/7.0.0-upgrade-plan.md), [docs/release/7.0.0-validator-runbook.md](release/7.0.0-validator-runbook.md) | No route listed |
| Paxctl operator CLI (paxctl) | [tools/paxctl](../tools/paxctl/) | [tools/paxctl/README.md](../tools/paxctl/README.md) | [docs/site/paxeer-docs/src/app/cli/page.tsx](site/paxeer-docs/src/app/cli/page.tsx) | `/cli` |
| LayerX developer CLI (layerx-cli) | [tools/platform/cli](../tools/platform/cli/) | [tools/platform/cli/README.md](../tools/platform/cli/README.md) | [docs/system/cli.md](system/cli.md), [docs/site/paxeer-docs/src/app/cli/page.tsx](site/paxeer-docs/src/app/cli/page.tsx) | `/cli` |
| Wasm tools (wasm-tools) | [tools/wasm-tools](../tools/wasm-tools/) | [tools/wasm-tools/README.md](../tools/wasm-tools/README.md) | [docs/site/paxeer-docs/src/app/wasm/page.tsx](site/paxeer-docs/src/app/wasm/page.tsx) | `/wasm` |
| Load test tool (loadtest) | [tools/loadtest](../tools/loadtest/) | [tools/loadtest/README.md](../tools/loadtest/README.md) | [docs/site/paxeer-docs/src/app/debugging-transactions/page.tsx](site/paxeer-docs/src/app/debugging-transactions/page.tsx) | `/debugging-transactions` |
| System documentation generator (docs-generator) | [tools/docs](../tools/docs/) | README.md not present | [docs/system/INDEX.md](system/INDEX.md) | No route listed |
| Node bring-up tool (bringup) | [tools/bringup](../tools/bringup/) | [tools/bringup/README.md](../tools/bringup/README.md) | [docs/site/paxeer-docs/src/app/installation/page.tsx](site/paxeer-docs/src/app/installation/page.tsx) | `/installation` |
| Release rollback tool (rollback-tool) | [tools/release](../tools/release/) | [tools/release/README.md](../tools/release/README.md) | [docs/release/7.0.0-upgrade-plan.md](release/7.0.0-upgrade-plan.md) | No route listed |
| Backup and monitoring tools (operations) | [tools/ops](../tools/ops/) | [tools/ops/README.md](../tools/ops/README.md) | [docs/release/7.0.0-validator-runbook.md](release/7.0.0-validator-runbook.md), [docs/site/paxeer-docs/src/app/network-operations/page.tsx](site/paxeer-docs/src/app/network-operations/page.tsx) | `/network-operations` |
| Backup and restore tools (backup-tools) | [tools/ops/backup](../tools/ops/backup/) | README.md not present | [docs/release/7.0.0-validator-runbook.md](release/7.0.0-validator-runbook.md), [docs/site/paxeer-docs/src/app/network-operations/page.tsx](site/paxeer-docs/src/app/network-operations/page.tsx) | `/network-operations` |
| Monitoring stack (monitoring-tools) | [tools/ops/monitoring](../tools/ops/monitoring/) | README.md not present | [docs/release/7.0.0-validator-runbook.md](release/7.0.0-validator-runbook.md), [docs/site/paxeer-docs/src/app/network-operations/page.tsx](site/paxeer-docs/src/app/network-operations/page.tsx) | `/network-operations` |
| Paxeer load generator (paxload) | [tools/loadtest](../tools/loadtest/) | [tools/loadtest/README.md](../tools/loadtest/README.md) | [docs/site/paxeer-docs/src/app/operators/page.tsx](site/paxeer-docs/src/app/operators/page.tsx) | `/operators` |
| Cosmovisor supervisor (cosmovisor) | [sdk/cosmovisor](../sdk/cosmovisor/) | [sdk/cosmovisor/README.md](../sdk/cosmovisor/README.md) | [docs/release/7.0.0-validator-runbook.md](release/7.0.0-validator-runbook.md), [docs/site/paxeer-docs/src/app/operators/page.tsx](site/paxeer-docs/src/app/operators/page.tsx) | `/operators` |

## Operations

Release pages, runbooks and public endpoint references are listed here.

### Release pages

- [docs/release/7.0.0-candidate-report.md](release/7.0.0-candidate-report.md)
- [docs/release/7.0.0-proposal.md](release/7.0.0-proposal.md)
- [docs/release/7.0.0-readiness-checklist.md](release/7.0.0-readiness-checklist.md)
- [docs/release/7.0.0-readiness-matrix.md](release/7.0.0-readiness-matrix.md)
- [docs/release/7.0.0-release-notes.md](release/7.0.0-release-notes.md)
- [docs/release/7.0.0-rollback.md](release/7.0.0-rollback.md)
- [docs/release/7.0.0-service-operator-runbook.md](release/7.0.0-service-operator-runbook.md)
- [docs/release/7.0.0-systems.md](release/7.0.0-systems.md)
- [docs/release/7.0.0-upgrade-plan.md](release/7.0.0-upgrade-plan.md)
- [docs/release/7.0.0-validator-runbook.md](release/7.0.0-validator-runbook.md)
- [docs/release/consensus-commits.md](release/consensus-commits.md)

### Runbooks

- [docs/runbooks/paxeer-x-fork.md](runbooks/paxeer-x-fork.md)
- [docs/release/7.0.0-service-operator-runbook.md](release/7.0.0-service-operator-runbook.md)
- [docs/release/7.0.0-validator-runbook.md](release/7.0.0-validator-runbook.md)
- [docs/wallet/app/operations/runbooks.md](wallet/app/operations/runbooks.md)

### Public endpoints

- [Public RPC reference](site/paxeer-docs/src/app/public-rpc/page.tsx) — site route `/public-rpc`.
- [Network overview](paxeer-network.md) — describes the node JSON-RPC and tenant-domain views.
- [RPC reference](system/rpc.md) — documents the registered JSON-RPC methods.

## Reference

System reference pages:
- [docs/system/INDEX.md](system/INDEX.md)
- [docs/system/chain-modules.md](system/chain-modules.md)
- [docs/system/cli.md](system/cli.md)
- [docs/system/governance.md](system/governance.md)
- [docs/system/kernel-modules.md](system/kernel-modules.md)
- [docs/system/precompiles.md](system/precompiles.md)
- [docs/system/routes.md](system/routes.md)
- [docs/system/rpc.md](system/rpc.md)
- [docs/system/sdk.md](system/sdk.md)

Other documentation directories:
- [agent/](agent/)
- [build/](build/)
- [ci/](ci/)
- [explorer/](explorer/)
- [hyperfide/](hyperfide/)
- [interop/](interop/)
- [launchpad/](launchpad/)
- [perps/](perps/)
- [platform/](platform/)
- [readme/](readme/)
- [release/](release/)
- [runbooks/](runbooks/)
- [site/](site/)
- [swagger/](swagger/)
- [swagger-ui/](swagger-ui/)
- [wallet/](wallet/)
- [wiki/](wiki/)

The [monorepo layout](MONOREPO.md), [qualification guide](QUALIFICATION.md), [documentation site](site/paxeer-docs/README.md), [wiki](wiki/Home.md), [platform guides](platform/), [wallet guides](wallet/), [explorer documentation](explorer/), [agent documentation](agent/), [perpetuals documentation](perps/), and [interoperability documentation](interop/) provide additional current details.

Protocol specifications live under [spec/](../spec/).
