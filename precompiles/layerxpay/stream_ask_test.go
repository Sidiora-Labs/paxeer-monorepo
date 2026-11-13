package layerxpay_test

import (
	"encoding/hex"
	"math/big"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	custodytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxcustody/types"
	paykeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/keeper"
	paytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	lp "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxpay"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/cachemulti"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/dbadapter"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func streamAskFixture(t *testing.T) *escrowTestFixture {
	t.Helper()
	f := streamTestFixture(t)
	denom, err := sdk.GetBaseDenom()
	require.NoError(t, err)
	require.NoError(t, testkeeper.EVMTestApp.LayerXCustodyKeeper.SetAsset(f.h.db.Ctx(), custodytypes.AssetMapping{
		AssetId: hex.EncodeToString(f.asset[:]), Denom: denom, Enabled: true,
	}))
	return f
}

func streamAskArgs(f *escrowTestFixture, ask [32]byte) []interface{} {
	return []interface{}{ask, f.beneficiary.caller, big.NewInt(3), uint8(2), big.NewInt(100), f.arbiter.public}
}

func streamAskOpen(t *testing.T, f *escrowTestFixture, ask [32]byte) [32]byte {
	t.Helper()
	h := f.h
	a := h.precompile.GetABI()
	input, err := a.Pack("streamOpenForAsk", streamAskArgs(f, ask)...)
	require.NoError(t, err)
	before := h.keeper.ExportGenesis(h.db.Ctx())
	logs := len(h.db.GetAllLogs())
	output, _, err := h.evm.StaticCall(f.owner.caller, h.precompile.Address(), input, 2000000)
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	reason, err := abi.UnpackRevert(output)
	require.NoError(t, err)
	require.Contains(t, reason, "LXP_UNAUTHORIZED_DEBIT")
	require.Equal(t, before, h.keeper.ExportGenesis(h.db.Ctx()))
	require.Equal(t, []interface{}{[32]byte{}}, budgetTestView(t, h, "askStreamOf", ask))
	require.Len(t, h.db.GetAllLogs(), logs)
	output, _, err = h.evm.Call(f.owner.caller, h.precompile.Address(), input, 2000000, uint256.NewInt(0))
	require.NoError(t, err, "%x", output)
	values, err := a.Unpack("streamOpenForAsk", output)
	require.NoError(t, err)
	require.Len(t, values, 1)
	id := values[0].([32]byte)
	var selector [4]byte
	copy(selector[:], a.Methods["streamOpenForAsk"].ID)
	wantID := paytypes.DeriveIdempotencyKey([32]byte(h.db.Ctx().EVMTxHash()), 0, selector)
	clear(wantID[:16])
	require.Equal(t, wantID, id)
	require.NotEqual(t, [32]byte{}, id)
	require.Equal(t, [16]byte{}, [16]byte(id[:16]))
	all := h.db.GetAllLogs()
	require.Len(t, all, logs+1)
	log := all[logs]
	require.Equal(t, common.HexToAddress(lp.PayAddress), log.Address)
	require.Equal(t, []common.Hash{a.Events["AskStreamOpened"].ID, common.Hash(ask), common.Hash(id)}, log.Topics)
	data, err := a.Events["AskStreamOpened"].Inputs.NonIndexed().Unpack(log.Data)
	require.NoError(t, err)
	require.Equal(t, []interface{}{f.beneficiary.caller}, data)
	require.Equal(t, []interface{}{id}, budgetTestView(t, h, "askStreamOf", ask))
	return id
}

func TestStreamAskOpenAndHostMeter(t *testing.T) {
	f := streamAskFixture(t)
	h, ask := f.h, [32]byte{0xa5, 1}
	id := streamAskOpen(t, f, ask)
	r, err := h.keeper.GetStream(h.db.Ctx(), id)
	require.NoError(t, err)
	require.Equal(t, paytypes.StreamModeMetered, r.Mode)
	require.Equal(t, [][32]byte{f.arbiter.public}, r.MeterAuthorities)
	require.Equal(t, f.owner.account, r.Payer)
	require.Equal(t, f.beneficiary.account, r.Recipient)
	require.Equal(t, f.asset, r.AssetID)
	require.Equal(t, codec.U128{Lo: 3}, r.Rate)
	require.Equal(t, uint64(2), r.RateUnit)
	require.Equal(t, codec.U128{Lo: 100}, r.TotalCap)
	require.Zero(t, r.EndTimestamp)
	require.Zero(t, r.CumulativeMeter)
	budgetTestBalance(t, h, f.owner.account, f.asset, "900")
	budgetTestBalance(t, h, r.StreamAccount, f.asset, "100")
	budgetTestBalance(t, h, f.beneficiary.account, f.asset, "0")

	// The payer can relay the host's signature; a payer or settlement key
	// cannot replace the sole host authority even with a valid signature.
	for _, other := range []escrowTestActor{f.owner, f.beneficiary} {
		budgetTestRevert(t, h, other.caller, "streamMeter", id, uint64(15), other.public, streamTestSignature(t, other, id, 15))
	}
	r = streamTestWrite(t, h, f.owner.caller, "streamMeter", "StreamMetered",
		[]interface{}{uint64(15), big.NewInt(22), big.NewInt(22)},
		id, uint64(15), f.arbiter.public, streamTestSignature(t, f.arbiter, id, 15))
	require.Equal(t, uint64(15), r.CumulativeMeter)
	require.Equal(t, codec.U128{Lo: 22}, r.AccruedTotal)
	require.Equal(t, codec.U128{Lo: 1}, r.RemainderCarry)
	budgetTestRevert(t, h, f.arbiter.caller, "streamMeter", id, uint64(14), f.arbiter.public, streamTestSignature(t, f.arbiter, id, 14))
	afterRefusal, err := h.keeper.GetStream(h.db.Ctx(), id)
	require.NoError(t, err)
	require.Equal(t, r, afterRefusal)

	// A lap reading counts tokens cumulatively, independently of elapsed time
	// and settlement. Only another signed streamMeter call can advance it.
	h.db.WithCtx(h.db.Ctx().WithBlockTime(h.db.Ctx().BlockTime().Add(time.Minute)))
	require.Equal(t, []interface{}{big.NewInt(22), big.NewInt(22)}, budgetTestView(t, h, "streamAccrued", id))
	r = streamTestWrite(t, h, f.beneficiary.caller, "streamSettle", "StreamSettled",
		[]interface{}{big.NewInt(22), big.NewInt(22), false}, id, [32]byte{0xa5, 2})
	require.Equal(t, uint64(15), r.CumulativeMeter)
	budgetTestBalance(t, h, f.beneficiary.account, f.asset, "22")
	r = streamTestWrite(t, h, f.arbiter.caller, "streamMeter", "StreamMetered",
		[]interface{}{uint64(17), big.NewInt(3), big.NewInt(25)},
		id, uint64(17), f.arbiter.public, streamTestSignature(t, f.arbiter, id, 17))
	require.Equal(t, uint64(17), r.CumulativeMeter)
	require.Equal(t, codec.U128{Lo: 25}, r.AccruedTotal)
	require.Equal(t, []interface{}{id}, budgetTestView(t, h, "askStreamOf", ask))
	require.Equal(t, []interface{}{[32]byte{}}, budgetTestView(t, h, "askStreamOf", [32]byte{0xff}))

	// Closing keeps the binding, so an ask cannot acquire a second stream.
	streamTestWrite(t, h, f.owner.caller, "streamClose", "StreamClosed",
		[]interface{}{big.NewInt(3), big.NewInt(75)}, id, [32]byte{0xa5, 3})
	budgetTestRevert(t, h, f.owner.caller, "streamOpenForAsk", streamAskArgs(f, ask)...)
	require.Equal(t, []interface{}{id}, budgetTestView(t, h, "askStreamOf", ask))
}

func TestStreamAskOpenValidation(t *testing.T) {
	for _, name := range []string{"zero ask", "zero settlement", "unbound settlement", "zero host", "zero rate", "zero unit", "zero cap", "rate overflow", "cap overflow", "insufficient funding", "disabled", "paused"} {
		t.Run(name, func(t *testing.T) {
			f := streamAskFixture(t)
			h, ask := f.h, [32]byte{0xa6}
			args := streamAskArgs(f, ask)
			switch name {
			case "zero ask":
				args[0] = [32]byte{}
			case "zero settlement":
				args[1] = common.Address{}
			case "unbound settlement":
				args[1] = common.HexToAddress("0xbad")
			case "zero host":
				args[5] = [32]byte{}
			case "zero rate":
				args[2] = big.NewInt(0)
			case "zero unit":
				args[3] = uint8(0)
			case "zero cap":
				args[4] = big.NewInt(0)
			case "rate overflow":
				args[2] = new(big.Int).Lsh(big.NewInt(1), 128)
			case "cap overflow":
				args[4] = new(big.Int).Lsh(big.NewInt(1), 128)
			case "insufficient funding":
				args[4] = big.NewInt(1001)
			case "disabled":
				h.keeper.SetParams(h.db.Ctx(), paytypes.Params{})
			case "paused":
				h.keeper.SetParams(h.db.Ctx(), paytypes.Params{StreamEnabled: true, Paused: true})
			}
			budgetTestRevert(t, h, f.owner.caller, "streamOpenForAsk", args...)
			require.Equal(t, []interface{}{[32]byte{}}, budgetTestView(t, h, "askStreamOf", args[0].([32]byte)))
			budgetTestBalance(t, h, f.owner.account, f.asset, "1000")
		})
	}
}

func TestStreamAskBindingPersists(t *testing.T) {
	f := streamAskFixture(t)
	ask := [32]byte{0xa8}
	id := streamAskOpen(t, f, ask)
	ctx := f.h.db.Ctx()
	directory := t.TempDir()
	stores := map[storetypes.StoreKey]storetypes.CacheWrapper{}
	keys := map[string]storetypes.StoreKey{}
	var payKey, hostKey storetypes.StoreKey
	for _, key := range ctx.MultiStore().StoreKeys() {
		if key.Name() != paytypes.StoreKey && key.Name() != "lxphost" {
			continue
		}
		keys[key.Name()] = key
		kv := ctx.KVStore(key)
		if key.Name() == paytypes.StoreKey {
			payKey = key
		} else {
			hostKey = key
		}
		// Persist the actual producer's complete records, then close and reopen
		// both databases before constructing the new host and keeper.
		db, err := dbm.NewGoLevelDB(key.Name(), directory)
		require.NoError(t, err)
		it := kv.Iterator(nil, nil)
		for ; it.Valid(); it.Next() {
			require.NoError(t, db.SetSync(it.Key(), it.Value()))
		}
		require.NoError(t, it.Close())
		require.NoError(t, db.Close())
		db, err = dbm.NewGoLevelDB(key.Name(), directory)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, db.Close()) })
		stores[key] = dbadapter.Store{DB: db}
	}
	require.NotNil(t, payKey)
	require.NotNil(t, hostKey)
	ctx = ctx.WithMultiStore(cachemulti.NewStore(dbm.NewMemDB(), stores, keys, nil, nil, nil, 0))
	host, err := lxphost.NewHost(lxphost.Config{StoreKey: hostKey, NetworkID: 7, ParameterVersion: 1, TimestampWindowMs: 60000, GasLimit: 1000000})
	require.NoError(t, err)
	t.Cleanup(host.Close)
	app := testkeeper.EVMTestApp
	keeper, err := paykeeper.NewKeeper(app.AppCodec(), payKey, hostKey, host, &app.EvmKeeper, app.LayerXCustodyKeeper, "pay-governance")
	require.NoError(t, err)
	got, err := keeper.AskStreamOf(ctx, ask)
	require.NoError(t, err)
	require.Equal(t, id, got)
	r, err := keeper.GetStream(ctx, got)
	require.NoError(t, err)
	require.Equal(t, paytypes.StreamModeMetered, r.Mode)
	require.Equal(t, [][32]byte{f.arbiter.public}, r.MeterAuthorities)
	metadata := ctx.KVStore(payKey).Get(append([]byte(paykeeper.StreamMetadataAskRef), id[:]...))
	require.Equal(t, ask[:], metadata)
	ctx.KVStore(payKey).Set(append([]byte(paykeeper.StreamMetadataAskRef), id[:]...), []byte{1})
	_, err = keeper.AskStreamOf(ctx, ask)
	require.ErrorContains(t, err, "metadata mismatch")
}

