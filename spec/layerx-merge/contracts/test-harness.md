# Test harness contract

The repository includes C harnesses, Go process-block utilities, OCC helpers and integration runners. The tables below identify their current declarations and fixture sources.

The tables identify code declarations and configuration names. Activation, authentication and transport registration are defined by the linked implementations.

## c-harness

| Kind | Declaration | Source |
|---|---|---|
| `c_symbol` | `int lxp_test_assert_bytes_eq(const void *expected, const void *produced, size_t length, const char *file, int line);` | [tests/lxp_test_harness.h](../../../tests/lxp_test_harness.h) |
| `c_symbol` | `int lxp_test_assert_result(lxp_result expected, lxp_result produced, const char *file, int line);` | [tests/lxp_test_harness.h](../../../tests/lxp_test_harness.h) |
| `c_symbol` | `int lxp_test_assert_u64_eq(uint64_t expected, uint64_t produced, const char *file, int line);` | [tests/lxp_test_harness.h](../../../tests/lxp_test_harness.h) |
| `c_symbol` | `typedef int (*lxp_test_function)(void);` | [tests/lxp_test_harness.h](../../../tests/lxp_test_harness.h) |
| `c_symbol` | `int lxp_test_main(int argc, char **argv);` | [tests/lxp_test_harness.h](../../../tests/lxp_test_harness.h) |
| `c_symbol` | `int lxp_test_register(const char *name, lxp_test_function function);` | [tests/lxp_test_harness.h](../../../tests/lxp_test_harness.h) |
| `c_symbol` | `int lxp_test_run_all(int list_only);` | [tests/lxp_test_harness.h](../../../tests/lxp_test_harness.h) |
| `c_symbol` | `int lxp_test_run_one(const char *name);` | [tests/lxp_test_harness.h](../../../tests/lxp_test_harness.h) |
| source | — | [tests/protocol/lxp_test_smoke.c](../../../tests/protocol/lxp_test_smoke.c) |

## fixtures

| Kind | Declaration | Source |
|---|---|---|
| source | — | [tests/fixtures/asset/bound-native-withdrawal/activity.lxa](../../../tests/fixtures/asset/bound-native-withdrawal/activity.lxa) |
| source | — | [tests/fixtures/authority/legacy-paid-grant-replay.bin](../../../tests/fixtures/authority/legacy-paid-grant-replay.bin) |
| source | — | [tests/fixtures/custody/daemon-credit-receipt/activity](../../../tests/fixtures/custody/daemon-credit-receipt/activity) |
| source | — | [tests/fixtures/example/contracts/README](../../../tests/fixtures/example/contracts/README) |
| source | — | [tests/fixtures/fee-params-v3.bin](../../../tests/fixtures/fee-params-v3.bin) |
| source | — | [tests/fixtures/fee-params-v4.bin](../../../tests/fixtures/fee-params-v4.bin) |
| source | — | [tests/fixtures/governance/handover-maintenance/activity](../../../tests/fixtures/governance/handover-maintenance/activity) |
| source | — | [tests/fixtures/pay1-retired-genesis.manifest](../../../tests/fixtures/pay1-retired-genesis.manifest) |
| source | — | [tests/fixtures/pay1-retired-issuance.lxs](../../../tests/fixtures/pay1-retired-issuance.lxs) |
| source | — | [tests/fixtures/programs/account-bound-call/execution.json](../../../tests/fixtures/programs/account-bound-call/execution.json) |
| source | — | [tests/fixtures/public-testnet-genesis/00000000000000000000.lxs](../../../tests/fixtures/public-testnet-genesis/00000000000000000000.lxs) |
| source | — | [tests/fixtures/public-testnet-genesis-v3/00000000000000000000.lxs](../../../tests/fixtures/public-testnet-genesis-v3/00000000000000000000.lxs) |
| source | — | [tests/fixtures/public-testnet-modules/registry.json](../../../tests/fixtures/public-testnet-modules/registry.json) |
| source | — | [tests/fixtures/receipt-supply-v2.bin](../../../tests/fixtures/receipt-supply-v2.bin) |
| source | — | [tests/fixtures/regen-retired-genesis.sh](../../../tests/fixtures/regen-retired-genesis.sh) |
| source | — | [tests/fixtures/trading-payloads/vectors.json](../../../tests/fixtures/trading-payloads/vectors.json) |
| source | — | [tests/fixtures/web/observation-activity.hex](../../../tests/fixtures/web/observation-activity.hex) |
| source | — | [tests/replay/golden/history.lxl](../../../tests/replay/golden/history.lxl) |

