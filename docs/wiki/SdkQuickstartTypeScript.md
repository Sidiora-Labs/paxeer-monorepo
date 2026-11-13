# TypeScript SDK quickstart

Package: `@sidiora/layerx-sdk` at `services/agent/sdk/typescript`.
The package requires Node.js 22 or later. Its public exports are in
[`src/index.ts`](../../services/agent/sdk/typescript/src/index.ts).
Offline receipt verification lives in `src/verifier.ts`; the hosted
human-plane client is `ProductionClient` in `src/production.ts`.

Paxeer mainnet-beta is one BFT network. Its kernel participates in the
network and proposes blocks; a receipt is final when its block is committed.
Public node JSON-RPC uses the configured node's HTTPS root, with EVM chain ID
`125` (`0x7d`). See [Public RPC](PublicRpc.md) for node access. Hosted onboarding
and payment APIs require an operator-provided deployment; the node origins are not hosted service origins.

---

## Register

`JsonRpcClient` has no `register` method. Its private `call` is not
part of the public surface, so this package cannot invoke
`px_register` through that client.

The generated agent operation `"agent.register"` exists on
`Client.call` and `ProductionClient.agent`. That is agent
registration, not public self-service principal creation.
`AgentHttpTransport` only routes `program.*` operations; calling
`agent.register` through it returns `unavailable-capability`.

For hosted principal creation, the gateway contract declares `px_register`
with positional parameters `[signer_public_key, registration_signature]`.
Use the operator's configured gateway and its authentication requirements;
do not send this onboarding request to a public node. See
[Public JSON-RPC](PublicRpc.md) for the hosted contract and
[`rpc_register.rs`](../../services/platform/hosted/gateway/src/rpc_register.rs)
for the registration implementation.

---

## Fund from faucet

`JsonRpcClient` has no `requestFunds` method. Hosted faucet requests use the
configured faucet service.

Use [Hosted faucet](HostedFaucet.md) `POST /v1/faucet/claims` with a
Bearer session, an `Idempotency-Key`, and JSON `did` and `public_key` fields
on a configured hosted faucet. A `202` response means funding is still being
checked; it is not confirmation of a funded account. Hosted funding does not
establish that the public nodes offer a faucet.

---

## Send

The current `JsonRpcClient` exposes `sendRawTransaction(raw)` for signed EVM
transaction bytes and `sequencerReceipt(txHash)` for `pax_sequencerReceipt`.
Its `Commitment` type is `"submitted" | "receipted" | "committed"`.
Other exposed methods include `getAccount`, `getBalance`, `getBalances`,
`getSequence`, `getIdentitySequence`, `getBatchHeader`, `getNodeInfo`,
`listAssets`, `listAssetsPage`, `getAsset`, `estimateFee`, `getProof`,
`subscribe`, and `subscribeFrom`. Check `src/rpc.ts` for their actual wire
methods; these wrappers do not all target public-node namespaces.

The `JsonRpcClient` constructor in `src/rpc.ts` accepts an HTTPS URL (or
loopback HTTP) without credentials, query or fragment, and posts to that URL
unchanged; pass the node root URL. For a plain root-path request without the
SDK, this example accepts a transaction already signed by your wallet for
chain ID 125 and returns its submission hash:

```ts
import { Buffer } from "node:buffer";

export async function send(nodeRoot: string, raw: Uint8Array): Promise<string> {
  if (raw.length === 0 || raw.length > 524288) {
    throw new Error("Invalid transaction length");
  }
  const response = await fetch(nodeRoot, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    signal: AbortSignal.timeout(30_000),
    body: JSON.stringify({
      jsonrpc: "2.0",
      id: "send",
      method: "eth_sendRawTransaction",
      params: [`0x${Buffer.from(raw).toString("hex")}`],
    }),
  });
  if (!response.ok) throw new Error(`RPC HTTP ${response.status}`);
  const result = await response.json();
  if (result.jsonrpc !== "2.0" || result.id !== "send") {
    throw new Error("Invalid RPC response");
  }
  if (result.error) {
    throw new Error(`RPC ${result.error.code}: ${result.error.message}`);
  }
  if (typeof result.result !== "string" || !/^0x[0-9a-fA-F]{64}$/.test(result.result)) {
    throw new Error("Invalid transaction hash");
  }
  return result.result;
}
```

Submission is not finality. Query `pax_sequencerReceipt` with `[txHash]` at
the same node root to follow its `rung`: `submitted` has no signed receipt,
`receipted` carries signed receipt bytes, and `committed` also identifies the
landing block with `blockNumber` and `blockHash`. Node support depends on its
receipt source or forwarding configuration; the unconfigured implementation
returns `-32000`. The response contract is in
[`rpc/sequencer_receipt.go`](../../rpc/sequencer_receipt.go).

