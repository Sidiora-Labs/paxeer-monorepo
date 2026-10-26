package evmrpc_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/stretchr/testify/require"
)

func TestTxPoolStatusCountsPendingEVMTxs(t *testing.T) {
	original := UnconfirmedTx
	t.Cleanup(func() { UnconfirmedTx = original })
	cosmosTx := TxConfig.NewTxBuilder()
	require.NoError(t, cosmosTx.SetMsgs(&banktypes.MsgSend{
		FromAddress: sdk.AccAddress(bytes.Repeat([]byte{1}, 20)).String(),
		ToAddress:   sdk.AccAddress(bytes.Repeat([]byte{2}, 20)).String(),
		Amount:      sdk.NewCoins(sdk.NewInt64Coin("uhpx", 1)),
	}))
	for _, tc := range []struct {
		name    string
		tx      sdk.Tx
		pending hexutil.Uint
	}{
		{name: "evm", tx: original, pending: 1},
		{name: "non_evm", tx: cosmosTx.GetTx(), pending: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			UnconfirmedTx = tc.tx
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			unconfirmed, err := (&MockClient{}).UnconfirmedTxs(ctx, nil, nil)
			require.NoError(t, err)
			require.Len(t, unconfirmed.Txs, 1)
			var expected hexutil.Uint
			for _, tx := range unconfirmed.Txs {
				decoded, err := Decoder(tx)
				require.NoError(t, err)
				msg := evmtypes.GetEVMTransactionMessage(decoded)
				if msg != nil && !msg.IsAssociateTx() {
					ethTx, txData := msg.AsTransaction()
					require.NotNil(t, txData)
					require.NotNil(t, ethTx)
					expected++
				}
			}
			require.Equal(t, tc.pending, expected)

			body := `{"jsonrpc":"2.0","method":"txpool_status","params":[],"id":"test"}`
			req, err := http.NewRequestWithContext(ctx, http.MethodPost,
				fmt.Sprintf("http://%s:%d", TestAddr, TestPort), strings.NewReader(body))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")
			res, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer res.Body.Close()
			require.Equal(t, http.StatusOK, res.StatusCode)
			var response struct {
				JSONRPC string            `json:"jsonrpc"`
				ID      string            `json:"id"`
				Result  map[string]string `json:"result"`
				Error   json.RawMessage   `json:"error"`
			}
			require.NoError(t, json.NewDecoder(res.Body).Decode(&response))
			require.Empty(t, response.Error)
			require.Equal(t, "2.0", response.JSONRPC)
			require.Equal(t, "test", response.ID)
			require.Equal(t, map[string]string{
				"pending": expected.String(),
				"queued":  "0x0",
			}, response.Result)
		})
	}
}
