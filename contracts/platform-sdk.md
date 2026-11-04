# Platform SDK contract

This contract describes the platform clients in [`sdk/platform`](../sdk/platform)
and supplies the stable section anchors used by the interface registry. Public
names retain `LayerX` where that is the package or wire-format name. The clients
connect to Paxeer X Network: one BFT chain, with its kernel participating in the
network and proposing blocks. A transaction receipt is final when its block is
committed. Sub-chains are tenant domains in the database, without separate chain
infrastructure.

## Network and transport

The EVM chain ID is **125** (`0x7d`). Public JSON-RPC uses the root of
`https://api1.mainnet-beta.paxeer.network` through
`https://api14.mainnet-beta.paxeer.network`, or
`https://api-mainnet-beta.paxeer.network`. Configure the chosen node origin
explicitly and check `eth_chainId` before signing. The node namespaces are
`debug`, `echo`, `eth`, `net`, `pax`, `test` and `txpool`.

Hosted `px_` methods have their own declared
[gateway contract](../services/platform/hosted/gateway/openrpc.json). Agent and
human service operations also require their configured service origins and
credentials; their generated catalogues do not promise that a public node
implements those operations. The retired router and separate daemon are not
required to access the merged network.
`node.hyperpaxeer.com` is the hpx library mirror, not a JSON-RPC endpoint.

## SDK JVM

The [Maven project](../sdk/platform/jvm/pom.xml) targets Java 21, publishes the
coordinate `com.sidiora.layerx:layerx-sdk`, and exposes Java package
`com.sidiora.layerx.sdk`. Kotlin helpers live in
[`LayerX.kt`](../sdk/platform/jvm/src/main/kotlin/com/sidiora/layerx/sdk/LayerX.kt).

[`ProductionClient`](../sdk/platform/jvm/src/main/java/com/sidiora/layerx/sdk/ProductionClient.java)
accepts a `ProductionTransport`. Its typed `agent` and `human` overloads accept
generated operation descriptors and request records and return
`CompletionStage` results. Plane mismatches and unknown operation names are
rejected. `Options` carries an idempotency key and path parameters; an operation
that requires a key is refused locally with `idempotency-required` if it is
missing. The client delegates each call once; retry metadata is a caller decision,
not an automatic retry loop.

[`ProtocolAmount`](../sdk/platform/jvm/src/main/java/com/sidiora/layerx/sdk/ProtocolAmount.java)
represents unsigned 128-bit base units, from zero through `2^128 - 1`. JSON uses
decimal strings matching `0|[1-9][0-9]*`; negative values, leading zeroes,
floating-point values and JSON number tokens are rejected. Do not convert money
through a floating-point intermediary.

[`PlatformSdkException`](../sdk/platform/jvm/src/main/java/com/sidiora/layerx/sdk/PlatformSdkException.java)
preserves the wire code, retry classification, request ID, protocol result code,
retry delay and plane-specific error classification where present. Retry values
are `never`, `safe`, `after` and `unknown-outcome`; `retryAfterMs` is milliseconds.
A timeout or lost response to a mutation can leave its outcome unknown. Resolve
that submission before issuing a replacement. A verified execution failure uses
`revert`, while a failed proof uses `verification-failure`.

## Receipt verification

The maintained JVM entry points are
[`LocalVerifier`](../sdk/platform/jvm/src/main/java/com/sidiora/layerx/sdk/verify/LocalVerifier.java),
[`Committed`](../sdk/platform/jvm/src/main/java/com/sidiora/layerx/sdk/verify/Committed.java)
and
[`PortableReceipt`](../sdk/platform/jvm/src/main/java/com/sidiora/layerx/sdk/verify/PortableReceipt.java).
The verification levels are **`receipted`** and **`committed`**. A JSON field
claiming a level does not itself establish that level.

`LocalVerifier.verifySequencerReceipt(receiptBytes, rawTransaction, keys)` takes
the canonical signed bytes, the exact signed EVM transaction and independently
authorized `ReceiptKeys`. The method name is retained as an SDK API name. Each
`ReceiptKey` contains a nonzero 32-byte `keyId`, a nonzero 32-byte Ed25519
`publicKey`, and inclusive unsigned 64-bit `firstHeight` and `lastHeight` bounds.
The key set is nonempty and rejects duplicate IDs. Applications must obtain and
retain these pins independently of the receipt or portable bundle.

