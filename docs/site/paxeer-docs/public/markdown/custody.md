<!-- Source: https://docs.paxeer.app/custody/ -->

# Custody, deposits & exits

The pre-merge custody precompile is retired. This page states what replaced it and what still holds for key custody.

## The custody precompile is retired

The `layerxcustody` precompile at `0x0000000000000000000000000000000000001013` was retired from the merge upgrade height on the merged network. Every one of its 20 ABI methods — including `deposit`, `depositToken`, `requestWithdrawal`, `finaliseWithdrawal`, `requestForcedExit`, and `executeForcedExit` — now reverts with `LXP_MODULE_DISABLED: module disabled; layerxCustody retired`. The retirement gate runs before call-shape checks, so no argument combination reaches the old logic. Do not build new deposit, withdrawal, or exit flows against this address.

The full historical interface remains in `contracts/src/precompiles/ILayerXCustody.sol` for binding compatibility and audit. It also declares views such as `registerDepositRoot`, the deposit-root authority and registration digests, `depositCount`, `depositNonce`, `assetByPointer`, and `nativeAssetId`; all of them revert the same way at runtime.

## What replaced it

| Need | Current path |
| --- | --- |
| Establish an owner account | Bind the DID to the owner EVM address with `bindLayerX(bytes32, bytes)` through the address precompile at `0x…1004`, then register identity and recovery through LayerXAuth. See [identity](https://docs.paxeer.app/identity). |
| Fund an account on the merged network | Fund the kernel account directly with the correct native asset. No custody credit is issued and no deposit proof is required. |
| Move value to or from an external chain | Use the governed bridge at `0x0000000000000000000000000000000000001016`, which requires registered chains, attestors, caps, and enabled policy. See [the bridge](https://docs.paxeer.app/bridge). |
| Move funds inside the Human interface | Use a `move` journey on the merged network. The wallet-binding, deposit, withdraw, and exit journey kinds are retired and survive only as closed history. |

## Key custody is unchanged

Retiring the precompile did not change who holds signing keys in the Human interface. The three identities remain distinct:

| Identity | Purpose | Signing authority |
| --- | --- | --- |
| Application identity | A passkey opens the Human application session. | Authenticates the application session; it is separate from the protocol key. |
| Protocol identity | A LayerX DID authorizes native activities and account operations. | The custody service holds Human and managed-agent keys and signs disclosed activities. |
| Owner EVM address | Binds the DID and submits Governance lifecycle transactions. | Signs the binding and lifecycle transactions on chain ID 125. |

The browser does not hold the Human protocol identity's private key. The one exception is the explicit `export-primary-key` security action, which is the only route by which a DID primary key leaves the service; see [the Human interface](https://docs.paxeer.app/human-interface).

## Historical deposits

Pre-merge deposits were recorded by the custody module and credited through a native custody-credit activity bound to the deposit record. That bootstrap is retired together with the precompile: it is not a prerequisite for funding an owner on the merged network, and it cannot be replayed to issue new credit. Deposit replay used the nullifier `SHA256("LX:DEPOSIT:NULLIFIER:v1" || deposit_id)`, so a recorded deposit could only be credited once. Historical claim and deposit records remain readable in module state for audit, but the EVM entry points that acted on them no longer execute.

## Show evidence accurately

Human journeys report three current rungs: `unverified` (submitted), `receipt-verified` (receipted), and `checkpoint-finalised` (committed). The former `paxeer-finalised` and `settlement-anchored` labels are retired decoding values and do not define additional rungs. Evidence returned by `GET /v1/evidence/{evidence_id}` includes its class, verification level, content type, and encoded bytes.

Continue with [owner registration](https://docs.paxeer.app/identity), [the governed bridge](https://docs.paxeer.app/bridge), [checkpoint settlement and proof verification](https://docs.paxeer.app/settlement), and [Paxeer JSON-RPC](https://docs.paxeer.app/json-rpc).
