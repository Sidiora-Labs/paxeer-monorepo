package integration_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"math"
	"math/big"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	paytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	paypc "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxpay"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/kv"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

type genesisActor struct {
	private ed25519.PrivateKey
	public  [32]byte
	did     []byte
	id      [32]byte
	caller  common.Address
}

func genesisExecute(t *testing.T, w *app.TestWrapper, actor genesisActor, kind uint32, payload []byte, key [32]byte) {
	t.Helper()
	host, ctx := w.App.LayerXHost, w.Ctx
	last, err := host.ReadLastSequence(ctx, actor.did)
	require.NoError(t, err)
	sequence, err := host.ReadIdentitySequence(ctx, actor.did)
	require.NoError(t, err)
	activity := lxphost.Activity{
		ProtocolVersion: 3, Type: kind, ActorDID: actor.did, AuthorityKey: actor.public,
		AccountSequence: sequence, NotBefore: uint64(ctx.BlockTime().UnixMilli()),
		NotAfter:       uint64(ctx.BlockTime().UnixMilli()) + host.TimestampWindowMs(),
		IdempotencyKey: key, Payload: payload,
	}
	digest, err := host.SigningHash(activity)
	require.NoError(t, err)
	activity.Signature = ed25519.Sign(actor.private, digest[:])
	receipt, err := host.HostExecute(lxphost.WithSequence(ctx, last+1), activity,
		lxphost.KernelAuthority{Actor: actor.id, Principal: actor.id, VerifiedKey: actor.public})
	require.NoError(t, err)
	require.Zero(t, receipt.ResultCode)
}

func genesisRegister(t *testing.T, w *app.TestWrapper, seed byte) genesisActor {
	t.Helper()
	a := genesisActor{private: ed25519.NewKeyFromSeed(bytes.Repeat([]byte{seed}, ed25519.SeedSize)), caller: common.BytesToAddress([]byte{seed})}
	copy(a.public[:], a.private.Public().(ed25519.PublicKey))
	a.did = []byte(evmtypes.LayerXDid(a.public))
	var err error
	a.id, err = lxphost.DIDID(a.did)
	require.NoError(t, err)
	message, err := evmtypes.LayerXBindMessage(w.App.EvmKeeper.ChainID(w.Ctx), a.caller, 0)
	require.NoError(t, err)
	var signature [64]byte
	copy(signature[:], ed25519.Sign(a.private, message))
	_, err = w.App.EvmKeeper.BindLayerX(w.Ctx, a.caller, a.public, signature)
	require.NoError(t, err)
	payload := append([]byte{0x71, 1, 0, 2}, a.id[:]...)
	payload = append(payload, a.public[:]...)
	genesisExecute(t, w, a, 0x00070001, payload, sha256.Sum256(a.did))
	return a
}

func genesisMainAccount(t *testing.T, w *app.TestWrapper, actor genesisActor, asset [32]byte) [32]byte {
	t.Helper()
	name, id, err := lxphost.MainAccountID(actor.public)
	require.NoError(t, err)
	field := func(out, value []byte) []byte {
		return append(binary.BigEndian.AppendUint32(out, uint32(len(value))), value...)
	}
	row := field(nil, id[:])
	row = field(row, []byte(name))
	row = append(row, 1)
	row = append(row, make([]byte, 16)...)
	row = field(row, asset[:])
	row = append(row, 1)
	row = binary.BigEndian.AppendUint64(row, 0)
	row = binary.BigEndian.AppendUint64(row, 1)
	row = append(row, 0, 0)
	row = field(row, actor.public[:])
	row = append(row, 1)
	w.Ctx.KVStore(w.App.GetKey(app.LayerXHostStoreKey)).Set(append([]byte{0x06}, id[:]...), row)
	return id
}

