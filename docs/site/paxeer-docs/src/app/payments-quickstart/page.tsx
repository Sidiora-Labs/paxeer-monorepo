import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import Link from 'next/link'

export const metadata = metadataFor('/payments-quickstart')

export default function PaymentsQuickstart() {
  return (
    <DocsLayout>
      <div className="page-header">
        <h1 className="page-title">Your first payment</h1>
        <p className="page-description">Quote, commit, and verify a Paxeer X payment using the application SDK or a native Asset transfer.</p>
      </div>
      <h2>Choose your integration</h2>
      <p>Use the application SDK when you want account-level payment journeys and bearer-token authentication. Use the native CLI when you control a signing identity and need explicit canonical activities, receipt policy, fee limits, and commitment selection. The LayerX package and command names are the integration interfaces of Paxeer X.</p>
      <h2>1. Install the application packages</h2>
      <pre><code>{`npm install @sidiora/layerx-sdk @sidiora/layerx-buyer-middleware`}</code></pre>
      <p>Configure <code>LAYERX_API_URL</code> with your environment&apos;s API base URL and <code>LAYERX_API_TOKEN</code> with its account bearer token. Use HTTPS outside a loopback emulator. These credentials identify your API account; do not embed them in browser bundles or logs.</p>
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
      <p>The CLI uses your local key, a stored gateway credential, and a receipt policy supplied independently of the RPC response you verify. A policy binds protocol version, network, sequencer identity and key, permitted batches, and checkpoint context. Keep private seeds out of shell history.</p>
      <pre><code>{`cargo build --manifest-path tools/platform/cli/Cargo.toml
export PATH="$PWD/tools/platform/target/debug:$PATH"
layerx key create alice
layerx key default alice
layerx wallet list

export RPC_URL=https://api-mainnet-beta.paxeer.network/
export RECEIPT_POLICY=/absolute/path/to/receipt-policy.json
export EVM_KEY_FILE=/absolute/path/to/evm-key-export
export FEE_LIMIT='<maximum fee in base units>'
export ASSET_ID='<64-hex Asset id>'
export RECIPIENT_DID='<recipient DID>'`}</code></pre>
      <p>Public JSON-RPC endpoints are HTTPS URLs at the root path. The old <code>/rpc</code> and <code>/rpc/ws</code> routes are retired. Note the current checkout still has CLI transport gaps: setting <code>--rpc</code> to a correct public root URL does not by itself make wallet commands work against a public node. Treat the native commands below as the command surface and verify connectivity in your environment before relying on them.</p>
      <p>The OS keyring stores keys by default; headless environments use the configured encrypted file store. The payer must already hold a spendable balance on the merged network before sending. Do not fund through the former custody precompile path: the custody module is retired, its keeper refuses deposits, and the precompile reverts. Confirm the account balance before spending.</p>
      <h2>Open the recipient account and send</h2>
      <p>Select the recipient wallet to open its account for the chosen Asset, then select the funded payer wallet to transfer. Identity sequence and source-account sequence are obtained separately. The signing flow discloses the debit authorization and outer activity, including recipient, amount, Asset, fee ceiling, expiry, network, and idempotency key.</p>
      <pre><code>{`# Run with the recipient wallet selected.
# open-account requires --evm-key-file, --receipt-policy, and --fee-limit.
layerx --rpc "$RPC_URL" --gateway-credential beta wallet open-account \\
  --asset "$ASSET_ID" --evm-key-file "$EVM_KEY_FILE" \\
  --receipt-policy "$RECEIPT_POLICY" --fee-limit "$FEE_LIMIT"

# Select the funded payer before sending.
layerx key default alice
layerx --rpc "$RPC_URL" --gateway-credential beta wallet send \\
  --to "$RECIPIENT_DID" --asset "$ASSET_ID" --amount 10 \\
  --wait executed --receipt-policy "$RECEIPT_POLICY" \\
  --fee-limit "$FEE_LIMIT"`}</code></pre>
      <p>Choose <code>executed</code> for a verified execution receipt, <code>batched</code> for authenticated batch inclusion, or <code>finalised</code> for matching finalised checkpoint evidence. The exact wire spelling is <code>finalised</code>.</p>
      <h2>Recover a pending native payment</h2>
      <p>Each new CLI write creates a new idempotency key. If the first attempt times out, save its transaction hash and recover the original receipt instead of repeating <code>wallet send</code>. <code>wallet receipt</code> takes a transaction hash, with <code>--wait</code> and <code>--receipt-policy</code> as its options.</p>
      <pre><code>{`layerx --rpc "$RPC_URL" --gateway-credential beta wallet receipt \\
  "$TX_HASH" --receipt-policy "$RECEIPT_POLICY" --wait finalised`}</code></pre>
      <p><code>--timeout-seconds</code> (1 to 300, default 60) belongs to <code>wallet watch</code> and to write commands such as <code>wallet send</code>, not to <code>wallet receipt</code>. Stream notifications are wake-ups: reconcile them with verified reads or a receipt verification.</p>
      <h2>Public JSON-RPC surface</h2>
      <p>The hosted gateway publishes <code>px_*</code> methods — including <code>px_register</code>, <code>px_getAccount</code>, <code>px_getBalances</code>, <code>px_listAssets</code>, and <code>px_requestFunds</code> — plus <code>pax_sequencerReceipt</code>. Every legacy <code>lx_*</code> method name is refused wholesale with JSON-RPC <code>-32601</code> (<code>Method not found</code>). Do not build submission or recovery flows on <code>lx_sendActivity</code>, <code>lx_getActivityStatus</code>, <code>lx_getReceipt</code>, or <code>lx_getProof</code>; no such public transport exists in this checkout. Use the SDK, the CLI, or the published <code>px_*</code> and node namespaces instead, and verify recovered evidence before treating a payment as settled.</p>
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
