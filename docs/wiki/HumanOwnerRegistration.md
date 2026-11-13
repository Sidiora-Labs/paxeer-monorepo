# Human owner registration

Human owners use the Paxeer network's account, authority and recovery state.
The kernel runs inside `paxd` and participates in block proposal; a receipt is
final when its block commits. Registration, funding and recovery are transactions
on this one chain.

Use a root node URL from [Public JSON-RPC](PublicRpc.md), with EVM chain ID
125 (`0x7d`).
Method availability depends on the node's configuration and upgrade state.
Sub-chains are tenant domains in the database; keep the tenant and principal
bound to every hosted identity request.

## Owner identity and account

Keep the hosted principal, its exported DID, its signing authority and its
native account identifier distinct:

- The LXIP identity provider's `provision-owner` command persists a principal
  and a `did:layerx:act_...` DID with the supplied recovery policy. It does not
  return a signing key or prove that the identity has registered on-chain.
- The node's address binding uses `did:layerx:<64 lowercase hex characters of
  an Ed25519 public key>`. A hosted principal DID and a key-derived node DID
  are different identities; do not replace one with the other in retained
  registration evidence.
- A main account's canonical name is `agent:<DID>:main`. Its H32 account ID
  is derived by the protocol account codec, not by interpreting the DID as
  an address. The identity provider's `provision-account` command derives
  that ID from an exported DID; derivation alone does not create or fund it.

The Human service reads these sponsor settings in
[`production_components.rs`](../../services/human/crates/layerx-human-service/src/server/production_components.rs):

| Variable | Required binding |
| --- | --- |
| `LAYERX_HUMAN_AGENT_ACTOR` | Exact DID of the configured sponsor, matching the resolved principal owner. |
| `LAYERX_HUMAN_AGENT_AUTHORITY` | Owner authority reference. The sponsor check compares it to the resolved owner's lowercase public-key hex. |
| `LAYERX_HUMAN_AGENT_OWNER_ACCOUNT` | Canonical `agent:<DID>:main` name, matching the resolved owner account. |
| `LAYERX_HUMAN_AGENT_RECOVERY_ROOT` | Unpadded base64url encoding of the registered 32-byte recovery commitment. |
| `LAYERX_HUMAN_AGENT_RECOVERY_THRESHOLD` | Decimal u16 matching the registered policy. |

These are part of the service's configuration, not a complete environment.
[`production_onboarding.rs`](../../services/human/crates/layerx-human-service/src/server/production_onboarding.rs)
checks the sponsor's actor, authority, account and recovery policy together.
[`production_owner.rs`](../../services/human/crates/layerx-human-service/src/server/production_owner.rs)
also checks the custody descriptor, EVM binding, network, authenticated identity
state, primary key and revocation sequence. Setting environment variables does
not establish those bindings.

## Node admission and native funding

For an owner using the node's key-derived DID binding:

1. Read `layerXBindNonce(address)` from the address precompile at
   `0x0000000000000000000000000000000000001004`.
2. Have the Ed25519 DID key sign the exact message
   `"LX:PAXEER-BIND:v1" || chain_id:uint256be || evm_address:20 bytes || nonce:u64be`.
   The EVM transaction supplies the EVM address's consent. Call
   `bindLayerX(bytes32,bytes)` from that address with the public key and
   64-byte signature.
3. Verify the successful committed transaction and `LayerXBound` event.
   Query `getUnifiedAccount(address)` to obtain the binding and main account
   ID. It returns zero DID/account fields for an unbound address, so a
   successful query alone is insufficient.
4. Register the identity and recovery state through the governed lifecycle
   path below, then fund the intended account with the correct native asset.
   Keep EVM gas funding distinct from the kernel account's asset balance.

The binding is one EVM address per DID and one DID per EVM address. The keeper
rejects noncanonical keys, invalid signatures, reused bindings and exhausted
nonces. `unbindLayerX()` consumes a nonce and removes the binding. See
[`Addr.sol`](../../precompiles/addr/Addr.sol) and
[`layerx_binding.go`](../../modules/evm/keeper/layerx_binding.go).

Kernel subaccounts map to native bank addresses through
`lxphost.BankAddress(accountID)`, implemented as
`address.Module("layerx", accountID[:])`.
[`accountmap.go`](../../modules/lxphost/accountmap.go) derives main and asset
account IDs; an asset account is named `agent:<DID>:asset:<asset-id-hex>`.
Use those derivations when selecting a funding destination. A bytes32 account
ID is not an EVM address.

For kernel ledger assets, LayerXLedger at
`0x000000000000000000000000000000000000101A` provides `openAccount(bytes32)`,
`accountId(bytes32,bytes32)` and `balanceOf(bytes32,bytes32)`.
Its authorized transfer paths apply receipt transfer legs to the corresponding
bank addresses in
[`mirror.go`](../../modules/layerxledger/keeper/mirror.go). Check both the asset
and destination account in the committed funding evidence; an unrelated bank
balance or a locally derived account name is insufficient evidence of usable
kernel funds.

The pre-merge custody precompile at `0x1013`, `deposit(bytes32)`, custody-credit
bootstrap and LNI identity-file admission are retired procedures. They are not
prerequisites for funding an owner on the merged network. External-chain
movement, when needed, uses the governed bridge at
`0x0000000000000000000000000000000000001016`; its
[`LayerXBridge.sol`](../../precompiles/layerxbridge/LayerXBridge.sol) contract
requires registered chains, attestors, caps and enabled policy. External bridge
attestations do not establish Paxeer block commitment.

## Native Governance registration encoding

The EVM lifecycle entry point is LayerXAuth at
`0x000000000000000000000000000000000000101C`:
`recordLifecycle(bytes payload, bytes32 idempotencyKey)` returns the global
sequence and emits `IdentityStateUpdated(bytes32,bytes)`.
The keeper resolves the caller's current EVM/DID binding and executes Governance
module 7 in the node's transaction context. An unbound caller is refused with
`LXP_AUTH_SCOPE`; a malformed lifecycle payload is refused with
`LXP_NON_CANONICAL`.

The basic owner forms are:

| Operation | Payload, with big-endian integers |
| --- | --- |
| Identity registration, ordinal 1 | `71 01 00 02 || did_id32 || primary_key32` |
| Rotation announcement, ordinal 2 | `71 02 00 04 || did_id32 || next_key32 || begin_ms:u64 || end_ms:u64 || effective_sequence:u64` |
| Recovery commitment, ordinal 3 | `71 03 00 03 || did_id32 || recovery_root32 || threshold:u16` |
| Recovery commitment with delays, ordinal 3 | `71 03 00 05 || did_id32 || recovery_root32 || threshold:u16 || required_delay_seconds:u64 || maximum_delay_seconds:u64` |

Derive `did_id32` with the protocol DID codec (`lxphost.DIDID`), and bind the
primary key to the caller's verified authority. The three-field recovery form
records zero delay fields; use the five-field form when asserting a delay
policy. It requires a positive delay and a maximum at least that delay.
Recovery roots and thresholds must be nonzero. These encodings are implemented
in [`lxp_governance.c`](../../src/modules/governance/lxp_governance.c), with the
EVM adapter in [`lifecycle.go`](../../precompiles/layerxauth/lifecycle.go).

`recordLifecycle` accepts identity and recovery ordinals, plus rotation ordinal
2 in its 92-byte version-zero form. Rotation requires a canonical new key,
no pending rotation, a future beginning, an end after that beginning and an
effective sequence greater than the current sequence. The caller's current
primary key authorizes the announcement. See
[`keeper/lifecycle.go`](../../modules/layerxauth/keeper/lifecycle.go).
Sponsored onboarding uses a separate signed consent format in
[`lxp_governance_onboarding.c`](../../src/modules/governance/lxp_governance_onboarding.c).

Session authorization uses the dedicated `grantSession(bytes,uint64,bytes32)`
and `revokeGrant(bytes32,uint8,uint64)` methods from the
[LayerXAuth ABI](../../precompiles/layerxauth/abi.json).
The version-1 native session payload begins `71 05 01 03` and contains a
length-prefixed canonical grant, expiry sequence and length-prefixed 32-byte
action key. Version zero is refused. The canonical grant is stored at
`05 || grant_id`; its 209-byte `LXGS2` summary is stored at `15 || grant_id`
and emitted as event `0x7145`. The ABI also exposes successor-session and
capability operations; use their declared methods and verify the scope and
revocation state before authorizing a spend.

## Guardian enrollment, rotation and cluster bootstrap

Recovery material must remain exercisable and separate from the primary owner
key. The retained
[`guardians.py`](../../services/platform/hosted/human/guardians.py) tooling
provides `enroll`, `authorize` and `assemble` operations for recovery documents:

- `enroll` generates a fresh Ed25519 key into a mode-0700 directory
  `human-guardians/<role>-e<epoch>`, with an exclusive mode-0600 `seed` file.
  Epochs start at one and are bounded at 64.
- `authorize` reads the outgoing guardian's seed and signs the successor
  binding. Assembly verifies the chain back to the first enrollment.
- `assemble` publishes identical `recovery-guardians.json` and
  `recovery-guardian-bindings.json` documents, plus `recovery-policy.json`.
  The documents carry `version`, `threshold`, `public_keys` and signed
  `members`; each member binds role, operator identity, public key, epoch,
  custody directory, signature and predecessor rotation evidence.

The retained tool uses roles `guarantor-1`, `guarantor-2` and `sequencer`, with
threshold two. Those labels describe its recovery-key records; they confer no
consensus authority. Its commitment is SHA-256 of `LX:HUMAN:RECOVERY:v1`, NUL,
threshold u16be, guardian count u16be and sorted public-key32 values.
`owner_native.guardian_set` verifies distinct roles, identities, keys and
custody directories, the published key manifest and rotation signatures.
Separate directories on one host do not demonstrate independent operators.
Never publish seeds or copy them into registration JSON.

In the Human service, `advance_native_onboarding` finalizes the configured
sponsor from retained evidence. Other unfinished principals receive
`operation-retired`; complete or refused journeys retain their current status.
See
[`production_onboarding.rs`](../../services/human/crates/layerx-human-service/src/server/production_onboarding.rs).

## Submission and retained evidence

The current [`layerxctl`](../../cmd/layerxctl/src/main.rs) uses JSON-RPC:

```text
layerxctl submit --rpc-url URL --transaction SIGNED_EIP1559_FILE --ca-cert ROOT_DER
layerxctl read-state --rpc-url URL --to HEX40 --data ABI_HEX --block-height HEIGHT --ca-cert ROOT_DER
```

`--transaction` reads binary signed EIP-1559 bytes. `read-state` uses `eth_call`
and defaults to `pending` without `--block-height`; a pending view is not
committed evidence. HTTPS requires a pinned DER root. Supply the deployed ABI's
encoded calldata and select an explicit committed height for retained reads.

Submission reports `receipted` only after verifying the receipt under a receipt
key obtained through the configured endpoint. An unverifiable or uncertain
submission remains `submitted` with exit 3; an authenticated execution failure
exits 1. The CLI reports only the receipted assurance level even when the node
returns a committed receipt. Authenticate receipt-key governance independently
and use [SDK terminal verification](SdkTerminalVerification.md) for block
inclusion and consensus evidence before claiming final registration.

The retained Human validator in
[`provision.py`](../../services/platform/hosted/human/provision.py) accepts
`human-evidence-input/owner-registration.json` with exactly `owner_account`
(H32 hex), textual `authority` and `identity`. Identity evidence references
`activity_id` and the canonical unsigned receipt digest in the same principal's
activity records. JSON validation checks the document contract; it does not
prove successful merged-network registration, recovery or funding.
Retain signed transaction bytes, receipts, authenticated key ranges, block
proofs and principal/tenant bindings. Reconcile an unknown outcome by the same
transaction hash and retained action identity before retrying.

## Hosted provisioning tests

`make human-test-hosted-provisioning` builds `layerx-human-identity-provider`
from `services/human/Cargo.toml`, then runs the hosted Python tests through
`tools/runtime/run-with-clock.sh`. `HUMAN_TARGET_DIR` uses `CARGO_TARGET_DIR`
when set and otherwise `services/human/target`;
`LAYERX_HUMAN_IDENTITY_PROVIDER_BIN` points the tests to the built binary.
The target checks for `pytest`, PyYAML, cryptography and the bridge requirements
(`Crypto.Hash.keccak`, `eth_account`, `eth_utils`). This is a provisioning test
entry point, not evidence that a live owner completed registration. See the
[identity provider guide](../../services/human/crates/layerx-human-identity-provider/README.md)
for its protected state and recovery-policy inputs.

[SDK terminal verification](SdkTerminalVerification.md) · [Home](Home.md)
