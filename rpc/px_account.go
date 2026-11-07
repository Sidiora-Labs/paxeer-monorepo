package evmrpc

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"math/big"
	"regexp"
	"strings"
	"sync"
	"time"

	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/bank"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxprograms"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	storekeys "github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/keys"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/export"
	"github.com/ethereum/go-ethereum/rpc"
)

type pxError struct {
	code    int
	message string
	data    map[string]interface{}
}

func (e *pxError) Error() string          { return e.message }
func (e *pxError) ErrorCode() int         { return e.code }
func (e *pxError) ErrorData() interface{} { return e.data }
func pxUnavailable(code string) error {
	return &pxError{-32001, "Paxeer read unavailable", map[string]interface{}{"code": code}}
}
func pxInvalidParams() error { return &pxError{-32602, "Invalid params", nil} }
func pxHosted(method string) error {
	return &pxError{-32004, "Hosted gateway service operation", map[string]interface{}{"code": "hosted_gateway_service_required", "method": method}}
}

type pxAccount struct {
	EVMAddress    *string `json:"evm_address"`
	PaxAddress    *string `json:"pax_address"`
	LayerXDid     *string `json:"layerx_did"`
	LayerXAccount *string `json:"layerx_account"`
	Bound         bool    `json:"bound"`
}

type pxAccountAPI struct {
	inference      *pxInferenceOption
	state          *StateAPI
	tx             *TransactionAPI
	simulation     *SimulationAPI
	indexer        *pxIndexerOption
	methods        func() []string
	capabilitiesMu sync.Mutex
	capabilities   map[string]interface{}
	probedAt       time.Time
}

var pxAddrABI = mustPxABI(`[
 {"type":"function","name":"getUnifiedAccount","inputs":[{"type":"address"}],"outputs":[{"type":"address"},{"type":"string"},{"type":"bytes32"},{"type":"bytes32"}]},
 {"type":"function","name":"getEvmAddrByLayerX","inputs":[{"type":"bytes32"}],"outputs":[{"type":"address"}]}
]`)

func mustPxABI(document string) abi.ABI {
	parsed, err := abi.JSON(strings.NewReader(document))
	if err != nil {
		panic(err)
	}
	return parsed
}

// Decode only canonical ABI output; in particular a short uint or a non-bool
// word must never turn a failed capability probe into a healthy surface.
func pxDecode(method abi.Method, answer []byte) ([]interface{}, error) {
	values, err := method.Outputs.Unpack(answer)
	if err != nil {
		return nil, err
	}
	canonical, err := method.Outputs.Pack(values...)
	if err != nil || !bytes.Equal(canonical, answer) {
		return nil, errors.New("noncanonical ABI response")
	}
	return values, nil
}
func (a *pxAccountAPI) call(ctx context.Context, height int64, address common.Address, contract abi.ABI, method string, args ...interface{}) ([]interface{}, error) {
	return a.callFrom(ctx, height, common.Address{}, address, contract, method, args...)
}
func (a *pxAccountAPI) callFrom(ctx context.Context, height int64, from, address common.Address, contract abi.ABI, method string, args ...interface{}) ([]interface{}, error) {
	data, err := contract.Pack(method, args...)
	if err != nil {
		return nil, err
	}
	input := hexutil.Bytes(data)
	tag := rpc.BlockNumberOrHashWithNumber(rpc.BlockNumber(height))
	answer, err := a.simulation.Call(ctx, export.TransactionArgs{From: &from, To: &address, Data: &input}, &tag, nil, nil)
	if err != nil {
		return nil, err
	}
	return pxDecode(contract.Methods[method], answer)
}
func (a *pxAccountAPI) height(ctx context.Context) (int64, error) {
	height, err := a.state.watermarks.ResolveHeight(ctx, rpc.BlockNumberOrHashWithNumber(rpc.LatestBlockNumber))
	if err != nil || height < 0 {
		return 0, pxUnavailable("paxeer_state_unavailable")
	}
	if err := CheckVersion(a.state.ctxProvider(height), a.state.keeper); err != nil {
		return 0, pxUnavailable("paxeer_state_unavailable")
	}
	return height, nil
}

type pxSelector struct {
	evm *common.Address
	did *[32]byte
}

