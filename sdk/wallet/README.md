# `@paxeer/wallet`

TypeScript SDK for Paxeer wallet applications: Supabase email and OAuth sign-in, an embedded wallet, an EIP-1193 provider, EIP-6963 discovery, EVM transaction review, authenticated receipts, and clients for node account reads, hosted operations, and the human service. Paxeer mainnet-beta uses EVM chain ID `125` (`0x7d`). The LayerX kernel inside `paxd` executes and orders each transaction and builds each block; validators check and commit it. A transaction is final once its block is committed. Sub-chains are tenant domains in the database, not separate networks that a wallet must switch to.

## Layout

- `src/` — TypeScript SDK sources for authentication, embedded and injected wallets, EIP-1193 and EIP-6963 integration, node and hosted clients, receipt verification, kernel availability, human intents, agent signing, transfer tracking, and module calls.
- `test/` — Vitest tests, support helpers, and JSON fixtures for wallet, provider, endpoint, receipt, module, and network behavior.
- `.gitignore` — ignores generated package output and coverage files.
- `package.json` — package metadata, public exports, dependencies, and build, test, typecheck, and development scripts.
- `pnpm-lock.yaml` — pnpm dependency lockfile for the package workspace.
- `tsconfig.json` — TypeScript compiler configuration for the `src/` tree.
- `README.md` — this package page.

## Build and test

The package is a member of the [`services/wallet` pnpm workspace](../../services/wallet/pnpm-workspace.yaml). Workspace consumers can depend on `"@paxeer/wallet": "workspace:*"`; [`apps/wallet`](../../apps/wallet/package.json) uses `link:../../sdk/wallet`. The entry points are `@paxeer/wallet`, `@paxeer/wallet/react`, `@paxeer/wallet/provider`, `@paxeer/wallet/receipt` and `@paxeer/wallet/types`. The last three expose TypeScript source and require a consumer that supports it. React is an optional peer dependency (version 18 or later).

From the repository root, install the wallet workspace and run the package scripts declared in `package.json`:

```sh
cd services/wallet
pnpm install
pnpm --filter @paxeer/wallet build
pnpm --filter @paxeer/wallet test
pnpm --filter @paxeer/wallet typecheck
```

`build` and `test` build the TypeScript agent SDK dependency before compiling or running Vitest. `typecheck` performs the same dependency build and runs `tsc --noEmit`.

## Configuration

The runtime SDK receives `apiUrl`, `supabaseUrl`, `supabaseAnonKey`, and optional `fetch` through `PaxeerWalletConfig`; `EndpointClient` receives `nodeUrl` or `url`, optional `hostedUrl`, `fetch`, `headers`, `timeoutMs`, and `maxResponseBytes`; `PaxeerProvider` receives `gatewayUrl`, `rpcUrl`, `token`, optional `chainId`, `confirm`, and `fetch`; `HumanClient` receives `url`, `kernel`, optional `fetch`, and `authorization`. The SDK source does not read runtime environment variables directly.

The integration tests read `PG_BIN_DIR`, `PATH`, and `HOME` for subprocesses and `PX_NODE_CATALOGUE_URL` and `PX_NODE_CATALOGUE_METHODS` for the node route catalogue test.

The application examples use `NEXT_PUBLIC_PAXEER_WALLET_API`, `NEXT_PUBLIC_SUPABASE_URL`, and `NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY`, as read by the [wallet demo](../../services/wallet/demo/src/lib/paxeer.ts). These are application configuration names.

Build configuration is in [`package.json`](package.json), [`pnpm-lock.yaml`](pnpm-lock.yaml), and [`tsconfig.json`](tsconfig.json), which extends [`services/wallet/tsconfig.base.json`](../../services/wallet/tsconfig.base.json).

