# Interop mirror signer contract

Source revision: `2a1033aa2`. This contract freezes the cited interface and acceptance blocks of the specification. Statuses include the specification overrides. A source marked planned has line 0; a positive source line is verified in the pinned revision.

The context and decision tables reproduce the rules linked by interface identifiers. An anchor without a linked context gains no additional storage, gas, activation or pause rule here. Acceptance qualification remains UNRUN until its declared gate succeeds.

## cli-msigner

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cli-msigner</code> | <code>cli_command</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror-signer/src/main.rs:13</code> | <code>layerx-mirror-signer [bridge]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cli-msigner</code> | notes | default serves the mirror signer socket; bridge loads LAYERX_BRIDGE_SIGNER_KEYS_FILE and answers the co-located bridge relayer; 29 reference lines in 14 files; callers lists the first reference in each file |

## env-msigner-layerx-bridge-signer-keys-file

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.env-msigner-layerx-bridge-signer-keys-file</code> | <code>env_var</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror-signer/src/lib.rs:68</code> | <code>LAYERX_BRIDGE_SIGNER_KEYS_FILE: bridge key manifest path (BRIDGE_KEYS_FILE_VARIABLE)</code> |

## env-msigner-layerx-bridge-signer-socket

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.env-msigner-layerx-bridge-signer-socket</code> | <code>env_var</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror-signer/src/lib.rs:66</code> | <code>LAYERX_BRIDGE_SIGNER_SOCKET: Unix socket path of the bridge signer mode (BRIDGE_SOCKET_VARIABLE)</code> |

## env-msigner-layerx-mirror-signer-ethereum-key-file

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.env-msigner-layerx-mirror-signer-ethereum-key-file</code> | <code>env_var</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror-signer/src/lib.rs:44</code> | <code>LAYERX_MIRROR_SIGNER_ETHEREUM_KEY_FILE: Ethereum secp256k1 key file (ETHEREUM_KEY_FILE_VARIABLE)</code> |

## env-msigner-layerx-mirror-signer-ethereum-key-handle

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.env-msigner-layerx-mirror-signer-ethereum-key-handle</code> | <code>env_var</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror-signer/src/lib.rs:48</code> | <code>LAYERX_MIRROR_SIGNER_ETHEREUM_KEY_HANDLE: Ethereum key handle name (ETHEREUM_KEY_HANDLE_VARIABLE)</code> |

## env-msigner-layerx-mirror-signer-socket

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.env-msigner-layerx-mirror-signer-socket</code> | <code>env_var</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror-signer/src/lib.rs:42</code> | <code>LAYERX_MIRROR_SIGNER_SOCKET: Unix socket path the mirror signer serves (SOCKET_VARIABLE)</code> |

## env-msigner-layerx-mirror-signer-solana-key-file

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.env-msigner-layerx-mirror-signer-solana-key-file</code> | <code>env_var</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror-signer/src/lib.rs:46</code> | <code>LAYERX_MIRROR_SIGNER_SOLANA_KEY_FILE: Solana ed25519 key file (SOLANA_KEY_FILE_VARIABLE)</code> |

## env-msigner-layerx-mirror-signer-solana-key-handle

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.env-msigner-layerx-mirror-signer-solana-key-handle</code> | <code>env_var</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror-signer/src/lib.rs:50</code> | <code>LAYERX_MIRROR_SIGNER_SOLANA_KEY_HANDLE: Solana key handle name (SOLANA_KEY_HANDLE_VARIABLE)</code> |

## fmt-msigner-request

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-msigner-request</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror-signer/src/lib.rs:78</code> | <code>signer request = u32be length &#124; "LXCS" &#124; version u16be=1 &#124; algorithm u8 (1 recoverable low-S secp256k1 over digest, message_len 0; 2 Ed25519 over message, digest = sha256(message)) &#124; handle_len u16be &#124; handle &#124; domain_len u16be &#124; domain &#124; digest 32 &#124; message_len u32be &#124; message; response = u32be length &#124; 01 refused &#124; 00 signature (65-byte r&#124;s&#124;recovery_id or 64-byte Ed25519)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-msigner-request</code> | notes | served on a Unix socket or mutual-TLS; parser services/interop/crates/layerx-mirror-signer/src/lib.rs:368; documented in services/interop/deploy/mirror/signer-protocol.md; stale task 7.7.5 do_4 keeps the signer client and keys (services/interop/crates/layerx-mirror-signer/src/lib.rs:428) |

