# Paxeer X Network bridge deployment tooling

This directory holds deployment and configuration tools for the external-chain bridge: EVM vault and Solana custody deployment, governance proposal generation, deployment readback, and bridge app environment generation. The tools connect foreign custody configuration to the `layerxbridge` module and `LayerXBridge` precompile on Paxeer, whose kernel executes within the same BFT chain.

## Layout

- [deploy/](deploy/) — deployment scripts, Go packages, configuration examples, and their tests.
- [README.md](README.md) — overview and commands for this directory.
- [deploy/chainconfig/](deploy/chainconfig/) — chain configuration schema, validation, and deployment overlays.
- [deploy/proposals/](deploy/proposals/) — governance message and proposal generation; its `cmd/paxeer-bridge-proposals/` directory holds the command entry point.
- [deploy/appenv/](deploy/appenv/) — deployment record validation and bridge app environment generation, with Node dependencies for interface tests.
- [deploy/cmd/](deploy/cmd/) — the `paxeer-bridge-app-env` command entry point.
- [deploy/deploy-evm-chain.sh](deploy/deploy-evm-chain.sh) — EVM vault deployment and receipt reconciliation.
- [deploy/deploy-solana-program.sh](deploy/deploy-solana-program.sh) — Solana program deployment, initialization, and asset registration.
- [deploy/checklist.sh](deploy/checklist.sh) — compares foreign custody and Paxeer bridge state with the deployment configuration.
- [deploy/verify-evm-chain.sh](deploy/verify-evm-chain.sh) — submits deployed vault source verification to the chain's explorer.
- [deploy/overlay.sh](deploy/overlay.sh) and [deploy/overlay.jq](deploy/overlay.jq) — configuration overlay application and deployment across chains.
- [deploy/evm-run-journal.py](deploy/evm-run-journal.py) and [deploy/solana-run-journal.py](deploy/solana-run-journal.py) — persistent deployment checkpoints and reconciliation.
- [deploy/attestors.json](deploy/attestors.json) and [deploy/check-attestor-manifest.py](deploy/check-attestor-manifest.py) — attestor manifest and validation.
- [deploy/relayer-solana.example.json](deploy/relayer-solana.example.json) — example relayer configuration with Solana custody settings.
- [deploy/tests/](deploy/tests/) — shell checks, deployment exercises, and fixtures.

## Build and test

The Go packages use the repository's [go.mod](../../go.mod). They compile as part of the test command below. Run these commands from the repository root, as declared in the [bridge workflow](../../.github/workflows/bridge-test.yml):

```sh
npm ci --no-audit --no-fund --prefix services/bridge/deploy/appenv
npm ci --no-audit --no-fund --prefix services/bridge/deploy/proposals/testdata/repair_270
go test -count=1 ./contracts/bridge/vectors/... ./services/bridge/deploy/...
bash services/bridge/deploy/tests/workflow-check.sh
bash services/bridge/deploy/tests/deploy-scripts-check.sh
bash services/bridge/deploy/tests/checklist-check.sh
bash services/bridge/deploy/tests/docs-check.sh
```

The workflow supplies Go and Node.js. The shell checks also use Python, Bash, and `jq`; the deployment script checks use `curl`. The Node installs provide the app configuration and Solidity compiler dependencies used by the Go tests.

## Configuration

Chain inputs are the `config.json` files below [contracts/bridge/evm/chains/](../../contracts/bridge/evm/chains/) and [contracts/bridge/solana/chains/](../../contracts/bridge/solana/chains/). They declare chain identity, assets, caps, ownership, attestors, finality settings, and environment variable names. `BRIDGE_DEPLOY_OVERLAY` selects a JSON overlay for deployment settings. The tools validate the resulting configuration before deployment.

The scripts read these environment variables:

