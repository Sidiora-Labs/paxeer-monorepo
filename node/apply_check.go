package app

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmconfig "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	tmcrypto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/version"
	gigaconfig "github.com/Sidiora-Labs/Paxeer-X-Network/engine/executor/config"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types/ethtx"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server"
	servertypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	upgradetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/spf13/viper"
	dbm "github.com/tendermint/tm-db"
	"golang.org/x/sys/unix"
)

const ApplyCheckMarker = "applycheck-copy"

// ApplyCheckSequencer supplies the execution-on-arrival path without making the
// application depend on the sequencer package that hosts it.
type ApplyCheckSequencer interface {
	BuildBlock(*App, *abci.RequestFinalizeBlock, string, servertypes.AppOptions) error
	Execute(context.Context, *App, *abci.RequestFinalizeBlock) (*abci.ResponseFinalizeBlock, error)
}

// ApplyCheckStatePreparer installs any temporary state needed by an offline
// execution form before its block is built.
type ApplyCheckStatePreparer interface {
	PrepareApplyCheckState(*App, int64) error
}

// ApplyCheckOptions selects the credentials used by ApplyCheckHome.
type ApplyCheckOptions struct {
	StandInReceiptKey bool
}

type ApplyCheckReport struct {
	ChainID          string               `json:"chain_id"`
	Height           int64                `json:"height"`
	Time             time.Time            `json:"time"`
	ReceiptKeyMode   string               `json:"receipt_key_mode"`
	CommittedHeight  int64                `json:"committed_height"`
	CommittedAppHash string               `json:"committed_app_hash"`
	Forms            []ApplyCheckForm     `json:"forms"`
	Comparison       ApplyCheckComparison `json:"comparison"`
}

type ApplyCheckForm struct {
	Name             string             `json:"name"`
	EngineLibrary    string             `json:"engine_library,omitempty"`
	Results          []ApplyCheckResult `json:"results"`
	AppHash          string             `json:"app_hash"`
	Panic            string             `json:"panic,omitempty"`
	Error            string             `json:"error,omitempty"`
	ErrorTransaction int                `json:"error_transaction,omitempty"`
}

type ApplyCheckTransactionError struct {
	Transaction int
	Err         error
}

func (e *ApplyCheckTransactionError) Error() string {
	return fmt.Sprintf("transaction %d admission: %v", e.Transaction, e.Err)
}
func (e *ApplyCheckTransactionError) Unwrap() error { return e.Err }

type ApplyCheckPanicError struct{ Message string }

func (e *ApplyCheckPanicError) Error() string { return e.Message }

type ApplyCheckResult struct {
	Code            uint32            `json:"code"`
	Codespace       string            `json:"codespace"`
	GasWanted       int64             `json:"gas_wanted"`
	GasUsed         int64             `json:"gas_used"`
	Events          []abci.Event      `json:"events"`
	Status          *uint32           `json:"status,omitempty"`
	Logs            []*evmtypes.Log   `json:"logs,omitempty"`
	ContractAddress string            `json:"contract_address,omitempty"`
	Receipt         *evmtypes.Receipt `json:"receipt,omitempty"`
	ReceiptError    string            `json:"receipt_error,omitempty"`
}

type ApplyCheckComparison struct {
	Equal     bool   `json:"equal"`
	Form      string `json:"form,omitempty"`
	Reference string `json:"reference,omitempty"`
	// Transaction is one-based in the supplied list, excluding a sequence batch.
	Transaction int    `json:"transaction,omitempty"`
	Field       string `json:"field,omitempty"`
}

