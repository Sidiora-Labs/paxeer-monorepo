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
load it. An absent binding cannot resolve or sign a wallet; the provider does
not generate owner assertions or substitute a local EVM key.

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
can close the connection before an operation response is available.

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

## Primary-key export

Version-8 export accepts an `ExportCeremony` for a class-1 record and returns an
`ExportEnvelope` encrypted to the recipient. It atomically marks the key
self-custodied, removes the seed and clears wallet/send state before replying.
The same ceremony can retrieve the exact retained envelope after a lost response
or restart; a different ceremony conflicts. Acknowledgement proves recovery of
the envelope and removes the retained envelope. Later custody mutations and
signing are refused for the self-custodied identity. Raw private-key bytes are
never returned by LXKP.

The exported [`attestor`](src/attestor.rs) library also implements pinned-TLS
quorum signing, generation, wallet lookup and native consent operations. Those
client APIs are separate from the LXKP version/operation table above.
