package keeper

import (
	"bytes"
	"encoding/binary"
	"math"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"

	"github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm/types"
)

// ValidatorSetSource is a subset of the staking keeper
type ValidatorSetSource interface {
	ApplyAndReturnValidatorSetUpdates(sdk.Context) (updates []abci.ValidatorUpdate, err error)
}

// InitGenesis sets supply information for genesis.
//
// CONTRACT: all types of accounts must have been already initialized/created
func InitGenesis(ctx sdk.Context, keeper *Keeper, data types.GenesisState, stakingKeeper ValidatorSetSource, msgHandler sdk.Handler) ([]abci.ValidatorUpdate, error) {
	if handled, err := importGenesisStreamControl(ctx, keeper, data); handled {
		return nil, err
	}
	contractKeeper := NewGovPermissionKeeper(keeper)
	keeper.SetParams(ctx, data.Params)
	var maxCodeID uint64
	for i, code := range data.Codes {
		err := keeper.ImportCode(ctx, code.CodeID, code.CodeInfo, code.CodeBytes)
		if err != nil {
			return nil, sdkerrors.Wrapf(err, "code %d with id: %d", i, code.CodeID)
		}
		if code.CodeID > maxCodeID {
			maxCodeID = code.CodeID
		}
		if code.Pinned {
			if err := contractKeeper.PinCode(ctx, code.CodeID); err != nil {
				return nil, sdkerrors.Wrapf(err, "contract number %d", i)
			}
		}
	}

	var maxContractID uint64
	for i, contract := range data.Contracts {
		contractAddr, err := sdk.AccAddressFromBech32(contract.ContractAddress)
		if err != nil {
			return nil, sdkerrors.Wrapf(err, "address in contract number %d", i)
		}
		err = keeper.importContract(ctx, contractAddr, &contract.ContractInfo, contract.ContractState)
		if err != nil {
			return nil, sdkerrors.Wrapf(err, "contract number %d", i)
		}
		maxContractID = uint64(i) + 1 // #nosec G115 -- loop index i is always non-negative
	}

	for i, seq := range data.Sequences {
		err := keeper.importAutoIncrementID(ctx, seq.IDKey, seq.Value)
		if err != nil {
			return nil, sdkerrors.Wrapf(err, "sequence number %d", i)
		}
	}

	// sanity check seq values
	seqVal := keeper.PeekAutoIncrementID(ctx, types.KeyLastCodeID)
	if seqVal <= maxCodeID {
		return nil, sdkerrors.Wrapf(types.ErrInvalid, "seq %s with value: %d must be greater than: %d ", string(types.KeyLastCodeID), seqVal, maxCodeID)
	}
	seqVal = keeper.PeekAutoIncrementID(ctx, types.KeyLastInstanceID)
	if seqVal <= maxContractID {
		return nil, sdkerrors.Wrapf(types.ErrInvalid, "seq %s with value: %d must be greater than: %d ", string(types.KeyLastInstanceID), seqVal, maxContractID)
	}

	if len(data.GenMsgs) == 0 {
		return nil, nil
	}
	for _, genTx := range data.GenMsgs {
		msg := genTx.AsMsg()
		if msg == nil {
			return nil, sdkerrors.Wrapf(sdkerrors.ErrInvalidType, "unknown message")
		}
		_, err := msgHandler(ctx, msg)
		if err != nil {
			return nil, sdkerrors.Wrap(err, "genesis")
		}
	}
	return stakingKeeper.ApplyAndReturnValidatorSetUpdates(ctx)
}

// ExportGenesis returns a GenesisState for a given context and keeper.
func ExportGenesis(ctx sdk.Context, keeper *Keeper) *types.GenesisState {
	var genState types.GenesisState

	genState.Params = keeper.GetParams(ctx)

	keeper.IterateCodeInfos(ctx, func(codeID uint64, info types.CodeInfo) bool {
		bytecode, err := keeper.GetByteCode(ctx, codeID)
		if err != nil {
			panic(err)
		}
		genState.Codes = append(genState.Codes, types.Code{
			CodeID:    codeID,
			CodeInfo:  info,
			CodeBytes: bytecode,
			Pinned:    keeper.IsPinnedCode(ctx, codeID),
		})
		return false
	})

	keeper.IterateContractInfo(ctx, func(addr sdk.AccAddress, contract types.ContractInfo) bool {
		var state []types.Model
		keeper.IterateContractState(ctx, addr, func(key, value []byte) bool {
			state = append(state, types.Model{Key: key, Value: value})
			return false
		})
		// redact contract info
		contract.Created = nil
		genState.Contracts = append(genState.Contracts, types.Contract{
			ContractAddress: addr.String(),
			ContractInfo:    contract,
			ContractState:   state,
		})
		return false
	})

	for _, k := range [][]byte{types.KeyLastCodeID, types.KeyLastInstanceID} {
		genState.Sequences = append(genState.Sequences, types.Sequence{
			IDKey: k,
			Value: keeper.PeekAutoIncrementID(ctx, k),
		})
	}

	return &genState
}

