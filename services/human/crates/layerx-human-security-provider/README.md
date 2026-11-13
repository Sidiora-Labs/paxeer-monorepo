# LXSP security provider contract

The library and `layerx-human-security-provider` binary serve the LXSP client in
the [Human service](../layerx-human-service/README.md), implemented in
`src/server/production_auth.rs` there. The provider manages TOTP methods and
reveals locally verified recovery receipts. Discovery, principal authorization,
step-up and capability ownership remain in the Human service.

## Framing and encodings

Each call opens a new Unix stream and sends one request. Both directions begin
with a four-byte unsigned big-endian payload length, excluding that prefix.
The request payload is `LXSP`, version byte `1`, operation byte, a four-byte
big-endian field count, then fields. Each field is a four-byte big-endian byte
length followed by exactly that many bytes. Integers carried as fields still
have the field-length prefix: u32 has length 4, u64 has length 8. Strings are
UTF-8, without terminators. Optional u64 is either an empty field or eight bytes.

The response has the same structure, replacing operation with result code.
Code 0 means success; code 1 maps to `SecurityBoundaryError::Refused` immediately
(the client does not decode the remaining refusal payload). Other result codes,
wrong magic/version, malformed successful fields, trailing bytes in a successful
payload, and invalid response lengths map to `InvalidEvidence`. Connection,
timeout configuration, read and write failures map to `Unavailable`.

Client configuration requires an absolute socket path, nonzero deadline and
`maximum_frame_bytes` in 64..=1,048,576. The payload, excluding its outer prefix,
must fit this maximum. Outgoing oversize requests are refused locally. Incoming
zero-length or oversized responses are invalid evidence. Decoded text fields
must contain 1..=4096 UTF-8 bytes. Request strings are not similarly validated
by `call`; the server must validate them. There is no request ID or multiplexing.

## Operations

Here `p` is the UTF-8 `PrincipalId::as_str()`, `now` is a u64 field, and all
unspecified scalar fields are UTF-8. Response lists below exclude framing.

| Opcode | Request fields | Successful response fields |
| --- | --- | --- |
| 0 probe | none | none |
| 1 status | p | status projection |
| 2 begin_setup | p, label, now | setup_id, secret, secret_remask_at:u64, otpauth_uri, uri_remask_at:u64, expires_at:u64 |
| 3 finish_setup | p, setup_id, code, now | method_id, label, enabled_at:u64, last_used_at:optional-u64, backup_remask_at:u64, one or more backup codes |
| 4 disable | p, method_id, now | status projection |
| 5 rotate_backup_codes | p, now | backup_remask_at:u64, one or more backup codes |
| 6 reveal_verified_receipt | p, evidence_id, now | canonical_receipt, remask_at:u64, single byte 0x01 |

A status projection is `backup_codes_remaining:u32, method_count:u32`, followed
by exactly `method_count` groups of four fields: method_id, label, enabled_at:u64,
last_used_at:optional-u64. No other LXSP operations exist in this client.

`TimedSecret` and `BackupCodeSet` require remask_at strictly greater than the
request's now. Empty secrets or backup code lists are invalid. Setup secret and
receipt are copyable; otpauth URI is not. The client decodes setup expires_at but
does not enforce its relation to now. Setup lifetime, TOTP algorithm and period,
clock-skew window, code attempt limits, backup-code consumption, and recovery
record ingestion are not defined by the LXSP client. `RecoveryEvidenceProvider`
requires local verification of stored receipts before returning canonical bytes;
a stored boolean alone does not meet that requirement.

## Authenticator policy

TOTP uses a random 160-bit secret, HMAC-SHA1, six digits, a 30-second period,
and a ±1-period window. Setup expires after 300 seconds; equality is accepted.
A principal has one pending setup; beginning another replaces it. A setup admits
five code attempts, durably counted even when refused, and successful completion
consumes it. Requests cannot move an account's mutation clock backwards.
Method secrets and the accepted TOTP counter are retained. The current wire has
no login-code or backup-code consumption operation, so last_used_at remains
absent. Each completion or rotation generates ten independent 160-bit backup
codes, returned once; only SHA-256 digests are retained. Removing the last method
clears backups. Maximums: 1024 accounts, 16 methods per account, 256-byte labels.
All disclosures remask after 60 seconds; timestamp overflow is refused.

## Configuration

