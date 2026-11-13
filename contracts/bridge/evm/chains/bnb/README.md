# BNB Smart Chain

PAX against BNB. This page is the BNB Smart Chain leg of the Paxeer X Network bridge: a BNB deposit locked in the vault on BNB Smart Chain mints its bridged denom on Paxeer X Network, and a burn of that denom on Paxeer X Network releases the BNB locked here. The operator runbook is [bridge runbook](../../../README.md); this page lists what this chain needs.

| Field | Value |
| --- | --- |
| Chain name | `bnb` |
| Chain id | `56` |
| Native coin | `BNB`, 18 decimals |
| Finality depth | `30` blocks |
| Attestors | 1, signing at a threshold of 1 in the tracked configuration |
| Vault | `PaxeerXVault`, the same bytecode every EVM chain of the bridge deploys |
| Configuration | `contracts/bridge/evm/chains/bnb/config.json` |

## Assets

The configuration registers one asset, the native coin. It is the first asset of the file and is always registered, and its caps and Paxeer-side asset registration must be applied before deposits are used.

| Asset | Address and asset id | Decimals | Per-transaction cap | Total cap |
| --- | --- | --- | --- | --- |
| BNB | `0x0000000000000000000000000000000000000000` | 18 | `1000000000000000000000` (1,000 BNB) | `20000000000000000000000` (20,000 BNB) |

BNB is deposited through `depositNative` and paid out by the vault's native release path. The caps are in base units and apply to the vault on this chain; the same values go into the `MsgSetCap` body for this asset on the Paxeer side.

## Environment variables

The configuration names the variables; neither the configuration nor the scripts carry a value for any of them.

| Variable | Read by | Holds |
| --- | --- | --- |
| `PAXEER_BRIDGE_BNB_RPC_URL` | `services/bridge/deploy/deploy-evm-chain.sh` | the BNB Smart Chain endpoint |
| `PAXEER_BRIDGE_BNB_DEPLOY_KEY` | `services/bridge/deploy/deploy-evm-chain.sh` | the deployer key |
| `PAXEER_BRIDGE_BNB_EXPLORER_KEY` | `services/bridge/deploy/verify-evm-chain.sh` | the explorer verification key |
| `PAXEER_BRIDGE_DEPLOYMENT_RECORD` | both scripts | the deployment record: written by the deploy, read by the verification |
| `PAXEER_BRIDGE_EVM_CHAINS_ROOT` | both scripts, optional | a chains root holding `bnb/config.json`, in place of `contracts/bridge/evm/chains` |
| `BRIDGE_DEPLOY_OVERLAY` | both scripts, optional | JSON overlay for owner, deployer, attestors, environment names, caps and chain-specific settings |
| `PAXEER_BRIDGE_ATTESTOR_MANIFEST` | deployment and proposal generation, optional | approved signer manifest; defaults to `services/bridge/deploy/attestors.json` |

## Before running the deploy script

- `owner` in `contracts/bridge/evm/chains/bnb/config.json` requires a configured address. Supply it in a private configuration or `BRIDGE_DEPLOY_OVERLAY`. Attestors must be nonzero and strictly ascending; proposal generation requires their set and threshold to match the selected attestor manifest.
- The endpoint must answer `eth_chainId` with `56`; the script stops on any other chain id.
- `python3`, `jq`, `forge` and `cast` are on the `PATH`; deployment also uses `git` to bootstrap its pinned libraries and `flock` to serialize writes.
- The deployer account holds enough BNB to pay for the vault creation, the `setCap` call and the `transferOwnership` call the deployment makes.

```sh
bash services/bridge/deploy/deploy-evm-chain.sh --preflight bnb
bash services/bridge/deploy/deploy-evm-chain.sh bnb
bash services/bridge/deploy/verify-evm-chain.sh --preflight bnb
bash services/bridge/deploy/verify-evm-chain.sh bnb
```

The deployment proposes ownership to the configured owner, who calls `acceptOwnership()` to complete the transfer. The deployment record reports `"ownership": "proposed"` when readback still names the deployer, or `"ownership": "held"` when it names the configured owner. The record is a snapshot of that readback.
