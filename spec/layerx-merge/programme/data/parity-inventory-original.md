# Parity inventory

Parity coverage connects kernel constructs to their chain implementations and
client-visible results. The tests below cover result codes, state transitions,
receipts, events and ordering through the corresponding packages. The selector
list is also represented in [parity.kvx](parity.kvx).

## Implementations and fixtures

| Surface | Source and test roots |
| --- | --- |
| Native kernel | `src`, `include/layerx`, `tests/ledger`, `tests/modules`, `tests/programs` |
| Chain constructs | `modules/layerxledger`, `modules/layerxauth`, `modules/layerxpay`, `precompiles` |
| Sequencing and committed receipts | `node/sequencer`, `modules/sequencer`, `rpc` |
| Programs | `programs/crates`, `programs/fixtures`, `programs/tests/vectors` |
| Web requests | `modules/xweb`, `precompiles/xweb`, `services/interop/crates/x-websearch` |
| Agent clients | `services/agent/crates`, `services/agent/sdk` |
| Wallet clients | `apps/wallet`, `services/wallet`, `sdk/wallet` |
| Explorer | `apps/explorer`, `services/explorer/backend` |
| Platform clients and services | `sdk/platform`, `services/platform/hosted` |
| Bridge and mirror | `modules/layerxbridge`, `services/interop`, `contracts/bridge` |

