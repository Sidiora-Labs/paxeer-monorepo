package rest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	grpctypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/grpc"
	"github.com/gogo/protobuf/proto"
	"github.com/gorilla/mux"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/oracle/types"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/rest"
)

func registerQueryRoutes(cliCtx client.Context, rtr *mux.Router) {
	rtr.HandleFunc(fmt.Sprintf("/oracle/denoms/{%s}/exchange_rate", RestDenom), queryExchangeRateHandlerFunction(cliCtx)).Methods("GET")
	rtr.HandleFunc("/oracle/denoms/price_snapshot_history", queryPriceSnapshotHistoryHandlerFunction(cliCtx)).Methods("GET")
	rtr.HandleFunc("/oracle/denoms/actives", queryActivesHandlerFunction(cliCtx)).Methods("GET")
	rtr.HandleFunc("/oracle/denoms/exchange_rates", queryExchangeRatesHandlerFunction(cliCtx)).Methods("GET")
	rtr.HandleFunc("/oracle/denoms/twaps", queryTwapsHandlerFunction(cliCtx)).Methods("GET")
	rtr.HandleFunc("/oracle/denoms/vote_targets", queryVoteTargetsHandlerFunction(cliCtx)).Methods("GET")
	rtr.HandleFunc(fmt.Sprintf("/oracle/voters/{%s}/feeder", RestVoter), queryFeederDelegationHandlerFunction(cliCtx)).Methods("GET")
	rtr.HandleFunc(fmt.Sprintf("/oracle/voters/{%s}/vote_penalty_counter", RestVoter), queryVotePenaltyCounterHandlerFunction(cliCtx)).Methods("GET")
	rtr.HandleFunc("/oracle/parameters", queryParamsHandlerFunction(cliCtx)).Methods("GET")
}

func queryExchangeRateHandlerFunction(cliCtx client.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		denom, ok := checkDenomVar(w, r)
		if !ok {
			return
		}
		queryOracle(w, r, cliCtx, func(ctx context.Context, q types.QueryClient, opts ...grpc.CallOption) (proto.Message, error) {
			return q.ExchangeRate(ctx, &types.QueryExchangeRateRequest{Denom: denom}, opts...)
		})
	}
}

func queryExchangeRatesHandlerFunction(cliCtx client.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		queryOracle(w, r, cliCtx, func(ctx context.Context, q types.QueryClient, opts ...grpc.CallOption) (proto.Message, error) {
			return q.ExchangeRates(ctx, &types.QueryExchangeRatesRequest{}, opts...)
		})
	}
}

func queryPriceSnapshotHistoryHandlerFunction(cliCtx client.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		queryOracle(w, r, cliCtx, func(ctx context.Context, q types.QueryClient, opts ...grpc.CallOption) (proto.Message, error) {
			return q.PriceSnapshotHistory(ctx, &types.QueryPriceSnapshotHistoryRequest{}, opts...)
		})
	}
}

func queryTwapsHandlerFunction(cliCtx client.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		lookbackSeconds, ok := checkLookbackSecondsVar(w, r)
		if !ok {
			return
		}
		queryOracle(w, r, cliCtx, func(ctx context.Context, q types.QueryClient, opts ...grpc.CallOption) (proto.Message, error) {
			params, err := q.Params(ctx, &types.QueryParamsRequest{}, opts...)
			if err != nil {
				return nil, err
			}
			if uint64(lookbackSeconds) > params.Params.LookbackDuration {
				return nil, status.Error(codes.InvalidArgument, types.ErrInvalidTwapLookback.Error())
			}
			return q.Twaps(ctx, &types.QueryTwapsRequest{LookbackSeconds: uint64(lookbackSeconds)}, opts...)
		})
	}
}

func queryActivesHandlerFunction(cliCtx client.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		queryOracle(w, r, cliCtx, func(ctx context.Context, q types.QueryClient, opts ...grpc.CallOption) (proto.Message, error) {
			return q.Actives(ctx, &types.QueryActivesRequest{}, opts...)
		})
	}
}

func queryParamsHandlerFunction(cliCtx client.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		queryOracle(w, r, cliCtx, func(ctx context.Context, q types.QueryClient, opts ...grpc.CallOption) (proto.Message, error) {
			return q.Params(ctx, &types.QueryParamsRequest{}, opts...)
		})
	}
}

