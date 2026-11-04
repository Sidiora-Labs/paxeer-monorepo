package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	rpctypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/flags"
	clienttx "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/tx"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/hd"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keyring"
	kmultisig "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/multisig"
	cryptotypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/std"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/testdata"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	txtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/tx"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/tx/signing"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/client/cli"
	authsigning "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/signing"
	authtx "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/tx"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
)

type repair660Fixture struct {
	ctx   client.Context
	codec *codec.ProtoCodec
	keys  []keyring.Info
	multi keyring.Info
}

func repair660Setup(t *testing.T) repair660Fixture {
	t.Helper()
	registry := codectypes.NewInterfaceRegistry()
	std.RegisterInterfaces(registry)
	authtypes.RegisterInterfaces(registry)
	banktypes.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	amino := codec.NewLegacyAmino()
	std.RegisterLegacyAminoCodec(amino)
	authtypes.RegisterLegacyAminoCodec(amino)
	banktypes.RegisterLegacyAminoCodec(amino)
	kb := keyring.NewInMemory()
	r := repair660Fixture{codec: cdc}
	for i, name := range []string{"alice", "bob"} {
		info, err := kb.NewAccount(name, testdata.TestMnemonic, "", hd.CreateHDPath(118, 0, uint32(i)).String(), hd.Secp256k1)
		require.NoError(t, err)
		r.keys = append(r.keys, info)
	}
	multi := kmultisig.NewLegacyAminoPubKey(2, []cryptotypes.PubKey{r.keys[0].GetPubKey(), r.keys[1].GetPubKey()})
	var err error
	r.multi, err = kb.SaveMultisig("multi", multi)
	require.NoError(t, err)
	r.ctx = client.Context{}.WithCodec(cdc).WithLegacyAmino(amino).WithInterfaceRegistry(registry).
		WithTxConfig(authtx.NewTxConfig(cdc, authtx.DefaultSignModes)).WithKeyring(kb).
		WithChainID("repair-660").WithAccountRetriever(authtypes.AccountRetriever{}).
		WithHomeDir(t.TempDir()).WithOutput(io.Discard)
	return r
}

func repair660File(t *testing.T, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "batch.json")
	require.NoError(t, os.WriteFile(path, data, 0600))
	return path
}

func repair660Execute(cmd *cobra.Command, ctx client.Context, input io.Reader, args ...string) ([]byte, string, error) {
	var out, stderr bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&stderr)
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	if input != nil {
		cmd.SetIn(input)
	}
	if cmd.Flags().Lookup(flags.FlagChainID) != nil {
		args = append(args, "--"+flags.FlagChainID+"="+ctx.ChainID)
	}
	cmd.SetArgs(args)
	err := cmd.ExecuteContext(context.WithValue(context.Background(), client.ClientContextKey, &ctx))
	return out.Bytes(), stderr.String(), err
}

func repair660OfflineArgs(args ...string) []string {
	return append(args, "--offline", "--account-number=7", "--sequence=11")
}

func (r repair660Fixture) unsigned(t *testing.T, multi bool, memos ...string) []byte {
	t.Helper()
	signer := r.keys[0].GetAddress()
	if multi {
		signer = r.multi.GetAddress()
	}
	var batch bytes.Buffer
	for _, memo := range memos {
		factory := clienttx.Factory{}.WithTxConfig(r.ctx.TxConfig).WithChainID(r.ctx.ChainID).
			WithGas(200000).WithMemo(memo)
		builder, err := factory.BuildUnsignedTx(&banktypes.MsgSend{
			FromAddress: signer.String(), ToAddress: r.keys[1].GetAddress().String(),
			Amount: sdk.NewCoins(sdk.NewInt64Coin("stake", 1)),
		})
		require.NoError(t, err)
		bz, err := r.ctx.TxConfig.TxJSONEncoder()(builder.GetTx())
		require.NoError(t, err)
		batch.Write(bz)
		batch.WriteByte('\n')
	}
	return batch.Bytes()
}