const GENSIS_STATE_STREAM_BUF_THRESHOLD = 50000

func ExportGenesisStream(ctx sdk.Context, keeper *Keeper) <-chan *types.GenesisState {
	ch := make(chan *types.GenesisState)
	go func() {
		var genState types.GenesisState
		genState.Params = keeper.GetParams(ctx)
		genState.Sequences = []types.Sequence{{IDKey: []byte(genesisStreamStart), Value: 1}}
		ch <- &genState

		// Needs to be first because there are invariant checks when importing that need sequences info
		for _, k := range [][]byte{types.KeyLastCodeID, types.KeyLastInstanceID} {
			var genState types.GenesisState
			genState.Params = keeper.GetParams(ctx)
			genState.Sequences = append(genState.Sequences, types.Sequence{
				IDKey: k,
				Value: keeper.PeekAutoIncrementID(ctx, k),
			})
			ch <- &genState
		}

		keeper.IterateCodeInfos(ctx, func(codeID uint64, info types.CodeInfo) bool {
			var genState types.GenesisState
			genState.Params = keeper.GetParams(ctx)
			bytecode, err := keeper.GetByteCode(ctx, codeID)
			if err != nil {
				panic(err)
			}
			genState.Codes = append(genState.Codes, types.Code{
				CodeID:    codeID,
				CodeInfo:  info,
				CodeBytes: bytecode,
				Pinned:    keeper.IsPinnedCode(ctx, codeID),
			})
			ch <- &genState
			return false
		})

		var contractCount uint64
		keeper.IterateContractInfo(ctx, func(addr sdk.AccAddress, contract types.ContractInfo) bool {
			contractCount++
			contract.Created = nil
			var state []types.Model
			var offset uint64
			emit := func(models []types.Model, final bool) {
				value := offset
				if final {
					value |= genesisStreamFinalBit
				}
				ch <- &types.GenesisState{
					Params:    keeper.GetParams(ctx),
					Sequences: []types.Sequence{{IDKey: append([]byte(genesisStreamChunk), addr...), Value: value}},
					Contracts: []types.Contract{{ContractAddress: addr.String(), ContractInfo: contract, ContractState: models}},
				}
				offset += uint64(len(models))
			}
			keeper.IterateContractState(ctx, addr, func(key, value []byte) bool {
				state = append(state, types.Model{Key: bytes.Clone(key), Value: bytes.Clone(value)})
				if len(state) > GENSIS_STATE_STREAM_BUF_THRESHOLD {
					emit(state[:GENSIS_STATE_STREAM_BUF_THRESHOLD], false)
					state = state[GENSIS_STATE_STREAM_BUF_THRESHOLD:]
				}
				return false
			})
			emit(state, true)
			return false
		})
		ch <- &types.GenesisState{Params: keeper.GetParams(ctx), Sequences: []types.Sequence{{IDKey: []byte(genesisStreamEnd), Value: contractCount}}}

		close(ch)
	}()
	return ch
}

// Stream controls travel through the existing Sequence protobuf so the module's
// JSON boundary preserves them without an out-of-scope schema change.
const (
	genesisStreamStart           = "wasm/genesis-stream/v1/start"
	genesisStreamEnd             = "wasm/genesis-stream/v1/end"
	genesisStreamChunk           = "wasm/genesis-stream/v1/contract/"
	genesisStreamFinalBit uint64 = 1 << 63
)

var genesisStreamStorePrefix = []byte{0x09}