func queryFeederDelegationHandlerFunction(cliCtx client.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		voterAddr, ok := checkVoterAddressVar(w, r)
		if !ok {
			return
		}
		queryOracle(w, r, cliCtx, func(ctx context.Context, q types.QueryClient, opts ...grpc.CallOption) (proto.Message, error) {
			return q.FeederDelegation(ctx, &types.QueryFeederDelegationRequest{ValidatorAddr: voterAddr.String()}, opts...)
		})
	}
}

func queryVotePenaltyCounterHandlerFunction(cliCtx client.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		voterAddr, ok := checkVoterAddressVar(w, r)
		if !ok {
			return
		}
		queryOracle(w, r, cliCtx, func(ctx context.Context, q types.QueryClient, opts ...grpc.CallOption) (proto.Message, error) {
			return q.VotePenaltyCounter(ctx, &types.QueryVotePenaltyCounterRequest{ValidatorAddr: voterAddr.String()}, opts...)
		})
	}
}

func queryVoteTargetsHandlerFunction(cliCtx client.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		queryOracle(w, r, cliCtx, func(ctx context.Context, q types.QueryClient, opts ...grpc.CallOption) (proto.Message, error) {
			return q.VoteTargets(ctx, &types.QueryVoteTargetsRequest{}, opts...)
		})
	}
}

// queryOracle uses the registered typed query service and retains the legacy
// height/result envelope around the protobuf JSON response.
func queryOracle(w http.ResponseWriter, r *http.Request, cliCtx client.Context,
	query func(context.Context, types.QueryClient, ...grpc.CallOption) (proto.Message, error),
) {
	cliCtx, ok := rest.ParseQueryHeightOrReturnBadRequest(w, cliCtx, r)
	if !ok {
		return
	}
	var header metadata.MD
	response, err := query(r.Context(), types.NewQueryClient(cliCtx), grpc.Header(&header))
	if err != nil {
		writeOracleQueryError(w, runtime.HTTPStatusFromCode(status.Code(err)), err.Error())
		return
	}
	heights := header.Get(grpctypes.GRPCBlockHeightHeader)
	if len(heights) != 1 {
		writeOracleQueryError(w, http.StatusInternalServerError, "missing query response height")
		return
	}
	height, err := strconv.ParseInt(heights[0], 10, 64)
	if rest.CheckInternalServerError(w, err) {
		return
	}
	bz, err := codec.ProtoMarshalJSON(response, nil)
	if rest.CheckInternalServerError(w, err) {
		return
	}
	rest.PostProcessResponse(w, cliCtx.WithHeight(height), bz)
}

// writeOracleQueryError includes the HTTP status in the JSON error body.
func writeOracleQueryError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(rest.NewErrorResponse(statusCode, message))
}

func checkDenomVar(w http.ResponseWriter, r *http.Request) (string, bool) {
	denom := mux.Vars(r)[RestDenom]
	err := sdk.ValidateDenom(denom)
	if rest.CheckBadRequestError(w, err) {
		return "", false
	}

	return denom, true
}

// The retained TWAP route accepts exactly one lookback_seconds query value.
func checkLookbackSecondsVar(w http.ResponseWriter, r *http.Request) (int64, bool) {
	values, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		writeOracleQueryError(w, http.StatusBadRequest, err.Error())
		return 0, false
	}
	lookbacks := values[RestLookbackSeconds]
	if len(lookbacks) != 1 {
		writeOracleQueryError(w, http.StatusBadRequest, "exactly one lookback_seconds query parameter is required")
		return 0, false
	}
	lookbackSeconds, err := strconv.ParseInt(lookbacks[0], 10, 64)
	if err != nil || lookbackSeconds <= 0 {
		writeOracleQueryError(w, http.StatusBadRequest, "lookback_seconds must be a positive int64")
		return 0, false
	}
	return lookbackSeconds, true
}

func checkVoterAddressVar(w http.ResponseWriter, r *http.Request) (sdk.ValAddress, bool) {
	addr, err := sdk.ValAddressFromBech32(mux.Vars(r)[RestVoter])
	if rest.CheckBadRequestError(w, err) {
		return nil, false
	}

	return addr, true
}
