# Test harness contract

Source revision: `2a1033aa2`. This contract freezes the cited interface and acceptance blocks of the specification. Statuses include the specification overrides. A source marked planned has line 0; a positive source line is verified in the pinned revision.

The context and decision tables reproduce the rules linked by interface identifiers. An anchor without a linked context gains no additional storage, gas, activation or pause rule here. Acceptance qualification remains UNRUN until its declared gate succeeds.

## c-harness

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.16.c-lxp-assert-bytes-macro-j59</code> | <code>c_symbol</code> | <code>stable</code> | <code>tests/lxp_test_harness.h:22</code> | <code>#define LXP_ASSERT_BYTES(expected, produced, length)</code> |
| <code>interface.16.c-lxp-assert-result-macro-j59</code> | <code>c_symbol</code> | <code>stable</code> | <code>tests/lxp_test_harness.h:24</code> | <code>#define LXP_ASSERT_RESULT(expected, produced)</code> |
| <code>interface.16.c-lxp-assert-u64-macro-j59</code> | <code>c_symbol</code> | <code>stable</code> | <code>tests/lxp_test_harness.h:20</code> | <code>#define LXP_ASSERT_U64(expected, produced)</code> |
| <code>interface.16.c-lxp-test-assert-bytes-eq-j59</code> | <code>c_symbol</code> | <code>stable</code> | <code>tests/lxp_test_harness.h:15</code> | <code>int lxp_test_assert_bytes_eq(const void *expected, const void *produced, size_t length, const char *file, int line);</code> |
| <code>interface.16.c-lxp-test-assert-result-j59</code> | <code>c_symbol</code> | <code>stable</code> | <code>tests/lxp_test_harness.h:17</code> | <code>int lxp_test_assert_result(lxp_result expected, lxp_result produced, const char *file, int line);</code> |
| <code>interface.16.c-lxp-test-assert-u64-eq-j59</code> | <code>c_symbol</code> | <code>stable</code> | <code>tests/lxp_test_harness.h:13</code> | <code>int lxp_test_assert_u64_eq(uint64_t expected, uint64_t produced, const char *file, int line);</code> |
| <code>interface.16.c-lxp-test-function-j59</code> | <code>c_symbol</code> | <code>stable</code> | <code>tests/lxp_test_harness.h:9</code> | <code>typedef int (*lxp_test_function)(void);</code> |
| <code>interface.16.c-lxp-test-main-j59</code> | <code>c_symbol</code> | <code>new</code> | <code>tests/lxp_test_harness.h</code> (planned) | <code>int lxp_test_main(int argc, char **argv);</code> |
| <code>interface.16.c-lxp-test-register-j59</code> | <code>c_symbol</code> | <code>stable</code> | <code>tests/lxp_test_harness.h:11</code> | <code>int lxp_test_register(const char *name, lxp_test_function function);</code> |
| <code>interface.16.c-lxp-test-run-all-j59</code> | <code>c_symbol</code> | <code>stable</code> | <code>tests/lxp_test_harness.h:12</code> | <code>int lxp_test_run_all(int list_only);</code> |
| <code>interface.16.c-lxp-test-run-one-j59</code> | <code>c_symbol</code> | <code>new</code> | <code>tests/lxp_test_harness.h</code> (planned) | <code>int lxp_test_run_one(const char *name);</code> |
| <code>interface.16.cli-lxp-test-harness-list-j59</code> | <code>cli_command</code> | <code>stable</code> | <code>tests/protocol/lxp_test_smoke.c:17</code> | <code>lxp_test_harness --list</code> |
| <code>interface.16.cli-lxp-test-harness-run-j59</code> | <code>cli_command</code> | <code>new</code> | <code>tests/protocol/lxp_test_smoke.c</code> (planned) | <code>&lt;suite binary&gt; --run &lt;case name&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.16.c-lxp-test-main-j59</code> | notes | Parses --list and --run &lt;name&gt;; any other argument exits 2. |
| <code>interface.16.c-lxp-test-register-j59</code> | notes | Refuses a null or empty name, a null function and a full registry of LXP_TEST_CAPACITY = 1024 cases. |
| <code>interface.16.c-lxp-test-run-one-j59</code> | notes | Returns 0 when the named case passes, 1 when it fails and 2 when no case has that name. |

Contract context <code>contract_context.16.c-test-harness</code>:

| Field | Frozen rule |
|---|---|
| interface | tests/lxp_test_harness.h:11-25 plus lxp_test_run_one and lxp_test_main (planned) |
| storage | static registry of LXP_TEST_CAPACITY = 1024 cases at tests/lxp_test_harness.c:7-15 |
| errors | lxp_test_register returns 1 on a null or empty name, null function or full registry; lxp_test_run_one returns 2 for an unknown name; lxp_test_main returns 2 on an unknown argument |
| events | one PASS &lt;name&gt; line on stdout or FAIL &lt;name&gt; line on stderr per executed case |
| gas | none |
| activation | wave 0 task 16.21.1 |
| invariants | a binary invoked with --run &lt;name&gt; executes exactly one registered case; no argument runs every case in registration order |
| pause | none |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.16.242.1</code> | req | 16242.1 |
| <code>acceptance.16.242.1</code> | scenario | Given the harness binary built from tests/lxp_test_harness.c and tests/protocol/lxp_test_smoke.c with cases protocol.no-op, harness.run-one-runs-named-case and harness.run-one-reports-unknown-case registered, when case harness.run-one-runs-named-case calls lxp_test_run_one("protocol.no-op"), then the call returns 0 |
| <code>acceptance.16.242.1</code> | test | tests/protocol/lxp_test_smoke.c::harness.run-one-runs-named-case (planned) |
| <code>acceptance.16.242.1</code> | qualification | UNRUN |
| <code>req.16242</code> | ac_1 | lxp_test_run_one (planned in tests/lxp_test_harness.h) runs only the registered case with the given name and returns its result, so a harness binary invoked with --run protocol.no-op prints exactly one PASS line; at HEAD tests/lxp_test_harness.c:29 can only run every case or list them. |
| <code>acceptance.16.242.2</code> | req | 16242.2 |
| <code>acceptance.16.242.2</code> | scenario | Given the same harness binary, when case harness.run-one-reports-unknown-case calls lxp_test_run_one("harness.no-such-case"), then the call returns 2 and prints nothing to stdout |
| <code>acceptance.16.242.2</code> | test | tests/protocol/lxp_test_smoke.c::harness.run-one-reports-unknown-case (planned) |
| <code>acceptance.16.242.2</code> | qualification | UNRUN |
| <code>req.16242</code> | ac_2 | lxp_test_run_one returns 2 for a name no case registered and lxp_test_main exits 2 when --run names an unknown case, instead of passing silently. |
| <code>acceptance.16.245.1</code> | req | 16245.1 |
| <code>acceptance.16.245.1</code> | scenario | Given the module-table suite registered with the C harness, when it runs with --run genesis.module-table.no-handover-type, then the default plan for LXP_PROTOCOL_VERSION_STATE_COMMITMENT has no module whose activity_types contain 0x00070009 and the binary exits 0 |
| <code>acceptance.16.245.1</code> | test | tests/test_genesis_module_table.c::genesis.module-table.no-handover-type (planned) |
| <code>acceptance.16.245.1</code> | qualification | UNRUN |
| <code>req.16245</code> | ac_1 | check_handover_registration at tests/test_genesis_module_table.c:181 and its call at tests/test_genesis_module_table.c:761 are removed, and a new case genesis.module-table.no-handover-type asserts that no module in the default plan from lxp_genesis_module_plan_default lists activity type 0x00070009 (LXP_GOVERNANCE_HANDOVER at include/layerx/lxp_handover.h:9). |
| <code>acceptance.16.246.1</code> | req | 16246.1 |
| <code>acceptance.16.246.1</code> | scenario | Given the builder CLI suite registered with the C harness, when it runs with --run genesis.builder-cli.snapshot-issuance-refused, then lxp_genesis_builder_cli_main with --migrate-snapshot-issuance returns non-zero, access(output_directory, F_OK) fails and the binary exits 0 |
| <code>acceptance.16.246.1</code> | test | tests/test_genesis_builder_cli.c::genesis.builder-cli.snapshot-issuance-refused (planned) |
| <code>acceptance.16.246.1</code> | qualification | UNRUN |
| <code>req.16246</code> | ac_1 | The snapshot issuance case at tests/test_genesis_builder_cli.c:218 expects lxp_genesis_builder_cli_main(6, migration_argv) to return non-zero and leave the output directory absent, matching the refusal that the node-runtime domain gives to --migrate-snapshot-issuance (dispatch at cmd/layerx-genesis/lxp_genesis_build_cli.c:726). |
| <code>capability.16.21.1</code> | title | C harness single-case selector |
| <code>capability.16.21.1</code> | reqs | 16242.1<br>16242.2 |
| <code>capability.16.23.1</code> | title | Genesis module-table suite without handover trust |
| <code>capability.16.23.1</code> | reqs | 16245.1 |
| <code>capability.16.23.2</code> | title | Genesis builder suite expects snapshot issuance refusal |
| <code>capability.16.23.2</code> | reqs | 16246.1 |
| <code>resolution.17.983</code> | title | decision.1621 binds: C suites are selected by Makefile target, and named cases by --run |
| <code>resolution.17.983</code> | applies_to | decision.1621<br>task.16.21.1<br>task.16.23.1<br>task.16.23.2 |
| <code>resolution.17.983</code> | source | decision.1621 in fragments/J59/decision.kvx; tasks from affects |

