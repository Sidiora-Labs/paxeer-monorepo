package genutil_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	cfg "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	cryptocodec "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/hd"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keyring"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/module"
	authtx "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/tx"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/genutil"
	genutilcli "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/genutil/client/cli"
	gentypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/genutil/types"
	stakingtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/types"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

const repairM653Mnemonic = "side video kiss hotel essence door angle student degree during vague adjust submit trick globe muscle frozen vacuum artwork million shield bind useful wave"

type repairM653Env struct {
	config *cfg.Config
	client client.Context
	basics module.BasicManager
}

func repairM653Environment(t *testing.T) repairM653Env {
	t.Helper()
	config := cfg.TestConfig().SetRoot(t.TempDir())
	require.NoError(t, os.MkdirAll(filepath.Dir(config.NodeKeyFile()), 0700))
	require.NoError(t, os.MkdirAll(filepath.Dir(config.PrivValidator.StateFile()), 0700))
	registry := codectypes.NewInterfaceRegistry()
	sdk.RegisterInterfaces(registry)
	cryptocodec.RegisterInterfaces(registry)
	banktypes.RegisterInterfaces(registry)
	stakingtypes.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	amino := codec.NewLegacyAmino()
	sdk.RegisterLegacyAminoCodec(amino)
	cryptocodec.RegisterCrypto(amino)
	stakingtypes.RegisterLegacyAminoCodec(amino)
	return repairM653Env{
		config: config,
		client: client.Context{}.WithCodec(cdc).WithLegacyAmino(amino).
			WithTxConfig(authtx.NewTxConfig(cdc, authtx.DefaultSignModes)).
			WithInterfaceRegistry(registry).WithHomeDir(config.RootDir).
			WithChainID("repair-m653").WithOffline(true).WithKeyring(keyring.NewInMemory()),
		basics: module.NewBasicManager(genutil.AppModuleBasic{}),
	}
}

func repairM653Execute(t *testing.T, env repairM653Env, cmd *cobra.Command, args ...string) (string, error) {
	t.Helper()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetArgs(args)
	// displayInfo writes directly to stderr, so capture both process streams too.
	stdout, err := os.CreateTemp(t.TempDir(), "stdout")
	require.NoError(t, err)
	defer stdout.Close()
	stderr, err := os.CreateTemp(t.TempDir(), "stderr")
	require.NoError(t, err)
	defer stderr.Close()
	originalOut, originalErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = stdout, stderr
	defer func() { os.Stdout, os.Stderr = originalOut, originalErr }()
	ctx := context.WithValue(context.Background(), client.ClientContextKey, &env.client)
	ctx = context.WithValue(ctx, server.ServerContextKey, server.NewContext(viper.New(), env.config))
	err = cmd.ExecuteContext(ctx)
	_, seekErr := stdout.Seek(0, io.SeekStart)
	require.NoError(t, seekErr)
	_, readErr := io.Copy(&output, stdout)
	require.NoError(t, readErr)
	_, seekErr = stderr.Seek(0, io.SeekStart)
	require.NoError(t, seekErr)
	_, readErr = io.Copy(&output, stderr)
	require.NoError(t, readErr)
	return output.String(), err
}

func repairM653Read(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return data
}

func repairM653Generate(t *testing.T, env repairM653Env) (string, []byte) {
	t.Helper()
	info, err := env.client.Keyring.NewAccount("validator", repairM653Mnemonic, "", sdk.GetConfig().GetFullBIP44Path(), hd.Secp256k1)
	require.NoError(t, err)
	bank := banktypes.DefaultGenesisState()
	bank.Balances = []banktypes.Balance{{Address: info.GetAddress().String(), Coins: sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, 10000000))}}
	require.NoError(t, bank.Validate())
	state := env.basics.DefaultGenesis(env.client.Codec)
	state[banktypes.ModuleName] = env.client.Codec.MustMarshalJSON(bank)
	state[stakingtypes.ModuleName] = env.client.Codec.MustMarshalJSON(stakingtypes.DefaultGenesisState())
	appState, err := json.Marshal(state)
	require.NoError(t, err)
	doc := &tmtypes.GenesisDoc{ChainID: env.client.ChainID, AppState: appState}
	require.NoError(t, genutil.ExportGenesisFile(doc, env.config.GenesisFile()))
	dir := filepath.Join(env.config.RootDir, "config", "gentx")
	require.NoError(t, os.MkdirAll(dir, 0700))
	path := filepath.Join(dir, "ceremony.json")
	output, err := repairM653Execute(t, env,
		genutilcli.GenTxCmd(env.basics, env.client.TxConfig, banktypes.GenesisBalancesIterator{}, env.config.RootDir),
		"validator", "1000000"+sdk.DefaultBondDenom, "--chain-id="+env.client.ChainID,
		"--ip=127.0.0.1", "--moniker=producer", "--output-document="+path)
	require.NoError(t, err, output)
	require.Contains(t, output, "Genesis transaction written")
	encoded := repairM653Read(t, path)
	tx, err := env.client.TxConfig.TxJSONDecoder()(encoded)
	require.NoError(t, err)
	require.Len(t, tx.GetMsgs(), 1)
	require.NoError(t, tx.GetMsgs()[0].ValidateBasic())
	require.NoError(t, tx.ValidateBasic())
	return path, encoded
}