| Environment variable | Requirement |
| --- | --- |
| `LAYERX_HUMAN_SECURITY_PROVIDER_STATE_ROOT` | Required absolute provider-owned state directory; created mode 0700 if absent, parent must exist. |
| `LAYERX_HUMAN_SECURITY_PROVIDER_TRUST_HISTORY` | Required absolute protected binary chain `TrustRoot` file, mode 0600, owned by provider UID. |
| `LAYERX_HUMAN_SECURITY_PROVIDER_SOCKET` | Required absolute UDS path, for example `/run/layerx/human/security.sock`. Existing paths are refused. Parent must be owned by provider UID and not group/other writable. |
| `LAYERX_HUMAN_SECURITY_PROVIDER_ALLOWED_UID` | Required decimal UID of the human service; no default. Checked using Linux SO_PEERCRED. |
| `LAYERX_HUMAN_SECURITY_PROVIDER_DEADLINE_SECONDS` | Optional integer 1–60, default 5; total request/response I/O deadline. |
| `LAYERX_RUNTIME_CLOCK_SOCKET` | Required authenticated clock socket; socket and parent directory must be private to the clock UID. |
| `LAYERX_RUNTIME_CLOCK_PID` | Required nonzero decimal supervisor PID, checked against SO_PEERCRED. |
| `LAYERX_RUNTIME_CLOCK_UID` | Required decimal supervisor UID, checked against ownership and SO_PEERCRED. |

Run the binary without arguments to serve. It connects to the authenticated
clock supervisor before serving; missing or mismatched clock configuration
refuses startup. The development wrapper is described in the
[workspace guide](../../README.md). Probe and ingestion do not require clock
bindings. The Human client uses `LAYERX_HUMAN_SECURITY_SOCKET`,
`LAYERX_HUMAN_SECURITY_DEADLINE_SECONDS` and
`LAYERX_HUMAN_SECURITY_MAX_FRAME_BYTES` (normally 1048576).

The socket is 0660. The human service also needs filesystem access via the
socket's owner/group and parent directories. The listener handles one active
connection at a time, with an OS-bounded backlog, frames at most 1 MiB, at most
four request fields and at most 4096 bytes per field. One request per connection.
SIGTERM and SIGINT stop accepting and interrupt pending I/O within 100 ms;
shutdown removes only the socket inode this process created. A stale socket
following SIGKILL requires operator removal after checking no provider owns it.

## Durable file layout

All state files are regular, single-link, provider-owned 0600 files. Paths with
symlink components are refused. `writer.lock` is held with an exclusive flock
for the process lifetime, including administration. `snapshot.json` is a
canonical initial empty snapshot. `initialized` commits its SHA-256 digest.
`00000000000000000001.json` and subsequent numbered files are canonical state
records with sequence and previous-record digest. `head` is exactly 40 bytes: the latest
sequence as a big-endian u64 followed by the 32-byte SHA-256 record digest. It
detects a missing journal tail. `transaction.tmp` is the
exclusive temporary file used by atomic rename. Files are fsynced before rename;
the state directory is fsynced afterward. Initial root creation fsyncs its parent.

Startup replays every record, validates bounds and account clocks, verifies all
stored recovery proofs, and checks snapshot, marker, chain and head consistency.
On reopen, a protected `transaction.tmp` (regular, single-link, provider-owned
0600, at most 16 MiB) is removed only after the retained state and head relation
are validated; its contents are never applied as a record. If the journal contains exactly one fully valid record beyond the
published head and that head equals the preceding record's sequence and digest,
startup republishes the head to the journal tip. This handles a crash after
journal rename and before head publication; it does not discard that record.
A gap of two or more records, an incorrect preceding digest, other head
mismatches, missing records, unexpected files or invalid proofs are refused
without cleaning up the pending file. A partially initialized directory is not silently reset.

Each live operation rechecks consistency without startup repair. A failed
consistency check or durable write withdraws readiness for that store instance;
reopen it to replay and perform only the bounded recovery above. Inconsistency
outside those recovery cases requires restoration of a consistent backup.
Limits are 4096 transactions and
16 MiB per record, after which mutation is refused. No automatic compaction is
provided. Permissions protect secret material; hashes detect corruption, not
malicious rewriting of the whole state by its privileged owner. Rollback of an
entire consistent backup cannot be detected without an external monotonic anchor.

## Probe

```sh
layerx-human-security-provider probe
```

