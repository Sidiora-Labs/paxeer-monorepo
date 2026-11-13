# Off-chain service contracts

Application services communicate with one Paxeer BFT network. `paxd` contains the sequencer runtime and shared kernel host; EVM chain ID is `125` (`0x7d`), and block commitment establishes finality. Hosted service responses and indexed projections do not establish another finality stage.

The code roots below identify the service implementations. HTTP routes, configuration and signing boundaries belong to those implementations and the linked component contracts. Node JSON-RPC uses its root path; hosted methods are declared in [the gateway OpenRPC contract](../../../services/platform/hosted/gateway/openrpc.json).

## Services

### perps-data

Indexes the perps 0x1015, pricefeed 0x1020 and spot 0x1021-0x1025 events into the market data API.

Implementation: [services/platform/perps-data](../../../services/platform/perps-data).

### launchpad-gateway

Public gateway of the launchpad service set over the 0x1017 venue.

Implementation: [services/launchpad/gateway](../../../services/launchpad/gateway).

### launchpad-indexer

Indexes launchpad 0x1017 factory and pool events.

Implementation: [services/launchpad/indexer](../../../services/launchpad/indexer).

### sequencer

The node-integrated runtime executes transactions on open block state, signs receipts, replicates its WAL and seals batches for consensus. Its receipt and recovery behavior is described in [sequencer](sequencer.md).

Implementation: [node/sequencer](../../../node/sequencer), [modules/sequencer](../../../modules/sequencer), [rpc/sequencer_receipt.go](../../../rpc/sequencer_receipt.go).

### platform-agent-boundary

Authenticated boundary between the gateway, registry and webhook planes and the chain for receipt and deployment-proof reads

Implementation: [services/platform/hosted/agent-boundary](../../../services/platform/hosted/agent-boundary).

### platform-ai-artifacts

Encrypted model, dataset and evidence storage.

Implementation: [services/platform/hosted/ai-artifacts](../../../services/platform/hosted/ai-artifacts).

### platform-ai-evaluator

Signs assessments and commits and reveals scores for ai-market epochs.

Implementation: [services/platform/hosted/ai-evaluator](../../../services/platform/hosted/ai-evaluator).

### platform-core-boundary

Hosted core boundary: sequencer and state reads, receipts and the admin testnet fund and reset plane

Implementation: [services/platform/hosted/core](../../../services/platform/hosted/core).

### platform-dashboard

Operator dashboard of the hosted platform services.

Implementation: [services/platform/hosted/dashboard](../../../services/platform/hosted/dashboard).

### platform-faucet

Funds admitted claims by an EVM bank send from the faucet account.

Implementation: [services/platform/hosted/faucet](../../../services/platform/hosted/faucet).

### platform-gateway

Public router of the platform: JSON-RPC px_ methods, the relayed EVM namespaces, WebSocket subscriptions, HTTP product routes and UI proxies

Implementation: [services/platform/hosted/gateway](../../../services/platform/hosted/gateway).

### platform-identity

Registers tenants and keys and serves the binding for addr 0x1004 bindLayerX, through px_register.

Implementation: [services/platform/hosted/identity](../../../services/platform/hosted/identity).

### platform-indexer

Decodes paxeer chain blocks, logs and consensus events into account history, assets and holder records

Implementation: [services/platform/hosted/indexer](../../../services/platform/hosted/indexer).

### platform-paxai-worker

Inference and embedding worker submitting ai-market operations.

Implementation: [services/platform/hosted/paxai-worker](../../../services/platform/hosted/paxai-worker).

### platform-price-feeder

Signs price observations and pushes them to pricefeed 0x1020 (decision.1702); the 0x1015 pushOracle path is void (decision.304).

Implementation: [services/platform/hosted/feeder](../../../services/platform/hosted/feeder).

### platform-program-registry

Publishes verified program records and heads read from the programs construct

Implementation: [services/platform/hosted/registry](../../../services/platform/hosted/registry).

