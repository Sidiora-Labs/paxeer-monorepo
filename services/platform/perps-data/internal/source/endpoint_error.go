package source

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"

	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/decode"
)

// endpointError keeps provider-controlled text and endpoint credentials out of
// diagnostics while preserving error identity for retry and refusal decisions.
type endpointError struct {
	method string
	index  int
	class  string
	cause  error
}

func (e *endpointError) Error() string {
	return fmt.Sprintf("source: endpoint %d %s failed (%s)", e.index, e.method, e.class)
}
func (e *endpointError) Unwrap() error { return e.cause }

func endpointFailure(method string, index int, err error) error {
	return &endpointError{method: method, index: index, class: endpointFailureClass(err), cause: err}
}

type endpointHTTPError struct{ status int }

func (e *endpointHTTPError) Error() string { return fmt.Sprintf("source: http %d", e.status) }

type endpointRPCError struct{ code int }

func (e *endpointRPCError) Error() string { return fmt.Sprintf("source: json-rpc error %d", e.code) }

func endpointFailureClass(err error) string {
	var diagnostic *endpointError
	var httpErr *endpointHTTPError
	var rpcErr *endpointRPCError
	var urlErr *url.Error
	var network net.Error
	var syntax *json.SyntaxError
	var revert *decode.RevertError
	switch {
	case errors.As(err, &diagnostic):
		return diagnostic.class
	case errors.Is(err, context.Canceled):
		return "canceled"
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	case errors.Is(err, ErrResponseSize):
		return "response_size"
	case errors.Is(err, decode.ErrStatePruned):
		return "state_pruned"
	case errors.As(err, &revert):
		return "execution_reverted"
	case errors.As(err, &httpErr):
		return fmt.Sprintf("http_%d", httpErr.status)
	case errors.As(err, &rpcErr):
		return fmt.Sprintf("rpc_%d", rpcErr.code)
	case errors.As(err, &network) && network.Timeout():
		return "timeout"
	case errors.As(err, &urlErr):
		if urlErr.Op == "parse" {
			return "invalid_url"
		}
		return "transport"
	case errors.As(err, &syntax):
		return "invalid_json"
	default:
		return "request"
	}
}
