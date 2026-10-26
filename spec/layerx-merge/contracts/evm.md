# Evm contract

Source revision: `2a1033aa2`. This contract freezes the cited interface and acceptance blocks of the specification. Statuses include the specification overrides. A source marked planned has line 0; a positive source line is verified in the pinned revision.

The context and decision tables reproduce the rules linked by interface identifiers. An anchor without a linked context gains no additional storage, gas, activation or pause rule here. Acceptance qualification remains UNRUN until its declared gate succeeds.

## activation-list

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.6.go-evm-keeper-activationaddresseswithoutprecompile-j34</code> | <code>go_exported</code> | <code>new</code> | <code>modules/evm/keeper/keeper.go</code> (planned) | <code>func (k *Keeper) ActivationAddressesWithoutPrecompile() []common.Address</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.6.go-evm-keeper-activationaddresseswithoutprecompile-j34</code> | notes | sorted addresses named by a recorded CustomPrecompileActivation (modules/evm/keeper/keeper.go:207) that have no entry in the custom precompile set (task 6.7.3) |

Contract context <code>contract_context.6.evm-activation</code>:

| Field | Frozen rule |
|---|---|
| interface | modules/evm/keeper/keeper.go: SetCustomPrecompileActivation, CustomPrecompiles, ActivationAddressesWithoutPrecompile |
| storage | In-memory activation records set at application construction |
| errors | SetCustomPrecompileActivation panics on an incomplete record |
| events | none |
| gas | none |
| activation | Each entry serves its addresses from its own applied plan |
| invariants | No activation address lacks a registered precompile at node start |
| pause | none |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.6.28.1</code> | req | 6028.1 |
| <code>acceptance.6.28.1</code> | scenario | Given an activation naming 0x0901 and 0x101A and a custom precompile set holding only 0x0901, when ActivationAddressesWithoutPrecompile is called, then it returns [0x101A] |
| <code>acceptance.6.28.1</code> | test | modules/evm/keeper/keeper_activation_audit_test.go::TestActivationAddressesWithoutPrecompile (planned) |
| <code>acceptance.6.28.1</code> | qualification | UNRUN |
| <code>req.6028</code> | ac_1 | ActivationAddressesWithoutPrecompile lists exactly the activation addresses missing from the custom precompile set, sorted (modules/evm/keeper/keeper.go:207, :172). |
| <code>capability.6.7.3</code> | title | Activation list audit |
| <code>capability.6.7.3</code> | reqs | 6028.1 |

## cli-surface

No interface block names this anchor; the citing criteria and rules below define its scope.

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.6.37.1</code> | req | 6037.1 |
| <code>acceptance.6.37.1</code> | scenario | Given GetTxCmd, when each sub-command Use string is scanned for --name tokens, then each name resolves through cmd.Flags().Lookup |
| <code>acceptance.6.37.1</code> | test | modules/evm/client/cli/usage_flags_test.go::TestEVMTxUsageNamesRegisteredFlags (planned) |
| <code>acceptance.6.37.1</code> | qualification | UNRUN |
| <code>req.6037</code> | ac_1 | Every --flag named in an evm tx Use string is registered on that command, which --gas-limt (modules/evm/client/cli/tx.go:231) and --rpc (modules/evm/client/cli/tx.go:81) violate today. |
| <code>acceptance.6.57.1</code> | req | 6057.1 |
| <code>acceptance.6.57.1</code> | scenario | Given GetTxCmd, when its sub-commands are listed, then the pointer-binding proposal command is present |
| <code>acceptance.6.57.1</code> | test | modules/evm/client/cli/tx_test.go::TestBindERCNativePointerProposalCommandIsMounted (existing) |
| <code>acceptance.6.57.1</code> | qualification | UNRUN |
| <code>req.6057</code> | ac_1 | The bind-erc-native-pointer proposal command stays mounted under the evm tx command (modules/evm/client/cli/tx.go:49). |
| <code>acceptance.6.57.2</code> | req | 6057.2 |
| <code>acceptance.6.57.2</code> | scenario | Given an RPC endpoint serving eth_chainId, when getChainId runs, then it returns that id |
| <code>acceptance.6.57.2</code> | test | modules/evm/client/cli/tx_test.go::TestGetChainId (existing) |
| <code>acceptance.6.57.2</code> | qualification | UNRUN |
| <code>req.6057</code> | ac_2 | The CLI reads the chain id from the evm-rpc endpoint (modules/evm/client/cli/tx.go:44). |
| <code>acceptance.6.57.3</code> | req | 6057.3 |
| <code>acceptance.6.57.3</code> | scenario | Given an RPC endpoint serving eth_getTransactionCount, when getNonce runs, then it returns that nonce |
| <code>acceptance.6.57.3</code> | test | modules/evm/client/cli/tx_test.go::TestGetNonce (existing) |
| <code>acceptance.6.57.3</code> | qualification | UNRUN |
| <code>req.6057</code> | ac_3 | The CLI reads the pending nonce from the evm-rpc endpoint (modules/evm/client/cli/tx.go:44). |
| <code>acceptance.6.7.1</code> | req | 6007.1 |
| <code>acceptance.6.7.1</code> | scenario | Given the surface at modules/evm/client/cli/tx.go:49 and modules/evm/client/cli/query.go:38, when TestEVMCLISurfaceFrozen runs, then every entry equals the frozen table |
| <code>acceptance.6.7.1</code> | test | modules/evm/client/cli/cli_surface_test.go::TestEVMCLISurfaceFrozen (planned) |
| <code>acceptance.6.7.1</code> | qualification | UNRUN |
| <code>req.6007</code> | ac_1 | The evm tx and query command trees keep their command names and flag names (modules/evm/client/cli/tx.go:49, modules/evm/client/cli/query.go:38). |
| <code>acceptance.6.999.1</code> | req | 6999.1 |
| <code>acceptance.6.999.1</code> | scenario | Given the candidate after every domain-6 wave-6 task, when TestChainModulesCompletionRetiredSurfacesAbsent runs, then no retired surface remains and every planned path exists |
| <code>acceptance.6.999.1</code> | test | modules/evm/client/cli/completion_gate_test.go::TestChainModulesCompletionRetiredSurfacesAbsent (planned) |
| <code>acceptance.6.999.1</code> | qualification | UNRUN |
| <code>req.6999</code> | ac_1 | TestChainModulesCompletionRetiredSurfacesAbsent passes on the merged candidate: no retired domain-6 surface remains and every planned domain-6 wave-6 path exists. |
| <code>capability.6.0.9</code> | title | Domain 6 completion gate |
| <code>capability.6.0.9</code> | reqs | 6999.1 |
| <code>capability.6.11.4</code> | title | evm tx and query CLI command tree and flags freeze |
| <code>capability.6.11.4</code> | reqs | 6007.1 |
| <code>capability.6.20.4</code> | title | Domain-6 client anchor |
| <code>capability.6.20.4</code> | reqs | 6057.1<br>6057.2<br>6057.3 |
| <code>capability.6.8.3</code> | title | CLI usage strings |
| <code>capability.6.8.3</code> | reqs | 6037.1 |

## events

No interface block names this anchor; the citing criteria and rules below define its scope.

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.6.5.1</code> | req | 6005.1 |
| <code>acceptance.6.5.1</code> | scenario | Given the surface at modules/evm/types/events.go:4-17, when TestEVMEventKeysFrozen runs, then every entry equals the frozen table |
| <code>acceptance.6.5.1</code> | test | modules/evm/types/events_freeze_test.go::TestEVMEventKeysFrozen (planned) |
| <code>acceptance.6.5.1</code> | qualification | UNRUN |
| <code>req.6005</code> | ac_1 | The evm event types and attribute keys keep their strings (modules/evm/types/events.go:4-17). |
| <code>capability.6.11.2</code> | title | evm event types and attribute keys freeze |
| <code>capability.6.11.2</code> | reqs | 6005.1 |

