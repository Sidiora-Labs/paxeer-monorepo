#!/usr/bin/env bash
# paxeer-x-services: agent-mcp agentd bridge-relayer embedded-wallet-gateway explorer gas-station human-kms human-service interop-gateway interop-service launchpad-gateway launchpad-indexer mirror-publisher mirror-signer perps-data platform-agent-boundary platform-ai-artifacts platform-ai-evaluator platform-core-boundary platform-dashboard platform-faucet platform-gateway platform-identity platform-indexer platform-paxai-worker platform-price-feeder platform-program-registry platform-receipt-authority platform-testnet-control rollout-controller sequencer wallet-attestors x-websearch x402-seller-middleware
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/../../.."
exec timeout 15m python3 tools/paxeer-x/tests/routes_test.py
