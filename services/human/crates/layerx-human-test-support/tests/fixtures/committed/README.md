# Committed evidence recording

`vector-v4-committed.json` is fresh output from the repository's
`daemon/committed-fixture` recorder at source revision
`44823b0a8346dd793d93d2e438a7b4688d933eda` with the recorder changes from task `22.31.486`. It starts the production `paxd` nodes
with an isolated `committed-fixture` chain (EVM chain id 125, EIP-1559 type 2), executes a
ledger payment, and captures the receipt, native sequence-batch transaction,
inclusion proofs, linked execution/successor headers, commits and validators.
The receipt inclusion envelope is assembled and signed by the recorder under
the local genesis receipt pin; the consensus evidence comes from the nodes.
These are local fixture authorities, not public-chain trust roots.

`vector-v4-conformance.json` is derived from those exact bytes by the existing
interop converter. Its `provenance.sourceSha256` pins the complete original
recording. The Rust helper retains the source-digest and receipt-identity checks
before replaying the receipt over loopback HTTP through `NodeRpc`, verifying
the consensus and inclusion evidence, persisting it in `PrincipalStore`, and
reopening and verifying the recovered evidence. Altered and foreign evidence
remain refusal cases.

The recording SHA-256 is
`76cfee5540d369ad93286723b81117ec90ca85f12b7b074afffb78456a4cc3c6`;
execution height 10 is linked to successor commitment height 11.

Regenerate during implementation from the repository root:

```sh
make -j4 paxd-native-libs
go build -p 4 -o /tmp/repair223355-paxd ./daemon/paxd
python3 services/human/crates/layerx-human-test-support/tests/support/record_committed_evidence.py --record /tmp/repair223355-paxd
python3 services/human/crates/layerx-human-test-support/tests/support/derive_committed_evidence.py
```

The recording launcher provisions a temporary CA, separate primary/standby
certificates and peer SPKI pins for the production replication mTLS checks.
It passes these settings to the unmodified `paxd` binary; certificate keys are
deleted after capture and are never part of the saved recording.

The Cargo verification uses only the saved recording and vector. It requires
no running node, Go toolchain, native build, credentials or live endpoint.