## fixtures

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.16.fmt-fixture-asset-j59</code> | <code>file_format</code> | <code>stable</code> | <code>tests/fixtures/asset/bound-native-withdrawal/activity.lxa:1</code> | <code>tests/fixtures/asset/bound-native-withdrawal/activity.lxa: binary fixture</code> |
| <code>interface.16.fmt-fixture-authority-j59</code> | <code>file_format</code> | <code>stable</code> | <code>tests/fixtures/authority/legacy-paid-grant-replay.bin:1</code> | <code>tests/fixtures/authority/legacy-paid-grant-replay.bin: binary fixture</code> |
| <code>interface.16.fmt-fixture-custody-j59</code> | <code>file_format</code> | <code>stable</code> | <code>tests/fixtures/custody/daemon-credit-receipt/activity:1</code> | <code>tests/fixtures/custody/daemon-credit-receipt/activity: binary fixture</code> |
| <code>interface.16.fmt-fixture-example-j59</code> | <code>file_format</code> | <code>stable</code> | <code>tests/fixtures/example/contracts/README:1</code> | <code>tests/fixtures/example/contracts/README: Example Solidity contracts with their compiled artifacts and Go bindings, used by</code> |
| <code>interface.16.fmt-fixture-fee-params-v3-bin-j59</code> | <code>file_format</code> | <code>stable</code> | <code>tests/fixtures/fee-params-v3.bin:1</code> | <code>tests/fixtures/fee-params-v3.bin: binary fixture</code> |
| <code>interface.16.fmt-fixture-fee-params-v4-bin-j59</code> | <code>file_format</code> | <code>stable</code> | <code>tests/fixtures/fee-params-v4.bin:1</code> | <code>tests/fixtures/fee-params-v4.bin: binary fixture</code> |
| <code>interface.16.fmt-fixture-governance-j59</code> | <code>file_format</code> | <code>stable</code> | <code>tests/fixtures/governance/handover-maintenance/activity:1</code> | <code>tests/fixtures/governance/handover-maintenance/activity: binary fixture</code> |
| <code>interface.16.fmt-fixture-pay1-retired-genesis-manifest-j59</code> | <code>file_format</code> | <code>stable</code> | <code>tests/fixtures/pay1-retired-genesis.manifest:1</code> | <code>tests/fixtures/pay1-retired-genesis.manifest: binary fixture</code> |
| <code>interface.16.fmt-fixture-pay1-retired-issuance-lxs-j59</code> | <code>file_format</code> | <code>stable</code> | <code>tests/fixtures/pay1-retired-issuance.lxs:1</code> | <code>tests/fixtures/pay1-retired-issuance.lxs: binary fixture</code> |
| <code>interface.16.fmt-fixture-programs-j59</code> | <code>file_format</code> | <code>stable</code> | <code>tests/fixtures/programs/account-bound-call/execution.json:1</code> | <code>tests/fixtures/programs/account-bound-call/execution.json: {</code> |
| <code>interface.16.fmt-fixture-public-testnet-genesis-j59</code> | <code>file_format</code> | <code>stable</code> | <code>tests/fixtures/public-testnet-genesis/00000000000000000000.lxs:1</code> | <code>tests/fixtures/public-testnet-genesis/00000000000000000000.lxs: binary fixture</code> |
| <code>interface.16.fmt-fixture-public-testnet-genesis-v3-j59</code> | <code>file_format</code> | <code>stable</code> | <code>tests/fixtures/public-testnet-genesis-v3/00000000000000000000.lxs:1</code> | <code>tests/fixtures/public-testnet-genesis-v3/00000000000000000000.lxs: binary fixture</code> |
| <code>interface.16.fmt-fixture-public-testnet-modules-j59</code> | <code>file_format</code> | <code>stable</code> | <code>tests/fixtures/public-testnet-modules/registry.json:1</code> | <code>tests/fixtures/public-testnet-modules/registry.json: {"schema_version":2,"assets":[{"asset":"b5a32b12029f8ddfb905f90f280f664b46390de0fc62770fc197dd87b18cd898","symbol":"LXR"</code> |
| <code>interface.16.fmt-fixture-receipt-supply-v2-bin-j59</code> | <code>file_format</code> | <code>stable</code> | <code>tests/fixtures/receipt-supply-v2.bin:1</code> | <code>tests/fixtures/receipt-supply-v2.bin: binary fixture</code> |
| <code>interface.16.fmt-fixture-regen-retired-genesis-sh-j59</code> | <code>file_format</code> | <code>stable</code> | <code>tests/fixtures/regen-retired-genesis.sh:1</code> | <code>tests/fixtures/regen-retired-genesis.sh: #!/usr/bin/env bash</code> |
| <code>interface.16.fmt-fixture-trading-payloads-j59</code> | <code>file_format</code> | <code>stable</code> | <code>tests/fixtures/trading-payloads/vectors.json:1</code> | <code>tests/fixtures/trading-payloads/vectors.json: {</code> |
| <code>interface.16.fmt-fixture-web-j59</code> | <code>file_format</code> | <code>stable</code> | <code>tests/fixtures/web/observation-activity.hex:1</code> | <code>tests/fixtures/web/observation-activity.hex: 000210010c010002020000000903000b000104000000186469643a6b65793a7765622d6174746573746f722d6f6e6505000000203e55e9643a89e218</code> |
| <code>interface.16.fmt-replay-golden-history-j59</code> | <code>file_format</code> | <code>stable</code> | <code>tests/replay/golden/history.lxl:1</code> | <code>tests/replay/golden/history.lxl: binary replay history</code> |

## go-testutil

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.16.go-occ-messages-banktransfer-j59</code> | <code>go_exported</code> | <code>stable</code> | <code>tests/occ/messages/test_msgs.go:116</code> | <code>func BankTransfer(tCtx *utils.TestContext, count int) []*utils.TestMessage</code> |
| <code>interface.16.go-occ-messages-erc20tocwassets-j59</code> | <code>go_exported</code> | <code>stable</code> | <code>tests/occ/messages/test_msgs.go:140</code> | <code>func ERC20toCWAssets(tCtx *utils.TestContext, count int) []*utils.TestMessage</code> |
| <code>interface.16.go-occ-messages-evmtransferconflicting-j59</code> | <code>go_exported</code> | <code>stable</code> | <code>tests/occ/messages/test_msgs.go:74</code> | <code>func EVMTransferConflicting(tCtx *utils.TestContext, count int) []*utils.TestMessage</code> |
| <code>interface.16.go-occ-messages-evmtransfernonconflicting-j59</code> | <code>go_exported</code> | <code>stable</code> | <code>tests/occ/messages/test_msgs.go:64</code> | <code>func EVMTransferNonConflicting(tCtx *utils.TestContext, count int) []*utils.TestMessage</code> |
| <code>interface.16.go-occ-messages-governancesubmitproposal-j59</code> | <code>go_exported</code> | <code>stable</code> | <code>tests/occ/messages/test_msgs.go:125</code> | <code>func GovernanceSubmitProposal(tCtx *utils.TestContext, count int) []*utils.TestMessage</code> |
| <code>interface.16.go-occ-messages-wasminstantiate-j59</code> | <code>go_exported</code> | <code>change</code> | <code>tests/occ/messages/test_msgs.go:44</code> | <code>func WasmInstantiate(tCtx *utils.TestContext, count int) []*utils.TestMessage</code> |
| <code>interface.16.go-occ-utils-comparestores-j59</code> | <code>go_exported</code> | <code>stable</code> | <code>tests/occ/utils/utils.go:369</code> | <code>func CompareStores(t *testing.T, storeKey sdk.StoreKey, expected store.KVStore, actual store.KVStore, testName string)</code> |
| <code>interface.16.go-occ-utils-funds-j59</code> | <code>go_exported</code> | <code>stable</code> | <code>tests/occ/utils/utils.go:105</code> | <code>func Funds(amount int64) sdk.Coins</code> |
| <code>interface.16.go-occ-utils-joinmsgs-j59</code> | <code>go_exported</code> | <code>stable</code> | <code>tests/occ/utils/utils.go:349</code> | <code>func JoinMsgs(msgsList ...[]*TestMessage) []*TestMessage</code> |
| <code>interface.16.go-occ-utils-newsigner-j59</code> | <code>go_exported</code> | <code>stable</code> | <code>tests/occ/utils/utils.go:84</code> | <code>func NewSigner() TestAcct</code> |
| <code>interface.16.go-occ-utils-newtestaccounts-j59</code> | <code>go_exported</code> | <code>stable</code> | <code>tests/occ/utils/utils.go:76</code> | <code>func NewTestAccounts(count int) []TestAcct</code> |
| <code>interface.16.go-occ-utils-newtestcontext-j59</code> | <code>go_exported</code> | <code>change</code> | <code>tests/occ/utils/utils.go:159</code> | <code>func NewTestContext(tb testing.TB, testAccts []TestAcct, blockTime time.Time, workers int, occEnabled bool) *TestContext</code> |
| <code>interface.16.go-occ-utils-processblockdirect-j59</code> | <code>go_exported</code> | <code>stable</code> | <code>tests/occ/utils/utils.go:343</code> | <code>func ProcessBlockDirect(testCtx *TestContext, txs [][]byte, occ bool) ([]types.Event, []*types.ExecTxResult, types.ResponseEndBlock, error)</code> |
| <code>interface.16.go-occ-utils-runwithocc-j59</code> | <code>go_exported</code> | <code>stable</code> | <code>tests/occ/utils/utils.go:324</code> | <code>func RunWithOCC(testCtx *TestContext, msgs []*TestMessage) ([]types.Event, []*types.ExecTxResult, types.ResponseEndBlock, error)</code> |
| <code>interface.16.go-occ-utils-runwithoutocc-j59</code> | <code>go_exported</code> | <code>stable</code> | <code>tests/occ/utils/utils.go:329</code> | <code>func RunWithoutOCC(testCtx *TestContext, msgs []*TestMessage) ([]types.Event, []*types.ExecTxResult, types.ResponseEndBlock, error)</code> |
| <code>interface.16.go-occ-utils-shuffle-j59</code> | <code>go_exported</code> | <code>stable</code> | <code>tests/occ/utils/utils.go:361</code> | <code>func Shuffle(msgs []*TestMessage) []*TestMessage</code> |
| <code>interface.16.go-occ-utils-testacct-j59</code> | <code>go_exported</code> | <code>stable</code> | <code>tests/occ/utils/utils.go:66</code> | <code>type TestAcct struct</code> |
| <code>interface.16.go-occ-utils-testcontext-j59</code> | <code>go_exported</code> | <code>stable</code> | <code>tests/occ/utils/utils.go:55</code> | <code>type TestContext struct</code> |
| <code>interface.16.go-occ-utils-testmessage-j59</code> | <code>go_exported</code> | <code>stable</code> | <code>tests/occ/utils/utils.go:48</code> | <code>type TestMessage struct</code> |
| <code>interface.16.go-occ-utils-totxbytes-j59</code> | <code>go_exported</code> | <code>stable</code> | <code>tests/occ/utils/utils.go:214</code> | <code>func ToTxBytes(testCtx *TestContext, msgs []*TestMessage) [][]byte</code> |
| <code>interface.16.go-testutil-fuzzing-fuzzcoin-j59</code> | <code>go_exported</code> | <code>retire</code> | <code>tests/testutil/fuzzing/cosmos.go:14</code> | <code>func FuzzCoin(denom string, isNil bool, i int64) sdk.Coin</code> |
| <code>interface.16.go-testutil-fuzzing-fuzzdec-j59</code> | <code>go_exported</code> | <code>retire</code> | <code>tests/testutil/fuzzing/cosmos.go:7</code> | <code>func FuzzDec(i int64, isNil bool) sdk.Dec</code> |
| <code>interface.16.go-testutil-keeper-epochkeeper-j59</code> | <code>go_exported</code> | <code>stable</code> | <code>tests/testutil/keeper/epoch.go:24</code> | <code>func EpochKeeper(t testing.TB) (*keeper.Keeper, sdk.Context)</code> |
| <code>interface.16.go-testutil-keeper-evmtestapp-j59</code> | <code>go_exported</code> | <code>stable</code> | <code>tests/testutil/keeper/evm.go:25</code> | <code>var EVMTestApp = v6120Applied(app.SetupWithDefaultHome(false, true, false))</code> |
| <code>interface.16.go-testutil-keeper-mockaddresspair-j59</code> | <code>go_exported</code> | <code>stable</code> | <code>tests/testutil/keeper/evm.go:111</code> | <code>func MockAddressPair() (sdk.AccAddress, common.Address)</code> |
| <code>interface.16.go-testutil-keeper-mockevmkeeper-j59</code> | <code>go_exported</code> | <code>stable</code> | <code>tests/testutil/keeper/evm.go:67</code> | <code>func MockEVMKeeper(t *testing.T) (*evmkeeper.Keeper, sdk.Context)</code> |
| <code>interface.16.go-testutil-keeper-mockevmkeeperprecompiles-j59</code> | <code>go_exported</code> | <code>stable</code> | <code>tests/testutil/keeper/evm.go:89</code> | <code>func MockEVMKeeperPrecompiles(t *testing.T) (*evmkeeper.Keeper, sdk.Context)</code> |
| <code>interface.16.go-testutil-keeper-mockevmkeeperwithprecompiles-j59</code> | <code>go_exported</code> | <code>stable</code> | <code>tests/testutil/keeper/evm.go:39</code> | <code>func MockEVMKeeperWithPrecompiles() (*evmkeeper.Keeper, sdk.Context)</code> |
| <code>interface.16.go-testutil-keeper-mockprivatekey-j59</code> | <code>go_exported</code> | <code>stable</code> | <code>tests/testutil/keeper/evm.go:115</code> | <code>func MockPrivateKey() cryptotypes.PrivKey</code> |
| <code>interface.16.go-testutil-keeper-mustmockreceipt-j59</code> | <code>go_exported</code> | <code>stable</code> | <code>tests/testutil/keeper/evm.go:166</code> | <code>func MustMockReceipt(t *testing.T, k *evmkeeper.Keeper, ctx sdk.Context, txHash common.Hash, receipt *evmtypes.Receipt)</code> |
| <code>interface.16.go-testutil-keeper-privatekeytoaddresses-j59</code> | <code>go_exported</code> | <code>stable</code> | <code>tests/testutil/keeper/evm.go:124</code> | <code>func PrivateKeyToAddresses(privKey cryptotypes.PrivKey) (sdk.AccAddress, common.Address)</code> |
| <code>interface.16.go-testutil-keeper-testapp-j59</code> | <code>go_exported</code> | <code>stable</code> | <code>tests/testutil/keeper/epoch.go:20</code> | <code>func TestApp(t *testing.T) *app.App</code> |
| <code>interface.16.go-testutil-keeper-uhpxcoins-j59</code> | <code>go_exported</code> | <code>stable</code> | <code>tests/testutil/keeper/evm.go:140</code> | <code>func UhpxCoins(amount int64) sdk.Coins</code> |
| <code>interface.16.go-testutil-keeper-waitforreceipt-j59</code> | <code>go_exported</code> | <code>stable</code> | <code>tests/testutil/keeper/evm.go:144</code> | <code>func WaitForReceipt(t *testing.T, k *evmkeeper.Keeper, ctx sdk.Context, txHash common.Hash) *evmtypes.Receipt</code> |
| <code>interface.16.go-testutil-keeper-waitforreceiptfromstore-j59</code> | <code>go_exported</code> | <code>stable</code> | <code>tests/testutil/keeper/evm.go:155</code> | <code>func WaitForReceiptFromStore(t *testing.T, k *evmkeeper.Keeper, ctx sdk.Context, txHash common.Hash) *evmtypes.Receipt</code> |
| <code>interface.16.go-testutil-network-defaultconfig-j59</code> | <code>go_exported</code> | <code>retire</code> | <code>tests/testutil/network/network.go:62</code> | <code>func DefaultConfig() network.Config</code> |
| <code>interface.16.go-testutil-network-new-j59</code> | <code>go_exported</code> | <code>retire</code> | <code>tests/testutil/network/network.go:45</code> | <code>func New(t *testing.T, configs ...network.Config) *network.Network</code> |
| <code>interface.16.go-testutil-network-testappoptions-get-j59</code> | <code>go_exported</code> | <code>retire</code> | <code>tests/testutil/network/network.go:30</code> | <code>func (t TestAppOptions) Get(s string) any</code> |
| <code>interface.16.go-testutil-network-testappoptions-j59</code> | <code>go_exported</code> | <code>retire</code> | <code>tests/testutil/network/network.go:28</code> | <code>type TestAppOptions struct{}</code> |
| <code>interface.16.go-testutil-nullify-fill-j59</code> | <code>go_exported</code> | <code>stable</code> | <code>tests/testutil/nullify/nullify.go:19</code> | <code>func Fill(x interface{}) interface{}</code> |
| <code>interface.16.go-testutil-processblock-app-ctx-j59</code> | <code>go_exported</code> | <code>stable</code> | <code>tests/testutil/processblock/common.go:65</code> | <code>func (a *App) Ctx() sdk.Context</code> |
| <code>interface.16.go-testutil-processblock-app-fastepoch-j59</code> | <code>go_exported</code> | <code>stable</code> | <code>tests/testutil/processblock/genesisepoch.go:5</code> | <code>func (a *App) FastEpoch()</code> |
| <code>interface.16.go-testutil-processblock-app-fundaccount-j59</code> | <code>go_exported</code> | <code>stable</code> | <code>tests/testutil/processblock/genesisbank.go:9</code> | <code>func (a *App) FundAccount(acc sdk.AccAddress, amount int64)</code> |
| <code>interface.16.go-testutil-processblock-app-getallvalidators-j59</code> | <code>go_exported</code> | <code>stable</code> | <code>tests/testutil/processblock/common.go:123</code> | <code>func (a *App) GetAllValidators() []stakingtypes.Validator</code> |
| <code>interface.16.go-testutil-processblock-app-j59</code> | <code>go_exported</code> | <code>stable</code> | <code>tests/testutil/processblock/common.go:26</code> | <code>type App struct</code> |
| <code>interface.16.go-testutil-processblock-app-newminter-j59</code> | <code>go_exported</code> | <code>stable</code> | <code>tests/testutil/processblock/genesismint.go:9</code> | <code>func (a *App) NewMinter(amount uint64)</code> |
| <code>interface.16.go-testutil-processblock-app-runblock-j59</code> | <code>go_exported</code> | <code>stable</code> | <code>tests/testutil/processblock/common.go:72</code> | <code>func (a *App) RunBlock(txs []signing.Tx) (resultCodes []uint32)</code> |
| <code>interface.16.go-testutil-processblock-app-sign-j59</code> | <code>go_exported</code> | <code>stable</code> | <code>tests/testutil/processblock/tx.go:18</code> | <code>func (a *App) Sign(account sdk.AccAddress, fee int64, msgs ...sdk.Msg) xauthsigning.Tx</code> |
| <code>interface.16.go-testutil-processblock-commonpreset-j59</code> | <code>go_exported</code> | <code>stable</code> | <code>tests/testutil/processblock/presets.go:24</code> | <code>func CommonPreset(app *App) *Preset</code> |
| <code>interface.16.go-testutil-processblock-getkey-j59</code> | <code>go_exported</code> | <code>stable</code> | <code>tests/testutil/processblock/tx.go:82</code> | <code>func GetKey(mnemonic string) cryptotypes.PrivKey</code> |
| <code>interface.16.go-testutil-processblock-msgs-feedenompreference-j59</code> | <code>go_exported</code> | <code>stable</code> | <code>tests/testutil/processblock/msgs/feetoken.go:79</code> | <code>func FeeDenomPreference(app *processblock.App, payer FeeTokenSigner, nonce uint64, gasPrice *big.Int,</code> |
| <code>interface.16.go-testutil-processblock-msgs-feetokensigner-j59</code> | <code>go_exported</code> | <code>stable</code> | <code>tests/testutil/processblock/msgs/feetoken.go:34</code> | <code>type FeeTokenSigner struct</code> |
| <code>interface.16.go-testutil-processblock-msgs-feetokensignerfromseed-j59</code> | <code>go_exported</code> | <code>stable</code> | <code>tests/testutil/processblock/msgs/feetoken.go:42</code> | <code>func FeeTokenSignerFromSeed(seed byte) FeeTokenSigner</code> |
| <code>interface.16.go-testutil-processblock-msgs-feetokentransfer-j59</code> | <code>go_exported</code> | <code>stable</code> | <code>tests/testutil/processblock/msgs/feetoken.go:91</code> | <code>func FeeTokenTransfer(app *processblock.App, sender FeeTokenSigner, nonce uint64, gasPrice *big.Int,</code> |
| <code>interface.16.go-testutil-processblock-msgs-recipientword-j59</code> | <code>go_exported</code> | <code>stable</code> | <code>tests/testutil/processblock/msgs/feetoken.go:59</code> | <code>func RecipientWord(recipient common.Address) [32]byte</code> |
| <code>interface.16.go-testutil-processblock-msgs-send-j59</code> | <code>go_exported</code> | <code>stable</code> | <code>tests/testutil/processblock/msgs/bank.go:8</code> | <code>func Send(from sdk.AccAddress, to sdk.AccAddress, amount int64) *banktypes.MsgSend</code> |
| <code>interface.16.go-testutil-processblock-msgs-sidiorabridgein-j59</code> | <code>go_exported</code> | <code>stable</code> | <code>tests/testutil/processblock/msgs/feetoken.go:65</code> | <code>func SidioraBridgeIn(app *processblock.App, relayer FeeTokenSigner, nonce uint64, gasPrice *big.Int, chain uint64,</code> |
| <code>interface.16.go-testutil-processblock-newtestapp-j59</code> | <code>go_exported</code> | <code>stable</code> | <code>tests/testutil/processblock/common.go:36</code> | <code>func NewTestApp(t *testing.T) *App</code> |
| <code>interface.16.go-testutil-processblock-newtestappwithgenesis-j59</code> | <code>go_exported</code> | <code>new</code> | <code>tests/testutil/processblock/common.go</code> (planned) | <code>func NewTestAppWithGenesis(t *testing.T, override map[string]json.RawMessage) *App</code> |
| <code>interface.16.go-testutil-processblock-preset-adminsign-j59</code> | <code>go_exported</code> | <code>stable</code> | <code>tests/testutil/processblock/presets.go:53</code> | <code>func (p *Preset) AdminSign(app *App, msgs ...sdk.Msg) signing.Tx</code> |
| <code>interface.16.go-testutil-processblock-txconfig-j59</code> | <code>go_exported</code> | <code>stable</code> | <code>tests/testutil/processblock/tx.go:16</code> | <code>var TxConfig = tx.NewTxConfig(Marshaler, tx.DefaultSignModes)</code> |
| <code>interface.16.go-testutil-processblock-verify-allocation-j59</code> | <code>go_exported</code> | <code>stable</code> | <code>tests/testutil/processblock/verify/distribution.go:18</code> | <code>func Allocation(t *testing.T, app *processblock.App, f BlockRunnable, _ []signing.Tx) BlockRunnable</code> |
| <code>interface.16.go-testutil-processblock-verify-balance-j59</code> | <code>go_exported</code> | <code>stable</code> | <code>tests/testutil/processblock/verify/bank.go:15</code> | <code>func Balance(t *testing.T, app *processblock.App, f BlockRunnable, txs []signing.Tx) BlockRunnable</code> |
| <code>interface.16.go-testutil-processblock-verify-epoch-j59</code> | <code>go_exported</code> | <code>stable</code> | <code>tests/testutil/processblock/verify/epoch.go:11</code> | <code>func Epoch(t *testing.T, app *processblock.App, f BlockRunnable, _ []signing.Tx) BlockRunnable</code> |
| <code>interface.16.go-testutil-processblock-verify-feetokenbalances-j59</code> | <code>go_exported</code> | <code>stable</code> | <code>tests/testutil/processblock/verify/feetoken.go:44</code> | <code>func FeeTokenBalances(expectation FeeTokenExpectation) Verifier</code> |
| <code>interface.16.go-testutil-processblock-verify-feetokenexpectation-j59</code> | <code>go_exported</code> | <code>stable</code> | <code>tests/testutil/processblock/verify/feetoken.go:20</code> | <code>type FeeTokenExpectation struct</code> |
| <code>interface.16.go-testutil-processblock-verify-mintrelease-j59</code> | <code>go_exported</code> | <code>stable</code> | <code>tests/testutil/processblock/verify/mint.go:13</code> | <code>func MintRelease(t *testing.T, app *processblock.App, f BlockRunnable, _ []signing.Tx) BlockRunnable</code> |
| <code>interface.16.go-testutil-processblock-verify-verifier-j59</code> | <code>go_exported</code> | <code>stable</code> | <code>tests/testutil/processblock/verify/common.go:12</code> | <code>type Verifier func(*testing.T, *processblock.App, BlockRunnable, []signing.Tx) BlockRunnable</code> |
| <code>interface.16.go-testutil-sample-accaddress-j59</code> | <code>go_exported</code> | <code>stable</code> | <code>tests/testutil/sample/sample.go:9</code> | <code>func AccAddress() string</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.16.go-testutil-fuzzing-fuzzcoin-j59</code> | notes | Package has no importer at HEAD. |
| <code>interface.16.go-testutil-fuzzing-fuzzdec-j59</code> | notes | Package has no importer at HEAD. |
| <code>interface.16.go-testutil-network-defaultconfig-j59</code> | notes | Package has no importer at HEAD. |
| <code>interface.16.go-testutil-network-new-j59</code> | notes | Package has no importer at HEAD. |
| <code>interface.16.go-testutil-network-testappoptions-get-j59</code> | notes | Package has no importer at HEAD. |
| <code>interface.16.go-testutil-network-testappoptions-j59</code> | notes | Package has no importer at HEAD. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.16.243.1</code> | req | 16243.1 |
| <code>acceptance.16.243.1</code> | scenario | Given an override map whose "mint" entry is the default mint genesis with params.mint_denom set to "utest", when NewTestAppWithGenesis(t, override) returns, then MintKeeper.GetParams(app.Ctx()).MintDenom equals "utest" and the bank genesis is the default one |
| <code>acceptance.16.243.1</code> | test | tests/testutil/processblock/common_test.go::TestNewTestAppWithGenesisOverridesModuleSection (planned) |
| <code>acceptance.16.243.1</code> | qualification | UNRUN |
| <code>req.16243</code> | ac_1 | NewTestAppWithGenesis(t, override) (planned next to NewTestApp at tests/testutil/processblock/common.go:36) starts the app from app.NewDefaultGenesisState with each named module section replaced by the given raw JSON before InitChain, so a test can boot with non-default module genesis. |
| <code>acceptance.16.243.2</code> | req | 16243.2 |
| <code>acceptance.16.243.2</code> | scenario | Given an override map with key "nosuchmodule", when NewTestAppWithGenesis runs under a sub-test, then the sub-test fails with a message naming nosuchmodule |
| <code>acceptance.16.243.2</code> | test | tests/testutil/processblock/common_test.go::TestNewTestAppWithGenesisRejectsUnknownModule (planned) |
| <code>acceptance.16.243.2</code> | qualification | UNRUN |
| <code>req.16243</code> | ac_2 | NewTestAppWithGenesis fails the test through t.Fatalf when an override names a module that NewDefaultGenesisState does not contain, instead of booting with the key ignored. |
| <code>acceptance.16.244.1</code> | req | 16244.1 |
| <code>acceptance.16.244.1</code> | scenario | Given the repository after the deletion, when the retirement test checks both paths and the import strings tests/testutil/network and tests/testutil/fuzzing, then both files are absent and no .go file contains either import string |
| <code>acceptance.16.244.1</code> | test | tests/ci/test_retired_testutil_network.py::test_testutil_network_and_fuzzing_retired (planned) |
| <code>acceptance.16.244.1</code> | qualification | UNRUN |
| <code>req.16244</code> | ac_1 | tests/testutil/network/network.go and tests/testutil/fuzzing/cosmos.go are deleted because no package imports them at HEAD (tests/testutil/network/network.go:30 Get, tests/testutil/network/network.go:45 New, tests/testutil/fuzzing/cosmos.go:7 FuzzDec), and no Go file references their import paths. |
| <code>capability.16.22.1</code> | title | Genesis-override test app constructor |
| <code>capability.16.22.1</code> | reqs | 16243.1<br>16243.2 |
| <code>capability.16.22.2</code> | title | Unimported testutil packages retired |
| <code>capability.16.22.2</code> | reqs | 16244.1 |

## integration-runner

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.16.fmt-integration-yaml-suite-j59</code> | <code>file_format</code> | <code>stable</code> | <code>tests/integration/bank_module/send_funds_test.yaml:1</code> | <code>YAML list of cases: name, inputs (cmd, env, node), verifiers (type eval&#124;regex, expr, result)</code> |
| <code>interface.16.go-integration-runner-evalexpr-j59</code> | <code>go_exported</code> | <code>change</code> | <code>tests/integration/runner/runner.go:208</code> | <code>func evalExpr(expr string, envMap map[string]string) (bool, error)</code> |
| <code>interface.16.go-integration-runner-excludedsuites-j59</code> | <code>go_exported</code> | <code>new</code> | <code>tests/integration/runner/suites.go</code> (planned) | <code>var ExcludedSuites = map[string]string{...}</code> |
| <code>interface.16.go-integration-runner-runfile-j59</code> | <code>go_exported</code> | <code>stable</code> | <code>tests/integration/runner/runner.go:95</code> | <code>func RunFile(t *testing.T, path string, opts ...Option)</code> |
| <code>interface.16.go-integration-runner-suites-j59</code> | <code>go_exported</code> | <code>new</code> | <code>tests/integration/runner/suites.go</code> (planned) | <code>var Suites = []Suite{...}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.16.go-integration-runner-evalexpr-j59</code> | notes | Unexported but it is the eval verifier semantics every YAML suite relies on; the or-branch drops evaluation errors. |

Contract context <code>contract_context.16.integration-runner</code>:

| Field | Frozen rule |
|---|---|
| interface | tests/integration/runner RunFile and the Suites and ExcludedSuites tables (planned) |
| storage | YAML case files under tests/integration/&lt;module&gt; |
| errors | an eval verifier error is returned, never treated as false |
| events | one Go sub-test per YAML case |
| gas | none |
| activation | waves 1 and 2 |
| invariants | every module YAML is in exactly one of Suites or ExcludedSuites |
| pause | none |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.16.259.1</code> | req | 16259.1 |
| <code>acceptance.16.259.1</code> | scenario | Given envMap {"A": "1"}, when evalExpr("A == 2 or MISSING", envMap) runs, then it returns false and an error containing no operator in "MISSING" |
| <code>acceptance.16.259.1</code> | test | tests/integration/runner/eval_test.go::TestEvalExprOrPropagatesEvaluationError (planned) |
| <code>acceptance.16.259.1</code> | qualification | UNRUN |
| <code>req.16259</code> | ac_1 | evalExpr returns the first evaluation error when no branch of an or-expression evaluates to true, where at HEAD tests/integration/runner/runner.go:224 discards it and returns false with a nil error. |
| <code>acceptance.16.260.1</code> | req | 16260.1 |
| <code>acceptance.16.260.1</code> | scenario | Given the 27 YAML files under tests/integration, when TestSuitesCoverEveryModuleYAML walks them, then each path is in exactly one of Suites or ExcludedSuites and every listed path exists |
| <code>acceptance.16.260.1</code> | test | tests/integration/runner/suites_test.go::TestSuitesCoverEveryModuleYAML (planned) |
| <code>acceptance.16.260.1</code> | qualification | UNRUN |
| <code>req.16260</code> | ac_1 | runner.Suites (planned in tests/integration/runner/suites.go) lists each Go test name with its YAML files in CI order, runner_test.go iterates it, and runner.ExcludedSuites names every YAML deliberately run elsewhere with a reason; at HEAD tests/integration/runner/runner_test.go:26 hard-codes each file and tests/integration/upgrade_module/major_upgrade_test.yaml is run only by .github/workflows/paxeer-integration-test.yml:336. |
| <code>capability.16.29.1</code> | title | YAML runner eval error propagation |
| <code>capability.16.29.1</code> | reqs | 16259.1 |
| <code>capability.16.29.2</code> | title | YAML suite registry |
| <code>capability.16.29.2</code> | reqs | 16260.1 |

## occ-harness

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.16.go-occ-utils-cw20instantiatemsg-j59</code> | <code>go_exported</code> | <code>new</code> | <code>tests/occ/utils/utils.go</code> (planned) | <code>func CW20InstantiateMsg(tCtx *TestContext) []byte</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.16.go-occ-utils-cw20instantiatemsg-j59</code> | notes | Planned in task 16.38.1; extracted from deployCW20Token. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.16.301.1</code> | req | 16301.1 |
| <code>acceptance.16.301.1</code> | scenario | Given the node capability list iterator,staking,stargate,pax at node/app.go:772, when NewTestContext(t, NewTestAccounts(2), now, 4, true) runs, then it returns without panic, CodeID and CW20CodeID are non-zero and WasmKeeper.GetCodeInfo returns a code info for each |
| <code>acceptance.16.301.1</code> | test | tests/occ/fixture_test.go::TestNewTestContextStoresFixturesUnderNodeCapabilities (planned) |
| <code>acceptance.16.301.1</code> | qualification | UNRUN |
| <code>req.16301</code> | ac_1 | NewTestContext at tests/occ/utils/utils.go:159 stores no wasm whose exports require a capability outside the node list at node/app.go:772: it drops the mars.wasm upload at tests/occ/utils/utils.go:160 (which exports requires_sei) and sets CodeID to the cw20_base.wasm code stored at tests/occ/utils/utils.go:185, so both CodeID and CW20CodeID resolve through WasmKeeper.GetCodeInfo. |
| <code>acceptance.16.301.2</code> | req | 16301.2 |
| <code>acceptance.16.301.2</code> | scenario | Given the harness after the change, when TestParallelTransactionsWasmInstantiate runs ten instantiations three times with and without OCC, then every ExecTxResult code is 0 and assertEqualState and assertEqualEvents find no difference |
| <code>acceptance.16.301.2</code> | test | tests/occ/occ_test.go::TestParallelTransactionsWasmInstantiate (existing) |
| <code>acceptance.16.301.2</code> | qualification | UNRUN |
| <code>req.16301</code> | ac_2 | WasmInstantiate at tests/occ/messages/test_msgs.go:44 instantiates the cw20_base code with a cw20 instantiate message instead of the mars instantiateMsg at tests/occ/messages/test_msgs.go:21, and TestParallelTransactionsWasmInstantiate at tests/occ/occ_test.go:108 passes with every result code 0 and equal OCC and sequential state. |
| <code>capability.16.38.1</code> | title | OCC harness fixtures within the node wasm capability list |
| <code>capability.16.38.1</code> | reqs | 16301.1<br>16301.2 |
| <code>resolution.17.990</code> | title | decision.1628 binds: OCC harness fixtures follow the node capability list |
| <code>resolution.17.990</code> | applies_to | decision.1628<br>task.16.38.1 |
| <code>resolution.17.990</code> | source | decision.1628 in fragments/J59/decision.kvx; tasks from affects |

## reachability

No interface block names this anchor; the citing criteria and rules below define its scope.

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.16.297.1</code> | req | 16297.1 |
| <code>acceptance.16.297.1</code> | scenario | Given the repository after the wave-6 retirements, when the guard walks tests/ excluding the tests/arith, tests/codec, tests/crypto and tests/fuzz kernel suites, then every C file with int main has a build reference and the test passes |
| <code>acceptance.16.297.1</code> | test | tools/repotest/reachability_test.go::TestEveryCTestMainHasBuildReference (planned) |
| <code>acceptance.16.297.1</code> | qualification | UNRUN |
| <code>req.16297</code> | ac_1 | TestEveryCTestMainHasBuildReference (planned) fails naming every tests/**/*.c file that defines int main and whose path appears in none of Makefile, chain.mk or a file under tools/ or scripts/; at HEAD tests/test_qual_replay.c:20 is built only by tools/lxp_qual_replay_matrix.sh:1 and passes. |
| <code>acceptance.16.297.2</code> | req | 16297.2 |
| <code>acceptance.16.297.2</code> | scenario | Given the repository, when the guard walks tests/ for *.rs, then each file is referenced from a crate and the test passes |
| <code>acceptance.16.297.2</code> | test | tools/repotest/reachability_test.go::TestEveryRustTestFileIsCompiled (planned) |
| <code>acceptance.16.297.2</code> | qualification | UNRUN |
| <code>req.16297</code> | ac_2 | TestEveryRustTestFileIsCompiled (planned) fails naming every tests/**/*.rs file that no #[path = ...] attribute, build.rs string or Cargo.toml path references; at HEAD tests/support/lxgb_metadata.rs is reached by services/platform/hosted/registry/tests/native_deployment.rs:1. |
| <code>acceptance.16.298.1</code> | req | 16298.1 |
| <code>acceptance.16.298.1</code> | scenario | Given the Makefile at the wave-6 revision, when the guard checks every go test line with -run, then every expression matches at least one Test function in its package directory and the test passes |
| <code>acceptance.16.298.1</code> | test | tools/repotest/selectors_test.go::TestMakefileRunSelectorsMatchTests (planned) |
| <code>acceptance.16.298.1</code> | qualification | UNRUN |
| <code>req.16298</code> | ac_1 | TestMakefileRunSelectorsMatchTests (planned) parses each go test command in Makefile and chain.mk, compiles its -run expression and fails naming the command when no func Test in the named package matches, closing the gap where a -run that matches nothing exits 0 with no tests to run. |
| <code>acceptance.16.299.1</code> | req | 16299.1 |
| <code>acceptance.16.299.1</code> | scenario | Given the repository after the requested test-tests-python target lands, when the guard walks tests/ for test_*.py outside tests/bridge and tests/explorer*, then every file is referenced and the test passes |
| <code>acceptance.16.299.1</code> | test | tools/repotest/pytest_reachability_test.go::TestEveryPytestSuiteIsReferenced (planned) |
| <code>acceptance.16.299.1</code> | qualification | UNRUN |
| <code>req.16299</code> | ac_1 | TestEveryPytestSuiteIsReferenced (planned) fails naming every tests/**/test_*.py file whose path no Makefile, chain.mk, .github/workflows file or script under tools/ and scripts/ names; at HEAD tests/ci/test_real_node_build_ownership.py:1, tests/ci/test_runner_routing_container_runtime.py:1 and tests/test_lxgb_producers.py:1 are named nowhere and gain a Makefile target through wiring. |
| <code>acceptance.16.300.1</code> | req | 16300.1 |
| <code>acceptance.16.300.1</code> | scenario | Given the repository, when the test walks every go.mod outside vendor and node_modules directories, then each require path and each go.sum module passes isBannedModule and the test names any offending file and module |
| <code>acceptance.16.300.1</code> | test | tools/repotest/mod_test.go::TestEveryGoModHasNoBannedDependencies (planned) |
| <code>acceptance.16.300.1</code> | qualification | UNRUN |
| <code>req.16300</code> | ac_1 | TestEveryGoModHasNoBannedDependencies (planned in tools/repotest/mod_test.go) applies isBannedModule (tools/repotest/mod_test.go:35) to every go.mod and go.sum found under the repository, where at HEAD tools/repotest/mod_test.go:57 and tools/repotest/mod_test.go:73 read only the root go.mod and go.sum and skip tools/loadtest/go.mod. |
| <code>capability.16.28.1</code> | title | C and Rust suite reachability guard |
| <code>capability.16.28.1</code> | reqs | 16297.1<br>16297.2 |
| <code>capability.16.28.2</code> | title | Makefile go test selector guard |
| <code>capability.16.28.2</code> | reqs | 16298.1 |
| <code>capability.16.28.3</code> | title | pytest suite reachability guard |
| <code>capability.16.28.3</code> | reqs | 16299.1 |
| <code>capability.16.28.4</code> | title | Banned-module check for every module |
| <code>capability.16.28.4</code> | reqs | 16300.1 |

## retirement

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.16.c-lxp-real-replay-activity-j59</code> | <code>c_symbol</code> | <code>retire</code> | <code>tests/support/lxp_real_replay.h:328</code> | <code>static inline int lxp_real_replay_activity(lxp_real_replay_fixture *f, ...)</code> |
| <code>interface.16.c-lxp-real-replay-build-j59</code> | <code>c_symbol</code> | <code>retire</code> | <code>tests/support/lxp_real_replay.h:359</code> | <code>static inline int lxp_real_replay_build(lxp_real_replay_fixture *f, ...)</code> |
| <code>interface.16.c-lxp-real-replay-init-j59</code> | <code>c_symbol</code> | <code>retire</code> | <code>tests/support/lxp_real_replay.h:305</code> | <code>static inline int lxp_real_replay_init(lxp_real_replay_fixture *f)</code> |
| <code>interface.16.c-lxp-real-replay-prepare-j59</code> | <code>c_symbol</code> | <code>retire</code> | <code>tests/support/lxp_real_replay.h:67</code> | <code>static inline int lxp_real_replay_prepare(lxp_real_replay_fixture *f, uint16_t version, bool success)</code> |
| <code>interface.16.c-pay1-guarantor-replay-j59</code> | <code>c_symbol</code> | <code>retire</code> | <code>tests/support/lxp_pay1_replay.h:112</code> | <code>static int pay1_guarantor_replay(void)</code> |
| <code>interface.16.c-pay1-replay-activities-j59</code> | <code>c_symbol</code> | <code>retire</code> | <code>tests/support/lxp_pay1_replay.h:8</code> | <code>static int pay1_replay_activities(lxp_real_replay_fixture *f, lxp_arena *arena, ...)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.16.c-lxp-real-replay-init-j59</code> | notes | Binds lxp_replay_engine_init, retired by the kernel replay retirement. |
| <code>interface.16.c-pay1-guarantor-replay-j59</code> | notes | Calls lxp_guarantor_recompute_roots, retired by the guarantor retirement. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.16.241.1</code> | req | 16241.1 |
| <code>acceptance.16.241.1</code> | scenario | Given a temporary tree holding tests/a.c and no tests/b.c, when assert_paths_absent(['tests/a.c', 'tests/b.c'], root=tree) runs, then it raises AssertionError whose message names tests/a.c and not tests/b.c |
| <code>acceptance.16.241.1</code> | test | tests/ci/test_retired_tests.py::test_assert_paths_absent_names_present_file (planned) |
| <code>acceptance.16.241.1</code> | qualification | UNRUN |
| <code>req.16241</code> | ac_1 | tests/ci/retired_tests.py (planned) assert_paths_absent raises AssertionError naming every listed path that still exists under the given root. |
| <code>acceptance.16.241.2</code> | req | 16241.2 |
| <code>acceptance.16.241.2</code> | scenario | Given a temporary tree whose tests/x.c line 3 calls lxp_da_store_init and whose tests/y.c mentions lxp_da_store_init_v2 only, when assert_symbols_unreferenced(['lxp_da_store_init'], ['tests'], root=tree) runs, then it raises AssertionError naming tests/x.c (planned fixture line 3) and not tests/y.c |
| <code>acceptance.16.241.2</code> | test | tests/ci/test_retired_tests.py::test_assert_symbols_unreferenced_names_live_caller (planned) |
| <code>acceptance.16.241.2</code> | qualification | UNRUN |
| <code>req.16241</code> | ac_2 | tests/ci/retired_tests.py (planned) assert_symbols_unreferenced raises AssertionError naming path:line for every word-boundary use of a listed symbol in .c, .h, .py, .sh, .sol and .go files under the given roots. |
| <code>acceptance.16.241.3</code> | req | 16241.3 |
| <code>acceptance.16.241.3</code> | scenario | Given a temporary tree whose Makefile target build/tests/fails compiles a C program returning 3, when build_and_run_c_suite('build/tests/fails', root=tree) runs, then it raises AssertionError whose message contains exit code 3 |
| <code>acceptance.16.241.3</code> | test | tests/ci/test_retired_tests.py::test_build_and_run_c_suite_reports_exit_code (planned) |
| <code>acceptance.16.241.3</code> | qualification | UNRUN |
| <code>req.16241</code> | ac_3 | tests/ci/retired_tests.py (planned) build_and_run_c_suite runs make -C &lt;root&gt; &lt;target&gt; and then the built binary, raising AssertionError with the exit code and stderr tail when either step exits non-zero. |
| <code>acceptance.16.261.1</code> | req | 16261.1 |
| <code>acceptance.16.261.1</code> | scenario | Given the repository after the deletion, when the retirement test checks the ten paths and the import strings of the retired contracts, then no path exists and no .sol or .py file under tests/ references CheckpointRegistry.t.sol, Ed25519Signer.sol or ContractIntegration.t.sol |
| <code>acceptance.16.261.1</code> | test | tests/ci/test_retired_solidity_checkpoint.py::test_checkpoint_and_library_suites_retired (planned) |
| <code>acceptance.16.261.1</code> | qualification | UNRUN |
| <code>req.16261</code> | ac_1 | The ten files tests/solidity/CheckpointChallengeManager.t.sol, tests/solidity/CheckpointRegistry.t.sol, tests/solidity/GuarantorBond.t.sol, tests/solidity/ContractIntegration.t.sol, tests/solidity/Ed25519.t.sol, tests/solidity/Ed25519Signer.sol, tests/solidity/NativeStateProof.t.sol, tests/solidity/MerkleDomainVector.t.sol, tests/solidity/ContractPrimitives.t.sol, tests/solidity/ContractExecutionSafety.t.sol are deleted because every contracts/ source they import retires in the chain-modules domain (tests/solidity/CheckpointChallengeManager.t.sol:4, tests/solidity/CheckpointChallengeManager.t.sol:5, tests/solidity/CheckpointRegistry.t.sol:4, tests/solidity/CheckpointRegistry.t.sol:5, tests/solidity/GuarantorBond.t.sol:4, tests/solidity/GuarantorBond.t.sol:5, tests/solidity/ContractIntegration.t.sol:4, tests/solidity/ContractIntegration.t.sol:7), and no file under tests/ or scripts/ imports a deleted file. |
| <code>acceptance.16.262.1</code> | req | 16262.1 |
| <code>acceptance.16.262.1</code> | scenario | Given the repository after the deletion, when the retirement test checks the eight paths, then none exists and tests/solidity holds no .sol file |
| <code>acceptance.16.262.1</code> | test | tests/ci/test_retired_solidity_deployment.py::test_deployment_and_governance_suites_retired (planned) |
| <code>acceptance.16.262.1</code> | qualification | UNRUN |
| <code>req.16262</code> | ac_1 | The eight files tests/solidity/ContractDeployment.t.sol, tests/solidity/ContractManager.t.sol, tests/solidity/ContractInvariants.t.sol, tests/solidity/DepositAuthorityTimelock.t.sol, tests/solidity/LayerXTimelock.t.sol, tests/solidity/PaxeerBetaDeploy.t.sol, tests/solidity/PaxeerBetaDeploymentValidator.t.sol, tests/solidity/SidioraProxyGovernance.t.sol are deleted because the deployment, manager, timelock and proxy-governance contracts they import retire (tests/solidity/ContractDeployment.t.sol:4, tests/solidity/ContractDeployment.t.sol:5, tests/solidity/ContractManager.t.sol:4, tests/solidity/ContractManager.t.sol:5, tests/solidity/ContractInvariants.t.sol:4, tests/solidity/ContractInvariants.t.sol:5, tests/solidity/DepositAuthorityTimelock.t.sol:4, tests/solidity/DepositAuthorityTimelock.t.sol:5), leaving tests/solidity empty. |
| <code>acceptance.16.263.1</code> | req | 16263.1 |
| <code>acceptance.16.263.1</code> | scenario | Given the repository after the change, when test_gateway_receive_retired runs, then 1 deleted file is absent, no file under tests/ outside the tests/daemon, tests/bridge, tests/chain, tests/explorer and kernel suite trees names lxp_gateway_receive_claim, lxp_gateway_grant_present_test_locked |
| <code>acceptance.16.263.1</code> | test | tests/ci/test_retired_gateway_receive.py::test_gateway_receive_retired (planned) |
| <code>acceptance.16.263.1</code> | qualification | UNRUN |
| <code>req.16263</code> | ac_1 | tests/test_gateway_receive.c is deleted because it exercises kernel names retired by the kernel domain (lxp_gateway_receive_claim at tests/test_gateway_receive.c:38); tests/ci/test_retired_gateway_receive.py asserts the deleted paths are absent and no .c, .h, .py, .sh, .sol or .go file under tests/ outside the tests/daemon, tests/bridge, tests/chain, tests/explorer and kernel suite trees names lxp_gateway_receive_claim, lxp_gateway_grant_present_test_locked. |
| <code>acceptance.16.264.1</code> | req | 16264.1 |
| <code>acceptance.16.264.1</code> | scenario | Given the repository after the change, when test_sequencer_retired runs, then 2 deleted files are absent, no file under tests/ outside the tests/daemon, tests/bridge, tests/chain, tests/explorer and kernel suite trees names lxp_sequencer_recovery_ops, lxp_sequencer_recovery_result, lxp_sequencer_recover, lxp_sequencer_equivocation_evidence, lxp_sequencer_header_registry, lxp_sequencer_header_registry_init |
| <code>acceptance.16.264.1</code> | test | tests/ci/test_retired_sequencer.py::test_sequencer_retired (planned) |
| <code>acceptance.16.264.1</code> | qualification | UNRUN |
| <code>req.16264</code> | ac_1 | tests/test_sequencer_recovery.c, tests/test_sequencer_sequence.c are deleted because they exercise kernel names retired by the kernel domain (lxp_sequencer_recovery_ops at tests/test_sequencer_recovery.c:98; lxp_seq_allocator at tests/test_sequencer_sequence.c:23); tests/ci/test_retired_sequencer.py asserts the deleted paths are absent and no .c, .h, .py, .sh, .sol or .go file under tests/ outside the tests/daemon, tests/bridge, tests/chain, tests/explorer and kernel suite trees names lxp_sequencer_recovery_ops, lxp_sequencer_recovery_result, lxp_sequencer_recover, lxp_sequencer_equivocation_evidence, lxp_sequencer_header_registry, lxp_sequencer_header_registry_init. |
| <code>acceptance.16.265.1</code> | req | 16265.1 |
| <code>acceptance.16.265.1</code> | scenario | Given the repository after the change, when test_batch_publication_retired runs, then 3 deleted files are absent, no file under tests/ outside the tests/daemon, tests/bridge, tests/chain, tests/explorer and kernel suite trees names lxp_batch_replica_target, lxp_batch_eligibility_state, lxp_batch_body_decode, lxp_batch_publish, lxp_batch_eligibility_init, lxp_batch_eligibility |
| <code>acceptance.16.265.1</code> | test | tests/ci/test_retired_batch_publication.py::test_batch_publication_retired (planned) |
| <code>acceptance.16.265.1</code> | qualification | UNRUN |
| <code>req.16265</code> | ac_1 | tests/test_batch_publish.c, tests/test_batch_time.c, tests/test_history_query.c are deleted because they exercise kernel names retired by the kernel domain (lxp_batch_body at tests/test_batch_publish.c:44; lxp_batch_timestamp_select at tests/test_batch_time.c:17; lxp_batch_body at tests/test_history_query.c:100); tests/ci/test_retired_batch_publication.py asserts the deleted paths are absent and no .c, .h, .py, .sh, .sol or .go file under tests/ outside the tests/daemon, tests/bridge, tests/chain, tests/explorer and kernel suite trees names lxp_batch_replica_target, lxp_batch_eligibility_state, lxp_batch_body_decode, lxp_batch_publish, lxp_batch_eligibility_init, lxp_batch_eligibility. |
| <code>acceptance.16.266.1</code> | req | 16266.1 |
| <code>acceptance.16.266.1</code> | scenario | Given the repository after the change, when test_replay_crossarch_retired runs, then 2 deleted files are absent, no file under tests/ outside the tests/daemon, tests/bridge, tests/chain, tests/explorer and kernel suite trees names lxp_replay_fixture, lxp_replay_fixture_load, lxp_replay_digest, lxp_replay_crossarch_case |
| <code>acceptance.16.266.1</code> | test | tests/ci/test_retired_replay_crossarch.py::test_replay_crossarch_retired (planned) |
| <code>acceptance.16.266.1</code> | qualification | UNRUN |
| <code>req.16266</code> | ac_1 | tests/test_replay_crossarch.c, tests/replica/lxp_replay_matrix.sh are deleted because they exercise kernel names retired by the kernel domain (lxp_replay_fixture at tests/test_replay_crossarch.c:3; lxp_replay_fixture at tests/replica/lxp_replay_matrix.sh:9); tests/ci/test_retired_replay_crossarch.py asserts the deleted paths are absent and no .c, .h, .py, .sh, .sol or .go file under tests/ outside the tests/daemon, tests/bridge, tests/chain, tests/explorer and kernel suite trees names lxp_replay_fixture, lxp_replay_fixture_load, lxp_replay_digest, lxp_replay_crossarch_case. |
| <code>acceptance.16.267.1</code> | req | 16267.1 |
| <code>acceptance.16.267.1</code> | scenario | Given the repository after the change, when test_replica_retired runs, then 3 deleted files are absent, no file under tests/ outside the tests/daemon, tests/bridge, tests/chain, tests/explorer and kernel suite trees names lxp_replica_init, lxp_replica_ingest_batch, lxp_replica_bind_execution, lxp_replica_bind_eligibility, lxp_replica_batch_eligible, lxp_divergence_component |
| <code>acceptance.16.267.1</code> | test | tests/ci/test_retired_replica.py::test_replica_retired (planned) |
| <code>acceptance.16.267.1</code> | qualification | UNRUN |
| <code>req.16267</code> | ac_1 | tests/test_replica_ingest.c, tests/test_replica_divergence.c, tests/test_replica_replay.c are deleted because they exercise kernel names retired by the kernel domain (lxp_replica at tests/test_replica_ingest.c:5; lxp_replica at tests/test_replica_divergence.c:1; lxp_batch_body at tests/test_replica_replay.c:17); tests/ci/test_retired_replica.py asserts the deleted paths are absent and no .c, .h, .py, .sh, .sol or .go file under tests/ outside the tests/daemon, tests/bridge, tests/chain, tests/explorer and kernel suite trees names lxp_replica_init, lxp_replica_ingest_batch, lxp_replica_bind_execution, lxp_replica_bind_eligibility, lxp_replica_batch_eligible, lxp_divergence_component. |
| <code>acceptance.16.268.1</code> | req | 16268.1 |
| <code>acceptance.16.268.1</code> | scenario | Given the repository after the change, when test_qual_faults_retired runs, then tests/qualification/lxp_qual_faults.c contains none of lxp_replica, lxp_replica_chain_link, lxp_replica_halt, lxp_sequencer_liveness, lxp_sequencer_loss, lxp_sequencer_can_seal, lxp_sequencer_handover_authorize, lxp_batch_range_check, tests/test_qual_recovery.c contains none of lxp_partition_sim, lxp_sequencer_loss_sim, make -C . build/tests/test_qual_recovery succeeds and each binary exits 0 |
| <code>acceptance.16.268.1</code> | test | tests/ci/test_retired_qual_faults.py::test_qual_faults_retired (planned) |
| <code>acceptance.16.268.1</code> | qualification | UNRUN |
| <code>req.16268</code> | ac_1 | tests/qualification/lxp_qual_faults.c drops every use of lxp_replica, lxp_replica_chain_link, lxp_replica_halt, lxp_sequencer_liveness, lxp_sequencer_loss, lxp_sequencer_can_seal, lxp_sequencer_handover_authorize, lxp_batch_range_check (first at tests/qualification/lxp_qual_faults.c:8, tests/qualification/lxp_qual_faults.c:457, tests/qualification/lxp_qual_faults.c:548, tests/qualification/lxp_qual_faults.c:562); tests/test_qual_recovery.c drops every use of lxp_partition_sim, lxp_sequencer_loss_sim (first at tests/test_qual_recovery.c:8, tests/test_qual_recovery.c:9); tests/ci/test_retired_qual_faults.py asserts tests/qualification/lxp_qual_faults.c no longer names lxp_replica, lxp_replica_chain_link, lxp_replica_halt, lxp_sequencer_liveness, lxp_sequencer_loss, lxp_sequencer_can_seal, lxp_sequencer_handover_authorize, lxp_batch_range_check; tests/test_qual_recovery.c no longer names lxp_partition_sim, lxp_sequencer_loss_sim, then builds and runs build/tests/test_qual_recovery with exit 0. |
| <code>acceptance.16.269.1</code> | req | 16269.1 |
| <code>acceptance.16.269.1</code> | scenario | Given the repository after the change, when test_batch_seal_retired runs, then 1 deleted file is absent |
| <code>acceptance.16.269.1</code> | test | tests/ci/test_retired_batch_seal.py::test_batch_seal_retired (planned) |
| <code>acceptance.16.269.1</code> | qualification | UNRUN |
| <code>req.16269</code> | ac_1 | tests/test_batch_seal.c is deleted because it exercises kernel names retired by the kernel domain (lxp_batch_seal_input at tests/test_batch_seal.c:42); tests/ci/test_retired_batch_seal.py asserts the deleted paths are absent. |
| <code>acceptance.16.270.1</code> | req | 16270.1 |
| <code>acceptance.16.270.1</code> | scenario | Given the repository after the change, when test_guarantor_duties_retired runs, then 2 deleted files are absent, no file under tests/ outside the tests/daemon, tests/bridge, tests/chain, tests/explorer and kernel suite trees names lxp_guarantor_authority_verdict, lxp_guarantor_process_batch, lxp_guarantor_dissent_record, lxp_guarantor_divergence, lxp_guarantor_first_divergence, lxp_guarantor_withhold |
| <code>acceptance.16.270.1</code> | test | tests/ci/test_retired_guarantor_duties.py::test_guarantor_duties_retired (planned) |
| <code>acceptance.16.270.1</code> | qualification | UNRUN |
| <code>req.16270</code> | ac_1 | tests/test_guarantor_duties.c, tests/test_guarantor_disagreement.c are deleted because they exercise kernel names retired by the kernel domain (lxp_guarantor_authority_verdict at tests/test_guarantor_duties.c:49; lxp_guarantor_dissent_record at tests/test_guarantor_disagreement.c:12); tests/ci/test_retired_guarantor_duties.py asserts the deleted paths are absent and no .c, .h, .py, .sh, .sol or .go file under tests/ outside the tests/daemon, tests/bridge, tests/chain, tests/explorer and kernel suite trees names lxp_guarantor_authority_verdict, lxp_guarantor_process_batch, lxp_guarantor_dissent_record, lxp_guarantor_divergence, lxp_guarantor_first_divergence, lxp_guarantor_withhold. |
| <code>acceptance.16.271.1</code> | req | 16271.1 |
| <code>acceptance.16.271.1</code> | scenario | Given the repository after the change, when test_equivocation_bond_retired runs, then 2 deleted files are absent, no file under tests/ outside the tests/daemon, tests/bridge, tests/chain, tests/explorer and kernel suite trees names lxp_sealed_header_record, lxp_equivocation_kind, lxp_slashing_submit, lxp_paxeer_slash_submit |
| <code>acceptance.16.271.1</code> | test | tests/ci/test_retired_equivocation_bond.py::test_equivocation_bond_retired (planned) |
| <code>acceptance.16.271.1</code> | qualification | UNRUN |
| <code>req.16271</code> | ac_1 | tests/test_equivocation.c, tests/test_paxeer_bond.c are deleted because they exercise kernel names retired by the kernel domain (lxp_checkpoint_certificate at tests/test_equivocation.c:54; lxp_checkpoint_certificate at tests/test_paxeer_bond.c:47); tests/ci/test_retired_equivocation_bond.py asserts the deleted paths are absent and no .c, .h, .py, .sh, .sol or .go file under tests/ outside the tests/daemon, tests/bridge, tests/chain, tests/explorer and kernel suite trees names lxp_sealed_header_record, lxp_equivocation_kind, lxp_slashing_submit, lxp_paxeer_slash_submit. |
| <code>acceptance.16.272.1</code> | req | 16272.1 |
| <code>acceptance.16.272.1</code> | scenario | Given the repository after the change, when test_guarantor_cert_retired runs, then 2 deleted files are absent, no file under tests/ outside the tests/daemon, tests/bridge, tests/chain, tests/explorer and kernel suite trees names lxp_augmented_receipt, lxp_receipt_augment |
| <code>acceptance.16.272.1</code> | test | tests/ci/test_retired_guarantor_cert.py::test_guarantor_cert_retired (planned) |
| <code>acceptance.16.272.1</code> | qualification | UNRUN |
| <code>req.16272</code> | ac_1 | tests/test_guarantor_cert.c, tests/test_receipt_offline.c are deleted because they exercise kernel names retired by the kernel domain (lxp_checkpoint_certificate at tests/test_guarantor_cert.c:46; lxp_checkpoint_certificate at tests/test_receipt_offline.c:75); tests/ci/test_retired_guarantor_cert.py asserts the deleted paths are absent and no .c, .h, .py, .sh, .sol or .go file under tests/ outside the tests/daemon, tests/bridge, tests/chain, tests/explorer and kernel suite trees names lxp_augmented_receipt, lxp_receipt_augment. |
| <code>acceptance.16.273.1</code> | req | 16273.1 |
| <code>acceptance.16.273.1</code> | scenario | Given the repository after the change, when test_guarantor_bond_custody_retired runs, then 2 deleted files are absent, no file under tests/ outside the tests/daemon, tests/bridge, tests/chain, tests/explorer and kernel suite trees names lxp_paxeer_custody_abi, lxp_checkpoint_registry_state, lxp_checkpoint_registration, lxp_paxeer_guarantor_attestation, lxp_paxeer_custody_abi_init, LXP_PAXEER_CUSTODY_INPUT_COUNT |
| <code>acceptance.16.273.1</code> | test | tests/ci/test_retired_guarantor_bond_custody.py::test_guarantor_bond_custody_retired (planned) |
| <code>acceptance.16.273.1</code> | qualification | UNRUN |
| <code>req.16273</code> | ac_1 | tests/test_guarantor_bond.c, tests/test_paxeer_custody.c are deleted because they exercise kernel names retired by the kernel domain (lxp_finalisation_state at tests/test_guarantor_bond.c:43; lxp_guarantor_ctx at tests/test_paxeer_custody.c:36); tests/ci/test_retired_guarantor_bond_custody.py asserts the deleted paths are absent and no .c, .h, .py, .sh, .sol or .go file under tests/ outside the tests/daemon, tests/bridge, tests/chain, tests/explorer and kernel suite trees names lxp_paxeer_custody_abi, lxp_checkpoint_registry_state, lxp_checkpoint_registration, lxp_paxeer_guarantor_attestation, lxp_paxeer_custody_abi_init, LXP_PAXEER_CUSTODY_INPUT_COUNT. |
| <code>acceptance.16.274.1</code> | req | 16274.1 |
| <code>acceptance.16.274.1</code> | scenario | Given the repository after the change, when test_emergency_exit_retired runs, then 1 deleted file is absent |
| <code>acceptance.16.274.1</code> | test | tests/ci/test_retired_emergency_exit.py::test_emergency_exit_retired (planned) |
| <code>acceptance.16.274.1</code> | qualification | UNRUN |
| <code>req.16274</code> | ac_1 | tests/test_emergency_exit.c is deleted because it exercises kernel names retired by the kernel domain (lxp_checkpoint_certificate at tests/test_emergency_exit.c:55); tests/ci/test_retired_emergency_exit.py asserts the deleted paths are absent. |
| <code>acceptance.16.275.1</code> | req | 16275.1 |
| <code>acceptance.16.275.1</code> | scenario | Given the repository after the change, when test_da_challenge_retired runs, then 2 deleted files are absent, no file under tests/ outside the tests/daemon, tests/bridge, tests/chain, tests/explorer and kernel suite trees names lxp_da_failure_evidence, lxp_da_challenge, lxp_da_challenge_response, lxp_da_challenge_registry, lxp_da_challenge_indices, lxp_da_challenge_issue |
| <code>acceptance.16.275.1</code> | test | tests/ci/test_retired_da_challenge.py::test_da_challenge_retired (planned) |
| <code>acceptance.16.275.1</code> | qualification | UNRUN |
| <code>req.16275</code> | ac_1 | tests/test_da_challenge.c, tests/test_da_unavailable.c are deleted because they exercise kernel names retired by the kernel domain (lxp_da_failure_evidence at tests/test_da_challenge.c:47; lxp_da_class at tests/test_da_unavailable.c:48); tests/ci/test_retired_da_challenge.py asserts the deleted paths are absent and no .c, .h, .py, .sh, .sol or .go file under tests/ outside the tests/daemon, tests/bridge, tests/chain, tests/explorer and kernel suite trees names lxp_da_failure_evidence, lxp_da_challenge, lxp_da_challenge_response, lxp_da_challenge_registry, lxp_da_challenge_indices, lxp_da_challenge_issue. |
| <code>acceptance.16.276.1</code> | req | 16276.1 |
| <code>acceptance.16.276.1</code> | scenario | Given the repository after the change, when test_da_retrieval_retired runs, then 2 deleted files are absent, no file under tests/ outside the tests/daemon, tests/bridge, tests/chain, tests/explorer and kernel suite trees names lxp_da_retrieval_request, lxp_da_serve_chunk, lxp_da_fetch, lxp_da_verify_served_bytes, lxp_da_lookup_kind, lxp_da_serve_chunk_proof |
| <code>acceptance.16.276.1</code> | test | tests/ci/test_retired_da_retrieval.py::test_da_retrieval_retired (planned) |
| <code>acceptance.16.276.1</code> | qualification | UNRUN |
| <code>req.16276</code> | ac_1 | tests/test_da_retrieval.c, tests/storage/lxp_test_da_verified.c are deleted because they exercise kernel names retired by the kernel domain (lxp_replica at tests/test_da_retrieval.c:6; lxp_batch_body at tests/storage/lxp_test_da_verified.c:36); tests/ci/test_retired_da_retrieval.py asserts the deleted paths are absent and no .c, .h, .py, .sh, .sol or .go file under tests/ outside the tests/daemon, tests/bridge, tests/chain, tests/explorer and kernel suite trees names lxp_da_retrieval_request, lxp_da_serve_chunk, lxp_da_fetch, lxp_da_verify_served_bytes, lxp_da_lookup_kind, lxp_da_serve_chunk_proof. |
| <code>acceptance.16.277.1</code> | req | 16277.1 |
| <code>acceptance.16.277.1</code> | scenario | Given the repository after the change, when test_da_bundle_retired runs, then 2 deleted files are absent, no file under tests/ outside the tests/daemon, tests/bridge, tests/chain, tests/explorer and kernel suite trees names LXP_DA_CLASS_COUNT, lxp_da_chunk, lxp_da_module_root, lxp_da_recovery_input, lxp_da_recovery_metadata_encode, lxp_da_possession_verify |
| <code>acceptance.16.277.1</code> | test | tests/ci/test_retired_da_bundle.py::test_da_bundle_retired (planned) |
| <code>acceptance.16.277.1</code> | qualification | UNRUN |
| <code>req.16277</code> | ac_1 | tests/test_da_bundle.c, tests/test_da_possession.c are deleted because they exercise kernel names retired by the kernel domain (lxp_da_bundle at tests/test_da_bundle.c:11; lxp_batch_body at tests/test_da_possession.c:48); tests/ci/test_retired_da_bundle.py asserts the deleted paths are absent and no .c, .h, .py, .sh, .sol or .go file under tests/ outside the tests/daemon, tests/bridge, tests/chain, tests/explorer and kernel suite trees names LXP_DA_CLASS_COUNT, lxp_da_chunk, lxp_da_module_root, lxp_da_recovery_input, lxp_da_recovery_metadata_encode, lxp_da_possession_verify. |
| <code>acceptance.16.278.1</code> | req | 16278.1 |
| <code>acceptance.16.278.1</code> | scenario | Given the repository after the change, when test_batch_wal_retired runs, then 3 deleted files are absent |
| <code>acceptance.16.278.1</code> | test | tests/ci/test_retired_batch_wal.py::test_batch_wal_retired (planned) |
| <code>acceptance.16.278.1</code> | qualification | UNRUN |
| <code>req.16278</code> | ac_1 | tests/storage/lxp_test_batch_wal_recovery.c, tests/storage/lxp_test_maintenance_publication.c, tests/storage/check_daemon_recovery_order.sh are deleted because they exercise kernel names retired by the kernel domain (lxp_batch_body at tests/storage/lxp_test_batch_wal_recovery.c:285; lxp_history_open at tests/storage/lxp_test_maintenance_publication.c:116; lxp_history_open at tests/storage/check_daemon_recovery_order.sh:34); tests/ci/test_retired_batch_wal.py asserts the deleted paths are absent. |
| <code>acceptance.16.279.1</code> | req | 16279.1 |
| <code>acceptance.16.279.1</code> | scenario | Given the repository after the change, when test_batch_identity_retired runs, then 2 deleted files are absent |
| <code>acceptance.16.279.1</code> | test | tests/ci/test_retired_batch_identity.py::test_batch_identity_retired (planned) |
| <code>acceptance.16.279.1</code> | qualification | UNRUN |
| <code>req.16279</code> | ac_1 | tests/protocol/lxp_test_batch_identity.c, tests/protocol/lxp_test_terminal_rejection.c are deleted because they exercise kernel names retired by the kernel domain (lxp_history_open at tests/protocol/lxp_test_terminal_rejection.c:168); tests/ci/test_retired_batch_identity.py asserts the deleted paths are absent. |
| <code>acceptance.16.280.1</code> | req | 16280.1 |
| <code>acceptance.16.280.1</code> | scenario | Given the repository after the change, when test_finality_evidence_retired runs, then 1 deleted file is absent, no file under tests/ outside the tests/daemon, tests/bridge, tests/chain, tests/explorer and kernel suite trees names lxp_da_log_store_body |
| <code>acceptance.16.280.1</code> | test | tests/ci/test_retired_finality_evidence.py::test_finality_evidence_retired (planned) |
| <code>acceptance.16.280.1</code> | qualification | UNRUN |
| <code>req.16280</code> | ac_1 | tests/storage/lxp_test_finality_evidence.c is deleted because it exercises kernel names retired by the kernel domain (lxp_replica at tests/storage/lxp_test_finality_evidence.c:7); tests/ci/test_retired_finality_evidence.py asserts the deleted paths are absent and no .c, .h, .py, .sh, .sol or .go file under tests/ outside the tests/daemon, tests/bridge, tests/chain, tests/explorer and kernel suite trees names lxp_da_log_store_body. |
| <code>acceptance.16.281.1</code> | req | 16281.1 |
| <code>acceptance.16.281.1</code> | scenario | Given the repository after the change, when test_log_batch_header_retired runs, then tests/storage/lxp_test_recovery.c contains none of LXP_LOG_BATCH_HEADER, tests/storage/lxp_test_log_durability.c contains none of LXP_LOG_BATCH_HEADER, make -C . build/tests/lxp_test_recovery build/tests/lxp_test_log_durability succeeds and each binary exits 0 |
| <code>acceptance.16.281.1</code> | test | tests/ci/test_retired_log_batch_header.py::test_log_batch_header_retired (planned) |
| <code>acceptance.16.281.1</code> | qualification | UNRUN |
| <code>req.16281</code> | ac_1 | tests/storage/lxp_test_recovery.c drops every use of LXP_LOG_BATCH_HEADER (first at tests/storage/lxp_test_recovery.c:405); tests/storage/lxp_test_log_durability.c drops every use of LXP_LOG_BATCH_HEADER (first at tests/storage/lxp_test_log_durability.c:32); tests/ci/test_retired_log_batch_header.py asserts tests/storage/lxp_test_recovery.c no longer names LXP_LOG_BATCH_HEADER; tests/storage/lxp_test_log_durability.c no longer names LXP_LOG_BATCH_HEADER, then builds and runs build/tests/lxp_test_recovery, build/tests/lxp_test_log_durability with exit 0. |
| <code>acceptance.16.282.1</code> | req | 16282.1 |
| <code>acceptance.16.282.1</code> | scenario | Given the repository after the change, when test_protocol_cases_retired runs, then tests/protocol/lxp_test_protocol.c contains none of lxp_checkpoint_maximum_attestation_delay_ms, lxp_guarantor_attestation, lxp_checkpoint_certificate, lxp_guarantor_key_record, lxp_guarantor_cert, lxp_guarantor_set, lxp_finalisation_state, lxp_finalisation_requirements, tests/protocol/lxp_test_dispatch.c contains none of lx_batch_header, lx_web_observation_encode, lx_web_root, lx_batch_header_set_web_root, make -C . build/tests/lxp_test_protocol build/tests/lxp_test_dispatch succeeds and each binary exits 0 |
| <code>acceptance.16.282.1</code> | test | tests/ci/test_retired_protocol_cases.py::test_protocol_cases_retired (planned) |
| <code>acceptance.16.282.1</code> | qualification | UNRUN |
| <code>req.16282</code> | ac_1 | tests/protocol/lxp_test_protocol.c drops every use of lxp_checkpoint_maximum_attestation_delay_ms, lxp_guarantor_attestation, lxp_checkpoint_certificate, lxp_guarantor_key_record, lxp_guarantor_cert, lxp_guarantor_set, lxp_finalisation_state, lxp_finalisation_requirements (first at tests/protocol/lxp_test_protocol.c:367, tests/protocol/lxp_test_protocol.c:458, tests/protocol/lxp_test_protocol.c:532, tests/protocol/lxp_test_protocol.c:534); tests/protocol/lxp_test_dispatch.c drops every use of lx_batch_header, lx_web_observation_encode, lx_web_root, lx_batch_header_set_web_root (first at tests/protocol/lxp_test_dispatch.c:167, tests/protocol/lxp_test_dispatch.c:228, tests/protocol/lxp_test_dispatch.c:301, tests/protocol/lxp_test_dispatch.c:303); tests/ci/test_retired_protocol_cases.py asserts tests/protocol/lxp_test_protocol.c no longer names lxp_checkpoint_maximum_attestation_delay_ms, lxp_guarantor_attestation, lxp_checkpoint_certificate, lxp_guarantor_key_record, lxp_guarantor_cert, lxp_guarantor_set, lxp_finalisation_state, lxp_finalisation_requirements; tests/protocol/lxp_test_dispatch.c no longer names lx_batch_header, lx_web_observation_encode, lx_web_root, lx_batch_header_set_web_root, then builds and runs build/tests/lxp_test_protocol, build/tests/lxp_test_dispatch with exit 0. |
| <code>acceptance.16.283.1</code> | req | 16283.1 |
| <code>acceptance.16.283.1</code> | scenario | Given the repository after the change, when test_state_withdraw_web_retired runs, then tests/state/lxp_test_state_diff.c contains none of lxp_batch_body, lxp_da_bundle, lxp_da_receipt_section_encode, lxp_da_receipt_section_decode, lxp_batch_availability_root, lxp_da_bundle_build, lxp_da_bundle_root, lxp_da_recovery_from_kernel, tests/test_bridge_withdraw.c contains none of lxp_checkpoint_certificate, lxp_guarantor_ctx, lxp_guarantor_attestation, lxp_guarantor_key_record, lxp_guarantor_cert, lxp_guarantor_attest, lxp_guarantor_cert_assemble, lxp_checkpoint_certificate_hash, tests/test_web_program_path.c contains none of lx_web_observation_encode, lx_batch_header, lx_web_root, lx_batch_header_set_web_root, make -C . build/tests/lxp_test_state_diff build/tests/test_bridge_withdraw build/tests/lxp_test_program_artifacts succeeds and each binary exits 0 |
| <code>acceptance.16.283.1</code> | test | tests/ci/test_retired_state_withdraw_web.py::test_state_withdraw_web_retired (planned) |
| <code>acceptance.16.283.1</code> | qualification | UNRUN |
| <code>req.16283</code> | ac_1 | tests/state/lxp_test_state_diff.c drops every use of lxp_batch_body, lxp_da_bundle, lxp_da_receipt_section_encode, lxp_da_receipt_section_decode, lxp_batch_availability_root, lxp_da_bundle_build, lxp_da_bundle_root, lxp_da_recovery_from_kernel (first at tests/state/lxp_test_state_diff.c:32, tests/state/lxp_test_state_diff.c:33, tests/state/lxp_test_state_diff.c:44, tests/state/lxp_test_state_diff.c:48); tests/test_bridge_withdraw.c drops every use of lxp_checkpoint_certificate, lxp_guarantor_ctx, lxp_guarantor_attestation, lxp_guarantor_key_record, lxp_guarantor_cert, lxp_guarantor_attest, lxp_guarantor_cert_assemble, lxp_checkpoint_certificate_hash (first at tests/test_bridge_withdraw.c:289, tests/test_bridge_withdraw.c:290, tests/test_bridge_withdraw.c:291, tests/test_bridge_withdraw.c:292); tests/test_web_program_path.c drops every use of lx_web_observation_encode, lx_batch_header, lx_web_root, lx_batch_header_set_web_root (first at tests/test_web_program_path.c:711, tests/test_web_program_path.c:789, tests/test_web_program_path.c:793, tests/test_web_program_path.c:795); tests/ci/test_retired_state_withdraw_web.py asserts tests/state/lxp_test_state_diff.c no longer names lxp_batch_body, lxp_da_bundle, lxp_da_receipt_section_encode, lxp_da_receipt_section_decode, lxp_batch_availability_root, lxp_da_bundle_build, lxp_da_bundle_root, lxp_da_recovery_from_kernel; tests/test_bridge_withdraw.c no longer names lxp_checkpoint_certificate, lxp_guarantor_ctx, lxp_guarantor_attestation, lxp_guarantor_key_record, lxp_guarantor_cert, lxp_guarantor_attest, lxp_guarantor_cert_assemble, lxp_checkpoint_certificate_hash; tests/test_web_program_path.c no longer names lx_web_observation_encode, lx_batch_header, lx_web_root, lx_batch_header_set_web_root, then builds and runs build/tests/lxp_test_state_diff, build/tests/test_bridge_withdraw, build/tests/lxp_test_program_artifacts with exit 0. |
| <code>acceptance.16.284.1</code> | req | 16284.1 |
| <code>acceptance.16.284.1</code> | scenario | Given the repository after the change, when test_oracle_retired runs, then 3 deleted files are absent, no file under tests/ outside the tests/daemon, tests/bridge, tests/chain, tests/explorer and kernel suite trees names lx_oracle_transport_encode, lx_oracle_adapter_config, lx_oracle_activity_encode_signed, lx_oracle_activity_encode, lx_oracle_adapter_isolation_check, lx_oracle_availability_bundle |
| <code>acceptance.16.284.1</code> | test | tests/ci/test_retired_oracle.py::test_oracle_retired (planned) |
| <code>acceptance.16.284.1</code> | qualification | UNRUN |
| <code>req.16284</code> | ac_1 | tests/modules/test_oracle_transport.c, tests/network/test_oracle_adapter.c, tests/sequencer/test_oracle_root.c are deleted because they exercise kernel names retired by the kernel domain (lx_oracle_observation_sign at tests/modules/test_oracle_transport.c:16; lx_oracle_observation_sign at tests/network/test_oracle_adapter.c:26; lx_oracle_observation_sign at tests/sequencer/test_oracle_root.c:19); tests/ci/test_retired_oracle.py asserts the deleted paths are absent and no .c, .h, .py, .sh, .sol or .go file under tests/ outside the tests/daemon, tests/bridge, tests/chain, tests/explorer and kernel suite trees names lx_oracle_transport_encode, lx_oracle_adapter_config, lx_oracle_activity_encode_signed, lx_oracle_activity_encode, lx_oracle_adapter_isolation_check, lx_oracle_availability_bundle. |
| <code>acceptance.16.285.1</code> | req | 16285.1 |
| <code>acceptance.16.285.1</code> | scenario | Given the repository after the change, when test_web_retired runs, then 3 deleted files are absent, no file under tests/ outside the tests/daemon, tests/bridge, tests/chain, tests/explorer and kernel suite trees names lx_web_activity_encode, lx_web_adapter_run, lx_web_root_from_availability, lx_web_leaf_encode, lx_web_availability_bundle_build |
| <code>acceptance.16.285.1</code> | test | tests/ci/test_retired_web.py::test_web_retired (planned) |
| <code>acceptance.16.285.1</code> | qualification | UNRUN |
| <code>req.16285</code> | ac_1 | tests/modules/test_web_intake.c, tests/network/test_web_adapter.c, tests/sequencer/test_web_root.c are deleted because they exercise kernel names retired by the kernel domain (lx_web_observation_encode at tests/modules/test_web_intake.c:231; lx_web_observation_encode at tests/network/test_web_adapter.c:174; lx_batch_header at tests/sequencer/test_web_root.c:154); tests/ci/test_retired_web.py asserts the deleted paths are absent and no .c, .h, .py, .sh, .sol or .go file under tests/ outside the tests/daemon, tests/bridge, tests/chain, tests/explorer and kernel suite trees names lx_web_activity_encode, lx_web_adapter_run, lx_web_root_from_availability, lx_web_leaf_encode, lx_web_availability_bundle_build. |
| <code>acceptance.16.286.1</code> | req | 16286.1 |
| <code>acceptance.16.286.1</code> | scenario | Given the repository after the change, when test_programs_batch_retired runs, then 3 deleted files are absent, no file under tests/ outside the tests/daemon, tests/bridge, tests/chain, tests/explorer and kernel suite trees names lxp_replay_batch_publication, tests/programs/test_call_activity.c contains none of lxp_batch_sign, make -C . build/tests/lxp_test_maintenance_publication succeeds and each binary exits 0 |
| <code>acceptance.16.286.1</code> | test | tests/ci/test_retired_programs_batch.py::test_programs_batch_retired (planned) |
| <code>acceptance.16.286.1</code> | qualification | UNRUN |
| <code>req.16286</code> | ac_1 | tests/programs/test_occupancy_batch.c, tests/programs/test_balance_read.c, tests/programs/test_interface_protocol.c are deleted because they exercise kernel names retired by the kernel domain (lxp_batch_body at tests/programs/test_occupancy_batch.c:86; lxp_history_open at tests/programs/test_balance_read.c:283; lxp_batch_sign at tests/programs/test_interface_protocol.c:315); tests/programs/test_call_activity.c drops every use of lxp_batch_sign (first at tests/programs/test_call_activity.c:1152); tests/ci/test_retired_programs_batch.py asserts the deleted paths are absent and no .c, .h, .py, .sh, .sol or .go file under tests/ outside the tests/daemon, tests/bridge, tests/chain, tests/explorer and kernel suite trees names lxp_replay_batch_publication and tests/programs/test_call_activity.c no longer names lxp_batch_sign, then builds and runs build/tests/lxp_test_maintenance_publication with exit 0. |
| <code>acceptance.16.287.1</code> | req | 16287.1 |
| <code>acceptance.16.287.1</code> | scenario | Given the repository after the change, when test_replay_support_retired runs, then 2 deleted files are absent, no file under tests/ outside the tests/daemon, tests/bridge, tests/chain, tests/explorer and kernel suite trees names lxp_guarantor_recompute_roots, lxp_replay_checkpoint, lxp_replay_checkpoint_create, lxp_replay_checkpoint_restore, lxp_replay_checkpoint_destroy, lxp_replay_engine_bind_transaction, tests/test_tools.c contains none of lxp_real_replay.h, lxp_batch_body, lxp_da_bundle, lxp_checkpoint_certificate, lxp_guarantor_ctx, lxp_guarantor_attestation, lxp_guarantor_key_record, lxp_guarantor_cert, make -C . build/tests/test_tools succeeds and each binary exits 0 |
| <code>acceptance.16.287.1</code> | test | tests/ci/test_retired_replay_support.py::test_replay_support_retired (planned) |
| <code>acceptance.16.287.1</code> | qualification | UNRUN |
| <code>req.16287</code> | ac_1 | tests/support/lxp_pay1_replay.h, tests/support/lxp_real_replay.h are deleted because they exercise kernel names retired by the kernel domain (lxp_batch_body at tests/support/lxp_pay1_replay.h:119; lxp_replay_engine at tests/support/lxp_real_replay.h:42); tests/test_tools.c drops every use of lxp_real_replay.h, lxp_batch_body, lxp_da_bundle, lxp_checkpoint_certificate, lxp_guarantor_ctx, lxp_guarantor_attestation, lxp_guarantor_key_record, lxp_guarantor_cert (first at tests/test_tools.c:5, tests/test_tools.c:103, tests/test_tools.c:104, tests/test_tools.c:106); tests/ci/test_retired_replay_support.py asserts the deleted paths are absent and no .c, .h, .py, .sh, .sol or .go file under tests/ outside the tests/daemon, tests/bridge, tests/chain, tests/explorer and kernel suite trees names lxp_guarantor_recompute_roots, lxp_replay_checkpoint, lxp_replay_checkpoint_create, lxp_replay_checkpoint_restore, lxp_replay_checkpoint_destroy, lxp_replay_engine_bind_transaction and tests/test_tools.c no longer names lxp_real_replay.h, lxp_batch_body, lxp_da_bundle, lxp_checkpoint_certificate, lxp_guarantor_ctx, lxp_guarantor_attestation, lxp_guarantor_key_record, lxp_guarantor_cert, then builds and runs build/tests/test_tools with exit 0. |
| <code>acceptance.16.288.1</code> | req | 16288.1 |
| <code>acceptance.16.288.1</code> | scenario | Given the repository after the change, when test_daemon_lni_retired runs, then 1 deleted file is absent, tests/programs/test_blob_admission.c contains none of lxp_daemon_lni_internal.h, tests/programs/test_paxai_native_gaps.c contains none of lxp_daemon_lni_internal.h, tests/programs/test_paxai_storage_continuity.c contains none of lxp_daemon_lni_internal.h |
| <code>acceptance.16.288.1</code> | test | tests/ci/test_retired_daemon_lni.py::test_daemon_lni_retired (planned) |
| <code>acceptance.16.288.1</code> | qualification | UNRUN |
| <code>req.16288</code> | ac_1 | tests/test_daemon_lni_admission.c is deleted because None; tests/programs/test_blob_admission.c drops every use of lxp_daemon_lni_internal.h (first at tests/programs/test_blob_admission.c:17); tests/programs/test_paxai_native_gaps.c drops every use of lxp_daemon_lni_internal.h (first at tests/programs/test_paxai_native_gaps.c:22); tests/programs/test_paxai_storage_continuity.c drops every use of lxp_daemon_lni_internal.h (first at tests/programs/test_paxai_storage_continuity.c:24); tests/ci/test_retired_daemon_lni.py asserts the deleted paths are absent and tests/programs/test_blob_admission.c no longer names lxp_daemon_lni_internal.h; tests/programs/test_paxai_native_gaps.c no longer names lxp_daemon_lni_internal.h; tests/programs/test_paxai_storage_continuity.c no longer names lxp_daemon_lni_internal.h. |
| <code>acceptance.16.289.1</code> | req | 16289.1 |
| <code>acceptance.16.289.1</code> | scenario | Given the repository after the change, when test_layerxd_retired runs, then 2 deleted files are absent |
| <code>acceptance.16.289.1</code> | test | tests/ci/test_retired_layerxd.py::test_layerxd_retired (planned) |
| <code>acceptance.16.289.1</code> | qualification | UNRUN |
| <code>req.16289</code> | ac_1 | tests/test_layerxd.c, tests/test_daemon_bootstrap_artifact.c are deleted because they compile cmd/layerxd sources that the node-runtime domain retires; tests/ci/test_retired_layerxd.py asserts the deleted paths are absent. |
| <code>acceptance.16.290.1</code> | req | 16290.1 |
| <code>acceptance.16.290.1</code> | scenario | Given the repository after the change, when test_verify_cli_retired runs, then 1 deleted file is absent |
| <code>acceptance.16.290.1</code> | test | tests/ci/test_retired_verify_cli.py::test_verify_cli_retired (planned) |
| <code>acceptance.16.290.1</code> | qualification | UNRUN |
| <code>req.16290</code> | ac_1 | tests/tools/test_verify_cli.c is deleted because it exercises kernel names retired by the kernel domain (lxp_batch_sign at tests/tools/test_verify_cli.c:363); tests/ci/test_retired_verify_cli.py asserts the deleted paths are absent. |
| <code>acceptance.16.291.1</code> | req | 16291.1 |
| <code>acceptance.16.291.1</code> | scenario | Given the repository after the change, when test_relay_archive_retired runs, then 4 deleted files are absent, no file under tests/ outside the tests/daemon, tests/bridge, tests/chain, tests/explorer and kernel suite trees names layerx-archive-codec |
| <code>acceptance.16.291.1</code> | test | tests/ci/test_retired_relay_archive.py::test_relay_archive_retired (planned) |
| <code>acceptance.16.291.1</code> | qualification | UNRUN |
| <code>req.16291</code> | ac_1 | tests/relay-archive/e2e.py, tests/relay-archive/plain_listener.py, tests/relay-archive/sign_activity.c, tests/relay-archive/state_archive_lifecycle.py are deleted because they start layerxd and bin/layerx-archive-codec, which the node-runtime domain retires; tests/ci/test_retired_relay_archive.py asserts the deleted paths are absent and no .c, .h, .py, .sh, .sol or .go file under tests/ outside the tests/daemon, tests/bridge, tests/chain, tests/explorer and kernel suite trees names layerx-archive-codec. |
| <code>acceptance.16.292.1</code> | req | 16292.1 |
| <code>acceptance.16.292.1</code> | scenario | Given the repository after the change, when test_module_drivers_retired runs, then 2 deleted files are absent, tests/fixtures/custody/paxeer-light-v1/README contains none of custody-proof-build |
| <code>acceptance.16.292.1</code> | test | tests/ci/test_retired_module_drivers.py::test_module_drivers_retired (planned) |
| <code>acceptance.16.292.1</code> | qualification | UNRUN |
| <code>req.16292</code> | ac_1 | tests/modules/paxeer_x_budget_lifecycle.py, tests/modules/paxeer_x_escrow_replay_binding.py are deleted because they compile tests/daemon programs and run layerx-custody-proof, which the node-runtime domain retires (tests/modules/paxeer_x_budget_lifecycle.py:26); tests/fixtures/custody/paxeer-light-v1/README drops every use of custody-proof-build (first at tests/fixtures/custody/paxeer-light-v1/README:44); tests/ci/test_retired_module_drivers.py asserts the deleted paths are absent and tests/fixtures/custody/paxeer-light-v1/README no longer names custody-proof-build. |
| <code>acceptance.16.293.1</code> | req | 16293.1 |
| <code>acceptance.16.293.1</code> | scenario | Given the repository after the change, when test_qualification_drivers_retired runs, then 3 deleted files are absent, no file under tests/ outside the tests/daemon, tests/bridge, tests/chain, tests/explorer and kernel suite trees names lx_oracle_adapter_run |
| <code>acceptance.16.293.1</code> | test | tests/ci/test_retired_qualification_drivers.py::test_qualification_drivers_retired (planned) |
| <code>acceptance.16.293.1</code> | qualification | UNRUN |
| <code>req.16293</code> | ac_1 | tests/qualification/paxeer_x_exchange_tif.py, tests/qualification/paxeer_x_oracle_transport.py, tests/qualification/paxeer_x_idempotency_lifecycle.py are deleted because they exercise kernel names retired by the kernel domain (lx_oracle_adapter_run at tests/qualification/paxeer_x_oracle_transport.py:386); tests/ci/test_retired_qualification_drivers.py asserts the deleted paths are absent and no .c, .h, .py, .sh, .sol or .go file under tests/ outside the tests/daemon, tests/bridge, tests/chain, tests/explorer and kernel suite trees names lx_oracle_adapter_run. |
| <code>acceptance.16.294.1</code> | req | 16294.1 |
| <code>acceptance.16.294.1</code> | scenario | Given the repository after the change, when test_paxeer_anchor_retired runs, then 2 deleted files are absent, no file under tests/ outside the tests/daemon, tests/bridge, tests/chain, tests/explorer and kernel suite trees names lxp_paxeer_abi_selector, lxp_paxeer_checkpoint_certificate_encode, lxp_checkpoint_submit_calldata, lxp_paxeer_abi_encode_bytes |
| <code>acceptance.16.294.1</code> | test | tests/ci/test_retired_paxeer_anchor.py::test_paxeer_anchor_retired (planned) |
| <code>acceptance.16.294.1</code> | qualification | UNRUN |
| <code>req.16294</code> | ac_1 | tests/paxeer/test_anchor_submit.c, tests/paxeer/lxp_da_withhold.sh are deleted because they exercise kernel names retired by the kernel domain (lxp_guarantor_cert at tests/paxeer/test_anchor_submit.c:103); tests/ci/test_retired_paxeer_anchor.py asserts the deleted paths are absent and no .c, .h, .py, .sh, .sol or .go file under tests/ outside the tests/daemon, tests/bridge, tests/chain, tests/explorer and kernel suite trees names lxp_paxeer_abi_selector, lxp_paxeer_checkpoint_certificate_encode, lxp_checkpoint_submit_calldata, lxp_paxeer_abi_encode_bytes. |
| <code>acceptance.16.295.1</code> | req | 16295.1 |
| <code>acceptance.16.295.1</code> | scenario | Given the repository after the change, when test_paxeer_membership_retired runs, then 3 deleted files are absent, no file under tests/ outside the tests/daemon, tests/bridge, tests/chain, tests/explorer and kernel suite trees names lxp_paxeer_bond_binding_encode, lxp_paxeer_bond_binding, lxp_paxeer_bond_binding_decode, lxp_paxeer_bond_binding_adopt, lxp_paxeer_membership_commitment, lxp_paxeer_bond_deposit_record |
| <code>acceptance.16.295.1</code> | test | tests/ci/test_retired_paxeer_membership.py::test_paxeer_membership_retired (planned) |
| <code>acceptance.16.295.1</code> | qualification | UNRUN |
| <code>req.16295</code> | ac_1 | tests/paxeer/membership_sync_driver.c, tests/paxeer/test_membership_sync.c, tests/paxeer/membership-sync-chain.py are deleted because they exercise kernel names retired by the kernel domain (lxp_paxeer_bond_state at tests/paxeer/membership_sync_driver.c:55; lxp_guarantor_bond_state at tests/paxeer/test_membership_sync.c:34); tests/ci/test_retired_paxeer_membership.py asserts the deleted paths are absent and no .c, .h, .py, .sh, .sol or .go file under tests/ outside the tests/daemon, tests/bridge, tests/chain, tests/explorer and kernel suite trees names lxp_paxeer_bond_binding_encode, lxp_paxeer_bond_binding, lxp_paxeer_bond_binding_decode, lxp_paxeer_bond_binding_adopt, lxp_paxeer_membership_commitment, lxp_paxeer_bond_deposit_record. |
| <code>acceptance.16.296.1</code> | req | 16296.1 |
| <code>acceptance.16.296.1</code> | scenario | Given tests/ci/websearch-context.py after the edit, when test_websearch_settlement_retired runs it as a subprocess, then it exits 0 and the script text no longer contains checkpoint-settlement.json |
| <code>acceptance.16.296.1</code> | test | tests/ci/test_retired_websearch_settlement.py::test_websearch_settlement_retired (planned) |
| <code>acceptance.16.296.1</code> | qualification | UNRUN |
| <code>req.16296</code> | ac_1 | contracts/config/checkpoint-settlement.json is removed from EXTRA at tests/ci/websearch-context.py:22, so tests/ci/websearch-context.py stops requiring a file the chain-modules domain retires and still exits 0. |
| <code>capability.16.30.1</code> | title | Checkpoint and library Solidity suites retired |
| <code>capability.16.30.1</code> | reqs | 16261.1 |
| <code>capability.16.30.2</code> | title | Deployment and governance Solidity suites retired |
| <code>capability.16.30.2</code> | reqs | 16262.1 |
| <code>capability.16.31.1</code> | title | Delete the gateway receive suite with the retired gateway claim path |
| <code>capability.16.31.1</code> | reqs | 16263.1 |
| <code>capability.16.31.2</code> | title | Delete the sequencer recovery and sequence suites |
| <code>capability.16.31.2</code> | reqs | 16264.1 |
| <code>capability.16.31.3</code> | title | Delete the batch publication, batch time and history query suites |
| <code>capability.16.31.3</code> | reqs | 16265.1 |
| <code>capability.16.31.4</code> | title | Delete the cross-architecture replay suite and its matrix script |
| <code>capability.16.31.4</code> | reqs | 16266.1 |
| <code>capability.16.31.5</code> | title | Delete the replica ingest, divergence and replay suites |
| <code>capability.16.31.5</code> | reqs | 16267.1 |
| <code>capability.16.31.6</code> | title | Remove the retired sequencer and replica fault cases from lxp_qual_faults |
| <code>capability.16.31.6</code> | reqs | 16268.1 |
| <code>capability.16.31.7</code> | title | Delete the batch seal suite |
| <code>capability.16.31.7</code> | reqs | 16269.1 |
| <code>capability.16.32.1</code> | title | Delete the guarantor duty and disagreement suites |
| <code>capability.16.32.1</code> | reqs | 16270.1 |
| <code>capability.16.32.10</code> | title | Delete the batch identity and terminal rejection protocol suites |
| <code>capability.16.32.10</code> | reqs | 16279.1 |
| <code>capability.16.32.11</code> | title | Delete the finality evidence suite built on guarantor certificates |
| <code>capability.16.32.11</code> | reqs | 16280.1 |
| <code>capability.16.32.12</code> | title | Switch the storage recovery and durability suites from batch headers to activity records |
| <code>capability.16.32.12</code> | reqs | 16281.1 |
| <code>capability.16.32.2</code> | title | Delete the equivocation and paxeer bond suites |
| <code>capability.16.32.2</code> | reqs | 16271.1 |
| <code>capability.16.32.3</code> | title | Delete the guarantor certificate and offline receipt suites |
| <code>capability.16.32.3</code> | reqs | 16272.1 |
| <code>capability.16.32.4</code> | title | Delete the guarantor bond and paxeer custody suites |
| <code>capability.16.32.4</code> | reqs | 16273.1 |
| <code>capability.16.32.5</code> | title | Delete the emergency exit suite built on guarantor certificates |
| <code>capability.16.32.5</code> | reqs | 16274.1 |
| <code>capability.16.32.6</code> | title | Delete the data-availability challenge and unavailability suites |
| <code>capability.16.32.6</code> | reqs | 16275.1 |
| <code>capability.16.32.7</code> | title | Delete the data-availability retrieval and verified-read suites |
| <code>capability.16.32.7</code> | reqs | 16276.1 |
| <code>capability.16.32.8</code> | title | Delete the data-availability bundle and possession suites |
| <code>capability.16.32.8</code> | reqs | 16277.1 |
| <code>capability.16.32.9</code> | title | Delete the batch write-ahead log, maintenance publication and recovery-order checks |
| <code>capability.16.32.9</code> | reqs | 16278.1 |
| <code>capability.16.33.1</code> | title | Remove retired guarantor, checkpoint and web-root cases from the protocol and dispatch suites |
| <code>capability.16.33.1</code> | reqs | 16282.1 |
| <code>capability.16.33.2</code> | title | Remove retired bundle, certificate and web-root uses from the state diff, withdraw and web program suites |
| <code>capability.16.33.2</code> | reqs | 16283.1 |
| <code>capability.16.33.3</code> | title | Delete the oracle transport, adapter and root suites |
| <code>capability.16.33.3</code> | reqs | 16284.1 |
| <code>capability.16.33.4</code> | title | Delete the web intake, adapter and root suites |
| <code>capability.16.33.4</code> | reqs | 16285.1 |
| <code>capability.16.33.5</code> | title | Delete the programs occupancy, balance-read and interface suites; drop batch signing from call activity |
| <code>capability.16.33.5</code> | reqs | 16286.1 |
| <code>capability.16.33.6</code> | title | Retire the replay fixture headers and the replay and certificate cases of the tools suite |
| <code>capability.16.33.6</code> | reqs | 16287.1 |
| <code>capability.16.34.1</code> | title | Drop the retired daemon LNI socket server from the program suites |
| <code>capability.16.34.1</code> | reqs | 16288.1 |
| <code>capability.16.34.2</code> | title | Delete the layerxd and daemon bootstrap artifact suites |
| <code>capability.16.34.2</code> | reqs | 16289.1 |
| <code>capability.16.34.3</code> | title | Delete the verify CLI suite of the retired daemon tools |
| <code>capability.16.34.3</code> | reqs | 16290.1 |
| <code>capability.16.34.4</code> | title | Delete the relay-archive end-to-end suite that drives the retired daemon |
| <code>capability.16.34.4</code> | reqs | 16291.1 |
| <code>capability.16.35.1</code> | title | Delete the daemon-driven budget and escrow module drivers |
| <code>capability.16.35.1</code> | reqs | 16292.1 |
| <code>capability.16.35.2</code> | title | Delete the daemon-driven exchange, oracle and idempotency qualification drivers |
| <code>capability.16.35.2</code> | reqs | 16293.1 |
| <code>capability.16.36.1</code> | title | Delete the paxeer anchor submission and DA-withhold suites |
| <code>capability.16.36.1</code> | reqs | 16294.1 |
| <code>capability.16.36.2</code> | title | Delete the paxeer membership sync suites |
| <code>capability.16.36.2</code> | reqs | 16295.1 |
| <code>capability.16.36.3</code> | title | Drop the retired checkpoint settlement document from the websearch context check |
| <code>capability.16.36.3</code> | reqs | 16296.1 |
| <code>capability.16.37.1</code> | title | Retired-suite assertion helper |
| <code>capability.16.37.1</code> | reqs | 16241.1<br>16241.2<br>16241.3 |
| <code>resolution.17.984</code> | title | decision.1622 binds: No kernel-to-chain parity harness under tests/parity |
| <code>resolution.17.984</code> | applies_to | decision.1622<br>task.16.31.2<br>task.16.33.4 |
| <code>resolution.17.984</code> | source | decision.1622 in fragments/J59/decision.kvx; tasks from affects |
| <code>resolution.17.985</code> | title | decision.1623 binds: tests/solidity retires as a whole |
| <code>resolution.17.985</code> | applies_to | decision.1623<br>task.16.30.1<br>task.16.30.2 |
| <code>resolution.17.985</code> | source | decision.1623 in fragments/J59/decision.kvx; tasks from affects |
| <code>resolution.17.988</code> | title | decision.1626 binds: Daemon-driven test suites retire with the daemon |
| <code>resolution.17.988</code> | applies_to | decision.1626<br>task.16.34.1<br>task.16.34.2<br>task.16.34.3<br>task.16.35.1 |
| <code>resolution.17.988</code> | source | decision.1626 in fragments/J59/decision.kvx; tasks from affects |
