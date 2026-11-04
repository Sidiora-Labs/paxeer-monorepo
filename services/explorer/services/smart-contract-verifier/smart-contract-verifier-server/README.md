# <h1 align="center"> Smart-contract Verifier (Server) </h1>

This service exposes HTTP and gRPC APIs for comparing supplied contract bytecode
with compiler output, or forwarding verification to Sourcify. It keeps compiler
caches but does not persist verification results in a database. The explorer
supplies the bytecode and sources for local verification.

For Paxeer, the network chain id is **125** (`0x7d`). Chain JSON-RPC is served at
the root of public node hosts such as `https://api1.mainnet-beta.paxeer.network/`.
The verifier's routes below belong to its own HTTP listener. A successful source
verification establishes a bytecode match; network receipt finality comes from
committed blocks on the single Paxeer BFT network, whose kernel proposes blocks.

## Configuration

[Settings](src/settings.rs) defines service-specific fields and defaults.
The launcher loads the file named by `SMART_CONTRACT_VERIFIER__CONFIG`, if set,
then applies environment overrides. Missing fields use Serde defaults. A sample
file is not loaded automatically. Unknown service settings are rejected.

Common server, metrics, tracing and Jaeger settings come from the registry
[blockscout-service-launcher 0.17.0](https://docs.rs/blockscout-service-launcher/0.17.0/blockscout_service_launcher/),
as declared in the parent workspace and resolved in its lockfile.

### Configuration file

[config/base.toml](config/base.toml) is a partial example. In particular, it omits
`zksync_solidity` and `tracing`, which still receive defaults. The following
example includes those sections and the Linux compiler-list defaults:

```toml
[server.http]
enabled = true
addr = "0.0.0.0:8050"
max_body_size = 2097152

[server.grpc]
enabled = false
addr = "0.0.0.0:8051"

[solidity]
enabled = true
compilers_dir = "/tmp/solidity-compilers"
refresh_versions_schedule = "0 0 * * * * *"

[solidity.fetcher.list]
list_url = "https://binaries.soliditylang.org/linux-amd64/list.json"

[vyper]
enabled = true
compilers_dir = "/tmp/vyper-compilers"
refresh_versions_schedule = "0 0 * * * * *"

[vyper.fetcher.list]
list_url = "https://raw.githubusercontent.com/blockscout/solc-bin/main/vyper.list.json"

[sourcify]
enabled = true
api_url = "https://sourcify.dev/server/"
verification_attempts = 3
request_timeout = 15

[zksync_solidity]
enabled = true
evm_compilers_dir = "/tmp/zksync-solc-compilers"
evm_refresh_versions_schedule = "0 0 * * * * *"
zk_compilers_dir = "/tmp/zksync-zksolc-compilers"
zk_refresh_versions_schedule = "0 0 * * * * *"

[zksync_solidity.evm_fetcher.list]
list_url = "https://binaries.soliditylang.org/linux-amd64/list.json"

[zksync_solidity.era_evm_fetcher.list]
list_url = "https://raw.githubusercontent.com/blockscout/solc-bin/main/era-solidity.linux-amd64.list.json"

[zksync_solidity.zk_fetcher.list]
list_url = "https://raw.githubusercontent.com/blockscout/solc-bin/main/zksolc.linux-amd64.list.json"

[metrics]
enabled = false
addr = "0.0.0.0:6060"
route = "/metrics"

[jaeger]
enabled = false
agent_endpoint = "127.0.0.1:6831"

[tracing]
enabled = true
format = "default"

[compilers]
# Explicit example limit; omit to use available parallelism.
max_threads = 8
```

The cache defaults are subdirectories of `std::env::temp_dir()`; `/tmp` above is
the usual Linux location. [Compiler-list constants](../smart-contract-verifier/src/consts.rs)
also define macOS defaults. List URLs must supply compilers compatible with the
host OS and architecture. The seven-field cron expression refreshes version
lists hourly. `compilers.max_threads` must be positive and limits concurrent
compilations across the enabled verifier services; when omitted it uses available
parallelism, falling back to 8 if that cannot be determined.

Solidity, Vyper, Sourcify and ZkSync Solidity are all enabled by default.
[Startup](src/run.rs) initializes enabled services before opening the listeners.
Solidity and Vyper each initialize a compiler fetcher. ZkSync initializes three:
ordinary solc, Era solc and zksolc. The first two share `evm_compilers_dir` and
`evm_refresh_versions_schedule`; zksolc uses the `zk_*` directory and schedule.
Initial version-list or S3 fetch failures propagate as startup failures. Compiler
binaries are downloaded as needed for verification and cached locally. Sourcify
requests require access to its configured API; its timeout is in seconds and
`verification_attempts` must be at least 1.

Set an unused verifier's `enabled` field to `false` to avoid its initialization
and omit its HTTP/gRPC handlers. For example, a deployment that verifies only
ordinary EVM contracts can explicitly disable `zksync_solidity`; omitting the
section leaves it enabled.

#### List and S3 fetchers

Each of these settings accepts exactly one fetcher variant, `list` or `s3`:

| Setting | Role |
| --- | --- |
| `solidity.fetcher` | Solidity compiler inventory |
| `vyper.fetcher` | Vyper compiler inventory |
| `zksync_solidity.evm_fetcher` | Ordinary solc inventory for ZkSync |
| `zksync_solidity.era_evm_fetcher` | Era solc inventory |
| `zksync_solidity.zk_fetcher` | zksolc inventory |

The `list` variant has `list_url`. To use S3, replace that fetcher's `.list`
section with `.s3`. Its fields are `bucket`, `region`, `endpoint`, `access_key`
and `secret_key`. `bucket` is required; configure a valid AWS region or a custom
endpoint. Credentials can be supplied through the SDK credential resolution or
explicit settings. The conversion and initialization are in
[services/common.rs](src/services/common.rs). Do not keep both variants in the
same fetcher configuration.

### Environment variables

Use the `SMART_CONTRACT_VERIFIER` prefix and `__` for each hierarchy separator.
For example, `solidity.compilers_dir` becomes
`SMART_CONTRACT_VERIFIER__SOLIDITY__COMPILERS_DIR`.
[config/base.env](config/base.env) is a partial collection of examples, not the
complete settings schema. The ZkSync and tracing names omitted there include:

```sh
SMART_CONTRACT_VERIFIER__ZKSYNC_SOLIDITY__ENABLED=true
SMART_CONTRACT_VERIFIER__ZKSYNC_SOLIDITY__EVM_COMPILERS_DIR=/tmp/zksync-solc-compilers
SMART_CONTRACT_VERIFIER__ZKSYNC_SOLIDITY__EVM_REFRESH_VERSIONS_SCHEDULE='0 0 * * * * *'
SMART_CONTRACT_VERIFIER__ZKSYNC_SOLIDITY__EVM_FETCHER__LIST__LIST_URL=https://binaries.soliditylang.org/linux-amd64/list.json
SMART_CONTRACT_VERIFIER__ZKSYNC_SOLIDITY__ERA_EVM_FETCHER__LIST__LIST_URL=https://raw.githubusercontent.com/blockscout/solc-bin/main/era-solidity.linux-amd64.list.json
SMART_CONTRACT_VERIFIER__ZKSYNC_SOLIDITY__ZK_COMPILERS_DIR=/tmp/zksync-zksolc-compilers
SMART_CONTRACT_VERIFIER__ZKSYNC_SOLIDITY__ZK_REFRESH_VERSIONS_SCHEDULE='0 0 * * * * *'
SMART_CONTRACT_VERIFIER__ZKSYNC_SOLIDITY__ZK_FETCHER__LIST__LIST_URL=https://raw.githubusercontent.com/blockscout/solc-bin/main/zksolc.linux-amd64.list.json
SMART_CONTRACT_VERIFIER__TRACING__ENABLED=true
SMART_CONTRACT_VERIFIER__TRACING__FORMAT=default
```

For S3 use the same fetcher prefix followed by `__S3__BUCKET`, `__S3__REGION`,
`__S3__ENDPOINT`, `__S3__ACCESS_KEY` or `__S3__SECRET_KEY`, replacing its list
configuration. Tracing formats are `default` and `json`. Disabling tracing also
disables Jaeger tracing initialization. The commented `extensions` entries in
the shipped examples are not fields of the current `Settings` type.

## Grpc Api

The [v2 protobuf directory](../smart-contract-verifier-proto/proto/v2) defines
SolidityVerifier, VyperVerifier, SourcifyVerifier, ZkSyncSolidityVerifier and
Health services. HTTP and gRPC register handlers according to the same verifier
enablement settings.

## Http Api

The [HTTP route mapping](../smart-contract-verifier-proto/proto/v2/api_config_http.yaml)
and [Swagger schema](../smart-contract-verifier-proto/swagger/v2/smart-contract-verifier.swagger.yaml)
describe the wire contract. HTTP JSON uses camelCase field names. Examples below
show request shapes; replace the illustrative bytecode with the actual creation
input or deployed bytecode and supply the matching sources and compiler version.
`input` fields contain serialized JSON strings.

### Solidity Multi-Part files

`POST /api/v2/verifier/solidity/sources:verify-multi-part`

```json
{
  "bytecode": "0x00",
  "bytecodeType": "CREATION_INPUT",
  "compilerVersion": "v0.8.18+commit.87f61d96",
  "evmVersion": "default",
  "optimizationRuns": 200,
  "sourceFiles": {
    "A.sol": "pragma solidity ^0.8.18; contract A {}"
  },
  "metadata": {"chainId": "125"}
}
```

`evmVersion` and `optimizationRuns` are optional. Omitting the latter disables
optimizer enablement in the constructed input; supplying it enables optimization
with that run count. The former `libraries` request field is reserved and is
not supported. Verification can infer manual library links from the compared
bytecode. To provide compiler linking settings explicitly, use Standard JSON.
Optional `metadata.chainId` and `metadata.contractAddress` identify the explorer
request; they do not cause the local verifier to fetch on-chain bytecode.

### Solidity Standard-JSON input

`POST /api/v2/verifier/solidity/sources:verify-standard-json`

```json
{
  "bytecode": "0x00",
  "bytecodeType": "DEPLOYED_BYTECODE",
  "compilerVersion": "v0.8.18+commit.87f61d96",
  "input": "{\"language\":\"Solidity\",\"sources\":{\"A.sol\":{\"content\":\"pragma solidity ^0.8.18; contract A {}\"}},\"settings\":{\"optimizer\":{\"enabled\":true,\"runs\":200},\"libraries\":{}}}"
}
```

Use the compiler's [Standard JSON input](https://docs.soliditylang.org/en/latest/using-the-compiler.html#input-description)
for `settings.libraries`, optimizer settings, EVM version and other supported
compiler options. See [Compiler Settings (transition)](#compiler-settings-transition)
for the difference between input settings and detected library links.

### Vyper Multi-Part files

`POST /api/v2/verifier/vyper/sources:verify-multi-part`

```json
{
  "bytecode": "0x00",
  "bytecodeType": "DEPLOYED_BYTECODE",
  "compilerVersion": "v0.3.7+commit.6020b8bb",
  "evmVersion": "istanbul",
  "sourceFiles": {
    "A.vy": "# @version 0.3.7\n@external\n@view\ndef value() -> uint256:\n    return 1\n"
  },
  "interfaces": {}
}
```

`evmVersion`, `interfaces` and `metadata` are optional. Interface values are
source strings (including serialized JSON for JSON interfaces). The former
`optimizations` field is reserved. Multipart verification uses the compiler's
default optimization behavior.

### Vyper Standard-JSON input

`POST /api/v2/verifier/vyper/sources:verify-standard-json`

Use `bytecode`, `bytecodeType`, `compilerVersion`, serialized `input` and optional
`metadata`, as defined by `VerifyVyperStandardJsonRequest` in the
[v2 contract](../smart-contract-verifier-proto/proto/v2/smart-contract-verifier.proto).
The current [Vyper settings model](../smart-contract-verifier/src/verify/vyper_compiler_input.rs)
accepts `settings.optimize` as a boolean, for example `{"optimize": false}`, and
also supports `evmVersion`, `bytecodeMetadata`, `outputSelection` and
`search_paths`. Supply those settings inside `input`, using compiler-compatible
values. A complete source/interface example is in the
[Standard JSON fixture](tests/test_cases_vyper/standard_json_with_interfaces.json).

### Solidity batch verification

- `POST /api/v2/verifier/solidity/sources:batch-verify-multi-part`
- `POST /api/v2/verifier/solidity/sources:batch-verify-standard-json`

Both take `contracts`, an array of objects with optional `creationCode`,
`runtimeCode` and `metadata`, plus `compilerVersion`. Each contract needs at least
one non-empty code value. Multipart uses `sources` (not `sourceFiles`), optional
`evmVersion` and optional `optimizationRuns`. Standard JSON uses a serialized
`input` string. See the [batch request contract](../smart-contract-verifier-proto/proto/v2/smart-contract-verifier.proto)
and [complete Standard JSON request fixture](tests/test_cases_batch_solidity/basic_standard_json.json).
The response family is `BatchVerifyResponse`.

### Sourcify

- `POST /api/v2/verifier/sourcify/sources:verify`
- `POST /api/v2/verifier/sourcify/sources:verify-from-etherscan`

The first proxies a file-based request to the configured Sourcify API:

```json
{
  "address": "0xcafecafecafecafecafecafecafecafecafecafe",
  "chain": "125",
  "files": {
    "A.sol": "pragma solidity ^0.8.18; contract A {}",
    "metadata.json": "{}"
  },
  "chosenContract": 0
}
```

`files` must contain the actual sources and compiler metadata; the empty metadata
above is illustrative. `chosenContract` is optional. The Etherscan-import route
takes only `address` and `chain`. Both depend on the upstream Sourcify service's
support for the requested chain and return the solo `VerifyResponse` family.

### ZkSync Solidity Standard-JSON input

`POST /api/v2/zksync-verifier/solidity/sources:verify-standard-json`

This API takes `code` (deployed code), optional `constructorArguments`,
`zkCompiler`, `solcCompiler` and serialized `input`. It uses the distinct
[ZkSync protobuf contract](../smart-contract-verifier-proto/proto/v2/zksync-solidity.proto).
The [Greeter fixture](tests/test_cases_zksync_solidity/simple_standard_json.json)
contains full deployed code, constructor arguments and input for zksolc `v1.4.0`
and solc `v0.8.17+commit.8df45f5f`. Its response family is the ZkSync
`VerifyResponse`, described separately below.

## Outputs

Select the response decoder by endpoint. Solo verification, Solidity batch
verification and ZkSync verification have distinct envelopes. The success
examples below are excerpts using values from the linked compiler fixtures;
large source, bytecode and artifact fields are omitted.

### Success

Solo Solidity, Vyper and Sourcify routes return `VerifyResponse`. A successful
response has `message: "OK"`, `status: "SUCCESS"` and a `source`. For example,
selected fields for the [manually linked library fixture](tests/test_cases_solidity/libraries_manually_linked.json)
are:

```json5
{
  "message": "OK",
  "status": "SUCCESS",
  "source": {
    "fileName": "contracts/1_Storage.sol",
    "contractName": "Storage",
    "compilerVersion": "v0.8.18+commit.87f61d96",
    "sourceType": "SOLIDITY",
    "matchType": "FULL",
    "libraries": {
      "contracts/1_Storage.sol:Journal": "0x7d53f102f4d4aa014db4e10d6deec2009b3cda6b"
    }
    // sourceFiles, compilerSettings, abi and artifact fields omitted
  }
  // extraData and postActionResponses omitted
}
```

`source` also carries `compilerSettings`, `sourceFiles`, optional `abi`, optional
`constructorArguments`, optional `compilationArtifacts`,
`creationInputArtifacts`, `deployedBytecodeArtifacts`, and `isBlueprint`.
`sourceType` is `SOLIDITY`, `VYPER` or `YUL`; `matchType` is `PARTIAL` or `FULL`.
Unspecified enum values are not successful match classifications.

`compilerSettings`, `abi` and the artifact fields are JSON encoded **as strings**;
parse them separately after decoding the response. The artifact fields contain
compiler output such as ABI/docs, source maps, link references and immutable
references. Sourcify responses omit those optional artifacts.

`extraData.localCreationInputParts` and `localDeployedBytecodeParts` contain
locally compiled bytecode parts with `type` (`main` or `meta`) and hex `data`.
Sourcify returns empty part arrays. For local verification the arrays may also
be empty when there is no CBOR auxiliary-data partition to report.

### Verification Failure

A compilation failure, invalid Standard JSON content, or a bytecode mismatch
can be an HTTP 200 / gRPC OK response with solo `status: "FAILURE"`:

```json
{
  "message": "No contract could be verified with provided data",
  "status": "FAILURE"
}
```

Such a response has no verified `source`. Check the application status even when
the transport succeeded.

### Batch responses

`BatchVerifyResponse` has a oneof: either `compilationFailure` or
`contractVerificationResults`. Results are in request order. Each `items` entry
has either `success` or `failure`; a mismatch is `{"failure": {}}`. An excerpt
of a successful item from the [batch fixture](tests/test_cases_batch_solidity/basic_standard_json.json)
is:

```json5
{
  "contractVerificationResults": {
    "items": [{
      "success": {
        "compiler": "SOLC",
        "compilerVersion": "v0.8.24+commit.e11b9ed9",
        "language": "SOLIDITY",
        "fileName": "contracts/VerifierAlliance.sol",
        "contractName": "Storage"
        // bytecode, sources, settings, artifacts and match details omitted
      }
    }]
  }
}
```

Batch success fields include `creationCode`, `runtimeCode`, `sources`,
`compilerSettings`, `compilationArtifacts`, `creationCodeArtifacts`,
`runtimeCodeArtifacts`, and optional `creationMatchDetails` and
`runtimeMatchDetails`. Settings and artifacts are serialized JSON strings.
Match details have `matchType` plus JSON-string `values` and `transformations`.
A compilation or Standard JSON content failure instead uses:

```json5
{
  "compilationFailure": {
    "message": "Compilation error: ..." // diagnostic text varies
  }
}
```

Batch responses have no solo `status` or `source` field. A successful transport
can contain a compilation failure or a mixture of item successes and failures.

### ZkSync responses

The ZkSync envelope contains exactly one of `verificationSuccess`,
`verificationFailure` or `compilationFailure`. Selected success fields for the
[Greeter fixture](tests/test_cases_zksync_solidity/simple_standard_json.json) are:

```json5
{
  "verificationSuccess": {
    "fileName": "contracts/Greeter.sol",
    "contractName": "Greeter",
    "zkCompiler": {"compiler": "zksolc", "version": "v1.4.0"},
    "evmCompiler": {"compiler": "solc", "version": "v0.8.17+commit.8df45f5f"},
    "language": "SOLIDITY"
    // sources, settings, artifacts and matches omitted
  }
}
```

Success includes `compilerSettings`, `sources`, `compilationArtifacts`,
`creationCodeArtifacts`, `runtimeCodeArtifacts`, optional `creationMatch` and
`runtimeMatch`. Settings and artifacts are JSON strings. Each match has `type`
(`PARTIAL` or `FULL`), and JSON-string `values` and `transformations`.
A mismatch returns:

```json
{
  "verificationFailure": {
    "message": "No contract could be verified with provided data"
  }
}
```

Compilation or Standard JSON content failures use `compilationFailure.message`
as in the batch example. These outcomes are HTTP 200 / gRPC OK and have no solo
`status` field.

### Bad Request

Transport errors are distinct from the verification outcomes above. Malformed
hex, an invalid bytecode type, an invalid compiler version or a compiler absent
from the configured inventory can produce gRPC `INVALID_ARGUMENT` (code 3), mapped
to HTTP 400. Internal compiler/download failures produce gRPC `INTERNAL` (code 13),
mapped to HTTP 500. The HTTP adapter's error envelope has numeric `code` and
`message`. JSON extraction errors can also reject a request before the verifier
handler runs. Parseable outer JSON with invalid compiler-input content follows
the endpoint's application failure family instead.

## Version List

| Route | Response |
| --- | --- |
| `GET /api/v2/verifier/solidity/versions` | `{"compilerVersions": [...]}` |
| `GET /api/v2/verifier/vyper/versions` | `{"compilerVersions": [...]}` |
| `GET /api/v2/zksync-verifier/solidity/versions` | `{"zkCompilers": [...], "solcCompilers": [...]}` |

These requests have no input. Versions come from the configured fetchers and are
sorted in descending order. Use the listed version strings for verification.
`POST /api/v2/verifier/solidity/methods:lookup` is also declared in the
[v2 contract](../smart-contract-verifier-proto/proto/v2/smart-contract-verifier.proto);
`GET /health` is registered independently of verifier enablement.

## Compiler Settings (transition)

Clients of the solo API read `source.compilerSettings`, a serialized Standard
JSON settings object, for optimizer and EVM settings formerly exposed as
`optimization`, `optimization_runs` and `evm_version` fields. Parse the string
first. For Solidity, `optimizer.enabled` indicates enablement and
`optimizer.runs` gives the run count when present. Missing keys mean the compiler's
defaults for that version apply. `evmVersion` similarly records an explicit EVM
target when present.

Library addresses require both input settings and verification results.
`settings.libraries` is the compiler-input map, keyed by source file and library
name. `source.libraries` is the response map keyed by fully qualified
`file:library` name; local verification combines compiler-provided addresses with
manually linked addresses detected in the match values.

The two existing fixtures demonstrate the difference:

| Fixture | Input `settings.libraries` | Returned `source.libraries` |
| --- | --- | --- |
| [Compiler-linked](tests/test_cases_solidity/libraries_linked_by_compiler.json) | `contracts/1_Storage.sol` maps `Journal` to `0x7d53f102f4d4aa014db4e10d6deec2009b3cda6b` | `contracts/1_Storage.sol:Journal` maps to that address |
| [Manually linked](tests/test_cases_solidity/libraries_manually_linked.json) | `{}` | `contracts/1_Storage.sol:Journal` maps to the same detected address |

An empty compiler-settings library map does not mean the deployed contract has
no linked libraries. Preserve `source.libraries` when migrating clients from the
former `contract_libraries` field. The local response conversion is in
[types/verification_result.rs](src/types/verification_result.rs).

## Related services

[Sig-provider](../../sig-provider/README.md) supplies candidate transaction and
event decodings. [Env-collector](../../libs/env-collector/README.md) explains how
to maintain environment tables from a serializable settings type and an example
configuration; its output covers the example's shape, not every fetcher variant.