func genesisEVM(t *testing.T, w *app.TestWrapper) (*vm.EVM, *state.DBImpl, abi.ABI) {
	t.Helper()
	served := w.App.EvmKeeper.CustomPrecompiles(w.Ctx)
	contract, found := served[common.HexToAddress(paypc.PayAddress)]
	require.True(t, found, "application must serve layerxpay at 0x101B")
	withABI, ok := contract.(interface{ GetABI() abi.ABI })
	require.True(t, ok)
	db := state.NewDBImpl(w.Ctx.WithEventManager(sdk.NewEventManager()), &w.App.EvmKeeper, false)
	block, err := w.App.EvmKeeper.GetVMBlockContext(w.Ctx, core.GasPool(math.MaxUint64))
	require.NoError(t, err)
	chain := evmtypes.DefaultChainConfig().EthereumConfig(w.App.EvmKeeper.ChainID(w.Ctx))
	return vm.NewEVM(*block, db, chain, vm.Config{}, served), db, withABI.GetABI()
}

func genesisSharedRecords(t *testing.T, w *app.TestWrapper, genesis paytypes.GenesisState) {
	t.Helper()
	require.Same(t, w.App.LayerXHost, w.App.LayerXPayKeeper.Host())
	hostStore := w.Ctx.KVStore(w.App.GetKey(app.LayerXHostStoreKey))
	var records []kv.Pair
	for _, prefix := range [][]byte{paytypes.EscrowKVPrefix, paytypes.BudgetKVPrefix, paytypes.StreamKVPrefix, paytypes.ServiceKVPrefix, paytypes.GovernanceKVPrefix} {
		iterator := sdk.KVStorePrefixIterator(hostStore, prefix)
		for ; iterator.Valid(); iterator.Next() {
			records = append(records, kv.Pair{Key: bytes.Clone(iterator.Key()), Value: bytes.Clone(iterator.Value())})
		}
		require.NoError(t, iterator.Close())
	}
	require.NotEmpty(t, records)
	require.Equal(t, records, genesis.KernelRecords)
	moduleStore := w.Ctx.KVStore(w.App.GetKey(paytypes.StoreKey))
	for _, record := range records {
		require.False(t, moduleStore.Has(record.Key), "kernel records belong to the shared host")
	}
}

