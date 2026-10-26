# Platform middleware contract

Source revision: `2a1033aa2`. This contract freezes the cited interface and acceptance blocks of the specification. Statuses include the specification overrides. A source marked planned has line 0; a positive source line is verified in the pinned revision.

The context and decision tables reproduce the rules linked by interface identifiers. An anchor without a linked context gains no additional storage, gas, activation or pause rule here. Acceptance qualification remains UNRUN until its declared gate succeeds.

## examples-agent-spend-env-vars

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.env-examples-agent-spend-layerx-actor</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/examples/agent-spend/services.mjs:141</code> | <code>tenant: required(environment, "LAYERX_TENANT"), actor: required(environment, "LAYERX_ACTOR"),</code> |
| <code>interface.8.env-examples-agent-spend-layerx-agent-services-module</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/examples/agent-spend/index.mjs:14</code> | <code>const servicesEnvironment = process.env.LAYERX_AGENT_SERVICES_MODULE === undefined</code> |
| <code>interface.8.env-examples-agent-spend-layerx-daemon-preparation-store-path</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/examples/agent-spend/services.mjs:136</code> | <code>const preparationPath = required(environment, "LAYERX_DAEMON_PREPARATION_STORE_PATH");</code> |
| <code>interface.8.env-examples-agent-spend-layerx-daemon-receipt-store-path</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/examples/agent-spend/services.mjs:135</code> | <code>const receiptPath = required(environment, "LAYERX_DAEMON_RECEIPT_STORE_PATH");</code> |
| <code>interface.8.env-examples-agent-spend-layerx-session-id</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/examples/agent-spend/services.mjs:142</code> | <code>sessionId: required(environment, "LAYERX_SESSION_ID"), receiptPath, preparationPath });</code> |
| <code>interface.8.env-examples-agent-spend-layerx-tenant</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/examples/agent-spend/services.mjs:141</code> | <code>tenant: required(environment, "LAYERX_TENANT"), actor: required(environment, "LAYERX_ACTOR"),</code> |
| <code>interface.8.env-examples-agent-spend-layerx-wallet-api-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/examples/agent-spend/services.mjs:129</code> | <code>const apiUrl = required(environment, "LAYERX_WALLET_API_URL");</code> |
| <code>interface.8.env-examples-agent-spend-layerx-wallet-principal</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/examples/agent-spend/services.mjs:128</code> | <code>const principal = required(environment, "LAYERX_WALLET_PRINCIPAL");</code> |
| <code>interface.8.env-examples-agent-spend-layerx-wallet-session-file</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/examples/agent-spend/services.mjs:149</code> | <code>const session = saved === null ? protectedJson(required(environment, "LAYERX_WALLET_SESSION_FILE")) : JSON.parse(saved);</code> |
| <code>interface.8.env-examples-agent-spend-layerx-wallet-store-path</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/examples/agent-spend/services.mjs:137</code> | <code>const storePath = required(environment, "LAYERX_WALLET_STORE_PATH");</code> |
| <code>interface.8.env-examples-agent-spend-layerx-wallet-supabase-anon-key</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/examples/agent-spend/services.mjs:146</code> | <code>supabaseAnonKey: required(environment, "LAYERX_WALLET_SUPABASE_ANON_KEY"),</code> |
| <code>interface.8.env-examples-agent-spend-layerx-wallet-supabase-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/examples/agent-spend/services.mjs:130</code> | <code>const supabaseUrl = required(environment, "LAYERX_WALLET_SUPABASE_URL");</code> |
| <code>interface.8.env-examples-layerx-spend-request-json-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/examples/agent-spend/index.mjs:35</code> | <code>const request = JSON.parse(required("LAYERX_SPEND_REQUEST_JSON"));</code> |
| <code>interface.8.env-examples-layerx-wallet-preparation-ref-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/examples/agent-spend/index.mjs:30</code> | <code>const preparationRef = required("LAYERX_WALLET_PREPARATION_REF");</code> |
| <code>interface.8.env-examples-layerx-wallet-review-id-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/examples/agent-spend/index.mjs:31</code> | <code>const reviewId = required("LAYERX_WALLET_REVIEW_ID");</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.env-examples-agent-spend-layerx-daemon-preparation-store-path</code> | notes | no in-repo setter or reader outside the defining file |
| <code>interface.8.env-examples-agent-spend-layerx-daemon-receipt-store-path</code> | notes | no in-repo setter or reader outside the defining file |
| <code>interface.8.env-examples-agent-spend-layerx-wallet-api-url</code> | notes | no in-repo setter or reader outside the defining file |
| <code>interface.8.env-examples-agent-spend-layerx-wallet-principal</code> | notes | no in-repo setter or reader outside the defining file |
| <code>interface.8.env-examples-agent-spend-layerx-wallet-session-file</code> | notes | no in-repo setter or reader outside the defining file |
| <code>interface.8.env-examples-agent-spend-layerx-wallet-store-path</code> | notes | no in-repo setter or reader outside the defining file |
| <code>interface.8.env-examples-agent-spend-layerx-wallet-supabase-anon-key</code> | notes | no in-repo setter or reader outside the defining file |
| <code>interface.8.env-examples-agent-spend-layerx-wallet-supabase-url</code> | notes | no in-repo setter or reader outside the defining file |
| <code>interface.8.env-examples-layerx-spend-request-json-j69</code> | notes | read at services/platform/examples/agent-spend/index.mjs:35; 9 in-repo references, first 8 listed |
| <code>interface.8.env-examples-layerx-wallet-preparation-ref-j69</code> | notes | read at services/platform/examples/agent-spend/index.mjs:30 |
| <code>interface.8.env-examples-layerx-wallet-review-id-j69</code> | notes | read at services/platform/examples/agent-spend/index.mjs:31 |

