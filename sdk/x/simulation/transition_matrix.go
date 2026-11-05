package simulation

import (
	"fmt"
	"math"
	"math/rand"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/simulation"
)

// TransitionMatrix is _almost_ a left stochastic matrix.  It is technically
// not one due to not normalizing the column values.  In the future, if we want
// to find the steady state distribution, it will be quite easy to normalize
// these values to get a stochastic matrix.  Floats aren't currently used as
// the default due to non-determinism across architectures
type TransitionMatrix struct {
	weights [][]int
	// total in each column
	totals []int
	n      int
}

// CreateTransitionMatrix creates a transition matrix from the provided weights.
// TODO: Provide example usage
func CreateTransitionMatrix(weights [][]int) (simulation.TransitionMatrix, error) {
	n := len(weights)
	if n == 0 {
		return TransitionMatrix{}, fmt.Errorf("transition matrix: empty state set")
	}
	for i := 0; i < n; i++ {
		if len(weights[i]) != n {
			return TransitionMatrix{},
				fmt.Errorf("transition matrix: non-square matrix provided, error on row %d", i)
		}
	}

	totals := make([]int, n)
	owned := make([][]int, n)

	for row := 0; row < n; row++ {
		owned[row] = append([]int(nil), weights[row]...)
		for col, weight := range owned[row] {
			if weight < 0 {
				return TransitionMatrix{}, fmt.Errorf("transition matrix: negative weight at row %d column %d", row, col)
			}
			if weight > math.MaxInt-totals[col] {
				return TransitionMatrix{}, fmt.Errorf("transition matrix: total overflows in column %d", col)
			}
			totals[col] += weight
		}
	}
	for col, total := range totals {
		if total == 0 {
			return TransitionMatrix{}, fmt.Errorf("transition matrix: zero total in column %d", col)
		}
	}

	return TransitionMatrix{owned, totals, n}, nil
}

// NextState returns the next state randomly chosen using r, and the weightings
// provided in the transition matrix. It panics with a descriptive error for an invalid state or nil RNG.
func (t TransitionMatrix) NextState(r *rand.Rand, i int) int {
	if i < 0 || i >= t.n {
		panic(fmt.Sprintf("transition matrix: state %d outside [0,%d)", i, t.n))
	}
	if r == nil {
		panic("transition matrix: nil random source")
	}
	randNum := r.Intn(t.totals[i])
	for row := 0; row < t.n; row++ {
		if randNum < t.weights[row][i] {
			return row
		}

		randNum -= t.weights[row][i]
	}
	// This line should never get executed
	return -1
}

// GetMemberOfInitialState takes an initial array of weights, of size n.
// It returns a weighted random number in [0,n), panicking on invalid weights or a nil RNG.
func GetMemberOfInitialState(r *rand.Rand, weights []int) int {
	n := len(weights)
	if n == 0 {
		panic("initial state: empty state set")
	}
	if r == nil {
		panic("initial state: nil random source")
	}
	total := 0

	for i := 0; i < n; i++ {
		if weights[i] < 0 {
			panic(fmt.Sprintf("initial state: negative weight at state %d", i))
		}
		if weights[i] > math.MaxInt-total {
			panic("initial state: total overflows")
		}
		total += weights[i]
	}

	if total == 0 {
		panic("initial state: zero total")
	}
	randNum := r.Intn(total)

	for state := 0; state < n; state++ {
		if randNum < weights[state] {
			return state
		}

		randNum -= weights[state]
	}
	// This line should never get executed
	return -1
}