func TestRepairM_653_CollectMalformedGentxs(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*testing.T, client.TxBuilder)
		want   string
	}{
		{"zero_messages", func(t *testing.T, b client.TxBuilder) { require.NoError(t, b.SetMsgs()) }, "exactly 1 CreateValidator message, got 0"},
		{"wrong_message", func(t *testing.T, b client.TxBuilder) {
			msg := b.GetTx().GetMsgs()[0].(*stakingtypes.MsgCreateValidator)
			addr, err := sdk.AccAddressFromBech32(msg.DelegatorAddress)
			require.NoError(t, err)
			send := banktypes.NewMsgSend(addr, addr, sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, 1)))
			require.NoError(t, send.ValidateBasic())
			require.NoError(t, b.SetMsgs(send))
		}, "expected MsgCreateValidator"},
		{"multiple_messages", func(t *testing.T, b client.TxBuilder) {
			msg := b.GetTx().GetMsgs()[0]
			require.NoError(t, b.SetMsgs(msg, msg))
		}, "exactly 1 CreateValidator message, got 2"},
		{"invalid_address", func(t *testing.T, b client.TxBuilder) {
			msg := b.GetTx().GetMsgs()[0].(*stakingtypes.MsgCreateValidator)
			msg.DelegatorAddress = "invalid"
			require.NoError(t, b.SetMsgs(msg))
		}, "invalid MsgCreateValidator"},
		{"zero_stake", func(t *testing.T, b client.TxBuilder) {
			msg := b.GetTx().GetMsgs()[0].(*stakingtypes.MsgCreateValidator)
			msg.Value = sdk.NewInt64Coin(sdk.DefaultBondDenom, 0)
			require.NoError(t, b.SetMsgs(msg))
		}, "invalid delegation amount"},
		{"invalid_commission", func(t *testing.T, b client.TxBuilder) {
			msg := b.GetTx().GetMsgs()[0].(*stakingtypes.MsgCreateValidator)
			msg.Commission.Rate = sdk.NewDec(2)
			require.NoError(t, b.SetMsgs(msg))
		}, "invalid MsgCreateValidator"},
		{"unsigned", func(t *testing.T, b client.TxBuilder) { require.NoError(t, b.SetSignatures()) }, "invalid genesis transaction"},
		{"excessive_gas", func(t *testing.T, b client.TxBuilder) { b.SetGasLimit(^uint64(0)) }, "invalid gas supplied"},
		{"missing_memo", func(t *testing.T, b client.TxBuilder) { b.SetMemo("") }, "failed to find node's address and IP"},
		{"insufficient_balance", func(t *testing.T, b client.TxBuilder) {
			msg := b.GetTx().GetMsgs()[0].(*stakingtypes.MsgCreateValidator)
			msg.Value = sdk.NewInt64Coin(sdk.DefaultBondDenom, 10000001)
			require.NoError(t, b.SetMsgs(msg))
		}, "insufficient fund"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := repairM653Environment(t)
			path, encoded := repairM653Generate(t, env)
			decoded, err := env.client.TxConfig.TxJSONDecoder()(encoded)
			require.NoError(t, err)
			builder, err := env.client.TxConfig.WrapTxBuilder(decoded)
			require.NoError(t, err)
			tc.mutate(t, builder)
			encoded, err = env.client.TxConfig.TxJSONEncoder()(builder.GetTx())
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(path, encoded, 0600))
			// The real decoder must accept the fixture before collection rejects it.
			_, err = env.client.TxConfig.TxJSONDecoder()(encoded)
			require.NoError(t, err)
			before := repairM653Read(t, env.config.GenesisFile())
			doc, err := tmtypes.GenesisDocFromFile(env.config.GenesisFile())
			require.NoError(t, err)
			accepted, peers, err := genutil.CollectTxs(env.client.Codec, env.client.TxConfig.TxJSONDecoder(), env.config.Moniker, filepath.Dir(path), *doc, banktypes.GenesisBalancesIterator{})
			require.ErrorContains(t, err, tc.want)
			require.ErrorContains(t, err, filepath.Base(path))
			require.Empty(t, accepted)
			require.Empty(t, peers)
			output, err := repairM653Execute(t, env, genutilcli.CollectGenTxsCmd(banktypes.GenesisBalancesIterator{}, env.config.RootDir))
			require.ErrorContains(t, err, tc.want)
			require.ErrorContains(t, err, filepath.Base(path))
			require.Empty(t, output)
			require.Equal(t, before, repairM653Read(t, env.config.GenesisFile()))
			require.Equal(t, encoded, repairM653Read(t, path))
		})
	}
}

