package layerxprograms

import (
	"errors"
	"fmt"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

func init() {
	register("getProgram", getProgramHandler)
	register("getInterface", getInterfaceHandler)
	register("getProgramOwner", getProgramOwnerHandler)
	register("getStateHead", getStateHeadHandler)
}

func registryProgramID(p *PrecompileExecutor, call callContext, args []interface{}) ([32]byte, error) {
	decoded, err := decodeArgs(call.method, args)
	if err != nil {
		return [32]byte{}, err
	}
	if p == nil || p.host == nil {
		return [32]byte{}, fmt.Errorf("LXP_PRECOMPILE_NO_CONTEXT: programs host is unavailable")
	}
	if len(decoded) != 1 {
		return [32]byte{}, fmt.Errorf("LXP_NON_CANONICAL: expected one program ID")
	}
	id, ok := decoded[0].([32]byte)
	if !ok {
		return [32]byte{}, fmt.Errorf("LXP_NON_CANONICAL: expected bytes32 program ID, got %T", decoded[0])
	}
	return id, nil
}

func registryReadError(err error, name string) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, lxphost.Code(-7)) {
		return fmt.Errorf("LXP_NOT_FOUND: %s: %w", name, err)
	}
	var code lxphost.Code
	if errors.As(err, &code) {
		return kernelError(code, name)
	}
	return err
}

func getProgramHandler(ctx sdk.Context, p *PrecompileExecutor, call callContext, args []interface{}) ([]byte, error) {
	id, err := registryProgramID(p, call, args)
	if err != nil {
		return nil, err
	}
	record, err := p.host.ReadProgram(ctx, id)
	if err != nil {
		return nil, registryReadError(err, "getProgram")
	}
	return programsABI.Methods["getProgram"].Outputs.Pack(record.Policy, record.Authority, record.CodeHash, record.ABIVersion, record.Version, record.Active)
}

func getInterfaceHandler(ctx sdk.Context, p *PrecompileExecutor, call callContext, args []interface{}) ([]byte, error) {
	id, err := registryProgramID(p, call, args)
	if err != nil {
		return nil, err
	}
	if _, err := p.host.ReadProgram(ctx, id); err != nil {
		return nil, registryReadError(err, "getInterface")
	}
	record, err := p.host.ReadProgramInterface(ctx, id)
	if err != nil {
		return nil, registryReadError(err, "getInterface")
	}
	return programsABI.Methods["getInterface"].Outputs.Pack(record.Version, record.Digest, record.Encoding)
}

func getProgramOwnerHandler(ctx sdk.Context, p *PrecompileExecutor, call callContext, args []interface{}) ([]byte, error) {
	id, err := registryProgramID(p, call, args)
	if err != nil {
		return nil, err
	}
	record, err := p.host.ReadProgram(ctx, id)
	if err != nil {
		return nil, registryReadError(err, "getProgramOwner")
	}
	return programsABI.Methods["getProgramOwner"].Outputs.Pack(record.Authority)
}

func getStateHeadHandler(ctx sdk.Context, p *PrecompileExecutor, call callContext, args []interface{}) ([]byte, error) {
	id, err := registryProgramID(p, call, args)
	if err != nil {
		return nil, err
	}
	if _, err := p.host.ReadProgram(ctx, id); err != nil {
		return nil, registryReadError(err, "getStateHead")
	}
	head, err := p.host.ReadProgramStateHead(ctx, id)
	if err != nil {
		return nil, registryReadError(err, "getStateHead")
	}
	return programsABI.Methods["getStateHead"].Outputs.Pack(head)
}