func importGenesisStreamControl(ctx sdk.Context, k *Keeper, data types.GenesisState) (bool, error) {
	var control *types.Sequence
	for i := range data.Sequences {
		if bytes.HasPrefix(data.Sequences[i].IDKey, []byte("wasm/genesis-stream/")) {
			control = &data.Sequences[i]
			break
		}
	}
	if control == nil {
		return false, nil
	}
	if len(data.Sequences) != 1 || len(data.Codes) != 0 || len(data.GenMsgs) != 0 {
		return true, sdkerrors.Wrap(types.ErrInvalid, "mixed genesis stream control")
	}
	if err := data.ValidateBasic(); err != nil {
		return true, err
	}
	cache, write := ctx.CacheContext()
	store := cache.KVStore(k.storeKey)
	key := string(control.IDKey)
	switch {
	case key == genesisStreamStart:
		if len(data.Contracts) != 0 || control.Value != 1 || store.Has(genesisStreamStorePrefix) {
			return true, sdkerrors.Wrap(types.ErrInvalid, "invalid genesis stream start")
		}
		var occupied bool
		k.IterateContractInfo(cache, func(_ sdk.AccAddress, _ types.ContractInfo) bool { occupied = true; return true })
		if occupied {
			return true, sdkerrors.Wrap(types.ErrDuplicate, "genesis stream requires an empty contract store")
		}
		k.SetParams(cache, data.Params)
		store.Set(genesisStreamStorePrefix, []byte{1})
	case key == genesisStreamEnd:
		if len(data.Contracts) != 0 || !store.Has(genesisStreamStorePrefix) {
			return true, sdkerrors.Wrap(types.ErrInvalid, "unexpected genesis stream end")
		}
		if err := k.validateGenesisImportChunks(cache); err != nil {
			return true, err
		}
		var count uint64
		k.IterateContractInfo(cache, func(_ sdk.AccAddress, _ types.ContractInfo) bool { count++; return false })
		if count != control.Value || k.PeekAutoIncrementID(cache, types.KeyLastInstanceID) <= count {
			return true, sdkerrors.Wrap(types.ErrInvalid, "genesis contract count or sequence mismatch")
		}
		var maxCodeID uint64
		k.IterateCodeInfos(cache, func(id uint64, _ types.CodeInfo) bool {
			if id > maxCodeID {
				maxCodeID = id
			}
			return false
		})
		if k.PeekAutoIncrementID(cache, types.KeyLastCodeID) <= maxCodeID {
			return true, sdkerrors.Wrap(types.ErrInvalid, "genesis code sequence mismatch")
		}
		store.Delete(genesisStreamStorePrefix)
	case bytes.HasPrefix(control.IDKey, []byte(genesisStreamChunk)):
		if len(data.Contracts) != 1 || !store.Has(genesisStreamStorePrefix) {
			return true, sdkerrors.Wrap(types.ErrInvalid, "contract chunk outside genesis stream")
		}
		contract := data.Contracts[0]
		addr, err := sdk.AccAddressFromBech32(contract.ContractAddress)
		if err != nil {
			return true, err
		}
		if !bytes.Equal(control.IDKey[len(genesisStreamChunk):], addr) {
			return true, sdkerrors.Wrap(types.ErrInvalid, "contract chunk address mismatch")
		}
		if err := k.importContractChunk(cache, addr, contract, control.Value); err != nil {
			return true, err
		}
	default:
		return true, sdkerrors.Wrap(types.ErrInvalid, "unknown genesis stream control")
	}
	write()
	return true, nil
}

func (k Keeper) importContractChunk(ctx sdk.Context, addr sdk.AccAddress, contract types.Contract, value uint64) error {
	offset, final := value&^genesisStreamFinalBit, value&genesisStreamFinalBit != 0
	if uint64(len(contract.ContractState)) > math.MaxInt64-offset || (!final && len(contract.ContractState) == 0) {
		return sdkerrors.Wrap(types.ErrInvalid, "invalid contract chunk size")
	}
	store := ctx.KVStore(k.storeKey)
	markerKey := append(bytes.Clone(genesisStreamStorePrefix), addr...)
	marker := store.Get(markerKey)
	var previousKey []byte
	if offset == 0 {
		if marker != nil || k.HasContractInfo(ctx, addr) {
			return sdkerrors.Wrap(types.ErrDuplicate, "contract chunk start")
		}
	} else {
		if len(marker) < 8 || binary.BigEndian.Uint64(marker[:8]) != offset {
			return sdkerrors.Wrap(types.ErrInvalid, "contract chunk offset mismatch")
		}
		previousKey = marker[8:]
		existing := k.GetContractInfo(ctx, addr)
		if existing == nil {
			return sdkerrors.Wrap(types.ErrNotFound, "contract chunk metadata")
		}
		existing.Created = nil
		if !bytes.Equal(k.cdc.MustMarshal(existing), k.cdc.MustMarshal(&contract.ContractInfo)) {
			return sdkerrors.Wrap(types.ErrDuplicate, "conflicting contract chunk metadata")
		}
	}
	for _, model := range contract.ContractState {
		if len(model.Key) == 0 || (previousKey != nil && bytes.Compare(previousKey, model.Key) >= 0) {
			return sdkerrors.Wrap(types.ErrInvalid, "contract chunk keys must be strictly increasing")
		}
		previousKey = model.Key
	}
	if offset == 0 {
		if err := k.importContract(ctx, addr, &contract.ContractInfo, contract.ContractState); err != nil {
			return err
		}
	} else if err := k.importContractState(ctx, addr, contract.ContractState); err != nil {
		return err
	}
	if final {
		store.Delete(markerKey)
	} else {
		store.Set(markerKey, append(sdk.Uint64ToBigEndian(offset+uint64(len(contract.ContractState))), previousKey...))
	}
	return nil
}

// FinalizeGenesisImport refuses incomplete contract state, including after a restart.
// The streaming module owner must also call this when an input stream ends early.
func (k Keeper) FinalizeGenesisImport(ctx sdk.Context) error {
	if ctx.KVStore(k.storeKey).Has(genesisStreamStorePrefix) {
		return sdkerrors.Wrap(types.ErrInvalid, "genesis stream has not reached its final record")
	}
	return k.validateGenesisImportChunks(ctx)
}

func (k Keeper) validateGenesisImportChunks(ctx sdk.Context) error {
	iter := ctx.KVStore(k.storeKey).Iterator([]byte{0x09, 0x00}, []byte{0x0a})
	defer iter.Close()
	if iter.Valid() {
		return sdkerrors.Wrap(types.ErrInvalid, "unfinished contract genesis chunks")
	}
	return nil
}
