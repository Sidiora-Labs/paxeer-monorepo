# Native node build components

The node links the C kernel, Rust program sandbox and EVM native library. The repository [Makefile](../../../Makefile) declares the build targets; the target names below describe build behavior without asserting a particular binary has been built.

## Node targets

| Target | Behavior |
|---|---|
| `make paxd-native-libs` | Builds the kernel library, program release archives and evmone library. |
| `make paxd-build` | Builds native dependencies, reads the release version and invokes the node build. |
| `make paxd-version-check` | Compares `build/paxd version` with the release version file. |

[paxd-build](../../../Makefile) reads [tools/release/version.json](../../../tools/release/version.json). Node Go dependencies are declared in [go.mod](../../../go.mod); [programs/Cargo.toml](../../../programs/Cargo.toml) declares the runtime workspace and its release profiles. [modules/lxphost/host.go](../../../modules/lxphost/host.go) declares the cgo link inputs.

## Program guest

The AI market crate's release profile uses `opt-level = "z"`. Its [deployment-domain reader](../../../programs/crates/layerx-programs-ai-market/src/lib.rs) uses the merged chain-125 domain when the build-time `PAXAI_CHAIN_DOMAIN` setting is absent and requires an explicit value to match that domain. The domain is SHA-256 of `PAXAI/chain-domain/v1` followed by chain ID `125` as a big-endian `u64`.

The [guest recipe](../../../programs/crates/layerx-programs-ai-market/tests/fixtures/guest-recipe.sh) compiles a release `cdylib` for `wasm32-unknown-unknown`, then packages it using `wasm-opt` from Binaryen 133. The `paxai-host-boundary-build` target limits the packaged guest to `524184` bytes, leaving room for the `104`-byte envelope within the `524288`-byte deployment payload cap. See [the AI market crate](../../../programs/crates/layerx-programs-ai-market) and [programs contract](../contracts/programs.md).

## Runtime assembly

The node imports native merge constructors and gates their addresses by the active upgrade and module. Thirteen merge addresses are declared: `0x101A`–`0x101E` and `0x0901`–`0x0908`. The [design](../design.body.md) describes host execution, authority and storage; the [contracts index](../contracts/README.md) links the served interfaces.

Paxeer mainnet-beta uses chain ID `125` (`0x7d`). Build artifacts alone do not establish upgrade activation or endpoint availability. Transaction finality follows BFT block commitment.

<a id="build-report-final-build-j100"></a>
<a id="result"></a>
<a id="document-shape-speckvx-and-interfaceskvx"></a>
<a id="hyperfide-cli-defects-cg-220-measured-on-this-plan"></a>
<a id="operational-precondition-for-the-fleet"></a>
<a id="sweeps-j98"></a>
<a id="warnings-19-all-accepted"></a>
<a id="changes-made-by-j98"></a>
<a id="drift-audit-j100"></a>
<a id="stale-generators-do-not-rerun-their-fragments-were-edited-in-place"></a>
<a id="open"></a>
