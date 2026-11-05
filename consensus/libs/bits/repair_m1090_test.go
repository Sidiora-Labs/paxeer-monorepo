package bits

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils/scope"
	tmprotobits "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/libs/bits"
)

func repairM1090Array(t *testing.T, bits []bool) *BitArray {
	t.Helper()
	produced := NewBitArray(len(bits))
	for i, set := range bits {
		if !produced.SetIndex(i, set) {
			t.Fatalf("producer rejected bit %d", i)
		}
	}
	encoded, err := produced.ToProto().Marshal()
	if err != nil {
		t.Fatal(err)
	}
	var message tmprotobits.BitArray
	if err := message.Unmarshal(encoded); err != nil {
		t.Fatal(err)
	}
	consumed := new(BitArray)
	if err := consumed.FromProto(&message); err != nil {
		t.Fatal(err)
	}
	if err := repairM1090CheckBits(consumed, bits); err != nil {
		t.Fatal(err)
	}
	return consumed
}

func repairM1090CheckBits(got *BitArray, want []bool) error {
	if got == nil || got.Size() != len(want) {
		return fmt.Errorf("got size %d, want %d", got.Size(), len(want))
	}
	for i, set := range want {
		if got.GetIndex(i) != set {
			return fmt.Errorf("bit %d: got %t, want %t", i, got.GetIndex(i), set)
		}
	}
	return nil
}

func repairM1090Bounded(t *testing.T, work func() error) {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- work() }()
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-timer.C:
		t.Fatal("two-array operations did not terminate within five seconds")
	case <-t.Context().Done():
		t.Fatal(t.Context().Err())
	}
}

