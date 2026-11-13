<!-- Source: https://docs.paxeer.app/custody/ -->

# Account funding & key custody

Fund accounts on the merged network, bind owner identities, and understand signing-key custody in the Human interface.

## Custody precompile behavior

The `layerxcustody` precompile at `0x0000000000000000000000000000000000001013` is disabled on the merged network. Its 20 ABI methods, including `deposit`, `depositToken`, `requestWithdrawal`, `finaliseWithdrawal`, `requestForcedExit`, and `executeForcedExit`, revert with `LXP_MODULE_DISABLED: module disabled; layerxCustody retired`. The disabled-module check precedes call-shape checks.

The ABI is declared in `contracts/src/precompiles/ILayerXCustody.sol`. Its deposit-root registration method, `registerDepositRoot`, and its views, including `depositCount`, `depositNonce`, `assetByPointer`, and `nativeAssetId`, are subject to the same runtime refusal.

## Account and transfer paths

| Need | Current path |
| --- | --- |
| Establish an owner account | Bind the DID to the owner EVM address with `bindLayerX(bytes32, bytes)` through the address precompile at `0x0000000000000000000000000000000000001004`, then register identity and recovery through LayerXAuth. See [identity](https://docs.paxeer.app/identity). |
| Fund an account on the merged network | Transfer the native asset directly to the bound EVM address. Development allocations use a signed native transfer; see the developer funding guide. |
| Move value to or from an external chain | Use the governed bridge at `0x0000000000000000000000000000000000001016`, which requires registered chains, attestors, caps, and enabled policy. See [the bridge](https://docs.paxeer.app/bridge). |
| Move funds inside the Human interface | Use a `move` journey and track its receipt and block commitment. |

## Signing-key custody

The Human interface separates application authentication, protocol signing, and owner EVM authority:

| Identity | Purpose | Signing authority |
| --- | --- | --- |
| Application identity | A passkey opens the Human application session. | Authenticates the application session; it is separate from the protocol key. |
| Protocol identity | A LayerX DID authorizes native activities and account operations. | The custody service holds Human and managed-agent keys and signs disclosed activities. |
| Owner EVM address | Binds the DID and submits Governance lifecycle transactions. | Signs the binding and lifecycle transactions on chain ID 125. |

The browser does not hold the Human protocol identity's private key. The one exception is the explicit `export-primary-key` security action, which is the only route by which a DID primary key leaves the service; see [the Human interface](https://docs.paxeer.app/human-interface).

## Confirm account funding

Confirm the DID-to-EVM binding before funding. Retain the transaction hash, inspect the native balance and receipt, and check the recipient and amount. The native funding path resolves the recipient through the address precompile and checks the transaction receipt. A funding reference alone does not establish a successful transfer.

## Show evidence accurately

Human journeys report three rungs: `unverified` (submitted), `receipt-verified` (receipted), and `checkpoint-finalised` (committed). A receipt is final once its block is committed. Evidence returned by `GET /v1/evidence/{evidence_id}` includes its class, verification level, content type, and encoded bytes.

Continue with [owner registration](https://docs.paxeer.app/identity), [the governed bridge](https://docs.paxeer.app/bridge), [block commitment and proof verification](https://docs.paxeer.app/settlement), and [Paxeer JSON-RPC](https://docs.paxeer.app/json-rpc).
