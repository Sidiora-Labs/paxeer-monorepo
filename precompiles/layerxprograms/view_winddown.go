package layerxprograms

import (
	"errors"
	"fmt"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

func getWindDownHandler(ctx sdk.Context, p *PrecompileExecutor, call callContext, args []interface{}) ([]byte, error) {
	decoded, err := decodeArgs(call.method, args)
	if err != nil {
		return nil, err
	}
	program, ok := decoded[0].([32]byte)
	if !ok || program == ([32]byte{}) {
		return nil, fmt.Errorf("LXP_NON_CANONICAL: getWindDown requires a nonzero bytes32 program ID")
	}
	if p == nil || p.host == nil {
		return nil, fmt.Errorf("LXP_PRECOMPILE_NO_CONTEXT: programs host is unavailable")
	}
	// The kernel's 54-byte wind-down record is version[0], status[1],
	// exit program[2:34], deadline[34:42], effective sequence[42:50],
	// account count[50:52], live count[52:54]; integers are big-endian.
	record, err := p.host.ReadWindDown(ctx, program)
	if errors.Is(err, lxphost.Code(-7)) {
		if _, err = p.host.ReadProgram(ctx, program); err != nil {
			return nil, windDownViewError(err, call.method.Name)
		}
		record = lxphost.WindDown{ProgramID: program, Status: 1}
	} else if err != nil {
		return nil, windDownViewError(err, call.method.Name)
	}
	return call.method.Outputs.Pack(record.Status, record.ExitProgram, record.Deadline,
		record.EffectiveSequence, record.ValueAccountCount, record.LiveValueAccountCount)
}

func getExitRouteHandler(ctx sdk.Context, p *PrecompileExecutor, call callContext, args []interface{}) ([]byte, error) {
	decoded, err := decodeArgs(call.method, args)
	if err != nil {
		return nil, err
	}
	program, programOK := decoded[0].([32]byte)
	account, accountOK := decoded[1].([32]byte)
	if !programOK || !accountOK || program == ([32]byte{}) || account == ([32]byte{}) {
		return nil, fmt.Errorf("LXP_NON_CANONICAL: getExitRoute requires nonzero bytes32 program and account IDs")
	}
	if p == nil || p.host == nil {
		return nil, fmt.Errorf("LXP_PRECOMPILE_NO_CONTEXT: programs host is unavailable")
	}
	route, err := p.host.ReadExitRoute(ctx, program, account)
	if err != nil {
		return nil, windDownViewError(err, call.method.Name)
	}
	return call.method.Outputs.Pack(route.AssetID, route.Destination, route.Seed)
}

func windDownViewError(err error, op string) error {
	if errors.Is(err, lxphost.Code(-7)) {
		return fmt.Errorf("LXP_NOT_FOUND: %s: %w", op, err)
	}
	var code lxphost.Code
	if errors.As(err, &code) {
		return kernelError(code, op)
	}
	return err
}

func init() {
	register("getWindDown", getWindDownHandler)
	register("getExitRoute", getExitRouteHandler)
}
