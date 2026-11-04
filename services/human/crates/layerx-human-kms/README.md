# Human production KMS provider

`layerx-human-kms` serves LXKP over mandatory mutual TLS. It verifies the client
CA and pins the client leaf certificate. Its software store seals Ed25519
custody seeds and action state with AES-256-GCM, holds an exclusive process lock,
and syncs atomically replaced snapshots. EVM signing uses configured threshold
attestor wallets on chain **125**; this provider does not hold their secp256k1
private keys. It is a software custody provider, not a hardware KMS certification.
The authenticated [Human service](../layerx-human-service/README.md) remains
responsible for principal authorization, step-up and account policy.

## Configuration

The executable takes no arguments. Build from the repository root:

```sh
cargo build --locked --manifest-path services/human/Cargo.toml -p layerx-human-kms
```

The registry file contains this complete chain snapshot (the attestor's
`network_id` is a separate field, not a replacement registry key):

```json
{"chain_id":125}
```

The following variables use the `LAYERX_HUMAN_KMS_` prefix:

| Suffix | Requirement |
| --- | --- |
| `LISTEN` | TCP socket address |
| `STATE_DIR` | Absolute provider-owned private state directory |
| `PROVIDER_REFERENCE` | Nonempty provider identity, at most 256 bytes, without NUL |
| `DEADLINE_SECONDS` | Required integer 1–60 for socket I/O timeouts |
| `REGISTRY_FILE` | Protected JSON file containing exactly `{"chain_id":125}`; unknown fields and other chain ids are refused |
| `CLIENT_CA_DER` | DER client CA certificate |
| `CLIENT_CERT_DER` | DER leaf certificate pinned for the Human service |
| `TLS_CERT_DER`, `TLS_KEY_DER` | DER server certificate and private key |
| `SEAL_SECRET_FILE` | Exactly 32 random binary bytes; keep separately from the state volume |
| `EVM_CLIENT_CERT_DER` | Optional distinct leaf certificate for the restricted executor identity |
| `ATTESTOR_CONFIG_FILE` | Principal-bound attestor configuration, required for wallet resolution and EVM signing |

Configuration files must be absolute regular files owned by the provider UID.
Keys, seal, attestor configuration and assertion files must have no group/other
permissions; public certificate and registry files must not be group/other
writable. The executable also requires `LAYERX_RUNTIME_CLOCK_SOCKET`,
`LAYERX_RUNTIME_CLOCK_PID` and `LAYERX_RUNTIME_CLOCK_UID` for the authenticated
clock supervisor. Its socket and parent directory must be private to the stated
UID, and the peer PID/UID must match. See the [workspace guide](../../README.md)
for the development clock wrapper.

The attestor configuration is strict JSON with a top-level `bindings` array.
Each entry contains `binding` (32 byte integers), `tenant`, `principal`, `config`
and `assertion_file`. The custody binding must be nonzero and unique; the EVM
address and EVM key id must each be unique across bindings. `config` is the
[`AttestorSignerConfig`](src/attestor.rs) structure:

| Field | Value |
| --- | --- |
| `nodes` | Entries with `id` and TCP `address` |
| `trust_anchors` | Paths to DER trust certificates |
| `client_certificate_chain` | Paths to DER client certificates |
| `client_private_key_pkcs8` | Protected DER private-key path |
| `key_id`, `public_key` | Attestor Ed25519 identity; public key is 32 byte integers |
| `evm_key_id`, `evm_address` | Bound EVM wallet identity; address is 20 byte integers |
| `signers` | Selected signer ids; minimum quorum is three |
| `network_id` | 125 |
| `timeout_seconds` | 1–60 |

`assertion_file` supplies the owner assertion sent with EVM signing requests.
The file is read and trimmed when the store opens; the running process does not
refresh it. Renew the assertion through its issuer and restart the provider to
load it. Provision a replacement assertion before its expiry, install it as a
provider-owned private regular file, then restart with the same state and seal.
An empty or unreadable assertion, or an invalid configured credential, prevents
the store from opening. The standalone `EVM_ASSERTION_FILE` variable is not a
source for this principal-bound store: use each binding's `assertion_file`.
Without `ATTESTOR_CONFIG_FILE`, custody-only startup is possible, but wallet
resolution and signing are unavailable; reopening state with a retained wallet
requires its matching attestor binding. The provider does not generate owner
assertions or substitute a local EVM key.