## msigner-bridge-attest-inbound-domain

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-msigner-bridge-attest-inbound-domain</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror-signer/src/lib.rs:52</code> | <code>pub const BRIDGE_ATTEST_INBOUND_DOMAIN: &amp;[u8]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-msigner-bridge-attest-inbound-domain</code> | notes | const layerx_mirror_signer::BRIDGE_ATTEST_INBOUND_DOMAIN |

## msigner-bridge-attest-outbound-domain

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-msigner-bridge-attest-outbound-domain</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror-signer/src/lib.rs:54</code> | <code>pub const BRIDGE_ATTEST_OUTBOUND_DOMAIN: &amp;[u8]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-msigner-bridge-attest-outbound-domain</code> | notes | const layerx_mirror_signer::BRIDGE_ATTEST_OUTBOUND_DOMAIN |

## msigner-bridge-ethereum-transaction-domain

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-msigner-bridge-ethereum-transaction-domain</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror-signer/src/lib.rs:58</code> | <code>pub const BRIDGE_ETHEREUM_TRANSACTION_DOMAIN: &amp;[u8]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-msigner-bridge-ethereum-transaction-domain</code> | notes | const layerx_mirror_signer::BRIDGE_ETHEREUM_TRANSACTION_DOMAIN |

## msigner-bridge-keys-file-variable

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-msigner-bridge-keys-file-variable</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror-signer/src/lib.rs:68</code> | <code>pub const BRIDGE_KEYS_FILE_VARIABLE: &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-msigner-bridge-keys-file-variable</code> | notes | const layerx_mirror_signer::BRIDGE_KEYS_FILE_VARIABLE; no reference outside the defining file found by name match |

## msigner-bridge-options

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-msigner-bridge-options</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror-signer/src/lib.rs:272</code> | <code>pub struct BridgeOptions</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-msigner-bridge-options</code> | notes | struct layerx_mirror_signer::BridgeOptions |

## msigner-bridge-options-parse

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-msigner-bridge-options-parse</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror-signer/src/lib.rs:296</code> | <code>pub fn parse&lt;I: IntoIterator&lt;Item = String&gt;&gt;(arguments: I) -&gt; Result&lt;Self, StartupError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-msigner-bridge-options-parse</code> | notes | method layerx_mirror_signer::BridgeOptions::parse |

## msigner-bridge-paxeer-transaction-domain

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-msigner-bridge-paxeer-transaction-domain</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror-signer/src/lib.rs:56</code> | <code>pub const BRIDGE_PAXEER_TRANSACTION_DOMAIN: &amp;[u8]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-msigner-bridge-paxeer-transaction-domain</code> | notes | const layerx_mirror_signer::BRIDGE_PAXEER_TRANSACTION_DOMAIN |

## msigner-bridge-socket-variable

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-msigner-bridge-socket-variable</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror-signer/src/lib.rs:66</code> | <code>pub const BRIDGE_SOCKET_VARIABLE: &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-msigner-bridge-socket-variable</code> | notes | const layerx_mirror_signer::BRIDGE_SOCKET_VARIABLE; no reference outside the defining file found by name match |

## msigner-bridge-solana-transaction-domain

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-msigner-bridge-solana-transaction-domain</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror-signer/src/lib.rs:60</code> | <code>pub const BRIDGE_SOLANA_TRANSACTION_DOMAIN: &amp;[u8]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-msigner-bridge-solana-transaction-domain</code> | notes | const layerx_mirror_signer::BRIDGE_SOLANA_TRANSACTION_DOMAIN |

## msigner-bridge-usage

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-msigner-bridge-usage</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror-signer/src/lib.rs:71</code> | <code>pub const BRIDGE_USAGE: &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-msigner-bridge-usage</code> | notes | const layerx_mirror_signer::BRIDGE_USAGE; no reference outside the defining file found by name match |

## msigner-default-bridge-keys-file

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-msigner-default-bridge-keys-file</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror-signer/src/lib.rs:64</code> | <code>pub const DEFAULT_BRIDGE_KEYS_FILE: &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-msigner-default-bridge-keys-file</code> | notes | const layerx_mirror_signer::DEFAULT_BRIDGE_KEYS_FILE; no reference outside the defining file found by name match |

## msigner-default-bridge-socket

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-msigner-default-bridge-socket</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror-signer/src/lib.rs:62</code> | <code>pub const DEFAULT_BRIDGE_SOCKET: &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-msigner-default-bridge-socket</code> | notes | const layerx_mirror_signer::DEFAULT_BRIDGE_SOCKET; no reference outside the defining file found by name match |

## msigner-default-ethereum-key-file

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-msigner-default-ethereum-key-file</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror-signer/src/lib.rs:34</code> | <code>pub const DEFAULT_ETHEREUM_KEY_FILE: &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-msigner-default-ethereum-key-file</code> | notes | const layerx_mirror_signer::DEFAULT_ETHEREUM_KEY_FILE; no reference outside the defining file found by name match |

