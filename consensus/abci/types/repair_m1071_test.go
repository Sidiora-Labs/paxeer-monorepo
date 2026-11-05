package types_test

import (
	"bytes"
	"fmt"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/ed25519"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

func repairM1071Update(key int, power int64) abci.ValidatorUpdate {
	pub := ed25519.TestSecretKey([]byte(fmt.Sprintf("repair-m1071-key-%d", key))).Public()
	return tmtypes.TM2PB.ValidatorUpdate(tmtypes.NewValidator(pub, power))
}

func repairM1071Transport(t *testing.T, updates []abci.ValidatorUpdate) abci.ValidatorUpdates {
	t.Helper()
	response := abci.ResponseFinalizeBlock{ValidatorUpdates: updates}
	wire, err := response.Marshal()
	require.NoError(t, err)
	var received abci.ResponseFinalizeBlock
	require.NoError(t, received.Unmarshal(wire))
	require.Len(t, received.ValidatorUpdates, len(updates))
	for i := range updates {
		require.True(t, updates[i].PubKey.Equal(received.ValidatorUpdates[i].PubKey))
		require.Equal(t, updates[i].Power, received.ValidatorUpdates[i].Power)
	}
	return abci.ValidatorUpdates(received.ValidatorUpdates)
}

func TestRepairM_1071_EqualKeys(t *testing.T) {
	updates := repairM1071Transport(t, []abci.ValidatorUpdate{
		repairM1071Update(0, 10),
		repairM1071Update(0, 10),
		repairM1071Update(0, 0),
		repairM1071Update(0, 20),
	})
	for i := range updates {
		for j := range updates {
			require.False(t, updates.Less(i, j), "equal keys must not precede one another: %d, %d", i, j)
		}
	}
	require.True(t, sort.IsSorted(updates))
	sort.Sort(updates)
	require.True(t, sort.IsSorted(updates))
}

func TestRepairM_1071_SortingCallers(t *testing.T) {
	for _, sorter := range []struct {
		name string
		run  func(sort.Interface)
	}{
		{"Sort", sort.Sort},
		{"Stable", sort.Stable},
	} {
		for _, fixture := range []struct {
			name string
			size int
			keys int
		}{
			{"empty", 0, 1},
			{"singleton", 1, 1},
			{"equal", 64, 1},
			{"duplicates", 257, 7},
			{"distinct", 32, 32},
		} {
			t.Run(sorter.name+"/"+fixture.name, func(t *testing.T) {
				produced := make([]abci.ValidatorUpdate, fixture.size)
				for i := range produced {
					produced[i] = repairM1071Update((fixture.size-1-i)%fixture.keys, int64(i))
				}
				updates := repairM1071Transport(t, produced)
				sorter.run(updates)
				require.True(t, sort.IsSorted(updates))
				for i := range updates {
					require.False(t, updates.Less(i, i))
					for j := range updates {
						cmp := bytes.Compare(updates[i].PubKey.GetEd25519(), updates[j].PubKey.GetEd25519())
						require.Equal(t, cmp < 0, updates.Less(i, j), "ordering at %d, %d", i, j)
					}
					if i > 0 {
						cmp := bytes.Compare(updates[i-1].PubKey.GetEd25519(), updates[i].PubKey.GetEd25519())
						require.LessOrEqual(t, cmp, 0)
						if sorter.name == "Stable" && cmp == 0 {
							require.Less(t, updates[i-1].Power, updates[i].Power)
						}
					}
				}

				consumed, err := tmtypes.PB2TM.ValidatorUpdates(updates)
				require.NoError(t, err)
				require.Len(t, consumed, len(produced))
				seen := make([]bool, len(produced))
				for i, validator := range consumed {
					power := validator.VotingPower
					require.GreaterOrEqual(t, power, int64(0))
					require.Less(t, power, int64(len(produced)))
					require.False(t, seen[power], "sorting duplicated an update")
					seen[power] = true
					require.Equal(t, produced[power].PubKey.GetEd25519(), validator.PubKey.Bytes())
					require.Equal(t, updates[i].Power, power)
				}
				for i, found := range seen {
					require.True(t, found, "sorting lost update %d", i)
				}
			})
		}
	}
}
