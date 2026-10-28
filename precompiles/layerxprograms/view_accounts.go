package layerxprograms

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

func init() {
	register("getProgramAccountById", getProgramAccountByIdHandler)
	register("getAccountProfile", getAccountProfileHandler)
	register("getValueAccount", getValueAccountHandler)
}

func decodeAccountViewArgs(call callContext, args []interface{}, input interface{}) error {
	decoded, err := decodeArgs(call.method, args)
	if err != nil {
		if call.method != nil && len(args) != len(call.method.Inputs) {
			return fmt.Errorf("LXP_PRECOMPILE_ARG_COUNT: %w", err)
		}
		return err
	}
	return call.method.Inputs.Copy(input, decoded)
}

func accountViewError(err error, method string) error {
	if errors.Is(err, lxphost.Code(-7)) {
		return fmt.Errorf("LXP_NOT_FOUND: %s: %w", method, err)
	}
	var code lxphost.Code
	if errors.As(err, &code) {
		return kernelError(code, method)
	}
	return err
}

func getProgramAccountHandler(ctx sdk.Context, p *PrecompileExecutor, call callContext, args []interface{}) ([]byte, error) {
	var input struct {
		ProgramID [32]byte `abi:"programId"`
		AssetID   [32]byte `abi:"assetId"`
		Seed      []byte   `abi:"seed"`
	}
	if err := decodeAccountViewArgs(call, args, &input); err != nil {
		return nil, err
	}
	account, err := p.host.ReadProgramAccount(ctx, input.ProgramID, input.AssetID, input.Seed)
	if err != nil {
		return nil, accountViewError(err, call.method.Name)
	}
	return call.method.Outputs.Pack(account.AccountID, account.AssetID, account.RegisteredSequence)
}

func getProgramAccountByIdHandler(ctx sdk.Context, p *PrecompileExecutor, call callContext, args []interface{}) ([]byte, error) {
	var input struct {
		AccountID [32]byte `abi:"accountId"`
	}
	if err := decodeAccountViewArgs(call, args, &input); err != nil {
		return nil, err
	}
	account, err := p.host.ReadProgramAccountByID(ctx, input.AccountID)
	if err != nil {
		return nil, accountViewError(err, call.method.Name)
	}
	return call.method.Outputs.Pack(account.ProgramID, account.AssetID, account.Seed)
}

func getAccountProfileHandler(ctx sdk.Context, p *PrecompileExecutor, call callContext, args []interface{}) ([]byte, error) {
	var input struct {
		ProgramID [32]byte `abi:"programId"`
	}
	if err := decodeAccountViewArgs(call, args, &input); err != nil {
		return nil, err
	}
	profile, err := p.host.ReadAccountProfile(ctx, input.ProgramID)
	if err != nil {
		return nil, accountViewError(err, call.method.Name)
	}
	return call.method.Outputs.Pack(profile.Profile, profile.Principal)
}

func getValueAccountHandler(ctx sdk.Context, p *PrecompileExecutor, call callContext, args []interface{}) ([]byte, error) {
	var input struct {
		ProgramID [32]byte `abi:"programId"`
		AccountID [32]byte `abi:"accountId"`
	}
	if err := decodeAccountViewArgs(call, args, &input); err != nil {
		return nil, err
	}
	account, err := p.host.ReadValueAccount(ctx, input.ProgramID, input.AccountID)
	if errors.Is(err, lxphost.Code(-7)) {
		return call.method.Outputs.Pack(new(big.Int), false)
	}
	if err != nil {
		return nil, accountViewError(err, call.method.Name)
	}
	return call.method.Outputs.Pack(account.Balance.BigInt(), true)
}
