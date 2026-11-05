package app

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPrecompileKeepersAnchorKCarriesTheKeeper(t *testing.T) {
	a := Setup(t, false, false, false)
	t.Cleanup(func() {
		if err := a.Close(); err != nil {
			t.Fatal(err)
		}
	})

	keepers := NewPrecompileKeepers(a)
	anchor := keepers.AnchorK()
	require.NotNil(t, anchor)
	require.Same(t, &a.LayerXAnchorKeeper, anchor)
	ctx := a.GetContextForDeliverTx([]byte{})
	require.Equal(t, a.LayerXAnchorKeeper.GetParams(ctx), anchor.GetParams(ctx))
}