// DecodeApplyCheckTransaction accepts a signed Ethereum transaction in hex, or
// the protobuf bytes of a signed chain transaction in base64.
func DecodeApplyCheckTransaction(line string) ([]byte, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil, errors.New("empty transaction")
	}
	raw, hexErr := hex.DecodeString(strings.TrimPrefix(strings.TrimPrefix(line, "0x"), "0X"))
	var ethereumTx ethtypes.Transaction
	if hexErr == nil && ethereumTx.UnmarshalBinary(raw) == nil {
		if _, err := ethtypes.Sender(ethtypes.LatestSignerForChainID(ethereumTx.ChainId()), &ethereumTx); err != nil {
			return nil, fmt.Errorf("invalid Ethereum signature: %w", err)
		}
		data, err := ethtx.NewTxDataFromTx(&ethereumTx)
		if err != nil {
			return nil, err
		}
		msg, err := evmtypes.NewMsgEVMTransaction(data)
		if err != nil {
			return nil, err
		}
		config := MakeEncodingConfig().TxConfig
		builder := config.NewTxBuilder()
		if err := builder.SetMsgs(msg); err != nil {
			return nil, err
		}
		builder.SetGasLimit(ethereumTx.Gas())
		return config.TxEncoder()(builder.GetTx())
	}
	raw, err := base64.StdEncoding.DecodeString(line)
	if err != nil {
		return nil, errors.New("transaction must be signed Ethereum hex or chain base64")
	}
	if _, err := MakeEncodingConfig().TxConfig.TxDecoder()(raw); err != nil {
		return nil, fmt.Errorf("invalid chain transaction: %w", err)
	}
	return raw, nil
}

// ApplyCheckHome runs all forms on independent disposable copies. It neither
// opens the source application nor calls Commit on any of the applications.
func ApplyCheckHome(ctx context.Context, home string, txs [][]byte, at time.Time, sequence ApplyCheckSequencer) (*ApplyCheckReport, error) {
	return ApplyCheckHomeWithOptions(ctx, home, txs, at, sequence, ApplyCheckOptions{})
}