## msigner-default-ethereum-key-handle

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-msigner-default-ethereum-key-handle</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror-signer/src/lib.rs:38</code> | <code>pub const DEFAULT_ETHEREUM_KEY_HANDLE: &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-msigner-default-ethereum-key-handle</code> | notes | const layerx_mirror_signer::DEFAULT_ETHEREUM_KEY_HANDLE; no reference outside the defining file found by name match |

## msigner-default-socket

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-msigner-default-socket</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror-signer/src/lib.rs:32</code> | <code>pub const DEFAULT_SOCKET: &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-msigner-default-socket</code> | notes | const layerx_mirror_signer::DEFAULT_SOCKET; no reference outside the defining file found by name match |

## msigner-default-solana-key-handle

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-msigner-default-solana-key-handle</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror-signer/src/lib.rs:40</code> | <code>pub const DEFAULT_SOLANA_KEY_HANDLE: &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-msigner-default-solana-key-handle</code> | notes | const layerx_mirror_signer::DEFAULT_SOLANA_KEY_HANDLE; no reference outside the defining file found by name match |

## msigner-default-solana-keypair-file

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-msigner-default-solana-keypair-file</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror-signer/src/lib.rs:36</code> | <code>pub const DEFAULT_SOLANA_KEYPAIR_FILE: &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-msigner-default-solana-keypair-file</code> | notes | const layerx_mirror_signer::DEFAULT_SOLANA_KEYPAIR_FILE; no reference outside the defining file found by name match |

## msigner-ethereum-key-file-variable

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-msigner-ethereum-key-file-variable</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror-signer/src/lib.rs:44</code> | <code>pub const ETHEREUM_KEY_FILE_VARIABLE: &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-msigner-ethereum-key-file-variable</code> | notes | const layerx_mirror_signer::ETHEREUM_KEY_FILE_VARIABLE; no reference outside the defining file found by name match |

## msigner-ethereum-key-handle-variable

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-msigner-ethereum-key-handle-variable</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror-signer/src/lib.rs:48</code> | <code>pub const ETHEREUM_KEY_HANDLE_VARIABLE: &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-msigner-ethereum-key-handle-variable</code> | notes | const layerx_mirror_signer::ETHEREUM_KEY_HANDLE_VARIABLE; no reference outside the defining file found by name match |

## msigner-ethereum-policy-domain

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-msigner-ethereum-policy-domain</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror-signer/src/lib.rs:28</code> | <code>pub const ETHEREUM_POLICY_DOMAIN: &amp;[u8]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-msigner-ethereum-policy-domain</code> | notes | const layerx_mirror_signer::ETHEREUM_POLICY_DOMAIN |

## msigner-options

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-msigner-options</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror-signer/src/lib.rs:171</code> | <code>pub struct Options</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-msigner-options</code> | notes | struct layerx_mirror_signer::Options |

## msigner-options-from-environment

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-msigner-options-from-environment</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror-signer/src/lib.rs:206</code> | <code>pub fn from_environment() -&gt; Self</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-msigner-options-from-environment</code> | notes | method layerx_mirror_signer::Options::from_environment; no reference outside the defining file found by name match |

## msigner-options-parse

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-msigner-options-parse</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror-signer/src/lib.rs:232</code> | <code>pub fn parse&lt;I: IntoIterator&lt;Item = String&gt;&gt;(arguments: I) -&gt; Result&lt;Self, StartupError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-msigner-options-parse</code> | notes | method layerx_mirror_signer::Options::parse |

## msigner-signer-listener

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-msigner-signer-listener</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror-signer/src/lib.rs:652</code> | <code>pub struct SignerListener</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-msigner-signer-listener</code> | notes | struct layerx_mirror_signer::SignerListener |

## msigner-signer-listener-bind

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-msigner-signer-listener-bind</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror-signer/src/lib.rs:666</code> | <code>pub fn bind(options: &amp;Options) -&gt; Result&lt;Self, StartupError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-msigner-signer-listener-bind</code> | notes | method layerx_mirror_signer::SignerListener::bind |

## msigner-signer-listener-bind-bridge

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-msigner-signer-listener-bind-bridge</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror-signer/src/lib.rs:676</code> | <code>pub fn bind_bridge(options: &amp;BridgeOptions) -&gt; Result&lt;Self, StartupError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-msigner-signer-listener-bind-bridge</code> | notes | method layerx_mirror_signer::SignerListener::bind_bridge |

