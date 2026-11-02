# Authenticated custody credit

Custody credit is Bridge module 8, ordinal 1, version 1, on protocol 3. It is
available only with an explicitly signed custody genesis profile. Ordinary
genesis still reconstructs Programs 4 and Asset 1, with unchanged encoding and
zero balances. A custody genesis adds Bridge 1; it does not preallocate value.
Natively issued tokens and per-asset accounts are a separate surface on
[Assets](../concepts/assets.md). Paxeer-custody asset ids stay their existing registered
values.
An existing deployment cannot acquire this profile through an environment
variable, snapshot substitution, or a new manifest with different roots.

## Trust and evidence

The genesis signer authorizes the Paxeer EVM chain ID `125`, the native custody
module address and its module identity, one asset, the canonical reserve
account, the Comet chain ID, a trusted Comet height with its next-validators
hash and header time, and a trusting period. There is no custody attestor key.
A deposit is admitted by a Tendermint light-client proof of the custody
module's own `Deposit` record against that trusted header
(`include/layerx/lxp_bridge_light.h`, `src/modules/bridge/lxp_bridge_light.c`).
Custody is the `layerxcustody` module behind the precompile at
`0x0000000000000000000000000000000000001013`; no vault, wrapped token or asset
registry contract is pinned.

The trust root is the validator set the profile commits to. An `LXLB1` bundle
is refused unless its signed header carries more than two thirds of that
header's own validator power, the header is no older than the trusted one, and,
when the validator set has changed, the signers overlap the trusted set by more
than one third of its power. Trust that has aged past the profile's trusting
period is refused outright rather than degraded to a weaker check. The profile
pins the module identity and the LayerX asset ID, not Paxeer governance or
token economics; those remain explicit custody trust assumptions.

Local custody evidence production is retired. Credit evidence is produced
by the chain, not by a local script. The former local proof producer and
its RPC checks are no longer an operator funding path. For current ledger
payments, follow the [Human service payment path](payments).

The native verifier checks the light-client bundle and every bound field of
the credit head: profile digest, LayerX network and protocol, deposit ID,
asset, beneficiary account, beneficiary key, depositor, amount, nonce, state
height, the proven header hash, its application root, the header height, the
header's validators hash, and the SHA-256 of the retained bundle. The proven
`Deposit` record must match the head field for field. The actor also signs the
full canonical LayerX activity. Identity resolution and existing sequence/fee
admission are not bypassed.
The beneficiary public key is a producer-supplied identity binding, not a field
carried by the proven deposit record. The kernel separately checks that key
against the authenticated actor DID and signed activity. The producer requires
an explicit expected amount and refuses a deposit with any different amount.

## Profile and payload

The profile is 223 bytes, stored under Bridge key `custody-credit-profile/v1`
(zero-padded to 32 bytes) in the signed genesis module values. Fields are:

| Offset | Bytes | Field |
|---:|---:|---|
| 0 | 5 | `LXBC3` |
| 5 | 8 | EVM chain ID (125) |
| 13 | 20 | custody precompile address |
| 33 | 32 | `sha256("LX:CUSTODY:MODULE:v1" \|\| "layerxcustody" \|\| address)` |
| 65 | 32 | trusted next-validators hash |
| 97 | 32 | asset ID |
| 129 | 32 | `system:paxeer-reserve` account ID |
| 161 | 8 | trusted Comet height |
| 169 | 32 | Comet chain ID, NUL-padded |
| 201 | 4 | LayerX network ID |
| 205 | 2 | LayerX protocol (3) |
| 207 | 8 | trusting period, seconds |
| 215 | 8 | trusted header time, Unix seconds |

`lxp_bridge_light_trust_load` seeds trust from those fields. An accepted later
header is stored under Bridge key `paxeer-light-trust/v1`, and a stored trust
at or below the profile's height is refused.

Profile-enabled genesis reconstruction also inserts `custody-issued:` followed
by the asset ID, with an exact 16-byte zero value. This is committed by the
signed genesis root and gives initial issuance a real inclusion proof. It adds
no balances. Without the profile, legacy reconstruction remains unchanged.

A credit payload is the 363-byte `LXDC3` head followed by the `LXLB1`
light-client bundle; the committed vectors in
`tests/fixtures/custody/paxeer-light-v1` are 1495 bytes. Nothing in the payload
carries a custody signature. The head is `LXDC3` (5), profile SHA-256 (32),
network (4), protocol (2), deposit ID (32), asset (32), beneficiary (32),
beneficiary key (32), depositor (20), amount (16), nonce (8), state height H
(8), signed header H+1 hash (32), that header's application root (32), header
height H+1 (8), its validators hash (32), SHA-256 of the bundle (32), and proof
kind `2` (4). All integers are unsigned big-endian. The 363-byte head is what
the module retains under `deposit-nullifier:`; the bundle is verified and
discarded. Legacy deposit-root registrations are not reinterpreted.

Idempotency is `SHA256("LX:DEPOSIT:NULLIFIER:v1" || deposit_id)`. The activity's
idempotency field must equal it. The HTTP `Idempotency-Key` uses its hexadecimal
spelling. A different activity sequence cannot credit the deposit again, and
changing the idempotency field is refused.

## Monetary transaction

Before issuing, the kernel verifies that its committed `custody-issued:` total
equals the sum of all account balances for this asset. It checks overflow,
asset pause state, reserve identity, beneficiary identity/key and replay state.
It stages an exact issuance increase equal to the verified custody amount,
snapshots the reserve, credits that amount into the reserve, then runs the
ordinary **conserved** reserve-to-beneficiary transfer. The recipient is created
at zero, with its real authority key, in the same staged transaction if absent.
The final summed balance must equal the new issued total.

Supply, full replay evidence, beneficiary registration, balances and sequences
commit together through the existing module/state journal. Failures roll them
back. No change is made to generic transfer conservation or bootstrap balance
semantics. Receipts carry the conserved transfer root in the transfer effect and a 208-byte custody
event binding deposit, asset, beneficiary, amount, profile/evidence digests and
before/after issued supply. A second 112-byte event binds reserve ID (32), actual
reserve before/after balances (16 each), beneficiary before/after balances
(16 each), and reserve before/after sequences (8 each). These are transition
boundary balances: the reserve does not claim newly issued funds at the previous
state root. The custody receipt has module 8, version 1 and operation 0; all
canonical conserved-ledger projection fields remain zero. Existing ledger
receipt assertions are unchanged and intentionally do not accept this as a
conserved Asset transfer. Verify canonical receipt signature and signed-header
inclusion, then the Bridge effects and account/issued-state proofs instead.
The native binding checks event balances against real transaction snapshots and
issued values against committed/staged KV. Before/after issuance values have
ordinary Bridge subtree inclusion proofs, including the initial zero. Absence of
the beneficiary derives from reconstruction of the complete signed empty genesis;
no non-inclusion proof is fabricated for missing accounts. Account proofs bind through `account-tree` and
the universal subtree to the receipt root.
Canonical activity replay restores the same state.

## Reproducible local funding

The local custody funding procedure is retired. Credit evidence is produced
by the chain, not by a local script. Local profile generation, evidence
production, signing and submission of custody funding activities, and their
standalone qualification steps are no longer an operator funding path.

For current ledger payments, use the
[Human service payment path](payments).

[Home](../index.md)