The probe sends LXSP operation 0 to the running provider at SOCKET and exits 0 only when it answers ready within DEADLINE_SECONDS (default 5). It reads only SOCKET and DEADLINE_SECONDS.

## Recovery administration

The administration path is `ingest_recovery_receipt` in [state.rs](src/state.rs).
It decodes and verifies the complete node evidence before opening the persistent
store. Ingestion never accepts a receipt signature or an inclusion proof alone.

Stop the provider (administration takes the same exclusive writer lock), then:

```sh
layerx-human-security-provider ingest-recovery-receipt /protected/recovery.json
```

This command needs `LAYERX_HUMAN_SECURITY_PROVIDER_STATE_ROOT` and
`LAYERX_HUMAN_SECURITY_PROVIDER_TRUST_HISTORY`. Run as the state owner. The receipt
file must be a protected, single-link 0600 regular file, bounded to 32 KiB.
The exact outer JSON shape, in serialization order, is:

<!-- recovery-json -->
```json
{"version":2,"principal":"alice","evidence_id":"recovery-1","canonical_receipt":"BASE64_RECEIPT","header":"BASE64_HEADER","commit":"BASE64_COMMIT","transaction_proof":"BASE64_EVIDENCE_JSON"}
```

The displayed binary values are shape labels, not usable proof material.
Generate compact JSON using `RecoveryReceipt` and `serde_json::to_vec`; the
administrative decoder requires byte-for-byte canonical reserialization, with
no whitespace, final newline or unknown fields. Version 1 is refused. Binary
strings use canonical padded standard base64. `canonical_receipt` is canonical
protocol receipt encoding, bounded by the client's 4096-byte text limit.
`header` and `commit` encode the committed successor block's chain header and
consensus commit, rather than a signed batch header.

`transaction_proof` is base64 of compact canonical JSON serialized from
[`RecoveryTransactionEvidence`](src/recovery.rs), not a bare Merkle proof.
`RecoveryReceipt::with_transaction_evidence` sets version 2 and encodes that
nested object. The nested JSON shape, also in serialization order, is:

<!-- recovery-evidence-json -->
```json
{"containing_transaction":"BASE64_BATCH_TRANSACTION","transaction_proof":"BASE64_BATCH_PROOF","execution_header":"BASE64_EXECUTION_HEADER","execution_commit":"BASE64_EXECUTION_COMMIT","activity_ids":[[42,42,42,42,42,42,42,42,42,42,42,42,42,42,42,42,42,42,42,42,42,42,42,42,42,42,42,42,42,42,42,42]],"receipt_index":0,"raw_transaction":"BASE64_SIGNED_TRANSACTION","receipt_keys_proof":"BASE64_RECEIPT_KEYS_PROOF","result_code":0,"result_data":"BASE64_RESULT_DATA","gas_wanted":100000,"gas_used":1,"result_proof":"BASE64_RESULT_PROOF","recovery_did":[9,9,9,9,9,9,9,9,9,9,9,9,9,9,9,9,9,9,9,9,9,9,9,9,9,9,9,9,9,9,9,9],"recovery_root":[7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7,7],"recovery_threshold":2}
```

Here the repeated integer arrays illustrate 32-byte binary identities; replace
`activity_ids`, DID, root, threshold, receipt index and gas values with the
values from the committed recovery transaction and result. Replace every
`BASE64_...` label with canonical proof material. These labels cannot be ingested
as evidence. No fields are optional, and nested JSON has the same canonical
reserialization requirement as the outer object. The nested fields mean:

| Fields | Encoding and purpose |
| --- | --- |
| `containing_transaction`, `transaction_proof` | Base64 sequence-batch transaction and transaction inclusion proof for the execution block |
| `execution_header`, `execution_commit` | Base64 header and commit for that execution block |
| `activity_ids`, `receipt_index` | Ordered array of 32-byte integer arrays and u32 index selecting the receipt's activity |
| `raw_transaction`, `receipt_keys_proof` | Base64 signed EIP-1559 transaction and receipt-key commitment proof at the successor header |
| `result_code`, `result_data` | u32 result code and base64 transaction result data |
| `gas_wanted`, `gas_used`, `result_proof` | Signed i64 gas values and base64 result inclusion proof |
| `recovery_did`, `recovery_root`, `recovery_threshold` | Two 32-byte integer arrays and nonzero u16 threshold bound to the successful recovery-registration call |