func pxParseSelector(value string) (pxSelector, error) {
	if strings.HasPrefix(value, "0x") && len(value) == 42 && common.IsHexAddress(value) {
		address := common.HexToAddress(value)
		return pxSelector{evm: &address}, nil
	}
	explicit := strings.HasPrefix(value, "did:layerx:")
	key := strings.TrimPrefix(value, "did:layerx:")
	if !explicit {
		key = strings.TrimPrefix(key, "0x")
	}
	decoded, err := hex.DecodeString(key)
	if err != nil || len(decoded) != 32 || bytes.Equal(decoded, make([]byte, 32)) {
		return pxSelector{}, pxInvalidParams()
	}
	if !explicit {
		return pxSelector{}, pxUnavailable("native_account_reverse_resolution_unsupported")
	}
	var did [32]byte
	copy(did[:], decoded)
	return pxSelector{did: &did}, nil
}
func pxString(value string) *string { return &value }
func (a *pxAccountAPI) resolve(ctx context.Context, selector pxSelector, height int64) (*pxAccount, error) {
	address := common.Address{}
	if selector.evm != nil {
		address = *selector.evm
	} else {
		// The addr view reverts for a DID without a binding. Read that explicit
		// absence from its owning keeper at the same height so getAccount can
		// return the documented native-only identity, without swallowing a call
		// failure for a DID which actually has a binding.
		if _, bound := a.state.keeper.GetEVMAddressByLayerXDid(a.state.ctxProvider(height), *selector.did); !bound {
			account, err := evmtypes.LayerXMainAccountID(*selector.did)
			if err != nil {
				return nil, pxUnavailable("conflicting_identity_binding")
			}
			return &pxAccount{LayerXDid: pxString(evmtypes.LayerXDid(*selector.did)), LayerXAccount: pxString(hex.EncodeToString(account[:]))}, nil
		}
		values, err := a.call(ctx, height, common.HexToAddress("0x1004"), pxAddrABI, "getEvmAddrByLayerX", *selector.did)
		if err != nil {
			return nil, pxUnavailable("identity_binding_unavailable")
		}
		address = values[0].(common.Address)
		if address == (common.Address{}) {
			return nil, pxUnavailable("conflicting_identity_binding")
		}
	}
	values, err := a.call(ctx, height, common.HexToAddress("0x1004"), pxAddrABI, "getUnifiedAccount", address)
	if err != nil {
		return nil, pxUnavailable("identity_binding_unavailable")
	}
	evm, pax, did, account := values[0].(common.Address), values[1].(string), values[2].([32]byte), values[3].([32]byte)
	bound := did != ([32]byte{})
	if evm != address || bound != (account != ([32]byte{})) || (selector.did != nil && *selector.did != did) {
		return nil, pxUnavailable("conflicting_identity_binding")
	}
	result := &pxAccount{EVMAddress: pxString(strings.ToLower(address.Hex())), Bound: bound}
	if pax != "" {
		result.PaxAddress = &pax
	}
	if bound {
		expected, err := evmtypes.LayerXMainAccountID(did)
		if err != nil || expected != account {
			return nil, pxUnavailable("conflicting_identity_binding")
		}
		reverse, err := a.call(ctx, height, common.HexToAddress("0x1004"), pxAddrABI, "getEvmAddrByLayerX", did)
		if err != nil {
			return nil, pxUnavailable("identity_binding_unavailable")
		}
		if reverse[0].(common.Address) != address {
			return nil, pxUnavailable("conflicting_identity_binding")
		}
		result.LayerXDid = pxString(evmtypes.LayerXDid(did))
		result.LayerXAccount = pxString(hex.EncodeToString(account[:]))
	}
	return result, nil
}
func (a *pxAccountAPI) snapshot(ctx context.Context, value string) (*pxAccount, int64, error) {
	selector, err := pxParseSelector(value)
	if err != nil {
		return nil, 0, err
	}
	height, err := a.height(ctx)
	if err != nil {
		return nil, 0, err
	}
	account, err := a.resolve(ctx, selector, height)
	return account, height, err
}
func (a *pxAccountAPI) ResolveAccount(ctx context.Context, value string) (result *pxAccount, returnErr error) {
	start := time.Now()
	defer func() {
		recordMetricsWithError(ctx, "px_resolveAccount", a.state.connectionType, start, returnErr, recover())
	}()
	result, _, returnErr = a.snapshot(ctx, value)
	if returnErr == nil && result.EVMAddress == nil {
		return nil, pxUnavailable("identity_binding_unavailable")
	}
	return
}
func (a *pxAccountAPI) GetAccount(ctx context.Context, value string) (result map[string]interface{}, returnErr error) {
	start := time.Now()
	defer func() {
		recordMetricsWithError(ctx, "px_getAccount", a.state.connectionType, start, returnErr, recover())
	}()
	account, height, err := a.snapshot(ctx, value)
	if err != nil {
		return nil, err
	}
	result = map[string]interface{}{"account": account, "paxeer": nil}
	if account.EVMAddress == nil {
		return result, nil
	}
	address := common.HexToAddress(*account.EVMAddress)
	tag := rpc.BlockNumberOrHashWithNumber(rpc.BlockNumber(height))
	balance, err := a.state.GetBalance(ctx, address, tag)
	if err != nil {
		return nil, pxUnavailable("paxeer_state_unavailable")
	}
	nonce, err := a.tx.GetTransactionCount(ctx, address, tag)
	if err != nil {
		return nil, pxUnavailable("paxeer_state_unavailable")
	}
	result["paxeer"] = map[string]interface{}{"address": account.EVMAddress, "balance": balance, "nonce": nonce, "height": height}
	return result, nil
}
func (a *pxAccountAPI) GetBalances(ctx context.Context, value string) (result map[string]interface{}, returnErr error) {
	start := time.Now()
	defer func() {
		recordMetricsWithError(ctx, "px_getBalances", a.state.connectionType, start, returnErr, recover())
	}()
	account, height, err := a.snapshot(ctx, value)
	if err != nil {
		return nil, err
	}
	if account.EVMAddress == nil {
		return nil, pxUnavailable("identity_binding_unavailable")
	}
	values, err := a.call(ctx, height, common.HexToAddress("0x1001"), bank.GetABI(), "all_balances", common.HexToAddress(*account.EVMAddress))
	if err != nil {
		return nil, pxUnavailable("invalid_paxeer_response")
	}
	coins := *abi.ConvertType(values[0], new([]struct {
		Amount *big.Int
		Denom  string
	})).(*[]struct {
		Amount *big.Int
		Denom  string
	})
	balances := make([]map[string]string, 0, len(coins))
	for _, coin := range coins {
		balances = append(balances, map[string]string{"denom": coin.Denom, "amount": coin.Amount.String()})
	}
	return map[string]interface{}{"account": account, "height": height, "balances": balances}, nil
}
func (a *pxAccountAPI) GetNetwork(ctx context.Context) (result map[string]interface{}, returnErr error) {
	start := time.Now()
	defer func() {
		recordMetricsWithError(ctx, "px_getNetwork", a.state.connectionType, start, returnErr, recover())
	}()
	// Network discovery still exposes the committed EVM head when consensus
	// Status is unavailable, as required by the optional-consensus contract.
	localWatermarks := *a.state.watermarks
	localWatermarks.tmClient = nil
	height, err := localWatermarks.LatestHeight(ctx)
	if err != nil || height < 0 {
		return nil, pxUnavailable("paxeer_state_unavailable")
	}
	sdkCtx := a.state.ctxProvider(height)
	result = map[string]interface{}{"network_id": sdkCtx.ChainID(), "paxeer": map[string]interface{}{"chain_id": hexutil.EncodeBig(a.state.keeper.ChainID(sdkCtx)), "latest_block": hexutil.EncodeUint64(uint64(height))}, "consensus": nil}
	status, err := a.state.tmClient.Status(ctx)
	if err != nil || status == nil || status.SyncInfo.LatestBlockHeight < 0 || status.NodeInfo.Network != sdkCtx.ChainID() {
		result["reason"] = "consensus_unavailable"
		return result, nil
	}
	result["consensus"] = map[string]interface{}{"latest_height": new(big.Int).SetInt64(status.SyncInfo.LatestBlockHeight).String(), "latest_block_hash": status.SyncInfo.LatestBlockHash.String(), "latest_app_hash": status.SyncInfo.LatestAppHash.String(), "catching_up": status.SyncInfo.CatchingUp}
	return result, nil
}

