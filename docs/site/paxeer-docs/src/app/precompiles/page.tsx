import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import Link from 'next/link'

export const metadata = metadataFor('/precompiles')

export default function Precompiles() {
  return (
    <DocsLayout>
      <div className="page-header">
        <h1 className="page-title">Precompiles</h1>
        <p className="page-description">Call native Paxeer X capabilities from the EVM with the correct addresses, ABIs, permissions, and token units.</p>
      </div>

      <div className="source-note"><strong>Source:</strong> <code>precompiles/setup.go</code> and each precompile&apos;s <code>abi.json</code> and Go implementation.</div>
      <h2>Kernel and native application precompiles</h2>
      <p>Paxeer X extends the EVM with the LayerX kernel and native financial applications. The tables use shortened addresses; pad each address to 20 bytes for an EVM call.</p>
      <table><thead><tr><th>Address</th><th>Interface</th><th>Operations</th></tr></thead><tbody>
        <tr><td><code>0x1012</code></td><td>Disabled verification surface</td><td>Calls revert with <code>LXP_MODULE_DISABLED</code></td></tr>
        <tr><td><code>0x1013</code></td><td>Disabled custody surface</td><td>Calls revert with <code>LXP_MODULE_DISABLED</code></td></tr>
        <tr><td><code>0x1014</code></td><td>Anchor reads</td><td>Read-only queries use the available anchor store. Writes revert with <code>LXP_MODULE_DISABLED</code>. Network finality is block commit</td></tr>
        <tr><td><code>0x1015</code></td><td>Perpetuals</td><td><code>addCollateral</code>, <code>addSize</code>, <code>closePosition</code>, <code>executeOrder</code>, <code>cancelOrder</code>, account, balance, and market queries; every method is nonpayable. Use <code>precompiles/perps/abi/abi.json</code> and <code>modules/perps</code> for the served venue</td></tr>
        <tr><td><code>0x1016</code></td><td>Bridge</td><td><code>bridgeIn</code>, <code>bridgeOut</code>, <code>getChain</code>, <code>getAttestors</code>, <code>getCap</code>, <code>isNullified</code>, <code>isPaused</code></td></tr>
        <tr><td><code>0x1017</code></td><td>Launchpad</td><td>Market creation, quotes, buys and sells, reserves and price snapshots, fee strategies, fee claims, burns, airdrops, and LP rewards</td></tr>
        <tr><td><code>0x1018</code></td><td>Fee token</td><td><code>setFeeDenom</code>, <code>getFeeDenom</code>, <code>clearFeeDenom</code> select an approved fee denomination</td></tr>
        <tr><td><code>0x1019</code></td><td>XWeb</td><td><code>request</code>, <code>fulfil</code>, <code>fulfilProgram</code>, <code>refund</code>, <code>getRequest</code>, <code>getResult</code>, <code>getProgramResult</code>, <code>getKernelNetworkId</code>, attestors, threshold, fee, and parameters</td></tr>
        <tr><td><code>0x1020</code></td><td>Price feed</td><td><code>getPrice</code>, <code>getPriceBySymbol</code>, <code>getPrices</code>, market and observation queries, feed-key registration and revocation</td></tr>
        <tr><td><code>0x1021</code>–<code>0x1025</code></td><td>Spot (PECOR)</td><td>Market and swap rail (<code>0x1021</code>), limit orders (<code>0x1022</code>), stop orders (<code>0x1023</code>), read-only quoter (<code>0x1024</code>), and liquidity vault (<code>0x1025</code>); see the <Link href="/spot-exchange">spot exchange guide</Link></td></tr>
      </tbody></table>
      <h3>Kernel constructs and agent functions</h3>
      <p>The kernel constructs and agent functions are listed in <code>MergeAddressTable</code> in <code>precompiles/setup.go</code>. They are served through registered constructors and are gated by the merge activation height. Use the ABI for the address and queried height.</p>
      <table><thead><tr><th>Address</th><th>Interface</th></tr></thead><tbody>
        <tr><td><code>0x101a</code></td><td><code>layerxledger</code></td></tr>
        <tr><td><code>0x101b</code></td><td><code>layerxpay</code></td></tr>
        <tr><td><code>0x101c</code></td><td><code>layerxauth</code></td></tr>
        <tr><td><code>0x101d</code></td><td><code>layerxprograms</code></td></tr>
        <tr><td><code>0x101e</code></td><td><code>layerxchains</code></td></tr>
        <tr><td><code>0x0901</code>–<code>0x0908</code></td><td>Agent functions: <code>orob</code>, <code>clearing</code>, <code>oracle aggregator</code>, <code>pofq</code>, <code>scheduler</code>, <code>streams</code>, <code>tee attestor</code>, <code>eip712</code></td></tr>
      </tbody></table>
      <p>Use each precompile&apos;s ABI for exact tuple layouts, argument types, mutability, and events. Perpetuals and pricefeed ABIs are under <code>precompiles/perps/abi/</code> and <code>precompiles/pricefeed/abi/</code>; spot ABIs are under <code>precompiles/spot/abi/</code>. Solidity interfaces for the bank, address, JSON, WASM and XWeb surfaces are in <code>contracts/src/precompiles/</code>. Verify the operation result and its committed block before treating execution as final.</p>
      <h3>One account across execution surfaces</h3>
      <p>The address precompile also exposes <code>bindLayerX</code>, <code>unbindLayerX</code>, <code>layerXBindNonce</code>, <code>getLayerXDid</code>, <code>getEvmAddrByLayerX</code>, and <code>getUnifiedAccount</code>. The unified account returns EVM, Cosmos, DID public key, and LayerX main account identifiers. Binding requires the account signature and current nonce; applications retain each identifier in its native encoding.</p>
      <p>Continue with <Link href="/xdk">XDK</Link> for client bindings, <Link href="/settlement">settlement</Link> for receipt verification and block finality, <Link href="/bridge">bridge transfers</Link>, and <Link href="/assets">assets</Link> for denomination and pointer rules.</p>

      <h2>Fixed addresses and interfaces</h2>
      <p>Paxeer X implements precompiles natively in the node. Call them with their matching ABI rather than expecting deployed Solidity bytecode. Match your client bindings to the node revision you use.</p>
      <table>
        <thead><tr><th>Address</th><th>Capability</th><th>Current interface</th></tr></thead>
        <tbody>
          <tr><td><code>0x1001</code></td><td>Bank</td><td>Native balances, metadata, supply, pointer-authorized sends, native value transfers</td></tr>
          <tr><td><code>0x1002</code></td><td>CosmWasm</td><td><code>instantiate</code>, <code>execute</code>, <code>execute_batch</code>, <code>query</code></td></tr>
          <tr><td><code>0x1003</code></td><td>JSON</td><td>Extract bytes, byte lists, unsigned integers, and array elements</td></tr>
          <tr><td><code>0x1004</code></td><td>Address association</td><td><code>getPaxAddr</code>, <code>getEvmAddr</code>, <code>associate</code>, <code>associatePubKey</code></td></tr>
          <tr><td><code>0x1005</code></td><td>Staking</td><td>Delegation and validator transactions, queries, pagination, pool, and parameters</td></tr>
          <tr><td><code>0x1006</code></td><td>Governance</td><td><code>vote</code>, <code>voteWeighted</code>, <code>deposit</code>, <code>submitProposal</code></td></tr>
          <tr><td><code>0x1007</code></td><td>Distribution</td><td>Reward queries, withdrawals, withdrawal address, and validator commission</td></tr>
          <tr><td><code>0x1008</code></td><td>Oracle</td><td>Both price-query methods revert with an explicit disabled-query error</td></tr>
          <tr><td><code>0x1009</code></td><td>IBC transfer</td><td><code>transfer</code>, <code>transferWithDefaultTimeout</code></td></tr>
          <tr><td><code>0x100a</code></td><td>Pointer lookup</td><td>Native, CW20, CW721, and CW1155 pointer address, version, and existence</td></tr>
          <tr><td><code>0x100b</code></td><td>Pointer creation</td><td><code>addNativePointer</code>, <code>addCW20Pointer</code>, <code>addCW721Pointer</code>, <code>addCW1155Pointer</code></td></tr>
          <tr><td><code>0x100c</code></td><td>Solo asset claims</td><td><code>claim(bytes)</code>, <code>claimSpecific(bytes)</code> using signed Cosmos transaction payloads</td></tr>
          <tr><td><code>0x1011</code></td><td>P-256 signatures</td><td><code>verify(bytes)</code></td></tr>
        </tbody>
      </table>
      <p>Addresses in the table are shortened for readability. Pad them to 20 bytes, for example <code>0x0000000000000000000000000000000000001001</code>. <code>precompiles/common/</code> supplies shared gas, payment, ABI, and error handling; it is not another callable precompile.</p>

      <h2>Address association</h2>
      <p><code>getPaxAddr(address)</code> resolves an existing EVM-to-Paxeer association. <code>getEvmAddr(string)</code> performs the reverse lookup. Account addresses use <code>pax1</code>; validator operator addresses use <code>paxvaloper1</code>. These lookups can fail when no association exists; they are not interchangeable with a bech32 encoding of the same 20 bytes.</p>
      <p>The manual association methods derive both addresses from a public key or signature. Use the key belonging to the account being linked. Creating a new random wallet would associate a different account. Signature construction must match the exact message bytes expected by <code>precompiles/addr/addr.go</code>. Use <code>layerXBindNonce</code> when preparing the consent message for <code>bindLayerX</code>.</p>

      <h2>Bank balances and transfers</h2>
      <p>Bank amounts are integers in the supplied base denomination. The native bank denomination is <code>uhpx</code>: one PAX equals <code>1,000,000 uhpx</code>, while EVM transaction value uses <code>10^18</code> wei per PAX. The internal human-unit identifier is <code>hpx</code>; see <Link href="/network-parameters">network parameters</Link> for the unit mapping.</p>
      <pre><code>{`// Minimal read ABI; the complete interface is precompiles/bank/abi.json.
import { Contract, JsonRpcProvider, formatUnits } from 'ethers'

export async function readBankBalance(rpcUrl, account) {
const provider = new JsonRpcProvider(rpcUrl)
if ((await provider.getNetwork()).chainId !== 125n) throw new Error('Wrong chain')
const bank = new Contract(
  '0x0000000000000000000000000000000000001001',
  ['function balance(address,string) view returns (uint256)',
   'function decimals(string) view returns (uint8)'],
  provider,
)
const amount = await bank.balance(account, 'uhpx')
console.log('bank base units:', amount.toString())
console.log('PAX:', formatUnits(amount, 6))
console.log('bank denomination decimals:', await bank.decimals('uhpx')) // 0
}`}</code></pre>
      <p><code>bank.decimals(denom)</code> returns <code>0</code> because the ABI represents base-denomination integers. It does not return the display exponent of PAX. The <code>name</code> and <code>symbol</code> methods require bank denomination metadata.</p>
      <ul>
        <li><code>send(address from, address to, string denom, uint256 amount)</code> is nonpayable and restricted to the registered ERC-20 native pointer for that denomination. An arbitrary wrapper or wallet cannot call it successfully just by supplying a sender.</li>
        <li><code>sendNative(string toAddress)</code> is payable, sends EVM value to a Paxeer bech32 recipient, and preserves both whole <code>uhpx</code> and the smaller wei remainder. The caller must be associated; delegatecall is rejected.</li>
        <li><code>all_balances(address)</code> and <code>supply(string)</code> also expose base-denomination amounts.</li>
      </ul>

      <h2>Staking and distribution</h2>
      <p>Staking writes act for the immediate caller&apos;s associated account. A contract wrapper therefore controls its own position; querying its deployer&apos;s delegation does not inspect that position. Delegatecall is rejected.</p>
      <p>Payable staking methods receive EVM value and convert it into the native staking denomination; amount arguments and query results are denomination integers or fixed-point share and reward values whose exact scale is defined by each method&apos;s handler. Read the scale for a method in <code>precompiles/staking/</code> and <code>precompiles/distribution/</code> before converting a result for display, and keep delegation shares, pending rewards, and withdrawn amounts as separate quantities.</p>
      <p>The staking ABI also exposes validator creation and editing; validator, delegation, unbonding, and redelegation queries; historical information; <code>pool</code>; and <code>params</code>. Read current parameters for unbonding time and entry limits. Paginated queries return a bytes cursor: with ethers, an empty cursor is <code>0x</code>, whose JavaScript string length is two.</p>
      <p>Distribution supports <code>setWithdrawAddress</code>, <code>withdrawDelegationRewards</code>, <code>withdrawMultipleDelegationRewards</code>, <code>withdrawValidatorCommission</code>, and <code>rewards</code>. The withdrawal target must be associated. Some validator addresses are indexed strings in EVM events; an indexed string topic is a hash, not the original bech32 text.</p>

      <h2>Governance transactions</h2>
      <p>The governance ABI exposes writes only: <code>vote</code>, <code>voteWeighted</code>, <code>deposit</code>, and <code>submitProposal</code>. Use the native module query APIs to inspect proposals and live deposit or voting parameters. Take the exact argument and message shapes from <code>precompiles/gov/abi.json</code> and <code>precompiles/gov/handler.go</code> for the node revision you target, and confirm a submission through the native governance queries rather than assuming a particular EVM event.</p>

      <h2>JSON extraction</h2>
      <p>The JSON ABI provides <code>extractAsBytes(bytes,string)</code>, <code>extractAsBytesList(bytes,string)</code>, <code>extractAsUint256(bytes,string)</code>, and <code>extractAsBytesFromArray(bytes,uint16)</code>. Keys select a literal top-level field. To read a nested field, first extract its containing object.</p>
      <p>Byte-list results preserve each element&apos;s JSON representation, including quotes around strings. Decode returned hex bytes before parsing JSON. Integer extraction accepts decimal integers and quoted integer strings, rejects negative or oversized values, and does not implement floating-point conversion.</p>
      <pre><code>{`// With json connected to the current precompiles/json/abi.json:
const document = toUtf8Bytes(JSON.stringify({ user: { count: '42' }, tags: ['one'] }))
const user = await json.extractAsBytes(document, 'user')
const count = await json.extractAsUint256(user, 'count') // 42n
const tags = await json.extractAsBytesList(document, 'tags')
const firstTag = JSON.parse(toUtf8String(tags[0])) // 'one'`}</code></pre>
      <p>The helper functions <code>toUtf8Bytes</code> and <code>toUtf8String</code> above are ethers exports. The implementation charges 100 gas per ABI argument byte, excluding the selector; total transaction gas includes other execution and transaction costs.</p>

      <h2>P-256 signature verification</h2>
      <p>At <code>0x1011</code>, <code>verify(bytes)</code> takes exactly 160 payload bytes: a 32-byte digest followed by 32-byte <code>r</code>, <code>s</code>, public-key <code>x</code>, and public-key <code>y</code>. Valid signatures return ABI-encoded <code>bytes</code> containing a 32-byte word with value one. Invalid signatures return empty low-level call data rather than an ABI-encoded result; handle that case before decoding. Malformed payload length reverts.</p>
      <p>This is an ABI call with a dynamic bytes argument. The 160-byte payload occupies 224 argument bytes after its offset and length words, so the precompile charges <code>224 × 300 = 67,200</code> precompile execution gas, before other call or transaction costs. P-256 verification is one part of a passkey flow; challenge, origin, authenticator data, and replay handling belong to the complete application ceremony.</p>

      <h2>Cross-runtime and cross-chain calls</h2>
      <p>The <Link href="/wasm">CosmWasm guide</Link> covers matching the native coin list to payable EVM value. <Link href="/wasmbinding">WASM bindings</Link> expose calls in the other direction. The <Link href="/interchain">IBC guide</Link> covers the transfer ABI, which does not expose channel queries.</p>
      <p>Pointer lookup returns the pointer address, a version, and an existence flag for one supplied native denomination or CosmWasm address. It does not provide a list-all query. Solo claims consume signed Cosmos transaction payloads; they are asset-claim operations rather than a general administrative interface.</p>

      <h2>Oracle query behavior</h2>
      <p><code>getExchangeRates</code> and <code>getOracleTwaps</code> at <code>0x1008</code> revert with <code>oracle precompile is retired; oracle data queries are disabled</code>. See the <Link href="/modules/oracle">Oracle module guide</Link> for native module queries. The EVM pricefeed read interface is at <code>0x1020</code>.</p>

      <h2>ABI and failure handling</h2>
      <p>Use the matching <code>precompiles/&lt;name&gt;/abi.json</code>, simulate the real caller and value, and inspect revert data before sending a transaction. Module-backed precompiles can meter native execution dynamically; kernel constructs and the perpetuals venue use method gas tables. Use the gas policy of the specific precompile. An off-chain read needs no funded signer, while an on-chain call still consumes gas.</p>
      <p>In particular, address lookup is <code>getPaxAddr</code> in this tree. An SDK package targeting another chain can carry different selectors, chain IDs, prefixes, or addresses even when its API looks similar.</p>
      <div className="prev-next">
        <Link href="/evm"><div className="prev-next-label">Previous</div><div className="prev-next-title">EVM Module</div></Link>
        <Link href="/wasm"><div className="prev-next-label">Next</div><div className="prev-next-title">WASM</div></Link>
      </div>
    </DocsLayout>
  )
}