- `PAXEER_BRIDGE_EVM_CHAINS_ROOT` and `PAXEER_BRIDGE_SOLANA_CHAINS_ROOT` — alternative chain configuration directories.
- `PAXEER_BRIDGE_ATTESTOR_MANIFEST` — attestor manifest selection; the default file is [deploy/attestors.json](deploy/attestors.json).
- `PAXEER_BRIDGE_DEPLOYMENT_RECORD` and `PAXEER_BRIDGE_DEPLOYMENT_RECORD_DIR` — deployment record file and directory selection.
- `PAXEER_BRIDGE_PAXEER_RPC_URL` and `PAXEER_BRIDGE_GOVERNANCE_AUTHORITY` — Paxeer readback and governance proposal inputs for the checklist.
- `PAXEER_BRIDGE_APP_ENV` — optional app environment file checked against deployment records.
- `PAXEER_BRIDGE_SOLANA_ADMIN_CLI`, `PAXEER_BRIDGE_SOLANA_PROGRAM_KEYPAIR_FILE`, and `PAXEER_BRIDGE_SOLANA_EXECUTABLE_WAIT_SECONDS` — Solana administration, program identity, and executable-account wait settings.

The committed chain configurations name the following endpoint and deployment variables. An overlay can replace the names through its `environment` fields.

| Chain | Environment variables |
| --- | --- |
| Ethereum | `PAXEER_BRIDGE_ETHEREUM_RPC_URL`, `PAXEER_BRIDGE_ETHEREUM_DEPLOY_KEY`, `PAXEER_BRIDGE_ETHEREUM_EXPLORER_KEY` |
| Base | `PAXEER_BRIDGE_BASE_RPC_URL`, `PAXEER_BRIDGE_BASE_DEPLOY_KEY`, `PAXEER_BRIDGE_BASE_EXPLORER_KEY` |
| Arbitrum | `PAXEER_BRIDGE_ARBITRUM_RPC_URL`, `PAXEER_BRIDGE_ARBITRUM_DEPLOY_KEY`, `PAXEER_BRIDGE_ARBITRUM_EXPLORER_KEY` |
| Optimism | `PAXEER_BRIDGE_OPTIMISM_RPC_URL`, `PAXEER_BRIDGE_OPTIMISM_DEPLOY_KEY`, `PAXEER_BRIDGE_OPTIMISM_EXPLORER_KEY` |
| BNB | `PAXEER_BRIDGE_BNB_RPC_URL`, `PAXEER_BRIDGE_BNB_DEPLOY_KEY`, `PAXEER_BRIDGE_BNB_EXPLORER_KEY` |
| Polygon | `PAXEER_BRIDGE_POLYGON_RPC_URL`, `PAXEER_BRIDGE_POLYGON_DEPLOY_KEY`, `PAXEER_BRIDGE_POLYGON_EXPLORER_KEY` |
| Avalanche | `PAXEER_BRIDGE_AVALANCHE_RPC_URL`, `PAXEER_BRIDGE_AVALANCHE_DEPLOY_KEY`, `PAXEER_BRIDGE_AVALANCHE_EXPLORER_KEY` |
| HyperEVM | `PAXEER_BRIDGE_HYPEREVM_RPC_URL`, `PAXEER_BRIDGE_HYPEREVM_DEPLOY_KEY`, `PAXEER_BRIDGE_HYPEREVM_EXPLORER_KEY` |
| Solana | `PAXEER_BRIDGE_SOLANA_RPC_URL`, `PAXEER_BRIDGE_SOLANA_KEYPAIR_FILE`, `PAXEER_BRIDGE_SOLANA_TOOLCHAIN_BIN` |

The proposal command reads its attestor manifest through `-manifest`. The app environment command reads deployment JSON records and, with `--check`, an environment file. It emits `NEXT_PUBLIC_BRIDGE_VAULT_ETHEREUM`, `NEXT_PUBLIC_BRIDGE_VAULT_BASE`, `NEXT_PUBLIC_BRIDGE_VAULT_ARBITRUM`, `NEXT_PUBLIC_BRIDGE_VAULT_OPTIMISM`, `NEXT_PUBLIC_BRIDGE_VAULT_BNB`, `NEXT_PUBLIC_BRIDGE_VAULT_POLYGON`, `NEXT_PUBLIC_BRIDGE_VAULT_AVALANCHE`, `NEXT_PUBLIC_BRIDGE_VAULT_HYPEREVM`, and `NEXT_PUBLIC_BRIDGE_SOLANA_PROGRAM_ID` for the bridge app.

## Documentation

- [Bridge module](../../docs/wiki/Bridge.md)
- [Documentation site bridge page](../../docs/site/paxeer-docs/src/app/bridge/page.tsx)
- [Documentation site bridge Markdown](../../docs/site/paxeer-docs/public/markdown/bridge.md)
- [Bridge deployment runbook](../../contracts/bridge/README.md)
