# Cosmwasm contract

Source revision: `2a1033aa2`. This contract freezes the cited interface and acceptance blocks of the specification. Statuses include the specification overrides. A source marked planned has line 0; a positive source line is verified in the pinned revision.

The context and decision tables reproduce the rules linked by interface identifiers. An anchor without a linked context gains no additional storage, gas, activation or pause rule here. Acceptance qualification remains UNRUN until its declared gate succeeds.

## capabilities

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.5.wasm-capabilities</code> | <code>wasm_binding</code> | <code>stable</code> | <code>node/app.go:772</code> | <code>supportedFeatures := "iterator,staking,stargate,pax" (wasmvm.NewVM supportedCapabilities)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.5.wasm-capabilities</code> | notes | Mismatch: tests/integration/contracts/mars.wasm, saturn.wasm, venus.wasm, tests/chain/mars.wasm and tools/benchmark/loadtest/contracts/pax_tester.wasm export a requires_ capability for the custom bindings under a name other than pax, which this list does not contain, so StoreCode rejects them during static analysis; tests/occ/utils/utils.go:160 loads mars.wasm; wasm/x/wasm/keeper/testdata/cyberpunk.wasm exports requires_cosmwasm_1_1..1_3, also absent, while the keeper tests use SupportedFeatures "iterator,staking,stargate" (wasm/x/wasm/keeper/keeper_test.go:42). 7 in-repo callers (6 non-test, 1 test). |
| <code>interface.5.wasm-capabilities</code> | overridden_by | override.J31.1 |

## cli

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.6.cli-wasm-all-j37</code> | <code>cli_command</code> | <code>stable</code> | <code>wasm/x/wasm/client/cli/query.go:278</code> | <code>cobra Use: "all [bech32_address]" (built by GetCmdGetContractStateAll)</code> |
| <code>interface.6.cli-wasm-clear-contract-admin-2-j37</code> | <code>cli_command</code> | <code>stable</code> | <code>wasm/x/wasm/client/cli/new_tx.go:99</code> | <code>cobra Use: "clear-contract-admin [contract_addr_bech32]" (built by ClearContractAdminCmd)</code> |
| <code>interface.6.cli-wasm-clear-contract-admin-j37</code> | <code>cli_command</code> | <code>stable</code> | <code>wasm/x/wasm/client/cli/gov_tx.go:437</code> | <code>cobra Use: "clear-contract-admin [contract_addr_bech32]" (built by ProposalClearContractAdminCmd)</code> |
| <code>interface.6.cli-wasm-code-info-j37</code> | <code>cli_command</code> | <code>stable</code> | <code>wasm/x/wasm/client/cli/query.go:187</code> | <code>cobra Use: "code-info [code_id]" (built by GetCmdQueryCodeInfo)</code> |
| <code>interface.6.cli-wasm-code-j37</code> | <code>cli_command</code> | <code>stable</code> | <code>wasm/x/wasm/client/cli/query.go:146</code> | <code>cobra Use: "code [code_id] [output filename]" (built by GetCmdQueryCode)</code> |
| <code>interface.6.cli-wasm-contract-history-j37</code> | <code>cli_command</code> | <code>stable</code> | <code>wasm/x/wasm/client/cli/query.go:408</code> | <code>cobra Use: "contract-history [bech32_address]" (built by GetCmdGetContractHistory)</code> |
| <code>interface.6.cli-wasm-contract-j37</code> | <code>cli_command</code> | <code>stable</code> | <code>wasm/x/wasm/client/cli/query.go:226</code> | <code>cobra Use: "contract [bech32_address]" (built by GetCmdGetContractInfo)</code> |
| <code>interface.6.cli-wasm-contract-state-j37</code> | <code>cli_command</code> | <code>stable</code> | <code>wasm/x/wasm/client/cli/query.go:261</code> | <code>cobra Use: "contract-state" (built by GetCmdGetContractState)</code> |
| <code>interface.6.cli-wasm-execute-2-j37</code> | <code>cli_command</code> | <code>stable</code> | <code>wasm/x/wasm/client/cli/tx.go:238</code> | <code>cobra Use: "execute [contract_addr_bech32] [json_encoded_send_args] --amount [coins,optional]" (built by ExecuteContractCmd)</code> |
| <code>interface.6.cli-wasm-execute-contract-j37</code> | <code>cli_command</code> | <code>stable</code> | <code>wasm/x/wasm/client/cli/gov_tx.go:237</code> | <code>cobra Use: "execute-contract [contract_addr_bech32] [json_encoded_migration_args]" (built by ProposalExecuteContractCmd)</code> |
| <code>interface.6.cli-wasm-execute-j37</code> | <code>cli_command</code> | <code>stable</code> | <code>wasm/x/wasm/client/cli/genesis_msg.go:155</code> | <code>cobra Use: "execute [contract_addr_bech32] [json_encoded_send_args] --run-as [address] --amount [coins,optional]" (built by GenesisExecuteContractCmd)</code> |
| <code>interface.6.cli-wasm-instantiate-contract-2-j37</code> | <code>cli_command</code> | <code>stable</code> | <code>wasm/x/wasm/client/cli/gov_tx.go:96</code> | <code>cobra Use: "instantiate-contract [code_id_int64] [json_encoded_init_args] --label [text] --title [text] --description [text] --run-as [address] --admin [address,optional] --amount [coins,optional]" (built by ProposalInstantiateContractCmd)</code> |
| <code>interface.6.cli-wasm-instantiate-contract-j37</code> | <code>cli_command</code> | <code>stable</code> | <code>wasm/x/wasm/client/cli/genesis_msg.go:87</code> | <code>cobra Use: "instantiate-contract [code_id_int64] [json_encoded_init_args] --label [text] --run-as [address] --admin [address,optional] --amount [coins,optional]" (built by GenesisInstantiateContractCmd)</code> |
| <code>interface.6.cli-wasm-instantiate-j37</code> | <code>cli_command</code> | <code>stable</code> | <code>wasm/x/wasm/client/cli/tx.go:155</code> | <code>cobra Use: "instantiate [code_id_int64] [json_encoded_init_args] --label [text] --admin [address,optional] --amount [coins,optional]" (built by InstantiateContractCmd)</code> |
| <code>interface.6.cli-wasm-libwasmvm-version-j37</code> | <code>cli_command</code> | <code>stable</code> | <code>wasm/x/wasm/client/cli/query.go:48</code> | <code>cobra Use: "libwasmvm-version" (built by GetCmdLibVersion)</code> |
| <code>interface.6.cli-wasm-list-code-j37</code> | <code>cli_command</code> | <code>stable</code> | <code>wasm/x/wasm/client/cli/query.go:68</code> | <code>cobra Use: "list-code" (built by GetCmdListCode)</code> |
| <code>interface.6.cli-wasm-list-codes-j37</code> | <code>cli_command</code> | <code>stable</code> | <code>wasm/x/wasm/client/cli/genesis_msg.go:205</code> | <code>cobra Use: "list-codes " (built by GenesisListCodesCmd)</code> |
| <code>interface.6.cli-wasm-list-contract-by-code-j37</code> | <code>cli_command</code> | <code>stable</code> | <code>wasm/x/wasm/client/cli/query.go:104</code> | <code>cobra Use: "list-contract-by-code [code_id]" (built by GetCmdListContractByCode)</code> |
| <code>interface.6.cli-wasm-list-contracts-j37</code> | <code>cli_command</code> | <code>stable</code> | <code>wasm/x/wasm/client/cli/genesis_msg.go:229</code> | <code>cobra Use: "list-contracts " (built by GenesisListContractsCmd)</code> |
| <code>interface.6.cli-wasm-migrate-contract-j37</code> | <code>cli_command</code> | <code>stable</code> | <code>wasm/x/wasm/client/cli/gov_tx.go:174</code> | <code>cobra Use: "migrate-contract [contract_addr_bech32] [new_code_id_int64] [json_encoded_migration_args]" (built by ProposalMigrateContractCmd)</code> |
| <code>interface.6.cli-wasm-migrate-j37</code> | <code>cli_command</code> | <code>stable</code> | <code>wasm/x/wasm/client/cli/new_tx.go:18</code> | <code>cobra Use: "migrate [contract_addr_bech32] [new_code_id_int64] [json_encoded_migration_args]" (built by MigrateContractCmd)</code> |
| <code>interface.6.cli-wasm-pin-codes-j37</code> | <code>cli_command</code> | <code>stable</code> | <code>wasm/x/wasm/client/cli/gov_tx.go:492</code> | <code>cobra Use: "pin-codes [code-ids]" (built by ProposalPinCodesCmd)</code> |
| <code>interface.6.cli-wasm-pinned-j37</code> | <code>cli_command</code> | <code>stable</code> | <code>wasm/x/wasm/client/cli/query.go:452</code> | <code>cobra Use: "pinned" (built by GetCmdListPinnedCode)</code> |
| <code>interface.6.cli-wasm-raw-j37</code> | <code>cli_command</code> | <code>stable</code> | <code>wasm/x/wasm/client/cli/query.go:319</code> | <code>cobra Use: "raw [bech32_address] [key]" (built by GetCmdGetContractStateRaw)</code> |
| <code>interface.6.cli-wasm-set-contract-admin-2-j37</code> | <code>cli_command</code> | <code>stable</code> | <code>wasm/x/wasm/client/cli/new_tx.go:63</code> | <code>cobra Use: "set-contract-admin [contract_addr_bech32] [new_admin_addr_bech32]" (built by UpdateContractAdminCmd)</code> |
| <code>interface.6.cli-wasm-set-contract-admin-j37</code> | <code>cli_command</code> | <code>stable</code> | <code>wasm/x/wasm/client/cli/gov_tx.go:376</code> | <code>cobra Use: "set-contract-admin [contract_addr_bech32] [new_admin_addr_bech32]" (built by ProposalUpdateContractAdminCmd)</code> |
| <code>interface.6.cli-wasm-smart-j37</code> | <code>cli_command</code> | <code>stable</code> | <code>wasm/x/wasm/client/cli/query.go:360</code> | <code>cobra Use: "smart [bech32_address] [query]" (built by GetCmdGetContractStateSmart)</code> |
| <code>interface.6.cli-wasm-store-2-j37</code> | <code>cli_command</code> | <code>stable</code> | <code>wasm/x/wasm/client/cli/tx.go:57</code> | <code>cobra Use: "store [wasm file]" (built by StoreCodeCmd)</code> |
| <code>interface.6.cli-wasm-store-j37</code> | <code>cli_command</code> | <code>stable</code> | <code>wasm/x/wasm/client/cli/genesis_msg.go:47</code> | <code>cobra Use: "store [wasm file] --run-as [owner_address_or_key_name]\"," (built by GenesisStoreCodeCmd)</code> |
| <code>interface.6.cli-wasm-sudo-contract-j37</code> | <code>cli_command</code> | <code>stable</code> | <code>wasm/x/wasm/client/cli/gov_tx.go:316</code> | <code>cobra Use: "sudo-contract [contract_addr_bech32] [json_encoded_migration_args]" (built by ProposalSudoContractCmd)</code> |
| <code>interface.6.cli-wasm-types-modulename-2-j37</code> | <code>cli_command</code> | <code>stable</code> | <code>wasm/x/wasm/client/cli/tx.go:37</code> | <code>cobra Use: types.ModuleName (built by GetTxCmd)</code> |
| <code>interface.6.cli-wasm-types-modulename-j37</code> | <code>cli_command</code> | <code>stable</code> | <code>wasm/x/wasm/client/cli/query.go:25</code> | <code>cobra Use: types.ModuleName (built by GetQueryCmd)</code> |
| <code>interface.6.cli-wasm-unpin-codes-j37</code> | <code>cli_command</code> | <code>stable</code> | <code>wasm/x/wasm/client/cli/gov_tx.go:563</code> | <code>cobra Use: "unpin-codes [code-ids]" (built by ProposalUnpinCodesCmd)</code> |
| <code>interface.6.cli-wasm-update-instantiate-config-j37</code> | <code>cli_command</code> | <code>stable</code> | <code>wasm/x/wasm/client/cli/gov_tx.go:665</code> | <code>cobra Use: "update-instantiate-config [code-id,permission]..." (built by ProposalUpdateInstantiateConfigCmd)</code> |
| <code>interface.6.cli-wasm-wasm-store-j37</code> | <code>cli_command</code> | <code>stable</code> | <code>wasm/x/wasm/client/cli/gov_tx.go:22</code> | <code>cobra Use: "wasm-store [wasm file] --title [text] --description [text] --run-as [address]" (built by ProposalStoreCodeCmd)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.6.cli-wasm-all-j37</code> | notes | CLI command built by GetCmdGetContractStateAll in wasm/x/wasm/client/cli; 1 caller site(s) of GetCmdGetContractStateAll listed. |
| <code>interface.6.cli-wasm-clear-contract-admin-2-j37</code> | notes | CLI command built by ClearContractAdminCmd in wasm/x/wasm/client/cli; 1 caller site(s) of ClearContractAdminCmd listed. |
| <code>interface.6.cli-wasm-clear-contract-admin-j37</code> | notes | CLI command built by ProposalClearContractAdminCmd in wasm/x/wasm/client/cli; 1 caller site(s) of ProposalClearContractAdminCmd listed. |
| <code>interface.6.cli-wasm-code-info-j37</code> | notes | CLI command built by GetCmdQueryCodeInfo in wasm/x/wasm/client/cli; 1 caller site(s) of GetCmdQueryCodeInfo listed. |
| <code>interface.6.cli-wasm-code-j37</code> | notes | CLI command built by GetCmdQueryCode in wasm/x/wasm/client/cli; 1 caller site(s) of GetCmdQueryCode listed. |
| <code>interface.6.cli-wasm-contract-history-j37</code> | notes | CLI command built by GetCmdGetContractHistory in wasm/x/wasm/client/cli; 1 caller site(s) of GetCmdGetContractHistory listed. |
| <code>interface.6.cli-wasm-contract-j37</code> | notes | CLI command built by GetCmdGetContractInfo in wasm/x/wasm/client/cli; 1 caller site(s) of GetCmdGetContractInfo listed. |
| <code>interface.6.cli-wasm-contract-state-j37</code> | notes | CLI command built by GetCmdGetContractState in wasm/x/wasm/client/cli; 1 caller site(s) of GetCmdGetContractState listed. |
| <code>interface.6.cli-wasm-execute-2-j37</code> | notes | CLI command built by ExecuteContractCmd in wasm/x/wasm/client/cli; 1 caller site(s) of ExecuteContractCmd listed. |
| <code>interface.6.cli-wasm-execute-contract-j37</code> | notes | CLI command built by ProposalExecuteContractCmd in wasm/x/wasm/client/cli; 1 caller site(s) of ProposalExecuteContractCmd listed. |
| <code>interface.6.cli-wasm-execute-j37</code> | notes | CLI command built by GenesisExecuteContractCmd in wasm/x/wasm/client/cli; 0 caller site(s) of GenesisExecuteContractCmd listed. |
| <code>interface.6.cli-wasm-instantiate-contract-2-j37</code> | notes | CLI command built by ProposalInstantiateContractCmd in wasm/x/wasm/client/cli; 1 caller site(s) of ProposalInstantiateContractCmd listed. |
| <code>interface.6.cli-wasm-instantiate-contract-j37</code> | notes | CLI command built by GenesisInstantiateContractCmd in wasm/x/wasm/client/cli; 0 caller site(s) of GenesisInstantiateContractCmd listed. |
| <code>interface.6.cli-wasm-instantiate-j37</code> | notes | CLI command built by InstantiateContractCmd in wasm/x/wasm/client/cli; 1 caller site(s) of InstantiateContractCmd listed. |
| <code>interface.6.cli-wasm-libwasmvm-version-j37</code> | notes | CLI command built by GetCmdLibVersion in wasm/x/wasm/client/cli; 1 caller site(s) of GetCmdLibVersion listed. |
| <code>interface.6.cli-wasm-list-code-j37</code> | notes | CLI command built by GetCmdListCode in wasm/x/wasm/client/cli; 1 caller site(s) of GetCmdListCode listed. |
| <code>interface.6.cli-wasm-list-codes-j37</code> | notes | CLI command built by GenesisListCodesCmd in wasm/x/wasm/client/cli; 0 caller site(s) of GenesisListCodesCmd listed. |
| <code>interface.6.cli-wasm-list-contract-by-code-j37</code> | notes | CLI command built by GetCmdListContractByCode in wasm/x/wasm/client/cli; 1 caller site(s) of GetCmdListContractByCode listed. |
| <code>interface.6.cli-wasm-list-contracts-j37</code> | notes | CLI command built by GenesisListContractsCmd in wasm/x/wasm/client/cli; 0 caller site(s) of GenesisListContractsCmd listed. |
| <code>interface.6.cli-wasm-migrate-contract-j37</code> | notes | CLI command built by ProposalMigrateContractCmd in wasm/x/wasm/client/cli; 1 caller site(s) of ProposalMigrateContractCmd listed. |
| <code>interface.6.cli-wasm-migrate-j37</code> | notes | CLI command built by MigrateContractCmd in wasm/x/wasm/client/cli; 1 caller site(s) of MigrateContractCmd listed. |
| <code>interface.6.cli-wasm-pin-codes-j37</code> | notes | CLI command built by ProposalPinCodesCmd in wasm/x/wasm/client/cli; 1 caller site(s) of ProposalPinCodesCmd listed. |
| <code>interface.6.cli-wasm-pinned-j37</code> | notes | CLI command built by GetCmdListPinnedCode in wasm/x/wasm/client/cli; 1 caller site(s) of GetCmdListPinnedCode listed. |
| <code>interface.6.cli-wasm-raw-j37</code> | notes | CLI command built by GetCmdGetContractStateRaw in wasm/x/wasm/client/cli; 1 caller site(s) of GetCmdGetContractStateRaw listed. |
| <code>interface.6.cli-wasm-set-contract-admin-2-j37</code> | notes | CLI command built by UpdateContractAdminCmd in wasm/x/wasm/client/cli; 1 caller site(s) of UpdateContractAdminCmd listed. |
| <code>interface.6.cli-wasm-set-contract-admin-j37</code> | notes | CLI command built by ProposalUpdateContractAdminCmd in wasm/x/wasm/client/cli; 1 caller site(s) of ProposalUpdateContractAdminCmd listed. |
| <code>interface.6.cli-wasm-smart-j37</code> | notes | CLI command built by GetCmdGetContractStateSmart in wasm/x/wasm/client/cli; 1 caller site(s) of GetCmdGetContractStateSmart listed. |
| <code>interface.6.cli-wasm-store-2-j37</code> | notes | CLI command built by StoreCodeCmd in wasm/x/wasm/client/cli; 1 caller site(s) of StoreCodeCmd listed. |
| <code>interface.6.cli-wasm-store-j37</code> | notes | CLI command built by GenesisStoreCodeCmd in wasm/x/wasm/client/cli; 0 caller site(s) of GenesisStoreCodeCmd listed. |
| <code>interface.6.cli-wasm-sudo-contract-j37</code> | notes | CLI command built by ProposalSudoContractCmd in wasm/x/wasm/client/cli; 1 caller site(s) of ProposalSudoContractCmd listed. |
| <code>interface.6.cli-wasm-types-modulename-2-j37</code> | notes | CLI command built by GetTxCmd in wasm/x/wasm/client/cli; 1 caller site(s) of GetTxCmd listed. |
| <code>interface.6.cli-wasm-types-modulename-j37</code> | notes | CLI command built by GetQueryCmd in wasm/x/wasm/client/cli; 1 caller site(s) of GetQueryCmd listed. |
| <code>interface.6.cli-wasm-unpin-codes-j37</code> | notes | CLI command built by ProposalUnpinCodesCmd in wasm/x/wasm/client/cli; 1 caller site(s) of ProposalUnpinCodesCmd listed. |
| <code>interface.6.cli-wasm-update-instantiate-config-j37</code> | notes | CLI command built by ProposalUpdateInstantiateConfigCmd in wasm/x/wasm/client/cli; 1 caller site(s) of ProposalUpdateInstantiateConfigCmd listed. |
| <code>interface.6.cli-wasm-wasm-store-j37</code> | notes | CLI command built by ProposalStoreCodeCmd in wasm/x/wasm/client/cli; 1 caller site(s) of ProposalStoreCodeCmd listed. |

## config

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.6.cfg-wasm-contract-debug-mode-trace</code> | <code>config_key</code> | <code>stable</code> | <code>wasm/x/wasm/module.go:274</code> | <code>global "trace" option -&gt; WasmConfig.ContractDebugMode (contract debug output to the VM printDebug flag)</code> |
| <code>interface.6.cfg-wasm-memory-cache-size</code> | <code>config_key</code> | <code>stable</code> | <code>wasm/x/wasm/module.go:37</code> | <code>app.toml [wasm] memory_cache_size -&gt; WasmConfig.MemoryCacheSize (ReadWasmConfig, opts.Get("wasm.memory_cache_size"))</code> |
| <code>interface.6.cfg-wasm-query-gas-limit</code> | <code>config_key</code> | <code>stable</code> | <code>wasm/x/wasm/module.go:38</code> | <code>app.toml [wasm] query_gas_limit -&gt; WasmConfig.SmartQueryGasLimit (ReadWasmConfig, opts.Get("wasm.query_gas_limit"))</code> |
| <code>interface.6.cfg-wasm-simulation-gas-limit</code> | <code>config_key</code> | <code>stable</code> | <code>wasm/x/wasm/module.go:39</code> | <code>app.toml [wasm] simulation_gas_limit -&gt; WasmConfig.SimulationGasLimit (ReadWasmConfig, opts.Get("wasm.simulation_gas_limit"))</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.6.cfg-wasm-contract-debug-mode-trace</code> | notes | Default defaultContractDebugMode = false. 2 in-repo callers (2 non-test, 0 test). |
| <code>interface.6.cfg-wasm-memory-cache-size</code> | notes | Default defaultMemoryCacheSize uint32 = 100 MiB (wasm/x/wasm/types/types.go:15); not present in the paxd app.toml template; AddModuleInitFlags (wasm/x/wasm/module.go:243) has no caller, so paxd exposes no --wasm.memory_cache_size flag. 2 in-repo callers (2 non-test, 0 test). |
| <code>interface.6.cfg-wasm-query-gas-limit</code> | notes | Default defaultSmartQueryGasLimit uint64 = 3_000_000 (wasm/x/wasm/types/types.go:16); the paxd template writes query_gas_limit = 300000 (daemon/paxd/cmd/root.go:439) and docker/localnode/config/app.toml:204 the same, below the code default; AddModuleInitFlags (wasm/x/wasm/module.go:243) has no caller, so paxd exposes no --wasm.query_gas_limit flag. 5 in-repo callers (5 non-test, 0 test). |
| <code>interface.6.cfg-wasm-simulation-gas-limit</code> | notes | Default unset (nil) = consensus max block gas (wasm/x/wasm/types/types.go:306); not present in the paxd app.toml template; AddModuleInitFlags (wasm/x/wasm/module.go:243) has no caller, so paxd exposes no --wasm.simulation_gas_limit flag. 2 in-repo callers (2 non-test, 0 test). |

