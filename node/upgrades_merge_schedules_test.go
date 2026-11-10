package app

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"math"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/params"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	programs "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxprograms"
	upgradetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

func mergeSchedulesView(t *testing.T, f *hostedProgramsABIFixture, name string) []interface{} {
	t.Helper()
	ctx := f.ctx.WithBlockHeight(mergeHeight + 1)
	address := common.HexToAddress("0x101D")
	served := f.app.EvmKeeper.CustomPrecompiles(ctx)
	contract, ok := served[address].(precompiles.IPrecompile)
	require.True(t, ok)
	definition := contract.GetABI()
	input, err := definition.Pack(name)
	require.NoError(t, err)
	db := state.NewDBImpl(ctx, &f.app.EvmKeeper, false)
	block, err := f.app.EvmKeeper.GetVMBlockContext(ctx, core.GasPool(math.MaxUint64))
	require.NoError(t, err)
	chain := evmtypes.DefaultChainConfig().EthereumConfig(f.app.EvmKeeper.ChainID(ctx))
	evm := vm.NewEVM(*block, db, chain, vm.Config{}, served)
	output, _, err := evm.StaticCall(common.HexToAddress("0xa111"), address, input, 2000000)
	require.NoError(t, err, "%s: %v", name, db.GetPrecompileError())
	values, err := definition.Methods[name].Outputs.Unpack(output)
	require.NoError(t, err)
	return values
}

func mergeSchedulesGuest() []byte {
	section := func(id byte, body []byte) []byte { return append([]byte{id, byte(len(body))}, body...) }
	label := func(s string) []byte { return append([]byte{byte(len(s))}, s...) }
	guest := []byte{0, 97, 115, 109, 1, 0, 0, 0}
	guest = append(guest, section(1, []byte{2, 0x60, 1, 0x7f, 1, 0x7f, 0x60, 2, 0x7f, 0x7f, 1, 0x7f})...)
	guest = append(guest, section(3, []byte{2, 0, 1})...)
	guest = append(guest, section(5, []byte{1, 1, 1, 1})...)
	exports := append(append([]byte{3}, label("layerx_reserve")...), 0, 0)
	exports = append(append(exports, label("layerx_call")...), 0, 1)
	exports = append(append(exports, label("memory")...), 2, 0)
	guest = append(guest, section(7, exports)...)
	return append(guest, section(10, []byte{2, 4, 0, 0x41, 0, 0x0b, 4, 0, 0x41, 0, 0x0b})...)
}

