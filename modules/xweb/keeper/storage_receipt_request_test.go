package keeper_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
)

func TestRequestAdmitsStorageReceiptKind(t *testing.T) {
	s := newSuite(t, true)
	payload := types.StorageReceiptPayload{
		ObjectDigest: sha256.Sum256([]byte("stored object")),
		BaseURL:      "https://storage.example/bucket",
	}
	body, err := types.EncodeStorageReceiptPayload(payload)
	require.NoError(t, err)
	before := s.balance(s.account(requester))
	id, err := s.k.Request(s.ctx, requester, types.KindStorageReceipt, body, callbackGas, sdk.NewInt(fee))
	require.NoError(t, err)
	require.Equal(t, uint64(1), id)
	request, found := s.k.GetRequest(s.ctx, id)
	require.True(t, found)
	require.Equal(t, types.KindStorageReceipt, request.Kind)
	require.Equal(t, types.Keccak(body), request.PayloadHash)
	require.Equal(t, types.LevelMajority, request.Level)
	require.Equal(t, types.Address20{}, request.Attestor)
	require.NoError(t, request.Validate())
	require.Equal(t, before.Sub(sdk.NewInt(fee)), s.balance(s.account(requester)))
	require.Equal(t, sdk.NewInt(fee), s.balance(s.k.ModuleAddress()))
	events := s.events(types.EventRequested)
	require.Len(t, events, 1)
	require.Equal(t, "4", attribute(events[0], types.AttributeKind))
	require.Equal(t, hex.EncodeToString(body), attribute(events[0], types.AttributePayload))
	genesis := s.k.ExportGenesis(s.ctx)
	require.NoError(t, genesis.Validate())
	require.Equal(t, []types.Request{request}, genesis.Requests)
	rawGenesis, err := json.Marshal(genesis)
	require.NoError(t, err)
	var imported types.GenesisState
	require.NoError(t, json.Unmarshal(rawGenesis, &imported))
	restarted := newSuite(t, false)
	restarted.k.InitGenesis(restarted.ctx, imported)
	restored, found := restarted.k.GetRequest(restarted.ctx, id)
	require.True(t, found)
	require.Equal(t, request, restored)
	require.Equal(t, genesis, restarted.k.ExportGenesis(restarted.ctx))

	single := request
	single.Level, single.Attestor = types.LevelSingle, s.attestors[0].Signer
	require.ErrorIs(t, single.Validate(), types.ErrInvalidLevel)

	zeroDigest := bytes.Clone(body)
	copy(zeroDigest[:32], make([]byte, 32))
	for name, invalid := range map[string][]byte{
		"http":         bytes.Replace(body, []byte("https://"), []byte("http://"), 1),
		"zero digest":  zeroDigest,
		"short digest": body[:31],
	} {
		t.Run(name, func(t *testing.T) {
			refusedID, err := s.k.Request(s.ctx, requester, types.KindStorageReceipt, invalid, callbackGas, sdk.NewInt(fee))
			require.ErrorIs(t, err, types.ErrInvalidPayload)
			require.Zero(t, refusedID)
			require.Equal(t, id, s.k.Nonce(s.ctx))
			_, found := s.k.GetRequest(s.ctx, id+1)
			require.False(t, found)
			require.Equal(t, before.Sub(sdk.NewInt(fee)), s.balance(s.account(requester)))
			require.Equal(t, sdk.NewInt(fee), s.balance(s.k.ModuleAddress()))
			require.Len(t, s.events(types.EventRequested), 1)
		})
	}
}