## LXKP versions and operations

Requests and responses have a four-byte big-endian body length prefix. The body
starts with `LXKP`, a big-endian u16 version and a u8 operation. Requests then
carry a length-prefixed provider reference. Except for probes, the common
custody fields are binding (32 bytes), network (u32), key class (u8) and a
length-prefixed opaque reference. Class 1 is the human primary key; class 2 is
an agent primary key. Create requires an empty reference; other custody
operations require the existing reference. Frames are bounded to 2 MiB.

| Version | Operation | Contract |
| --- | --- | --- |
| 1 | 0 probe | Provider readiness |
| 1 | 1 create, 2 describe, 4 destroy | Custody key lifecycle |
| 2 | 3 rotate | Append the expected current 32-byte public key |
| 7 | 5 Sign | Digest, 20-byte sender, length-prefixed unsigned EIP-1559 transaction and disclosure; returns 65-byte `r \| s \| v` with v 27/28 |
| 3 | 6 wallet | Resolve the configured attestor wallet address |
| 3 | 7 authorize | Register an exact principal-bound EVM plan and reserve its nonce |
| 3 | 8 sign, 9 acknowledge, 10 recover | Sign a registered action, acknowledge its exact transaction hash, or recover the journaled action |
| 3 | 11 authorize send | Register and sign the exact bank-send transaction and disclosure; returns signed type-2 transaction bytes |
| 3 | 12 external signature | Verify a supplied signature for an already authorized action and journal its signed transaction |
| 6 | 15 executor probe | Echo a nonzero challenge and network id; restricted executor certificate only |
| 8 | 14 export, 16 acknowledge export | Recipient-encrypted primary-key handover and proof of recipient recovery |

Version-1 rotate is decoded but refused. Version-1 Sign, the retired recipient
authorization operation 13, and all unlisted version/operation pairs are refused.
Operations 7–12, 14 and 16 carry their operation-specific length-prefixed JSON
payloads; operation 6 carries none. Exact layouts are in [wire.rs](src/wire.rs),
[evm_types.rs](src/evm_types.rs), [send.rs](src/send.rs) and [export.rs](src/export.rs).

Only the service certificate can create, rotate, destroy, authorize plans,
invoke Sign, authorize bank sends or export keys. The separate executor
certificate permits operations 6, 8–10, 12 and its dedicated probe, so it cannot
register a new signing authorization.

Responses echo the version and operation, followed by a status byte and any
success data. Statuses are 0 success, 1 `Refused`, 2 `NotFound`, 3 `Conflict`,
4 `Unavailable`, 5 `Integrity` and 6 `SelfCustodied`. Framing and TLS failures
can close the connection before an operation response is available. In
particular, operation 13 is refused by the decoder before dispatch: callers
must handle a closed connection rather than expect a status-1 reply.

## Signing and recovery

Version-7 Sign reconstructs the call disclosure and checks the exact signing
digest and sender. Its nonce reservation is persisted before contacting the
attestor. It returns a signature, rather than a signed transaction journal entry.
The version-3 action path additionally binds principal, tenant, custody binding,
wallet, plan, stable action key, validity window and every transaction field.
Nonces are durably reserved per wallet and chain; a conflicting or regressing
reservation is refused, while an exact retry can reuse its reservation.

Operation 11 checks a zero-value call to the bank precompile
`0x0000000000000000000000000000000000001001`. Recipient, denomination and amount
must match its disclosure. Authorization and nonce are committed before remote
signing, and the signed bytes are persisted before success. It does not return
a detached native SEND authorization signature.

Operations 8 and 12 retain canonical signed transaction bytes and their Keccak
hash. Operation 12 checks low-s encoding, recovered wallet and the exact
EIP-1559 preimage, accepting parity 0/1 or 27/28. Recovery permits resubmission
after an acknowledgement gap; acknowledgement requires the journaled hash.
An admission acknowledgement alone does not prove execution or block commitment.

