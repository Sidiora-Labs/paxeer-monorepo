import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import Link from 'next/link'

export const metadata = metadataFor('/governance')

export default function Governance() {
  return <DocsLayout>
    <h1>Governance</h1>
    <p className="page-description">Authorized module proposals, scoped pauses, and constrained proxy upgrades on Paxeer.</p>
    <p>Governance authorizes changes on one network. Chain proposals carry typed module messages, protocol activities carry their own authority, and Solidity proxy upgrades use their declared governance contract.</p>
    <h2>Governance activities</h2>
    <p>The kernel governance module processes identity and authority activities under the canonical activity envelope, ordering, fee, and receipt rules. Its implementation is <code>src/modules/governance/lxp_governance.c</code>.</p>
    <p>The governance activity catalogue includes identity registration, session authority, and delegated grants. The following identifiers are handled by the kernel:</p>
    <table><thead><tr><th>Activity</th><th>Purpose</th></tr></thead><tbody>
      <tr><td><code>0x00070001</code></td><td>Identity registration and onboarding.</td></tr>
      <tr><td><code>0x00070005</code></td><td>Session authority.</td></tr>
      <tr><td><code>0x00070008</code></td><td>Delegated grant.</td></tr>
    </tbody></table>
    <p>Use the declared encoding and authority for each activity. Governance proposal handling in <code>modules/layerxgov</code> is a separate typed chain interface. See <Link href="/identity">Identity and authority</Link> for account delegation.</p>
    <h2>Typed module proposals</h2>
    <p><code>modules/layerxgov/types/proposal.go</code> defines <code>LayerXProposal</code>, which contains a title, description, and packed messages. Validation requires supported messages whose authority is the governance module account. The handler in <code>modules/layerxgov/handler.go</code> dispatches every message in a cached context and commits the changes only when all handlers succeed.</p>
    <h2>Scoped pause controls</h2>
    <p><code>CheckPauseScope</code> enforces module pauses while allowing named exit operations. Its scopes cover payment, exchange, ledger, authorization, and agent operations. A paused operation outside its allowed exit set returns <code>ErrPausedScope</code>.</p>
    <p>Payment exits include escrow release and timeout, budget closure and defunding, stream pause and close, and service-offer withdrawal. Exchange exits include order cancellation, position closure, and margin withdrawal. Read the scope&apos;s exact method names before invoking an exit.</p>
    <h2>Proxy upgrade roles</h2>
    <table><thead><tr><th>Role</th><th>Authority</th></tr></thead><tbody><tr><td>Governance authority</td><td>Schedules the permitted proxy upgrade with sufficient delay.</td></tr><tr><td>Executor</td><td>Executes the exact scheduled operation within its readiness window.</td></tr><tr><td>Guardian</td><td>Cancels a scheduled operation before it becomes ready.</td></tr><tr><td>Proxy target</td><td>Receives the authorized upgradeToAndCall operation with zero value.</td></tr></tbody></table>
    <p><code>contracts/governance/SidioraProxyTimelock.sol</code> binds an immutable proxy, governance authority, executor, guardian, minimum delay, delay floor, and grace period. Its operation identifier commits to the chain, timelock, target, value, calldata hash, salt, and nonce. Execution requires the scheduled operation to be ready and within its grace period.</p>
    <h3>Constrained proxy upgrades</h3>
    <p>The Sidiora timelock permits only <code>upgradeToAndCall</code> on its configured proxy, requires zero call value and deployed implementation code, and checks canonical upgrade calldata. Its roles and delay settings are immutable.</p>
    <p>Compare the target, implementation, calldata, salt, nonce, and ready time before execution. Inspect the operation event and resulting proxy state afterwards. A failed target call reverts execution; a completed operation cannot execute again.</p>
    <h2>Continue</h2><p>Read <Link href="/guarantors">Validators and commitment</Link> for consensus evidence, <Link href="/security">Security and trust boundaries</Link> for verification responsibilities, and <Link href="/finality">Finality</Link> for block commitment.</p>
  </DocsLayout>
}
