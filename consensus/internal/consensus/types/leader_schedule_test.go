package types

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/ed25519"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

func TestLeaderPinFromScheduleZeroActivation(t *testing.T) {
	p, s, vals := leaderPinValidators()
	a, b := [32]byte(p.Bytes()), [32]byte(s.Bytes())
	for _, tc := range []struct {
		name    string
		leaders [][32]byte
	}{
		{"nil", nil},
		{"empty", [][32]byte{}},
		{"valid", [][32]byte{a, b}},
		{"zero key", [][32]byte{{}}},
		{"duplicate key", [][32]byte{a, a}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pin, err := LeaderPinFromSchedule(0, tc.leaders)
			require.NoError(t, err)
			require.Equal(t, LeaderPin{}, pin)
			vs := tmtypes.NewValidatorSet(vals)
			rs := RoundState{Validators: vs, LeaderPin: pin}
			for _, h := range []int64{1, leaderPinActivation, math.MaxInt64} {
				for r := range int32(4) {
					rs.Height, rs.Round = h, r
					require.Equal(t, weightedLeader(vs, h, r), rs.Leader())
				}
			}
		})
	}
}

func TestLeaderPinFromScheduleConvertsKeys(t *testing.T) {
	a := [32]byte(ed25519.TestSecretKey([]byte("schedule-primary")).Public().Bytes())
	b := [32]byte(ed25519.TestSecretKey([]byte("schedule-standby")).Public().Bytes())
	c := [32]byte(ed25519.TestSecretKey([]byte("schedule-third")).Public().Bytes())
	leaders := [][32]byte{a, b, c}
	keys := make([]crypto.PubKey, len(leaders))
	for i, leader := range leaders {
		key, err := ed25519.PublicKeyFromBytes(leader[:])
		require.NoError(t, err)
		keys[i] = key
	}
	want, err := NewLeaderPin(leaderPinActivation, keys)
	require.NoError(t, err)
	pin, err := LeaderPinFromSchedule(leaderPinActivation, leaders)
	require.NoError(t, err)
	require.Equal(t, want, pin)
	for r := range int32(9) {
		key, ok := pin.Leader(leaderPinActivation, r)
		require.True(t, ok)
		require.Equal(t, keys[int(r)%len(keys)], key)
	}
	leaders[0] = b
	key, ok := pin.Leader(leaderPinActivation, 0)
	require.True(t, ok)
	require.Equal(t, keys[0], key)

	for _, tc := range []struct {
		name    string
		height  int64
		leaders [][32]byte
	}{
		{"negative activation", -1, [][32]byte{a, b}},
		{"nil list", leaderPinActivation, nil},
		{"empty list", leaderPinActivation, [][32]byte{}},
		{"zero key", leaderPinActivation, [][32]byte{a, {}}},
		{"duplicate key", leaderPinActivation, [][32]byte{a, b, a}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pin, err := LeaderPinFromSchedule(tc.height, tc.leaders)
			require.Error(t, err)
			require.Equal(t, LeaderPin{}, pin)
		})
	}
}

func TestLeaderPinAppliesFromActivation(t *testing.T) {
	var zero LeaderPin
	for _, h := range []int64{math.MinInt64, -1, 0, 1, leaderPinActivation, math.MaxInt64} {
		require.False(t, zero.Applies(h), "height=%d", h)
		_, ok := zero.Leader(h, 0)
		require.Equal(t, ok, zero.Applies(h))
	}
	key := ed25519.TestSecretKey([]byte("schedule-applies")).Public()
	for _, activation := range []int64{1, leaderPinActivation, math.MaxInt64} {
		pin, err := NewLeaderPin(activation, []crypto.PubKey{key})
		require.NoError(t, err)
		for _, h := range []int64{math.MinInt64, -1, 0, activation - 1, activation, math.MaxInt64} {
			require.Equal(t, h >= activation, pin.Applies(h), "activation=%d height=%d", activation, h)
			for _, r := range []int32{0, 1, math.MaxInt32} {
				_, ok := pin.Leader(h, r)
				require.Equal(t, ok, pin.Applies(h), "activation=%d height=%d round=%d", activation, h, r)
			}
		}
	}
}
