package policy

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"

	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/policy/evm"
)

// PerpsCollateralPin comes from the trusted chain/header reader, never from a
// signing request. ConfiguredToken must be read from governed params at this pin.
type PerpsCollateralPin struct {
	ChainID   uint64
	Account   common.Address
	BlockHash common.Hash
	StateRoot common.Hash
}

type PerpsCollateralCaller interface {
	CallContractAtHash(context.Context, ethereum.CallMsg, common.Hash) ([]byte, error)
}

// PerpsCollateralState is immutable after a pinned execution view has agreed
// with governed params. The execution view derives its token from GetParams.
type PerpsCollateralState struct {
	pin      PerpsCollateralPin
	position [32]byte
	token    common.Address
}

func ResolvePerpsCollateral(ctx context.Context, pin PerpsCollateralPin, position *big.Int, configuredToken common.Address, caller PerpsCollateralCaller) (*PerpsCollateralState, error) {
	if pin.ChainID == 0 || pin.Account == (common.Address{}) || pin.BlockHash == (common.Hash{}) || pin.StateRoot == (common.Hash{}) || configuredToken == (common.Address{}) || caller == nil || position == nil || position.Sign() <= 0 || position.BitLen() > 256 {
		return nil, errors.New("perps collateral requires a trusted chain, signer, block, params and position")
	}
	contract, err := evm.PrecompileABI("perps")
	if err != nil {
		return nil, err
	}
	data, err := contract.Pack("getPosition", position)
	if err != nil {
		return nil, err
	}
	to, _ := evm.PrecompileAddress("perps")
	raw, err := caller.CallContractAtHash(ctx, ethereum.CallMsg{From: pin.Account, To: &to, Data: data}, pin.BlockHash)
	if err != nil {
		return nil, fmt.Errorf("pinned perps execution view: %w", err)
	}
	outputs := contract.Methods["getPosition"].Outputs
	values, err := outputs.Unpack(raw)
	if err != nil || len(values) != 10 {
		return nil, errors.New("invalid perps position execution view")
	}
	canonical, err := outputs.Pack(values...)
	if err != nil || !bytes.Equal(canonical, raw) {
		return nil, errors.New("non-canonical perps position execution view")
	}
	owner, ownerOK := values[0].(common.Address)
	token, tokenOK := values[5].(common.Address)
	active, activeOK := values[9].(bool)
	if !ownerOK || owner != pin.Account || !activeOK || !active || !tokenOK || token == (common.Address{}) || token != configuredToken {
		return nil, errors.New("perps execution collateral configuration or position disagrees with reviewed state")
	}
	s := &PerpsCollateralState{pin: pin, token: token}
	position.FillBytes(s.position[:])
	return s, nil
}

// SetPerpsCollateralReader installs the trusted params/header adapter. Each
// evaluation refreshes it; no deployment token or cached state is a fallback.
func (p *Policy) SetPerpsCollateralReader(reader func(Context, *big.Int) (*PerpsCollateralState, error)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.perpsCollateralReader = reader
}

func (ctx Context) perpsCollateral(position *big.Int) (*PerpsCollateralState, error) {
	if ctx.readPerpsCollateral == nil {
		return nil, refuse(CodeMissingField, "no pinned perps collateral configuration reader")
	}
	s, err := ctx.readPerpsCollateral(position)
	if err != nil {
		return nil, refuse(CodeMissingField, "perps collateral configuration: %v", err)
	}
	var id [32]byte
	position.FillBytes(id[:])
	if s == nil || ctx.ChainID == nil || !ctx.ChainID.IsUint64() || s.pin.ChainID != ctx.ChainID.Uint64() || s.pin.Account != ctx.Account || s.position != id {
		return nil, refuse(CodeMissingField, "perps collateral state is not bound to the reviewed chain, signer and position")
	}
	return s, nil
}