Rotation persists the new key and immediately previous public key together.
A retry naming that previous key returns the committed description after restart;
a next rotation names the current key. Older expectations conflict. Create is
idempotent before rotation; rotated or destroyed identities cannot be recreated.
Destroy clears the seed, wallet and send journal, retaining a tombstone.

Back up the state directory and keep the seal secret separately. Reopening
requires that seal, the same provider reference and chain snapshot, and matching
attestor identities for retained wallets. A missing seal or failed state
authentication is an error, not permission to start with empty state. The
provider removes a validated leftover `state.next` when reopening and loads the
committed `state.aead`; do not reset tombstones, nonce reservations or action
journals to retry a request.

## Primary-key export

Version-8 export accepts an `ExportCeremony` for a class-1 record and returns an
`ExportEnvelope` encrypted to the recipient. It atomically marks the key
self-custodied, removes the seed and clears wallet/send state before replying.
The same ceremony can retrieve the exact retained envelope after a lost response
or restart; a different ceremony conflicts. Acknowledgement proves recovery of
the envelope and removes the retained envelope. Later custody mutations and
signing are refused for the self-custodied identity. Raw private-key bytes are
never returned by LXKP. This exports the Ed25519 custody seed, not an attestor
secp256k1 private key. Preserve the recipient secret and exact ceremony before
requesting export. Persist the recovered seed at the recipient before sending
acknowledgement: after acknowledgement, the provider cannot return the envelope.
Acknowledgement is idempotent, including after restart. Removing a seed from the
current snapshot does not erase prior backups; protect and retire those under
the operator's backup policy.

The exported [`attestor`](src/attestor.rs) library also implements pinned-TLS
quorum signing, generation, wallet lookup and native consent operations. Those
client APIs are separate from the LXKP version/operation table above.

## Client examples and qualification

These fragments use the Human service's `RemoteKmsProvider` with its service
mTLS identity, `KmsProvider` in scope, and an existing principal `binding` and
provider `reference`. Supply a chain-125 `Eip1559Call` and derive its disclosure
with `layerx_crypto::disclosure::call::bind_call`; never reuse a nonce for a
different transaction. Version-7 signing returns a detached signature:

```rust
let wallet = remote.wallet_descriptor(&binding, reference)?;
let disclosure = bind_call(&call, wallet.evm_secp256k1)?;
let signature = remote.sign_wallet_call(&binding, reference, &call, &disclosure).await?;
```

For bank SEND, serialize `SendAuthorization` from [send.rs](src/send.rs):
`authorization` is the complete `EvmPlanAuthorization`, and `disclosure` contains
`recipient`, `denom` and the 32-byte big-endian `amount`. Its zero-value bank call
must encode `send(address,address,string,uint256)` with the wallet as sender.
The low-level client returns signed transaction bytes; recover the action with
the same 32-byte action key after a response gap:

```rust
let payload = serde_json::to_vec(&authorization)?;
let raw = remote.evm_operation(11, &binding, reference, &payload)?;
let action_key = serde_json::to_vec(&authorization.authorization.action_key)?;
let recovered = remote.evm_operation(10, &binding, reference, &action_key)?;
```

Decode `recovered` as `EvmAction`; its `raw_transaction` equals `raw` and its
`transaction_hash` is the Keccak hash of those bytes. Use operation 9 only with
that exact action key and hash.

For handover, create an `ExportRecipient` from a securely generated valid secret
scalar and retain it at the recipient. Generate a nonzero ceremony id once and
persist the ceremony. Reuse both on retry, including after provider restart:

```rust
let envelope = remote.export_envelope(&binding, reference, &ceremony)?;
let (seed, acknowledgement) = recipient.open(provider, binding.digest(), reference.as_bytes(), &envelope)?;
```

After securely retaining `seed`, finish with:

```rust
remote.acknowledge_export(&binding, reference, &acknowledgement)?;
```

[repair_22_33_31.rs](tests/repair_22_33_31.rs) executes these client sequences
against the production TLS handler, codec and durable store. It also checks the
registry and credential lifecycle, retired encodings, signed bank calldata and
sender, and recovery across store restarts. Its committed loopback signing peers
use independent fixture keys and the production EIP-1559 encoder; it needs no
live attestor cluster or external credential. It qualifies the KMS/client
contract, not deployment of the threshold cluster.
