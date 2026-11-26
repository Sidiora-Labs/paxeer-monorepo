package quoter

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

func TestKeyFileIsPrivateAndDiagnosticsOmitItsContents(t *testing.T) {
	key, err := crypto.GenerateKey()
	require.NoError(t, err)
	encoded := "0x" + commonKeyHex(key.D.Bytes())
	path := filepath.Join(t.TempDir(), "signer.hex")
	require.NoError(t, os.WriteFile(path, []byte(encoded+"\n"), 0600))
	loaded, err := readKey(path)
	require.NoError(t, err)
	require.Equal(t, key.D, loaded.D)
	require.NoError(t, os.Chmod(path, 0644))
	_, err = readKey(path)
	require.Error(t, err)
	require.NotContains(t, err.Error(), encoded)
	require.NotContains(t, err.Error(), path)
	require.NoError(t, os.Chmod(path, 0400))
	_, err = readKey(path)
	require.NoError(t, err)
	link := filepath.Join(t.TempDir(), "link")
	require.NoError(t, os.Symlink(path, link))
	_, err = readKey(link)
	require.Error(t, err)
	bad := filepath.Join(t.TempDir(), "bad")
	require.NoError(t, os.WriteFile(bad, []byte("private-key-material"), 0600))
	_, err = readKey(bad)
	require.Error(t, err)
	require.NotContains(t, err.Error(), "private-key-material")
}
func commonKeyHex(data []byte) string {
	const digits = "0123456789abcdef"
	output := make([]byte, 64)
	for i := range output {
		output[i] = '0'
	}
	start := 64 - 2*len(data)
	for i, b := range data {
		output[start+2*i] = digits[b>>4]
		output[start+2*i+1] = digits[b&15]
	}
	return string(output)
}
func TestDryTransportCannotSubmit(t *testing.T) {
	r := &RPC{mode: "dry"}
	err := r.Place(context.Background(), Order{Market: 0, Buy: true, Price: clone(unit), SizeUSD: clone(minimumOrderUSD), Base: clone(unit), Cloid: ClientID(0, true, 1, 1)})
	require.EqualError(t, err, "dry mode sends no transactions")
	require.Nil(t, r.pending)
}
func TestRPCErrorReasonsDoNotContainEndpointCredentials(t *testing.T) {
	private := errors.New("https://user:password@rpc.example/secret-path?token=private")
	require.Equal(t, "chain request failed", rpcError(private, false).Error())
	require.Equal(t, "chain request failed", rpcError(private, true).Error())
	require.Equal(t, "insufficient gas balance", rpcError(errors.New("insufficient funds: private-key-material"), true).Error())
}
