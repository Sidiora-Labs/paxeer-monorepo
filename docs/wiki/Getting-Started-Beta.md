# Getting started

Paxeer mainnet-beta is one BFT network, with EVM chain ID `125` (`0x7d`).
The kernel runs inside the network and proposes blocks. Use ordinary signed
EVM transactions for transfers and native precompile calls; a receipt is final
when its block is committed. [Unified network](UnifiedNetwork.md) describes
the execution and evidence model.

## Endpoints and trust

Choose a public node from [Public RPC](PublicRpc.md) and set `RPC_URL`
to its root URL. Check its chain ID before signing:

```sh
curl --fail-with-body --silent --show-error "$RPC_URL" \
  --header 'Content-Type: application/json' \
  --data '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
```

Require `result` to be `"0x7d"` before signing. Compare
`eth_blockNumber` when choosing a current node; an unavailable or lagging node
is not evidence that a transaction failed.

The node also serves the `px` namespace at the root. `px_register` and
`px_requestFunds` are operations of the operator-deployed hosted gateway,
whose contract is `services/platform/hosted/gateway/openrpc.json`; obtain its
endpoint and any credentials from the operator when using those operations.

## Wallet and funding

Connect a signing wallet to chain `125` and obtain native PAX for transaction
fees from an existing funded account or an operator-provided allocation.
Check the address and balance with `eth_getBalance(address, "latest")`.
A DID binding does not supply gas or create a ledger balance.

For a wallet whose EVM key and DID derive from one secret, follow
[One account](../site/paxeer-docs/src/app/accounts/page.tsx). The CLI can derive the pair from
standard input:

```sh
layerx wallet derive --index 0 < phrase.txt
```

Keep the phrase file private. To submit the binding with the CLI, use:

```sh
layerx --rpc "$RPC_URL" wallet derive --index 0 --bind < phrase.txt
```

The command checks chain ID `125` and the binding state, then returns
`submitted` or `already_bound`. A submitted bind transaction needs native gas
funding and must commit successfully before a later operation uses the binding.
The TypeScript SDK also exports `autoBindLayerX` and signed bind transaction
helpers in
[`account-derivation.ts`](../../services/agent/sdk/typescript/src/account-derivation.ts).
See [CLI transport](Cli.md#emulator-hosted-gateway-and-node) for node and hosted
endpoint selection.

## Submit a native transaction

For a browser application using ethers v6 and an EIP-1193 wallet, this function
requests the user's approval for a native PAX transfer. Supply the intended
recipient and amount from the application's payment screen.

```ts
import { BrowserProvider, parseEther } from "ethers";

async function transferPax(recipient: string, amountPax: string) {
  const provider = new BrowserProvider(window.ethereum);
  if ((await provider.getNetwork()).chainId !== 125n) {
    throw new Error("Connect the wallet to Paxeer mainnet-beta (chain 125)");
  }
  await provider.send("eth_requestAccounts", []);
  const signer = await provider.getSigner();
  const tx = await signer.sendTransaction({
    to: recipient,
    value: parseEther(amountPax),
  });
  const receipt = await tx.wait();
  if (receipt === null || receipt.status !== 1) {
    throw new Error(`Transfer did not succeed: ${tx.hash}`);
  }
  return { transactionHash: tx.hash, receipt };
}
```

Applications that sign locally submit the signed bytes with
`eth_sendRawTransaction`. Retain those bytes and their transaction hash before
broadcasting. The browser wallet handles signing in the example above; the
public node does not hold the user's signing key.

## Ledger assets and payments

The native LayerXLedger precompile is
`0x000000000000000000000000000000000000101a`. Its exact ABI is
[`precompiles/layerxledger/abi.json`](../../precompiles/layerxledger/abi.json); implementation and keeper logic live in
`precompiles/layerxledger/` and `modules/layerxledger/`.

The register/open/mint/payment lifecycle uses EVM transactions to that
precompile. Bind each participating EVM address to its DID first. Wait for a
successful committed receipt after each prerequisite write:

| Step | Call and authority |
| --- | --- |
| Register | The issuer calls `register(assetId, salt, symbol, name, decimals, supplyCap, issuerKind, custodyReference)` with the asset's canonical parameters. |
| Open | Each owner calls `openAccount(assetId)` to open its per-asset account. |
| Identify | Read `accountId(owner, assetId)`, where `owner` is the bound DID public key as `bytes32`. Use these per-asset IDs for payments. |
| Mint | The authorized issuer calls `mint(assetId, accountId, amount)` within the asset's cap and policy. This is asset issuance, not a public gas faucet. |
| Pay | The source owner calls `payExact(from, to, assetId, amount, expiresAtMs, paymentReference)`. Amount is integer asset units; expiry is Unix milliseconds and the reference is `bytes32`. |
| Read | Use `balanceOf(accountId, assetId)`, `getAsset(assetId)` and `nextSequence(accountId, assetId)` through `eth_call`. |

`payExact` selects the next account sequence in the keeper and emits
`PaymentExact`. The `send(bytes envelope)` entry point also exists for the
canonical signed SEND envelope; its encoding is not interchangeable with
`payExact` calldata. Use the ABI and canonical builders for the chosen entry
point. The real-process lifecycle in `daemon/committed-fixture/vector_v4.go`
binds two identities, registers an asset, opens both accounts, mints and pays
through these EVM calls.

A native wallet balance and a per-asset ledger balance are different records
within the same chain. Read the balance appropriate to the operation; opening
an account does not mint units.

## Receipts and recovery

Read `eth_getTransactionReceipt(transactionHash)` for the EVM outcome.
A null receipt means no receipt is available from that node yet; status
`"0x0"` records an execution failure and `"0x1"` records success. Commitment
and execution success must both be established before delivering a payment.

Where configured, `pax_sequencerReceipt(transactionHash)` exposes
`submitted`, `receipted` and `committed` progress with a signed execution
receipt. A signature alone proves neither block inclusion nor commitment.
For independent verification, authenticate receipt key pins and chain trust,
then verify the transaction and execution-result inclusion proofs and BFT
commits as described in [SDK terminal verification](SdkTerminalVerification.md).

If a request times out, recover by the existing transaction hash. Check the
receipt and account nonce before authorizing another debit; rebroadcasting
identical signed bytes preserves the hash. Do not turn an uncertain payment
into a newly signed payment automatically.

Inspect the JSON-RPC `error` field even after a successful HTTP response.
`-32601` means the method is unavailable on that endpoint; `-32602` means
invalid parameters. The optional receipt API answers `-32000` when it has
neither a local receipt source nor a configured forwarding endpoint. Gateway
errors have their own declared method contract; preserve their typed data
instead of interpreting unavailable evidence as a zero balance or success.

For service-managed payments, use
[Human service journeys](../site/paxeer-docs/src/app/human-interface/page.tsx) and the deployment's
authentication and allowance policy.

[One account](../site/paxeer-docs/src/app/accounts/page.tsx) ·
[Unified network](UnifiedNetwork.md) ·
[SDK terminal verification](SdkTerminalVerification.md) ·
[Home](Home.md)