func TestRepairM_653_CollectGeneratedGentx(t *testing.T) {
	env := repairM653Environment(t)
	path, encoded := repairM653Generate(t, env)
	generated, err := env.client.TxConfig.TxJSONDecoder()(encoded)
	require.NoError(t, err)
	memo := generated.(sdk.TxWithMemo).GetMemo()
	env.config.Moniker = "collector"
	output, err := repairM653Execute(t, env, genutilcli.CollectGenTxsCmd(banktypes.GenesisBalancesIterator{}, env.config.RootDir))
	require.NoError(t, err, output)
	require.Contains(t, output, `"app_message"`)
	require.Equal(t, memo, env.config.P2P.PersistentPeers)
	doc, err := tmtypes.GenesisDocFromFile(env.config.GenesisFile())
	require.NoError(t, err)
	state, err := gentypes.GenesisStateFromGenDoc(*doc)
	require.NoError(t, err)
	collected := gentypes.GetGenesisStateFromAppState(env.client.Codec, state)
	require.Len(t, collected.GenTxs, 1)
	require.JSONEq(t, string(encoded), string(collected.GenTxs[0]))
	require.NoError(t, gentypes.ValidateGenesis(collected, env.client.TxConfig.TxJSONDecoder()))
	reopened, err := env.client.TxConfig.TxJSONDecoder()(collected.GenTxs[0])
	require.NoError(t, err)
	require.NoError(t, reopened.ValidateBasic())
	require.Equal(t, generated.GetMsgs(), reopened.GetMsgs())
	require.Equal(t, encoded, repairM653Read(t, path))
}

func TestRepairM_653_ValidatorLoadErrors(t *testing.T) {
	for _, corrupt := range []string{"key", "state"} {
		t.Run(corrupt, func(t *testing.T) {
			env := repairM653Environment(t)
			nodeID, pub, err := genutil.InitializeNodeValidatorFiles(env.config)
			require.NoError(t, err)
			require.NotEmpty(t, nodeID)
			require.NotNil(t, pub)
			path := env.config.PrivValidator.KeyFile()
			if corrupt == "state" {
				path = env.config.PrivValidator.StateFile()
			}
			require.NoError(t, os.WriteFile(path, []byte("malformed validator JSON"), 0600))
			before := repairM653Identity(t, env)
			_, originalErr := privval.LoadOrGenFilePV(env.config.PrivValidator.KeyFile(), env.config.PrivValidator.StateFile())
			require.Error(t, originalErr)
			nodeID, pub, err = genutil.InitializeNodeValidatorFiles(env.config)
			require.EqualError(t, err, originalErr.Error())
			require.Empty(t, nodeID)
			require.Nil(t, pub)
			require.Equal(t, before, repairM653Identity(t, env))
			for _, cmd := range []*cobra.Command{
				genutilcli.InitCmd(env.basics, env.config.RootDir),
				genutilcli.CollectGenTxsCmd(banktypes.GenesisBalancesIterator{}, env.config.RootDir),
			} {
				var args []string
				if cmd.Name() == "init" {
					args = []string{"validator"}
				}
				output, err := repairM653Execute(t, env, cmd, args...)
				require.ErrorContains(t, err, originalErr.Error())
				require.Empty(t, output)
				require.Equal(t, before, repairM653Identity(t, env))
				_, err = os.Stat(env.config.GenesisFile())
				require.True(t, os.IsNotExist(err))
			}
		})
	}
}

