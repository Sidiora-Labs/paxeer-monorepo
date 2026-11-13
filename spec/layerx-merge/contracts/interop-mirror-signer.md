# Interop mirror signer contract

The [mirror signer](../../../services/interop/crates/layerx-mirror-signer/src/lib.rs) signs publisher requests over a Unix-domain socket. The `bridge` command selects the bridge signing service and key manifest.

## cli-msigner

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cli-msigner</code> | <code>cli_command</code> | [services/interop/crates/layerx-mirror-signer/src/main.rs](../../../services/interop/crates/layerx-mirror-signer/src/main.rs) | <code>layerx-mirror-signer [bridge]</code> |

## env-msigner-layerx-bridge-signer-keys-file

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.env-msigner-layerx-bridge-signer-keys-file</code> | <code>env_var</code> | [services/interop/crates/layerx-mirror-signer/src/lib.rs](../../../services/interop/crates/layerx-mirror-signer/src/lib.rs) | <code>LAYERX_BRIDGE_SIGNER_KEYS_FILE</code> |

## env-msigner-layerx-bridge-signer-socket

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.env-msigner-layerx-bridge-signer-socket</code> | <code>env_var</code> | [services/interop/crates/layerx-mirror-signer/src/lib.rs](../../../services/interop/crates/layerx-mirror-signer/src/lib.rs) | <code>LAYERX_BRIDGE_SIGNER_SOCKET</code> |

## env-msigner-layerx-mirror-signer-ethereum-key-file

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.env-msigner-layerx-mirror-signer-ethereum-key-file</code> | <code>env_var</code> | [services/interop/crates/layerx-mirror-signer/src/lib.rs](../../../services/interop/crates/layerx-mirror-signer/src/lib.rs) | <code>LAYERX_MIRROR_SIGNER_ETHEREUM_KEY_FILE</code> |

## env-msigner-layerx-mirror-signer-ethereum-key-handle

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.env-msigner-layerx-mirror-signer-ethereum-key-handle</code> | <code>env_var</code> | [services/interop/crates/layerx-mirror-signer/src/lib.rs](../../../services/interop/crates/layerx-mirror-signer/src/lib.rs) | <code>LAYERX_MIRROR_SIGNER_ETHEREUM_KEY_HANDLE</code> |

## env-msigner-layerx-mirror-signer-socket

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.env-msigner-layerx-mirror-signer-socket</code> | <code>env_var</code> | [services/interop/crates/layerx-mirror-signer/src/lib.rs](../../../services/interop/crates/layerx-mirror-signer/src/lib.rs) | <code>LAYERX_MIRROR_SIGNER_SOCKET</code> |

## env-msigner-layerx-mirror-signer-solana-key-file

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.env-msigner-layerx-mirror-signer-solana-key-file</code> | <code>env_var</code> | [services/interop/crates/layerx-mirror-signer/src/lib.rs](../../../services/interop/crates/layerx-mirror-signer/src/lib.rs) | <code>LAYERX_MIRROR_SIGNER_SOLANA_KEY_FILE</code> |

## env-msigner-layerx-mirror-signer-solana-key-handle

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.env-msigner-layerx-mirror-signer-solana-key-handle</code> | <code>env_var</code> | [services/interop/crates/layerx-mirror-signer/src/lib.rs](../../../services/interop/crates/layerx-mirror-signer/src/lib.rs) | <code>LAYERX_MIRROR_SIGNER_SOLANA_KEY_HANDLE</code> |

## fmt-msigner-request

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-msigner-request</code> | <code>file_format</code> | [services/interop/crates/layerx-mirror-signer/src/lib.rs](../../../services/interop/crates/layerx-mirror-signer/src/lib.rs) | <code>signer request = u32be length &#124; "LXCS" &#124; version u16be=1 &#124; algorithm u8 (1 recoverable low-S secp256k1 over digest, message_len 0; 2 Ed25519 over message, digest = sha256(message)) &#124; handle_len u16be &#124; handle &#124; domain_len u16be &#124; domain &#124; digest 32 &#124; message_len u32be &#124; message; response = u32be length &#124; 01 refused &#124; 00 signature (65-byte r&#124;s&#124;recovery_id or 64-byte Ed25519)</code> |