func TestStreamAskGas(t *testing.T) {
	f := streamAskFixture(t)
	h := f.h
	a := h.precompile.GetABI()
	args := streamAskArgs(f, [32]byte{0xa7})
	input, err := a.Pack("streamOpenForAsk", args...)
	require.NoError(t, err)
	require.Equal(t, "streamOpenForAsk(bytes32,address,uint256,uint8,uint256,bytes32)", a.Methods["streamOpenForAsk"].Sig)
	require.Equal(t, "AskStreamOpened(bytes32,bytes32,address)", a.Events["AskStreamOpened"].Sig)
	row, ok := lp.MethodGas["streamOpenForAsk"]
	require.True(t, ok)
	require.True(t, row.Write)
	require.Equal(t, uint64(50000), row.Gas(nil))
	require.Equal(t, uint64(50000+30*len(input)), lp.RequiredGas(input))
	require.Equal(t, uint64(50000+30*(len(input)-4)), h.precompile.RequiredGas(input))
	before := h.keeper.ExportGenesis(h.db.Ctx())
	output, left, err := h.evm.Call(f.owner.caller, h.precompile.Address(), input, row.Gas(input[4:])-1, uint256.NewInt(0))
	require.ErrorIs(t, err, vm.ErrOutOfGas)
	require.Empty(t, output)
	require.Zero(t, left)
	require.Equal(t, before, h.keeper.ExportGenesis(h.db.Ctx()))
	require.Equal(t, []interface{}{[32]byte{}}, budgetTestView(t, h, "askStreamOf", args[0].([32]byte)))
	output, left, err = h.evm.Call(f.owner.caller, h.precompile.Address(), input, 2000000, uint256.NewInt(0))
	require.NoError(t, err, "%x", output)
	require.Greater(t, uint64(2000000)-left, row.Gas(input[4:]))
	values, err := a.Unpack("streamOpenForAsk", output)
	require.NoError(t, err)
	require.NotEqual(t, [32]byte{}, values[0])
	view, err := a.Pack("askStreamOf", args[0])
	require.NoError(t, err)
	require.False(t, lp.MethodGas["askStreamOf"].Write)
	require.Equal(t, uint64(1000+3*len(view)), lp.RequiredGas(view))
	require.Equal(t, values, budgetTestView(t, h, "askStreamOf", args[0].([32]byte)))
}
