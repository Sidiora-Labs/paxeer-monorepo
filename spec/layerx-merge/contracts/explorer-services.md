# Explorer services contract

Source revision: `2a1033aa2`. This contract freezes the cited interface and acceptance blocks of the specification. Statuses include the specification overrides. A source marked planned has line 0; a positive source line is verified in the pinned revision.

The context and decision tables reproduce the rules linked by interface identifiers. An anchor without a linked context gains no additional storage, gas, activation or pause rule here. Acceptance qualification remains UNRUN until its declared gate succeeds.

## sig-provider-precompiles

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.cfg-sig-provider-sources-precompiles-j54</code> | <code>config_key</code> | <code>new</code> | <code>services/explorer/services/sig-provider/sig-provider-server/src/settings.rs</code> (planned) | <code>[sources.precompiles] enabled = true</code> |
| <code>interface.14.rs-sig-provider-precompile-source-j54</code> | <code>rust_pub_item</code> | <code>new</code> | <code>services/explorer/services/sig-provider/sig-provider/src/sources/precompiles.rs</code> (planned) | <code>pub struct PrecompileSource; impl SignatureSource for PrecompileSource</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.cfg-sig-provider-sources-precompiles-j54</code> | notes | Enables the precompile signature source; default true. |
| <code>interface.14.rs-sig-provider-precompile-source-j54</code> | notes | Answers paxeer precompile selectors from an embedded table built from the precompile ABI files. |

Contract context <code>contract_context.14.sig-provider-precompiles</code>:

| Field | Frozen rule |
|---|---|
| interface | PrecompileSource behind [sources.precompiles] enabled = true (planned) |
| storage | Embedded table from the precompile ABI files. |
| errors | Unknown selectors answer an empty list. |
| events | none |
| gas | none (off-chain service) |
| activation | Enabled by default from wave 2. |
| invariants | The embedded table equals the ABI files at the same revision. |
| pause | none |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.14.276.1</code> | req | 14276.1 |
| <code>acceptance.14.276.1</code> | scenario | Given the embedded precompile signature table, when get_function_signatures is called with the selector of the first layerxexchange function, then it returns that signature |
| <code>acceptance.14.276.1</code> | test | services/explorer/services/sig-provider/sig-provider/src/sources/precompiles.rs::selector_resolves_to_exchange_signature (planned) |
| <code>acceptance.14.276.1</code> | qualification | UNRUN |
| <code>req.14276</code> | ac_1 | PrecompileSource answers the 4-byte selector of a layerxexchange function with its text signature. |
| <code>acceptance.14.276.2</code> | req | 14276.2 |
| <code>acceptance.14.276.2</code> | scenario | Given the embedded table, when get_function_signatures is called with 0xdeadbeef, then it returns an empty list |
| <code>acceptance.14.276.2</code> | test | services/explorer/services/sig-provider/sig-provider/src/sources/precompiles.rs::unknown_selector_returns_empty (planned) |
| <code>acceptance.14.276.2</code> | qualification | UNRUN |
| <code>req.14276</code> | ac_2 | PrecompileSource answers an unknown selector with an empty list. |
| <code>acceptance.14.277.1</code> | req | 14277.1 |
| <code>acceptance.14.277.1</code> | scenario | Given config/base.toml, when Settings are loaded, then sources.precompiles.enabled is true |
| <code>acceptance.14.277.1</code> | test | services/explorer/services/sig-provider/sig-provider-server/tests/settings.rs::precompiles_source_enabled_by_default (planned) |
| <code>acceptance.14.277.1</code> | qualification | UNRUN |
| <code>req.14277</code> | ac_1 | The base settings (services/explorer/services/sig-provider/sig-provider-server/config/base.toml:1) enable the precompile source. |
| <code>acceptance.14.278.1</code> | req | 14278.1 |
| <code>acceptance.14.278.1</code> | scenario | Given precompiles.json and the four abi.json files, when both signature sets are built, then they are equal |
| <code>acceptance.14.278.1</code> | test | services/explorer/services/sig-provider/sig-provider/src/sources/precompiles_abi_test.rs::embedded_table_matches_abi_files (planned) |
| <code>acceptance.14.278.1</code> | qualification | UNRUN |
| <code>req.14278</code> | ac_1 | The embedded precompile signature table equals the set of signatures in the four precompile ABI files. |
| <code>acceptance.14.279.1</code> | req | 14279.1 |
| <code>acceptance.14.279.1</code> | scenario | Given an aggregator over PrecompileSource and a mock source answering a different text for the same selector, when the selector is resolved, then the precompile text is first |
| <code>acceptance.14.279.1</code> | test | services/explorer/services/sig-provider/sig-provider/src/aggregator_precompiles_test.rs::precompile_signature_ranks_first (planned) |
| <code>acceptance.14.279.1</code> | qualification | UNRUN |
| <code>req.14279</code> | ac_1 | SourceAggregator (services/explorer/services/sig-provider/sig-provider/src/aggregator.rs:46) returns the precompile signature ahead of a colliding signature from another source. |
| <code>capability.14.30.1</code> | title | Add a sig-provider source that answers the paxeer precompile selectors |
| <code>capability.14.30.1</code> | reqs | 14276.1<br>14276.2 |
| <code>capability.14.30.2</code> | title | Enable the precompile source in sig-provider-server settings and startup |
| <code>capability.14.30.2</code> | reqs | 14277.1 |
| <code>capability.14.30.3</code> | title | Check the embedded precompile signatures against the precompile ABI files |
| <code>capability.14.30.3</code> | reqs | 14278.1 |
| <code>capability.14.30.4</code> | title | Rank precompile signatures first in the sig-provider aggregator |
| <code>capability.14.30.4</code> | reqs | 14279.1 |
| <code>resolution.17.950</code> | title | decision.1425 binds: The contract verifier stays at its baseline |
| <code>resolution.17.950</code> | applies_to | decision.1425<br>task.14.30.2 |
| <code>resolution.17.950</code> | source | decision.1425 in fragments/J54/decision.kvx; tasks from affects |
| <code>resolution.17.953</code> | title | decision.1428 binds: The sig-provider answers precompile selectors from the ABI files in the repository |
| <code>resolution.17.953</code> | applies_to | decision.1428<br>task.14.30.1<br>task.14.30.2<br>task.14.30.3<br>task.14.30.4 |
| <code>resolution.17.953</code> | source | decision.1428 in fragments/J54/decision.kvx; tasks from affects |
