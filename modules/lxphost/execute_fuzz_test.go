//go:build cgo

package lxphost

import (
	"bytes"
	"crypto/ed25519"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func FuzzHostExecutePayload(f *testing.F) {
	const (
		actorSeed = 21
		assetSalt = 0x33
		blockTime = int64(1700000000000)
	)
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{actorSeed}, ed25519.SeedSize))
	did := []byte("did:layerx:" + hex.EncodeToString(private.Public().(ed25519.PublicKey)))
	actorID, err := DIDID(did)
	require.NoError(f, err)
	registerPayload, assetID := executeActivityAssetPayload(KernelAuthority{Actor: actorID}, assetSalt)
	accountID, err := kernelAccountID([]byte("agent:" + string(did) + ":asset:" + hex.EncodeToString(assetID[:])))
	require.NoError(f, err)
	mintPayload := append(append([]byte{0, 1}, assetID[:]...), accountID[:]...)
	mintPayload = binary.BigEndian.AppendUint64(binary.BigEndian.AppendUint64(mintPayload, 0), 250)

	f.Add(false, registerPayload)
	f.Add(true, mintPayload)
	f.Add(false, []byte{})
	f.Add(true, make([]byte, 4096))

	f.Fuzz(func(t *testing.T, mint bool, payload []byte) {
		chain := newTestChain(t)
		host := chain.host(t)
		actor := newTestActor(t, actorSeed)
		asset := newTestAsset(t, actor, assetSalt)
		registered := execute(t, host, chain.context(1, blockTime, 41),
			actor.registerAsset(t, host, 0, blockTime, asset, assetSalt, 1000), actor.authority())
		require.Equal(t, Code(0), registered.ResultCode)
		opened := execute(t, host, chain.context(1, blockTime, 42),
			actor.openAssetAccount(t, host, 1, blockTime, asset), actor.authority())
		require.Equal(t, Code(0), opened.ResultCode)
		chain.commit()

		activityType := uint32(assetRegisterType)
		if mint {
			activityType = assetMintType
		}
		activity := actor.activity(t, host, activityType, 2, blockTime+1000, payload)
		ctx := chain.context(2, blockTime+1000, 43)
		before := chain.dump()
		var receipt Receipt
		var executeErr error
		require.NotPanics(t, func() {
			receipt, executeErr = host.Execute(ctx, activity, actor.authority())
		})
		require.Greater(t, receipt.ResultCode, codeFatalInvariant)
		if executeErr != nil {
			require.Equal(t, before, chain.dump())
			var code Code
			if errors.As(executeErr, &code) {
				require.Greater(t, code, codeFatalInvariant)
			}
		}
	})
}