### platform-receipt-authority

Admits and serves sequencer receipts for hosted clients after verifying them against pinned keys

Implementation: [services/platform/hosted/authority](../../../services/platform/hosted/authority).

### platform-runtime-clock

Hosted runtime clock for program and stream ticks of the platform.

Implementation: [services/platform/hosted/runtime-clock](../../../services/platform/hosted/runtime-clock).

### platform-testnet-control

Testnet control plane reporting journey readiness and forwarding funding and reset commands

Implementation: [services/platform/hosted/testnet](../../../services/platform/hosted/testnet).

### platform-webhooks

Signs and delivers protocol event webhooks to registered endpoints

Implementation: [services/platform/hosted/webhooks](../../../services/platform/hosted/webhooks).

### agent-mcp

MCP server exposing the agent API tools.

Implementation: [services/agent/crates/layerx-mcp](../../../services/agent/crates/layerx-mcp).

### agentd

Agent daemon and host: policy, approvals, sessions, outbox and web payer.

Implementation: [services/agent/crates/layerx-agentd](../../../services/agent/crates/layerx-agentd), [services/agent/crates/layerx-agentd-host](../../../services/agent/crates/layerx-agentd-host).

### human-kms

Identity provider, security provider and KMS over Unix sockets.

Implementation: [services/human/crates/layerx-human-kms](../../../services/human/crates/layerx-human-kms).

### human-network-gateway

Network gateway of the human services toward the chain.

Implementation: [services/human/crates/layerx-network-gateway](../../../services/human/crates/layerx-network-gateway).

### human-service

Web home, moves, agents, approvals, statements, native send and onboarding.

Implementation: [services/human/crates/layerx-human-service](../../../services/human/crates/layerx-human-service).

### bridge-relayer

Relays attested remote deposits by bridgeIn and BridgeOut releases.

Implementation: [services/interop/crates/layerx-bridge-relayer](../../../services/interop/crates/layerx-bridge-relayer).

### gas-station

Sponsors user batches through BatchCallAndSponsor with per-DID budgets.

Implementation: [services/interop/crates/layerx-gas-station](../../../services/interop/crates/layerx-gas-station).

### interop-gateway

Public gateway of the interop protocols (x402, AP2, UCP, Visa TAP).

Implementation: [services/interop/crates/layerx-interop-gateway](../../../services/interop/crates/layerx-interop-gateway).

### interop-service

Payment protocol service and adapters settling on layerxledger with sequencer receipts.

Implementation: [services/interop/crates/layerx-interop-service](../../../services/interop/crates/layerx-interop-service).

### mirror-publisher

Archives committed Paxeer headers and app hashes to Ethereum and Solana.

Implementation: [services/interop/crates/layerx-mirror](../../../services/interop/crates/layerx-mirror).

### mirror-signer

Private socket signer for the mirror and bridge.

Implementation: [services/interop/crates/layerx-mirror-signer](../../../services/interop/crates/layerx-mirror-signer).

### x-websearch

Per-node sidecar: search, attest and fulfil xweb requests; paid HTTP settled by layerxledger draw.

Implementation: [services/interop/crates/x-websearch](../../../services/interop/crates/x-websearch).

### x402-seller-middleware

Merchant middleware verifying sequencer receipts before serving.

Implementation: [services/interop/crates/layerx-x402](../../../services/interop/crates/layerx-x402), [services/platform/middleware/seller](../../../services/platform/middleware/seller).

### embedded-wallet-gateway

Embedded wallet API signing through the attestor quorum.

Implementation: [services/wallet/gateway](../../../services/wallet/gateway).

### wallet-attestors

Key shards and threshold signing for the embedded wallet.

Implementation: [services/wallet/attestor](../../../services/wallet/attestor).

### bridge-deploy

Bridge chain deployment and verification scripts with the relayer configuration.

Implementation: [services/bridge/deploy](../../../services/bridge/deploy).

### explorer

