//go:build cgo

package lxphost

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost/internal/m909fixture"
	"github.com/stretchr/testify/require"
)

func programsScheduleTestParameters(asset, signer [32]byte) ProgramsScheduleParameters {
	return ProgramsScheduleParameters{
		FeePrices: [7]uint64{1, 1, 1, 1, 1, 1, 1}, MeteringCoefficients: [9]uint64{1, 1, 1, 1, 1, 8, 8, 64, 8},
		OccupancyAssetID: asset, AuthorityKey: signer, TargetOccupancy: 1000,
		ResponseDenominator: 8, MaxChangeNumerator: 1, MaxChangeDenominator: 8, MinFee: 1, MaxFee: 100,
	}
}

func TestProgramsSchedulesInstallIsProducedByTheKernel(t *testing.T) {
	chain := newTestChain(t)
	host := chain.host(t)
	ctx := chain.context(1, resolutionBlockTime, 1)
	actor := newTestActor(t, 201)
	asset := [32]byte{0x91}
	p := programsScheduleTestParameters(asset, actor.public)
	installed, err := InstallProgramsSchedules(ctx, chain.key, p)
	require.NoError(t, err)
	require.Equal(t, ProgramsSchedulesInstalled{Fee: true, Metering: true}, installed)
	second := newTestActor(t, 202)
	produced, err := m909fixture.Records(asset, actor.id, actor.public, second.public, [32]byte{0x71})
	require.NoError(t, err)
	for _, name := range []string{"progfee/active/v1", "progfee/history/v1/\x00\x00\x00\x01", "progmet/active/v1", "progmet/history/v1/\x00\x00\x00\x01"} {
		require.Equal(t, produced[name], ctx.KVStore(chain.key).Get(moduleKVKey(9, []byte(name))), name)
	}
	fee, err := host.ReadFeeSchedule(ctx)
	require.NoError(t, err)
	require.Equal(t, produced["progfee/active/v1"], fee)
	require.Equal(t, "LXFR1", string(fee[:5]))
	require.Equal(t, uint32(1), binary.BigEndian.Uint32(fee[5:9]))
	for i, want := range p.FeePrices {
		require.Equal(t, want, binary.BigEndian.Uint64(fee[9+8*i:17+8*i]))
	}
	require.Equal(t, asset[:], fee[65:97])
	require.Equal(t, []uint64{1000, 8, 1, 8, 1, 100}, []uint64{
		binary.BigEndian.Uint64(fee[97:105]), binary.BigEndian.Uint64(fee[105:113]), binary.BigEndian.Uint64(fee[113:121]),
		binary.BigEndian.Uint64(fee[121:129]), binary.BigEndian.Uint64(fee[129:137]), binary.BigEndian.Uint64(fee[137:145]),
	})
	require.Equal(t, uint64(1), binary.BigEndian.Uint64(fee[145:153]))
	metering, err := host.ReadMeteringSchedule(ctx)
	require.NoError(t, err)
	require.Equal(t, produced["progmet/active/v1"], metering)
	require.Equal(t, "LXMR1", string(metering[:5]))
	require.Equal(t, uint32(1), binary.BigEndian.Uint32(metering[5:9]))
	for i, want := range p.MeteringCoefficients {
		require.Equal(t, want, binary.BigEndian.Uint64(metering[9+8*i:17+8*i]))
	}
	require.Equal(t, uint64(1), binary.BigEndian.Uint64(metering[81:89]))
	require.Equal(t, byte(1), metering[89])
	require.False(t, bytes.Equal(make([]byte, 32), metering[90:122]))
}

func TestProgramsSchedulesInstallKeepsStatedRows(t *testing.T) {
	for _, keep := range []string{"fee", "metering"} {
		t.Run(keep, func(t *testing.T) {
			chain := newTestChain(t)
			ctx := chain.context(1, resolutionBlockTime, 1)
			p := programsScheduleTestParameters([32]byte{0x91}, [32]byte{0x61})
			first, err := InstallProgramsSchedules(ctx, chain.key, p)
			require.NoError(t, err)
			require.Equal(t, ProgramsSchedulesInstalled{Fee: true, Metering: true}, first)
			store := ctx.KVStore(chain.key)
			prefix := "progfee/"
			if keep == "metering" {
				prefix = "progmet/"
			}
			before := map[string][]byte{}
			for _, name := range []string{prefix + "active/v1", prefix + "history/v1/\x00\x00\x00\x01"} {
				before[name] = bytes.Clone(store.Get(moduleKVKey(9, []byte(name))))
			}
			other := "progmet/"
			if keep == "metering" {
				other = "progfee/"
			}
			for _, name := range []string{other + "active/v1", other + "history/v1/\x00\x00\x00\x01"} {
				store.Delete(moduleKVKey(9, []byte(name)))
			}
			p.AuthorityKey = [32]byte{0x62}
			p.OccupancyAssetID = [32]byte{0x92}
			installed, err := InstallProgramsSchedules(ctx, chain.key, p)
			require.NoError(t, err)
			require.Equal(t, ProgramsSchedulesInstalled{Fee: keep == "metering", Metering: keep == "fee"}, installed)
			for name, value := range before {
				require.Equal(t, value, store.Get(moduleKVKey(9, []byte(name))), name)
			}
			snapshot := chain.dump()
			installed, err = InstallProgramsSchedules(ctx, chain.key, p)
			require.NoError(t, err)
			require.Equal(t, ProgramsSchedulesInstalled{}, installed)
			require.Equal(t, snapshot, chain.dump())
		})
	}
}
