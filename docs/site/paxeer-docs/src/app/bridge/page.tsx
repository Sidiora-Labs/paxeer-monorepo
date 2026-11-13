import Link from 'next/link'
import { DocsLayout } from '@/components/DocsLayout'
import { metadataFor } from '@/lib/seo'

export const metadata = metadataFor('/bridge')

export default function BridgePage() {
  return <DocsLayout>
    <div className="page-header"><h1 className="page-title">Bridging and custody</h1><p className="page-description">Move assets between external chains, Paxeer X settlement, and LayerX execution with explicit proofs, replay protection, and withdrawal guarantees.</p></div>
    <h2>Choose the asset path</h2>
    <p>Paxeer X connects two distinct asset boundaries. External-chain bridging moves value between a foreign vault and a Paxeer X representation. The LayerX custody bridge moves value between Paxeer X custody and the LayerX execution ledger. An application can compose these paths while retaining a separate identifier and acceptance proof for each step.</p>
    <table><thead><tr><th>Boundary</th><th>Asset backing</th><th>Acceptance evidence</th></tr></thead><tbody>
      <tr><td>External chain ↔ Paxeer X</td><td>Foreign-chain vault or Solana custody program</td><td>Threshold attestor signatures, registered chain and asset, caps, unique source event</td></tr>
      <tr><td>Paxeer X ↔ LayerX</td><td>Paxeer X custody with a LayerX reserve mirror</td><td>Tendermint light-client proof for deposits; finalised checkpoint and membership evidence for withdrawals</td></tr>
    </tbody></table>
    <h2>External-chain connectors</h2>
    <p>The external bridge uses <code>PaxeerXVault</code> on EVM chains and a custody program on Solana. Its Paxeer X interface is the <code>LayerXBridge</code> precompile at <code>0x0000000000000000000000000000000000001016</code>. Governance controls chain registration, vault identity, finality depth, attestor sets, per-asset limits, and pause state.</p>
    <p>The bridge ships dormant. Its default genesis registers no chains, sets no attestors, and sets no caps; only a governance registration brings a chain up. The nine connectors below are the chains the deployment tooling is built for, not chains a deposit can use today. The chain configurations committed with that tooling also carry placeholder owner, attestor, and program-id values by design, and the deployability gate refuses them until the owner supplies real values. Confirm the chain, asset, attestor set, and caps are registered on-chain before treating any connector as a live deposit path.</p>
    <table><thead><tr><th>Connector</th><th>Chain identifier</th><th>Native counterpart</th></tr></thead><tbody>
      <tr><td>Ethereum</td><td>1</td><td>ETH</td></tr><tr><td>Base</td><td>8453</td><td>ETH</td></tr><tr><td>Arbitrum</td><td>42161</td><td>ETH</td></tr><tr><td>Optimism</td><td>10</td><td>ETH</td></tr><tr><td>BNB Chain</td><td>56</td><td>BNB</td></tr><tr><td>Polygon</td><td>137</td><td>POL</td></tr><tr><td>Avalanche</td><td>43114</td><td>AVAX</td></tr><tr><td>HyperEVM</td><td>999</td><td>HYPE</td></tr><tr><td>Solana</td><td>91600046870081</td><td>SOL; also Sidiora (SID)</td></tr>
    </tbody></table>
    <p>Resolve the registered vault and asset configuration for the selected connector before depositing. Solana uses the bridge&apos;s mapped handles for its keys, mints, and transaction identities. Do not treat a Solana chain identifier as an EVM RPC chain ID.</p>
    <h3>Deposit from an external chain</h3>
    <p>Each step below presumes the chain and asset have been registered by governance.</p>
    <ol><li>Choose the registered chain, asset, amount, and Paxeer X recipient. Check pause state and the asset&apos;s per-transaction and in-flight limits.</li><li>Lock the asset in the configured foreign vault. EVM native-coin deposits use <code>depositNative(bytes32 paxeerRecipient)</code>.</li><li>The relayer observes the deposit after the connector&apos;s configured finality depth. Attestors sign the deposit digest binding chain, vault, transaction hash, log index, recipient, asset, and amount.</li><li>Submit <code>bridgeIn</code> with the event and threshold signatures. The precompile credits the bridged denomination and emits <code>BridgeIn</code>.</li><li>Retain the source event and Paxeer X receipt. The source tuple <code>(chain, txHash, logIndex)</code> is consumable once.</li></ol>
    <h3>Withdraw to an external chain</h3>
    <p><code>bridgeOut(chain, asset, amount, recipient)</code> burns the caller&apos;s bridged denomination and returns a nonce. The <code>BridgeOut</code> event identifies the outbound transfer. Attestors sign the foreign vault&apos;s release digest, and the relayer submits the release to the destination custody contract. Track both the Paxeer X burn and foreign release; a burn receipt alone is not the destination payout receipt.</p>
    <h3>Precompile reference</h3>
    <pre><code>{`interface ILayerXBridge {
  function bridgeIn(uint64 chain, address vault, bytes32 txHash,
    uint64 logIndex, bytes32 recipient, address asset,
    uint256 amount, bytes[] calldata signatures)
    external returns (string memory denom);
  function bridgeOut(uint64 chain, address asset, uint256 amount,
    address recipient) external returns (uint64 nonce);
}`}</code></pre>
    <p>Use <code>getChain</code>, <code>getAttestors</code>, <code>getCap</code>, <code>isPaused</code>, and <code>isNullified</code> for preflight and reconciliation. Inbound signatures are 65-byte <code>r || s || v</code> signatures over the raw bridge digest, with low s, v equal to 27 or 28, and strictly ascending signer addresses. Signing a prefixed personal message produces different evidence.</p>
    <h2>Deposit into LayerX execution</h2>
    <p>Paxeer X holds custody while LayerX maintains the reserve mirror. A successful deposit credit moves value from the reserve to the beneficiary on the LayerX ledger.</p>
    <ol><li>Create the custody deposit with the intended asset, beneficiary, owner key, amount, and nonce.</li><li>Obtain a proof of the custody <code>Deposit</code> record against a trusted Paxeer X header.</li><li>Build the <code>LXDC3</code> credit head and attach its <code>LXLB1</code> light-client bundle.</li><li>Submit the signed credit activity and verify its LayerX receipt. Keep the deposit identifier for idempotent reconciliation.</li></ol>
    <h3>Proof acceptance</h3>
    <p>The credit head is 363 bytes. <code>LXDC3</code> occupies its first five bytes, the SHA-256 of the attached proof bundle sits at bytes 327–359, and the <code>LXLB1</code> bundle begins at byte 363. The head also binds the network, protocol version, deposit identifier, asset, beneficiary, owner key, depositor, amount, and nonce, and verification checks the Tendermint proof in the bundle.</p>
    <p>The proof binds state height H to signed header H+1, its application root, and validator hash. These fields describe consensus state evidence. This custody path proves the deposit record directly; it does not accept an external bridge attestor signature as a substitute. Reused deposit identifiers, mismatched assets, incorrect beneficiaries, invalid signatures, and non-final proofs are refused.</p>
    <h2>Withdraw from LayerX custody</h2>
    <p>A withdrawal request debits the LayerX account into the withdrawals holding account before payout and binds the asset recorded at request time. Finalisation requires a finalised checkpoint, guarantor certificate, membership proof, and a closed challenge window.</p>
    <ol><li>Submit the withdrawal request and preserve its receipt and requested destination.</li><li>Obtain checkpoint and membership evidence for the request.</li><li>Wait for checkpoint finality and challenge-window closure.</li><li>Finalise on Paxeer X and reconcile the payout with the request&apos;s nullifier.</li></ol>
    <table><thead><tr><th>Condition</th><th>Result</th></tr></thead><tbody><tr><td>Challenge window remains open</td><td>Finalisation is refused until the window closes</td></tr><tr><td>Claim is cancelled</td><td>Finalisation is refused</td></tr><tr><td>Nullifier has already settled</td><td>A second payout is refused</td></tr><tr><td>Caller supplies a different asset</td><td>Request is refused without a transfer</td></tr></tbody></table>
    <h2>Emergency exit and reconciliation</h2>
    <p>When the sequencer is unavailable or its liveness bound is exceeded, emergency exit uses the last finalised checkpoint, a balance proof, and recorded guarantor signatures. The library exposes <code>lxp_exit_eligibility</code> for the eligibility decision. Emergency exit is a dedicated recovery path rather than another bridge activity.</p>
    <p>Keep source evidence, admission acknowledgement, execution receipt, checkpoint evidence, and payout receipt as distinct records. A retry first looks up the deposit identifier, operation receipt, or withdrawal nullifier; it does not create a fresh claim for the same economic transfer.</p>
    <h2>Continue building</h2><p>Read <Link href="/interop">interoperability</Link> for foreign protocol connectors, <Link href="/paxeer-vs-layerx">the unified architecture</Link> for execution and settlement responsibilities, and <Link href="/precompiles">precompiles</Link> for EVM integration.</p>
  </DocsLayout>
}