The verifier accepts receipt versions 2 and 3 and checks canonical encoding,
signature, transaction hash, key ID and authorized height, receipt fields and
effect layout. The receipt's `activityId` equals Keccak-256 of the raw transaction;
`contextHash` identifies its receipt key. `batchId` encodes the height in its last
eight bytes with the first 24 bytes zero. The signature authenticates SHA-256 of
`LXP/v1/receipt` followed by a NUL byte and the unsigned canonical receipt. The
sequence hash binds the predecessor hash, sequence and activity ID under the
`PAXEER/v1/sequence` domain followed by a NUL byte.

The overload taking `JsonNode view` and `previousSequenceHash` also binds the JSON
view and an expected predecessor. Supply an independently retained predecessor
when checking continuity. The view includes `receipt`, `txHash`, `rung`, `seq`,
`height`, `timestamp`, `nonce`, `status`, `keyId`, `from`, `to`, `feeAmount`,
`feeDenom`, `paymentDenom`, gas fields, `index`, `contractAddress`, `sponsor`,
`vmError`, `returnData` and `logs`. Hashes and byte strings have a `0x` prefix;
quantities use hexadecimal strings without redundant leading zeroes. Return data
and log contents must agree with the signed effects. A view claiming `committed`
also carries `blockNumber` and `blockHash`; the local receipt verifier still
returns `receipted` until commit evidence is checked.

A successful execution has receipt `resultCode = 0` and verification `status = 1`.
A receipted revert has `resultCode = 1` and `status = 0`. Both can have valid
signatures and commit proofs. Call `requireSuccess()` before treating a verified
receipt as successful; the view-aware verifier can extract an authenticated
`Error(string)` revert reason from return data.

For committed verification, use the `Committed.verify` overload that also takes
the raw transaction, execution header, `ReceiptInclusion`, execution commit and
validator set. It binds the EVM transaction inside the committed block
transaction, transaction and batch proofs against the execution header's
`dataHash`, and the execution result against the successor header's
`lastResultsHash`. It checks the successor link, validator hashes, signatures,
voting power, result status and receipt data. The successor header supplies the
result proof; this is evidence from the same chain.

Consensus authorization is an additional caller obligation. Use
[`ConsensusTrust`](../sdk/platform/jvm/src/main/java/com/sidiora/layerx/sdk/verify/ConsensusTrust.java)
with an independently configured consensus chain ID, height and validator hash.
Its authenticated transitions connect the trusted height to the target header.
The consensus chain ID is a string distinct from the EVM numeric chain ID.
`Committed.verify` alone checks the supplied proof and validator set; authenticate
that header against the trust root before accepting it as the intended network.

Portable bundles use format `layerx-receipt-proof-v2`. Common fields are `format`,
`verificationLevel`, `rawTransaction`, `signedReceipt`, `receiptDigest`,
`receiptKey` and `receiptJson`. Committed bundles additionally carry `blockHeader`,
`receiptInclusion`, `commit` and `validatorSet`. `PortableReceipt.verify` checks
the carried receipt key against the caller's pins and requires `ConsensusTrust`
for a committed bundle. Preserve the bundle's encodings; use the provided codecs
for nested consensus evidence rather than converting every field to hex. A
verified bundle can be re-exported with `PortableReceipt.export(verified)`.

Receipt refusal details are generated from
[`receipt.kvx`](../sdk/platform/generators/receipt.kvx): `decode`,
`canonical-encoding`, `receipt-version`, `activity-id`, `key-id`, `key-height`,
`raw-transaction`, `fields`, `return-data`, `logs`, `sequence-hash` and `signature`.
Do not downgrade a failed check to an unverified success.

## Retry and restart ownership

Before broadcasting, applications must durably retain the signed transaction,
its hash, sender and nonce, the request's idempotency key where applicable, tenant
identity and expected effect. After restart, resolve that hash and verify the
receipt with the retained trust configuration. A transport failure does not prove
that a transaction was rejected. Reusing a nonce with different signed bytes can
replace the original transaction; do not do so as an implicit retry.