func (r repair660Fixture) signatures(t *testing.T, unsigned []byte, multi bool, name string) []byte {
	t.Helper()
	args := repair660OfflineArgs(repair660File(t, unsigned), "--from="+name, "--sign-mode=amino-json")
	if multi {
		args = append(args, "--multisig=multi")
	}
	out, _, err := repair660Execute(cli.GetSignBatchCommand(), r.ctx, nil, args...)
	require.NoError(t, err)
	for _, line := range bytes.Split(bytes.TrimSpace(out), []byte{'\n'}) {
		sigs, err := r.ctx.TxConfig.UnmarshalSignatureJSON(line)
		require.NoError(t, err)
		require.Len(t, sigs, 1)
		require.NotEmpty(t, sigs[0].Data.(*signing.SingleSignatureData).Signature)
	}
	return out
}

func (r repair660Fixture) signatureFiles(t *testing.T, unsigned []byte) []string {
	t.Helper()
	var paths []string
	for _, key := range r.keys {
		paths = append(paths, repair660File(t, r.signatures(t, unsigned, true, key.GetName())))
	}
	return paths
}

func (r repair660Fixture) assertMultisigned(t *testing.T, data []byte, memos ...string) {
	t.Helper()
	lines := bytes.Split(bytes.TrimSpace(data), []byte{'\n'})
	require.Len(t, lines, len(memos))
	for i, line := range lines {
		parsed, err := r.ctx.TxConfig.TxJSONDecoder()(line)
		require.NoError(t, err)
		tx := parsed.(authsigning.Tx)
		require.Equal(t, memos[i], tx.GetMemo())
		sigs, err := tx.GetSignaturesV2()
		require.NoError(t, err)
		require.Len(t, sigs, 1)
		require.True(t, sigs[0].PubKey.Equals(r.multi.GetPubKey()))
		require.Equal(t, uint64(11+i), sigs[0].Sequence)
		require.NoError(t, authsigning.VerifySignature(sigs[0].PubKey, authsigning.SignerData{
			ChainID: r.ctx.ChainID, AccountNumber: 7, Sequence: uint64(11 + i),
		}, sigs[0].Data, r.ctx.TxConfig.SignModeHandler(), parsed))
	}
}

func TestRepairM_660_MultisignOutputFile(t *testing.T) {
	for _, batch := range []bool{false, true} {
		name := "single"
		if batch {
			name = "batch"
		}
		t.Run(name, func(t *testing.T) {
			r := repair660Setup(t)
			memos := []string{"first"}
			if batch {
				memos = append(memos, "second")
			}
			unsigned := r.unsigned(t, true, memos...)
			sigFiles := r.signatureFiles(t, unsigned)
			output := repair660File(t, []byte("previous document contents"))
			var previous bytes.Buffer
			ctx := r.ctx.WithOutput(&previous)
			args := append([]string{repair660File(t, unsigned), "multi"}, sigFiles...)
			args = repair660OfflineArgs(append(args, "--output-document="+output)...)
			cmd := cli.GetMultiSignCommand()
			if batch {
				cmd = cli.GetMultiSignBatchCmd()
			}
			out, _, err := repair660Execute(cmd, ctx, nil, args...)
			require.NoError(t, err)
			require.Empty(t, out)
			require.Empty(t, previous.Bytes())
			data, err := os.ReadFile(output)
			require.NoError(t, err)
			r.assertMultisigned(t, data, memos...)
		})
	}
	t.Run("single signature-only", func(t *testing.T) {
		r := repair660Setup(t)
		unsigned := r.unsigned(t, true, "signature")
		args := append([]string{repair660File(t, unsigned), "multi"}, r.signatureFiles(t, unsigned)...)
		output := repair660File(t, nil)
		var previous bytes.Buffer
		out, _, err := repair660Execute(cli.GetMultiSignCommand(), r.ctx.WithOutput(&previous), nil,
			repair660OfflineArgs(append(args, "--signature-only", "--output-document="+output)...)...)
		require.NoError(t, err)
		require.Empty(t, out)
		require.Empty(t, previous.Bytes())
		data, err := os.ReadFile(output)
		require.NoError(t, err)
		sigs, err := r.ctx.TxConfig.UnmarshalSignatureJSON(data)
		require.NoError(t, err)
		require.Len(t, sigs, 1)
		require.True(t, sigs[0].PubKey.Equals(r.multi.GetPubKey()))
		require.Len(t, sigs[0].Data.(*signing.MultiSignatureData).Signatures, 2)
	})
}

