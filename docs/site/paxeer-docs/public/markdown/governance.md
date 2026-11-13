<!-- Source: https://docs.paxeer.app/governance/ -->

# Governance

Authorized module proposals, scoped pauses, and constrained proxy upgrades on Paxeer.

Governance authorizes changes on one network. Chain proposals carry typed module messages, protocol activities carry their own authority, and Solidity proxy upgrades use their declared governance contract.

## Governance activities

The kernel governance module processes identity and authority activities under the canonical activity envelope, ordering, fee, and receipt rules. Its implementation is `src/modules/governance/lxp_governance.c`.

The governance activity catalogue includes identity registration, session authority, and delegated grants. The following identifiers are handled by the kernel:

| Activity | Purpose |
| --- | --- |
| `0x00070001` | Identity registration and onboarding. |
| `0x00070005` | Session authority. |
| `0x00070008` | Delegated grant. |

Use the declared encoding and authority for each activity. Governance proposal handling in `modules/layerxgov` is a separate typed chain interface. See [Identity and authority](https://docs.paxeer.app/identity) for account delegation.

## Typed module proposals

`modules/layerxgov/types/proposal.go` defines `LayerXProposal`, which contains a title, description, and packed messages. Validation requires supported messages whose authority is the governance module account. The handler in `modules/layerxgov/handler.go` dispatches every message in a cached context and commits the changes only when all handlers succeed.

## Scoped pause controls

`CheckPauseScope` enforces module pauses while allowing named exit operations. Its scopes cover payment, exchange, ledger, authorization, and agent operations. A paused operation outside its allowed exit set returns `ErrPausedScope`.

Payment exits include escrow release and timeout, budget closure and defunding, stream pause and close, and service-offer withdrawal. Exchange exits include order cancellation, position closure, and margin withdrawal. Read the scope's exact method names before invoking an exit.

## Proxy upgrade roles

| Role | Authority |
| --- | --- |
| Governance authority | Schedules the permitted proxy upgrade with sufficient delay. |
| Executor | Executes the exact scheduled operation within its readiness window. |
| Guardian | Cancels a scheduled operation before it becomes ready. |
| Proxy target | Receives the authorized upgradeToAndCall operation with zero value. |

`contracts/governance/SidioraProxyTimelock.sol` binds an immutable proxy, governance authority, executor, guardian, minimum delay, delay floor, and grace period. Its operation identifier commits to the chain, timelock, target, value, calldata hash, salt, and nonce. Execution requires the scheduled operation to be ready and within its grace period.

### Constrained proxy upgrades

The Sidiora timelock permits only `upgradeToAndCall` on its configured proxy, requires zero call value and deployed implementation code, and checks canonical upgrade calldata. Its roles and delay settings are immutable.

Compare the target, implementation, calldata, salt, nonce, and ready time before execution. Inspect the operation event and resulting proxy state afterwards. A failed target call reverts execution; a completed operation cannot execute again.

## Continue

Read [Validators and commitment](https://docs.paxeer.app/guarantors) for consensus evidence, [Security and trust boundaries](https://docs.paxeer.app/security) for verification responsibilities, and [Finality](https://docs.paxeer.app/finality) for block commitment.
