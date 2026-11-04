# Human owner registration details

Owner enrollment connects a durable Human principal, protected signing keys,
an authorized chain account and a recovery policy. Directory provisioning,
account funding and committed authority state are separate facts; producing a
local JSON file does not establish all three.

Paxeer is one network. The kernel executes inside `paxd`, and a receipt becomes
final at BFT block commit. Use [Public RPC endpoints](../reference/public-rpc.md)
for node transactions and [Checkpoints](../concepts/checkpoints.md) for the
distinction between execution receipts and committed evidence.

## Directory provisioning and chain admission

The identity provider at
`services/human/crates/layerx-human-identity-provider/` implements LXIP
`provision-owner`. It consumes `email`, `display_name`, `idempotency_key` and
`now` on standard input and returns `principal`, `did`, `recovery_root`,
`recovery_threshold` and `recovery_delay_seconds`. It returns no signing key.
Retain the exact returned principal and DID and their mapping to chain
authority; do not substitute a locally invented identity.

The provider reads its established recovery policy from
`LAYERX_HUMAN_IDENTITY_PROVIDER_RECOVERY_POLICY_FILE`: an absolute path to an
owner-held, mode-0600, single-link regular JSON file. The fields are `root`
(32 byte integers), `threshold` (nonzero u16) and `delay_seconds` (nonzero u64).
Its durable directory is `LAYERX_HUMAN_IDENTITY_PROVIDER_STATE_ROOT`.
Changing that policy file does not change existing accounts' recorded policy.

Directory provisioning does not submit a chain transaction. Likewise,
`provision-account` derives a protocol account identifier; derivation does not
open or fund an account. Retain directory identifiers and chain identifiers
with their explicit binding rather than assuming they are interchangeable.

For chain authority, the merged EVM keeper verifies a signed DID-key binding
for the caller (`modules/evm/keeper/`). The authority keeper resolves that
persisted binding on every write (`modules/layerxauth/keeper/authority.go`).
An unbound caller is refused. Identity registration and recovery policy records
use
`recordLifecycle(bytes,bytes32)` on the `layerxauth` precompile at
`0x000000000000000000000000000000000000101C`; the second argument is the
idempotency key. The payload must match the authorized caller's identity and
the lifecycle encoding in `src/modules/governance/`. The current
`RecordLifecycle` entry point accepts identity and recovery ordinals 1 and 3;
it is not a generic route for arbitrary Governance operations.

The local directory and chain registration must both be retained and checked.
The old genesis identity-file admission and standalone socket producer are
retired onboarding procedures; they do not replace a committed chain write.

## Funding the owner

Fund the intended derived sub-account through a native bank send or the
appropriate ledger transfer, and check the committed balance for the exact
account and asset. Account opening and transfer execution are implemented in
`modules/layerxledger/keeper/accounts.go` and `send.go`; `mirror.go` maps
successful ledger effects to bank balances. Deriving an account ID alone is
not evidence that it exists or has funds. The transaction sender also needs
the funds required by the node's fee policy.

The former `layerxcustody` deposit at `0x1013`, custody profile bootstrap,
bridge credit attestation and deposit-root publication are retired funding
paths. Do not run the old custody producer to establish an owner balance.
See [Custody credit retirement](custody.md).

## Guardian key custody

The retained guardian enrollment utility is
`services/platform/hosted/human/guardians.py`. Its file and signature format
remains useful for recovery material; its role labels do not describe current
consensus operators. The utility uses three roles, `guarantor-1`,
`guarantor-2` and `sequencer`, with threshold two and a policy delay of 86400
seconds. These recovery guardians do not finalize network blocks.

Each `enroll` process generates a separate random Ed25519 key into its own
mode-0700 directory at `$SECRETS_DIR/human-guardians/<role>-e<epoch>`.
The directory holds a mode-0600, 32-byte `seed`. Epochs start at one and are
bounded at 64. Existing material is refused rather than replaced. Recovery
keys must remain separate from transaction and consensus signing keys.

The enrollment signature binds `LX:HUMAN:GUARDIAN:BINDING:v1` plus NUL,
big-endian u16 epoch, big-endian u16 role length, the ASCII role, the 32-byte
operator identity and the guardian's 32-byte public key.

`assemble` reads the public enrollment documents without opening seed
directories. It writes `recovery-guardians.json`, the identical
`recovery-guardian-bindings.json`, and `recovery-policy.json`. The guardian
document has `version`, `threshold`, `public_keys` and `members`. Each member
has `role`, `identity`, `public_key`, `epoch`, `custody`, `signature` and
`rotation`. At epoch one, `rotation` is null. Later members carry the
predecessor member document and the outgoing key's rotation signature.

To rotate, enroll the successor at the next epoch, then run `authorize`
using only the outgoing guardian's seed directory. The signature binds
`LX:HUMAN:GUARDIAN:ROTATION:v1` plus NUL, big-endian u16 epoch and role length,
the ASCII role, operator identity, outgoing key and successor key. Assembly
validates the complete chain from epoch one and refuses missing or mismatched
authorization.

`owner_native.guardian_set` verifies distinct roles, operator identities,
public keys and custody directories, the directory naming convention, the
public key manifest and the policy threshold. The document version is the
highest member epoch. The recovery commitment is SHA-256 of
`LX:HUMAN:RECOVERY:v1` plus NUL, big-endian u16 threshold and key count, then
the sorted Ed25519 public keys.

Keep seeds in separately controlled custody and distribute only the public
bindings and recovery policy. Directory and signature separation on one host
does not establish independent operators. Assembling a valid guardian file
also does not prove that its recovery policy was registered on chain.

## Committed evidence and unknown outcomes

Identity, capability and recovery claims need evidence tied to the same
principal, authorized account and committed chain state. The security provider
at `services/human/crates/layerx-human-security-provider/src/recovery.rs`
verifies the execution header and commit, sequence-batch transaction inclusion,
receipt signature and authorized receipt keys, successful transaction result,
and recovery call binding. Its state and result proofs use the authenticated
successor header where required by the chain's commitment layout.

A signed receipt or a locally supplied recovery policy alone cannot establish
a successful committed recovery transition. The former guarantor checkpoint
certificate prerequisite is retired; current evidence is tied to block
headers, commits and the relevant inclusion proofs.

Retain the provisioning idempotency key, signed transaction bytes, transaction
hash, chain idempotency key, receipts and evidence before retrying an interrupted
operation. Reconcile directory state and query the original transaction and
committed state first. A timeout is an unknown outcome, not proof of failure;
do not create a new principal, key or funding transfer to bypass it. Resume
with the original operation identity only after reconciling retained evidence.

The historical `owner-native.started`, `custody-bootstrap.started` and
`custody-deposit.started` files signal potentially completed old operations.
Retain them for reconciliation; deleting them and rerunning the retired
producer is not a merged-network recovery procedure.
