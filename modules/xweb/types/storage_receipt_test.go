package types_test

import (
	"crypto/sha256"
	"strings"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/types"
	"github.com/stretchr/testify/require"
)

func TestStorageReceiptPayloadRoundTripAndRefusals(t *testing.T) {
	digest := sha256.Sum256([]byte("stored object"))
	valid := types.StorageReceiptPayload{ObjectDigest: digest, BaseURL: "https://storage.example/bucket"}
	require.Equal(t, uint8(4), types.KindStorageReceipt)
	require.True(t, types.KnownKind(types.KindStorageReceipt))
	require.False(t, types.KnownKind(5))

	for _, baseURL := range []string{
		valid.BaseURL,
		"https://storage.example",
		"https://storage.example/" + strings.Repeat("x", 512-len("https://storage.example/")),
		"https://storage.example/桶",
	} {
		t.Run("round trip "+baseURL, func(t *testing.T) {
			payload := types.StorageReceiptPayload{ObjectDigest: digest, BaseURL: baseURL}
			require.NoError(t, types.ValidateStorageReceiptPayload(payload))
			raw, err := types.EncodeStorageReceiptPayload(payload)
			require.NoError(t, err)
			require.Equal(t, 32+len(baseURL), len(raw))
			require.Equal(t, digest[:], raw[:32])
			require.Equal(t, []byte(baseURL), raw[32:])
			decoded, err := types.DecodeStorageReceiptPayload(raw)
			require.NoError(t, err)
			require.Equal(t, payload, decoded)
			roundTrip, err := types.EncodeStorageReceiptPayload(decoded)
			require.NoError(t, err)
			require.Equal(t, raw, roundTrip)
		})
	}

	for name, payload := range map[string]types.StorageReceiptPayload{
		"zero digest":     {BaseURL: valid.BaseURL},
		"http":            {ObjectDigest: digest, BaseURL: "http://storage.example/bucket"},
		"query":           {ObjectDigest: digest, BaseURL: valid.BaseURL + "?version=1"},
		"empty query":     {ObjectDigest: digest, BaseURL: valid.BaseURL + "?"},
		"fragment":        {ObjectDigest: digest, BaseURL: valid.BaseURL + "#object"},
		"empty fragment":  {ObjectDigest: digest, BaseURL: valid.BaseURL + "#"},
		"userinfo":        {ObjectDigest: digest, BaseURL: "https://user@storage.example/bucket"},
		"no host":         {ObjectDigest: digest, BaseURL: "https:///bucket"},
		"empty hostname":  {ObjectDigest: digest, BaseURL: "https://:443/bucket"},
		"trailing slash":  {ObjectDigest: digest, BaseURL: valid.BaseURL + "/"},
		"empty URL":       {ObjectDigest: digest},
		"invalid UTF-8":   {ObjectDigest: digest, BaseURL: valid.BaseURL + string([]byte{0xff})},
		"malformed URL":   {ObjectDigest: digest, BaseURL: valid.BaseURL + "%zz"},
		"513-byte URL":    {ObjectDigest: digest, BaseURL: "https://storage.example/" + strings.Repeat("x", 513-len("https://storage.example/"))},
		"513 UTF-8 bytes": {ObjectDigest: digest, BaseURL: "https://storage.example/" + strings.Repeat("桶", 163)},
	} {
		t.Run(name, func(t *testing.T) {
			require.ErrorIs(t, types.ValidateStorageReceiptPayload(payload), types.ErrInvalidPayload)
			_, err := types.EncodeStorageReceiptPayload(payload)
			require.ErrorIs(t, err, types.ErrInvalidPayload)
			raw := append(append([]byte(nil), payload.ObjectDigest[:]...), []byte(payload.BaseURL)...)
			_, err = types.DecodeStorageReceiptPayload(raw)
			require.ErrorIs(t, err, types.ErrInvalidPayload)
		})
	}
	for _, length := range []int{0, 31} {
		_, err := types.DecodeStorageReceiptPayload(make([]byte, length))
		require.ErrorIs(t, err, types.ErrInvalidPayload)
	}
}
