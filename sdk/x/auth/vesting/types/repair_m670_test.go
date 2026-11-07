package types_test

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/vesting/exported"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/vesting/types"
)

func repairM670Codec() *codec.ProtoCodec {
	registry := codectypes.NewInterfaceRegistry()
	authtypes.RegisterInterfaces(registry)
	types.RegisterInterfaces(registry)
	return codec.NewProtoCodec(registry)
}

func repairM670Accounts() []struct {
	name string
	acc  authtypes.GenesisAccount
} {
	address := sdk.AccAddress([]byte("vesting-account-0670"))
	admin := sdk.AccAddress([]byte("vesting-admin---0670"))
	base := func() *authtypes.BaseAccount {
		return authtypes.NewBaseAccount(address, nil, 7, 3)
	}
	coins := sdk.NewCoins(sdk.NewInt64Coin("uhpx", 100))
	return []struct {
		name string
		acc  authtypes.GenesisAccount
	}{
		{"base", types.NewBaseVestingAccount(base(), coins, 200, admin)},
		{"continuous", types.NewContinuousVestingAccount(base(), coins, 100, 200, admin)},
		{"delayed", types.NewDelayedVestingAccount(base(), coins, 200, admin)},
		{"periodic", types.NewPeriodicVestingAccount(base(), coins, 100, types.Periods{
			{Length: 50, Amount: sdk.NewCoins(sdk.NewInt64Coin("uhpx", 50))},
			{Length: 50, Amount: sdk.NewCoins(sdk.NewInt64Coin("uhpx", 50))},
		}, admin)},
		{"permanent", types.NewPermanentLockedAccount(base(), coins, admin)},
	}
}

func repairM670Base(t *testing.T, acc authtypes.GenesisAccount) *types.BaseVestingAccount {
	t.Helper()
	switch acc := acc.(type) {
	case *types.BaseVestingAccount:
		return acc
	case *types.ContinuousVestingAccount:
		return acc.BaseVestingAccount
	case *types.DelayedVestingAccount:
		return acc.BaseVestingAccount
	case *types.PeriodicVestingAccount:
		return acc.BaseVestingAccount
	case *types.PermanentLockedAccount:
		return acc.BaseVestingAccount
	default:
		t.Fatalf("unexpected vesting account type %T", acc)
		return nil
	}
}

func TestRepairM_670_RejectCancellationMarkers(t *testing.T) {
	for _, tc := range repairM670Accounts() {
		t.Run(tc.name, func(t *testing.T) {
			base := repairM670Base(t, tc.acc)
			require.NotEmpty(t, base.Admin)
			require.NoError(t, tc.acc.Validate())
			for _, marker := range []int64{-1 << 63, -1, 1, 99, 150, 200, 1<<63 - 1} {
				t.Run(strconv.FormatInt(marker, 10), func(t *testing.T) {
					base.CancelledTime = marker
					before := *base
					require.EqualError(t, tc.acc.Validate(), "vesting cancellation is unsupported: cancelled_time must be zero")
					require.Equal(t, before, *base, "validation must not erase unsupported state")
				})
			}
			base.CancelledTime = 0
			require.NoError(t, tc.acc.Validate())
		})
	}
}

