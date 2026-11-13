# Interface catalogue contract

## Catalogue schema

The [interface catalogue](../interfaces.kvx) uses generated `[interface.<id>]` blocks referenced by task and environment interface IDs. The [fleet contracts](../../../tools/hyperfide/contracts/fleet/index.mjs) validate installation inputs and interface selections.

| Field | Observed representation | Intended use |
|---|---|---|
| Section | `[interface.<id>]` | Stable identity referenced by a task or environment |
| `kind` | String | Declaration category; samples include `c_symbol`, `go_exported`, `env_var` |
| `path` | String | Repository-relative declaration location |
| `line` | Integer | Source location hint at the pinned revision |
| `signature` | String | Declaration or configuration contract text |
| `callers` | Array of strings | Known consumer locations |
| `status` | String | Samples include `stable` and `retire` |
| `contract` | String | Supporting document and anchor |
| `notes` | String | Extraction or migration context |

Interface identity is separate from its declaration language. Caller lists describe known consumers and are not a completeness guarantee.

## Execution contract

The fleet consumes the same flat KVX grammar and stable block identities. The local [example](examples/fleet/interfaces.kvx) illustrates the interface block shape. New policy/provenance fields belong to a versioned catalogue envelope or explicit schema extension, rather than silently changing the meaning of existing fields.

The authoritative publication includes a catalogue version, immutable source identity, fragment digests, extractor version and a shard/index manifest. Resolve selected interface IDs through that index, verify each returned block and inclusion in the pinned publication, and transfer only the requested bounded closure. Do not require the whole interface file to exist inside a sandbox, in an agent prompt, or in a single service's memory. Missing dependencies return explicit partial coverage or refusal; clients cannot infer completeness from a successful byte-range read.

`package.kvx` selects the interface IDs needed by the environment. Task assignments separately declare interfaces they provide, consume or are authorized to change. A ready installation records the exact selected block digests and publication provenance. A worker may fetch further authorized blocks explicitly without changing the pinned publication implicitly.

Line numbers support navigation; identity and compatibility depend on the source revision, declaration/contract digests and checker results. Verification combines source extraction, declared producer/consumer constraints, affected-consumer checks and behavioral acceptance evidence. Matching a signature string cannot establish behavioral correctness. Historical `stable` or `retire` labels never grant execution or deletion authority.

An interface change becomes a versioned proposal identifying affected IDs and consumers. The Primary coordinates cross-wave changes; the responsible managers update assignments and environment generations after admission. Required dependency and acceptance constraints remain enforced. Generated catalogue entries are regenerated from their fragments/source; worker edits to a downloaded subset cannot redefine the authoritative contract.

## Qualification

SCM-026 must prove syntax/type rejection, stable-ID lookup, exact publication membership, unknown-version refusal, missing and stale consumers, tampered blocks, bounded selection and declared compatibility checks. SCM-027 binds those selected contracts to the installed environment and real consumer execution. A schema parse alone proves neither complete catalogue coverage nor fleet readiness.

Persistent interface stores keep a private canonical root and verify its identity on reads. Where available, Linux directory descriptors anchor access; other Node hosts use canonical-path and inode checks without requiring `/proc`. Bounded record descriptors reject symlink entries, verify persisted data and report malformed records as `INTERFACE_INVALID`. Shared read-only regular records remain supported.
