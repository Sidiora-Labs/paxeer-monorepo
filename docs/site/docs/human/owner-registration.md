# Human owner registration

Owner registration binds a Human principal to its signing authority, account and
recovery policy in chain state. Paxeer mainnet-beta is one BFT network, running
`paxd` with the kernel inside the network and participating in block proposal.
The EVM chain ID is **125** (`0x7d`). A receipt is final when its block is
committed.

Public Ethereum JSON-RPC requests go to the node's root URL, for example
`https://api1.mainnet-beta.paxeer.network`. Other node names include
`api2` through `api14.mainnet-beta.paxeer.network` and
`https://api-mainnet-beta.paxeer.network`. See [public RPC endpoints](../reference/public-rpc.md)
for the node list. Hosted gateway `px_` methods are a separate deployment
capability; their presence must not be inferred from a node's Ethereum RPC.

## Human configuration

`ProductionComponentsConfig::from_environment` in
`services/human/crates/layerx-human-service/src/server/production_components.rs`
reads the following owner inputs. They describe an existing owner; setting them
does not register an identity or establish a recovery policy.

| Variable | Encoding and binding |
| --- | --- |
| `LAYERX_HUMAN_AGENT_ACTOR` | Exact owner DID. Human compares it with the principal's resolved owner identity. Retained LXIP provisioning output must preserve its DID exactly. |
| `LAYERX_HUMAN_AGENT_AUTHORITY` | Primary public key as lowercase 64-character hex. The production sponsor check compares this text with the resolved owner's canonical public-key hex. |
| `LAYERX_HUMAN_AGENT_OWNER_ACCOUNT` | Canonical `agent:<actor-DID>:main` account name, matching the resolved principal account. This is distinct from the H32 account identifier stored in provisioning JSON. |
| `LAYERX_HUMAN_AGENT_RECOVERY_ROOT` | Unpadded base64url encoding of the registered 32-byte recovery commitment. |
| `LAYERX_HUMAN_AGENT_RECOVERY_THRESHOLD` | Decimal u16 matching the nonzero threshold in committed identity state. |

The production owner resolver also checks the custody key's EVM binding,
network ID, protocol version 3, primary key, revocation sequence and verified
identity state. A locally generated DID, account name or random recovery root
does not satisfy those checks. The configuration parser accepts nonempty actor
and authority strings; the later owner checks establish their binding.

## Registration through the chain

The current EVM entry points are:

| Entry point | Purpose |
| --- | --- |
| `addr`, `0x0000000000000000000000000000000000001004` | `bindLayerX(bytes32,bytes)` binds the EVM caller to an Ed25519 DID key. `getLayerXDid(address)` and `layerXBindNonce(address)` read the binding and its nonce. |
| `layerxauth`, `0x000000000000000000000000000000000000101C` | `recordLifecycle(bytes,bytes32)` executes identity registration or recovery-policy registration. The second argument is the idempotency key. |

`bindLayerX` requires the EVM transaction's consent and an Ed25519 signature
over the chain ID, EVM caller and current binding nonce. The auth keeper resolves
the persisted binding on every write, deriving the actor DID from the bound key.
Its Governance activity uses protocol 3, module 7 and the current identity
sequence. `recordLifecycle` accepts lifecycle payload ordinals 1 and 3; it is
not a general dispatcher for every Governance operation. Session and capability
operations have their own methods in `precompiles/layerxauth/abi.json`.

The binding and lifecycle implementations are in `precompiles/addr/addr.go`,
`modules/layerxauth/keeper/authority.go`,
`modules/layerxauth/keeper/execute.go` and
`precompiles/layerxauth/lifecycle.go`. Preserve the exact identity used by the
consumer: an LXIP result alone does not prove that a key-derived chain DID is
the same owner.

### Current CLI transport and outcomes

`cmd/layerxctl/src/main.rs` implements two commands:

```text
layerxctl read-state --rpc-url URL --to HEX40 --data HEX [--block-height N] [--ca-cert DER_FILE]
layerxctl submit --rpc-url URL --transaction FILE [--ca-cert DER_FILE]
```

