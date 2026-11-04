package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/flags"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/hd"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keyring"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	genutiltypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/genutil/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

func TestRepairM_1093_ReplayProfilerLoopbackAndCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	profiler, err := startReplayProfiler(ctx, 0)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, profiler.Close()) })

	conn, err := net.DialTimeout("tcp", profiler.Addr, time.Second)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	bound, ok := conn.RemoteAddr().(*net.TCPAddr)
	require.True(t, ok)
	require.True(t, bound.IP.IsLoopback(), "inspect the connected listener, not the profiling log")
	require.Equal(t, net.IPv4(127, 0, 0, 1).To4(), bound.IP.To4())
	require.Positive(t, bound.Port)

	transport := &http.Transport{Proxy: nil}
	t.Cleanup(transport.CloseIdleConnections)
	client := &http.Client{Transport: transport, Timeout: 3 * time.Second}
	response, err := client.Get("http://" + conn.RemoteAddr().String() + "/debug/pprof/")
	require.NoError(t, err)
	body, err := io.ReadAll(response.Body)
	require.NoError(t, response.Body.Close())
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.Contains(t, string(body), "goroutine")
	require.Contains(t, string(body), "profile")

	duplicate, err := startReplayProfiler(t.Context(), uint16(bound.Port))
	require.Error(t, err, "a bind failure must be returned to the replay command")
	require.Nil(t, duplicate)

	// Leave a real HTTP connection open while cancellation closes the server.
	require.NoError(t, conn.SetDeadline(time.Now().Add(3*time.Second)))
	_, err = io.WriteString(conn, "GET /debug/pprof/ HTTP/1.1\r\nHost: localhost\r\n")
	require.NoError(t, err)
	cancel()
	_, err = conn.Read(make([]byte, 1))
	require.Error(t, err)
	if networkError, ok := err.(net.Error); ok {
		require.False(t, networkError.Timeout(), "cancellation must close established connections")
	}
	require.Eventually(t, func() bool {
		connection, dialErr := net.DialTimeout("tcp", profiler.Addr, 100*time.Millisecond)
		if dialErr != nil {
			return true
		}
		_ = connection.Close()
		return false
	}, 3*time.Second, 10*time.Millisecond, "cancellation must close the listener")

	closed, err := startReplayProfiler(ctx, 0)
	require.ErrorIs(t, err, context.Canceled)
	require.Nil(t, closed)
}

func TestRepairM_1093_ReplayProfilerClose(t *testing.T) {
	profiler, err := startReplayProfiler(t.Context(), 0)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, profiler.Close()) })
	transport := &http.Transport{Proxy: nil}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 3 * time.Second}
	response, err := client.Get("http://" + profiler.Addr + "/debug/pprof/goroutine?debug=1")
	require.NoError(t, err)
	body, err := io.ReadAll(response.Body)
	require.NoError(t, response.Body.Close())
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.Contains(t, string(body), "goroutine profile:")
	require.NoError(t, profiler.Close(), "the replay command closes profiling on return, even without cancellation")
	conn, err := net.DialTimeout("tcp", profiler.Addr, time.Second)
	if conn != nil {
		_ = conn.Close()
	}
	require.Error(t, err)
}

