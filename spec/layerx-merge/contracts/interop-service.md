# Interop service contract

The [interop service](../../../services/interop/crates/layerx-interop-service/src/lib.rs) serves authenticated protocol operations through `layerx-interop-gateway`. It journals idempotent operations in Redis, relays payer-signed EVM transactions and verifies signed receipt evidence before settlement.

## cfg-svc-adapter-pin-conformance-sha256

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-svc-adapter-pin-conformance-sha256</code> | <code>config_key</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>interop manifest adapters[] conformance_sha256: String (required)</code> |

## cfg-svc-adapter-pin-conformance-suite

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-svc-adapter-pin-conformance-suite</code> | <code>config_key</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>interop manifest adapters[] conformance_suite: String (required)</code> |

## cfg-svc-adapter-pin-conformance-vectors

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-svc-adapter-pin-conformance-vectors</code> | <code>config_key</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>interop manifest adapters[] conformance_vectors: u64 (required)</code> |

## cfg-svc-adapter-pin-evidence-policy

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-svc-adapter-pin-evidence-policy</code> | <code>config_key</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>interop manifest adapters[] evidence_policy: String (required)</code> |

## cfg-svc-adapter-pin-id

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-svc-adapter-pin-id</code> | <code>config_key</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>interop manifest adapters[] id: String (required)</code> |

## cfg-svc-adapter-pin-specification

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-svc-adapter-pin-specification</code> | <code>config_key</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>interop manifest adapters[] specification: String (required)</code> |

## cfg-svc-adapter-pin-specification-sha256

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-svc-adapter-pin-specification-sha256</code> | <code>config_key</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>interop manifest adapters[] specification_sha256: String (required)</code> |

## cfg-svc-adapter-pin-version

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-svc-adapter-pin-version</code> | <code>config_key</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>interop manifest adapters[] version: String (required)</code> |

## cfg-svc-ap2-asset-binding-asset

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-svc-ap2-asset-binding-asset</code> | <code>config_key</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>interop manifest ap2_assets[] asset: String (required)</code> |

## cfg-svc-ap2-asset-binding-atomic-units-per-minor-unit

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-svc-ap2-asset-binding-atomic-units-per-minor-unit</code> | <code>config_key</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>interop manifest ap2_assets[] atomic_units_per_minor_unit: String (required)</code> |

## cfg-svc-ap2-asset-binding-audience

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-svc-ap2-asset-binding-audience</code> | <code>config_key</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>interop manifest ap2_assets[] audience: String (required)</code> |

## cfg-svc-ap2-asset-binding-currency

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-svc-ap2-asset-binding-currency</code> | <code>config_key</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>interop manifest ap2_assets[] currency: String (required)</code> |

## cfg-svc-ap2-asset-binding-minor-unit-exponent

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-svc-ap2-asset-binding-minor-unit-exponent</code> | <code>config_key</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>interop manifest ap2_assets[] minor_unit_exponent: u8 (required)</code> |

## cfg-svc-ap2-asset-binding-payee-account

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-svc-ap2-asset-binding-payee-account</code> | <code>config_key</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>interop manifest ap2_assets[] payee_account: String (required)</code> |

## cfg-svc-ap2-asset-binding-payee-merchant-id

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-svc-ap2-asset-binding-payee-merchant-id</code> | <code>config_key</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>interop manifest ap2_assets[] payee_merchant_id: String (required)</code> |

## cfg-svc-ap2-asset-binding-payee-merchant-name

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-svc-ap2-asset-binding-payee-merchant-name</code> | <code>config_key</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>interop manifest ap2_assets[] payee_merchant_name: String (required)</code> |

## cfg-svc-ap2-asset-binding-payee-merchant-website

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-svc-ap2-asset-binding-payee-merchant-website</code> | <code>config_key</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>interop manifest ap2_assets[] payee_merchant_website: Option&lt;String&gt; (optional)</code> |

## cfg-svc-ap2-asset-binding-payer-account

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-svc-ap2-asset-binding-payer-account</code> | <code>config_key</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>interop manifest ap2_assets[] payer_account: String (required)</code> |

## cfg-svc-ap2-asset-binding-principal-digest

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-svc-ap2-asset-binding-principal-digest</code> | <code>config_key</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>interop manifest ap2_assets[] principal_digest: String (required)</code> |

## cfg-svc-ap2-key-pin-key-id

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-svc-ap2-key-pin-key-id</code> | <code>config_key</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>interop manifest ap2_keys[] key_id: String (required)</code> |

## cfg-svc-ap2-key-pin-public-key-sec1

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-svc-ap2-key-pin-public-key-sec1</code> | <code>config_key</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>interop manifest ap2_keys[] public_key_sec1: String (required)</code> |

## cfg-svc-ap2-key-pin-use-case

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-svc-ap2-key-pin-use-case</code> | <code>config_key</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>interop manifest ap2_keys[] use_case: String (required)</code> |

## cfg-svc-fiat-provider-pin-provider

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-svc-fiat-provider-pin-provider</code> | <code>config_key</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>interop manifest fiat_providers[] provider: String (required)</code> |

## cfg-svc-fiat-provider-pin-public-key-ed25519

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-svc-fiat-provider-pin-public-key-ed25519</code> | <code>config_key</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>interop manifest fiat_providers[] public_key_ed25519: String (required)</code> |

## cfg-svc-manifest-file-adapters

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-svc-manifest-file-adapters</code> | <code>config_key</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>interop manifest adapters: Vec&lt;AdapterPin&gt; (required)</code> |

## cfg-svc-manifest-file-ap2-assets

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-svc-manifest-file-ap2-assets</code> | <code>config_key</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>interop manifest ap2_assets: Vec&lt;Ap2AssetBinding&gt; (required)</code> |

