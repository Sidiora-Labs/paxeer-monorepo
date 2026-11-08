# Bridge module

The kernel custody credit activity and its light client are retired.
Bridge module ID `8` (`LXP_MODULE_BRIDGE`) no longer provides a custody
credit activity. The former deposit verification, execution, genesis profile,
payload and issuance procedures are no longer supported funding paths.

Withdraw requests are Asset ordinal 9. Emergency exit is a library path,
not a `0x0008xxxx` activity.

The LayerX↔Paxeer custody boundary is a different surface
from the Paxeer-side `layerxbridge` module and `LayerXBridge` precompile
(EVM address `0x1016`) that bridges Paxeer to external chains such as
Ethereum; see [Unified Network Architecture](UnifiedNetwork.md).

Sources: `include/layerx/lxp_bridge.h`,
`src/modules/bridge/lxp_bridge_withdraw.c`,
`src/modules/bridge/lxp_bridge_exit.c`.

---

## Custody credit retirement

For the retirement notice, see [Custody](Custody.md). For current ledger
payments, use the
[Human service payment path](PaymentsQuickstart.md).

---

## Withdraw request

See [Asset](Asset.md#withdraw). The LayerX debit lands in
`system:paxeer-withdrawals` before any Paxeer payout. Settlement binds the
asset recorded at request time (`spec/layerx-beta/spec.kvx` requirement 10).
A caller-supplied asset that disagrees is refused with no transfer.

The former bridge withdraw finalization path is retired.

---

## Emergency exit

`lxp_exit_eligibility` and `lxp_exit_declare`
(`include/layerx/lxp_bridge.h`) check eligibility and declare an emergency
exit against the last finalised checkpoint when the liveness bound is
exceeded. This is not an ordinary activity ordinal.

The former exit-claim construction path using guarantor signatures is retired.

---

## Start here

- [Asset](Asset.md)
- [Finality](Finality.md)
- [Sequencing](Sequencing.md)
- [Paxeer boundary](PaxeerBoundary.md)