The node-backed JVM `ProgramsClient` keeps submitted transaction and nonce maps
in memory. Those maps are not a durable submission journal. Its `ProgramSubmission`
retains the transaction hash and signed bytes and is pending until a receipt is
attached; applications own persistence and recovery.

[`ResumableStream`](../sdk/platform/jvm/src/main/java/com/sidiora/layerx/sdk/ResumableStream.java)
checks requested cursors, event cursor continuity and duplicate event IDs before
acceptance. Its publisher supports one subscriber and demand-driven delivery.
The cursor and duplicate set live in memory. Persist an application's last
processed cursor together with its durable side effects, and deduplicate on
replay; publisher delivery is not a durable acknowledgement.

## Agent envelope

[`HttpProductionTransport`](../sdk/platform/jvm/src/main/java/com/sidiora/layerx/sdk/HttpProductionTransport.java)
serializes agent requests with `version = 2`, `request_id`, `operation`, `request`,
`credential` and `idempotency_key`. The credential contains `tenant`, `session_id`,
`token_id` and `generation`. Mutating operations require a canonical lowercase
32-byte hexadecimal idempotency key; reads carry null. The encoded envelope limit
is 1,048,576 bytes. The response must echo the request ID and match the expected
version and shape. Preserve tenant and session coordinates across the whole call.

HTTP authentication and session authorization are separate inputs. Keep token
material in `SecretBytes`, close credential objects when finished and exclude
credentials and signed request bodies from telemetry. Service verification
metadata must not replace the receipt and consensus checks above.

## Program operations

The node-backed
[`ProgramsClient`](../sdk/platform/jvm/src/main/java/com/sidiora/layerx/sdk/ProgramsClient.java)
uses EVM JSON-RPC. Programs is the precompile at
`0x000000000000000000000000000000000000101d`; authorization is at
`0x000000000000000000000000000000000000101c`. The implementation and ABI live in
[`precompiles/layerxprograms`](../precompiles/layerxprograms) and
[`precompiles/layerxauth`](../precompiles/layerxauth).

Preparation fetches the pending nonce and estimates gas. `ProgramCall` describes
an EIP-1559 type-2 transaction for chain 125, with zero native value and an empty
access list. `submit(prepared, signedTransaction)` binds the signed transaction
to the preparation before `eth_sendRawTransaction`. Receipt lookup uses
`pax_sequencerReceipt` and verifies the returned bytes against the retained raw
transaction and receipt keys. A submitted or missing outcome is not successful
execution. `simulate` uses `eth_call` and returns a result whose `committed()` is
false. Deployment, upgrade and wind-down use their precompile ABI calls.

## Native codecs

[`NativeProgramCall`](../sdk/platform/jvm/src/main/java/com/sidiora/layerx/sdk/NativeProgramCall.java)
encodes the program ID, guest ABI, entrypoint, calldata, capability bytes, access
declaration, response capacity and seven resource bounds. Its native frame is
distinct from EVM ABI calldata: `calldata(feeLimit)` wraps a call for Programs,
while `simulateCalldata(feeLimit)` selects simulation. Signed transaction binding
checks the decoded call against the original request. Use the matching lifecycle
types for deployment, upgrade and wind-down and the generated ABI support table;
do not infer an encoding version from a filename or a receipt's claimed level.

## SDK Go

The [Go module](../sdk/platform/go/go.mod) is
`github.com/Sidiora-Labs/LayerX-Network/sdk/platform/go`.
[`verify.go`](../sdk/platform/go/verify.go) exposes `VerifySequencerReceipt`;
[`committed.go`](../sdk/platform/go/committed.go) exposes
`VerifyCommittedReceipt`; and
[`portable_receipt.go`](../sdk/platform/go/portable_receipt.go) exposes
`ExportPortableReceipt` and `VerifyPortableReceipt`. Supply independently
authorized receipt keys and committed trust. Use the Go types and codecs for
consensus evidence; their JSON byte encodings differ from EVM RPC quantities.

## SDK Swift