func TestRepairM_1093_MixedKeyringGenesisAccount(t *testing.T) {
	for _, selection := range []string{"address", "name"} {
		t.Run(selection, func(t *testing.T) {
			home := t.TempDir()
			kb, err := keyring.New(sdk.KeyringServiceName(), keyring.BackendTest, home, bytes.NewReader(nil))
			require.NoError(t, err)
			offlinePublic := hd.Secp256k1.Generate()(bytes.Repeat([]byte{1}, 32)).PubKey()
			offline, err := kb.SavePubKey("a-offline", offlinePublic, hd.Secp256k1Type)
			require.NoError(t, err)
			const mnemonic = "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"
			const path = "m/44'/118'/0'/0/0"
			local, err := kb.NewAccount("z-local", mnemonic, "", path, hd.Secp256k1)
			require.NoError(t, err)
			keys, err := kb.List()
			require.NoError(t, err)
			require.Len(t, keys, 2)
			require.Equal(t, keyring.TypeOffline, keys[0].GetType())
			require.Equal(t, "a-offline", keys[0].GetName())
			require.Equal(t, keyring.TypeLocal, keys[1].GetType())
			require.Equal(t, local.GetAddress(), keys[1].GetAddress())

			derived, err := hd.Secp256k1.Derive()(mnemonic, "", path)
			require.NoError(t, err)
			expectedPrivate, err := crypto.ToECDSA(derived)
			require.NoError(t, err)
			private, err := getPrivateKeyOfAddr(kb, local.GetAddress())
			require.NoError(t, err)
			require.Equal(t, derived, crypto.FromECDSA(private))
			require.Equal(t, local.GetPubKey().Bytes(), crypto.CompressPubkey(&private.PublicKey))
			unsupported, err := getPrivateKeyOfAddr(kb, offline.GetAddress())
			require.EqualError(t, err, "not local key")
			require.Nil(t, unsupported)
			missingAddress := sdk.AccAddress(hd.Secp256k1.Generate()(bytes.Repeat([]byte{2}, 32)).PubKey().Address())
			missing, err := getPrivateKeyOfAddr(kb, missingAddress)
			require.ErrorContains(t, err, "not found")
			require.Nil(t, missing)

			encoding := app.MakeEncodingConfig()
			state := app.NewDefaultGenesisState(encoding.Marshaler)
			raw, err := json.Marshal(state)
			require.NoError(t, err)
			serverCtx := server.NewDefaultContext()
			serverCtx.Config.SetRoot(home)
			genesisPath := serverCtx.Config.GenesisFile()
			require.NoError(t, os.MkdirAll(filepath.Dir(genesisPath), 0700))
			doc := &tmtypes.GenesisDoc{ChainID: "repair-m1093", AppState: raw}
			require.NoError(t, doc.ValidateAndComplete())
			require.NoError(t, doc.SaveAs(genesisPath))

			clientCtx := client.Context{}.WithCodec(encoding.Marshaler).WithHomeDir(home)
			ctx := context.WithValue(t.Context(), client.ClientContextKey, &clientCtx)
			ctx = context.WithValue(ctx, server.ServerContextKey, serverCtx)
			command := AddGenesisAccountCmd(home)
			command.SetContext(ctx)
			command.SetOut(io.Discard)
			command.SetErr(io.Discard)
			account := local.GetAddress().String()
			if selection == "name" {
				account = local.GetName()
			}
			command.SetArgs([]string{account, "123uhpx", "--" + flags.FlagKeyringBackend, keyring.BackendTest})
			require.NoError(t, command.Execute(), "the command reopens the on-disk mixed keyring")

			after, _, err := genutiltypes.GenesisStateFromGenFile(genesisPath)
			require.NoError(t, err)
			evmGenesis := evm.GetGenesisStateFromAppState(encoding.Marshaler, after)
			require.Equal(t, []*evmtypes.AddressAssociation{{
				PaxAddress: local.GetAddress().String(),
				EthAddress: crypto.PubkeyToAddress(expectedPrivate.PublicKey).Hex(),
			}}, evmGenesis.AddressAssociations)
			authGenesis := authtypes.GetGenesisStateFromAppState(encoding.Marshaler, after)
			accounts, err := authtypes.UnpackAccounts(authGenesis.Accounts)
			require.NoError(t, err)
			require.Len(t, accounts, 1)
			require.Equal(t, local.GetAddress(), accounts[0].GetAddress())
			require.NoError(t, accounts[0].Validate())
			bankGenesis := banktypes.GetGenesisStateFromAppState(encoding.Marshaler, after)
			require.Equal(t, []banktypes.Balance{{
				Address: local.GetAddress().String(), Coins: sdk.NewCoins(sdk.NewInt64Coin("uhpx", 123)),
			}}, bankGenesis.Balances)
		})
	}
}