// ApplyCheckHomeWithOptions runs the comparison with the requested receipt
// key mode. Stand-in keys are kept in memory and are installed only in the
// disposable application copies.
func ApplyCheckHomeWithOptions(ctx context.Context, home string, txs [][]byte, at time.Time, sequence ApplyCheckSequencer, applyOptions ApplyCheckOptions) (*ApplyCheckReport, error) {
	if len(txs) == 0 {
		return nil, errors.New("applycheck requires at least one transaction")
	}
	if sequence == nil {
		return nil, errors.New("applycheck requires a sequencer executor")
	}
	home, err := filepath.Abs(home)
	if err != nil {
		return nil, err
	}
	home, err = filepath.EvalSymlinks(home)
	if err != nil {
		return nil, err
	}
	marker, err := os.Lstat(filepath.Join(home, ApplyCheckMarker))
	if err != nil || !marker.Mode().IsRegular() {
		return nil, errors.New("home must contain the regular marker file applycheck-copy")
	}
	unlock, err := lockApplyCheckHome(home)
	if err != nil {
		return nil, err
	}
	defer unlock()
	temporary, err := os.MkdirTemp("", "paxeer-applycheck-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(temporary)
	baseline := filepath.Join(temporary, "baseline")
	if err := copyApplyCheckHome(ctx, home, baseline); err != nil {
		return nil, err
	}
	var report *ApplyCheckReport
	var request *abci.RequestFinalizeBlock
	for _, name := range []string{"sequencer", "follower_fast", "follower_ordinary"} {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		branch := filepath.Join(temporary, name)
		if err := copyApplyCheckHome(ctx, baseline, branch); err != nil {
			return nil, err
		}
		options, err := applyCheckOptions(branch, home)
		if err != nil {
			return nil, err
		}
		a, err := openApplyCheckApp(branch, options, name == "follower_fast")
		if err != nil {
			return nil, err
		}
		if report == nil {
			request, err = buildApplyCheckBlock(a, txs, at)
			if err == nil {
				if applyOptions.StandInReceiptKey {
					preparer, ok := sequence.(ApplyCheckStatePreparer)
					if !ok {
						_ = a.Close()
						return nil, errors.New("stand-in receipt key is unavailable")
					}
					err = preparer.PrepareApplyCheckState(a, request.Header.Height)
				}
			}
			if err == nil {
				err = sequence.BuildBlock(a, request, branch, options)
			}
			if err != nil {
				_ = a.Close()
				return nil, err
			}
			report = &ApplyCheckReport{ChainID: request.Header.ChainID, Height: request.Header.Height, Time: request.Header.Time,
				ReceiptKeyMode:  map[bool]string{true: "stand-in", false: "registered"}[applyOptions.StandInReceiptKey],
				CommittedHeight: a.LastBlockHeight(), CommittedAppHash: hex.EncodeToString(a.LastCommitID().Hash)}
			setApplyCheckBlockHash(request)
		} else if applyOptions.StandInReceiptKey {
			preparer, ok := sequence.(ApplyCheckStatePreparer)
			if !ok {
				_ = a.Close()
				return nil, errors.New("stand-in receipt key is unavailable")
			}
			if err := preparer.PrepareApplyCheckState(a, request.Header.Height); err != nil {
				_ = a.Close()
				return nil, err
			}
		}
		if a.LastBlockHeight() != report.CommittedHeight || hex.EncodeToString(a.LastCommitID().Hash) != report.CommittedAppHash {
			_ = a.Close()
			return nil, errors.New("application copies have different committed states")
		}
		form := runApplyCheckForm(ctx, a, request, txs, name, sequence)
		if err := a.Close(); err != nil {
			return nil, fmt.Errorf("close %s: %w", name, err)
		}
		report.Forms = append(report.Forms, form)
	}
	report.Comparison = CompareApplyCheck(report.Forms)
	return report, nil
}

func applyCheckOptions(home, source string) (*viper.Viper, error) {
	v := viper.New()
	v.SetDefault(FlagSCEnable, true)
	v.SetDefault(FlagSSEnable, true)
	v.SetDefault(FlagSSBackend, "pebbledb")
	v.SetConfigFile(filepath.Join(home, "config", "app.toml"))
	if err := v.ReadInConfig(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	genesis, err := os.ReadFile(filepath.Join(home, "config", "genesis.json"))
	if err != nil {
		return nil, errors.New("home must contain config/genesis.json")
	}
	var document struct {
		ChainID string `json:"chain_id"`
	}
	if err := json.Unmarshal(genesis, &document); err != nil {
		return nil, err
	}
	if document.ChainID == "" {
		return nil, errors.New("genesis has no chain id")
	}
	v.Set("chain-id", document.ChainID)
	for _, key := range []string{FlagSCDirectory, FlagSSDirectory, FlagEVMSSDirectory, receiptStoreDBDirectoryKey, "sequencer.receipt-key-file"} {
		path := v.GetString(key)
		if path == "" {
			continue
		}
		if !filepath.IsAbs(path) {
			path = filepath.Join(source, path)
		}
		relative, err := filepath.Rel(source, path)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return nil, fmt.Errorf("%s must refer to a path inside the copied home", key)
		}
		v.Set(key, filepath.Join(home, relative))
	}
	v.Set(FlagSCSnapshotInterval, 0)
	v.Set("state-sync.snapshot-interval", 0)
	v.Set("evm.http_enabled", false)
	v.Set("evm.ws_enabled", false)
	return v, nil
}

func openApplyCheckApp(home string, options *viper.Viper, fast bool) (a *App, err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("open application: %v", p)
		}
	}()
	options.Set(gigaconfig.FlagEnabled, fast)
	options.Set(gigaconfig.FlagOCCEnabled, false)
	a = New(dbm.NewMemDB(), nil, true, map[int64]bool{}, home, 0, true,
		tmconfig.DefaultConfig().SetRoot(home), MakeEncodingConfig(), wasm.EnableAllProposals, options, EmptyWasmOpts, nil,
		baseapp.SetMinGasPrices(options.GetString(server.FlagMinGasPrices)),
		func(ba *baseapp.BaseApp) { ba.SetOccEnabled(false); ba.SetConcurrencyWorkers(1) })
	return a, nil
}