## examples-buyer-agent-config-keys

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.cfg-examples-buyer-agent-application</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/buyer-agent/layerx.example.json:3</code> | <code>"application": "buyer-agent",</code> |
| <code>interface.8.cfg-examples-buyer-agent-environments</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/buyer-agent/layerx.example.json:4</code> | <code>"environments": {</code> |
| <code>interface.8.cfg-examples-buyer-agent-environments-beta</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/buyer-agent/layerx.example.json:16</code> | <code>"beta": {</code> |
| <code>interface.8.cfg-examples-buyer-agent-environments-beta-humanurl</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/buyer-agent/layerx.example.json:6</code> | <code>"humanUrl": "&lt;url&gt;",</code> |
| <code>interface.8.cfg-examples-buyer-agent-environments-beta-idempotencyenvironment</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/buyer-agent/layerx.example.json:14</code> | <code>"idempotencyEnvironment": "LAYERX_EMULATOR_PAYMENT_KEY"</code> |
| <code>interface.8.cfg-examples-buyer-agent-environments-beta-network</code> | <code>config_key</code> | <code>change</code> | <code>services/platform/examples/buyer-agent/layerx.example.json:10</code> | <code>"network": "layerx:emulator",</code> |
| <code>interface.8.cfg-examples-buyer-agent-environments-beta-protocolversion</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/examples/buyer-agent/layerx.example.json:11</code> | <code>"protocolVersion": 3,</code> |
| <code>interface.8.cfg-examples-buyer-agent-environments-beta-receiptauthorityurl</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/examples/buyer-agent/layerx.example.json:7</code> | <code>"receiptAuthorityUrl": "&lt;url&gt;",</code> |
| <code>interface.8.cfg-examples-buyer-agent-environments-beta-resourceurl</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/buyer-agent/layerx.example.json:8</code> | <code>"resourceUrl": "&lt;url&gt;",</code> |
| <code>interface.8.cfg-examples-buyer-agent-environments-beta-scheme</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/buyer-agent/layerx.example.json:9</code> | <code>"scheme": "exact",</code> |
| <code>interface.8.cfg-examples-buyer-agent-environments-beta-sourceenvironment</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/buyer-agent/layerx.example.json:13</code> | <code>"sourceEnvironment": "LAYERX_EMULATOR_SOURCE",</code> |
| <code>interface.8.cfg-examples-buyer-agent-environments-beta-tokenenvironment</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/buyer-agent/layerx.example.json:12</code> | <code>"tokenEnvironment": "LAYERX_EMULATOR_TOKEN",</code> |
| <code>interface.8.cfg-examples-buyer-agent-environments-emulator</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/buyer-agent/layerx.example.json:5</code> | <code>"emulator": {</code> |
| <code>interface.8.cfg-examples-buyer-agent-environments-emulator-humanurl</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/buyer-agent/layerx.example.json:6</code> | <code>"humanUrl": "&lt;url&gt;",</code> |
| <code>interface.8.cfg-examples-buyer-agent-environments-emulator-idempotencyenvironment</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/buyer-agent/layerx.example.json:14</code> | <code>"idempotencyEnvironment": "LAYERX_EMULATOR_PAYMENT_KEY"</code> |
| <code>interface.8.cfg-examples-buyer-agent-environments-emulator-network</code> | <code>config_key</code> | <code>change</code> | <code>services/platform/examples/buyer-agent/layerx.example.json:10</code> | <code>"network": "layerx:emulator",</code> |
| <code>interface.8.cfg-examples-buyer-agent-environments-emulator-protocolversion</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/examples/buyer-agent/layerx.example.json:11</code> | <code>"protocolVersion": 3,</code> |
| <code>interface.8.cfg-examples-buyer-agent-environments-emulator-receiptauthorityurl</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/examples/buyer-agent/layerx.example.json:7</code> | <code>"receiptAuthorityUrl": "&lt;url&gt;",</code> |
| <code>interface.8.cfg-examples-buyer-agent-environments-emulator-resourceurl</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/buyer-agent/layerx.example.json:8</code> | <code>"resourceUrl": "&lt;url&gt;",</code> |
| <code>interface.8.cfg-examples-buyer-agent-environments-emulator-scheme</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/buyer-agent/layerx.example.json:9</code> | <code>"scheme": "exact",</code> |
| <code>interface.8.cfg-examples-buyer-agent-environments-emulator-sourceenvironment</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/buyer-agent/layerx.example.json:13</code> | <code>"sourceEnvironment": "LAYERX_EMULATOR_SOURCE",</code> |
| <code>interface.8.cfg-examples-buyer-agent-environments-emulator-tokenenvironment</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/buyer-agent/layerx.example.json:12</code> | <code>"tokenEnvironment": "LAYERX_EMULATOR_TOKEN",</code> |
| <code>interface.8.cfg-examples-buyer-agent-version</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/buyer-agent/layerx.example.json:2</code> | <code>"version": 1,</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.cfg-examples-buyer-agent-environments</code> | notes | 30 callers total |
| <code>interface.8.cfg-examples-buyer-agent-environments-beta</code> | notes | 68 callers total |
| <code>interface.8.cfg-examples-buyer-agent-environments-beta-network</code> | notes | 386 callers total |
| <code>interface.8.cfg-examples-buyer-agent-environments-beta-network</code> | overridden_by | override.J41.97 |
| <code>interface.8.cfg-examples-buyer-agent-environments-beta-protocolversion</code> | notes | 62 callers total |
| <code>interface.8.cfg-examples-buyer-agent-environments-beta-protocolversion</code> | overridden_by | override.J41.98 |
| <code>interface.8.cfg-examples-buyer-agent-environments-beta-receiptauthorityurl</code> | overridden_by | override.J41.99 |
| <code>interface.8.cfg-examples-buyer-agent-environments-beta-scheme</code> | notes | 245 callers total |
| <code>interface.8.cfg-examples-buyer-agent-environments-beta-sourceenvironment</code> | notes | no in-repo config file sets the key outside the defining file |
| <code>interface.8.cfg-examples-buyer-agent-environments-emulator</code> | notes | 54 callers total |
| <code>interface.8.cfg-examples-buyer-agent-environments-emulator-network</code> | notes | 386 callers total |
| <code>interface.8.cfg-examples-buyer-agent-environments-emulator-network</code> | overridden_by | override.J41.100 |
| <code>interface.8.cfg-examples-buyer-agent-environments-emulator-protocolversion</code> | notes | 62 callers total |
| <code>interface.8.cfg-examples-buyer-agent-environments-emulator-protocolversion</code> | overridden_by | override.J41.101 |
| <code>interface.8.cfg-examples-buyer-agent-environments-emulator-receiptauthorityurl</code> | overridden_by | override.J41.102 |
| <code>interface.8.cfg-examples-buyer-agent-environments-emulator-scheme</code> | notes | 245 callers total |
| <code>interface.8.cfg-examples-buyer-agent-environments-emulator-sourceenvironment</code> | notes | no in-repo config file sets the key outside the defining file |
| <code>interface.8.cfg-examples-buyer-agent-version</code> | notes | 5296 callers total |

## examples-cli-commands

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.cli-examples-run-reference-apps-mjs-check</code> | <code>cli_command</code> | <code>change</code> | <code>services/platform/examples/run-reference-apps.mjs:41</code> | <code>if (arguments_.length === 1 &amp;&amp; arguments_[0] === "--check") {</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.cli-examples-run-reference-apps-mjs-check</code> | overridden_by | override.J41.106 |

## examples-config-keys

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.cfg-examples-applications</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/reference-apps.json:3</code> | <code>"applications": [</code> |
| <code>interface.8.cfg-examples-applications-commands</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/reference-apps.json:10</code> | <code>"commands": {</code> |
| <code>interface.8.cfg-examples-applications-commands-beta</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/reference-apps.json:12</code> | <code>"beta": ["npm", "run", "start:beta", "--workspace", "@sidiora/layerx-example-buyer-agent"]</code> |
| <code>interface.8.cfg-examples-applications-commands-emulator</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/reference-apps.json:11</code> | <code>"emulator": ["npm", "run", "start:emulator", "--workspace", "@sidiora/layerx-example-buyer-agent"],</code> |
| <code>interface.8.cfg-examples-applications-config</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/reference-apps.json:8</code> | <code>"config": "layerx.example.json",</code> |
| <code>interface.8.cfg-examples-applications-name</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/reference-apps.json:5</code> | <code>"name": "buyer-agent",</code> |
| <code>interface.8.cfg-examples-applications-package</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/reference-apps.json:6</code> | <code>"package": "@sidiora/layerx-example-buyer-agent",</code> |
| <code>interface.8.cfg-examples-applications-path</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/reference-apps.json:7</code> | <code>"path": "services/platform/examples/buyer-agent",</code> |
| <code>interface.8.cfg-examples-applications-symbol</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/reference-apps.json:9</code> | <code>"symbol": "platform_ref_buyer",</code> |
| <code>interface.8.cfg-examples-version</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/reference-apps.json:2</code> | <code>"version": 1,</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.cfg-examples-applications-commands</code> | notes | 734 callers total |
| <code>interface.8.cfg-examples-applications-commands-beta</code> | notes | 65 callers total |
| <code>interface.8.cfg-examples-applications-commands-emulator</code> | notes | 51 callers total |
| <code>interface.8.cfg-examples-applications-config</code> | notes | 439 callers total |
| <code>interface.8.cfg-examples-applications-name</code> | notes | 48652 callers total |
| <code>interface.8.cfg-examples-applications-package</code> | notes | 673 callers total |
| <code>interface.8.cfg-examples-applications-path</code> | notes | 6772 callers total; no in-repo config file sets the key outside the defining file |
| <code>interface.8.cfg-examples-applications-symbol</code> | notes | 1417 callers total |
| <code>interface.8.cfg-examples-version</code> | notes | 5296 callers total |

## examples-env-vars

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.env-examples-cargo</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/examples/build-reference-apps.mjs:32</code> | <code>const compiled = await command(process.env.CARGO ?? "cargo", ["build", "--locked", "--manifest-path", "tools/platform/Cargo.toml", "-p", "layerx-platform-cli", "--bin", "layerx", "--message-format=json"]);</code> |
| <code>interface.8.env-examples-layerx-bin</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/examples/run-reference-apps.mjs:155</code> | <code>const explicit = optionalEnvironment("LAYERX_BIN");</code> |
| <code>interface.8.env-examples-layerx-credential-passphrase</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/examples/run-reference-apps.mjs:179</code> | <code>LAYERX_CREDENTIAL_PASSPHRASE: optionalEnvironment("LAYERX_CREDENTIAL_PASSPHRASE") ?? randomBytes(24).toString("hex"),</code> |
| <code>interface.8.env-examples-layerx-credential-store</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/examples/run-reference-apps.mjs:178</code> | <code>LAYERX_CREDENTIAL_STORE: optionalEnvironment("LAYERX_CREDENTIAL_STORE") ?? "file",</code> |
| <code>interface.8.env-examples-layerx-emulator-merchant-checkout-key-j69</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/examples/run-reference-apps.mjs:453</code> | <code>? "LAYERX_EMULATOR_MERCHANT_CHECKOUT_KEY"</code> |
| <code>interface.8.env-examples-layerx-emulator-price</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/examples/run-reference-apps.mjs:288</code> | <code>const price = optionalEnvironment("LAYERX_EMULATOR_PRICE") ?? EMULATOR_DEFAULT_PRICE;</code> |
| <code>interface.8.env-examples-layerx-emulator-seed-file</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/examples/run-reference-apps.mjs:181</code> | <code>const suppliedSeed = optionalEnvironment("LAYERX_EMULATOR_SEED_FILE");</code> |
| <code>interface.8.env-examples-layerx-emulator-token</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/examples/run-reference-apps.mjs:268</code> | <code>const token = optionalEnvironment("LAYERX_EMULATOR_TOKEN") ?? randomBytes(32).toString("hex");</code> |
| <code>interface.8.env-examples-paxeer-x-evidence-dir</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/examples/qualify-reference-apps.mjs:11</code> | <code>const evidence = process.env.PAXEER_X_EVIDENCE_DIR;</code> |
| <code>interface.8.env-examples-paxeer-x-reference-app-artifacts</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/examples/build-reference-apps.mjs:8</code> | <code>const output = process.env.PAXEER_X_REFERENCE_APP_ARTIFACTS;</code> |
| <code>interface.8.env-examples-paxeer-x-reference-apps-environment</code> | <code>env_var</code> | <code>change</code> | <code>services/platform/examples/qualify-reference-apps.mjs:9</code> | <code>const environment = process.env.PAXEER_X_REFERENCE_APPS_ENVIRONMENT ?? "emulator";</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.env-examples-cargo</code> | notes | system variable; callers not enumerated |
| <code>interface.8.env-examples-layerx-bin</code> | notes | 32 callers total |
| <code>interface.8.env-examples-layerx-emulator-merchant-checkout-key-j69</code> | notes | read at services/platform/examples/run-reference-apps.mjs:453 |
| <code>interface.8.env-examples-layerx-emulator-seed-file</code> | notes | no in-repo setter or reader outside the defining file |
| <code>interface.8.env-examples-paxeer-x-evidence-dir</code> | notes | 142 callers total |
| <code>interface.8.env-examples-paxeer-x-reference-apps-environment</code> | notes | no in-repo setter or reader outside the defining file |
| <code>interface.8.env-examples-paxeer-x-reference-apps-environment</code> | overridden_by | override.J41.107 |

## examples-marketplace-config-keys

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.cfg-examples-marketplace-application</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/marketplace/layerx.example.json:3</code> | <code>"application": "marketplace",</code> |
| <code>interface.8.cfg-examples-marketplace-environments</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/marketplace/layerx.example.json:4</code> | <code>"environments": {</code> |
| <code>interface.8.cfg-examples-marketplace-environments-beta</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/marketplace/layerx.example.json:20</code> | <code>"beta": {</code> |
| <code>interface.8.cfg-examples-marketplace-environments-beta-assetenvironment</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/marketplace/layerx.example.json:14</code> | <code>"assetEnvironment": "LAYERX_EMULATOR_ASSET",</code> |
| <code>interface.8.cfg-examples-marketplace-environments-beta-clibinaryenvironment</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/marketplace/layerx.example.json:9</code> | <code>"cliBinaryEnvironment": "LAYERX_BIN",</code> |
| <code>interface.8.cfg-examples-marketplace-environments-beta-endpoint</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/marketplace/layerx.example.json:6</code> | <code>"endpoint": "&lt;url&gt;",</code> |
| <code>interface.8.cfg-examples-marketplace-environments-beta-idempotencyenvironment</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/marketplace/layerx.example.json:18</code> | <code>"idempotencyEnvironment": "LAYERX_EMULATOR_MARKETPLACE_KEY"</code> |
| <code>interface.8.cfg-examples-marketplace-environments-beta-listingidenvironment</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/marketplace/layerx.example.json:13</code> | <code>"listingIdEnvironment": "LAYERX_EMULATOR_MARKETPLACE_LISTING_ID",</code> |
| <code>interface.8.cfg-examples-marketplace-environments-beta-network</code> | <code>config_key</code> | <code>change</code> | <code>services/platform/examples/marketplace/layerx.example.json:7</code> | <code>"network": "layerx:emulator",</code> |
| <code>interface.8.cfg-examples-marketplace-environments-beta-priceenvironment</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/marketplace/layerx.example.json:16</code> | <code>"priceEnvironment": "LAYERX_EMULATOR_PRICE",</code> |
| <code>interface.8.cfg-examples-marketplace-environments-beta-programidenvironment</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/marketplace/layerx.example.json:12</code> | <code>"programIdEnvironment": "LAYERX_EMULATOR_MARKETPLACE_PROGRAM_ID",</code> |
| <code>interface.8.cfg-examples-marketplace-environments-beta-protocolversion</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/examples/marketplace/layerx.example.json:8</code> | <code>"protocolVersion": 3,</code> |
| <code>interface.8.cfg-examples-marketplace-environments-beta-receiptauthorityurl</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/examples/marketplace/layerx.example.json:10</code> | <code>"receiptAuthorityUrl": "&lt;url&gt;",</code> |
| <code>interface.8.cfg-examples-marketplace-environments-beta-receiptdigestenvironment</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/marketplace/layerx.example.json:17</code> | <code>"receiptDigestEnvironment": "LAYERX_EMULATOR_MARKETPLACE_RECEIPT_DIGEST",</code> |
| <code>interface.8.cfg-examples-marketplace-environments-beta-sellerenvironment</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/marketplace/layerx.example.json:15</code> | <code>"sellerEnvironment": "LAYERX_EMULATOR_MERCHANT",</code> |
| <code>interface.8.cfg-examples-marketplace-environments-beta-tokenenvironment</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/marketplace/layerx.example.json:11</code> | <code>"tokenEnvironment": "LAYERX_EMULATOR_TOKEN",</code> |
| <code>interface.8.cfg-examples-marketplace-environments-emulator</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/marketplace/layerx.example.json:5</code> | <code>"emulator": {</code> |
| <code>interface.8.cfg-examples-marketplace-environments-emulator-assetenvironment</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/marketplace/layerx.example.json:14</code> | <code>"assetEnvironment": "LAYERX_EMULATOR_ASSET",</code> |
| <code>interface.8.cfg-examples-marketplace-environments-emulator-clibinaryenvironment</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/marketplace/layerx.example.json:9</code> | <code>"cliBinaryEnvironment": "LAYERX_BIN",</code> |
| <code>interface.8.cfg-examples-marketplace-environments-emulator-endpoint</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/marketplace/layerx.example.json:6</code> | <code>"endpoint": "&lt;url&gt;",</code> |
| <code>interface.8.cfg-examples-marketplace-environments-emulator-idempotencyenvironment</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/marketplace/layerx.example.json:18</code> | <code>"idempotencyEnvironment": "LAYERX_EMULATOR_MARKETPLACE_KEY"</code> |
| <code>interface.8.cfg-examples-marketplace-environments-emulator-listingidenvironment</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/marketplace/layerx.example.json:13</code> | <code>"listingIdEnvironment": "LAYERX_EMULATOR_MARKETPLACE_LISTING_ID",</code> |
| <code>interface.8.cfg-examples-marketplace-environments-emulator-network</code> | <code>config_key</code> | <code>change</code> | <code>services/platform/examples/marketplace/layerx.example.json:7</code> | <code>"network": "layerx:emulator",</code> |
| <code>interface.8.cfg-examples-marketplace-environments-emulator-priceenvironment</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/marketplace/layerx.example.json:16</code> | <code>"priceEnvironment": "LAYERX_EMULATOR_PRICE",</code> |
| <code>interface.8.cfg-examples-marketplace-environments-emulator-programidenvironment</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/marketplace/layerx.example.json:12</code> | <code>"programIdEnvironment": "LAYERX_EMULATOR_MARKETPLACE_PROGRAM_ID",</code> |
| <code>interface.8.cfg-examples-marketplace-environments-emulator-protocolversion</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/examples/marketplace/layerx.example.json:8</code> | <code>"protocolVersion": 3,</code> |
| <code>interface.8.cfg-examples-marketplace-environments-emulator-receiptauthorityurl</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/examples/marketplace/layerx.example.json:10</code> | <code>"receiptAuthorityUrl": "&lt;url&gt;",</code> |
| <code>interface.8.cfg-examples-marketplace-environments-emulator-receiptdigestenvironment</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/marketplace/layerx.example.json:17</code> | <code>"receiptDigestEnvironment": "LAYERX_EMULATOR_MARKETPLACE_RECEIPT_DIGEST",</code> |
| <code>interface.8.cfg-examples-marketplace-environments-emulator-sellerenvironment</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/marketplace/layerx.example.json:15</code> | <code>"sellerEnvironment": "LAYERX_EMULATOR_MERCHANT",</code> |
| <code>interface.8.cfg-examples-marketplace-environments-emulator-tokenenvironment</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/marketplace/layerx.example.json:11</code> | <code>"tokenEnvironment": "LAYERX_EMULATOR_TOKEN",</code> |
| <code>interface.8.cfg-examples-marketplace-version</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/marketplace/layerx.example.json:2</code> | <code>"version": 1,</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.cfg-examples-marketplace-environments</code> | notes | 30 callers total |
| <code>interface.8.cfg-examples-marketplace-environments-beta</code> | notes | 68 callers total |
| <code>interface.8.cfg-examples-marketplace-environments-beta-clibinaryenvironment</code> | notes | no in-repo config file sets the key outside the defining file |
| <code>interface.8.cfg-examples-marketplace-environments-beta-endpoint</code> | notes | 488 callers total |
| <code>interface.8.cfg-examples-marketplace-environments-beta-listingidenvironment</code> | notes | no in-repo config file sets the key outside the defining file |
| <code>interface.8.cfg-examples-marketplace-environments-beta-network</code> | notes | 386 callers total |
| <code>interface.8.cfg-examples-marketplace-environments-beta-network</code> | overridden_by | override.J41.91 |
| <code>interface.8.cfg-examples-marketplace-environments-beta-programidenvironment</code> | notes | no in-repo config file sets the key outside the defining file |
| <code>interface.8.cfg-examples-marketplace-environments-beta-protocolversion</code> | notes | 62 callers total |
| <code>interface.8.cfg-examples-marketplace-environments-beta-protocolversion</code> | overridden_by | override.J41.92 |
| <code>interface.8.cfg-examples-marketplace-environments-beta-receiptauthorityurl</code> | overridden_by | override.J41.93 |
| <code>interface.8.cfg-examples-marketplace-environments-beta-receiptdigestenvironment</code> | notes | no in-repo config file sets the key outside the defining file |
| <code>interface.8.cfg-examples-marketplace-environments-beta-sellerenvironment</code> | notes | no in-repo config file sets the key outside the defining file |
| <code>interface.8.cfg-examples-marketplace-environments-emulator</code> | notes | 54 callers total |
| <code>interface.8.cfg-examples-marketplace-environments-emulator-clibinaryenvironment</code> | notes | no in-repo config file sets the key outside the defining file |
| <code>interface.8.cfg-examples-marketplace-environments-emulator-endpoint</code> | notes | 488 callers total |
| <code>interface.8.cfg-examples-marketplace-environments-emulator-listingidenvironment</code> | notes | no in-repo config file sets the key outside the defining file |
| <code>interface.8.cfg-examples-marketplace-environments-emulator-network</code> | notes | 386 callers total |
| <code>interface.8.cfg-examples-marketplace-environments-emulator-network</code> | overridden_by | override.J41.94 |
| <code>interface.8.cfg-examples-marketplace-environments-emulator-programidenvironment</code> | notes | no in-repo config file sets the key outside the defining file |
| <code>interface.8.cfg-examples-marketplace-environments-emulator-protocolversion</code> | notes | 62 callers total |
| <code>interface.8.cfg-examples-marketplace-environments-emulator-protocolversion</code> | overridden_by | override.J41.95 |
| <code>interface.8.cfg-examples-marketplace-environments-emulator-receiptauthorityurl</code> | overridden_by | override.J41.96 |
| <code>interface.8.cfg-examples-marketplace-environments-emulator-receiptdigestenvironment</code> | notes | no in-repo config file sets the key outside the defining file |
| <code>interface.8.cfg-examples-marketplace-environments-emulator-sellerenvironment</code> | notes | no in-repo config file sets the key outside the defining file |
| <code>interface.8.cfg-examples-marketplace-version</code> | notes | 5296 callers total |

## examples-marketplace-env-vars

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.env-examples-marketplace-layerx-credential-passphrase</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/examples/marketplace/index.mjs:36</code> | <code>optionalEnvironment("LAYERX_CREDENTIAL_PASSPHRASE"),</code> |
| <code>interface.8.env-examples-marketplace-paxeer-x-reference-app-artifacts</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/examples/marketplace/index.mjs:190</code> | <code>const prebuiltPath = optionalEnvironment("PAXEER_X_REFERENCE_APP_ARTIFACTS");</code> |

## examples-merchant-checkout-config-keys

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.cfg-examples-merchant-checkout-application</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/merchant-checkout/layerx.example.json:3</code> | <code>"application": "merchant-checkout",</code> |
| <code>interface.8.cfg-examples-merchant-checkout-environments</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/merchant-checkout/layerx.example.json:4</code> | <code>"environments": {</code> |
| <code>interface.8.cfg-examples-merchant-checkout-environments-beta</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/merchant-checkout/layerx.example.json:22</code> | <code>"beta": {</code> |
| <code>interface.8.cfg-examples-merchant-checkout-environments-beta-accountenvironment</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/merchant-checkout/layerx.example.json:16</code> | <code>"accountEnvironment": "LAYERX_EMULATOR_MERCHANT_ACCOUNT",</code> |
| <code>interface.8.cfg-examples-merchant-checkout-environments-beta-assetenvironment</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/merchant-checkout/layerx.example.json:14</code> | <code>"assetEnvironment": "LAYERX_EMULATOR_ASSET",</code> |
| <code>interface.8.cfg-examples-merchant-checkout-environments-beta-currencyenvironment</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/merchant-checkout/layerx.example.json:17</code> | <code>"currencyEnvironment": "LAYERX_EMULATOR_CURRENCY",</code> |
| <code>interface.8.cfg-examples-merchant-checkout-environments-beta-network</code> | <code>config_key</code> | <code>change</code> | <code>services/platform/examples/merchant-checkout/layerx.example.json:10</code> | <code>"network": "layerx:emulator",</code> |
| <code>interface.8.cfg-examples-merchant-checkout-environments-beta-paytoenvironment</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/merchant-checkout/layerx.example.json:15</code> | <code>"payToEnvironment": "LAYERX_EMULATOR_MERCHANT",</code> |
| <code>interface.8.cfg-examples-merchant-checkout-environments-beta-port</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/merchant-checkout/layerx.example.json:20</code> | <code>"port": 8082</code> |
| <code>interface.8.cfg-examples-merchant-checkout-environments-beta-priceenvironment</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/merchant-checkout/layerx.example.json:18</code> | <code>"priceEnvironment": "LAYERX_EMULATOR_PRICE",</code> |
| <code>interface.8.cfg-examples-merchant-checkout-environments-beta-protocolversion</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/examples/merchant-checkout/layerx.example.json:11</code> | <code>"protocolVersion": 3,</code> |
| <code>interface.8.cfg-examples-merchant-checkout-environments-beta-publicurl</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/merchant-checkout/layerx.example.json:6</code> | <code>"publicUrl": "&lt;url&gt;",</code> |
| <code>interface.8.cfg-examples-merchant-checkout-environments-beta-receiptauthorityurl</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/examples/merchant-checkout/layerx.example.json:8</code> | <code>"receiptAuthorityUrl": "&lt;url&gt;",</code> |
| <code>interface.8.cfg-examples-merchant-checkout-environments-beta-scheme</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/merchant-checkout/layerx.example.json:9</code> | <code>"scheme": "exact",</code> |
| <code>interface.8.cfg-examples-merchant-checkout-environments-beta-settlementurl</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/merchant-checkout/layerx.example.json:7</code> | <code>"settlementUrl": "&lt;url&gt;",</code> |
| <code>interface.8.cfg-examples-merchant-checkout-environments-beta-statedirectory</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/merchant-checkout/layerx.example.json:19</code> | <code>"stateDirectory": "state",</code> |
| <code>interface.8.cfg-examples-merchant-checkout-environments-beta-tokenenvironment</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/merchant-checkout/layerx.example.json:12</code> | <code>"tokenEnvironment": "LAYERX_EMULATOR_TOKEN",</code> |
| <code>interface.8.cfg-examples-merchant-checkout-environments-beta-webhookkeysenvironment</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/merchant-checkout/layerx.example.json:13</code> | <code>"webhookKeysEnvironment": "LAYERX_EMULATOR_WEBHOOK_PUBLIC_KEYS_JSON",</code> |
| <code>interface.8.cfg-examples-merchant-checkout-environments-emulator</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/merchant-checkout/layerx.example.json:5</code> | <code>"emulator": {</code> |
| <code>interface.8.cfg-examples-merchant-checkout-environments-emulator-accountenvironment</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/merchant-checkout/layerx.example.json:16</code> | <code>"accountEnvironment": "LAYERX_EMULATOR_MERCHANT_ACCOUNT",</code> |
| <code>interface.8.cfg-examples-merchant-checkout-environments-emulator-assetenvironment</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/merchant-checkout/layerx.example.json:14</code> | <code>"assetEnvironment": "LAYERX_EMULATOR_ASSET",</code> |
| <code>interface.8.cfg-examples-merchant-checkout-environments-emulator-currencyenvironment</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/merchant-checkout/layerx.example.json:17</code> | <code>"currencyEnvironment": "LAYERX_EMULATOR_CURRENCY",</code> |
| <code>interface.8.cfg-examples-merchant-checkout-environments-emulator-network</code> | <code>config_key</code> | <code>change</code> | <code>services/platform/examples/merchant-checkout/layerx.example.json:10</code> | <code>"network": "layerx:emulator",</code> |
| <code>interface.8.cfg-examples-merchant-checkout-environments-emulator-paytoenvironment</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/merchant-checkout/layerx.example.json:15</code> | <code>"payToEnvironment": "LAYERX_EMULATOR_MERCHANT",</code> |
| <code>interface.8.cfg-examples-merchant-checkout-environments-emulator-port</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/merchant-checkout/layerx.example.json:20</code> | <code>"port": 8082</code> |
| <code>interface.8.cfg-examples-merchant-checkout-environments-emulator-priceenvironment</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/merchant-checkout/layerx.example.json:18</code> | <code>"priceEnvironment": "LAYERX_EMULATOR_PRICE",</code> |
| <code>interface.8.cfg-examples-merchant-checkout-environments-emulator-protocolversion</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/examples/merchant-checkout/layerx.example.json:11</code> | <code>"protocolVersion": 3,</code> |
| <code>interface.8.cfg-examples-merchant-checkout-environments-emulator-publicurl</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/merchant-checkout/layerx.example.json:6</code> | <code>"publicUrl": "&lt;url&gt;",</code> |
| <code>interface.8.cfg-examples-merchant-checkout-environments-emulator-receiptauthorityurl</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/examples/merchant-checkout/layerx.example.json:8</code> | <code>"receiptAuthorityUrl": "&lt;url&gt;",</code> |
| <code>interface.8.cfg-examples-merchant-checkout-environments-emulator-scheme</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/merchant-checkout/layerx.example.json:9</code> | <code>"scheme": "exact",</code> |
| <code>interface.8.cfg-examples-merchant-checkout-environments-emulator-settlementurl</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/merchant-checkout/layerx.example.json:7</code> | <code>"settlementUrl": "&lt;url&gt;",</code> |
| <code>interface.8.cfg-examples-merchant-checkout-environments-emulator-statedirectory</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/merchant-checkout/layerx.example.json:19</code> | <code>"stateDirectory": "state",</code> |
| <code>interface.8.cfg-examples-merchant-checkout-environments-emulator-tokenenvironment</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/merchant-checkout/layerx.example.json:12</code> | <code>"tokenEnvironment": "LAYERX_EMULATOR_TOKEN",</code> |
| <code>interface.8.cfg-examples-merchant-checkout-environments-emulator-webhookkeysenvironment</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/merchant-checkout/layerx.example.json:13</code> | <code>"webhookKeysEnvironment": "LAYERX_EMULATOR_WEBHOOK_PUBLIC_KEYS_JSON",</code> |
| <code>interface.8.cfg-examples-merchant-checkout-version</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/merchant-checkout/layerx.example.json:2</code> | <code>"version": 1,</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.cfg-examples-merchant-checkout-environments</code> | notes | 30 callers total |
| <code>interface.8.cfg-examples-merchant-checkout-environments-beta</code> | notes | 68 callers total |
| <code>interface.8.cfg-examples-merchant-checkout-environments-beta-network</code> | notes | 386 callers total |
| <code>interface.8.cfg-examples-merchant-checkout-environments-beta-network</code> | overridden_by | override.J41.108 |
| <code>interface.8.cfg-examples-merchant-checkout-environments-beta-port</code> | notes | 843 callers total |
| <code>interface.8.cfg-examples-merchant-checkout-environments-beta-protocolversion</code> | notes | 62 callers total |
| <code>interface.8.cfg-examples-merchant-checkout-environments-beta-protocolversion</code> | overridden_by | override.J41.109 |
| <code>interface.8.cfg-examples-merchant-checkout-environments-beta-receiptauthorityurl</code> | overridden_by | override.J41.110 |
| <code>interface.8.cfg-examples-merchant-checkout-environments-beta-scheme</code> | notes | 245 callers total |
| <code>interface.8.cfg-examples-merchant-checkout-environments-emulator</code> | notes | 54 callers total |
| <code>interface.8.cfg-examples-merchant-checkout-environments-emulator-network</code> | notes | 386 callers total |
| <code>interface.8.cfg-examples-merchant-checkout-environments-emulator-network</code> | overridden_by | override.J41.111 |
| <code>interface.8.cfg-examples-merchant-checkout-environments-emulator-port</code> | notes | 843 callers total |
| <code>interface.8.cfg-examples-merchant-checkout-environments-emulator-protocolversion</code> | notes | 62 callers total |
| <code>interface.8.cfg-examples-merchant-checkout-environments-emulator-protocolversion</code> | overridden_by | override.J41.112 |
| <code>interface.8.cfg-examples-merchant-checkout-environments-emulator-receiptauthorityurl</code> | overridden_by | override.J41.113 |
| <code>interface.8.cfg-examples-merchant-checkout-environments-emulator-scheme</code> | notes | 245 callers total |
| <code>interface.8.cfg-examples-merchant-checkout-version</code> | notes | 5296 callers total |

## examples-merchant-shop-config-keys

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.cfg-examples-merchant-shop-application</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/merchant-shop/layerx.example.json:3</code> | <code>"application": "merchant-shop",</code> |
| <code>interface.8.cfg-examples-merchant-shop-environments</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/merchant-shop/layerx.example.json:4</code> | <code>"environments": {</code> |
| <code>interface.8.cfg-examples-merchant-shop-environments-beta</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/merchant-shop/layerx.example.json:22</code> | <code>"beta": {</code> |
| <code>interface.8.cfg-examples-merchant-shop-environments-beta-accountenvironment</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/merchant-shop/layerx.example.json:16</code> | <code>"accountEnvironment": "LAYERX_EMULATOR_MERCHANT_ACCOUNT",</code> |
| <code>interface.8.cfg-examples-merchant-shop-environments-beta-assetenvironment</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/merchant-shop/layerx.example.json:14</code> | <code>"assetEnvironment": "LAYERX_EMULATOR_ASSET",</code> |
| <code>interface.8.cfg-examples-merchant-shop-environments-beta-currencyenvironment</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/merchant-shop/layerx.example.json:17</code> | <code>"currencyEnvironment": "LAYERX_EMULATOR_CURRENCY",</code> |
| <code>interface.8.cfg-examples-merchant-shop-environments-beta-network</code> | <code>config_key</code> | <code>change</code> | <code>services/platform/examples/merchant-shop/layerx.example.json:10</code> | <code>"network": "layerx:emulator",</code> |
| <code>interface.8.cfg-examples-merchant-shop-environments-beta-paytoenvironment</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/merchant-shop/layerx.example.json:15</code> | <code>"payToEnvironment": "LAYERX_EMULATOR_MERCHANT",</code> |
| <code>interface.8.cfg-examples-merchant-shop-environments-beta-port</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/merchant-shop/layerx.example.json:20</code> | <code>"port": 8082</code> |
| <code>interface.8.cfg-examples-merchant-shop-environments-beta-priceenvironment</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/merchant-shop/layerx.example.json:18</code> | <code>"priceEnvironment": "LAYERX_EMULATOR_PRICE",</code> |
| <code>interface.8.cfg-examples-merchant-shop-environments-beta-protocolversion</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/examples/merchant-shop/layerx.example.json:11</code> | <code>"protocolVersion": 3,</code> |
| <code>interface.8.cfg-examples-merchant-shop-environments-beta-publicurl</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/merchant-shop/layerx.example.json:6</code> | <code>"publicUrl": "&lt;url&gt;",</code> |
| <code>interface.8.cfg-examples-merchant-shop-environments-beta-receiptauthorityurl</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/examples/merchant-shop/layerx.example.json:8</code> | <code>"receiptAuthorityUrl": "&lt;url&gt;",</code> |
| <code>interface.8.cfg-examples-merchant-shop-environments-beta-scheme</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/merchant-shop/layerx.example.json:9</code> | <code>"scheme": "exact",</code> |
| <code>interface.8.cfg-examples-merchant-shop-environments-beta-settlementurl</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/merchant-shop/layerx.example.json:7</code> | <code>"settlementUrl": "&lt;url&gt;",</code> |
| <code>interface.8.cfg-examples-merchant-shop-environments-beta-statedirectory</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/merchant-shop/layerx.example.json:19</code> | <code>"stateDirectory": "state",</code> |
| <code>interface.8.cfg-examples-merchant-shop-environments-beta-tokenenvironment</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/merchant-shop/layerx.example.json:12</code> | <code>"tokenEnvironment": "LAYERX_EMULATOR_TOKEN",</code> |
| <code>interface.8.cfg-examples-merchant-shop-environments-beta-webhookkeysenvironment</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/merchant-shop/layerx.example.json:13</code> | <code>"webhookKeysEnvironment": "LAYERX_EMULATOR_WEBHOOK_PUBLIC_KEYS_JSON",</code> |
| <code>interface.8.cfg-examples-merchant-shop-environments-emulator</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/merchant-shop/layerx.example.json:5</code> | <code>"emulator": {</code> |
| <code>interface.8.cfg-examples-merchant-shop-environments-emulator-accountenvironment</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/merchant-shop/layerx.example.json:16</code> | <code>"accountEnvironment": "LAYERX_EMULATOR_MERCHANT_ACCOUNT",</code> |
| <code>interface.8.cfg-examples-merchant-shop-environments-emulator-assetenvironment</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/merchant-shop/layerx.example.json:14</code> | <code>"assetEnvironment": "LAYERX_EMULATOR_ASSET",</code> |
| <code>interface.8.cfg-examples-merchant-shop-environments-emulator-currencyenvironment</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/merchant-shop/layerx.example.json:17</code> | <code>"currencyEnvironment": "LAYERX_EMULATOR_CURRENCY",</code> |
| <code>interface.8.cfg-examples-merchant-shop-environments-emulator-network</code> | <code>config_key</code> | <code>change</code> | <code>services/platform/examples/merchant-shop/layerx.example.json:10</code> | <code>"network": "layerx:emulator",</code> |
| <code>interface.8.cfg-examples-merchant-shop-environments-emulator-paytoenvironment</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/merchant-shop/layerx.example.json:15</code> | <code>"payToEnvironment": "LAYERX_EMULATOR_MERCHANT",</code> |
| <code>interface.8.cfg-examples-merchant-shop-environments-emulator-port</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/merchant-shop/layerx.example.json:20</code> | <code>"port": 8082</code> |
| <code>interface.8.cfg-examples-merchant-shop-environments-emulator-priceenvironment</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/merchant-shop/layerx.example.json:18</code> | <code>"priceEnvironment": "LAYERX_EMULATOR_PRICE",</code> |
| <code>interface.8.cfg-examples-merchant-shop-environments-emulator-protocolversion</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/examples/merchant-shop/layerx.example.json:11</code> | <code>"protocolVersion": 3,</code> |
| <code>interface.8.cfg-examples-merchant-shop-environments-emulator-publicurl</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/merchant-shop/layerx.example.json:6</code> | <code>"publicUrl": "&lt;url&gt;",</code> |
| <code>interface.8.cfg-examples-merchant-shop-environments-emulator-receiptauthorityurl</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/examples/merchant-shop/layerx.example.json:8</code> | <code>"receiptAuthorityUrl": "&lt;url&gt;",</code> |
| <code>interface.8.cfg-examples-merchant-shop-environments-emulator-scheme</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/merchant-shop/layerx.example.json:9</code> | <code>"scheme": "exact",</code> |
| <code>interface.8.cfg-examples-merchant-shop-environments-emulator-settlementurl</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/merchant-shop/layerx.example.json:7</code> | <code>"settlementUrl": "&lt;url&gt;",</code> |
| <code>interface.8.cfg-examples-merchant-shop-environments-emulator-statedirectory</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/merchant-shop/layerx.example.json:19</code> | <code>"stateDirectory": "state",</code> |
| <code>interface.8.cfg-examples-merchant-shop-environments-emulator-tokenenvironment</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/merchant-shop/layerx.example.json:12</code> | <code>"tokenEnvironment": "LAYERX_EMULATOR_TOKEN",</code> |
| <code>interface.8.cfg-examples-merchant-shop-environments-emulator-webhookkeysenvironment</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/merchant-shop/layerx.example.json:13</code> | <code>"webhookKeysEnvironment": "LAYERX_EMULATOR_WEBHOOK_PUBLIC_KEYS_JSON",</code> |
| <code>interface.8.cfg-examples-merchant-shop-version</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/merchant-shop/layerx.example.json:2</code> | <code>"version": 1,</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.cfg-examples-merchant-shop-environments</code> | notes | 30 callers total |
| <code>interface.8.cfg-examples-merchant-shop-environments-beta</code> | notes | 68 callers total |
| <code>interface.8.cfg-examples-merchant-shop-environments-beta-network</code> | notes | 386 callers total |
| <code>interface.8.cfg-examples-merchant-shop-environments-beta-network</code> | overridden_by | override.J41.114 |
| <code>interface.8.cfg-examples-merchant-shop-environments-beta-port</code> | notes | 843 callers total |
| <code>interface.8.cfg-examples-merchant-shop-environments-beta-protocolversion</code> | notes | 62 callers total |
| <code>interface.8.cfg-examples-merchant-shop-environments-beta-protocolversion</code> | overridden_by | override.J41.115 |
| <code>interface.8.cfg-examples-merchant-shop-environments-beta-receiptauthorityurl</code> | overridden_by | override.J41.116 |
| <code>interface.8.cfg-examples-merchant-shop-environments-beta-scheme</code> | notes | 245 callers total |
| <code>interface.8.cfg-examples-merchant-shop-environments-emulator</code> | notes | 54 callers total |
| <code>interface.8.cfg-examples-merchant-shop-environments-emulator-network</code> | notes | 386 callers total |
| <code>interface.8.cfg-examples-merchant-shop-environments-emulator-network</code> | overridden_by | override.J41.117 |
| <code>interface.8.cfg-examples-merchant-shop-environments-emulator-port</code> | notes | 843 callers total |
| <code>interface.8.cfg-examples-merchant-shop-environments-emulator-protocolversion</code> | notes | 62 callers total |
| <code>interface.8.cfg-examples-merchant-shop-environments-emulator-protocolversion</code> | overridden_by | override.J41.118 |
| <code>interface.8.cfg-examples-merchant-shop-environments-emulator-receiptauthorityurl</code> | overridden_by | override.J41.119 |
| <code>interface.8.cfg-examples-merchant-shop-environments-emulator-scheme</code> | notes | 245 callers total |
| <code>interface.8.cfg-examples-merchant-shop-version</code> | notes | 5296 callers total |

## examples-paid-api-config-keys

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.cfg-examples-paid-api-application</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/paid-api/layerx.example.json:3</code> | <code>"application": "paid-api",</code> |
| <code>interface.8.cfg-examples-paid-api-environments</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/paid-api/layerx.example.json:4</code> | <code>"environments": {</code> |
| <code>interface.8.cfg-examples-paid-api-environments-beta</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/paid-api/layerx.example.json:21</code> | <code>"beta": {</code> |
| <code>interface.8.cfg-examples-paid-api-environments-beta-accountenvironment</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/paid-api/layerx.example.json:14</code> | <code>"accountEnvironment": "LAYERX_EMULATOR_SELLER_ACCOUNT",</code> |
| <code>interface.8.cfg-examples-paid-api-environments-beta-assetenvironment</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/paid-api/layerx.example.json:12</code> | <code>"assetEnvironment": "LAYERX_EMULATOR_ASSET",</code> |
| <code>interface.8.cfg-examples-paid-api-environments-beta-currencyenvironment</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/paid-api/layerx.example.json:15</code> | <code>"currencyEnvironment": "LAYERX_EMULATOR_CURRENCY",</code> |
| <code>interface.8.cfg-examples-paid-api-environments-beta-fulfillmentdirectory</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/paid-api/layerx.example.json:18</code> | <code>"fulfillmentDirectory": "state/fulfillments",</code> |
| <code>interface.8.cfg-examples-paid-api-environments-beta-network</code> | <code>config_key</code> | <code>change</code> | <code>services/platform/examples/paid-api/layerx.example.json:9</code> | <code>"network": "layerx:emulator",</code> |
| <code>interface.8.cfg-examples-paid-api-environments-beta-paytoenvironment</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/paid-api/layerx.example.json:13</code> | <code>"payToEnvironment": "LAYERX_EMULATOR_SELLER",</code> |
| <code>interface.8.cfg-examples-paid-api-environments-beta-port</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/paid-api/layerx.example.json:19</code> | <code>"port": 8081</code> |
| <code>interface.8.cfg-examples-paid-api-environments-beta-priceenvironment</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/paid-api/layerx.example.json:16</code> | <code>"priceEnvironment": "LAYERX_EMULATOR_PRICE",</code> |
| <code>interface.8.cfg-examples-paid-api-environments-beta-protocolversion</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/examples/paid-api/layerx.example.json:10</code> | <code>"protocolVersion": 3,</code> |
| <code>interface.8.cfg-examples-paid-api-environments-beta-receiptauthorityurl</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/examples/paid-api/layerx.example.json:6</code> | <code>"receiptAuthorityUrl": "&lt;url&gt;",</code> |
| <code>interface.8.cfg-examples-paid-api-environments-beta-resourcefile</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/paid-api/layerx.example.json:17</code> | <code>"resourceFile": "resource.json",</code> |
| <code>interface.8.cfg-examples-paid-api-environments-beta-resourceurl</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/paid-api/layerx.example.json:7</code> | <code>"resourceUrl": "&lt;url&gt;",</code> |
| <code>interface.8.cfg-examples-paid-api-environments-beta-scheme</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/paid-api/layerx.example.json:8</code> | <code>"scheme": "exact",</code> |
| <code>interface.8.cfg-examples-paid-api-environments-beta-tokenenvironment</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/paid-api/layerx.example.json:11</code> | <code>"tokenEnvironment": "LAYERX_EMULATOR_TOKEN",</code> |
| <code>interface.8.cfg-examples-paid-api-environments-emulator</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/paid-api/layerx.example.json:5</code> | <code>"emulator": {</code> |
| <code>interface.8.cfg-examples-paid-api-environments-emulator-accountenvironment</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/paid-api/layerx.example.json:14</code> | <code>"accountEnvironment": "LAYERX_EMULATOR_SELLER_ACCOUNT",</code> |
| <code>interface.8.cfg-examples-paid-api-environments-emulator-assetenvironment</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/paid-api/layerx.example.json:12</code> | <code>"assetEnvironment": "LAYERX_EMULATOR_ASSET",</code> |
| <code>interface.8.cfg-examples-paid-api-environments-emulator-currencyenvironment</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/paid-api/layerx.example.json:15</code> | <code>"currencyEnvironment": "LAYERX_EMULATOR_CURRENCY",</code> |
| <code>interface.8.cfg-examples-paid-api-environments-emulator-fulfillmentdirectory</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/paid-api/layerx.example.json:18</code> | <code>"fulfillmentDirectory": "state/fulfillments",</code> |
| <code>interface.8.cfg-examples-paid-api-environments-emulator-network</code> | <code>config_key</code> | <code>change</code> | <code>services/platform/examples/paid-api/layerx.example.json:9</code> | <code>"network": "layerx:emulator",</code> |
| <code>interface.8.cfg-examples-paid-api-environments-emulator-paytoenvironment</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/paid-api/layerx.example.json:13</code> | <code>"payToEnvironment": "LAYERX_EMULATOR_SELLER",</code> |
| <code>interface.8.cfg-examples-paid-api-environments-emulator-port</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/paid-api/layerx.example.json:19</code> | <code>"port": 8081</code> |
| <code>interface.8.cfg-examples-paid-api-environments-emulator-priceenvironment</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/paid-api/layerx.example.json:16</code> | <code>"priceEnvironment": "LAYERX_EMULATOR_PRICE",</code> |
| <code>interface.8.cfg-examples-paid-api-environments-emulator-protocolversion</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/examples/paid-api/layerx.example.json:10</code> | <code>"protocolVersion": 3,</code> |
| <code>interface.8.cfg-examples-paid-api-environments-emulator-receiptauthorityurl</code> | <code>config_key</code> | <code>retire</code> | <code>services/platform/examples/paid-api/layerx.example.json:6</code> | <code>"receiptAuthorityUrl": "&lt;url&gt;",</code> |
| <code>interface.8.cfg-examples-paid-api-environments-emulator-resourcefile</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/paid-api/layerx.example.json:17</code> | <code>"resourceFile": "resource.json",</code> |
| <code>interface.8.cfg-examples-paid-api-environments-emulator-resourceurl</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/paid-api/layerx.example.json:7</code> | <code>"resourceUrl": "&lt;url&gt;",</code> |
| <code>interface.8.cfg-examples-paid-api-environments-emulator-scheme</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/paid-api/layerx.example.json:8</code> | <code>"scheme": "exact",</code> |
| <code>interface.8.cfg-examples-paid-api-environments-emulator-tokenenvironment</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/paid-api/layerx.example.json:11</code> | <code>"tokenEnvironment": "LAYERX_EMULATOR_TOKEN",</code> |
| <code>interface.8.cfg-examples-paid-api-forecast</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/paid-api/resource.json:3</code> | <code>"forecast": "clear",</code> |
| <code>interface.8.cfg-examples-paid-api-service</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/paid-api/resource.json:2</code> | <code>"service": "LayerX metered weather",</code> |
| <code>interface.8.cfg-examples-paid-api-units</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/paid-api/resource.json:4</code> | <code>"units": "protocol-integer"</code> |
| <code>interface.8.cfg-examples-paid-api-version</code> | <code>config_key</code> | <code>stable</code> | <code>services/platform/examples/paid-api/layerx.example.json:2</code> | <code>"version": 1,</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.cfg-examples-paid-api-environments</code> | notes | 30 callers total |
| <code>interface.8.cfg-examples-paid-api-environments-beta</code> | notes | 68 callers total |
| <code>interface.8.cfg-examples-paid-api-environments-beta-fulfillmentdirectory</code> | notes | no in-repo config file sets the key outside the defining file |
| <code>interface.8.cfg-examples-paid-api-environments-beta-network</code> | notes | 386 callers total |
| <code>interface.8.cfg-examples-paid-api-environments-beta-network</code> | overridden_by | override.J41.84 |
| <code>interface.8.cfg-examples-paid-api-environments-beta-port</code> | notes | 843 callers total |
| <code>interface.8.cfg-examples-paid-api-environments-beta-protocolversion</code> | notes | 62 callers total |
| <code>interface.8.cfg-examples-paid-api-environments-beta-protocolversion</code> | overridden_by | override.J41.85 |
| <code>interface.8.cfg-examples-paid-api-environments-beta-receiptauthorityurl</code> | overridden_by | override.J41.86 |
| <code>interface.8.cfg-examples-paid-api-environments-beta-resourcefile</code> | notes | no in-repo config file sets the key outside the defining file |
| <code>interface.8.cfg-examples-paid-api-environments-beta-scheme</code> | notes | 245 callers total |
| <code>interface.8.cfg-examples-paid-api-environments-emulator</code> | notes | 54 callers total |
| <code>interface.8.cfg-examples-paid-api-environments-emulator-fulfillmentdirectory</code> | notes | no in-repo config file sets the key outside the defining file |
| <code>interface.8.cfg-examples-paid-api-environments-emulator-network</code> | notes | 386 callers total |
| <code>interface.8.cfg-examples-paid-api-environments-emulator-network</code> | overridden_by | override.J41.87 |
| <code>interface.8.cfg-examples-paid-api-environments-emulator-port</code> | notes | 843 callers total |
| <code>interface.8.cfg-examples-paid-api-environments-emulator-protocolversion</code> | notes | 62 callers total |
| <code>interface.8.cfg-examples-paid-api-environments-emulator-protocolversion</code> | overridden_by | override.J41.88 |
| <code>interface.8.cfg-examples-paid-api-environments-emulator-receiptauthorityurl</code> | overridden_by | override.J41.89 |
| <code>interface.8.cfg-examples-paid-api-environments-emulator-resourcefile</code> | notes | no in-repo config file sets the key outside the defining file |
| <code>interface.8.cfg-examples-paid-api-environments-emulator-scheme</code> | notes | 245 callers total |
| <code>interface.8.cfg-examples-paid-api-service</code> | notes | 1211 callers total |
| <code>interface.8.cfg-examples-paid-api-units</code> | notes | 77 callers total |
| <code>interface.8.cfg-examples-paid-api-version</code> | notes | 5296 callers total |

## examples-paid-api-http-routes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.http-examples-paid-api-get-paid</code> | <code>http_route</code> | <code>change</code> | <code>services/platform/examples/paid-api/index.mjs:159</code> | <code>if (request.method !== "GET" &#124;&#124; request.url !== "/paid") {</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.http-examples-paid-api-get-paid</code> | overridden_by | override.J41.90 |

## examples-support-cli-commands

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.cli-examples-support-runtime-mjs-action</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/examples/support/runtime.mjs:191</code> | <code>if (arguments_.length === 4 &amp;&amp; (arguments_[2] !== "--action" &#124;&#124; !["deploy", "list", "buy"].includes(arguments_[3]))) {</code> |
| <code>interface.8.cli-examples-support-runtime-mjs-environment</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/examples/support/runtime.mjs:188</code> | <code>if ((arguments_.length !== 2 &amp;&amp; arguments_.length !== 4) &#124;&#124; arguments_[0] !== "--environment" &#124;&#124; !["emulator", "beta"].includes(arguments_[1])) {</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.cli-examples-support-runtime-mjs-action</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-examples-support-runtime-mjs-environment</code> | notes | no in-repo invocation outside the defining file |

## examples-support-env-vars

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.env-examples-support-layerx-example-config</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/examples/support/runtime.mjs:31</code> | <code>const document = exactObject(JSON.parse(await readFile(resolve(directory, optionalEnvironment("LAYERX_EXAMPLE_CONFIG") ?? "layerx.example.json"), "utf8")));</code> |
| <code>interface.8.env-examples-support-layerx-example-endpoint</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/examples/support/runtime.mjs:70</code> | <code>const override = optionalEnvironment("LAYERX_EXAMPLE_ENDPOINT");</code> |
| <code>interface.8.env-examples-support-layerx-example-state-root</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/examples/support/runtime.mjs:51</code> | <code>const stateRoot = optionalEnvironment("LAYERX_EXAMPLE_STATE_ROOT");</code> |

## examples-support-http-routes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.http-examples-support-get-orders</code> | <code>http_route</code> | <code>change</code> | <code>services/platform/examples/support/merchant-app.mjs:382</code> | <code>if (request.method === "GET" &amp;&amp; request.url?.startsWith("/orders/")) {</code> |
| <code>interface.8.http-examples-support-post-checkout</code> | <code>http_route</code> | <code>change</code> | <code>services/platform/examples/support/merchant-app.mjs:400</code> | <code>if (request.method !== "POST" &#124;&#124; request.url !== "/checkout") {</code> |
| <code>interface.8.http-examples-support-post-webhooks-settlement</code> | <code>http_route</code> | <code>change</code> | <code>services/platform/examples/support/merchant-app.mjs:388</code> | <code>if (request.method === "POST" &amp;&amp; request.url === "/webhooks/settlement") {</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.http-examples-support-get-orders</code> | notes | 32 callers total |
| <code>interface.8.http-examples-support-get-orders</code> | overridden_by | override.J41.103 |
| <code>interface.8.http-examples-support-post-checkout</code> | notes | 506 callers total; no in-repo client cites the path outside the defining file |
| <code>interface.8.http-examples-support-post-checkout</code> | overridden_by | override.J41.104 |
| <code>interface.8.http-examples-support-post-webhooks-settlement</code> | overridden_by | override.J41.105 |

## middleware-agent-error-codes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.err-middleware-agent-budget-refused</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/middleware/agent/src/index.ts:852</code> | <code>if (result.kind === "exhausted") return { kind: "refused", reason: "budget_refused" };</code> |
| <code>interface.8.err-middleware-agent-budget-released</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/middleware/agent/src/index.ts:856</code> | <code>if (reservation.state === "released") return { kind: "refused", reason: "budget_released" };</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.err-middleware-agent-budget-refused</code> | notes | no in-repo consumer quotes the code outside the defining file |
| <code>interface.8.err-middleware-agent-budget-released</code> | notes | no in-repo consumer quotes the code outside the defining file |

## middleware-agent-ts-exports

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.ts-middleware-agent-index-agentgrantmiddleware-j41</code> | <code>ts_export</code> | <code>change</code> | <code>services/platform/middleware/agent/src/index.ts:828</code> | <code>export class AgentGrantMiddleware implements GrantDrawExecution {</code> |
| <code>interface.8.ts-middleware-agent-index-agentmiddlewareconfig-j41</code> | <code>ts_export</code> | <code>change</code> | <code>services/platform/middleware/agent/src/index.ts:201</code> | <code>export interface AgentMiddlewareConfig {</code> |
| <code>interface.8.ts-middleware-agent-index-agentmiddlewareerrorcode-j41</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/agent/src/index.ts:546</code> | <code>export type AgentMiddlewareErrorCode =</code> |
| <code>interface.8.ts-middleware-agent-index-agentrefusalretry-j41</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/agent/src/index.ts:171</code> | <code>export type AgentRefusalRetry = Exclude&lt;RetryClass, "unknown-outcome"&gt;;</code> |
| <code>interface.8.ts-middleware-agent-index-approvalhold-j41</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/agent/src/index.ts:164</code> | <code>export interface ApprovalHold {</code> |
| <code>interface.8.ts-middleware-agent-index-budgettransition-j41</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/agent/src/index.ts:82</code> | <code>export interface BudgetTransition {</code> |
| <code>interface.8.ts-middleware-agent-index-platform-mw-agent-j41</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/agent/src/index.ts:561</code> | <code>export function platform_mw_agent(): "budget-aware-receipt-verified-agent" {</code> |
| <code>interface.8.ts-middleware-agent-index-reservedbudgetreservation-j41</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/agent/src/index.ts:51</code> | <code>export type ReservedBudgetReservation = BudgetReservationBase &amp; {</code> |
| <code>interface.8.ts-middleware-agent-index-submission-j41</code> | <code>ts_export</code> | <code>change</code> | <code>services/platform/middleware/agent/src/index.ts:132</code> | <code>export interface Submission {</code> |
| <code>interface.8.ts-middleware-agent-index-verifycommittedagentpayment-j41</code> | <code>ts_export</code> | <code>change</code> | <code>services/platform/middleware/agent/src/index.ts:795</code> | <code>export async function verifyCommittedAgentPayment(</code> |
| <code>interface.8.ts-middleware-agent-sidiora-layerx-agent-middleware-agentbudgetledger</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/agent/src/index.ts:102</code> | <code>export interface AgentBudgetLedger {</code> |
| <code>interface.8.ts-middleware-agent-sidiora-layerx-agent-middleware-agentmiddleware</code> | <code>ts_export</code> | <code>change</code> | <code>services/platform/middleware/agent/src/index.ts:227</code> | <code>export class AgentMiddleware {</code> |
| <code>interface.8.ts-middleware-agent-sidiora-layerx-agent-middleware-agentmiddlewareerror</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/agent/src/index.ts:554</code> | <code>export class AgentMiddlewareError extends Error {</code> |
| <code>interface.8.ts-middleware-agent-sidiora-layerx-agent-middleware-agentpreparationbudget</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/agent/src/index.ts:192</code> | <code>export interface AgentPreparationBudget {</code> |
| <code>interface.8.ts-middleware-agent-sidiora-layerx-agent-middleware-agentreceiptcontext</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/agent/src/index.ts:148</code> | <code>export interface AgentReceiptContext {</code> |
| <code>interface.8.ts-middleware-agent-sidiora-layerx-agent-middleware-agentreceiptevidence</code> | <code>ts_export</code> | <code>change</code> | <code>services/platform/middleware/agent/src/index.ts:143</code> | <code>export interface AgentReceiptEvidence {</code> |
| <code>interface.8.ts-middleware-agent-sidiora-layerx-agent-middleware-agentreceiptrecording</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/agent/src/index.ts:153</code> | <code>export interface AgentReceiptRecording extends AgentReceiptContext {</code> |
| <code>interface.8.ts-middleware-agent-sidiora-layerx-agent-middleware-agentreceiptresolver</code> | <code>ts_export</code> | <code>change</code> | <code>services/platform/middleware/agent/src/index.ts:158</code> | <code>export interface AgentReceiptResolver {</code> |
| <code>interface.8.ts-middleware-agent-sidiora-layerx-agent-middleware-agentrefusal</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/agent/src/index.ts:173</code> | <code>export interface AgentRefusal {</code> |
| <code>interface.8.ts-middleware-agent-sidiora-layerx-agent-middleware-agentsigner</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/agent/src/index.ts:126</code> | <code>export interface AgentSigner {</code> |
| <code>interface.8.ts-middleware-agent-sidiora-layerx-agent-middleware-agentspendrequest</code> | <code>ts_export</code> | <code>change</code> | <code>services/platform/middleware/agent/src/index.ts:28</code> | <code>export interface AgentSpendRequest {</code> |
| <code>interface.8.ts-middleware-agent-sidiora-layerx-agent-middleware-agentspendresult</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/agent/src/index.ts:213</code> | <code>export type AgentSpendResult =</code> |
| <code>interface.8.ts-middleware-agent-sidiora-layerx-agent-middleware-budgetcommittransition</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/agent/src/index.ts:94</code> | <code>export interface BudgetCommitTransition extends BudgetTransition {</code> |
| <code>interface.8.ts-middleware-agent-sidiora-layerx-agent-middleware-budgetholdtransition</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/agent/src/index.ts:89</code> | <code>export interface BudgetHoldTransition extends BudgetTransition {</code> |
| <code>interface.8.ts-middleware-agent-sidiora-layerx-agent-middleware-budgetreleasetransition</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/agent/src/index.ts:98</code> | <code>export interface BudgetReleaseTransition extends BudgetTransition {</code> |
| <code>interface.8.ts-middleware-agent-sidiora-layerx-agent-middleware-budgetreservation</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/agent/src/index.ts:71</code> | <code>export type BudgetReservation =</code> |
| <code>interface.8.ts-middleware-agent-sidiora-layerx-agent-middleware-budgetreserveresult</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/agent/src/index.ts:77</code> | <code>export type BudgetReserveResult =</code> |
| <code>interface.8.ts-middleware-agent-sidiora-layerx-agent-middleware-committedbudgetreservation</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/agent/src/index.ts:61</code> | <code>export type CommittedBudgetReservation = BudgetReservationBase &amp; {</code> |
| <code>interface.8.ts-middleware-agent-sidiora-layerx-agent-middleware-decodedaemonprepared</code> | <code>ts_export</code> | <code>change</code> | <code>services/platform/middleware/agent/src/daemon.ts:98</code> | <code>export function decodeDaemonPrepared(value: unknown, request: AgentSpendRequest, protocolVersion: number): PreparedActivity {</code> |
| <code>interface.8.ts-middleware-agent-sidiora-layerx-agent-middleware-decodedaemonsubmission</code> | <code>ts_export</code> | <code>change</code> | <code>services/platform/middleware/agent/src/daemon.ts:130</code> | <code>export function decodeDaemonSubmission(value: unknown, expectedActivityId?: string, expectedReference?: string): Submission {</code> |
| <code>interface.8.ts-middleware-agent-sidiora-layerx-agent-middleware-heldbudgetreservation</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/agent/src/index.ts:55</code> | <code>export type HeldBudgetReservation = BudgetReservationBase &amp; {</code> |
| <code>interface.8.ts-middleware-agent-sidiora-layerx-agent-middleware-ownerbudgetspendresult</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/agent/src/index.ts:181</code> | <code>export interface OwnerBudgetSpendResult {</code> |
| <code>interface.8.ts-middleware-agent-sidiora-layerx-agent-middleware-preparedactivity</code> | <code>ts_export</code> | <code>change</code> | <code>services/platform/middleware/agent/src/index.ts:115</code> | <code>export interface PreparedActivity {</code> |
| <code>interface.8.ts-middleware-agent-sidiora-layerx-agent-middleware-releasedbudgetreservation</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/agent/src/index.ts:66</code> | <code>export type ReleasedBudgetReservation = BudgetReservationBase &amp; {</code> |
| <code>interface.8.ts-middleware-agent-sidiora-layerx-agent-middleware-signedactivityid</code> | <code>ts_export</code> | <code>change</code> | <code>services/platform/middleware/agent/src/daemon.ts:122</code> | <code>export function signedActivityId(prepared: PreparedActivity, signature: string, publicKey: string): string {</code> |
| <code>interface.8.ts-middleware-agent-sidiora-layerx-agent-middleware-verifyagentpayment</code> | <code>ts_export</code> | <code>change</code> | <code>services/platform/middleware/agent/src/index.ts:816</code> | <code>export async function verifyAgentPayment(evidence: AgentReceiptEvidence, request: AgentSpendRequest,</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.ts-middleware-agent-index-platform-mw-agent-j41</code> | notes | public item of its package; no in-repo caller outside the defining line found at HEAD |
| <code>interface.8.ts-middleware-agent-sidiora-layerx-agent-middleware-agentmiddleware</code> | overridden_by | override.J41.25 |
| <code>interface.8.ts-middleware-agent-sidiora-layerx-agent-middleware-agentreceiptevidence</code> | overridden_by | override.J41.26 |
| <code>interface.8.ts-middleware-agent-sidiora-layerx-agent-middleware-agentreceiptresolver</code> | overridden_by | override.J41.27 |
| <code>interface.8.ts-middleware-agent-sidiora-layerx-agent-middleware-agentspendrequest</code> | overridden_by | override.J41.20 |
| <code>interface.8.ts-middleware-agent-sidiora-layerx-agent-middleware-decodedaemonprepared</code> | overridden_by | override.J41.21 |
| <code>interface.8.ts-middleware-agent-sidiora-layerx-agent-middleware-decodedaemonsubmission</code> | overridden_by | override.J41.22 |
| <code>interface.8.ts-middleware-agent-sidiora-layerx-agent-middleware-preparedactivity</code> | overridden_by | override.J41.23 |
| <code>interface.8.ts-middleware-agent-sidiora-layerx-agent-middleware-signedactivityid</code> | overridden_by | override.J41.24 |
| <code>interface.8.ts-middleware-agent-sidiora-layerx-agent-middleware-verifyagentpayment</code> | overridden_by | override.J41.28 |

## middleware-buyer-ts-exports

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.ts-middleware-buyer-index-buyermiddlewareconfig-j41</code> | <code>ts_export</code> | <code>change</code> | <code>services/platform/middleware/buyer/src/index.ts:62</code> | <code>export interface BuyerMiddlewareConfig {</code> |
| <code>interface.8.ts-middleware-buyer-index-buyerretrypolicy-j41</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/buyer/src/index.ts:54</code> | <code>export interface BuyerRetryPolicy {</code> |
| <code>interface.8.ts-middleware-buyer-index-capturedsettlement-j41</code> | <code>ts_export</code> | <code>change</code> | <code>services/platform/middleware/buyer/src/index.ts:166</code> | <code>export interface CapturedSettlement {</code> |
| <code>interface.8.ts-middleware-buyer-index-evidencematerial-j41</code> | <code>ts_export</code> | <code>change</code> | <code>services/platform/middleware/buyer/src/index.ts:139</code> | <code>export interface EvidenceMaterial {</code> |
| <code>interface.8.ts-middleware-buyer-index-evidencereference-j41</code> | <code>ts_export</code> | <code>change</code> | <code>services/platform/middleware/buyer/src/index.ts:112</code> | <code>export interface EvidenceReference {</code> |
| <code>interface.8.ts-middleware-buyer-index-grantpaymentheader-j41</code> | <code>ts_export</code> | <code>change</code> | <code>services/platform/middleware/buyer/src/index.ts:952</code> | <code>export function grantPaymentHeader(required: PaymentRequired, accepted: PaymentRequirements, receiveHex: string): string {</code> |
| <code>interface.8.ts-middleware-buyer-index-journeystate-j41</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/buyer/src/index.ts:102</code> | <code>export type JourneyState =</code> |
| <code>interface.8.ts-middleware-buyer-index-layerxhttptransportconfig-j41</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/buyer/src/index.ts:516</code> | <code>export interface LayerXHttpTransportConfig {</code> |
| <code>interface.8.ts-middleware-buyer-index-movequoterequest-j41</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/buyer/src/index.ts:79</code> | <code>export interface MoveQuoteRequest {</code> |
| <code>interface.8.ts-middleware-buyer-index-paymentpreparation-j41</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/buyer/src/index.ts:160</code> | <code>export type PaymentPreparation =</code> |
| <code>interface.8.ts-middleware-buyer-index-paymentquoteterms-2-j41</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/buyer/src/index.ts:865</code> | <code>export function paymentQuoteTerms(requirements: PaymentRequirements): PaymentQuoteTerms {</code> |
| <code>interface.8.ts-middleware-buyer-index-paymentquoteterms-j41</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/buyer/src/index.ts:85</code> | <code>export interface PaymentQuoteTerms {</code> |
| <code>interface.8.ts-middleware-buyer-index-platform-mw-buyer-j41</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/buyer/src/index.ts:605</code> | <code>export function platform_mw_buyer(): "sdk-quoted-receipt-verified-x402-buyer" {</code> |
| <code>interface.8.ts-middleware-buyer-sidiora-layerx-buyer-middleware-accountidentifiers</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/buyer/src/index.ts:877</code> | <code>export async function accountIdentifiers(account: string): Promise&lt;readonly Uint8Array[]&gt; {</code> |
| <code>interface.8.ts-middleware-buyer-sidiora-layerx-buyer-middleware-buyermiddleware</code> | <code>ts_export</code> | <code>change</code> | <code>services/platform/middleware/buyer/src/index.ts:184</code> | <code>export class BuyerMiddleware {</code> |
| <code>interface.8.ts-middleware-buyer-sidiora-layerx-buyer-middleware-buyersupportedkind</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/buyer/src/index.ts:49</code> | <code>export interface BuyerSupportedKind {</code> |
| <code>interface.8.ts-middleware-buyer-sidiora-layerx-buyer-middleware-journey</code> | <code>ts_export</code> | <code>change</code> | <code>services/platform/middleware/buyer/src/index.ts:118</code> | <code>export interface Journey {</code> |
| <code>interface.8.ts-middleware-buyer-sidiora-layerx-buyer-middleware-layerxpaymenthttptransport</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/buyer/src/index.ts:522</code> | <code>export class LayerXPaymentHttpTransport implements ProductionTransport {</code> |
| <code>interface.8.ts-middleware-buyer-sidiora-layerx-buyer-middleware-movequote</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/buyer/src/index.ts:90</code> | <code>export interface MoveQuote {</code> |
| <code>interface.8.ts-middleware-buyer-sidiora-layerx-buyer-middleware-paidfetchresult</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/buyer/src/index.ts:172</code> | <code>export type PaidFetchResult =</code> |
| <code>interface.8.ts-middleware-buyer-sidiora-layerx-buyer-middleware-parsedoffer</code> | <code>ts_export</code> | <code>change</code> | <code>services/platform/middleware/buyer/src/index.ts:74</code> | <code>export interface ParsedOffer {</code> |
| <code>interface.8.ts-middleware-buyer-sidiora-layerx-buyer-middleware-preparedpayment</code> | <code>ts_export</code> | <code>change</code> | <code>services/platform/middleware/buyer/src/index.ts:147</code> | <code>export interface PreparedPayment {</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.ts-middleware-buyer-index-platform-mw-buyer-j41</code> | notes | public item of its package; no in-repo caller outside the defining line found at HEAD |
| <code>interface.8.ts-middleware-buyer-sidiora-layerx-buyer-middleware-buyermiddleware</code> | overridden_by | override.J41.17 |
| <code>interface.8.ts-middleware-buyer-sidiora-layerx-buyer-middleware-journey</code> | overridden_by | override.J41.18 |
| <code>interface.8.ts-middleware-buyer-sidiora-layerx-buyer-middleware-parsedoffer</code> | overridden_by | override.J41.16 |
| <code>interface.8.ts-middleware-buyer-sidiora-layerx-buyer-middleware-preparedpayment</code> | overridden_by | override.J41.19 |

## middleware-conformance-env-vars

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.env-middleware-conformance-cc</code> | <code>env_var</code> | <code>retire</code> | <code>services/platform/middleware/conformance/pay6/generate_batch.mjs:20</code> | <code>run(process.env.CC ?? "gcc", [</code> |
| <code>interface.8.env-middleware-conformance-layerx-account</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/middleware/conformance/service.mjs:110</code> | <code>accountEnvironment: "LAYERX_ACCOUNT", currencyEnvironment: "LAYERX_CURRENCY",</code> |
| <code>interface.8.env-middleware-conformance-layerx-actor</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/middleware/conformance/connected.mjs:128</code> | <code>const response = await authenticated.transport.call({ plane: "agent", operation: "read.balance", request: { tenant: request.tenant, agent: process.env.LAYERX_ACTOR } });</code> |
| <code>interface.8.env-middleware-conformance-layerx-agent-rpc-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/middleware/conformance/connected.mjs:90</code> | <code>endpoint(process.env.LAYERX_AGENT_RPC_URL);</code> |
| <code>interface.8.env-middleware-conformance-layerx-asset</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/middleware/conformance/service.mjs:109</code> | <code>priceEnvironment: "LAYERX_PRICE", assetEnvironment: "LAYERX_ASSET", payToEnvironment: "LAYERX_PAY_TO",</code> |
| <code>interface.8.env-middleware-conformance-layerx-authorized-batch-json</code> | <code>env_var</code> | <code>retire</code> | <code>services/platform/middleware/conformance/service.mjs:111</code> | <code>authorizedBatchEnvironment: "LAYERX_AUTHORIZED_BATCH_JSON",</code> |
| <code>interface.8.env-middleware-conformance-layerx-connected-middleware-fixture</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/middleware/conformance/connected.mjs:82</code> | <code>const fixture = await privateJson(process.env.LAYERX_CONNECTED_MIDDLEWARE_FIXTURE);</code> |
| <code>interface.8.env-middleware-conformance-layerx-currency</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/middleware/conformance/service.mjs:110</code> | <code>accountEnvironment: "LAYERX_ACCOUNT", currencyEnvironment: "LAYERX_CURRENCY",</code> |
| <code>interface.8.env-middleware-conformance-layerx-example-endpoint</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/middleware/conformance/connected.mjs:99</code> | <code>assert.equal(process.env.LAYERX_EXAMPLE_ENDPOINT, undefined, "acceptance must use its declared service origins");</code> |
| <code>interface.8.env-middleware-conformance-layerx-pay-to</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/middleware/conformance/service.mjs:109</code> | <code>priceEnvironment: "LAYERX_PRICE", assetEnvironment: "LAYERX_ASSET", payToEnvironment: "LAYERX_PAY_TO",</code> |
| <code>interface.8.env-middleware-conformance-layerx-price</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/middleware/conformance/service.mjs:109</code> | <code>priceEnvironment: "LAYERX_PRICE", assetEnvironment: "LAYERX_ASSET", payToEnvironment: "LAYERX_PAY_TO",</code> |
| <code>interface.8.env-middleware-conformance-layerx-protocol-version</code> | <code>env_var</code> | <code>retire</code> | <code>services/platform/middleware/conformance/connected.mjs:97</code> | <code>assert.equal(selected.protocolVersion, Number(process.env.LAYERX_PROTOCOL_VERSION));</code> |
| <code>interface.8.env-middleware-conformance-layerx-settlement-token</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/middleware/conformance/service.mjs:262</code> | <code>tokenEnvironment: "LAYERX_SETTLEMENT_TOKEN", priceEnvironment: "LAYERX_PRICE",</code> |
| <code>interface.8.env-middleware-conformance-layerx-tenant</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/middleware/conformance/connected.mjs:89</code> | <code>assert.equal(request.tenant, process.env.LAYERX_TENANT);</code> |
| <code>interface.8.env-middleware-conformance-layerx-webhook-keys</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/middleware/conformance/service.mjs:265</code> | <code>webhookKeysEnvironment: "LAYERX_WEBHOOK_KEYS",</code> |
| <code>interface.8.env-middleware-conformance-path</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/middleware/conformance/pay6/examples.test.mjs:7</code> | <code>const env = { PATH: process.env.PATH, PYTHONPATH: root + "services/agent/sdk/python", LAYERX_RPC_URL: "&lt;url&gt;", LAYERX_DID: "did:lxp:example" };</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.env-middleware-conformance-cc</code> | notes | 531 callers total |
| <code>interface.8.env-middleware-conformance-cc</code> | overridden_by | override.J41.32 |
| <code>interface.8.env-middleware-conformance-layerx-authorized-batch-json</code> | overridden_by | override.J41.30 |
| <code>interface.8.env-middleware-conformance-layerx-protocol-version</code> | overridden_by | override.J41.31 |
| <code>interface.8.env-middleware-conformance-layerx-settlement-token</code> | notes | no in-repo setter or reader outside the defining file |
| <code>interface.8.env-middleware-conformance-layerx-webhook-keys</code> | notes | no in-repo setter or reader outside the defining file |
| <code>interface.8.env-middleware-conformance-path</code> | notes | system variable; callers not enumerated |

## middleware-conformance-environment

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.env-middleware-conformance-layerx-receipt-keys-json-j41</code> | <code>env_var</code> | <code>new</code> | <code>services/platform/middleware/conformance/service.mjs</code> (planned) | <code>LAYERX_RECEIPT_KEYS_JSON: JSON array of {keyId, firstHeight, lastHeight, publicKey} receipt key entries</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.env-middleware-conformance-layerx-receipt-keys-json-j41</code> | notes | planned replacement for LAYERX_AUTHORIZED_BATCH_JSON at services/platform/middleware/conformance/service.mjs:111 |

## middleware-conformance-http-routes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.http-middleware-conformance-any-agent</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/middleware/conformance/service.mjs:366</code> | <code>if (path === "/agent") {</code> |
| <code>interface.8.http-middleware-conformance-any-budget</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/middleware/conformance/service.mjs:431</code> | <code>if (path === "/budget") {</code> |
| <code>interface.8.http-middleware-conformance-any-receipt</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/middleware/conformance/service.mjs:425</code> | <code>if (path === "/receipt") {</code> |
| <code>interface.8.http-middleware-conformance-any-signer</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/middleware/conformance/service.mjs:420</code> | <code>if (path === "/signer") {</code> |
| <code>interface.8.http-middleware-conformance-post-settle</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/middleware/conformance/service.mjs:210</code> | <code>if (request.method !== "POST" &#124;&#124; request.url !== "/settle") {</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.http-middleware-conformance-any-agent</code> | notes | 54 callers total |
| <code>interface.8.http-middleware-conformance-any-receipt</code> | notes | 163 callers total |
| <code>interface.8.http-middleware-conformance-post-settle</code> | notes | 57 callers total |

## middleware-conformance-ts-exports

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.ts-middleware-conformance-receipts-buildsignedreceipt-j41</code> | <code>ts_export</code> | <code>change</code> | <code>services/platform/middleware/conformance/src/receipts.ts:86</code> | <code>export async function buildSignedReceipt(</code> |
| <code>interface.8.ts-middleware-conformance-receipts-buyerpayload-j41</code> | <code>ts_export</code> | <code>change</code> | <code>services/platform/middleware/conformance/src/receipts.ts:184</code> | <code>export function buyerPayload(offer: OfferFixture, receipt: SignedReceipt, key: string): PaymentPayload</code> |
| <code>interface.8.ts-middleware-conformance-receipts-conformance-protocol-version-j41</code> | <code>ts_export</code> | <code>retire</code> | <code>services/platform/middleware/conformance/src/receipts.ts:20</code> | <code>export const CONFORMANCE_PROTOCOL_VERSION: SelectableProtocolVersion = DEFAULT_PROTOCOL_VERSION;</code> |
| <code>interface.8.ts-middleware-conformance-receipts-conformancesequencer-j41</code> | <code>ts_export</code> | <code>change</code> | <code>services/platform/middleware/conformance/src/receipts.ts:31</code> | <code>export class ConformanceSequencer</code> |
| <code>interface.8.ts-middleware-conformance-receipts-encodebase64-j41</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/conformance/src/receipts.ts:295</code> | <code>export function encodeBase64(value: Uint8Array): string</code> |
| <code>interface.8.ts-middleware-conformance-receipts-fixedbytes-j41</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/conformance/src/receipts.ts:199</code> | <code>export function fixedBytes(seed: number): Uint8Array</code> |
| <code>interface.8.ts-middleware-conformance-receipts-offerfixture-2-j41</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/conformance/src/receipts.ts:162</code> | <code>export function offerFixture(payTo: Uint8Array, asset: Uint8Array, amount: bigint): OfferFixture</code> |
| <code>interface.8.ts-middleware-conformance-receipts-offerfixture-j41</code> | <code>ts_export</code> | <code>change</code> | <code>services/platform/middleware/conformance/src/receipts.ts:157</code> | <code>export interface OfferFixture</code> |
| <code>interface.8.ts-middleware-conformance-receipts-receiptfacts-j41</code> | <code>ts_export</code> | <code>change</code> | <code>services/platform/middleware/conformance/src/receipts.ts:61</code> | <code>export interface ReceiptFacts</code> |
| <code>interface.8.ts-middleware-conformance-receipts-signedreceipt-j41</code> | <code>ts_export</code> | <code>change</code> | <code>services/platform/middleware/conformance/src/receipts.ts:71</code> | <code>export interface SignedReceipt</code> |
| <code>interface.8.ts-middleware-conformance-receipts-tohex-j41</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/conformance/src/receipts.ts:303</code> | <code>export function toHex(value: Uint8Array): string</code> |
| <code>interface.8.ts-middleware-conformance-scenarios-runscenarios-j41</code> | <code>ts_export</code> | <code>change</code> | <code>services/platform/middleware/conformance/src/scenarios.ts:200</code> | <code>export async function runScenarios(): Promise&lt;Suite&gt;</code> |
| <code>interface.8.ts-middleware-conformance-service-assertserviceaccountpayto-j41</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/conformance/service.mjs:49</code> | <code>export async function assertServiceAccountPayTo(account, payTo)</code> |
| <code>interface.8.ts-middleware-conformance-service-runservicescenarios-j41</code> | <code>ts_export</code> | <code>change</code> | <code>services/platform/middleware/conformance/service.mjs:56</code> | <code>export async function runServiceScenarios(suite)</code> |
| <code>interface.8.ts-middleware-conformance-service-service-payee-account-j41</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/conformance/service.mjs:32</code> | <code>export const SERVICE_PAYEE_ACCOUNT = "agent:did:layerx:conformance-seller:main";</code> |
| <code>interface.8.ts-middleware-conformance-service-servicepayee-j41</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/conformance/service.mjs:42</code> | <code>export async function servicePayee(account)</code> |
| <code>interface.8.ts-middleware-conformance-support-assert-j41</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/conformance/src/support.ts:121</code> | <code>export function assert(condition: boolean, message: string): void</code> |
| <code>interface.8.ts-middleware-conformance-support-checkresult-j41</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/conformance/src/support.ts:81</code> | <code>export interface CheckResult</code> |
| <code>interface.8.ts-middleware-conformance-support-expectthrows-j41</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/conformance/src/support.ts:105</code> | <code>export async function expectThrows(</code> |
| <code>interface.8.ts-middleware-conformance-support-fixedbatchresolver-j41</code> | <code>ts_export</code> | <code>retire</code> | <code>services/platform/middleware/conformance/src/support.ts:12</code> | <code>export class FixedBatchResolver implements AuthorizedBatchResolver</code> |
| <code>interface.8.ts-middleware-conformance-support-fixedreceiptkeys-j41</code> | <code>ts_export</code> | <code>new</code> | <code>services/platform/middleware/conformance/src/support.ts</code> (planned) | <code>export class FixedReceiptKeys { resolve(): Promise&lt;readonly PxReceiptKeyEntry[]&gt; }</code> |
| <code>interface.8.ts-middleware-conformance-support-inmemorydeliverystore-j41</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/conformance/src/support.ts:51</code> | <code>export class InMemoryDeliveryStore implements WebhookDeliveryStore</code> |
| <code>interface.8.ts-middleware-conformance-support-inmemoryfulfillmentrepository-j41</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/conformance/src/support.ts:26</code> | <code>export class InMemoryFulfillmentRepository&lt;T&gt; implements FulfillmentRepository&lt;T&gt;</code> |
| <code>interface.8.ts-middleware-conformance-support-suite-j41</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/conformance/src/support.ts:88</code> | <code>export class Suite</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.ts-middleware-conformance-support-fixedreceiptkeys-j41</code> | notes | planned replacement for FixedBatchResolver at services/platform/middleware/conformance/src/support.ts:12 |

## middleware-examples-cli

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.cli-middleware-examples-public-rpc-mjs-receipt-keys-j41</code> | <code>cli_command</code> | <code>new</code> | <code>services/platform/middleware/examples/public-rpc.mjs</code> (planned) | <code>public-rpc.mjs --receipt-keys RECEIPT_KEYS_JSON_FILE</code> |
| <code>interface.8.cli-middleware-examples-public-rpc-py-receipt-keys-j41</code> | <code>cli_command</code> | <code>new</code> | <code>services/platform/middleware/examples/public_rpc.py</code> (planned) | <code>public_rpc.py --receipt-keys RECEIPT_KEYS_JSON_FILE</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.cli-middleware-examples-public-rpc-mjs-receipt-keys-j41</code> | notes | planned replacement for the --authority option of services/platform/middleware/examples/public-rpc.mjs:104 |
| <code>interface.8.cli-middleware-examples-public-rpc-py-receipt-keys-j41</code> | notes | planned replacement for --authority at services/platform/middleware/examples/public_rpc.py:96 |

## middleware-examples-cli-commands

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.cli-middleware-examples-public-rpc-py-activity</code> | <code>cli_command</code> | <code>change</code> | <code>services/platform/middleware/examples/public_rpc.py:94</code> | <code>parser.add_argument("--activity")</code> |
| <code>interface.8.cli-middleware-examples-public-rpc-py-authority</code> | <code>cli_command</code> | <code>retire</code> | <code>services/platform/middleware/examples/public_rpc.py:96</code> | <code>parser.add_argument("--authority")</code> |
| <code>interface.8.cli-middleware-examples-public-rpc-py-claim-key</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/middleware/examples/public_rpc.py:93</code> | <code>parser.add_argument("--claim-key")</code> |
| <code>interface.8.cli-middleware-examples-public-rpc-py-did</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/middleware/examples/public_rpc.py:90</code> | <code>parser.add_argument("--did", default=os.environ.get("LAYERX_DID"))</code> |
| <code>interface.8.cli-middleware-examples-public-rpc-py-faucet</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/middleware/examples/public_rpc.py:91</code> | <code>parser.add_argument("--faucet", default=os.environ.get("LAYERX_FAUCET_URL"))</code> |
| <code>interface.8.cli-middleware-examples-public-rpc-py-offer</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/middleware/examples/public_rpc.py:95</code> | <code>parser.add_argument("--offer")</code> |
| <code>interface.8.cli-middleware-examples-public-rpc-py-payer</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/middleware/examples/public_rpc.py:97</code> | <code>parser.add_argument("--payer")</code> |
| <code>interface.8.cli-middleware-examples-public-rpc-py-public-key</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/middleware/examples/public_rpc.py:92</code> | <code>parser.add_argument("--public-key")</code> |
| <code>interface.8.cli-middleware-examples-public-rpc-py-rpc</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/middleware/examples/public_rpc.py:89</code> | <code>parser.add_argument("--rpc", default=os.environ.get("LAYERX_RPC_URL"))</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.cli-middleware-examples-public-rpc-py-activity</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-middleware-examples-public-rpc-py-activity</code> | overridden_by | override.J41.33 |
| <code>interface.8.cli-middleware-examples-public-rpc-py-authority</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-middleware-examples-public-rpc-py-authority</code> | overridden_by | override.J41.34 |
| <code>interface.8.cli-middleware-examples-public-rpc-py-claim-key</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-middleware-examples-public-rpc-py-did</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-middleware-examples-public-rpc-py-faucet</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-middleware-examples-public-rpc-py-offer</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-middleware-examples-public-rpc-py-payer</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-middleware-examples-public-rpc-py-public-key</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-middleware-examples-public-rpc-py-rpc</code> | notes | no in-repo invocation outside the defining file |

## middleware-examples-env-vars

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.env-middleware-examples-layerx-did</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/middleware/examples/public-rpc.mjs:110</code> | <code>values.did ??= process.env.LAYERX_DID;</code> |
| <code>interface.8.env-middleware-examples-layerx-faucet-token</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/middleware/examples/public-rpc.mjs:144</code> | <code>if (!values["claim-key"] &#124;&#124; !process.env.LAYERX_FAUCET_TOKEN)</code> |
| <code>interface.8.env-middleware-examples-layerx-faucet-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/middleware/examples/examples-live-check.sh:75</code> | <code>refuse_persistent_chain LAYERX_FAUCET_URL "${LAYERX_FAUCET_URL:-}"</code> |
| <code>interface.8.env-middleware-examples-layerx-gateway-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/middleware/examples/examples-live-check.sh:74</code> | <code>refuse_persistent_chain LAYERX_GATEWAY_URL "${LAYERX_GATEWAY_URL:-}"</code> |
| <code>interface.8.env-middleware-examples-layerx-rpc-token</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/middleware/examples/public-rpc.mjs:124</code> | <code>process.env.LAYERX_RPC_TOKEN</code> |
| <code>interface.8.env-middleware-examples-layerx-rpc-url</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/middleware/examples/public-rpc.mjs:108</code> | <code>values.rpc ??= process.env.LAYERX_RPC_URL;</code> |
| <code>interface.8.env-middleware-examples-platform-beta-cluster-env</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/middleware/examples/examples-live-check.sh:60</code> | <code>env_file=${PLATFORM_BETA_CLUSTER_ENV:-build/beta-cluster/env}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.env-middleware-examples-layerx-gateway-url</code> | notes | 54 callers total |

## middleware-examples-http-routes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.http-middleware-examples-any-v1-faucet-claims</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/middleware/examples/public-rpc.mjs:135</code> | <code>url.pathname !== "/v1/faucet/claims" &#124;&#124;</code> |

## middleware-merchant-ts-exports

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.ts-middleware-merchant-index-cartline-j41</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/merchant/src/index.ts:39</code> | <code>export interface CartLine {</code> |
| <code>interface.8.ts-middleware-merchant-index-catalogitem-j41</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/merchant/src/index.ts:23</code> | <code>export interface CatalogItem {</code> |
| <code>interface.8.ts-middleware-merchant-index-catalogprovider-j41</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/merchant/src/index.ts:35</code> | <code>export interface CatalogProvider {</code> |
| <code>interface.8.ts-middleware-merchant-index-merchantcheckoutresult-j41</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/merchant/src/index.ts:107</code> | <code>export type MerchantCheckoutResult =</code> |
| <code>interface.8.ts-middleware-merchant-index-merchanterrorcode-j41</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/merchant/src/index.ts:380</code> | <code>export type MerchantErrorCode =</code> |
| <code>interface.8.ts-middleware-merchant-index-merchantmiddlewareconfig-j41</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/merchant/src/index.ts:99</code> | <code>export interface MerchantMiddlewareConfig {</code> |
| <code>interface.8.ts-middleware-merchant-index-merchantorderstate-j41</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/merchant/src/index.ts:61</code> | <code>export type MerchantOrderState = "awaiting-payment" &#124; "paid-verified" &#124; "refused";</code> |
| <code>interface.8.ts-middleware-merchant-index-merchantquote-j41</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/merchant/src/index.ts:50</code> | <code>export interface MerchantQuote {</code> |
| <code>interface.8.ts-middleware-merchant-index-merchantreceiptevidence-j41</code> | <code>ts_export</code> | <code>change</code> | <code>services/platform/middleware/merchant/src/index.ts:292</code> | <code>export interface MerchantReceiptEvidence {</code> |
| <code>interface.8.ts-middleware-merchant-index-merchantreceiptresolver-j41</code> | <code>ts_export</code> | <code>change</code> | <code>services/platform/middleware/merchant/src/index.ts:303</code> | <code>export interface MerchantReceiptResolver {</code> |
| <code>interface.8.ts-middleware-merchant-index-merchantsellerfactory-j41</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/merchant/src/index.ts:92</code> | <code>export interface MerchantSellerFactory {</code> |
| <code>interface.8.ts-middleware-merchant-index-merchantsettlementwebhooksconfig-j41</code> | <code>ts_export</code> | <code>change</code> | <code>services/platform/middleware/merchant/src/index.ts:307</code> | <code>export interface MerchantSettlementWebhooksConfig {</code> |
| <code>interface.8.ts-middleware-merchant-index-platform-mw-merchant-j41</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/merchant/src/index.ts:396</code> | <code>export function platform_mw_merchant(): "receipt-backed-merchant-checkout" {</code> |
| <code>interface.8.ts-middleware-merchant-index-quotedline-j41</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/merchant/src/index.ts:44</code> | <code>export interface QuotedLine extends CartLine {</code> |
| <code>interface.8.ts-middleware-merchant-index-settlementwebhookevent-j41</code> | <code>ts_export</code> | <code>change</code> | <code>services/platform/middleware/merchant/src/index.ts:283</code> | <code>export interface SettlementWebhookEvent {</code> |
| <code>interface.8.ts-middleware-merchant-sidiora-layerx-merchant-middleware-checkoutopenrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/merchant/src/index.ts:73</code> | <code>export interface CheckoutOpenRequest {</code> |
| <code>interface.8.ts-middleware-merchant-sidiora-layerx-merchant-middleware-merchanterror</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/merchant/src/index.ts:389</code> | <code>export class MerchantError extends Error {</code> |
| <code>interface.8.ts-middleware-merchant-sidiora-layerx-merchant-middleware-merchantmiddleware</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/merchant/src/index.ts:118</code> | <code>export class MerchantMiddleware {</code> |
| <code>interface.8.ts-middleware-merchant-sidiora-layerx-merchant-middleware-merchantorder</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/merchant/src/index.ts:63</code> | <code>export interface MerchantOrder {</code> |
| <code>interface.8.ts-middleware-merchant-sidiora-layerx-merchant-middleware-merchantorderstore</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/merchant/src/index.ts:79</code> | <code>export interface MerchantOrderStore {</code> |
| <code>interface.8.ts-middleware-merchant-sidiora-layerx-merchant-middleware-merchantsettlementwebhooks</code> | <code>ts_export</code> | <code>change</code> | <code>services/platform/middleware/merchant/src/index.ts:315</code> | <code>export class MerchantSettlementWebhooks {</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.ts-middleware-merchant-index-platform-mw-merchant-j41</code> | notes | public item of its package; no in-repo caller outside the defining line found at HEAD |
| <code>interface.8.ts-middleware-merchant-sidiora-layerx-merchant-middleware-merchantsettlementwebhooks</code> | overridden_by | override.J41.29 |

## middleware-seller-error-codes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.err-middleware-seller-payment-refused</code> | <code>error_code</code> | <code>stable</code> | <code>services/platform/middleware/seller/src/index.ts:691</code> | <code>const safeReason = /^[a-z][a-z0-9_]{0,63}$/u.test(reason) ? reason : "payment_refused";</code> |

## middleware-seller-http-routes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.http-middleware-seller-any-rpc</code> | <code>http_route</code> | <code>stable</code> | <code>services/platform/middleware/seller/src/rpc.ts:25</code> | <code>&#124;&#124; this.#url.pathname !== "/rpc"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.http-middleware-seller-any-rpc</code> | notes | 612 callers total |

## middleware-seller-ts-exports

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.ts-middleware-seller-checkpoint-rpccheckpointauthority-j41</code> | <code>ts_export</code> | <code>retire</code> | <code>services/platform/middleware/seller/src/checkpoint.ts:5</code> | <code>export interface RpcCheckpointAuthority {</code> |
| <code>interface.8.ts-middleware-seller-checkpoint-rpccheckpointevidence-j41</code> | <code>ts_export</code> | <code>retire</code> | <code>services/platform/middleware/seller/src/checkpoint.ts:30</code> | <code>export function rpcCheckpointEvidence(result: Record&lt;string, unknown&gt;, batch: PaymentCommitmentEvidence,</code> |
| <code>interface.8.ts-middleware-seller-commitment-max-payment-account-bytes-j41</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/seller/src/commitment.ts:67</code> | <code>export const MAX_PAYMENT_ACCOUNT_BYTES = 512;</code> |
| <code>interface.8.ts-middleware-seller-commitment-max-payment-currency-bytes-j41</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/seller/src/commitment.ts:69</code> | <code>export const MAX_PAYMENT_CURRENCY_BYTES = 16;</code> |
| <code>interface.8.ts-middleware-seller-commitment-max-payment-did-bytes-j41</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/seller/src/commitment.ts:68</code> | <code>export const MAX_PAYMENT_DID_BYTES = 255;</code> |
| <code>interface.8.ts-middleware-seller-commitment-paymentcommitmentevidence-j41</code> | <code>ts_export</code> | <code>change</code> | <code>services/platform/middleware/seller/src/commitment.ts:14</code> | <code>export interface PaymentCommitmentEvidence {</code> |
| <code>interface.8.ts-middleware-seller-commitment-verifypaymentcommitmentevidence-j41</code> | <code>ts_export</code> | <code>change</code> | <code>services/platform/middleware/seller/src/commitment.ts:108</code> | <code>export async function verifyPaymentCommitmentEvidence(</code> |
| <code>interface.8.ts-middleware-seller-grant-grantpaymentauthority-j41</code> | <code>ts_export</code> | <code>change</code> | <code>services/platform/middleware/seller/src/grant.ts:8</code> | <code>export class GrantPaymentAuthority implements SellerPaymentAuthority {</code> |
| <code>interface.8.ts-middleware-seller-index-platform-mw-seller-j41</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/seller/src/index.ts:314</code> | <code>export function platform_mw_seller(): "receipt-gated-x402-seller" {</code> |
| <code>interface.8.ts-middleware-seller-index-portablereceiptv2-j41</code> | <code>ts_export</code> | <code>new</code> | <code>services/platform/middleware/seller/src/index.ts</code> (planned) | <code>export interface PortableReceiptV2 { readonly format: "layerx-receipt-proof-v2"; ... }</code> |
| <code>interface.8.ts-middleware-seller-index-sellermiddlewareconfig-j41</code> | <code>ts_export</code> | <code>change</code> | <code>services/platform/middleware/seller/src/index.ts:151</code> | <code>export interface SellerMiddlewareConfig&lt;T&gt; {</code> |
| <code>interface.8.ts-middleware-seller-index-verifiedwebhookconsumerconfig-j41</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/seller/src/index.ts:405</code> | <code>export interface VerifiedWebhookConsumerConfig {</code> |
| <code>interface.8.ts-middleware-seller-index-x402extension-j41</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/seller/src/index.ts:42</code> | <code>export interface X402Extension {</code> |
| <code>interface.8.ts-middleware-seller-ledger-decodeledgertransferlog-j41</code> | <code>ts_export</code> | <code>new</code> | <code>services/platform/middleware/seller/src/ledger.ts</code> (planned) | <code>export function decodeLedgerTransferLog(log: LedgerLog): LedgerTransferEvent</code> |
| <code>interface.8.ts-middleware-seller-ledger-decodepaymentdrawnlog-j41</code> | <code>ts_export</code> | <code>new</code> | <code>services/platform/middleware/seller/src/ledger.ts</code> (planned) | <code>export function decodePaymentDrawnLog(log: LedgerLog): PaymentDrawnEvent</code> |
| <code>interface.8.ts-middleware-seller-ledger-decodepaymentexactlog-j41</code> | <code>ts_export</code> | <code>new</code> | <code>services/platform/middleware/seller/src/ledger.ts</code> (planned) | <code>export function decodePaymentExactLog(log: LedgerLog): PaymentExactEvent</code> |
| <code>interface.8.ts-middleware-seller-ledger-encodeledgerdraw-j41</code> | <code>ts_export</code> | <code>new</code> | <code>services/platform/middleware/seller/src/ledger.ts</code> (planned) | <code>export function encodeLedgerDraw(draw: LedgerDraw): Uint8Array</code> |
| <code>interface.8.ts-middleware-seller-ledger-layerx-ledger-address-j41</code> | <code>ts_export</code> | <code>new</code> | <code>services/platform/middleware/seller/src/ledger.ts</code> (planned) | <code>export const LAYERX_LEDGER_ADDRESS = "0x000000000000000000000000000000000000101a";</code> |
| <code>interface.8.ts-middleware-seller-rpc-paymentrpc-j41</code> | <code>ts_export</code> | <code>change</code> | <code>services/platform/middleware/seller/src/rpc.ts:19</code> | <code>export class PaymentRpc {</code> |
| <code>interface.8.ts-middleware-seller-rpc-paymentrpcerror-j41</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/seller/src/rpc.ts:15</code> | <code>export class PaymentRpcError extends Error {</code> |
| <code>interface.8.ts-middleware-seller-rpc-rpcbatchevidence-j41</code> | <code>ts_export</code> | <code>retire</code> | <code>services/platform/middleware/seller/src/rpc.ts:80</code> | <code>export function rpcBatchEvidence(result: Record&lt;string, unknown&gt;, activityId: string, receipt: Uint8Array,</code> |
| <code>interface.8.ts-middleware-seller-rpc-rpchex-j41</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/seller/src/rpc.ts:9</code> | <code>export function rpcHex(value: unknown, size?: number): Uint8Array {</code> |
| <code>interface.8.ts-middleware-seller-rpc-rpcobject-j41</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/seller/src/rpc.ts:4</code> | <code>export function rpcObject(value: unknown): Record&lt;string, unknown&gt; {</code> |
| <code>interface.8.ts-middleware-seller-rpc-verifyrpcpayment-j41</code> | <code>ts_export</code> | <code>change</code> | <code>services/platform/middleware/seller/src/rpc.ts:62</code> | <code>export async function verifyRpcPayment(</code> |
| <code>interface.8.ts-middleware-seller-sidiora-layerx-seller-middleware-authorizedbatchresolver</code> | <code>ts_export</code> | <code>retire</code> | <code>services/platform/middleware/seller/src/index.ts:111</code> | <code>export interface AuthorizedBatchResolver {</code> |
| <code>interface.8.ts-middleware-seller-sidiora-layerx-seller-middleware-decodepaymentpayloadheader</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/seller/src/index.ts:330</code> | <code>export function decodePaymentPayloadHeader(value: string): PaymentPayload {</code> |
| <code>interface.8.ts-middleware-seller-sidiora-layerx-seller-middleware-decodepaymentrequiredheader</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/seller/src/index.ts:322</code> | <code>export function decodePaymentRequiredHeader(value: string): PaymentRequired {</code> |
| <code>interface.8.ts-middleware-seller-sidiora-layerx-seller-middleware-decodesettlementheader</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/seller/src/index.ts:338</code> | <code>export function decodeSettlementHeader(value: string): SettlementResponse {</code> |
| <code>interface.8.ts-middleware-seller-sidiora-layerx-seller-middleware-encodepaymentpayloadheader</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/seller/src/index.ts:326</code> | <code>export function encodePaymentPayloadHeader(value: PaymentPayload): string {</code> |
| <code>interface.8.ts-middleware-seller-sidiora-layerx-seller-middleware-encodepaymentrequiredheader</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/seller/src/index.ts:318</code> | <code>export function encodePaymentRequiredHeader(value: PaymentRequired): string {</code> |
| <code>interface.8.ts-middleware-seller-sidiora-layerx-seller-middleware-encodesettlementheader</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/seller/src/index.ts:334</code> | <code>export function encodeSettlementHeader(value: SettlementResponse): string {</code> |
| <code>interface.8.ts-middleware-seller-sidiora-layerx-seller-middleware-fulfillmentrepository</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/seller/src/index.ts:144</code> | <code>export interface FulfillmentRepository&lt;T&gt; {</code> |
| <code>interface.8.ts-middleware-seller-sidiora-layerx-seller-middleware-grantdrawexecution</code> | <code>ts_export</code> | <code>change</code> | <code>services/platform/middleware/seller/src/grant.ts:3</code> | <code>export interface GrantDrawExecution {</code> |
| <code>interface.8.ts-middleware-seller-sidiora-layerx-seller-middleware-jsonvalue</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/seller/src/index.ts:31</code> | <code>export type JsonValue = null &#124; boolean &#124; number &#124; string &#124; JsonValue[] &#124; { readonly [key: string]: JsonValue };</code> |
| <code>interface.8.ts-middleware-seller-sidiora-layerx-seller-middleware-layerxreceiptevidence</code> | <code>ts_export</code> | <code>change</code> | <code>services/platform/middleware/seller/src/index.ts:73</code> | <code>export interface LayerXReceiptEvidence {</code> |
| <code>interface.8.ts-middleware-seller-sidiora-layerx-seller-middleware-middlewareerror</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/seller/src/index.ts:104</code> | <code>export class MiddlewareError extends Error {</code> |
| <code>interface.8.ts-middleware-seller-sidiora-layerx-seller-middleware-middlewareerrorcode</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/seller/src/index.ts:90</code> | <code>export type MiddlewareErrorCode =</code> |
| <code>interface.8.ts-middleware-seller-sidiora-layerx-seller-middleware-payment-required-header</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/seller/src/index.ts:22</code> | <code>export const PAYMENT_REQUIRED_HEADER = "PAYMENT-REQUIRED" as const;</code> |
| <code>interface.8.ts-middleware-seller-sidiora-layerx-seller-middleware-payment-response-header</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/seller/src/index.ts:24</code> | <code>export const PAYMENT_RESPONSE_HEADER = "PAYMENT-RESPONSE" as const;</code> |
| <code>interface.8.ts-middleware-seller-sidiora-layerx-seller-middleware-payment-signature-header</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/seller/src/index.ts:23</code> | <code>export const PAYMENT_SIGNATURE_HEADER = "PAYMENT-SIGNATURE" as const;</code> |
| <code>interface.8.ts-middleware-seller-sidiora-layerx-seller-middleware-paymentaccount</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/seller/src/commitment.ts:71</code> | <code>export function paymentAccount(extra: unknown, required = false): string &#124; undefined {</code> |
| <code>interface.8.ts-middleware-seller-sidiora-layerx-seller-middleware-paymentcommitment</code> | <code>ts_export</code> | <code>change</code> | <code>services/platform/middleware/seller/src/commitment.ts:12</code> | <code>export type PaymentCommitment = "executed" &#124; "batched" &#124; "finalised";</code> |
| <code>interface.8.ts-middleware-seller-sidiora-layerx-seller-middleware-paymentcommitment-2</code> | <code>ts_export</code> | <code>change</code> | <code>services/platform/middleware/seller/src/commitment.ts:35</code> | <code>export function paymentCommitment(extra: unknown): PaymentCommitment {</code> |
| <code>interface.8.ts-middleware-seller-sidiora-layerx-seller-middleware-paymentcommitmentresolver</code> | <code>ts_export</code> | <code>change</code> | <code>services/platform/middleware/seller/src/commitment.ts:27</code> | <code>export interface PaymentCommitmentResolver {</code> |
| <code>interface.8.ts-middleware-seller-sidiora-layerx-seller-middleware-paymentcurrency</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/seller/src/commitment.ts:81</code> | <code>export function paymentCurrency(extra: unknown, required = false): string &#124; undefined {</code> |
| <code>interface.8.ts-middleware-seller-sidiora-layerx-seller-middleware-paymentpayer</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/seller/src/commitment.ts:53</code> | <code>export function paymentPayer(extra: unknown, required = false): string &#124; undefined {</code> |
| <code>interface.8.ts-middleware-seller-sidiora-layerx-seller-middleware-paymentpayload</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/seller/src/index.ts:65</code> | <code>export interface PaymentPayload {</code> |
| <code>interface.8.ts-middleware-seller-sidiora-layerx-seller-middleware-paymentpurpose</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/seller/src/commitment.ts:60</code> | <code>export function paymentPurpose(extra: unknown, required = false): string &#124; undefined {</code> |
| <code>interface.8.ts-middleware-seller-sidiora-layerx-seller-middleware-paymentrequired</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/seller/src/index.ts:57</code> | <code>export interface PaymentRequired {</code> |
| <code>interface.8.ts-middleware-seller-sidiora-layerx-seller-middleware-paymentrequirements</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/seller/src/index.ts:47</code> | <code>export interface PaymentRequirements {</code> |
| <code>interface.8.ts-middleware-seller-sidiora-layerx-seller-middleware-protocolselection</code> | <code>ts_export</code> | <code>retire</code> | <code>services/platform/middleware/seller/src/index.ts:166</code> | <code>export function protocolSelection(value: unknown): ProtocolSelection {</code> |
| <code>interface.8.ts-middleware-seller-sidiora-layerx-seller-middleware-receiptpayloadauthority</code> | <code>ts_export</code> | <code>change</code> | <code>services/platform/middleware/seller/src/index.ts:193</code> | <code>export class ReceiptPayloadAuthority implements SellerPaymentAuthority {</code> |
| <code>interface.8.ts-middleware-seller-sidiora-layerx-seller-middleware-requireprotocolversion</code> | <code>ts_export</code> | <code>retire</code> | <code>services/platform/middleware/seller/src/index.ts:159</code> | <code>export function requireProtocolVersion(value: unknown): SelectableProtocolVersion {</code> |
| <code>interface.8.ts-middleware-seller-sidiora-layerx-seller-middleware-resourceinfo</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/seller/src/index.ts:33</code> | <code>export interface ResourceInfo {</code> |
| <code>interface.8.ts-middleware-seller-sidiora-layerx-seller-middleware-sellerdecision</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/seller/src/index.ts:170</code> | <code>export type SellerDecision&lt;T&gt; =</code> |
| <code>interface.8.ts-middleware-seller-sidiora-layerx-seller-middleware-sellermiddleware</code> | <code>ts_export</code> | <code>change</code> | <code>services/platform/middleware/seller/src/index.ts:208</code> | <code>export class SellerMiddleware&lt;T&gt; {</code> |
| <code>interface.8.ts-middleware-seller-sidiora-layerx-seller-middleware-sellerpaymentauthority</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/seller/src/index.ts:132</code> | <code>export interface SellerPaymentAuthority {</code> |
| <code>interface.8.ts-middleware-seller-sidiora-layerx-seller-middleware-sellersettlementoutcome</code> | <code>ts_export</code> | <code>change</code> | <code>services/platform/middleware/seller/src/index.ts:123</code> | <code>export type SellerSettlementOutcome =</code> |
| <code>interface.8.ts-middleware-seller-sidiora-layerx-seller-middleware-sellersettlementrequest</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/seller/src/index.ts:115</code> | <code>export interface SellerSettlementRequest {</code> |
| <code>interface.8.ts-middleware-seller-sidiora-layerx-seller-middleware-settlementresponse</code> | <code>ts_export</code> | <code>change</code> | <code>services/platform/middleware/seller/src/index.ts:80</code> | <code>export interface SettlementResponse {</code> |
| <code>interface.8.ts-middleware-seller-sidiora-layerx-seller-middleware-storedfulfillment</code> | <code>ts_export</code> | <code>change</code> | <code>services/platform/middleware/seller/src/index.ts:136</code> | <code>export interface StoredFulfillment&lt;T&gt; {</code> |
| <code>interface.8.ts-middleware-seller-sidiora-layerx-seller-middleware-verifiedwebhookconsumer</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/seller/src/index.ts:413</code> | <code>export class VerifiedWebhookConsumer {</code> |
| <code>interface.8.ts-middleware-seller-sidiora-layerx-seller-middleware-verifypaymentcommitment</code> | <code>ts_export</code> | <code>change</code> | <code>services/platform/middleware/seller/src/commitment.ts:94</code> | <code>export async function verifyPaymentCommitment(</code> |
| <code>interface.8.ts-middleware-seller-sidiora-layerx-seller-middleware-verifypaymentreceipt</code> | <code>ts_export</code> | <code>change</code> | <code>services/platform/middleware/seller/src/index.ts:342</code> | <code>export async function verifyPaymentReceipt(</code> |
| <code>interface.8.ts-middleware-seller-sidiora-layerx-seller-middleware-webhookclaimresult</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/seller/src/index.ts:395</code> | <code>export type WebhookClaimResult = "claimed" &#124; "processing" &#124; "completed" &#124; "conflict";</code> |
| <code>interface.8.ts-middleware-seller-sidiora-layerx-seller-middleware-webhookconsumeresult</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/seller/src/index.ts:403</code> | <code>export type WebhookConsumeResult = "processed" &#124; "duplicate" &#124; "processing";</code> |
| <code>interface.8.ts-middleware-seller-sidiora-layerx-seller-middleware-webhookdeliveryclaim</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/seller/src/index.ts:389</code> | <code>export interface WebhookDeliveryClaim {</code> |
| <code>interface.8.ts-middleware-seller-sidiora-layerx-seller-middleware-webhookdeliverystore</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/seller/src/index.ts:397</code> | <code>export interface WebhookDeliveryStore {</code> |
| <code>interface.8.ts-middleware-seller-sidiora-layerx-seller-middleware-webhookrequestheaders</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/seller/src/index.ts:382</code> | <code>export interface WebhookRequestHeaders {</code> |
| <code>interface.8.ts-middleware-seller-sidiora-layerx-seller-middleware-x402-version</code> | <code>ts_export</code> | <code>stable</code> | <code>services/platform/middleware/seller/src/index.ts:21</code> | <code>export const X402_VERSION = 2 as const;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.ts-middleware-seller-grant-grantpaymentauthority-j41</code> | notes | public item of its package; no in-repo caller outside the defining line found at HEAD |
| <code>interface.8.ts-middleware-seller-index-platform-mw-seller-j41</code> | notes | public item of its package; no in-repo caller outside the defining line found at HEAD |
| <code>interface.8.ts-middleware-seller-index-portablereceiptv2-j41</code> | notes | planned export of src/index.ts; the file exists at HEAD so line is 0 until written |
| <code>interface.8.ts-middleware-seller-sidiora-layerx-seller-middleware-authorizedbatchresolver</code> | overridden_by | override.J41.15 |
| <code>interface.8.ts-middleware-seller-sidiora-layerx-seller-middleware-grantdrawexecution</code> | overridden_by | override.J41.14 |
| <code>interface.8.ts-middleware-seller-sidiora-layerx-seller-middleware-layerxreceiptevidence</code> | overridden_by | override.J41.3 |
| <code>interface.8.ts-middleware-seller-sidiora-layerx-seller-middleware-paymentcommitment</code> | overridden_by | override.J41.1 |
| <code>interface.8.ts-middleware-seller-sidiora-layerx-seller-middleware-paymentcommitment-2</code> | overridden_by | override.J41.2 |
| <code>interface.8.ts-middleware-seller-sidiora-layerx-seller-middleware-paymentcommitmentresolver</code> | overridden_by | override.J41.8 |
| <code>interface.8.ts-middleware-seller-sidiora-layerx-seller-middleware-protocolselection</code> | overridden_by | override.J41.4 |
| <code>interface.8.ts-middleware-seller-sidiora-layerx-seller-middleware-receiptpayloadauthority</code> | overridden_by | override.J41.5 |
| <code>interface.8.ts-middleware-seller-sidiora-layerx-seller-middleware-requireprotocolversion</code> | overridden_by | override.J41.6 |
| <code>interface.8.ts-middleware-seller-sidiora-layerx-seller-middleware-sellermiddleware</code> | overridden_by | override.J41.10 |
| <code>interface.8.ts-middleware-seller-sidiora-layerx-seller-middleware-sellersettlementoutcome</code> | overridden_by | override.J41.11 |
| <code>interface.8.ts-middleware-seller-sidiora-layerx-seller-middleware-settlementresponse</code> | overridden_by | override.J41.12 |
| <code>interface.8.ts-middleware-seller-sidiora-layerx-seller-middleware-storedfulfillment</code> | overridden_by | override.J41.13 |
| <code>interface.8.ts-middleware-seller-sidiora-layerx-seller-middleware-verifypaymentcommitment</code> | overridden_by | override.J41.9 |
| <code>interface.8.ts-middleware-seller-sidiora-layerx-seller-middleware-verifypaymentreceipt</code> | overridden_by | override.J41.7 |

Contract context <code>contract_context.8.x402-portable-receipt-j41</code>:

| Field | Frozen rule |
|---|---|
| interface | Seller, buyer, agent and merchant middleware exchange PortableReceiptV2 evidence on eip155 networks with commitment receipted or committed. |
| storage | none on chain; merchant order stores keep tx_hash and receiptDigest per order. |
| errors | MiddlewareError codes unsupported-payment, verification-failure, invalid-payment-required. |
| events | PaymentExact and PaymentDrawn logs of the 0x101A ledger bind amount, assetId, to and paymentReference. |
| gas | not applicable to the middleware; transactions carry the payer's EIP-1559 fee fields. |
| activation | effective when the merged node serves pax_sequencerReceipt and 0x101A, anchor task 8.0.3. |
| invariants | a receipt verifies only under a pinned key valid at its height and binds exactly one PaymentExact log equal to the accepted requirements; a committed rung needs a commit of more than 2/3 of voting power. |
| pause | none; operators withdraw an offer by serving no PAYMENT-REQUIRED header. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.8.248.1</code> | req | 8248.1 |
| <code>acceptance.8.248.1</code> | scenario | Given the built seller package, when the test imports its entry module, then the keys rpcBatchEvidence and rpcCheckpointEvidence are absent from the namespace. |
| <code>acceptance.8.248.1</code> | test | services/platform/middleware/seller/test/retired-exports.test.mjs::packageEntryHasNoKernelBatchEvidenceExports (planned) |
| <code>acceptance.8.248.1</code> | qualification | UNRUN |
| <code>req.8248</code> | ac_1 | The built package entry exports neither rpcBatchEvidence nor rpcCheckpointEvidence, which HEAD exports through services/platform/middleware/seller/src/index.ts:976-978. |
| <code>acceptance.8.248.2</code> | req | 8248.2 |
| <code>acceptance.8.248.2</code> | scenario | Given the built seller package, when the test imports its entry module, then verifyPaymentReceipt, verifyRpcPayment and decodePaymentExactLog are functions. |
| <code>acceptance.8.248.2</code> | test | services/platform/middleware/seller/test/retired-exports.test.mjs::packageEntryKeepsLedgerAndReceiptExports (planned) |
| <code>acceptance.8.248.2</code> | qualification | UNRUN |
| <code>req.8248</code> | ac_2 | The built package entry still exports verifyPaymentReceipt, verifyRpcPayment and decodePaymentExactLog after the retirement. |
| <code>capability.8.22.6</code> | title | Retire the kernel batch, checkpoint and authorized-batch seller exports |
| <code>capability.8.22.6</code> | reqs | 8248.1<br>8248.2 |
