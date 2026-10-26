package keeper_test

import (
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"testing"

	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/testvectors"
	anchorkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxanchor/keeper"
	anchortypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxanchor/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxcustody/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/stretchr/testify/require"
)

func TestDepositRootRegistrationIsRefusedFromTheMergePlanHeight(t *testing.T) {
	app := testkeeper.EVMTestApp
	ctx, _ := app.NewContext(false, tmtypes.Header{}).WithBlockHeight(7).WithBlockTime(genesisTime).CacheContext()
	fixture, err := testvectors.Load()
	require.NoError(t, err)
	vector := fixture["withdrawal"][0]
	k := app.LayerXCustodyKeeper
	anchor := app.LayerXAnchorKeeper
	k.SetAnchorReader(anchorkeeper.NewCustodyAnchor(anchor))

	headerBytes := vectorBytes(t, vector, "header")
	header, err := codec.DecodeBatchHeader(headerBytes)
	require.NoError(t, err)
	public, private, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)

	k.InitGenesis(ctx, *types.DefaultGenesis())
	params := types.DefaultParams()
	params.NetworkId = header.NetworkID
	params.DepositRootAuthority = hex.EncodeToString(public)
	require.NoError(t, k.SetParams(ctx, params))

	authority, _ := testkeeper.MockAddressPair()
	genesis := anchortypes.DefaultGenesis()
	genesis.Params = anchortypes.DefaultParams(authority.String())
	genesis.Params.NetworkID = header.NetworkID
	genesis.Params.PaxeerChainID = anchorPaxeerChainID
	genesis.Params.Threshold = 1
	genesis.Anchor = anchortypes.Anchor{Set: true, BatchNumber: header.BatchNumber - 1, LastSequence: header.FirstSequence - 1,
		StateRoot: anchortypes.Hash32(header.PreviousStateRoot)}
	genesis.Sequencers = []anchortypes.SequencerAuthorization{{SequencerID: anchortypes.Hash32(header.SequencerID),
		PublicKey:        anchortypes.Hash32(vectorArray(t, vector, "public_key")),
		FirstBatchNumber: header.BatchNumber, LastBatchNumber: header.BatchNumber}}
	anchor.InitGenesis(ctx, *genesis)

	guarantorID := [32]byte{0x62}
	certificate, signer := guarantorCertificate(t, headerBytes, guarantorID, [20]byte(genesis.Params.SettlementContract))
	operator, _ := testkeeper.MockAddressPair()
	bond := sdk.NewCoins(sdk.NewCoin(genesis.Params.BondDenom, genesis.Params.MinBond))
	require.NoError(t, app.BankKeeper.MintCoins(ctx, "evm", bond))
	require.NoError(t, app.BankKeeper.SendCoinsFromModuleToAccount(ctx, "evm", operator, bond))
	_, err = anchor.RegisterGuarantor(ctx, operator, guarantorID, signer, genesis.Params.MinBond)
	require.NoError(t, err)
	require.NoError(t, anchor.ActivateGuarantor(ctx, authority, guarantorID))

	var headerSignature [64]byte
	copy(headerSignature[:], vectorBytes(t, vector, "header_signature"))
	checkpoint, err := anchor.SubmitCheckpoint(ctx, operator, headerBytes, headerSignature, certificate)
	require.NoError(t, err)
	require.Equal(t, anchortypes.CheckpointFinal, checkpoint.Status)

	checkpointID := codec.CheckpointHash(headerBytes, nil)
	depositRoot, custodyReference := [32]byte{0xd1}, [32]byte{0xc1}
	registration := depositRootRegistration(checkpointID, header.ResultingStateRoot, depositRoot, custodyReference,
		header.NetworkID, header.ProtocolVersion)
	signature := ed25519.Sign(private, registration)
	ordering := [][32]byte{{0x01}, {0x02}}
	_, found := k.GetDepositRoot(ctx, checkpointID)
	require.False(t, found)
	app.UpgradeKeeper.SetDone(ctx, types.MergePlanUpgrade)

	t.Run("below merge plan height", func(t *testing.T) {
		before, _ := ctx.WithBlockHeight(6).CacheContext()
		recorded, err := k.RegisterDepositRoot(before, operator, registration, signature, ordering)
		require.NoError(t, err)
		require.Equal(t, types.Hash32(depositRoot), recorded.DepositRoot)
		stored, found := k.GetDepositRoot(before, checkpointID)
		require.True(t, found)
		require.Equal(t, recorded, stored)
	})

	for _, height := range []int64{7, 8} {
		t.Run(fmt.Sprintf("height %d", height), func(t *testing.T) {
			retired := ctx.WithBlockHeight(height).WithEventManager(sdk.NewEventManager()).WithGasMeter(sdk.NewGasMeter(1, 1, 1))
			recorded, err := k.RegisterDepositRoot(retired, operator, registration, signature, ordering)
			require.ErrorIs(t, err, types.ErrModuleRetired)
			require.Contains(t, err.Error(), fmt.Sprintf("%s from height %d", types.MergePlanUpgrade, height))
			require.Equal(t, types.DepositRootRegistration{}, recorded)
			_, err = k.RegisterDepositRoot(retired, operator, nil, nil, nil)
			require.ErrorIs(t, err, types.ErrModuleRetired)
			require.Zero(t, retired.GasMeter().GasConsumed())
			require.Empty(t, retired.EventManager().Events())
			_, found := k.GetDepositRoot(ctx, checkpointID)
			require.False(t, found)
		})
	}
}