`read-state` performs `eth_call`. It defaults to `pending`; `--block-height`
selects a decimal block height and produces an explicit hexadecimal block tag.
Addresses and calldata require a `0x` prefix. A pending read is useful for
preparation, but does not establish committed registration.

`submit` reads binary signed EIP-1559 type-2 transaction bytes, sends them with
`eth_sendRawTransaction`, then looks up `pax_sequencerReceipt` and
`px_getReceiptKeys` at the configured URL. HTTPS requires a pinned DER root
certificate via `--ca-cert`. The CLI does not create or sign the transaction.
Because `px_getReceiptKeys` belongs to the hosted gateway contract, a public
node URL alone does not guarantee that this CLI can finish receipt verification.

Successful signature and receipt-field verification yields `state: receipted`.
Exit 0 additionally requires protocol result code 0; a verified nonzero result
exits 1. Missing or unverifiable receipt evidence leaves `state: submitted`
and exit 3. An interrupted send can report `outcome: unknown` with the locally
computed transaction hash. Retain the exact signed bytes, hash and idempotency
key for reconciliation. Neither a submitted hash nor the CLI's `receipted`
label proves block commitment; confirm the transaction's committed block before
treating registration as final.

## Native Governance registration encoding

The kernel's Governance implementation remains in
`src/modules/governance/lxp_governance.c`. These retained protocol-3 encodings
are useful for decoding registration material and constructing the corresponding
ABI payloads. Integers are big-endian; concatenation is written as `||`, and
`did_id32` is the protocol DID hash, not a textual DID.

| Operation | Payload |
| --- | --- |
| Identity, ordinal 1, version 0 | `7101 0002 \|\| did_id32 \|\| primary_key32` |
| Rotation announcement, ordinal 2, version 0 | `7102 0004 \|\| did_id32 \|\| pending_key32 \|\| not_before_ms:u64 \|\| not_after_ms:u64 \|\| effective_sequence:u64` |
| Recovery, ordinal 3, three fields | `7103 0003 \|\| did_id32 \|\| recovery_root32 \|\| threshold:u16` |
| Recovery, ordinal 3, five fields | `7103 0005 \|\| did_id32 \|\| recovery_root32 \|\| threshold:u16 \|\| required_delay_seconds:u64 \|\| maximum_delay_seconds:u64` |
| Session registration, ordinal 5, version 1 | `7105 0103 \|\| length:u32 \|\| canonical_grant \|\| expiry_sequence:u64 \|\| 00000020 \|\| action_key32` |
| Revoke, ordinal 6 | `7106 0003 \|\| grant_id32 \|\| reason:u8 \|\| effective_sequence:u64` |

This table lists the retained forms, not every version the kernel accepts.
Identity registration checks that the supplied canonical primary key matches
the resolved owner authority. The three-field recovery form sets no delay
policy; the five-field form requires a positive minimum and a maximum at least
that minimum. A rotation announcement requires a distinct canonical pending
key, a future window and an effective sequence later than execution.

Session registration requires a canonical session-key grant bound to the owner
and current revocation generation, an expiry later than execution and a nonzero
action key. Version 0 is refused. Version 2 adds predecessor-grant and expected
fee-commitment fields for succession. Revoke requires the grant's owner, a
reason in 1 through 5, and an effective sequence equal to execution's sequence.

Governance stores identity state under `did_id32`, session grants under
`05 || grant_id32`, revocations under `06 || grant_id32` and session summaries
under `15 || grant_id32`. Event `0x7110` contains the 223-byte `LXGI1` identity
value in this order:

```text
LXGI1, DID32, primary32, revocation:u64, recovery_root32, threshold:u16,
pending32, rotation_begin:u64, rotation_end:u64, rotation_effective_sequence:u64,
rotation_revision:u64, recovery_revision:u64, recovery_delay_seconds:u64,
recovery_maximum_seconds:u64, rotation_delay_ms:u64, rotation_maximum_ms:u64,
execution_sequence:u64
```

Event `0x7145` carries the 209-byte `LXGS2` session summary: grant ID, grantor
DID hash, owner public key, action key, session key, expiry sequence, module
mask, ordinal minimum and maximum, not-before and not-after milliseconds, and
revocation sequence. Hashes and keys are 32 bytes, ordinals are u16 and other
numbers are u64. Grant bytes are emitted in chunks of at most 256 bytes through
`0x7105` and `0x7125`; revoke emits `0x7106`.

