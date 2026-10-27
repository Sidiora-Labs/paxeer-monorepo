package keeper_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"testing"

	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	ledgerkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/keeper"
	ledgertypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	tokenfactorytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	bankkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/keeper"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestLedgerRegisterBindsIssuerDid(t *testing.T) {
	s := new(KeeperTestSuite)
	s.SetT(t)
	s.SetupTest()
	t.Cleanup(s.App.LayerXHost.Close)
	s.Ctx = s.Ctx.WithChainID("hyperpax_125-1")

	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x65}, ed25519.SeedSize))
	var public [ed25519.PublicKeySize]byte
	copy(public[:], private.Public().(ed25519.PublicKey))
	caller := common.BytesToAddress(s.TestAccs[0])
	message, err := evmtypes.LayerXBindMessage(s.App.EvmKeeper.ChainID(s.Ctx), caller, 0)
	require.NoError(t, err)
	var signature [ed25519.SignatureSize]byte
	copy(signature[:], ed25519.Sign(private, message))
	_, err = s.App.EvmKeeper.BindLayerX(s.Ctx, caller, public, signature)
	require.NoError(t, err)

	ledger := ledgerkeeper.NewIssuerBindingKeeper(s.App.LayerXLedgerKeeper,
		ledgerkeeper.NewTokenfactoryIssuerKeeper(s.App.TokenFactoryKeeper))
	actor, err := ledger.Actor(s.Ctx, caller)
	require.NoError(t, err)
	issuerDid := evmtypes.LayerXDid(public)
	require.Equal(t, issuerDid, string(actor.DID))
	issuerID, err := lxphost.DIDID([]byte(issuerDid))
	require.NoError(t, err)
	salt := [32]byte(bytes.Repeat([]byte{0x41}, 32))
	material := append([]byte("LX:ASSET:v1"), issuerID[:]...)
	assetID := sha256.Sum256(append(material, salt[:]...))
	args := ledgertypes.RegisterArgs{
		AssetID: assetID, Salt: salt, Symbol: "TKN", Name: "Token", Decimals: 6,
		SupplyCap: sdk.NewInt(1000), IssuerKind: ledgertypes.AssetIssuerNative,
	}
	payload, err := ledgertypes.EncodeRegisterPayload(args)
	require.NoError(t, err)
	require.NoError(t, ledgertypes.ValidateRegisterPayload(payload))
	creator := authtypes.NewModuleAddress(ledgertypes.ModuleName).String()
	subdenom := fmt.Sprintf("%043s", new(big.Int).SetBytes(assetID[:]).Text(62))
	require.Len(t, subdenom, 43)
	denom := tokenfactorytypes.ModuleDenomPrefix + "/" + creator + "/" + subdenom
	require.NoError(t, sdk.ValidateDenom(denom))
	_, found := s.App.BankKeeper.GetDenomMetaData(s.Ctx, denom)
	require.False(t, found)

	invalid := args
	invalid.Symbol = ""
	_, err = ledger.RegisterAsset(s.Ctx, actor, invalid, [32]byte{1})
	require.ErrorIs(t, err, ledgertypes.ErrNonCanonical)
	_, found = s.App.BankKeeper.GetDenomMetaData(s.Ctx, denom)
	require.False(t, found)

	result, err := ledger.RegisterAsset(s.Ctx, actor, args, [32]byte{2})
	require.NoError(t, err, "a validated ledger registration must create its tokenfactory denom")
	require.Zero(t, result.Receipt.ResultCode)
	storedDid, err := s.App.TokenFactoryKeeper.IssuerDid(s.Ctx, denom)
	require.NoError(t, err)
	require.Equal(t, issuerDid, storedDid)
	authority, err := s.App.TokenFactoryKeeper.GetAuthorityMetadata(s.Ctx, denom)
	require.NoError(t, err)
	require.Equal(t, tokenfactorytypes.DenomAuthorityMetadata{Admin: creator, IssuerDid: issuerDid}, authority)
	metadata, found := s.App.BankKeeper.GetDenomMetaData(s.Ctx, denom)
	require.True(t, found)
	require.Equal(t, denom, metadata.Base)
	require.NoError(t, metadata.Validate())
	asset, err := ledger.GetAsset(s.Ctx, assetID)
	require.NoError(t, err)
	require.Equal(t, issuerID, asset.IssuerDID)
	require.False(t, asset.Paused)
	require.False(t, s.App.TokenFactoryKeeper.IsDenomPaused(s.Ctx, denom))
	mirrorDenom := "layerx/" + hex.EncodeToString(assetID[:])
	require.Equal(t, mirrorDenom, ledgerkeeper.MirrorDenom(assetID))
	_, issuance, err := ledgerkeeper.IssuanceAccountID(assetID)
	require.NoError(t, err)
	require.Equal(t, args.SupplyCap, s.App.BankKeeper.GetBalance(s.Ctx, ledgerkeeper.BankAddress(issuance), mirrorDenom).Amount)

	from, to := s.TestAccs[0], s.TestAccs[1]
	require.NoError(t, s.App.TokenFactoryKeeper.MintByIssuer(s.Ctx, issuerDid, sdk.NewInt64Coin(denom, 100), from.String()))
	bank := bankkeeper.NewMsgServerImpl(s.App.BankKeeper)
	send := banktypes.NewMsgSend(from, to, sdk.NewCoins(sdk.NewInt64Coin(denom, 10)))
	require.NoError(t, send.ValidateBasic())
	_, err = bank.Send(sdk.WrapSDKContext(s.Ctx), send)
	require.NoError(t, err)
	require.Equal(t, sdk.NewInt(90), s.App.BankKeeper.GetBalance(s.Ctx, from, denom).Amount)
	require.Equal(t, sdk.NewInt(10), s.App.BankKeeper.GetBalance(s.Ctx, to, denom).Amount)

	require.NoError(t, ledgertypes.ValidatePausePayload(ledgertypes.EncodePausePayload(assetID)))
	result, err = ledger.PauseAsset(s.Ctx, actor, assetID, [32]byte{3})
	require.NoError(t, err)
	require.Zero(t, result.Receipt.ResultCode)
	asset, err = ledger.GetAsset(s.Ctx, assetID)
	require.NoError(t, err)
	require.True(t, asset.Paused)
	require.True(t, s.App.TokenFactoryKeeper.IsDenomPaused(s.Ctx, denom))
	storedDid, err = s.App.TokenFactoryKeeper.IssuerDid(s.Ctx, denom)
	require.NoError(t, err)
	require.Equal(t, issuerDid, storedDid)
	_, err = bank.Send(sdk.WrapSDKContext(s.Ctx), send)
	require.ErrorIs(t, err, banktypes.ErrSendDisabled)
	require.Equal(t, sdk.NewInt(90), s.App.BankKeeper.GetBalance(s.Ctx, from, denom).Amount)
	require.Equal(t, sdk.NewInt(10), s.App.BankKeeper.GetBalance(s.Ctx, to, denom).Amount)
}