## cfg-svc-manifest-file-ap2-keys

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-svc-manifest-file-ap2-keys</code> | <code>config_key</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>interop manifest ap2_keys: Vec&lt;Ap2KeyPin&gt; (required)</code> |

## cfg-svc-manifest-file-fiat-providers

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-svc-manifest-file-fiat-providers</code> | <code>config_key</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>interop manifest fiat_providers: Vec&lt;FiatProviderPin&gt; (required)</code> |

## cfg-svc-manifest-file-transports

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-svc-manifest-file-transports</code> | <code>config_key</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>interop manifest transports: Vec&lt;TransportPin&gt; (required)</code> |

## cfg-svc-manifest-file-ucp-payment-handler

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-svc-manifest-file-ucp-payment-handler</code> | <code>config_key</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>interop manifest ucp_payment_handler: UcpHandlerPin (required)</code> |

## cfg-svc-manifest-file-visa-agents

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-svc-manifest-file-visa-agents</code> | <code>config_key</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>interop manifest visa_agents: Vec&lt;VisaAgentPin&gt; (required)</code> |

## cfg-svc-manifest-file-visa-targets

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-svc-manifest-file-visa-targets</code> | <code>config_key</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>interop manifest visa_targets: Vec&lt;VisaTargetPin&gt; (required)</code> |

## cfg-svc-manifest-file-x402-supported

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-svc-manifest-file-x402-supported</code> | <code>config_key</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>interop manifest x402_supported: SupportedResponse (required)</code> |

## cfg-svc-migration-v2-file-ethereum

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-svc-migration-v2-file-ethereum</code> | <code>config_key</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>interop migration-v2 config ethereum: Option&lt;EthereumConfig&gt; (optional)</code> |

## cfg-svc-migration-v2-file-history-journal

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-svc-migration-v2-file-history-journal</code> | <code>config_key</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>interop migration-v2 config history_journal: Option&lt;JournalConfig&gt; (optional)</code> |

## cfg-svc-migration-v2-file-mapping-journal

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-svc-migration-v2-file-mapping-journal</code> | <code>config_key</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>interop migration-v2 config mapping_journal: JournalConfig (required)</code> |

## cfg-svc-migration-v2-file-paxeer-binding

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-svc-migration-v2-file-paxeer-binding</code> | <code>config_key</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>interop migration-v2 config paxeer_binding: PaxeerBindingConfigV2 (required)</code> |

## cfg-svc-migration-v2-file-ramp-intake

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-svc-migration-v2-file-ramp-intake</code> | <code>config_key</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>interop migration-v2 config ramp_intake: Option&lt;RampIntakeV2File&gt; (optional)</code> |

## cfg-svc-migration-v2-file-solana

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-svc-migration-v2-file-solana</code> | <code>config_key</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>interop migration-v2 config solana: Option&lt;SolanaConfig&gt; (optional)</code> |

## cfg-svc-module-declaration-module

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-svc-module-declaration-module</code> | <code>config_key</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>interop module registry modules[] module: u16 (required)</code> |

## cfg-svc-module-declaration-ordinals

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-svc-module-declaration-ordinals</code> | <code>config_key</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>interop module registry modules[] ordinals: Vec&lt;u16&gt; (required)</code> |

## cfg-svc-module-file-modules

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-svc-module-file-modules</code> | <code>config_key</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>interop module registry modules: Vec&lt;ModuleDeclaration&gt; (required)</code> |

## cfg-svc-ramp-intake-v2-file-endpoint

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-svc-ramp-intake-v2-file-endpoint</code> | <code>config_key</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>interop migration-v2 ramp_intake endpoint: String (required)</code> |

## cfg-svc-ramp-intake-v2-file-token-file

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-svc-ramp-intake-v2-file-token-file</code> | <code>config_key</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>interop migration-v2 ramp_intake token_file: PathBuf (required)</code> |

## cfg-svc-transport-pin-conformance-sha256

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-svc-transport-pin-conformance-sha256</code> | <code>config_key</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>interop manifest transport conformance_sha256: String (required)</code> |

## cfg-svc-transport-pin-id

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-svc-transport-pin-id</code> | <code>config_key</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>interop manifest transport id: String (required)</code> |

## cfg-svc-transport-pin-specification-sha256

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-svc-transport-pin-specification-sha256</code> | <code>config_key</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>interop manifest transport specification_sha256: String (required)</code> |

## cfg-svc-transport-pin-version

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-svc-transport-pin-version</code> | <code>config_key</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>interop manifest transport version: String (required)</code> |

## cfg-svc-ucp-handler-pin-id

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-svc-ucp-handler-pin-id</code> | <code>config_key</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>interop manifest ucp_payment_handler id: String (required)</code> |

## cfg-svc-ucp-handler-pin-schema

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-svc-ucp-handler-pin-schema</code> | <code>config_key</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>interop manifest ucp_payment_handler schema: String (required)</code> |

## cfg-svc-ucp-handler-pin-spec

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-svc-ucp-handler-pin-spec</code> | <code>config_key</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>interop manifest ucp_payment_handler spec: String (required)</code> |

## cfg-svc-ucp-handler-pin-version

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-svc-ucp-handler-pin-version</code> | <code>config_key</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>interop manifest ucp_payment_handler version: String (required)</code> |

## cfg-svc-visa-agent-pin-agent-domain

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-svc-visa-agent-pin-agent-domain</code> | <code>config_key</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>interop manifest visa_agents[] agent_domain: String (required)</code> |

## cfg-svc-visa-agent-pin-agent-id

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-svc-visa-agent-pin-agent-id</code> | <code>config_key</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>interop manifest visa_agents[] agent_id: String (required)</code> |

