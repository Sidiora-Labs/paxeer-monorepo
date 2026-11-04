package types_test

import (
	"bytes"
	"encoding/hex"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	clienttx "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/tx"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	cryptocodec "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/ed25519"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/bech32"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/tx/signing"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/ante"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/keeper"
	authsigning "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/signing"
	authtx "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/tx"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	evidence "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/evidence"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/evidence/exported"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/evidence/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/evidence/types"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	slashingkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/slashing/keeper"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

type repairM604Harness struct {
	cdc    *codec.ProtoCodec
	ctx    sdk.Context
	keeper *keeper.Keeper
	auth   authkeeper.AccountKeeper
	router *baseapp.MsgServiceRouter
}

func repairM604Setup(t *testing.T) repairM604Harness {
	t.Helper()
	registry := codectypes.NewInterfaceRegistry()
	types.RegisterInterfaces(registry)
	cryptocodec.RegisterInterfaces(registry)
	authtypes.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	db := dbm.NewMemDB()
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	cms := store.NewCommitMultiStore(db)
	evidenceKey := sdk.NewKVStoreKey(types.StoreKey)
	authKey := sdk.NewKVStoreKey(authtypes.StoreKey)
	paramsKey := sdk.NewKVStoreKey(paramtypes.StoreKey)
	slashingKey := sdk.NewKVStoreKey("slashing")
	transientKey := sdk.NewTransientStoreKey(paramtypes.TStoreKey)
	for _, key := range []*sdk.KVStoreKey{evidenceKey, authKey, paramsKey, slashingKey} {
		cms.MountStoreWithDB(key, sdk.StoreTypeIAVL, db)
	}
	cms.MountStoreWithDB(transientKey, sdk.StoreTypeTransient, db)
	require.NoError(t, cms.LoadLatestVersion())
	ctx := sdk.NewContext(cms, tmproto.Header{
		Height: 100, ChainID: "repair-m604", Time: time.Unix(1700000100, 0).UTC(),
	}, false)
	amino := codec.NewLegacyAmino()
	auth := authkeeper.NewAccountKeeper(cdc, authKey,
		paramtypes.NewSubspace(cdc, amino, paramsKey, transientKey, "auth"), authtypes.ProtoBaseAccount, nil)
	auth.SetParams(ctx, authtypes.DefaultParams())
	slashing := slashingkeeper.NewKeeper(cdc, slashingKey, nil,
		paramtypes.NewSubspace(cdc, amino, paramsKey, transientKey, "slashing"))
	k := keeper.NewKeeper(cdc, evidenceKey, nil, slashing)
	// Adapt the production ABCI handler to the evidence router. Unknown validators
	// follow its real ignore path; SubmitEvidence still records the submission.
	k.SetRouter(types.NewRouter().AddRoute(types.RouteEquivocation, func(ctx sdk.Context, e exported.Evidence) error {
		equivocation, ok := e.(*types.Equivocation)
		if !ok {
			return types.ErrInvalidEvidence
		}
		k.HandleEquivocationEvidence(ctx, equivocation)
		return nil
	}))
	router := baseapp.NewMsgServiceRouter()
	router.SetInterfaceRegistry(registry)
	types.RegisterMsgServer(router, keeper.NewMsgServerImpl(*k))
	return repairM604Harness{cdc: cdc, ctx: ctx, keeper: k, auth: auth, router: router}
}

func repairM604Producer() abci.Evidence {
	key := ed25519.GenPrivKeyFromSecret([]byte("repair-m604-validator"))
	return abci.Evidence{
		Type:      abci.MisbehaviorType_DUPLICATE_VOTE,
		Validator: abci.Validator{Address: key.PubKey().Address(), Power: 100},
		Height:    10, Time: time.Unix(1700000000, 0).UTC(), TotalVotingPower: 100,
	}
}

func repairM604GenesisRoundTrip(t *testing.T, gs *types.GenesisState) []exported.Evidence {
	t.Helper()
	h := repairM604Setup(t)
	var decoded types.GenesisState
	require.NoError(t, h.cdc.UnmarshalAsJSON(h.cdc.MustMarshalJSON(gs), &decoded))
	require.NoError(t, decoded.Validate())
	evidence.InitGenesis(h.ctx, *h.keeper, &decoded)
	stored := h.keeper.GetAllEvidence(h.ctx)
	require.Len(t, stored, len(gs.Evidence))
	return stored
}

func repairM604MessageRoundTrip(t *testing.T, msg *types.MsgSubmitEvidence) *types.Equivocation {
	t.Helper()
	h := repairM604Setup(t)
	var decoded types.MsgSubmitEvidence
	require.NoError(t, h.cdc.Unmarshal(h.cdc.MustMarshal(msg), &decoded))
	require.NoError(t, decoded.ValidateBasic())
	result, err := h.router.Handler(&decoded)(h.ctx, &decoded)
	require.NoError(t, err)
	require.NotNil(t, result)
	stored, ok := h.keeper.GetEvidence(h.ctx, msg.GetEvidence().Hash())
	require.True(t, ok)
	require.Equal(t, msg.GetEvidence().Hash(), stored.Hash())
	return stored.(*types.Equivocation)
}

func repairM604Encode(t *testing.T, prefix string, address []byte) string {
	t.Helper()
	encoded, err := bech32.ConvertAndEncode(prefix, address)
	require.NoError(t, err)
	return encoded
}

func TestRepairM_604_AddressValidation(t *testing.T) {
	producer := repairM604Producer()
	valid := types.FromABCIEvidence(producer).(*types.Equivocation)
	require.NoError(t, valid.ValidateBasic())
	prefix := sdk.GetConfig().GetBech32ConsensusAddrPrefix()
	cases := map[string]string{
		"empty": "", "whitespace": " ", "text": "not-a-consensus-address",
		"hex": hex.EncodeToString(producer.Validator.Address), "malformed_hex": "0xnothex",
		"wrong_prefix":  repairM604Encode(t, sdk.GetConfig().GetBech32AccountAddrPrefix(), producer.Validator.Address),
		"empty_payload": repairM604Encode(t, prefix, nil),
		"short":         repairM604Encode(t, prefix, producer.Validator.Address[:19]),
		"long":          repairM604Encode(t, prefix, append(bytes.Clone(producer.Validator.Address), 1)),
		"32_bytes":      repairM604Encode(t, prefix, bytes.Repeat([]byte{1}, 32)),
		"checksum":      valid.ConsensusAddress[:len(valid.ConsensusAddress)-1] + "!",
	}
	for name, address := range cases {
		t.Run(name, func(t *testing.T) {
			h := repairM604Setup(t)
			e := *valid
			e.ConsensusAddress = address
			require.ErrorIs(t, e.ValidateBasic(), types.ErrInvalidEvidence)
			require.Panics(t, func() { e.GetConsensusAddress() })

			gs := types.NewGenesisState([]exported.Evidence{&e})
			var genesis types.GenesisState
			require.NoError(t, h.cdc.UnmarshalAsJSON(h.cdc.MustMarshalJSON(gs), &genesis))
			require.ErrorIs(t, genesis.Validate(), types.ErrInvalidEvidence)
			require.Panics(t, func() { evidence.InitGenesis(h.ctx, *h.keeper, &genesis) })
			require.Empty(t, h.keeper.GetAllEvidence(h.ctx))

			msg, err := types.NewMsgSubmitEvidence(sdk.AccAddress(producer.Validator.Address), &e)
			require.NoError(t, err)
			var decoded types.MsgSubmitEvidence
			require.NoError(t, h.cdc.Unmarshal(h.cdc.MustMarshal(msg), &decoded))
			require.ErrorIs(t, decoded.ValidateBasic(), types.ErrInvalidEvidence)
			result, err := h.router.Handler(&decoded)(h.ctx, &decoded)
			require.ErrorIs(t, err, types.ErrInvalidEvidence)
			require.Nil(t, result)
			require.Empty(t, h.keeper.GetAllEvidence(h.ctx))
			require.Empty(t, h.ctx.EventManager().Events())
		})
	}
}

func TestRepairM_604_GenesisFixture(t *testing.T) {
	producer := repairM604Producer()
	e := types.FromABCIEvidence(producer)
	stored := repairM604GenesisRoundTrip(t, types.NewGenesisState([]exported.Evidence{e}))
	require.Len(t, stored, 1)
	require.Equal(t, e.Hash(), stored[0].Hash())
	require.Equal(t, []byte(producer.Validator.Address), stored[0].(*types.Equivocation).GetConsensusAddress().Bytes())
	// Exercise the repaired original fixture, including all its rejection cases.
	TestGenesisStateValidate(t)
}

func TestRepairM_604_MessageFixture(t *testing.T) {
	producer := repairM604Producer()
	msg, err := types.NewMsgSubmitEvidence(sdk.AccAddress(producer.Validator.Address), types.FromABCIEvidence(producer))
	require.NoError(t, err)
	stored := repairM604MessageRoundTrip(t, msg)
	require.Equal(t, []byte(producer.Validator.Address), stored.GetConsensusAddress().Bytes())
	TestMsgSubmitEvidence(t)
}

func TestRepairM_604_SubmitterValidation(t *testing.T) {
	producer := repairM604Producer()
	priv := ed25519.GenPrivKeyFromSecret([]byte("repair-m604-submitter"))
	submitter := sdk.AccAddress(priv.PubKey().Address())
	cases := map[string]string{
		"valid_signed": submitter.String(), "empty": "", "whitespace": " ",
		"text": "not-an-account", "hex": hex.EncodeToString(submitter), "malformed_hex": "0xnothex",
		"wrong_prefix":  sdk.ConsAddress(submitter).String(),
		"empty_payload": repairM604Encode(t, sdk.GetConfig().GetBech32AccountAddrPrefix(), nil),
		"checksum":      submitter.String()[:len(submitter.String())-1] + "!",
	}
	for name, address := range cases {
		t.Run(name, func(t *testing.T) {
			h := repairM604Setup(t)
			account := h.auth.NewAccountWithAddress(h.ctx, submitter)
			require.NoError(t, account.SetPubKey(priv.PubKey()))
			h.auth.SetAccount(h.ctx, account)
			msg, err := types.NewMsgSubmitEvidence(submitter, types.FromABCIEvidence(producer))
			require.NoError(t, err)
			msg.Submitter = address
			config := authtx.NewTxConfig(h.cdc, authtx.DefaultSignModes)
			builder := config.NewTxBuilder()
			require.NoError(t, builder.SetMsgs(msg))
			builder.SetGasLimit(200000)
			mode := config.SignModeHandler().DefaultMode()
			require.NoError(t, builder.SetSignatures(signing.SignatureV2{
				PubKey: priv.PubKey(), Sequence: account.GetSequence(),
				Data: &signing.SingleSignatureData{SignMode: mode},
			}))
			sig, err := clienttx.SignWithPrivKey(mode, authsigning.SignerData{
				ChainID: h.ctx.ChainID(), AccountNumber: account.GetAccountNumber(), Sequence: account.GetSequence(),
			}, builder, priv, config, account.GetSequence())
			require.NoError(t, err)
			require.NoError(t, builder.SetSignatures(sig))
			wire, err := config.TxEncoder()(builder.GetTx())
			require.NoError(t, err)
			tx, err := config.TxDecoder()(wire)
			require.NoError(t, err)
			decoded := tx.GetMsgs()[0].(*types.MsgSubmitEvidence)
			anteHandler := sdk.ChainAnteDecorators(ante.NewValidateBasicDecorator(),
				ante.NewSigVerificationDecorator(h.auth, config.SignModeHandler()))
			_, anteErr := anteHandler(h.ctx, tx, false)
			result, routeErr := h.router.Handler(decoded)(h.ctx, decoded)
			if name == "valid_signed" {
				require.NoError(t, decoded.ValidateBasic())
				require.Equal(t, []sdk.AccAddress{submitter}, decoded.GetSigners())
				require.Equal(t, submitter, decoded.GetSubmitter())
				require.NoError(t, anteErr)
				require.NoError(t, routeErr)
				require.NotNil(t, result)
				stored, ok := h.keeper.GetEvidence(h.ctx, decoded.GetEvidence().Hash())
				require.True(t, ok)
				require.Equal(t, []byte(producer.Validator.Address), stored.(*types.Equivocation).GetConsensusAddress().Bytes())
			} else {
				require.ErrorIs(t, decoded.ValidateBasic(), sdkerrors.ErrInvalidAddress)
				// Tx.ValidateBasic rejects the signed transaction's missing signer;
				// MsgServiceRouter preserves the message's typed address failure.
				require.ErrorIs(t, anteErr, sdkerrors.ErrUnauthorized)
				require.ErrorIs(t, routeErr, sdkerrors.ErrInvalidAddress)
				require.Nil(t, result)
				require.Empty(t, h.keeper.GetAllEvidence(h.ctx))
				require.Empty(t, h.ctx.EventManager().Events())
			}
		})
	}
	t.Run("missing_evidence", func(t *testing.T) {
		msg := types.MsgSubmitEvidence{Submitter: submitter.String()}
		require.ErrorIs(t, msg.ValidateBasic(), types.ErrInvalidEvidence)
	})
}
