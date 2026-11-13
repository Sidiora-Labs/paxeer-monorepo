import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import Link from 'next/link'
export const metadata = metadataFor('/cli')
export default function CliPage() {
  return <DocsLayout>
    <div className="page-header"><h1 className="page-title">Command-Line Workflows</h1><p className="page-description">Operate Paxeer X identities, native assets, payments, deterministic Programs and agent transports with the layerx CLI.</p></div>
    <h2>The Paxeer X developer CLI</h2>
    <p>The CLI source is tools/platform/cli/. The binary is named <code>layerx</code>. Use <code>layerx &lt;command&gt; --help</code> for the command's current flags. Global <code>--json</code> emits one structured object: success contains <code>ok</code>, <code>kind</code>, <code>message</code> and <code>data</code>; failure contains <code>ok: false</code> and <code>error.code</code> with <code>error.detail</code>. Use these fields in automation instead of parsing presentation text.</p>
    <h2>Bind an environment</h2>
    <pre><code>{`layerx environment list
layerx environment current
layerx environment use beta \
  --endpoint "<hosted-api-base-url>" \
  --network-id <native-network-id> \
  --sequencer-trust-anchor-file <trusted-anchor-file>`}</code></pre>
    <p>Profiles are <code>emulator</code>, <code>beta</code> and <code>production</code>. Configure a hosted API base URL, native network ID and receipt-signing trust anchor together when binding a new profile. This profile is separate from the node JSON-RPC root selected by --rpc. Reusing a configured profile needs only its name. Non-loopback HTTP endpoints are refused; use HTTPS. The native network ID is distinct from EVM chain ID 125.</p>
    <h3>Configuration and credential storage</h3>
    <p><code>LAYERX_CONFIG</code> selects a config file. Otherwise the CLI uses <code>$XDG_CONFIG_HOME/layerx/config.json</code> or <code>$HOME/.config/layerx/config.json</code>. Public metadata and endpoints live in config; keys, hosted tokens and gateway credentials live in credential storage. The OS keyring is the default.</p>
    <pre><code>{`# Headless Linux: select the encrypted file store before key creation.
export LAYERX_CREDENTIAL_STORE=file
read -r -s -p 'Credential passphrase: ' LAYERX_CREDENTIAL_PASSPHRASE
export LAYERX_CREDENTIAL_PASSPHRASE
layerx --json key create operator
layerx key default operator
layerx --json key list
# auth set reads the session token from standard input.
layerx auth set --environment beta
layerx --json auth status --environment beta`}</code></pre>
    <p>The file store requires a 12–16,384 byte passphrase and encrypts the vault beside the configuration using AES-256-GCM and PBKDF2-HMAC-SHA256. Preserve the vault and its passphrase securely. Store selection has no automatic fallback or migration. Seeds and tokens supplied to import commands are read from stdin and never belong in shell arguments or public configuration.</p>
    <h2>Register and inspect an identity</h2>
    <pre><code>{`layerx key show operator
layerx register --key operator --gateway-endpoint "<hosted-registration-url>"
layerx account get
layerx wallet list
layerx wallet balance --gateway-rpc "<hosted-json-rpc-url>"`}</code></pre>
    <p>Registration signs the tenant binding and accepts a response only when its tenant, subject and signer key match. An imported key establishes local custody; registration and funding are separate steps. <code>layerx faucet --key operator --service-endpoint &lt;hosted-faucet-url&gt;</code> requests an approved bounded beta allocation using the stored identity session.</p>
    <h2>Virtual chains</h2>
    <pre><code>{`layerx --rpc "$PAXEER_RPC_URL" chains list
layerx --rpc "$PAXEER_RPC_URL" chains show <id-or-name>
layerx --rpc "$PAXEER_RPC_URL" chains create --name <name> --tier <tier> --fee-share-bps <0-5000> --evm-key-file <path>`}</code></pre>
    <p><code>layerx chains</code> lists, inspects and creates virtual chains on the chain registry. Like wallet, token, program and <code>receipt get</code> commands it accepts the global <code>--rpc</code> and <code>--gateway-credential</code> flags; the node RPC URL must use the root path <code>/</code>. Set PAXEER_RPC_URL in your shell to that node URL for these examples. The tier is starter, growth or pro. Creation requires an EVM key file (for example from <code>wallet derive --export-private-keys</code>) or <code>LAYERX_EVM_KEY_FILE</code>.</p>
    <h2>Wallets and native assets</h2>
    <table><thead><tr><th>Command</th><th>Workflow</th></tr></thead><tbody>
      <tr><td><code>wallet create &lt;name&gt;</code>, <code>wallet import &lt;name&gt;</code></td><td>With the emulator transport, create a key, register its identity and open its main account (optional <code>--did</code>). Import stores a 32-byte hexadecimal seed from stdin without registering an account.</td></tr>
      <tr><td><code>wallet derive</code></td><td>Derive the EVM account and native identity from one BIP-39 phrase read from stdin or a file.</td></tr>
      <tr><td><code>wallet balance</code>, <code>wallet list</code></td><td>Inspect DID accounts through --gateway-rpc, or verify --proof-snapshot with --proof-policy; list local public wallet metadata.</td></tr>
      <tr><td><code>wallet open-account</code></td><td>Open a wallet's native account for a specified asset.</td></tr>
      <tr><td><code>wallet send</code>, <code>token transfer</code></td><td>Prepare native transfers with recipient, asset, amount and signing policy.</td></tr>
      <tr><td><code>token create</code></td><td>Register a native asset with symbol, name, decimals, salt and optional supply cap.</td></tr>
      <tr><td><code>token mint</code>, <code>token burn</code></td><td>Issue into an existing asset account or return units to issuance.</td></tr>
      <tr><td><code>token info &lt;asset_id&gt;</code>, <code>token list</code></td><td>Read metadata and the ascending native asset registry.</td></tr>
      <tr><td><code>wallet estimate-fee</code>, <code>wallet watch</code>, <code>wallet receipt</code></td><td>Estimate fees from unsigned EIP-1559 transaction bytes at pending (requires <code>--from</code>), receive a notification, and verify receipt evidence for a transaction hash at the submitted, receipted or committed level. Supply --receipt-policy for verified receipt evidence.</td></tr>
    </tbody></table>
    <p>Global <code>--rpc</code> selects the node root. Hosted px_ queries use an explicitly configured service URL through <code>--gateway-rpc</code>; <code>--gateway-credential</code> selects a stored gateway alias. Use independently trusted receipt policy, explicit fee limits and current sequences for monetary writes. Native asset amounts are integer units rather than floating-point display amounts.</p>
    <h2>Quote and commit a payment</h2>
    <pre><code>{`layerx --json payment test \
  --from <source-account> \
  --to <destination-account> \
  --currency <asset-id> \
  --amount 1000 \
  --idempotency-key treasury-payment-0001`}</code></pre>
    <p>This workflow requests a quote through <code>/v1/moves/quote</code> and commits through <code>/v1/moves</code>. Keep the key stable for retries. The payment command returns the quote and commit journey; independently verify receipt bytes before treating the outcome as settled.</p>
    <h2>Fetch and verify receipts</h2>
    <pre><code>{`layerx --json --rpc "$PAXEER_RPC_URL" receipt get <0x-transaction-hash>
layerx --json receipt verify \
  --receipt receipt.hex \
  --batch-id <batch-id> \
  --asset <asset-id> \
  --previous-state-root <previous-root> \
  --resulting-state-root <resulting-root> \
  --sequencer-public-key <trusted-public-key>`}</code></pre>
    <p><code>receipt get</code> fetches a sequencer receipt by its 0x-prefixed transaction hash. Supply the receipt file as hex text or raw bytes and obtain batch facts through an independently trusted source. Verification is local and requires no node or gateway. The command reports <code>verified: true</code> only after checking the canonical outcome against the supplied facts. To check committed transaction evidence, use wallet receipt with --wait committed and --receipt-policy; Paxeer finality comes from the block commit. A timeout or pending response retains the original transaction hash: look it up again rather than submitting a new payment.</p>
    <h2>Build and operate Programs</h2>
    <pre><code>{`layerx new treasury-program
layerx program build --manifest-path treasury-program/Cargo.toml
layerx --rpc "$PAXEER_RPC_URL" program discover <program-id>
layerx --rpc "$PAXEER_RPC_URL" program interface get <program-id>
layerx --rpc "$PAXEER_RPC_URL" program registry get <program-id>`}</code></pre>
    <p>Build compiles WASM and enforces deterministic runtime policy locally. Discovery reads the program registry through eth_call at pending and requires the program to be active. Interface reads check the canonical interface encoding and digest. These reads do not establish committed deployment evidence. Bindings generation checks interface digest and code hash and produces typed SDK and guest bindings; supply verified deployment evidence and trust history according to <code>layerx program bindings --help</code>.</p>
    <table><thead><tr><th>Command family</th><th>Operation</th></tr></thead><tbody>
      <tr><td><code>program deploy</code>, <code>program upgrade</code></td><td>Validate and submit artifacts with lifecycle authorization and receipt verification.</td></tr>
      <tr><td><code>program interface publish</code></td><td>Publish a canonical interface bound to program code.</td></tr>
      <tr><td><code>program simulate</code></td><td>Execute against current state with sealed non-commit evidence.</td></tr>
      <tr><td><code>program call</code></td><td>Sign and submit an EIP-1559 call to a deployed program; render a receipt-verified typed result.</td></tr>
      <tr><td><code>program wind-down route</code>, <code>deprecate</code>, <code>tombstone</code>, <code>exit</code></td><td>Route owned accounts, define an exit deadline, retire code and exit account positions.</td></tr>
      <tr><td><code>program registry mirror-source</code>, <code>program registry list</code></td><td>Publish a build plan and source archive to the configured mirror, or list deployment events. Mirroring returns the deployed code hash and source digest.</td></tr>
    </tbody></table>
    <p>Deploy, upgrade and wind-down writes require --receipt-policy and <code>--program-id</code>, a required <code>--evm-key-file</code>, and <code>--fee-limit</code> (default <code>0</code>). Interface publication takes the program ID positionally and also requires an artifact, interface, key file, receipt policy and idempotency key. The <code>program interface publish</code> command requires <code>--idempotency-key</code>. Calls also bind fuel, ABI, memory and capability requirements. Receipt verification binds the returned activity, sequencer, state root, ABI and terminal payload before rendering typed success. Simulation explicitly establishes <code>committed: false</code>.</p>
    <h2>Connect agent transports</h2>
    <pre><code>{`layerx install mcp --host cursor --daemon-binding <binding.json>
layerx mcp serve --daemon-binding <binding.json>
layerx install a2a --environment beta --key operator --source-account <64-hex-source-account> --asset <64-hex-asset-id>
layerx a2a start
layerx a2a status
layerx a2a stop`}</code></pre>
    <p>MCP installation reads the binding document produced by agent-daemon enrollment and serves its tool catalogue over stdin/stdout. The transport carries no signing seed or gateway credential. Supported host names are <code>layerx</code>, <code>claude-code</code>, <code>claude-desktop</code>, <code>cursor</code> and <code>vscode</code>. Use <code>--read-only</code> for a read-only MCP installation.</p>
    <p>A2A installation targets a hosted environment, provisions a scoped gateway credential in secure storage and serves the agent card and task interface on loopback. The default listener uses loopback port <code>9433</code>. Managed start, status and stop use the installed runtime configuration.</p>
    <h2>Local development and workspace operations</h2>
    <p><code>layerx emulator provision</code> creates sequencer seed and trust-anchor files; <code>layerx emulator up --sequencer-seed-file &lt;path&gt;</code> runs the local gateway around real protocol transitions. Bind the emulator profile to its advertised identity before use. <code>layerx workspace</code> opens the visual workspace on a TTY. Its <code>modules</code>, <code>doctor</code>, <code>install</code>, <code>build</code>, <code>test</code> and <code>all</code> commands support repeatable module operations; use dry-run and explicit module selection for automation.</p>
    <h2>Read failures before retrying</h2>
    <p>Check the JSON error code. Network or sequencer trust mismatches require correcting the profile. Credential failures require the selected store and session. Receipt verification and ABI mismatches require resolving the evidence or interface. Policy and capability refusals require appropriate authorization. Retry uncertain writes only with their original identity, and retain existing activity IDs for reconciliation.</p>
    <p>Continue with the <Link href="/platform-api">platform API reference</Link>, <Link href="/identity">identity model</Link> and <Link href="/wasm-runtime">Programs runtime</Link>.</p>
  </DocsLayout>
}
