# `@paxeer/wallet`

TypeScript SDK for Paxeer wallet applications: Supabase email and OAuth sign-in, an embedded wallet, an EIP-1193 provider, EIP-6963 discovery, EVM transaction review, authenticated receipts, and clients for hosted account reads and the human service.

Paxeer mainnet-beta is one BFT network with EVM chain ID `125` (`0x7d`). Its kernel participates in block proposal and execution. A transaction is final once its block is committed. Sub-chains are tenant domains in the database, not separate networks that a wallet must switch to.

## Install and build

The package is a member of the [`services/wallet` pnpm workspace](../../services/wallet/pnpm-workspace.yaml). Workspace consumers can depend on `"@paxeer/wallet": "workspace:*"`; [`apps/wallet`](../../apps/wallet/package.json) uses `link:../../sdk/wallet`. The entry points are `@paxeer/wallet`, `@paxeer/wallet/react` and `@paxeer/wallet/provider`. React is an optional peer dependency (version 18 or later).

From the repository root:

```sh
cd services/wallet
pnpm install
pnpm --filter @paxeer/wallet build       # builds services/agent/sdk/typescript first
pnpm --filter @paxeer/wallet test        # vitest; also builds the agent SDK first
pnpm --filter @paxeer/wallet typecheck
```

## Usage (vanilla)

The following example runs in a browser. `apiUrl` is the base URL of the deployed [wallet gateway](../../services/wallet/gateway/src/routes/), which serves authenticated `/v1/wallet/*` routes. It is separate from the node JSON-RPC URL. The environment variable names below are application configuration; the SDK receives the three explicit configuration fields.

```ts
import { PaxeerWallet } from '@paxeer/wallet';

const paxeer = new PaxeerWallet({
  apiUrl: process.env.NEXT_PUBLIC_PAXEER_WALLET_API!,
  supabaseUrl: process.env.NEXT_PUBLIC_SUPABASE_URL!,
  supabaseAnonKey: process.env.NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY!,
  evmReview: { storage: window.localStorage },
});

// Start sign-in; resume wallet operations after the magic link establishes a session.
const { error } = await paxeer.signInWithEmail('alice@example.com');
if (error) throw error;
```

After sign-in, the SDK attaches the Supabase session's bearer token to wallet requests:

```ts
if (!(await paxeer.getSession())) throw new Error('Sign in before using the wallet');

// Auto-provisions if the signed-in user has no wallet yet.
const { wallet, chain } = await paxeer.getWallet();
if (chain.id !== 125) throw new Error('Expected Paxeer chain 125');
console.log(wallet.address);

const sent = await paxeer.sendTransaction({
  to: '0x1111111111111111111111111111111111111111', // replace with your recipient
  value: 1_000_000_000_000_000n, // 0.001 PAX, in wei
  chainId: 125,
});
console.log(sent.tx_hash, sent.receipt?.rung);
```

Use `getStandardSelf()` for a passive read: it returns `null` when the authenticated user has no wallet. Call `provisionStandardWallet()` after an explicit creation choice. `getWallet()` combines these operations; provision before calling the signing or sending methods. Signing happens on the API server; this client does not receive custody key material. Requests remain subject to authentication, signing policy and gateway refusals.

## EVM review and approval

Configure `evmReview.storage` with persistent `getItem` and `setItem` methods before using this flow. Browser `localStorage` satisfies that interface; other environments must supply their own storage. Reviews are retained under the authenticated user's ID and reused when the client is recreated with the same storage.

The SDK accepts an unsigned EIP-1559 transaction with a destination and no signature fields. Supply the pending nonce, gas and fee limits for the intended transaction. It serializes the transaction and checks the returned destination, value, calldata, signing hash, chain and principal against the request before retaining the review.

Using the `paxeer` client above, this function displays the retained artifact and approves only after user confirmation:

```ts
import type { TransactionSerializable } from 'viem';

async function reviewAndApprove(unsignedTransaction: TransactionSerializable) {
  if (unsignedTransaction.chainId !== 125) throw new Error('Expected Paxeer chain 125');
  const review = await paxeer.reviewEvmTransaction(unsignedTransaction);

  // Display destination, value, calldata, signing hash and custody/session binding.
  if (!window.confirm(JSON.stringify(review, null, 2))) return null;

  const approval = await paxeer.approveEvmTransaction(review.id);
  console.log(approval.id, approval.state);
  return paxeer.evmApprovalStatus(review.id);
}
```

