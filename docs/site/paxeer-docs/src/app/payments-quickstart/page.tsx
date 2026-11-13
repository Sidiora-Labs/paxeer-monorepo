import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import Link from 'next/link'

export const metadata = metadataFor('/payments-quickstart')

export default function PaymentsQuickstart() {
  return (
    <DocsLayout>
      <div className="page-header">
        <h1 className="page-title">Your first payment</h1>
        <p className="page-description">Quote, commit, and verify a Paxeer X payment using the application SDK or a signed ledger transaction.</p>
      </div>
      <h2>Choose your integration</h2>
      <p>Use the application SDK when you want account-level payment journeys and bearer-token authentication. Use the native CLI when you control the wallet identity and its EVM key and need explicit transaction signing, receipt policy, fee limits, and commitment selection. The CLI submits ledger calls as signed EIP-1559 transactions on chain ID 125.</p>
      <h2>1. Install the application packages</h2>
      <pre><code>{`npm install @sidiora/layerx-sdk@0.1.0 @sidiora/layerx-buyer-middleware@0.1.0`}</code></pre>
      <p>The packages require Node.js 22 or newer. Their sources are <code>services/agent/sdk/typescript</code> and <code>services/platform/middleware/buyer</code>. In the sample under <code>services/platform/examples/samples/first-payment-typescript</code>, configure <code>LAYERX_API_URL</code> with your environment&apos;s API base URL and <code>LAYERX_API_TOKEN</code> with its account bearer token. Use HTTPS outside a loopback emulator. These credentials identify your API account; do not embed them in browser bundles or logs.</p>
      <h2>2. Quote and commit the payment</h2>
      <p>This SDK integration accepts your typed source, destination, and money values. Choose <code>paymentKey</code> from a persistent order identifier or request digest and retain it before committing.</p>
      <pre><code>{`import { LayerXPaymentHttpTransport } from "@sidiora/layerx-buyer-middleware";
import { ProductionClient, SecretBytes, idempotencyKey } from "@sidiora/layerx-sdk";

export const openLayerX = (apiUrl, apiToken) => new ProductionClient(
  new LayerXPaymentHttpTransport({
    baseUrl: apiUrl,
    bearerToken: new SecretBytes(new TextEncoder().encode(apiToken))
  })
);

export const pay = async (layerx, source, destination, money, paymentKey) => {
  const quote = await layerx.human("move.quote", { source, destination, money });
  return layerx.human(
    "move.commit",
    { quote_id: quote.quote_id },
    { idempotencyKey: idempotencyKey(paymentKey) }
  );
};`}</code></pre>
      <p>The quote establishes the payment terms, mechanism, fee ceiling, arrival expectation, and irreversible legs. Present these terms for approval before committing. An expired quote must be refreshed and approved again. Monetary quantities use integer base units rather than floating-point arithmetic.</p>
      <h2>3. Follow the original journey</h2>
      <p>Store the returned journey identifier alongside the purchase key. Read its progress through <code>journey.get</code> and resolve evidence references through <code>evidence.get</code>. A <code>layerx-receipt</code> reference identifies canonical receipt evidence. Finish only when the journey reaches <code>done</code> or <code>done-finalised</code> with verified evidence.</p>
      <table><thead><tr><th>Observed outcome</th><th>Next action</th></tr></thead><tbody>
        <tr><td><code>waiting-for-you</code></td><td>Obtain the requested decision before expiry.</td></tr>
        <tr><td><code>still-checking</code></td><td>Track the existing journey; lock controls that could create a duplicate payment.</td></tr>
        <tr><td><code>refused</code></td><td>Read refusal authority and <code>money_left</code>.</td></tr>
        <tr><td><code>done</code> / <code>done-finalised</code></td><td>Verify evidence and record the settlement before fulfilment.</td></tr>
      </tbody></table>
      <h2>Native path: prepare a funded identity</h2>
      <p>The CLI uses a selected local identity, its matching owner-only EVM key export, and an independently supplied receipt policy. The policy requires protocol version 2, network 125, a receipt signer identity and key, and a bounded height range. Prepare the matching identity and EVM key with the wallet derivation and import workflow in the CLI guide. Receipt verification also relies on the configured node connection.</p>
      <pre><code>{`cargo build --locked --manifest-path tools/platform/cli/Cargo.toml
export PATH="$PWD/tools/platform/target/debug:$PATH"
layerx wallet list
layerx key default payer`}</code></pre>
      <p>In the examples below, <code>RPC_URL</code> is the node JSON-RPC root URL, and <code>GATEWAY_RPC_URL</code> is the hosted gateway root URL supplied by its operator. <code>GATEWAY_CREDENTIAL</code> names a locally stored gateway credential. These shell variables are sample inputs. Configure <code>RECEIPT_POLICY</code>, <code>EVM_KEY_FILE</code>, <code>FEE_LIMIT</code>, <code>ASSET_ID</code>, and <code>RECIPIENT_DID</code> for the selected wallet, fee ceiling and payment.</p>
      <p>The selected wallet must hold a spendable balance of the chosen Asset, and its EVM sender must hold enough native currency for gas. <code>EVM_KEY_FILE</code> must be a private-key export from <code>wallet derive --export-private-keys</code> for the same identity. Confirm the source account and recipient account through the hosted gateway before sending.</p>
      <h2>Open the recipient account and send</h2>
      <p>Select the recipient wallet to open its account for the chosen Asset, then select the funded payer wallet to transfer. The CLI reads the EVM sender nonce from the node and payment account state from the hosted gateway. The signing flow discloses the transaction and debit authorization, including recipient, amount, Asset, fee ceiling, expiry, network, and idempotency key.</p>
      <pre><code>{`# Run with the recipient wallet and its matching EVM_KEY_FILE selected.
layerx --rpc "$RPC_URL" --gateway-credential "$GATEWAY_CREDENTIAL" wallet open-account \\
  --gateway-rpc "$GATEWAY_RPC_URL" --asset "$ASSET_ID" \\
  --evm-key-file "$EVM_KEY_FILE" --receipt-policy "$RECEIPT_POLICY" \\
  --fee-limit "$FEE_LIMIT" --wait committed

# Select the funded payer and set EVM_KEY_FILE to its matching export.
layerx key default payer
layerx --rpc "$RPC_URL" --gateway-credential "$GATEWAY_CREDENTIAL" wallet send \\
  --gateway-rpc "$GATEWAY_RPC_URL" --evm-key-file "$EVM_KEY_FILE" \\
  --to "$RECIPIENT_DID" --asset "$ASSET_ID" --amount 10 \\
  --wait committed --receipt-policy "$RECEIPT_POLICY" \\
  --fee-limit "$FEE_LIMIT"`}</code></pre>
      <p>Choose <code>submitted</code> for an accepted transaction hash, <code>receipted</code> for a verified signed execution receipt, or <code>committed</code> for receipt and committed-block evidence. Only <code>committed</code> establishes finality. The amount is an integer in the Asset&apos;s base units.</p>
      <h2>Recover a pending native payment</h2>
      <p>Each new send creates a new payment idempotency key. The CLI journals the signed transaction before submission. Retain its transaction hash and use the journal-backed recovery flow when the response is uncertain. <code>wallet receipt</code> takes a transaction hash, with <code>--wait</code> and <code>--receipt-policy</code> as its options.</p>
      <pre><code>{`layerx --rpc "$RPC_URL" --gateway-credential "$GATEWAY_CREDENTIAL" wallet receipt \\
  "$TX_HASH" --receipt-policy "$RECEIPT_POLICY" --wait committed`}</code></pre>
      <p><code>--timeout-seconds</code> (1 to 300, default 60) belongs to <code>wallet watch</code> and to write commands such as <code>wallet send</code>, not to <code>wallet receipt</code>. Use the recovered receipt and block evidence to establish the payment outcome.</p>
      <h2>Node and hosted JSON-RPC</h2>
      <p>The hosted gateway publishes <code>px_*</code> methods — including <code>px_register</code>, <code>px_getAccount</code>, <code>px_getBalances</code>, <code>px_listAssets</code>, and <code>px_requestFunds</code> — plus <code>pax_sequencerReceipt</code>. The node accepts signed transactions through <code>eth_sendRawTransaction</code> and reports receipt progress through <code>pax_sequencerReceipt</code>. The hosted gateway supplies account and asset reads at its separately configured origin. Use each endpoint for its declared methods and verify committed evidence before treating a payment as final.</p>
      <h2>Handle common failures</h2>
      <table><thead><tr><th>Code or condition</th><th>Response</th></tr></thead><tbody>
        <tr><td><code>idempotency-required</code></td><td>Supply the persistent purchase key before committing.</td></tr>
        <tr><td><code>idempotency-conflict</code></td><td>Resolve the original request; do not reuse its key for a different body.</td></tr>
        <tr><td><code>rate-limit</code></td><td>Respect the carried retry timing.</td></tr>
        <tr><td><code>unknown-outcome</code></td><td>Recover the original payment by key and evidence.</td></tr>
        <tr><td><code>budget-refusal</code></td><td>Inspect the funded budget and approved spending limits.</td></tr>
        <tr><td>Invalid signature, stale sequence, missing scope, or evidence mismatch</td><td>Resolve the specific refusal before preparing any new payment.</td></tr>
      </tbody></table>
      <h2>Next steps</h2>
      <ul><li><Link href="/payments">Payment lifecycle and merchant workflows</Link></li><li><Link href="/assets">Create an Asset and manage accounts</Link></li><li><Link href="/funding">Funding and money movement</Link></li><li><Link href="/paid-apis">Charge for HTTP resources</Link></li><li><Link href="/fees">Estimate fees and set limits</Link></li><li><Link href="/platform-api">Public payment API reference</Link></li></ul>
    </DocsLayout>
  )
}