## cfg-svc-visa-agent-pin-algorithm

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-svc-visa-agent-pin-algorithm</code> | <code>config_key</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>interop manifest visa_agents[] algorithm: String (required)</code> |

## cfg-svc-visa-agent-pin-expires-at

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-svc-visa-agent-pin-expires-at</code> | <code>config_key</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>interop manifest visa_agents[] expires_at: u64 (required)</code> |

## cfg-svc-visa-agent-pin-key-id

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-svc-visa-agent-pin-key-id</code> | <code>config_key</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>interop manifest visa_agents[] key_id: String (required)</code> |

## cfg-svc-visa-agent-pin-layerx-agent

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-svc-visa-agent-pin-layerx-agent</code> | <code>config_key</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>interop manifest visa_agents[] layerx_agent: String (required)</code> |

## cfg-svc-visa-agent-pin-public-key

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-svc-visa-agent-pin-public-key</code> | <code>config_key</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>interop manifest visa_agents[] public_key: String (required)</code> |

## cfg-svc-visa-agent-pin-status

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-svc-visa-agent-pin-status</code> | <code>config_key</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>interop manifest visa_agents[] status: String (required)</code> |

## cfg-svc-visa-target-pin-authority

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-svc-visa-target-pin-authority</code> | <code>config_key</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>interop manifest visa_targets[] authority: String (required)</code> |

## cfg-svc-visa-target-pin-path

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-svc-visa-target-pin-path</code> | <code>config_key</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>interop manifest visa_targets[] path: String (required)</code> |

## cfg-svc-visa-target-pin-principal-digest

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-svc-visa-target-pin-principal-digest</code> | <code>config_key</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>interop manifest visa_targets[] principal_digest: String (required)</code> |

## cli-svc-layerx-interop-gateway

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cli-svc-layerx-interop-gateway</code> | <code>cli_command</code> | [services/interop/crates/layerx-interop-service/src/main.rs](../../../services/interop/crates/layerx-interop-service/src/main.rs) | <code>layerx-interop-gateway (no arguments)</code> |

## env-svc-layerx-interop-allow-hosted-private-network

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.env-svc-layerx-interop-allow-hosted-private-network</code> | <code>env_var</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>LAYERX_INTEROP_ALLOW_RAILWAY_PRIVATE_NETWORK: 0&#124;1; allows the plain-HTTP node endpoint on the hosted platform private network; optional, default 0</code> |

## env-svc-layerx-interop-chain-id

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.env-svc-layerx-interop-chain-id</code> | <code>env_var</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>LAYERX_INTEROP_CHAIN_ID</code> |

## env-svc-layerx-interop-client-identity-password-file

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.env-svc-layerx-interop-client-identity-password-file</code> | <code>env_var</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>LAYERX_INTEROP_CLIENT_IDENTITY_PASSWORD_FILE</code> |

## env-svc-layerx-interop-client-identity-pkcs12

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.env-svc-layerx-interop-client-identity-pkcs12</code> | <code>env_var</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>LAYERX_INTEROP_CLIENT_IDENTITY_PKCS12: path to the outbound mTLS client identity PKCS#12 (&lt;= 128 KiB); required</code> |

## env-svc-layerx-interop-config

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.env-svc-layerx-interop-config</code> | <code>env_var</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>LAYERX_INTEROP_CONFIG</code> |

## env-svc-layerx-interop-idempotency-seconds

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.env-svc-layerx-interop-idempotency-seconds</code> | <code>env_var</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>LAYERX_INTEROP_IDEMPOTENCY_SECONDS</code> |

## env-svc-layerx-interop-listen

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.env-svc-layerx-interop-listen</code> | <code>env_var</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>LAYERX_INTEROP_LISTEN</code> |

## env-svc-layerx-interop-listener

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.env-svc-layerx-interop-listener</code> | <code>env_var</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>LAYERX_INTEROP_LISTENER</code> |

## env-svc-layerx-interop-migration-v2-config

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.env-svc-layerx-interop-migration-v2-config</code> | <code>env_var</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>LAYERX_INTEROP_MIGRATION_V2_CONFIG</code> |

## env-svc-layerx-interop-module-registry-file

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.env-svc-layerx-interop-module-registry-file</code> | <code>env_var</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>LAYERX_INTEROP_MODULE_REGISTRY_FILE</code> |

## env-svc-layerx-interop-network-id

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.env-svc-layerx-interop-network-id</code> | <code>env_var</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>LAYERX_INTEROP_NETWORK_ID: network label (&lt;= 128 bytes); required</code> |

## env-svc-layerx-interop-outbound-ca-der

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.env-svc-layerx-interop-outbound-ca-der</code> | <code>env_var</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>LAYERX_INTEROP_OUTBOUND_CA_DER: path to the DER CA certificate for outbound TLS (&lt;= 64 KiB); required</code> |

## env-svc-layerx-interop-protocol-network-id

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.env-svc-layerx-interop-protocol-network-id</code> | <code>env_var</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>LAYERX_INTEROP_PROTOCOL_NETWORK_ID</code> |

## env-svc-layerx-interop-readiness-max-age-ms

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.env-svc-layerx-interop-readiness-max-age-ms</code> | <code>env_var</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>LAYERX_INTEROP_READINESS_MAX_AGE_MS</code> |

## env-svc-layerx-interop-redis-password-file

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.env-svc-layerx-interop-redis-password-file</code> | <code>env_var</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>LAYERX_INTEROP_REDIS_PASSWORD_FILE</code> |

## env-svc-layerx-interop-redis-url

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.env-svc-layerx-interop-redis-url</code> | <code>env_var</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>LAYERX_INTEROP_REDIS_URL</code> |

