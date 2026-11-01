//go:build cgo

package lxphost

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

func receiptTestAmount(value codec.U128) sdk.Int {
	encoded := value.Bytes()
	return sdk.NewIntFromBigInt(new(big.Int).SetBytes(encoded[:]))
}

func receiptTestExecutions(t *testing.T) []struct {
	name    string
	receipt Receipt
} {
	t.Helper()
	const blockTime = int64(1700000000000)
	chain := newTestChain(t)
	host := chain.host(t)
	issuer := newTestActor(t, 51)
	asset := newTestAsset(t, issuer, 0x55)

	registered := execute(t, host, chain.context(1, blockTime, 41),
		issuer.registerAsset(t, host, 0, blockTime, asset, 0x55, 1000), issuer.authority())
	require.Equal(t, Code(0), registered.ResultCode)
	opened := execute(t, host, chain.context(1, blockTime, 42),
		issuer.openAssetAccount(t, host, 1, blockTime, asset), issuer.authority())
	require.Equal(t, Code(0), opened.ResultCode)
	minted := execute(t, host, chain.context(1, blockTime, 43),
		issuer.mint(t, host, 2, blockTime, asset, 250), issuer.authority())
	require.Equal(t, Code(0), minted.ResultCode)
	host.Close()

	return []struct {
		name    string
		receipt Receipt
	}{
		{"register", registered},
		{"mint", minted},
	}
}

func TestHostReceiptFeeMatchesEncoded(t *testing.T) {
	for _, execution := range receiptTestExecutions(t) {
		t.Run(execution.name, func(t *testing.T) {
			decoded, err := codec.DecodeReceipt(execution.receipt.Encoded)
			require.NoError(t, err)
			require.False(t, execution.receipt.FeeCharged.IsNil())
			require.True(t, receiptTestAmount(decoded.FeeCharged).Equal(execution.receipt.FeeCharged),
				"host fee %s differs from encoded fee %+v", execution.receipt.FeeCharged, decoded.FeeCharged)
		})
	}
}

func TestHostReceiptEffectsMatchEncoded(t *testing.T) {
	for _, execution := range receiptTestExecutions(t) {
		t.Run(execution.name, func(t *testing.T) {
			receipt := execution.receipt
			decoded, err := codec.DecodeReceipt(receipt.Encoded)
			require.NoError(t, err)
			var events []ReceiptEvent
			for _, effect := range decoded.Effects {
				if effect.Kind == 3 {
					events = append(events, ReceiptEvent{Code: effect.EventType, Body: effect.Body})
				}
			}
			require.Equal(t, events, receipt.Events)
			var transfers []TransferLeg
			if !decoded.Amount.IsZero() {
				transfers = append(transfers, TransferLeg{
					Asset:  decoded.Asset,
					From:   decoded.From,
					To:     decoded.To,
					Amount: receiptTestAmount(decoded.Amount),
				})
			}
			require.Equal(t, transfers, receipt.Transfers)
			if execution.name == "register" {
				require.NotEmpty(t, events)
				require.Empty(t, transfers)
			} else {
				require.Len(t, transfers, 1)
				require.True(t, sdk.NewInt(250).Equal(transfers[0].Amount))
			}
			require.Nil(t, decoded.ProgramOutcome)
			require.Nil(t, receipt.ProgramOutcome)
		})
	}
}