## go-api

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.6.go-evm-ante-basicdecorator-antehandle-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/ante/basic.go:25</code> | <code>func (gl BasicDecorator) AnteHandle(ctx sdk.Context, tx sdk.Tx, simulate bool, next sdk.AnteHandler) (sdk.Context, error)</code> |
| <code>interface.6.go-evm-ante-evmaddressdecorator-antehandle-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/ante/preprocess.go:267</code> | <code>func (p *EVMAddressDecorator) AnteHandle(ctx sdk.Context, tx sdk.Tx, simulate bool, next sdk.AnteHandler) (sdk.Context, error)</code> |
| <code>interface.6.go-evm-ante-evmfeecheckdecorator-antehandle-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/ante/fee.go:36</code> | <code>func (fc EVMFeeCheckDecorator) AnteHandle(ctx sdk.Context, tx sdk.Tx, simulate bool, next sdk.AnteHandler) (sdk.Context, error)</code> |
| <code>interface.6.go-evm-ante-evmnocosmosfieldsdecorator-antehandle-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/ante/no_cosmos_fields.go:17</code> | <code>func (d EVMNoCosmosFieldsDecorator) AnteHandle(ctx sdk.Context, tx sdk.Tx, simulate bool, next sdk.AnteHandler) (sdk.Context, error)</code> |
| <code>interface.6.go-evm-ante-evmpreprocessdecorator-antehandle-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/ante/preprocess.go:59</code> | <code>func (p *EVMPreprocessDecorator) AnteHandle(ctx sdk.Context, tx sdk.Tx, simulate bool, next sdk.AnteHandler) (sdk.Context, error)</code> |
| <code>interface.6.go-evm-ante-evmrouterdecorator-antehandle-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/ante/router.go:25</code> | <code>func (r EVMRouterDecorator) AnteHandle(ctx sdk.Context, tx sdk.Tx, simulate bool) (sdk.Context, error)</code> |
| <code>interface.6.go-evm-ante-evmsigverifydecorator-antehandle-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/ante/sig.go:31</code> | <code>func (svd *EVMSigVerifyDecorator) AnteHandle(ctx sdk.Context, tx sdk.Tx, simulate bool, next sdk.AnteHandler) (sdk.Context, error)</code> |
| <code>interface.6.go-evm-ante-gasdecorator-antehandle-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/ante/gas.go:27</code> | <code>func (gl GasDecorator) AnteHandle(ctx sdk.Context, tx sdk.Tx, simulate bool, next sdk.AnteHandler) (sdk.Context, error)</code> |
| <code>interface.6.go-evm-ante-getversion-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/ante/preprocess.go:244</code> | <code>func GetVersion(ctx sdk.Context, ethCfg *params.ChainConfig) derived.SignerVersion</code> |
| <code>interface.6.go-evm-ante-isevmmessage-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/ante/router.go:35</code> | <code>func IsEVMMessage(tx sdk.Tx) (bool, error)</code> |
| <code>interface.6.go-evm-ante-istxtypeallowed-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/ante/preprocess.go:211</code> | <code>func IsTxTypeAllowed(version derived.SignerVersion, txType uint8) bool</code> |
| <code>interface.6.go-evm-ante-mingasevmtx-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/ante/gas.go:15</code> | <code>MinGasEVMTx = 21000</code> |
| <code>interface.6.go-evm-ante-newbasicdecorator-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/ante/basic.go:20</code> | <code>func NewBasicDecorator(k *keeper.Keeper) *BasicDecorator</code> |
| <code>interface.6.go-evm-ante-newevmaddressdecorator-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/ante/preprocess.go:262</code> | <code>func NewEVMAddressDecorator(evmKeeper *evmkeeper.Keeper, accountKeeper *accountkeeper.AccountKeeper) *EVMAddressDecorator</code> |
| <code>interface.6.go-evm-ante-newevmfeecheckdecorator-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/ante/fee.go:29</code> | <code>func NewEVMFeeCheckDecorator(evmKeeper *evmkeeper.Keeper, upgradeKeeper *upgradekeeper.Keeper) *EVMFeeCheckDecorator</code> |
| <code>interface.6.go-evm-ante-newevmnocosmosfieldsdecorator-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/ante/no_cosmos_fields.go:13</code> | <code>func NewEVMNoCosmosFieldsDecorator() EVMNoCosmosFieldsDecorator</code> |
| <code>interface.6.go-evm-ante-newevmpreprocessdecorator-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/ante/preprocess.go:54</code> | <code>func NewEVMPreprocessDecorator(evmKeeper *evmkeeper.Keeper, accountKeeper *accountkeeper.AccountKeeper) *EVMPreprocessDecorator</code> |
| <code>interface.6.go-evm-ante-newevmrouterdecorator-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/ante/router.go:15</code> | <code>func NewEVMRouterDecorator(</code> |
| <code>interface.6.go-evm-ante-newevmsigverifydecorator-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/ante/sig.go:25</code> | <code>func NewEVMSigVerifyDecorator(evmKeeper *evmkeeper.Keeper, _ func() sdk.Context) *EVMSigVerifyDecorator</code> |
| <code>interface.6.go-evm-ante-newgasdecorator-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/ante/gas.go:22</code> | <code>func NewGasDecorator(evmKeeper *evmkeeper.Keeper) *GasDecorator</code> |
| <code>interface.6.go-evm-ante-preprocess-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/ante/preprocess.go:123</code> | <code>func Preprocess(ctx sdk.Context, msgEVMTransaction *evmtypes.MsgEVMTransaction, chainID *big.Int, isBlockTest bool) error</code> |
| <code>interface.6.go-evm-ante-preprocessunpacked-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/ante/preprocess.go:129</code> | <code>func PreprocessUnpacked(ctx sdk.Context, msgEVMTransaction *evmtypes.MsgEVMTransaction, chainID *big.Int, isBlockTest bool, txData ethtx.TxData) error</code> |
| <code>interface.6.go-evm-ante-recoversenderfromethtx-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/ante/preprocess.go:226</code> | <code>func RecoverSenderFromEthTx(ctx sdk.Context, ethTx *ethtypes.Transaction, chainID *big.Int) (common.Address, sdk.AccAddress, cryptotypes.PubKey, error)</code> |
| <code>interface.6.go-evm-ante-signermap-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/ante/preprocess.go:38</code> | <code>var SignerMap = map[derived.SignerVersion]func(*big.Int) ethtypes.Signer{</code> |
| <code>interface.6.go-evm-ante-validatenocosmostxfields-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/ante/no_cosmos_fields.go:29</code> | <code>func ValidateNoCosmosTxFields(tx sdk.Tx) error</code> |
| <code>interface.6.go-evm-appmodule-initgenesis-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/module.go:256</code> | <code>func (am AppModule) InitGenesis(ctx sdk.Context, cdc codec.JSONCodec, gs json.RawMessage) []abci.ValidatorUpdate</code> |
| <code>interface.6.go-evm-appmodule-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/module.go:115</code> | <code>type AppModule struct</code> |
| <code>interface.6.go-evm-appmodule-registerservices-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/module.go:151</code> | <code>func (am AppModule) RegisterServices(cfg module.Configurator)</code> |
| <code>interface.6.go-evm-appmodulebasic-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/module.go:32</code> | <code>type AppModuleBasic struct</code> |
| <code>interface.6.go-evm-artifacts-cw1155-currentversion-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/artifacts/cw1155/artifacts.go:15</code> | <code>const CurrentVersion uint16 = 2</code> |
| <code>interface.6.go-evm-artifacts-cw1155-getparsedabi-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/artifacts/cw1155/artifacts.go:33</code> | <code>func GetParsedABI() *abi.ABI</code> |
| <code>interface.6.go-evm-artifacts-cw20-currentversion-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/artifacts/cw20/artifacts.go:28</code> | <code>func CurrentVersion(ctx sdk.Context) uint16</code> |
| <code>interface.6.go-evm-artifacts-cw20-getbin-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/artifacts/cw20/artifacts.go:62</code> | <code>func GetBin() []byte</code> |
| <code>interface.6.go-evm-artifacts-cw20-getparsedabi-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/artifacts/cw20/artifacts.go:50</code> | <code>func GetParsedABI() *abi.ABI</code> |
| <code>interface.6.go-evm-artifacts-cw20-setversionwithoffset-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/artifacts/cw20/artifacts.go:23</code> | <code>func SetVersionWithOffset(offset int16)</code> |
| <code>interface.6.go-evm-artifacts-cw721-currentversion-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/artifacts/cw721/artifacts.go:15</code> | <code>const CurrentVersion uint16 = 6</code> |
| <code>interface.6.go-evm-artifacts-cw721-getbin-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/artifacts/cw721/artifacts.go:47</code> | <code>func GetBin() []byte</code> |
| <code>interface.6.go-evm-artifacts-cw721-getparsedabi-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/artifacts/cw721/artifacts.go:35</code> | <code>func GetParsedABI() *abi.ABI</code> |
| <code>interface.6.go-evm-artifacts-getparsedabi-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/artifacts/artifacts.go:13</code> | <code>func GetParsedABI(typ string) *abi.ABI</code> |
| <code>interface.6.go-evm-artifacts-native-currentversion-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/artifacts/native/artifacts.go:15</code> | <code>const CurrentVersion uint16 = 1</code> |
| <code>interface.6.go-evm-artifacts-native-getbin-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/artifacts/native/artifacts.go:45</code> | <code>func GetBin() []byte</code> |
| <code>interface.6.go-evm-artifacts-native-getparsedabi-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/artifacts/native/artifacts.go:33</code> | <code>func GetParsedABI() *abi.ABI</code> |
| <code>interface.6.go-evm-artifacts-native-nativeapprovaliterator-close-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/artifacts/native/native.go:649</code> | <code>func (it *NativeApprovalIterator) Close() error</code> |
| <code>interface.6.go-evm-artifacts-native-nativetransferiterator-close-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/artifacts/native/native.go:803</code> | <code>func (it *NativeTransferIterator) Close() error</code> |
| <code>interface.6.go-evm-basic-registergrpcgatewayroutes-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/module.go:96</code> | <code>func (AppModuleBasic) RegisterGRPCGatewayRoutes(clientCtx client.Context, mux *runtime.ServeMux)</code> |
| <code>interface.6.go-evm-basic-registerrestroutes-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/module.go:93</code> | <code>func (AppModuleBasic) RegisterRESTRoutes(_ client.Context, _ *mux.Router) {}</code> |
| <code>interface.6.go-evm-blocktest-config-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/blocktest/config.go:8</code> | <code>type Config struct</code> |
| <code>interface.6.go-evm-blocktest-defaultconfig-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/blocktest/config.go:13</code> | <code>var DefaultConfig = Config{</code> |
| <code>interface.6.go-evm-blocktest-readconfig-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/blocktest/config.go:23</code> | <code>func ReadConfig(opts servertypes.AppOptions) (Config, error)</code> |
| <code>interface.6.go-evm-client-wasm-bindings-erc1155balanceofbatchtype-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/client/wasm/bindings/queries.go:32</code> | <code>ERC1155BalanceOfBatchType      EVMQueryType = "evm_query_erc1155_balance_of_batch"</code> |
| <code>interface.6.go-evm-client-wasm-bindings-erc1155balanceoftype-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/client/wasm/bindings/queries.go:31</code> | <code>ERC1155BalanceOfType           EVMQueryType = "evm_query_erc1155_balance_of"</code> |
| <code>interface.6.go-evm-client-wasm-bindings-erc1155batchtransfertype-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/client/wasm/bindings/queries.go:28</code> | <code>ERC1155BatchTransferType       EVMQueryType = "evm_query_erc1155_batch_transfer"</code> |
| <code>interface.6.go-evm-client-wasm-bindings-erc1155isapprovedforalltype-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/client/wasm/bindings/queries.go:30</code> | <code>ERC1155IsApprovedForAllType    EVMQueryType = "evm_query_erc1155_is_approved_for_all"</code> |
| <code>interface.6.go-evm-client-wasm-bindings-erc1155namesymboltype-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/client/wasm/bindings/queries.go:36</code> | <code>ERC1155NameSymbolType          EVMQueryType = "evm_query_erc1155_name_symbol"</code> |
| <code>interface.6.go-evm-client-wasm-bindings-erc1155royaltyinfotype-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/client/wasm/bindings/queries.go:38</code> | <code>ERC1155RoyaltyInfoType         EVMQueryType = "evm_query_erc1155_royalty_info"</code> |
| <code>interface.6.go-evm-client-wasm-bindings-erc1155setapprovalalltype-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/client/wasm/bindings/queries.go:29</code> | <code>ERC1155SetApprovalAllType      EVMQueryType = "evm_query_erc1155_set_approval_all"</code> |
| <code>interface.6.go-evm-client-wasm-bindings-erc1155tokenexiststype-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/client/wasm/bindings/queries.go:35</code> | <code>ERC1155TokenExistsType         EVMQueryType = "evm_query_erc1155_token_exists"           //nolint:gosec</code> |
| <code>interface.6.go-evm-client-wasm-bindings-erc1155totalsupplyfortokentype-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/client/wasm/bindings/queries.go:34</code> | <code>ERC1155TotalSupplyForTokenType EVMQueryType = "evm_query_erc1155_total_supply_for_token" //nolint:gosec</code> |
| <code>interface.6.go-evm-client-wasm-bindings-erc1155totalsupplytype-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/client/wasm/bindings/queries.go:33</code> | <code>ERC1155TotalSupplyType         EVMQueryType = "evm_query_erc1155_total_supply"</code> |
| <code>interface.6.go-evm-client-wasm-bindings-erc1155transfertype-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/client/wasm/bindings/queries.go:27</code> | <code>ERC1155TransferType            EVMQueryType = "evm_query_erc1155_transfer"</code> |
| <code>interface.6.go-evm-client-wasm-bindings-erc1155uritype-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/client/wasm/bindings/queries.go:37</code> | <code>ERC1155UriType                 EVMQueryType = "evm_query_erc1155_uri"</code> |
| <code>interface.6.go-evm-client-wasm-bindings-erc20allowancetype-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/client/wasm/bindings/queries.go:14</code> | <code>ERC20AllowanceType             EVMQueryType = "evm_query_erc20_allowance"</code> |
| <code>interface.6.go-evm-client-wasm-bindings-erc20approvetype-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/client/wasm/bindings/queries.go:13</code> | <code>ERC20ApproveType               EVMQueryType = "evm_query_erc20_approve"</code> |
| <code>interface.6.go-evm-client-wasm-bindings-erc20balancetype-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/client/wasm/bindings/queries.go:16</code> | <code>ERC20BalanceType               EVMQueryType = "evm_query_erc20_balance"</code> |
| <code>interface.6.go-evm-client-wasm-bindings-erc20tokeninfotype-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/client/wasm/bindings/queries.go:15</code> | <code>ERC20TokenInfoType             EVMQueryType = "evm_query_erc20_token_info" //nolint:gosec</code> |
| <code>interface.6.go-evm-client-wasm-bindings-erc20transferfromtype-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/client/wasm/bindings/queries.go:12</code> | <code>ERC20TransferFromType          EVMQueryType = "evm_query_erc20_transfer_from"</code> |
| <code>interface.6.go-evm-client-wasm-bindings-erc20transfertype-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/client/wasm/bindings/queries.go:11</code> | <code>ERC20TransferType              EVMQueryType = "evm_query_erc20_transfer"</code> |
| <code>interface.6.go-evm-client-wasm-bindings-erc721approvedtype-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/client/wasm/bindings/queries.go:21</code> | <code>ERC721ApprovedType             EVMQueryType = "evm_query_erc721_approved"</code> |
| <code>interface.6.go-evm-client-wasm-bindings-erc721approvetype-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/client/wasm/bindings/queries.go:19</code> | <code>ERC721ApproveType              EVMQueryType = "evm_query_erc721_approve"</code> |
| <code>interface.6.go-evm-client-wasm-bindings-erc721isapprovedforalltype-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/client/wasm/bindings/queries.go:22</code> | <code>ERC721IsApprovedForAllType     EVMQueryType = "evm_query_erc721_is_approved_for_all"</code> |
| <code>interface.6.go-evm-client-wasm-bindings-erc721namesymboltype-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/client/wasm/bindings/queries.go:24</code> | <code>ERC721NameSymbolType           EVMQueryType = "evm_query_erc721_name_symbol"</code> |
| <code>interface.6.go-evm-client-wasm-bindings-erc721ownertype-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/client/wasm/bindings/queries.go:17</code> | <code>ERC721OwnerType                EVMQueryType = "evm_query_erc721_owner"</code> |
| <code>interface.6.go-evm-client-wasm-bindings-erc721royaltyinfotype-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/client/wasm/bindings/queries.go:26</code> | <code>ERC721RoyaltyInfoType          EVMQueryType = "evm_query_erc721_royalty_info"</code> |
| <code>interface.6.go-evm-client-wasm-bindings-erc721setapprovalalltype-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/client/wasm/bindings/queries.go:20</code> | <code>ERC721SetApprovalAllType       EVMQueryType = "evm_query_erc721_set_approval_all"</code> |
| <code>interface.6.go-evm-client-wasm-bindings-erc721totalsupplytype-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/client/wasm/bindings/queries.go:23</code> | <code>ERC721TotalSupplyType          EVMQueryType = "evm_query_erc721_total_supply"</code> |
| <code>interface.6.go-evm-client-wasm-bindings-erc721transfertype-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/client/wasm/bindings/queries.go:18</code> | <code>ERC721TransferType             EVMQueryType = "evm_query_erc721_transfer"</code> |
| <code>interface.6.go-evm-client-wasm-bindings-erc721uritype-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/client/wasm/bindings/queries.go:25</code> | <code>ERC721UriType                  EVMQueryType = "evm_query_erc721_uri"</code> |
| <code>interface.6.go-evm-client-wasm-bindings-evmquerytype-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/client/wasm/bindings/queries.go:7</code> | <code>type EVMQueryType string</code> |
| <code>interface.6.go-evm-client-wasm-bindings-getevmaddresstype-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/client/wasm/bindings/queries.go:39</code> | <code>GetEvmAddressType              EVMQueryType = "evm_query_get_evm_address"</code> |
| <code>interface.6.go-evm-client-wasm-bindings-getpaxaddresstype-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/client/wasm/bindings/queries.go:40</code> | <code>GetPaxAddressType              EVMQueryType = "evm_query_get_pax_address"</code> |
| <code>interface.6.go-evm-client-wasm-bindings-paxevmquery-getquerytype-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/client/wasm/bindings/queries.go:44</code> | <code>func (q *PaxEVMQuery) GetQueryType() EVMQueryType</code> |
| <code>interface.6.go-evm-client-wasm-bindings-paxevmquery-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/client/wasm/bindings/queries.go:144</code> | <code>type PaxEVMQuery struct</code> |
| <code>interface.6.go-evm-client-wasm-bindings-staticcalltype-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/client/wasm/bindings/queries.go:10</code> | <code>StaticCallType                 EVMQueryType = "evm_query_static_call"</code> |
| <code>interface.6.go-evm-client-wasm-bindings-supportsinterfacetype-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/client/wasm/bindings/queries.go:41</code> | <code>SupportsInterfaceType          EVMQueryType = "evm_query_supports_interface"</code> |
| <code>interface.6.go-evm-client-wasm-encodecallevm-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/client/wasm/encoder.go:14</code> | <code>func EncodeCallEVM(rawMsg json.RawMessage, sender sdk.AccAddress, info wasmvmtypes.MessageInfo) ([]sdk.Msg, error)</code> |
| <code>interface.6.go-evm-client-wasm-encodedelegatecallevm-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/client/wasm/encoder.go:32</code> | <code>func EncodeDelegateCallEVM(rawMsg json.RawMessage, sender sdk.AccAddress, info wasmvmtypes.MessageInfo, codeInfo wasmtypes.CodeInfo) ([]sdk.Msg, error)</code> |
| <code>interface.6.go-evm-client-wasm-evmqueryhandler-handleerc1155balanceof-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/client/wasm/query.go:741</code> | <code>func (h *EVMQueryHandler) HandleERC1155BalanceOf(ctx sdk.Context, caller string, contractAddress string, account string, tokenId string) ([]byte, error)</code> |
| <code>interface.6.go-evm-client-wasm-evmqueryhandler-handleerc1155balanceofbatch-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/client/wasm/query.go:780</code> | <code>func (h *EVMQueryHandler) HandleERC1155BalanceOfBatch(ctx sdk.Context, caller string, contractAddress string, accounts []string, tokenIds []string) ([]byte, error)</code> |
| <code>interface.6.go-evm-client-wasm-evmqueryhandler-handleerc1155batchtransferpayload-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/client/wasm/query.go:634</code> | <code>func (h *EVMQueryHandler) HandleERC1155BatchTransferPayload(ctx sdk.Context, from string, recipient string, tokenIds []string, amounts []*sdk.Int) ([]byte, error)</code> |
| <code>interface.6.go-evm-client-wasm-evmqueryhandler-handleerc1155isapprovedforall-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/client/wasm/query.go:699</code> | <code>func (h *EVMQueryHandler) HandleERC1155IsApprovedForAll(ctx sdk.Context, caller string, contractAddress string, owner string, operator string) ([]byte, error)</code> |
| <code>interface.6.go-evm-client-wasm-evmqueryhandler-handleerc1155namesymbol-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/client/wasm/query.go:955</code> | <code>func (h *EVMQueryHandler) HandleERC1155NameSymbol(ctx sdk.Context, caller string, contractAddress string) ([]byte, error)</code> |
| <code>interface.6.go-evm-client-wasm-evmqueryhandler-handleerc1155royaltyinfo-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/client/wasm/query.go:995</code> | <code>func (h *EVMQueryHandler) HandleERC1155RoyaltyInfo(ctx sdk.Context, caller string, contractAddress string, tokenId string, salePrice *sdk.Int) ([]byte, error)</code> |
| <code>interface.6.go-evm-client-wasm-evmqueryhandler-handleerc1155setapprovalallpayload-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/client/wasm/query.go:678</code> | <code>func (h *EVMQueryHandler) HandleERC1155SetApprovalAllPayload(ctx sdk.Context, to string, approved bool) ([]byte, error)</code> |
| <code>interface.6.go-evm-client-wasm-evmqueryhandler-handleerc1155tokenexists-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/client/wasm/query.go:925</code> | <code>func (h *EVMQueryHandler) HandleERC1155TokenExists(ctx sdk.Context, caller string, contractAddress string, tokenId string) ([]byte, error)</code> |
| <code>interface.6.go-evm-client-wasm-evmqueryhandler-handleerc1155totalsupply-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/client/wasm/query.go:867</code> | <code>func (h *EVMQueryHandler) HandleERC1155TotalSupply(ctx sdk.Context, caller string, contractAddress string) ([]byte, error)</code> |
| <code>interface.6.go-evm-client-wasm-evmqueryhandler-handleerc1155totalsupplyfortoken-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/client/wasm/query.go:894</code> | <code>func (h *EVMQueryHandler) HandleERC1155TotalSupplyForToken(ctx sdk.Context, caller string, contractAddress string, tokenId string) ([]byte, error)</code> |
| <code>interface.6.go-evm-client-wasm-evmqueryhandler-handleerc1155transferpayload-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/client/wasm/query.go:601</code> | <code>func (h *EVMQueryHandler) HandleERC1155TransferPayload(ctx sdk.Context, from string, recipient string, tokenId string, amount *sdk.Int) ([]byte, error)</code> |
| <code>interface.6.go-evm-client-wasm-evmqueryhandler-handleerc1155uri-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/client/wasm/query.go:837</code> | <code>func (h *EVMQueryHandler) HandleERC1155Uri(ctx sdk.Context, caller string, contractAddress string, tokenId string) ([]byte, error)</code> |
| <code>interface.6.go-evm-client-wasm-evmqueryhandler-handleerc20allowance-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/client/wasm/query.go:341</code> | <code>func (h *EVMQueryHandler) HandleERC20Allowance(ctx sdk.Context, contractAddress string, owner string, spender string) ([]byte, error)</code> |
| <code>interface.6.go-evm-client-wasm-evmqueryhandler-handleerc20approvepayload-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/client/wasm/query.go:319</code> | <code>func (h *EVMQueryHandler) HandleERC20ApprovePayload(ctx sdk.Context, spender string, amount *sdk.Int) ([]byte, error)</code> |
| <code>interface.6.go-evm-client-wasm-evmqueryhandler-handleerc20balance-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/client/wasm/query.go:140</code> | <code>func (h *EVMQueryHandler) HandleERC20Balance(ctx sdk.Context, contractAddress string, account string) ([]byte, error)</code> |
| <code>interface.6.go-evm-client-wasm-evmqueryhandler-handleerc20tokeninfo-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/client/wasm/query.go:68</code> | <code>func (h *EVMQueryHandler) HandleERC20TokenInfo(ctx sdk.Context, contractAddress string, caller string) ([]byte, error)</code> |
| <code>interface.6.go-evm-client-wasm-evmqueryhandler-handleerc20transferfrompayload-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/client/wasm/query.go:290</code> | <code>func (h *EVMQueryHandler) HandleERC20TransferFromPayload(ctx sdk.Context, owner string, recipient string, amount *sdk.Int) ([]byte, error)</code> |
| <code>interface.6.go-evm-client-wasm-evmqueryhandler-handleerc20transferpayload-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/client/wasm/query.go:47</code> | <code>func (h *EVMQueryHandler) HandleERC20TransferPayload(ctx sdk.Context, recipient string, amount *sdk.Int) ([]byte, error)</code> |
| <code>interface.6.go-evm-client-wasm-evmqueryhandler-handleerc721approved-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/client/wasm/query.go:390</code> | <code>func (h *EVMQueryHandler) HandleERC721Approved(ctx sdk.Context, caller string, contractAddress string, tokenId string) ([]byte, error)</code> |
| <code>interface.6.go-evm-client-wasm-evmqueryhandler-handleerc721approvepayload-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/client/wasm/query.go:239</code> | <code>func (h *EVMQueryHandler) HandleERC721ApprovePayload(ctx sdk.Context, spender string, tokenId string) ([]byte, error)</code> |
| <code>interface.6.go-evm-client-wasm-evmqueryhandler-handleerc721isapprovedforall-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/client/wasm/query.go:425</code> | <code>func (h *EVMQueryHandler) HandleERC721IsApprovedForAll(ctx sdk.Context, caller string, contractAddress string, owner string, operator string) ([]byte, error)</code> |
| <code>interface.6.go-evm-client-wasm-evmqueryhandler-handleerc721namesymbol-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/client/wasm/query.go:494</code> | <code>func (h *EVMQueryHandler) HandleERC721NameSymbol(ctx sdk.Context, caller string, contractAddress string) ([]byte, error)</code> |
| <code>interface.6.go-evm-client-wasm-evmqueryhandler-handleerc721owner-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/client/wasm/query.go:171</code> | <code>func (h *EVMQueryHandler) HandleERC721Owner(ctx sdk.Context, caller string, contractAddress string, tokenId string) ([]byte, error)</code> |
| <code>interface.6.go-evm-client-wasm-evmqueryhandler-handleerc721royaltyinfo-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/client/wasm/query.go:564</code> | <code>func (h *EVMQueryHandler) HandleERC721RoyaltyInfo(ctx sdk.Context, caller string, contractAddress string, tokenId string, salePrice *sdk.Int) ([]byte, error)</code> |
| <code>interface.6.go-evm-client-wasm-evmqueryhandler-handleerc721setapprovalallpayload-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/client/wasm/query.go:269</code> | <code>func (h *EVMQueryHandler) HandleERC721SetApprovalAllPayload(ctx sdk.Context, to string, approved bool) ([]byte, error)</code> |
| <code>interface.6.go-evm-client-wasm-evmqueryhandler-handleerc721totalsupply-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/client/wasm/query.go:467</code> | <code>func (h *EVMQueryHandler) HandleERC721TotalSupply(ctx sdk.Context, caller string, contractAddress string) ([]byte, error)</code> |
| <code>interface.6.go-evm-client-wasm-evmqueryhandler-handleerc721transferpayload-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/client/wasm/query.go:206</code> | <code>func (h *EVMQueryHandler) HandleERC721TransferPayload(ctx sdk.Context, from string, recipient string, tokenId string) ([]byte, error)</code> |
| <code>interface.6.go-evm-client-wasm-evmqueryhandler-handleerc721uri-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/client/wasm/query.go:534</code> | <code>func (h *EVMQueryHandler) HandleERC721Uri(ctx sdk.Context, caller string, contractAddress string, tokenId string) ([]byte, error)</code> |
| <code>interface.6.go-evm-client-wasm-evmqueryhandler-handlegetevmaddress-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/client/wasm/query.go:1063</code> | <code>func (h *EVMQueryHandler) HandleGetEvmAddress(ctx sdk.Context, paxAddr string) ([]byte, error)</code> |
| <code>interface.6.go-evm-client-wasm-evmqueryhandler-handlegetpaxaddress-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/client/wasm/query.go:1073</code> | <code>func (h *EVMQueryHandler) HandleGetPaxAddress(ctx sdk.Context, evmAddr string) ([]byte, error)</code> |
| <code>interface.6.go-evm-client-wasm-evmqueryhandler-handlestaticcall-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/client/wasm/query.go:29</code> | <code>func (h *EVMQueryHandler) HandleStaticCall(ctx sdk.Context, from string, to string, data []byte) ([]byte, error)</code> |
| <code>interface.6.go-evm-client-wasm-evmqueryhandler-handlesupportsinterface-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/client/wasm/query.go:1032</code> | <code>func (h *EVMQueryHandler) HandleSupportsInterface(ctx sdk.Context, caller string, id string, contractAddress string) ([]byte, error)</code> |
| <code>interface.6.go-evm-client-wasm-evmqueryhandler-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/client/wasm/query.go:21</code> | <code>type EVMQueryHandler struct</code> |
| <code>interface.6.go-evm-client-wasm-newevmqueryhandler-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/client/wasm/query.go:25</code> | <code>func NewEVMQueryHandler(k *keeper.Keeper) *EVMQueryHandler</code> |
| <code>interface.6.go-evm-config-chainidmapping-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/config/config.go:12</code> | <code>var ChainIDMapping = map[string]int64{</code> |
| <code>interface.6.go-evm-config-defaultchainid-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/config/config.go:9</code> | <code>const DefaultChainID = int64(713714)</code> |
| <code>interface.6.go-evm-config-evmchainidmapping-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/config/config.go:23</code> | <code>var EVMChainIDMapping = map[int64]string{</code> |
| <code>interface.6.go-evm-config-getevmchainid-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/config/config.go:33</code> | <code>func GetEVMChainID(cosmosChainID string) *big.Int</code> |
| <code>interface.6.go-evm-config-islivechainid-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/config/config.go:49</code> | <code>func IsLiveChainID(ctx sdk.Context) bool</code> |
| <code>interface.6.go-evm-config-isliveevmchainid-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/config/config.go:55</code> | <code>func IsLiveEVMChainID(evmChainID int64) bool</code> |
| <code>interface.6.go-evm-derived-cancun-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/derived/derived.go:13</code> | <code>Cancun</code> |
| <code>interface.6.go-evm-derived-derived-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/derived/derived.go:17</code> | <code>type Derived struct</code> |
| <code>interface.6.go-evm-derived-london-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/derived/derived.go:12</code> | <code>London SignerVersion = iota</code> |
| <code>interface.6.go-evm-derived-prague-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/derived/derived.go:14</code> | <code>Prague</code> |
| <code>interface.6.go-evm-derived-signerversion-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/derived/derived.go:9</code> | <code>type SignerVersion int</code> |
| <code>interface.6.go-evm-getgenesisstatefromappstate-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/genesis.go:186</code> | <code>func GetGenesisStateFromAppState(cdc codec.JSONCodec, appState map[string]json.RawMessage) *types.GenesisState</code> |
| <code>interface.6.go-evm-keeper-basedenom-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/params.go:22</code> | <code>const BaseDenom = "uhpx"</code> |
| <code>interface.6.go-evm-keeper-convertethlog-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/log.go:100</code> | <code>func ConvertEthLog(l *ethtypes.Log) *types.Log</code> |
| <code>interface.6.go-evm-keeper-convertfeefromdenom-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/feetoken.go:52</code> | <code>func ConvertFeeFromDenom(amount sdk.Int, rate sdk.Dec, roundUp bool) (sdk.Int, error)</code> |
| <code>interface.6.go-evm-keeper-convertfeetodenom-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/feetoken.go:47</code> | <code>func ConvertFeeToDenom(amount sdk.Int, rate sdk.Dec, roundUp bool) (sdk.Int, error)</code> |
| <code>interface.6.go-evm-keeper-convertsyntheticethlog-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/log.go:109</code> | <code>func ConvertSyntheticEthLog(l *ethtypes.Log) *types.Log</code> |
| <code>interface.6.go-evm-keeper-errfeetokendenomnotallowed-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/feetoken.go:43</code> | <code>ErrFeeTokenDenomNotAllowed = errors.New("fee-token denom is not allowed")</code> |
| <code>interface.6.go-evm-keeper-errfeetokenoverflow-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/feetoken.go:42</code> | <code>ErrFeeTokenOverflow        = errors.New("fee-token conversion exceeds 256 bits")</code> |
| <code>interface.6.go-evm-keeper-errfeetokenrateinvalid-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/params.go:18</code> | <code>ErrFeeTokenRateInvalid     = errors.New("fee-token rate is invalid")</code> |
| <code>interface.6.go-evm-keeper-errfeetokenratespread-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/params.go:19</code> | <code>ErrFeeTokenRateSpread      = errors.New("fee-token rate update exceeds max_fee_token_spread")</code> |
| <code>interface.6.go-evm-keeper-errfeetokenratestale-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/params.go:16</code> | <code>ErrFeeTokenRateStale       = errors.New("fee-token rate is stale")</code> |
| <code>interface.6.go-evm-keeper-errfeetokenrateunavailable-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/params.go:17</code> | <code>ErrFeeTokenRateUnavailable = errors.New("fee-token rate is unavailable")</code> |
| <code>interface.6.go-evm-keeper-getlogsfortx-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/log.go:85</code> | <code>func GetLogsForTx(receipt *types.Receipt, logStartIndex uint) []*ethtypes.Log</code> |
| <code>interface.6.go-evm-keeper-ispayableprecompile-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/precompile.go:23</code> | <code>func IsPayablePrecompile(addr *common.Address) bool</code> |
| <code>interface.6.go-evm-keeper-keeper-accountkeeper-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/keeper.go:387</code> | <code>func (k *Keeper) AccountKeeper() *authkeeper.AccountKeeper</code> |
| <code>interface.6.go-evm-keeper-keeper-addantesurplus-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/ante.go:12</code> | <code>func (k *Keeper) AddAnteSurplus(ctx sdk.Context, txHash common.Hash, surplus sdk.Int) error</code> |
| <code>interface.6.go-evm-keeper-keeper-appendtoevmtxdeferredinfo-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/deferred.go:51</code> | <code>func (k *Keeper) AppendToEvmTxDeferredInfo(ctx sdk.Context, bloom ethtypes.Bloom, txHash common.Hash, surplus sdk.Int)</code> |
| <code>interface.6.go-evm-keeper-keeper-bankkeeper-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/keeper.go:391</code> | <code>func (k *Keeper) BankKeeper() bankkeeper.Keeper</code> |
| <code>interface.6.go-evm-keeper-keeper-beginblock-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/abci.go:25</code> | <code>func (k *Keeper) BeginBlock(ctx sdk.Context)</code> |
| <code>interface.6.go-evm-keeper-keeper-callevm-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/evm.go:81</code> | <code>func (k *Keeper) CallEVM(ctx sdk.Context, from common.Address, to *common.Address, val *sdk.Int, data []byte) (retdata []byte, reterr error)</code> |
| <code>interface.6.go-evm-keeper-keeper-chainid-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/params.go:239</code> | <code>func (k *Keeper) ChainID(ctx sdk.Context) *big.Int</code> |
| <code>interface.6.go-evm-keeper-keeper-convertfeefromdenom-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/feetoken.go:88</code> | <code>func (k *Keeper) ConvertFeeFromDenom(amount sdk.Int, rate sdk.Dec, roundUp bool) (sdk.Int, error)</code> |
| <code>interface.6.go-evm-keeper-keeper-convertfeetodenom-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/feetoken.go:84</code> | <code>func (k *Keeper) ConvertFeeToDenom(amount sdk.Int, rate sdk.Dec, roundUp bool) (sdk.Int, error)</code> |
| <code>interface.6.go-evm-keeper-keeper-customprecompiles-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/keeper.go:248</code> | <code>func (k *Keeper) CustomPrecompiles(ctx sdk.Context) map[common.Address]vm.PrecompiledContract</code> |
| <code>interface.6.go-evm-keeper-keeper-endblock-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/abci.go:74</code> | <code>func (k *Keeper) EndBlock(ctx sdk.Context, height int64, blockGasUsed int64)</code> |
| <code>interface.6.go-evm-keeper-keeper-flushtransientreceipts-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/receipt.go:121</code> | <code>func (k *Keeper) FlushTransientReceipts(ctx sdk.Context) error</code> |
| <code>interface.6.go-evm-keeper-keeper-getallowedfeedenoms-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/params.go:283</code> | <code>func (k *Keeper) GetAllowedFeeDenoms(ctx sdk.Context) []types.AllowedFeeDenom</code> |
| <code>interface.6.go-evm-keeper-keeper-getantefeetokencharge-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/feetoken.go:124</code> | <code>func (k *Keeper) GetAnteFeeTokenCharge(ctx sdk.Context, hash common.Hash) (*state.FeeTokenCharge, error)</code> |
| <code>interface.6.go-evm-keeper-keeper-getantesurplussum-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/ante.go:22</code> | <code>func (k *Keeper) GetAnteSurplusSum(ctx sdk.Context) (sdk.Int, error)</code> |
| <code>interface.6.go-evm-keeper-keeper-getanypointeeinfo-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/pointer.go:304</code> | <code>func (k *Keeper) GetAnyPointeeInfo(ctx sdk.Context, cwAddress string) (common.Address, uint16, bool)</code> |
| <code>interface.6.go-evm-keeper-keeper-getbalance-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/balance.go:10</code> | <code>func (k *Keeper) GetBalance(ctx sdk.Context, addr sdk.AccAddress) *big.Int</code> |
| <code>interface.6.go-evm-keeper-keeper-getbasedenom-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/params.go:83</code> | <code>func (k *Keeper) GetBaseDenom(ctx sdk.Context) string</code> |
| <code>interface.6.go-evm-keeper-keeper-getbasefee-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/keeper.go:565</code> | <code>func (k *Keeper) GetBaseFee(ctx sdk.Context) *big.Int</code> |
| <code>interface.6.go-evm-keeper-keeper-getbasefeepergas-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/params.go:105</code> | <code>func (k *Keeper) GetBaseFeePerGas(ctx sdk.Context) sdk.Dec</code> |
| <code>interface.6.go-evm-keeper-keeper-getblockbloom-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/log.go:14</code> | <code>func (k *Keeper) GetBlockBloom(ctx sdk.Context) (res ethtypes.Bloom)</code> |
| <code>interface.6.go-evm-keeper-keeper-getcode-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/code.go:15</code> | <code>func (k *Keeper) GetCode(ctx sdk.Context, addr common.Address) []byte</code> |
| <code>interface.6.go-evm-keeper-keeper-getcurrbasefeepergas-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/fee.go:63</code> | <code>func (k *Keeper) GetCurrBaseFeePerGas(ctx sdk.Context) sdk.Dec</code> |
| <code>interface.6.go-evm-keeper-keeper-getcustomprecompilesversions-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/keeper.go:356</code> | <code>func (k *Keeper) GetCustomPrecompilesVersions(ctx sdk.Context) map[common.Address]string</code> |
| <code>interface.6.go-evm-keeper-keeper-getdelivertxhookwasmgaslimit-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/params.go:207</code> | <code>func (k *Keeper) GetDeliverTxHookWasmGasLimit(ctx sdk.Context) uint64</code> |
| <code>interface.6.go-evm-keeper-keeper-geterc1155cw1155pointer-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/pointer.go:151</code> | <code>func (k *Keeper) GetERC1155CW1155Pointer(ctx sdk.Context, cw1155Address string) (addr common.Address, version uint16, exists bool)</code> |
| <code>interface.6.go-evm-keeper-keeper-geterc20cw20pointer-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/pointer.go:81</code> | <code>func (k *Keeper) GetERC20CW20Pointer(ctx sdk.Context, cw20Address string) (addr common.Address, version uint16, exists bool)</code> |
| <code>interface.6.go-evm-keeper-keeper-geterc20nativepointer-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/pointer.go:46</code> | <code>func (k *Keeper) GetERC20NativePointer(ctx sdk.Context, token string) (addr common.Address, version uint16, exists bool)</code> |
| <code>interface.6.go-evm-keeper-keeper-geterc721cw721pointer-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/pointer.go:116</code> | <code>func (k *Keeper) GetERC721CW721Pointer(ctx sdk.Context, cw721Address string) (addr common.Address, version uint16, exists bool)</code> |
| <code>interface.6.go-evm-keeper-keeper-getevmaddress-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/address.go:30</code> | <code>func (k *Keeper) GetEVMAddress(ctx sdk.Context, paxAddress sdk.AccAddress) (common.Address, bool)</code> |
| <code>interface.6.go-evm-keeper-keeper-getevmaddressordefault-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/address.go:41</code> | <code>func (k *Keeper) GetEVMAddressOrDefault(ctx sdk.Context, paxAddress sdk.AccAddress) common.Address</code> |
| <code>interface.6.go-evm-keeper-keeper-getevmmessage-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/msg_server.go:204</code> | <code>func (k *Keeper) GetEVMMessage(ctx sdk.Context, tx *ethtypes.Transaction, sender common.Address) *core.Message</code> |
| <code>interface.6.go-evm-keeper-keeper-getevmonlyblockbloom-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/log.go:28</code> | <code>func (k *Keeper) GetEvmOnlyBlockBloom(ctx sdk.Context) (res ethtypes.Bloom)</code> |
| <code>interface.6.go-evm-keeper-keeper-getevmtxdeferredinfo-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/deferred.go:81</code> | <code>func (k *Keeper) GetEVMTxDeferredInfo(ctx sdk.Context) (*types.DeferredInfo, bool)</code> |
| <code>interface.6.go-evm-keeper-keeper-getfeecollectoraddress-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/coinbase.go:15</code> | <code>func (k *Keeper) GetFeeCollectorAddress(ctx sdk.Context) (common.Address, error)</code> |
| <code>interface.6.go-evm-keeper-keeper-getfeetokencharge-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/feetoken.go:92</code> | <code>func (k *Keeper) GetFeeTokenCharge(ctx sdk.Context, payer common.Address) (*state.FeeTokenCharge, error)</code> |
| <code>interface.6.go-evm-keeper-keeper-getfeetokenenabled-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/params.go:295</code> | <code>func (k *Keeper) GetFeeTokenEnabled(ctx sdk.Context) bool</code> |
| <code>interface.6.go-evm-keeper-keeper-getfeetokenrate-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/params.go:318</code> | <code>func (k *Keeper) GetFeeTokenRate(ctx sdk.Context, denom string) (sdk.Dec, error)</code> |
| <code>interface.6.go-evm-keeper-keeper-getgaspool-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/msg_server.go:200</code> | <code>func (k *Keeper) GetGasPool() core.GasPool</code> |
| <code>interface.6.go-evm-keeper-keeper-getminimumfeepergas-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/params.go:157</code> | <code>func (k *Keeper) GetMinimumFeePerGas(ctx sdk.Context) sdk.Dec</code> |
| <code>interface.6.go-evm-keeper-keeper-getnativepointee-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/pointer.go:444</code> | <code>func (k *Keeper) GetNativePointee(ctx sdk.Context, erc20Address string) (token string, version uint16, exists bool)</code> |
| <code>interface.6.go-evm-keeper-keeper-getnextbasefeepergas-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/fee.go:100</code> | <code>func (k *Keeper) GetNextBaseFeePerGas(ctx sdk.Context) sdk.Dec</code> |
| <code>interface.6.go-evm-keeper-keeper-getnonce-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/nonce.go:12</code> | <code>func (k *Keeper) GetNonce(ctx sdk.Context, addr common.Address) uint64</code> |
| <code>interface.6.go-evm-keeper-keeper-getparams-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/params.go:28</code> | <code>func (k *Keeper) GetParams(ctx sdk.Context) (params types.Params)</code> |
| <code>interface.6.go-evm-keeper-keeper-getpaxaddress-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/address.go:49</code> | <code>func (k *Keeper) GetPaxAddress(ctx sdk.Context, evmAddress common.Address) (sdk.AccAddress, bool)</code> |
| <code>interface.6.go-evm-keeper-keeper-getpaxaddressordefault-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/address.go:58</code> | <code>func (k *Keeper) GetPaxAddressOrDefault(ctx sdk.Context, evmAddress common.Address) sdk.AccAddress</code> |
| <code>interface.6.go-evm-keeper-keeper-getprioritynormalizer-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/params.go:87</code> | <code>func (k *Keeper) GetPriorityNormalizer(ctx sdk.Context) sdk.Dec</code> |
| <code>interface.6.go-evm-keeper-keeper-getreceipt-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/receipt.go:59</code> | <code>func (k *Keeper) GetReceipt(ctx sdk.Context, txHash common.Hash) (*types.Receipt, error)</code> |
| <code>interface.6.go-evm-keeper-keeper-getreceiptfromreceiptstore-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/receipt.go:67</code> | <code>func (k *Keeper) GetReceiptFromReceiptStore(ctx sdk.Context, txHash common.Hash) (*types.Receipt, error)</code> |
| <code>interface.6.go-evm-keeper-keeper-getreceiptwithretry-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/receipt.go:76</code> | <code>func (k *Keeper) GetReceiptWithRetry(ctx sdk.Context, txHash common.Hash, maxRetries int) (*types.Receipt, error)</code> |
| <code>interface.6.go-evm-keeper-keeper-getsstoresetgaseip2200-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/params.go:275</code> | <code>func (k *Keeper) GetSstoreSetGasEIP2200(ctx sdk.Context) uint64</code> |
| <code>interface.6.go-evm-keeper-keeper-getstate-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/state.go:10</code> | <code>func (k *Keeper) GetState(ctx sdk.Context, addr common.Address, hash common.Hash) common.Hash</code> |
| <code>interface.6.go-evm-keeper-keeper-getstorekey-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/keeper.go:407</code> | <code>func (k *Keeper) GetStoreKey() sdk.StoreKey</code> |
| <code>interface.6.go-evm-keeper-keeper-gettransientreceipt-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/receipt.go:38</code> | <code>func (k *Keeper) GetTransientReceipt(ctx sdk.Context, txHash common.Hash, txIndex uint64) (*types.Receipt, error)</code> |
| <code>interface.6.go-evm-keeper-keeper-getvmblockcontext-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/keeper.go:433</code> | <code>func (k *Keeper) GetVMBlockContext(ctx sdk.Context, gp core.GasPool) (*vm.BlockContext, error)</code> |
| <code>interface.6.go-evm-keeper-keeper-handleinternalevmcall-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/evm.go:31</code> | <code>func (k *Keeper) HandleInternalEVMCall(ctx sdk.Context, req *types.MsgInternalEVMCall) (*sdk.Result, error)</code> |
| <code>interface.6.go-evm-keeper-keeper-handleinternalevmdelegatecall-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/evm.go:48</code> | <code>func (k *Keeper) HandleInternalEVMDelegateCall(ctx sdk.Context, req *types.MsgInternalEVMDelegateCall) (*sdk.Result, error)</code> |
| <code>interface.6.go-evm-keeper-keeper-initgenesis-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/genesis.go:27</code> | <code>func (k *Keeper) InitGenesis(ctx sdk.Context, genState types.GenesisState)</code> |
| <code>interface.6.go-evm-keeper-keeper-isallowedfeedenom-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/params.go:301</code> | <code>func (k *Keeper) IsAllowedFeeDenom(ctx sdk.Context, denom string) (bool, sdk.Dec)</code> |
| <code>interface.6.go-evm-keeper-keeper-iteratestate-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/state.go:39</code> | <code>func (k *Keeper) IterateState(ctx sdk.Context, cb func(addr common.Address, key common.Hash, val common.Hash) bool)</code> |
| <code>interface.6.go-evm-keeper-keeper-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/keeper.go:53</code> | <code>type Keeper struct</code> |
| <code>interface.6.go-evm-keeper-keeper-mockreceipt-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/receipt.go:99</code> | <code>func (k *Keeper) MockReceipt(ctx sdk.Context, txHash common.Hash, rcpt *types.Receipt) error</code> |
| <code>interface.6.go-evm-keeper-keeper-preparectxforevmtransaction-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/msg_server.go:52</code> | <code>func (k *Keeper) PrepareCtxForEVMTransaction(ctx sdk.Context, tx *ethtypes.Transaction) (sdk.Context, sdk.GasMeter)</code> |
| <code>interface.6.go-evm-keeper-keeper-preparereplayedaddr-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/keeper.go:527</code> | <code>func (k *Keeper) PrepareReplayedAddr(ctx sdk.Context, addr common.Address)</code> |
| <code>interface.6.go-evm-keeper-keeper-receiptstore-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/keeper.go:395</code> | <code>func (k *Keeper) ReceiptStore() receipt.ReceiptStore</code> |
| <code>interface.6.go-evm-keeper-keeper-routecollectedfeetokens-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/feecollect.go:17</code> | <code>func (k *Keeper) RouteCollectedFeeTokens(ctx sdk.Context) error</code> |
| <code>interface.6.go-evm-keeper-keeper-runwithoneoffevminstance-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/pointer_upgrade.go:17</code> | <code>func (k *Keeper) RunWithOneOffEVMInstance(</code> |
| <code>interface.6.go-evm-keeper-keeper-setaccountfeedenom-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/feetoken.go:25</code> | <code>func (k *Keeper) SetAccountFeeDenom(ctx sdk.Context, account common.Address, denom string) error</code> |
| <code>interface.6.go-evm-keeper-keeper-setaddressmapping-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/address.go:10</code> | <code>func (k *Keeper) SetAddressMapping(ctx sdk.Context, paxAddress sdk.AccAddress, evmAddress common.Address)</code> |
| <code>interface.6.go-evm-keeper-keeper-setantefeetokencharge-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/feetoken.go:110</code> | <code>func (k *Keeper) SetAnteFeeTokenCharge(ctx sdk.Context, hash common.Hash, charge *state.FeeTokenCharge) error</code> |
| <code>interface.6.go-evm-keeper-keeper-setblockbloom-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/log.go:56</code> | <code>func (k *Keeper) SetBlockBloom(ctx sdk.Context, blooms []ethtypes.Bloom)</code> |
| <code>interface.6.go-evm-keeper-keeper-setcode-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/code.go:23</code> | <code>func (k *Keeper) SetCode(ctx sdk.Context, addr common.Address, code []byte)</code> |
| <code>interface.6.go-evm-keeper-keeper-setcustomprecompileactivation-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/keeper.go:207</code> | <code>func (k *Keeper) SetCustomPrecompileActivation(activations []CustomPrecompileActivation)</code> |
| <code>interface.6.go-evm-keeper-keeper-seterc20cw20pointer-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/pointer.go:64</code> | <code>func (k *Keeper) SetERC20CW20Pointer(ctx sdk.Context, cw20Address string, addr common.Address) error</code> |
| <code>interface.6.go-evm-keeper-keeper-seterc20nativepointer-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/pointer.go:29</code> | <code>func (k *Keeper) SetERC20NativePointer(ctx sdk.Context, token string, addr common.Address) error</code> |
| <code>interface.6.go-evm-keeper-keeper-setevmonlyblockbloom-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/log.go:51</code> | <code>func (k *Keeper) SetEvmOnlyBlockBloom(ctx sdk.Context, blooms []ethtypes.Bloom)</code> |
| <code>interface.6.go-evm-keeper-keeper-setmsgs-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/keeper.go:522</code> | <code>func (k *Keeper) SetMsgs(msgs []*types.MsgEVMTransaction)</code> |
| <code>interface.6.go-evm-keeper-keeper-setnonce-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/nonce.go:20</code> | <code>func (k *Keeper) SetNonce(ctx sdk.Context, addr common.Address, nonce uint64)</code> |
| <code>interface.6.go-evm-keeper-keeper-setnoncebumped-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/deferred.go:69</code> | <code>func (k *Keeper) SetNonceBumped(ctx sdk.Context)</code> |
| <code>interface.6.go-evm-keeper-keeper-setparams-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/params.go:24</code> | <code>func (k Keeper) SetParams(ctx sdk.Context, params types.Params)</code> |
| <code>interface.6.go-evm-keeper-keeper-setstate-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/state.go:30</code> | <code>func (k *Keeper) SetState(ctx sdk.Context, addr common.Address, key common.Hash, val common.Hash)</code> |
| <code>interface.6.go-evm-keeper-keeper-settracedb-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/keeper.go:163</code> | <code>func (k *Keeper) SetTraceDB(c *TraceDB) { k.traceDB = c }</code> |
| <code>interface.6.go-evm-keeper-keeper-settracesnapshotcapture-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/keeper.go:168</code> | <code>func (k *Keeper) SetTraceSnapshotCapture(f func() sctypes.Committer)</code> |
| <code>interface.6.go-evm-keeper-keeper-settracesnapshotstore-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/keeper.go:166</code> | <code>func (k *Keeper) SetTraceSnapshotStore(s *TraceSnapshotStore) { k.traceSnapshotStore = s }</code> |
| <code>interface.6.go-evm-keeper-keeper-settransientreceipt-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/receipt.go:28</code> | <code>func (k *Keeper) SetTransientReceipt(ctx sdk.Context, txHash common.Hash, receipt *types.Receipt) error</code> |
| <code>interface.6.go-evm-keeper-keeper-settxresults-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/keeper.go:518</code> | <code>func (k *Keeper) SetTxResults(txResults []*abci.ExecTxResult)</code> |
| <code>interface.6.go-evm-keeper-keeper-staticcallevm-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/evm.go:157</code> | <code>func (k *Keeper) StaticCallEVM(ctx sdk.Context, from sdk.AccAddress, to *common.Address, data []byte) ([]byte, error)</code> |
| <code>interface.6.go-evm-keeper-keeper-tracedb-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/keeper.go:164</code> | <code>func (k *Keeper) TraceDB() *TraceDB     { return k.traceDB }</code> |
| <code>interface.6.go-evm-keeper-keeper-tracesnapshotstore-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/keeper.go:167</code> | <code>func (k *Keeper) TraceSnapshotStore() *TraceSnapshotStore     { return k.traceSnapshotStore }</code> |
| <code>interface.6.go-evm-keeper-keeper-upgradekeeper-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/keeper.go:403</code> | <code>func (k *Keeper) UpgradeKeeper() *upgradekeeper.Keeper</code> |
| <code>interface.6.go-evm-keeper-keeper-upserterccw20pointer-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/pointer_upgrade.go:59</code> | <code>func (k *Keeper) UpsertERCCW20Pointer(</code> |
| <code>interface.6.go-evm-keeper-keeper-upsertercnativepointer-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/pointer_upgrade.go:49</code> | <code>func (k *Keeper) UpsertERCNativePointer(</code> |
| <code>interface.6.go-evm-keeper-keeper-wasmkeeper-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/keeper.go:399</code> | <code>func (k *Keeper) WasmKeeper() *wasmkeeper.PermissionedKeeper</code> |
| <code>interface.6.go-evm-keeper-keeper-writereceipt-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/receipt.go:243</code> | <code>func (k *Keeper) WriteReceipt(</code> |
| <code>interface.6.go-evm-keeper-newevmaddresshandler-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/address.go:92</code> | <code>func NewEvmAddressHandler(evmKeeper *Keeper) EvmAddressHandler</code> |
| <code>interface.6.go-evm-keeper-newkeeper-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/keeper.go:138</code> | <code>func NewKeeper(</code> |
| <code>interface.6.go-evm-keeper-newmsgserverimpl-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/msg_server.go:46</code> | <code>func NewMsgServerImpl(keeper *Keeper) types.MsgServer</code> |
| <code>interface.6.go-evm-keeper-newtracedb-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/trace_db.go:34</code> | <code>func NewTraceDB(homeDir string) (*TraceDB, error)</code> |
| <code>interface.6.go-evm-keeper-newtracesnapshotstore-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/trace_snapshot.go:29</code> | <code>func NewTraceSnapshotStore(window int64) *TraceSnapshotStore</code> |
| <code>interface.6.go-evm-keeper-querier-staticcall-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/grpc_query.go:66</code> | <code>func (q Querier) StaticCall(c context.Context, req *types.QueryStaticCallRequest) (*types.QueryStaticCallResponse, error)</code> |
| <code>interface.6.go-evm-keeper-syntheticreceiptactivation-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/synthetic_receipts.go:19</code> | <code>SyntheticReceiptActivation SyntheticReceiptKind = 3</code> |
| <code>interface.6.go-evm-keeper-syntheticreceiptcontext-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/synthetic_receipts.go:59</code> | <code>func SyntheticReceiptContext(ctx sdk.Context, kind SyntheticReceiptKind) sdk.Context</code> |
| <code>interface.6.go-evm-keeper-syntheticreceipthash-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/synthetic_receipts.go:40</code> | <code>func SyntheticReceiptHash(kind SyntheticReceiptKind, height int64) common.Hash</code> |
| <code>interface.6.go-evm-keeper-syntheticreceiptkind-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/synthetic_receipts.go:14</code> | <code>type SyntheticReceiptKind uint8</code> |
| <code>interface.6.go-evm-keeper-syntheticreceiptkindof-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/synthetic_receipts.go:64</code> | <code>func SyntheticReceiptKindOf(hash common.Hash, height int64) (SyntheticReceiptKind, bool)</code> |
| <code>interface.6.go-evm-keeper-syntheticreceiptkinds-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/synthetic_receipts.go:23</code> | <code>var SyntheticReceiptKinds = [...]SyntheticReceiptKind{SyntheticReceiptPerps, SyntheticReceiptSpot, SyntheticReceiptActivation}</code> |
| <code>interface.6.go-evm-keeper-syntheticreceiptperps-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/synthetic_receipts.go:17</code> | <code>SyntheticReceiptPerps      SyntheticReceiptKind = 1</code> |
| <code>interface.6.go-evm-keeper-syntheticreceiptprovisionalindex-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/synthetic_receipts.go:53</code> | <code>func SyntheticReceiptProvisionalIndex(kind SyntheticReceiptKind) uint32</code> |
| <code>interface.6.go-evm-keeper-syntheticreceiptspot-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/synthetic_receipts.go:18</code> | <code>SyntheticReceiptSpot       SyntheticReceiptKind = 2</code> |
| <code>interface.6.go-evm-keeper-tracedb-close-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/trace_db.go:43</code> | <code>func (c *TraceDB) Close() error</code> |
| <code>interface.6.go-evm-keeper-tracedb-enqueue-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/trace_db.go:212</code> | <code>func (c *TraceDB) Enqueue(height int64)</code> |
| <code>interface.6.go-evm-keeper-tracedb-get-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/trace_db.go:94</code> | <code>func (c *TraceDB) Get(height int64, tracer string, txHash common.Hash) (json.RawMessage, bool, error)</code> |
| <code>interface.6.go-evm-keeper-tracedb-getblock-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/trace_db.go:118</code> | <code>func (c *TraceDB) GetBlock(height int64, tracer string) (json.RawMessage, bool, error)</code> |
| <code>interface.6.go-evm-keeper-tracedb-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/trace_db.go:21</code> | <code>type TraceDB struct</code> |
| <code>interface.6.go-evm-keeper-tracedb-lastbakedheight-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/trace_db.go:155</code> | <code>func (c *TraceDB) LastBakedHeight() (int64, error)</code> |
| <code>interface.6.go-evm-keeper-tracedb-prune-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/trace_db.go:180</code> | <code>func (c *TraceDB) Prune(belowHeight int64) error</code> |
| <code>interface.6.go-evm-keeper-tracedb-put-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/trace_db.go:87</code> | <code>func (c *TraceDB) Put(height int64, tracer string, txHash common.Hash, value json.RawMessage) error</code> |
| <code>interface.6.go-evm-keeper-tracedb-putblock-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/trace_db.go:111</code> | <code>func (c *TraceDB) PutBlock(height int64, tracer string, value json.RawMessage) error</code> |
| <code>interface.6.go-evm-keeper-tracedb-setlastbakedheight-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/trace_db.go:137</code> | <code>func (c *TraceDB) SetLastBakedHeight(h int64) error</code> |
| <code>interface.6.go-evm-keeper-tracedb-settraceenqueuer-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/trace_db.go:203</code> | <code>func (c *TraceDB) SetTraceEnqueuer(e TraceEnqueuer)</code> |
| <code>interface.6.go-evm-keeper-tracesnapshotstore-close-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/trace_snapshot.go:99</code> | <code>func (s *TraceSnapshotStore) Close()</code> |
| <code>interface.6.go-evm-keeper-tracesnapshotstore-get-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/trace_snapshot.go:89</code> | <code>func (s *TraceSnapshotStore) Get(height int64) sctypes.Committer</code> |
| <code>interface.6.go-evm-keeper-tracesnapshotstore-lease-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/trace_snapshot.go:72</code> | <code>func (s *TraceSnapshotStore) Lease(height int64) (sctypes.Committer, func())</code> |
| <code>interface.6.go-evm-keeper-tracesnapshotstore-put-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/keeper/trace_snapshot.go:43</code> | <code>func (s *TraceSnapshotStore) Put(height int64, snap sctypes.Committer)</code> |
| <code>interface.6.go-evm-module-consensusversion-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/module.go:286</code> | <code>func (AppModule) ConsensusVersion() uint64 { return 22 }</code> |
| <code>interface.6.go-evm-newappmodule-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/module.go:121</code> | <code>func NewAppModule(</code> |
| <code>interface.6.go-evm-newparamchangeproposalhandler-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/gov.go:26</code> | <code>func NewParamChangeProposalHandler(k *keeper.Keeper, paramsKeeper paramskeeper.Keeper) govtypes.Handler</code> |
| <code>interface.6.go-evm-newproposalhandler-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/handler.go:43</code> | <code>func NewProposalHandler(k keeper.Keeper) govtypes.Handler</code> |
| <code>interface.6.go-evm-querier-config-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/querier/config.go:8</code> | <code>type Config struct</code> |
| <code>interface.6.go-evm-querier-defaultconfig-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/querier/config.go:12</code> | <code>var DefaultConfig = Config{</code> |
| <code>interface.6.go-evm-querier-readconfig-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/querier/config.go:20</code> | <code>func ReadConfig(opts servertypes.AppOptions) (Config, error)</code> |
| <code>interface.6.go-evm-replay-config-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/replay/config.go:8</code> | <code>type Config struct</code> |
| <code>interface.6.go-evm-replay-defaultconfig-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/replay/config.go:15</code> | <code>var DefaultConfig = Config{</code> |
| <code>interface.6.go-evm-replay-readconfig-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/replay/config.go:29</code> | <code>func ReadConfig(opts servertypes.AppOptions) (Config, error)</code> |
| <code>interface.6.go-evm-state-dbimpl-addbalance-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/state/balance.go:69</code> | <code>func (s *DBImpl) AddBalance(evmAddr common.Address, amtUint256 *uint256.Int, reason tracing.BalanceChangeReason) uint256.Int</code> |
| <code>interface.6.go-evm-state-dbimpl-addlog-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/state/log.go:12</code> | <code>func (s *DBImpl) AddLog(l *ethtypes.Log)</code> |
| <code>interface.6.go-evm-state-dbimpl-cleanup-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/state/statedb.go:91</code> | <code>func (s *DBImpl) Cleanup()</code> |
| <code>interface.6.go-evm-state-dbimpl-cleanupfortracer-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/state/statedb.go:99</code> | <code>func (s *DBImpl) CleanupForTracer()</code> |
| <code>interface.6.go-evm-state-dbimpl-commit-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/state/statedb.go:210</code> | <code>func (s *DBImpl) Commit(uint64, bool, bool) (common.Hash, error)</code> |
| <code>interface.6.go-evm-state-dbimpl-copy-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/state/statedb.go:182</code> | <code>func (s *DBImpl) Copy() vm.StateDB</code> |
| <code>interface.6.go-evm-state-dbimpl-ctx-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/state/statedb.go:266</code> | <code>func (s *DBImpl) Ctx() sdk.Context</code> |
| <code>interface.6.go-evm-state-dbimpl-empty-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/state/check.go:34</code> | <code>func (s *DBImpl) Empty(addr common.Address) bool</code> |
| <code>interface.6.go-evm-state-dbimpl-err-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/state/statedb.go:258</code> | <code>func (s *DBImpl) Err() error</code> |
| <code>interface.6.go-evm-state-dbimpl-finalize-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/state/statedb.go:128</code> | <code>func (s *DBImpl) Finalize() (surplus sdk.Int, err error)</code> |
| <code>interface.6.go-evm-state-dbimpl-getalllogs-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/state/log.go:22</code> | <code>func (s *DBImpl) GetAllLogs() []*ethtypes.Log</code> |
| <code>interface.6.go-evm-state-dbimpl-getbalance-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/state/balance.go:123</code> | <code>func (s *DBImpl) GetBalance(evmAddr common.Address) *uint256.Int</code> |
| <code>interface.6.go-evm-state-dbimpl-getcode-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/state/code.go:13</code> | <code>func (s *DBImpl) GetCode(addr common.Address) []byte</code> |
| <code>interface.6.go-evm-state-dbimpl-getcodehash-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/state/code.go:8</code> | <code>func (s *DBImpl) GetCodeHash(addr common.Address) common.Hash</code> |
| <code>interface.6.go-evm-state-dbimpl-getcodesize-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/state/code.go:33</code> | <code>func (s *DBImpl) GetCodeSize(addr common.Address) int</code> |
| <code>interface.6.go-evm-state-dbimpl-getnonce-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/state/nonce.go:8</code> | <code>func (s *DBImpl) GetNonce(addr common.Address) uint64</code> |
| <code>interface.6.go-evm-state-dbimpl-getprecompileerror-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/state/statedb.go:253</code> | <code>func (s *DBImpl) GetPrecompileError() error</code> |
| <code>interface.6.go-evm-state-dbimpl-getstate-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/state/state.go:30</code> | <code>func (s *DBImpl) GetState(addr common.Address, hash common.Hash) common.Hash</code> |
| <code>interface.6.go-evm-state-dbimpl-gettransientstate-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/state/state.go:51</code> | <code>func (s *DBImpl) GetTransientState(addr common.Address, key common.Hash) common.Hash</code> |
| <code>interface.6.go-evm-state-dbimpl-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/state/statedb.go:23</code> | <code>type DBImpl struct</code> |
| <code>interface.6.go-evm-state-dbimpl-prepare-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/state/accesslist.go:70</code> | <code>func (s *DBImpl) Prepare(_ params.Rules, sender, coinbase common.Address, dest *common.Address, precompiles []common.Address, txAccesses ethtypes.AccessList)</code> |
| <code>interface.6.go-evm-state-dbimpl-resetfortracer-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/state/statedb.go:118</code> | <code>func (s *DBImpl) ResetForTracer()</code> |
| <code>interface.6.go-evm-state-dbimpl-reverttosnapshot-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/state/state.go:118</code> | <code>func (s *DBImpl) RevertToSnapshot(rev int)</code> |
| <code>interface.6.go-evm-state-dbimpl-setcode-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/state/code.go:18</code> | <code>func (s *DBImpl) SetCode(addr common.Address, code []byte) []byte</code> |
| <code>interface.6.go-evm-state-dbimpl-setfeetokencharge-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/state/balance.go:197</code> | <code>func (s *DBImpl) SetFeeTokenCharge(charge *FeeTokenCharge, gasBought bool)</code> |
| <code>interface.6.go-evm-state-dbimpl-setlogger-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/state/statedb.go:78</code> | <code>func (s *DBImpl) SetLogger(logger *tracing.Hooks)</code> |
| <code>interface.6.go-evm-state-dbimpl-setnonce-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/state/nonce.go:13</code> | <code>func (s *DBImpl) SetNonce(addr common.Address, nonce uint64, reason tracing.NonceChangeReason)</code> |
| <code>interface.6.go-evm-state-dbimpl-setprecompileerror-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/state/statedb.go:249</code> | <code>func (s *DBImpl) SetPrecompileError(err error)</code> |
| <code>interface.6.go-evm-state-dbimpl-setstate-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/state/state.go:39</code> | <code>func (s *DBImpl) SetState(addr common.Address, key common.Hash, val common.Hash) common.Hash</code> |
| <code>interface.6.go-evm-state-dbimpl-settransientstate-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/state/state.go:59</code> | <code>func (s *DBImpl) SetTransientState(addr common.Address, key, val common.Hash)</code> |
| <code>interface.6.go-evm-state-dbimpl-settxcontext-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/state/statedb.go:214</code> | <code>func (s *DBImpl) SetTxContext(common.Hash, int)</code> |
| <code>interface.6.go-evm-state-dbimpl-snapshot-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/state/state.go:109</code> | <code>func (s *DBImpl) Snapshot() int</code> |
| <code>interface.6.go-evm-state-dbimpl-subbalance-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/state/balance.go:16</code> | <code>func (s *DBImpl) SubBalance(evmAddr common.Address, amtUint256 *uint256.Int, reason tracing.BalanceChangeReason) uint256.Int</code> |
| <code>interface.6.go-evm-state-dbimpl-txindex-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/state/statedb.go:241</code> | <code>func (s *DBImpl) TxIndex() int</code> |
| <code>interface.6.go-evm-state-dbimpl-withctx-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/state/statedb.go:270</code> | <code>func (s *DBImpl) WithCtx(ctx sdk.Context)</code> |
| <code>interface.6.go-evm-state-evmkeeper-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/state/expected_keepers.go:13</code> | <code>type EVMKeeper interface</code> |
| <code>interface.6.go-evm-state-feetokencharge-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/state/balance.go:191</code> | <code>type FeeTokenCharge struct</code> |
| <code>interface.6.go-evm-state-getcoinbaseaddress-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/state/utils.go:17</code> | <code>func GetCoinbaseAddress(txIdx int) sdk.AccAddress</code> |
| <code>interface.6.go-evm-state-getdbimpl-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/state/statedb.go:331</code> | <code>func GetDBImpl(vmsdb vm.StateDB) *DBImpl</code> |
| <code>interface.6.go-evm-state-newdbimpl-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/state/statedb.go:54</code> | <code>func NewDBImpl(ctx sdk.Context, k EVMKeeper, simulation bool) *DBImpl</code> |
| <code>interface.6.go-evm-state-sdkuhpxtosweimultiplier-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/state/utils.go:13</code> | <code>var SdkUhpxToSweiMultiplier = sdk.NewIntFromBigInt(UhpxToSweiMultiplier)</code> |
| <code>interface.6.go-evm-state-splituhpxweiamount-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/state/utils.go:23</code> | <code>func SplitUhpxWeiAmount(amt *big.Int) (sdk.Int, sdk.Int)</code> |
| <code>interface.6.go-evm-state-uhpxtosweimultiplier-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/state/utils.go:12</code> | <code>var UhpxToSweiMultiplier = big.NewInt(1_000_000_000_000)</code> |
| <code>interface.6.go-evm-types-accountfeedenomkey-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/keys.go:190</code> | <code>func AccountFeeDenomKey(evmAddress common.Address) []byte</code> |
| <code>interface.6.go-evm-types-associationmissingerr-addresstype-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/errors.go:20</code> | <code>func (e AssociationMissingErr) AddressType() string</code> |
| <code>interface.6.go-evm-types-associationmissingerr-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/errors.go:8</code> | <code>type AssociationMissingErr struct</code> |
| <code>interface.6.go-evm-types-attributekeyevmaddress-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/events.go:11</code> | <code>AttributeKeyEvmAddress     = "evm_addr"</code> |
| <code>interface.6.go-evm-types-attributekeypaxaddress-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/events.go:10</code> | <code>AttributeKeyPaxAddress     = "pax_addr"</code> |
| <code>interface.6.go-evm-types-attributekeypointee-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/events.go:13</code> | <code>AttributeKeyPointee        = "pointee"</code> |
| <code>interface.6.go-evm-types-attributekeypointeraddress-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/events.go:14</code> | <code>AttributeKeyPointerAddress = "pointer_address"</code> |
| <code>interface.6.go-evm-types-attributekeypointertype-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/events.go:12</code> | <code>AttributeKeyPointerType    = "pointer_type"</code> |
| <code>interface.6.go-evm-types-attributekeypointerversion-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/events.go:15</code> | <code>AttributeKeyPointerVersion = "pointer_version"</code> |
| <code>interface.6.go-evm-types-cancuntime-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/config.go:11</code> | <code>var CancunTime int64 = 0</code> |
| <code>interface.6.go-evm-types-chainconfig-ethereumconfig-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/config.go:23</code> | <code>func (cc ChainConfig) EthereumConfig(chainID *big.Int) *params.ChainConfig</code> |
| <code>interface.6.go-evm-types-chainconfig-ethereumconfigwithsstore-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/config.go:51</code> | <code>func (cc ChainConfig) EthereumConfigWithSstore(chainID *big.Int, sstoreSetGasEIP2200 *uint64) *params.ChainConfig</code> |
| <code>interface.6.go-evm-types-chainconfig-validate-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/config.go:74</code> | <code>func (cc ChainConfig) Validate() error</code> |
| <code>interface.6.go-evm-types-defaultchainconfig-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/config.go:57</code> | <code>func DefaultChainConfig() ChainConfig</code> |
| <code>interface.6.go-evm-types-defaultfeetokendistribution-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/params.go:44</code> | <code>var DefaultFeeTokenDistribution = false</code> |
| <code>interface.6.go-evm-types-defaultfeetokenenabled-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/params.go:43</code> | <code>var DefaultFeeTokenEnabled = false</code> |
| <code>interface.6.go-evm-types-defaultgenesis-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/genesis.go:3</code> | <code>func DefaultGenesis() *GenesisState</code> |
| <code>interface.6.go-evm-types-defaultmaxfeetokenrateage-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/params.go:39</code> | <code>const DefaultMaxFeeTokenRateAge int64 = 1000</code> |
| <code>interface.6.go-evm-types-defaultmaxfeetokenspread-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/params.go:42</code> | <code>var DefaultMaxFeeTokenSpread = sdk.NewDecWithPrec(5, 2)</code> |
| <code>interface.6.go-evm-types-defaultminfeepergas-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/params.go:52</code> | <code>var DefaultMinFeePerGas = sdk.NewDec(67000000) // 0.067gwei</code> |
| <code>interface.6.go-evm-types-defaultparams-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/params.go:74</code> | <code>func DefaultParams() Params</code> |
| <code>interface.6.go-evm-types-defaultpaxsstoresetgaseip2200-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/params.go:66</code> | <code>var DefaultPaxSstoreSetGasEIP2200 = uint64(20000) // 20k</code> |
| <code>interface.6.go-evm-types-ethtx-accesslisttx-asethereumdata-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/ethtx/access_list_tx.go:113</code> | <code>func (tx *AccessListTx) AsEthereumData() ethtypes.TxData</code> |
| <code>interface.6.go-evm-types-ethtx-accesslisttx-effectivegasprice-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/ethtx/access_list_tx.go:213</code> | <code>func (tx AccessListTx) EffectiveGasPrice(_ *big.Int) *big.Int</code> |
| <code>interface.6.go-evm-types-ethtx-accesslisttx-getblobfeecap-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/ethtx/access_list_tx.go:150</code> | <code>func (tx *AccessListTx) GetBlobFeeCap() *big.Int</code> |
| <code>interface.6.go-evm-types-ethtx-accesslisttx-getblobhashes-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/ethtx/access_list_tx.go:154</code> | <code>func (tx *AccessListTx) GetBlobHashes() []common.Hash</code> |
| <code>interface.6.go-evm-types-ethtx-accesslisttx-getchainid-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/ethtx/access_list_tx.go:57</code> | <code>func (tx *AccessListTx) GetChainID() *big.Int</code> |
| <code>interface.6.go-evm-types-ethtx-accesslisttx-getgas-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/ethtx/access_list_tx.go:76</code> | <code>func (tx *AccessListTx) GetGas() uint64</code> |
| <code>interface.6.go-evm-types-ethtx-accesslisttx-getgasfeecap-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/ethtx/access_list_tx.go:91</code> | <code>func (tx *AccessListTx) GetGasFeeCap() *big.Int</code> |
| <code>interface.6.go-evm-types-ethtx-accesslisttx-getgastipcap-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/ethtx/access_list_tx.go:87</code> | <code>func (tx *AccessListTx) GetGasTipCap() *big.Int</code> |
| <code>interface.6.go-evm-types-ethtx-accesslisttx-getnonce-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/ethtx/access_list_tx.go:103</code> | <code>func (tx *AccessListTx) GetNonce() uint64 { return tx.Nonce }</code> |
| <code>interface.6.go-evm-types-ethtx-accesslisttx-getrawsignaturevalues-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/ethtx/access_list_tx.go:130</code> | <code>func (tx *AccessListTx) GetRawSignatureValues() (v, r, s *big.Int)</code> |
| <code>interface.6.go-evm-types-ethtx-accesslisttx-validate-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/ethtx/access_list_tx.go:158</code> | <code>func (tx AccessListTx) Validate() error</code> |
| <code>interface.6.go-evm-types-ethtx-associatetx-getrawsignaturevalues-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/ethtx/associate_tx.go:29</code> | <code>func (tx *AssociateTx) GetRawSignatureValues() (v, r, s *big.Int)</code> |
| <code>interface.6.go-evm-types-ethtx-associatetx-validate-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/ethtx/associate_tx.go:32</code> | <code>func (tx *AssociateTx) Validate() error</code> |
| <code>interface.6.go-evm-types-ethtx-authlist-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/ethtx/auth_list.go:12</code> | <code>type AuthList []SetCodeAuthorization</code> |
| <code>interface.6.go-evm-types-ethtx-blobtx-asethereumdata-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/ethtx/blob_tx.go:152</code> | <code>func (tx *BlobTx) AsEthereumData() ethtypes.TxData</code> |
| <code>interface.6.go-evm-types-ethtx-blobtx-blobgas-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/ethtx/blob_tx.go:92</code> | <code>func (tx *BlobTx) BlobGas() uint64</code> |
| <code>interface.6.go-evm-types-ethtx-blobtx-effectivegasprice-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/ethtx/blob_tx.go:268</code> | <code>func (tx *BlobTx) EffectiveGasPrice(baseFee *big.Int) *big.Int</code> |
| <code>interface.6.go-evm-types-ethtx-blobtx-getblobfeecap-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/ethtx/blob_tx.go:132</code> | <code>func (tx *BlobTx) GetBlobFeeCap() *big.Int</code> |
| <code>interface.6.go-evm-types-ethtx-blobtx-getblobhashes-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/ethtx/blob_tx.go:140</code> | <code>func (tx *BlobTx) GetBlobHashes() []common.Hash</code> |
| <code>interface.6.go-evm-types-ethtx-blobtx-getchainid-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/ethtx/blob_tx.go:69</code> | <code>func (tx *BlobTx) GetChainID() *big.Int</code> |
| <code>interface.6.go-evm-types-ethtx-blobtx-getgas-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/ethtx/blob_tx.go:88</code> | <code>func (tx *BlobTx) GetGas() uint64</code> |
| <code>interface.6.go-evm-types-ethtx-blobtx-getgasfeecap-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/ethtx/blob_tx.go:107</code> | <code>func (tx *BlobTx) GetGasFeeCap() *big.Int</code> |
| <code>interface.6.go-evm-types-ethtx-blobtx-getgastipcap-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/ethtx/blob_tx.go:100</code> | <code>func (tx *BlobTx) GetGasTipCap() *big.Int</code> |
| <code>interface.6.go-evm-types-ethtx-blobtx-getnonce-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/ethtx/blob_tx.go:122</code> | <code>func (tx *BlobTx) GetNonce() uint64 { return tx.Nonce }</code> |
| <code>interface.6.go-evm-types-ethtx-blobtx-getrawsignaturevalues-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/ethtx/blob_tx.go:173</code> | <code>func (tx *BlobTx) GetRawSignatureValues() (v, r, s *big.Int)</code> |
| <code>interface.6.go-evm-types-ethtx-blobtx-validate-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/ethtx/blob_tx.go:193</code> | <code>func (tx BlobTx) Validate() error</code> |
| <code>interface.6.go-evm-types-ethtx-dynamicfeetx-asethereumdata-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/ethtx/dynamic_fee_tx.go:115</code> | <code>func (tx *DynamicFeeTx) AsEthereumData() ethtypes.TxData</code> |
| <code>interface.6.go-evm-types-ethtx-dynamicfeetx-effectivegasprice-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/ethtx/dynamic_fee_tx.go:232</code> | <code>func (tx *DynamicFeeTx) EffectiveGasPrice(baseFee *big.Int) *big.Int</code> |
| <code>interface.6.go-evm-types-ethtx-dynamicfeetx-getblobfeecap-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/ethtx/dynamic_fee_tx.go:153</code> | <code>func (tx *DynamicFeeTx) GetBlobFeeCap() *big.Int</code> |
| <code>interface.6.go-evm-types-ethtx-dynamicfeetx-getblobhashes-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/ethtx/dynamic_fee_tx.go:157</code> | <code>func (tx *DynamicFeeTx) GetBlobHashes() []common.Hash</code> |
| <code>interface.6.go-evm-types-ethtx-dynamicfeetx-getchainid-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/ethtx/dynamic_fee_tx.go:56</code> | <code>func (tx *DynamicFeeTx) GetChainID() *big.Int</code> |
| <code>interface.6.go-evm-types-ethtx-dynamicfeetx-getgas-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/ethtx/dynamic_fee_tx.go:75</code> | <code>func (tx *DynamicFeeTx) GetGas() uint64</code> |
| <code>interface.6.go-evm-types-ethtx-dynamicfeetx-getgasfeecap-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/ethtx/dynamic_fee_tx.go:90</code> | <code>func (tx *DynamicFeeTx) GetGasFeeCap() *big.Int</code> |
| <code>interface.6.go-evm-types-ethtx-dynamicfeetx-getgastipcap-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/ethtx/dynamic_fee_tx.go:83</code> | <code>func (tx *DynamicFeeTx) GetGasTipCap() *big.Int</code> |
| <code>interface.6.go-evm-types-ethtx-dynamicfeetx-getnonce-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/ethtx/dynamic_fee_tx.go:105</code> | <code>func (tx *DynamicFeeTx) GetNonce() uint64 { return tx.Nonce }</code> |
| <code>interface.6.go-evm-types-ethtx-dynamicfeetx-getrawsignaturevalues-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/ethtx/dynamic_fee_tx.go:133</code> | <code>func (tx *DynamicFeeTx) GetRawSignatureValues() (v, r, s *big.Int)</code> |
| <code>interface.6.go-evm-types-ethtx-dynamicfeetx-validate-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/ethtx/dynamic_fee_tx.go:161</code> | <code>func (tx DynamicFeeTx) Validate() error</code> |
| <code>interface.6.go-evm-types-ethtx-ethereumtxdata-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/ethtx/txdata.go:37</code> | <code>type EthereumTxData interface</code> |
| <code>interface.6.go-evm-types-ethtx-legacytx-asethereumdata-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/ethtx/legacy_tx.go:109</code> | <code>func (tx *LegacyTx) AsEthereumData() ethtypes.TxData</code> |
| <code>interface.6.go-evm-types-ethtx-legacytx-effectivegasprice-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/ethtx/legacy_tx.go:202</code> | <code>func (tx LegacyTx) EffectiveGasPrice(_ *big.Int) *big.Int</code> |
| <code>interface.6.go-evm-types-ethtx-legacytx-getblobfeecap-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/ethtx/legacy_tx.go:140</code> | <code>func (tx *LegacyTx) GetBlobFeeCap() *big.Int</code> |
| <code>interface.6.go-evm-types-ethtx-legacytx-getblobhashes-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/ethtx/legacy_tx.go:144</code> | <code>func (tx *LegacyTx) GetBlobHashes() []common.Hash</code> |
| <code>interface.6.go-evm-types-ethtx-legacytx-getchainid-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/ethtx/legacy_tx.go:52</code> | <code>func (tx *LegacyTx) GetChainID() *big.Int</code> |
| <code>interface.6.go-evm-types-ethtx-legacytx-getgas-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/ethtx/legacy_tx.go:73</code> | <code>func (tx *LegacyTx) GetGas() uint64</code> |
| <code>interface.6.go-evm-types-ethtx-legacytx-getgasfeecap-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/ethtx/legacy_tx.go:88</code> | <code>func (tx *LegacyTx) GetGasFeeCap() *big.Int</code> |
| <code>interface.6.go-evm-types-ethtx-legacytx-getgastipcap-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/ethtx/legacy_tx.go:84</code> | <code>func (tx *LegacyTx) GetGasTipCap() *big.Int</code> |
| <code>interface.6.go-evm-types-ethtx-legacytx-getnonce-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/ethtx/legacy_tx.go:99</code> | <code>func (tx *LegacyTx) GetNonce() uint64 { return tx.Nonce }</code> |
| <code>interface.6.go-evm-types-ethtx-legacytx-getrawsignaturevalues-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/ethtx/legacy_tx.go:124</code> | <code>func (tx *LegacyTx) GetRawSignatureValues() (v, r, s *big.Int)</code> |
| <code>interface.6.go-evm-types-ethtx-legacytx-validate-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/ethtx/legacy_tx.go:148</code> | <code>func (tx *LegacyTx) Validate() error</code> |
| <code>interface.6.go-evm-types-ethtx-newaccesslisttx-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/ethtx/access_list_tx.go:14</code> | <code>func NewAccessListTx(tx *ethtypes.Transaction) (*AccessListTx, error)</code> |
| <code>interface.6.go-evm-types-ethtx-newblobtx-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/ethtx/blob_tx.go:17</code> | <code>func NewBlobTx(tx *ethtypes.Transaction) (*BlobTx, error)</code> |
| <code>interface.6.go-evm-types-ethtx-newdynamicfeetx-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/ethtx/dynamic_fee_tx.go:13</code> | <code>func NewDynamicFeeTx(tx *ethtypes.Transaction) (*DynamicFeeTx, error)</code> |
| <code>interface.6.go-evm-types-ethtx-newlegacytx-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/ethtx/legacy_tx.go:14</code> | <code>func NewLegacyTx(tx *ethtypes.Transaction) (*LegacyTx, error)</code> |
| <code>interface.6.go-evm-types-ethtx-newsetcodetx-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/ethtx/set_code_tx.go:14</code> | <code>func NewSetCodeTx(tx *ethtypes.Transaction) (*SetCodeTx, error)</code> |
| <code>interface.6.go-evm-types-ethtx-newtxdatafromtx-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/ethtx/txdata.go:66</code> | <code>func NewTxDataFromTx(tx *ethtypes.Transaction) (EthereumTxData, error)</code> |
| <code>interface.6.go-evm-types-ethtx-setcodetx-asethereumdata-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/ethtx/set_code_tx.go:125</code> | <code>func (tx *SetCodeTx) AsEthereumData() ethtypes.TxData</code> |
| <code>interface.6.go-evm-types-ethtx-setcodetx-effectivegasprice-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/ethtx/set_code_tx.go:255</code> | <code>func (tx *SetCodeTx) EffectiveGasPrice(baseFee *big.Int) *big.Int</code> |
| <code>interface.6.go-evm-types-ethtx-setcodetx-getblobfeecap-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/ethtx/set_code_tx.go:173</code> | <code>func (tx *SetCodeTx) GetBlobFeeCap() *big.Int</code> |
| <code>interface.6.go-evm-types-ethtx-setcodetx-getblobhashes-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/ethtx/set_code_tx.go:177</code> | <code>func (tx *SetCodeTx) GetBlobHashes() []common.Hash</code> |
| <code>interface.6.go-evm-types-ethtx-setcodetx-getchainid-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/ethtx/set_code_tx.go:59</code> | <code>func (tx *SetCodeTx) GetChainID() *big.Int</code> |
| <code>interface.6.go-evm-types-ethtx-setcodetx-getgas-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/ethtx/set_code_tx.go:85</code> | <code>func (tx *SetCodeTx) GetGas() uint64</code> |
| <code>interface.6.go-evm-types-ethtx-setcodetx-getgasfeecap-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/ethtx/set_code_tx.go:100</code> | <code>func (tx *SetCodeTx) GetGasFeeCap() *big.Int</code> |
| <code>interface.6.go-evm-types-ethtx-setcodetx-getgastipcap-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/ethtx/set_code_tx.go:93</code> | <code>func (tx *SetCodeTx) GetGasTipCap() *big.Int</code> |
| <code>interface.6.go-evm-types-ethtx-setcodetx-getnonce-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/ethtx/set_code_tx.go:115</code> | <code>func (tx *SetCodeTx) GetNonce() uint64 { return tx.Nonce }</code> |
| <code>interface.6.go-evm-types-ethtx-setcodetx-getrawsignaturevalues-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/ethtx/set_code_tx.go:153</code> | <code>func (tx *SetCodeTx) GetRawSignatureValues() (v, r, s *big.Int)</code> |
| <code>interface.6.go-evm-types-ethtx-setcodetx-validate-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/ethtx/set_code_tx.go:181</code> | <code>func (tx SetCodeTx) Validate() error</code> |
| <code>interface.6.go-evm-types-feetokenholdingaccount-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/params.go:12</code> | <code>const FeeTokenHoldingAccount = "fee_token_holding"</code> |
| <code>interface.6.go-evm-types-genesisstate-validate-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/genesis.go:9</code> | <code>func (gs GenesisState) Validate() error</code> |
| <code>interface.6.go-evm-types-getevmtransactionmessage-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/message_evm_transaction.go:110</code> | <code>func GetEVMTransactionMessage(tx sdk.Tx) *MsgEVMTransaction</code> |
| <code>interface.6.go-evm-types-initialsidiorabaseunitsperpax-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/params.go:36</code> | <code>const InitialSidioraBaseUnitsPerPax int64 = 3_114_000</code> |
| <code>interface.6.go-evm-types-istxmsgassociate-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/message_associate.go:50</code> | <code>func IsTxMsgAssociate(tx sdk.Tx) bool</code> |
| <code>interface.6.go-evm-types-layerxbindmessage-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/layerx_binding.go:43</code> | <code>func LayerXBindMessage(chainID *big.Int, evmAddress common.Address, nonce uint64) ([]byte, error)</code> |
| <code>interface.6.go-evm-types-layerxbindnoncekey-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/keys.go:100</code> | <code>func LayerXBindNonceKey(evmAddress common.Address) []byte</code> |
| <code>interface.6.go-evm-types-layerxdid-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/layerx_binding.go:58</code> | <code>func LayerXDid(didPublicKey [32]byte) string</code> |
| <code>interface.6.go-evm-types-layerxdidprefix-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/layerx_binding.go:21</code> | <code>LayerXDidPrefix = "did:layerx:"</code> |
| <code>interface.6.go-evm-types-layerxmainaccountid-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/layerx_binding.go:69</code> | <code>func LayerXMainAccountID(didPublicKey [32]byte) ([32]byte, error)</code> |
| <code>interface.6.go-evm-types-layerxmainaccountname-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/layerx_binding.go:63</code> | <code>func LayerXMainAccountName(didPublicKey [32]byte) string</code> |
| <code>interface.6.go-evm-types-maxminfeepergas-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/params.go:55</code> | <code>var MaxMinFeePerGas = sdk.NewDec(67000000)</code> |
| <code>interface.6.go-evm-types-modulename-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/keys.go:14</code> | <code>ModuleName = "evm"</code> |
| <code>interface.6.go-evm-types-msgevmtransaction-astransaction-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/message_evm_transaction.go:67</code> | <code>func (msg *MsgEVMTransaction) AsTransaction() (*ethtypes.Transaction, ethtx.TxData)</code> |
| <code>interface.6.go-evm-types-msgevmtransaction-isassociatetx-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/message_evm_transaction.go:85</code> | <code>func (msg *MsgEVMTransaction) IsAssociateTx() bool</code> |
| <code>interface.6.go-evm-types-mustgetevmtransactionmessage-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/message_evm_transaction.go:99</code> | <code>func MustGetEVMTransactionMessage(tx sdk.Tx) *MsgEVMTransaction</code> |
| <code>interface.6.go-evm-types-newassociationmissingerr-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/errors.go:12</code> | <code>func NewAssociationMissingErr(address string) AssociationMissingErr</code> |
| <code>interface.6.go-evm-types-newlogsfrometh-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/logs.go:5</code> | <code>func NewLogsFromEth(ethlogs []*ethtypes.Log) []*Log</code> |
| <code>interface.6.go-evm-types-newmsgclaim-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/message_claim.go:15</code> | <code>func NewMsgClaim(sender sdk.AccAddress, claimer common.Address) *MsgClaim</code> |
| <code>interface.6.go-evm-types-newmsgclaimspecific-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/message_claim_specific.go:16</code> | <code>func NewMsgClaimSpecific(sender sdk.AccAddress, claimer common.Address, assets ...*Asset) *MsgClaimSpecific</code> |
| <code>interface.6.go-evm-types-newmsgevmtransaction-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/message_evm_transaction.go:20</code> | <code>func NewMsgEVMTransaction(txData proto.Message) (*MsgEVMTransaction, error)</code> |
| <code>interface.6.go-evm-types-params-validate-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/params.go:162</code> | <code>func (p Params) Validate() error</code> |
| <code>interface.6.go-evm-types-pointerbindingproposal-getmessages-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/gov.go:417</code> | <code>func (p *PointerBindingProposal) GetMessages() ([]sdk.Msg, error)</code> |
| <code>interface.6.go-evm-types-receiptkey-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/keys.go:108</code> | <code>func ReceiptKey(txHash common.Hash) []byte</code> |
| <code>interface.6.go-evm-types-receiptstorekey-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/keys.go:23</code> | <code>ReceiptStoreKey = "receipt"</code> |
| <code>interface.6.go-evm-types-registerinterfaces-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/codec.go:45</code> | <code>func RegisterInterfaces(registry codectypes.InterfaceRegistry)</code> |
| <code>interface.6.go-evm-types-routerkey-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/keys.go:16</code> | <code>RouterKey = ModuleName</code> |
| <code>interface.6.go-evm-types-shellevmtxtype-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/constants.go:6</code> | <code>const ShellEVMTxType = math.MaxUint32</code> |
| <code>interface.6.go-evm-types-statekey-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/keys.go:104</code> | <code>func StateKey(evmAddress common.Address) []byte</code> |
| <code>interface.6.go-evm-types-storekey-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/keys.go:19</code> | <code>StoreKey = "evm"</code> |
| <code>interface.6.go-evm-types-transientstorekey-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/keys.go:21</code> | <code>TransientStoreKey = "evm_transient"</code> |
| <code>interface.6.go-evm-types-txhasheskey-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/keys.go:131</code> | <code>func TxHashesKey(height int64) []byte</code> |
| <code>interface.6.go-evm-types-unpacktxdata-j34</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/evm/types/codec.go:96</code> | <code>func UnpackTxData(any *codectypes.Any) (ethtx.TxData, error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.6.go-evm-ante-basicdecorator-antehandle-j34</code> | notes | method AnteHandle on BasicDecorator; 2 in-repo uses outside its package tree, 0 outside tests; first 2 listed |
| <code>interface.6.go-evm-ante-evmaddressdecorator-antehandle-j34</code> | notes | method AnteHandle on EVMAddressDecorator; 2 in-repo uses outside its package tree, 0 outside tests; first 2 listed |
| <code>interface.6.go-evm-ante-evmfeecheckdecorator-antehandle-j34</code> | notes | method AnteHandle on EVMFeeCheckDecorator; 2 in-repo uses outside its package tree, 0 outside tests; first 2 listed |
| <code>interface.6.go-evm-ante-evmnocosmosfieldsdecorator-antehandle-j34</code> | notes | method AnteHandle on EVMNoCosmosFieldsDecorator; 2 in-repo uses outside its package tree, 0 outside tests; first 2 listed |
| <code>interface.6.go-evm-ante-evmpreprocessdecorator-antehandle-j34</code> | notes | method AnteHandle on EVMPreprocessDecorator; 2 in-repo uses outside its package tree, 0 outside tests; first 2 listed |
| <code>interface.6.go-evm-ante-evmrouterdecorator-antehandle-j34</code> | notes | method AnteHandle on EVMRouterDecorator; 2 in-repo uses outside its package tree, 0 outside tests; first 2 listed |
| <code>interface.6.go-evm-ante-evmsigverifydecorator-antehandle-j34</code> | notes | method AnteHandle on EVMSigVerifyDecorator; 2 in-repo uses outside its package tree, 0 outside tests; first 2 listed |
| <code>interface.6.go-evm-ante-gasdecorator-antehandle-j34</code> | notes | method AnteHandle on GasDecorator; 2 in-repo uses outside its package tree, 0 outside tests; first 2 listed |
| <code>interface.6.go-evm-ante-getversion-j34</code> | notes | func GetVersion; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-ante-isevmmessage-j34</code> | notes | func IsEVMMessage; 7 in-repo uses outside its package tree, 7 outside tests; first 7 listed |
| <code>interface.6.go-evm-ante-istxtypeallowed-j34</code> | notes | func IsTxTypeAllowed; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-ante-mingasevmtx-j34</code> | notes | const MinGasEVMTx; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-ante-newbasicdecorator-j34</code> | notes | func NewBasicDecorator; 2 in-repo uses outside its package tree, 2 outside tests; first 2 listed |
| <code>interface.6.go-evm-ante-newevmaddressdecorator-j34</code> | notes | func NewEVMAddressDecorator; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-ante-newevmfeecheckdecorator-j34</code> | notes | func NewEVMFeeCheckDecorator; 3 in-repo uses outside its package tree, 1 outside tests; first 3 listed |
| <code>interface.6.go-evm-ante-newevmnocosmosfieldsdecorator-j34</code> | notes | func NewEVMNoCosmosFieldsDecorator; 2 in-repo uses outside its package tree, 2 outside tests; first 2 listed |
| <code>interface.6.go-evm-ante-newevmpreprocessdecorator-j34</code> | notes | func NewEVMPreprocessDecorator; 2 in-repo uses outside its package tree, 2 outside tests; first 2 listed |
| <code>interface.6.go-evm-ante-newevmrouterdecorator-j34</code> | notes | func NewEVMRouterDecorator; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-ante-newevmsigverifydecorator-j34</code> | notes | func NewEVMSigVerifyDecorator; 2 in-repo uses outside its package tree, 2 outside tests; first 2 listed |
| <code>interface.6.go-evm-ante-newgasdecorator-j34</code> | notes | func NewGasDecorator; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-ante-preprocess-j34</code> | notes | func Preprocess; 24 in-repo uses outside its package tree, 1 outside tests; first 12 listed |
| <code>interface.6.go-evm-ante-preprocessunpacked-j34</code> | notes | func PreprocessUnpacked; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-ante-recoversenderfromethtx-j34</code> | notes | func RecoverSenderFromEthTx; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-ante-signermap-j34</code> | notes | var SignerMap; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-ante-validatenocosmostxfields-j34</code> | notes | func ValidateNoCosmosTxFields; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-appmodule-initgenesis-j34</code> | notes | method InitGenesis on AppModule; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-appmodule-j34</code> | notes | type AppModule; 2 in-repo uses outside its package tree, 0 outside tests; first 2 listed |
| <code>interface.6.go-evm-appmodule-registerservices-j34</code> | notes | method RegisterServices on AppModule; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-appmodulebasic-j34</code> | notes | type AppModuleBasic; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-artifacts-cw1155-currentversion-j34</code> | notes | const CurrentVersion; 1 in-repo uses outside its package tree, 0 outside tests; first 1 listed |
| <code>interface.6.go-evm-artifacts-cw1155-getparsedabi-j34</code> | notes | func GetParsedABI; 3 in-repo uses outside its package tree, 1 outside tests; first 3 listed |
| <code>interface.6.go-evm-artifacts-cw20-currentversion-j34</code> | notes | func CurrentVersion; 7 in-repo uses outside its package tree, 6 outside tests; first 7 listed |
| <code>interface.6.go-evm-artifacts-cw20-getbin-j34</code> | notes | func GetBin; 3 in-repo uses outside its package tree, 3 outside tests; first 3 listed |
| <code>interface.6.go-evm-artifacts-cw20-getparsedabi-j34</code> | notes | func GetParsedABI; 3 in-repo uses outside its package tree, 3 outside tests; first 3 listed |
| <code>interface.6.go-evm-artifacts-cw20-setversionwithoffset-j34</code> | notes | func SetVersionWithOffset; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-artifacts-cw721-currentversion-j34</code> | notes | const CurrentVersion; 7 in-repo uses outside its package tree, 6 outside tests; first 7 listed |
| <code>interface.6.go-evm-artifacts-cw721-getbin-j34</code> | notes | func GetBin; 3 in-repo uses outside its package tree, 3 outside tests; first 3 listed |
| <code>interface.6.go-evm-artifacts-cw721-getparsedabi-j34</code> | notes | func GetParsedABI; 3 in-repo uses outside its package tree, 3 outside tests; first 3 listed |
| <code>interface.6.go-evm-artifacts-getparsedabi-j34</code> | notes | func GetParsedABI; 2 in-repo uses outside its package tree, 0 outside tests; first 2 listed |
| <code>interface.6.go-evm-artifacts-native-currentversion-j34</code> | notes | const CurrentVersion; 8 in-repo uses outside its package tree, 6 outside tests; first 8 listed |
| <code>interface.6.go-evm-artifacts-native-getbin-j34</code> | notes | func GetBin; 3 in-repo uses outside its package tree, 3 outside tests; first 3 listed |
| <code>interface.6.go-evm-artifacts-native-getparsedabi-j34</code> | notes | func GetParsedABI; 7 in-repo uses outside its package tree, 3 outside tests; first 7 listed |
| <code>interface.6.go-evm-artifacts-native-nativeapprovaliterator-close-j34</code> | notes | method Close on NativeApprovalIterator; 1 in-repo uses outside its package tree, 0 outside tests; first 1 listed |
| <code>interface.6.go-evm-artifacts-native-nativetransferiterator-close-j34</code> | notes | method Close on NativeTransferIterator; 1 in-repo uses outside its package tree, 0 outside tests; first 1 listed |
| <code>interface.6.go-evm-basic-registergrpcgatewayroutes-j34</code> | notes | method RegisterGRPCGatewayRoutes on Basic; 3 in-repo uses outside its package tree, 3 outside tests; first 3 listed |
| <code>interface.6.go-evm-basic-registerrestroutes-j34</code> | notes | method RegisterRESTRoutes on Basic; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-blocktest-config-j34</code> | notes | type Config; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-blocktest-defaultconfig-j34</code> | notes | var DefaultConfig; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-blocktest-readconfig-j34</code> | notes | func ReadConfig; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-client-wasm-bindings-erc1155balanceofbatchtype-j34</code> | notes | const ERC1155BalanceOfBatchType; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-client-wasm-bindings-erc1155balanceoftype-j34</code> | notes | const ERC1155BalanceOfType; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-client-wasm-bindings-erc1155batchtransfertype-j34</code> | notes | const ERC1155BatchTransferType; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-client-wasm-bindings-erc1155isapprovedforalltype-j34</code> | notes | const ERC1155IsApprovedForAllType; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-client-wasm-bindings-erc1155namesymboltype-j34</code> | notes | const ERC1155NameSymbolType; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-client-wasm-bindings-erc1155royaltyinfotype-j34</code> | notes | const ERC1155RoyaltyInfoType; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-client-wasm-bindings-erc1155setapprovalalltype-j34</code> | notes | const ERC1155SetApprovalAllType; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-client-wasm-bindings-erc1155tokenexiststype-j34</code> | notes | const ERC1155TokenExistsType; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-client-wasm-bindings-erc1155totalsupplyfortokentype-j34</code> | notes | const ERC1155TotalSupplyForTokenType; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-client-wasm-bindings-erc1155totalsupplytype-j34</code> | notes | const ERC1155TotalSupplyType; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-client-wasm-bindings-erc1155transfertype-j34</code> | notes | const ERC1155TransferType; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-client-wasm-bindings-erc1155uritype-j34</code> | notes | const ERC1155UriType; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-client-wasm-bindings-erc20allowancetype-j34</code> | notes | const ERC20AllowanceType; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-client-wasm-bindings-erc20approvetype-j34</code> | notes | const ERC20ApproveType; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-client-wasm-bindings-erc20balancetype-j34</code> | notes | const ERC20BalanceType; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-client-wasm-bindings-erc20tokeninfotype-j34</code> | notes | const ERC20TokenInfoType; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-client-wasm-bindings-erc20transferfromtype-j34</code> | notes | const ERC20TransferFromType; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-client-wasm-bindings-erc20transfertype-j34</code> | notes | const ERC20TransferType; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-client-wasm-bindings-erc721approvedtype-j34</code> | notes | const ERC721ApprovedType; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-client-wasm-bindings-erc721approvetype-j34</code> | notes | const ERC721ApproveType; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-client-wasm-bindings-erc721isapprovedforalltype-j34</code> | notes | const ERC721IsApprovedForAllType; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-client-wasm-bindings-erc721namesymboltype-j34</code> | notes | const ERC721NameSymbolType; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-client-wasm-bindings-erc721ownertype-j34</code> | notes | const ERC721OwnerType; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-client-wasm-bindings-erc721royaltyinfotype-j34</code> | notes | const ERC721RoyaltyInfoType; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-client-wasm-bindings-erc721setapprovalalltype-j34</code> | notes | const ERC721SetApprovalAllType; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-client-wasm-bindings-erc721totalsupplytype-j34</code> | notes | const ERC721TotalSupplyType; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-client-wasm-bindings-erc721transfertype-j34</code> | notes | const ERC721TransferType; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-client-wasm-bindings-erc721uritype-j34</code> | notes | const ERC721UriType; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-client-wasm-bindings-evmquerytype-j34</code> | notes | type EVMQueryType; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-client-wasm-bindings-getevmaddresstype-j34</code> | notes | const GetEvmAddressType; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-client-wasm-bindings-getpaxaddresstype-j34</code> | notes | const GetPaxAddressType; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-client-wasm-bindings-paxevmquery-getquerytype-j34</code> | notes | method GetQueryType on PaxEVMQuery; 2 in-repo uses outside its package tree, 2 outside tests; first 2 listed |
| <code>interface.6.go-evm-client-wasm-bindings-paxevmquery-j34</code> | notes | type PaxEVMQuery; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-client-wasm-bindings-staticcalltype-j34</code> | notes | const StaticCallType; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-client-wasm-bindings-supportsinterfacetype-j34</code> | notes | const SupportsInterfaceType; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-client-wasm-encodecallevm-j34</code> | notes | func EncodeCallEVM; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-client-wasm-encodedelegatecallevm-j34</code> | notes | func EncodeDelegateCallEVM; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-client-wasm-evmqueryhandler-handleerc1155balanceof-j34</code> | notes | method HandleERC1155BalanceOf on EVMQueryHandler; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-client-wasm-evmqueryhandler-handleerc1155balanceofbatch-j34</code> | notes | method HandleERC1155BalanceOfBatch on EVMQueryHandler; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-client-wasm-evmqueryhandler-handleerc1155batchtransferpayload-j34</code> | notes | method HandleERC1155BatchTransferPayload on EVMQueryHandler; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-client-wasm-evmqueryhandler-handleerc1155isapprovedforall-j34</code> | notes | method HandleERC1155IsApprovedForAll on EVMQueryHandler; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-client-wasm-evmqueryhandler-handleerc1155namesymbol-j34</code> | notes | method HandleERC1155NameSymbol on EVMQueryHandler; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-client-wasm-evmqueryhandler-handleerc1155royaltyinfo-j34</code> | notes | method HandleERC1155RoyaltyInfo on EVMQueryHandler; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-client-wasm-evmqueryhandler-handleerc1155setapprovalallpayload-j34</code> | notes | method HandleERC1155SetApprovalAllPayload on EVMQueryHandler; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-client-wasm-evmqueryhandler-handleerc1155tokenexists-j34</code> | notes | method HandleERC1155TokenExists on EVMQueryHandler; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-client-wasm-evmqueryhandler-handleerc1155totalsupply-j34</code> | notes | method HandleERC1155TotalSupply on EVMQueryHandler; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-client-wasm-evmqueryhandler-handleerc1155totalsupplyfortoken-j34</code> | notes | method HandleERC1155TotalSupplyForToken on EVMQueryHandler; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-client-wasm-evmqueryhandler-handleerc1155transferpayload-j34</code> | notes | method HandleERC1155TransferPayload on EVMQueryHandler; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-client-wasm-evmqueryhandler-handleerc1155uri-j34</code> | notes | method HandleERC1155Uri on EVMQueryHandler; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-client-wasm-evmqueryhandler-handleerc20allowance-j34</code> | notes | method HandleERC20Allowance on EVMQueryHandler; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-client-wasm-evmqueryhandler-handleerc20approvepayload-j34</code> | notes | method HandleERC20ApprovePayload on EVMQueryHandler; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-client-wasm-evmqueryhandler-handleerc20balance-j34</code> | notes | method HandleERC20Balance on EVMQueryHandler; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-client-wasm-evmqueryhandler-handleerc20tokeninfo-j34</code> | notes | method HandleERC20TokenInfo on EVMQueryHandler; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-client-wasm-evmqueryhandler-handleerc20transferfrompayload-j34</code> | notes | method HandleERC20TransferFromPayload on EVMQueryHandler; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-client-wasm-evmqueryhandler-handleerc20transferpayload-j34</code> | notes | method HandleERC20TransferPayload on EVMQueryHandler; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-client-wasm-evmqueryhandler-handleerc721approved-j34</code> | notes | method HandleERC721Approved on EVMQueryHandler; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-client-wasm-evmqueryhandler-handleerc721approvepayload-j34</code> | notes | method HandleERC721ApprovePayload on EVMQueryHandler; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-client-wasm-evmqueryhandler-handleerc721isapprovedforall-j34</code> | notes | method HandleERC721IsApprovedForAll on EVMQueryHandler; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-client-wasm-evmqueryhandler-handleerc721namesymbol-j34</code> | notes | method HandleERC721NameSymbol on EVMQueryHandler; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-client-wasm-evmqueryhandler-handleerc721owner-j34</code> | notes | method HandleERC721Owner on EVMQueryHandler; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-client-wasm-evmqueryhandler-handleerc721royaltyinfo-j34</code> | notes | method HandleERC721RoyaltyInfo on EVMQueryHandler; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-client-wasm-evmqueryhandler-handleerc721setapprovalallpayload-j34</code> | notes | method HandleERC721SetApprovalAllPayload on EVMQueryHandler; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-client-wasm-evmqueryhandler-handleerc721totalsupply-j34</code> | notes | method HandleERC721TotalSupply on EVMQueryHandler; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-client-wasm-evmqueryhandler-handleerc721transferpayload-j34</code> | notes | method HandleERC721TransferPayload on EVMQueryHandler; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-client-wasm-evmqueryhandler-handleerc721uri-j34</code> | notes | method HandleERC721Uri on EVMQueryHandler; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-client-wasm-evmqueryhandler-handlegetevmaddress-j34</code> | notes | method HandleGetEvmAddress on EVMQueryHandler; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-client-wasm-evmqueryhandler-handlegetpaxaddress-j34</code> | notes | method HandleGetPaxAddress on EVMQueryHandler; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-client-wasm-evmqueryhandler-handlestaticcall-j34</code> | notes | method HandleStaticCall on EVMQueryHandler; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-client-wasm-evmqueryhandler-handlesupportsinterface-j34</code> | notes | method HandleSupportsInterface on EVMQueryHandler; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-client-wasm-evmqueryhandler-j34</code> | notes | type EVMQueryHandler; 2 in-repo uses outside its package tree, 2 outside tests; first 2 listed |
| <code>interface.6.go-evm-client-wasm-newevmqueryhandler-j34</code> | notes | func NewEVMQueryHandler; 2 in-repo uses outside its package tree, 1 outside tests; first 2 listed |
| <code>interface.6.go-evm-config-chainidmapping-j34</code> | notes | var ChainIDMapping; 2 in-repo uses outside its package tree, 1 outside tests; first 2 listed |
| <code>interface.6.go-evm-config-defaultchainid-j34</code> | notes | const DefaultChainID; 46 in-repo uses outside its package tree, 7 outside tests; first 12 listed |
| <code>interface.6.go-evm-config-evmchainidmapping-j34</code> | notes | var EVMChainIDMapping; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-config-getevmchainid-j34</code> | notes | func GetEVMChainID; 7 in-repo uses outside its package tree, 6 outside tests; first 7 listed |
| <code>interface.6.go-evm-config-islivechainid-j34</code> | notes | func IsLiveChainID; 4 in-repo uses outside its package tree, 4 outside tests; first 4 listed |
| <code>interface.6.go-evm-config-isliveevmchainid-j34</code> | notes | func IsLiveEVMChainID; 2 in-repo uses outside its package tree, 2 outside tests; first 2 listed |
| <code>interface.6.go-evm-derived-cancun-j34</code> | notes | const Cancun; 4 in-repo uses outside its package tree, 4 outside tests; first 4 listed |
| <code>interface.6.go-evm-derived-derived-j34</code> | notes | type Derived; 3 in-repo uses outside its package tree, 3 outside tests; first 3 listed |
| <code>interface.6.go-evm-derived-london-j34</code> | notes | const London; 2 in-repo uses outside its package tree, 2 outside tests; first 2 listed |
| <code>interface.6.go-evm-derived-prague-j34</code> | notes | const Prague; 2 in-repo uses outside its package tree, 2 outside tests; first 2 listed |
| <code>interface.6.go-evm-derived-signerversion-j34</code> | notes | type SignerVersion; 6 in-repo uses outside its package tree, 6 outside tests; first 6 listed |
| <code>interface.6.go-evm-getgenesisstatefromappstate-j34</code> | notes | func GetGenesisStateFromAppState; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-keeper-basedenom-j34</code> | notes | const BaseDenom; 50 in-repo uses outside its package tree, 7 outside tests; first 12 listed |
| <code>interface.6.go-evm-keeper-convertethlog-j34</code> | notes | func ConvertEthLog; 1 in-repo uses outside its package tree, 0 outside tests; first 1 listed |
| <code>interface.6.go-evm-keeper-convertfeefromdenom-j34</code> | notes | func ConvertFeeFromDenom; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-keeper-convertfeetodenom-j34</code> | notes | func ConvertFeeToDenom; 5 in-repo uses outside its package tree, 5 outside tests; first 5 listed |
| <code>interface.6.go-evm-keeper-convertsyntheticethlog-j34</code> | notes | func ConvertSyntheticEthLog; 7 in-repo uses outside its package tree, 7 outside tests; first 7 listed |
| <code>interface.6.go-evm-keeper-errfeetokendenomnotallowed-j34</code> | notes | var ErrFeeTokenDenomNotAllowed; 2 in-repo uses outside its package tree, 1 outside tests; first 2 listed |
| <code>interface.6.go-evm-keeper-errfeetokenoverflow-j34</code> | notes | var ErrFeeTokenOverflow; 4 in-repo uses outside its package tree, 3 outside tests; first 4 listed |
| <code>interface.6.go-evm-keeper-errfeetokenrateinvalid-j34</code> | notes | var ErrFeeTokenRateInvalid; 3 in-repo uses outside its package tree, 0 outside tests; first 3 listed |
| <code>interface.6.go-evm-keeper-errfeetokenratespread-j34</code> | notes | var ErrFeeTokenRateSpread; 1 in-repo uses outside its package tree, 0 outside tests; first 1 listed |
| <code>interface.6.go-evm-keeper-errfeetokenratestale-j34</code> | notes | var ErrFeeTokenRateStale; 1 in-repo uses outside its package tree, 0 outside tests; first 1 listed |
| <code>interface.6.go-evm-keeper-errfeetokenrateunavailable-j34</code> | notes | var ErrFeeTokenRateUnavailable; 2 in-repo uses outside its package tree, 1 outside tests; first 2 listed |
| <code>interface.6.go-evm-keeper-getlogsfortx-j34</code> | notes | func GetLogsForTx; 15 in-repo uses outside its package tree, 6 outside tests; first 12 listed |
| <code>interface.6.go-evm-keeper-ispayableprecompile-j34</code> | notes | func IsPayablePrecompile; 1 in-repo uses outside its package tree, 0 outside tests; first 1 listed |
| <code>interface.6.go-evm-keeper-keeper-accountkeeper-j34</code> | notes | method AccountKeeper on Keeper; 6 in-repo uses outside its package tree, 4 outside tests; first 6 listed |
| <code>interface.6.go-evm-keeper-keeper-addantesurplus-j34</code> | notes | method AddAnteSurplus on Keeper; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-keeper-keeper-appendtoevmtxdeferredinfo-j34</code> | notes | method AppendToEvmTxDeferredInfo on Keeper; 4 in-repo uses outside its package tree, 4 outside tests; first 4 listed |
| <code>interface.6.go-evm-keeper-keeper-bankkeeper-j34</code> | notes | method BankKeeper on Keeper; 86 in-repo uses outside its package tree, 11 outside tests; first 12 listed |
| <code>interface.6.go-evm-keeper-keeper-beginblock-j34</code> | notes | method BeginBlock on Keeper; 17 in-repo uses outside its package tree, 4 outside tests; first 12 listed |
| <code>interface.6.go-evm-keeper-keeper-callevm-j34</code> | notes | method CallEVM on Keeper; 11 in-repo uses outside its package tree, 0 outside tests; first 11 listed |
| <code>interface.6.go-evm-keeper-keeper-chainid-j34</code> | notes | method ChainID on Keeper; 71 in-repo uses outside its package tree, 25 outside tests; first 12 listed |
| <code>interface.6.go-evm-keeper-keeper-convertfeefromdenom-j34</code> | notes | method ConvertFeeFromDenom on Keeper; 3 in-repo uses outside its package tree, 2 outside tests; first 3 listed |
| <code>interface.6.go-evm-keeper-keeper-convertfeetodenom-j34</code> | notes | method ConvertFeeToDenom on Keeper; 5 in-repo uses outside its package tree, 5 outside tests; first 5 listed |
| <code>interface.6.go-evm-keeper-keeper-customprecompiles-j34</code> | notes | method CustomPrecompiles on Keeper; 32 in-repo uses outside its package tree, 4 outside tests; first 12 listed |
| <code>interface.6.go-evm-keeper-keeper-endblock-j34</code> | notes | method EndBlock on Keeper; 13 in-repo uses outside its package tree, 2 outside tests; first 12 listed |
| <code>interface.6.go-evm-keeper-keeper-flushtransientreceipts-j34</code> | notes | method FlushTransientReceipts on Keeper; 5 in-repo uses outside its package tree, 1 outside tests; first 5 listed |
| <code>interface.6.go-evm-keeper-keeper-getallowedfeedenoms-j34</code> | notes | method GetAllowedFeeDenoms on Keeper; 6 in-repo uses outside its package tree, 4 outside tests; first 6 listed |
| <code>interface.6.go-evm-keeper-keeper-getantefeetokencharge-j34</code> | notes | method GetAnteFeeTokenCharge on Keeper; 8 in-repo uses outside its package tree, 1 outside tests; first 8 listed |
| <code>interface.6.go-evm-keeper-keeper-getantesurplussum-j34</code> | notes | method GetAnteSurplusSum on Keeper; 1 in-repo uses outside its package tree, 0 outside tests; first 1 listed |
| <code>interface.6.go-evm-keeper-keeper-getanypointeeinfo-j34</code> | notes | method GetAnyPointeeInfo on Keeper; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-keeper-keeper-getbalance-j34</code> | notes | method GetBalance on Keeper; 38 in-repo uses outside its package tree, 10 outside tests; first 12 listed |
| <code>interface.6.go-evm-keeper-keeper-getbasedenom-j34</code> | notes | method GetBaseDenom on Keeper; 32 in-repo uses outside its package tree, 4 outside tests; first 12 listed |
| <code>interface.6.go-evm-keeper-keeper-getbasefee-j34</code> | notes | method GetBaseFee on Keeper; 3 in-repo uses outside its package tree, 3 outside tests; first 3 listed |
| <code>interface.6.go-evm-keeper-keeper-getbasefeepergas-j34</code> | notes | method GetBaseFeePerGas on Keeper; 2 in-repo uses outside its package tree, 2 outside tests; first 2 listed |
| <code>interface.6.go-evm-keeper-keeper-getblockbloom-j34</code> | notes | method GetBlockBloom on Keeper; 2 in-repo uses outside its package tree, 1 outside tests; first 2 listed |
| <code>interface.6.go-evm-keeper-keeper-getcode-j34</code> | notes | method GetCode on Keeper; 16 in-repo uses outside its package tree, 4 outside tests; first 12 listed |
| <code>interface.6.go-evm-keeper-keeper-getcurrbasefeepergas-j34</code> | notes | method GetCurrBaseFeePerGas on Keeper; 2 in-repo uses outside its package tree, 2 outside tests; first 2 listed |
| <code>interface.6.go-evm-keeper-keeper-getcustomprecompilesversions-j34</code> | notes | method GetCustomPrecompilesVersions on Keeper; 6 in-repo uses outside its package tree, 0 outside tests; first 6 listed |
| <code>interface.6.go-evm-keeper-keeper-getdelivertxhookwasmgaslimit-j34</code> | notes | method GetDeliverTxHookWasmGasLimit on Keeper; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-keeper-keeper-geterc1155cw1155pointer-j34</code> | notes | method GetERC1155CW1155Pointer on Keeper; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-keeper-keeper-geterc20cw20pointer-j34</code> | notes | method GetERC20CW20Pointer on Keeper; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-keeper-keeper-geterc20nativepointer-j34</code> | notes | method GetERC20NativePointer on Keeper; 15 in-repo uses outside its package tree, 2 outside tests; first 12 listed |
| <code>interface.6.go-evm-keeper-keeper-geterc721cw721pointer-j34</code> | notes | method GetERC721CW721Pointer on Keeper; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-keeper-keeper-getevmaddress-j34</code> | notes | method GetEVMAddress on Keeper; 7 in-repo uses outside its package tree, 7 outside tests; first 7 listed |
| <code>interface.6.go-evm-keeper-keeper-getevmaddressordefault-j34</code> | notes | method GetEVMAddressOrDefault on Keeper; 8 in-repo uses outside its package tree, 6 outside tests; first 8 listed |
| <code>interface.6.go-evm-keeper-keeper-getevmmessage-j34</code> | notes | method GetEVMMessage on Keeper; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-keeper-keeper-getevmonlyblockbloom-j34</code> | notes | method GetEvmOnlyBlockBloom on Keeper; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-keeper-keeper-getevmtxdeferredinfo-j34</code> | notes | method GetEVMTxDeferredInfo on Keeper; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-keeper-keeper-getfeecollectoraddress-j34</code> | notes | method GetFeeCollectorAddress on Keeper; 3 in-repo uses outside its package tree, 3 outside tests; first 3 listed |
| <code>interface.6.go-evm-keeper-keeper-getfeetokencharge-j34</code> | notes | method GetFeeTokenCharge on Keeper; 2 in-repo uses outside its package tree, 2 outside tests; first 2 listed |
| <code>interface.6.go-evm-keeper-keeper-getfeetokenenabled-j34</code> | notes | method GetFeeTokenEnabled on Keeper; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-keeper-keeper-getfeetokenrate-j34</code> | notes | method GetFeeTokenRate on Keeper; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-keeper-keeper-getgaspool-j34</code> | notes | method GetGasPool on Keeper; 6 in-repo uses outside its package tree, 6 outside tests; first 6 listed |
| <code>interface.6.go-evm-keeper-keeper-getminimumfeepergas-j34</code> | notes | method GetMinimumFeePerGas on Keeper; 3 in-repo uses outside its package tree, 3 outside tests; first 3 listed |
| <code>interface.6.go-evm-keeper-keeper-getnativepointee-j34</code> | notes | method GetNativePointee on Keeper; 7 in-repo uses outside its package tree, 3 outside tests; first 7 listed |
| <code>interface.6.go-evm-keeper-keeper-getnextbasefeepergas-j34</code> | notes | method GetNextBaseFeePerGas on Keeper; 12 in-repo uses outside its package tree, 12 outside tests; first 12 listed |
| <code>interface.6.go-evm-keeper-keeper-getnonce-j34</code> | notes | method GetNonce on Keeper; 26 in-repo uses outside its package tree, 12 outside tests; first 12 listed |
| <code>interface.6.go-evm-keeper-keeper-getparams-j34</code> | notes | method GetParams on Keeper; 43 in-repo uses outside its package tree, 13 outside tests; first 12 listed |
| <code>interface.6.go-evm-keeper-keeper-getpaxaddress-j34</code> | notes | method GetPaxAddress on Keeper; 2 in-repo uses outside its package tree, 1 outside tests; first 2 listed |
| <code>interface.6.go-evm-keeper-keeper-getpaxaddressordefault-j34</code> | notes | method GetPaxAddressOrDefault on Keeper; 14 in-repo uses outside its package tree, 5 outside tests; first 12 listed |
| <code>interface.6.go-evm-keeper-keeper-getprioritynormalizer-j34</code> | notes | method GetPriorityNormalizer on Keeper; 3 in-repo uses outside its package tree, 3 outside tests; first 3 listed |
| <code>interface.6.go-evm-keeper-keeper-getreceipt-j34</code> | notes | method GetReceipt on Keeper; 20 in-repo uses outside its package tree, 14 outside tests; first 12 listed |
| <code>interface.6.go-evm-keeper-keeper-getreceiptfromreceiptstore-j34</code> | notes | method GetReceiptFromReceiptStore on Keeper; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-keeper-keeper-getreceiptwithretry-j34</code> | notes | method GetReceiptWithRetry on Keeper; 2 in-repo uses outside its package tree, 2 outside tests; first 2 listed |
| <code>interface.6.go-evm-keeper-keeper-getsstoresetgaseip2200-j34</code> | notes | method GetSstoreSetGasEIP2200 on Keeper; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-keeper-keeper-getstate-j34</code> | notes | method GetState on Keeper; 10 in-repo uses outside its package tree, 1 outside tests; first 10 listed |
| <code>interface.6.go-evm-keeper-keeper-getstorekey-j34</code> | notes | method GetStoreKey on Keeper; 3 in-repo uses outside its package tree, 3 outside tests; first 3 listed |
| <code>interface.6.go-evm-keeper-keeper-gettransientreceipt-j34</code> | notes | method GetTransientReceipt on Keeper; 28 in-repo uses outside its package tree, 8 outside tests; first 12 listed |
| <code>interface.6.go-evm-keeper-keeper-getvmblockcontext-j34</code> | notes | method GetVMBlockContext on Keeper; 10 in-repo uses outside its package tree, 6 outside tests; first 10 listed |
| <code>interface.6.go-evm-keeper-keeper-handleinternalevmcall-j34</code> | notes | method HandleInternalEVMCall on Keeper; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-keeper-keeper-handleinternalevmdelegatecall-j34</code> | notes | method HandleInternalEVMDelegateCall on Keeper; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-keeper-keeper-initgenesis-j34</code> | notes | method InitGenesis on Keeper; 16 in-repo uses outside its package tree, 6 outside tests; first 12 listed |
| <code>interface.6.go-evm-keeper-keeper-isallowedfeedenom-j34</code> | notes | method IsAllowedFeeDenom on Keeper; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-keeper-keeper-iteratestate-j34</code> | notes | method IterateState on Keeper; 1 in-repo uses outside its package tree, 0 outside tests; first 1 listed |
| <code>interface.6.go-evm-keeper-keeper-j34</code> | notes | type Keeper; 126 in-repo uses outside its package tree, 100 outside tests; first 12 listed |
| <code>interface.6.go-evm-keeper-keeper-mockreceipt-j34</code> | notes | method MockReceipt on Keeper; 16 in-repo uses outside its package tree, 1 outside tests; first 12 listed |
| <code>interface.6.go-evm-keeper-keeper-preparectxforevmtransaction-j34</code> | notes | method PrepareCtxForEVMTransaction on Keeper; 2 in-repo uses outside its package tree, 2 outside tests; first 2 listed |
| <code>interface.6.go-evm-keeper-keeper-preparereplayedaddr-j34</code> | notes | method PrepareReplayedAddr on Keeper; 2 in-repo uses outside its package tree, 2 outside tests; first 2 listed |
| <code>interface.6.go-evm-keeper-keeper-receiptstore-j34</code> | notes | method ReceiptStore on Keeper; 5 in-repo uses outside its package tree, 3 outside tests; first 5 listed |
| <code>interface.6.go-evm-keeper-keeper-routecollectedfeetokens-j34</code> | notes | method RouteCollectedFeeTokens on Keeper; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-keeper-keeper-runwithoneoffevminstance-j34</code> | notes | method RunWithOneOffEVMInstance on Keeper; 10 in-repo uses outside its package tree, 3 outside tests; first 10 listed |
| <code>interface.6.go-evm-keeper-keeper-setaccountfeedenom-j34</code> | notes | method SetAccountFeeDenom on Keeper; 2 in-repo uses outside its package tree, 0 outside tests; first 2 listed |
| <code>interface.6.go-evm-keeper-keeper-setaddressmapping-j34</code> | notes | method SetAddressMapping on Keeper; 50 in-repo uses outside its package tree, 3 outside tests; first 12 listed |
| <code>interface.6.go-evm-keeper-keeper-setantefeetokencharge-j34</code> | notes | method SetAnteFeeTokenCharge on Keeper; 2 in-repo uses outside its package tree, 2 outside tests; first 2 listed |
| <code>interface.6.go-evm-keeper-keeper-setblockbloom-j34</code> | notes | method SetBlockBloom on Keeper; 2 in-repo uses outside its package tree, 0 outside tests; first 2 listed |
| <code>interface.6.go-evm-keeper-keeper-setcode-j34</code> | notes | method SetCode on Keeper; 7 in-repo uses outside its package tree, 0 outside tests; first 7 listed |
| <code>interface.6.go-evm-keeper-keeper-setcustomprecompileactivation-j34</code> | notes | method SetCustomPrecompileActivation on Keeper; 8 in-repo uses outside its package tree, 1 outside tests; first 8 listed |
| <code>interface.6.go-evm-keeper-keeper-seterc20cw20pointer-j34</code> | notes | method SetERC20CW20Pointer on Keeper; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-keeper-keeper-seterc20nativepointer-j34</code> | notes | method SetERC20NativePointer on Keeper; 4 in-repo uses outside its package tree, 0 outside tests; first 4 listed |
| <code>interface.6.go-evm-keeper-keeper-setevmonlyblockbloom-j34</code> | notes | method SetEvmOnlyBlockBloom on Keeper; 2 in-repo uses outside its package tree, 0 outside tests; first 2 listed |
| <code>interface.6.go-evm-keeper-keeper-setmsgs-j34</code> | notes | method SetMsgs on Keeper; 28 in-repo uses outside its package tree, 6 outside tests; first 12 listed |
| <code>interface.6.go-evm-keeper-keeper-setnonce-j34</code> | notes | method SetNonce on Keeper; 6 in-repo uses outside its package tree, 3 outside tests; first 6 listed |
| <code>interface.6.go-evm-keeper-keeper-setnoncebumped-j34</code> | notes | method SetNonceBumped on Keeper; 2 in-repo uses outside its package tree, 2 outside tests; first 2 listed |
| <code>interface.6.go-evm-keeper-keeper-setparams-j34</code> | notes | method SetParams on Keeper; 12 in-repo uses outside its package tree, 3 outside tests; first 12 listed |
| <code>interface.6.go-evm-keeper-keeper-setstate-j34</code> | notes | method SetState on Keeper; 11 in-repo uses outside its package tree, 0 outside tests; first 11 listed |
| <code>interface.6.go-evm-keeper-keeper-settracedb-j34</code> | notes | method SetTraceDB on Keeper; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-keeper-keeper-settracesnapshotcapture-j34</code> | notes | method SetTraceSnapshotCapture on Keeper; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-keeper-keeper-settracesnapshotstore-j34</code> | notes | method SetTraceSnapshotStore on Keeper; 2 in-repo uses outside its package tree, 1 outside tests; first 2 listed |
| <code>interface.6.go-evm-keeper-keeper-settransientreceipt-j34</code> | notes | method SetTransientReceipt on Keeper; 6 in-repo uses outside its package tree, 6 outside tests; first 6 listed |
| <code>interface.6.go-evm-keeper-keeper-settxresults-j34</code> | notes | method SetTxResults on Keeper; 5 in-repo uses outside its package tree, 1 outside tests; first 5 listed |
| <code>interface.6.go-evm-keeper-keeper-staticcallevm-j34</code> | notes | method StaticCallEVM on Keeper; 11 in-repo uses outside its package tree, 3 outside tests; first 11 listed |
| <code>interface.6.go-evm-keeper-keeper-tracedb-j34</code> | notes | method TraceDB on Keeper; 7 in-repo uses outside its package tree, 7 outside tests; first 7 listed |
| <code>interface.6.go-evm-keeper-keeper-tracesnapshotstore-j34</code> | notes | method TraceSnapshotStore on Keeper; 3 in-repo uses outside its package tree, 2 outside tests; first 3 listed |
| <code>interface.6.go-evm-keeper-keeper-upgradekeeper-j34</code> | notes | method UpgradeKeeper on Keeper; 7 in-repo uses outside its package tree, 4 outside tests; first 7 listed |
| <code>interface.6.go-evm-keeper-keeper-upserterccw20pointer-j34</code> | notes | method UpsertERCCW20Pointer on Keeper; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-keeper-keeper-upsertercnativepointer-j34</code> | notes | method UpsertERCNativePointer on Keeper; 5 in-repo uses outside its package tree, 1 outside tests; first 5 listed |
| <code>interface.6.go-evm-keeper-keeper-wasmkeeper-j34</code> | notes | method WasmKeeper on Keeper; 4 in-repo uses outside its package tree, 4 outside tests; first 4 listed |
| <code>interface.6.go-evm-keeper-keeper-writereceipt-j34</code> | notes | method WriteReceipt on Keeper; 3 in-repo uses outside its package tree, 2 outside tests; first 3 listed |
| <code>interface.6.go-evm-keeper-newevmaddresshandler-j34</code> | notes | func NewEvmAddressHandler; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-keeper-newkeeper-j34</code> | notes | func NewKeeper; 5 in-repo uses outside its package tree, 1 outside tests; first 5 listed |
| <code>interface.6.go-evm-keeper-newmsgserverimpl-j34</code> | notes | func NewMsgServerImpl; 15 in-repo uses outside its package tree, 2 outside tests; first 12 listed |
| <code>interface.6.go-evm-keeper-newtracedb-j34</code> | notes | func NewTraceDB; 18 in-repo uses outside its package tree, 1 outside tests; first 12 listed |
| <code>interface.6.go-evm-keeper-newtracesnapshotstore-j34</code> | notes | func NewTraceSnapshotStore; 2 in-repo uses outside its package tree, 1 outside tests; first 2 listed |
| <code>interface.6.go-evm-keeper-querier-staticcall-j34</code> | notes | method StaticCall on Querier; 3 in-repo uses outside its package tree, 0 outside tests; first 3 listed |
| <code>interface.6.go-evm-keeper-syntheticreceiptactivation-j34</code> | notes | const SyntheticReceiptActivation; 5 in-repo uses outside its package tree, 1 outside tests; first 5 listed |
| <code>interface.6.go-evm-keeper-syntheticreceiptcontext-j34</code> | notes | func SyntheticReceiptContext; 5 in-repo uses outside its package tree, 3 outside tests; first 5 listed |
| <code>interface.6.go-evm-keeper-syntheticreceipthash-j34</code> | notes | func SyntheticReceiptHash; 27 in-repo uses outside its package tree, 5 outside tests; first 12 listed |
| <code>interface.6.go-evm-keeper-syntheticreceiptkind-j34</code> | notes | type SyntheticReceiptKind; 2 in-repo uses outside its package tree, 1 outside tests; first 2 listed |
| <code>interface.6.go-evm-keeper-syntheticreceiptkindof-j34</code> | notes | func SyntheticReceiptKindOf; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-keeper-syntheticreceiptkinds-j34</code> | notes | var SyntheticReceiptKinds; 2 in-repo uses outside its package tree, 2 outside tests; first 2 listed |
| <code>interface.6.go-evm-keeper-syntheticreceiptperps-j34</code> | notes | const SyntheticReceiptPerps; 14 in-repo uses outside its package tree, 1 outside tests; first 12 listed |
| <code>interface.6.go-evm-keeper-syntheticreceiptprovisionalindex-j34</code> | notes | func SyntheticReceiptProvisionalIndex; 12 in-repo uses outside its package tree, 3 outside tests; first 12 listed |
| <code>interface.6.go-evm-keeper-syntheticreceiptspot-j34</code> | notes | const SyntheticReceiptSpot; 15 in-repo uses outside its package tree, 5 outside tests; first 12 listed |
| <code>interface.6.go-evm-keeper-tracedb-close-j34</code> | notes | method Close on TraceDB; 36 in-repo uses outside its package tree, 12 outside tests; first 12 listed |
| <code>interface.6.go-evm-keeper-tracedb-enqueue-j34</code> | notes | method Enqueue on TraceDB; 15 in-repo uses outside its package tree, 0 outside tests; first 12 listed |
| <code>interface.6.go-evm-keeper-tracedb-get-j34</code> | notes | method Get on TraceDB; 45 in-repo uses outside its package tree, 19 outside tests; first 12 listed |
| <code>interface.6.go-evm-keeper-tracedb-getblock-j34</code> | notes | method GetBlock on TraceDB; 3 in-repo uses outside its package tree, 2 outside tests; first 3 listed |
| <code>interface.6.go-evm-keeper-tracedb-j34</code> | notes | type TraceDB; 11 in-repo uses outside its package tree, 11 outside tests; first 11 listed |
| <code>interface.6.go-evm-keeper-tracedb-lastbakedheight-j34</code> | notes | method LastBakedHeight on TraceDB; 8 in-repo uses outside its package tree, 1 outside tests; first 8 listed |
| <code>interface.6.go-evm-keeper-tracedb-prune-j34</code> | notes | method Prune on TraceDB; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-keeper-tracedb-put-j34</code> | notes | method Put on TraceDB; 7 in-repo uses outside its package tree, 3 outside tests; first 7 listed |
| <code>interface.6.go-evm-keeper-tracedb-putblock-j34</code> | notes | method PutBlock on TraceDB; 4 in-repo uses outside its package tree, 1 outside tests; first 4 listed |
| <code>interface.6.go-evm-keeper-tracedb-setlastbakedheight-j34</code> | notes | method SetLastBakedHeight on TraceDB; 3 in-repo uses outside its package tree, 1 outside tests; first 3 listed |
| <code>interface.6.go-evm-keeper-tracedb-settraceenqueuer-j34</code> | notes | method SetTraceEnqueuer on TraceDB; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-keeper-tracesnapshotstore-close-j34</code> | notes | method Close on TraceSnapshotStore; 36 in-repo uses outside its package tree, 12 outside tests; first 12 listed |
| <code>interface.6.go-evm-keeper-tracesnapshotstore-get-j34</code> | notes | method Get on TraceSnapshotStore; 45 in-repo uses outside its package tree, 19 outside tests; first 12 listed |
| <code>interface.6.go-evm-keeper-tracesnapshotstore-lease-j34</code> | notes | method Lease on TraceSnapshotStore; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-keeper-tracesnapshotstore-put-j34</code> | notes | method Put on TraceSnapshotStore; 7 in-repo uses outside its package tree, 3 outside tests; first 7 listed |
| <code>interface.6.go-evm-module-consensusversion-j34</code> | notes | method ConsensusVersion on Module; 2 in-repo uses outside its package tree, 0 outside tests; first 2 listed |
| <code>interface.6.go-evm-newappmodule-j34</code> | notes | func NewAppModule; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-newparamchangeproposalhandler-j34</code> | notes | func NewParamChangeProposalHandler; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-newproposalhandler-j34</code> | notes | func NewProposalHandler; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-querier-config-j34</code> | notes | type Config; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-querier-defaultconfig-j34</code> | notes | var DefaultConfig; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-querier-readconfig-j34</code> | notes | func ReadConfig; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-replay-config-j34</code> | notes | type Config; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-replay-defaultconfig-j34</code> | notes | var DefaultConfig; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-replay-readconfig-j34</code> | notes | func ReadConfig; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-state-dbimpl-addbalance-j34</code> | notes | method AddBalance on DBImpl; 3 in-repo uses outside its package tree, 2 outside tests; first 3 listed |
| <code>interface.6.go-evm-state-dbimpl-addlog-j34</code> | notes | method AddLog on DBImpl; 8 in-repo uses outside its package tree, 8 outside tests; first 8 listed |
| <code>interface.6.go-evm-state-dbimpl-cleanup-j34</code> | notes | method Cleanup on DBImpl; 11 in-repo uses outside its package tree, 1 outside tests; first 11 listed |
| <code>interface.6.go-evm-state-dbimpl-cleanupfortracer-j34</code> | notes | method CleanupForTracer on DBImpl; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-state-dbimpl-commit-j34</code> | notes | method Commit on DBImpl; 7 in-repo uses outside its package tree, 2 outside tests; first 7 listed |
| <code>interface.6.go-evm-state-dbimpl-copy-j34</code> | notes | method Copy on DBImpl; 6 in-repo uses outside its package tree, 0 outside tests; first 6 listed |
| <code>interface.6.go-evm-state-dbimpl-ctx-j34</code> | notes | method Ctx on DBImpl; 187 in-repo uses outside its package tree, 51 outside tests; first 12 listed |
| <code>interface.6.go-evm-state-dbimpl-empty-j34</code> | notes | method Empty on DBImpl; 227 in-repo uses outside its package tree, 0 outside tests; first 12 listed |
| <code>interface.6.go-evm-state-dbimpl-err-j34</code> | notes | method Err on DBImpl; 4 in-repo uses outside its package tree, 3 outside tests; first 4 listed |
| <code>interface.6.go-evm-state-dbimpl-finalize-j34</code> | notes | method Finalize on DBImpl; 10 in-repo uses outside its package tree, 4 outside tests; first 10 listed |
| <code>interface.6.go-evm-state-dbimpl-getalllogs-j34</code> | notes | method GetAllLogs on DBImpl; 97 in-repo uses outside its package tree, 1 outside tests; first 12 listed |
| <code>interface.6.go-evm-state-dbimpl-getbalance-j34</code> | notes | method GetBalance on DBImpl; 52 in-repo uses outside its package tree, 13 outside tests; first 12 listed |
| <code>interface.6.go-evm-state-dbimpl-getcode-j34</code> | notes | method GetCode on DBImpl; 45 in-repo uses outside its package tree, 3 outside tests; first 12 listed |
| <code>interface.6.go-evm-state-dbimpl-getcodehash-j34</code> | notes | method GetCodeHash on DBImpl; 4 in-repo uses outside its package tree, 0 outside tests; first 4 listed |
| <code>interface.6.go-evm-state-dbimpl-getcodesize-j34</code> | notes | method GetCodeSize on DBImpl; 2 in-repo uses outside its package tree, 2 outside tests; first 2 listed |
| <code>interface.6.go-evm-state-dbimpl-getnonce-j34</code> | notes | method GetNonce on DBImpl; 24 in-repo uses outside its package tree, 5 outside tests; first 12 listed |
| <code>interface.6.go-evm-state-dbimpl-getprecompileerror-j34</code> | notes | method GetPrecompileError on DBImpl; 32 in-repo uses outside its package tree, 0 outside tests; first 12 listed |
| <code>interface.6.go-evm-state-dbimpl-getstate-j34</code> | notes | method GetState on DBImpl; 19 in-repo uses outside its package tree, 1 outside tests; first 12 listed |
| <code>interface.6.go-evm-state-dbimpl-gettransientstate-j34</code> | notes | method GetTransientState on DBImpl; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-state-dbimpl-j34</code> | notes | type DBImpl; 58 in-repo uses outside its package tree, 3 outside tests; first 12 listed |
| <code>interface.6.go-evm-state-dbimpl-prepare-j34</code> | notes | method Prepare on DBImpl; 48 in-repo uses outside its package tree, 48 outside tests; first 12 listed |
| <code>interface.6.go-evm-state-dbimpl-resetfortracer-j34</code> | notes | method ResetForTracer on DBImpl; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-state-dbimpl-reverttosnapshot-j34</code> | notes | method RevertToSnapshot on DBImpl; 17 in-repo uses outside its package tree, 5 outside tests; first 12 listed |
| <code>interface.6.go-evm-state-dbimpl-setcode-j34</code> | notes | method SetCode on DBImpl; 30 in-repo uses outside its package tree, 1 outside tests; first 12 listed |
| <code>interface.6.go-evm-state-dbimpl-setfeetokencharge-j34</code> | notes | method SetFeeTokenCharge on DBImpl; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-state-dbimpl-setlogger-j34</code> | notes | method SetLogger on DBImpl; 1 in-repo uses outside its package tree, 0 outside tests; first 1 listed |
| <code>interface.6.go-evm-state-dbimpl-setnonce-j34</code> | notes | method SetNonce on DBImpl; 6 in-repo uses outside its package tree, 3 outside tests; first 6 listed |
| <code>interface.6.go-evm-state-dbimpl-setprecompileerror-j34</code> | notes | method SetPrecompileError on DBImpl; 38 in-repo uses outside its package tree, 31 outside tests; first 12 listed |
| <code>interface.6.go-evm-state-dbimpl-setstate-j34</code> | notes | method SetState on DBImpl; 28 in-repo uses outside its package tree, 1 outside tests; first 12 listed |
| <code>interface.6.go-evm-state-dbimpl-settransientstate-j34</code> | notes | method SetTransientState on DBImpl; 2 in-repo uses outside its package tree, 2 outside tests; first 2 listed |
| <code>interface.6.go-evm-state-dbimpl-settxcontext-j34</code> | notes | method SetTxContext on DBImpl; 4 in-repo uses outside its package tree, 3 outside tests; first 4 listed |
| <code>interface.6.go-evm-state-dbimpl-snapshot-j34</code> | notes | method Snapshot on DBImpl; 22 in-repo uses outside its package tree, 7 outside tests; first 12 listed |
| <code>interface.6.go-evm-state-dbimpl-subbalance-j34</code> | notes | method SubBalance on DBImpl; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-state-dbimpl-txindex-j34</code> | notes | method TxIndex on DBImpl; 7 in-repo uses outside its package tree, 2 outside tests; first 7 listed |
| <code>interface.6.go-evm-state-dbimpl-withctx-j34</code> | notes | method WithCtx on DBImpl; 27 in-repo uses outside its package tree, 2 outside tests; first 12 listed |
| <code>interface.6.go-evm-state-evmkeeper-j34</code> | notes | type EVMKeeper; 10 in-repo uses outside its package tree, 10 outside tests; first 10 listed |
| <code>interface.6.go-evm-state-feetokencharge-j34</code> | notes | type FeeTokenCharge; 2 in-repo uses outside its package tree, 2 outside tests; first 2 listed |
| <code>interface.6.go-evm-state-getcoinbaseaddress-j34</code> | notes | func GetCoinbaseAddress; 4 in-repo uses outside its package tree, 2 outside tests; first 4 listed |
| <code>interface.6.go-evm-state-getdbimpl-j34</code> | notes | func GetDBImpl; 98 in-repo uses outside its package tree, 76 outside tests; first 12 listed |
| <code>interface.6.go-evm-state-newdbimpl-j34</code> | notes | func NewDBImpl; 138 in-repo uses outside its package tree, 12 outside tests; first 12 listed |
| <code>interface.6.go-evm-state-sdkuhpxtosweimultiplier-j34</code> | notes | var SdkUhpxToSweiMultiplier; 109 in-repo uses outside its package tree, 105 outside tests; first 12 listed |
| <code>interface.6.go-evm-state-splituhpxweiamount-j34</code> | notes | func SplitUhpxWeiAmount; 36 in-repo uses outside its package tree, 35 outside tests; first 12 listed |
| <code>interface.6.go-evm-state-uhpxtosweimultiplier-j34</code> | notes | var UhpxToSweiMultiplier; 1 in-repo uses outside its package tree, 0 outside tests; first 1 listed |
| <code>interface.6.go-evm-types-accountfeedenomkey-j34</code> | notes | func AccountFeeDenomKey; 2 in-repo uses outside its package tree, 0 outside tests; first 2 listed |
| <code>interface.6.go-evm-types-associationmissingerr-addresstype-j34</code> | notes | method AddressType on AssociationMissingErr; 2 in-repo uses outside its package tree, 2 outside tests; first 2 listed |
| <code>interface.6.go-evm-types-associationmissingerr-j34</code> | notes | type AssociationMissingErr; 4 in-repo uses outside its package tree, 4 outside tests; first 4 listed |
| <code>interface.6.go-evm-types-attributekeyevmaddress-j34</code> | notes | const AttributeKeyEvmAddress; 2 in-repo uses outside its package tree, 2 outside tests; first 2 listed |
| <code>interface.6.go-evm-types-attributekeypaxaddress-j34</code> | notes | const AttributeKeyPaxAddress; 5 in-repo uses outside its package tree, 5 outside tests; first 5 listed |
| <code>interface.6.go-evm-types-attributekeypointee-j34</code> | notes | const AttributeKeyPointee; 9 in-repo uses outside its package tree, 9 outside tests; first 9 listed |
| <code>interface.6.go-evm-types-attributekeypointeraddress-j34</code> | notes | const AttributeKeyPointerAddress; 9 in-repo uses outside its package tree, 9 outside tests; first 9 listed |
| <code>interface.6.go-evm-types-attributekeypointertype-j34</code> | notes | const AttributeKeyPointerType; 9 in-repo uses outside its package tree, 9 outside tests; first 9 listed |
| <code>interface.6.go-evm-types-attributekeypointerversion-j34</code> | notes | const AttributeKeyPointerVersion; 9 in-repo uses outside its package tree, 9 outside tests; first 9 listed |
| <code>interface.6.go-evm-types-cancuntime-j34</code> | notes | var CancunTime; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-types-chainconfig-ethereumconfig-j34</code> | notes | method EthereumConfig on ChainConfig; 63 in-repo uses outside its package tree, 12 outside tests; first 12 listed |
| <code>interface.6.go-evm-types-chainconfig-ethereumconfigwithsstore-j34</code> | notes | method EthereumConfigWithSstore on ChainConfig; 3 in-repo uses outside its package tree, 3 outside tests; first 3 listed |
| <code>interface.6.go-evm-types-chainconfig-validate-j34</code> | notes | method Validate on ChainConfig; 29 in-repo uses outside its package tree, 2 outside tests; first 12 listed |
| <code>interface.6.go-evm-types-defaultchainconfig-j34</code> | notes | func DefaultChainConfig; 68 in-repo uses outside its package tree, 17 outside tests; first 12 listed |
| <code>interface.6.go-evm-types-defaultfeetokendistribution-j34</code> | notes | var DefaultFeeTokenDistribution; 1 in-repo uses outside its package tree, 0 outside tests; first 1 listed |
| <code>interface.6.go-evm-types-defaultfeetokenenabled-j34</code> | notes | var DefaultFeeTokenEnabled; 1 in-repo uses outside its package tree, 0 outside tests; first 1 listed |
| <code>interface.6.go-evm-types-defaultgenesis-j34</code> | notes | func DefaultGenesis; 6 in-repo uses outside its package tree, 5 outside tests; first 6 listed |
| <code>interface.6.go-evm-types-defaultmaxfeetokenrateage-j34</code> | notes | const DefaultMaxFeeTokenRateAge; 8 in-repo uses outside its package tree, 0 outside tests; first 8 listed |
| <code>interface.6.go-evm-types-defaultmaxfeetokenspread-j34</code> | notes | var DefaultMaxFeeTokenSpread; 1 in-repo uses outside its package tree, 0 outside tests; first 1 listed |
| <code>interface.6.go-evm-types-defaultminfeepergas-j34</code> | notes | var DefaultMinFeePerGas; 12 in-repo uses outside its package tree, 5 outside tests; first 12 listed |
| <code>interface.6.go-evm-types-defaultparams-j34</code> | notes | func DefaultParams; 4 in-repo uses outside its package tree, 0 outside tests; first 4 listed |
| <code>interface.6.go-evm-types-defaultpaxsstoresetgaseip2200-j34</code> | notes | var DefaultPaxSstoreSetGasEIP2200; 4 in-repo uses outside its package tree, 0 outside tests; first 4 listed |
| <code>interface.6.go-evm-types-ethtx-accesslisttx-asethereumdata-j34</code> | notes | method AsEthereumData on AccessListTx; 3 in-repo uses outside its package tree, 3 outside tests; first 3 listed |
| <code>interface.6.go-evm-types-ethtx-accesslisttx-effectivegasprice-j34</code> | notes | method EffectiveGasPrice on AccessListTx; 2 in-repo uses outside its package tree, 2 outside tests; first 2 listed |
| <code>interface.6.go-evm-types-ethtx-accesslisttx-getblobfeecap-j34</code> | notes | method GetBlobFeeCap on AccessListTx; 2 in-repo uses outside its package tree, 2 outside tests; first 2 listed |
| <code>interface.6.go-evm-types-ethtx-accesslisttx-getblobhashes-j34</code> | notes | method GetBlobHashes on AccessListTx; 2 in-repo uses outside its package tree, 2 outside tests; first 2 listed |
| <code>interface.6.go-evm-types-ethtx-accesslisttx-getchainid-j34</code> | notes | method GetChainID on AccessListTx; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-types-ethtx-accesslisttx-getgas-j34</code> | notes | method GetGas on AccessListTx; 3 in-repo uses outside its package tree, 3 outside tests; first 3 listed |
| <code>interface.6.go-evm-types-ethtx-accesslisttx-getgasfeecap-j34</code> | notes | method GetGasFeeCap on AccessListTx; 3 in-repo uses outside its package tree, 3 outside tests; first 3 listed |
| <code>interface.6.go-evm-types-ethtx-accesslisttx-getgastipcap-j34</code> | notes | method GetGasTipCap on AccessListTx; 2 in-repo uses outside its package tree, 2 outside tests; first 2 listed |
| <code>interface.6.go-evm-types-ethtx-accesslisttx-getnonce-j34</code> | notes | method GetNonce on AccessListTx; 74 in-repo uses outside its package tree, 4 outside tests; first 12 listed |
| <code>interface.6.go-evm-types-ethtx-accesslisttx-getrawsignaturevalues-j34</code> | notes | method GetRawSignatureValues on AccessListTx; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-types-ethtx-accesslisttx-validate-j34</code> | notes | method Validate on AccessListTx; 1 in-repo uses outside its package tree, 0 outside tests; first 1 listed |
| <code>interface.6.go-evm-types-ethtx-associatetx-getrawsignaturevalues-j34</code> | notes | method GetRawSignatureValues on AssociateTx; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-types-ethtx-associatetx-validate-j34</code> | notes | method Validate on AssociateTx; 1 in-repo uses outside its package tree, 0 outside tests; first 1 listed |
| <code>interface.6.go-evm-types-ethtx-authlist-j34</code> | notes | type AuthList; 1 in-repo uses outside its package tree, 0 outside tests; first 1 listed |
| <code>interface.6.go-evm-types-ethtx-blobtx-asethereumdata-j34</code> | notes | method AsEthereumData on BlobTx; 3 in-repo uses outside its package tree, 3 outside tests; first 3 listed |
| <code>interface.6.go-evm-types-ethtx-blobtx-blobgas-j34</code> | notes | method BlobGas on BlobTx; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-types-ethtx-blobtx-effectivegasprice-j34</code> | notes | method EffectiveGasPrice on BlobTx; 2 in-repo uses outside its package tree, 2 outside tests; first 2 listed |
| <code>interface.6.go-evm-types-ethtx-blobtx-getblobfeecap-j34</code> | notes | method GetBlobFeeCap on BlobTx; 2 in-repo uses outside its package tree, 2 outside tests; first 2 listed |
| <code>interface.6.go-evm-types-ethtx-blobtx-getblobhashes-j34</code> | notes | method GetBlobHashes on BlobTx; 2 in-repo uses outside its package tree, 2 outside tests; first 2 listed |
| <code>interface.6.go-evm-types-ethtx-blobtx-getchainid-j34</code> | notes | method GetChainID on BlobTx; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-types-ethtx-blobtx-getgas-j34</code> | notes | method GetGas on BlobTx; 3 in-repo uses outside its package tree, 3 outside tests; first 3 listed |
| <code>interface.6.go-evm-types-ethtx-blobtx-getgasfeecap-j34</code> | notes | method GetGasFeeCap on BlobTx; 3 in-repo uses outside its package tree, 3 outside tests; first 3 listed |
| <code>interface.6.go-evm-types-ethtx-blobtx-getgastipcap-j34</code> | notes | method GetGasTipCap on BlobTx; 2 in-repo uses outside its package tree, 2 outside tests; first 2 listed |
| <code>interface.6.go-evm-types-ethtx-blobtx-getnonce-j34</code> | notes | method GetNonce on BlobTx; 74 in-repo uses outside its package tree, 4 outside tests; first 12 listed |
| <code>interface.6.go-evm-types-ethtx-blobtx-getrawsignaturevalues-j34</code> | notes | method GetRawSignatureValues on BlobTx; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-types-ethtx-blobtx-validate-j34</code> | notes | method Validate on BlobTx; 1 in-repo uses outside its package tree, 0 outside tests; first 1 listed |
| <code>interface.6.go-evm-types-ethtx-dynamicfeetx-asethereumdata-j34</code> | notes | method AsEthereumData on DynamicFeeTx; 3 in-repo uses outside its package tree, 3 outside tests; first 3 listed |
| <code>interface.6.go-evm-types-ethtx-dynamicfeetx-effectivegasprice-j34</code> | notes | method EffectiveGasPrice on DynamicFeeTx; 2 in-repo uses outside its package tree, 2 outside tests; first 2 listed |
| <code>interface.6.go-evm-types-ethtx-dynamicfeetx-getblobfeecap-j34</code> | notes | method GetBlobFeeCap on DynamicFeeTx; 2 in-repo uses outside its package tree, 2 outside tests; first 2 listed |
| <code>interface.6.go-evm-types-ethtx-dynamicfeetx-getblobhashes-j34</code> | notes | method GetBlobHashes on DynamicFeeTx; 2 in-repo uses outside its package tree, 2 outside tests; first 2 listed |
| <code>interface.6.go-evm-types-ethtx-dynamicfeetx-getchainid-j34</code> | notes | method GetChainID on DynamicFeeTx; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-types-ethtx-dynamicfeetx-getgas-j34</code> | notes | method GetGas on DynamicFeeTx; 3 in-repo uses outside its package tree, 3 outside tests; first 3 listed |
| <code>interface.6.go-evm-types-ethtx-dynamicfeetx-getgasfeecap-j34</code> | notes | method GetGasFeeCap on DynamicFeeTx; 3 in-repo uses outside its package tree, 3 outside tests; first 3 listed |
| <code>interface.6.go-evm-types-ethtx-dynamicfeetx-getgastipcap-j34</code> | notes | method GetGasTipCap on DynamicFeeTx; 2 in-repo uses outside its package tree, 2 outside tests; first 2 listed |
| <code>interface.6.go-evm-types-ethtx-dynamicfeetx-getnonce-j34</code> | notes | method GetNonce on DynamicFeeTx; 74 in-repo uses outside its package tree, 4 outside tests; first 12 listed |
| <code>interface.6.go-evm-types-ethtx-dynamicfeetx-getrawsignaturevalues-j34</code> | notes | method GetRawSignatureValues on DynamicFeeTx; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-types-ethtx-dynamicfeetx-validate-j34</code> | notes | method Validate on DynamicFeeTx; 1 in-repo uses outside its package tree, 0 outside tests; first 1 listed |
| <code>interface.6.go-evm-types-ethtx-ethereumtxdata-j34</code> | notes | type EthereumTxData; 11 in-repo uses outside its package tree, 11 outside tests; first 11 listed |
| <code>interface.6.go-evm-types-ethtx-legacytx-asethereumdata-j34</code> | notes | method AsEthereumData on LegacyTx; 3 in-repo uses outside its package tree, 3 outside tests; first 3 listed |
| <code>interface.6.go-evm-types-ethtx-legacytx-effectivegasprice-j34</code> | notes | method EffectiveGasPrice on LegacyTx; 2 in-repo uses outside its package tree, 2 outside tests; first 2 listed |
| <code>interface.6.go-evm-types-ethtx-legacytx-getblobfeecap-j34</code> | notes | method GetBlobFeeCap on LegacyTx; 2 in-repo uses outside its package tree, 2 outside tests; first 2 listed |
| <code>interface.6.go-evm-types-ethtx-legacytx-getblobhashes-j34</code> | notes | method GetBlobHashes on LegacyTx; 2 in-repo uses outside its package tree, 2 outside tests; first 2 listed |
| <code>interface.6.go-evm-types-ethtx-legacytx-getchainid-j34</code> | notes | method GetChainID on LegacyTx; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-types-ethtx-legacytx-getgas-j34</code> | notes | method GetGas on LegacyTx; 3 in-repo uses outside its package tree, 3 outside tests; first 3 listed |
| <code>interface.6.go-evm-types-ethtx-legacytx-getgasfeecap-j34</code> | notes | method GetGasFeeCap on LegacyTx; 3 in-repo uses outside its package tree, 3 outside tests; first 3 listed |
| <code>interface.6.go-evm-types-ethtx-legacytx-getgastipcap-j34</code> | notes | method GetGasTipCap on LegacyTx; 2 in-repo uses outside its package tree, 2 outside tests; first 2 listed |
| <code>interface.6.go-evm-types-ethtx-legacytx-getnonce-j34</code> | notes | method GetNonce on LegacyTx; 74 in-repo uses outside its package tree, 4 outside tests; first 12 listed |
| <code>interface.6.go-evm-types-ethtx-legacytx-getrawsignaturevalues-j34</code> | notes | method GetRawSignatureValues on LegacyTx; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-types-ethtx-legacytx-validate-j34</code> | notes | method Validate on LegacyTx; 1 in-repo uses outside its package tree, 0 outside tests; first 1 listed |
| <code>interface.6.go-evm-types-ethtx-newaccesslisttx-j34</code> | notes | func NewAccessListTx; 2 in-repo uses outside its package tree, 2 outside tests; first 2 listed |
| <code>interface.6.go-evm-types-ethtx-newblobtx-j34</code> | notes | func NewBlobTx; 2 in-repo uses outside its package tree, 2 outside tests; first 2 listed |
| <code>interface.6.go-evm-types-ethtx-newdynamicfeetx-j34</code> | notes | func NewDynamicFeeTx; 4 in-repo uses outside its package tree, 2 outside tests; first 4 listed |
| <code>interface.6.go-evm-types-ethtx-newlegacytx-j34</code> | notes | func NewLegacyTx; 33 in-repo uses outside its package tree, 5 outside tests; first 12 listed |
| <code>interface.6.go-evm-types-ethtx-newsetcodetx-j34</code> | notes | func NewSetCodeTx; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-types-ethtx-newtxdatafromtx-j34</code> | notes | func NewTxDataFromTx; 29 in-repo uses outside its package tree, 8 outside tests; first 12 listed |
| <code>interface.6.go-evm-types-ethtx-setcodetx-asethereumdata-j34</code> | notes | method AsEthereumData on SetCodeTx; 3 in-repo uses outside its package tree, 3 outside tests; first 3 listed |
| <code>interface.6.go-evm-types-ethtx-setcodetx-effectivegasprice-j34</code> | notes | method EffectiveGasPrice on SetCodeTx; 2 in-repo uses outside its package tree, 2 outside tests; first 2 listed |
| <code>interface.6.go-evm-types-ethtx-setcodetx-getblobfeecap-j34</code> | notes | method GetBlobFeeCap on SetCodeTx; 2 in-repo uses outside its package tree, 2 outside tests; first 2 listed |
| <code>interface.6.go-evm-types-ethtx-setcodetx-getblobhashes-j34</code> | notes | method GetBlobHashes on SetCodeTx; 2 in-repo uses outside its package tree, 2 outside tests; first 2 listed |
| <code>interface.6.go-evm-types-ethtx-setcodetx-getchainid-j34</code> | notes | method GetChainID on SetCodeTx; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-types-ethtx-setcodetx-getgas-j34</code> | notes | method GetGas on SetCodeTx; 3 in-repo uses outside its package tree, 3 outside tests; first 3 listed |
| <code>interface.6.go-evm-types-ethtx-setcodetx-getgasfeecap-j34</code> | notes | method GetGasFeeCap on SetCodeTx; 3 in-repo uses outside its package tree, 3 outside tests; first 3 listed |
| <code>interface.6.go-evm-types-ethtx-setcodetx-getgastipcap-j34</code> | notes | method GetGasTipCap on SetCodeTx; 2 in-repo uses outside its package tree, 2 outside tests; first 2 listed |
| <code>interface.6.go-evm-types-ethtx-setcodetx-getnonce-j34</code> | notes | method GetNonce on SetCodeTx; 74 in-repo uses outside its package tree, 4 outside tests; first 12 listed |
| <code>interface.6.go-evm-types-ethtx-setcodetx-getrawsignaturevalues-j34</code> | notes | method GetRawSignatureValues on SetCodeTx; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-types-ethtx-setcodetx-validate-j34</code> | notes | method Validate on SetCodeTx; 1 in-repo uses outside its package tree, 0 outside tests; first 1 listed |
| <code>interface.6.go-evm-types-feetokenholdingaccount-j34</code> | notes | const FeeTokenHoldingAccount; 5 in-repo uses outside its package tree, 2 outside tests; first 5 listed |
| <code>interface.6.go-evm-types-genesisstate-validate-j34</code> | notes | method Validate on GenesisState; 29 in-repo uses outside its package tree, 2 outside tests; first 12 listed |
| <code>interface.6.go-evm-types-getevmtransactionmessage-j34</code> | notes | func GetEVMTransactionMessage; 4 in-repo uses outside its package tree, 4 outside tests; first 4 listed |
| <code>interface.6.go-evm-types-initialsidiorabaseunitsperpax-j34</code> | notes | const InitialSidioraBaseUnitsPerPax; 9 in-repo uses outside its package tree, 0 outside tests; first 9 listed |
| <code>interface.6.go-evm-types-istxmsgassociate-j34</code> | notes | func IsTxMsgAssociate; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-types-layerxbindmessage-j34</code> | notes | func LayerXBindMessage; 5 in-repo uses outside its package tree, 0 outside tests; first 5 listed |
| <code>interface.6.go-evm-types-layerxbindnoncekey-j34</code> | notes | func LayerXBindNonceKey; 1 in-repo uses outside its package tree, 0 outside tests; first 1 listed |
| <code>interface.6.go-evm-types-layerxdid-j34</code> | notes | func LayerXDid; 14 in-repo uses outside its package tree, 4 outside tests; first 12 listed |
| <code>interface.6.go-evm-types-layerxdidprefix-j34</code> | notes | const LayerXDidPrefix; 4 in-repo uses outside its package tree, 1 outside tests; first 4 listed |
| <code>interface.6.go-evm-types-layerxmainaccountid-j34</code> | notes | func LayerXMainAccountID; 3 in-repo uses outside its package tree, 2 outside tests; first 3 listed |
| <code>interface.6.go-evm-types-layerxmainaccountname-j34</code> | notes | func LayerXMainAccountName; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-types-maxminfeepergas-j34</code> | notes | var MaxMinFeePerGas; 3 in-repo uses outside its package tree, 0 outside tests; first 3 listed |
| <code>interface.6.go-evm-types-modulename-j34</code> | notes | const ModuleName; 98 in-repo uses outside its package tree, 10 outside tests; first 12 listed |
| <code>interface.6.go-evm-types-msgevmtransaction-astransaction-j34</code> | notes | method AsTransaction on MsgEVMTransaction; 33 in-repo uses outside its package tree, 19 outside tests; first 12 listed |
| <code>interface.6.go-evm-types-msgevmtransaction-isassociatetx-j34</code> | notes | method IsAssociateTx on MsgEVMTransaction; 8 in-repo uses outside its package tree, 7 outside tests; first 8 listed |
| <code>interface.6.go-evm-types-mustgetevmtransactionmessage-j34</code> | notes | func MustGetEVMTransactionMessage; 3 in-repo uses outside its package tree, 2 outside tests; first 3 listed |
| <code>interface.6.go-evm-types-newassociationmissingerr-j34</code> | notes | func NewAssociationMissingErr; 212 in-repo uses outside its package tree, 204 outside tests; first 12 listed |
| <code>interface.6.go-evm-types-newlogsfrometh-j34</code> | notes | func NewLogsFromEth; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-types-newmsgclaim-j34</code> | notes | func NewMsgClaim; 10 in-repo uses outside its package tree, 0 outside tests; first 10 listed |
| <code>interface.6.go-evm-types-newmsgclaimspecific-j34</code> | notes | func NewMsgClaimSpecific; 5 in-repo uses outside its package tree, 0 outside tests; first 5 listed |
| <code>interface.6.go-evm-types-newmsgevmtransaction-j34</code> | notes | func NewMsgEVMTransaction; 67 in-repo uses outside its package tree, 14 outside tests; first 12 listed |
| <code>interface.6.go-evm-types-params-validate-j34</code> | notes | method Validate on Params; 29 in-repo uses outside its package tree, 2 outside tests; first 12 listed |
| <code>interface.6.go-evm-types-pointerbindingproposal-getmessages-j34</code> | notes | method GetMessages on PointerBindingProposal; 2 in-repo uses outside its package tree, 2 outside tests; first 2 listed |
| <code>interface.6.go-evm-types-receiptkey-j34</code> | notes | func ReceiptKey; 8 in-repo uses outside its package tree, 4 outside tests; first 8 listed |
| <code>interface.6.go-evm-types-receiptstorekey-j34</code> | notes | const ReceiptStoreKey; 6 in-repo uses outside its package tree, 3 outside tests; first 6 listed |
| <code>interface.6.go-evm-types-registerinterfaces-j34</code> | notes | func RegisterInterfaces; 1 in-repo uses outside its package tree, 0 outside tests; first 1 listed |
| <code>interface.6.go-evm-types-routerkey-j34</code> | notes | const RouterKey; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-types-shellevmtxtype-j34</code> | notes | const ShellEVMTxType; 9 in-repo uses outside its package tree, 5 outside tests; first 9 listed |
| <code>interface.6.go-evm-types-statekey-j34</code> | notes | func StateKey; 1 in-repo uses outside its package tree, 1 outside tests; first 1 listed |
| <code>interface.6.go-evm-types-storekey-j34</code> | notes | const StoreKey; 14 in-repo uses outside its package tree, 10 outside tests; first 12 listed |
| <code>interface.6.go-evm-types-transientstorekey-j34</code> | notes | const TransientStoreKey; 5 in-repo uses outside its package tree, 4 outside tests; first 5 listed |
| <code>interface.6.go-evm-types-txhasheskey-j34</code> | notes | func TxHashesKey; 1 in-repo uses outside its package tree, 0 outside tests; first 1 listed |
| <code>interface.6.go-evm-types-unpacktxdata-j34</code> | notes | func UnpackTxData; 5 in-repo uses outside its package tree, 5 outside tests; first 5 listed |

## params

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.6.gen-evm-keyallowedfeedenoms-j34</code> | <code>genesis_param</code> | <code>stable</code> | <code>modules/evm/types/params.go:17</code> | <code>KeyAllowedFeeDenoms = []byte("KeyAllowedFeeDenoms")</code> |
| <code>interface.6.gen-evm-keybasefeepergas-j34</code> | <code>genesis_param</code> | <code>stable</code> | <code>modules/evm/types/params.go:29</code> | <code>KeyBaseFeePerGas = []byte("KeyBaseFeePerGas")</code> |
| <code>interface.6.gen-evm-keydelivertxhookwasmgaslimit-j34</code> | <code>genesis_param</code> | <code>stable</code> | <code>modules/evm/types/params.go:23</code> | <code>KeyDeliverTxHookWasmGasLimit = []byte("KeyDeliverTxHookWasmGasLimit")</code> |
| <code>interface.6.gen-evm-keyfeetokendistribution-j34</code> | <code>genesis_param</code> | <code>stable</code> | <code>modules/evm/types/params.go:15</code> | <code>KeyFeeTokenDistribution = []byte("KeyFeeTokenDistribution")</code> |
| <code>interface.6.gen-evm-keyfeetokenenabled-j34</code> | <code>genesis_param</code> | <code>stable</code> | <code>modules/evm/types/params.go:19</code> | <code>KeyFeeTokenEnabled = []byte("KeyFeeTokenEnabled")</code> |
| <code>interface.6.gen-evm-keymaxdynamicbasefeedownwardadjustment-j34</code> | <code>genesis_param</code> | <code>stable</code> | <code>modules/evm/types/params.go:25</code> | <code>KeyMaxDynamicBaseFeeDownwardAdjustment = []byte("KeyMaxDynamicBaseFeeDownwardAdjustment")</code> |
| <code>interface.6.gen-evm-keymaxdynamicbasefeeupwardadjustment-j34</code> | <code>genesis_param</code> | <code>stable</code> | <code>modules/evm/types/params.go:24</code> | <code>KeyMaxDynamicBaseFeeUpwardAdjustment = []byte("KeyMaxDynamicBaseFeeUpwardAdjustment")</code> |
| <code>interface.6.gen-evm-keymaxfeepergas-j34</code> | <code>genesis_param</code> | <code>stable</code> | <code>modules/evm/types/params.go:22</code> | <code>KeyMaxFeePerGas = []byte("KeyMaximumFeePerGas")</code> |
| <code>interface.6.gen-evm-keymaxfeetokenrateage-j34</code> | <code>genesis_param</code> | <code>stable</code> | <code>modules/evm/types/params.go:16</code> | <code>KeyMaxFeeTokenRateAge = []byte("KeyMaxFeeTokenRateAge")</code> |
| <code>interface.6.gen-evm-keymaxfeetokenspread-j34</code> | <code>genesis_param</code> | <code>stable</code> | <code>modules/evm/types/params.go:18</code> | <code>KeyMaxFeeTokenSpread = []byte("KeyMaxFeeTokenSpread")</code> |
| <code>interface.6.gen-evm-keyminfeepergas-j34</code> | <code>genesis_param</code> | <code>stable</code> | <code>modules/evm/types/params.go:21</code> | <code>KeyMinFeePerGas = []byte("KeyMinFeePerGas")</code> |
| <code>interface.6.gen-evm-keypaxsstoresetgaseip2200-j34</code> | <code>genesis_param</code> | <code>stable</code> | <code>modules/evm/types/params.go:27</code> | <code>KeyPaxSstoreSetGasEIP2200 = []byte("KeyPaxSstoreSetGasEIP2200")</code> |
| <code>interface.6.gen-evm-keyprioritynormalizer-j34</code> | <code>genesis_param</code> | <code>stable</code> | <code>modules/evm/types/params.go:20</code> | <code>KeyPriorityNormalizer = []byte("KeyPriorityNormalizer")</code> |
| <code>interface.6.gen-evm-keyregisterpointerdisabled-j34</code> | <code>genesis_param</code> | <code>stable</code> | <code>modules/evm/types/params.go:31</code> | <code>KeyRegisterPointerDisabled = []byte("KeyRegisterPointerDisabled")</code> |
| <code>interface.6.gen-evm-keytargetgasusedperblock-j34</code> | <code>genesis_param</code> | <code>stable</code> | <code>modules/evm/types/params.go:26</code> | <code>KeyTargetGasUsedPerBlock = []byte("KeyTargetGasUsedPerBlock")</code> |
| <code>interface.6.gen-evm-keywhitelistedcwcodehashesfordelegatecall-j34</code> | <code>genesis_param</code> | <code>stable</code> | <code>modules/evm/types/params.go:30</code> | <code>KeyWhitelistedCwCodeHashesForDelegateCall = []byte("KeyWhitelistedCwCodeHashesForDelegateCall")</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.6.gen-evm-keyallowedfeedenoms-j34</code> | notes | evm params subspace key "KeyAllowedFeeDenoms" |
| <code>interface.6.gen-evm-keybasefeepergas-j34</code> | notes | evm params subspace key "KeyBaseFeePerGas"; marked deprecated in modules/evm/types/params.go:28 |
| <code>interface.6.gen-evm-keydelivertxhookwasmgaslimit-j34</code> | notes | evm params subspace key "KeyDeliverTxHookWasmGasLimit" |
| <code>interface.6.gen-evm-keyfeetokendistribution-j34</code> | notes | evm params subspace key "KeyFeeTokenDistribution" |
| <code>interface.6.gen-evm-keyfeetokenenabled-j34</code> | notes | evm params subspace key "KeyFeeTokenEnabled" |
| <code>interface.6.gen-evm-keymaxdynamicbasefeedownwardadjustment-j34</code> | notes | evm params subspace key "KeyMaxDynamicBaseFeeDownwardAdjustment" |
| <code>interface.6.gen-evm-keymaxdynamicbasefeeupwardadjustment-j34</code> | notes | evm params subspace key "KeyMaxDynamicBaseFeeUpwardAdjustment" |
| <code>interface.6.gen-evm-keymaxfeepergas-j34</code> | notes | evm params subspace key "KeyMaximumFeePerGas" |
| <code>interface.6.gen-evm-keymaxfeetokenrateage-j34</code> | notes | evm params subspace key "KeyMaxFeeTokenRateAge" |
| <code>interface.6.gen-evm-keymaxfeetokenspread-j34</code> | notes | evm params subspace key "KeyMaxFeeTokenSpread" |
| <code>interface.6.gen-evm-keyminfeepergas-j34</code> | notes | evm params subspace key "KeyMinFeePerGas" |
| <code>interface.6.gen-evm-keypaxsstoresetgaseip2200-j34</code> | notes | evm params subspace key "KeyPaxSstoreSetGasEIP2200" |
| <code>interface.6.gen-evm-keyprioritynormalizer-j34</code> | notes | evm params subspace key "KeyPriorityNormalizer" |
| <code>interface.6.gen-evm-keyregisterpointerdisabled-j34</code> | notes | evm params subspace key "KeyRegisterPointerDisabled"; marked deprecated in modules/evm/types/params.go:28 |
| <code>interface.6.gen-evm-keytargetgasusedperblock-j34</code> | notes | evm params subspace key "KeyTargetGasUsedPerBlock" |
| <code>interface.6.gen-evm-keywhitelistedcwcodehashesfordelegatecall-j34</code> | notes | evm params subspace key "KeyWhitelistedCwCodeHashesForDelegateCall"; marked deprecated in modules/evm/types/params.go:28 |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.6.8.1</code> | req | 6008.1 |
| <code>acceptance.6.8.1</code> | scenario | Given the surface at modules/evm/types/params.go:15-31, when TestEVMParamKeysFrozen runs, then every entry equals the frozen table |
| <code>acceptance.6.8.1</code> | test | modules/evm/types/params_keys_freeze_test.go::TestEVMParamKeysFrozen (planned) |
| <code>acceptance.6.8.1</code> | qualification | UNRUN |
| <code>req.6008</code> | ac_1 | The evm params subspace keys keep their byte strings (modules/evm/types/params.go:15-31). |
| <code>capability.6.11.5</code> | title | evm params subspace keys freeze |
| <code>capability.6.11.5</code> | reqs | 6008.1 |

## proto-surface

No interface block names this anchor; the citing criteria and rules below define its scope.

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.6.4.1</code> | req | 6004.1 |
| <code>acceptance.6.4.1</code> | scenario | Given the surface at api/evm/params.proto:1 and api/evm/tx.proto:1, when TestEVMProtoSurfaceFrozen runs, then every entry equals the frozen table |
| <code>acceptance.6.4.1</code> | test | modules/evm/types/proto_freeze_test.go::TestEVMProtoSurfaceFrozen (planned) |
| <code>acceptance.6.4.1</code> | qualification | UNRUN |
| <code>req.6004</code> | ac_1 | The evm proto messages keep their full names and field numbers (api/evm/params.proto:1, api/evm/tx.proto:1, api/evm/query.proto:1). |
| <code>capability.6.11.1</code> | title | evm proto message names and field numbers freeze |
| <code>capability.6.11.1</code> | reqs | 6004.1 |

## storage-keys

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.6.go-evm-types-validateaccountfeedenomgenesisentry-j34</code> | <code>go_exported</code> | <code>new</code> | <code>modules/evm/types/fee_denom_genesis.go</code> (planned) | <code>func ValidateAccountFeeDenomGenesisEntry(prefix, key, value []byte) error</code> |
| <code>interface.6.key-evm-accountfeedenomkeyprefix-j34</code> | <code>storage_key</code> | <code>change</code> | <code>modules/evm/types/keys.go:71</code> | <code>AccountFeeDenomKeyPrefix = []byte{0x23}</code> |
| <code>interface.6.key-evm-accounttransientstatekeyprefix-j34</code> | <code>storage_key</code> | <code>stable</code> | <code>modules/evm/types/keys.go:34</code> | <code>AccountTransientStateKeyPrefix = []byte{0x05}</code> |
| <code>interface.6.key-evm-antefeetokenchargeprefix-j34</code> | <code>storage_key</code> | <code>stable</code> | <code>modules/evm/types/keys.go:57</code> | <code>AnteFeeTokenChargePrefix = []byte{0x24}</code> |
| <code>interface.6.key-evm-antesurplusprefix-j34</code> | <code>storage_key</code> | <code>stable</code> | <code>modules/evm/types/keys.go:58</code> | <code>AnteSurplusPrefix = []byte{0x18}</code> |
| <code>interface.6.key-evm-basefeepergasprefix-j34</code> | <code>storage_key</code> | <code>stable</code> | <code>modules/evm/types/keys.go:62</code> | <code>BaseFeePerGasPrefix = []byte{0x1b}</code> |
| <code>interface.6.key-evm-blockbloomprefix-j34</code> | <code>storage_key</code> | <code>stable</code> | <code>modules/evm/types/keys.go:42</code> | <code>BlockBloomPrefix = []byte{0x0d}</code> |
| <code>interface.6.key-evm-codehashkeyprefix-j34</code> | <code>storage_key</code> | <code>stable</code> | <code>modules/evm/types/keys.go:37</code> | <code>CodeHashKeyPrefix = []byte{0x08}</code> |
| <code>interface.6.key-evm-codekeyprefix-j34</code> | <code>storage_key</code> | <code>stable</code> | <code>modules/evm/types/keys.go:36</code> | <code>CodeKeyPrefix = []byte{0x07}</code> |
| <code>interface.6.key-evm-codesizekeyprefix-j34</code> | <code>storage_key</code> | <code>stable</code> | <code>modules/evm/types/keys.go:38</code> | <code>CodeSizeKeyPrefix = []byte{0x09}</code> |
| <code>interface.6.key-evm-deferredinfoprefix-j34</code> | <code>storage_key</code> | <code>stable</code> | <code>modules/evm/types/keys.go:59</code> | <code>DeferredInfoPrefix = []byte{0x19}</code> |
| <code>interface.6.key-evm-evmaddresstolayerxdidkeyprefix-j34</code> | <code>storage_key</code> | <code>stable</code> | <code>modules/evm/types/keys.go:68</code> | <code>EVMAddressToLayerXDidKeyPrefix = []byte{0x20}</code> |
| <code>interface.6.key-evm-evmaddresstopaxaddresskeyprefix-j34</code> | <code>storage_key</code> | <code>stable</code> | <code>modules/evm/types/keys.go:30</code> | <code>EVMAddressToPaxAddressKeyPrefix = []byte{0x01}</code> |
| <code>interface.6.key-evm-evmonlyblockbloomprefix-j34</code> | <code>storage_key</code> | <code>stable</code> | <code>modules/evm/types/keys.go:64</code> | <code>EvmOnlyBlockBloomPrefix = []byte{0x1d}</code> |
| <code>interface.6.key-evm-layerxbindnoncekeyprefix-j34</code> | <code>storage_key</code> | <code>stable</code> | <code>modules/evm/types/keys.go:70</code> | <code>LayerXBindNonceKeyPrefix = []byte{0x22}</code> |
| <code>interface.6.key-evm-layerxdidtoevmaddresskeyprefix-j34</code> | <code>storage_key</code> | <code>stable</code> | <code>modules/evm/types/keys.go:69</code> | <code>LayerXDidToEVMAddressKeyPrefix = []byte{0x21}</code> |
| <code>interface.6.key-evm-legacyblockbloomcutoffheightkey-j34</code> | <code>storage_key</code> | <code>stable</code> | <code>modules/evm/types/keys.go:61</code> | <code>LegacyBlockBloomCutoffHeightKey = []byte{0x1a}</code> |
| <code>interface.6.key-evm-nextbasefeepergasprefix-j34</code> | <code>storage_key</code> | <code>stable</code> | <code>modules/evm/types/keys.go:63</code> | <code>NextBaseFeePerGasPrefix = []byte{0x1c}</code> |
| <code>interface.6.key-evm-noncebumpprefix-j34</code> | <code>storage_key</code> | <code>stable</code> | <code>modules/evm/types/keys.go:66</code> | <code>NonceBumpPrefix = []byte{0x1f}</code> |
| <code>interface.6.key-evm-noncekeyprefix-j34</code> | <code>storage_key</code> | <code>stable</code> | <code>modules/evm/types/keys.go:39</code> | <code>NonceKeyPrefix = []byte{0x0a}</code> |
| <code>interface.6.key-evm-paxaddresstoevmaddresskeyprefix-j34</code> | <code>storage_key</code> | <code>stable</code> | <code>modules/evm/types/keys.go:31</code> | <code>PaxAddressToEVMAddressKeyPrefix = []byte{0x02}</code> |
| <code>interface.6.key-evm-pointercwcodeprefix-j34</code> | <code>storage_key</code> | <code>stable</code> | <code>modules/evm/types/keys.go:54</code> | <code>PointerCWCodePrefix = []byte{0x16}</code> |
| <code>interface.6.key-evm-pointererc20nativeprefix-j34</code> | <code>storage_key</code> | <code>stable</code> | <code>modules/evm/types/keys.go:75</code> | <code>PointerERC20NativePrefix = []byte{0x0}</code> |
| <code>interface.6.key-evm-pointerregistryprefix-j34</code> | <code>storage_key</code> | <code>stable</code> | <code>modules/evm/types/keys.go:53</code> | <code>PointerRegistryPrefix = []byte{0x15}</code> |
| <code>interface.6.key-evm-pointerreverseregistryprefix-j34</code> | <code>storage_key</code> | <code>stable</code> | <code>modules/evm/types/keys.go:55</code> | <code>PointerReverseRegistryPrefix = []byte{0x17}</code> |
| <code>interface.6.key-evm-receiptkeyprefix-j34</code> | <code>storage_key</code> | <code>stable</code> | <code>modules/evm/types/keys.go:40</code> | <code>ReceiptKeyPrefix = []byte{0x0b}</code> |
| <code>interface.6.key-evm-replayedheight-j34</code> | <code>storage_key</code> | <code>stable</code> | <code>modules/evm/types/keys.go:50</code> | <code>ReplayedHeight = []byte{0x13}</code> |
| <code>interface.6.key-evm-replayinitialheight-j34</code> | <code>storage_key</code> | <code>stable</code> | <code>modules/evm/types/keys.go:51</code> | <code>ReplayInitialHeight = []byte{0x14}</code> |
| <code>interface.6.key-evm-replayseenaddrprefix-j34</code> | <code>storage_key</code> | <code>stable</code> | <code>modules/evm/types/keys.go:49</code> | <code>ReplaySeenAddrPrefix = []byte{0x12}</code> |
| <code>interface.6.key-evm-statekeyprefix-j34</code> | <code>storage_key</code> | <code>stable</code> | <code>modules/evm/types/keys.go:32</code> | <code>StateKeyPrefix = []byte{0x03}</code> |
| <code>interface.6.key-evm-transientmodulestatekeyprefix-j34</code> | <code>storage_key</code> | <code>stable</code> | <code>modules/evm/types/keys.go:35</code> | <code>TransientModuleStateKeyPrefix = []byte{0x06}</code> |
| <code>interface.6.key-evm-transientstatekeyprefix-j34</code> | <code>storage_key</code> | <code>stable</code> | <code>modules/evm/types/keys.go:33</code> | <code>TransientStateKeyPrefix = []byte{0x04}</code> |
| <code>interface.6.key-evm-txhashesprefix-j34</code> | <code>storage_key</code> | <code>stable</code> | <code>modules/evm/types/keys.go:43</code> | <code>TxHashesPrefix = []byte{0x0e}</code> |
| <code>interface.6.key-evm-whitelistedcodehashesforbanksendprefix-j34</code> | <code>storage_key</code> | <code>stable</code> | <code>modules/evm/types/keys.go:41</code> | <code>WhitelistedCodeHashesForBankSendPrefix = []byte{0x0c}</code> |
| <code>interface.6.key-evm-whitelistedcodehashesfordelegatecallprefix-j34</code> | <code>storage_key</code> | <code>stable</code> | <code>modules/evm/types/keys.go:44</code> | <code>WhitelistedCodeHashesForDelegateCallPrefix = []byte{0x0f}</code> |
| <code>interface.6.key-evm-zerostoragecleanupcheckpointkey-j34</code> | <code>storage_key</code> | <code>stable</code> | <code>modules/evm/types/keys.go:65</code> | <code>ZeroStorageCleanupCheckpointKey = []byte{0x1e}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.6.go-evm-types-validateaccountfeedenomgenesisentry-j34</code> | notes | planned file; refuses a 0x23 entry whose key is not 20 bytes or whose denom fails sdk.ValidateDenom (task 6.7.6) |
| <code>interface.6.key-evm-accountfeedenomkeyprefix-j34</code> | notes | evm KV prefix 0x23; read and written inside modules/evm only; the fee-denom preferences under this prefix are absent from ExportGenesis (modules/evm/genesis.go:72-82) and ExportGenesisStream (modules/evm/genesis.go:151-161), tasks 6.7.6, 6.7.7 and 6.7.9 |
| <code>interface.6.key-evm-accounttransientstatekeyprefix-j34</code> | notes | evm KV prefix 0x05 // deprecated; read and written inside modules/evm only |
| <code>interface.6.key-evm-antefeetokenchargeprefix-j34</code> | notes | evm KV prefix 0x24 // transient; read and written inside modules/evm only |
| <code>interface.6.key-evm-antesurplusprefix-j34</code> | notes | evm KV prefix 0x18 // transient; read and written inside modules/evm only |
| <code>interface.6.key-evm-basefeepergasprefix-j34</code> | notes | evm KV prefix 0x1b; read and written inside modules/evm only |
| <code>interface.6.key-evm-blockbloomprefix-j34</code> | notes | evm KV prefix 0x0d; read and written inside modules/evm only |
| <code>interface.6.key-evm-codehashkeyprefix-j34</code> | notes | evm KV prefix 0x08; 1 uses outside modules/evm |
| <code>interface.6.key-evm-codekeyprefix-j34</code> | notes | evm KV prefix 0x07; 1 uses outside modules/evm |
| <code>interface.6.key-evm-codesizekeyprefix-j34</code> | notes | evm KV prefix 0x09; 1 uses outside modules/evm |
| <code>interface.6.key-evm-deferredinfoprefix-j34</code> | notes | evm KV prefix 0x19 // transient; 1 uses outside modules/evm |
| <code>interface.6.key-evm-evmaddresstolayerxdidkeyprefix-j34</code> | notes | evm KV prefix 0x20; read and written inside modules/evm only |
| <code>interface.6.key-evm-evmaddresstopaxaddresskeyprefix-j34</code> | notes | evm KV prefix 0x01; read and written inside modules/evm only |
| <code>interface.6.key-evm-evmonlyblockbloomprefix-j34</code> | notes | evm KV prefix 0x1d; read and written inside modules/evm only |
| <code>interface.6.key-evm-layerxbindnoncekeyprefix-j34</code> | notes | evm KV prefix 0x22; read and written inside modules/evm only |
| <code>interface.6.key-evm-layerxdidtoevmaddresskeyprefix-j34</code> | notes | evm KV prefix 0x21; read and written inside modules/evm only |
| <code>interface.6.key-evm-legacyblockbloomcutoffheightkey-j34</code> | notes | evm KV prefix 0x1a; read and written inside modules/evm only |
| <code>interface.6.key-evm-nextbasefeepergasprefix-j34</code> | notes | evm KV prefix 0x1c; read and written inside modules/evm only |
| <code>interface.6.key-evm-noncebumpprefix-j34</code> | notes | evm KV prefix 0x1f // transient; read and written inside modules/evm only |
| <code>interface.6.key-evm-noncekeyprefix-j34</code> | notes | evm KV prefix 0x0a; 1 uses outside modules/evm |
| <code>interface.6.key-evm-paxaddresstoevmaddresskeyprefix-j34</code> | notes | evm KV prefix 0x02; read and written inside modules/evm only |
| <code>interface.6.key-evm-pointercwcodeprefix-j34</code> | notes | evm KV prefix 0x16; read and written inside modules/evm only |
| <code>interface.6.key-evm-pointererc20nativeprefix-j34</code> | notes | evm KV prefix 0x0; read and written inside modules/evm only |
| <code>interface.6.key-evm-pointerregistryprefix-j34</code> | notes | evm KV prefix 0x15; read and written inside modules/evm only |
| <code>interface.6.key-evm-pointerreverseregistryprefix-j34</code> | notes | evm KV prefix 0x17; read and written inside modules/evm only |
| <code>interface.6.key-evm-receiptkeyprefix-j34</code> | notes | evm KV prefix 0x0b; read and written inside modules/evm only |
| <code>interface.6.key-evm-replayedheight-j34</code> | notes | evm KV prefix 0x13; read and written inside modules/evm only |
| <code>interface.6.key-evm-replayinitialheight-j34</code> | notes | evm KV prefix 0x14; read and written inside modules/evm only |
| <code>interface.6.key-evm-replayseenaddrprefix-j34</code> | notes | evm KV prefix 0x12; read and written inside modules/evm only |
| <code>interface.6.key-evm-statekeyprefix-j34</code> | notes | evm KV prefix 0x03; 4 uses outside modules/evm |
| <code>interface.6.key-evm-transientmodulestatekeyprefix-j34</code> | notes | evm KV prefix 0x06 // deprecated; read and written inside modules/evm only |
| <code>interface.6.key-evm-transientstatekeyprefix-j34</code> | notes | evm KV prefix 0x04 // deprecated; read and written inside modules/evm only |
| <code>interface.6.key-evm-txhashesprefix-j34</code> | notes | evm KV prefix 0x0e // deprecated; 2 uses outside modules/evm |
| <code>interface.6.key-evm-whitelistedcodehashesforbanksendprefix-j34</code> | notes | evm KV prefix 0x0c; read and written inside modules/evm only |
| <code>interface.6.key-evm-whitelistedcodehashesfordelegatecallprefix-j34</code> | notes | evm KV prefix 0x0f; read and written inside modules/evm only |
| <code>interface.6.key-evm-zerostoragecleanupcheckpointkey-j34</code> | notes | evm KV prefix 0x1e; read and written inside modules/evm only |

Contract context <code>contract_context.6.evm-genesis-fee-denom</code>:

| Field | Frozen rule |
|---|---|
| interface | modules/evm/genesis.go: InitGenesis, ExportGenesis, ExportGenesisStream; modules/evm/types: ValidateAccountFeeDenomGenesisEntry |
| storage | Prefix 0x23 AccountFeeDenomKeyPrefix, key 20-byte address, value denom string |
| errors | InitGenesis panics on an invalid 0x23 entry |
| events | none |
| gas | none |
| activation | none |
| invariants | Export then import preserves every fee-denom preference |
| pause | none |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.6.29.1</code> | req | 6029.1 |
| <code>acceptance.6.29.1</code> | scenario | Given 0x23 entries with a 20-byte key and denom "usid", a 19-byte key, and denom "1x", when ValidateAccountFeeDenomGenesisEntry runs, then the first passes and the other two return errors |
| <code>acceptance.6.29.1</code> | test | modules/evm/types/fee_denom_genesis_test.go::TestValidateAccountFeeDenomGenesisEntry (planned) |
| <code>acceptance.6.29.1</code> | qualification | UNRUN |
| <code>req.6029</code> | ac_1 | A 0x23 genesis entry with a 20-byte key and a valid denom passes and a short key or an invalid denom is refused (modules/evm/types/keys.go:71, :190). |
| <code>acceptance.6.36.1</code> | req | 6036.1 |
| <code>acceptance.6.36.1</code> | scenario | Given account A with fee denom "usid", when genesis is exported and imported into a fresh application, then GetAccountFeeDenom(A) returns "usid" |
| <code>acceptance.6.36.1</code> | test | modules/evm/genesis_fee_denom_test.go::TestExportImportAccountFeeDenom (planned) |
| <code>acceptance.6.36.1</code> | qualification | UNRUN |
| <code>req.6036</code> | ac_1 | A fee-denom preference under prefix 0x23 survives ExportGenesis and InitGenesis, which drop it today (modules/evm/genesis.go:72-82, modules/evm/types/keys.go:71). |
| <code>acceptance.6.41.1</code> | req | 6041.1 |
| <code>acceptance.6.41.1</code> | scenario | Given account A with fee denom "usid", when every chunk of ExportGenesisStream is collected, then one Serialized entry has prefix 0x23, key A and value "usid" |
| <code>acceptance.6.41.1</code> | test | modules/evm/genesis_stream_fee_denom_test.go::TestExportGenesisStreamAccountFeeDenom (planned) |
| <code>acceptance.6.41.1</code> | qualification | UNRUN |
| <code>req.6041</code> | ac_1 | ExportGenesisStream emits the 0x23 fee-denom entries that ExportGenesis emits after task 6.7.7 (modules/evm/genesis.go:151-161). |
| <code>acceptance.6.6.1</code> | req | 6006.1 |
| <code>acceptance.6.6.1</code> | scenario | Given the surface at modules/evm/types/keys.go:29-71, when TestEVMStorePrefixesFrozen runs, then every entry equals the frozen table |
| <code>acceptance.6.6.1</code> | test | modules/evm/types/keys_freeze_test.go::TestEVMStorePrefixesFrozen (planned) |
| <code>acceptance.6.6.1</code> | qualification | UNRUN |
| <code>req.6006</code> | ac_1 | Each evm store prefix keeps its byte and no two prefixes share a byte (modules/evm/types/keys.go:29-71). |
| <code>capability.6.11.3</code> | title | evm store prefixes 0x01-0x24 freeze |
| <code>capability.6.11.3</code> | reqs | 6006.1 |
| <code>capability.6.7.6</code> | title | Fee-denom genesis validation |
| <code>capability.6.7.6</code> | reqs | 6029.1 |
| <code>capability.6.7.7</code> | title | Fee-denom genesis export |
| <code>capability.6.7.7</code> | reqs | 6036.1 |
| <code>capability.6.7.9</code> | title | Fee-denom genesis stream |
| <code>capability.6.7.9</code> | reqs | 6041.1 |
| <code>resolution.17.713</code> | title | decision.610 binds: Fee-denom preferences are genesis state |
| <code>resolution.17.713</code> | applies_to | decision.610<br>task.6.7.6<br>task.6.7.7<br>task.6.7.9 |
| <code>resolution.17.713</code> | source | decision.610 in fragments/J34/decision.kvx; tasks from affects |
| <code>resolution.17.715</code> | title | decision.612 binds: Stale binding call-form task is superseded |
| <code>resolution.17.715</code> | applies_to | decision.612<br>task.6.7.6 |
| <code>resolution.17.715</code> | source | decision.612 in fragments/J34/decision.kvx; tasks from affects |