## env-svc-layerx-interop-redis-username-file

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.env-svc-layerx-interop-redis-username-file</code> | <code>env_var</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>LAYERX_INTEROP_REDIS_USERNAME_FILE</code> |

## env-svc-layerx-interop-tap-clock-skew-seconds

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.env-svc-layerx-interop-tap-clock-skew-seconds</code> | <code>env_var</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>LAYERX_INTEROP_TAP_CLOCK_SKEW_SECONDS</code> |

## env-svc-layerx-interop-tls-cert-der

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.env-svc-layerx-interop-tls-cert-der</code> | <code>env_var</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>LAYERX_INTEROP_TLS_CERT_DER</code> |

## env-svc-layerx-interop-tls-key-der

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.env-svc-layerx-interop-tls-key-der</code> | <code>env_var</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>LAYERX_INTEROP_TLS_KEY_DER</code> |

## env-svc-layerx-interop-wire-version

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.env-svc-layerx-interop-wire-version</code> | <code>env_var</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>LAYERX_INTEROP_WIRE_VERSION: numeric protocol version using occupancy, supplied as a label of at most 64 bytes; required</code> |

## err-svc-account-binding-invalid

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-svc-account-binding-invalid</code> | <code>error_code</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>HTTP 400 {"ok":false,"operation":..,"error":{"code":"account_binding_invalid"},"trace":..}</code> |

## err-svc-adapter-configuration-invalid

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-svc-adapter-configuration-invalid</code> | <code>error_code</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>HTTP 503 {"ok":false,"operation":..,"error":{"code":"adapter_configuration_invalid"},"trace":..}</code> |

## err-svc-api-key-required

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-svc-api-key-required</code> | <code>error_code</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>HTTP 401 {"ok":false,"operation":..,"error":{"code":"api_key_required"},"trace":..}</code> |

## err-svc-asset-binding-ambiguous

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-svc-asset-binding-ambiguous</code> | <code>error_code</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>HTTP 400 {"ok":false,"operation":..,"error":{"code":"asset_binding_ambiguous"},"trace":..}</code> |

## err-svc-asset-binding-invalid

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-svc-asset-binding-invalid</code> | <code>error_code</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>HTTP 400 {"ok":false,"operation":..,"error":{"code":"asset_binding_invalid"},"trace":..}</code> |

## err-svc-authenticated-principal-invalid

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-svc-authenticated-principal-invalid</code> | <code>error_code</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>HTTP 503 {"ok":false,"operation":..,"error":{"code":"authenticated_principal_invalid"},"trace":..}</code> |

## err-svc-authenticated-signer-invalid

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-svc-authenticated-signer-invalid</code> | <code>error_code</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>HTTP 503 {"ok":false,"operation":..,"error":{"code":"authenticated_signer_invalid"},"trace":..}</code> |

## err-svc-clock-unavailable

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-svc-clock-unavailable</code> | <code>error_code</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>HTTP 503 {"ok":false,"operation":..,"error":{"code":"clock_unavailable"},"trace":..}</code> |

## err-svc-continuation-invalid

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-svc-continuation-invalid</code> | <code>error_code</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>HTTP 503 {"ok":false,"operation":..,"error":{"code":"continuation_invalid"},"trace":..}</code> |

## err-svc-idempotency-conflict

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-svc-idempotency-conflict</code> | <code>error_code</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>HTTP 409 {"ok":false,"operation":..,"error":{"code":"idempotency_conflict"},"trace":..}</code> |

## err-svc-idempotency-key-required

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-svc-idempotency-key-required</code> | <code>error_code</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>HTTP 400 {"ok":false,"operation":..,"error":{"code":"idempotency_key_required"},"trace":..}</code> |

## err-svc-invalid-ap2-request

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-svc-invalid-ap2-request</code> | <code>error_code</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>HTTP 400 {"ok":false,"operation":..,"error":{"code":"invalid_ap2_request"},"trace":..}</code> |

## err-svc-invalid-evidence

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-svc-invalid-evidence</code> | <code>error_code</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>HTTP 400 {"ok":false,"operation":..,"error":{"code":"invalid_evidence"},"trace":..}</code> |

## err-svc-invalid-history

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-svc-invalid-history</code> | <code>error_code</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>HTTP 400 {"ok":false,"operation":..,"error":{"code":"invalid_history"},"trace":..}</code> |

## err-svc-invalid-migration-request

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-svc-invalid-migration-request</code> | <code>error_code</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>HTTP 400 {"ok":false,"operation":..,"error":{"code":"invalid_migration_request"},"trace":..}</code> |

## err-svc-invalid-provider-callback

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-svc-invalid-provider-callback</code> | <code>error_code</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>HTTP 400 {"ok":false,"operation":..,"error":{"code":"invalid_provider_callback"},"trace":..}</code> |

## err-svc-invalid-ucp-request

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-svc-invalid-ucp-request</code> | <code>error_code</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>HTTP 400 {"ok":false,"operation":..,"error":{"code":"invalid_ucp_request"},"trace":..}</code> |

## err-svc-invalid-visa-tap-request

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-svc-invalid-visa-tap-request</code> | <code>error_code</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>HTTP 400 {"ok":false,"operation":..,"error":{"code":"invalid_visa_tap_request"},"trace":..}</code> |

## err-svc-invalid-x402-buyer-request

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-svc-invalid-x402-buyer-request</code> | <code>error_code</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>HTTP 400 {"ok":false,"operation":..,"error":{"code":"invalid_x402_buyer_request"},"trace":..}</code> |

## err-svc-invalid-x402-offer

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-svc-invalid-x402-offer</code> | <code>error_code</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>HTTP 400 {"ok":false,"operation":..,"error":{"code":"invalid_x402_offer"},"trace":..}</code> |