func TestRepairM_1090_LockOrder(t *testing.T) {
	operations := []struct {
		name  string
		apply func(*BitArray, *BitArray) *BitArray
		bit   func(bool, bool) bool
	}{
		{"Or", (*BitArray).Or, func(a, b bool) bool { return a || b }},
		{"And", (*BitArray).And, func(a, b bool) bool { return a && b }},
		{"Sub", (*BitArray).Sub, func(a, b bool) bool { return a && !b }},
		{"Update", func(a, b *BitArray) *BitArray { a.Update(b); return a }, func(a, b bool) bool { return b }},
	}
	for _, op := range operations {
		for _, self := range []bool{true, false} {
			kind := "reciprocal"
			if self {
				kind = "self"
			}
			t.Run(op.name+"/"+kind, func(t *testing.T) {
				left, right := make([]bool, 129), make([]bool, 129)
				for i := range left {
					left[i] = i%3 == 0 || i == 128
					right[i] = i%5 == 0 || i == 64
				}
				// Reciprocal updates use equal values so every observed result is defined.
				if op.name == "Update" {
					copy(right, left)
				}
				a, b := repairM1090Array(t, left), repairM1090Array(t, right)
				if self {
					b, right = a, left
				}
				ctx := t.Context()
				repairM1090Bounded(t, func() error {
					return scope.Run(ctx, func(ctx context.Context, s scope.Scope) error {
						start := make(chan struct{})
						for worker := range 4 {
							first, second, firstBits, secondBits := a, b, left, right
							if worker%2 != 0 {
								first, second, firstBits, secondBits = b, a, right, left
							}
							want := make([]bool, len(firstBits))
							for i := range want {
								want[i] = op.bit(firstBits[i], secondBits[i])
							}
							s.Spawn(func() error {
								<-start
								for range 256 {
									if err := ctx.Err(); err != nil {
										return err
									}
									if err := repairM1090CheckBits(op.apply(first, second), want); err != nil {
										return fmt.Errorf("worker %d: %w", worker, err)
									}
									runtime.Gosched()
								}
								return nil
							})
						}
						close(start)
						return nil
					})
				})
				if err := repairM1090CheckBits(a, left); err != nil {
					t.Fatal(err)
				}
				if err := repairM1090CheckBits(b, right); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestRepairM_1090_OrRightTail(t *testing.T) {
	for _, leftSize := range []int{1, 64, 65, 129} {
		for _, rightSize := range []int{1, 64, 65, 129} {
			for _, pattern := range []string{"boundary", "dense", "sparse"} {
				t.Run(fmt.Sprintf("%d_%d/%s", leftSize, rightSize, pattern), func(t *testing.T) {
					left, right := make([]bool, leftSize), make([]bool, rightSize)
					for i := range left {
						left[i] = pattern == "dense" || pattern == "sparse" && i%3 == 0
					}
					for i := range right {
						right[i] = i == rightSize-1 || i == 0 || i == 63 || i == 64 || i == 127 || i == 128 || pattern == "dense" || pattern == "sparse" && i%5 == 0
					}
					a, b := repairM1090Array(t, left), repairM1090Array(t, right)
					want := make([]bool, max(leftSize, rightSize))
					for i := range want {
						want[i] = i < leftSize && left[i] || i < rightSize && right[i]
					}
					var union *BitArray
					repairM1090Bounded(t, func() error {
						union = a.Or(b)
						if err := repairM1090CheckBits(union, want); err != nil {
							return err
						}
						return repairM1090CheckBits(b.Or(a), want)
					})
					// Exercise the production JSON transport and a consumer of the union.
					encoded, err := json.Marshal(union)
					if err != nil {
						t.Fatal(err)
					}
					var consumed *BitArray
					if err := json.Unmarshal(encoded, &consumed); err != nil {
						t.Fatal(err)
					}
					if err := repairM1090CheckBits(consumed, want); err != nil {
						t.Fatal(err)
					}
					if err := repairM1090CheckBits(a, left); err != nil {
						t.Fatal(err)
					}
					if err := repairM1090CheckBits(b, right); err != nil {
						t.Fatal(err)
					}
					union.SetIndex(0, !union.GetIndex(0))
					if err := repairM1090CheckBits(a, left); err != nil {
						t.Fatalf("union aliases left operand: %v", err)
					}
					if err := repairM1090CheckBits(b, right); err != nil {
						t.Fatalf("union aliases right operand: %v", err)
					}
				})
			}
		}
	}
}

func TestRepairM_1090_UpdateAndNilContracts(t *testing.T) {
	for _, sizes := range [][2]int{{1, 129}, {129, 1}, {64, 65}, {65, 64}} {
		t.Run(fmt.Sprintf("%d_%d", sizes[0], sizes[1]), func(t *testing.T) {
			left, right := make([]bool, sizes[0]), make([]bool, sizes[1])
			for i := range left {
				left[i] = i%2 == 0
			}
			for i := range right {
				right[i] = i%3 == 0
			}
			a, b := repairM1090Array(t, left), repairM1090Array(t, right)
			want := append([]bool(nil), left...)
			// Update copies whole overlapping words, retaining any later receiver words.
			for i := 0; i < min(len(left), ((len(right)+63)/64)*64); i++ {
				want[i] = i < len(right) && right[i]
			}
			repairM1090Bounded(t, func() error { a.Update(b); return repairM1090CheckBits(a, want) })
			b.SetIndex(0, !right[0])
			if err := repairM1090CheckBits(a, want); err != nil {
				t.Fatal(err)
			}
		})
	}
	var empty *BitArray
	var zero BitArray
	a := repairM1090Array(t, []bool{true})
	repairM1090Bounded(t, func() error {
		if empty.Or(empty) != nil || empty.And(a) != nil || a.And(empty) != nil || empty.Sub(a) != nil || a.Sub(empty) != nil {
			return fmt.Errorf("nil operand contract changed")
		}
		empty.Update(a)
		a.Update(empty)
		empty.Update(empty)
		zero.Update(&zero)
		for _, result := range []*BitArray{zero.Or(&zero), zero.And(&zero), zero.Sub(&zero), &zero} {
			if err := repairM1090CheckBits(result, []bool{}); err != nil {
				return err
			}
		}
		if err := repairM1090CheckBits(empty.Or(a), []bool{true}); err != nil {
			return err
		}
		if err := repairM1090CheckBits(a.Or(empty), []bool{true}); err != nil {
			return err
		}
		return repairM1090CheckBits(a, []bool{true})
	})
}