func repairM653Identity(t *testing.T, env repairM653Env) map[string][]byte {
	t.Helper()
	result := make(map[string][]byte)
	for _, path := range []string{env.config.NodeKeyFile(), env.config.PrivValidator.KeyFile(), env.config.PrivValidator.StateFile(), filepath.Join(filepath.Dir(env.config.PrivValidator.KeyFile()), "validator_pubkey.txt")} {
		result[path] = repairM653Read(t, path)
	}
	return result
}

func TestRepairM_653_ValidatorWriteFailure(t *testing.T) {
	for _, entry := range []string{"initializer", "init", "collect-gentxs"} {
		t.Run(entry, func(t *testing.T) {
			env := repairM653Environment(t)
			// Generate the P2P identity before inducing a real validator save failure.
			_, err := env.config.LoadOrGenNodeKeyID()
			require.NoError(t, err)
			nodeBefore := repairM653Read(t, env.config.NodeKeyFile())
			blocker := filepath.Join(filepath.Dir(env.config.PrivValidator.KeyFile()), "validator_pubkey.txt")
			require.NoError(t, os.Mkdir(blocker, 0700))
			sentinel := filepath.Join(blocker, "existing-identity")
			require.NoError(t, os.WriteFile(sentinel, []byte("retain this identity"), 0600))
			var output string
			switch entry {
			case "initializer":
				nodeID, pub, initErr := genutil.InitializeNodeValidatorFiles(env.config)
				err = initErr
				require.Empty(t, nodeID)
				require.Nil(t, pub)
			case "init":
				output, err = repairM653Execute(t, env, genutilcli.InitCmd(env.basics, env.config.RootDir), "validator")
			case "collect-gentxs":
				output, err = repairM653Execute(t, env, genutilcli.CollectGenTxsCmd(banktypes.GenesisBalancesIterator{}, env.config.RootDir))
			}
			require.Error(t, err)
			var pathErr *os.PathError
			require.True(t, errors.As(err, &pathErr), "%v", err)
			require.Equal(t, "open", pathErr.Op)
			require.Equal(t, blocker, pathErr.Path)
			require.Empty(t, output)
			require.Equal(t, nodeBefore, repairM653Read(t, env.config.NodeKeyFile()))
			require.Equal(t, []byte("retain this identity"), repairM653Read(t, sentinel))
			_, err = os.Stat(env.config.PrivValidator.StateFile())
			require.True(t, os.IsNotExist(err))
			_, err = os.Stat(env.config.GenesisFile())
			require.True(t, os.IsNotExist(err))
			// The failed save cannot establish a usable validator identity on reopen.
			_, err = privval.LoadFilePV(env.config.PrivValidator.KeyFile(), env.config.PrivValidator.StateFile())
			require.Error(t, err)
		})
	}
}

func TestRepairM_653_CollectMissingBasicFields(t *testing.T) {
	for _, field := range []string{"min_self_delegation", "commission", "rate", "max_rate", "max_change_rate"} {
		t.Run(field, func(t *testing.T) {
			env := repairM653Environment(t)
			path, encoded := repairM653Generate(t, env)
			var raw map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(encoded, &raw))
			var body map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(raw["body"], &body))
			var messages []map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(body["messages"], &messages))
			if field == "rate" || field == "max_rate" || field == "max_change_rate" {
				var commission map[string]json.RawMessage
				require.NoError(t, json.Unmarshal(messages[0]["commission"], &commission))
				delete(commission, field)
				value, err := json.Marshal(commission)
				require.NoError(t, err)
				messages[0]["commission"] = value
			} else {
				delete(messages[0], field)
			}
			value, err := json.Marshal(messages)
			require.NoError(t, err)
			body["messages"] = value
			value, err = json.Marshal(body)
			require.NoError(t, err)
			raw["body"] = value
			encoded, err = json.Marshal(raw)
			require.NoError(t, err)
			_, err = env.client.TxConfig.TxJSONDecoder()(encoded)
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(path, encoded, 0600))
			before := repairM653Read(t, env.config.GenesisFile())
			output, err := repairM653Execute(t, env, genutilcli.CollectGenTxsCmd(banktypes.GenesisBalancesIterator{}, env.config.RootDir))
			require.ErrorContains(t, err, filepath.Base(path))
			require.ErrorContains(t, err, "invalid MsgCreateValidator")
			if field != "min_self_delegation" {
				require.ErrorContains(t, err, field)
			}
			require.Empty(t, output)
			require.Equal(t, before, repairM653Read(t, env.config.GenesisFile()))
			require.Equal(t, encoded, repairM653Read(t, path))
		})
	}
}