## err-svc-invalid-x402-request

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-svc-invalid-x402-request</code> | <code>error_code</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>HTTP 400 {"ok":false,"operation":..,"error":{"code":"invalid_x402_request"},"trace":..}</code> |

## err-svc-layerx-agent-binding-required

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-svc-layerx-agent-binding-required</code> | <code>error_code</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>HTTP 400 {"ok":false,"operation":..,"error":{"code":"layerx_agent_binding_required"},"trace":..}</code> |

## err-svc-mandate-merchant-mismatch

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-svc-mandate-merchant-mismatch</code> | <code>error_code</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>HTTP 400 {"ok":false,"operation":..,"error":{"code":"mandate_merchant_mismatch"},"trace":..}</code> |

## err-svc-mandate-verification-refused

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-svc-mandate-verification-refused</code> | <code>error_code</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>HTTP 400 {"ok":false,"operation":..,"error":{"code":"mandate_verification_refused"},"trace":..}</code> |

## err-svc-mapping-encoding-invalid

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-svc-mapping-encoding-invalid</code> | <code>error_code</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>HTTP 503 {"ok":false,"operation":..,"error":{"code":"mapping_encoding_invalid"},"trace":..}</code> |

## err-svc-migration-encoding-invalid

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-svc-migration-encoding-invalid</code> | <code>error_code</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>HTTP 503 {"ok":false,"operation":..,"error":{"code":"migration_encoding_invalid"},"trace":..}</code> |

## err-svc-migration-history-unconfigured

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-svc-migration-history-unconfigured</code> | <code>error_code</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>HTTP 503 {"ok":false,"operation":..,"error":{"code":"migration_history_unconfigured"},"trace":..}</code> |

## err-svc-migration-ramp-unavailable

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-svc-migration-ramp-unavailable</code> | <code>error_code</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>HTTP 503 {"ok":false,"operation":..,"error":{"code":"migration_ramp_unavailable"},"trace":..}</code> |

## err-svc-migration-ramp-unconfigured

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-svc-migration-ramp-unconfigured</code> | <code>error_code</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>HTTP 503 {"ok":false,"operation":..,"error":{"code":"migration_ramp_unconfigured"},"trace":..}</code> |

## err-svc-migration-response-invalid

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-svc-migration-response-invalid</code> | <code>error_code</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>HTTP 503 {"ok":false,"operation":..,"error":{"code":"migration_response_invalid"},"trace":..}</code> |

## err-svc-migration-response-mismatch

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-svc-migration-response-mismatch</code> | <code>error_code</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>HTTP 503 {"ok":false,"operation":..,"error":{"code":"migration_response_mismatch"},"trace":..}</code> |

## err-svc-migration-source-unconfigured

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-svc-migration-source-unconfigured</code> | <code>error_code</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>HTTP 503 {"ok":false,"operation":..,"error":{"code":"migration_source_unconfigured"},"trace":..}</code> |

## err-svc-migration-v2-unconfigured

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-svc-migration-v2-unconfigured</code> | <code>error_code</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>HTTP 503 {"ok":false,"operation":..,"error":{"code":"migration_v2_unconfigured"},"trace":..}</code> |

## err-svc-not-found

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-svc-not-found</code> | <code>error_code</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>HTTP 404 {"ok":false,"operation":..,"error":{"code":"not_found"},"trace":..}</code> |

## err-svc-operation-identity-invalid

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-svc-operation-identity-invalid</code> | <code>error_code</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>HTTP 503 {"ok":false,"operation":..,"error":{"code":"operation_identity_invalid"},"trace":..}</code> |

## err-svc-operation-not-found

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-svc-operation-not-found</code> | <code>error_code</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>HTTP 404 {"ok":false,"operation":..,"error":{"code":"operation_not_found"},"trace":..}</code> |

## err-svc-persistence-unavailable

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-svc-persistence-unavailable</code> | <code>error_code</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>HTTP 503 {"ok":false,"operation":..,"error":{"code":"persistence_unavailable"},"trace":..}</code> |

## err-svc-protocol-idempotency-required

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-svc-protocol-idempotency-required</code> | <code>error_code</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>HTTP 400 {"ok":false,"operation":..,"error":{"code":"protocol_idempotency_required"},"trace":..}</code> |

## err-svc-provider-callback-refused

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-svc-provider-callback-refused</code> | <code>error_code</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>HTTP 400 {"ok":false,"operation":..,"error":{"code":"provider_callback_refused"},"trace":..}</code> |

## err-svc-quota-exceeded

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-svc-quota-exceeded</code> | <code>error_code</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>HTTP 429 {"ok":false,"operation":..,"error":{"code":"quota_exceeded"},"trace":..}</code> |

## err-svc-receipt-intent-mismatch

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-svc-receipt-intent-mismatch</code> | <code>error_code</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>HTTP 503 {"ok":false,"operation":..,"error":{"code":"receipt_intent_mismatch"},"trace":..}</code> |

## err-svc-receipt-verification-failed

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-svc-receipt-verification-failed</code> | <code>error_code</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>HTTP 503 {"ok":false,"operation":..,"error":{"code":"receipt_verification_failed"},"trace":..}</code> |

## err-svc-request-too-large

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-svc-request-too-large</code> | <code>error_code</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>HTTP 400 {"ok":false,"operation":..,"error":{"code":"request_too_large"},"trace":..}</code> |

## err-svc-server-clock-unavailable

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-svc-server-clock-unavailable</code> | <code>error_code</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>HTTP 503 {"ok":false,"operation":..,"error":{"code":"server_clock_unavailable"},"trace":..}</code> |

## err-svc-settlement-unavailable

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-svc-settlement-unavailable</code> | <code>error_code</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>HTTP 503 {"ok":false,"operation":..,"error":{"code":"settlement_unavailable"},"trace":..}</code> |