## go-testutil

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `func BankTransfer(tCtx *utils.TestContext, count int) []*utils.TestMessage` | [tests/occ/messages/test_msgs.go](../../../tests/occ/messages/test_msgs.go) |
| `go_exported` | `func ERC20toCWAssets(tCtx *utils.TestContext, count int) []*utils.TestMessage` | [tests/occ/messages/test_msgs.go](../../../tests/occ/messages/test_msgs.go) |
| `go_exported` | `func EVMTransferConflicting(tCtx *utils.TestContext, count int) []*utils.TestMessage` | [tests/occ/messages/test_msgs.go](../../../tests/occ/messages/test_msgs.go) |
| `go_exported` | `func EVMTransferNonConflicting(tCtx *utils.TestContext, count int) []*utils.TestMessage` | [tests/occ/messages/test_msgs.go](../../../tests/occ/messages/test_msgs.go) |
| `go_exported` | `func GovernanceSubmitProposal(tCtx *utils.TestContext, count int) []*utils.TestMessage` | [tests/occ/messages/test_msgs.go](../../../tests/occ/messages/test_msgs.go) |
| `go_exported` | `func WasmInstantiate(tCtx *utils.TestContext, count int) []*utils.TestMessage` | [tests/occ/messages/test_msgs.go](../../../tests/occ/messages/test_msgs.go) |
| `go_exported` | `func CompareStores(t *testing.T, storeKey sdk.StoreKey, expected store.KVStore, actual store.KVStore, testName string)` | [tests/occ/utils/utils.go](../../../tests/occ/utils/utils.go) |
| `go_exported` | `func Funds(amount int64) sdk.Coins` | [tests/occ/utils/utils.go](../../../tests/occ/utils/utils.go) |
| `go_exported` | `func JoinMsgs(msgsList ...[]*TestMessage) []*TestMessage` | [tests/occ/utils/utils.go](../../../tests/occ/utils/utils.go) |
| `go_exported` | `func NewSigner() TestAcct` | [tests/occ/utils/utils.go](../../../tests/occ/utils/utils.go) |
| `go_exported` | `func NewTestAccounts(count int) []TestAcct` | [tests/occ/utils/utils.go](../../../tests/occ/utils/utils.go) |
| `go_exported` | `func NewTestContext(tb testing.TB, testAccts []TestAcct, blockTime time.Time, workers int, occEnabled bool) *TestContext` | [tests/occ/utils/utils.go](../../../tests/occ/utils/utils.go) |
| `go_exported` | `func ProcessBlockDirect(testCtx *TestContext, txs [][]byte, occ bool) ([]types.Event, []*types.ExecTxResult, types.ResponseEndBlock, error)` | [tests/occ/utils/utils.go](../../../tests/occ/utils/utils.go) |
| `go_exported` | `func RunWithOCC(testCtx *TestContext, msgs []*TestMessage) ([]types.Event, []*types.ExecTxResult, types.ResponseEndBlock, error)` | [tests/occ/utils/utils.go](../../../tests/occ/utils/utils.go) |
| `go_exported` | `func RunWithoutOCC(testCtx *TestContext, msgs []*TestMessage) ([]types.Event, []*types.ExecTxResult, types.ResponseEndBlock, error)` | [tests/occ/utils/utils.go](../../../tests/occ/utils/utils.go) |
| `go_exported` | `func Shuffle(msgs []*TestMessage) []*TestMessage` | [tests/occ/utils/utils.go](../../../tests/occ/utils/utils.go) |
| `go_exported` | `type TestAcct struct` | [tests/occ/utils/utils.go](../../../tests/occ/utils/utils.go) |
| `go_exported` | `type TestContext struct` | [tests/occ/utils/utils.go](../../../tests/occ/utils/utils.go) |
| `go_exported` | `type TestMessage struct` | [tests/occ/utils/utils.go](../../../tests/occ/utils/utils.go) |
| `go_exported` | `func ToTxBytes(testCtx *TestContext, msgs []*TestMessage) [][]byte` | [tests/occ/utils/utils.go](../../../tests/occ/utils/utils.go) |
| `go_exported` | `func EpochKeeper(t testing.TB) (*keeper.Keeper, sdk.Context)` | [tests/testutil/keeper/epoch.go](../../../tests/testutil/keeper/epoch.go) |
| `go_exported` | `var EVMTestApp = v6120Applied(app.SetupWithDefaultHome(false, true, false))` | [tests/testutil/keeper/evm.go](../../../tests/testutil/keeper/evm.go) |
| `go_exported` | `func MockAddressPair() (sdk.AccAddress, common.Address)` | [tests/testutil/keeper/evm.go](../../../tests/testutil/keeper/evm.go) |
| `go_exported` | `func MockEVMKeeper(t *testing.T) (*evmkeeper.Keeper, sdk.Context)` | [tests/testutil/keeper/evm.go](../../../tests/testutil/keeper/evm.go) |
| `go_exported` | `func MockEVMKeeperPrecompiles(t *testing.T) (*evmkeeper.Keeper, sdk.Context)` | [tests/testutil/keeper/evm.go](../../../tests/testutil/keeper/evm.go) |
| `go_exported` | `func MockEVMKeeperWithPrecompiles() (*evmkeeper.Keeper, sdk.Context)` | [tests/testutil/keeper/evm.go](../../../tests/testutil/keeper/evm.go) |
| `go_exported` | `func MockPrivateKey() cryptotypes.PrivKey` | [tests/testutil/keeper/evm.go](../../../tests/testutil/keeper/evm.go) |
| `go_exported` | `func MustMockReceipt(t *testing.T, k *evmkeeper.Keeper, ctx sdk.Context, txHash common.Hash, receipt *evmtypes.Receipt)` | [tests/testutil/keeper/evm.go](../../../tests/testutil/keeper/evm.go) |
| `go_exported` | `func PrivateKeyToAddresses(privKey cryptotypes.PrivKey) (sdk.AccAddress, common.Address)` | [tests/testutil/keeper/evm.go](../../../tests/testutil/keeper/evm.go) |
| `go_exported` | `func TestApp(t *testing.T) *app.App` | [tests/testutil/keeper/epoch.go](../../../tests/testutil/keeper/epoch.go) |
| `go_exported` | `func UhpxCoins(amount int64) sdk.Coins` | [tests/testutil/keeper/evm.go](../../../tests/testutil/keeper/evm.go) |
| `go_exported` | `func WaitForReceipt(t *testing.T, k *evmkeeper.Keeper, ctx sdk.Context, txHash common.Hash) *evmtypes.Receipt` | [tests/testutil/keeper/evm.go](../../../tests/testutil/keeper/evm.go) |
| `go_exported` | `func WaitForReceiptFromStore(t *testing.T, k *evmkeeper.Keeper, ctx sdk.Context, txHash common.Hash) *evmtypes.Receipt` | [tests/testutil/keeper/evm.go](../../../tests/testutil/keeper/evm.go) |
| `go_exported` | `func Fill(x interface{}) interface{}` | [tests/testutil/nullify/nullify.go](../../../tests/testutil/nullify/nullify.go) |
| `go_exported` | `func (a *App) Ctx() sdk.Context` | [tests/testutil/processblock/common.go](../../../tests/testutil/processblock/common.go) |
| `go_exported` | `func (a *App) FastEpoch()` | [tests/testutil/processblock/genesisepoch.go](../../../tests/testutil/processblock/genesisepoch.go) |
| `go_exported` | `func (a *App) FundAccount(acc sdk.AccAddress, amount int64)` | [tests/testutil/processblock/genesisbank.go](../../../tests/testutil/processblock/genesisbank.go) |
| `go_exported` | `func (a *App) GetAllValidators() []stakingtypes.Validator` | [tests/testutil/processblock/common.go](../../../tests/testutil/processblock/common.go) |
| `go_exported` | `type App struct` | [tests/testutil/processblock/common.go](../../../tests/testutil/processblock/common.go) |
| `go_exported` | `func (a *App) NewMinter(amount uint64)` | [tests/testutil/processblock/genesismint.go](../../../tests/testutil/processblock/genesismint.go) |
| `go_exported` | `func (a *App) RunBlock(txs []signing.Tx) (resultCodes []uint32)` | [tests/testutil/processblock/common.go](../../../tests/testutil/processblock/common.go) |
| `go_exported` | `func (a *App) Sign(account sdk.AccAddress, fee int64, msgs ...sdk.Msg) xauthsigning.Tx` | [tests/testutil/processblock/tx.go](../../../tests/testutil/processblock/tx.go) |
| `go_exported` | `func CommonPreset(app *App) *Preset` | [tests/testutil/processblock/presets.go](../../../tests/testutil/processblock/presets.go) |
| `go_exported` | `func GetKey(mnemonic string) cryptotypes.PrivKey` | [tests/testutil/processblock/tx.go](../../../tests/testutil/processblock/tx.go) |
| `go_exported` | `func FeeDenomPreference(app *processblock.App, payer FeeTokenSigner, nonce uint64, gasPrice *big.Int,` | [tests/testutil/processblock/msgs/feetoken.go](../../../tests/testutil/processblock/msgs/feetoken.go) |
| `go_exported` | `type FeeTokenSigner struct` | [tests/testutil/processblock/msgs/feetoken.go](../../../tests/testutil/processblock/msgs/feetoken.go) |
| `go_exported` | `func FeeTokenSignerFromSeed(seed byte) FeeTokenSigner` | [tests/testutil/processblock/msgs/feetoken.go](../../../tests/testutil/processblock/msgs/feetoken.go) |
| `go_exported` | `func FeeTokenTransfer(app *processblock.App, sender FeeTokenSigner, nonce uint64, gasPrice *big.Int,` | [tests/testutil/processblock/msgs/feetoken.go](../../../tests/testutil/processblock/msgs/feetoken.go) |
| `go_exported` | `func RecipientWord(recipient common.Address) [32]byte` | [tests/testutil/processblock/msgs/feetoken.go](../../../tests/testutil/processblock/msgs/feetoken.go) |
| `go_exported` | `func Send(from sdk.AccAddress, to sdk.AccAddress, amount int64) *banktypes.MsgSend` | [tests/testutil/processblock/msgs/bank.go](../../../tests/testutil/processblock/msgs/bank.go) |
| `go_exported` | `func SidioraBridgeIn(app *processblock.App, relayer FeeTokenSigner, nonce uint64, gasPrice *big.Int, chain uint64,` | [tests/testutil/processblock/msgs/feetoken.go](../../../tests/testutil/processblock/msgs/feetoken.go) |
| `go_exported` | `func NewTestApp(t *testing.T) *App` | [tests/testutil/processblock/common.go](../../../tests/testutil/processblock/common.go) |
| `go_exported` | `func NewTestAppWithGenesis(t *testing.T, override map[string]json.RawMessage) *App` | [tests/testutil/processblock/common.go](../../../tests/testutil/processblock/common.go) |
| `go_exported` | `func (p *Preset) AdminSign(app *App, msgs ...sdk.Msg) signing.Tx` | [tests/testutil/processblock/presets.go](../../../tests/testutil/processblock/presets.go) |
| `go_exported` | `var TxConfig = tx.NewTxConfig(Marshaler, tx.DefaultSignModes)` | [tests/testutil/processblock/tx.go](../../../tests/testutil/processblock/tx.go) |
| `go_exported` | `func Allocation(t *testing.T, app *processblock.App, f BlockRunnable, _ []signing.Tx) BlockRunnable` | [tests/testutil/processblock/verify/distribution.go](../../../tests/testutil/processblock/verify/distribution.go) |
| `go_exported` | `func Balance(t *testing.T, app *processblock.App, f BlockRunnable, txs []signing.Tx) BlockRunnable` | [tests/testutil/processblock/verify/bank.go](../../../tests/testutil/processblock/verify/bank.go) |
| `go_exported` | `func Epoch(t *testing.T, app *processblock.App, f BlockRunnable, _ []signing.Tx) BlockRunnable` | [tests/testutil/processblock/verify/epoch.go](../../../tests/testutil/processblock/verify/epoch.go) |
| `go_exported` | `func FeeTokenBalances(expectation FeeTokenExpectation) Verifier` | [tests/testutil/processblock/verify/feetoken.go](../../../tests/testutil/processblock/verify/feetoken.go) |
| `go_exported` | `type FeeTokenExpectation struct` | [tests/testutil/processblock/verify/feetoken.go](../../../tests/testutil/processblock/verify/feetoken.go) |
| `go_exported` | `func MintRelease(t *testing.T, app *processblock.App, f BlockRunnable, _ []signing.Tx) BlockRunnable` | [tests/testutil/processblock/verify/mint.go](../../../tests/testutil/processblock/verify/mint.go) |
| `go_exported` | `type Verifier func(*testing.T, *processblock.App, BlockRunnable, []signing.Tx) BlockRunnable` | [tests/testutil/processblock/verify/common.go](../../../tests/testutil/processblock/verify/common.go) |
| `go_exported` | `func AccAddress() string` | [tests/testutil/sample/sample.go](../../../tests/testutil/sample/sample.go) |

## integration-runner

| Kind | Declaration | Source |
|---|---|---|
| `go_internal` | `func evalExpr(expr string, envMap map[string]string) (bool, error)` | [tests/integration/runner/runner.go](../../../tests/integration/runner/runner.go) |
| `go_exported` | `var ExcludedSuites = map[string]string{` | [tests/integration/runner/suites.go](../../../tests/integration/runner/suites.go) |
| `go_exported` | `func RunFile(t *testing.T, path string, opts ...Option)` | [tests/integration/runner/runner.go](../../../tests/integration/runner/runner.go) |
| `go_exported` | `var Suites = []Suite{` | [tests/integration/runner/suites.go](../../../tests/integration/runner/suites.go) |
| source | — | [tests/integration/bank_module/send_funds_test.yaml](../../../tests/integration/bank_module/send_funds_test.yaml) |

## occ-harness

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `func CW20InstantiateMsg(tCtx *TestContext) []byte` | [tests/occ/utils/utils.go](../../../tests/occ/utils/utils.go) |

<a id="reachability"></a>
<a id="retirement"></a>

See the implementation sources above.
