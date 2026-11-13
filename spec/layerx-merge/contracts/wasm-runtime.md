# Wasm runtime contract

The [Wasm runtime](../../../wasm-runtime) supplies the Go bindings, message types, errors and VM API used by the chain Wasm module. Its declarations are listed below.

The tables identify code declarations and configuration names. Activation, authentication and transport registration are defined by the linked implementations.

## errors

| Kind | Declaration | Source |
|---|---|---|
| `error_code` | `OutOfGasError` | [wasm-runtime/types/types.go](../../../wasm-runtime/types/types.go) |
| `error_code` | `type SystemError struct {` | [wasm-runtime/types/systemerror.go](../../../wasm-runtime/types/systemerror.go) |
| `error_code` | `type InvalidRequest struct {` | [wasm-runtime/types/systemerror.go](../../../wasm-runtime/types/systemerror.go) |
| `error_code` | `type InvalidResponse struct {` | [wasm-runtime/types/systemerror.go](../../../wasm-runtime/types/systemerror.go) |
| `error_code` | `type NoSuchCode struct {` | [wasm-runtime/types/systemerror.go](../../../wasm-runtime/types/systemerror.go) |
| `error_code` | `type NoSuchContract struct {` | [wasm-runtime/types/systemerror.go](../../../wasm-runtime/types/systemerror.go) |
| `error_code` | `Unknown` | [wasm-runtime/types/systemerror.go](../../../wasm-runtime/types/systemerror.go) |
| `error_code` | `type UnsupportedRequest struct {` | [wasm-runtime/types/systemerror.go](../../../wasm-runtime/types/systemerror.go) |
| `error_code` | `func ToSystemError(err error) *SystemError` | [wasm-runtime/types/systemerror.go](../../../wasm-runtime/types/systemerror.go) |

