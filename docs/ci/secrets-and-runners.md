# Workflow secrets, variables and runners

This inventory covers the public checks in [`.github/workflows`](../../.github/workflows/).
It records configuration names, their readers, their purpose and the role responsible
for provisioning them. It contains no credential values or private operational endpoints.
Workflow filenames in the tables link to the checks that read each name.

Repository administrators install repository or organization Actions secrets and
variables supplied by the responsible role below. For jobs with an `environment`,
that environment's administrator provisions the protected settings: `migration-testnets`
for migration, `mirror-testnets` for mirrors, `paxeer-x-qualification` for qualification,
and `devnet` or `testnet` for dapp checks. Platform and release jobs use repository
or organization settings unless a job declares an environment. Role names describe
provisioning responsibility, not a claim
that a credential is currently installed. `GITHUB_TOKEN` is issued automatically by
GitHub for each job; maintainers control its declared permissions.

## Secrets

Each secret read must have its own row, including identifiers stored as secrets.

| Name | Checks that read it | Purpose | Provisioned by |
| --- | --- | --- | --- |
| `AWS_ACCESS_KEY_ID` | [paxeer-integration-test.yml](../../.github/workflows/paxeer-integration-test.yml) | AWS access-key identifier for integration-test services | Integration infrastructure owner |
| `AWS_SECRET_ACCESS_KEY` | [paxeer-integration-test.yml](../../.github/workflows/paxeer-integration-test.yml) | AWS authentication key paired with the integration access-key identifier | Integration infrastructure owner |
| `BUF_TOKEN` | [paxeer-proto-registry.yml](../../.github/workflows/paxeer-proto-registry.yml) | Authenticate protobuf schema publication to the Buf registry | Protobuf registry maintainer |
| `DAPP_TESTS_MNEMONIC` | [paxeer-dapp-tests.yml](../../.github/workflows/paxeer-dapp-tests.yml), [paxeer-integration-test.yml](../../.github/workflows/paxeer-integration-test.yml) | Funded devnet or testnet account mnemonic for dapp and integration journeys | Devnet/testnet operator |
| `DOCKERHUB_TOKEN` | [paxeer-db-tests.yml](../../.github/workflows/paxeer-db-tests.yml), [paxeer-docker-build.yml](../../.github/workflows/paxeer-docker-build.yml), [paxeer-go-test.yml](../../.github/workflows/paxeer-go-test.yml), [paxeer-integration-test.yml](../../.github/workflows/paxeer-integration-test.yml) | Authenticate Docker Hub pulls and image builds | Container registry maintainer |
| `DOCKERHUB_USERNAME` | [paxeer-db-tests.yml](../../.github/workflows/paxeer-db-tests.yml), [paxeer-docker-build.yml](../../.github/workflows/paxeer-docker-build.yml), [paxeer-go-test.yml](../../.github/workflows/paxeer-go-test.yml), [paxeer-integration-test.yml](../../.github/workflows/paxeer-integration-test.yml) | Docker Hub account used by the optional registry login | Container registry maintainer |
| `ETHEREUM_ASSET_EVIDENCE_B64` | [interop-migration-testnets.yml](../../.github/workflows/interop-migration-testnets.yml) | Base64 source asset evidence for Ethereum migration checks | Migration testnet operator |
| `ETHEREUM_CONFIG_B64` | [interop-migration-testnets.yml](../../.github/workflows/interop-migration-testnets.yml) | Base64 live source-verifier configuration for Ethereum migration checks | Migration testnet operator |
| `ETHEREUM_HISTORY_EVIDENCE_B64` | [interop-migration-testnets.yml](../../.github/workflows/interop-migration-testnets.yml) | Base64 source history evidence for Ethereum migration checks | Migration testnet operator |
| `ETHEREUM_HISTORY_STORE_CONFIG_B64` | [interop-migration-testnets.yml](../../.github/workflows/interop-migration-testnets.yml) | Base64 history-store configuration for Ethereum migration checks | Migration testnet operator |
| `ETHEREUM_OWNERSHIP_EVIDENCE_B64` | [interop-migration-testnets.yml](../../.github/workflows/interop-migration-testnets.yml) | Base64 account ownership evidence for Ethereum migration checks | Migration testnet operator |
| `ETHEREUM_RPC_A_TOKEN` | [interop-migration-testnets.yml](../../.github/workflows/interop-migration-testnets.yml) | Authentication for the first independent RPC provider for Ethereum migration checks | Migration testnet operator |
| `ETHEREUM_RPC_B_TOKEN` | [interop-migration-testnets.yml](../../.github/workflows/interop-migration-testnets.yml) | Authentication for the second independent RPC provider for Ethereum migration checks | Migration testnet operator |
| `GITHUB_TOKEN` | [explorer-images.yml](../../.github/workflows/explorer-images.yml), [labeler.yml](../../.github/workflows/labeler.yml), [offchain-images.yml](../../.github/workflows/offchain-images.yml), [paxeer-docker-build.yml](../../.github/workflows/paxeer-docker-build.yml), [paxeer-docker-publish.yml](../../.github/workflows/paxeer-docker-publish.yml), [paxeer-ghcr-integration-test-cleanup.yml](../../.github/workflows/paxeer-ghcr-integration-test-cleanup.yml), [paxeer-integration-test.yml](../../.github/workflows/paxeer-integration-test.yml), [publish-images.yml](../../.github/workflows/publish-images.yml) | Per-job GitHub API and GHCR authentication for labels, images and package cleanup | GitHub Actions automatically; repository maintainers set job permissions |
| `JOURNAL_ANCHOR_A_TOKEN` | [interop-migration-testnets.yml](../../.github/workflows/interop-migration-testnets.yml) | Authenticate the first migration journal anchor | Migration testnet operator |
| `JOURNAL_ANCHOR_B_TOKEN` | [interop-migration-testnets.yml](../../.github/workflows/interop-migration-testnets.yml) | Authenticate the second migration journal anchor | Migration testnet operator |
| `JOURNAL_AUTHENTICATION_KEY` | [interop-migration-testnets.yml](../../.github/workflows/interop-migration-testnets.yml) | Authenticate migration journal records | Migration testnet operator |
| `LAYERX_AGENT_INSTALL_IDENTITY_TOKEN` | [platform.yml](../../.github/workflows/platform.yml) | Identity authentication for the real agent installation journey | Platform integration/testnet operator |
| `LAYERX_AGENT_INSTALL_SIGNING_SEED` | [platform.yml](../../.github/workflows/platform.yml) | Signing seed for the real agent installation journey | Platform integration/testnet operator |
| `LAYERX_CLIENT_CERTIFICATE_PEM` | [platform.yml](../../.github/workflows/platform.yml) | mTLS client certificate for connected middleware | Platform integration/testnet operator |
| `LAYERX_CLIENT_KEY_PEM` | [platform.yml](../../.github/workflows/platform.yml) | mTLS private key for connected middleware | Platform integration/testnet operator |
| `LAYERX_EMULATOR_TOKEN` | [platform.yml](../../.github/workflows/platform.yml) | Authenticate connected middleware against the emulator | Platform integration/testnet operator |
| `LAYERX_GATEWAY_KEY` | [platform.yml](../../.github/workflows/platform.yml) | Authenticate connected middleware gateway requests | Platform integration/testnet operator |
| `LAYERX_RELEASE_CRATES_IO_TOKEN` | [platform.yml](../../.github/workflows/platform.yml) | Authenticate Rust crate publication | SDK release maintainer |
| `LAYERX_RELEASE_GO_TAG_TOKEN` | [platform.yml](../../.github/workflows/platform.yml) | Authenticate pushing signed Go module release tags | SDK release maintainer |
| `LAYERX_RELEASE_GPG_PASSPHRASE` | [platform.yml](../../.github/workflows/platform.yml) | Unlock the release signing key for Go, Maven and Swift publication | SDK release maintainer |
| `LAYERX_RELEASE_GPG_PRIVATE_KEY` | [platform.yml](../../.github/workflows/platform.yml) | Sign Go and Swift release tags and Maven artifacts | SDK release maintainer |
| `LAYERX_RELEASE_MAVEN_CENTRAL_TOKEN` | [platform.yml](../../.github/workflows/platform.yml) | Authenticate signed bundle uploads to Maven Central | SDK release maintainer |
| `LAYERX_RELEASE_NPM_TOKEN` | [platform.yml](../../.github/workflows/platform.yml) | Authenticate npm package publication | SDK release maintainer |
| `LAYERX_RELEASE_NUGET_API_KEY` | [platform.yml](../../.github/workflows/platform.yml) | Authenticate NuGet package publication | SDK release maintainer |
| `LAYERX_RELEASE_SWIFTPM_REMOTE` | [platform.yml](../../.github/workflows/platform.yml) | Authenticated destination remote for the split Swift package | SDK release maintainer |
| `LAYERX_SCHEDULED_AGENT_SESSION_TOKEN` | [platform.yml](../../.github/workflows/platform.yml) | Agent session authentication for connected middleware and framework journeys | Platform integration/testnet operator |
| `LAYERX_SCHEDULED_AGENT_TOKEN` | [platform.yml](../../.github/workflows/platform.yml) | Authenticate scheduled framework spend-tool requests | Platform integration/testnet operator |
| `LAYERX_SCHEDULED_CLIENT_CERTIFICATE_PEM` | [platform.yml](../../.github/workflows/platform.yml) | mTLS client certificate for scheduled framework journeys | Platform integration/testnet operator |
| `LAYERX_SCHEDULED_CLIENT_KEY_PEM` | [platform.yml](../../.github/workflows/platform.yml) | mTLS private key for scheduled framework journeys | Platform integration/testnet operator |
| `LAYERX_SCHEDULED_DESTINATION_TOKEN` | [platform.yml](../../.github/workflows/platform.yml) | Destination authentication for live scheduled testnet transfers | Platform integration/testnet operator |
| `LAYERX_SCHEDULED_GATEWAY_KEY` | [platform.yml](../../.github/workflows/platform.yml) | Gateway authentication for scheduled framework journeys | Platform integration/testnet operator |
| `LAYERX_SCHEDULED_SOURCE_KEY_BASE64` | [platform.yml](../../.github/workflows/platform.yml) | Base64 source signing key for live scheduled transfers | Platform integration/testnet operator |
| `LAYERX_SCHEDULED_SPEND_TOOL_INPUT_JSON` | [platform.yml](../../.github/workflows/platform.yml) | Protected real spend-tool input for scheduled framework journeys | Platform integration/testnet operator |
| `LAYERX_SCHEDULED_TESTNET_CA_BASE64` | [platform.yml](../../.github/workflows/platform.yml) | Base64 testnet CA used to materialize scheduled client trust | Platform integration/testnet operator |
| `LAYERX_SCHEDULED_TESTNET_TOKEN` | [platform.yml](../../.github/workflows/platform.yml) | Authenticate live testnet, installation and reference-app journeys | Platform integration/testnet operator |
| `LAYERX_SCHEDULED_TRUSTED_CA_PEM` | [platform.yml](../../.github/workflows/platform.yml) | Trusted CA bundle for scheduled framework TLS | Platform integration/testnet operator |
| `LAYERX_SCHEDULED_WEBHOOK_FIXTURE_TOKEN` | [platform.yml](../../.github/workflows/platform.yml) | Authenticate retrieval of fresh signed webhook deliveries | Platform integration/testnet operator |
| `LAYERX_TESTNET_TOKEN` | [platform.yml](../../.github/workflows/platform.yml) | Authenticate connected middleware against the testnet | Platform integration/testnet operator |
| `LAYERX_TRUSTED_CA_PEM` | [platform.yml](../../.github/workflows/platform.yml) | Trusted CA bundle for connected middleware TLS | Platform integration/testnet operator |
| `LAYERX_WALLET_SUPABASE_ANON_KEY` | [platform.yml](../../.github/workflows/platform.yml) | Supabase client authentication for the connected wallet journey | Platform integration/testnet operator |
| `MIGRATION_CA_DER_B64` | [interop-migration-testnets.yml](../../.github/workflows/interop-migration-testnets.yml) | Base64 DER trust root for migration RPC TLS verification | Migration testnet operator |
| `MIRROR_LIVE_CONFIG` | [mirror-live.yml](../../.github/workflows/mirror-live.yml) | Runtime configuration for live archive publication and retrieval | Mirror testnet operator |
| `MIRROR_VERIFY_CONFIG` | [mirror-verification-live.yml](../../.github/workflows/mirror-verification-live.yml) | Operator configuration for mirror-only live verification | Mirror testnet operator |
| `MIRROR_VERIFY_LIVE_VECTORS` | [mirror-verification-live.yml](../../.github/workflows/mirror-verification-live.yml) | Base64 archive of canonical, failover, divergence and tamper requests | Mirror testnet operator |
| `OPENAI_API_KEY` | [paxeer-pr-to-slack-codex.yml](../../.github/workflows/paxeer-pr-to-slack-codex.yml) | Authenticate automated PR review with the Codex CLI | Repository review automation maintainer |
| `OPENROUTER_API_KEY` | [jev-pr-advisory.yml](../../.github/workflows/jev-pr-advisory.yml) | Authenticate the JEV advisory review provider | Repository review automation maintainer |
| `PAXEER_X_CANDIDATE_JSON` | [paxeer-x-qualification.yml](../../.github/workflows/paxeer-x-qualification.yml) | Private candidate configuration for qualification planning and execution | Release qualification owner |
| `PAXEER_X_CONTRACT_JSON` | [paxeer-x-qualification.yml](../../.github/workflows/paxeer-x-qualification.yml) | Private qualification contract for planning and execution | Release qualification owner |
| `SLACK_BOT_TOKEN` | [paxeer-pr-to-slack-codex.yml](../../.github/workflows/paxeer-pr-to-slack-codex.yml) | Authenticate review notification messages and file uploads | Slack app administrator |
| `SLACK_CHANNEL_ID` | [paxeer-pr-to-slack-codex.yml](../../.github/workflows/paxeer-pr-to-slack-codex.yml) | Select the review notification destination channel | Slack workspace/repository automation maintainer |
| `SOLANA_ASSET_EVIDENCE_B64` | [interop-migration-testnets.yml](../../.github/workflows/interop-migration-testnets.yml) | Base64 source asset evidence for Solana migration checks | Migration testnet operator |
| `SOLANA_CONFIG_B64` | [interop-migration-testnets.yml](../../.github/workflows/interop-migration-testnets.yml) | Base64 live source-verifier configuration for Solana migration checks | Migration testnet operator |
| `SOLANA_HISTORY_EVIDENCE_B64` | [interop-migration-testnets.yml](../../.github/workflows/interop-migration-testnets.yml) | Base64 source history evidence for Solana migration checks | Migration testnet operator |
| `SOLANA_HISTORY_STORE_CONFIG_B64` | [interop-migration-testnets.yml](../../.github/workflows/interop-migration-testnets.yml) | Base64 history-store configuration for Solana migration checks | Migration testnet operator |
| `SOLANA_OWNERSHIP_EVIDENCE_B64` | [interop-migration-testnets.yml](../../.github/workflows/interop-migration-testnets.yml) | Base64 account ownership evidence for Solana migration checks | Migration testnet operator |
| `SOLANA_RPC_A_TOKEN` | [interop-migration-testnets.yml](../../.github/workflows/interop-migration-testnets.yml) | Authentication for the first independent RPC provider for Solana migration checks | Migration testnet operator |
| `SOLANA_RPC_B_TOKEN` | [interop-migration-testnets.yml](../../.github/workflows/interop-migration-testnets.yml) | Authentication for the second independent RPC provider for Solana migration checks | Migration testnet operator |

