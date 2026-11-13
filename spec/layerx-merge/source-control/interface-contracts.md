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

The versioned publication contains `schema_version`, `kind`, `source_revision`, `catalogue_digest`, `root_digest`, `entry_count` and `provenance`. Provenance binds `kind`, `source_revision`, `producer`, `version` and `input_digest`. Indexed entries carry block digests, declared references and Merkle inclusion proofs. Selection fetches requested IDs and their declared closure through the index, checks every block and proof against the pinned publication, and enforces block and byte limits.

`package.kvx` selects the interface IDs needed by the environment. Task assignments separately declare interfaces they provide, consume or are authorized to change. A ready installation records the exact selected block digests and publication provenance. A worker may fetch further authorized blocks explicitly without changing the pinned publication implicitly.

Line numbers support navigation. Selection returns `coverage = DECLARED_CLOSURE` and `compatibility = UNVERIFIED`; the consumer also supplies expected block digests and required/provided interface IDs. Behavioral acceptance is evaluated separately from catalogue membership. The `stable` and `retire` labels describe catalogue entries.

An interface change becomes a versioned proposal identifying affected IDs and consumers. The Primary coordinates cross-wave changes; the responsible managers update assignments and environment generations after admission. Required dependency and acceptance constraints remain enforced. Generated catalogue entries are regenerated from their fragments/source; worker edits to a downloaded subset cannot redefine the authoritative contract.

## Qualification

The [fleet contract module](../../../tools/hyperfide/contracts/fleet/index.mjs) exposes `parseInterfaces`, `buildInterfaceIndex`, `selectInterfaces` and `verifyFleetInterfaceSelection`. Installation verifies the delivered fragment against its publication proof and the pinned consumer binding before using the selected contracts.

Persistent interface stores keep a private canonical root and verify its identity on reads. Where available, Linux directory descriptors anchor access; other Node hosts use canonical-path and inode checks without requiring `/proc`. Bounded record descriptors reject symlink entries, verify persisted data and report malformed records as `INTERFACE_INVALID`. Shared read-only regular records remain supported.