## types

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `Abstain` | [wasm-runtime/types/msg.go](../../../wasm-runtime/types/msg.go) |
| `go_exported` | `type AllBalancesQuery struct` | [wasm-runtime/types/queries.go](../../../wasm-runtime/types/queries.go) |
| `go_exported` | `type AllBalancesResponse struct` | [wasm-runtime/types/queries.go](../../../wasm-runtime/types/queries.go) |
| `go_exported` | `type AllDelegationsQuery struct` | [wasm-runtime/types/queries.go](../../../wasm-runtime/types/queries.go) |
| `go_exported` | `type AllDelegationsResponse struct` | [wasm-runtime/types/queries.go](../../../wasm-runtime/types/queries.go) |
| `go_exported` | `type AllValidatorsQuery struct{}` | [wasm-runtime/types/queries.go](../../../wasm-runtime/types/queries.go) |
| `go_exported` | `type AllValidatorsResponse struct` | [wasm-runtime/types/queries.go](../../../wasm-runtime/types/queries.go) |
| `go_exported` | `type AnalysisReport struct` | [wasm-runtime/types/types.go](../../../wasm-runtime/types/types.go) |
| `go_exported` | `type BalanceQuery struct` | [wasm-runtime/types/queries.go](../../../wasm-runtime/types/queries.go) |
| `go_exported` | `type BalanceResponse struct` | [wasm-runtime/types/queries.go](../../../wasm-runtime/types/queries.go) |
| `go_exported` | `type BankMsg struct` | [wasm-runtime/types/msg.go](../../../wasm-runtime/types/msg.go) |
| `go_exported` | `type BankQuery struct` | [wasm-runtime/types/queries.go](../../../wasm-runtime/types/queries.go) |
| `go_exported` | `type BlockInfo struct` | [wasm-runtime/types/env.go](../../../wasm-runtime/types/env.go) |
| `go_exported` | `type BondedDenomResponse struct` | [wasm-runtime/types/queries.go](../../../wasm-runtime/types/queries.go) |
| `go_exported` | `type BurnMsg struct` | [wasm-runtime/types/msg.go](../../../wasm-runtime/types/msg.go) |
| `go_exported` | `type ChannelQuery struct` | [wasm-runtime/types/queries.go](../../../wasm-runtime/types/queries.go) |
| `go_exported` | `type ChannelResponse struct` | [wasm-runtime/types/queries.go](../../../wasm-runtime/types/queries.go) |
| `go_exported` | `type Checksum []byte` | [wasm-runtime/types/checksum.go](../../../wasm-runtime/types/checksum.go) |
| `go_exported` | `type ClearAdminMsg struct` | [wasm-runtime/types/msg.go](../../../wasm-runtime/types/msg.go) |
| `go_exported` | `type CloseChannelMsg struct` | [wasm-runtime/types/msg.go](../../../wasm-runtime/types/msg.go) |
| `go_exported` | `type Coin struct` | [wasm-runtime/types/types.go](../../../wasm-runtime/types/types.go) |
| `go_exported` | `type Coins []Coin` | [wasm-runtime/types/types.go](../../../wasm-runtime/types/types.go) |
| `go_exported` | `type ContractInfo struct` | [wasm-runtime/types/env.go](../../../wasm-runtime/types/env.go) |
| `go_exported` | `type ContractInfoQuery struct` | [wasm-runtime/types/queries.go](../../../wasm-runtime/types/queries.go) |
| `go_exported` | `type ContractInfoResponse struct` | [wasm-runtime/types/queries.go](../../../wasm-runtime/types/queries.go) |
| `go_exported` | `type ContractResult struct` | [wasm-runtime/types/msg.go](../../../wasm-runtime/types/msg.go) |
| `go_exported` | `type CosmosMsg struct` | [wasm-runtime/types/msg.go](../../../wasm-runtime/types/msg.go) |
| `go_exported` | `type DelegateMsg struct` | [wasm-runtime/types/msg.go](../../../wasm-runtime/types/msg.go) |
| `go_exported` | `type Delegation struct` | [wasm-runtime/types/queries.go](../../../wasm-runtime/types/queries.go) |
| `go_exported` | `type DelegationQuery struct` | [wasm-runtime/types/queries.go](../../../wasm-runtime/types/queries.go) |
| `go_exported` | `type DelegationResponse struct` | [wasm-runtime/types/queries.go](../../../wasm-runtime/types/queries.go) |
| `go_exported` | `type Delegations []Delegation` | [wasm-runtime/types/queries.go](../../../wasm-runtime/types/queries.go) |
| `go_exported` | `type DistributionMsg struct` | [wasm-runtime/types/msg.go](../../../wasm-runtime/types/msg.go) |
| `go_exported` | `type Env struct` | [wasm-runtime/types/env.go](../../../wasm-runtime/types/env.go) |
| `go_exported` | `type Event struct` | [wasm-runtime/types/msg.go](../../../wasm-runtime/types/msg.go) |
| `go_exported` | `type EventAttribute struct` | [wasm-runtime/types/msg.go](../../../wasm-runtime/types/msg.go) |
| `go_exported` | `type EventAttributes []EventAttribute` | [wasm-runtime/types/msg.go](../../../wasm-runtime/types/msg.go) |
| `go_exported` | `type Events []Event` | [wasm-runtime/types/msg.go](../../../wasm-runtime/types/msg.go) |
| `go_exported` | `type ExecuteMsg struct` | [wasm-runtime/types/msg.go](../../../wasm-runtime/types/msg.go) |
| `go_exported` | `type FullDelegation struct` | [wasm-runtime/types/queries.go](../../../wasm-runtime/types/queries.go) |
| `go_exported` | `type Gas = uint64` | [wasm-runtime/types/gas.go](../../../wasm-runtime/types/gas.go) |
| `go_exported` | `type GasMeter interface` | [wasm-runtime/types/gas.go](../../../wasm-runtime/types/gas.go) |
| `go_exported` | `type GasReport struct` | [wasm-runtime/types/types.go](../../../wasm-runtime/types/types.go) |
| `go_exported` | `type GoAPI struct` | [wasm-runtime/types/api.go](../../../wasm-runtime/types/api.go) |
| `go_exported` | `type GovMsg struct` | [wasm-runtime/types/msg.go](../../../wasm-runtime/types/msg.go) |
| `go_exported` | `type HumanAddress = string` | [wasm-runtime/types/types.go](../../../wasm-runtime/types/types.go) |
| `go_exported` | `type IBC3ChannelOpenResponse struct` | [wasm-runtime/types/ibc.go](../../../wasm-runtime/types/ibc.go) |
| `go_exported` | `type IBCAcknowledgement struct` | [wasm-runtime/types/ibc.go](../../../wasm-runtime/types/ibc.go) |
| `go_exported` | `type IBCBasicResponse struct` | [wasm-runtime/types/ibc.go](../../../wasm-runtime/types/ibc.go) |
| `go_exported` | `type IBCBasicResult struct` | [wasm-runtime/types/ibc.go](../../../wasm-runtime/types/ibc.go) |
| `go_exported` | `type IBCChannel struct` | [wasm-runtime/types/ibc.go](../../../wasm-runtime/types/ibc.go) |
| `go_exported` | `type IBCChannelCloseMsg struct` | [wasm-runtime/types/ibc.go](../../../wasm-runtime/types/ibc.go) |
| `go_exported` | `type IBCChannelConnectMsg struct` | [wasm-runtime/types/ibc.go](../../../wasm-runtime/types/ibc.go) |
| `go_exported` | `type IBCChannelOpenMsg struct` | [wasm-runtime/types/ibc.go](../../../wasm-runtime/types/ibc.go) |
| `go_exported` | `type IBCChannelOpenResult struct` | [wasm-runtime/types/ibc.go](../../../wasm-runtime/types/ibc.go) |
| `go_exported` | `type IBCChannels []IBCChannel` | [wasm-runtime/types/queries.go](../../../wasm-runtime/types/queries.go) |
| `go_exported` | `type IBCCloseConfirm struct` | [wasm-runtime/types/ibc.go](../../../wasm-runtime/types/ibc.go) |
| `go_exported` | `type IBCCloseInit struct` | [wasm-runtime/types/ibc.go](../../../wasm-runtime/types/ibc.go) |
| `go_exported` | `type IBCEndpoint struct` | [wasm-runtime/types/ibc.go](../../../wasm-runtime/types/ibc.go) |
| `go_exported` | `type IBCMsg struct` | [wasm-runtime/types/msg.go](../../../wasm-runtime/types/msg.go) |
| `go_exported` | `type IBCOpenAck struct` | [wasm-runtime/types/ibc.go](../../../wasm-runtime/types/ibc.go) |
| `go_exported` | `type IBCOpenConfirm struct` | [wasm-runtime/types/ibc.go](../../../wasm-runtime/types/ibc.go) |
| `go_exported` | `type IBCOpenInit struct` | [wasm-runtime/types/ibc.go](../../../wasm-runtime/types/ibc.go) |
| `go_exported` | `type IBCOpenTry struct` | [wasm-runtime/types/ibc.go](../../../wasm-runtime/types/ibc.go) |
| `go_exported` | `type IBCOrder = string` | [wasm-runtime/types/ibc.go](../../../wasm-runtime/types/ibc.go) |
| `go_exported` | `type IBCPacket struct` | [wasm-runtime/types/ibc.go](../../../wasm-runtime/types/ibc.go) |
| `go_exported` | `type IBCPacketAckMsg struct` | [wasm-runtime/types/ibc.go](../../../wasm-runtime/types/ibc.go) |
| `go_exported` | `type IBCPacketReceiveMsg struct` | [wasm-runtime/types/ibc.go](../../../wasm-runtime/types/ibc.go) |
| `go_exported` | `type IBCPacketTimeoutMsg struct` | [wasm-runtime/types/ibc.go](../../../wasm-runtime/types/ibc.go) |
| `go_exported` | `type IBCQuery struct` | [wasm-runtime/types/queries.go](../../../wasm-runtime/types/queries.go) |
| `go_exported` | `type IBCReceiveResponse struct` | [wasm-runtime/types/ibc.go](../../../wasm-runtime/types/ibc.go) |
| `go_exported` | `type IBCReceiveResult struct` | [wasm-runtime/types/ibc.go](../../../wasm-runtime/types/ibc.go) |
| `go_exported` | `type IBCTimeout struct` | [wasm-runtime/types/ibc.go](../../../wasm-runtime/types/ibc.go) |
| `go_exported` | `type IBCTimeoutBlock struct` | [wasm-runtime/types/ibc.go](../../../wasm-runtime/types/ibc.go) |
| `go_exported` | `type InstantiateMsg struct` | [wasm-runtime/types/msg.go](../../../wasm-runtime/types/msg.go) |
| `go_exported` | `type Iterator interface` | [wasm-runtime/types/store.go](../../../wasm-runtime/types/store.go) |
| `go_exported` | `type KVStore interface` | [wasm-runtime/types/store.go](../../../wasm-runtime/types/store.go) |
| `go_exported` | `type ListChannelsQuery struct` | [wasm-runtime/types/queries.go](../../../wasm-runtime/types/queries.go) |
| `go_exported` | `type ListChannelsResponse struct` | [wasm-runtime/types/queries.go](../../../wasm-runtime/types/queries.go) |
| `go_exported` | `type MessageInfo struct` | [wasm-runtime/types/env.go](../../../wasm-runtime/types/env.go) |
| `go_exported` | `type Metrics struct` | [wasm-runtime/types/types.go](../../../wasm-runtime/types/types.go) |
| `go_exported` | `type MigrateMsg struct` | [wasm-runtime/types/msg.go](../../../wasm-runtime/types/msg.go) |
| `go_exported` | `func NewCoin(amount uint64, denom string) Coin` | [wasm-runtime/types/types.go](../../../wasm-runtime/types/types.go) |
| `go_exported` | `No` | [wasm-runtime/types/msg.go](../../../wasm-runtime/types/msg.go) |
| `go_exported` | `type NoSuchContract struct` | [wasm-runtime/types/systemerror.go](../../../wasm-runtime/types/systemerror.go) |
| `go_exported` | `NoWithVeto` | [wasm-runtime/types/msg.go](../../../wasm-runtime/types/msg.go) |
| `go_exported` | `type OutOfGasError struct{}` | [wasm-runtime/types/types.go](../../../wasm-runtime/types/types.go) |
| `go_exported` | `type PortIDQuery struct{}` | [wasm-runtime/types/queries.go](../../../wasm-runtime/types/queries.go) |
| `go_exported` | `type PortIDResponse struct` | [wasm-runtime/types/queries.go](../../../wasm-runtime/types/queries.go) |
| `go_exported` | `type Querier interface` | [wasm-runtime/types/queries.go](../../../wasm-runtime/types/queries.go) |
| `go_exported` | `type QueryRequest struct` | [wasm-runtime/types/queries.go](../../../wasm-runtime/types/queries.go) |
| `go_exported` | `type QueryResponse queryResponseImpl` | [wasm-runtime/types/queries.go](../../../wasm-runtime/types/queries.go) |
| `go_exported` | `type RawQuery struct` | [wasm-runtime/types/queries.go](../../../wasm-runtime/types/queries.go) |
| `go_exported` | `type RedelegateMsg struct` | [wasm-runtime/types/msg.go](../../../wasm-runtime/types/msg.go) |
| `go_exported` | `type Reply struct` | [wasm-runtime/types/submessages.go](../../../wasm-runtime/types/submessages.go) |
| `go_exported` | `ReplyAlways` | [wasm-runtime/types/submessages.go](../../../wasm-runtime/types/submessages.go) |
| `go_exported` | `ReplyError` | [wasm-runtime/types/submessages.go](../../../wasm-runtime/types/submessages.go) |
| `go_exported` | `ReplyNever` | [wasm-runtime/types/submessages.go](../../../wasm-runtime/types/submessages.go) |
| `go_exported` | `ReplySuccess` | [wasm-runtime/types/submessages.go](../../../wasm-runtime/types/submessages.go) |
| `go_exported` | `type Response struct` | [wasm-runtime/types/msg.go](../../../wasm-runtime/types/msg.go) |
| `go_exported` | `func RustQuery(querier Querier, binRequest []byte, gasLimit uint64) QuerierResult` | [wasm-runtime/types/queries.go](../../../wasm-runtime/types/queries.go) |
| `go_exported` | `type SendMsg struct` | [wasm-runtime/types/msg.go](../../../wasm-runtime/types/msg.go) |
| `go_exported` | `type SendPacketMsg struct` | [wasm-runtime/types/msg.go](../../../wasm-runtime/types/msg.go) |
| `go_exported` | `type SetWithdrawAddressMsg struct` | [wasm-runtime/types/msg.go](../../../wasm-runtime/types/msg.go) |
| `go_exported` | `type SmartQuery struct` | [wasm-runtime/types/queries.go](../../../wasm-runtime/types/queries.go) |
| `go_exported` | `type StakingMsg struct` | [wasm-runtime/types/msg.go](../../../wasm-runtime/types/msg.go) |
| `go_exported` | `type StakingQuery struct` | [wasm-runtime/types/queries.go](../../../wasm-runtime/types/queries.go) |
| `go_exported` | `type StargateMsg struct` | [wasm-runtime/types/msg.go](../../../wasm-runtime/types/msg.go) |
| `go_exported` | `type StargateQuery struct` | [wasm-runtime/types/queries.go](../../../wasm-runtime/types/queries.go) |
| `go_exported` | `type SubMsg struct` | [wasm-runtime/types/submessages.go](../../../wasm-runtime/types/submessages.go) |
| `go_exported` | `type SubMsgResponse struct` | [wasm-runtime/types/submessages.go](../../../wasm-runtime/types/submessages.go) |
| `go_exported` | `type SubMsgResult struct` | [wasm-runtime/types/submessages.go](../../../wasm-runtime/types/submessages.go) |
| `go_exported` | `func ToSystemError(err error) *SystemError` | [wasm-runtime/types/systemerror.go](../../../wasm-runtime/types/systemerror.go) |
| `go_exported` | `type TransactionInfo struct` | [wasm-runtime/types/env.go](../../../wasm-runtime/types/env.go) |
| `go_exported` | `type TransferMsg struct` | [wasm-runtime/types/msg.go](../../../wasm-runtime/types/msg.go) |
| `go_exported` | `type UFraction struct` | [wasm-runtime/types/fraction.go](../../../wasm-runtime/types/fraction.go) |
| `go_exported` | `type UnbondingDelegationEntry struct` | [wasm-runtime/types/queries.go](../../../wasm-runtime/types/queries.go) |
| `go_exported` | `type UnbondingDelegationResponse struct` | [wasm-runtime/types/queries.go](../../../wasm-runtime/types/queries.go) |
| `go_exported` | `type UnbondingDelegationsResponse struct` | [wasm-runtime/types/queries.go](../../../wasm-runtime/types/queries.go) |
| `go_exported` | `type UndelegateMsg struct` | [wasm-runtime/types/msg.go](../../../wasm-runtime/types/msg.go) |
| `go_exported` | `type Unknown struct{}` | [wasm-runtime/types/systemerror.go](../../../wasm-runtime/types/systemerror.go) |
| `go_exported` | `type UnsupportedRequest struct` | [wasm-runtime/types/systemerror.go](../../../wasm-runtime/types/systemerror.go) |
| `go_exported` | `type UpdateAdminMsg struct` | [wasm-runtime/types/msg.go](../../../wasm-runtime/types/msg.go) |
| `go_exported` | `type Validator struct` | [wasm-runtime/types/queries.go](../../../wasm-runtime/types/queries.go) |
| `go_exported` | `type ValidatorQuery struct` | [wasm-runtime/types/queries.go](../../../wasm-runtime/types/queries.go) |
| `go_exported` | `type ValidatorResponse struct` | [wasm-runtime/types/queries.go](../../../wasm-runtime/types/queries.go) |
| `go_exported` | `type VoteMsg struct` | [wasm-runtime/types/msg.go](../../../wasm-runtime/types/msg.go) |
| `go_exported` | `type WasmMsg struct` | [wasm-runtime/types/msg.go](../../../wasm-runtime/types/msg.go) |
| `go_exported` | `type WasmQuery struct` | [wasm-runtime/types/queries.go](../../../wasm-runtime/types/queries.go) |
| `go_exported` | `type WithdrawDelegatorRewardMsg struct` | [wasm-runtime/types/msg.go](../../../wasm-runtime/types/msg.go) |
| `go_exported` | `Yes` | [wasm-runtime/types/msg.go](../../../wasm-runtime/types/msg.go) |