## err-svc-typed-intent-required

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-svc-typed-intent-required</code> | <code>error_code</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>HTTP 400 {"ok":false,"operation":..,"error":{"code":"typed_intent_required"},"trace":..}</code> |

## err-svc-ucp-amount-invalid

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-svc-ucp-amount-invalid</code> | <code>error_code</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>HTTP 400 {"ok":false,"operation":..,"error":{"code":"ucp_amount_invalid"},"trace":..}</code> |

## err-svc-ucp-asset-invalid

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-svc-ucp-asset-invalid</code> | <code>error_code</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>HTTP 400 {"ok":false,"operation":..,"error":{"code":"ucp_asset_invalid"},"trace":..}</code> |

## err-svc-ucp-capability-refused

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-svc-ucp-capability-refused</code> | <code>error_code</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>HTTP 400 {"ok":false,"operation":..,"error":{"code":"ucp_capability_refused"},"trace":..}</code> |

## err-svc-ucp-currency-invalid

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-svc-ucp-currency-invalid</code> | <code>error_code</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>HTTP 400 {"ok":false,"operation":..,"error":{"code":"ucp_currency_invalid"},"trace":..}</code> |

## err-svc-ucp-idempotency-invalid

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-svc-ucp-idempotency-invalid</code> | <code>error_code</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>HTTP 400 {"ok":false,"operation":..,"error":{"code":"ucp_idempotency_invalid"},"trace":..}</code> |

## err-svc-ucp-order-invalid

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-svc-ucp-order-invalid</code> | <code>error_code</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>HTTP 400 {"ok":false,"operation":..,"error":{"code":"ucp_order_invalid"},"trace":..}</code> |

## err-svc-ucp-profile-refused

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-svc-ucp-profile-refused</code> | <code>error_code</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>HTTP 400 {"ok":false,"operation":..,"error":{"code":"ucp_profile_refused"},"trace":..}</code> |

## err-svc-ucp-recipient-invalid

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-svc-ucp-recipient-invalid</code> | <code>error_code</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>HTTP 400 {"ok":false,"operation":..,"error":{"code":"ucp_recipient_invalid"},"trace":..}</code> |

## err-svc-unsupported-x402-offer

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-svc-unsupported-x402-offer</code> | <code>error_code</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>HTTP 400 {"ok":false,"operation":..,"error":{"code":"unsupported_x402_offer"},"trace":..}</code> |

## err-svc-untrusted-identity-header

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-svc-untrusted-identity-header</code> | <code>error_code</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>HTTP 400 {"ok":false,"operation":..,"error":{"code":"untrusted_identity_header"},"trace":..}</code> |

## err-svc-visa-tap-refused

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-svc-visa-tap-refused</code> | <code>error_code</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>HTTP 400 {"ok":false,"operation":..,"error":{"code":"visa_tap_refused"},"trace":..}</code> |

## err-svc-visa-tap-replayed

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-svc-visa-tap-replayed</code> | <code>error_code</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>HTTP 409 {"ok":false,"operation":..,"error":{"code":"visa_tap_replayed"},"trace":..}</code> |

## err-svc-visa-tap-target-mismatch

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-svc-visa-tap-target-mismatch</code> | <code>error_code</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>HTTP 400 {"ok":false,"operation":..,"error":{"code":"visa_tap_target_mismatch"},"trace":..}</code> |

## err-svc-visa-tap-target-unavailable

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-svc-visa-tap-target-unavailable</code> | <code>error_code</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>HTTP 400 {"ok":false,"operation":..,"error":{"code":"visa_tap_target_unavailable"},"trace":..}</code> |

## err-svc-x402-buyer-refused

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-svc-x402-buyer-refused</code> | <code>error_code</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>HTTP 400 {"ok":false,"operation":..,"error":{"code":"x402_buyer_refused"},"trace":..}</code> |

## err-svc-x402-offer-encoding-refused

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-svc-x402-offer-encoding-refused</code> | <code>error_code</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>HTTP 400 {"ok":false,"operation":..,"error":{"code":"x402_offer_encoding_refused"},"trace":..}</code> |

## fmt-svc-ap2-request

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-svc-ap2-request</code> | <code>file_format</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>Rust model: #[serde(deny_unknown_fields)] struct Ap2Request { checkout_presentation: String, payment_presentation: String, nonce: String, #[serde(default)] activity: String, }</code> |

## fmt-svc-buyer-request

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-svc-buyer-request</code> | <code>file_format</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>Rust model: #[serde(deny_unknown_fields)] struct BuyerRequest { payment_required: PaymentRequired, scheme_payload: Value, }</code> |

## fmt-svc-durable-continuation

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-svc-durable-continuation</code> | <code>file_format</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>Rust model: #[serde(deny_unknown_fields)] struct DurableContinuation { method: String, path: String, content_type: String, idempotency_key: Option&lt;String&gt;, body: String, }</code> |

## fmt-svc-fiat-callback

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-svc-fiat-callback</code> | <code>file_format</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>Rust model: #[serde(deny_unknown_fields)] struct FiatCallback { token_reference: String, evidence: FiatEvidenceEnvelope, #[serde(default)] activity: String, }</code> |

## fmt-svc-fiat-evidence-envelope

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-svc-fiat-evidence-envelope</code> | <code>file_format</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>Rust model: #[serde(deny_unknown_fields)] struct FiatEvidenceEnvelope { facts: FiatFacts, signature: String, }</code> |

## fmt-svc-fiat-facts

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-svc-fiat-facts</code> | <code>file_format</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>Rust model: #[serde(deny_unknown_fields)] struct FiatFacts { provider: String, settlement: String, token_reference_sha256: String, rail: String, class: String, amount: String, asset: String, destination: String, observed_at: u64, hold_until: Option&lt;u64&gt;, }</code> |