func TestRepairM_660_MultisignBatchCardinality(t *testing.T) {
	for _, longer := range []bool{false, true} {
		name := "shorter"
		if longer {
			name = "longer"
		}
		t.Run(name, func(t *testing.T) {
			r := repair660Setup(t)
			two := r.unsigned(t, true, "first", "second")
			one := r.unsigned(t, true, "first")
			sigFiles := r.signatureFiles(t, two)
			input, badSigs := two, one
			want := "missing signature for transaction 2"
			if longer {
				input, badSigs, want = one, two, "unused signature for transaction 2"
			}
			// Keep the first file valid so the error identifies the second signer file.
			sigFiles[0] = repair660File(t, r.signatures(t, input, true, "alice"))
			sigFiles[1] = repair660File(t, r.signatures(t, badSigs, true, "bob"))
			args := append([]string{repair660File(t, input), "multi"}, sigFiles...)
			var out []byte
			var err error
			require.NotPanics(t, func() {
				out, _, err = repair660Execute(cli.GetMultiSignBatchCmd(), r.ctx, nil, repair660OfflineArgs(args...)...)
			})
			require.ErrorContains(t, err, want)
			require.ErrorContains(t, err, "signature file 2")
			require.ErrorContains(t, err, sigFiles[1])
			r.assertMultisigned(t, out, "first")
		})
	}
}

func repair660FailingPipe(t *testing.T, prefix []byte, failure error) io.Reader {
	t.Helper()
	r, w := io.Pipe()
	t.Cleanup(func() { _ = r.Close() })
	go func() {
		_, err := w.Write(prefix)
		if err != nil {
			_ = w.CloseWithError(err)
			return
		}
		_ = w.CloseWithError(failure)
	}()
	return r
}

func repair660ReaderCases(t *testing.T, multi bool) {
	t.Helper()
	for _, scenario := range []string{"over 64 KiB", "line limit", "reader failure", "JSON failure"} {
		t.Run(scenario, func(t *testing.T) {
			r := repair660Setup(t)
			first := r.unsigned(t, multi, "first")
			input := first
			var reader io.Reader
			readErr := errors.New("repair660 input transport failure")
			wantErr := ""
			wantLines := 1
			if scenario == "over 64 KiB" {
				input = r.unsigned(t, multi, strings.Repeat("m", 80*1024))
				require.Greater(t, len(input), 64*1024)
			} else if scenario == "line limit" {
				input = append(append([]byte(nil), first...), r.unsigned(t, multi, strings.Repeat("m", 16*1024*1024))...)
				wantErr = "token too long"
			} else if scenario == "reader failure" {
				reader = repair660FailingPipe(t, first, readErr)
				wantErr = readErr.Error()
			} else {
				input = append(append([]byte(nil), first...), []byte("{invalid JSON}\n")...)
				wantErr = "failed to decode"
			}
			cmd := cli.GetSignBatchCommand()
			args := []string{repair660File(t, input), "--from=alice", "--sign-mode=amino-json"}
			if multi {
				cmd = cli.GetMultiSignBatchCmd()
				signInput := first
				if scenario == "over 64 KiB" {
					signInput = input
				}
				args = append([]string{args[0], "multi"}, r.signatureFiles(t, signInput)...)
			}
			if reader != nil {
				args[0] = "-"
			}
			out, _, err := repair660Execute(cmd, r.ctx, reader, repair660OfflineArgs(args...)...)
			if wantErr == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				if scenario == "reader failure" {
					require.ErrorIs(t, err, readErr)
				} else if scenario == "line limit" {
					require.ErrorContains(t, err, wantErr)
				}
			}
			lines := bytes.Split(bytes.TrimSpace(out), []byte{'\n'})
			require.Len(t, lines, wantLines)
			if multi {
				memo := "first"
				if scenario == "over 64 KiB" {
					memo = strings.Repeat("m", 80*1024)
				}
				r.assertMultisigned(t, out, memo)
			} else {
				sigs, decodeErr := r.ctx.TxConfig.UnmarshalSignatureJSON(lines[0])
				require.NoError(t, decodeErr)
				require.Len(t, sigs, 1)
				require.True(t, sigs[0].PubKey.Equals(r.keys[0].GetPubKey()))
			}
		})
	}
}