## vm-api

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `type Checksum = types.Checksum` | [wasm-runtime/lib_no_cgo.go](../../../wasm-runtime/lib_no_cgo.go) |
| `go_exported` | `func DeserializeResponse(gasLimit uint64, deserCost types.UFraction, gasReport *types.GasReport, data []byte, response any) error` | [wasm-runtime/lib.go](../../../wasm-runtime/lib.go) |
| `go_exported` | `type GasMeter = types.GasMeter` | [wasm-runtime/lib_no_cgo.go](../../../wasm-runtime/lib_no_cgo.go) |
| `go_exported` | `type GoAPI = types.GoAPI` | [wasm-runtime/lib_no_cgo.go](../../../wasm-runtime/lib_no_cgo.go) |
| `go_exported` | `type KVStore = types.KVStore` | [wasm-runtime/lib_no_cgo.go](../../../wasm-runtime/lib_no_cgo.go) |
| `go_exported` | `func LibwasmvmVersion() (string, error)` | [wasm-runtime/lib_no_cgo.go](../../../wasm-runtime/lib_no_cgo.go) |
| `go_exported` | `func NewVM(dataDir string, supportedCapabilities string, memoryLimit uint32, printDebug bool, cacheSize uint32) (*VM, error)` | [wasm-runtime/lib.go](../../../wasm-runtime/lib.go) |
| `go_exported` | `type Querier = types.Querier` | [wasm-runtime/lib_no_cgo.go](../../../wasm-runtime/lib_no_cgo.go) |
| `go_exported` | `type VM struct {` | [wasm-runtime/lib.go](../../../wasm-runtime/lib.go) |
| `go_exported` | `func (vm *VM) AnalyzeCode(checksum Checksum) (*types.AnalysisReport, error)` | [wasm-runtime/lib.go](../../../wasm-runtime/lib.go) |
| `go_exported` | `func (vm *VM) Cleanup()` | [wasm-runtime/lib.go](../../../wasm-runtime/lib.go) |
| `go_exported` | `func (vm *VM) Create(code WasmCode) (Checksum, error)` | [wasm-runtime/lib.go](../../../wasm-runtime/lib.go) |
| `go_exported` | `func (vm *VM) Execute(` | [wasm-runtime/lib.go](../../../wasm-runtime/lib.go) |
| `go_exported` | `func (vm *VM) GetCode(checksum Checksum) (WasmCode, error)` | [wasm-runtime/lib.go](../../../wasm-runtime/lib.go) |
| `go_exported` | `func (vm *VM) GetMetrics() (*types.Metrics, error)` | [wasm-runtime/lib.go](../../../wasm-runtime/lib.go) |
| `go_exported` | `func (vm *VM) IBCChannelClose(` | [wasm-runtime/lib.go](../../../wasm-runtime/lib.go) |
| `go_exported` | `func (vm *VM) IBCChannelConnect(` | [wasm-runtime/lib.go](../../../wasm-runtime/lib.go) |
| `go_exported` | `func (vm *VM) IBCChannelOpen(` | [wasm-runtime/lib.go](../../../wasm-runtime/lib.go) |
| `go_exported` | `func (vm *VM) IBCPacketAck(` | [wasm-runtime/lib.go](../../../wasm-runtime/lib.go) |
| `go_exported` | `func (vm *VM) IBCPacketReceive(` | [wasm-runtime/lib.go](../../../wasm-runtime/lib.go) |
| `go_exported` | `func (vm *VM) IBCPacketTimeout(` | [wasm-runtime/lib.go](../../../wasm-runtime/lib.go) |
| `go_exported` | `func (vm *VM) Instantiate(` | [wasm-runtime/lib.go](../../../wasm-runtime/lib.go) |
| `go_exported` | `func (vm *VM) Migrate(` | [wasm-runtime/lib.go](../../../wasm-runtime/lib.go) |
| `go_exported` | `func (vm *VM) Pin(checksum Checksum) error` | [wasm-runtime/lib.go](../../../wasm-runtime/lib.go) |
| `go_exported` | `func (vm *VM) Query(` | [wasm-runtime/lib.go](../../../wasm-runtime/lib.go) |
| `go_exported` | `func (vm *VM) RemoveCode(checksum Checksum) error` | [wasm-runtime/lib.go](../../../wasm-runtime/lib.go) |
| `go_exported` | `func (vm *VM) Reply(` | [wasm-runtime/lib.go](../../../wasm-runtime/lib.go) |
| `go_exported` | `func (vm *VM) StoreCode(code WasmCode) (Checksum, error)` | [wasm-runtime/lib.go](../../../wasm-runtime/lib.go) |
| `go_exported` | `func (vm *VM) StoreCodeUnchecked(code WasmCode) (Checksum, error)` | [wasm-runtime/lib.go](../../../wasm-runtime/lib.go) |
| `go_exported` | `func (vm *VM) Sudo(` | [wasm-runtime/lib.go](../../../wasm-runtime/lib.go) |
| `go_exported` | `func (vm *VM) Unpin(checksum Checksum) error` | [wasm-runtime/lib.go](../../../wasm-runtime/lib.go) |
| `go_exported` | `type WasmCode []byte` | [wasm-runtime/lib_no_cgo.go](../../../wasm-runtime/lib_no_cgo.go) |
| `go_exported` | `func NewVM(dataDir string, supportedCapabilities string, memoryLimit uint32, printDebug bool, cacheSize uint32) (*VM, error)` | [wasm/x/wasm/artifacts/v152/lib.go](../../../wasm/x/wasm/artifacts/v152/lib.go) |
| `go_exported` | `type VM struct {` | [wasm/x/wasm/artifacts/v152/lib.go](../../../wasm/x/wasm/artifacts/v152/lib.go) |
| `go_exported` | `func NewVM(dataDir string, supportedCapabilities string, memoryLimit uint32, printDebug bool, cacheSize uint32) (*VM, error)` | [wasm/x/wasm/artifacts/v155/lib.go](../../../wasm/x/wasm/artifacts/v155/lib.go) |
| `go_exported` | `type VM struct {` | [wasm/x/wasm/artifacts/v155/lib.go](../../../wasm/x/wasm/artifacts/v155/lib.go) |
