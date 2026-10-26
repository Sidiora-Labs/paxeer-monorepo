package types_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/types"
)

func TestStoreKeyLayoutFrozen(t *testing.T) {
	require.Equal(t, "|", types.KeySeparator)
	require.Equal(t, "mem_tokenfactory", types.MemStoreKey)

	for _, tc := range []struct {
		name string
		got  []byte
		want []byte
	}{
		{
			name: "authority metadata key",
			got:  []byte(types.DenomAuthorityMetadataKey),
			want: []byte("authoritymetadata"),
		},
		{
			name: "denoms prefix key",
			got:  []byte(types.DenomsPrefixKey),
			want: []byte("denoms"),
		},
		{
			name: "creator prefix key",
			got:  []byte(types.CreatorPrefixKey),
			want: []byte("creator"),
		},
		{
			name: "paused prefix key",
			got:  []byte(types.DenomPausedPrefixKey),
			want: []byte("paused"),
		},
		{
			name: "allowlist max size key",
			got:  types.DenomAllowListMaxSizeKey,
			want: []byte("allowlistmaxsize"),
		},
		{
			name: "denom prefix",
			got:  types.GetDenomPrefixStore("d"),
			want: []byte("denoms|d|"),
		},
		{
			name: "creator prefix",
			got:  types.GetCreatorPrefix("c"),
			want: []byte("creator|c|"),
		},
		{
			name: "creators prefix",
			got:  types.GetCreatorsPrefix(),
			want: []byte("creator|"),
		},
		{
			name: "paused denom key",
			got:  types.GetDenomPausedKey("d"),
			want: []byte("paused|d"),
		},
		{
			name: "empty denom prefix",
			got:  types.GetDenomPrefixStore(""),
			want: []byte("denoms||"),
		},
		{
			name: "empty creator prefix",
			got:  types.GetCreatorPrefix(""),
			want: []byte("creator||"),
		},
		{
			name: "empty paused denom key",
			got:  types.GetDenomPausedKey(""),
			want: []byte("paused|"),
		},
		{
			name: "factory denom prefix",
			got:  types.GetDenomPrefixStore("factory/c/sub/denom"),
			want: []byte("denoms|factory/c/sub/denom|"),
		},
		{
			name: "paused factory denom key",
			got:  types.GetDenomPausedKey("factory/c/sub/denom"),
			want: []byte("paused|factory/c/sub/denom"),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, tc.got)
		})
	}
}