Explorer fork and explorer index over one chain.

Implementation: [services/explorer](../../../services/explorer).

### merge-tools

One-shot tools run at the activation height.

Implementation: [services/interop/crates/layerx-migrate](../../../services/interop/crates/layerx-migrate), [tools/paxeer-x](../../../tools/paxeer-x).

### rollout-controller

Deploys services at pinned revisions in wave order.

Implementation: [tools/rollout](../../../tools/rollout).

## Receipts and reads

A signed receipt is checked against the key identifier and inclusive height range. Its advertised signing key alone is not a trust pin. `eth_getTransactionReceipt` returns committed EVM receipt data; `pax_sequencerReceipt` reports the supported receipt stage. Indexed API data carries the indexer's observed height rather than transaction finality.

## Component contracts

[Hosted platform](platform-hosted.md), [gateway](platform-gateway.md), [market data](perps-data.md), [MCP tools](mcp-tools.md), [ramps](platform-ramps.md), [middleware](platform-middleware.md) and [verifier](verifier-http.md) describe the corresponding APIs and configuration declarations.

<a id="1-reading-a-service-contract"></a>

<a id="2-catalogue"></a>

<a id="3-dependencies"></a>

<a id="4-service-contracts"></a>

<a id="domain-3-markets"></a>

<a id="domain-4-launchpad"></a>

<a id="domain-5-node-runtime"></a>

<a id="domain-8-platform"></a>

<a id="domain-9-agent"></a>

<a id="domain-10-human"></a>

<a id="human-explorer-index"></a>

<a id="domain-11-interop"></a>

<a id="domain-12-wallet"></a>

<a id="domain-14-web-explorer-bridge"></a>

<a id="domain-16-tools-ops"></a>

<a id="services-without-a-service-block"></a>

<a id="archive-node"></a>

<a id="monitoring"></a>

<a id="validator-oracle-feeders"></a>

<a id="perps-keeper"></a>

<a id="wallet-app-backends"></a>

<a id="5-interface-groups-of-the-service-trees"></a>

<a id="dashboard-errors"></a>

<a id="human-api-errors"></a>

<a id="human-api-routes"></a>

<a id="human-api-schema"></a>

<a id="human-identity-provider"></a>

<a id="human-intents"></a>

<a id="human-kms-wire"></a>

<a id="human-movement-provider"></a>

<a id="human-paxeer-client"></a>

<a id="human-security-provider"></a>

<a id="human-service-config"></a>

<a id="human-test-support"></a>

<a id="human-tools"></a>

<a id="launchpad-api"></a>

<a id="launchpad-discovery"></a>

<a id="launchpad-livestream"></a>

<a id="launchpad-metadata"></a>

<a id="launchpad-notifications"></a>

<a id="launchpad-pnl-tracker"></a>

<a id="launchpad-ranking-algo"></a>

<a id="launchpad-security"></a>

<a id="launchpad-social"></a>

<a id="launchpad-stats-workers"></a>

<a id="launchpad-trading-charts"></a>

<a id="launchpad-user"></a>

<a id="launchpad-workspace"></a>

<a id="layerx-agent-api-errors"></a>

<a id="layerx-agentd-errors"></a>

<a id="layerx-agentd-metrics"></a>

<a id="layerx-ap2-errors"></a>

<a id="layerx-interop-gateway-errors"></a>

<a id="layerx-types-errors"></a>

<a id="perf-db-errors"></a>

<a id="relay-archive-retirement"></a>

<a id="s32-wallet-app-backends"></a>

<a id="wallet-attestor-api-schema"></a>

<a id="wallet-deploy-and-cutover"></a>

<a id="wallet-key-ceremony"></a>

<a id="webhooks-errors"></a>

<a id="wiring-integration"></a>

<a id="6-open-items"></a>

<a id="explorer-index"></a>

<a id="s27-embedded-wallet-gateway"></a>

<a id="s28-wallet-attestors-tss"></a>