func TestAppExportsPayGenesisWithSharedHost(t *testing.T) {
	now := time.UnixMilli(1_800_000_000_000).UTC()
	validator := secp256k1.GenPrivKey().PubKey()
	source := app.NewTestWrapper(t, now, validator, true)
	t.Cleanup(func() { require.NoError(t, source.App.Close()) })
	source.Ctx = source.Ctx.WithEVMTxHash(common.HexToHash("0x2386"))
	source.App.UpgradeKeeper.SetDone(source.Ctx, app.MergeUpgrade)
	enableKey, err := paytypes.KernelModuleEnableKey(paytypes.EscrowModuleID)
	require.NoError(t, err)
	enabled := make([]byte, 32)
	enabled[31] = 1
	initial := paytypes.GenesisState{Params: paytypes.Params{EscrowEnabled: true}, KernelRecords: []kv.Pair{{Key: enableKey, Value: enabled}}}
	initialJSON, err := json.Marshal(initial)
	require.NoError(t, err)
	stateJSON, err := json.Marshal(map[string]json.RawMessage{paytypes.ModuleName: initialJSON})
	require.NoError(t, err)
	source.App.InitChainer(source.Ctx, abci.RequestInitChain{AppStateBytes: stateJSON})
	owner := genesisRegister(t, source, 1)
	beneficiary := genesisRegister(t, source, 2)
	salt := [32]byte{0x38, 0x06}
	asset := sha256.Sum256(append(append([]byte("LX:ASSET:v1"), owner.id[:]...), salt[:]...))
	main := genesisMainAccount(t, source, owner, asset)
	payee := genesisMainAccount(t, source, beneficiary, asset)
	assetName, assetAccount, err := lxphost.AssetAccountID(owner.public, asset)
	require.NoError(t, err)
	payload := append([]byte{0, 1}, asset[:]...)
	payload = append(payload, salt[:]...)
	payload = append(payload, 1, 'T', 1, 'T', 6)
	payload = binary.BigEndian.AppendUint64(payload, 0)
	payload = binary.BigEndian.AppendUint64(payload, 2000)
	payload = append(payload, 1, 0)
	genesisExecute(t, source, owner, 0x00010001, payload, [32]byte{0x31})
	genesisExecute(t, source, owner, 0x00010004, append([]byte{0, 1}, asset[:]...), [32]byte{0x32})
	payload = append(append([]byte{0, 1}, asset[:]...), assetAccount[:]...)
	payload = binary.BigEndian.AppendUint64(payload, 0)
	payload = binary.BigEndian.AppendUint64(payload, 1000)
	genesisExecute(t, source, owner, 0x0001000a, payload, [32]byte{0x33})
	amount := (codec.U128{Lo: 1000}).Bytes()
	key := [32]byte{0x34}
	material := append(append(append(append([]byte(nil), assetAccount[:]...), main[:]...), asset[:]...), amount[:]...)
	contextHash, err := codec.DomainHash(codec.DomainContextHash, append(bytes.Clone(material), key[:]...))
	require.NoError(t, err)
	account, err := source.App.LayerXHost.ReadAccount(source.Ctx, assetName, assetAccount)
	require.NoError(t, err)
	transfer := binary.BigEndian.AppendUint64(material, account.NextSequence)
	transfer = append(transfer, key[:]...)
	transfer = binary.BigEndian.AppendUint64(transfer, uint64(now.UnixMilli())+10000)
	transfer = append(transfer, contextHash[:]...)
	transfer = append(transfer, 0)
	networkID := source.App.EvmKeeper.ChainID(source.Ctx)
	require.True(t, networkID.IsUint64())
	require.LessOrEqual(t, networkID.Uint64(), uint64(math.MaxUint32))
	suffix := binary.BigEndian.AppendUint16(binary.BigEndian.AppendUint32(nil, uint32(networkID.Uint64())), 3)
	message := append([]byte{0x53, 1}, transfer...)
	message = append(message, 1)
	message = append(message, assetAccount[:]...)
	message = append(message, contextHash[:]...)
	message = append(message, suffix...)
	digest, err := codec.DomainHash(codec.DomainSignaturePreimage, message)
	require.NoError(t, err)
	payload = append([]byte{0x53, 1, 0, 10}, transfer...)
	payload = append(payload, 1)
	payload = append(payload, assetAccount[:]...)
	payload = append(payload, owner.public[:]...)
	payload = append(payload, ed25519.Sign(owner.private, digest[:])...)
	payload = append(payload, contextHash[:]...)
	payload = append(payload, suffix...)
	genesisExecute(t, source, owner, 0x00010005, payload, key)

	id := [32]byte{0x38, 0x06}
	escrowAccount, err := codec.DeriveAccountID([]byte("agent:" + string(owner.did) + ":escrow:" + hex.EncodeToString(id[:])))
	require.NoError(t, err)
	expiry, disputeWindow := uint64(now.UnixMilli())+1000000, uint64(12345)
	terms, agreement := [32]byte{0x35}, [32]byte{0x36}
	evm, db, contractABI := genesisEVM(t, source)
	input, err := contractABI.Pack("escrowOpen", id, main, escrowAccount, payee, owner.id, asset, big.NewInt(600), expiry, disputeWindow, terms, agreement)
	require.NoError(t, err)
	output, _, err := evm.Call(owner.caller, common.HexToAddress(paypc.PayAddress), input, 2000000, uint256.NewInt(0))
	require.NoError(t, err, "%x", output)
	require.Empty(t, output)
	_, err = db.Finalize()
	require.NoError(t, err)
	stored, found := source.App.LayerXPayKeeper.GetEscrow(source.Ctx, id)
	require.True(t, found)
	require.Equal(t, paytypes.EscrowRecord{EscrowID: id, Owner: main, EscrowAccount: escrowAccount, Beneficiary: payee, Arbiter: owner.id, AssetID: asset, LockedAmount: codec.U128{Lo: 600}, State: 1, ExpiryMs: expiry, DisputeWindowMs: disputeWindow, TermsHash: terms, AgreementReference: agreement}, stored)
	for _, balance := range []struct {
		account [32]byte
		want    string
	}{{main, "400"}, {escrowAccount, "600"}} {
		actual, err := source.App.LayerXHost.ReadBalance(source.Ctx, balance.account, asset)
		require.NoError(t, err)
		require.Equal(t, balance.want, actual.String())
	}
	viewInput, err := contractABI.Pack("getEscrow", id)
	require.NoError(t, err)
	beforeEVM, _, _ := genesisEVM(t, source)
	before, _, err := beforeEVM.StaticCall(owner.caller, common.HexToAddress(paypc.PayAddress), viewInput, 2000000)
	require.NoError(t, err, "%x", before)
	values, err := contractABI.Unpack("getEscrow", before)
	require.NoError(t, err)
	require.Len(t, values, 1)
	require.Len(t, before, 12*32)

	*source.App.DeliverContext() = source.Ctx
	source.App.SetDeliverStateToCommit()
	_, err = source.App.Commit(context.Background())
	require.NoError(t, err)
	source.Ctx = source.App.NewContext(true, tmproto.Header{Height: source.App.LastBlockHeight(), ChainID: "pax-test", Time: now})
	exported, err := source.App.ExportAppStateAndValidators(false, nil)
	require.NoError(t, err)
	var exportedState map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(exported.AppState, &exportedState))
	require.Contains(t, exportedState, paytypes.ModuleName)
	var payGenesis paytypes.GenesisState
	require.NoError(t, json.Unmarshal(exportedState[paytypes.ModuleName], &payGenesis))
	require.NoError(t, payGenesis.Validate())
	require.Equal(t, initial.Params, payGenesis.Params)
	genesisSharedRecords(t, source, payGenesis)
	holdKey := paytypes.KernelKVKey(paytypes.EscrowModuleID, append([]byte("hold:"), id[:]...))
	var hold []byte
	for _, record := range payGenesis.KernelRecords {
		if bytes.Equal(record.Key, holdKey) {
			hold = record.Value
		}
	}
	decoded, err := paytypes.DecodeEscrowRecord(hold)
	require.NoError(t, err)
	require.Equal(t, stored, decoded)

	restored := &app.TestWrapper{App: app.Setup(t, true, true, false)}
	t.Cleanup(func() { require.NoError(t, restored.App.Close()) })
	restored.Ctx = sdk.NewContext(restored.App.CommitMultiStore(), tmproto.Header{Height: exported.Height, ChainID: "pax-test", Time: now}, false)
	require.NotSame(t, source.App.LayerXHost, restored.App.LayerXHost)
	_, found = restored.App.LayerXPayKeeper.GetEscrow(restored.Ctx, id)
	require.False(t, found)
	_, err = restored.App.InitChain(context.Background(), &abci.RequestInitChain{
		Time: now, ChainId: "pax-test", InitialHeight: exported.Height,
		ConsensusParams: exported.ConsensusParams, AppStateBytes: exported.AppState,
	})
	require.NoError(t, err)
	restored.Ctx = restored.App.NewContext(false, tmproto.Header{Height: exported.Height, ChainID: "pax-test", Time: now})
	genesisSharedRecords(t, restored, payGenesis)
	restoredRecord, found := restored.App.LayerXPayKeeper.GetEscrow(restored.Ctx, id)
	require.True(t, found)
	require.Equal(t, stored, restoredRecord)
	afterEVM, _, afterABI := genesisEVM(t, restored)
	after, _, err := afterEVM.StaticCall(owner.caller, common.HexToAddress(paypc.PayAddress), viewInput, 2000000)
	require.NoError(t, err, "%x", after)
	afterValues, err := afterABI.Unpack("getEscrow", after)
	require.NoError(t, err)
	require.Equal(t, before, after)
	require.Equal(t, values, afterValues)

	*restored.App.DeliverContext() = restored.Ctx
	restored.App.SetDeliverStateToCommit()
	_, err = restored.App.Commit(context.Background())
	require.NoError(t, err)
	reexported, err := restored.App.ExportAppStateAndValidators(false, nil)
	require.NoError(t, err)
	var reexportedState map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(reexported.AppState, &reexportedState))
	require.Contains(t, reexportedState, paytypes.ModuleName)
	require.JSONEq(t, string(exportedState[paytypes.ModuleName]), string(reexportedState[paytypes.ModuleName]))
}
