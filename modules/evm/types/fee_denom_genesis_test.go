package types_test

import (
	"strings"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestValidateAccountFeeDenomGenesisEntry(t *testing.T) {
	account := common.HexToAddress("0x1234567890abcdef1234567890abcdef12345678")
	storageKey := types.AccountFeeDenomKey(account)
	require.Equal(t, byte(0x23), storageKey[0])
	cases := []struct {
		name    string
		key     []byte
		value   []byte
		wantErr bool
	}{
		{name: "native denom", key: storageKey[1:], value: []byte("uhpx")},
		{name: "IBC denom", key: storageKey[1:], value: []byte("ibc/" + strings.Repeat("A", 64))},
		{name: "minimum denom length", key: storageKey[1:], value: []byte("abc")},
		{name: "maximum denom length", key: storageKey[1:], value: []byte(strings.Repeat("a", 128))},
		{name: "zero address", key: make([]byte, common.AddressLength), value: []byte("uhpx")},
		{name: "missing key", value: []byte("uhpx"), wantErr: true},
		{name: "short key", key: storageKey[1:20], value: []byte("uhpx"), wantErr: true},
		{name: "long key", key: make([]byte, common.AddressLength+1), value: []byte("uhpx"), wantErr: true},
		{name: "missing denom", key: storageKey[1:], wantErr: true},
		{name: "empty denom", key: storageKey[1:], value: []byte{}, wantErr: true},
		{name: "short denom", key: storageKey[1:], value: []byte("ab"), wantErr: true},
		{name: "long denom", key: storageKey[1:], value: []byte(strings.Repeat("a", 129)), wantErr: true},
		{name: "numeric denom start", key: storageKey[1:], value: []byte("1hpx"), wantErr: true},
		{name: "whitespace in denom", key: storageKey[1:], value: []byte("u hpx"), wantErr: true},
		{name: "invalid denom character", key: storageKey[1:], value: []byte("uhpx!"), wantErr: true},
		{name: "non UTF-8 denom", key: storageKey[1:], value: []byte{'u', 0xff, 'x'}, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, encoding := range []struct {
				name   string
				prefix []byte
				key    []byte
			}{
				{name: "split", prefix: types.AccountFeeDenomKeyPrefix, key: tc.key},
				{name: "prefix only", prefix: append([]byte{0x23}, tc.key...)},
				{name: "prefix with empty key", prefix: append([]byte{0x23}, tc.key...), key: []byte{}},
			} {
				t.Run(encoding.name, func(t *testing.T) {
					err := types.ValidateAccountFeeDenomGenesisEntry(encoding.prefix, encoding.key, tc.value)
					if tc.wantErr {
						require.Error(t, err)
					} else {
						require.NoError(t, err)
					}
				})
			}
		})
	}
	t.Run("other prefixes ignore malformed entries", func(t *testing.T) {
		require.NoError(t, types.ValidateAccountFeeDenomGenesisEntry(nil, nil, nil))
		for prefix := 0; prefix <= 255; prefix++ {
			if byte(prefix) == types.AccountFeeDenomKeyPrefix[0] {
				continue
			}
			require.NoError(t, types.ValidateAccountFeeDenomGenesisEntry([]byte{byte(prefix)}, []byte{1}, []byte("!")), "prefix %x", prefix)
			require.NoError(t, types.ValidateAccountFeeDenomGenesisEntry([]byte{byte(prefix), 1}, nil, []byte("!")), "prefix-only %x", prefix)
		}
	})
}
