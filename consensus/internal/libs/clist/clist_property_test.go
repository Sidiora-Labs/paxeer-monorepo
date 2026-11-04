package clist_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/libs/clist"
)

func TestCListProperties(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		m := &clistModel{}
		m.init(t)
		t.Repeat(rapid.StateMachineActions(m))
	})
}

// clistModel is used by the rapid state machine testing framework.
// clistModel contains both the clist that is being tested and a slice of *clist.CElements
// that will be used to model the expected clist behavior.
type clistModel struct {
	clist *clist.CList[string]

	model []*clist.CElement[string]
}

// init is called when the test starts to initialize the data that will be used
// in the state machine test.
func (m *clistModel) init(t *rapid.T) {
	m.clist = clist.New[string]()
	m.model = []*clist.CElement[string]{}
}

// PushBack defines an action that will be randomly selected across by the rapid state
// machines testing library. Every call to PushBack calls PushBack on the clist and
// performs a similar action on the model data.
func (m *clistModel) PushBack(t *rapid.T) {
	value := rapid.String().Draw(t, "value")
	el := m.clist.PushBack(value)
	m.model = append(m.model, el)
}

// Remove defines an action that will be randomly selected across by the rapid state
// machine testing library. Every call to Remove selects an element from the model
// and calls Remove on the CList with that element. The same element is removed from
// the model to keep the objects in sync.
func (m *clistModel) Remove(t *rapid.T) {
	if len(m.model) == 0 {
		return
	}
	ix := rapid.IntRange(0, len(m.model)-1).Draw(t, "index")
	value := m.model[ix]
	m.model = append(m.model[:ix], m.model[ix+1:]...)
	require.Equal(t, value.Value(), m.clist.Remove(value))
	require.True(t, value.Removed())
}

// Check is a method required by the rapid state machine testing library.
// Check is run after each action and is used to verify that the state of the object,
// in this case a clist.CList matches the state of the objec.
func (m *clistModel) Check(t *rapid.T) {
	require.Equal(t, len(m.model), m.clist.Len())
	if len(m.model) == 0 {
		require.Nil(t, m.clist.Front())
		require.Nil(t, m.clist.Back())
		return
	}
	require.Equal(t, m.model[0], m.clist.Front())
	require.Equal(t, m.model[len(m.model)-1], m.clist.Back())

	iter := m.clist.Front()
	for _, val := range m.model {
		require.Equal(t, val, iter)
		require.False(t, iter.Removed())
		iter = iter.Next()
	}
	require.Nil(t, iter)

	iter = m.clist.Back()
	for i := len(m.model) - 1; i >= 0; i-- {
		require.Equal(t, m.model[i], iter)
		iter = iter.Prev()
	}
	require.Nil(t, iter)
}