var pxFundsDID = regexp.MustCompile(`^did:[0-9A-Za-z._-]+:[0-9A-Za-z._:-]+$`)

func pxLowerHex(value string, length int) bool {
	_, err := hex.DecodeString(value)
	return len(value) == length*2 && err == nil && strings.ToLower(value) == value
}
func (a *pxAccountAPI) Register(ctx context.Context, signer, signature string) (result interface{}, returnErr error) {
	start := time.Now()
	defer func() {
		recordMetricsWithError(ctx, "px_register", a.state.connectionType, start, returnErr, recover())
	}()
	if !pxLowerHex(signer, 32) || !pxLowerHex(signature, 64) {
		return nil, pxInvalidParams()
	}
	return nil, pxHosted("px_register")
}
func (a *pxAccountAPI) RequestFunds(ctx context.Context, did, signer string) (result interface{}, returnErr error) {
	start := time.Now()
	defer func() {
		recordMetricsWithError(ctx, "px_requestFunds", a.state.connectionType, start, returnErr, recover())
	}()
	if len(did) > 512 || !pxFundsDID.MatchString(did) || !pxLowerHex(signer, 32) || signer == strings.Repeat("0", 64) {
		return nil, pxInvalidParams()
	}
	return nil, pxHosted("px_requestFunds")
}

