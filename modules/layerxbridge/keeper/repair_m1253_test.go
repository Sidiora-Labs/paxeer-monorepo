package keeper_test

import (
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"testing"

	bridgetestutil "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxbridge/testutil"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxbridge/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func assertExhaustedReleaseUnchanged(t *testing.T, s *suite) {
	t.Helper()
	before := s.k.ExportGenesis(s.ctx)
	require.NoError(t, before.Validate())
	require.Equal(t, []types.OutboundNonce{{ChainID: chainID, Nonce: math.MaxUint64}}, before.OutboundNonces)
	bank := s.app.BankKeeper.ExportGenesis(s.ctx)
	factory := s.app.TokenFactoryKeeper.ExportGenesis(s.ctx)
	events := append(sdk.Events{}, s.ctx.EventManager().Events()...)
	balance := s.balance(recipient)
	supply := s.app.BankKeeper.GetSupply(s.ctx, s.denom)
	inFlight := s.k.InFlight(s.ctx, s.denom)
	for _, sender := range []common.Address{recipient, {0x99}} {
		_, err := s.k.BridgeOut(s.ctx, sender, chainID, asset, big.NewInt(1), types.Address20{0x33})
		require.ErrorIs(t, err, types.ErrInvalidRequest)
		require.Contains(t, err.Error(), "outbound nonce exhausted")
		codespace, code, _ := sdkerrors.ABCIInfo(err, false)
		require.Equal(t, types.ErrInvalidRequest.Codespace(), codespace)
		require.Equal(t, types.ErrInvalidRequest.ABCICode(), code)
		require.Equal(t, before, s.k.ExportGenesis(s.ctx))
		require.Equal(t, bank, s.app.BankKeeper.ExportGenesis(s.ctx))
		require.Equal(t, factory, s.app.TokenFactoryKeeper.ExportGenesis(s.ctx))
		require.Equal(t, events, s.ctx.EventManager().Events())
		require.Equal(t, balance, s.balance(recipient))
		require.Equal(t, supply, s.app.BankKeeper.GetSupply(s.ctx, s.denom))
		require.Equal(t, inFlight, s.k.InFlight(s.ctx, s.denom))
		require.Equal(t, uint64(math.MaxUint64), s.k.OutboundNonce(s.ctx, chainID))
		require.NoError(t, s.k.ExportGenesis(s.ctx).Validate())
	}
}

func TestRepairM_1253_ReleaseNonceOverflow(t *testing.T) {
	for _, initial := range []uint64{math.MaxUint64 - 1, math.MaxUint64} {
		t.Run(fmt.Sprint(initial), func(t *testing.T) {
			genesis := types.DefaultGenesis()
			genesis.Chains = []types.Chain{{ChainID: chainID, Vault: vault, FinalityDepth: 64, Enabled: true}}
			genesis.OutboundNonces = []types.OutboundNonce{{ChainID: chainID, Nonce: initial}}
			require.NoError(t, genesis.Validate())
			state := app.NewDefaultGenesisState(app.MakeEncodingConfig().Marshaler)
			encoded, err := json.Marshal(genesis)
			require.NoError(t, err)
			state[types.ModuleName] = encoded
			p := newPersistentBridgeApp(t, state, 1)
			s := p.s
			s.denom = types.Denom(chainID, asset)
			require.Equal(t, initial, s.k.OutboundNonce(s.ctx, chainID))
			router := s.router()
			require.NoError(t, s.route(router, &types.MsgSetAttestors{Authority: authority, Set: bridgetestutil.Set(s.attestors, 1_000_000, 2)}))
			require.NoError(t, s.route(router, &types.MsgSetCap{Authority: authority, ChainID: chainID, Asset: asset,
				MaxInFlight: sdk.NewInt(1500), MaxPerTx: sdk.NewInt(1000)}))
			in := deposit(73, 900)
			_, err = s.k.BridgeIn(s.ctx, in, s.signed(in, s.attestors[0], s.attestors[1]))
			require.NoError(t, err)
			remaining := int64(900)
			if initial == math.MaxUint64-1 {
				out, err := s.k.BridgeOut(s.ctx, recipient, chainID, asset, big.NewInt(100), types.Address20{0x33})
				require.NoError(t, err)
				require.Equal(t, uint64(math.MaxUint64), out.Nonce)
				require.Equal(t, fmt.Sprint(uint64(math.MaxUint64)), attribute(s.events(types.EventBridgeOut)[0], types.AttributeNonce))
				remaining -= 100
			}
			require.Equal(t, sdk.NewInt(remaining), s.balance(recipient))
			require.Equal(t, sdk.NewInt(remaining), s.app.BankKeeper.GetSupply(s.ctx, s.denom).Amount)
			require.Equal(t, sdk.NewInt(remaining), s.k.InFlight(s.ctx, s.denom))
			assertExhaustedReleaseUnchanged(t, s)
			for index, fresh := range p.recover() {
				t.Run([]string{"reopened", "imported"}[index], func(t *testing.T) {
					require.Equal(t, sdk.NewInt(remaining), fresh.balance(recipient))
					require.Equal(t, sdk.NewInt(remaining), fresh.app.BankKeeper.GetSupply(fresh.ctx, fresh.denom).Amount)
					require.Equal(t, sdk.NewInt(remaining), fresh.k.InFlight(fresh.ctx, fresh.denom))
					assertExhaustedReleaseUnchanged(t, fresh)
				})
			}
		})
	}
}

func TestRepairM_1253_SidioraReopenProof(t *testing.T) {
	testSidioraPersistentRecovery(t)
}

func TestRepairM_1253_GeneralReopenProof(t *testing.T) {
	testBridgePersistentRecovery(t)
}

func TestRepairM_1253_CapFirstInvariant(t *testing.T) {
	// Also retain the former producer's persisted-mapping refusal assertions.
	TestSidioraPairIsRefusedOnceACapBoundItElsewhere(t)
}

func TestRepairM_1253_SidioraHelperHome(t *testing.T) {
	p := newPersistentBridgeApp(t, nil, 1)
	s := p.s
	router := s.router()
	for _, chain := range []types.Chain{solanaChain, {ChainID: chainID, Vault: vault, FinalityDepth: 64, Enabled: true}} {
		require.NoError(t, s.route(router, &types.MsgRegisterChain{Authority: authority, Chain: chain}))
	}
	bridge := s.k.ExportGenesis(s.ctx)
	bank := s.app.BankKeeper.ExportGenesis(s.ctx)
	factory := s.app.TokenFactoryKeeper.ExportGenesis(s.ctx)
	events := append(sdk.Events{}, s.ctx.EventManager().Events()...)
	for _, other := range []uint64{0, chainID, types.SidioraHomeChainID + 1} {
		msg := &types.MsgRegisterSidioraPair{Authority: authority, ChainID: other}
		require.ErrorIs(t, msg.ValidateBasic(), types.ErrInvalidRequest)
		require.ErrorIs(t, s.route(router, msg), types.ErrInvalidRequest)
		denom, err := s.k.EnsureSidioraDenom(s.ctx, other)
		require.ErrorIs(t, err, types.ErrInvalidRequest)
		require.Empty(t, denom)
		require.Equal(t, bridge, s.k.ExportGenesis(s.ctx))
		require.Equal(t, bank, s.app.BankKeeper.ExportGenesis(s.ctx))
		require.Equal(t, factory, s.app.TokenFactoryKeeper.ExportGenesis(s.ctx))
		require.Equal(t, events, s.ctx.EventManager().Events())
		require.NoError(t, s.k.ExportGenesis(s.ctx).Validate())
	}
	for _, fresh := range p.recover() {
		require.NoError(t, fresh.k.ExportGenesis(fresh.ctx).Validate())
		denom, err := fresh.k.EnsureSidioraDenom(fresh.ctx, types.SidioraHomeChainID)
		require.NoError(t, err)
		require.Equal(t, types.SidioraDenom(), denom)
		require.NoError(t, fresh.k.ExportGenesis(fresh.ctx).Validate())
	}
}

func TestRepairM_1253_SidioraFixtureChain(t *testing.T) {
	t.Run("metadata", TestSidioraDenomAndMetadata)
	t.Run("mint_burn_admin", TestSidioraBridgeMintBurnAndAdmin)
	t.Run("bridge_refusals", TestSidioraBridgeRefusals)
	t.Run("initialization_refusals", TestSidioraInitializationRefusals)
}