func TestRepairM_660_MultisignBatchReaderErrors(t *testing.T) {
	repair660ReaderCases(t, true)
}

func TestRepairM_660_SignBatchReaderErrors(t *testing.T) {
	repair660ReaderCases(t, false)
}

func (r repair660Fixture) signedSingle(t *testing.T) *txtypes.Tx {
	t.Helper()
	unsigned := r.unsigned(t, false, "single")
	out, _, err := repair660Execute(cli.GetSignCommand(), r.ctx, nil,
		repair660OfflineArgs(repair660File(t, unsigned), "--from=alice", "--sign-mode=amino-json")...)
	require.NoError(t, err)
	decoded, err := r.ctx.TxConfig.TxJSONDecoder()(out)
	require.NoError(t, err)
	return decoded.(interface{ GetProtoTx() *txtypes.Tx }).GetProtoTx()
}

func (r repair660Fixture) validate(t *testing.T, tx *txtypes.Tx, online bool, ctx client.Context) (string, error) {
	t.Helper()
	bz, err := r.ctx.TxConfig.TxJSONEncoder()(tx)
	require.NoError(t, err)
	args := []string{repair660File(t, bz)}
	if !online {
		args = repair660OfflineArgs(args...)
	}
	var stderr string
	require.NotPanics(t, func() {
		_, stderr, err = repair660Execute(cli.GetValidateSignaturesCommand(), ctx, nil, args...)
	})
	return stderr, err
}

