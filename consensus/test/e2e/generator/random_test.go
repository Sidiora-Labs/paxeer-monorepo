package main

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCombinations(t *testing.T) {
	input := map[string][]interface{}{
		"bool":   {false, true},
		"int":    {1, 2, 3},
		"string": {"foo", "bar"},
	}

	c := combinations(input)
	assert.Equal(t, []map[string]interface{}{
		{"bool": false, "int": 1, "string": "foo"},
		{"bool": false, "int": 1, "string": "bar"},
		{"bool": false, "int": 2, "string": "foo"},
		{"bool": false, "int": 2, "string": "bar"},
		{"bool": false, "int": 3, "string": "foo"},
		{"bool": false, "int": 3, "string": "bar"},
		{"bool": true, "int": 1, "string": "foo"},
		{"bool": true, "int": 1, "string": "bar"},
		{"bool": true, "int": 2, "string": "foo"},
		{"bool": true, "int": 2, "string": "bar"},
		{"bool": true, "int": 3, "string": "foo"},
		{"bool": true, "int": 3, "string": "bar"},
	}, c)
}

func TestUniformSetChoice(t *testing.T) {
	set := uniformSetChoice([]string{"a", "b", "c"})
	r := rand.New(rand.NewSource(2384))

	for i := 0; i < 100; i++ {
		t.Run(fmt.Sprintf("Iteration%03d", i), func(t *testing.T) {
			set = append(set, t.Name())

			t.Run("ChooseAtLeastSubset", func(t *testing.T) {
				require.True(t, len(set.ChooseAtLeast(r, 1)) >= 1)
				require.True(t, len(set.ChooseAtLeast(r, 2)) >= 2)
				require.True(t, len(set.ChooseAtLeast(r, len(set)/2)) >= len(set)/2)
			})
			t.Run("ChooseAtLeastEqualOrGreaterToLength", func(t *testing.T) {
				require.Len(t, set.ChooseAtLeast(r, len(set)), len(set))
				require.Len(t, set.ChooseAtLeast(r, len(set)+1), len(set))
				require.Len(t, set.ChooseAtLeast(r, len(set)*10), len(set))
			})
			t.Run("ChooseSingle", func(t *testing.T) {
				require.True(t, len(set.Choose(r)) >= 1)
			})
		})
	}
}

func TestUniformSetChoiceCardinalities(t *testing.T) {
	assertUniformSetChoiceCardinalities(t)
}

func assertUniformSetChoiceCardinalities(t *testing.T) {
	t.Helper()
	for size := 0; size <= 6; size++ {
		set := make(uniformSetChoice, size)
		for i := range set {
			set[i] = fmt.Sprintf("item%d", i)
		}
		for minimum := 0; minimum <= size+1; minimum++ {
			t.Run(fmt.Sprintf("size%d/minimum%d", size, minimum), func(t *testing.T) {
				assertUniformSetChoiceRange(t, set, minimum, func(r *rand.Rand) []string {
					return set.ChooseAtLeast(r, minimum)
				})
			})
		}
		t.Run(fmt.Sprintf("size%d/Choose", size), func(t *testing.T) {
			assertUniformSetChoiceRange(t, set, 1, set.Choose)
		})
	}
}

func assertUniformSetChoiceRange(t *testing.T, set uniformSetChoice, minimum int, choose func(*rand.Rand) []string) {
	t.Helper()
	r := rand.New(rand.NewSource(2384))
	seen := make(map[int]bool)
	for i := 0; i < 512; i++ {
		choices := choose(r)
		require.GreaterOrEqual(t, len(choices), min(minimum, len(set)))
		require.LessOrEqual(t, len(choices), len(set))
		members := make(map[string]bool)
		for _, choice := range choices {
			require.Contains(t, set, choice)
			require.False(t, members[choice], "duplicate choice %q", choice)
			members[choice] = true
		}
		seen[len(choices)] = true
	}
	for count := min(minimum, len(set)); count <= len(set); count++ {
		require.True(t, seen[count], "cardinality %d is unreachable", count)
	}
}