### Usage (vanilla)

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
const email = document.querySelector<HTMLInputElement>('input[type="email"]')?.value;
if (!email) throw new Error('Enter your email address');
const { error } = await paxeer.signInWithEmail(email);
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
  to: wallet.address, // send to this wallet
  value: 1_000_000_000_000_000n, // 0.001 PAX, in wei
  chainId: 125,
});
console.log(sent.tx_hash, sent.receipt?.rung);
```

Use `getStandardSelf()` for a passive read: it returns `null` when the authenticated user has no wallet. Call `provisionStandardWallet()` after an explicit creation choice. `getWallet()` combines these operations; provision before calling the signing or sending methods. Signing happens on the API server; this client does not receive custody key material. Requests remain subject to authentication, signing policy and gateway refusals.

### EVM review and approval

Configure `evmReview.storage` with persistent `getItem` and `setItem` methods before using this flow. Browser `localStorage` satisfies that interface; other environments must supply their own storage. Reviews are retained under the gateway origin and authenticated user's ID, and reused when the client is recreated with the same origin, user and storage.

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

### Authenticated receipts

`sendTransaction()` returns `{ tx_hash, address, chain_id, receipt? }`. It validates the send response and preserves a returned `submitted` answer as pending. It authenticates any execution receipt, and queries `pax_sequencerReceipt` through the wallet's `chain.rpc_url` when no receipt was returned. A missing receipt is still pending evidence, not transaction failure. A receipt lookup or authentication error can occur after broadcast; retain the original transaction hash and reconcile before submitting again.

The wire API and exported types retain the names `pax_sequencerReceipt` and `SequencerReceiptAnswer`. These describe receipts from the merged network's kernel. The receipt stages are:

- `submitted`: transaction hash and nonce only; no signed execution evidence yet.
- `receipted`: signed execution evidence, including status, logs and fees.
- `committed`: execution evidence plus block number and block hash.

The [receipt helpers](src/receipt.ts) check transaction identity, signed receipt bytes, decoded execution fields, the Ed25519 signature, and the signing key's governed height range. They obtain key pins from the node's `px_getReceiptKeys` method and the signed transaction from `eth_getRawTransactionByHash`, then bind the receipt to that transaction on chain `125`. Authentication failure throws; it must not be displayed as successful execution. Execution status `0x0` means failure even when the transaction is committed.

Continue the vanilla send example to wait for commitment with bounded polling:

```ts
import { JsonRpcClient, ReceiptTimeoutError, waitForSequencerReceipt } from '@paxeer/wallet';