func TestRepairM_660_ValidateOptionalKey(t *testing.T) {
	t.Run("offline omitted key", func(t *testing.T) {
		r := repair660Setup(t)
		tx := r.signedSingle(t)
		tx.AuthInfo.SignerInfos[0].PublicKey = nil
		stderr, err := r.validate(t, tx, false, r.ctx)
		require.Error(t, err)
		require.Contains(t, stderr, "missing public key for signature 0")
		require.Contains(t, stderr, "online account lookup is required")
	})
	t.Run("unsigned transaction", func(t *testing.T) {
		r := repair660Setup(t)
		decoded, err := r.ctx.TxConfig.TxJSONDecoder()(r.unsigned(t, false, "unsigned"))
		require.NoError(t, err)
		stderr, err := r.validate(t, decoded.(interface{ GetProtoTx() *txtypes.Tx }).GetProtoTx(), false, r.ctx)
		require.Error(t, err)
		require.Contains(t, stderr, "signature count 0 does not match required signer count 1")
	})
	for _, scenario := range []string{"established key", "no established key", "wrong established key", "tampered transaction"} {
		t.Run("online "+scenario, func(t *testing.T) {
			r := repair660Setup(t)
			tx := r.signedSingle(t)
			tx.AuthInfo.SignerInfos[0].PublicKey = nil
			var publicKey cryptotypes.PubKey = r.keys[0].GetPubKey()
			if scenario == "no established key" {
				publicKey = nil
			}
			if scenario == "wrong established key" {
				publicKey = r.keys[1].GetPubKey()
			}
			if scenario == "tampered transaction" {
				tx.Body.Memo = "changed after signing"
			}
			account := authtypes.NewBaseAccount(r.keys[0].GetAddress(), publicKey, 7, 11)
			accountAny, err := codectypes.NewAnyWithValue(account)
			require.NoError(t, err)
			response, err := r.codec.Marshal(&authtypes.QueryAccountResponse{Account: accountAny})
			require.NoError(t, err)
			var queries atomic.Int64
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				var rpcReq rpctypes.RPCRequest
				if err := json.NewDecoder(req.Body).Decode(&rpcReq); err != nil {
					t.Error(err)
					http.Error(w, err.Error(), 400)
					return
				}
				var params coretypes.RequestABCIQuery
				if err := json.Unmarshal(rpcReq.Params, &params); err != nil {
					t.Error(err)
					http.Error(w, err.Error(), 400)
					return
				}
				require.Equal(t, "abci_query", rpcReq.Method)
				require.Equal(t, "/cosmos.auth.v1beta1.Query/Account", params.Path)
				var query authtypes.QueryAccountRequest
				require.NoError(t, r.codec.Unmarshal(params.Data, &query))
				require.Equal(t, r.keys[0].GetAddress().String(), query.Address)
				queries.Add(1)
				reply := rpcReq.MakeResponse(&coretypes.ResultABCIQuery{Response: abci.ResponseQuery{Value: response, Height: 1}})
				require.NoError(t, json.NewEncoder(w).Encode(reply))
			}))
			defer server.Close()
			rpc, err := client.NewClientFromNode(server.URL)
			require.NoError(t, err)
			stderr, err := r.validate(t, tx, true, r.ctx.WithClient(rpc))
			if scenario == "established key" {
				require.NoError(t, err, stderr)
				require.Equal(t, int64(2), queries.Load())
			} else {
				require.Error(t, err)
				require.Greater(t, queries.Load(), int64(0))
				if scenario == "no established key" {
					require.Contains(t, stderr, "no public key established")
				}
				if scenario == "tampered transaction" {
					require.Contains(t, stderr, "unable to verify single signer signature")
				}
			}
		})
	}
}

func TestRepairM_660_ValidateMalformedMultisig(t *testing.T) {
	for _, scenario := range []string{"invalid bytes", "conversion error", "missing member", "missing bit array", "missing outer signature", "missing mode info"} {
		t.Run(scenario, func(t *testing.T) {
			r := repair660Setup(t)
			unsigned := r.unsigned(t, true, "multi")
			args := append([]string{repair660File(t, unsigned), "multi"}, r.signatureFiles(t, unsigned)...)
			out, _, err := repair660Execute(cli.GetMultiSignCommand(), r.ctx, nil, repair660OfflineArgs(args...)...)
			require.NoError(t, err)
			parsed, err := r.ctx.TxConfig.TxJSONDecoder()(out)
			require.NoError(t, err)
			tx := parsed.(interface{ GetProtoTx() *txtypes.Tx }).GetProtoTx()
			// Establish a valid producer/consumer path before corrupting its transport data.
			stderr, err := r.validate(t, tx, false, r.ctx)
			require.NoError(t, err, stderr)
			want := "invalid signature 0"
			switch scenario {
			case "invalid bytes":
				tx.Signatures[0] = []byte{0xff}
			case "conversion error":
				tx.Signatures[0] = append(tx.Signatures[0], 0x10, 0x01) // unknown protobuf field
				want = "failed to convert signatures"
			case "missing member":
				var raw cryptotypes.MultiSignature
				require.NoError(t, raw.Unmarshal(tx.Signatures[0]))
				raw.Signatures = raw.Signatures[:1]
				tx.Signatures[0], err = raw.Marshal()
				require.NoError(t, err)
			case "missing bit array":
				tx.AuthInfo.SignerInfos[0].ModeInfo.GetMulti().Bitarray = nil
			case "missing outer signature":
				tx.Signatures = nil
				want = "signature count does not match signer info count"
			case "missing mode info":
				tx.AuthInfo.SignerInfos[0].ModeInfo = nil
			}
			stderr, err = r.validate(t, tx, false, r.ctx)
			require.Error(t, err)
			require.Contains(t, stderr, want)
		})
	}
}