// programRead executes the F11 selector as a native, read-only program envelope.
// Every nested call is pinned to the height resolved once by the public method.
var pxProgramABI = mustPxABI(`[
 {"type":"function","name":"getStateHead","inputs":[{"type":"bytes32"}],"outputs":[{"type":"bytes"}]},
 {"type":"function","name":"simulateCall","inputs":[{"type":"bytes32"},{"type":"uint16"},{"type":"string"},{"type":"bytes"},{"type":"bytes"},{"type":"bytes"},{"type":"uint32"},{"type":"uint64[7]"},{"type":"uint128"}],"outputs":[{"type":"int32"},{"type":"uint8"},{"type":"bytes"}]}
]`)

func (a *pxAccountAPI) programRead(ctx context.Context, height int64, selector uint16, payload []byte) ([]byte, error) {
	if a.inference == nil {
		return nil, pxUnavailable("inference_program_not_configured")
	}
	cfg := a.inference
	sdkCtx := a.state.ctxProvider(height)
	did, bound := a.state.keeper.GetLayerXDid(sdkCtx, cfg.reader)
	if !bound {
		return nil, pxUnavailable("inference_reader_unbound")
	}
	actor, err := lxphost.DIDID([]byte(evmtypes.LayerXDid(did)))
	if err != nil {
		return nil, pxUnavailable("inference_reader_unbound")
	}
	chainID := a.state.keeper.ChainID(sdkCtx)
	if !chainID.IsUint64() {
		return nil, pxUnavailable("invalid_inference_response")
	}
	chainBytes := binary.BigEndian.AppendUint64([]byte("PAXAI/chain-domain/v1"), chainID.Uint64())
	chain := sha256.Sum256(chainBytes)
	market := sha256.Sum256(append(append([]byte("PAXAI/market/v1\x00"), chain[:]...), cfg.program[:]...))
	envelope := append([]byte("PAXAI1\x00\x01"), byte(selector>>8), byte(selector))
	envelope = append(envelope, chain[:]...)
	envelope = append(envelope, cfg.program[:]...)
	envelope = append(envelope, market[:]...)
	envelope = append(envelope, actor[:]...)
	envelope = append(envelope, make([]byte, 56)...)
	envelope = binary.BigEndian.AppendUint64(envelope, ^uint64(0))
	request := sha256.Sum256(append(binary.BigEndian.AppendUint64(nil, uint64(height)), envelope...))
	envelope = append(envelope, request[:]...)
	envelope = binary.BigEndian.AppendUint32(envelope, uint32(len(payload)))
	envelope = append(envelope, payload...)
	requestDigest := sha256.Sum256(append([]byte("PAXAI/request/v1\x00"), envelope...))
	envelope = append(envelope, 0)
	budget := [7]uint64{1000000, 16777216, 1048576, 1048576, 64, 1048576, 4096}
	values, err := a.callFrom(ctx, height, cfg.reader, common.HexToAddress("0x101d"), pxProgramABI, "simulateCall", cfg.program, uint16(5), "layerx_call", envelope, []byte{0, 1, 7}, []byte("LayerX/programs/access-declaration/v1\x00\x00"), uint32(16384), budget, new(big.Int).SetUint64(^uint64(0)))
	if err != nil {
		return nil, pxUnavailable("inference_program_unavailable")
	}
	encoded, err := pxProgramABI.Methods["simulateCall"].Outputs.Pack(values...)
	if err != nil {
		return nil, pxUnavailable("invalid_inference_response")
	}
	execution, err := layerxprograms.DecodeProgramExecutionResult(encoded, 16384, [32]byte{})
	if err != nil || execution.TerminalResult != 0 || execution.TerminalKind != 1 || len(execution.AppliedLegs) != 0 {
		return nil, pxUnavailable("inference_program_unavailable")
	}
	answer := execution.GuestResponse
	if len(answer) < 82 || binary.BigEndian.Uint16(answer[:2]) != 1 || binary.BigEndian.Uint16(answer[2:4]) != 0 || binary.BigEndian.Uint16(answer[4:6]) != 0 || !bytes.Equal(answer[6:38], requestDigest[:]) || int(binary.BigEndian.Uint32(answer[78:82])) != len(answer)-82 {
		return nil, pxUnavailable("invalid_inference_response")
	}
	digest := sha256.Sum256(append([]byte("PAXAI/result/v1\x00"), answer[82:]...))
	if !bytes.Equal(digest[:], answer[46:78]) {
		return nil, pxUnavailable("invalid_inference_response")
	}
	return bytes.Clone(answer[82:]), nil
}