The verifier checks both block commits, their adjacent-height parent linkage,
the sequence-batch inclusion and receipt position, sequence and state-root
transitions, authenticated receipt keys, receipt signature, matching signed
transaction and successful result proof. It also checks the recovery call to
the auth precompile at `0x000000000000000000000000000000000000101c` and its
successful return data. The protocol receipt must belong to the EVM module
(id 12), name recovery registration (operation 2), and have result code zero; the
committed transaction result must also have code zero. DID and recovery root
must be nonzero, and the nonzero threshold must match the signed call.
These are proofs from blocks in the same network;
EVM chain id 125 is not a separate settlement domain.

## Trust root and replay verification

Provision the trust file with the chain-proof library's
`TrustRoot::canonical_bytes()` format (`LXTR` version 1). Its fields are the
consensus chain-id string, height, time in milliseconds, header hash and validator
set with addresses, Ed25519 public keys and voting powers. Obtain the root from
an independently authenticated chain head and validator set; this provider
cannot establish trust from caller-supplied receipt material. Do not substitute
JSON or a receipt-key history file. In a trusted provisioning process, call
`TrustRoot::from_head(&trusted_head, trusted_validators)` using a head and validator
set authenticated independently of the ingestion request, then write
`canonical_bytes()` without a newline to a new provider-owned 0600 file. The
parent directory must already exist, and neither the path nor its components
may be symlinks. The trust file, like other protected inputs, is bounded to
16 MiB. See
[trust_store.rs](../../../../programs/crates/layerx-programs-registry/src/chain_proof/trust_store.rs)
from the repository's `programs` workspace.

The provider decodes that file with `TrustRoot::decode` and constructs
`ChainProofVerifier` with its chain id and validators. This path uses that fixed
validator set; it does not run a light-client validator-update process or enforce
a current-head freshness window. Supply a trusted set that can authenticate the
proof's headers. Stored receipts are reverified during journal replay and on
reveal, so trust-file changes can make old material unreadable.

The trusted administrator binds the principal and evidence id to the proof;
that local association is not asserted by the protocol receipt. Identical
re-ingestion is idempotent; changing material for an existing pair is refused.
Maximums are 1024 receipt principals and 64 receipts per principal. Reveal returns
the exact stored canonical receipt base64 string after verification, not a
stored success boolean. Ingestion is administrative only; LXSP operations remain
0–6. See [test support](../layerx-human-test-support/README.md) for the distinction
between submission fixtures and committed proof evidence.

## Provisioning and ingestion example

The following Rust helper writes the binary trust root and compact receipt JSON
to **new** protected files and calls the same administration entry point as the
binary. Supply an independently authenticated `TrustRoot` and a `RecoveryReceipt`
assembled from node evidence using `with_transaction_evidence`; do not derive
trust from that receipt. Run as the provider UID with a restrictive umask such
as 0077, in an existing protected parent directory, while the provider is stopped.
Existing files cause an error; the helper does not replace provisioned trust.
The CLI command above consumes these same two file formats.

<!-- administration-rust -->
```rust
fn administer_recovery(
    state_root: &std::path::Path,
    trust_path: &std::path::Path,
    receipt_path: &std::path::Path,
    trusted_root: &layerx_programs::chain_proof::trust_store::TrustRoot,
    receipt: &layerx_human_security_provider::RecoveryReceipt,
) -> Result<(), Box<dyn std::error::Error>> {
    use std::io::Write as _;
    use std::os::unix::fs::OpenOptionsExt as _;

    for (path, bytes) in [
        (trust_path, trusted_root.canonical_bytes()?),
        (receipt_path, serde_json::to_vec(receipt)?),
    ] {
        let mut file = std::fs::OpenOptions::new()
            .write(true)
            .create_new(true)
            .mode(0o600)
            .open(path)?;
        file.write_all(&bytes)?;
        file.sync_all()?;
    }
    layerx_human_security_provider::ingest_recovery_receipt(state_root, trust_path, receipt_path)?;
    Ok(())
}
```

The integration test [repair_22_33_54.rs](tests/repair_22_33_54.rs) compiles this
helper, checks it against this text, fills the JSON examples using signed,
committed in-process chain evidence, ingests through the production decoder,
and reveals the receipt through the production Unix server and Human client
before and after restart. It also proves the one-record head repair, pending
file cleanup and refusal of a larger gap without deleting the pending file.
