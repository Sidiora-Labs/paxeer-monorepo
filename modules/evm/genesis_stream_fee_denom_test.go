package evm_test

import (
	"bytes"
	"math/big"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestExportGenesisStreamAccountFeeDenom(t *testing.T) {
	source := app.Setup(t, false, false, false)
	ctx := source.GetContextForDeliverTx(nil).WithBlockHeight(8)
	k := &source.EvmKeeper
	const denom = "usid"
	params := types.DefaultParams()
	params.FeeTokenEnabled = true
	params.AllowedFeeDenoms = []types.AllowedFeeDenom{{
		Denom: denom, Rate: sdk.NewDec(types.InitialSidioraBaseUnitsPerPax), RateUpdateHeight: ctx.BlockHeight(),
	}}
	k.SetParams(ctx, params)

	require.Equal(t, 1000, evm.GENESIS_EXPORT_STREAM_SERIALIZED_LEN_MAX)
	var expected []*types.Serialized
	for i := 1; i <= evm.GENESIS_EXPORT_STREAM_SERIALIZED_LEN_MAX+2; i++ {
		account := common.BigToAddress(big.NewInt(int64(i)))
		require.NoError(t, k.SetAccountFeeDenom(ctx, account, denom))
		expected = append(expected, &types.Serialized{
			Prefix: []byte{0x23}, Key: account.Bytes(), Value: []byte(denom),
		})
	}

	var chunks []*types.GenesisState
	for chunk := range evm.ExportGenesisStream(ctx, k) {
		chunks = append(chunks, chunk)
	}

	var serialized []*types.Serialized
	var feeDenomChunkSizes []int
	for _, chunk := range chunks {
		feeDenomCount := 0
		for _, entry := range chunk.Serialized {
			if bytes.Equal(entry.Prefix, []byte{0x23}) {
				serialized = append(serialized, entry)
				feeDenomCount++
			}
		}
		if feeDenomCount > 0 {
			feeDenomChunkSizes = append(feeDenomChunkSizes, feeDenomCount)
		}
	}
	require.Equal(t, []int{evm.GENESIS_EXPORT_STREAM_SERIALIZED_LEN_MAX + 1, 1}, feeDenomChunkSizes)
	require.Equal(t, expected, serialized)

	var regular []*types.Serialized
	for _, entry := range evm.ExportGenesis(ctx, k).Serialized {
		if bytes.Equal(entry.Prefix, types.AccountFeeDenomKeyPrefix) {
			regular = append(regular, entry)
		}
	}
	require.Equal(t, regular, serialized)
}
