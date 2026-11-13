import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import Link from 'next/link'

export const metadata = metadataFor('/governance')

export default function Governance() {
  return <DocsLayout>
    <h1>Governance</h1>
    <p className="page-description">Ordered authority changes, versioned parameters, staged rollouts, and controlled settlement upgrades across Paxeer X.</p>
    <p>Paxeer X governance operates across two boundaries: the Layer X activity log governs protocol execution, while Paxeer settlement contracts enforce their own authorized roles and delayed calls. A governance decision is evidence with an actor, scope, ordering position, and activation condition.</p>
    <h2>Governance in the activity log</h2>
    <p>Module 7 processes governance activities through the same envelope, authority, ordering, fee, and receipt rules as other activities. Identity creation, key rotation, recovery changes, session grants, revocation, and delegated capability issuance therefore remain part of replayable history.</p>
    <p>Only the following Module-7 ordinals were verified in monorepo source (<code>modules/lxphost</code> tests). Meanings for other ordinals (<code>0x00070002</code>, <code>0x00070003</code>, <code>0x00070006</code>, <code>0x00070009</code>) were not verified and are not listed.</p>
    <table><thead><tr><th>Activity</th><th>Purpose (as used in source)</th></tr></thead><tbody>
      <tr><td><code>0x00070001</code></td><td>Register / create-or-onboard identity (<code>registerType</code>).</td></tr>
      <tr><td><code>0x00070005</code></td><td>Session-authority activity (used with fee-record loading in host tests).</td></tr>
      <tr><td><code>0x00070008</code></td><td>Grant (<code>grantType</code>).</td></tr>
    </tbody></table>
    <p>Governance activity types are specific operations. Parameter proposals and emergency-state operations use the governance interfaces; applications must not manufacture an activity ordinal for a library operation. See <Link href="/identity">Identity and authority</Link> for account delegation.</p>
    <h2>Parameter proposals and activation</h2>
    <p>On the Cosmos side, <code>modules/layerxgov</code> handles proposals and pause scopes (including <code>ErrPausedScope</code>), separate from activity-log governance. Parameter-proposal field sets, any minimum activation delay, and staged-rollout scopes (All/Module/Market/Account set) were not verified in source and are not specified here. Before submitting a proposal, read the current value, bounds, and version from the deployed module, retain the proposal identifier and receipt, and verify the enacted value after activation.</p>
    <h2>Emergency controls</h2>
    <p>Governance records emergency halt and resume events with an ordered sequence. A pause identifies a module, market, or network scope, a trigger commitment, exit conditions, and entry epoch. Module enablement is also an ordered governance action. These interfaces require governance authorization and do not create an operator shortcut outside the log.</p>
    <p>When responding to an incident, retain the triggering evidence, select the narrowest applicable scope, order the halt, and inspect the resulting emergency state. Resume against the recorded exit conditions through a subsequent ordered action. Applications continue to distinguish execution availability from the availability of existing receipts and proofs.</p>
    <h2>Settlement roles and timelocks</h2>
    <table><thead><tr><th>Role</th><th>Authority</th></tr></thead><tbody><tr><td>Proposer</td><td>Schedules an allowed target and function selector with a sufficient delay.</td></tr><tr><td>Executor</td><td>Executes the exact scheduled operation after readiness and before grace-period expiry.</td></tr><tr><td>Guardian</td><td>Cancels a pending, incomplete operation.</td></tr><tr><td>Timelock itself</td><td>Changes roles, minimum delay, and call permissions through authorized self-calls.</td></tr></tbody></table>
    <p>The operation identifier commits to the chain, timelock address, target, value, calldata hash, salt, and nonce. Scheduling and execution both enforce call permissions and value limits. Completion prevents operation replay; target failure reverts execution. Read the deployed contract&apos;s <code>minDelay</code>, <code>delayFloor</code>, and <code>gracePeriod</code> rather than assuming one universal waiting period.</p>
    <h3>Upgrades respect component boundaries</h3>
    <p>Layer X settlement components expose their role, static configuration hash, release version, and storage-layout version. The component base disables UUPS upgrades. The Sidiora proxy governance timelock provides a separate constrained upgrade path for its proxy targets. A proxy upgrade therefore follows that target&apos;s governance contract, while protocol parameter changes follow their own versioned activity rules.</p>
    <p>Before execution, compare the scheduled target, implementation, calldata, configuration commitments, and activation dependencies. After execution, retain the operation event and verify the target&apos;s resulting version and state.</p>
    <h2>Continue</h2><p>Read <Link href="/guarantors">Guarantors</Link> for membership and bonds, <Link href="/security">Security and trust boundaries</Link> for verification responsibilities, and <Link href="/finality">Finality</Link> for checkpoint settlement.</p>
  </DocsLayout>
}