The [Swift package](../sdk/platform/swift/Package.swift) exports `LayerXSDK` for
iOS 15 and macOS 12 or later. Its generated operation catalogue, mirror schema
and receipt contract are under `Sources/LayerXSDK/Generated`; transport,
production calls, receipt verification, committed verification and portable
receipts are implemented alongside those generated files.

## SDK dotnet

The [.NET project](../sdk/platform/dotnet/LayerX.Sdk.csproj) targets `net8.0` and
uses package ID `LayerX.Sdk`. `Generated` contains the operation, mirror and
receipt contract definitions. `Transport.cs`, `Production.cs`, `Verifier.cs`,
`Committed.cs` and `PortableReceipt.cs` implement the corresponding client paths.
Use each language's exposed types rather than assuming Java signatures are shared.

## Operation catalog

[`operations.json`](../sdk/platform/conformance/operations.json) records operation
groups and their plane, request and response names, method, path, idempotency and
transport metadata. JVM `GeneratedSchema` and `OperationCatalog` expose the typed
descriptors. An operation name identifies a contract, not a universally available
route. Respect its transport: EVM calls use the node, while service operations
use their configured service. Do not substitute an HTTP path for EVM calldata.

## Mirror schema

[`mirror-v2.kvx`](../sdk/platform/schema/mirror-v2.kvx) defines archive framing,
source policies, finality requirements, limits and typed errors. Its archive
format uses unsigned big-endian integers, SHA-256 commitments and rejects trailing
bytes. Policies are `exact`, `ordered-preference` and `agreement`; evidence from
different sources cannot be composed into a single proof.

## Mirror verification

JVM
[`MirrorSourceVerifier`](../sdk/platform/jvm/src/main/java/com/sidiora/layerx/sdk/verify/MirrorSourceVerifier.java)
invokes a configured verifier executable with a configured source policy and
bounded timeout. Its result carries source identity, target, canonical position,
provenance, lag and agreement information as well as proof information. Keep
cryptographic validity, source finality and freshness distinct. An unknown lag
does not establish that a mirror is current, and mirror evidence does not replace
the merged chain's committed receipt verification.

## PAXAI schema

[`paxai-v1.kvx`](../sdk/platform/schema/paxai-v1.kvx) defines the AI market client
codecs and limits. Its producer is
[`layerx-programs-ai-market`](../programs/crates/layerx-programs-ai-market), with
Rust, TypeScript and Python consumers named in the schema. Amounts and heights
use decimal strings, identifiers are nonzero 32-byte values, absence is explicit
null, and trailing bytes and unknown enum codes are refused. Market projection
state must be assessed with its receipt evidence and freshness bindings.

## Conformance vectors

Shared fixtures live in
[`sdk/platform/conformance/fixtures`](../sdk/platform/conformance/fixtures).
V1 covers successful receipted transactions, V2 a receipted revert, V3 receipt
refusals, V4 committed evidence including insufficient voting power, and V5
portable receipt bundles. Receipt keys and consensus roots used to authorize
verification are independent inputs, even when a bundle carries copies.
[`run-jvm.sh`](../sdk/platform/conformance/run-jvm.sh) checks generation and invokes
the JVM conformance profile. Fixtures and catalogue checks do not establish live
deployment availability.

## Generators

[`layerx-platform-sdkgen`](../sdk/platform/generators/src/main.rs) coordinates
platform generation.
[`generate_jvm.py`](../sdk/platform/generators/generate_jvm.py) reads the agent
and human schemas to emit Java models and operation descriptors.
[`generate_portable.py`](../sdk/platform/generators/generate_portable.py) reads
the operation manifest for Swift and C# surfaces. Receipt checks come from
[`receipt.kvx`](../sdk/platform/generators/receipt.kvx), with mirror and PAXAI
definitions in `sdk/platform/schema`. Change the source contract and regenerate
consumers together instead of hand-editing generated declarations.

## Pipeline

[`pipeline.kvx`](../sdk/platform/pipeline.kvx) records source and generator
digests, output roots, generated file digests and handwritten files. It covers
agent, human, platform and conformance outputs. Generator drift is checked
against that manifest; a matching digest records generation consistency, not
receipt validity, network availability or proof of a successful transaction.
