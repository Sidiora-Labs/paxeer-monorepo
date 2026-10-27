//go:build cgo

package lxphost

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"

	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

func executeActivityAuthority(t *testing.T, key [32]byte) KernelAuthority {
	t.Helper()
	id, err := DIDID([]byte("did:layerx:" + hex.EncodeToString(key[:])))
	require.NoError(t, err)
	return KernelAuthority{Bound: true, Actor: id, Principal: id, VerifiedKey: key}
}

func executeActivityAssetPayload(authority KernelAuthority, salt byte) ([]byte, [32]byte) {
	saltBytes := bytes.Repeat([]byte{salt}, 32)
	preimage := append([]byte("LX:ASSET:v1"), authority.Actor[:]...)
	asset := sha256.Sum256(append(preimage, saltBytes...))
	payload := append([]byte{0, 1}, asset[:]...)
	payload = append(payload, saltBytes...)
	payload = append(payload, 1, 'T', 1, 'T', 6)
	payload = binary.BigEndian.AppendUint64(payload, 0)
	payload = binary.BigEndian.AppendUint64(payload, 1000)
	return append(payload, 1, 0), asset
}

func TestExecuteActivityBindingAuthority(t *testing.T) {
	chain := newTestChain(t)
	host := chain.host(t)
	authority := executeActivityAuthority(t, [32]byte{0x71})
	payload, asset := executeActivityAssetPayload(authority, 0x31)
	ctx := chain.context(1, bindingBlockTime, 41)
	before := chain.dump()
	unbound := authority
	unbound.Bound = false
	_, err := ExecuteActivity(ctx, host, unbound, assetRegisterType, payload, 0)
	require.Equal(t, Code(-201), err)
	require.Equal(t, before, chain.dump())

	receipt, err := ExecuteActivity(ctx, host, authority, assetRegisterType, payload, 0)
	require.NoError(t, err)
	require.Equal(t, Code(0), receipt.ResultCode)
	require.Equal(t, uint64(41), receipt.GlobalSequence)
	require.Equal(t, uint64(bindingBlockTime), receipt.Timestamp)
	require.NotEmpty(t, receipt.Encoded)
	require.True(t, bool(host.node.asset_runtime.binding_authority))
	require.True(t, signature_valid(host.node))
	require.Contains(t, receipt.Events, ReceiptEvent{Code: 1, Body: asset[:]})
	chain.commit()
	for _, reader := range []*Host{host, chain.host(t)} {
		record, err := reader.ReadAsset(ctx, asset)
		require.NoError(t, err)
		require.Equal(t, asset, record.AssetID)
		require.Equal(t, authority.Actor, record.IssuerDID)
		require.Equal(t, "T", record.Symbol)
		require.Equal(t, "T", record.Name)
		require.Equal(t, uint8(6), record.Decimals)
		require.Equal(t, bytes.Repeat([]byte{0x31}, 32), record.Salt[:])
		require.Equal(t, sdk.NewInt(1000), record.SupplyCap)
		require.Equal(t, sdk.ZeroInt(), record.TotalUnits)
	}
}

func TestExecuteActivityAccountSequence(t *testing.T) {
	chain := newTestChain(t)
	host := chain.host(t)
	authority := executeActivityAuthority(t, [32]byte{0x72})
	for sequence := uint64(0); sequence < 2; sequence++ {
		payload, asset := executeActivityAssetPayload(authority, byte(sequence+1))
		ctx := chain.context(1, bindingBlockTime, 41+sequence)
		receipt, err := ExecuteActivity(ctx, host, authority, assetRegisterType, payload, sequence)
		require.NoError(t, err)
		require.Equal(t, Code(0), receipt.ResultCode)
		record, err := host.ReadAsset(ctx, asset)
		require.NoError(t, err)
		require.Equal(t, asset, record.AssetID)
	}
	chain.commit()
	before := chain.dump()
	payload, asset := executeActivityAssetPayload(authority, 3)
	ctx := chain.context(2, bindingBlockTime+1000, 43)
	_, err := ExecuteActivity(ctx, host, authority, assetRegisterType, payload, 1)
	require.Equal(t, codeSequenceReused, err)
	require.EqualError(t, err, "LXP_ERR_SEQUENCE_REUSED")
	require.Equal(t, before, chain.dump())
	missing, err := host.ReadAsset(ctx, asset)
	require.Equal(t, Code(-402), err)
	require.EqualError(t, err, "LXP_ERR_ASSET_MISMATCH")
	require.Equal(t, AssetRecord{}, missing)
	require.Equal(t, before, chain.dump())

	receipt, err := ExecuteActivity(ctx, host, authority, assetRegisterType, payload, 2)
	require.NoError(t, err)
	require.Equal(t, Code(0), receipt.ResultCode)
	record, err := chain.host(t).ReadAsset(ctx, asset)
	require.NoError(t, err)
	require.Equal(t, asset, record.AssetID)
}

func TestResultErrorText(t *testing.T) {
	require.Nil(t, ResultError(Code(0)))
	for _, test := range []struct {
		code Code
		text string
	}{
		{Code(-201), "LXP_BAD_SIGNATURE"},
		{Code(-301), "LXP_SEQUENCE_REUSED"},
		{Code(-1003), "LXP_FATAL_SUPPLY_MISMATCH"},
	} {
		t.Run(test.text, func(t *testing.T) {
			require.EqualError(t, ResultError(test.code), test.text)
		})
	}
}