The ledger, pay and auth construct precompiles use `0x101A`, `0x101B` and
`0x101C`; Programs uses `0x101D`, and chain domains use `0x101E`. Addresses are
short forms of twenty-byte EVM addresses padded with leading zero bytes. The
[activation table](../../docs/plans/activation-and-migration.md#merge-precompiles)
defines the merge entries and module gates.

## Construct coverage

### Sequencer execution and block parity

- `node/sequencer/mixed_load_test.go::TestMixedLoadSealedEqualsProcessBlock`
- `node/sequencer/seal_giga_test.go::TestSealBatchEqualsGigaProcessBlock`

### Receipts and admission

- `node/sequencer/receipt_equality_test.go::TestReceiptEqualsCommittedReceipt`
- `node/sequencer/receipt_equality_test.go::TestReceiptLogEffects`

### Fees, fee token and sponsored gas

- `precompiles/feetoken/feetoken_test.go::TestFeeDenomRoundTripAndCallerIsolation`
- `precompiles/feetoken/feetoken_test.go::TestFeeDenomRefusalsPreservePreference`
- `precompiles/feetoken/feetoken_test.go::TestFeeDenomRefusalsCarryRegistryCodesAtMergeHeight`
- `precompiles/feetoken/feetoken_test.go::TestFeeDenomClearAfterGovernanceChanges`
- `precompiles/feetoken/feetoken_test.go::TestFeeDenomClearWithdrawnPreference`
- `precompiles/feetoken/feetoken_test.go::TestFeeDenomViewDoesNotWriteOrChargeAnteGas`
- `precompiles/feetoken/feetoken_test.go::TestFeeDenomGasAndTransactions`
- `precompiles/feetoken/feetoken_test.go::TestFeeDenomStaticClearAndRollback`
- `precompiles/feetoken/feetoken_test.go::TestFeeDenomStoreOutOfGasPropagates`
- `precompiles/feetoken/feetoken_test.go::TestFeeDenomMissingKeeper`

### Kernel replay corpus

- `tests/test_qual_replay_tail.c::main`

### Asset ledger

- `modules/layerxledger/keeper/assets_test.go::TestRegisterAsset`
- `modules/layerxledger/keeper/assets_test.go::TestRegisterAssetTwiceRefused`
- `modules/layerxledger/keeper/assets_test.go::TestPauseUnpauseAsset`
- `modules/layerxledger/keeper/supply_test.go::TestMintWithinCap`
- `modules/layerxledger/keeper/supply_test.go::TestBurnReducesBalance`
- `modules/layerxledger/keeper/accounts_test.go::TestOpenAccount`
- `modules/layerxledger/keeper/accounts_test.go::TestOpenAccountPausedAsset`
- `modules/layerxledger/keeper/send_test.go::TestSendMovesBalance`
- `modules/layerxledger/keeper/send_test.go::TestSendRefusals`
- `modules/layerxledger/keeper/payexact_test.go::TestPayExact`
- `modules/layerxledger/keeper/payexact_test.go::TestPayExactExpired`
- `modules/layerxledger/keeper/views_test.go::TestLedgerViews`
- `modules/layerxledger/keeper/views_test.go::TestNextSequence`

### Authority, sessions and idempotency

- `modules/layerxauth/keeper/execute_test.go::TestAuthExecuteIdempotentReplay`
- `modules/layerxauth/keeper/execute_test.go::TestAuthExecuteSequenceReused`
- `modules/layerxauth/keeper/sessions_test.go::TestGrantSession`
- `modules/layerxauth/keeper/sessions_test.go::TestGrantSessionSuccessor`
- `modules/layerxauth/keeper/capability_test.go::TestGrantCapability`
- `modules/layerxauth/keeper/capability_test.go::TestGrantCapabilityRefusesOwnKey`
- `modules/layerxauth/keeper/revoke_test.go::TestRevokeGrantAdvancesRevocation`
- `modules/layerxauth/keeper/revoke_test.go::TestRevokeGrantRefusesOtherGrantor`
- `modules/layerxauth/keeper/views_test.go::TestAuthGrantViews`
- `modules/layerxauth/keeper/views_test.go::TestAuthIsLive`
- `modules/layerxauth/keeper/views_test.go::TestAuthCheckActivity`
- `modules/layerxauth/keeper/retire_test.go::TestAuthModuleStoreHasNoIdempotencyKeys`

### Escrow

- `modules/layerxpay/keeper/escrow_open_test.go::TestEscrowOpenLocksFunds`
- `modules/layerxpay/keeper/escrow_open_test.go::TestEscrowCaptureAndPartialCapture`
- `modules/layerxpay/keeper/escrow_settle_test.go::TestEscrowReleaseAndTimeout`
- `modules/layerxpay/keeper/escrow_settle_test.go::TestEscrowDisputeResolveSplit`
- `modules/layerxpay/keeper/escrow_views_test.go::TestEscrowViewsReadKernelRecord`
- `modules/layerxpay/keeper/escrow_views_test.go::TestEscrowViewsUnknownIDEmpty`

### Budget

- `modules/layerxpay/keeper/budget_create_test.go::TestBudgetCreateAndFund`
- `modules/layerxpay/keeper/budget_create_test.go::TestBudgetNativeVariantsAndAmend`
- `modules/layerxpay/keeper/budget_spend_test.go::TestBudgetSpendWithinPeriodLimit`
- `modules/layerxpay/keeper/budget_spend_test.go::TestBudgetDelegateSpendDerivedKey`
- `modules/layerxpay/keeper/budget_pull_test.go::TestBudgetPullIsRetired`
- `modules/layerxpay/keeper/budget_pull_test.go::TestBudgetPullRefusesMalformedGrant`
- `modules/layerxpay/keeper/budget_close_test.go::TestBudgetCloseDrainsToOwner`
- `modules/layerxpay/keeper/budget_close_test.go::TestBudgetDefundAndRevoke`
- `modules/layerxpay/keeper/budget_views_test.go::TestBudgetViewsReadKernelRecord`

### Stream

- `modules/layerxpay/keeper/stream_open_test.go::TestStreamOpenFundsFromPayer`
- `modules/layerxpay/keeper/stream_open_test.go::TestStreamTopUpAddsFunding`
- `modules/layerxpay/keeper/stream_settle_test.go::TestStreamSettlePaysAccrued`
- `modules/layerxpay/keeper/stream_settle_test.go::TestStreamMeterThenClose`
- `modules/layerxpay/keeper/stream_lifecycle_test.go::TestStreamPauseResumeStopsAccrual`
- `modules/layerxpay/keeper/stream_lifecycle_test.go::TestStreamRateChangeSettlesAtOldRate`
- `modules/layerxpay/keeper/stream_views_test.go::TestStreamAccruedAtBlockOpen`
- `modules/layerxpay/keeper/stream_views_test.go::TestStreamRecordAndResultViews`
- `modules/layerxpay/keeper/stream_agent_test.go::TestAgentStreamOpenMapsTokenAndUnits`
- `modules/layerxpay/keeper/stream_agent_test.go::TestAgentStreamSettleCloseRateAndView`
- `modules/layerxpay/keeper/stream_agent_test.go::TestAgentStreamUnknownTokenRefused`

### Service

- `modules/layerxpay/keeper/service_offer_test.go::TestServiceOfferPublishWithdraw`
- `modules/layerxpay/keeper/service_agreement_test.go::TestServiceAgreementProposeAccept`
- `modules/layerxpay/keeper/service_agreement_test.go::TestServiceCommitAndAbandon`
- `modules/layerxpay/keeper/service_delivery_test.go::TestServiceDeliverAcceptFlow`
- `modules/layerxpay/keeper/service_delivery_test.go::TestServiceRejectAndToolAttest`
- `modules/layerxpay/keeper/service_dispute_test.go::TestServiceDisputeResolvedByEscrowArbiter`
- `modules/layerxpay/keeper/service_dispute_test.go::TestServiceDisputeArbiterRefusals`
- `modules/layerxpay/keeper/service_views_test.go::TestServiceViewsReadKernelRecords`
- `modules/layerxpay/keeper/service_views_test.go::TestServiceViewsUnknownEmpty`

### Native perps, spot and oracle intake

- `modules/perps/keeper/msg_server_market_test.go::TestMsgSetMarketListsNewMarket`
- `modules/perps/keeper/msg_server_market_test.go::TestMsgSetMarketRefusesUnlistedPricefeedMarket`
- `modules/perps/keeper/msg_server_market_test.go::TestMsgSetMarketRefusesIdentityChange`
- `modules/perps/keeper/funding_test.go::TestFundingPremiumSample`
- `modules/perps/keeper/funding_test.go::TestFundingRateFormula`
- `modules/perps/keeper/funding_test.go::TestFundingSettlementIndex`
- `modules/perps/keeper/funding_test.go::TestFundingRoundingTowardVenue`
- `modules/perps/keeper/funding_test.go::TestFundingBeforeLiquidation`
- `modules/perps/keeper/liquidation_test.go::TestLiquidationAccountLevel`
- `modules/perps/keeper/liquidation_test.go::TestLiquidationSlices`
- `modules/perps/keeper/liquidation_test.go::TestLiquidationPerBlockBound`
- `modules/perps/keeper/liquidation_test.go::TestLiquidationPriceFormula`
- `modules/perps/keeper/liquidation_test.go::TestLiquidationHealthyRefused`

### Launchpad

- `modules/launchpad/keeper/keeper_test.go::TestCreateMarketChargesFeeAndEscrowsSupply`
- `modules/launchpad/keeper/keeper_test.go::TestSwapVectors`
- `modules/launchpad/keeper/keeper_test.go::TestBuyMovesUsdlIntoCustody`
- `modules/launchpad/keeper/keeper_test.go::TestSnapshotRingWraps`
- `modules/launchpad/keeper/keeper_test.go::TestQuoteConservation`
- `modules/launchpad/keeper/keeper_test.go::TestSwapRejections`
- `modules/launchpad/keeper/keeper_test.go::TestFeeSplitMatchesFeeAccumulator`
- `modules/launchpad/keeper/keeper_test.go::TestFeeStrategies`
- `modules/launchpad/keeper/keeper_test.go::TestAirdropSharesByHolding`
- `modules/launchpad/keeper/keeper_test.go::TestGuardianPause`
- `modules/launchpad/keeper/keeper_test.go::TestParamsChangeOnlyThroughGovernance`
- `modules/launchpad/keeper/keeper_test.go::TestGenesisRoundTrip`
- `modules/launchpad/keeper/keeper_test.go::TestMsgUpdateParamsGovernanceRoute`
- `modules/launchpad/keeper/keeper_test.go::TestMsgUpdateParamsCodecAndSigners`
- `modules/launchpad/keeper/keeper_test.go::TestMsgUpdateParamsRefusals`
- `modules/launchpad/keeper/keeper_test.go::TestAirdropEntitlements`
- `modules/launchpad/keeper/history_test.go::TestAirdropHoldingHistory`

### Web requests, origin 1 and origin 2

- `modules/xweb/keeper/request_test.go::TestRequestStoresUnderTheNextNonceAndTakesTheFee`
- `modules/xweb/keeper/request_test.go::TestRequestEventCarriesWhatAnAttestorNeeds`
- `modules/xweb/keeper/request_test.go::TestRequestRefusals`
- `modules/xweb/keeper/request_test.go::TestApiRequestStoresTheLevelAndTheNamedAttestor`
- `modules/xweb/keeper/request_test.go::TestApiRequestRefusals`
- `modules/xweb/keeper/fulfil_test.go::TestFulfilAtThresholdStoresTheResultAndSplitsTheFee`
- `modules/xweb/keeper/fulfil_test.go::TestFulfilByEveryAttestorSplitsThreeWays`
- `modules/xweb/keeper/fulfil_test.go::TestFulfilKeeperAttestationMatchesTheIndependentPreimage`
- `modules/xweb/keeper/fulfil_test.go::TestFulfilSignatureRefusals`
- `modules/xweb/keeper/fulfil_test.go::TestFulfilSizeRefusals`
- `modules/xweb/keeper/fulfil_test.go::TestFulfilStateRefusals`
- `modules/xweb/keeper/fulfil_test.go::TestRecordCallback`
- `modules/xweb/keeper/fulfil_test.go::TestFulfilSingleLevelTakesTheNamedAttestorAlone`
- `modules/xweb/keeper/fulfil_test.go::TestFulfilSingleLevelRefusals`
- `modules/xweb/keeper/fulfil_test.go::TestFulfilMajorityApiRequestKeepsTheThreshold`
- `modules/xweb/keeper/refund_test.go::TestRefundAfterTimeoutReturnsTheFeeOnce`
- `modules/xweb/keeper/refund_test.go::TestRefundRefusals`
- `modules/xweb/keeper/refund_test.go::TestRefundKeepsTheStoredTimeoutAndWorksWhilePaused`

### Programs

- `programs/crates/layerx-programs-runtime/tests/lifecycle.rs::deploy_returns_an_opaque_receipt_without_a_runtime_trust_bypass`
- `programs/crates/layerx-programs-runtime/tests/lifecycle.rs::immutable_is_default_and_upgrade_records_hash_history`
- `programs/crates/layerx-programs-runtime/tests/lifecycle.rs::deploy_and_upgrade_record_exact_hashes_and_migration_outcome`
- `programs/crates/layerx-programs-runtime/tests/lifecycle.rs::deploy_refuses_a_mismatched_code_hash_without_installing_the_program`
- `programs/crates/layerx-programs-runtime/tests/lifecycle.rs::upgrade_refuses_a_mismatched_code_hash_without_advancing_history`
- `programs/crates/layerx-programs-runtime/tests/lifecycle.rs::zero_upgrade_authorities_are_never_admitted`
- `programs/crates/layerx-programs-runtime/tests/lifecycle.rs::failed_validation_is_preserved_but_never_executable`
- `programs/crates/layerx-programs-runtime/tests/lifecycle.rs::unknown_program_and_incompatible_abi_are_typed`
- `programs/crates/layerx-programs-runtime/tests/lifecycle.rs::abi_upgrades_are_monotonic_and_historical_versions_remain_admitted`
- `programs/crates/layerx-programs-runtime/tests/lifecycle.rs::every_supported_abi_transition_preserves_lifecycle_history`

### AI and market guests

- `programs/crates/layerx-programs-ai-market/tests/market_lifecycle.rs::a01_create_registers_once_at_committed_origin`
- `programs/crates/layerx-programs-ai-market/tests/market_lifecycle.rs::a03_policy_staging_activation_boundary_cancellation_and_history`
- `programs/crates/layerx-programs-ai-market/tests/market_lifecycle.rs::a05_suspend_preserves_admitted_work_and_clears_schedule`
- `programs/crates/layerx-programs-ai-market/tests/market_lifecycle.rs::a06_concurrent_stage_stale_revision_replay_and_domain_binding`
- `programs/crates/layerx-programs-ai-market/tests/market_lifecycle.rs::a10_operator_grant_replacement_revocation_and_metadata_isolation`
- `programs/crates/layerx-programs-ai-market/tests/market_lifecycle.rs::a19_treasury_binding_presence_rules_and_section_worksheet`

### Web search sidecar and 402LXP payment

- `services/interop/crates/x-websearch/tests/submit.rs::the_kernel_network_id_is_read_from_the_xweb_precompile`
- `services/interop/crates/x-websearch/tests/submit.rs::fulfil_calldata_and_the_signed_transaction_match_an_independent_encoder`
- `services/interop/crates/x-websearch/tests/payment.rs::a_pending_exact_payment_recovers_from_the_sequencer_receipt_and_releases_once`
- `services/interop/crates/x-websearch/tests/attest.rs::the_watcher_follows_requested_logs_to_the_confirmation_depth`
- `services/interop/crates/x-websearch/tests/attest.rs::scanned_requests_stay_journalled_across_restart_until_retired`
- `services/interop/crates/x-websearch/tests/api.rs::the_api_payload_codec_matches_every_pinned_vector_and_refusal`

### Bridge

- `modules/layerxbridge/keeper/keeper_test.go::TestDefaultGenesisIsDormant`
- `modules/layerxbridge/keeper/keeper_test.go::TestGovernanceMessagesRequireTheAuthority`
- `modules/layerxbridge/keeper/keeper_test.go::TestMsgServiceRoutesEveryGovernanceMessageToTheKeeper`
- `modules/layerxbridge/keeper/keeper_test.go::TestMsgServiceRefusesAWrongAuthority`
- `modules/layerxbridge/keeper/keeper_test.go::TestBridgeInMintsAtThreshold`
- `modules/layerxbridge/keeper/keeper_test.go::TestBridgeInRefusals`
- `modules/layerxbridge/keeper/keeper_test.go::TestBridgeOutBurnsAndEmits`
- `modules/layerxbridge/keeper/keeper_test.go::TestGenesisRoundTrip`
- `modules/layerxbridge/keeper/keeper_test.go::TestProposalHandlerExecutesEveryCarriedMessage`
- `modules/layerxbridge/keeper/keeper_test.go::TestProposalHandlerRefusesAWrongAuthority`
- `modules/layerxbridge/keeper/keeper_test.go::TestProposalHandlerRefusesAMalformedProposal`
- `modules/layerxbridge/keeper/keeper_test.go::TestSidioraHomeChainIDIsSolana`
- `modules/layerxbridge/keeper/keeper_test.go::TestMsgServiceRegistersTheSidioraPair`
- `modules/layerxbridge/keeper/keeper_test.go::TestSidioraPairIsRefusedOnceACapBoundItElsewhere`
- `modules/layerxbridge/keeper/keeper_test.go::TestProposalHandlerRegistersTheSidioraPairAheadOfItsCap`
- `modules/layerxbridge/keeper/keeper_test.go::TestBridgeOutReleaseCapBoundary`
- `modules/layerxbridge/keeper/attestation_test.go::TestDigestVectorsFromAttestationDoc`

### Agent daemon, SDKs, MCP and CLI

- `services/agent/crates/layerx-sdk/tests/native_programs.rs::native_request_binds_real_signed_fixture`

### Hosted gateway, authority and indexer

- `services/platform/hosted/gateway/tests/one_source_reads.rs::px_get_balances_reads_bank_all_balances_at_one_height`
- `services/platform/hosted/gateway/tests/one_source_reads.rs::px_get_account_reads_no_kernel_backend`
- `services/platform/hosted/gateway/tests/one_source_reads.rs::px_list_assets_joins_indexer_assets_with_chain_metadata`
- `services/platform/hosted/gateway/tests/one_source_reads.rs::px_get_network_reports_the_consensus_head`

### Wallet, human service and explorer

- `apps/wallet/e2e/send.spec.ts::send_shows_receipted_from_the_sequencer_receipt_and_keeps_it_on_reload`

### Wallet, human service and explorer

- `apps/wallet/src/surfaces/BridgeView.test.tsx::bridge_view_shows_the_receipted_rung_and_events_from_the_sequencer_receipt`
- `apps/wallet/src/account/StatusLadder.test.tsx::status_ladder_labels_submitted_receipted_and_committed`
- `apps/wallet/src/account/StatusLadder.test.tsx::transaction_ladder_reaches_receipted_from_the_sequencer_before_the_explorer`

### Cut-over balances and genesis import

- `modules/layerxcustody/keeper/release_test.go::TestReleaseRetiredMovesEveryCustodiedBalanceToTheAuthority`
- `modules/layerxcustody/keeper/release_test.go::TestReleaseRetiredCancelsPendingClaims`
- `modules/layerxcustody/keeper/release_test.go::TestReleaseRetiredIsRefusedBeforeTheMergePlanHeight`

### Retired two-domain surfaces

- `precompiles/layerxcustody/retire_test.go::TestEveryCustodyMethodRevertsModuleDisabledFromTheMergePlanHeight`
- `precompiles/layerxcustody/retire_test.go::TestCustodyRetiredGateRunsBeforeTheCallShapeChecks`
- `precompiles/layerxcustody/retire_test.go::TestCustodyViewsRevertModuleDisabledUnderStaticcall`

### Mirror

- `services/interop/crates/layerx-mirror/tests/publish.rs::confirms_and_retrieves_the_same_archive_on_both_test_networks`
- `services/interop/crates/layerx-mirror/tests/publish.rs::publishes_pure_archives_without_custody_semantics`
- `services/interop/crates/layerx-mirror/tests/sequencer_vectors.rs::every_shared_receipted_case_verifies_in_the_mirror`
- `services/interop/crates/layerx-mirror/tests/sequencer_vectors.rs::every_shared_v3_case_is_classified_at_the_signature_only_boundary`
- `services/interop/crates/layerx-mirror/tests/sequencer_vectors.rs::shared_signature_and_decode_refusals_reach_the_mirror_consumer`
- `services/interop/crates/layerx-mirror/tests/committed_freshness.rs::freshness_reports_committed_lag_in_blocks`

## Additional behavior coverage

| Behavior | Selectors |
| --- | --- |
| Sequencer | `node/sequencer/executor_test.go::TestExecutorSequentialOrder`<br>`node/sequencer/mixed_load_test.go::TestMixedLoadSealedEqualsProcessBlock` |
| Sequencer receipt | `node/sequencer/receipt_equality_test.go::TestReceiptEqualsCommittedReceipt` |
| Sequencer standby | `node/sequencer/standby_test.go::TestStandbyReplayMatchesPrimary` |
| Order commitment | `node/process_proposal_order_test.go::TestOrderCheckRefusesSeqGap`<br>`node/process_proposal_order_test.go::TestOrderCheckRefusesPrevSeqHashMismatch` |
| Sponsored gas | `services/interop/crates/layerx-gas-station/tests/did_budget.rs::accounts_sharing_did_share_budget` |
| Asset ledger idempotency | `modules/layerxledger/keeper/execute_test.go::TestExecuteIdempotentReplay` |
| Budget | `modules/layerxpay/keeper/budget_create_test.go::TestBudgetCreateAndFund`<br>`modules/layerxpay/keeper/budget_create_test.go::TestBudgetNativeVariantsAndAmend`<br>`modules/layerxpay/keeper/budget_spend_test.go::TestBudgetSpendWithinPeriodLimit`<br>`modules/layerxpay/keeper/budget_spend_test.go::TestBudgetDelegateSpendDerivedKey`<br>`modules/layerxpay/keeper/budget_pull_test.go::TestBudgetPullIsRetired`<br>`modules/layerxpay/keeper/budget_pull_test.go::TestBudgetPullRefusesMalformedGrant`<br>`modules/layerxpay/keeper/budget_close_test.go::TestBudgetCloseDrainsToOwner`<br>`modules/layerxpay/keeper/budget_close_test.go::TestBudgetDefundAndRevoke`<br>`modules/layerxpay/keeper/budget_views_test.go::TestBudgetViewsReadKernelRecord` |
| Governance pause | `modules/layerxgov/handler_pay_pause_live_test.go::TestLayerXProposalPausesPayLive`<br>`modules/layerxgov/handler_pause_live_test.go::TestLayerXProposalPausesLedgerAndAuthLive` |
| Programs registry | `precompiles/layerxprograms/method_registry_test.go::TestProgramsRegistryReadEmitsEvent`<br>`precompiles/layerxprograms/method_registry_test.go::TestProgramsRegistryReadZeroProgramReverts`<br>`precompiles/layerxprograms/view_registry_test.go::TestProgramsViewGetProgramRecord`<br>`precompiles/layerxprograms/view_registry_test.go::TestProgramsViewGetInterfaceAndAbsent` |
| Programs deploy | `precompiles/layerxprograms/method_deploy_test.go::TestProgramsDeployRegistersProgram`<br>`precompiles/layerxprograms/method_deploy_test.go::TestProgramsDeployRefusesCodeHashMismatch`<br>`precompiles/layerxprograms/method_upgrade_test.go::TestProgramsUpgradeReplacesCodeHash`<br>`precompiles/layerxprograms/method_upgrade_test.go::TestProgramsUpgradeRefusesStaleOldHash` |
| Programs events | `precompiles/layerxprograms/events_test.go::TestProgramsLifecycleEventsFromReceipt`<br>`precompiles/layerxprograms/events_test.go::TestProgramsWindDownAndAccountEventsFromReceipt`<br>`precompiles/layerxprograms/events_call_test.go::TestProgramsGuestEventLogs`<br>`precompiles/layerxprograms/events_call_test.go::TestProgramsCallOutcomeLog`<br>`precompiles/layerxprograms/events_sandbox_test.go::TestProgramsSandboxUsageReceiptLog`<br>`precompiles/layerxprograms/events_sandbox_test.go::TestProgramsSandboxDestroyedAndRetiredLogs` |
| Programs archive proof | `programs/crates/layerx-programs-registry/tests/chain_verifier_deployment.rs::chain_deployment_current_and_historical_verify`<br>`programs/crates/layerx-programs-registry/tests/chain_verifier_deployment.rs::chain_deployment_refuses_code_hash_mismatch` |
| Web origin 2 | `modules/xweb/keeper/program_kernel_parity_test.go::TestFulfilProgramMatchesTheKernelRefusalMatrix`<br>`modules/xweb/keeper/program_kernel_parity_test.go::TestFulfilProgramAcceptsTheKernelFirstObservation`<br>`modules/xweb/keeper/program_record_parity_test.go::TestDecodeProgramRequestMatchesTheKernelRecordRows`<br>`modules/xweb/keeper/program_record_parity_test.go::TestFulfilProgramTwiceMatchesTheKernelReuseRefusal`<br>`modules/xweb/keeper/program_read_parity_test.go::TestGetProgramResultMatchesTheKernelCommittedRead` |
| Perps markets | `modules/perps/keeper/msg_server_market_test.go::TestMsgSetMarketListsNewMarket`<br>`modules/perps/keeper/msg_server_market_test.go::TestMsgSetMarketRefusesUnlistedPricefeedMarket`<br>`modules/perps/keeper/msg_server_market_test.go::TestMsgSetMarketRefusesIdentityChange` |
| Agent signing | `services/agent/crates/layerx-agentd/tests/evm_preimage_golden.rs::prepared_transfer_signing_hash_matches_the_eip1559_golden`<br>`services/agent/crates/layerx-agentd/tests/evm_preimage_golden.rs::prepare_refuses_an_expected_nonce_behind_the_pending_nonce`<br>`services/agent/crates/layerx-agentd/tests/evm_preimage_golden.rs::prepare_derives_fee_caps_from_the_head_base_fee` |
| Agent web payer | `services/agent/crates/layerx-agentd-host/tests/payer.rs::web_payment_receipt_must_be_signed_by_a_published_key`<br>`services/agent/crates/layerx-agentd-host/tests/payer.rs::web_payment_reports_the_pay_exact_log_amount` |
| Hosted gateway balances | `services/platform/hosted/gateway/tests/one_source_reads.rs::px_get_balances_reads_bank_all_balances_at_one_height`<br>`services/platform/hosted/gateway/tests/one_source_reads.rs::px_get_account_reads_no_kernel_backend` |
| Explorer signature | `services/explorer/backend/apps/block_scout_web/test/block_scout_web/controllers/api/v2/paxeer_x/transaction_signature_controller_test.exs::signature_route_returns_verified_receipt`<br>`services/explorer/backend/apps/block_scout_web/test/block_scout_web/controllers/api/v2/paxeer_x/transaction_signature_controller_test.exs::signature_route_404_when_unverified` |
| Custody balances | `modules/layerxcustody/keeper/release_test.go::TestReleaseRetiredMovesEveryCustodiedBalanceToTheAuthority`<br>`modules/layerxcustody/keeper/release_test.go::TestReleaseRetiredCancelsPendingClaims`<br>`modules/layerxcustody/keeper/release_test.go::TestReleaseRetiredIsRefusedBeforeTheMergePlanHeight` |
| Anchor bonds | `modules/layerxanchor/keeper/retire_merge_test.go::TestRetireAtMergeReturnsBondsAndEmits` |
| Signed-receipt logs | `apps/wallet/e2e/send.spec.ts::send_shows_receipted_from_the_sequencer_receipt_and_keeps_it_on_reload`<br>`apps/wallet/src/surfaces/BridgeView.test.tsx::bridge_view_shows_the_receipted_rung_and_events_from_the_sequencer_receipt`<br>`apps/wallet/src/lib/swap/executor.test.ts::execute_swap_returns_after_the_receipt`<br>`apps/wallet/src/lib/swap/executor.test.ts::execute_swap_refuses_a_reverted_swap` |
| Retired methods | `precompiles/layerxcustody/retire_test.go::TestEveryCustodyMethodRevertsModuleDisabledFromTheMergePlanHeight`<br>`precompiles/layerxcustody/retire_test.go::TestCustodyRetiredGateRunsBeforeTheCallShapeChecks`<br>`precompiles/layerxcustody/retire_test.go::TestCustodyViewsRevertModuleDisabledUnderStaticcall` |
| Web conformance digest | `services/interop/crates/x-websearch/tests/binary.rs::the_pinned_conformance_suite_is_the_recorded_gateway_exchange`<br>`services/interop/crates/x-websearch/tests/binary.rs::the_conformance_vector_count_is_the_number_of_recorded_answers` |

## Activation and retirement

Custody release cancels pending claims and transfers registered custody balances
to the configured authority. Anchor retirement refunds bonds to their recorded
operators and returns open challenge deposits to challengers. These are separate
operations within the merge handler; they do not import a second ledger or create
unbound-account claims. The anchor store is deleted by a later plan, with the
executor retained for retirement responses.

### Balance conservation

- `modules/layerxcustody/keeper/release_test.go::TestReleaseRetiredMovesEveryCustodiedBalanceToTheAuthority`
- `modules/layerxcustody/keeper/release_test.go::TestReleaseRetiredCancelsPendingClaims`
- `modules/layerxcustody/keeper/release_test.go::TestReleaseRetiredIsRefusedBeforeTheMergePlanHeight`

### Bond return

- `modules/layerxanchor/keeper/retire_merge_test.go::TestRetireAtMergeReturnsBondsAndEmits`

### Receipt-order equality

- `node/sequencer/receipt_equality_test.go::TestReceiptEqualsCommittedReceipt`
- `node/sequencer/executor_test.go::TestExecutorSequentialOrder`
- `node/sequencer/mixed_load_test.go::TestMixedLoadSealedEqualsProcessBlock`

The signed receipt's full-block position includes the sequence batch at position
zero. JSON-RPC exposes an EVM-only transaction index. The receipt equality tests
compare those projections explicitly. See the
[activation and migration page](../../docs/plans/activation-and-migration.md) for
height boundaries and the [test plan](../../docs/plans/test-plan.md) for focused
runner examples and native-library dependencies.