`reviewEvmTransaction`, `approveEvmTransaction` and `evmApprovalStatus` call `POST /v1/wallet/evm/review`, `POST /v1/wallet/evm/approve` and `GET /v1/wallet/evm/approvals/:id`, respectively. Approval requires the original locally retained review. Artifacts bind the principal, key, session, signing hash and expiry; the gateway rejects expired approvals.

Approval records consent and does not broadcast. The gateway has an [EVM signing route](../../services/wallet/gateway/src/routes/evmApprovals.ts) at `POST /v1/wallet/evm/sign`, but this SDK currently exposes no helper for that route. `signTransaction()` and `sendTransaction()` use their own routes and do not consume an EVM approval ID. Status can be `reviewed`, `approved`, `signing_unknown` or `signed`; an uncertain signing outcome must be reconciled through status rather than treated as a fresh approval.

## Authenticated receipts

`sendTransaction()` returns `{ tx_hash, address, chain_id, receipt? }`. It validates the send response, authenticates any returned receipt, and otherwise queries `pax_sequencerReceipt` through the wallet's `chain.rpc_url`. A missing receipt is still pending evidence, not transaction failure. A receipt lookup or authentication error can occur after broadcast; retain the original transaction hash and reconcile before submitting again.

The wire API and exported types retain the names `pax_sequencerReceipt` and `SequencerReceiptAnswer`. These describe receipts from the merged network's kernel. The receipt stages are:

- `submitted`: transaction hash and nonce only; no signed execution evidence yet.
- `receipted`: signed execution evidence, including status, logs and fees.
- `committed`: execution evidence plus block number and block hash.

The [receipt helpers](src/receipt.ts) check transaction identity, signed receipt bytes, decoded execution fields, the Ed25519 signature, and the signing key's governed height range. They obtain key pins from the node's `px_getReceiptKeys` method. Authentication failure throws; it must not be displayed as successful execution. Execution status `0x0` means failure even when the transaction is committed.

Continue the vanilla send example to wait for commitment with bounded polling:

```ts
import { JsonRpcClient, ReceiptTimeoutError, waitForSequencerReceipt } from '@paxeer/wallet';

const rpc = new JsonRpcClient({ url: chain.rpc_url });
try {
  const receipt = await waitForSequencerReceipt(
    (method, params) => rpc.call(method, params),
    sent.tx_hash,
    { rung: 'committed', attempts: 30, intervalMs: 1000 },
  );
  if (receipt.rung === 'committed') {
    console.log(receipt.blockNumber, receipt.blockHash, receipt.status);
  }
} catch (error) {
  if (error instanceof ReceiptTimeoutError) {
    console.log('Still checking', sent.tx_hash, error.lastAnswer?.rung);
  } else {
    throw error;
  }
}
```

`readSequencerReceipt(rpc, hash)` performs one authenticated lookup and can return `null`. `authenticateSequencerReceipt(rpc, value, expectedHash)` authenticates an existing answer. `waitForSequencerReceipt` also accepts an `AbortSignal`; its polling limit bounds attempts, not the duration of each fetch. A timeout does not cancel or reverse the submitted transaction.

## Usage (React)

Import hooks from `@paxeer/wallet/react`. Mount the wallet reader after authentication, and remount it when the user changes. `useWallet()` auto-provisions; `useStandardAccount()` provides passive reads and an explicit `refresh()` instead.

```tsx
import type { PaxeerWallet } from '@paxeer/wallet';
import { usePaxeerWallet, useSession, useWallet } from '@paxeer/wallet/react';

const config = {
  apiUrl: process.env.NEXT_PUBLIC_PAXEER_WALLET_API!,
  supabaseUrl: process.env.NEXT_PUBLIC_SUPABASE_URL!,
  supabaseAnonKey: process.env.NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY!,
};

export function App() {
  const paxeer = usePaxeerWallet(config);
  const { user, loading } = useSession(paxeer);

  if (loading) return <p>Loading…</p>;
  if (!user) return <button onClick={() => paxeer.signInWithOAuth('google')}>Sign in</button>;
  return <WalletDetails key={user.id} paxeer={paxeer} />;
}

function WalletDetails({ paxeer }: { paxeer: PaxeerWallet }) {
  const { wallet, loading, error } = useWallet(paxeer);
  if (loading) return <p>Loading wallet…</p>;
  if (error) return <p>{error.message}</p>;
  return <p>Wallet: {wallet?.address ?? 'Unavailable'}</p>;
}
```