## fmt-svc-migration-account-request-v2

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-svc-migration-account-request-v2</code> | <code>file_format</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>Rust model: #[serde(deny_unknown_fields)] struct MigrationAccountRequestV2 { chain: MigrationSourceV2, source_evidence: String, }</code> |

## fmt-svc-migration-asset-request-v2

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-svc-migration-asset-request-v2</code> | <code>file_format</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>Rust model: #[serde(deny_unknown_fields)] struct MigrationAssetRequestV2 { order_digest: [u8; 32], chain: MigrationSourceV2, source_evidence: String, }</code> |

## fmt-svc-migration-history-import-request-v2

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-svc-migration-history-import-request-v2</code> | <code>file_format</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>Rust model: #[serde(deny_unknown_fields)] struct MigrationHistoryImportRequestV2 { chain: MigrationSourceV2, ownership_evidence: String, source_evidence: String, }</code> |

## fmt-svc-migration-history-read-request-v2

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-svc-migration-history-read-request-v2</code> | <code>file_format</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>Rust model: #[serde(deny_unknown_fields)] struct MigrationHistoryReadRequestV2 { cursor: Option&lt;String&gt;, limit: usize, }</code> |

## fmt-svc-migration-source-v2

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-svc-migration-source-v2</code> | <code>file_format</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>MigrationSourceV2 = ethereum &#124; solana</code> |

## fmt-svc-migration-source-v2-labels

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-svc-migration-source-v2-labels</code> | <code>file_format</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>MigrationSourceV2::label() -&gt; "ethereum" &#124; "solana"</code> |

## fmt-svc-ucp-capability-wire

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-svc-ucp-capability-wire</code> | <code>file_format</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>Rust model: #[serde(deny_unknown_fields)] struct UcpCapabilityWire { name: String, version: String, spec: String, schema: String, }</code> |

## fmt-svc-ucp-handler-wire

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-svc-ucp-handler-wire</code> | <code>file_format</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>Rust model: #[serde(deny_unknown_fields)] struct UcpHandlerWire { id: String, version: String, spec: String, schema: String, }</code> |

## fmt-svc-ucp-request

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-svc-ucp-request</code> | <code>file_format</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>Rust model: #[serde(deny_unknown_fields)] struct UcpRequest { checkout_id: String, currency: String, total_minor: String, asset: String, recipient: String, idempotency_key: String, profile_url: String, capabilities: Vec&lt;UcpCapabilityWire&gt;, payment_handlers: Vec&lt;UcpHandlerWire&gt;, activity: String, order_id: String, permalink_url: String, }</code> |

## fmt-svc-visa-request

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-svc-visa-request</code> | <code>file_format</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>Rust model: #[serde(deny_unknown_fields)] struct VisaRequest { authority: String, path: String, signature_input: String, signature: String, #[serde(default)] activity: String, }</code> |

## http-svc-invalid-http-request

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.http-svc-invalid-http-request</code> | <code>http_route</code> | [services/interop/crates/layerx-interop-service/src/main.rs](../../../services/interop/crates/layerx-interop-service/src/main.rs) | <code>any unparsable request -&gt; 400 {"ok":false,"error":{"code":"invalid_http_request"}}</code> |

## http-svc-out-dependency-readyz

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.http-svc-out-dependency-readyz</code> | <code>http_route</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>outbound GET /readyz</code> |

## http-svc-out-ramp-source-settlements

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.http-svc-out-ramp-source-settlements</code> | <code>http_route</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>outbound POST /internal/v2/source-settlements</code> |

## service-chain-relay

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-svc-transaction-refusals-j47</code> | <code>error_code</code> | [services/interop/crates/layerx-interop-service/src/payer_tx.rs](../../../services/interop/crates/layerx-interop-service/src/payer_tx.rs) | <code>transaction_malformed &#124; transaction_target_refused &#124; transaction_selector_refused &#124; transaction_signature_refused &#124; transaction_replacement_refused</code> |
| <code>interface.11.fmt-svc-layerx-transaction-payload-j47</code> | <code>file_format</code> | [services/interop/crates/layerx-interop-service/src/payer_tx.rs](../../../services/interop/crates/layerx-interop-service/src/payer_tx.rs) | <code>payload.layerxTransaction: 0x hex of a payer-signed EIP-1559 payExact transaction to the layerxledger precompile</code> |

| Field | Rule |
|---|---|
| interface | The interop service relays payer-signed transactions to node JSON-RPC and reads authenticated receipt states. |
| storage | Redis journal keyed by idempotency key holding hash and raw bytes. |
| errors | transaction_malformed, transaction_target_refused, transaction_selector_refused, transaction_signature_refused, transaction_replacement_refused. |
| events | None. |
| gas | Paid by the payer. |
| invariants | A retry never resubmits different bytes under one key; receipts verify under the pinned keys before use. |
| pause | Readiness answers 503 when the node receipt-key list cannot be read. |

## service-config

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.env-svc-layerx-interop-receipt-keys-file-j47</code> | <code>env_var</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>LAYERX_INTEROP_RECEIPT_KEYS_FILE</code> |
| <code>interface.11.env-svc-layerx-interop-sequencer-url-j47</code> | <code>env_var</code> | [services/interop/crates/layerx-interop-service/src/config.rs](../../../services/interop/crates/layerx-interop-service/src/config.rs) | <code>LAYERX_INTEROP_SEQUENCER_URL</code> |