func buildApplyCheckBlock(a *App, txs [][]byte, at time.Time) (*abci.RequestFinalizeBlock, error) {
	height := a.LastBlockHeight()
	if height < 1 {
		return nil, errors.New("home has no committed block")
	}
	ctx := a.NewUncachedContext(true, tmproto.Header{Height: height})
	history, found := a.StakingKeeper.GetHistoricalInfo(ctx, height)
	if !found {
		return nil, errors.New("committed block header is unavailable")
	}
	if at.IsZero() {
		at = history.Header.Time.Add(time.Second)
	}
	if !at.After(history.Header.Time) || at.UnixMilli() <= 0 {
		return nil, errors.New("block time must be after the last committed block")
	}
	validators := a.StakingKeeper.GetLastValidators(ctx)
	if len(validators) == 0 {
		return nil, errors.New("committed state has no validators")
	}
	var votes []abci.VoteInfo
	var set []*tmtypes.Validator
	for _, validator := range validators {
		public, err := validator.TmConsPublicKey()
		if err != nil {
			return nil, err
		}
		key, err := tmcrypto.PubKeyFromProto(public)
		if err != nil {
			return nil, err
		}
		power := a.StakingKeeper.GetLastValidatorPower(ctx, validator.GetOperator())
		val := tmtypes.NewValidator(key, power)
		set = append(set, val)
		votes = append(votes, abci.VoteInfo{Validator: abci.Validator{Address: val.Address, Power: power}, SignedLastBlock: true})
	}
	validatorSet := tmtypes.NewValidatorSet(set)
	proposer := validatorSet.GetProposer().Address
	header := history.Header
	header.Version = version.Consensus{Block: version.BlockProtocol}.ToProto()
	if params := a.GetConsensusParams(ctx); params != nil && params.Version != nil {
		header.Version.App = params.Version.AppVersion
	}
	header.Height, header.Time = height+1, at.UTC()
	header.ProposerAddress, header.AppHash = bytes.Clone(proposer), bytes.Clone(a.LastCommitID().Hash)
	header.LastBlockId = a.LastBlockID()
	header.ValidatorsHash, header.NextValidatorsHash = validatorSet.Hash(), validatorSet.Hash()
	header.DataHash, header.LastCommitHash = nil, nil
	return &abci.RequestFinalizeBlock{Header: &header, Txs: cloneApplyCheckTxs(txs), DecidedLastCommit: abci.CommitInfo{Votes: votes}}, nil
}

func setApplyCheckBlockHash(req *abci.RequestFinalizeBlock) {
	transactions := make(tmtypes.Txs, len(req.Txs))
	for i, tx := range req.Txs {
		transactions[i] = tx
	}
	req.Header.DataHash = transactions.Hash()
	header, err := tmtypes.HeaderFromProto(req.Header)
	if err != nil {
		panic(err)
	}
	req.Hash = header.Hash()
}

func cloneApplyCheckTxs(txs [][]byte) [][]byte {
	cloned := make([][]byte, len(txs))
	for i := range txs {
		cloned[i] = bytes.Clone(txs[i])
	}
	return cloned
}

