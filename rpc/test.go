package evmrpc

import (
	"errors"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/artifacts/cw20"
)

// TestAPI owns the process-wide test pointer override. A replacement API starts
// from the compiled version and invalidates its predecessor. HTTPServer releases
// the owner when its RPC services stop; overrides never survive server replacement.
type TestAPI struct{ version *cw20.VersionOverride }

func NewTestAPI() *TestAPI {
	return &TestAPI{version: cw20.BeginVersionOverride()}
}

func (a *TestAPI) IncrementPointerVersion(pointerType string, offset int16) error {
	switch pointerType {
	case "cw20":
		return a.version.SetVersionWithOffset(offset)
	default:
		return errors.New("invalid pointer type")
	}
}

func (a *TestAPI) close() { a.version.Close() }