## msigner-bridge-attest-inbound-domain

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-msigner-bridge-attest-inbound-domain</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror-signer/src/lib.rs](../../../services/interop/crates/layerx-mirror-signer/src/lib.rs) | <code>pub const BRIDGE_ATTEST_INBOUND_DOMAIN: &amp;[u8]</code> |

## msigner-bridge-attest-outbound-domain

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-msigner-bridge-attest-outbound-domain</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror-signer/src/lib.rs](../../../services/interop/crates/layerx-mirror-signer/src/lib.rs) | <code>pub const BRIDGE_ATTEST_OUTBOUND_DOMAIN: &amp;[u8]</code> |

## msigner-bridge-ethereum-transaction-domain

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-msigner-bridge-ethereum-transaction-domain</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror-signer/src/lib.rs](../../../services/interop/crates/layerx-mirror-signer/src/lib.rs) | <code>pub const BRIDGE_ETHEREUM_TRANSACTION_DOMAIN: &amp;[u8]</code> |

## msigner-bridge-keys-file-variable

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-msigner-bridge-keys-file-variable</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror-signer/src/lib.rs](../../../services/interop/crates/layerx-mirror-signer/src/lib.rs) | <code>pub const BRIDGE_KEYS_FILE_VARIABLE: &amp;str</code> |

## msigner-bridge-options

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-msigner-bridge-options</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror-signer/src/lib.rs](../../../services/interop/crates/layerx-mirror-signer/src/lib.rs) | <code>pub struct BridgeOptions</code> |

## msigner-bridge-options-parse

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-msigner-bridge-options-parse</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror-signer/src/lib.rs](../../../services/interop/crates/layerx-mirror-signer/src/lib.rs) | <code>pub fn parse&lt;I: IntoIterator&lt;Item = String&gt;&gt;(arguments: I) -&gt; Result&lt;Self, StartupError&gt;</code> |

## msigner-bridge-paxeer-transaction-domain

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-msigner-bridge-paxeer-transaction-domain</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror-signer/src/lib.rs](../../../services/interop/crates/layerx-mirror-signer/src/lib.rs) | <code>pub const BRIDGE_PAXEER_TRANSACTION_DOMAIN: &amp;[u8]</code> |

## msigner-bridge-socket-variable

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-msigner-bridge-socket-variable</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror-signer/src/lib.rs](../../../services/interop/crates/layerx-mirror-signer/src/lib.rs) | <code>pub const BRIDGE_SOCKET_VARIABLE: &amp;str</code> |

## msigner-bridge-solana-transaction-domain

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-msigner-bridge-solana-transaction-domain</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror-signer/src/lib.rs](../../../services/interop/crates/layerx-mirror-signer/src/lib.rs) | <code>pub const BRIDGE_SOLANA_TRANSACTION_DOMAIN: &amp;[u8]</code> |

## msigner-bridge-usage

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-msigner-bridge-usage</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror-signer/src/lib.rs](../../../services/interop/crates/layerx-mirror-signer/src/lib.rs) | <code>pub const BRIDGE_USAGE: &amp;str</code> |

## msigner-default-bridge-keys-file

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-msigner-default-bridge-keys-file</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror-signer/src/lib.rs](../../../services/interop/crates/layerx-mirror-signer/src/lib.rs) | <code>pub const DEFAULT_BRIDGE_KEYS_FILE: &amp;str</code> |

## msigner-default-bridge-socket

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-msigner-default-bridge-socket</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror-signer/src/lib.rs](../../../services/interop/crates/layerx-mirror-signer/src/lib.rs) | <code>pub const DEFAULT_BRIDGE_SOCKET: &amp;str</code> |