func TestMergeUpgradeInstallsProgramsSchedules(t *testing.T) {
	f := newHostedProgramsABIFixture(t)
	store := f.ctx.KVStore(f.app.GetKey(LayerXHostStoreKey))
	key := func(name string) []byte { return append([]byte{1, 0, 9}, []byte(name)...) }
	names := []string{"progfee/active/v1", "progfee/history/v1/\x00\x00\x00\x01", "progmet/active/v1", "progmet/history/v1/\x00\x00\x00\x01"}
	// This fixture performed its legacy deployment before the merge. Remove its
	// legacy schedules to represent an upgrading chain with neither kind stated.
	for _, name := range names {
		store.Delete(key(name))
		require.Nil(t, store.Get(key(name)))
	}
	f.merge(t)
	store = f.ctx.KVStore(f.app.GetKey(LayerXHostStoreKey))
	for _, name := range names {
		require.NotEmpty(t, store.Get(key(name)), name)
	}
	fee := mergeSchedulesView(t, f, "getCurrentFeeSchedule")
	require.Equal(t, []interface{}{uint32(1), [7]uint64{1, 1, 1, 1, 1, 1, 1}, sha256.Sum256([]byte(params.BaseCoinUnit))}, fee)
	metering := mergeSchedulesView(t, f, "getCurrentMeteringSchedule")
	require.Equal(t, []interface{}{uint32(1), [9]uint64{1, 1, 1, 1, 1, 8, 8, 64, 8}}, metering)
	feeRow := store.Get(key(names[0]))
	require.Equal(t, uint64(1000), binary.BigEndian.Uint64(feeRow[97:105]))
	require.Equal(t, uint64(8), binary.BigEndian.Uint64(feeRow[105:113]))
	require.Equal(t, uint64(1), binary.BigEndian.Uint64(feeRow[113:121]))
	require.Equal(t, uint64(8), binary.BigEndian.Uint64(feeRow[121:129]))
	require.Equal(t, uint64(1), binary.BigEndian.Uint64(feeRow[129:137]))
	require.Equal(t, uint64(100), binary.BigEndian.Uint64(feeRow[137:145]))
	require.Equal(t, uint64(1), binary.BigEndian.Uint64(feeRow[145:153]))
	require.Equal(t, uint64(1), binary.BigEndian.Uint64(store.Get(key(names[2]))[81:89]))
	before := make(map[string][]byte)
	for _, name := range names {
		before[name] = bytes.Clone(store.Get(key(name)))
	}
	installed, err := lxphost.InstallProgramsSchedules(f.ctx, f.app.GetKey(LayerXHostStoreKey), mergeProgramsScheduleParameters())
	require.NoError(t, err)
	require.Equal(t, lxphost.ProgramsSchedulesInstalled{}, installed)
	for name, row := range before {
		require.Equal(t, row, store.Get(key(name)), name)
	}
	branch, _ := f.ctx.CacheContext()
	branchStore := branch.KVStore(f.app.GetKey(LayerXHostStoreKey))
	branchStore.Delete(key(names[2]))
	branchStore.Delete(key(names[3]))
	changed := mergeProgramsScheduleParameters()
	changed.FeePrices[0] = 2
	installed, err = lxphost.InstallProgramsSchedules(branch, f.app.GetKey(LayerXHostStoreKey), changed)
	require.NoError(t, err)
	require.Equal(t, lxphost.ProgramsSchedulesInstalled{Metering: true}, installed)
	require.Equal(t, before[names[0]], branchStore.Get(key(names[0])))
	require.Equal(t, before[names[1]], branchStore.Get(key(names[1])))
	require.Equal(t, before[names[2]], branchStore.Get(key(names[2])))
	require.Equal(t, before[names[3]], branchStore.Get(key(names[3])))

	f.ctx = f.ctx.WithBlockHeight(mergeHeight + 1)
	f.app.DistrKeeper.SetPreviousProposerConsAddr(f.ctx, f.proposer)
	f.app.BeginBlock(f.ctx, f.ctx.BlockHeight(), nil, nil, false)
	guest := mergeSchedulesGuest()
	program := [32]byte{0x77}
	deployed, err := programs.EncodeDeployPayload(program, 2, 1, f.authority.Principal, sha256.Sum256(guest), nil, guest)
	require.NoError(t, err)
	receipt := f.execute(t, 0x00090001, deployed)
	require.Zero(t, receipt.ResultCode, receipt.ResultCode.Error())
	stored, err := f.app.LayerXHost.ReadProgram(f.ctx, program)
	require.NoError(t, err)
	require.Equal(t, uint32(1), stored.Version)
	require.True(t, stored.Active)

	t.Run("failed install leaves the upgrade uncommitted", func(t *testing.T) {
		other := newHostedProgramsABIFixture(t)
		otherStore := other.ctx.KVStore(other.app.GetKey(LayerXHostStoreKey))
		otherStore.Delete(key(names[0]))
		cached, _ := other.ctx.WithBlockHeight(mergeHeight).CacheContext()
		_, err := other.app.mergeUpgradePlan(cached,
			upgradetypes.Plan{Name: MergeUpgrade, Height: mergeHeight, Info: activationInfo(t)},
			other.app.UpgradeKeeper.GetModuleVersionMap(cached))
		require.ErrorContains(t, err, "history exists without its active row")
		require.Nil(t, otherStore.Get([]byte(hostedProgramsGuestABIKey)))
		require.Zero(t, other.app.UpgradeKeeper.GetDoneHeight(other.ctx, MergeUpgrade))
	})
}