// programSections resolves the content-addressed program manifest and shared state
// at the same committed height used by programRead. It is used only for index discovery.
func (a *pxAccountAPI) programSections(ctx context.Context, height int64) ([6][]byte, error) {
	var sections [6][]byte
	invalid := pxUnavailable("invalid_inference_response")
	if a.inference == nil {
		return sections, pxUnavailable("inference_program_not_configured")
	}
	values, err := a.call(ctx, height, common.HexToAddress("0x101d"), pxProgramABI, "getStateHead", a.inference.program)
	if err != nil {
		return sections, pxUnavailable("inference_program_unavailable")
	}
	head := values[0].([]byte)
	if len(head) != 38 || binary.BigEndian.Uint16(head[:2]) != 1 {
		return sections, invalid
	}
	sdkCtx := a.state.ctxProvider(height)
	var key sdk.StoreKey
	for _, candidate := range sdkCtx.MultiStore().StoreKeys() {
		if candidate.Name() == storekeys.LayerXHostStoreKey {
			key = candidate
			break
		}
	}
	if key == nil {
		return sections, pxUnavailable("inference_program_unavailable")
	}
	store := sdkCtx.KVStore(key)
	blob := func(hash []byte) ([]byte, error) {
		b := store.Get(append([]byte{5, 0, 9}, hash...))
		digest := sha256.Sum256(b)
		if !bytes.Equal(digest[:], hash) {
			return nil, invalid
		}
		return b, nil
	}
	manifest, err := blob(head[6:])
	if err != nil {
		return sections, err
	}
	r := pxFixed{data: manifest}
	if r.number(2) != 1 {
		return sections, invalid
	}
	count := r.number(4)
	if count > 65536 || count != uint64(binary.BigEndian.Uint32(head[2:6])) {
		return sections, invalid
	}
	var state []byte
	var previous []byte
	for i := uint64(0); i < count; i++ {
		n := r.number(2)
		if n == 0 || n > 128 {
			return sections, invalid
		}
		name := r.take(int(n))
		hash := r.take(32)
		size := r.number(4)
		if r.bad || i > 0 && bytes.Compare(previous, name) >= 0 {
			return sections, invalid
		}
		previous = name
		if string(name) == "paxai/state/v1" {
			if size > 196608 {
				return sections, invalid
			}
			state, err = blob(hash)
			if err != nil || uint64(len(state)) != size {
				return sections, invalid
			}
		}
	}
	if r.end() != nil || len(state) < 16 || string(state[:6]) != "PAXAS1" || binary.BigEndian.Uint16(state[6:8]) != 1 {
		return sections, invalid
	}
	r = pxFixed{data: state[16:]}
	caps := [6]int{16384, 24576, 24576, 81920, 24576, 24576}
	for i := range sections {
		if r.number(2) != uint64(i+1) || r.number(2) != 0 {
			return sections, invalid
		}
		n := r.number(4)
		cap := caps[i] - 8
		if i == 5 {
			cap -= 16
		}
		if n > uint64(cap) || n > uint64(len(r.data)) {
			return sections, invalid
		}
		sections[i] = bytes.Clone(r.take(int(n)))
	}
	return sections, r.end()
}
