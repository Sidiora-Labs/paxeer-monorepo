# SDKs

`sdk/platform/pipeline.kvx` records the shared `agent-api`, `human-api`, and
mirror schemas, their generators, and the Rust, TypeScript, Python, Go, JVM,
Swift, and C# outputs. Language idioms differ; shared wire shapes, errors,
and receipt verification are checked through `sdk/platform/conformance`.
Regeneration checks reject hand edits to generated output.

The Rust SDK is authored in `services/agent/crates/layerx-sdk/src/lib.rs`.
`services/agent/tools/sdk-gen` generates TypeScript and Python bindings and
shared Rust types from `services/agent/schema/agent-api/v1.kvx`.
`sdk/platform/generators` supplies the other language outputs recorded in
the pipeline.

## Trees in this repository

| Language | Path |
| --- | --- |
| Rust | `services/agent/crates/layerx-sdk` |
| Python | `services/agent/sdk/python` |
| TypeScript | `services/agent/sdk/typescript` |
| Go | `sdk/platform/go` |
| JVM | `sdk/platform/jvm` |
| .NET | `sdk/platform/dotnet` |
| Swift | `sdk/platform/swift` |

The root README lists that table. Client SDKs under `sdk/platform/` also
appear in [Programs](../programs/index.md) for CALL receipt terminals.

## What an SDK may do

The agent-interface non-authority rule applies: an SDK constructs canonical
activities, obtains a disclosure-bound signature, submits those exact bytes,
and returns only values decoded from core-produced bytes with a verification
status. It does not apply transfers, derive state roots, or treat admission
as execution.

For signed receipts returned by `pax_sequencerReceipt`, see
[SDK sequencer receipt verification](sdk-verification.md).

## CLI and RPC

The developer CLI in `tools/platform/cli` is [CLI](../platform/cli.md).
See [Public node JSON-RPC](../reference/public-rpc.md) for node access and
[Gateway JSON-RPC](../platform/gateway-rpc.md) for hosted operations.
`layerx --rpc` and `--gateway-credential` apply to `wallet`, `token`, and
`program`.

## Publication (beta)

The release workflow in `.github/workflows/platform.yml` publishes each
declared ecosystem under a beta pre-release version with digest, signature,
SBOM, and provenance attestation.
The release artifact manifest lists package versions, registry locations
and immutable digests. Use it to select and authenticate an installable artifact.

## Status

The seven language trees are present. Cross-language parity is checked in
`sdk/platform/conformance`. `tools/qualification/beta_driver.py` measures
the ten-line integration benchmark and the five-minute first-payment
journey. These benchmarks measure integration behavior against their
configured services.