## msigner-default-ethereum-key-file

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-msigner-default-ethereum-key-file</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror-signer/src/lib.rs](../../../services/interop/crates/layerx-mirror-signer/src/lib.rs) | <code>pub const DEFAULT_ETHEREUM_KEY_FILE: &amp;str</code> |

## msigner-default-ethereum-key-handle

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-msigner-default-ethereum-key-handle</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror-signer/src/lib.rs](../../../services/interop/crates/layerx-mirror-signer/src/lib.rs) | <code>pub const DEFAULT_ETHEREUM_KEY_HANDLE: &amp;str</code> |

## msigner-default-socket

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-msigner-default-socket</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror-signer/src/lib.rs](../../../services/interop/crates/layerx-mirror-signer/src/lib.rs) | <code>pub const DEFAULT_SOCKET: &amp;str</code> |

## msigner-default-solana-key-handle

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-msigner-default-solana-key-handle</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror-signer/src/lib.rs](../../../services/interop/crates/layerx-mirror-signer/src/lib.rs) | <code>pub const DEFAULT_SOLANA_KEY_HANDLE: &amp;str</code> |

## msigner-default-solana-keypair-file

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-msigner-default-solana-keypair-file</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror-signer/src/lib.rs](../../../services/interop/crates/layerx-mirror-signer/src/lib.rs) | <code>pub const DEFAULT_SOLANA_KEYPAIR_FILE: &amp;str</code> |

## msigner-ethereum-key-file-variable

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-msigner-ethereum-key-file-variable</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror-signer/src/lib.rs](../../../services/interop/crates/layerx-mirror-signer/src/lib.rs) | <code>pub const ETHEREUM_KEY_FILE_VARIABLE: &amp;str</code> |

## msigner-ethereum-key-handle-variable

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-msigner-ethereum-key-handle-variable</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror-signer/src/lib.rs](../../../services/interop/crates/layerx-mirror-signer/src/lib.rs) | <code>pub const ETHEREUM_KEY_HANDLE_VARIABLE: &amp;str</code> |

## msigner-ethereum-policy-domain

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-msigner-ethereum-policy-domain</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror-signer/src/lib.rs](../../../services/interop/crates/layerx-mirror-signer/src/lib.rs) | <code>pub const ETHEREUM_POLICY_DOMAIN: &amp;[u8]</code> |

## msigner-options

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-msigner-options</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror-signer/src/lib.rs](../../../services/interop/crates/layerx-mirror-signer/src/lib.rs) | <code>pub struct Options</code> |

## msigner-options-from-environment

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-msigner-options-from-environment</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror-signer/src/lib.rs](../../../services/interop/crates/layerx-mirror-signer/src/lib.rs) | <code>pub fn from_environment() -&gt; Self</code> |

## msigner-options-parse

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-msigner-options-parse</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror-signer/src/lib.rs](../../../services/interop/crates/layerx-mirror-signer/src/lib.rs) | <code>pub fn parse&lt;I: IntoIterator&lt;Item = String&gt;&gt;(arguments: I) -&gt; Result&lt;Self, StartupError&gt;</code> |

## msigner-signer-listener

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-msigner-signer-listener</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror-signer/src/lib.rs](../../../services/interop/crates/layerx-mirror-signer/src/lib.rs) | <code>pub struct SignerListener</code> |

## msigner-signer-listener-bind

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-msigner-signer-listener-bind</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror-signer/src/lib.rs](../../../services/interop/crates/layerx-mirror-signer/src/lib.rs) | <code>pub fn bind(options: &amp;Options) -&gt; Result&lt;Self, StartupError&gt;</code> |

## msigner-signer-listener-bind-bridge

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-msigner-signer-listener-bind-bridge</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror-signer/src/lib.rs](../../../services/interop/crates/layerx-mirror-signer/src/lib.rs) | <code>pub fn bind_bridge(options: &amp;BridgeOptions) -&gt; Result&lt;Self, StartupError&gt;</code> |

