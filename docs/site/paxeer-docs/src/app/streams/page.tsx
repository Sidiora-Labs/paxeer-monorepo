import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import Link from 'next/link'

export const metadata = metadataFor('/streams')

export default function StreamsPage() {
  return (
    <DocsLayout>
      <div className="page-header">
        <h1 className="page-title">Streaming Payments</h1>
        <p className="page-description">Pay for elapsed time or verified usage with funded, capped streams, deterministic accrual, settlement, and cancellation.</p>
      </div>
      <h2>Continuous Commerce on Paxeer X</h2>
      <p>Streams turn ongoing work into incremental payment obligations. Time streams support service retainers, compute reservations, and recurring agent work. Metered streams pay for signed usage readings such as completed requests or consumed resources. Both use an isolated <code>{'agent:<did>:stream:<id>'}</code> account and settle through <code>402LXP</code> transfer legs.</p>
      <p>Accrual and settlement are separate: accrual calculates how much the recipient has earned, while settlement transfers available funds. A stream has a lifetime cap and can be topped up, paused, resumed, or closed by its payer.</p>
      <h2>Choose a Payment Mode</h2>
      <table><thead><tr><th>Mode</th><th>Accrual input</th><th>Typical use</th></tr></thead><tbody>
        <tr><td>Time (<code>1</code>)</td><td>Elapsed batch time in milliseconds, bounded by the stream end time.</td><td>Agent availability, access, subscriptions, reserved compute.</td></tr>
        <tr><td>Metered (<code>2</code>)</td><td>Increase in a signed cumulative meter reading.</td><td>API calls, work units, bandwidth, resource consumption.</td></tr>
      </tbody></table>
      <h2>Opening and Funding a Stream</h2>
      <p>Set the stream ID, dedicated account, recipient, asset, mode, rate, rate unit, start and end timestamps, total cap, and positive initial funding. Amounts and rates use integer asset base units. The payer supplies authorized funds; the stream account must differ from the recipient account.</p>
      <p>For time mode, <code>rate_unit</code> is a time interval in milliseconds. For metered mode, it is a quantity of meter units. A metered stream registers between one and eight authority keys in strictly increasing order. A time stream has no meter authorities.</p>
      <h3>Accrual Formula</h3>
      <pre><code>{`new_amount = floor((rate × delta + remainder_carry) / rate_unit)
new_amount = min(new_amount, total_cap - accrued_total)
claimable  = accrued_total - settled_total
payment    = min(claimable, stream_account_balance)`}</code></pre>
      <p>The remainder carries across accrual calculations so frequent settlement does not repeatedly discard fractional base units. Batch timestamps provide a shared clock; regressing timestamps fail. Reaching the cap stops additional accrual.</p>
      <h2>Time Stream Example</h2>
      <ol>
        <li>Fund a stream with 600 base units and set a lifetime cap of 600.</li>
        <li>Set <code>rate = 10</code> and <code>rate_unit = 60000</code>, paying 10 base units per minute.</li>
        <li>After 15 active minutes, 150 units have accrued. Settle to pay those 150 to the named recipient.</li>
        <li>Pause for five minutes, then resume. The paused interval earns no additional time payment.</li>
        <li>Close after another 10 active minutes. With sufficient funding, closing pays the remaining 100 accrued units and refunds the unused 350 to the payer.</li>
      </ol>
      <h2>Metered Stream Workflow</h2>
      <p>A meter authority signs the stream ID, cumulative reading, and authority key under the <code>LXP:STREAM:METER:v1</code> message tag. The stream verifies the Ed25519 signature and checks that the key belongs to its registered authority set.</p>
      <p>For a rate of 20 base units per 100 usage units, a reading increase from 1,000 to 1,250 accrues 50 base units. Readings cannot move backward. Repeating a reading produces no new accrual; multiple authority keys attest to the same cumulative meter rather than maintaining independent counters.</p>
      <p>Submit authenticated readings, then settle the accrued amount. Meter readings accepted while paused or underfunded advance the meter baseline without accruing payment, preventing that recorded usage from being charged retroactively when payment resumes.</p>
      <h2>Funding, Claims, and Cancellation</h2>
      <table><thead><tr><th>Action</th><th>Behavior</th></tr></thead><tbody>
        <tr><td>Top up</td><td>Add authorized funds to an existing open stream. Funding does not increase its lifetime cap.</td></tr>
        <tr><td>Settle / claim</td><td>Pay accrued, unsettled value to the fixed recipient, limited to the funded balance.</td></tr>
        <tr><td>Pause</td><td>Payer checkpoints time accrual and stops further earning while paused.</td></tr>
        <tr><td>Resume</td><td>Payer restarts from the current batch timestamp without charging paused time.</td></tr>
        <tr><td>Rate change</td><td>Payer settles accrued value at the existing rate, then replaces the rate for subsequent accrual under a fresh idempotency key.</td></tr>
        <tr><td>Close / cancel</td><td>Payer settles payable accrued value, refunds the excess balance, and permanently closes the stream.</td></tr>
      </tbody></table>
      <h3>Underfunding</h3>
      <p>If settlement cannot cover all accrued value, it pays the available balance, resets accrued value to the settled total, and marks the stream underfunded. This does not preserve an unpaid debt balance. Accrual stops while underfunded. A top-up clears underfunding and starts the next time-accrual interval at the top-up timestamp.</p>
      <div className="callout"><strong>Maintain funding for continuous service.</strong> Monitor the stream balance, earned amount, cap, pause state, and underfunding state together. A payment cap limits earned value; a funded balance determines what settlement can actually pay.</div>
      <h3>Closing a Stream</h3>
      <p>Closing a funded active time stream first accounts for eligible elapsed time. It pays the recipient up to the available balance and returns the rest to the payer in the same transfer set. Closing is final: the stream cannot resume or receive further top-ups.</p>
      <h2>Protocol Reference</h2>
      <p>Streams are module <code>4</code>, with payload version <code>1</code>. The version prefix is two bytes, <code>00 01</code>.</p>
      <table><thead><tr><th>Activity</th><th>Type</th><th>Payload bytes</th></tr></thead><tbody>
        <tr><td><code>LX_STREAM_OPEN</code></td><td><code>0x00040001</code></td><td>204 + 32 per authority</td></tr>
        <tr><td><code>LX_STREAM_TOP_UP</code></td><td><code>0x00040002</code></td><td>50</td></tr>
        <tr><td><code>LX_STREAM_METER</code></td><td><code>0x00040003</code></td><td>138</td></tr>
        <tr><td><code>LX_STREAM_SETTLE</code></td><td><code>0x00040004</code></td><td>66</td></tr>
        <tr><td><code>LX_STREAM_PAUSE</code></td><td><code>0x00040005</code></td><td>34</td></tr>
        <tr><td><code>LX_STREAM_RESUME</code></td><td><code>0x00040006</code></td><td>34</td></tr>
        <tr><td><code>LX_STREAM_CLOSE</code></td><td><code>0x00040007</code></td><td>66</td></tr>
        <tr><td><code>LX_STREAM_RATE_CHANGE</code></td><td><code>0x00040008</code></td><td>82</td></tr>
      </tbody></table>
      <p>The rate-change payload carries the stream ID, a positive new rate as an unsigned 128-bit integer, and a nonzero idempotency key. The operation settles payable accrued value before storing the new rate. It preserves the stream&apos;s cap and rate unit.</p>
      <h3>Retries and Validation</h3>
      <p>Settle and close include a nonzero idempotency key. Retry the same economic operation with its original key and retain the result receipt. A key cannot identify a different stream or operation. Reject malformed versions, trailing payload bytes, meter regressions, unauthorized meters, and operations against a closed stream.</p>
      <p>Funding, draws, and refunds use published transfer assets and follow asset pause and asset-match rules. The stream account only permits its protocol-authorized draw and refund paths. Payloads are defined in <code>include/layerx/lx_stream.h</code> and <code>modules/layerxpay/types/payload_stream.go</code>; execution is implemented in <code>src/modules/stream/</code>.</p>
      <h2>Related Guides</h2>
      <ul><li><Link href="/escrow">Escrow and milestone payments</Link></li><li><Link href="/modules">Protocol modules</Link></li><li><Link href="/sdk">SDK integration</Link></li></ul>
    </DocsLayout>
  )
}