func runApplyCheckForm(ctx context.Context, a *App, block *abci.RequestFinalizeBlock, txs [][]byte, name string, sequence ApplyCheckSequencer) (form ApplyCheckForm) {
	form.Name = name
	if name == "follower_fast" {
		form.EngineLibrary = "absent"
		if a.GigaEvmKeeper.EvmoneVM != nil {
			form.EngineLibrary = "loaded"
		}
	}
	defer func() {
		if p := recover(); p != nil {
			form.Panic = fmt.Sprint(p)
		}
	}()
	header := *block.Header
	req := *block
	req.Header, req.Txs = &header, cloneApplyCheckTxs(block.Txs)
	var response *abci.ResponseFinalizeBlock
	var err error
	if name == "sequencer" {
		response, err = sequence.Execute(ctx, a, &req)
	} else {
		response, err = a.FinalizeBlock(ctx, &req)
	}
	if err != nil {
		var panicError *ApplyCheckPanicError
		if errors.As(err, &panicError) {
			form.Panic = panicError.Message
			return
		}
		if strings.HasPrefix(err.Error(), "ProcessBlock panic:") {
			form.Panic = err.Error()
			return
		}
		form.Error = err.Error()
		var transactionError *ApplyCheckTransactionError
		if errors.As(err, &transactionError) {
			form.ErrorTransaction = transactionError.Transaction
		}
		return
	}
	if response == nil {
		form.Error = "application returned no block result"
		return
	}
	if len(response.AppHash) == 0 {
		form.Error = "application returned no application hash"
		return
	}
	form.AppHash = hex.EncodeToString(response.AppHash)
	offset := len(req.Txs) - len(txs)
	if len(response.TxResults) != len(req.Txs) {
		form.Error = "application returned an incorrect transaction count"
		return
	}
	state := a.NewUncachedContext(false, header)
	for i, tx := range txs {
		result := response.TxResults[i+offset]
		if result == nil {
			form.Error = fmt.Sprintf("transaction %d has no result", i+1)
			return
		}
		out := ApplyCheckResult{Code: result.Code, Codespace: result.Codespace, GasWanted: result.GasWanted, GasUsed: result.GasUsed, Events: result.Events}
		typed, decodeErr := a.GetTxConfig().TxDecoder()(tx)
		if decodeErr == nil {
			if msg := a.GetEVMMsg(typed); msg != nil && !msg.IsAssociateTx() {
				ethereumTx, _ := msg.AsTransaction()
				receipt, receiptErr := a.EvmKeeper.GetTransientReceipt(state, ethereumTx.Hash(), uint64(i+offset))
				if receiptErr != nil {
					out.ReceiptError = receiptErr.Error()
				} else {
					out.Receipt, out.Status, out.Logs, out.ContractAddress = receipt, &receipt.Status, receipt.Logs, receipt.ContractAddress
				}
			}
		}
		form.Results = append(form.Results, out)
	}
	return
}

// FinalizeApplyCheckLayer applies the ordinary end-of-block and hash steps to
// an execution-on-arrival layer on a home where sequencing is not yet active.
func (a *App) FinalizeApplyCheckLayer(ctx context.Context, state sdk.Context, req *abci.RequestFinalizeBlock, results []*abci.ExecTxResult, messages []*evmtypes.MsgEVMTransaction) (*abci.ResponseFinalizeBlock, error) {
	state = state.WithBlockHeader(*req.Header).WithHeaderHash(req.Hash)
	events, end, err := a.CompleteBlock(state, req.Header.Height, results, messages)
	if err != nil {
		return nil, err
	}
	ms, ok := state.MultiStore().(sdk.CacheMultiStore)
	if !ok {
		return nil, errors.New("execution layer has no cache multistore")
	}
	if err := a.SetSealedStateToCommit(state, ms); err != nil {
		return nil, err
	}
	if err := a.finalizeProgramsOccupancy(state); err != nil {
		return nil, err
	}
	cms := a.WriteState()
	if err := a.LightInvarianceChecks(ctx, cms, a.lightInvarianceConfig); err != nil {
		return nil, err
	}
	id, err := (&tmproto.BlockID{Hash: req.Hash}).Marshal()
	if err != nil {
		return nil, err
	}
	a.NewUncachedContext(false, *req.Header).KVStore(a.GetKey(upgradetypes.StoreKey)).Set(committedBlockIdentityKey, id)
	return &abci.ResponseFinalizeBlock{TxResults: results, Events: events, ValidatorUpdates: end.ValidatorUpdates, AppHash: a.GetWorkingHash()}, nil
}

