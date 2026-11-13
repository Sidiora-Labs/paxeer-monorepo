# Runtime crate

This page is the authoritative contract for `terminal::decode_terminal_payload`,
execution replay, and trace runner attestations. It is included in the
terminal module's Rust documentation.

## Encodings and refusal fields

Canonical integers use network byte order. Every length is checked before its
payload is read; truncation, trailing bytes and unallocated discriminants refuse.
The `LXP/program-execution/v2\0` and `v3\0` terminal envelopes are the frozen
ABI 1 ordinary and traced layouts. `v4\0` is the ABI 2 terminal;
`v5\0` carries ABI 3 through 5. Envelope versions and guest ABI versions are
separate version domains. The terminal kind must match success, failure or
resource exhaustion, and the supplied ABI must match the encoded ABI.

`LXP/programs/failure-detail/v1\0` carries a one-byte category and a u32-sized
category payload. Composition category 2, subtag 2 (`WrongVersion`), contains
two one-byte allocated ABI revisions, expected then actual. Allocated revisions
are 1, 2, 3, 4, 5; an unallocated value is `Malformed`. A valid refusal remains a
failure receipt. Malformed bytes, mismatched kind and mismatched ABI return
`TerminalDecodeError`, never successful execution. Occupancy, transfer authority
and sandbox settlement wrappers are decoded strictly without permitting duplicate
or reordered nested wrappers.

## Recovery and evidence

`RecordedExecution` pins runtime, ABI, fee schedule and metering schedule along
with immutable Wasm, export and arguments. The historical catalog admits ABI
1 through 5 with the declared resource budget and Wasmi 0.31.2 metering policy.
Replay creates a fresh engine and instance. The ordinary replay entry point
requires the historical genesis prices; the fee-history entry point selects
exactly the recorded append-only governed schedule, even after a newer schedule
has been appended. Unknown runtime, ABI, fee or metering versions refuse with
`ReplayRefusal`; replay never selects the current head as a substitute.
Callers retain authenticated inputs, schedule history and canonical evidence
across a restart and compare the recovered execution's evidence byte for byte.
The runtime replay dispatcher itself is stateless and does not own a journal.

## Trace runner attestations

`LXP/trace-runner-artifact/v1\0` encodes three 32-byte identities (executable,
build manifest, workload), a u16-sized nonempty platform identity, u64-sized
canonical observation and a 64-byte Ed25519 signature. Observations are bounded
at 128 MiB consistently in the producer, encoder and decoder. Runner executable
and build-manifest inputs each have a separate 128 MiB host provenance bound.
The complete artifact bound includes all fields, up to 65535 platform bytes,
and the signature. Workload identity includes module, export, typed arguments,
trace policy and runtime version under its domain separator.

The attestation digest is host SHA-256 of the canonical artifact excluding the
signature. This preserves its v1 preimage while allowing every declared artifact
size. Host hashing is separately bounded; guest `MAX_HASH_INPUT_BYTES` remains
1048576 bytes. The differential verifier rehashes submitted runner and manifest bytes,
checks pinned platform and provenance identities, and verifies the signature.
It requires distinct platforms and signing keys, identical workload identities
and identical canonical observations. Any failed check refuses the comparison.
