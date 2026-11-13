# Explorer services contract

The signature provider uses [PrecompileSource](../../../services/explorer/services/sig-provider/sig-provider/src/sources/precompiles.rs) to resolve function selectors and event signatures from the embedded precompile ABI table. `sources.precompiles.enabled` controls the source and defaults to `true`.

## sig-provider-precompiles

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.cfg-sig-provider-sources-precompiles-j54</code> | <code>config_key</code> | [services/explorer/services/sig-provider/sig-provider-server/src/settings.rs](../../../services/explorer/services/sig-provider/sig-provider-server/src/settings.rs) | <code>[sources.precompiles] enabled = true</code> |
| <code>interface.14.rs-sig-provider-precompile-source-j54</code> | <code>rust_pub_item</code> | [services/explorer/services/sig-provider/sig-provider/src/sources/precompiles.rs](../../../services/explorer/services/sig-provider/sig-provider/src/sources/precompiles.rs) | <code>pub struct PrecompileSource; impl SignatureSource for PrecompileSource</code> |

| Field | Rule |
|---|---|
| interface | PrecompileSource behind [sources.precompiles] enabled = true |
| storage | Embedded table from the precompile ABI files. |
| errors | Unknown selectors answer an empty list. |
| events | none |
| gas | none (off-chain service) |
| invariants | The embedded table equals the ABI files at the same revision. |
| pause | none |