## API surface

The current [`PaxeerWallet`](src/index.ts) methods are:

- Authentication: `signInWithEmail(email, redirectTo?)`, `signInWithOAuth(provider, redirectTo?)`, `signOut()`, `getSession()`, `getUser()`, `onAuthStateChange(cb)` (returns an unsubscribe function).
- Provisioning: `getWallet()`, `getStandardSelf()`, `provisionStandardWallet()`.
- Signing and sending: `signTransaction(tx)` returns `{ signed_tx, address, chain_id }`; `sendTransaction(tx)` returns the send response described above; `signMessage(message)` returns `{ signature, address }` for EIP-191 personal signing. Transaction amounts and fees accept bigint or decimal strings; `nonce` and `chainId` are numbers.
- EVM consent: `reviewEvmTransaction(unsignedTransaction)`, `approveEvmTransaction(reviewId)`, `evmApprovalStatus(reviewId)`.

Funded accounts are retired. `listFundedTiers()`, `getFundedSelf()`, `provisionFundedAccount()`, `signFundedTransaction()`, `sendFundedTransaction()` and `signFundedMessage()` remain compatibility methods that reject with `FundedUnsupportedError` (`FUNDED_UNSUPPORTED`, status `410`). The exported `useFundedTiers` and `useFundedAccount` hooks surface those errors; they do not enable funded accounts.

Other exports include:

- [`PaxeerProvider`](src/provider.ts) (EIP-1193), [`WalletInterface`](src/wallet.ts) for embedded or injected providers, and EIP-6963 [`announceProvider`, `discoverProviders` and `install`](src/eip6963.ts).
- [`EndpointClient`](src/endpoint.ts), [`KernelClient` and `KernelAvailability`](src/kernel.ts), [`HumanClient`](src/human.ts), receipt and transfer-tracking helpers, [module clients](src/modules/), and [agent request and binding signing helpers](src/agent.ts).
- React hooks: `usePaxeerWallet`, `useSession`, `useWallet` and `useStandardAccount`, plus the retired funded compatibility hooks above.

Wallet HTTP failures use `PaxeerWalletError` with `code`, `status` and `detail`; a missing session produces `NO_SESSION` (401). JSON-RPC callers use `JsonRpcError` or `JsonRpcTransportError`. EIP-1193 provider failures use the codes in [`errors.ts`](src/errors.ts).

## Node reads and hosted operations

`EndpointClient` defaults to `https://api-mainnet-beta.paxeer.network` for node calls. Public node JSON-RPC uses the root URL of that alias or `api1` through `api14.mainnet-beta.paxeer.network`, with chain ID `125`. See [public RPC endpoints](../../docs/site/docs/reference/public-rpc.md).

Set `nodeUrl` (or the legacy `url` option) to choose a node. `px_` reads go to the node root; only `px_register` and `px_requestFunds` require `hostedUrl`. Account, asset, balance, network, capabilities and history reads use the node, as does `px_getReceiptKeys`. Configure `hostedUrl` separately for the hosted gateway's two operations; its contract is [OpenRPC](../../services/platform/hosted/gateway/openrpc.json).

`EndpointClient.batch()` uses the same split: it sends `px_register` and `px_requestFunds` to the hosted gateway and all other methods, including `px_getReceiptKeys`, to the node, then returns results in the original call order. Without `hostedUrl`, a batch containing either hosted operation throws `JsonRpcTransportError`. The legacy error message refers to hosted reads, but read-only batches do not require `hostedUrl`.

`KernelClient` uses this endpoint and `KernelAvailability` to return either `{ available: true, result }` or a typed unavailable state. Availability depends on a consensus head that is not catching up. Recognized kernel-unavailable responses use JSON-RPC code `-32010` with `data.code = "kernel_unavailable"`; unrelated errors propagate. These clients access the same network and its hosted services.
