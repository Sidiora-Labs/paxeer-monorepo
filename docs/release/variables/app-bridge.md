# Bridge application variables

The Paxeer Bridge frontend is the application in `apps/bridge`. It uses these build-time public variables to connect to Paxeer mainnet-beta, foreign chains, and the attestation service; unset values leave the affected route or action unavailable.

## Required

| Variable | What it sets | Default | Secret | Source |
| --- | --- | --- | --- | --- |
| `NEXT_PUBLIC_BRIDGE_API_URL` | Base URL for the bridge attestation API used by claim and release actions. | None; attestation actions stay unavailable when unset. | No | `apps/bridge/src/lib/bridge/config.ts` |
| `NEXT_PUBLIC_PAXEER_RPC_URL` | Paxeer JSON-RPC endpoint used for chain state and bridge verification. | None; Paxeer reads stay unavailable when unset. | No | `apps/bridge/src/lib/bridge/config.ts` |

## Optional

| Variable | What it sets | Default | Secret | Source |
| --- | --- | --- | --- | --- |
| `NEXT_PUBLIC_BRIDGE_RPC_ARBITRUM` | RPC override for the Arbitrum route. | Viem Arbitrum chain default. | No | `apps/bridge/src/lib/bridge/config.ts` |
| `NEXT_PUBLIC_BRIDGE_RPC_AVALANCHE` | RPC override for the Avalanche route. | Viem Avalanche chain default. | No | `apps/bridge/src/lib/bridge/config.ts` |
| `NEXT_PUBLIC_BRIDGE_RPC_BASE` | RPC override for the Base route. | Viem Base chain default. | No | `apps/bridge/src/lib/bridge/config.ts` |
| `NEXT_PUBLIC_BRIDGE_RPC_BNB` | RPC override for the BNB Chain route. | Viem BNB Chain default. | No | `apps/bridge/src/lib/bridge/config.ts` |
| `NEXT_PUBLIC_BRIDGE_RPC_ETHEREUM` | RPC override for the Ethereum route. | Viem Ethereum chain default. | No | `apps/bridge/src/lib/bridge/config.ts` |
| `NEXT_PUBLIC_BRIDGE_RPC_HYPEREVM` | RPC override for the HyperEVM route. | Viem HyperEVM chain default. | No | `apps/bridge/src/lib/bridge/config.ts` |
| `NEXT_PUBLIC_BRIDGE_RPC_OPTIMISM` | RPC override for the Optimism route. | Viem Optimism chain default. | No | `apps/bridge/src/lib/bridge/config.ts` |
| `NEXT_PUBLIC_BRIDGE_RPC_POLYGON` | RPC override for the Polygon route. | Viem Polygon chain default. | No | `apps/bridge/src/lib/bridge/config.ts` |
| `NEXT_PUBLIC_BRIDGE_RPC_SOLANA` | Solana JSON-RPC endpoint used for recipient-record lookups. | None; Solana recipient lookups stay unavailable when unset. | No | `apps/bridge/src/lib/bridge/config.ts` |
| `NEXT_PUBLIC_BRIDGE_SOLANA_PROGRAM_ID` | Solana custody program public key. | None; the Solana route remains awaiting backend configuration when unset. | No | `apps/bridge/src/lib/bridge/config.ts` |
| `NEXT_PUBLIC_BRIDGE_VAULT_ARBITRUM` | Deployed PaxeerXVault address for Arbitrum. | None; the route is marked not deployed when unset. | No | `apps/bridge/src/lib/bridge/config.ts` |
| `NEXT_PUBLIC_BRIDGE_VAULT_AVALANCHE` | Deployed PaxeerXVault address for Avalanche. | None; the route is marked not deployed when unset. | No | `apps/bridge/src/lib/bridge/config.ts` |
| `NEXT_PUBLIC_BRIDGE_VAULT_BASE` | Deployed PaxeerXVault address for Base. | None; the route is marked not deployed when unset. | No | `apps/bridge/src/lib/bridge/config.ts` |
| `NEXT_PUBLIC_BRIDGE_VAULT_BNB` | Deployed PaxeerXVault address for BNB Chain. | None; the route is marked not deployed when unset. | No | `apps/bridge/src/lib/bridge/config.ts` |
| `NEXT_PUBLIC_BRIDGE_VAULT_ETHEREUM` | Deployed PaxeerXVault address for Ethereum. | None; the route is marked not deployed when unset. | No | `apps/bridge/src/lib/bridge/config.ts` |
| `NEXT_PUBLIC_BRIDGE_VAULT_HYPEREVM` | Deployed PaxeerXVault address for HyperEVM. | None; the route is marked not deployed when unset. | No | `apps/bridge/src/lib/bridge/config.ts` |
| `NEXT_PUBLIC_BRIDGE_VAULT_OPTIMISM` | Deployed PaxeerXVault address for Optimism. | None; the route is marked not deployed when unset. | No | `apps/bridge/src/lib/bridge/config.ts` |
| `NEXT_PUBLIC_BRIDGE_VAULT_POLYGON` | Deployed PaxeerXVault address for Polygon. | None; the route is marked not deployed when unset. | No | `apps/bridge/src/lib/bridge/config.ts` |
| `NEXT_PUBLIC_PAXEER_EXPLORER_URL` | Paxeer explorer base URL for address and transaction links. | None; explorer links are omitted when unset. | No | `apps/bridge/src/lib/bridge/config.ts` |