## Variables

These are GitHub Actions `vars` names, including feature flags, identifiers and paths.
A variable ending in `FILE` or `PATH` selects a runner-local input; its operator also
provisions that protected file. Put credential contents in secrets or protected files.

| Name | Checks that read it | Purpose | Provisioned by |
| --- | --- | --- | --- |
| `AWS_ECR_ROLE_ARN` | [paxeer-ecr.yml](../../.github/workflows/paxeer-ecr.yml), [paxeer-nightly-ecr.yml](../../.github/workflows/paxeer-nightly-ecr.yml) | OIDC role assumed for ECR image publication; an empty setting disables the gated publish job | AWS/release infrastructure owner |
| `CI_LINUX_RUNNER` | [agent.yml](../../.github/workflows/agent.yml), [bridge-test.yml](../../.github/workflows/bridge-test.yml), [ci.yml](../../.github/workflows/ci.yml), [docs-links.yml](../../.github/workflows/docs-links.yml), [docs-site.yml](../../.github/workflows/docs-site.yml), [explorer-build.yml](../../.github/workflows/explorer-build.yml), [hosted-developer.yml](../../.github/workflows/hosted-developer.yml), [human.yml](../../.github/workflows/human.yml), [interop-migration-testnets.yml](../../.github/workflows/interop-migration-testnets.yml), [interop-x402.yml](../../.github/workflows/interop-x402.yml), [jev-pr-advisory.yml](../../.github/workflows/jev-pr-advisory.yml), [labeler.yml](../../.github/workflows/labeler.yml), [node-build.yml](../../.github/workflows/node-build.yml), [offchain-images.yml](../../.github/workflows/offchain-images.yml), [paxeer-buf-check.yml](../../.github/workflows/paxeer-buf-check.yml), [paxeer-cross-arch-build.yml](../../.github/workflows/paxeer-cross-arch-build.yml), [paxeer-dapp-tests.yml](../../.github/workflows/paxeer-dapp-tests.yml), [paxeer-enforce-labels.yml](../../.github/workflows/paxeer-enforce-labels.yml), [paxeer-eth-blocktests.yml](../../.github/workflows/paxeer-eth-blocktests.yml), [paxeer-forge-test.yml](../../.github/workflows/paxeer-forge-test.yml), [paxeer-golangci.yml](../../.github/workflows/paxeer-golangci.yml), [paxeer-integration-test.yml](../../.github/workflows/paxeer-integration-test.yml), [paxeer-libwasmvm.yml](../../.github/workflows/paxeer-libwasmvm.yml), [paxeer-mock-balances-build.yml](../../.github/workflows/paxeer-mock-balances-build.yml), [paxeer-pr-to-slack-codex.yml](../../.github/workflows/paxeer-pr-to-slack-codex.yml), [paxeer-release-check.yml](../../.github/workflows/paxeer-release-check.yml), [paxeer-rocksdb-unit-tests.yml](../../.github/workflows/paxeer-rocksdb-unit-tests.yml), [paxeer-uci-backport.yml](../../.github/workflows/paxeer-uci-backport.yml), [paxeer-uci-go-lint.yml](../../.github/workflows/paxeer-uci-go-lint.yml), [paxeer-x-qualification.yml](../../.github/workflows/paxeer-x-qualification.yml), [platform.yml](../../.github/workflows/platform.yml), [programs-conformance.yml](../../.github/workflows/programs-conformance.yml), [ramp-sandbox.yml](../../.github/workflows/ramp-sandbox.yml), [runner-canary.yml](../../.github/workflows/runner-canary.yml), [wallet-go-test.yml](../../.github/workflows/wallet-go-test.yml), [wallet-test.yml](../../.github/workflows/wallet-test.yml), [xweb-test.yml](../../.github/workflows/xweb-test.yml) | Optional Linux runner selector; see the fallback and fork policy below | Repository CI administrator |
| `CODECOV_ENABLED` | [paxeer-db-tests.yml](../../.github/workflows/paxeer-db-tests.yml), [paxeer-go-test.yml](../../.github/workflows/paxeer-go-test.yml), [paxeer-rocksdb-unit-tests.yml](../../.github/workflows/paxeer-rocksdb-unit-tests.yml) | Enable coverage upload when set to true | Repository CI administrator |
| `HUMAN_E2E_BASE_URL` | [human.yml](../../.github/workflows/human.yml) | Base URL for the Human browser journey | Human application maintainer |
| `HUMAN_VISUAL_BASELINE_REVIEWED` | [human.yml](../../.github/workflows/human.yml) | Record approval of the Human visual baseline | Human application maintainer |
| `LAYERX_AGENT_INSTALL_A2A_SEQUENCE` | [platform.yml](../../.github/workflows/platform.yml) | A2A request sequence used by the installation journey | Platform integration/testnet operator |
| `LAYERX_AGENT_INSTALL_ASSET` | [platform.yml](../../.github/workflows/platform.yml) | Payment asset for the installation journey | Platform integration/testnet operator |
| `LAYERX_AGENT_INSTALL_DESTINATION` | [platform.yml](../../.github/workflows/platform.yml) | Payment destination for the installation journey | Platform integration/testnet operator |
| `LAYERX_AGENT_INSTALL_MCP_SEQUENCE` | [platform.yml](../../.github/workflows/platform.yml) | MCP request sequence used by the installation journey | Platform integration/testnet operator |
| `LAYERX_AGENT_INSTALL_NETWORK_ID` | [platform.yml](../../.github/workflows/platform.yml) | Network identifier for the installation journey | Platform integration/testnet operator |
| `LAYERX_AGENT_INSTALL_SOURCE_ACCOUNT` | [platform.yml](../../.github/workflows/platform.yml) | Funded source account for the installation journey | Platform integration/testnet operator |
| `LAYERX_CONNECTED_MIDDLEWARE_FIXTURE` | [platform.yml](../../.github/workflows/platform.yml) | Protected fixture path for original-wallet consent and merchant settlement | Platform integration/testnet operator |
| `LAYERX_DAEMON_PREPARATION_STORE_PATH` | [platform.yml](../../.github/workflows/platform.yml) | Persistent preparation-store path for the connected daemon | Platform integration/testnet operator |
| `LAYERX_DAEMON_RECEIPT_STORE_PATH` | [platform.yml](../../.github/workflows/platform.yml) | Persistent receipt-store path for the connected daemon | Platform integration/testnet operator |
| `LAYERX_EMULATOR_ASSET` | [platform.yml](../../.github/workflows/platform.yml) | Asset identifier for connected middleware emulator checks | Platform integration/testnet operator |
| `LAYERX_EMULATOR_CURRENCY` | [platform.yml](../../.github/workflows/platform.yml) | Payment currency for connected middleware emulator checks | Platform integration/testnet operator |
| `LAYERX_EMULATOR_MERCHANT` | [platform.yml](../../.github/workflows/platform.yml) | Merchant identifier for connected middleware emulator checks | Platform integration/testnet operator |
| `LAYERX_EMULATOR_MERCHANT_ACCOUNT` | [platform.yml](../../.github/workflows/platform.yml) | Merchant settlement account for connected middleware emulator checks | Platform integration/testnet operator |
| `LAYERX_EMULATOR_PRICE` | [platform.yml](../../.github/workflows/platform.yml) | Payment price for connected middleware emulator checks | Platform integration/testnet operator |
| `LAYERX_EMULATOR_WEBHOOK_PUBLIC_KEYS_JSON` | [platform.yml](../../.github/workflows/platform.yml) | Webhook verification public-key set for connected middleware emulator checks | Platform integration/testnet operator |
| `LAYERX_GATEWAY_KEY_ID` | [platform.yml](../../.github/workflows/platform.yml) | Identifier of the connected middleware gateway authentication key | Platform integration/testnet operator |
| `LAYERX_MIGRATION_V2_FIXTURE` | [interop-migration-testnets.yml](../../.github/workflows/interop-migration-testnets.yml) | Runner-local protected owner fixture path for funded migration V2 | Migration testnet operator |
| `LAYERX_REFERENCE_APPS_ENABLED` | [platform.yml](../../.github/workflows/platform.yml) | Enable real reference-app journeys when set to true | Platform integration/testnet operator |
| `LAYERX_REFERENCE_CHECKOUT_KEY` | [platform.yml](../../.github/workflows/platform.yml) | Merchant checkout key identifier | Platform integration/testnet operator |
| `LAYERX_REFERENCE_CURRENCY` | [platform.yml](../../.github/workflows/platform.yml) | Reference payment currency | Platform integration/testnet operator |
| `LAYERX_REFERENCE_MARKETPLACE_KEY` | [platform.yml](../../.github/workflows/platform.yml) | Marketplace request key identifier | Platform integration/testnet operator |
| `LAYERX_REFERENCE_MARKETPLACE_LISTING_ID` | [platform.yml](../../.github/workflows/platform.yml) | Marketplace listing identifier | Platform integration/testnet operator |
| `LAYERX_REFERENCE_MARKETPLACE_PROGRAM_ID` | [platform.yml](../../.github/workflows/platform.yml) | Marketplace program identifier | Platform integration/testnet operator |
| `LAYERX_REFERENCE_MARKETPLACE_RECEIPT_DIGEST` | [platform.yml](../../.github/workflows/platform.yml) | Expected marketplace receipt digest | Platform integration/testnet operator |
| `LAYERX_REFERENCE_MERCHANT` | [platform.yml](../../.github/workflows/platform.yml) | Reference merchant identifier | Platform integration/testnet operator |
| `LAYERX_REFERENCE_MERCHANT_ACCOUNT` | [platform.yml](../../.github/workflows/platform.yml) | Reference merchant settlement account | Platform integration/testnet operator |
| `LAYERX_REFERENCE_PAYMENT_KEY` | [platform.yml](../../.github/workflows/platform.yml) | Reference payment key identifier | Platform integration/testnet operator |
| `LAYERX_REFERENCE_SELLER` | [platform.yml](../../.github/workflows/platform.yml) | Reference seller identifier | Platform integration/testnet operator |
| `LAYERX_REFERENCE_SELLER_ACCOUNT` | [platform.yml](../../.github/workflows/platform.yml) | Reference seller settlement account | Platform integration/testnet operator |
| `LAYERX_REFERENCE_WEBHOOK_KEYS_JSON` | [platform.yml](../../.github/workflows/platform.yml) | Reference webhook verification public-key set | Platform integration/testnet operator |
| `LAYERX_RELEASE_CRATES_IO_INDEX` | [platform.yml](../../.github/workflows/platform.yml) | Optional Rust registry index override | SDK release maintainer |
| `LAYERX_RELEASE_GO_PROXY` | [platform.yml](../../.github/workflows/platform.yml) | Go module proxy target | SDK release maintainer |
| `LAYERX_RELEASE_MAVEN_REPOSITORY_URL` | [platform.yml](../../.github/workflows/platform.yml) | Maven consumer repository target | SDK release maintainer |
| `LAYERX_RELEASE_MAVEN_UPLOAD_URL` | [platform.yml](../../.github/workflows/platform.yml) | Maven publisher upload endpoint | SDK release maintainer |
| `LAYERX_RELEASE_NPM_REGISTRY` | [platform.yml](../../.github/workflows/platform.yml) | npm registry target | SDK release maintainer |
| `LAYERX_RELEASE_NUGET_SOURCE` | [platform.yml](../../.github/workflows/platform.yml) | NuGet package source | SDK release maintainer |
| `LAYERX_RELEASE_PYPI_INDEX_URL` | [platform.yml](../../.github/workflows/platform.yml) | Python consumer package index | SDK release maintainer |
| `LAYERX_RELEASE_PYPI_REPOSITORY_URL` | [platform.yml](../../.github/workflows/platform.yml) | Python publisher upload endpoint | SDK release maintainer |
| `LAYERX_RELEASE_SWIFTPM_URL` | [platform.yml](../../.github/workflows/platform.yml) | Public Swift package URL recorded for consumers | SDK release maintainer |
| `LAYERX_SCHEDULED_A2A_URL` | [platform.yml](../../.github/workflows/platform.yml) | A2A service endpoint | Platform integration/testnet operator |
| `LAYERX_SCHEDULED_AGENT_ACTOR` | [platform.yml](../../.github/workflows/platform.yml) | Agent actor identifier | Platform integration/testnet operator |
| `LAYERX_SCHEDULED_AGENT_AUTHORITY` | [platform.yml](../../.github/workflows/platform.yml) | Agent spending authority identifier | Platform integration/testnet operator |
| `LAYERX_SCHEDULED_AGENT_FEE_LIMIT` | [platform.yml](../../.github/workflows/platform.yml) | Agent journey fee limit | Platform integration/testnet operator |
| `LAYERX_SCHEDULED_AGENT_RPC_URL` | [platform.yml](../../.github/workflows/platform.yml) | Agent RPC service endpoint | Platform integration/testnet operator |
| `LAYERX_SCHEDULED_AGENT_SERVICES_MODULE` | [platform.yml](../../.github/workflows/platform.yml) | Runner-local real framework services module path | Platform integration/testnet operator |
| `LAYERX_SCHEDULED_AGENT_SESSION_GENERATION` | [platform.yml](../../.github/workflows/platform.yml) | Agent session generation | Platform integration/testnet operator |
| `LAYERX_SCHEDULED_AGENT_SESSION_ID` | [platform.yml](../../.github/workflows/platform.yml) | Agent session identifier | Platform integration/testnet operator |
| `LAYERX_SCHEDULED_AGENT_SURFACE` | [platform.yml](../../.github/workflows/platform.yml) | Agent framework surface selector | Platform integration/testnet operator |
| `LAYERX_SCHEDULED_AGENT_TENANT` | [platform.yml](../../.github/workflows/platform.yml) | Agent tenant identifier | Platform integration/testnet operator |
| `LAYERX_SCHEDULED_ASSET` | [platform.yml](../../.github/workflows/platform.yml) | Live transfer and mobile sample asset identifier | Platform integration/testnet operator |
| `LAYERX_SCHEDULED_BUDGET_SERVICE_URL` | [platform.yml](../../.github/workflows/platform.yml) | Budget service endpoint | Platform integration/testnet operator |
| `LAYERX_SCHEDULED_DESTINATION_DID` | [platform.yml](../../.github/workflows/platform.yml) | Live transfer destination identity | Platform integration/testnet operator |
| `LAYERX_SCHEDULED_DESTINATION_PUBLIC_KEY` | [platform.yml](../../.github/workflows/platform.yml) | Live destination verification public key | Platform integration/testnet operator |
| `LAYERX_SCHEDULED_EVENT_KEY_ID` | [platform.yml](../../.github/workflows/platform.yml) | Mobile event verification key identifier | Platform integration/testnet operator |
| `LAYERX_SCHEDULED_EVENT_PUBLIC_KEY` | [platform.yml](../../.github/workflows/platform.yml) | Mobile event verification public key | Platform integration/testnet operator |
| `LAYERX_SCHEDULED_FAUCET_URL` | [platform.yml](../../.github/workflows/platform.yml) | Live testnet faucet endpoint | Platform integration/testnet operator |
| `LAYERX_SCHEDULED_GATEWAY_KEY_ID` | [platform.yml](../../.github/workflows/platform.yml) | Scheduled gateway authentication key identifier | Platform integration/testnet operator |
| `LAYERX_SCHEDULED_GATEWAY_URL` | [platform.yml](../../.github/workflows/platform.yml) | Live testnet gateway endpoint | Platform integration/testnet operator |
| `LAYERX_SCHEDULED_HUMAN_SERVICE_URL` | [platform.yml](../../.github/workflows/platform.yml) | Human service endpoint for mobile journeys | Platform integration/testnet operator |
| `LAYERX_SCHEDULED_MOVE_QUOTE_JSON` | [platform.yml](../../.github/workflows/platform.yml) | Real move quote input for mobile journeys | Platform integration/testnet operator |
| `LAYERX_SCHEDULED_PROTOCOL_VERSION` | [platform.yml](../../.github/workflows/platform.yml) | Live protocol version selector | Platform integration/testnet operator |
| `LAYERX_SCHEDULED_RECEIPT_RELAY_URL` | [platform.yml](../../.github/workflows/platform.yml) | Mobile receipt relay endpoint | Platform integration/testnet operator |
| `LAYERX_SCHEDULED_RECEIPT_SERVICE_URL` | [platform.yml](../../.github/workflows/platform.yml) | Agent receipt service endpoint | Platform integration/testnet operator |
| `LAYERX_SCHEDULED_RECIPIENT` | [platform.yml](../../.github/workflows/platform.yml) | Mobile sample payment recipient | Platform integration/testnet operator |
| `LAYERX_SCHEDULED_SEQUENCER_PUBLIC_KEY` | [platform.yml](../../.github/workflows/platform.yml) | Live receipt sequencer verification public key | Platform integration/testnet operator |
| `LAYERX_SCHEDULED_SESSION_BROKER_URL` | [platform.yml](../../.github/workflows/platform.yml) | Mobile session broker endpoint | Platform integration/testnet operator |
| `LAYERX_SCHEDULED_SIGNER_SERVICE_URL` | [platform.yml](../../.github/workflows/platform.yml) | Agent signer service endpoint | Platform integration/testnet operator |
| `LAYERX_SCHEDULED_SOURCE_DID` | [platform.yml](../../.github/workflows/platform.yml) | Funded live transfer source identity | Platform integration/testnet operator |
| `LAYERX_SCHEDULED_SOURCE_PUBLIC_KEY` | [platform.yml](../../.github/workflows/platform.yml) | Live source verification public key | Platform integration/testnet operator |
| `LAYERX_SCHEDULED_TESTNET_URL` | [platform.yml](../../.github/workflows/platform.yml) | Live testnet service endpoint | Platform integration/testnet operator |
| `LAYERX_SCHEDULED_WEBHOOK_FIXTURE_URL` | [platform.yml](../../.github/workflows/platform.yml) | Service providing fresh signed webhook deliveries | Platform integration/testnet operator |
| `LAYERX_SCHEDULED_WEBHOOK_PUBLIC_KEYS_JSON` | [platform.yml](../../.github/workflows/platform.yml) | Scheduled webhook verification public-key set | Platform integration/testnet operator |
| `LAYERX_TEN_LINE_REGISTRY_SNAPSHOT` | [platform.yml](../../.github/workflows/platform.yml) | Registry snapshot input for the ten-line integration checks | Platform integration/testnet operator |
| `LAYERX_TEN_LINE_REGISTRY_URL` | [platform.yml](../../.github/workflows/platform.yml) | Registry service endpoint for the ten-line integration checks | Platform integration/testnet operator |
| `LAYERX_TESTNET_ASSET` | [platform.yml](../../.github/workflows/platform.yml) | Asset identifier for connected middleware testnet checks | Platform integration/testnet operator |
| `LAYERX_TESTNET_CURRENCY` | [platform.yml](../../.github/workflows/platform.yml) | Payment currency for connected middleware testnet checks | Platform integration/testnet operator |
| `LAYERX_TESTNET_MERCHANT` | [platform.yml](../../.github/workflows/platform.yml) | Merchant identifier for connected middleware testnet checks | Platform integration/testnet operator |
| `LAYERX_TESTNET_MERCHANT_ACCOUNT` | [platform.yml](../../.github/workflows/platform.yml) | Merchant settlement account for connected middleware testnet checks | Platform integration/testnet operator |
| `LAYERX_TESTNET_PRICE` | [platform.yml](../../.github/workflows/platform.yml) | Payment price for connected middleware testnet checks | Platform integration/testnet operator |
| `LAYERX_TESTNET_WEBHOOK_PUBLIC_KEYS_JSON` | [platform.yml](../../.github/workflows/platform.yml) | Webhook verification public-key set for connected middleware testnet checks | Platform integration/testnet operator |
| `LAYERX_WALLET_API_URL` | [platform.yml](../../.github/workflows/platform.yml) | Connected wallet API endpoint | Platform integration/testnet operator |
| `LAYERX_WALLET_PRINCIPAL` | [platform.yml](../../.github/workflows/platform.yml) | Principal used by the connected wallet | Platform integration/testnet operator |
| `LAYERX_WALLET_SESSION_FILE` | [platform.yml](../../.github/workflows/platform.yml) | Runner-local wallet session file path | Platform integration/testnet operator |
| `LAYERX_WALLET_STORE_PATH` | [platform.yml](../../.github/workflows/platform.yml) | Runner-local wallet state store path | Platform integration/testnet operator |
| `LAYERX_WALLET_SUPABASE_URL` | [platform.yml](../../.github/workflows/platform.yml) | Connected wallet Supabase endpoint | Platform integration/testnet operator |
| `MIRROR_DOTNET_CONFORMANCE` | [mirror-verification-live.yml](../../.github/workflows/mirror-verification-live.yml) | DOTNET conformance command for mirror-only live verification | Mirror testnet operator |
| `MIRROR_GO_CONFORMANCE` | [mirror-verification-live.yml](../../.github/workflows/mirror-verification-live.yml) | GO conformance command for mirror-only live verification | Mirror testnet operator |
| `MIRROR_JVM_CONFORMANCE` | [mirror-verification-live.yml](../../.github/workflows/mirror-verification-live.yml) | JVM conformance command for mirror-only live verification | Mirror testnet operator |
| `MIRROR_PYTHON_CONFORMANCE` | [mirror-verification-live.yml](../../.github/workflows/mirror-verification-live.yml) | PYTHON conformance command for mirror-only live verification | Mirror testnet operator |
| `MIRROR_SWIFT_CONFORMANCE` | [mirror-verification-live.yml](../../.github/workflows/mirror-verification-live.yml) | SWIFT conformance command for mirror-only live verification | Mirror testnet operator |
| `MIRROR_TS_CONFORMANCE` | [mirror-verification-live.yml](../../.github/workflows/mirror-verification-live.yml) | TS conformance command for mirror-only live verification | Mirror testnet operator |
| `PERPS_BUILD_IMAGE` | [offchain-images.yml](../../.github/workflows/offchain-images.yml) | Digest-pinned Perps builder base image | Container/release maintainer |
| `PERPS_RUNTIME_IMAGE` | [offchain-images.yml](../../.github/workflows/offchain-images.yml) | Digest-pinned Perps runtime base image | Container/release maintainer |
| `RAMP_AUTHORITY_TOKEN_FILE` | [ramp-sandbox.yml](../../.github/workflows/ramp-sandbox.yml) | Runner-local authority authentication token file | Ramp sandbox operator |
| `RAMP_AUTHORITY_URL` | [ramp-sandbox.yml](../../.github/workflows/ramp-sandbox.yml) | Ramp authority endpoint | Ramp sandbox operator |
| `RAMP_CANDIDATE_MANIFEST_FILE` | [ramp-sandbox.yml](../../.github/workflows/ramp-sandbox.yml) | Runner-local candidate manifest path | Ramp sandbox operator |
| `RAMP_CA_PEM_FILE` | [ramp-sandbox.yml](../../.github/workflows/ramp-sandbox.yml) | Runner-local TLS trust-root file | Ramp sandbox operator |
| `RAMP_CLIENT_CERT_PEM_FILE` | [ramp-sandbox.yml](../../.github/workflows/ramp-sandbox.yml) | Runner-local mTLS client certificate file | Ramp sandbox operator |
| `RAMP_CLIENT_KEY_FILE` | [ramp-sandbox.yml](../../.github/workflows/ramp-sandbox.yml) | Runner-local mTLS private-key file | Ramp sandbox operator |
| `RAMP_CUSTOMER_TOKEN_FILE` | [ramp-sandbox.yml](../../.github/workflows/ramp-sandbox.yml) | Runner-local customer authentication token file | Ramp sandbox operator |
| `RAMP_INPUTS_FILE` | [ramp-sandbox.yml](../../.github/workflows/ramp-sandbox.yml) | Runner-local real journey inputs file | Ramp sandbox operator |
| `RAMP_OPERATOR_TOKEN_FILE` | [ramp-sandbox.yml](../../.github/workflows/ramp-sandbox.yml) | Runner-local operator authentication token file | Ramp sandbox operator |
| `RAMP_OPERATOR_URL` | [ramp-sandbox.yml](../../.github/workflows/ramp-sandbox.yml) | Ramp operator endpoint | Ramp sandbox operator |
| `RAMP_OTHER_CUSTOMER_TOKEN_FILE` | [ramp-sandbox.yml](../../.github/workflows/ramp-sandbox.yml) | Runner-local second customer token file for isolation checks | Ramp sandbox operator |
| `RAMP_RECEIPT_VERIFIER_CONFIG_FILE` | [ramp-sandbox.yml](../../.github/workflows/ramp-sandbox.yml) | Runner-local receipt verifier configuration file | Ramp sandbox operator |
| `RAMP_RECOVERY_CONTRACT_CALLBACK_FILE` | [ramp-sandbox.yml](../../.github/workflows/ramp-sandbox.yml) | Runner-local recovery contract callback input file | Ramp sandbox operator |
| `RAMP_RECOVERY_CONTRACT_CONFIG_FILE` | [ramp-sandbox.yml](../../.github/workflows/ramp-sandbox.yml) | Runner-local recovery contract configuration file | Ramp sandbox operator |
| `RAMP_RECOVERY_CONTRACT_ORDER_FILE` | [ramp-sandbox.yml](../../.github/workflows/ramp-sandbox.yml) | Runner-local recovery contract order input file | Ramp sandbox operator |
| `RAMP_UNIFIED_URL` | [ramp-sandbox.yml](../../.github/workflows/ramp-sandbox.yml) | Unified ramp service endpoint | Ramp sandbox operator |
| `REVIEWED_BUILD_IMAGE` | [offchain-images.yml](../../.github/workflows/offchain-images.yml) | Reviewed digest-pinned builder base image | Container/release maintainer |
| `REVIEWED_RUNTIME_IMAGE` | [offchain-images.yml](../../.github/workflows/offchain-images.yml) | Reviewed digest-pinned runtime base image | Container/release maintainer |