## msigner-signer-listener-serve

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-msigner-signer-listener-serve</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror-signer/src/lib.rs:760</code> | <code>pub fn serve(&amp;self) -&gt; !</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-msigner-signer-listener-serve</code> | notes | method layerx_mirror_signer::SignerListener::serve; no reference outside the defining file found by name match |

## msigner-signer-listener-serve-once

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-msigner-signer-listener-serve-once</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror-signer/src/lib.rs:751</code> | <code>pub fn serve_once(&amp;self) -&gt; Result&lt;(), std::io::Error&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-msigner-signer-listener-serve-once</code> | notes | method layerx_mirror_signer::SignerListener::serve_once; no reference outside the defining file found by name match |

## msigner-signer-listener-service

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-msigner-signer-listener-service</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror-signer/src/lib.rs:740</code> | <code>pub fn service(&amp;self) -&gt; &amp;SignerService</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-msigner-signer-listener-service</code> | notes | method layerx_mirror_signer::SignerListener::service |

## msigner-signer-listener-socket

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-msigner-signer-listener-socket</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror-signer/src/lib.rs:734</code> | <code>pub fn socket(&amp;self) -&gt; &amp;Path</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-msigner-signer-listener-socket</code> | notes | method layerx_mirror_signer::SignerListener::socket |

## msigner-signer-service

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-msigner-signer-service</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror-signer/src/lib.rs:412</code> | <code>pub struct SignerService</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-msigner-signer-service</code> | notes | struct layerx_mirror_signer::SignerService |

## msigner-signer-service-handles

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-msigner-signer-service-handles</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror-signer/src/lib.rs:568</code> | <code>pub fn handles(&amp;self) -&gt; Vec&lt;&amp;str&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-msigner-signer-service-handles</code> | notes | method layerx_mirror_signer::SignerService::handles |

## msigner-signer-service-load

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-msigner-signer-service-load</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror-signer/src/lib.rs:427</code> | <code>pub fn load(options: &amp;Options) -&gt; Result&lt;Self, StartupError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-msigner-signer-service-load</code> | notes | method layerx_mirror_signer::SignerService::load; no reference outside the defining file found by name match |

## msigner-signer-service-load-bridge

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-msigner-signer-service-load-bridge</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror-signer/src/lib.rs:456</code> | <code>pub fn load_bridge(options: &amp;BridgeOptions) -&gt; Result&lt;Self, StartupError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-msigner-signer-service-load-bridge</code> | notes | method layerx_mirror_signer::SignerService::load_bridge |

## msigner-signer-service-serves-solana

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-msigner-signer-service-serves-solana</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror-signer/src/lib.rs:577</code> | <code>pub fn serves_solana(&amp;self) -&gt; bool</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-msigner-signer-service-serves-solana</code> | notes | method layerx_mirror_signer::SignerService::serves_solana; no reference outside the defining file found by name match |

## msigner-socket-variable

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-msigner-socket-variable</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror-signer/src/lib.rs:42</code> | <code>pub const SOCKET_VARIABLE: &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-msigner-socket-variable</code> | notes | const layerx_mirror_signer::SOCKET_VARIABLE; no reference outside the defining file found by name match |

## msigner-solana-key-file-variable

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-msigner-solana-key-file-variable</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror-signer/src/lib.rs:46</code> | <code>pub const SOLANA_KEY_FILE_VARIABLE: &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-msigner-solana-key-file-variable</code> | notes | const layerx_mirror_signer::SOLANA_KEY_FILE_VARIABLE; no reference outside the defining file found by name match |

## msigner-solana-key-handle-variable

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-msigner-solana-key-handle-variable</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror-signer/src/lib.rs:50</code> | <code>pub const SOLANA_KEY_HANDLE_VARIABLE: &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-msigner-solana-key-handle-variable</code> | notes | const layerx_mirror_signer::SOLANA_KEY_HANDLE_VARIABLE; no reference outside the defining file found by name match |

## msigner-solana-policy-domain

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-msigner-solana-policy-domain</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror-signer/src/lib.rs:30</code> | <code>pub const SOLANA_POLICY_DOMAIN: &amp;[u8]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-msigner-solana-policy-domain</code> | notes | const layerx_mirror_signer::SOLANA_POLICY_DOMAIN |

## msigner-startup-error

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-msigner-startup-error</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror-signer/src/lib.rs:125</code> | <code>pub enum StartupError</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-msigner-startup-error</code> | notes | enum layerx_mirror_signer::StartupError |

## msigner-usage

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-msigner-usage</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-mirror-signer/src/lib.rs:75</code> | <code>pub const USAGE: &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-msigner-usage</code> | notes | const layerx_mirror_signer::USAGE; no reference outside the defining file found by name match |
