package layerxverify_test

import (
	"crypto/ed25519"
	"fmt"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxverify"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

func TestReceiptVerifierMethodsRevertRetired(t *testing.T) {
	h := newHarness(t)
	want := pcommon.RetiredRevert("layerxVerify")
	reason, err := abi.UnpackRevert(want)
	require.NoError(t, err)
	require.Contains(t, reason, "LXP_MODULE_DISABLED")
	require.Contains(t, reason, "layerxVerify retired")
	assertRetired := func(t *testing.T, method string, args ...interface{}) {
		t.Helper()
		out, err := h.precompile.Run(h.evm, common.Address{}, common.Address{}, h.input(method, args...), nil, true, false, nil)
		require.ErrorIs(t, err, vm.ErrExecutionReverted)
		require.Equal(t, want, out)
	}
	for _, v := range h.cases("inclusion") {
		t.Run("inclusion/"+v.Name, func(t *testing.T) {
			assertRetired(t, layerxverify.VerifyReceiptInclusionMethod, inclusionArgs(t, v)...)
		})
	}
	for _, v := range h.cases("discovery") {
		t.Run("discovery/"+v.Name, func(t *testing.T) {
			assertRetired(t, layerxverify.VerifyDiscoveryProofMethod, bytesOf(t, v, "payload"),
				bytesOf(t, v, "proof_material"), array(t, v, "program_id"), number(t, v, "staleness_ms"), array(t, v, "public_key"))
		})
	}
	t.Run("malformed proofs", func(t *testing.T) {
		assertRetired(t, layerxverify.VerifyReceiptInclusionMethod, []byte{0xff}, []byte{0xff}, []byte{0xff}, []byte{1},
			[32]byte{}, [32]byte{}, uint64(9), uint64(8))
		assertRetired(t, layerxverify.VerifyDiscoveryProofMethod, []byte{0xff}, []byte{0xff}, [32]byte{}, uint64(0), [32]byte{})
	})
	for _, name := range []string{layerxverify.VerifyReceiptInclusionMethod, layerxverify.VerifyDiscoveryProofMethod} {
		t.Run(name+"/before argument parsing", func(t *testing.T) {
			out, err := (layerxverify.PrecompileExecutor{}).Execute(sdk.Context{}, h.method(name), common.Address{}, common.Address{},
				nil, nil, true, nil, nil)
			require.ErrorIs(t, err, vm.ErrExecutionReverted)
			require.Equal(t, want, out)
		})
	}
	message := []byte("raw Ed25519 remains available")
	key := ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize))
	publicKey := [32]byte(key.Public().(ed25519.PublicKey))
	signature := ed25519.Sign(key, message)
	for domain := uint16(0); domain < uint16(layerxverify.RawMessageDomain); domain++ {
		t.Run(fmt.Sprintf("domain/%d", domain), func(t *testing.T) {
			assertRetired(t, layerxverify.VerifyEd25519Method, publicKey, uint8(domain), message, signature)
		})
	}
	t.Run("domain before signature and message validation", func(t *testing.T) {
		assertRetired(t, layerxverify.VerifyEd25519Method, publicKey, uint8(2), make([]byte, codec.MaxMessageBytes+1), []byte{1})
	})
	t.Run("raw signature", func(t *testing.T) {
		out, err := h.call(layerxverify.VerifyEd25519Method, publicKey, layerxverify.RawMessageDomain, message, signature)
		require.NoError(t, err)
		require.Equal(t, []interface{}{true}, out)
		signature[0] ^= 1
		out, err = h.call(layerxverify.VerifyEd25519Method, publicKey, layerxverify.RawMessageDomain, message, signature)
		require.NoError(t, err)
		require.Equal(t, []interface{}{false}, out)
	})
	t.Run("standalone receipt", func(t *testing.T) {
		v := h.cases("receipts")[0]
		require.True(t, v.Valid)
		out, err := h.call(layerxverify.VerifyReceiptMethod, bytesOf(t, v, "receipt"), array(t, v, "public_key"))
		require.NoError(t, err)
		facts := *abi.ConvertType(out[0], new(receiptFacts)).(*receiptFacts)
		require.Equal(t, array(t, v, "digest"), facts.ReceiptDigest)
		require.Equal(t, number(t, v, "amount"), facts.Amount.Uint64())
	})
}