## msigner-signer-listener-serve

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-msigner-signer-listener-serve</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror-signer/src/lib.rs](../../../services/interop/crates/layerx-mirror-signer/src/lib.rs) | <code>pub fn serve(&amp;self) -&gt; !</code> |

## msigner-signer-listener-serve-once

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-msigner-signer-listener-serve-once</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror-signer/src/lib.rs](../../../services/interop/crates/layerx-mirror-signer/src/lib.rs) | <code>pub fn serve_once(&amp;self) -&gt; Result&lt;(), std::io::Error&gt;</code> |

## msigner-signer-listener-service

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-msigner-signer-listener-service</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror-signer/src/lib.rs](../../../services/interop/crates/layerx-mirror-signer/src/lib.rs) | <code>pub fn service(&amp;self) -&gt; &amp;SignerService</code> |

## msigner-signer-listener-socket

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-msigner-signer-listener-socket</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror-signer/src/lib.rs](../../../services/interop/crates/layerx-mirror-signer/src/lib.rs) | <code>pub fn socket(&amp;self) -&gt; &amp;Path</code> |

## msigner-signer-service

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-msigner-signer-service</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror-signer/src/lib.rs](../../../services/interop/crates/layerx-mirror-signer/src/lib.rs) | <code>pub struct SignerService</code> |

## msigner-signer-service-handles

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-msigner-signer-service-handles</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror-signer/src/lib.rs](../../../services/interop/crates/layerx-mirror-signer/src/lib.rs) | <code>pub fn handles(&amp;self) -&gt; Vec&lt;&amp;str&gt;</code> |

## msigner-signer-service-load

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-msigner-signer-service-load</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror-signer/src/lib.rs](../../../services/interop/crates/layerx-mirror-signer/src/lib.rs) | <code>pub fn load(options: &amp;Options) -&gt; Result&lt;Self, StartupError&gt;</code> |

## msigner-signer-service-load-bridge

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-msigner-signer-service-load-bridge</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror-signer/src/lib.rs](../../../services/interop/crates/layerx-mirror-signer/src/lib.rs) | <code>pub fn load_bridge(options: &amp;BridgeOptions) -&gt; Result&lt;Self, StartupError&gt;</code> |

## msigner-signer-service-serves-solana

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-msigner-signer-service-serves-solana</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror-signer/src/lib.rs](../../../services/interop/crates/layerx-mirror-signer/src/lib.rs) | <code>pub fn serves_solana(&amp;self) -&gt; bool</code> |

## msigner-socket-variable

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-msigner-socket-variable</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror-signer/src/lib.rs](../../../services/interop/crates/layerx-mirror-signer/src/lib.rs) | <code>pub const SOCKET_VARIABLE: &amp;str</code> |

## msigner-solana-key-file-variable

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-msigner-solana-key-file-variable</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror-signer/src/lib.rs](../../../services/interop/crates/layerx-mirror-signer/src/lib.rs) | <code>pub const SOLANA_KEY_FILE_VARIABLE: &amp;str</code> |

## msigner-solana-key-handle-variable

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-msigner-solana-key-handle-variable</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror-signer/src/lib.rs](../../../services/interop/crates/layerx-mirror-signer/src/lib.rs) | <code>pub const SOLANA_KEY_HANDLE_VARIABLE: &amp;str</code> |

## msigner-solana-policy-domain

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-msigner-solana-policy-domain</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror-signer/src/lib.rs](../../../services/interop/crates/layerx-mirror-signer/src/lib.rs) | <code>pub const SOLANA_POLICY_DOMAIN: &amp;[u8]</code> |

## msigner-startup-error

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-msigner-startup-error</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror-signer/src/lib.rs](../../../services/interop/crates/layerx-mirror-signer/src/lib.rs) | <code>pub enum StartupError</code> |

## msigner-usage

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-msigner-usage</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-mirror-signer/src/lib.rs](../../../services/interop/crates/layerx-mirror-signer/src/lib.rs) | <code>pub const USAGE: &amp;str</code> |