## custom-messages

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.5.wasm-pax-custom-encoder</code> | <code>wasm_binding</code> | <code>stable</code> | <code>node/wasmbinding/encoder.go:24</code> | <code>func CustomEncoder(sender sdk.AccAddress, msg json.RawMessage, info wasmvmtypes.MessageInfo, codeInfo wasmtypes.CodeInfo) ([]sdk.Msg, error)</code> |
| <code>interface.5.wasm-pax-custom-message-handler</code> | <code>wasm_binding</code> | <code>stable</code> | <code>node/wasmbinding/message_plugin.go:35</code> | <code>func CustomMessageHandler(router, channelKeeper, capabilityKeeper, bankKeeper, unpacker, portSource, customEncoders) wasmkeeper.Messenger: chain NewSDKMessageHandler, NewIBCRawPacketHandler, NewBurnCoinMessageHandler</code> |
| <code>interface.5.wasm-pax-custom-router</code> | <code>wasm_binding</code> | <code>stable</code> | <code>node/wasmbinding/message_plugin.go:13</code> | <code>CustomRouter.Handler(msg sdk.Msg): *evmtypes.MsgInternalEVMCall and *evmtypes.MsgInternalEVMDelegateCall -&gt; evm keeper msg server; others -&gt; app MsgServiceRouter</code> |
| <code>interface.5.wasm-pax-msg-burn-tokens</code> | <code>wasm_binding</code> | <code>stable</code> | <code>node/wasmbinding/encoder.go:17</code> | <code>CosmosMsg.custom {"burn_tokens": bindings.BurnTokens{amount}} -&gt; EncodeTokenFactoryBurn</code> |
| <code>interface.5.wasm-pax-msg-call-evm</code> | <code>wasm_binding</code> | <code>stable</code> | <code>node/wasmbinding/encoder.go:20</code> | <code>CosmosMsg.custom {"call_evm": bindings.CallEVM{value, to, data}} -&gt; EncodeCallEVM</code> |
| <code>interface.5.wasm-pax-msg-change-admin</code> | <code>wasm_binding</code> | <code>stable</code> | <code>node/wasmbinding/encoder.go:18</code> | <code>CosmosMsg.custom {"change_admin": bindings.ChangeAdmin{denom, new_admin_address}} -&gt; EncodeTokenFactoryChangeAdmin</code> |
| <code>interface.5.wasm-pax-msg-create-denom</code> | <code>wasm_binding</code> | <code>stable</code> | <code>node/wasmbinding/encoder.go:15</code> | <code>CosmosMsg.custom {"create_denom": bindings.CreateDenom{subdenom}} -&gt; EncodeTokenFactoryCreateDenom</code> |
| <code>interface.5.wasm-pax-msg-delegate-call-evm</code> | <code>wasm_binding</code> | <code>stable</code> | <code>node/wasmbinding/encoder.go:21</code> | <code>CosmosMsg.custom {"delegate_call_evm": bindings.DelegateCallEVM{to, data}} -&gt; EncodeDelegateCallEVM</code> |
| <code>interface.5.wasm-pax-msg-mint-tokens</code> | <code>wasm_binding</code> | <code>stable</code> | <code>node/wasmbinding/encoder.go:16</code> | <code>CosmosMsg.custom {"mint_tokens": bindings.MintTokens{amount}} -&gt; EncodeTokenFactoryMint</code> |
| <code>interface.5.wasm-pax-msg-set-metadata</code> | <code>wasm_binding</code> | <code>stable</code> | <code>node/wasmbinding/encoder.go:19</code> | <code>CosmosMsg.custom {"set_metadata": bindings.SetMetadata{metadata}} -&gt; EncodeTokenFactorySetMetadata</code> |
| <code>interface.5.wasm-pax-register-custom-plugins</code> | <code>wasm_binding</code> | <code>stable</code> | <code>node/wasmbinding/wasm.go:20</code> | <code>func RegisterCustomPlugins(</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.5.wasm-pax-custom-encoder</code> | notes | Unknown key returns "Unknown Pax Wasm Message" wrapped in ErrParsingPaxWasmMsg (node/wasmbinding/encoder.go:45); 1 in-repo callers (1 non-test, 0 test). |
| <code>interface.5.wasm-pax-custom-message-handler</code> | notes | 1 in-repo callers (1 non-test, 0 test). |
| <code>interface.5.wasm-pax-custom-router</code> | notes | Routes the sdk.Msg produced by call_evm and delegate_call_evm; 4 in-repo callers (3 non-test, 1 test). |
| <code>interface.5.wasm-pax-msg-burn-tokens</code> | notes | Payload type at node/wasmbinding/bindings/msg.go:33; encoded by modules/tokenfactory/client/wasm/encoder.go; 3 in-repo callers (2 non-test, 1 test). |
| <code>interface.5.wasm-pax-msg-call-evm</code> | notes | Payload type at node/wasmbinding/bindings/msg.go:37; encoded by modules/evm/client/wasm/encoder.go; 2 in-repo callers (2 non-test, 0 test). |
| <code>interface.5.wasm-pax-msg-change-admin</code> | notes | Payload type at node/wasmbinding/bindings/msg.go:20; encoded by modules/tokenfactory/client/wasm/encoder.go; 3 in-repo callers (2 non-test, 1 test). |
| <code>interface.5.wasm-pax-msg-create-denom</code> | notes | Payload type at node/wasmbinding/bindings/msg.go:14; encoded by modules/tokenfactory/client/wasm/encoder.go; 3 in-repo callers (2 non-test, 1 test). |
| <code>interface.5.wasm-pax-msg-delegate-call-evm</code> | notes | Payload type at node/wasmbinding/bindings/msg.go:43; encoded by modules/evm/client/wasm/encoder.go; 15 in-repo callers (2 non-test, 13 test). |
| <code>interface.5.wasm-pax-msg-mint-tokens</code> | notes | Payload type at node/wasmbinding/bindings/msg.go:29; encoded by modules/tokenfactory/client/wasm/encoder.go; 3 in-repo callers (2 non-test, 1 test). |
| <code>interface.5.wasm-pax-msg-set-metadata</code> | notes | Payload type at node/wasmbinding/bindings/msg.go:25; encoded by modules/tokenfactory/client/wasm/encoder.go; 2 in-repo callers (2 non-test, 0 test). |
| <code>interface.5.wasm-pax-register-custom-plugins</code> | notes | Installs WithQueryPlugins(Custom: CustomQuerier) and WithMessageHandlerDecorator(CustomMessageHandler) with Custom: CustomEncoder; 1 in-repo callers (1 non-test, 0 test). |

## custom-queries

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.3.wasm-pax-epoch-epoch</code> | <code>wasm_binding</code> | <code>stable</code> | <code>modules/epoch/client/wasm/bindings/queries.go:7</code> | <code>route "epoch": PaxEpochQuery.Epoch *types.QueryEpochRequest `json:"epoch"` -&gt; GetEpoch</code> |
| <code>interface.3.wasm-pax-oracle-exchange-rates</code> | <code>wasm_binding</code> | <code>stable</code> | <code>modules/oracle/client/wasm/bindings/queries.go:7</code> | <code>route "oracle": PaxOracleQuery.ExchangeRates *types.QueryExchangeRatesRequest `json:"exchange_rates"` -&gt; GetExchangeRates</code> |
| <code>interface.3.wasm-pax-oracle-oracle-twaps</code> | <code>wasm_binding</code> | <code>stable</code> | <code>modules/oracle/client/wasm/bindings/queries.go:9</code> | <code>route "oracle": PaxOracleQuery.OracleTwaps *types.QueryTwapsRequest `json:"oracle_twaps"` -&gt; GetOracleTwaps</code> |
| <code>interface.5.wasm-pax-custom-querier</code> | <code>wasm_binding</code> | <code>stable</code> | <code>node/wasmbinding/query_plugin.go:26</code> | <code>func CustomQuerier(qp *QueryPlugin) func(ctx sdk.Context, request json.RawMessage) ([]byte, error)</code> |
| <code>interface.5.wasm-pax-query-wrapper</code> | <code>wasm_binding</code> | <code>stable</code> | <code>node/wasmbinding/query_plugin.go:19</code> | <code>type PaxQueryWrapper struct { Route string `json:"route,omitempty"`; QueryData json.RawMessage `json:"query_data,omitempty"` }</code> |
| <code>interface.5.wasm-pax-route-epoch</code> | <code>wasm_binding</code> | <code>stable</code> | <code>node/wasmbinding/query_plugin.go:13</code> | <code>EpochRoute        = "epoch" (custom query route; request {"route":"epoch","query_data":{...}})</code> |
| <code>interface.5.wasm-pax-route-evm</code> | <code>wasm_binding</code> | <code>stable</code> | <code>node/wasmbinding/query_plugin.go:15</code> | <code>EVMRoute          = "evm" (custom query route; request {"route":"evm","query_data":{...}})</code> |
| <code>interface.5.wasm-pax-route-oracle</code> | <code>wasm_binding</code> | <code>stable</code> | <code>node/wasmbinding/query_plugin.go:12</code> | <code>OracleRoute       = "oracle" (custom query route; request {"route":"oracle","query_data":{...}})</code> |
| <code>interface.5.wasm-pax-route-stakingext</code> | <code>wasm_binding</code> | <code>stable</code> | <code>node/wasmbinding/query_plugin.go:16</code> | <code>StakingExtRoute   = "stakingext" (custom query route; request {"route":"stakingext","query_data":{...}})</code> |
| <code>interface.5.wasm-pax-route-tokenfactory</code> | <code>wasm_binding</code> | <code>stable</code> | <code>node/wasmbinding/query_plugin.go:14</code> | <code>TokenFactoryRoute = "tokenfactory" (custom query route; request {"route":"tokenfactory","query_data":{...}})</code> |
| <code>interface.5.wasm-pax-stakingext-unbonding-delegations</code> | <code>wasm_binding</code> | <code>change</code> | <code>node/wasmbinding/queries.go:251</code> | <code>route "stakingext": StakingExtQuery.UnbondingDelegations *UnbondingDelegationsRequest{delegator} `json:"unbonding_delegations"` -&gt; UnbondingDelegationsResponse{entries[{creation_height, completion_time, initial_balance, balance}]}</code> |
| <code>interface.6.wasm-pax-evm-erc1155-balance-of</code> | <code>wasm_binding</code> | <code>stable</code> | <code>modules/evm/client/wasm/bindings/queries.go:166</code> | <code>route "evm": PaxEVMQuery.ERC1155BalanceOf *ERC1155BalanceOfRequest `json:"erc1155_balance_of"`</code> |
| <code>interface.6.wasm-pax-evm-erc1155-balance-of-batch</code> | <code>wasm_binding</code> | <code>stable</code> | <code>modules/evm/client/wasm/bindings/queries.go:167</code> | <code>route "evm": PaxEVMQuery.ERC1155BalanceOfBatch *ERC1155BalanceOfBatchRequest `json:"erc1155_balance_of_batch"`</code> |
| <code>interface.6.wasm-pax-evm-erc1155-batch-transfer-payload</code> | <code>wasm_binding</code> | <code>stable</code> | <code>modules/evm/client/wasm/bindings/queries.go:163</code> | <code>route "evm": PaxEVMQuery.ERC1155BatchTransferPayload *ERC1155BatchTransferPayloadRequest `json:"erc1155_batch_transfer_payload"`</code> |
| <code>interface.6.wasm-pax-evm-erc1155-is-approved-for-all</code> | <code>wasm_binding</code> | <code>stable</code> | <code>modules/evm/client/wasm/bindings/queries.go:165</code> | <code>route "evm": PaxEVMQuery.ERC1155IsApprovedForAll *ERC1155IsApprovedForAllRequest `json:"erc1155_is_approved_for_all"`</code> |
| <code>interface.6.wasm-pax-evm-erc1155-name-symbol</code> | <code>wasm_binding</code> | <code>stable</code> | <code>modules/evm/client/wasm/bindings/queries.go:171</code> | <code>route "evm": PaxEVMQuery.ERC1155NameSymbol *ERC1155NameSymbolRequest `json:"erc1155_name_symbol"`</code> |
| <code>interface.6.wasm-pax-evm-erc1155-royalty-info</code> | <code>wasm_binding</code> | <code>stable</code> | <code>modules/evm/client/wasm/bindings/queries.go:173</code> | <code>route "evm": PaxEVMQuery.ERC1155RoyaltyInfo *ERC1155RoyaltyInfoRequest `json:"erc1155_royalty_info"`</code> |
| <code>interface.6.wasm-pax-evm-erc1155-set-approval-all-payload</code> | <code>wasm_binding</code> | <code>stable</code> | <code>modules/evm/client/wasm/bindings/queries.go:164</code> | <code>route "evm": PaxEVMQuery.ERC1155SetApprovalAllPayload *ERC1155SetApprovalAllPayloadRequest `json:"erc1155_set_approval_all_payload"`</code> |
| <code>interface.6.wasm-pax-evm-erc1155-token-exists</code> | <code>wasm_binding</code> | <code>stable</code> | <code>modules/evm/client/wasm/bindings/queries.go:170</code> | <code>route "evm": PaxEVMQuery.ERC1155TokenExists *ERC1155TokenExistsRequest `json:"erc1155_token_exists"`</code> |
| <code>interface.6.wasm-pax-evm-erc1155-total-supply</code> | <code>wasm_binding</code> | <code>stable</code> | <code>modules/evm/client/wasm/bindings/queries.go:168</code> | <code>route "evm": PaxEVMQuery.ERC1155TotalSupply *ERC1155TotalSupplyRequest `json:"erc1155_total_supply"`</code> |
| <code>interface.6.wasm-pax-evm-erc1155-total-supply-for-token</code> | <code>wasm_binding</code> | <code>stable</code> | <code>modules/evm/client/wasm/bindings/queries.go:169</code> | <code>route "evm": PaxEVMQuery.ERC1155TotalSupplyForToken *ERC1155TotalSupplyForTokenRequest `json:"erc1155_total_supply_for_token"`</code> |
| <code>interface.6.wasm-pax-evm-erc1155-transfer-payload</code> | <code>wasm_binding</code> | <code>stable</code> | <code>modules/evm/client/wasm/bindings/queries.go:162</code> | <code>route "evm": PaxEVMQuery.ERC1155TransferPayload *ERC1155TransferPayloadRequest `json:"erc1155_transfer_payload"`</code> |
| <code>interface.6.wasm-pax-evm-erc1155-uri</code> | <code>wasm_binding</code> | <code>stable</code> | <code>modules/evm/client/wasm/bindings/queries.go:172</code> | <code>route "evm": PaxEVMQuery.ERC1155Uri *ERC1155UriRequest `json:"erc1155_uri"`</code> |
| <code>interface.6.wasm-pax-evm-erc20-allowance</code> | <code>wasm_binding</code> | <code>stable</code> | <code>modules/evm/client/wasm/bindings/queries.go:149</code> | <code>route "evm": PaxEVMQuery.ERC20Allowance *ERC20AllowanceRequest `json:"erc20_allowance"`</code> |
| <code>interface.6.wasm-pax-evm-erc20-approve-payload</code> | <code>wasm_binding</code> | <code>stable</code> | <code>modules/evm/client/wasm/bindings/queries.go:148</code> | <code>route "evm": PaxEVMQuery.ERC20ApprovePayload *ERC20ApprovePayloadRequest `json:"erc20_approve_payload"`</code> |
| <code>interface.6.wasm-pax-evm-erc20-balance</code> | <code>wasm_binding</code> | <code>stable</code> | <code>modules/evm/client/wasm/bindings/queries.go:151</code> | <code>route "evm": PaxEVMQuery.ERC20Balance *ERC20BalanceRequest `json:"erc20_balance"`</code> |
| <code>interface.6.wasm-pax-evm-erc20-token-info</code> | <code>wasm_binding</code> | <code>stable</code> | <code>modules/evm/client/wasm/bindings/queries.go:150</code> | <code>route "evm": PaxEVMQuery.ERC20TokenInfo *ERC20TokenInfoRequest `json:"erc20_token_info"`</code> |
| <code>interface.6.wasm-pax-evm-erc20-transfer-from-payload</code> | <code>wasm_binding</code> | <code>stable</code> | <code>modules/evm/client/wasm/bindings/queries.go:147</code> | <code>route "evm": PaxEVMQuery.ERC20TransferFromPayload *ERC20TransferFromPayloadRequest `json:"erc20_transfer_from_payload"`</code> |
| <code>interface.6.wasm-pax-evm-erc20-transfer-payload</code> | <code>wasm_binding</code> | <code>stable</code> | <code>modules/evm/client/wasm/bindings/queries.go:146</code> | <code>route "evm": PaxEVMQuery.ERC20TransferPayload *ERC20TransferPayloadRequest `json:"erc20_transfer_payload"`</code> |
| <code>interface.6.wasm-pax-evm-erc721-approve-payload</code> | <code>wasm_binding</code> | <code>stable</code> | <code>modules/evm/client/wasm/bindings/queries.go:154</code> | <code>route "evm": PaxEVMQuery.ERC721ApprovePayload *ERC721ApprovePayloadRequest `json:"erc721_approve_payload"`</code> |
| <code>interface.6.wasm-pax-evm-erc721-approved</code> | <code>wasm_binding</code> | <code>stable</code> | <code>modules/evm/client/wasm/bindings/queries.go:156</code> | <code>route "evm": PaxEVMQuery.ERC721Approved *ERC721ApprovedRequest `json:"erc721_approved"`</code> |
| <code>interface.6.wasm-pax-evm-erc721-is-approved-for-all</code> | <code>wasm_binding</code> | <code>stable</code> | <code>modules/evm/client/wasm/bindings/queries.go:157</code> | <code>route "evm": PaxEVMQuery.ERC721IsApprovedForAll *ERC721IsApprovedForAllRequest `json:"erc721_is_approved_for_all"`</code> |
| <code>interface.6.wasm-pax-evm-erc721-name-symbol</code> | <code>wasm_binding</code> | <code>stable</code> | <code>modules/evm/client/wasm/bindings/queries.go:159</code> | <code>route "evm": PaxEVMQuery.ERC721NameSymbol *ERC721NameSymbolRequest `json:"erc721_name_symbol"`</code> |
| <code>interface.6.wasm-pax-evm-erc721-owner</code> | <code>wasm_binding</code> | <code>stable</code> | <code>modules/evm/client/wasm/bindings/queries.go:152</code> | <code>route "evm": PaxEVMQuery.ERC721Owner *ERC721OwnerRequest `json:"erc721_owner"`</code> |
| <code>interface.6.wasm-pax-evm-erc721-royalty-info</code> | <code>wasm_binding</code> | <code>stable</code> | <code>modules/evm/client/wasm/bindings/queries.go:161</code> | <code>route "evm": PaxEVMQuery.ERC721RoyaltyInfo *ERC721RoyaltyInfoRequest `json:"erc721_royalty_info"`</code> |
| <code>interface.6.wasm-pax-evm-erc721-set-approval-all-payload</code> | <code>wasm_binding</code> | <code>stable</code> | <code>modules/evm/client/wasm/bindings/queries.go:155</code> | <code>route "evm": PaxEVMQuery.ERC721SetApprovalAllPayload *ERC721SetApprovalAllPayloadRequest `json:"erc721_set_approval_all_payload"`</code> |
| <code>interface.6.wasm-pax-evm-erc721-total-supply</code> | <code>wasm_binding</code> | <code>stable</code> | <code>modules/evm/client/wasm/bindings/queries.go:158</code> | <code>route "evm": PaxEVMQuery.ERC721TotalSupply *ERC721TotalSupplyRequest `json:"erc721_total_supply"`</code> |
| <code>interface.6.wasm-pax-evm-erc721-transfer-payload</code> | <code>wasm_binding</code> | <code>stable</code> | <code>modules/evm/client/wasm/bindings/queries.go:153</code> | <code>route "evm": PaxEVMQuery.ERC721TransferPayload *ERC721TransferPayloadRequest `json:"erc721_transfer_payload"`</code> |
| <code>interface.6.wasm-pax-evm-erc721-uri</code> | <code>wasm_binding</code> | <code>stable</code> | <code>modules/evm/client/wasm/bindings/queries.go:160</code> | <code>route "evm": PaxEVMQuery.ERC721Uri *ERC721UriRequest `json:"erc721_uri"`</code> |
| <code>interface.6.wasm-pax-evm-get-evm-address</code> | <code>wasm_binding</code> | <code>stable</code> | <code>modules/evm/client/wasm/bindings/queries.go:174</code> | <code>route "evm": PaxEVMQuery.GetEvmAddress *GetEvmAddressRequest `json:"get_evm_address"`</code> |
| <code>interface.6.wasm-pax-evm-get-pax-address</code> | <code>wasm_binding</code> | <code>stable</code> | <code>modules/evm/client/wasm/bindings/queries.go:175</code> | <code>route "evm": PaxEVMQuery.GetPaxAddress *GetPaxAddressRequest `json:"get_pax_address"`</code> |
| <code>interface.6.wasm-pax-evm-static-call</code> | <code>wasm_binding</code> | <code>stable</code> | <code>modules/evm/client/wasm/bindings/queries.go:145</code> | <code>route "evm": PaxEVMQuery.StaticCall *StaticCallRequest `json:"static_call"`</code> |
| <code>interface.6.wasm-pax-evm-supports-interface</code> | <code>wasm_binding</code> | <code>stable</code> | <code>modules/evm/client/wasm/bindings/queries.go:176</code> | <code>route "evm": PaxEVMQuery.SupportsInterface *SupportsInterfaceRequest `json:"supports_interface"`</code> |
| <code>interface.6.wasm-pax-tokenfactory-denom-authority-metadata</code> | <code>wasm_binding</code> | <code>stable</code> | <code>modules/tokenfactory/client/wasm/bindings/queries.go:7</code> | <code>route "tokenfactory": PaxTokenFactoryQuery.DenomAuthorityMetadata *types.QueryDenomAuthorityMetadataRequest `json:"denom_authority_metadata"` -&gt; GetDenomAuthorityMetadata</code> |
| <code>interface.6.wasm-pax-tokenfactory-denoms-from-creator</code> | <code>wasm_binding</code> | <code>stable</code> | <code>modules/tokenfactory/client/wasm/bindings/queries.go:9</code> | <code>route "tokenfactory": PaxTokenFactoryQuery.DenomsFromCreator *types.QueryDenomsFromCreatorRequest `json:"denoms_from_creator"` -&gt; GetDenomsFromCreator</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.3.wasm-pax-epoch-epoch</code> | notes | Served by node/wasmbinding/queries.go:77 through modules/epoch/client/wasm/query.go:19; no fixture contract sends it; 3 in-repo callers (2 non-test, 1 test). |
| <code>interface.3.wasm-pax-oracle-exchange-rates</code> | notes | Served by node/wasmbinding/queries.go:44 through modules/oracle/client/wasm/query.go:19; no fixture contract sends it; 3 in-repo callers (2 non-test, 1 test). |
| <code>interface.3.wasm-pax-oracle-oracle-twaps</code> | notes | Served by node/wasmbinding/queries.go:44 through modules/oracle/client/wasm/query.go:25; no fixture contract sends it; 6 in-repo callers (2 non-test, 4 test). |
| <code>interface.5.wasm-pax-custom-querier</code> | notes | Unknown route returns wasmvmtypes.UnsupportedRequest{Kind: "Unknown Pax Query Route"} (node/wasmbinding/query_plugin.go:44); 2 in-repo callers (1 non-test, 1 test). |
| <code>interface.5.wasm-pax-query-wrapper</code> | notes | Envelope of every custom query (QueryRequest.custom); fixture contracts mirror it as EvmQueryWrapper {route, query_data}; 13 in-repo callers (1 non-test, 12 test). |
| <code>interface.5.wasm-pax-route-epoch</code> | notes | CustomQuerier dispatches the route to its handler; 3 in-repo callers (1 non-test, 2 test). |
| <code>interface.5.wasm-pax-route-evm</code> | notes | CustomQuerier dispatches the route to its handler; 4 in-repo callers (1 non-test, 3 test). |
| <code>interface.5.wasm-pax-route-oracle</code> | notes | CustomQuerier dispatches the route to its handler; 6 in-repo callers (1 non-test, 5 test). |
| <code>interface.5.wasm-pax-route-stakingext</code> | notes | CustomQuerier dispatches the route to its handler; 1 in-repo callers (1 non-test, 0 test). |
| <code>interface.5.wasm-pax-route-tokenfactory</code> | notes | CustomQuerier dispatches the route to its handler; 3 in-repo callers (1 non-test, 2 test). |
| <code>interface.5.wasm-pax-stakingext-unbonding-delegations</code> | notes | Mismatch: response field initial_balance (node/wasmbinding/queries.go:272) is filled from entry.Balance at node/wasmbinding/queries.go:300, not the entry's InitialBalance, so it equals balance after partial slashing; query type staking_ext_unbonding_delegations (node/wasmbinding/queries.go:247); 1 in-repo callers (1 non-test, 0 test). |
| <code>interface.6.wasm-pax-evm-erc1155-balance-of</code> | notes | Query type evm_query_erc1155_balance_of (modules/evm/client/wasm/bindings/queries.go:31); request fields ['caller', 'contract_address', 'account', 'token_id']; handler HandleERC1155BalanceOf; 5 in-repo callers (2 non-test, 3 test). |
| <code>interface.6.wasm-pax-evm-erc1155-balance-of-batch</code> | notes | Query type evm_query_erc1155_balance_of_batch (modules/evm/client/wasm/bindings/queries.go:32); request fields ['caller', 'contract_address', 'accounts', 'token_ids']; handler HandleERC1155BalanceOfBatch; 5 in-repo callers (2 non-test, 3 test). |
| <code>interface.6.wasm-pax-evm-erc1155-batch-transfer-payload</code> | notes | Query type evm_query_erc1155_batch_transfer (modules/evm/client/wasm/bindings/queries.go:28); request fields ['from', 'recipient', 'token_ids', 'amounts']; handler HandleERC1155BatchTransferPayload; 8 in-repo callers (2 non-test, 6 test). |
| <code>interface.6.wasm-pax-evm-erc1155-is-approved-for-all</code> | notes | Query type evm_query_erc1155_is_approved_for_all (modules/evm/client/wasm/bindings/queries.go:30); request fields ['caller', 'contract_address', 'owner', 'operator']; handler HandleERC1155IsApprovedForAll; 7 in-repo callers (2 non-test, 5 test). |
| <code>interface.6.wasm-pax-evm-erc1155-name-symbol</code> | notes | Query type evm_query_erc1155_name_symbol (modules/evm/client/wasm/bindings/queries.go:36); request fields ['caller', 'contract_address']; handler HandleERC1155NameSymbol; 5 in-repo callers (2 non-test, 3 test). |
| <code>interface.6.wasm-pax-evm-erc1155-royalty-info</code> | notes | Query type evm_query_erc1155_royalty_info (modules/evm/client/wasm/bindings/queries.go:38); request fields ['caller', 'contract_address', 'token_id', 'sale_price']; handler HandleERC1155RoyaltyInfo; 5 in-repo callers (2 non-test, 3 test). |
| <code>interface.6.wasm-pax-evm-erc1155-set-approval-all-payload</code> | notes | Query type evm_query_erc1155_set_approval_all (modules/evm/client/wasm/bindings/queries.go:29); request fields ['to', 'approved']; handler HandleERC1155SetApprovalAllPayload; 6 in-repo callers (2 non-test, 4 test). |
| <code>interface.6.wasm-pax-evm-erc1155-token-exists</code> | notes | Query type evm_query_erc1155_token_exists (modules/evm/client/wasm/bindings/queries.go:35); request fields ['caller', 'contract_address', 'token_id']; handler HandleERC1155TokenExists; 4 in-repo callers (2 non-test, 2 test). |
| <code>interface.6.wasm-pax-evm-erc1155-total-supply</code> | notes | Query type evm_query_erc1155_total_supply (modules/evm/client/wasm/bindings/queries.go:33); request fields ['caller', 'contract_address']; handler HandleERC1155TotalSupply; 5 in-repo callers (2 non-test, 3 test). |
| <code>interface.6.wasm-pax-evm-erc1155-total-supply-for-token</code> | notes | Query type evm_query_erc1155_total_supply_for_token (modules/evm/client/wasm/bindings/queries.go:34); request fields ['caller', 'contract_address', 'token_id']; handler HandleERC1155TotalSupplyForToken; 5 in-repo callers (2 non-test, 3 test). |
| <code>interface.6.wasm-pax-evm-erc1155-transfer-payload</code> | notes | Query type evm_query_erc1155_transfer (modules/evm/client/wasm/bindings/queries.go:27); request fields ['from', 'recipient', 'token_id', 'amount']; handler HandleERC1155TransferPayload; 7 in-repo callers (2 non-test, 5 test). |
| <code>interface.6.wasm-pax-evm-erc1155-uri</code> | notes | Query type evm_query_erc1155_uri (modules/evm/client/wasm/bindings/queries.go:37); request fields ['caller', 'contract_address', 'token_id']; handler HandleERC1155Uri; 5 in-repo callers (2 non-test, 3 test). |
| <code>interface.6.wasm-pax-evm-erc20-allowance</code> | notes | Query type evm_query_erc20_allowance (modules/evm/client/wasm/bindings/queries.go:14); request fields ['contract_address', 'owner', 'spender']; handler HandleERC20Allowance; 6 in-repo callers (2 non-test, 4 test). |
| <code>interface.6.wasm-pax-evm-erc20-approve-payload</code> | notes | Query type evm_query_erc20_approve (modules/evm/client/wasm/bindings/queries.go:13); request fields ['spender', 'amount']; handler HandleERC20ApprovePayload; 6 in-repo callers (2 non-test, 4 test). |
| <code>interface.6.wasm-pax-evm-erc20-balance</code> | notes | Query type evm_query_erc20_balance (modules/evm/client/wasm/bindings/queries.go:16); request fields ['contract_address', 'account']; handler HandleERC20Balance; 5 in-repo callers (2 non-test, 3 test). |
| <code>interface.6.wasm-pax-evm-erc20-token-info</code> | notes | Query type evm_query_erc20_token_info (modules/evm/client/wasm/bindings/queries.go:15); request fields ['contract_address', 'caller']; handler HandleERC20TokenInfo; 5 in-repo callers (2 non-test, 3 test). |
| <code>interface.6.wasm-pax-evm-erc20-transfer-from-payload</code> | notes | Query type evm_query_erc20_transfer_from (modules/evm/client/wasm/bindings/queries.go:12); request fields ['owner', 'recipient', 'amount']; handler HandleERC20TransferFromPayload; 7 in-repo callers (2 non-test, 5 test). |
| <code>interface.6.wasm-pax-evm-erc20-transfer-payload</code> | notes | Query type evm_query_erc20_transfer (modules/evm/client/wasm/bindings/queries.go:11); request fields ['recipient', 'amount']; handler HandleERC20TransferPayload; 7 in-repo callers (2 non-test, 5 test). |
| <code>interface.6.wasm-pax-evm-erc721-approve-payload</code> | notes | Query type evm_query_erc721_approve (modules/evm/client/wasm/bindings/queries.go:19); request fields ['spender', 'token_id']; handler HandleERC721ApprovePayload; 6 in-repo callers (2 non-test, 4 test). |
| <code>interface.6.wasm-pax-evm-erc721-approved</code> | notes | Query type evm_query_erc721_approved (modules/evm/client/wasm/bindings/queries.go:21); request fields ['caller', 'contract_address', 'token_id']; handler HandleERC721Approved; 5 in-repo callers (2 non-test, 3 test). |
| <code>interface.6.wasm-pax-evm-erc721-is-approved-for-all</code> | notes | Query type evm_query_erc721_is_approved_for_all (modules/evm/client/wasm/bindings/queries.go:22); request fields ['caller', 'contract_address', 'owner', 'operator']; handler HandleERC721IsApprovedForAll; 7 in-repo callers (2 non-test, 5 test). |
| <code>interface.6.wasm-pax-evm-erc721-name-symbol</code> | notes | Query type evm_query_erc721_name_symbol (modules/evm/client/wasm/bindings/queries.go:24); request fields ['caller', 'contract_address']; handler HandleERC721NameSymbol; 5 in-repo callers (2 non-test, 3 test). |
| <code>interface.6.wasm-pax-evm-erc721-owner</code> | notes | Query type evm_query_erc721_owner (modules/evm/client/wasm/bindings/queries.go:17); request fields ['caller', 'contract_address', 'token_id']; handler HandleERC721Owner; 5 in-repo callers (2 non-test, 3 test). |
| <code>interface.6.wasm-pax-evm-erc721-royalty-info</code> | notes | Query type evm_query_erc721_royalty_info (modules/evm/client/wasm/bindings/queries.go:26); request fields ['caller', 'contract_address', 'token_id', 'sale_price']; handler HandleERC721RoyaltyInfo; 5 in-repo callers (2 non-test, 3 test). |
| <code>interface.6.wasm-pax-evm-erc721-set-approval-all-payload</code> | notes | Query type evm_query_erc721_set_approval_all (modules/evm/client/wasm/bindings/queries.go:20); request fields ['to', 'approved']; handler HandleERC721SetApprovalAllPayload; 6 in-repo callers (2 non-test, 4 test). |
| <code>interface.6.wasm-pax-evm-erc721-total-supply</code> | notes | Query type evm_query_erc721_total_supply (modules/evm/client/wasm/bindings/queries.go:23); request fields ['caller', 'contract_address']; handler HandleERC721TotalSupply; 5 in-repo callers (2 non-test, 3 test). |
| <code>interface.6.wasm-pax-evm-erc721-transfer-payload</code> | notes | Query type evm_query_erc721_transfer (modules/evm/client/wasm/bindings/queries.go:18); request fields ['from', 'recipient', 'token_id']; handler HandleERC721TransferPayload; 8 in-repo callers (2 non-test, 6 test). |
| <code>interface.6.wasm-pax-evm-erc721-uri</code> | notes | Query type evm_query_erc721_uri (modules/evm/client/wasm/bindings/queries.go:25); request fields ['caller', 'contract_address', 'token_id']; handler HandleERC721Uri; 5 in-repo callers (2 non-test, 3 test). |
| <code>interface.6.wasm-pax-evm-get-evm-address</code> | notes | Query type evm_query_get_evm_address (modules/evm/client/wasm/bindings/queries.go:39); request fields ['pax_address']; handler HandleGetEvmAddress; 4 in-repo callers (2 non-test, 2 test). |
| <code>interface.6.wasm-pax-evm-get-pax-address</code> | notes | Query type evm_query_get_pax_address (modules/evm/client/wasm/bindings/queries.go:40); request fields ['evm_address']; handler HandleGetPaxAddress; 4 in-repo callers (2 non-test, 2 test). |
| <code>interface.6.wasm-pax-evm-static-call</code> | notes | Query type evm_query_static_call (modules/evm/client/wasm/bindings/queries.go:10); request fields ['from', 'to', 'data']; handler HandleStaticCall; 3 in-repo callers (2 non-test, 1 test). |
| <code>interface.6.wasm-pax-evm-supports-interface</code> | notes | Query type evm_query_supports_interface (modules/evm/client/wasm/bindings/queries.go:41); request fields ['caller', 'contract_address', 'interface_id']; handler HandleSupportsInterface; 6 in-repo callers (2 non-test, 4 test). |
| <code>interface.6.wasm-pax-tokenfactory-denom-authority-metadata</code> | notes | Served by node/wasmbinding/queries.go:99 through modules/tokenfactory/client/wasm/query.go:19; no fixture contract sends it; 3 in-repo callers (2 non-test, 1 test). |
| <code>interface.6.wasm-pax-tokenfactory-denoms-from-creator</code> | notes | Served by node/wasmbinding/queries.go:99 through modules/tokenfactory/client/wasm/query.go:24; no fixture contract sends it; 3 in-repo callers (2 non-test, 1 test). |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>resolution.17.656</code> | title | decision.506 binds: The wasm capability list is kept |
| <code>resolution.17.656</code> | applies_to | decision.506<br>task.5.13.1 |
| <code>resolution.17.656</code> | source | decision.506 in fragments/J31/decision.kvx; tasks from affects |

## env

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.6.wasm-env-transaction-index</code> | <code>wasm_binding</code> | <code>stable</code> | <code>wasm/x/wasm/types/types.go:276</code> | <code>Env.transaction = &amp;TransactionInfo{Index: txCounter} only when TXCounter(ctx) is present; otherwise env.transaction is null</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.6.wasm-env-transaction-index</code> | notes | Mismatch: stale decision 57 requires every deliver path to set the counter to the block position from the merge plan height; at HEAD no path sets it (node/ante.go:35 keeps TXCounterStoreKey but no CountTXDecorator, wasm/x/wasm/keeper/ante.go:20 is unused by the node, and node/sequencer/position_test.go:180 asserts the counter is absent); JSON shape wasm-runtime/types/env.go:12. 12 in-repo callers (12 non-test, 0 test). |
| <code>interface.6.wasm-env-transaction-index</code> | overridden_by | override.J37.1 |

Contract context <code>contract_context.6.wasm-x-j97</code>:

| Field | Frozen rule |
|---|---|
| interface | 1 surfaces under wasm/x (1 wasm_binding; 1 change) frozen at the anchors contracts/cosmwasm.md#env: Env.transaction = &amp;TransactionInfo{Index: txCounter} only when TXCounter(ctx) is present;  (wasm/x/wasm/types/types.go:276) |
| storage | no storage_key item in this context; the wasm/x items read or write state only through the keepers and stores frozen by the other domain-6 contexts |
| errors | no error_code item in this context; failures surface through the error registries frozen by the other domain-6 contexts |
| events | no event, precompile_event or sol_event item is named by this context; events of the handlers behind these items are frozen by the other domain-6 contexts |
| gas | not metered: no item in this context is a precompile method |
| activation | no task names these items yet |
| invariants | each item keeps the kind, path and signature of its interface block once its landing task passes; a later change of any item needs a new decision of domain 6 |
| pause | no pause parameter governs these items; they are disabled only by reverting the touches of their landing tasks |

## errors

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.5.err-wasmbinding-2-parsing-pax-wasm-msg</code> | <code>error_code</code> | <code>stable</code> | <code>node/wasmbinding/bindings/errors.go:11</code> | <code>ErrParsingPaxWasmMsg = Register("wasmbinding", 2, "Error parsing Pax Wasm Message")</code> |
| <code>interface.6.err-wasm-10-invalid-msg</code> | <code>error_code</code> | <code>stable</code> | <code>wasm/x/wasm/types/errors.go:38</code> | <code>ErrInvalidMsg = Register("wasm", 10, "invalid CosmosMsg from the contract")</code> |
| <code>interface.6.err-wasm-11-migration-failed</code> | <code>error_code</code> | <code>stable</code> | <code>wasm/x/wasm/types/errors.go:41</code> | <code>ErrMigrationFailed = Register("wasm", 11, "migrate wasm contract failed")</code> |
| <code>interface.6.err-wasm-12-empty</code> | <code>error_code</code> | <code>stable</code> | <code>wasm/x/wasm/types/errors.go:44</code> | <code>ErrEmpty = Register("wasm", 12, "empty")</code> |
| <code>interface.6.err-wasm-13-limit</code> | <code>error_code</code> | <code>stable</code> | <code>wasm/x/wasm/types/errors.go:47</code> | <code>ErrLimit = Register("wasm", 13, "exceeds limit")</code> |
| <code>interface.6.err-wasm-14-invalid</code> | <code>error_code</code> | <code>stable</code> | <code>wasm/x/wasm/types/errors.go:50</code> | <code>ErrInvalid = Register("wasm", 14, "invalid")</code> |
| <code>interface.6.err-wasm-15-duplicate</code> | <code>error_code</code> | <code>stable</code> | <code>wasm/x/wasm/types/errors.go:53</code> | <code>ErrDuplicate = Register("wasm", 15, "duplicate")</code> |
| <code>interface.6.err-wasm-16-max-i-b-c-channels</code> | <code>error_code</code> | <code>stable</code> | <code>wasm/x/wasm/types/errors.go:56</code> | <code>ErrMaxIBCChannels = Register("wasm", 16, "max transfer channels")</code> |
| <code>interface.6.err-wasm-17-unsupported-for-contract</code> | <code>error_code</code> | <code>stable</code> | <code>wasm/x/wasm/types/errors.go:59</code> | <code>ErrUnsupportedForContract = Register("wasm", 17, "unsupported for this contract")</code> |
| <code>interface.6.err-wasm-18-pin-contract-failed</code> | <code>error_code</code> | <code>stable</code> | <code>wasm/x/wasm/types/errors.go:62</code> | <code>ErrPinContractFailed = Register("wasm", 18, "pinning contract failed")</code> |
| <code>interface.6.err-wasm-19-unpin-contract-failed</code> | <code>error_code</code> | <code>stable</code> | <code>wasm/x/wasm/types/errors.go:65</code> | <code>ErrUnpinContractFailed = Register("wasm", 19, "unpinning contract failed")</code> |
| <code>interface.6.err-wasm-2-create-failed</code> | <code>error_code</code> | <code>stable</code> | <code>wasm/x/wasm/types/errors.go:14</code> | <code>ErrCreateFailed = Register("wasm", 2, "create wasm contract failed")</code> |
| <code>interface.6.err-wasm-20-unknown-msg</code> | <code>error_code</code> | <code>stable</code> | <code>wasm/x/wasm/types/errors.go:68</code> | <code>ErrUnknownMsg = Register("wasm", 20, "unknown message from the contract")</code> |
| <code>interface.6.err-wasm-21-invalid-event</code> | <code>error_code</code> | <code>stable</code> | <code>wasm/x/wasm/types/errors.go:71</code> | <code>ErrInvalidEvent = Register("wasm", 21, "invalid event")</code> |
| <code>interface.6.err-wasm-22-no-such-contract</code> | <code>error_code</code> | <code>stable</code> | <code>wasm/x/wasm/types/errors.go:74</code> | <code>_ = Register("wasm", 22, "no such contract")</code> |
| <code>interface.6.err-wasm-23-not-a-j-s-o-n-object</code> | <code>error_code</code> | <code>stable</code> | <code>wasm/x/wasm/types/errors.go:77</code> | <code>ErrNotAJSONObject = Register("wasm", 23, "not a JSON object")</code> |
| <code>interface.6.err-wasm-24-no-top-level-key</code> | <code>error_code</code> | <code>stable</code> | <code>wasm/x/wasm/types/errors.go:80</code> | <code>ErrNoTopLevelKey = Register("wasm", 24, "no top-level key")</code> |
| <code>interface.6.err-wasm-25-multiple-top-level-keys</code> | <code>error_code</code> | <code>stable</code> | <code>wasm/x/wasm/types/errors.go:83</code> | <code>ErrMultipleTopLevelKeys = Register("wasm", 25, "multiple top-level keys")</code> |
| <code>interface.6.err-wasm-26-top-kevel-key-not-allowed</code> | <code>error_code</code> | <code>stable</code> | <code>wasm/x/wasm/types/errors.go:86</code> | <code>ErrTopKevelKeyNotAllowed = Register("wasm", 26, "top-level key is not allowed")</code> |
| <code>interface.6.err-wasm-27-exceed-max-query-stack-size</code> | <code>error_code</code> | <code>stable</code> | <code>wasm/x/wasm/types/errors.go:89</code> | <code>ErrExceedMaxQueryStackSize = Register("wasm", 27, "max query stack size exceeded")</code> |
| <code>interface.6.err-wasm-3-account-exists</code> | <code>error_code</code> | <code>stable</code> | <code>wasm/x/wasm/types/errors.go:17</code> | <code>ErrAccountExists = Register("wasm", 3, "contract account already exists")</code> |
| <code>interface.6.err-wasm-30-exceed-max-call-depth</code> | <code>error_code</code> | <code>stable</code> | <code>wasm/x/wasm/types/errors.go:94</code> | <code>ErrExceedMaxCallDepth = Register("wasm", 30, "max call depth exceeded")</code> |
| <code>interface.6.err-wasm-4-instantiate-failed</code> | <code>error_code</code> | <code>stable</code> | <code>wasm/x/wasm/types/errors.go:20</code> | <code>ErrInstantiateFailed = Register("wasm", 4, "instantiate wasm contract failed")</code> |
| <code>interface.6.err-wasm-5-execute-failed</code> | <code>error_code</code> | <code>stable</code> | <code>wasm/x/wasm/types/errors.go:23</code> | <code>ErrExecuteFailed = Register("wasm", 5, "execute wasm contract failed")</code> |
| <code>interface.6.err-wasm-6-gas-limit</code> | <code>error_code</code> | <code>stable</code> | <code>wasm/x/wasm/types/errors.go:26</code> | <code>ErrGasLimit = Register("wasm", 6, "insufficient gas")</code> |
| <code>interface.6.err-wasm-7-invalid-genesis</code> | <code>error_code</code> | <code>stable</code> | <code>wasm/x/wasm/types/errors.go:29</code> | <code>ErrInvalidGenesis = Register("wasm", 7, "invalid genesis")</code> |
| <code>interface.6.err-wasm-8-not-found</code> | <code>error_code</code> | <code>stable</code> | <code>wasm/x/wasm/types/errors.go:32</code> | <code>ErrNotFound = Register("wasm", 8, "not found")</code> |
| <code>interface.6.err-wasm-9-query-failed</code> | <code>error_code</code> | <code>stable</code> | <code>wasm/x/wasm/types/errors.go:35</code> | <code>ErrQueryFailed = Register("wasm", 9, "query wasm contract failed")</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.5.err-wasmbinding-2-parsing-pax-wasm-msg</code> | notes | Registered but never referenced in the repository. Codespace wasmbinding for the custom message encoder package. 0 in-repo callers (0 non-test, 0 test). |
| <code>interface.6.err-wasm-10-invalid-msg</code> | notes | 8 in-repo callers (3 non-test, 5 test). |
| <code>interface.6.err-wasm-11-migration-failed</code> | notes | 6 in-repo callers (3 non-test, 3 test). |
| <code>interface.6.err-wasm-12-empty</code> | notes | 15 in-repo callers (14 non-test, 1 test). |
| <code>interface.6.err-wasm-13-limit</code> | notes | 7 in-repo callers (4 non-test, 3 test). |
| <code>interface.6.err-wasm-14-invalid</code> | notes | 19 in-repo callers (16 non-test, 3 test). |
| <code>interface.6.err-wasm-15-duplicate</code> | notes | 5 in-repo callers (5 non-test, 0 test). |
| <code>interface.6.err-wasm-16-max-i-b-c-channels</code> | notes | 1 in-repo callers (1 non-test, 0 test). |
| <code>interface.6.err-wasm-17-unsupported-for-contract</code> | notes | 1 in-repo callers (1 non-test, 0 test). |
| <code>interface.6.err-wasm-18-pin-contract-failed</code> | notes | 2 in-repo callers (2 non-test, 0 test). |
| <code>interface.6.err-wasm-19-unpin-contract-failed</code> | notes | 3 in-repo callers (1 non-test, 2 test). |
| <code>interface.6.err-wasm-2-create-failed</code> | notes | 8 in-repo callers (8 non-test, 0 test). |
| <code>interface.6.err-wasm-20-unknown-msg</code> | notes | 13 in-repo callers (11 non-test, 2 test). |
| <code>interface.6.err-wasm-21-invalid-event</code> | notes | 4 in-repo callers (4 non-test, 0 test). |
| <code>interface.6.err-wasm-22-no-such-contract</code> | notes | Registered without a variable; raised through the ErrNoSuchContract type (wasm/x/wasm/types/errors.go:97) whose ABCICode returns 22 and Codespace returns wasm. 8 in-repo callers (6 non-test, 2 test). |
| <code>interface.6.err-wasm-23-not-a-j-s-o-n-object</code> | notes | 4 in-repo callers (1 non-test, 3 test). |
| <code>interface.6.err-wasm-24-no-top-level-key</code> | notes | 2 in-repo callers (1 non-test, 1 test). |
| <code>interface.6.err-wasm-25-multiple-top-level-keys</code> | notes | 2 in-repo callers (1 non-test, 1 test). |
| <code>interface.6.err-wasm-26-top-kevel-key-not-allowed</code> | notes | 3 in-repo callers (1 non-test, 2 test). |
| <code>interface.6.err-wasm-27-exceed-max-query-stack-size</code> | notes | 1 in-repo callers (1 non-test, 0 test). |
| <code>interface.6.err-wasm-3-account-exists</code> | notes | 1 in-repo callers (1 non-test, 0 test). |
| <code>interface.6.err-wasm-30-exceed-max-call-depth</code> | notes | Codes 28 and 29 are unused (comment at wasm/x/wasm/types/errors.go:91). 1 in-repo callers (1 non-test, 0 test). |
| <code>interface.6.err-wasm-4-instantiate-failed</code> | notes | 2 in-repo callers (2 non-test, 0 test). |
| <code>interface.6.err-wasm-5-execute-failed</code> | notes | 12 in-repo callers (10 non-test, 2 test). |
| <code>interface.6.err-wasm-6-gas-limit</code> | notes | Registered but never referenced in the repository. 0 in-repo callers (0 non-test, 0 test). |
| <code>interface.6.err-wasm-7-invalid-genesis</code> | notes | Registered but never referenced in the repository. 0 in-repo callers (0 non-test, 0 test). |
| <code>interface.6.err-wasm-8-not-found</code> | notes | 21 in-repo callers (15 non-test, 6 test). |
| <code>interface.6.err-wasm-9-query-failed</code> | notes | 3 in-repo callers (1 non-test, 2 test). |

## events

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.6.evt-wasm-cw721-pretransfer-owner-j37</code> | <code>event</code> | <code>stable</code> | <code>wasm/x/wasm/types/events.go:18</code> | <code>EventTypeCW721PreTransferOwner = "cw721_pretransfer_owner"</code> |
| <code>interface.6.evt-wasm-execute-j37</code> | <code>event</code> | <code>stable</code> | <code>wasm/x/wasm/types/events.go:11</code> | <code>EventTypeExecute = "execute"</code> |
| <code>interface.6.evt-wasm-gov-contract-result-j37</code> | <code>event</code> | <code>stable</code> | <code>wasm/x/wasm/types/events.go:17</code> | <code>EventTypeGovContractResult = "gov_contract_result"</code> |
| <code>interface.6.evt-wasm-instantiate-j37</code> | <code>event</code> | <code>stable</code> | <code>wasm/x/wasm/types/events.go:10</code> | <code>EventTypeInstantiate = "instantiate"</code> |
| <code>interface.6.evt-wasm-migrate-j37</code> | <code>event</code> | <code>stable</code> | <code>wasm/x/wasm/types/events.go:12</code> | <code>EventTypeMigrate = "migrate"</code> |
| <code>interface.6.evt-wasm-pin-code-j37</code> | <code>event</code> | <code>stable</code> | <code>wasm/x/wasm/types/events.go:13</code> | <code>EventTypePinCode = "pin_code"</code> |
| <code>interface.6.evt-wasm-reply-j37</code> | <code>event</code> | <code>stable</code> | <code>wasm/x/wasm/types/events.go:16</code> | <code>EventTypeReply = "reply"</code> |
| <code>interface.6.evt-wasm-store-code-j37</code> | <code>event</code> | <code>stable</code> | <code>wasm/x/wasm/types/events.go:9</code> | <code>EventTypeStoreCode = "store_code"</code> |
| <code>interface.6.evt-wasm-sudo-j37</code> | <code>event</code> | <code>stable</code> | <code>wasm/x/wasm/types/events.go:15</code> | <code>EventTypeSudo = "sudo"</code> |
| <code>interface.6.evt-wasm-unpin-code-j37</code> | <code>event</code> | <code>stable</code> | <code>wasm/x/wasm/types/events.go:14</code> | <code>EventTypeUnpinCode = "unpin_code"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.6.evt-wasm-cw721-pretransfer-owner-j37</code> | notes | Event type emitted by wasm/x/wasm/types; 5 in-repo reference(s) outside the definition, first 5 listed. |
| <code>interface.6.evt-wasm-execute-j37</code> | notes | Event type emitted by wasm/x/wasm/types; 1 in-repo reference(s) outside the definition, first 1 listed. |
| <code>interface.6.evt-wasm-gov-contract-result-j37</code> | notes | Event type emitted by wasm/x/wasm/types; 7 in-repo reference(s) outside the definition, first 7 listed. |
| <code>interface.6.evt-wasm-instantiate-j37</code> | notes | Event type emitted by wasm/x/wasm/types; 3 in-repo reference(s) outside the definition, first 3 listed. |
| <code>interface.6.evt-wasm-migrate-j37</code> | notes | Event type emitted by wasm/x/wasm/types; 2 in-repo reference(s) outside the definition, first 2 listed. |
| <code>interface.6.evt-wasm-pin-code-j37</code> | notes | Event type emitted by wasm/x/wasm/types; 1 in-repo reference(s) outside the definition, first 1 listed. |
| <code>interface.6.evt-wasm-reply-j37</code> | notes | Event type emitted by wasm/x/wasm/types; 2 in-repo reference(s) outside the definition, first 2 listed. |
| <code>interface.6.evt-wasm-store-code-j37</code> | notes | Event type emitted by wasm/x/wasm/types; 1 in-repo reference(s) outside the definition, first 1 listed. |
| <code>interface.6.evt-wasm-sudo-j37</code> | notes | Event type emitted by wasm/x/wasm/types; 1 in-repo reference(s) outside the definition, first 1 listed. |
| <code>interface.6.evt-wasm-unpin-code-j37</code> | notes | Event type emitted by wasm/x/wasm/types; 1 in-repo reference(s) outside the definition, first 1 listed. |

## genesis

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.6.gen-wasm-codes</code> | <code>genesis_param</code> | <code>stable</code> | <code>wasm/proto/cosmwasm/wasm/v1/genesis.proto:13</code> | <code>GenesisState.codes = 2</code> |
| <code>interface.6.gen-wasm-contracts</code> | <code>genesis_param</code> | <code>stable</code> | <code>wasm/proto/cosmwasm/wasm/v1/genesis.proto:17</code> | <code>GenesisState.contracts = 3</code> |
| <code>interface.6.gen-wasm-gen-msgs</code> | <code>genesis_param</code> | <code>stable</code> | <code>wasm/proto/cosmwasm/wasm/v1/genesis.proto:25</code> | <code>GenesisState.gen_msgs = 5</code> |
| <code>interface.6.gen-wasm-params-code-upload-access</code> | <code>genesis_param</code> | <code>stable</code> | <code>wasm/proto/cosmwasm/wasm/v1/types.proto:42</code> | <code>Params.code_upload_access AccessConfig{permission, address} = 1 (param key uploadAccess)</code> |
| <code>interface.6.gen-wasm-params-instantiate-default-permission</code> | <code>genesis_param</code> | <code>stable</code> | <code>wasm/proto/cosmwasm/wasm/v1/types.proto:46</code> | <code>Params.instantiate_default_permission AccessType = 2 (param key instantiateAccess)</code> |
| <code>interface.6.gen-wasm-sequences</code> | <code>genesis_param</code> | <code>stable</code> | <code>wasm/proto/cosmwasm/wasm/v1/genesis.proto:21</code> | <code>GenesisState.sequences = 4</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.6.gen-wasm-codes</code> | notes | Chain genesis files carry an empty list. 6 in-repo callers (6 non-test, 0 test). |
| <code>interface.6.gen-wasm-contracts</code> | notes | Chain genesis files carry an empty list. 7 in-repo callers (7 non-test, 0 test). |
| <code>interface.6.gen-wasm-gen-msgs</code> | notes | Chain genesis files carry an empty list. 5 in-repo callers (5 non-test, 0 test). |
| <code>interface.6.gen-wasm-params-code-upload-access</code> | notes | Default AllowEverybody; the three chain genesis files set {permission Everybody, address ""}. 5 in-repo callers (5 non-test, 0 test). |
| <code>interface.6.gen-wasm-params-instantiate-default-permission</code> | notes | Default AccessTypeEverybody; chain genesis files set Everybody. 4 in-repo callers (4 non-test, 0 test). |
| <code>interface.6.gen-wasm-sequences</code> | notes | Chain genesis files carry an empty list. 6 in-repo callers (6 non-test, 0 test). |

## go-api

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.6.go-staticarchive-verify</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm/staticarchive/archive.go:239</code> | <code>func Verify(directory, name string) (string, error)</code> |
| <code>interface.6.go-wasm-appmodulebasic</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm/x/wasm/module.go:43</code> | <code>type AppModuleBasic struct{}</code> |
| <code>interface.6.go-wasm-client-cli-genesisexecutecontractcmd</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm/x/wasm/client/cli/genesis_msg.go:153</code> | <code>func GenesisExecuteContractCmd(defaultNodeHome string, genesisMutator GenesisMutator) *cobra.Command</code> |
| <code>interface.6.go-wasm-client-cli-genesisinstantiatecontractcmd</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm/x/wasm/client/cli/genesis_msg.go:85</code> | <code>func GenesisInstantiateContractCmd(defaultNodeHome string, genesisMutator GenesisMutator) *cobra.Command</code> |
| <code>interface.6.go-wasm-client-cli-genesislistcodescmd</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm/x/wasm/client/cli/genesis_msg.go:203</code> | <code>func GenesisListCodesCmd(defaultNodeHome string, genReader GenesisReader) *cobra.Command</code> |
| <code>interface.6.go-wasm-client-cli-genesislistcontractscmd</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm/x/wasm/client/cli/genesis_msg.go:227</code> | <code>func GenesisListContractsCmd(defaultNodeHome string, genReader GenesisReader) *cobra.Command</code> |
| <code>interface.6.go-wasm-client-cli-genesisstorecodecmd</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm/x/wasm/client/cli/genesis_msg.go:45</code> | <code>func GenesisStoreCodeCmd(defaultNodeHome string, genesisMutator GenesisMutator) *cobra.Command</code> |
| <code>interface.6.go-wasm-client-cli-newdefaultgenesisio</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm/x/wasm/client/cli/genesis_msg.go:419</code> | <code>func NewDefaultGenesisIO() *DefaultGenesisIO</code> |
| <code>interface.6.go-wasm-client-proposalhandlers</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm/x/wasm/client/proposal_handler.go:11</code> | <code>var ProposalHandlers = []govclient.ProposalHandler{</code> |
| <code>interface.6.go-wasm-converttoproposals</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm/x/wasm/alias.go:33</code> | <code>ConvertToProposals = types.ConvertToProposals</code> |
| <code>interface.6.go-wasm-disableallproposals</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm/x/wasm/alias.go:47</code> | <code>DisableAllProposals = types.DisableAllProposals</code> |
| <code>interface.6.go-wasm-enableallproposals</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm/x/wasm/alias.go:46</code> | <code>EnableAllProposals  = types.EnableAllProposals</code> |
| <code>interface.6.go-wasm-keeper</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm/x/wasm/alias.go:80</code> | <code>Keeper                         = keeper.Keeper</code> |
| <code>interface.6.go-wasm-modulename</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm/x/wasm/alias.go:15</code> | <code>ModuleName                      = types.ModuleName</code> |
| <code>interface.6.go-wasm-msginstantiatecontract</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm/x/wasm/alias.go:56</code> | <code>MsgInstantiateContract         = types.MsgInstantiateContract</code> |
| <code>interface.6.go-wasm-newappmodule</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm/x/wasm/module.go:143</code> | <code>func NewAppModule( cdc codec.Codec, keeper *Keeper, validatorSetSource keeper.ValidatorSetSource, ak types.AccountKeeper, bk simKeeper.BankKeeper) AppModule</code> |
| <code>interface.6.go-wasm-newibchandler</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm/x/wasm/ibc.go:26</code> | <code>func NewIBCHandler(k types.IBCContractKeeper, ck types.ChannelKeeper) IBCHandler</code> |
| <code>interface.6.go-wasm-newkeeper</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm/x/wasm/alias.go:40</code> | <code>NewKeeper              = keeper.NewKeeper</code> |
| <code>interface.6.go-wasm-newwasmproposalhandler</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm/x/wasm/alias.go:42</code> | <code>NewWasmProposalHandler = keeper.NewWasmProposalHandler</code> |
| <code>interface.6.go-wasm-option</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm/x/wasm/alias.go:84</code> | <code>Option                         = keeper.Option</code> |
| <code>interface.6.go-wasm-proposaltype</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm/x/wasm/alias.go:51</code> | <code>ProposalType                   = types.ProposalType</code> |
| <code>interface.6.go-wasm-readwasmconfig</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm/x/wasm/module.go:251</code> | <code>func ReadWasmConfig(opts servertypes.AppOptions) (types.WasmConfig, error)</code> |
| <code>interface.6.go-wasm-routerkey</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm/x/wasm/alias.go:18</code> | <code>RouterKey                       = types.RouterKey</code> |
| <code>interface.6.go-wasm-storekey</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm/x/wasm/alias.go:16</code> | <code>StoreKey                        = types.StoreKey</code> |
| <code>interface.6.go-wasmapp-makeencodingconfig</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm/app/encoding.go:10</code> | <code>func MakeEncodingConfig() params.EncodingConfig</code> |
| <code>interface.6.go-wasmapp-params-makeencodingconfig</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm/app/params/proto.go:10</code> | <code>func MakeEncodingConfig() EncodingConfig</code> |
| <code>interface.6.go-wasmkeeper-defaultencoders</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/handler_plugin_encoders.go:43</code> | <code>func DefaultEncoders(unpacker codectypes.AnyUnpacker, portSource types.ICS20TransferPortSource) MessageEncoders</code> |
| <code>interface.6.go-wasmkeeper-defaultgasmetersetter</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/ante.go:96</code> | <code>func DefaultGasMeterSetter() func(bool, sdk.Context, uint64, sdk.Tx) sdk.Context</code> |
| <code>interface.6.go-wasmkeeper-defaultgasregisterconfig</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/gas_register.go:100</code> | <code>func DefaultGasRegisterConfig() WasmGasRegisterConfig</code> |
| <code>interface.6.go-wasmkeeper-keeper</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/keeper.go:76</code> | <code>type Keeper struct</code> |
| <code>interface.6.go-wasmkeeper-messageencoders</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/handler_plugin_encoders.go:32</code> | <code>type MessageEncoders struct</code> |
| <code>interface.6.go-wasmkeeper-messagerouter</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/handler_plugin.go:25</code> | <code>type MessageRouter interface</code> |
| <code>interface.6.go-wasmkeeper-messenger</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/msg_dispatcher.go:15</code> | <code>type Messenger interface</code> |
| <code>interface.6.go-wasmkeeper-newburncoinmessagehandler</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/handler_plugin.go:205</code> | <code>func NewBurnCoinMessageHandler(burner types.Burner) MessageHandlerFunc</code> |
| <code>interface.6.go-wasmkeeper-newdefaultpermissionkeeper</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/contract_keeper.go:38</code> | <code>func NewDefaultPermissionKeeper(nested decoratedKeeper) *PermissionedKeeper</code> |
| <code>interface.6.go-wasmkeeper-newibcrawpackethandler</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/handler_plugin.go:149</code> | <code>func NewIBCRawPacketHandler(chk types.ChannelKeeper, cak types.CapabilityKeeper) IBCRawPacketHandler</code> |
| <code>interface.6.go-wasmkeeper-newlimitsimulationgasdecorator</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/ante.go:89</code> | <code>func NewLimitSimulationGasDecorator(gasLimit *sdk.Gas, gasMeterSetter func(bool, sdk.Context, uint64, sdk.Tx) sdk.Context) *LimitSimulationGasDecorator</code> |
| <code>interface.6.go-wasmkeeper-newmessagehandlerchain</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/handler_plugin.go:114</code> | <code>func NewMessageHandlerChain(first Messenger, others ...Messenger) *MessageHandlerChain</code> |
| <code>interface.6.go-wasmkeeper-newsdkmessagehandler</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/handler_plugin.go:55</code> | <code>func NewSDKMessageHandler(router MessageRouter, encoders msgEncoder) SDKMessageHandler</code> |
| <code>interface.6.go-wasmkeeper-newwasmgasregister</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/gas_register.go:124</code> | <code>func NewWasmGasRegister(c WasmGasRegisterConfig) WasmGasRegister</code> |
| <code>interface.6.go-wasmkeeper-newwasmsnapshotter</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/snapshotter.go:35</code> | <code>func NewWasmSnapshotter(cms sdk.MultiStore, wasm *Keeper) *WasmSnapshotter</code> |
| <code>interface.6.go-wasmkeeper-option</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/keeper.go:46</code> | <code>type Option interface</code> |
| <code>interface.6.go-wasmkeeper-permissionedkeeper</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/contract_keeper.go:25</code> | <code>type PermissionedKeeper struct</code> |
| <code>interface.6.go-wasmkeeper-queryplugins</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/query_plugins.go:79</code> | <code>type QueryPlugins struct</code> |
| <code>interface.6.go-wasmkeeper-withgasregister</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/options.go:106</code> | <code>func WithGasRegister(x GasRegister) Option</code> |
| <code>interface.6.go-wasmkeeper-withmessagehandler</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/options.go:27</code> | <code>func WithMessageHandler(x Messenger) Option</code> |
| <code>interface.6.go-wasmkeeper-withqueryplugins</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/options.go:59</code> | <code>func WithQueryPlugins(x *QueryPlugins) Option</code> |
| <code>interface.6.go-wasmtypes-accessconfig</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm/x/wasm/types/types.pb.go:136</code> | <code>type AccessConfig struct</code> |
| <code>interface.6.go-wasmtypes-attributekeycontractaddr</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm/x/wasm/types/events.go:25</code> | <code>AttributeKeyContractAddr  = "_contract_address"</code> |
| <code>interface.6.go-wasmtypes-attributekeyowner</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm/x/wasm/types/events.go:30</code> | <code>AttributeKeyOwner         = "owner"</code> |
| <code>interface.6.go-wasmtypes-attributekeytokenid</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm/x/wasm/types/events.go:29</code> | <code>AttributeKeyTokenId       = "token_id"</code> |
| <code>interface.6.go-wasmtypes-burner</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm/x/wasm/types/expected_keepers.go:24</code> | <code>type Burner interface</code> |
| <code>interface.6.go-wasmtypes-capabilitykeeper</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm/x/wasm/types/expected_keepers.go:99</code> | <code>type CapabilityKeeper interface</code> |
| <code>interface.6.go-wasmtypes-channelkeeper</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm/x/wasm/types/expected_keepers.go:74</code> | <code>type ChannelKeeper interface</code> |
| <code>interface.6.go-wasmtypes-codeinfo</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm/x/wasm/types/types.pb.go:213</code> | <code>type CodeInfo struct</code> |
| <code>interface.6.go-wasmtypes-defaultwasmconfig</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm/x/wasm/types/types.go:316</code> | <code>func DefaultWasmConfig() WasmConfig</code> |
| <code>interface.6.go-wasmtypes-eventtypecw721pretransferowner</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm/x/wasm/types/events.go:18</code> | <code>EventTypeCW721PreTransferOwner = "cw721_pretransfer_owner"</code> |
| <code>interface.6.go-wasmtypes-ics20transferportsource</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm/x/wasm/types/expected_keepers.go:114</code> | <code>type ICS20TransferPortSource interface</code> |
| <code>interface.6.go-wasmtypes-maxwasmsize</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm/x/wasm/types/validation.go:12</code> | <code>MaxWasmSize = 800 * 1024 // extension point for chains to customize via compile flag.</code> |
| <code>interface.6.go-wasmtypes-msgexecutecontract</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm/x/wasm/types/tx.pb.go:208</code> | <code>type MsgExecuteContract struct</code> |
| <code>interface.6.go-wasmtypes-msginstantiatecontract</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm/x/wasm/types/tx.pb.go:118</code> | <code>type MsgInstantiateContract struct</code> |
| <code>interface.6.go-wasmtypes-rawcontractmessage</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm/x/wasm/types/tx.go:15</code> | <code>type RawContractMessage []byte</code> |
| <code>interface.6.go-wasmtypes-txcounter</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm/x/wasm/types/ante.go:21</code> | <code>func TXCounter(ctx sdk.Context) (uint32, bool)</code> |
| <code>interface.6.go-wasmtypes-wasmconfig</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm/x/wasm/types/types.go:303</code> | <code>type WasmConfig struct</code> |
| <code>interface.6.go-wasmtypes-wasmmoduleeventtype</code> | <code>go_exported</code> | <code>stable</code> | <code>wasm/x/wasm/types/events.go:5</code> | <code>WasmModuleEventType = "wasm"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.6.go-staticarchive-verify</code> | notes | Only test callers; no production path verifies the static archive. Exported func used outside its tree; 2 in-repo callers (0 non-test, 2 test). |
| <code>interface.6.go-wasm-appmodulebasic</code> | notes | Exported type used outside its tree; 1 in-repo callers (1 non-test, 0 test). |
| <code>interface.6.go-wasm-client-cli-genesisexecutecontractcmd</code> | notes | Exported func used outside its tree; 1 in-repo callers (1 non-test, 0 test). |
| <code>interface.6.go-wasm-client-cli-genesisinstantiatecontractcmd</code> | notes | Exported func used outside its tree; 1 in-repo callers (1 non-test, 0 test). |
| <code>interface.6.go-wasm-client-cli-genesislistcodescmd</code> | notes | Exported func used outside its tree; 1 in-repo callers (1 non-test, 0 test). |
| <code>interface.6.go-wasm-client-cli-genesislistcontractscmd</code> | notes | Exported func used outside its tree; 1 in-repo callers (1 non-test, 0 test). |
| <code>interface.6.go-wasm-client-cli-genesisstorecodecmd</code> | notes | Exported func used outside its tree; 1 in-repo callers (1 non-test, 0 test). |
| <code>interface.6.go-wasm-client-cli-newdefaultgenesisio</code> | notes | Exported func used outside its tree; 1 in-repo callers (1 non-test, 0 test). |
| <code>interface.6.go-wasm-client-proposalhandlers</code> | notes | Exported var used outside its tree; 1 in-repo callers (1 non-test, 0 test). |
| <code>interface.6.go-wasm-converttoproposals</code> | notes | Exported var used outside its tree; 1 in-repo callers (1 non-test, 0 test). |
| <code>interface.6.go-wasm-disableallproposals</code> | notes | Exported var used outside its tree; 1 in-repo callers (1 non-test, 0 test). |
| <code>interface.6.go-wasm-enableallproposals</code> | notes | Exported var used outside its tree; 17 in-repo callers (11 non-test, 6 test). |
| <code>interface.6.go-wasm-keeper</code> | notes | Exported type used outside its tree; 2 in-repo callers (2 non-test, 0 test). |
| <code>interface.6.go-wasm-modulename</code> | notes | Exported const used outside its tree; 8 in-repo callers (6 non-test, 2 test). |
| <code>interface.6.go-wasm-msginstantiatecontract</code> | notes | Exported type used outside its tree; 1 in-repo callers (0 non-test, 1 test). |
| <code>interface.6.go-wasm-newappmodule</code> | notes | Exported func used outside its tree; 2 in-repo callers (2 non-test, 0 test). |
| <code>interface.6.go-wasm-newibchandler</code> | notes | Exported func used outside its tree; 1 in-repo callers (1 non-test, 0 test). |
| <code>interface.6.go-wasm-newkeeper</code> | notes | Exported var used outside its tree; 1 in-repo callers (1 non-test, 0 test). |
| <code>interface.6.go-wasm-newwasmproposalhandler</code> | notes | Exported var used outside its tree; 1 in-repo callers (1 non-test, 0 test). |
| <code>interface.6.go-wasm-option</code> | notes | Exported type used outside its tree; 9 in-repo callers (9 non-test, 0 test). |
| <code>interface.6.go-wasm-proposaltype</code> | notes | Exported type used outside its tree; 2 in-repo callers (2 non-test, 0 test). |
| <code>interface.6.go-wasm-readwasmconfig</code> | notes | Exported func used outside its tree; 1 in-repo callers (1 non-test, 0 test). |
| <code>interface.6.go-wasm-routerkey</code> | notes | Exported const used outside its tree; 1 in-repo callers (1 non-test, 0 test). |
| <code>interface.6.go-wasm-storekey</code> | notes | Exported const used outside its tree; 4 in-repo callers (4 non-test, 0 test). |
| <code>interface.6.go-wasmapp-makeencodingconfig</code> | notes | Exported func used outside its tree; 6 in-repo callers (0 non-test, 6 test). |
| <code>interface.6.go-wasmapp-params-makeencodingconfig</code> | notes | Exported func used outside its tree; 1 in-repo callers (0 non-test, 1 test). |
| <code>interface.6.go-wasmkeeper-defaultencoders</code> | notes | Exported func used outside its tree; 1 in-repo callers (1 non-test, 0 test). |
| <code>interface.6.go-wasmkeeper-defaultgasmetersetter</code> | notes | Exported func used outside its tree; 2 in-repo callers (1 non-test, 1 test). |
| <code>interface.6.go-wasmkeeper-defaultgasregisterconfig</code> | notes | Exported func used outside its tree; 5 in-repo callers (5 non-test, 0 test). |
| <code>interface.6.go-wasmkeeper-keeper</code> | notes | Exported type used outside its tree; 6 in-repo callers (4 non-test, 2 test). |
| <code>interface.6.go-wasmkeeper-messageencoders</code> | notes | Exported type used outside its tree; 1 in-repo callers (1 non-test, 0 test). |
| <code>interface.6.go-wasmkeeper-messagerouter</code> | notes | Exported type used outside its tree; 3 in-repo callers (3 non-test, 0 test). |
| <code>interface.6.go-wasmkeeper-messenger</code> | notes | Exported type used outside its tree; 1 in-repo callers (1 non-test, 0 test). |
| <code>interface.6.go-wasmkeeper-newburncoinmessagehandler</code> | notes | Exported func used outside its tree; 1 in-repo callers (1 non-test, 0 test). |
| <code>interface.6.go-wasmkeeper-newdefaultpermissionkeeper</code> | notes | Exported func used outside its tree; 18 in-repo callers (3 non-test, 15 test). |
| <code>interface.6.go-wasmkeeper-newibcrawpackethandler</code> | notes | Exported func used outside its tree; 1 in-repo callers (1 non-test, 0 test). |
| <code>interface.6.go-wasmkeeper-newlimitsimulationgasdecorator</code> | notes | Exported func used outside its tree; 4 in-repo callers (1 non-test, 3 test). |
| <code>interface.6.go-wasmkeeper-newmessagehandlerchain</code> | notes | Exported func used outside its tree; 1 in-repo callers (1 non-test, 0 test). |
| <code>interface.6.go-wasmkeeper-newsdkmessagehandler</code> | notes | Exported func used outside its tree; 1 in-repo callers (1 non-test, 0 test). |
| <code>interface.6.go-wasmkeeper-newwasmgasregister</code> | notes | Exported func used outside its tree; 5 in-repo callers (5 non-test, 0 test). |
| <code>interface.6.go-wasmkeeper-newwasmsnapshotter</code> | notes | Exported func used outside its tree; 1 in-repo callers (1 non-test, 0 test). |
| <code>interface.6.go-wasmkeeper-option</code> | notes | Exported type used outside its tree; 1 in-repo callers (1 non-test, 0 test). |
| <code>interface.6.go-wasmkeeper-permissionedkeeper</code> | notes | Exported type used outside its tree; 9 in-repo callers (6 non-test, 3 test). |
| <code>interface.6.go-wasmkeeper-queryplugins</code> | notes | Exported type used outside its tree; 3 in-repo callers (1 non-test, 2 test). |
| <code>interface.6.go-wasmkeeper-withgasregister</code> | notes | Exported func used outside its tree; 5 in-repo callers (5 non-test, 0 test). |
| <code>interface.6.go-wasmkeeper-withmessagehandler</code> | notes | Exported func used outside its tree; 1 in-repo callers (1 non-test, 0 test). |
| <code>interface.6.go-wasmkeeper-withqueryplugins</code> | notes | Exported func used outside its tree; 1 in-repo callers (1 non-test, 0 test). |
| <code>interface.6.go-wasmtypes-accessconfig</code> | notes | Generated protobuf type. Exported type used outside its tree; 3 in-repo callers (0 non-test, 3 test). |
| <code>interface.6.go-wasmtypes-attributekeycontractaddr</code> | notes | Exported const used outside its tree; 5 in-repo callers (2 non-test, 3 test). |
| <code>interface.6.go-wasmtypes-attributekeyowner</code> | notes | Exported const used outside its tree; 4 in-repo callers (2 non-test, 2 test). |
| <code>interface.6.go-wasmtypes-attributekeytokenid</code> | notes | Exported const used outside its tree; 4 in-repo callers (1 non-test, 3 test). |
| <code>interface.6.go-wasmtypes-burner</code> | notes | Exported type used outside its tree; 2 in-repo callers (2 non-test, 0 test). |
| <code>interface.6.go-wasmtypes-capabilitykeeper</code> | notes | Exported type used outside its tree; 2 in-repo callers (2 non-test, 0 test). |
| <code>interface.6.go-wasmtypes-channelkeeper</code> | notes | Exported type used outside its tree; 2 in-repo callers (2 non-test, 0 test). |
| <code>interface.6.go-wasmtypes-codeinfo</code> | notes | Generated protobuf type. Exported type used outside its tree; 3 in-repo callers (2 non-test, 1 test). |
| <code>interface.6.go-wasmtypes-defaultwasmconfig</code> | notes | Exported func used outside its tree; 1 in-repo callers (0 non-test, 1 test). |
| <code>interface.6.go-wasmtypes-eventtypecw721pretransferowner</code> | notes | Exported const used outside its tree; 4 in-repo callers (1 non-test, 3 test). |
| <code>interface.6.go-wasmtypes-ics20transferportsource</code> | notes | Exported type used outside its tree; 2 in-repo callers (2 non-test, 0 test). |
| <code>interface.6.go-wasmtypes-maxwasmsize</code> | notes | Exported var used outside its tree; 1 in-repo callers (1 non-test, 0 test). |
| <code>interface.6.go-wasmtypes-msgexecutecontract</code> | notes | Generated protobuf type. Exported type used outside its tree; 79 in-repo callers (44 non-test, 35 test), first 40 listed with non-test callers first. |
| <code>interface.6.go-wasmtypes-msginstantiatecontract</code> | notes | Generated protobuf type. Exported type used outside its tree; 18 in-repo callers (18 non-test, 0 test). |
| <code>interface.6.go-wasmtypes-rawcontractmessage</code> | notes | Exported type used outside its tree; 22 in-repo callers (22 non-test, 0 test). |
| <code>interface.6.go-wasmtypes-txcounter</code> | notes | Exported func used outside its tree; 1 in-repo callers (0 non-test, 1 test). |
| <code>interface.6.go-wasmtypes-wasmconfig</code> | notes | Exported type used outside its tree; 1 in-repo callers (1 non-test, 0 test). |
| <code>interface.6.go-wasmtypes-wasmmoduleeventtype</code> | notes | Exported const used outside its tree; 1 in-repo callers (1 non-test, 0 test). |

## messages

No interface block names this anchor; the citing criteria and rules below define its scope.

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.6.853.1</code> | req | 6853.1 |
| <code>acceptance.6.853.1</code> | scenario | Given a contract gov message carrying only vote_weighted, when EncodeGovMsg runs, then it returns an ErrUnknownMsg error and no message, without a panic |
| <code>acceptance.6.853.1</code> | test | wasm/x/wasm/keeper/handler_plugin_gov_test.go::TestEncodeGovMsgVoteWeightedIsRefused (planned) |
| <code>acceptance.6.853.1</code> | qualification | UNRUN |
| <code>req.6853</code> | ac_1 | EncodeGovMsg (wasm/x/wasm/keeper/handler_plugin_encoders.go:296) refuses a vote_weighted gov message with ErrUnknownMsg. |
| <code>acceptance.6.853.2</code> | req | 6853.2 |
| <code>acceptance.6.853.2</code> | scenario | Given a gov message with no variant set or a vote option string outside the four options, when EncodeGovMsg runs, then it returns ErrUnknownMsg, and a yes vote still encodes to one MsgVote |
| <code>acceptance.6.853.2</code> | test | wasm/x/wasm/keeper/handler_plugin_gov_test.go::TestEncodeGovMsgWithoutAVariantIsRefused (planned) |
| <code>acceptance.6.853.2</code> | qualification | UNRUN |
| <code>req.6853</code> | ac_2 | A gov message without a plain vote or with an unknown option is refused with ErrUnknownMsg, and the four plain options still encode to MsgVote. |
| <code>capability.6.72.1</code> | title | Refuse a CosmWasm gov message without a plain vote instead of panicking |
| <code>capability.6.72.1</code> | reqs | 6853.1<br>6853.2 |
| <code>resolution.17.752</code> | title | decision.663 binds: The CosmWasm stargate querier stays disabled |
| <code>resolution.17.752</code> | applies_to | decision.663<br>task.6.72.1 |
| <code>resolution.17.752</code> | source | decision.663 in fragments/J37/decision.kvx; tasks from affects |
| <code>resolution.17.753</code> | title | decision.664 binds: A CosmWasm gov message without a plain vote is refused |
| <code>resolution.17.753</code> | applies_to | decision.664<br>task.6.72.1 |
| <code>resolution.17.753</code> | source | decision.664 in fragments/J37/decision.kvx; tasks from affects |

## msgs

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.6.msg-wasm-clearadmin</code> | <code>cosmos_msg</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/msg_server.go:165</code> | <code>rpc cosmwasm.wasm.v1.Msg/ClearAdmin(MsgClearAdmin) returns (MsgClearAdminResponse)</code> |
| <code>interface.6.msg-wasm-executecontract</code> | <code>cosmos_msg</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/msg_server.go:78</code> | <code>rpc cosmwasm.wasm.v1.Msg/ExecuteContract(MsgExecuteContract) returns (MsgExecuteContractResponse)</code> |
| <code>interface.6.msg-wasm-instantiatecontract</code> | <code>cosmos_msg</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/msg_server.go:47</code> | <code>rpc cosmwasm.wasm.v1.Msg/InstantiateContract(MsgInstantiateContract) returns (MsgInstantiateContractResponse)</code> |
| <code>interface.6.msg-wasm-migratecontract</code> | <code>cosmos_msg</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/msg_server.go:110</code> | <code>rpc cosmwasm.wasm.v1.Msg/MigrateContract(MsgMigrateContract) returns (MsgMigrateContractResponse)</code> |
| <code>interface.6.msg-wasm-storecode</code> | <code>cosmos_msg</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/msg_server.go:24</code> | <code>rpc cosmwasm.wasm.v1.Msg/StoreCode(MsgStoreCode) returns (MsgStoreCodeResponse)</code> |
| <code>interface.6.msg-wasm-updateadmin</code> | <code>cosmos_msg</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/msg_server.go:137</code> | <code>rpc cosmwasm.wasm.v1.Msg/UpdateAdmin(MsgUpdateAdmin) returns (MsgUpdateAdminResponse)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.6.msg-wasm-clearadmin</code> | notes | 4 in-repo callers (4 non-test, 0 test). |
| <code>interface.6.msg-wasm-executecontract</code> | notes | 4 in-repo callers (4 non-test, 0 test). |
| <code>interface.6.msg-wasm-instantiatecontract</code> | notes | 7 in-repo callers (4 non-test, 3 test). |
| <code>interface.6.msg-wasm-migratecontract</code> | notes | 4 in-repo callers (4 non-test, 0 test). |
| <code>interface.6.msg-wasm-storecode</code> | notes | 6 in-repo callers (6 non-test, 0 test). |
| <code>interface.6.msg-wasm-updateadmin</code> | notes | 4 in-repo callers (4 non-test, 0 test). |

## precompile-wasmd

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.6.go-pc-wasmd-getversioned</code> | <code>go_exported</code> | <code>stable</code> | <code>precompiles/wasmd/setup.go:25</code> | <code>func GetVersioned(latestUpgrade string, keepers utils.Keepers) utils.VersionedPrecompiles</code> |
| <code>interface.6.go-pc-wasmd-iswasmdcall</code> | <code>go_exported</code> | <code>stable</code> | <code>precompiles/wasmd/wasmd.go:466</code> | <code>func IsWasmdCall(to *common.Address) bool</code> |
| <code>interface.6.go-pc-wasmd-legacy-v552-new-precompile</code> | <code>go_exported</code> | <code>stable</code> | <code>precompiles/wasmd/legacy/v552/wasmd.go:60</code> | <code>func NewPrecompile(keepers putils.Keepers) (*Precompile, error)</code> |
| <code>interface.6.go-pc-wasmd-legacy-v555-new-precompile</code> | <code>go_exported</code> | <code>stable</code> | <code>precompiles/wasmd/legacy/v555/wasmd.go:60</code> | <code>func NewPrecompile(keepers putils.Keepers) (*Precompile, error)</code> |
| <code>interface.6.go-pc-wasmd-legacy-v562-new-precompile</code> | <code>go_exported</code> | <code>stable</code> | <code>precompiles/wasmd/legacy/v562/wasmd.go:57</code> | <code>func NewPrecompile(keepers utils.Keepers) (*pcommon.DynamicGasPrecompile, error)</code> |
| <code>interface.6.go-pc-wasmd-legacy-v575-new-precompile</code> | <code>go_exported</code> | <code>stable</code> | <code>precompiles/wasmd/legacy/v575/wasmd.go:56</code> | <code>func NewPrecompile(keepers utils.Keepers) (*pcommon.DynamicGasPrecompile, error)</code> |
| <code>interface.6.go-pc-wasmd-legacy-v580-new-precompile</code> | <code>go_exported</code> | <code>stable</code> | <code>precompiles/wasmd/legacy/v580/wasmd.go:62</code> | <code>func NewPrecompile(keepers utils.Keepers) (*pcommon.DynamicGasPrecompile, error)</code> |
| <code>interface.6.go-pc-wasmd-legacy-v600-new-precompile</code> | <code>go_exported</code> | <code>stable</code> | <code>precompiles/wasmd/legacy/v600/wasmd.go:62</code> | <code>func NewPrecompile(keepers utils.Keepers) (*pcommon.DynamicGasPrecompile, error)</code> |
| <code>interface.6.go-pc-wasmd-legacy-v601-new-precompile</code> | <code>go_exported</code> | <code>stable</code> | <code>precompiles/wasmd/legacy/v601/wasmd.go:62</code> | <code>func NewPrecompile(keepers utils.Keepers) (*pcommon.DynamicGasPrecompile, error)</code> |
| <code>interface.6.go-pc-wasmd-legacy-v603-new-precompile</code> | <code>go_exported</code> | <code>stable</code> | <code>precompiles/wasmd/legacy/v603/wasmd.go:62</code> | <code>func NewPrecompile(keepers utils.Keepers) (*pcommon.DynamicGasPrecompile, error)</code> |
| <code>interface.6.go-pc-wasmd-legacy-v605-new-precompile</code> | <code>go_exported</code> | <code>stable</code> | <code>precompiles/wasmd/legacy/v605/wasmd.go:63</code> | <code>func NewPrecompile(keepers utils.Keepers) (*pcommon.DynamicGasPrecompile, error)</code> |
| <code>interface.6.go-pc-wasmd-legacy-v606-new-precompile</code> | <code>go_exported</code> | <code>stable</code> | <code>precompiles/wasmd/legacy/v606/wasmd.go:63</code> | <code>func NewPrecompile(keepers utils.Keepers) (*pcommon.DynamicGasPrecompile, error)</code> |
| <code>interface.6.go-pc-wasmd-legacy-v610-new-precompile</code> | <code>go_exported</code> | <code>stable</code> | <code>precompiles/wasmd/legacy/v610/wasmd.go:63</code> | <code>func NewPrecompile(keepers utils.Keepers) (*pcommon.DynamicGasPrecompile, error)</code> |
| <code>interface.6.go-pc-wasmd-legacy-v614-new-precompile</code> | <code>go_exported</code> | <code>stable</code> | <code>precompiles/wasmd/legacy/v614/wasmd.go:63</code> | <code>func NewPrecompile(keepers utils.Keepers) (*pcommon.DynamicGasPrecompile, error)</code> |
| <code>interface.6.go-pc-wasmd-legacy-v620-new-precompile</code> | <code>go_exported</code> | <code>stable</code> | <code>precompiles/wasmd/legacy/v620/wasmd.go:63</code> | <code>func NewPrecompile(keepers utils.Keepers) (*pcommon.DynamicGasPrecompile, error)</code> |
| <code>interface.6.go-pc-wasmd-legacy-v630-new-precompile</code> | <code>go_exported</code> | <code>stable</code> | <code>precompiles/wasmd/legacy/v630/wasmd.go:63</code> | <code>func NewPrecompile(keepers utils.Keepers) (*pcommon.DynamicGasPrecompile, error)</code> |
| <code>interface.6.go-pc-wasmd-legacy-v640-new-precompile</code> | <code>go_exported</code> | <code>stable</code> | <code>precompiles/wasmd/legacy/v640/wasmd.go:63</code> | <code>func NewPrecompile(keepers utils.Keepers) (*pcommon.DynamicGasPrecompile, error)</code> |
| <code>interface.6.go-pc-wasmd-legacy-v65-new-precompile</code> | <code>go_exported</code> | <code>stable</code> | <code>precompiles/wasmd/legacy/v65/wasmd.go:63</code> | <code>func NewPrecompile(keepers utils.Keepers) (*pcommon.DynamicGasPrecompile, error)</code> |
| <code>interface.6.go-pc-wasmd-newprecompile</code> | <code>go_exported</code> | <code>stable</code> | <code>precompiles/wasmd/wasmd.go:63</code> | <code>func NewPrecompile(keepers utils.Keepers) (*pcommon.DynamicGasPrecompile, error)</code> |
| <code>interface.6.go-pc-wasmd-precompilename</code> | <code>go_exported</code> | <code>stable</code> | <code>precompiles/wasmd/wasmd.go:31</code> | <code>const PrecompileName = "wasmd"</code> |
| <code>interface.6.go-pc-wasmd-wasmdaddress</code> | <code>go_exported</code> | <code>stable</code> | <code>precompiles/wasmd/wasmd.go:30</code> | <code>const WasmdAddress = "0x0000000000000000000000000000000000001002"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.6.go-pc-wasmd-getversioned</code> | notes | Exported func used outside its tree; 1 in-repo callers (1 non-test, 0 test). |
| <code>interface.6.go-pc-wasmd-iswasmdcall</code> | notes | Exported func used outside its tree; 7 in-repo callers (7 non-test, 0 test). |
| <code>interface.6.go-pc-wasmd-legacy-v552-new-precompile</code> | notes | Legacy wasmd precompile constructor for version v552, registered by GetVersioned in precompiles/wasmd/setup.go. 1 in-repo callers (1 non-test, 0 test). |
| <code>interface.6.go-pc-wasmd-legacy-v555-new-precompile</code> | notes | Legacy wasmd precompile constructor for version v555, registered by GetVersioned in precompiles/wasmd/setup.go. 1 in-repo callers (1 non-test, 0 test). |
| <code>interface.6.go-pc-wasmd-legacy-v562-new-precompile</code> | notes | Legacy wasmd precompile constructor for version v562, registered by GetVersioned in precompiles/wasmd/setup.go. 1 in-repo callers (1 non-test, 0 test). |
| <code>interface.6.go-pc-wasmd-legacy-v575-new-precompile</code> | notes | Legacy wasmd precompile constructor for version v575, registered by GetVersioned in precompiles/wasmd/setup.go. 1 in-repo callers (1 non-test, 0 test). |
| <code>interface.6.go-pc-wasmd-legacy-v580-new-precompile</code> | notes | Legacy wasmd precompile constructor for version v580, registered by GetVersioned in precompiles/wasmd/setup.go. 1 in-repo callers (1 non-test, 0 test). |
| <code>interface.6.go-pc-wasmd-legacy-v600-new-precompile</code> | notes | Legacy wasmd precompile constructor for version v600, registered by GetVersioned in precompiles/wasmd/setup.go. 1 in-repo callers (1 non-test, 0 test). |
| <code>interface.6.go-pc-wasmd-legacy-v601-new-precompile</code> | notes | Legacy wasmd precompile constructor for version v601, registered by GetVersioned in precompiles/wasmd/setup.go. 1 in-repo callers (1 non-test, 0 test). |
| <code>interface.6.go-pc-wasmd-legacy-v603-new-precompile</code> | notes | Legacy wasmd precompile constructor for version v603, registered by GetVersioned in precompiles/wasmd/setup.go. 1 in-repo callers (1 non-test, 0 test). |
| <code>interface.6.go-pc-wasmd-legacy-v605-new-precompile</code> | notes | Legacy wasmd precompile constructor for version v605, registered by GetVersioned in precompiles/wasmd/setup.go. 1 in-repo callers (1 non-test, 0 test). |
| <code>interface.6.go-pc-wasmd-legacy-v606-new-precompile</code> | notes | Legacy wasmd precompile constructor for version v606, registered by GetVersioned in precompiles/wasmd/setup.go. 1 in-repo callers (1 non-test, 0 test). |
| <code>interface.6.go-pc-wasmd-legacy-v610-new-precompile</code> | notes | Legacy wasmd precompile constructor for version v610, registered by GetVersioned in precompiles/wasmd/setup.go. 1 in-repo callers (1 non-test, 0 test). |
| <code>interface.6.go-pc-wasmd-legacy-v614-new-precompile</code> | notes | Legacy wasmd precompile constructor for version v614, registered by GetVersioned in precompiles/wasmd/setup.go. 1 in-repo callers (1 non-test, 0 test). |
| <code>interface.6.go-pc-wasmd-legacy-v620-new-precompile</code> | notes | Legacy wasmd precompile constructor for version v620, registered by GetVersioned in precompiles/wasmd/setup.go. 1 in-repo callers (1 non-test, 0 test). |
| <code>interface.6.go-pc-wasmd-legacy-v630-new-precompile</code> | notes | Legacy wasmd precompile constructor for version v630, registered by GetVersioned in precompiles/wasmd/setup.go. 1 in-repo callers (1 non-test, 0 test). |
| <code>interface.6.go-pc-wasmd-legacy-v640-new-precompile</code> | notes | Legacy wasmd precompile constructor for version v640, registered by GetVersioned in precompiles/wasmd/setup.go. 1 in-repo callers (1 non-test, 0 test). |
| <code>interface.6.go-pc-wasmd-legacy-v65-new-precompile</code> | notes | Legacy wasmd precompile constructor for version v65; not imported by precompiles/wasmd/setup.go or any other file, so this package is compiled only as a standalone package and never served. 0 in-repo callers (0 non-test, 0 test). |
| <code>interface.6.go-pc-wasmd-newprecompile</code> | notes | Exported func used outside its tree; 1 in-repo callers (1 non-test, 0 test). |
| <code>interface.6.go-pc-wasmd-precompilename</code> | notes | Exported var used outside its tree; 1 in-repo callers (0 non-test, 1 test). |
| <code>interface.6.go-pc-wasmd-wasmdaddress</code> | notes | Exported var used outside its tree; 8 in-repo callers (4 non-test, 4 test). |

## proposals

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.6.msg-wasm-proposal-clearadmin</code> | <code>cosmos_msg</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/proposal_handler.go:188</code> | <code>gov proposal content cosmwasm.wasm.v1.ClearAdminProposal (ProposalType "ClearAdmin", route wasm) -&gt; handleClearAdminProposal</code> |
| <code>interface.6.msg-wasm-proposal-executecontract</code> | <code>cosmos_msg</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/proposal_handler.go:147</code> | <code>gov proposal content cosmwasm.wasm.v1.ExecuteContractProposal (ProposalType "ExecuteContract", route wasm) -&gt; handleExecuteProposal</code> |
| <code>interface.6.msg-wasm-proposal-instantiatecontract</code> | <code>cosmos_msg</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/proposal_handler.go:74</code> | <code>gov proposal content cosmwasm.wasm.v1.InstantiateContractProposal (ProposalType "InstantiateContract", route wasm) -&gt; handleInstantiateProposal</code> |
| <code>interface.6.msg-wasm-proposal-migratecontract</code> | <code>cosmos_msg</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/proposal_handler.go:101</code> | <code>gov proposal content cosmwasm.wasm.v1.MigrateContractProposal (ProposalType "MigrateContract", route wasm) -&gt; handleMigrateProposal</code> |
| <code>interface.6.msg-wasm-proposal-pincodes</code> | <code>cosmos_msg</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/proposal_handler.go:203</code> | <code>gov proposal content cosmwasm.wasm.v1.PinCodesProposal (ProposalType "PinCodes", route wasm) -&gt; handlePinCodesProposal</code> |
| <code>interface.6.msg-wasm-proposal-storecode</code> | <code>cosmos_msg</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/proposal_handler.go:58</code> | <code>gov proposal content cosmwasm.wasm.v1.StoreCodeProposal (ProposalType "StoreCode", route wasm) -&gt; handleStoreCodeProposal</code> |
| <code>interface.6.msg-wasm-proposal-sudocontract</code> | <code>cosmos_msg</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/proposal_handler.go:126</code> | <code>gov proposal content cosmwasm.wasm.v1.SudoContractProposal (ProposalType "SudoContract", route wasm) -&gt; handleSudoProposal</code> |
| <code>interface.6.msg-wasm-proposal-unpincodes</code> | <code>cosmos_msg</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/proposal_handler.go:215</code> | <code>gov proposal content cosmwasm.wasm.v1.UnpinCodesProposal (ProposalType "UnpinCodes", route wasm) -&gt; handleUnpinCodesProposal</code> |
| <code>interface.6.msg-wasm-proposal-updateadmin</code> | <code>cosmos_msg</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/proposal_handler.go:172</code> | <code>gov proposal content cosmwasm.wasm.v1.UpdateAdminProposal (ProposalType "UpdateAdmin", route wasm) -&gt; handleUpdateAdminProposal</code> |
| <code>interface.6.msg-wasm-proposal-updateinstantiateconfig</code> | <code>cosmos_msg</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/proposal_handler.go:227</code> | <code>gov proposal content cosmwasm.wasm.v1.UpdateInstantiateConfigProposal (ProposalType "UpdateInstantiateConfig", route wasm) -&gt; handleUpdateInstantiateConfigProposal</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.6.msg-wasm-proposal-clearadmin</code> | notes | Route registered at node/app.go:972 only when GetWasmEnabledProposals (node/app.go:406) returns a non-empty list; build var WasmProposalsEnabled defaults to true (node/app.go:359) so all ten types are enabled. 14 in-repo callers (12 non-test, 2 test). |
| <code>interface.6.msg-wasm-proposal-executecontract</code> | notes | Route registered at node/app.go:972 only when GetWasmEnabledProposals (node/app.go:406) returns a non-empty list; build var WasmProposalsEnabled defaults to true (node/app.go:359) so all ten types are enabled. 14 in-repo callers (12 non-test, 2 test). |
| <code>interface.6.msg-wasm-proposal-instantiatecontract</code> | notes | Route registered at node/app.go:972 only when GetWasmEnabledProposals (node/app.go:406) returns a non-empty list; build var WasmProposalsEnabled defaults to true (node/app.go:359) so all ten types are enabled. 18 in-repo callers (13 non-test, 5 test). |
| <code>interface.6.msg-wasm-proposal-migratecontract</code> | notes | Route registered at node/app.go:972 only when GetWasmEnabledProposals (node/app.go:406) returns a non-empty list; build var WasmProposalsEnabled defaults to true (node/app.go:359) so all ten types are enabled. 17 in-repo callers (12 non-test, 5 test). |
| <code>interface.6.msg-wasm-proposal-pincodes</code> | notes | Route registered at node/app.go:972 only when GetWasmEnabledProposals (node/app.go:406) returns a non-empty list; build var WasmProposalsEnabled defaults to true (node/app.go:359) so all ten types are enabled. 14 in-repo callers (11 non-test, 3 test). |
| <code>interface.6.msg-wasm-proposal-storecode</code> | notes | Route registered at node/app.go:972 only when GetWasmEnabledProposals (node/app.go:406) returns a non-empty list; build var WasmProposalsEnabled defaults to true (node/app.go:359) so all ten types are enabled. 15 in-repo callers (13 non-test, 2 test). |
| <code>interface.6.msg-wasm-proposal-sudocontract</code> | notes | Route registered at node/app.go:972 only when GetWasmEnabledProposals (node/app.go:406) returns a non-empty list; build var WasmProposalsEnabled defaults to true (node/app.go:359) so all ten types are enabled. 13 in-repo callers (12 non-test, 1 test). |
| <code>interface.6.msg-wasm-proposal-unpincodes</code> | notes | Route registered at node/app.go:972 only when GetWasmEnabledProposals (node/app.go:406) returns a non-empty list; build var WasmProposalsEnabled defaults to true (node/app.go:359) so all ten types are enabled. 13 in-repo callers (11 non-test, 2 test). |
| <code>interface.6.msg-wasm-proposal-updateadmin</code> | notes | Route registered at node/app.go:972 only when GetWasmEnabledProposals (node/app.go:406) returns a non-empty list; build var WasmProposalsEnabled defaults to true (node/app.go:359) so all ten types are enabled. 15 in-repo callers (12 non-test, 3 test). |
| <code>interface.6.msg-wasm-proposal-updateinstantiateconfig</code> | notes | Route registered at node/app.go:972 only when GetWasmEnabledProposals (node/app.go:406) returns a non-empty list; build var WasmProposalsEnabled defaults to true (node/app.go:359) so all ten types are enabled. 12 in-repo callers (11 non-test, 1 test). |

## proto

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.6.proto-cosmwasm-wasm-v1-absolutetxposition-j37</code> | <code>proto_message</code> | <code>stable</code> | <code>wasm/proto/cosmwasm/wasm/v1/types.proto:109</code> | <code>message AbsoluteTxPosition (package cosmwasm.wasm.v1)</code> |
| <code>interface.6.proto-cosmwasm-wasm-v1-accessconfig-j37</code> | <code>proto_message</code> | <code>stable</code> | <code>wasm/proto/cosmwasm/wasm/v1/types.proto:33</code> | <code>message AccessConfig (package cosmwasm.wasm.v1)</code> |
| <code>interface.6.proto-cosmwasm-wasm-v1-accessconfigupdate-j37</code> | <code>proto_message</code> | <code>stable</code> | <code>wasm/proto/cosmwasm/wasm/v1/proposal.proto:154</code> | <code>message AccessConfigUpdate (package cosmwasm.wasm.v1)</code> |
| <code>interface.6.proto-cosmwasm-wasm-v1-accesstypeparam-j37</code> | <code>proto_message</code> | <code>stable</code> | <code>wasm/proto/cosmwasm/wasm/v1/types.proto:27</code> | <code>message AccessTypeParam (package cosmwasm.wasm.v1)</code> |
| <code>interface.6.proto-cosmwasm-wasm-v1-clearadminproposal-j37</code> | <code>proto_message</code> | <code>stable</code> | <code>wasm/proto/cosmwasm/wasm/v1/proposal.proto:115</code> | <code>message ClearAdminProposal (package cosmwasm.wasm.v1)</code> |
| <code>interface.6.proto-cosmwasm-wasm-v1-code-j37</code> | <code>proto_message</code> | <code>stable</code> | <code>wasm/proto/cosmwasm/wasm/v1/genesis.proto:43</code> | <code>message Code (package cosmwasm.wasm.v1)</code> |
| <code>interface.6.proto-cosmwasm-wasm-v1-codeinfo-j37</code> | <code>proto_message</code> | <code>stable</code> | <code>wasm/proto/cosmwasm/wasm/v1/types.proto:50</code> | <code>message CodeInfo (package cosmwasm.wasm.v1)</code> |
| <code>interface.6.proto-cosmwasm-wasm-v1-codeinforesponse-j37</code> | <code>proto_message</code> | <code>stable</code> | <code>wasm/proto/cosmwasm/wasm/v1/query.proto:164</code> | <code>message CodeInfoResponse (package cosmwasm.wasm.v1)</code> |
| <code>interface.6.proto-cosmwasm-wasm-v1-contract-j37</code> | <code>proto_message</code> | <code>stable</code> | <code>wasm/proto/cosmwasm/wasm/v1/genesis.proto:52</code> | <code>message Contract (package cosmwasm.wasm.v1)</code> |
| <code>interface.6.proto-cosmwasm-wasm-v1-contractcodehistoryentry-j37</code> | <code>proto_message</code> | <code>stable</code> | <code>wasm/proto/cosmwasm/wasm/v1/types.proto:98</code> | <code>message ContractCodeHistoryEntry (package cosmwasm.wasm.v1)</code> |
| <code>interface.6.proto-cosmwasm-wasm-v1-contractinfo-j37</code> | <code>proto_message</code> | <code>stable</code> | <code>wasm/proto/cosmwasm/wasm/v1/types.proto:62</code> | <code>message ContractInfo (package cosmwasm.wasm.v1)</code> |
| <code>interface.6.proto-cosmwasm-wasm-v1-executecontractproposal-j37</code> | <code>proto_message</code> | <code>stable</code> | <code>wasm/proto/cosmwasm/wasm/v1/proposal.proto:83</code> | <code>message ExecuteContractProposal (package cosmwasm.wasm.v1)</code> |
| <code>interface.6.proto-cosmwasm-wasm-v1-genesisstate-genmsgs-j37</code> | <code>proto_message</code> | <code>stable</code> | <code>wasm/proto/cosmwasm/wasm/v1/genesis.proto:32</code> | <code>message GenesisState.GenMsgs (package cosmwasm.wasm.v1)</code> |
| <code>interface.6.proto-cosmwasm-wasm-v1-genesisstate-j37</code> | <code>proto_message</code> | <code>stable</code> | <code>wasm/proto/cosmwasm/wasm/v1/genesis.proto:11</code> | <code>message GenesisState (package cosmwasm.wasm.v1)</code> |
| <code>interface.6.proto-cosmwasm-wasm-v1-instantiatecontractproposal-j37</code> | <code>proto_message</code> | <code>stable</code> | <code>wasm/proto/cosmwasm/wasm/v1/proposal.proto:31</code> | <code>message InstantiateContractProposal (package cosmwasm.wasm.v1)</code> |
| <code>interface.6.proto-cosmwasm-wasm-v1-migratecontractproposal-j37</code> | <code>proto_message</code> | <code>stable</code> | <code>wasm/proto/cosmwasm/wasm/v1/proposal.proto:54</code> | <code>message MigrateContractProposal (package cosmwasm.wasm.v1)</code> |
| <code>interface.6.proto-cosmwasm-wasm-v1-model-j37</code> | <code>proto_message</code> | <code>stable</code> | <code>wasm/proto/cosmwasm/wasm/v1/types.proto:118</code> | <code>message Model (package cosmwasm.wasm.v1)</code> |
| <code>interface.6.proto-cosmwasm-wasm-v1-msgclearadmin-j37</code> | <code>proto_message</code> | <code>stable</code> | <code>wasm/proto/cosmwasm/wasm/v1/tx.proto:128</code> | <code>message MsgClearAdmin (package cosmwasm.wasm.v1)</code> |
| <code>interface.6.proto-cosmwasm-wasm-v1-msgclearadminresponse-j37</code> | <code>proto_message</code> | <code>stable</code> | <code>wasm/proto/cosmwasm/wasm/v1/tx.proto:136</code> | <code>message MsgClearAdminResponse (package cosmwasm.wasm.v1)</code> |
| <code>interface.6.proto-cosmwasm-wasm-v1-msgexecutecontract-j37</code> | <code>proto_message</code> | <code>stable</code> | <code>wasm/proto/cosmwasm/wasm/v1/tx.proto:75</code> | <code>message MsgExecuteContract (package cosmwasm.wasm.v1)</code> |
| <code>interface.6.proto-cosmwasm-wasm-v1-msgexecutecontractresponse-j37</code> | <code>proto_message</code> | <code>stable</code> | <code>wasm/proto/cosmwasm/wasm/v1/tx.proto:90</code> | <code>message MsgExecuteContractResponse (package cosmwasm.wasm.v1)</code> |
| <code>interface.6.proto-cosmwasm-wasm-v1-msgibcclosechannel-j37</code> | <code>proto_message</code> | <code>stable</code> | <code>wasm/proto/cosmwasm/wasm/v1/ibc.proto:27</code> | <code>message MsgIBCCloseChannel (package cosmwasm.wasm.v1)</code> |
| <code>interface.6.proto-cosmwasm-wasm-v1-msgibcsend-j37</code> | <code>proto_message</code> | <code>stable</code> | <code>wasm/proto/cosmwasm/wasm/v1/ibc.proto:10</code> | <code>message MsgIBCSend (package cosmwasm.wasm.v1)</code> |
| <code>interface.6.proto-cosmwasm-wasm-v1-msginstantiatecontract-j37</code> | <code>proto_message</code> | <code>stable</code> | <code>wasm/proto/cosmwasm/wasm/v1/tx.proto:48</code> | <code>message MsgInstantiateContract (package cosmwasm.wasm.v1)</code> |
| <code>interface.6.proto-cosmwasm-wasm-v1-msginstantiatecontractresponse-j37</code> | <code>proto_message</code> | <code>stable</code> | <code>wasm/proto/cosmwasm/wasm/v1/tx.proto:67</code> | <code>message MsgInstantiateContractResponse (package cosmwasm.wasm.v1)</code> |
| <code>interface.6.proto-cosmwasm-wasm-v1-msgmigratecontract-j37</code> | <code>proto_message</code> | <code>stable</code> | <code>wasm/proto/cosmwasm/wasm/v1/tx.proto:96</code> | <code>message MsgMigrateContract (package cosmwasm.wasm.v1)</code> |
| <code>interface.6.proto-cosmwasm-wasm-v1-msgmigratecontractresponse-j37</code> | <code>proto_message</code> | <code>stable</code> | <code>wasm/proto/cosmwasm/wasm/v1/tx.proto:108</code> | <code>message MsgMigrateContractResponse (package cosmwasm.wasm.v1)</code> |
| <code>interface.6.proto-cosmwasm-wasm-v1-msgstorecode-j37</code> | <code>proto_message</code> | <code>stable</code> | <code>wasm/proto/cosmwasm/wasm/v1/tx.proto:28</code> | <code>message MsgStoreCode (package cosmwasm.wasm.v1)</code> |
| <code>interface.6.proto-cosmwasm-wasm-v1-msgstorecoderesponse-j37</code> | <code>proto_message</code> | <code>stable</code> | <code>wasm/proto/cosmwasm/wasm/v1/tx.proto:41</code> | <code>message MsgStoreCodeResponse (package cosmwasm.wasm.v1)</code> |
| <code>interface.6.proto-cosmwasm-wasm-v1-msgupdateadmin-j37</code> | <code>proto_message</code> | <code>stable</code> | <code>wasm/proto/cosmwasm/wasm/v1/tx.proto:115</code> | <code>message MsgUpdateAdmin (package cosmwasm.wasm.v1)</code> |
| <code>interface.6.proto-cosmwasm-wasm-v1-msgupdateadminresponse-j37</code> | <code>proto_message</code> | <code>stable</code> | <code>wasm/proto/cosmwasm/wasm/v1/tx.proto:125</code> | <code>message MsgUpdateAdminResponse (package cosmwasm.wasm.v1)</code> |
| <code>interface.6.proto-cosmwasm-wasm-v1-params-j37</code> | <code>proto_message</code> | <code>stable</code> | <code>wasm/proto/cosmwasm/wasm/v1/types.proto:40</code> | <code>message Params (package cosmwasm.wasm.v1)</code> |
| <code>interface.6.proto-cosmwasm-wasm-v1-pincodesproposal-j37</code> | <code>proto_message</code> | <code>stable</code> | <code>wasm/proto/cosmwasm/wasm/v1/proposal.proto:126</code> | <code>message PinCodesProposal (package cosmwasm.wasm.v1)</code> |
| <code>interface.6.proto-cosmwasm-wasm-v1-queryallcontractstaterequest-j37</code> | <code>proto_message</code> | <code>stable</code> | <code>wasm/proto/cosmwasm/wasm/v1/query.proto:112</code> | <code>message QueryAllContractStateRequest (package cosmwasm.wasm.v1)</code> |
| <code>interface.6.proto-cosmwasm-wasm-v1-queryallcontractstateresponse-j37</code> | <code>proto_message</code> | <code>stable</code> | <code>wasm/proto/cosmwasm/wasm/v1/query.proto:121</code> | <code>message QueryAllContractStateResponse (package cosmwasm.wasm.v1)</code> |
| <code>interface.6.proto-cosmwasm-wasm-v1-querycoderequest-j37</code> | <code>proto_message</code> | <code>stable</code> | <code>wasm/proto/cosmwasm/wasm/v1/query.proto:159</code> | <code>message QueryCodeRequest (package cosmwasm.wasm.v1)</code> |
| <code>interface.6.proto-cosmwasm-wasm-v1-querycoderesponse-j37</code> | <code>proto_message</code> | <code>stable</code> | <code>wasm/proto/cosmwasm/wasm/v1/query.proto:179</code> | <code>message QueryCodeResponse (package cosmwasm.wasm.v1)</code> |
| <code>interface.6.proto-cosmwasm-wasm-v1-querycodesrequest-j37</code> | <code>proto_message</code> | <code>stable</code> | <code>wasm/proto/cosmwasm/wasm/v1/query.proto:189</code> | <code>message QueryCodesRequest (package cosmwasm.wasm.v1)</code> |
| <code>interface.6.proto-cosmwasm-wasm-v1-querycodesresponse-j37</code> | <code>proto_message</code> | <code>stable</code> | <code>wasm/proto/cosmwasm/wasm/v1/query.proto:195</code> | <code>message QueryCodesResponse (package cosmwasm.wasm.v1)</code> |
| <code>interface.6.proto-cosmwasm-wasm-v1-querycontracthistoryrequest-j37</code> | <code>proto_message</code> | <code>stable</code> | <code>wasm/proto/cosmwasm/wasm/v1/query.proto:77</code> | <code>message QueryContractHistoryRequest (package cosmwasm.wasm.v1)</code> |
| <code>interface.6.proto-cosmwasm-wasm-v1-querycontracthistoryresponse-j37</code> | <code>proto_message</code> | <code>stable</code> | <code>wasm/proto/cosmwasm/wasm/v1/query.proto:86</code> | <code>message QueryContractHistoryResponse (package cosmwasm.wasm.v1)</code> |
| <code>interface.6.proto-cosmwasm-wasm-v1-querycontractinforequest-j37</code> | <code>proto_message</code> | <code>stable</code> | <code>wasm/proto/cosmwasm/wasm/v1/query.proto:56</code> | <code>message QueryContractInfoRequest (package cosmwasm.wasm.v1)</code> |
| <code>interface.6.proto-cosmwasm-wasm-v1-querycontractinforesponse-j37</code> | <code>proto_message</code> | <code>stable</code> | <code>wasm/proto/cosmwasm/wasm/v1/query.proto:63</code> | <code>message QueryContractInfoResponse (package cosmwasm.wasm.v1)</code> |
| <code>interface.6.proto-cosmwasm-wasm-v1-querycontractsbycoderequest-j37</code> | <code>proto_message</code> | <code>stable</code> | <code>wasm/proto/cosmwasm/wasm/v1/query.proto:94</code> | <code>message QueryContractsByCodeRequest (package cosmwasm.wasm.v1)</code> |
| <code>interface.6.proto-cosmwasm-wasm-v1-querycontractsbycoderesponse-j37</code> | <code>proto_message</code> | <code>stable</code> | <code>wasm/proto/cosmwasm/wasm/v1/query.proto:102</code> | <code>message QueryContractsByCodeResponse (package cosmwasm.wasm.v1)</code> |
| <code>interface.6.proto-cosmwasm-wasm-v1-querypinnedcodesrequest-j37</code> | <code>proto_message</code> | <code>stable</code> | <code>wasm/proto/cosmwasm/wasm/v1/query.proto:203</code> | <code>message QueryPinnedCodesRequest (package cosmwasm.wasm.v1)</code> |
| <code>interface.6.proto-cosmwasm-wasm-v1-querypinnedcodesresponse-j37</code> | <code>proto_message</code> | <code>stable</code> | <code>wasm/proto/cosmwasm/wasm/v1/query.proto:210</code> | <code>message QueryPinnedCodesResponse (package cosmwasm.wasm.v1)</code> |
| <code>interface.6.proto-cosmwasm-wasm-v1-queryrawcontractstaterequest-j37</code> | <code>proto_message</code> | <code>stable</code> | <code>wasm/proto/cosmwasm/wasm/v1/query.proto:129</code> | <code>message QueryRawContractStateRequest (package cosmwasm.wasm.v1)</code> |
| <code>interface.6.proto-cosmwasm-wasm-v1-queryrawcontractstateresponse-j37</code> | <code>proto_message</code> | <code>stable</code> | <code>wasm/proto/cosmwasm/wasm/v1/query.proto:137</code> | <code>message QueryRawContractStateResponse (package cosmwasm.wasm.v1)</code> |
| <code>interface.6.proto-cosmwasm-wasm-v1-querysmartcontractstaterequest-j37</code> | <code>proto_message</code> | <code>stable</code> | <code>wasm/proto/cosmwasm/wasm/v1/query.proto:144</code> | <code>message QuerySmartContractStateRequest (package cosmwasm.wasm.v1)</code> |
| <code>interface.6.proto-cosmwasm-wasm-v1-querysmartcontractstateresponse-j37</code> | <code>proto_message</code> | <code>stable</code> | <code>wasm/proto/cosmwasm/wasm/v1/query.proto:153</code> | <code>message QuerySmartContractStateResponse (package cosmwasm.wasm.v1)</code> |
| <code>interface.6.proto-cosmwasm-wasm-v1-sequence-j37</code> | <code>proto_message</code> | <code>stable</code> | <code>wasm/proto/cosmwasm/wasm/v1/genesis.proto:59</code> | <code>message Sequence (package cosmwasm.wasm.v1)</code> |
| <code>interface.6.proto-cosmwasm-wasm-v1-storecodeproposal-j37</code> | <code>proto_message</code> | <code>stable</code> | <code>wasm/proto/cosmwasm/wasm/v1/proposal.proto:14</code> | <code>message StoreCodeProposal (package cosmwasm.wasm.v1)</code> |
| <code>interface.6.proto-cosmwasm-wasm-v1-sudocontractproposal-j37</code> | <code>proto_message</code> | <code>stable</code> | <code>wasm/proto/cosmwasm/wasm/v1/proposal.proto:70</code> | <code>message SudoContractProposal (package cosmwasm.wasm.v1)</code> |
| <code>interface.6.proto-cosmwasm-wasm-v1-unpincodesproposal-j37</code> | <code>proto_message</code> | <code>stable</code> | <code>wasm/proto/cosmwasm/wasm/v1/proposal.proto:140</code> | <code>message UnpinCodesProposal (package cosmwasm.wasm.v1)</code> |
| <code>interface.6.proto-cosmwasm-wasm-v1-updateadminproposal-j37</code> | <code>proto_message</code> | <code>stable</code> | <code>wasm/proto/cosmwasm/wasm/v1/proposal.proto:102</code> | <code>message UpdateAdminProposal (package cosmwasm.wasm.v1)</code> |
| <code>interface.6.proto-cosmwasm-wasm-v1-updateinstantiateconfigproposal-j37</code> | <code>proto_message</code> | <code>stable</code> | <code>wasm/proto/cosmwasm/wasm/v1/proposal.proto:163</code> | <code>message UpdateInstantiateConfigProposal (package cosmwasm.wasm.v1)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.6.proto-cosmwasm-wasm-v1-absolutetxposition-j37</code> | notes | Proto message of package cosmwasm.wasm.v1; Go type AbsoluteTxPosition is generated into wasm/x/wasm/types; referenced only through the generated code, so no direct in-repo callers are listed. |
| <code>interface.6.proto-cosmwasm-wasm-v1-accessconfig-j37</code> | notes | Proto message of package cosmwasm.wasm.v1; Go type AccessConfig is generated into wasm/x/wasm/types; referenced only through the generated code, so no direct in-repo callers are listed. |
| <code>interface.6.proto-cosmwasm-wasm-v1-accessconfigupdate-j37</code> | notes | Proto message of package cosmwasm.wasm.v1; Go type AccessConfigUpdate is generated into wasm/x/wasm/types; referenced only through the generated code, so no direct in-repo callers are listed. |
| <code>interface.6.proto-cosmwasm-wasm-v1-accesstypeparam-j37</code> | notes | Proto message of package cosmwasm.wasm.v1; Go type AccessTypeParam is generated into wasm/x/wasm/types; referenced only through the generated code, so no direct in-repo callers are listed. |
| <code>interface.6.proto-cosmwasm-wasm-v1-clearadminproposal-j37</code> | notes | Proto message of package cosmwasm.wasm.v1; Go type ClearAdminProposal is generated into wasm/x/wasm/types; referenced only through the generated code, so no direct in-repo callers are listed. |
| <code>interface.6.proto-cosmwasm-wasm-v1-code-j37</code> | notes | Proto message of package cosmwasm.wasm.v1; Go type Code is generated into wasm/x/wasm/types; referenced only through the generated code, so no direct in-repo callers are listed. |
| <code>interface.6.proto-cosmwasm-wasm-v1-codeinfo-j37</code> | notes | Proto message of package cosmwasm.wasm.v1; Go type CodeInfo is generated into wasm/x/wasm/types; referenced only through the generated code, so no direct in-repo callers are listed. |
| <code>interface.6.proto-cosmwasm-wasm-v1-codeinforesponse-j37</code> | notes | Proto message of package cosmwasm.wasm.v1; Go type CodeInfoResponse is generated into wasm/x/wasm/types; referenced only through the generated code, so no direct in-repo callers are listed. |
| <code>interface.6.proto-cosmwasm-wasm-v1-contract-j37</code> | notes | Proto message of package cosmwasm.wasm.v1; Go type Contract is generated into wasm/x/wasm/types; referenced only through the generated code, so no direct in-repo callers are listed. |
| <code>interface.6.proto-cosmwasm-wasm-v1-contractcodehistoryentry-j37</code> | notes | Proto message of package cosmwasm.wasm.v1; Go type ContractCodeHistoryEntry is generated into wasm/x/wasm/types; referenced only through the generated code, so no direct in-repo callers are listed. |
| <code>interface.6.proto-cosmwasm-wasm-v1-contractinfo-j37</code> | notes | Proto message of package cosmwasm.wasm.v1; Go type ContractInfo is generated into wasm/x/wasm/types; referenced only through the generated code, so no direct in-repo callers are listed. |
| <code>interface.6.proto-cosmwasm-wasm-v1-executecontractproposal-j37</code> | notes | Proto message of package cosmwasm.wasm.v1; Go type ExecuteContractProposal is generated into wasm/x/wasm/types; referenced only through the generated code, so no direct in-repo callers are listed. |
| <code>interface.6.proto-cosmwasm-wasm-v1-genesisstate-genmsgs-j37</code> | notes | Proto message of package cosmwasm.wasm.v1; Go type GenesisState_GenMsgs is generated into wasm/x/wasm/types; referenced only through the generated code, so no direct in-repo callers are listed. |
| <code>interface.6.proto-cosmwasm-wasm-v1-genesisstate-j37</code> | notes | Proto message of package cosmwasm.wasm.v1; Go type GenesisState is generated into wasm/x/wasm/types; referenced only through the generated code, so no direct in-repo callers are listed. |
| <code>interface.6.proto-cosmwasm-wasm-v1-instantiatecontractproposal-j37</code> | notes | Proto message of package cosmwasm.wasm.v1; Go type InstantiateContractProposal is generated into wasm/x/wasm/types; referenced only through the generated code, so no direct in-repo callers are listed. |
| <code>interface.6.proto-cosmwasm-wasm-v1-migratecontractproposal-j37</code> | notes | Proto message of package cosmwasm.wasm.v1; Go type MigrateContractProposal is generated into wasm/x/wasm/types; referenced only through the generated code, so no direct in-repo callers are listed. |
| <code>interface.6.proto-cosmwasm-wasm-v1-model-j37</code> | notes | Proto message of package cosmwasm.wasm.v1; Go type Model is generated into wasm/x/wasm/types; referenced only through the generated code, so no direct in-repo callers are listed. |
| <code>interface.6.proto-cosmwasm-wasm-v1-msgclearadmin-j37</code> | notes | Proto message of package cosmwasm.wasm.v1; Go type MsgClearAdmin is generated into wasm/x/wasm/types; referenced only through the generated code, so no direct in-repo callers are listed. |
| <code>interface.6.proto-cosmwasm-wasm-v1-msgclearadminresponse-j37</code> | notes | Proto message of package cosmwasm.wasm.v1; Go type MsgClearAdminResponse is generated into wasm/x/wasm/types; referenced only through the generated code, so no direct in-repo callers are listed. |
| <code>interface.6.proto-cosmwasm-wasm-v1-msgexecutecontract-j37</code> | notes | Proto message of package cosmwasm.wasm.v1; Go type MsgExecuteContract is generated into wasm/x/wasm/types; referenced only through the generated code, so no direct in-repo callers are listed. |
| <code>interface.6.proto-cosmwasm-wasm-v1-msgexecutecontractresponse-j37</code> | notes | Proto message of package cosmwasm.wasm.v1; Go type MsgExecuteContractResponse is generated into wasm/x/wasm/types; referenced only through the generated code, so no direct in-repo callers are listed. |
| <code>interface.6.proto-cosmwasm-wasm-v1-msgibcclosechannel-j37</code> | notes | Proto message of package cosmwasm.wasm.v1; Go type MsgIBCCloseChannel is generated into wasm/x/wasm/types; referenced only through the generated code, so no direct in-repo callers are listed. |
| <code>interface.6.proto-cosmwasm-wasm-v1-msgibcsend-j37</code> | notes | Proto message of package cosmwasm.wasm.v1; Go type MsgIBCSend is generated into wasm/x/wasm/types; referenced only through the generated code, so no direct in-repo callers are listed. |
| <code>interface.6.proto-cosmwasm-wasm-v1-msginstantiatecontract-j37</code> | notes | Proto message of package cosmwasm.wasm.v1; Go type MsgInstantiateContract is generated into wasm/x/wasm/types; referenced only through the generated code, so no direct in-repo callers are listed. |
| <code>interface.6.proto-cosmwasm-wasm-v1-msginstantiatecontractresponse-j37</code> | notes | Proto message of package cosmwasm.wasm.v1; Go type MsgInstantiateContractResponse is generated into wasm/x/wasm/types; referenced only through the generated code, so no direct in-repo callers are listed. |
| <code>interface.6.proto-cosmwasm-wasm-v1-msgmigratecontract-j37</code> | notes | Proto message of package cosmwasm.wasm.v1; Go type MsgMigrateContract is generated into wasm/x/wasm/types; referenced only through the generated code, so no direct in-repo callers are listed. |
| <code>interface.6.proto-cosmwasm-wasm-v1-msgmigratecontractresponse-j37</code> | notes | Proto message of package cosmwasm.wasm.v1; Go type MsgMigrateContractResponse is generated into wasm/x/wasm/types; referenced only through the generated code, so no direct in-repo callers are listed. |
| <code>interface.6.proto-cosmwasm-wasm-v1-msgstorecode-j37</code> | notes | Proto message of package cosmwasm.wasm.v1; Go type MsgStoreCode is generated into wasm/x/wasm/types; referenced only through the generated code, so no direct in-repo callers are listed. |
| <code>interface.6.proto-cosmwasm-wasm-v1-msgstorecoderesponse-j37</code> | notes | Proto message of package cosmwasm.wasm.v1; Go type MsgStoreCodeResponse is generated into wasm/x/wasm/types; referenced only through the generated code, so no direct in-repo callers are listed. |
| <code>interface.6.proto-cosmwasm-wasm-v1-msgupdateadmin-j37</code> | notes | Proto message of package cosmwasm.wasm.v1; Go type MsgUpdateAdmin is generated into wasm/x/wasm/types; referenced only through the generated code, so no direct in-repo callers are listed. |
| <code>interface.6.proto-cosmwasm-wasm-v1-msgupdateadminresponse-j37</code> | notes | Proto message of package cosmwasm.wasm.v1; Go type MsgUpdateAdminResponse is generated into wasm/x/wasm/types; referenced only through the generated code, so no direct in-repo callers are listed. |
| <code>interface.6.proto-cosmwasm-wasm-v1-params-j37</code> | notes | Proto message of package cosmwasm.wasm.v1; Go type Params is generated into wasm/x/wasm/types; referenced only through the generated code, so no direct in-repo callers are listed. |
| <code>interface.6.proto-cosmwasm-wasm-v1-pincodesproposal-j37</code> | notes | Proto message of package cosmwasm.wasm.v1; Go type PinCodesProposal is generated into wasm/x/wasm/types; referenced only through the generated code, so no direct in-repo callers are listed. |
| <code>interface.6.proto-cosmwasm-wasm-v1-queryallcontractstaterequest-j37</code> | notes | Proto message of package cosmwasm.wasm.v1; Go type QueryAllContractStateRequest is generated into wasm/x/wasm/types; referenced only through the generated code, so no direct in-repo callers are listed. |
| <code>interface.6.proto-cosmwasm-wasm-v1-queryallcontractstateresponse-j37</code> | notes | Proto message of package cosmwasm.wasm.v1; Go type QueryAllContractStateResponse is generated into wasm/x/wasm/types; referenced only through the generated code, so no direct in-repo callers are listed. |
| <code>interface.6.proto-cosmwasm-wasm-v1-querycoderequest-j37</code> | notes | Proto message of package cosmwasm.wasm.v1; Go type QueryCodeRequest is generated into wasm/x/wasm/types; referenced only through the generated code, so no direct in-repo callers are listed. |
| <code>interface.6.proto-cosmwasm-wasm-v1-querycoderesponse-j37</code> | notes | Proto message of package cosmwasm.wasm.v1; Go type QueryCodeResponse is generated into wasm/x/wasm/types; referenced only through the generated code, so no direct in-repo callers are listed. |
| <code>interface.6.proto-cosmwasm-wasm-v1-querycodesrequest-j37</code> | notes | Proto message of package cosmwasm.wasm.v1; Go type QueryCodesRequest is generated into wasm/x/wasm/types; referenced only through the generated code, so no direct in-repo callers are listed. |
| <code>interface.6.proto-cosmwasm-wasm-v1-querycodesresponse-j37</code> | notes | Proto message of package cosmwasm.wasm.v1; Go type QueryCodesResponse is generated into wasm/x/wasm/types; referenced only through the generated code, so no direct in-repo callers are listed. |
| <code>interface.6.proto-cosmwasm-wasm-v1-querycontracthistoryrequest-j37</code> | notes | Proto message of package cosmwasm.wasm.v1; Go type QueryContractHistoryRequest is generated into wasm/x/wasm/types; referenced only through the generated code, so no direct in-repo callers are listed. |
| <code>interface.6.proto-cosmwasm-wasm-v1-querycontracthistoryresponse-j37</code> | notes | Proto message of package cosmwasm.wasm.v1; Go type QueryContractHistoryResponse is generated into wasm/x/wasm/types; referenced only through the generated code, so no direct in-repo callers are listed. |
| <code>interface.6.proto-cosmwasm-wasm-v1-querycontractinforequest-j37</code> | notes | Proto message of package cosmwasm.wasm.v1; Go type QueryContractInfoRequest is generated into wasm/x/wasm/types; referenced only through the generated code, so no direct in-repo callers are listed. |
| <code>interface.6.proto-cosmwasm-wasm-v1-querycontractinforesponse-j37</code> | notes | Proto message of package cosmwasm.wasm.v1; Go type QueryContractInfoResponse is generated into wasm/x/wasm/types; referenced only through the generated code, so no direct in-repo callers are listed. |
| <code>interface.6.proto-cosmwasm-wasm-v1-querycontractsbycoderequest-j37</code> | notes | Proto message of package cosmwasm.wasm.v1; Go type QueryContractsByCodeRequest is generated into wasm/x/wasm/types; referenced only through the generated code, so no direct in-repo callers are listed. |
| <code>interface.6.proto-cosmwasm-wasm-v1-querycontractsbycoderesponse-j37</code> | notes | Proto message of package cosmwasm.wasm.v1; Go type QueryContractsByCodeResponse is generated into wasm/x/wasm/types; referenced only through the generated code, so no direct in-repo callers are listed. |
| <code>interface.6.proto-cosmwasm-wasm-v1-querypinnedcodesrequest-j37</code> | notes | Proto message of package cosmwasm.wasm.v1; Go type QueryPinnedCodesRequest is generated into wasm/x/wasm/types; referenced only through the generated code, so no direct in-repo callers are listed. |
| <code>interface.6.proto-cosmwasm-wasm-v1-querypinnedcodesresponse-j37</code> | notes | Proto message of package cosmwasm.wasm.v1; Go type QueryPinnedCodesResponse is generated into wasm/x/wasm/types; referenced only through the generated code, so no direct in-repo callers are listed. |
| <code>interface.6.proto-cosmwasm-wasm-v1-queryrawcontractstaterequest-j37</code> | notes | Proto message of package cosmwasm.wasm.v1; Go type QueryRawContractStateRequest is generated into wasm/x/wasm/types; referenced only through the generated code, so no direct in-repo callers are listed. |
| <code>interface.6.proto-cosmwasm-wasm-v1-queryrawcontractstateresponse-j37</code> | notes | Proto message of package cosmwasm.wasm.v1; Go type QueryRawContractStateResponse is generated into wasm/x/wasm/types; referenced only through the generated code, so no direct in-repo callers are listed. |
| <code>interface.6.proto-cosmwasm-wasm-v1-querysmartcontractstaterequest-j37</code> | notes | Proto message of package cosmwasm.wasm.v1; Go type QuerySmartContractStateRequest is generated into wasm/x/wasm/types; referenced only through the generated code, so no direct in-repo callers are listed. |
| <code>interface.6.proto-cosmwasm-wasm-v1-querysmartcontractstateresponse-j37</code> | notes | Proto message of package cosmwasm.wasm.v1; Go type QuerySmartContractStateResponse is generated into wasm/x/wasm/types; referenced only through the generated code, so no direct in-repo callers are listed. |
| <code>interface.6.proto-cosmwasm-wasm-v1-sequence-j37</code> | notes | Proto message of package cosmwasm.wasm.v1; Go type Sequence is generated into wasm/x/wasm/types; referenced only through the generated code, so no direct in-repo callers are listed. |
| <code>interface.6.proto-cosmwasm-wasm-v1-storecodeproposal-j37</code> | notes | Proto message of package cosmwasm.wasm.v1; Go type StoreCodeProposal is generated into wasm/x/wasm/types; referenced only through the generated code, so no direct in-repo callers are listed. |
| <code>interface.6.proto-cosmwasm-wasm-v1-sudocontractproposal-j37</code> | notes | Proto message of package cosmwasm.wasm.v1; Go type SudoContractProposal is generated into wasm/x/wasm/types; referenced only through the generated code, so no direct in-repo callers are listed. |
| <code>interface.6.proto-cosmwasm-wasm-v1-unpincodesproposal-j37</code> | notes | Proto message of package cosmwasm.wasm.v1; Go type UnpinCodesProposal is generated into wasm/x/wasm/types; referenced only through the generated code, so no direct in-repo callers are listed. |
| <code>interface.6.proto-cosmwasm-wasm-v1-updateadminproposal-j37</code> | notes | Proto message of package cosmwasm.wasm.v1; Go type UpdateAdminProposal is generated into wasm/x/wasm/types; referenced only through the generated code, so no direct in-repo callers are listed. |
| <code>interface.6.proto-cosmwasm-wasm-v1-updateinstantiateconfigproposal-j37</code> | notes | Proto message of package cosmwasm.wasm.v1; Go type UpdateInstantiateConfigProposal is generated into wasm/x/wasm/types; referenced only through the generated code, so no direct in-repo callers are listed. |

## queries

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.6.qry-wasm-allcontractstate</code> | <code>grpc_query</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/querier.go:114</code> | <code>rpc cosmwasm.wasm.v1.Query/AllContractState(QueryAllContractStateRequest) returns (QueryAllContractStateResponse); GET /cosmwasm/wasm/v1/contract/{address}/state</code> |
| <code>interface.6.qry-wasm-code</code> | <code>grpc_query</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/querier.go:212</code> | <code>rpc cosmwasm.wasm.v1.Query/Code(QueryCodeRequest) returns (QueryCodeResponse); GET /cosmwasm/wasm/v1/code/{code_id}</code> |
| <code>interface.6.qry-wasm-codes</code> | <code>grpc_query</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/querier.go:232</code> | <code>rpc cosmwasm.wasm.v1.Query/Codes(QueryCodesRequest) returns (QueryCodesResponse); GET /cosmwasm/wasm/v1/code</code> |
| <code>interface.6.qry-wasm-contracthistory</code> | <code>grpc_query</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/querier.go:53</code> | <code>rpc cosmwasm.wasm.v1.Query/ContractHistory(QueryContractHistoryRequest) returns (QueryContractHistoryResponse); GET /cosmwasm/wasm/v1/contract/{address}/history</code> |
| <code>interface.6.qry-wasm-contractinfo</code> | <code>grpc_query</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/querier.go:35</code> | <code>rpc cosmwasm.wasm.v1.Query/ContractInfo(QueryContractInfoRequest) returns (QueryContractInfoResponse); GET /cosmwasm/wasm/v1/contract/{address}</code> |
| <code>interface.6.qry-wasm-contractsbycode</code> | <code>grpc_query</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/querier.go:87</code> | <code>rpc cosmwasm.wasm.v1.Query/ContractsByCode(QueryContractsByCodeRequest) returns (QueryContractsByCodeResponse); GET /cosmwasm/wasm/v1/code/{code_id}/contracts</code> |
| <code>interface.6.qry-wasm-pinnedcodes</code> | <code>grpc_query</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/querier.go:297</code> | <code>rpc cosmwasm.wasm.v1.Query/PinnedCodes(QueryPinnedCodesRequest) returns (QueryPinnedCodesResponse); GET /cosmwasm/wasm/v1/codes/pinned</code> |
| <code>interface.6.qry-wasm-rawcontractstate</code> | <code>grpc_query</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/querier.go:147</code> | <code>rpc cosmwasm.wasm.v1.Query/RawContractState(QueryRawContractStateRequest) returns (QueryRawContractStateResponse); GET /cosmwasm/wasm/v1/contract/{address}/raw/{query_data}</code> |
| <code>interface.6.qry-wasm-smartcontractstate</code> | <code>grpc_query</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/querier.go:165</code> | <code>rpc cosmwasm.wasm.v1.Query/SmartContractState(QuerySmartContractStateRequest) returns (QuerySmartContractStateResponse); GET /cosmwasm/wasm/v1/contract/{address}/smart/{query_data}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.6.qry-wasm-allcontractstate</code> | notes | 6 in-repo callers (5 non-test, 1 test). |
| <code>interface.6.qry-wasm-code</code> | notes | 6 in-repo callers (5 non-test, 1 test). |
| <code>interface.6.qry-wasm-codes</code> | notes | 7 in-repo callers (5 non-test, 2 test). |
| <code>interface.6.qry-wasm-contracthistory</code> | notes | 6 in-repo callers (5 non-test, 1 test). |
| <code>interface.6.qry-wasm-contractinfo</code> | notes | 6 in-repo callers (5 non-test, 1 test). |
| <code>interface.6.qry-wasm-contractsbycode</code> | notes | 6 in-repo callers (5 non-test, 1 test). |
| <code>interface.6.qry-wasm-pinnedcodes</code> | notes | 6 in-repo callers (5 non-test, 1 test). |
| <code>interface.6.qry-wasm-rawcontractstate</code> | notes | 6 in-repo callers (5 non-test, 1 test). |
| <code>interface.6.qry-wasm-smartcontractstate</code> | notes | 7 in-repo callers (5 non-test, 2 test). |

## rest

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.6.rest-wasm-allcontractstate</code> | <code>rest_route</code> | <code>stable</code> | <code>wasm/proto/cosmwasm/wasm/v1/query.proto:29</code> | <code>GET /cosmwasm/wasm/v1/contract/{address}/state -&gt; cosmwasm.wasm.v1.Query/AllContractState</code> |
| <code>interface.6.rest-wasm-code</code> | <code>rest_route</code> | <code>stable</code> | <code>wasm/proto/cosmwasm/wasm/v1/query.proto:41</code> | <code>GET /cosmwasm/wasm/v1/code/{code_id} -&gt; cosmwasm.wasm.v1.Query/Code</code> |
| <code>interface.6.rest-wasm-codes</code> | <code>rest_route</code> | <code>stable</code> | <code>wasm/proto/cosmwasm/wasm/v1/query.proto:45</code> | <code>GET /cosmwasm/wasm/v1/code -&gt; cosmwasm.wasm.v1.Query/Codes</code> |
| <code>interface.6.rest-wasm-contracthistory</code> | <code>rest_route</code> | <code>stable</code> | <code>wasm/proto/cosmwasm/wasm/v1/query.proto:21</code> | <code>GET /cosmwasm/wasm/v1/contract/{address}/history -&gt; cosmwasm.wasm.v1.Query/ContractHistory</code> |
| <code>interface.6.rest-wasm-contractinfo</code> | <code>rest_route</code> | <code>stable</code> | <code>wasm/proto/cosmwasm/wasm/v1/query.proto:17</code> | <code>GET /cosmwasm/wasm/v1/contract/{address} -&gt; cosmwasm.wasm.v1.Query/ContractInfo</code> |
| <code>interface.6.rest-wasm-contractsbycode</code> | <code>rest_route</code> | <code>stable</code> | <code>wasm/proto/cosmwasm/wasm/v1/query.proto:25</code> | <code>GET /cosmwasm/wasm/v1/code/{code_id}/contracts -&gt; cosmwasm.wasm.v1.Query/ContractsByCode</code> |
| <code>interface.6.rest-wasm-legacy-get-wasm-code</code> | <code>rest_route</code> | <code>stable</code> | <code>wasm/x/wasm/client/rest/query.go:21</code> | <code>GET /wasm/code -&gt; listCodesHandlerFn (legacy amino REST)</code> |
| <code>interface.6.rest-wasm-legacy-get-wasm-code-x</code> | <code>rest_route</code> | <code>stable</code> | <code>wasm/x/wasm/client/rest/query.go:22</code> | <code>GET /wasm/code/{codeID} -&gt; queryCodeHandlerFn (legacy amino REST)</code> |
| <code>interface.6.rest-wasm-legacy-get-wasm-code-x-contracts</code> | <code>rest_route</code> | <code>stable</code> | <code>wasm/x/wasm/client/rest/query.go:23</code> | <code>GET /wasm/code/{codeID}/contracts -&gt; listContractsByCodeHandlerFn (legacy amino REST)</code> |
| <code>interface.6.rest-wasm-legacy-get-wasm-contract-x</code> | <code>rest_route</code> | <code>stable</code> | <code>wasm/x/wasm/client/rest/query.go:24</code> | <code>GET /wasm/contract/{contractAddr} -&gt; queryContractHandlerFn (legacy amino REST)</code> |
| <code>interface.6.rest-wasm-legacy-get-wasm-contract-x-history</code> | <code>rest_route</code> | <code>stable</code> | <code>wasm/x/wasm/client/rest/query.go:26</code> | <code>GET /wasm/contract/{contractAddr}/history -&gt; queryContractHistoryFn (legacy amino REST)</code> |
| <code>interface.6.rest-wasm-legacy-get-wasm-contract-x-raw-x</code> | <code>rest_route</code> | <code>stable</code> | <code>wasm/x/wasm/client/rest/query.go:28</code> | <code>GET /wasm/contract/{contractAddr}/raw/{key}?encoding={encoding} -&gt; queryContractStateRawHandlerFn (legacy amino REST)</code> |
| <code>interface.6.rest-wasm-legacy-get-wasm-contract-x-smart-x</code> | <code>rest_route</code> | <code>stable</code> | <code>wasm/x/wasm/client/rest/query.go:27</code> | <code>GET /wasm/contract/{contractAddr}/smart/{query}?encoding={encoding} -&gt; queryContractStateSmartHandlerFn (legacy amino REST)</code> |
| <code>interface.6.rest-wasm-legacy-get-wasm-contract-x-state</code> | <code>rest_route</code> | <code>stable</code> | <code>wasm/x/wasm/client/rest/query.go:25</code> | <code>GET /wasm/contract/{contractAddr}/state -&gt; queryContractStateAllHandlerFn (legacy amino REST)</code> |
| <code>interface.6.rest-wasm-legacy-post-wasm-code</code> | <code>rest_route</code> | <code>stable</code> | <code>wasm/x/wasm/client/rest/tx.go:18</code> | <code>POST /wasm/code -&gt; storeCodeHandlerFn (legacy amino REST)</code> |
| <code>interface.6.rest-wasm-legacy-post-wasm-code-x</code> | <code>rest_route</code> | <code>stable</code> | <code>wasm/x/wasm/client/rest/tx.go:19</code> | <code>POST /wasm/code/{codeId} -&gt; instantiateContractHandlerFn (legacy amino REST)</code> |
| <code>interface.6.rest-wasm-legacy-post-wasm-contract-x</code> | <code>rest_route</code> | <code>stable</code> | <code>wasm/x/wasm/client/rest/tx.go:20</code> | <code>POST /wasm/contract/{contractAddr} -&gt; executeContractHandlerFn (legacy amino REST)</code> |
| <code>interface.6.rest-wasm-legacy-put-wasm-contract-x-admin</code> | <code>rest_route</code> | <code>stable</code> | <code>wasm/x/wasm/client/rest/new_tx.go:15</code> | <code>PUT /wasm/contract/{contractAddr}/admin -&gt; setContractAdminHandlerFn (legacy amino REST)</code> |
| <code>interface.6.rest-wasm-legacy-put-wasm-contract-x-code</code> | <code>rest_route</code> | <code>stable</code> | <code>wasm/x/wasm/client/rest/new_tx.go:16</code> | <code>PUT /wasm/contract/{contractAddr}/code -&gt; migrateContractHandlerFn (legacy amino REST)</code> |
| <code>interface.6.rest-wasm-pinnedcodes</code> | <code>rest_route</code> | <code>stable</code> | <code>wasm/proto/cosmwasm/wasm/v1/query.proto:50</code> | <code>GET /cosmwasm/wasm/v1/codes/pinned -&gt; cosmwasm.wasm.v1.Query/PinnedCodes</code> |
| <code>interface.6.rest-wasm-rawcontractstate</code> | <code>rest_route</code> | <code>stable</code> | <code>wasm/proto/cosmwasm/wasm/v1/query.proto:33</code> | <code>GET /cosmwasm/wasm/v1/contract/{address}/raw/{query_data} -&gt; cosmwasm.wasm.v1.Query/RawContractState</code> |
| <code>interface.6.rest-wasm-smartcontractstate</code> | <code>rest_route</code> | <code>stable</code> | <code>wasm/proto/cosmwasm/wasm/v1/query.proto:37</code> | <code>GET /cosmwasm/wasm/v1/contract/{address}/smart/{query_data} -&gt; cosmwasm.wasm.v1.Query/SmartContractState</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.6.rest-wasm-allcontractstate</code> | notes | grpc-gateway route from the google.api.http annotation. 2 in-repo callers (2 non-test, 0 test). |
| <code>interface.6.rest-wasm-code</code> | notes | grpc-gateway route from the google.api.http annotation. 2 in-repo callers (2 non-test, 0 test). |
| <code>interface.6.rest-wasm-codes</code> | notes | grpc-gateway route from the google.api.http annotation. 2 in-repo callers (2 non-test, 0 test). |
| <code>interface.6.rest-wasm-contracthistory</code> | notes | grpc-gateway route from the google.api.http annotation. 2 in-repo callers (2 non-test, 0 test). |
| <code>interface.6.rest-wasm-contractinfo</code> | notes | grpc-gateway route from the google.api.http annotation. 2 in-repo callers (2 non-test, 0 test). |
| <code>interface.6.rest-wasm-contractsbycode</code> | notes | grpc-gateway route from the google.api.http annotation. 2 in-repo callers (2 non-test, 0 test). |
| <code>interface.6.rest-wasm-legacy-get-wasm-code</code> | notes | Registered through the wasm module RegisterRESTRoutes (wasm/x/wasm/module.go:104); 7 in-repo callers (7 non-test, 0 test). |
| <code>interface.6.rest-wasm-legacy-get-wasm-code-x</code> | notes | Registered through the wasm module RegisterRESTRoutes (wasm/x/wasm/module.go:104); 7 in-repo callers (7 non-test, 0 test). |
| <code>interface.6.rest-wasm-legacy-get-wasm-code-x-contracts</code> | notes | Registered through the wasm module RegisterRESTRoutes (wasm/x/wasm/module.go:104); 7 in-repo callers (7 non-test, 0 test). |
| <code>interface.6.rest-wasm-legacy-get-wasm-contract-x</code> | notes | Registered through the wasm module RegisterRESTRoutes (wasm/x/wasm/module.go:104); 7 in-repo callers (7 non-test, 0 test). |
| <code>interface.6.rest-wasm-legacy-get-wasm-contract-x-history</code> | notes | Registered through the wasm module RegisterRESTRoutes (wasm/x/wasm/module.go:104); 7 in-repo callers (7 non-test, 0 test). |
| <code>interface.6.rest-wasm-legacy-get-wasm-contract-x-raw-x</code> | notes | Registered through the wasm module RegisterRESTRoutes (wasm/x/wasm/module.go:104); 7 in-repo callers (7 non-test, 0 test). |
| <code>interface.6.rest-wasm-legacy-get-wasm-contract-x-smart-x</code> | notes | Registered through the wasm module RegisterRESTRoutes (wasm/x/wasm/module.go:104); 7 in-repo callers (7 non-test, 0 test). |
| <code>interface.6.rest-wasm-legacy-get-wasm-contract-x-state</code> | notes | Registered through the wasm module RegisterRESTRoutes (wasm/x/wasm/module.go:104); 7 in-repo callers (7 non-test, 0 test). |
| <code>interface.6.rest-wasm-legacy-post-wasm-code</code> | notes | Registered through the wasm module RegisterRESTRoutes (wasm/x/wasm/module.go:104); 7 in-repo callers (7 non-test, 0 test). |
| <code>interface.6.rest-wasm-legacy-post-wasm-code-x</code> | notes | Registered through the wasm module RegisterRESTRoutes (wasm/x/wasm/module.go:104); 7 in-repo callers (7 non-test, 0 test). |
| <code>interface.6.rest-wasm-legacy-post-wasm-contract-x</code> | notes | Registered through the wasm module RegisterRESTRoutes (wasm/x/wasm/module.go:104); 7 in-repo callers (7 non-test, 0 test). |
| <code>interface.6.rest-wasm-legacy-put-wasm-contract-x-admin</code> | notes | Registered through the wasm module RegisterRESTRoutes (wasm/x/wasm/module.go:104); 7 in-repo callers (7 non-test, 0 test). |
| <code>interface.6.rest-wasm-legacy-put-wasm-contract-x-code</code> | notes | Registered through the wasm module RegisterRESTRoutes (wasm/x/wasm/module.go:104); 7 in-repo callers (7 non-test, 0 test). |
| <code>interface.6.rest-wasm-pinnedcodes</code> | notes | grpc-gateway route from the google.api.http annotation. 2 in-repo callers (2 non-test, 0 test). |
| <code>interface.6.rest-wasm-rawcontractstate</code> | notes | grpc-gateway route from the google.api.http annotation. 2 in-repo callers (2 non-test, 0 test). |
| <code>interface.6.rest-wasm-smartcontractstate</code> | notes | grpc-gateway route from the google.api.http annotation. 2 in-repo callers (2 non-test, 0 test). |

## standard-messages

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.5.wasm-std-msg-distribution-fund-community-pool</code> | <code>wasm_binding</code> | <code>stable</code> | <code>wasm-runtime/types/msg.go:251</code> | <code>CosmosMsg.distribution.fund_community_pool not encoded: EncodeDistributionMsg default returns ErrUnknownMsg "unknown variant of Distribution"</code> |
| <code>interface.5.wasm-std-msg-wasm-instantiate2</code> | <code>wasm_binding</code> | <code>stable</code> | <code>wasm-runtime/types/msg.go:285</code> | <code>CosmosMsg.wasm.instantiate2 not encoded: EncodeWasmMsg default returns ErrUnknownMsg</code> |
| <code>interface.6.wasm-std-msg-bank-burn</code> | <code>wasm_binding</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/handler_plugin.go:205</code> | <code>CosmosMsg.bank.burn -&gt; bankKeeper.BurnCoins(ctx, wasm module, coins) after a SendCoinsFromAccountToModule</code> |
| <code>interface.6.wasm-std-msg-bank-send</code> | <code>wasm_binding</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/handler_plugin_encoders.go:109</code> | <code>CosmosMsg.bank.send -&gt; banktypes.MsgSend{FromAddress: contract, ToAddress, Amount}; empty amount yields no message</code> |
| <code>interface.6.wasm-std-msg-custom</code> | <code>wasm_binding</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/handler_plugin_encoders.go:91</code> | <code>CosmosMsg.custom -&gt; MessageEncoders.Custom (node CustomEncoder)</code> |
| <code>interface.6.wasm-std-msg-distribution-set-withdraw-address</code> | <code>wasm_binding</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/handler_plugin_encoders.go:134</code> | <code>CosmosMsg.distribution.set_withdraw_address -&gt; distributiontypes.MsgSetWithdrawAddress</code> |
| <code>interface.6.wasm-std-msg-distribution-withdraw-delegator-reward</code> | <code>wasm_binding</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/handler_plugin_encoders.go:140</code> | <code>CosmosMsg.distribution.withdraw_delegator_reward -&gt; distributiontypes.MsgWithdrawDelegatorReward</code> |
| <code>interface.6.wasm-std-msg-gov-vote</code> | <code>wasm_binding</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/handler_plugin_encoders.go:296</code> | <code>CosmosMsg.gov.vote -&gt; govtypes.MsgVote{ProposalId, Voter: contract, Option yes&#124;no&#124;no_with_veto&#124;abstain}</code> |
| <code>interface.6.wasm-std-msg-gov-vote-weighted</code> | <code>wasm_binding</code> | <code>change</code> | <code>wasm/x/wasm/keeper/handler_plugin_encoders.go:298</code> | <code>CosmosMsg.gov.vote_weighted not encoded: EncodeGovMsg dereferences msg.Vote unconditionally</code> |
| <code>interface.6.wasm-std-msg-ibc-close-channel</code> | <code>wasm_binding</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/handler_plugin_encoders.go:269</code> | <code>CosmosMsg.ibc.close_channel -&gt; channeltypes.MsgChannelCloseInit{PortId: contract port, ChannelId}</code> |
| <code>interface.6.wasm-std-msg-ibc-send-packet</code> | <code>wasm_binding</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/handler_plugin.go:154</code> | <code>CosmosMsg.ibc.send_packet -&gt; channelKeeper.SendPacket on the contract port wasm.&lt;contract&gt; with the next send sequence</code> |
| <code>interface.6.wasm-std-msg-ibc-transfer</code> | <code>wasm_binding</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/handler_plugin_encoders.go:275</code> | <code>CosmosMsg.ibc.transfer -&gt; ibctransfertypes.MsgTransfer{SourcePort: ICS20 port, SourceChannel, Token, Sender: contract, Receiver, TimeoutHeight, TimeoutTimestamp}</code> |
| <code>interface.6.wasm-std-msg-staking-delegate</code> | <code>wasm_binding</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/handler_plugin_encoders.go:153</code> | <code>CosmosMsg.staking.delegate -&gt; stakingtypes.MsgDelegate</code> |
| <code>interface.6.wasm-std-msg-staking-redelegate</code> | <code>wasm_binding</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/handler_plugin_encoders.go:165</code> | <code>CosmosMsg.staking.redelegate -&gt; stakingtypes.MsgBeginRedelegate</code> |
| <code>interface.6.wasm-std-msg-staking-undelegate</code> | <code>wasm_binding</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/handler_plugin_encoders.go:177</code> | <code>CosmosMsg.staking.undelegate -&gt; stakingtypes.MsgUndelegate</code> |
| <code>interface.6.wasm-std-msg-stargate</code> | <code>wasm_binding</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/handler_plugin_encoders.go:193</code> | <code>CosmosMsg.stargate {type_url, value} -&gt; codectypes.Any unpacked to any registered sdk.Msg</code> |
| <code>interface.6.wasm-std-msg-wasm-clear-admin</code> | <code>wasm_binding</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/handler_plugin_encoders.go:248</code> | <code>CosmosMsg.wasm.clear_admin -&gt; types.MsgClearAdmin</code> |
| <code>interface.6.wasm-std-msg-wasm-execute</code> | <code>wasm_binding</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/handler_plugin_encoders.go:212</code> | <code>CosmosMsg.wasm.execute -&gt; types.MsgExecuteContract</code> |
| <code>interface.6.wasm-std-msg-wasm-instantiate</code> | <code>wasm_binding</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/handler_plugin_encoders.go:225</code> | <code>CosmosMsg.wasm.instantiate -&gt; types.MsgInstantiateContract</code> |
| <code>interface.6.wasm-std-msg-wasm-migrate</code> | <code>wasm_binding</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/handler_plugin_encoders.go:240</code> | <code>CosmosMsg.wasm.migrate -&gt; types.MsgMigrateContract</code> |
| <code>interface.6.wasm-std-msg-wasm-update-admin</code> | <code>wasm_binding</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/handler_plugin_encoders.go:254</code> | <code>CosmosMsg.wasm.update_admin -&gt; types.MsgUpdateAdmin</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.5.wasm-std-msg-distribution-fund-community-pool</code> | notes | No case in EncodeDistributionMsg (wasm/x/wasm/keeper/handler_plugin_encoders.go:132); requires capability cosmwasm_1_3 which the node does not advertise. JSON field wasm-runtime/types/msg.go:251; 0 in-repo callers (0 non-test, 0 test). |
| <code>interface.5.wasm-std-msg-wasm-instantiate2</code> | notes | No case in EncodeWasmMsg (wasm/x/wasm/keeper/handler_plugin_encoders.go:210); requires capability cosmwasm_1_2 which the node does not advertise. JSON field wasm-runtime/types/msg.go:285; 0 in-repo callers (0 non-test, 0 test). |
| <code>interface.6.wasm-std-msg-bank-burn</code> | notes | JSON field wasm-runtime/types/msg.go:112; 1 in-repo callers (1 non-test, 0 test). |
| <code>interface.6.wasm-std-msg-bank-send</code> | notes | JSON field wasm-runtime/types/msg.go:111; 12 in-repo callers (1 non-test, 11 test). |
| <code>interface.6.wasm-std-msg-custom</code> | notes | Default encoder is NoCustomMsg returning ErrUnknownMsg; the node installs CustomEncoder. JSON field wasm-runtime/types/msg.go:101; 1 in-repo callers (1 non-test, 0 test). |
| <code>interface.6.wasm-std-msg-distribution-set-withdraw-address</code> | notes | JSON field wasm-runtime/types/msg.go:249; 2 in-repo callers (1 non-test, 1 test). |
| <code>interface.6.wasm-std-msg-distribution-withdraw-delegator-reward</code> | notes | JSON field wasm-runtime/types/msg.go:250; 2 in-repo callers (1 non-test, 1 test). |
| <code>interface.6.wasm-std-msg-gov-vote</code> | notes | JSON field wasm-runtime/types/msg.go:137; 5 in-repo callers (1 non-test, 4 test). |
| <code>interface.6.wasm-std-msg-gov-vote-weighted</code> | notes | Mismatch: the binding type declares vote_weighted (wasm-runtime/types/msg.go:139) but EncodeGovMsg reads msg.Vote.Vote at wasm/x/wasm/keeper/handler_plugin_encoders.go:298 without a nil check, so a GovMsg carrying only vote_weighted panics with a nil pointer dereference instead of returning ErrUnknownMsg. JSON field wasm-runtime/types/msg.go:139; 1 in-repo callers (1 non-test, 0 test). |
| <code>interface.6.wasm-std-msg-ibc-close-channel</code> | notes | JSON field wasm-runtime/types/msg.go:132; 2 in-repo callers (1 non-test, 1 test). |
| <code>interface.6.wasm-std-msg-ibc-send-packet</code> | notes | JSON field wasm-runtime/types/msg.go:131; 4 in-repo callers (1 non-test, 3 test). |
| <code>interface.6.wasm-std-msg-ibc-transfer</code> | notes | JSON field wasm-runtime/types/msg.go:130; 5 in-repo callers (1 non-test, 4 test). |
| <code>interface.6.wasm-std-msg-staking-delegate</code> | notes | JSON field wasm-runtime/types/msg.go:227; 3 in-repo callers (1 non-test, 2 test). |
| <code>interface.6.wasm-std-msg-staking-redelegate</code> | notes | JSON field wasm-runtime/types/msg.go:229; 2 in-repo callers (1 non-test, 1 test). |
| <code>interface.6.wasm-std-msg-staking-undelegate</code> | notes | JSON field wasm-runtime/types/msg.go:228; 2 in-repo callers (1 non-test, 1 test). |
| <code>interface.6.wasm-std-msg-stargate</code> | notes | JSON field wasm-runtime/types/msg.go:106; 4 in-repo callers (1 non-test, 3 test). |
| <code>interface.6.wasm-std-msg-wasm-clear-admin</code> | notes | JSON field wasm-runtime/types/msg.go:288; 2 in-repo callers (1 non-test, 1 test). |
| <code>interface.6.wasm-std-msg-wasm-execute</code> | notes | JSON field wasm-runtime/types/msg.go:283; 5 in-repo callers (1 non-test, 4 test). |
| <code>interface.6.wasm-std-msg-wasm-instantiate</code> | notes | JSON field wasm-runtime/types/msg.go:284; 3 in-repo callers (1 non-test, 2 test). |
| <code>interface.6.wasm-std-msg-wasm-migrate</code> | notes | JSON field wasm-runtime/types/msg.go:286; 2 in-repo callers (1 non-test, 1 test). |
| <code>interface.6.wasm-std-msg-wasm-update-admin</code> | notes | JSON field wasm-runtime/types/msg.go:287; 2 in-repo callers (1 non-test, 1 test). |

Contract context <code>contract_context.6.wasm-gov-msg-j37</code>:

| Field | Frozen rule |
|---|---|
| interface | CosmWasm GovMsg encoding through EncodeGovMsg (wasm/x/wasm/keeper/handler_plugin_encoders.go:296) |
| storage | No state |
| errors | ErrUnknownMsg for a gov message without a plain vote or with an unknown vote option |
| events | The gov vote events of the encoded MsgVote |
| gas | Contract execution gas |
| activation | Live at the release |
| invariants | A contract message never panics the encoder |
| pause | No pause |

## standard-queries

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.5.wasm-std-qry-bank-all-denom-metadata</code> | <code>wasm_binding</code> | <code>stable</code> | <code>wasm-runtime/types/queries.go:117</code> | <code>QueryRequest.bank.all_denom_metadata not served: UnsupportedRequest "unknown BankQuery variant"</code> |
| <code>interface.5.wasm-std-qry-bank-denom-metadata</code> | <code>wasm_binding</code> | <code>stable</code> | <code>wasm-runtime/types/queries.go:116</code> | <code>QueryRequest.bank.denom_metadata not served: UnsupportedRequest "unknown BankQuery variant"</code> |
| <code>interface.5.wasm-std-qry-bank-supply</code> | <code>wasm_binding</code> | <code>stable</code> | <code>wasm-runtime/types/queries.go:113</code> | <code>QueryRequest.bank.supply not served: BankQuerier returns UnsupportedRequest "unknown BankQuery variant"</code> |
| <code>interface.5.wasm-std-qry-distribution-delegation-rewards</code> | <code>wasm_binding</code> | <code>stable</code> | <code>wasm-runtime/types/queries.go:372</code> | <code>QueryRequest.distribution.delegation_rewards not served: QueryPlugins.HandleQuery has no Distribution branch and returns UnsupportedRequest</code> |
| <code>interface.5.wasm-std-qry-distribution-delegation-total-rewards</code> | <code>wasm_binding</code> | <code>stable</code> | <code>wasm-runtime/types/queries.go:374</code> | <code>QueryRequest.distribution.delegation_total_rewards not served: QueryPlugins.HandleQuery has no Distribution branch and returns UnsupportedRequest</code> |
| <code>interface.5.wasm-std-qry-distribution-delegator-validators</code> | <code>wasm_binding</code> | <code>stable</code> | <code>wasm-runtime/types/queries.go:376</code> | <code>QueryRequest.distribution.delegator_validators not served: QueryPlugins.HandleQuery has no Distribution branch and returns UnsupportedRequest</code> |
| <code>interface.5.wasm-std-qry-distribution-delegator-withdraw-address</code> | <code>wasm_binding</code> | <code>stable</code> | <code>wasm-runtime/types/queries.go:370</code> | <code>QueryRequest.distribution.delegator_withdraw_address not served: QueryPlugins.HandleQuery has no Distribution branch and returns UnsupportedRequest</code> |
| <code>interface.5.wasm-std-qry-wasm-code-info</code> | <code>wasm_binding</code> | <code>stable</code> | <code>wasm-runtime/types/queries.go:479</code> | <code>QueryRequest.wasm.code_info not served: WasmQuerier returns UnsupportedRequest "unknown WasmQuery variant"</code> |
| <code>interface.6.wasm-std-qry-bank-all-balances</code> | <code>wasm_binding</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/query_plugins.go:169</code> | <code>QueryRequest.bank.all_balances {address} -&gt; AllBalancesResponse{amount}</code> |
| <code>interface.6.wasm-std-qry-bank-balance</code> | <code>wasm_binding</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/query_plugins.go:180</code> | <code>QueryRequest.bank.balance {address, denom} -&gt; BalanceResponse{amount}</code> |
| <code>interface.6.wasm-std-qry-custom</code> | <code>wasm_binding</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/query_plugins.go:149</code> | <code>QueryRequest.custom -&gt; QueryPlugins.Custom (node CustomQuerier)</code> |
| <code>interface.6.wasm-std-qry-ibc-channel</code> | <code>wasm_binding</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/query_plugins.go:239</code> | <code>QueryRequest.ibc.channel {port_id?, channel_id} -&gt; ChannelResponse{channel?} (OPEN only)</code> |
| <code>interface.6.wasm-std-qry-ibc-list-channels</code> | <code>wasm_binding</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/query_plugins.go:211</code> | <code>QueryRequest.ibc.list_channels {port_id?} -&gt; ListChannelsResponse{channels} (OPEN channels of the port)</code> |
| <code>interface.6.wasm-std-qry-ibc-port-id</code> | <code>wasm_binding</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/query_plugins.go:204</code> | <code>QueryRequest.ibc.port_id {} -&gt; PortIDResponse{port_id: contract IBC port}</code> |
| <code>interface.6.wasm-std-qry-staking-all-delegations</code> | <code>wasm_binding</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/query_plugins.go:322</code> | <code>QueryRequest.staking.all_delegations {delegator} -&gt; AllDelegationsResponse{delegations}</code> |
| <code>interface.6.wasm-std-qry-staking-all-validators</code> | <code>wasm_binding</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/query_plugins.go:288</code> | <code>QueryRequest.staking.all_validators {} -&gt; AllValidatorsResponse{validators} (bonded only)</code> |
| <code>interface.6.wasm-std-qry-staking-bonded-denom</code> | <code>wasm_binding</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/query_plugins.go:281</code> | <code>QueryRequest.staking.bonded_denom {} -&gt; BondedDenomResponse{denom}</code> |
| <code>interface.6.wasm-std-qry-staking-delegation</code> | <code>wasm_binding</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/query_plugins.go:337</code> | <code>QueryRequest.staking.delegation {delegator, validator} -&gt; DelegationResponse{delegation?}</code> |
| <code>interface.6.wasm-std-qry-staking-unbonding-delegation</code> | <code>wasm_binding</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/query_plugins.go:357</code> | <code>QueryRequest.staking.unbonding_delegation {delegator, validator} -&gt; UnbondingDelegationResponse (chain-specific variant)</code> |
| <code>interface.6.wasm-std-qry-staking-unbonding-delegations</code> | <code>wasm_binding</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/query_plugins.go:383</code> | <code>QueryRequest.staking.unbonding_delegations {delegator} -&gt; UnbondingDelegationsResponse (chain-specific variant)</code> |
| <code>interface.6.wasm-std-qry-staking-validator</code> | <code>wasm_binding</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/query_plugins.go:305</code> | <code>QueryRequest.staking.validator {address} -&gt; ValidatorResponse{validator?}</code> |
| <code>interface.6.wasm-std-qry-stargate</code> | <code>wasm_binding</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/query_plugins.go:273</code> | <code>QueryRequest.stargate always UnsupportedRequest{Kind: "Stargate queries are disabled."}</code> |
| <code>interface.6.wasm-std-qry-wasm-contract-info</code> | <code>wasm_binding</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/query_plugins.go:528</code> | <code>QueryRequest.wasm.contract_info {contract_addr} -&gt; ContractInfoResponse{code_id, creator, admin, pinned, ibc_port}</code> |
| <code>interface.6.wasm-std-qry-wasm-raw</code> | <code>wasm_binding</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/query_plugins.go:522</code> | <code>QueryRequest.wasm.raw {contract_addr, key} -&gt; raw store value</code> |
| <code>interface.6.wasm-std-qry-wasm-smart</code> | <code>wasm_binding</code> | <code>stable</code> | <code>wasm/x/wasm/keeper/query_plugins.go:512</code> | <code>QueryRequest.wasm.smart {contract_addr, msg} -&gt; contract query result</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.5.wasm-std-qry-bank-all-denom-metadata</code> | notes | No branch in BankQuerier (wasm/x/wasm/keeper/query_plugins.go:167); requires capability cosmwasm_1_3 which the node does not advertise. JSON field wasm-runtime/types/queries.go:117; 0 in-repo callers (0 non-test, 0 test). |
| <code>interface.5.wasm-std-qry-bank-denom-metadata</code> | notes | No branch in BankQuerier (wasm/x/wasm/keeper/query_plugins.go:167); requires capability cosmwasm_1_3 which the node does not advertise. JSON field wasm-runtime/types/queries.go:116; 0 in-repo callers (0 non-test, 0 test). |
| <code>interface.5.wasm-std-qry-bank-supply</code> | notes | No branch in BankQuerier (wasm/x/wasm/keeper/query_plugins.go:167); requires capability cosmwasm_1_1 which the node does not advertise. JSON field wasm-runtime/types/queries.go:113; 0 in-repo callers (0 non-test, 0 test). |
| <code>interface.5.wasm-std-qry-distribution-delegation-rewards</code> | notes | QueryRequest.distribution (wasm-runtime/types/queries.go:107) is not dispatched by HandleQuery (wasm/x/wasm/keeper/query_plugins.go:144); requires capability cosmwasm_1_3 which the node does not advertise. JSON field wasm-runtime/types/queries.go:372; 0 in-repo callers (0 non-test, 0 test). |
| <code>interface.5.wasm-std-qry-distribution-delegation-total-rewards</code> | notes | QueryRequest.distribution (wasm-runtime/types/queries.go:107) is not dispatched by HandleQuery (wasm/x/wasm/keeper/query_plugins.go:144); requires capability cosmwasm_1_3 which the node does not advertise. JSON field wasm-runtime/types/queries.go:374; 0 in-repo callers (0 non-test, 0 test). |
| <code>interface.5.wasm-std-qry-distribution-delegator-validators</code> | notes | QueryRequest.distribution (wasm-runtime/types/queries.go:107) is not dispatched by HandleQuery (wasm/x/wasm/keeper/query_plugins.go:144); requires capability cosmwasm_1_3 which the node does not advertise. JSON field wasm-runtime/types/queries.go:376; 0 in-repo callers (0 non-test, 0 test). |
| <code>interface.5.wasm-std-qry-distribution-delegator-withdraw-address</code> | notes | QueryRequest.distribution (wasm-runtime/types/queries.go:107) is not dispatched by HandleQuery (wasm/x/wasm/keeper/query_plugins.go:144); requires capability cosmwasm_1_3 which the node does not advertise. JSON field wasm-runtime/types/queries.go:370; 0 in-repo callers (0 non-test, 0 test). |
| <code>interface.5.wasm-std-qry-wasm-code-info</code> | notes | No case in WasmQuerier (wasm/x/wasm/keeper/query_plugins.go:509); requires capability cosmwasm_1_2 which the node does not advertise. JSON field wasm-runtime/types/queries.go:479; 0 in-repo callers (0 non-test, 0 test). |
| <code>interface.6.wasm-std-qry-bank-all-balances</code> | notes | JSON field wasm-runtime/types/queries.go:115; 2 in-repo callers (1 non-test, 1 test). |
| <code>interface.6.wasm-std-qry-bank-balance</code> | notes | JSON field wasm-runtime/types/queries.go:114; 3 in-repo callers (1 non-test, 2 test). |
| <code>interface.6.wasm-std-qry-custom</code> | notes | Default is NoCustomQuerier returning UnsupportedRequest{Kind: "custom"}; the node installs CustomQuerier. JSON field wasm-runtime/types/queries.go:104; 1 in-repo callers (1 non-test, 0 test). |
| <code>interface.6.wasm-std-qry-ibc-channel</code> | notes | JSON field wasm-runtime/types/queries.go:175; 6 in-repo callers (1 non-test, 5 test). |
| <code>interface.6.wasm-std-qry-ibc-list-channels</code> | notes | JSON field wasm-runtime/types/queries.go:174; 4 in-repo callers (1 non-test, 3 test). |
| <code>interface.6.wasm-std-qry-ibc-port-id</code> | notes | JSON field wasm-runtime/types/queries.go:173; 2 in-repo callers (1 non-test, 1 test). |
| <code>interface.6.wasm-std-qry-staking-all-delegations</code> | notes | JSON field wasm-runtime/types/queries.go:263; 2 in-repo callers (1 non-test, 1 test). |
| <code>interface.6.wasm-std-qry-staking-all-validators</code> | notes | JSON field wasm-runtime/types/queries.go:261; 2 in-repo callers (1 non-test, 1 test). |
| <code>interface.6.wasm-std-qry-staking-bonded-denom</code> | notes | JSON field wasm-runtime/types/queries.go:265; 1 in-repo callers (1 non-test, 0 test). |
| <code>interface.6.wasm-std-qry-staking-delegation</code> | notes | JSON field wasm-runtime/types/queries.go:264; 3 in-repo callers (1 non-test, 2 test). |
| <code>interface.6.wasm-std-qry-staking-unbonding-delegation</code> | notes | JSON field wasm-runtime/types/queries.go:266; 1 in-repo callers (1 non-test, 0 test). |
| <code>interface.6.wasm-std-qry-staking-unbonding-delegations</code> | notes | JSON field wasm-runtime/types/queries.go:267; 1 in-repo callers (1 non-test, 0 test). |
| <code>interface.6.wasm-std-qry-staking-validator</code> | notes | JSON field wasm-runtime/types/queries.go:262; 3 in-repo callers (1 non-test, 2 test). |
| <code>interface.6.wasm-std-qry-stargate</code> | notes | Disabled at wasm/x/wasm/keeper/query_plugins.go:275 while the stargate capability stays advertised for Stargate messages (node/app.go:772). JSON field wasm-runtime/types/queries.go:108; 4 in-repo callers (1 non-test, 3 test). |
| <code>interface.6.wasm-std-qry-wasm-contract-info</code> | notes | JSON field wasm-runtime/types/queries.go:478; 6 in-repo callers (1 non-test, 5 test). |
| <code>interface.6.wasm-std-qry-wasm-raw</code> | notes | JSON field wasm-runtime/types/queries.go:477; 3 in-repo callers (1 non-test, 2 test). |
| <code>interface.6.wasm-std-qry-wasm-smart</code> | notes | JSON field wasm-runtime/types/queries.go:476; 2 in-repo callers (1 non-test, 1 test). |
