package types_test

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	tokenfactory "github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory"
	tfcli "github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/client/cli"
	tfwasm "github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/client/wasm"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/types"
	wasmbindings "github.com/Sidiora-Labs/Paxeer-X-Network/node/wasmbinding/bindings"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	clienttx "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/tx"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keyring"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/std"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/module"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/tx/signing"
	authsigning "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/signing"
	authtx "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/tx"
	"github.com/stretchr/testify/require"
)

func TestRepairM_1229_IsolatedValidation(t *testing.T) {
	// Execute the repaired original cases under the task's focused gate.
	t.Run("burn", TestMsgBurn)
	t.Run("change_admin", TestMsgChangeAdmin)
}

func TestRepairM_1229_RenunciationCLI(t *testing.T) {
	repairM1229AdminLifecycle(t, "cli")
}

func TestRepairM_1229_RenunciationWasm(t *testing.T) {
	repairM1229AdminLifecycle(t, "wasm")
}

// Both producers feed the same SDK transaction codecs, signature verifier and
// application module registration. The committed fixture supplies real keepers
// and a disk store; no daemon, RPC endpoint or native runtime is involved.
func repairM1229AdminLifecycle(t *testing.T, producer string) {
	t.Helper()
	first := &secp256k1.PrivKey{Key: bytes.Repeat([]byte{1}, 32)}
	second := &secp256k1.PrivKey{Key: bytes.Repeat([]byte{2}, 32)}
	address := func(key *secp256k1.PrivKey) sdk.AccAddress {
		return sdk.AccAddress(key.PubKey().Address())
	}
	s := newProofEventsState(t)
	s.admin = address(first).String()
	denom := s.create(t, "renunciation")
	issuer := "did:layerx:" + strings.Repeat("ab", 32)
	require.NoError(t, s.keeper.SetIssuerDid(s.ctx, denom, issuer))

	registry := codectypes.NewInterfaceRegistry()
	std.RegisterInterfaces(registry)
	tokenfactory.NewAppModuleBasic().RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	txConfig := authtx.NewTxConfig(cdc, authtx.DefaultSignModes)
	var routes *baseapp.MsgServiceRouter
	var query types.QueryClient
	register := func() {
		routes = baseapp.NewMsgServiceRouter()
		routes.SetInterfaceRegistry(registry)
		queries := baseapp.NewQueryServerTestHelper(s.ctx, registry)
		tokenfactory.NewAppModule(s.keeper, nil, nil).RegisterServices(module.NewConfigurator(cdc, routes, queries))
		query = types.NewQueryClient(queries)
	}
	register()
	observe := func(admin string) {
		t.Helper()
		metadata, err := s.keeper.GetAuthorityMetadata(s.ctx, denom)
		require.NoError(t, err)
		require.Equal(t, types.DenomAuthorityMetadata{Admin: admin, IssuerDid: issuer}, metadata)
		response, err := query.DenomAuthorityMetadata(context.Background(), &types.QueryDenomAuthorityMetadataRequest{Denom: denom})
		require.NoError(t, err)
		require.Equal(t, metadata, response.AuthorityMetadata)
		did, err := s.keeper.IssuerDid(s.ctx, denom)
		require.NoError(t, err)
		require.Equal(t, issuer, did)
	}
	produce := func(sender sdk.AccAddress, newAdmin string) (client.TxBuilder, error) {
		t.Helper()
		switch producer {
		case "cli":
			var output bytes.Buffer
			clientCtx := client.Context{}.WithCodec(cdc).WithInterfaceRegistry(registry).
				WithTxConfig(txConfig).WithKeyring(keyring.NewInMemory()).WithChainID("repair-m1229").WithFrom(sender.String()).
				WithFromAddress(sender).WithGenerateOnly(true).WithOffline(true).WithOutput(&output)
			cmd := tfcli.NewChangeAdminCmd()
			cmd.SilenceUsage, cmd.SilenceErrors = true, true
			cmd.SetOut(&output)
			cmd.SetErr(&bytes.Buffer{})
			cmd.SetContext(context.WithValue(context.Background(), client.ClientContextKey, &clientCtx))
			cmd.SetArgs([]string{denom, newAdmin})
			if err := cmd.Execute(); err != nil {
				require.Empty(t, output.String(), "a refused CLI command must not emit a transaction")
				return nil, err
			}
			decoded, err := txConfig.TxJSONDecoder()(output.Bytes())
			require.NoError(t, err)
			return txConfig.WrapTxBuilder(decoded)
		case "wasm":
			raw, err := json.Marshal(wasmbindings.ChangeAdmin{Denom: denom, NewAdminAddress: newAdmin})
			require.NoError(t, err)
			msgs, err := tfwasm.EncodeTokenFactoryChangeAdmin(raw, sender)
			require.NoError(t, err)
			require.Len(t, msgs, 1)
			builder := txConfig.NewTxBuilder()
			require.NoError(t, builder.SetMsgs(msgs...))
			return builder, nil
		default:
			t.Fatalf("unknown producer %q", producer)
			return nil, nil
		}
	}
	deliver := func(builder client.TxBuilder, key *secp256k1.PrivKey) (*sdk.Result, error) {
		t.Helper()
		mode := signing.SignMode_SIGN_MODE_DIRECT
		data := authsigning.SignerData{ChainID: "repair-m1229", AccountNumber: 3, Sequence: 7}
		require.NoError(t, builder.SetSignatures(signing.SignatureV2{
			PubKey: key.PubKey(), Data: &signing.SingleSignatureData{SignMode: mode}, Sequence: data.Sequence,
		}))
		sig, err := clienttx.SignWithPrivKey(mode, data, builder, key, txConfig, data.Sequence)
		require.NoError(t, err)
		require.NoError(t, builder.SetSignatures(sig))
		wire, err := txConfig.TxEncoder()(builder.GetTx())
		require.NoError(t, err)
		decoded, err := txConfig.TxDecoder()(wire)
		require.NoError(t, err)
		require.NoError(t, decoded.ValidateBasic())
		signed, ok := decoded.(authsigning.Tx)
		require.True(t, ok)
		require.Equal(t, []sdk.AccAddress{address(key)}, signed.GetSigners())
		signatures, err := signed.GetSignaturesV2()
		require.NoError(t, err)
		require.Len(t, signatures, 1)
		require.Equal(t, address(key), sdk.AccAddress(signatures[0].PubKey.Address()))
		require.NoError(t, authsigning.VerifySignature(signatures[0].PubKey, data, signatures[0].Data, txConfig.SignModeHandler(), decoded))
		wrongChain := data
		wrongChain.ChainID += "-other"
		require.Error(t, authsigning.VerifySignature(signatures[0].PubKey, wrongChain, signatures[0].Data, txConfig.SignModeHandler(), decoded))
		msgs := decoded.GetMsgs()
		require.Len(t, msgs, 1)
		require.NoError(t, msgs[0].ValidateBasic())
		require.IsType(t, &types.MsgChangeAdmin{}, msgs[0])
		require.Equal(t, builder.GetTx().GetMsgs()[0], msgs[0])
		handler := routes.Handler(msgs[0])
		require.NotNil(t, handler)
		return handler(s.ctx, msgs[0])
	}

	observe(address(first).String())
	bad, err := produce(address(first), "invalid-admin")
	if producer == "cli" {
		require.ErrorIs(t, err, sdkerrors.ErrInvalidAddress)
		require.Nil(t, bad)
	} else {
		require.NoError(t, err)
		msg := bad.GetTx().GetMsgs()[0]
		require.ErrorIs(t, msg.ValidateBasic(), sdkerrors.ErrInvalidAddress)
		result, err := routes.Handler(msg)(s.ctx, msg)
		require.ErrorIs(t, err, sdkerrors.ErrInvalidAddress)
		require.Nil(t, result)
	}
	observe(address(first).String())
	for _, step := range []struct {
		key      *secp256k1.PrivKey
		newAdmin string
	}{
		{first, address(second).String()},
		{second, ""},
	} {
		builder, err := produce(address(step.key), step.newAdmin)
		require.NoError(t, err)
		msg := builder.GetTx().GetMsgs()[0].(*types.MsgChangeAdmin)
		require.Equal(t, types.NewMsgChangeAdmin(address(step.key).String(), denom, step.newAdmin), msg)
		require.JSONEq(t, string(types.NewMsgChangeAdmin(address(step.key).String(), denom, step.newAdmin).GetSignBytes()), string(msg.GetSignBytes()))
		result, err := deliver(builder, step.key)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Len(t, result.Events, 1)
		require.Equal(t, types.TypeMsgChangeAdmin, result.Events[0].Type)
		attributes := map[string]string{}
		for _, attr := range result.Events[0].Attributes {
			attributes[string(attr.Key)] = string(attr.Value)
		}
		require.Equal(t, map[string]string{types.AttributeDenom: denom, types.AttributeNewAdmin: step.newAdmin}, attributes)
		observe(step.newAdmin)
		s.reopen(t)
		register()
		observe(step.newAdmin)
	}
	// Neither former admin can restore authority or mint after renunciation.
	for _, key := range []*secp256k1.PrivKey{first, second} {
		builder, err := produce(address(key), address(key).String())
		require.NoError(t, err)
		result, err := deliver(builder, key)
		require.ErrorIs(t, err, types.ErrUnauthorized)
		require.Nil(t, result)
		mint := types.NewMsgMint(address(key).String(), sdk.NewInt64Coin(denom, 1))
		result, err = routes.Handler(mint)(s.ctx, mint)
		require.ErrorIs(t, err, types.ErrUnauthorized)
		require.Nil(t, result)
		observe("")
	}
	emptySender := types.NewMsgChangeAdmin("", denom, address(first).String())
	result, err := routes.Handler(emptySender)(s.ctx, emptySender)
	require.ErrorIs(t, err, sdkerrors.ErrInvalidAddress)
	require.Nil(t, result)
	observe("")
	require.Empty(t, s.ctx.EventManager().Events())
}
