# Paxeer Bridge

Next.js frontend for the attested PaxeerX bridge. The app connects injected EVM
wallets, submits deposits and withdrawals, requests attestation bundles, and
tracks transfers. The page is [src/app/page.tsx](src/app/page.tsx); bridge UI
components live in [src/components/bridge](src/components/bridge), with client
configuration and transaction helpers in [src/lib/bridge](src/lib/bridge).

Paxeer mainnet-beta is one BFT network running `paxd`, with EVM chain ID **125**
(`0x7d`) and native currency **PAX**. The kernel participates in the network and
proposes blocks. A Paxeer receipt is final when its block is committed. Transfers
across chains additionally require source-chain confirmation depth and threshold
attestor signatures; those bridge requirements do not change Paxeer consensus
finality.

## Getting Started

Use the package's pinned package manager, `pnpm@10.34.9`, and run these commands
from this directory:

```bash
pnpm install --frozen-lockfile
pnpm dev
```

Open [http://localhost:3000](http://localhost:3000). Edit
[src/app/page.tsx](src/app/page.tsx) to change the page; the development server
updates it automatically. The layout uses `next/font` with Geist and Geist Mono.

The UI can be opened without deployment configuration, but missing configuration
blocks transfer actions. Configure the values below in an untracked `.env.local`
before starting development or building a deployment.

## Build-time configuration

[config.ts](src/lib/bridge/config.ts) reads the following public variables.
Next.js embeds `NEXT_PUBLIC_*` values in the browser bundle at build time:
**rebuild after changing them**. They must contain public deployment information,
never credentials or signing keys.

| Variable | Purpose |
| --- | --- |
| `NEXT_PUBLIC_PAXEER_RPC_URL` | Paxeer JSON-RPC endpoint. Required for Paxeer state reads and bridge verification. |
| `NEXT_PUBLIC_PAXEER_EXPLORER_URL` | Optional explorer base URL for Paxeer address and transaction links. |
| `NEXT_PUBLIC_BRIDGE_API_URL` | Base URL for the attestation API used by claim and release actions. |
| `NEXT_PUBLIC_BRIDGE_VAULT_<CHAIN>` | Deployed `PaxeerXVault` address for an EVM route. |
| `NEXT_PUBLIC_BRIDGE_RPC_<CHAIN>` | Optional foreign EVM RPC override; otherwise the viem chain default is used. |
| `NEXT_PUBLIC_BRIDGE_SOLANA_PROGRAM_ID` | Deployed Solana custody program's base58 public key. |
| `NEXT_PUBLIC_BRIDGE_RPC_SOLANA` | Solana HTTP JSON-RPC endpoint, required for the withdrawal recipient-record lookup. |

For the two EVM variable families, replace `<CHAIN>` with exactly one of
`ETHEREUM`, `BASE`, `ARBITRUM`, `OPTIMISM`, `BNB`, `POLYGON`, `AVALANCHE`, or
`HYPEREVM`. For example, the Base vault key is `NEXT_PUBLIC_BRIDGE_VAULT_BASE`.
Use addresses from actual deployment records; the repository's chain
configuration files contain placeholders rather than proof of deployment.
URL values must use HTTP or HTTPS without embedded credentials; EVM vault
addresses must be valid and nonzero. Invalid values are treated as unset.

For public Paxeer JSON-RPC, use a node's root URL for chain ID 125.
See the [public RPC reference](../../docs/site/docs/reference/public-rpc.md).
The app supplies no default Paxeer RPC URL.

## Routes and prerequisites

The [chain registry](src/lib/bridge/chains.ts) lists Ethereum, Base, Arbitrum,
Optimism, BNB Chain, Polygon, Avalanche, HyperEVM, and Solana. An EVM route's
`configured` state means a valid vault address was supplied. Solana becomes
`configured` when its program ID and attestation API URL are set. These labels
describe frontend configuration; on-chain registration and transfer requirements
must also be satisfied.

- Deploy the foreign custody contract or program, and register and enable its
  chain on Paxeer through governance. Configure attestors, their threshold, and
  asset caps. The bridge starts dormant with no chains registered at genesis.
- For an EVM route, the configured vault must match `getChain` on Paxeer. The UI
  reads Paxeer bridge state and foreign vault pause state, caps, outstanding
  balances, and attestors. Read errors, pauses, disabled chains, vault mismatches,
  or exceeded limits prevent the affected action.
- Supply a reachable attestation API. The frontend expects
  `GET /v1/deposit/{chainId}/{txHash}/{logIndex}` and
  `GET /v1/release/{chainId}/{paxeerTxHash}/{nonce}` under its configured base URL.
  See [attestation-api.ts](src/lib/bridge/attestation-api.ts) for response types
  and validation. A 404 means the event is not yet observed; requests time out
  after ten seconds. Claim and release require a `ready` bundle with signatures.
- Connect an injected EVM wallet and switch to the source chain. Supply a valid
  destination, a positive amount within caps and balance, and funds for gas.
  Token deposits use an approval step when the allowance is insufficient.
- Solana deposits remain disabled: this app has no Solana wallet connector.
  Withdrawing to Solana also requires an RPC-verified, registered recipient
  record matching the full destination public key. The UI submits the burn on
  Paxeer, but its release action targets EVM vaults; payout through the Solana
  custody program requires external integration.

The Paxeer bridge is the native `layerxbridge` module exposed through
[`ILayerXBridge`](../../precompiles/layerxbridge/LayerXBridge.sol) at
`0x0000000000000000000000000000000000001016`. Bridged balances are read through
[`IBank`](../../precompiles/bank/Bank.sol) at
`0x0000000000000000000000000000000000001001`. Client addresses and ABIs are in
[contracts.ts](src/lib/bridge/contracts.ts).

For deposits, the foreign
[`PaxeerXVault`](../../contracts/bridge/evm/src/PaxeerXVault.sol) locks assets and
emits `BridgeDeposit`. After confirmations and attestation, **Claim on Paxeer**
submits `bridgeIn`, which mints the bridged denomination. For withdrawals,
`bridgeOut` burns that denomination on Paxeer and emits `BridgeOut`; after
attestation, **Release** submits the payout to the foreign EVM vault. The client
checks bundle identity, digest, and threshold signatures against chain state
before requesting the claim or release wallet signature. This is an attested
custody bridge. See the
[attestation specification](../../modules/layerxbridge/ATTESTATION.md) for the
signed message formats.

## Transfer recovery

The transfer list uses browser `localStorage`, keyed by
`paxeer-bridge:transfers:v1`, with up to 50 records per account. It is a local
history, not a wallet-wide transaction index. Keep source and claim/release
transaction hashes so transfers can be checked after a browser change or lost
storage.

To recover an EVM transfer, connect the wallet that started it and use **Import a
transfer started elsewhere** in the transfers panel. Choose the foreign EVM
source route for a deposit, or **Paxeer → foreign chain (withdrawal)** for a burn,
then enter the source transaction hash. Import requires a successful receipt and
a matching bridge event; it restores tracking without submitting another source
transaction. Deposit import lists only EVM routes with a configured vault.

While a transfer is confirming, check its source receipt and the selected RPC.
While it is attesting, check backend reachability and threshold bundle status.
Once ready, switch to Paxeer to claim a deposit or to the destination EVM chain
to release a withdrawal. Missing or mismatched attestations must be resolved at
the backend or deployment configuration before submission.

A submitted claim or release hash is saved immediately, and the transfer stays
settling while its result is pending. Completion requires the destination
nullifier or verified Solana release record. Reverted or dropped EVM attempts
show a retry action. When neither the settlement receipt nor the transaction is
found, the attempt is classified as dropped once two minutes have elapsed since
submission. Inspect the destination receipt and nullifier before retrying.
Importing an already tracked transfer preserves its existing local record.

## Learn More

- [Next.js documentation](https://nextjs.org/docs) for framework usage.
- [App layout](src/app/layout.tsx) for metadata, fonts, theme initialization, and providers.
- [Transfer actions](src/components/bridge/transfer-actions.ts) for claim/release verification and writes.
- [Transfer storage](src/lib/bridge/transfers.ts) and [receipt import](src/lib/bridge/import.ts) for recovery behavior.

## Deploy

Set deployment variables before the production build, then run from this directory:

```bash
pnpm build
pnpm start
```

Use a Next.js-compatible host, including Vercel if desired. The deployment needs
browser access to its configured RPC and attestation services; those services
must permit requests from the app's origin. A successful frontend build does not
deploy custody contracts, register bridge routes, or provision attestors.
