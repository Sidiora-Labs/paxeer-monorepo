package errors

import (
	"encoding/json"
	"fmt"
	"strconv"
	"testing"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/stretchr/testify/require"
)

func TestRepairM_647_RecoveredPanic(t *testing.T) {
	const secret = "private panic: database password=p647-secret"
	err := func() (err error) {
		defer Recover(&err)
		panic(secret)
	}()
	require.Error(t, err)
	require.True(t, ErrPanic.Is(err))
	require.Contains(t, err.Error(), secret)

	for _, tc := range []struct {
		name string
		err  error
	}{
		{"recovered", err},
		{"wrapped", Wrap(err, "private transaction execution context")},
		{"standard_wrapped", fmt.Errorf("private query context: %w", err)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			space, code := "undefined", uint32(111222)
			if tc.name == "standard_wrapped" {
				// The existing code resolver recognizes Cause, not stdlib Unwrap.
				code = 1
			} else {
				require.Contains(t, fmt.Sprintf("%+v", tc.err), "repair_m647_test.go")
			}
			repairM647Responses(t, tc.err, space, code, "internal error", secret)
		})
	}
}

func TestRepairM_647_UnknownInternalErrors(t *testing.T) {
	const secret = "private-db-password=p647-secret"
	_, parseErr := strconv.ParseUint(secret, 10, 64)
	require.Error(t, parseErr)
	require.Contains(t, parseErr.Error(), secret)

	for _, tc := range []struct {
		name  string
		err   error
		space string
		code  uint32
	}{
		{"unknown", parseErr, "undefined", 1},
		{"wrapped_unknown", Wrap(Wrap(parseErr, "private storage operation"), "request"), "undefined", 1},
		{"standard_wrapped_unknown", fmt.Errorf("private context: %w", parseErr), "undefined", 1},
		{"reserved_internal", Wrap(errInternal, secret), "undefined", 1},
		{"unregistered_root", New("repair-m647-unregistered", 647, secret), "repair-m647-unregistered", 647},
		{"forged_registration", New(RootCodespace, 4, secret), "sdk", 4},
		{"unregistered_coder", customErr{}, "extern", 999},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repairM647Responses(t, tc.err, tc.space, tc.code, "internal error", "")
		})
	}
}

func TestRepairM_647_RegisteredErrors(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		log  string
	}{
		{"registered", ErrUnauthorized, "unauthorized"},
		{"wrapped_registered", Wrap(Wrap(ErrUnauthorized, "missing signer"), "request rejected"), "request rejected: missing signer: unauthorized"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repairM647Responses(t, tc.err, "sdk", 4, tc.log, "")
		})
	}
}

func TestRepairM_647_Success(t *testing.T) {
	for _, err := range []error{nil, (*Error)(nil)} {
		repairM647Responses(t, err, "", 0, "", "")
	}
}

func repairM647Responses(t *testing.T, err error, space string, code uint32, publicLog, secret string) {
	t.Helper()
	events := []abci.Event{{Type: "repair-m647", Attributes: []abci.EventAttribute{{Key: []byte("operation"), Value: []byte("transaction"), Index: true}}}}
	for _, debug := range []bool{false, true} {
		t.Run(fmt.Sprintf("debug=%t", debug), func(t *testing.T) {
			wantLog := publicLog
			if debug && !errIsNil(err) {
				wantLog = fmt.Sprintf("%+v", err)
			}
			gotSpace, gotCode, gotLog := ABCIInfo(err, debug)
			require.Equal(t, space, gotSpace)
			require.Equal(t, code, gotCode)
			require.Equal(t, wantLog, gotLog)

			query := QueryResultWithDebug(err, debug)
			if !debug {
				require.Equal(t, query, QueryResult(err))
			}
			for _, transport := range []string{"protobuf", "json"} {
				t.Run(transport, func(t *testing.T) {
					var wire []byte
					var marshalErr error
					var decoded abci.ResponseQuery
					if transport == "protobuf" {
						wire, marshalErr = query.Marshal()
						require.NoError(t, marshalErr)
						require.NoError(t, decoded.Unmarshal(wire))
					} else {
						wire, marshalErr = json.Marshal(&query)
						require.NoError(t, marshalErr)
						require.NoError(t, json.Unmarshal(wire, &decoded))
					}
					repairM647Consumer(t, decoded.Codespace, decoded.Code, decoded.Log, space, code, wantLog)
					if !debug && secret != "" {
						require.NotContains(t, string(wire), secret)
					}

					for _, tx := range []abci.ResponseDeliverTx{
						ResponseDeliverTx(err, 17, 9, debug),
						ResponseDeliverTxWithEvents(err, 17, 9, events, debug),
					} {
						var decodedTx abci.ResponseDeliverTx
						if transport == "protobuf" {
							wire, marshalErr = tx.Marshal()
							require.NoError(t, marshalErr)
							require.NoError(t, decodedTx.Unmarshal(wire))
						} else {
							wire, marshalErr = json.Marshal(&tx)
							require.NoError(t, marshalErr)
							require.NoError(t, json.Unmarshal(wire, &decodedTx))
						}
						repairM647Consumer(t, decodedTx.Codespace, decodedTx.Code, decodedTx.Log, space, code, wantLog)
						require.Equal(t, int64(17), decodedTx.GasWanted)
						require.Equal(t, int64(9), decodedTx.GasUsed)
						if transport == "json" && tx.Events == nil {
							// The production JSON codec emits defaults, including events: [].
							require.Equal(t, []abci.Event{}, decodedTx.Events)
						} else {
							require.Equal(t, tx.Events, decodedTx.Events)
						}
						if !debug && secret != "" {
							require.NotContains(t, string(wire), secret)
						}
					}
				})
			}
			if debug && secret != "" {
				require.Contains(t, gotLog, secret)
			}
		})
	}
}

func repairM647Consumer(t *testing.T, space string, code uint32, log, wantSpace string, wantCode uint32, wantLog string) {
	t.Helper()
	require.Equal(t, wantSpace, space)
	require.Equal(t, wantCode, code)
	require.Equal(t, wantLog, log)
	if code == 0 {
		return
	}
	clientErr := ABCIError(space, code, log)
	require.Contains(t, clientErr.Error(), wantLog)
	if root := getUsed(wantSpace, wantCode); root != nil {
		require.True(t, root.Is(clientErr))
	}
}