| Field | Rule |
|---|---|
| interface | The interop service relays payer-signed transactions to node JSON-RPC and reads authenticated receipt states. |
| storage | Redis journal keyed by idempotency key holding hash and raw bytes. |
| errors | transaction_malformed, transaction_target_refused, transaction_selector_refused, transaction_signature_refused, transaction_replacement_refused. |
| events | None. |
| gas | Paid by the payer. |
| invariants | A retry never resubmits different bytes under one key; receipts verify under the pinned keys before use. |
| pause | Readiness answers 503 when the node receipt-key list cannot be read. |

## service-readiness

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-svc-readyz-sequencer-component-j47</code> | <code>file_format</code> | [services/interop/crates/layerx-interop-service/src/server.rs](../../../services/interop/crates/layerx-interop-service/src/server.rs) | <code>/readyz components.sequencer: string enum ("ready" or "unavailable")</code> |

| Field | Rule |
|---|---|
| interface | The interop service relays payer-signed transactions to node JSON-RPC and reads authenticated receipt states. |
| storage | Redis journal keyed by idempotency key holding hash and raw bytes. |
| errors | transaction_malformed, transaction_target_refused, transaction_selector_refused, transaction_signature_refused, transaction_replacement_refused. |
| events | None. |
| gas | Paid by the payer. |
| invariants | A retry never resubmits different bytes under one key; receipts verify under the pinned keys before use. |
| pause | Readiness answers 503 when the node receipt-key list cannot be read. |

## Service configuration and receipt handling

Run `layerx-interop-gateway` with no arguments. Configure these active environment
inputs, all with the `LAYERX_INTEROP_` prefix:

| Inputs | Required value |
|---|---|
| `CONFIG` | Manifest JSON containing the pinned adapters, transports and protocol trust policy |
| `LISTEN`, `LISTENER` | Socket address and `plain` or TLS listener mode; TLS requires `TLS_CERT_DER` and `TLS_KEY_DER` together |
| `SEQUENCER_URL` | Node JSON-RPC origin; HTTPS by default; a private Railway HTTP endpoint additionally requires `ALLOW_RAILWAY_PRIVATE_NETWORK=1` |
| `RECEIPT_KEYS_FILE` | Nonempty JSON array of `{keyId, publicKey, firstHeight, lastHeight}`; 0x-prefixed 32-byte identifiers and keys, inclusive nonoverlapping numeric height windows; startup compares every pin with `px_getReceiptKeys` |
| `OUTBOUND_CA_DER`, `CLIENT_IDENTITY_PKCS12`, `CLIENT_IDENTITY_PASSWORD_FILE` | Outbound CA, mTLS identity and private password file |
| `REDIS_URL`, `REDIS_USERNAME_FILE`, `REDIS_PASSWORD_FILE` | TLS Redis endpoint and private credential files |
| `NETWORK_ID`, `WIRE_VERSION`, `PROTOCOL_NETWORK_ID` | Network label, occupancy protocol version (2 or 3), nonzero protocol network identifier |
| `CHAIN_ID`, `READINESS_MAX_AGE_MS`, `MODULE_REGISTRY_FILE`, `TAP_CLOCK_SKEW_SECONDS` | 125; 1..=30000; module registry JSON; 0..=300 seconds |
| `AP2_RECEIPT_SIGNERS_FILE` (required for AP2 execution/readiness) | Private (mode 0600 or stricter), at most 4096-byte JSON file; `payment` and `checkout` each contain `issuer`, `key_id`, `private_key_hex` (32-byte ES256 private scalar, lowercase hexadecimal without 0x); pin each issuer public key independently at the consumer |
| `IDEMPOTENCY_SECONDS` (optional) | 1..=2592000 seconds, default 604800; also bounds finalized relay raw-byte retention |

Optional migration configuration is separate from the protocol adapter manifest.

Read `GET /readyz` with `operations::ReadinessDocument::from_json` and
`is_ready()`: the component values are exactly the strings `ready` or
`unavailable`; ready HTTP status is 200, degraded status is 503. For payment
proofs, obtain a concrete `pax_sequencerReceipt` response and its original signed
transaction, export with `layerx_ap2::PortableLayerXEvidence::from_sequencer_receipt`,
then pass its JSON and independently pinned `SequencerAuthorization` entries to
`operations::verify_receipted_json`. This verifies PortableReceiptV2 evidence offline. A node-reported committed rung or matching block hash does not
supply validator-commit or receipt-batch inclusion evidence: hosted exports stay
`receipted` until that complete proof is verified.

The relay keeps unresolved signed transactions without expiry. After verifying
the signed receipt/logs and observing a matching committed block, it atomically
creates a permanent 32-byte transaction-hash tombstone and sets the raw record
and submission-progress TTL to the gateway replay window. Repeated finality
observations do not extend that window. After compaction, recovery reads evidence
by hash and refuses unavailable evidence without resubmitting. Tombstones never
expire, including when gateway operation records have expired.

Payment expiry uses Unix milliseconds and refuses `expires_at <= payment_now`.
AP2 mandate and Visa credential NumericDate checks continue using Unix seconds.

Hosted `POST /v1/http/ap2/execute` retains the AP2 library's authorized mandate
binding and `SignedAp2Evidence`/`ReceiptSigner` boundary on the EVM relay path.
A successful result includes `payment_receipt_jwt`, `checkout_receipt_jwt` and
`layerx` (PortableReceiptV2 with the original signed payer transaction). Each JWT
binds the original verified mandate receipt reference, the canonical receipt
digest and the same portable document. The gateway persists this entire result
before replying; restarted replay returns the saved bytes without another
submission or a new signature. Missing signer configuration refuses execution
with `ap2_receipt_signers_unconfigured` before submission and makes AP2 readiness
unavailable. Invalid configured signing material refuses startup. The hosted service uses the public AP2 authorization, signer and portable APIs.
