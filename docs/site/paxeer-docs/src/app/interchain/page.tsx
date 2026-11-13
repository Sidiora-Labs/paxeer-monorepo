import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import Link from 'next/link'

export const metadata = metadataFor('/interchain')

export default function Interchain() {
  return (
    <DocsLayout>
      <div className="page-header">
        <h1 className="page-title">Interchain (IBC)</h1>
        <p className="page-description">Connect Paxeer X to other chains through IBC packets, native token transfers, and the attested bridge.</p>
      </div>

      <div className="source-note"><strong>Source:</strong> <code>Paxeer-X-Network/interchain/</code>, <code>precompiles/ibc/</code>, and the IBC keeper and router wiring in <code>node/app.go</code>.</div>
      <h2>Cross-chain routes in Paxeer X</h2>
      <p>Paxeer X supports IBC transfers through its Cosmos protocol stack and attested external-chain transfers through the <Link href="/bridge">native bridge</Link>. IBC authenticates packets with light clients, connections, channel state, and packet proofs. The bridge authenticates external deposits with a configured attestor threshold and enforces registered chains, asset caps, nullifiers, and pause controls.</p>
      <p>The bridge precompile at <code>0x0000000000000000000000000000000000001016</code> exposes <code>bridgeIn</code>, <code>bridgeOut</code>, chain and attestor queries, cap queries, replay status, and pause state. IBC transfers use <code>0x1009</code>. Choose the route supported by the destination and asset; an IBC channel identifier is separate from the bridge chain registry identifier.</p>
      <p>Record the source transaction, destination identifier, asset denomination, amount, and transfer status. Treat packet acknowledgement or timeout separately from bridge attestation and destination release. Continue with <Link href="/assets">assets</Link>, <Link href="/settlement">settlement</Link>, and the <Link href="/unified-network">unified network</Link>.</p>

      <h2>Protocol and implementation</h2>
      <p>The in-tree IBC implementation provides clients, connections, channels, packet commitments, and application routing. Relayers submit proofs and carry packets and acknowledgements between chains. A working transfer requires compatible live clients, an open channel, and relayer delivery.</p>
      <p>Query the target network for the port, channel, counterparty, and supported denomination before sending funds.</p>
      <table>
        <thead><tr><th>Source directory</th><th>Purpose</th></tr></thead>
        <tbody>
          <tr><td><code>interchain/modules/core/</code></td><td>Client, connection, channel, port, and commitment protocol logic.</td></tr>
          <tr><td><code>interchain/modules/apps/transfer/</code></td><td>ICS-20 fungible-token transfer messages, packets, and keeper.</td></tr>
          <tr><td><code>interchain/modules/apps/27-interchain-accounts/</code></td><td>Interchain-account implementation.</td></tr>
          <tr><td><code>interchain/modules/light-clients/</code></td><td>Tendermint (<code>07-tendermint</code>), solo-machine (<code>06-solomachine</code>), and localhost (<code>09-localhost</code>) light-client code.</td></tr>
          <tr><td><code>precompiles/ibc/</code></td><td>EVM interface to native ICS-20 transfers.</td></tr>
        </tbody>
      </table>

      <h2>Inspect clients and channels</h2>
      <p>Configure the CLI&apos;s Cosmos RPC endpoint for the Paxeer node being queried, then inspect its state:</p>
      <pre><code>{`paxd query ibc client states
paxd query ibc connection connections
paxd query ibc channel channels`}</code></pre>
      <p>Verify the channel&apos;s port, connection, remote chain, ordering, and open state. Channel identifiers such as <code>channel-0</code> are local to a chain; they are not universal destinations.</p>

      <h2>Transfer through the CLI</h2>
      <p>This command template sends <code>1,000,000 uhpx</code>, one PAX. Set <code>IBC_CHANNEL</code> to a verified channel and <code>IBC_RECIPIENT</code> to an address valid on its destination chain.</p>
      <pre><code>{`paxd tx ibc-transfer transfer \\
  transfer "$IBC_CHANNEL" "$IBC_RECIPIENT" 1000000uhpx \\
  --from mykey --chain-id hyperpax_125-1`}</code></pre>
      <p>Use the actual bank denomination for other assets. An IBC voucher has its own denomination trace; do not substitute a display ticker for a bank denomination.</p>

      <h2>Transfer directly from the EVM</h2>
      <p>The precompile at <code>0x0000000000000000000000000000000000001009</code> exposes two nonpayable transfer methods. Use the full <code>precompiles/ibc/abi.json</code> for client bindings.</p>
      <pre><code>{`interface IBC {
    function transfer(
        string calldata toAddress,
        string calldata port,
        string calldata channel,
        string calldata denom,
        uint256 amount,
        uint64 revisionNumber,
        uint64 revisionHeight,
        uint64 timeoutTimestamp,
        string calldata memo
    ) external returns (bool success);

    function transferWithDefaultTimeout(
        string calldata toAddress,
        string calldata port,
        string calldata channel,
        string calldata denom,
        uint256 amount,
        string calldata memo
    ) external returns (bool success);
}`}</code></pre>
      <p>The immediate EVM caller must have a Paxeer address association and sufficient native-denomination balance. A wrapper uses the wrapper&apos;s associated account. <code>amount</code> is a bank base-unit integer, not payable EVM value; delegatecall and staticcall are rejected.</p>
      <p>The explicit method takes an absolute remote timeout height and timestamp. IBC timestamps use nanoseconds. The default-timeout method derives its timeout from the channel&apos;s connection and client state: the height is the client&apos;s latest height plus the relative default <code>0-1000</code> (revision 0, height 1000), and the timestamp is ten minutes in nanoseconds beyond the later of the client consensus timestamp and the current block time. It can fail when required channel or client state is absent.</p>
      <p>The ABI submits transfers; it does not expose channel enumeration or a general incoming-packet callback. Use the native query interfaces to inspect IBC state.</p>

      <h2>Track the whole transfer</h2>
      <p>A successful local transaction means the source-chain transfer was accepted. It does not mean the destination has already received the assets. Retain the source transaction and packet identifiers, then track relay, acknowledgement, or timeout/refund handling on both chains.</p>
      <p>Configure your relayer for both networks, check that its clients are current, and monitor packet delivery and acknowledgement processing.</p>

      <h2>Implementation references</h2>
      <ul>
        <li><code>precompiles/ibc/IBC.sol</code>: exact public selectors.</li>
        <li><code>precompiles/ibc/ibc.go</code>: associated sender, amount conversion, timeout derivation, and keeper invocation.</li>
        <li><code>interchain/modules/apps/transfer/</code>: packet lifecycle and native transfer behavior.</li>
        <li><Link href="/precompiles">Precompile reference</Link> and <Link href="/rest-grpc">native query interfaces</Link>.</li>
      </ul>
      <div className="prev-next">
        <Link href="/sdk"><div className="prev-next-label">Previous</div><div className="prev-next-title">SDK</div></Link>
        <Link href="/admin-hpx"><div className="prev-next-label">Next</div><div className="prev-next-title">Admin &amp; HPX</div></Link>
      </div>
    </DocsLayout>
  )
}
