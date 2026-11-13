<!-- Source: https://docs.paxeer.app/governance/ -->

# Governance

Ordered authority changes, versioned parameters, staged rollouts, and controlled settlement upgrades across Paxeer X.

Paxeer X governance operates across two boundaries: the Layer X activity log governs protocol execution, while Paxeer settlement contracts enforce their own authorized roles and delayed calls. A governance decision is evidence with an actor, scope, ordering position, and activation condition.

## Governance in the activity log

Module 7 processes governance activities through the same envelope, authority, ordering, fee, and receipt rules as other activities. Identity creation, key rotation, recovery changes, session grants, revocation, and delegated capability issuance therefore remain part of replayable history.

Only the following Module-7 ordinals were verified in monorepo source (`modules/lxphost` tests). Meanings for other ordinals (`0x00070002`, `0x00070003`, `0x00070006`, `0x00070009`) were not verified and are not listed.

| Activity | Purpose (as used in source) |
| --- | --- |
| `0x00070001` | Register / create-or-onboard identity (`registerType`). |
| `0x00070005` | Session-authority activity (used with fee-record loading in host tests). |
| `0x00070008` | Grant (`grantType`). |

Governance activity types are specific operations. Parameter proposals and emergency-state operations use the governance interfaces; applications must not manufacture an activity ordinal for a library operation. See [Identity and authority](https://docs.paxeer.app/identity) for account delegation.

## Parameter proposals and activation

On the Cosmos side, `modules/layerxgov` handles proposals and pause scopes (including `ErrPausedScope`), separate from activity-log governance. Parameter-proposal field sets, any minimum activation delay, and staged-rollout scopes (All/Module/Market/Account set) were not verified in source and are not specified here. Before submitting a proposal, read the current value, bounds, and version from the deployed module, retain the proposal identifier and receipt, and verify the enacted value after activation.

## Emergency controls

Governance records emergency halt and resume events with an ordered sequence. A pause identifies a module, market, or network scope, a trigger commitment, exit conditions, and entry epoch. Module enablement is also an ordered governance action. These interfaces require governance authorization and do not create an operator shortcut outside the log.

When responding to an incident, retain the triggering evidence, select the narrowest applicable scope, order the halt, and inspect the resulting emergency state. Resume against the recorded exit conditions through a subsequent ordered action. Applications continue to distinguish execution availability from the availability of existing receipts and proofs.

## Settlement roles and timelocks

| Role | Authority |
| --- | --- |
| Proposer | Schedules an allowed target and function selector with a sufficient delay. |
| Executor | Executes the exact scheduled operation after readiness and before grace-period expiry. |
| Guardian | Cancels a pending, incomplete operation. |
| Timelock itself | Changes roles, minimum delay, and call permissions through authorized self-calls. |

The operation identifier commits to the chain, timelock address, target, value, calldata hash, salt, and nonce. Scheduling and execution both enforce call permissions and value limits. Completion prevents operation replay; target failure reverts execution. Read the deployed contract's `minDelay`, `delayFloor`, and `gracePeriod` rather than assuming one universal waiting period.

### Upgrades respect component boundaries

Layer X settlement components expose their role, static configuration hash, release version, and storage-layout version. The component base disables UUPS upgrades. The Sidiora proxy governance timelock provides a separate constrained upgrade path for its proxy targets. A proxy upgrade therefore follows that target's governance contract, while protocol parameter changes follow their own versioned activity rules.

Before execution, compare the scheduled target, implementation, calldata, configuration commitments, and activation dependencies. After execution, retain the operation event and verify the target's resulting version and state.

## Continue

Read [Guarantors](https://docs.paxeer.app/guarantors) for membership and bonds, [Security and trust boundaries](https://docs.paxeer.app/security) for verification responsibilities, and [Finality](https://docs.paxeer.app/finality) for checkpoint settlement.