// CompareApplyCheck returns the first differing transaction and field. A panic
// or execution error is always a difference, even if every form fails alike.
func CompareApplyCheck(forms []ApplyCheckForm) ApplyCheckComparison {
	if len(forms) != 3 {
		return ApplyCheckComparison{Field: "forms"}
	}
	for _, form := range forms {
		if form.Panic != "" {
			return ApplyCheckComparison{Form: form.Name, Field: "panic"}
		}
		if form.Error != "" {
			return ApplyCheckComparison{Form: form.Name, Field: "error", Transaction: form.ErrorTransaction}
		}
	}
	ref := forms[0]
	for i := range ref.Results {
		for _, form := range forms[1:] {
			comparison := ApplyCheckComparison{Form: form.Name, Reference: ref.Name, Transaction: i + 1}
			if i >= len(form.Results) {
				comparison.Field = "results.length"
				return comparison
			}
			if field := applyCheckDifference(ref.Results[i], form.Results[i], ""); field != "" {
				comparison.Field = field
				return comparison
			}
		}
	}
	for _, form := range forms[1:] {
		if len(form.Results) != len(ref.Results) {
			return ApplyCheckComparison{Form: form.Name, Reference: ref.Name, Field: "results.length"}
		}
		if form.AppHash != ref.AppHash {
			return ApplyCheckComparison{Form: form.Name, Reference: ref.Name, Field: "app_hash"}
		}
	}
	return ApplyCheckComparison{Equal: true}
}

func applyCheckDifference(left, right interface{}, path string) string {
	if reflect.DeepEqual(left, right) {
		return ""
	}
	l, r := reflect.ValueOf(left), reflect.ValueOf(right)
	if l.IsValid() && r.IsValid() && l.Type() == r.Type() {
		switch l.Kind() {
		case reflect.Ptr:
			if !l.IsNil() && !r.IsNil() {
				return applyCheckDifference(l.Elem().Interface(), r.Elem().Interface(), path)
			}
		case reflect.Slice, reflect.Array:
			if l.Len() != r.Len() {
				return path + ".length"
			}
			for i := 0; i < l.Len(); i++ {
				if different := applyCheckDifference(l.Index(i).Interface(), r.Index(i).Interface(), fmt.Sprintf("%s[%d]", path, i)); different != "" {
					return different
				}
			}
		}
	}
	if l.IsValid() && r.IsValid() && l.Type() == r.Type() && l.Kind() == reflect.Struct {
		for i := 0; i < l.NumField(); i++ {
			field := l.Type().Field(i)
			name := strings.Split(field.Tag.Get("json"), ",")[0]
			if name == "" {
				name = field.Name
			}
			if path != "" {
				name = path + "." + name
			}
			if different := applyCheckDifference(l.Field(i).Interface(), r.Field(i).Interface(), name); different != "" {
				return different
			}
		}
	}
	return path
}

func lockApplyCheckHome(home string) (func(), error) {
	var held []*os.File
	release := func() {
		for _, file := range held {
			_ = file.Close()
		}
	}
	err := filepath.WalkDir(home, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		name := entry.Name()
		if entry.IsDir() || !(name == "LOCK" || strings.HasSuffix(name, "-LOCK") || strings.HasSuffix(name, ".lock") || strings.HasSuffix(name, ".lock.guard")) {
			return nil
		}
		file, err := os.OpenFile(path, os.O_RDWR, 0)
		if err != nil {
			return err
		}
		held = append(held, file)
		if err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
			return errors.New("home stores are held by another process")
		}
		lock := unix.Flock_t{Type: unix.F_WRLCK, Whence: int16(io.SeekStart), Start: 0, Len: 0}
		if err := unix.FcntlFlock(file.Fd(), unix.F_SETLK, &lock); err != nil {
			return errors.New("home stores are held by another process")
		}
		return nil
	})
	if err != nil {
		release()
		return nil, err
	}
	return release, nil
}

func copyApplyCheckHome(ctx context.Context, source, destination string) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o700)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			resolved, err := filepath.EvalSymlinks(path)
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(source, resolved)
			if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				return errors.New("home contains a symlink outside the copied home")
			}
			if filepath.IsAbs(link) {
				link = filepath.Join(destination, rel)
			}
			return os.Symlink(link, target)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("home contains a non-regular file: %s", relative)
		}
		input, err := os.Open(path)
		if err != nil {
			return err
		}
		defer input.Close()
		output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, info.Mode().Perm())
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(output, input)
		return errors.Join(copyErr, output.Close())
	})
}
