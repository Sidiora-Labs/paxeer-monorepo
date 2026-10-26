package types_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/types"
)

func TestGenesisRejectsMalformedIssuerDid(t *testing.T) {
	const admin = "pax1y3pxq5dp900czh0mkudhjdqjq5m8cpmmsnt288"
	const denom = "factory/" + admin + "/bitcoin"

	for _, tc := range []struct {
		name      string
		issuerDid string
		valid     bool
	}{
		{name: "short key", issuerDid: "did:layerx:XYZ"},
		{name: "uppercase key", issuerDid: "did:layerx:" + strings.Repeat("AB", 32)},
		{name: "valid key", issuerDid: "did:layerx:" + strings.Repeat("ab", 32), valid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			genesis := types.DefaultGenesis()
			genesis.FactoryDenoms = []types.GenesisDenom{{
				Denom: denom,
				AuthorityMetadata: types.DenomAuthorityMetadata{
					Admin:     admin,
					IssuerDid: tc.issuerDid,
				},
			}}

			err := genesis.Validate()
			if tc.valid {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, types.ErrInvalidAuthorityMetadata)
				require.ErrorContains(t, err, "denom "+denom+":")
				require.ErrorContains(t, err, "invalid issuer did ("+tc.issuerDid+")")
			}
		})
	}
}