func TestRepairM_670_GenesisTransportRejectsCancellation(t *testing.T) {
	for _, tc := range repairM670Accounts() {
		for _, marker := range []int64{-1, 0, 150} {
			t.Run(tc.name+"/"+strconv.FormatInt(marker, 10), func(t *testing.T) {
				repairM670Base(t, tc.acc).CancelledTime = marker
				producer := authtypes.NewGenesisState(authtypes.DefaultParams(), authtypes.GenesisAccounts{tc.acc})
				for _, transport := range []string{"json", "protobuf"} {
					t.Run(transport, func(t *testing.T) {
						writer, reader := repairM670Codec(), repairM670Codec()
						var recovered authtypes.GenesisState
						if transport == "json" {
							wire, err := writer.MarshalAsJSON(producer)
							require.NoError(t, err)
							require.NoError(t, reader.UnmarshalAsJSON(wire, &recovered))
						} else {
							wire, err := writer.Marshal(producer)
							require.NoError(t, err)
							require.NoError(t, reader.Unmarshal(wire, &recovered))
						}
						accounts, err := authtypes.UnpackAccounts(recovered.Accounts)
						require.NoError(t, err)
						require.Len(t, accounts, 1)
						require.NotSame(t, tc.acc, accounts[0])
						base := repairM670Base(t, accounts[0])
						require.Equal(t, marker, base.CancelledTime)
						require.Equal(t, repairM670Base(t, tc.acc).Admin, base.Admin)
						require.Equal(t, tc.acc.GetAddress(), accounts[0].GetAddress())
						if marker == 0 {
							require.NoError(t, authtypes.ValidateGenesis(recovered))
						} else {
							require.ErrorContains(t, authtypes.ValidateGenesis(recovered), "vesting cancellation is unsupported: cancelled_time must be zero")
						}
					})
				}
			})
		}
	}
}

func TestRepairM_670_ActiveSchedulesAndDelegationRoundTrip(t *testing.T) {
	for _, tc := range repairM670Accounts() {
		if tc.name == "base" {
			continue // BaseVestingAccount has no schedule; its validation is covered above.
		}
		t.Run(tc.name, func(t *testing.T) {
			account := tc.acc.(exported.VestingAccount)
			coins := sdk.NewCoins(sdk.NewInt64Coin("uhpx", 100))
			delegation := sdk.NewCoins(sdk.NewInt64Coin("uhpx", 40))
			account.TrackDelegation(time.Unix(100, 0), coins, delegation)
			producer := authtypes.NewGenesisState(authtypes.DefaultParams(), authtypes.GenesisAccounts{tc.acc})
			wire, err := repairM670Codec().MarshalAsJSON(producer)
			require.NoError(t, err)
			var recovered authtypes.GenesisState
			require.NoError(t, repairM670Codec().UnmarshalAsJSON(wire, &recovered))
			require.NoError(t, authtypes.ValidateGenesis(recovered))
			accounts, err := authtypes.UnpackAccounts(recovered.Accounts)
			require.NoError(t, err)
			require.Len(t, accounts, 1)
			require.NotSame(t, tc.acc, accounts[0])
			account = accounts[0].(exported.VestingAccount)
			require.Equal(t, delegation, account.GetDelegatedVesting())
			require.True(t, account.GetDelegatedFree().IsZero())
			require.Equal(t, repairM670Base(t, tc.acc).Admin, repairM670Base(t, accounts[0]).Admin)
			for _, timestamp := range []int64{99, 100, 149, 150, 199, 200, 201} {
				vested := int64(0)
				switch tc.name {
				case "continuous":
					if timestamp > 100 {
						vested = timestamp - 100
					}
					if timestamp >= 200 {
						vested = 100
					}
				case "periodic":
					if timestamp >= 150 {
						vested = 50
					}
					if timestamp >= 200 {
						vested = 100
					}
				case "delayed":
					if timestamp >= 200 {
						vested = 100
					}
				}
				at := time.Unix(timestamp, 0)
				require.Equal(t, sdk.NewInt(vested), account.GetVestedCoins(at).AmountOf("uhpx"), "time %d", timestamp)
				require.Equal(t, sdk.NewInt(100-vested), account.GetVestingCoins(at).AmountOf("uhpx"), "time %d", timestamp)
				locked := int64(100 - vested - 40)
				if locked < 0 {
					locked = 0
				}
				require.Equal(t, sdk.NewInt(locked), account.LockedCoins(at).AmountOf("uhpx"), "time %d", timestamp)
			}
			account.TrackUndelegation(sdk.NewCoins(sdk.NewInt64Coin("uhpx", 20)))
			require.Equal(t, sdk.NewCoins(sdk.NewInt64Coin("uhpx", 20)), account.GetDelegatedVesting())
			require.True(t, account.GetDelegatedFree().IsZero())
			require.NoError(t, accounts[0].Validate())
		})
	}
}
