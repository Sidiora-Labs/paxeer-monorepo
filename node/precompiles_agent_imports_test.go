package app

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPrecompileKeepersExposePayAndAgent(t *testing.T) {
	a := Setup(t, false, false, false)
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	keepers := NewPrecompileKeepers(a)

	require.NotNil(t, a.LayerXPayKeeper)
	require.NotNil(t, keepers.LayerXPayK())
	require.NotNil(t, keepers.LayerXAgentK())
	require.Same(t, a.LayerXPayKeeper, keepers.LayerXPayK())
	require.Same(t, &a.LayerXAgentKeeper.Keeper, keepers.LayerXAgentK())
}

func TestPrecompileKeepersExposeSharedHost(t *testing.T) {
	a := Setup(t, false, false, false)
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	keepers := NewPrecompileKeepers(a)

	require.NotNil(t, a.LayerXHost)
	require.NotNil(t, keepers.LayerXHost())
	require.Same(t, a.LayerXHost, keepers.LayerXHost())
	require.Same(t, a.LayerXPayKeeper.Host(), keepers.LayerXHost())
	require.Same(t, a.LayerXAgentKeeper.Host(), keepers.LayerXHost())
	require.NotNil(t, keepers.XWebK())
	require.Same(t, &a.XWebKeeper, keepers.XWebK())
}