## Runner selection and labels

`CI_LINUX_RUNNER` is the optional repository/organization Actions variable selecting
the trusted Linux runner label. Most routed checks use
`vars.CI_LINUX_RUNNER || 'ubuntu-24.04'`: if the variable is unset or empty,
GitHub schedules `ubuntu-24.04`. Some chain workflows retain an `ubuntu-latest`
fallback. Their `runs-on` expressions are the source for the per-job choice.
Fork pull requests select the workflow's GitHub-hosted fallback before consulting
this variable. The repository CI administrator provisions the variable and registers
the matching runner; setting a label alone does not register a runner.

The custom actionlint label **`layerx-testnet`** is used together with **`self-hosted`**
by [mirror-live.yml](../../.github/workflows/mirror-live.yml) (`publish-retrieve`) and
[mirror-verification-live.yml](../../.github/workflows/mirror-verification-live.yml)
(`mirror-only`). The mirror testnet operator registers and labels these runners and
installs the live testnet tools and protected configuration they require.
`self-hosted` is GitHub's standard label for a registered self-hosted runner.
The built-in hosted labels used by the checks are `ubuntu-24.04`, `ubuntu-latest`,
`macos-15` and `macos-latest`; GitHub provisions these hosted runners.
Only custom labels belong in [actionlint.yaml](../../.github/actionlint.yaml).

## Keeping the inventory current

When a workflow gains a secret or variable, add its name, consuming workflow, purpose
and provisioning role to the corresponding table. Keep custom actionlint labels tied
to real `runs-on` uses and describe them here. The focused inventory gate is
`python3 -m pytest -q tests/ci/test_workflow_secrets_documented.py`.
