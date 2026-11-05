package reservoir

import (
	"math/rand/v2"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRepairM_1156_DirtyStateSurvivesEqualReplacement(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// This fixture selects the only slot for both the changed and equal
		// replacements, then rejects the next input and selects the following one.
		fixture := rand.New(rand.NewPCG(15, 2))
		for i, want := range []int64{0, 0, 2, 0} {
			require.Equal(t, want, fixture.Int64N(int64(i+2)))
		}
		s := New[int](1, 0.5, rand.New(rand.NewPCG(15, 2)))
		s.Add(10)
		got, ok := s.Percentile()
		require.True(t, ok)
		require.Equal(t, 10, got)

		s.Add(20)
		s.Add(20)
		require.Equal(t, []int{20}, s.samples)
		require.EqualValues(t, 3, s.Seen())

		// Sleep advances only synctest's virtual clock, with no wall-clock wait.
		time.Sleep(5*time.Second - time.Nanosecond)
		got, ok = s.Percentile()
		require.True(t, ok)
		require.Equal(t, 10, got, "keep the cached value before the TTL")

		time.Sleep(time.Nanosecond)
		got, ok = s.Percentile()
		require.True(t, ok)
		require.Equal(t, 20, got, "equal replacement must not hide the changed sample at the TTL")

		s.Add(999) // The seeded fixture rejects this item.
		time.Sleep(5 * time.Second)
		got, ok = s.Percentile()
		require.True(t, ok)
		require.Equal(t, 20, got, "an unselected item must not affect the percentile")

		s.Add(30)
		got, ok = s.Percentile()
		require.True(t, ok)
		require.Equal(t, 30, got, "a later change must invalidate an already expired cache")
		require.EqualValues(t, 5, s.Seen())
	})
}
