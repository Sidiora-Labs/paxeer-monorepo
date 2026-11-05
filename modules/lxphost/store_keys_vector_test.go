//go:build cgo

package lxphost

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStoreKeysVectorV1(t *testing.T) {
	type moduleVector struct {
		Name         string `json:"name"`
		Module       uint16 `json:"module"`
		KernelKeyHex string `json:"kernel_key_hex"`
		StoreKeyHex  string `json:"store_key_hex"`
	}
	type accountVector struct {
		Name         string `json:"name"`
		AccountIDHex string `json:"account_id_hex"`
		StoreKeyHex  string `json:"store_key_hex"`
	}
	vectors := struct {
		Version     int             `json:"version"`
		ModuleKeys  []moduleVector  `json:"module_keys"`
		AccountKeys []accountVector `json:"account_keys"`
	}{Version: 1}

	var programID, accountID, leaseID [32]byte
	for i := range programID {
		programID[i] = byte(i)
		accountID[i] = byte(0x80 + i)
		leaseID[i] = byte(0xe0 + i)
	}
	kernelKey := func(prefix string, id [32]byte) []byte {
		return append([]byte(prefix), id[:]...)
	}
	route := kernelKey("wind-down\x00r", programID)
	route = append(route, accountID[:]...)
	history := kernelKey("wind-down\x00h", programID)
	history = binary.BigEndian.AppendUint64(history, 0x0102030405060708)
	for _, input := range []struct {
		name string
		key  []byte
	}{
		{"program", kernelKey("program\x00", programID)},
		{"program_interface", kernelKey("interface\x00", programID)},
		{"program_storage_head", append(kernelKey("progstor", programID), 1)},
		{"program_owner", kernelKey("program-owner\x00", programID)},
		{"wind_down_status", kernelKey("wind-down\x00s", programID)},
		{"wind_down_route", route},
		{"wind_down_history", history},
		{"sandbox_lease", kernelKey("s\x00", leaseID)},
		{"sandbox_escrow", kernelKey("s\x01", leaseID)},
		{"sandbox_usage", kernelKey("s\x02", leaseID)},
	} {
		key := moduleKVKey(9, input.key)
		require.Len(t, key, 3+len(input.key), input.name)
		require.True(t, bytes.HasPrefix(key, []byte{0x01, 0x00, 0x09}), input.name)
		require.Equal(t, input.key, key[3:], input.name)
		vectors.ModuleKeys = append(vectors.ModuleKeys, moduleVector{
			Name: input.name, Module: 9,
			KernelKeyHex: hex.EncodeToString(input.key),
			StoreKeyHex:  hex.EncodeToString(key),
		})
	}
	for _, input := range []struct {
		name string
		id   [32]byte
	}{
		{"wind_down_account", accountID},
		{"zero_account", [32]byte{}},
	} {
		key := accountKey(input.id)
		require.Len(t, key, 33, input.name)
		require.Equal(t, byte(0x06), key[0], input.name)
		require.Equal(t, input.id[:], key[1:], input.name)
		vectors.AccountKeys = append(vectors.AccountKeys, accountVector{
			Name: input.name, AccountIDHex: hex.EncodeToString(input.id[:]),
			StoreKeyHex: hex.EncodeToString(key),
		})
	}

	encoded, err := json.MarshalIndent(vectors, "", "  ")
	require.NoError(t, err)
	encoded = append(encoded, '\n')
	path := filepath.Join("testdata", "store_keys_v1.json")
	if os.Getenv("LXP_WRITE_VECTORS") == "1" {
		require.NoError(t, os.WriteFile(path, encoded, 0o644))
	}
	committed, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, string(committed), string(encoded), "store key vector differs; regenerate with LXP_WRITE_VECTORS=1")
}
