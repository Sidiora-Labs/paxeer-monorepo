# Platform cli contract

Source revision: `2a1033aa2`. This contract freezes the cited interface and acceptance blocks of the specification. Statuses include the specification overrides. A source marked planned has line 0; a positive source line is verified in the pinned revision.

The context and decision tables reproduce the rules linked by interface identifiers. An anchor without a linked context gains no additional storage, gas, activation or pause rule here. Acceptance qualification remains UNRUN until its declared gate succeeds.

## cli-cli-commands

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.cli-cli-layerx</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:86</code> | <code>struct Cli {</code> |
| <code>interface.8.cli-cli-layerx-a2a</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:153</code> | <code>A2a(A2aCommand),</code> |
| <code>interface.8.cli-cli-layerx-a2a-serve</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:540</code> | <code>Serve {</code> |
| <code>interface.8.cli-cli-layerx-a2a-serve-asset</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:550</code> | <code>asset: Option&lt;String&gt;,</code> |
| <code>interface.8.cli-cli-layerx-a2a-serve-authorization-file</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:554</code> | <code>authorization_file: PathBuf,</code> |
| <code>interface.8.cli-cli-layerx-a2a-serve-environment</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:542</code> | <code>environment: Option&lt;String&gt;,</code> |
| <code>interface.8.cli-cli-layerx-a2a-serve-gateway-credential</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:546</code> | <code>gateway_credential: String,</code> |
| <code>interface.8.cli-cli-layerx-a2a-serve-key</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:544</code> | <code>key: Option&lt;String&gt;,</code> |
| <code>interface.8.cli-cli-layerx-a2a-serve-listen</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:552</code> | <code>listen: String,</code> |
| <code>interface.8.cli-cli-layerx-a2a-serve-read-only</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:556</code> | <code>read_only: bool,</code> |
| <code>interface.8.cli-cli-layerx-a2a-serve-source-account</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:548</code> | <code>source_account: Option&lt;String&gt;,</code> |
| <code>interface.8.cli-cli-layerx-a2a-start</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:559</code> | <code>Start,</code> |
| <code>interface.8.cli-cli-layerx-a2a-status</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:563</code> | <code>Status,</code> |
| <code>interface.8.cli-cli-layerx-a2a-stop</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:561</code> | <code>Stop,</code> |
| <code>interface.8.cli-cli-layerx-account</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:128</code> | <code>Account(AccountCommand),</code> |
| <code>interface.8.cli-cli-layerx-account-create</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:229</code> | <code>Create {</code> |
| <code>interface.8.cli-cli-layerx-account-create-display-name</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:237</code> | <code>display_name: Option&lt;String&gt;,</code> |
| <code>interface.8.cli-cli-layerx-account-create-email</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:235</code> | <code>email: Option&lt;String&gt;,</code> |
| <code>interface.8.cli-cli-layerx-account-create-idempotency-key</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:239</code> | <code>idempotency_key: Option&lt;String&gt;,</code> |
| <code>interface.8.cli-cli-layerx-account-create-initial-amount</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:233</code> | <code>initial_amount: String,</code> |
| <code>interface.8.cli-cli-layerx-account-create-key</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:231</code> | <code>key: Option&lt;String&gt;,</code> |
| <code>interface.8.cli-cli-layerx-account-get</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:242</code> | <code>Get {</code> |
| <code>interface.8.cli-cli-layerx-account-get-did</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:244</code> | <code>did: Option&lt;String&gt;,</code> |
| <code>interface.8.cli-cli-layerx-auth</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:125</code> | <code>Auth(AuthCommand),</code> |
| <code>interface.8.cli-cli-layerx-auth-delete</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:220</code> | <code>Delete {</code> |
| <code>interface.8.cli-cli-layerx-auth-delete-environment</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:222</code> | <code>environment: Option&lt;String&gt;,</code> |
| <code>interface.8.cli-cli-layerx-auth-set</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:210</code> | <code>Set {</code> |
| <code>interface.8.cli-cli-layerx-auth-set-environment</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:212</code> | <code>environment: Option&lt;String&gt;,</code> |
| <code>interface.8.cli-cli-layerx-auth-status</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:215</code> | <code>Status {</code> |
| <code>interface.8.cli-cli-layerx-auth-status-environment</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:217</code> | <code>environment: Option&lt;String&gt;,</code> |
| <code>interface.8.cli-cli-layerx-emulator</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:144</code> | <code>Emulator(EmulatorCommand),</code> |
| <code>interface.8.cli-cli-layerx-emulator-provision</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:463</code> | <code>Provision {</code> |
| <code>interface.8.cli-cli-layerx-emulator-provision-force</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:466</code> | <code>force: bool,</code> |
| <code>interface.8.cli-cli-layerx-emulator-up</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:461</code> | <code>Up(EmulatorUpArgs),</code> |
| <code>interface.8.cli-cli-layerx-emulator-up-listen</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:473</code> | <code>listen: Option&lt;String&gt;,</code> |
| <code>interface.8.cli-cli-layerx-emulator-up-network-id</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:475</code> | <code>network_id: Option&lt;u32&gt;,</code> |
| <code>interface.8.cli-cli-layerx-emulator-up-prefund</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:481</code> | <code>prefund: Vec&lt;String&gt;,</code> |
| <code>interface.8.cli-cli-layerx-emulator-up-protocol-version</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:477</code> | <code>protocol_version: Option&lt;u16&gt;,</code> |
| <code>interface.8.cli-cli-layerx-emulator-up-sequencer-seed-file</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:483</code> | <code>sequencer_seed_file: PathBuf,</code> |
| <code>interface.8.cli-cli-layerx-emulator-up-time-ms</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:479</code> | <code>time_ms: Option&lt;u64&gt;,</code> |
| <code>interface.8.cli-cli-layerx-environment</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:119</code> | <code>Environment(EnvironmentCommand),</code> |
| <code>interface.8.cli-cli-layerx-environment-current</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:168</code> | <code>Current,</code> |
| <code>interface.8.cli-cli-layerx-environment-list</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:166</code> | <code>List,</code> |
| <code>interface.8.cli-cli-layerx-environment-use</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:170</code> | <code>Use {</code> |
| <code>interface.8.cli-cli-layerx-environment-use-endpoint</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:173</code> | <code>endpoint: Option&lt;String&gt;,</code> |
| <code>interface.8.cli-cli-layerx-environment-use-name</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:171</code> | <code>name: String,</code> |
| <code>interface.8.cli-cli-layerx-environment-use-network-id</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:175</code> | <code>network_id: Option&lt;u32&gt;,</code> |
| <code>interface.8.cli-cli-layerx-environment-use-sequencer-trust-anchor</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:177</code> | <code>sequencer_trust_anchor: Option&lt;String&gt;,</code> |
| <code>interface.8.cli-cli-layerx-environment-use-sequencer-trust-anchor-file</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:179</code> | <code>sequencer_trust_anchor_file: Option&lt;PathBuf&gt;,</code> |
| <code>interface.8.cli-cli-layerx-faucet</code> | <code>cli_command</code> | <code>change</code> | <code>tools/platform/cli/src/main.rs:132</code> | <code>Faucet(faucet::FaucetArgs),</code> |
| <code>interface.8.cli-cli-layerx-faucet-key</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/faucet.rs:11</code> | <code>key: Option&lt;String&gt;,</code> |
| <code>interface.8.cli-cli-layerx-gateway-credential</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:100</code> | <code>gateway_credential: Option&lt;String&gt;,</code> |
| <code>interface.8.cli-cli-layerx-install</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:147</code> | <code>Install(InstallCommand),</code> |
| <code>interface.8.cli-cli-layerx-install-a2a</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:491</code> | <code>A2a(InstallA2aArgs),</code> |
| <code>interface.8.cli-cli-layerx-install-a2a-asset</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:523</code> | <code>asset: Option&lt;String&gt;,</code> |
| <code>interface.8.cli-cli-layerx-install-a2a-environment</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:507</code> | <code>environment: Option&lt;String&gt;,</code> |
| <code>interface.8.cli-cli-layerx-install-a2a-key</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:511</code> | <code>key: Option&lt;String&gt;,</code> |
| <code>interface.8.cli-cli-layerx-install-a2a-listen</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:509</code> | <code>listen: String,</code> |
| <code>interface.8.cli-cli-layerx-install-a2a-read-only</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:515</code> | <code>read_only: bool,</code> |
| <code>interface.8.cli-cli-layerx-install-a2a-rotate</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:519</code> | <code>rotate: bool,</code> |
| <code>interface.8.cli-cli-layerx-install-a2a-source-account</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:521</code> | <code>source_account: Option&lt;String&gt;,</code> |
| <code>interface.8.cli-cli-layerx-install-a2a-token-stdin</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:517</code> | <code>token_stdin: bool,</code> |
| <code>interface.8.cli-cli-layerx-install-a2a-well-known</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:513</code> | <code>well_known: Option&lt;PathBuf&gt;,</code> |
| <code>interface.8.cli-cli-layerx-install-mcp</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:489</code> | <code>Mcp(InstallMcpArgs),</code> |
| <code>interface.8.cli-cli-layerx-install-mcp-daemon-binding</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:501</code> | <code>daemon_binding: Option&lt;PathBuf&gt;,</code> |
| <code>interface.8.cli-cli-layerx-install-mcp-host</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:497</code> | <code>host: Vec&lt;String&gt;,</code> |
| <code>interface.8.cli-cli-layerx-install-mcp-read-only</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:499</code> | <code>read_only: bool,</code> |
| <code>interface.8.cli-cli-layerx-json</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:92</code> | <code>json: bool,</code> |
| <code>interface.8.cli-cli-layerx-key</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:122</code> | <code>Key(KeyCommand),</code> |
| <code>interface.8.cli-cli-layerx-key-create</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:186</code> | <code>Create {</code> |
| <code>interface.8.cli-cli-layerx-key-create-did</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:189</code> | <code>did: Option&lt;String&gt;,</code> |
| <code>interface.8.cli-cli-layerx-key-create-name</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:187</code> | <code>name: String,</code> |
| <code>interface.8.cli-cli-layerx-key-default</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:202</code> | <code>Default { name: String },</code> |
| <code>interface.8.cli-cli-layerx-key-default-name</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:202</code> | <code>Default { name: String },</code> |
| <code>interface.8.cli-cli-layerx-key-delete</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:204</code> | <code>Delete { name: String },</code> |
| <code>interface.8.cli-cli-layerx-key-delete-name</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:204</code> | <code>Delete { name: String },</code> |
| <code>interface.8.cli-cli-layerx-key-import</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:192</code> | <code>Import {</code> |
| <code>interface.8.cli-cli-layerx-key-import-did</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:195</code> | <code>did: Option&lt;String&gt;,</code> |
| <code>interface.8.cli-cli-layerx-key-import-name</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:193</code> | <code>name: String,</code> |
| <code>interface.8.cli-cli-layerx-key-list</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:198</code> | <code>List,</code> |
| <code>interface.8.cli-cli-layerx-key-show</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:200</code> | <code>Show { name: String },</code> |
| <code>interface.8.cli-cli-layerx-key-show-name</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:200</code> | <code>Show { name: String },</code> |
| <code>interface.8.cli-cli-layerx-mcp</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:150</code> | <code>Mcp(McpCommand),</code> |
| <code>interface.8.cli-cli-layerx-mcp-serve</code> | <code>cli_command</code> | <code>change</code> | <code>tools/platform/cli/src/main.rs:529</code> | <code>Serve {</code> |
| <code>interface.8.cli-cli-layerx-mcp-serve-daemon-binding</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:531</code> | <code>daemon_binding: PathBuf,</code> |
| <code>interface.8.cli-cli-layerx-mcp-serve-read-only</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:533</code> | <code>read_only: bool,</code> |
| <code>interface.8.cli-cli-layerx-new</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:114</code> | <code>New(NewArgs),</code> |
| <code>interface.8.cli-cli-layerx-new-directory</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:160</code> | <code>directory: PathBuf,</code> |
| <code>interface.8.cli-cli-layerx-new-name</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:158</code> | <code>name: String,</code> |
| <code>interface.8.cli-cli-layerx-payment</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:135</code> | <code>Payment(PaymentCommand),</code> |
| <code>interface.8.cli-cli-layerx-payment-test</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:251</code> | <code>Test {</code> |
| <code>interface.8.cli-cli-layerx-payment-test-amount</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:259</code> | <code>amount: String,</code> |
| <code>interface.8.cli-cli-layerx-payment-test-currency</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:257</code> | <code>currency: String,</code> |
| <code>interface.8.cli-cli-layerx-payment-test-from</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:253</code> | <code>from: String,</code> |
| <code>interface.8.cli-cli-layerx-payment-test-idempotency-key</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:261</code> | <code>idempotency_key: String,</code> |
| <code>interface.8.cli-cli-layerx-payment-test-to</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:255</code> | <code>to: String,</code> |
| <code>interface.8.cli-cli-layerx-program</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:141</code> | <code>Program(ProgramCommand),</code> |
| <code>interface.8.cli-cli-layerx-program-bindings</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:304</code> | <code>Bindings {</code> |
| <code>interface.8.cli-cli-layerx-program-bindings-code-hash</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:313</code> | <code>code_hash: String,</code> |
| <code>interface.8.cli-cli-layerx-program-bindings-deployment-proof</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:316</code> | <code>deployment_proof: PathBuf,</code> |
| <code>interface.8.cli-cli-layerx-program-bindings-digest</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:310</code> | <code>digest: String,</code> |
| <code>interface.8.cli-cli-layerx-program-bindings-historical</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:322</code> | <code>historical: bool,</code> |
| <code>interface.8.cli-cli-layerx-program-bindings-interface</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:307</code> | <code>interface: PathBuf,</code> |
| <code>interface.8.cli-cli-layerx-program-bindings-output</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:325</code> | <code>output: PathBuf,</code> |
| <code>interface.8.cli-cli-layerx-program-bindings-trust-history</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:319</code> | <code>trust_history: PathBuf,</code> |
| <code>interface.8.cli-cli-layerx-program-build</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:297</code> | <code>Build {</code> |
| <code>interface.8.cli-cli-layerx-program-build-artifact</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:301</code> | <code>artifact: Option&lt;PathBuf&gt;,</code> |
| <code>interface.8.cli-cli-layerx-program-build-manifest-path</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:299</code> | <code>manifest_path: PathBuf,</code> |
| <code>interface.8.cli-cli-layerx-program-call</code> | <code>cli_command</code> | <code>change</code> | <code>tools/platform/cli/src/main.rs:355</code> | <code>Call(ProgramExecutionArgs),</code> |
| <code>interface.8.cli-cli-layerx-program-call-abi-version</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/programs.rs:1013</code> | <code>pub abi_version: u16,</code> |
| <code>interface.8.cli-cli-layerx-program-call-access-declaration</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/programs.rs:1024</code> | <code>pub access_declaration: Option&lt;String&gt;,</code> |
| <code>interface.8.cli-cli-layerx-program-call-account-sequence</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:1227</code> | <code>account_sequence: u64,</code> |
| <code>interface.8.cli-cli-layerx-program-call-calldata</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:1215</code> | <code>calldata: Option&lt;String&gt;,</code> |
| <code>interface.8.cli-cli-layerx-program-call-capability</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:1221</code> | <code>capabilities: Vec&lt;String&gt;,</code> |
| <code>interface.8.cli-cli-layerx-program-call-entrypoint</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/programs.rs:1019</code> | <code>pub entrypoint: String,</code> |
| <code>interface.8.cli-cli-layerx-program-call-expires-at-ms</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:1231</code> | <code>expires_at_ms: u64,</code> |
| <code>interface.8.cli-cli-layerx-program-call-fee-limit</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:1219</code> | <code>fee_limit: String,</code> |
| <code>interface.8.cli-cli-layerx-program-call-fuel</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:1217</code> | <code>fuel: u64,</code> |
| <code>interface.8.cli-cli-layerx-program-call-idempotency-key</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:1223</code> | <code>idempotency_key: String,</code> |
| <code>interface.8.cli-cli-layerx-program-call-key</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:1225</code> | <code>key: Option&lt;String&gt;,</code> |
| <code>interface.8.cli-cli-layerx-program-call-memory-bytes</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/programs.rs:1028</code> | <code>pub memory_bytes: u64,</code> |
| <code>interface.8.cli-cli-layerx-program-call-not-before-ms</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:1229</code> | <code>not_before_ms: u64,</code> |
| <code>interface.8.cli-cli-layerx-program-call-output-bytes</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/programs.rs:1036</code> | <code>pub output_bytes: u64,</code> |
| <code>interface.8.cli-cli-layerx-program-call-output-values</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/programs.rs:1034</code> | <code>pub output_values: u64,</code> |
| <code>interface.8.cli-cli-layerx-program-call-program-id</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:1211</code> | <code>program_id: String,</code> |
| <code>interface.8.cli-cli-layerx-program-call-response-capacity</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/programs.rs:1026</code> | <code>pub response_capacity: u32,</code> |
| <code>interface.8.cli-cli-layerx-program-call-storage-read-bytes</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/programs.rs:1030</code> | <code>pub storage_read_bytes: u64,</code> |
| <code>interface.8.cli-cli-layerx-program-call-storage-write-bytes</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/programs.rs:1032</code> | <code>pub storage_write_bytes: u64,</code> |
| <code>interface.8.cli-cli-layerx-program-call-table-elements</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/programs.rs:1038</code> | <code>pub table_elements: u64,</code> |
| <code>interface.8.cli-cli-layerx-program-deploy</code> | <code>cli_command</code> | <code>change</code> | <code>tools/platform/cli/src/main.rs:328</code> | <code>Deploy {</code> |
| <code>interface.8.cli-cli-layerx-program-deploy-account-sequence</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:372</code> | <code>account_sequence: u64,</code> |
| <code>interface.8.cli-cli-layerx-program-deploy-artifact</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:329</code> | <code>artifact: PathBuf,</code> |
| <code>interface.8.cli-cli-layerx-program-deploy-expires-at-ms</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:376</code> | <code>expires_at_ms: u64,</code> |
| <code>interface.8.cli-cli-layerx-program-deploy-fee-limit</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:378</code> | <code>fee_limit: String,</code> |
| <code>interface.8.cli-cli-layerx-program-deploy-idempotency-key</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:368</code> | <code>idempotency_key: String,</code> |
| <code>interface.8.cli-cli-layerx-program-deploy-interface</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:333</code> | <code>interface: Option&lt;PathBuf&gt;,</code> |
| <code>interface.8.cli-cli-layerx-program-deploy-key</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:370</code> | <code>key: Option&lt;String&gt;,</code> |
| <code>interface.8.cli-cli-layerx-program-deploy-not-before-ms</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:374</code> | <code>not_before_ms: u64,</code> |
| <code>interface.8.cli-cli-layerx-program-deploy-previous-state-root</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:380</code> | <code>previous_state_root: String,</code> |
| <code>interface.8.cli-cli-layerx-program-deploy-program-id</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:366</code> | <code>program_id: String,</code> |
| <code>interface.8.cli-cli-layerx-program-deploy-upgrade-authority</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:331</code> | <code>upgrade_authority: Option&lt;String&gt;,</code> |
| <code>interface.8.cli-cli-layerx-program-discover</code> | <code>cli_command</code> | <code>change</code> | <code>tools/platform/cli/src/main.rs:292</code> | <code>Discover { program_id: String },</code> |
| <code>interface.8.cli-cli-layerx-program-discover-program-id</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:292</code> | <code>Discover { program_id: String },</code> |
| <code>interface.8.cli-cli-layerx-program-interface</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:295</code> | <code>Interface(ProgramInterfaceCommand),</code> |
| <code>interface.8.cli-cli-layerx-program-interface-get</code> | <code>cli_command</code> | <code>change</code> | <code>tools/platform/cli/src/main.rs:419</code> | <code>Get {</code> |
| <code>interface.8.cli-cli-layerx-program-interface-get-program-id</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:420</code> | <code>program_id: String,</code> |
| <code>interface.8.cli-cli-layerx-program-interface-publish</code> | <code>cli_command</code> | <code>change</code> | <code>tools/platform/cli/src/main.rs:422</code> | <code>Publish {</code> |
| <code>interface.8.cli-cli-layerx-program-interface-publish-idempotency-key</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:427</code> | <code>idempotency_key: String,</code> |
| <code>interface.8.cli-cli-layerx-program-interface-publish-interface</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:425</code> | <code>interface: PathBuf,</code> |
| <code>interface.8.cli-cli-layerx-program-interface-publish-program-id</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:423</code> | <code>program_id: String,</code> |
| <code>interface.8.cli-cli-layerx-program-registry</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:360</code> | <code>Registry(RegistryCommand),</code> |
| <code>interface.8.cli-cli-layerx-program-registry-get</code> | <code>cli_command</code> | <code>change</code> | <code>tools/platform/cli/src/main.rs:436</code> | <code>Get { program_id: String },</code> |
| <code>interface.8.cli-cli-layerx-program-registry-get-program-id</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:436</code> | <code>Get { program_id: String },</code> |
| <code>interface.8.cli-cli-layerx-program-registry-list</code> | <code>cli_command</code> | <code>change</code> | <code>tools/platform/cli/src/main.rs:434</code> | <code>List,</code> |
| <code>interface.8.cli-cli-layerx-program-registry-mirror-source</code> | <code>cli_command</code> | <code>change</code> | <code>tools/platform/cli/src/main.rs:438</code> | <code>MirrorSource {</code> |
| <code>interface.8.cli-cli-layerx-program-registry-mirror-source-archive</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:444</code> | <code>archive: PathBuf,</code> |
| <code>interface.8.cli-cli-layerx-program-registry-mirror-source-plan</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:442</code> | <code>plan: PathBuf,</code> |
| <code>interface.8.cli-cli-layerx-program-registry-mirror-source-source-uri</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:440</code> | <code>source_uri: String,</code> |
| <code>interface.8.cli-cli-layerx-program-registry-verify-source</code> | <code>cli_command</code> | <code>change</code> | <code>tools/platform/cli/src/main.rs:447</code> | <code>VerifySource {</code> |
| <code>interface.8.cli-cli-layerx-program-registry-verify-source-idempotency-key</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:454</code> | <code>idempotency_key: String,</code> |
| <code>interface.8.cli-cli-layerx-program-registry-verify-source-program-id</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:448</code> | <code>program_id: String,</code> |
| <code>interface.8.cli-cli-layerx-program-registry-verify-source-source-digest</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:452</code> | <code>source_digest: String,</code> |
| <code>interface.8.cli-cli-layerx-program-registry-verify-source-source-uri</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:450</code> | <code>source_uri: String,</code> |
| <code>interface.8.cli-cli-layerx-program-simulate</code> | <code>cli_command</code> | <code>change</code> | <code>tools/platform/cli/src/main.rs:357</code> | <code>Simulate(ProgramExecutionArgs),</code> |
| <code>interface.8.cli-cli-layerx-program-simulate-abi-version</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/programs.rs:1013</code> | <code>pub abi_version: u16,</code> |
| <code>interface.8.cli-cli-layerx-program-simulate-access-declaration</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/programs.rs:1024</code> | <code>pub access_declaration: Option&lt;String&gt;,</code> |
| <code>interface.8.cli-cli-layerx-program-simulate-account-sequence</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:1227</code> | <code>account_sequence: u64,</code> |
| <code>interface.8.cli-cli-layerx-program-simulate-calldata</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:1215</code> | <code>calldata: Option&lt;String&gt;,</code> |
| <code>interface.8.cli-cli-layerx-program-simulate-capability</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:1221</code> | <code>capabilities: Vec&lt;String&gt;,</code> |
| <code>interface.8.cli-cli-layerx-program-simulate-entrypoint</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/programs.rs:1019</code> | <code>pub entrypoint: String,</code> |
| <code>interface.8.cli-cli-layerx-program-simulate-expires-at-ms</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:1231</code> | <code>expires_at_ms: u64,</code> |
| <code>interface.8.cli-cli-layerx-program-simulate-fee-limit</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:1219</code> | <code>fee_limit: String,</code> |
| <code>interface.8.cli-cli-layerx-program-simulate-fuel</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:1217</code> | <code>fuel: u64,</code> |
| <code>interface.8.cli-cli-layerx-program-simulate-idempotency-key</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:1223</code> | <code>idempotency_key: String,</code> |
| <code>interface.8.cli-cli-layerx-program-simulate-key</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:1225</code> | <code>key: Option&lt;String&gt;,</code> |
| <code>interface.8.cli-cli-layerx-program-simulate-memory-bytes</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/programs.rs:1028</code> | <code>pub memory_bytes: u64,</code> |
| <code>interface.8.cli-cli-layerx-program-simulate-not-before-ms</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:1229</code> | <code>not_before_ms: u64,</code> |
| <code>interface.8.cli-cli-layerx-program-simulate-output-bytes</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/programs.rs:1036</code> | <code>pub output_bytes: u64,</code> |
| <code>interface.8.cli-cli-layerx-program-simulate-output-values</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/programs.rs:1034</code> | <code>pub output_values: u64,</code> |
| <code>interface.8.cli-cli-layerx-program-simulate-program-id</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:1211</code> | <code>program_id: String,</code> |
| <code>interface.8.cli-cli-layerx-program-simulate-response-capacity</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/programs.rs:1026</code> | <code>pub response_capacity: u32,</code> |
| <code>interface.8.cli-cli-layerx-program-simulate-storage-read-bytes</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/programs.rs:1030</code> | <code>pub storage_read_bytes: u64,</code> |
| <code>interface.8.cli-cli-layerx-program-simulate-storage-write-bytes</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/programs.rs:1032</code> | <code>pub storage_write_bytes: u64,</code> |
| <code>interface.8.cli-cli-layerx-program-simulate-table-elements</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/programs.rs:1038</code> | <code>pub table_elements: u64,</code> |
| <code>interface.8.cli-cli-layerx-program-upgrade</code> | <code>cli_command</code> | <code>change</code> | <code>tools/platform/cli/src/main.rs:337</code> | <code>Upgrade {</code> |
| <code>interface.8.cli-cli-layerx-program-upgrade-account-sequence</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:372</code> | <code>account_sequence: u64,</code> |
| <code>interface.8.cli-cli-layerx-program-upgrade-allow-breaking-interface</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:348</code> | <code>allow_breaking_interface: bool,</code> |
| <code>interface.8.cli-cli-layerx-program-upgrade-artifact</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:338</code> | <code>artifact: PathBuf,</code> |
| <code>interface.8.cli-cli-layerx-program-upgrade-clear-interface</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:346</code> | <code>clear_interface: bool,</code> |
| <code>interface.8.cli-cli-layerx-program-upgrade-expires-at-ms</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:376</code> | <code>expires_at_ms: u64,</code> |
| <code>interface.8.cli-cli-layerx-program-upgrade-fee-limit</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:378</code> | <code>fee_limit: String,</code> |
| <code>interface.8.cli-cli-layerx-program-upgrade-idempotency-key</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:368</code> | <code>idempotency_key: String,</code> |
| <code>interface.8.cli-cli-layerx-program-upgrade-interface</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:344</code> | <code>interface: Option&lt;PathBuf&gt;,</code> |
| <code>interface.8.cli-cli-layerx-program-upgrade-key</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:370</code> | <code>key: Option&lt;String&gt;,</code> |
| <code>interface.8.cli-cli-layerx-program-upgrade-migration-hook</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:342</code> | <code>migration_hook: Option&lt;PathBuf&gt;,</code> |
| <code>interface.8.cli-cli-layerx-program-upgrade-not-before-ms</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:374</code> | <code>not_before_ms: u64,</code> |
| <code>interface.8.cli-cli-layerx-program-upgrade-old-hash</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:340</code> | <code>old_hash: String,</code> |
| <code>interface.8.cli-cli-layerx-program-upgrade-previous-state-root</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:380</code> | <code>previous_state_root: String,</code> |
| <code>interface.8.cli-cli-layerx-program-upgrade-program-id</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:366</code> | <code>program_id: String,</code> |
| <code>interface.8.cli-cli-layerx-program-wind-down</code> | <code>cli_command</code> | <code>change</code> | <code>tools/platform/cli/src/main.rs:353</code> | <code>WindDown(ProgramWindDownCommand),</code> |
| <code>interface.8.cli-cli-layerx-program-wind-down-deprecate</code> | <code>cli_command</code> | <code>change</code> | <code>tools/platform/cli/src/main.rs:397</code> | <code>Deprecate {</code> |
| <code>interface.8.cli-cli-layerx-program-wind-down-deprecate-account-sequence</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:372</code> | <code>account_sequence: u64,</code> |
| <code>interface.8.cli-cli-layerx-program-wind-down-deprecate-deadline-batch</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:401</code> | <code>deadline_batch: u64,</code> |
| <code>interface.8.cli-cli-layerx-program-wind-down-deprecate-exit-program</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:399</code> | <code>exit_program: String,</code> |
| <code>interface.8.cli-cli-layerx-program-wind-down-deprecate-expires-at-ms</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:376</code> | <code>expires_at_ms: u64,</code> |
| <code>interface.8.cli-cli-layerx-program-wind-down-deprecate-fee-limit</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:378</code> | <code>fee_limit: String,</code> |
| <code>interface.8.cli-cli-layerx-program-wind-down-deprecate-idempotency-key</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:368</code> | <code>idempotency_key: String,</code> |
| <code>interface.8.cli-cli-layerx-program-wind-down-deprecate-key</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:370</code> | <code>key: Option&lt;String&gt;,</code> |
| <code>interface.8.cli-cli-layerx-program-wind-down-deprecate-not-before-ms</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:374</code> | <code>not_before_ms: u64,</code> |
| <code>interface.8.cli-cli-layerx-program-wind-down-deprecate-previous-state-root</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:380</code> | <code>previous_state_root: String,</code> |
| <code>interface.8.cli-cli-layerx-program-wind-down-deprecate-program-id</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:366</code> | <code>program_id: String,</code> |
| <code>interface.8.cli-cli-layerx-program-wind-down-exit</code> | <code>cli_command</code> | <code>change</code> | <code>tools/platform/cli/src/main.rs:409</code> | <code>Exit {</code> |
| <code>interface.8.cli-cli-layerx-program-wind-down-exit-account</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:411</code> | <code>account: String,</code> |
| <code>interface.8.cli-cli-layerx-program-wind-down-exit-account-sequence</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:372</code> | <code>account_sequence: u64,</code> |
| <code>interface.8.cli-cli-layerx-program-wind-down-exit-expires-at-ms</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:376</code> | <code>expires_at_ms: u64,</code> |
| <code>interface.8.cli-cli-layerx-program-wind-down-exit-fee-limit</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:378</code> | <code>fee_limit: String,</code> |
| <code>interface.8.cli-cli-layerx-program-wind-down-exit-idempotency-key</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:368</code> | <code>idempotency_key: String,</code> |
| <code>interface.8.cli-cli-layerx-program-wind-down-exit-key</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:370</code> | <code>key: Option&lt;String&gt;,</code> |
| <code>interface.8.cli-cli-layerx-program-wind-down-exit-not-before-ms</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:374</code> | <code>not_before_ms: u64,</code> |
| <code>interface.8.cli-cli-layerx-program-wind-down-exit-previous-state-root</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:380</code> | <code>previous_state_root: String,</code> |
| <code>interface.8.cli-cli-layerx-program-wind-down-exit-program-id</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:366</code> | <code>program_id: String,</code> |
| <code>interface.8.cli-cli-layerx-program-wind-down-route</code> | <code>cli_command</code> | <code>change</code> | <code>tools/platform/cli/src/main.rs:385</code> | <code>Route {</code> |
| <code>interface.8.cli-cli-layerx-program-wind-down-route-account</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:387</code> | <code>account: String,</code> |
| <code>interface.8.cli-cli-layerx-program-wind-down-route-account-sequence</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:372</code> | <code>account_sequence: u64,</code> |
| <code>interface.8.cli-cli-layerx-program-wind-down-route-asset</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:389</code> | <code>asset: String,</code> |
| <code>interface.8.cli-cli-layerx-program-wind-down-route-destination</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:391</code> | <code>destination: String,</code> |
| <code>interface.8.cli-cli-layerx-program-wind-down-route-expires-at-ms</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:376</code> | <code>expires_at_ms: u64,</code> |
| <code>interface.8.cli-cli-layerx-program-wind-down-route-fee-limit</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:378</code> | <code>fee_limit: String,</code> |
| <code>interface.8.cli-cli-layerx-program-wind-down-route-idempotency-key</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:368</code> | <code>idempotency_key: String,</code> |
| <code>interface.8.cli-cli-layerx-program-wind-down-route-key</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:370</code> | <code>key: Option&lt;String&gt;,</code> |
| <code>interface.8.cli-cli-layerx-program-wind-down-route-not-before-ms</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:374</code> | <code>not_before_ms: u64,</code> |
| <code>interface.8.cli-cli-layerx-program-wind-down-route-previous-state-root</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:380</code> | <code>previous_state_root: String,</code> |
| <code>interface.8.cli-cli-layerx-program-wind-down-route-program-id</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:366</code> | <code>program_id: String,</code> |
| <code>interface.8.cli-cli-layerx-program-wind-down-route-seed</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:393</code> | <code>seed: String,</code> |
| <code>interface.8.cli-cli-layerx-program-wind-down-tombstone</code> | <code>cli_command</code> | <code>change</code> | <code>tools/platform/cli/src/main.rs:405</code> | <code>Tombstone {</code> |
| <code>interface.8.cli-cli-layerx-program-wind-down-tombstone-account-sequence</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:372</code> | <code>account_sequence: u64,</code> |
| <code>interface.8.cli-cli-layerx-program-wind-down-tombstone-expires-at-ms</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:376</code> | <code>expires_at_ms: u64,</code> |
| <code>interface.8.cli-cli-layerx-program-wind-down-tombstone-fee-limit</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:378</code> | <code>fee_limit: String,</code> |
| <code>interface.8.cli-cli-layerx-program-wind-down-tombstone-idempotency-key</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:368</code> | <code>idempotency_key: String,</code> |
| <code>interface.8.cli-cli-layerx-program-wind-down-tombstone-key</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:370</code> | <code>key: Option&lt;String&gt;,</code> |
| <code>interface.8.cli-cli-layerx-program-wind-down-tombstone-not-before-ms</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:374</code> | <code>not_before_ms: u64,</code> |
| <code>interface.8.cli-cli-layerx-program-wind-down-tombstone-previous-state-root</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:380</code> | <code>previous_state_root: String,</code> |
| <code>interface.8.cli-cli-layerx-program-wind-down-tombstone-program-id</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:366</code> | <code>program_id: String,</code> |
| <code>interface.8.cli-cli-layerx-receipt</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:138</code> | <code>Receipt(ReceiptCommand),</code> |
| <code>interface.8.cli-cli-layerx-receipt-get</code> | <code>cli_command</code> | <code>change</code> | <code>tools/platform/cli/src/main.rs:268</code> | <code>Get { id: String },</code> |
| <code>interface.8.cli-cli-layerx-receipt-get-id</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:268</code> | <code>Get { id: String },</code> |
| <code>interface.8.cli-cli-layerx-receipt-verify</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:270</code> | <code>Verify(VerifyReceiptArgs),</code> |
| <code>interface.8.cli-cli-layerx-receipt-verify-asset</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:280</code> | <code>asset: String,</code> |
| <code>interface.8.cli-cli-layerx-receipt-verify-batch-id</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:278</code> | <code>batch_id: String,</code> |
| <code>interface.8.cli-cli-layerx-receipt-verify-previous-state-root</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:282</code> | <code>previous_state_root: String,</code> |
| <code>interface.8.cli-cli-layerx-receipt-verify-receipt</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:276</code> | <code>receipt: PathBuf,</code> |
| <code>interface.8.cli-cli-layerx-receipt-verify-resulting-state-root</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:284</code> | <code>resulting_state_root: String,</code> |
| <code>interface.8.cli-cli-layerx-receipt-verify-sequencer-public-key</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:286</code> | <code>sequencer_public_key: String,</code> |
| <code>interface.8.cli-cli-layerx-register</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:130</code> | <code>Register(register::RegisterArgs),</code> |
| <code>interface.8.cli-cli-layerx-register-key</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/register.rs:20</code> | <code>key: Option&lt;String&gt;,</code> |
| <code>interface.8.cli-cli-layerx-rpc</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:98</code> | <code>rpc: Option&lt;String&gt;,</code> |
| <code>interface.8.cli-cli-layerx-token</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:112</code> | <code>Token(wallet::TokenCommand),</code> |
| <code>interface.8.cli-cli-layerx-token-burn</code> | <code>cli_command</code> | <code>change</code> | <code>tools/platform/cli/src/wallet.rs:161</code> | <code>Burn {</code> |
| <code>interface.8.cli-cli-layerx-token-burn-amount</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet.rs:165</code> | <code>amount: String,</code> |
| <code>interface.8.cli-cli-layerx-token-burn-asset</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet.rs:163</code> | <code>asset: String,</code> |
| <code>interface.8.cli-cli-layerx-token-burn-fee-limit</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet.rs:121</code> | <code>fee_limit: String,</code> |
| <code>interface.8.cli-cli-layerx-token-burn-key</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet.rs:167</code> | <code>key: Option&lt;String&gt;,</code> |
| <code>interface.8.cli-cli-layerx-token-burn-receipt-policy</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet.rs:119</code> | <code>receipt_policy: std::path::PathBuf,</code> |
| <code>interface.8.cli-cli-layerx-token-burn-timeout-seconds</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet.rs:125</code> | <code>timeout_seconds: u64,</code> |
| <code>interface.8.cli-cli-layerx-token-burn-wait</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet.rs:123</code> | <code>wait: Commitment,</code> |
| <code>interface.8.cli-cli-layerx-token-create</code> | <code>cli_command</code> | <code>change</code> | <code>tools/platform/cli/src/wallet.rs:131</code> | <code>Create {</code> |
| <code>interface.8.cli-cli-layerx-token-create-decimals</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet.rs:137</code> | <code>decimals: u8,</code> |
| <code>interface.8.cli-cli-layerx-token-create-fee-limit</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet.rs:121</code> | <code>fee_limit: String,</code> |
| <code>interface.8.cli-cli-layerx-token-create-key</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet.rs:143</code> | <code>key: Option&lt;String&gt;,</code> |
| <code>interface.8.cli-cli-layerx-token-create-name</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet.rs:135</code> | <code>name: String,</code> |
| <code>interface.8.cli-cli-layerx-token-create-receipt-policy</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet.rs:119</code> | <code>receipt_policy: std::path::PathBuf,</code> |
| <code>interface.8.cli-cli-layerx-token-create-salt</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet.rs:141</code> | <code>salt: String,</code> |
| <code>interface.8.cli-cli-layerx-token-create-supply-cap</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet.rs:139</code> | <code>supply_cap: String,</code> |
| <code>interface.8.cli-cli-layerx-token-create-symbol</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet.rs:133</code> | <code>symbol: String,</code> |
| <code>interface.8.cli-cli-layerx-token-create-timeout-seconds</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet.rs:125</code> | <code>timeout_seconds: u64,</code> |
| <code>interface.8.cli-cli-layerx-token-create-wait</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet.rs:123</code> | <code>wait: Commitment,</code> |
| <code>interface.8.cli-cli-layerx-token-info</code> | <code>cli_command</code> | <code>change</code> | <code>tools/platform/cli/src/wallet.rs:174</code> | <code>Info { asset: String },</code> |
| <code>interface.8.cli-cli-layerx-token-info-asset</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet.rs:174</code> | <code>Info { asset: String },</code> |
| <code>interface.8.cli-cli-layerx-token-list</code> | <code>cli_command</code> | <code>change</code> | <code>tools/platform/cli/src/wallet.rs:176</code> | <code>List {</code> |
| <code>interface.8.cli-cli-layerx-token-list-cursor</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet.rs:179</code> | <code>cursor: Option&lt;String&gt;,</code> |
| <code>interface.8.cli-cli-layerx-token-list-limit</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet.rs:182</code> | <code>limit: Option&lt;u16&gt;,</code> |
| <code>interface.8.cli-cli-layerx-token-mint</code> | <code>cli_command</code> | <code>change</code> | <code>tools/platform/cli/src/wallet.rs:148</code> | <code>Mint {</code> |
| <code>interface.8.cli-cli-layerx-token-mint-amount</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet.rs:154</code> | <code>amount: String,</code> |
| <code>interface.8.cli-cli-layerx-token-mint-asset</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet.rs:150</code> | <code>asset: String,</code> |
| <code>interface.8.cli-cli-layerx-token-mint-fee-limit</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet.rs:121</code> | <code>fee_limit: String,</code> |
| <code>interface.8.cli-cli-layerx-token-mint-key</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet.rs:156</code> | <code>key: Option&lt;String&gt;,</code> |
| <code>interface.8.cli-cli-layerx-token-mint-receipt-policy</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet.rs:119</code> | <code>receipt_policy: std::path::PathBuf,</code> |
| <code>interface.8.cli-cli-layerx-token-mint-timeout-seconds</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet.rs:125</code> | <code>timeout_seconds: u64,</code> |
| <code>interface.8.cli-cli-layerx-token-mint-to</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet.rs:152</code> | <code>to: String,</code> |
| <code>interface.8.cli-cli-layerx-token-mint-wait</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet.rs:123</code> | <code>wait: Commitment,</code> |
| <code>interface.8.cli-cli-layerx-token-transfer</code> | <code>cli_command</code> | <code>change</code> | <code>tools/platform/cli/src/wallet.rs:172</code> | <code>Transfer(TransferArgs),</code> |
| <code>interface.8.cli-cli-layerx-token-transfer-amount</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet.rs:97</code> | <code>amount: String,</code> |
| <code>interface.8.cli-cli-layerx-token-transfer-asset</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet.rs:95</code> | <code>asset: String,</code> |
| <code>interface.8.cli-cli-layerx-token-transfer-fee-limit</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet.rs:109</code> | <code>fee_limit: Option&lt;String&gt;,</code> |
| <code>interface.8.cli-cli-layerx-token-transfer-key</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet.rs:99</code> | <code>key: Option&lt;String&gt;,</code> |
| <code>interface.8.cli-cli-layerx-token-transfer-receipt-policy</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet.rs:107</code> | <code>receipt_policy: Option&lt;std::path::PathBuf&gt;,</code> |
| <code>interface.8.cli-cli-layerx-token-transfer-timeout-seconds</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet.rs:113</code> | <code>timeout_seconds: u64,</code> |
| <code>interface.8.cli-cli-layerx-token-transfer-to</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet.rs:93</code> | <code>to: String,</code> |
| <code>interface.8.cli-cli-layerx-token-transfer-wait</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet.rs:111</code> | <code>wait: Commitment,</code> |
| <code>interface.8.cli-cli-layerx-wallet</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:109</code> | <code>Wallet(wallet::WalletCommand),</code> |
| <code>interface.8.cli-cli-layerx-wallet-balance</code> | <code>cli_command</code> | <code>change</code> | <code>tools/platform/cli/src/wallet.rs:32</code> | <code>Balance {</code> |
| <code>interface.8.cli-cli-layerx-wallet-balance-asset</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet.rs:36</code> | <code>asset: Option&lt;String&gt;,</code> |
| <code>interface.8.cli-cli-layerx-wallet-balance-did</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet.rs:34</code> | <code>did: Option&lt;String&gt;,</code> |
| <code>interface.8.cli-cli-layerx-wallet-create</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet.rs:15</code> | <code>Create {</code> |
| <code>interface.8.cli-cli-layerx-wallet-create-did</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet.rs:18</code> | <code>did: Option&lt;String&gt;,</code> |
| <code>interface.8.cli-cli-layerx-wallet-create-name</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet.rs:16</code> | <code>name: String,</code> |
| <code>interface.8.cli-cli-layerx-wallet-derive</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet.rs:28</code> | <code>Derive(crate::wallet_derive::DeriveArgs),</code> |
| <code>interface.8.cli-cli-layerx-wallet-derive-bind</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet_derive.rs:42</code> | <code>bind: bool,</code> |
| <code>interface.8.cli-cli-layerx-wallet-derive-export-private-keys</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet_derive.rs:39</code> | <code>export_private_keys: Option&lt;PathBuf&gt;,</code> |
| <code>interface.8.cli-cli-layerx-wallet-derive-index</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet_derive.rs:36</code> | <code>index: u32,</code> |
| <code>interface.8.cli-cli-layerx-wallet-derive-mnemonic-file</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet_derive.rs:30</code> | <code>mnemonic_file: Option&lt;PathBuf&gt;,</code> |
| <code>interface.8.cli-cli-layerx-wallet-derive-passphrase-file</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet_derive.rs:33</code> | <code>passphrase_file: Option&lt;PathBuf&gt;,</code> |
| <code>interface.8.cli-cli-layerx-wallet-estimate-fee</code> | <code>cli_command</code> | <code>change</code> | <code>tools/platform/cli/src/wallet.rs:56</code> | <code>EstimateFee { canonical_hex: String },</code> |
| <code>interface.8.cli-cli-layerx-wallet-estimate-fee-canonical-hex</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet.rs:56</code> | <code>EstimateFee { canonical_hex: String },</code> |
| <code>interface.8.cli-cli-layerx-wallet-history</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet.rs:42</code> | <code>History {</code> |
| <code>interface.8.cli-cli-layerx-wallet-history-did</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet.rs:44</code> | <code>did: Option&lt;String&gt;,</code> |
| <code>interface.8.cli-cli-layerx-wallet-import</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet.rs:21</code> | <code>Import {</code> |
| <code>interface.8.cli-cli-layerx-wallet-import-did</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet.rs:24</code> | <code>did: Option&lt;String&gt;,</code> |
| <code>interface.8.cli-cli-layerx-wallet-import-name</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet.rs:22</code> | <code>name: String,</code> |
| <code>interface.8.cli-cli-layerx-wallet-list</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet.rs:30</code> | <code>List,</code> |
| <code>interface.8.cli-cli-layerx-wallet-open-account</code> | <code>cli_command</code> | <code>change</code> | <code>tools/platform/cli/src/wallet.rs:66</code> | <code>OpenAccount {</code> |
| <code>interface.8.cli-cli-layerx-wallet-open-account-asset</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet.rs:68</code> | <code>asset: String,</code> |
| <code>interface.8.cli-cli-layerx-wallet-open-account-fee-limit</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet.rs:121</code> | <code>fee_limit: String,</code> |
| <code>interface.8.cli-cli-layerx-wallet-open-account-key</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet.rs:70</code> | <code>key: Option&lt;String&gt;,</code> |
| <code>interface.8.cli-cli-layerx-wallet-open-account-receipt-policy</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet.rs:119</code> | <code>receipt_policy: std::path::PathBuf,</code> |
| <code>interface.8.cli-cli-layerx-wallet-open-account-timeout-seconds</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet.rs:125</code> | <code>timeout_seconds: u64,</code> |
| <code>interface.8.cli-cli-layerx-wallet-open-account-wait</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet.rs:123</code> | <code>wait: Commitment,</code> |
| <code>interface.8.cli-cli-layerx-wallet-receipt</code> | <code>cli_command</code> | <code>change</code> | <code>tools/platform/cli/src/wallet.rs:58</code> | <code>Receipt {</code> |
| <code>interface.8.cli-cli-layerx-wallet-receipt-activity-id</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet.rs:59</code> | <code>activity_id: String,</code> |
| <code>interface.8.cli-cli-layerx-wallet-receipt-receipt-policy</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet.rs:63</code> | <code>receipt_policy: Option&lt;std::path::PathBuf&gt;,</code> |
| <code>interface.8.cli-cli-layerx-wallet-receipt-wait</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet.rs:61</code> | <code>wait: Commitment,</code> |
| <code>interface.8.cli-cli-layerx-wallet-send</code> | <code>cli_command</code> | <code>change</code> | <code>tools/platform/cli/src/wallet.rs:39</code> | <code>Send(TransferArgs),</code> |
| <code>interface.8.cli-cli-layerx-wallet-send-amount</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet.rs:97</code> | <code>amount: String,</code> |
| <code>interface.8.cli-cli-layerx-wallet-send-asset</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet.rs:95</code> | <code>asset: String,</code> |
| <code>interface.8.cli-cli-layerx-wallet-send-fee-limit</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet.rs:109</code> | <code>fee_limit: Option&lt;String&gt;,</code> |
| <code>interface.8.cli-cli-layerx-wallet-send-key</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet.rs:99</code> | <code>key: Option&lt;String&gt;,</code> |
| <code>interface.8.cli-cli-layerx-wallet-send-receipt-policy</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet.rs:107</code> | <code>receipt_policy: Option&lt;std::path::PathBuf&gt;,</code> |
| <code>interface.8.cli-cli-layerx-wallet-send-timeout-seconds</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet.rs:113</code> | <code>timeout_seconds: u64,</code> |
| <code>interface.8.cli-cli-layerx-wallet-send-to</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet.rs:93</code> | <code>to: String,</code> |
| <code>interface.8.cli-cli-layerx-wallet-send-wait</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet.rs:111</code> | <code>wait: Commitment,</code> |
| <code>interface.8.cli-cli-layerx-wallet-watch</code> | <code>cli_command</code> | <code>change</code> | <code>tools/platform/cli/src/wallet.rs:47</code> | <code>Watch {</code> |
| <code>interface.8.cli-cli-layerx-wallet-watch-account-id</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet.rs:51</code> | <code>account_id: Option&lt;String&gt;,</code> |
| <code>interface.8.cli-cli-layerx-wallet-watch-timeout-seconds</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet.rs:53</code> | <code>timeout_seconds: u64,</code> |
| <code>interface.8.cli-cli-layerx-wallet-watch-topic</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet.rs:49</code> | <code>topic: SubscriptionTopic,</code> |
| <code>interface.8.cli-cli-layerx-workspace</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/main.rs:116</code> | <code>Workspace(workspace::WorkspaceArgs),</code> |
| <code>interface.8.cli-cli-layerx-workspace-all</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/workspace.rs:33</code> | <code>All(RunArgs),</code> |
| <code>interface.8.cli-cli-layerx-workspace-all-all</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/workspace.rs:43</code> | <code>all: bool,</code> |
| <code>interface.8.cli-cli-layerx-workspace-all-allow-production</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/workspace.rs:68</code> | <code>allow_production: bool,</code> |
| <code>interface.8.cli-cli-layerx-workspace-all-ci</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/workspace.rs:65</code> | <code>ci: bool,</code> |
| <code>interface.8.cli-cli-layerx-workspace-all-dry-run</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/workspace.rs:56</code> | <code>dry_run: bool,</code> |
| <code>interface.8.cli-cli-layerx-workspace-all-environment</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/workspace.rs:46</code> | <code>environment: Option&lt;String&gt;,</code> |
| <code>interface.8.cli-cli-layerx-workspace-all-fail-fast</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/workspace.rs:59</code> | <code>fail_fast: bool,</code> |
| <code>interface.8.cli-cli-layerx-workspace-all-module</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/workspace.rs:40</code> | <code>modules: Vec&lt;String&gt;,</code> |
| <code>interface.8.cli-cli-layerx-workspace-all-yes</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/workspace.rs:62</code> | <code>yes: bool,</code> |
| <code>interface.8.cli-cli-layerx-workspace-build</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/workspace.rs:29</code> | <code>Build(RunArgs),</code> |
| <code>interface.8.cli-cli-layerx-workspace-build-all</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/workspace.rs:43</code> | <code>all: bool,</code> |
| <code>interface.8.cli-cli-layerx-workspace-build-allow-production</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/workspace.rs:68</code> | <code>allow_production: bool,</code> |
| <code>interface.8.cli-cli-layerx-workspace-build-ci</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/workspace.rs:65</code> | <code>ci: bool,</code> |
| <code>interface.8.cli-cli-layerx-workspace-build-dry-run</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/workspace.rs:56</code> | <code>dry_run: bool,</code> |
| <code>interface.8.cli-cli-layerx-workspace-build-environment</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/workspace.rs:46</code> | <code>environment: Option&lt;String&gt;,</code> |
| <code>interface.8.cli-cli-layerx-workspace-build-fail-fast</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/workspace.rs:59</code> | <code>fail_fast: bool,</code> |
| <code>interface.8.cli-cli-layerx-workspace-build-module</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/workspace.rs:40</code> | <code>modules: Vec&lt;String&gt;,</code> |
| <code>interface.8.cli-cli-layerx-workspace-build-yes</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/workspace.rs:62</code> | <code>yes: bool,</code> |
| <code>interface.8.cli-cli-layerx-workspace-doctor</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/workspace.rs:25</code> | <code>Doctor(SelectionArgs),</code> |
| <code>interface.8.cli-cli-layerx-workspace-doctor-all</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/workspace.rs:43</code> | <code>all: bool,</code> |
| <code>interface.8.cli-cli-layerx-workspace-doctor-environment</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/workspace.rs:46</code> | <code>environment: Option&lt;String&gt;,</code> |
| <code>interface.8.cli-cli-layerx-workspace-doctor-module</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/workspace.rs:40</code> | <code>modules: Vec&lt;String&gt;,</code> |
| <code>interface.8.cli-cli-layerx-workspace-install</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/workspace.rs:27</code> | <code>Install(RunArgs),</code> |
| <code>interface.8.cli-cli-layerx-workspace-install-all</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/workspace.rs:43</code> | <code>all: bool,</code> |
| <code>interface.8.cli-cli-layerx-workspace-install-allow-production</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/workspace.rs:68</code> | <code>allow_production: bool,</code> |
| <code>interface.8.cli-cli-layerx-workspace-install-ci</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/workspace.rs:65</code> | <code>ci: bool,</code> |
| <code>interface.8.cli-cli-layerx-workspace-install-dry-run</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/workspace.rs:56</code> | <code>dry_run: bool,</code> |
| <code>interface.8.cli-cli-layerx-workspace-install-environment</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/workspace.rs:46</code> | <code>environment: Option&lt;String&gt;,</code> |
| <code>interface.8.cli-cli-layerx-workspace-install-fail-fast</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/workspace.rs:59</code> | <code>fail_fast: bool,</code> |
| <code>interface.8.cli-cli-layerx-workspace-install-module</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/workspace.rs:40</code> | <code>modules: Vec&lt;String&gt;,</code> |
| <code>interface.8.cli-cli-layerx-workspace-install-yes</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/workspace.rs:62</code> | <code>yes: bool,</code> |
| <code>interface.8.cli-cli-layerx-workspace-modules</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/workspace.rs:23</code> | <code>Modules,</code> |
| <code>interface.8.cli-cli-layerx-workspace-test</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/workspace.rs:31</code> | <code>Test(RunArgs),</code> |
| <code>interface.8.cli-cli-layerx-workspace-test-all</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/workspace.rs:43</code> | <code>all: bool,</code> |
| <code>interface.8.cli-cli-layerx-workspace-test-allow-production</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/workspace.rs:68</code> | <code>allow_production: bool,</code> |
| <code>interface.8.cli-cli-layerx-workspace-test-ci</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/workspace.rs:65</code> | <code>ci: bool,</code> |
| <code>interface.8.cli-cli-layerx-workspace-test-dry-run</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/workspace.rs:56</code> | <code>dry_run: bool,</code> |
| <code>interface.8.cli-cli-layerx-workspace-test-environment</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/workspace.rs:46</code> | <code>environment: Option&lt;String&gt;,</code> |
| <code>interface.8.cli-cli-layerx-workspace-test-fail-fast</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/workspace.rs:59</code> | <code>fail_fast: bool,</code> |
| <code>interface.8.cli-cli-layerx-workspace-test-module</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/workspace.rs:40</code> | <code>modules: Vec&lt;String&gt;,</code> |
| <code>interface.8.cli-cli-layerx-workspace-test-yes</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/cli/src/workspace.rs:62</code> | <code>yes: bool,</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.cli-cli-layerx</code> | notes | 30743 callers total; no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-a2a</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-a2a-serve</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-a2a-serve-asset</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-a2a-serve-authorization-file</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-a2a-serve-environment</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-a2a-serve-gateway-credential</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-a2a-serve-key</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-a2a-serve-listen</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-a2a-serve-read-only</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-a2a-serve-source-account</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-account-create</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-account-create-display-name</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-account-create-email</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-account-create-idempotency-key</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-account-create-initial-amount</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-account-create-key</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-auth-delete-environment</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-environment-use-sequencer-trust-anchor</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-environment-use-sequencer-trust-anchor-file</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-faucet</code> | notes | tools/platform/cli/src/faucet.rs:18 sends lx_requestFunds; the design replaces it with px_requestFunds (spec/layerx-merge/spec.kvx:20889); no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-faucet-key</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-gateway-credential</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-install-a2a-asset</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-install-a2a-key</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-install-a2a-listen</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-install-a2a-read-only</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-install-a2a-rotate</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-install-a2a-source-account</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-install-a2a-token-stdin</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-install-a2a-well-known</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-install-mcp-daemon-binding</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-install-mcp-read-only</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-key-create-did</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-key-import-did</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-mcp-serve</code> | notes | tools/platform/cli/src/toolset.rs:363 toolset invoke posts /v1/activities; stale task 10.5.29 do_3 moves it to eth_sendRawTransaction (spec/layerx-merge/spec.kvx:53202) |
| <code>interface.8.cli-cli-layerx-mcp-serve-read-only</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-new</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-new-directory</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-new-name</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-payment</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-payment-test</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-payment-test-amount</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-payment-test-currency</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-payment-test-from</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-payment-test-idempotency-key</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-payment-test-to</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-bindings</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-bindings-code-hash</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-bindings-deployment-proof</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-bindings-digest</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-bindings-historical</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-bindings-interface</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-bindings-output</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-bindings-trust-history</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-build-artifact</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-call</code> | notes | tools/platform/cli/src/programs.rs:1100 uses the gateway /v1/programs routes that stale task 4.7.5 removes; the design submits 0x101D transactions and reads 0x101D views (spec/layerx-merge/spec.kvx:53202) |
| <code>interface.8.cli-cli-layerx-program-call-abi-version</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-call-access-declaration</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-call-account-sequence</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-call-calldata</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-call-capability</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-call-entrypoint</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-call-expires-at-ms</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-call-fee-limit</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-call-fuel</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-call-idempotency-key</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-call-key</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-call-memory-bytes</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-call-not-before-ms</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-call-output-bytes</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-call-output-values</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-call-response-capacity</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-call-storage-read-bytes</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-call-storage-write-bytes</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-call-table-elements</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-deploy</code> | notes | tools/platform/cli/src/programs.rs:520 uses the gateway /v1/programs routes that stale task 4.7.5 removes; the design submits 0x101D transactions and reads 0x101D views (spec/layerx-merge/spec.kvx:53202); no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-deploy-account-sequence</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-deploy-artifact</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-deploy-expires-at-ms</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-deploy-fee-limit</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-deploy-idempotency-key</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-deploy-interface</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-deploy-key</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-deploy-not-before-ms</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-deploy-previous-state-root</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-deploy-program-id</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-deploy-upgrade-authority</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-discover</code> | notes | tools/platform/cli/src/programs.rs:1181 uses the gateway /v1/programs routes that stale task 4.7.5 removes; the design submits 0x101D transactions and reads 0x101D views (spec/layerx-merge/spec.kvx:53202) |
| <code>interface.8.cli-cli-layerx-program-interface</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-interface-get</code> | notes | tools/platform/cli/src/programs.rs:861 uses the gateway /v1/programs routes that stale task 4.7.5 removes; the design submits 0x101D transactions and reads 0x101D views (spec/layerx-merge/spec.kvx:53202); no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-interface-get-program-id</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-interface-publish</code> | notes | tools/platform/cli/src/programs.rs:861 uses the gateway /v1/programs routes that stale task 4.7.5 removes; the design submits 0x101D transactions and reads 0x101D views (spec/layerx-merge/spec.kvx:53202); no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-interface-publish-idempotency-key</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-interface-publish-interface</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-interface-publish-program-id</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-registry-get</code> | notes | tools/platform/cli/src/programs.rs:1181 uses the gateway /v1/programs routes that stale task 4.7.5 removes; the design submits 0x101D transactions and reads 0x101D views (spec/layerx-merge/spec.kvx:53202) |
| <code>interface.8.cli-cli-layerx-program-registry-list</code> | notes | tools/platform/cli/src/programs.rs:881 uses the gateway /v1/programs routes that stale task 4.7.5 removes; the design submits 0x101D transactions and reads 0x101D views (spec/layerx-merge/spec.kvx:53202) |
| <code>interface.8.cli-cli-layerx-program-registry-mirror-source</code> | notes | tools/platform/cli/src/programs.rs:994 uses the gateway /v1/programs routes that stale task 4.7.5 removes; the design submits 0x101D transactions and reads 0x101D views (spec/layerx-merge/spec.kvx:53202); no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-registry-mirror-source-archive</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-registry-mirror-source-plan</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-registry-mirror-source-source-uri</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-registry-verify-source</code> | notes | tools/platform/cli/src/programs.rs:994 uses the gateway /v1/programs routes that stale task 4.7.5 removes; the design submits 0x101D transactions and reads 0x101D views (spec/layerx-merge/spec.kvx:53202); no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-registry-verify-source-idempotency-key</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-registry-verify-source-program-id</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-registry-verify-source-source-digest</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-registry-verify-source-source-uri</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-simulate</code> | notes | tools/platform/cli/src/programs.rs:1190 uses the gateway /v1/programs routes that stale task 4.7.5 removes; the design submits 0x101D transactions and reads 0x101D views (spec/layerx-merge/spec.kvx:53202); no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-simulate-abi-version</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-simulate-access-declaration</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-simulate-account-sequence</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-simulate-calldata</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-simulate-capability</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-simulate-entrypoint</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-simulate-expires-at-ms</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-simulate-fee-limit</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-simulate-fuel</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-simulate-idempotency-key</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-simulate-key</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-simulate-memory-bytes</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-simulate-not-before-ms</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-simulate-output-bytes</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-simulate-output-values</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-simulate-program-id</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-simulate-response-capacity</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-simulate-storage-read-bytes</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-simulate-storage-write-bytes</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-simulate-table-elements</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-upgrade</code> | notes | tools/platform/cli/src/programs.rs:521 uses the gateway /v1/programs routes that stale task 4.7.5 removes; the design submits 0x101D transactions and reads 0x101D views (spec/layerx-merge/spec.kvx:53202); no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-upgrade-account-sequence</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-upgrade-allow-breaking-interface</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-upgrade-artifact</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-upgrade-clear-interface</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-upgrade-expires-at-ms</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-upgrade-fee-limit</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-upgrade-idempotency-key</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-upgrade-interface</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-upgrade-key</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-upgrade-migration-hook</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-upgrade-not-before-ms</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-upgrade-old-hash</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-upgrade-previous-state-root</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-upgrade-program-id</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-wind-down</code> | notes | tools/platform/cli/src/programs.rs:522 uses the gateway /v1/programs routes that stale task 4.7.5 removes; the design submits 0x101D transactions and reads 0x101D views (spec/layerx-merge/spec.kvx:53202); no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-wind-down-deprecate</code> | notes | tools/platform/cli/src/programs.rs:522 uses the gateway /v1/programs routes that stale task 4.7.5 removes; the design submits 0x101D transactions and reads 0x101D views (spec/layerx-merge/spec.kvx:53202); no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-wind-down-deprecate-account-sequence</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-wind-down-deprecate-deadline-batch</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-wind-down-deprecate-exit-program</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-wind-down-deprecate-expires-at-ms</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-wind-down-deprecate-fee-limit</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-wind-down-deprecate-idempotency-key</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-wind-down-deprecate-key</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-wind-down-deprecate-not-before-ms</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-wind-down-deprecate-previous-state-root</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-wind-down-deprecate-program-id</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-wind-down-exit</code> | notes | tools/platform/cli/src/programs.rs:522 uses the gateway /v1/programs routes that stale task 4.7.5 removes; the design submits 0x101D transactions and reads 0x101D views (spec/layerx-merge/spec.kvx:53202); no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-wind-down-exit-account</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-wind-down-exit-account-sequence</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-wind-down-exit-expires-at-ms</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-wind-down-exit-fee-limit</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-wind-down-exit-idempotency-key</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-wind-down-exit-key</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-wind-down-exit-not-before-ms</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-wind-down-exit-previous-state-root</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-wind-down-exit-program-id</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-wind-down-route</code> | notes | tools/platform/cli/src/programs.rs:522 uses the gateway /v1/programs routes that stale task 4.7.5 removes; the design submits 0x101D transactions and reads 0x101D views (spec/layerx-merge/spec.kvx:53202); no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-wind-down-route-account</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-wind-down-route-account-sequence</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-wind-down-route-asset</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-wind-down-route-destination</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-wind-down-route-expires-at-ms</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-wind-down-route-fee-limit</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-wind-down-route-idempotency-key</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-wind-down-route-key</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-wind-down-route-not-before-ms</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-wind-down-route-previous-state-root</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-wind-down-route-program-id</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-wind-down-route-seed</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-wind-down-tombstone</code> | notes | tools/platform/cli/src/programs.rs:522 uses the gateway /v1/programs routes that stale task 4.7.5 removes; the design submits 0x101D transactions and reads 0x101D views (spec/layerx-merge/spec.kvx:53202); no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-wind-down-tombstone-account-sequence</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-wind-down-tombstone-expires-at-ms</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-wind-down-tombstone-fee-limit</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-wind-down-tombstone-idempotency-key</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-wind-down-tombstone-key</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-wind-down-tombstone-not-before-ms</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-wind-down-tombstone-previous-state-root</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-program-wind-down-tombstone-program-id</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-receipt-get</code> | notes | tools/platform/cli/src/main.rs:1054 reads /v1/receipts; stale task 10.5.27 moves it to pax_sequencerReceipt and eth_getTransactionReceipt (spec/layerx-merge/spec.kvx:53146) |
| <code>interface.8.cli-cli-layerx-receipt-verify-asset</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-receipt-verify-batch-id</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-receipt-verify-previous-state-root</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-receipt-verify-resulting-state-root</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-receipt-verify-sequencer-public-key</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-register-key</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-token</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-token-burn</code> | notes | tools/platform/cli/src/wallet.rs:738 writes through execute_write on lx_getSequence; the design submits an EVM transaction (spec/layerx-merge/spec.kvx:53116); no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-token-burn-amount</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-token-burn-asset</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-token-burn-fee-limit</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-token-burn-key</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-token-burn-receipt-policy</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-token-burn-timeout-seconds</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-token-burn-wait</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-token-create</code> | notes | tools/platform/cli/src/wallet.rs:738 writes through execute_write on lx_getSequence; the design submits an EVM transaction (spec/layerx-merge/spec.kvx:53116); no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-token-create-decimals</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-token-create-fee-limit</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-token-create-key</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-token-create-name</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-token-create-receipt-policy</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-token-create-salt</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-token-create-supply-cap</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-token-create-symbol</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-token-create-timeout-seconds</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-token-create-wait</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-token-info</code> | notes | tools/platform/cli/src/wallet.rs:544 reads lx_getAsset; stale task 10.5.26 do_6 moves it to px_listAssets (spec/layerx-merge/spec.kvx:53116); no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-token-info-asset</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-token-list</code> | notes | tools/platform/cli/src/wallet.rs:553 reads lx_listAssets; stale task 10.5.26 do_6 moves it to px_listAssets (spec/layerx-merge/spec.kvx:53116) |
| <code>interface.8.cli-cli-layerx-token-list-cursor</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-token-list-limit</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-token-mint</code> | notes | tools/platform/cli/src/wallet.rs:738 writes through execute_write on lx_getSequence; the design submits an EVM transaction (spec/layerx-merge/spec.kvx:53116); no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-token-mint-amount</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-token-mint-asset</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-token-mint-fee-limit</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-token-mint-key</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-token-mint-receipt-policy</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-token-mint-timeout-seconds</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-token-mint-to</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-token-mint-wait</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-token-transfer</code> | notes | tools/platform/cli/src/wallet.rs:738 writes through execute_write on lx_getSequence; the design submits an EVM transaction (spec/layerx-merge/spec.kvx:53116); no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-token-transfer-amount</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-token-transfer-asset</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-token-transfer-fee-limit</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-token-transfer-key</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-token-transfer-receipt-policy</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-token-transfer-timeout-seconds</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-token-transfer-to</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-token-transfer-wait</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-wallet-balance</code> | notes | tools/platform/cli/src/wallet.rs:262 reads lx_getBalances; stale task 10.5.26 do_6 moves it to px_getBalances (spec/layerx-merge/spec.kvx:53116) |
| <code>interface.8.cli-cli-layerx-wallet-balance-asset</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-wallet-create</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-wallet-create-did</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-wallet-create-name</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-wallet-derive-export-private-keys</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-wallet-derive-mnemonic-file</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-wallet-derive-passphrase-file</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-wallet-estimate-fee</code> | notes | tools/platform/cli/src/rpc.rs:146 calls lx_estimateFee, which the design switches off (spec/layerx-merge/spec.kvx:109); no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-wallet-estimate-fee-canonical-hex</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-wallet-history</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-wallet-history-did</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-wallet-import</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-wallet-import-did</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-wallet-import-name</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-wallet-open-account</code> | notes | tools/platform/cli/src/wallet.rs:738 writes through execute_write on lx_getSequence; the design submits an EVM transaction (spec/layerx-merge/spec.kvx:53116); no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-wallet-open-account-asset</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-wallet-open-account-fee-limit</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-wallet-open-account-key</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-wallet-open-account-receipt-policy</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-wallet-open-account-timeout-seconds</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-wallet-open-account-wait</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-wallet-receipt</code> | notes | tools/platform/cli/src/wallet.rs:505 reads lx_getReceipt and /v1/receipts; the design reads pax_sequencerReceipt (spec/layerx-merge/spec.kvx:53116); no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-wallet-receipt-activity-id</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-wallet-receipt-receipt-policy</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-wallet-receipt-wait</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-wallet-send</code> | notes | tools/platform/cli/src/wallet.rs:738 writes through lx_getSequence and send_activity; stale task 10.5.26 moves it to eth_sendRawTransaction (spec/layerx-merge/spec.kvx:53116); no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-wallet-send-amount</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-wallet-send-asset</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-wallet-send-fee-limit</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-wallet-send-key</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-wallet-send-receipt-policy</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-wallet-send-timeout-seconds</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-wallet-send-to</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-wallet-send-wait</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-wallet-watch</code> | notes | tools/platform/cli/src/rpc_subscription.rs:19 subscribes with lx_subscribe; stale task 10.5.30 moves it to eth_subscribe (spec/layerx-merge/spec.kvx:53230); no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-wallet-watch-account-id</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-wallet-watch-timeout-seconds</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-wallet-watch-topic</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-workspace-all-allow-production</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-workspace-all-ci</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-workspace-all-environment</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-workspace-all-fail-fast</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-workspace-all-module</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-workspace-all-yes</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-workspace-build-allow-production</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-workspace-build-ci</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-workspace-build-environment</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-workspace-build-fail-fast</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-workspace-build-module</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-workspace-build-yes</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-workspace-doctor</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-workspace-doctor-all</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-workspace-doctor-environment</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-workspace-doctor-module</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-workspace-install</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-workspace-install-all</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-workspace-install-allow-production</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-workspace-install-ci</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-workspace-install-dry-run</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-workspace-install-environment</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-workspace-install-fail-fast</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-workspace-install-module</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-workspace-install-yes</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-workspace-test</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-workspace-test-all</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-workspace-test-allow-production</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-workspace-test-ci</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-workspace-test-dry-run</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-workspace-test-environment</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-workspace-test-fail-fast</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-workspace-test-module</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-cli-layerx-workspace-test-yes</code> | notes | no in-repo invocation outside the defining file |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.8.534.1</code> | req | 8534.1 |
| <code>acceptance.8.534.1</code> | scenario | Given a stub answering eth_estimateGas 21000 and eth_gasPrice 2, when layerx wallet estimate-fee runs with an EIP-1559 transaction, then the reported fee is 42000 and the stub saw block tag pending. |
| <code>acceptance.8.534.1</code> | test | tools/platform/cli/tests/wallet_estimate_fee.rs::estimate_fee_multiplies_gas_by_gas_price (planned) |
| <code>acceptance.8.534.1</code> | qualification | UNRUN |
| <code>req.8534</code> | ac_1 | layerx wallet estimate-fee reports eth_estimateGas at pending multiplied by eth_gasPrice. |
| <code>acceptance.8.534.2</code> | req | 8534.2 |
| <code>acceptance.8.534.2</code> | scenario | Given a kernel canonical activity hex, when estimate-fee runs, then it fails with an invalid transaction error and the stub received no request. |
| <code>acceptance.8.534.2</code> | test | tools/platform/cli/tests/wallet_estimate_fee.rs::estimate_fee_refuses_kernel_activity_hex (planned) |
| <code>acceptance.8.534.2</code> | qualification | UNRUN |
| <code>req.8534</code> | ac_2 | estimate-fee refuses a kernel canonical activity hex without calling the endpoint. |
| <code>acceptance.8.535.1</code> | req | 8535.1 |
| <code>acceptance.8.535.1</code> | scenario | Given a stub with eth_getTransactionCount 7 and a receipted pax_sequencerReceipt, when layerx wallet send runs, then the stub saw one eth_sendRawTransaction whose decoded nonce is 7 and chainId 125, and the output names rung receipted and the txHash. |
| <code>acceptance.8.535.1</code> | test | tools/platform/cli/tests/wallet_evm_writes.rs::wallet_send_submits_raw_evm_transaction (planned) |
| <code>acceptance.8.535.1</code> | qualification | UNRUN |
| <code>req.8535</code> | ac_1 | layerx wallet send submits one eth_sendRawTransaction with chainId 125 and the pending nonce, then reports the receipted rung. |
| <code>acceptance.8.535.2</code> | req | 8535.2 |
| <code>acceptance.8.535.2</code> | scenario | Given the recording stub, when each token write runs once, then no recorded method starts with lx_. |
| <code>acceptance.8.535.2</code> | test | tools/platform/cli/tests/wallet_evm_writes.rs::token_writes_never_call_lx_methods (planned) |
| <code>acceptance.8.535.2</code> | qualification | UNRUN |
| <code>req.8535</code> | ac_2 | token create, mint, burn and transfer issue no lx_ method. |
| <code>acceptance.8.535.3</code> | req | 8535.3 |
| <code>acceptance.8.535.3</code> | scenario | Given a stub whose pax_sequencerReceipt returns status 0 with Error(string) LXP_INSUFFICIENT_BALANCE, when wallet send runs, then the command fails and prints LXP_INSUFFICIENT_BALANCE. |
| <code>acceptance.8.535.3</code> | test | tools/platform/cli/tests/wallet_evm_writes.rs::wallet_send_reports_receipted_revert (planned) |
| <code>acceptance.8.535.3</code> | qualification | UNRUN |
| <code>req.8535</code> | ac_3 | A receipted revert is reported with its LXP_ reason and a failing exit status. |
| <code>acceptance.8.536.1</code> | req | 8536.1 |
| <code>acceptance.8.536.1</code> | scenario | Given a recording stub answering px_getBalances, when wallet balance runs, then exactly px_getBalances was called and the printed balance equals the stub's. |
| <code>acceptance.8.536.1</code> | test | tools/platform/cli/tests/wallet_px_reads.rs::wallet_balance_reads_px_get_balances (planned) |
| <code>acceptance.8.536.1</code> | qualification | UNRUN |
| <code>req.8536</code> | ac_1 | layerx wallet balance issues px_getBalances and no lx_ method. |
| <code>acceptance.8.536.2</code> | req | 8536.2 |
| <code>acceptance.8.536.2</code> | scenario | Given the stub, when token list and token info run, then each issued px_listAssets and neither issued lx_listAssets or lx_getAsset. |
| <code>acceptance.8.536.2</code> | test | tools/platform/cli/tests/wallet_px_reads.rs::token_list_reads_px_list_assets (planned) |
| <code>acceptance.8.536.2</code> | qualification | UNRUN |
| <code>req.8536</code> | ac_2 | layerx token list and token info issue px_listAssets. |
| <code>acceptance.8.536.3</code> | req | 8536.3 |
| <code>acceptance.8.536.3</code> | scenario | Given a stub returning the V1 receipt of sdk/platform/conformance/fixtures/sequencer-receipt-v1.json, when wallet receipt runs with its txHash, then the output names rung receipted. |
| <code>acceptance.8.536.3</code> | test | tools/platform/cli/tests/wallet_px_reads.rs::wallet_receipt_reads_sequencer_receipt_by_tx_hash (planned) |
| <code>acceptance.8.536.3</code> | qualification | UNRUN |
| <code>req.8536</code> | ac_3 | layerx wallet receipt reads pax_sequencerReceipt for a txHash and prints the verified rung. |
| <code>acceptance.8.537.1</code> | req | 8537.1 |
| <code>acceptance.8.537.1</code> | scenario | Given a recording stub, when program simulate runs, then one eth_call with to 0x101D, data starting 0x3044bd56 and tag pending was sent and the output has data.outcome. |
| <code>acceptance.8.537.1</code> | test | tools/platform/cli/tests/program_evm_calls.rs::program_simulate_uses_eth_call_on_layerxprograms (planned) |
| <code>acceptance.8.537.1</code> | qualification | UNRUN |
| <code>req.8537</code> | ac_1 | layerx program simulate issues eth_call to 0x101D with selector 0x3044bd56 at pending and prints data.outcome. |
| <code>acceptance.8.537.2</code> | req | 8537.2 |
| <code>acceptance.8.537.2</code> | scenario | Given a recording stub with a receipted pax_sequencerReceipt, when program call runs, then one eth_sendRawTransaction was sent whose decoded to is 0x101D and data starts 0x54bd2b48, and the output has data.tx_hash and data.rung. |
| <code>acceptance.8.537.2</code> | test | tools/platform/cli/tests/program_evm_calls.rs::program_call_submits_raw_transaction (planned) |
| <code>acceptance.8.537.2</code> | qualification | UNRUN |
| <code>req.8537</code> | ac_2 | layerx program call submits one eth_sendRawTransaction to 0x101D and prints data.tx_hash and data.rung. |
| <code>acceptance.8.537.3</code> | req | 8537.3 |
| <code>acceptance.8.537.3</code> | scenario | Given a stub answering pax_sequencerReceipt, when receipt get runs with a 0x-prefixed 32-byte hash, then the stub saw pax_sequencerReceipt with that hash and no HTTP request to /v1/receipts. |
| <code>acceptance.8.537.3</code> | test | tools/platform/cli/tests/program_evm_calls.rs::receipt_get_accepts_transaction_hash (planned) |
| <code>acceptance.8.537.3</code> | qualification | UNRUN |
| <code>req.8537</code> | ac_3 | layerx receipt get accepts a txHash and reads pax_sequencerReceipt. |
| <code>acceptance.8.538.1</code> | req | 8538.1 |
| <code>acceptance.8.538.1</code> | scenario | Given the deploy-v4 vector inputs and a recording stub, when program deploy runs, then the submitted transaction data equals the vector's evm.calldata and to is 0x101D. |
| <code>acceptance.8.538.1</code> | test | tools/platform/cli/tests/program_evm_lifecycle.rs::program_deploy_calldata_matches_vector (planned) |
| <code>acceptance.8.538.1</code> | qualification | UNRUN |
| <code>req.8538</code> | ac_1 | layerx program deploy submits calldata equal to the evm.calldata of native-program-deploy-v4.json. |
| <code>acceptance.8.538.2</code> | req | 8538.2 |
| <code>acceptance.8.538.2</code> | scenario | Given a recording stub, when program wind-down route, deprecate, tombstone and exit each run, then four raw transactions to 0x101D are sent and no HTTP path starts with /v1/programs. |
| <code>acceptance.8.538.2</code> | test | tools/platform/cli/tests/program_evm_lifecycle.rs::program_wind_down_modes_use_layerxprograms (planned) |
| <code>acceptance.8.538.2</code> | qualification | UNRUN |
| <code>req.8538</code> | ac_2 | Each wind-down mode submits a transaction to 0x101D and no request to /v1/programs. |
| <code>acceptance.8.539.1</code> | req | 8539.1 |
| <code>acceptance.8.539.1</code> | scenario | Given a recording stub returning an ABI-encoded program record, when program discover runs, then one eth_call with data starting 0xb1ffb3d4 was sent and the printed program id equals the record's. |
| <code>acceptance.8.539.1</code> | test | tools/platform/cli/tests/program_evm_registry.rs::program_discover_reads_get_program (planned) |
| <code>acceptance.8.539.1</code> | qualification | UNRUN |
| <code>req.8539</code> | ac_1 | layerx program discover issues eth_call to 0x101D with selector 0xb1ffb3d4 and no /v1/programs request. |
| <code>acceptance.8.539.2</code> | req | 8539.2 |
| <code>acceptance.8.539.2</code> | scenario | Given the stub, when program interface get runs, then one eth_call with data starting 0x8efc7240 was sent and no HTTP path starts with /v1/programs. |
| <code>acceptance.8.539.2</code> | test | tools/platform/cli/tests/program_evm_registry.rs::program_interface_get_reads_get_interface (planned) |
| <code>acceptance.8.539.2</code> | qualification | UNRUN |
| <code>req.8539</code> | ac_2 | layerx program interface get issues eth_call with selector 0x8efc7240. |
| <code>acceptance.8.539.3</code> | req | 8539.3 |
| <code>acceptance.8.539.3</code> | scenario | Given the stub with two program events, when registry list runs, then one eth_getLogs with address 0x101D was sent and two programs are printed. |
| <code>acceptance.8.539.3</code> | test | tools/platform/cli/tests/program_evm_registry.rs::program_registry_list_reads_logs (planned) |
| <code>acceptance.8.539.3</code> | qualification | UNRUN |
| <code>req.8539</code> | ac_3 | layerx program registry list reads eth_getLogs on 0x101D. |
| <code>acceptance.8.540.1</code> | req | 8540.1 |
| <code>acceptance.8.540.1</code> | scenario | Given a WebSocket stub that answers eth_subscribe and pushes one eth_subscription log, when wallet watch runs with a short timeout, then the stub saw eth_subscribe ["logs", filter] and the output contains the log's transaction hash. |
| <code>acceptance.8.540.1</code> | test | tools/platform/cli/tests/wallet_watch_logs.rs::wallet_watch_subscribes_to_logs (planned) |
| <code>acceptance.8.540.1</code> | qualification | UNRUN |
| <code>req.8540</code> | ac_1 | layerx wallet watch sends eth_subscribe with the logs kind and prints a received log. |
| <code>acceptance.8.540.2</code> | req | 8540.2 |
| <code>acceptance.8.540.2</code> | scenario | Given a stub that pushes an lx_subscription notification, when wallet watch runs, then it fails with an unexpected notification error. |
| <code>acceptance.8.540.2</code> | test | tools/platform/cli/tests/wallet_watch_logs.rs::wallet_watch_refuses_lx_subscription_notifications (planned) |
| <code>acceptance.8.540.2</code> | qualification | UNRUN |
| <code>req.8540</code> | ac_2 | A notification with method lx_subscription is refused. |
| <code>acceptance.8.541.1</code> | req | 8541.1 |
| <code>acceptance.8.541.1</code> | scenario | Given a recording stub, when layerx faucet runs with an identity session, then one faucet service-claim request was sent and no method lx_requestFunds was called. |
| <code>acceptance.8.541.1</code> | test | tools/platform/cli/tests/faucet_route.rs::faucet_posts_service_claim_route (planned) |
| <code>acceptance.8.541.1</code> | qualification | UNRUN |
| <code>req.8541</code> | ac_1 | layerx faucet sends one service-claim request and no lx_requestFunds. |
| <code>acceptance.8.541.2</code> | req | 8541.2 |
| <code>acceptance.8.541.2</code> | scenario | Given no identity session configured, when layerx faucet runs, then it fails with identity_session_required and the stub received nothing. |
| <code>acceptance.8.541.2</code> | test | tools/platform/cli/tests/faucet_route.rs::faucet_requires_identity_session (planned) |
| <code>acceptance.8.541.2</code> | qualification | UNRUN |
| <code>req.8541</code> | ac_2 | layerx faucet without an identity session fails before any request. |
| <code>acceptance.8.542.1</code> | req | 8542.1 |
| <code>acceptance.8.542.1</code> | scenario | Given layerx mcp serve over stdio and a recording JSON-RPC stub, when the submit tool is invoked, then one eth_sendRawTransaction was sent, no HTTP path /v1/activities was requested, and the tool result has txHash and rung. |
| <code>acceptance.8.542.1</code> | test | tools/platform/cli/tests/mcp_evm_submit.rs::mcp_submit_tool_sends_raw_transaction (planned) |
| <code>acceptance.8.542.1</code> | qualification | UNRUN |
| <code>req.8542</code> | ac_1 | The MCP submit tool served by layerx mcp serve submits eth_sendRawTransaction and returns txHash and rung. |
| <code>acceptance.8.542.2</code> | req | 8542.2 |
| <code>acceptance.8.542.2</code> | scenario | Given valid parameter lists for each name, when request builds each envelope, then each succeeds with the method name unchanged. |
| <code>acceptance.8.542.2</code> | test | tools/platform/cli/tests/mcp_evm_submit.rs::rpc_request_admits_major_two_names (planned) |
| <code>acceptance.8.542.2</code> | qualification | UNRUN |
| <code>req.8542</code> | ac_2 | request accepts eth_sendRawTransaction, eth_getTransactionCount and pax_sequencerReceipt with valid parameters. |
| <code>capability.8.48.1</code> | title | layerx wallet estimate-fee quotes through eth_estimateGas |
| <code>capability.8.48.1</code> | reqs | 8534.1<br>8534.2 |
| <code>capability.8.48.2</code> | title | layerx wallet send, open-account and token writes submit EVM transactions |
| <code>capability.8.48.2</code> | reqs | 8535.1<br>8535.2<br>8535.3 |
| <code>capability.8.48.3</code> | title | layerx wallet and token reads use px_getBalances, px_listAssets and pax_sequencerReceipt |
| <code>capability.8.48.3</code> | reqs | 8536.1<br>8536.2<br>8536.3 |
| <code>capability.8.49.1</code> | title | layerx program call and simulate use 0x101D and receipt get takes a transaction hash |
| <code>capability.8.49.1</code> | reqs | 8537.1<br>8537.2<br>8537.3 |
| <code>capability.8.49.2</code> | title | layerx program deploy, upgrade and wind-down submit layerxprograms transactions |
| <code>capability.8.49.2</code> | reqs | 8538.1<br>8538.2 |
| <code>capability.8.49.3</code> | title | layerx program discover, interface and registry commands read layerxprograms views |
| <code>capability.8.49.3</code> | reqs | 8539.1<br>8539.2<br>8539.3 |
| <code>capability.8.50.1</code> | title | layerx wallet watch subscribes with eth_subscribe logs |
| <code>capability.8.50.1</code> | reqs | 8540.1<br>8540.2 |
| <code>capability.8.50.2</code> | title | layerx faucet claims through the router faucet route |
| <code>capability.8.50.2</code> | reqs | 8541.1<br>8541.2 |
| <code>capability.8.50.3</code> | title | layerx mcp serve and a2a submit raw EVM transactions |
| <code>capability.8.50.3</code> | reqs | 8542.1<br>8542.2 |

## cli-config-keys

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.cfg-cli-configuration-current-environment</code> | <code>config_key</code> | <code>stable</code> | <code>tools/platform/cli/src/config.rs:30</code> | <code>pub current_environment: String,</code> |
| <code>interface.8.cfg-cli-configuration-default-key</code> | <code>config_key</code> | <code>stable</code> | <code>tools/platform/cli/src/config.rs:31</code> | <code>pub default_key: Option&lt;String&gt;,</code> |
| <code>interface.8.cfg-cli-configuration-environments</code> | <code>config_key</code> | <code>stable</code> | <code>tools/platform/cli/src/config.rs:32</code> | <code>pub environments: BTreeMap&lt;String, Environment&gt;,</code> |
| <code>interface.8.cfg-cli-configuration-keys</code> | <code>config_key</code> | <code>stable</code> | <code>tools/platform/cli/src/config.rs:33</code> | <code>pub keys: BTreeMap&lt;String, KeyMetadata&gt;,</code> |
| <code>interface.8.cfg-cli-configuration-version</code> | <code>config_key</code> | <code>stable</code> | <code>tools/platform/cli/src/config.rs:29</code> | <code>version: u16,</code> |
| <code>interface.8.cfg-cli-environment-endpoint</code> | <code>config_key</code> | <code>stable</code> | <code>tools/platform/cli/src/config.rs:15</code> | <code>pub endpoint: String,</code> |
| <code>interface.8.cfg-cli-environment-network-id</code> | <code>config_key</code> | <code>stable</code> | <code>tools/platform/cli/src/config.rs:16</code> | <code>pub network_id: u32,</code> |
| <code>interface.8.cfg-cli-environment-sequencer-trust-anchor</code> | <code>config_key</code> | <code>stable</code> | <code>tools/platform/cli/src/config.rs:18</code> | <code>pub sequencer_trust_anchor: Option&lt;String&gt;,</code> |
| <code>interface.8.cfg-cli-keymetadata-did</code> | <code>config_key</code> | <code>stable</code> | <code>tools/platform/cli/src/config.rs:23</code> | <code>pub did: String,</code> |
| <code>interface.8.cfg-cli-keymetadata-public-key</code> | <code>config_key</code> | <code>stable</code> | <code>tools/platform/cli/src/config.rs:24</code> | <code>pub public_key: String,</code> |
| <code>interface.8.cfg-cli-walletpolicy-checkpoint-context-digest</code> | <code>config_key</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet.rs:1002</code> | <code>checkpoint_context_digest: Option&lt;String&gt;,</code> |
| <code>interface.8.cfg-cli-walletpolicy-first-batch</code> | <code>config_key</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet.rs:1000</code> | <code>first_batch: u64,</code> |
| <code>interface.8.cfg-cli-walletpolicy-last-batch</code> | <code>config_key</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet.rs:1001</code> | <code>last_batch: u64,</code> |
| <code>interface.8.cfg-cli-walletpolicy-network-id</code> | <code>config_key</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet.rs:997</code> | <code>network_id: u32,</code> |
| <code>interface.8.cfg-cli-walletpolicy-protocol-version</code> | <code>config_key</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet.rs:996</code> | <code>protocol_version: u16,</code> |
| <code>interface.8.cfg-cli-walletpolicy-sequencer-id</code> | <code>config_key</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet.rs:998</code> | <code>sequencer_id: String,</code> |
| <code>interface.8.cfg-cli-walletpolicy-sequencer-key</code> | <code>config_key</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet.rs:999</code> | <code>sequencer_key: String,</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.cfg-cli-configuration-default-key</code> | notes | no in-repo config file sets the key outside the defining file |
| <code>interface.8.cfg-cli-configuration-environments</code> | notes | 31 callers total |
| <code>interface.8.cfg-cli-configuration-keys</code> | notes | 158 callers total |
| <code>interface.8.cfg-cli-configuration-version</code> | notes | 5297 callers total |
| <code>interface.8.cfg-cli-environment-endpoint</code> | notes | 490 callers total |
| <code>interface.8.cfg-cli-environment-network-id</code> | notes | 336 callers total |
| <code>interface.8.cfg-cli-keymetadata-did</code> | notes | 269 callers total |
| <code>interface.8.cfg-cli-keymetadata-public-key</code> | notes | 431 callers total |
| <code>interface.8.cfg-cli-walletpolicy-checkpoint-context-digest</code> | notes | no in-repo config file sets the key outside the defining file |
| <code>interface.8.cfg-cli-walletpolicy-first-batch</code> | notes | 33 callers total |
| <code>interface.8.cfg-cli-walletpolicy-last-batch</code> | notes | 31 callers total |
| <code>interface.8.cfg-cli-walletpolicy-network-id</code> | notes | 336 callers total |
| <code>interface.8.cfg-cli-walletpolicy-protocol-version</code> | notes | 171 callers total |
| <code>interface.8.cfg-cli-walletpolicy-sequencer-id</code> | notes | 139 callers total |

## cli-env-vars

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.env-cli-cargo</code> | <code>env_var</code> | <code>stable</code> | <code>tools/platform/cli/src/programs.rs:253</code> | <code>let cargo = std::env::var_os("CARGO").unwrap_or_else(&#124;&#124; "cargo".into());</code> |
| <code>interface.8.env-cli-home</code> | <code>env_var</code> | <code>stable</code> | <code>tools/platform/cli/src/config.rs:155</code> | <code>let base = env::var_os("HOME").ok_or_else(&#124;&#124; {</code> |
| <code>interface.8.env-cli-layerx-api-token</code> | <code>env_var</code> | <code>stable</code> | <code>tools/platform/cli/src/a2a.rs:678</code> | <code>.env_remove("LAYERX_API_TOKEN")</code> |
| <code>interface.8.env-cli-layerx-auth-token</code> | <code>env_var</code> | <code>stable</code> | <code>tools/platform/cli/src/a2a.rs:679</code> | <code>.env_remove("LAYERX_AUTH_TOKEN");</code> |
| <code>interface.8.env-cli-layerx-beta-registry-publication-key-file</code> | <code>env_var</code> | <code>stable</code> | <code>tools/platform/cli/src/programs.rs:874</code> | <code>pub const REGISTRY_PUBLICATION_KEY_VARIABLE: &amp;str = "LAYERX_BETA_REGISTRY_PUBLICATION_KEY_FILE";</code> |
| <code>interface.8.env-cli-layerx-beta-registry-publication-token-file</code> | <code>env_var</code> | <code>stable</code> | <code>tools/platform/cli/src/programs.rs:873</code> | <code>pub const REGISTRY_PUBLICATION_TOKEN_VARIABLE: &amp;str = "LAYERX_BETA_REGISTRY_PUBLICATION_TOKEN_FILE";</code> |
| <code>interface.8.env-cli-layerx-beta-registry-url</code> | <code>env_var</code> | <code>stable</code> | <code>tools/platform/cli/src/programs.rs:872</code> | <code>pub const REGISTRY_URL_VARIABLE: &amp;str = "LAYERX_BETA_REGISTRY_URL";</code> |
| <code>interface.8.env-cli-layerx-client-certificate-file</code> | <code>env_var</code> | <code>stable</code> | <code>tools/platform/cli/src/http.rs:298</code> | <code>let certificate = std::env::var_os("LAYERX_CLIENT_CERTIFICATE_FILE");</code> |
| <code>interface.8.env-cli-layerx-client-private-key-file</code> | <code>env_var</code> | <code>stable</code> | <code>tools/platform/cli/src/http.rs:299</code> | <code>let key = std::env::var_os("LAYERX_CLIENT_PRIVATE_KEY_FILE");</code> |
| <code>interface.8.env-cli-layerx-config</code> | <code>env_var</code> | <code>stable</code> | <code>tools/platform/cli/src/config.rs:149</code> | <code>if let Some(explicit) = env::var_os("LAYERX_CONFIG") {</code> |
| <code>interface.8.env-cli-layerx-credential-passphrase</code> | <code>env_var</code> | <code>stable</code> | <code>tools/platform/cli/src/file_store.rs:36</code> | <code>std::env::var("LAYERX_CREDENTIAL_PASSPHRASE")</code> |
| <code>interface.8.env-cli-layerx-credential-store</code> | <code>env_var</code> | <code>stable</code> | <code>tools/platform/cli/src/credential.rs:14</code> | <code>const STORE_VARIABLE: &amp;str = "LAYERX_CREDENTIAL_STORE";</code> |
| <code>interface.8.env-cli-layerx-endpoint</code> | <code>env_var</code> | <code>stable</code> | <code>tools/platform/cli/src/workspace.rs:997</code> | <code>.env("LAYERX_ENDPOINT", &amp;environment.endpoint)</code> |
| <code>interface.8.env-cli-layerx-environment</code> | <code>env_var</code> | <code>stable</code> | <code>tools/platform/cli/src/workspace.rs:996</code> | <code>.env("LAYERX_ENVIRONMENT", &amp;environment.name)</code> |
| <code>interface.8.env-cli-layerx-gateway-key-id</code> | <code>env_var</code> | <code>stable</code> | <code>tools/platform/cli/src/toolset.rs:67</code> | <code>if let Ok(expected) = std::env::var("LAYERX_GATEWAY_KEY_ID") {</code> |
| <code>interface.8.env-cli-layerx-install-root</code> | <code>env_var</code> | <code>stable</code> | <code>tools/platform/cli/src/install/mod.rs:1117</code> | <code>if let Some(root) = env::var_os("LAYERX_INSTALL_ROOT") {</code> |
| <code>interface.8.env-cli-layerx-network-id</code> | <code>env_var</code> | <code>stable</code> | <code>tools/platform/cli/src/workspace.rs:998</code> | <code>.env("LAYERX_NETWORK_ID", environment.network_id.to_string());</code> |
| <code>interface.8.env-cli-layerx-program-sdk</code> | <code>env_var</code> | <code>stable</code> | <code>tools/platform/cli/src/scaffold.rs:68</code> | <code>std::env::var("LAYERX_PROGRAM_SDK").unwrap_or_else(&#124;_&#124; SDK_DEFAULT_PATH.to_string())</code> |
| <code>interface.8.env-cli-layerx-repo-root</code> | <code>env_var</code> | <code>stable</code> | <code>tools/platform/cli/src/workspace.rs:1123</code> | <code>if let Some(explicit) = env::var_os("LAYERX_REPO_ROOT") {</code> |
| <code>interface.8.env-cli-layerx-token</code> | <code>env_var</code> | <code>stable</code> | <code>tools/platform/cli/src/a2a.rs:677</code> | <code>.env_remove("LAYERX_TOKEN")</code> |
| <code>interface.8.env-cli-no-color</code> | <code>env_var</code> | <code>stable</code> | <code>tools/platform/cli/src/workspace.rs:1427</code> | <code>io::stdout().is_terminal() &amp;&amp; env::var_os("NO_COLOR").is_none()</code> |
| <code>interface.8.env-cli-path</code> | <code>env_var</code> | <code>stable</code> | <code>tools/platform/cli/src/workspace.rs:1150</code> | <code>let Some(path) = env::var_os("PATH") else {</code> |
| <code>interface.8.env-cli-pathext</code> | <code>env_var</code> | <code>stable</code> | <code>tools/platform/cli/src/workspace.rs:1162</code> | <code>return env::var_os("PATHEXT").is_some_and(&#124;extensions&#124; {</code> |
| <code>interface.8.env-cli-xdg-config-home</code> | <code>env_var</code> | <code>stable</code> | <code>tools/platform/cli/src/config.rs:152</code> | <code>if let Some(base) = env::var_os("XDG_CONFIG_HOME") {</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.env-cli-cargo</code> | notes | system variable; callers not enumerated |
| <code>interface.8.env-cli-home</code> | notes | system variable; callers not enumerated |
| <code>interface.8.env-cli-layerx-auth-token</code> | notes | no in-repo setter or reader outside the defining file |
| <code>interface.8.env-cli-layerx-beta-registry-publication-key-file</code> | notes | no in-repo setter or reader outside the defining file |
| <code>interface.8.env-cli-layerx-beta-registry-publication-token-file</code> | notes | no in-repo setter or reader outside the defining file |
| <code>interface.8.env-cli-layerx-beta-registry-url</code> | notes | no in-repo setter or reader outside the defining file |
| <code>interface.8.env-cli-layerx-client-certificate-file</code> | notes | no in-repo setter or reader outside the defining file |
| <code>interface.8.env-cli-layerx-client-private-key-file</code> | notes | no in-repo setter or reader outside the defining file |
| <code>interface.8.env-cli-layerx-endpoint</code> | notes | no in-repo setter or reader outside the defining file |
| <code>interface.8.env-cli-layerx-environment</code> | notes | no in-repo setter or reader outside the defining file |
| <code>interface.8.env-cli-layerx-program-sdk</code> | notes | no in-repo setter or reader outside the defining file |
| <code>interface.8.env-cli-no-color</code> | notes | system variable; callers not enumerated |
| <code>interface.8.env-cli-path</code> | notes | system variable; callers not enumerated |
| <code>interface.8.env-cli-pathext</code> | notes | system variable; callers not enumerated |
| <code>interface.8.env-cli-xdg-config-home</code> | notes | system variable; callers not enumerated |

## cli-error-codes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.err-cli-32001</code> | <code>error_code</code> | <code>stable</code> | <code>tools/platform/cli/src/a2a.rs:371</code> | <code>return failure(identifier, -32001, "the task was not found");</code> |
| <code>interface.8.err-cli-32600</code> | <code>error_code</code> | <code>stable</code> | <code>tools/platform/cli/src/a2a.rs:223</code> | <code>return failure(identifier, -32600, "the message did not name a method");</code> |
| <code>interface.8.err-cli-32601</code> | <code>error_code</code> | <code>stable</code> | <code>tools/platform/cli/src/a2a.rs:235</code> | <code>_ =&gt; failure(identifier, -32601, "the method is not implemented"),</code> |
| <code>interface.8.err-cli-32602</code> | <code>error_code</code> | <code>stable</code> | <code>tools/platform/cli/src/a2a.rs:247</code> | <code>return failure(identifier, -32602, "the request carried no message");</code> |
| <code>interface.8.err-cli-32603</code> | <code>error_code</code> | <code>stable</code> | <code>tools/platform/cli/src/a2a.rs:295</code> | <code>return failure(identifier, -32603, "the task ledger is unavailable");</code> |
| <code>interface.8.err-cli-32700</code> | <code>error_code</code> | <code>stable</code> | <code>tools/platform/cli/src/a2a.rs:219</code> | <code>return failure(Value::Null, -32700, "the message is not valid JSON");</code> |
| <code>interface.8.err-cli-gateway-response-invalid</code> | <code>error_code</code> | <code>stable</code> | <code>tools/platform/cli/src/http.rs:431</code> | <code>"code": "gateway_response_invalid",</code> |
| <code>interface.8.err-cli-gateway-response-unreadable</code> | <code>error_code</code> | <code>stable</code> | <code>tools/platform/cli/src/http.rs:414</code> | <code>"code": "gateway_response_unreadable",</code> |
| <code>interface.8.err-cli-gateway-transport-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>tools/platform/cli/src/http.rs:188</code> | <code>"failure": {"code": "gateway_transport_unavailable", "detail": error.to_string()},</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.err-cli-32001</code> | notes | 183 callers total |
| <code>interface.8.err-cli-32600</code> | notes | 41 callers total |
| <code>interface.8.err-cli-32601</code> | notes | 115 callers total |
| <code>interface.8.err-cli-32602</code> | notes | 440 callers total |
| <code>interface.8.err-cli-32603</code> | notes | 69 callers total |
| <code>interface.8.err-cli-gateway-response-invalid</code> | notes | no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-cli-gateway-response-unreadable</code> | notes | no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-cli-gateway-transport-unavailable</code> | notes | no in-repo consumer quotes the code outside the defining file |

## cli-file-formats

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.fmt-cli-vault-file</code> | <code>file_format</code> | <code>stable</code> | <code>tools/platform/cli/src/file_store.rs:11</code> | <code>const HEADER: &amp;[u8; 8] = b"LXVAULT1";</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.fmt-cli-vault-file</code> | notes | credential vault file header; read and written only by the defining module |

## cli-http-routes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.http-cli-get-well-known-agent-card-json</code> | <code>http_route</code> | <code>stable</code> | <code>tools/platform/cli/src/a2a.rs:31</code> | <code>const CARD_ROUTES: [&amp;str; 2] = ["/.well-known/agent-card.json", "/.well-known/agent.json"];</code> |
| <code>interface.8.http-cli-get-well-known-agent-json</code> | <code>http_route</code> | <code>stable</code> | <code>tools/platform/cli/src/a2a.rs:31</code> | <code>const CARD_ROUTES: [&amp;str; 2] = ["/.well-known/agent-card.json", "/.well-known/agent.json"];</code> |
| <code>interface.8.http-cli-post</code> | <code>http_route</code> | <code>stable</code> | <code>tools/platform/cli/src/a2a.rs:192</code> | <code>"POST" if request.path == "/" =&gt; {</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.http-cli-get-well-known-agent-card-json</code> | notes | agent card served by layerx a2a serve |
| <code>interface.8.http-cli-get-well-known-agent-json</code> | notes | agent card served by layerx a2a serve; no in-repo client cites the path outside the defining file |
| <code>interface.8.http-cli-post</code> | notes | path is too generic to attribute callers |

## cli-jsonrpc-methods

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.rpc-cli-a2a-message-send</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>tools/platform/cli/src/a2a.rs:227</code> | <code>"message/send" =&gt; send(runtime, tools, tasks, identifier, &amp;parameters),</code> |
| <code>interface.8.rpc-cli-a2a-message-stream-refused</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>tools/platform/cli/src/a2a.rs:230</code> | <code>"message/stream" &#124; "tasks/resubscribe" &#124; "tasks/pushNotificationConfig/set" =&gt; failure(</code> |
| <code>interface.8.rpc-cli-a2a-tasks-cancel</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>tools/platform/cli/src/a2a.rs:229</code> | <code>"tasks/cancel" =&gt; cancel(tasks, identifier, &amp;parameters),</code> |
| <code>interface.8.rpc-cli-a2a-tasks-get</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>tools/platform/cli/src/a2a.rs:228</code> | <code>"tasks/get" =&gt; fetch(tasks, identifier, &amp;parameters),</code> |
| <code>interface.8.rpc-cli-a2a-tasks-pushnotificationconfig-set-refused</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>tools/platform/cli/src/a2a.rs:230</code> | <code>"message/stream" &#124; "tasks/resubscribe" &#124; "tasks/pushNotificationConfig/set" =&gt; failure(</code> |
| <code>interface.8.rpc-cli-a2a-tasks-resubscribe-refused</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>tools/platform/cli/src/a2a.rs:230</code> | <code>"message/stream" &#124; "tasks/resubscribe" &#124; "tasks/pushNotificationConfig/set" =&gt; failure(</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.rpc-cli-a2a-message-stream-refused</code> | notes | answered with a failure; not implemented; no in-repo caller outside the defining file |
| <code>interface.8.rpc-cli-a2a-tasks-cancel</code> | notes | no in-repo caller outside the defining file |
| <code>interface.8.rpc-cli-a2a-tasks-get</code> | notes | no in-repo caller outside the defining file |
| <code>interface.8.rpc-cli-a2a-tasks-pushnotificationconfig-set-refused</code> | notes | answered with a failure; not implemented; no in-repo caller outside the defining file |
| <code>interface.8.rpc-cli-a2a-tasks-resubscribe-refused</code> | notes | answered with a failure; not implemented; no in-repo caller outside the defining file |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.8.543.1</code> | req | 8543.1 |
| <code>acceptance.8.543.1</code> | scenario | Given the 26 lx_ names of the switch-off table, when request is called with each, then each returns an unsupported-method error. |
| <code>acceptance.8.543.1</code> | test | tools/platform/cli/tests/lx_switch_off.rs::request_refuses_every_switched_off_name (planned) |
| <code>acceptance.8.543.1</code> | qualification | UNRUN |
| <code>req.8543</code> | ac_1 | request refuses each of the 26 L1-L26 lx_ names without sending anything. |
| <code>acceptance.8.543.2</code> | req | 8543.2 |
| <code>acceptance.8.543.2</code> | scenario | Given tools/platform/cli/tests/fixtures/openrpc.json, when method names are read, then none starts with lx_. |
| <code>acceptance.8.543.2</code> | test | tools/platform/cli/tests/lx_switch_off.rs::openrpc_fixture_lists_no_lx_method (planned) |
| <code>acceptance.8.543.2</code> | qualification | UNRUN |
| <code>req.8543</code> | ac_2 | The CLI OpenRPC fixture lists no method starting lx_. |
| <code>acceptance.8.543.3</code> | req | 8543.3 |
| <code>acceptance.8.543.3</code> | scenario | Given a stub answering -32601 for px_getBalances, when wallet balance runs, then the command fails and the message names px_getBalances and -32601. |
| <code>acceptance.8.543.3</code> | test | tools/platform/cli/tests/lx_switch_off.rs::server_minus_32601_is_reported_as_switched_off (planned) |
| <code>acceptance.8.543.3</code> | qualification | UNRUN |
| <code>req.8543</code> | ac_3 | A -32601 answer from the endpoint is reported as method not found with the method name. |
| <code>capability.8.50.4</code> | title | layerx CLI refuses every lx_ method and drops the kernel program-events and activity paths |
| <code>capability.8.50.4</code> | reqs | 8543.1<br>8543.2<br>8543.3 |
| <code>resolution.17.832</code> | title | decision.845 binds: Already landed CLI register and derive bind are not re-specified |
| <code>resolution.17.832</code> | applies_to | decision.845<br>task.8.50.4 |
| <code>resolution.17.832</code> | source | decision.845 in fragments/J42/decision.kvx; tasks from affects |

## cli-rust-pub-items

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.rs-cli-http-client-j42</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>tools/platform/cli/src/http.rs:15</code> | <code>pub struct Client</code> |
| <code>interface.8.rs-cli-http-delete-j42</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>tools/platform/cli/src/http.rs:263</code> | <code>pub fn delete(&amp;self, path: &amp;str) -&gt; Result&lt;Value, String&gt;</code> |
| <code>interface.8.rs-cli-http-get-j42</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>tools/platform/cli/src/http.rs:80</code> | <code>pub fn get(&amp;self, path: &amp;str) -&gt; Result&lt;Value, String&gt;</code> |
| <code>interface.8.rs-cli-http-get-with-body-j42</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>tools/platform/cli/src/http.rs:95</code> | <code>pub fn get_with_body(&amp;self, path: &amp;str, selector: &amp;Value) -&gt; Result&lt;Value, String&gt;</code> |
| <code>interface.8.rs-cli-http-new-gateway-j42</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>tools/platform/cli/src/http.rs:61</code> | <code>pub fn new_gateway(endpoint: &amp;str, credential: Zeroizing&lt;String&gt;) -&gt; Result&lt;Self, String&gt;</code> |
| <code>interface.8.rs-cli-http-new-j42</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>tools/platform/cli/src/http.rs:24</code> | <code>pub fn new(endpoint: &amp;str, token: Option&lt;Zeroizing&lt;String&gt;&gt;) -&gt; Result&lt;Self, String&gt;</code> |
| <code>interface.8.rs-cli-http-post-activity-j42</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>tools/platform/cli/src/http.rs:197</code> | <code>pub fn post_activity(</code> |
| <code>interface.8.rs-cli-http-post-j42</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>tools/platform/cli/src/http.rs:119</code> | <code>pub fn post(</code> |
| <code>interface.8.rs-cli-http-post-publication-j42</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>tools/platform/cli/src/http.rs:145</code> | <code>pub fn post_publication(</code> |
| <code>interface.8.rs-cli-http-post-sensitive-j42</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>tools/platform/cli/src/http.rs:234</code> | <code>pub fn post_sensitive&lt;T: DeserializeOwned&gt;(</code> |
| <code>interface.8.rs-cli-http-post-stateful-j42</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>tools/platform/cli/src/http.rs:171</code> | <code>pub fn post_stateful(</code> |
| <code>interface.8.rs-cli-http-validate-idempotency-key-j42</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>tools/platform/cli/src/http.rs:500</code> | <code>pub fn validate_idempotency_key(value: &amp;str) -&gt; Result&lt;(), String&gt;</code> |
| <code>interface.8.rs-cli-http-validate-resource-id-j42</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>tools/platform/cli/src/http.rs:484</code> | <code>pub fn validate_resource_id(value: &amp;str, name: &amp;str) -&gt; Result&lt;(), String&gt;</code> |
| <code>interface.8.rs-cli-rpc-call-j42</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>tools/platform/cli/src/rpc.rs:51</code> | <code>pub fn call(&amp;self, method: &amp;str, params: &amp;Value) -&gt; Result&lt;Value, String&gt;</code> |
| <code>interface.8.rs-cli-rpc-decode-response-j42</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>tools/platform/cli/src/rpc.rs:330</code> | <code>pub fn decode_response(method: &amp;str, response: &amp;Value) -&gt; Result&lt;Value, String&gt;</code> |
| <code>interface.8.rs-cli-rpc-new-j42</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>tools/platform/cli/src/rpc.rs:20</code> | <code>pub fn new(url: &amp;str, credential: Option&lt;zeroize::Zeroizing&lt;String&gt;&gt;) -&gt; Result&lt;Self, String&gt;</code> |
| <code>interface.8.rs-cli-rpc-new-session-j42</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>tools/platform/cli/src/rpc.rs:36</code> | <code>pub fn new_session(url: &amp;str, session: zeroize::Zeroizing&lt;String&gt;) -&gt; Result&lt;Self, String&gt;</code> |
| <code>interface.8.rs-cli-rpc-program-events-page-j42</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>tools/platform/cli/src/rpc.rs:206</code> | <code>pub fn program_events_page(params: &amp;Value, result: &amp;Value) -&gt; Result&lt;(), String&gt;</code> |
| <code>interface.8.rs-cli-rpc-request-j42</code> | <code>rust_pub_item</code> | <code>change</code> | <code>tools/platform/cli/src/rpc.rs:84</code> | <code>pub fn request(method: &amp;str, params: &amp;Value) -&gt; Result&lt;Value, String&gt;</code> |
| <code>interface.8.rs-cli-rpc-rpcclient-j42</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>tools/platform/cli/src/rpc.rs:10</code> | <code>pub struct RpcClient</code> |
| <code>interface.8.rs-cli-rpc-subscribe-j42</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>tools/platform/cli/src/rpc.rs:73</code> | <code>pub fn subscribe(&amp;self, params: &amp;Value, timeout: std::time::Duration) -&gt; Result&lt;Value, String&gt;</code> |
| <code>interface.8.rs-cli-wallet-encoding-assetoperation-j42</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet_encoding.rs:20</code> | <code>pub enum AssetOperation&lt;'a&gt;</code> |
| <code>interface.8.rs-cli-wallet-encoding-assetregistration-j42</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet_encoding.rs:10</code> | <code>pub struct AssetRegistration&lt;'a&gt;</code> |
| <code>interface.8.rs-cli-wallet-encoding-encode-j42</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet_encoding.rs:57</code> | <code>pub fn encode(&amp;self) -&gt; Result&lt;Vec&lt;u8&gt;, String&gt;</code> |
| <code>interface.8.rs-cli-wallet-encoding-native-issuer-id-j42</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet_encoding.rs:111</code> | <code>pub fn native_issuer_id(did: &amp;str) -&gt; Result&lt;[u8; 32], String&gt;</code> |
| <code>interface.8.rs-cli-wallet-encoding-ordinal-j42</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet_encoding.rs:43</code> | <code>pub const fn ordinal(&amp;self) -&gt; u16</code> |
| <code>interface.8.rs-cli-wallet-send-authorization-j42</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet_send.rs:21</code> | <code>pub struct Authorization</code> |
| <code>interface.8.rs-cli-wallet-send-authorization-message-j42</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet_send.rs:43</code> | <code>pub fn authorization_message(&amp;self) -&gt; Result&lt;Vec&lt;u8&gt;, String&gt;</code> |
| <code>interface.8.rs-cli-wallet-send-disclosure-j42</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet_send.rs:108</code> | <code>pub fn disclosure(&amp;self) -&gt; Value</code> |
| <code>interface.8.rs-cli-wallet-send-encode-j42</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet_send.rs:34</code> | <code>pub fn encode(&amp;self) -&gt; Result&lt;Vec&lt;u8&gt;, String&gt;</code> |
| <code>interface.8.rs-cli-wallet-send-prepare-j42</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet_send.rs:94</code> | <code>pub fn prepare(&amp;self, facts: &amp;SigningFacts&lt;'_&gt;) -&gt; Result&lt;PreparedPayment, String&gt;</code> |
| <code>interface.8.rs-cli-wallet-send-send-j42</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet_send.rs:7</code> | <code>pub struct Send</code> |
| <code>interface.8.rs-cli-wallet-signing-canonical-unsigned-j42</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet_signing.rs:103</code> | <code>pub fn canonical_unsigned(&amp;self) -&gt; &amp;[u8]</code> |
| <code>interface.8.rs-cli-wallet-signing-confirmation-j42</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet_signing.rs:108</code> | <code>pub fn confirmation(&amp;self) -&gt; Value</code> |
| <code>interface.8.rs-cli-wallet-signing-from-send-j42</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet_signing.rs:48</code> | <code>pub fn from_send(payload: &amp;[u8], facts: &amp;SigningFacts&lt;'_&gt;) -&gt; Result&lt;Self, String&gt;</code> |
| <code>interface.8.rs-cli-wallet-signing-new-j42</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet_signing.rs:39</code> | <code>pub fn new(payment: &amp;Payment, facts: &amp;SigningFacts&lt;'_&gt;) -&gt; Result&lt;Self, String&gt;</code> |
| <code>interface.8.rs-cli-wallet-signing-preparedpayment-j42</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet_signing.rs:25</code> | <code>pub struct PreparedPayment</code> |
| <code>interface.8.rs-cli-wallet-signing-sign-j42</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet_signing.rs:150</code> | <code>pub fn sign(self, signer: &amp;dyn Signer) -&gt; Result&lt;Vec&lt;u8&gt;, String&gt;</code> |
| <code>interface.8.rs-cli-wallet-signing-sign-with-id-j42</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet_signing.rs:156</code> | <code>pub fn sign_with_id(self, signer: &amp;dyn Signer) -&gt; Result&lt;(Vec&lt;u8&gt;, [u8; 32]), String&gt;</code> |
| <code>interface.8.rs-cli-wallet-signing-signingfacts-j42</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>tools/platform/cli/src/wallet_signing.rs:14</code> | <code>pub struct SigningFacts&lt;'a&gt;</code> |

## emulator-cli-commands

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.cli-emulator-layerx-emulator-up-listen</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/emulator/src/main.rs:3610</code> | <code>"--listen" =&gt; {</code> |
| <code>interface.8.cli-emulator-layerx-emulator-up-network-id</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/emulator/src/main.rs:3619</code> | <code>"--network-id" =&gt; {</code> |
| <code>interface.8.cli-emulator-layerx-emulator-up-parse-config</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/emulator/src/main.rs:3594</code> | <code>if arguments.next().as_deref() != Some("up") {</code> |
| <code>interface.8.cli-emulator-layerx-emulator-up-prefund</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/emulator/src/main.rs:3649</code> | <code>"--prefund" =&gt; config.prefunds.push(parse_prefund(&amp;value)?),</code> |
| <code>interface.8.cli-emulator-layerx-emulator-up-protocol-version</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/emulator/src/main.rs:3622</code> | <code>"--protocol-version" =&gt; {</code> |
| <code>interface.8.cli-emulator-layerx-emulator-up-sequencer-seed-file</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/emulator/src/main.rs:3632</code> | <code>"--sequencer-seed-file" =&gt; {</code> |
| <code>interface.8.cli-emulator-layerx-emulator-up-time-ms</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/emulator/src/main.rs:3629</code> | <code>"--time-ms" =&gt; {</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.cli-emulator-layerx-emulator-up-network-id</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-emulator-layerx-emulator-up-prefund</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-emulator-layerx-emulator-up-protocol-version</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-emulator-layerx-emulator-up-sequencer-seed-file</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-emulator-layerx-emulator-up-time-ms</code> | notes | no in-repo invocation outside the defining file |

## emulator-env-vars

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.env-emulator-ar</code> | <code>env_var</code> | <code>stable</code> | <code>tools/platform/emulator/build.rs:63</code> | <code>let mut archiver = Command::new(env::var_os("AR").unwrap_or_else(&#124;&#124; "ar".into()));</code> |
| <code>interface.8.env-emulator-cc</code> | <code>env_var</code> | <code>stable</code> | <code>tools/platform/emulator/build.rs:36</code> | <code>let mut compiler = Command::new(env::var_os("CC").unwrap_or_else(&#124;&#124; "cc".into()));</code> |
| <code>interface.8.env-emulator-opt-level</code> | <code>env_var</code> | <code>stable</code> | <code>tools/platform/emulator/build.rs:30</code> | <code>.env("OPT_LEVEL", "-O2")</code> |
| <code>interface.8.env-emulator-out-dir</code> | <code>env_var</code> | <code>stable</code> | <code>tools/platform/emulator/build.rs:22</code> | <code>let out = PathBuf::from(env::var_os("OUT_DIR").unwrap_or_else(&#124;&#124; {</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.env-emulator-ar</code> | notes | 36 callers total |
| <code>interface.8.env-emulator-cc</code> | notes | 531 callers total |
| <code>interface.8.env-emulator-out-dir</code> | notes | 92 callers total |

## emulator-error-codes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.err-emulator-core-invalid-output</code> | <code>error_code</code> | <code>stable</code> | <code>tools/platform/emulator/src/main.rs:1370</code> | <code>Err(error) =&gt; return refusal(trace, 503, "core_invalid_output", &amp;error),</code> |
| <code>interface.8.err-emulator-invalid-argument</code> | <code>error_code</code> | <code>stable</code> | <code>tools/platform/emulator/src/main.rs:1331</code> | <code>Ok(_) =&gt; return refusal(trace, 400, "invalid_argument", "activity must not be empty"),</code> |
| <code>interface.8.err-emulator-invalid-did</code> | <code>error_code</code> | <code>stable</code> | <code>tools/platform/emulator/src/main.rs:2149</code> | <code>return refusal(trace, 400, "invalid_did", "did is not a canonical DID");</code> |
| <code>interface.8.err-emulator-invalid-move-commit</code> | <code>error_code</code> | <code>stable</code> | <code>tools/platform/emulator/src/main.rs:2535</code> | <code>Err(error) =&gt; return refusal(trace, 400, "invalid_move_commit", &amp;error),</code> |
| <code>interface.8.err-emulator-invalid-move-quote</code> | <code>error_code</code> | <code>stable</code> | <code>tools/platform/emulator/src/main.rs:2380</code> | <code>Err(error) =&gt; return refusal(trace, 400, "invalid_move_quote", &amp;error),</code> |
| <code>interface.8.err-emulator-invalid-program-activity</code> | <code>error_code</code> | <code>stable</code> | <code>tools/platform/emulator/src/main.rs:2014</code> | <code>Err(error) =&gt; return refusal(trace, 400, "invalid_program_activity", &amp;error),</code> |
| <code>interface.8.err-emulator-invalid-program-activity-selector</code> | <code>error_code</code> | <code>stable</code> | <code>tools/platform/emulator/src/main.rs:3082</code> | <code>return refusal(trace, 400, "invalid_program_activity_selector", &amp;error);</code> |
| <code>interface.8.err-emulator-invalid-program-call</code> | <code>error_code</code> | <code>stable</code> | <code>tools/platform/emulator/src/main.rs:3464</code> | <code>Err(error) =&gt; return refusal(trace, 400, "invalid_program_call", &amp;error),</code> |
| <code>interface.8.err-emulator-invalid-program-receipt-selector</code> | <code>error_code</code> | <code>stable</code> | <code>tools/platform/emulator/src/main.rs:3043</code> | <code>Err(error) =&gt; return refusal(trace, 400, "invalid_program_receipt_selector", &amp;error),</code> |
| <code>interface.8.err-emulator-invalid-program-selector</code> | <code>error_code</code> | <code>stable</code> | <code>tools/platform/emulator/src/main.rs:3395</code> | <code>return refusal(trace, 400, "invalid_program_selector", &amp;error);</code> |
| <code>interface.8.err-emulator-invalid-program-success</code> | <code>error_code</code> | <code>stable</code> | <code>tools/platform/emulator/src/main.rs:1150</code> | <code>"reason": "invalid_program_success",</code> |
| <code>interface.8.err-emulator-invalid-program-u64</code> | <code>error_code</code> | <code>stable</code> | <code>tools/platform/emulator/src/main.rs:1168</code> | <code>"reason": "invalid_program_u64",</code> |
| <code>interface.8.err-emulator-invalid-request</code> | <code>error_code</code> | <code>stable</code> | <code>tools/platform/emulator/src/main.rs:4936</code> | <code>refusal(trace, 400, "invalid_request", &amp;error)</code> |
| <code>interface.8.err-emulator-invalid-snapshot</code> | <code>error_code</code> | <code>stable</code> | <code>tools/platform/emulator/src/main.rs:2929</code> | <code>Err(error) =&gt; return refusal(trace, 400, "invalid_snapshot", &amp;error),</code> |
| <code>interface.8.err-emulator-lxp-err</code> | <code>error_code</code> | <code>stable</code> | <code>tools/platform/emulator/src/main.rs:901</code> | <code>} else if code.starts_with("LXP_ERR_") {</code> |
| <code>interface.8.err-emulator-lxp-err-unknown</code> | <code>error_code</code> | <code>stable</code> | <code>tools/platform/emulator/src/main.rs:821</code> | <code>return "LXP_ERR_UNKNOWN".into();</code> |
| <code>interface.8.err-emulator-move-encoding-failed</code> | <code>error_code</code> | <code>stable</code> | <code>tools/platform/emulator/src/main.rs:5932</code> | <code>Err(error) =&gt; return refusal(trace, 503, "move_encoding_failed", &amp;error),</code> |
| <code>interface.8.err-emulator-not-found</code> | <code>error_code</code> | <code>stable</code> | <code>tools/platform/emulator/src/main.rs:2146</code> | <code>return refusal(trace, 404, "not_found", "route does not exist");</code> |
| <code>interface.8.err-emulator-receipt-pending</code> | <code>error_code</code> | <code>stable</code> | <code>tools/platform/emulator/src/main.rs:937</code> | <code>"reason": "receipt_pending",</code> |
| <code>interface.8.err-emulator-server-side-receipt-verification-only</code> | <code>error_code</code> | <code>stable</code> | <code>tools/platform/emulator/src/main.rs:964</code> | <code>"reason": "server_side_receipt_verification_only",</code> |
| <code>interface.8.err-emulator-snapshot-encoding-failed</code> | <code>error_code</code> | <code>stable</code> | <code>tools/platform/emulator/src/main.rs:2922</code> | <code>Err(error) =&gt; refusal(trace, 503, "snapshot_encoding_failed", &amp;error),</code> |
| <code>interface.8.err-emulator-unknown-program</code> | <code>error_code</code> | <code>stable</code> | <code>tools/platform/emulator/src/main.rs:1771</code> | <code>refusal(trace, 404, "unknown_program", "program is not registered")</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.err-emulator-core-invalid-output</code> | notes | HTTP 503; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-emulator-invalid-argument</code> | notes | HTTP 400; 64 callers total |
| <code>interface.8.err-emulator-invalid-did</code> | notes | HTTP 400 |
| <code>interface.8.err-emulator-invalid-move-commit</code> | notes | HTTP 400; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-emulator-invalid-move-quote</code> | notes | HTTP 400; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-emulator-invalid-program-activity</code> | notes | HTTP 400; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-emulator-invalid-program-activity-selector</code> | notes | HTTP 400 |
| <code>interface.8.err-emulator-invalid-program-call</code> | notes | HTTP 400 |
| <code>interface.8.err-emulator-invalid-program-receipt-selector</code> | notes | HTTP 400 |
| <code>interface.8.err-emulator-invalid-program-selector</code> | notes | HTTP 400 |
| <code>interface.8.err-emulator-invalid-request</code> | notes | HTTP 400; 78 callers total |
| <code>interface.8.err-emulator-invalid-snapshot</code> | notes | HTTP 400; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-emulator-move-encoding-failed</code> | notes | HTTP 503; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-emulator-not-found</code> | notes | HTTP 404; 132 callers total |
| <code>interface.8.err-emulator-snapshot-encoding-failed</code> | notes | HTTP 503; no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-emulator-unknown-program</code> | notes | HTTP 404 |

## emulator-file-formats

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.fmt-emulator-recovery-snapshot</code> | <code>file_format</code> | <code>stable</code> | <code>tools/platform/emulator/src/main.rs:62</code> | <code>const RECOVERY_SNAPSHOT_MAGIC: &amp;[u8; 8] = b"LXEMR001";</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.fmt-emulator-recovery-snapshot</code> | notes | emulator recovery snapshot magic (RECOVERY_SNAPSHOT_VERSION 4); read and written only by the defining module |

## emulator-http-routes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.http-emulator-any-v1-programs-wind-down</code> | <code>http_route</code> | <code>stable</code> | <code>tools/platform/emulator/src/main.rs:858</code> | <code>&#124; "/v1/programs/wind-down",</code> |
| <code>interface.8.http-emulator-get-emulator-snapshot</code> | <code>http_route</code> | <code>stable</code> | <code>tools/platform/emulator/src/main.rs:3317</code> | <code>("GET", "/__emulator/snapshot") =&gt; export_snapshot(emulator, trace),</code> |
| <code>interface.8.http-emulator-get-healthz</code> | <code>http_route</code> | <code>stable</code> | <code>tools/platform/emulator/src/main.rs:3278</code> | <code>("GET", "/healthz") =&gt; health(emulator, trace),</code> |
| <code>interface.8.http-emulator-get-sequence</code> | <code>http_route</code> | <code>stable</code> | <code>tools/platform/emulator/src/main.rs:3305</code> | <code>("GET", path) if path.starts_with("/v1/dids/") &amp;&amp; path.ends_with("/sequence") =&gt; {</code> |
| <code>interface.8.http-emulator-get-v1-dids</code> | <code>http_route</code> | <code>stable</code> | <code>tools/platform/emulator/src/main.rs:3305</code> | <code>("GET", path) if path.starts_with("/v1/dids/") &amp;&amp; path.ends_with("/sequence") =&gt; {</code> |
| <code>interface.8.http-emulator-get-v1-evidence</code> | <code>http_route</code> | <code>stable</code> | <code>tools/platform/emulator/src/main.rs:3319</code> | <code>("GET", path) if path.starts_with("/v1/evidence/") =&gt; {</code> |
| <code>interface.8.http-emulator-get-v1-journeys</code> | <code>http_route</code> | <code>stable</code> | <code>tools/platform/emulator/src/main.rs:3322</code> | <code>("GET", path) if path.starts_with("/v1/journeys/") =&gt; {</code> |
| <code>interface.8.http-emulator-get-v1-programs-activities</code> | <code>http_route</code> | <code>stable</code> | <code>tools/platform/emulator/src/main.rs:867</code> | <code>("GET", path) if path.starts_with("/v1/programs/activities/") =&gt; path</code> |
| <code>interface.8.http-emulator-get-v1-programs-receipts-by-idempotency</code> | <code>http_route</code> | <code>stable</code> | <code>tools/platform/emulator/src/main.rs:864</code> | <code>("GET", path) if path.starts_with("/v1/programs/receipts/by-idempotency/") =&gt; path</code> |
| <code>interface.8.http-emulator-get-v1-programs-registry</code> | <code>http_route</code> | <code>stable</code> | <code>tools/platform/emulator/src/main.rs:860</code> | <code>("GET", path) if path.starts_with("/v1/programs/registry/") =&gt; {</code> |
| <code>interface.8.http-emulator-get-v1-protocol-account-state-head</code> | <code>http_route</code> | <code>stable</code> | <code>tools/platform/emulator/src/main.rs:3280</code> | <code>("GET", "/v1/protocol/account-state/head") =&gt; current_head(emulator, trace),</code> |
| <code>interface.8.http-emulator-get-v1-receipts</code> | <code>http_route</code> | <code>stable</code> | <code>tools/platform/emulator/src/main.rs:3325</code> | <code>("GET", path) if path.starts_with("/v1/receipts/") =&gt; {</code> |
| <code>interface.8.http-emulator-get-v1-sequencer</code> | <code>http_route</code> | <code>stable</code> | <code>tools/platform/emulator/src/main.rs:3279</code> | <code>("GET", "/v1/sequencer") =&gt; sequencer_identity(emulator, trace),</code> |
| <code>interface.8.http-emulator-get-v1-state</code> | <code>http_route</code> | <code>stable</code> | <code>tools/platform/emulator/src/main.rs:3304</code> | <code>("GET", "/v1/state") =&gt; inspect(emulator, trace),</code> |
| <code>interface.8.http-emulator-post-emulator-accounts-prefund</code> | <code>http_route</code> | <code>stable</code> | <code>tools/platform/emulator/src/main.rs:3313</code> | <code>("POST", "/__emulator/accounts/prefund") =&gt; prefund(emulator, request, trace),</code> |
| <code>interface.8.http-emulator-post-emulator-faults</code> | <code>http_route</code> | <code>stable</code> | <code>tools/platform/emulator/src/main.rs:3316</code> | <code>("POST", "/__emulator/faults") =&gt; inject_fault(emulator, request, trace),</code> |
| <code>interface.8.http-emulator-post-emulator-time-advance</code> | <code>http_route</code> | <code>stable</code> | <code>tools/platform/emulator/src/main.rs:3315</code> | <code>("POST", "/__emulator/time/advance") =&gt; update_time(emulator, request, trace, true),</code> |
| <code>interface.8.http-emulator-post-emulator-time-set</code> | <code>http_route</code> | <code>stable</code> | <code>tools/platform/emulator/src/main.rs:3314</code> | <code>("POST", "/__emulator/time/set") =&gt; update_time(emulator, request, trace, false),</code> |
| <code>interface.8.http-emulator-post-v1-activities</code> | <code>http_route</code> | <code>stable</code> | <code>tools/platform/emulator/src/main.rs:3281</code> | <code>("POST", "/v1/activities") =&gt; submit(emulator, request, trace),</code> |
| <code>interface.8.http-emulator-post-v1-moves</code> | <code>http_route</code> | <code>stable</code> | <code>tools/platform/emulator/src/main.rs:3284</code> | <code>("POST", "/v1/moves") =&gt; move_commit(emulator, request, trace),</code> |
| <code>interface.8.http-emulator-post-v1-moves-quote</code> | <code>http_route</code> | <code>stable</code> | <code>tools/platform/emulator/src/main.rs:3283</code> | <code>("POST", "/v1/moves/quote") =&gt; move_quote(emulator, request, trace),</code> |
| <code>interface.8.http-emulator-post-v1-programs-call</code> | <code>http_route</code> | <code>stable</code> | <code>tools/platform/emulator/src/main.rs:3361</code> | <code>&amp;&amp; (request.path == "/v1/programs/call"</code> |
| <code>interface.8.http-emulator-post-v1-programs-deploy</code> | <code>http_route</code> | <code>stable</code> | <code>tools/platform/emulator/src/main.rs:856</code> | <code>&#124; "/v1/programs/deploy"</code> |
| <code>interface.8.http-emulator-post-v1-programs-simulate</code> | <code>http_route</code> | <code>stable</code> | <code>tools/platform/emulator/src/main.rs:855</code> | <code>&#124; "/v1/programs/simulate"</code> |
| <code>interface.8.http-emulator-post-v1-programs-upgrade</code> | <code>http_route</code> | <code>stable</code> | <code>tools/platform/emulator/src/main.rs:3289</code> | <code>&#124; "/v1/programs/upgrade"</code> |
| <code>interface.8.http-emulator-post-v1-settle</code> | <code>http_route</code> | <code>stable</code> | <code>tools/platform/emulator/src/main.rs:3282</code> | <code>("POST", "/v1/settle") =&gt; settle(emulator, request, trace),</code> |
| <code>interface.8.http-emulator-put-emulator-snapshot</code> | <code>http_route</code> | <code>stable</code> | <code>tools/platform/emulator/src/main.rs:3318</code> | <code>("PUT", "/__emulator/snapshot") =&gt; import_snapshot(emulator, &amp;request.body, trace),</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.http-emulator-any-v1-programs-wind-down</code> | notes | 56 callers total |
| <code>interface.8.http-emulator-get-healthz</code> | notes | 331 callers total |
| <code>interface.8.http-emulator-get-sequence</code> | notes | 33 callers total |
| <code>interface.8.http-emulator-get-v1-journeys</code> | notes | 57 callers total |
| <code>interface.8.http-emulator-get-v1-programs-activities</code> | notes | 51 callers total |
| <code>interface.8.http-emulator-get-v1-programs-receipts-by-idempotency</code> | notes | 39 callers total |
| <code>interface.8.http-emulator-get-v1-programs-registry</code> | notes | 94 callers total |
| <code>interface.8.http-emulator-get-v1-protocol-account-state-head</code> | notes | 27 callers total |
| <code>interface.8.http-emulator-get-v1-receipts</code> | notes | 58 callers total |
| <code>interface.8.http-emulator-get-v1-state</code> | notes | 36 callers total |
| <code>interface.8.http-emulator-post-v1-activities</code> | notes | 103 callers total |
| <code>interface.8.http-emulator-post-v1-moves</code> | notes | 28 callers total |
| <code>interface.8.http-emulator-post-v1-programs-call</code> | notes | 72 callers total |
| <code>interface.8.http-emulator-post-v1-programs-deploy</code> | notes | 61 callers total |
| <code>interface.8.http-emulator-post-v1-programs-simulate</code> | notes | 46 callers total |
| <code>interface.8.http-emulator-post-v1-programs-upgrade</code> | notes | 51 callers total |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.8.544.1</code> | req | 8544.1 |
| <code>acceptance.8.544.1</code> | scenario | Given the built liblayerx_emulator_core.a, when its undefined and defined symbols are listed, then none of the four names appears. |
| <code>acceptance.8.544.1</code> | test | tools/platform/emulator/tests/core_linkage.rs::emulator_core_builds_without_batch_and_history_symbols (planned) |
| <code>acceptance.8.544.1</code> | qualification | UNRUN |
| <code>req.8544</code> | ac_1 | The emulator core archive defines and references none of lxp_batch_seal, lxp_batch_sign, lxp_history_open or lxp_history_close. |
| <code>acceptance.8.544.2</code> | req | 8544.2 |
| <code>acceptance.8.544.2</code> | scenario | Given a fresh emulator with one prefunded account, when a transfer runs and the head is read, then the head digest signature verifies with the emulator public key and the proof root matches. |
| <code>acceptance.8.544.2</code> | test | tools/platform/emulator/tests/core_linkage.rs::emulator_head_proof_still_verifies (planned) |
| <code>acceptance.8.544.2</code> | qualification | UNRUN |
| <code>req.8544</code> | ac_2 | After a prefunded transfer the emulator head proof verifies against the emulator's public key. |
| <code>capability.8.51.1</code> | title | Emulator core stops calling the retired batch seal, batch sign and history functions |
| <code>capability.8.51.1</code> | reqs | 8544.1<br>8544.2 |

## emulator-rust-pub-items

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.rs-emulator-run-j42</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>tools/platform/emulator/src/main.rs:3716</code> | <code>pub fn run(arguments: impl IntoIterator&lt;Item = String&gt;) -&gt; Result&lt;(), String&gt;</code> |

## release-cli-commands

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.cli-release-layerx-platform-release</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/release/src/main.rs:2429</code> | <code>fn run() -&gt; Result&lt;(), String&gt; {</code> |
| <code>interface.8.cli-release-layerx-platform-release-check</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/release/src/main.rs:2330</code> | <code>if arguments.len() &gt; 3 &#124;&#124; !matches!(mode, "--check" &#124; "--plan") {</code> |
| <code>interface.8.cli-release-layerx-platform-release-manifest</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/release/src/main.rs:2432</code> | <code>Some("manifest") =&gt; run_manifest(&amp;arguments[1..]),</code> |
| <code>interface.8.cli-release-layerx-platform-release-manifest-output</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/release/src/main.rs:2345</code> | <code>"--output",</code> |
| <code>interface.8.cli-release-layerx-platform-release-manifest-registries</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/release/src/main.rs:2347</code> | <code>"--registries",</code> |
| <code>interface.8.cli-release-layerx-platform-release-manifest-release-dir</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/release/src/main.rs:2344</code> | <code>"--release-dir",</code> |
| <code>interface.8.cli-release-layerx-platform-release-manifest-rollback-version</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/release/src/main.rs:2346</code> | <code>"--rollback-version",</code> |
| <code>interface.8.cli-release-layerx-platform-release-manifest-workflow</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/release/src/main.rs:2348</code> | <code>"--workflow",</code> |
| <code>interface.8.cli-release-layerx-platform-release-plan</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/release/src/main.rs:2330</code> | <code>if arguments.len() &gt; 3 &#124;&#124; !matches!(mode, "--check" &#124; "--plan") {</code> |
| <code>interface.8.cli-release-layerx-platform-release-verify</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/release/src/main.rs:2433</code> | <code>Some("verify") =&gt; run_verify(&amp;arguments[1..]),</code> |
| <code>interface.8.cli-release-layerx-platform-release-verify-fetch</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/release/src/main.rs:23</code> | <code>const FETCH_FLAG: &amp;str = "--fetch";</code> |
| <code>interface.8.cli-release-layerx-platform-release-verify-from</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/release/src/main.rs:2385</code> | <code>&amp;["--manifest", "--from", FETCH_FLAG, "--source-revision"],</code> |
| <code>interface.8.cli-release-layerx-platform-release-verify-manifest</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/release/src/main.rs:2385</code> | <code>&amp;["--manifest", "--from", FETCH_FLAG, "--source-revision"],</code> |
| <code>interface.8.cli-release-layerx-platform-release-verify-source-revision</code> | <code>cli_command</code> | <code>stable</code> | <code>tools/platform/release/src/main.rs:2385</code> | <code>&amp;["--manifest", "--from", FETCH_FLAG, "--source-revision"],</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.cli-release-layerx-platform-release</code> | notes | default mode runs the check |
| <code>interface.8.cli-release-layerx-platform-release-manifest</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-release-layerx-platform-release-manifest-output</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-release-layerx-platform-release-manifest-registries</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-release-layerx-platform-release-manifest-release-dir</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-release-layerx-platform-release-manifest-rollback-version</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-release-layerx-platform-release-manifest-workflow</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-release-layerx-platform-release-verify</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-release-layerx-platform-release-verify-fetch</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-release-layerx-platform-release-verify-from</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-release-layerx-platform-release-verify-manifest</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-release-layerx-platform-release-verify-source-revision</code> | notes | no in-repo invocation outside the defining file |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.8.545.1</code> | req | 8545.1 |
| <code>acceptance.8.545.1</code> | scenario | Given the release pipeline description, when its gates and the release-pipeline needs are listed, then no entry is replay-matrix or make test-replay-golden. |
| <code>acceptance.8.545.1</code> | test | tools/platform/release/tests/registries.rs::release_pipeline_has_no_replay_gate (planned) |
| <code>acceptance.8.545.1</code> | qualification | UNRUN |
| <code>req.8545</code> | ac_1 | The release pipeline gates contain no replay-matrix job and the release-pipeline needs list omits it. |
| <code>capability.8.51.2</code> | title | Release pipeline drops the replay-matrix gate |
| <code>capability.8.51.2</code> | reqs | 8545.1 |

## release-config-keys

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.cfg-release-artifactmanifest-artifacts</code> | <code>config_key</code> | <code>stable</code> | <code>tools/platform/release/src/main.rs:311</code> | <code>pub artifacts: Vec&lt;ArtifactManifestEntry&gt;,</code> |
| <code>interface.8.cfg-release-artifactmanifest-images</code> | <code>config_key</code> | <code>stable</code> | <code>tools/platform/release/src/main.rs:312</code> | <code>pub images: Vec&lt;ImageManifestEntry&gt;,</code> |
| <code>interface.8.cfg-release-artifactmanifest-rollback-version</code> | <code>config_key</code> | <code>stable</code> | <code>tools/platform/release/src/main.rs:310</code> | <code>pub rollback_version: Option&lt;String&gt;,</code> |
| <code>interface.8.cfg-release-artifactmanifest-schema</code> | <code>config_key</code> | <code>stable</code> | <code>tools/platform/release/src/main.rs:305</code> | <code>pub schema: String,</code> |
| <code>interface.8.cfg-release-artifactmanifest-source-digest</code> | <code>config_key</code> | <code>stable</code> | <code>tools/platform/release/src/main.rs:309</code> | <code>pub source_digest: String,</code> |
| <code>interface.8.cfg-release-artifactmanifest-source-revision</code> | <code>config_key</code> | <code>stable</code> | <code>tools/platform/release/src/main.rs:308</code> | <code>pub source_revision: String,</code> |
| <code>interface.8.cfg-release-artifactmanifest-tag</code> | <code>config_key</code> | <code>stable</code> | <code>tools/platform/release/src/main.rs:307</code> | <code>pub tag: String,</code> |
| <code>interface.8.cfg-release-artifactmanifest-version</code> | <code>config_key</code> | <code>stable</code> | <code>tools/platform/release/src/main.rs:306</code> | <code>pub version: String,</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.cfg-release-artifactmanifest-artifacts</code> | notes | 576 callers total |
| <code>interface.8.cfg-release-artifactmanifest-images</code> | notes | 36 callers total |
| <code>interface.8.cfg-release-artifactmanifest-rollback-version</code> | notes | no in-repo config file sets the key outside the defining file |
| <code>interface.8.cfg-release-artifactmanifest-schema</code> | notes | 2031 callers total |
| <code>interface.8.cfg-release-artifactmanifest-source-digest</code> | notes | 55 callers total |
| <code>interface.8.cfg-release-artifactmanifest-source-revision</code> | notes | 224 callers total |
| <code>interface.8.cfg-release-artifactmanifest-tag</code> | notes | 153 callers total |
| <code>interface.8.cfg-release-artifactmanifest-version</code> | notes | 5297 callers total |

## release-env-vars

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.env-release-layerx-release-distribution</code> | <code>env_var</code> | <code>stable</code> | <code>tools/platform/release/src/main.rs:56</code> | <code>const DISTRIBUTION_ENV: &amp;str = "LAYERX_RELEASE_DISTRIBUTION";</code> |
| <code>interface.8.env-release-layerx-release-packages</code> | <code>env_var</code> | <code>stable</code> | <code>tools/platform/release/src/main.rs:57</code> | <code>const PACKAGES_ENV: &amp;str = "LAYERX_RELEASE_PACKAGES";</code> |
| <code>interface.8.env-release-layerx-release-registry</code> | <code>env_var</code> | <code>stable</code> | <code>tools/platform/release/src/main.rs:55</code> | <code>const REGISTRY_ENV: &amp;str = "LAYERX_RELEASE_REGISTRY";</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.env-release-layerx-release-distribution</code> | notes | no in-repo setter or reader outside the defining file |
| <code>interface.8.env-release-layerx-release-packages</code> | notes | 7 callers total; no in-repo setter or reader outside the defining file |

## release-file-formats

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.fmt-release-artifact-manifest</code> | <code>file_format</code> | <code>stable</code> | <code>tools/platform/release/src/main.rs:18</code> | <code>const ARTIFACT_MANIFEST_SCHEMA: &amp;str = "layerx/artifact-manifest/1";</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.fmt-release-artifact-manifest</code> | notes | release artifact manifest schema id; read and written only by the defining module |
