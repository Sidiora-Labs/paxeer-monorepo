package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"math/big"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	programs "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxprograms"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
)

func programsOccupancyPayerAsset(f *hostedProgramsABIFixture) [32]byte {
	actor := f.authority.Principal
	salt := bytes.Repeat([]byte{0x55}, 32)
	return sha256.Sum256(append(append([]byte("LX:ASSET:v1"), actor[:]...), salt...))
}

func programsOccupancyPayerMain(t *testing.T, f *hostedProgramsABIFixture, asset [32]byte) {
	t.Helper()
	mainName, mainID, err := lxphost.MainAccountID(f.authority.VerifiedKey)
	require.NoError(t, err)
	_, assetID, err := lxphost.AssetAccountID(f.authority.VerifiedKey, asset)
	require.NoError(t, err)
	field := func(out, value []byte) []byte {
		return append(binary.BigEndian.AppendUint32(out, uint32(len(value))), value...)
	}
	main := field(nil, mainID[:])
	main = field(main, []byte(mainName))
	main = append(main, 1)
	main = binary.BigEndian.AppendUint64(main, 0)
	main = binary.BigEndian.AppendUint64(main, 500000000)
	main = field(main, asset[:])
	main = append(main, 1)
	main = binary.BigEndian.AppendUint64(main, 0)
	main = binary.BigEndian.AppendUint64(main, 1)
	main = append(main, 0, 0)
	main = field(main, f.authority.VerifiedKey[:])
	store := f.ctx.KVStore(f.app.GetKey(LayerXHostStoreKey))
	store.Set(append([]byte{6}, mainID[:]...), append(main, 1))
	store.Delete(append([]byte{6}, assetID[:]...))
	_, err = f.app.LayerXHost.ReadAccount(f.ctx, mainName, mainID)
	require.NoError(t, err)
}

func programsOccupancyPayerGuest() []byte {
	section := func(id byte, body []byte) []byte { return append([]byte{id, byte(len(body))}, body...) }
	label := func(value string) []byte { return append([]byte{byte(len(value))}, value...) }
	guest := []byte{0, 97, 115, 109, 1, 0, 0, 0}
	guest = append(guest, section(1, []byte{3,
		0x60, 4, 0x7f, 0x7f, 0x7f, 0x7f, 1, 0x7f,
		0x60, 1, 0x7f, 1, 0x7f,
		0x60, 2, 0x7f, 0x7f, 1, 0x7f})...)
	imports := append([]byte{1}, label("layerx_v1")...)
	imports = append(imports, label("storage_write")...)
	guest = append(guest, section(2, append(imports, 0, 0))...)
	guest = append(guest, section(3, []byte{2, 1, 2})...)
	guest = append(guest, section(5, []byte{1, 1, 1, 1})...)
	exports := append([]byte{3}, label("layerx_reserve")...)
	exports = append(exports, 0, 1)
	exports = append(exports, label("layerx_call")...)
	exports = append(exports, 0, 2)
	exports = append(exports, label("memory")...)
	guest = append(guest, section(7, append(exports, 2, 0))...)
	call := []byte{0, 0x41, 0, 0x41, 1, 0x41, 1, 0x41, 1, 0x10, 0, 0x0b}
	code := append([]byte{2, 4, 0, 0x41, 0, 0x0b, byte(len(call))}, call...)
	guest = append(guest, section(10, code)...)
	guest = append(guest, section(11, []byte{1, 0, 0x41, 0, 0x0b, 2, 'k', 'v'})...)
	return guest
}

func programsOccupancyPayerDeploy(t *testing.T, f *hostedProgramsABIFixture) {
	t.Helper()
	guest := programsOccupancyPayerGuest()
	hash := sha256.Sum256(guest)
	f.program = [32]byte{0x79}
	deploy, err := programs.EncodeDeployPayload(f.program, 2, 1, f.authority.Principal, hash, nil, guest)
	require.NoError(t, err)
	receipt := f.execute(t, 0x00090001, deploy)
	require.Zero(t, receipt.ResultCode, receipt.ResultCode.Error())
	upgrade, err := programs.EncodeUpgradePayload(f.program, 5, 0, hash, hash, nil, nil, guest)
	require.NoError(t, err)
	receipt = f.execute(t, 0x00090002, upgrade)
	require.Zero(t, receipt.ResultCode, receipt.ResultCode.Error())
}

func programsOccupancyPayerCall(t *testing.T, f *hostedProgramsABIFixture) {
	t.Helper()
	accessSet := append([]byte("LayerX/programs/access-set/v1\x00"), 0, 1)
	accessSet = append(accessSet, f.program[:]...)
	accessSet = append(accessSet, 0)
	accessSet = append(accessSet, f.authority.Principal[:]...)
	accessSet = append(accessSet, 1, 0, 0, 1, 'k', 0, 0, 0, 0)
	access := append([]byte("LayerX/programs/access-declaration/v1\x00"), 1)
	access = binary.BigEndian.AppendUint32(access, uint32(len(accessSet)))
	access = append(access, accessSet...)
	budget := [7]uint64{1000000, 16777216, 1048576, 1048576, 64, 1048576, 4096}
	payload, err := programs.EncodeCallPayload(f.program, 5, "layerx_call", nil, []byte{0, 1, 2}, access, 16, budget)
	require.NoError(t, err)
	receipt := f.execute(t, 0x00090003, payload)
	require.Zero(t, receipt.ResultCode, receipt.ResultCode.Error())
	require.NotNil(t, receipt.ProgramOutcome)
	require.Zero(t, receipt.ProgramOutcome.Status)
	decoded, err := codec.DecodeReceipt(receipt.Encoded)
	require.NoError(t, err)
	require.NotNil(t, decoded.ProgramOutcome)
	require.Positive(t, decoded.ProgramOutcome.StorageWriteBytes)
}