## Retained hosted provisioning material

`services/platform/hosted/human/provision.py` validates
`$WORK_DIR/human-evidence-input/owner-registration.json`. Its top-level fields
are exactly `owner_account`, `authority` and `identity`. The account is a
nonzero lowercase H32 identifier; the authority is text. `identity` carries
`did`, `authorities`, `revocation_sequence`, `frozen`, `evidence`,
`capabilities`, `rotation` and `recovery`.

Evidence references have exactly `{activity_id, receipt_digest}`, both nonzero
lowercase H32 values. When an activity set or owner DID is supplied to the
validator, references must belong to that activity set and the DID must match.
Rotation and recovery records contain `policy_revision`,
`required_delay_seconds`, `maximum_delay_seconds`, `effective_sequence` and
`evidence`. Validation checks their shape and bounds; it does not submit a
transaction, authenticate chain state or establish finality.

The producer selected by `provision.py --produce-owner-registration --work-dir PATH`
still calls `owner_native.py`, which builds socket-based CLI arguments and
expects an `acknowledged` native activity response. These are pre-merge inputs
and do not match the current `layerxctl --rpc-url` commands above. Its
`owner-native.json`, `owner-admission.txt`, `custody-credit.bin` and receipt
authority setup must not be treated as current network registration steps.
The old custody precompile at `0x0000000000000000000000000000000000001013`
is retired and refuses calls with `LXP_MODULE_DISABLED`; a Bridge Credit is
not a registration prerequisite for the merged network. Preserve any existing
`owner-native-run` directory and partial output for reconciliation after an
interrupted legacy run.

The guardian document format remains useful for checking retained recovery
material. `recovery-guardians.json` and `recovery-guardian-bindings.json` contain
`version`, `threshold`, `public_keys` and signed `members`. Each member has
`role`, operator `identity`, `public_key`, `epoch`, `custody`, `signature` and
`rotation`. `owner_native.guardian_set` verifies bindings and predecessor
signatures, distinct roles, identities, keys and custody directories, and the
manifest's agreement with the verified keys. The legacy role names
`guarantor-1`, `guarantor-2` and `sequencer` identify guardian records only;
they do not establish consensus finality.

The recovery commitment is SHA-256 of `LX:HUMAN:RECOVERY:v1` followed by NUL,
threshold u16be, guardian-count u16be and sorted public-key32 values. Threshold
must not exceed the guardian count. Binding signatures use
`LX:HUMAN:GUARDIAN:BINDING:v1` plus NUL, epoch u16be, role-length u16be, role,
identity32 and public-key32. Rotation signatures use
`LX:HUMAN:GUARDIAN:ROTATION:v1` with the same prefix fields, outgoing key32 and
successor key32. Seeds remain private in per-guardian mode-0700 directories
with mode-0600 seed files. [The provisioning details](owner-registration-details.md)
retain the enrollment and rotation document descriptions; their pre-merge
bootstrap and checkpoint instructions are historical.

## Hosted provisioning tests

`make human-test-hosted-provisioning` builds `layerx-human-identity-provider`
from `services/human/Cargo.toml` into `$(HUMAN_TARGET_DIR)/debug`, checks that
the binary is executable, and runs
`python3 -m pytest services/platform/hosted/human -q` through
`tools/runtime/run-with-clock.sh`. It supplies the built binary through
`LAYERX_HUMAN_IDENTITY_PROVIDER_BIN`. `HUMAN_TARGET_DIR` defaults to
`CARGO_TARGET_DIR` when set, otherwise `services/human/target`, and can be
overridden on the make command line.

The target checks for `pytest`, `PyYAML`, `cryptography`, `Crypto.Hash.keccak`,
`eth_account` and `eth_utils`; the bridge dependencies are declared in
`tests/bridge/requirements.txt`. It is a prerequisite of `make human-test`.
These are hosted provisioning checks, not evidence of a live deployment or
successful registration against the merged network.
