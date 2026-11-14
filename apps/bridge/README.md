# Paxeer Bridge

This directory holds the Next.js frontend for the attested PaxeerX bridge between
Paxeer mainnet-beta and external chains. It connects injected EVM wallets,
submits deposits and withdrawal burns, requests attestation bundles, verifies
claim and release transactions, and tracks transfers against chain state.

## Layout

- [public/](public/) — static SVG assets.
- [src/](src/) — application routes, bridge and shared UI components, client libraries, and tests.
- [package.json](package.json) — dependencies, package manager pin, and development, build, and test scripts.
- [pnpm-lock.yaml](pnpm-lock.yaml) — pinned dependency graph.
- [pnpm-workspace.yaml](pnpm-workspace.yaml) — dependency build permissions.
- [.pnpmfile.cjs](.pnpmfile.cjs) — applies the manifest's dependency overrides to pnpm configuration.
- [.nvmrc](.nvmrc) — Node.js version requirement.
- [.env.example](.env.example) — public deployment variable names.
- [.gitignore](.gitignore) — dependency, build output, and local configuration exclusions.
- [next.config.ts](next.config.ts) — Next.js caching, prefetching, and Tailwind CSS loader configuration.
- [next-env.d.ts](next-env.d.ts) — Next.js type references.
- [tsconfig.json](tsconfig.json) — TypeScript options and source aliases.
- [vitest.config.ts](vitest.config.ts) — Vitest environment, test selection, and Vite plugins.
- [eslint.config.mjs](eslint.config.mjs) — JavaScript and TypeScript lint rules.
- [dx.json](dx.json) — UI registry, stylesheet, and component aliases.
- [cspell.jsonc](cspell.jsonc) — spelling configuration and dictionary.
- [svgo.config.js](svgo.config.js) — SVG optimization settings.
- [svgo.config.format.js](svgo.config.format.js) — SVG formatting settings with ID prefixes.
- [.license-reportrc](.license-reportrc) — license report configuration.

The page is [src/app/page.tsx](src/app/page.tsx); bridge UI components live in
[src/components/bridge](src/components/bridge), with client configuration and
transaction helpers in [src/lib/bridge](src/lib/bridge). The
[app layout](src/app/layout.tsx) supplies metadata, Geist and Geist Mono fonts
through `next/font`, theme initialization, and providers.

## Build and test

Use the package's pinned package manager, `pnpm@10.34.9`, and install dependencies
from this directory:

```bash
pnpm install --frozen-lockfile
```

The commands below run the scripts declared in [package.json](package.json),
from this directory:

| Command | Purpose |
| --- | --- |
| `pnpm dev` | Start the Next.js development server with automatic page updates. |
| `pnpm build` | Create the production Next.js build. |
| `pnpm start` | Serve the production build. |
| `pnpm test` | Run the Vitest suite. |
| `pnpm test:manifest` | Run the test manifest checks. |
| `pnpm typecheck` | Generate Next.js types and check TypeScript without emitting output. |
| `pnpm lint` | Run ESLint. |

Set deployment variables before the production build, then run `pnpm build`
and `pnpm start`. Use a Next.js-compatible host, including Vercel if desired.
The deployment needs browser access to its configured RPC and attestation
services; those services must permit requests from the app's origin. Custody
contracts, route registration, and attestors are provisioned separately.

## Configuration

The UI can be opened without deployment configuration, but missing configuration
blocks the affected transfer actions. Set public deployment inputs in an
untracked `.env.local` before development or a production build;
[.env.example](.env.example) lists the supported keys.

[config.ts](src/lib/bridge/config.ts) reads the following public variables.
Next.js embeds `NEXT_PUBLIC_*` values in the browser bundle at build time;
rebuild after changing them. They contain public deployment information.

| Variable | Purpose |
| --- | --- |
| `NEXT_PUBLIC_PAXEER_RPC_URL` | Paxeer JSON-RPC endpoint for state reads and bridge verification. |
| `NEXT_PUBLIC_PAXEER_EXPLORER_URL` | Optional explorer base URL for Paxeer address and transaction links. |
| `NEXT_PUBLIC_BRIDGE_API_URL` | Attestation API base URL for claim and release bundles. |
| `NEXT_PUBLIC_BRIDGE_SOLANA_PROGRAM_ID` | Deployed Solana custody program public key. |
| `NEXT_PUBLIC_BRIDGE_RPC_SOLANA` | Solana JSON-RPC endpoint for custody, recipient, and release records. |

