import Link from 'next/link'
import { DocsLayout } from '@/components/DocsLayout'
import { metadataFor } from '@/lib/seo'

export const metadata = metadataFor('/bridge')

export default function BridgePage() {
  return <DocsLayout>
    <div className="page-header"><h1 className="page-title">Bridging and balances</h1><p className="page-description">Move assets between registered external chains and Paxeer with attestor signatures, asset limits, and replay protection.</p></div>
    <h2>Choose the asset path</h2>
    <p>External-chain bridging moves value between a foreign vault and a Paxeer bank denomination. Paxeer mainnet-beta is one BFT chain with EVM chain ID 125; its kernel executes within the network. A successful Paxeer receipt is final when its block is committed. An external payout also requires the destination chain receipt.</p>
    <table><thead><tr><th>Boundary</th><th>Asset backing</th><th>Acceptance evidence</th></tr></thead><tbody>
      <tr><td>External chain ↔ Paxeer X</td><td>Foreign-chain vault or Solana custody program</td><td>Threshold attestor signatures, registered chain and asset, caps, unique source event</td></tr>
      <tr><td>Transfers within Paxeer</td><td>Native bank balances and derived sub-accounts</td><td>Successful transaction execution in a committed block</td></tr>
    </tbody></table>
    <h2>External-chain connectors</h2>
    <p>The external bridge uses <code>PaxeerXVault</code> on EVM chains and a custody program on Solana. Its Paxeer X interface is the <code>LayerXBridge</code> precompile at <code>0x0000000000000000000000000000000000001016</code>. Governance controls chain registration, vault identity, finality depth, attestor sets, per-asset limits, and pause state.</p>
    <p>Default bridge genesis contains no registered chains, attestors or asset caps. Governance registers and enables each chain, sets the attestor threshold, and registers asset limits before deposits can be accepted. The connector configurations are under contracts/bridge/evm/chains/ and contracts/bridge/solana/chains/solana/. Read the active on-chain configuration for the selected chain before depositing.</p>
    <table><thead><tr><th>Connector</th><th>Chain identifier</th><th>Native counterpart</th></tr></thead><tbody>
      <tr><td>Ethereum</td><td>1</td><td>ETH</td></tr><tr><td>Base</td><td>8453</td><td>ETH</td></tr><tr><td>Arbitrum</td><td>42161</td><td>ETH</td></tr><tr><td>Optimism</td><td>10</td><td>ETH</td></tr><tr><td>BNB Chain</td><td>56</td><td>BNB</td></tr><tr><td>Polygon</td><td>137</td><td>POL</td></tr><tr><td>Avalanche</td><td>43114</td><td>AVAX</td></tr><tr><td>HyperEVM</td><td>999</td><td>HYPE</td></tr><tr><td>Solana</td><td>91600046870081</td><td>SOL; also Sidiora (SID)</td></tr>
    </tbody></table>
    <p>Resolve the registered vault and asset configuration for the selected connector before depositing. Solana uses chain-specific encodings for its keys, mints, and transaction identities; the SID asset identifier is declared explicitly in its connector configuration. Do not treat a Solana chain identifier as an EVM RPC chain ID.</p>
    <h3>Deposit from an external chain</h3>
    <p>Each step below presumes the chain and asset have been registered by governance.</p>
    <ol><li>Choose the registered chain, asset, amount, and Paxeer EVM recipient encoded as a left-padded bytes32 value. Check pause state and the asset&apos;s per-transaction and in-flight limits.</li><li>Lock the asset in the configured foreign vault. EVM native-coin deposits use <code>depositNative(bytes32 paxeerRecipient)</code>.</li><li>The relayer observes the deposit after the connector&apos;s configured finality depth. Attestors sign the deposit digest binding chain, vault, transaction hash, log index, recipient, asset, and amount.</li><li>Submit <code>bridgeIn</code> with the event and threshold signatures. The precompile credits the bridged denomination and emits <code>BridgeIn</code>.</li><li>Retain the source event and Paxeer X receipt. The source tuple <code>(chain, txHash, logIndex)</code> is consumable once.</li></ol>
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
    <h2>Read bridge configuration</h2>
    <p>The bridge ABI is in precompiles/layerxbridge/LayerXBridge.sol. Its view methods expose the chain registration, attestor set, caps and replay state used by the keeper.</p>
    <ol><li>Read the registered vault, finality depth and enabled flag for the selected chain.</li><li>Read the current attestor identities and signature threshold.</li><li>Read the asset denomination, per-transaction cap and in-flight supply cap.</li><li>Check pause and source-event replay state before submitting a deposit.</li></ol>
    <h3>Signature acceptance</h3>
    <p>The inbound digest binds the remote chain, registered vault, source transaction hash, log index, Paxeer recipient, asset and amount. The recipient is a nonzero EVM address in the low 20 bytes of a bytes32 value, with its high 12 bytes zero.</p>
    <p>The signature format is specified in modules/layerxbridge/ATTESTATION.md. The inbound domain is PAXEERX_BRIDGE_IN_V1; the EVM outbound domain is PAXEERX_BRIDGE_OUT_V1. Attestors sign the raw Keccak-256 digest. Each accepted signature must recover a current attestor, and the recovered addresses must be strictly ascending.</p>
    <h2>Track an external withdrawal</h2>
    <p>A successful bridgeOut burns the caller&apos;s bridged denomination and advances the registered chain&apos;s outbound nonce. The foreign vault releases the corresponding asset after checking its outbound signatures, cap, pause state and replay protection.</p>
    <ol><li>Preserve the Paxeer transaction hash, chain, asset, recipient, amount and outbound nonce.</li><li>Obtain signatures for that exact transfer using the registered foreign custody configuration.</li><li>Submit the release and wait for the destination chain receipt.</li><li>Reconcile the Paxeer burn with the foreign release event before reporting payout.</li></ol>
    <table><thead><tr><th>Condition</th><th>Result</th></tr></thead><tbody><tr><td>Bridge is paused</td><td>Inbound and outbound transfers are refused</td></tr><tr><td>Chain is unregistered or disabled</td><td>Transfers for that chain are refused</td></tr><tr><td>Source event has already been credited</td><td>A second inbound credit is refused</td></tr><tr><td>Transfer exceeds its per-transaction cap</td><td>The transfer is refused</td></tr></tbody></table>
    <h2>Recovery and reconciliation</h2>
    <p>Reconcile a timed-out deposit against its original source event. The bridge exposes <code>isNullified</code> to check whether the inbound event has been consumed. For EVM withdrawals, the foreign vault consumes a nullifier derived from the Paxeer transaction hash and outbound nonce.</p>
    <p>Keep the foreign deposit, Paxeer credit, Paxeer burn and destination release as separate records. Inspect the existing transfer before retrying. Native account funding uses bank transfers within Paxeer; it does not require a separate custody deposit or checkpoint settlement step.</p>
    <h2>Continue building</h2><p>Read <Link href="/interop">interoperability</Link> for foreign protocol connectors, <Link href="/paxeer-vs-layerx">the unified architecture</Link> for execution and settlement responsibilities, and <Link href="/precompiles">precompiles</Link> for EVM integration.</p>
  </DocsLayout>
}
