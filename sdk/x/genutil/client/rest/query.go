package rest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/rest"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/genutil/types"
)

// QueryGenesisTxs writes the genesis transactions to the response if no error
// occurs.
func QueryGenesisTxs(clientCtx client.Context, w http.ResponseWriter) {
	client, err := clientCtx.GetNode()
	if err != nil {
		rest.WriteErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	resultGenesis, err := client.Genesis(context.Background())
	if err != nil {
		rest.WriteErrorResponse(
			w, http.StatusInternalServerError,
			fmt.Sprintf("failed to retrieve genesis from client: %s", err),
		)
		return
	}

	appState, err := types.GenesisStateFromGenDoc(*resultGenesis.Genesis)
	if err != nil {
		rest.WriteErrorResponse(
			w, http.StatusInternalServerError,
			fmt.Sprintf("failed to decode genesis doc: %s", err),
		)
		return
	}

	genState := types.GetGenesisStateFromAppState(clientCtx.Codec, appState)
	genTxs := make([]json.RawMessage, len(genState.GenTxs))
	for i, tx := range genState.GenTxs {
		genTx, err := clientCtx.TxConfig.TxJSONDecoder()(tx)
		if err != nil {
			rest.WriteErrorResponse(
				w, http.StatusInternalServerError,
				fmt.Sprintf("failed to decode genesis transaction: %s", err),
			)
			return
		}
		genTxs[i], err = clientCtx.TxConfig.TxJSONEncoder()(genTx)
		if err != nil {
			rest.WriteErrorResponse(
				w, http.StatusInternalServerError,
				fmt.Sprintf("failed to encode genesis transaction: %s", err),
			)
			return
		}
	}

	response, err := json.Marshal(genTxs)
	if rest.CheckInternalServerError(w, err) {
		return
	}
	rest.PostProcessResponseBare(w, clientCtx, response)
}