const rpc = new JsonRpcClient({ url: chain.rpc_url });
try {
  const receipt = await waitForSequencerReceipt(
    (method, params) => rpc.call(method, params),
    sent.tx_hash,
    { rung: 'committed', attempts: 30, intervalMs: 1000 }
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

`readSequencerReceipt(rpc, hash)` performs one lookup and can return `null` or a pending `submitted` answer. It authenticates `receipted` and `committed` answers. `authenticateSequencerReceipt(rpc, value, expectedHash)` authenticates an existing answer. `waitForSequencerReceipt` also accepts an `AbortSignal`; its polling limit bounds attempts, not the duration of each fetch. A timeout does not cancel or reverse the submitted transaction.

### Agent and owner clients

`OwnerClient` covers the signed-in user's agent inventory, ownership, policy and treasury routes. Supply the user's bearer token, or a callback that reads the current session token before each call. `AgentClient` takes an agent read token for reads and refresh, and `AgentSigningOptions` for writes. It signs the exact JSON bytes with `signAgentRequest`; the Ed25519 key stays with the caller. These clients are exported from `@paxeer/wallet` and the `@paxeer/wallet/owner` and `@paxeer/wallet/agent-client` subpaths.

```ts
import { OwnerClient, AgentClient, GatewayClientError } from '@paxeer/wallet';

const owner = new OwnerClient({
  apiUrl: process.env.PAXEER_WALLET_API!,
  token: async () => (await paxeer.getSession())?.access_token ?? '',
});
const { agents } = await owner.list();

const agent = new AgentClient({ apiUrl: process.env.PAXEER_WALLET_API! });
// Use the registered did:matrix identity and its Ed25519 seed supplied by the caller.
async function readAndSend(did: string, privateKey: Uint8Array, agentToken: string) {
  const self = await agent.me(agentToken);
  if (!self.wallet) throw new Error('Provision the agent wallet first');
  try {
    return await agent.send(
      { tx: { to: self.wallet.address, value: '1', chainId: 125 } },
      {
        keyId: did,
        privateKey,
      }
    );
  } catch (error) {
    if (error instanceof GatewayClientError) {
      console.error(error.status, error.code, error.body);
    }
    throw error;
  }
}
```

Amounts and fees use decimal strings in base units. `challenge({ did })` returns the exact UTF-8 `message` to sign with Ed25519; pass that signature, public key, DID and nonce to `verify`. `verify` and `refresh` return the agent's read token; the caller retains and replaces it. Signed writes generate a fresh nonce and a 60-second expiry by default. `authorizationId` carries a retained attestor authorization when needed. Supplying `nonce` or `expiresAt` explicitly gives the caller responsibility for freshness and the gateway's expiry limit.

The clients return every JSON response with a 2xx HTTP status. `simulate` returns `AgentSimulationResponse`: read `ok`, `gas`, `return_data` and `error` to distinguish a successful simulation from a revert. `send`, `transfer` and `approve` return `AgentBroadcastResponse`; when `error` is `outcome_unknown`, retain the transaction hash and follow the receipt polling instructions without resubmitting. Non-2xx responses throw `GatewayClientError`, which preserves the HTTP `status`, gateway `code`, message and original `body`, including nested action errors and recovery instructions. A 2xx response with invalid JSON throws `invalid_gateway_response`. The clients send each request once. `allowanceAndCall` preserves `idempotency_key` and returns the durable action, including its status, error, retry guidance and poll endpoint; `action` reads that state even when its `error` envelope is present. Accepted actions and provisioning can return before completion. Successful send responses preserve the gateway's receipt and rung; callers must authenticate execution receipts before treating them as verified evidence.

Owner methods take `did` before any request body or resource ID. All owner routes use the user bearer token:

| Method                        | Gateway route                              |
| ----------------------------- | ------------------------------------------ |
| `list()`                      | `GET /v1/agents`                           |
| `read(did)`                   | `GET /v1/agents/:did`                      |
| `claim(did, claim)`           | `POST /v1/agents/:did/claim`               |
| `updatePolicy(did, patch)`    | `PUT /v1/agents/:did/policy`               |
| `freeze(did)`                 | `POST /v1/agents/:did/freeze`              |
| `unfreeze(did)`               | `POST /v1/agents/:did/unfreeze`            |
| `listRules(did)`              | `GET /v1/agents/:did/rules`                |
| `addRule(did, rule)`          | `POST /v1/agents/:did/rules`               |
| `removeRule(did, ruleId)`     | `DELETE /v1/agents/:did/rules/:ruleId`     |
| `listBudgets(did)`            | `GET /v1/agents/:did/budgets`              |
| `addBudget(did, budget)`      | `POST /v1/agents/:did/budgets`             |
| `removeBudget(did, budgetId)` | `DELETE /v1/agents/:did/budgets/:budgetId` |
| `fund(did, funding)`          | `POST /v1/agents/:did/fund`                |
| `sweep(did, sweep)`           | `POST /v1/agents/:did/sweep`               |
| `activity(did)`               | `GET /v1/agents/:did/activity`             |

Use `buildClaim` to construct the agent's countersignature for `claim`. Funding moves native coin or tokens from the owner's standard wallet to the agent. Sweep defaults to the owner's standard wallet when `to` is omitted. Policy patches preserve explicit `null` limits, and budget creation returns the created budget without the usage fields included by budget reads.

| Agent method                      | Gateway route                               | Authentication                      |
| --------------------------------- | ------------------------------------------- | ----------------------------------- |
| `challenge(body)`                 | `POST /v1/agent/auth/challenge`             | Public                              |
| `verify(body)`                    | `POST /v1/agent/auth/verify`                | Public, challenge signature in body |
| `refresh(token)`                  | `POST /v1/agent/auth/refresh`               | Agent token                         |
| `whoami(token)`                   | `GET /v1/agent/whoami`                      | Agent token                         |
| `provision(body, signing)`        | `POST /v1/agent/provision`                  | Signed request                      |
| `me(token)`                       | `GET /v1/agent/me`                          | Agent token                         |
| `balances(token, tokens?)`        | `GET /v1/agent/balances?tokens=…`           | Agent token                         |
| `nonce(token)`                    | `GET /v1/agent/nonce`                       | Agent token                         |
| `simulate(token, body)`           | `POST /v1/agent/simulate`                   | Agent token                         |
| `allowance(token, query)`         | `GET /v1/agent/allowance?token=…&spender=…` | Agent token                         |
| `transaction(token, hash)`        | `GET /v1/agent/tx/:hash`                    | Agent token                         |
| `activity(token)`                 | `GET /v1/agent/activity`                    | Agent token                         |
| `sign(body, signing)`             | `POST /v1/agent/sign`                       | Signed request                      |
| `send(body, signing)`             | `POST /v1/agent/send`                       | Signed request                      |
| `signMessage(body, signing)`      | `POST /v1/agent/sign-message`               | Signed request                      |
| `signTypedData(body, signing)`    | `POST /v1/agent/sign-typed-data`            | Signed request                      |
| `transfer(body, signing)`         | `POST /v1/agent/transfer`                   | Signed request                      |
| `approve(body, signing)`          | `POST /v1/agent/approve`                    | Signed request                      |
| `allowanceAndCall(body, signing)` | `POST /v1/agent/actions/allowance-and-call` | Signed request                      |
| `action(token, actionId)`         | `GET /v1/agent/actions/:action_id`          | Agent token                         |

### Usage (React)

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

### API surface

The current [`PaxeerWallet`](src/index.ts) methods are:

- Authentication: `signInWithEmail(email, redirectTo?)`, `signInWithOAuth(provider, redirectTo?)`, `signOut()`, `getSession()`, `getUser()`, `onAuthStateChange(cb)` (returns an unsubscribe function).
- Provisioning: `getWallet()`, `getStandardSelf()`, `provisionStandardWallet()`.
- Signing and sending: `signTransaction(tx)` returns `{ signed_tx, address, chain_id }`; `sendTransaction(tx)` returns the send response described above; `signMessage(message)` returns `{ signature, address }` for EIP-191 personal signing. Transaction amounts and fees accept bigint or decimal strings; `nonce` and `chainId` are numbers.
- EVM consent: `reviewEvmTransaction(unsignedTransaction)`, `approveEvmTransaction(reviewId)`, `evmApprovalStatus(reviewId)`.

The funded compatibility methods `listFundedTiers()`, `getFundedSelf()`, `provisionFundedAccount()`, `signFundedTransaction()`, `sendFundedTransaction()` and `signFundedMessage()` reject with `FundedUnsupportedError` (`FUNDED_UNSUPPORTED`, status `410`). The exported `useFundedTiers` and `useFundedAccount` hooks surface those errors; they do not enable funded accounts.

Other exports include:

- [`PaxeerProvider`](src/provider.ts) (EIP-1193), [`WalletInterface`](src/wallet.ts) for embedded or injected providers, and EIP-6963 [`announceProvider`, `discoverProviders` and `install`](src/eip6963.ts).
- [`EndpointClient`](src/endpoint.ts), [`KernelClient` and `KernelAvailability`](src/kernel.ts), [`HumanClient`](src/human.ts), receipt and transfer-tracking helpers, [module clients](src/modules/), and [agent request and binding signing helpers](src/agent.ts).
- React hooks: `usePaxeerWallet`, `useSession`, `useWallet` and `useStandardAccount`, plus the funded compatibility hooks above.

Wallet HTTP failures use `PaxeerWalletError` with `code`, `status` and `detail`; a missing session produces `NO_SESSION` (401). JSON-RPC callers use `JsonRpcError` or `JsonRpcTransportError`. EIP-1193 provider failures use the codes in [`errors.ts`](src/errors.ts).

### Node reads and hosted operations

`EndpointClient` uses the public alias in `DEFAULT_ENDPOINT_URL` for node calls. Public node JSON-RPC uses the root URL, with chain ID `125`. See the [public RPC documentation site page](../../docs/site/paxeer-docs/src/app/public-rpc/page.tsx).

Set `nodeUrl` (or the legacy `url` option) to choose a node. `px_` reads go to the node root; only `px_register` and `px_requestFunds` require `hostedUrl`. Account, asset, balance, network, capabilities and history reads use the node, as does `px_getReceiptKeys`. Configure `hostedUrl` separately for the hosted gateway's two operations; its contract is [OpenRPC](../../services/platform/hosted/gateway/openrpc.json).

`EndpointClient.batch()` uses the same split: it sends `px_register` and `px_requestFunds` to the hosted gateway and all other methods, including `px_getReceiptKeys`, to the node, then returns results in the original call order. Without `hostedUrl`, a batch containing either hosted operation throws `JsonRpcTransportError`. Read-only batches do not require `hostedUrl`.

`KernelClient` uses this endpoint and `KernelAvailability` to return either `{ available: true, result }` or a typed unavailable state. Availability depends on a consensus head that is not catching up. Recognized kernel-unavailable responses use JSON-RPC code `-32010` with `data.code = "kernel_unavailable"`; unrelated errors propagate. These clients access the same network and its hosted services.

## Documentation

- [SDK surface index](../../docs/system/sdk.md)
- [Wallet app architecture](../../docs/wallet/app/architecture/README.md)
- [Wallet app development](../../docs/wallet/app/development/README.md)
- [SDK documentation site page](../../docs/site/paxeer-docs/src/app/sdk/page.tsx)
- [SDK documentation site markdown](../../docs/site/paxeer-docs/public/markdown/sdk.md)
- [Public RPC documentation site page](../../docs/site/paxeer-docs/src/app/public-rpc/page.tsx)