func programsOccupancyPayerCallBlock(t *testing.T, f *hostedProgramsABIFixture) {
	t.Helper()
	f.ctx = f.ctx.WithBlockHeight(mergeHeight)
	req := programsBlockEndRequest(f, mergeHeight)
	proposal, err := f.app.ProcessProposal(context.Background(), &abci.RequestProcessProposal{
		Hash: req.Hash, Header: req.Header,
	})
	require.NoError(t, err)
	require.Equal(t, abci.ResponseProcessProposal_ACCEPT, proposal.Status)
	info := f.app.GetOptimisticProcessingInfo()
	require.NotNil(t, info.Completion)
	select {
	case <-info.Completion:
	case <-time.After(30 * time.Second):
		t.Fatal("optimistic block processing did not finish")
	}
	info = f.app.GetOptimisticProcessingInfo()
	require.False(t, info.Aborted)
	f.ctx = info.StateCtx.WithBlockHeader(*req.Header)
	programsOccupancyPayerCall(t, f)
	_, err = f.app.FinalizeBlock(context.Background(), req)
	require.NoError(t, err)
	programsBlockEndCommit(t, f)
}

func programsOccupancyPayerCharge(t *testing.T, encoded []byte, height int64, asset, principal [32]byte) sdk.Int {
	t.Helper()
	domain := []byte("LXP/programs/occupancy-receipt/v2\x00")
	require.True(t, bytes.HasPrefix(encoded, domain))
	require.GreaterOrEqual(t, len(encoded), len(domain)+8+8+4+4+56+32+64+2+81+160)
	require.Equal(t, uint64(height), binary.BigEndian.Uint64(encoded[len(domain):]))
	assetAt := len(domain) + 8 + 8 + 4 + 4 + 56
	require.Equal(t, asset[:], encoded[assetAt:assetAt+32])
	payersAt := assetAt + 32 + 64
	require.Equal(t, uint16(1), binary.BigEndian.Uint16(encoded[payersAt:]))
	payer := encoded[payersAt+2 : payersAt+2+81]
	require.Equal(t, principal[:], payer[:32])
	due := sdk.NewIntFromBigInt(new(big.Int).SetBytes(payer[32:48]))
	paid := sdk.NewIntFromBigInt(new(big.Int).SetBytes(payer[48:64]))
	require.True(t, paid.IsPositive())
	require.Equal(t, due, paid)
	require.Equal(t, make([]byte, 16), payer[64:80])
	require.Zero(t, payer[80])
	return paid
}

func programsOccupancyPayerBlocks(t *testing.T, main bool) {
	t.Helper()
	f := newHostedProgramsABIFixture(t, 1)
	asset := programsOccupancyPayerAsset(f)
	assetName, assetID, err := lxphost.AssetAccountID(f.authority.VerifiedKey, asset)
	require.NoError(t, err)
	mainName, mainID, err := lxphost.MainAccountID(f.authority.VerifiedKey)
	require.NoError(t, err)
	name, id := assetName, assetID
	if main {
		programsOccupancyPayerMain(t, f, asset)
		name, id = mainName, mainID
		_, err = f.app.LayerXHost.ReadAccount(f.ctx, assetName, assetID)
		require.Error(t, err)
	} else {
		mainAccount, err := f.app.LayerXHost.ReadAccount(f.ctx, mainName, mainID)
		require.NoError(t, err)
		require.NotEqual(t, asset, mainAccount.AssetID)
	}
	programsOccupancyPayerDeploy(t, f)
	programsBlockEndPrepareMerge(t, f)
	programsOccupancyPayerCallBlock(t, f)
	key := f.app.GetKey(LayerXHostStoreKey)
	principal := f.authority.Principal
	for height := mergeHeight + 1; height <= mergeHeight+3; height++ {
		before, err := f.app.LayerXHost.ReadAccount(f.ctx, name, id)
		require.NoError(t, err)
		preview, _ := f.ctx.WithBlockHeight(height).CacheContext()
		sequence, err := lxphost.NextSequence(preview, key)
		require.NoError(t, err)
		encoded, err := f.app.LayerXHost.FinalizeProgramsOccupancy(
			lxphost.WithSequence(preview, sequence), codec.StateCommitmentProtocolVersion)
		require.NoError(t, err)
		charge := programsOccupancyPayerCharge(t, encoded, height, asset, principal)
		programsBlockEndStep(t, f, height, false)
		after, err := f.app.LayerXHost.ReadAccount(f.app.GetCheckCtx(), name, id)
		require.NoError(t, err)
		require.Equal(t, before.Balance.Sub(charge), after.Balance)
		require.Equal(t, before.NextSequence, after.NextSequence)
		final := f.app.GetCheckCtx().KVStore(key).Get(append([]byte{1, 0, 9}, []byte("progocc/final/v3")...))
		require.Len(t, final, 84)
		require.Equal(t, uint64(height), binary.BigEndian.Uint64(final[:8]))
		require.Equal(t, encoded[len(encoded)-96:len(encoded)-64], final[52:84])
	}
	require.Equal(t, mergeHeight+3, f.app.LastBlockHeight())
}

func TestBlockEndChargesAPayerHoldingTheAssetInAnAssetAccount(t *testing.T) {
	programsOccupancyPayerBlocks(t, false)
}

func TestBlockEndChargesAPayerHoldingTheAssetInTheMainAccount(t *testing.T) {
	programsOccupancyPayerBlocks(t, true)
}
