import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import Link from 'next/link'

export const metadata = metadataFor('/accounts')

export default function Accounts() {
  return <DocsLayout>
    <div className="page-header"><h1 className="page-title">Accounts &amp; Unified Identity</h1><p className="page-description">An EVM address bound to a native identity on Paxeer mainnet-beta, with explicit asset accounts and financial subaccounts.</p></div>
    <p>Paxeer X presents a unified account while preserving distinct cryptographic roles on one network. An EVM account uses secp256k1. A LayerX identity uses Ed25519 and the key-derived identifier <code>did:layerx:&lt;64 hex characters of the public key&gt;</code>. An on-chain binding connects the two, and named LayerX accounts hold native assets, issued assets, escrow, budgets, streams, and margin.</p>
    <h2>One secret, two keys</h2>
    <p>A BIP-39 English mnemonic and optional passphrase produce a 64-byte seed. The same account index selects a matching EVM account and LayerX identity.</p>
    <table><thead><tr><th>Key</th><th>Derivation</th><th>Path</th></tr></thead><tbody>
      <tr><td>Paxeer EVM</td><td>BIP-32 secp256k1</td><td><code>m/44&apos;/60&apos;/0&apos;/0/i</code></td></tr>
      <tr><td>LayerX identity</td><td>SLIP-0010 Ed25519</td><td><code>m/44&apos;/19544&apos;/i&apos;/0&apos;</code></td></tr>
    </tbody></table>
    <p>The index is below 2³¹ and is identical on both paths. LayerX derivation is entirely hardened. Coin type <code>19544</code> is the fixed version-1 branch value <code>0x4c58</code>, the ASCII bytes LX. Rust, TypeScript, Python, and the CLI use shared derivation vectors.</p>
    <pre><code>{`# Read the mnemonic through standard input or a file.
layerx wallet derive --index 0 < phrase.txt
layerx wallet derive --mnemonic-file phrase.txt --passphrase-file pass.txt

# Bind the derived pair using the configured Paxeer RPC.
layerx wallet derive --bind --rpc <endpoint> < phrase.txt`}</code></pre>
    <p>Derivation prints the EVM address and DID. Private-key export requires the explicit <code>--export-private-keys &lt;file&gt;</code> option; it creates an owner-readable file and refuses to overwrite an existing file.</p>
    <h3>External-wallet derivation</h3>
    <p>A browser wallet that keeps its seed private can derive the LayerX key from a fixed EIP-712 signature. The domain is <code>Paxeer X Network</code>, version <code>1</code>, with the connected chain identifier. The primary type is <code>LayerXKeyDerivation</code>; the message binds purpose, warning, address, index, and version.</p>
    <pre><code>{`seed = HKDF-SHA256(
  ikm  = canonical signature r || s || v,
  salt = "paxeer-x-network/layerx-account-key/v1",
  info = "LX:ACCOUNT-KEY:v1" || chainId_u256_be
         || address_20_bytes || index_u32_be,
  L    = 32
)`}</code></pre>
    <p>The SDK normalizes the signature to low-s form with v equal to 27 or 28, verifies that it recovers the requested signing address, asks the wallet twice, and compares the canonical signatures. A mismatch refuses derivation with <code>wallet_signature_not_deterministic</code>. A signer mismatch refuses with <code>wallet_signer_mismatch</code>.</p>
    <p>The derivation signature is itself secret: anyone holding it can derive the LayerX key. The message names the authorized origin, selected by <code>ACCOUNT_DERIVATION_ORIGIN</code> in the TypeScript SDK. Keep the signature out of application logs and storage. A self-hosted origin changes the message and the resulting key, so the chosen origin remains stable for recovery.</p>
    <pre><code>{`import { deriveFromBrowserWallet, autoBindLayerX } from '@sidiora/layerx-sdk/browser'

const account = await deriveFromBrowserWallet(window.ethereum, {
  chainId: 125,
  address,
})
await autoBindLayerX(window.ethereum, account, 125)`}</code></pre>
    <h2>Binding both identities</h2>
    <p>The address precompile at <code>0x0000000000000000000000000000000000001004</code> records mutual consent.</p>
    <ol><li>Read <code>layerXBindNonce(address)</code> and <code>getUnifiedAccount(address)</code>.</li><li>If the derived DID is already bound, return the existing binding without sending another transaction.</li><li>If a different DID is bound, refuse with <code>bound_to_different_did</code>. Changing identity requires the owner’s deliberate <code>unbindLayerX</code>.</li><li>Sign the domain-separated <code>LX:PAXEER-BIND:v1</code> preimage containing chain identifier, EVM address, and nonce with the LayerX key.</li><li>Call <code>bindLayerX(didPublicKey, signature)</code> from the EVM address. The transaction supplies EVM consent and the Ed25519 signature supplies LayerX consent.</li></ol>
    <p>After binding, the hosted gateway method <code>px_resolveAccount</code> resolves the EVM address and DID to the same unified account. It is part of the gateway&apos;s <code>px_</code> API, defined by its OpenRPC contract. Obtain the gateway URL and credentials from the operator. The CLI bind flow requires EVM chain ID 125. Seed-based helpers construct a signed EIP-1559 transaction; browser-wallet binding uses <code>eth_sendTransaction</code> with the pending EVM nonce on that chain.</p>
    <h2>Named financial accounts</h2>
    <p>Native escrow, budget, margin, and stream commitments hold value in separate accounts. Each module authorizes transfers from its own accounts.</p>
    <table><thead><tr><th>Namespace</th><th>Role</th></tr></thead><tbody>
      <tr><td><code>agent:&lt;did&gt;:main</code></td><td>The identity’s native-asset account.</td></tr>
      <tr><td><code>agent:&lt;did&gt;:asset:&lt;asset_id&gt;</code></td><td>A separate account for an issued asset.</td></tr>
      <tr><td><code>agent:&lt;did&gt;:budget:&lt;id&gt;</code></td><td>Budget-controlled value.</td></tr>
      <tr><td><code>agent:&lt;did&gt;:escrow:&lt;id&gt;</code></td><td>Escrow-held value.</td></tr>
      <tr><td><code>agent:&lt;did&gt;:stream:&lt;id&gt;</code></td><td>Stream-controlled value.</td></tr>
      <tr><td><code>agent:&lt;did&gt;:margin:&lt;position&gt;</code></td><td>Position margin.</td></tr>
      <tr><td><code>system:liquidity:&lt;market&gt;</code></td><td>Market liquidity.</td></tr>
      <tr><td><code>system:funding:&lt;market&gt;:long|short</code></td><td>Protocol-3 market funding accounts.</td></tr>
      <tr><td><code>system:insurance</code>, <code>system:fees</code></td><td>Insurance pool and fee treasury.</td></tr>
      <tr><td><code>system:paxeer-reserve</code>, <code>system:paxeer-withdrawals</code></td><td>System reserve and withdrawal identifiers recognized by the account namespace decoder.</td></tr>
      <tr><td><code>module:asset:value:&lt;issuance-account-id&gt;</code></td><td>Asset issuance account records.</td></tr>
    </tbody></table>
    <h3>Canonical identifiers</h3>
    <pre><code>{`account_id32 = SHA-256(
  "LX:ACCOUNT:v1" || u32_be(name_length) || name_bytes
)`}</code></pre>
    <p>The hash above derives ordinary named accounts. A module value name ends in its existing 32-byte account ID as 64 lowercase hex characters, which is decoded directly. Hexadecimal identifiers in names use lowercase text; asset and protocol-3 object identifiers use 64 hex characters. Signed Escrow OPEN, Budget CREATE, and Stream OPEN derive subaccounts from the signed actor DID and object identifier. Signed Perps MARKET_CREATE derives market liquidity and funding accounts. System accounts are created by genesis or authorized governance.</p>
    <h2>Authority and sequence ownership</h2>
    <p>An owner’s ordinary signature or session key cannot directly debit a budget, escrow, margin, or stream subaccount. The owning escrow authority, budget allowance, or protocol-module capability authorizes release. This makes the account boundary enforce the financial commitment.</p>
    <table><thead><tr><th>Counter</th><th>Scope</th></tr></thead><tbody>
      <tr><td>Activity <code>account_sequence</code></td><td>The actor DID’s identity counter.</td></tr>
      <tr><td>Transfer-set <code>actor_sequence</code></td><td>The designated sequence account, normally the debited record and therefore per DID and asset.</td></tr>
      <tr><td>Send, mint, and burn</td><td>Send advances the source account counter; mint uses the issuance account counter; burn uses the source asset account counter.</td></tr>
    </tbody></table>
    <p>An EVM transaction also carries its own sender nonce, read with eth_getTransactionCount at pending. A new per-asset account starts at zero independently of the main account’s counter. Account identifiers are protocol namespace values, not arbitrary application strings. Agent SDK type boundaries reject identifiers outside the supported namespace.</p>
    <h2>Recovery and supported wallet paths</h2>
    <p>The same phrase, passphrase, and index restore the same key pair. External-wallet recovery requires the same account, chain, index, origin, and deterministic signing message; the existing binding returns <code>already_bound</code>. Wallets with nondeterministic signatures cannot use signature derivation. Smart-contract accounts whose signatures do not recover to the account address are refused on this path.</p>
    <p>An existing independently generated Ed25519 identity remains valid and can bind manually, though it cannot be re-derived from the EVM secret. Mnemonic derivation accepts the English BIP-39 list. Rotating a derived LayerX key breaks the original derivation link; selecting the next account index preserves a reproducible new pair.</p>
    <h2>Continue reading</h2><ul><li><Link href="/identity">Identity, grants, and recovery</Link></li><li><Link href="/assets">Issued assets</Link></li><li><Link href="/activities">Activity sequences</Link></li><li><Link href="/precompiles">Paxeer precompiles</Link></li><li><Link href="/escrow">Escrow accounts</Link></li><li><Link href="/agent-budgets">Budget accounts</Link></li></ul>
  </DocsLayout>
}
