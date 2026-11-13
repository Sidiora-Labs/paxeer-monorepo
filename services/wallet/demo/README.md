# `@paxeer/demo`

Next.js reference app for [`@paxeer/wallet`](../../../sdk/wallet/README.md), the workspace wallet SDK. The demo offers an embedded threshold-custody wallet authenticated with Supabase, or an injected EIP-6963 wallet. `ConnectButton` opens `WalletModal`, which renders `SupportedWalletPanel` from `FundedAccountPanel.tsx` for wallet actions.

The demo targets Paxeer mainnet-beta, one BFT network with EVM chain ID `125` (`0x7d`). The kernel is part of the network and proposes blocks. A transaction receipt is final when its block is committed; receiving a transaction hash from the send button alone does not establish commitment.

## Run

Use pnpm and Node matching the wallet workspace's `engines` requirement (`>=22.0.0`). From `services/wallet`:

```sh
pnpm install
pnpm --filter @paxeer/wallet build
pnpm --filter @paxeer/demo dev      # http://localhost:3000
```

The SDK build also installs and builds its agent SDK dependency. Configure the demo before opening it. Set these variables in the environment or in an ignored `demo/.env.local`:

| Variable | Purpose |
| --- | --- |
| `NEXT_PUBLIC_PAXEER_WALLET_API` | Required API origin used by the wallet SDK and panel; HTTPS, or loopback HTTP (`localhost`, `127.0.0.1`, `[::1]`), without a path, credentials, query or fragment |
| `NEXT_PUBLIC_SUPABASE_URL` | Required Supabase project URL |
| `NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY` | Required browser publishable key for that project |
| `NEXT_PUBLIC_AUTH_REDIRECT_URL` | Optional OAuth and magic-link return URL; defaults to `<browser origin>/auth/callback` |
| `NEXT_PUBLIC_SITE_URL` | Optional metadata base URL |

Enable the desired Supabase sign-in providers (Google, X/Twitter, GitHub, email magic link) and allow the running demo's `/auth/callback` URL, or the explicit redirect override, as a redirect target. These `NEXT_PUBLIC_` values are browser configuration; use the Supabase publishable key here.

The configured origin must serve the authenticated `/v1/wallet/*` routes. The panel also constructs its node JSON-RPC URL as `<origin>/`; deployment must provide that root endpoint for forwarded RPC calls. The wallet gateway's `buildApp()` in `../gateway/src/index.ts` registers HTTP wallet routes, but does not register a root JSON-RPC handler. Starting that gateway alone does not supply every endpoint the panel assumes. See the [wallet workspace guide](../README.md) for gateway configuration.

Public node JSON-RPC uses the root path of the configured node origin. See [public RPC endpoints](../../../docs/site/docs/reference/public-rpc.md). These node endpoints do not supply the authenticated wallet HTTP API, so they are not substitutes for `NEXT_PUBLIC_PAXEER_WALLET_API`.

Other package commands, from `services/wallet`:

```sh
pnpm --filter @paxeer/demo build
pnpm --filter @paxeer/demo start      # serves the production build on port 3000
pnpm --filter @paxeer/demo typecheck
pnpm --filter @paxeer/demo lint
```

The `lint` script runs `eslint .` directly with the demo's `eslint.config.mjs` and
declared `eslint` and `eslint-config-next` development dependencies. The flat
configuration applies Next.js Core Web Vitals rules to the demo, including its
TypeScript and TSX sources. Generated Next.js output, build output, coverage and
`next-env.d.ts` are excluded; application source and tests remain in scope.

## WalletModal and SupportedWalletPanel flow

`ConnectButton` opens `WalletModal`. The modal handles wallet selection,
authentication and explicit provisioning before rendering `SupportedWalletPanel`
for the chosen embedded or injected wallet. The supported send path starts in
that panel; `SendTxForm` is not mounted by the modal.

1. Open **Connect wallet** and choose an embedded wallet or an announced injected wallet.
2. For embedded custody, sign in through Supabase. If no standard wallet exists, choose **Create embedded threshold-custody wallet**. The modal calls `provisionStandardWallet()` and refreshes `useStandardAccount()`; sign-in alone does not create the wallet. Injected custody uses the selected wallet's provider.
3. In the panel, choose **Connect embedded wallet** or **Connect injected wallet** to request accounts. The panel requires chain ID `125`. Account or chain changes clear its connected state and require connecting again.
4. Enter a recipient (`0x` plus 40 hex characters), a non-negative integer amount in **wei**, and byte calldata (`0x` for a plain transfer). Choose **Review and send transaction**. `WalletInterface.sendTransaction()` requests `eth_sendTransaction`; the embedded `PaxeerProvider` uses the authenticated `/v1/wallet/send` route, while the injected provider handles its own signing and submission. The sending account needs funds for the transaction and gas.
5. Review the approval prompt: embedded requests use the panel's `window.confirm` callback; injected requests use the selected wallet's approval UI. The result area shows the returned transaction hash or an error. The panel does not wait for a committed receipt.

The panel also accepts typed-data JSON and calls `WalletInterface.signTypedData()` through `eth_signTypedData_v4` after checking the connected account and chain.

**Read availability and plan intent** checks kernel availability before calling
`HumanClient.planIntent()` (`POST /v1/intents/plan`) and displays the plan, an
unavailable state or an error. It does not submit or execute a plan.
`EndpointClient.getNetwork()` reads `px_getNetwork` from the configured node;
registration and funding use a separately configured `hostedUrl`. Planning is
unavailable when the consensus head is null or still catching up.

## Source

| File | Role |
| --- | --- |
| [`src/lib/paxeer.ts`](src/lib/paxeer.ts) | `paxeerWallet()` SDK singleton from the required environment, and `getAuthRedirectUrl()` |
| [`src/lib/format.ts`](src/lib/format.ts) | Address and amount helpers; the active panel accepts integer wei directly rather than using `nativeToWei()` |
| [`src/components/ConnectButton.tsx`](src/components/ConnectButton.tsx) | Validates configuration, initializes the SDK and opens the wallet modal |
| [`src/components/WalletModal.tsx`](src/components/WalletModal.tsx) | Radix Dialog wallet chooser, Supabase sign-in, explicit embedded provisioning and `SupportedWalletPanel` wiring |
| [`src/components/ProviderButton.tsx`](src/components/ProviderButton.tsx) | One sign-in provider button |
| [`src/components/FundedAccountPanel.tsx`](src/components/FundedAccountPanel.tsx) | `unifiedOrigin()` validation and `SupportedWalletPanel`: provider connection, chain checks, transaction and typed-data approval, and intent planning attempt |
| [`src/components/SendTxForm.tsx`](src/components/SendTxForm.tsx) | Unused alternative form; no demo source imports it, and the modal uses `SupportedWalletPanel` |
| [`src/app/auth/callback/page.tsx`](src/app/auth/callback/page.tsx) | Reads or waits for the Supabase session, then returns to `/`; shows an error if sign-in does not complete |
| [`public/icons/`](public/icons/) | Sign-in provider icons |

Use the modal and panel above as the integration reference. The landing page's “Three components” copy still names the unused `SendTxForm`; the rendered flow is the one documented here.