The EVM vault variables name deployed `PaxeerXVault` contracts:

- `NEXT_PUBLIC_BRIDGE_VAULT_ETHEREUM`
- `NEXT_PUBLIC_BRIDGE_VAULT_BASE`
- `NEXT_PUBLIC_BRIDGE_VAULT_ARBITRUM`
- `NEXT_PUBLIC_BRIDGE_VAULT_OPTIMISM`
- `NEXT_PUBLIC_BRIDGE_VAULT_BNB`
- `NEXT_PUBLIC_BRIDGE_VAULT_POLYGON`
- `NEXT_PUBLIC_BRIDGE_VAULT_AVALANCHE`
- `NEXT_PUBLIC_BRIDGE_VAULT_HYPEREVM`

The optional foreign EVM RPC overrides are:

- `NEXT_PUBLIC_BRIDGE_RPC_ETHEREUM`
- `NEXT_PUBLIC_BRIDGE_RPC_BASE`
- `NEXT_PUBLIC_BRIDGE_RPC_ARBITRUM`
- `NEXT_PUBLIC_BRIDGE_RPC_OPTIMISM`
- `NEXT_PUBLIC_BRIDGE_RPC_BNB`
- `NEXT_PUBLIC_BRIDGE_RPC_POLYGON`
- `NEXT_PUBLIC_BRIDGE_RPC_AVALANCHE`
- `NEXT_PUBLIC_BRIDGE_RPC_HYPEREVM`

Unset EVM RPC overrides use the viem chain defaults. Use vault addresses and the
Solana program identity from deployment records. URL inputs accept HTTP or HTTPS
without embedded credentials; EVM vault inputs accept valid nonzero addresses.
Invalid inputs are treated as unset. The app requires an explicitly configured
Paxeer RPC endpoint using a node's root URL for chain ID 125; see the
[public RPC reference](../../docs/readme/PUBLIC-RPC.md).

Build and test tooling reads [next.config.ts](next.config.ts),
[tsconfig.json](tsconfig.json), [vitest.config.ts](vitest.config.ts), and
[eslint.config.mjs](eslint.config.mjs). Package installation uses
[pnpm-workspace.yaml](pnpm-workspace.yaml) and [.pnpmfile.cjs](.pnpmfile.cjs).
UI registry settings are in [dx.json](dx.json); spelling, SVG, and license report
settings are listed under Layout.

### Network and routes

Paxeer mainnet-beta runs `paxd` with EVM chain ID **125** (`0x7d`) and native
currency **PAX**. The LayerX kernel inside `paxd` executes and orders each
transaction and builds each block; validators check and commit it. A Paxeer receipt is final when its block is committed. Transfers
across chains additionally require source-chain confirmation depth and threshold
attestor signatures; those bridge requirements do not change Paxeer consensus
finality.

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
  Paxeer. The transfer panel observes the automatic relay payout through the
  Solana custody program and verifies its finalized release record.

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

### Transfer recovery

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
Importing an already tracked transfer reconciles receipt metadata and retains
its saved settlement progress.

## Documentation

- [Bridge module](../../docs/wiki/Bridge.md) — network bridge operations and governance prerequisites.
- [Bridge application variables](../../docs/release/variables/app-bridge.md) — public frontend configuration reference.
- [Public RPC reference](../../docs/readme/PUBLIC-RPC.md) — Paxeer JSON-RPC access.
- [Bridging and balances](../../docs/site/paxeer-docs/public/markdown/bridge.md) — documentation site bridge guide.
- [Documentation site bridge page](../../docs/site/paxeer-docs/src/app/bridge/page.tsx) — site page source.
- [Attestation specification](../../modules/layerxbridge/ATTESTATION.md) — signed message formats.
- [Transfer actions](src/components/bridge/transfer-actions.ts) — claim and release verification and writes.
- [Transfer storage](src/lib/bridge/transfers.ts) and [receipt import](src/lib/bridge/import.ts) — transfer recovery behavior.