Hosted human-plane send (quote then commit), as in
`services/platform/examples/samples/first-payment-typescript`:

```ts
import { ProductionClient, idempotencyKey } from "@sidiora/layerx-sdk";

type MoveQuote = { quote_id: string };

export async function pay(
  layerx: ProductionClient,
  source: string,
  destination: string,
  money: { amount: string; currency: string },
  paymentKey: string,
) {
  const request = { source, destination, money };
  const quote = await layerx.human<typeof request, MoveQuote>("move.quote", request);
  return layerx.human(
    "move.commit",
    { quote_id: quote.quote_id },
    { idempotencyKey: idempotencyKey(paymentKey) },
  );
}
```

`ProductionClient` needs a `ProductionTransport`. The in-repo sample
uses `@sidiora/layerx-buyer-middleware`'s
`LayerXPaymentHttpTransport`. That transport is not defined inside
`@sidiora/layerx-sdk`. Configure it with the hosted API URL and Bearer token;
`move.commit` requires an idempotency key. The returned journey must be
followed to its outcome, as the sample does with `journey.get`.

---

## Verify a receipt

Use the two current verification functions, in order:

```ts
import { verifyCommittedReceipt, verifySequencerReceipt } from "@sidiora/layerx-sdk";
import type {
  CommittedReceiptVerification,
  NodeCommittedReceiptEvidence,
  SequencerReceiptKey,
} from "@sidiora/layerx-sdk";

export async function verify(
  canonicalReceipt: Uint8Array,
  rawTransaction: Uint8Array,
  trustedReceiptKeys: readonly SequencerReceiptKey[],
  evidence: NodeCommittedReceiptEvidence,
): Promise<CommittedReceiptVerification> {
  const signed = await verifySequencerReceipt(
    canonicalReceipt,
    rawTransaction,
    trustedReceiptKeys,
  );
  return verifyCommittedReceipt(signed, evidence);
}
```

`verifySequencerReceipt` binds the canonical receipt to the Keccak-256 hash
of the exact signed `rawTransaction` and checks its Ed25519 signature against
a pinned key active at the receipt height. Each `SequencerReceiptKey` has
32-byte `keyId` and `publicKey` arrays and inclusive `bigint` bounds
`first_height` and `last_height`. Authenticate those pins through a trusted
network source; the `signingKey` returned alongside an untrusted receipt is
not a trust anchor. This first step returns `level: "sequencer-signed"`,
which alone does not prove inclusion in a committed block.

`verifyCommittedReceipt` requires the full `NodeCommittedReceiptEvidence`,
including the Comet header, block ID, commit signatures, and the validator
set trusted by the caller for that chain and landing height. Its `chainId`
is the consensus chain identifier, not the EVM numeric chain ID. The caller
must authenticate that chain identifier and validator set independently;
do not trust a validator set merely because it accompanies a proof.

The evidence's `inclusion` contains `batchTransaction`, `batchProof`
(`total`, `index`, `aunts`), and the ordered `activityIds`. The verifier binds
the header hash to the block ID and the trusted validator set to the header,
verifies the sequence-batch and wrapped EVM transaction proofs against the
execution header's data hash, and verifies the result proof against the
successor header's last-results hash. The evidence also requires
`executionHeader`, `successorHeader`, `executionCommit`, `executionValidators`,
`transaction`, `transactionProof`, `result` and `resultProof`. The verifier binds
the batch and receipt's sequence position and requires valid commit signatures
representing strictly more than two-thirds of total voting power. The landing
height must be at least the receipt height. A commit object without the header and inclusion proof is
insufficient, even though the broader `CommittedReceiptEvidence` type exists.
Supply decoded evidence from the network; this SDK does not automatically
fetch or assemble it for this function.

The second step returns `level: "committed"`, the landing height and block
hash. That is finality on the merged BFT network. A server's `rung` string or
an ordinary EVM receipt is not a substitute for this offline proof.
Verification establishes authenticity and commitment, not successful
execution: inspect `verified.receipt.receipt.resultCode` on the returned
`CommittedReceiptVerification`; zero means success and a nonzero code is a
committed failure. Both verification stages can throw
`SequencerReceiptVerificationError` with `code: "verification-failure"`;
its `check` distinguishes `unpinned_key`, `key_out_of_range`, `bad_signature`,
`hash_mismatch`, `height_regression`, `insufficient_power`, and
`inclusion_mismatch`.

See
[`src/verifier.ts`](../../services/agent/sdk/typescript/src/verifier.ts)
for the exact evidence types and checks.

[SDK quickstarts](SdkQuickstarts.md) · [Home](Home.md)
