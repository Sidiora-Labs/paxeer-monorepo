package baseapp

import (
	"testing"
	"time"

	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
)

func TestRepair2239367CloseReleasesApplicationStoreBeforeHandler(t *testing.T) {
	home := t.TempDir()
	app := newBaseApp(t.Name())
	app.SetCMS(newVersionedTestStore(t, home))
	key := sdk.NewKVStoreKey("shutdown")
	app.MountStores(key)
	handlerCalled := false
	app.SetCloseHandler(func() error {
		// Opening the same real store in the close handler requires the
		// application store's database and ownership lock to be closed first.
		reopened := newVersionedTestStore(t, home)
		reopened.MountStoreWithDB(key, sdk.StoreTypeIAVL, nil)
		if err := reopened.LoadLatestVersion(); err != nil {
			return err
		}
		handlerCalled = true
		return reopened.Close()
	})
	require.NoError(t, app.LoadLatestVersion())
	done := make(chan error, 1)
	go func() { done <- app.Close() }()
	select {
	case err := <-done:
		require.NoError(t, err)
		require.True(t, handlerCalled)
	case <-time.After(2 * time.Second):
		t.Fatal("application store close did not finish before the close handler within two seconds")
	}
}
