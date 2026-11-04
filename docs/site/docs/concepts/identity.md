# Identity and authority

Paxeer mainnet-beta is one BFT network with EVM chain ID `125` (`0x7d`). The
kernel is embedded in `paxd` and participates in block proposal and execution.
Identity, account ownership and authorization are evaluated against that
network's state. A successful receipt is final when its block is committed.

A Human login, a DID and an EVM wallet identify different things. The passkey
authenticates an application principal. The DID names a protocol identity. The
wallet signs chain transactions, and its persisted DID binding supplies native
authority for the associated ledger operations.

See [Human journeys](../human/journeys.md),
[Human web app](../human/web-app.md) and
[Public node and hosted gateway JSON-RPC](../platform/gateway-rpc.md).

## Accounts and native authority

The address precompile `addr` at
`0x0000000000000000000000000000000000001004` binds an EVM address to an
Ed25519 public key. The corresponding DID is
`did:layerx:<64 lowercase hex characters of the public key>`. The main account
name is `agent:<did>:main`; named budget, escrow and margin accounts remain
protocol account references. An account name alone grants no spending rights.

For native ledger calls, `modules/layerxledger/keeper/authority.go` resolves
the EVM caller through the persisted address binding and constructs the kernel
actor. The `layerxledger` precompile at
`0x000000000000000000000000000000000000101a` requires this authority on state
changes. Unbound callers are refused. Its `accountId(bytes32,bytes32)` view
derives an asset account from a DID key and asset ID; deriving the ID does not
open or fund the account.

The bound EVM wallet therefore has native economic authority, subject to
account ownership, grants, budgets and the operation's checks. The old
payout-only binding description is retired. Holding an unrelated key or an
application session does not authorize a debit.

## Binding a wallet

The wallet calls `bindLayerX(bytes32,bytes)` on `addr`, supplying the DID public
key and its 64-byte Ed25519 consent signature. The signed message is exactly:

```text
"LX:PAXEER-BIND:v1" || chain ID (32-byte big-endian)
                    || EVM address (20 bytes) || binding nonce (8-byte big-endian)
```

On this network the chain ID is `125`. The binding nonce is distinct from the
EVM transaction nonce. Each address and DID can have only one current binding;
the keeper rejects conflicting bindings and invalid consent. The successful
call emits `LayerXBound(address,bytes32,uint64)`.

`unbindLayerX()` removes the caller's binding on that wallet's authority.
Native authorization resolves the binding again on subsequent calls; it does
not reuse an actor cached before unbinding. The message encoding lives in
`modules/evm/types/layerx_binding.go`, the keeper in
`modules/evm/keeper/layerx_binding.go`, and the ABI entry points in
`precompiles/addr/addr.go`.

## Sessions, grants and recovery

Protocol session keys and capabilities have explicit scope, expiration and
revocation state. A service session token authenticates access to the service;
it is not itself a protocol grant. Revocation prevents later authorized use
without undoing already committed operations. Rotation and recovery must use
the registered authority and their challenge windows.

The native `layerxauth` module and precompile at
`0x000000000000000000000000000000000000101c` handle authorization operations
through the embedded kernel. The module resolves the caller's current DID
binding and uses consensus block time for execution validity. Pre-merge batch
timestamp descriptions are historical protocol terminology, not a second
clock or a separate identity service on the network.

## Human onboarding

The Human service keeps passkey authentication, protocol identity and wallet
authority bound to the same principal. Managed signing uses the KMS and
attestor providers; private key material stays out of the browser. The custody
EVM path persists the exact authorized transaction and reserves its nonce
before signing, then retains signed bytes and submission acknowledgements for
recovery.

Wallet setup uses the chain-125 `bindLayerX` transaction. The onboarding sponsor
prepares its call through the gas station, which supplies a quote and sponsored
submission. A transaction hash is pending evidence. The sponsor checks the
successful transaction status and matching `LayerXBound` log; this intermediate
result alone does not establish `Committed` completion.

Native owner setup accepts registration and recovery evidence only at
`Rung::Committed`, bound to the expected action and successful result. Where
the setup plan includes initial funding, that stage also needs matching
committed evidence. The internal `account_active` flag tracks verified identity
registration; it does not mean every onboarding stage, including recovery, has
finished.

The relevant implementations are
`services/human/crates/layerx-human-service/src/onboarding/native.rs`,
`src/server/onboarding_sponsor.rs` and `src/custody/evm.rs` within that crate.
Sponsor configuration reads `LAYERX_HUMAN_ONBOARDING_REGISTRY_FILE` with
`chain_id: 125` and an optional `gas_station` HTTPS origin; otherwise it reads
`LAYERX_HUMAN_GAS_STATION_ENDPOINT`. That origin must match the configured
`LAYERX_HUMAN_GAS_STATION` upstream.

## Availability and related material

Local provisioning is not evidence of a working onboarding deployment. The
production native onboarding dispatcher currently advances its configured
sponsor principal; other unfinished native onboardings receive
`operation-retired`. Clients must surface the returned state or refusal.
Hosted `px_register` creates an application principal and is not a substitute
for a committed wallet binding or completed owner setup.

- [Hosted identity](../platform/identity.md)
- [Human owner registration](../human/owner-registration.md)
- [Hosted Human and KMS configuration](../human/hosted.md)
- [Protocol reference](../protocol/index.md)
