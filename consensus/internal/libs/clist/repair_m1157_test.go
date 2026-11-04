package clist_test

import (
	"context"
	"fmt"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/libs/clist"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils/scope"
)

func TestRepairM_1157_ZeroValueLifecycle(t *testing.T) {
	for _, constructor := range []struct {
		name    string
		newList func() *clist.CList[int]
	}{
		{"zero", func() *clist.CList[int] { return new(clist.CList[int]) }},
		{"new", clist.New[int]},
	} {
		t.Run(constructor.name, func(t *testing.T) {
			l := constructor.newList()
			require.Zero(t, l.Len())
			require.Nil(t, l.Front())
			require.Nil(t, l.Back())

			first := l.PushBack(11)
			require.Equal(t, 11, first.Value())
			require.Equal(t, 1, l.Len())
			require.Same(t, first, l.Front())
			require.Same(t, first, l.Back())
			front, err := l.WaitFront(t.Context())
			require.NoError(t, err)
			require.Same(t, first, front)
			require.Equal(t, 11, l.Remove(first))
			require.True(t, first.Removed())
			next, err := first.NextWait(t.Context())
			require.ErrorIs(t, err, clist.ErrRemoved)
			require.Nil(t, next)
			require.Zero(t, l.Len())
			require.Nil(t, l.Front())
			require.Nil(t, l.Back())

			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			front, err = l.WaitFront(ctx)
			require.ErrorIs(t, err, context.Canceled)
			require.Nil(t, front)

			second := l.PushBack(22)
			tail := l.PushBack(33)
			require.Same(t, tail, second.Next())
			require.Same(t, second, tail.Prev())
			l.Clear()
			require.Zero(t, l.Len())
			require.Nil(t, l.Front())
			require.Nil(t, l.Back())
			require.True(t, second.Removed())
			require.True(t, tail.Removed())
			next, err = tail.NextWait(t.Context())
			require.ErrorIs(t, err, clist.ErrRemoved)
			require.Nil(t, next)
			l.Clear()
			last := l.PushBack(44)
			front, err = l.WaitFront(t.Context())
			require.NoError(t, err)
			require.Same(t, last, front)
			require.Equal(t, 44, l.Remove(last))
		})
	}
}

func TestRepairM_1157_ZeroValueWaitFront(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var l clist.CList[int]
		err := scope.Run(t.Context(), func(ctx context.Context, s scope.Scope) error {
			if err := repairM1157WaitForPush(ctx, s, &l); err != nil {
				return err
			}
			l.Remove(l.Front())
			if err := repairM1157WaitForPush(ctx, s, &l); err != nil {
				return err
			}
			l.Clear()
			return repairM1157WaitForPush(ctx, s, &l)
		})
		require.NoError(t, err)
	})
}

func repairM1157WaitForPush(ctx context.Context, s scope.Scope, l *clist.CList[int]) error {
	type result struct {
		element *clist.CElement[int]
		err     error
	}
	const waiters = 4
	results := make(chan result, waiters)
	for range waiters {
		s.SpawnBg(func() error {
			element, err := l.WaitFront(ctx)
			results <- result{element, err}
			return nil
		})
	}
	synctest.Wait()
	select {
	case r := <-results:
		return fmt.Errorf("empty list waiter returned before push: %v, %w", r.element, r.err)
	default:
	}
	l.Clear()
	synctest.Wait()
	select {
	case r := <-results:
		return fmt.Errorf("empty list waiter returned after clear: %v, %w", r.element, r.err)
	default:
	}
	expected := l.PushBack(57)
	synctest.Wait()
	for range waiters {
		select {
		case r := <-results:
			if r.err != nil || r.element != expected {
				return fmt.Errorf("waiter did not receive pushed element: %v, %w", r.element, r.err)
			}
		default:
			return fmt.Errorf("waiter remained blocked after push")
		}
	}
	return nil
}

func TestRepairM_1157_PropertyStateMachine(t *testing.T) {
	TestCListProperties(t)
}
